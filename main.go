package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

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
	zapLog, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	logger := zapr.NewLogger(zapLog)
	if err := run(logger); err != nil {
		logger.WithName("main").Error(err, "run error")
	}
}

func run(logger logr.Logger) error {
	mainLogger := logger.WithName("main")

	// Load configuration and authorization
	mainLogger.Info("Reading configuration", "path", configPath)
	path, err := filepath.Rel("/", configPath)
	if err != nil {
		return fmt.Errorf("could not get relative path: %w", err)
	}
	cfg, err := config.LoadConfiguration(os.DirFS("/"), path)
	if err != nil {
		return fmt.Errorf("could not load configuration: %w", err)
	}
	mainLogger.Info("Generating authorization")
	authz, err := auth.NewAuthorization(cfg)
	if err != nil {
		return fmt.Errorf("could not generate authorization: %w", err)
	}
	client, err := getKubernetesClient(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("invalid kubernetes client: %w", err)
	}

	// Setup signal handler and error groups
	ctx := context.Background()
	ctx = logr.NewContext(ctx, logger)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stopCh)

	g, ctx := errgroup.WithContext(ctx)

	// Run services
	mainLogger.Info("Starting metrics server", "port", metricsPort)
	metricsSrv := &http.Server{Addr: metricsPort, Handler: promhttp.Handler()}
	g.Go(func() error {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	mainLogger.Info("Starting token writer")
	tokenWriter := token.NewTokenWriter(logger, client, authz)
	g.Go(func() error {
		if err := tokenWriter.Start(ctx.Done()); err != nil {
			return err
		}
		return nil
	})

	mainLogger.Info("Starting proxy server")
	azdoSrv, err := server.NewAzdoServer(logger, port, authz)
	if err != nil {
		return fmt.Errorf("could not create azdo proxy server: %w", err)
	}
	g.Go(func() error {
		if err := azdoSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// Wait for stop signal and shutdown
	select {
	case <-stopCh:
		break
	case <-ctx.Done():
		break
	}

	cancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer timeoutCancel()
	go metricsSrv.Shutdown(timeoutCtx)
	go azdoSrv.Shutdown(timeoutCtx)

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error groups error: %w", err)
	}

	mainLogger.Info("gracefully shutdown")
	return nil
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
