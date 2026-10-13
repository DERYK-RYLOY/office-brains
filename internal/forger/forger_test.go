package forger

import (
	"strings"
	"testing"
)

func TestFrozenDeterministicIDs(t *testing.T) {
	old := Frozen
	Frozen = true
	defer func() { Frozen = old }()
	ResetCounter()
	a := ParaID()
	b := ParaID()
	if a == b {
		t.Fatal("ids must be unique")
	}
	if strings.HasPrefix(a, "8") || strings.HasPrefix(a, "9") {
		t.Fatalf("high bit set: %s", a)
	}
	ResetCounter()
	if c := ParaID(); c != a {
		t.Fatalf("counter reset must replay: %s vs %s", c, a)
	}
	ResetCounter()
}

func TestFrozenClock(t *testing.T) {
	p := Props{Profile: M365, Frozen: true}
	p.Defaults("Microsoft Office Word")
	got := string(CoreXML(p))
	if !strings.Contains(got, "2020-01-01") {
		t.Fatalf("frozen clock missing:\n%s", got)
	}
}

func TestCorporateCompany(t *testing.T) {
	p := Props{Profile: Corporate}
	p.Defaults("Microsoft Office Word")
	if !strings.Contains(string(AppXML(p, 47)), "Contoso") {
		t.Fatal("corporate must inject Contoso")
	}
}
