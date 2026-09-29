package store

// Integration tests for the Mini App content register, against a REAL PostgreSQL.
//
// THE OTHER HALF OF content_item_test.go, NOT A REPLACEMENT FOR IT. That file runs a fake driver
// which RECORDS statements and hands back whatever rows the test supplied. It proves the Go side —
// the positional scan, the commune binding, the filter placeholders, the escaping — and it runs
// everywhere, always. What it CANNOT prove is anything the database decides, and for these two tables
// the database decides most of what matters:
//
//  1. every column in contentItemDetailColumns — and both table names — exists as migration 0006
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
// TestMain, openTestDB and uniqueTenants live in map_asset_type_pg_test.go and are shared: ONE schema
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

// insertCategoryRow inserts one category directly. EVERY COLUMN IS PASSED EXPLICITLY rather than left
// to its default: a test that relied on a default could not tell "the column is read" from "the
// column is always its default".
func insertCategoryRow(t *testing.T, tenantID, id, name, slug, parentID string, order int) {
	t.Helper()
	db := openTestDB(t)

	var parent any
	if parentID != "" {
		parent = parentID
	}
	_, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, cha_id, thu_tu)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, id, name, slug, parent, order)
	if err != nil {
		t.Fatalf("thêm danh mục %q: %v", id, err)
	}
}

// insertContentRow inserts one item directly.
func insertContentRow(t *testing.T, tenantID, id, typ, title, status, source, externalRef string,
	createdAt time.Time) {
	t.Helper()
	db := openTestDB(t)

	var ref any
	if externalRef != "" {
		ref = externalRef
	}
	_, err := db.Exec(
		`INSERT INTO noi_dung_mini_app
		 (tenant_id, id, loai, tieu_de, tom_tat, noi_dung, ngay_dang, trang_thai,
		  nguon, nguon_id_ngoai, nguoi_tao_ma, tao_luc)
		 VALUES ($1,$2,$3,$4,'Tóm tắt.','<p>Toàn văn.</p>',$5,$6,$7,$8,'CB-2026-7K3M9Q',$9)`,
		tenantID, id, typ, title, createdAt, status, source, ref, createdAt)
	if err != nil {
		t.Fatalf("thêm nội dung %q: %v", id, err)
	}
}

func newRealContentItemStore(t *testing.T) *ContentItemStore {
	t.Helper()
	return NewContentItemStore(pkgstore.New(openTestDB(t)))
}

func newRealContentCategoryStore(t *testing.T) *ContentCategoryStore {
	t.Helper()
	return NewContentCategoryStore(pkgstore.New(openTestDB(t)))
}

func realContentFirstPage(t *testing.T) pkgpage.Request {
	t.Helper()
	req, err := pkgpage.New(ContentItemSorts, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return req
}

// --- (1) the column lists against the real schema -----------------------------------------------

func TestPgContentItemColumnsMatchRealSchema(t *testing.T) {
	// THE REASON THIS FILE EXISTS. contentItemColumns is a string, and the fake driver builds its rows
	// from that same string — so a column renamed in a later migration, or misspelled here from the
	// start, is invisible to every test that does not touch a real schema. It would surface as a
	// broken screen with "column ... does not exist" in a log nobody is reading yet.
	db := openTestDB(t)

	have := map[string]bool{}
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
		have[c] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("duyệt tên cột: %v", err)
	}
	if len(have) == 0 {
		t.Fatal("bảng noi_dung_mini_app không tồn tại trong schema thật — migration 0006 chưa chạy?")
	}

	for _, c := range strings.Split(contentItemDetailColumns, ",") {
		name := strings.TrimSpace(c)
		if name == "" {
			continue
		}
		if !have[name] {
			t.Errorf("cột %q có trong contentItemDetailColumns nhưng KHÔNG có trong lược đồ thật", name)
		}
	}
	// The soft-delete triple, checked by name: rule 7, invariant 1 names all three, and a table
	// carrying two of them is a table where a deletion cannot say who did it or why.
	for _, c := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !have[c] {
			t.Errorf("thiếu cột xoá mềm %q (luật 7, bất biến 1)", c)
		}
	}
	// `tep_dinh_kem` IS IN THE SCHEMA AND IN NO COLUMN LIST, on purpose (§8 declares it, nothing
	// writes it in this pass). Asserting it exists is what stops the next reader from concluding the
	// migration forgot it.
	if !have["tep_dinh_kem"] {
		t.Error("thiếu cột `tep_dinh_kem` mà §8 khai — nó cố ý chưa có đường ghi, không phải chưa có cột")
	}
}

func TestPgContentCategoryColumnsMatchRealSchema(t *testing.T) {
	db := openTestDB(t)

	have := map[string]bool{}
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
		have[c] = true
	}
	if len(have) == 0 {
		t.Fatal("bảng danh_muc_mini_app không tồn tại trong schema thật")
	}
	for _, c := range strings.Split(contentCategoryColumns, ",") {
		name := strings.TrimSpace(c)
		if name != "" && !have[name] {
			t.Errorf("cột %q có trong contentCategoryColumns nhưng KHÔNG có trong lược đồ thật", name)
		}
	}
}

// --- (2) two communes ------------------------------------------------------------------------

func TestPgTwoTenantsDoNotSeeEachOthersContent(t *testing.T) {
	// RULE 1, THE ASSERTION NO SINGLE-COMMUNE TEST CAN MAKE. The same id is inserted in both communes;
	// a primary key without `tenant_id` would make the second INSERT collide with the first, and a
	// read that forgot the predicate would return both.
	t1, t2 := uniqueTenants(t)
	at := time.Now().UTC()

	insertContentRow(t, t1, "nd-chung", "tin-tuc", "Tin của xã 1", "dang-hien", "thu-cong", "", at)
	insertContentRow(t, t2, "nd-chung", "tin-tuc", "Tin của xã 2", "dang-hien", "thu-cong", "", at)

	s := newRealContentItemStore(t)

	res1, err := s.List(tenantCtx(tenant.ID(t1)), ContentItemFilter{}, realContentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc xã 1: %v", err)
	}
	for _, c := range res1.Items {
		if c.Title == "Tin của xã 2" {
			t.Fatal("xã 1 đọc được nội dung của xã 2 — VI PHẠM CÁCH LY (luật 1)")
		}
	}

	one, err := s.ByID(tenantCtx(tenant.ID(t2)), "nd-chung")
	if err != nil {
		t.Fatalf("đọc chi tiết xã 2: %v", err)
	}
	if one.Title != "Tin của xã 2" {
		t.Errorf("tuyến chi tiết trả %q cho xã 2 — hai xã cùng id phải là hai dòng", one.Title)
	}
}

func TestPgTwoTenantsShareOnePortalArticleID(t *testing.T) {
	// `UNIQUE (tenant_id, nguon_id_ngoai)` COMPOSED WITH THE COMMUNE. Two communes whose portals hand
	// out the same article id are two different articles, and a single-column key would make the
	// second commune unable to take its own news — a defect that appears on the day of the second
	// commune, when fixing it is a migration on live data.
	t1, t2 := uniqueTenants(t)
	at := time.Now().UTC()

	insertContentRow(t, t1, "nd-1", "tin-tuc", "Bài của xã 1", "dang-hien", "dong-bo-cong", "cong-42", at)
	insertContentRow(t, t2, "nd-2", "tin-tuc", "Bài của xã 2", "dang-hien", "dong-bo-cong", "cong-42", at)
}

// --- (3) the deduplication key survives a soft delete ---------------------------------------------

func TestPgPortalArticleIDStaysTakenAfterSoftDelete(t *testing.T) {
	// §10.1 AND RULE 7, INVARIANT 3 IN ONE CASE, and it is the sharpest one in this file. A member of
	// staff removed a portal article on purpose. The next sync runs in six hours; if the soft-deleted
	// row released its `nguon_id_ngoai`, the article would come straight back — silently, and forever.
	t1, _ := uniqueTenants(t)
	db := openTestDB(t)
	at := time.Now().UTC()

	insertContentRow(t, t1, "nd-1", "tin-tuc", "Bài từ Cổng", "dang-hien", "dong-bo-cong", "cong-99", at)
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		  WHERE tenant_id = $1 AND id = $2`,
		t1, "nd-1", "CB-2026-7K3M9Q", "Bài đăng nhầm"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	_, err := db.Exec(
		`INSERT INTO noi_dung_mini_app
		 (tenant_id, id, loai, tieu_de, ngay_dang, trang_thai, nguon, nguon_id_ngoai, nguoi_tao_ma)
		 VALUES ($1,'nd-2','tin-tuc','Cùng bài ấy',$2,'dang-hien','dong-bo-cong','cong-99','CB-1')`,
		t1, at)
	if err == nil {
		t.Fatal("mã bài trên Cổng được cấp lại sau khi xoá mềm — lượt đồng bộ sau sẽ mang bài đã gỡ quay về")
	}

	// AND THE DELETED ROW IS GONE FROM THE LIST, through the real partial index (rule 7, invariant 2).
	s := newRealContentItemStore(t)
	res, err := s.List(tenantCtx(tenant.ID(t1)), ContentItemFilter{}, realContentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ: %v", err)
	}
	for _, c := range res.Items {
		if c.ID == "nd-1" {
			t.Error("dòng đã xoá mềm vẫn nằm trong danh sách (luật 7, bất biến 2)")
		}
	}
}

// --- (4) the CHECK constraints ------------------------------------------------------------------

func TestPgCheckConstraintsRefuseValuesOutsideSpec(t *testing.T) {
	t1, _ := uniqueTenants(t)
	db := openTestDB(t)
	at := time.Now().UTC()

	insert := func(typ, status, source string, externalRef any, handEdited bool) error {
		_, err := db.Exec(
			`INSERT INTO noi_dung_mini_app
			 (tenant_id, id, loai, tieu_de, ngay_dang, trang_thai, nguon, nguon_id_ngoai,
			  da_sua_tay, nguoi_tao_ma)
			 VALUES ($1,$2,$3,'Tiêu đề',$4,$5,$6,$7,$8,'CB-1')`,
			t1, "nd-"+typ+status+source+time.Now().Format("150405.000000000"),
			typ, at, status, source, externalRef, handEdited)
		return err
	}

	// A SEVENTH `loai` — a tab no screen draws, holding rows nobody can find.
	if err := insert("podcast", "an", "thu-cong", nil, false); err == nil {
		t.Error("`loai` ngoài sáu mã của §5 mà vẫn chèn được")
	}
	// A FOURTH state — §6 draws three chips.
	if err := insert("tin-tuc", "da-duyet", "thu-cong", nil, false); err == nil {
		t.Error("`trang_thai` ngoài ba giá trị của §6 mà vẫn chèn được")
	}
	// PROVENANCE AND THE PORTAL ID ARE ONE FACT, both directions. A synced article with no portal id
	// cannot be deduplicated, so the next run imports it again, and again.
	if err := insert("tin-tuc", "an", "dong-bo-cong", nil, false); err == nil {
		t.Error("bài `dong-bo-cong` không có `nguon_id_ngoai` mà vẫn chèn được — §10.1 khử trùng bằng cột ấy")
	}
	// And a hand-composed article carrying one is a row a sync would believe it owns.
	if err := insert("tin-tuc", "an", "thu-cong", "cong-1", false); err == nil {
		t.Error("bài `thu-cong` mang `nguon_id_ngoai` mà vẫn chèn được")
	}
	// §10.4's flag on a row the sync never touches claims a protection that protects nothing.
	if err := insert("tin-tuc", "an", "thu-cong", nil, true); err == nil {
		t.Error("bài soạn tay mang cờ `da_sua_tay` mà vẫn chèn được")
	}
	// The ordinary rows must still go in.
	if err := insert("banner", "cho-duyet", "dong-bo-cong", "cong-ok-1", true); err != nil {
		t.Errorf("dòng hợp lệ bị từ chối: %v", err)
	}
	if err := insert("truyen-thanh", "dang-hien", "thu-cong", nil, false); err != nil {
		t.Errorf("dòng hợp lệ bị từ chối: %v", err)
	}
}

// --- (5) the immutability trigger ------------------------------------------------------------------

func TestPgProvenanceImmutableAndHandEditedCannotBeCleared(t *testing.T) {
	t1, _ := uniqueTenants(t)
	db := openTestDB(t)
	at := time.Now().UTC()

	insertContentRow(t, t1, "nd-1", "tin-tuc", "Bài từ Cổng", "dang-hien", "dong-bo-cong", "cong-7", at)

	// PROVENANCE IS IMMUTABLE: a row that could change it would move in or out of §10.1's
	// deduplication and, worse, out of §10.4's protection.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET nguon = 'thu-cong' WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err == nil {
		t.Error("đổi được `nguon` của một bài đã có")
	}
	// THE PORTAL ID IS IMMUTABLE ONCE SET: freeing it imports the same article again as a second row.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET nguon_id_ngoai = 'cong-8' WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err == nil {
		t.Error("đổi được `nguon_id_ngoai` của một bài đã có")
	}

	// §10.4: false -> true IS THE ORDINARY ACT and must be allowed.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET da_sua_tay = true WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err != nil {
		t.Fatalf("không đặt được cờ da_sua_tay: %v", err)
	}
	// true -> false IS REFUSED: clearing it withdraws a promise made to a member of staff, and the
	// next run overwrites their correction with nobody being told.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET da_sua_tay = false WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err == nil {
		t.Error("gỡ được cờ `da_sua_tay` — lượt đồng bộ sau sẽ ghi đè bản sửa tay (§10.4)")
	}

	// THE AUTHOR AND THE CREATION TIME ARE THE ANCHORS OF THE RECORD.
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET nguoi_tao_ma = 'CB-KHAC' WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err == nil {
		t.Error("đổi được người tạo")
	}
}

func TestPgViewCountCannotDecrease(t *testing.T) {
	// A figure that can go down with no event behind it is the defect rule 10, invariant 3 describes
	// for `qua_han`, in a different column — and the report going upward is what reads it.
	t1, _ := uniqueTenants(t)
	db := openTestDB(t)

	insertContentRow(t, t1, "nd-1", "tin-tuc", "Bài", "dang-hien", "thu-cong", "", time.Now().UTC())

	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET luot_xem = 10 WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err != nil {
		t.Fatalf("không tăng được lượt xem: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET luot_xem = 3 WHERE tenant_id = $1 AND id = 'nd-1'`,
		t1); err == nil {
		t.Error("giảm được lượt xem")
	}
}

// --- (6) the category tree ----------------------------------------------------------------------

func TestPgCategorySlugUniquePerTenantAndNeverReissued(t *testing.T) {
	t1, t2 := uniqueTenants(t)
	db := openTestDB(t)

	insertCategoryRow(t, t1, "dm-1", "Chuyển đổi số", "chuyen-doi-so", "", 1)

	// THE SAME SLUG IN ANOTHER COMMUNE IS A DIFFERENT CATEGORY — that is what makes the key composite.
	insertCategoryRow(t, t2, "dm-2", "Chuyển đổi số", "chuyen-doi-so", "", 1)

	// TWICE IN ONE COMMUNE IS REFUSED.
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, thu_tu)
		 VALUES ($1,'dm-3','Chuyển đổi số (2)','chuyen-doi-so',2)`, t1); err == nil {
		t.Error("slug trùng trong một xã mà vẫn chèn được")
	}

	// AND IT STAYS TAKEN AFTER A SOFT DELETE (rule 7, invariant 3): every article filed under it
	// holds the slug as a value, and reissuing it would refile them under a new, unrelated category.
	if _, err := db.Exec(
		`UPDATE danh_muc_mini_app SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'gộp mục'
		  WHERE tenant_id = $1 AND id = 'dm-1'`, t1); err != nil {
		t.Fatalf("xoá mềm danh mục: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, thu_tu)
		 VALUES ($1,'dm-4','Chuyển đổi số (mới)','chuyen-doi-so',3)`, t1); err == nil {
		t.Error("slug đã cấp được cấp lại sau khi xoá mềm (luật 7, bất biến 3)")
	}
}

func TestPgCategoryTreeForeignKeyAndNoSelfParent(t *testing.T) {
	t1, t2 := uniqueTenants(t)
	db := openTestDB(t)

	insertCategoryRow(t, t1, "dm-goc", "Danh mục", "danh-muc", "", 1)
	insertCategoryRow(t, t1, "dm-con", "Chuyển đổi số", "chuyen-doi-so", "dm-goc", 2)

	// A PARENT THAT DOES NOT EXIST IS REFUSED.
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, cha_id, thu_tu)
		 VALUES ($1,'dm-x','Mục lạ','muc-la','dm-khong-co',1)`, t1); err == nil {
		t.Error("cha không tồn tại mà vẫn chèn được")
	}
	// A PARENT IN ANOTHER COMMUNE IS REFUSED — the key is composed with `tenant_id`, so a child
	// cannot reach across authorities even if two ids ever collided.
	insertCategoryRow(t, t2, "dm-cua-xa-2", "Mục của xã 2", "muc-cua-xa-2", "", 1)
	if _, err := db.Exec(
		`INSERT INTO danh_muc_mini_app (tenant_id, id, ten, slug, cha_id, thu_tu)
		 VALUES ($1,'dm-y','Mục lạ','muc-la-2','dm-cua-xa-2',1)`, t1); err == nil {
		t.Error("cha thuộc xã khác mà vẫn chèn được — CÁCH LY (luật 1) hỏng ở khoá ngoại")
	}
	// A CATEGORY CANNOT BE ITS OWN PARENT. This CHECK catches the one-node cycle only; a cycle through
	// two or more rows is the write path's job — migration 0006 records the obligation.
	if _, err := db.Exec(
		`UPDATE danh_muc_mini_app SET cha_id = 'dm-con' WHERE tenant_id = $1 AND id = 'dm-con'`,
		t1); err == nil {
		t.Error("một danh mục tự làm cha của chính nó mà vẫn ghi được")
	}
}

func TestPgContentForeignKeyStaysWithinOwnTenantCategory(t *testing.T) {
	t1, t2 := uniqueTenants(t)
	db := openTestDB(t)
	at := time.Now().UTC()

	insertCategoryRow(t, t2, "dm-cua-xa-2", "Mục của xã 2", "muc-cua-xa-2", "", 1)

	if _, err := db.Exec(
		`INSERT INTO noi_dung_mini_app
		 (tenant_id, id, loai, danh_muc_id, tieu_de, ngay_dang, trang_thai, nguon, nguoi_tao_ma)
		 VALUES ($1,'nd-1','tin-tuc','dm-cua-xa-2','Tiêu đề',$2,'an','thu-cong','CB-1')`,
		t1, at); err == nil {
		t.Error("bài của xã 1 xếp được vào danh mục của xã 2 — CÁCH LY (luật 1) hỏng ở khoá ngoại")
	}
}

// --- (7) the filters, against the real indexes ------------------------------------------------------

func TestPgFilterByTypeCategoryAndTitle(t *testing.T) {
	t1, _ := uniqueTenants(t)
	at := time.Now().UTC()

	insertCategoryRow(t, t1, "dm-1", "Chuyển đổi số", "chuyen-doi-so", "", 1)
	insertContentRow(t, t1, "nd-tin", "tin-tuc", "Xã khai giảng năm học mới", "dang-hien", "thu-cong", "", at)
	insertContentRow(t, t1, "nd-banner", "banner", "Khẩu hiệu chào mừng", "dang-hien", "thu-cong", "", at.Add(time.Second))

	s := newRealContentItemStore(t)
	ctx := tenantCtx(tenant.ID(t1))

	res, err := s.List(ctx, ContentItemFilter{Type: "banner"}, realContentFirstPage(t))
	if err != nil {
		t.Fatalf("lọc theo loại: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "nd-banner" {
		t.Errorf("lọc `loai=banner` trả %d dòng: %+v", len(res.Items), res.Items)
	}

	res, err = s.List(ctx, ContentItemFilter{Search: "khai giảng"}, realContentFirstPage(t))
	if err != nil {
		t.Fatalf("tìm theo tiêu đề: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "nd-tin" {
		t.Errorf("tìm `khai giảng` trả %d dòng: %+v", len(res.Items), res.Items)
	}

	// ILIKE IS CASE-INSENSITIVE AND MUST BE — §6's box is `Tìm theo tiêu đề…`, and a member of staff
	// typing lower case must find a title that starts with a capital.
	res, err = s.List(ctx, ContentItemFilter{Search: "KHAI GIẢNG"}, realContentFirstPage(t))
	if err != nil {
		t.Fatalf("tìm không phân biệt hoa thường: %v", err)
	}
	if len(res.Items) != 1 {
		t.Errorf("tìm `KHAI GIẢNG` trả %d dòng, muốn 1", len(res.Items))
	}

	// THE WILDCARD IS ESCAPED: `%` typed in the box is a literal percent sign, not "match everything".
	res, err = s.List(ctx, ContentItemFilter{Search: "%"}, realContentFirstPage(t))
	if err != nil {
		t.Fatalf("tìm ký tự đại diện: %v", err)
	}
	if len(res.Items) != 0 {
		t.Errorf("gõ `%%` vào ô tìm trả %d dòng, muốn 0 — ký tự đại diện phải được thoát", len(res.Items))
	}
}

func TestPgListCategoriesOrdersAndSkipsDeleted(t *testing.T) {
	t1, _ := uniqueTenants(t)
	db := openTestDB(t)

	insertCategoryRow(t, t1, "dm-b", "Mục B", "muc-b", "", 2)
	insertCategoryRow(t, t1, "dm-a", "Mục A", "muc-a", "", 1)
	insertCategoryRow(t, t1, "dm-x", "Mục đã gỡ", "muc-da-go", "", 3)
	if _, err := db.Exec(
		`UPDATE danh_muc_mini_app SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'gộp'
		  WHERE tenant_id = $1 AND id = 'dm-x'`, t1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	list, err := newRealContentCategoryStore(t).List(tenantCtx(tenant.ID(t1)))
	if err != nil {
		t.Fatalf("đọc danh mục: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("số danh mục = %d, muốn 2 (dòng đã xoá mềm phải biến mất)", len(list))
	}
	if list[0].ID != "dm-a" || list[1].ID != "dm-b" {
		t.Errorf("thứ tự = %q, %q — muốn theo thu_tu", list[0].ID, list[1].ID)
	}
}

// --- (8) the write path, end to end ---------------------------------------------------------------

func TestPgInsertAndUpdateThroughRealStore(t *testing.T) {
	t1, _ := uniqueTenants(t)
	s := newRealContentItemStore(t)
	ctx := tenantCtx(tenant.ID(t1))
	at := time.Now().UTC()

	insertCategoryRow(t, t1, "dm-1", "Chuyển đổi số", "chuyen-doi-so", "", 1)

	err := pkgstore.New(openTestDB(t)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.ContentItem{
			ID: "nd-1", Type: domain.ContentTypeNews, CategoryID: "dm-1",
			Title: "Xã khai giảng", Summary: "", Body: "<p>Toàn văn</p>",
			PublishedOn: at, Status: domain.ContentStatusHidden, AuthorCode: "CB-2026-7K3M9Q",
		})
	})
	if err != nil {
		t.Fatalf("chèn qua kho thật: %v", err)
	}

	one, err := s.ByID(ctx, "nd-1")
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	// `nguon` WAS WRITTEN AS A LITERAL — the store binds no parameter for it.
	if one.Source != domain.ContentSourceManual || one.SourceRef != "" {
		t.Errorf("xuất xứ = %q / %q, muốn thu-cong và rỗng", one.Source, one.SourceRef)
	}
	// AN EMPTY SUMMARY WAS STORED AS NULL AND READS BACK AS "" — one spelling of "nothing".
	if one.Summary != "" {
		t.Errorf("tóm tắt = %q, muốn rỗng", one.Summary)
	}
	if one.ViewCount != 0 || one.HandEdited {
		t.Errorf("dòng mới phải có 0 lượt xem và chưa sửa tay: %+v", one)
	}

	err = pkgstore.New(openTestDB(t)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		before, err := s.ByIDForUpdate(ctx, tx, "nd-1")
		if err != nil {
			return err
		}
		before.Title = "Xã khai giảng năm học mới"
		before.Status = domain.ContentStatusVisible
		return s.Update(ctx, tx, before)
	})
	if err != nil {
		t.Fatalf("cập nhật qua kho thật: %v", err)
	}

	one, err = s.ByID(ctx, "nd-1")
	if err != nil {
		t.Fatalf("đọc lại sau sửa: %v", err)
	}
	if one.Title != "Xã khai giảng năm học mới" || !one.IsVisibleToCitizens() {
		t.Errorf("sau khi sửa = %+v", one)
	}
}

func TestPgUpdateSoftDeletedRowIs404(t *testing.T) {
	// `AND deleted_at IS NULL` IS WHAT MAKES EDITING A DELETED ITEM A 404 rather than a resurrection.
	t1, _ := uniqueTenants(t)
	db := openTestDB(t)
	ctx := tenantCtx(tenant.ID(t1))

	insertContentRow(t, t1, "nd-1", "tin-tuc", "Bài", "an", "thu-cong", "", time.Now().UTC())
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'gỡ'
		  WHERE tenant_id = $1 AND id = 'nd-1'`, t1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	s := newRealContentItemStore(t)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := s.ByIDForUpdate(ctx, tx, "nd-1")
		return err
	})
	if err == nil {
		t.Fatal("đọc được dòng đã xoá mềm để sửa")
	}
}

// tenantCtx lives in map_asset_type_test.go, next to the first fake driver, and is shared with
// this file. ONE way to put a commune into a context, not two: the second copy is the one that ends
// up subtly different from what the edge really does.
var _ = context.Background
