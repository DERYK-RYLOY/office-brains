// Package verify runs offline OOXML checks (no XSD dep yet).
package verify

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
)

// Result is one check outcome.
type Result struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Info string `json:"info"`
}

// File validates an Office ZIP on disk. Read-only.
func File(path string) (results []Result, ok bool) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return []Result{{Name: "zip_open", OK: false, Info: err.Error()}}, false
	}
	defer zr.Close()
	parts := map[string]string{}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(io.LimitReader(rc, 2<<20))
			rc.Close()
			parts[f.Name] = string(b)
		} else {
			parts[f.Name] = ""
		}
	}
	has := func(n string) bool { _, o := parts[n]; return o }
	add := func(name string, pass bool, info string) {
		results = append(results, Result{Name: name, OK: pass, Info: info})
	}

	// 1. content types exists
	add("has_content_types", has("[Content_Types].xml"), "[Content_Types].xml present")
	// 2. no Strict namespaces (must be Transitional)
	strict := 0
	for n, c := range parts {
		if strings.Contains(c, "purl.oclc.org/ooxml") {
			strict++
			add("strict_ns:"+n, false, "Strict namespace found, must be Transitional")
		}
	}
	if strict == 0 {
		add("transitional_ns", true, "no Strict purl.oclc.org namespaces")
	}
	// 3. no provenance markers
	markers := []string{"python-docx", "excelize", "openpyxl", "Codex", "OpenAI"}
	hits := 0
	for n, c := range parts {
		low := strings.ToLower(c)
		for _, m := range markers {
			if strings.Contains(low, strings.ToLower(m)) {
				hits++
				add("provenance:"+n, false, "contains "+m)
			}
		}
	}
	if hits == 0 {
		add("provenance_clean", true, "no AI lib markers")
	}
	// 4. type-specific
	switch {
	case has("word/document.xml"):
		add("type", true, "docx")
		doc := parts["word/document.xml"]
		// tblPr order
		order := []string{"tblStyle", "tblW", "tblLayout", "tblBorders", "tblCellMar"}
		idx := make([]int, len(order))
		for i, o := range order {
			idx[i] = strings.Index(doc, o)
		}
		ordered := true
		last := -1
		found := false
		for _, v := range idx {
			if v < 0 {
				continue
			}
			found = true
			if v < last {
				ordered = false
			}
			last = v
		}
		if !found {
			add("docx_tblPr_order", true, "no tables, order N/A")
		} else {
			add("docx_tblPr_order", ordered, fmt.Sprint(idx))
		}
		// sectPr last in body
		si := strings.LastIndex(doc, "<w:sectPr>")
		bi := strings.LastIndex(doc, "</w:body>")
		add("docx_sectPr_last", si > 0 && bi > 0 && si < bi, "sectPr must precede /body")
		// paraId present
		add("docx_paraId", strings.Contains(doc, "w14:paraId"), "w14:paraId present")
		// 1.1 chrome: header/footer/images (informational pass if absent, checked if present)
		if strings.Contains(doc, "headerReference") {
			add("docx_header_ref", has("word/header1.xml"), "header ref resolves")
		}
		if strings.Contains(doc, "footerReference") {
			add("docx_footer_ref", has("word/footer1.xml"), "footer ref resolves")
		}
		if strings.Contains(doc, "a:blip") {
			hasMedia := false
			for n := range parts {
				if len(n) >= 11 && n[:11] == "word/media/" {
					hasMedia = true
				}
			}
			add("docx_image_embed", hasMedia, "blip has media part")
		}
	case has("xl/workbook.xml"):
		// xlsx vs pptx share xl/ prefix? pptx uses ppt/. be precise
		if has("ppt/presentation.xml") {
			add("type", true, "pptx")
			pres := parts["ppt/presentation.xml"]
			add("pptx_sldSz_16x9", strings.Contains(pres, `type="screen16x9"`), "16:9 size tag")
			add("pptx_slide_exists", has("ppt/slides/slide1.xml"), "slide1 present")
			break
		}
		add("type", true, "xlsx")
		add("xlsx_no_calcChain", !has("xl/calcChain.xml"), "calcChain must be absent")
		wb := parts["xl/workbook.xml"]
		add("xlsx_fullCalcOnLoad", strings.Contains(wb, `fullCalcOnLoad="1"`), "fullCalcOnLoad=1")
		// multisheet: count sheets vs sheet parts
		sheetCount := strings.Count(wb, "<sheet ")
		sheetParts := 0
		for n := range parts {
			if len(n) >= 14 && n[:14] == "xl/worksheets/" && strings.HasSuffix(n, ".xml") && !strings.Contains(n, "_rels") {
				sheetParts++
			}
		}
		add("xlsx_multisheet", sheetParts == sheetCount && sheetCount >= 1, fmt.Sprintf("%d sheets, %d parts", sheetCount, sheetParts))
		sh := parts["xl/worksheets/sheet1.xml"]
		add("xlsx_dimension", strings.Contains(sh, "<dimension"), "dimension ref present")
		add("xlsx_sharedStrings", has("xl/sharedStrings.xml"), "sharedStrings present")
		add("xlsx_no_inlineStr", !strings.Contains(sh, "<is>"), "no inlineStr (must use sharedStrings)")
		// merges informational
		hasMerge := false
		for n, c := range parts {
			if len(n) >= 14 && n[:14] == "xl/worksheets/" && strings.Contains(c, "<mergeCell") {
				hasMerge = true
			}
		}
		if hasMerge {
			add("xlsx_merges", true, "mergeCells present")
		}
		// drawings/images: refs must resolve
		hasDrawingRef := false
		for n, c := range parts {
			if len(n) >= 14 && n[:14] == "xl/worksheets/" && strings.HasSuffix(n, ".xml") && !strings.Contains(n, "_rels") && strings.Contains(c, "<drawing") {
				hasDrawingRef = true
			}
		}
		if hasDrawingRef {
			hasDrawing := false
			hasMedia := false
			drawingHasPic := false
			for n, c := range parts {
				if len(n) >= 12 && n[:12] == "xl/drawings/" && strings.HasSuffix(n, ".xml") && !strings.Contains(n, "_rels") {
					hasDrawing = true
					if strings.Contains(c, "a:blip") || strings.Contains(c, "<xdr:pic") {
						drawingHasPic = true
					}
				}
				if len(n) >= 9 && n[:9] == "xl/media/" {
					hasMedia = true
				}
			}
			add("xlsx_drawing", hasDrawing, "drawing parts resolve")
			if drawingHasPic {
				add("xlsx_image_embed", hasMedia, "media parts resolve")
			}
			// charts
			hasChart := false
			for n := range parts {
				if len(n) >= 10 && n[:10] == "xl/charts/" {
					hasChart = true
				}
			}
			if hasChart {
				add("xlsx_chart", true, "chart parts present")
			}
		}
	case has("ppt/presentation.xml"):
		add("type", true, "pptx")
		pres := parts["ppt/presentation.xml"]
		add("pptx_sldSz_16x9", strings.Contains(pres, `type="screen16x9"`), "16:9 size tag")
		slideCount := strings.Count(pres, "<p:sldId ")
		slideParts := 0
		for n := range parts {
			if len(n) > 11 && n[:11] == "ppt/slides/" && strings.HasSuffix(n, ".xml") && !strings.Contains(n, "_rels") {
				slideParts++
			}
		}
		add("pptx_multislide", slideParts == slideCount && slideCount >= 1, fmt.Sprintf("%d slides, %d parts", slideCount, slideParts))
		// ppt images: p:pic must resolve to ppt/media
		hasPic := false
		for n, c := range parts {
			if len(n) >= 11 && n[:11] == "ppt/slides/" && strings.HasSuffix(n, ".xml") && !strings.Contains(n, "_rels") && strings.Contains(c, "<p:pic") {
				hasPic = true
			}
		}
		if hasPic {
			hasMedia := false
			for n := range parts {
				if len(n) >= 10 && n[:10] == "ppt/media/" {
					hasMedia = true
				}
			}
			add("pptx_image_embed", hasMedia, "media parts resolve")
		}
		hasChart := false
		for n := range parts {
			if len(n) >= 11 && n[:11] == "ppt/charts/" {
				hasChart = true
			}
		}
		if hasChart {
			add("pptx_chart", true, "chart parts present")
		}
	case has("ppt/presentation.xml"):
		add("type", true, "pptx")
		pres := parts["ppt/presentation.xml"]
		add("pptx_sldSz_16x9", strings.Contains(pres, `type="screen16x9"`), "16:9 size tag")
		add("pptx_slide_exists", has("ppt/slides/slide1.xml"), "slide1 present")
	default:
		add("type", false, "unknown: no word/document.xml, xl/workbook.xml, ppt/presentation.xml")
	}

	ok = true
	for _, r := range results {
		if !r.OK {
			ok = false
			break
		}
	}
	return results, ok
}
