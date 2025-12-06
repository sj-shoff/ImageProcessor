package image

import "errors"

var (
	ErrImageNotFound          = errors.New("image not found")
	ErrProcessedImageNotFound = errors.New("processed image not found")
	ErrFileNotFound           = errors.New("file not found")
	ErrStorageError           = errors.New("storage error")
	ErrStorageValidation      = errors.New("storage validation failed")
	ErrDuplicateKey           = errors.New("duplicate key violation")
	ErrForeignKeyViolation    = errors.New("foreign key violation")
)
