package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	// "image-processor/internal/broker/kafka"
	"image-processor/internal/config"
	// image_h "image-processor/internal/http-server/handler/image"
	"image-processor/internal/http-server/router"
	// minio_repo "image-processor/internal/repository/image/cloud/minio"
	// postgres_repo "image-processor/internal/repository/image/db/postgres"
	// image_uc "image-processor/internal/usecase/image"

	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/zlog"
)

type App struct {
	cfg    *config.Config
	server *http.Server
	logger *zlog.Zerolog
	db     *dbpg.DB
}

func NewApp(cfg *config.Config, logger *zlog.Zerolog) (*App, error) {
	// retries := cfg.DefaultRetryStrategy()

	dbOpts := &dbpg.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
	}

	db, err := dbpg.New(cfg.DBDSN(), []string{}, dbOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// fileRepo, err := minio_repo.NewMinIORepository(file_client, cfg, retries)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create file repository: %w", err)
	// }
	// imageRepo := postgres_repo.NewImageRepository(db, cfg.DefaultRetryStrategy())

	// kafkaProducer := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ProcessingTopic)
	// imageUsecase := image_uc.NewImageUsecase(imageRepo, fileRepo, kafkaProducer, logger)
	// imageHandler := image_h.NewImageHandler(imageUsecase, logger)

	h := &router.Handler{
		//ImageHandler: imageHandler,
	}

	mux := router.SetupRouter(h)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &App{
		cfg:    cfg,
		server: server,
		logger: logger,
		db:     db,
	}, nil
}

func (a *App) Run() error {
	a.logger.Info().Str("addr", a.cfg.Server.Addr).Msg("Starting server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.handleSignals(cancel)

	serverErr := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		a.logger.Error().Err(err).Msg("Server error")
		return err
	case <-ctx.Done():
		a.logger.Info().Msg("Shutting down server")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
		defer shutdownCancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			a.logger.Error().Err(err).Msg("Server shutdown failed")
		}

		a.db.Master.Close()
		a.logger.Info().Msg("Server stopped gracefully")
		return nil
	}
}

func (a *App) handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	a.logger.Info().Str("signal", sig.String()).Msg("Received signal")
	cancel()
}
