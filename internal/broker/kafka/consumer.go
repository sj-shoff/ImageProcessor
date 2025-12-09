package kafka

import (
	"context"
	"image-processor/internal/config"

	kafka "github.com/segmentio/kafka-go"
	wbkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

type ConsumerClient struct {
	consumer *wbkafka.Consumer
}

func NewConsumerClient(cfg *config.Config) *ConsumerClient {
	return &ConsumerClient{
		consumer: wbkafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.ProcessingTopic, cfg.Kafka.GroupID),
	}
}

func (c *ConsumerClient) Fetch(ctx context.Context, strategy retry.Strategy) (kafka.Message, error) {
	return c.consumer.FetchWithRetry(ctx, strategy)
}

func (c *ConsumerClient) Commit(ctx context.Context, msg kafka.Message) error {
	return c.consumer.Commit(ctx, msg)
}

func (c *ConsumerClient) StartConsuming(ctx context.Context, out chan<- kafka.Message, strategy retry.Strategy) {
	c.consumer.StartConsuming(ctx, out, strategy)
}

func (c *ConsumerClient) Close() error {
	return c.consumer.Close()
}
