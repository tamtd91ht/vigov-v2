package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Integration tests for the citizen notification ledger, against a real PostgreSQL.
//
// THE OTHER HALF OF thong_bao_gui_cong_dan_test.go, NOT A REPLACEMENT FOR IT. That file runs a
// fake driver which RECORDS statements and reports whatever row count the test asked for. It
// proves the Go side — the positional scan, the ceiling refusal, the wrapped errors — and it runs
// everywhere, always. What it CANNOT prove is anything the database decides, and for this table
// the database decides almost everything that matters:
//
//  1. the UNIQUE (tenant_id, khoa_lan_gui) constraint is what makes ON CONFLICT DO NOTHING mean
//     "this citizen has already been told". Without a real server, "idempotent" is a claim about
//     a boolean the test itself supplied;
//  2. the same key in TWO communes must both be accepted — a single-column unique key would
//     silently make the second commune's notification a duplicate of the first's (rule 1,
//     invariant 6). No single-commune test can show this;
//  3. the CHECK that refuses an unmasked recipient. This is the constraint standing between this
//     table and a whole commune's phone numbers in every backup (rule 3);
//  4. the trigger that refuses to rewrite which record, which transition or whom a notification
//     was about, and refuses to move a delivered notification back;
//  5. every column in cotThongBao — and the table name — exists as migration 0004 creates it.
//     THIS IS THE ASSERTION THE FAKE DRIVER STRUCTURALLY CANNOT MAKE.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, AND THAT IS NOT A FOOTNOTE — read it as a warning about
// this repository. No PostgreSQL is reachable from this build environment, so on every machine
// anybody has run this on so far, every test in this file SKIPS while the package still prints
// `ok`. A green `go test` here means this file COMPILES. Nothing in it may be described as
// verified until somebody sets the DSN and says what came out.
//
// TestMain, moKetNoi and xaRieng live in loai_tai_nguyen_ban_do_pg_test.go and are shared: ONE
// schema per run, built by the REAL migration runner, so a migration added later is exercised the
// day it is added rather than the day somebody remembers to extend a list.
//
// ONE REFUSAL IS DELIBERATELY NOT TESTED HERE, and it is named rather than quietly skipped: that
// a hard delete of a ledger row is refused by the trigger. Writing that test means writing the
// statement, and `data_safety_guard` blocks the literal in a Go file — it cannot tell a test that
// ASSERTS the refusal from code that performs the deletion. Routing around a guard is worse than
// the missing case, so the case is reported instead. Until the guard can tell the two apart, that
// clause of the trigger is covered by reading migration 0004 and by nothing else.

// themThongBao inserts one ledger row directly. EVERY COLUMN IS PASSED EXPLICITLY rather than
// left to its default: a test that relied on a default could not tell "the column is read" from
// "the column is always its default".
func themThongBao(t *testing.T, tenantID, id, khoa, doiTuongMa, moc string, lan int,
	nguoiNhan, trangThai string) {
	t.Helper()
	db := moKetNoi(t)
	thamSo := fmt.Sprintf(
		`{"ma_tra_cuu":%q,"moc_nhan":"Đã đóng","viec_tiep_theo":"Ông/bà có thể đánh giá kết quả trong ứng dụng."}`,
		doiTuongMa)

	var guiLuc any
	if trangThai == string(domain.DaGui) {
		guiLuc = time.Now().UTC()
	}
	_, err := db.Exec(
		`INSERT INTO thong_bao_gui_cong_dan
		 (tenant_id, id, khoa_lan_gui, doi_tuong_loai, doi_tuong_ma, moc, lan, kenh,
		  nguoi_nhan_ma, mau_ma, tham_so, trang_thai, gui_luc)
		 VALUES ($1,$2,$3,'phieu-phan-anh',$4,$5,$6,'zalo-zns',$7,'zns-phan-anh-doi-trang-thai',$8,$9,$10)`,
		tenantID, id, khoa, doiTuongMa, moc, lan, nguoiNhan, thamSo, trangThai, guiLuc)
	if err != nil {
		t.Fatalf("thêm thông báo %q: %v", id, err)
	}
}

func khoaMau(doiTuongMa, moc string, lan int, nguoiNhan string) string {
	k, err := domain.KhoaLanGui(domain.DoiTuongPhieuPhanAnh, doiTuongMa, moc, lan, nguoiNhan,
		domain.KenhZaloZNS)
	if err != nil {
		panic(err)
	}
	return k
}

func khoThongBaoThat(t *testing.T) *ThongBaoGuiCongDanStore {
	t.Helper()
	return NewThongBaoGuiCongDanStore(pkgstore.New(moKetNoi(t)))
}

// --- (5) the column list against the real schema -------------------------------------------------

func TestPgThongBaoCotTrongMaKhopVoiLuocDoThat(t *testing.T) {
	// THE REASON THIS FILE EXISTS. cotThongBao is a string, and the fake driver builds its rows
	// from that same string — so a column renamed in a later migration, or misspelled here from
	// the start, is invisible to every test that does not touch a real schema. It would surface
	// as a failed consumer with "column ... does not exist" in a log nobody is reading yet, while
	// citizens quietly stop being notified.
	db := moKetNoi(t)

	co := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'thong_bao_gui_cong_dan'`)
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
		t.Fatal("bảng thong_bao_gui_cong_dan không tồn tại — tên bảng trong kho sai, hoặc migration 0004 chưa chạy")
	}

	var thieu []string
	for _, c := range strings.Split(cotThongBao, ",") {
		if c = strings.TrimSpace(c); c != "" && !co[c] {
			thieu = append(thieu, c)
		}
	}
	// Not in the SELECT list, but every statement in this store is built around them.
	for _, c := range []string{"tenant_id", "deleted_at", "cap_nhat_luc"} {
		if !co[c] {
			thieu = append(thieu, c)
		}
	}
	// AND THE TWO COLUMNS THAT MUST NOT EXIST. This is the assertion nobody thinks to write, and
	// it is the one guarding the expensive mistake: a raw phone number or a rendered message body
	// in this table puts an entire commune's personal data into every backup, permanently
	// (rule 3, and migration 0004's PERSONAL DATA block). Adding either turns this red on the day
	// it is added, not on the day somebody audits the schema.
	for _, cam := range []string{"so_dien_thoai", "dien_thoai", "nguoi_nhan_sdt", "noi_dung", "noi_dung_da_render"} {
		if co[cam] {
			t.Errorf("cột %q có trong bảng — sổ thông báo đang giữ dữ liệu cá nhân thô", cam)
		}
	}
	if len(thieu) > 0 {
		sort.Strings(thieu)
		t.Fatalf("cột không có trong bảng thật: %v — câu lệnh sẽ hỏng ở lần gọi thật đầu tiên", thieu)
	}
}

// --- (1) the constraint that makes the idempotency real -------------------------------------------

func TestPgChenLanHaiCungKhoaThiKhongGhiThem(t *testing.T) {
	// THE MOST IMPORTANT CASE IN THIS FILE. Everything the unit suite proves about redelivery
	// rests on UNIQUE (tenant_id, khoa_lan_gui) actually existing: the fake driver reports
	// whatever row count the test hands it, so with no real constraint "idempotent" is a claim
	// about a boolean rather than about the database.
	//
	// What is simulated here is exactly what an at-least-once queue does: the same fact, twice,
	// with a DIFFERENT row id — because a producer that republished after a crash mints a new one.
	// A key built from the message id would let the second one through and send the citizen the
	// same message twice.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))
	kho := khoThongBaoThat(t)

	tb := thongBaoThu()
	tb.KhoaLanGui = khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu)
	tb.Moc = "da-dong"

	var lan1, lan2 bool
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		lan1, err = kho.Chen(ctx, tx, tb)
		return err
	}); err != nil {
		t.Fatalf("lần một: %v", err)
	}

	tb2 := tb
	tb2.ID = tb.ID + "X" // a different row id, the same fact
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		lan2, err = kho.Chen(ctx, tx, tb2)
		return err
	}); err != nil {
		t.Fatalf("lần hai phải là no-op, nhận lỗi: %v", err)
	}

	if !lan1 {
		t.Error("lần ghi đầu tiên báo là đã có")
	}
	if lan2 {
		t.Fatal("giao lại cùng một sự kiện vẫn ghi thêm một dòng — công dân sẽ nhận hai tin")
	}

	ra, err := kho.TheoDoiTuong(ctx, domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("sổ có %d dòng cho một sự kiện, muốn 1", len(ra))
	}
}

func TestPgCungKhoaOHaiXaKhacNhauDeuGhiDuoc(t *testing.T) {
	// RULE 1, INVARIANT 6: every unique key is COMPOSITE with tenant_id. A single-column key on
	// `khoa_lan_gui` would look correct for as long as one commune is live and then silently make
	// the second commune's notification a duplicate of the first's — a citizen of commune B never
	// told, with nothing in any log saying so.
	//
	// The keys here are IDENTICAL on purpose. Two communes really can mint the same key: the
	// lookup code series is per commune (rule 1, invariant 6) and the citizen identity is
	// platform-wide, so one person dealing with two communes about two petitions that happen to
	// share a code is not a contrived case.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	khoa := khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu)

	themThongBao(t, xa, "tb-a", khoa, maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")
	themThongBao(t, xaKhac, "tb-b", khoa, maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	ra, err := kho.TheoDoiTuong(ctxXa(tenant.ID(xa)), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "tb-a" {
		t.Fatalf("RÒ RỈ hoặc thiếu dòng: %+v", ra)
	}
}

// --- (2) one commune's ledger never reaches another ------------------------------------------------

func TestPgTheoDoiTuongChiDocXaCuaChinhMinh(t *testing.T) {
	// The predicate that keeps two public authorities apart is a single `WHERE tenant_id = $1`
	// bound by Scoped.Query, and no test with one commune in it can show whether it is there.
	// What would leak is the list of what another authority told a citizen — who it notified, when
	// and about what.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themThongBao(t, xa, "tb-cua-a", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")
	themThongBao(t, xaKhac, "tb-cua-b", khoaMau(maPhieuThu, "dang-xu-ly", 1, maCongDanThu),
		maPhieuThu, "dang-xu-ly", 1, maCongDanThu, "cho-gui")

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	ra, err := kho.TheoDoiTuong(ctxXa(tenant.ID(xaKhac)), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "tb-cua-b" {
		t.Fatalf("RÒ RỈ: xã B nhận được sổ của xã A: %+v", ra)
	}
}

func TestPgTheoDoiTuongBoQuaDongDaXoaMem(t *testing.T) {
	// Rule 7, invariant 2: a soft-deleted row leaves EVERY read path. The UPDATE below is the only
	// way a row goes at all — the trigger refuses a hard delete outright, which is itself the
	// reason this read may never rely on rows disappearing physically.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThongBao(t, xa, "tb-con", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")
	themThongBao(t, xa, "tb-xoa", khoaMau(maPhieuThu, "dang-xu-ly", 1, maCongDanThu),
		maPhieuThu, "dang-xu-ly", 1, maCongDanThu, "cho-gui")

	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET deleted_at = now(), deleted_by = $1, delete_reason = $2
		  WHERE tenant_id = $3 AND id = $4`,
		"CB-TEST", "test", xa, "tb-xoa"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	ra, err := kho.TheoDoiTuong(ctxXa(tenant.ID(xa)), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "tb-con" {
		t.Fatalf("dòng đã xoá mềm vẫn lọt vào sổ: %+v", ra)
	}
}

func TestPgTheoDoiTuongMoiNhatTruoc(t *testing.T) {
	// NEWEST FIRST, because the last thing the citizen was told is what they are holding when they
	// ring. The two rows are inserted oldest-first on purpose, so an ORDER BY that lost its
	// direction would be free to return them as inserted.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThongBao(t, xa, "tb-cu", khoaMau(maPhieuThu, "da-tiep-nhan", 1, maCongDanThu),
		maPhieuThu, "da-tiep-nhan", 1, maCongDanThu, "cho-gui")
	// A distinct `tao_luc`, since the column defaults to now() and two inserts in the same
	// transaction-less millisecond would leave the order to the tie-break alone.
	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET cap_nhat_luc = now() WHERE tenant_id = $1 AND id = $2`,
		xa, "tb-cu"); err != nil {
		t.Fatalf("chạm dòng cũ: %v", err)
	}
	themThongBao(t, xa, "tb-moi", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	ra, err := kho.TheoDoiTuong(ctxXa(tenant.ID(xa)), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d dòng, muốn 2", len(ra))
	}
	if ra[0].TaoLuc.Before(ra[1].TaoLuc) {
		t.Fatalf("sổ không xếp mới nhất trước: %v rồi %v", ra[0].TaoLuc, ra[1].TaoLuc)
	}
}

// --- (3) the constraint standing between this table and a commune's phone numbers -------------------

func TestPgSoNguoiNhanChuaCheBiCSDLTuChoi(t *testing.T) {
	// THE EXPENSIVE ONE. A notification ledger is the classic place a whole commune's phone
	// numbers end up in one table, from which they reach every backup and every log aggregator —
	// with no lawful basis under Decree 13/2023 and no way to undo it, because the copies are
	// already made.
	//
	// The Go layer refuses this too (domain.KetQuaGui.KiemTra). BOTH LAYERS ON PURPOSE: this one
	// holds against a statement typed at a psql prompt during an incident, which is exactly when
	// the mistake gets made. The number is the agreed fake one (rule 3, invariant 5); what makes
	// it fail is that it carries no mask.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThongBao(t, xa, "tb-che", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")

	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET nguoi_nhan_che = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "tb-che", "0900000000"); err == nil {
		t.Fatal("CSDL nhận một số chưa che — sổ thông báo đang là kho số điện thoại của cả xã")
	}

	// And the masked form is accepted, so the constraint refuses the right thing rather than
	// everything.
	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET nguoi_nhan_che = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "tb-che", "09****0000"); err != nil {
		t.Fatalf("số đã che mà vẫn bị từ chối: %v", err)
	}
}

func TestPgThamSoThieuKhoaHoacLechMaThiBiTuChoi(t *testing.T) {
	// The lookup code appears twice — as `doi_tuong_ma` and as a template parameter — because the
	// parameter set is what Zalo approved. Two copies of one fact are safe only when they cannot
	// drift, so the CHECK ties them together. Without it, a notification could carry one citizen's
	// code in its body while the ledger files it under another's.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	ca := []struct {
		ten    string
		thamSo string
	}{
		{"thiếu việc tiếp theo", `{"ma_tra_cuu":"` + maPhieuThu + `","moc_nhan":"Đã đóng"}`},
		{"thiếu mốc", `{"ma_tra_cuu":"` + maPhieuThu + `","viec_tiep_theo":"Mời ông/bà đánh giá."}`},
		{"mã trong tham số lệch mã hồ sơ",
			`{"ma_tra_cuu":"PA-2026-KHAC00","moc_nhan":"Đã đóng","viec_tiep_theo":"Mời ông/bà đánh giá."}`},
	}
	for i, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			_, err := db.Exec(
				`INSERT INTO thong_bao_gui_cong_dan
				 (tenant_id, id, khoa_lan_gui, doi_tuong_loai, doi_tuong_ma, moc, lan, kenh,
				  nguoi_nhan_ma, mau_ma, tham_so, trang_thai)
				 VALUES ($1,$2,$3,'phieu-phan-anh',$4,'da-dong',1,'zalo-zns',$5,'m',$6,'cho-gui')`,
				xa, fmt.Sprintf("tb-ts-%d", i), fmt.Sprintf("khoa-ts-%d", i), maPhieuThu,
				maCongDanThu, c.thamSo)
			if err == nil {
				t.Fatalf("CSDL nhận tham số %q", c.ten)
			}
		})
	}
}

func TestPgDaGuiMaThieuMocGioThiBiTuChoi(t *testing.T) {
	// "Sent" with no date is a claim with nothing behind it, and the date is the whole answer when
	// an inspection asks when the citizen was told.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThongBao(t, xa, "tb-gio", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")

	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET trang_thai = 'da-gui' WHERE tenant_id = $1 AND id = $2`,
		xa, "tb-gio"); err == nil {
		t.Fatal("CSDL nhận trạng thái đã gửi mà không có mốc giờ")
	}
}

// --- (4) the immutability trigger ------------------------------------------------------------------

func TestPgCotDinhDanhKhongSuaDuoc(t *testing.T) {
	// THE CLAUSE THAT MAKES THIS LEDGER EVIDENCE RATHER THAN A CACHE. Changing which record, which
	// transition or whom a notification was about turns a true statement about one citizen into a
	// false statement about another, and nothing downstream could tell.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThongBao(t, xa, "tb-bb", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")

	cot := []string{"doi_tuong_ma", "moc", "nguoi_nhan_ma", "khoa_lan_gui", "kenh", "mau_ma"}
	for _, c := range cot {
		t.Run(c, func(t *testing.T) {
			_, err := db.Exec(fmt.Sprintf(
				`UPDATE thong_bao_gui_cong_dan SET %s = 'x' WHERE tenant_id = $1 AND id = $2`, c),
				xa, "tb-bb")
			if err == nil {
				t.Fatalf("cột %q sửa được — có thể viết lại xã đã nói gì với ai", c)
			}
		})
	}
}

func TestPgDaGuiKhongQuayNguocDuoc(t *testing.T) {
	// A message the channel accepted cannot become unsent: the citizen has it. Allowing it would
	// let a failed retry downgrade a successful send back into the retry set and produce a second
	// message.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themThongBao(t, xa, "tb-daigui", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "da-gui")

	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET trang_thai = 'that-bai' WHERE tenant_id = $1 AND id = $2`,
		xa, "tb-daigui"); err == nil {
		t.Fatal("một thông báo đã gửi lại trở thành chưa gửi")
	}
}

func TestPgGhiKetQuaKhongDungToiDongDaGui(t *testing.T) {
	// THE IDEMPOTENCY OF THE OUTCOME PATH, against the real trigger. Reporting the same success
	// twice must match no row and change nothing — NOT hit the trigger and fail the caller's whole
	// transaction, which would roll back an unrelated business write sharing it.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))

	themThongBao(t, xa, "tb-kq", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "da-gui")

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	var doi bool
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		doi, err = kho.GhiKetQua(ctx, tx, "tb-kq", domain.KetQuaGui{
			TrangThai: domain.DaGui, GuiLuc: time.Now().UTC(),
			NguoiNhanChe: "09****0000", SoLanThu: 2,
		})
		return err
	})
	if err != nil {
		t.Fatalf("báo lại một lần gửi đã thành công phải là no-op, nhận lỗi: %v", err)
	}
	if doi {
		t.Error("dòng đã gửi vẫn bị ghi đè")
	}
}

func TestPgGhiKetQuaMaKhongTonTaiThiBaoRo(t *testing.T) {
	// "No such notification" and "already delivered" lead an operator to opposite conclusions, so
	// the store separates them with a second, narrow read rather than guessing from a row count.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(ctx, tx, "tb-khong-co", domain.KetQuaGui{
			TrangThai: domain.ThatBai, LoiMa: "zns-429", SoLanThu: 3,
		})
		return err
	})
	if !errors.Is(err, ErrThongBaoKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrThongBaoKhongTonTai", err)
	}
}

func TestPgGhiKetQuaKhongXoaSoDaCheDaBiet(t *testing.T) {
	// A first attempt that reached the gateway knows which number it used; a later attempt that
	// failed before resolving one does not, and must not overwrite that fact with nothing. The
	// COALESCE in the statement is what holds this, and it is the kind of clause that gets
	// simplified away by somebody reading it as noise.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))

	themThongBao(t, xa, "tb-coal", khoaMau(maPhieuThu, "da-dong", 1, maCongDanThu),
		maPhieuThu, "da-dong", 1, maCongDanThu, "cho-gui")
	if _, err := db.Exec(
		`UPDATE thong_bao_gui_cong_dan SET nguoi_nhan_che = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, "tb-coal", "09****0000"); err != nil {
		t.Fatalf("đặt số đã che: %v", err)
	}

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(ctx, tx, "tb-coal", domain.KetQuaGui{
			TrangThai: domain.ThatBai, LoiMa: "zns-503", SoLanThu: 2,
		})
		return err
	}); err != nil {
		t.Fatalf("GhiKetQua: %v", err)
	}

	ra, err := kho.TheoDoiTuong(ctx, domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 1 || ra[0].NguoiNhanChe != "09****0000" {
		t.Fatalf("lần thử sau đã xoá mất số đã biết: %+v", ra)
	}
	if ra[0].TrangThai != domain.ThatBai || ra[0].LoiMa != "zns-503" || ra[0].SoLanThu != 2 {
		t.Errorf("kết quả gửi ghi sai: %+v", ra[0])
	}
}

// --- the ceiling, against real rows -----------------------------------------------------------------

func TestPgVuotTranMotHoSoThiTuChoiChuKhongCatBot(t *testing.T) {
	// ONE ROW OVER THE CEILING, the only interesting number: at exactly the ceiling the history is
	// complete and must come back whole, and the LIMIT is ceiling+1 precisely so the extra row is
	// visible. The refusal must carry NO rows — a caller handed a list alongside an error is a
	// caller that renders it, and a partial history of what a commune told a citizen reads as
	// complete.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	for i := 0; i <= TranThongBaoMotHoSo; i++ {
		moc := fmt.Sprintf("moc-%04d", i)
		themThongBao(t, xa, fmt.Sprintf("tb-%04d", i), khoaMau(maPhieuThu, moc, 1, maCongDanThu),
			maPhieuThu, moc, 1, maCongDanThu, "cho-gui")
	}

	kho := NewThongBaoGuiCongDanStore(pkgstore.New(db))
	ra, err := kho.TheoDoiTuong(ctxXa(tenant.ID(xa)), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if !errors.Is(err, ErrQuaNhieuThongBao) {
		t.Fatalf("err = %v, muốn ErrQuaNhieuThongBao", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}
