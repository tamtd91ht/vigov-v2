package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a suite that prints `ok`
// while asserting nothing is this repository's worst known trap.
//
// It runs with NO PostgreSQL. The fake driver below RECORDS the statement the store builds and
// hands back rows the test supplied; it does not execute SQL. So:
//
//	PROVED HERE   the commune reaches the statement as $1 and comes from the CONTEXT, never from
//	              an argument · BOTH sides of the join are constrained to that same $1 · the
//	              soft-delete predicate is present on the project AND on the voucher subquery ·
//	              the disbursed total is an AGGREGATE and not a column of du_an · no voucher state
//	              is excluded from the total · the budget year is required rather than defaulted ·
//	              the LIMIT is the ceiling PLUS ONE and the refusal fires at ceiling+1 returning
//	              NO rows · a missing project is ErrKhongThayDuAn and not a zero value · a driver
//	              failure is wrapped, not swallowed · no commune in the context panics rather than
//	              defaulting.
//
//	NOT PROVED    anything PostgreSQL does with that statement. The fake returns whatever rows the
//	              test supplies whatever the column list says, so a typo in cotDuAn passes cleanly
//	              here. Partition routing, the CHECK constraints, the hard-delete trigger and the
//	              locked-voucher trigger are all unreachable from this file — they need a real
//	              server, and none is reachable from this build environment.

// --- the fake driver ------------------------------------------------------------------------

// dongDuAnGia is one row as the fake hands it back, in the order of cotDuAn plus the aggregate.
// Values are distinct per field so a mis-wired Scan shows up as WRONG DATA rather than as a zero
// that looks plausible — the two amounts especially, which swap with no error at all.
type dongDuAnGia struct {
	id, ma            string
	nam               int64
	hangMucID, ten    string
	moTa              string
	keHoach, tongMuc  int64
	donVi, canBo      string
	khoiCong, hoanTat any // nil or time.Time
	thoiHan           time.Time
	daGiaiNgan        int64
}

func (d dongDuAnGia) giaTri() []driver.Value {
	return []driver.Value{
		d.id, d.ma, d.nam, d.hangMucID, d.ten, d.moTa,
		d.keHoach, d.tongMuc, d.donVi, d.canBo,
		d.khoiCong, d.hoanTat, d.thoiHan, d.daGiaiNgan,
	}
}

type khoDuAnGia struct {
	lenh []lenhGia
	hang []dongDuAnGia
	loi  error
}

func (k *khoDuAnGia) Connect(context.Context) (driver.Conn, error) { return &connDuAnGia{k: k}, nil }
func (k *khoDuAnGia) Driver() driver.Driver                        { return trinhDuAnGia{} }

type trinhDuAnGia struct{}

func (trinhDuAnGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connDuAnGia struct{ k *khoDuAnGia }

func (c *connDuAnGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connDuAnGia) Close() error { return nil }
func (c *connDuAnGia) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *connDuAnGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGia{sql: q, args: gt})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	dong := make([][]driver.Value, 0, len(c.k.hang))
	for _, h := range c.k.hang {
		dong = append(dong, h.giaTri())
	}
	// 14 unnamed columns: cotDuAn's thirteen plus the aggregate. The fake does not parse the
	// column list — see the NOT PROVED note at the top.
	cot := make([]string, 14)
	return &rowsGia{cot: cot, hang: dong}, nil
}

func dungKhoDuAn(k *khoDuAnGia) *DuAnStore {
	return NewDuAnStore(pkgstore.New(sql.OpenDB(k)))
}

func mauDuAn() []dongDuAnGia {
	return []dongDuAnGia{{
		id: "da-001", ma: "DA-2026-be-tong-hoa-duong-ngo-xo-2", nam: 2026,
		hangMucID: "hm-001", ten: "Bê tông hoá đường ngõ xóm tổ 6", moTa: "mô tả",
		// §8's worked project: plan 100 triệu, disbursed 90 triệu. The two amounts differ from
		// each other and from the total, so a swap between them cannot pass.
		keHoach: 100_000_000, tongMuc: 120_000_000,
		donVi: "bp-001", canBo: "cb-001",
		thoiHan: time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
		// NOT a column of du_an: the aggregate over the project's live vouchers.
		daGiaiNgan: 90_000_000,
	}}
}

// --- the statement the store builds ----------------------------------------------------------

func TestDanhSachDuAnBuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of DanhSach and cannot be: it
	// arrives in the context and QueryJoin binds it to $1.
	k := &khoDuAnGia{hang: mauDuAn()}
	if _, err := dungKhoDuAn(k).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("số câu lệnh = %d, muốn 1", len(k.lenh))
	}
	if got := k.lenh[0].args[0]; got != string(xaThu) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", got, xaThu)
	}
	if got := k.lenh[0].args[1]; got != int64(2026) {
		t.Fatalf("$2 = %v, muốn năm ngân sách 2026", got)
	}
}

func TestDanhSachDuAnDoiXaOCAHAIVEcuaPhepNoi(t *testing.T) {
	// THE PROPERTY THIS CASE EXISTS FOR: a join reaches two tables, and constraining only the
	// outer one leaves the other joinable across communes wherever ids collide. No test of a
	// single commune would ever show it, so it is asserted on the TEXT of the statement.
	k := &khoDuAnGia{hang: mauDuAn()}
	if _, err := dungKhoDuAn(k).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	q := k.lenh[0].sql

	for _, phai := range []string{
		"da.tenant_id = $1",           // the project side
		"ct.tenant_id = $1",           // the voucher subquery
		"ct.tenant_id = da.tenant_id", // and the join itself, not id alone
		"da.deleted_at IS NULL",       // rule 7, invariant 2 — the project
		"ct.deleted_at IS NULL",       // rule 7, invariant 2 — the vouchers
		"SUM(ct.so_tien)",             // the total is DERIVED, never a stored column
		"chung_tu_giai_ngan",          // ... from the vouchers, not from anywhere else
		"ORDER BY da.ma",              // total order: `ma` is unique per commune
	} {
		if !strings.Contains(q, phai) {
			t.Fatalf("câu lệnh thiếu %q:\n%s", phai, q)
		}
	}
	// No voucher state is excluded: §11 counts from `ke-toan-nhap` upward, which is every state.
	if strings.Contains(q, "trang_thai") {
		t.Fatalf("câu lệnh lọc theo trang_thai — §11 tính TỪ `ke-toan-nhap` trở đi, tức là cả ba:\n%s", q)
	}
	// And `da_giai_ngan` must never be read as a column of du_an.
	if strings.Contains(q, "da.da_giai_ngan") {
		t.Fatalf("đọc `da_giai_ngan` như một CỘT của du_an — nó là tổng suy ra:\n%s", q)
	}
}

func TestDanhSachDuAnDoiNamNganSachChuKhongMacDinh(t *testing.T) {
	// A default here would report a different year's money under this year's heading, and nothing
	// on the screen would say so (§13 rule 8). Fail closed.
	k := &khoDuAnGia{hang: mauDuAn()}
	_, err := dungKhoDuAn(k).DanhSach(ctxXa(xaThu), LocDuAn{})
	if !errors.Is(err, ErrThieuNamNganSach) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNamNganSach", err)
	}
	if len(k.lenh) != 0 {
		t.Fatal("thiếu năm mà vẫn chạy truy vấn — phải từ chối TRƯỚC khi chạm kho")
	}
}

func TestDanhSachDuAnDocDungTongSuyRa(t *testing.T) {
	k := &khoDuAnGia{hang: mauDuAn()}
	ra, err := dungKhoDuAn(k).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026})
	if err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("số dòng = %d, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.DuAn.Ma != "DA-2026-be-tong-hoa-duong-ngo-xo-2" {
		t.Fatalf("mã = %q", mot.DuAn.Ma)
	}
	if mot.DuAn.KeHoachVonNam != 100_000_000 {
		t.Fatalf("kế hoạch vốn = %d, muốn 100000000 — hai số tiền đứng cạnh nhau, đảo là im lặng", mot.DuAn.KeHoachVonNam)
	}
	if mot.DuAn.TongMucDuocDuyet != 120_000_000 {
		t.Fatalf("tổng mức = %d, muốn 120000000", mot.DuAn.TongMucDuocDuyet)
	}
	if mot.DaGiaiNgan != 90_000_000 {
		t.Fatalf("đã giải ngân = %d, muốn 90000000", mot.DaGiaiNgan)
	}
	// And the derived figures line up with §8's worked project: 90%.
	ty, ok := mot.TyLeGiaiNgan()
	if !ok || ty != 9000 {
		t.Fatalf("tỷ lệ = %d phần vạn (ok=%v), muốn 9000", ty, ok)
	}
}

func TestDanhSachDuAnLIMITLaTranCongMot(t *testing.T) {
	k := &khoDuAnGia{hang: mauDuAn()}
	if _, err := dungKhoDuAn(k).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	cuoi := k.lenh[0].args[len(k.lenh[0].args)-1]
	if cuoi != int64(TranDuAnMotNam+1) {
		t.Fatalf("LIMIT = %v, muốn trần+1 = %d — lấy đúng trần thì một danh sách bị cắt trông y hệt một danh sách đủ",
			cuoi, TranDuAnMotNam+1)
	}
}

func TestDanhSachDuAnVuotTranThiTUCHOIChuKhongCat(t *testing.T) {
	hang := make([]dongDuAnGia, 0, TranDuAnMotNam+1)
	for i := 0; i <= TranDuAnMotNam; i++ {
		hang = append(hang, dongDuAnGia{id: "da", ma: "DA", nam: 2026, keHoach: 1, daGiaiNgan: 0,
			thoiHan: time.Now()})
	}
	ra, err := dungKhoDuAn(&khoDuAnGia{hang: hang}).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026})
	if !errors.Is(err, ErrQuaNhieuDuAn) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuDuAn", err)
	}
	if ra != nil {
		// Handing back rows alongside the error is how a refusal turns back into a truncation:
		// these figures are totalled on the screen.
		t.Fatalf("trả về %d dòng CÙNG với lỗi — phải trả nil", len(ra))
	}
}

func TestDanhSachDuAnBocLoiChuKhongNuot(t *testing.T) {
	loi := errors.New("kho hỏng")
	_, err := dungKhoDuAn(&khoDuAnGia{loi: loi}).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026})
	if !errors.Is(err, loi) {
		t.Fatalf("lỗi = %v, phải bọc lỗi gốc bằng %%w", err)
	}
}

func TestDanhSachDuAnKhongCoXaThiPANICChuKhongMacDinh(t *testing.T) {
	// FAIL CLOSED. A default on the isolation path collapses the whole isolation silently, with
	// tests still green (rule 1, forbidden #1).
	defer func() {
		if recover() == nil {
			t.Fatal("không có xã trong context mà vẫn chạy — phải panic, không được mặc định")
		}
	}()
	_, _ = dungKhoDuAn(&khoDuAnGia{}).DanhSach(context.Background(), LocDuAn{Nam: 2026})
}

func TestChiTietDuAnKhongThayThiKhongPhaiGiaTriRONG(t *testing.T) {
	// A zero value returned with a nil error is a project that renders as an empty screen with
	// 0 đ on it — a figure, and a wrong one. The caller answers 404.
	_, err := dungKhoDuAn(&khoDuAnGia{}).ChiTiet(ctxXa(xaThu), "da-khong-ton-tai")
	if !errors.Is(err, ErrKhongThayDuAn) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayDuAn", err)
	}
}

func TestChiTietDuAnDungCUNGCongThucTongVoiDanhSach(t *testing.T) {
	// The detail page and the list must not be able to disagree about one project's disbursed
	// figure. Asserted on the text: both statements carry the same aggregate expression.
	kDanh := &khoDuAnGia{hang: mauDuAn()}
	if _, err := dungKhoDuAn(kDanh).DanhSach(ctxXa(xaThu), LocDuAn{Nam: 2026}); err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}
	kChi := &khoDuAnGia{hang: mauDuAn()}
	mot, err := dungKhoDuAn(kChi).ChiTiet(ctxXa(xaThu), "da-001")
	if err != nil {
		t.Fatalf("đọc chi tiết: %v", err)
	}
	if mot.DaGiaiNgan != 90_000_000 {
		t.Fatalf("đã giải ngân = %d, muốn 90000000", mot.DaGiaiNgan)
	}
	if !strings.Contains(kDanh.lenh[0].sql, tongChungTu) || !strings.Contains(kChi.lenh[0].sql, tongChungTu) {
		t.Fatal("hai tuyến đọc không dùng chung một biểu thức tổng — hai màn hình sẽ lệch nhau")
	}
}

func TestChiTietDuAnChiDocXaTrongContext(t *testing.T) {
	// Another commune's project is indistinguishable from one that does not exist, because the
	// query cannot reach it: $1 comes from the context.
	k := &khoDuAnGia{hang: mauDuAn()}
	if _, err := dungKhoDuAn(k).ChiTiet(ctxXa(xaThuHai), "da-001"); err != nil {
		t.Fatalf("đọc chi tiết: %v", err)
	}
	if got := k.lenh[0].args[0]; got != string(xaThuHai) {
		t.Fatalf("$1 = %v, muốn %q", got, xaThuHai)
	}
	if got := k.lenh[0].args[1]; got != "da-001" {
		t.Fatalf("$2 = %v, muốn id dự án", got)
	}
}

// domain is imported for the type assertion below; keeping the reference explicit stops a future
// edit from dropping the import and quietly changing what these tests scan into.
var _ = domain.TienDoDuAn{}
