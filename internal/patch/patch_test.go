package patch

import (
	"encoding/json"
	"testing"
)

func TestReplaceTxt(t *testing.T) {
	base := []byte(`{"oir":"1.1","doc":{"sections":[{"blocks":[{"t":"p","txt":"old"}]}]}}`)
	ops := []byte(`[{"op":"replace","path":"/doc/sections/0/blocks/0/txt","value":"new"}]`)
	out, err := Apply(base, ops)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	got := m["doc"].(map[string]any)["sections"].([]any)[0].(map[string]any)["blocks"].([]any)[0].(map[string]any)["txt"]
	if got != "new" {
		t.Fatalf("got %v", got)
	}
}

func TestBadOp(t *testing.T) {
	_, err := Apply([]byte(`{"a":1}`), []byte(`[{"op":"move","path":"/a"}]`))
	if err == nil {
		t.Fatal("want error")
	}
	if ce, ok := err.(*CodedError); !ok || ce.Code != "ERR_PATCH_BAD_OP" {
		t.Fatalf("want ERR_PATCH_BAD_OP, got %v", err)
	}
}

func TestReplaceMissing(t *testing.T) {
	_, err := Apply([]byte(`{"a":1}`), []byte(`[{"op":"replace","path":"/b","value":2}]`))
	if err == nil {
		t.Fatal("want error")
	}
}

func TestEmptyOps(t *testing.T) {
	_, err := Apply([]byte(`{"a":1}`), []byte(`[]`))
	if err == nil {
		t.Fatal("want error")
	}
}
