package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"image-processor/internal/domain"
	"image-processor/internal/repository/image"

	"github.com/google/uuid"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/retry"
)

type ImagesRepository struct {
	db      *dbpg.DB
	retries retry.Strategy
}

func NewImagesRepository(db *dbpg.DB, retries retry.Strategy) *ImagesRepository {
	return &ImagesRepository{
		db:      db,
		retries: retries,
	}
}

func (r *ImagesRepository) Save(ctx context.Context, img *domain.Image) error {
	query := `
		INSERT INTO images (
			id, original_filename, original_size, mime_type,
			status, original_path, bucket, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecWithRetry(ctx, r.retries, query,
		img.ID,
		img.OriginalFilename,
		img.OriginalSize,
		img.MimeType,
		img.Status,
		img.OriginalPath,
		img.Bucket,
		img.CreatedAt,
		img.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	return nil
}

func (r *ImagesRepository) GetByID(ctx context.Context, id string) (*domain.Image, error) {
	query := `
		SELECT id, original_filename, original_size, mime_type,
		       status, original_path, bucket, created_at, updated_at
		FROM images
		WHERE id = $1 AND status != $2
	`

	row, err := r.db.QueryRowWithRetry(ctx, r.retries, query, id, domain.StatusDeleted)
	if err != nil {
		return nil, fmt.Errorf("failed to query image: %w", err)
	}

	var img domain.Image
	err = row.Scan(
		&img.ID,
		&img.OriginalFilename,
		&img.OriginalSize,
		&img.MimeType,
		&img.Status,
		&img.OriginalPath,
		&img.Bucket,
		&img.CreatedAt,
		&img.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, image.ErrImageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan image: %w", err)
	}

	return &img, nil
}

func (r *ImagesRepository) UpdateStatus(ctx context.Context, id string, status domain.ImageStatus) error {
	query := `UPDATE images SET status = $1, updated_at = $2 WHERE id = $3`

	result, err := r.db.ExecWithRetry(ctx, r.retries, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return image.ErrImageNotFound
	}

	return nil
}

func (r *ImagesRepository) Update(ctx context.Context, img *domain.Image) error {
	query := `UPDATE images SET status = $1, updated_at = $2 WHERE id = $3`

	img.UpdatedAt = time.Now()
	result, err := r.db.ExecWithRetry(ctx, r.retries, query, img.Status, img.UpdatedAt, img.ID)
	if err != nil {
		return fmt.Errorf("failed to update image: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return image.ErrImageNotFound
	}

	return nil
}

func (r *ImagesRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE images SET status = $1, updated_at = $2 WHERE id = $3`

	result, err := r.db.ExecWithRetry(ctx, r.retries, query, domain.StatusDeleted, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return image.ErrImageNotFound
	}

	return nil
}

func (r *ImagesRepository) SaveProcessedImage(ctx context.Context, processed *domain.ProcessedImage) error {
	query := `
		INSERT INTO processed_images (
			id, image_id, operation, parameters, path,
			size, mime_type, format, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	processed.ID = uuid.New().String()
	processed.CreatedAt = time.Now()

	_, err := r.db.ExecWithRetry(ctx, r.retries, query,
		processed.ID,
		processed.ImageID,
		processed.Operation,
		processed.Parameters,
		processed.Path,
		processed.Size,
		processed.MimeType,
		processed.Format,
		processed.Status,
		processed.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save processed image: %w", err)
	}

	return nil
}

func (r *ImagesRepository) GetProcessedImages(ctx context.Context, imageID string) ([]domain.ProcessedImage, error) {
	query := `
		SELECT id, image_id, operation, parameters, path,
		       size, mime_type, format, status, created_at
		FROM processed_images
		WHERE image_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryWithRetry(ctx, r.retries, query, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query processed images: %w", err)
	}
	defer rows.Close()

	var processed []domain.ProcessedImage
	for rows.Next() {
		var p domain.ProcessedImage
		err := rows.Scan(
			&p.ID,
			&p.ImageID,
			&p.Operation,
			&p.Parameters,
			&p.Path,
			&p.Size,
			&p.MimeType,
			&p.Format,
			&p.Status,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan processed image: %w", err)
		}
		processed = append(processed, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating processed images: %w", err)
	}

	return processed, nil
}

func (r *ImagesRepository) GetProcessedImageByOperation(ctx context.Context, imageID, operation string) (*domain.ProcessedImage, error) {
	query := `
		SELECT id, image_id, operation, parameters, path,
		       size, mime_type, format, status, created_at
		FROM processed_images
		WHERE image_id = $1 AND operation = $2
		LIMIT 1
	`

	row, err := r.db.QueryRowWithRetry(ctx, r.retries, query, imageID, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to query processed image: %w", err)
	}

	var processed domain.ProcessedImage
	err = row.Scan(
		&processed.ID,
		&processed.ImageID,
		&processed.Operation,
		&processed.Parameters,
		&processed.Path,
		&processed.Size,
		&processed.MimeType,
		&processed.Format,
		&processed.Status,
		&processed.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan processed image: %w", err)
	}

	return &processed, nil
}

func (r *ImagesRepository) DeleteProcessedImages(ctx context.Context, imageID string) error {
	query := `DELETE FROM processed_images WHERE image_id = $1`

	result, err := r.db.ExecWithRetry(ctx, r.retries, query, imageID)
	if err != nil {
		return fmt.Errorf("failed to delete processed images: %w", err)
	}

	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	return nil
}

func (r *ImagesRepository) List(ctx context.Context, limit, offset int) ([]domain.Image, error) {
	query := `
		SELECT id, original_filename, original_size, mime_type,
		       status, original_path, bucket, created_at, updated_at
		FROM images
		WHERE status != $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryWithRetry(ctx, r.retries, query, domain.StatusDeleted, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	defer rows.Close()

	var images []domain.Image
	for rows.Next() {
		var img domain.Image
		err := rows.Scan(
			&img.ID,
			&img.OriginalFilename,
			&img.OriginalSize,
			&img.MimeType,
			&img.Status,
			&img.OriginalPath,
			&img.Bucket,
			&img.CreatedAt,
			&img.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan image: %w", err)
		}
		images = append(images, img)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating images: %w", err)
	}

	return images, nil
}

func (r *ImagesRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM images WHERE status != $1`

	row, err := r.db.QueryRowWithRetry(ctx, r.retries, query, domain.StatusDeleted)
	if err != nil {
		return 0, fmt.Errorf("failed to count images: %w", err)
	}

	var count int
	err = row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to scan count: %w", err)
	}

	return count, nil
}
