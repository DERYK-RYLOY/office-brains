package oir

import (
	"strings"
	"testing"
)

func TestValidSpec(t *testing.T) {
	s, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"name":"A","blocks":[{"t":"h1","txt":"Hi"},{"t":"table","rows":[["a","b"],[1,2]]}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.CountBlocks() != 2 {
		t.Fatalf("blocks=%d", s.CountBlocks())
	}
}

func TestOldVersionAccepted(t *testing.T) {
	if _, err := ParseAndValidate([]byte(`{"oir":"1.0","doc":{"sections":[{"blocks":[{"t":"p","txt":"x"}]}]}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestRaggedTableCode(t *testing.T) {
	_, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"table","rows":[["a"],["a","b"]]}]}]}}`))
	if err == nil || CodeOf(err) != "ERR_OIR_RAGGED_TABLE" {
		t.Fatalf("want ERR_OIR_RAGGED_TABLE, got %v", err)
	}
}

func TestUnknownBlockCode(t *testing.T) {
	_, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"dance"}]}]}}`))
	if err == nil || CodeOf(err) != "ERR_OIR_UNKNOWN_BLOCK" {
		t.Fatalf("want ERR_OIR_UNKNOWN_BLOCK, got %v", err)
	}
}

func TestChartKindCode(t *testing.T) {
	_, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"chart","kind":"3d","labels":["a"],"series":[{"values":[1]}]}]}]}}`))
	if err == nil || CodeOf(err) != "ERR_OIR_CHART_KIND" {
		t.Fatalf("want ERR_OIR_CHART_KIND, got %v", err)
	}
}

func TestDupID(t *testing.T) {
	_, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"a","id":"x"},{"t":"p","txt":"b","id":"x"}]}]}}`))
	if err == nil || CodeOf(err) != "ERR_OIR_DUP_ID" {
		t.Fatalf("want ERR_OIR_DUP_ID, got %v", err)
	}
}

func TestNeedsClean(t *testing.T) {
	s, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"image","img":"/x.png"}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if need, _ := s.NeedsClean(); !need {
		t.Fatal("image spec must need clean")
	}
	s2, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"x"}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if need, _ := s2.NeedsClean(); need {
		t.Fatal("plain spec must be transplant-safe")
	}
}

func TestBadMerge(t *testing.T) {
	_, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"table","merge":["nope"],"rows":[["a"],["b"]]}]}]}}`))
	if err == nil || CodeOf(err) != "ERR_OIR_BAD_MERGE" {
		t.Fatalf("want ERR_OIR_BAD_MERGE, got %v", err)
	}
}

func TestTxtCap(t *testing.T) {
	big := strings.Repeat("x", 100001)
	_, err := ParseAndValidate([]byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"` + big + `"}]}]}}`))
	if err == nil || CodeOf(err) != "ERR_OIR_TXT_TOO_BIG" {
		t.Fatalf("want ERR_OIR_TXT_TOO_BIG, got %v", err)
	}
}
