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
		"empty":             nil,
		"html":              []byte("<!DOCTYPE html><html><script>alert(1)</script>"),
		"html leading ws":   []byte("  \n<html><body>"),
		"svg":               []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)">`),
		"svg bare":          []byte(`<svg xmlns="http://www.w3.org/2000/svg">`),
		"exe":               []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff"),
		"elf":               []byte("\x7fELF\x02\x01\x01\x00"),
		"zip":               []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"),
		"zip named .jpg":    []byte("PK\x03\x04\x14\x00\x06\x00"),
		"docx (zip)":        []byte("PK\x03\x04\x14\x00\x06\x00\x08\x00\x00\x00!\x00"),
		"gif":               []byte("GIF89a\x01\x00\x01\x00"),
		"riff wav":          []byte("RIFF\x24\x00\x00\x00WAVEfmt "),
		"m4a audio":         ftyp("M4A ", "M4A ", "mp42", "isom"),
		"avif":              ftyp("avif", "avif", "mif1"),
		"mif1 without heic": ftyp("mif1", "mif1", "avif"),
		"3gp":               ftyp("3gp4", "3gp4"),
		"pdf not at 0":      []byte("junk%PDF-1.4"),
		"jpeg truncated":    {0xFF, 0xD8},
		"text":              []byte("hello, world"),
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
