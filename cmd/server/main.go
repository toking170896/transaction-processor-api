package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"transaction-processor/internal/config"
	"transaction-processor/internal/database"
	"transaction-processor/internal/handler"
	"transaction-processor/internal/logger"
	"transaction-processor/internal/repository/postgres"
	"transaction-processor/internal/service"
	"transaction-processor/internal/worker"

	_ "transaction-processor/docs"
)

// @title Transaction Processor API
// @version 1.0
// @description API for processing third-party provider transactions
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Setup logger
	log := logger.New(true)

	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Initialize database connection
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbPool, err := database.NewPool(dbCtx, cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer dbPool.Close()

	// Repositories
	userRepo := postgres.NewUserRepository(dbPool)
	transactionRepo := postgres.NewTransactionRepository(dbPool)

	// Transaction manage used by services
	txManager := postgres.NewTransactionManager(dbPool)

	// Services
	transService := service.NewTransactionService(userRepo, transactionRepo, txManager, log)
	cancelService := service.NewCancellationService(userRepo, transactionRepo, txManager, log)

	// Root context to be caceled on SIGINT / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Worker for odd transaction cancellation
	cancellationWorker := worker.NewCancellationWorker(cancelService, cfg.Worker.CancellationInterval, log)
	cancellationWorker.Start(ctx)
	defer cancellationWorker.Stop()

	// http handler
	h := handler.NewHandler(transService, log)
	router := h.SetupRoutes()

	// http server configuration
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	log.Info().Str("port", cfg.Server.Port).Msg("Server started")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info().Msg("Shutdown signal received, starting graceful shutdown...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	} else {
		log.Info().Msg("HTTP server stopped gracefully")
	}

	log.Info().Msg("Shutdown complete")
}
