package store

import (
	"database/sql"
	"testing"
)

// Integration tests for TenTheoNhieuMa — the read behind ResolveStaffNames — against a real
// PostgreSQL, sharing the harness in checker_pg_test.go (moKetNoi, xaRieng, ctxXa, bamMauPg).
//
// WHY A REAL DATABASE, and here it is the whole point rather than a formality: EVERY property this
// file asserts lives in the SQL predicate, and a fake store would simply agree with whatever the
// query already does. The handler tests in internal/grpc cannot reach any of them.
//
//  1. `nd.deleted_at` IS NOT FILTERED. A record removed from the directory must come back, carrying
//     its name — that absence is the defect ADR 0034 exists to close, and it only shows against a
//     row that is really soft deleted.
//  2. `nd.tenant_id = $1` IS STILL THERE. A code of another commune must be ABSENT, indistinguishable
//     from "does not exist". Relaxing the deleted filter must not relax this one — a leak between two
//     communes is a leak between two public authorities whatever the query was for.
//  3. THE KEY IS `nd.ma`, NOT `nd.id`. Keyed on the internal id, this lookup resolves nothing the
//     audit trail has ever stored (rule 6, invariant 8) and renders a blank where a name belongs.
//  4. `dang_hoat_dong` IS NOT READ. A locked account is still in the directory (open question #10),
//     and only a real row carrying `dang_hoat_dong = false` proves the column is not consulted.
//
// AND ONE PROPERTY OF THE OTHER QUERY: TheoNhieuID must STILL hide a soft-deleted record. The two
// predicates live side by side in this package precisely so neither drifts into the other, and the
// last test below pins both against the SAME row.
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8).

// themNguoiCoMa inserts one staff row with an explicit code, name and lock state — the three things
// themNguoi in vai_tro_pg_test.go fixes for every row it creates, and the three these tests vary.
//
// THE NAME IS A FAKE ONE. Rule 3, forbidden #2: no real person's data in source.
func themNguoiCoMa(t *testing.T, db *sql.DB, tenantID, id, ma, hoTen string, dangHoatDong bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash, dang_hoat_dong)
		 VALUES ($1,$2,$3,$4,$5,true,$6,$7)`,
		tenantID, id, ma, hoTen, id+"@xa.danang.gov.vn", bamMauPg, dangHoatDong)
	if err != nil {
		t.Fatalf("thêm cán bộ: %v", err)
	}
}

func xoaMem(t *testing.T, db *sql.DB, tenantID, id string) {
	t.Helper()
	// SOFT DELETE, never a DELETE (rule 7). This is the state the whole file is about.
	if _, err := db.Exec(
		`UPDATE nguoi_dung SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, id); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}
}

// timTen reads one code out of what the batch query returned, or reports it absent.
func timTen(t *testing.T, db *sql.DB, xa string, hoi []string, ma string) (string, bool, bool) {
	t.Helper()
	hang, err := dungCanBoStore(db).TenTheoNhieuMa(ctxXa(xa), hoi)
	if err != nil {
		t.Fatalf("TenTheoNhieuMa: %v", err)
	}
	for _, it := range hang {
		if it.Ma == ma {
			return it.HoTen, it.ConTrongDanhBa, true
		}
	}
	return "", false, false
}

// CA BẮT BUỘC 1 — một mã ĐÃ XOÁ MỀM vẫn trả về TÊN, và nói rõ bản ghi đã bị gỡ khỏi danh bạ.
func TestPgTenTraCaNguoiDaXoaMem(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoiCoMa(t, db, xa, "nd-xoa-"+xa, "CB-XOA-"+xa[:8], "Trần Thị B", true)
	xoaMem(t, db, xa, "nd-xoa-"+xa)

	hoTen, conTrong, co := timTen(t, db, xa, []string{"CB-XOA-" + xa[:8]}, "CB-XOA-"+xa[:8])
	if !co {
		t.Fatal("MÃ ĐÃ XOÁ MỀM KHÔNG TRẢ VỀ GÌ — hồ sơ lưu trữ sẽ hiện mã trần ở chỗ tên người xử lý, " +
			"đúng khiếm khuyết ADR 0034 sinh ra để đóng")
	}
	if hoTen != "Trần Thị B" {
		t.Errorf("ho_ten = %q, muốn tên người", hoTen)
	}
	if conTrong {
		t.Error("bản ghi đã xoá mềm báo là CÒN trong danh bạ — bên gọi sẽ mời người đã bị gỡ vào ô chọn người phụ trách")
	}
}

// CA BẮT BUỘC 2 — mã của XÃ KHÁC không trả về gì, kể cả khi bản ghi ấy đã bị xoá mềm.
//
// ĐÂY LÀ CA CỦA ĐỘT BIẾN: bỏ `nd.tenant_id = $1` khỏi truy vấn mới thì test này phải ĐỎ.
func TestPgTenKhongVuotSangXaKhac(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	// The SAME code in both communes — legitimate, because `ma` is unique WITHIN a commune only
	// (UNIQUE (tenant_id, ma), migration 0001). If the commune predicate were dropped, commune A's
	// caller would read commune B's person, and the two names would be indistinguishable on screen.
	maChung := "CB-CHUNG-01"
	themNguoiCoMa(t, db, xaA, "nd-a-"+xaA, maChung, "Nguyễn Văn A", true)
	themNguoiCoMa(t, db, xaB, "nd-b-"+xaB, maChung, "Phạm Thị D", true)

	// And one code that exists ONLY in commune B, soft deleted — the shape a widened predicate would
	// leak most quietly, because the row is invisible to every other read path in this package.
	themNguoiCoMa(t, db, xaB, "nd-b2-"+xaB, "CB-CHI-CO-O-B", "Lê Văn C", true)
	xoaMem(t, db, xaB, "nd-b2-"+xaB)

	hoi := []string{maChung, "CB-CHI-CO-O-B"}

	// COUNTED, NOT JUST LOOKED UP. A predicate that dropped the commune returns BOTH rows for the
	// shared code, and a test that stops at the first match would pass on whichever row the planner
	// happened to return first — green for a reason that has nothing to do with isolation.
	hang, err := dungCanBoStore(db).TenTheoNhieuMa(ctxXa(xaA), hoi)
	if err != nil {
		t.Fatalf("TenTheoNhieuMa: %v", err)
	}
	soDongMaChung := 0
	for _, it := range hang {
		if it.Ma == maChung {
			soDongMaChung++
		}
	}
	if soDongMaChung != 1 {
		t.Fatalf("mã dùng chung trả %d dòng, muốn đúng 1 — nhiều hơn nghĩa là đã đọc sang xã khác",
			soDongMaChung)
	}

	hoTen, _, co := timTen(t, db, xaA, hoi, maChung)
	if !co {
		t.Fatal("không đọc được cán bộ của chính xã mình")
	}
	if hoTen != "Nguyễn Văn A" {
		t.Errorf("ho_ten = %q — đã đọc sang bản ghi của xã khác mang cùng mã", hoTen)
	}
	if _, _, co := timTen(t, db, xaA, hoi, "CB-CHI-CO-O-B"); co {
		t.Error("ĐỌC ĐƯỢC CÁN BỘ CỦA XÃ KHÁC — đây là rò rỉ giữa hai cơ quan nhà nước (luật 1)")
	}
}

// CA BẮT BUỘC 3 — người đang tại chức CÒN trong danh bạ, và TÀI KHOẢN BỊ KHOÁ CŨNG CÒN.
//
// Câu mở #10 (chốt 22/09/2026): nghỉ hưu / chuyển công tác là KHOÁ, người ấy vẫn hiện trong danh bạ.
// Dòng bị khoá bên dưới là thứ duy nhất chứng minh `dang_hoat_dong` KHÔNG nằm trong vị từ.
func TestPgTenTaiChucVaBiKhoaDeuConTrongDanhBa(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoiCoMa(t, db, xa, "nd-tc-"+xa, "CB-TAICHUC-1", "Nguyễn Văn A", true)
	themNguoiCoMa(t, db, xa, "nd-khoa-"+xa, "CB-BIKHOA-1", "Lê Văn C", false)

	hoi := []string{"CB-TAICHUC-1", "CB-BIKHOA-1"}

	for _, ma := range hoi {
		hoTen, conTrong, co := timTen(t, db, xa, hoi, ma)
		if !co {
			t.Fatalf("%s không trả về", ma)
		}
		if hoTen == "" {
			t.Errorf("%s: ho_ten rỗng — một dòng lịch sử không mang tên ai", ma)
		}
		if !conTrong {
			t.Errorf("%s: báo ĐÃ GỠ khỏi danh bạ — tài khoản bị KHOÁ vẫn trong danh bạ (câu mở #10)", ma)
		}
	}
}

// CA BẮT BUỘC 4 — BatchGetStaff VẪN KHÔNG trả bản ghi đã xoá mềm. Hai vị từ, một dòng: đường mới mở
// ra không được nới đường đang chạy.
func TestPgTenMoRongMaKhongNoiTruyVanCuaBatchGetStaff(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoiCoMa(t, db, xa, "nd-cu-"+xa, "CB-CA-HAI-1", "Trần Thị B", true)
	xoaMem(t, db, xa, "nd-cu-"+xa)

	if _, _, co := timTen(t, db, xa, []string{"CB-CA-HAI-1"}, "CB-CA-HAI-1"); !co {
		t.Fatal("truy vấn tên không trả bản ghi đã xoá mềm — đường mới không làm được việc của nó")
	}
	if _, co := maVaiTro(t, db, xa, []string{"nd-cu-" + xa}, "nd-cu-"+xa); co {
		t.Error("BatchGetStaff TRẢ VỀ BẢN GHI ĐÃ XOÁ MỀM — ô chọn người phụ trách sẽ mời người đã bị gỡ khỏi danh bạ")
	}
}

// Khoá tra là `ma`, KHÔNG phải id nội bộ. Hỏi bằng id nội bộ phải không ra gì — nếu ra, nghĩa là ai
// đó đã đổi cột tra và mọi dòng vết kiểm toán (lưu `ma`, luật 6 bất biến 8) sẽ phân giải ra ô trống.
func TestPgTenTraTheoMaChuKhongPhaiIdNoiBo(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoiCoMa(t, db, xa, "nd-khoa-tra-"+xa, "CB-KHOA-TRA", "Nguyễn Văn A", true)

	hang, err := dungCanBoStore(db).TenTheoNhieuMa(ctxXa(xa), []string{"nd-khoa-tra-" + xa})
	if err != nil {
		t.Fatalf("TenTheoNhieuMa: %v", err)
	}
	if len(hang) != 0 {
		t.Fatalf("tra bằng id nội bộ lại ra %d dòng — khoá tra phải là `ma`", len(hang))
	}

	if _, _, co := timTen(t, db, xa, []string{"CB-KHOA-TRA"}, "CB-KHOA-TRA"); !co {
		t.Error("tra bằng `ma` không ra gì")
	}
}

// Danh sách rỗng đọc KHÔNG GÌ CẢ và trả rỗng — không bao giờ là "tất cả" (ADR 0034, tính chất 2).
func TestPgTenDanhSachRongTraRong(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguoiCoMa(t, db, xa, "nd-khong-hoi-"+xa, "CB-KHONG-HOI", "Nguyễn Văn A", true)

	hang, err := dungCanBoStore(db).TenTheoNhieuMa(ctxXa(xa), nil)
	if err != nil {
		t.Fatalf("TenTheoNhieuMa: %v", err)
	}
	if len(hang) != 0 {
		t.Fatalf("danh sách rỗng trả %d dòng — DANH SÁCH RỖNG KHÔNG BAO GIỜ NGHĨA LÀ 'TẤT CẢ'", len(hang))
	}
}

// Một mã không khớp gì là câu trả lời RỖNG, không phải lỗi: bên gọi ánh xạ theo `ma` và vẽ thứ nó
// nhận được. "Ít mục hơn số mã đã hỏi" là trường hợp bình thường.
func TestPgTenKhongKhopGiThiRong(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	hang, err := dungCanBoStore(db).TenTheoNhieuMa(ctxXa(xa), []string{"CB-KHONG-TON-TAI"})
	if err != nil {
		t.Fatalf("TenTheoNhieuMa: %v", err)
	}
	if len(hang) != 0 {
		t.Errorf("số dòng = %d, muốn 0", len(hang))
	}
}
