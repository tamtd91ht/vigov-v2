package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// THE WRITE-SIDE TWIN OF THE POSITIONAL SCAN TRAP.
//
// cotTomTat's comment warns about adjacent same-typed columns on the READ side. The write statements
// in can_bo_ghi.go have exactly the same trap on the other side: `$n` in the SQL text is bound by
// POSITION to the Go argument list of tx.Exec, and two adjacent TEXT arguments traded over raise
// nothing anywhere — PostgreSQL stores a string in a text column either way. The costly pairs:
//
//	XoaMem       deleted_by ↔ delete_reason   the archival record of WHO removed a row (rule 6
//	                                           invariant 8, rule 7 invariant 1) holds the reason text,
//	                                           and the reason column holds a staff code. For ever:
//	                                           a second delete cannot overwrite it.
//	CapNhatHoSo  dien_thoai_co_quan ↔ di_dong_ca_nhan   every PATCH swaps the office landline and the
//	                                           personal mobile (#16) — the masking rules then apply to
//	                                           the wrong one.
//	Chen         ho_ten / email / chuc_vu, and the two phones, the same way on INSERT.
//
// The app tests cannot see this: they run a FAKE store. The pg suites could, but they SKIP without
// VIGOV_TEST_DSN. So this file runs the REAL store methods inside a real *sql.Tx over a recording
// driver, then reads each `column = $n` out of the statement the method actually sent and checks
// that argument n is the value the caller gave for THAT column. Every value is distinct, so a swap
// cannot pass.

// --- recording driver with transactions ---------------------------------------------------------

type gtsLenh struct {
	sql  string
	args []driver.Value
}

type gtsGhi struct{ lenh []gtsLenh }

type gtsKetNoi struct{ g *gtsGhi }

func (k gtsKetNoi) Connect(context.Context) (driver.Conn, error) { return &gtsConn{g: k.g}, nil }
func (k gtsKetNoi) Driver() driver.Driver                        { return gtsTrinh{} }

type gtsTrinh struct{}

func (gtsTrinh) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type gtsConn struct{ g *gtsGhi }

func (c *gtsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *gtsConn) Close() error              { return nil }
func (c *gtsConn) Begin() (driver.Tx, error) { return gtsTx{}, nil }
func (c *gtsConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.g.lenh = append(c.g.lenh, gtsLenh{sql: q, args: gt})
	return driver.RowsAffected(1), nil
}

type gtsTx struct{}

func (gtsTx) Commit() error   { return nil }
func (gtsTx) Rollback() error { return nil }

// gtsChay runs fn inside one transaction of commune xaMau and returns the single statement it sent.
func gtsChay(t *testing.T, fn func(s *CanBoStore, tx *pkgstore.ScopedTx) error) gtsLenh {
	t.Helper()
	g := &gtsGhi{}
	db := sql.OpenDB(gtsKetNoi{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	pdb := pkgstore.New(db)
	s := NewCanBoStore(pdb)
	ctx := ctxXa(xaMau)
	if err := pdb.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(s, tx) }); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	if len(g.lenh) != 1 {
		t.Fatalf("muốn đúng 1 câu lệnh ghi, có %d", len(g.lenh))
	}
	return g.lenh[0]
}

// gtsReGan matches `column = $n` and `column = nullif($n, <empty string>)`.
var gtsReGan = regexp.MustCompile(`([a-z_]+) = (?:nullif\()?\$(\d+)`)

// gtsGanCot maps each column named in `col = $n` to the argument actually bound at $n.
func gtsGanCot(t *testing.T, l gtsLenh) map[string]driver.Value {
	t.Helper()
	ra := map[string]driver.Value{}
	for _, m := range gtsReGan.FindAllStringSubmatch(strings.Join(strings.Fields(l.sql), " "), -1) {
		n, _ := strconv.Atoi(m[2])
		if n < 1 || n > len(l.args) {
			t.Fatalf("placeholder $%d ngoài %d tham số: %s", n, len(l.args), l.sql)
		}
		ra[m[1]] = l.args[n-1]
	}
	return ra
}

// gtsKiem asserts every expected column is bound to its own value — and that the expectation is not
// vacuous: a column the statement no longer names is a failure, not a pass.
func gtsKiem(t *testing.T, viec string, gan map[string]driver.Value, muon map[string]driver.Value) {
	t.Helper()
	for cot, v := range muon {
		got, co := gan[cot]
		if !co {
			t.Errorf("%s: câu lệnh không còn gán cột %q bằng placeholder — bài kiểm mù ở cột này", viec, cot)
			continue
		}
		if tg, ok := v.(time.Time); ok {
			if gt, ok := got.(time.Time); !ok || !gt.Equal(tg) {
				t.Errorf("%s: %s nhận %#v, muốn %v", viec, cot, got, tg)
			}
			continue
		}
		if got != v {
			t.Errorf("%s: %s nhận %#v, muốn %#v — hai tham số vị trí đã đổi chỗ", viec, cot, got, v)
		}
	}
}

// --- the four write statements ------------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: in XoaMem, pass `lyDo, xoaBoi` instead of `xoaBoi, lyDo`.
// Verified 2026-09-24: the whole suite stayed green under that swap before this test existed.
func TestXoaMemThamSoDungCot(t *testing.T) {
	luc := time.Date(2026, 9, 24, 9, 30, 0, 0, time.UTC)
	l := gtsChay(t, func(s *CanBoStore, tx *pkgstore.ScopedTx) error {
		return s.XoaMem(context.Background(), tx, "nd-xoa-01", "CB-2026-XOA001", "Nhập trùng do nhập Excel hai lần", luc)
	})
	gtsKiem(t, "XoaMem", gtsGanCot(t, l), map[string]driver.Value{
		"tenant_id":     xaMau,
		"id":            "nd-xoa-01",
		"deleted_at":    luc,
		"deleted_by":    "CB-2026-XOA001",
		"delete_reason": "Nhập trùng do nhập Excel hai lần",
	})
}

// MUTATION THAT MUST TURN THIS RED: in CapNhatHoSo, trade cb.DienThoaiCoQuan and cb.DiDongCaNhan.
// Verified 2026-09-24: the whole suite stayed green under that swap before this test existed.
func TestCapNhatHoSoThamSoDungCot(t *testing.T) {
	cb := domain.CanBoTomTat{
		ID: "nd-sua-01", HoTen: "Trần Thị B", Email: "b@example.gov.vn", ChucVu: "Kế toán",
		BoPhanID: "bp-009", DienThoaiCoQuan: "0200000001", DiDongCaNhan: "0900000000", CoZalo: true,
	}
	l := gtsChay(t, func(s *CanBoStore, tx *pkgstore.ScopedTx) error {
		return s.CapNhatHoSo(context.Background(), tx, cb)
	})
	gtsKiem(t, "CapNhatHoSo", gtsGanCot(t, l), map[string]driver.Value{
		"tenant_id": xaMau, "id": cb.ID,
		"ho_ten": cb.HoTen, "email": cb.Email, "chuc_vu": cb.ChucVu, "bo_phan_id": cb.BoPhanID,
		"dien_thoai_co_quan": cb.DienThoaiCoQuan, "di_dong_ca_nhan": cb.DiDongCaNhan,
		"co_zalo": true,
	})
}

// The publication write binds each consent mark to its own column. The four arguments have four
// different Go types, so a swap would error on a real server rather than pass — this pins the
// mapping anyway because the pg proof of it does not run here.
func TestDatCongKhaiThamSoDungCot(t *testing.T) {
	luc := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	thuTu := 7
	cb := domain.CanBoTomTat{ID: "nd-ck-01", HienTrenMiniApp: true, DongYCongKhaiLuc: &luc,
		DongYCongKhaiGhiBoi: "CB-2026-GHI777", ThuTuDanhBa: &thuTu}
	l := gtsChay(t, func(s *CanBoStore, tx *pkgstore.ScopedTx) error {
		return s.DatCongKhai(context.Background(), tx, cb)
	})
	gtsKiem(t, "DatCongKhai", gtsGanCot(t, l), map[string]driver.Value{
		"tenant_id": xaMau, "id": cb.ID,
		"hien_tren_mini_app": true, "dong_y_cong_khai_luc": luc,
		"dong_y_cong_khai_ghi_boi": "CB-2026-GHI777", "thu_tu_danh_ba": int64(7),
	})
}

// gtsTachCot splits a parenthesised SQL list on top-level commas (the nullif call on $7 stays whole).
func gtsTachCot(s string) []string {
	var ra []string
	sau, bat := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			sau++
		case ')':
			sau--
		case ',':
			if sau == 0 {
				ra = append(ra, strings.TrimSpace(s[bat:i]))
				bat = i + 1
			}
		}
	}
	return append(ra, strings.TrimSpace(s[bat:]))
}

// The INSERT pairs its column list with its VALUES list by position, then each `$n` with the Go
// argument list by position — two lockstep lists in one statement.
func TestChenThamSoDungCot(t *testing.T) {
	cb := domain.CanBoTomTat{ID: "nd-moi-01", Ma: "CB-2026-MOI001", HoTen: "Lê Văn C",
		Email: "c@example.gov.vn", ChucVu: "Văn thư", BoPhanID: "bp-003",
		DienThoaiCoQuan: "0200000003", DiDongCaNhan: "0900000000"}
	l := gtsChay(t, func(s *CanBoStore, tx *pkgstore.ScopedTx) error {
		return s.Chen(context.Background(), tx, cb)
	})

	m := regexp.MustCompile(`(?s)\((.*?)\)\s*VALUES\s*\((.*)\)\s*$`).FindStringSubmatch(l.sql)
	if m == nil {
		t.Fatalf("không đọc được INSERT: %s", l.sql)
	}
	cot, gia := gtsTachCot(m[1]), gtsTachCot(m[2])
	if len(cot) != len(gia) {
		t.Fatalf("INSERT có %d cột nhưng %d giá trị", len(cot), len(gia))
	}
	reSo := regexp.MustCompile(`\$(\d+)`)
	gan := map[string]driver.Value{}
	for i, c := range cot {
		if so := reSo.FindStringSubmatch(gia[i]); so != nil {
			n, _ := strconv.Atoi(so[1])
			gan[c] = l.args[n-1]
		}
	}
	gtsKiem(t, "Chen", gan, map[string]driver.Value{
		"tenant_id": xaMau, "id": cb.ID, "ma": cb.Ma, "ho_ten": cb.HoTen, "email": cb.Email,
		"chuc_vu": cb.ChucVu, "bo_phan_id": cb.BoPhanID,
		"dien_thoai_co_quan": cb.DienThoaiCoQuan, "di_dong_ca_nhan": cb.DiDongCaNhan,
	})
}

// --- who may write the publication columns --------------------------------------------------------

// ONLY THE PUBLICATION STATEMENT MAY SET THE MINI APP FLAG OR A CONSENT MARK (#12). The profile
// PATCH (`admin.user`) and the create route must not name any of the three: either would be a path
// that publishes a personal mobile without passing DatCongKhai's consent gate. The soft delete names
// them only as the literals that UNPUBLISH (store/can_bo_xoa_test.go).
//
// MUTATION THAT MUST TURN THIS RED: add `hien_tren_mini_app = $9` to capNhatHoSoCanBo (publishing
// everybody who has Zalo). Verified 2026-09-24: green before this test existed; on a real server the
// 0010 CHECK would refuse it, but that proof is a pg test that skips here.
func TestChiCauCongKhaiGhiCotMiniApp(t *testing.T) {
	for ten, stmt := range map[string]string{"chenCanBo": chenCanBo, "capNhatHoSoCanBo": capNhatHoSoCanBo} {
		for _, cot := range []string{"hien_tren_mini_app", "dong_y_cong_khai_luc", "dong_y_cong_khai_ghi_boi"} {
			if strings.Contains(stmt, cot) {
				t.Errorf("%s ghi cột công khai %q — một lối công khai số di động không qua cổng đồng ý (#12)", ten, cot)
			}
		}
	}
}

// THE PUBLICATION WRITE IS SCOPED AND CANNOT LAND ON A SOFT-DELETED ROW, and it writes nothing but
// the publication state — no personal field is reachable from `content.update`.
//
// MUTATION THAT MUST TURN THIS RED: drop `AND deleted_at IS NULL` from datCongKhaiCanBo. Verified
// 2026-09-24: green before this test existed. (TheoIDDeGhi also excludes deleted rows; this is the
// second of the two locks, and nothing else checked it.)
func TestDatCongKhaiChiGhiCotCongKhaiTrenDongChuaXoa(t *testing.T) {
	gon := strings.Join(strings.Fields(datCongKhaiCanBo), " ")
	_, where, co := strings.Cut(gon, " WHERE ")
	if !co {
		t.Fatalf("không có WHERE: %s", gon)
	}
	for _, can := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL"} {
		if !strings.Contains(where, can) {
			t.Errorf("WHERE của câu công khai thiếu %q: %s", can, where)
		}
	}

	var cot []string
	for _, phan := range gtsTachCot(setCua(t, datCongKhaiCanBo)) {
		cot = append(cot, strings.TrimSpace(strings.SplitN(phan, "=", 2)[0]))
	}
	muon := "hien_tren_mini_app,dong_y_cong_khai_luc,dong_y_cong_khai_ghi_boi,thu_tu_danh_ba,cap_nhat_luc"
	if strings.Join(cot, ",") != muon {
		t.Errorf("câu công khai ghi các cột %v, muốn đúng %s", cot, muon)
	}
}

// --- ResolveStaffNames keeps naming a soft-deleted row (#10, ADR 0034) ---------------------------

// A DUPLICATE ROW DELETED THROUGH DELETE /api/v1/staff/{id} MUST STILL BE NAMED beside the archival
// records that quote its code. Since 86f9534 that deletion is reachable from a screen, so a
// `deleted_at IS NULL` added to this read turns names on old records into blanks — silently, and
// the proof against a real server (can_bo_ten_pg_test.go) skips without VIGOV_TEST_DSN.
//
// TEXT-LEVEL AND STATED AS SUCH: it proves the WHERE carries the commune and no deletion filter, and
// that the deletion state is SELECTED as the flag instead. It cannot prove PostgreSQL's answer.
//
// MUTATION THAT MUST TURN THIS RED: append `AND nd.deleted_at IS NULL` to truyVanTenCanBoTheoMa.
// Verified 2026-09-24: green before this test existed.
func TestTenTheoMaKhongLocDongDaXoaNhungVanTheoXa(t *testing.T) {
	gon := strings.Join(strings.Fields(truyVanTenCanBoTheoMa), " ")
	chon, where, co := strings.Cut(gon, " WHERE ")
	if !co {
		t.Fatalf("không có WHERE: %s", gon)
	}
	if !strings.Contains(where, "nd.tenant_id = $1") {
		t.Errorf("tra tên mất ràng buộc xã — đọc chéo xã: %s", where)
	}
	if strings.Contains(where, "deleted_at") {
		t.Errorf("tra tên lọc dòng đã xoá mềm — tên trên hồ sơ cũ thành trống (#10): %s", where)
	}
	if !strings.Contains(chon, "nd.deleted_at IS NULL") {
		t.Errorf("tra tên không còn trả cờ còn-trong-danh-bạ: %s", chon)
	}
}
