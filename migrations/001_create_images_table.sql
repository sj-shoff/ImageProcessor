-- +goose Up
CREATE TABLE IF NOT EXISTS images (
    id VARCHAR(36) PRIMARY KEY,
    original_filename VARCHAR(255) NOT NULL,
    original_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'uploaded',
    original_path VARCHAR(500) NOT NULL,
    bucket VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS processed_images (
    id VARCHAR(36) PRIMARY KEY,
    image_id VARCHAR(36) NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    operation VARCHAR(50) NOT NULL,
    parameters TEXT,
    path VARCHAR(500) NOT NULL,
    size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    format VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'processing',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_images_status ON images(status);
CREATE INDEX idx_processed_images_image_id ON processed_images(image_id);
CREATE INDEX idx_processed_images_operation ON processed_images(operation);

-- +goose Down
DROP TABLE IF EXISTS processed_images;
DROP TABLE IF EXISTS images;