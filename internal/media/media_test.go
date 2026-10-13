package media

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

const png1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestLoadPNG(t *testing.T) {
	b, _ := base64.StdEncoding.DecodeString(png1x1)
	p := filepath.Join(t.TempDir(), "r.png")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	im, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if im.Ext != "png" || im.WPx != 1 || im.HPx != 1 {
		t.Fatalf("%+v", im)
	}
}

func TestRejectText(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.txt")
	_ = os.WriteFile(p, []byte("hello world, not an image file at all"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("want error for non-image")
	}
}

func TestEMUSize(t *testing.T) {
	cx, cy := EMUSize(2.0, 100, 50)
	if cx != 2*914400 || cy != cx*50/100 {
		t.Fatalf("cx=%d cy=%d", cx, cy)
	}
}
