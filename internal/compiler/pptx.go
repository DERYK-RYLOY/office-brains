// PPTX compiler: OIR -> Transitional PresentationML (stdlib only).
// Bootstrap renders blocks as native text shapes (title + body).
// Native a:tbl graphicFrame lands after user confirms basic deck opens clean.
package compiler

import (
	"fmt"
	"strings"

	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/media"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

// 16:9 M365 default: 13.33 x 7.5in in EMU (914400 per inch).
const (
	pptCX = 12192000
	pptCY = 6858000
)

// BuildPptx returns all ZIP parts. OIR 1.1: one slide per section, images embedded.
func BuildPptx(spec *oir.Spec, meta forger.Props) map[string][]byte {
	resetWarnings()
	meta.Defaults("Microsoft Office PowerPoint")
	meta.Words = countWords(spec)
	meta.Pages = len(spec.Doc.Sections)
	if meta.Pages < 1 {
		meta.Pages = 1
	}
	n := len(spec.Doc.Sections)
	type pimg = pptImg
	type pchart = pptChart
	slideImgs := make([][]pimg, n)
	slideCharts := make([][]pchart, n)
	mediaCounter := 0
	chartCounter := 0
	// layout pass: tables height + image/chart stacking per slide
	for si, sec := range spec.Doc.Sections {
		// estimate tables height like renderSlide: each table rows*320000
		tablesH := int64(0)
		for _, bl := range sec.Blocks {
			if bl.T == "table" {
				tablesH += int64(len(bl.Rows))*320000 + 200000
			}
		}
		y := int64(1554480+2000000) + tablesH
		for _, bl := range sec.Blocks {
			if bl.T != "image" {
				continue
			}
			m, err := media.Load(bl.Img)
			if err != nil {
				warn("slide %d: image %q not embedded (%v)", si+1, bl.Img, err)
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
			x := (12192000 - cx) / 2
			if y+cy > pptCY-200000 {
				warn("slide %d: image %q dropped (slide full, split content across sections)", si+1, bl.Alt)
				continue // try next block; y unchanged
			}
			mediaCounter++
			nm := fmt.Sprintf("ppt/media/image%d.%s", mediaCounter, m.Ext)
			slideImgs[si] = append(slideImgs[si], pimg{data: m.Data, ext: m.Ext, alt: bl.Alt, cx: cx, cy: cy, x: x, y: y, name: nm})
			y += cy + 200000
		}
		for _, bl := range sec.Blocks {
			if bl.T != "chart" {
				continue
			}
			cx, cy := int64(8000000), int64(2800000)
			x := (12192000 - cx) / 2
			if y+cy > pptCY-200000 {
				warn("slide %d: chart %q dropped (slide full, split content across sections)", si+1, bl.Title)
				continue
			}
			chartCounter++
			nm := fmt.Sprintf("ppt/charts/chart%d.xml", chartCounter)
			slideCharts[si] = append(slideCharts[si], pchart{title: bl.Title, kind: bl.Kind, labels: bl.Labels, series: bl.Series, x: x, y: y, cx: cx, cy: cy, name: nm})
			y += cy + 200000
		}
	}
	hasMedia := mediaCounter > 0
	hasCharts := chartCounter > 0
	chartNames := []string{}
	for _, chs := range slideCharts {
		for _, ch := range chs {
			chartNames = append(chartNames, ch.name)
		}
	}
	parts := map[string][]byte{
		"[Content_Types].xml":             []byte(pptxContentTypesMediaCharts(n, hasMedia, chartNames)),
		"_rels/.rels":                     []byte(pptxRootRels()),
		"ppt/_rels/presentation.xml.rels": []byte(pptxPresRelsN(n)),
		"ppt/presentation.xml":            []byte(pptxPresentationN(n)),
		"ppt/slideLayouts/slideLayout1.xml":             []byte(pptxLayout()),
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels":  []byte(pptxLayoutRels()),
		"ppt/slideMasters/slideMaster1.xml":             []byte(pptxMaster()),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels":  []byte(pptxMasterRels()),
		"ppt/theme/theme1.xml":                          []byte(pptxTheme()),
		"docProps/core.xml":                             forger.CoreXML(meta),
		"docProps/app.xml":                              forger.AppXML(meta, 8),
	}
	for i, sec := range spec.Doc.Sections {
		parts["ppt/slides/slide"+itoa(i+1)+".xml"] = []byte(renderSectionSlideFull(sec, slideImgs[i], slideCharts[i]))
		parts["ppt/slides/_rels/slide"+itoa(i+1)+".xml.rels"] = []byte(pptxSlideRelsFull(slideImgs[i], slideCharts[i]))
	}
	for _, imgs := range slideImgs {
		for _, im := range imgs {
			parts[im.name] = im.data
		}
	}
	for _, chs := range slideCharts {
		for _, ch := range chs {
			parts[ch.name] = []byte(ChartXML(ch.kind, ch.title, ch.labels, ch.series))
		}
	}
	_ = hasCharts
	return parts
}

type pptChart struct {
	title  string
	kind   string
	labels []string
	series []oir.ChartSeries
	x      int64
	y      int64
	cx     int64
	cy     int64
	name   string
}

func pptxContentTypesMedia(n int, hasMedia bool) string {
	return pptxContentTypesMediaCharts(n, hasMedia, nil)
}

func pptxContentTypesMediaCharts(n int, hasMedia bool, chartNames []string) string {
	base := pptxContentTypesN(n)
	if hasMedia {
		base = strings.Replace(base, `<Default Extension="xml"`, `<Default Extension="png" ContentType="image/png"/><Default Extension="jpg" ContentType="image/jpeg"/><Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="xml"`, 1)
	}
	for _, cn := range chartNames {
		base = strings.Replace(base, `</Types>`, fmt.Sprintf(`<Override PartName="/%s" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/></Types>`, cn), 1)
	}
	return base
}

func pptxSlideRelsFull(imgs []pptImg, charts []pptChart) string {
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`
	next := 2
	for _, im := range imgs {
		mediaTarget := "../" + strings.TrimPrefix(im.name, "ppt/")
		s += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="%s"/>`, next, mediaTarget)
		next++
	}
	for _, ch := range charts {
		chartTarget := "../" + strings.TrimPrefix(ch.name, "ppt/")
		s += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="%s"/>`, next, chartTarget)
		next++
	}
	s += `</Relationships>`
	return s
}

// renderSectionSlideFull renders section + pics + chart graphicFrames.
func renderSectionSlideFull(sec oir.Section, imgs []pptImg, charts []pptChart) string {
	base := renderSectionSlideImages(sec, imgs)
	if len(charts) == 0 {
		return base
	}
	frames := ""
	for i, ch := range charts {
		rid := fmt.Sprintf("rId%d", len(imgs)+2+i)
		t := ch.title
		if t == "" {
			t = fmt.Sprintf("chart%d", i+1)
		}
		frames += fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="%s"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="%s"/></a:graphicData></a:graphic></p:graphicFrame>`, 20+i, esc(t), ch.x, ch.y, ch.cx, ch.cy, rid)
	}
	return strings.Replace(base, `</p:spTree>`, frames+`</p:spTree>`, 1)
}

type pptImg struct {
	data []byte
	ext  string
	alt  string
	cx   int64
	cy   int64
	x    int64
	y    int64
	name string
}

func pptxContentTypesN(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i+1)
	}
	b.WriteString(`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`)
	return b.String()
}

func pptxPresRelsN(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+1, i+1)
	}
	fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`, n+1)
	fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`, n+2)
	b.WriteString(`</Relationships>`)
	return b.String()
}

func pptxPresentationN(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		fmt.Sprintf(`<p:sldMasterIdLst><p:sldMasterId r:id="rId%d"/></p:sldMasterIdLst><p:sldIdLst>`, n+1))
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+1)
	}
	fmt.Fprintf(&b, `</p:sldIdLst><p:sldSz cx="%d" cy="%d" type="screen16x9"/></p:presentation>`, pptCX, pptCY)
	return b.String()
}

// renderSectionSlideImages renders one section + positioned pictures.
func renderSectionSlideImages(sec oir.Section, imgs []pptImg) string {
	base := renderSlide(&oir.Spec{OIR: "1.1", Doc: oir.Doc{Sections: []oir.Section{sec}}})
	if len(imgs) == 0 {
		return base
	}
	pics := ""
	for i, im := range imgs {
		alt := im.alt
		if alt == "" {
			alt = fmt.Sprintf("image%d", i+1)
		}
		rid := fmt.Sprintf("rId%d", i+2) // rId1 = layout
		pics += fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="%s" descr="%s"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`, 10+i, esc(alt), esc(alt), rid, im.x, im.y, im.cx, im.cy)
	}
	// inject pics before </p:spTree>
	_ = pics
	return strings.Replace(base, `</p:spTree>`, pics+`</p:spTree>`, 1)
}

func hasChartBlock(sec oir.Section) bool {
	for _, bl := range sec.Blocks {
		if bl.T == "chart" || bl.T == "image" {
			return true
		}
	}
	return false
}

func pptxRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func pptxLayoutRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
		`</Relationships>`
}

func pptxLayout() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" type="titleAndContent" preserve="1">` +
		`<p:cSld name="Title and Content"><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func pptxMaster() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:effectLst/></p:bgPr></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>` +
		`<p:txStyles><p:titleStyle><a:lvl1pPr><a:defRPr sz="4400"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="Calibri Light"/></a:defRPr></a:lvl1pPr></p:titleStyle><p:bodyStyle><a:lvl1pPr><a:defRPr sz="1800"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="Calibri"/></a:defRPr></a:lvl1pPr></p:bodyStyle><p:otherStyle><a:lvl1pPr><a:defRPr sz="1800"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="Calibri"/></a:defRPr></a:lvl1pPr></p:otherStyle></p:txStyles>` +
		`</p:sldMaster>`
}

func pptxMasterRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
		`</Relationships>`
}

func pptxTheme() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">` +
		`<a:themeElements>` +
		`<a:clrScheme name="Office">` +
		`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
		`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
		`<a:dk2><a:srgbClr val="44546A"/></a:dk2>` +
		`<a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>` +
		`<a:accent1><a:srgbClr val="4472C4"/></a:accent1>` +
		`<a:accent2><a:srgbClr val="ED7D31"/></a:accent2>` +
		`<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3>` +
		`<a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
		`<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5>` +
		`<a:accent6><a:srgbClr val="70AD47"/></a:accent6>` +
		`<a:hlink><a:srgbClr val="0563C1"/></a:hlink>` +
		`<a:folHlink><a:srgbClr val="9542EB"/></a:folHlink>` +
		`</a:clrScheme>` +
		`<a:fmtScheme name="Office">` +
		`<a:fillStyleLst>` +
		`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:gradFill rotWithShape="0"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="50000"/><a:satMod val="300000"/></a:schemeClr></a:gs><a:gs pos="35000"><a:schemeClr val="phClr"><a:tint val="37000"/><a:satMod val="300000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:tint val="15000"/><a:satMod val="350000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="16200000" scaled="1"/></a:gradFill>` +
		`<a:gradFill rotWithShape="0"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:shade val="51000"/><a:satMod val="130000"/></a:schemeClr></a:gs><a:gs pos="80000"><a:schemeClr val="phClr"><a:shade val="93000"/><a:satMod val="130000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="94000"/><a:satMod val="135000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="16200000" scaled="0"/></a:gradFill>` +
		`</a:fillStyleLst>` +
		`<a:lnStyleLst>` +
		`<a:ln w="6350" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln>` +
		`<a:ln w="12700" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln>` +
		`<a:ln w="19050" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/></a:ln>` +
		`</a:lnStyleLst>` +
		`<a:effectStyleLst>` +
		`<a:effectStyle><a:effectLst><a:outerShdw blurRad="40000" dist="20000" dir="5400000" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="35000"/></a:srgbClr></a:outerShdw></a:effectLst></a:effectStyle>` +
		`<a:effectStyle><a:effectLst><a:outerShdw blurRad="40000" dist="23000" dir="5400000" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="35000"/></a:srgbClr></a:outerShdw></a:effectLst></a:effectStyle>` +
		`<a:effectStyle><a:effectLst><a:outerShdw blurRad="40000" dist="23000" dir="5400000" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="35000"/></a:srgbClr></a:outerShdw></a:effectLst><a:scene3d><a:camera prst="orthographicFront"><a:rot lat="0" lon="0" rev="0"/></a:camera><a:lightRig rig="threePt" dir="t"><a:rot lat="0" lon="0" rev="12000000"/></a:lightRig></a:scene3d><a:sp3d><a:bevelT w="63500" h="25400"/></a:sp3d></a:effectStyle>` +
		`</a:effectStyleLst>` +
		`<a:bgFillStyleLst>` +
		`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:gradFill rotWithShape="0"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="40000"/><a:satMod val="350000"/></a:schemeClr></a:gs><a:gs pos="40000"><a:schemeClr val="phClr"><a:tint val="45000"/><a:shade val="99000"/><a:satMod val="350000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="20000"/><a:satMod val="255000"/></a:schemeClr></a:gs></a:gsLst><a:path path="circle"><a:fillToRect l="50000" t="-80000" r="50000" b="180000"/></a:path></a:gradFill>` +
		`<a:gradFill rotWithShape="0"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="80000"/><a:satMod val="300000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="30000"/><a:satMod val="200000"/></a:schemeClr></a:gs></a:gsLst><a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path></a:gradFill>` +
		`</a:bgFillStyleLst>` +
		`</a:fmtScheme>` +
		`<a:fontScheme name="Office">` +
		`<a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
		`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>` +
		`</a:fontScheme>` +
		`</a:themeElements></a:theme>`
}

// renderSlide builds 1 slide: title shape + body shape + native tables.
// Tables become editable a:tbl graphicFrames stacked below body text.
func renderSlide(spec *oir.Spec) string {
	title := "Untitled"
	bodyParas := []pptPara{}
	tables := [][][]oir.Cell{}
	hasVisuals := false
	for _, sec := range spec.Doc.Sections {
		if hasChartBlock(sec) {
			hasVisuals = true
		}
		for _, bl := range sec.Blocks {
			switch bl.T {
			case "h1":
				if title == "Untitled" {
					title = bl.Txt
				} else {
					bodyParas = append(bodyParas, pptPara{Text: bl.Txt, Bold: true, Size: 2800})
				}
			case "h2":
				bodyParas = append(bodyParas, pptPara{Text: bl.Txt, Bold: true, Size: 2400})
			case "p":
				bodyParas = append(bodyParas, pptPara{Text: bl.Txt, Size: 1800})
			case "bullet":
				bodyParas = append(bodyParas, pptPara{Text: "•  " + bl.Txt, Size: 1800, Bullet: true})
			case "code":
				bodyParas = append(bodyParas, pptPara{Text: bl.Txt, Size: 1600, Mono: true})
			case "table":
				cp := make([][]oir.Cell, len(bl.Rows))
				for i, r := range bl.Rows {
					rr := make([]oir.Cell, len(r))
					copy(rr, r)
					cp[i] = rr
				}
				tables = append(tables, cp)
			case "chart":
				// counted by caller for layout; no body text
			case "image":
				// pics positioned by caller; no body text
			}
		}
	}
	if len(bodyParas) == 0 && len(tables) == 0 {
		bodyParas = []pptPara{{Text: "", Size: 1800}}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree>`)
	b.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	// title shape: x=0.5in y=0.3in w=12.33in h=1.2in
	b.WriteString(pptShape(2, "Title", 457200, 274320, 11273880, 1097280, []pptPara{{Text: title, Bold: true, Size: 3600}}))
	nextID := 3
	if len(bodyParas) > 0 {
		// body shape shrinks when tables/charts exist (top half), else full
		var by, bh int64 = 1554480, 4846320
		if len(tables) > 0 || hasVisuals {
			bh = 1800000
		}
		b.WriteString(pptShape(nextID, "Body", 457200, by, 11273880, bh, bodyParas))
		nextID++
	}
	y := int64(1554480 + 2000000)
	for ti, t := range tables {
		h := int64(len(t)) * 320000 // ~0.35in per row
		if y+h > pptCY-300000 {
			h = pptCY - 300000 - y // clamp to slide
		}
		b.WriteString(pptTableFrame(nextID, 457200, y, 11273880, h, t))
		nextID++
		y += h + 200000
		if y > pptCY-400000 {
			if ti+1 < len(tables) {
				warn("slide: %d table(s) dropped (slide full, split content across sections)", len(tables)-ti-1)
			}
			break // bootstrap: one slide only, overflow truncated by design
		}
	}
	b.WriteString(`</p:spTree></p:cSld></p:sld>`)
	return b.String()
}

// pptTableFrame emits an editable DrawingML table (a:tbl) in a graphicFrame.
func pptTableFrame(id int, x, y, cx, cy int64, rows [][]oir.Cell) string {
	if len(rows) == 0 {
		return ""
	}
	ncols := len(rows[0])
	if ncols == 0 {
		return ""
	}
	colW := cx / int64(ncols)
	rowH := cy / int64(len(rows))
	if rowH < 200000 {
		rowH = 200000
	}
	var b strings.Builder
	b.WriteString(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="` + fmt.Sprint(id) + `" name="Table"/>` +
		`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`)
	fmt.Fprintf(&b, `<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, x, y, cx, cy)
	b.WriteString(`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/><a:tblGrid>`)
	for i := 0; i < ncols; i++ {
		fmt.Fprintf(&b, `<a:gridCol w="%d"/>`, colW)
	}
	b.WriteString(`</a:tblGrid>`)
	for ri, r := range rows {
		fmt.Fprintf(&b, `<a:tr h="%d">`, rowH)
		for ci := 0; ci < ncols; ci++ {
			txt := ""
			if ci < len(r) {
				txt = cellString(r[ci].V)
				if r[ci].F != "" {
					txt = cellString(r[ci].V) // cached display
				}
			}
			bold := ri == 0
			fill := ""
			if bold {
				fill = `<a:tcPr><a:solidFill><a:srgbClr val="D9E2F3"/></a:solidFill></a:tcPr>`
			}
			b.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr`)
			if bold {
				b.WriteString(` b="1"`)
			}
			b.WriteString(` sz="1400"><a:latin typeface="Calibri"/></a:rPr><a:t xml:space="preserve">` + esc(txt) + `</a:t></a:r></a:p></a:txBody>` + fill + `</a:tc>`)
		}
		b.WriteString(`</a:tr>`)
	}
	b.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	return b.String()
}

type pptPara struct {
	Text   string
	Bold   bool
	Size   int // 100ths of point, 1800 = 18pt
	Mono   bool
	Bullet bool
}

func pptShape(id int, name string, x, y, cx, cy int64, paras []pptPara) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/>`, id, esc(name), x, y, cx, cy)
	for _, p := range paras {
		b.WriteString(`<a:p>`)
		if p.Bullet {
			b.WriteString(`<a:pPr marL="342900" indent="-342900"><a:buChar char="•"/>`)
		} else {
			b.WriteString(`<a:pPr>`)
		}
		if p.Bold {
			// bold handled at run level below; keep pPr minimal
		}
		b.WriteString(`</a:pPr><a:r><a:rPr`)
		if p.Bold {
			b.WriteString(` b="1"`)
		}
		fmt.Fprintf(&b, ` sz="%d"`, p.Size)
		if p.Mono {
			b.WriteString(`><a:latin typeface="Consolas"/>`)
		} else {
			b.WriteString(`><a:latin typeface="Calibri"/>`)
		}
		b.WriteString(`</a:rPr><a:t xml:space="preserve">` + esc(p.Text) + `</a:t></a:r></a:p>`)
	}
	b.WriteString(`</p:txBody></p:sp>`)
	return b.String()
}
