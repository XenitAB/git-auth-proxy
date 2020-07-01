package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	flag "github.com/spf13/pflag"

	"github.com/xenitab/azdo-git-proxy/pkg/auth"
	"github.com/xenitab/azdo-git-proxy/pkg/config"
)

func main() {
	// Read configuration
	port := flag.Int("port", 8080, "Port to bind server to.")
	configPath := flag.String("config", "/var/config.json", "Path to configuration file.")
	flag.Parse()

	log.Printf("Reading configuration file at path: %v\n", *configPath)
	config, err := config.LoadConfigurationFromPath(*configPath)
	if err != nil {
		log.Fatalf("Failed loading configuration: %v\n", err)
	}

	log.Printf("Starting git proxy for host: %v on port %v\n", config.Domain, *port)
	remote, err := url.Parse("https://" + config.Domain)
	if err != nil {
		log.Fatalf("Invalid remote url: %v\n", err)
	}

	// Configure revers proxy and http server
	proxy := httputil.NewSingleHostReverseProxy(remote)
	router := mux.NewRouter()
	router.HandleFunc("/readyz", readinessHandler).Methods("GET")
	router.HandleFunc("/healthz", livenessHandler).Methods("GET")
	router.PathPrefix("/").HandlerFunc(proxyHandler(proxy, config))

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
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Print("Server Started")

	// Blocks until singal is sent
	<-done
	log.Print("Server Stopped")

	// Shutdown http server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Print("Server Exited Properly")
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"status\": \"ok\"}"))
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"status\": \"ok\"}"))
}

func proxyHandler(p *httputil.ReverseProxy, c *config.Configuration) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// If basic auth is missing return error to force client to retry
		username, _, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Missing basic authentication", http.StatusUnauthorized)
			return
		}

		// Check basic auth with local auth configuration
		err := auth.IsPermitted(c, r.URL.Path, username)
		if err != nil {
			log.Printf("Received unauthorized request: %v\n", err)
			http.Error(w, "User not permitted", http.StatusForbidden)
			return
		}

		// Forward request to destination server
		log.Printf("Succesfully authenticated at path: %v\n", r.URL.Path)
		r.Host = c.Domain
		patB64 := base64.StdEncoding.EncodeToString([]byte("pat:" + c.Pat))
		r.Header.Del("Authorization")
		r.Header.Add("Authorization", "Basic "+patB64)
		p.ServeHTTP(w, r)
	}
}
