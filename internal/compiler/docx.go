// DOCX compiler: OIR -> Transitional OOXML parts (stdlib only).
package compiler

import (
	"fmt"
	"strings"

	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/layout"
	"github.com/DERYK-RYLOY/office-brains/internal/media"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

const (
	// Letter 8.5x11in in twips; M365 default margins 1in.
	pageW      = 12240
	marginL    = 1440
	marginR    = 1440
	availWidth = pageW - marginL - marginR // 9360
)

// BuildDocx returns all ZIP parts for a minimal but Word-clean .docx.
// OIR 1.1: images embed best-effort (missing -> placeholder text),
// meta.header/footer emit header1/footer1 parts, chart blocks emit native
// c:chart parts (same renderers as xlsx/pptx), and a toc block emits a
// Word TOC field that Word populates on open (w:updateFields).
func BuildDocx(spec *oir.Spec, meta forger.Props) map[string][]byte {
	resetWarnings()
	meta.Defaults("Microsoft Office Word")
	words := countWords(spec)
	meta.Words = words
	if words > 400 {
		meta.Pages = (words + 400) / 500
		if meta.Pages < 1 {
			meta.Pages = 1
		}
	}

	type imgPart = docxImgPart
	imgs := []imgPart{}
	// collect images in document order
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			if bl.T != "image" {
				continue
			}
			m, err := media.Load(bl.Img)
			if err != nil {
				warn("image %q not embedded (%v); placeholder text used", bl.Img, err)
				continue // placeholder rendered in body; no part
			}
			wIn := bl.W
			if wIn == 0 {
				wIn = 2.5
				if float64(m.WPx)/float64(m.HPx) > 3 {
					wIn = 5.0
				}
			}
			cx, cy := media.EMUSize(wIn, m.WPx, m.HPx)
			// clamp height to page (9in max)
			if cy > 9*914400 {
				cy = 9 * 914400
				cx = cy * int64(m.WPx) / int64(m.HPx)
			}
			ext := m.Ext
			if ext == "jpg" {
				ext = "jpeg"
			}
			name := fmt.Sprintf("word/media/image%d.%s", len(imgs)+1, m.Ext)
			imgs = append(imgs, imgPart{name: name, data: m.Data, ext: ext, cx: cx, cy: cy, alt: bl.Alt})
		}
	}
	// collect charts in document order (native c:chart parts)
	type chartPart struct {
		name   string
		kind   string
		title  string
		labels []string
		series []oir.ChartSeries
	}
	chartParts := []chartPart{}
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			if bl.T != "chart" {
				continue
			}
			chartParts = append(chartParts, chartPart{
				name:   fmt.Sprintf("word/charts/chart%d.xml", len(chartParts)+1),
				kind:   bl.Kind,
				title:  bl.Title,
				labels: append([]string{}, bl.Labels...),
				series: append([]oir.ChartSeries{}, bl.Series...),
			})
		}
	}

	hasHeader := spec.Meta != nil && spec.Meta.Header != ""
	hasFooter := spec.Meta != nil && spec.Meta.Footer != ""
	hasTOC := false
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			if bl.T == "toc" {
				hasTOC = true
			}
		}
	}

	docXML := renderDocument11(spec, docxImgInfos(imgs), len(chartParts), hasHeader, hasFooter)
	imgNames := make([]string, len(imgs))
	for i, im := range imgs {
		imgNames[i] = im.name
	}
	chartNames := make([]string, len(chartParts))
	for i, c := range chartParts {
		chartNames[i] = c.name
	}
	parts := map[string][]byte{
		"[Content_Types].xml":          []byte(docxContentTypes11(len(imgs) > 0, hasHeader, hasFooter, chartNames)),
		"_rels/.rels":                  []byte(docxRootRels()),
		"word/_rels/document.xml.rels": []byte(docxDocRels11(imgNames, hasHeader, hasFooter, chartNames)),
		"word/document.xml":            []byte(docXML),
		"word/styles.xml":              []byte(docxStyles()),
		"word/settings.xml":            []byte(docxSettings(hasTOC)),
		"word/fontTable.xml":           []byte(docxFontTable()),
		"docProps/core.xml":            forger.CoreXML(meta),
		"docProps/app.xml":             forger.AppXML(meta, 47),
	}
	for _, im := range imgs {
		parts[im.name] = im.data
	}
	for _, c := range chartParts {
		parts[c.name] = []byte(ChartXML(c.kind, c.title, c.labels, c.series))
	}
	if hasHeader {
		parts["word/header1.xml"] = []byte(docxHeaderFooterXML(spec.Meta.Header, false))
	}
	if hasFooter {
		parts["word/footer1.xml"] = []byte(docxHeaderFooterXML(spec.Meta.Footer, true))
	}
	return parts
}

type docxImgPart struct {
	name string
	data []byte
	ext  string
	cx   int64
	cy   int64
	alt  string
}

// DocxImg is the render model for one embedded picture.
type DocxImg struct {
	Cx  int64
	Cy  int64
	Alt string
	RID string
}

func docxImgInfos(parts []docxImgPart) []DocxImg {
	out := make([]DocxImg, len(parts))
	for i, p := range parts {
		out[i] = DocxImg{Cx: p.cx, Cy: p.cy, Alt: p.alt}
	}
	return out
}

func renderDocument11(spec *oir.Spec, imgs []DocxImg, nCharts int, hasHeader, hasFooter bool) string {
	// assign rIds in order: base 1-3, header?, footer?, images..., charts...
	next := 4
	headerRID, footerRID := "", ""
	if hasHeader {
		headerRID = fmt.Sprintf("rId%d", next)
		next++
	}
	if hasFooter {
		footerRID = fmt.Sprintf("rId%d", next)
		next++
	}
	for i := range imgs {
		imgs[i].RID = fmt.Sprintf("rId%d", next)
		next++
	}
	chartRIDs := make([]string, nCharts)
	for i := 0; i < nCharts; i++ {
		chartRIDs[i] = fmt.Sprintf("rId%d", next)
		next++
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><w:body>`)
	imgIdx := 0
	chartIdx := 0
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			switch bl.T {
			case "h1":
				b.WriteString(para("Heading1", bl.Txt, false, false))
			case "h2":
				b.WriteString(para("Heading2", bl.Txt, false, false))
			case "h3":
				b.WriteString(para("Heading3", bl.Txt, false, false))
			case "p":
				b.WriteString(para("Normal", bl.Txt, false, false))
			case "bullet":
				b.WriteString(para("Normal", "•  "+bl.Txt, false, false))
			case "quote":
				b.WriteString(para("Quote", bl.Txt, false, false))
			case "code":
				b.WriteString(para("Normal", bl.Txt, true, false))
			case "divider":
				b.WriteString(dividerPara())
			case "pagebreak":
				b.WriteString(pageBreakPara())
			case "toc":
				b.WriteString(tocDocx())
			case "image":
				if imgIdx < len(imgs) {
					b.WriteString(docxImagePara(imgs[imgIdx], imgIdx))
					imgIdx++
				} else {
					b.WriteString(para("Normal", "[image missing: "+bl.Img+"]", false, false))
				}
			case "chart":
				b.WriteString(docxChartPara(bl, chartRIDs[chartIdx], chartIdx))
				chartIdx++
			case "table":
				b.WriteString(renderTable(bl.Rows))
			}
		}
	}
	// sectPr last: header/footer refs + page geometry
	b.WriteString(`<w:sectPr>`)
	if hasHeader {
		b.WriteString(`<w:headerReference w:type="default" r:id="` + headerRID + `"/>`)
	}
	if hasFooter {
		b.WriteString(`<w:footerReference w:type="default" r:id="` + footerRID + `"/>`)
	}
	fmt.Fprintf(&b, `<w:pgSz w:w="%d" w:h="15840"/><w:pgMar w:top="1440" w:right="%d" w:bottom="1440" w:left="%d" w:header="720" w:footer="720"/></w:sectPr>`, pageW, marginR, marginL)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

// docxChartPara emits an inline chart graphicFrame referencing a c:chart part.
func docxChartPara(bl oir.Block, rid string, idx int) string {
	const cx, cy = 5486400, 3200400 // 6in x 3.5in
	t := bl.Title
	if t == "" {
		t = fmt.Sprintf("Chart %d", idx+1)
	}
	return `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:jc w:val="center"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="` + fmt.Sprint(cx) + `" cy="` + fmt.Sprint(cy) + `"/><wp:docPr id="` + fmt.Sprint(idx + 100) + `" name="` + esc(t) + `"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="` + rid + `"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`
}

// dividerPara is an empty paragraph with a bottom border (horizontal rule).
func dividerPara() string {
	return `<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="B4C6E7"/></w:pBdr></w:pPr></w:p>`
}

// pageBreakPara breaks the page.
func pageBreakPara() string {
	return `<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:br w:type="page"/></w:r></w:p>`
}

// tocDocx emits a Word TOC field (populated by Word on open via w:updateFields).
func tocDocx() string {
	return `<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Table of Contents"/><w:docPartUnique/></w:docPartObj></w:sdtPr><w:sdtContent>` +
		`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Table of Contents</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> TOC \\o "1-3" \\h \\z \\u </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>Table of contents will populate when opened in Word.</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p>` +
		`</w:sdtContent></w:sdt>`
}

func docxImagePara(im DocxImg, idx int) string {
	alt := im.Alt
	if alt == "" {
		alt = fmt.Sprintf("image%d", idx+1)
	}
	return `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:jc w:val="center"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="` + fmt.Sprint(im.Cx) + `" cy="` + fmt.Sprint(im.Cy) + `"/><wp:docPr id="` + fmt.Sprint(idx+2) + `" name="` + esc(alt) + `" descr="` + esc(alt) + `"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="` + fmt.Sprint(idx+2) + `" name="` + esc(alt) + `" descr="` + esc(alt) + `"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="` + im.RID + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + fmt.Sprint(im.Cx) + `" cy="` + fmt.Sprint(im.Cy) + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`
}

func docxContentTypes11(hasImg, hasHdr, hasFtr bool, chartNames []string) string {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="png" ContentType="image/png"/>` +
		`<Default Extension="jpeg" ContentType="image/jpeg"/>` +
		`<Default Extension="jpg" ContentType="image/jpeg"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
		`<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
		`<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>`
	if hasHdr {
		s += `<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>`
	}
	if hasFtr {
		s += `<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>`
	}
	for _, cn := range chartNames {
		s += fmt.Sprintf(`<Override PartName="/%s" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`, cn)
	}
	_ = hasImg
	s += `<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`
	return s
}

func docxDocRels11(imgNames []string, hasHdr, hasFtr bool, chartNames []string) string {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable" Target="fontTable.xml"/>`
	next := 4
	if hasHdr {
		s += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>`, next)
		next++
	}
	if hasFtr {
		s += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>`, next)
		next++
	}
	for _, nm := range imgNames {
		// nm like word/media/image1.png -> target media/image1.png
		target := strings.TrimPrefix(nm, "word/")
		s += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="%s"/>`, next, target)
		next++
	}
	for _, nm := range chartNames {
		target := strings.TrimPrefix(nm, "word/")
		s += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="%s"/>`, next, target)
		next++
	}
	s += `</Relationships>`
	return s
}

// docxHeaderFooterXML emits a header1.xml-equivalent part. When footer is
// true the root element becomes w:ftr (Word requires matching part tags).
func docxHeaderFooterXML(txt string, footer bool) string {
	tag := "w:hdr"
	if footer {
		tag = "w:ftr"
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<` + tag + ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:rPr><w:sz w:val="18"/><w:color w:val="808080"/></w:rPr><w:t xml:space="preserve">` + esc(txt) + `</w:t></w:r></w:p></` + tag + `>`
}

func docxRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func docxStyles() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/><w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="en-US"/></w:rPr></w:rPrDefault></w:docDefaults>` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="60"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:rFonts w:ascii="Calibri Light" w:hAnsi="Calibri Light"/><w:color w:val="2E74B5"/><w:sz w:val="32"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="200" w:after="40"/><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:rFonts w:ascii="Calibri Light" w:hAnsi="Calibri Light"/><w:color w:val="2E74B5"/><w:sz w:val="26"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="160" w:after="40"/><w:outlineLvl w:val="2"/></w:pPr><w:rPr><w:rFonts w:ascii="Calibri Light" w:hAnsi="Calibri Light"/><w:color w:val="2E74B5"/><w:sz w:val="24"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="quote"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:ind w:left="360" w:right="360"/><w:spacing w:before="80" w:after="80"/></w:pPr><w:rPr><w:i/><w:color w:val="595959"/></w:rPr></w:style>` +
		`</w:styles>`
}

func docxSettings(updateFields bool) string {
	update := ""
	if updateFields {
		update = `<w:updateFields w:val="true"/>`
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + update + `<w:defaultTabStop w:val="720"/><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat></w:settings>`
}

func docxFontTable() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:font w:name="Calibri"><w:panose1 w:val="020F0302020204030204"/><w:charset w:val="00"/><w:family w:val="swiss"/></w:font>` +
		`<w:font w:name="Calibri Light"><w:panose1 w:val="020F0302020204030204"/><w:charset w:val="00"/><w:family w:val="swiss"/></w:font>` +
		`<w:font w:name="Consolas"><w:panose1 w:val="020B0609020204030204"/><w:charset w:val="00"/><w:family w:val="modern"/></w:font>` +
		`</w:fonts>`
}

func para(style, txt string, mono, bold bool) string {
	pid := forger.ParaID()
	rpr := ""
	if mono {
		rpr = `<w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/><w:sz w:val="20"/></w:rPr>`
	} else if bold {
		rpr = `<w:rPr><w:b/></w:rPr>`
	}
	return `<w:p><w:pPr><w:pStyle w:val="` + style + `"/><w14:paraId w14:val="` + pid + `"/></w:pPr><w:r>` + rpr + `<w:t xml:space="preserve">` + esc(txt) + `</w:t></w:r></w:p>`
}

func renderTable(rows [][]oir.Cell) string {
	if len(rows) == 0 {
		return ""
	}
	ncols := len(rows[0])
	widths := layout.WordTableColWidths(availWidth, ncols)
	var b strings.Builder
	// ORDER IS LOAD-BEARING: tblStyle -> tblW -> tblLayout -> tblBorders -> tblCellMar
	b.WriteString(`<w:tbl><w:tblPr>`)
	b.WriteString(`<w:tblStyle w:val="TableGrid"/>`)
	b.WriteString(`<w:tblW w:w="0" w:type="auto"/>`)
	b.WriteString(`<w:tblLayout w:type="fixed"/>`)
	b.WriteString(`<w:tblBorders><w:top w:val="single" w:sz="4" w:color="B4C6E7"/><w:left w:val="single" w:sz="4" w:color="B4C6E7"/><w:bottom w:val="single" w:sz="4" w:color="B4C6E7"/><w:right w:val="single" w:sz="4" w:color="B4C6E7"/><w:insideH w:val="single" w:sz="4" w:color="B4C6E7"/><w:insideV w:val="single" w:sz="4" w:color="B4C6E7"/></w:tblBorders>`)
	b.WriteString(`<w:tblCellMar><w:left w:w="108" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar>`)
	b.WriteString(`</w:tblPr><w:tblGrid>`)
	for _, w := range widths {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, w)
	}
	b.WriteString(`</w:tblGrid>`)
	for ri, row := range rows {
		isHeader := ri == 0
		b.WriteString(`<w:tr>`)
		for ci, cell := range row {
			shd := ""
			if isHeader {
				shd = `<w:shd w:fill="D9E2F3" w:val="clear"/>`
			}
			fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>%s</w:tcPr>`, widths[ci], shd)
			txt := cellString(cell.V)
			if isHeader {
				b.WriteString(`<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">` + esc(txt) + `</w:t></w:r></w:p>`)
			} else {
				b.WriteString(`<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:t xml:space="preserve">` + esc(txt) + `</w:t></w:r></w:p>`)
			}
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}

func cellString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// avoid 100.000000
		if t == float64(int64(t)) {
			return fmt.Sprint(int64(t))
		}
		return fmt.Sprint(t)
	default:
		return fmt.Sprint(t)
	}
}

func countWords(spec *oir.Spec) int {
	n := 0
	for _, sec := range spec.Doc.Sections {
		for _, bl := range sec.Blocks {
			n += len(strings.Fields(bl.Txt))
			for _, r := range bl.Rows {
				for _, c := range r {
					n += len(strings.Fields(cellString(c.V)))
				}
			}
		}
	}
	return n
}

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", `'`, "&apos;")
	return r.Replace(s)
}
