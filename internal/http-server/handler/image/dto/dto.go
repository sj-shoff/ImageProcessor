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

type UploadRequest struct {
	File          interface{} `form:"file" binding:"required"`
	Thumbnail     bool        `form:"thumbnail"`
	Resize        bool        `form:"resize"`
	Watermark     bool        `form:"watermark"`
	WatermarkText string      `form:"watermark_text"`
}

type GetImageRequest struct {
	ID        string `uri:"id" binding:"required"`
	Operation string `form:"operation"`
}

type StatusRequest struct {
	ID string `uri:"id" binding:"required"`
}

type DeleteRequest struct {
	ID string `uri:"id" binding:"required"`
}

type ImageResponse struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
