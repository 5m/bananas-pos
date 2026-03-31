package transform

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"bananas-pos/internal/job"
)

func TestEncodeESCPOSRasterPacksPixelsIntoBytes(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		img.Set(x, 0, color.White)
	}
	img.Set(0, 0, color.Black)
	img.Set(2, 0, color.Black)
	img.Set(7, 0, color.Black)

	encoded := encodeESCPOSRaster(img)
	if len(encoded) < 14 {
		t.Fatalf("encoded raster too short: %d", len(encoded))
	}

	if got := encoded[10]; got != 0xA1 {
		t.Fatalf("expected raster byte 0xA1, got 0x%02X", got)
	}
	feedStart := len(encoded) - len(escposFullCut) - len(escposFeedBeforeCut)
	if got := encoded[feedStart : feedStart+len(escposFeedBeforeCut)]; !bytes.Equal(got, escposFeedBeforeCut) {
		t.Fatalf("expected feed-before-cut suffix %v, got %v", escposFeedBeforeCut, got)
	}
	if got := encoded[len(encoded)-3:]; !bytes.Equal(got, escposFullCut) {
		t.Fatalf("expected full cut suffix %v, got %v", escposFullCut, got)
	}
}

func TestApplyConvertsToBinaryESCPOSTraffic(t *testing.T) {
	originalFetcher := previewFetcher
	t.Cleanup(func() {
		previewFetcher = originalFetcher
	})

	img := image.NewNRGBA(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		img.Set(x, 0, color.White)
	}
	img.Set(0, 0, color.Black)

	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	previewFetcher = func(context.Context, string, int) ([]byte, error) {
		return pngBuffer.Bytes(), nil
	}

	printJob, err := Apply(context.Background(), job.PrintJob{
		Raw:         []byte("^XA^FO0,0^FDTEST^FS^XZ"),
		ContentType: "application/zpl",
	}, TransformEpsonESCPOS)
	if err != nil {
		t.Fatalf("apply transform: %v", err)
	}

	if got := printJob.ContentType; got != "application/octet-stream" {
		t.Fatalf("expected application/octet-stream, got %q", got)
	}
	if len(printJob.Raw) < 10 || printJob.Raw[0] != 0x1b || printJob.Raw[1] != 0x40 {
		t.Fatalf("expected ESC/POS init prefix, got %v", printJob.Raw)
	}
	feedStart := len(printJob.Raw) - len(escposFullCut) - len(escposFeedBeforeCut)
	if got := printJob.Raw[feedStart : feedStart+len(escposFeedBeforeCut)]; !bytes.Equal(got, escposFeedBeforeCut) {
		t.Fatalf("expected feed-before-cut suffix %v, got %v", escposFeedBeforeCut, got)
	}
	if got := printJob.Raw[len(printJob.Raw)-3:]; !bytes.Equal(got, escposFullCut) {
		t.Fatalf("expected full cut suffix %v, got %v", escposFullCut, got)
	}
}
