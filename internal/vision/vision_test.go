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

func TestPptxVisualDropped(t *testing.T) {
	// an 8-row table leaves no room for images/charts: they genuinely drop
	specJSON := `{"oir":"1.1","doc":{"sections":[{"blocks":[` +
		`{"t":"table","rows":[[1],[2],[3],[4],[5],[6],[7],[8]]},` +
		`{"t":"chart","kind":"bar","labels":["a"],"series":[{"values":[1]}]},` +
		`{"t":"chart","kind":"pie","labels":["a"],"series":[{"values":[1]}]},` +
		`{"t":"image","img":"/x.png"}]}]}}`
	spec := mustParse(t, specJSON)
	iss := Issues(spec, "pptx")
	foundDrop := false
	for _, is := range iss {
		if is.Rule == "pptx.visual_dropped" && is.Severity == "error" {
			foundDrop = true
		}
		if is.Rule == "pptx.visual_scaled" {
			t.Fatalf("nothing should scale here - table eats the slide, got %+v", iss)
		}
	}
	if !foundDrop {
		t.Fatalf("want pptx.visual_dropped error, got %+v", iss)
	}
}

func TestPptxVisualScaled(t *testing.T) {
	// small table + a chart that needs more room than remains: scales to fit
	specJSON := `{"oir":"1.1","doc":{"sections":[{"blocks":[` +
		`{"t":"table","rows":[[1],[2],[3],[4]]},` +
		`{"t":"chart","kind":"bar","labels":["a"],"series":[{"values":[1]}]}]}]}}`
	spec := mustParse(t, specJSON)
	iss := Issues(spec, "pptx")
	foundScaled := false
	for _, is := range iss {
		if is.Rule == "pptx.visual_scaled" && is.Severity == "warn" {
			foundScaled = true
		}
		if is.Rule == "pptx.visual_dropped" {
			t.Fatalf("chart should scale, not drop, got %+v", iss)
		}
	}
	if !foundScaled {
		t.Fatalf("want pptx.visual_scaled warn, got %+v", iss)
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
