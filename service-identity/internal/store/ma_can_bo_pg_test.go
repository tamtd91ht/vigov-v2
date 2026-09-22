package store

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The half of `nguoi_dung`'s schema that only a real PostgreSQL can show, added by the decisions
// of 2026-09-22 (open questions #9, #15, #16 — migration 0009).
//
// WHY A REAL DATABASE, and here it is the whole point rather than a formality: every property
// below lives in a constraint or a default, not in Go. A fake driver would agree with whatever
// the test asserted and prove nothing — which is exactly how three of this package's own fixtures
// spent weeks inserting an account with no password and nobody noticed.
//
//	§4 of 0009  a code, once issued, is never issued again — INCLUDING after a soft delete
//	            (rule 7, invariant 3). The mechanism is `UNIQUE (tenant_id, ma)` carrying NO
//	            `WHERE deleted_at IS NULL`, and only the server can be asked whether it does.
//	§4 of 0009  the same key is COMPOSITE, so one commune's codes never constrain another's
//	            (rule 1, invariant 6).
//	§3 of 0009  `co_tai_khoan` implies a non-empty `mat_khau_hash`.
//	§1 of 0009  `phai_doi_mat_khau` defaults to TRUE — fail closed.
//	§2 of 0009  `di_dong_ca_nhan` exists, separately from `dien_thoai_co_quan`.
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8). The shared harness is in checker_pg_test.go (moKetNoi, xaRieng, ctxXa).

// themCanBoMa inserts one staff row with a chosen code, and RETURNS THE ERROR rather than failing
// the test: every case in this file is about which inserts the server refuses.
func themCanBoMa(db *sql.DB, xa, id, ma string, daXoa bool) error {
	xoa := "NULL"
	if daXoa {
		xoa = "now()"
	}
	_, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash, deleted_at)
		 VALUES ($1,$2,$3,$4,$5,true,$6,`+xoa+`)`,
		xa, id, ma, "Nguyễn Văn A", id+"@xa.danang.gov.vn", bamMauPg)
	return err
}

// THE PROPERTY RULE 7 INVARIANT 3 IS MADE OF. A staff member leaves, their row is soft-deleted —
// kept, because the audit trail points at it — and a new arrival must NOT be able to receive the
// code that now names somebody else in years of administrative records.
//
// THE ONE-LINE CHANGE THIS CATCHES: adding `WHERE deleted_at IS NULL` to `UNIQUE (tenant_id, ma)`
// so it matches the two partial indexes beside it. That edit looks like tidying, has no symptom
// on the day it is made, and surfaces only when one code appears against two different people in
// a record nobody may rewrite.
func TestPgMaCuaNguoiDaXoaMemKhongCapLaiDuoc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ma := "CB-2026-" + xa[len(xa)-6:]

	if err := themCanBoMa(db, xa, "cu-"+xa, ma, true); err != nil {
		t.Fatalf("thêm người đã xoá mềm: %v", err)
	}

	err := themCanBoMa(db, xa, "moi-"+xa, ma, false)
	if err == nil {
		t.Fatal("MÃ CỦA NGƯỜI ĐÃ XOÁ MỀM ĐƯỢC CẤP LẠI — luật 7 bất biến 3 vỡ. " +
			"Kiểm UNIQUE (tenant_id, ma): nó KHÔNG được mang `WHERE deleted_at IS NULL`")
	}
	// The refusal must be the unique key, not something incidental — a NOT NULL violation would
	// also be non-nil and would pass a test that only checked for an error.
	if !strings.Contains(strings.ToLower(err.Error()), "nguoi_dung_tenant_id_ma_key") &&
		!strings.Contains(strings.ToLower(err.Error()), "unique") &&
		!strings.Contains(err.Error(), "23505") {
		t.Fatalf("bị từ chối, nhưng không phải vì khoá duy nhất: %v", err)
	}
}

// The other half of the same key: it is COMPOSITE WITH tenant_id (rule 1, invariant 6). Two
// communes minting codes independently WILL collide sooner or later — the generator is random and
// knows nothing about other communes — and that must be an ordinary, silent non-event. A
// single-column unique key on `ma` would make the second commune unable to onboard, which is
// rule 1, forbidden #4.
func TestPgCungMotMaODuocOHaiXaKhacNhau(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	ma := "CB-2026-" + xaA[len(xaA)-6:]

	if err := themCanBoMa(db, xaA, "a-"+xaA, ma, false); err != nil {
		t.Fatalf("xã A: %v", err)
	}
	if err := themCanBoMa(db, xaB, "b-"+xaB, ma, false); err != nil {
		t.Fatalf("XÃ THỨ HAI KHÔNG DÙNG ĐƯỢC CÙNG MỘT MÃ: %v — "+
			"khoá duy nhất đang thiếu tenant_id, xã thứ hai sẽ không onboard được", err)
	}
}

// A generated code is accepted as it comes out of the generator. Cheap, and it is the only place
// the two halves of §4 meet: the value domain.SinhMaCanBo produces and the column that stores it.
func TestPgMaSinhRaLuuDuocVaHaiLanSinhKhongDungNhau(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	a, err := domain.SinhMaCanBo(time.Now())
	if err != nil {
		t.Fatalf("sinh mã lần 1: %v", err)
	}
	b, err := domain.SinhMaCanBo(time.Now())
	if err != nil {
		t.Fatalf("sinh mã lần 2: %v", err)
	}
	if a == b {
		t.Fatalf("hai lần sinh ra cùng một mã %q", a)
	}

	if err := themCanBoMa(db, xa, "sinh-a-"+xa, a, false); err != nil {
		t.Fatalf("lưu mã sinh ra lần 1: %v", err)
	}
	if err := themCanBoMa(db, xa, "sinh-b-"+xa, b, false); err != nil {
		t.Fatalf("lưu mã sinh ra lần 2: %v", err)
	}
}

// THE CONSTRAINT QUESTION #9 UNBLOCKED (migration 0009 §3). Both directions are asserted, because
// a constraint that refuses everything would pass the first case on its own.
//
// THE MUTATION THAT MUST TURN THIS RED: delete the ADD CONSTRAINT block from 0009 §3.
func TestPgTaiKhoanKhongCoMatKhauBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	_, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash)
		 VALUES ($1,$2,$3,$4,$5,true,'')`,
		xa, "rong-"+xa, "CB-2026-RONG01", "Nguyễn Văn A", "rong@xa.danang.gov.vn")
	if err == nil {
		t.Fatal("MỘT TÀI KHOẢN KHÔNG CÓ MẬT KHẨU ĐƯỢC GHI VÀO — ràng buộc " +
			"nguoi_dung_co_tai_khoan_co_mat_khau (migration 0009 §3) không còn hiệu lực")
	}
	if !strings.Contains(err.Error(), "nguoi_dung_co_tai_khoan_co_mat_khau") &&
		!strings.Contains(err.Error(), "23514") {
		t.Fatalf("bị từ chối, nhưng không phải vì ràng buộc ấy: %v", err)
	}
}

// The other direction, and it is not symmetry for its own sake: ONE `nguoi_dung` table holds both
// the 26 people of the public directory and the accounts that sign in (migration 0003). A
// directory-only person has no password and never will. A constraint that refused them would
// empty the screen the directory exists for.
func TestPgDongDanhBaKhongCoMatKhauVanGhiDuoc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	_, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash)
		 VALUES ($1,$2,$3,$4,$5,false,'')`,
		xa, "danh-ba-"+xa, "CB-2026-DBA001", "Nguyễn Văn A", "danhba@xa.danang.gov.vn")
	if err != nil {
		t.Fatalf("MỘT NGƯỜI CHỈ CÓ TRONG DANH BẠ BỊ TỪ CHỐI: %v — "+
			"ràng buộc đang chặn cả 26 người của danh bạ, không chỉ tài khoản", err)
	}
}

// phai_doi_mat_khau DEFAULTS TO TRUE (migration 0009 §1), and the direction is the property.
//
// A write path that forgets the column must land on "this password is not yet the person's own",
// not on "carry on". With DEFAULT false, an administrator-minted account would keep a password a
// second person knows, for good, with nothing reporting it (#9, rule 6 invariant 2). This case
// turns red the moment somebody "tidies" the default to false.
func TestPgPhaiDoiMatKhauMacDinhLaTrue(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if err := themCanBoMa(db, xa, "md-"+xa, "CB-2026-MD0001", false); err != nil {
		t.Fatalf("thêm cán bộ: %v", err)
	}

	var phaiDoi bool
	var diDongCaNhan string
	err := db.QueryRow(
		`SELECT phai_doi_mat_khau, di_dong_ca_nhan FROM nguoi_dung WHERE tenant_id = $1 AND id = $2`,
		xa, "md-"+xa).Scan(&phaiDoi, &diDongCaNhan)
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	if !phaiDoi {
		t.Error("phai_doi_mat_khau mặc định FALSE — một tài khoản do quản trị viên tạo sẽ " +
			"giữ mãi mật khẩu người khác đặt (câu #9, luật 6 bất biến 2)")
	}
	// di_dong_ca_nhan exists and is '' rather than NULL — one spelling of "no number", so no query has to
	// handle two (migration 0009 §2).
	if diDongCaNhan != "" {
		t.Errorf("di_dong_ca_nhan mặc định %q, muốn chuỗi rỗng", diDongCaNhan)
	}
}
