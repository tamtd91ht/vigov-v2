package storage

import "testing"

// ftyp builds an ISO BMFF `ftyp` box: size, "ftyp", major brand, minor version, compatible brands.
func ftyp(major string, compatible ...string) []byte {
	size := 16 + 4*len(compatible)
	b := []byte{byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)}
	b = append(b, "ftyp"...)
	b = append(b, major...)
	b = append(b, 0, 0, 0, 0)
	for _, c := range compatible {
		b = append(b, c...)
	}
	return append(b, 0, 0, 0, 8, 'f', 'r', 'e', 'e')
}

func TestSniffMIMEAllowed(t *testing.T) {
	cases := []struct {
		name, mime, ext string
		head            []byte
	}{
		{"jpeg jfif", MIMEJPEG, "jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}},
		{"jpeg exif", MIMEJPEG, "jpg", []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x12, 0x34, 'E', 'x', 'i', 'f', 0x00}},
		{"png", MIMEPNG, "png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0x0D, 'I', 'H', 'D', 'R'}},
		{"webp", MIMEWebP, "webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")},
		{"pdf", MIMEPDF, "pdf", []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")},
		{"mp4 isom", MIMEMP4, "mp4", ftyp("isom", "isom", "iso2", "avc1", "mp41")},
		{"mp4 mp42", MIMEMP4, "mp4", ftyp("mp42", "mp42", "isom")},
		{"quicktime", MIMEQuickTime, "mov", ftyp("qt  ", "qt  ")},
		{"heic", MIMEHEIC, "heic", ftyp("heic", "mif1", "heic")},
		{"heif mif1 with heic", MIMEHEIC, "heic", ftyp("mif1", "mif1", "heic")},
		// ADR 0067 §4: audio of a truyen-thanh item.
		{"m4a itunes", MIMEM4A, "m4a", ftyp("M4A ", "M4A ", "mp42", "isom")},
		{"m4a ffmpeg ipod", MIMEM4A, "m4a", ftyp("M4A ", "M4A ", "isom", "iso2")},
		{"m4b audiobook", MIMEM4A, "m4a", ftyp("M4B ", "M4B ", "mp42", "isom")},
		{"mp42 major, M4A compatible", MIMEM4A, "m4a", ftyp("mp42", "M4A ", "mp42", "isom")},
		{"isom major, M4A compatible", MIMEM4A, "m4a", ftyp("isom", "isom", "M4A ")},
		{"mp3 id3v2.3", MIMEMP3, "mp3", append([]byte("ID3"), 0x03, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00)},
		{"mp3 id3v2.4", MIMEMP3, "mp3", append([]byte("ID3"), 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14)},
		{"mp3 frame mpeg1 128k", MIMEMP3, "mp3", []byte{0xFF, 0xFB, 0x90, 0x64}},
		{"mp3 frame mpeg2 64k", MIMEMP3, "mp3", []byte{0xFF, 0xF3, 0x80, 0xC4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mime, ext, ok := SniffMIME(c.head)
			if !ok || mime != c.mime || ext != c.ext {
				t.Fatalf("SniffMIME = %q, %q, %v; want %q, %q", mime, ext, ok, c.mime, c.ext)
			}
		})
	}
}

func TestSniffMIMERefused(t *testing.T) {
	cases := map[string][]byte{
		"empty":           nil,
		"html":            []byte("<!DOCTYPE html><html><script>alert(1)</script>"),
		"html leading ws": []byte("  \n<html><body>"),
		"svg":             []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)">`),
		"svg bare":        []byte(`<svg xmlns="http://www.w3.org/2000/svg">`),
		"exe":             []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff"),
		"elf":             []byte("\x7fELF\x02\x01\x01\x00"),
		"zip":             []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"),
		"zip named .jpg":  []byte("PK\x03\x04\x14\x00\x06\x00"),
		"docx (zip)":      []byte("PK\x03\x04\x14\x00\x06\x00\x08\x00\x00\x00!\x00"),
		"gif":             []byte("GIF89a\x01\x00\x01\x00"),
		"riff wav":        []byte("RIFF\x24\x00\x00\x00WAVEfmt "),
		// Audio the decision does not name (ADR 0067 §4 says MP3 / M4A, nothing else), DRM-protected
		// iTunes audio (`M4P `), and MPEG headers that are not a valid Layer III frame.
		"m4p drm audio":        ftyp("M4P ", "M4P ", "mp42", "isom"),
		"riff wav pcm":         append([]byte("RIFF\xff\xff\x00\x00WAVEfmt "), 0x10, 0, 0, 0),
		"ogg":                  append([]byte("OggS"), 0, 2, 0, 0, 0, 0, 0, 0, 0, 0),
		"flac":                 append([]byte("fLaC"), 0, 0, 0, 0x22),
		"adts aac (layer 00)":  {0xFF, 0xF1, 0x50, 0x80},
		"mp2 (layer II)":       {0xFF, 0xFD, 0x90, 0x64},
		"mp3 reserved version": {0xFF, 0xEB, 0x90, 0x64},
		"mp3 free bitrate":     {0xFF, 0xFB, 0x00, 0x64},
		"mp3 bad bitrate":      {0xFF, 0xFB, 0xF0, 0x64},
		"mp3 reserved rate":    {0xFF, 0xFB, 0x9C, 0x64},
		"id3 bad version":      append([]byte("ID3"), 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14),
		"id3 not syncsafe":     append([]byte("ID3"), 0x03, 0x00, 0x00, 0x80, 0x00, 0x00, 0x14),
		"id3 truncated":        append([]byte("ID3"), 0x03, 0x00),
		"midi":                 append([]byte("MThd"), 0, 0, 0, 6),
		"avif":                 ftyp("avif", "avif", "mif1"),
		"mif1 without heic":    ftyp("mif1", "mif1", "avif"),
		"3gp":                  ftyp("3gp4", "3gp4"),
		"pdf not at 0":         []byte("junk%PDF-1.4"),
		"jpeg truncated":       {0xFF, 0xD8},
		"text":                 []byte("hello, world"),
	}
	for name, head := range cases {
		t.Run(name, func(t *testing.T) {
			if mime, _, ok := SniffMIME(head); ok {
				t.Fatalf("SniffMIME accepted %q as %q", name, mime)
			}
		})
	}
}

func TestEveryAllowedTypeHasAKeyExtension(t *testing.T) {
	for mime, ext := range extByMIME {
		if mimeByExt[ext] != mime {
			t.Errorf("ext %q does not map back to %q", ext, mime)
		}
		if !isSegment(ext) {
			t.Errorf("ext %q is not [a-z0-9-]+", ext)
		}
	}
}

// A video must never sniff as audio/mp4: every ordinary video head stays video/mp4 or QuickTime, and
// an M4A brand listed NEXT TO a video brand does not turn a video into audio.
func TestSniffVideoNeverAudio(t *testing.T) {
	cases := map[string][]byte{
		"ffmpeg mp4":             ftyp("isom", "isom", "iso2", "avc1", "mp41"),
		"android mp42":           ftyp("mp42", "isom", "mp42"),
		"M4V":                    ftyp("M4V ", "M4V ", "M4A ", "mp42", "isom"),
		"isom with M4A and avc1": ftyp("isom", "isom", "M4A ", "avc1"),
		"mp42 with M4A and M4V":  ftyp("mp42", "M4A ", "M4V ", "mp42"),
		"iphone mov":             ftyp("qt  ", "qt  "),
		"isom with M4A and qt":   ftyp("isom", "M4A ", "qt  "),
	}
	for name, head := range cases {
		t.Run(name, func(t *testing.T) {
			mime, _, ok := SniffMIME(head)
			if !ok {
				t.Fatalf("SniffMIME refused a video head")
			}
			if mime == MIMEM4A || mime == MIMEMP3 {
				t.Fatalf("SniffMIME = %q: a video sniffed as audio", mime)
			}
		})
	}
}
