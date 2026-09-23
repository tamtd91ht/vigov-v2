package store

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for migration 0006 — the task register, its progress log and its extension
// requests.
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: every test here SKIPS unless VIGOV_TEST_DSN is set, and
// the package still prints `ok`. NO PostgreSQL IS REACHABLE FROM THE ENVIRONMENT THIS WAS WRITTEN IN
// — the DSN is unset and Docker is off — so NONE of the assertions below have ever been executed.
// `ok` next to this package means the fake-driver suites ran; it does not mean the SQL here was
// verified. The schema harness (TestMain, moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go
// and is shared.
//
// WHY THEY ARE WORTH WRITING WHILE THEY CANNOT RUN: every property below is enforced by the DATABASE
// and by nothing else, so the fake-driver suites cannot see any of it. Each one is also a rule that,
// broken, produces NO error at all:
//
//	an edited `han_ban_dau`              every granted extension reads as a deadline met, in §11.3's
//	                                     ratio, retroactively
//	a reissued `ma`                      two tasks sharing one number in minutes nobody rewrites
//	a hard DELETE                        an administrative record and its whole log destroyed
//	an edited progress entry             a timeline with no evidentiary value
//	two pending extension requests       a leader with two buttons and no way to tell what either
//	                                     one means
//	a `loai` no catalogue defines        a raw slug on the screen where a label should be

// themNhiemVu inserts one task and returns the error UNTOUCHED, so a test can assert on the
// constraint that refused it. Every value is passed EXPLICITLY, including those the schema defaults:
// a test resting on a default cannot tell "the column is written" from "every row happens to carry
// the same value".
func themNhiemVu(db *sql.DB, xa, id, ma, loai, trangThai string,
	hanXuLy, hanBanDau, ngayHoanThanh any) error {

	_, err := db.Exec(
		`INSERT INTO nhiem_vu
		 (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao,
		  han_xu_ly, han_ban_dau, ngay_hoan_thanh, tien_do, nguoi_tao_ma)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		xa, id, ma, loai, "Nhiệm vụ dùng cho phép kiểm.", trangThai, "truc-tiep",
		hanXuLy, hanBanDau, ngayHoanThanh, 0, "CB-00123")
	return err
}

// danhMucChoXa sows the ONE catalogue row every task below needs.
//
// IT IS NOT DECORATION. `nhiem_vu.loai` carries a real FOREIGN KEY into `loai_nhiem_vu` (migration
// 0006), so a commune with an empty catalogue cannot hold a task at all — which is the fail-closed
// state every commune is in today, since nothing seeds those rows and the onboarding step does not
// exist. A test that skipped this step would fail for a reason that has nothing to do with what it
// is asserting.
func danhMucChoXa(t *testing.T, db *sql.DB, xa string) {
	t.Helper()
	themLoai(t, db, xa, "lnv-"+xa[:8], "theo-van-ban", "Theo văn bản", 1, true)
	themUuTien(t, db, xa, "uu-"+xa[:8], "cao", "Cao", 2)
}

// The fixture instants are FIXED and all different — see the note in nhiem_vu_test.go.
var (
	mocHanPg    = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mocHanGocPg = time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	mocXongPg   = time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
)

// TestPgCotNhiemVuTrongMaKhopVoiLuocDoThat is THE ONE ASSERTION ANYWHERE that catches a later
// migration renaming a column out from under this store.
//
// cotNhiemVu is a string, and the fake driver builds its rows from that same string — so a column
// misspelled from the start, or renamed months from now, is invisible to every test that does not
// touch a real schema. It surfaces as a 500 on a member of staff's screen with "column … does not
// exist" in a log nobody is reading yet.
func TestPgCotNhiemVuTrongMaKhopVoiLuocDoThat(t *testing.T) {
	db := moKetNoi(t)

	for _, tr := range []struct {
		bang string
		cot  []string
	}{
		{"nhiem_vu", append(tachCot(cotNhiemVu),
			// THREE COLUMNS THAT NEVER APPEAR IN A SELECT LIST, because the statement is built
			// around them and cannot run without them.
			"tenant_id",  // the predicate that keeps two public authorities apart (rule 1)
			"deleted_at", // the predicate that keeps removed rows out of every read (rule 7)
			"cap_nhat_luc",
		)},
		// THE TWO TABLES THAT HAVE NO STORE YET. They are asserted here because migration 0006
		// creates them and the NEXT pass writes against them: a column misspelled today is cheapest
		// to find today, and this is the only test in the repository that can see them at all.
		{"nhat_ky_nhiem_vu", []string{
			"tenant_id", "id", "nhiem_vu_id", "thoi_diem", "nguoi_ma",
			"trang_thai_tai_thoi_diem", "bo_phan_id", "nguoi_phu_trach_ma", "noi_dung", "dinh_kem",
		}},
		{"de_nghi_lui_han", []string{
			"tenant_id", "id", "nhiem_vu_id", "nguoi_de_nghi_ma", "nguoi_duyet_ma",
			"han_moi", "ly_do", "trang_thai", "thoi_diem", "duyet_luc", "deleted_at",
		}},
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

// TestPgKhongDoiDuocHanBanDau is the single most valuable assertion in this file.
//
// §5.6 promises on the screen that the original deadline does not move when an extension is granted,
// and §11.3 measures the on-time ratio against it. An UPDATE that moved it alongside `han_xu_ly`
// would not fail, would not log, and would make every extension ever granted read as a deadline met
// — retroactively, in a figure that goes upward.
func TestPgKhongDoiDuocHanBanDau(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-han-1", "NV01", "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}

	// GRANTING AN EXTENSION MUST WORK, or the refusal below would be indistinguishable from a row
	// nobody can update at all.
	if _, err := db.Exec(
		`UPDATE nhiem_vu SET han_xu_ly = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-han-1", mocHanPg); err != nil {
		t.Fatalf("gia hạn bị chặn: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE nhiem_vu SET han_ban_dau = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-han-1", mocHanPg); err == nil {
		t.Error("đổi được `han_ban_dau` — mọi lần gia hạn sẽ đọc thành đúng hạn (§11.3)")
	}
}

func TestPgKhongXoaCungDuocNhiemVuVaKhongDoiDuocMa(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-lt-1", "NV02", "theo-van-ban", "moi-giao",
		nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM nhiem_vu WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-lt-1"); err == nil {
		t.Error("xoá cứng được một nhiệm vụ — luật 7 bất biến 1, và đặc tả §11.5 cũng nói xoá mềm")
	}

	if _, err := db.Exec(`UPDATE nhiem_vu SET ma = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-lt-1", "NV99"); err == nil {
		t.Error("đổi được mã nhiệm vụ — số đã cấp nằm trong biên bản họp, không ai viết lại (luật 7 bất biến 3)")
	}

	// AND THE SOFT DELETE MUST STILL WORK, or the refusals above would be indistinguishable from a
	// table nobody can write to at all.
	if _, err := db.Exec(
		`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-lt-1", "CB-00123", "trùng nhiệm vụ"); err != nil {
		t.Fatalf("xoá mềm bị chặn: %v", err)
	}
}

// TestPgMaNhiemVuKhongCapLaiKeCaSauXoaMem: the unique key counts soft-deleted rows on purpose.
// Restricted to live rows, a commune could soft-delete NV19 and mint a second NV19 — and the meeting
// minutes quoting the first would then name the second.
func TestPgMaNhiemVuKhongCapLaiKeCaSauXoaMem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-cl-1", "NV19", "theo-van-ban", "moi-giao",
		nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-cl-1", "CB-00123", "nhập nhầm"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	if err := themNhiemVu(db, xa, "nv-cl-2", "NV19", "theo-van-ban", "moi-giao",
		nil, nil, nil); err == nil {
		t.Error("cấp lại được mã NV19 sau khi xoá mềm — hai nhiệm vụ mang một số trong sổ")
	}
}

// TestPgHaiXaCungMaNhiemVuVanGhiDuoc is the other half of the key: it is COMPOSITE with tenant_id.
// A single-column unique key here would make the second commune unable to onboard — every commune's
// register starts at NV01 — and no test of one commune could show it.
func TestPgHaiXaCungMaNhiemVuVanGhiDuoc(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	danhMucChoXa(t, db, xaA)
	danhMucChoXa(t, db, xaB)

	for _, xa := range []string{xaA, xaB} {
		if err := themNhiemVu(db, xa, "nv-hx-1", "NV01", "theo-van-ban", "moi-giao",
			nil, nil, nil); err != nil {
			t.Fatalf("xã %s không ghi được: %v — khoá duy nhất phải GHÉP với tenant_id", xa, err)
		}
	}
}

func TestPgNhiemVuChiDocXaCuaMinh(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	danhMucChoXa(t, db, xaA)

	if err := themNhiemVu(db, xaA, "nv-cly-a", "NV07", "theo-van-ban", "moi-giao",
		nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))

	if _, err := s.TheoMa(ctxXa(tenant.ID(xaA)), "NV07"); err != nil {
		t.Fatalf("xã A không đọc được nhiệm vụ của chính mình: %v", err)
	}
	// THE SAME NUMBER, THE SAME STORE, A DIFFERENT COMMUNE IN THE CONTEXT. Data leaking from one
	// commune to another is a breach between two public authorities, not a bug.
	if _, err := s.TheoMa(ctxXa(tenant.ID(xaB)), "NV07"); !errors.Is(err, ErrNhiemVuKhongTonTai) {
		t.Fatalf("xã B đọc được nhiệm vụ của xã A: lỗi = %v", err)
	}
}

func TestPgTrangThaiNgoaiBayMaBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	// `chua-thuc-hien` is what the KANBAN BOARD calls `moi-giao` (§4.1) — a LABEL, and the one most
	// likely to be typed as a code by somebody reading the screen instead of §6. The others are a
	// state the BA/PM note explicitly says must NOT be a status, an English spelling, and empty.
	for i, tt := range []string{"chua-thuc-hien", "cho-duyet-lui-han", "in-progress", "moi_giao", ""} {
		if err := themNhiemVu(db, xa, fmt.Sprintf("nv-tt-%d", i), fmt.Sprintf("NV%02d", 20+i),
			"theo-van-ban", tt, nil, nil, nil); err == nil {
			t.Errorf("ghi được trạng thái %q ngoài bảy mã đã chốt (câu hỏi mở #21)", tt)
		}
	}
}

// TestPgLoaiPhaiCoTrongDanhMucCuaCHINHXaDo is what the foreign key buys, and the second half is the
// part a single-commune test cannot show: the key is COMPOSITE, so commune B's catalogue entry does
// not let commune A use the code.
func TestPgLoaiPhaiCoTrongDanhMucCuaChinhXaDo(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	danhMucChoXa(t, db, xaB) // ONLY commune B has the catalogue

	if err := themNhiemVu(db, xaA, "nv-fk-1", "NV30", "theo-van-ban", "moi-giao",
		nil, nil, nil); err == nil {
		t.Error("ghi được nhiệm vụ với loại mà danh mục CỦA XÃ ĐÓ không có — màn hình sẽ hiện " +
			"slug thô ở chỗ đáng lẽ là nhãn")
	}

	danhMucChoXa(t, db, xaA)
	if err := themNhiemVu(db, xaA, "nv-fk-2", "NV31", "theo-van-ban", "moi-giao",
		nil, nil, nil); err != nil {
		t.Errorf("có danh mục rồi vẫn không ghi được: %v", err)
	}
}

// TestPgHaiHanCungCoCungKhong: a task with one deadline and not the other is either a commitment
// with no recorded origin — so §11.3's ratio has no denominator for it — or an origin for a
// commitment that was never made.
func TestPgHaiHanCungCoCungKhong(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	for i, c := range []struct {
		ten            string
		han, hanBanDau any
		nhanVe         bool
	}{
		{"không hạn nào", nil, nil, true},
		{"đủ hai hạn", mocHanPg, mocHanGocPg, true},
		{"chỉ có hạn hiện hành", mocHanPg, nil, false},
		{"chỉ có hạn ban đầu", nil, mocHanGocPg, false},
	} {
		err := themNhiemVu(db, xa, fmt.Sprintf("nv-2h-%d", i), fmt.Sprintf("NV%02d", 40+i),
			"theo-van-ban", "moi-giao", c.han, c.hanBanDau, nil)
		if c.nhanVe && err != nil {
			t.Errorf("%s: bị từ chối (%v)", c.ten, err)
		}
		if !c.nhanVe && err == nil {
			t.Errorf("%s: được nhận", c.ten)
		}
	}
}

// TestPgHoanThanhThiPhaiCoNgayHoanThanh: §11.3 computes the on-time ratio from `ngay_hoan_thanh`. A
// task in `hoan-thanh` without one silently leaves the numerator, and an instant on a task that is
// not finished would put it in.
func TestPgHoanThanhThiPhaiCoNgayHoanThanh(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-ht-1", "NV50", "theo-van-ban", "hoan-thanh",
		mocHanPg, mocHanGocPg, nil); err == nil {
		t.Error("ghi được nhiệm vụ `hoan-thanh` không có ngày hoàn thành — tỷ lệ đúng hạn mất mẫu")
	}
	if err := themNhiemVu(db, xa, "nv-ht-2", "NV51", "theo-van-ban", "dang-thuc-hien",
		mocHanPg, mocHanGocPg, mocXongPg); err == nil {
		t.Error("ghi được ngày hoàn thành cho nhiệm vụ CHƯA hoàn thành — tỷ lệ đúng hạn thừa mẫu")
	}
	if err := themNhiemVu(db, xa, "nv-ht-3", "NV52", "theo-van-ban", "hoan-thanh",
		mocHanPg, mocHanGocPg, mocXongPg); err != nil {
		t.Errorf("nhiệm vụ hoàn thành hợp lệ bị từ chối: %v", err)
	}
}

// --- the progress log ------------------------------------------------------------------------------

func themNhatKy(db *sql.DB, xa, id, nhiemVuID, trangThai, noiDung string) error {
	_, err := db.Exec(
		`INSERT INTO nhat_ky_nhiem_vu
		 (tenant_id, id, nhiem_vu_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem, noi_dung)
		 VALUES ($1,$2,$3,now(),$4,$5,$6)`,
		xa, id, nhiemVuID, "CB-00311", trangThai, noiDung)
	return err
}

// TestPgNhatKyChiThem — rule 7, forbidden #5. A timeline entry that can be edited afterwards is a
// record with no evidentiary value, which is precisely the thing it exists to be.
func TestPgNhatKyChiThem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-nk-1", "NV60", "theo-van-ban", "dang-thuc-hien",
		nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if err := themNhatKy(db, xa, "nk-1", "nv-nk-1", "dang-thuc-hien", "Đã liên hệ ba chi bộ."); err != nil {
		t.Fatalf("ghi nhật ký: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE nhat_ky_nhiem_vu SET noi_dung = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "nk-1", "Sửa lại cho đẹp"); err == nil {
		t.Error("sửa được một dòng nhật ký — sổ chỉ-thêm mà sửa được thì không còn giá trị chứng cứ")
	}
	if _, err := db.Exec(`DELETE FROM nhat_ky_nhiem_vu WHERE tenant_id = $1 AND id = $2`,
		xa, "nk-1"); err == nil {
		t.Error("xoá được một dòng nhật ký")
	}

	// THE CORRECTION PATH MUST STILL WORK: another entry carrying the correction.
	if err := themNhatKy(db, xa, "nk-2", "nv-nk-1", "dang-thuc-hien",
		"Đính chính: mới liên hệ được hai chi bộ."); err != nil {
		t.Errorf("không ghi thêm được dòng đính chính: %v", err)
	}
}

func TestPgNhatKyTrangThaiNgoaiBayMaBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-nk-2", "NV61", "theo-van-ban", "dang-thuc-hien",
		nil, nil, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	// A timeline holding a status the register would refuse is a timeline that contradicts the row
	// it hangs off.
	if err := themNhatKy(db, xa, "nk-x", "nv-nk-2", "chua-thuc-hien", "Ghi chép."); err == nil {
		t.Error("ghi được trạng thái ngoài bảy mã vào nhật ký")
	}
}

// --- the extension requests --------------------------------------------------------------------------

func themDeNghi(db *sql.DB, xa, id, nhiemVuID, trangThai string, nguoiDuyet, duyetLuc any) error {
	_, err := db.Exec(
		`INSERT INTO de_nghi_lui_han
		 (tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, nguoi_duyet_ma, han_moi, ly_do,
		  trang_thai, thoi_diem, duyet_luc)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),$9)`,
		xa, id, nhiemVuID, "CB-00311", nguoiDuyet, mocHanPg,
		"Chờ số liệu của ba chi bộ.", trangThai, duyetLuc)
	return err
}

// TestPgMotDeNghiLuiHanChoDuyetMotLuc: two pending requests carrying two different `han_moi` is a
// leader with two buttons and no way to tell what approving either one means; approving both moves
// the commitment twice, and the second move is invisible on a screen showing one chip.
func TestPgMotDeNghiLuiHanChoDuyetMotLuc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-dn-1", "NV70", "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if err := themDeNghi(db, xa, "dn-1", "nv-dn-1", "cho-duyet", nil, nil); err != nil {
		t.Fatalf("gửi đề nghị: %v", err)
	}
	if err := themDeNghi(db, xa, "dn-2", "nv-dn-1", "cho-duyet", nil, nil); err == nil {
		t.Error("gửi được đề nghị lùi hạn thứ hai khi đề nghị thứ nhất còn chờ duyệt")
	}

	// REJECTING THE FIRST FREES THE SLOT — the generated marker goes NULL, and NULLs do not collide.
	// Without this half the test above would pass on a table that accepted no second request ever.
	if _, err := db.Exec(
		`UPDATE de_nghi_lui_han SET trang_thai = 'tu-choi', nguoi_duyet_ma = $3, duyet_luc = now()
		 WHERE tenant_id = $1 AND id = $2`, xa, "dn-1", "CB-00007"); err != nil {
		t.Fatalf("từ chối đề nghị: %v", err)
	}
	if err := themDeNghi(db, xa, "dn-3", "nv-dn-1", "cho-duyet", nil, nil); err != nil {
		t.Errorf("đề nghị bị từ chối rồi mà vẫn không gửi lại được: %v", err)
	}
}

// TestPgDeNghiDaQuyetPhaiCoNguoiDuyet — rule 6, invariant 8. An approval that moved a commitment and
// is attributed to nobody is exactly the row an inspection asks about.
func TestPgDeNghiDaQuyetPhaiCoNguoiDuyet(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-dn-2", "NV71", "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if err := themDeNghi(db, xa, "dn-10", "nv-dn-2", "da-duyet", nil, nil); err == nil {
		t.Error("ghi được đề nghị ĐÃ DUYỆT mà không có người duyệt và thời điểm duyệt")
	}
	if err := themDeNghi(db, xa, "dn-11", "nv-dn-2", "cho-duyet", "CB-00007", nil); err == nil {
		t.Error("ghi được đề nghị CÒN CHỜ mà đã có người duyệt")
	}
	if err := themDeNghi(db, xa, "dn-12", "nv-dn-2", "da-duyet", "CB-00007", mocHanPg); err != nil {
		t.Errorf("đề nghị đã duyệt hợp lệ bị từ chối: %v", err)
	}
}

// TestPgDeNghiDaGuiThiKhongSuaDuocNoiDung: only the DECISION may move. Rewriting the requested date
// after a decision leaves an approval attached to something the approver never read.
func TestPgDeNghiDaGuiThiKhongSuaDuocNoiDung(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)

	if err := themNhiemVu(db, xa, "nv-dn-3", "NV72", "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if err := themDeNghi(db, xa, "dn-20", "nv-dn-3", "cho-duyet", nil, nil); err != nil {
		t.Fatalf("gửi đề nghị: %v", err)
	}

	for ten, cau := range map[string]string{
		"hạn mới": `UPDATE de_nghi_lui_han SET han_moi = now() WHERE tenant_id = $1 AND id = $2`,
		"lý do":   `UPDATE de_nghi_lui_han SET ly_do = 'khác' WHERE tenant_id = $1 AND id = $2`,
		"người đề nghị": `UPDATE de_nghi_lui_han SET nguoi_de_nghi_ma = 'CB-99999'
		                  WHERE tenant_id = $1 AND id = $2`,
	} {
		if _, err := db.Exec(cau, xa, "dn-20"); err == nil {
			t.Errorf("sửa được %s của một đề nghị đã gửi", ten)
		}
	}

	// THE DECISION MUST STILL BE WRITABLE, or the refusals above would be a row nobody can decide.
	if _, err := db.Exec(
		`UPDATE de_nghi_lui_han SET trang_thai = 'da-duyet', nguoi_duyet_ma = $3, duyet_luc = now()
		 WHERE tenant_id = $1 AND id = $2`, xa, "dn-20", "CB-00007"); err != nil {
		t.Fatalf("duyệt đề nghị bị chặn: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM de_nghi_lui_han WHERE tenant_id = $1 AND id = $2`,
		xa, "dn-20"); err == nil {
		t.Error("xoá cứng được một đề nghị lùi hạn")
	}
}
