package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)

	log.WithFields(log.Fields{
		"version":   version,
		"commit":    commit,
		"buildDate": buildDate,
	}).Info("Starting CloudScan Storage Service")

	// Get configuration from environment
	port := getEnv("PORT", "8082")
	storagePath := getEnv("STORAGE_PATH", "/app/storage")
	storageType := getEnv("STORAGE_TYPE", "local")

	log.WithFields(log.Fields{
		"port":        port,
		"storagePath": storagePath,
		"storageType": storageType,
	}).Info("Storage service configuration loaded")

	// Ensure storage directory exists
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	// Start HTTP server
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Health check endpoints
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":      "healthy",
			"service":     "storage",
			"version":     version,
			"commit":      commit,
			"buildDate":   buildDate,
			"timestamp":   time.Now().UTC(),
			"storageType": storageType,
		})
	})

	e.GET("/ready", func(c echo.Context) error {
		// Check if storage path is accessible
		if _, err := os.Stat(storagePath); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
				"status": "not ready",
				"error":  "storage path not accessible",
			})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status": "ready",
		})
	})

	// Storage API endpoints (placeholder)
	api := e.Group("/api/v1")

	api.POST("/upload", func(c echo.Context) error {
		// TODO: Implement file upload
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Upload endpoint - to be implemented",
		})
	})

	api.GET("/download/:id", func(c echo.Context) error {
		// TODO: Implement file download
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Download endpoint - to be implemented",
		})
	})

	api.DELETE("/delete/:id", func(c echo.Context) error {
		// TODO: Implement file deletion
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Delete endpoint - to be implemented",
		})
	})

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		log.Info("Shutting down storage service...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			log.Errorf("Error during shutdown: %v", err)
		}
	}()

	addr := ":" + port
	log.Infof("Storage service listening on %s", addr)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func init() {
	// Set up logging
	logLevel := getEnv("LOG_LEVEL", "info")
	switch logLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	log.Infof("Log level set to: %s", logLevel)
}