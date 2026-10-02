package app

// SoanNoiDungMiniApp.Delete over the REAL store over the fake driver (noi_dung_mini_app_test.go): the
// soft delete, the unpublish and the trail in ONE transaction; refusals write nothing; the cover's
// public copy is withdrawn the way an unpublish withdraws it.
//
// WHAT THIS DOES NOT PROVE: what PostgreSQL does with the statement — the CHECK tying the three
// soft-delete columns together, the partial indexes the reads walk. That half is
// store/noi_dung_mini_app_pg_test.go's TestPgSoftDeleteLeavesEveryReadAndTheSyncSkipsIt (skipped
// without VIGOV_TEST_DSN).

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

func TestDeleteContentItemSoftDeletesUnpublishesAndAuditsInOneTransaction(t *testing.T) {
	row := dongDongBo() // a synced, published item: the case where resurrection would matter
	k := &khoNDGia{dongHienCo: row}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	if err := uc.Delete(ctx, row.ID, "  Bài đăng nhầm xã  ", nguoiSoanND()); err != nil {
		t.Fatalf("xoá lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở=%d chốt=%d huỷ=%d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}

	// The read is the LOCKED one, and only of live rows — a second delete finds nothing.
	locked := k.cau("FOR UPDATE")
	if len(locked) != 1 || !strings.Contains(locked[0].sql, "deleted_at IS NULL") {
		t.Fatalf("lượt đọc khoá dòng = %+v", locked)
	}

	upd := k.cau("UPDATE noi_dung_mini_app")
	if len(upd) != 1 {
		t.Fatalf("số câu UPDATE = %d, muốn 1", len(upd))
	}
	q, args := upd[0].sql, upd[0].args
	for _, want := range []string{"deleted_at = $5", "deleted_by = $3", "delete_reason = $4", "trang_thai = 'an'",
		"AND deleted_at IS NULL"} {
		if !strings.Contains(q, want) {
			t.Errorf("câu xoá mềm thiếu %q: %s", want, q)
		}
	}
	// THE SYNC'S DEDUPLICATION KEY IS NOT TOUCHED: the row keeps `nguon_id_ngoai`, so the next portal
	// run sees it held by a deleted row and skips it (ExistingPortalItems → skipped_deleted).
	for _, banned := range []string{"nguon_id_ngoai", "nguon =", "cover_image_file_id", "audio_file_id"} {
		if strings.Contains(q, banned) {
			t.Errorf("câu xoá mềm chạm %q: %s", banned, q)
		}
	}
	if args[0] != string(xaA) || args[1] != row.ID || args[2] != maCanBoSoanND || args[3] != "Bài đăng nhầm xã" {
		t.Errorf("tham số = %v", args)
	}
	if k.coCau("DELETE FROM") {
		t.Fatal("xoá cứng")
	}

	delta := decodedAuditDelta(t, k, ActionDeleteContentItem)
	before, _ := delta["truoc"].(map[string]any)
	after, _ := delta["sau"].(map[string]any)
	if before["da_xoa"] != false || before["trang_thai"] != string(domain.TrangThaiDangHien) ||
		after["da_xoa"] != true || after["trang_thai"] != string(domain.TrangThaiAn) {
		t.Errorf("delta trước/sau = %v / %v", before, after)
	}
	// The reason's LENGTH, never its text (service-identity's staff delete convention).
	if delta["do_dai_ly_do"] != float64(len([]rune("Bài đăng nhầm xã"))) {
		t.Errorf("do_dai_ly_do = %v", delta["do_dai_ly_do"])
	}
	raw := string(k.cau("INSERT INTO audit_log")[0].args[7].([]byte))
	if strings.Contains(raw, "đăng nhầm") || strings.Contains(raw, row.TieuDe) {
		t.Errorf("vết mang văn bản tự do: %s", raw)
	}
}

func TestDeleteContentItemNeedsAReasonAndAStaffCodeBeforeAnyTransaction(t *testing.T) {
	for name, tc := range map[string]struct {
		reason string
		noCode bool
		want   error
	}{
		"blank reason":  {reason: "   ", want: domain.ErrThieuLyDoXoa},
		"long reason":   {reason: strings.Repeat("ệ", domain.LyDoXoaToiDa+1), want: domain.ErrLyDoXoaQuaDai},
		"no staff code": {reason: "Đăng nhầm", noCode: true, want: ErrThieuNguoiTaoNoiDung},
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoNDGia{dongHienCo: draftRow()}
			uc, _, ctx := dungUseCaseNoiDung(t, k)
			actor := nguoiSoanND()
			if tc.noCode {
				actor.ID = ""
			}
			if err := uc.Delete(ctx, draftRow().ID, tc.reason, actor); !errors.Is(err, tc.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.want)
			}
			if k.batDau != 0 || len(k.lenh) != 0 {
				t.Errorf("từ chối mà vẫn mở giao dịch / chạy câu lệnh: %d, %d", k.batDau, len(k.lenh))
			}
		})
	}
}

func TestDeleteContentItemMissingOrAlreadyDeletedIsNotFoundAndWritesNothing(t *testing.T) {
	// The locked read binds the commune and `deleted_at IS NULL`: another commune's id, an invented id
	// and an already-deleted item all find no row — one answer, nothing written.
	k := &khoNDGia{dongHienCo: nil}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	err := uc.Delete(ctx, "nd-da-xoa", "Xoá lần hai", nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungKhongTonTai", err)
	}
	if k.coCau("UPDATE noi_dung_mini_app") || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
		t.Error("không có dòng mà vẫn ghi")
	}
}

func TestDeleteContentItemAuditFailureRollsTheDeleteBack(t *testing.T) {
	k := &khoNDGia{dongHienCo: draftRow(), loiSau: "INSERT INTO audit_log"}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	if err := uc.Delete(ctx, draftRow().ID, "Đăng nhầm", nguoiSoanND()); err == nil {
		t.Fatal("vết hỏng mà xoá vẫn thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("chốt=%d huỷ=%d, muốn 0/1 — xoá và vết phải cùng còn hoặc cùng mất", k.daCommit, k.daRollback)
	}
}

func TestDeleteContentItemWithdrawsThePublishedCoverAfterCommit(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiDangHien, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID, PublishedAt: lucNDPinned}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	f := readyCover(files, coverFileID, coverItemID)
	f.PublicObjectKey = "public-media/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/thumb-1280.jpg"
	wantKey := f.PublicObjectKey

	if err := uc.Delete(ctx, coverItemID, "Ảnh sai người", nguoiSoanND()); err != nil {
		t.Fatalf("xoá lỗi: %v", err)
	}
	if len(objects.unpublished) != 1 || objects.unpublished[0] != wantKey {
		t.Fatalf("unpublished = %v, muốn [%s]", objects.unpublished, wantKey)
	}
	if len(objects.published) != 0 {
		t.Error("xoá mà lại đăng ảnh")
	}
	// Two transactions, as for an unpublish: the delete (trail inside), then the key cleared with its own.
	if k.daCommit != 2 || files.get(coverFileID).PublicObjectKey != "" {
		t.Errorf("commits=%d key=%q", k.daCommit, files.get(coverFileID).PublicObjectKey)
	}
	if n := len(k.cau("INSERT INTO audit_log")); n != 2 {
		t.Errorf("số vết = %d, muốn 2 (xoá, gỡ ảnh công khai)", n)
	}
}

func TestDeleteContentItemSurvivesAFailedWithdrawalAndKeepsTheKey(t *testing.T) {
	item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiDangHien, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		CoverImageFileID: coverFileID, PublishedAt: lucNDPinned}
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	f := readyCover(files, coverFileID, coverItemID)
	f.PublicObjectKey = "public-media/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-image/" +
		strings.ToLower(coverFileID) + "/thumb-1280.jpg"
	objects.unpublishErr = errors.New("minio down")

	// Removing an item from residents' view must never wait on storage.
	if err := uc.Delete(ctx, coverItemID, "Ảnh sai người", nguoiSoanND()); err != nil {
		t.Fatalf("xoá phải thành công dù MinIO hỏng: %v", err)
	}
	if files.get(coverFileID).PublicObjectKey == "" {
		t.Error("gỡ hỏng mà khoá công khai bị xoá — không còn dấu vết rằng bản sao có thể còn")
	}
}
