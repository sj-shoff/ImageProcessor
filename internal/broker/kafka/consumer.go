package kafka

import (
	"context"
	"image-processor/internal/config"

	kafka "github.com/segmentio/kafka-go"
	wbkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

type consumerClient struct {
	consumer *wbkafka.Consumer
}

func NewConsumerClient(cfg *config.Config) *consumerClient {
	return &consumerClient{
		consumer: wbkafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.ResultsTopic, cfg.Kafka.GroupID),
	}
}

func (c *consumerClient) Fetch(ctx context.Context, strategy retry.Strategy) (kafka.Message, error) {
	return c.consumer.FetchWithRetry(ctx, strategy)
}

func (c *consumerClient) Commit(ctx context.Context, msg kafka.Message) error {
	return c.consumer.Commit(ctx, msg)
}

func (c *consumerClient) Close() error {
	return c.consumer.Close()
}

func (c *consumerClient) StartConsuming(ctx context.Context, out chan<- kafka.Message, strategy retry.Strategy) {
	c.consumer.StartConsuming(ctx, out, strategy)
}
