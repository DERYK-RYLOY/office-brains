package surgeon

import (
	"bytes"
	"testing"
)

func TestBuildDeterministic(t *testing.T) {
	parts := map[string][]byte{
		"b.xml": []byte("<b/>"),
		"a.xml": []byte("<a/>"),
	}
	x, err := Build(parts)
	if err != nil {
		t.Fatal(err)
	}
	y, err := Build(parts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x, y) {
		t.Fatal("same parts must give identical zip")
	}
}

func TestTransplantKeepsGolden(t *testing.T) {
	golden, err := Build(map[string][]byte{
		"[Content_Types].xml": []byte("GOLDEN_CT"),
		"theme/theme1.xml":    []byte("GOLDEN_THEME"),
		"word/document.xml":   []byte("OLD"),
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Transplant(golden, map[string][]byte{"word/document.xml": []byte("NEW")})
	if err != nil {
		t.Fatal(err)
	}
	// read back via UnzipMap and compare
	mp, err := UnzipMap(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(mp["word/document.xml"]) != "NEW" {
		t.Fatal("document not replaced")
	}
	if string(mp["theme/theme1.xml"]) != "GOLDEN_THEME" {
		t.Fatal("theme must pass through")
	}
	if string(mp["[Content_Types].xml"]) != "GOLDEN_CT" {
		t.Fatal("content types must pass through")
	}
}

func TestSafeReplaceKeepsTheme(t *testing.T) {
	parts := map[string][]byte{
		"[Content_Types].xml": []byte("x"),
		"word/document.xml":   []byte("x"),
		"word/styles.xml":     []byte("x"),
		"docProps/core.xml":   []byte("x"),
	}
	safe := SafeReplace("docx", parts)
	if _, ok := safe["word/document.xml"]; !ok {
		t.Fatal("document must be transplantable")
	}
	if _, ok := safe["word/styles.xml"]; ok {
		t.Fatal("styles must never transplant")
	}
	if _, ok := safe["[Content_Types].xml"]; ok {
		t.Fatal("content types must never transplant")
	}
}
