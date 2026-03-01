package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/example/rate-limiter/internal/config"
	"github.com/example/rate-limiter/internal/limiter"
	"github.com/example/rate-limiter/internal/log"
	"github.com/example/rate-limiter/internal/proxy"
	"github.com/example/rate-limiter/internal/version"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("rate-limiter version %s\n", version.Version)
		fmt.Printf("Build time: %s\n", version.BuildTime)
		fmt.Printf("Git commit: %s\n", version.GitCommit)
		os.Exit(0)
	}

	logger, err := log.New()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("starting rate-limiter",
		zap.String("version", version.Version),
		zap.String("build_time", version.BuildTime),
		zap.String("git_commit", version.GitCommit),
	)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// In-memory rate limiter, не требует внешних зависимостей.
	rl := limiter.NewMemoryLimiter()
	defer func() {
		if err := rl.Close(); err != nil {
			logger.Error("failed to close limiter", zap.Error(err))
		}
	}()

	handler, err := proxy.NewHandler(cfg, rl, logger)
	if err != nil {
		logger.Fatal("failed to create proxy handler", zap.Error(err))
	}

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      handler.Router(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info("starting rate-limiter", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	logger.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
}

