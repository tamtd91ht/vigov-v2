package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for migration 0004 — the petition register and its label overrides.
//
// READ THIS BEFORE BELIEVING A GREEN RUN: every test here SKIPS unless VIGOV_TEST_DSN is set, and
// the package still prints `ok`. The schema harness (TestMain, moKetNoi, xaRieng) lives in
// danh_muc_nhiem_vu_pg_test.go and is shared.
//
// WHY THESE ARE WORTH WRITING WHILE THEY CANNOT RUN: every property below is enforced by the
// DATABASE and by nothing else, so the fake-driver suites cannot see any of it. Each one is also
// a rule that, broken, produces no error at all:
//
//	`0 giờ` written into han_tiep_nhan of a staff-booked petition   an average acknowledge time
//	                                                                near zero, reported upward
//	a `goc_dem_han` eight days back                                 a petition that was overdue
//	                                                                before it existed
//	a hard DELETE                                                   an archival record destroyed
//	a reissued ma_tra_cuu                                           a citizen's slip opening
//	                                                                somebody else's petition

// themPhieu inserts one petition and returns the error UNTOUCHED, so a test can assert on the
// constraint that refused it. Every value is passed EXPLICITLY, including those the schema
// defaults: a test resting on a default cannot tell "the column is written" from "every row
// happens to carry the same value".
func themPhieu(db *sql.DB, xa, id, ma, kenh, linhVuc, trangThai string,
	gocDemHan, vaoSoLuc time.Time, hanTiepNhan, hanXuLyXong any) error {

	_, err := db.Exec(
		`INSERT INTO phieu_phan_anh
		 (tenant_id, id, ma_tra_cuu, kenh_tiep_nhan, noi_dung, linh_vuc, trang_thai,
		  goc_dem_han, vao_so_luc, han_tiep_nhan, han_xu_ly_xong)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		xa, id, ma, kenh, "Nội dung phản ánh dùng cho phép kiểm.", nullHoac(linhVuc), trangThai,
		gocDemHan, vaoSoLuc, hanTiepNhan, hanXuLyXong)
	return err
}

func nullHoac(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func TestPgCotPhieuTrongMaKhopVoiLuocDoThat(t *testing.T) {
	// THE ONE ASSERTION ANYWHERE THAT CATCHES A LATER MIGRATION RENAMING A COLUMN OUT FROM UNDER
	// THIS STORE. cotPhieu is a string, and the fake driver builds its rows from that same
	// string — so a column misspelled from the start, or renamed months from now, is invisible to
	// every test that does not touch a real schema. It surfaces as a 500 on a member of staff's
	// screen with "column … does not exist" in a log nobody is reading yet.
	db := moKetNoi(t)

	for _, tr := range []struct {
		bang string
		cot  []string
	}{
		{"phieu_phan_anh", append(tachCot(cotPhieu),
			// THREE COLUMNS THAT NEVER APPEAR IN A SELECT LIST, because the statement is built
			// around them and cannot run without them.
			"tenant_id",  // the predicate that keeps two public authorities apart (rule 1)
			"deleted_at", // the predicate that keeps archival rows out of every read (rule 7)
			"cap_nhat_luc",
		)},
		{"nhan_linh_vuc", append(tachCot(cotNhanLinhVuc), "tenant_id", "deleted_at", "thu_tu")},
	} {
		for _, c := range tr.cot {
			var co bool
			err := db.QueryRow(
				`SELECT EXISTS (SELECT 1 FROM information_schema.columns
				 WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2)`,
				tr.bang, c).Scan(&co)
			if err != nil {
				t.Fatalf("tra information_schema: %v", err)
			}
			if !co {
				t.Errorf("bảng %s không có cột %q mà mã đang đọc", tr.bang, c)
			}
		}
	}
}

func tachCot(danh string) []string {
	var ra []string
	for _, c := range strings.Split(danh, ",") {
		if c = strings.TrimSpace(strings.ReplaceAll(c, "\n", "")); c != "" {
			ra = append(ra, strings.TrimSpace(c))
		}
	}
	return ra
}

// TestPgPhieuNhapHoKhongBaoGioCoHanTiepNhan is ADR 0028, decision F #5, enforced by the database
// rather than promised by a comment.
//
// `0 giờ` in this column would make every staff-booked petition a valid "acknowledged instantly"
// sample. A commune that books many of them would report an average acknowledge time near
// nothing — a figure that is beautiful, wrong, and unreadable from the number itself.
func TestPgPhieuNhapHoKhongBaoGioCoHanTiepNhan(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	gui := time.Now().UTC().Add(-2 * time.Hour)

	err := themPhieu(db, xa, "pa-nh-1", "PA-NH1-0000-0001", "can-bo-nhap-ho", "dien", "da-tiep-nhan",
		gui, gui.Add(time.Hour), gui.Add(90*time.Minute), gui.Add(48*time.Hour))
	if err == nil {
		t.Fatal("ghi được hạn tiếp nhận cho phiếu nhập hộ — ADR 0028 điều kiện dừng #2")
	}

	// And the same row WITHOUT it must be accepted, or the test above would pass for the wrong
	// reason — any insert failure at all would look like the constraint working.
	if err := themPhieu(db, xa, "pa-nh-2", "PA-NH2-0000-0002", "can-bo-nhap-ho", "dien", "da-tiep-nhan",
		gui, gui.Add(time.Hour), nil, gui.Add(48*time.Hour)); err != nil {
		t.Fatalf("phiếu nhập hộ hợp lệ bị từ chối: %v", err)
	}
}

// TestPgPhieuNhapHoBatBuocCoLinhVuc: the staff form requires the field at booking, and that is
// the whole reason its resolve deadline can be fixed there. A row claiming the channel with no
// field would sit with a NULL han_xu_ly_xong nobody will ever come back to fill, because the
// classification step it depends on never happens on that channel.
func TestPgPhieuNhapHoBatBuocCoLinhVuc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	gui := time.Now().UTC().Add(-2 * time.Hour)

	if err := themPhieu(db, xa, "pa-nh-3", "PA-NH3-0000-0003", "can-bo-nhap-ho", "", "da-tiep-nhan",
		gui, gui.Add(time.Hour), nil, gui.Add(48*time.Hour)); err == nil {
		t.Fatal("ghi được phiếu nhập hộ không có lĩnh vực")
	}
}

// TestPgGocDemHanChanBayNgayVaTuChoiChuKhongCatVeBien — ADR 0028, decision F #3.
//
// An arbitrary past mark is the right to manufacture an already-overdue petition for somebody
// else, or to hide a late one by moving its origin back. Both falsify a figure through a form
// field, with no software fault involved.
func TestPgGocDemHanChanBayNgayVaTuChoiChuKhongCatVeBien(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	vaoSo := time.Now().UTC()

	for i, c := range []struct {
		ten    string
		goc    time.Time
		nhanVe bool
	}{
		{"đúng lúc vào sổ", vaoSo, true},
		{"bảy ngày trước — đúng biên", vaoSo.AddDate(0, 0, -7).Add(time.Minute), true},
		{"tám ngày trước", vaoSo.AddDate(0, 0, -8), false},
		{"muộn hơn lúc vào sổ", vaoSo.Add(time.Minute), false},
	} {
		err := themPhieu(db, xa, fmt.Sprintf("pa-goc-%d", i),
			fmt.Sprintf("PA-GOC%d-0000-0000", i), "can-bo-nhap-ho", "dien", "da-tiep-nhan",
			c.goc, vaoSo, nil, vaoSo.Add(48*time.Hour))
		if c.nhanVe && err != nil {
			t.Errorf("%s: bị từ chối (%v)", c.ten, err)
		}
		if !c.nhanVe && err == nil {
			t.Errorf("%s: được nhận — lẽ ra TỪ CHỐI, không cắt về biên", c.ten)
		}
	}
}

func TestPgTrangThaiNgoaiChinMaBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	gui := time.Now().UTC()

	// `received` is the ENGLISH string the specification carried before 2026-09-20. It is a name
	// that was REPLACED, not a second spelling still in use, and the database must say so.
	for _, tt := range []string{"received", "screening", "da_tiep_nhan", ""} {
		if err := themPhieu(db, xa, "pa-tt-"+tt, "PA-TT"+tt+"-0000", "web-xa", "", tt,
			gui, gui, gui.Add(8*time.Hour), nil); err == nil {
			t.Errorf("ghi được trạng thái %q ngoài chín mã đã chốt", tt)
		}
	}
}

func TestPgKhongXoaCungDuocPhieuVaKhongDoiDuocMaTraCuu(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	gui := time.Now().UTC()
	const ma = "PA-LUU-TRU-0001"

	if err := themPhieu(db, xa, "pa-lt-1", ma, "web-xa", "", "da-tiep-nhan",
		gui, gui, gui.Add(8*time.Hour), nil); err != nil {
		t.Fatalf("thêm phiếu: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM phieu_phan_anh WHERE tenant_id = $1 AND id = $2`,
		xa, "pa-lt-1"); err == nil {
		t.Error("xoá cứng được một hồ sơ lưu trữ — luật 7 bất biến 1")
	}

	if _, err := db.Exec(
		`UPDATE phieu_phan_anh SET ma_tra_cuu = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "pa-lt-1", "PA-LUU-TRU-0002"); err == nil {
		t.Error("đổi được mã tra cứu — mã đã trao cho dân không bao giờ đánh lại (luật 7 bất biến 3)")
	}

	// AND THE SOFT DELETE MUST STILL WORK, or the refusal above would be indistinguishable from a
	// table nobody can write to at all.
	if _, err := db.Exec(
		`UPDATE phieu_phan_anh SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		xa, "pa-lt-1", "nd-test", "trùng phiếu"); err != nil {
		t.Fatalf("xoá mềm bị chặn: %v", err)
	}
}

// TestPgMaTraCuuKhongCapLaiKeCaSauXoaMem: the unique key counts soft-deleted rows on purpose.
// Restricted to live rows, a commune could soft-delete a petition and later mint the same code
// for an unrelated one — and the citizen holding the old slip would open somebody else's report.
func TestPgMaTraCuuKhongCapLaiKeCaSauXoaMem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	gui := time.Now().UTC()
	const ma = "PA-CAP-LAI-0001"

	if err := themPhieu(db, xa, "pa-cl-1", ma, "web-xa", "", "da-tiep-nhan",
		gui, gui, gui.Add(8*time.Hour), nil); err != nil {
		t.Fatalf("thêm phiếu: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE phieu_phan_anh SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "pa-cl-1"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	if err := themPhieu(db, xa, "pa-cl-2", ma, "web-xa", "", "da-tiep-nhan",
		gui, gui, gui.Add(8*time.Hour), nil); err == nil {
		t.Error("cấp lại được mã tra cứu sau khi xoá mềm — phiếu của dân sẽ mở ra hồ sơ của người khác")
	}
}

// TestPgHaiXaCungMaTraCuuVanGhiDuoc is the other half of the key: it is COMPOSITE with
// tenant_id. A single-column unique key here would make the second commune unable to onboard,
// and no test of one commune could show it.
func TestPgHaiXaCungMaTraCuuVanGhiDuoc(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	gui := time.Now().UTC()
	const ma = "PA-HAI-XA-0001"

	for _, xa := range []string{xaA, xaB} {
		if err := themPhieu(db, xa, "pa-hx-1", ma, "web-xa", "", "da-tiep-nhan",
			gui, gui, gui.Add(8*time.Hour), nil); err != nil {
			t.Fatalf("xã %s không ghi được: %v — khoá duy nhất phải GHÉP với tenant_id", xa, err)
		}
	}
}

func TestPgPhieuChiDocXaCuaMinh(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	gui := time.Now().UTC()
	const ma = "PA-CACH-LY-0001"

	if err := themPhieu(db, xaA, "pa-cl-a", ma, "web-xa", "", "da-tiep-nhan",
		gui, gui, gui.Add(8*time.Hour), nil); err != nil {
		t.Fatalf("thêm phiếu: %v", err)
	}

	s := NewPhieuPhanAnhStore(pkgstore.New(db))

	if _, err := s.TheoMaTraCuu(ctxXa(tenant.ID(xaA)), ma); err != nil {
		t.Fatalf("xã A không đọc được phiếu của chính mình: %v", err)
	}
	// THE SAME CODE, THE SAME STORE, A DIFFERENT COMMUNE IN THE CONTEXT. Data leaking from one
	// commune to another is a breach between two public authorities, not a bug.
	if _, err := s.TheoMaTraCuu(ctxXa(tenant.ID(xaB)), ma); !errors.Is(err, ErrPhieuKhongTonTai) {
		t.Fatalf("xã B đọc được phiếu của xã A: lỗi = %v", err)
	}
}
