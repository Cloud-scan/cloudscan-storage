# cloudscan-storage

> Multi-cloud storage abstraction service for CloudScan - handles artifact upload/download with presigned URLs

---

## 🎯 Overview

Provides unified API for:
- **S3** (AWS, MinIO, DigitalOcean Spaces)
- **GCS** (Google Cloud Storage)
- **Azure Blob Storage**

**Features:**
- Presigned URL generation (upload/download)
- Multipart upload support for large files
- Artifact metadata tracking in PostgreSQL
- Automatic expiration and cleanup
- Storage backend abstraction

---

## 🏗️ Architecture

```
cloudscan-storage
├── cmd/
│   └── main.go
├── pkg/
│   ├── controller/
│   ├── handlers/
│   │   └── grpc/
│   │       └── storage.go
│   ├── storage/
│   │   ├── s3.go
│   │   ├── gcs.go
│   │   └── azure.go
│   └── persistence/
│       └── artifacts.go
├── proto/                         # Protocol buffers definitions
│   └── storage.proto             # Storage service gRPC API
├── Dockerfile
├── go.mod
└── README.md
```

---

## 🚀 Quick Start

```bash
go run cmd/main.go \
  --storage-type=s3 \
  --s3-bucket=my-bucket \
  --s3-region=us-west-2
```

---

## 📡 API

### gRPC API

The storage service exposes gRPC services defined in `proto/storage.proto`:

**Key RPCs:**
- `CreateArtifact` - Get presigned upload URL
- `GetArtifact` - Get presigned download URL
- `DeleteArtifact` - Remove artifact
- `ListArtifacts` - List artifacts for scan

---

## ⚙️ Configuration

```bash
# S3
export STORAGE_TYPE=s3
export S3_BUCKET=cloudscan-artifacts
export S3_REGION=us-west-2

# GCS
export STORAGE_TYPE=gcs
export GCS_BUCKET=cloudscan-artifacts
export GCS_PROJECT_ID=my-project

# Azure
export STORAGE_TYPE=azure
export AZURE_ACCOUNT_NAME=cloudscan
export AZURE_CONTAINER=artifacts
```

---

## 🚢 Deployment

See [cloudscan-umbrella](https://github.com/cloudscan/cloudscan-umbrella) for complete Helm deployment.

---

## 📄 License

Apache 2.0 - See [LICENSE](../LICENSE)