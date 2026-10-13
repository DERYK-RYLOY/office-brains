package main

import (
	"os"
	"path/filepath"

	"github.com/you/office-brains/internal/oir"
	"github.com/you/office-brains/internal/surgeon"
)

// tryTransplant clones dir/blank.{docx,xlsx,pptx} and swaps safe parts.
// Returns (blob, note, ok). Never fails the build: ok=false means clean fallback.
func tryTransplant(dir, docType string, spec *oir.Spec, parts map[string][]byte) ([]byte, string, bool) {
	if need, reason := spec.NeedsClean(); need {
		return nil, "needs clean: " + reason, false
	}
	blank := filepath.Join(dir, "blank."+docType)
	gb, err := os.ReadFile(blank)
	if err != nil {
		// also try golden.<ext> naming
		blank2 := filepath.Join(dir, "golden."+docType)
		gb, err = os.ReadFile(blank2)
		if err != nil {
			return nil, "no " + blank + " (drop a real M365 blank there)", false
		}
		blank = blank2
	}
	safe := surgeon.SafeReplace(docType, parts)
	blob, err := surgeon.Transplant(gb, safe)
	if err != nil {
		return nil, "transplant failed: " + err.Error(), false
	}
	return blob, "cloned " + blank, true
}
