// Shot + See: real PNGs for VLM eyes via headless Edge (preinstalled).
// `shot` screenshots any HTML. `see` renders OIR -> HTML -> PNG + math issues
// in ONE call: the agent gets pixels and a clean bill in a single round trip.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/you/office-brains/internal/oir"
	"github.com/you/office-brains/internal/preview"
	"github.com/you/office-brains/internal/vision"
)

func edgeBinary() string {
	cands := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath("msedge"); err == nil {
		return p
	}
	if p, err := exec.LookPath("chromium"); err == nil {
		return p
	}
	return ""
}

func cmdShot(args []string) {
	fs := flag.NewFlagSet("shot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	html := fs.String("html", "", "input HTML file")
	out := fs.String("o", "", "output PNG path")
	width := fs.Int("w", 1280, "viewport width")
	height := fs.Int("h", 1000, "viewport height")
	human := fs.Bool("human", false, "human output instead of JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *html == "" || *out == "" {
		emitErr("ERR_SHOT_ARGS", nil)
		logHuman("usage: brains shot --html preview.html -o out.png [--w 1280 --h 1000]")
		os.Exit(2)
	}
	png, err := shoot(*html, *out, *width, *height)
	if err != nil {
		emitErr("ERR_SHOT_FAILED", err)
		os.Exit(1)
	}
	if *human {
		printHumanOK("shot", map[string]string{"png": png})
		return
	}
	emitJSON(map[string]any{"ok": true, "command": "shot", "data": map[string]any{"png": png}})
}

func cmdSee(args []string) {
	fs := flag.NewFlagSet("see", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "", "OIR spec JSON file")
	docType := fs.String("type", "docx", "docx|xlsx|pptx")
	out := fs.String("o", "", "output PNG path")
	width := fs.Int("w", 1280, "viewport width")
	height := fs.Int("h", 1000, "viewport height")
	human := fs.Bool("human", false, "human output instead of JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *specPath == "" || *out == "" {
		emitErr("ERR_SEE_ARGS", nil)
		logHuman("usage: brains see --spec job.json --type docx -o out.png")
		os.Exit(2)
	}
	data, err := os.ReadFile(*specPath)
	if err != nil {
		emitErr("ERR_SEE_READ", err)
		os.Exit(1)
	}
	spec, verr := oir.ParseAndValidate(data)
	if verr != nil {
		emitErr("ERR_OIR_INVALID", verr)
		os.Exit(2)
	}
	issues := vision.Issues(spec, *docType)
	html := preview.Render(spec, *docType)
	tmp, err := os.CreateTemp("", "brains-see-*.html")
	if err != nil {
		emitErr("ERR_SEE_TMP", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	_, _ = tmp.WriteString(html)
	_ = tmp.Close()
	defer os.Remove(tmpName)
	absOut, _ := filepath.Abs(*out)
	png, err := shoot(tmpName, absOut, *width, *height)
	if err != nil {
		emitErr("ERR_SHOT_FAILED", err)
		os.Exit(1)
	}
	for _, is := range issues {
		logHuman("[%s] %s: %s", is.Severity, is.Rule, is.Detail)
	}
	if *human {
		printHumanOK("see", map[string]string{"png": png, "issues": fmt.Sprint(len(issues))})
		return
	}
	emitJSON(map[string]any{"ok": true, "command": "see", "data": map[string]any{
		"png": png, "type": *docType, "blocks": spec.CountBlocks(), "issues": issues,
	}})
}

// shoot renders HTML to PNG via headless Edge/Chromium. No installs.
func shoot(htmlPath, outPath string, w, h int) (string, error) {
	edge := edgeBinary()
	if edge == "" {
		return "", fmt.Errorf("no Edge/Chromium found (need msedge.exe)")
	}
	absHTML, _ := filepath.Abs(htmlPath)
	absOut, _ := filepath.Abs(outPath)
	url := "file:///" + filepath.ToSlash(absHTML)
	cmd := exec.Command(edge, "--headless", "--disable-gpu", "--no-sandbox", "--hide-scrollbars",
		fmt.Sprintf("--screenshot=%s", absOut),
		fmt.Sprintf("--window-size=%d,%d", w, h), url)
	cmd.WaitDelay = 60 * time.Second
	out, err := cmd.CombinedOutput()
	_ = out // Edge logs noise to stderr; PNG presence is the verdict
	if st, serr := os.Stat(absOut); serr != nil || st.Size() == 0 {
		return "", fmt.Errorf("screenshot missing: %v", err)
	}
	return absOut, nil
}
