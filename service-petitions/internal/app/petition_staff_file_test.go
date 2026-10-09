package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"strings"
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

// The staff files on a petition (migration 0027) and the close gate (ADR 0008 decision 3).
//
//	PROVED HERE   close gate: switch on + no stored photo → refused, NOTHING written, rolled back · switch
//	              on + a photo → closed · switch off → closed with none · a switch read failure and a
//	              missing wiring both refuse (fail closed) · the count is over THIS petition, THIS purpose,
//	              stored rows only · the read runs inside the closing transaction.
//	              note: attachment ids are checked under lock and linked to the row just written, in the
//	              same transaction; a foreign file refuses and writes nothing.
//	              verification photo: staff only, `feedback.restricted` narrows, every status but the
//	              endings, ≤ policy count incl. live slots · the row is records / private / the officer's
//	              BUSINESS CODE · completion re-encodes WITHOUT EXIF and stores ONLY the clean JPEG · another
//	              officer's upload is 404 · a late refusal under the lock does NOT purge the records copy and
//	              says so in the trail · the staff list is audited by business code · the citizen list is the
//	              petition's own citizen only, purpose-bound, never a log attachment · slots are SHARED
//	              with the citizen flow.
//	              log attachment: the note rule decides, the file is promoted as uploaded, the download is
//	              audited and committed before the link is returned.
//	NOT PROVED    PostgreSQL (0027's triggers — stored_file_pg_test.go SKIPS without VIGOV_TEST_DSN).

// --- the close gate's two fakes (dungXuLy wires both) ---------------------------------------------------

type closeSwitchFake struct {
	required bool
	err      error
	calls    int
	inTx     bool
}

func (f *closeSwitchFake) VerificationPhotoRequiredTx(_ context.Context, tx *pkgstore.ScopedTx) (bool, error) {
	f.calls++
	f.inTx = tx != nil
	return f.required, f.err
}

type staffFilesFake struct {
	photos   int
	countErr error
	counted  []string // "subjectType|subjectID|purpose|pendingWindow"

	photosAfter int         // stored photos uploaded after the reopening cut
	cuts        []time.Time // the cuts CountStoredCreatedAfterTx was asked for

	cands      map[string]domain.AttachCandidate
	linkedTo   string
	linkedIDs  []string
	linkCalled int
}

func (f *staffFilesFake) CountForSubjectTx(_ context.Context, tx *pkgstore.ScopedTx, subjectType, subjectID,
	purpose string, pendingSince time.Time) (int, error) {
	window := "stored-only"
	if !pendingSince.IsZero() {
		window = "with-pending"
	}
	f.counted = append(f.counted, subjectType+"|"+subjectID+"|"+purpose+"|"+window)
	if tx == nil {
		return 0, errors.New("count outside a transaction")
	}
	return f.photos, f.countErr
}

// CountStoredCreatedAfterTx answers photosAfter — the stored photos uploaded after the cut — and records
// the cut it was asked for.
func (f *staffFilesFake) CountStoredCreatedAfterTx(_ context.Context, tx *pkgstore.ScopedTx, subjectType, subjectID,
	purpose string, after time.Time) (int, error) {
	f.counted = append(f.counted, subjectType+"|"+subjectID+"|"+purpose+"|after")
	f.cuts = append(f.cuts, after)
	if tx == nil {
		return 0, errors.New("count outside a transaction")
	}
	return f.photosAfter, f.countErr
}

func (f *staffFilesFake) PetitionAttachCandidates(_ context.Context, _ *pkgstore.ScopedTx, ids []string) (
	map[string]domain.AttachCandidate, error) {
	out := map[string]domain.AttachCandidate{}
	for _, id := range ids {
		if c, ok := f.cands[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

func (f *staffFilesFake) LinkToPetitionLogEntry(_ context.Context, _ *pkgstore.ScopedTx, entry string, ids []string) error {
	f.linkCalled++
	f.linkedTo, f.linkedIDs = entry, ids
	return nil
}

func closablePetition() *khoPhieuXuLyGia {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.ChoDanXacNhan), "xu_ly_xong_luc": mocThaoTac, "linh_vuc": "rac-thai",
	})
	return k
}

func TestCloseGate_SwitchOnNoPhotoRefusesAndWritesNothing(t *testing.T) {
	k := closablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	sw, files := &closeSwitchFake{required: true}, &staffFilesFake{photos: 0}
	uc.settings, uc.staffFiles = sw, files

	_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, domain.ErrVerificationPhotoRequired) {
		t.Fatalf("err = %v, want ErrVerificationPhotoRequired", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") || k.coCau("INSERT INTO nhat_ky_phan_anh") {
		t.Error("closed, audited or logged although the gate refused")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d rollback %d — want 0 and 1", k.daCommit, k.daRollback)
	}
	if !sw.inTx {
		t.Error("the switch was read outside the closing transaction")
	}
	if len(files.counted) != 1 || files.counted[0] !=
		domain.StoredFileSubjectPetition+"|"+idPhieuThu+"|"+domain.PurposePetitionVerificationPhoto+"|stored-only" {
		t.Errorf("counted %v — want THIS petition, the verification purpose, stored rows only", files.counted)
	}
	if strings.Contains(err.Error(), maPhieuThu) {
		t.Errorf("error carries the lookup code: %v", err)
	}
}

func TestCloseGate_SwitchOnWithPhotoCloses(t *testing.T) {
	k := closablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, &staffFilesFake{photos: 2}
	sau, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe)
	if err != nil || sau.TrangThai != domain.DaDong {
		t.Fatalf("Dong = %v, %v", sau.TrangThai, err)
	}
	if !k.coCau("UPDATE phieu_phan_anh") || k.daCommit != 1 {
		t.Error("closing not written")
	}
}

func TestCloseGate_SwitchOffClosesWithoutPhoto(t *testing.T) {
	k := closablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{photos: 0}
	uc.settings, uc.staffFiles = &closeSwitchFake{required: false}, files
	if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("Dong with the switch off: %v", err)
	}
	if len(files.counted) != 0 {
		t.Error("photos counted although the switch is off")
	}
}

// A commune with NO settings row is ON: that answer is the store's (petition_settings_test.go,
// TestVerificationPhotoRequiredTxNoRowIsTrueAndScoped); here, an ON answer with no photo refuses —
// together the two prove "row-less commune = on" end to end, one layer each.
func TestCloseGate_RowlessCommuneIsOn(t *testing.T) {
	k := closablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	// What PetitionSettingsStore answers for a commune with no row.
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, &staffFilesFake{}
	if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe); !errors.Is(err, domain.ErrVerificationPhotoRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestCloseGate_FailsClosed(t *testing.T) {
	for name, set := range map[string]func(uc *XuLyPhanAnh){
		"switch read fails": func(uc *XuLyPhanAnh) {
			uc.settings, uc.staffFiles = &closeSwitchFake{err: errors.New("db down")}, &staffFilesFake{photos: 1}
		},
		"count fails": func(uc *XuLyPhanAnh) {
			uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, &staffFilesFake{countErr: errors.New("db down")}
		},
		"switch not wired": func(uc *XuLyPhanAnh) { uc.settings = nil },
		"files not wired":  func(uc *XuLyPhanAnh) { uc.staffFiles = nil },
	} {
		t.Run(name, func(t *testing.T) {
			k := closablePetition()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			set(uc)
			if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe); err == nil {
				t.Fatal("closed although the gate could not be checked")
			}
			if k.coCau("UPDATE phieu_phan_anh") || k.daCommit != 0 {
				t.Error("closing written although the gate could not be checked")
			}
		})
	}
}

// The state refusal comes first: a petition that cannot be closed anyway never reaches the gate.
func TestCloseGate_AfterTheLifecycleCheck(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DangXuLy)})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	sw := &closeSwitchFake{required: true}
	uc.settings, uc.staffFiles = sw, &staffFilesFake{}
	if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe); !errors.Is(err, domain.ErrDongSaiLuc) {
		t.Fatalf("err = %v, want ErrDongSaiLuc", err)
	}
	if sw.calls != 0 {
		t.Error("gate read before the lifecycle check")
	}
}

// --- the citizen's view of verification photos by status (owner decision (b), 02/10/2026) ----------

func TestVerificationPhoto_CitizenSeesThemOnlyFromAwaitingConfirmation(t *testing.T) {
	reopened := vpPetitionRow(domain.DangXuLy, "rac-thai")
	reopened.SoLanMoLai = 1 // shown at cho-dan-xac-nhan before, reopened since: hidden again
	for name, c := range map[string]struct {
		row  domain.PhieuPhanAnh
		want int
	}{
		"dang-xu-ly":                   {vpPetitionRow(domain.DangXuLy, "rac-thai"), 0},
		"da-xu-ly":                     {vpPetitionRow(domain.DaXuLy, "rac-thai"), 0},
		"reopened, back at dang-xu-ly": {reopened, 0},
		"khong-tiep-nhan":              {vpPetitionRow(domain.KhongTiepNhan, "rac-thai"), 0},
		"cho-dan-xac-nhan":             {vpPetitionRow(domain.ChoDanXacNhan, "rac-thai"), 1},
		"da-dong":                      {vpPetitionRow(domain.DaDong, "rac-thai"), 1},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildVerification(t)
			h.pets.byCommune[xaThu][vpCode] = c.row
			h.addFile("v1", domain.PurposePetitionVerificationPhoto, domain.StoredFileStored)
			uc := NewCitizenVerificationPhotos(h.pets, h.files, h.objects)
			links, err := uc.ListPhotos(h.ctx, vpCode, ppCitizenActor())
			if err != nil {
				t.Fatalf("ListPhotos: %v — the petition is the citizen's own, so never an error", err)
			}
			if links == nil {
				t.Fatal("nil list — the route must answer `items: []`, not null")
			}
			if len(links) != c.want {
				t.Errorf("links = %d, want %d", len(links), c.want)
			}
			if len(h.objects.uploads) != 0 {
				t.Error("an upload was written on a read")
			}
		})
	}
	// Hidden is NOT a different answer for somebody else's petition: another citizen still gets the 404.
	h := buildVerification(t)
	uc := NewCitizenVerificationPhotos(h.pets, h.files, h.objects)
	if _, err := uc.ListPhotos(h.ctx, vpCode, audit.Actor{ID: ppOther, Kind: "citizen"}); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Errorf("another citizen at dang-xu-ly: %v, want the one 404", err)
	}
}

// --- the close gate on a REOPENED petition (owner decision (c), 02/10/2026) ---------------------------

// reopenedAtFixture is when the citizen's 1–2 star rating reopened the petition — before the close.
var reopenedAtFixture = time.Date(2026, 9, 21, 10, 11, 12, 0, time.UTC)

func reopenedClosablePetition() *khoPhieuXuLyGia {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.ChoDanXacNhan), "xu_ly_xong_luc": mocThaoTac, "linh_vuc": "rac-thai",
		"so_lan_mo_lai": int64(1),
	})
	k.latestReopen = reopenedAtFixture
	return k
}

func TestCloseGate_ReopenedWithOnlyOldPhotosRefuses(t *testing.T) {
	k := reopenedClosablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{photos: 3, photosAfter: 0} // three photos, all from before the reopening
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, files

	_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, domain.ErrVerificationPhotoRequired) {
		t.Fatalf("err = %v, want ErrVerificationPhotoRequired", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") || k.coCau("INSERT INTO nhat_ky_phan_anh") {
		t.Error("closed, audited or logged although the gate refused")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d rollback %d — want 0 and 1", k.daCommit, k.daRollback)
	}
	if len(files.counted) != 1 || files.counted[0] !=
		domain.StoredFileSubjectPetition+"|"+idPhieuThu+"|"+domain.PurposePetitionVerificationPhoto+"|after" {
		t.Errorf("counted %v — want ONLY the after-the-reopening count of THIS petition's verification photos",
			files.counted)
	}
	if len(files.cuts) != 1 || !files.cuts[0].Equal(reopenedAtFixture) {
		t.Errorf("cut = %v, want the latest reopening %v", files.cuts, reopenedAtFixture)
	}
	if strings.Contains(err.Error(), maPhieuThu) {
		t.Errorf("error carries the lookup code: %v", err)
	}
	// The reopening instant is read IN the closing transaction, scoped, from the reopening rows only.
	q := k.cau("max(thoi_diem)")
	if len(q) != 1 || !q[0].trongGiaoDich {
		t.Fatalf("reopen read = %+v — want one read inside the transaction", q)
	}
	for _, frag := range []string{"FROM nhat_ky_phan_anh WHERE tenant_id = $1", "phieu_phan_anh_id = $2", "hanh_vi = $3"} {
		if !strings.Contains(q[0].sql, frag) {
			t.Errorf("reopen read lacks %q: %s", frag, q[0].sql)
		}
	}
	if len(q[0].args) != 3 || q[0].args[0] != string(xaThu) || q[0].args[1] != idPhieuThu ||
		q[0].args[2] != string(domain.LogActionReopenByRating) {
		t.Errorf("reopen read args = %v", q[0].args)
	}
}

func TestCloseGate_ReopenedWithANewPhotoCloses(t *testing.T) {
	k := reopenedClosablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, &staffFilesFake{photos: 4, photosAfter: 1}
	after, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe)
	if err != nil || after.TrangThai != domain.DaDong {
		t.Fatalf("Dong = %v, %v", after.TrangThai, err)
	}
	if !k.coCau("UPDATE phieu_phan_anh") || k.daCommit != 1 {
		t.Error("closing not written")
	}
}

func TestCloseGate_ReopenedSwitchOffClosesRegardless(t *testing.T) {
	k := reopenedClosablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{}
	uc.settings, uc.staffFiles = &closeSwitchFake{required: false}, files
	if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("Dong with the switch off: %v", err)
	}
	if len(files.counted) != 0 || k.coCau("max(thoi_diem)") {
		t.Error("photos counted or the reopening read although the switch is off")
	}
}

// A counter above zero with no reopening row on the timeline: the gate cannot tell old from new, so it
// refuses as a fault (500) — never ErrVerificationPhotoRequired, never a close on the old photos.
func TestCloseGate_ReopenedWithoutATimelineRowFailsClosed(t *testing.T) {
	k := reopenedClosablePetition()
	k.latestReopen = nil
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{photos: 5, photosAfter: 5}
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, files
	_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, errReopenInstantMissing) || errors.Is(err, domain.ErrVerificationPhotoRequired) {
		t.Fatalf("err = %v, want errReopenInstantMissing", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.daCommit != 0 || len(files.counted) != 0 {
		t.Error("closed or counted although the reopening instant is unknown")
	}
}

// Never reopened: the rule is unchanged — any stored photo, and the timeline is not read.
func TestCloseGate_NeverReopenedCountsEveryStoredPhoto(t *testing.T) {
	k := closablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{photos: 1, photosAfter: 0}
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, files
	if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("Dong: %v", err)
	}
	if k.coCau("max(thoi_diem)") || len(files.cuts) != 0 {
		t.Error("a never-reopened petition was measured against a reopening")
	}
	if len(files.counted) != 1 || !strings.HasSuffix(files.counted[0], "|stored-only") {
		t.Errorf("counted %v — want the unchanged every-stored-photo count", files.counted)
	}
}

// --- the note's attachments ------------------------------------------------------------------------------

func petitionLogCandidate(id, petitionID, uploader string, status domain.StoredFileStatus, linked string) domain.AttachCandidate {
	return domain.AttachCandidate{File: domain.StoredFile{ID: id, SubjectType: domain.StoredFileSubjectPetition,
		SubjectID: petitionID, Purpose: domain.PurposePetitionLogAttachment, UploadedBy: uploader, Status: status,
		OriginalName: "bien-ban.pdf", MIMEType: "application/pdf", SizeBytes: 10}, LinkedTo: linked}
}

func TestNoteLinksAttachmentsToTheRowItWrote(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuDaGiaoCho(maCanBoThu)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{photos: 1, cands: map[string]domain.AttachCandidate{
		"f1": petitionLogCandidate("f1", idPhieuThu, maCanBoThu, domain.StoredFileStored, ""),
	}}
	uc.staffFiles = files
	row, attached, err := uc.GhiChuNoiBo(ctx, maPhieuThu, ghiChuThu, []string{"f1"}, canBoThu(),
		khongQuyenGhiChu, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("GhiChuNoiBo: %v", err)
	}
	if files.linkCalled != 1 || files.linkedTo != row.ID || len(files.linkedIDs) != 1 {
		t.Errorf("linked %v to %q, want [f1] to the row just written %q", files.linkedIDs, files.linkedTo, row.ID)
	}
	if len(attached) != 1 || attached[0].FileID != "f1" || attached[0].LogEntryID != row.ID {
		t.Errorf("attached = %+v", attached)
	}
	if k.daCommit != 1 {
		t.Errorf("commit %d", k.daCommit)
	}
}

func TestNoteRefusesAForeignFileAndWritesNothing(t *testing.T) {
	for name, c := range map[string]domain.AttachCandidate{
		"another officer's file": petitionLogCandidate("f1", idPhieuThu, "CB-00999", domain.StoredFileStored, ""),
		"another petition's":     petitionLogCandidate("f1", "pa-other", maCanBoThu, domain.StoredFileStored, ""),
		"not finished":           petitionLogCandidate("f1", idPhieuThu, maCanBoThu, domain.StoredFilePending, ""),
		"already on an entry":    petitionLogCandidate("f1", idPhieuThu, maCanBoThu, domain.StoredFileStored, "nk-0"),
		"a verification photo": func() domain.AttachCandidate {
			c := petitionLogCandidate("f1", idPhieuThu, maCanBoThu, domain.StoredFileStored, "")
			c.File.Purpose = domain.PurposePetitionVerificationPhoto
			return c
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = phieuDaGiaoCho(maCanBoThu)
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			files := &staffFilesFake{cands: map[string]domain.AttachCandidate{"f1": c}}
			uc.staffFiles = files
			_, _, err := uc.GhiChuNoiBo(ctx, maPhieuThu, ghiChuThu, []string{"f1"}, canBoThu(),
				khongQuyenGhiChu, khongQuyenHanChe)
			if !errors.Is(err, domain.ErrPetitionAttachmentNotUsable) {
				t.Fatalf("err = %v", err)
			}
			if k.coCau("INSERT INTO nhat_ky_phan_anh") || files.linkCalled != 0 || k.daCommit != 0 {
				t.Error("written although a file was refused")
			}
		})
	}
}

// --- verification photos: the harness ------------------------------------------------------------------

const vpCode = ppCode

type vpPetitions struct {
	byCommune  map[tenant.ID]map[string]domain.PhieuPhanAnh
	statusAtLk *domain.TrangThai
	locked     int

	// reopenedAt is the timeline's latest `mo-lai-theo-danh-gia` instant per commune and petition id;
	// absent = no such row (the zero time, as store.LatestReopenAtTx answers NULL).
	reopenedAt  map[tenant.ID]map[string]time.Time
	reopenReads int

	// logged are the timeline rows written through GhiNhatKy (the removal's line) — petition_log_attachment_remove_test.go.
	logged []domain.NhatKyPhanAnh
}

func (p *vpPetitions) LatestReopenAtTx(_ context.Context, tx *pkgstore.ScopedTx, petitionID string) (time.Time, error) {
	p.reopenReads++
	return p.reopenedAt[tx.TenantID()][petitionID], nil
}

func (p *vpPetitions) TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) { // vi-name-ok: implements the existing store method
	pa, ok := p.byCommune[tenant.MustFrom(ctx)][ma]
	if !ok {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

func (p *vpPetitions) TheoMaTraCuuDeSua(_ context.Context, tx *pkgstore.ScopedTx, ma string) (domain.PhieuPhanAnh, error) { // vi-name-ok: implements the existing store method
	p.locked++
	pa, ok := p.byCommune[tx.TenantID()][ma]
	if !ok {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	if p.statusAtLk != nil {
		pa.TrangThai = *p.statusAtLk
	}
	return pa, nil
}

func (p *vpPetitions) CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (domain.PhieuPhanAnh, error) { // vi-name-ok: implements the existing store method
	pa, ok := p.byCommune[tenant.MustFrom(ctx)][ma]
	if !ok || congDanID == "" || pa.CongDanID != congDanID {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

func (p *vpPetitions) CitizenPetitionForUpdate(context.Context, *pkgstore.ScopedTx, string, string) (domain.PhieuPhanAnh, error) {
	return domain.PhieuPhanAnh{}, errors.New("not on this path")
}

// vpFiles is ppFiles with a PURPOSE-aware count, the verification read and the petition link read.
type vpFiles struct {
	*ppFiles
	links map[string]string
	// softDeleted records "id|by|reason" per SoftDelete — petition_log_attachment_remove_test.go.
	softDeleted []string
}

func (f *vpFiles) countPurpose(commune tenant.ID, subjectID, purpose string, pendingSince time.Time) int {
	n := 0
	for _, sf := range f.all(commune) {
		if sf.SubjectID != subjectID || sf.Purpose != purpose {
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

func (f *vpFiles) CountForSubjectTx(_ context.Context, tx *pkgstore.ScopedTx, _, subjectID, purpose string,
	pendingSince time.Time) (int, error) {
	return f.countPurpose(tx.TenantID(), subjectID, purpose, pendingSince), nil
}

func (f *vpFiles) CountForSubject(ctx context.Context, _, subjectID, purpose string, pendingSince time.Time) (int, error) {
	return f.countPurpose(tenant.MustFrom(ctx), subjectID, purpose, pendingSince), nil
}

// countAfter is countPurpose restricted to slots issued STRICTLY AFTER `after` — the store's
// `created_at > $n` cut.
func (f *vpFiles) countAfter(commune tenant.ID, subjectID, purpose string, after, pendingSince time.Time) int {
	n := 0
	for _, sf := range f.all(commune) {
		if sf.SubjectID != subjectID || sf.Purpose != purpose || !sf.CreatedAt.After(after) {
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

func (f *vpFiles) CountStoredCreatedAfterTx(_ context.Context, tx *pkgstore.ScopedTx, _, subjectID, purpose string,
	after time.Time) (int, error) {
	if after.IsZero() {
		return 0, errors.New("fake: a zero cut")
	}
	return f.countAfter(tx.TenantID(), subjectID, purpose, after, time.Time{}), nil
}

func (f *vpFiles) CountLiveCreatedAfterTx(_ context.Context, tx *pkgstore.ScopedTx, _, subjectID, purpose string,
	after, pendingSince time.Time) (int, error) {
	if after.IsZero() || pendingSince.IsZero() {
		return 0, errors.New("fake: a zero bound")
	}
	return f.countAfter(tx.TenantID(), subjectID, purpose, after, pendingSince), nil
}

// VerificationPhotos returns EVERY petition file the store's purpose filter would NOT drop — and a log
// attachment besides, so the use case's own shape wall is what keeps it out.
func (f *vpFiles) VerificationPhotos(ctx context.Context, petitionID string) ([]domain.StoredFile, error) {
	out := []domain.StoredFile{}
	for _, sf := range f.all(tenant.MustFrom(ctx)) {
		if sf.SubjectID == petitionID && sf.Status.Attachable() &&
			(sf.Purpose == domain.PurposePetitionVerificationPhoto || sf.Purpose == domain.PurposePetitionLogAttachment) {
			out = append(out, sf)
		}
	}
	return out, nil
}

func (f *vpFiles) LinkedPetitionLogEntry(_ context.Context, id string) (string, error) {
	return f.links[id], nil
}

// vpObjects accepts PutServerProduced for exactly core/storage's flow (c) key — records, petitions,
// verification purpose, variant original — and REFUSES to purge a records object outside temp.
type vpObjects struct {
	*objectStoreFake
	produced []string
}

func (o *vpObjects) PutServerProduced(_ context.Context, dst storage.Key, r io.Reader, size int64) (storage.Produced, error) {
	if dst.Class != storage.ClassRecords || dst.Purpose != storage.PurposePetitionVerificationPhoto ||
		dst.Variant != storage.VariantOriginal {
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
	key, _ := dst.Path()
	if o.has(storage.BucketPrivate, key) {
		return storage.Produced{}, storage.ErrExists
	}
	o.put(storage.BucketPrivate, key, d)
	o.produced = append(o.produced, key)
	s := sha256.Sum256(d)
	return storage.Produced{Key: key, Size: size, ContentType: mime, SHA256: hex.EncodeToString(s[:])}, nil
}

func (o *vpObjects) PurgeAllVersions(ctx context.Context, b storage.Bucket, key string) error {
	if b != storage.BucketTemp && strings.HasPrefix(key, "records/") {
		return storage.ErrRecordsNotPurgeable
	}
	return o.objectStoreFake.PurgeAllVersions(ctx, b, key)
}

type purposePolicies map[storage.Purpose]uploadpolicy.Policy

func (p purposePolicies) Policy(_ context.Context, purpose storage.Purpose) (uploadpolicy.Policy, bool, error) {
	pol, ok := p[purpose]
	return pol, ok, nil
}

func staffFilePolicies() purposePolicies {
	return purposePolicies{
		storage.PurposePetitionVerificationPhoto: {Purpose: storage.PurposePetitionVerificationPhoto, MaxBytes: 10 << 20,
			AllowedMIMETypes: []string{storage.MIMEJPEG, storage.MIMEPNG, storage.MIMEWebP},
			FileCountLimited: true, MaxFilesPerSubject: 5},
		storage.PurposePetitionLogAttachment: {Purpose: storage.PurposePetitionLogAttachment, MaxBytes: 50 << 20,
			AllowedMIMETypes: []string{"application/pdf", storage.MIMEJPEG, storage.MIMEPNG},
			FileCountLimited: true, MaxFilesPerSubject: 20},
	}
}

type vpHarness struct {
	uc      *StaffVerificationPhotos
	citizen *CitizenPetitionPhotos
	db      *ppDB
	store   *pkgstore.DB
	pets    *vpPetitions
	files   *vpFiles
	objects *vpObjects
	scanner *scannerFake
	ctx     context.Context
}

func vpPetitionRow(status domain.TrangThai, field string) domain.PhieuPhanAnh {
	return domain.PhieuPhanAnh{ID: ppPetition, MaTraCuu: vpCode, CongDanID: ppCitizen, TrangThai: status,
		LinhVuc: field, Kenh: domain.KenhZaloMiniApp}
}

func buildVerification(t *testing.T) *vpHarness {
	t.Helper()
	d := &ppDB{}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := &vpHarness{
		db: d,
		pets: &vpPetitions{byCommune: map[tenant.ID]map[string]domain.PhieuPhanAnh{
			xaThu: {vpCode: vpPetitionRow(domain.DangXuLy, "rac-thai"),
				"PA-CANBO-0000-0001": {ID: "pa-canbo", MaTraCuu: "PA-CANBO-0000-0001", LinhVuc: domain.LinhVucHanChe,
					TrangThai: domain.DangXuLy}},
		}},
		files:   &vpFiles{ppFiles: &ppFiles{rows: map[tenant.ID]map[string]domain.StoredFile{}}, links: map[string]string{}},
		objects: &vpObjects{objectStoreFake: newObjectStoreFake()},
		scanner: &scannerFake{res: malwarescan.Result{Clean: true}},
		ctx:     ctxXa(xaThu),
	}
	h.store = pkgstore.New(db)
	h.citizen = NewCitizenPetitionPhotos(h.store, nil, nil, h.objects, h.scanner, staffFilePolicies())
	h.uc = NewStaffVerificationPhotos(h.store, h.pets, h.files, h.citizen)
	h.uc.newID = func() (string, error) { return ppFileID, nil }
	h.uc.now = func() time.Time { return ppNow }
	return h
}

func vpDestKey() string {
	return "records/t_" + strings.ToLower(string(xaThu)) + "/2026/10/petitions/petition-verification-photo/" +
		strings.ToLower(ppFileID) + "/original.jpg"
}

func vpUploadKey(ext string) string { return "upload/" + strings.TrimSuffix(vpDestKey(), "jpg") + ext }

func (h *vpHarness) seedPending(uploader string, ext string, data []byte, created time.Time) {
	h.files.all(xaThu)[ppFileID] = domain.StoredFile{
		ID: ppFileID, Bucket: domain.StoredFileBucketPrivate, ObjectKey: vpDestKey(),
		RetentionClass: string(storage.ClassRecords), Purpose: domain.PurposePetitionVerificationPhoto,
		SubjectType: domain.StoredFileSubjectPetition, SubjectID: ppPetition, OriginalName: domain.VerificationPhotoName,
		Status: domain.StoredFilePending, UploadedBy: uploader, CreatedAt: created, UpdatedAt: created,
	}
	if data != nil {
		h.objects.put(storage.BucketTemp, vpUploadKey(ext), data)
	}
}

// --- verification photos: upload -------------------------------------------------------------------------

func TestVerificationPhoto_ReserveWritesARecordsRowByBusinessCode(t *testing.T) {
	h := buildVerification(t)
	up, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEPNG, Size: 812_000},
		canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	f := up.file
	if f.RetentionClass != string(storage.ClassRecords) || f.Bucket != domain.StoredFileBucketPrivate ||
		f.Purpose != domain.PurposePetitionVerificationPhoto || f.UploadedBy != maCanBoThu ||
		f.OriginalName != domain.VerificationPhotoName || f.ObjectKey != vpDestKey() {
		t.Errorf("pending row = %+v", f)
	}
	if up.uploadKey != vpUploadKey("png") || up.maxBytes != 10<<20 || up.subject != vpCode {
		t.Errorf("reservation = key %q, cap %d, subject %q — want the declared type's temp key", up.uploadKey,
			up.maxBytes, up.subject)
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != maCanBoThu || a[0][2] != ActionVerificationPhotoRequested || a[0][3] != vpCode {
		t.Errorf("trail = %v", a)
	}
	if h.pets.locked != 1 {
		t.Error("petition not read under the lock")
	}
}

// One request: reserve, stream, re-encode, store — the trail is the request and the stored entries.
func TestVerificationPhoto_UploadStoresTheReEncodeInOneRequest(t *testing.T) {
	h := buildVerification(t)
	raw := ppJPEG(t, 64, 32, 6)
	body := uploadBodyOf(raw)
	f, err := h.uc.Upload(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: int64(len(raw))},
		body, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if f.Status != domain.StoredFileStored || f.ObjectKey != vpDestKey() || f.UploadedBy != maCanBoThu {
		t.Errorf("stored = %+v", f)
	}
	if len(h.objects.uploads) != 1 || h.objects.uploads[0].key != vpUploadKey("jpg") || body.finished != 1 {
		t.Errorf("uploads = %+v, finish = %d", h.objects.uploads, body.finished)
	}
	stored, err := h.objects.get(storage.BucketPrivate, vpDestKey(), "")
	if err != nil || bytes.Contains(stored, []byte("GPS-16.05N")) {
		t.Errorf("clean copy: %v", err)
	}
	a := h.db.audits()
	if len(a) != 2 || a[0][2] != ActionVerificationPhotoRequested || a[1][2] != ActionVerificationPhotoStored ||
		a[1][0] != maCanBoThu {
		t.Errorf("trail = %v", a)
	}
}

// A stream that fails closes the row with the staff verb and the officer's business code.
func TestVerificationPhoto_UploadStreamFailureClosesTheRow(t *testing.T) {
	h := buildVerification(t)
	h.objects.uploadErr = errors.New("minio: connection reset")
	raw := ppJPEG(t, 16, 16, 1)
	if _, err := h.uc.Upload(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: int64(len(raw))},
		uploadBodyOf(raw), canBoThu(), khongQuyenHanChe); err == nil {
		t.Fatal("no error")
	}
	if got := h.files.all(xaThu)[ppFileID].Status; got != domain.StoredFileFailed {
		t.Errorf("row = %s, want failed", got)
	}
	a := h.db.audits()
	if len(a) != 2 || a[1][2] != ActionVerificationPhotoExpired || a[1][0] != maCanBoThu || a[1][3] != vpCode {
		t.Errorf("trail = %v", a)
	}
}

func TestVerificationPhoto_RequestRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		setup func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor)
		want  error
	}{
		"closed petition": {func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor) {
			h.pets.byCommune[xaThu][vpCode] = vpPetitionRow(domain.DaDong, "rac-thai")
			return vpCode, khongQuyenHanChe, canBoThu()
		}, ErrVerificationPhotoWindowClosed},
		"refused petition": {func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor) {
			h.pets.byCommune[xaThu][vpCode] = vpPetitionRow(domain.KhongTiepNhan, "rac-thai")
			return vpCode, khongQuyenHanChe, canBoThu()
		}, ErrVerificationPhotoWindowClosed},
		"full, live pending slots included": {func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor) {
			for i, s := range []domain.StoredFileStatus{"stored", "stored", "ready", "stored", "pending"} {
				id := "v" + string(rune('a'+i))
				h.files.all(xaThu)[id] = domain.StoredFile{ID: id, SubjectID: ppPetition,
					Purpose: domain.PurposePetitionVerificationPhoto, Status: s, CreatedAt: ppNow}
			}
			return vpCode, khongQuyenHanChe, canBoThu()
		}, ErrVerificationPhotoCountReached},
		"restricted without the key": {func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor) {
			return "PA-CANBO-0000-0001", khongQuyenHanChe, canBoThu()
		}, ErrPhieuHanChe},
		"another commune's code": {func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor) {
			h.ctx = ctxXa(xaKia)
			return vpCode, khongQuyenHanChe, canBoThu()
		}, petstore.ErrPhieuKhongTonTai},
		"a citizen principal": {func(h *vpHarness) (string, QuyenXemHanChe, audit.Actor) {
			return vpCode, khongQuyenHanChe, ppCitizenActor()
		}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildVerification(t)
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
}

// --- verification photos: the cap counts PER ROUND (ADR 0047 (e)) ------------------------------------------

// vpReopen marks the harness petition reopened once, at `at`, with the timeline row to match.
func (h *vpHarness) vpReopen(at time.Time) {
	p := h.pets.byCommune[xaThu][vpCode]
	p.SoLanMoLai = 1
	h.pets.byCommune[xaThu][vpCode] = p
	h.pets.reopenedAt = map[tenant.ID]map[string]time.Time{xaThu: {ppPetition: at}}
}

// vpSeedPhotos puts n verification photos of `status` on the petition, their slots issued at `created`.
func (h *vpHarness) vpSeedPhotos(prefix string, n int, status domain.StoredFileStatus, created time.Time) {
	for i := 0; i < n; i++ {
		id := prefix + string(rune('a'+i))
		h.files.all(xaThu)[id] = domain.StoredFile{ID: id, SubjectType: domain.StoredFileSubjectPetition,
			SubjectID: ppPetition, Purpose: domain.PurposePetitionVerificationPhoto, Status: status,
			UploadedBy: maCanBoThu, CreatedAt: created, UpdatedAt: created}
	}
}

func TestVerificationPhoto_ReopenedWithFiveOldPhotosAcceptsANewSlot(t *testing.T) {
	h := buildVerification(t)
	reopenedAt := ppNow.Add(-2 * time.Hour)
	h.vpSeedPhotos("old", 5, domain.StoredFileStored, reopenedAt.Add(-time.Hour))
	h.vpReopen(reopenedAt)
	if _, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
		canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("five photos of the rejected round blocked the new round's first photo: %v", err)
	}
	if h.pets.reopenReads != 1 {
		t.Errorf("round start read %d times, want once, inside the transaction", h.pets.reopenReads)
	}
}

func TestVerificationPhoto_FiveInTheCurrentRoundRefuse(t *testing.T) {
	reopenedAt := ppNow.Add(-2 * time.Hour)
	for name, seed := range map[string]func(h *vpHarness){
		"five stored": func(h *vpHarness) {
			h.vpSeedPhotos("new", 5, domain.StoredFileStored, reopenedAt.Add(time.Minute))
		},
		// An unexpired form of THIS round is a place already promised, exactly as before the change.
		"four stored, one live pending slot": func(h *vpHarness) {
			h.vpSeedPhotos("new", 4, domain.StoredFileStored, reopenedAt.Add(time.Minute))
			h.vpSeedPhotos("pnd", 1, domain.StoredFilePending, ppNow.Add(-time.Minute))
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildVerification(t)
			h.vpSeedPhotos("old", 5, domain.StoredFileStored, reopenedAt.Add(-time.Hour))
			seed(h)
			h.vpReopen(reopenedAt)
			_, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
				canBoThu(), khongQuyenHanChe)
			if !errors.Is(err, ErrVerificationPhotoCountReached) {
				t.Fatalf("err = %v, want ErrVerificationPhotoCountReached (409 photo_limit)", err)
			}
			if len(h.files.inserted) != 0 || len(h.db.audits()) != 0 {
				t.Error("a refused request wrote a row or a trail")
			}
		})
	}
}

// An EXPIRED pending slot of the current round holds no place — the pending window still applies.
func TestVerificationPhoto_ExpiredSlotOfTheRoundDoesNotCount(t *testing.T) {
	h := buildVerification(t)
	reopenedAt := ppNow.Add(-2 * time.Hour)
	h.vpSeedPhotos("new", 4, domain.StoredFileStored, reopenedAt.Add(time.Minute))
	h.vpSeedPhotos("exp", 1, domain.StoredFilePending, reopenedAt.Add(2*time.Minute)) // long expired
	h.vpReopen(reopenedAt)
	if _, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
		canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("an abandoned form held a place: %v", err)
	}
}

func TestVerificationPhoto_ReopenedWithoutTimelineRowRefuses(t *testing.T) {
	h := buildVerification(t)
	p := h.pets.byCommune[xaThu][vpCode]
	p.SoLanMoLai = 1 // the counter says reopened; the timeline has no reopen row
	h.pets.byCommune[xaThu][vpCode] = p
	_, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
		canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, errReopenInstantMissing) {
		t.Fatalf("err = %v, want errReopenInstantMissing — never a guess at the round", err)
	}
	if len(h.files.inserted) != 0 {
		t.Error("a slot was issued without knowing the round")
	}
}

func TestVerificationPhoto_CompletionRecheckCountsPerRound(t *testing.T) {
	reopenedAt := ppNow.Add(-2 * time.Hour)
	for name, c := range map[string]struct {
		currentRound int
		wantStored   bool
	}{
		"five old, none this round: stored":  {0, true},
		"five old, four this round: stored":  {4, true},
		"five old, five this round: refused": {5, false},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildVerification(t)
			h.vpSeedPhotos("old", 5, domain.StoredFileStored, reopenedAt.Add(-time.Hour))
			h.vpSeedPhotos("new", c.currentRound, domain.StoredFileStored, reopenedAt.Add(time.Minute))
			h.vpReopen(reopenedAt)
			h.seedPending(maCanBoThu, "jpg", ppJPEG(t, 16, 16, 1), ppNow)
			f, err := h.uc.complete(h.ctx, vpCode, ppFileID, canBoThu(), khongQuyenHanChe)
			if c.wantStored {
				if err != nil || f.Status != domain.StoredFileStored {
					t.Fatalf("Complete = %+v, %v — want stored", f, err)
				}
				return
			}
			var rej *AttachmentRejection
			if !errors.As(err, &rej) || rej.Reason != RejectCountReached {
				t.Fatalf("err = %v, want the count-reached rejection", err)
			}
			if h.files.all(xaThu)[ppFileID].Status != domain.StoredFileRejected {
				t.Errorf("row = %s, want rejected", h.files.all(xaThu)[ppFileID].Status)
			}
		})
	}
}

// Never reopened: unchanged — every stored photo counts, the timeline is not read.
func TestVerificationPhoto_NeverReopenedCountsEveryPhoto(t *testing.T) {
	h := buildVerification(t)
	h.vpSeedPhotos("old", 5, domain.StoredFileStored, ppNow.Add(-48*time.Hour))
	_, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
		canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, ErrVerificationPhotoCountReached) {
		t.Fatalf("err = %v, want ErrVerificationPhotoCountReached", err)
	}
	if h.pets.reopenReads != 0 {
		t.Error("the timeline was read for a petition never reopened")
	}
}

// Citizen photos on the same petition never consume the staff count — and the other way round.
func TestVerificationPhoto_CountIsItsOwnPurpose(t *testing.T) {
	h := buildVerification(t)
	for i := 0; i < 5; i++ {
		id := "c" + string(rune('a'+i))
		h.files.all(xaThu)[id] = domain.StoredFile{ID: id, SubjectID: ppPetition, Purpose: domain.PurposePetitionPhoto,
			Status: domain.StoredFileStored, UploadedBy: domain.CitizenLogActor}
	}
	if _, err := h.uc.reserve(h.ctx, vpCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 1000},
		canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("five citizen photos blocked a staff photo: %v", err)
	}
}

// --- verification photos: completion -----------------------------------------------------------------------

func TestVerificationPhoto_CompleteReencodesWithoutEXIFAndStoresOnlyTheCleanCopy(t *testing.T) {
	h := buildVerification(t)
	raw := ppJPEG(t, 64, 32, 6)
	h.seedPending(maCanBoThu, "jpg", raw, ppNow)
	f, err := h.uc.complete(h.ctx, vpCode, ppFileID, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if f.Status != domain.StoredFileStored || f.MIMEType != storage.MIMEJPEG {
		t.Errorf("stored = %+v", f)
	}
	stored, err := h.objects.get(storage.BucketPrivate, vpDestKey(), "")
	if err != nil {
		t.Fatalf("no clean copy at the records key: %v", err)
	}
	if bytes.Equal(stored, raw) || bytes.Contains(stored, []byte("GPS-16.05N")) || bytes.Contains(stored, []byte("Exif\x00\x00")) {
		t.Error("the stored bytes carry the upload's EXIF — the raw upload reached the private bucket")
	}
	if h.objects.has(storage.BucketTemp, vpUploadKey("jpg")) {
		t.Error("the raw upload was not purged from temp")
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != maCanBoThu || a[0][2] != ActionVerificationPhotoStored ||
		!strings.Contains(a[0][4], `"ma_hoa_lai_bo_exif":true`) {
		t.Errorf("trail = %v", a)
	}
}

func TestVerificationPhoto_AnotherOfficersUploadIs404(t *testing.T) {
	h := buildVerification(t)
	h.seedPending("CB-00999", "jpg", ppJPEG(t, 8, 8, 1), ppNow)
	if _, err := h.uc.complete(h.ctx, vpCode, ppFileID, canBoThu(), khongQuyenHanChe); !errors.Is(err, ErrVerificationPhotoNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(h.objects.produced) != 0 {
		t.Error("bytes processed for somebody else's upload")
	}
}

func TestVerificationPhoto_LateRefusalUnderTheLockKeepsTheRecordsCopyAndSaysSo(t *testing.T) {
	h := buildVerification(t)
	h.seedPending(maCanBoThu, "jpg", ppJPEG(t, 16, 16, 1), ppNow)
	closed := domain.DaDong
	h.pets.statusAtLk = &closed // closed by another officer while the image was processed
	_, err := h.uc.complete(h.ctx, vpCode, ppFileID, canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, ErrVerificationPhotoWindowClosed) {
		t.Fatalf("err = %v", err)
	}
	if h.files.all(xaThu)[ppFileID].Status != domain.StoredFileRejected {
		t.Errorf("row = %s, want rejected", h.files.all(xaThu)[ppFileID].Status)
	}
	if !h.objects.has(storage.BucketPrivate, vpDestKey()) {
		t.Error("a records object was purged from the private bucket")
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][2] != ActionVerificationPhotoRejected || !strings.Contains(a[0][4], `"ban_sach_con_trong_kho":true`) {
		t.Errorf("trail = %v", a)
	}
}

func TestVerificationPhoto_StoredAnswersAsStored(t *testing.T) {
	h := buildVerification(t)
	h.seedPending(maCanBoThu, "jpg", nil, ppNow)
	row := h.files.all(xaThu)[ppFileID]
	row.Status = domain.StoredFileStored
	h.files.all(xaThu)[ppFileID] = row
	f, err := h.uc.complete(h.ctx, vpCode, ppFileID, canBoThu(), khongQuyenHanChe)
	if err != nil || f.Status != domain.StoredFileStored || len(h.db.audits()) != 0 {
		t.Fatalf("second completion: %+v %v (%d audits)", f, err, len(h.db.audits()))
	}
}

func TestVerificationPhoto_SharesTheCitizenFlowsSlots(t *testing.T) {
	h := buildVerification(t)
	if h.uc.photoSlots != h.citizen.photoSlots || h.uc.photoSlots == nil {
		t.Fatal("the staff flow has its own decode/read slots — two decode budgets against one pod")
	}
}

// --- verification photos: the two lists ----------------------------------------------------------------------

func (h *vpHarness) addFile(id, purpose string, status domain.StoredFileStatus) {
	h.files.all(xaThu)[id] = domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate,
		ObjectKey: "records/k/" + id + "/original.jpg", Purpose: purpose, SubjectType: domain.StoredFileSubjectPetition,
		SubjectID: ppPetition, Status: status, UploadedBy: maCanBoThu, OriginalName: "x.jpg"}
}

func TestVerificationPhoto_StaffListIsAuditedByBusinessCode(t *testing.T) {
	h := buildVerification(t)
	h.addFile("v1", domain.PurposePetitionVerificationPhoto, domain.StoredFileStored)
	h.addFile("l1", domain.PurposePetitionLogAttachment, domain.StoredFileStored)
	links, err := h.uc.ListPhotos(h.ctx, vpCode, false, canBoThu())
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(links) != 1 || links[0].File.ID != "v1" {
		t.Errorf("links = %+v — a log attachment must never be signed here", links)
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != maCanBoThu || a[0][1] != "staff" || a[0][2] != ActionVerificationPhotosViewed {
		t.Errorf("trail = %v", a)
	}
	if _, err := h.uc.ListPhotos(h.ctx, vpCode, false, audit.Actor{Kind: "staff"}); err == nil {
		t.Error("a reader with no business code was handed links")
	}
	if _, err := h.uc.ListPhotos(h.ctx, "PA-CANBO-0000-0001", false, canBoThu()); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Errorf("restricted without the key: %v", err)
	}
}

func TestVerificationPhoto_CitizenListOwnPetitionOnlyNeverALogAttachment(t *testing.T) {
	h := buildVerification(t)
	// At a status where the citizen sees the photos at all (owner decision (b)) — the visibility rule
	// has its own test below; this one is about whose photos and which purpose.
	h.pets.byCommune[xaThu][vpCode] = vpPetitionRow(domain.ChoDanXacNhan, "rac-thai")
	h.addFile("v1", domain.PurposePetitionVerificationPhoto, domain.StoredFileStored)
	h.addFile("v2", domain.PurposePetitionVerificationPhoto, domain.StoredFilePending)
	h.addFile("l1", domain.PurposePetitionLogAttachment, domain.StoredFileStored)
	uc := NewCitizenVerificationPhotos(h.pets, h.files, h.objects)
	links, err := uc.ListPhotos(h.ctx, vpCode, ppCitizenActor())
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(links) != 1 || links[0].File.ID != "v1" {
		t.Errorf("links = %+v — only the stored verification photo", links)
	}
	for name, actor := range map[string]audit.Actor{
		"another citizen": {ID: ppOther, Kind: "citizen"},
		"a staff actor":   canBoThu(),
	} {
		if _, err := uc.ListPhotos(h.ctx, vpCode, actor); err == nil {
			t.Errorf("%s was handed the photos", name)
		}
	}
	if _, err := uc.ListPhotos(ctxXa(xaKia), vpCode, ppCitizenActor()); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Errorf("another commune: %v", err)
	}
	if len(h.db.audits()) != 0 {
		t.Error("the citizen's own read was audited (it is not, as the scene-photo list is not)")
	}
}

// --- log attachments --------------------------------------------------------------------------------------------

type laHarness struct {
	uc      *PetitionLogAttachments
	db      *ppDB
	pets    *vpPetitions
	files   *vpFiles
	objects *objectStoreFake
	ctx     context.Context
}

func buildLogAttachments(t *testing.T) *laHarness {
	t.Helper()
	d := &ppDB{}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	assigned := vpPetitionRow(domain.DaDong, "rac-thai") // a CLOSED petition: attachments ride notes, every status
	assigned.CanBoXuLyID = maCanBoThu
	h := &laHarness{
		db:      d,
		pets:    &vpPetitions{byCommune: map[tenant.ID]map[string]domain.PhieuPhanAnh{xaThu: {vpCode: assigned}}},
		files:   &vpFiles{ppFiles: &ppFiles{rows: map[tenant.ID]map[string]domain.StoredFile{}}, links: map[string]string{}},
		objects: newObjectStoreFake(),
		ctx:     ctxXa(xaThu),
	}
	h.uc = NewPetitionLogAttachments(pkgstore.New(db), h.pets, h.files, h.objects,
		&scannerFake{res: malwarescan.Result{Clean: true}}, staffFilePolicies())
	h.uc.newID = func() (string, error) { return ppFileID, nil }
	h.uc.now = func() time.Time { return ppNow }
	return h
}

func laDestKey() string {
	return "records/t_" + strings.ToLower(string(xaThu)) + "/2026/10/petitions/petition-log-attachment/" +
		strings.ToLower(ppFileID) + "/original.pdf"
}

func TestPetitionLogAttachment_UploadThenDownload(t *testing.T) {
	h := buildLogAttachments(t)
	body := uploadBodyOf(pdfBytes)
	f, err := h.uc.Upload(h.ctx, vpCode, AttachmentUploadRequest{FileName: "bien-ban.pdf",
		ContentType: "application/pdf", Size: int64(len(pdfBytes))}, body, canBoThu(), khongQuyenGhiChu,
		khongQuyenHanChe)
	if err != nil || f.Status != domain.StoredFileStored {
		t.Fatalf("Upload: %+v %v", f, err)
	}
	if f.Purpose != domain.PurposePetitionLogAttachment || f.UploadedBy != maCanBoThu || f.OriginalName != "bien-ban.pdf" ||
		f.RetentionClass != string(storage.ClassRecords) || f.ObjectKey != laDestKey() {
		t.Errorf("stored row = %+v", f)
	}
	if len(h.objects.uploads) != 1 || h.objects.uploads[0].key != "upload/"+laDestKey() ||
		h.objects.uploads[0].maxBytes != 50<<20 || body.finished != 1 {
		t.Errorf("uploads = %+v, finish = %d", h.objects.uploads, body.finished)
	}
	if got, _ := h.objects.get(storage.BucketPrivate, laDestKey(), ""); !bytes.Equal(got, pdfBytes) {
		t.Error("the record was not promoted as uploaded")
	}
	h.files.links[ppFileID] = "nkpa-1"
	d, err := h.uc.DownloadLink(h.ctx, vpCode, ppFileID, audit.Actor{ID: "CB-00777", Kind: "staff"}, khongQuyenHanChe)
	if err != nil || d.URL == "" {
		t.Fatalf("DownloadLink: %v", err)
	}
	actions := []string{}
	for _, a := range h.db.audits() {
		actions = append(actions, a[2])
	}
	want := []string{ActionPetitionLogAttachmentRequested, ActionPetitionLogAttachmentStored, ActionPetitionLogAttachmentDownloaded}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Errorf("trail = %v, want %v", actions, want)
	}
	if last := h.db.audits()[2]; last[0] != "CB-00777" || strings.Contains(last[4], "bien-ban") {
		t.Errorf("download trail = %v — the reader's code, never the file name", last)
	}
}

func TestPetitionLogAttachment_TheNoteRuleDecides(t *testing.T) {
	h := buildLogAttachments(t)
	other := audit.Actor{ID: "CB-00999", Kind: "staff"}
	req := AttachmentUploadRequest{FileName: "a.pdf", ContentType: "application/pdf", Size: 10}
	body := uploadBodyOf(make([]byte, 10))
	if _, err := h.uc.Upload(h.ctx, vpCode, req, body, other, khongQuyenGhiChu, khongQuyenHanChe); !errors.Is(err, ErrKhongPhaiNguoiDuocGiao) {
		t.Fatalf("not the assignee, no commune-wide key: %v", err)
	}
	if body.reads != 0 || len(h.objects.uploads) != 0 {
		t.Error("a refused upload read the body or wrote to the store")
	}
	if _, err := h.uc.reserve(h.ctx, vpCode, req, other, coQuyenGhiChu, khongQuyenHanChe); err != nil {
		t.Fatalf("commune-wide key: %v", err)
	}
}

func TestPetitionLogAttachment_DraftIsTheUploadersOnly(t *testing.T) {
	h := buildLogAttachments(t)
	h.files.all(xaThu)[ppFileID] = domain.StoredFile{ID: ppFileID, Bucket: domain.StoredFileBucketPrivate,
		ObjectKey: laDestKey(), Purpose: domain.PurposePetitionLogAttachment, SubjectType: domain.StoredFileSubjectPetition,
		SubjectID: ppPetition, Status: domain.StoredFileStored, UploadedBy: maCanBoThu, OriginalName: "a.pdf"}
	if _, err := h.uc.DownloadLink(h.ctx, vpCode, ppFileID, audit.Actor{ID: "CB-00777", Kind: "staff"}, khongQuyenHanChe); !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("an unattached draft to another officer: %v", err)
	}
	if _, err := h.uc.DownloadLink(h.ctx, vpCode, ppFileID, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("its uploader: %v", err)
	}
}

// --- pins --------------------------------------------------------------------------------------------------------

func TestStaffFilePurposesMatchCoreStorage(t *testing.T) {
	if domain.PurposePetitionVerificationPhoto != string(storage.PurposePetitionVerificationPhoto) ||
		domain.PurposePetitionLogAttachment != string(storage.PurposePetitionLogAttachment) {
		t.Fatal("domain purposes drifted from core/storage")
	}
}

func TestVerificationPhotosVisibleToCitizen(t *testing.T) {
	for _, s := range []domain.TrangThai{domain.DaTiepNhan, domain.DangPhanLoai, domain.DaChuyenXuLy, domain.DangXuLy,
		domain.DaXuLy, domain.ChoDanXacNhan, domain.DaDong, domain.KhongTiepNhan, domain.ChuyenCapTren, "not-a-status"} {
		want := s == domain.ChoDanXacNhan || s == domain.DaDong
		if got := domain.VerificationPhotosVisibleToCitizen(s); got != want {
			t.Errorf("%s: visible = %v, want %v", s, got, want)
		}
	}
}

func TestVerificationPhotoUploadOpen(t *testing.T) {
	for _, s := range []domain.TrangThai{domain.DaDong, domain.KhongTiepNhan, domain.ChuyenCapTren} {
		if domain.VerificationPhotoUploadOpen(s) {
			t.Errorf("%s is an ending and must be closed", s)
		}
	}
	for _, s := range []domain.TrangThai{domain.DaTiepNhan, domain.DangPhanLoai, domain.DaChuyenXuLy, domain.DangXuLy,
		domain.DaXuLy, domain.ChoDanXacNhan} {
		if !domain.VerificationPhotoUploadOpen(s) {
			t.Errorf("%s must be open", s)
		}
	}
}
