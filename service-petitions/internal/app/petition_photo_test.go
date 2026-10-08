package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the citizen scene-photo use case (petition_photo.go) with in-memory fakes for the two
// stores, MinIO, clamd and the platform policy, and a recording SQL driver for the transaction and the
// audit INSERT it carries.
//
//	PROVED HERE   request: only the petition's own citizen, only while `da-tiep-nhan`, at most the
//	              policy's count INCLUDING live pending slots, under the petition lock · the pending row
//	              is citizen-media / private / `cong-dan` / fixed name, its key `.jpg` whatever was
//	              declared, the form signed for the declared type's temp key · row + trail in ONE
//	              committed transaction · HEIC refused even when a policy lists it · no policy count is
//	              not configured. completion: the raw upload is scanned, decoded, oriented, re-encoded as
//	              JPEG with NO EXIF, written to private, and the temp purged — the raw bytes never reach
//	              private · PNG/WebP come out JPEG · malware, type mismatch, undecodable are rejected with
//	              the trail · scanner down writes nothing · the petition moving on under the lock rejects
//	              the photo and removes the clean copy · another citizen's / petition's file is 404 ·
//	              stored answers as stored. lists: signed links for stored photos only; restricted field.
//	NOT PROVED    PostgreSQL (migration 0026's trigger and CHECKs: stored_file_pg_test.go, SKIPS without
//	              VIGOV_TEST_DSN), MinIO and clamd themselves (core suites; integration SKIPS without them).

const (
	ppFileID   = "01JPH0T0000000000000000001" // a valid ULID: it becomes a key segment
	ppCitizen  = "cd-01JCONGDANTHU"
	ppOther    = "cd-01JNGUOIKHAC"
	ppCode     = "PA-7K3M-9QXT-4HBD"
	ppPetition = "pa-01JPETITION"
)

var ppNow = time.Date(2026, 10, 2, 3, 4, 5, 0, time.UTC)

// --- the recording driver: transactions and the audit INSERT --------------------------------------

type ppExec struct {
	sql  string
	args []driver.Value
}

type ppDB struct {
	mu        sync.Mutex
	begun     int
	committed []ppExec
	open      []ppExec
	rolled    int
	execErr   error
}

func (d *ppDB) Connect(context.Context) (driver.Conn, error) { return &ppConn{d: d}, nil }
func (d *ppDB) Driver() driver.Driver                        { return ppDriver{} }

type ppDriver struct{}

func (ppDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type ppConn struct{ d *ppDB }

func (c *ppConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *ppConn) Close() error                        { return nil }
func (c *ppConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *ppConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.begun++
	c.d.open = nil
	return &ppTx{d: c.d}, nil
}
func (c *ppConn) CheckNamedValue(*driver.NamedValue) error { return nil }
func (c *ppConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	if c.d.execErr != nil {
		return nil, c.d.execErr
	}
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	c.d.open = append(c.d.open, ppExec{sql: q, args: v})
	return driver.RowsAffected(1), nil
}

type ppTx struct{ d *ppDB }

func (t *ppTx) Commit() error {
	t.d.mu.Lock()
	defer t.d.mu.Unlock()
	t.d.committed = append(t.d.committed, t.d.open...)
	t.d.open = nil
	return nil
}
func (t *ppTx) Rollback() error {
	t.d.mu.Lock()
	defer t.d.mu.Unlock()
	t.d.open = nil
	t.d.rolled++
	return nil
}

// audits returns the committed audit entries as (actor id, actor kind, action, subject, delta).
func (d *ppDB) audits() [][5]string {
	var out [][5]string
	for _, e := range d.committed {
		if strings.Contains(e.sql, "INSERT INTO audit_log") {
			out = append(out, [5]string{fmt.Sprint(e.args[1]), fmt.Sprint(e.args[2]), fmt.Sprint(e.args[4]),
				fmt.Sprint(e.args[5]), string(e.args[7].([]byte))})
		}
	}
	return out
}

// --- the two stores ---------------------------------------------------------------------------------

type ppPetitions struct {
	byCommune  map[tenant.ID]map[string]domain.PhieuPhanAnh
	statusAtLk *domain.TrangThai // the status a LOCKING read sees — staff moved it meanwhile
	locked     int
}

func (p *ppPetitions) find(commune tenant.ID, citizen, code string) (domain.PhieuPhanAnh, error) {
	if citizen == "" {
		return domain.PhieuPhanAnh{}, petstore.ErrThieuDinhDanhCongDan
	}
	pa, ok := p.byCommune[commune][code]
	if !ok || pa.CongDanID != citizen {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

func (p *ppPetitions) CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (domain.PhieuPhanAnh, error) { // vi-name-ok: implements the existing store method
	return p.find(tenant.MustFrom(ctx), congDanID, ma)
}

func (p *ppPetitions) CitizenPetitionForUpdate(_ context.Context, tx *pkgstore.ScopedTx, congDanID, ma string) (
	domain.PhieuPhanAnh, error) {
	p.locked++
	pa, err := p.find(tx.TenantID(), congDanID, ma)
	if err == nil && p.statusAtLk != nil {
		pa.TrangThai = *p.statusAtLk
	}
	return pa, err
}

// findOwned applies the owner filter by KIND, as the store's ownerFilter does in SQL: a citizen owner
// matches `cong_dan_id`, a Zalo account `zalo_account_id`, and never the other column (ADR 0080).
func (p *ppPetitions) findOwned(commune tenant.ID, owner domain.PetitionOwner, code string) (domain.PhieuPhanAnh, error) {
	if !owner.Valid() {
		return domain.PhieuPhanAnh{}, petstore.ErrThieuDinhDanhCongDan
	}
	pa, ok := p.byCommune[commune][code]
	if !ok {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	switch {
	case owner.Kind == domain.OwnerCitizen && pa.CongDanID == owner.ID:
	case owner.Kind == domain.OwnerZaloAccount && pa.ZaloAccountID == owner.ID:
	default:
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

func (p *ppPetitions) OwnedByCode(ctx context.Context, owner domain.PetitionOwner, ma string) (domain.PhieuPhanAnh, error) {
	return p.findOwned(tenant.MustFrom(ctx), owner, ma)
}

func (p *ppPetitions) OwnedForUpdate(_ context.Context, tx *pkgstore.ScopedTx, owner domain.PetitionOwner, ma string) (
	domain.PhieuPhanAnh, error) {
	p.locked++
	pa, err := p.findOwned(tx.TenantID(), owner, ma)
	if err == nil && p.statusAtLk != nil {
		pa.TrangThai = *p.statusAtLk
	}
	return pa, err
}

func (p *ppPetitions) TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) { // vi-name-ok: implements the existing store method
	pa, ok := p.byCommune[tenant.MustFrom(ctx)][ma]
	if !ok {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

type ppFiles struct {
	rows        map[tenant.ID]map[string]domain.StoredFile
	inserted    []domain.StoredFile
	transitions []string
	marked      []domain.StoredFileFacts
}

func (f *ppFiles) all(commune tenant.ID) map[string]domain.StoredFile {
	if f.rows[commune] == nil {
		f.rows[commune] = map[string]domain.StoredFile{}
	}
	return f.rows[commune]
}

func (f *ppFiles) InsertPending(_ context.Context, tx *pkgstore.ScopedTx, sf domain.StoredFile) error {
	sf.Status = domain.StoredFilePending
	f.all(tx.TenantID())[sf.ID] = sf
	f.inserted = append(f.inserted, sf)
	return nil
}

func (f *ppFiles) ForUpdate(_ context.Context, tx *pkgstore.ScopedTx, id string) (*domain.StoredFile, error) {
	sf, ok := f.all(tx.TenantID())[id]
	if !ok {
		return nil, nil
	}
	return &sf, nil
}

func (f *ppFiles) ByID(ctx context.Context, id string) (*domain.StoredFile, error) {
	sf, ok := f.all(tenant.MustFrom(ctx))[id]
	if !ok {
		return nil, nil
	}
	return &sf, nil
}

func (f *ppFiles) Transition(_ context.Context, tx *pkgstore.ScopedTx, id string, from, to domain.StoredFileStatus,
	_ time.Time) error {
	rows := f.all(tx.TenantID())
	sf, ok := rows[id]
	if !ok || sf.Status != from || !from.CanMoveTo(to) {
		return petstore.ErrStoredFileMoved
	}
	sf.Status = to
	rows[id] = sf
	f.transitions = append(f.transitions, string(from)+"->"+string(to))
	return nil
}

func (f *ppFiles) MarkStored(_ context.Context, tx *pkgstore.ScopedTx, id string, facts domain.StoredFileFacts,
	_ time.Time) error {
	rows := f.all(tx.TenantID())
	sf := rows[id]
	if sf.Status != domain.StoredFileScanning {
		return petstore.ErrStoredFileMoved
	}
	sf.Status, sf.MIMEType, sf.SizeBytes, sf.SHA256 = domain.StoredFileStored, facts.MIMEType, facts.SizeBytes, facts.SHA256
	rows[id] = sf
	f.marked = append(f.marked, facts)
	return nil
}

func (f *ppFiles) count(commune tenant.ID, subjectID string, pendingSince time.Time) int {
	n := 0
	for _, sf := range f.all(commune) {
		if sf.SubjectType != domain.StoredFileSubjectPetition || sf.SubjectID != subjectID ||
			sf.Purpose != domain.PurposePetitionPhoto {
			continue
		}
		switch sf.Status {
		case domain.StoredFileStored, domain.StoredFileProcessing, domain.StoredFileReady:
			n++
		case domain.StoredFilePending, domain.StoredFileScanning:
			if !pendingSince.IsZero() && !sf.CreatedAt.Before(pendingSince) {
				n++
			}
		}
	}
	return n
}

func (f *ppFiles) CountForSubjectTx(_ context.Context, tx *pkgstore.ScopedTx, _, subjectID, _ string,
	pendingSince time.Time) (int, error) {
	return f.count(tx.TenantID(), subjectID, pendingSince), nil
}

func (f *ppFiles) CountForSubject(ctx context.Context, _, subjectID, _ string, pendingSince time.Time) (int, error) {
	return f.count(tenant.MustFrom(ctx), subjectID, pendingSince), nil
}

func (f *ppFiles) PetitionPhotos(ctx context.Context, petitionID string) ([]domain.StoredFile, error) {
	out := []domain.StoredFile{}
	for _, sf := range f.all(tenant.MustFrom(ctx)) {
		if domain.IsCitizenPhotoOf(sf, petitionID) && sf.Status.Attachable() {
			out = append(out, sf)
		}
	}
	return out, nil
}

// --- MinIO with PutServerProduced -------------------------------------------------------------------

type ppObjects struct {
	*objectStoreFake
	produced []string
	putErr   error

	opens    atomic.Int32
	opened   chan struct{} // when set, signalled on every Open (the caller then holds a read slot)
	openGate chan struct{} // when set, Open waits for it to close — or for the caller's context to end
}

func (o *ppObjects) Open(ctx context.Context, b storage.Bucket, key, etag string) (io.ReadCloser, int64, error) {
	o.opens.Add(1)
	if o.opened != nil {
		o.opened <- struct{}{}
	}
	if o.openGate != nil {
		select {
		case <-o.openGate:
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
	return o.objectStoreFake.Open(ctx, b, key, etag)
}

func (o *ppObjects) PutServerProduced(_ context.Context, dst storage.Key, r io.Reader, size int64) (storage.Produced, error) {
	if o.putErr != nil {
		return storage.Produced{}, o.putErr
	}
	if dst.Class != storage.ClassCitizenMedia || dst.Variant != storage.VariantOriginal {
		return storage.Produced{}, storage.ErrInvalidArgument
	}
	d, err := io.ReadAll(r)
	if err != nil || int64(len(d)) != size {
		return storage.Produced{}, storage.ErrInvalidArgument
	}
	mime, ext, ok := storage.SniffMIME(d)
	if !ok || ext != dst.Ext {
		return storage.Produced{}, storage.ErrInvalidArgument
	}
	key, err := dst.Path()
	if err != nil {
		return storage.Produced{}, err
	}
	if o.has(storage.BucketPrivate, key) {
		return storage.Produced{}, storage.ErrExists
	}
	o.put(storage.BucketPrivate, key, d)
	o.produced = append(o.produced, key)
	s := sha256.Sum256(d)
	return storage.Produced{Key: key, Size: size, ContentType: mime, SHA256: hex.EncodeToString(s[:])}, nil
}

type ppPolicy struct {
	p   uploadpolicy.Policy
	ok  bool
	err error
}

func (p *ppPolicy) Policy(_ context.Context, purpose storage.Purpose) (uploadpolicy.Policy, bool, error) {
	if p.err != nil {
		return uploadpolicy.Policy{}, false, p.err
	}
	if purpose != storage.PurposePetitionPhoto {
		return uploadpolicy.Policy{}, false, nil
	}
	return p.p, p.ok, nil
}

// ppPhotoPolicy is the CONFIGURED shape of platform's petition-photo row (test data, not a default).
func ppPhotoPolicy() *ppPolicy {
	return &ppPolicy{ok: true, p: uploadpolicy.Policy{
		Purpose: storage.PurposePetitionPhoto, MaxBytes: 10 << 20,
		AllowedMIMETypes: []string{storage.MIMEJPEG, storage.MIMEPNG, storage.MIMEWebP},
		FileCountLimited: true, MaxFilesPerSubject: 5,
	}}
}

// --- harness ----------------------------------------------------------------------------------------

type ppHarness struct {
	uc      *CitizenPetitionPhotos
	db      *ppDB
	store   *pkgstore.DB
	pets    *ppPetitions
	files   *ppFiles
	objects *ppObjects
	scanner *scannerFake
	policy  *ppPolicy
	ctx     context.Context
}

func ppPetitionRow(status domain.TrangThai) domain.PhieuPhanAnh {
	return domain.PhieuPhanAnh{ID: ppPetition, MaTraCuu: ppCode, CongDanID: ppCitizen, TrangThai: status,
		Kenh: domain.KenhZaloMiniApp}
}

func buildPhotos(t *testing.T) *ppHarness {
	t.Helper()
	d := &ppDB{}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := &ppHarness{
		db: d,
		pets: &ppPetitions{byCommune: map[tenant.ID]map[string]domain.PhieuPhanAnh{
			xaThu: {ppCode: ppPetitionRow(domain.DaTiepNhan),
				"PA-OTHER-CITIZEN-1": {ID: "pa-other", MaTraCuu: "PA-OTHER-CITIZEN-1", CongDanID: ppOther,
					TrangThai: domain.DaTiepNhan}},
		}},
		files:   &ppFiles{rows: map[tenant.ID]map[string]domain.StoredFile{}},
		objects: &ppObjects{objectStoreFake: newObjectStoreFake()},
		scanner: &scannerFake{res: malwarescan.Result{Clean: true}},
		policy:  ppPhotoPolicy(),
		ctx:     ctxXa(xaThu),
	}
	h.store = pkgstore.New(db)
	h.uc = NewCitizenPetitionPhotos(h.store, h.pets, h.files, h.objects, h.scanner, h.policy)
	h.uc.newID = func() (string, error) { return ppFileID, nil }
	h.uc.now = func() time.Time { return ppNow }
	return h
}

func ppCitizenActor() audit.Actor { return audit.Actor{ID: ppCitizen, Kind: "citizen", IP: "10.0.0.9"} }

func ppDestKey() string {
	return "citizen-media/t_" + strings.ToLower(string(xaThu)) + "/2026/10/petitions/petition-photo/" +
		strings.ToLower(ppFileID) + "/original.jpg"
}

func ppUploadKey(ext string) string {
	return "upload/" + strings.TrimSuffix(ppDestKey(), "jpg") + ext
}

// ppJPEG is a w×h JPEG, left half red and right half blue, with an EXIF APP1 carrying Orientation
// and a fake GPS payload — the shape a phone writes.
func ppJPEG(t *testing.T, w, h, orientation int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	j := buf.Bytes()
	tiff := []byte("II*\x00\x08\x00\x00\x00")
	ifd := make([]byte, 2+12+4)
	binary.LittleEndian.PutUint16(ifd[0:2], 1)
	binary.LittleEndian.PutUint16(ifd[2:4], 0x0112)
	binary.LittleEndian.PutUint16(ifd[4:6], 3)
	binary.LittleEndian.PutUint32(ifd[6:10], 1)
	binary.LittleEndian.PutUint16(ifd[10:12], uint16(orientation))
	payload := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	payload = append(payload, []byte("GPS-16.05N-108.20E")...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:4], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, j[:2]...)
	out = append(out, seg...)
	return append(out, j[2:]...)
}

// seedPendingPhoto puts the row RequestUpload would have written, and the phone's upload in temp.
func (h *ppHarness) seedPendingPhoto(ext string, data []byte, created time.Time) {
	h.files.all(xaThu)[ppFileID] = domain.StoredFile{
		ID: ppFileID, Bucket: domain.StoredFileBucketPrivate, ObjectKey: ppDestKey(),
		RetentionClass: "citizen-media", Purpose: domain.PurposePetitionPhoto,
		SubjectType: domain.StoredFileSubjectPetition, SubjectID: ppPetition, OriginalName: domain.PetitionPhotoName,
		Status: domain.StoredFilePending, UploadedBy: domain.CitizenLogActor, CreatedAt: created, UpdatedAt: created,
	}
	if data != nil {
		h.objects.put(storage.BucketTemp, ppUploadKey(ext), data)
	}
}

func (h *ppHarness) addPhoto(id string, status domain.StoredFileStatus, created time.Time) {
	h.files.all(xaThu)[id] = domain.StoredFile{
		ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: "citizen-media/k/" + id + "/original.jpg",
		RetentionClass: "citizen-media", Purpose: domain.PurposePetitionPhoto,
		SubjectType: domain.StoredFileSubjectPetition, SubjectID: ppPetition, OriginalName: domain.PetitionPhotoName,
		Status: status, UploadedBy: domain.CitizenLogActor, CreatedAt: created, UpdatedAt: created,
	}
}

// --- a. request -------------------------------------------------------------------------------------

func TestPhotoRequestIssuesCitizenMediaRowFormAndTrailInOneTransaction(t *testing.T) {
	h := buildPhotos(t)
	up, err := h.uc.RequestUpload(h.ctx, ppCode, PhotoUploadRequest{ContentType: storage.MIMEPNG, Size: 812_000},
		ppCitizenActor())
	if err != nil {
		t.Fatalf("RequestUpload: %v", err)
	}
	f := up.File
	if f.ObjectKey != ppDestKey() || f.RetentionClass != "citizen-media" || f.Bucket != domain.StoredFileBucketPrivate ||
		f.Purpose != "petition-photo" || f.SubjectType != "petition" || f.SubjectID != ppPetition ||
		f.UploadedBy != domain.CitizenLogActor || f.OriginalName != domain.PetitionPhotoName ||
		f.Status != domain.StoredFilePending {
		t.Errorf("pending row = %+v", f)
	}
	if h.pets.locked != 1 {
		t.Errorf("the petition was read with %d locking reads, want 1", h.pets.locked)
	}
	// The form is for the DECLARED type's temp key; the destination is .jpg whatever was declared.
	if len(h.objects.presigned) != 1 {
		t.Fatalf("%d forms signed", len(h.objects.presigned))
	}
	p := h.objects.presigned[0]
	if p.key != ppUploadKey("png") || p.contentType != storage.MIMEPNG || p.maxBytes != 10<<20 || p.ttl != storage.UploadTTL {
		t.Errorf("form = %+v", p)
	}
	if h.db.begun != 1 {
		t.Fatalf("%d transactions, want 1", h.db.begun)
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != ppCitizen || a[0][1] != "citizen" || a[0][2] != ActionPetitionPhotoRequested ||
		a[0][3] != ppCode {
		t.Fatalf("committed trail = %v", a)
	}
	for _, want := range []string{`"tep_id":"` + ppFileID + `"`, `"loai_khai_bao":"image/png"`, `"kich_thuoc_khai":812000`} {
		if !strings.Contains(a[0][4], want) {
			t.Errorf("delta lacks %s: %s", want, a[0][4])
		}
	}
}

func TestPhotoRequestRefusalsWriteNothing(t *testing.T) {
	moved := domain.DangPhanLoai
	for _, c := range []struct {
		name  string
		mod   func(*ppHarness, *PhotoUploadRequest, *string, *audit.Actor)
		want  error
		inTx  bool // the refusal may come after the transaction opened (then it must roll back)
		check func(*testing.T, error)
	}{
		{name: "another citizen's petition", inTx: true, want: petstore.ErrPhieuKhongTonTai,
			mod: func(_ *ppHarness, _ *PhotoUploadRequest, code *string, _ *audit.Actor) { *code = "PA-OTHER-CITIZEN-1" }},
		{name: "unknown code", inTx: true, want: petstore.ErrPhieuKhongTonTai,
			mod: func(_ *ppHarness, _ *PhotoUploadRequest, code *string, _ *audit.Actor) { *code = "PA-0000-0000-0000" }},
		{name: "another commune", inTx: true, want: petstore.ErrPhieuKhongTonTai,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) {
				h.ctx = ctxXa(tenant.ID("01JB" + strings.Repeat("B", 22)))
			}},
		{name: "petition moved on", inTx: true, want: ErrPhotoWindowClosed,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) {
				h.pets.byCommune[xaThu][ppCode] = ppPetitionRow(moved)
			}},
		{name: "five already (live pending counts, abandoned does not)", inTx: true, want: ErrPhotoCountReached,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) {
				for i, s := range []domain.StoredFileStatus{domain.StoredFileStored, domain.StoredFileStored,
					domain.StoredFileReady, domain.StoredFileStored} {
					h.addPhoto(fmt.Sprintf("01JPHOTOOLD00000000000000%d", i), s, ppNow.Add(-time.Hour))
				}
				h.addPhoto("01JPHOTOLIVEPENDING0000000", domain.StoredFilePending, ppNow.Add(-5*time.Minute))
				h.addPhoto("01JPHOTOABANDONED000000000", domain.StoredFilePending, ppNow.Add(-time.Hour))
				h.addPhoto("01JPHOTOREJECTED0000000000", domain.StoredFileRejected, ppNow.Add(-time.Hour))
			}},
		{name: "HEIC even if a policy lists it", want: ErrPhotoTypeNotAllowed,
			mod: func(h *ppHarness, r *PhotoUploadRequest, _ *string, _ *audit.Actor) {
				h.policy.p.AllowedMIMETypes = append(h.policy.p.AllowedMIMETypes, storage.MIMEHEIC)
				r.ContentType = storage.MIMEHEIC
			}},
		{name: "video", want: ErrPhotoTypeNotAllowed,
			mod: func(_ *ppHarness, r *PhotoUploadRequest, _ *string, _ *audit.Actor) { r.ContentType = storage.MIMEMP4 }},
		{name: "over the policy size", want: ErrPhotoTooLarge,
			mod: func(_ *ppHarness, r *PhotoUploadRequest, _ *string, _ *audit.Actor) { r.Size = 10<<20 + 1 }},
		{name: "size zero", want: ErrPhotoSizeInvalid,
			mod: func(_ *ppHarness, r *PhotoUploadRequest, _ *string, _ *audit.Actor) { r.Size = 0 }},
		{name: "no policy", want: ErrUploadNotConfigured,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) { h.policy.ok = false }},
		{name: "policy without a count", want: ErrUploadNotConfigured,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) {
				h.policy.p.FileCountLimited = false
			}},
		{name: "platform unreachable", want: ErrUploadLimitsUnavailable,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) {
				h.policy.err = uploadpolicy.ErrUnavailable
			}},
		{name: "no object store", want: ErrUploadNotConfigured,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) { h.uc.objects = nil }},
		{name: "no scanner", want: ErrUploadNotConfigured,
			mod: func(h *ppHarness, _ *PhotoUploadRequest, _ *string, _ *audit.Actor) { h.uc.scanner = nil }},
		{name: "a staff actor", check: func(t *testing.T, err error) {
			if err == nil || !strings.Contains(err.Error(), "không phải công dân") {
				t.Errorf("err = %v", err)
			}
		}, mod: func(_ *ppHarness, _ *PhotoUploadRequest, _ *string, a *audit.Actor) {
			*a = audit.Actor{ID: "CB-00123", Kind: "staff"}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := buildPhotos(t)
			req, code, actor := PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 400_000}, ppCode, ppCitizenActor()
			c.mod(h, &req, &code, &actor)
			_, err := h.uc.RequestUpload(h.ctx, code, req, actor)
			if c.check != nil {
				c.check(t, err)
			} else if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if len(h.db.committed) != 0 || len(h.files.inserted) != 0 || len(h.objects.presigned) != 0 {
				t.Errorf("a refusal wrote: %d committed, %d rows, %d forms", len(h.db.committed), len(h.files.inserted),
					len(h.objects.presigned))
			}
			if !c.inTx && h.db.begun != 0 {
				t.Errorf("a refusal before any data opened %d transactions", h.db.begun)
			}
			if err != nil && (strings.Contains(err.Error(), ppCode) || strings.Contains(err.Error(), ppCitizen)) {
				t.Errorf("error carries the lookup code or the citizen id (rule 3): %v", err)
			}
		})
	}
}

// --- c. complete ------------------------------------------------------------------------------------

func TestPhotoCompleteStoresOnlyTheCleanReEncode(t *testing.T) {
	h := buildPhotos(t)
	raw := ppJPEG(t, 4000, 3000, 6) // stored landscape, viewed portrait, carrying EXIF + GPS
	h.seedPendingPhoto("jpg", raw, ppNow.Add(-2*time.Minute))

	f, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if f.Status != domain.StoredFileStored || f.MIMEType != storage.MIMEJPEG {
		t.Errorf("file = %+v", f)
	}
	// The scanner saw the RAW upload, before any decode.
	if !bytes.Equal(h.scanner.scanned, raw) {
		t.Error("the scan did not cover the uploaded bytes")
	}
	// Only the clean copy is in private, at the .jpg destination; the temp upload is gone.
	if len(h.objects.produced) != 1 || h.objects.produced[0] != ppDestKey() {
		t.Fatalf("produced = %v", h.objects.produced)
	}
	if h.objects.has(storage.BucketTemp, ppUploadKey("jpg")) {
		t.Error("the raw upload is still in temp")
	}
	if len(h.objects.promoted) != 0 {
		t.Error("the raw upload was PROMOTED — it must never reach private")
	}
	stored, _ := h.objects.get(storage.BucketPrivate, ppDestKey(), "")
	if bytes.Equal(stored, raw) || bytes.Contains(stored, []byte("Exif")) || bytes.Contains(stored, []byte("GPS-16.05N")) {
		t.Fatal("the stored object still carries EXIF / is the raw upload")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(stored))
	if err != nil || cfg.Width != 1920 || cfg.Height != 2560 {
		t.Errorf("stored %d×%d (err %v) — want oriented upright and fitted to 2560 on the long side", cfg.Width, cfg.Height, err)
	}
	sum := sha256.Sum256(stored)
	if f.SHA256 != hex.EncodeToString(sum[:]) || f.SizeBytes != int64(len(stored)) {
		t.Errorf("recorded facts are not the stored object's: %+v", f)
	}
	if got := strings.Join(h.files.transitions, ","); got != "pending->scanning" || len(h.files.marked) != 1 {
		t.Errorf("transitions = %s, marked = %d", got, len(h.files.marked))
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][2] != ActionPetitionPhotoStored || a[0][3] != ppCode || a[0][1] != "citizen" {
		t.Fatalf("trail = %v", a)
	}
	for _, want := range []string{`"loai_tai_len":"image/jpeg"`, `"ma_hoa_lai_bo_exif":true`, `"da_xoa_tep_tam":true`} {
		if !strings.Contains(a[0][4], want) {
			t.Errorf("delta lacks %s: %s", want, a[0][4])
		}
	}
}

func TestPhotoCompletePNGAndWebPComeOutJPEG(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 40, 30)))
	webpData, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	for _, c := range []struct {
		ext  string
		data []byte
		mime string
	}{{"png", buf.Bytes(), storage.MIMEPNG}, {"webp", webpData, storage.MIMEWebP}} {
		t.Run(c.ext, func(t *testing.T) {
			h := buildPhotos(t)
			h.seedPendingPhoto(c.ext, c.data, ppNow.Add(-time.Minute))
			f, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			stored, _ := h.objects.get(storage.BucketPrivate, ppDestKey(), "")
			if mime, _, _ := storage.SniffMIME(stored); mime != storage.MIMEJPEG || f.MIMEType != storage.MIMEJPEG {
				t.Errorf("stored as %q / recorded %q, want JPEG", mime, f.MIMEType)
			}
			if a := h.db.audits(); len(a) != 1 || !strings.Contains(a[0][4], `"loai_tai_len":"`+c.mime+`"`) {
				t.Errorf("trail = %v", a)
			}
		})
	}
}

func TestPhotoCompleteRejectionsPurgeTempAndAudit(t *testing.T) {
	for _, c := range []struct {
		name, ext, reason string
		data              func(*testing.T) []byte
		infected          bool
	}{
		{"malware", "jpg", RejectMalware, func(t *testing.T) []byte { return ppJPEG(t, 20, 10, 1) }, true},
		{"declared PNG, uploaded JPEG", "png", RejectTypeMismatch, func(t *testing.T) []byte { return ppJPEG(t, 20, 10, 1) }, false},
		{"not an image", "jpg", RejectTypeNotAllowed, func(*testing.T) []byte { return []byte("%PDF-1.7\n...") }, false},
		{"JPEG magic, garbage after", "jpg", RejectUndecodable, func(*testing.T) []byte {
			return append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 4, 1, 2}, bytes.Repeat([]byte{7}, 64)...)
		}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := buildPhotos(t)
			if c.infected {
				h.scanner.res = malwarescan.Result{Clean: false, Signature: "Eicar-Test-Signature"}
			}
			h.seedPendingPhoto(c.ext, c.data(t), ppNow.Add(-time.Minute))
			_, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
			var rej *AttachmentRejection
			if !errors.As(err, &rej) || rej.Reason != c.reason {
				t.Fatalf("err = %v, want rejection %s", err, c.reason)
			}
			if h.objects.has(storage.BucketTemp, ppUploadKey(c.ext)) || len(h.objects.produced) != 0 {
				t.Error("temp kept or something written to private")
			}
			if h.files.all(xaThu)[ppFileID].Status != domain.StoredFileRejected {
				t.Errorf("row status = %s", h.files.all(xaThu)[ppFileID].Status)
			}
			if a := h.db.audits(); len(a) != 1 || a[0][2] != ActionPetitionPhotoRejected || !strings.Contains(a[0][4], c.reason) {
				t.Errorf("trail = %v", a)
			}
		})
	}
}

func TestPhotoCompleteRetryableFailuresWriteNothing(t *testing.T) {
	for name, mod := range map[string]func(*ppHarness){
		"scanner unreachable":  func(h *ppHarness) { h.scanner.err = errors.New("clamd: dial tcp: refused") },
		"store write fails":    func(h *ppHarness) { h.objects.putErr = errors.New("minio: 500") },
		"scanner not set up":   func(h *ppHarness) { h.scanner.err = malwarescan.ErrNotConfigured },
		"platform unreachable": func(h *ppHarness) { h.policy.err = uploadpolicy.ErrUnavailable },
	} {
		t.Run(name, func(t *testing.T) {
			h := buildPhotos(t)
			h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
			mod(h)
			if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); err == nil {
				t.Fatal("no error")
			}
			if len(h.db.committed) != 0 || h.files.all(xaThu)[ppFileID].Status != domain.StoredFilePending {
				t.Error("a retryable failure wrote something or moved the row")
			}
			if !h.objects.has(storage.BucketTemp, ppUploadKey("jpg")) {
				t.Error("the upload was deleted although the completion can be retried")
			}
		})
	}
}

func TestPhotoCompleteScannerDownIs503Class(t *testing.T) {
	h := buildPhotos(t)
	h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
	h.scanner.err = errors.New("clamd timeout")
	if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrScanUnavailable) {
		t.Fatalf("err = %v, want ErrScanUnavailable", err)
	}
}

func TestPhotoCompletePetitionMovedBeforeOrDuring(t *testing.T) {
	t.Run("before: refused, nothing done", func(t *testing.T) {
		h := buildPhotos(t)
		h.pets.byCommune[xaThu][ppCode] = ppPetitionRow(domain.DangPhanLoai)
		h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
		if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrPhotoWindowClosed) {
			t.Fatalf("err = %v", err)
		}
		if h.scanner.calls != 0 || len(h.db.committed) != 0 {
			t.Error("processed a photo for a petition that had already moved on")
		}
	})
	t.Run("during: recorded as refused, clean copy removed", func(t *testing.T) {
		h := buildPhotos(t)
		moved := domain.DangPhanLoai
		h.pets.statusAtLk = &moved
		h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
		if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrPhotoWindowClosed) {
			t.Fatalf("err = %v", err)
		}
		if h.objects.has(storage.BucketPrivate, ppDestKey()) {
			t.Error("the clean copy of a refused photo is still in private")
		}
		if h.files.all(xaThu)[ppFileID].Status != domain.StoredFileRejected {
			t.Errorf("row = %s", h.files.all(xaThu)[ppFileID].Status)
		}
		if a := h.db.audits(); len(a) != 1 || !strings.Contains(a[0][4], RejectPetitionMoved) ||
			!strings.Contains(a[0][4], `"da_xoa_ban_sach":true`) {
			t.Errorf("trail = %v", a)
		}
	})
}

func TestPhotoCompleteCountReachedIsRejected(t *testing.T) {
	h := buildPhotos(t)
	for i := 0; i < 5; i++ {
		h.addPhoto(fmt.Sprintf("01JPHOTOOLD00000000000000%d", i), domain.StoredFileStored, ppNow.Add(-time.Hour))
	}
	h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
	_, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
	var rej *AttachmentRejection
	if !errors.As(err, &rej) || rej.Reason != RejectCountReached {
		t.Fatalf("err = %v", err)
	}
	if h.scanner.calls != 0 || len(h.objects.produced) != 0 {
		t.Error("a sixth photo was scanned or written")
	}
}

func TestPhotoCompleteOnlyOwnPetitionsPhoto(t *testing.T) {
	t.Run("another citizen's code", func(t *testing.T) {
		h := buildPhotos(t)
		h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
		_, err := h.uc.Complete(h.ctx, ppCode, ppFileID, audit.Actor{ID: ppOther, Kind: "citizen"})
		if !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a file of another petition", func(t *testing.T) {
		h := buildPhotos(t)
		h.seedPendingPhoto("jpg", ppJPEG(t, 20, 10, 1), ppNow.Add(-time.Minute))
		sf := h.files.all(xaThu)[ppFileID]
		sf.SubjectID = "pa-other"
		h.files.all(xaThu)[ppFileID] = sf
		if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrPhotoNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a staff task file id", func(t *testing.T) {
		h := buildPhotos(t)
		h.files.all(xaThu)[ppFileID] = domain.StoredFile{ID: ppFileID, SubjectType: "task", SubjectID: ppPetition,
			Purpose: "task-attachment", UploadedBy: "CB-00123", Status: domain.StoredFilePending}
		if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrPhotoNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPhotoCompleteAlreadyStoredAndNotYetAndExpired(t *testing.T) {
	t.Run("stored answers as stored", func(t *testing.T) {
		h := buildPhotos(t)
		h.addPhoto(ppFileID, domain.StoredFileStored, ppNow.Add(-time.Hour))
		f, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
		if err != nil || f.Status != domain.StoredFileStored || h.db.begun != 0 {
			t.Fatalf("f=%+v err=%v tx=%d", f, err, h.db.begun)
		}
	})
	t.Run("nothing uploaded yet", func(t *testing.T) {
		h := buildPhotos(t)
		h.seedPendingPhoto("jpg", nil, ppNow.Add(-time.Minute))
		if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrUploadNotReceived) {
			t.Fatalf("err = %v", err)
		}
		if h.db.begun != 0 {
			t.Error("a transaction for nothing")
		}
	})
	t.Run("form expired", func(t *testing.T) {
		h := buildPhotos(t)
		h.seedPendingPhoto("jpg", nil, ppNow.Add(-time.Hour))
		if _, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, ErrUploadExpired) {
			t.Fatalf("err = %v", err)
		}
		if h.files.all(xaThu)[ppFileID].Status != domain.StoredFileFailed {
			t.Error("expired row not failed")
		}
		if a := h.db.audits(); len(a) != 1 || a[0][2] != ActionPetitionPhotoExpired {
			t.Errorf("trail = %v", a)
		}
	})
	t.Run("recovered from the destination", func(t *testing.T) {
		h := buildPhotos(t)
		h.seedPendingPhoto("jpg", nil, ppNow.Add(-time.Minute))
		clean := ppJPEG(t, 20, 10, 1)
		h.objects.put(storage.BucketPrivate, ppDestKey(), clean)
		f, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
		if err != nil || f.Status != domain.StoredFileStored {
			t.Fatalf("f=%+v err=%v", f, err)
		}
		if a := h.db.audits(); len(a) != 1 || !strings.Contains(a[0][4], `"khoi_phuc_tu_dich":true`) {
			t.Errorf("trail = %v", a)
		}
	})
}

// --- lists ------------------------------------------------------------------------------------------

func TestPhotoListsSignOnlyStoredPhotosAfterTheIsolationCheck(t *testing.T) {
	h := buildPhotos(t)
	h.addPhoto("01JPHOTOSTORED000000000001", domain.StoredFileStored, ppNow.Add(-time.Hour))
	h.addPhoto("01JPHOTOPENDING00000000001", domain.StoredFilePending, ppNow.Add(-time.Minute))
	h.addPhoto("01JPHOTOREJECT000000000001", domain.StoredFileRejected, ppNow.Add(-time.Minute))

	links, err := h.uc.ListPhotos(h.ctx, ppCode, ppCitizenActor())
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].File.ID != "01JPHOTOSTORED000000000001" ||
		!links[0].ExpiresAt.Equal(ppNow.Add(storage.MaxDownloadTTL)) {
		t.Fatalf("links = %+v", links)
	}
	if len(h.objects.downloads) != 1 || h.objects.downloads[0].ttl != storage.MaxDownloadTTL ||
		h.objects.downloads[0].filename != domain.PetitionPhotoName {
		t.Errorf("signed = %+v", h.objects.downloads)
	}
	if _, err := h.uc.ListPhotos(h.ctx, ppCode, audit.Actor{ID: ppOther, Kind: "citizen"}); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Errorf("another citizen: %v", err)
	}
	h.uc.objects = nil
	if _, err := h.uc.ListPhotos(h.ctx, ppCode, ppCitizenActor()); !errors.Is(err, ErrUploadNotConfigured) {
		t.Errorf("no store: %v", err)
	}

	// The citizen's own reads left no trail (their own petition, as the detail route).
	if a := h.db.audits(); len(a) != 0 {
		t.Errorf("the citizen's own list wrote a trail: %v", a)
	}

	staff := NewStaffPetitionPhotos(h.store, h.pets, h.files, h.objects)
	staff.now = func() time.Time { return ppNow }
	if links, err := staff.ListPhotos(h.ctx, ppCode, false, ppStaffActor()); err != nil || len(links) != 1 {
		t.Errorf("staff list = %v, %v", links, err)
	}
	restricted := ppPetitionRow(domain.DangXuLy)
	restricted.LinhVuc = domain.LinhVucHanChe
	h.pets.byCommune[xaThu][ppCode] = restricted
	if _, err := staff.ListPhotos(h.ctx, ppCode, false, ppStaffActor()); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Errorf("restricted without the key: %v", err)
	}
	if _, err := staff.ListPhotos(h.ctx, ppCode, true, ppStaffActor()); err != nil {
		t.Errorf("restricted with the key: %v", err)
	}
	if _, err := staff.ListPhotos(ctxXa(tenant.ID("01JB"+strings.Repeat("B", 22))), ppCode, true, ppStaffActor()); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Errorf("another commune: %v", err)
	}
}

func ppStaffActor() audit.Actor { return audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"} }

// Rule 6, invariant 7: a scene photo cannot be masked, so a staff list that hands out links is a read of
// full personal data and leaves ONE committed entry per call — by the officer's business code, on the
// petition's code, naming the files and never a URL.
func TestStaffPhotoListIsAuditedBeforeTheLinksLeave(t *testing.T) {
	h := buildPhotos(t)
	h.addPhoto("01JPHOTOSTORED000000000001", domain.StoredFileStored, ppNow.Add(-time.Hour))
	h.addPhoto("01JPHOTOSTORED000000000002", domain.StoredFileStored, ppNow.Add(-time.Hour))
	staff := NewStaffPetitionPhotos(h.store, h.pets, h.files, h.objects)
	staff.now = func() time.Time { return ppNow }

	links, err := staff.ListPhotos(h.ctx, ppCode, false, ppStaffActor())
	if err != nil || len(links) != 2 {
		t.Fatalf("links = %v, err = %v", links, err)
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != "CB-00123" || a[0][1] != "staff" || a[0][2] != ActionPetitionPhotosViewed ||
		a[0][3] != ppCode {
		t.Fatalf("trail = %v", a)
	}
	for _, want := range []string{`"so_anh":2`, "01JPHOTOSTORED000000000001", "01JPHOTOSTORED000000000002"} {
		if !strings.Contains(a[0][4], want) {
			t.Errorf("delta lacks %s: %s", want, a[0][4])
		}
	}
	for _, never := range []string{"http", "Signature", domain.PetitionPhotoName, "citizen-media/"} {
		if strings.Contains(a[0][4], never) {
			t.Errorf("delta carries %q — a link, a name or a key: %s", never, a[0][4])
		}
	}

	t.Run("the trail cannot be written: no links", func(t *testing.T) {
		h.db.execErr = errors.New("audit_log: disk full")
		defer func() { h.db.execErr = nil }()
		links, err := staff.ListPhotos(h.ctx, ppCode, false, ppStaffActor())
		if err == nil || links != nil {
			t.Fatalf("links handed out without a trail: %v, %v", links, err)
		}
		if strings.Contains(err.Error(), ppCode) || strings.Contains(err.Error(), "CB-00123") {
			t.Errorf("error carries the code or the officer: %v", err)
		}
	})
	t.Run("no business code: refused before any read, no fallback", func(t *testing.T) {
		before := len(h.objects.downloads)
		for _, r := range []audit.Actor{{Kind: "staff", IP: "10.0.0.7"}, {ID: ppCitizen, Kind: "citizen"}} {
			if _, err := staff.ListPhotos(h.ctx, ppCode, false, r); err == nil {
				t.Errorf("reader %+v accepted", r)
			}
		}
		if len(h.objects.downloads) != before {
			t.Error("links signed for a reader with no business code")
		}
	})
	t.Run("nothing stored: nothing disclosed, nothing written", func(t *testing.T) {
		h2 := buildPhotos(t)
		s2 := NewStaffPetitionPhotos(h2.store, h2.pets, h2.files, h2.objects)
		if links, err := s2.ListPhotos(h2.ctx, ppCode, false, ppStaffActor()); err != nil || len(links) != 0 {
			t.Fatalf("links = %v, err = %v", links, err)
		}
		if h2.db.begun != 0 {
			t.Error("a transaction for an empty list")
		}
	})
}

func TestPhotoPurposePinnedToCoreStorage(t *testing.T) {
	if domain.PurposePetitionPhoto != string(storage.PurposePetitionPhoto) {
		t.Fatal("domain.PurposePetitionPhoto drifted from core/storage")
	}
}

// --- H2: memory held by concurrent completions ---------------------------------------------------------

// waitForWaiters blocks until n callers are waiting on the in-flight completion of ppFileID.
func waitForWaiters(t *testing.T, uc *CitizenPetitionPhotos, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		uc.mu.Lock()
		c := uc.inflight[string(xaThu)+"/"+ppFileID]
		got := 0
		if c != nil {
			got = c.waiters
		}
		uc.mu.Unlock()
		if got == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%d callers never joined the in-flight completion", n)
}

// Concurrent completions of ONE file read its bytes ONCE: the others wait for, and return, that answer.
func TestPhotoConcurrentCompletionsOfOneFileReadTheBytesOnce(t *testing.T) {
	h := buildPhotos(t)
	h.seedPendingPhoto("jpg", ppJPEG(t, 40, 30, 1), ppNow.Add(-time.Minute))
	h.objects.opened, h.objects.openGate = make(chan struct{}, 8), make(chan struct{})

	type result struct {
		f   domain.StoredFile
		err error
	}
	const followers = 5
	results := make(chan result, followers+1)
	run := func() {
		f, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
		results <- result{f, err}
	}
	go run()
	<-h.objects.opened // the first caller holds the bytes' read slot and is registered in flight
	for i := 0; i < followers; i++ {
		go run()
	}
	waitForWaiters(t, h.uc, followers)
	close(h.objects.openGate)

	for i := 0; i < followers+1; i++ {
		r := <-results
		if r.err != nil || r.f.ID != ppFileID || r.f.Status != domain.StoredFileStored {
			t.Errorf("caller %d: %+v, %v", i, r.f, r.err)
		}
	}
	if n := h.objects.opens.Load(); n != 1 {
		t.Errorf("the upload was opened %d times, want 1", n)
	}
	if h.scanner.calls != 1 || len(h.objects.produced) != 1 || len(h.db.audits()) != 1 {
		t.Errorf("scans %d, produced %d, trail %d — want one of each", h.scanner.calls, len(h.objects.produced),
			len(h.db.audits()))
	}
}

// A first caller that gives up does not strand the one waiting on it: that one does the work.
func TestPhotoWaitingCompletionTakesOverWhenTheFirstGivesUp(t *testing.T) {
	h := buildPhotos(t)
	h.seedPendingPhoto("jpg", ppJPEG(t, 40, 30, 1), ppNow.Add(-time.Minute))
	h.objects.opened, h.objects.openGate = make(chan struct{}, 8), make(chan struct{})

	firstCtx, cancel := context.WithCancel(h.ctx)
	firstErr := make(chan error, 1)
	go func() {
		_, err := h.uc.Complete(firstCtx, ppCode, ppFileID, ppCitizenActor())
		firstErr <- err
	}()
	<-h.objects.opened
	second := make(chan error, 1)
	go func() {
		_, err := h.uc.Complete(h.ctx, ppCode, ppFileID, ppCitizenActor())
		second <- err
	}()
	waitForWaiters(t, h.uc, 1)
	cancel()
	if err := <-firstErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("first caller: %v", err)
	}
	close(h.objects.openGate)
	if err := <-second; err != nil {
		t.Fatalf("the waiting caller was stranded: %v", err)
	}
	if h.files.all(xaThu)[ppFileID].Status != domain.StoredFileStored || h.objects.opens.Load() != 2 {
		t.Errorf("row %s, opens %d", h.files.all(xaThu)[ppFileID].Status, h.objects.opens.Load())
	}
}

// The read slot is taken BEFORE the bytes: with every slot held, a completion waits without reading.
func TestPhotoCompletionWaitsForAReadSlotBeforeReadingTheBytes(t *testing.T) {
	h := buildPhotos(t)
	h.seedPendingPhoto("jpg", ppJPEG(t, 40, 30, 1), ppNow.Add(-time.Minute))
	for i := 0; i < photoReadSlots; i++ {
		h.uc.readSlot <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(h.ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := h.uc.Complete(ctx, ppCode, ppFileID, ppCitizenActor()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the caller's deadline", err)
	}
	if n := h.objects.opens.Load(); n != 0 {
		t.Errorf("the upload was opened %d times with no read slot free", n)
	}
	if len(h.db.committed) != 0 || h.files.all(xaThu)[ppFileID].Status != domain.StoredFilePending ||
		!h.objects.has(storage.BucketTemp, ppUploadKey("jpg")) {
		t.Error("a completion that never got a slot changed something")
	}
	if len(h.uc.inflight) != 0 {
		t.Error("the abandoned completion is still registered in flight")
	}
}
