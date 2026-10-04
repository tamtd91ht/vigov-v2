package domain

import "time"

// UploadPolicy is one live row of `upload_policy` (migration 0008): the upload limits Vihat set
// for ONE purpose, platform-wide (ADR 0052 §10). Configuration, not business content — no commune,
// no personal data.
//
// Purpose is the core/storage spelling ("content-video"). The proto enum is DERIVED from it in
// internal/grpc; it is kept as a string here because the store must be able to hand back a row
// whose purpose this build's enum does not know, so the handler can skip it loudly instead of the
// scan failing for every purpose.
type UploadPolicy struct {
	Purpose          string
	MaxBytes         int64
	AllowedMIMETypes []string

	// FileCountLimited is false when the column is NULL: Vihat deliberately set no count limit.
	// MaxFilesPerSubject is then 0 and means nothing — never "zero files allowed".
	FileCountLimited   bool
	MaxFilesPerSubject int32

	UpdatedAt time.Time
	// UpdatedBy is the business code that last set the row (`VH-…`, or 'system' for a migration).
	UpdatedBy string
}
