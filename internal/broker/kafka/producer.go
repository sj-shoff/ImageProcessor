package kafka

import (
	"context"
	"fmt"

	"image-processor/internal/config"

	wbkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

type ProducerClient struct {
	resultsProducer    *wbkafka.Producer
	processingProducer *wbkafka.Producer
	cfg                *config.Config
}

func NewProducerClient(cfg *config.Config) *ProducerClient {
	return &ProducerClient{
		resultsProducer:    wbkafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ResultsTopic),
		processingProducer: wbkafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ProcessingTopic),
		cfg:                cfg,
	}
}

func (p *ProducerClient) SendTask(ctx context.Context, strategy retry.Strategy, key, value []byte) error {
	return p.processingProducer.SendWithRetry(ctx, strategy, key, value)
}

func (p *ProducerClient) SendResult(ctx context.Context, strategy retry.Strategy, key, value []byte) error {
	return p.resultsProducer.SendWithRetry(ctx, strategy, key, value)
}

func (p *ProducerClient) Close() error {
	var err error
	if p.resultsProducer != nil {
		if closeErr := p.resultsProducer.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close results producer: %w", closeErr)
		}
	}
	if p.processingProducer != nil {
		if closeErr := p.processingProducer.Close(); closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; failed to close processing producer: %w", err, closeErr)
			} else {
				err = fmt.Errorf("failed to close processing producer: %w", closeErr)
			}
		}
	}
	return err
}
