package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/internal/worker/handlers"
	"github.com/14mdzk/goscratch/pkg/logger"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logLevel := "info"
	if cfg.IsDevelopment() {
		logLevel = "debug"
	}
	appLogger := logger.New(logger.Config{
		Level:  logLevel,
		Format: "json",
	})

	appLogger.Info("Starting worker process",
		"app", cfg.App.Name,
		"env", cfg.App.Env,
	)

	ctx := context.Background()

	// Initialize database connection
	appLogger.Info("Connecting to database...")
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	appLogger.Info("Database connected successfully")

	// Initialize queue
	if !cfg.RabbitMQ.Enabled {
		log.Fatal("RabbitMQ must be enabled to run the worker. Set rabbitmq.enabled=true in config.")
	}

	appLogger.Info("Connecting to RabbitMQ...")
	queueAdapter, err := queue.NewRabbitMQ(cfg.RabbitMQ.URL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer queueAdapter.Close()
	appLogger.Info("RabbitMQ connected successfully")

	// Get worker config with defaults
	queueName := cfg.Worker.QueueName
	if queueName == "" {
		queueName = "jobs"
	}
	concurrency := cfg.Worker.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}

	// Create worker
	workerCfg := worker.Config{
		QueueName:   queueName,
		Exchange:    cfg.Worker.Exchange,
		Concurrency: concurrency,
	}
	w := worker.New(queueAdapter, appLogger, workerCfg)

	// Register job handlers
	w.RegisterHandler(handlers.NewEmailHandler(appLogger))
	w.RegisterHandler(handlers.NewAuditCleanupHandler(pool, appLogger))

	// Start worker
	if err := w.Start(); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	appLogger.Info("Worker is running",
		"queue", queueName,
		"concurrency", concurrency,
	)
	appLogger.Info("Press Ctrl+C to stop.")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := w.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("Worker shutdown error", "error", err)
	}

	appLogger.Info("Worker process stopped")
}
