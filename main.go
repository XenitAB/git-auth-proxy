package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/alexflint/go-arg"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/afero"
	"github.com/xenitab/pkg/kubernetes"
	"go.uber.org/zap"

	"github.com/xenitab/git-auth-proxy/pkg/auth"
	"github.com/xenitab/git-auth-proxy/pkg/config"
	"github.com/xenitab/git-auth-proxy/pkg/server"
	"github.com/xenitab/git-auth-proxy/pkg/token"
)

func main() {
	var args struct {
		Addr           string `arg:"--addr" default:":8080"`
		MetricsAddr    string `arg:"--metrics-addr" default:":9090"`
		CfgPath        string `arg:"--config,required"`
		KubeconfigPath string `arg:"--kubeconfig"`
	}
	arg.MustParse(&args)

	zapLog, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("who watches the watchmen (%v)?", err)
		os.Exit(1)
	}
	logger := zapr.NewLogger(zapLog)

	if err := run(logger, args.Addr, args.MetricsAddr, args.CfgPath, args.KubeconfigPath); err != nil {
		logger.WithName("main").Error(err, "run error")
		os.Exit(1)
	}
}

func run(logger logr.Logger, addr, metricsAddr, cfgPath, kubeconfigPath string) error {
	authz, err := getAutorization(cfgPath)
	if err != nil {
		return err
	}

	client, err := kubernetes.GetKubernetesClientset(kubeconfigPath)
	if err != nil {
		return err
	}

	ctx := logr.NewContext(context.Background(), logger)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	metricsSrv := &http.Server{Addr: metricsAddr, Handler: promhttp.Handler()}
	g.Go(func() error {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return metricsSrv.Shutdown(shutdownCtx)
	})

	tokenWriter := token.NewTokenWriter(logger, client, authz)
	g.Go(func() error {
		if err := tokenWriter.Start(ctx); err != nil {
			return err
		}
		return nil
	})

	srv := server.NewServer(logger, addr, authz)
	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil {
		return err
	}
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
