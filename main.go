package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/gorilla/mux"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/xenitab/azdo-proxy/pkg/auth"
	"github.com/xenitab/azdo-proxy/pkg/config"
)

func main() {
	// Initiate logs
	var log logr.Logger
	zapLog, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	log = zapr.NewLogger(zapLog)
	setupLog := log.WithName("setup")

	// Read configuration
	port := flag.Int("port", 8080, "Port to bind server to.")
	configPath := flag.String("config", "/var/config.json", "Path to configuration file.")
	flag.Parse()

	setupLog.Info("Reading configuration file", "path", *configPath)
	config, err := config.LoadConfigurationFromPath(*configPath)
	if err != nil {
		setupLog.Error(err, "Failed loading configuration")
		os.Exit(1)
	}

	setupLog.Info("Starting git proxy", "domain", config.Domain, "port", *port)
	remote, err := url.Parse("https://" + config.Domain)
	if err != nil {
		setupLog.Error(err, "Invalid domain")
		os.Exit(1)
	}

	// Generate authorization object
	auth, err := auth.GenerateAuthorization(*config)
	if err != nil {
		setupLog.Error(err, "Could not generate Authorization")
		os.Exit(1)
	}

	// Configure revers proxy and http server
	proxy := httputil.NewSingleHostReverseProxy(remote)
	router := mux.NewRouter()
	router.HandleFunc("/readyz", readinessHandler(log.WithName("readiness"))).Methods("GET")
	router.HandleFunc("/healthz", livenessHandler(log.WithName("liveness"))).Methods("GET")
	router.PathPrefix("/").HandlerFunc(proxyHandler(proxy, log.WithName("reverse-proxy"), auth, config.Domain, config.Pat))

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(*port),
		Handler: router,
	}

	// Signal handler
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			setupLog.Error(err, "Http Server Error")
		}
	}()
	setupLog.Info("Server started")

	// Blocks until singal is sent
	<-done
	setupLog.Info("Server stopped")

	// Shutdown http server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := srv.Shutdown(ctx); err != nil {
		setupLog.Error(err, "Server shutdown failed")
		os.Exit(1)
	}

	setupLog.Info("Server exited properly")
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

func proxyHandler(p *httputil.ReverseProxy, log logr.Logger, a *auth.Authorization, domain string, pat string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// If basic auth is missing return error to force client to retry
		username, _, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Missing basic authentication", http.StatusUnauthorized)
			return
		}

		// Check basic auth with local auth configuration
		err := auth.IsPermitted(a, r.URL.EscapedPath(), username)
		if err != nil {
			log.Error(err, "Received unauthorized request")
			http.Error(w, "User not permitted", http.StatusForbidden)
			return
		}

		// Forward request to destination server
		log.Info("Authenticated request", "path", r.URL.Path)
		r.Host = domain
		r.Header.Del("Authorization")
		patB64 := base64.StdEncoding.EncodeToString([]byte("pat:" + pat))
		r.Header.Add("Authorization", "Basic "+patB64)

		p.ServeHTTP(w, r)
	}
}
