package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/xenitab/azdo-proxy/pkg/auth"
)

type AzdoServer struct {
	logger logr.Logger
	port   string
	proxy  *httputil.ReverseProxy
	authz  auth.Authorization
}

func NewAzdoServer(logger logr.Logger, port string, authz auth.Authorization) *AzdoServer {
	director := func(req *http.Request) {
		token, err := tokenFromRequest(req)
		if err != nil {
			logger.Error(err, "token not present in request")
			return
		}
		target, err := authz.TargetForToken(token)
		if err != nil {
			logger.Error(err, "could not get target for token")
			return
		}

		targetQuery := target.RawQuery
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}

	return &AzdoServer{
		logger: logger.WithName("azdo-server"),
		port:   port,
		proxy:  &httputil.ReverseProxy{Director: director},
		authz:  authz,
	}
}

// ListenAndServe starts the HTTP server on the specified port
func (a *AzdoServer) ListenAndServe(stopCh <-chan struct{}) {
	a.logger.Info("Starting git proxy", "port", a.port)
	router := mux.NewRouter()
	router.HandleFunc("/readyz", readinessHandler(a.logger)).Methods("GET")
	router.HandleFunc("/healthz", livenessHandler(a.logger)).Methods("GET")
	router.PathPrefix("/").HandlerFunc(proxyHandler(a.logger, a.proxy, a.authz))
	srv := &http.Server{Addr: a.port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
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

func proxyHandler(logger logr.Logger, p *httputil.ReverseProxy, authz auth.Authorization) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
		pat, err := authz.TargetForToken(token)

		// Forward request to destination server
		logger.Info("Authenticated request", "path", r.URL.Path)
		r.Header.Del("Authorization")
		patB64 := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("pat:%s", pat)))
		r.Header.Add("Authorization", "Basic "+patB64)
		p.ServeHTTP(w, r)
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

	return token, nil
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")

	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}

	return a + b
}

func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}

	// Same as singleJoiningSlash, but uses EscapedPath to determine
	// whether a slash should be added
	apath := a.EscapedPath()
	bpath := b.EscapedPath()
	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")

	switch {
	case aslash && bslash:
		return a.Path + b.Path[1:], apath + bpath[1:]
	case !aslash && !bslash:
		return a.Path + "/" + b.Path, apath + "/" + bpath
	}

	return a.Path + b.Path, apath + bpath
}
