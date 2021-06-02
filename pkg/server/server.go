package server

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	"github.com/xenitab/azdo-proxy/pkg/auth"
)

type AzdoServer struct {
	logger  logr.Logger
	port    string
	proxies map[string]*httputil.ReverseProxy
	authz   auth.Authorization
}

func NewAzdoServer(logger logr.Logger, port string, authz auth.Authorization) (*AzdoServer, error) {
	proxies := map[string]*httputil.ReverseProxy{}
	for k := range authz.GetEndpoints() {
		target, err := authz.GetTargetForToken(k)
		if err != nil {
			return nil, fmt.Errorf("could not create http proxy from endpoints: %w", err)
		}
		proxies[target.String()] = httputil.NewSingleHostReverseProxy(target)
	}

	return &AzdoServer{
		logger:  logger.WithName("azdo-server"),
		port:    port,
		proxies: proxies,
		authz:   authz,
	}, nil
}

// ListenAndServe starts the HTTP server on the specified port.
func (a *AzdoServer) ListenAndServe(stopCh <-chan struct{}) {
	a.logger.Info("Starting git proxy", "port", a.port)
	router := mux.NewRouter()

	prometheus_mdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix: "azdo_proxy",
		}),
	})

	router.Use(std.HandlerProvider("", prometheus_mdlw))
	router.HandleFunc("/readyz", readinessHandler(a.logger)).Methods("GET")
	router.HandleFunc("/healthz", livenessHandler(a.logger)).Methods("GET")
	router.PathPrefix("/").HandlerFunc(proxyHandler(a.logger, a.proxies, a.authz))
	srv := &http.Server{Addr: a.port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			a.logger.Error(err, "azdo-proxy server crashed")
			os.Exit(1)
		}
	}()

	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		a.logger.Error(err, "azdo-proxy server graceful shutdown failed")
	} else {
		a.logger.Info("azdo-proxy server stopped")
	}
}

//nolint:lll // difficult to make this shorter right now (Philip)
func proxyHandler(logger logr.Logger, proxies map[string]*httputil.ReverseProxy, authz auth.Authorization) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the token from the request
		token, err := tokenFromRequest(r)
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Missing basic authentication", http.StatusUnauthorized)
			return
		}

		// Check basic auth with local auth configuration
		err = authz.IsPermitted(r.URL.EscapedPath(), token)
		if err != nil {
			logger.Error(err, "Received unauthorized request")
			http.Error(w, "User not permitted", http.StatusForbidden)
			return
		}
		pat, err := authz.GetPatForToken(token)
		if err != nil {
			logger.Error(err, "Could not find the PAT for the given token")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		target, err := authz.GetTargetForToken(token)
		if err != nil {
			logger.Error(err, "Target could not be created from the token")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Overwrite the authorization header with the PAT token
		logger.Info("Authenticated request", "path", r.URL.Path)
		r.Header.Del("Authorization")
		patB64 := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("pat:%s", pat)))
		r.Header.Add("Authorization", "Basic "+patB64)

		// Forward the request to the correct proxy
		proxy, ok := proxies[target.String()]
		if !ok {
			logger.Info("missing proxy for target")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		proxy.ServeHTTP(w, r)
	}
}

func readinessHandler(log logr.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{\"status\": \"ok\"}")); err != nil {
			log.Error(err, "Could not write response data")
		}
	}
}

func livenessHandler(log logr.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{\"status\": \"ok\"}")); err != nil {
			log.Error(err, "Could not write response data")
		}
	}
}

func tokenFromRequest(r *http.Request) (string, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return "", fmt.Errorf("basic auth not present")
	}

	token := username
	if len(password) > 0 {
		token = password
	}

	if token == "" {
		return "", errors.New("token cannot be empty")
	}

	return token, nil
}
