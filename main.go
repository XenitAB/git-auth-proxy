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
	if err := run(logger, configPath, metricsPort); err != nil {
		logger.WithName("main").Error(err, "run error")
	}
}

func run(logger logr.Logger, configPath string, metricsPort string) error {
	mainLogger := logger.WithName("main")
	mainLogger.Info("read configuration from", "path", configPath)
	mainLogger.Info("serve metrics", "port", metricsPort, "endpoint", "/metrics")

	authz, err := getAutorization(configPath)
	if err != nil {
		return err
	}
	client, err := getKubernetesClient(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("invalid kubernetes client: %w", err)
	}

	ctx := context.Background()
	ctx = logr.NewContext(ctx, logger)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stopCh)

	metricsSrv := &http.Server{Addr: metricsPort, Handler: promhttp.Handler()}
	g.Go(func() error {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	tokenWriter := token.NewTokenWriter(logger, client, authz)
	g.Go(func() error {
		if err := tokenWriter.Start(ctx.Done()); err != nil {
			return err
		}
		return nil
	})
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
	g.Go(func() error {
		if err := metricsSrv.Shutdown(timeoutCtx); err != nil {
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := azdoSrv.Shutdown(timeoutCtx); err != nil {
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error groups error: %w", err)
	}
	mainLogger.Info("gracefully shutdown")
	return nil
}

func getAutorization(path string) (auth.Authorization, error) {
	path, err := filepath.Rel("/", path)
	if err != nil {
		return auth.Authorization{}, fmt.Errorf("could not get relative path: %w", err)
	}
	cfg, err := config.LoadConfiguration(os.DirFS("/"), path)
	if err != nil {
		return auth.Authorization{}, fmt.Errorf("could not load configuration: %w", err)
	}
	authz, err := auth.NewAuthorization(cfg)
	if err != nil {
		return auth.Authorization{}, fmt.Errorf("could not generate authorization: %w", err)
	}
	return authz, nil
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
