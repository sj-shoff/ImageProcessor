package kafka

import (
	"context"
	"fmt"

	"image-processor/internal/broker"
	"image-processor/internal/config"

	"github.com/segmentio/kafka-go"
	wbkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

type ConsumerClient struct {
	consumer     *wbkafka.Consumer
	cfg          *config.Config
	latestOffset int64
}

func NewConsumerClient(cfg *config.Config) *ConsumerClient {
	return &ConsumerClient{
		consumer: wbkafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.ProcessingTopic, cfg.Kafka.GroupID),
		cfg:      cfg,
	}
}

func (c *ConsumerClient) Fetch(ctx context.Context, strategy retry.Strategy) (*broker.Message, error) {
	msg, err := c.consumer.FetchWithRetry(ctx, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}
	c.latestOffset = msg.Offset
	return &broker.Message{
		Key:    msg.Key,
		Value:  msg.Value,
		Offset: msg.Offset,
	}, nil
}

func (c *ConsumerClient) Commit(ctx context.Context, key []byte, offset int64) error {
	if c.latestOffset != offset {
		return fmt.Errorf("offset mismatch: expected %d, got %d", c.latestOffset, offset)
	}
	fakeMsg := kafka.Message{
		Topic:     c.cfg.Kafka.ProcessingTopic,
		Partition: 0,
		Offset:    offset,
		Key:       key,
	}
	return c.consumer.Commit(ctx, fakeMsg)
}

func (c *ConsumerClient) Start(ctx context.Context, out chan<- *broker.Message, strategy retry.Strategy) {
	kafkaOut := make(chan kafka.Message)
	go c.consumer.StartConsuming(ctx, kafkaOut, strategy)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case kafkaMsg, ok := <-kafkaOut:
				if !ok {
					return
				}
				msg := &broker.Message{
					Key:    kafkaMsg.Key,
					Value:  kafkaMsg.Value,
					Offset: kafkaMsg.Offset,
				}
				c.latestOffset = kafkaMsg.Offset
				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}

func (c *ConsumerClient) Close() error {
	if c.consumer == nil {
		return nil
	}
	return c.consumer.Close()
}
