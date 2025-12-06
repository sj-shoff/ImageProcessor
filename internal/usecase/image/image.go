package image

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"image-processor/internal/domain"
	repoImage "image-processor/internal/repository/image"

	"github.com/google/uuid"
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
	imageID := uuid.New().String()

	originalPath, err := i.fileRepo.SaveOriginal(ctx, filename, file, fileSize)
	if err != nil {
		i.logger.Error().Err(err).Str("filename", filename).Msg("Failed to save original image")
		return nil, fmt.Errorf("failed to save image: %w", err)
	}

	img := &domain.Image{
		ID:               imageID,
		OriginalFilename: filename,
		OriginalSize:     fileSize,
		MimeType:         contentType,
		Status:           domain.StatusUploaded,
		OriginalPath:     originalPath,
		Bucket:           "images",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := i.repo.Save(ctx, img); err != nil {
		i.fileRepo.DeleteObject(ctx, originalPath)
		return nil, fmt.Errorf("failed to save image metadata: %w", err)
	}

	task := &domain.ProcessingTask{
		ID:           uuid.New().String(),
		ImageID:      imageID,
		OriginalPath: originalPath,
		Bucket:       "images",
		Operations:   operations,
		Format:       getFormatFromContentType(contentType),
	}

	if err := i.producer.Send(ctx, task); err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to send task to Kafka")
		i.updateStatus(ctx, imageID, domain.StatusFailed)
		return nil, fmt.Errorf("failed to send processing task: %w", err)
	}

	if err := i.repo.UpdateStatus(ctx, imageID, domain.StatusProcessing); err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to update status")
		img.Status = domain.StatusUploaded
	} else {
		img.Status = domain.StatusProcessing
	}

	i.logger.Info().Str("image_id", imageID).Str("filename", filename).Msg("Image uploaded and queued for processing")
	return img, nil
}

func (i *ImageUsecase) GetImage(ctx context.Context, id, operation string) (*domain.Image, io.ReadCloser, error) {
	img, err := i.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get image: %w", err)
	}

	if operation == "" {
		reader, err := i.fileRepo.GetObject(ctx, img.OriginalPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get original image: %w", err)
		}
		return img, reader, nil
	}

	processed, err := i.repo.GetProcessedImageByOperation(ctx, id, operation)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get processed image: %w", err)
	}

	if processed == nil {
		return nil, nil, repoImage.ErrProcessedImageNotFound
	}

	reader, err := i.fileRepo.GetObject(ctx, processed.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get processed image file: %w", err)
	}

	return img, reader, nil
}

func (i *ImageUsecase) GetStatus(ctx context.Context, id string) (domain.ImageStatus, error) {
	img, err := i.repo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get image status: %w", err)
	}
	return img.Status, nil
}

func (i *ImageUsecase) DeleteImage(ctx context.Context, id string) error {
	img, err := i.repo.GetByID(ctx, id)
	if err != nil {
		if err == repoImage.ErrImageNotFound {
			return repoImage.ErrImageNotFound
		}
		return fmt.Errorf("failed to get image for deletion: %w", err)
	}

	if err := i.fileRepo.DeleteObject(ctx, img.OriginalPath); err != nil {
		i.logger.Error().Err(err).Str("path", img.OriginalPath).Msg("Failed to delete original file")
	}

	processedPrefix := fmt.Sprintf("processed/%s/", id)
	if err := i.fileRepo.DeleteObjectsWithPrefix(ctx, processedPrefix); err != nil {
		i.logger.Error().Err(err).Str("prefix", processedPrefix).Msg("Failed to delete processed files")
	}

	if err := i.repo.DeleteProcessedImages(ctx, id); err != nil {
		i.logger.Error().Err(err).Str("image_id", id).Msg("Failed to delete processed images from DB")
	}

	if err := i.repo.UpdateStatus(ctx, id, domain.StatusDeleted); err != nil {
		return fmt.Errorf("failed to update image status to deleted: %w", err)
	}

	i.logger.Info().Str("image_id", id).Msg("Image deleted successfully")
	return nil
}

func (i *ImageUsecase) updateStatus(ctx context.Context, imageID string, status domain.ImageStatus) {
	if err := i.repo.UpdateStatus(ctx, imageID, status); err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Str("status", string(status)).Msg("Failed to update status")
	}
}

func getFormatFromContentType(contentType string) domain.ImageFormat {
	switch {
	case strings.Contains(contentType, "jpeg"):
		return domain.FormatJPEG
	case strings.Contains(contentType, "png"):
		return domain.FormatPNG
	case strings.Contains(contentType, "gif"):
		return domain.FormatGIF
	case strings.Contains(contentType, "webp"):
		return domain.FormatWebP
	default:
		return domain.FormatJPEG
	}
}

func (i *ImageUsecase) SaveProcessingResult(ctx context.Context, result *domain.ProcessingResult) error {
	if err := i.repo.UpdateStatus(ctx, result.ImageID, result.Status); err != nil {
		return fmt.Errorf("failed to update image status: %w", err)
	}

	for operation, path := range result.ProcessedPaths {
		processed := &domain.ProcessedImage{
			ImageID:   result.ImageID,
			Operation: domain.OperationType(operation),
			Path:      path,
			Size:      0,
			MimeType:  "image/jpeg",
			Format:    domain.FormatJPEG,
			Status:    "completed",
			CreatedAt: time.Now(),
		}

		if err := i.repo.SaveProcessedImage(ctx, processed); err != nil {
			i.logger.Error().Err(err).Str("image_id", result.ImageID).Str("operation", operation).Msg("Failed to save processed image info")
		}
	}

	i.logger.Info().Str("image_id", result.ImageID).Str("status", string(result.Status)).Msg("Processing result saved")
	return nil
}
