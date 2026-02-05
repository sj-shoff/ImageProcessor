package broker

import (
	"context"

	"github.com/wb-go/wbf/retry"
)

type Message struct {
	Key    []byte
	Value  []byte
	Offset int64
}

type Producer interface {
	SendTask(ctx context.Context, strategy retry.Strategy, key, value []byte) error
	SendResult(ctx context.Context, strategy retry.Strategy, key, value []byte) error
	Close() error
}

type Consumer interface {
	Fetch(ctx context.Context, strategy retry.Strategy) (*Message, error)
	Commit(ctx context.Context, key []byte, offset int64) error
	Start(ctx context.Context, out chan<- *Message, strategy retry.Strategy)
	Close() error
}
