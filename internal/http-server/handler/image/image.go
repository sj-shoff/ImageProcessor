package image

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"image-processor/internal/domain"
	"image-processor/internal/http-server/handler/image/dto"
	image_uc "image-processor/internal/usecase/image"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/wb-go/wbf/zlog"
)

const (
	maxMemory = 32 << 20
)

type ImageHandler struct {
	usecase  imageUsecase
	validate *validator.Validate
	logger   *zlog.Zerolog
}

func NewImageHandler(usecase imageUsecase, logger *zlog.Zerolog) *ImageHandler {
	return &ImageHandler{
		usecase:  usecase,
		validate: validator.New(),
		logger:   logger,
	}
}

func (h *ImageHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, domain.DefaultMaxUploadSize)

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to parse multipart form")
		h.respondError(w, http.StatusBadRequest, "Invalid request format", nil)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		h.logger.Warn().Err(err).Msg("File not found in request")
		h.respondError(w, http.StatusBadRequest, "File is required", nil)
		return
	}
	defer file.Close()

	if err := h.validateFile(handler); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error().Err(err).Str("filename", handler.Filename).Msg("Failed to read file")
		h.respondError(w, http.StatusInternalServerError, "Failed to read file", err)
		return
	}

	operations := h.parseOperationsFromForm(r.Form)

	image, err := h.usecase.UploadImage(
		ctx,
		bytes.NewReader(fileBytes),
		handler.Filename,
		handler.Header.Get("Content-Type"),
		int64(len(fileBytes)),
		operations,
	)
	if err != nil {
		h.handleUploadError(w, err, handler.Filename)
		return
	}

	response := dto.UploadResponse{
		ID:        image.ID,
		Filename:  image.OriginalFilename,
		Status:    string(image.Status),
		Size:      image.OriginalSize,
		CreatedAt: image.CreatedAt,
	}

	h.logger.Info().
		Str("image_id", image.ID).
		Str("filename", image.OriginalFilename).
		Str("status", string(image.Status)).
		Msg("Image uploaded successfully")

	h.respondJSON(w, http.StatusAccepted, response)
}

func (h *ImageHandler) GetImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req := dto.GetImageRequest{
		ID:        chi.URLParam(r, "id"),
		Operation: r.URL.Query().Get("operation"),
	}

	if req.ID == "" {
		h.respondError(w, http.StatusBadRequest, "Image ID is required", nil)
		return
	}

	img, reader, err := h.usecase.GetImage(ctx, req.ID, req.Operation)
	if err != nil {
		h.handleGetImageError(w, err, req.ID, req.Operation)
		return
	}
	defer reader.Close()

	filename := h.getDownloadFilename(img.OriginalFilename, req.Operation)
	w.Header().Set("Content-Type", img.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	w.Header().Set("Cache-Control", "public, max-age=3600")

	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Error().
			Err(err).
			Str("image_id", req.ID).
			Str("operation", req.Operation).
			Msg("Failed to stream image")
	}
}

func (h *ImageHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req := dto.StatusRequest{
		ID: chi.URLParam(r, "id"),
	}

	if req.ID == "" {
		h.respondError(w, http.StatusBadRequest, "Image ID is required", nil)
		return
	}

	status, err := h.usecase.GetStatus(ctx, req.ID)
	if err != nil {
		h.handleStatusError(w, err, req.ID)
		return
	}

	response := dto.StatusResponse{
		ID:     req.ID,
		Status: string(status),
	}

	h.respondJSON(w, http.StatusOK, response)
}

func (h *ImageHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req := dto.DeleteRequest{
		ID: chi.URLParam(r, "id"),
	}

	if req.ID == "" {
		h.respondError(w, http.StatusBadRequest, "Image ID is required", nil)
		return
	}

	if err := h.usecase.DeleteImage(ctx, req.ID); err != nil {
		h.handleDeleteError(w, err, req.ID)
		return
	}

	h.logger.Info().Str("image_id", req.ID).Msg("Image deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (h *ImageHandler) validateFile(handler *multipart.FileHeader) error {
	if handler.Size > domain.DefaultMaxUploadSize {
		return fmt.Errorf("File is too large (max %d MB)", domain.DefaultMaxUploadSize/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(handler.Filename))
	if !h.isValidExtension(ext) {
		return fmt.Errorf("Unsupported file format. Allowed: jpg, jpeg, png, gif, webp, bmp")
	}

	contentType := handler.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("File must be an image")
	}

	return nil
}

func (h *ImageHandler) isValidExtension(ext string) bool {
	allowed := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".bmp":  true,
		".tiff": true,
	}
	return allowed[ext]
}

func (h *ImageHandler) parseOperationsFromForm(form url.Values) []domain.OperationParams {
	var operations []domain.OperationParams

	if form.Get("thumbnail") == "true" {
		operations = append(operations, domain.OperationParams{
			Type: domain.OpThumbnail,
			Parameters: map[string]interface{}{
				"size":        200,
				"crop_to_fit": true,
			},
		})
	}

	if form.Get("resize") == "true" {
		operations = append(operations, domain.OperationParams{
			Type: domain.OpResize,
			Parameters: map[string]interface{}{
				"width":       1024,
				"height":      768,
				"keep_aspect": true,
			},
		})
	}

	if form.Get("watermark") == "true" {
		params := map[string]interface{}{
			"text":     "© ImageProcessor",
			"opacity":  0.5,
			"position": "bottom-right",
		}

		if text := form.Get("watermark_text"); text != "" {
			params["text"] = text
		}

		operations = append(operations, domain.OperationParams{
			Type:       domain.OpWatermark,
			Parameters: params,
		})
	}

	if len(operations) == 0 {
		operations = []domain.OperationParams{
			{
				Type: domain.OpThumbnail,
				Parameters: map[string]interface{}{
					"size":        200,
					"crop_to_fit": true,
				},
			},
			{
				Type: domain.OpResize,
				Parameters: map[string]interface{}{
					"width":       1024,
					"height":      768,
					"keep_aspect": true,
				},
			},
		}
	}

	return operations
}

func (h *ImageHandler) handleUploadError(w http.ResponseWriter, err error, filename string) {
	switch {
	case errors.Is(err, image_uc.ErrInvalidFileFormat):
		h.logger.Warn().Str("filename", filename).Msg("Invalid file format")
		h.respondError(w, http.StatusBadRequest, "Unsupported file format", nil)
	case errors.Is(err, image_uc.ErrFileTooLarge):
		h.logger.Warn().Str("filename", filename).Msg("File too large")
		h.respondError(w, http.StatusRequestEntityTooLarge, "File too large", nil)
	default:
		h.logger.Error().Err(err).Str("filename", filename).Msg("Upload failed")
		h.respondError(w, http.StatusInternalServerError, "Failed to upload file", err)
	}
}

func (h *ImageHandler) handleGetImageError(w http.ResponseWriter, err error, imageID, operation string) {
	switch {
	case errors.Is(err, image_uc.ErrImageNotFound):
		h.logger.Info().Str("image_id", imageID).Msg("Image not found")
		h.respondError(w, http.StatusNotFound, "Image not found", nil)
	case errors.Is(err, image_uc.ErrProcessedImageNotFound):
		h.logger.Info().Str("image_id", imageID).Str("operation", operation).Msg("Processed image not found")
		h.respondError(w, http.StatusNotFound, "Processed version not found", nil)
	default:
		h.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to get image")
		h.respondError(w, http.StatusInternalServerError, "Failed to get image", err)
	}
}

func (h *ImageHandler) handleStatusError(w http.ResponseWriter, err error, imageID string) {
	switch {
	case errors.Is(err, image_uc.ErrImageNotFound):
		h.respondError(w, http.StatusNotFound, "Image not found", nil)
	default:
		h.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to get status")
		h.respondError(w, http.StatusInternalServerError, "Failed to get status", err)
	}
}

func (h *ImageHandler) handleDeleteError(w http.ResponseWriter, err error, imageID string) {
	switch {
	case errors.Is(err, image_uc.ErrImageNotFound):
		h.respondError(w, http.StatusNotFound, "Image not found", nil)
	default:
		h.logger.Error().Err(err).Str("image_id", imageID).Msg("Failed to delete image")
		h.respondError(w, http.StatusInternalServerError, "Failed to delete image", err)
	}
}

func (h *ImageHandler) getDownloadFilename(originalName, operation string) string {
	if operation == "" {
		return originalName
	}

	ext := filepath.Ext(originalName)
	name := strings.TrimSuffix(originalName, ext)
	return fmt.Sprintf("%s_%s%s", name, operation, ext)
}

func (h *ImageHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Interface("data", data).Msg("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *ImageHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	response := dto.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	}

	if err != nil {
		response.Details = err.Error()
	}

	h.respondJSON(w, status, response)
}
