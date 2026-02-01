--liquibase formatted sql

--changeset cloudscan:1 labels:v1.0.0 context:schema
--comment: CloudScan Storage - Initial Schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- Artifacts
-- =============================================================================
CREATE TABLE IF NOT EXISTS artifacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scan_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('source', 'result', 'log', 'report')),
    filename VARCHAR(500) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    content_type VARCHAR(100),
    storage_path VARCHAR(1000) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_artifacts_scan_id ON artifacts(scan_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_org_id ON artifacts(organization_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_type ON artifacts(type);
CREATE INDEX IF NOT EXISTS idx_artifacts_expires_at ON artifacts(expires_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_artifacts_deleted_at ON artifacts(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_artifacts_storage_path ON artifacts(storage_path);

-- =============================================================================
-- Comments
-- =============================================================================

COMMENT ON TABLE artifacts IS 'Stored artifacts (source code, scan results, logs, reports)';
COMMENT ON COLUMN artifacts.type IS 'Artifact type: source, result, log, report';
COMMENT ON COLUMN artifacts.storage_path IS 'Path in object storage (S3/MinIO)';
COMMENT ON COLUMN artifacts.expires_at IS 'Expiration time for automatic cleanup';

--rollback DROP TABLE IF EXISTS artifacts CASCADE;
--rollback DROP EXTENSION IF EXISTS "uuid-ossp";
