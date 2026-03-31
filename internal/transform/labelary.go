package transform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPreviewWidthPx  = 228
	defaultPreviewHeightPx = 140
	DefaultRenderDPMM      = 8
)

func FetchLabelaryPreview(ctx context.Context, client *http.Client, zpl string, dpmm int) ([]byte, error) {
	widthMM, heightMM := labelSizeMM(zpl, dpmm)
	previewURL := zebraPreviewURL(zpl, dpmm, widthMM, heightMM)

	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, previewURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			return body, nil
		}

		retryDelay := retryAfterDelay(resp.Header.Get("Retry-After"))
		statusErr := fmt.Errorf("labelary preview failed: %s", resp.Status)
		resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests || attempt == 2 {
			return nil, statusErr
		}

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, nil
}

func zebraPreviewURL(zpl string, dpmm int, widthMM, heightMM float64) string {
	return fmt.Sprintf(
		"https://api.labelary.com/v1/printers/%ddpmm/labels/%sx%s/0/%s",
		dpmm,
		strconv.FormatFloat(widthMM/25.4, 'f', -1, 64),
		strconv.FormatFloat(heightMM/25.4, 'f', -1, 64),
		url.PathEscape(zpl),
	)
}

func retryAfterDelay(value string) time.Duration {
	if value == "" {
		return 2 * time.Second
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 2 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func labelSizeMM(zpl string, dpmm int) (float64, float64) {
	widthDots := zplCommandInt(zpl, "^PW")
	heightDots := zplCommandInt(zpl, "^LL")

	widthMM := float64(defaultPreviewWidthPx*2) / float64(dpmm)
	if widthDots > 0 {
		widthMM = float64(widthDots) / float64(dpmm)
	}

	heightMM := float64(defaultPreviewHeightPx*2) / float64(dpmm)
	if heightDots > 0 {
		heightMM = float64(heightDots) / float64(dpmm)
	}

	return widthMM, heightMM
}

func LabelSizeMM(zpl string, dpmm int) (float64, float64) {
	return labelSizeMM(zpl, dpmm)
}

func zplCommandInt(zpl, command string) int {
	index := strings.Index(zpl, command)
	if index < 0 {
		return 0
	}

	start := index + len(command)
	end := start
	for end < len(zpl) {
		ch := zpl[end]
		if ch < '0' || ch > '9' {
			break
		}
		end++
	}
	if end == start {
		return 0
	}

	value, err := strconv.Atoi(zpl[start:end])
	if err != nil {
		return 0
	}
	return value
}
