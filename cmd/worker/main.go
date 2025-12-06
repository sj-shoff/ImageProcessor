package main

import (
	"image-processor/internal/app/worker"
	"image-processor/internal/config"
	"os"

	"github.com/wb-go/wbf/zlog"
)

func main() {
	zlog.Init()

	cfg, err := config.MustLoad()
	if err != nil {
		zlog.Logger.Fatal().Err(err).Msg("Failed to load config")
	}

	workerApp, err := worker.NewWorker(cfg, &zlog.Logger)
	if err != nil {
		zlog.Logger.Fatal().Err(err).Msg("Failed to create worker")
	}

	if err := workerApp.Run(); err != nil {
		zlog.Logger.Fatal().Err(err).Msg("Worker failed")
	}

	zlog.Logger.Info().Msg("Worker exited successfully")
	os.Exit(0)
}
