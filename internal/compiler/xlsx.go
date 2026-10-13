// XLSX compiler: OIR -> Transitional SpreadsheetML (stdlib only).
package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/you/office-brains/internal/forger"
	"github.com/you/office-brains/internal/layout"
	"github.com/you/office-brains/internal/media"
	"github.com/you/office-brains/internal/oir"
)

// BuildXlsx returns all ZIP parts for a minimal but Excel-clean .xlsx.
// OIR 1.1: one sheet per section (name sanitized). Single-section output keeps
// sheet1.xml naming. Rules: sharedStrings dedup, no calcChain, dimension fixed,
// widths calculated, formulas carry cached <v>, merges emitted as mergeCells.
func BuildXlsx(spec *oir.Spec, meta forger.Props) map[string][]byte {
	resetWarnings()
	meta.Defaults("Microsoft Excel")
	meta.Words = countWords(spec)

	type ximg = xlsxImg
	type xchart = xlsxChart
	type sheet struct {
		name   string
		rows   [][]oir.Cell
		merges []string
		imgs   []ximg
		charts []xchart
	}
	sheets := []sheet{}
	mediaCounter := 0
	chartCounter := 0
	for i, sec := range spec.Doc.Sections {
		nm := sanitizeSheetName(sec.Name, i)
		rows, merges := sectionXRows(sec)
		// collect images with anchor rows: find placeholder rows in order
		imgs := []ximg{}
		phNeed := 0
		for _, bl := range sec.Blocks {
			if bl.T == "image" {
				phNeed++
			}
		}
		_ = phNeed
		imgBlocks := []oir.Block{}
		for _, bl := range sec.Blocks {
			if bl.T == "image" {
				imgBlocks = append(imgBlocks, bl)
			}
		}
		// anchor row = index of Nth placeholder row ([image: ...])
		phSeen := 0
		for ri, r := range rows {
			if len(r) == 1 {
				if s, ok := r[0].V.(string); ok && len(s) >= 8 && s[:8] == "[image: " {
					if phSeen < len(imgBlocks) {
						bl := imgBlocks[phSeen]
						if m, err := media.Load(bl.Img); err == nil {
							wIn := bl.W
							if wIn == 0 {
								wIn = 2.5
							}
							cx, cy := media.EMUSize(wIn, m.WPx, m.HPx)
							if cy > 6*914400 {
								cy = 6 * 914400
								cx = cy * int64(m.WPx) / int64(m.HPx)
							}
							mediaCounter++
							nm2 := fmt.Sprintf("xl/media/image%d.%s", mediaCounter, m.Ext)
							imgs = append(imgs, ximg{data: m.Data, ext: m.Ext, alt: bl.Alt, cx: cx, cy: cy, row: ri, name: nm2})
						} else {
							warn("sheet %d: image %q not embedded (%v); label row only", i+1, bl.Img, err)
						}
					}
					phSeen++
				}
			}
		}
		// charts: placeholder rows [chart: title]
		charts := []xchart{}
		chartBlocks := []oir.Block{}
		for _, bl := range sec.Blocks {
			if bl.T == "chart" {
				chartBlocks = append(chartBlocks, bl)
			}
		}
		chSeen := 0
		for ri, r := range rows {
			if len(r) == 1 {
				if s, ok := r[0].V.(string); ok && len(s) >= 8 && s[:8] == "[chart: " {
					if chSeen < len(chartBlocks) {
						bl := chartBlocks[chSeen]
						chartCounter++
						nm3 := fmt.Sprintf("xl/charts/chart%d.xml", chartCounter)
						charts = append(charts, xchart{title: bl.Title, kind: bl.Kind, labels: bl.Labels, series: bl.Series, row: ri, name: nm3, cx: 5486400, cy: 3200400})
					}
					chSeen++
				}
			}
		}
		sheets = append(sheets, sheet{name: nm, rows: rows, merges: merges, imgs: imgs, charts: charts})
	}
	// global sharedStrings over all sheets (plain strings only)
	uniq := map[string]int{}
	ordered := []string{}
	strCount := 0
	for _, sh := range sheets {
		for _, r := range sh.rows {
			for _, c := range r {
				if c.F != "" {
					continue
				}
				if s, ok := c.V.(string); ok {
					if s == "" {
						continue
					}
					strCount++
					if _, ok := uniq[s]; !ok {
						uniq[s] = len(ordered)
						ordered = append(ordered, s)
					}
				}
			}
		}
	}
	names := make([]string, len(sheets))
	for i, sh := range sheets {
		names[i] = sh.name
	}
	drawSheets := []int{}
	chartSheets := []int{}
	for i, sh := range sheets {
		if len(sh.imgs) > 0 || len(sh.charts) > 0 {
			drawSheets = append(drawSheets, i+1)
		}
		if len(sh.charts) > 0 {
			chartSheets = append(chartSheets, i+1)
		}
	}
	// chart part names per sheet
	chartNames := map[int][]string{}
	for i, sh := range sheets {
		for _, ch := range sh.charts {
			chartNames[i+1] = append(chartNames[i+1], ch.name)
		}
	}
	parts := map[string][]byte{
		"[Content_Types].xml":        []byte(xlsxContentTypesDrawingsCharts(len(sheets), drawSheets, chartNames)),
		"_rels/.rels":                []byte(xlsxRootRels()),
		"xl/_rels/workbook.xml.rels": []byte(xlsxWbRelsN(len(sheets))),
		"xl/workbook.xml":            []byte(xlsxWorkbookN(names)),
		"xl/sharedStrings.xml":       []byte(renderSharedStringsCount(ordered, strCount)),
		"xl/styles.xml":              []byte(xlsxStyles()),
		"docProps/core.xml":          forger.CoreXML(meta),
		"docProps/app.xml":           forger.AppXML(meta, 12),
	}
	for i, sh := range sheets {
		// Float drawings BELOW the data grid so they never cover cells.
		// Anchors start past the last data row, 6 rows apart.
		base := len(sh.rows) + 1
		for k := range sh.imgs {
			sh.imgs[k].row = base + k*6
		}
		for k := range sh.charts {
			sh.charts[k].row = base + len(sh.imgs)*6 + k*10
		}
		ncols := 1
		for _, r := range sh.rows {
			if len(r) > ncols {
				ncols = len(r)
			}
		}
		if ncols > 20 {
			ncols = 20
		}
		colWidths := make([]float64, ncols)
		for c := 0; c < ncols; c++ {
			col := []string{}
			for _, r := range sh.rows {
				if c < len(r) {
					col = append(col, xDisplay(r[c]))
				} else {
					col = append(col, "")
				}
			}
			colWidths[c] = layout.MaxWidth(col, true)
		}
		hasDrawing := len(sh.imgs) > 0 || len(sh.charts) > 0
		parts[sheetPart(i)] = []byte(renderXSheetMergesDrawing(sh.rows, uniq, ncols, colWidths, sh.merges, hasDrawing))
		if hasDrawing {
			dn := i + 1
			parts[fmt.Sprintf("xl/drawings/drawing%d.xml", dn)] = []byte(xlsxDrawingFull(sh.imgs, sh.charts))
			// sheet rels -> drawing
			parts[fmt.Sprintf("xl/worksheets/_rels/sheet%d.xml.rels", dn)] = []byte(
				`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
					`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
					`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing` + itoa(dn) + `.xml"/>` +
					`</Relationships>`)
			// drawing rels -> images + charts
			dr := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
				`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
			for j, im := range sh.imgs {
				target := strings.TrimPrefix(im.name, "xl/drawings/")
				// from drawings/ dir: ../media/imageN.ext
				mediaTarget := "../" + strings.TrimPrefix(im.name, "xl/")
				_ = target
				dr += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="%s"/>`, j+1, mediaTarget)
			}
			for k, ch := range sh.charts {
				chartTarget := "../" + strings.TrimPrefix(ch.name, "xl/")
				dr += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="%s"/>`, len(sh.imgs)+k+1, chartTarget)
			}
			dr += `</Relationships>`
			parts[fmt.Sprintf("xl/drawings/_rels/drawing%d.xml.rels", dn)] = []byte(dr)
			for _, im := range sh.imgs {
				parts[im.name] = im.data
			}
			for _, ch := range sh.charts {
				parts[ch.name] = []byte(ChartXML(ch.kind, ch.title, ch.labels, ch.series))
			}
		}
	}
	return parts
}

type xlsxImg struct {
	data []byte
	ext  string // png|jpg
	alt  string
	cx   int64 // EMU
	cy   int64
	row  int // 0-based anchor row
	name string
}

type xlsxChart struct {
	title  string
	kind   string
	labels []string
	series []oir.ChartSeries
	row    int
	name   string
	cx     int64
	cy     int64
}

// xlsxDrawingFull adds graphicFrames for charts after pics.
func xlsxDrawingFull(imgs []xlsxImg, charts []xlsxChart) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">`)
	for i, im := range imgs {
		alt := im.alt
		if alt == "" {
			alt = fmt.Sprintf("image%d", i+1)
		}
		fromRow := im.row
		toRow := im.row + 4
		fmt.Fprintf(&b, `<xdr:twoCellAnchor editAs="oneCell"><xdr:from><xdr:col>0</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:to><xdr:col>2</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>`, fromRow, toRow)
		fmt.Fprintf(&b, `<xdr:pic><xdr:nvPicPr><xdr:cNvPr id="%d" name="%s" descr="%s"/><xdr:cNvPicPr/></xdr:nvPicPr><xdr:blipFill><a:blip r:embed="rId%d"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill><xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr></xdr:pic><xdr:clientData/></xdr:twoCellAnchor>`, i+2, esc(alt), esc(alt), i+1, im.cx, im.cy)
	}
	for j, ch := range charts {
		rid := len(imgs) + j + 1
		fromRow := ch.row
		toRow := ch.row + 8
		fmt.Fprintf(&b, `<xdr:twoCellAnchor editAs="oneCell"><xdr:from><xdr:col>0</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:to><xdr:col>4</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>`, fromRow, toRow)
		fmt.Fprintf(&b, `<xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="%d" name="%s"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></xdr:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId%d"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:twoCellAnchor>`, 100+j, esc(chartName(ch.title, j)), ch.cx, ch.cy, rid)
	}
	b.WriteString(`</xdr:wsDr>`)
	return b.String()
}

func chartName(t string, j int) string {
	if t == "" {
		return fmt.Sprintf("Chart %d", j+1)
	}
	return t
}

func sheetPart(i int) string { return "xl/worksheets/sheet" + itoa(i+1) + ".xml" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func sanitizeSheetName(name string, idx int) string {
	if name == "" {
		if idx == 0 {
			return "Sheet1"
		}
		return "Sheet" + itoa(idx+1)
	}
	out := ""
	for _, r := range name {
		switch r {
		case ':', '\\', '/', '?', '*', '[', ']':
			out += " "
		default:
			out += string(r)
		}
		if len(out) >= 31 {
			break
		}
	}
	// trim spaces
	for len(out) > 0 && out[0] == ' ' {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	if out == "" {
		out = "Sheet" + itoa(idx+1)
	}
	return out
}

// sectionXRows flattens one section to grid rows + merges with row offsets.
func sectionXRows(sec oir.Section) ([][]oir.Cell, []string) {
	str := func(s string) oir.Cell { return oir.Cell{V: s} }
	out := [][]oir.Cell{}
	merges := []string{}
	for _, bl := range sec.Blocks {
		switch bl.T {
		case "h1", "h2", "p":
			out = append(out, []oir.Cell{str(bl.Txt)})
		case "bullet":
			out = append(out, []oir.Cell{str("•  " + bl.Txt)})
		case "code":
			out = append(out, []oir.Cell{str(bl.Txt)})
		case "image":
			alt := bl.Alt
			if alt == "" {
				alt = "image"
			}
			out = append(out, []oir.Cell{str("[image: " + alt + "]")})
		case "chart":
			t := bl.Title
			if t == "" {
				t = "chart"
			}
			out = append(out, []oir.Cell{str("[chart: " + t + "]")})
		case "table":
			base := len(out) // 0-based; excel rows are base+1
			for _, r := range bl.Rows {
				cp := make([]oir.Cell, len(r))
				copy(cp, r)
				out = append(out, cp)
			}
			for _, m := range bl.Merge {
				merges = append(merges, shiftMerge(m, base))
			}
			out = append(out, []oir.Cell{str("")})
		}
	}
	if len(out) == 0 {
		out = [][]oir.Cell{{str("")}}
	}
	return out, merges
}

// shiftMerge offsets a merge ref like B2:C2 by base rows (base = rows before table).
func shiftMerge(m string, base int) string {
	// split A1:B2 at colon, shift row numbers
	ci := -1
	for i := 0; i < len(m); i++ {
		if m[i] == ':' {
			ci = i
			break
		}
	}
	if ci < 0 {
		return m
	}
	return shiftRef(m[:ci], base) + ":" + shiftRef(m[ci+1:], base)
}

func shiftRef(ref string, base int) string {
	i := 0
	for i < len(ref) && ((ref[i] >= 'A' && ref[i] <= 'Z') || (ref[i] >= 'a' && ref[i] <= 'z')) {
		i++
	}
	letters := ref[:i]
	num := 0
	for ; i < len(ref); i++ {
		num = num*10 + int(ref[i]-'0')
	}
	return letters + itoa(num+base)
}

// xDisplay is the human-visible string for width calc.
func xDisplay(c oir.Cell) string {
	if c.F != "" {
		return cellString(c.V)
	}
	if c.V == nil {
		return ""
	}
	return cellString(c.V)
}

func xlsxContentTypesN(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	b.WriteString(`<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`)
	return b.String()
}

// renderXSheetMergesDrawing adds drawing ref + merges in schema order.
func renderXSheetMergesDrawing(rows [][]oir.Cell, uniq map[string]int, ncols int, widths []float64, merges []string, hasDrawing bool) string {
	base := renderXSheet(rows, uniq, ncols, widths)
	if hasDrawing {
		base = strings.Replace(base, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`, 1)
		base = strings.Replace(base, `</worksheet>`, `<drawing r:id="rId1"/></worksheet>`, 1)
	}
	if len(merges) == 0 {
		return base
	}
	mc := `<mergeCells count="` + itoa(len(merges)) + `">`
	for _, m := range merges {
		mc += `<mergeCell ref="` + esc(m) + `"/>`
	}
	mc += `</mergeCells>`
	if hasDrawing {
		base = strings.Replace(base, `<drawing r:id="rId1"/>`, mc+`<drawing r:id="rId1"/>`, 1)
		return base
	}
	return strings.Replace(base, `</worksheet>`, mc+`</worksheet>`, 1)
}

func xlsxWbRelsN(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
	}
	fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>`, n+1)
	fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, n+2)
	b.WriteString(`</Relationships>`)
	return b.String()
}

func xlsxWorkbookN(names []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	for i, nm := range names {
		fmt.Fprintf(&b, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, esc(nm), i+1, i+1)
	}
	b.WriteString(`</sheets><calcPr calcId="191029" fullCalcOnLoad="1"/></workbook>`)
	return b.String()
}

func xlsxContentTypesDrawingsCharts(n int, drawSheets []int, chartNames map[int][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="png" ContentType="image/png"/><Default Extension="jpg" ContentType="image/jpeg"/><Default Extension="jpeg" ContentType="image/jpeg"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	for _, dn := range drawSheets {
		fmt.Fprintf(&b, `<Override PartName="/xl/drawings/drawing%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`, dn)
	}
	for _, names := range chartNames {
		for _, cn := range names {
			fmt.Fprintf(&b, `<Override PartName="/%s" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`, cn)
		}
	}
	b.WriteString(`<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`)
	return b.String()
}

func xlsxContentTypesDrawings(n int, drawSheets []int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="png" ContentType="image/png"/><Default Extension="jpg" ContentType="image/jpeg"/><Default Extension="jpeg" ContentType="image/jpeg"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	for _, dn := range drawSheets {
		fmt.Fprintf(&b, `<Override PartName="/xl/drawings/drawing%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`, dn)
	}
	b.WriteString(`<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`)
	return b.String()
}

// renderXSheetMerges wraps renderXSheet with mergeCells.
func renderXSheetMerges(rows [][]oir.Cell, uniq map[string]int, ncols int, widths []float64, merges []string) string {
	base := renderXSheet(rows, uniq, ncols, widths)
	if len(merges) == 0 {
		return base
	}
	// insert <mergeCells> before </worksheet>
	mc := `<mergeCells count="` + itoa(len(merges)) + `">`
	for _, m := range merges {
		mc += `<mergeCell ref="` + esc(m) + `"/>`
	}
	mc += `</mergeCells>`
	return strings.Replace(base, `</worksheet>`, mc+`</worksheet>`, 1)
}

func xlsxRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func xlsxStyles() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>` +
		`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs>` +
	`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
		`</styleSheet>`
}

func renderSharedStringsCount(ordered []string, strCount int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	fmt.Fprintf(&b, `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, strCount, len(ordered))
	for _, s := range ordered {
		b.WriteString(`<si><t xml:space="preserve">` + esc(s) + `</t></si>`)
	}
	b.WriteString(`</sst>`)
	return b.String()
}

// renderXSheet emits typed cells: numbers, bools, shared strings, formulas+cached values.
func renderXSheet(rows [][]oir.Cell, uniq map[string]int, ncols int, widths []float64) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="`)
	b.WriteString(dimRef(len(rows), ncols))
	b.WriteString(`"/><sheetViews><sheetView workbookViewId="0"/></sheetViews><sheetFormat defaultRowHeight="15"/>`)
	b.WriteString(`<cols>`)
	for i, w := range widths {
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%.2f" customWidth="1"/>`, i+1, i+1, w)
	}
	b.WriteString(`</cols><sheetData>`)
	for ri, r := range rows {
		fmt.Fprintf(&b, `<row r="%d">`, ri+1)
		for ci := 0; ci < ncols; ci++ {
			var cell oir.Cell
			has := ci < len(r)
			if has {
				cell = r[ci]
			}
			if !has || cell.V == nil && cell.F == "" {
				continue
			}
			// empty plain string -> sparse skip
			if cell.F == "" {
				if s, ok := cell.V.(string); ok && s == "" {
					continue
				}
			}
			ref := colName(ci) + fmt.Sprint(ri+1)
			style := 0
			if ri == 0 {
				style = 1
			}
			b.WriteString(renderXCell(ref, style, cell, uniq))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func renderXCell(ref string, style int, c oir.Cell, uniq map[string]int) string {
	if c.F != "" {
		f := strings.TrimPrefix(strings.TrimSpace(c.F), "=")
		f = esc(f)
		switch v := c.V.(type) {
		case float64:
			return fmt.Sprintf(`<c r="%s" s="%d"><f>%s</f><v>%s</v></c>`, ref, style, f, numStr(v))
		case bool:
			n := "0"
			if v {
				n = "1"
			}
			return fmt.Sprintf(`<c r="%s" s="%d" t="b"><f>%s</f><v>%s</v></c>`, ref, style, f, n)
		default:
			return fmt.Sprintf(`<c r="%s" s="%d" t="str"><f>%s</f><v>%s</v></c>`, ref, style, f, esc(cellString(c.V)))
		}
	}
	switch v := c.V.(type) {
	case float64:
		return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, style, numStr(v))
	case bool:
		n := "0"
		if v {
			n = "1"
		}
		return fmt.Sprintf(`<c r="%s" s="%d" t="b"><v>%s</v></c>`, ref, style, n)
	case string:
		return fmt.Sprintf(`<c r="%s" s="%d" t="s"><v>%d</v></c>`, ref, style, uniq[v])
	default:
		return fmt.Sprintf(`<c r="%s" s="%d" t="s"><v>%d</v></c>`, ref, style, uniq[cellString(c.V)])
	}
}

func numStr(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprint(int64(f))
	}
	return fmt.Sprint(f)
}

func renderSheet(rows [][]string, uniq map[string]int, ncols int, widths []float64) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="`)
	b.WriteString(dimRef(len(rows), ncols))
	b.WriteString(`"/><sheetViews><sheetView workbookViewId="0"/></sheetViews><sheetFormat defaultRowHeight="15"/>`)
	// cols with calculated widths (never auto)
	b.WriteString(`<cols>`)
	for i, w := range widths {
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%.2f" customWidth="1"/>`, i+1, i+1, w)
	}
	b.WriteString(`</cols><sheetData>`)
	for ri, r := range rows {
		fmt.Fprintf(&b, `<row r="%d">`, ri+1)
		for ci := 0; ci < ncols; ci++ {
			val := ""
			if ci < len(r) {
				val = r[ci]
			}
			if val == "" {
				continue // skip empty cells (sparse)
			}
			ref := colName(ci) + fmt.Sprint(ri+1)
			style := 0
			// bold first row of each table-ish area? bootstrap: bold row 1 + any h1 rows (single-col short)
			if ri == 0 {
				style = 1
			}
			fmt.Fprintf(&b, `<c r="%s" s="%d" t="s"><v>%d</v></c>`, ref, style, uniq[val])
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func dimRef(nrows, ncols int) string {
	if nrows == 0 {
		nrows = 1
	}
	return "A1:" + colName(ncols-1) + fmt.Sprint(nrows)
}

func colName(i int) string {
	// 0->A, 25->Z, 26->AA
	s := ""
	i++
	for i > 0 {
		i--
		s = string(rune('A'+(i%26))) + s
		i /= 26
	}
	return s
}

// sortedKeys helper kept for future diff tool (used to avoid unused import drift).
var _ = sort.Strings
