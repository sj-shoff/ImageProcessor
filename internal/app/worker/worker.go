package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"image-processor/internal/broker/kafka"
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
	consumer    *kafka.ConsumerClient
	processor   *processor.ImageProcessor
	imageRepo   *postgres_repo.ImagesRepository
	fileRepo    *minio_repo.FileRepository
	concurrency int
	wg          sync.WaitGroup
	stopChan    chan struct{}
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

	fileRepo, err := minio_repo.NewMinIORepository(cfg, retries, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create file repository: %w", err)
	}

	imageRepo := postgres_repo.NewImagesRepository(db, retries)
	consumer := kafka.NewConsumerClient(cfg)
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
		stopChan:    make(chan struct{}),
	}, nil
}

func (w *Worker) Run() error {
	w.logger.Info().Int("concurrency", w.concurrency).Msg("Starting worker")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages := make(chan []byte, w.concurrency*2)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.consumeMessages(ctx, messages)
	}()

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go func(id int) {
			defer w.wg.Done()
			w.worker(ctx, id, messages)
		}(i)
	}

	w.logger.Info().Msg("Worker started successfully")

	<-w.stopChan
	w.logger.Info().Msg("Shutting down worker")

	cancel()

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

func (w *Worker) Stop() {
	select {
	case w.stopChan <- struct{}{}:
	default:
	}
}

func (w *Worker) consumeMessages(ctx context.Context, messages chan<- []byte) {
	w.logger.Info().Msg("Starting Kafka consumer")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("Stopping Kafka consumer")
			return
		default:
			msg, err := w.consumer.Fetch(ctx, w.cfg.DefaultRetryStrategy())
			if err != nil {
				if ctx.Err() == nil {
					w.logger.Error().Err(err).Msg("Failed to fetch message from Kafka")
					select {
					case <-time.After(2 * time.Second):
						continue
					case <-ctx.Done():
						return
					}
				}
				return
			}

			w.logger.Info().
				Str("topic", msg.Topic).
				Int("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Int("size", len(msg.Value)).
				Msg("Message received from Kafka")

			select {
			case messages <- msg.Value:
				if err := w.consumer.Commit(ctx, msg); err != nil {
					w.logger.Error().Err(err).Msg("Failed to commit message")
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Worker) worker(ctx context.Context, id int, messages <-chan []byte) {
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
			w.logger.Debug().Int("worker_id", id).Int("message_size", len(msg)).Msg("Processing message")

			if err := w.safeProcessMessage(ctx, id, msg); err != nil {
				w.logger.Error().
					Err(err).
					Int("worker_id", id).
					Msg("Failed to process message")
			} else {
				w.logger.Debug().
					Int("worker_id", id).
					Dur("duration", time.Since(startTime)).
					Msg("Message processed successfully")
			}
		}
	}
}

func (w *Worker) safeProcessMessage(ctx context.Context, workerID int, msg []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error().
				Int("worker_id", workerID).
				Interface("panic", r).
				Msg("Panic recovered while processing message")
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return w.processMessage(ctx, msg)
}

func (w *Worker) processMessage(ctx context.Context, msg []byte) error {
	var task domain.ProcessingTask
	if err := json.Unmarshal(msg, &task); err != nil {
		w.logger.Error().Err(err).Str("message", string(msg)).Msg("Failed to unmarshal task")
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	w.logger.Info().
		Str("task_id", task.ID).
		Str("image_id", task.ImageID).
		Int("operations", len(task.Operations)).
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

		return fmt.Errorf("failed to read image data: %w", err)
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
		} else {
			w.logger.Debug().
				Str("image_id", task.ImageID).
				Str("operation", operation).
				Str("path", path).
				Msg("Processed image saved to database")
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
