// Package layout implements Excel/Word width solvers.
// Bootstrap: pure-Go approximation of Excel's algorithm.
// Real glyph shaping (freetype) lands in Phase 2.
package layout

import "math"

// ExcelCharUnit is Excel's default char width divisor for Calibri 11.
const ExcelCharUnit = 7.0

// ExcelWidthChars approximates Excel column width in character units.
// Formula: Trunc((pixels + 5) / 7 * 256) / 256  (Excel spec, ECMA-376 §18.3.1.13)
// Pixels use measured-ish advances for Calibri 11: lowercase ~7px, caps/digits
// ~8px, Latin-1 ~8.5px, CJK 11px. Bold adds ~8%.
// Upgrade path is a real glyph oracle (Pillow/freetype measuring Calibri).
func ExcelWidthChars(s string, bold bool) float64 {
	px := 0.0
	for _, r := range s {
		switch {
		case r < 32:
			// ignore control
		case r < 127:
			if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				px += 8.0
			} else {
				px += 7.0
			}
		case r < 0x1100:
			px += 8.5
		default:
			px += 11.0 // CJK / wide
		}
	}
	if bold {
		px *= 1.08
	}
	w := math.Trunc((px+5.0)/ExcelCharUnit*256.0) / 256.0
	if w < 8.43 {
		// Excel default; never shrink below readable unless explicitly requested
	}
	if w > 255 {
		w = 255
	}
	return w
}

// MaxWidth returns max width over a column's cells (header + rows).
func MaxWidth(cells []string, boldHeader bool) float64 {
	max := 8.43
	for i, c := range cells {
		w := ExcelWidthChars(c, i == 0 && boldHeader)
		if w > max {
			max = w
		}
	}
	// Excel adds ~2.5 chars breathing room (grid + filter + render slop)
	return max + 2.5
}

// TwipsPerInch is Word's unit: 1440 twips = 1 inch.
const TwipsPerInch = 1440

// WordTableColWidths distributes availableWidthTwips evenly.
// Later: weighted by content + MaxWidth solver.
func WordTableColWidths(availableTwips, ncols int) []int {
	if ncols <= 0 {
		return nil
	}
	base := availableTwips / ncols
	rem := availableTwips % ncols
	out := make([]int, ncols)
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}
