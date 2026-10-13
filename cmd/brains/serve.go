// Serve: localhost-only read-only preview + validate API.
// Safety: binds 127.0.0.1 only, never 0.0.0.0. No file writes, no exec.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/DERYK-RYLOY/office-brains/internal/verify"
)

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.String("port", "3000", "localhost port")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "service": "brains", "version": version})
	})
	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		f := r.URL.Query().Get("file")
		if f == "" {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "need ?file=abs-path"})
			return
		}
		results, ok := verify.File(f)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": ok, "file": f, "checks": results})
	})
	mux.HandleFunc("/inspect", func(w http.ResponseWriter, r *http.Request) {
		f := r.URL.Query().Get("file")
		if f == "" {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "need ?file=abs-path"})
			return
		}
		parts, prov := inspectZip(f)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "file": f, "parts": parts, "provenance_hits": prov})
	})
	addr := "127.0.0.1:" + *port
	fmt.Fprintf(os.Stderr, "brains serve on http://%s (localhost only)\n", addr)
	fmt.Fprintf(os.Stderr, "  /healthz  /validate?file=  /inspect?file=\n")
	if err := http.ListenAndServe(addr, mux); err != nil {
		emitErr("serve failed", err)
		os.Exit(1)
	}
}
