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
	return &fakeCoverFiles{rows: map[string]*domain.StoredFile{}}
}

func (f *fakeCoverFiles) get(id string) *domain.StoredFile {
	r, ok := f.rows[id]
	if !ok {
		return nil
	}
	c := *r
	return &c
}

func (f *fakeCoverFiles) InsertPending(_ context.Context, _ *store.ScopedTx, r domain.StoredFile) error {
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
	for _, r := range f.rows {
		if r.SubjectID == subjectID && r.PublicObjectKey != "" {
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
func (f *fakeCoverFiles) CountForSubjectTx(context.Context, *store.ScopedTx, string, string, time.Time) (int, error) {
	return 0, nil
}
func (f *fakeCoverFiles) CountForSubject(context.Context, string, string) (int, error) { return 0, nil }

type fakeCoverObjects struct {
	temp, private map[string][]byte
	presignMax    int64
	promoted      int
	produced      map[string][]byte
	purged        []string
	published     []string // public dst keys
	publishedSrc  []string
	unpublished   []string
	publishErr    error
	unpublishErr  error
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

func (o *fakeCoverObjects) PresignUpload(_ context.Context, uploadKey string, maxBytes int64, _ string,
	_ time.Duration) (storage.PresignedPost, error) {
	o.presignMax = maxBytes
	return storage.PresignedPost{URL: "https://minio.example/temp", Fields: map[string]string{"key": uploadKey}}, nil
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
}

func (s *fakeScanner) Scan(_ context.Context, r io.Reader, _ int64) (malwarescan.Result, error) {
	s.scanned++
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

// pendingUpload issues an upload through the real RequestUpload and drops bytes into the temp bucket
// at the key the browser would POST to.
func (r *coverRig) pendingUpload(t *testing.T, data []byte) domain.StoredFile {
	t.Helper()
	up, err := r.uc.RequestUpload(r.ctx, CoverUploadRequest{FileName: "C:\\fakepath\\trao-qua.jpg",
		ContentType: storage.MIMEJPEG, Size: int64(len(data))}, nguoiSoanND())
	if err != nil {
		t.Fatalf("RequestUpload: %v", err)
	}
	r.objects.temp[up.Post.Fields["key"]] = data
	return up.File
}

// --- a. request ----------------------------------------------------------------------------------

func TestCoverRequestRefusesWhatThePolicyRefusesBeforeAnyRow(t *testing.T) {
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
			if _, err := r.uc.RequestUpload(r.ctx, tc.req, nguoiSoanND()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if r.k.batDau != 0 || len(r.files.inserted) != 0 {
				t.Errorf("a refused declaration opened a transaction (%d) or wrote a row (%d)", r.k.batDau, len(r.files.inserted))
			}
		})
	}
}

func TestCoverRequestFailsClosedWithoutPolicyOrStorage(t *testing.T) {
	r := newCoverRig(t, nil)
	r.uc.policies = fakePolicies{ok: false}
	req := CoverUploadRequest{FileName: "a.jpg", ContentType: storage.MIMEJPEG, Size: 10}
	if _, err := r.uc.RequestUpload(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no policy: err = %v", err)
	}
	r.uc.policies = fakePolicies{err: uploadpolicy.ErrUnavailable}
	if _, err := r.uc.RequestUpload(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverLimitsUnavailable) {
		t.Errorf("platform down: err = %v", err)
	}
	r.uc.objects = nil
	if _, err := r.uc.RequestUpload(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrCoverUploadNotConfigured) {
		t.Errorf("no object store: err = %v", err)
	}
}

func TestCoverRequestWritesRowAndTrailInOneTransaction(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, []byte("whatever"))
	if r.k.batDau != 1 || r.k.daCommit != 1 || len(r.k.cau("INSERT INTO audit_log")) != 1 {
		t.Fatalf("tx: begin=%d commit=%d audit=%d", r.k.batDau, r.k.daCommit, len(r.k.cau("INSERT INTO audit_log")))
	}
	if f.SubjectID != coverItemID || f.UploadedBy != maCanBoSoanND || f.OriginalName != "trao-qua.jpg" {
		t.Errorf("row = %+v", f)
	}
	want := "content-source/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/original.jpg"
	if f.ObjectKey != want {
		t.Errorf("object key = %q, want %q", f.ObjectKey, want)
	}
	if r.objects.presignMax != 50<<20 {
		t.Errorf("presigned limit = %d, want the POLICY's", r.objects.presignMax)
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

func TestCoverRequestForMissingItemIs404AndWritesNothing(t *testing.T) {
	r := newCoverRig(t, &khoNDGia{}) // TheoIDDeSua finds nothing
	_, err := r.uc.RequestUpload(r.ctx, CoverUploadRequest{ContentItemID: "nd-khac", FileName: "a.jpg",
		ContentType: storage.MIMEJPEG, Size: 10}, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		t.Fatalf("err = %v", err)
	}
	if len(r.files.inserted) != 0 || r.k.daRollback != 1 {
		t.Errorf("inserted=%d rollback=%d", len(r.files.inserted), r.k.daRollback)
	}
}

// --- c. complete ---------------------------------------------------------------------------------

func TestCoverCompleteCleanProducesDerivativeAndIsReady(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, testJPEG(t, 2000, 1500, 0))
	r.k.lenh = nil

	got, err := r.uc.Complete(r.ctx, f.ID, nguoiSoanND())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got.Status != domain.StoredFileReady || r.scanner.scanned != 1 || r.objects.promoted != 1 {
		t.Fatalf("status=%s scanned=%d promoted=%d", got.Status, r.scanner.scanned, r.objects.promoted)
	}
	want := []string{"pending>scanning", "scanning>stored", "stored>processing", "processing>ready"}
	if strings.Join(r.files.transitions, ",") != strings.Join(want, ",") {
		t.Errorf("transitions = %v, want %v", r.files.transitions, want)
	}
	deriv := strings.Replace(f.ObjectKey, "/original.jpg", "/thumb-1280.jpg", 1)
	out, ok := r.objects.produced[deriv]
	if !ok {
		t.Fatalf("no derivative at %q: %v", deriv, r.objects.produced)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 1280 || cfg.Height != 960 {
		t.Errorf("derivative %v %d×%d, want 1280×960", err, cfg.Width, cfg.Height)
	}
	if len(r.k.cau("INSERT INTO audit_log")) != 1 || r.k.daCommit != 2 {
		t.Errorf("completion audit=%d commits=%d", len(r.k.cau("INSERT INTO audit_log")), r.k.daCommit)
	}
	if len(r.objects.published) != 0 {
		t.Error("completing an upload must not publish anything")
	}
}

func TestCoverCompleteInfectedIsRejectedNeverStored(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, testJPEG(t, 10, 10, 0))
	r.scanner.res = malwarescan.Result{Clean: false, Signature: "Eicar-Test-Signature"}

	_, err := r.uc.Complete(r.ctx, f.ID, nguoiSoanND())
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectMalware {
		t.Fatalf("err = %v, want malware rejection", err)
	}
	if r.objects.promoted != 0 || len(r.objects.produced) != 0 {
		t.Error("an infected file reached the private bucket")
	}
	if len(r.objects.purged) != 1 || r.files.get(f.ID).Status != domain.StoredFileRejected {
		t.Errorf("temp purged=%v status=%s", r.objects.purged, r.files.get(f.ID).Status)
	}
}

func TestCoverCompleteUnscannableIsNeverTreatedClean(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, testJPEG(t, 10, 10, 0))
	r.scanner.err = malwarescan.ErrUnavailable
	before := r.k.batDau

	if _, err := r.uc.Complete(r.ctx, f.ID, nguoiSoanND()); !errors.Is(err, ErrCoverScanUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if r.objects.promoted != 0 || r.k.batDau != before || r.files.get(f.ID).Status != domain.StoredFilePending {
		t.Errorf("scanner down: promoted=%d tx=%d status=%s — must write nothing and stay retryable",
			r.objects.promoted, r.k.batDau-before, r.files.get(f.ID).Status)
	}
}

func TestCoverCompleteTightenedPolicyRejectsOversize(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, testJPEG(t, 10, 10, 0))
	p := coverPolicy()
	p.p.MaxBytes = 10 // tightened since the request: the CURRENT policy decides
	r.uc.policies = p
	_, err := r.uc.Complete(r.ctx, f.ID, nguoiSoanND())
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectTooLarge || r.scanner.scanned != 0 {
		t.Fatalf("err = %v scanned = %d", err, r.scanner.scanned)
	}
}

func TestCoverCompleteUndecodableFailsAfterScan(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{7}, 64)...))
	_, err := r.uc.Complete(r.ctx, f.ID, nguoiSoanND())
	var rej *CoverRejection
	if !errors.As(err, &rej) || rej.Reason != CoverRejectUndecodable {
		t.Fatalf("err = %v", err)
	}
	if st := r.files.get(f.ID).Status; st != domain.StoredFileFailed {
		t.Errorf("status = %s, want failed", st)
	}
}

func TestCoverCompleteByAnotherOfficerIs404(t *testing.T) {
	r := newCoverRig(t, nil)
	f := r.pendingUpload(t, testJPEG(t, 10, 10, 0))
	other := nguoiSoanND()
	other.ID = "CB-2026-KHAC00"
	if _, err := r.uc.Complete(r.ctx, f.ID, other); !errors.Is(err, ErrCoverFileNotFound) {
		t.Fatalf("err = %v", err)
	}
	if r.scanner.scanned != 0 {
		t.Error("another officer's completion reached the scanner")
	}
}

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
