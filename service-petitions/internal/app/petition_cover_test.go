package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The register list's cover photo (StaffPetitionPhotos.Covers, owner decision 12/10/2026).
//
//	PROVED HERE   the first stored citizen photo per petition, signed for storage.MaxDownloadTTL · a
//	              petition with none is absent · `can-bo` without `feedback.restricted` is absent and its
//	              photos are never read · ONE batched read and ONE committed entry for the page, by the
//	              officer's code, naming files and never a URL or a petition code · the entry cannot be
//	              written → no links · no business code / no object store → refused before any read.
//	NOT PROVED    the SQL (store/petition_photo_test.go) and PostgreSQL (stored_file_pg_test.go).

// PetitionCoverPhotos on the fake: the same selection the store makes — citizen photos that reached the
// destination, oldest first per petition, the first one kept — and it records each call.
func (f *ppFiles) PetitionCoverPhotos(ctx context.Context, petitionIDs []string) ([]domain.StoredFile, error) {
	f.coverCalls = append(f.coverCalls, append([]string(nil), petitionIDs...))
	all := []domain.StoredFile{}
	for _, sf := range f.all(tenant.MustFrom(ctx)) {
		all = append(all, sf)
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.Before(all[j].CreatedAt)
		}
		return all[i].ID < all[j].ID
	})
	out := []domain.StoredFile{}
	seen := map[string]bool{}
	for _, id := range petitionIDs {
		for _, sf := range all {
			if !seen[id] && domain.IsCitizenPhotoOf(sf, id) && sf.Status.Attachable() {
				seen[id] = true
				out = append(out, sf)
			}
		}
	}
	return out, nil
}

const (
	cvRestricted   = "pa-01JRESTRICTED"
	cvRestrictedMa = "PA-RSTR-CTED-0001"
	cvBare         = "pa-01JNOPHOTO"
)

// cvPage is one register page: the ordinary petition with photos, a `can-bo` one with a photo, and one
// with no photo at all.
func cvPage(h *ppHarness) []domain.PhieuPhanAnh {
	restricted := domain.PhieuPhanAnh{ID: cvRestricted, MaTraCuu: cvRestrictedMa, LinhVuc: domain.LinhVucHanChe,
		TrangThai: domain.DangXuLy}
	h.files.all(xaThu)["01JPHOTORESTRICTED00000001"] = domain.StoredFile{
		ID: "01JPHOTORESTRICTED00000001", Bucket: domain.StoredFileBucketPrivate,
		ObjectKey: "citizen-media/k/r/original.jpg", RetentionClass: "citizen-media",
		Purpose: domain.PurposePetitionPhoto, SubjectType: domain.StoredFileSubjectPetition, SubjectID: cvRestricted,
		OriginalName: domain.PetitionPhotoName, Status: domain.StoredFileStored, UploadedBy: domain.CitizenLogActor,
		CreatedAt: ppNow.Add(-time.Hour),
	}
	return []domain.PhieuPhanAnh{ppPetitionRow(domain.DaTiepNhan), restricted,
		{ID: cvBare, MaTraCuu: "PA-NOPH-OTO0-0001", TrangThai: domain.DaTiepNhan}}
}

func cvStaff(h *ppHarness) *StaffPetitionPhotos {
	s := NewStaffPetitionPhotos(h.store, h.pets, h.files, h.objects)
	s.now = func() time.Time { return ppNow }
	return s
}

func TestCoversFirstPhotoSignedAuditedOnce(t *testing.T) {
	h := buildPhotos(t)
	// The SECOND photo is added first and is newer: the cover must be the OLDEST one, not insertion order.
	h.addPhoto("01JPHOTOSECOND000000000002", domain.StoredFileStored, ppNow.Add(-time.Hour))
	h.addPhoto("01JPHOTOFIRST0000000000001", domain.StoredFileStored, ppNow.Add(-2*time.Hour))
	h.addPhoto("01JPHOTOPENDING00000000001", domain.StoredFilePending, ppNow.Add(-3*time.Hour))
	page := cvPage(h)

	covers, err := cvStaff(h).Covers(h.ctx, page, false, ppStaffActor())
	if err != nil {
		t.Fatal(err)
	}
	c, ok := covers[ppPetition]
	if !ok || c.File.ID != "01JPHOTOFIRST0000000000001" || c.URL == "" ||
		!c.ExpiresAt.Equal(ppNow.Add(storage.MaxDownloadTTL)) {
		t.Fatalf("cover = %+v (present %v) — want the oldest STORED photo, expiring at now+%v", c, ok,
			storage.MaxDownloadTTL)
	}
	if storage.MaxDownloadTTL > 15*time.Minute {
		t.Errorf("link lifetime %v above the 15 minutes the owner allowed", storage.MaxDownloadTTL)
	}
	if _, ok := covers[cvBare]; ok {
		t.Error("a petition with no photo has a cover")
	}
	if _, ok := covers[cvRestricted]; ok {
		t.Error("`can-bo` cover handed out without feedback.restricted")
	}
	if len(covers) != 1 {
		t.Errorf("covers = %v", covers)
	}

	// ONE batched read for the page, and the restricted petition was never even asked for.
	if len(h.files.coverCalls) != 1 {
		t.Fatalf("%d cover reads for one page — want ONE", len(h.files.coverCalls))
	}
	for _, id := range h.files.coverCalls[0] {
		if id == cvRestricted {
			t.Error("the restricted petition's photos were read")
		}
	}

	// ONE committed entry for the page, by the officer's business code, naming the file — never a URL,
	// never a petition code (the audit screen withholds `can-bo` by subject, not by delta).
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != "CB-00123" || a[0][1] != "staff" || a[0][2] != ActionPetitionPhotosViewed ||
		a[0][3] != CoverAuditSubject {
		t.Fatalf("trail = %v", a)
	}
	if !strings.Contains(a[0][4], "01JPHOTOFIRST0000000000001") || !strings.Contains(a[0][4], `"so_anh":1`) {
		t.Errorf("delta = %s", a[0][4])
	}
	for _, never := range []string{"http", "Signature", ppCode, cvRestrictedMa, "citizen-media/"} {
		if strings.Contains(a[0][4], never) {
			t.Errorf("delta carries %q: %s", never, a[0][4])
		}
	}
}

func TestCoversRestrictedWithTheKey(t *testing.T) {
	h := buildPhotos(t)
	h.addPhoto("01JPHOTOFIRST0000000000001", domain.StoredFileStored, ppNow.Add(-2*time.Hour))
	covers, err := cvStaff(h).Covers(h.ctx, cvPage(h), true, ppStaffActor())
	if err != nil {
		t.Fatal(err)
	}
	if covers[cvRestricted].File.ID != "01JPHOTORESTRICTED00000001" || covers[ppPetition].File.ID == "" {
		t.Errorf("covers = %v", covers)
	}
	if a := h.db.audits(); len(a) != 1 || !strings.Contains(a[0][4], `"so_anh":2`) {
		t.Errorf("trail = %v — want ONE entry for both covers", a)
	}
}

func TestCoversNoPhotoNoTrail(t *testing.T) {
	h := buildPhotos(t)
	covers, err := cvStaff(h).Covers(h.ctx, []domain.PhieuPhanAnh{ppPetitionRow(domain.DaTiepNhan)}, false,
		ppStaffActor())
	if err != nil || covers == nil || len(covers) != 0 {
		t.Fatalf("covers = %v, err = %v", covers, err)
	}
	if h.db.begun != 0 || len(h.objects.downloads) != 0 {
		t.Error("a page with nothing to disclose opened a transaction or signed a link")
	}
	// An empty page asks nothing at all.
	if _, err := cvStaff(h).Covers(h.ctx, nil, false, ppStaffActor()); err != nil || len(h.files.coverCalls) != 1 {
		t.Errorf("empty page: err %v, reads %d", err, len(h.files.coverCalls))
	}
}

func TestCoversTrailFailsNoLinks(t *testing.T) {
	h := buildPhotos(t)
	h.addPhoto("01JPHOTOFIRST0000000000001", domain.StoredFileStored, ppNow.Add(-2*time.Hour))
	h.db.execErr = errors.New("audit_log: disk full")
	covers, err := cvStaff(h).Covers(h.ctx, cvPage(h), false, ppStaffActor())
	if err == nil || covers != nil {
		t.Fatalf("links handed out without a trail: %v, %v", covers, err)
	}
	if strings.Contains(err.Error(), ppCode) || strings.Contains(err.Error(), "CB-00123") {
		t.Errorf("error carries the code or the officer: %v", err)
	}
}

func TestCoversRefusedBeforeAnyRead(t *testing.T) {
	h := buildPhotos(t)
	h.addPhoto("01JPHOTOFIRST0000000000001", domain.StoredFileStored, ppNow.Add(-2*time.Hour))
	s := cvStaff(h)
	for _, r := range []audit.Actor{{Kind: "staff", IP: "10.0.0.7"}, {ID: ppCitizen, Kind: "citizen"}} {
		if _, err := s.Covers(h.ctx, cvPage(h), false, r); !errors.Is(err, ErrPhotoReaderUnattributed) {
			t.Errorf("reader %+v: err = %v", r, err)
		}
	}
	s.objects = nil
	if _, err := s.Covers(h.ctx, cvPage(h), false, ppStaffActor()); !errors.Is(err, ErrUploadNotConfigured) {
		t.Errorf("no object store: err = %v", err)
	}
	if len(h.files.coverCalls) != 0 || len(h.objects.downloads) != 0 || h.db.begun != 0 {
		t.Error("a refused call read, signed or wrote")
	}
}
