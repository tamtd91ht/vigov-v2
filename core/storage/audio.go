package storage

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

// CheckAudioStream reads an object whose head sniffed as MIMEMP3 or MIMEM4A and reports whether the
// WHOLE file is audio of that type. ok=false is a FILE outcome (refuse the upload); err is a read
// failure the caller retries (the reader's own errors — storage.ErrChanged included — come back
// wrapped, never as ok=false).
//
// WHY SNIFFING IS NOT ENOUGH FOR AN AUDIO-ONLY PURPOSE (content-audio, ADR 0067 §4): SniffMIME sees
// 512 bytes, and in both formats those bytes are a claim about what follows, not the thing itself.
//
//	audio/mp4   the `ftyp` brand says "M4A", but the tracks are declared in `moov`, which may sit
//	            anywhere in the file. A video with an M4A brand is a video. This walks the top-level
//	            boxes (skipping `mdat` without buffering it), parses `moov` (bounded: maxMoovBytes),
//	            and requires at least one sound track (`hdlr` = `soun`) and no track whose handler is
//	            outside audioTrackHandlers — a `vide` track refuses the file.
//	audio/mpeg  an ID3v2 tag can precede anything. This skips the tag (and up to maxID3Padding zero
//	            bytes some taggers leave after it), then requires TWO consecutive Layer III frames —
//	            the second at the offset the first one's header computes, so two bytes of luck are
//	            not enough.
//
// It never decodes audio and never looks at the duration: the duration is typed by the officer
// (ADR 0067 §4.1, ADR 0047 G7).
func CheckAudioStream(mime string, r io.Reader) (bool, error) {
	if r == nil {
		return false, fmt.Errorf("%w: reader is nil", ErrInvalidArgument)
	}
	br := bufio.NewReaderSize(r, 64<<10)
	switch mime {
	case MIMEMP3:
		return checkMP3Stream(br)
	case MIMEM4A:
		return checkM4AStream(br)
	}
	return false, fmt.Errorf("%w: %q is not an audio type", ErrInvalidArgument, mime)
}

// readOutcome sorts a read error: the file ending early is the FILE's fault (ok=false, nil); anything
// else is the store's (retry).
func readOutcome(err error) (bool, error) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return false, nil
	}
	return false, fmt.Errorf("storage: read audio: %w", err)
}

// --- MP3 --------------------------------------------------------------------------------------------

// maxID3Padding is how many zero bytes after a declared ID3 tag are tolerated before the first frame.
// The spec puts padding INSIDE the declared size; some taggers write it outside. Beyond this the
// file is not what the tag says.
const maxID3Padding = 64 << 10

func checkMP3Stream(br *bufio.Reader) (bool, error) {
	head, err := br.Peek(10)
	if err != nil {
		return readOutcome(err)
	}
	if isID3v2(head) {
		size := int64(head[6])<<21 | int64(head[7])<<14 | int64(head[8])<<7 | int64(head[9])
		skip := 10 + size
		if head[5]&0x10 != 0 { // footer present (ID3v2.4 §3.4)
			skip += 10
		}
		if _, err := io.CopyN(io.Discard, br, skip); err != nil {
			return readOutcome(err)
		}
		for zeros := 0; ; zeros++ {
			b, err := br.Peek(1)
			if err != nil {
				return readOutcome(err)
			}
			if b[0] != 0x00 {
				break
			}
			if zeros >= maxID3Padding {
				return false, nil
			}
			if _, err := br.Discard(1); err != nil {
				return readOutcome(err)
			}
		}
	}
	for frame := 0; frame < 2; frame++ {
		h, err := br.Peek(4)
		if err != nil {
			return readOutcome(err)
		}
		if !isMP3Frame(h) {
			return false, nil
		}
		n := mp3FrameLen(h)
		if n <= 4 {
			return false, nil
		}
		if _, err := io.CopyN(io.Discard, br, int64(n)); err != nil {
			return readOutcome(err)
		}
	}
	return true, nil
}

// Layer III bitrates (kbit/s) by bitrate index, and sampling rates (Hz) by rate index, per MPEG
// version. Index 0 (free) and 15 (bad) are already refused by isMP3Frame.
var (
	mp3BitratesV1 = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	mp3BitratesV2 = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
	mp3RatesV1    = [3]int{44100, 48000, 32000}
	mp3RatesV2    = [3]int{22050, 24000, 16000}
	mp3RatesV25   = [3]int{11025, 12000, 8000}
)

// MPEG audio version bits (header byte 1, bits 4-3). 0x01 is reserved and refused by isMP3Frame.
const (
	mpegV25 = 0x00
	mpegV2  = 0x02
	mpegV1  = 0x03
)

// mp3FrameLen is the length in bytes of the Layer III frame whose header is h (isMP3Frame(h) true).
func mp3FrameLen(h []byte) int {
	version := (h[1] >> 3) & 0x03
	bi := int(h[2] >> 4)
	ri := int((h[2] >> 2) & 0x03)
	pad := int((h[2] >> 1) & 0x01)
	switch version {
	case mpegV1:
		return 144000*mp3BitratesV1[bi]/mp3RatesV1[ri] + pad
	case mpegV2:
		return 72000*mp3BitratesV2[bi]/mp3RatesV2[ri] + pad
	case mpegV25:
		return 72000*mp3BitratesV2[bi]/mp3RatesV25[ri] + pad
	}
	return 0
}

// --- M4A (ISO BMFF) ---------------------------------------------------------------------------------

// maxMoovBytes bounds the `moov` box read into memory. An audio-only movie's `moov` is its sample
// tables: a 30 MiB AAC file at the lowest realistic bitrate holds well under 100 000 frames, i.e. a
// few hundred KiB of tables. 16 MiB is a wide margin that still refuses a crafted giant box.
const maxMoovBytes = 16 << 20

// audioTrackHandlers are the track handler types an audio-only file may carry: sound, and `text`
// (the chapter track of an .m4b audiobook). Everything else — `vide`, `pict`, `auxv`, `hint`,
// `meta`, `subt`… — refuses the file: closed list, false refusal over a guess.
var audioTrackHandlers = []string{"soun", "text"}

func checkM4AStream(br *bufio.Reader) (bool, error) {
	moovSeen := false
	first := true
	for {
		if _, err := br.Peek(1); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return readOutcome(err)
		}
		var hdr [16]byte
		if _, err := io.ReadFull(br, hdr[:8]); err != nil {
			return readOutcome(err)
		}
		size := uint64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		hl := uint64(8)
		toEOF := false
		switch size {
		case 1:
			if _, err := io.ReadFull(br, hdr[8:16]); err != nil {
				return readOutcome(err)
			}
			size, hl = binary.BigEndian.Uint64(hdr[8:16]), 16
		case 0:
			toEOF = true
		}
		if first && typ != "ftyp" {
			return false, nil // SniffMIME required `ftyp` at offset 0; anything else is not this file
		}
		first = false
		if !toEOF && size < hl {
			return false, nil
		}
		body := size - hl
		if typ == "moov" {
			if moovSeen {
				return false, nil // two movies: which one a player reads is the player's choice
			}
			moovSeen = true
			buf, err := readBox(br, body, toEOF)
			if err != nil || buf == nil {
				return false, err
			}
			if !moovAudioOnly(buf) {
				return false, nil
			}
		} else if toEOF {
			if _, err := io.Copy(io.Discard, br); err != nil {
				return readOutcome(err)
			}
		} else if body > 0 {
			if body > 1<<62 {
				return false, nil
			}
			if _, err := io.CopyN(io.Discard, br, int64(body)); err != nil {
				return readOutcome(err)
			}
		}
		if toEOF {
			break
		}
	}
	return moovSeen, nil
}

// readBox reads one box body of at most maxMoovBytes. (nil, nil) is a file outcome: too large or
// truncated.
func readBox(br *bufio.Reader, body uint64, toEOF bool) ([]byte, error) {
	if toEOF {
		buf, err := io.ReadAll(io.LimitReader(br, maxMoovBytes+1))
		if err != nil {
			_, err = readOutcome(err)
			return nil, err
		}
		if len(buf) > maxMoovBytes {
			return nil, nil
		}
		return buf, nil
	}
	if body > maxMoovBytes {
		return nil, nil
	}
	buf := make([]byte, body)
	if _, err := io.ReadFull(br, buf); err != nil {
		_, err = readOutcome(err)
		return nil, err
	}
	return buf, nil
}

// isoBox is one parsed child box.
type isoBox struct {
	typ  string
	body []byte
}

// children splits a box body into its child boxes. ok=false when a child's size runs past the
// parent — a malformed box is refused, never read around.
func children(b []byte) (out []isoBox, ok bool) {
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, false
		}
		size := uint64(binary.BigEndian.Uint32(b[0:4]))
		typ := string(b[4:8])
		hl := uint64(8)
		switch size {
		case 1:
			if len(b) < 16 {
				return nil, false
			}
			size, hl = binary.BigEndian.Uint64(b[8:16]), 16
		case 0:
			size = uint64(len(b))
		}
		if size < hl || size > uint64(len(b)) {
			return nil, false
		}
		out = append(out, isoBox{typ, b[hl:size]})
		b = b[size:]
	}
	return out, true
}

// moovAudioOnly: every `trak` has a `mdia/hdlr` whose handler is in audioTrackHandlers, and at least
// one is `soun`.
func moovAudioOnly(moov []byte) bool {
	boxes, ok := children(moov)
	if !ok {
		return false
	}
	sound := 0
	for _, trak := range boxes {
		if trak.typ != "trak" {
			continue
		}
		handler, ok := trackHandler(trak.body)
		if !ok || !slices.Contains(audioTrackHandlers, handler) {
			return false
		}
		if handler == "soun" {
			sound++
		}
	}
	return sound > 0
}

// trackHandler is `trak/mdia/hdlr` handler_type: a FullBox (4 bytes version+flags), pre_defined (4),
// then the handler (4) — ISO/IEC 14496-12 §8.4.3.
func trackHandler(trak []byte) (string, bool) {
	boxes, ok := children(trak)
	if !ok {
		return "", false
	}
	for _, mdia := range boxes {
		if mdia.typ != "mdia" {
			continue
		}
		inner, ok := children(mdia.body)
		if !ok {
			return "", false
		}
		for _, h := range inner {
			if h.typ == "hdlr" {
				if len(h.body) < 12 {
					return "", false
				}
				return string(h.body[8:12]), true
			}
		}
	}
	return "", false
}
