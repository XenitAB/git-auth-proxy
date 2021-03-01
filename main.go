package main

import (
	"context"
	"encoding/base64"
	"errors"
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

var (
	port       int
	configPath string
)

func init() {
	flag.IntVar(&port, "port", 8080, "port to bind server to.")
	flag.StringVar(&configPath, "config", "/var/config.json", "path to configuration file.")
	flag.Parse()
}

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
	setupLog.Info("Reading configuration file", "path", configPath)
	config, err := config.LoadConfigurationFromPath(configPath)
	if err != nil {
		setupLog.Error(err, "Failed loading configuration")
		os.Exit(1)
	}
	remote, err := url.Parse("https://" + config.Domain)
	if err != nil {
		setupLog.Error(err, "Invalid Azure DevOps domain")
		os.Exit(1)
	}
	auth, err := auth.GenerateAuthorization(*config)
	if err != nil {
		setupLog.Error(err, "Could not generate authorization")
		os.Exit(1)
	}

	// Signal handler
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Validate PAT with Azure DevOps
	err = validatePat(config.Pat, config.Organization)
	if err != nil {
		setupLog.Error(err, "PAT is invalid")
		os.Exit(1)
	}

	// Run PAT validation every minute
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				setupLog.Info("Validating PAT")
				err = validatePat(config.Pat, config.Organization)
				if err != nil {
					setupLog.Error(err, "PAT is invalid")
					os.Exit(1)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	// Configure revers proxy and http server
	setupLog.Info("Starting git proxy", "domain", config.Domain, "port", port)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	router := mux.NewRouter()
	router.HandleFunc("/readyz", readinessHandler(log.WithName("readiness"))).Methods("GET")
	router.HandleFunc("/healthz", livenessHandler(log.WithName("liveness"))).Methods("GET")
	router.PathPrefix("/").HandlerFunc(proxyHandler(proxy, log.WithName("reverse-proxy"), auth, config.Domain, config.Pat))
	srv := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: router}

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
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Missing basic authentication", http.StatusUnauthorized)
			return
		}

		t := username
		if len(password) > 0 {
			t = password
		}

		// Check basic auth with local auth configuration
		err := auth.IsPermitted(a, r.URL.EscapedPath(), t)
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

func validatePat(pat string, org string) error {
	url := fmt.Sprintf("https://dev.azure.com/%v/_apis/connectionData", org)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	patB64 := base64.StdEncoding.EncodeToString([]byte(":" + pat))
	req.Header.Add("Authorization", "Basic "+patB64)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("authorization failed")
	}

	return nil
}
