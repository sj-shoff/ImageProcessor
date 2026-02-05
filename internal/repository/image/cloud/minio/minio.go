package minio

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"image-processor/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wb-go/wbf/retry"
)

type FileRepository struct {
	client  *minio.Client
	cfg     *config.Config
	retries retry.Strategy
}

func sanitizePath(path string) (string, error) {
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("invalid path: path traversal detected")
	}
	return strings.TrimPrefix(clean, "/"), nil
}

func sanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	invalidChars := []string{"..", "/", "\\", "<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		base = strings.ReplaceAll(base, char, "_")
	}
	return base
}

func NewMinIORepository(cfg *config.Config, retries retry.Strategy) (*FileRepository, error) {
	minioClient, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
		Secure: cfg.MinIO.UseSSL,
		Region: cfg.MinIO.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := minioClient.BucketExists(ctx, cfg.MinIO.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, cfg.MinIO.Bucket, minio.MakeBucketOptions{
			Region: cfg.MinIO.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return &FileRepository{
		client:  minioClient,
		cfg:     cfg,
		retries: retries,
	}, nil
}

func (r *FileRepository) SaveOriginal(ctx context.Context, filename string, data io.Reader, size int64) (string, error) {
	safeFilename := sanitizeFilename(filename)
	ext := strings.ToLower(filepath.Ext(safeFilename))
	if ext == "" {
		ext = ".dat"
	}
	now := time.Now()
	objectName := fmt.Sprintf("original/%s/%d%s",
		now.Format("2006/01/02"),
		now.UnixNano(),
		ext,
	)
	contentType := getContentType(ext)
	safePath, err := sanitizePath(objectName)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	_, err = r.client.PutObject(ctx, r.cfg.MinIO.Bucket, safePath, data, size, minio.PutObjectOptions{
		ContentType:        contentType,
		ContentDisposition: fmt.Sprintf("attachment; filename=\"%s\"", safeFilename),
		UserMetadata: map[string]string{
			"original-filename": safeFilename,
			"uploaded-at":       now.Format(time.RFC3339),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	return safePath, nil
}

func (r *FileRepository) GetObject(ctx context.Context, path string) (io.ReadCloser, error) {
	safePath, err := sanitizePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	obj, err := r.client.GetObject(ctx, r.cfg.MinIO.Bucket, safePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	_, err = obj.Stat()
	if err != nil {
		obj.Close()
		return nil, fmt.Errorf("object not found: %w", err)
	}
	return obj, nil
}

func (r *FileRepository) SaveProcessed(ctx context.Context, path string, data io.Reader, size int64, contentType string) error {
	safePath, err := sanitizePath(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	_, err = r.client.PutObject(ctx, r.cfg.MinIO.Bucket, safePath, data, size, minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000",
	})
	if err != nil {
		return fmt.Errorf("failed to save processed image: %w", err)
	}
	return nil
}

func (r *FileRepository) DeleteObject(ctx context.Context, path string) error {
	safePath, err := sanitizePath(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	err = r.client.RemoveObject(ctx, r.cfg.MinIO.Bucket, safePath, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

func (r *FileRepository) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	safePrefix, err := sanitizePath(prefix)
	if err != nil {
		return fmt.Errorf("invalid prefix: %w", err)
	}
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range r.client.ListObjects(ctx, r.cfg.MinIO.Bucket,
			minio.ListObjectsOptions{
				Prefix:    safePrefix,
				Recursive: true,
			}) {
			if obj.Err != nil {
				continue
			}
			objectsCh <- obj
		}
	}()
	errorCh := r.client.RemoveObjects(ctx, r.cfg.MinIO.Bucket, objectsCh, minio.RemoveObjectsOptions{})
	var deleteErrors []error
	for err := range errorCh {
		if err.Err != nil {
			deleteErrors = append(deleteErrors, err.Err)
		}
	}
	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete %d objects", len(deleteErrors))
	}
	return nil
}

func (r *FileRepository) GetObjectURL(path string) string {
	scheme := "http"
	if r.cfg.MinIO.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s",
		scheme,
		r.cfg.MinIO.Endpoint,
		r.cfg.MinIO.Bucket,
		path,
	)
}

func getContentType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}
