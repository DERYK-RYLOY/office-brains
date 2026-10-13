// Package forger builds docProps + ids with no AI provenance.
package forger

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// Frozen, when true, makes output byte-deterministic: fixed clock + counter IDs.
// Set by CLI --frozen. Same spec -> identical bytes, every run.
var Frozen bool

var paraCounter atomic.Uint32

// FrozenTime is the fixed clock for deterministic builds.
func FrozenTime() (created, modified time.Time) {
	utc := time.UTC
	modified = time.Date(2020, 1, 1, 14, 32, 0, 0, utc)
	created = time.Date(2019, 12, 29, 9, 14, 0, 0, utc)
	return created, modified
}

// Profile selects metadata behavior.
type Profile string

const (
	M365      Profile = "m365"
	Corporate Profile = "corporate"
	Clean     Profile = "clean"
)

// Props input for forging.
type Props struct {
	Profile Profile
	Company string
	Author  string
	// Times: if zero, human-like defaults are used.
	Created  time.Time
	Modified time.Time
	// Stats for app.xml
	Pages int
	Words int
	// AppName override e.g. Microsoft Office Word / Microsoft Excel
	AppName string
	// Frozen forces fixed clock (deterministic builds).
	Frozen bool
}

// HumanTimes returns Created 3 days ago 09:14, Modified today 14:32 local.
func HumanTimes(now time.Time) (created, modified time.Time) {
	modified = time.Date(now.Year(), now.Month(), now.Day(), 14, 32, 0, 0, now.Location())
	created = modified.AddDate(0, 0, -3)
	created = time.Date(created.Year(), created.Month(), created.Day(), 9, 14, 0, 0, created.Location())
	return created, modified
}

// Defaults fills empty fields.
func (p *Props) Defaults(appName string) {
	if p.Author == "" {
		p.Author = "User"
	}
	if p.Profile == "" {
		p.Profile = M365
	}
	if p.AppName == "" {
		p.AppName = appName
	}
	now := time.Now()
	if p.Created.IsZero() || p.Modified.IsZero() {
		var c, m time.Time
		if p.Frozen || Frozen {
			c, m = FrozenTime()
		} else {
			c, m = HumanTimes(now)
		}
		if p.Created.IsZero() {
			p.Created = c
		}
		if p.Modified.IsZero() {
			p.Modified = m
		}
	}
	if p.Pages == 0 {
		p.Pages = 1
	}
	switch p.Profile {
	case Clean:
		p.Company = ""
		p.Author = "User"
	case Corporate:
		if p.Company == "" {
			p.Company = "Contoso"
		}
	case M365:
		// Company empty is normal for personal M365
	}
}

// CoreXML renders docProps/core.xml (Transitional, Dublin Core).
func CoreXML(p Props) []byte {
	// dcterms:created/modified must be UTC ISO8601 with Z
	created := p.Created.UTC().Format("2006-01-02T15:04:05Z")
	modified := p.Modified.UTC().Format("2006-01-02T15:04:05Z")
	esc := xmlEsc(p.Author)
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:creator>` + esc + `</dc:creator>` +
		`<cp:lastModifiedBy>` + esc + `</cp:lastModifiedBy>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + created + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + modified + `</dcterms:modified>` +
		`</cp:coreProperties>`
	return []byte(s)
}

// AppXML renders docProps/app.xml. totalTime is minutes (human, not 0).
func AppXML(p Props, totalTime int) []byte {
	company := ""
	if p.Company != "" {
		company = `<Company>` + xmlEsc(p.Company) + `</Company>`
	}
	s := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
		`<Application>` + xmlEsc(p.AppName) + `</Application>` +
		company +
		`<TotalTime>` + fmt.Sprint(totalTime) + `</TotalTime>` +
		`<Pages>` + fmt.Sprint(p.Pages) + `</Pages>` +
		`<Words>` + fmt.Sprint(p.Words) + `</Words>` +
		`<ApplicationVersion>16.0300</ApplicationVersion>` +
		`</Properties>`
	return []byte(s)
}

// ResetCounter resets the frozen ID counter (tests only).
func ResetCounter() { paraCounter.Store(0) }

// ParaID generates w14:paraId hex (8 chars, value < 0x80000000, unique per paragraph).
// In Frozen mode IDs come from a process counter: same spec -> same IDs.
func ParaID() string {
	if Frozen {
		n := paraCounter.Add(1)
		var b [4]byte
		b[0] = byte((n >> 24) & 0x7F) // high bit clear
		b[1] = byte((n >> 16) & 0xFF)
		b[2] = byte((n >> 8) & 0xFF)
		b[3] = byte(n & 0xFF)
		if n == 0 {
			b[3] = 1
		}
		return hex.EncodeToString(b[:])
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	// clear high bit to stay < 0x80000000
	b[0] &= 0x7F
	if b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		b[3] = 1
	}
	return hex.EncodeToString(b[:])
}

func xmlEsc(s string) string {
	out := ""
	for _, r := range s {
		switch r {
		case '&':
			out += "&amp;"
		case '<':
			out += "&lt;"
		case '>':
			out += "&gt;"
		case '"':
			out += "&quot;"
		case '\'':
			out += "&apos;"
		default:
			out += string(r)
		}
	}
	return out
}
