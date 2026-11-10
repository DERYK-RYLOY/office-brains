package preview

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func renderSpec(t *testing.T, body, docType string) string {
	t.Helper()
	s, err := oir.ParseAndValidate([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return Render(s, docType)
}

func TestDocxPreviewRendersImageAndChart(t *testing.T) {
	img := writeTinyPNG(t)
	html := renderSpec(t, fmt.Sprintf(`{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h1","txt":"T"},
		{"t":"image","img":%q,"alt":"dot"},
		{"t":"chart","kind":"bar","title":"R","labels":["A"],"series":[{"name":"s","values":[3]}]}
	]}]}}`, img), "docx")
	if !strings.Contains(html, "data:image/png;base64") {
		t.Fatal("docx preview must embed the image")
	}
	if !strings.Contains(html, "<svg") {
		t.Fatal("docx preview must render the chart as SVG")
	}
}

func TestPptxPreviewRendersEverySlide(t *testing.T) {
	html := renderSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h1","txt":"One"}
	]},{"blocks":[
		{"t":"h1","txt":"Two"}
	]}]}}`, "pptx")
	if n := strings.Count(html, `class="slide"`); n != 2 {
		t.Fatalf("pptx preview must render both slides, got %d", n)
	}
	if !strings.Contains(html, "One") || !strings.Contains(html, "Two") {
		t.Fatal("slide content missing")
	}
}

func TestPptxPreviewShowsImageAndChart(t *testing.T) {
	img := writeTinyPNG(t)
	html := renderSpec(t, fmt.Sprintf(`{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h1","txt":"T"},
		{"t":"image","img":%q,"alt":"dot","w":2.0},
		{"t":"chart","kind":"pie","title":"Share","labels":["X","Y"],"series":[{"name":"s","values":[60,40]}]}
	]}]}}`, img), "pptx")
	if !strings.Contains(html, "data:image/png;base64") {
		t.Fatal("pptx preview must embed the image")
	}
	if !strings.Contains(html, "<svg") {
		t.Fatal("pptx preview must render the chart as SVG")
	}
}

func TestXlsxPreviewFreezeBadge(t *testing.T) {
	html := renderSpec(t, `{"oir":"1.1","meta":{"freeze_header":true},"doc":{"sections":[{"blocks":[{"t":"table","rows":[["H"],["x"]]}]}]}}`, "xlsx")
	if !strings.Contains(html, "freeze_header") {
		t.Fatal("xlsx preview must show the freeze_header badge")
	}
}

func TestPreviewHandlesNewBlocks(t *testing.T) {
	html := renderSpec(t, `{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h3","txt":"Sub"},
		{"t":"quote","txt":"Wisdom"},
		{"t":"divider"},
		{"t":"toc"}
	]}]}}`, "docx")
	for _, want := range []string{"Sub", "Wisdom", "tocbox"} {
		if !strings.Contains(html, want) {
			t.Fatalf("docx preview missing %q", want)
		}
	}
}
