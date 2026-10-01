package storage

import (
	"bytes"
	"slices"
)

// SniffBytes is how much of an object ReadHead should fetch for SniffMIME. Every signature
// below sits in the first 16 bytes except an `ftyp` compatible-brand list (HEIF, M4A), which lives
// inside the first `ftyp` box; 512 covers that box with a wide margin and costs one small range GET.
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
	// Audio of a Mini App `truyen-thanh` item (ADR 0067 §4). audio/mp4 is the IANA type of an .m4a
	// file (RFC 4337); `audio/x-m4a` is a client spelling, never what is sniffed or stored.
	MIMEMP3 = "audio/mpeg"
	MIMEM4A = "audio/mp4"
)

var extByMIME = map[string]string{
	MIMEJPEG:      "jpg",
	MIMEPNG:       "png",
	MIMEWebP:      "webp",
	MIMEHEIC:      "heic",
	MIMEMP4:       "mp4",
	MIMEQuickTime: "mov",
	MIMEPDF:       "pdf",
	MIMEMP3:       "mp3",
	MIMEM4A:       "m4a",
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
	case MIMEJPEG, MIMEPNG, MIMEWebP, MIMEHEIC, MIMEMP4, MIMEQuickTime, MIMEMP3, MIMEM4A:
		return true
	}
	return false
}

// ISO BMFF brands (the `ftyp` box). Only brands that mean the allowed type; AVIF, 3GP, `M4P `
// (DRM-protected iTunes audio) and the rest are refused by omission.
//
// AUDIO (`M4A ` / `M4B `) WAS REFUSED BY OMISSION UNTIL ADR 0067 §4, because no purpose allowed
// audio and a brand list that admitted it would have let an .m4a be stored as video/mp4. It is now
// its own type, audio/mp4, decided so that VIDEO CANNOT SNEAK IN AS AUDIO:
//   - major brand `M4A ` / `M4B ` → audio/mp4. The brand is the file's own claim of audio-only;
//   - major brand one of mp4Brands (`isom`, `mp42`…) → audio/mp4 ONLY when a compatible brand is
//     `M4A ` / `M4B ` AND no compatible brand names video (videoMarkerBrands). Otherwise it stays
//     video/mp4 — which is how every video upload sniffed before this change, so content-video's
//     classification moves only for a file that itself claims to be M4A audio;
//   - an `isom`-major audio file with no M4A brand at all sniffs as video/mp4 and is refused under
//     content-audio. A false refusal, which a closed list prefers to a guess.
//
// A brand is still only a CLAIM about the first 512 bytes. A purpose that must hold audio and
// nothing else (content-audio) ALSO runs CheckAudioStream over the whole object, which reads the
// track handlers in `moov` and refuses any video track — the proof the brand cannot give.
var (
	mp4Brands         = []string{"isom", "iso2", "iso3", "iso4", "iso5", "iso6", "mp41", "mp42", "avc1", "M4V "}
	m4aBrands         = []string{"M4A ", "M4B "}
	videoMarkerBrands = []string{"M4V ", "M4VH", "M4VP", "avc1", "qt  "}
	heicBrands        = []string{"heic", "heix", "heim", "heis", "hevc", "hevx"}
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
	case isID3v2(head) || isMP3Frame(head):
		mime = MIMEMP3
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
	case slices.Contains(m4aBrands, major):
		return MIMEM4A
	case slices.Contains(mp4Brands, major):
		compat := compatibleBrands(head)
		if slices.ContainsFunc(compat, func(b string) bool { return slices.Contains(m4aBrands, b) }) &&
			!slices.ContainsFunc(compat, func(b string) bool { return slices.Contains(videoMarkerBrands, b) }) {
			return MIMEM4A
		}
		return MIMEMP4
	case slices.Contains(heicBrands, major):
		return MIMEHEIC
	case major == "mif1" || major == "msf1":
		if slices.ContainsFunc(compatibleBrands(head), func(b string) bool { return slices.Contains(heicBrands, b) }) {
			return MIMEHEIC
		}
	}
	return ""
}

// compatibleBrands lists the compatible brands of the `ftyp` box at offset 0, as far as head holds
// it. Box size is big-endian at offset 0; compatible brands start at 16, four bytes each.
func compatibleBrands(head []byte) []string {
	size := int(head[0])<<24 | int(head[1])<<16 | int(head[2])<<8 | int(head[3])
	if size > len(head) {
		size = len(head)
	}
	var out []string
	for off := 16; off+4 <= size; off += 4 {
		out = append(out, string(head[off:off+4]))
	}
	return out
}

// isID3v2 reports an ID3v2 tag header at offset 0 (id3.org, ID3v2.4 §3.1): "ID3", major version
// 2..4, revision not 0xFF, and a four-byte syncsafe size (every byte < 0x80). An ID3 tag can in
// principle precede anything, so this is the HEAD'S claim only; CheckAudioStream verifies that an
// MPEG audio frame actually follows the tag.
func isID3v2(head []byte) bool {
	if len(head) < 10 || !bytes.HasPrefix(head, []byte("ID3")) {
		return false
	}
	if head[3] < 2 || head[3] > 4 || head[4] == 0xFF {
		return false
	}
	for _, b := range head[6:10] {
		if b >= 0x80 {
			return false
		}
	}
	return true
}

// isMP3Frame reports an MPEG audio LAYER III frame header at offset 0 (ISO/IEC 11172-3 §2.4.1.3,
// 13818-3): 11-bit sync, a version that is not the reserved one, layer III, a bitrate index that is
// neither "free" (0) nor "bad" (15), a sampling-rate index that is not reserved. Layer I/II and
// ADTS AAC (layer bits 00) are refused: the decision says MP3.
//
// NO COLLISION WITH THE OTHER SIGNATURES: JPEG starts FF D8 — 0xD8's top three bits are 110, not the
// 111 of a frame sync.
func isMP3Frame(head []byte) bool {
	if len(head) < 4 || head[0] != 0xFF || head[1]&0xE0 != 0xE0 {
		return false
	}
	version := (head[1] >> 3) & 0x03
	layer := (head[1] >> 1) & 0x03
	bitrate := head[2] >> 4
	rate := (head[2] >> 2) & 0x03
	return version != 0x01 && layer == 0x01 && bitrate != 0x00 && bitrate != 0x0F && rate != 0x03
}
