package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/DERYK-RYLOY/office-brains/internal/compiler"
	"github.com/DERYK-RYLOY/office-brains/internal/forger"
	"github.com/DERYK-RYLOY/office-brains/internal/oir"
	"github.com/DERYK-RYLOY/office-brains/internal/patch"
	"github.com/DERYK-RYLOY/office-brains/internal/surgeon"
	"github.com/DERYK-RYLOY/office-brains/internal/verify"
)

// patch: surgical OIR update without full rewrite.
// Ops are RFC6902-style [{op,path,value}]. Array elements: use replace on index.
// Example: {"op":"replace","path":"/doc/sections/0/blocks/1/txt","value":"fixed"}
func cmdPatch(args []string) {
	fs := flag.NewFlagSet("patch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "", "base OIR spec JSON")
	patchPath := fs.String("patch", "", "patch ops JSON file (array of {op,path,value})")
	outSpec := fs.String("o", "", "output patched spec JSON path")
	buildType := fs.String("type", "", "optional: docx|xlsx|pptx to also build")
	outFile := fs.String("out-file", "", "optional: output Office file path (needs --type)")
	doValidate := fs.Bool("validate", false, "validate built file (needs --type)")
	frozen := fs.Bool("frozen", false, "byte-deterministic output")
	human := fs.Bool("human", false, "human output to stdout instead of JSON")
	noColor := fs.Bool("no-color", false, "disable color (also respects NO_COLOR=1)")
	_ = noColor
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	_ = colorOK(*noColor)
	if *specPath == "" || *patchPath == "" || *outSpec == "" {
		emitErr("ERR_PATCH_ARGS", nil)
		logHuman("patch requires --spec base.json --patch ops.json -o patched.json")
		os.Exit(2)
	}
	base, err := os.ReadFile(*specPath)
	if err != nil {
		emitErr("ERR_PATCH_READ_BASE", err)
		os.Exit(1)
	}
	ops, err := os.ReadFile(*patchPath)
	if err != nil {
		emitErr("ERR_PATCH_READ_OPS", err)
		os.Exit(1)
	}
	patched, err := patch.Apply(base, ops)
	if err != nil {
		emitErr("ERR_PATCH_APPLY", err)
		os.Exit(1)
	}
	spec, verr := oir.ParseAndValidate(patched)
	if verr != nil {
		emitErr("ERR_PATCH_INVALID_RESULT", verr)
		os.Exit(1)
	}
	absOut, _ := filepath.Abs(*outSpec)
	if err := os.WriteFile(absOut, patched, 0o644); err != nil {
		emitErr("ERR_PATCH_WRITE", err)
		os.Exit(1)
	}
	// optional build
	builtOut := ""
	var checks any
	if *buildType != "" {
		if *outFile == "" {
			emitErr("ERR_PATCH_BUILD_ARGS", nil)
			logHuman("patch build needs --out-file with --type")
			os.Exit(2)
		}
		meta := forger.Props{Profile: forger.M365, Frozen: *frozen}
		if *frozen {
			forger.Frozen = true
		}
		var parts map[string][]byte
		switch *buildType {
		case "docx":
			parts = compiler.BuildDocx(spec, meta)
		case "xlsx":
			parts = compiler.BuildXlsx(spec, meta)
		case "pptx":
			parts = compiler.BuildPptx(spec, meta)
		default:
			emitErr("ERR_PATCH_BUILD_TYPE", nil)
			os.Exit(2)
		}
		blob, err := surgeon.Build(parts)
		if err != nil {
			emitErr("ERR_PATCH_ZIP", err)
			os.Exit(1)
		}
		absB, _ := filepath.Abs(*outFile)
		if err := os.WriteFile(absB, blob, 0o644); err != nil {
			emitErr("ERR_PATCH_WRITE_FILE", err)
			os.Exit(1)
		}
		builtOut = absB
		if *doValidate {
			results, ok := verify.File(absB)
			checks = results
			if !ok {
				emitErr("ERR_VALIDATE_FAILED", nil)
				emitJSON(map[string]any{"ok": false, "command": "patch", "data": map[string]any{
					"spec": absOut, "blocks": spec.CountBlocks(), "built": builtOut, "checks": results,
				}})
				os.Exit(1)
			}
		}
	}
	if *human {
		logHuman("patched %s blocks=%d -> %s", *specPath, spec.CountBlocks(), absOut)
		if builtOut != "" {
			logHuman("built %s [%s]", builtOut, *buildType)
		}
		printHumanOK("patch", map[string]string{"spec": absOut, "built": builtOut})
		return
	}
	data := map[string]any{"spec": absOut, "blocks": spec.CountBlocks()}
	if builtOut != "" {
		data["built"] = builtOut
		data["type"] = *buildType
	}
	if checks != nil {
		data["checks"] = checks
	}
	emitJSON(map[string]any{"ok": true, "command": "patch", "data": data})
}
