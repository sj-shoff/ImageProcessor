package image

import (
	"context"
	"image-processor/internal/domain"
	"io"
)

type imageRepository interface {
	Save(ctx context.Context, image *domain.Image) error
	GetByID(ctx context.Context, id string) (*domain.Image, error)
	UpdateStatus(ctx context.Context, id string, status domain.ImageStatus) error
	Delete(ctx context.Context, id string) error
	SaveProcessedImage(ctx context.Context, processed *domain.ProcessedImage) error
	GetProcessedImages(ctx context.Context, imageID string) ([]domain.ProcessedImage, error)
	DeleteProcessedImages(ctx context.Context, imageID string) error
}

type fileRepository interface {
	SaveOriginal(ctx context.Context, filename string, data io.Reader, size int64) (string, error)
	GetObject(ctx context.Context, path string) (io.ReadCloser, error)
	SaveProcessed(ctx context.Context, path string, data io.Reader, size int64, contentType string) error
	DeleteObject(ctx context.Context, path string) error
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}

type imageProducer interface {
	Send(ctx context.Context, task *domain.ProcessingTask) error
}
