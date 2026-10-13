// Command brains is the OFFICE BRAINS CLI.
// Design: stdout = data JSON only, stderr = human logs.
// No external deps (stdlib only) so `go build ./...` works offline.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/you/office-brains/internal/compiler"
	"github.com/you/office-brains/internal/dna"
	"github.com/you/office-brains/internal/forger"
	"github.com/you/office-brains/internal/oir"
	"github.com/you/office-brains/internal/surgeon"
	"github.com/you/office-brains/internal/verify"
)

const version = "v0.1.0"

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// emitErr writes structured error JSON to stderr with stable code.
// Emits {"ok":false,"error":CODE,"detail":msg} so agents can switch on error.
func emitErr(msg string, err error) {
	code := msg
	if code == "" {
		code = "ERR_UNKNOWN"
	}
	// if caller passed a CodedError as err and msg is generic, prefer its code
	if err != nil {
		if c := oir.CodeOf(err); c != "ERR_UNKNOWN" {
			code = c
		}
	}
	m := map[string]any{"ok": false, "error": code}
	detail := ""
	if err != nil {
		detail = err.Error()
	} else if msg != code {
		detail = msg
	}
	if detail != "" {
		m["detail"] = detail
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(m)
}

// colorOK reports whether color output is allowed (NO_COLOR / --no-color).
func colorOK(noColorFlag bool) bool {
	if noColorFlag {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func logHuman(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "create":
		cmdCreate(args)
	case "inspect":
		cmdInspect(args)
	case "validate":
		cmdValidate(args)
	case "forge":
		cmdForge(args)
	case "diff":
		cmdDiff(args)
	case "oracle":
		cmdOracle(args)
	case "dna":
		cmdDNA(args)
	case "patch":
		cmdPatch(args)
	case "preview":
		cmdPreview(args)
	case "shot":
		cmdShot(args)
	case "see":
		cmdSee(args)
	case "serve":
		cmdServe(args)
	case "doctor":
		cmdDoctor(args)
	case "version", "--version", "-v":
		emitJSON(map[string]any{"ok": true, "data": map[string]any{"version": version}})
	default:
		emitErr("unknown command: "+cmd, nil)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	logHuman(`brains %s - Office Brains (JSON default, --human for people)
Usage:
  brains create --spec <file|-> --type docx|xlsx|pptx -o <out> [--profile m365] [--dna dna.json] [--golden dir] [--frozen] [--validate] [--human] [--no-color]
  brains patch --spec base.json --patch ops.json -o patched.json [--type docx --out-file out.docx --validate] [--human]
  brains inspect --file <f> [--human] [--no-color]
  brains validate --file <f> [--human]
  brains forge --file <in> -o <out> --profile m365|corporate|clean [--human]
  brains diff --a <f> --b <f> [--human]
  brains oracle setup|check [--file F]   (strict 3rd-party validation)
  brains preview --spec <j> --type docx -o <html>
  brains shot --html <f> -o <png> | see --spec <j> --type docx -o <png>  (VLM eyes)
  brains dna learn <dir> -o dna.json
  brains serve --port 3000
  brains doctor [--human]
Errors: stable codes ERR_OIR_*, ERR_PATCH_*, ERR_VALIDATE_FAILED to stderr as {"ok":false,"error":CODE}.
NO_COLOR=1 respected (no color codes emitted ever).`, version)
}

// --- create: OIR -> real Transitional OOXML via compiler+surgeon+forger ---
func cmdCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "", "OIR spec JSON file (or - for stdin)")
	docType := fs.String("type", "docx", "docx|xlsx|pptx")
	out := fs.String("o", "", "output path (absolute preferred)")
	profile := fs.String("profile", "m365", "m365|corporate|clean")
	company := fs.String("company", "", "company for corporate profile")
	dnaPath := fs.String("dna", "", "dna.json from dna learn (fills company/author)")
	goldenDir := fs.String("golden", "", "dir with blank.docx/xlsx/pptx for transplant (fallback: clean build)")
	doValidate := fs.Bool("validate", false, "run verify after build, include checks, fail non-zero if bad")
	frozen := fs.Bool("frozen", false, "byte-deterministic output (fixed clock + counter IDs)")
	human := fs.Bool("human", false, "human output to stdout instead of JSON")
	noColor := fs.Bool("no-color", false, "disable color (also respects NO_COLOR=1)")
	asJSON := fs.Bool("json", true, "emit JSON to stdout")
	_ = asJSON
	_ = colorOK(*noColor)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *specPath == "" || *out == "" {
		emitErr("ERR_CREATE_ARGS", nil)
		logHuman("create requires --spec and -o")
		os.Exit(2)
	}
	var data []byte
	var err error
	if *specPath == "-" {
		// read stdin fully (bounded 10MB)
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 4096)
		total := 0
		for {
			n, rerr := os.Stdin.Read(tmp)
			if n > 0 {
				total += n
				if total > 10<<20 {
					emitErr("spec too large (>10MB)", nil)
					os.Exit(2)
				}
				buf = append(buf, tmp[:n]...)
			}
			if rerr != nil {
				break
			}
		}
		data = buf
	} else {
		data, err = os.ReadFile(*specPath)
		if err != nil {
			emitErr("read spec failed", err)
			os.Exit(2)
		}
	}
	spec, verr := oir.ParseAndValidate(data)
	if verr != nil {
		emitErr("ERR_OIR_INVALID", verr)
		os.Exit(2)
	}
	absOut, _ := filepath.Abs(*out)
	if *frozen {
		forger.Frozen = true
	}
	meta := forger.Props{Profile: forger.Profile(*profile), Company: *company, Frozen: *frozen}
	if *dnaPath != "" {
		if dp, derr := dna.Load(*dnaPath); derr == nil {
			if meta.Company == "" {
				meta.Company = dp.Company
			}
			if meta.Author == "" {
				meta.Author = dp.Author
			}
		} else {
			emitErr("ERR_DNA_LOAD", derr)
			os.Exit(2)
		}
	}
	if spec.Meta != nil && spec.Meta.Profile != "" {
		meta.Profile = forger.Profile(spec.Meta.Profile)
	}
	if spec.Meta != nil && spec.Meta.Company != "" && meta.Company == "" {
		meta.Company = spec.Meta.Company
	}
	if spec.Meta != nil && spec.Meta.Author != "" {
		meta.Author = spec.Meta.Author
	}

	var parts map[string][]byte
	switch *docType {
	case "docx":
		parts = compiler.BuildDocx(spec, meta)
	case "xlsx":
		parts = compiler.BuildXlsx(spec, meta)
	case "pptx":
		parts = compiler.BuildPptx(spec, meta)
	default:
		emitErr("ERR_CREATE_TYPE", nil)
		os.Exit(2)
	}
	blob, err := surgeon.Build(parts)
	transplanted := false
	transplantNote := ""
	if *goldenDir != "" {
		if gb, note, ok := tryTransplant(*goldenDir, *docType, spec, parts); ok {
			blob, transplanted, transplantNote = gb, true, note
		} else {
			transplantNote = note // clean fallback, reason logged
			logHuman("golden skipped: %s (clean build)", note)
		}
	}
	if err != nil {
		emitErr("ERR_CREATE_ZIP", err)
		os.Exit(2)
	}
	if err := os.WriteFile(absOut, blob, 0o644); err != nil {
		emitErr("ERR_CREATE_WRITE", err)
		os.Exit(2)
	}
	logHuman("wrote %s (%d bytes, %d parts)", absOut, len(blob), len(parts))
	if transplanted {
		logHuman("transplant: golden clone (%s)", transplantNote)
	}
	warns := compiler.Truncated
	for _, w := range warns {
		logHuman("WARN: %s", w)
	}
	if *doValidate {
		results, ok := verify.File(absOut)
		if !ok {
			emitErr("ERR_VALIDATE_FAILED", nil)
			emitJSON(map[string]any{"ok": false, "command": "create", "data": map[string]any{
				"out": absOut, "type": *docType, "checks": results,
			}})
			os.Exit(1)
		}
		if *human {
			printHumanOK("create", map[string]string{"out": absOut, "type": *docType, "validate": "pass"})
			return
		}
		emitJSON(map[string]any{
			"ok": true, "command": "create",
			"data": map[string]any{
				"spec_valid": true, "oir": spec.OIR, "blocks": spec.CountBlocks(),
				"type": *docType, "out": absOut, "written": true, "bytes": len(blob),
				"parts": surgeon.List(parts), "profile": string(meta.Profile), "checks": results,
				"transplanted": transplanted, "warnings": warns,
			},
		})
		return
	}
	if *human {
		printHumanOK("create", map[string]string{"out": absOut, "type": *docType, "bytes": fmt.Sprint(len(blob))})
		return
	}
	emitJSON(map[string]any{
		"ok":      true,
		"command": "create",
		"data": map[string]any{
			"spec_valid": true,
			"oir":        spec.OIR,
			"blocks":     spec.CountBlocks(),
			"type":       *docType,
			"out":        absOut,
			"written":    true,
			"bytes":      len(blob),
			"parts":      surgeon.List(parts),
			"profile":    string(meta.Profile),
			"transplanted": transplanted,
			"warnings":   warns,
		},
	})
}

func cmdInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	f := fs.String("file", "", "file to inspect")
	visual := fs.Bool("visual", false, "request visual preview (requires LibreOffice, not run yet)")
	_ = visual
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *f == "" {
		emitErr("inspect requires --file", nil)
		os.Exit(2)
	}
	abs, _ := filepath.Abs(*f)
	st, err := os.Stat(abs)
	if err != nil {
		emitErr("stat failed", err)
		os.Exit(2)
	}
	parts, prov := inspectZip(abs)
	emitJSON(map[string]any{"ok": true, "command": "inspect", "data": map[string]any{
		"file": abs, "size": st.Size(), "visual": false,
		"parts": parts, "provenance_hits": prov,
	}})
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	f := fs.String("file", "", "file to validate")
	human := fs.Bool("human", false, "human output instead of JSON")
	xsdDir := fs.String("xsd", "", "optional ECMA-376 XSD dir (wml.xsd/sml.xsd/pml.xsd + xmllint)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *f == "" {
		emitErr("ERR_VALIDATE_ARGS", nil)
		os.Exit(2)
	}
	abs, _ := filepath.Abs(*f)
	if _, err := os.Stat(abs); err != nil {
		emitErr("ERR_VALIDATE_STAT", err)
		os.Exit(2)
	}
	results, ok := verify.File(abs)
	xsdResults, xsdOK, xsdSkipped := verify.XSD(abs, *xsdDir)
	if *xsdDir != "" {
		if xsdSkipped != "" {
			logHuman("xsd gate skipped: %s", xsdSkipped)
		} else {
			results = append(results, xsdResults...)
			ok = ok && xsdOK
		}
	}
	if !ok {
		emitErr("ERR_VALIDATE_FAILED", nil)
		// still emit data to stdout for agent parsing, then non-zero exit
		emitJSON(map[string]any{"ok": false, "command": "validate", "data": map[string]any{"file": abs, "checks": results}})
		os.Exit(1)
	}
	if *human {
		pass := 0
		for _, r := range results {
			if r.OK {
				pass++
			}
		}
		printHumanOK("validate", map[string]string{"file": abs, "checks": fmt.Sprintf("%d/%d pass", pass, len(results))})
		return
	}
	emitJSON(map[string]any{"ok": true, "command": "validate", "data": map[string]any{
		"file": abs, "checks": results,
	}})
}

func cmdForge(args []string) {
	fs := flag.NewFlagSet("forge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	f := fs.String("file", "", "file to forge")
	profile := fs.String("profile", "m365", "m365|corporate|clean")
	company := fs.String("company", "", "company for corporate profile")
	author := fs.String("author", "", "author override (default User)")
	out := fs.String("o", "", "output path (default: overwrite with .forged suffix guard - requires -o)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *f == "" || *out == "" {
		emitErr("forge requires --file and -o (never overwrites in place)", nil)
		os.Exit(2)
	}
	abs, _ := filepath.Abs(*f)
	absOut, _ := filepath.Abs(*out)
	if abs == absOut {
		emitErr("refusing to overwrite input (choose different -o)", nil)
		os.Exit(2)
	}
	dtype, err := forger.ForgeFile(abs, absOut, forger.Profile(*profile), *company, *author)
	if err != nil {
		emitErr("forge failed", err)
		os.Exit(1)
	}
	logHuman("forged %s -> %s [%s profile=%s]", abs, absOut, dtype, *profile)
	emitJSON(map[string]any{"ok": true, "command": "forge", "data": map[string]any{
		"in": abs, "out": absOut, "type": dtype, "profile": *profile, "forged": true,
	}})
}

func cmdDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	human := fs.Bool("human", false, "human output instead of JSON")
	_ = fs.Parse(args)
	rep := doctorReport()
	if *human {
		if data, ok := rep["data"].(map[string]any); ok {
			printHumanOK("doctor", map[string]string{"version": fmt.Sprint(rep["version"]), "checks": fmt.Sprint(len(fmt.Sprint(data["checks"])))})
		}
		// human-readable check list to stderr for people
		if data, ok := rep["data"].(map[string]any); ok {
			logHuman("doctor %v", data["checks"])
		}
		return
	}
	emitJSON(rep)
}
