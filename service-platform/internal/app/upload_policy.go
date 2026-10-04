package app

import (
	"errors"
	"slices"

	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// The operator's edit of an upload policy (ADR 0073 #5, ADR 0052 §10) is checked here against what
// the PIPELINE consuming each purpose can actually handle — not merely against core/storage's global
// allow-list. A policy only NARROWS (migration 0008): a value the consumer cannot process is a limit
// that admits files the service will then fail on, with the operator believing it allowed them.
//
// ONE CLASS PER PURPOSE. A purpose missing from purposeClass is refused: a new core/storage purpose
// must be classed here before the console can edit it (TestEveryPurposeHasAClass pins that).

// mediaClass is what the consumer does with a file of that purpose, and so which types and sizes are
// sane for it.
type mediaClass struct {
	name  string
	mimes []string // the types the consumer handles; each one in core/storage's allow-list
	// maxBytes is the upper bound an operator may set. Bytes, not MB.
	maxBytes int64
}

const (
	mib = int64(1) << 20
	gib = int64(1) << 30
)

var (
	// REENCODED IMAGES: decoded and re-encoded server-side (core/imaging — EXIF stripped, resized),
	// which decodes JPEG, PNG and WebP only. HEIC is therefore NEVER allowed here: G3 (ADR 0047) for
	// the scene photo, ADR 0069 #4 for logo/banner, the 01/10/2026 decision for the Mini App cover.
	// Cap 50 MiB: the largest value the owner has set for any image purpose (0012), and the whole file
	// is read into memory to be re-encoded — a larger cap is an out-of-memory route, not a limit.
	classReencodedImage = mediaClass{"reencoded-image",
		[]string{storage.MIMEJPEG, storage.MIMEPNG, storage.MIMEWebP}, 50 * mib}
	// VIDEO: stored as uploaded. Cap 5 GiB = S3's single-POST limit, the database CHECK of 0008.
	classVideo = mediaClass{"video", []string{storage.MIMEMP4, storage.MIMEQuickTime}, 5 * gib}
	// AUDIO: the broadcast file of a `truyen-thanh` item (ADR 0067 §4); its whole stream is checked.
	// Cap 100 MiB — chosen here (owner set 30 MiB); stated in the report, not decided by the owner.
	classAudio = mediaClass{"audio", []string{storage.MIMEMP3, storage.MIMEM4A}, 100 * mib}
	// DOCUMENTS: stored as uploaded and served as an attachment. The types are the widest set the owner
	// has set for any document purpose (document-scan, 0008); WebP/HEIC were never chosen for one.
	// Cap 100 MiB — chosen here (owner set 50 MiB); stated in the report.
	classDocument = mediaClass{"document",
		[]string{storage.MIMEPDF, storage.MIMEJPEG, storage.MIMEPNG}, 100 * mib}
)

var purposeClass = map[storage.Purpose]mediaClass{
	storage.PurposeTenantLogo:                classReencodedImage,
	storage.PurposeTenantBanner:              classReencodedImage,
	storage.PurposePetitionPhoto:             classReencodedImage,
	storage.PurposePetitionVerificationPhoto: classReencodedImage,
	storage.PurposeContentImage:              classReencodedImage,
	storage.PurposeContentBodyImage:          classReencodedImage,
	storage.PurposeStaffAvatar:               classReencodedImage,
	storage.PurposeContentVideo:              classVideo,
	storage.PurposeContentAudio:              classAudio,
	storage.PurposeContentAttachment:         classDocument,
	storage.PurposeDocumentScan:              classDocument,
	storage.PurposeTaskAttachment:            classDocument,
	storage.PurposePetitionLogAttachment:     classDocument,
}

// MaxFilesCap bounds max_files_per_subject when it is set: the largest value the owner set is 20
// (content-body-image, 0018). Chosen here; stated in the report.
const MaxFilesCap = 100

var (
	ErrUnknownPurpose  = errors.New("upload policy: mục đích không có")
	ErrMaxBytes        = errors.New("upload policy: max_bytes ngoài khoảng cho phép")
	ErrMIMETypes       = errors.New("upload policy: kiểu tệp không hợp lệ cho mục đích này")
	ErrMaxFilesInvalid = errors.New("upload policy: max_files_per_subject ngoài khoảng cho phép")
)

// UploadPolicyChoices is what the console may offer for one purpose.
type UploadPolicyChoices struct {
	MIMETypes   []string
	MaxBytesCap int64
}

// ChoicesFor returns the types and the byte cap an operator may set for purpose; ok=false for a
// purpose this build does not class.
func ChoicesFor(purpose string) (UploadPolicyChoices, bool) {
	c, ok := purposeClass[storage.Purpose(purpose)]
	if !ok {
		return UploadPolicyChoices{}, false
	}
	return UploadPolicyChoices{MIMETypes: slices.Clone(c.mimes), MaxBytesCap: c.maxBytes}, true
}

// ValidateUploadPolicy checks an operator's edit. p.Purpose must be a core/storage purpose with a
// class; the MIME list non-empty, without repeats, every type in core/storage's allow-list AND in the
// purpose's class; max_bytes in (0, cap]; max_files_per_subject, when limited, in [1, MaxFilesCap].
func ValidateUploadPolicy(p domain.UploadPolicy) error {
	c, ok := purposeClass[storage.Purpose(p.Purpose)]
	if !ok || !slices.Contains(storage.Purposes(), storage.Purpose(p.Purpose)) {
		return ErrUnknownPurpose
	}
	if p.MaxBytes <= 0 || p.MaxBytes > c.maxBytes {
		return ErrMaxBytes
	}
	if len(p.AllowedMIMETypes) == 0 {
		return ErrMIMETypes
	}
	seen := map[string]bool{}
	for _, m := range p.AllowedMIMETypes {
		if _, known := storage.ExtForMIME(m); !known || !slices.Contains(c.mimes, m) || seen[m] {
			return ErrMIMETypes
		}
		seen[m] = true
	}
	if p.FileCountLimited && (p.MaxFilesPerSubject < 1 || p.MaxFilesPerSubject > MaxFilesCap) {
		return ErrMaxFilesInvalid
	}
	return nil
}
