# Contributing to Office Brains

Help is welcome — issues, specs, tests, docs, oracle rules. Stealing isn't:
the code is under the Business Source License 1.1 (see LICENSE). By
contributing you agree your work ships under the same terms, which is exactly
what earns contributors the full-use grant in the Additional Use Grant.

## How to help

1. **Use the tool, file what breaks.** Open an issue with the spec JSON,
   the command, and the `error` code. Bonus: the `see` PNG.
2. **Fix it.** Fork, branch, then prove it:
   - `go vet ./...` clean, `go test -count=1 ./...` green.
   - New emitter behavior needs a `validate` + `oracle check` pass on a
     real file (paste the check summary in the PR).
   - Never hand-edit OOXML in a PR description — patch the OIR or the Go.
3. **One concern per PR.** Small, reviewable, tested.

## Rules

- Sign off your commits (`git commit -s`) — Developer Certificate of Origin.
- Keep `stdout = JSON, stderr = logs`. No color codes, ever (`NO_COLOR`).
- New block types need: OIR validation + error code, emitter support in all
  three formats (or a documented fallback), schema + docs updates.
- Don't commit generated files, `.oracle/`, `golden/*.{docx,xlsx,pptx}`,
  or `dna.json` files with real company names.

## Commercial use

Contributors who submit their changes back may use the Licensed Work for any
purpose (LICENSE, Additional Use Grant). Everyone else: non-production is
free; production needs a commercial license. Talk to Deryk Ryloy.
