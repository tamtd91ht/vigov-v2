package imaging_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/vihat/vigov/core/imaging"
)

// The two renderers service-platform uses for the commune's identity images (ADR 0069 #4, #5). What
// they promise: the logo keeps its transparency (the defect the ADR names is a transparent logo turned
// into a white box), is always exactly square, is padded rather than cropped; the banner is exactly the
// requested width with its aspect kept, and never taller than the ceiling.

// transparentLogo is a w×h PNG whose left half is fully transparent and right half opaque red.
func transparentLogo(t *testing.T, w, h int) image.Image {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= w/2 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	dec, err := imaging.Decode(imaging.MIMEPNG, &buf)
	if err != nil {
		t.Fatal(err)
	}
	return dec
}

func TestRenderPNGSquareKeepsAlphaAndIsExactlySquare(t *testing.T) {
	out, err := imaging.RenderPNGSquare(transparentLogo(t, 300, 300), 1, 512)
	if err != nil {
		t.Fatal(err)
	}
	if mime, _, ok := sniffPNG(out); !ok {
		t.Fatalf("output is not a PNG: %q", mime)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output does not decode as PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("size = %dx%d, want 512x512 (enlarged to the one size every consumer gets)", b.Dx(), b.Dy())
	}
	// THE ASSERTION ADR 0069 EXISTS FOR: the transparent half stays transparent.
	if _, _, _, a := img.At(10, 256).RGBA(); a != 0 {
		t.Errorf("left (transparent) pixel alpha = %d, want 0 — the logo was flattened", a)
	}
	if r, _, _, a := img.At(500, 256).RGBA(); a != 0xffff || r < 0xf000 {
		t.Errorf("right (opaque red) pixel = r%d a%d, want opaque red", r, a)
	}
}

func TestRenderPNGSquarePadsAWideLogoInsteadOfCropping(t *testing.T) {
	// 400×100 opaque: fitted to 512×128, centred vertically; rows above and below are transparent.
	src := image.NewNRGBA(image.Rect(0, 0, 400, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 400; x++ {
			src.SetNRGBA(x, y, color.NRGBA{G: 200, A: 255})
		}
	}
	out, err := imaging.RenderPNGSquare(src, 1, 512)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := img.At(256, 5).RGBA(); a != 0 {
		t.Errorf("padding row alpha = %d, want 0", a)
	}
	if _, _, _, a := img.At(256, 256).RGBA(); a != 0xffff {
		t.Errorf("centre pixel alpha = %d, want opaque (the logo itself)", a)
	}
	if _, _, _, a := img.At(1, 256).RGBA(); a != 0xffff {
		t.Errorf("left edge of the logo row alpha = %d — the logo was cropped or shrunk", a)
	}
}

func TestRenderJPEGWidthIsExactlyTheWidthAndKeepsAspect(t *testing.T) {
	for _, tc := range []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"wide, scaled down", 3200, 400, 1600, 200},
		{"narrow, enlarged", 800, 100, 1600, 200},
		{"taller than the ceiling: fitted inside 1600 x 1600", 1000, 4000, 400, 1600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))
			out, err := imaging.RenderJPEGWidth(src, 1, 1600, 1600, 85)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("output is not a JPEG: %v", err)
			}
			if cfg.Width != tc.wantW || cfg.Height != tc.wantH {
				t.Errorf("size = %dx%d, want %dx%d", cfg.Width, cfg.Height, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestBrandingRenderersRefuseNonsense(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if _, err := imaging.RenderPNGSquare(src, 1, 0); err == nil {
		t.Error("side 0 accepted")
	}
	if _, err := imaging.RenderJPEGWidth(src, 1, 1600, 0, 85); err == nil {
		t.Error("maxHeight 0 accepted")
	}
	if _, err := imaging.RenderPNGSquare(image.NewRGBA(image.Rect(0, 0, 0, 0)), 1, 512); err == nil {
		t.Error("empty image accepted")
	}
}

// sniffPNG reports whether b starts with the PNG signature.
func sniffPNG(b []byte) (string, string, bool) {
	if len(b) < 8 || string(b[:8]) != "\x89PNG\r\n\x1a\n" {
		return "", "", false
	}
	return imaging.MIMEPNG, "png", true
}
