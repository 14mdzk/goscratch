package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/14mdzk/goscratch/internal/platform/app"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/pkg/types"
)

func main() {
	// Register custom Fiber decoders for Opt types
	types.RegisterFiberDecoders()
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context
	ctx := context.Background()

	// Initialize application
	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start server in goroutine
	go func() {
		if err := application.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shutdown application: %v", err)
	}
}
