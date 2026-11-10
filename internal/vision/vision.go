// Package vision gives the agent eyes without a VLM.
// Level 1 (here, stdlib, no network): math issues computed from the same
// constants as the emitters - overflow percentages, cover-ups, drops.
// Level 2 (cmd/brains shot+see): real PNGs via headless Edge for VLM review.
// Level 3 (oracle/diff_png.py): Pillow before/after diff of two PNGs.
package vision

import (
	"fmt"
	"strings"

	"github.com/DERYK-RYLOY/office-brains/internal/compiler"
	"github.com/DERYK-RYLOY/office-brains/internal/layout"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

// Issue is one machine-readable layout finding.
type Issue struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"` // error|warn|info
	Detail   string `json:"detail"`
}

// Issues audits a spec with emitter-exact math. Zero issues = clean bill.
func Issues(spec *oir.Spec, docType string) []Issue {
	out := []Issue{}
	switch docType {
	case "docx":
		for si, sec := range spec.Doc.Sections {
			for bi, bl := range sec.Blocks {
				if bl.T != "table" || len(bl.Rows) == 0 {
					continue
				}
				ncols := len(bl.Rows[0])
				widths := layout.WordTableColWidths(9360, ncols)
				sum := 0
				for _, w := range widths {
					sum += w
				}
				// grid always sums to 9360 by construction; flag absurd density instead
				avgPx := sum / ncols / 15
				if ncols > 8 {
					out = append(out, Issue{"docx.many_cols", "warn",
						fmt.Sprintf("sections[%d].blocks[%d]: %d cols (~%dpx each) - text will wrap hard", si, bi, ncols, avgPx)})
				}
				for ri, r := range bl.Rows {
					for ci, c := range r {
						if s, ok := c.V.(string); ok && len(s) > 120 {
							out = append(out, Issue{"docx.long_cell", "warn",
								fmt.Sprintf("sections[%d].blocks[%d].rows[%d][%d]: %d chars in fixed-layout cell - will wrap tall", si, bi, ri, ci, len(s))})
						}
					}
				}
			}
		}
	case "xlsx":
		for si, sec := range spec.Doc.Sections {
			rows, _ := flatRows(sec)
			ncols := 0
			for _, r := range rows {
				if len(r) > ncols {
					ncols = len(r)
				}
			}
			widths := make([]float64, ncols)
			for c := 0; c < ncols; c++ {
				col := []string{}
				for _, r := range rows {
					if c < len(r) {
						col = append(col, disp(r[c]))
					} else {
						col = append(col, "")
					}
				}
				widths[c] = layout.MaxWidth(col, true)
			}
			total := 0.0
			for i, w := range widths {
				total += w
				if w > 50 {
					out = append(out, Issue{"xlsx.wide_col", "warn",
						fmt.Sprintf("sheet %d col %s: %.1f chars - dominates print width", si+1, colName(i), w)})
				}
			}
			if total > 200 {
				out = append(out, Issue{"xlsx.wide_grid", "warn",
					fmt.Sprintf("sheet %d: total %.0f chars - will not fit landscape print", si+1, total)})
			}
		}
	case "pptx":
		for si, sec := range spec.Doc.Sections {
			// reuse the emitter's layout pass verbatim so audit, emitter and
			// preview can never disagree; translate its warnings into issues
			mc, cc := 0, 0
			vimgs, vcharts := compiler.LayoutSlideVisuals(sec, si, &mc, &cc)
			nTables := 0
			for _, bl := range sec.Blocks {
				if bl.T == "table" {
					nTables++
				}
			}
			for _, w := range compiler.Truncated {
				if strings.Contains(w, "dropped") {
					out = append(out, Issue{"pptx.visual_dropped", "error", w})
				} else if strings.Contains(w, "scaled") {
					out = append(out, Issue{"pptx.visual_scaled", "warn", w})
				} else if strings.Contains(w, "not embedded") {
					out = append(out, Issue{"pptx.image_missing", "warn", w})
				}
			}
			compiler.ResetTruncated()
			if nVisuals := nTables + len(vimgs) + len(vcharts); nVisuals > 2 {
				out = append(out, Issue{"pptx.crowded", "warn",
					fmt.Sprintf("section %d: %d visuals on one slide - tight but fits", si+1, nVisuals)})
			}
		}
	}
	if len(out) == 0 {
		out = append(out, Issue{"layout.clean", "info", "no overflow, cover-ups or drops detected by math audit"})
	}
	return out
}

func flatRows(sec oir.Section) ([][]oir.Cell, bool) {
	out := [][]oir.Cell{}
	for _, bl := range sec.Blocks {
		switch bl.T {
		case "h1", "h2", "h3", "p":
			out = append(out, []oir.Cell{{V: bl.Txt}})
		case "bullet":
			out = append(out, []oir.Cell{{V: "•  " + bl.Txt}})
		case "quote":
			out = append(out, []oir.Cell{{V: "“" + bl.Txt + "”"}})
		case "divider":
			out = append(out, []oir.Cell{{V: "────────────────────────"}})
		case "code":
			out = append(out, []oir.Cell{{V: bl.Txt}})
		case "image":
			out = append(out, []oir.Cell{{V: "[image]"}})
		case "chart":
			out = append(out, []oir.Cell{{V: "[chart]"}})
		case "table":
			for _, r := range bl.Rows {
				cp := make([]oir.Cell, len(r))
				copy(cp, r)
				out = append(out, cp)
			}
			out = append(out, []oir.Cell{{V: ""}})
		}
	}
	return out, true
}

func disp(c oir.Cell) string {
	if c.V == nil {
		return ""
	}
	if s, ok := c.V.(string); ok {
		return s
	}
	return fmt.Sprint(c.V)
}

func colName(i int) string {
	s := ""
	i++
	for i > 0 {
		i--
		s = string(rune('A'+(i%26))) + s
		i /= 26
	}
	return s
}
