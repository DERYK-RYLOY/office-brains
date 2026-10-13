package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DERYK-RYLOY/office-brains/internal/compiler"
	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
	"github.com/DERYK-RYLOY/office-brains/internal/surgeon"
)

func writeBuilt(t *testing.T, parts map[string][]byte) string {
	t.Helper()
	blob, err := surgeon.Build(parts)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(p, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustSpec(t *testing.T) *oir.Spec {
	t.Helper()
	s, err := oir.ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"hi"}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyDocxOK(t *testing.T) {
	p := writeBuilt(t, compiler.BuildDocx(mustSpec(t), forger.Props{}))
	res, ok := File(p)
	if !ok {
		t.Fatalf("want ok: %+v", res)
	}
}

func TestVerifyXlsxOK(t *testing.T) {
	p := writeBuilt(t, compiler.BuildXlsx(mustSpec(t), forger.Props{}))
	res, ok := File(p)
	if !ok {
		t.Fatalf("want ok: %+v", res)
	}
}

func TestVerifyPptxOK(t *testing.T) {
	p := writeBuilt(t, compiler.BuildPptx(mustSpec(t), forger.Props{}))
	res, ok := File(p)
	if !ok {
		t.Fatalf("want ok: %+v", res)
	}
}

func TestVerifyDetectsStrict(t *testing.T) {
	p := writeBuilt(t, map[string][]byte{
		"[Content_Types].xml": []byte("<Types/>"),
		"word/document.xml":   []byte(`<w:document xmlns:w="http://purl.oclc.org/ooxml/officeDocument/ml"/>`),
	})
	_, ok := File(p)
	if ok {
		t.Fatal("strict namespace must fail")
	}
}

func TestXSDSkippedWithoutDir(t *testing.T) {
	p := writeBuilt(t, compiler.BuildDocx(mustSpec(t), forger.Props{}))
	_, _, skipped := XSD(p, "")
	if skipped == "" {
		t.Fatal("want skip reason")
	}
}
