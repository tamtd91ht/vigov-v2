package app

// THE COVER DERIVATIVE — what the server makes from a scanned, promoted original before an article may
// publish it (ADR 0052 "Bổ sung 30/09/2026 (lần hai)" (a), ADR 0047 §6 (1)):
//
//	decode (JPEG · PNG · WebP — the platform policy's list, no HEIC) → orient by EXIF Orientation →
//	fit inside 1280×1280 (never enlarged) → flatten onto white → re-encode as JPEG, no metadata at all
//
// THE PIXEL WORK LIVES IN core/imaging SINCE 2026-10-02. It was written here, and moved unchanged when
// service-petitions needed the same re-encode for a citizen's scene photo (ADR 0047 G3): one service
// may not import another's internal/ package (rule 2, forbidden #1), and two copies of an EXIF stripper
// are two copies of the one function whose drift leaks a citizen's home coordinates. What stays here
// is what is THIS flow's: the 1280 px side, the quality, the decode budget sized for the comms pod, and
// the sentinels its callers map to a CoverRejection. The functions below keep their old names and
// signatures so content_cover.go, portal_image.go and cover_image_test.go read exactly as before — the
// test suite passing unchanged is the proof the output did not move.
//
// JPEG AND NOT WebP FOR THE OUTPUT, and why the re-encode is also what strips EXIF: core/imaging's
// package comment.

import (
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/vihat/vigov/core/imaging"
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
	// and the JPEG frame type.
	coverHeadBytes = imaging.HeadBytes
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

func (h coverHeader) core() imaging.Header {
	return imaging.Header{Config: h.cfg, Orientation: h.orientation, Progressive: h.progressive}
}

// readCoverHeader inspects the head of an original of sniffed type mime.
func readCoverHeader(mime string, head []byte) (coverHeader, error) {
	h, err := imaging.ReadHeader(mime, head)
	if err != nil {
		return coverHeader{}, errCoverUndecodable
	}
	return coverHeader{cfg: h.Config, orientation: h.Orientation, progressive: h.Progressive}, nil
}

// decodeCost estimates the bytes a full decode allocates (imaging.DecodeCost).
func decodeCost(mime string, h coverHeader) int64 { return imaging.DecodeCost(mime, h.core()) }

// decodeCover decodes a full original of sniffed type mime. A read error of the object (storage
// ErrChanged / ErrNotFound) stays in the chain: content_cover.go tells it apart from a bad image.
func decodeCover(mime string, r io.Reader) (image.Image, error) {
	img, err := imaging.Decode(mime, r)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCoverUndecodable, err)
	}
	return img, nil
}

// renderCoverDerivative orients, fits and re-encodes a decoded original as the JPEG derivative.
func renderCoverDerivative(src image.Image, orientation int) ([]byte, error) {
	out, err := imaging.RenderJPEG(src, orientation, coverMaxSide, coverJPEGQuality)
	switch {
	case errors.Is(err, imaging.ErrUndecodable):
		return nil, errCoverUndecodable
	case err != nil:
		return nil, fmt.Errorf("ảnh bìa: mã hoá JPEG: %w", err)
	}
	return out, nil
}

// fitInside returns w×h scaled down so the longer side is at most max, never scaled up, never 0.
func fitInside(w, h, max int) (int, int) { return imaging.FitInside(w, h, max) }

// jpegOrientation reads EXIF Orientation from the APP1 segment of a JPEG head; 1 when absent.
func jpegOrientation(head []byte) int { return imaging.JPEGOrientation(head) }
