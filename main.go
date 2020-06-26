package main

import (
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/xenitab/azdo-git-proxy/pkg/config"
)

func main() {
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

	proxy := httputil.NewSingleHostReverseProxy(remote)
	http.HandleFunc("/readyz", readinessHandler())
	http.HandleFunc("/healthz", livenessHandler())
	http.HandleFunc("/", proxyHandler(proxy, config))
	err = http.ListenAndServe(":"+strconv.Itoa(*port), nil)
	if err != nil {
		log.Fatalf("Could not start http server: %v\n", err)
	}
}

func readinessHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\": \"ok\"}"))
	}
}

func livenessHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\": \"ok\"}"))
	}
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

		if !isPermitted(c, r.URL.Path, username) {
			log.Printf("Received unauthorized request: %v\n", r.URL.Path)
			http.Error(w, "User not permitted", http.StatusForbidden)
			return
		}

		log.Printf("Succesfully authenticated at path: %v\n", r.URL.Path)
		r.Host = c.Domain
		patB64 := base64.StdEncoding.EncodeToString([]byte("pat:" + c.Pat))
		r.Header.Del("Authorization")
		r.Header.Add("Authorization", "Basic "+patB64)
		p.ServeHTTP(w, r)
	}
}

// isPermitted checks if a specific user is permitted to access a path
func isPermitted(c *config.Configuration, p string, t string) bool {
	comp := strings.Split(p, "/")
	org := comp[1]
	proj := comp[2]
	repo := comp[4]

	if comp[3] != "_git" {
		return false
	}

	if c.Organization != org {
		return false
	}

	for _, repository := range c.Repositories {
		if repository.Project == proj && repository.Name == repo && repository.Token == t {
			return true
		}
	}

	return false
}
