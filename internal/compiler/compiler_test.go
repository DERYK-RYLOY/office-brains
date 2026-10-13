package compiler

import (
	"strings"
	"testing"

	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
)

func testSpec(t *testing.T) *oir.Spec {
	t.Helper()
	s, err := oir.ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"name":"S","blocks":[{"t":"h1","txt":"Hi"},{"t":"table","rows":[["a","b"],[1,2]]}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDocxParts(t *testing.T) {
	parts := BuildDocx(testSpec(t), forger.Props{Profile: forger.M365})
	for _, need := range []string{"[Content_Types].xml", "word/document.xml", "word/styles.xml", "docProps/core.xml"} {
		if _, ok := parts[need]; !ok {
			t.Fatalf("missing %s", need)
		}
	}
	doc := string(parts["word/document.xml"])
	if strings.Contains(doc, "purl.oclc.org") {
		t.Fatal("strict namespace leaked")
	}
	order := []string{"tblStyle", "tblW", "tblLayout", "tblBorders", "tblCellMar"}
	last := -1
	for _, o := range order {
		i := strings.Index(doc, o)
		if i < 0 || i < last {
			t.Fatalf("tblPr order broken at %s", o)
		}
		last = i
	}
}

func TestXlsxNoCalcChain(t *testing.T) {
	parts := BuildXlsx(testSpec(t), forger.Props{Profile: forger.M365})
	if _, ok := parts["xl/calcChain.xml"]; ok {
		t.Fatal("calcChain must never be emitted")
	}
	if !strings.Contains(string(parts["xl/workbook.xml"]), `fullCalcOnLoad="1"`) {
		t.Fatal("fullCalcOnLoad missing")
	}
	if !strings.Contains(string(parts["xl/worksheets/sheet1.xml"]), "<dimension") {
		t.Fatal("dimension missing")
	}
}

func TestPptxSlide(t *testing.T) {
	parts := BuildPptx(testSpec(t), forger.Props{Profile: forger.M365})
	if _, ok := parts["ppt/slides/slide1.xml"]; !ok {
		t.Fatal("slide1 missing")
	}
	if !strings.Contains(string(parts["ppt/presentation.xml"]), "screen16x9") {
		t.Fatal("16:9 missing")
	}
}

func TestFrozenDeterministic(t *testing.T) {
	old := forger.Frozen
	forger.Frozen = true
	defer func() { forger.Frozen = old }()
	forger.ResetCounter()
	a := BuildDocx(testSpec(t), forger.Props{Profile: forger.M365, Frozen: true})
	forger.ResetCounter()
	b := BuildDocx(testSpec(t), forger.Props{Profile: forger.M365, Frozen: true})
	if len(a) != len(b) {
		t.Fatal("part count differs")
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || string(va) != string(vb) {
			t.Fatalf("part %s differs", k)
		}
	}
	forger.ResetCounter()
}

func TestChartKinds(t *testing.T) {
	for _, kind := range []string{"bar", "line", "pie"} {
		x := ChartXML(kind, "T", []string{"a"}, []oir.ChartSeries{{Name: "s", Values: []float64{1}}})
		if !strings.Contains(x, "c:chartSpace") {
			t.Fatalf("%s: no chartSpace", kind)
		}
	}
	if !strings.Contains(ChartXML("line", "T", []string{"a"}, []oir.ChartSeries{{Values: []float64{1}}}), "lineChart") {
		t.Fatal("line kind must emit lineChart")
	}
	if !strings.Contains(ChartXML("pie", "T", []string{"a"}, []oir.ChartSeries{{Values: []float64{1}}}), "pieChart") {
		t.Fatal("pie kind must emit pieChart")
	}
}
