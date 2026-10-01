package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// box builds one ISO BMFF box: 32-bit size, type, body.
func box(typ string, body ...[]byte) []byte {
	b := bytes.Join(body, nil)
	out := make([]byte, 8, 8+len(b))
	binary.BigEndian.PutUint32(out, uint32(8+len(b)))
	copy(out[4:], typ)
	return append(out, b...)
}

// trak is a track whose mdia/hdlr names handler.
func trak(handler string) []byte {
	hdlr := append([]byte{0, 0, 0, 0, 0, 0, 0, 0}, handler...)
	hdlr = append(hdlr, make([]byte, 13)...) // reserved[3] + empty name
	return box("trak", box("tkhd", make([]byte, 84)), box("mdia", box("mdhd", make([]byte, 24)), box("hdlr", hdlr)))
}

func ftypBox(major string, compatible ...string) []byte {
	body := append([]byte(major), 0, 0, 0, 0)
	for _, c := range compatible {
		body = append(body, c...)
	}
	return box("ftyp", body)
}

func m4aFile(major string, compatible []string, handlers ...string) []byte {
	moov := [][]byte{box("mvhd", make([]byte, 100))}
	for _, h := range handlers {
		moov = append(moov, trak(h))
	}
	return bytes.Join([][]byte{ftypBox(major, compatible...), box("moov", moov...), box("mdat", make([]byte, 4096))}, nil)
}

// mp3Frame is one MPEG-1 Layer III frame, 128 kbit/s, 44.1 kHz, no padding: 417 bytes.
func mp3Frame() []byte {
	f := make([]byte, 417)
	copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})
	return f
}

func id3Tag(size int) []byte {
	h := append([]byte("ID3"), 0x03, 0x00, 0x00,
		byte(size>>21&0x7F), byte(size>>14&0x7F), byte(size>>7&0x7F), byte(size&0x7F))
	return append(h, make([]byte, size)...)
}

func TestMP3FrameLen(t *testing.T) {
	cases := map[string]struct {
		h    []byte
		want int
	}{
		"mpeg1 128k 44.1k":        {[]byte{0xFF, 0xFB, 0x90, 0x00}, 417},
		"mpeg1 128k 44.1k padded": {[]byte{0xFF, 0xFB, 0x92, 0x00}, 418},
		"mpeg1 320k 48k":          {[]byte{0xFF, 0xFB, 0xE4, 0x00}, 960},
		"mpeg2 64k 22.05k":        {[]byte{0xFF, 0xF3, 0x80, 0x00}, 208},
		"mpeg2.5 8k 8k":           {[]byte{0xFF, 0xE3, 0x18, 0x00}, 72},
	}
	for name, c := range cases {
		if !isMP3Frame(c.h) {
			t.Fatalf("%s: not a frame header", name)
		}
		if got := mp3FrameLen(c.h); got != c.want {
			t.Errorf("%s: mp3FrameLen = %d, want %d", name, got, c.want)
		}
	}
}

func TestCheckAudioStreamAccepts(t *testing.T) {
	two := append(mp3Frame(), mp3Frame()...)
	cases := map[string]struct {
		mime string
		file []byte
	}{
		"mp3 bare frames":           {MIMEMP3, two},
		"mp3 after id3":             {MIMEMP3, append(id3Tag(300), two...)},
		"mp3 after id3 and padding": {MIMEMP3, append(append(id3Tag(20), make([]byte, 1000)...), two...)},
		"m4a one sound track":       {MIMEM4A, m4aFile("M4A ", []string{"M4A ", "isom"}, "soun")},
		"m4b sound plus chapters":   {MIMEM4A, m4aFile("M4B ", []string{"M4B "}, "soun", "text")},
		"m4a moov after mdat": {MIMEM4A, bytes.Join([][]byte{ftypBox("M4A "), box("mdat", make([]byte, 70000)),
			box("moov", box("mvhd", make([]byte, 100)), trak("soun"))}, nil)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ok, err := CheckAudioStream(c.mime, bytes.NewReader(c.file))
			if err != nil || !ok {
				t.Fatalf("CheckAudioStream = %v, %v; want true", ok, err)
			}
		})
	}
}

func TestCheckAudioStreamRefuses(t *testing.T) {
	cases := map[string]struct {
		mime string
		file []byte
	}{
		// THE CASE THIS FILE EXISTS FOR: a video wearing an M4A brand.
		"video track behind M4A brand":  {MIMEM4A, m4aFile("M4A ", []string{"M4A "}, "soun", "vide")},
		"video only behind M4A brand":   {MIMEM4A, m4aFile("M4A ", []string{"M4A "}, "vide")},
		"renamed mp4 video":             {MIMEM4A, m4aFile("isom", []string{"isom", "avc1"}, "vide", "soun")},
		"no sound track":                {MIMEM4A, m4aFile("M4A ", nil, "text")},
		"no moov":                       {MIMEM4A, append(ftypBox("M4A "), box("mdat", make([]byte, 64))...)},
		"two moov":                      {MIMEM4A, append(m4aFile("M4A ", nil, "soun"), box("moov", trak("soun"))...)},
		"hint track":                    {MIMEM4A, m4aFile("M4A ", nil, "soun", "hint")},
		"truncated mdat":                {MIMEM4A, m4aFile("M4A ", nil, "soun")[:200]},
		"not starting with ftyp":        {MIMEM4A, box("moov", trak("soun"))},
		"mp3 one frame only":            {MIMEMP3, mp3Frame()},
		"mp3 second header not a frame": {MIMEMP3, append(mp3Frame(), make([]byte, 417)...)},
		"id3 then mp4 video":            {MIMEMP3, append(id3Tag(20), m4aFile("isom", nil, "vide")...)},
		"id3 then nothing":              {MIMEMP3, id3Tag(50)},
		"id3 then too much padding":     {MIMEMP3, append(append(id3Tag(10), make([]byte, maxID3Padding+10)...), mp3Frame()...)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ok, err := CheckAudioStream(c.mime, bytes.NewReader(c.file))
			if err != nil {
				t.Fatalf("CheckAudioStream error %v; a malformed FILE is ok=false, not an error", err)
			}
			if ok {
				t.Fatal("CheckAudioStream accepted it")
			}
		})
	}
}

func TestCheckAudioStreamGiantMoovRefused(t *testing.T) {
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint32(hdr, uint32(8+maxMoovBytes+1))
	copy(hdr[4:], "moov")
	ok, err := CheckAudioStream(MIMEM4A, bytes.NewReader(append(ftypBox("M4A "), hdr...)))
	if ok || err != nil {
		t.Fatalf("CheckAudioStream = %v, %v; want false, nil", ok, err)
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

// A read failure is the STORE's, not the file's: it must come back as an error (retry), never as a
// refusal that rejects a good upload.
func TestCheckAudioStreamReadErrorIsError(t *testing.T) {
	sentinel := errors.New("store down")
	for _, mime := range []string{MIMEMP3, MIMEM4A} {
		r := io.MultiReader(bytes.NewReader(m4aFile("M4A ", nil, "soun")[:20]), failingReader{sentinel})
		if mime == MIMEMP3 {
			r = io.MultiReader(bytes.NewReader(mp3Frame()[:100]), failingReader{sentinel})
		}
		ok, err := CheckAudioStream(mime, r)
		if ok || !errors.Is(err, sentinel) {
			t.Errorf("%s: CheckAudioStream = %v, %v; want the reader's error", mime, ok, err)
		}
	}
}

func TestCheckAudioStreamNonAudioType(t *testing.T) {
	if _, err := CheckAudioStream(MIMEMP4, bytes.NewReader(nil)); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
}

// Audio is media: served inline so an <audio> element can play it (ADR 0052 §4).
func TestAudioIsInline(t *testing.T) {
	for _, m := range []string{MIMEMP3, MIMEM4A} {
		if !isMedia(m) {
			t.Errorf("%s is not media", m)
		}
	}
}
