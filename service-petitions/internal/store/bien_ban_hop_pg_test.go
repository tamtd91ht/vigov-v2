package store

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for migration 0007 — the meeting-minutes register and its conclusions.
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: every test here SKIPS unless VIGOV_TEST_DSN is set, and
// the package still prints `ok`. NO PostgreSQL IS REACHABLE FROM THE ENVIRONMENT THIS WAS WRITTEN IN
// — the DSN is unset and Docker is off — so NONE of the assertions below have ever been executed.
// `ok` next to this package means the fake-driver suites ran; it does not mean the SQL of 0007 was
// verified. The schema harness (TestMain, moKetNoi, xaRieng, tachCot) lives in
// danh_muc_nhiem_vu_pg_test.go and phieu_phan_anh_pg_test.go and is shared.
//
// WHY THEY ARE WORTH WRITING WHILE THEY CANNOT RUN: every property below is enforced by the DATABASE
// and by nothing else, so the fake-driver suite cannot see any of it — and each is a rule that,
// broken, produces NO error at all:
//
//	a hard DELETE                    minutes, or a conclusion a task still points back at, destroyed
//	                                 with one statement — and §1's traceability gone with it
//	a reused ordinal                 two conclusions that have both been "②" in one meeting, one of
//	                                 which has tasks quoting it
//	a conclusion of another meeting  the foreign key is the only thing that refuses it
//	a LATERAL join that never ran    the badge is the whole point of the screen, and a query that
//	                                 does not parse is a 500 nobody sees until a commune opens it

// themBienBan inserts one meeting and returns the error UNTOUCHED, so a test can assert on the
// constraint that refused it.
func themBienBan(db *sql.DB, xa, id, ten string, ngay any) error {
	_, err := db.Exec(
		`INSERT INTO bien_ban_hop (tenant_id, id, ten_cuoc_hop, ngay_hop, so_hieu, dia_diem,
		                           chu_tri_ma, noi_dung, nguoi_tao_ma)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		xa, id, ten, ngay, "31/BB-UBND", "Phòng họp UBND xã", "CB-00007",
		"Toàn văn biên bản dùng cho phép kiểm.", "CB-00123")
	return err
}

// themKetLuan inserts one conclusion, likewise returning the raw error.
func themKetLuan(db *sql.DB, xa, id, bienBanID string, thuTu int, noiDung string) error {
	_, err := db.Exec(
		`INSERT INTO ket_luan_hop (tenant_id, id, bien_ban_id, thu_tu, noi_dung)
		 VALUES ($1,$2,$3,$4,$5)`,
		xa, id, bienBanID, thuTu, noiDung)
	return err
}

var mocNgayHopPg = time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)

// TestPgCotBienBanTrongMaKhopVoiLuocDoThat is THE ONE ASSERTION ANYWHERE that catches a later
// migration renaming a column out from under this store.
//
// cotBienBan is a string, and the fake driver builds its rows from that same string — so a column
// misspelled from the start, or renamed months from now, is invisible to every test that does not
// touch a real schema. It surfaces as a 500 on a member of staff's screen with "column … does not
// exist" in a log nobody is reading yet.
func TestPgCotBienBanTrongMaKhopVoiLuocDoThat(t *testing.T) {
	db := moKetNoi(t)

	for _, tr := range []struct {
		bang string
		cot  []string
	}{
		{"bien_ban_hop", append(tachCot(cotBienBan),
			// COLUMNS THAT NEVER APPEAR IN THE SELECT LIST, because the statement is built around
			// them or because this pass deliberately does not read them.
			"tenant_id",  // the predicate that keeps two public authorities apart (rule 1)
			"deleted_at", // the predicate that keeps removed rows out of every read (rule 7)
			"deleted_by", "delete_reason", "cap_nhat_luc",
			// The three the card does not draw. They are asserted here because the write path and a
			// later detail route will read them: a column misspelled today is cheapest to find today.
			"noi_dung", "thanh_phan", "dinh_kem",
		)},
		{"ket_luan_hop", []string{
			"tenant_id", "id", "bien_ban_id", "thu_tu", "noi_dung",
			"deleted_at", "deleted_by", "delete_reason", "tao_luc", "cap_nhat_luc",
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

// TestPgNgayHopLaKieuDate is the divergence from 0006 stated as an assertion rather than as a
// comment. A meeting day is a calendar fact; storing it as an instant would make "midnight in which
// timezone" decide which DAY the card shows, and the response formats it as `2006-01-02`.
func TestPgNgayHopLaKieuDate(t *testing.T) {
	db := moKetNoi(t)

	var kieu string
	err := db.QueryRow(
		`SELECT data_type FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'bien_ban_hop'
		   AND column_name = 'ngay_hop'`).Scan(&kieu)
	if err != nil {
		t.Fatalf("tra kiểu cột: %v", err)
	}
	if kieu != "date" {
		t.Errorf("ngay_hop kiểu %q, muốn `date`", kieu)
	}
}

// TestPgBienBanKhongXoaCungDuoc — rule 7, forbidden #1, enforced by `ho_so_luu_tru_cam_xoa_cung`
// rather than promised by a comment. §7.1 says a conclusion that has produced tasks is never hard
// deleted, because the tasks must stay traceable to it (§1).
func TestPgBienBanKhongXoaCungDuoc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if err := themBienBan(db, xa, "bb-xoa", "Giao ban tháng 8", mocNgayHopPg); err != nil {
		t.Fatalf("thêm biên bản: %v", err)
	}
	if err := themKetLuan(db, xa, "kl-xoa", "bb-xoa", 1, "Một kết luận."); err != nil {
		t.Fatalf("thêm kết luận: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM ket_luan_hop WHERE tenant_id = $1 AND id = $2`,
		xa, "kl-xoa"); err == nil {
		t.Error("XOÁ CỨNG ĐƯỢC một kết luận — nhiệm vụ tách từ nó mất đường truy vết về gốc")
	}
	if _, err := db.Exec(`DELETE FROM bien_ban_hop WHERE tenant_id = $1 AND id = $2`,
		xa, "bb-xoa"); err == nil {
		t.Error("XOÁ CỨNG ĐƯỢC một biên bản — hồ sơ lưu trữ bị huỷ bằng một câu lệnh")
	}

	// THE SOFT DELETE IS THE WAY OUT, and it needs all three columns (rule 7, invariant 1).
	if _, err := db.Exec(
		`UPDATE ket_luan_hop SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		xa, "kl-xoa", "CB-00123", "Ghi nhầm sang biên bản khác."); err != nil {
		t.Errorf("xoá mềm đủ ba cột vẫn bị từ chối: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE ket_luan_hop SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "kl-xoa"); err == nil {
		t.Error("xoá mềm THIẾU người xoá và lý do vẫn được ghi — một lần gỡ không ai chịu trách nhiệm")
	}
}

// TestPgThuTuKetLuanDuyNhatTrongMotBienBanVaKHONGTraLaiSauKhiXoaMem is §7.2 and rule 7, invariant 3
// in one case.
//
// THE SECOND HALF IS THE EXPENSIVE ONE. A partial unique key (`WHERE deleted_at IS NULL`) would let
// a commune remove conclusion ② and mint a SECOND ② — while the printed minutes still say "②" and
// the first one's tasks still point at it. tools/check_khoa_duy_nhat.py refuses that shape; this
// asserts the database really behaves the way that refusal assumes.
func TestPgThuTuKetLuanDuyNhatTrongMotBienBanVaKHONGTraLaiSauKhiXoaMem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if err := themBienBan(db, xa, "bb-tt", "Giao ban tháng 8", mocNgayHopPg); err != nil {
		t.Fatalf("thêm biên bản: %v", err)
	}
	if err := themKetLuan(db, xa, "kl-1", "bb-tt", 1, "Kết luận thứ nhất."); err != nil {
		t.Fatalf("thêm kết luận: %v", err)
	}
	if err := themKetLuan(db, xa, "kl-2", "bb-tt", 1, "Trùng số thứ tự."); err == nil {
		t.Error("hai kết luận cùng số ① trong một biên bản — ô tròn trên màn hình không còn phân biệt được")
	}

	if _, err := db.Exec(
		`UPDATE ket_luan_hop SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		xa, "kl-1", "CB-00123", "Gỡ khỏi biên bản."); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}
	if err := themKetLuan(db, xa, "kl-3", "bb-tt", 1, "Số ① cấp lại sau khi xoá mềm."); err == nil {
		t.Error("SỐ THỨ TỰ ĐÃ CẤP ĐƯỢC CẤP LẠI sau một lần xoá mềm (luật 7 bất biến 3)")
	}

	// A SECOND MEETING STARTS AT ① AGAIN — the key is per meeting, not per commune.
	if err := themBienBan(db, xa, "bb-tt2", "Giao ban tháng 9", mocNgayHopPg); err != nil {
		t.Fatalf("thêm biên bản thứ hai: %v", err)
	}
	if err := themKetLuan(db, xa, "kl-4", "bb-tt2", 1, "Kết luận ① của biên bản khác."); err != nil {
		t.Errorf("biên bản thứ hai không bắt đầu lại từ ①: %v", err)
	}
}

// TestPgHaiXaDungChungSoThuTuVaChungIdBienBan — rule 1, invariant 6. The unique key is composite
// with `tenant_id`, so the second commune onboards; and the foreign key is composite too, so a
// conclusion cannot attach to another commune's meeting even when the ids match exactly.
func TestPgHaiXaDungChungSoThuTuVaChungIdBienBan(t *testing.T) {
	db := moKetNoi(t)
	xa1, xa2 := xaRieng(t)

	for _, xa := range []string{xa1, xa2} {
		if err := themBienBan(db, xa, "bb-chung", "Giao ban tháng 8", mocNgayHopPg); err != nil {
			t.Fatalf("xã %s không thêm được biên bản cùng id: %v", xa, err)
		}
		if err := themKetLuan(db, xa, "kl-chung", "bb-chung", 1, "Kết luận ① của xã này."); err != nil {
			t.Fatalf("xã %s không thêm được kết luận cùng số: %v", xa, err)
		}
	}

	// The commune boundary on the FOREIGN KEY: commune 2's conclusion may not hang off commune 1's
	// meeting, and the only thing stopping it is `tenant_id` being part of the key.
	if err := themKetLuan(db, xa2, "kl-lac", "bb-khong-co", 2, "Biên bản không tồn tại."); err == nil {
		t.Error("kết luận gắn được vào một biên bản không tồn tại")
	}
}

// TestPgCauDemNhiemVuChayThatVaDemDungLiveRows is the ONE test that runs the badge query against a
// real server. The fake driver cannot execute a LATERAL join, cannot aggregate, and would happily
// accept a statement PostgreSQL rejects.
//
// IT ALSO ASSERTS THE THREE WAYS THE COUNT MUST BE WRONG AND IS NOT: a soft-deleted task leaves both
// halves; another commune's task never enters; and a conclusion nobody has split comes back as `0/0`
// rather than vanishing from the card.
func TestPgCauDemNhiemVuChayThatVaDemDungLiveRows(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	danhMucChoXa(t, db, xa)
	danhMucChoXa(t, db, xaKhac)

	if err := themBienBan(db, xa, "bb-dem", "Giao ban tháng 8", mocNgayHopPg); err != nil {
		t.Fatalf("thêm biên bản: %v", err)
	}
	for i, noi := range []string{"Kết luận có việc.", "Kết luận chưa tách."} {
		if err := themKetLuan(db, xa, fmt.Sprintf("kl-dem-%d", i+1), "bb-dem", i+1, noi); err != nil {
			t.Fatalf("thêm kết luận: %v", err)
		}
	}

	// THREE TASKS ON CONCLUSION ①: one finished, one running, one soft-deleted afterwards.
	themNhiemVuTuKetLuan(t, db, xa, "nv-1", "NV01", "hoan-thanh", "kl-dem-1")
	themNhiemVuTuKetLuan(t, db, xa, "nv-2", "NV02", "dang-thuc-hien", "kl-dem-1")
	themNhiemVuTuKetLuan(t, db, xa, "nv-3", "NV03", "hoan-thanh", "kl-dem-1")
	if _, err := db.Exec(
		`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-3", "CB-00123", "Giao nhầm."); err != nil {
		t.Fatalf("xoá mềm nhiệm vụ: %v", err)
	}
	// ANOTHER COMMUNE'S TASK POINTING AT THE SAME CONCLUSION ID. It must not be counted here, and
	// only `nv.tenant_id = $1` inside the join stops it.
	themNhiemVuTuKetLuan(t, db, xaKhac, "nv-x", "NV01", "hoan-thanh", "kl-dem-1")

	s := NewBienBanHopStore(pkgstore.New(db))
	theo, err := s.ketLuanTheoBienBan(ctxXa(tenant.ID(xa)), []string{"bb-dem"})
	if err != nil {
		t.Fatalf("đọc kết luận kèm bộ đếm: %v", err)
	}
	ds := theo["bb-dem"]
	if len(ds) != 2 {
		t.Fatalf("đọc %d kết luận, muốn 2 — kết luận chưa tách nhiệm vụ nào VẪN phải có mặt", len(ds))
	}
	if ds[0].ThuTu != 1 || ds[1].ThuTu != 2 {
		t.Errorf("kết luận không theo thứ tự ô tròn: %d, %d", ds[0].ThuTu, ds[1].ThuTu)
	}
	if ds[0].SoNhiemVu != 2 || ds[0].SoNhiemVuXong != 1 {
		t.Errorf("kết luận ① đếm %d/%d, muốn 1/2 (một đã xoá mềm, một của xã khác — cả hai phải ngoài)",
			ds[0].SoNhiemVuXong, ds[0].SoNhiemVu)
	}
	if ds[1].SoNhiemVu != 0 || ds[1].SoNhiemVuXong != 0 {
		t.Errorf("kết luận ② đếm %d/%d, muốn 0/0", ds[1].SoNhiemVuXong, ds[1].SoNhiemVu)
	}
}

// themNhiemVuTuKetLuan books a task BORN FROM A MEETING CONCLUSION — the blurred pair of 0006 with
// `nguon_giao = 'ket-luan-hop'`, which is the whole link this migration relies on and adds no column
// for.
func themNhiemVuTuKetLuan(t *testing.T, db *sql.DB, xa, id, ma, trangThai, ketLuanID string) {
	t.Helper()
	var xong any
	if trangThai == "hoan-thanh" {
		xong = time.Now()
	}
	_, err := db.Exec(
		`INSERT INTO nhiem_vu (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao, nguon_id,
		                       ngay_hoan_thanh, tien_do, nguoi_tao_ma)
		 VALUES ($1,$2,$3,$4,$5,$6,'ket-luan-hop',$7,$8,0,'CB-00123')`,
		xa, id, ma, "theo-van-ban", "Việc tách từ kết luận họp.", trangThai, ketLuanID, xong)
	if err != nil {
		t.Fatalf("thêm nhiệm vụ %s: %v", id, err)
	}
}

// TestPgChiMucNguonNhiemVuTonTai — the index 0007 puts on `nhiem_vu` is what keeps the badge from
// scanning the commune's whole register once per conclusion. Its absence costs nothing today (every
// register is empty) and is unaffordable to notice later, so it is asserted by name.
func TestPgChiMucNguonNhiemVuTonTai(t *testing.T) {
	db := moKetNoi(t)

	for _, ten := range []string{"nhiem_vu_nguon", "ket_luan_theo_bien_ban", "bien_ban_hop_so"} {
		var co bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM pg_indexes
			 WHERE schemaname = current_schema() AND indexname = $1)`, ten).Scan(&co)
		if err != nil {
			t.Fatalf("tra pg_indexes: %v", err)
		}
		if !co {
			t.Errorf("thiếu chỉ mục %q", ten)
		}
	}
}

// TestPgPhanManhCuaHaiBangDuyDu — a partitioned table with no partitions REJECTS EVERY INSERT,
// silently, until the first real write. The migration's own BACKSTOP block checks this at apply
// time; this checks it from the outside, against the number ADR 0010 fixes.
func TestPgPhanManhCuaHaiBangDuyDu(t *testing.T) {
	db := moKetNoi(t)

	for _, bang := range []string{"bien_ban_hop", "ket_luan_hop"} {
		var n int
		err := db.QueryRow(
			`SELECT count(*) FROM pg_inherits WHERE inhparent = $1::regclass`, bang).Scan(&n)
		if err != nil {
			t.Fatalf("đếm phân mảnh của %s: %v", bang, err)
		}
		if n != 32 {
			t.Errorf("bảng %s có %d phân mảnh, muốn 32 (ADR 0010)", bang, n)
		}
	}
}

// TestPgTruongBatBuocCuaBienBan — the two CHECKs that keep an unreadable card out of the register.
func TestPgTruongBatBuocCuaBienBan(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if err := themBienBan(db, xa, "bb-rong", "   ", mocNgayHopPg); err == nil {
		t.Error("lưu được biên bản có tên toàn khoảng trắng — thẻ trên màn hình không ai nhận ra")
	}
	if err := themBienBan(db, xa, "bb-khong-ngay", "Giao ban", nil); err == nil {
		t.Error("lưu được biên bản không có ngày họp")
	}
	if err := themBienBan(db, xa, "bb-ok", "Giao ban", mocNgayHopPg); err != nil {
		t.Fatalf("biên bản hợp lệ bị từ chối: %v", err)
	}
	// §7.3: minutes with NO conclusions are saved. Nothing above requires one, and this says so.
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM ket_luan_hop WHERE tenant_id = $1 AND bien_ban_id = $2`,
		xa, "bb-ok").Scan(&n); err != nil {
		t.Fatalf("đếm kết luận: %v", err)
	}
	if n != 0 {
		t.Errorf("biên bản mới có sẵn %d kết luận", n)
	}

	if err := themKetLuan(db, xa, "kl-0", "bb-ok", 0, "Số thứ tự 0."); err == nil {
		t.Error("lưu được kết luận mang số thứ tự 0 — §7.2 đánh số từ 1")
	}
	if err := themKetLuan(db, xa, "kl-rong", "bb-ok", 1, "   "); err == nil {
		t.Error("lưu được kết luận rỗng — không ai tách nó thành nhiệm vụ được")
	}
}
