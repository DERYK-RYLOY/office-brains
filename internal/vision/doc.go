// Package vision renders previews via LibreOffice for the VLM loop.
// Phase 2 target. Bootstrap: contract only. Never auto-run soffice yet.
package vision

// Preview(docxPath) -> pngPath + issues[] via:
// soffice --headless --convert-to png. Caller (agent) views PNG, patches OIR.
