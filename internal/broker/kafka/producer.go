package kafka

import (
	"context"
	"image-processor/internal/config"

	wbkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

type producerClient struct {
	producer *wbkafka.Producer
}

func NewProducerClient(cfg *config.Config) *producerClient {
	return &producerClient{
		producer: wbkafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ProcessingTopic),
	}
}

func (p *producerClient) Send(ctx context.Context, strategy retry.Strategy, key, value []byte) error {
	return p.producer.SendWithRetry(ctx, strategy, key, value)
}

func (p *producerClient) Close() error {
	return p.producer.Close()
}
