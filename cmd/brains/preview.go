package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/you/office-brains/internal/oir"
	"github.com/you/office-brains/internal/preview"
)

// preview: OIR -> self-contained HTML. Pure Go, offline, zero deps.
// Same constants as compilers so widths/positions match emitted OOXML.
func cmdPreview(args []string) {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "", "OIR spec JSON file")
	docType := fs.String("type", "docx", "docx|xlsx|pptx")
	out := fs.String("o", "", "output HTML path")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *specPath == "" || *out == "" {
		emitErr("preview requires --spec and -o", nil)
		os.Exit(2)
	}
	data, err := os.ReadFile(*specPath)
	if err != nil {
		emitErr("read spec failed", err)
		os.Exit(2)
	}
	spec, verr := oir.ParseAndValidate(data)
	if verr != nil {
		emitErr("invalid OIR spec", verr)
		os.Exit(2)
	}
	html := preview.Render(spec, *docType)
	absOut, _ := filepath.Abs(*out)
	if err := os.WriteFile(absOut, []byte(html), 0o644); err != nil {
		emitErr("write failed", err)
		os.Exit(1)
	}
	logHuman("preview %s (%d bytes) -> open in browser", absOut, len(html))
	emitJSON(map[string]any{"ok": true, "command": "preview", "data": map[string]any{
		"out": absOut, "bytes": len(html), "type": *docType, "blocks": spec.CountBlocks(),
	}})
}
