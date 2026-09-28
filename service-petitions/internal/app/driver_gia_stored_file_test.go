package app

// The fake driver's half for `stored_file` and `task_log_attachment` (migration 0021), and in-memory
// fakes of the three ADR 0052 dependencies: the object store, the malware scanner, the upload policy.
//
// THE FILE ROWS LIVE IN THE SAME FAKE AS THE TASK ROWS (khoNhiemVuGia) so a statement's
// `trongGiaoDich` flag and the commit count cover both: "the pending row, the link and the audit entry
// are in ONE transaction" is then a property the tests can see.
//
// WHAT IT DOES NOT DO: the triggers of migration 0021 (stored_file_guard, task_log_attachment_check).
// The status edges are honoured the way the store's WHERE clauses honour them (`status = $3`,
// `status = 'scanning'`), which is what the use case depends on; the trigger's own refusals are
// stored_file_pg_test.go's, which SKIPS without VIGOV_TEST_DSN.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// --- the driver half -------------------------------------------------------------------------------

// storedFileRow is one `stored_file` row in the shape the store's SELECT list names.
func storedFileRow(commune, id, subjectID, uploader string, status domain.StoredFileStatus,
	created time.Time) map[string]driver.Value {

	r := map[string]driver.Value{
		"tenant_id": commune, "id": id, "bucket": "private",
		"object_key":      "records/t_" + strings.ToLower(commune) + "/2026/09/petitions/task-attachment/" + strings.ToLower(id) + "/original.pdf",
		"retention_class": "records", "purpose": "task-attachment", "subject_type": "task",
		"subject_id": subjectID, "original_name": "Biên bản nghiệm thu.pdf",
		"mime_type": nil, "size_bytes": nil, "sha256": nil,
		"status": string(status), "uploaded_by": uploader, "retain_until": nil, "legal_hold": false,
		"created_at": created, "updated_at": created,
	}
	if status.Attachable() {
		r["mime_type"], r["size_bytes"], r["sha256"] = storage.MIMEPDF, int64(len(pdfBytes)), strings.Repeat("b", 64)
	}
	return r
}

func (k *khoNhiemVuGia) addStoredFile(r map[string]driver.Value) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.storedFiles == nil {
		k.storedFiles = map[string]map[string]driver.Value{}
	}
	k.storedFiles[fmt.Sprint(r["id"])] = r
}

func (k *khoNhiemVuGia) storedFile(id string) map[string]driver.Value {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.storedFiles[id]
}

// execStoredFile answers the four writes; handled is false for every other statement.
func (k *khoNhiemVuGia) execStoredFile(q string, named []driver.NamedValue) (driver.Result, bool, error) {
	args := make([]driver.Value, 0, len(named))
	for _, a := range named {
		args = append(args, a.Value)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.storedFiles == nil {
		k.storedFiles = map[string]map[string]driver.Value{}
	}
	if k.fileLinks == nil {
		k.fileLinks = map[string]string{}
	}

	switch {
	case strings.Contains(q, "INSERT INTO stored_file"):
		// tenant, id, bucket, object_key, class, purpose, subject_type, subject_id, name, uploader, at.
		k.storedFiles[fmt.Sprint(args[1])] = map[string]driver.Value{
			"tenant_id": args[0], "id": args[1], "bucket": args[2], "object_key": args[3],
			"retention_class": args[4], "purpose": args[5], "subject_type": args[6], "subject_id": args[7],
			"original_name": args[8], "mime_type": nil, "size_bytes": nil, "sha256": nil,
			"status": "pending", "uploaded_by": args[9], "retain_until": nil, "legal_hold": false,
			"created_at": args[10], "updated_at": args[10],
		}
		return driver.RowsAffected(1), true, nil

	case strings.Contains(q, "UPDATE stored_file SET status = 'stored'"):
		// tenant, id, mime, size, sha, at — only from `scanning`, as the WHERE says.
		r := k.storedFiles[fmt.Sprint(args[1])]
		if r == nil || r["tenant_id"] != args[0] || r["status"] != "scanning" {
			return driver.RowsAffected(0), true, nil
		}
		r["status"], r["mime_type"], r["size_bytes"], r["sha256"], r["updated_at"] =
			"stored", args[2], args[3], args[4], args[5]
		return driver.RowsAffected(1), true, nil

	case strings.Contains(q, "UPDATE stored_file"):
		// Transition: tenant, id, from, to, at.
		r := k.storedFiles[fmt.Sprint(args[1])]
		if r == nil || r["tenant_id"] != args[0] || r["status"] != args[2] {
			return driver.RowsAffected(0), true, nil
		}
		r["status"], r["updated_at"] = args[3], args[4]
		return driver.RowsAffected(1), true, nil

	case strings.Contains(q, "INSERT INTO task_log_attachment"):
		// tenant, entry, file ids… — the primary key (tenant_id, stored_file_id) refuses a second link.
		for _, id := range args[2:] {
			if _, taken := k.fileLinks[fmt.Sprint(id)]; taken {
				return nil, true, errors.New("driver giả: duplicate key value violates task_log_attachment_pkey")
			}
		}
		for _, id := range args[2:] {
			k.fileLinks[fmt.Sprint(id)] = fmt.Sprint(args[1])
		}
		return driver.RowsAffected(int64(len(args) - 2)), true, nil
	}
	return nil, false, nil
}

// doStoredFile answers the reads. EVERY ROW IS MATCHED ON $1 AS WELL AS ITS ID, so a statement that
// lost its commune predicate — or a caller in another commune — reads nothing.
func (k *khoNhiemVuGia) doStoredFile(q string, cot []string, args []driver.Value) (driver.Rows, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(args) < 2 {
		return nil, fmt.Errorf("driver giả: câu đọc tệp chỉ mang %d tham số — xã phải là $1: %q", len(args), q)
	}
	commune := args[0]
	live := func(id string) map[string]driver.Value {
		r := k.storedFiles[id]
		if r == nil || r["tenant_id"] != commune {
			return nil
		}
		return r
	}

	switch {
	case strings.Contains(q, "FROM task_log_attachment"):
		entry, ok := k.fileLinks[fmt.Sprint(args[1])]
		if !ok || live(fmt.Sprint(args[1])) == nil {
			return &rowsNVGia{cot: cot}, nil
		}
		return &rowsNVGia{cot: cot, hang: [][]driver.Value{{entry}}}, nil

	case strings.Contains(q, "LEFT JOIN task_log_attachment"):
		var rows []map[string]driver.Value
		for _, id := range args[1:] {
			r := live(fmt.Sprint(id))
			if r == nil {
				continue
			}
			withLink := make(map[string]driver.Value, len(r)+1)
			for c, v := range r {
				withLink[c] = v
			}
			withLink["a.log_entry_id"] = nil
			if e, ok := k.fileLinks[fmt.Sprint(id)]; ok {
				withLink["a.log_entry_id"] = e
			}
			rows = append(rows, withLink)
		}
		return dungRows(cot, rows)

	case strings.Contains(q, "count(*)"):
		// tenant, subject_type, subject_id, purpose[, pendingSince] — the store's own predicate.
		var n int64
		for _, r := range k.storedFiles {
			if r["tenant_id"] != commune || r["subject_type"] != args[1] || r["subject_id"] != args[2] ||
				r["purpose"] != args[3] {
				continue
			}
			switch r["status"] {
			case "stored", "processing", "ready":
				n++
			case "pending", "scanning":
				if len(args) > 4 && !r["created_at"].(time.Time).Before(args[4].(time.Time)) {
					n++
				}
			}
		}
		return &rowsNVGia{cot: []string{"n"}, hang: [][]driver.Value{{n}}}, nil

	default:
		// ForUpdate and ByID: $2 is the id.
		r := live(fmt.Sprint(args[1]))
		if r == nil {
			return &rowsNVGia{cot: cot}, nil
		}
		return dungRows(cot, []map[string]driver.Value{r})
	}
}

// --- the object store ------------------------------------------------------------------------------

// pdfBytes and pngBytes sniff as what they claim (core/storage.SniffMIME); htmlBytes as nothing.
var (
	pdfBytes  = []byte("%PDF-1.7\n1 0 obj << /Type /Catalog >> endobj\n%%EOF\n")
	pngBytes  = append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte("IHDR-fake")...)
	htmlBytes = []byte("<html><script>alert(1)</script></html>")
)

type fakeObject struct {
	data []byte
	etag string
}

// objectStoreFake is core/storage's contract in memory: ETag-bound reads fail with ErrChanged, a
// missing object with ErrNotFound, Promote re-sniffs and refuses an existing destination. It records
// what it was asked, so a test can say "the upload form was signed for exactly this key and limit".
type objectStoreFake struct {
	mu      sync.Mutex
	temp    map[string]fakeObject
	private map[string]fakeObject
	gen     int

	// replaceAfterStat swaps the temp object's ETag right after the first Stat — the client re-posting
	// through the still-valid form while the service inspects.
	replaceAfterStat bool
	statErr          error // returned by every Stat when set
	purgeErr         error

	presigned []presignCall
	downloads []downloadCall
	promoted  []string
	purged    []string
}

type presignCall struct {
	key         string
	maxBytes    int64
	contentType string
	ttl         time.Duration
}

type downloadCall struct {
	key      string
	ttl      time.Duration
	filename string
}

func newObjectStoreFake() *objectStoreFake {
	return &objectStoreFake{temp: map[string]fakeObject{}, private: map[string]fakeObject{}}
}

func (o *objectStoreFake) put(b storage.Bucket, key string, data []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.gen++
	obj := fakeObject{data: data, etag: fmt.Sprintf("etag-%d", o.gen)}
	if b == storage.BucketTemp {
		o.temp[key] = obj
	} else {
		o.private[key] = obj
	}
}

func (o *objectStoreFake) bucket(b storage.Bucket) map[string]fakeObject {
	if b == storage.BucketTemp {
		return o.temp
	}
	return o.private
}

func (o *objectStoreFake) has(b storage.Bucket, key string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.bucket(b)[key]
	return ok
}

func (o *objectStoreFake) PresignUpload(_ context.Context, uploadKey string, maxBytes int64,
	contentType string, ttl time.Duration) (storage.PresignedPost, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.presigned = append(o.presigned, presignCall{uploadKey, maxBytes, contentType, ttl})
	return storage.PresignedPost{
		URL:       "https://s3.example.gov.vn/vigov-test-temp",
		Fields:    map[string]string{"key": uploadKey, "policy": "chinh-sach-gia", "x-amz-signature": "chu-ky-gia"},
		ExpiresAt: time.Date(2026, 9, 23, 8, 20, 0, 0, time.UTC),
	}, nil
}

func (o *objectStoreFake) Stat(_ context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.statErr != nil {
		return storage.ObjectInfo{}, o.statErr
	}
	obj, ok := o.bucket(b)[key]
	if !ok {
		return storage.ObjectInfo{}, fmt.Errorf("fake stat: %w", storage.ErrNotFound)
	}
	info := storage.ObjectInfo{Key: key, Size: int64(len(obj.data)), ETag: obj.etag}
	if o.replaceAfterStat && b == storage.BucketTemp {
		o.replaceAfterStat = false
		obj.etag += "-replaced"
		o.bucket(b)[key] = obj
	}
	return info, nil
}

func (o *objectStoreFake) get(b storage.Bucket, key, etag string) ([]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	obj, ok := o.bucket(b)[key]
	if !ok {
		return nil, fmt.Errorf("fake get: %w", storage.ErrNotFound)
	}
	if etag != "" && obj.etag != etag {
		return nil, fmt.Errorf("fake get: %w", storage.ErrChanged)
	}
	return obj.data, nil
}

func (o *objectStoreFake) ReadHead(_ context.Context, b storage.Bucket, key, etag string, n int) ([]byte, error) {
	d, err := o.get(b, key, etag)
	if err != nil {
		return nil, err
	}
	if len(d) > n {
		d = d[:n]
	}
	return d, nil
}

func (o *objectStoreFake) Open(_ context.Context, b storage.Bucket, key, etag string) (io.ReadCloser, int64, error) {
	d, err := o.get(b, key, etag)
	if err != nil {
		return nil, 0, err
	}
	return io.NopCloser(bytes.NewReader(d)), int64(len(d)), nil
}

func (o *objectStoreFake) SHA256(_ context.Context, b storage.Bucket, key, etag string) (string, error) {
	d, err := o.get(b, key, etag)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(d)
	return hex.EncodeToString(s[:]), nil
}

func (o *objectStoreFake) Promote(_ context.Context, src, etag string, dst storage.Key, b storage.Bucket) (
	storage.Promoted, error) {
	if b != storage.BucketPrivate {
		return storage.Promoted{}, storage.ErrInvalidArgument
	}
	d, err := o.get(storage.BucketTemp, src, etag)
	if err != nil {
		return storage.Promoted{}, err
	}
	mime, ext, ok := storage.SniffMIME(d)
	if !ok || ext != dst.Ext {
		return storage.Promoted{}, storage.ErrTypeNotAllowed
	}
	dstKey, err := dst.Path()
	if err != nil {
		return storage.Promoted{}, err
	}
	if "upload/"+dstKey != src {
		return storage.Promoted{}, fmt.Errorf("fake promote: %w: not the upload's own destination", storage.ErrInvalidArgument)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, taken := o.private[dstKey]; taken {
		return storage.Promoted{}, storage.ErrExists
	}
	o.gen++
	o.private[dstKey] = fakeObject{data: d, etag: fmt.Sprintf("etag-%d", o.gen)}
	delete(o.temp, src)
	o.promoted = append(o.promoted, dstKey)
	return storage.Promoted{Key: dstKey, Size: int64(len(d)), ContentType: mime}, nil
}

func (o *objectStoreFake) PresignDownload(_ context.Context, b storage.Bucket, key string, ttl time.Duration,
	filename string) (storage.PresignedURL, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if b != storage.BucketPrivate {
		return "", storage.ErrInvalidArgument
	}
	o.downloads = append(o.downloads, downloadCall{key, ttl, filename})
	return storage.PresignedURL("https://s3.example.gov.vn/vigov-test-private/" + key + "?X-Amz-Signature=gia"), nil
}

func (o *objectStoreFake) PurgeAllVersions(_ context.Context, b storage.Bucket, key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.purgeErr != nil {
		return o.purgeErr
	}
	o.purged = append(o.purged, key)
	delete(o.bucket(b), key)
	return nil
}

// --- the scanner and the policy --------------------------------------------------------------------

// scannerFake reads exactly what it is handed and answers the configured verdict. `scanned` is the
// bytes it saw, so a test can prove the scan covered the object that was then stored.
type scannerFake struct {
	res     malwarescan.Result
	err     error
	scanned []byte
	calls   int
}

func (s *scannerFake) Scan(_ context.Context, r io.Reader, size int64) (malwarescan.Result, error) {
	s.calls++
	if s.err != nil {
		return malwarescan.Result{}, s.err
	}
	d, err := io.ReadAll(r)
	if err != nil {
		return malwarescan.Result{}, err
	}
	if int64(len(d)) != size {
		return malwarescan.Result{}, malwarescan.ErrSizeMismatch
	}
	s.scanned = d
	return s.res, nil
}

type policyFake struct {
	p     uploadpolicy.Policy
	ok    bool
	err   error
	calls int
}

func (p *policyFake) Policy(_ context.Context, purpose storage.Purpose) (uploadpolicy.Policy, bool, error) {
	p.calls++
	if p.err != nil {
		return uploadpolicy.Policy{}, false, p.err
	}
	if purpose != storage.PurposeTaskAttachment {
		return uploadpolicy.Policy{}, false, nil
	}
	return p.p, p.ok, nil
}

// taskAttachmentPolicy is a CONFIGURED limit of the shape Vihat would set: 10 MB, PDF / JPEG / PNG,
// three files per task. Test data, not a default — the code under test holds no number.
func taskAttachmentPolicy() *policyFake {
	return &policyFake{ok: true, p: uploadpolicy.Policy{
		Purpose: storage.PurposeTaskAttachment, MaxBytes: 10 << 20,
		AllowedMIMETypes: []string{storage.MIMEPDF, storage.MIMEJPEG, storage.MIMEPNG},
		FileCountLimited: true, MaxFilesPerSubject: 3,
	}}
}
