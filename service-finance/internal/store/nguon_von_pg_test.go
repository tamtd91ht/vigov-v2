package store

import (
	"database/sql"
	"sort"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for funding sources, allocations, and the `nguon_von_id` column migration 0007
// adds to chung_tu_giai_ngan — against a real PostgreSQL.
//
// WHY A REAL DATABASE: almost everything asserted here lives in SQL or in the schema itself — the
// `tenant_id` predicate Scoped.Query binds, `deleted_at IS NULL`, COUNT(DISTINCT), the two LEFT
// JOINs, the NULLABILITY of the new column, and the trigger that refuses to let a locked voucher
// change its funding source. A fake driver would only agree with whatever the query already says,
// and a fake cannot hold a trigger at all.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, and that is not a footnote. On a machine without it every
// test in this file SKIPS while the package still prints `ok` — a green run here means the code
// COMPILES. Nothing in this file may be described as verified until somebody sets the DSN and says
// what came out.
//
// TestMain, moKetNoi and xaRieng live in hang_muc_ke_hoach_von_pg_test.go and are shared: ONE way
// to build the schema for this package, not two.

// --- fixtures ---------------------------------------------------------------------------------

func themNguonVon(t *testing.T, db *sql.DB, tenantID, id, ten string, nam, thuTu int, tong int64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO nguon_von (tenant_id, id, ten, nam, thu_tu, tong_nguon)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, id, ten, nam, thuTu, tong)
	if err != nil {
		t.Fatalf("thêm nguồn vốn %q: %v", ten, err)
	}
}

func themPhanBo(t *testing.T, db *sql.DB, tenantID, id, duAnID, nguonVonID string, soTien int64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO phan_bo_nguon_von (tenant_id, id, du_an_id, nguon_von_id, so_tien_phan_bo)
		 VALUES ($1,$2,$3,$4,$5)`,
		tenantID, id, duAnID, nguonVonID, soTien)
	if err != nil {
		t.Fatalf("thêm phân bổ %q: %v", id, err)
	}
}

func themDuAnToiThieu(t *testing.T, db *sql.DB, tenantID, id, ma string, nam int, keHoach int64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO du_an (tenant_id, id, ma, nam, hang_muc_id, ten, ke_hoach_von_nam, thoi_han_giai_ngan)
		 VALUES ($1,$2,$3,$4,'hm-test','Dự án thử',$5, make_date($4,12,31))`,
		tenantID, id, ma, nam, keHoach)
	if err != nil {
		t.Fatalf("thêm dự án %q: %v", ma, err)
	}
}

// themChungTu inserts one voucher. nguonVonID is passed as *string so that a test can write NULL —
// which is the whole state §13 rule 6 is about, and an empty string would NOT be it (the schema
// refuses that spelling outright).
func themChungTu(t *testing.T, db *sql.DB, tenantID, id, duAnID string, soTien int64, nguonVonID *string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO chung_tu_giai_ngan
		   (tenant_id, id, du_an_id, ngay_chi, so_tien, noi_dung, nguoi_nhap_id, nguon_von_id)
		 VALUES ($1,$2,$3,'2026-03-01',$4,'Thanh toán đợt 1','CB-TEST',$5)`,
		tenantID, id, duAnID, soTien, nguonVonID)
	if err != nil {
		t.Fatalf("thêm chứng từ %q: %v", id, err)
	}
}

func dungNguonVonStore(db *sql.DB) *NguonVonStore {
	return NewNguonVonStore(pkgstore.New(db))
}

func cotCuaBang(t *testing.T, db *sql.DB, bang string) map[string]string {
	t.Helper()
	ra := map[string]string{}
	rows, err := db.Query(
		`SELECT column_name, is_nullable FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = $1`, bang)
	if err != nil {
		t.Fatalf("đọc information_schema cho %s: %v", bang, err)
	}
	defer rows.Close()
	for rows.Next() {
		var c, nullable string
		if err := rows.Scan(&c, &nullable); err != nil {
			t.Fatalf("đọc tên cột: %v", err)
		}
		ra[c] = nullable
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("duyệt tên cột: %v", err)
	}
	return ra
}

// --- the schema itself ---------------------------------------------------------------------------

func TestPgCotNguonVonKhopVoiLuocDoThat(t *testing.T) {
	// THE REASON THIS CASE EXISTS: cotNguonVon is a STRING. A column misspelled in it, or renamed by
	// a later migration, would surface as a 500 on the first real request with "column ... does not
	// exist" in a log nobody is reading yet. This names the missing column in one line.
	//
	// `tenant_id` and `deleted_at` are checked although neither appears in cotNguonVon: Scoped.Query
	// builds `WHERE tenant_id = $1` around the first and DanhSach appends `AND deleted_at IS NULL`
	// around the second, so the statement cannot run without either.
	db := moKetNoi(t)

	co := cotCuaBang(t, db, "nguon_von")
	if len(co) == 0 {
		t.Fatal("bảng nguon_von không tồn tại trong schema — migration 0007 chưa chạy")
	}

	var thieu []string
	for _, c := range strings.Split(cotNguonVon, ",") {
		if c = strings.TrimSpace(c); c != "" {
			if _, ok := co[c]; !ok {
				thieu = append(thieu, c)
			}
		}
	}
	for _, c := range []string{"tenant_id", "deleted_at", "deleted_by", "delete_reason"} {
		if _, ok := co[c]; !ok {
			thieu = append(thieu, c)
		}
	}
	if len(thieu) > 0 {
		sort.Strings(thieu)
		t.Fatalf("cột không có trong bảng thật: %v — câu lệnh sẽ hỏng ở lần gọi thật đầu tiên", thieu)
	}
}

func TestPgCotPhanBoNguonVonKhopVoiLuocDoThat(t *testing.T) {
	db := moKetNoi(t)

	co := cotCuaBang(t, db, "phan_bo_nguon_von")
	if len(co) == 0 {
		t.Fatal("bảng phan_bo_nguon_von không tồn tại trong schema — migration 0007 chưa chạy")
	}
	var thieu []string
	for _, c := range []string{"tenant_id", "id", "du_an_id", "nguon_von_id", "so_tien_phan_bo",
		"deleted_at", "deleted_by", "delete_reason"} {
		if _, ok := co[c]; !ok {
			thieu = append(thieu, c)
		}
	}
	if len(thieu) > 0 {
		sort.Strings(thieu)
		t.Fatalf("cột không có trong bảng thật: %v", thieu)
	}
}

func TestPgChungTuNguonVonIDPhaiNULLABLE(t *testing.T) {
	// THE MOST IMPORTANT ASSERTION ABOUT THE SCHEMA IN THIS FILE, and it is about an ABSENCE of a
	// constraint rather than a presence — which is why it needs its own case rather than being
	// implied by "the column exists".
	//
	// §13 rule 6: "Chứng từ chưa gắn nguồn vốn vẫn cộng vào tổng đã giải ngân, nhưng bị nêu ở cảnh
	// báo 'đã chi nhưng chưa ghi rút từ nguồn nào'". NOT NULL here would refuse exactly the
	// operation the specification permits, and a commune would meet it as a 500 while recording a
	// payment that has already left the account.
	db := moKetNoi(t)

	co := cotCuaBang(t, db, "chung_tu_giai_ngan")
	nullable, ok := co["nguon_von_id"]
	if !ok {
		t.Fatal("chung_tu_giai_ngan.nguon_von_id không tồn tại — migration 0007 chưa chạy")
	}
	if nullable != "YES" {
		t.Fatalf("nguon_von_id is_nullable = %q, phải là YES — NOT NULL chặn đúng thao tác §13 quy tắc 6 cho phép", nullable)
	}
}

// --- the commune boundary ------------------------------------------------------------------------

func TestPgDanhSachNguonVonChiDocXaCuaChinhMinh(t *testing.T) {
	// THE MOST IMPORTANT CASE IN THIS FILE. The predicate that keeps two public authorities apart is
	// a single `WHERE tenant_id = $1` bound by Scoped.Query, and no test with one commune in it can
	// show whether it is there.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themNguonVon(t, db, xa, "nv-a", "Ngân sách xã, phường", 2026, 1, 9_200_000_000)
	themNguonVon(t, db, xaKhac, "nv-b", "Ngân sách thành phố hỗ trợ", 2026, 1, 6_500_000_000)

	ra, err := dungNguonVonStore(db).DanhSach(ctxXa(tenant.ID(xa)), 2026)
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "nv-a" {
		t.Fatalf("RÒ RỈ hoặc thiếu dòng: %+v", ra)
	}
}

func TestPgTienDoTheoNguonChiDocXaCuaChinhMinh(t *testing.T) {
	// The aggregate read has THREE tables in it and therefore three places the commune could be
	// dropped. Two communes hold sources with the SAME id here, which is the case a join on id alone
	// would total together.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themNguonVon(t, db, xa, "nv-chung", "Ngân sách xã, phường", 2026, 1, 1_000_000_000)
	themNguonVon(t, db, xaKhac, "nv-chung", "Ngân sách xã, phường", 2026, 1, 1_000_000_000)

	themDuAnToiThieu(t, db, xaKhac, "da-khac", "DA-KHAC", 2026, 500_000_000)
	themPhanBo(t, db, xaKhac, "pb-khac", "da-khac", "nv-chung", 500_000_000)
	themChungTu(t, db, xaKhac, "ct-khac", "da-khac", 250_000_000, strp("nv-chung"))

	ra, err := dungNguonVonStore(db).TienDoTheoNguon(ctxXa(tenant.ID(xa)), 2026)
	if err != nil {
		t.Fatalf("TienDoTheoNguon: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	if ra[0].DaPhanBo != 0 || ra[0].DaGiaiNgan != 0 || ra[0].SoDuAn != 0 {
		t.Fatalf("RÒ RỈ số liệu của xã khác: đã phân bổ %d, đã giải ngân %d, %d dự án",
			ra[0].DaPhanBo, ra[0].DaGiaiNgan, ra[0].SoDuAn)
	}
}

// --- soft delete and the year filter ---------------------------------------------------------------

func TestPgDanhSachNguonVonBoQuaDongDaXoaMemVaKhacNam(t *testing.T) {
	// Rule 7, invariant 2: a soft-deleted row leaves EVERY read path. The UPDATE below is the only
	// way a row goes — the trigger refuses a hard DELETE outright (see the case at the bottom).
	//
	// The other year is in the same case on purpose: §13 rule 8 makes each budget year its own set,
	// and a list mixing two years would total one commune's funding twice on one screen.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguonVon(t, db, xa, "nv-con", "Ngân sách xã, phường", 2026, 1, 1_000)
	themNguonVon(t, db, xa, "nv-xoa", "Nguồn xã hội hoá", 2026, 2, 1_000)
	themNguonVon(t, db, xa, "nv-2027", "Ngân sách xã, phường", 2027, 1, 1_000)
	if _, err := db.Exec(
		`UPDATE nguon_von SET deleted_at = now(), deleted_by = $1, delete_reason = $2
		  WHERE tenant_id = $3 AND id = $4`,
		"CB-TEST", "test", xa, "nv-xoa"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ra, err := dungNguonVonStore(db).DanhSach(ctxXa(tenant.ID(xa)), 2026)
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "nv-con" {
		t.Fatalf("danh sách = %+v — muốn đúng một dòng nv-con", ra)
	}
}

// --- §6, against the specification's own figures ---------------------------------------------------

func TestPgTienDoTheoNguonTheoViDuCuaDacTa(t *testing.T) {
	// §6's first card: "Ngân sách xã, phường — tổng 9,2 tỷ · đã phân bổ 70 triệu · đã giải ngân 13,2
	// triệu · 2 dự án".
	//
	// THE VOUCHER WITH NO SOURCE IS THE POINT OF THE LAST ASSERTION. §13 rule 6 keeps it in the
	// commune's disbursed total but out of every card; if the subquery dropped its
	// `nguon_von_id IS NOT NULL`, that 500 triệu would land on this card and the screen would report
	// money drawn from a source nobody named.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguonVon(t, db, xa, "nv-xa", "Ngân sách xã, phường", 2026, 1, 9_200_000_000)
	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 50_000_000)
	themDuAnToiThieu(t, db, xa, "da-2", "DA02", 2026, 20_000_000)
	themPhanBo(t, db, xa, "pb-1", "da-1", "nv-xa", 50_000_000)
	themPhanBo(t, db, xa, "pb-2", "da-2", "nv-xa", 20_000_000)
	themChungTu(t, db, xa, "ct-1", "da-1", 13_200_000, strp("nv-xa"))
	themChungTu(t, db, xa, "ct-khong-nguon", "da-2", 500_000_000, nil)

	ra, err := dungNguonVonStore(db).TienDoTheoNguon(ctxXa(tenant.ID(xa)), 2026)
	if err != nil {
		t.Fatalf("TienDoTheoNguon: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	card := ra[0]
	if card.NguonVon.TongNguon != 9_200_000_000 {
		t.Errorf("tổng nguồn = %d, muốn 9200000000", card.NguonVon.TongNguon)
	}
	if card.DaPhanBo != 70_000_000 {
		t.Errorf("đã phân bổ = %d, muốn 70000000 (§6)", card.DaPhanBo)
	}
	if card.SoDuAn != 2 {
		t.Errorf("số dự án = %d, muốn 2 (§6)", card.SoDuAn)
	}
	if card.DaGiaiNgan != 13_200_000 {
		t.Errorf("đã giải ngân = %d, muốn 13200000 — chứng từ KHÔNG gắn nguồn không được rơi vào thẻ này (§13 quy tắc 6)",
			card.DaGiaiNgan)
	}
}

func TestPgTienDoTheoNguonDemDuAnRIENGBIET(t *testing.T) {
	// Migration 0007 deliberately leaves `UNIQUE (tenant_id, du_an_id, nguon_von_id)` undeclared —
	// it is an open question for the customer — so one project CAN hold two lines naming one source.
	// COUNT(DISTINCT du_an_id) is what keeps "Số dự án" honest until that question is answered;
	// COUNT(*) would report two projects where there is one.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguonVon(t, db, xa, "nv-xa", "Ngân sách xã, phường", 2026, 1, 1_000_000_000)
	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 60_000_000)
	themPhanBo(t, db, xa, "pb-1", "da-1", "nv-xa", 30_000_000)
	themPhanBo(t, db, xa, "pb-2", "da-1", "nv-xa", 30_000_000)

	ra, err := dungNguonVonStore(db).TienDoTheoNguon(ctxXa(tenant.ID(xa)), 2026)
	if err != nil {
		t.Fatalf("TienDoTheoNguon: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	if ra[0].SoDuAn != 1 {
		t.Errorf("số dự án = %d, muốn 1 — đang đếm DÒNG thay vì đếm dự án riêng biệt", ra[0].SoDuAn)
	}
	// The money is the sum of both lines: two allocations from one source are two real amounts.
	if ra[0].DaPhanBo != 60_000_000 {
		t.Errorf("đã phân bổ = %d, muốn 60000000", ra[0].DaPhanBo)
	}
}

func TestPgTienDoTheoNguonGiuNguonChuaPhanBoDongNao(t *testing.T) {
	// §6's third sample row: "Chương trình mục tiêu quốc gia, 10,4 tỷ, đã phân bổ 0 đ, 0 dự án". Both
	// joins must be LEFT — an INNER join would make a source the commune declared vanish from the
	// screen that exists to show the commune's sources.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguonVon(t, db, xa, "nv-ctmt", "Chương trình mục tiêu quốc gia", 2026, 3, 10_400_000_000)

	ra, err := dungNguonVonStore(db).TienDoTheoNguon(ctxXa(tenant.ID(xa)), 2026)
	if err != nil {
		t.Fatalf("TienDoTheoNguon: %v", err)
	}
	if len(ra) != 1 || ra[0].NguonVon.ID != "nv-ctmt" {
		t.Fatalf("nguồn chưa phân bổ đồng nào bị rơi khỏi §6: %+v", ra)
	}
	if ra[0].DaPhanBo != 0 || ra[0].SoDuAn != 0 || ra[0].DaGiaiNgan != 0 {
		t.Fatalf("ba số phải là 0: %+v", ra[0])
	}
}

// --- allocations of one project --------------------------------------------------------------------

func TestPgPhanBoTheoDuAnChiDocXaVaBoDongDaXoaMem(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 100_000_000)
	themDuAnToiThieu(t, db, xaKhac, "da-1", "DA01", 2026, 100_000_000)
	themPhanBo(t, db, xa, "pb-1", "da-1", "nv-xa", 60_000_000)
	themPhanBo(t, db, xa, "pb-xoa", "da-1", "nv-tp", 40_000_000)
	// Same project id in the other commune — the row a query missing `tenant_id` would pick up.
	themPhanBo(t, db, xaKhac, "pb-khac", "da-1", "nv-xa", 99_000_000)

	if _, err := db.Exec(
		`UPDATE phan_bo_nguon_von SET deleted_at = now(), deleted_by = $1, delete_reason = $2
		  WHERE tenant_id = $3 AND id = $4`,
		"CB-TEST", "test", xa, "pb-xoa"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ra, err := dungNguonVonStore(db).PhanBoTheoDuAn(ctxXa(tenant.ID(xa)), "da-1")
	if err != nil {
		t.Fatalf("PhanBoTheoDuAn: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "pb-1" || ra[0].SoTien != 60_000_000 {
		t.Fatalf("phân bổ = %+v — muốn đúng dòng pb-1 của xã này", ra)
	}
}

// --- the floor in the database ----------------------------------------------------------------------

func TestPgKhongXoaCungDuocNguonVonVaPhanBo(t *testing.T) {
	// Rule 7, forbidden #1, enforced in the DATABASE and not in the application — a promise the
	// application makes is a promise a psql session never heard (ADR 0013).
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themNguonVon(t, db, xa, "nv-xa", "Ngân sách xã, phường", 2026, 1, 1_000)
	themPhanBo(t, db, xa, "pb-1", "da-1", "nv-xa", 1_000)

	if _, err := db.Exec(`DELETE FROM nguon_von WHERE tenant_id = $1 AND id = $2`, xa, "nv-xa"); err == nil {
		t.Error("XOÁ CỨNG nguon_von được chấp nhận — trigger ho_so_luu_tru_cam_xoa_cung không gắn")
	}
	if _, err := db.Exec(`DELETE FROM phan_bo_nguon_von WHERE tenant_id = $1 AND id = $2`, xa, "pb-1"); err == nil {
		t.Error("XOÁ CỨNG phan_bo_nguon_von được chấp nhận — trigger ho_so_luu_tru_cam_xoa_cung không gắn")
	}
}

func TestPgChungTuDaKhoaTuChoiDoiNguonVon(t *testing.T) {
	// THE CASE 0004 ASKED FOR IN ADVANCE (0004:137-139): "A COLUMN ADDED LATER AND NOT ADDED HERE IS
	// AN EDITABLE LOCKED FIELD ... nguon_von_id is the first column that will have to be added to
	// this list". Migration 0007 replaces chung_tu_da_khoa() to add it, and this is the only thing
	// that can show the replacement actually took.
	//
	// Moving a locked voucher between two funding sources changes two cards on §6 — after the figure
	// was signed off and frozen — and leaves the voucher itself looking untouched.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 100_000_000)
	themNguonVon(t, db, xa, "nv-xa", "Ngân sách xã, phường", 2026, 1, 1_000_000_000)
	themNguonVon(t, db, xa, "nv-tp", "Ngân sách thành phố hỗ trợ", 2026, 2, 1_000_000_000)

	// A voucher already locked. The INSERT carries the two facts `da-khoa` requires
	// (chung_tu_giai_ngan_khoa_co_thoi_diem, ..._khoa_co_nguoi); the trigger guards UPDATE only.
	if _, err := db.Exec(
		`INSERT INTO chung_tu_giai_ngan
		   (tenant_id, id, du_an_id, ngay_chi, so_tien, noi_dung, nguoi_nhap_id,
		    trang_thai, thoi_diem_khoa, nguoi_khoa_id, nguon_von_id)
		 VALUES ($1,'ct-khoa','da-1','2026-03-01',10000000,'Đợt 1','CB-A',
		         'da-khoa', now(), 'CB-B', 'nv-xa')`, xa); err != nil {
		t.Fatalf("thêm chứng từ đã khoá: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE chung_tu_giai_ngan SET nguon_von_id = 'nv-tp' WHERE tenant_id = $1 AND id = 'ct-khoa'`,
		xa); err == nil {
		t.Fatal("ĐỔI ĐƯỢC nguồn vốn của chứng từ ĐÃ KHOÁ — chung_tu_da_khoa() chưa có nguon_von_id trong danh sách từ chối")
	}

	// AND THE GUARD MUST BE SPECIFIC: a voucher that is NOT locked may still have its source set.
	// Without this half, a trigger that refused every UPDATE would pass the case above while
	// breaking the ordinary operation §13 rule 6 exists to allow.
	themChungTu(t, db, xa, "ct-mo", "da-1", 5_000_000, nil)
	if _, err := db.Exec(
		`UPDATE chung_tu_giai_ngan SET nguon_von_id = 'nv-tp' WHERE tenant_id = $1 AND id = 'ct-mo'`,
		xa); err != nil {
		t.Fatalf("chứng từ CHƯA khoá mà không gắn được nguồn vốn: %v", err)
	}
}

func TestPgChungTuChuaGanNguonVanCongVaoTongDaGiaiNgan(t *testing.T) {
	// §13 rule 6, the half that is about a number rather than a column: "Chứng từ chưa gắn nguồn vốn
	// VẪN CỘNG vào tổng đã giải ngân". This reads through DuAnStore — the path every disbursement
	// screen totals — so it also answers migration question 4 with evidence: adding the column did
	// not change what that figure means.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 100_000_000)
	themChungTu(t, db, xa, "ct-khong-nguon", "da-1", 7_000_000, nil)

	td, err := NewDuAnStore(pkgstore.New(db)).ChiTiet(ctxXa(tenant.ID(xa)), "da-1")
	if err != nil {
		t.Fatalf("ChiTiet: %v", err)
	}
	if td.DaGiaiNgan != 7_000_000 {
		t.Fatalf("đã giải ngân = %d, muốn 7000000 — chứng từ chưa gắn nguồn bị loại khỏi tổng (§13 quy tắc 6)",
			td.DaGiaiNgan)
	}
}

func TestPgChungTuNguonVonRongBiTuChoi(t *testing.T) {
	// '' IS NOT NULL. A write path sending an empty string would drop the voucher out of the "đã chi
	// nhưng chưa ghi rút từ nguồn nào" warning while attaching it to no source either — money
	// missing from both sides of §6, with the row looking filled in.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 100_000_000)
	if _, err := db.Exec(
		`INSERT INTO chung_tu_giai_ngan
		   (tenant_id, id, du_an_id, ngay_chi, so_tien, noi_dung, nguoi_nhap_id, nguon_von_id)
		 VALUES ($1,'ct-rong','da-1','2026-03-01',1000,'Đợt 1','CB-A','')`, xa); err == nil {
		t.Error("nguon_von_id = '' được chấp nhận — CHECK chung_tu_giai_ngan_nguon_von_khong_rong không có")
	}
}

// strp is `&s` for a literal, so a test can pass NULL as nil and a value as strp("…") at the same
// call site without a temporary variable per case.
func strp(s string) *string { return &s }
