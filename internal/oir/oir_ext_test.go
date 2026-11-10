package oir

import "testing"

func TestBOMTolerated(t *testing.T) {
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"h1","txt":"Hi"}]}]}}`)...)
	s, err := ParseAndValidate(withBOM)
	if err != nil {
		t.Fatalf("BOM should be tolerated, got %v", err)
	}
	if len(s.Doc.Sections) != 1 {
		t.Fatal("sections missing")
	}
	// leading whitespace also tolerated
	withWS := append([]byte("  \r\n\t"), []byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"x"}]}]}}`)...)
	if _, err := ParseAndValidate(withWS); err != nil {
		t.Fatalf("leading whitespace should be tolerated, got %v", err)
	}
}

func TestNewBlockTypes(t *testing.T) {
	spec := `{"oir":"1.1","doc":{"sections":[{"blocks":[
		{"t":"h3","txt":"sub"},
		{"t":"quote","txt":"q"},
		{"t":"divider"},
		{"t":"pagebreak"},
		{"t":"toc"},
		{"t":"bullet","txt":"b"}
	]}]}}`
	s, err := ParseAndValidate([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.CountBlocks(); got != 6 {
		t.Fatalf("blocks=%d", got)
	}
}

func TestQuoteRequiresTxt(t *testing.T) {
	spec := `{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"quote"}]}]}}`
	if _, err := ParseAndValidate([]byte(spec)); err == nil {
		t.Fatal("quote without txt must fail")
	}
}
