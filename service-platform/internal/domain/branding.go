package domain

// THE COMMUNE'S IDENTITY IMAGES (ADR 0069) — this service's own file metadata (`stored_file`, entity
// PlatformStoredFile, migration 0017) and the two references from the display profile to the CURRENT
// logo and the CURRENT web-admin banner.
//
// THE SHAPE IS service-comms/internal/domain/content_cover.go's, field for field, because 0017's table is
// comms 0011's column for column (0017 says so). Not imported from there: another service's `internal/`
// is rule 2, forbidden #1. What differs is what 0017 changes — the purpose is a closed pair, and the
// subject is always the commune's own display profile (subject_id = tenant_id).
//
// PLAIN STRINGS for the class, the purpose and the bucket: domain imports the standard library only
// (go-service-pattern). The app layer converts, and the closed lists are enforced where the key is built
// (core/storage Key.Path) and again by the schema.

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// StoredFileStatus is ADR 0052 §5's status. Mirrored by migration 0017 (`stored_file_status_known`,
// `stored_file_guard`); the edge list below is the guard's, character for character.
type StoredFileStatus string

const (
	StoredFilePending    StoredFileStatus = "pending"    // upload issued, bytes not yet checked
	StoredFileScanning   StoredFileStatus = "scanning"   // completion running: sniff, scan, hash
	StoredFileStored     StoredFileStatus = "stored"     // original copied to the private bucket, measured
	StoredFileProcessing StoredFileStatus = "processing" // the normalised derivative is being produced
	StoredFileReady      StoredFileStatus = "ready"      // derivative published and the profile pointed at it
	StoredFileFailed     StoredFileStatus = "failed"     // expired unused, or the image could not be re-encoded
	StoredFileRejected   StoredFileStatus = "rejected"   // refused: infected, wrong type, over the limit
	StoredFilePurged     StoredFileStatus = "purged"     // object removed; the row is kept (rule 7)
)

var storedFileEdges = map[StoredFileStatus][]StoredFileStatus{
	StoredFilePending:    {StoredFileScanning, StoredFileFailed, StoredFileRejected},
	StoredFileScanning:   {StoredFileStored, StoredFileFailed, StoredFileRejected, StoredFilePending},
	StoredFileStored:     {StoredFileProcessing, StoredFilePurged},
	StoredFileProcessing: {StoredFileReady, StoredFileFailed},
	StoredFileReady:      {StoredFilePurged},
	StoredFileFailed:     {StoredFilePurged},
	StoredFileRejected:   {StoredFilePurged},
	StoredFilePurged:     nil,
}

// CanMoveTo reports whether s → to is an edge of stored_file_guard. A same-status "move" is not.
func (s StoredFileStatus) CanMoveTo(to StoredFileStatus) bool {
	for _, t := range storedFileEdges[s] {
		if t == to {
			return true
		}
	}
	return false
}

// Values of the columns migration 0017 closes with a CHECK.
const (
	// StoredFileSubjectDisplayProfile: every file here belongs to the commune's display profile, whose
	// key IS tenant_id — so SubjectID always equals the commune (0017 `stored_file_subject_is_own_commune`).
	StoredFileSubjectDisplayProfile = "tenant-display-profile"
	// StoredFileBucketPrivate is the bucket ROLE of every original (0017 admits no other).
	StoredFileBucketPrivate = "private"
)

// BrandingImage is which of the two identity images an act is about. Its value IS the stored_file
// purpose (core/storage.PurposeTenantLogo / PurposeTenantBanner) — one spelling, checked by 0017.
type BrandingImage string

const (
	BrandingLogo   BrandingImage = "tenant-logo"   // ADR 0069 #4 — sidebar, login screen, Mini App
	BrandingBanner BrandingImage = "tenant-banner" // ADR 0069 #5 — the strip under the web-admin topbar
)

// Valid reports whether b is one of the two images.
func (b BrandingImage) Valid() bool { return b == BrandingLogo || b == BrandingBanner }

// StoredFile is one `stored_file` row of this service.
//
// ⚠ OriginalName IS PERSONAL DATA WHEN IT NAMES A PERSON ("logo-chu-tich-nguyen-van-a.png", 0017):
// never logged, never in an error, never in a key or the audit delta.
type StoredFile struct {
	ID             string // also the {object_id} segment of ObjectKey
	Bucket         string // StoredFileBucketPrivate
	ObjectKey      string // the ORIGINAL's key (ADR 0052 §3); temp copy is `upload/` + this
	RetentionClass string // core/storage.Class value — always content-source here
	Purpose        string // BrandingImage value
	SubjectType    string // StoredFileSubjectDisplayProfile
	SubjectID      string // = the commune
	OriginalName   string

	// Measured by the completion; zero until then, write-once after (stored_file_guard).
	MIMEType  string
	SizeBytes int64
	SHA256    string

	Status StoredFileStatus

	// PublicObjectKey is the published derivative's key in the PUBLIC bucket; "" = not public. The one
	// path a public URL is built from (0017) — never the original's ObjectKey.
	PublicObjectKey string

	// UploadedBy is a STAFF BUSINESS CODE (rule 6, invariant 8).
	UploadedBy string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// StoredFileFacts are what the completion measures before `scanning → stored`.
type StoredFileFacts struct {
	MIMEType  string // SNIFFED, never the client's Content-Type
	SizeBytes int64
	SHA256    string // lowercase hex
}

// ProfileBranding is the display-profile row as the branding write path sees it, read FOR UPDATE.
type ProfileBranding struct {
	// Deleted: the row exists but is soft-deleted. The write path REFUSES (ErrProfileDeleted) — it
	// never revives the row and never inserts a second one (the primary key is tenant_id).
	Deleted      bool
	LogoFileID   string // "" = no logo
	BannerFileID string // "" = no banner
}

// FileID returns the current file of one image.
func (p ProfileBranding) FileID(img BrandingImage) string {
	if img == BrandingLogo {
		return p.LogoFileID
	}
	return p.BannerFileID
}

// BrandingView is what the staff settings tab reads: the PUBLIC KEYS of the current images (the app
// layer turns them into URLs) and who last changed the profile row. Keys are "" unless the referenced
// file is `ready`, live and published.
type BrandingView struct {
	LogoPublicKey   string
	BannerPublicKey string
	// UpdatedAt / UpdatedBy are the profile row's cap_nhat_luc / cap_nhat_boi; zero / "" when the
	// commune has no live profile row.
	UpdatedAt time.Time
	UpdatedBy string // staff business code (CB-…)
}

// MaxFileNameRunes is migration 0017's `char_length(original_name) <= 255`.
const MaxFileNameRunes = 255

// MaxFileIDLen bounds a file id off the wire. File ids are 26-character ULIDs minted here.
const MaxFileIDLen = 64

var (
	// ErrBrandingFileNameInvalid — empty, longer than MaxFileNameRunes, not UTF-8, or a control
	// character (0017 `stored_file_original_name_shape`). Never quotes the name (rule 3).
	ErrBrandingFileNameInvalid = errors.New(
		"nhận diện xã: tên tệp không hợp lệ — cần có tên, không quá 255 ký tự và không chứa ký tự điều khiển")

	// ErrBrandingSizeInvalid — the declared size is not a positive number of bytes.
	ErrBrandingSizeInvalid = errors.New("nhận diện xã: kích thước tệp khai báo không hợp lệ")

	// ErrProfileDeleted — the commune's display profile row is soft-deleted. Writing branding would
	// either revive a row somebody retired (with its reason) or need a second row the key forbids.
	// Neither is this path's decision: an operator looks at why it was retired. 409.
	ErrProfileDeleted = errors.New("nhận diện xã: hồ sơ hiển thị của xã đã bị gỡ — không ghi nhận diện lên hồ sơ đã gỡ")
)

// CleanFileName returns the name an upload is recorded under: the last path element (some browsers
// still send `C:\fakepath\…`), trimmed. It never reaches a key.
func CleanFileName(s string) (string, error) {
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxFileNameRunes {
		return "", ErrBrandingFileNameInvalid
	}
	if strings.ContainsFunc(s, unicode.IsControl) {
		return "", ErrBrandingFileNameInvalid
	}
	return s, nil
}
