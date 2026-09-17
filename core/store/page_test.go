package store_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// THE DEFECT CLASS THIS FILE CLOSES: a list route that skips or repeats records between pages.
//
// It is the quietest failure in the whole pagination story. The list renders, the counts look
// plausible, nothing errors — and a citizen paging through a register is shown one record twice
// while another is never shown at all. On an archival register that is not a rendering glitch:
// it is a record that, as far as the person looking is concerned, does not exist.
//
// WHY THE FAKE DRIVER ACTUALLY EXECUTES THE KEYSET: recording the statement and asserting on its
// text would only prove the string was built, not that walking it visits every row exactly once.
// So this driver implements the small part of PostgreSQL the walk depends on — the row-value
// comparison, the ORDER BY including its tie-break, and LIMIT — over an in-memory table. Drop the
// `, id` from the ORDER BY and the dataset below really does lose a row.
//
// No PostgreSQL, because a test that needs infrastructure is a test that stops being run.

const (
	xaA = tenant.ID("01J0000000000000000000000A")
	xaB = tenant.ID("01J0000000000000000000000B")
)

var moc0 = time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)

// Two rows share ngay_tao ON PURPOSE (a3 and a4), and with limit 3 the pair straddles a page
// boundary — which is the only arrangement that can tell a total order from a partial one.
var duLieu = []hang{
	{xa: string(xaA), id: "01JA0000000000000000000001", ma: "PA-0001", ngayTao: moc0.Add(1 * time.Minute)},
	{xa: string(xaA), id: "01JA0000000000000000000002", ma: "PA-0002", ngayTao: moc0.Add(2 * time.Minute)},
	{xa: string(xaA), id: "01JA0000000000000000000003", ma: "PA-0003", ngayTao: moc0.Add(3 * time.Minute)},
	{xa: string(xaA), id: "01JA0000000000000000000004", ma: "PA-0004", ngayTao: moc0.Add(3 * time.Minute)},
	{xa: string(xaA), id: "01JA0000000000000000000005", ma: "PA-0005", ngayTao: moc0.Add(4 * time.Minute)},
	{xa: string(xaA), id: "01JA0000000000000000000006", ma: "PA-0006", ngayTao: moc0.Add(5 * time.Minute)},
	{xa: string(xaA), id: "01JA0000000000000000000007", ma: "PA-0007", ngayTao: moc0.Add(6 * time.Minute)},

	// Commune B, deliberately LATER than every A row: a cursor from A replayed here would return
	// these if the scope were ever taken from the cursor instead of from the context.
	{xa: string(xaB), id: "01JB0000000000000000000001", ma: "PA-0001", ngayTao: moc0.Add(10 * time.Minute)},
	{xa: string(xaB), id: "01JB0000000000000000000002", ma: "PA-0002", ngayTao: moc0.Add(11 * time.Minute)},
}

var idCuaA = []string{
	"01JA0000000000000000000001", "01JA0000000000000000000002",
	"01JA0000000000000000000003", "01JA0000000000000000000004",
	"01JA0000000000000000000005", "01JA0000000000000000000006",
	"01JA0000000000000000000007",
}

var (
	cotNgayTao = page.Col("created_at", "ngay_tao", page.KindTime)
	cotMa      = page.Col("code", "ma", page.KindText)
	danhSach   = page.NewAllowlist(page.Asc, cotNgayTao, cotMa)
)

type phanAnh struct {
	ID      string
	Ma      string
	NgayTao time.Time
}

func quet(rows *sql.Rows) (phanAnh, page.Anchor, error) {
	var p phanAnh
	if err := rows.Scan(&p.ID, &p.Ma, &p.NgayTao); err != nil {
		return phanAnh{}, page.Anchor{}, err
	}
	return p, page.Anchor{Key: page.TimeKey(p.NgayTao), ID: p.ID}, nil
}

func spec() store.PageSpec {
	return store.PageSpec{Columns: "id, ma, ngay_tao", Table: "phan_anh", Filter: "AND deleted_at IS NULL"}
}

// --- harness ----------------------------------------------------------------------------------

type banThu struct {
	db  *store.DB
	ghi *ghiChepTrang
}

func dungBanThu(t *testing.T) *banThu {
	t.Helper()
	g := &ghiChepTrang{}
	db := sql.OpenDB(ketNoiTrang{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return &banThu{db: store.New(db), ghi: g}
}

func (b *banThu) trang(t *testing.T, xa tenant.ID, truyVan string) (page.Result[phanAnh], error) {
	t.Helper()
	ctx := tenant.Into(context.Background(), xa)
	q, err := url.ParseQuery(truyVan)
	if err != nil {
		t.Fatalf("chuỗi truy vấn hỏng trong chính bài kiểm: %v", err)
	}
	yc, err := page.Parse(q, danhSach)
	if err != nil {
		// The handler's own shape: a rejected request runs no statement at all.
		return page.NewResult[phanAnh](), err
	}
	return store.QueryPage(ctx, b.db.For(ctx), spec(), yc, quet)
}

// duyetHet walks every page and returns the ids in the order they were handed out, plus the number
// of pages it took. It stops hard at 50 iterations so a broken cursor is a failure, not a hang.
func (b *banThu) duyetHet(t *testing.T, xa tenant.ID, truyVan string) ([]string, [][]string) {
	t.Helper()
	var ids []string
	var trang [][]string
	conTro := ""
	for i := 0; ; i++ {
		if i > 50 {
			t.Fatal("con trỏ không tiến — vòng lặp vô hạn, đây là cách nó biểu hiện trên sản phẩm")
		}
		q := truyVan
		if conTro != "" {
			q += "&cursor=" + url.QueryEscape(conTro)
		}
		kq, err := b.trang(t, xa, q)
		if err != nil {
			t.Fatalf("trang %d lỗi: %v", i, err)
		}
		var mot []string
		for _, p := range kq.Items {
			ids = append(ids, p.ID)
			mot = append(mot, p.ID)
		}
		trang = append(trang, mot)
		if !kq.HasMore {
			if kq.NextCursor != "" {
				t.Error("has_more = false mà vẫn có next_cursor — client sẽ xin một trang không tồn tại")
			}
			return ids, trang
		}
		if kq.NextCursor == "" {
			t.Fatal("has_more = true mà không có next_cursor — không có cách nào đi tiếp")
		}
		conTro = kq.NextCursor
	}
}

// --- the walk ------------------------------------------------------------------------------------

func TestTrangDauKhongCanConTro(t *testing.T) {
	b := dungBanThu(t)

	kq, err := b.trang(t, xaA, "limit=3")
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 3 {
		t.Fatalf("trang đầu có %d dòng, muốn 3", len(kq.Items))
	}
	if !kq.HasMore || kq.NextCursor == "" {
		t.Fatal("còn dữ liệu mà has_more/next_cursor không nói thế")
	}

	l := b.ghi.chua("FROM phan_anh")
	if l == nil {
		t.Fatal("không câu lệnh nào chạm tới driver")
	}
	if strings.Contains(l.sql, " AND (ngay_tao, id)") {
		t.Errorf("trang đầu vẫn kèm mệnh đề mốc: %q", l.sql)
	}
	if !strings.Contains(l.sql, "ORDER BY ngay_tao ASC, id ASC") {
		t.Errorf("thiếu thứ tự toàn phần: %q", l.sql)
	}
}

func TestDuyetHetKhongLapKhongSot(t *testing.T) {
	// The whole point of the package in one assertion. Note the dataset has a duplicate ngay_tao:
	// remove `, id` from the ORDER BY or from the row comparison and this test loses a record.
	b := dungBanThu(t)

	ids, trang := b.duyetHet(t, xaA, "limit=3")

	if len(ids) != len(idCuaA) {
		t.Fatalf("duyệt được %d bản ghi, muốn %d — %v", len(ids), len(idCuaA), ids)
	}
	for i, muon := range idCuaA {
		if ids[i] != muon {
			t.Fatalf("vị trí %d là %s, muốn %s — thứ tự đi qua các trang: %v", i, ids[i], muon, ids)
		}
	}
	dem := map[string]int{}
	for _, id := range ids {
		dem[id]++
	}
	for id, n := range dem {
		if n != 1 {
			t.Errorf("%s xuất hiện %d lần", id, n)
		}
	}
	if len(trang) != 3 {
		t.Errorf("chia thành %d trang, muốn 3 (3+3+1): %v", len(trang), trang)
	}
}

func TestHaiBanGhiTrungKhoaSapXepVanDuyetDu(t *testing.T) {
	// THE CASE THAT PROVES THE TIE-BREAK. a3 and a4 share ngay_tao and, at limit 3, sit either side
	// of a page boundary. With a sort on ngay_tao alone the anchor is "everything after 09:03", and
	// a4 — which is also at 09:03 — is never handed out. Nothing errors; the record simply is not
	// in the list.
	b := dungBanThu(t)

	_, trang := b.duyetHet(t, xaA, "limit=3")
	if len(trang) < 2 {
		t.Fatal("bộ dữ liệu không còn chia được trang — bài kiểm này không chứng minh gì nữa")
	}
	cuoiTrang1 := trang[0][len(trang[0])-1]
	dauTrang2 := trang[1][0]
	if cuoiTrang1 != "01JA0000000000000000000003" || dauTrang2 != "01JA0000000000000000000004" {
		t.Fatalf("cặp trùng khoá không nằm hai bên ranh giới trang (%s | %s) — sửa bộ dữ liệu, "+
			"nếu không bài kiểm không còn kiểm khoá phá hoà", cuoiTrang1, dauTrang2)
	}

	l := b.ghi.chua(" AND (ngay_tao, id) > ")
	if l == nil {
		t.Fatal("mốc không so sánh theo cặp (khoá, id) — bản ghi trùng khoá sẽ bị bỏ sót")
	}
}

func TestChieuGiamDungToanTuVaThuTuNguoc(t *testing.T) {
	b := dungBanThu(t)

	ids, _ := b.duyetHet(t, xaA, "limit=2&order=desc")
	if len(ids) != len(idCuaA) {
		t.Fatalf("duyệt ngược được %d bản ghi, muốn %d — %v", len(ids), len(idCuaA), ids)
	}
	for i, id := range ids {
		muon := idCuaA[len(idCuaA)-1-i]
		if id != muon {
			t.Fatalf("vị trí %d là %s, muốn %s — %v", i, id, muon, ids)
		}
	}
	if l := b.ghi.chua(" AND (ngay_tao, id) < "); l == nil {
		t.Error("chiều giảm phải dùng toán tử < — dùng > sẽ đi ngược lại về đầu danh sách")
	}
	if l := b.ghi.chua("ORDER BY ngay_tao DESC, id DESC"); l == nil {
		t.Error("khoá phá hoà phải đi cùng chiều với khoá sắp xếp")
	}
}

func TestTrangCuoiBaoHetVaKhongConConTro(t *testing.T) {
	b := dungBanThu(t)

	// 7 rows, limit 7: the page is full and there is nothing after it. This is the case a naive
	// `has_more = len(items) == limit` gets wrong, sending the client to an empty page.
	kq, err := b.trang(t, xaA, "limit=7")
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 7 {
		t.Fatalf("có %d dòng, muốn 7", len(kq.Items))
	}
	if kq.HasMore {
		t.Error("has_more = true ở trang cuối — client sẽ xin một trang rỗng")
	}
	if kq.NextCursor != "" {
		t.Error("trang cuối vẫn trả con trỏ")
	}
}

func TestLayThemMotDongNhungKhongTraVe(t *testing.T) {
	// limit+1 is how has_more is known without COUNT(*). The extra row is a signal: it must never
	// reach the response, or the client gets one row more than it asked for and the next page
	// starts one row late.
	b := dungBanThu(t)

	kq, err := b.trang(t, xaA, "limit=3")
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 3 {
		t.Fatalf("trả về %d dòng cho limit=3", len(kq.Items))
	}
	l := b.ghi.chua("FROM phan_anh")
	if n := l.args[len(l.args)-1]; n != int64(4) {
		t.Errorf("LIMIT = %v, muốn 4 (limit+1)", n)
	}
}

func TestKhongDemTong(t *testing.T) {
	// §5 #4: no COUNT(*) to decorate a list. phan_anh is partitioned 32 ways, so a count is 32
	// partition scans on every page request, and the number is not even in the response shape.
	b := dungBanThu(t)
	b.duyetHet(t, xaA, "limit=2")

	for _, l := range b.ghi.tatCa() {
		thap := strings.ToLower(l.sql)
		if strings.Contains(thap, "count(") {
			t.Errorf("câu lệnh đếm tổng: %q", l.sql)
		}
		if strings.Contains(thap, "offset") {
			t.Errorf("câu lệnh dùng offset: %q", l.sql)
		}
	}
}

// --- the bound ---------------------------------------------------------------------------------

func TestLimitVuotTranBiKepChuKhongLoi(t *testing.T) {
	b := dungBanThu(t)

	kq, err := b.trang(t, xaA, "limit=10000")
	if err != nil {
		t.Fatalf("xin quá trần phải nhận trần, không phải lỗi: %v", err)
	}
	if len(kq.Items) != len(idCuaA) {
		t.Errorf("trả về %d dòng", len(kq.Items))
	}
	l := b.ghi.chua("FROM phan_anh")
	if n := l.args[len(l.args)-1]; n != int64(page.MaxLimit+1) {
		t.Errorf("LIMIT = %v, muốn %d — trần phía máy chủ phải tới được câu lệnh",
			n, page.MaxLimit+1)
	}
}

func TestYeuCauSaiThiKhongCauLenhNaoChay(t *testing.T) {
	// A rejected request must not reach the database at all: a garbage cursor that produced a
	// statement would be a string from a URL taking part in a query, which is the shape this whole
	// design exists to make impossible.
	cases := map[string]error{
		"limit=0":                        page.ErrLimit,
		"limit=-5":                       page.ErrLimit,
		"limit=nhieu":                    page.ErrLimit,
		"cursor=khong-phai-con-tro":      page.ErrCursor,
		"cursor=" + url.QueryEscape("!"): page.ErrCursor,
		"sort=ho_ten":                    page.ErrSort,
		"sort=" + url.QueryEscape("ngay_tao; DROP TABLE phan_anh"): page.ErrSort,
		"order=nguoc": page.ErrSort,
	}
	for truyVan, muon := range cases {
		t.Run(truyVan, func(t *testing.T) {
			b := dungBanThu(t)
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC trên tham số từ URL: %v", r)
				}
			}()
			_, err := b.trang(t, xaA, truyVan)
			if !errors.Is(err, muon) {
				t.Fatalf("muốn %v, nhận %v", muon, err)
			}
			if n := len(b.ghi.tatCa()); n != 0 {
				t.Fatalf("đã chạy %d câu lệnh cho một yêu cầu bị từ chối: %v", n, b.ghi.tatCa())
			}
		})
	}
}

func TestFilterTuMangThuTuHoacGioiHanThiTuChoi(t *testing.T) {
	// The caller owns the filter, QueryPage owns the order and the bound. A caller-supplied ORDER BY
	// does not fail loudly — it quietly reorders the keyset walk, and the rows start going missing.
	b := dungBanThu(t)
	ctx := tenant.Into(context.Background(), xaA)
	yc, err := page.New(danhSach, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	for _, loc := range []string{
		"AND deleted_at IS NULL ORDER BY ho_ten",
		"AND deleted_at IS NULL LIMIT 5",
		"AND deleted_at IS NULL OFFSET 10",
		"AND deleted_at IS NULL; DROP TABLE phan_anh",
	} {
		s := spec()
		s.Filter = loc
		if _, err := store.QueryPage(ctx, b.db.For(ctx), s, yc, quet); err == nil {
			t.Errorf("Filter %q được chấp nhận", loc)
		}
	}
	if n := len(b.ghi.tatCa()); n != 0 {
		t.Errorf("đã chạy %d câu lệnh dù spec không hợp lệ", n)
	}
}

// --- the commune ---------------------------------------------------------------------------------

func TestThamSoMotVanLaXaSauKhiThemMenhDeConTro(t *testing.T) {
	// The contract of pkg/store, unchanged by pagination: $1 is the commune, the caller's own
	// placeholders start at $2, and the cursor's placeholders come after those. Swap any of them
	// and every list silently reads with the wrong scope.
	b := dungBanThu(t)
	ctx := tenant.Into(context.Background(), xaA)

	yc, err := page.New(danhSach, "", "asc", "3", page.Encode(cotNgayTao, page.Asc,
		page.Anchor{Key: page.TimeKey(moc0.Add(2 * time.Minute)), ID: "01JA0000000000000000000002"}))
	if err != nil {
		t.Fatal(err)
	}
	s := spec()
	s.Filter = "AND deleted_at IS NULL AND trang_thai = $2"
	s.Args = []any{"dang-xu-ly"}

	if _, err := store.QueryPage(ctx, b.db.For(ctx), s, yc, quet); err != nil {
		t.Fatal(err)
	}

	l := b.ghi.chua("FROM phan_anh")
	if l == nil {
		t.Fatal("không câu lệnh nào chạm tới driver")
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Fatalf("câu lệnh thiếu ràng buộc xã: %q", l.sql)
	}
	if !strings.Contains(l.sql, "AND (ngay_tao, id) > ($3, $4)") {
		t.Fatalf("mệnh đề mốc không nối tiếp tham số của người gọi: %q", l.sql)
	}
	if len(l.args) != 5 {
		t.Fatalf("có %d tham số, muốn 5: %v", len(l.args), l.args)
	}
	if l.args[0] != string(xaA) {
		t.Errorf("$1 = %v, muốn mã xã %q — $1 phải LUÔN là xã", l.args[0], xaA)
	}
	if l.args[1] != "dang-xu-ly" {
		t.Errorf("$2 = %v, muốn tham số của người gọi", l.args[1])
	}
	if got, ok := l.args[2].(time.Time); !ok || !got.Equal(moc0.Add(2*time.Minute)) {
		t.Errorf("$3 = %#v, muốn mốc thời gian CÓ KIỂU — giá trị giải từ con trỏ phải được ràng buộc", l.args[2])
	}
	if l.args[3] != "01JA0000000000000000000002" {
		t.Errorf("$4 = %v, muốn id của mốc", l.args[3])
	}

	// And nothing decoded from the cursor may appear in the statement TEXT.
	for _, cam := range []string{"01JA0000000000000000000002", "2026-03-14", "dang-xu-ly"} {
		if strings.Contains(l.sql, cam) {
			t.Errorf("giá trị %q bị nối vào câu lệnh: %q", cam, l.sql)
		}
	}
}

func TestConTroCuaXaAKhongLayDuocDuLieuXaA(t *testing.T) {
	// A cursor is not a capability. Pasted into commune B's domain it can only move the anchor
	// WITHIN commune B — the scope comes from the context and is bound to $1, and the cursor
	// carries no commune at all (rule 1, forbidden #2).
	b := dungBanThu(t)

	kqA, err := b.trang(t, xaA, "limit=3")
	if err != nil {
		t.Fatal(err)
	}
	if kqA.NextCursor == "" {
		t.Fatal("không có con trỏ của xã A để thử")
	}

	kqB, err := b.trang(t, xaB, "limit=3&cursor="+url.QueryEscape(kqA.NextCursor))
	if err != nil {
		t.Fatalf("con trỏ của xã khác phải chạy được trong phạm vi xã B, không phải lỗi: %v", err)
	}
	for _, p := range kqB.Items {
		if !strings.HasPrefix(p.ID, "01JB") {
			t.Fatalf("xã B nhận được bản ghi %s của xã A — RÒ RỈ GIỮA HAI XÃ", p.ID)
		}
	}

	cuoi := b.ghi.tatCa()
	l := cuoi[len(cuoi)-1]
	if l.args[0] != string(xaB) {
		t.Errorf("$1 = %v, muốn xã B — con trỏ không được quyết định phạm vi", l.args[0])
	}
	for _, a := range l.args {
		if s, ok := a.(string); ok && s == string(xaA) {
			t.Errorf("mã xã A lọt vào tham số của câu lệnh xã B: %v", l.args)
		}
	}
	if strings.Contains(l.sql, string(xaA)) {
		t.Errorf("mã xã A lọt vào câu lệnh: %q", l.sql)
	}
}

func TestKhongCoXaThiPanicChuKhongDocMoXa(t *testing.T) {
	b := dungBanThu(t)
	yc, err := page.New(danhSach, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("thiếu xã trong context mà vẫn đọc — phải panic (tenant.MustFrom)")
		}
		if n := len(b.ghi.tatCa()); n != 0 {
			t.Errorf("đã chạy %d câu lệnh dù không biết xã nào", n)
		}
	}()
	ctx := context.Background()
	_, _ = store.QueryPage(ctx, b.db.For(ctx), spec(), yc, quet)
}

// --- fake driver: a small, honest keyset engine -----------------------------------------------

type hang struct {
	xa      string
	id      string
	ma      string
	ngayTao time.Time
}

func (h hang) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return h.id
	case "ma":
		return h.ma
	case "ngay_tao":
		return h.ngayTao
	case "tenant_id":
		return h.xa
	default:
		return nil
	}
}

type lenh struct {
	sql  string
	args []driver.Value
}

type ghiChepTrang struct {
	mu   sync.Mutex
	lenh []lenh
}

func (g *ghiChepTrang) them(l lenh) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lenh = append(g.lenh, l)
}

func (g *ghiChepTrang) tatCa() []lenh {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]lenh(nil), g.lenh...)
}

func (g *ghiChepTrang) chua(manh string) *lenh {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.lenh {
		if strings.Contains(g.lenh[i].sql, manh) {
			return &g.lenh[i]
		}
	}
	return nil
}

var (
	reCot     = regexp.MustCompile(`(?s)^SELECT (.+?) FROM `)
	reCap     = regexp.MustCompile(`AND \(([a-z_]+), ([a-z_]+)\) ([<>]) \(\$(\d+), \$(\d+)\)`)
	reDon     = regexp.MustCompile(`AND ([a-z_]+) ([<>]) \$(\d+)`)
	reThuTu   = regexp.MustCompile(`ORDER BY ([a-z_]+) (ASC|DESC)(?:, ([a-z_]+) (ASC|DESC))?`)
	reGioiHan = regexp.MustCompile(`LIMIT \$(\d+)`)
)

// chay is the whole fake engine: filter by commune, apply the anchor comparison, order, limit.
//
// It reads the statement rather than being told what to do, which is what makes a mutation in
// pkg/store/page.go show up here as WRONG DATA instead of as a fake that was updated to match.
func chay(q string, args []driver.Value) (*hangGia, error) {
	cot := strings.Split(reCot.FindStringSubmatch(q)[1], ", ")
	lay := func(n int) driver.Value { return args[n-1] } // $n

	var ra []hang
	for _, h := range duLieu {
		if h.xa != args[0] {
			continue
		}
		ra = append(ra, h)
	}

	if m := reCap.FindStringSubmatch(q); m != nil {
		khoa, phaHoa, op := m[1], m[2], m[3]
		mkA, _ := strconv.Atoi(m[4])
		mkB, _ := strconv.Atoi(m[5])
		ra = loc(ra, func(h hang) bool {
			c := soSanh(h.giaTri(khoa), lay(mkA))
			if c == 0 {
				c = soSanh(h.giaTri(phaHoa), lay(mkB))
			}
			return hop(c, op)
		})
	} else if m := reDon.FindStringSubmatch(q); m != nil {
		khoa, op := m[1], m[2]
		mk, _ := strconv.Atoi(m[3])
		ra = loc(ra, func(h hang) bool { return hop(soSanh(h.giaTri(khoa), lay(mk)), op) })
	}

	if m := reThuTu.FindStringSubmatch(q); m != nil {
		k1, c1, k2, c2 := m[1], m[2], m[3], m[4]
		if k2 == "" {
			// NO TIE-BREAK DECLARED. PostgreSQL is then free to return rows of equal sort key in any
			// order it likes, and it does not promise the same order twice. The fake models the
			// UNHELPFUL choice on purpose — ties come back opposite to the declared direction —
			// because the helpful one would let an untied ORDER BY pass this test and fail in
			// production, which is precisely the defect being hunted.
			k2 = "id"
			c2 = "ASC"
			if c1 == "ASC" {
				c2 = "DESC"
			}
		}
		sort.SliceStable(ra, func(i, j int) bool {
			c := soSanh(ra[i].giaTri(k1), ra[j].giaTri(k1))
			if c == 0 && k2 != "" {
				c = soSanh(ra[i].giaTri(k2), ra[j].giaTri(k2))
				if c2 == "DESC" {
					return c > 0
				}
				return c < 0
			}
			if c1 == "DESC" {
				return c > 0
			}
			return c < 0
		})
	}

	if m := reGioiHan.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[1])
		gh, ok := lay(n).(int64)
		if !ok {
			return nil, errors.New("driver giả: LIMIT không phải số nguyên")
		}
		if int64(len(ra)) > gh {
			ra = ra[:gh]
		}
	}

	hg := &hangGia{cot: cot}
	for _, h := range ra {
		dong := make([]driver.Value, len(cot))
		for i, c := range cot {
			dong[i] = h.giaTri(c)
		}
		hg.hang = append(hg.hang, dong)
	}
	return hg, nil
}

func loc(in []hang, giu func(hang) bool) []hang {
	var ra []hang
	for _, h := range in {
		if giu(h) {
			ra = append(ra, h)
		}
	}
	return ra
}

func hop(c int, op string) bool {
	if op == ">" {
		return c > 0
	}
	return c < 0
}

func soSanh(a, b driver.Value) int {
	switch x := a.(type) {
	case time.Time:
		y, ok := b.(time.Time)
		if !ok {
			panic("driver giả: so sánh thời gian với một giá trị khác kiểu — con trỏ đã ràng buộc sai kiểu")
		}
		switch {
		case x.Before(y):
			return -1
		case x.After(y):
			return 1
		default:
			return 0
		}
	case string:
		y, _ := b.(string)
		return strings.Compare(x, y)
	case int64:
		y, _ := b.(int64)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		default:
			return 0
		}
	default:
		panic("driver giả: kiểu không so sánh được")
	}
}

type ketNoiTrang struct{ g *ghiChepTrang }

func (c ketNoiTrang) Connect(context.Context) (driver.Conn, error) { return &connTrang{g: c.g}, nil }
func (c ketNoiTrang) Driver() driver.Driver                        { return trinhTrang{} }

type trinhTrang struct{}

func (trinhTrang) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connTrang struct{ g *ghiChepTrang }

func (c *connTrang) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connTrang) Close() error { return nil }
func (c *connTrang) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *connTrang) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.g.them(lenh{sql: q, args: gt})
	return chay(q, gt)
}

type hangGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *hangGia) Columns() []string { return r.cot }
func (r *hangGia) Close() error      { return nil }
func (r *hangGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}
