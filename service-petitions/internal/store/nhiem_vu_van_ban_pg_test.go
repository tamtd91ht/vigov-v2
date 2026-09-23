package store

import (
	"database/sql"
	"testing"
	"time"
)

// Integration tests for migration 0009 — §5.4's document block.
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: every test here SKIPS unless VIGOV_TEST_DSN is set, and
// the package still prints `ok`. NO PostgreSQL IS REACHABLE FROM THE ENVIRONMENT THIS WAS WRITTEN IN
// — the DSN is unset and Docker is off — so NONE of the assertions below have ever been executed.
// `ok` next to this package means the fake-driver suites ran; it does not mean the SQL here was
// verified. The schema harness (TestMain, moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go.
//
// WHY THEY ARE WORTH WRITING WHILE THEY CANNOT RUN: every property below is enforced by the DATABASE
// and by nothing else, so no fake-driver suite can see any of it. Each is also a rule that, broken,
// produces NO error at all until a real commune is running:
//
//	a reissued `thu_tu`            a line's position taken from one that was removed, so the printed
//	                              order of a block stops matching what the register holds
//	a hard removal                 part of an administrative record destroyed by one statement
//	a `nhom` outside the three     a line filed under a box no screen draws — invisible for ever
//	`ngay_van_ban` as an instant   "midnight in which timezone" decides which DAY a document is dated
//	a line on another commune's    the breach between two public authorities this system exists to
//	task                          prevent (rule 1)

// themVanBanNhiemVu inserts one document line and returns the error UNTOUCHED, so a test can assert
// on the constraint that refused it.
func themVanBanNhiemVu(db *sql.DB, xa, id, nhiemVuID, nhom, trichYeu string, thuTu int,
	soKyHieu, ngayVanBan any) error {

	_, err := db.Exec(
		`INSERT INTO nhiem_vu_van_ban
		 (tenant_id, id, nhiem_vu_id, nhom, so_ky_hieu, ngay_van_ban, trich_yeu, thu_tu)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		xa, id, nhiemVuID, nhom, soKyHieu, ngayVanBan, trichYeu, thuTu)
	return err
}

// nhiemVuChoVanBan sows the one task every line below hangs off, catalogue included.
func nhiemVuChoVanBan(t *testing.T, db *sql.DB, xa, id, ma string) {
	t.Helper()
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, id, ma, "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
}

var mocNgayVanBanPg = time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

// TestPgCotVanBanNhiemVuTrongMaKhopVoiLuocDoThat catches a later migration renaming a column out
// from under this store.
//
// `cotVanBanNhiemVu` is a string and the fake driver builds its rows from that same string, so a
// column misspelled from the start is invisible to every test that does not touch a real schema. It
// surfaces as a 500 on a member of staff's screen with "column … does not exist" in a log nobody is
// reading yet.
func TestPgCotVanBanNhiemVuTrongMaKhopVoiLuocDoThat(t *testing.T) {
	db := moKetNoi(t)

	for _, c := range append(tachCot(cotVanBanNhiemVu),
		// COLUMNS THAT NEVER APPEAR IN A SELECT LIST, because the statements are built around them.
		"tenant_id",    // the predicate that keeps two public authorities apart (rule 1)
		"deleted_at",   // the predicate that keeps removed lines out of every read (rule 7)
		"deleted_by",   // who removed it, as a STAFF BUSINESS CODE (rule 6, invariant 8)
		"cap_nhat_luc", //
	) {
		var co bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			 WHERE table_schema = current_schema() AND table_name = 'nhiem_vu_van_ban'
			   AND column_name = $1)`, c).Scan(&co)
		if err != nil {
			t.Fatalf("tra information_schema: %v", err)
		}
		if !co {
			t.Errorf("bảng nhiem_vu_van_ban không có cột %q mà mã đang đọc", c)
		}
	}
}

// TestPgKhongCoCotDeleteReason states an ABSENCE as an assertion, so the decision cannot be undone by
// accident.
//
// Migration 0009 leaves the column out deliberately: removing a line is an EDIT of the task, and the
// reason for an edit lives on the audit entry of the act, not on each line the act touched. Adding
// the column later would be a decision, and it would need the write path to have a reason to put in
// it — which §7.2's `✕` does not collect.
func TestPgKhongCoCotDeleteReason(t *testing.T) {
	db := moKetNoi(t)

	var co bool
	err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'nhiem_vu_van_ban'
		   AND column_name = 'delete_reason')`).Scan(&co)
	if err != nil {
		t.Fatalf("tra information_schema: %v", err)
	}
	if co {
		t.Error("bảng có cột `delete_reason` — 0009 cố ý không tạo nó, và một cột không ai điền nổi " +
			"là một cột sẽ được điền bằng chữ máy tự bịa")
	}
}

// TestPgNgayVanBanLaKieuDate is the divergence from 0006's two deadline columns stated as an
// assertion rather than as a comment.
//
// A deadline is an INSTANT counted in working hours (ADR 0007). A document's date is the day printed
// on paper: storing it as an instant would make "midnight in which timezone" decide which DAY a
// document is dated, and the response formats it as `2006-01-02`.
func TestPgNgayVanBanLaKieuDate(t *testing.T) {
	db := moKetNoi(t)

	var kieu string
	err := db.QueryRow(
		`SELECT data_type FROM information_schema.columns
		 WHERE table_schema = current_schema() AND table_name = 'nhiem_vu_van_ban'
		   AND column_name = 'ngay_van_ban'`).Scan(&kieu)
	if err != nil {
		t.Fatalf("tra kiểu cột: %v", err)
	}
	if kieu != "date" {
		t.Errorf("ngay_van_ban là %q, muốn `date`", kieu)
	}
}

// TestPgThuTuVanBanKhongCapLai IS THE MOST VALUABLE ASSERTION IN THIS FILE.
//
// `UNIQUE (tenant_id, nhiem_vu_id, nhom, thu_tu)` carries NO `WHERE deleted_at IS NULL`, so a removed
// line keeps its number for ever. That is what makes "renumber the block by array position" fail
// loudly instead of silently reissuing a position — and `tools/check_khoa_duy_nhat.py` refuses the
// partial form for exactly this reason.
func TestPgThuTuVanBanKhongCapLai(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-1", "NV01")

	if err := themVanBanNhiemVu(db, xa, "vb-1", "nv-vb-1", "cap-tren-giao",
		"Công văn của Ban Tổ chức Thành uỷ", 1, "1742-CV/BTCTU", mocNgayVanBanPg); err != nil {
		t.Fatalf("thêm dòng văn bản: %v", err)
	}

	// The clerk removes it. SOFT — two columns, together.
	if _, err := db.Exec(
		`UPDATE nhiem_vu_van_ban SET deleted_at = now(), deleted_by = 'CB-00123'
		 WHERE tenant_id = $1 AND id = $2`, xa, "vb-1"); err != nil {
		t.Fatalf("gỡ mềm bị chặn: %v", err)
	}

	// A SECOND LINE AT THE SAME POSITION IS REFUSED, although the first is gone from every screen.
	if err := themVanBanNhiemVu(db, xa, "vb-2", "nv-vb-1", "cap-tren-giao",
		"Một văn bản khác", 1, nil, nil); err == nil {
		t.Fatal("cấp lại được số thứ tự 1 sau khi gỡ mềm — đường ghi sẽ đánh số lại theo vị trí mảng " +
			"mà không có gì đỏ (luật 7 bất biến 3)")
	}

	// AND THE NEXT POSITION IS FREE, so the gap is a gap and not a wall.
	if err := themVanBanNhiemVu(db, xa, "vb-3", "nv-vb-1", "cap-tren-giao",
		"Một văn bản khác", 2, nil, nil); err != nil {
		t.Fatalf("số thứ tự 2 bị từ chối: %v", err)
	}
}

// TestPgThuTuVanBanRiengTungNhom — the three lists number INDEPENDENTLY. A shared counter would make
// `san-pham-dau-ra` start at 4 because `cap-tren-giao` already holds three lines, and §5.4 draws each
// box from ①.
func TestPgThuTuVanBanRiengTungNhom(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-2", "NV02")

	if err := themVanBanNhiemVu(db, xa, "vb-a", "nv-vb-2", "cap-tren-giao",
		"Công văn cấp trên", 1, nil, nil); err != nil {
		t.Fatalf("thêm dòng nhóm một: %v", err)
	}
	if err := themVanBanNhiemVu(db, xa, "vb-b", "nv-vb-2", "san-pham-dau-ra",
		"Báo cáo đầu ra", 1, nil, nil); err != nil {
		t.Fatalf("số thứ tự 1 của nhóm THỨ HAI bị từ chối: %v — ba danh sách đánh số độc lập", err)
	}
}

// TestPgKhongXoaCungDuocDongVanBan — rule 7, forbidden #1, enforced by the trigger of migration 0009
// rather than by convention. A promise the application layer makes is a promise a psql session never
// heard (ADR 0013).
func TestPgKhongXoaCungDuocDongVanBan(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-3", "NV03")

	if err := themVanBanNhiemVu(db, xa, "vb-c", "nv-vb-3", "chi-dao-dang-uy",
		"Công văn 416-CV/ĐU", 1, nil, nil); err != nil {
		t.Fatalf("thêm dòng văn bản: %v", err)
	}

	if _, err := db.Exec(
		`DELETE FROM nhiem_vu_van_ban WHERE tenant_id = $1 AND id = $2`, xa, "vb-c"); err == nil {
		t.Fatal("xoá cứng được một dòng văn bản — nó là một phần của hồ sơ hành chính (luật 7)")
	}
}

// TestPgNhomVanBanNgoaiBaGiaTriBiTuChoi — the CHECK is the floor under domain.NhomVanBanNhiemVu.
// A line filed under a group no box draws is a line nobody ever sees again, on a table nothing hard
// deletes.
func TestPgNhomVanBanNgoaiBaGiaTriBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-4", "NV04")

	if err := themVanBanNhiemVu(db, xa, "vb-d", "nv-vb-4", "van-ban-den",
		"Một văn bản", 1, nil, nil); err == nil {
		t.Fatal("nhận được `nhom = van-ban-den` — danh sách ba nhóm là ĐÓNG (§5.4, §7.2)")
	}
}

// TestPgTrichYeuRongBiTuChoi — a blank line in a dynamic list is a row nobody can read and nobody can
// act on, and `✕` is how a line is removed.
func TestPgTrichYeuRongBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-5", "NV05")

	if err := themVanBanNhiemVu(db, xa, "vb-e", "nv-vb-5", "cap-tren-giao",
		"   \t ", 1, nil, nil); err == nil {
		t.Fatal("nhận được một dòng chỉ có khoảng trắng")
	}
}

// TestPgThuTuVanBanTuMot — ① is the first number. A zero or a negative ordinal renders as nothing and
// breaks the `max + 1` the write path mints with.
func TestPgThuTuVanBanTuMot(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-6", "NV06")

	if err := themVanBanNhiemVu(db, xa, "vb-f", "nv-vb-6", "cap-tren-giao",
		"Một văn bản", 0, nil, nil); err == nil {
		t.Fatal("nhận được thu_tu = 0")
	}
}

// TestPgXoaMemVanBanPhaiDayDu — `deleted_at` without `deleted_by` is a line that vanished from every
// screen with nobody answerable for it (rule 6, invariant 8).
func TestPgXoaMemVanBanPhaiDayDu(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-vb-7", "NV07")

	if err := themVanBanNhiemVu(db, xa, "vb-g", "nv-vb-7", "cap-tren-giao",
		"Một văn bản", 1, nil, nil); err != nil {
		t.Fatalf("thêm dòng văn bản: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE nhiem_vu_van_ban SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "vb-g"); err == nil {
		t.Fatal("gỡ được một dòng mà không ghi ai gỡ")
	}
}

// TestPgDongVanBanKhongTreoDuocVaoNhiemVuXaKhac IS RULE 1 AT THE SCHEMA LEVEL.
//
// The foreign key is COMPOSITE with `tenant_id`, so even two ids that collided could not put one
// commune's document line on another commune's task. Data crossing that line is not a software bug,
// it is a breach between two public authorities.
func TestPgDongVanBanKhongTreoDuocVaoNhiemVuXaKhac(t *testing.T) {
	db := moKetNoi(t)
	xaMot, _ := xaRieng(t)
	xaHai, _ := xaRieng(t)
	nhiemVuChoVanBan(t, db, xaMot, "nv-vb-8", "NV08")
	danhMucChoXa(t, db, xaHai)

	// Commune TWO tries to hang a line off commune ONE's task, naming that task's real id.
	if err := themVanBanNhiemVu(db, xaHai, "vb-h", "nv-vb-8", "cap-tren-giao",
		"Một văn bản", 1, nil, nil); err == nil {
		t.Fatal("treo được một dòng văn bản vào nhiệm vụ của XÃ KHÁC — khoá ngoại phải hợp thành " +
			"với tenant_id (luật 1 bất biến 6)")
	}
}
