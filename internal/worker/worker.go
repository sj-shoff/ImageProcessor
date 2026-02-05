package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"image-processor/internal/broker"
	kafka_impl "image-processor/internal/broker/kafka"
	"image-processor/internal/config"
	"image-processor/internal/domain"
	minio_repo "image-processor/internal/repository/image/cloud/minio"
	postgres_repo "image-processor/internal/repository/image/db/postgres"
	"image-processor/internal/usecase/processor"

	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/zlog"
)

type Worker struct {
	cfg         *config.Config
	logger      *zlog.Zerolog
	db          *dbpg.DB
	consumer    broker.Consumer
	processor   *processor.ImageProcessor
	imageRepo   *postgres_repo.ImagesRepository
	fileRepo    *minio_repo.FileRepository
	concurrency int
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

func NewWorker(cfg *config.Config, logger *zlog.Zerolog) (*Worker, error) {
	retries := cfg.DefaultRetryStrategy()
	dbOpts := &dbpg.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
	}
	db, err := dbpg.New(cfg.DBDSN(), []string{}, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	fileRepo, err := minio_repo.NewMinIORepository(cfg, retries)
	if err != nil {
		return nil, fmt.Errorf("failed to create file repository: %w", err)
	}
	imageRepo := postgres_repo.NewImagesRepository(db, retries)
	consumer := kafka_impl.NewConsumerClient(cfg)
	processor := processor.NewImageProcessor(fileRepo, logger)
	concurrency := cfg.Worker.Concurrency
	logger.Info().
		Strs("brokers", cfg.Kafka.Brokers).
		Str("topic", cfg.Kafka.ProcessingTopic).
		Str("group", cfg.Kafka.GroupID).
		Int("concurrency", concurrency).
		Msg("Worker configuration")
	return &Worker{
		cfg:         cfg,
		logger:      logger,
		db:          db,
		consumer:    consumer,
		processor:   processor,
		imageRepo:   imageRepo,
		fileRepo:    fileRepo,
		concurrency: concurrency,
	}, nil
}

func (w *Worker) Run() error {
	w.logger.Info().Int("concurrency", w.concurrency).Msg("Starting worker")
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		w.logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal, stopping worker...")
		cancel()
	}()
	messages := make(chan *broker.Message, w.concurrency*2)
	w.consumer.Start(ctx, messages, w.cfg.DefaultRetryStrategy())
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go func(id int) {
			defer w.wg.Done()
			w.processWorker(ctx, id, messages)
		}(i)
	}
	w.logger.Info().Msg("Worker started successfully")
	<-ctx.Done()
	w.logger.Info().Msg("Shutting down worker gracefully...")
	close(messages)
	w.wg.Wait()
	if w.db != nil && w.db.Master != nil {
		w.db.Master.Close()
	}
	if w.consumer != nil {
		w.consumer.Close()
	}
	w.logger.Info().Msg("Worker stopped gracefully")
	return nil
}

func (w *Worker) processWorker(ctx context.Context, id int, messages <-chan *broker.Message) {
	w.logger.Info().Int("worker_id", id).Msg("Worker started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Debug().Int("worker_id", id).Msg("Worker stopping")
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			startTime := time.Now()
			w.logger.Debug().Int("worker_id", id).Int("message_size", len(msg.Value)).Msg("Processing message")
			if err := w.safeProcessMessage(ctx, id, msg); err != nil {
				w.logger.Error().
					Err(err).
					Int("worker_id", id).
					Int64("offset", msg.Offset).
					Msg("Failed to process message")
			} else {
				commitErr := w.consumer.Commit(ctx, msg.Key, msg.Offset)
				if commitErr != nil {
					w.logger.Error().
						Err(commitErr).
						Int64("offset", msg.Offset).
						Int("worker_id", id).
						Msg("Failed to commit message after successful processing")
				} else {
					w.logger.Debug().
						Int("worker_id", id).
						Int64("offset", msg.Offset).
						Dur("duration", time.Since(startTime)).
						Msg("Message processed and committed successfully")
				}
			}
		}
	}
}

func (w *Worker) safeProcessMessage(ctx context.Context, workerID int, msg *broker.Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error().
				Int("worker_id", workerID).
				Interface("panic", r).
				Int64("offset", msg.Offset).
				Msg("Panic recovered while processing message")
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return w.processMessage(ctx, msg)
}

func (w *Worker) processMessage(ctx context.Context, msg *broker.Message) error {
	var task domain.ProcessingTask
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		w.logger.Error().Err(err).Str("message", string(msg.Value)).Int64("offset", msg.Offset).Msg("Failed to unmarshal task")
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}
	w.logger.Info().
		Str("task_id", task.ID).
		Str("image_id", task.ImageID).
		Int("operations", len(task.Operations)).
		Int64("offset", msg.Offset).
		Msg("Processing task started")
	reader, err := w.fileRepo.GetObject(ctx, task.OriginalPath)
	if err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Str("path", task.OriginalPath).Msg("Failed to get original image")
		if updateErr := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusFailed); updateErr != nil {
			w.logger.Error().Err(updateErr).Str("image_id", task.ImageID).Msg("Failed to update status to failed")
		}
		return fmt.Errorf("failed to get original image: %w", err)
	}
	defer reader.Close()
	imageData, err := io.ReadAll(reader)
	if err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to read image data")
		if updateErr := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusFailed); updateErr != nil {
			w.logger.Error().Err(updateErr).Str("image_id", task.ImageID).Msg("Failed to update status to failed")
		}
		return fmt.Errorf("failed to read image %w", err)
	}
	result, err := w.processor.Process(ctx, &task, imageData)
	if err != nil {
		w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Image processing failed")
		if updateErr := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusFailed); updateErr != nil {
			w.logger.Error().Err(updateErr).Str("image_id", task.ImageID).Msg("Failed to update status to failed")
		}
		return fmt.Errorf("image processing failed: %w", err)
	}
	for operation, path := range result.ProcessedPaths {
		processedImage := &domain.ProcessedImage{
			ImageID:   task.ImageID,
			Operation: domain.OperationType(operation),
			Path:      path,
			Status:    "completed",
			Format:    task.Format,
			CreatedAt: time.Now(),
		}
		if err := w.imageRepo.SaveProcessedImage(ctx, processedImage); err != nil {
			w.logger.Error().Err(err).Str("image_id", task.ImageID).Str("operation", operation).Msg("Failed to save processed image metadata")
		}
	}
	if result.Status == domain.StatusCompleted {
		if err := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusCompleted); err != nil {
			w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to update status to completed")
		} else {
			w.logger.Info().
				Str("image_id", task.ImageID).
				Int("processed_operations", len(result.ProcessedPaths)).
				Msg("Image processing completed successfully")
		}
	} else {
		if err := w.imageRepo.UpdateStatus(ctx, task.ImageID, domain.StatusFailed); err != nil {
			w.logger.Error().Err(err).Str("image_id", task.ImageID).Msg("Failed to update status to failed")
		}
		w.logger.Error().
			Str("image_id", task.ImageID).
			Str("error", result.Error).
			Msg("Image processing failed")
	}
	return nil
}
