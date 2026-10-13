package vision

import (
	"testing"

	"github.com/you/office-brains/internal/oir"
)

func mustParse(t *testing.T, s string) *oir.Spec {
	t.Helper()
	spec, err := oir.ParseAndValidate([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}
