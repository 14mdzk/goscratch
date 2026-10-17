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
	notificationadapter "github.com/14mdzk/goscratch/internal/adapter/notification"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/platform/observability"
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

	// Initialize tracing so worker-side job spans export. When disabled the
	// global provider stays a no-op and handleMessage spans are dropped.
	if cfg.Observability.Tracing.Enabled {
		appLogger.Info("Initializing OpenTelemetry tracing...",
			"endpoint", cfg.Observability.Tracing.Endpoint)
		shutdown, err := observability.InitTracer(ctx, observability.TracerConfig{
			ServiceName:    cfg.App.Name,
			ServiceVersion: "1.0.0",
			Environment:    cfg.App.Env,
			Endpoint:       cfg.Observability.Tracing.Endpoint,
			Enabled:        true,
		})
		if err != nil {
			appLogger.Warn("Failed to initialize tracing", "error", err)
		} else {
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdown(shutdownCtx); err != nil {
					appLogger.Warn("Tracing shutdown error", "error", err)
				}
			}()
			appLogger.Info("Tracing initialized successfully")
		}
	}

	// Validate queue configuration before creating connections.
	// cmd/worker is the external consumer: it only runs the rabbitmq
	// backend. The goroutine tier is consumed in-process by the API
	// (worker.in_process=true); redis is reserved.
	backend, err := cfg.ResolveQueueBackend()
	if err != nil {
		return err
	}
	if backend != config.QueueBackendRabbitMQ {
		return fmt.Errorf("cmd/worker only runs the rabbitmq backend (resolved queue.backend=%q): use worker.in_process=true with the goroutine backend, or set queue.backend=rabbitmq with a reachable broker", backend)
	}

	// Validate notification configuration before accepting jobs
	if err := cfg.Notification.Validate(); err != nil {
		return err
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
	queueAdapter, err := queue.NewRabbitMQWithOptions(cfg.RabbitMQ.URL, queue.Options{
		PrefetchCount: cfg.RabbitMQ.PrefetchCount,
	})
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

	// Initialize notification sender
	var notificationSender port.NotificationSender
	if cfg.Notification.Enabled {
		appLogger.Info("Initializing webhook notification sender...")
		notificationSender = notificationadapter.NewWebhookSender(notificationadapter.WebhookConfig{
			URL: cfg.Notification.WebhookURL,
		})
	} else {
		notificationSender = notificationadapter.NewNoOpSender(appLogger)
	}
	defer notificationSender.Close()

	// Register job handlers
	w.RegisterHandler(handlers.NewEmailHandler(appLogger, emailSender))
	w.RegisterHandler(handlers.NewNotificationHandler(appLogger, notificationSender))
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
