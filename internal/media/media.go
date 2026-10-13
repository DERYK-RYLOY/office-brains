// Package media loads PNG/JPEG for embedding (stdlib only).
package media

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

// Image is a loaded picture.
type Image struct {
	Data []byte
	Ext  string // png|jpg
	WPx  int
	HPx  int
}

// Load reads path (absolute preferred), sniffs png/jpeg, returns dims.
func Load(path string) (*Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ERR_MEDIA_READ: %v", err)
	}
	if len(b) < 8 {
		return nil, fmt.Errorf("ERR_MEDIA_TINY: %s", path)
	}
	ext := ""
	if bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) {
		ext = "png"
	} else if bytes.HasPrefix(b, []byte("\xff\xd8\xff")) {
		ext = "jpg"
	} else {
		// fallback by extension
		low := strings.ToLower(filepath.Ext(path))
		if low == ".png" {
			ext = "png"
		} else if low == ".jpg" || low == ".jpeg" {
			ext = "jpg"
		} else {
			return nil, fmt.Errorf("ERR_MEDIA_TYPE: need PNG/JPEG, got %s", path)
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("ERR_MEDIA_DECODE: %v", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8000 || cfg.Height > 8000 {
		return nil, fmt.Errorf("ERR_MEDIA_DIMS: bad %dx%d", cfg.Width, cfg.Height)
	}
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("ERR_MEDIA_BIG: max 8MB, got %d", len(b))
	}
	_ = image.Point{}
	return &Image{Data: b, Ext: ext, WPx: cfg.Width, HPx: cfg.Height}, nil
}

// EMUSize converts width inches + aspect to EMU pair. 914400 EMU = 1 inch.
func EMUSize(wIn float64, wPx, hPx int) (cx, cy int64) {
	if wIn <= 0 {
		wIn = 2.0
	}
	cx = int64(wIn * 914400)
	cy = int64(float64(cx) * float64(hPx) / float64(wPx))
	return cx, cy
}
