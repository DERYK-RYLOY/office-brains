# OFFICE BRAINS

The last Office CLI that ever needs to exist. AI writes intent (OIR JSON),
`brains` compiles it to clean, Transitional, forensically-quiet
`.docx` / `.xlsx` / `.pptx` — stdlib Go only, no network, no daemons.

> Status: v0.1.0 bootstrap. Not published. Nothing here leaves your machine
> unless you push it yourself.

## Install

**Option A — binary (no Go needed, recommended for most people):**

1. Open the latest GitHub Release, download `brains-windows-amd64.exe`
   (or `brains-linux-amd64`).
2. Run `brains doctor`. Done — creating files works fully offline.

**Option B — from source (developers):**

```powershell
git clone <your-repo-url>
cd office-brains
go build -o brains ./cmd/brains
```

Requires Go 1.22+. No dependencies outside the standard library.

**Oracle (optional, strict checking):** needs one network download, once:

```powershell
.\brains oracle setup    # portable Python 3.12 + pinned pip packages into .oracle/
```

Windows only today (`oracle setup` fetches the Windows embeddable build;
Linux/macOS support is planned). Everything else runs anywhere Go runs.

## 60-second tour

```powershell
# 1. health check
.\brains doctor

# 2. write a spec (see pkg/spec/example.job.json for the shape)
.\brains create --spec pkg/spec/example.job.json --type docx -o out.docx --validate

# 3. check what you made
.\brains validate --file out.docx
.\brains inspect --file out.docx

# 4. deterministic bytes (same spec -> identical file)
.\brains create --spec pkg/spec/example.job.json --type docx -o out.docx --frozen
```

## OIR — the input contract

Agents never write XML. They write `pkg/spec/oir.schema.json`-validated JSON:

```json
{"oir": "1.1", "doc": {"sections": [
  {"name": "Q4", "blocks": [
    {"t": "h1", "txt": "Q4 Financials"},
    {"t": "table", "rows": [["Product","Rev"],["A",100]]},
    {"t": "chart", "kind": "bar", "title": "Rev",
     "labels": ["A"], "series": [{"name": "2026", "values": [100]}]}
  ]}
]}}
```

Block types: `h1 h2 p bullet code image chart table`.
Sections become sheets (xlsx) / slides (pptx).

## Commands

| Command | Purpose |
|---|---|
| `create --spec --type -o` | Build a file. Flags: `--profile`, `--company`, `--dna`, `--golden`, `--frozen`, `--validate`, `--human` |
| `patch --spec --patch -o` | RFC6902 ops (`replace`/`add`/`remove`) on a spec, optional `--type/--out-file/--validate` build |
| `inspect --file` | Parts + provenance scan |
| `validate --file [--xsd dir]` | Offline OOXML checks, optional xmllint XSD gate |
| `forge --file -o` | Replace `docProps` per profile, body byte-identical. Never overwrites in place |
| `diff --a --b` | Part-level diff with SHA + ignored `docProps` |
| `preview --spec --type -o` | Self-contained HTML preview, same math as emitters |
| `dna learn <dir> -o` | Learn company/author lineage from a corpus |
| `serve --port` | Localhost-only inspect/validate API |
| `doctor` | Environment + engine matrix |

Output contract: **stdout = JSON data, stderr = logs + `{"ok":false,"error":CODE}`**.
Stable codes: `ERR_OIR_*`, `ERR_PATCH_*`, `ERR_VALIDATE_FAILED`, `ERR_CREATE_*`.
`--human` switches stdout to people text. `NO_COLOR=1` respected (no color emitted, ever).

## Oracle squad (strict third-party validation)

Our own `validate` checks structure. The oracle opens every file with the
independent reference libraries — anything THEY reject is our bug:

| Package | Version | Job in the suite |
|---|---|---|
| Pillow | 12.3.0 | Real Calibri glyph measurement (`oracle/measure.py`) — the upgrade path for column widths |
| python-pptx | 1.0.2 | Strict PPTX open; shape/table/chart/picture inventory |
| openpyxl | 3.1.5 | Strict XLSX open; sheets, dimensions, merges, drawings |
| XlsxWriter | 3.2.9 | Chart/image gold standard reference |
| python-docx | 1.2.0 | Strict DOCX open; paragraphs, tables, images, author |
| lxml | 6.0.1 | Byte-strict XML parsing of every part (caught our `p:xfrm`/`a:xfrm` bug) |

Setup downloads automatically, isolated — no system Python touched, no admin:

```powershell
.\brains oracle setup    # portable Python 3.12 + pinned pip packages into .oracle/
.\brains oracle check --file out.docx --file out.xlsx --file out.pptx
```

`oracle/requirements.txt` pins every version. `.oracle/` is git-ignored.

## Vision loop (AI eyes, zero installs)

Humans catch "looks off" instantly; blind agents can't. The loop:

```powershell
.\brains see --spec job.json --type pptx -o see.png
# -> PNG for the VLM + math issues (overflow %, drops) in one call
.\brains patch --spec job.json --patch fix.json -o job2.json
.\brains see --spec job2.json --type pptx -o see2.png
.\brains oracle check --file out.pptx   # strict re-verify
```

- `see` = `preview` + headless Edge screenshot + `vision.Issues` audit.
  Single round trip: pixels for the VLM, rules for the code.
- Rules today: `pptx.slide_full` (error, with EMU math), `pptx.crowded`,
  `xlsx.wide_col/grid`, `docx.many_cols/long_cell`, plus `layout.clean`.
- `oracle/diff_png.py` diffs before/after PNGs (changed %, bbox) to prove
  the fix worked visually.
- Honest limit: the VLM sees the faithful preview (same constants as the
  emitters), not the binary. Glyph-level truth still needs Word/Excel.

## Golden transplant

Drop real Microsoft blanks next to the binary dir to clone instead of generate:

- `golden/blank.docx`, `golden/blank.xlsx`, `golden/blank.pptx`

then `create --golden golden`. Text/table specs transplant (styles/theme stay
Microsoft-original); image/chart/header specs fall back to clean builds.

Never commit licensed Microsoft files.

## Repo layout

```
cmd/brains        CLI (cobra-free, stdlib flags)
internal/oir      Spec + validation (coded errors)
internal/compiler docx/xlsx/pptx emitters + charts
internal/surgeon  Deterministic ZIP + golden transplant
internal/forger   Metadata (incl. --frozen deterministic mode)
internal/layout   Column/table width solvers
internal/verify   Offline checks + optional XSD gate
internal/patch    RFC6902 subset for specs
internal/preview  OIR -> HTML (same constants as emitters)
internal/dna      Corpus lineage learning
internal/media    PNG/JPEG loading (stdlib)
pkg/spec          oir.schema.json + example specs
golden/           Your blanks live here (empty, git-ignored)
```

## Limits (honest)

- Widths are approximations (no glyph shaping); dense sheets can drift low single digits
- DOCX charts render as data tables; bar/line/pie native on xlsx/pptx
- Pixel parity with Word is impossible for any non-Microsoft tool
- 50 sections, 500 table rows, 20 images, 10 charts, 8MB image caps

## License

MIT — see LICENSE.
