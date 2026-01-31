package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudscan/cloudscan-storage/internal/config"
	"github.com/cloudscan/cloudscan-storage/internal/database"
	grpcserver "github.com/cloudscan/cloudscan-storage/internal/grpc"
	"github.com/cloudscan/cloudscan-storage/internal/interfaces"
	"github.com/cloudscan/cloudscan-storage/internal/storage"
	log "github.com/sirupsen/logrus"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Initialize logger
	initLogger()

	log.WithFields(log.Fields{
		"version":   version,
		"commit":    commit,
		"buildDate": buildDate,
	}).Info("Starting cloudscan-storage")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Initialize database connection
	db, err := database.NewPostgresDB(
		cfg.Database.DSN(),
		cfg.Database.MaxConnections,
		cfg.Database.MinConnections,
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	log.Info("Database connection established")

	// Run migrations
	if err := database.RunMigrations(db.DB, cfg.Database.MigrationsPath); err != nil {
		log.WithError(err).Fatal("Failed to run database migrations")
	}

	// Initialize artifact repository
	artifactRepo := database.NewArtifactRepository(db)

	// Initialize storage backend based on type
	var storageBackend interfaces.StorageBackend
	switch cfg.Storage.Type {
	case config.StorageTypeS3, config.StorageTypeMinIO:
		storageBackend, err = storage.NewS3Storage(cfg.Storage.S3Config)
		if err != nil {
			log.WithError(err).Fatal("Failed to initialize S3/MinIO storage")
		}
	case config.StorageTypeGCS:
		storageBackend, err = storage.NewGCSStorage(cfg.Storage.GCSConfig)
		if err != nil {
			log.WithError(err).Fatal("Failed to initialize GCS storage")
		}
		log.Warn("GCS storage backend is not fully implemented yet - operations will fail")
	case config.StorageTypeAzure:
		storageBackend, err = storage.NewAzureStorage(cfg.Storage.AzureConfig)
		if err != nil {
			log.WithError(err).Fatal("Failed to initialize Azure Blob storage")
		}
		log.Warn("Azure Blob storage backend is not fully implemented yet - operations will fail")
	default:
		log.Fatalf("Unsupported storage type: %s", cfg.Storage.Type)
	}

	log.WithField("type", cfg.Storage.Type).Info("Storage backend initialized")

	// Initialize gRPC service
	storageService := grpcserver.NewStorageServiceServer(
		storageBackend,
		artifactRepo,
		cfg.Storage.DefaultExpiration,
	)

	// Initialize gRPC server
	grpcSrv := grpcserver.NewServer(cfg.Server.GRPCPort, storageService)

	// Initialize HTTP server for health checks
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.HTTPPort),
		Handler:      setupHTTPHandlers(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server
	go func() {
		log.WithField("port", cfg.Server.HTTPPort).Info("Starting HTTP server")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Start gRPC server
	go func() {
		if err := grpcSrv.Start(); err != nil {
			log.WithError(err).Fatal("gRPC server failed")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutdown signal received, gracefully stopping...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop HTTP server
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("HTTP server shutdown error")
	}

	// Stop gRPC server
	grpcSrv.Stop()

	log.Info("Server stopped")
}

// initLogger configures the logger
func initLogger() {
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}

	logLevel, err := log.ParseLevel(level)
	if err != nil {
		logLevel = log.InfoLevel
	}

	log.SetLevel(logLevel)
	log.SetOutput(os.Stdout)
}

// setupHTTPHandlers configures HTTP routes for health and metrics
func setupHTTPHandlers() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"storage","version":"%s","commit":"%s","buildDate":"%s"}`, version, commit, buildDate)
	})

	// Readiness check endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Check storage backend and database connectivity
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "# HELP cloudscan_storage_info Build info\n")
		fmt.Fprintf(w, "# TYPE cloudscan_storage_info gauge\n")
		fmt.Fprintf(w, "cloudscan_storage_info{version=\"%s\",commit=\"%s\",buildDate=\"%s\"} 1\n", version, commit, buildDate)
	})

	return mux
}