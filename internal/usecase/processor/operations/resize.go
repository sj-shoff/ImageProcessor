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

	xdraw "golang.org/x/image/draw"
)

type Resizer struct{}

func NewResizer() *Resizer {
	return &Resizer{}
}

func (r *Resizer) Process(ctx context.Context, img image.Image, format string, params map[string]interface{}) (io.Reader, string, error) {
	width, ok := params["width"].(int)
	if !ok {
		return nil, "", fmt.Errorf("width parameter is required")
	}

	height, ok := params["height"].(int)
	if !ok {
		return nil, "", fmt.Errorf("height parameter is required")
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

		ratio := float64(origWidth) / float64(origHeight)
		newHeight := int(float64(width) / ratio)

		resized = resizeImage(img, width, newHeight)
	} else {
		resized = resizeImage(img, width, height)
	}

	buf := new(bytes.Buffer)
	var err error

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		err = jpeg.Encode(buf, resized, &jpeg.Options{Quality: 85})
		format = "jpeg"
	case "png":
		err = png.Encode(buf, resized)
		format = "png"
	case "gif":
		err = gif.Encode(buf, resized, nil)
		format = "gif"
	default:
		err = jpeg.Encode(buf, resized, &jpeg.Options{Quality: 85})
		format = "jpeg"
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to encode resized image: %w", err)
	}

	return buf, format, nil
}

func (r *Resizer) processGIF(img image.Image, width, height int, keepAspect bool) (io.Reader, string, error) {
	buf := new(bytes.Buffer)

	if keepAspect {
		bounds := img.Bounds()
		origWidth := bounds.Dx()
		origHeight := bounds.Dy()

		ratio := float64(origWidth) / float64(origHeight)
		newHeight := int(float64(width) / ratio)

		resized := resizeImage(img, width, newHeight)
		err := gif.Encode(buf, resized, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to encode gif: %w", err)
		}
	} else {
		resized := resizeImage(img, width, height)
		err := gif.Encode(buf, resized, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to encode gif: %w", err)
		}
	}

	return buf, "gif", nil
}

func resizeImage(img image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}
