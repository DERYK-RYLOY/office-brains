// Package oir defines Office Intermediate Representation (v1 bootstrap).
// AI writes OIR JSON. Compiler turns it into OOXML. Never the reverse.
package oir

import (
	"encoding/json"
	"fmt"
	"math"
)

// CodedError is a stable machine-readable failure for agents.
// Code is stable (grep-able), Msg is human detail with path.
type CodedError struct {
	Code string
	Msg  string
}

func (e *CodedError) Error() string { return e.Msg }

// CodeOf extracts stable code or ERR_UNKNOWN.
func CodeOf(err error) string {
	if ce, ok := err.(*CodedError); ok {
		return ce.Code
	}
	return "ERR_UNKNOWN"
}

func coded(code, msg string, a ...any) *CodedError {
	return &CodedError{Code: code, Msg: fmt.Sprintf(msg, a...)}
}

const CurrentVersion = "1.1"
const OldVersion = "1.0"

// Spec is the top-level OIR document.
type Spec struct {
	OIR  string `json:"oir"`
	Doc  Doc   `json:"doc"`
	Meta *Meta `json:"meta,omitempty"`
}

// Meta controls forger profile + doc chrome.
type Meta struct {
	Profile string `json:"profile,omitempty"` // m365|corporate|clean
	Company string `json:"company,omitempty"`
	Author  string `json:"author,omitempty"`
	Header  string `json:"header,omitempty"` // docx header text (1.1)
	Footer  string `json:"footer,omitempty"` // docx footer text (1.1)
}

// Doc is a list of sections.
// docx: sections concatenate (single page flow). xlsx: one sheet per section.
// pptx: one slide per section.
type Doc struct {
	Sections []Section `json:"sections"`
}

// Section groups blocks.
type Section struct {
	ID     string  `json:"id,omitempty"`   // stable ref for patch/diff (1.1)
	Name   string  `json:"name,omitempty"` // sheet name / slide title fallback
	Blocks []Block `json:"blocks"`
}

// Block is one semantic unit.
type Block struct {
	ID     string        `json:"id,omitempty"` // stable ref for patch (1.1)
	T      string        `json:"t"`            // h1|h2|p|table|bullet|code|image|chart
	Txt    string        `json:"txt,omitempty"`
	Rows   [][]Cell      `json:"rows,omitempty"`
	Merge  []string      `json:"merge,omitempty"` // xlsx table merges like "A1:B1" (1.1)
	Img    string        `json:"img,omitempty"`   // image abs path (1.1)
	Alt    string        `json:"alt,omitempty"`   // image alt text
	W      float64       `json:"w,omitempty"`     // image width inches (1.1)
	Title  string        `json:"title,omitempty"` // chart title (1.2)
	Kind   string        `json:"kind,omitempty"`  // chart kind: bar|line|pie (default bar)
	Labels []string      `json:"labels,omitempty"`
	Series []ChartSeries `json:"series,omitempty"`
}

// ChartSeries is one data series in a chart block.
type ChartSeries struct {
	Name   string    `json:"name,omitempty"`
	Values []float64 `json:"values,omitempty"`
}

// Cell is a table cell: plain value or formula with cached value.
// Plain: "foo" | 123 | true | {"v":...}
// Formula: {"f":"SUM(A1:A10)","v":123} or {"f":"CONCAT(A1,B1)","v":"text"}
// Excel NEVER evaluates on open without cached <v>; we always require v with f.
type Cell struct {
	V any    `json:"v,omitempty"`
	F string `json:"f,omitempty"`
}

// UnmarshalJSON allows Cell to be "foo" or 123 or {"v":...} or {"f":...,"v":...}.
func (c *Cell) UnmarshalJSON(b []byte) error {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err == nil && (m["v"] != nil || m["f"] != nil) {
		c.V = m["v"]
		if f, ok := m["f"].(string); ok {
			c.F = f
		}
		if c.F != "" && c.V == nil {
			return fmt.Errorf("formula cell needs cached v, got f=%q without v", c.F)
		}
		if c.F != "" {
			switch c.V.(type) {
			case string, float64, bool:
				return nil
			default:
				return fmt.Errorf("formula cached v must be string|number|bool")
			}
		}
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v.(type) {
	case string, float64, bool, nil:
		c.V = v
		return nil
	default:
		return fmt.Errorf("cell must be string|number|bool|{v}, got %T", v)
	}
}

// ParseAndValidate parses and validates OIR 1.0 (compat) and 1.1 rules.
// All failures are *CodedError with stable ERR_OIR_* codes.
func ParseAndValidate(data []byte) (*Spec, error) {
	var s Spec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, coded("ERR_OIR_JSON", "invalid JSON: %v", err)
	}
	if s.OIR == "" {
		return nil, coded("ERR_OIR_VERSION_MISSING", `missing "oir" version, expected "1.0" or "1.1"`)
	}
	if s.OIR != CurrentVersion && s.OIR != OldVersion {
		return nil, coded("ERR_OIR_VERSION", "unsupported oir version %q, want %q", s.OIR, CurrentVersion)
	}
	if len(s.Doc.Sections) == 0 {
		return nil, coded("ERR_OIR_NO_SECTIONS", `"doc.sections" must have >=1 section`)
	}
	if len(s.Doc.Sections) > 50 {
		return nil, coded("ERR_OIR_TOO_MANY_SECTIONS", "max 50 sections (sheets/slides), got %d", len(s.Doc.Sections))
	}
	seenIDs := map[string]bool{}
	nImages, nCharts := 0, 0
	for si, sec := range s.Doc.Sections {
		if sec.ID != "" {
			if seenIDs["s:"+sec.ID] {
				return nil, coded("ERR_OIR_DUP_ID", "duplicate section id %q", sec.ID)
			}
			seenIDs["s:"+sec.ID] = true
		}
		if len(sec.Blocks) == 0 {
			return nil, coded("ERR_OIR_EMPTY_BLOCKS", "sections[%d].blocks must have >=1 block", si)
		}
		for bi, b := range sec.Blocks {
			if b.ID != "" {
				k := "b:" + b.ID
				if seenIDs[k] {
					return nil, coded("ERR_OIR_DUP_ID", "duplicate block id %q", b.ID)
				}
				seenIDs[k] = true
			}
			switch b.T {
			case "h1", "h2", "p", "bullet", "code":
				if b.Txt == "" {
					return nil, coded("ERR_OIR_MISSING_TXT", "sections[%d].blocks[%d]: type %q needs txt", si, bi, b.T)
				}
				if len(b.Txt) > 100000 {
					return nil, coded("ERR_OIR_TXT_TOO_BIG", "sections[%d].blocks[%d]: txt max 100000 chars, got %d", si, bi, len(b.Txt))
				}
			case "image":
				nImages++
				if nImages > 20 {
					return nil, coded("ERR_OIR_TOO_MANY_IMAGES", "max 20 image blocks, got >20")
				}
				if b.Img == "" {
					return nil, coded("ERR_OIR_IMAGE_PATH", "sections[%d].blocks[%d]: image needs img path", si, bi)
				}
				if b.W != 0 && (b.W < 0.3 || b.W > 7.5) {
					return nil, coded("ERR_OIR_IMAGE_SIZE", "sections[%d].blocks[%d]: w inches 0.3..7.5, got %v", si, bi, b.W)
				}
			case "chart":
				nCharts++
				if nCharts > 10 {
					return nil, coded("ERR_OIR_TOO_MANY_CHARTS", "max 10 chart blocks, got >10")
				}
				if b.Kind != "" && b.Kind != "bar" && b.Kind != "line" && b.Kind != "pie" {
					return nil, coded("ERR_OIR_CHART_KIND", "sections[%d].blocks[%d]: kind must be bar|line|pie, got %q", si, bi, b.Kind)
				}
				if len(b.Labels) == 0 || len(b.Labels) > 20 {
					return nil, coded("ERR_OIR_CHART_LABELS", "sections[%d].blocks[%d]: chart needs 1..20 labels", si, bi)
				}
				if len(b.Series) == 0 || len(b.Series) > 8 {
					return nil, coded("ERR_OIR_CHART_SERIES", "sections[%d].blocks[%d]: chart needs 1..8 series", si, bi)
				}
				for sei, se := range b.Series {
					if len(se.Values) != len(b.Labels) {
						return nil, coded("ERR_OIR_CHART_RAGGED", "sections[%d].blocks[%d].series[%d]: %d values vs %d labels", si, bi, sei, len(se.Values), len(b.Labels))
					}
					for vi, v := range se.Values {
						if math.IsNaN(v) || math.IsInf(v, 0) {
							return nil, coded("ERR_OIR_BAD_NUMBER", "sections[%d].blocks[%d].series[%d].values[%d]: non-finite number", si, bi, sei, vi)
						}
					}
				}
			case "table":
				if len(b.Rows) < 1 {
					return nil, coded("ERR_OIR_TABLE_NO_ROWS", "sections[%d].blocks[%d]: table needs >=1 row", si, bi)
				}
				w := len(b.Rows[0])
				if w == 0 || w > 20 {
					return nil, coded("ERR_OIR_TABLE_WIDTH", "sections[%d].blocks[%d]: table width 1..20, got %d", si, bi, w)
				}
				for ri, r := range b.Rows {
					if len(r) != w {
						return nil, coded("ERR_OIR_RAGGED_TABLE", "sections[%d].blocks[%d].rows[%d]: ragged %d vs %d", si, bi, ri, len(r), w)
					}
					for ci, c := range r {
						if f, ok := c.V.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
							return nil, coded("ERR_OIR_BAD_NUMBER", "sections[%d].blocks[%d].rows[%d][%d]: non-finite number", si, bi, ri, ci)
						}
						if c.F != "" {
							if f, ok := c.V.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
								return nil, coded("ERR_OIR_BAD_NUMBER", "sections[%d].blocks[%d].rows[%d][%d]: non-finite cached value", si, bi, ri, ci)
							}
						}
					}
				}
				if len(b.Rows) > 500 {
					return nil, coded("ERR_OIR_TABLE_TOO_BIG", "sections[%d].blocks[%d]: table max 500 rows, got %d", si, bi, len(b.Rows))
				}
				for _, m := range b.Merge {
					if !validMerge(m) {
						return nil, coded("ERR_OIR_BAD_MERGE", "sections[%d].blocks[%d]: bad merge %q (want A1:B2)", si, bi, m)
					}
				}
			default:
				return nil, coded("ERR_OIR_UNKNOWN_BLOCK", "sections[%d].blocks[%d]: unknown t %q (want h1|h2|p|bullet|code|image|chart|table)", si, bi, b.T)
			}
		}
	}
	return &s, nil
}

func validMerge(m string) bool {
	// A1:B2 — 1-3 letters, digits, colon, same shape
	parts := splitColon(m)
	if len(parts) != 2 {
		return false
	}
	return validCellRef(parts[0]) && validCellRef(parts[1])
}

func splitColon(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func validCellRef(s string) bool {
	if len(s) < 2 || len(s) > 7 {
		return false
	}
	i := 0
	for i < len(s) && ((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
		i++
	}
	if i == 0 || i > 3 {
		return false
	}
	d := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		d++
	}
	return d > 0 && i == len(s)
}

// NeedsClean reports whether a spec needs generated parts beyond transplant-safe
// ones (images, charts, header/footer). Such specs always build clean so that
// media/drawings/rels stay consistent.
func (s *Spec) NeedsClean() (bool, string) {
	if s.Meta != nil && (s.Meta.Header != "" || s.Meta.Footer != "") {
		return true, "header/footer need generated rels"
	}
	for _, sec := range s.Doc.Sections {
		for _, b := range sec.Blocks {
			switch b.T {
			case "image":
				return true, "images need media+rels"
			case "chart":
				return true, "charts need chart parts"
			}
		}
	}
	return false, ""
}

// CountBlocks counts total blocks across sections.
func (s *Spec) CountBlocks() int {
	n := 0
	for _, sec := range s.Doc.Sections {
		n += len(sec.Blocks)
	}
	return n
}
