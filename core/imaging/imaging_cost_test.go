package imaging_test

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image/color"
	"runtime"
	"testing"

	"github.com/vihat/vigov/core/imaging"
)

// DecodeCost against what the Go decoders REALLY allocate. Each case crafts a file the standard
// library cannot write (CMYK / YCbCrK / RGB JPEG, progressive JPEG, Adam7 PNG, gray PNG with tRNS),
// decodes it, and measures runtime.MemStats.TotalAlloc across the decode. TotalAlloc is the SUM of
// every allocation, so it bounds the live peak from above: the estimate must be at least that.

// jpegSpec describes a crafted JPEG: every block flat (DC 0, no AC), Huffman tables of one 1-bit code.
type jpegSpec struct {
	w, h        int
	comps       int // 1, 3 or 4
	adobe       int // APP14 transform byte; -1 = no APP14 segment
	progressive bool
	headOnly    bool // stop after SOS: enough for ReadHeader, not decodable
}

func craftJPEG(s jpegSpec) []byte {
	var b bytes.Buffer
	seg := func(marker byte, data []byte) {
		b.Write([]byte{0xFF, marker})
		_ = binary.Write(&b, binary.BigEndian, uint16(len(data)+2))
		b.Write(data)
	}
	b.Write([]byte{0xFF, 0xD8})
	if s.adobe >= 0 {
		seg(0xEE, append([]byte("Adobe"), 0, 100, 0, 0, 0, 0, byte(s.adobe)))
	}
	seg(0xDB, append([]byte{0}, bytes.Repeat([]byte{1}, 64)...))
	sof := []byte{8, byte(s.h >> 8), byte(s.h), byte(s.w >> 8), byte(s.w), byte(s.comps)}
	for c := 1; c <= s.comps; c++ {
		sof = append(sof, byte(c), 0x11, 0)
	}
	if s.progressive {
		seg(0xC2, sof)
	} else {
		seg(0xC0, sof)
	}
	one := append([]byte{1}, make([]byte, 15)...)         // one code of length 1 …
	seg(0xC4, append(append([]byte{0x00}, one...), 0x00)) // … DC: category 0
	seg(0xC4, append(append([]byte{0x10}, one...), 0x00)) // … AC: EOB
	sos := []byte{byte(s.comps)}
	for c := 1; c <= s.comps; c++ {
		sos = append(sos, byte(c), 0x00)
	}
	if s.progressive {
		sos = append(sos, 0, 0, 0) // DC-first scan, all components interleaved
	} else {
		sos = append(sos, 0, 63, 0)
	}
	seg(0xDA, sos)
	if s.headOnly {
		return b.Bytes()
	}
	blocks := ((s.w + 7) / 8) * ((s.h + 7) / 8)
	perBlock := 2 // DC code + EOB
	if s.progressive {
		perBlock = 1 // DC code only
	}
	bits := blocks * s.comps * perBlock
	b.Write(make([]byte, bits/8))
	if bits%8 != 0 {
		b.WriteByte(byte(0xFF >> (bits % 8))) // pad with 1-bits
	}
	b.Write([]byte{0xFF, 0xD9})
	return b.Bytes()
}

// pngSpec describes a crafted all-zero PNG.
type pngSpec struct {
	w, h       int
	colorType  byte // 0 gray · 2 RGB · 6 RGBA
	interlaced bool
	trns       bool
	padBefore  int // a tEXt chunk this long before IDAT (pushes IDAT out of the head)
}

func craftPNG(t *testing.T, s pngSpec) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(typ string, data []byte) {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
		td := append([]byte(typ), data...)
		b.Write(td)
		_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(td))
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(s.w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(s.h))
	ihdr[8], ihdr[9] = 8, s.colorType
	if s.interlaced {
		ihdr[12] = 1
	}
	chunk("IHDR", ihdr)
	if s.padBefore > 0 {
		chunk("tEXt", append([]byte("Comment\x00"), bytes.Repeat([]byte("x"), s.padBefore)...))
	}
	bpp := map[byte]int{0: 1, 2: 3, 6: 4}[s.colorType]
	if s.trns {
		chunk("tRNS", make([]byte, map[byte]int{0: 2, 2: 6}[s.colorType]))
	}
	var raw bytes.Buffer
	rows := func(w, h int) {
		if w == 0 || h == 0 {
			return
		}
		for y := 0; y < h; y++ {
			raw.Write(make([]byte, 1+w*bpp))
		}
	}
	if s.interlaced {
		for _, p := range [][4]int{{8, 8, 0, 0}, {8, 8, 4, 0}, {4, 8, 0, 4}, {4, 4, 2, 0}, {2, 4, 0, 2}, {2, 2, 1, 0}, {1, 2, 0, 1}} {
			rows((s.w-p[2]+p[0]-1)/p[0], (s.h-p[3]+p[1]-1)/p[1])
		}
	} else {
		rows(s.w, s.h)
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	chunk("IDAT", z.Bytes())
	chunk("IEND", nil)
	return b.Bytes()
}

// measureDecode decodes data and returns the bytes allocated during the decode.
func measureDecode(t *testing.T, mime string, data []byte) uint64 {
	t.Helper()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	img, err := imaging.Decode(mime, bytes.NewReader(data))
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("crafted image does not decode: %v", err)
	}
	runtime.KeepAlive(img)
	return after.TotalAlloc - before.TotalAlloc
}

func TestDecodeCostCoversMeasuredJPEGAllocation(t *testing.T) {
	const w, h = 1536, 1536 // multiples of 32: the MCU rounding adds nothing, so B/px is exact
	px := float64(w * h)
	for _, c := range []struct {
		name  string
		spec  jpegSpec
		model color.Model
		// floor is the measured B/px the case must at least reach — proof the crafted file really
		// takes the decoder path named, not a cheaper one.
		floor float64
	}{
		{"YCbCr baseline", jpegSpec{w: w, h: h, comps: 3, adobe: -1}, color.YCbCrModel, 2.9},
		{"gray baseline", jpegSpec{w: w, h: h, comps: 1, adobe: -1}, color.GrayModel, 0.9},
		{"RGB (Adobe 0) baseline", jpegSpec{w: w, h: h, comps: 3, adobe: 0}, color.RGBAModel, 6.9},
		{"CMYK (Adobe 0) baseline", jpegSpec{w: w, h: h, comps: 4, adobe: 0}, color.CMYKModel, 7.9},
		{"YCbCrK (Adobe 2) baseline", jpegSpec{w: w, h: h, comps: 4, adobe: 2}, color.CMYKModel, 7.9},
		{"YCbCr progressive", jpegSpec{w: w, h: h, comps: 3, adobe: -1, progressive: true}, color.YCbCrModel, 14.9},
		{"CMYK progressive", jpegSpec{w: w, h: h, comps: 4, adobe: 0, progressive: true}, color.CMYKModel, 23.9},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := craftJPEG(c.spec)
			hd, err := imaging.ReadHeader(imaging.MIMEJPEG, data)
			if err != nil {
				t.Fatal(err)
			}
			if hd.Config.ColorModel != c.model || hd.Progressive != c.spec.progressive {
				t.Fatalf("header model %T progressive %v — the crafted file is not the case named",
					hd.Config.ColorModel, hd.Progressive)
			}
			got := measureDecode(t, imaging.MIMEJPEG, data)
			est := imaging.DecodeCost(imaging.MIMEJPEG, hd)
			t.Logf("file %d B · measured %d B (%.2f B/px) · estimated %d B (%.2f B/px)",
				len(data), got, float64(got)/px, est, float64(est)/px)
			if float64(got)/px < c.floor {
				t.Errorf("measured %.2f B/px, expected at least %.1f", float64(got)/px, c.floor)
			}
			if est < int64(got) {
				t.Errorf("DecodeCost %d < allocated %d: the estimate admits an image that costs more", est, got)
			}
		})
	}
}

func TestDecodeCostCoversMeasuredPNGAllocation(t *testing.T) {
	const w, h = 1024, 1024
	px := float64(w * h)
	for _, c := range []struct {
		name  string
		spec  pngSpec
		floor float64
	}{
		{"gray", pngSpec{w: w, h: h, colorType: 0}, 0.9},
		{"gray with tRNS decodes to NRGBA", pngSpec{w: w, h: h, colorType: 0, trns: true}, 3.9},
		{"RGBA", pngSpec{w: w, h: h, colorType: 6}, 3.9},
		{"RGBA Adam7", pngSpec{w: w, h: h, colorType: 6, interlaced: true}, 7.9},
		{"gray Adam7 with tRNS", pngSpec{w: w, h: h, colorType: 0, interlaced: true, trns: true}, 7.9},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := craftPNG(t, c.spec)
			hd, err := imaging.ReadHeader(imaging.MIMEPNG, data)
			if err != nil {
				t.Fatal(err)
			}
			got := measureDecode(t, imaging.MIMEPNG, data)
			est := imaging.DecodeCost(imaging.MIMEPNG, hd)
			t.Logf("file %d B · measured %d B (%.2f B/px) · estimated %d B (%.2f B/px)",
				len(data), got, float64(got)/px, est, float64(est)/px)
			if float64(got)/px < c.floor {
				t.Errorf("measured %.2f B/px, expected at least %.1f", float64(got)/px, c.floor)
			}
			if est < int64(got) {
				t.Errorf("DecodeCost %d < allocated %d", est, got)
			}
		})
	}
}

// The finding as it was reported: a flat 7300×7300 CMYK JPEG is under 1 MB, passed the 160 MiB
// budget at 3 B/px, and allocates ~8 B/px. Now refused from the header alone.
func TestLargeCMYKJPEGRefusedFromHeader(t *testing.T) {
	const budget = 160 << 20
	full := craftJPEG(jpegSpec{w: 7300, h: 7300, comps: 4, adobe: 0})
	if len(full) >= 10<<20 {
		t.Fatalf("the crafted file is %d B — the attack needs it under the 10 MB upload limit", len(full))
	}
	hd, err := imaging.ReadHeader(imaging.MIMEJPEG, full[:min(len(full), imaging.HeadBytes)])
	if err != nil {
		t.Fatal(err)
	}
	if old := int64(7300) * 7300 * 3; old > budget {
		t.Fatalf("precondition: the old 3 B/px estimate (%d) was supposed to pass the budget", old)
	}
	if est := imaging.DecodeCost(imaging.MIMEJPEG, hd); est <= budget {
		t.Errorf("7300×7300 CMYK estimated %d B, within the %d budget", est, budget)
	}
	// A 48 MP phone photo (baseline YCbCr) still fits, so the fix refuses nothing a phone produces.
	phone, err := imaging.ReadHeader(imaging.MIMEJPEG, craftJPEG(jpegSpec{w: 8000, h: 6000, comps: 3, adobe: -1, headOnly: true}))
	if err != nil {
		t.Fatal(err)
	}
	if est := imaging.DecodeCost(imaging.MIMEJPEG, phone); est > budget {
		t.Errorf("a 48 MP baseline JPEG is estimated %d B, above the budget", est)
	}
}

// What the head cannot prove is charged as the worst case: interlaced + transparent PNG, extended WebP.
func TestUnprovedLayoutIsTheWorstCase(t *testing.T) {
	// IDAT pushed past the head by a large text chunk: a tRNS might follow, so gray is charged as NRGBA.
	data := craftPNG(t, pngSpec{w: 100, h: 100, colorType: 0, padBefore: imaging.HeadBytes})
	hd, err := imaging.ReadHeader(imaging.MIMEPNG, data[:imaging.HeadBytes])
	if err != nil {
		t.Fatal(err)
	}
	// Every figure below adds DecodeCost's fixed 1 MiB and, for PNG, its two filter rows.
	const overhead, pngRows = 1 << 20, 2 * (1 + 8*100)
	if got, want := imaging.DecodeCost(imaging.MIMEPNG, hd), int64(100*100*4+overhead+pngRows); got != want {
		t.Errorf("gray PNG with IDAT beyond the head: cost %d, want %d (NRGBA, not interlaced)", got, want)
	}
	proved := craftPNG(t, pngSpec{w: 100, h: 100, colorType: 0})
	if hd, _ = imaging.ReadHeader(imaging.MIMEPNG, proved); imaging.DecodeCost(imaging.MIMEPNG, hd) != 100*100+overhead+pngRows {
		t.Errorf("plain gray PNG: cost %d, want 1 B/px", imaging.DecodeCost(imaging.MIMEPNG, hd))
	}
	// A Header built by hand never proved anything.
	byHand := imaging.Header{Config: hd.Config}
	if got, want := imaging.DecodeCost(imaging.MIMEPNG, byHand), int64(100*100*4*2+overhead+pngRows); got != want {
		t.Errorf("hand-built gray PNG header: cost %d, want %d", got, want)
	}
	if got := imaging.DecodeCost(imaging.MIMEWebP, imaging.Header{Config: hd.Config}); got != 100*100*12+overhead {
		t.Errorf("hand-built WebP header: cost %d, want 12 B/px", got)
	}
	// A real lossless file is charged 8 B/px, not the 12 of the unproved case.
	lossless, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	wh, err := imaging.ReadHeader(imaging.MIMEWebP, lossless)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := imaging.DecodeCost(imaging.MIMEWebP, wh), int64(wh.Config.Width*wh.Config.Height*8+overhead); got != want {
		t.Errorf("VP8L WebP: cost %d, want %d (8 B/px)", got, want)
	}
}
