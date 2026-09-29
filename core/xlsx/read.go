// Package xlsx is the ONE reader of spreadsheet uploads, plus the writer of their templates, shared
// by every import in every service (residential units, staff, the catalogues).
//
// WHY IT IS IN core: every import used to carry its own copy of the reader, and the copies drifted.
// service-identity's got the XML depth guard (GO-2026-6088) and the macro refusal; service-petitions'
// did not. Rule 2 forbids importing another service's internal/, so the copy that got the hardening
// could not be reused, and the next import would have copied whichever one it found first. A guard
// that exists in one of two copies is a guard the attacker picks the other copy to avoid.
//
// THE UPLOAD IS UNTRUSTED INPUT FROM A BROWSER, and an .xlsx is a zip of XML — the two shapes with the
// longest history of parser abuse. What ReadSheet does about it, in the order it happens:
//
//  1. IN MEMORY ONLY, under a byte cap read from the stream itself (the declared size is used only to
//     refuse early; it is never trusted to bound the read). excelize is configured so it never spills
//     a part to a temporary file (openOptions).
//  2. THE ZIP IS INSPECTED BEFORE EXCELIZE SEES IT: magic bytes, entry count, the DECLARED
//     uncompressed size of every entry summed against a cap (archive/zip refuses an entry whose data
//     outruns its declared size, so the declaration is binding) — a zip bomb is refused by
//     arithmetic, before one byte is inflated.
//  3. MACROS ARE REFUSED, not ignored: a vbaProject part, or a macro-enabled content type. Nothing here
//     would run a macro, but a file that carries one is not the file a template produced, and it would
//     travel on in backups as something this system accepted.
//  4. EVERY ZIP ENTRY — whatever its name — IS SCANNED AS XML, NO DEEPER THAN MaxXMLDepth, and every
//     XML part must be well-formed, before excelize opens the file (checkPart). excelize's row
//     iterator stops SILENTLY at a decoding error, so a truncated sheet would import its first half
//     and report success — exactly the partial write an all-or-nothing import exists to prevent.
//  5. NO FORMULA IS EVALUATED. Cells are read raw (RawCellValue): a formula cell yields the value the
//     spreadsheet cached, never a computation; a number yields its stored digits ("1.5", not a display
//     format that rounds it to "2"); a date yields its serial number, which the caller converts.
//  6. THE SHEET IS BOUNDED: at most MaxRows rows scanned, blank ones included, and at most MaxColumns
//     cells kept per row.
//
// ERRORS ARE THE BARE SENTINELS BELOW, never wrapped with a part name or a cell value. A part name
// is attacker-chosen text and a cell may hold a citizen's name or phone number (rule 3, forbidden
// #3): the caller maps each sentinel to ONE fixed sentence, and nothing from the file reaches a log
// or a response through this package.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html/charset"
)

// The refusals. A caller maps each one to a fixed sentence (and a status: ErrTooLarge 413,
// ErrNotXLSX / ErrMacroEnabled 415, the rest 400 or 422). Never echo err.Error() to a client.
var (
	// ErrNotXLSX — not a zip, not an ordinary workbook (.xlsx), encrypted (an OLE container), or no
	// sheet excelize can open.
	ErrNotXLSX = errors.New("xlsx: not an .xlsx workbook")
	// ErrMacroEnabled — the package carries a VBA project or declares a macro-enabled content type.
	ErrMacroEnabled = errors.New("xlsx: macro-enabled workbook refused")
	// ErrTooLarge — over the file cap, the zip-entry cap, or the unzipped cap.
	ErrTooLarge = errors.New("xlsx: workbook too large")
	// ErrMalformed — a part nests deeper than MaxXMLDepth, or an XML part is truncated / not
	// well-formed, or a row cannot be decoded.
	ErrMalformed = errors.New("xlsx: workbook is malformed")
	// ErrTooManyRows — the first sheet has more than MaxRows rows, blank ones included.
	ErrTooManyRows = errors.New("xlsx: too many rows")
	// ErrEmptySheet — the first sheet has no non-blank cell.
	ErrEmptySheet = errors.New("xlsx: first sheet is empty")
)

// Limits bounds one read. The zero value means DefaultLimits.
//
// A FIELD CAN ONLY TIGHTEN, NEVER LOOSEN: zero, a negative value, or a value above the default is
// replaced by the default (normalize). The defaults are the security thresholds of rule 13, and
// "silently loosening a threshold" is its forbidden #4 — a caller that genuinely needs a larger file
// changes the default here, in review, where every import sees the change.
type Limits struct {
	MaxFileBytes     int64  // the upload itself
	MaxZipEntries    int    // parts in the package
	MaxUnzippedBytes uint64 // sum of every part's declared uncompressed size
	MaxXMLDepth      int    // element nesting in any part
	MaxRows          int    // rows of the first sheet scanned, blank ones included
	MaxColumns       int    // cells kept per row; the rest are dropped unread
}

// DefaultLimits is the STRICTER of the two readers this package replaces
// (service-identity/internal/orgunitxlsx, service-petitions/internal/http/task_import.go):
//
//	MaxFileBytes      2 MB    both. A 500-row template is under 40 KB; 2 MB is fifty times that
//	MaxZipEntries     64      petitions (identity allowed 200). A workbook saved by Excel has a
//	                          dozen parts; one with pictures and printer settings, some twenty
//	MaxUnzippedBytes  20 MB   both. A real template inflates to well under 1 MB
//	MaxXMLDepth       64      identity (petitions had none). A real workbook nests about ten deep
//	MaxRows           5000    petitions (identity bounded non-blank rows only, so a sheet of
//	                          blank rows was scanned without end)
//	MaxColumns        64      petitions kept its headings + 8; no import here has 20 columns
var DefaultLimits = Limits{
	MaxFileBytes:     2 << 20,
	MaxZipEntries:    64,
	MaxUnzippedBytes: 20 << 20,
	MaxXMLDepth:      64,
	MaxRows:          5000,
	MaxColumns:       64,
}

// normalize replaces every field that is unset or LOOSER than the default with the default.
func (l Limits) normalize() Limits {
	d := DefaultLimits
	if l.MaxFileBytes <= 0 || l.MaxFileBytes > d.MaxFileBytes {
		l.MaxFileBytes = d.MaxFileBytes
	}
	if l.MaxZipEntries <= 0 || l.MaxZipEntries > d.MaxZipEntries {
		l.MaxZipEntries = d.MaxZipEntries
	}
	if l.MaxUnzippedBytes == 0 || l.MaxUnzippedBytes > d.MaxUnzippedBytes {
		l.MaxUnzippedBytes = d.MaxUnzippedBytes
	}
	if l.MaxXMLDepth <= 0 || l.MaxXMLDepth > d.MaxXMLDepth {
		l.MaxXMLDepth = d.MaxXMLDepth
	}
	if l.MaxRows <= 0 || l.MaxRows > d.MaxRows {
		l.MaxRows = d.MaxRows
	}
	if l.MaxColumns <= 0 || l.MaxColumns > d.MaxColumns {
		l.MaxColumns = d.MaxColumns
	}
	return l
}

// contentTypeMainXLSX is the one main-part type an ordinary workbook declares. A macro-enabled
// workbook (.xlsm), a template (.xltx) and a binary workbook (.xlsb) each declare a different one.
const contentTypeMainXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"

// ReadSheet reads the FIRST sheet of an uploaded workbook into rows of raw cell text.
//
// size is the length the caller was told (a multipart header's Size); pass -1 when unknown. It only
// refuses early — the read itself is capped at MaxFileBytes whatever size says.
//
// rows[i] is spreadsheet row i+1: the header is rows[0], and a blank row in the middle is kept as an
// empty slice so a caller's row numbers stay the spreadsheet's own. Trailing blank rows are dropped.
// Rows are ragged: a row is as long as its last non-empty cell (at most MaxColumns). Values are NOT
// trimmed — that, the header check and every business rule are the caller's.
//
// A merged range reads as its top-left cell holding the value and the rest empty.
//
// The error is one of the sentinels above, or — only when r itself fails — a wrapped read error.
func ReadSheet(r io.Reader, size int64, lim Limits) ([][]string, error) {
	lim = lim.normalize()
	if size > lim.MaxFileBytes {
		return nil, ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, lim.MaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("xlsx: read upload: %w", err)
	}
	if int64(len(data)) > lim.MaxFileBytes {
		return nil, ErrTooLarge
	}
	if err := inspect(data, lim); err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(bytes.NewReader(data), openOptions(lim))
	if err != nil {
		// inspect has already bounded the sum, so this is belt and braces. excelize reports an
		// exceeded limit with an unexported error type, hence the text match.
		if errors.Is(err, excelize.ErrOptionsUnzipSizeLimit) || strings.Contains(err.Error(), "unzip size") {
			return nil, ErrTooLarge
		}
		return nil, ErrNotXLSX
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, ErrNotXLSX
	}
	it, err := f.Rows(sheets[0])
	if err != nil {
		return nil, ErrNotXLSX
	}
	defer it.Close()

	var out [][]string
	lastNonBlank := -1
	for it.Next() {
		if len(out) >= lim.MaxRows {
			return nil, ErrTooManyRows
		}
		cells, err := it.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, ErrMalformed
		}
		if len(cells) > lim.MaxColumns {
			cells = cells[:lim.MaxColumns]
		}
		out = append(out, cells)
		if !blank(cells) {
			lastNonBlank = len(out) - 1
		}
	}
	if err := it.Error(); err != nil {
		return nil, ErrMalformed
	}
	if lastNonBlank < 0 {
		return nil, ErrEmptySheet
	}
	return out[:lastNonBlank+1], nil
}

// blank reports whether every cell is empty after trimming spaces.
func blank(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// inspect is steps 2–4 of the package comment, over the raw bytes.
func inspect(data []byte, lim Limits) error {
	if len(data) < 4 || !bytes.Equal(data[:4], []byte("PK\x03\x04")) {
		// An encrypted workbook is an OLE container, not a zip, and lands here too: there is no
		// password to open it with, and asking for one is not an import's job.
		return ErrNotXLSX
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ErrNotXLSX
	}
	if len(zr.File) > lim.MaxZipEntries {
		return ErrTooLarge
	}
	var total uint64
	var contentTypes []byte
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > lim.MaxUnzippedBytes {
			return ErrTooLarge
		}
		name := normName(f.Name)
		if strings.Contains(name, "vbaproject") || strings.Contains(name, "vbadata") {
			return ErrMacroEnabled
		}
		if name == "[content_types].xml" {
			if contentTypes, err = readPart(f, lim); err != nil {
				return ErrNotXLSX
			}
		}
	}
	if contentTypes == nil {
		return ErrNotXLSX
	}
	ct := strings.ToLower(string(contentTypes))
	if strings.Contains(ct, "macroenabled") || strings.Contains(ct, "vbaproject") {
		return ErrMacroEnabled
	}
	if !strings.Contains(ct, contentTypeMainXLSX) {
		return ErrNotXLSX
	}
	// EVERY ENTRY, WHATEVER ITS NAME — see checkPart for why the name cannot be trusted.
	for _, f := range zr.File {
		if err := checkPart(f, lim); err != nil {
			return ErrMalformed
		}
	}
	return nil
}

func normName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
}

// readPart inflates one entry. Bounded by its declared size, which inspect has already summed.
func readPart(f *zip.File, lim Limits) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("xlsx: open part: %w", err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, int64(lim.MaxUnzippedBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("xlsx: read part: %w", err)
	}
	return b, nil
}

// errXMLTooDeep — a part nests deeper than MaxXMLDepth. Internal: inspect returns ErrMalformed.
//
// WHY THE DEPTH GUARD EXISTS: GO-2026-6088 — encoding/xml before go1.26.6 has no recursion-depth
// guard in Decode / Unmarshal, and excelize unmarshals every part on open, so a deeply nested part
// exhausts the stack of the request's goroutine — fatal, it kills the replica and every commune it
// serves. Token() is iterative and safe on the same input, which is why the depth is counted HERE,
// before excelize sees a byte. Keep the cap after the toolchain is upgraded: it also bounds the work
// a hostile file can ask for.
var errXMLTooDeep = errors.New("xlsx: XML nested too deep")

// isXMLName reports whether an entry is NAMED as XML: `.xml`, relationship parts (`.rels`) and legacy
// VML drawings (`.vml`). Such an entry must be well-formed outright. The name is NOT what decides
// whether excelize decodes an entry — see checkPart.
func isXMLName(name string) bool {
	return strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") || strings.HasSuffix(name, ".vml")
}

// checkPart is the XML guard of ONE zip entry, and it runs on EVERY entry.
//
// WHY NOT ONLY THE `.xml` ONES: excelize finds its parts through relationship TARGETS, not names — the
// workbook part is whatever `_rels/.rels` points at (excelize@v2.11.0 workbook.go getWorkbookPath),
// the sheets are whatever the workbook's relationships point at — and it XML-decodes each one. A
// workbook part named `xl/wb.bin` skipped a name-based guard and reached encoding/xml's recursive
// Unmarshal (found and fixed in service-identity, 92ddffc). Chasing the targets instead is weaker
// still: a relationship's declared Type is attacker text too, and excelize loads sheets by rId
// whatever Type says.
//
// WHY THIS RULE PROVABLY COVERS EVERY PART EXCELIZE DECODES: the scan uses the same decoder excelize
// builds (xmlDecoder: encoding/xml, Strict, the same CharsetReader), over the same bytes, so it sees
// the same token stream. Unmarshal's recursion only ever follows the element depth of the tokens it
// has read, so:
//
//   - depth past MaxXMLDepth anywhere in the stream is refused, EVEN IF the stream is broken later —
//     Unmarshal would already have recursed that deep before reaching the break;
//   - a stream that breaks after at least one element is refused: that is truncated or corrupted XML,
//     and excelize's row iterator would import the half before the break silently;
//   - a stream that breaks BEFORE ITS FIRST ELEMENT is accepted when the entry is not named as XML:
//     that is binary (a PNG's first byte is not UTF-8, a printerSettings .bin opens with NUL bytes),
//     and excelize's decoder gets no element out of it either, so it can neither recurse nor import a
//     row. This is what keeps a real file with a picture or printer settings importable.
//
// excelize's own re-decodes (extLst fragments wrapped in one extra element) are inner XML of a part
// already scanned, so they add one level at most.
func checkPart(f *zip.File, lim Limits) error {
	elements, err := scanXML(f, lim)
	if err == nil || errors.Is(err, errXMLTooDeep) {
		return err
	}
	if elements > 0 || isXMLName(normName(f.Name)) {
		return err
	}
	return nil
}

// xmlDecoder is excelize's decoder, built the way excelize@v2.11.0 builds it (excelize.go
// xmlNewDecoder, with the default Options.CharsetReader = charset.NewReaderLabel). A decoder that
// differed — no CharsetReader, say — would stop at an `encoding="…"` declaration that excelize reads
// straight through, and would count no depth where excelize recurses.
func xmlDecoder(r io.Reader) *xml.Decoder {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	return dec
}

// scanXML decodes one entry token by token, counting start elements and refusing nesting past
// MaxXMLDepth. encoding/xml expands no external entities and has no DTD processing, so this is not
// itself an XXE surface; Token() is iterative, so the scan is safe on the input it refuses. The
// errors it returns stay inside this package (inspect maps them to ErrMalformed).
func scanXML(f *zip.File, lim Limits) (int, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("xlsx: open part: %w", err)
	}
	defer rc.Close()
	dec := xmlDecoder(io.LimitReader(rc, int64(lim.MaxUnzippedBytes)+1))
	depth, elements := 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return elements, nil
		}
		if err != nil {
			return elements, fmt.Errorf("xlsx: invalid part: %w", err)
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
			elements++
			if depth > lim.MaxXMLDepth {
				return elements, errXMLTooDeep
			}
		case xml.EndElement:
			depth--
		}
	}
}

// openOptions keeps excelize in memory: UnzipXMLSizeLimit equal to the whole unzipped cap means no
// single part can exceed it (inspect has already bounded their sum), and a part over that limit is
// the one thing excelize would otherwise extract to the system temporary directory.
func openOptions(lim Limits) excelize.Options {
	return excelize.Options{
		RawCellValue:      true,
		UnzipSizeLimit:    int64(lim.MaxUnzippedBytes),
		UnzipXMLSizeLimit: int64(lim.MaxUnzippedBytes),
	}
}
