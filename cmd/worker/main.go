package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	emailadapter "github.com/14mdzk/goscratch/internal/adapter/email"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/internal/worker/handlers"
	"github.com/14mdzk/goscratch/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.default.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
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

	// Validate queue configuration before creating connections
	if !cfg.RabbitMQ.Enabled {
		return fmt.Errorf("RabbitMQ must be enabled to run the worker. Set rabbitmq.enabled=true in config")
	}

	// Initialize database connection
	appLogger.Info("Connecting to database...")
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()
	appLogger.Info("Database connected successfully")

	appLogger.Info("Connecting to RabbitMQ...")
	queueAdapter, err := queue.NewRabbitMQ(cfg.RabbitMQ.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
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

	// Initialize email sender
	var emailSender port.EmailSender
	if cfg.Email.Enabled {
		appLogger.Info("Initializing SMTP email sender...")
		emailSender = emailadapter.NewSMTPSender(emailadapter.SMTPConfig{
			Host:     cfg.Email.Host,
			Port:     cfg.Email.Port,
			Username: cfg.Email.Username,
			Password: cfg.Email.Password,
			From:     cfg.Email.From,
		})
	} else {
		emailSender = emailadapter.NewNoOpSender(appLogger)
	}
	defer emailSender.Close()

	// Register job handlers
	w.RegisterHandler(handlers.NewEmailHandler(appLogger, emailSender))
	w.RegisterHandler(handlers.NewAuditCleanupHandler(pool, appLogger))

	// Start worker
	if err := w.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
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
	return nil
}
