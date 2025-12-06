package processor

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"image-processor/internal/domain"
	"image-processor/internal/usecase/processor/operations"

	"github.com/wb-go/wbf/zlog"
)

type ImageProcessor struct {
	resizer     *operations.Resizer
	thumbnailer *operations.Thumbnailer
	watermarker *operations.Watermarker
	logger      *zlog.Zerolog
}

func NewImageProcessor(logger *zlog.Zerolog) *ImageProcessor {
	return &ImageProcessor{
		resizer:     operations.NewResizer(),
		thumbnailer: operations.NewThumbnailer(),
		watermarker: operations.NewWatermarker(),
		logger:      logger,
	}
}

func (p *ImageProcessor) Process(ctx context.Context, task *domain.ProcessingTask, originalData []byte) (*domain.ProcessingResult, error) {
	result := &domain.ProcessingResult{
		ID:             task.ID,
		ImageID:        task.ImageID,
		Status:         domain.StatusCompleted,
		ProcessedPaths: make(map[string]string),
		Error:          "",
	}

	img, format, err := image.Decode(bytes.NewReader(originalData))
	if err != nil {
		result.Status = domain.StatusFailed
		result.Error = fmt.Sprintf("Failed to decode image: %v", err)
		p.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to decode image")
		return result, fmt.Errorf("failed to decode image: %w", err)
	}

	targetFormat := string(task.Format)
	if targetFormat == "" {
		targetFormat = format
	}

	p.logger.Info().
		Str("image_id", task.ImageID).
		Str("original_format", format).
		Str("target_format", targetFormat).
		Int("operations", len(task.Operations)).
		Msg("Starting image processing")

	for _, operation := range task.Operations {
		processedPath, err := p.applyOperation(ctx, task, img, targetFormat, operation)
		if err != nil {
			result.Status = domain.StatusFailed
			result.Error = fmt.Sprintf("Operation %s failed: %v", operation.Type, err)
			p.logger.Error().
				Err(err).
				Str("image_id", task.ImageID).
				Str("operation", string(operation.Type)).
				Msg("Operation failed")
			return result, fmt.Errorf("operation %s failed: %w", operation.Type, err)
		}

		result.ProcessedPaths[string(operation.Type)] = processedPath
		p.logger.Debug().
			Str("image_id", task.ImageID).
			Str("operation", string(operation.Type)).
			Str("path", processedPath).
			Msg("Operation completed")
	}

	p.logger.Info().
		Str("image_id", task.ImageID).
		Str("status", string(result.Status)).
		Int("processed_operations", len(result.ProcessedPaths)).
		Msg("Image processing completed")

	return result, nil
}

func (p *ImageProcessor) applyOperation(ctx context.Context, task *domain.ProcessingTask, img image.Image, format string, operation domain.OperationParams) (string, error) {
	var processedData io.Reader
	var processedFormat string
	var err error

	switch operation.Type {
	case domain.OpResize:
		processedData, processedFormat, err = p.resizer.Process(ctx, img, format, operation.Parameters)
	case domain.OpThumbnail:
		processedData, processedFormat, err = p.thumbnailer.Process(ctx, img, format, operation.Parameters)
	case domain.OpWatermark:
		processedData, processedFormat, err = p.watermarker.Process(ctx, img, format, operation.Parameters)
	default:
		return "", fmt.Errorf("unsupported operation type: %s", operation.Type)
	}

	if err != nil {
		return "", fmt.Errorf("failed to process operation %s: %w", operation.Type, err)
	}

	path := p.generatePath(task.ImageID, operation.Type, processedFormat, operation.Parameters)

	data, err := io.ReadAll(processedData)
	if err != nil {
		return "", fmt.Errorf("failed to read processed data: %w", err)
	}

	p.logger.Debug().
		Str("image_id", task.ImageID).
		Str("operation", string(operation.Type)).
		Int("data_size", len(data)).
		Msg("Processed data ready for storage")

	return path, nil
}

func (p *ImageProcessor) generatePath(imageID string, operation domain.OperationType, format string, params map[string]interface{}) string {
	basePath := "processed/"

	switch operation {
	case domain.OpResize:
		width, _ := params["width"].(int)
		height, _ := params["height"].(int)
		return fmt.Sprintf("%sresize/%s/%dx%d.%s", basePath, imageID, width, height, format)
	case domain.OpThumbnail:
		size, _ := params["size"].(int)
		if size == 0 {
			size = domain.DefaultThumbnailSize
		}
		return fmt.Sprintf("%sthumbnails/%s/%d.%s", basePath, imageID, size, format)
	case domain.OpWatermark:
		return fmt.Sprintf("%swatermarked/%s/watermarked.%s", basePath, imageID, format)
	default:
		return fmt.Sprintf("%s%s/%s/processed.%s", basePath, strings.ToLower(string(operation)), imageID, format)
	}
}
