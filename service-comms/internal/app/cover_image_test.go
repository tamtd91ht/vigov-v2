package app

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/vihat/vigov/core/storage"
)

// testJPEG encodes a w×h JPEG whose left half is red and right half blue, optionally with an EXIF
// APP1 carrying Orientation — the shape a phone writes.
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
	return insertExifOrientation(out, orientation)
}

// insertExifOrientation puts an APP1 `Exif\0\0` + little-endian TIFF with one IFD0 entry (0x0112)
// right after SOI. It also carries a fake GPS tag value, to show the derivative never keeps EXIF.
func insertExifOrientation(j []byte, o int) []byte {
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

func TestJPEGOrientationIsRead(t *testing.T) {
	for _, o := range []int{1, 3, 6, 8} {
		if got := jpegOrientation(testJPEG(t, 20, 10, o)); got != o {
			t.Errorf("orientation %d read as %d", o, got)
		}
	}
	if got := jpegOrientation(testJPEG(t, 20, 10, 0)); got != 1 {
		t.Errorf("no EXIF must read as 1, got %d", got)
	}
	if got := jpegOrientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0, 0}); got != 1 {
		t.Errorf("a truncated EXIF block must read as 1, got %d", got)
	}
}

func TestDerivativeIsFittedOrientedAndCarriesNoEXIF(t *testing.T) {
	src := testJPEG(t, 2000, 1000, 6) // stored landscape, viewed portrait
	h, err := readCoverHeader(storage.MIMEJPEG, src)
	if err != nil {
		t.Fatal(err)
	}
	img, err := decodeCover(storage.MIMEJPEG, bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderCoverDerivative(img, h.orientation)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("derivative is not a JPEG: %v", err)
	}
	// Orientation 6 turns 2000×1000 upright into 1000×2000; fitted to 1280 on the long side.
	if cfg.Width != 640 || cfg.Height != 1280 {
		t.Errorf("derivative = %d×%d, want 640×1280", cfg.Width, cfg.Height)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS-16.05N")) {
		t.Error("the derivative still carries EXIF / GPS")
	}
	if mime, _, ok := storage.SniffMIME(out); !ok || mime != storage.MIMEJPEG {
		t.Errorf("derivative sniffs as %q", mime)
	}
	// After a 90° clockwise turn the stored LEFT (red) half is the TOP of the upright image.
	dec, _ := jpeg.Decode(bytes.NewReader(out))
	r, _, b, _ := dec.At(320, 100).RGBA()
	if r < b {
		t.Errorf("orientation 6 not applied: the top is not red (r=%d b=%d)", r>>8, b>>8)
	}
}

func TestSmallImageIsNotEnlarged(t *testing.T) {
	if w, h := fitInside(800, 600, coverMaxSide); w != 800 || h != 600 {
		t.Errorf("fitInside enlarged or changed a small image: %d×%d", w, h)
	}
	if w, h := fitInside(5000, 3, coverMaxSide); w != 1280 || h != 1 {
		t.Errorf("a thin panorama must keep at least one row: %d×%d", w, h)
	}
}

func TestTransparentPNGIsFlattenedOntoWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10)) // fully transparent
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	h, err := readCoverHeader(storage.MIMEPNG, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	dec, err := decodeCover(storage.MIMEPNG, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderCoverDerivative(dec, h.orientation)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := jpeg.Decode(bytes.NewReader(out))
	if r, g, b, _ := j.At(5, 5).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("transparent pixel became (%d,%d,%d), want white", r>>8, g>>8, b>>8)
	}
}

// A 1×1 lossless WebP — the policy admits WebP INPUT, which is why golang.org/x/image/webp is here.
const tinyWebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func TestWebPInputDecodes(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(tinyWebP)
	if err != nil {
		t.Fatal(err)
	}
	if mime, _, ok := storage.SniffMIME(data); !ok || mime != storage.MIMEWebP {
		t.Fatalf("sample sniffs as %q", mime)
	}
	h, err := readCoverHeader(storage.MIMEWebP, data)
	if err != nil {
		t.Fatalf("webp header: %v", err)
	}
	img, err := decodeCover(storage.MIMEWebP, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("webp decode: %v", err)
	}
	if _, err := renderCoverDerivative(img, h.orientation); err != nil {
		t.Fatal(err)
	}
}

func TestDecompressionBombIsRefusedFromTheHeader(t *testing.T) {
	// A PNG header declaring 30 000 × 30 000 RGBA: 3.6 GB decoded, a few bytes on the wire. Only the
	// header is read — the budget refuses it before any pixel is allocated.
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	b := buf.Bytes()
	binary.BigEndian.PutUint32(b[16:20], 30000)                        // IHDR width
	binary.BigEndian.PutUint32(b[20:24], 30000)                        // IHDR height
	binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29])) // IHDR CRC over type + data
	h, err := readCoverHeader(storage.MIMEPNG, b)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if decodeCost(storage.MIMEPNG, h) <= coverDecodeBudget {
		t.Errorf("a 900-megapixel image fits the budget: cost %d", decodeCost(storage.MIMEPNG, h))
	}
	// A 48-megapixel baseline phone JPEG must fit.
	phone := coverHeader{cfg: image.Config{Width: 8000, Height: 6000}}
	if decodeCost(storage.MIMEJPEG, phone) > coverDecodeBudget {
		t.Error("a 48 MP baseline JPEG must fit the decode budget")
	}
}

func TestGarbageThatSniffsAsJPEGIsUndecodable(t *testing.T) {
	if _, err := readCoverHeader(storage.MIMEJPEG, []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}); err == nil {
		t.Error("garbage after a JPEG magic must not decode")
	}
}
