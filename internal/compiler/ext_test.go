package compiler

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func writeTinyPNG(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dot.png")
	b, _ := base64.StdEncoding.DecodeString(tinyPNG)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func buildSpec(t *testing.T, body string) *oir.Spec {
	t.Helper()
	s, err := oir.ParseAndValidate([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDocxNativeChart(t *testing.T) {
	spec := buildSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h1","txt":"Report"},
		{"t":"chart","kind":"bar","title":"Rev","labels":["A","B"],"series":[{"name":"2026","values":[10,20]}]}
	]}]}}`)
	parts := BuildDocx(spec, forger.Props{Profile: forger.M365})
	chart, ok := parts["word/charts/chart1.xml"]
	if !ok {
		t.Fatal("docx must embed a native chart part")
	}
	if !strings.Contains(string(chart), "c:chartSpace") {
		t.Fatal("chart part malformed")
	}
	ct := string(parts["[Content_Types].xml"])
	if !strings.Contains(ct, "word/charts/chart1.xml") {
		t.Fatal("content types missing chart override")
	}
	rels := string(parts["word/_rels/document.xml.rels"])
	if !strings.Contains(rels, "relationships/chart") {
		t.Fatal("document rels missing chart relationship")
	}
	doc := string(parts["word/document.xml"])
	if !strings.Contains(doc, "drawingml/2006/chart") {
		t.Fatal("document.xml must reference the chart graphicFrame")
	}
}

func TestDocxTOCSetsUpdateFields(t *testing.T) {
	spec := buildSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"toc"},
		{"t":"h1","txt":"Intro"}
	]}]}}`)
	parts := BuildDocx(spec, forger.Props{Profile: forger.M365})
	settings := string(parts["word/settings.xml"])
	if !strings.Contains(settings, "updateFields") {
		t.Fatal("settings.xml must carry w:updateFields when a toc block is present")
	}
	doc := string(parts["word/document.xml"])
	if !strings.Contains(doc, "TOC") {
		t.Fatal("document.xml must contain the TOC field")
	}
	// without a toc block, no updateFields
	parts2 := BuildDocx(buildSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"x"}]}]}}`), forger.Props{Profile: forger.M365})
	if strings.Contains(string(parts2["word/settings.xml"]), "updateFields") {
		t.Fatal("updateFields must not be emitted without a toc block")
	}
}

func TestPptxImageScaledNotDropped(t *testing.T) {
	img := writeTinyPNG(t)
	spec := buildSpec(t, fmt.Sprintf(`{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h1","txt":"Title"},
		{"t":"table","rows":[["H1","H2"],["a","b"],["c","d"],["e","f"]]},
		{"t":"image","img":%q,"alt":"dot","w":4.0}
	]}]}}`, img))
	resetWarnings()
	parts := BuildPptx(spec, forger.Props{Profile: forger.M365})
	foundMedia := false
	for name := range parts {
		if strings.HasPrefix(name, "ppt/media/") {
			foundMedia = true
		}
	}
	if !foundMedia {
		t.Fatalf("image must be embedded (scaled), not dropped. warnings: %v", Truncated)
	}
}

func TestPptxPageBreakSplitsSlides(t *testing.T) {
	spec := buildSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h1","txt":"One"},
		{"t":"pagebreak"},
		{"t":"h1","txt":"Two"}
	]}]}}`)
	parts := BuildPptx(spec, forger.Props{Profile: forger.M365})
	if _, ok := parts["ppt/slides/slide1.xml"]; !ok {
		t.Fatal("slide1 missing")
	}
	if _, ok := parts["ppt/slides/slide2.xml"]; !ok {
		t.Fatal("pagebreak must split into slide2")
	}
	if !strings.Contains(string(parts["ppt/presentation.xml"]), `r:id="rId2"`) {
		t.Fatal("presentation must reference both slides")
	}
}

func TestXlsxFreezeHeader(t *testing.T) {
	spec := buildSpec(t, `{"oir":"1.1","meta":{"freeze_header":true},"doc":{"sections":[{"blocks":[{"t":"table","rows":[["H"],["x"]]}]}]}}`)
	parts := BuildXlsx(spec, forger.Props{Profile: forger.M365})
	sheet := string(parts["xl/worksheets/sheet1.xml"])
	if !strings.Contains(sheet, `state="frozen"`) {
		t.Fatal("freeze_header must emit a frozen pane")
	}
	parts2 := BuildXlsx(buildSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"table","rows":[["H"],["x"]]}]}]}}`), forger.Props{Profile: forger.M365})
	if strings.Contains(string(parts2["xl/worksheets/sheet1.xml"]), `state="frozen"`) {
		t.Fatal("no frozen pane without freeze_header")
	}
}
