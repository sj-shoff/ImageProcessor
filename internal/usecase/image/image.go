package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"image-processor/internal/domain"

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
	i.logger.Info().Str("filename", filename).Int64("size", fileSize).Msg("Starting image upload")
	if fileSize > domain.DefaultMaxUploadSize {
		i.logger.Warn().Str("filename", filename).Int64("size", fileSize).Msg("File too large")
		return nil, fmt.Errorf("%w: max size is %d bytes", ErrFileTooLarge, domain.DefaultMaxUploadSize)
	}
	buf := make([]byte, 512)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		i.logger.Error().Err(err).Str("filename", filename).Msg("Failed to read file header")
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	detectedType := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(detectedType, "image/") {
		i.logger.Warn().Str("filename", filename).Str("detected_type", detectedType).Msg("Invalid file signature")
		return nil, fmt.Errorf("%w: file is not an image", ErrInvalidFileFormat)
	}
	combinedReader := io.MultiReader(bytes.NewReader(buf[:n]), file)
	imageID := uuid.New().String()
	originalPath := ""
	defer func() {
		if err != nil && originalPath != "" {
			if delErr := i.fileRepo.DeleteObject(ctx, originalPath); delErr != nil {
				i.logger.Error().Err(delErr).Str("path", originalPath).Msg("Failed to cleanup original file after error")
			}
		}
	}()
	originalPath, err = i.fileRepo.SaveOriginal(ctx, filename, combinedReader, fileSize)
	if err != nil {
		i.logger.Error().Err(err).Str("filename", filename).Msg("Failed to save original image")
		return nil, fmt.Errorf("%w: %v", ErrStorageError, err)
	}
	img := &domain.Image{
		ID:               imageID,
		OriginalFilename: filename,
		OriginalSize:     fileSize,
		MimeType:         detectedType,
		Status:           domain.StatusUploaded,
		OriginalPath:     originalPath,
		Bucket:           "images",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err = i.repo.Save(ctx, img); err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to save image metadata")
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	task := &domain.ProcessingTask{
		ID:           uuid.New().String(),
		ImageID:      imageID,
		OriginalPath: originalPath,
		Bucket:       "images",
		Operations:   operations,
		Format:       getFormatFromContentType(detectedType),
	}
	taskBytes, err := json.Marshal(task)
	if err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to marshal task")
		return nil, fmt.Errorf("%w: %v", ErrMessageQueueError, err)
	}
	if err = i.producer.SendTask(ctx, i.retries, []byte(imageID), taskBytes); err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to send task to broker")
		if updateErr := i.repo.UpdateStatus(ctx, imageID, domain.StatusFailed); updateErr != nil {
			i.logger.Error().Err(updateErr).Str("image_id", imageID).Msg("Failed to update status to failed")
		}
		return nil, fmt.Errorf("%w: %v", ErrMessageQueueError, err)
	}
	if err = i.repo.UpdateStatus(ctx, imageID, domain.StatusProcessing); err != nil {
		i.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to update status to processing")
		img.Status = domain.StatusUploaded
	} else {
		img.Status = domain.StatusProcessing
	}
	i.logger.Info().Str("image_id", imageID).Str("filename", filename).Msg("Image uploaded and queued for processing")
	return img, nil
}

func (i *ImageUsecase) GetImage(ctx context.Context, id, operation string) (*domain.Image, io.ReadCloser, error) {
	i.logger.Debug().Str("image_id", id).Str("operation", operation).Msg("Getting image")
	img, err := i.repo.GetByID(ctx, id)
	if err != nil {
		if err == ErrImageNotFound {
			i.logger.Info().Str("image_id", id).Msg("Image not found")
			return nil, nil, ErrImageNotFound
		}
		i.logger.Error().Err(err).Str("image_id", id).Msg("Failed to get image from DB")
		return nil, nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	if operation == "" {
		reader, err := i.fileRepo.GetObject(ctx, img.OriginalPath)
		if err != nil {
			i.logger.Error().Err(err).Str("image_id", id).Str("path", img.OriginalPath).Msg("Failed to get original image from storage")
			return nil, nil, fmt.Errorf("%w: %v", ErrStorageError, err)
		}
		return img, reader, nil
	}
	processed, err := i.repo.GetProcessedImageByOperation(ctx, id, operation)
	if err != nil {
		i.logger.Error().Err(err).Str("image_id", id).Str("operation", operation).Msg("Failed to get processed image from DB")
		return nil, nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	if processed == nil {
		i.logger.Info().Str("image_id", id).Str("operation", operation).Msg("Processed image not found")
		return nil, nil, ErrProcessedImageNotFound
	}
	reader, err := i.fileRepo.GetObject(ctx, processed.Path)
	if err != nil {
		i.logger.Error().Err(err).Str("image_id", id).Str("path", processed.Path).Msg("Failed to get processed image from storage")
		return nil, nil, fmt.Errorf("%w: %v", ErrStorageError, err)
	}
	return img, reader, nil
}

func (i *ImageUsecase) GetStatus(ctx context.Context, id string) (domain.ImageStatus, error) {
	i.logger.Debug().Str("image_id", id).Msg("Getting image status")
	img, err := i.repo.GetByID(ctx, id)
	if err != nil {
		if err == ErrImageNotFound {
			i.logger.Info().Str("image_id", id).Msg("Image not found when getting status")
			return "", ErrImageNotFound
		}
		i.logger.Error().Err(err).Str("image_id", id).Msg("Failed to get image status from DB")
		return "", fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	return img.Status, nil
}

func (i *ImageUsecase) DeleteImage(ctx context.Context, id string) error {
	i.logger.Info().Str("image_id", id).Msg("Deleting image")
	img, err := i.repo.GetByID(ctx, id)
	if err != nil {
		if err == ErrImageNotFound {
			i.logger.Info().Str("image_id", id).Msg("Image not found for deletion")
			return ErrImageNotFound
		}
		i.logger.Error().Err(err).Str("image_id", id).Msg("Failed to get image for deletion")
		return fmt.Errorf("%w: %v", ErrDatabaseError, err)
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
		i.logger.Error().Err(err).Str("image_id", id).Msg("Failed to update image status to deleted")
		return fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	i.logger.Info().Str("image_id", id).Msg("Image deleted successfully")
	return nil
}

func (i *ImageUsecase) ListImages(ctx context.Context, limit, offset int) ([]domain.Image, error) {
	return i.repo.List(ctx, limit, offset)
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
	case strings.Contains(contentType, "bmp"):
		return domain.FormatBMP
	case strings.Contains(contentType, "tiff"):
		return domain.FormatTIFF
	default:
		return domain.FormatJPEG
	}
}
