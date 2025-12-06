package dto

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
