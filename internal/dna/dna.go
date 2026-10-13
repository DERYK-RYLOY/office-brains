// Package dna learns corporate lineage from a corpus of Office files.
package dna

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Profile is the learned fingerprint.
type Profile struct {
	DNA       string `json:"dna"`
	Company   string `json:"company,omitempty"`
	Author    string `json:"author,omitempty"`
	Files     int    `json:"files"`
	LearnedAt string `json:"learned_at"`
	Dir       string `json:"dir"`
}

// Learn scans dir for docx/xlsx/pptx and extracts most common company/author.
func Learn(dir string) (*Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	companies := map[string]int{}
	authors := map[string]int{}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		low := strings.ToLower(e.Name())
		if !(strings.HasSuffix(low, ".docx") || strings.HasSuffix(low, ".xlsx") || strings.HasSuffix(low, ".pptx")) {
			continue
		}
		c, a := readProps(filepath.Join(dir, e.Name()))
		if c != "" {
			companies[c]++
		}
		if a != "" {
			authors[a]++
		}
		n++
		if n >= 200 {
			break
		}
	}
	if n == 0 {
		return &Profile{DNA: "1", Files: 0, LearnedAt: time.Now().UTC().Format(time.RFC3339), Dir: dir}, nil
	}
	return &Profile{
		DNA:       "1",
		Company:   top(companies),
		Author:    top(authors),
		Files:     n,
		LearnedAt: time.Now().UTC().Format(time.RFC3339),
		Dir:       dir,
	}, nil
}

func top(m map[string]int) string {
	best, bn := "", 0
	for k, v := range m {
		if v > bn {
			best, bn = k, v
		}
	}
	return best
}

func readProps(path string) (company, author string) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", ""
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "docProps/app.xml" && f.Name != "docProps/core.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(rc, 65536))
		rc.Close()
		s := string(b)
		if f.Name == "docProps/app.xml" {
			company = tag(s, "Company")
		} else {
			author = tag(s, "dc:creator")
			if author == "" {
				author = tag(s, "cp:lastModifiedBy")
			}
		}
	}
	return company, author
}

func tag(s, t string) string {
	o, c := "<"+t+">", "</"+t+">"
	i := strings.Index(s, o)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], c)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(s[i+len(o) : i+j])
}

// Load reads dna.json.
func Load(path string) (*Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
