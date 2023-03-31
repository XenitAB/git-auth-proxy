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

type Arguments struct {
	Addr           string `arg:"--addr" default:":8080"`
	MetricsAddr    string `arg:"--metrics-addr" default:":9090"`
	CfgPath        string `arg:"--config,required"`
	KubeconfigPath string `arg:"--kubeconfig"`
}

func main() {
	args := &Arguments{}
	arg.MustParse(args)

	zapLog, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	log := zapr.NewLogger(zapLog)
	ctx := logr.NewContext(context.Background(), log)

	if err := run(ctx, args.Addr, args.MetricsAddr, args.CfgPath, args.KubeconfigPath); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	log.Info("gracefully shutdown")
}

func run(ctx context.Context, addr, metricsAddr, cfgPath, kubeconfigPath string) error {
	authz, err := getAutorization(cfgPath)
	if err != nil {
		return err
	}
	client, err := kubernetes.GetKubernetesClientset(kubeconfigPath)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	metricsSrv := &http.Server{ReadTimeout: 5 * time.Second, Addr: metricsAddr, Handler: promhttp.Handler()}
	g.Go(func() error {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return metricsSrv.Shutdown(shutdownCtx)
	})

	tokenWriter := token.NewTokenWriter(client, authz)
	g.Go(func() error {
		if err := tokenWriter.Start(ctx); err != nil {
			return err
		}
		return nil
	})

	srv := server.NewServer(ctx, addr, authz)
	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	logr.FromContextOrDiscard(ctx).Info("running git-auth-proxy")
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
