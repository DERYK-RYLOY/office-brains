package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/DERYK-RYLOY/office-brains/internal/dna"
)

func cmdDNA(args []string) {
	if len(args) < 1 {
		emitErr("ERR_DNA_ARGS", nil)
		logHuman("usage: brains dna learn <dir> -o dna.json")
		os.Exit(2)
	}
	switch args[0] {
	case "learn":
		fs := flag.NewFlagSet("dna learn", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		out := fs.String("o", "", "output dna.json")
		human := fs.Bool("human", false, "human output")
		// flags may come before or after dir; extract manually for robustness
		dir := ""
		rest := []string{}
		for i, a := range args[1:] {
			if a == "-o" && i+1 < len(args[1:]) {
				*out = args[1:][i+1]
			}
			if a == "--human" || a == "--human=true" {
				*human = true
			}
			if !startsDash(a) && a != *out {
				// first non-flag that isn't the -o value is the dir (last wins)
				dir = a
			}
			_ = rest
		}
		_ = fs.Parse([]string{}) // keep flag pkg happy; manual parse above
		if dir == "" || *out == "" {
			emitErr("ERR_DNA_ARGS", nil)
			logHuman("usage: brains dna learn <dir> -o dna.json")
			os.Exit(2)
		}
		absDir, _ := filepath.Abs(dir)
		prof, err := dna.Learn(absDir)
		if err != nil {
			emitErr("ERR_DNA_LEARN", err)
			os.Exit(1)
		}
		absOut, _ := filepath.Abs(*out)
		b, _ := json.MarshalIndent(prof, "", "  ")
		if err := os.WriteFile(absOut, b, 0o644); err != nil {
			emitErr("ERR_DNA_WRITE", err)
			os.Exit(1)
		}
		if *human {
			printHumanOK("dna learn", map[string]string{"out": absOut, "company": prof.Company, "author": prof.Author, "files": itoaLocal(prof.Files)})
			return
		}
		emitJSON(map[string]any{"ok": true, "command": "dna learn", "data": prof})
	default:
		emitErr("ERR_DNA_CMD", nil)
		logHuman("usage: brains dna learn <dir> -o dna.json")
		os.Exit(2)
	}
}

func startsDash(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func itoaLocal(n int) string {	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
