package dna

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DERYK-RYLOY/office-brains/internal/compiler"
	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
	"github.com/DERYK-RYLOY/office-brains/internal/surgeon"
)

func TestLearnCompany(t *testing.T) {
	dir := t.TempDir()
	s, err := oir.ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"x"}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	parts := compiler.BuildDocx(s, forger.Props{Profile: forger.Corporate, Company: "Acme"})
	blob, err := surgeon.Build(parts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.docx"), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	prof, err := Learn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if prof.Files != 1 || prof.Company != "Acme" {
		t.Fatalf("%+v", prof)
	}
}

func TestLearnEmpty(t *testing.T) {
	prof, err := Learn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if prof.Files != 0 {
		t.Fatalf("%+v", prof)
	}
}
