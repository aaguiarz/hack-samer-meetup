package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mapping-engine/internal/config"
	"mapping-engine/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.LoadServiceConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start the webhook service
	svc, err := service.NewWebhookService(cfg)
	if err != nil {
		log.Fatalf("Failed to create webhook service: %v", err)
	}

	// Start the service in a goroutine
	go func() {
		if err := svc.Start(); err != nil {
			log.Fatalf("Failed to start webhook service: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down webhook service...")

	// Give the service 30 seconds to shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svc.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown webhook service: %v", err)
	}

	log.Println("Webhook service stopped")
}
