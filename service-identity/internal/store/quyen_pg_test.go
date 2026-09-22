package store

import (
	"database/sql"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for the Phân quyền matrix against a real PostgreSQL, sharing the harness in
// checker_pg_test.go (moKetNoi, xaRieng, ctxXa) and the row helpers in vai_tro_pg_test.go
// (themVaiTro, themNguoi).
//
// WHY A REAL DATABASE, and here it is not a formality — FOUR of these properties exist only in the
// SQL and a fake would simply agree with whatever the query already does:
//
//  1. count(nd.id) AND NOT count(*), and the second count's `FILTER` staying ON THE AGGREGATE. A
//     LEFT JOIN yields one all-NULL row for a role nobody holds; count(*) counts it and answers 1.
//     A FILTER moved into the ON clause narrows both counts; moved into the WHERE it deletes the
//     role from the matrix. All three are invisible from the Go side — see TestPgMaTranHaiSoDem.
//  2. `nd.deleted_at IS NULL` in the JOIN, not the WHERE. In the WHERE, a role whose only holder was
//     removed disappears from the matrix entirely — and a role missing from the matrix cannot be
//     granted anything.
//  3. The commune predicate on every join. Joining on role id alone would count another commune's
//     staff wherever ids collide (rule 1), which no single-commune test can show.
//  4. That the catalogue query — the one with no tenant filter, because `quyen` has no tenant_id —
//     is accepted by PostgreSQL at all. `WHERE $1::text <> ''` consumes the commune that
//     Scoped.QueryJoin binds regardless; a statement leaving $1 unreferenced is rejected outright
//     ("could not determine data type of parameter $1"), and that failure would 500 the whole tab.
//
// Skipped unless VIGOV_TEST_DSN is set.

func dungQuyenStore(db *sql.DB) *QuyenStore { return NewQuyenStore(pkgstore.New(db)) }

// themNguoiDaXoa inserts a soft-deleted staff row holding a role. Rule 7 keeps the row; the count
// must not.
func themNguoiDaXoa(t *testing.T, db *sql.DB, tenantID, id, vaiTroID string) {
	t.Helper()
	// A SOFT-DELETED ACCOUNT STILL HAS TO SATISFY `nguoi_dung_co_tai_khoan_co_mat_khau`
	// (migration 0009 §3): the constraint is unconditional on purpose, so withdrawing an account
	// means setting co_tai_khoan = false, never merely clearing the hash. This fixture keeps the
	// account flag, so it keeps the hash — see bamMauPg in checker_pg_test.go.
	_, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, vai_tro_id, co_tai_khoan, mat_khau_hash, deleted_at)
		 VALUES ($1,$2,$3,$4,$5,$6,true,$7,now())`,
		tenantID, id, "CB-"+id, "Nguyễn Văn A", id+"@xa.danang.gov.vn", vaiTroID, bamMauPg)
	if err != nil {
		t.Fatalf("thêm cán bộ đã xoá: %v", err)
	}
}

func capQuyenCho(t *testing.T, db *sql.DB, tenantID, vaiTroID, quyenMa string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma) VALUES ($1,$2,$3)`,
		tenantID, vaiTroID, quyenMa)
	if err != nil {
		t.Fatalf("cấp quyền: %v", err)
	}
}

func TestPgMaTranDanhMucQuyenKhongLocTheoXa(t *testing.T) {
	// The catalogue is the SOFTWARE's, not a commune's: two different communes must read exactly
	// the same keys, and a commune that has never been set up must still read them all — otherwise
	// there is nothing to tick on its Phân quyền tab.
	db := moKetNoi(t)
	xa1, xa2 := xaRieng(t)
	kho := dungQuyenStore(db)

	a, err := kho.MaTran(ctxXa(xa1))
	if err != nil {
		t.Fatalf("MaTran xã 1: %v", err)
	}
	b, err := kho.MaTran(ctxXa(xa2))
	if err != nil {
		t.Fatalf("MaTran xã 2: %v", err)
	}

	// 33 is what migration 0001 seeds (the specification's heading says 43 while listing 33 — an
	// open question, not something to invent). Asserted as "not empty and identical" rather than as
	// the number itself: a migration adding the missing keys must not turn this red.
	if len(a.DanhMuc) == 0 {
		t.Fatal("danh mục quyền rỗng — màn hình phân quyền không có gì để tick")
	}
	if len(a.DanhMuc) != len(b.DanhMuc) {
		t.Fatalf("hai xã đọc được số quyền khác nhau: %d vs %d", len(a.DanhMuc), len(b.DanhMuc))
	}
	for i := range a.DanhMuc {
		if a.DanhMuc[i] != b.DanhMuc[i] {
			t.Fatalf("danh mục lệch giữa hai xã ở vị trí %d: %+v vs %+v", i, a.DanhMuc[i], b.DanhMuc[i])
		}
	}
	// Ordered by thu_tu: the first key the specification lists is admin.audit (§4.2).
	if a.DanhMuc[0].Ma != "admin.audit" {
		t.Errorf("thứ tự danh mục sai, quyền đầu = %q, muốn admin.audit", a.DanhMuc[0].Ma)
	}
	if a.DanhMuc[0].Nhom != "QUẢN TRỊ" || a.DanhMuc[0].Nhan != "Xem nhật ký hệ thống" {
		t.Errorf("nhóm/nhãn sai — hai cột TEXT liền nhau có thể đã bị hoán vị: %+v", a.DanhMuc[0])
	}
}

func TestPgMaTranDemCanBoTrongSQL(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	// vt-A holds two live staff, one soft-deleted. vt-B holds nobody at all.
	themVaiTro(t, db, xa, "vtA-"+xa, "truong-bo-phan", "Trưởng bộ phận", false)
	themVaiTro(t, db, xa, "vtB-"+xa, "ke-toan", "Kế toán", true)
	themNguoi(t, db, xa, "nd1-"+xa, "vtA-"+xa)
	themNguoi(t, db, xa, "nd2-"+xa, "vtA-"+xa)
	themNguoiDaXoa(t, db, xa, "nd3-"+xa, "vtA-"+xa)
	// A person with NO role at all: they must not be counted against anybody.
	themNguoi(t, db, xa, "nd4-"+xa, "")

	mt, err := dungQuyenStore(db).MaTran(ctxXa(xa))
	if err != nil {
		t.Fatalf("MaTran: %v", err)
	}
	if len(mt.Cot) != 2 {
		t.Fatalf("nhận %d vai trò, muốn 2: %+v", len(mt.Cot), mt.Cot)
	}

	theoID := map[string]int{}
	for _, c := range mt.Cot {
		theoID[c.ID] = c.SoCanBo
	}
	if got := theoID["vtA-"+xa]; got != 2 {
		t.Errorf("số cán bộ của vai trò A = %d, muốn 2 — cán bộ đã xoá mềm vẫn bị đếm?", got)
	}
	// THE count(*) TRAP. A role nobody holds must read 0, not 1.
	if got := theoID["vtB-"+xa]; got != 0 {
		t.Errorf("vai trò không ai giữ = %d cán bộ, muốn 0 — count(*) đang đếm dòng NULL của LEFT JOIN", got)
	}
}

// themNguoiKhoa inserts a staff row holding a role whose ACCOUNT CANNOT BE USED — either there is
// no account at all (`co_tai_khoan = false`, a public-directory entry) or it is locked
// (`dang_hoat_dong = false`). Both are people the register counts and store.Checker refuses.
func themNguoiKhoa(t *testing.T, db *sql.DB, tenantID, id, vaiTroID string, coTaiKhoan, dangHoatDong bool) {
	t.Helper()
	// The hash goes in whatever coTaiKhoan is: harmless on a directory-only row, required on an
	// account row (migration 0009 §3). Making it conditional here would put a branch in a fixture
	// for no gain.
	_, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, vai_tro_id, co_tai_khoan, dang_hoat_dong, mat_khau_hash)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		tenantID, id, "CB-"+id, "Nguyễn Văn A", id+"@xa.danang.gov.vn", vaiTroID,
		coTaiKhoan, dangHoatDong, bamMauPg)
	if err != nil {
		t.Fatalf("thêm cán bộ không dùng được: %v", err)
	}
}

func TestPgMaTranHaiSoDem(t *testing.T) {
	// THE CASE BOTH NUMBERS EXIST FOR: a role three people hold and NONE of them can use — one has
	// no sign-in account, one is locked, one was soft-deleted. It must read (2, 0):
	//
	//	staff_count           2  the register figure — the deleted person is not a holder at all,
	//	                         the account-less and the locked one are
	//	active_account_count  0  ticking any cell on this column reaches nobody
	//
	// The two wrong answers are the two places the predicate can be misplaced, and both look
	// entirely normal on screen: (2, 2) means the FILTER was lost or moved into the JOIN's ON
	// clause; the role vanishing altogether means it was pushed up into the WHERE.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-ket-"+xa, "vai-tro-ket", "Vai trò không ai dùng được", false)
	themNguoiKhoa(t, db, xa, "nd-khong-tk-"+xa, "vt-ket-"+xa, false, true) // chỉ có trong danh bạ
	themNguoiKhoa(t, db, xa, "nd-bi-khoa-"+xa, "vt-ket-"+xa, true, false)  // tài khoản bị khoá
	themNguoiDaXoa(t, db, xa, "nd-da-xoa-"+xa, "vt-ket-"+xa)

	// A second role where every holder is usable, so the test cannot pass by answering 0 to
	// everything.
	themVaiTro(t, db, xa, "vt-song-"+xa, "vai-tro-song", "Vai trò dùng được", false)
	themNguoi(t, db, xa, "nd-song-"+xa, "vt-song-"+xa)

	mt, err := dungQuyenStore(db).MaTran(ctxXa(xa))
	if err != nil {
		t.Fatalf("MaTran: %v", err)
	}

	theoID := map[string][2]int{}
	for _, c := range mt.Cot {
		theoID[c.ID] = [2]int{c.SoCanBo, c.SoTaiKhoanDangHoatDong}
	}
	if got, co := theoID["vt-ket-"+xa]; !co {
		t.Fatalf("vai trò không ai dùng được đã biến mất khỏi ma trận — điều kiện bị đẩy lên WHERE: %+v", mt.Cot)
	} else if got != [2]int{2, 0} {
		t.Errorf("vai trò kẹt = (%d, %d), muốn (2, 0)", got[0], got[1])
	}
	if got := theoID["vt-song-"+xa]; got != [2]int{1, 1} {
		t.Errorf("vai trò dùng được = (%d, %d), muốn (1, 1)", got[0], got[1])
	}
}

func TestPgMaTranGiuVaiTroKhiCanBoDuyNhatDaBiXoa(t *testing.T) {
	// `nd.deleted_at IS NULL` in the JOIN rather than the WHERE. Moved to the WHERE, this role
	// vanishes from the matrix — and a role absent from the matrix can never be granted anything,
	// which is a screen that silently cannot do its job.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-"+xa, "chu-tich-ubnd", "Chủ tịch UBND", true)
	themNguoiDaXoa(t, db, xa, "nd-"+xa, "vt-"+xa)

	mt, err := dungQuyenStore(db).MaTran(ctxXa(xa))
	if err != nil {
		t.Fatalf("MaTran: %v", err)
	}
	if len(mt.Cot) != 1 || mt.Cot[0].SoCanBo != 0 {
		t.Fatalf("vai trò phải còn trên ma trận với 0 cán bộ, nhận: %+v", mt.Cot)
	}
	if !mt.Cot[0].LaLanhDao {
		t.Error("la_lanh_dao không được đọc — nhãn Lãnh đạo sẽ biến mất khỏi tiêu đề cột")
	}
}

func TestPgMaTranKhongDocSangXaKhac(t *testing.T) {
	// THE CASE THAT MATTERS MOST HERE: the matrix is another authority's authorisation model. Two
	// communes are given roles and grants at once; each must see exactly its own — and the staff
	// count must not pick up the neighbour's people either.
	db := moKetNoi(t)
	xa1, xa2 := xaRieng(t)

	themVaiTro(t, db, xa1, "vt-"+xa1, "chu-tich-ubnd", "Chủ tịch UBND", true)
	themNguoi(t, db, xa1, "nd-"+xa1, "vt-"+xa1)
	capQuyenCho(t, db, xa1, "vt-"+xa1, "admin.role")

	themVaiTro(t, db, xa2, "vt-"+xa2, "ke-toan", "Kế toán", false)
	themNguoi(t, db, xa2, "nd1-"+xa2, "vt-"+xa2)
	themNguoi(t, db, xa2, "nd2-"+xa2, "vt-"+xa2)
	capQuyenCho(t, db, xa2, "vt-"+xa2, "budget.read")

	kho := dungQuyenStore(db)

	mt1, err := kho.MaTran(ctxXa(xa1))
	if err != nil {
		t.Fatalf("MaTran xã 1: %v", err)
	}
	if len(mt1.Cot) != 1 || mt1.Cot[0].ID != "vt-"+xa1 || mt1.Cot[0].SoCanBo != 1 {
		t.Fatalf("xã 1 phải thấy đúng vai trò của mình với 1 cán bộ, nhận: %+v", mt1.Cot)
	}
	if len(mt1.DaCap) != 1 || mt1.DaCap[0].QuyenMa != "admin.role" {
		t.Fatalf("xã 1 phải thấy đúng ô đã cấp của mình, nhận: %+v", mt1.DaCap)
	}

	mt2, err := kho.MaTran(ctxXa(xa2))
	if err != nil {
		t.Fatalf("MaTran xã 2: %v", err)
	}
	if len(mt2.Cot) != 1 || mt2.Cot[0].ID != "vt-"+xa2 || mt2.Cot[0].SoCanBo != 2 {
		t.Fatalf("xã 2 phải thấy đúng vai trò của mình với 2 cán bộ, nhận: %+v", mt2.Cot)
	}
	if len(mt2.DaCap) != 1 || mt2.DaCap[0].QuyenMa != "budget.read" {
		t.Fatalf("xã 2 phải thấy đúng ô đã cấp của mình, nhận: %+v", mt2.DaCap)
	}
}

func TestPgMaTranBoQuaOCuaVaiTroDaXoaMem(t *testing.T) {
	// Rule 7 keeps a soft-deleted role and its grants. They must not reach the response: a pair
	// naming a role absent from `Cot` is a tick a client cannot place — or, worse, draws a column
	// for a role the commune has retired.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-song-"+xa, "can-bo-mot-cua", "Cán bộ một cửa", false)
	themVaiTro(t, db, xa, "vt-xoa-"+xa, "vai-tro-cu", "Vai trò cũ", false)
	capQuyenCho(t, db, xa, "vt-song-"+xa, "task.read")
	capQuyenCho(t, db, xa, "vt-xoa-"+xa, "task.approve")
	if _, err := db.Exec(
		`UPDATE vai_tro SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "vt-xoa-"+xa); err != nil {
		t.Fatalf("xoá mềm vai trò: %v", err)
	}

	mt, err := dungQuyenStore(db).MaTran(ctxXa(xa))
	if err != nil {
		t.Fatalf("MaTran: %v", err)
	}
	if len(mt.Cot) != 1 || mt.Cot[0].ID != "vt-song-"+xa {
		t.Fatalf("vai trò đã xoá mềm vẫn lên ma trận: %+v", mt.Cot)
	}
	if len(mt.DaCap) != 1 || mt.DaCap[0].VaiTroID != "vt-song-"+xa || mt.DaCap[0].QuyenMa != "task.read" {
		t.Fatalf("ô của vai trò đã xoá mềm vẫn lọt ra: %+v", mt.DaCap)
	}
}
