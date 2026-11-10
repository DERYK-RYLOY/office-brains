// Package oracle embeds the strict-validation assets (Python scripts and
// pinned requirement files) so binary-only installs of `brains` can run
// `brains oracle setup|check` without a repo checkout.
package oracle

import "embed"

//go:embed verify.py measure.py diff_png.py requirements.txt requirements-hashes.txt
var FS embed.FS
