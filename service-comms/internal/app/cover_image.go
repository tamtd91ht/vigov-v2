package app

// THE COVER DERIVATIVE — what the server makes from a scanned, promoted original before an article may
// publish it (ADR 0052 "Bổ sung 30/09/2026 (lần hai)" (a), ADR 0047 §6 (1)):
//
//	decode (JPEG · PNG · WebP — the platform policy's list, no HEIC) → orient by EXIF Orientation →
//	fit inside 1280×1280 (never enlarged) → flatten onto white → re-encode as JPEG, no metadata at all
//
// JPEG AND NOT WebP FOR THE OUTPUT, decided here and stated: ADR 0052 already names "mã hoá lại JPEG",
// the Go standard library encodes JPEG, and no WebP ENCODER exists in golang.org/x/image (it decodes
// only). A WebP output would mean cgo or a second, unvetted dependency for no reader who needs it — the
// Mini App webview renders JPEG everywhere.
//
// golang.org/x/image IS A NEW DEPENDENCY OF THIS SERVICE, for two things the standard library lacks:
// the WebP DECODER (the policy admits WebP input) and a proper downscaling kernel (x/image/draw).
//
// THE RE-ENCODE IS ALSO WHAT STRIPS EXIF. image/jpeg writes no APP1 segment, so GPS coordinates, the
// device serial and the capture time of the original never reach the derivative — and the derivative is
// the only thing that is ever published (ADR 0052 §2: the original stays private).

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

	"github.com/vihat/vigov/core/storage"
)

const (
	// CoverDerivativeVariant is the key variant of the derivative (core/storage's `thumb-<width>` family).
	CoverDerivativeVariant = "thumb-1280"
	// coverMaxSide is the 1280 of the variant: the longer side of the derivative, at most.
	coverMaxSide = 1280
	// coverJPEGQuality is a vendor choice: visually lossless for a news photo at 1280 px, ~150–400 KB.
	coverJPEGQuality = 85

	// coverDecodeBudget bounds the memory ONE decode may take, ESTIMATED from the header before any pixel
	// is decoded (decodeCost). A VENDOR BOUND AGAINST DECOMPRESSION BOMBS AND OUT-OF-MEMORY, not a
	// customer number: a 50 MB upload (the platform limit) can declare 30 000 × 30 000 pixels, and the
	// comms pod runs with a 384 MiB memory limit (deploy/base/comms/deployment.yaml). 160 MiB admits a
	// 48-megapixel phone photo (baseline JPEG) and refuses what would kill the process. Raising it needs
	// the pod's memory limit raised with it.
	coverDecodeBudget = 160 << 20

	// coverHeadBytes is how much of the original is read to find the dimensions, the EXIF orientation
	// and the JPEG frame type. EXIF lives in one APP1 segment of at most 64 KiB right after SOI.
	coverHeadBytes = 512 << 10
)

var (
	// errCoverUndecodable: the bytes sniffed as an allowed type but do not decode as one.
	errCoverUndecodable = errors.New("ảnh bìa: không giải mã được ảnh")
	// errCoverTooManyPixels: decoding would exceed coverDecodeBudget.
	errCoverTooManyPixels = errors.New("ảnh bìa: ảnh có quá nhiều điểm ảnh để xử lý")
)

// coverHeader is what the first coverHeadBytes of an original say about it.
type coverHeader struct {
	cfg         image.Config
	orientation int // EXIF 1..8; 1 when absent
	progressive bool
}

// readCoverHeader inspects the head of an original of sniffed type mime.
func readCoverHeader(mime string, head []byte) (coverHeader, error) {
	var (
		h   = coverHeader{orientation: 1}
		err error
	)
	switch mime {
	case storage.MIMEJPEG:
		h.cfg, err = jpeg.DecodeConfig(bytes.NewReader(head))
		h.orientation = jpegOrientation(head)
		h.progressive = jpegProgressive(head)
	case storage.MIMEPNG:
		h.cfg, err = png.DecodeConfig(bytes.NewReader(head))
	case storage.MIMEWebP:
		h.cfg, err = webp.DecodeConfig(bytes.NewReader(head))
	default:
		return coverHeader{}, errCoverUndecodable
	}
	if err != nil || h.cfg.Width <= 0 || h.cfg.Height <= 0 {
		return coverHeader{}, errCoverUndecodable
	}
	return h, nil
}

// decodeCost estimates the bytes a full decode allocates. Conservative by design: the case it gets
// wrong must be a refusal of a decodable image, never an out-of-memory kill of the pod.
//
//	JPEG baseline     ≤ 3 B/px (YCbCr 4:4:4; 4:2:0 is 1.5)
//	JPEG progressive  + 12 B/px: image/jpeg keeps every coefficient as int32 until the last scan
//	PNG               by colour model: 1 (gray, paletted) · 2 (gray16) · 4 (RGBA) · 8 (RGBA64)
//	WebP              4 B/px (lossless decodes to NRGBA; lossy YCbCr+alpha is ≤ 2.5)
func decodeCost(mime string, h coverHeader) int64 {
	px := int64(h.cfg.Width) * int64(h.cfg.Height)
	per := int64(8)
	switch mime {
	case storage.MIMEJPEG:
		per = 3
		if h.progressive {
			per += 12
		}
	case storage.MIMEPNG:
		switch h.cfg.ColorModel {
		case color.GrayModel:
			per = 1
		case color.Gray16Model:
			per = 2
		case color.RGBAModel, color.NRGBAModel:
			per = 4
		default:
			if _, ok := h.cfg.ColorModel.(color.Palette); ok {
				per = 1
			} else {
				per = 8
			}
		}
	case storage.MIMEWebP:
		per = 4
	}
	return px * per
}

// decodeCover decodes a full original of sniffed type mime.
func decodeCover(mime string, r io.Reader) (image.Image, error) {
	var (
		img image.Image
		err error
	)
	switch mime {
	case storage.MIMEJPEG:
		img, err = jpeg.Decode(r)
	case storage.MIMEPNG:
		img, err = png.Decode(r)
	case storage.MIMEWebP:
		img, err = webp.Decode(r)
	default:
		return nil, errCoverUndecodable
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCoverUndecodable, err)
	}
	return img, nil
}

// renderCoverDerivative orients, fits and re-encodes a decoded original as the JPEG derivative.
func renderCoverDerivative(src image.Image, orientation int) ([]byte, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, errCoverUndecodable
	}
	// The SCALED size is computed in the ORIENTED frame, so the long side is the long side residents see.
	ow, oh := w, h
	if orientation >= 5 && orientation <= 8 {
		ow, oh = h, w
	}
	tw, th := fitInside(ow, oh, coverMaxSide)
	// Scale in the stored frame (dimensions swapped back), then rotate the small result: rotating
	// first would walk every pixel of a 48-megapixel image a second time.
	sw, sh := tw, th
	if orientation >= 5 && orientation <= 8 {
		sw, sh = th, tw
	}
	// White first, then the source OVER it: JPEG has no alpha, and a transparent PNG flattened onto
	// black is a black rectangle on a resident's screen.
	scaled := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(scaled, scaled.Bounds(), image.White, image.Point{}, draw.Src)
	draw.BiLinear.Scale(scaled, scaled.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, orient(scaled, orientation), &jpeg.Options{Quality: coverJPEGQuality}); err != nil {
		return nil, fmt.Errorf("ảnh bìa: mã hoá JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// fitInside returns w×h scaled down so the longer side is at most max, never scaled up, never 0.
func fitInside(w, h, max int) (int, int) {
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

// jpegOrientation reads EXIF Orientation from the APP1 segment of a JPEG head; 1 when there is none or
// it cannot be read. A malformed EXIF block is NOT an error: the image is still decodable, and the
// worst outcome of ignoring it is a sideways photo, which staff see in the preview before publishing.
func jpegOrientation(head []byte) int {
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

// jpegProgressive reports whether the frame is progressive (SOF2) — what makes decodeCost add the
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
