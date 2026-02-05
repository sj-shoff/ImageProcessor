package processor

import (
	"context"
	"io"
)

type fileRepository interface {
	SaveOriginal(ctx context.Context, filename string, data io.Reader, size int64) (string, error)
	GetObject(ctx context.Context, path string) (io.ReadCloser, error)
	SaveProcessed(ctx context.Context, path string, data io.Reader, size int64, contentType string) error
	DeleteObject(ctx context.Context, path string) error
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}
