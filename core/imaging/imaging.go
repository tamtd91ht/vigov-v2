// Package imaging decodes an uploaded image, turns it upright by its EXIF Orientation, fits it
// inside a square and re-encodes it as a JPEG that carries NO metadata at all.
//
// TWO FLOWS USE IT, in two services, which is why it lives in core and not in either service's
// internal/ (rule 2, forbidden #1 — a service never imports another's internal package):
//
//	comms      news cover derivative `thumb-1280` (ADR 0052 "Bổ sung 30/09/2026 (lần hai)" (a))
//	petitions  a citizen's scene photo, stored as the clean `original` (ADR 0047 G3, ADR 0052 (b))
//
// The code was written for the first flow (service-comms/internal/app/cover_image.go) and moved
// here unchanged on 2026-10-02 when the second one arrived; comms now calls it with its own two
// numbers (1280 px, quality 85), so its derivatives are byte-identical to what it produced before.
//
// THE RE-ENCODE IS WHAT STRIPS EXIF. image/jpeg writes no APP1 segment, so GPS coordinates, the
// device serial and the capture time of the upload never reach the output. That is the whole
// reason a citizen photo goes through here (rule 3: the raw upload carries the coordinates of the
// citizen's home).
//
// JPEG AND NOT WebP FOR THE OUTPUT: the Go standard library encodes JPEG, and golang.org/x/image
// decodes WebP but has no encoder. A WebP output would mean cgo or a second, unvetted dependency.
//
// golang.org/x/image is used for two things the standard library lacks: the WebP DECODER (the
// platform policies admit WebP input) and a proper downscaling kernel (x/image/draw).
//
// THIS PACKAGE DOES NOT LOG, and an error from it never carries image content or a file name.
package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// The three input types this package decodes. They are the strings core/storage sniffs
// (storage.MIMEJPEG …); spelled here rather than imported so this package depends on nothing in
// core — the values are pinned against storage's by imaging_test.go.
const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEWebP = "image/webp"
)

// HeadBytes is how much of an image ReadHeader needs to find the dimensions, the EXIF orientation
// and the JPEG frame type. EXIF lives in one APP1 segment of at most 64 KiB right after SOI.
const HeadBytes = 512 << 10

// ErrUndecodable: the bytes sniffed as an allowed type but do not decode as one, or the type is
// not one this package decodes.
var ErrUndecodable = errors.New("imaging: image cannot be decoded")

// Header is what the first HeadBytes of an image say about it.
type Header struct {
	Config      image.Config
	Orientation int // EXIF 1..8; 1 when absent
	Progressive bool
}

// ReadHeader inspects the head of an image of sniffed type mime.
func ReadHeader(mime string, head []byte) (Header, error) {
	var (
		h   = Header{Orientation: 1}
		err error
	)
	switch mime {
	case MIMEJPEG:
		h.Config, err = jpeg.DecodeConfig(bytes.NewReader(head))
		h.Orientation = JPEGOrientation(head)
		h.Progressive = jpegProgressive(head)
	case MIMEPNG:
		h.Config, err = png.DecodeConfig(bytes.NewReader(head))
	case MIMEWebP:
		h.Config, err = webp.DecodeConfig(bytes.NewReader(head))
	default:
		return Header{}, ErrUndecodable
	}
	if err != nil || h.Config.Width <= 0 || h.Config.Height <= 0 {
		return Header{}, ErrUndecodable
	}
	return h, nil
}

// DecodeCost estimates the bytes a full decode allocates. Conservative by design: the case it gets
// wrong must be a refusal of a decodable image, never an out-of-memory kill of the pod. The caller
// compares it with its own budget, which depends on its pod's memory limit.
//
//	JPEG baseline     ≤ 3 B/px (YCbCr 4:4:4; 4:2:0 is 1.5)
//	JPEG progressive  + 12 B/px: image/jpeg keeps every coefficient as int32 until the last scan
//	PNG               by colour model: 1 (gray, paletted) · 2 (gray16) · 4 (RGBA) · 8 (RGBA64)
//	WebP              4 B/px (lossless decodes to NRGBA; lossy YCbCr+alpha is ≤ 2.5)
func DecodeCost(mime string, h Header) int64 {
	px := int64(h.Config.Width) * int64(h.Config.Height)
	per := int64(8)
	switch mime {
	case MIMEJPEG:
		per = 3
		if h.Progressive {
			per += 12
		}
	case MIMEPNG:
		switch h.Config.ColorModel {
		case color.GrayModel:
			per = 1
		case color.Gray16Model:
			per = 2
		case color.RGBAModel, color.NRGBAModel:
			per = 4
		default:
			if _, ok := h.Config.ColorModel.(color.Palette); ok {
				per = 1
			} else {
				per = 8
			}
		}
	case MIMEWebP:
		per = 4
	}
	return px * per
}

// Decode decodes a full image of sniffed type mime. A read error from r stays in the chain
// (errors.Is), so a caller can tell "the object changed under me" from "not an image".
func Decode(mime string, r io.Reader) (image.Image, error) {
	var (
		img image.Image
		err error
	)
	switch mime {
	case MIMEJPEG:
		img, err = jpeg.Decode(r)
	case MIMEPNG:
		img, err = png.Decode(r)
	case MIMEWebP:
		img, err = webp.Decode(r)
	default:
		return nil, ErrUndecodable
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUndecodable, err)
	}
	return img, nil
}

// RenderJPEG orients, fits inside maxSide × maxSide (never enlarged), flattens onto white and
// re-encodes a decoded image as a JPEG of the given quality, with no metadata.
func RenderJPEG(src image.Image, orientation, maxSide, quality int) ([]byte, error) {
	if maxSide <= 0 || quality < 1 || quality > 100 {
		return nil, fmt.Errorf("imaging: maxSide %d / quality %d out of range", maxSide, quality)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, ErrUndecodable
	}
	// The SCALED size is computed in the ORIENTED frame, so the long side is the long side a reader sees.
	ow, oh := w, h
	if orientation >= 5 && orientation <= 8 {
		ow, oh = h, w
	}
	tw, th := FitInside(ow, oh, maxSide)
	// Scale in the stored frame (dimensions swapped back), then rotate the small result: rotating
	// first would walk every pixel of a 48-megapixel image a second time.
	sw, sh := tw, th
	if orientation >= 5 && orientation <= 8 {
		sw, sh = th, tw
	}
	// White first, then the source OVER it: JPEG has no alpha, and a transparent PNG flattened onto
	// black is a black rectangle on a reader's screen.
	scaled := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(scaled, scaled.Bounds(), image.White, image.Point{}, draw.Src)
	draw.BiLinear.Scale(scaled, scaled.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, orient(scaled, orientation), &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("imaging: encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// FitInside returns w×h scaled down so the longer side is at most max, never scaled up, never 0.
func FitInside(w, h, max int) (int, int) {
	if w <= max && h <= max {
		return w, h
	}
	if w >= h {
		nh := int(int64(h) * int64(max) / int64(w))
		if nh < 1 {
			nh = 1
		}
		return max, nh
	}
	nw := int(int64(w) * int64(max) / int64(h))
	if nw < 1 {
		nw = 1
	}
	return nw, max
}

// orient applies EXIF Orientation 2..8 to img; 1 (or anything unknown) returns it unchanged.
//
// The eight values (TIFF 6.0 / EXIF 2.3, tag 0x0112) say where row 0 and column 0 of the STORED image
// sit when viewed upright. dst(x, y) below is read from the stored image at the mapped point.
func orient(img *image.RGBA, o int) image.Image {
	if o < 2 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch o {
			case 2: // mirrored horizontally
				sx, sy = w-1-x, y
			case 3: // rotated 180°
				sx, sy = w-1-x, h-1-y
			case 4: // mirrored vertically
				sx, sy = x, h-1-y
			case 5: // transposed
				sx, sy = y, x
			case 6: // rotated 90° clockwise to view
				sx, sy = y, h-1-x
			case 7: // transverse
				sx, sy = w-1-y, h-1-x
			case 8: // rotated 90° counter-clockwise to view
				sx, sy = w-1-y, x
			}
			dst.SetRGBA(x, y, img.RGBAAt(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

// JPEGOrientation reads EXIF Orientation from the APP1 segment of a JPEG head; 1 when there is none or
// it cannot be read. A malformed EXIF block is NOT an error: the image is still decodable, and the
// worst outcome of ignoring it is a sideways photo.
func JPEGOrientation(head []byte) int {
	for _, seg := range jpegSegments(head) {
		if seg.marker != 0xE1 || len(seg.data) < 14 || string(seg.data[:6]) != "Exif\x00\x00" {
			continue
		}
		if o := tiffOrientation(seg.data[6:]); o >= 1 && o <= 8 {
			return o
		}
		return 1
	}
	return 1
}

// jpegProgressive reports whether the frame is progressive (SOF2) — what makes DecodeCost add the
// coefficient buffers image/jpeg keeps for it.
func jpegProgressive(head []byte) bool {
	for _, seg := range jpegSegments(head) {
		switch seg.marker {
		case 0xC2, 0xC6, 0xCA, 0xCE: // SOF2, SOF6, SOF10, SOF14: the progressive frames
			return true
		case 0xC0, 0xC1, 0xC3:
			return false
		}
	}
	return false
}

type jpegSegment struct {
	marker byte
	data   []byte
}

// jpegSegments walks the marker segments of a JPEG head up to the first SOS (or the end of the head).
func jpegSegments(head []byte) []jpegSegment {
	if len(head) < 4 || head[0] != 0xFF || head[1] != 0xD8 {
		return nil
	}
	var out []jpegSegment
	i := 2
	for i+4 <= len(head) {
		if head[i] != 0xFF {
			return out
		}
		m := head[i+1]
		if m == 0xFF { // fill byte
			i++
			continue
		}
		if m == 0xD9 || m == 0xDA { // EOI, SOS: no more header segments
			return out
		}
		n := int(binary.BigEndian.Uint16(head[i+2 : i+4]))
		if n < 2 || i+2+n > len(head) {
			return out
		}
		out = append(out, jpegSegment{marker: m, data: head[i+4 : i+2+n]})
		i += 2 + n
	}
	return out
}

// tiffOrientation reads tag 0x0112 from IFD0 of a TIFF block (the EXIF payload); 0 when absent.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(t[2:4]) != 42 {
		return 0
	}
	off := int(bo.Uint32(t[4:8]))
	if off < 8 || off+2 > len(t) {
		return 0
	}
	count := int(bo.Uint16(t[off : off+2]))
	for k := 0; k < count; k++ {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 0
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 && bo.Uint16(t[e+2:e+4]) == 3 { // SHORT
			return int(bo.Uint16(t[e+8 : e+10]))
		}
	}
	return 0
}
