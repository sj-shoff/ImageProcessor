package operations

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"image-processor/internal/domain"

	xdraw "golang.org/x/image/draw"
)

type Thumbnailer struct{}

func NewThumbnailer() *Thumbnailer {
	return &Thumbnailer{}
}

func (t *Thumbnailer) Process(ctx context.Context, img image.Image, format string, params map[string]interface{}) (io.Reader, string, error) {
	var size int
	if s, ok := params["size"].(float64); ok {
		size = int(s)
	} else if s, ok := params["size"].(int); ok {
		size = s
	} else if s, ok := params["size"].(int64); ok {
		size = int(s)
	} else if s, ok := params["size"].(int32); ok {
		size = int(s)
	} else {
		size = domain.DefaultThumbnailSize
	}

	if size <= 0 {
		return nil, "", fmt.Errorf("size must be a positive number")
	}

	cropToFit, _ := params["crop_to_fit"].(bool)

	if strings.ToLower(format) == "gif" {
		return t.processGIF(img, size, cropToFit)
	}

	return t.processStaticImage(img, format, size, cropToFit)
}

func (t *Thumbnailer) processStaticImage(img image.Image, format string, size int, cropToFit bool) (io.Reader, string, error) {
	var thumbnail image.Image

	if cropToFit {
		thumbnail = t.cropAndResize(img, size)
	} else {
		bounds := img.Bounds()
		origWidth := bounds.Dx()
		origHeight := bounds.Dy()

		var newWidth, newHeight int
		if origWidth > origHeight {
			newHeight = size
			newWidth = int(float64(origWidth) * float64(size) / float64(origHeight))
		} else {
			newWidth = size
			newHeight = int(float64(origHeight) * float64(size) / float64(origWidth))
		}

		thumbnail = resizeImage(img, newWidth, newHeight)
	}

	buf := new(bytes.Buffer)
	var err error

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		err = jpeg.Encode(buf, thumbnail, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	case "png":
		err = png.Encode(buf, thumbnail)
		format = "png"
	case "gif":
		err = gif.Encode(buf, thumbnail, nil)
		format = "gif"
	default:
		err = jpeg.Encode(buf, thumbnail, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return buf, format, nil
}

func (t *Thumbnailer) processGIF(img image.Image, size int, cropToFit bool) (io.Reader, string, error) {
	var thumbnail image.Image

	if cropToFit {
		thumbnail = t.cropAndResize(img, size)
	} else {
		bounds := img.Bounds()
		origWidth := bounds.Dx()
		origHeight := bounds.Dy()

		var newWidth, newHeight int
		if origWidth > origHeight {
			newHeight = size
			newWidth = int(float64(origWidth) * float64(size) / float64(origHeight))
		} else {
			newWidth = size
			newHeight = int(float64(origHeight) * float64(size) / float64(origWidth))
		}

		thumbnail = resizeImage(img, newWidth, newHeight)
	}

	buf := new(bytes.Buffer)
	err := gif.Encode(buf, thumbnail, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode gif thumbnail: %w", err)
	}

	return buf, "gif", nil
}

func (t *Thumbnailer) cropAndResize(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	var cropX, cropY, cropSize int
	if origWidth > origHeight {
		cropSize = origHeight
		cropX = (origWidth - origHeight) / 2
		cropY = 0
	} else {
		cropSize = origWidth
		cropX = 0
		cropY = (origHeight - origWidth) / 2
	}

	cropped := image.NewRGBA(image.Rect(0, 0, cropSize, cropSize))
	xdraw.BiLinear.Scale(cropped, cropped.Bounds(), img,
		image.Rect(cropX, cropY, cropX+cropSize, cropY+cropSize), xdraw.Over, nil)

	return resizeImage(cropped, size, size)
}
