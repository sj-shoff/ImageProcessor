package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	kafka_impl "image-processor/internal/broker/kafka"
	"image-processor/internal/config"
	"image-processor/internal/domain"
	minio_repo "image-processor/internal/repository/image/cloud/minio"
	postgres_repo "image-processor/internal/repository/image/db/postgres"
	"image-processor/internal/usecase/processor"

	"github.com/segmentio/kafka-go"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/zlog"
)

type Worker struct {
	cfg       *config.Config
	logger    *zlog.Zerolog
	db        *dbpg.DB
	broker    *kafka_impl.KafkaClient
	fileRepo  *minio_repo.FileRepository
	processor *processor.ImageProcessor
	imageRepo *postgres_repo.ImagesRepository
}

func NewWorker(cfg *config.Config, logger *zlog.Zerolog) (*Worker, error) {
	dbOpts := &dbpg.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
	}

	db, err := dbpg.New(cfg.DBDSN(), []string{}, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	fileRepo, err := minio_repo.NewMinIORepository(cfg, cfg.DefaultRetryStrategy(), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create file repository: %w", err)
	}

	brokerClient := kafka_impl.NewKafkaClient(cfg)
	imageProcessor := processor.NewImageProcessor(logger)
	imageRepo := postgres_repo.NewImagesRepository(db, cfg.DefaultRetryStrategy())

	return &Worker{
		cfg:       cfg,
		logger:    logger,
		db:        db,
		broker:    brokerClient,
		fileRepo:  fileRepo,
		processor: imageProcessor,
		imageRepo: imageRepo,
	}, nil
}

func (w *Worker) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages := make(chan kafka.Message, w.cfg.Worker.Concurrency)

	go w.broker.StartConsuming(ctx, messages, w.cfg.DefaultRetryStrategy())

	for i := 0; i < w.cfg.Worker.Concurrency; i++ {
		go w.worker(ctx, i, messages)
	}

	w.logger.Info().Int("concurrency", w.cfg.Worker.Concurrency).Msg("Worker started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	w.logger.Info().Str("signal", sig.String()).Msg("Received signal, shutting down")
	cancel()

	return nil
}

func (w *Worker) worker(ctx context.Context, id int, messages <-chan kafka.Message) {
	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Int("worker_id", id).Msg("Worker stopped")
			return
		case msg := <-messages:
			w.processMessage(ctx, id, msg)
		}
	}
}

func (w *Worker) processMessage(ctx context.Context, workerID int, msg kafka.Message) {
	var task domain.ProcessingTask
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		w.logger.Error().Err(err).Int("worker_id", workerID).Msg("Failed to unmarshal task")
		return
	}

	w.logger.Info().
		Int("worker_id", workerID).
		Str("task_id", task.ID).
		Str("image_id", task.ImageID).
		Msg("Processing task")

	reader, err := w.fileRepo.GetObject(ctx, task.OriginalPath)
	if err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to get original image")
		w.sendFailureResult(ctx, task, fmt.Sprintf("Failed to get original image: %v", err))
		return
	}

	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to read image data")
		w.sendFailureResult(ctx, task, fmt.Sprintf("Failed to read image data: %v", err))
		return
	}

	result, err := w.processor.Process(ctx, &task, data)
	if err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Processing failed")
		w.sendFailureResult(ctx, task, fmt.Sprintf("Processing failed: %v", err))
		return
	}

	if result.Status == domain.StatusCompleted {
		for operation, path := range result.ProcessedPaths {
			mimeType := w.getMimeTypeFromPath(path)

			processed := &domain.ProcessedImage{
				ImageID:    task.ImageID,
				Operation:  domain.OperationType(operation),
				Parameters: "",
				Path:       path,
				Size:       0,
				MimeType:   mimeType,
				Format:     task.Format,
				Status:     "completed",
				CreatedAt:  time.Now(),
			}

			if err := w.imageRepo.SaveProcessedImage(ctx, processed); err != nil {
				w.logger.Error().Err(err).
					Str("image_id", task.ImageID).
					Str("operation", operation).
					Msg("Failed to save processed image info")
			}
		}

		if err := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusCompleted); err != nil {
			w.logger.Error().Err(err).
				Str("image_id", task.ImageID).
				Msg("Failed to update image status")
		}
	} else {
		if err := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusFailed); err != nil {
			w.logger.Error().Err(err).
				Str("image_id", task.ImageID).
				Msg("Failed to update image status to failed")
		}
	}

	if err := w.sendResult(ctx, result); err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to send result")
		return
	}

	if err := w.broker.Commit(ctx, msg); err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to commit message")
	}

	w.logger.Info().
		Int("worker_id", workerID).
		Str("image_id", task.ImageID).
		Str("status", string(result.Status)).
		Msg("Task completed")
}

func (w *Worker) getMimeTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func (w *Worker) sendResult(ctx context.Context, result *domain.ProcessingResult) error {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	return w.broker.Send(ctx, w.cfg.DefaultRetryStrategy(), []byte(result.ImageID), resultBytes)
}

func (w *Worker) sendFailureResult(ctx context.Context, task domain.ProcessingTask, errorMsg string) {
	result := &domain.ProcessingResult{
		ID:      task.ID,
		ImageID: task.ImageID,
		Status:  domain.StatusFailed,
		Error:   errorMsg,
	}

	if err := w.sendResult(ctx, result); err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to send failure result")
	}
}
