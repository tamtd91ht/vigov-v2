package imaging_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/vihat/vigov/core/imaging"
	"github.com/vihat/vigov/core/storage"
)

// What this package promises its two callers (comms cover derivative, petitions scene photo):
// orientation applied, fitted without enlarging, flattened onto white, re-encoded with NO EXIF,
// decompression bombs refused from the header, and the MIME spellings identical to core/storage's.
// The comms suite (service-comms/internal/app/cover_image_test.go) still runs against its thin
// wrappers and is the proof that moving the code here changed none of its output.

func testJPEG(t *testing.T, w, h, orientation int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	if orientation == 0 {
		return out
	}
	return withExifOrientation(out, orientation)
}

// withExifOrientation inserts an APP1 Exif block with Orientation and a fake GPS payload after SOI.
func withExifOrientation(j []byte, o int) []byte {
	tiff := []byte("II*\x00\x08\x00\x00\x00")
	ifd := make([]byte, 2+12+4)
	binary.LittleEndian.PutUint16(ifd[0:2], 1)
	binary.LittleEndian.PutUint16(ifd[2:4], 0x0112)
	binary.LittleEndian.PutUint16(ifd[4:6], 3)
	binary.LittleEndian.PutUint32(ifd[6:10], 1)
	binary.LittleEndian.PutUint16(ifd[10:12], uint16(o))
	payload := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	payload = append(payload, []byte("GPS-16.05N-108.20E")...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:4], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, j[:2]...)
	out = append(out, seg...)
	return append(out, j[2:]...)
}

func TestMIMESpellingsAreCoreStorages(t *testing.T) {
	if imaging.MIMEJPEG != storage.MIMEJPEG || imaging.MIMEPNG != storage.MIMEPNG || imaging.MIMEWebP != storage.MIMEWebP {
		t.Fatal("imaging's MIME constants drifted from core/storage's sniffed types")
	}
}

func TestOrientedFittedAndNoEXIF(t *testing.T) {
	src := testJPEG(t, 2000, 1000, 6)
	h, err := imaging.ReadHeader(imaging.MIMEJPEG, src)
	if err != nil {
		t.Fatal(err)
	}
	if h.Orientation != 6 {
		t.Fatalf("orientation = %d", h.Orientation)
	}
	img, err := imaging.Decode(imaging.MIMEJPEG, bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := imaging.RenderJPEG(img, h.Orientation, 1600, 85)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 800 || cfg.Height != 1600 {
		t.Errorf("rendered %d×%d, want 800×1600", cfg.Width, cfg.Height)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS-16.05N")) {
		t.Error("output still carries EXIF / GPS")
	}
	dec, _ := jpeg.Decode(bytes.NewReader(out))
	if r, _, b, _ := dec.At(400, 100).RGBA(); r < b {
		t.Errorf("orientation 6 not applied (r=%d b=%d)", r>>8, b>>8)
	}
}

func TestSmallImageKeepsItsSize(t *testing.T) {
	if w, h := imaging.FitInside(800, 600, 2560); w != 800 || h != 600 {
		t.Errorf("enlarged: %d×%d", w, h)
	}
	if w, h := imaging.FitInside(5000, 3, 1280); w != 1280 || h != 1 {
		t.Errorf("thin panorama: %d×%d", w, h)
	}
}

func TestPNGFlattenedAndWebPDecodes(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	dec, err := imaging.Decode(imaging.MIMEPNG, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	out, err := imaging.RenderJPEG(dec, 1, 1280, 85)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := jpeg.Decode(bytes.NewReader(out))
	if r, g, b, _ := j.At(5, 5).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("transparent pixel became (%d,%d,%d)", r>>8, g>>8, b>>8)
	}

	webpData, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if _, err := imaging.ReadHeader(imaging.MIMEWebP, webpData); err != nil {
		t.Fatalf("webp header: %v", err)
	}
	img, err := imaging.Decode(imaging.MIMEWebP, bytes.NewReader(webpData))
	if err != nil {
		t.Fatalf("webp decode: %v", err)
	}
	if out, err := imaging.RenderJPEG(img, 1, 1280, 85); err != nil {
		t.Fatal(err)
	} else if mime, _, ok := storage.SniffMIME(out); !ok || mime != storage.MIMEJPEG {
		t.Errorf("WebP input must come out as JPEG, sniffed %q", mime)
	}
}

func TestBombRefusedFromHeaderAndGarbageUndecodable(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	b := buf.Bytes()
	binary.BigEndian.PutUint32(b[16:20], 30000)
	binary.BigEndian.PutUint32(b[20:24], 30000)
	binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
	h, err := imaging.ReadHeader(imaging.MIMEPNG, b)
	if err != nil {
		t.Fatal(err)
	}
	if imaging.DecodeCost(imaging.MIMEPNG, h) <= 160<<20 {
		t.Error("a 900-megapixel PNG must cost more than a 160 MiB budget")
	}
	if _, err := imaging.ReadHeader(imaging.MIMEJPEG, []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}); !errors.Is(err, imaging.ErrUndecodable) {
		t.Errorf("garbage after a JPEG magic: %v", err)
	}
	if _, err := imaging.ReadHeader("image/heic", nil); !errors.Is(err, imaging.ErrUndecodable) {
		t.Errorf("HEIC must be undecodable here: %v", err)
	}
	if _, err := imaging.RenderJPEG(image.NewRGBA(image.Rect(0, 0, 1, 1)), 1, 0, 85); err == nil {
		t.Error("maxSide 0 accepted")
	}
}
