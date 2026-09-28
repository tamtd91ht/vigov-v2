package domain

import "time"

// This service's file metadata (ADR 0052 §5, migration 0021) and the attachment of a file to a task
// progress-log entry (docs/ui-ux/02-nhiem-vu.md §5.9 `📎 Đính kèm`).
//
// PLAIN STRINGS, NOT core/storage's types, for the retention class, the purpose and the bucket:
// domain stays free of the MinIO client, and the closed lists are enforced where the key is built
// (core/storage Key.Path) and again by the schema. The app layer converts.

// StoredFileStatus is ADR 0052 §5's status. The list and the edges are mirrored by migration 0021
// (`stored_file_status_known`, `stored_file_guard`); TestMigration0021… pins the two together.
type StoredFileStatus string

const (
	StoredFilePending    StoredFileStatus = "pending"    // upload issued, bytes not yet checked
	StoredFileScanning   StoredFileStatus = "scanning"   // complete step running: sniff, hash, scan
	StoredFileStored     StoredFileStatus = "stored"     // copied to the destination, measured
	StoredFileProcessing StoredFileStatus = "processing" // derivative being produced (video)
	StoredFileReady      StoredFileStatus = "ready"      // derivatives done
	StoredFileFailed     StoredFileStatus = "failed"     // could not be completed or processed
	StoredFileRejected   StoredFileStatus = "rejected"   // refused: infected, wrong type, over limit
	StoredFilePurged     StoredFileStatus = "purged"     // object removed; the row is kept (rule 7)
)

// storedFileEdges is the ONE transition table. `scanning → pending` is the retry ADR 0052 §9 allows
// when the scanner is unreachable — back to waiting, never forward to `stored`.
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

// StoredFileStatuses returns every status, in ADR 0052 §5's order.
func StoredFileStatuses() []StoredFileStatus {
	return []StoredFileStatus{StoredFilePending, StoredFileScanning, StoredFileStored,
		StoredFileProcessing, StoredFileReady, StoredFileFailed, StoredFileRejected, StoredFilePurged}
}

// StoredFileEdges returns a copy of the transition table, for the migration drift test.
func StoredFileEdges() map[StoredFileStatus][]StoredFileStatus {
	out := make(map[StoredFileStatus][]StoredFileStatus, len(storedFileEdges))
	for k, v := range storedFileEdges {
		out[k] = append([]StoredFileStatus(nil), v...)
	}
	return out
}

// CanMoveTo reports whether s → to is an edge. A same-status "move" is not an edge.
func (s StoredFileStatus) CanMoveTo(to StoredFileStatus) bool {
	for _, t := range storedFileEdges[s] {
		if t == to {
			return true
		}
	}
	return false
}

// Attachable reports whether a file in this status may be linked to a log entry: it reached the
// destination. Mirrored by `task_log_attachment_check` in migration 0021.
func (s StoredFileStatus) Attachable() bool {
	return s == StoredFileStored || s == StoredFileReady
}

// Values of the columns migration 0021 closes with a CHECK.
const (
	// StoredFileSubjectTask: the file was issued for one task. `SubjectID` is the task's INTERNAL id.
	StoredFileSubjectTask = "task"

	// Bucket ROLES (core/storage.Bucket), not bucket names — see migration 0021 on `bucket`.
	StoredFileBucketPrivate = "private"
	StoredFileBucketPublic  = "public"
)

// StoredFile is one `stored_file` row.
//
// ⚠ OriginalName IS PERSONAL DATA WHEN IT DESCRIBES A CASE (rule 3, ADR 0052 §3): never logged,
// never in an error, masked on the way out unless the caller holds full view.
type StoredFile struct {
	ID             string // also the {object_id} segment of ObjectKey
	Bucket         string // StoredFileBucket*
	ObjectKey      string // destination key (ADR 0052 §3); temp copy is `upload/` + this
	RetentionClass string // core/storage.Class value
	Purpose        string // core/storage.Purpose value
	SubjectType    string // StoredFileSubject*
	SubjectID      string
	OriginalName   string

	// Measured by the complete step; zero until then.
	MIMEType  string
	SizeBytes int64
	SHA256    string

	Status StoredFileStatus

	// UploadedBy is a STAFF BUSINESS CODE (rule 6, invariant 8).
	UploadedBy string

	RetainUntil time.Time // zero = not fixed (always zero for `records`)
	LegalHold   bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// StoredFileFacts are what the complete step measures before `scanning → stored`.
type StoredFileFacts struct {
	MIMEType  string // SNIFFED, never the client's Content-Type
	SizeBytes int64
	SHA256    string // lowercase hex, 64 characters
}

// TaskLogAttachment is one file as a log-entry timeline shows it. No object key, no uploader:
// the reader gets a file to name and a status to explain, and the download is a separate,
// signed request (rule 4, invariant 7; ADR 0052 §12).
type TaskLogAttachment struct {
	LogEntryID   string
	FileID       string
	OriginalName string // personal data — see StoredFile
	MIMEType     string
	SizeBytes    int64
	Status       StoredFileStatus
}
