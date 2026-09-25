package store

import (
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration tests for migration 0013 — the petition processing logbook — and the store that
// writes and reads it.
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: every test here SKIPS unless VIGOV_TEST_DSN is set, and
// the package still prints `ok`. NO PostgreSQL IS REACHABLE FROM THE ENVIRONMENT THIS WAS WRITTEN IN,
// so NONE of these assertions has been executed yet. The harness (TestMain, moKetNoi, xaRieng) lives
// in danh_muc_nhiem_vu_pg_test.go.
//
// WHY THEY ARE WORTH WRITING WHILE THEY CANNOT RUN: the append-only trigger and the CHECKs are
// enforced by the DATABASE and by nothing else, so the fake-driver suites cannot see any of it — and
// the store's column list is a string the fake driver accepts whatever it says.

// ghiNhatKyPg writes one row THROUGH THE STORE, in a real transaction, so the store's INSERT column
// list meets the real schema.
func ghiNhatKyPg(t *testing.T, db *sql.DB, xa string, e domain.NhatKyPhanAnh) error {
	t.Helper()
	kho := pkgstore.New(db)
	ctx := ctxXa(tenant.ID(xa))
	s := NewPhieuPhanAnhStore(kho)
	return kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.GhiNhatKy(ctx, tx, e)
	})
}

func dongNhatKyPg(id, phieuID string, hanhVi domain.HanhViNhatKy, luc time.Time) domain.NhatKyPhanAnh {
	return domain.NhatKyPhanAnh{
		ID: id, PhieuPhanAnhID: phieuID, ThoiDiem: luc, NguoiMa: "CB-00311",
		HanhVi: hanhVi, TrangThai: domain.DangXuLy,
	}
}

// TestPgNhatKyPhanAnhChiThem — rule 7, forbidden #5. A timeline entry that can be edited or removed
// afterwards is a record with no evidentiary value.
func TestPgNhatKyPhanAnhChiThem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	luc := time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC)

	e := dongNhatKyPg("nkpa-1", "pa-nk-1", domain.NhatKyGhiChu, luc)
	e.NoiDung = "Đã liên hệ tổ trưởng tổ dân phố."
	if err := ghiNhatKyPg(t, db, xa, e); err != nil {
		t.Fatalf("ghi nhật ký qua store: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE nhat_ky_phan_anh SET noi_dung = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "nkpa-1", "Sửa lại cho đẹp"); err == nil {
		t.Error("sửa được một dòng nhật ký phiếu — sổ chỉ-thêm mà sửa được thì không còn giá trị chứng cứ")
	}
	if _, err := db.Exec(`DELETE FROM nhat_ky_phan_anh WHERE tenant_id = $1 AND id = $2`,
		xa, "nkpa-1"); err == nil {
		t.Error("xoá được một dòng nhật ký phiếu")
	}

	// THE CORRECTION PATH MUST STILL WORK: another entry carrying the correction.
	sua := dongNhatKyPg("nkpa-2", "pa-nk-1", domain.NhatKyGhiChu, luc)
	sua.NoiDung = "Đính chính: đã liên hệ tổ phó, không phải tổ trưởng."
	if err := ghiNhatKyPg(t, db, xa, sua); err != nil {
		t.Errorf("không ghi thêm được dòng đính chính: %v", err)
	}
}

// TestPgNhatKyPhanAnhRangBuoc — the CHECKs the use case validates first, proved to be the floor.
func TestPgNhatKyPhanAnhRangBuoc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	luc := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)

	ghiChuRong := dongNhatKyPg("nkpa-r1", "pa-nk-2", domain.NhatKyGhiChu, luc)
	if err := ghiNhatKyPg(t, db, xa, ghiChuRong); err == nil {
		t.Error("ghi được dòng `ghi-chu` không có nội dung")
	}

	lacCanBo := dongNhatKyPg("nkpa-r2", "pa-nk-2", domain.NhatKyChuyenTrangThai, luc)
	lacCanBo.CanBoXuLyMa = "CB-00777"
	if err := ghiNhatKyPg(t, db, xa, lacCanBo); err == nil {
		t.Error("ghi được người nhận việc trên một dòng không phải `phan-cong` — đọc thành một lần giao việc chưa từng có")
	}

	phanCongThieuBoPhan := dongNhatKyPg("nkpa-r3", "pa-nk-2", domain.NhatKyPhanCong, luc)
	if err := ghiNhatKyPg(t, db, xa, phanCongThieuBoPhan); err == nil {
		t.Error("ghi được dòng `phan-cong` không có bộ phận")
	}

	phanCong := dongNhatKyPg("nkpa-r4", "pa-nk-2", domain.NhatKyPhanCong, luc)
	phanCong.BoPhanID, phanCong.CanBoXuLyMa = "bp-001", "CB-00777"
	if err := ghiNhatKyPg(t, db, xa, phanCong); err != nil {
		t.Errorf("dòng `phan-cong` hợp lệ bị từ chối: %v", err)
	}
}

// TestPgNhatKyPhanAnhDocMoiNhatTruocVaTheoXa — the read path: newest first, `id` as the tie-break,
// one petition only, one commune only.
func TestPgNhatKyPhanAnhDocMoiNhatTruocVaTheoXa(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	som := time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)
	muon := time.Date(2026, 9, 26, 2, 0, 0, 0, time.UTC)

	for _, e := range []domain.NhatKyPhanAnh{
		dongNhatKyPg("nkpa-d1", "pa-nk-3", domain.NhatKyPhanLoai, som),
		// Two rows of ONE transaction share an instant; the id decides.
		dongNhatKyPg("nkpa-d2", "pa-nk-3", domain.NhatKyChuyenTrangThai, muon),
		dongNhatKyPg("nkpa-d3", "pa-nk-3", domain.NhatKyChuyenTrangThai, muon),
		// Another petition of the same commune — must not appear.
		dongNhatKyPg("nkpa-d4", "pa-nk-4", domain.NhatKyChuyenTrangThai, muon),
	} {
		if err := ghiNhatKyPg(t, db, xa, e); err != nil {
			t.Fatalf("ghi %s: %v", e.ID, err)
		}
	}
	// The SAME petition id in another commune — must not appear either (rule 1).
	if err := ghiNhatKyPg(t, db, xaKhac, dongNhatKyPg("nkpa-d5", "pa-nk-3",
		domain.NhatKyChuyenTrangThai, muon)); err != nil {
		t.Fatalf("ghi của xã khác: %v", err)
	}

	yc, err := page.Parse(url.Values{}, SapXepNhatKyPhieu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	kq, err := NewPhieuPhanAnhStore(pkgstore.New(db)).NhatKyCuaPhieu(ctxXa(tenant.ID(xa)), "pa-nk-3", yc)
	if err != nil {
		t.Fatalf("đọc nhật ký: %v", err)
	}
	var ids []string
	for _, e := range kq.Items {
		ids = append(ids, e.ID)
	}
	muonIDs := []string{"nkpa-d3", "nkpa-d2", "nkpa-d1"}
	if len(ids) != len(muonIDs) {
		t.Fatalf("đọc được %v, muốn %v", ids, muonIDs)
	}
	for i := range muonIDs {
		if ids[i] != muonIDs[i] {
			t.Errorf("thứ tự = %v, muốn %v", ids, muonIDs)
			break
		}
	}
}
