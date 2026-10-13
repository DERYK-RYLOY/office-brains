// Forge mutation: replace docProps in an existing Office file, preserve everything else.
package forger

import (
	"archive/zip"
	"bytes"
	"io"
	"sort"
	"strings"
	"time"
)

// DetectType sniffs docx|xlsx|pptx|unknown from ZIP part names.
func DetectType(partNames []string) string {
	has := func(n string) bool {
		for _, p := range partNames {
			if p == n {
				return true
			}
		}
		return false
	}
	switch {
	case has("word/document.xml"):
		return "docx"
	case has("xl/workbook.xml"):
		return "xlsx"
	case has("ppt/presentation.xml"):
		return "pptx"
	default:
		return "unknown"
	}
}

// ForgeFile reads inPath, replaces docProps/core.xml + docProps/app.xml per profile,
// and writes outPath deterministically. All other parts pass through byte-identical.
// Returns detected type.
func ForgeFile(inPath, outPath string, profile Profile, company, author string) (string, error) {
	zr, err := zip.OpenReader(inPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	type part struct {
		name string
		data []byte
	}
	blobs := map[string][]byte{}
	names := []string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 100<<20))
		rc.Close()
		if err != nil {
			return "", err
		}
		blobs[f.Name] = b
		names = append(names, f.Name)
	}
	dtype := DetectType(names)
	if dtype == "unknown" {
		// still forge docProps if present; otherwise error
		if _, ok := blobs["docProps/core.xml"]; !ok {
			return dtype, errUnknownType
		}
	}
	appName := map[string]string{
		"docx": "Microsoft Office Word",
		"xlsx": "Microsoft Excel",
		"pptx": "Microsoft Office PowerPoint",
	}[dtype]
	if appName == "" {
		appName = "Microsoft Office Word"
	}

	// Preserve Pages/Words if existing app.xml has them; else defaults.
	pages, words := parseAppStats(blobs["docProps/app.xml"])
	p := Props{Profile: profile, Company: company, Author: author, AppName: appName, Pages: pages, Words: words, Frozen: Frozen}
	p.Defaults(appName)
	// Human totalTime per profile
	totalTime := 47
	if dtype == "xlsx" {
		totalTime = 12
	}
	if dtype == "pptx" {
		totalTime = 8
	}
	blobs["docProps/core.xml"] = CoreXML(p)
	blobs["docProps/app.xml"] = AppXML(p, totalTime)

	// Deterministic rebuild, sorted, Deflate, fixed modtime.
	sort.Strings(names)
	// dedupe names just in case
	seen := map[string]bool{}
	ordered := []part{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		ordered = append(ordered, part{name: n, data: blobs[n]})
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fixed := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, pt := range ordered {
		h := &zip.FileHeader{Name: pt.name, Method: zip.Deflate}
		h.SetModTime(fixed)
		h.SetMode(0o644)
		fw, err := w.CreateHeader(h)
		if err != nil {
			_ = w.Close()
			return dtype, err
		}
		if _, err := fw.Write(pt.data); err != nil {
			_ = w.Close()
			return dtype, err
		}
	}
	if err := w.Close(); err != nil {
		return dtype, err
	}
	return dtype, writeFile(outPath, buf.Bytes())
}

func parseAppStats(appXML []byte) (pages, words int) {
	pages, words = 1, 0
	if len(appXML) == 0 {
		return pages, words
	}
	s := string(appXML)
	pages = extractIntTag(s, "Pages", 1)
	words = extractIntTag(s, "Words", 0)
	return pages, words
}

func extractIntTag(s, tag string, def int) int {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	i := strings.Index(s, open)
	if i < 0 {
		return def
	}
	j := strings.Index(s[i:], close)
	if j < 0 {
		return def
	}
	num := strings.TrimSpace(s[i+len(open) : i+j])
	n := 0
	for _, r := range num {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}
