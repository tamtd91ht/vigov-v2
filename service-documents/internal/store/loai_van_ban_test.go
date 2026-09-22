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

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// WHAT THIS FILE PROVES, AND WHAT IT DOES NOT — stated first, because a test suite that prints
// `ok` while asserting nothing is this repository's worst known trap, and the sibling file
// loai_van_ban_pg_test.go is exactly that on a machine with no PostgreSQL.
//
// It runs with NO database. The fake driver below RECORDS the statement the store builds and hands
// back the rows the test supplied; it never executes SQL. So:
//
//	PROVED HERE   the commune reaches the query as $1 and comes from the CONTEXT, never from an
//	              argument · the soft-delete predicate is in the statement and `dang_dung` is NOT ·
//	              the order and its tie-break are in the statement · the LIMIT really is the ceiling
//	              PLUS ONE · the refusal fires at ceiling+1 and returns NO rows · the positional
//	              Scan lines up with the column list, BY NAME · a driver failure is wrapped, not
//	              swallowed · no commune in the context is a panic, never a default.
//
//	NOT PROVED    anything PostgreSQL does with that statement. The columns come from the store's
//	              OWN SELECT list, so a column that does not exist in `loai_van_ban` — a typo, a
//	              name the migration never created — passes here without a murmur. Nor does this
//	              say anything about the partial index being used, the hash partitioning routing a
//	              row, or the danh_muc_ba_tang trigger refusing what it claims to refuse. That half
//	              needs a real server, which is why loai_van_ban_pg_test.go stays and becomes
//	              valuable the day VIGOV_TEST_DSN exists.
//
// The column-name check is the one worth explaining: the fake builds each row BY COLUMN NAME out of
// the statement, so reordering cotLoaiVanBan without reordering the Scan turns these tests red.
// Read by position, `ma`/`nhan` and `dang_dung`/`la_mac_dinh` are two pairs of adjacent same-typed
// columns — swapping either produces no error at all, only slugs where labels belong, or a form
// pre-selecting a type the commune has taken out of use.

var xaThu = tenant.ID("01JA" + strings.Repeat("A", 22))

// ctxXa and dungLoaiVanBanStore are declared in loai_van_ban_pg_test.go and reused here on
// purpose: two helpers building the same context, or the same store, are two that drift.

// --- the fake driver --------------------------------------------------------------------------

type lenhGia struct {
	sql  string
	args []driver.Value
}

// hangGia is one row the fake returns. The values are distinct per column and, within each type,
// distinct from each other, so a mis-wired Scan shows up as WRONG DATA rather than as a zero value
// that looks plausible.
type hangGia struct {
	id, ma, nhan      string
	dangDung, macDinh bool
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
	case "dang_dung":
		return h.dangDung
	case "la_mac_dinh":
		return h.macDinh
	case "thu_tu":
		// int64 AND NOT int: database/sql only accepts the driver.Value set, and `int` is not in
		// it. A fake that handed back `int` would fail every Scan with a message about conversion
		// rather than about the column, which is the kind of noise that gets a fake deleted.
		return int64(h.thuTu)
	case "nguon":
		return h.nguon
	case "ma_nguon_re_nhanh":
		return h.reNhanh
	default:
		// A column was added to cotLoaiVanBan and not here. Failing loudly beats scanning a nil
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

// QueryContext is what makes this a driver.QueryerContext, so database/sql hands the statement
// over whole instead of preparing it — which is the only reason the statement can be recorded and
// asserted on at all.
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
		for i, ten := range cot {
			mot[i] = h.giaTri(ten)
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

// dungKhoGia builds the real store on top of the fake driver, through the SAME constructor
// production uses — sql.OpenDB turns a driver.Connector into a *sql.DB, and nothing in the store
// was widened to accommodate this.
func dungKhoGia(k *khoGia) *LoaiVanBanStore {
	return dungLoaiVanBanStore(sql.OpenDB(k))
}

// mauMotDong is one row whose every field is distinguishable from every other: a Scan reading
// `nhan` into Ma, or `dang_dung` into LaMacDinh, cannot produce a passing assertion.
func mauMotDong() []hangGia {
	return []hangGia{{
		id: "lvb-001", ma: "quyet-dinh", nhan: "Quyết định",
		// OPPOSITE VALUES ON PURPOSE. Both are BOOLEAN and adjacent in the column list, so equal
		// values would make a swap invisible. This row is the awkward-but-legal combination the
		// schema allows: a default that has been taken out of use.
		dangDung: false, macDinh: true,
		// `thu_tu` is deliberately NOT 0 and NOT 1: a zero would be indistinguishable from an
		// unscanned field, and 1 from a length. `nguon`/`ma_nguon_re_nhanh` describe a TIER 3 row,
		// the one combination whose tier cannot be guessed from either column alone.
		thuTu: 7, nguon: "he-thong", reNhanh: true,
	}}
}

func nhieuDong(n int) []hangGia {
	ra := make([]hangGia, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, hangGia{
			id:   fmt.Sprintf("lvb-%04d", i),
			ma:   fmt.Sprintf("loai-%04d", i),
			nhan: fmt.Sprintf("Loại %04d", i),
			// dang_dung true: rows in ordinary use, which is what a commune at its ceiling would
			// actually hold. `nguon` is the commune's own, which is what a catalogue grows into.
			dangDung: true,
			thuTu:    i,
			nguon:    "don-vi",
		})
	}
	return ra
}

// --- the statement the store builds --------------------------------------------------------------

func TestDanhSachBuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument of DanhSach and cannot be: it
	// arrives in the context and Scoped.Query binds it to $1. If it ever became a parameter, a
	// caller could pass another commune's id and nothing in this package would notice.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu))); err != nil {
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
	// The same store type, a different commune in the context, a different $1. This is what
	// "scoped repository" means in practice — and a store that cached the first commune it saw,
	// or that read one from its own construction, would fail here.
	k2 := &khoGia{hang: mauMotDong()}
	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := dungKhoGia(k2).DanhSach(ctxXa(string(xaKhac))); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if k2.lenh[0].args[0] != string(xaKhac) {
		t.Errorf("$1 = %v, muốn %q", k2.lenh[0].args[0], xaKhac)
	}
}

func TestDanhSachChiLocDongDaXoaMemVaSapXepOnDinh(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always. And
	// the order is total: `thu_tu` is the commune's own arrangement and `nhan` breaks ties, so two
	// calls cannot return the same rows in a different sequence.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu))); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	q := k.lenh[0].sql
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
	if !strings.Contains(q, "ORDER BY thu_tu, nhan") {
		t.Errorf("thứ tự không ổn định: %q", q)
	}
	// `dang_dung` must NOT be in the predicate, and this assertion is the one most likely to be
	// "fixed" by somebody being helpful: a type taken out of use is still read — the catalogue
	// screen shows it with a "Đã tắt" chip, and a document already registered under it needs its
	// label. Filtering here would silently rewrite what old documents display.
	if strings.Contains(q, "dang_dung =") || strings.Contains(q, "dang_dung IS") ||
		strings.Contains(q, "AND dang_dung") {
		t.Errorf("câu lệnh lọc mất loại đã tắt: %q", q)
	}
}

func TestDanhSachLayDuTranCongMot(t *testing.T) {
	// The LIMIT is the ceiling PLUS ONE, and that single character is what makes "there are too
	// many" detectable at all. Asking for exactly the ceiling returns a full list indistinguishable
	// from a complete one of that size — the truncation this route refuses to perform, performed by
	// the bound meant to prevent it.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu))); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "LIMIT $2") {
		t.Fatalf("không có trần trong câu lệnh: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != int64(TranDanhMucLoaiVanBan+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[1:], TranDanhMucLoaiVanBan+1)
	}
}

func TestDanhSachDocDungBangTable(t *testing.T) {
	// The table name is built by the store, not by Scoped: a typo here would reach PostgreSQL as a
	// relation that does not exist, and the pg suite is the only place that catches THAT. What this
	// asserts is the cheaper half — that the read goes to the catalogue table and to no other.
	k := &khoGia{hang: mauMotDong()}

	if _, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu))); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if !strings.Contains(k.lenh[0].sql, " FROM loai_van_ban ") {
		t.Errorf("đọc nhầm bảng: %q", k.lenh[0].sql)
	}
}

// --- what comes back ------------------------------------------------------------------------------

func TestDanhSachDocDungTungCot(t *testing.T) {
	// THE TEST THAT MATTERS MOST HERE, and the reason the fake builds rows by column NAME: the Scan
	// is positional, and two pairs of adjacent same-typed columns can be swapped without the
	// compiler, `go vet` or an ordinary test noticing.
	k := &khoGia{hang: mauMotDong()}

	ra, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu)))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.ID != "lvb-001" || mot.Ma != "quyet-dinh" || mot.Nhan != "Quyết định" {
		t.Errorf("ba cột TEXT đọc sai chỗ — ma/nhan có thể đã hoán vị: %+v", mot)
	}
	// The pair that no type can catch: both BOOLEAN, adjacent, and set to OPPOSITE values by the
	// fixture. Swapped, the screen pre-selects a type the commune has taken out of use.
	if mot.DangDung || !mot.LaMacDinh {
		t.Errorf("dang_dung / la_mac_dinh đọc ngược: %+v", mot)
	}
	// The three columns the WRITE surface added. `thu_tu` is what the configuration screen edits;
	// `nguon` and `ma_nguon_re_nhanh` are what decide which buttons that screen may even draw, so a
	// Scan reading them into the wrong field offers `Xoá` on a row the database will refuse to
	// delete — a button that always fails, on the one screen an administrator uses to fix things.
	if mot.ThuTu != 7 {
		t.Errorf("thu_tu đọc sai: %+v", mot)
	}
	if mot.Nguon != "he-thong" || !mot.MaNguonReNhanh {
		t.Errorf("nguon / ma_nguon_re_nhanh đọc sai: %+v", mot)
	}
	if mot.Tang() != domain.TangReNhanh {
		t.Errorf("tầng suy ra = %d, muốn %d (tầng 3)", mot.Tang(), domain.TangReNhanh)
	}
}

func TestDanhSachXaChuaCoDongNaoTraLatRong(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE. Migration 0003 creates the table and sows nothing, and the
	// onboarding step that would sow a commune's system rows does not exist. An empty catalogue is
	// correct, not a failure — and it is a slice, never a nil the caller has to branch on.
	k := &khoGia{}

	ra, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu)))
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

// --- the ceiling ------------------------------------------------------------------------------------

func TestDanhSachDungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid — and the screen it breaks is the one a document is
	// registered on.
	k := &khoGia{hang: nhieuDong(TranDanhMucLoaiVanBan)}

	ra, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu)))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranDanhMucLoaiVanBan {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranDanhMucLoaiVanBan)
	}
}

func TestDanhSachVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// THE DECISION THIS PINS: refuse, do not truncate. And the rows already read are DROPPED —
	// handing back a list the caller might render anyway is how a refusal turns back into a silent
	// truncation one careless `if err != nil { log }` later.
	k := &khoGia{hang: nhieuDong(TranDanhMucLoaiVanBan + 1)}

	ra, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu)))
	if !errors.Is(err, ErrQuaNhieuLoaiVanBan) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuLoaiVanBan", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- failures ---------------------------------------------------------------------------------------

func TestDanhSachLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Rule 6 of the service pattern: wrapped with %w, never swallowed with `_`. The handler
	// distinguishes the ceiling from an ordinary failure with errors.Is, and that only works if the
	// chain is intact — a fmt.Errorf with %v here would make the two indistinguishable and the
	// route would log the wrong sentence about a commune's data.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGia{loi: goc}

	ra, err := dungKhoGia(k).DanhSach(ctxXa(string(xaThu)))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuLoaiVanBan) {
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
	_, _ = dungKhoGia(k).DanhSach(context.Background())
}
