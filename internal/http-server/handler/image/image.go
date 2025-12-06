package image

import (
	"github.com/go-playground/validator/v10"
	"github.com/wb-go/wbf/zlog"
)

type ImageHandler struct {
}

type CommentsHandler struct {
	usecase  imageUsecase
	logger   *zlog.Zerolog
	validate *validator.Validate
}

func NewImageHandler(usecase imageUsecase, logger *zlog.Zerolog) *CommentsHandler {
	return &CommentsHandler{
		usecase:  usecase,
		logger:   logger,
		validate: validator.New(),
	}
}

func (h *ImageHandler) GetImage() error {
	panic("implement me")
}

func (h *ImageHandler) GetStatus() error {
	panic("implement me")
}

func (h *ImageHandler) UploadImage() error {
	panic("implement me")
}

func (h *ImageHandler) DeleteImage() error {
	panic("implement me")
}
