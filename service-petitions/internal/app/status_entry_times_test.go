package app

// Migration 0043's two status-entry times (ADR 0047 G4, ADR 0091 §5), over the REAL store on the fake
// driver of driver_gia_phieu_test.go.
//
//	PROVED HERE   every road into `da-chuyen-xu-ly` / `cho-dan-xac-nhan` — assignment, the forward advance,
//	              a merged petition following its main — writes the act's instant IN THE SAME UPDATE as
//	              the status, inside the transaction · a re-assignment that keeps the status writes NULL
//	              (COALESCE keeps the handed-over instant) · a RE-ENTRY after a reopening sends the NEW
//	              instant, so COALESCE(new, stored) overwrites (G4: "mở lại lấy giờ mới nhất") · no
//	              statement reads `nhat_ky_phan_anh` for it.
//	NOT PROVED    what PostgreSQL does with COALESCE or with 0043's CHECKs — the pg suites' half.

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// An entry recorded before a reopening: distinct from mocThaoTac, so "kept" and "overwritten" differ.
var earlierEntry = time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)

// statusUpdate returns the one UPDATE of the petition row that carries the two status-entry columns.
func statusUpdate(t *testing.T, k *khoPhieuXuLyGia) lenhPhieu {
	t.Helper()
	var hits []lenhPhieu
	for _, l := range k.cau("UPDATE phieu_phan_anh") {
		if strings.Contains(l.sql, "awaiting_confirmation_at = COALESCE(") {
			hits = append(hits, l)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("%d status UPDATEs carrying the entry times, want ONE", len(hits))
	}
	if !hits[0].trongGiaoDich {
		t.Error("the status and its entry time were written outside the transaction")
	}
	return hits[0]
}

func TestAssignmentIntoForwardedRecordsTheInstant(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k) // `dang-phan-loai`
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	after, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001"}, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("PhanCong: %v", err)
	}
	l := statusUpdate(t, k)
	if !strings.Contains(l.sql, "forwarded_for_processing_at = COALESCE($7, forwarded_for_processing_at)") {
		t.Errorf("statement = %s", l.sql)
	}
	// $5 = the new status, $7 forwarded, $8 awaiting (args are 0-based).
	if l.args[4] != string(domain.DaChuyenXuLy) || l.args[6] != mocThaoTac || l.args[7] != nil {
		t.Errorf("args = %v — want da-chuyen-xu-ly with the act's instant, awaiting untouched", l.args)
	}
	if !after.ForwardedForProcessingAt.Equal(mocThaoTac) {
		t.Errorf("response ForwardedForProcessingAt = %v", after.ForwardedForProcessingAt)
	}
	if k.coCau("FROM nhat_ky_phan_anh") {
		t.Error("the entry time was looked up in the internal logbook (G4 forbids it)")
	}
}

// Moving a petition between departments mid-processing keeps the status (docs/ui-ux/09 §8.5), so the
// instant it was first handed over must stay.
func TestReassignmentKeepsTheForwardedInstant(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DaChuyenXuLy), "linh_vuc": "rac-thai", "bo_phan_id": "bp-001",
		"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac, "forwarded_for_processing_at": earlierEntry,
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	after, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-002"}, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("PhanCong: %v", err)
	}
	if l := statusUpdate(t, k); l.args[6] != nil || l.args[7] != nil {
		t.Errorf("args = %v — a re-assignment is no entry; both times must be NULL (kept)", l.args)
	}
	if !after.ForwardedForProcessingAt.Equal(earlierEntry) {
		t.Errorf("response ForwardedForProcessingAt = %v, want the stored %v", after.ForwardedForProcessingAt, earlierEntry)
	}
}

// The forward advance into `cho-dan-xac-nhan`, on a petition that was there before and was reopened by
// the citizen's rating: the NEW instant goes into the statement, and COALESCE(new, stored) takes it.
func TestAdvanceIntoAwaitingConfirmationOverwritesOnReEntry(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DaXuLy), "linh_vuc": "rac-thai", "han_xu_ly_xong": mocXuLyXongThu,
		"phan_loai_luc": mocThaoTac, "xu_ly_xong_luc": mocThaoTac, "so_lan_mo_lai": int64(1),
		"forwarded_for_processing_at": earlierEntry, "awaiting_confirmation_at": earlierEntry,
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	after, err := uc.TienTrangThai(ctx, maPhieuThu, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("TienTrangThai: %v", err)
	}
	l := statusUpdate(t, k)
	if !strings.Contains(l.sql, "awaiting_confirmation_at = COALESCE($7, awaiting_confirmation_at)") {
		t.Errorf("statement = %s", l.sql)
	}
	// $3 the new status, $6 forwarded, $7 awaiting.
	if l.args[2] != string(domain.ChoDanXacNhan) || l.args[5] != nil || l.args[6] != mocThaoTac {
		t.Errorf("args = %v — want cho-dan-xac-nhan with the NEW instant, forwarded untouched", l.args)
	}
	if !after.AwaitingConfirmationAt.Equal(mocThaoTac) || !after.ForwardedForProcessingAt.Equal(earlierEntry) {
		t.Errorf("response times = %v / %v", after.AwaitingConfirmationAt, after.ForwardedForProcessingAt)
	}
}

// A step that enters neither status writes NULL to both.
func TestAdvanceIntoOtherStepsWritesNoEntryTime(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DaChuyenXuLy), "linh_vuc": "rac-thai", "bo_phan_id": "bp-001",
		"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac,
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.TienTrangThai(ctx, maPhieuThu, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("TienTrangThai: %v", err)
	}
	if l := statusUpdate(t, k); l.args[2] != string(domain.DangXuLy) || l.args[5] != nil || l.args[6] != nil {
		t.Errorf("args = %v", l.args)
	}
}

// A merged petition following its main into `cho-dan-xac-nhan` (ADR 0087 §2) is asked to confirm from
// the main's act — its OWN awaiting_confirmation_at, in its own statement.
func TestMergedPetitionFollowingRecordsItsAwaitingInstant(t *testing.T) {
	k := khoPhieuMau()
	k.hang = mainRow(domain.DaXuLy)
	k.children = []map[string]driver.Value{childRow(idChild, codeChild, string(domain.DangXuLy), nil)}
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.TienTrangThai(ctx, codeMain, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("TienTrangThai: %v", err)
	}
	follow := k.cau("SET trang_thai = 'cho-dan-xac-nhan'")
	if len(follow) != 1 || !strings.Contains(follow[0].sql, "awaiting_confirmation_at = COALESCE($6, awaiting_confirmation_at)") ||
		follow[0].args[5] != mocThaoTac {
		t.Fatalf("follow = %v — the merged petition's own awaiting instant is the main's act", follow)
	}
}
