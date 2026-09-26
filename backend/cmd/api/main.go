package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/config"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/handler"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/logger"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

func main() {
	if err := run(); err != nil {
		slog.Error("backend berhenti", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	gin.SetMode(cfg.GinMode)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.NewRouter(cfg, db, log),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("backend listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
