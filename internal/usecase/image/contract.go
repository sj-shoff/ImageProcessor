package image

import (
	"context"
	"io"

	"image-processor/internal/domain"

	"github.com/wb-go/wbf/retry"
)

type imageRepository interface {
	Save(ctx context.Context, image *domain.Image) error
	GetByID(ctx context.Context, id string) (*domain.Image, error)
	UpdateStatus(ctx context.Context, id string, status domain.ImageStatus) error
	Update(ctx context.Context, img *domain.Image) error
	Delete(ctx context.Context, id string) error
	SaveProcessedImage(ctx context.Context, processed *domain.ProcessedImage) error
	GetProcessedImages(ctx context.Context, imageID string) ([]domain.ProcessedImage, error)
	GetProcessedImageByOperation(ctx context.Context, imageID, operation string) (*domain.ProcessedImage, error)
	DeleteProcessedImages(ctx context.Context, imageID string) error
	List(ctx context.Context, limit, offset int) ([]domain.Image, error)
	Count(ctx context.Context) (int, error)
}

type fileRepository interface {
	SaveOriginal(ctx context.Context, filename string, data io.Reader, size int64) (string, error)
	GetObject(ctx context.Context, path string) (io.ReadCloser, error)
	SaveProcessed(ctx context.Context, path string, data io.Reader, size int64, contentType string) error
	DeleteObject(ctx context.Context, path string) error
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}

type imageProducer interface {
	SendProcessingTask(ctx context.Context, strategy retry.Strategy, key, value []byte) error
}
