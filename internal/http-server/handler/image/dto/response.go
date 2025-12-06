package dto

import "time"

type UploadResponse struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Status    string    `json:"status"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

type StatusResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
