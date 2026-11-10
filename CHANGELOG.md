# Changelog

## v0.2.0 — 2026-11-09

### Fixed
- **Oracle works for binary installs**: the Python scripts and requirement
  files are now embedded in the binary and materialized into `.oracle/`
  (or a repo checkout on disk, if present). Root discovery walks upward
  from the cwd and from the executable's directory, and
  `BRAINS_ORACLE_ROOT` overrides it. Previously `brains oracle setup`
  failed with `ERR_ORACLE_SETUP` outside a repo checkout.
- **`oracle check --file a --file b`**: repeated `--file` flags now
  accumulate instead of silently keeping only the last one.
- **PPTX layout**: images and charts are now *scaled to fit* the remaining
  slide space (with a warning) instead of being silently dropped whenever a
  table was present. Placement is computed by a shared layout pass
  (`LayoutSlideVisuals`) used by both the emitter and the preview, so the
  two can never drift.
- **Preview / `see` are no longer blind**: image blocks render as embedded
  data URIs and chart blocks render as inline SVG (bar/line/pie) in DOCX,
  XLSX and PPTX previews. PPTX preview renders *every* slide (pagebreak
  blocks split slides).
- **BOM-tolerant parser**: a leading UTF-8 BOM no longer fails with
  `ERR_OIR_JSON` (`\ufeff`).
- **Exit code 3**: new `--strict` flag on `create`/`patch` exits 3 when any
  content was dropped or truncated (default remains exit 0, JSON carries
  `truncated: true` + `warnings`).
- **Dead code removed** (`renderSheet`, `xlsxContentTypesN`,
  `xlsxContentTypesDrawings`, `renderXSheetMerges`, duplicate PPTX branch
  in `verify`).
- **Footer hack removed**: the footer part is emitted with a proper
  `w:ftr` root instead of string surgery on a generated header.
- **README license clarity**: MIT text for v0.1.0 now ships as `LICENSE-MIT`.

### Added
- **Native charts in DOCX** (previously data-table fallback): `c:chart`
  parts with full content-type/rels plumbing, same renderers as xlsx/pptx.
- **`toc` block**: Word TOC field populated on open via `w:updateFields`
  (DOCX only; other formats warn).
- **New block types**: `h3`, `quote`, `divider`, `pagebreak` (page/slide
  break), `toc`.
- **`meta.freeze_header: true`**: freezes row 1 on every XLSX sheet.
- **Hash-pinned oracle installs**: `requirements-hashes.txt` (pip
  hash-checking mode, `--require-hashes`, includes transitive deps),
  SHA-256-verified downloads of the portable Python build and get-pip.py
  (`BRAINS_ORACLE_ALLOW_UNPINNED=1` overrides, not recommended).
- **Installers verify SHA-256**: `install.ps1` / `install.sh` download
  `SHA256SUMS.txt` from the release and refuse to install on mismatch.
  Tag overridable via `BRAINS_TAG` / `$env:BRAINS_TAG`.
- **`docx_chart` and freeze-header checks** in `verify`, `freeze_header`
  badge in xlsx preview, multi-flag `--file` usage documented.

### Notes
- `brains oracle setup` remains Windows-only (portable embeddable Python);
  on Linux/macOS install `oracle/requirements-hashes.txt` yourself and
  `oracle check` will find `python3` on PATH.
- DOCX charts are native now; the "DOCX charts render as data tables"
  limit from v0.1.0 no longer applies.
- 50-section/500-row/20-image/10-chart caps unchanged; one header/footer;
  Calibri family only.

## v0.1.0 — 2026-10-13

Initial public release: OIR compiler (docx/xlsx/pptx), forensic forge,
golden transplant, `--frozen`, oracle squad, vision loop, agent manual.
