// Diff command: part-level OOXML diff, read-only, never mutates inputs.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"os"
	"path/filepath"
	"sort"
)

func cmdDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	a := fs.String("a", "", "first file (e.g. real.docx)")
	b := fs.String("b", "", "second file (e.g. generated.docx)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *a == "" || *b == "" {
		emitErr("diff requires --a and --b", nil)
		os.Exit(2)
	}
	absA, _ := filepath.Abs(*a)
	absB, _ := filepath.Abs(*b)
	ma, err := zipMap(absA)
	if err != nil {
		emitErr("read a failed", err)
		os.Exit(1)
	}
	mb, err := zipMap(absB)
	if err != nil {
		emitErr("read b failed", err)
		os.Exit(1)
	}
	keys := map[string]bool{}
	for k := range ma {
		keys[k] = true
	}
	for k := range mb {
		keys[k] = true
	}
	sorted := []string{}
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	type row struct {
		Part    string `json:"part"`
		Status  string `json:"status"` // identical|changed|only_in_a|only_in_b
		BytesA  int    `json:"bytes_a"`
		BytesB  int    `json:"bytes_b"`
		ShaA    string `json:"sha_a,omitempty"`
		ShaB    string `json:"sha_b,omitempty"`
		Ignored bool   `json:"ignored,omitempty"` // docProps often differ by design
	}
	rows := []row{}
	for _, k := range sorted {
		ba, oka := ma[k]
		bb, okb := mb[k]
		r := row{Part: k, BytesA: len(ba), BytesB: len(bb)}
		if oka {
			r.ShaA = shortHash(ba)
		}
		if okb {
			r.ShaB = shortHash(bb)
		}
		switch {
		case oka && okb && r.ShaA == r.ShaB:
			r.Status = "identical"
		case oka && okb:
			r.Status = "changed"
		case oka:
			r.Status = "only_in_a"
		default:
			r.Status = "only_in_b"
		}
		if len(k) >= 8 && k[:8] == "docProps" {
			r.Ignored = true // forger intentionally changes these
		}
		rows = append(rows, r)
	}
	emitJSON(map[string]any{"ok": true, "command": "diff", "data": map[string]any{
		"a": absA, "b": absB, "parts": rows,
	}})
}

func zipMap(path string) (map[string][]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		buf := make([]byte, 0, f.UncompressedSize64+16)
		tmp := make([]byte, 32768)
		for {
			n, rerr := rc.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if rerr != nil {
				break
			}
			if len(buf) > 100<<20 {
				break
			}
		}
		rc.Close()
		out[f.Name] = buf
	}
	return out, nil
}

func shortHash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}
