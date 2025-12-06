package minio

import (
	"image-processor/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/wb-go/wbf/retry"
)

type FileRepository struct {
	client  *minio.Client
	cfg     *config.Config
	retries *retry.Strategy
}

func NewMinIORepository(client *minio.Client, cfg *config.Config, retries *retry.Strategy) *FileRepository {
	return &FileRepository{
		client: client,
		cfg:    cfg,
	}
}
