package image

import (
	"context"
	"io"

	"image-processor/internal/domain"
)

type imageUsecase interface {
	UploadImage(ctx context.Context, file io.Reader, filename, contentType string, fileSize int64, operations []domain.OperationParams) (*domain.Image, error)
	GetImage(ctx context.Context, id, operation string) (*domain.Image, io.ReadCloser, error)
	GetStatus(ctx context.Context, id string) (domain.ImageStatus, error)
	DeleteImage(ctx context.Context, id string) error
}
