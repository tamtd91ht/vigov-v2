package storage

import (
	"bytes"
	"slices"
)

// SniffBytes is how much of an object ReadHead should fetch for SniffMIME. Every signature
// below sits in the first 16 bytes except an HEIF compatible-brand list, which lives inside the
// first `ftyp` box; 512 covers that box with a wide margin and costs one small range GET.
const SniffBytes = 512

// The closed allow-list: sniffed MIME type → key extension. A type absent here is refused,
// whatever the client declared. ADR 0052 §1c: the client's Content-Type is never trusted.
const (
	MIMEJPEG      = "image/jpeg"
	MIMEPNG       = "image/png"
	MIMEWebP      = "image/webp"
	MIMEHEIC      = "image/heic"
	MIMEMP4       = "video/mp4"
	MIMEQuickTime = "video/quicktime"
	MIMEPDF       = "application/pdf"
)

var extByMIME = map[string]string{
	MIMEJPEG:      "jpg",
	MIMEPNG:       "png",
	MIMEWebP:      "webp",
	MIMEHEIC:      "heic",
	MIMEMP4:       "mp4",
	MIMEQuickTime: "mov",
	MIMEPDF:       "pdf",
}

var mimeByExt = func() map[string]string {
	m := make(map[string]string, len(extByMIME))
	for mime, ext := range extByMIME {
		m[ext] = mime
	}
	return m
}()

// ExtForMIME returns the key extension of an allowed MIME type.
func ExtForMIME(mime string) (string, bool) {
	ext, ok := extByMIME[mime]
	return ext, ok
}

// isMedia reports whether a type may be served inline. ADR 0052 §4: anything that is not media
// is served as an attachment — PDF included, because a browser's PDF viewer is a script host.
func isMedia(mime string) bool {
	switch mime {
	case MIMEJPEG, MIMEPNG, MIMEWebP, MIMEHEIC, MIMEMP4, MIMEQuickTime:
		return true
	}
	return false
}

// ISO BMFF brands (the `ftyp` box). Only brands that mean the allowed type; audio-only
// (`M4A `), AVIF, 3GP and the rest are refused by omission.
var (
	mp4Brands  = []string{"isom", "iso2", "iso3", "iso4", "iso5", "iso6", "mp41", "mp42", "avc1", "M4V "}
	heicBrands = []string{"heic", "heix", "heim", "heis", "hevc", "hevx"}
)

// SniffMIME identifies a file from its first bytes against the closed allow-list. ok is false
// for anything else — HTML, SVG, executables, archives, and a PDF or image header that is not
// at offset 0.
//
// It never looks at a declared Content-Type or a file name. A polyglot (valid JPEG header,
// something else after it) sniffs as JPEG; it is then stored and served as image/jpeg with
// nosniff, where a browser will not execute it.
//
// QuickTime files without a leading `ftyp` box (very old `moov`/`mdat`-first files) are refused:
// a closed list prefers a false refusal to a guess. Current iPhone MOV files start with `ftyp qt  `.
func SniffMIME(head []byte) (mime, ext string, ok bool) {
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		mime = MIMEJPEG
	case bytes.HasPrefix(head, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		mime = MIMEPNG
	case len(head) >= 12 && bytes.Equal(head[0:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		mime = MIMEWebP
	case bytes.HasPrefix(head, []byte("%PDF-")):
		mime = MIMEPDF
	case len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")):
		mime = sniffISOBMFF(head)
	}
	if mime == "" {
		return "", "", false
	}
	return mime, extByMIME[mime], true
}

// sniffISOBMFF classifies an `ftyp` box: major brand first, then — for the generic HEIF brands
// `mif1`/`msf1` — the compatible brands, which must name HEIC.
func sniffISOBMFF(head []byte) string {
	major := string(head[8:12])
	switch {
	case major == "qt  ":
		return MIMEQuickTime
	case slices.Contains(mp4Brands, major):
		return MIMEMP4
	case slices.Contains(heicBrands, major):
		return MIMEHEIC
	case major == "mif1" || major == "msf1":
		// Box size is big-endian at offset 0; compatible brands start at 16, four bytes each.
		size := int(head[0])<<24 | int(head[1])<<16 | int(head[2])<<8 | int(head[3])
		if size > len(head) {
			size = len(head)
		}
		for off := 16; off+4 <= size; off += 4 {
			if slices.Contains(heicBrands, string(head[off:off+4])) {
				return MIMEHEIC
			}
		}
	}
	return ""
}
