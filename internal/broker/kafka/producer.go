package kafka

import (
	"context"
	"image-processor/internal/config"

	wbkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

type ProducerClient struct {
	resultsProducer    *wbkafka.Producer
	processingProducer *wbkafka.Producer
}

func NewProducerClient(cfg *config.Config) *ProducerClient {
	return &ProducerClient{
		resultsProducer:    wbkafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ResultsTopic),
		processingProducer: wbkafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ProcessingTopic),
	}
}

func (p *ProducerClient) SendResult(ctx context.Context, strategy retry.Strategy, key, value []byte) error {
	return p.resultsProducer.SendWithRetry(ctx, strategy, key, value)
}

func (p *ProducerClient) SendProcessingTask(ctx context.Context, strategy retry.Strategy, key, value []byte) error {
	return p.processingProducer.SendWithRetry(ctx, strategy, key, value)
}

func (p *ProducerClient) Close() error {
	if p.resultsProducer != nil {
		p.resultsProducer.Close()
	}
	if p.processingProducer != nil {
		p.processingProducer.Close()
	}
	return nil
}
