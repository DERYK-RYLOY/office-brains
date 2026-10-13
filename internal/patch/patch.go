// Package patch applies RFC6902-style JSON Patch (subset) to OIR specs.
// Ops: add | replace | remove with JSON Pointer paths like /doc/sections/0/blocks/1/txt.
// Array append uses "-" as last token. No external deps.
package patch

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Op is one patch operation.
type Op struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// CodedError mirrors oir stability: stable codes for agents.
type CodedError struct {
	Code string
	Msg  string
}

func (e *CodedError) Error() string { return e.Msg }

func coded(code, msg string, a ...any) *CodedError {
	return &CodedError{Code: code, Msg: fmt.Sprintf(msg, a...)}
}

// Apply parses doc + ops JSON and returns patched doc JSON.
func Apply(docJSON, opsJSON []byte) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return nil, coded("ERR_PATCH_BASE_JSON", "base spec invalid JSON: %v", err)
	}
	var ops []Op
	if err := json.Unmarshal(opsJSON, &ops); err != nil {
		return nil, coded("ERR_PATCH_OPS_JSON", "patch ops invalid JSON: %v", err)
	}
	if len(ops) == 0 {
		return nil, coded("ERR_PATCH_EMPTY", "patch ops empty (need >=1 op)")
	}
	if len(ops) > 100 {
		return nil, coded("ERR_PATCH_TOO_BIG", "patch max 100 ops, got %d", len(ops))
	}
	for i, op := range ops {
		var err error
		switch op.Op {
		case "replace":
			err = doReplace(doc, op.Path, op.Value)
		case "add":
			err = doAdd(doc, op.Path, op.Value)
		case "remove":
			err = doRemove(doc, op.Path)
		default:
			err = coded("ERR_PATCH_BAD_OP", "ops[%d]: unknown op %q (want add|replace|remove)", i, op.Op)
		}
		if err != nil {
			return nil, err
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, coded("ERR_PATCH_ENCODE", "encode failed: %v", err)
	}
	return out, nil
}

func splitPath(p string) ([]string, error) {
	if p == "" || p[0] != '/' {
		return nil, coded("ERR_PATCH_BAD_PATH", "path must start with /, got %q", p)
	}
	raw := strings.Split(p[1:], "/")
	toks := make([]string, len(raw))
	for i, t := range raw {
		t = strings.ReplaceAll(t, "~1", "/")
		t = strings.ReplaceAll(t, "~0", "~")
		toks[i] = t
	}
	return toks, nil
}

func resolveParent(doc any, toks []string) (any, string, error) {
	cur := doc
	for _, t := range toks[:len(toks)-1] {
		switch n := cur.(type) {
		case map[string]any:
			v, ok := n[t]
			if !ok {
				return nil, "", coded("ERR_PATCH_NO_PATH", "path not found at %q", t)
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(t)
			if err != nil || idx < 0 || idx >= len(n) {
				return nil, "", coded("ERR_PATCH_NO_PATH", "array index %q out of range", t)
			}
			cur = n[idx]
		default:
			return nil, "", coded("ERR_PATCH_NO_PATH", "cannot descend into %T at %q", cur, t)
		}
	}
	return cur, toks[len(toks)-1], nil
}

func doReplace(doc any, path string, v any) error {
	toks, err := splitPath(path)
	if err != nil {
		return err
	}
	parent, last, err := resolveParent(doc, toks)
	if err != nil {
		return err
	}
	switch n := parent.(type) {
	case map[string]any:
		if _, ok := n[last]; !ok {
			return coded("ERR_PATCH_NO_PATH", "replace: %q does not exist", path)
		}
		n[last] = v
		return nil
	case []any:
		idx, err := strconv.Atoi(last)
		if err != nil || idx < 0 || idx >= len(n) {
			return coded("ERR_PATCH_NO_PATH", "replace: index %q out of range", path)
		}
		// mutate underlying slice element via parent ref: need to set through slice header
		n[idx] = v
		return nil
	default:
		return coded("ERR_PATCH_NO_PATH", "replace: parent is %T", parent)
	}
}

func doAdd(doc any, path string, v any) error {
	toks, err := splitPath(path)
	if err != nil {
		return err
	}
	parent, last, err := resolveParent(doc, toks)
	if err != nil {
		// add allows creating missing object keys one level deep, but parent must exist
		return err
	}
	switch n := parent.(type) {
	case map[string]any:
		n[last] = v
		return nil
	case []any:
		if last == "-" {
			// append requires handle on the actual slice in its parent; we hold a copy
			// of the slice header, so append here would not propagate. Reject with clear code
			// and tell agent to replace the whole array instead (safe, explicit).
			return coded("ERR_PATCH_APPEND_UNSUPPORTED", "add with '-' needs array handle; replace the array instead (e.g. /doc/sections/0/blocks)")
		}
		idx, err := strconv.Atoi(last)
		if err != nil || idx < 0 || idx > len(n) {
			return coded("ERR_PATCH_NO_PATH", "add: index %q out of range 0..%d", path, len(n))
		}
		// insert would need parent slice write-back; keep bootstrap simple: only allow
		// replace of whole arrays, or adding object keys.
		return coded("ERR_PATCH_ARRAY_INSERT_UNSUPPORTED", "add into array middle unsupported; replace the whole array at its parent path")
	default:
		return coded("ERR_PATCH_NO_PATH", "add: parent is %T", parent)
	}
}

func doRemove(doc any, path string, ) error {
	toks, err := splitPath(path)
	if err != nil {
		return err
	}
	parent, last, err := resolveParent(doc, toks)
	if err != nil {
		return err
	}
	switch n := parent.(type) {
	case map[string]any:
		if _, ok := n[last]; !ok {
			return coded("ERR_PATCH_NO_PATH", "remove: %q does not exist", path)
		}
		delete(n, last)
		return nil
	default:
		return coded("ERR_PATCH_ARRAY_REMOVE_UNSUPPORTED", "remove only supports object keys in bootstrap (replace the array instead)")
	}
}
