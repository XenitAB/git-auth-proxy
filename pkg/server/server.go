package server

import (
	"context"
	"net/http"
	"net/http/httputil"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	prommetrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"

	"github.com/xenitab/azdo-proxy/pkg/auth"
)

type Server struct {
	logger logr.Logger
	srv    *http.Server
}

func NewServer(logger logr.Logger, port string, authz *auth.Authorizer) *Server {
	router := mux.NewRouter()
	prometheus_mdlw := middleware.New(middleware.Config{
		Recorder: prommetrics.NewRecorder(prommetrics.Config{
			Prefix: "azdo_proxy",
		}),
	})
	router.Use(std.HandlerProvider("", prometheus_mdlw))
	router.HandleFunc("/readyz", readinessHandler(logger)).Methods("GET")
	router.HandleFunc("/healthz", livenessHandler(logger)).Methods("GET")
	router.PathPrefix("/").HandlerFunc(proxyHandler(logger, authz))
	srv := &http.Server{Addr: port, Handler: router}

	return &Server{
		logger: logger.WithName("azdo-server"),
		srv:    srv,
	}
}

func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func proxyHandler(logger logr.Logger, authz *auth.Authorizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := context.Background()
		handlerLogger := logger.WithValues("path", req.URL.Path)

		// Get the token from the request
		token, err := getTokenFromRequest(req)
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Missing basic authentication", http.StatusUnauthorized)
			return
		}
		// Check basic auth with local auth configuration
		err = authz.IsPermitted(req.URL.EscapedPath(), token)
		if err != nil {
			handlerLogger.Error(err, "Received unauthorized request")
			http.Error(w, "User not permitted", http.StatusForbidden)
			return
		}
		// Authenticate the request with the proper token
		req, url, err := authz.UpdateRequest(ctx, req, token)
		if err != nil {
			handlerLogger.Error(err, "Could not authenticate request")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		handlerLogger.Info("Authenticated request")

		// TODO (Philip): Add caching of the proxy
		// Forward the request to the correct proxy
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(w, req)
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
