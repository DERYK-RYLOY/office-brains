// Package surgeon transplants generated XML into golden Office ZIPs.
// Phase 1 target. Bootstrap: no code, only the contract.
package surgeon

// Contract:
//   Transplant(golden []byte, parts map[string][]byte) []byte
// Rules:
//   - copy ZIP byte-identical, replace ONLY word/document.xml,
//     xl/worksheets/*, xl/sharedStrings.xml, docProps/*
//   - never regenerate [Content_Types].xml, _rels/*, theme/*, styles.xml
//   - sort entries, Deflate, timestamps 1980-01-01 for determinism
