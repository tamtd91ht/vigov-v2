package store

// Integration tests for the internal announcement book, against a REAL PostgreSQL.
//
// THE OTHER HALF OF announcement_test.go, NOT A REPLACEMENT FOR IT. That file runs a fake
// driver which RECORDS statements and hands back whatever rows the test supplied. It proves the Go
// side — the positional scan, the commune binding, one counter query per page — and it runs
// everywhere, always. What it CANNOT prove is anything the database decides, and for these three
// tables the database decides most of what matters:
//
//  1. every column in announcementColumns — and the table name — exists as migration 0005 creates it.
//     THIS IS THE ASSERTION THE FAKE DRIVER STRUCTURALLY CANNOT MAKE;
//  2. the same announcement id in TWO communes is two different rows. A primary key without
//     `tenant_id` would make the second commune's announcement collide with the first's, and no
//     single-commune test can show it (rule 1, invariant 6);
//  3. the CHECK that ties `trang_thai` to `phat_hanh_luc`, and the one that refuses a mail state on
//     an announcement that never asked for mail;
//  4. the trigger that refuses to edit an ISSUED announcement (§9.2) and the one that refuses to
//     clear an acknowledgement (§9.5);
//  5. the foreign key that stops a recipient row pointing at another commune's announcement;
//  6. that a soft-deleted announcement leaves the list — through the real partial index.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, AND THAT IS NOT A FOOTNOTE — read it as a warning about
// this repository. No PostgreSQL is reachable from this build environment, so on every machine
// anybody has run this on so far, every test in this file SKIPS while the package still prints
// `ok`. A green `go test` here means this file COMPILES. Nothing in it may be described as
// verified until somebody sets the DSN and says what came out.
//
// TestMain, openTestDB and uniqueTenants live in map_asset_type_pg_test.go and are shared: ONE
// schema per run, built by the REAL migration runner, so a migration added later is exercised the
// day it is added rather than the day somebody remembers to extend a list.
//
// ONE REFUSAL IS DELIBERATELY NOT TESTED HERE, and it is named rather than quietly skipped: that a
// hard delete of an announcement is refused by `ho_so_luu_tru_cam_xoa_cung`. Writing that test
// means writing the statement, and `data_safety_guard` blocks the literal in a Go file — it cannot
// tell a test that ASSERTS the refusal from code that performs the deletion. Routing around a
// guard is worse than the missing case, so the case is reported instead. Until the guard can tell
// the two apart, that trigger is covered by reading migration 0005 and by nothing else.

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

// insertAnnouncementRow inserts one announcement directly. EVERY COLUMN IS PASSED EXPLICITLY rather than
// left to its default: a test that relied on a default could not tell "the column is read" from
// "the column is always its default".
func insertAnnouncementRow(t *testing.T, tenantID, id, title, status string,
	pinned, ackRequired, emailRequested bool, createdAt time.Time) {
	t.Helper()
	db := openTestDB(t)

	var issuedAt any
	if status != string(domain.AnnouncementDraft) {
		issuedAt = createdAt
	}
	_, err := db.Exec(
		`INSERT INTO thong_bao
		 (tenant_id, id, tieu_de, noi_dung, trang_thai, ghim, bat_buoc_xac_nhan,
		  gui_thu_dien_tu, trang_thai_thu, nguoi_soan_ma, phat_hanh_luc, tao_luc)
		 VALUES ($1,$2,$3,'Toàn văn nội dung.',$4,$5,$6,$7,'chua-gui','CB-2026-7K3M9Q',$8,$9)`,
		tenantID, id, title, status, pinned, ackRequired, emailRequested, issuedAt, createdAt)
	if err != nil {
		t.Fatalf("thêm thông báo %q: %v", id, err)
	}
}

func insertRecipientRow(t *testing.T, tenantID, announcementID, code string, isNamed bool, acknowledged bool) {
	t.Helper()
	db := openTestDB(t)

	var readAt, ackAt any
	if acknowledged {
		readAt = time.Now().UTC()
		ackAt = time.Now().UTC()
	}
	_, err := db.Exec(
		`INSERT INTO thong_bao_nguoi_nhan
		 (tenant_id, thong_bao_id, nguoi_nhan_ma, dich_danh, da_mo_luc, da_xac_nhan_luc)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, announcementID, code, isNamed, readAt, ackAt)
	if err != nil {
		t.Fatalf("thêm người nhận %q: %v", code, err)
	}
}

func newRealAnnouncementStore(t *testing.T) *AnnouncementStore {
	t.Helper()
	return NewAnnouncementStore(pkgstore.New(openTestDB(t)))
}

func realFirstPage(t *testing.T) pkgpage.Request {
	t.Helper()
	req, err := pkgpage.New(AnnouncementSorts, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return req
}

// --- (1) the column list against the real schema -----------------------------------------------

func TestPgAnnouncementColumnsMatchRealSchema(t *testing.T) {
	// THE REASON THIS FILE EXISTS. announcementColumns is a string, and the fake driver builds its rows
	// from that same string — so a column renamed in a later migration, or misspelled here from the
	// start, is invisible to every test that does not touch a real schema. It would surface as a
	// broken screen with "column ... does not exist" in a log nobody is reading yet.
	db := openTestDB(t)

	have := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'thong_bao'`)
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
		t.Fatal("bảng thong_bao không tồn tại trong schema thật — migration 0005 chưa chạy?")
	}

	for _, c := range strings.Split(announcementColumns, ",") {
		name := strings.TrimSpace(c)
		if name == "" {
			continue
		}
		if !have[name] {
			t.Errorf("cột %q có trong announcementColumns nhưng KHÔNG có trong lược đồ thật", name)
		}
	}
	// The soft-delete triple, checked by name: rule 7, invariant 1 names all three, and a table
	// carrying two of them is a table where a deletion cannot say who did it or why.
	for _, c := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !have[c] {
			t.Errorf("thiếu cột xoá mềm %q (luật 7, bất biến 1)", c)
		}
	}
}

func TestPgRecipientColumnsMatchRealSchema(t *testing.T) {
	db := openTestDB(t)

	have := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'thong_bao_nguoi_nhan'`)
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

	for _, c := range []string{"tenant_id", "thong_bao_id", "nguoi_nhan_ma", "dich_danh",
		"da_mo_luc", "da_xac_nhan_luc", "thu_gui_luc", "thu_loi_ma"} {
		if !have[c] {
			t.Errorf("thiếu cột %q ở thong_bao_nguoi_nhan", c)
		}
	}
	// THE COLUMN §6 SKETCHES AS `thu_loi text` IS NOT HERE UNDER THAT NAME, on purpose: an SMTP
	// failure message quotes the envelope, and the envelope is somebody's address (rule 3). The
	// column stores the provider's CODE and its name says so.
	if have["thu_loi"] {
		t.Error("cột `thu_loi` tồn tại — phải là `thu_loi_ma`: lưu MÃ lỗi, không lưu câu lỗi của nhà cung cấp")
	}
}

// --- (2) two communes, one id ------------------------------------------------------------------

func TestPgTwoTenantsShareOneAnnouncementID(t *testing.T) {
	// A PRIMARY KEY WITHOUT `tenant_id` WOULD MAKE THIS FAIL, and with one commune nothing would
	// ever show it. The second commune simply could not be onboarded (rule 1, invariant 6).
	t1, t2 := uniqueTenants(t)
	id := "tb-" + t1

	at := time.Now().UTC()
	insertAnnouncementRow(t, t1, id, "Thông báo của xã một", "da-phat-hanh", false, false, false, at)
	insertAnnouncementRow(t, t2, id, "Thông báo của xã hai", "da-phat-hanh", false, false, false, at)

	s := newRealAnnouncementStore(t)
	res1, err := s.List(tenant.Into(context.Background(), tenant.ID(t1)), realFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ xã một: %v", err)
	}
	if len(res1.Items) != 1 || res1.Items[0].Title != "Thông báo của xã một" {
		t.Fatalf("xã một đọc được %d thẻ: %+v", len(res1.Items), res1.Items)
	}
	res2, err := s.List(tenant.Into(context.Background(), tenant.ID(t2)), realFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ xã hai: %v", err)
	}
	if len(res2.Items) != 1 || res2.Items[0].Title != "Thông báo của xã hai" {
		t.Fatalf("xã hai đọc được %d thẻ: %+v", len(res2.Items), res2.Items)
	}
}

// --- (3) the CHECK constraints ------------------------------------------------------------------

func TestPgDraftCannotHaveIssuedAt(t *testing.T) {
	// `(trang_thai = 'nhap') = (phat_hanh_luc IS NULL)` — a draft carrying an issue time is a draft
	// that will read as issued the day somebody writes a report over this table.
	tn, _ := uniqueTenants(t)
	db := openTestDB(t)

	_, err := db.Exec(
		`INSERT INTO thong_bao (tenant_id, id, tieu_de, noi_dung, trang_thai, nguoi_soan_ma, phat_hanh_luc)
		 VALUES ($1,'tb-nhap-sai','T','N','nhap','CB-1',now())`, tn)
	if err == nil {
		t.Fatal("CSDL nhận một bản nháp có mốc phát hành")
	}
	if !strings.Contains(err.Error(), "thong_bao_moc_phat_hanh_khop_trang_thai") {
		t.Errorf("lỗi = %v, muốn ràng buộc thong_bao_moc_phat_hanh_khop_trang_thai", err)
	}
}

func TestPgPublishedMustHaveIssuedAt(t *testing.T) {
	tn, _ := uniqueTenants(t)
	db := openTestDB(t)

	_, err := db.Exec(
		`INSERT INTO thong_bao (tenant_id, id, tieu_de, noi_dung, trang_thai, nguoi_soan_ma)
		 VALUES ($1,'tb-phat-sai','T','N','da-phat-hanh','CB-1')`, tn)
	if err == nil {
		t.Fatal("CSDL nhận một thông báo đã phát hành mà không có mốc giờ")
	}
}

func TestPgNoEmailRequestedMeansNoEmailStatus(t *testing.T) {
	// A "mail sent" claim on an announcement that never asked for mail is a claim nothing in this
	// system witnessed — and §3 would tell a member of staff their colleagues were emailed.
	tn, _ := uniqueTenants(t)
	db := openTestDB(t)

	_, err := db.Exec(
		`INSERT INTO thong_bao
		 (tenant_id, id, tieu_de, noi_dung, trang_thai, gui_thu_dien_tu, trang_thai_thu,
		  nguoi_soan_ma, phat_hanh_luc)
		 VALUES ($1,'tb-thu-sai','T','N','da-phat-hanh',false,'da-gui','CB-1',now())`, tn)
	if err == nil {
		t.Fatal("CSDL nhận trạng thái thư `da-gui` trên thông báo không yêu cầu gửi thư")
	}
	if !strings.Contains(err.Error(), "thong_bao_thu_chi_khi_duoc_yeu_cau") {
		t.Errorf("lỗi = %v, muốn ràng buộc thong_bao_thu_chi_khi_duoc_yeu_cau", err)
	}
}

func TestPgAcknowledgedWithoutReadRefused(t *testing.T) {
	tn, _ := uniqueTenants(t)
	at := time.Now().UTC()
	insertAnnouncementRow(t, tn, "tb-xn", "T", "da-phat-hanh", false, true, false, at)

	db := openTestDB(t)
	_, err := db.Exec(
		`INSERT INTO thong_bao_nguoi_nhan
		 (tenant_id, thong_bao_id, nguoi_nhan_ma, dich_danh, da_xac_nhan_luc)
		 VALUES ($1,'tb-xn','CB-A',true,now())`, tn)
	if err == nil {
		t.Fatal("CSDL nhận một người nhận đã xác nhận mà chưa mở")
	}
}

// --- (4) the immutability triggers --------------------------------------------------------------

func TestPgPublishedAnnouncementNotEditable(t *testing.T) {
	// §9.2: staff have already read it, and an acknowledgement must keep pointing at the words that
	// were agreed to. Enforced in the DATABASE, not in the application, because a promise the
	// application makes is a promise a psql session never heard.
	tn, _ := uniqueTenants(t)
	insertAnnouncementRow(t, tn, "tb-batbien", "Tiêu đề gốc", "da-phat-hanh", false, false, false, time.Now().UTC())

	db := openTestDB(t)
	_, err := db.Exec(
		`UPDATE thong_bao SET noi_dung = 'Nội dung đã bị sửa'
		  WHERE tenant_id = $1 AND id = 'tb-batbien'`, tn)
	if err == nil {
		t.Fatal("sửa được nội dung của một thông báo đã phát hành")
	}

	// `ghim` IS STILL EDITABLE — it is a display decision, not the words anybody agreed to.
	if _, err := db.Exec(
		`UPDATE thong_bao SET ghim = true WHERE tenant_id = $1 AND id = 'tb-batbien'`, tn); err != nil {
		t.Errorf("ghim phải sửa được trên thông báo đã phát hành: %v", err)
	}
}

func TestPgPublishedAnnouncementCannotReturnToDraft(t *testing.T) {
	tn, _ := uniqueTenants(t)
	insertAnnouncementRow(t, tn, "tb-velai", "T", "da-phat-hanh", false, false, false, time.Now().UTC())

	db := openTestDB(t)
	_, err := db.Exec(
		`UPDATE thong_bao SET trang_thai = 'nhap', phat_hanh_luc = NULL
		  WHERE tenant_id = $1 AND id = 'tb-velai'`, tn)
	if err == nil {
		t.Fatal("một thông báo đã phát hành quay lại trạng thái nháp")
	}
}

func TestPgRecordedAcknowledgementCannotBeCleared(t *testing.T) {
	// §9.5 keeps an unacknowledged announcement on a person's bell until they acknowledge it. A path
	// that could clear the timestamp could put it back there after the fact, and the `{x}/{y}`
	// figure reported upward would go DOWN with no event behind it.
	tn, _ := uniqueTenants(t)
	insertAnnouncementRow(t, tn, "tb-golai", "T", "da-phat-hanh", false, true, false, time.Now().UTC())
	insertRecipientRow(t, tn, "tb-golai", "CB-A", true, true)

	db := openTestDB(t)
	_, err := db.Exec(
		`UPDATE thong_bao_nguoi_nhan SET da_xac_nhan_luc = NULL
		  WHERE tenant_id = $1 AND thong_bao_id = 'tb-golai' AND nguoi_nhan_ma = 'CB-A'`, tn)
	if err == nil {
		t.Fatal("gỡ được một xác nhận đã ghi")
	}
}

// --- (5) the foreign key ------------------------------------------------------------------------

func TestPgRecipientCannotReachOtherTenantsAnnouncement(t *testing.T) {
	// COMPOSITE WITH `tenant_id`. Without it, a recipient row of commune B could point at commune
	// A's announcement wherever two ids collided — a breach between two authorities that no
	// single-commune test could show.
	t1, t2 := uniqueTenants(t)
	insertAnnouncementRow(t, t1, "tb-fk", "T", "da-phat-hanh", false, false, false, time.Now().UTC())

	db := openTestDB(t)
	_, err := db.Exec(
		`INSERT INTO thong_bao_nguoi_nhan (tenant_id, thong_bao_id, nguoi_nhan_ma, dich_danh)
		 VALUES ($1,'tb-fk','CB-A',true)`, t2)
	if err == nil {
		t.Fatal("người nhận của xã hai gắn được vào thông báo của xã một")
	}
}

// --- (6) the read path, end to end --------------------------------------------------------------

func TestPgListFiltersSoftDeletedAndCountsCorrectPair(t *testing.T) {
	tn, _ := uniqueTenants(t)
	now := time.Now().UTC()

	insertAnnouncementRow(t, tn, "tb-cu", "Thông báo cũ", "da-phat-hanh", false, true, false, now.Add(-2*time.Hour))
	insertAnnouncementRow(t, tn, "tb-moi", "Thông báo mới", "da-phat-hanh", false, true, false, now)
	insertAnnouncementRow(t, tn, "tb-xoa", "Thông báo đã xoá", "da-phat-hanh", false, false, false, now.Add(-time.Hour))

	db := openTestDB(t)
	if _, err := db.Exec(
		`UPDATE thong_bao SET deleted_at = now(), deleted_by = 'CB-1', delete_reason = 'nhập nhầm'
		  WHERE tenant_id = $1 AND id = 'tb-xoa'`, tn); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	insertRecipientRow(t, tn, "tb-cu", "CB-A", true, true)
	insertRecipientRow(t, tn, "tb-cu", "CB-B", true, false)
	insertRecipientRow(t, tn, "tb-cu", "CB-C", false, false)

	res, err := newRealAnnouncementStore(t).List(tenant.Into(context.Background(), tenant.ID(tn)), realFirstPage(t))
	if err != nil {
		t.Fatalf("đọc sổ: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("số thẻ = %d, muốn 2 — dòng đã xoá mềm phải biến mất khỏi MỌI đường đọc", len(res.Items))
	}
	// §2: "mới nhất ở trên".
	if res.Items[0].ID != "tb-moi" || res.Items[1].ID != "tb-cu" {
		t.Errorf("thứ tự = %q, %q; muốn tb-moi trước", res.Items[0].ID, res.Items[1].ID)
	}
	// §3's `{x}/{y}`: one of three acknowledged. The announcement with no recipients produces NO row
	// in the GROUP BY, and `0/0` is the correct answer rather than a missing key.
	if res.Items[1].RecipientCount != 3 || res.Items[1].AckCount != 1 {
		t.Errorf("bộ đếm = %d/%d, muốn 1/3", res.Items[1].AckCount, res.Items[1].RecipientCount)
	}
	if res.Items[0].RecipientCount != 0 || res.Items[0].AckCount != 0 {
		t.Errorf("thẻ chưa có người nhận = %d/%d, muốn 0/0", res.Items[0].AckCount, res.Items[0].RecipientCount)
	}
}

func TestPgInsertThroughStoreWritesInOneTransaction(t *testing.T) {
	// The store's own write methods, against the real schema: the announcement and its recipients
	// land together or not at all, and the recipient list collapses a person named twice into one
	// row — which is what makes §3's `{y}` count people rather than reasons.
	tn, _ := uniqueTenants(t)
	s := newRealAnnouncementStore(t)
	ctx := tenant.Into(context.Background(), tenant.ID(tn))
	at := time.Now().UTC()

	err := pkgstore.New(openTestDB(t)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := s.Insert(ctx, tx, domain.Announcement{
			ID: "tb-store", Title: "Mời họp giao ban", Body: "Kính mời.",
			Status: domain.AnnouncementPublished, AckRequired: true,
			AuthorCode: "CB-2026-7K3M9Q", IssuedAt: at,
		}); err != nil {
			return err
		}
		return s.InsertRecipients(ctx, tx, "tb-store", []domain.AnnouncementRecipient{
			{RecipientCode: "CB-A", IsNamed: true},
			{RecipientCode: "CB-B", IsNamed: true},
			{RecipientCode: "CB-A", IsNamed: true}, // the same person twice — ON CONFLICT DO NOTHING
		})
	})
	if err != nil {
		t.Fatalf("ghi qua store: %v", err)
	}

	res, err := s.List(ctx, realFirstPage(t))
	if err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("số thẻ = %d, muốn 1", len(res.Items))
	}
	if res.Items[0].RecipientCount != 2 {
		t.Errorf("số người nhận = %d, muốn 2 — một người hai lần vẫn là một dòng", res.Items[0].RecipientCount)
	}
	if res.Items[0].IssuedAt.IsZero() {
		t.Error("mốc phát hành không được ghi")
	}
	if res.Items[0].EmailStatus != domain.EmailNotSent {
		t.Errorf("trạng thái thư = %q, muốn chua-gui — kho này chưa có bộ gửi thư", res.Items[0].EmailStatus)
	}
}
