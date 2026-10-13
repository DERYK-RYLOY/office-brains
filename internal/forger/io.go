package forger

import (
	"errors"
	"os"
)

var errUnknownType = errors.New("unknown Office type (no word/document.xml, xl/workbook.xml, ppt/presentation.xml)")

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
