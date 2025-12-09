package image

import "errors"

var (
	ErrInvalidFileFormat      = errors.New("invalid file format")
	ErrFileTooLarge           = errors.New("file too large")
	ErrImageNotFound          = errors.New("image not found")
	ErrProcessedImageNotFound = errors.New("processed image not found")
	ErrStorageError           = errors.New("storage error")
	ErrDatabaseError          = errors.New("database error")
	ErrMessageQueueError      = errors.New("message queue error")
)
