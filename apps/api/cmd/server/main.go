package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/config"
	"github.com/OstKost/avari-p3-express/apps/api/internal/database"
	"github.com/OstKost/avari-p3-express/apps/api/internal/handler"
	"github.com/OstKost/avari-p3-express/apps/api/internal/repository/sqlite"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
)

// @title Avari P3.express & SDLC API
// @version 1.0
// @description Production-grade Project Management REST API based on P3.express, SDLC, and AI-SDLC methodologies.
// @contact.name Repository Maintainer
// @contact.url https://github.com/OstKost/avari-p3-express
// @license.name MIT
// @host localhost:4820
// @BasePath /
func main() {
	// 1. Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Avari P3 Project Management API service...")

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 3. Connect to Database and apply migrations
	db, err := database.NewConnection(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to connect to SQLite", "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Error closing database connection", "error", err)
		}
	}()

	if err := database.Migrate(db); err != nil {
		slog.Error("Failed to apply database migrations", "error", err)
		os.Exit(1)
	}

	managerKey, err := config.ManagerKey(cfg.ManagerKeyPath)
	if err != nil {
		slog.Error("Cannot load manager key", "error", err)
		os.Exit(1)
	}

	// 4. Dependency Injection / Composition Root
	p3Repo := sqlite.NewP3Repository(db)
	p3Service := service.NewP3Service(p3Repo)
	p3Handler := handler.NewP3Handler(p3Service)

	router := handler.NewRouter(handler.RouterConfig{
		P3Handler:      p3Handler,
		WorkHandler:    handler.NewWorkHandler(service.NewWorkService(sqlite.NewWorkRepository(db)), p3Service, cfg.AllowedOrigins, managerKey, cfg.PublicManagerOrigin),
		DB:             db,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Start Server in background
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP server is listening", "port", cfg.Port, "base_url", cfg.BaseURL)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 6. Graceful Shutdown listener
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		slog.Error("Fatal server error", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		slog.Info("Shutdown signal received, initiating graceful shutdown", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("Graceful shutdown failed, forcing server close", "error", err)
			_ = server.Close()
			os.Exit(1)
		}
		slog.Info("Server exited cleanly")
	}
}
