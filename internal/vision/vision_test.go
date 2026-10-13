package vision

import "testing"

func TestCleanBill(t *testing.T) {
	specJSON := `{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"hi"}]}]}}`
	spec := mustParse(t, specJSON)
	iss := Issues(spec, "docx")
	if len(iss) != 1 || iss[0].Rule != "layout.clean" {
		t.Fatalf("%+v", iss)
	}
}

func TestPptxSlideFull(t *testing.T) {
	specJSON := `{"oir":"1.1","doc":{"sections":[{"blocks":[` +
		`{"t":"table","rows":[[1],[2],[3],[4],[5],[6],[7],[8]]},` +
		`{"t":"chart","kind":"bar","labels":["a"],"series":[{"values":[1]}]},` +
		`{"t":"chart","kind":"pie","labels":["a"],"series":[{"values":[1]}]},` +
		`{"t":"image","img":"/x.png"}]}]}}`
	spec := mustParse(t, specJSON)
	iss := Issues(spec, "pptx")
	found := false
	for _, is := range iss {
		if is.Rule == "pptx.slide_full" && is.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want pptx.slide_full error, got %+v", iss)
	}
}

func TestXlsxWideCol(t *testing.T) {
	long := ""
	for i := 0; i < 60; i++ {
		long += "W"
	}
	specJSON := `{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"table","rows":[["` + long + `"]]}]}]}}`
	spec := mustParse(t, specJSON)
	iss := Issues(spec, "xlsx")
	found := false
	for _, is := range iss {
		if is.Rule == "xlsx.wide_col" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want xlsx.wide_col, got %+v", iss)
	}
}
