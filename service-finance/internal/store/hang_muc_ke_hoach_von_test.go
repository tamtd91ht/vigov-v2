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
// `ok` while asserting nothing is this repository's worst known trap, and the sibling file
// hang_muc_ke_hoach_von_pg_test.go is exactly that on every machine anybody has run it on.
//
// It runs with NO PostgreSQL. The fake driver below RECORDS the statement the store builds and
// hands back the rows the test supplied; it does not execute SQL. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the table read is the one this service owns · the soft-delete
//	              predicate is in the statement and `dang_dung` is NOT · the order and its
//	              tie-break are in the statement · the LIMIT really is the ceiling PLUS ONE · the
//	              refusal fires at ceiling+1 and returns NO rows · the positional Scan lines up
//	              with the column list, by NAME · a driver failure is wrapped, not swallowed ·
//	              no commune in the context panics rather than defaulting.
//
//	NOT PROVED    anything PostgreSQL does with that statement. THE SHARPEST LIMIT IS THIS: the
//	              fake builds its rows FROM THE COLUMN LIST THE STORE ITSELF HANDS IT, so a column
//	              named here that does not exist in the real table passes cleanly. A typo in
//	              cotHangMuc, a column renamed by a later migration, the partition routing, the
//	              index, the `danh_muc_ba_tang` trigger — none of that is reachable from here.
//	              That is why the pg suite stays: it is the only thing that will ever check the
//	              statement against a real schema, and it becomes valuable the day a DSN exists.
//
// The column-name check is the one worth explaining: the driver builds each row BY COLUMN NAME out
// of the statement, so reordering cotHangMuc without reordering the Scan turns these tests red.
// Read by position, `ma`/`nhan` and `la_mac_dinh`/`dang_dung` are two pairs of adjacent same-typed
// columns — swapping either produces no error at all, only slugs where labels belong, or a form
// pre-selecting a category the commune took out of use.

var (
	xaThu    = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaThuHai = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// ctxXa is shared with hang_muc_ke_hoach_von_pg_test.go, which converts its own string ids at the
// call site. ONE helper, not two: two ways to put a commune into a context is one of them ending up
// subtly different from what the edge really does.
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
	macDinh, dangDung bool
	thuTu             int
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
	case "la_mac_dinh":
		return h.macDinh
	case "dang_dung":
		return h.dangDung
	case "thu_tu":
		// int64 AND NOT int: database/sql only accepts the driver.Value set, and `int` is not in
		// it. A fake handing back `int` fails every Scan with a message about conversion rather
		// than about the column — the kind of noise that gets a fake deleted.
		return int64(h.thuTu)
	case "nguon":
		return h.nguon
	case "ma_nguon_re_nhanh":
		return h.reNhanh
	default:
		// A column was added to cotHangMuc and not here. Failing loudly beats scanning a nil that
		// "passes" while proving nothing.
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

func dungKho(k *khoGia) *HangMucKeHoachVonStore {
	return NewHangMucKeHoachVonStore(pkgstore.New(sql.OpenDB(k)))
}

func mauMotDong() []hangGia {
	// Values chosen so every field is distinguishable from every other: a Scan reading `nhan` into
	// Ma, or `dang_dung` into LaMacDinh, cannot produce a passing assertion. The two booleans are
	// set to OPPOSITE values for exactly that reason.
	return []hangGia{{
		id: "hm-001", ma: "xay-dung-moi", nhan: "Xây dựng mới",
		macDinh: true, dangDung: false,
		// `thu_tu` is deliberately NOT 0 and NOT 1: a zero is indistinguishable from an unscanned
		// field, and 1 from a length. `nguon`/`ma_nguon_re_nhanh` describe a TIER 3 row — the one
		// combination whose tier cannot be guessed from either column alone.
		thuTu: 7, nguon: "he-thong", reNhanh: true,
	}}
}

func nhieuDong(n int) []hangGia {
	ra := make([]hangGia, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, hangGia{
			id: fmt.Sprintf("hm-%04d", i), ma: fmt.Sprintf("hang-muc-%04d", i),
			nhan: fmt.Sprintf("Hạng mục %d", i), dangDung: true,
			thuTu: i, nguon: "don-vi",
		})
	}
	return ra
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
	// THE TABLE THIS SERVICE OWNS, and nothing else. A read against another service's table is a
	// path around the contract (rule 2, forbidden #2), and a typo in the name would otherwise only
	// surface on a machine that has a PostgreSQL — which is no machine here.
	if !strings.Contains(l.sql, "FROM hang_muc_ke_hoach_von ") {
		t.Errorf("đọc sai bảng: %q", l.sql)
	}

	// The same store, a different commune in the context, a different $1. This is what "scoped
	// repository" means in practice — and a store caching the first commune it saw would fail here.
	k2 := &khoGia{hang: mauMotDong()}
	if _, err := dungKho(k2).DanhSach(ctxXa(xaThuHai)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if k2.lenh[0].args[0] != string(xaThuHai) {
		t.Errorf("$1 = %v, muốn %q", k2.lenh[0].args[0], xaThuHai)
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
	// `dang_dung` MUST NOT BE IN THE PREDICATE, and this is the assertion most likely to be
	// "fixed" by somebody who thinks a catalogue should only return live rows. A category taken
	// out of use is still read: the catalogue screen shows it with a "Đã tắt" chip, and a capital
	// plan line of an earlier budget year holds its code AS A VALUE and would otherwise render
	// with no category name at all. It is selected as a column — hence the two specific shapes
	// below rather than a bare search for the word.
	if strings.Contains(q, "dang_dung =") || strings.Contains(q, "dang_dung IS") {
		t.Errorf("câu lệnh lọc mất hạng mục đã tắt: %q", q)
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
	if len(l.args) < 2 || l.args[1] != int64(TranDanhMucHangMuc+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], TranDanhMucHangMuc+1)
	}
}

// --- what comes back -------------------------------------------------------------------------

func TestDanhSachDocDungTungCot(t *testing.T) {
	// THE ONE THAT MATTERS MOST. Every column here is read BY POSITION, in lockstep with
	// cotHangMuc, and the compiler cannot help: `ma`/`nhan` are both TEXT and `la_mac_dinh`/
	// `dang_dung` are both BOOLEAN. A swap in either pair compiles, runs, and passes any test whose
	// fixture happens to agree with itself.
	k := &khoGia{hang: mauMotDong()}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.ID != "hm-001" || mot.Ma != "xay-dung-moi" || mot.Nhan != "Xây dựng mới" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	// The pair that cannot be caught by type: both are BOOLEAN and adjacent. The fixture sets them
	// to OPPOSITE values on purpose — swapped, this row would report a category the commune took
	// out of use as the one every form pre-selects.
	if !mot.LaMacDinh || mot.DangDung {
		t.Errorf("la_mac_dinh / dang_dung đọc ngược: %+v", mot)
	}
}

func TestDanhSachXaChuaCoDongNaoTraDanhSachRong(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE. Migration 0003 creates the table and seeds nothing: a row
	// here carries a tenant_id and the migration runner has no commune in it, so the step that
	// would sow a commune's system rows is commune onboarding — which does not exist yet. An empty
	// list is correct, not a failure, and it is a list rather than a nil the caller must branch on.
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
	k := &khoGia{hang: nhieuDong(TranDanhMucHangMuc)}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranDanhMucHangMuc {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranDanhMucHangMuc)
	}
}

func TestDanhSachVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. And the rows already read are DROPPED —
	// handing back a list the caller might render anyway is how a refusal turns back into a silent
	// truncation one careless `if err != nil { log }` later.
	k := &khoGia{hang: nhieuDong(TranDanhMucHangMuc + 1)}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, ErrQuaNhieuHangMuc) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuHangMuc", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- failures ----------------------------------------------------------------------------------

func TestDanhSachLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Errors are wrapped with %w, never swallowed with `_`. The caller distinguishes the ceiling
	// from an ordinary failure with errors.Is, which only works if the chain is intact.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGia{loi: goc}

	ra, err := dungKho(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuHangMuc) {
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
