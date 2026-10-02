package app

// The branding acts (branding.go) over the REAL core/store on a fake database/sql driver — so the
// transaction boundaries and the audit INSERT are real statements — and in-memory fakes for the file
// rows, the profile row and the object store, which is where this file's properties live: what is
// scanned, what is promoted, what is published, what is withdrawn, and that the trail rides in the tx.
//
// WHAT IT DOES NOT PROVE: MinIO, clamd, PostgreSQL. stored_file_guard, 0017's branding trigger and the
// CHECKs need a real server; the store's pg suites cover them when VIGOV_TEST_DSN is set (it is not on
// the machine this was written on — those cases SKIP).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

var (
	communeA = tenant.ID("01JA" + strings.Repeat("A", 22))
	communeB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

var staffA = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

// --- fake driver: records statements and transaction boundaries --------------------------------

type fakeDB struct {
	mu                     sync.Mutex
	stmts                  []string
	begins, commits, rolls int
	failOn                 string // the first statement containing this fails
	failed                 bool
}

func (k *fakeDB) record(q string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stmts = append(k.stmts, q)
	if k.failOn != "" && !k.failed && strings.Contains(q, k.failOn) {
		k.failed = true
		return errors.New("fake driver: statement built to fail")
	}
	return nil
}

func (k *fakeDB) count(sub string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	n := 0
	for _, s := range k.stmts {
		if strings.Contains(s, sub) {
			n++
		}
	}
	return n
}

func (k *fakeDB) Connect(context.Context) (driver.Conn, error) { return &fakeConn{k: k}, nil }
func (k *fakeDB) Driver() driver.Driver                        { return fakeDrv{} }

type fakeDrv struct{}

func (fakeDrv) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: Connector only")
}

type fakeConn struct{ k *fakeDB }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begins++
	c.k.mu.Unlock()
	return &fakeTx{k: c.k}, nil
}
func (c *fakeConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	if err := c.k.record(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

type fakeTx struct{ k *fakeDB }

func (t *fakeTx) Commit() error   { t.k.mu.Lock(); t.k.commits++; t.k.mu.Unlock(); return nil }
func (t *fakeTx) Rollback() error { t.k.mu.Lock(); t.k.rolls++; t.k.mu.Unlock(); return nil }

// --- fake stores --------------------------------------------------------------------------------

// fakeFiles keys rows BY COMMUNE, reading it from the transaction / context exactly as the real store
// binds $1 — keyed any other way, the cross-commune case would pass while proving nothing.
type fakeFiles struct {
	rows       map[tenant.ID]map[string]*domain.StoredFile
	softDelete []string
	// keyRolledBack: SetPublicObjectKey accepts a key but does not keep it — what a transaction that
	// rolls back leaves in PostgreSQL. The fake has no transactions of its own.
	keyRolledBack bool
}

func newFakeFiles() *fakeFiles {
	return &fakeFiles{rows: map[tenant.ID]map[string]*domain.StoredFile{}}
}

func (f *fakeFiles) of(t tenant.ID) map[string]*domain.StoredFile {
	if f.rows[t] == nil {
		f.rows[t] = map[string]*domain.StoredFile{}
	}
	return f.rows[t]
}

func (f *fakeFiles) InsertPending(_ context.Context, tx *store.ScopedTx, r domain.StoredFile) error {
	r.SubjectID = string(tx.TenantID())
	f.of(tx.TenantID())[r.ID] = &r
	return nil
}
func (f *fakeFiles) ForUpdate(_ context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error) {
	if r := f.of(tx.TenantID())[id]; r != nil {
		c := *r
		return &c, nil
	}
	return nil, nil
}
func (f *fakeFiles) ByID(ctx context.Context, id string) (*domain.StoredFile, error) {
	if r := f.of(tenant.MustFrom(ctx))[id]; r != nil {
		c := *r
		return &c, nil
	}
	return nil, nil
}
func (f *fakeFiles) Transition(_ context.Context, tx *store.ScopedTx, id string, from, to domain.StoredFileStatus,
	_ time.Time) error {
	r := f.of(tx.TenantID())[id]
	if r == nil || r.Status != from || !from.CanMoveTo(to) {
		return fmt.Errorf("fake: bad edge %s -> %s", from, to)
	}
	r.Status = to
	return nil
}
func (f *fakeFiles) MarkStored(_ context.Context, tx *store.ScopedTx, id string, facts domain.StoredFileFacts,
	_ time.Time) error {
	r := f.of(tx.TenantID())[id]
	if r == nil || r.Status != domain.StoredFileScanning {
		return errors.New("fake: not scanning")
	}
	r.Status, r.MIMEType, r.SizeBytes, r.SHA256 = domain.StoredFileStored, facts.MIMEType, facts.SizeBytes, facts.SHA256
	return nil
}
func (f *fakeFiles) SetPublicObjectKey(_ context.Context, tx *store.ScopedTx, id, key string, _ time.Time) error {
	r := f.of(tx.TenantID())[id]
	if r == nil || r.Status != domain.StoredFileReady {
		return errors.New("fake: not ready")
	}
	if f.keyRolledBack && key != "" {
		return nil
	}
	r.PublicObjectKey = key
	return nil
}
func (f *fakeFiles) PublishedOfPurpose(_ context.Context, tx *store.ScopedTx, purpose string) ([]domain.StoredFile, error) {
	var out []domain.StoredFile
	for _, r := range f.of(tx.TenantID()) {
		if r.Purpose == purpose && r.PublicObjectKey != "" {
			out = append(out, *r)
		}
	}
	return out, nil
}
func (f *fakeFiles) SoftDelete(_ context.Context, tx *store.ScopedTx, id, by, reason string, _ time.Time) error {
	r := f.of(tx.TenantID())[id]
	if r == nil || r.PublicObjectKey != "" || by == "" || reason == "" {
		return errors.New("fake: soft delete refused")
	}
	delete(f.of(tx.TenantID()), id)
	f.softDelete = append(f.softDelete, id)
	return nil
}
func (f *fakeFiles) CountLiveOfPurpose(context.Context, *store.ScopedTx, string, time.Time) (int, error) {
	return 0, nil
}

type fakeProfiles struct {
	rows    map[tenant.ID]*domain.ProfileBranding
	created int
	sets    []string
}

func (p *fakeProfiles) ProfileForUpdate(_ context.Context, tx *store.ScopedTx) (domain.ProfileBranding, bool, error) {
	r := p.rows[tx.TenantID()]
	if r == nil {
		return domain.ProfileBranding{}, false, nil
	}
	return *r, true, nil
}
func (p *fakeProfiles) EnsureProfile(_ context.Context, tx *store.ScopedTx, by string, _ time.Time) (bool, error) {
	if by == "" {
		return false, errors.New("fake: no actor")
	}
	if p.rows[tx.TenantID()] != nil {
		return false, nil
	}
	p.rows[tx.TenantID()] = &domain.ProfileBranding{}
	p.created++
	return true, nil
}
func (p *fakeProfiles) SetBrandingFile(_ context.Context, tx *store.ScopedTx, img domain.BrandingImage, id, by string,
	_ time.Time) error {
	r := p.rows[tx.TenantID()]
	if r == nil || r.Deleted {
		return domain.ErrProfileDeleted
	}
	if img == domain.BrandingLogo {
		r.LogoFileID = id
	} else {
		r.BannerFileID = id
	}
	p.sets = append(p.sets, string(img)+"="+id+" by "+by)
	return nil
}
func (p *fakeProfiles) BrandingView(context.Context) (domain.BrandingView, error) {
	return domain.BrandingView{}, nil
}

// fakeObjects is the three buckets in memory, keyed by object key.
type fakeObjects struct {
	temp, private, public map[string][]byte
	produced              map[string][]byte
	published, unpublish  []string
	publishErr            error
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{temp: map[string][]byte{}, private: map[string][]byte{}, public: map[string][]byte{},
		produced: map[string][]byte{}}
}

func (o *fakeObjects) bucket(b storage.Bucket) map[string][]byte {
	switch b {
	case storage.BucketTemp:
		return o.temp
	case storage.BucketPublic:
		return o.public
	}
	return o.private
}

func (o *fakeObjects) PresignUpload(_ context.Context, key string, _ int64, _ string, ttl time.Duration) (storage.PresignedPost, error) {
	return storage.PresignedPost{URL: "https://minio.example/temp", Fields: map[string]string{"key": key},
		ExpiresAt: time.Now().Add(ttl)}, nil
}
func (o *fakeObjects) Stat(_ context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error) {
	v, ok := o.bucket(b)[key]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrNotFound
	}
	return storage.ObjectInfo{Size: int64(len(v)), ETag: "etag"}, nil
}
func (o *fakeObjects) ReadHead(_ context.Context, b storage.Bucket, key, _ string, n int) ([]byte, error) {
	v := o.bucket(b)[key]
	if len(v) > n {
		v = v[:n]
	}
	return v, nil
}
func (o *fakeObjects) Open(_ context.Context, b storage.Bucket, key, _ string) (io.ReadCloser, int64, error) {
	v, ok := o.bucket(b)[key]
	if !ok {
		return nil, 0, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(v)), int64(len(v)), nil
}
func (o *fakeObjects) SHA256(_ context.Context, b storage.Bucket, key, _ string) (string, error) {
	s := sha256.Sum256(o.bucket(b)[key])
	return hex.EncodeToString(s[:]), nil
}
func (o *fakeObjects) Promote(_ context.Context, src, _ string, dst storage.Key, b storage.Bucket) (storage.Promoted, error) {
	p, _ := dst.Path()
	o.bucket(b)[p] = o.temp[src]
	delete(o.temp, src)
	mime, _, _ := storage.SniffMIME(o.private[p])
	return storage.Promoted{ContentType: mime, ETag: "etag"}, nil
}
func (o *fakeObjects) PutServerProduced(_ context.Context, dst storage.Key, r io.Reader, _ int64) (storage.Produced, error) {
	p, err := dst.Path()
	if err != nil {
		return storage.Produced{}, err
	}
	b, _ := io.ReadAll(r)
	o.private[p], o.produced[p] = b, b
	return storage.Produced{Key: p}, nil
}
func (o *fakeObjects) PurgeAllVersions(_ context.Context, b storage.Bucket, key string) error {
	delete(o.bucket(b), key)
	return nil
}
func (o *fakeObjects) PublishDerivative(_ context.Context, src, dst storage.Key) error {
	if o.publishErr != nil {
		return o.publishErr
	}
	s, _ := src.Path()
	d, _ := dst.Path()
	o.public[d] = o.private[s]
	o.published = append(o.published, d)
	return nil
}
func (o *fakeObjects) UnpublishDerivative(_ context.Context, dst storage.Key) error {
	d, _ := dst.Path()
	delete(o.public, d)
	o.unpublish = append(o.unpublish, d)
	return nil
}
func (o *fakeObjects) PublicURL(key string) (string, error) {
	return "https://media.example/" + key, nil
}

type fakeScanner struct{ infected bool }

func (s fakeScanner) Scan(context.Context, io.Reader, int64) (malwarescan.Result, error) {
	if s.infected {
		return malwarescan.Result{Signature: "Eicar-Test-Signature"}, nil
	}
	return malwarescan.Result{Clean: true}, nil
}

type fakePolicies struct{}

func (fakePolicies) Policy(_ context.Context, p storage.Purpose) (uploadpolicy.Policy, bool, error) {
	return uploadpolicy.Policy{Purpose: p, MaxBytes: 2 << 20,
		AllowedMIMETypes: []string{storage.MIMEPNG, storage.MIMEWebP, storage.MIMEJPEG}}, true, nil
}

// --- rig ----------------------------------------------------------------------------------------

type rig struct {
	uc       *Branding
	db       *fakeDB
	files    *fakeFiles
	profiles *fakeProfiles
	objects  *fakeObjects
	ids      int
}

func newRig(t *testing.T) *rig {
	t.Helper()
	k := &fakeDB{}
	sqlDB := sql.OpenDB(k)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	r := &rig{db: k, files: newFakeFiles(), profiles: &fakeProfiles{rows: map[tenant.ID]*domain.ProfileBranding{}},
		objects: newFakeObjects()}
	r.uc = NewBranding(store.New(sqlDB), r.files, r.profiles, r.objects, fakeScanner{}, fakePolicies{},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.uc.newID = func() (string, error) {
		r.ids++
		return fmt.Sprintf("01JF%022d", r.ids), nil
	}
	return r
}

// transparentPNG is a w×h PNG whose left half is fully transparent and right half opaque red.
func transparentPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := w / 2; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// upload issues an upload in commune c and drops body into the temp bucket under its upload key.
func (r *rig) upload(t *testing.T, c tenant.ID, img domain.BrandingImage, body []byte) string {
	t.Helper()
	up, err := r.uc.RequestUpload(tenant.Into(context.Background(), c), img,
		BrandingUploadRequest{FileName: "logo.png", ContentType: storage.MIMEPNG, Size: 1024}, staffA) // the DECLARED size; the bytes are measured at completion
	if err != nil {
		t.Fatalf("RequestUpload: %v", err)
	}
	r.objects.temp[storage.TempUploadPrefix+up.File.ObjectKey] = body
	return up.File.ID
}

// --- request ------------------------------------------------------------------------------------

func TestRequestUploadRefusesDeclarationsOutsideThePolicy(t *testing.T) {
	ctx := tenant.Into(context.Background(), communeA)
	for _, tc := range []struct {
		name string
		req  BrandingUploadRequest
		want error
	}{
		{"wrong mime (HEIC)", BrandingUploadRequest{FileName: "a.heic", ContentType: storage.MIMEHEIC, Size: 10}, ErrBrandingTypeNotAllowed},
		{"wrong mime (PDF)", BrandingUploadRequest{FileName: "a.pdf", ContentType: storage.MIMEPDF, Size: 10}, ErrBrandingTypeNotAllowed},
		{"over 2 MB", BrandingUploadRequest{FileName: "a.png", ContentType: storage.MIMEPNG, Size: 2<<20 + 1}, ErrBrandingTooLarge},
		{"no size", BrandingUploadRequest{FileName: "a.png", ContentType: storage.MIMEPNG}, domain.ErrBrandingSizeInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			if _, err := r.uc.RequestUpload(ctx, domain.BrandingLogo, tc.req, staffA); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if r.db.begins != 0 || len(r.files.of(communeA)) != 0 {
				t.Error("a refused declaration opened a transaction or wrote a row")
			}
		})
	}
}

func TestRequestUploadWritesRowAndAuditInOneTxUnderTheCommuneKey(t *testing.T) {
	r := newRig(t)
	id := r.upload(t, communeA, domain.BrandingLogo, []byte("x"))
	f := r.files.of(communeA)[id]
	if f == nil || f.Status != domain.StoredFilePending || f.UploadedBy != "CB-00123" {
		t.Fatalf("row = %+v", f)
	}
	// RULE 1 INVARIANT 7: the key carries the commune from ctx, the service and the purpose.
	if !strings.HasPrefix(f.ObjectKey, "content-source/t_"+strings.ToLower(string(communeA))+"/") ||
		!strings.Contains(f.ObjectKey, "/platform/tenant-logo/") {
		t.Errorf("object key %q does not name this commune / service / purpose", f.ObjectKey)
	}
	if r.db.begins != 1 || r.db.commits != 1 || r.db.count("INSERT INTO audit_log") != 1 {
		t.Errorf("begins=%d commits=%d audits=%d, want 1/1/1", r.db.begins, r.db.commits, r.db.count("INSERT INTO audit_log"))
	}
}

func TestRequestUploadRefusesNoActorAndDeletedProfile(t *testing.T) {
	r := newRig(t)
	ctx := tenant.Into(context.Background(), communeA)
	req := BrandingUploadRequest{FileName: "a.png", ContentType: storage.MIMEPNG, Size: 10}
	if _, err := r.uc.RequestUpload(ctx, domain.BrandingLogo, req, audit.Actor{Kind: "staff"}); !errors.Is(err, ErrBrandingNoActor) {
		t.Fatalf("no actor: err = %v", err)
	}
	r.profiles.rows[communeA] = &domain.ProfileBranding{Deleted: true}
	if _, err := r.uc.RequestUpload(ctx, domain.BrandingLogo, req, staffA); !errors.Is(err, domain.ErrProfileDeleted) {
		t.Fatalf("deleted profile: err = %v, want ErrProfileDeleted", err)
	}
	if len(r.files.of(communeA)) != 0 || r.db.commits != 0 {
		t.Error("a refused request left a row or committed")
	}
}

// --- complete -----------------------------------------------------------------------------------

func TestCompleteLogoPublishesAlphaPNGPointsProfileAndAuditsInTheTx(t *testing.T) {
	r := newRig(t)
	ctx := tenant.Into(context.Background(), communeA)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 300, 300))

	f, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if f.Status != domain.StoredFileReady || f.PublicObjectKey == "" {
		t.Fatalf("file = %+v", f)
	}
	if !strings.HasSuffix(f.PublicObjectKey, "/thumb-512.png") || !strings.HasPrefix(f.PublicObjectKey, "public-media/") {
		t.Errorf("public key %q is not the public thumb-512.png twin", f.PublicObjectKey)
	}
	// THE DERIVATIVE KEEPS ALPHA (ADR 0069 #4): decode what was published and look at a transparent pixel.
	pub := r.objects.public[f.PublicObjectKey]
	out, err := png.Decode(bytes.NewReader(pub))
	if err != nil {
		t.Fatalf("published logo is not a PNG: %v", err)
	}
	if b := out.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Errorf("published logo is %dx%d, want 512x512", b.Dx(), b.Dy())
	}
	if _, _, _, a := out.At(5, 256).RGBA(); a != 0 {
		t.Errorf("transparent pixel alpha = %d, want 0 — the logo was flattened", a)
	}
	if r.profiles.rows[communeA] == nil || r.profiles.rows[communeA].LogoFileID != id || r.profiles.created != 1 {
		t.Errorf("profile = %+v created=%d, want the new row pointing at %s", r.profiles.rows[communeA], r.profiles.created, id)
	}
	// Request tx + completion tx; the completion's audit entry committed with it.
	if r.db.commits != 2 || r.db.count("INSERT INTO audit_log") != 2 || r.db.rolls != 0 {
		t.Errorf("commits=%d audits=%d rollbacks=%d", r.db.commits, r.db.count("INSERT INTO audit_log"), r.db.rolls)
	}

	// IDEMPOTENT: a second completion writes nothing.
	before := r.db.count("INSERT INTO audit_log")
	if _, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA); err != nil {
		t.Fatal(err)
	}
	if r.db.count("INSERT INTO audit_log") != before {
		t.Error("a second completion of a ready file wrote another entry")
	}
}

func TestReplacingTheLogoPointsFirstThenWithdrawsAndSoftDeletesTheOld(t *testing.T) {
	r := newRig(t)
	ctx := tenant.Into(context.Background(), communeA)
	first := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 64, 64))
	f1, err := r.uc.Complete(ctx, domain.BrandingLogo, first, staffA)
	if err != nil {
		t.Fatal(err)
	}
	second := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 80, 80))
	f2, err := r.uc.Complete(ctx, domain.BrandingLogo, second, staffA)
	if err != nil {
		t.Fatal(err)
	}
	if f1.PublicObjectKey == f2.PublicObjectKey {
		t.Fatal("a new upload reused the old public key — public objects are cached as immutable")
	}
	if r.profiles.rows[communeA].LogoFileID != second {
		t.Errorf("profile points at %s, want %s", r.profiles.rows[communeA].LogoFileID, second)
	}
	if _, still := r.objects.public[f1.PublicObjectKey]; still {
		t.Error("the old logo's public copy was not withdrawn")
	}
	if len(r.files.softDelete) != 1 || r.files.softDelete[0] != first {
		t.Errorf("soft-deleted = %v, want [%s] (never a hard delete)", r.files.softDelete, first)
	}
	if r.db.count("INSERT INTO audit_log") != 5 { // 2 requests + 2 sets + 1 withdrawal
		t.Errorf("audit entries = %d, want 5", r.db.count("INSERT INTO audit_log"))
	}
}

func TestCompleteRejectsMalwareAndPublishesNothing(t *testing.T) {
	r := newRig(t)
	r.uc.scanner = fakeScanner{infected: true}
	ctx := tenant.Into(context.Background(), communeA)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 32, 32))

	_, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA)
	var rej *BrandingRejection
	if !errors.As(err, &rej) || rej.Reason != BrandingRejectMalware {
		t.Fatalf("err = %v, want a malware rejection", err)
	}
	if r.files.of(communeA)[id].Status != domain.StoredFileRejected || len(r.objects.published) != 0 ||
		len(r.objects.temp) != 0 || r.profiles.rows[communeA] != nil {
		t.Error("an infected upload was kept, published, left in temp or pointed at")
	}
	if r.db.count("INSERT INTO audit_log") != 2 {
		t.Error("the rejection was not audited")
	}
}

func TestCompleteRejectsWrongSniffedTypeAndOversizeAtCompletion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   []byte
		reason string
	}{
		{"declared PNG, bytes are a PDF", []byte("%PDF-1.7\n%âãÏÓ\n1 0 obj\n"), BrandingRejectTypeNotAllowed},
		{"grew past 2 MB through the form", append(transparentPNG(t, 8, 8), make([]byte, 2<<20)...), BrandingRejectTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			id := r.upload(t, communeA, domain.BrandingLogo, tc.body)
			_, err := r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingLogo, id, staffA)
			var rej *BrandingRejection
			if !errors.As(err, &rej) || rej.Reason != tc.reason {
				t.Fatalf("err = %v, want rejection %q", err, tc.reason)
			}
			if len(r.objects.published) != 0 {
				t.Error("published a rejected file")
			}
		})
	}
}

func TestCompleteInAnotherCommuneIsNotFound(t *testing.T) {
	// The upload id of commune A completed on commune B's host: the row is not B's — one answer, 404,
	// no inspection, nothing touched (rule 1; rule 4 forbidden #2 on the commune axis).
	r := newRig(t)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))
	_, err := r.uc.Complete(tenant.Into(context.Background(), communeB), domain.BrandingLogo, id, staffA)
	if !errors.Is(err, ErrBrandingFileNotFound) {
		t.Fatalf("err = %v, want ErrBrandingFileNotFound", err)
	}
	// Completed as the BANNER: also not found (the file was issued for the logo).
	_, err = r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingBanner, id, staffA)
	if !errors.Is(err, ErrBrandingFileNotFound) {
		t.Fatalf("other image: err = %v, want ErrBrandingFileNotFound", err)
	}
	// Another officer of the same commune: not found either.
	other := audit.Actor{ID: "CB-00999", Kind: "staff"}
	_, err = r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingLogo, id, other)
	if !errors.Is(err, ErrBrandingFileNotFound) {
		t.Fatalf("other officer: err = %v, want ErrBrandingFileNotFound", err)
	}
	if len(r.objects.published) != 0 {
		t.Error("published through a foreign completion")
	}
}

func TestCompleteOnSoftDeletedProfileRefusesAndPublishesNothing(t *testing.T) {
	r := newRig(t)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))
	r.profiles.rows[communeA] = &domain.ProfileBranding{Deleted: true}
	_, err := r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingLogo, id, staffA)
	if !errors.Is(err, domain.ErrProfileDeleted) {
		t.Fatalf("err = %v, want ErrProfileDeleted", err)
	}
	if len(r.objects.published) != 0 || r.profiles.created != 0 || r.db.rolls != 1 {
		t.Errorf("published=%v created=%d rollbacks=%d — a retired profile must be refused, never revived",
			r.objects.published, r.profiles.created, r.db.rolls)
	}
}

func TestAuditFailureRollsBackAndWithdrawsTheCopy(t *testing.T) {
	// The audit INSERT of the completion is the LAST statement of its transaction. It fails → the whole
	// transaction rolls back, and the public copy already made is undone (rule 6 inv 3; ADR 0052 §11).
	r := newRig(t)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))
	r.db.failOn = "INSERT INTO audit_log"
	r.files.keyRolledBack = true
	_, err := r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingLogo, id, staffA)
	if err == nil {
		t.Fatal("audit failure did not fail the completion")
	}
	if r.db.rolls != 1 {
		t.Errorf("rollbacks = %d, want 1", r.db.rolls)
	}
	if len(r.objects.published) != 1 || len(r.objects.unpublish) != 1 || r.objects.unpublish[0] != r.objects.published[0] {
		t.Fatalf("published=%v unpublished=%v — the copy made inside the rolled-back tx must be withdrawn",
			r.objects.published, r.objects.unpublish)
	}
}

func decodeJPEGConfig(b []byte) (image.Config, error) { return jpeg.DecodeConfig(bytes.NewReader(b)) }

func TestPublishFailureWritesNothing(t *testing.T) {
	r := newRig(t)
	r.objects.publishErr = errors.New("minio down")
	id := r.upload(t, communeA, domain.BrandingBanner, transparentPNG(t, 16, 16))
	_, err := r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingBanner, id, staffA)
	if !errors.Is(err, ErrBrandingPublishUnavailable) {
		t.Fatalf("err = %v, want ErrBrandingPublishUnavailable", err)
	}
	if r.db.rolls != 1 || r.db.count("INSERT INTO audit_log") != 1 {
		t.Errorf("rollbacks=%d audits=%d — the completion must leave nothing committed", r.db.rolls, r.db.count("INSERT INTO audit_log"))
	}
}

func TestBannerDerivativeIs1600WideJPEG(t *testing.T) {
	r := newRig(t)
	id := r.upload(t, communeA, domain.BrandingBanner, transparentPNG(t, 800, 100))
	f, err := r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingBanner, id, staffA)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(f.PublicObjectKey, "/thumb-1600.jpg") {
		t.Fatalf("public key = %q", f.PublicObjectKey)
	}
	cfg, err := decodeJPEGConfig(r.objects.public[f.PublicObjectKey])
	if err != nil || cfg.Width != 1600 || cfg.Height != 200 {
		t.Errorf("banner = %+v err=%v, want 1600x200 JPEG", cfg, err)
	}
}

func TestDecodeBombRefusedFromTheHeader(t *testing.T) {
	// A PNG header declaring 9000 x 9000: refused from the header before any pixel is decoded.
	r := newRig(t)
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewGray(image.Rect(0, 0, 9000, 1)))
	b := buf.Bytes()
	// Rewrite the IHDR height (bytes 20..23) to 9000 — the CRC is wrong, which the header read ignores.
	b[20], b[21], b[22], b[23] = 0, 0, 0x23, 0x28
	id := r.upload(t, communeA, domain.BrandingLogo, b)
	_, err := r.uc.Complete(tenant.Into(context.Background(), communeA), domain.BrandingLogo, id, staffA)
	var rej *BrandingRejection
	if !errors.As(err, &rej) || (rej.Reason != BrandingRejectTooManyPixels && rej.Reason != BrandingRejectUndecodable) {
		t.Fatalf("err = %v, want a too-many-pixels (or undecodable) rejection", err)
	}
}

// --- remove -------------------------------------------------------------------------------------

func TestRemoveClearsAuditsWithdrawsAndIsIdempotent(t *testing.T) {
	r := newRig(t)
	ctx := tenant.Into(context.Background(), communeA)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))
	f, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := r.uc.Remove(ctx, domain.BrandingLogo, staffA)
	if err != nil || !removed {
		t.Fatalf("Remove = %v, %v", removed, err)
	}
	if r.profiles.rows[communeA].LogoFileID != "" {
		t.Error("the logo column is still set")
	}
	if _, still := r.objects.public[f.PublicObjectKey]; still || len(r.files.softDelete) != 1 {
		t.Error("the removed logo was not withdrawn and soft-deleted")
	}
	audits := r.db.count("INSERT INTO audit_log")
	removed, err = r.uc.Remove(ctx, domain.BrandingLogo, staffA)
	if err != nil || removed || r.db.count("INSERT INTO audit_log") != audits {
		t.Errorf("second Remove = %v, %v, audits %d -> %d: must write nothing", removed, err, audits,
			r.db.count("INSERT INTO audit_log"))
	}
}

func TestRemoveOnSoftDeletedProfileRefuses(t *testing.T) {
	r := newRig(t)
	r.profiles.rows[communeA] = &domain.ProfileBranding{Deleted: true, LogoFileID: "x"}
	if _, err := r.uc.Remove(tenant.Into(context.Background(), communeA), domain.BrandingLogo, staffA); !errors.Is(err, domain.ErrProfileDeleted) {
		t.Fatalf("err = %v, want ErrProfileDeleted", err)
	}
}

// --- configuration --------------------------------------------------------------------------------

func TestUnconfiguredStorageRefusesAndBuildsNoURL(t *testing.T) {
	r := newRig(t)
	r.uc.objects = nil
	ctx := tenant.Into(context.Background(), communeA)
	_, err := r.uc.RequestUpload(ctx, domain.BrandingLogo,
		BrandingUploadRequest{FileName: "a.png", ContentType: storage.MIMEPNG, Size: 10}, staffA)
	if !errors.Is(err, ErrBrandingUploadNotConfigured) {
		t.Fatalf("err = %v", err)
	}
	if u := r.uc.PublicURL("public-media/x"); u != "" {
		t.Errorf("PublicURL without storage = %q, want empty", u)
	}
}

// --- compensation race and decode slot (review 02/10/2026) ---------------------------------------

func TestUndoPublishLeavesACopyAConcurrentCompletionCommitted(t *testing.T) {
	// The race: completion X copies, then its transaction fails; completion Y of the SAME upload
	// copies the SAME twin (idempotent) and commits its key. X's compensation must read the row UNDER
	// THE LOCK and, seeing Y's key, leave the object alone.
	r := newRig(t)
	ctx := tenant.Into(context.Background(), communeA)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))
	f, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA) // plays Y: committed, key recorded
	if err != nil {
		t.Fatal(err)
	}
	begins := r.db.begins
	r.uc.undoPublish(ctx, domain.BrandingLogo, id) // plays X's compensation, arriving late
	if len(r.objects.unpublish) != 0 {
		t.Fatalf("compensation withdrew %v — the copy a committed completion owns", r.objects.unpublish)
	}
	if _, ok := r.objects.public[f.PublicObjectKey]; !ok {
		t.Fatal("the current logo's public object is gone")
	}
	if r.db.begins != begins+1 {
		t.Errorf("compensation opened %d transactions, want 1 — the decision must be taken under the row lock",
			r.db.begins-begins)
	}
}

func TestUndoPublishWithdrawsAnUnrecordedCopyUnderTheLock(t *testing.T) {
	r := newRig(t)
	ctx := tenant.Into(context.Background(), communeA)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))
	r.files.keyRolledBack = true // the completion's tx "rolls back": key never kept
	if _, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA); err != nil {
		t.Fatal(err)
	}
	r.uc.undoPublish(ctx, domain.BrandingLogo, id)
	if len(r.objects.unpublish) != 1 {
		t.Fatalf("unpublished = %v, want the one unrecorded copy", r.objects.unpublish)
	}
}

func TestDecodeSlotBusyAnswers503AfterTheDeadlineAndIOIsOutsideTheSlot(t *testing.T) {
	r := newRig(t)
	r.uc.decodeWait = 50 * time.Millisecond
	ctx := tenant.Into(context.Background(), communeA)
	id := r.upload(t, communeA, domain.BrandingLogo, transparentPNG(t, 16, 16))

	r.uc.decodeSlot <- struct{}{} // another commune's decode holds the slot
	start := time.Now()
	_, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA)
	<-r.uc.decodeSlot
	if !errors.Is(err, ErrBrandingDecodeBusy) {
		t.Fatalf("err = %v, want ErrBrandingDecodeBusy", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("the slot deadline did not bound the wait")
	}
	// Nothing written: no completion transaction committed, no derivative produced, nothing published.
	if r.db.commits != 1 || len(r.objects.produced) != 0 || len(r.objects.published) != 0 {
		t.Errorf("commits=%d produced=%d published=%d — a busy refusal wrote something",
			r.db.commits, len(r.objects.produced), len(r.objects.published))
	}
	// Retryable: with the slot free the same completion succeeds.
	if _, err := r.uc.Complete(ctx, domain.BrandingLogo, id, staffA); err != nil {
		t.Fatalf("retry after busy: %v", err)
	}
}
