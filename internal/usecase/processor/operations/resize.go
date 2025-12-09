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

type Resizer struct{}

func NewResizer() *Resizer {
	return &Resizer{}
}

func (r *Resizer) Process(ctx context.Context, img image.Image, format string, params map[string]interface{}) (io.Reader, string, error) {
	var width int
	if w, ok := params["width"].(float64); ok {
		width = int(w)
	} else if w, ok := params["width"].(int); ok {
		width = w
	} else if w, ok := params["width"].(int64); ok {
		width = int(w)
	} else if w, ok := params["width"].(int32); ok {
		width = int(w)
	} else {
		return nil, "", fmt.Errorf("width parameter is required and must be a number")
	}

	var height int
	if h, ok := params["height"].(float64); ok {
		height = int(h)
	} else if h, ok := params["height"].(int); ok {
		height = h
	} else if h, ok := params["height"].(int64); ok {
		height = int(h)
	} else if h, ok := params["height"].(int32); ok {
		height = int(h)
	} else {
		return nil, "", fmt.Errorf("height parameter is required and must be a number")
	}

	if width <= 0 || height <= 0 {
		return nil, "", fmt.Errorf("width and height must be positive numbers")
	}

	keepAspect, _ := params["keep_aspect"].(bool)

	if strings.ToLower(format) == "gif" {
		return r.processGIF(img, width, height, keepAspect)
	}

	return r.processStaticImage(img, format, width, height, keepAspect)
}

func (r *Resizer) processStaticImage(img image.Image, format string, width, height int, keepAspect bool) (io.Reader, string, error) {
	var resized image.Image

	if keepAspect {
		bounds := img.Bounds()
		origWidth := bounds.Dx()
		origHeight := bounds.Dy()

		widthRatio := float64(width) / float64(origWidth)
		heightRatio := float64(height) / float64(origHeight)
		ratio := min(widthRatio, heightRatio)

		newWidth := int(float64(origWidth) * ratio)
		newHeight := int(float64(origHeight) * ratio)

		resized = resizeImage(img, newWidth, newHeight)
	} else {
		resized = resizeImage(img, width, height)
	}

	buf := new(bytes.Buffer)
	var err error

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		err = jpeg.Encode(buf, resized, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	case "png":
		err = png.Encode(buf, resized)
		format = "png"
	case "gif":
		err = gif.Encode(buf, resized, nil)
		format = "gif"
	default:
		err = jpeg.Encode(buf, resized, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to encode resized image: %w", err)
	}

	return buf, format, nil
}

func (r *Resizer) processGIF(img image.Image, width, height int, keepAspect bool) (io.Reader, string, error) {
	buf := new(bytes.Buffer)

	var resized image.Image
	if keepAspect {
		bounds := img.Bounds()
		origWidth := bounds.Dx()
		origHeight := bounds.Dy()

		widthRatio := float64(width) / float64(origWidth)
		heightRatio := float64(height) / float64(origHeight)
		ratio := min(widthRatio, heightRatio)

		newWidth := int(float64(origWidth) * ratio)
		newHeight := int(float64(origHeight) * ratio)

		resized = resizeImage(img, newWidth, newHeight)
	} else {
		resized = resizeImage(img, width, height)
	}

	err := gif.Encode(buf, resized, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode gif: %w", err)
	}

	return buf, "gif", nil
}

func resizeImage(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
