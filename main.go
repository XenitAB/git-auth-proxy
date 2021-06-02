package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/xenitab/azdo-proxy/pkg/auth"
	"github.com/xenitab/azdo-proxy/pkg/config"
	"github.com/xenitab/azdo-proxy/pkg/server"
	"github.com/xenitab/azdo-proxy/pkg/token"
)

var (
	port           string
	metricsPort    string
	kubeconfigPath string
	configPath     string
)

func init() {
	flag.StringVar(&port, "port", ":8080", "port to bind proxy server to.")
	flag.StringVar(&metricsPort, "metrics-port", ":9090", "port to bind metrics endpoint to.")
	flag.StringVar(&kubeconfigPath, "kubeconfig", "", "absolute path to the kubeconfig file.")
	flag.StringVar(&configPath, "config", "/var/config.json", "path to configuration file.")
	flag.Parse()
}

func main() {
	// Initiate logs
	var logger logr.Logger
	zapLog, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	logger = zapr.NewLogger(zapLog)
	setupLog := logger.WithName("setup")

	// Load configuration and authorization
	setupLog.Info("Reading configuration", "path", configPath)
	path, err := filepath.Rel("/", configPath)
	if err != nil {
		setupLog.Error(err, "Could not make relative path to basepath")
		os.Exit(1)
	}
	cfg, err := config.LoadConfiguration(os.DirFS("/"), path)
	if err != nil {
		setupLog.Error(err, "Could not get relative path")
		os.Exit(1)
	}
	if err != nil {
		setupLog.Error(err, "Failed loading configuration")
		os.Exit(1)
	}
	setupLog.Info("Generating authorization")
	authz, err := auth.NewAuthorization(cfg)
	if err != nil {
		setupLog.Error(err, "Could not generate authorization")
		os.Exit(1)
	}

	// Start Azure DevOps proxy server
	ctx := signals.SetupSignalHandler()

	setupLog.Info("Starting metrics server", "port", metricsPort)
	metricsSrv := &http.Server{Addr: metricsPort, Handler: promhttp.Handler()}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			setupLog.Error(err, "metrics server crashed")
			os.Exit(1)
		}
	}()

	setupLog.Info("Starting token writer")
	client, err := getKubernetesClient(kubeconfigPath)
	if err != nil {
		setupLog.Error(err, "Invalid kubernetes client")
		os.Exit(1)
	}
	tokenWriter := token.NewTokenWriter(logger, client, authz)
	go func() {
		err := tokenWriter.Start(ctx.Done())
		if err != nil {
			setupLog.Error(err, "Token writer returned error")
			os.Exit(1)
		}
	}()

	setupLog.Info("Starting server")
	azdoServer, err := server.NewAzdoServer(logger, port, authz)
	if err != nil {
		setupLog.Error(err, "Could not create server")
		os.Exit(1)
	}
	azdoServer.ListenAndServe(ctx.Done())

	setupLog.Info("Azure DevOps Proxy stopped")
}

func getKubernetesClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	if kubeconfigPath != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return clientset, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
