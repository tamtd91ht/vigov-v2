package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a test suite that prints
// `ok` while asserting nothing is this repository's worst known trap.
//
// It runs with NO PostgreSQL. The fake driver below RECORDS the statement the store builds and
// hands back the rows the test supplied; it does not execute SQL. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the soft-delete predicate is in the statement · the order and its
//	              tie-break are in the statement · the LIMIT really is the ceiling PLUS ONE · the
//	              refusal fires at ceiling+1 and returns NO rows · the positional Scan lines up
//	              with the column list, by NAME · a driver failure is wrapped, not swallowed.
//	NOT PROVED    anything PostgreSQL does with that statement — that the index is used, that the
//	              partition routing works, that the trigger refuses what it says it refuses. That
//	              needs a real server, and there is none on this machine.
//
// The column-name check is the one worth explaining: the driver builds each row BY COLUMN NAME out
// of the statement, so reordering cotLoaiTaiNguyen without reordering the Scan below turns these
// tests red. Read by position, `ma`/`nhan` and `la_mac_dinh`/`dang_dung` are two pairs of adjacent
// same-typed columns — swapping either produces no error at all, only slugs where labels belong,
// or a map opening on a group that was taken out of use.

var xaThu = tenant.ID("01JA" + strings.Repeat("A", 22))

func ctxXa(xa tenant.ID) context.Context {
	return tenant.Into(context.Background(), xa)
}

// --- the fake driver ------------------------------------------------------------------------

type lenhGia struct {
	sql  string
	args []driver.Value
}

// hangGia is one row the fake returns. The values are distinct per column and distinct per type,
// so a mis-wired Scan shows up as WRONG DATA rather than as a zero value that looks plausible.
type hangGia struct {
	id, ma, nhan      string
	thuTu             int64
	macDinh, dangDung bool
	nguon             string
	reNhanh           bool
}

func (h hangGia) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return h.id
	case "ma":
		return h.ma
	case "nhan":
		return h.nhan
	case "thu_tu":
		return h.thuTu
	case "la_mac_dinh":
		return h.macDinh
	case "dang_dung":
		return h.dangDung
	case "nguon":
		return h.nguon
	case "ma_nguon_re_nhanh":
		return h.reNhanh
	default:
		// A column was added to cotLoaiTaiNguyen and not here. Failing loudly beats scanning a nil
		// that "passes" while proving nothing.
		panic("driver giả: không có giá trị mẫu cho cột " + cot)
	}
}

type khoGia struct {
	lenh []lenhGia
	hang []hangGia
	loi  error
}

func (k *khoGia) Connect(context.Context) (driver.Conn, error) { return &connGia{k: k}, nil }
func (k *khoGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connGia struct{ k *khoGia }

func (c *connGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGia) Close() error { return nil }
func (c *connGia) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *connGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGia{sql: q, args: gt})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	cot, err := cotTrongCauLenh(q)
	if err != nil {
		return nil, err
	}
	dong := make([][]driver.Value, 0, len(c.k.hang))
	for _, h := range c.k.hang {
		mot := make([]driver.Value, len(cot))
		for i, c := range cot {
			mot[i] = h.giaTri(c)
		}
		dong = append(dong, mot)
	}
	return &rowsGia{cot: cot, hang: dong}, nil
}

func cotTrongCauLenh(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var ra []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		ra = append(ra, strings.TrimSpace(c))
	}
	return ra, nil
}

type rowsGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsGia) Columns() []string { return r.cot }
func (r *rowsGia) Close() error      { return nil }
func (r *rowsGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

func dungKho(k *khoGia) *LoaiTaiNguyenBanDoStore {
	return NewLoaiTaiNguyenBanDoStore(pkgstore.New(sql.OpenDB(k)))
}

func mauMotDong() []hangGia {
	// Values chosen so every field is distinguishable from every other: a Scan reading `nhan` into
	// Ma, or `dang_dung` into LaMacDinh, cannot produce a passing assertion.
	return []hangGia{{
		id: "ltn-001", ma: "mau-mot", nhan: "Nhóm mẫu một",
		thuTu: 7, macDinh: true, dangDung: false,
		// A TIER 3 row — the one combination whose tier cannot be guessed from either column alone.
		nguon: "he-thong", reNhanh: true,
	}}
}

// --- the statement the store builds ----------------------------------------------------------

func TestDanhSachBuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of DanhSach and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKho(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != string(xaThu) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xaThu)
	}
	// The same store, a different commune in the context, a different $1. This is what "scoped
	// repository" means in practice — and a store caching the first commune it saw would fail here.
	k2 := &khoGia{hang: mauMotDong()}
	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := dungKho(k2).DanhSach(ctxXa(xaKhac)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if k2.lenh[0].args[0] != string(xaKhac) {
		t.Errorf("$1 = %v, muốn %q", k2.lenh[0].args[0], xaKhac)
	}
}

func TestDanhSachLocDongDaXoaMemVaSapXepOnDinh(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always.
	// And the order is total: `thu_tu` is the commune's own arrangement, `ma` breaks ties and is
	// unique per commune, so two calls cannot return the same rows in a different sequence.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKho(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	q := k.lenh[0].sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY thu_tu, ma") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
	// `dang_dung` must NOT be in the predicate: a group taken out of use is still read — the
	// catalogue screen shows it with a "Đã tắt" chip, and assets already filed under it need its
	// name.
	if strings.Contains(q, "dang_dung =") || strings.Contains(q, "dang_dung IS") {
		t.Errorf("câu lệnh lọc mất nhóm đã tắt: %q", q)
	}
}

func TestDanhSachLayDuTranCongMot(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation this route refuses to perform, performed by
	// the bound meant to prevent it.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKho(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(TranDanhMucLoaiTaiNguyen+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], TranDanhMucLoaiTaiNguyen+1)
	}
}

// --- what comes back -------------------------------------------------------------------------

func TestDanhSachDocDungTungCot(t *testing.T) {
	k := &khoGia{hang: mauMotDong()}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.ID != "ltn-001" || mot.Ma != "mau-mot" || mot.Nhan != "Nhóm mẫu một" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	if mot.ThuTu != 7 {
		t.Errorf("thu_tu = %d, muốn 7", mot.ThuTu)
	}
	// The pair that cannot be caught by type: both are BOOLEAN and adjacent. The fixture sets them
	// to OPPOSITE values on purpose.
	if !mot.LaMacDinh || mot.DangDung {
		t.Errorf("la_mac_dinh / dang_dung đọc ngược: %+v", mot)
	}
}

func TestDanhSachXaChuaCoDongNaoTraDanhSachRong(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE. Migration 0003 creates the table and seeds nothing: the
	// specification contradicts itself about the code list, and the step that would sow a commune's
	// system rows does not exist. An empty list is correct, not a failure — and it is a list, never
	// a nil the caller has to branch on.
	k := &khoGia{}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("danh mục rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra) != 0 {
		t.Fatalf("nhận %d dòng từ một xã chưa có dòng nào", len(ra))
	}
}

// --- the ceiling -------------------------------------------------------------------------------

func TestDanhSachDungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid.
	k := &khoGia{hang: nhieuDong(TranDanhMucLoaiTaiNguyen)}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranDanhMucLoaiTaiNguyen {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranDanhMucLoaiTaiNguyen)
	}
}

func TestDanhSachVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. And the rows already read are DROPPED —
	// handing back a list the caller might render anyway is how a refusal turns back into a silent
	// truncation one careless `if err != nil { log }` later.
	k := &khoGia{hang: nhieuDong(TranDanhMucLoaiTaiNguyen + 1)}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, ErrQuaNhieuLoaiTaiNguyen) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuLoaiTaiNguyen", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

func nhieuDong(n int) []hangGia {
	ra := make([]hangGia, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, hangGia{
			id: fmt.Sprintf("ltn-%04d", i), ma: fmt.Sprintf("mau-%04d", i),
			nhan: fmt.Sprintf("Nhóm mẫu %d", i), thuTu: int64(i), dangDung: true,
			nguon: "don-vi",
		})
	}
	return ra
}

// --- failures ----------------------------------------------------------------------------------

func TestDanhSachLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Rule 6 of the service pattern: wrapped with %w, never swallowed with `_`. The caller
	// distinguishes the ceiling from an ordinary failure with errors.Is, which only works if the
	// chain is intact.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGia{loi: goc}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuLoaiTaiNguyen) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestDanhSachKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would either query every commune's
	// rows or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover turns that
	// into a traceable 500 at the edge. What must never happen is a default commune.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc danh mục khi context không có xã mà không panic")
		}
	}()
	k := &khoGia{hang: mauMotDong()}
	_, _ = dungKho(k).DanhSach(context.Background())
}
