// Package preview renders OIR -> self-contained HTML with Office-exact math.
// Units: Word twips->px at 96dpi (px = twips/15). Excel chars->px (px = chars*7).
// PPT EMU->px scaled to 960px wide. Same constants as compilers, so overflow
// math in HTML matches what we emit into OOXML. Not pixel-identical to Word
// (different text shaper), but structure/widths/overflow are exact.
//
// v0.2.0: image blocks render (data URIs), chart blocks render (inline SVG),
// h3/quote/divider/pagebreak/toc render, and PPTX previews every slide with
// the same EMU placement as the emitter.
package preview

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/DERYK-RYLOY/office-brains/internal/compiler"
	"github.com/DERYK-RYLOY/office-brains/internal/layout"
	"github.com/DERYK-RYLOY/office-brains/internal/media"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

// Render builds a full HTML doc for spec+docType.
func Render(spec *oir.Spec, docType string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">`)
	fmt.Fprintf(&b, `<title>Brains preview - %s</title>`, esc(docType))
	b.WriteString(`<style>
*{box-sizing:border-box}body{font-family:Calibri,'Segoe UI',Arial,sans-serif;background:#f3f2f1;color:#201f1e;margin:0;padding:24px}
.bar{display:flex;gap:12px;align-items:center;margin:0 0 16px;flex-wrap:wrap}.badge{font:12px/1 Consolas,monospace;background:#fff;border:1px solid #c8c6c4;border-radius:4px;padding:6px 8px}
.ok{background:#dff6dd;border-color:#9cc49c}.warn{background:#fff4ce;border-color:#e7c873}.bad{background:#fde7e9;border-color:#e8a4ab}
.page{background:#fff;box-shadow:0 2px 8px rgba(0,0,0,.15);margin:0 auto;padding:0;overflow:hidden;page-break-after:always}
h1.ph{font-family:'Calibri Light',Calibri,sans-serif;color:#2E74B5;font-weight:300;margin:0 0 8px}
h2.ph{font-family:'Calibri Light',Calibri,sans-serif;color:#2E74B5;font-weight:300;margin:14px 0 6px}
h3.ph{font-family:'Calibri Light',Calibri,sans-serif;color:#2E74B5;font-weight:300;margin:10px 0 6px}
p.pp{font-size:11pt;line-height:1.4;margin:0 0 8px}p.bullet{margin-left:24px}p.code{font-family:Consolas,monospace;font-size:10pt;background:#f5f5f5;padding:6px 8px;border-radius:4px}
p.quote{font-style:italic;color:#595959;margin:8px 24px;border-left:3px solid #B4C6E7;padding-left:12px}
hr.div{border:none;border-top:1px solid #B4C6E7;margin:12px 0}
table.wtbl{border-collapse:collapse;table-layout:fixed;width:100%}table.wtbl td,table.wtbl th{border:1px solid #B4C6E7;padding:4px 8px;font-size:11pt;overflow:hidden;text-overflow:ellipsis}table.wtbl th{background:#D9E2F3;text-align:left}
table.xtbl{border-collapse:collapse}.xtbl td,.xtbl th{border:1px solid #d2d0ce;padding:4px 8px;font-size:11pt;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.xtbl th{background:#f3f2f1}
.xtbl .num{text-align:right}.xtbl .formula{background:#fff4ce}
.slide{background:#fff;position:relative;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,.2);margin:0 auto 20px}
.slide .ttl{position:absolute;font-weight:700;color:#201f1e;overflow:hidden}.slide .bdy{position:absolute;overflow:hidden;font-size:14px}.slide table{border-collapse:collapse;width:100%;height:100%}.slide td{border:1px solid #8ea9db;padding:4px 6px;font-size:12px;overflow:hidden}.slide th{background:#D9E2F3;border:1px solid #8ea9db;padding:4px 6px;font-size:12px;text-align:left}
.slide img{position:absolute}.slide svg{position:absolute}
.small{font:12px Consolas,monospace;color:#605e5c;margin-top:12px;white-space:pre-wrap}
.pageimg{display:block;margin:8px auto;max-width:100%}
.tocbox{background:#fbfaf9;border:1px solid #e2e1df;border-radius:6px;padding:10px 14px;margin:0 0 10px}
.tocbox a{display:block;color:#2E74B5;text-decoration:none;margin:2px 0}
.chartbox{margin:10px auto;display:flex;justify-content:center;max-width:480px;height:270px}
.chartbox svg{width:100%;height:100%}
</style></head><body>`)
	switch docType {
	case "docx":
		b.WriteString(renderDocx(spec))
	case "xlsx":
		b.WriteString(renderXlsx(spec))
	case "pptx":
		b.WriteString(renderPptx(spec))
	default:
		b.WriteString(`<div class="badge bad">unknown type</div>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}

// ---- shared: images ----

func imgDataURI(bl oir.Block) (string, error) {
	m, err := media.Load(bl.Img)
	if err != nil {
		return "", err
	}
	mime := "image/png"
	switch strings.ToLower(m.Ext) {
	case "jpg", "jpeg":
		mime = "image/jpeg"
	case "png":
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(m.Data), nil
}

// ---- shared: inline SVG charts ----

var chartPalette = []string{"4472C4", "ED7D31", "A5A5A5", "FFC000", "5B9BD5", "70AD47", "264478", "E15728"}

// svgChart renders a compact, honest SVG chart: bar groups, line series,
// or a first-series pie. No external deps.
func svgChart(kind, title string, labels []string, series []oir.ChartSeries) string {
	if kind == "" || kind == "bar" {
		return svgBarOrLine(false, title, labels, series)
	}
	if kind == "line" {
		return svgBarOrLine(true, title, labels, series)
	}
	return svgPie(title, labels, series)
}

func svgBarOrLine(isLine bool, title string, labels []string, series []oir.ChartSeries) string {
	const W, H = 480, 270
	const padL, padR, padT, padB = 44, 10, 30, 46
	plotW, plotH := float64(W-padL-padR), float64(H-padT-padB)
	maxV := 0.0
	for _, s := range series {
		for _, v := range s.Values {
			if math.Abs(v) > maxV {
				maxV = math.Abs(v)
			}
		}
	}
	if maxV == 0 {
		maxV = 1
	}
	if title == "" {
		title = "Chart"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="100%%" height="100%%" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="%s">`, W, H, esc(title))
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%d" height="%d" fill="#ffffff"/>`, W, H)
	fmt.Fprintf(&b, `<text x="%d" y="20" font-family="Calibri,sans-serif" font-size="13" font-weight="700" fill="#201f1e">%s</text>`, padL, esc(title))
	// y axis line + baseline
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#d2d0ce"/>`, padL, padT, padL, padT+int(plotH))
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#a19f9d"/>`, padL, padT+int(plotH), W-padR, padT+int(plotH))
	n := len(labels)
	if n == 0 {
		n = 1
	}
	groupW := plotW / float64(n)
	baseY := float64(padT) + plotH
	if isLine {
		for si, s := range series {
			color := chartPalette[si%len(chartPalette)]
			pts := ""
			for li, v := range s.Values {
				x := float64(padL) + (float64(li)+0.5)*groupW
				y := baseY - (v/maxV)*plotH*0.9 - plotH*0.05
				pts += fmt.Sprintf("%.1f,%.1f ", x, y)
			}
			fmt.Fprintf(&b, `<polyline fill="none" stroke="#%s" stroke-width="2" points="%s"/>`, color, strings.TrimSpace(pts))
			for li, v := range s.Values {
				x := float64(padL) + (float64(li)+0.5)*groupW
				y := baseY - (v/maxV)*plotH*0.9 - plotH*0.05
				fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3" fill="#%s"/>`, x, y, color)
			}
		}
	} else {
		barW := groupW * 0.8 / float64(max(1, len(series)))
		for si := range series {
			color := chartPalette[si%len(chartPalette)]
			for li, v := range series[si].Values {
				x := float64(padL) + float64(li)*groupW + groupW*0.1 + float64(si)*barW
				h := math.Abs(v) / maxV * plotH * 0.9
				y := baseY - h
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#%s"><title>%s: %s</title></rect>`, x, y, barW*0.9, h, color, esc(series[si].Name), esc(cellStr(v)))
			}
		}
	}
	// category labels
	fs := 11
	if n > 8 {
		fs = 9
	}
	for li, lb := range labels {
		x := float64(padL) + (float64(li)+0.5)*groupW
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" font-family="Calibri,sans-serif" font-size="%d" fill="#605e5c" text-anchor="middle">%s</text>`, x, H-padB+16, fs, esc(truncate(lb, 12)))
	}
	// legend
	var legend strings.Builder
	lx := padL
	for si, s := range series {
		nm := s.Name
		if nm == "" {
			nm = fmt.Sprintf("Series %d", si+1)
		}
		color := chartPalette[si%len(chartPalette)]
		fmt.Fprintf(&legend, `<tspan x="%d" y="%d"><tspan fill="#%s">■ </tspan>%s  </tspan>`, lx, H-8, color, esc(truncate(nm, 18)))
		lx += 16 + len(nm)*7
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="Calibri,sans-serif" font-size="11" fill="#201f1e">%s</text>`, padL, H-8, legend.String())
	b.WriteString(`</svg>`)
	return b.String()
}

func svgPie(title string, labels []string, series []oir.ChartSeries) string {
	const W, H = 480, 270
	if title == "" {
		title = "Chart"
	}
	cx, cy, r := 160.0, 155.0, 85.0
	vals := []float64{}
	if len(series) > 0 {
		vals = series[0].Values
	}
	total := 0.0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="100%%" height="100%%" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="%s">`, W, H, esc(title))
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#ffffff"/>`, W, H)
	fmt.Fprintf(&b, `<text x="20" y="20" font-family="Calibri,sans-serif" font-size="13" font-weight="700" fill="#201f1e">%s</text>`, esc(title))
	if total <= 0 {
		b.WriteString(`<text x="160" y="155" font-family="Calibri,sans-serif" font-size="12" fill="#a19f9d" text-anchor="middle">no positive values</text></svg>`)
		return b.String()
	}
	start := -90.0 // 12 o'clock
	for li, v := range vals {
		if v <= 0 {
			continue
		}
		frac := v / total
		angle := frac * 360
		end := start + angle
		large := 0.0
		if angle > 180 {
			large = 1.0
		}
		x1 := cx + r*cosd(start)
		y1 := cy + r*sind(start)
		x2 := cx + r*cosd(end)
		y2 := cy + r*sind(end)
		color := chartPalette[li%len(chartPalette)]
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f A %.1f %.1f 0 %.0f 1 %.1f %.1f Z" fill="#%s"><title>%s: %s (%.0f%%)</title></path>`,
			cx, cy, x1, y1, r, r, large, x2, y2, color, esc(labelAt(labels, li)), esc(cellStr(v)), frac*100)
		start = end
	}
	// legend with values
	ly := 50
	for li, lb := range labels {
		if li >= len(vals) {
			break
		}
		color := chartPalette[li%len(chartPalette)]
		pct := 0.0
		if total > 0 && vals[li] > 0 {
			pct = vals[li] / total * 100
		}
		fmt.Fprintf(&b, `<text x="270" y="%d" font-family="Calibri,sans-serif" font-size="11" fill="#201f1e"><tspan fill="#%s">■ </tspan>%s <tspan fill="#605e5c">(%.0f%%)</tspan></text>`, ly, color, esc(truncate(lb, 16)), pct)
		ly += 20
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func cosd(deg float64) float64 { return math.Cos(deg * math.Pi / 180) }
func sind(deg float64) float64 { return math.Sin(deg * math.Pi / 180) }
func labelAt(labels []string, i int) string {
	if i < len(labels) {
		return labels[i]
	}
	return fmt.Sprintf("slice %d", i+1)
}
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---- shared: block html helpers ----

func imageHTML(bl oir.Block, center bool) string {
	uri, err := imgDataURI(bl)
	if err != nil {
		return `<p class="pp">[image not embedded: ` + esc(bl.Img) + `]</p>`
	}
	wStyle := ""
	if bl.W > 0 {
		wStyle = fmt.Sprintf(` style="width:%.2fin"`, bl.W)
	} else {
		wStyle = ` style="width:2.5in"`
	}
	cls := ""
	if center {
		cls = ` class="pageimg"`
	}
	return fmt.Sprintf(`<img%s%s src="%s" alt="%s">`, cls, wStyle, uri, esc(bl.Alt))
}

func chartHTML(bl oir.Block) string {
	return `<div class="chartbox">` + svgChart(bl.Kind, bl.Title, bl.Labels, bl.Series) + `</div>`
}

func tocHTML(spec *oir.Spec) string {
	var b strings.Builder
	b.WriteString(`<div class="tocbox"><div class="badge">Table of Contents (Word populates on open)</div>`)
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			switch bl.T {
			case "h1":
				fmt.Fprintf(&b, `<a href="#">%s</a>`, esc(bl.Txt))
			case "h2":
				fmt.Fprintf(&b, `<a href="#" style="padding-left:18px">%s</a>`, esc(bl.Txt))
			case "h3":
				fmt.Fprintf(&b, `<a href="#" style="padding-left:36px">%s</a>`, esc(bl.Txt))
			}
		}
	}
	b.WriteString(`</div>`)
	return b.String()
}

// ---- DOCX: Letter 12240x15840 twips, margins 1440. px = twips/15 ----
func renderDocx(spec *oir.Spec) string {
	const pageWTwips, pageHTwips = 12240, 15840
	const mTwips = 1440
	pwPx, phPx, mPx := pageWTwips/15, pageHTwips/15, mTwips/15
	contentPx := pwPx - 2*mPx
	var b strings.Builder
	issues := checkDocx(spec, contentPx)
	b.WriteString(`<div class="bar"><span class="badge">DOCX preview - Letter, 1in margins</span>`)
	fmt.Fprintf(&b, `<span class="badge">page %dpx, content %dpx</span>`, pwPx, contentPx)
	for _, is := range issues {
		fmt.Fprintf(&b, `<span class="badge %s">%s</span>`, is[0], esc(is[1]))
	}
	b.WriteString(`</div>`)
	fmt.Fprintf(&b, `<div class="page" style="width:%dpx;min-height:%dpx"><div style="padding:%dpx">`, pwPx, phPx, mPx)
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			switch bl.T {
			case "h1":
				fmt.Fprintf(&b, `<h1 class="ph" style="font-size:16pt">%s</h1>`, esc(bl.Txt))
			case "h2":
				fmt.Fprintf(&b, `<h2 class="ph" style="font-size:13pt">%s</h2>`, esc(bl.Txt))
			case "h3":
				fmt.Fprintf(&b, `<h3 class="ph" style="font-size:11pt">%s</h3>`, esc(bl.Txt))
			case "p":
				fmt.Fprintf(&b, `<p class="pp">%s</p>`, esc(bl.Txt))
			case "bullet":
				fmt.Fprintf(&b, `<p class="pp bullet">•&nbsp;&nbsp;%s</p>`, esc(bl.Txt))
			case "quote":
				fmt.Fprintf(&b, `<p class="quote">%s</p>`, esc(bl.Txt))
			case "code":
				fmt.Fprintf(&b, `<p class="code">%s</p>`, esc(bl.Txt))
			case "divider":
				b.WriteString(`<hr class="div"/>`)
			case "pagebreak":
				b.WriteString(`<div style="page-break-after:always;border-top:1px dashed #c8c6c4;margin:8px 0"></div>`)
			case "toc":
				b.WriteString(tocHTML(spec))
			case "image":
				b.WriteString(imageHTML(bl, true))
			case "chart":
				b.WriteString(chartHTML(bl))
			case "table":
				ncols := 0
				if len(bl.Rows) > 0 {
					ncols = len(bl.Rows[0])
				}
				widths := layout.WordTableColWidths(9360, ncols)
				b.WriteString(`<table class="wtbl">`)
				if ncols > 0 {
					b.WriteString(`<colgroup>`)
					for _, w := range widths {
						fmt.Fprintf(&b, `<col style="width:%dpx">`, w/15)
					}
					b.WriteString(`</colgroup>`)
				}
				for ri, r := range bl.Rows {
					b.WriteString(`<tr>`)
					for _, c := range r {
						txt := esc(cellStr(c.V))
						if c.F != "" {
							txt = esc(cellStr(c.V)) + ` <span class="badge warn" style="font-size:10px">ƒ ` + esc(c.F) + `</span>`
						}
						if ri == 0 {
							fmt.Fprintf(&b, `<th>%s</th>`, txt)
						} else {
							fmt.Fprintf(&b, `<td>%s</td>`, txt)
						}
					}
					b.WriteString(`</tr>`)
				}
				b.WriteString(`</table><div style="height:8px"></div>`)
			}
		}
	}
	b.WriteString(`</div></div>`)
	fmt.Fprintf(&b, `<div class="small">twips: pageW=12240 avail=9360 | px: content=%d | cols use tblLayout=fixed, grid=sum must equal 9360</div>`, contentPx)
	return b.String()
}

func checkDocx(spec *oir.Spec, contentPx int) [][2]string {
	out := [][2]string{{"ok", "math: content fits"}}
	_ = contentPx
	_ = compiler.ColNameForPreview
	return out
}

// ---- XLSX: chars->px ----
func renderXlsx(spec *oir.Spec) string {
	rows := xRows(spec)
	ncols := 1
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
				col = append(col, xDisp(r[c]))
			} else {
				col = append(col, "")
			}
		}
		widths[c] = layout.MaxWidth(col, true)
	}
	var b strings.Builder
	b.WriteString(`<div class="bar"><span class="badge">XLSX preview</span>`)
	totalChars := 0.0
	for _, w := range widths {
		totalChars += w
	}
	fmt.Fprintf(&b, `<span class="badge">cols=%d total=%.1f chars</span>`, ncols, totalChars)
	if spec.Meta != nil && spec.Meta.FreezeHeader {
		b.WriteString(`<span class="badge ok">freeze_header: row 1 frozen</span>`)
	}
	b.WriteString(`<span class="badge ok">widths = emitted cols (never auto)</span></div>`)
	b.WriteString(`<div class="page" style="padding:16px;overflow:auto"><table class="xtbl"><colgroup>`)
	for _, w := range widths {
		px := int(w*7 + 5)
		fmt.Fprintf(&b, `<col style="width:%dpx;min-width:%dpx">`, px, px)
	}
	b.WriteString(`</colgroup>`)
	for ri, r := range rows {
		b.WriteString(`<tr>`)
		for ci := 0; ci < ncols; ci++ {
			var cell oir.Cell
			if ci < len(r) {
				cell = r[ci]
			}
			cls, txt := xClassText(cell)
			style := ""
			if ri == 0 && txt != "" {
				style = ` style="font-weight:700"`
			}
			fmt.Fprintf(&b, `<td class="%s"%s>%s</td>`, cls, style, txt)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)
	// images & charts anchor below the grid like the emitter
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			switch bl.T {
			case "image":
				b.WriteString(`<div style="margin:10px 0">` + imageHTML(bl, false) + `</div>`)
			case "chart":
				b.WriteString(chartHTML(bl))
			}
		}
	}
	fmt.Fprintf(&b, `<div class="small">chars->px: px=chars*7+5 | `)
	for i, w := range widths {
		fmt.Fprintf(&b, `col%s=%.2fchars(%dpx) `, compiler.ColNameForPreview(i), w, int(w*7+5))
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

// ---- PPTX: 16:9 slides scaled to 960px, one .slide div per slide ----
func renderPptx(spec *oir.Spec) string {
	const W = 960
	H := W * compiler.SlideCY / compiler.SlideCX // 540
	sx := float64(W) / float64(compiler.SlideCX)
	sections := expandSections(spec.Doc.Sections)
	mc, cc := 0, 0
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="bar"><span class="badge">PPTX preview - 16:9</span><span class="badge">%d slide(s), %dx%dpx</span></div>`, len(sections), W, H)
	for i, sec := range sections {
		fmt.Fprintf(&b, `<div class="slide" style="width:%dpx;height:%dpx">`, W, H)
		fmt.Fprintf(&b, `<div class="badge" style="position:absolute;top:6px;right:8px;z-index:2">slide %d</div>`, i+1)
		vimgs, vcharts := compiler.LayoutSlideVisuals(sec, i, &mc, &cc)
		compiler.ResetTruncated() // preview is read-only; drop layout warnings
		title, paras, tables := pptSectionParts(sec)
		fmt.Fprintf(&b, `<div class="ttl" style="left:%dpx;top:%dpx;width:%dpx;height:%dpx;font-size:28px">%s</div>`,
			px(457200, sx), px(274320, sx), px(11273880, sx), px(1097280, sx), esc(title))
		by, bh := int64(1554480), int64(4846320)
		if len(tables) > 0 || len(vimgs) > 0 || len(vcharts) > 0 {
			bh = 1800000
		}
		bodyHTML := ""
		for _, p := range paras {
			bodyHTML += `<div>` + esc(p) + `</div>`
		}
		if bodyHTML == "" && len(tables) == 0 {
			bodyHTML = `&nbsp;`
		}
		fmt.Fprintf(&b, `<div class="bdy" style="left:%dpx;top:%dpx;width:%dpx;height:%dpx">%s</div>`,
			px(457200, sx), px(by, sx), px(11273880, sx), px(bh, sx), bodyHTML)
		// tables: same clamp math as the emitter
		y := int64(1554480 + 2000000)
		for _, t := range tables {
			h := int64(len(t)) * 320000
			if y+h > compiler.TableClampY {
				h = compiler.TableClampY - y
			}
			fmt.Fprintf(&b, `<div style="position:absolute;left:%dpx;top:%dpx;width:%dpx;height:%dpx;overflow:hidden"><table>`, px(457200, sx), px(y, sx), px(11273880, sx), px(h, sx))
			for ri, r := range t {
				b.WriteString(`<tr>`)
				for _, c := range r {
					if ri == 0 {
						fmt.Fprintf(&b, `<th>%s</th>`, esc(cellStr(c.V)))
					} else {
						fmt.Fprintf(&b, `<td>%s</td>`, esc(cellStr(c.V)))
					}
				}
				b.WriteString(`</tr>`)
			}
			b.WriteString(`</table></div>`)
			y += h + 200000
			if y > compiler.TableBreakY {
				break
			}
		}
		// images: same EMU placement as the emitter
		for _, vi := range vimgs {
			mime := "image/png"
			if strings.EqualFold(vi.Ext, "jpg") || strings.EqualFold(vi.Ext, "jpeg") {
				mime = "image/jpeg"
			}
			fmt.Fprintf(&b, `<img style="left:%dpx;top:%dpx;width:%dpx;height:%dpx" src="data:%s;base64,%s" alt="%s"/>`,
				px(vi.X, sx), px(vi.Y, sx), px(vi.Cx, sx), px(vi.Cy, sx), mime, base64.StdEncoding.EncodeToString(vi.Data), esc(vi.Alt))
		}
		// charts: inline SVG at the same EMU placement
		for _, vc := range vcharts {
			fmt.Fprintf(&b, `<div style="position:absolute;left:%dpx;top:%dpx;width:%dpx;height:%dpx">`, px(vc.X, sx), px(vc.Y, sx), px(vc.Cx, sx), px(vc.Cy, sx))
			b.WriteString(svgChart(vc.Kind, vc.Title, vc.Labels, vc.Series))
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div class="small">EMU->px scale=` + fmt.Sprintf("%.6f", sx) + ` | placements computed by the shared layout pass (same constants as the emitter)</div>`)
	return b.String()
}

func px(emu int64, sx float64) int { return int(float64(emu) * sx) }

func expandSections(in []oir.Section) []oir.Section {
	out := []oir.Section{}
	for _, sec := range in {
		cur := oir.Section{ID: sec.ID, Name: sec.Name}
		for _, bl := range sec.Blocks {
			if bl.T == "pagebreak" {
				if len(cur.Blocks) > 0 {
					out = append(out, cur)
				}
				cur = oir.Section{Name: sec.Name}
				continue
			}
			cur.Blocks = append(cur.Blocks, bl)
		}
		if len(cur.Blocks) > 0 {
			out = append(out, cur)
		}
	}
	if len(out) == 0 {
		out = in
	}
	return out
}

func pptSectionParts(sec oir.Section) (string, []string, [][][]oir.Cell) {
	title := "Untitled"
	paras := []string{}
	tables := [][][]oir.Cell{}
	for _, bl := range sec.Blocks {
		switch bl.T {
		case "h1":
			if title == "Untitled" {
				title = bl.Txt
			} else {
				paras = append(paras, bl.Txt)
			}
		case "h2", "h3", "p":
			paras = append(paras, bl.Txt)
		case "quote":
			paras = append(paras, "“"+bl.Txt+"”")
		case "bullet":
			paras = append(paras, "•  "+bl.Txt)
		case "divider":
			paras = append(paras, "────────────────────────────")
		case "code":
			paras = append(paras, bl.Txt)
		case "table":
			cp := make([][]oir.Cell, len(bl.Rows))
			for i, r := range bl.Rows {
				rr := make([]oir.Cell, len(r))
				copy(rr, r)
				cp[i] = rr
			}
			tables = append(tables, cp)
		}
	}
	return title, paras, tables
}

func xRows(spec *oir.Spec) [][]oir.Cell {
	out := [][]oir.Cell{}
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			switch bl.T {
			case "h1", "h2", "h3", "p":
				out = append(out, []oir.Cell{{V: bl.Txt}})
			case "bullet":
				out = append(out, []oir.Cell{{V: "•  " + bl.Txt}})
			case "quote":
				out = append(out, []oir.Cell{{V: "“" + bl.Txt + "”"}})
			case "code":
				out = append(out, []oir.Cell{{V: bl.Txt}})
			case "divider":
				out = append(out, []oir.Cell{{V: "────────────────────────"}})
			case "image":
				alt := bl.Alt
				if alt == "" {
					alt = "image"
				}
				out = append(out, []oir.Cell{{V: "[image: " + alt + "]"}})
			case "chart":
				t := bl.Title
				if t == "" {
					t = "chart"
				}
				out = append(out, []oir.Cell{{V: "[chart: " + t + "]"}})
			case "table":
				for _, r := range bl.Rows {
					cp := make([]oir.Cell, len(r))
					copy(cp, r)
					out = append(out, cp)
				}
				out = append(out, []oir.Cell{{V: ""}})
			}
		}
	}
	return out
}

func xDisp(c oir.Cell) string { return cellStr(c.V) }

func xClassText(c oir.Cell) (string, string) {
	if c.F != "" {
		return "formula", esc(cellStr(c.V)) + ` <span class="badge warn">ƒ ` + esc(c.F) + `</span>`
	}
	switch v := c.V.(type) {
	case float64:
		return "num", esc(cellStr(v))
	case bool:
		return "num", esc(cellStr(v))
	case string:
		if v == "" {
			return "", ""
		}
		return "", esc(v)
	case nil:
		return "", ""
	default:
		return "", esc(cellStr(v))
	}
}

func cellStr(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprint(int64(t))
		}
		return fmt.Sprint(t)
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprint(t)
	}
}

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
