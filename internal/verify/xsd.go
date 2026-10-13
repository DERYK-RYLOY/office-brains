// XSD gate: optional xmllint validation against ECMA-376 schemas.
// Skips cleanly when xmllint or schemas are absent (no hard dep).
package verify

import (
	"archive/zip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// XSD validates parts against schemas in xsdDir.
// Returns results + ok + skipped reason ("" when actually ran).
func XSD(path, xsdDir string) (results []Result, ok bool, skipped string) {
	if xsdDir == "" {
		return nil, true, "no --xsd dir supplied"
	}
	if _, err := os.Stat(xsdDir); err != nil {
		return nil, true, "xsd dir missing: " + xsdDir
	}
	if _, err := exec.LookPath("xmllint"); err != nil {
		return nil, true, "xmllint not found - XSD gate skipped"
	}
	schemas := map[string]string{} // part suffix -> schema file
	if des, err := os.ReadDir(xsdDir); err == nil {
		for _, e := range des {
			low := strings.ToLower(e.Name())
			schemas[low] = filepath.Join(xsdDir, e.Name())
		}
	}
	pick := func(part string) string {
		low := strings.ToLower(part)
		var want []string
		switch {
		case strings.HasPrefix(low, "word/"):
			want = []string{"wml.xsd"}
		case strings.HasPrefix(low, "xl/"):
			want = []string{"sml.xsd"}
		case strings.HasPrefix(low, "ppt/"):
			want = []string{"pml.xsd"}
		default:
			return ""
		}
		for _, w := range want {
			if p, ok := schemas[w]; ok {
				return p
			}
		}
		return ""
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return []Result{{Name: "xsd_open", OK: false, Info: err.Error()}}, false, ""
	}
	defer zr.Close()
	ran := 0
	for _, f := range zr.File {
		if !(strings.HasSuffix(f.Name, ".xml")) {
			continue
		}
		// only top-level document parts, not rels
		if strings.HasSuffix(f.Name, ".rels") {
			continue
		}
		schema := pick(f.Name)
		if schema == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		tmp, err := os.CreateTemp("", "brains-xsd-*.xml")
		if err != nil {
			rc.Close()
			continue
		}
		buf := make([]byte, 0, 65536)
		tb := make([]byte, 8192)
		for {
			n, rerr := rc.Read(tb)
			if n > 0 {
				buf = append(buf, tb[:n]...)
			}
			if rerr != nil {
				break
			}
		}
		rc.Close()
		_, _ = tmp.Write(buf)
		tmp.Close()
		cmd := exec.Command("xmllint", "--noout", "--schema", schema, tmp.Name())
		out, err := cmd.CombinedOutput()
		_ = os.Remove(tmp.Name())
		ran++
		name := "xsd:" + f.Name
		if err == nil {
			results = append(results, Result{Name: name, OK: true, Info: "valid vs " + filepath.Base(schema)})
		} else {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 300 {
				msg = msg[:300]
			}
			results = append(results, Result{Name: name, OK: false, Info: fmt.Sprintf("%s: %s", filepath.Base(schema), msg)})
		}
	}
	if ran == 0 {
		return nil, true, "no matching schemas (want wml.xsd/sml.xsd/pml.xsd in dir)"
	}
	ok = true
	for _, r := range results {
		if !r.OK {
			ok = false
		}
	}
	return results, ok, ""
}
