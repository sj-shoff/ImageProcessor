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
	"math"
	"strconv"
	"strings"

	"image-processor/internal/domain"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

type Watermarker struct {
	font *truetype.Font
}

func NewWatermarker() *Watermarker {
	fontBytes := goregular.TTF
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return &Watermarker{}
	}
	return &Watermarker{
		font: f,
	}
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
		fontSize = 36
	}

	fontColor, ok := params["font_color"].(string)
	if !ok {
		fontColor = "255,255,255"
	}

	watermarked, err := w.addTextWatermark(img, text, position, opacity, fontSize, fontColor)
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
	case "gif":
		err = jpeg.Encode(buf, watermarked, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	default:
		err = jpeg.Encode(buf, watermarked, &jpeg.Options{Quality: domain.DefaultJPEGQuality})
		format = "jpeg"
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to encode watermarked image: %w", err)
	}

	return buf, format, nil
}

func (w *Watermarker) addTextWatermark(img image.Image, text, position string, opacity, fontSize float64, fontColorStr string) (image.Image, error) {
	if w.font == nil {
		return nil, fmt.Errorf("font not loaded")
	}

	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, img, image.Point{}, draw.Src)

	col, err := parseColor(fontColorStr, opacity)
	if err != nil {
		fmt.Printf("Color parse error: %v, using black\n", err)
		col = color.RGBA{0, 0, 0, uint8(255 * opacity)}
	}

	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(w.font)
	c.SetFontSize(fontSize)
	c.SetClip(result.Bounds())
	c.SetDst(result)
	c.SetSrc(image.NewUniform(col))

	face := truetype.NewFace(w.font, &truetype.Options{
		Size: fontSize,
		DPI:  72,
	})

	var textWidth fixed.Int26_6
	for _, x := range text {
		awidth, ok := face.GlyphAdvance(x)
		if ok {
			textWidth += awidth
		}
	}

	textHeight := fixed.Int26_6(fontSize * 64 * 1.2)

	widthPx := int(textWidth.Ceil())
	heightPx := int(textHeight.Ceil())

	fmt.Printf("Text '%s': width=%dpx, height=%dpx\n", text, widthPx, heightPx)
	fmt.Printf("Image size: %dx%d\n", bounds.Dx(), bounds.Dy())

	margin := 20
	var pt fixed.Point26_6

	switch domain.WatermarkPosition(position) {
	case domain.WatermarkTopLeft:
		pt = freetype.Pt(margin, margin+heightPx)
		fmt.Println("Position: top-left")
	case domain.WatermarkTopRight:
		pt = freetype.Pt(bounds.Dx()-widthPx-margin, margin+heightPx)
		fmt.Println("Position: top-right")
	case domain.WatermarkTopCenter:
		pt = freetype.Pt((bounds.Dx()-widthPx)/2, margin+heightPx)
		fmt.Println("Position: top-center")
	case domain.WatermarkBottomLeft:
		pt = freetype.Pt(margin, bounds.Dy()-margin)
		fmt.Println("Position: bottom-left")
	case domain.WatermarkBottomRight:
		pt = freetype.Pt(bounds.Dx()-widthPx-margin, bounds.Dy()-margin)
		fmt.Println("Position: bottom-right")
	case domain.WatermarkBottomCenter:
		pt = freetype.Pt((bounds.Dx()-widthPx)/2, bounds.Dy()-margin)
		fmt.Println("Position: bottom-center")
	case domain.WatermarkCenter:
		pt = freetype.Pt((bounds.Dx()-widthPx)/2, (bounds.Dy()+heightPx)/2)
		fmt.Println("Position: center")
	default:
		pt = freetype.Pt(bounds.Dx()-widthPx-margin, bounds.Dy()-margin)
		fmt.Println("Position: default (bottom-right)")
	}

	fmt.Printf("Drawing at: x=%d, y=%d\n", pt.X.Ceil(), pt.Y.Ceil())

	c.SetFontSize(fontSize)
	_, err = c.DrawString(text, pt)
	if err != nil {
		return nil, fmt.Errorf("failed to draw watermark text: %w", err)
	}

	fmt.Println("Watermark drawn successfully")
	return result, nil
}

func parseColor(colorStr string, opacity float64) (color.RGBA, error) {
	colorStr = strings.ReplaceAll(colorStr, " ", "")
	parts := strings.Split(colorStr, ",")

	if len(parts) != 3 && len(parts) != 4 {
		return color.RGBA{255, 255, 255, uint8(255 * opacity)}, fmt.Errorf("invalid color format")
	}

	r, err1 := strconv.Atoi(parts[0])
	g, err2 := strconv.Atoi(parts[1])
	b, err3 := strconv.Atoi(parts[2])

	if err1 != nil || err2 != nil || err3 != nil {
		return color.RGBA{255, 255, 255, uint8(255 * opacity)}, fmt.Errorf("invalid color values")
	}

	r = clamp(r, 0, 255)
	g = clamp(g, 0, 255)
	b = clamp(b, 0, 255)

	var a uint8
	if len(parts) == 4 {
		aVal, err := strconv.Atoi(parts[3])
		if err == nil {
			a = uint8(clamp(aVal, 0, 255))
		} else {
			a = uint8(255 * opacity)
		}
	} else {
		a = uint8(255 * opacity)
	}

	return color.RGBA{uint8(r), uint8(g), uint8(b), a}, nil
}

func clamp(value, min, max int) int {
	return int(math.Max(float64(min), math.Min(float64(max), float64(value))))
}
