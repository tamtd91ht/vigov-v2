// Package imaging decodes an uploaded image, turns it upright by its EXIF Orientation, fits it
// inside a square and re-encodes it as a JPEG that carries NO metadata at all.
//
// THREE SERVICES USE IT, which is why it lives in core and not in any service's internal/ (rule 2,
// forbidden #1 — a service never imports another's internal package):
//
//	comms      news cover derivative `thumb-1280` (ADR 0052 "Bổ sung 30/09/2026 (lần hai)" (a))
//	petitions  a citizen's scene photo, stored as the clean `original` (ADR 0047 G3, ADR 0052 (b))
//	platform   the commune logo `thumb-512.png` (RenderPNGSquare, alpha KEPT) and the web-admin
//	           banner `thumb-1600.jpg` (RenderJPEGWidth) — ADR 0069 #4, #5
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

	// layout is what ReadHeader PROVED from the head about the container: PNG interlace and
	// transparency, the WebP chunk kind. Unexported on purpose, and its zero value is the WORST case:
	// a Header built by hand (service-comms rebuilds one from its own three fields) cannot claim a
	// cheap layout it never read, so DecodeCost charges it the most expensive decode the type allows.
	layout layout
}

// layout: see Header.layout. Every field's zero value is the expensive answer.
type layout struct {
	proved bool // the fields below were read from the head; false = assume the worst

	pngInterlaced  bool // IHDR interlace method 1 (Adam7)
	pngTransparent bool // a tRNS chunk before IDAT, OR the head ended before IDAT (not provable)

	webpSimpleLossy bool // the first chunk is `VP8 ` — lossy, no alpha, no extended header
	webpLossless    bool // the first chunk is `VP8L`
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
		h.layout = pngLayout(head)
	case MIMEWebP:
		h.Config, err = webp.DecodeConfig(bytes.NewReader(head))
		h.layout = webpLayout(head)
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
// Each figure is what the Go decoder ALLOCATES (go1.26 image/jpeg, image/png, x/image v0.46 webp),
// measured on crafted files by imaging_cost_test.go — not what the pixel format suggests. Until
// 2026-10-02 JPEG was 3 B/px whatever its colour model: a flat 7300×7300 CMYK JPEG of ~0.8 MB passed
// a 160 MiB budget at ~160 MB and allocates 8.01 B/px measured (~427 MB), above the 384 MiB pod limit.
//
//	JPEG gray / YCbCr      3 B/px   one YCbCr (≤ 4:4:4) or Gray plane set
//	JPEG RGB (Adobe 0)     7 B/px   the YCbCr planes + the RGBA it is converted into (reader.go convertToRGB)
//	JPEG CMYK / YCbCrK     8 B/px   the YCbCr planes + the K plane (scan.go:45) + the CMYK/RGBA output
//	                                (reader.go applyBlack)
//	JPEG progressive       + 4 B/px PER COMPONENT: every coefficient kept as int32 until the last
//	                                scan (scan.go:156) — +12 for 3 components, +16 for 4
//	                       dimensions rounded up to a 32-px MCU, the size image/jpeg really allocates
//	PNG                    by colour model 1 · 2 · 4 · 8, where a tRNS chunk promotes gray to NRGBA
//	                       (4) and gray16 to NRGBA64 (8) although DecodeConfig still says Gray;
//	                       ×2 when Adam7-interlaced (the full image plus every pass, reader.go:377-387)
//	WebP `VP8 ` lossy      4 B/px   YCbCr 4:2:0 is 1.5; kept at the old, generous figure
//	WebP `VP8L` lossless   8 B/px   ARGB pixels + the colour-indexing transform's second buffer
//	WebP `VP8X` extended  12 B/px   may carry a VP8L-coded alpha plane (the same 8) on top of a lossy
//	                                YCbCr image and the alpha copy
//
// A layout the head did not prove (Header.layout zero) is charged as interlaced + transparent PNG
// and as extended WebP.
//
// On top of the pixels: decodeOverhead for the decoder's own state (measured ~20 KB JPEG, ~48 KB PNG
// at 1024 px wide), and for PNG the two filter rows image/png keeps (2 × (1 + width × 8)), which grow
// with the width alone and so dominate a one-row image of enormous width.
func DecodeCost(mime string, h Header) int64 {
	w, ht := int64(h.Config.Width), int64(h.Config.Height)
	per, extra := int64(8), int64(decodeOverhead)
	switch mime {
	case MIMEJPEG:
		w, ht = roundUp(w, 32), roundUp(ht, 32)
		comps := int64(3)
		switch h.Config.ColorModel {
		case nil, color.GrayModel, color.YCbCrModel:
			// nil only from a Header built by hand: jpeg.DecodeConfig reports exactly these four models.
			per = 3
		case color.RGBAModel:
			per = 7
		default: // color.CMYKModel, and anything unrecognised is charged as the most expensive one
			per, comps = 8, 4
		}
		if h.Progressive {
			per += 4 * comps
		}
	case MIMEPNG:
		l := h.layout
		transparent := !l.proved || l.pngTransparent
		switch h.Config.ColorModel {
		case color.GrayModel:
			per = 1
			if transparent {
				per = 4
			}
		case color.Gray16Model:
			per = 2
			if transparent {
				per = 8
			}
		case color.RGBAModel, color.NRGBAModel:
			per = 4
		default:
			if _, ok := h.Config.ColorModel.(color.Palette); ok {
				per = 1
			} else {
				per = 8
			}
		}
		if !l.proved || l.pngInterlaced {
			per *= 2
		}
		extra += 2 * (1 + 8*w)
	case MIMEWebP:
		switch l := h.layout; {
		case l.proved && l.webpSimpleLossy:
			per = 4
		case l.proved && l.webpLossless:
			per = 8
		default:
			per = 12
		}
	}
	return w*ht*per + extra
}

// decodeOverhead: see DecodeCost. 1 MiB is ~20× the measured fixed cost — generous because it is
// negligible against any budget a caller would set, and the error it absorbs is the dangerous kind.
const decodeOverhead = 1 << 20

func roundUp(n, m int64) int64 { return (n + m - 1) / m * m }

// pngLayout reads the IHDR interlace byte and looks for tRNS among the chunks before IDAT. A head that
// ends before IDAT cannot prove there is no tRNS, so it reports one.
func pngLayout(head []byte) layout {
	const ihdrInterlace = 8 + 8 + 12 // signature · length+type · width,height,depth,colour,compression,filter
	if len(head) <= ihdrInterlace || string(head[12:16]) != "IHDR" {
		return layout{}
	}
	l := layout{proved: true, pngInterlaced: head[ihdrInterlace] != 0, pngTransparent: true}
	for off := 8; off+8 <= len(head); {
		n := int64(binary.BigEndian.Uint32(head[off : off+4]))
		switch string(head[off+4 : off+8]) {
		case "tRNS":
			return l
		case "IDAT":
			l.pngTransparent = false
			return l
		}
		next := int64(off) + 12 + n
		if next > int64(len(head)) {
			break
		}
		off = int(next)
	}
	return l
}

// webpLayout reads the FourCC of the first chunk after `RIFF....WEBP`.
func webpLayout(head []byte) layout {
	if len(head) < 16 || string(head[0:4]) != "RIFF" || string(head[8:12]) != "WEBP" {
		return layout{}
	}
	switch string(head[12:16]) {
	case "VP8 ":
		return layout{proved: true, webpSimpleLossy: true}
	case "VP8L":
		return layout{proved: true, webpLossless: true}
	}
	return layout{proved: true} // VP8X or unknown: the expensive case
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

// RenderPNGSquare orients a decoded image, scales it so its LONGER side is exactly side (up or
// down), centres it on a fully TRANSPARENT side × side canvas and encodes it as a PNG with no
// metadata. The commune logo (service-platform, ADR 0069 #4: "PNG vuông 512px giữ nền trong").
//
// WHY A SEPARATE RENDERER AND NOT A FLAG ON RenderJPEG: a logo with a transparent background
// flattened onto white is a white box on the web-admin sidebar — the defect ADR 0069 names. JPEG
// has no alpha channel, so the only fix is a different encoder; the flatten step is skipped here.
//
// PADDED, NEVER CROPPED: a seal or emblem cut at its edge is an official symbol shown wrong (an
// incident for a public authority), while transparent padding is invisible on every background.
// ENLARGED when smaller than side, so every consumer receives one size; a blurry upscale is visible
// to the commune that uploaded it and is fixed by uploading a larger image.
//
// image/png writes no tEXt / iTXt / eXIf chunk, so nothing of the upload's metadata survives.
func RenderPNGSquare(src image.Image, orientation, side int) ([]byte, error) {
	if side <= 0 {
		return nil, fmt.Errorf("imaging: side %d out of range", side)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, ErrUndecodable
	}
	ow, oh := w, h
	if orientation >= 5 && orientation <= 8 {
		ow, oh = h, w
	}
	tw, th := scaleLongerTo(ow, oh, side)
	sw, sh := tw, th
	if orientation >= 5 && orientation <= 8 {
		sw, sh = th, tw
	}
	// draw.Src onto a zeroed RGBA keeps every source alpha value: nothing is composited underneath.
	scaled := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.BiLinear.Scale(scaled, scaled.Bounds(), src, b, draw.Src, nil)
	upright := orient(scaled, orientation)

	canvas := image.NewRGBA(image.Rect(0, 0, side, side)) // zero value = fully transparent
	off := image.Pt((side-tw)/2, (side-th)/2)
	draw.Draw(canvas, image.Rectangle{Min: off, Max: off.Add(image.Pt(tw, th))}, upright,
		upright.Bounds().Min, draw.Src)

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("imaging: encode PNG: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderJPEGWidth orients a decoded image, scales it to EXACTLY width pixels wide (up or down,
// aspect kept), flattens it onto white and re-encodes it as a JPEG with no metadata. The web-admin
// banner (service-platform, ADR 0069 #5: "chuẩn hoá về rộng 1600px").
//
// THE ONE BOUND ON HEIGHT: when width-scaling would make the image TALLER than maxHeight, it is fitted
// inside width × maxHeight instead (narrower than width). Not a refusal — ADR 0069 states no aspect
// rule and none is invented here — only a ceiling so a tall upload cannot produce an unbounded strip.
func RenderJPEGWidth(src image.Image, orientation, width, maxHeight, quality int) ([]byte, error) {
	if width <= 0 || maxHeight <= 0 || quality < 1 || quality > 100 {
		return nil, fmt.Errorf("imaging: width %d / maxHeight %d / quality %d out of range",
			width, maxHeight, quality)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, ErrUndecodable
	}
	ow, oh := w, h
	if orientation >= 5 && orientation <= 8 {
		ow, oh = h, w
	}
	tw := width
	th := int(int64(oh) * int64(width) / int64(ow))
	if th > maxHeight {
		th = maxHeight
		tw = int(int64(ow) * int64(maxHeight) / int64(oh))
	}
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	sw, sh := tw, th
	if orientation >= 5 && orientation <= 8 {
		sw, sh = th, tw
	}
	scaled := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(scaled, scaled.Bounds(), image.White, image.Point{}, draw.Src)
	draw.BiLinear.Scale(scaled, scaled.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, orient(scaled, orientation), &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("imaging: encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// scaleLongerTo returns w×h scaled (up or down) so the longer side is exactly n, never 0.
func scaleLongerTo(w, h, n int) (int, int) {
	if w >= h {
		nh := int(int64(h) * int64(n) / int64(w))
		if nh < 1 {
			nh = 1
		}
		return n, nh
	}
	nw := int(int64(w) * int64(n) / int64(h))
	if nw < 1 {
		nw = 1
	}
	return nw, n
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
