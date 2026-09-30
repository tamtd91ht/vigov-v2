package store

// Integration tests for the Mini App content register, against a REAL PostgreSQL.
//
// THE OTHER HALF OF noi_dung_mini_app_test.go, NOT A REPLACEMENT FOR IT. That file runs a fake driver
// which RECORDS statements and hands back whatever rows the test supplied. It proves the Go side —
// the positional scan, the commune binding, the filter placeholders, the escaping — and it runs
// everywhere, always. What it CANNOT prove is anything the database decides, and for these two tables
// the database decides most of what matters:
//
//  1. every column in cotNoiDungMiniAppChiTiet — and both table names — exists as migration 0006
//     creates it. THIS IS THE ASSERTION THE FAKE DRIVER STRUCTURALLY CANNOT MAKE;
//  2. the same item id in TWO communes is two different rows, and the same portal id in two communes
//     is two different articles. A key without `tenant_id` would make the second commune collide
//     with the first, and no single-commune test can show it (rule 1, invariant 6);
//  3. `UNIQUE (tenant_id, nguon_id_ngoai)` really is what makes §10.1's deduplication atomic, and it
//     really does keep holding the id of a SOFT-DELETED article — which is what stops the next sync
//     from bringing back something a member of staff removed on purpose;
//  4. the CHECK constraints: the six `loai` codes, the three states, the equivalence tying `nguon` to
//     `nguon_id_ngoai`, and the one that keeps `da_sua_tay` off a hand-composed row;
//  5. the trigger that refuses to change provenance, to clear §10.4's flag, or to lower `luot_xem`;
//  6. the self-referencing foreign key on the category tree, composed with `tenant_id`;
//  7. that a soft-deleted item leaves the list — through the real partial indexes.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, AND THAT IS NOT A FOOTNOTE — read it as a warning about this
// repository. No PostgreSQL is reachable from this build environment, so on every machine anybody has
// run this on so far, every test in this file SKIPS while the package still prints `ok`. A green
// `go test` here means this file COMPILES. Nothing in it may be described as verified until somebody
// sets the DSN and says what came out.
//
// TestMain, moKetNoi and xaRieng live in loai_tai_nguyen_ban_do_pg_test.go and are shared: ONE schema
// per run, built by the REAL migration runner, so a migration added later is exercised the day it is
// added rather than the day somebody remembers to extend a list.
//
// ONE REFUSAL IS DELIBERATELY NOT TESTED HERE, and it is named rather than quietly skipped: that a
// hard delete of an item or a category is refused by `ho_so_luu_tru_cam_xoa_cung`. Writing that test
// means writing the statement, and `data_safety_guard` blocks the literal in a Go file — it cannot
// tell a test that ASSERTS the refusal from code that performs the deletion. Routing around a guard
// is worse than the missing case, so the case is reported instead. Until the guard can tell the two
// apart, that trigger is covered by reading migration 0006 and by nothing else.

import (
	"context"
	"strings"
	"testing"
	"time"

	pkgpage "github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// themDanhMucThat inserts one category directly. EVERY COLUMN IS PASSED EXPLICITLY rather than left
// to its default: a test that relied on a default could not tell "the column is read" from "the
// column is always its default".
func themDanhMucThat(t *testing.T, tenantID, id, ten, slug, chaID string, thuTu int) {
	t.Helper()
	db := moKetNoi(t)

	var cha any
	if chaID != "" {
		cha = chaID
	}
	_, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, cha_id, thu_tu)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, id, ten, slug, cha, thuTu)
	if err != nil {
		t.Fatalf("thêm danh mục %q: %v", id, err)
	}
}

// themNoiDungThat inserts one item directly.
func themNoiDungThat(t *testing.T, tenantID, id, loai, tieuDe, trangThai, nguon, maNgoai string,
	taoLuc time.Time) {
	t.Helper()
	db := moKetNoi(t)

	var ngoai any
	if maNgoai != "" {
		ngoai = maNgoai
	}
	_, err := db.Exec(
		`INSERT INTO noi_dung_mini_app
		 (tenant_id, id, loai, tieu_de, tom_tat, noi_dung, ngay_dang, trang_thai,
		  nguon, nguon_id_ngoai, nguoi_tao_ma, tao_luc)
		 VALUES ($1,$2,$3,$4,'Tóm tắt.','<p>Toàn văn.</p>',$5,$6,$7,$8,'CB-2026-7K3M9Q',$9)`,
		tenantID, id, loai, tieuDe, taoLuc, trangThai, nguon, ngoai, taoLuc)
	if err != nil {
		t.Fatalf("thêm nội dung %q: %v", id, err)
	}
}

func khoNoiDungThat(t *testing.T) *NoiDungMiniAppStore {
	t.Helper()
	return NewNoiDungMiniAppStore(pkgstore.New(moKetNoi(t)))
}

func khoDanhMucNDThat(t *testing.T) *DanhMucMiniAppStore {
	t.Helper()
	return NewDanhMucMiniAppStore(pkgstore.New(moKetNoi(t)))
}

func trangDauNDThat(t *testing.T) pkgpage.Request {
	t.Helper()
	yc, err := pkgpage.New(SapXepNoiDungMiniApp, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return yc
}

// --- (1) the column lists against the real schema -----------------------------------------------

func TestPgCotNoiDungMiniAppKhopVoiLuocDoThat(t *testing.T) {
	// THE REASON THIS FILE EXISTS. cotNoiDungMiniApp is a string, and the fake driver builds its rows
	// from that same string — so a column renamed in a later migration, or misspelled here from the
	// start, is invisible to every test that does not touch a real schema. It would surface as a
	// broken screen with "column ... does not exist" in a log nobody is reading yet.
	db := moKetNoi(t)

	co := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'noi_dung_mini_app'`)
	if err != nil {
		t.Fatalf("đọc information_schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("đọc tên cột: %v", err)
		}
		co[c] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("duyệt tên cột: %v", err)
	}
	if len(co) == 0 {
		t.Fatal("bảng noi_dung_mini_app không tồn tại trong schema thật — migration 0006 chưa chạy?")
	}

	for _, c := range strings.Split(cotNoiDungMiniAppChiTiet, ",") {
		ten := strings.TrimSpace(c)
		if ten == "" {
			continue
		}
		if !co[ten] {
			t.Errorf("cột %q có trong cotNoiDungMiniAppChiTiet nhưng KHÔNG có trong lược đồ thật", ten)
		}
	}
	// The soft-delete triple, checked by name: rule 7, invariant 1 names all three, and a table
	// carrying two of them is a table where a deletion cannot say who did it or why.
	for _, c := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !co[c] {
			t.Errorf("thiếu cột xoá mềm %q (luật 7, bất biến 1)", c)
		}
	}
	// `tep_dinh_kem` IS IN THE SCHEMA AND IN NO COLUMN LIST, on purpose (§8 declares it, nothing
	// writes it in this pass). Asserting it exists is what stops the next reader from concluding the
	// migration forgot it.
	if !co["tep_dinh_kem"] {
		t.Error("thiếu cột `tep_dinh_kem` mà §8 khai — nó cố ý chưa có đường ghi, không phải chưa có cột")
	}
}

func TestPgCotDanhMucMiniAppKhopVoiLuocDoThat(t *testing.T) {
	db := moKetNoi(t)

	co := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'danh_muc_mini_app'`)
	if err != nil {
		t.Fatalf("đọc information_schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("đọc tên cột: %v", err)
		}
		co[c] = true
	}
	if len(co) == 0 {
		t.Fatal("bảng danh_muc_mini_app không tồn tại trong schema thật")
	}
	for _, c := range strings.Split(cotDanhMucMiniApp, ",") {
		ten := strings.TrimSpace(c)
		if ten != "" && !co[ten] {
			t.Errorf("cột %q có trong cotDanhMucMiniApp nhưng KHÔNG có trong lược đồ thật", ten)
		}
	}
}

// --- (2) two communes ------------------------------------------------------------------------

func TestPgHaiXaKhongThayNoiDungCuaNhau(t *testing.T) {
	// RULE 1, THE ASSERTION NO SINGLE-COMMUNE TEST CAN MAKE. The same id is inserted in both communes;
	// a primary key without `tenant_id` would make the second INSERT collide with the first, and a
	// read that forgot the predicate would return both.
	xa1, xa2 := xaRieng(t)
	luc := time.Now().UTC()

	themNoiDungThat(t, xa1, "nd-chung", "tin-tuc", "Tin của xã 1", "dang-hien", "thu-cong", "", luc)
	themNoiDungThat(t, xa2, "nd-chung", "tin-tuc", "Tin của xã 2", "dang-hien", "thu-cong", "", luc)

	kho := khoNoiDungThat(t)

	kq1, err := kho.DanhSach(ctxXa(tenant.ID(xa1)), LocNoiDung{}, trangDauNDThat(t))
	if err != nil {
		t.Fatalf("đọc xã 1: %v", err)
	}
	for _, n := range kq1.Items {
		if n.TieuDe == "Tin của xã 2" {
			t.Fatal("xã 1 đọc được nội dung của xã 2 — VI PHẠM CÁCH LY (luật 1)")
		}
	}

	mot, err := kho.TheoID(ctxXa(tenant.ID(xa2)), "nd-chung")
	if err != nil {
		t.Fatalf("đọc chi tiết xã 2: %v", err)
	}
	if mot.TieuDe != "Tin của xã 2" {
		t.Errorf("tuyến chi tiết trả %q cho xã 2 — hai xã cùng id phải là hai dòng", mot.TieuDe)
	}
}

func TestPgHaiXaDungChungMotMaBaiTrenCong(t *testing.T) {
	// `UNIQUE (tenant_id, nguon_id_ngoai)` COMPOSED WITH THE COMMUNE. Two communes whose portals hand
	// out the same article id are two different articles, and a single-column key would make the
	// second commune unable to take its own news — a defect that appears on the day of the second
	// commune, when fixing it is a migration on live data.
	xa1, xa2 := xaRieng(t)
	luc := time.Now().UTC()

	themNoiDungThat(t, xa1, "nd-1", "tin-tuc", "Bài của xã 1", "dang-hien", "dong-bo-cong", "cong-42", luc)
	themNoiDungThat(t, xa2, "nd-2", "tin-tuc", "Bài của xã 2", "dang-hien", "dong-bo-cong", "cong-42", luc)
}

// --- (3) the deduplication key survives a soft delete ---------------------------------------------

func TestPgMaBaiTrenCongVanBiChiemSauKhiXoaMem(t *testing.T) {
	// §10.1 AND RULE 7, INVARIANT 3 IN ONE CASE, and it is the sharpest one in this file. A member of
	// staff removed a portal article on purpose. The next sync runs in six hours; if the soft-deleted
	// row released its `nguon_id_ngoai`, the article would come straight back — silently, and forever.
	xa1, _ := xaRieng(t)
	db := moKetNoi(t)
	luc := time.Now().UTC()

	themNoiDungThat(t, xa1, "nd-1", "tin-tuc", "Bài từ Cổng", "dang-hien", "dong-bo-cong", "cong-99", luc)
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		  WHERE tenant_id = $1 AND id = $2`,
		xa1, "nd-1", "CB-2026-7K3M9Q", "Bài đăng nhầm"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	_, err := db.Exec(
		`INSERT INTO noi_dung_mini_app
		 (tenant_id, id, loai, tieu_de, ngay_dang, trang_thai, nguon, nguon_id_ngoai, nguoi_tao_ma)
		 VALUES ($1,'nd-2','tin-tuc','Cùng bài ấy',$2,'dang-hien','dong-bo-cong','cong-99','CB-1')`,
		xa1, luc)
	if err == nil {
		t.Fatal("mã bài trên Cổng được cấp lại sau khi xoá mềm — lượt đồng bộ sau sẽ mang bài đã gỡ quay về")
	}

	// AND THE DELETED ROW IS GONE FROM THE LIST, through the real partial index (rule 7, invariant 2).
	kho := khoNoiDungThat(t)
	kq, err := kho.DanhSach(ctxXa(tenant.ID(xa1)), LocNoiDung{}, trangDauNDThat(t))
	if err != nil {
		t.Fatalf("đọc sổ: %v", err)
	}
	for _, n := range kq.Items {
		if n.ID == "nd-1" {
			t.Error("dòng đã xoá mềm vẫn nằm trong danh sách (luật 7, bất biến 2)")
		}
	}
}

// --- (4) the CHECK constraints ------------------------------------------------------------------

func TestPgRangBuocKiemTraTuChoiGiaTriNgoaiDacTa(t *testing.T) {
	xa1, _ := xaRieng(t)
	db := moKetNoi(t)
	luc := time.Now().UTC()

	chen := func(loai, trangThai, nguon string, maNgoai any, daSuaTay bool) error {
		_, err := db.Exec(
			`INSERT INTO noi_dung_mini_app
			 (tenant_id, id, loai, tieu_de, ngay_dang, trang_thai, nguon, nguon_id_ngoai,
			  da_sua_tay, nguoi_tao_ma)
			 VALUES ($1,$2,$3,'Tiêu đề',$4,$5,$6,$7,$8,'CB-1')`,
			xa1, "nd-"+loai+trangThai+nguon+time.Now().Format("150405.000000000"),
			loai, luc, trangThai, nguon, maNgoai, daSuaTay)
		return err
	}

	// A SEVENTH `loai` — a tab no screen draws, holding rows nobody can find.
	if err := chen("podcast", "an", "thu-cong", nil, false); err == nil {
		t.Error("`loai` ngoài sáu mã của §5 mà vẫn chèn được")
	}
	// A FOURTH state — §6 draws three chips.
	if err := chen("tin-tuc", "da-duyet", "thu-cong", nil, false); err == nil {
		t.Error("`trang_thai` ngoài ba giá trị của §6 mà vẫn chèn được")
	}
	// PROVENANCE AND THE PORTAL ID ARE ONE FACT, both directions. A synced article with no portal id
	// cannot be deduplicated, so the next run imports it again, and again.
	if err := chen("tin-tuc", "an", "dong-bo-cong", nil, false); err == nil {
		t.Error("bài `dong-bo-cong` không có `nguon_id_ngoai` mà vẫn chèn được — §10.1 khử trùng bằng cột ấy")
	}
	// And a hand-composed article carrying one is a row a sync would believe it owns.
	if err := chen("tin-tuc", "an", "thu-cong", "cong-1", false); err == nil {
		t.Error("bài `thu-cong` mang `nguon_id_ngoai` mà vẫn chèn được")
	}
	// §10.4's flag on a row the sync never touches claims a protection that protects nothing.
	if err := chen("tin-tuc", "an", "thu-cong", nil, true); err == nil {
		t.Error("bài soạn tay mang cờ `da_sua_tay` mà vẫn chèn được")
	}
	// The ordinary rows must still go in.
	if err := chen("banner", "cho-duyet", "dong-bo-cong", "cong-ok-1", true); err != nil {
		t.Errorf("dòng hợp lệ bị từ chối: %v", err)
	}
	if err := chen("truyen-thanh", "dang-hien", "thu-cong", nil, false); err != nil {
		t.Errorf("dòng hợp lệ bị từ chối: %v", err)
	}
}

// --- (5) the immutability trigger ------------------------------------------------------------------

func TestPgKhongDoiDuocXuatXuVaKhongGoDuocCoDaSuaTay(t *testing.T) {
	xa1, _ := xaRieng(t)
	db := moKetNoi(t)
	luc := time.Now().UTC()

	themNoiDungThat(t, xa1, "nd-1", "tin-tuc", "Bài từ Cổng", "dang-hien", "dong-bo-cong", "cong-7", luc)

	// PROVENANCE IS IMMUTABLE: a row that could change it would move in or out of §10.1's
	// deduplication and, worse, out of §10.4's protection.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET nguon = 'thu-cong' WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err == nil {
		t.Error("đổi được `nguon` của một bài đã có")
	}
	// THE PORTAL ID IS IMMUTABLE ONCE SET: freeing it imports the same article again as a second row.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET nguon_id_ngoai = 'cong-8' WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err == nil {
		t.Error("đổi được `nguon_id_ngoai` của một bài đã có")
	}

	// §10.4: false -> true IS THE ORDINARY ACT and must be allowed.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET da_sua_tay = true WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err != nil {
		t.Fatalf("không đặt được cờ da_sua_tay: %v", err)
	}
	// true -> false IS REFUSED: clearing it withdraws a promise made to a member of staff, and the
	// next run overwrites their correction with nobody being told.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET da_sua_tay = false WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err == nil {
		t.Error("gỡ được cờ `da_sua_tay` — lượt đồng bộ sau sẽ ghi đè bản sửa tay (§10.4)")
	}

	// THE AUTHOR AND THE CREATION TIME ARE THE ANCHORS OF THE RECORD.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET nguoi_tao_ma = 'CB-KHAC' WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err == nil {
		t.Error("đổi được người tạo")
	}
}

func TestPgLuotXemKhongGiamDuoc(t *testing.T) {
	// A figure that can go down with no event behind it is the defect rule 10, invariant 3 describes
	// for `qua_han`, in a different column — and the report going upward is what reads it.
	xa1, _ := xaRieng(t)
	db := moKetNoi(t)

	themNoiDungThat(t, xa1, "nd-1", "tin-tuc", "Bài", "dang-hien", "thu-cong", "", time.Now().UTC())

	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET luot_xem = 10 WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err != nil {
		t.Fatalf("không tăng được lượt xem: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET luot_xem = 3 WHERE tenant_id = $1 AND id = 'nd-1'`,
		xa1); err == nil {
		t.Error("giảm được lượt xem")
	}
}

// --- (6) the category tree ----------------------------------------------------------------------

func TestPgSlugDanhMucDuyNhatTrongMotXaVaKhongCapLai(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	db := moKetNoi(t)

	themDanhMucThat(t, xa1, "dm-1", "Chuyển đổi số", "chuyen-doi-so", "", 1)

	// THE SAME SLUG IN ANOTHER COMMUNE IS A DIFFERENT CATEGORY — that is what makes the key composite.
	themDanhMucThat(t, xa2, "dm-2", "Chuyển đổi số", "chuyen-doi-so", "", 1)

	// TWICE IN ONE COMMUNE IS REFUSED.
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, thu_tu)
		 VALUES ($1,'dm-3','Chuyển đổi số (2)','chuyen-doi-so',2)`, xa1); err == nil {
		t.Error("slug trùng trong một xã mà vẫn chèn được")
	}

	// AND IT STAYS TAKEN AFTER A SOFT DELETE (rule 7, invariant 3): every article filed under it
	// holds the slug as a value, and reissuing it would refile them under a new, unrelated category.
	if _, err := db.Exec(
		`UPDATE danh_muc_mini_app SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'gộp mục'
		  WHERE tenant_id = $1 AND id = 'dm-1'`, xa1); err != nil {
		t.Fatalf("xoá mềm danh mục: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, thu_tu)
		 VALUES ($1,'dm-4','Chuyển đổi số (mới)','chuyen-doi-so',3)`, xa1); err == nil {
		t.Error("slug đã cấp được cấp lại sau khi xoá mềm (luật 7, bất biến 3)")
	}
}

func TestPgKhoaNgoaiCayDanhMucVaKhongTuLamCha(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	db := moKetNoi(t)

	themDanhMucThat(t, xa1, "dm-goc", "Danh mục", "danh-muc", "", 1)
	themDanhMucThat(t, xa1, "dm-con", "Chuyển đổi số", "chuyen-doi-so", "dm-goc", 2)

	// A PARENT THAT DOES NOT EXIST IS REFUSED.
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, cha_id, thu_tu)
		 VALUES ($1,'dm-x','Mục lạ','muc-la','dm-khong-co',1)`, xa1); err == nil {
		t.Error("cha không tồn tại mà vẫn chèn được")
	}
	// A PARENT IN ANOTHER COMMUNE IS REFUSED — the key is composed with `tenant_id`, so a child
	// cannot reach across authorities even if two ids ever collided.
	themDanhMucThat(t, xa2, "dm-cua-xa-2", "Mục của xã 2", "muc-cua-xa-2", "", 1)
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, cha_id, thu_tu)
		 VALUES ($1,'dm-y','Mục lạ','muc-la-2','dm-cua-xa-2',1)`, xa1); err == nil {
		t.Error("cha thuộc xã khác mà vẫn chèn được — CÁCH LY (luật 1) hỏng ở khoá ngoại")
	}
	// A CATEGORY CANNOT BE ITS OWN PARENT. This CHECK catches the one-node cycle only; a cycle through
	// two or more rows is the write path's job — migration 0006 records the obligation.
	if _, err := db.Exec(
		`UPDATE danh_muc_mini_app SET cha_id = 'dm-con' WHERE tenant_id = $1 AND id = 'dm-con'`,
		xa1); err == nil {
		t.Error("một danh mục tự làm cha của chính nó mà vẫn ghi được")
	}
}

func TestPgKhoaNgoaiNoiDungTroiVaoDanhMucCuaChinhXa(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	db := moKetNoi(t)
	luc := time.Now().UTC()

	themDanhMucThat(t, xa2, "dm-cua-xa-2", "Mục của xã 2", "muc-cua-xa-2", "", 1)

	if _, err := db.Exec(
		`INSERT INTO noi_dung_mini_app
		 (tenant_id, id, loai, danh_muc_id, tieu_de, ngay_dang, trang_thai, nguon, nguoi_tao_ma)
		 VALUES ($1,'nd-1','tin-tuc','dm-cua-xa-2','Tiêu đề',$2,'an','thu-cong','CB-1')`,
		xa1, luc); err == nil {
		t.Error("bài của xã 1 xếp được vào danh mục của xã 2 — CÁCH LY (luật 1) hỏng ở khoá ngoại")
	}
}

// --- (7) the filters, against the real indexes ------------------------------------------------------

func TestPgLocTheoLoaiDanhMucVaTieuDe(t *testing.T) {
	xa1, _ := xaRieng(t)
	luc := time.Now().UTC()

	themDanhMucThat(t, xa1, "dm-1", "Chuyển đổi số", "chuyen-doi-so", "", 1)
	themNoiDungThat(t, xa1, "nd-tin", "tin-tuc", "Xã khai giảng năm học mới", "dang-hien", "thu-cong", "", luc)
	themNoiDungThat(t, xa1, "nd-banner", "banner", "Khẩu hiệu chào mừng", "dang-hien", "thu-cong", "", luc.Add(time.Second))

	kho := khoNoiDungThat(t)
	ctx := ctxXa(tenant.ID(xa1))

	kq, err := kho.DanhSach(ctx, LocNoiDung{Loai: "banner"}, trangDauNDThat(t))
	if err != nil {
		t.Fatalf("lọc theo loại: %v", err)
	}
	if len(kq.Items) != 1 || kq.Items[0].ID != "nd-banner" {
		t.Errorf("lọc `loai=banner` trả %d dòng: %+v", len(kq.Items), kq.Items)
	}

	kq, err = kho.DanhSach(ctx, LocNoiDung{Tu: "khai giảng"}, trangDauNDThat(t))
	if err != nil {
		t.Fatalf("tìm theo tiêu đề: %v", err)
	}
	if len(kq.Items) != 1 || kq.Items[0].ID != "nd-tin" {
		t.Errorf("tìm `khai giảng` trả %d dòng: %+v", len(kq.Items), kq.Items)
	}

	// ILIKE IS CASE-INSENSITIVE AND MUST BE — §6's box is `Tìm theo tiêu đề…`, and a member of staff
	// typing lower case must find a title that starts with a capital.
	kq, err = kho.DanhSach(ctx, LocNoiDung{Tu: "KHAI GIẢNG"}, trangDauNDThat(t))
	if err != nil {
		t.Fatalf("tìm không phân biệt hoa thường: %v", err)
	}
	if len(kq.Items) != 1 {
		t.Errorf("tìm `KHAI GIẢNG` trả %d dòng, muốn 1", len(kq.Items))
	}

	// THE WILDCARD IS ESCAPED: `%` typed in the box is a literal percent sign, not "match everything".
	kq, err = kho.DanhSach(ctx, LocNoiDung{Tu: "%"}, trangDauNDThat(t))
	if err != nil {
		t.Fatalf("tìm ký tự đại diện: %v", err)
	}
	if len(kq.Items) != 0 {
		t.Errorf("gõ `%%` vào ô tìm trả %d dòng, muốn 0 — ký tự đại diện phải được thoát", len(kq.Items))
	}
}

func TestPgDanhSachDanhMucSapXepVaLocDongDaXoa(t *testing.T) {
	xa1, _ := xaRieng(t)
	db := moKetNoi(t)

	themDanhMucThat(t, xa1, "dm-b", "Mục B", "muc-b", "", 2)
	themDanhMucThat(t, xa1, "dm-a", "Mục A", "muc-a", "", 1)
	themDanhMucThat(t, xa1, "dm-x", "Mục đã gỡ", "muc-da-go", "", 3)
	if _, err := db.Exec(
		`UPDATE danh_muc_mini_app SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'gộp'
		  WHERE tenant_id = $1 AND id = 'dm-x'`, xa1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ds, err := khoDanhMucNDThat(t).DanhSach(ctxXa(tenant.ID(xa1)))
	if err != nil {
		t.Fatalf("đọc danh mục: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("số danh mục = %d, muốn 2 (dòng đã xoá mềm phải biến mất)", len(ds))
	}
	if ds[0].ID != "dm-a" || ds[1].ID != "dm-b" {
		t.Errorf("thứ tự = %q, %q — muốn theo thu_tu", ds[0].ID, ds[1].ID)
	}
}

// --- (8) the write path, end to end ---------------------------------------------------------------

func TestPgChenVaCapNhatQuaKhoThat(t *testing.T) {
	xa1, _ := xaRieng(t)
	kho := khoNoiDungThat(t)
	ctx := ctxXa(tenant.ID(xa1))
	luc := time.Now().UTC()

	themDanhMucThat(t, xa1, "dm-1", "Chuyển đổi số", "chuyen-doi-so", "", 1)

	err := pkgstore.New(moKetNoi(t)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.Chen(ctx, tx, domain.NoiDungMiniApp{
			ID: "nd-1", Loai: domain.LoaiTinTuc, DanhMucID: "dm-1",
			TieuDe: "Xã khai giảng", TomTat: "", NoiDung: "<p>Toàn văn</p>",
			NgayDang: luc, TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-2026-7K3M9Q",
		})
	})
	if err != nil {
		t.Fatalf("chèn qua kho thật: %v", err)
	}

	mot, err := kho.TheoID(ctx, "nd-1")
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	// `nguon` WAS WRITTEN AS A LITERAL — the store binds no parameter for it.
	if mot.Nguon != domain.NguonThuCong || mot.NguonIDNgoai != "" {
		t.Errorf("xuất xứ = %q / %q, muốn thu-cong và rỗng", mot.Nguon, mot.NguonIDNgoai)
	}
	// AN EMPTY SUMMARY WAS STORED AS NULL AND READS BACK AS "" — one spelling of "nothing".
	if mot.TomTat != "" {
		t.Errorf("tóm tắt = %q, muốn rỗng", mot.TomTat)
	}
	if mot.LuotXem != 0 || mot.DaSuaTay {
		t.Errorf("dòng mới phải có 0 lượt xem và chưa sửa tay: %+v", mot)
	}

	err = pkgstore.New(moKetNoi(t)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		truoc, err := kho.TheoIDDeSua(ctx, tx, "nd-1")
		if err != nil {
			return err
		}
		truoc.TieuDe = "Xã khai giảng năm học mới"
		truoc.TrangThai = domain.TrangThaiDangHien
		return kho.CapNhat(ctx, tx, truoc)
	})
	if err != nil {
		t.Fatalf("cập nhật qua kho thật: %v", err)
	}

	mot, err = kho.TheoID(ctx, "nd-1")
	if err != nil {
		t.Fatalf("đọc lại sau sửa: %v", err)
	}
	if mot.TieuDe != "Xã khai giảng năm học mới" || !mot.HienChoDan() {
		t.Errorf("sau khi sửa = %+v", mot)
	}
}

func TestPgCapNhatDongDaXoaMemLa404(t *testing.T) {
	// `AND deleted_at IS NULL` IS WHAT MAKES EDITING A DELETED ITEM A 404 rather than a resurrection.
	xa1, _ := xaRieng(t)
	db := moKetNoi(t)
	ctx := ctxXa(tenant.ID(xa1))

	themNoiDungThat(t, xa1, "nd-1", "tin-tuc", "Bài", "an", "thu-cong", "", time.Now().UTC())
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'gỡ'
		  WHERE tenant_id = $1 AND id = 'nd-1'`, xa1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	kho := khoNoiDungThat(t)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.TheoIDDeSua(ctx, tx, "nd-1")
		return err
	})
	if err == nil {
		t.Fatal("đọc được dòng đã xoá mềm để sửa")
	}
}

// ctxXa lives in loai_tai_nguyen_ban_do_test.go, next to the first fake driver, and is shared with
// this file. ONE way to put a commune into a context, not two: the second copy is the one that ends
// up subtly different from what the edge really does.
var _ = context.Background
