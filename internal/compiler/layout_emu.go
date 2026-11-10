// Slide visual layout: EMU placement of images and charts on a 16:9 slide.
// Shared by the PPTX emitter (parts placement) and the HTML preview
// (so the PNG agents audit matches the file exactly).
package compiler

import (
	"fmt"

	"github.com/DERYK-RYLOY/office-brains/internal/media"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

// SlideVisual constants (16:9): 12192000x6858000 EMU.
const (
	SlideCX      = 12192000
	SlideCY      = 6858000
	ContentMinY  = int64(1554480 + 2000000)
	TableRowH    = int64(320000)
	TableClampY  = int64(SlideCY - 300000)
	TableBreakY  = int64(SlideCY - 400000)
	TableAdvance = int64(200000)
	BottomPad    = int64(200000)
)

// VisualImage is one placed image on a slide.
type VisualImage struct {
	Cx, Cy, X, Y int64
	Name, Ext    string
	Alt          string
	Data         []byte // image bytes for embedding / data URIs
	Index        int    // 0-based within the whole deck
}

// VisualChart is one placed chart on a slide.
type VisualChart struct {
	X, Y, Cx, Cy int64
	Name         string
	Title        string
	Kind         string
	Labels       []string
	Series       []oir.ChartSeries
	Index        int // 0-based within the whole deck
}

// LayoutSlideVisuals computes EMU placement for one section's images and
// charts, anchored after its tables exactly as the renderer places them.
// mediaCounter/chartCounter accumulate part numbering across slides.
// Scaled-to-fit images and charts record a warning (content preserved);
// only genuinely unplaceable visuals are dropped (with a warning).
func LayoutSlideVisuals(sec oir.Section, slideIdx int, mediaCounter, chartCounter *int) ([]VisualImage, []VisualChart) {
	// simulate renderSlide's table placement exactly
	y := ContentMinY
	for _, bl := range sec.Blocks {
		if bl.T != "table" {
			continue
		}
		h := int64(len(bl.Rows)) * TableRowH
		if y+h > TableClampY {
			h = TableClampY - y
		}
		y += h + TableAdvance
		if y > TableBreakY {
			break
		}
	}
	imgs := []VisualImage{}
	for _, bl := range sec.Blocks {
		if bl.T != "image" {
			continue
		}
		m, err := media.Load(bl.Img)
		if err != nil {
			warn("slide %d: image %q not embedded (%v)", slideIdx+1, bl.Img, err)
			continue
		}
		wIn := bl.W
		if wIn == 0 {
			wIn = 4.0
		}
		if wIn > 10 {
			wIn = 10
		}
		cx, cy := media.EMUSize(wIn, m.WPx, m.HPx)
		if cy > 2500000 {
			cy = 2500000
			cx = cy * int64(m.WPx) / int64(m.HPx)
		}
		if cx > 11273880 {
			cx = 11273880
			cy = cx * int64(m.HPx) / int64(m.WPx)
		}
		if remaining := SlideCY - BottomPad - y; cy > remaining {
			if remaining < 500000 { // under ~0.55in
				warn("slide %d: image %q dropped (slide full, split content across sections)", slideIdx+1, bl.Alt)
				continue
			}
			cx = cx * remaining / cy
			cy = remaining
			warn("slide %d: image %q scaled to fit remaining space", slideIdx+1, bl.Alt)
		}
		*mediaCounter++
		nm := fmt.Sprintf("ppt/media/image%d.%s", *mediaCounter, m.Ext)
		imgs = append(imgs, VisualImage{Cx: cx, Cy: cy, X: (SlideCX - cx) / 2, Y: y, Name: nm, Ext: m.Ext, Alt: bl.Alt, Data: m.Data, Index: *mediaCounter - 1})
		y += cy + TableAdvance
	}
	charts := []VisualChart{}
	for _, bl := range sec.Blocks {
		if bl.T != "chart" {
			continue
		}
		cx, cy := int64(8000000), int64(2800000)
		if remaining := SlideCY - BottomPad - y; cy > remaining {
			if remaining < 800000 { // under ~0.87in
				warn("slide %d: chart %q dropped (slide full, split content across sections)", slideIdx+1, bl.Title)
				continue
			}
			cx = cx * remaining / cy
			cy = remaining
			warn("slide %d: chart %q scaled to fit remaining space", slideIdx+1, bl.Title)
		}
		*chartCounter++
		nm := fmt.Sprintf("ppt/charts/chart%d.xml", *chartCounter)
		charts = append(charts, VisualChart{X: (SlideCX - cx) / 2, Y: y, Cx: cx, Cy: cy, Name: nm, Title: bl.Title, Kind: bl.Kind, Labels: append([]string{}, bl.Labels...), Series: append([]oir.ChartSeries{}, bl.Series...), Index: *chartCounter - 1})
		y += cy + TableAdvance
	}
	return imgs, charts
}
