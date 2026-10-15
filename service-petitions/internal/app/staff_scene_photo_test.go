package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/storage"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The officer's scene photos at intake (owner decision 15/10/2026), on the citizen photo flow's fakes:
// the same object store (citizen-media keys only), scanner, policy and file register.
//
//	PROVED HERE   one request stores the re-encode WITHOUT EXIF under the citizen-media key, uploaded by
//	              the officer's BUSINESS CODE, with two trail entries in the officer's name · only a
//	              staff-booked petition, only while `da-tiep-nhan`, the count over every uploader, the
//	              restricted field, another commune, a citizen principal — each refused with nothing
//	              written · another officer's pending file is not completable · the stored photo is then
//	              listed by the staff read of scene photos.
//	NOT PROVED    PostgreSQL's CHECKs/trigger on such a row — stored_file_pg_test.go territory (SKIPS
//	              without VIGOV_TEST_DSN).

type sspHarness struct {
	uc      *StaffScenePhotos
	db      *ppDB
	store   *pkgstore.DB
	pets    *vpPetitions
	files   *ppFiles
	objects *ppObjects
	ctx     context.Context
}

// sspStaffBookedRow is a petition the staff intake booked: no owner, a staff channel, just received.
func sspStaffBookedRow(status domain.TrangThai) domain.PhieuPhanAnh {
	return domain.PhieuPhanAnh{ID: ppPetition, MaTraCuu: ppCode, Kenh: domain.ChannelSwitchboard,
		TrangThai: status, LinhVuc: "rac-thai"}
}

func buildStaffScenePhotos(t *testing.T) *sspHarness {
	t.Helper()
	d := &ppDB{}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := &sspHarness{
		db: d,
		pets: &vpPetitions{byCommune: map[tenant.ID]map[string]domain.PhieuPhanAnh{
			xaThu: {
				ppCode: sspStaffBookedRow(domain.DaTiepNhan),
				"PA-CANBO-0000-0001": {ID: "pa-canbo", MaTraCuu: "PA-CANBO-0000-0001", LinhVuc: domain.LinhVucHanChe,
					Kenh: domain.KenhCanBoNhapHo, TrangThai: domain.DaTiepNhan},
				"PA-CITIZEN-000-0001": {ID: "pa-citizen", MaTraCuu: "PA-CITIZEN-000-0001", CongDanID: ppCitizen,
					Kenh: domain.KenhZaloMiniApp, TrangThai: domain.DaTiepNhan},
				"PA-NOACCOUNT-00-001": {ID: "pa-noaccount", MaTraCuu: "PA-NOACCOUNT-00-001",
					Kenh: domain.KenhZaloMiniApp, TrangThai: domain.DaTiepNhan},
			},
		}},
		files:   &ppFiles{rows: map[tenant.ID]map[string]domain.StoredFile{}},
		objects: &ppObjects{objectStoreFake: newObjectStoreFake()},
		ctx:     ctxXa(xaThu),
	}
	h.store = pkgstore.New(db)
	citizen := NewCitizenPetitionPhotos(h.store, nil, nil, h.objects, &scannerFake{res: malwarescan.Result{Clean: true}},
		ppPhotoPolicy())
	h.uc = NewStaffScenePhotos(h.store, h.pets, h.files, citizen)
	h.uc.newID = func() (string, error) { return ppFileID, nil }
	h.uc.now = func() time.Time { return ppNow }
	return h
}

func TestStaffScenePhoto_UploadStoresTheReEncodeByTheOfficer(t *testing.T) {
	h := buildStaffScenePhotos(t)
	raw := ppJPEG(t, 64, 32, 6)
	f, err := h.uc.Upload(h.ctx, ppCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: int64(len(raw))},
		uploadBodyOf(raw), canBoThu(), QuyenXemHanChe(false))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if f.Status != domain.StoredFileStored || f.ObjectKey != ppDestKey() || f.UploadedBy != maCanBoThu ||
		f.RetentionClass != string(storage.ClassCitizenMedia) || f.Bucket != domain.StoredFileBucketPrivate ||
		f.Purpose != domain.PurposePetitionPhoto || f.OriginalName != domain.PetitionPhotoName {
		t.Errorf("stored = %+v", f)
	}
	stored, err := h.objects.get(storage.BucketPrivate, ppDestKey(), "")
	if err != nil || bytes.Contains(stored, []byte("GPS-16.05N")) {
		t.Errorf("clean copy: err %v, EXIF still there = %v", err, bytes.Contains(stored, []byte("GPS-16.05N")))
	}
	if h.objects.has(storage.BucketTemp, ppUploadKey("jpg")) {
		t.Error("the raw upload (with EXIF) stayed in temp")
	}
	a := h.db.audits()
	if len(a) != 2 || a[0][2] != ActionStaffScenePhotoRequested || a[1][2] != ActionStaffScenePhotoStored {
		t.Fatalf("trail = %v", a)
	}
	for _, e := range a {
		if e[0] != maCanBoThu || e[1] != "staff" || e[3] != ppCode {
			t.Errorf("trail entry = %v — want the officer's BUSINESS CODE on the petition's code (rule 6, inv 8)", e)
		}
	}
	if h.pets.locked < 2 {
		t.Errorf("petition locked %d times — reserve and completion each decide on the locked row", h.pets.locked)
	}
}

func TestStaffScenePhoto_RequestRefusalsWriteNothing(t *testing.T) {
	for name, c := range map[string]struct {
		setup func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor)
		want  error
	}{
		"a citizen's petition": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			return "PA-CITIZEN-000-0001", false, canBoThu()
		}, ErrScenePhotoNotStaffBooked},
		"an accountless Mini App petition": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			return "PA-NOACCOUNT-00-001", false, canBoThu()
		}, ErrScenePhotoNotStaffBooked},
		"moved on from da-tiep-nhan": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			h.pets.byCommune[xaThu][ppCode] = sspStaffBookedRow(domain.DangPhanLoai)
			return ppCode, false, canBoThu()
		}, ErrPhotoWindowClosed},
		"full, every uploader and live pending slots counted": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			for i, s := range []domain.StoredFileStatus{"stored", "stored", "ready", "stored", "pending"} {
				id := "s" + string(rune('a'+i))
				h.files.all(xaThu)[id] = domain.StoredFile{ID: id, SubjectType: domain.StoredFileSubjectPetition,
					SubjectID: ppPetition, Purpose: domain.PurposePetitionPhoto, Status: s, UploadedBy: "CB-00999",
					CreatedAt: ppNow}
			}
			return ppCode, false, canBoThu()
		}, ErrPhotoCountReached},
		"restricted field without the key": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			return "PA-CANBO-0000-0001", false, canBoThu()
		}, ErrPhieuHanChe},
		"another commune's code": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			h.ctx = ctxXa(xaKia)
			return ppCode, false, canBoThu()
		}, petstore.ErrPhieuKhongTonTai},
		"a citizen principal": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			return ppCode, false, ppCitizenActor()
		}, nil},
		"an officer with no business code": {func(h *sspHarness) (string, QuyenXemHanChe, audit.Actor) {
			a := canBoThu()
			a.ID = ""
			return ppCode, false, a
		}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildStaffScenePhotos(t)
			code, restricted, actor := c.setup(h)
			_, err := h.uc.reserve(h.ctx, code, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
				actor, restricted)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if len(h.files.inserted) != 0 || len(h.db.audits()) != 0 {
				t.Error("a refused request wrote a row or a trail")
			}
		})
	}
	// With `feedback.restricted`, the `can-bo` staff-booked petition is open.
	h := buildStaffScenePhotos(t)
	if _, err := h.uc.reserve(h.ctx, "PA-CANBO-0000-0001", PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
		canBoThu(), true); err != nil {
		t.Errorf("restricted with the key: %v", err)
	}
}

// The completion is the UPLOADER's alone: another officer's pending file answers ErrPhotoNotFound, and a
// citizen's pending photo on the same petition is not an officer's to complete either.
func TestStaffScenePhoto_CompletionIsTheUploadersAlone(t *testing.T) {
	for name, uploader := range map[string]string{"another officer": "CB-00999", "the citizen": domain.CitizenLogActor} {
		t.Run(name, func(t *testing.T) {
			h := buildStaffScenePhotos(t)
			h.files.all(xaThu)[ppFileID] = domain.StoredFile{
				ID: ppFileID, Bucket: domain.StoredFileBucketPrivate, ObjectKey: ppDestKey(),
				RetentionClass: string(storage.ClassCitizenMedia), Purpose: domain.PurposePetitionPhoto,
				SubjectType: domain.StoredFileSubjectPetition, SubjectID: ppPetition,
				OriginalName: domain.PetitionPhotoName, Status: domain.StoredFilePending, UploadedBy: uploader,
				CreatedAt: ppNow, UpdatedAt: ppNow,
			}
			if _, err := h.uc.complete(h.ctx, ppCode, ppFileID, canBoThu(), false); !errors.Is(err, ErrPhotoNotFound) {
				t.Fatalf("err = %v, want ErrPhotoNotFound", err)
			}
			if len(h.db.audits()) != 0 || len(h.files.transitions) != 0 {
				t.Error("another uploader's row was touched")
			}
		})
	}
}

// Not configured is a 503-shaped refusal before any row, never an upload with no scan.
func TestStaffScenePhoto_NotConfiguredRefusesFirst(t *testing.T) {
	h := buildStaffScenePhotos(t)
	uc := NewStaffScenePhotos(h.store, h.pets, h.files, nil)
	if _, err := uc.MaxUploadBytes(h.ctx); !errors.Is(err, ErrUploadNotConfigured) {
		t.Errorf("MaxUploadBytes err = %v", err)
	}
	if _, err := uc.reserve(h.ctx, ppCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 10}, canBoThu(),
		false); !errors.Is(err, ErrUploadNotConfigured) {
		t.Errorf("reserve err = %v", err)
	}
	if len(h.files.inserted) != 0 {
		t.Error("a row was written with no store configured")
	}
}

// The photo the officer stored is what the staff detail's "TRƯỚC KHI XỬ LÝ" then lists.
func TestStaffScenePhoto_StoredPhotoIsListedToStaff(t *testing.T) {
	h := buildStaffScenePhotos(t)
	raw := ppJPEG(t, 16, 16, 1)
	if _, err := h.uc.Upload(h.ctx, ppCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: int64(len(raw))},
		uploadBodyOf(raw), canBoThu(), false); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	read := NewStaffPetitionPhotos(h.store, h.pets, h.files, h.objects)
	read.now = func() time.Time { return ppNow }
	links, err := read.ListPhotos(h.ctx, ppCode, false, canBoThu())
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(links) != 1 || links[0].File.ID != ppFileID || links[0].File.UploadedBy != maCanBoThu {
		t.Errorf("links = %+v — the officer's scene photo must be listed", links)
	}
}
