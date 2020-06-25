package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	flag "github.com/spf13/pflag"
)

type Repository struct {
	Token string `json:"token"`
}

type Project struct {
	Repositories map[string]Repository `json:"repositories"`
}

type Configuration struct {
	Domain       string                       `json:"domain"`
	Pat          string                       `json:"pat"`
	Organization string                       `json:"organization"`
	Repositories map[string]map[string]string `json:"repositories"`
}

func main() {
	port := flag.Int("port", 8080, "Port to bind server to.")
	configPath := flag.String("config", "/var/config.json", "Path to configuration file.")
	flag.Parse()

	log.Printf("Reading configuration file at path: %v\n", *configPath)
	config, err := getConfiguration(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting git proxy for host: %v on port %v\n", config.Domain, *port)
	remote, err := url.Parse("https://" + config.Domain)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	http.HandleFunc("/readyz", readinessHandler())
	http.HandleFunc("/healthz", livenessHandler())
	http.HandleFunc("/", proxyHandler(proxy, config))
	err = http.ListenAndServe(":"+strconv.Itoa(*port), nil)
	if err != nil {
		log.Fatal(err)
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

func proxyHandler(p *httputil.ReverseProxy, c *Configuration) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// If basic auth is missing return error to force client to retry
		username, _, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Missing basic authentication", http.StatusUnauthorized)
			return
		}

		if !isPermitted(c, r.URL.Path, username) {
			log.Printf("Received unauthorized request: %v\n", username)
			http.Error(w, "User not permitted", http.StatusForbidden)
			return
		}

		//log.Printf("Succesfully authenticated: user: %v, path: %v", check.Username, check.Path)
		r.Host = c.Domain
		patB64 := base64.StdEncoding.EncodeToString([]byte("pat:" + c.Pat))
		r.Header.Del("Authorization")
		r.Header.Add("Authorization", "Basic "+patB64)
		p.ServeHTTP(w, r)
	}
}

// getConfiguration reads json from the given path and parses it.
func getConfiguration(path string) (*Configuration, error) {
	file, _ := os.Open(path)
	defer file.Close()
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		return nil, err
	}

	return &configuration, nil
}

// isPermitted checks if a specific user is permitted to access a path
func isPermitted(c *Configuration, p string, t string) bool {
	comp := strings.Split(p, "/")
	org := comp[1]
	proj := comp[2]
	rep := comp[4]

	if comp[3] != "_git" {
		return false
	}

	if c.Organization != org {
		return false
	}

	if c.Repositories[proj][rep] != t {
		return false
	}

	return true
}
