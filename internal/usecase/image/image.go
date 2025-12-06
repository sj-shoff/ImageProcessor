package image

import (
	"context"
	"image-processor/internal/domain"
	"io"

	"github.com/wb-go/wbf/retry"
	"github.com/wb-go/wbf/zlog"
)

type ImageUsecase struct {
	repo     imageRepository
	fileRepo fileRepository
	producer imageProducer
	logger   *zlog.Zerolog
	retries  retry.Strategy
}

func NewImageUsecase(repo imageRepository, fileRepo fileRepository, producer imageProducer, logger *zlog.Zerolog, retries retry.Strategy) *ImageUsecase {
	return &ImageUsecase{
		repo:     repo,
		fileRepo: fileRepo,
		producer: producer,
		logger:   logger,
		retries:  retries,
	}
}

func (i *ImageUsecase) UploadImage(ctx context.Context, file io.Reader, filename, contentType string, fileSize int64, operations []domain.OperationParams) (*domain.Image, error) {

}

func (i *ImageUsecase) GetImage(ctx context.Context, id, operation string) (*domain.Image, io.ReadCloser, error) {

}

func (i *ImageUsecase) GetStatus(ctx context.Context, id string) (domain.ImageStatus, error) {

}

func (i *ImageUsecase) DeleteImage(ctx context.Context, id string) error {

}
