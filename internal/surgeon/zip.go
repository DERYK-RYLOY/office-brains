// Package surgeon builds deterministic Office ZIPs.
// Phase 0.5: build from parts map. Phase 1: clone golden ZIP then replace parts.
package surgeon

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"time"
)

// DeterministicTime is used for ZIP entry timestamps so builds are byte-stable.
// Internal docProps times (created/modified) stay human-like; only ZIP headers are fixed.
func DeterministicTime() time.Time {
	return time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
}

// Build assembles parts (name -> xml bytes) into a deterministic ZIP.
// Names must be like "[Content_Types].xml", "_rels/.rels", "word/document.xml".
func Build(parts map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(parts))
	for k := range parts {
		names = append(names, k)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range names {
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		h.SetModTime(DeterministicTime())
		// Office compat: files non-empty get at least read perms
		h.SetMode(0o644)
		fw, err := w.CreateHeader(h)
		if err != nil {
			_ = w.Close()
			return nil, err
		}
		if _, err := fw.Write(parts[name]); err != nil {
			_ = w.Close()
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// List returns sorted part names (for inspect/diff).
func List(parts map[string][]byte) []string {
	names := make([]string, 0, len(parts))
	for k := range parts {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Transplant clones a golden Office ZIP and surgically replaces parts.
// Everything not in replace passes through byte-identical (rels, theme,
// styles, settings stay Microsoft-original). Output is deterministic.
func Transplant(golden []byte, replace map[string][]byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(golden), int64(len(golden)))
	if err != nil {
		return nil, fmt.Errorf("golden unzip: %w", err)
	}
	merged := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("golden read %s: %w", f.Name, err)
		}
		b, err := io.ReadAll(io.LimitReader(rc, 100<<20))
		rc.Close()
		if err != nil {
			return nil, err
		}
		merged[f.Name] = b
	}
	for k, v := range replace {
		merged[k] = v
	}
	return Build(merged)
}

// UnzipMap reads a ZIP blob into name->bytes (tests, inspect).
func UnzipMap(blob []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 100<<20))
		rc.Close()
		if err != nil {
			return nil, err
		}
		out[f.Name] = b
	}
	return out, nil
}
// document content + shared strings + metadata. Content-Types, rels, styles,
// theme, media, drawings and charts always stay golden-original.
func SafeReplace(docType string, parts map[string][]byte) map[string][]byte {
	keep := map[string]bool{}
	switch docType {
	case "docx":
		keep["word/document.xml"] = true
	case "xlsx":
		for k := range parts {
			if len(k) >= 14 && k[:14] == "xl/worksheets/" && k[len(k)-4:] == ".xml" && !contains(k, "_rels") {
				keep[k] = true
			}
		}
		keep["xl/sharedStrings.xml"] = true
	case "pptx":
		for k := range parts {
			if len(k) >= 11 && k[:11] == "ppt/slides/" && k[len(k)-4:] == ".xml" && !contains(k, "_rels") {
				keep[k] = true
			}
		}
	}
	keep["docProps/core.xml"] = true
	keep["docProps/app.xml"] = true
	out := map[string][]byte{}
	for k, v := range parts {
		if keep[k] {
			out[k] = v
		}
	}
	return out
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
