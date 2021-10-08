package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cristalhq/aconfig"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/xenitab/git-auth-proxy/pkg/auth"
	"github.com/xenitab/git-auth-proxy/pkg/config"
	"github.com/xenitab/git-auth-proxy/pkg/server"
	"github.com/xenitab/git-auth-proxy/pkg/token"
)

type cfg struct {
	Addr           string `flag:"addr" default:":8080" required:"true"`
	MetricsAddr    string `flag:"metrics-addr" default:":9090" required:"true"`
	ConfigPath     string `flag:"config" required:"true"`
	KubeconfigPath string `flag:"kubeconfig"`
}

func main() {
	zapLog, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	logger := zapr.NewLogger(zapLog)
	if err := run(logger); err != nil {
		logger.WithName("main").Error(err, "run error")
		os.Exit(1)
	}
}

// nolint:funlen // cant make this shorter
func run(logger logr.Logger) error {
	mainLogger := logger.WithName("main")

	cfg := &cfg{}
	loader := aconfig.LoaderFor(cfg, aconfig.Config{
		FlagDelimiter:    "-",
		AllFieldRequired: false,
	})
	if err := loader.Load(); err != nil {
		return err
	}

	mainLogger.Info("read configuration from", "path", cfg.ConfigPath)
	mainLogger.Info("serve metrics", "port", cfg.MetricsAddr, "endpoint", "/metrics")

	authz, err := getAutorization(cfg.ConfigPath)
	if err != nil {
		return err
	}
	client, err := getKubernetesClient(cfg.KubeconfigPath)
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

	metricsSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: promhttp.Handler()}
	g.Go(func() error {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	tokenWriter := token.NewTokenWriter(logger, client, authz)
	g.Go(func() error {
		if err := tokenWriter.Start(ctx); err != nil {
			return err
		}
		return nil
	})
	srv := server.NewServer(logger, cfg.Addr, authz)
	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
		if err := srv.Shutdown(timeoutCtx); err != nil {
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

func getAutorization(path string) (*auth.Authorizer, error) {
	cfg, err := config.LoadConfiguration(afero.NewOsFs(), path)
	if err != nil {
		return nil, fmt.Errorf("could not load configuration: %w", err)
	}
	authz, err := auth.NewAuthorizer(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not generate authorization: %w", err)
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
