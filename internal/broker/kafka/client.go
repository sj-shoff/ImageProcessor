package kafka

import (
	"context"
	"errors"
	"image-processor/internal/config"

	"github.com/segmentio/kafka-go"
	"github.com/wb-go/wbf/retry"
)

type KafkaClient struct {
	producerClient *producerClient
	consumerClient *consumerClient
}

func NewKafkaClient(cfg *config.Config) *KafkaClient {
	return &KafkaClient{
		producerClient: NewProducerClient(cfg),
		consumerClient: NewConsumerClient(cfg),
	}
}

func (k *KafkaClient) Send(ctx context.Context, strategy retry.Strategy, key, value []byte) error {
	return k.producerClient.Send(ctx, strategy, key, value)
}

func (k *KafkaClient) Fetch(ctx context.Context, strategy retry.Strategy) (kafka.Message, error) {
	return k.consumerClient.Fetch(ctx, strategy)
}

func (k *KafkaClient) Commit(ctx context.Context, msg kafka.Message) error {
	return k.consumerClient.Commit(ctx, msg)
}

func (k *KafkaClient) StartConsuming(ctx context.Context, out chan<- kafka.Message, strategy retry.Strategy) {
	k.consumerClient.StartConsuming(ctx, out, strategy)
}

func (k *KafkaClient) Close() error {
	var errs []error

	if k.producerClient != nil {
		if err := k.producerClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if k.consumerClient != nil {
		if err := k.consumerClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
