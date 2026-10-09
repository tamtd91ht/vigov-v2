package app

// The cover acts (content_cover.go) and the cover half of Them / Sua, over the REAL content store on
// the fake driver of noi_dung_mini_app_test.go (transaction boundaries and the audit INSERT are real
// statements there) and in-memory fakes for the file rows and the object store — which is where the
// properties of this file live: what is scanned, what is promoted, what is made public and when.
//
// WHAT IT DOES NOT PROVE: MinIO, clamd, PostgreSQL. stored_file_guard, 0011's cover trigger and the
// CHECKs need a real server; the store's pg suites are where that lands the day a DSN exists.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// --- fakes ---------------------------------------------------------------------------------------

type fakeCoverFiles struct {
	rows        map[string]*domain.StoredFile
	inserted    []domain.StoredFile
	transitions []string
	publicSet   []string // "id=key", "id=" for a clear

	// publicSetRolledBack makes SetPublicObjectKey record the call but not keep the value — what a
	// transaction that rolls back leaves behind in PostgreSQL. The fake has no transactions of its own.
	publicSetRolledBack bool

	// deleted are the soft-deleted rows (SoftDelete): every read below skips them, like the store's
	// `deleted_at IS NULL`. softDeleted records each call, by id.
	deleted     map[string]bool
	softDeleted []string

	// admitCalls is the order of the admission calls (SubjectReservedBy, LockSubjectCount,
	// CountForSubjectTx, InsertPending), "<call>:<subject>" — the order IS the property for the count lock.
	admitCalls []string

	// issuedArticles are article ids a noi_dung_mini_app row carries, SOFT-DELETED ones included — the
	// store's NOT EXISTS in SubjectReservedBy. A live article is the content fake's (khoNDGia.dongHienCo).
	issuedArticles map[string]bool
}

// newFakeDB is the real core/store over the fake driver, one connection so statements stay ordered.
func newFakeDB(t *testing.T, k *khoNDGia) *store.DB {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

func newFakeCoverFiles() *fakeCoverFiles {
	return &fakeCoverFiles{rows: map[string]*domain.StoredFile{}, deleted: map[string]bool{}}
}

// get reads one LIVE row, as ByID / ForUpdate do: unknown and soft-deleted are both nil.
func (f *fakeCoverFiles) get(id string) *domain.StoredFile {
	r, ok := f.rows[id]
	if !ok || f.deleted[id] {
		return nil
	}
	c := *r
	return &c
}

// LiveForSubject mirrors the store: live AND ready rows only (liveForSubjectTail).
func (f *fakeCoverFiles) LiveForSubject(_ context.Context, subjectID, purpose string) ([]domain.StoredFile, error) {
	var out []domain.StoredFile
	for id, r := range f.rows {
		if !f.deleted[id] && r.SubjectID == subjectID && r.Purpose == purpose && r.Status == domain.StoredFileReady {
			out = append(out, *r)
		}
	}
	return out, nil
}
func (f *fakeCoverFiles) SubjectReservedBy(_ context.Context, _ *store.ScopedTx, subjectID, by string) (bool, error) {
	f.admitCalls = append(f.admitCalls, "reserved:"+subjectID)
	if f.issuedArticles[subjectID] {
		return false, nil
	}
	for id, r := range f.rows {
		if !f.deleted[id] && r.SubjectID == subjectID && r.UploadedBy == by {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeCoverFiles) SoftDelete(_ context.Context, _ *store.ScopedTx, id, by, _ string, _ time.Time) error {
	r := f.rows[id]
	if r == nil || f.deleted[id] || r.PublicObjectKey != "" || by == "" {
		return commsstore.ErrStoredFileMoved
	}
	f.deleted[id] = true
	f.softDeleted = append(f.softDeleted, id)
	return nil
}

// liveCount is the store's `max_files_per_subject` count (countForSubjectTail): live rows of the purpose
// on the subject that are past the scan, or pending (the fake has no clock for the TTL half).
func (f *fakeCoverFiles) liveCount(subjectID, purpose string, withPending bool) int {
	n := 0
	for id, r := range f.rows {
		if f.deleted[id] || r.SubjectID != subjectID || r.Purpose != purpose {
			continue
		}
		switch r.Status {
		case domain.StoredFileStored, domain.StoredFileProcessing, domain.StoredFileReady:
			n++
		case domain.StoredFilePending, domain.StoredFileScanning:
			if withPending {
				n++
			}
		}
	}
	return n
}

func (f *fakeCoverFiles) InsertPending(_ context.Context, _ *store.ScopedTx, r domain.StoredFile) error {
	f.admitCalls = append(f.admitCalls, "insert:"+r.SubjectID)
	f.inserted = append(f.inserted, r)
	c := r
	f.rows[r.ID] = &c
	return nil
}
func (f *fakeCoverFiles) ForUpdate(_ context.Context, _ *store.ScopedTx, id string) (*domain.StoredFile, error) {
	return f.get(id), nil
}
func (f *fakeCoverFiles) ByID(_ context.Context, id string) (*domain.StoredFile, error) {
	return f.get(id), nil
}
func (f *fakeCoverFiles) Transition(_ context.Context, _ *store.ScopedTx, id string, from, to domain.StoredFileStatus,
	_ time.Time) error {
	r := f.rows[id]
	if r == nil || r.Status != from || !from.CanMoveTo(to) {
		return commsstore.ErrStoredFileMoved
	}
	r.Status = to
	f.transitions = append(f.transitions, string(from)+">"+string(to))
	return nil
}
func (f *fakeCoverFiles) MarkStored(_ context.Context, _ *store.ScopedTx, id string, facts domain.StoredFileFacts,
	_ time.Time) error {
	r := f.rows[id]
	if r == nil || r.Status != domain.StoredFileScanning {
		return commsstore.ErrStoredFileMoved
	}
	r.Status, r.MIMEType, r.SizeBytes, r.SHA256 = domain.StoredFileStored, facts.MIMEType, facts.SizeBytes, facts.SHA256
	f.transitions = append(f.transitions, "scanning>stored")
	return nil
}
func (f *fakeCoverFiles) SetPublicObjectKey(_ context.Context, _ *store.ScopedTx, id, key string, _ time.Time) error {
	r := f.rows[id]
	if r == nil || r.Status != domain.StoredFileReady {
		return commsstore.ErrStoredFileMoved
	}
	if !f.publicSetRolledBack {
		r.PublicObjectKey = key
	}
	f.publicSet = append(f.publicSet, id+"="+key)
	return nil
}
func (f *fakeCoverFiles) PublicForSubject(_ context.Context, _ *store.ScopedTx, subjectID string) (
	[]domain.StoredFile, error) {
	var out []domain.StoredFile
	for id, r := range f.rows {
		if !f.deleted[id] && r.SubjectID == subjectID && r.PublicObjectKey != "" {
			out = append(out, *r)
		}
	}
	return out, nil
}
func (f *fakeCoverFiles) PublicObjectKeys(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if r := f.rows[id]; r != nil && r.PublicObjectKey != "" {
			out[id] = r.PublicObjectKey
		}
	}
	return out, nil
}
func (f *fakeCoverFiles) LockSubjectCount(_ context.Context, _ *store.ScopedTx, subjectID, purpose string) error {
	f.admitCalls = append(f.admitCalls, "lock:"+subjectID+":"+purpose)
	return nil
}
func (f *fakeCoverFiles) CountForSubjectTx(_ context.Context, _ *store.ScopedTx, subjectID, purpose string,
	_ time.Time) (int, error) {
	f.admitCalls = append(f.admitCalls, "count:"+subjectID)
	return f.liveCount(subjectID, purpose, true), nil
}
func (f *fakeCoverFiles) CountForSubject(_ context.Context, subjectID, purpose string) (int, error) {
	return f.liveCount(subjectID, purpose, false), nil
}

type fakeCoverObjects struct {
	temp, private map[string][]byte
	// putMax is the maxBytes PutUpload was given (the POLICY's); putErr fails the write after the stream
	// was read; putDeadline reports whether the write's context carried a deadline.
	putMax       int64
	putErr       error
	putDeadline  bool
	puts         int
	promoted     int
	produced     map[string][]byte
	purged       []string
	published    []string // public dst keys
	publishedSrc []string
	unpublished  []string
	publishErr   error
	unpublishErr error
}

func newFakeCoverObjects() *fakeCoverObjects {
	return &fakeCoverObjects{temp: map[string][]byte{}, private: map[string][]byte{}, produced: map[string][]byte{}}
}

func (o *fakeCoverObjects) bucket(b storage.Bucket) map[string][]byte {
	if b == storage.BucketTemp {
		return o.temp
	}
	return o.private
}

// PutUpload mirrors core/storage's contract: only `upload/…` keys, size ≤ maxBytes before a byte is read,
// exactly size bytes or ErrSizeMismatch, a reader error passed through wrapped.
func (o *fakeCoverObjects) PutUpload(ctx context.Context, uploadKey string, r io.Reader, size int64, maxBytes int64,
	contentType string) (storage.ObjectInfo, error) {
	o.puts++
	o.putMax = maxBytes
	_, o.putDeadline = ctx.Deadline()
	if !strings.HasPrefix(uploadKey, storage.TempUploadPrefix) {
		return storage.ObjectInfo{}, storage.ErrInvalidArgument
	}
	if size > maxBytes {
		return storage.ObjectInfo{}, storage.ErrTooLarge
	}
	d, err := io.ReadAll(r)
	if err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("storage: read: %w", err)
	}
	if int64(len(d)) != size {
		return storage.ObjectInfo{}, fmt.Errorf("%w: %w", storage.ErrInvalidArgument, storage.ErrSizeMismatch)
	}
	if o.putErr != nil {
		return storage.ObjectInfo{}, o.putErr
	}
	o.temp[uploadKey] = d
	return storage.ObjectInfo{Key: uploadKey, Size: size, ETag: "etag", ContentType: contentType}, nil
}
func (o *fakeCoverObjects) Stat(_ context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error) {
	d, ok := o.bucket(b)[key]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrNotFound
	}
	return storage.ObjectInfo{Key: key, Size: int64(len(d)), ETag: "etag"}, nil
}
func (o *fakeCoverObjects) ReadHead(_ context.Context, b storage.Bucket, key, _ string, n int) ([]byte, error) {
	d := o.bucket(b)[key]
	if len(d) > n {
		d = d[:n]
	}
	return d, nil
}
func (o *fakeCoverObjects) Open(_ context.Context, b storage.Bucket, key, _ string) (io.ReadCloser, int64, error) {
	d, ok := o.bucket(b)[key]
	if !ok {
		return nil, 0, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(d)), int64(len(d)), nil
}
func (o *fakeCoverObjects) SHA256(_ context.Context, b storage.Bucket, key, _ string) (string, error) {
	s := sha256.Sum256(o.bucket(b)[key])
	return hex.EncodeToString(s[:]), nil
}
func (o *fakeCoverObjects) Promote(_ context.Context, src, _ string, dst storage.Key, _ storage.Bucket) (
	storage.Promoted, error) {
	p, _ := dst.Path()
	o.private[p] = o.temp[src]
	delete(o.temp, src)
	o.promoted++
	mime, _, _ := storage.SniffMIME(o.private[p])
	return storage.Promoted{Key: p, Size: int64(len(o.private[p])), ETag: "etag", ContentType: mime}, nil
}
func (o *fakeCoverObjects) PutServerProduced(_ context.Context, dst storage.Key, r io.Reader, size int64) (
	storage.Produced, error) {
	p, _ := dst.Path()
	d, _ := io.ReadAll(r)
	if int64(len(d)) != size {
		return storage.Produced{}, storage.ErrInvalidArgument
	}
	o.produced[p], o.private[p] = d, d
	return storage.Produced{Key: p, Size: size}, nil
}
func (o *fakeCoverObjects) PresignDownload(_ context.Context, _ storage.Bucket, key string, _ time.Duration,
	_ string) (storage.PresignedURL, error) {
	return storage.PresignedURL("https://signed.example/" + key), nil
}
func (o *fakeCoverObjects) PurgeAllVersions(_ context.Context, b storage.Bucket, key string) error {
	delete(o.bucket(b), key)
	o.purged = append(o.purged, key)
	return nil
}
func (o *fakeCoverObjects) PublishDerivative(_ context.Context, src, dst storage.Key) error {
	if o.publishErr != nil {
		return o.publishErr
	}
	s, _ := src.Path()
	d, _ := dst.Path()
	o.publishedSrc, o.published = append(o.publishedSrc, s), append(o.published, d)
	return nil
}
func (o *fakeCoverObjects) UnpublishDerivative(_ context.Context, dst storage.Key) error {
	if o.unpublishErr != nil {
		return o.unpublishErr
	}
	d, _ := dst.Path()
	o.unpublished = append(o.unpublished, d)
	return nil
}
func (o *fakeCoverObjects) PublicURL(key string) (string, error) {
	return "https://media.example/" + key, nil
}

type fakeScanner struct {
	res     malwarescan.Result
	err     error
	scanned int
	// onScan runs during the scan: something that changes between the admission and the outcome.
	onScan func()
}

func (s *fakeScanner) Scan(_ context.Context, r io.Reader, _ int64) (malwarescan.Result, error) {
	s.scanned++
	if s.onScan != nil {
		s.onScan()
	}
	_, _ = io.Copy(io.Discard, r)
	return s.res, s.err
}

type fakePolicies struct {
	p   uploadpolicy.Policy
	ok  bool
	err error
}

func (p fakePolicies) Policy(context.Context, storage.Purpose) (uploadpolicy.Policy, bool, error) {
	return p.p, p.ok, p.err
}

// --- harness -------------------------------------------------------------------------------------

const (
	coverFileID  = "01JFFFFFFFFFFFFFFFFFFFFFFF"
	coverItemID  = "01JSSSSSSSSSSSSSSSSSSSSSSS"
	coverOtherID = "01JGGGGGGGGGGGGGGGGGGGGGGG"
)

var coverClock = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

// coverPolicy is the 01/10/2026 decision as platform serves it — the test reads the limit from the
// policy exactly like the use case does, never from a constant of its own.
func coverPolicy() fakePolicies {
	return fakePolicies{ok: true, p: uploadpolicy.Policy{Purpose: storage.PurposeContentImage, MaxBytes: 50 << 20,
		AllowedMIMETypes: []string{storage.MIMEJPEG, storage.MIMEPNG, storage.MIMEWebP}}}
}

type coverRig struct {
	k       *khoNDGia
	uc      *ContentCovers
	files   *fakeCoverFiles
	objects *fakeCoverObjects
	scanner *fakeScanner
	ctx     context.Context
}

func newCoverRig(t *testing.T, k *khoNDGia) *coverRig {
	t.Helper()
	if k == nil {
		k = &khoNDGia{}
	}
	db := newFakeDB(t, k)
	files, objects, scanner := newFakeCoverFiles(), newFakeCoverObjects(), &fakeScanner{res: malwarescan.Result{Clean: true}}
	uc := NewContentCovers(db, commsstore.NewNoiDungMiniAppStore(db), files, objects, scanner, coverPolicy())
	ids := []string{coverFileID, coverItemID}
	uc.newID = func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil }
	uc.now = func() time.Time { return coverClock }
	return &coverRig{k: k, uc: uc, files: files, objects: objects, scanner: scanner,
		ctx: tenant.Into(context.Background(), xaA)}
}

// streamOf is the file part as the handler hands it over: exactly data, then a Finish that reports what
// followed the file (nil = nothing).
func streamOf(data []byte, finishErr error) (io.Reader, func() error) {
	return bytes.NewReader(data), func() error { return finishErr }
}

// coverReq is one cover upload of data declared as mime, for item ("" = a fresh reservation).
func coverReq(item, mime string, data []byte) CoverUploadRequest {
	file, finish := streamOf(data, nil)
	return CoverUploadRequest{ContentItemID: item, FileName: "C:\\fakepath\\trao-qua.jpg", ContentType: mime,
		Size: int64(len(data)), File: file, Finish: finish}
}

// withStream gives a declaration-only request a stream of its declared size (zero bytes: refused by the
// sniff if the request ever gets that far — the tests using it stop at admission or assert the refusal).
func withStream(req CoverUploadRequest) CoverUploadRequest {
	if req.File == nil && req.Size > 0 && req.Size < 1<<20 {
		req.File, req.Finish = streamOf(make([]byte, req.Size), nil)
	} else if req.File == nil {
		req.File, req.Finish = streamOf(nil, nil)
	}
	return req
}

// upload runs the real UploadCover with data as the file part.
func (r *coverRig) upload(data []byte) (domain.StoredFile, error) {
	return r.uc.UploadCover(r.ctx, coverReq("", storage.MIMEJPEG, data), nguoiSoanND())
}

// row is the one row this rig's upload wrote (coverFileID).
func (r *coverRig) row(t *testing.T) *domain.StoredFile {
	t.Helper()
	f := r.files.get(coverFileID)
	if f == nil {
		t.Fatal("no row was written")
	}
	return f
}

// --- a. admission ----------------------------------------------------------------------------------

func TestCoverUploadRefusesWhatThePolicyRefusesBeforeAnyRowOrByte(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  CoverUploadRequest
		want error
	}{
		{"over the policy size", CoverUploadRequest{FileName: "a.jpg", ContentType: storage.MIMEJPEG, Size: 50<<20 + 1}, ErrCoverTooLarge},
		{"HEIC is not in the policy", CoverUploadRequest{FileName: "a.heic", ContentType: storage.MIMEHEIC, Size: 10}, ErrCoverTypeNotAllowed},
		{"declared PDF", CoverUploadRequest{FileName: "a.pdf", ContentType: storage.MIMEPDF, Size: 10}, ErrCoverTypeNotAllowed},
		{"no size", CoverUploadRequest{FileName: "a.jpg", ContentType: storage.MIMEJPEG}, domain.ErrCoverSizeInvalid},
		{"no name", CoverUploadRequest{ContentType: storage.MIMEJPEG, Size: 10}, domain.ErrCoverFileNameInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newCoverRig(t, nil)
			if _, err := r.uc.UploadCover(r.ctx, withStream(tc.req), nguoiSoanND()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if r.k.batDau != 0 || len(r.files.inserted) != 0 || r.objects.puts != 0 {
				t.Errorf("a refused declaration opened a transaction (%d), wrote a row (%d) or read the file (%d)",
					r.k.batDau, len(r.files.inserted), r.objects.puts)
			}
		})
	}
}

func TestCoverUploadFailsClosedWithoutPolicyOrStorage(t *testing.T) {
	r := newCoverRig(t, nil)
	r.uc.policies = fakePolicies{ok: false}
	req := withStream(CoverUploadRequest{FileName: "a.jpg", ContentType: storage.MIMEJPEG, Size: 10})
	if _, err := r.uc.UploadCover(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no policy: err = %v", err)
	}
	if _, err := r.uc.CoverUploadLimit(r.ctx); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no policy, limit: err = %v", err)
	}
	r.uc.policies = fakePolicies{err: uploadpolicy.ErrUnavailable}
	if _, err := r.uc.UploadCover(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverLimitsUnavailable) {
		t.Errorf("platform down: err = %v", err)
	}
	if _, err := r.uc.CoverUploadLimit(r.ctx); !errors.Is(err, ErrCoverLimitsUnavailable) {
		t.Errorf("platform down, limit: err = %v", err)
	}
	r.uc.objects = nil
	if _, err := r.uc.UploadCover(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no object store: err = %v", err)
	}
	if _, err := r.uc.BodyImageUploadLimit(r.ctx); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no object store, body-image limit: err = %v", err)
	}
}

// The handler caps the request body at the POLICY's size, read here — never a constant.
func TestCoverUploadLimitIsThePolicys(t *testing.T) {
	r := newCoverRig(t, nil)
	if n, err := r.uc.CoverUploadLimit(r.ctx); err != nil || n != 50<<20 {
		t.Fatalf("limit = %d, %v; want the policy's 50 MiB", n, err)
	}
	p := coverPolicy()
	p.p.MaxBytes = 0
	r.uc.policies = p
	if _, err := r.uc.CoverUploadLimit(r.ctx); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("a policy with no cap must refuse, never mean unbounded: %v", err)
	}
}

func TestCoverUploadWritesTheRequestedRowBeforeTheBytesAndTheTrailWithoutTheName(t *testing.T) {
	r := newCoverRig(t, nil)
	f, err := r.upload(testJPEG(t, 40, 30, 0))
	if err != nil {
		t.Fatalf("UploadCover: %v", err)
	}
	// Two transactions: the pending row + its trail, then the outcome + its trail.
	if r.k.batDau != 2 || r.k.daCommit != 2 || len(r.k.cau("INSERT INTO audit_log")) != 2 {
		t.Fatalf("tx: begin=%d commit=%d audit=%d", r.k.batDau, r.k.daCommit, len(r.k.cau("INSERT INTO audit_log")))
	}
	if !containsString(auditActions(r.k), ActionCoverUploadRequested) || !containsString(auditActions(r.k), ActionCoverStored) {
		t.Errorf("trail = %v", auditActions(r.k))
	}
	if f.SubjectID != coverItemID || f.UploadedBy != maCanBoSoanND || f.OriginalName != "trao-qua.jpg" {
		t.Errorf("row = %+v", f)
	}
	want := "content-source/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/original.jpg"
	if f.ObjectKey != want {
		t.Errorf("object key = %q, want %q", f.ObjectKey, want)
	}
	if r.objects.putMax != 50<<20 {
		t.Errorf("write limit = %d, want the POLICY's", r.objects.putMax)
	}
	// The original file name is personal data: never in the trail.
	for _, l := range r.k.cau("INSERT INTO audit_log") {
		for _, a := range l.args {
			if s, ok := a.(string); ok && strings.Contains(s, "trao-qua") {
				t.Errorf("the trail carries the file name: %s", s)
			}
		}
	}
}

func TestCoverUploadForMissingItemIs404AndWritesNothing(t *testing.T) {
	r := newCoverRig(t, &khoNDGia{}) // TheoIDDeSua finds nothing
	_, err := r.uc.UploadCover(r.ctx, coverReq("nd-khac", storage.MIMEJPEG, testJPEG(t, 10, 10, 0)), nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		t.Fatalf("err = %v", err)
	}
	if len(r.files.inserted) != 0 || r.k.daRollback != 1 || r.objects.puts != 0 {
		t.Errorf("inserted=%d rollback=%d puts=%d", len(r.files.inserted), r.k.daRollback, r.objects.puts)
	}
}

// --- b. the stream ---------------------------------------------------------------------------------

// A FAILED STREAM NEVER LEAVES A PENDING ROW: nothing can complete it any more (no completion route),
// and a pending row holds a slot of the article's count. `failed`, one abandoned entry, the cause kept.
func TestCoverStreamFailureMarksTheRowFailedWithItsTrail(t *testing.T) {
	brokenRead := errors.New("client hung up")
	for _, tc := range []struct {
		name  string
		setup func(r *coverRig, req *CoverUploadRequest)
		cause error
	}{
		{"the client's stream breaks", func(_ *coverRig, req *CoverUploadRequest) {
			req.File = io.MultiReader(bytes.NewReader([]byte{1, 2}), iotestErrReader{brokenRead})
		}, brokenRead},
		{"shorter than declared", func(_ *coverRig, req *CoverUploadRequest) { req.Size++ }, storage.ErrSizeMismatch},
		{"something after the file", func(_ *coverRig, req *CoverUploadRequest) {
			req.Finish = func() error { return brokenRead }
		}, brokenRead},
		{"the temp bucket refuses", func(r *coverRig, _ *CoverUploadRequest) {
			r.objects.putErr = errors.New("storage: put upload: bucket \"vigov-temp\" does not exist")
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newCoverRig(t, nil)
			req := coverReq("", storage.MIMEJPEG, testJPEG(t, 10, 10, 0))
			tc.setup(r, &req)
			_, err := r.uc.UploadCover(r.ctx, req, nguoiSoanND())
			if !errors.Is(err, ErrCoverUploadIncomplete) || (tc.cause != nil && !errors.Is(err, tc.cause)) {
				t.Fatalf("err = %v, want ErrCoverUploadIncomplete wrapping %v", err, tc.cause)
			}
			if st := r.row(t).Status; st != domain.StoredFileFailed {
				t.Errorf("status = %s, want failed", st)
			}
			if r.scanner.scanned != 0 || r.objects.promoted != 0 {
				t.Error("an incomplete upload reached the scanner or the private bucket")
			}
			acts := auditActions(r.k)
			if !containsString(acts, ActionCoverExpired) || !auditCarries(r.k.cau("INSERT INTO audit_log")[1], CoverAbandonNotReceived) {
				t.Errorf("trail = %v, want the abandoned entry with %q", acts, CoverAbandonNotReceived)
			}
		})
	}
}

// The write runs under the upload's deadline (httpx's 180 s), never unbounded.
func TestCoverWriteRunsUnderTheUploadDeadline(t *testing.T) {
	r := newCoverRig(t, nil)
	req := coverReq("", storage.MIMEJPEG, testJPEG(t, 10, 10, 0))
	req.Deadline = time.Now().Add(time.Minute)
	if _, err := r.uc.UploadCover(r.ctx, req, nguoiSoanND()); err != nil {
		t.Fatal(err)
	}
	if !r.objects.putDeadline {
		t.Error("PutUpload ran without the upload's deadline")
	}
}

// A client that hung up cancels the request's context: the row must still leave `pending`.
func TestCoverAbandonSurvivesACancelledRequest(t *testing.T) {
	r := newCoverRig(t, nil)
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	req := coverReq("", storage.MIMEJPEG, testJPEG(t, 10, 10, 0))
	req.File = readerFunc(func([]byte) (int, error) { cancel(); return 0, context.Canceled })
	if _, err := r.uc.UploadCover(ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverUploadIncomplete) {
		t.Fatalf("err = %v", err)
	}
	if st := r.row(t).Status; st != domain.StoredFileFailed {
		t.Errorf("status = %s, want failed even though the request was cancelled", st)
	}
}

// --- c. the completion -----------------------------------------------------------------------------

func TestCoverUploadCleanProducesDerivativeAndIsReady(t *testing.T) {
	r := newCoverRig(t, nil)
	got, err := r.upload(testJPEG(t, 2000, 1500, 0))
	if err != nil {
		t.Fatalf("UploadCover: %v", err)
	}
	if got.Status != domain.StoredFileReady || r.scanner.scanned != 1 || r.objects.promoted != 1 {
		t.Fatalf("status=%s scanned=%d promoted=%d", got.Status, r.scanner.scanned, r.objects.promoted)
	}
	want := []string{"pending>scanning", "scanning>stored", "stored>processing", "processing>ready"}
	if strings.Join(r.files.transitions, ",") != strings.Join(want, ",") {
		t.Errorf("transitions = %v, want %v", r.files.transitions, want)
	}
	deriv := strings.Replace(got.ObjectKey, "/original.jpg", "/thumb-1280.jpg", 1)
	out, ok := r.objects.produced[deriv]
	if !ok {
		t.Fatalf("no derivative at %q: %v", deriv, r.objects.produced)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 1280 || cfg.Height != 960 {
		t.Errorf("derivative %v %d×%d, want 1280×960", err, cfg.Width, cfg.Height)
	}
	if len(r.objects.published) != 0 {
		t.Error("uploading must not publish anything")
	}
}

func TestCoverUploadInfectedIsRejectedNeverStored(t *testing.T) {
	r := newCoverRig(t, nil)
	r.scanner.res = malwarescan.Result{Clean: false, Signature: "Eicar-Test-Signature"}

	_, err := r.upload(testJPEG(t, 10, 10, 0))
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectMalware {
		t.Fatalf("err = %v, want malware rejection", err)
	}
	if r.objects.promoted != 0 || len(r.objects.produced) != 0 {
		t.Error("an infected file reached the private bucket")
	}
	if len(r.objects.purged) != 1 || r.row(t).Status != domain.StoredFileRejected {
		t.Errorf("temp purged=%v status=%s", r.objects.purged, r.row(t).Status)
	}
	if containsString(auditActions(r.k), ActionCoverExpired) {
		t.Error("a rejection is not an abandoned upload")
	}
}

// UNSCANNABLE IS NEVER CLEAN (ADR 0052 §9) — and, with no completion route left to retry, the row is
// abandoned rather than left pending: `failed`, the cause returned for the handler's 503.
func TestCoverUploadUnscannableIsNeverTreatedClean(t *testing.T) {
	r := newCoverRig(t, nil)
	r.scanner.err = malwarescan.ErrUnavailable

	if _, err := r.upload(testJPEG(t, 10, 10, 0)); !errors.Is(err, ErrCoverScanUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if r.objects.promoted != 0 || r.row(t).Status != domain.StoredFileFailed {
		t.Errorf("scanner down: promoted=%d status=%s — never stored, never left pending",
			r.objects.promoted, r.row(t).Status)
	}
	audits := r.k.cau("INSERT INTO audit_log")
	if len(audits) != 2 || !auditCarries(audits[1], CoverAbandonNotInspected) {
		t.Errorf("trail = %v, want requested + abandoned (%s)", auditActions(r.k), CoverAbandonNotInspected)
	}
}

// seqPolicies answers its policies in turn, then the last one: a policy that changes between the
// admission and the completion's own read.
type seqPolicies struct {
	ps []fakePolicies
	n  int
}

func (s *seqPolicies) Policy(ctx context.Context, p storage.Purpose) (uploadpolicy.Policy, bool, error) {
	i := min(s.n, len(s.ps)-1)
	s.n++
	return s.ps[i].Policy(ctx, p)
}

func TestCoverCompletionTightenedPolicyRejectsOversize(t *testing.T) {
	r := newCoverRig(t, nil)
	tight := coverPolicy()
	tight.p.MaxBytes = 10 // tightened between admission and inspection: the CURRENT policy decides
	r.uc.policies = &seqPolicies{ps: []fakePolicies{coverPolicy(), tight}}
	_, err := r.upload(testJPEG(t, 10, 10, 0))
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectTooLarge || r.scanner.scanned != 0 {
		t.Fatalf("err = %v scanned = %d", err, r.scanner.scanned)
	}
}

func TestCoverUploadUndecodableFailsAfterScan(t *testing.T) {
	r := newCoverRig(t, nil)
	_, err := r.upload(append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{7}, 64)...))
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectUndecodable {
		t.Fatalf("err = %v", err)
	}
	if st := r.row(t).Status; st != domain.StoredFileFailed {
		t.Errorf("status = %s, want failed", st)
	}
}

// The completion re-reads the row and checks it is THIS officer's upload of THIS purpose — the guard it
// had as a route of its own, kept for a caller that would pass it another id.
func TestCoverCompletionOfAnotherOfficersOrPurposesRowIs404(t *testing.T) {
	r := newCoverRig(t, nil)
	f := readyCover(r.files, coverFileID, coverItemID)
	f.Status, f.Purpose = domain.StoredFilePending, string(coverPurpose)

	other := nguoiSoanND()
	other.ID = "CB-2026-KHAC00"
	if _, err := r.uc.complete(r.ctx, coverPurpose, f.ID, other); !errors.Is(err, ErrCoverFileNotFound) {
		t.Fatalf("another officer: err = %v", err)
	}
	if _, err := r.uc.complete(r.ctx, bodyImagePurpose, f.ID, nguoiSoanND()); !errors.Is(err, ErrCoverFileNotFound) {
		t.Fatalf("another purpose: err = %v", err)
	}
	if r.scanner.scanned != 0 {
		t.Error("a foreign completion reached the scanner")
	}
}

// iotestErrReader fails every read with err.
type iotestErrReader struct{ err error }

func (e iotestErrReader) Read([]byte) (int, error) { return 0, e.err }

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

// --- publishing with the article -----------------------------------------------------------------

// readyCover puts a ready cover row for item into the fake, as a completion would have left it.
func readyCover(files *fakeCoverFiles, id, item string) *domain.StoredFile {
	f := &domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate,
		ObjectKey:      "content-source/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" + strings.ToLower(id) + "/original.png",
		RetentionClass: string(storage.ClassContentSource), Purpose: string(storage.PurposeContentImage),
		SubjectType: domain.StoredFileSubjectContentItem, SubjectID: item, Status: domain.StoredFileReady,
		UploadedBy: maCanBoSoanND}
	files.rows[id] = f
	return f
}

func soanWithCovers(t *testing.T, k *khoNDGia) (*SoanNoiDungMiniApp, *fakeCoverFiles, *fakeCoverObjects, context.Context) {
	t.Helper()
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	files, objects := newFakeCoverFiles(), newFakeCoverObjects()
	uc.WithCovers(files, objects, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return uc, files, objects, ctx
}

func TestPublishingWithCoverPublishesTheDerivativeInTheTransaction(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyCover(files, coverFileID, coverItemID)

	publish := true
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &publish}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if len(objects.published) != 1 {
		t.Fatalf("published = %v", objects.published)
	}
	// NEVER THE ORIGINAL: the source is the thumb-1280 derivative, the destination its public twin.
	if !strings.HasSuffix(objects.publishedSrc[0], "/thumb-1280.jpg") || strings.Contains(objects.publishedSrc[0], "original") ||
		!strings.HasPrefix(objects.published[0], "public-media/") {
		t.Errorf("published %q -> %q", objects.publishedSrc[0], objects.published[0])
	}
	if len(files.publicSet) != 1 || !strings.HasSuffix(files.publicSet[0], objects.published[0]) {
		t.Errorf("public key recorded = %v", files.publicSet)
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit=%d rollback=%d", k.daCommit, k.daRollback)
	}
}

func TestUnpublishingWithdrawsThePublicCopyAfterCommit(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiDangHien, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID, PublishedAt: lucNDPinned}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	f := readyCover(files, coverFileID, coverItemID)
	f.PublicObjectKey = "public-media/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/thumb-1280.jpg"

	wantKey := f.PublicObjectKey // the fake row is cleared by the withdrawal itself
	hide := false
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &hide}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if len(objects.unpublished) != 1 || objects.unpublished[0] != wantKey {
		t.Fatalf("unpublished = %v, want [%s]", objects.unpublished, wantKey)
	}
	if len(objects.published) != 0 {
		t.Error("hiding an article must not publish anything")
	}
	// Two transactions: the article (trail inside), then the key cleared with its own trail.
	if k.daCommit != 2 || files.get(coverFileID).PublicObjectKey != "" {
		t.Errorf("commits=%d key=%q", k.daCommit, files.get(coverFileID).PublicObjectKey)
	}
	if n := len(k.cau("INSERT INTO audit_log")); n != 2 {
		t.Errorf("audit entries = %d, want 2 (edit, withdrawal)", n)
	}
}

func TestFailedWithdrawalKeepsTheKeyForTheNextEdit(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiDangHien, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID, PublishedAt: lucNDPinned}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	f := readyCover(files, coverFileID, coverItemID)
	f.PublicObjectKey = "public-media/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/thumb-1280.jpg"
	objects.unpublishErr = errors.New("minio down")

	hide := false
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &hide}, nguoiSoanND()); err != nil {
		t.Fatalf("hiding must succeed whatever MinIO does: %v", err)
	}
	if files.get(coverFileID).PublicObjectKey == "" {
		t.Error("a failed withdrawal cleared the key — nothing would retry it")
	}
}

func TestPublishCopyFailureRefusesAndWritesNothing(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyCover(files, coverFileID, coverItemID)
	objects.publishErr = errors.New("minio down")

	publish := true
	_, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &publish}, nguoiSoanND())
	if !errors.Is(err, ErrCoverPublishUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if k.daCommit != 0 || k.daRollback != 1 || len(files.publicSet) != 0 {
		t.Errorf("commit=%d rollback=%d publicSet=%v", k.daCommit, k.daRollback, files.publicSet)
	}
}

func TestRollbackAfterCopyWithdrawsTheCopy(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID}
	k := &khoNDGia{dongHienCo: &item, loiSau: "INSERT INTO audit_log"}
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyCover(files, coverFileID, coverItemID)
	files.publicSetRolledBack = true // the audit INSERT fails, so the key write rolls back with it

	publish := true
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &publish}, nguoiSoanND()); err == nil {
		t.Fatal("the audit failure must fail the edit")
	}
	if k.daRollback != 1 || len(objects.published) != 1 {
		t.Fatalf("rollback=%d published=%v", k.daRollback, objects.published)
	}
	// The compensation ran inside Sua: the copy made for a rolled-back edit is withdrawn.
	if len(objects.unpublished) != 1 {
		t.Errorf("the copy made for a rolled-back edit was not withdrawn: %v", objects.unpublished)
	}
}

func TestReplacingThePublishedCoverSwapsTheCopies(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiDangHien, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID, PublishedAt: lucNDPinned}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	old := readyCover(files, coverFileID, coverItemID)
	old.PublicObjectKey = "public-media/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/thumb-1280.jpg"
	readyCover(files, coverOtherID, coverItemID)

	oldKey := old.PublicObjectKey // the fake row is cleared by the withdrawal itself
	next := coverOtherID
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{CoverImageFileID: &next}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if len(objects.published) != 1 || !strings.Contains(objects.published[0], strings.ToLower(coverOtherID)) {
		t.Errorf("new cover not published: %v", objects.published)
	}
	if len(objects.unpublished) != 1 || objects.unpublished[0] != oldKey {
		t.Errorf("old cover not withdrawn: %v", objects.unpublished)
	}
	upd := k.cau("UPDATE noi_dung_mini_app")
	if len(upd) != 1 || upd[0].args[15] != coverOtherID {
		t.Errorf("cover_image_file_id not written: %v", upd)
	}
}

func TestAttachingAnotherArticlesFileIsRefused(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, _, ctx := soanWithCovers(t, k)
	readyCover(files, coverOtherID, "01JHHHHHHHHHHHHHHHHHHHHHHH") // uploaded for ANOTHER article

	id := coverOtherID
	_, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{CoverImageFileID: &id}, nguoiSoanND())
	if !errors.Is(err, domain.ErrCoverNotUsable) {
		t.Fatalf("err = %v", err)
	}
	if k.coCau("UPDATE noi_dung_mini_app") || k.daCommit != 0 {
		t.Error("a refused cover wrote the article")
	}
}

func TestCreateWithCoverTakesTheUploadsReservedID(t *testing.T) {
	k := &khoNDGia{} // no row under the reserved id
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyCover(files, coverFileID, coverItemID)

	yc := ycThemMau()
	yc.CoverImageFileID, yc.DangLenMiniApp = coverFileID, true
	moi, err := uc.Them(ctx, yc, nguoiSoanND())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if moi.ID != coverItemID {
		t.Errorf("id = %q, want the upload's reserved %q (never a client value, never a fresh one)", moi.ID, coverItemID)
	}
	ins := k.cau("INSERT INTO noi_dung_mini_app")
	if len(ins) != 1 || ins[0].args[1] != coverItemID || ins[0].args[16] != coverFileID {
		t.Fatalf("insert args = %v", ins)
	}
	if len(objects.published) != 1 || len(files.publicSet) != 1 {
		t.Errorf("born published with a cover: published=%v publicSet=%v", objects.published, files.publicSet)
	}
}

func TestCreateWithCoverOfAnExistingArticleIsRefused(t *testing.T) {
	existing := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Đã có", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND}
	k := &khoNDGia{dongHienCo: &existing}
	uc, files, _, ctx := soanWithCovers(t, k)
	readyCover(files, coverFileID, coverItemID)

	yc := ycThemMau()
	yc.CoverImageFileID = coverFileID
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrCoverNotUsable) {
		t.Fatalf("err = %v", err)
	}
	if k.coCau("INSERT INTO noi_dung_mini_app") {
		t.Error("a second article was created under an existing id")
	}
}

func TestPublicImageURLsOnlyForRecordedCopies(t *testing.T) {
	r := newCoverRig(t, nil)
	pub := readyCover(r.files, coverFileID, coverItemID)
	pub.PublicObjectKey = "public-media/t_x/2026/10/comms/content-image/f/thumb-1280.jpg"
	readyCover(r.files, coverOtherID, coverItemID) // ready, never published
	got, err := r.uc.PublicImageURLs(r.ctx, []string{coverFileID, coverOtherID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[coverFileID] != "https://media.example/"+pub.PublicObjectKey {
		t.Errorf("urls = %v", got)
	}
	r.uc.objects = nil
	if got, _ := r.uc.PublicImageURLs(r.ctx, []string{coverFileID}); len(got) != 0 {
		t.Error("without object storage no URL may be built")
	}
}
