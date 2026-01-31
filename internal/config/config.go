package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the storage service
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Storage  StorageConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	GRPCPort string
	HTTPPort string
	LogLevel string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Database       string
	SSLMode        string
	MaxConnections int
	MinConnections int
	MigrationsPath string
}

// StorageConfig holds storage backend configuration
type StorageConfig struct {
	Type              StorageType
	S3Config          S3Config
	GCSConfig         GCSConfig
	AzureConfig       AzureConfig
	DefaultExpiration int // hours
}

// StorageType represents the storage backend type
type StorageType string

const (
	StorageTypeS3    StorageType = "s3"
	StorageTypeMinIO StorageType = "minio"
	StorageTypeGCS   StorageType = "gcs"
	StorageTypeAzure StorageType = "azure"
)

// S3Config holds S3-specific configuration
type S3Config struct {
	Endpoint        string // For MinIO compatibility
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Bucket          string
	UseSSL          bool
}

// GCSConfig holds Google Cloud Storage configuration
type GCSConfig struct {
	ProjectID           string
	Bucket              string
	CredentialsFilePath string
}

// AzureConfig holds Azure Blob Storage configuration
type AzureConfig struct {
	AccountName string
	AccountKey  string
	Container   string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			GRPCPort: getEnv("GRPC_PORT", "8082"),
			HTTPPort: getEnv("HTTP_PORT", "8083"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           getEnvAsInt("DB_PORT", 5432),
			User:           getEnv("DB_USER", "cloudscan"),
			Password:       getEnv("DB_PASSWORD", "changeme"),
			Database:       getEnv("DB_NAME", "storage"),
			SSLMode:        getEnv("DB_SSLMODE", "prefer"),
			MaxConnections: getEnvAsInt("DB_MAX_CONNECTIONS", 25),
			MinConnections: getEnvAsInt("DB_MIN_CONNECTIONS", 5),
			MigrationsPath: getEnv("DB_MIGRATIONS_PATH", "file://migrations"),
		},
		Storage: StorageConfig{
			Type:              StorageType(getEnv("STORAGE_TYPE", "minio")),
			DefaultExpiration: getEnvAsInt("DEFAULT_EXPIRATION_HOURS", 24),
		},
	}

	// Load storage-specific configuration based on type
	switch cfg.Storage.Type {
	case StorageTypeS3, StorageTypeMinIO:
		cfg.Storage.S3Config = S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", ""),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
			Region:          getEnv("S3_REGION", "us-east-1"),
			Bucket:          getEnv("S3_BUCKET", "cloudscan-artifacts"),
			UseSSL:          getEnvAsBool("S3_USE_SSL", true),
		}

		if cfg.Storage.S3Config.AccessKeyID == "" {
			return nil, fmt.Errorf("S3_ACCESS_KEY_ID is required for S3/MinIO storage")
		}
		if cfg.Storage.S3Config.SecretAccessKey == "" {
			return nil, fmt.Errorf("S3_SECRET_ACCESS_KEY is required for S3/MinIO storage")
		}

	case StorageTypeGCS:
		cfg.Storage.GCSConfig = GCSConfig{
			ProjectID:           getEnv("GCS_PROJECT_ID", ""),
			Bucket:              getEnv("GCS_BUCKET", "cloudscan-artifacts"),
			CredentialsFilePath: getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
		}

		if cfg.Storage.GCSConfig.ProjectID == "" {
			return nil, fmt.Errorf("GCS_PROJECT_ID is required for GCS storage")
		}

	case StorageTypeAzure:
		cfg.Storage.AzureConfig = AzureConfig{
			AccountName: getEnv("AZURE_ACCOUNT_NAME", ""),
			AccountKey:  getEnv("AZURE_ACCOUNT_KEY", ""),
			Container:   getEnv("AZURE_CONTAINER", "cloudscan-artifacts"),
		}

		if cfg.Storage.AzureConfig.AccountName == "" {
			return nil, fmt.Errorf("AZURE_ACCOUNT_NAME is required for Azure storage")
		}
		if cfg.Storage.AzureConfig.AccountKey == "" {
			return nil, fmt.Errorf("AZURE_ACCOUNT_KEY is required for Azure storage")
		}

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}