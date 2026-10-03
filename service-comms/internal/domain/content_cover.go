package domain

// THE COVER IMAGE OF A MINI APP ARTICLE — this service's own file metadata (`stored_file`, entity
// CommsStoredFile, migration 0011) and the rules for pointing an article at one (ADR 0047 §6 (1),
// ADR 0052 §5, §11).
//
// THE SHAPE IS service-petitions/internal/domain/stored_file.go's, field for field, because the table
// is 0021's column for column (0011 says so). Not imported from there: another service's `internal/`
// is rule 2, forbidden #1. What differs is what 0011 changes — one subject type, one bucket, one
// retention class, and `public_object_key` for the published derivative.
//
// PLAIN STRINGS, NOT core/storage's types, for the class, the purpose and the bucket: domain imports
// the standard library only. The app layer converts, and the closed lists are enforced where the key
// is built (core/storage Key.Path) and again by the schema.

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// StoredFileStatus is ADR 0052 §5's status. Mirrored by migration 0011 (`stored_file_status_known`,
// `stored_file_guard`); the edge list below is the guard's, character for character.
type StoredFileStatus string

const (
	StoredFilePending    StoredFileStatus = "pending"    // upload issued, bytes not yet checked
	StoredFileScanning   StoredFileStatus = "scanning"   // completion running: sniff, scan, hash
	StoredFileStored     StoredFileStatus = "stored"     // original copied to the private bucket, measured
	StoredFileProcessing StoredFileStatus = "processing" // the cover derivative is being produced
	StoredFileReady      StoredFileStatus = "ready"      // the derivative exists; the file may be a cover
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

// Values of the columns migration 0011 closes with a CHECK.
const (
	// StoredFileSubjectContentItem: the file was issued for one article; SubjectID is its id — possibly
	// an id minted at upload time for an article not saved yet (0011: the modal uploads before `Lưu`).
	StoredFileSubjectContentItem = "content-item"
	// StoredFileBucketPrivate is the bucket ROLE of every original (0011 admits no other).
	StoredFileBucketPrivate = "private"
)

// StoredFile is one `stored_file` row of this service.
//
// ⚠ OriginalName IS PERSONAL DATA WHEN IT NAMES A PERSON ("trao-qua-ong-nguyen-van-a.jpg", 0011):
// never logged, never in an error, never in a key or the audit delta.
type StoredFile struct {
	ID             string // also the {object_id} segment of ObjectKey
	Bucket         string // StoredFileBucketPrivate
	ObjectKey      string // the ORIGINAL's key (ADR 0052 §3); temp copy is `upload/` + this
	RetentionClass string // core/storage.Class value — always content-source here
	Purpose        string // core/storage.Purpose value — content-image for a cover
	SubjectType    string // StoredFileSubjectContentItem
	SubjectID      string
	OriginalName   string

	// Measured by the completion; zero until then, write-once after (stored_file_guard).
	MIMEType  string
	SizeBytes int64
	SHA256    string

	Status StoredFileStatus

	// PublicObjectKey is the published derivative's key in the PUBLIC bucket; "" = not public. The
	// one path a public URL is built from (0011) — never the original's ObjectKey.
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

// MaxCoverFileNameRunes is migration 0011's `char_length(original_name) <= 255`.
const MaxCoverFileNameRunes = 255

var (
	// ErrCoverFileNameInvalid — empty, longer than MaxCoverFileNameRunes, not UTF-8, or a control
	// character (0011 `stored_file_original_name_shape`). Never quotes the name (rule 3).
	ErrCoverFileNameInvalid = errors.New(
		"ảnh bìa: tên tệp không hợp lệ — cần có tên, không quá 255 ký tự và không chứa ký tự điều khiển")

	// ErrCoverSizeInvalid — the declared size is not a positive number of bytes.
	ErrCoverSizeInvalid = errors.New("ảnh bìa: kích thước tệp khai báo không hợp lệ")

	// ErrCoverNotUsable refuses pointing an article at a file. ONE SENTENCE FOR EVERY CAUSE — unknown
	// id, another commune's, uploaded for another article, not finished, refused, deleted — because
	// telling them apart would say which ids exist (rule 4, forbidden #2, applied to staff).
	ErrCoverNotUsable = errors.New(
		"ảnh bìa: chỉ dùng được ảnh đã tải lên cho chính mục nội dung này và đã xử lý xong")
)

// CleanCoverFileName returns the name a cover is recorded under: the last path element (some browsers
// still send `C:\fakepath\…`), trimmed. Nothing else is rewritten — it never reaches a key, and the
// staff preview sanitises it for Content-Disposition (core/storage.ContentDisposition).
func CleanCoverFileName(s string) (string, error) {
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxCoverFileNameRunes {
		return "", ErrCoverFileNameInvalid
	}
	if strings.ContainsFunc(s, unicode.IsControl) {
		return "", ErrCoverFileNameInvalid
	}
	return s, nil
}

// MaxFileIDLen bounds a file id off the wire. File ids are 26-character ULIDs minted here; anything
// longer cannot name one and is refused before it reaches a statement.
const MaxFileIDLen = 64

// ErrCoverFileIDInvalid — `cover_image_file_id` longer than any id this service mints, or with a
// character no ULID has. Never echoes the value.
var ErrCoverFileIDInvalid = errors.New("noi_dung_mini_app: `cover_image_file_id` không hợp lệ")

// cleanFileID trims a file id from a request: "" stays "" (no cover / detach), anything else must be
// one short token of ULID characters. Whether the file EXISTS and fits is CheckCoverUsable's question.
func cleanFileID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) > MaxFileIDLen {
		return "", ErrCoverFileIDInvalid
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return "", ErrCoverFileIDInvalid
		}
	}
	return s, nil
}

// CheckCoverUsable is the write path's copy of migration 0011's `noi_dung_mini_app_cover_image_check`,
// ONE STEP STRICTER: the trigger admits `stored` and `processing`, this admits `ready` only, because a
// cover without its derivative could never be published (ADR 0052 §11 publishes the derivative, never
// the original) and the article would go live with no image and no error. The trigger stays the floor.
func CheckCoverUsable(f *StoredFile, itemID, purpose string) error {
	if !fileUsableFor(f, itemID, purpose) {
		return ErrCoverNotUsable
	}
	return nil
}

// ErrBodyImageNotUsable refuses a body naming an image file (`<img data-file-id>`, ADR 0067 §Sửa đổi
// 03/10/2026, K2). ONE SENTENCE FOR EVERY CAUSE — unknown id, another commune's, uploaded for another
// article, the cover's file, not finished, refused, deleted — for ErrCoverNotUsable's reason: telling them
// apart would say which ids exist (rule 4, forbidden #2, applied to staff).
var ErrBodyImageNotUsable = errors.New(
	"ảnh trong thân bài: chỉ dùng được ảnh đã tải lên cho thân bài của chính mục nội dung này và đã xử lý xong")

// CheckBodyImageUsable is CheckCoverUsable for a body image: uploaded FOR THIS ARTICLE under the body-image
// purpose, `ready` (its derivative exists — the one thing a publish copies), not deleted. There is NO
// trigger under it: the body is free text, so this check on the write path is the whole wall.
func CheckBodyImageUsable(f *StoredFile, itemID, purpose string) error {
	if !fileUsableFor(f, itemID, purpose) {
		return ErrBodyImageNotUsable
	}
	return nil
}

// fileUsableFor: a live row (the store reads live rows only), issued for itemID under purpose, ready.
func fileUsableFor(f *StoredFile, itemID, purpose string) bool {
	return f != nil && itemID != "" && purpose != "" && f.SubjectType == StoredFileSubjectContentItem &&
		f.SubjectID == itemID && f.Purpose == purpose && f.Status == StoredFileReady
}
