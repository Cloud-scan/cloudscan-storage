package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudscan/cloudscan-storage/internal/config"
	"github.com/cloudscan/cloudscan-storage/internal/domain"
	"github.com/cloudscan/cloudscan-storage/internal/interfaces"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
)

// S3Storage implements StorageBackend using MinIO/S3
type S3Storage struct {
	client *minio.Client
	bucket string
	logger *log.Entry
}

// NewS3Storage creates a new S3/MinIO storage backend
func NewS3Storage(cfg config.S3Config) (interfaces.StorageBackend, error) {
	logger := log.WithField("component", "s3-storage")

	// Determine endpoint - if empty, use AWS S3
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("s3.%s.amazonaws.com", cfg.Region)
	}

	logger.WithFields(log.Fields{
		"endpoint": endpoint,
		"bucket":   cfg.Bucket,
		"region":   cfg.Region,
		"use_ssl":  cfg.UseSSL,
	}).Info("Initializing S3/MinIO storage")

	// Create MinIO client
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Check if bucket exists, create if not
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		logger.WithField("bucket", cfg.Bucket).Info("Bucket does not exist, creating...")
		err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.WithField("bucket", cfg.Bucket).Info("Bucket created successfully")
	}

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
		logger: logger,
	}, nil
}

// GeneratePresignedUploadURL creates a presigned URL for uploading an object
func (s *S3Storage) GeneratePresignedUploadURL(ctx context.Context, path string, contentType string, expiresIn time.Duration) (*domain.PresignedURL, error) {
	logger := s.logger.WithFields(log.Fields{
		"path":         path,
		"content_type": contentType,
		"expires_in":   expiresIn,
	})
	logger.Debug("Generating presigned upload URL")

	// Generate presigned PUT URL
	url, err := s.client.PresignedPutObject(ctx, s.bucket, path, expiresIn)
	if err != nil {
		logger.WithError(err).Error("Failed to generate presigned upload URL")
		return nil, fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}

	headers := make(map[string]string)
	if contentType != "" {
		headers["Content-Type"] = contentType
	}

	presignedURL := &domain.PresignedURL{
		URL:       url.String(),
		Headers:   headers,
		ExpiresAt: time.Now().Add(expiresIn),
	}

	logger.WithField("url", url.String()).Debug("Presigned upload URL generated")
	return presignedURL, nil
}

// GeneratePresignedDownloadURL creates a presigned URL for downloading an object
func (s *S3Storage) GeneratePresignedDownloadURL(ctx context.Context, path string, expiresIn time.Duration) (*domain.PresignedURL, error) {
	logger := s.logger.WithFields(log.Fields{
		"path":       path,
		"expires_in": expiresIn,
	})
	logger.Debug("Generating presigned download URL")

	// Generate presigned GET URL
	url, err := s.client.PresignedGetObject(ctx, s.bucket, path, expiresIn, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to generate presigned download URL")
		return nil, fmt.Errorf("failed to generate presigned download URL: %w", err)
	}

	presignedURL := &domain.PresignedURL{
		URL:       url.String(),
		Headers:   make(map[string]string),
		ExpiresAt: time.Now().Add(expiresIn),
	}

	logger.WithField("url", url.String()).Debug("Presigned download URL generated")
	return presignedURL, nil
}

// DeleteObject deletes an object from storage
func (s *S3Storage) DeleteObject(ctx context.Context, path string) error {
	logger := s.logger.WithField("path", path)
	logger.Debug("Deleting object")

	err := s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{})
	if err != nil {
		logger.WithError(err).Error("Failed to delete object")
		return fmt.Errorf("failed to delete object: %w", err)
	}

	logger.Info("Object deleted successfully")
	return nil
}

// ObjectExists checks if an object exists
func (s *S3Storage) ObjectExists(ctx context.Context, path string) (bool, error) {
	logger := s.logger.WithField("path", path)
	logger.Debug("Checking if object exists")

	_, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		// Check if error is "not found"
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		logger.WithError(err).Error("Failed to check object existence")
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// GetObjectSize returns the size of an object in bytes
func (s *S3Storage) GetObjectSize(ctx context.Context, path string) (int64, error) {
	logger := s.logger.WithField("path", path)
	logger.Debug("Getting object size")

	info, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		logger.WithError(err).Error("Failed to get object size")
		return 0, fmt.Errorf("failed to get object size: %w", err)
	}

	return info.Size, nil
}

// Multipart upload operations

// InitiateMultipartUpload starts a multipart upload session
func (s *S3Storage) InitiateMultipartUpload(ctx context.Context, path string) (string, error) {
	logger := s.logger.WithField("path", path)
	logger.Debug("Initiating multipart upload")

	// TODO: Implement multipart upload initiation using MinIO SDK
	// MinIO SDK doesn't directly expose multipart upload APIs
	// Need to use AWS SDK v2 or implement custom logic

	logger.Warn("Multipart upload not fully implemented for S3/MinIO")
	return "", fmt.Errorf("multipart upload not implemented for S3/MinIO backend")
}

// GetMultipartUploadPartURLs generates presigned URLs for multipart upload parts
func (s *S3Storage) GetMultipartUploadPartURLs(ctx context.Context, path string, uploadID string, fromPart, numParts int, expiresIn time.Duration) ([]*domain.PresignedURL, error) {
	logger := s.logger.WithFields(log.Fields{
		"path":      path,
		"upload_id": uploadID,
		"from_part": fromPart,
		"num_parts": numParts,
	})
	logger.Debug("Generating multipart upload part URLs")

	// TODO: Implement multipart upload part URL generation
	logger.Warn("Multipart upload part URL generation not fully implemented for S3/MinIO")
	return nil, fmt.Errorf("multipart upload not implemented for S3/MinIO backend")
}

// CompleteMultipartUpload finalizes a multipart upload
func (s *S3Storage) CompleteMultipartUpload(ctx context.Context, path string, uploadID string, parts []interfaces.CompletedPart) error {
	logger := s.logger.WithFields(log.Fields{
		"path":       path,
		"upload_id":  uploadID,
		"num_parts":  len(parts),
	})
	logger.Debug("Completing multipart upload")

	// TODO: Implement multipart upload completion
	logger.Warn("Multipart upload completion not fully implemented for S3/MinIO")
	return fmt.Errorf("multipart upload not implemented for S3/MinIO backend")
}

// AbortMultipartUpload cancels a multipart upload
func (s *S3Storage) AbortMultipartUpload(ctx context.Context, path string, uploadID string) error {
	logger := s.logger.WithFields(log.Fields{
		"path":      path,
		"upload_id": uploadID,
	})
	logger.Debug("Aborting multipart upload")

	// TODO: Implement multipart upload abort
	logger.Warn("Multipart upload abort not fully implemented for S3/MinIO")
	return fmt.Errorf("multipart upload not implemented for S3/MinIO backend")
}