package operations

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"image-processor/internal/domain"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

type Watermarker struct{}

func NewWatermarker() *Watermarker {
	return &Watermarker{}
}

func (w *Watermarker) Process(ctx context.Context, img image.Image, format string, params map[string]interface{}) (io.Reader, string, error) {
	text, ok := params["text"].(string)
	if !ok || text == "" {
		text = domain.DefaultWatermarkText
	}

	opacity, ok := params["opacity"].(float64)
	if !ok || opacity <= 0 {
		opacity = domain.DefaultWatermarkOpacity
	}

	position, ok := params["position"].(string)
	if !ok {
		position = string(domain.WatermarkBottomRight)
	}

	fontSize, ok := params["font_size"].(float64)
	if !ok || fontSize <= 0 {
		fontSize = 24
	}

	fontColor, ok := params["font_color"].(string)
	if !ok {
		fontColor = "255,255,255,255"
	}

	watermarked, err := w.addTextWatermark(img, text, position, opacity, int(fontSize), fontColor)
	if err != nil {
		return nil, "", fmt.Errorf("failed to add watermark: %w", err)
	}

	buf := new(bytes.Buffer)

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		err = jpeg.Encode(buf, watermarked, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	case "png":
		err = png.Encode(buf, watermarked)
		format = "png"
	default:
		err = jpeg.Encode(buf, watermarked, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to encode watermarked image: %w", err)
	}

	return buf, format, nil
}

func (w *Watermarker) addTextWatermark(img image.Image, text, position string, opacity float64, fontSize int, fontColorStr string) (image.Image, error) {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, img, image.Point{}, draw.Src)

	fontBytes := goregular.TTF
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}

	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(float64(fontSize))

	col, err := parseColor(fontColorStr, opacity)
	if err != nil {
		col = color.RGBA{255, 255, 255, uint8(255 * opacity)}
	}
	c.SetSrc(image.NewUniform(col))

	textWidth := len(text) * fontSize / 2
	textHeight := fontSize

	var pt fixed.Point26_6
	switch domain.WatermarkPosition(position) {
	case domain.WatermarkTopLeft:
		pt = freetype.Pt(10, 10+textHeight)
	case domain.WatermarkTopRight:
		pt = freetype.Pt(bounds.Dx()-textWidth-10, 10+textHeight)
	case domain.WatermarkTopCenter:
		pt = freetype.Pt((bounds.Dx()-textWidth)/2, 10+textHeight)
	case domain.WatermarkBottomLeft:
		pt = freetype.Pt(10, bounds.Dy()-10)
	case domain.WatermarkBottomRight:
		pt = freetype.Pt(bounds.Dx()-textWidth-10, bounds.Dy()-10)
	case domain.WatermarkBottomCenter:
		pt = freetype.Pt((bounds.Dx()-textWidth)/2, bounds.Dy()-10)
	case domain.WatermarkCenter:
		pt = freetype.Pt((bounds.Dx()-textWidth)/2, bounds.Dy()/2)
	default:
		pt = freetype.Pt(bounds.Dx()-textWidth-10, bounds.Dy()-10)
	}

	c.SetClip(result.Bounds())
	c.SetDst(result)
	_, err = c.DrawString(text, pt)
	if err != nil {
		return nil, fmt.Errorf("failed to draw watermark text: %w", err)
	}

	return result, nil
}

func parseColor(colorStr string, opacity float64) (color.RGBA, error) {
	var r, g, b, a uint8
	a = uint8(255 * opacity)

	_, err := fmt.Sscanf(colorStr, "%d,%d,%d,%d", &r, &g, &b, &a)
	if err != nil {
		_, err = fmt.Sscanf(colorStr, "%d,%d,%d", &r, &g, &b)
		if err != nil {
			return color.RGBA{255, 255, 255, a}, err
		}
	}

	return color.RGBA{r, g, b, a}, nil
}
