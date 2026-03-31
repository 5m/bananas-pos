package transform

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"

	"bananas-pos/internal/job"
)

const (
	TransformEpsonESCPOS = "epson-escpos"
)

var escposFullCut = []byte{0x1d, 0x56, 0x00}
var escposFeedBeforeCut = []byte{'\n', '\n', '\n'}

var previewFetcher = func(ctx context.Context, zpl string, dpmm int) ([]byte, error) {
	client := &http.Client{}
	return FetchLabelaryPreview(ctx, client, zpl, dpmm)
}

func Apply(ctx context.Context, printJob job.PrintJob, selected string) (job.PrintJob, error) {
	switch selected {
	case "":
		return printJob, nil
	case TransformEpsonESCPOS:
		return toEpsonESCPOS(ctx, printJob)
	default:
		return printJob, nil
	}
}

func toEpsonESCPOS(ctx context.Context, printJob job.PrintJob) (job.PrintJob, error) {
	pngBytes, err := previewFetcher(ctx, string(printJob.Raw), DefaultRenderDPMM)
	if err != nil {
		return job.PrintJob{}, fmt.Errorf("render zpl preview: %w", err)
	}

	imageValue, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return job.PrintJob{}, fmt.Errorf("decode preview png: %w", err)
	}

	printJob.Raw = encodeESCPOSRaster(imageValue)
	printJob.ContentType = "application/octet-stream"
	return printJob, nil
}

func encodeESCPOSRaster(img image.Image) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	widthBytes := (width + 7) / 8
	data := make([]byte, widthBytes*height)

	for y := 0; y < height; y++ {
		rowOffset := y * widthBytes
		for x := 0; x < width; x++ {
			if !isBlackPixel(img.At(bounds.Min.X+x, bounds.Min.Y+y)) {
				continue
			}
			byteIndex := rowOffset + x/8
			bit := uint(7 - (x % 8))
			data[byteIndex] |= 1 << bit
		}
	}

	output := make([]byte, 0, len(data)+16)
	output = append(output, 0x1b, 0x40)
	output = append(output, 0x1d, 0x76, 0x30, 0x00)
	output = append(output, byte(widthBytes), byte(widthBytes>>8))
	output = append(output, byte(height), byte(height>>8))
	output = append(output, data...)
	output = append(output, escposFeedBeforeCut...)
	output = append(output, escposFullCut...)
	return output
}

func isBlackPixel(c color.Color) bool {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return false
	}

	alpha := float64(a) / 65535.0
	red := 255.0 - ((255.0 - float64(r>>8)) * alpha)
	green := 255.0 - ((255.0 - float64(g>>8)) * alpha)
	blue := 255.0 - ((255.0 - float64(b>>8)) * alpha)
	luminance := 0.299*red + 0.587*green + 0.114*blue
	return luminance < 160
}
