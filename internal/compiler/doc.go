// Package compiler turns OIR into ordered OOXML parts.
// Phase 1 target. Bootstrap: contract only.
package compiler

// Emit order matters: w:tblPr children must be
// tblStyle -> tblW -> tblLayout -> tblBorders -> tblCellMar
// or Word shows "unreadable content".
