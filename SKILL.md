# office-brains skill — forensic Office files from JSON

Use when the user wants a `.docx`, `.xlsx`, or `.pptx` file created or edited.

1. Write an OIR spec (`pkg/spec/oir.schema.json` in the repo; blocks:
   `h1 h2 p bullet code image chart table`).
2. `brains create --spec job.json --type docx|xlsx|pptx -o out --validate`
3. `brains oracle check --file out` — independent strict open.
4. `brains see --spec job.json --type <t> -o see.png` — look at the render;
   fix `error`-severity issues by editing the spec (split crowded slides),
   never by hand-editing XML.
5. Small fixes via `brains patch` (RFC6902 `replace` on JSON paths).

Full operator manual: `AGENTS.md`. JSON on stdout, errors on stderr.
