package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// THE DEFECT CLASS THIS FILE CLOSES: a staff register that skips or repeats a person between two
// pages, or shows one commune a person belonging to another.
//
// Neither failure looks like a failure. The screen renders, the counts look plausible, nothing
// errors — and a member of staff is simply not in the register, or is in the wrong one. On the
// screen that decides who holds `admin.user` in a commune, "this person is not listed" and "this
// person does not exist" are indistinguishable to whoever is looking.
//
// WHY THE FAKE DRIVER EXECUTES THE KEYSET INSTEAD OF RECORDING THE STATEMENT: asserting on the
// text would prove the string was built, not that walking it visits every person exactly once.
// This driver implements the small part of PostgreSQL the walk depends on — the row-value
// comparison, the ORDER BY including its tie-break, LIMIT, and the soft-delete predicate — over
// an in-memory table. Drop the `, id` from the ORDER BY built by pkg/store and the dataset below
// really does lose a row.
//
// No PostgreSQL: the pg suites next door skip themselves without VIGOV_TEST_DSN, and a check
// nobody runs is a check that does not exist.

var dsMoc = time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)

const dsXaB = "01J0000000000000000000000B"

// dsDuLieu is one `nguoi_dung` table across two communes.
//
// FOUR PROPERTIES ARE BUILT INTO IT, each one the only way to see a specific defect:
//
//	CB-002 and CB-003 share tao_luc      two rows of equal sort key, straddling a page boundary
//	                                     at limit=2 — the only arrangement that tells a total
//	                                     order from a partial one.
//	CB-004 has deleted_at set            a soft-deleted person, kept for the audit trail (rule 7)
//	                                     and shown on no screen.
//	CB-002 has co_tai_khoan = false      a directory-only person, with dang_hoat_dong = true: an
//	                                     ASYMMETRIC pair, which a swap of the two adjacent bools
//	                                     cannot survive.
//	commune B's rows sort LAST           by both code and time, so a walk that ever dropped the
//	                                     commune from the statement would return them at the end
//	                                     instead of stopping.
var dsDuLieu = []hangND{
	{xa: xaMau, id: "nd-01", ma: "CB-001", hoTen: "Nguyễn Văn A", email: "a@example.gov.vn",
		chucVu: "Chủ tịch UBND xã", boPhan: "bp-001", vaiTro: "vt-001", dienThoai: "0900000001",
		coTaiKhoan: true, dangHoatDong: true, dangNhap: &dsMoc, taoLuc: dsMoc.Add(1 * time.Minute)},
	{xa: xaMau, id: "nd-02", ma: "CB-002", hoTen: "Trần Thị B", email: "b@example.gov.vn",
		chucVu: "Trưởng thôn", boPhan: "bp-002", dienThoai: "0900000002",
		coTaiKhoan: false, dangHoatDong: true, taoLuc: dsMoc.Add(2 * time.Minute)},
	{xa: xaMau, id: "nd-03", ma: "CB-003", hoTen: "Lê Văn C", email: "c@example.gov.vn",
		chucVu: "Kế toán", boPhan: "bp-001", vaiTro: "vt-002", dienThoai: "0900000003",
		coTaiKhoan: true, dangHoatDong: false, taoLuc: dsMoc.Add(2 * time.Minute)},
	{xa: xaMau, id: "nd-04", ma: "CB-004", hoTen: "Phạm Thị D", email: "d@example.gov.vn",
		dienThoai: "0900000004", coTaiKhoan: true, dangHoatDong: true,
		taoLuc: dsMoc.Add(3 * time.Minute), daXoa: true},
	{xa: xaMau, id: "nd-05", ma: "CB-005", hoTen: "Đỗ Văn E", email: "e@example.gov.vn",
		dienThoai: "0900000005", coTaiKhoan: true, dangHoatDong: true,
		taoLuc: dsMoc.Add(4 * time.Minute)},

	{xa: dsXaB, id: "ndb-01", ma: "CB-900", hoTen: "Vũ Thị F", email: "f@example.gov.vn",
		dienThoai: "0900000009", coTaiKhoan: true, dangHoatDong: true,
		taoLuc: dsMoc.Add(30 * time.Minute)},
	{xa: dsXaB, id: "ndb-02", ma: "CB-901", hoTen: "Bùi Văn G", email: "g@example.gov.vn",
		dienThoai: "0900000010", coTaiKhoan: true, dangHoatDong: true,
		taoLuc: dsMoc.Add(31 * time.Minute)},
}

// idCuaXaMau is every visible row of the sample commune, in `ma` order — which for this dataset
// is also `tao_luc` order. The soft-deleted CB-004 is absent, and that absence is asserted.
var idCuaXaMau = []string{"nd-01", "nd-02", "nd-03", "nd-05"}

// --- harness ------------------------------------------------------------------------------

type banThuDS struct {
	kho *CanBoStore
	ghi *ghiLenhDS
}

func moBanThuDS(t *testing.T) *banThuDS {
	t.Helper()
	g := &ghiLenhDS{}
	db := sql.OpenDB(ketNoiDS{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return &banThuDS{kho: NewCanBoStore(pkgstore.New(db)), ghi: g}
}

func (b *banThuDS) trang(t *testing.T, xa, truyVan string) (page.Result[canBoRa], error) {
	t.Helper()
	q, err := url.ParseQuery(truyVan)
	if err != nil {
		t.Fatalf("chuỗi truy vấn hỏng trong chính bài kiểm: %v", err)
	}
	yc, err := page.Parse(q, SapXepCanBo)
	if err != nil {
		// The handler's own shape: a rejected request runs no statement at all.
		return page.Result[canBoRa]{}, err
	}
	kq, err := b.kho.DanhSach(ctxXa(xa), yc)
	return doiKieu(kq), err
}

// canBoRa / doiKieu exist only so the walk helper below can talk about ids without repeating the
// domain type's name on every line.
type canBoRa struct {
	ID, Ma       string
	CoTaiKhoan   bool
	DangHoatDong bool
	DangNhap     *time.Time
}

func doiKieu(kq page.Result[domain.CanBoTomTat]) page.Result[canBoRa] {
	ra := page.Result[canBoRa]{NextCursor: kq.NextCursor, HasMore: kq.HasMore, Items: []canBoRa{}}
	for _, cb := range kq.Items {
		ra.Items = append(ra.Items, canBoRa{
			ID: cb.ID, Ma: cb.Ma, CoTaiKhoan: cb.CoTaiKhoan,
			DangHoatDong: cb.DangHoatDong, DangNhap: cb.DangNhapGanNhat,
		})
	}
	return ra
}

// duyetHet walks every page and returns the ids in the order they were handed out.
//
// It stops hard at 50 iterations: a cursor that does not advance is an infinite loop in
// production, and a hanging test is how that is discovered here instead.
func (b *banThuDS) duyetHet(t *testing.T, xa, truyVan string) []string {
	t.Helper()
	var ids []string
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
		for _, cb := range kq.Items {
			ids = append(ids, cb.ID)
		}
		if !kq.HasMore {
			if kq.NextCursor != "" {
				t.Error("has_more=false mà vẫn có next_cursor")
			}
			return ids
		}
		if kq.NextCursor == "" {
			t.Fatal("has_more=true mà không có next_cursor — client không đi tiếp được")
		}
		conTro = kq.NextCursor
	}
}

// --- the walk ---------------------------------------------------------------------------------

func TestDanhSachDiHetCacTrangKhongLapKhongSot(t *testing.T) {
	// THE MOST IMPORTANT CASE IN THIS FILE.
	//
	// limit=2 puts CB-002 and CB-003 — two rows with the SAME tao_luc — on either side of a page
	// boundary. Without `, id` in the ORDER BY, PostgreSQL may return them in either order between
	// the two queries, and one of the two is then either handed out twice or never at all.
	//
	// MUTATION THAT MUST TURN THIS RED: in mocCanBo, return page.TextKey(cb.Ma) for the tao_luc
	// column as well. The `code` sort keeps working and only the `created_at` walk breaks.
	b := moBanThuDS(t)

	for _, truyVan := range []string{
		"limit=2",
		"limit=2&sort=code",
		"limit=2&sort=created_at",
		"limit=1&sort=created_at",
		"limit=100",
	} {
		t.Run(truyVan, func(t *testing.T) {
			got := b.duyetHet(t, xaMau, truyVan)
			if len(got) != len(idCuaXaMau) {
				t.Fatalf("đi hết được %v (%d bản ghi), muốn %d — lặp hoặc sót",
					got, len(got), len(idCuaXaMau))
			}
			thay := map[string]int{}
			for _, id := range got {
				thay[id]++
			}
			for _, id := range idCuaXaMau {
				if thay[id] != 1 {
					t.Errorf("%s xuất hiện %d lần trong %v", id, thay[id], got)
				}
			}
		})
	}
}

func TestDanhSachSapXepGiamCungDiHetDuoc(t *testing.T) {
	b := moBanThuDS(t)

	got := b.duyetHet(t, xaMau, "limit=2&sort=code&order=desc")
	muon := []string{"nd-05", "nd-03", "nd-02", "nd-01"}
	if strings.Join(got, ",") != strings.Join(muon, ",") {
		t.Fatalf("giảm dần cho %v, muốn %v", got, muon)
	}
}

// --- rule 7: a soft-deleted person is on no screen ----------------------------------------------

func TestDanhSachKhongTraDongDaXoaMem(t *testing.T) {
	// MUTATION THAT MUST TURN THIS RED: drop `AND deleted_at IS NULL` from locTomTat. A record
	// kept for the audit trail would come back onto the staff register — and onto the counts a
	// commune reports upward.
	b := moBanThuDS(t)

	for _, id := range b.duyetHet(t, xaMau, "limit=100") {
		if id == "nd-04" {
			t.Fatal("bản ghi đã xoá mềm vẫn nằm trong danh sách")
		}
	}
	if _, err := b.kho.ChiTiet(ctxXa(xaMau), "nd-04"); !errors.Is(err, ErrCanBoKhongTonTai) {
		t.Errorf("ChiTiet của bản ghi đã xoá mềm trả %v, muốn ErrCanBoKhongTonTai", err)
	}
}

// --- rule 1: one commune ------------------------------------------------------------------------

func TestDanhSachChiThayDuLieuCuaXaDangHoi(t *testing.T) {
	b := moBanThuDS(t)

	for _, id := range b.duyetHet(t, xaMau, "limit=2") {
		if strings.HasPrefix(id, "ndb-") {
			t.Fatalf("xã mẫu nhận được bản ghi %s của xã khác — RÒ RỈ GIỮA HAI XÃ", id)
		}
	}

	l := b.ghi.chua("FROM nguoi_dung")
	if l == nil {
		t.Fatal("không câu lệnh nào chạm tới driver")
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Fatalf("câu lệnh thiếu ràng buộc xã: %q", l.sql)
	}
	if l.args[0] != xaMau {
		t.Errorf("$1 = %v, muốn mã xã %q — $1 phải LUÔN là xã", l.args[0], xaMau)
	}
}

func TestConTroCuaMotXaKhongKeoDuocDuLieuXaKhac(t *testing.T) {
	// A cursor is not a capability. Replayed against another commune's context it can only move
	// the anchor WITHIN that commune: the scope comes from the context and is bound to $1, and the
	// cursor carries no commune at all (rule 1, forbidden #2).
	b := moBanThuDS(t)

	kqA, err := b.trang(t, xaMau, "limit=2")
	if err != nil {
		t.Fatal(err)
	}
	if kqA.NextCursor == "" {
		t.Fatal("không có con trỏ để thử")
	}

	kqB, err := b.trang(t, dsXaB, "limit=2&cursor="+url.QueryEscape(kqA.NextCursor))
	if err != nil {
		t.Fatalf("con trỏ của xã khác phải chạy trong phạm vi xã này, không phải lỗi: %v", err)
	}
	for _, cb := range kqB.Items {
		if !strings.HasPrefix(cb.ID, "ndb-") {
			t.Fatalf("xã B nhận được bản ghi %s của xã mẫu — RÒ RỈ GIỮA HAI XÃ", cb.ID)
		}
	}
}

func TestChiTietIDCuaXaKhacKhongTimThay(t *testing.T) {
	// The route answers 404 for this, and 404 is only safe because the store genuinely cannot see
	// the row: there is no signature here that takes a commune, so no caller can widen it.
	b := moBanThuDS(t)

	if _, err := b.kho.ChiTiet(ctxXa(xaMau), "ndb-01"); !errors.Is(err, ErrCanBoKhongTonTai) {
		t.Fatalf("đọc được bản ghi %q của xã khác: %v", "ndb-01", err)
	}
	if _, err := b.kho.ChiTiet(ctxXa(dsXaB), "ndb-01"); err != nil {
		t.Fatalf("xã sở hữu bản ghi lại không đọc được nó: %v", err)
	}
}

func TestKhongCoXaThiPanicChuKhongDocMoXa(t *testing.T) {
	b := moBanThuDS(t)
	yc, err := page.New(SapXepCanBo, "", "", "", "")
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
	_, _ = b.kho.DanhSach(context.Background(), yc)
}

// --- what the statement selects -----------------------------------------------------------------

func TestDanhSachKhongBaoGioChonMatKhauHash(t *testing.T) {
	// The first of the three layers that keep a credential inside this service. It is asserted on
	// the REAL statement, not on a copy of the column list: a copy would agree with a changed list
	// just as happily.
	b := moBanThuDS(t)
	if _, err := b.trang(t, xaMau, "limit=2"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.kho.ChiTiet(ctxXa(xaMau), "nd-01"); err != nil {
		t.Fatal(err)
	}

	for _, l := range b.ghi.tatCa() {
		if strings.Contains(l.sql, "mat_khau") {
			t.Errorf("câu lệnh chọn cả cột mật khẩu: %q", l.sql)
		}
	}
}

func TestCoTaiKhoanVaDangHoatDongVeDungChoCuaNo(t *testing.T) {
	// The same trap can_bo_test.go guards on the sign-in path, on the register path: two adjacent
	// BOOLEANs, swapped, produce no error — a directory-only person is reported as an account and
	// a locked account reads as open. The pair below is ASYMMETRIC, which is the only kind that
	// can tell a swap from a correct read.
	b := moBanThuDS(t)

	kq, err := b.trang(t, xaMau, "limit=100")
	if err != nil {
		t.Fatal(err)
	}
	muon := map[string][2]bool{
		"nd-01": {true, true},  // an ordinary account
		"nd-02": {false, true}, // directory only — never had an account to lock
		"nd-03": {true, false}, // a real account, locked
	}
	thay := map[string]bool{}
	for _, cb := range kq.Items {
		c, co := muon[cb.ID]
		if !co {
			continue
		}
		thay[cb.ID] = true
		if cb.CoTaiKhoan != c[0] || cb.DangHoatDong != c[1] {
			t.Errorf("%s đọc ra (CoTaiKhoan=%v, DangHoatDong=%v), muốn (%v, %v) — "+
				"hai cột bool cạnh nhau bị hoán đổi giữa cotTomTat và quetTomTat",
				cb.ID, cb.CoTaiKhoan, cb.DangHoatDong, c[0], c[1])
		}
	}
	// ALL THREE must have been seen. Without this the case passes on a register that quietly
	// filtered one of them out — and the row most likely to be filtered is nd-02, the
	// directory-only person, who is precisely the kind of record `co_tai_khoan` exists to keep.
	for id := range muon {
		if !thay[id] {
			t.Errorf("%s không có trong danh sách — danh sách đang lọc bớt một loại bản ghi", id)
		}
	}
}

func TestChuaDangNhapBaoGioThiDangNhapGanNhatLaNil(t *testing.T) {
	// dang_nhap_gan_nhat is NULLABLE. Scanned into a plain time.Time, every person who has never
	// signed in would be a scan error — and the register would stop listing exactly the accounts
	// an administrator opened it to find.
	b := moBanThuDS(t)

	kq, err := b.trang(t, xaMau, "limit=100")
	if err != nil {
		t.Fatal(err)
	}
	var thayNil, thayCo int
	for _, cb := range kq.Items {
		if cb.DangNhap == nil {
			thayNil++
		} else {
			thayCo++
		}
	}
	if thayNil == 0 || thayCo == 0 {
		t.Fatalf("mẫu không có đủ hai trường hợp: nil=%d, có=%d", thayNil, thayCo)
	}
}

// --- the anchor -----------------------------------------------------------------------------------

func TestMocCanBoKhopKieuVoiCotSapXep(t *testing.T) {
	// store.QueryPage compares the anchor's KIND against the column's, so a text anchor on a time
	// column is caught. A same-kind mismatch is NOT caught, and on a list short enough to fit one
	// page nothing is checked at all — which is why the default branch below is an error rather
	// than a fallback.
	cb := domain.CanBoTomTat{Ma: "CB-001", TaoLuc: dsMoc}

	for _, c := range []struct {
		cot  page.Column
		kieu page.Kind
	}{
		{page.Col("code", "ma", page.KindText), page.KindText},
		{page.Col("created_at", "tao_luc", page.KindTime), page.KindTime},
	} {
		k, err := mocCanBo(c.cot, cb)
		if err != nil {
			t.Fatalf("cột %q: %v", c.cot.SQL, err)
		}
		if k.Kind() != c.kieu {
			t.Errorf("cột %q cho mốc kiểu %q, muốn %q", c.cot.SQL, k.Kind(), c.kieu)
		}
	}

	// A sort added to SapXepCanBo and forgotten here must fail loudly on the first request, not
	// quietly reorder a register.
	if _, err := mocCanBo(page.Col("email", "email", page.KindText), cb); err == nil {
		t.Error("cột lạ vẫn được cấp mốc — SapXepCanBo và mocCanBo có thể lệch nhau mà không ai biết")
	}
}

// --- fake driver: a small, honest keyset engine ---------------------------------------------------

type hangND struct {
	xa, id, ma, hoTen, email, chucVu, boPhan, vaiTro, dienThoai string
	coTaiKhoan, dangHoatDong                                    bool
	dangNhap                                                    *time.Time
	taoLuc                                                      time.Time
	daXoa                                                       bool
}

func (h hangND) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return h.id
	case "ma":
		return h.ma
	case "ho_ten":
		return h.hoTen
	case "email":
		return h.email
	case "chuc_vu":
		return h.chucVu
	case "bo_phan_id":
		return h.boPhan
	case "vai_tro_id":
		return h.vaiTro
	case "dien_thoai":
		return h.dienThoai
	case "co_tai_khoan":
		return h.coTaiKhoan
	case "dang_hoat_dong":
		return h.dangHoatDong
	case "dang_nhap_gan_nhat":
		if h.dangNhap == nil {
			return nil // SQL NULL — "Chưa đăng nhập"
		}
		return *h.dangNhap
	case "tao_luc":
		return h.taoLuc
	case "tenant_id":
		return h.xa
	default:
		// A column was added to cotTomTat and not here. Failing loudly beats scanning a nil that
		// "passes" while proving nothing.
		panic("driver giả: không có giá trị mẫu cho cột " + cot)
	}
}

type lenhDS struct {
	sql  string
	args []driver.Value
}

type ghiLenhDS struct{ lenh []lenhDS }

func (g *ghiLenhDS) tatCa() []lenhDS { return append([]lenhDS(nil), g.lenh...) }

func (g *ghiLenhDS) chua(manh string) *lenhDS {
	for i := range g.lenh {
		if strings.Contains(g.lenh[i].sql, manh) {
			return &g.lenh[i]
		}
	}
	return nil
}

var (
	dsReBang    = regexp.MustCompile(`AND id = \$(\d+)`)
	dsReCap     = regexp.MustCompile(`AND \(([a-z_]+), ([a-z_]+)\) ([<>]) \(\$(\d+), \$(\d+)\)`)
	dsReThuTu   = regexp.MustCompile(`ORDER BY ([a-z_]+) (ASC|DESC)(?:, ([a-z_]+) (ASC|DESC))?`)
	dsReGioiHan = regexp.MustCompile(`LIMIT \$(\d+)`)
)

// dsChay is the whole fake engine. It READS the statement rather than being told what to do,
// which is what makes a mutation in the store show up here as WRONG DATA instead of as a fake
// somebody updated to match.
func dsChay(q string, args []driver.Value) (driver.Rows, error) {
	cot, err := cotTrongCauLenh(q)
	if err != nil {
		return nil, err
	}
	lay := func(n int) driver.Value { return args[n-1] } // $n

	var ra []hangND
	for _, h := range dsDuLieu {
		if h.xa == args[0] {
			ra = append(ra, h)
		}
	}

	// The soft-delete predicate is MODELLED, not assumed: remove it from the store and these rows
	// come back, which is the failure a test can name.
	if strings.Contains(q, "deleted_at IS NULL") {
		ra = dsLoc(ra, func(h hangND) bool { return !h.daXoa })
	}

	if m := dsReBang.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[1])
		ra = dsLoc(ra, func(h hangND) bool { return h.id == lay(n) })
	}

	if m := dsReCap.FindStringSubmatch(q); m != nil {
		khoa, phaHoa, op := m[1], m[2], m[3]
		mkA, _ := strconv.Atoi(m[4])
		mkB, _ := strconv.Atoi(m[5])
		ra = dsLoc(ra, func(h hangND) bool {
			c := dsSoSanh(h.giaTri(khoa), lay(mkA))
			if c == 0 {
				c = dsSoSanh(h.giaTri(phaHoa), lay(mkB))
			}
			if op == ">" {
				return c > 0
			}
			return c < 0
		})
	}

	if m := dsReThuTu.FindStringSubmatch(q); m != nil {
		k1, c1, k2, c2 := m[1], m[2], m[3], m[4]
		if k2 == "" {
			// NO TIE-BREAK DECLARED. PostgreSQL is then free to return rows of equal sort key in
			// any order, and it does not promise the same order twice. The fake models the
			// UNHELPFUL choice on purpose — ties come back opposite to the declared direction —
			// because the helpful one would let an untied ORDER BY pass here and fail in
			// production, which is precisely the defect being hunted.
			k2, c2 = "id", "ASC"
			if c1 == "ASC" {
				c2 = "DESC"
			}
		}
		sort.SliceStable(ra, func(i, j int) bool {
			c := dsSoSanh(ra[i].giaTri(k1), ra[j].giaTri(k1))
			if c == 0 {
				c = dsSoSanh(ra[i].giaTri(k2), ra[j].giaTri(k2))
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

	if m := dsReGioiHan.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[1])
		gh, ok := lay(n).(int64)
		if !ok {
			return nil, errors.New("driver giả: LIMIT không phải số nguyên")
		}
		if int64(len(ra)) > gh {
			ra = ra[:gh]
		}
	}

	// EVERY conjunct must be one the engine actually models. A fake that silently ignores a
	// predicate it has never seen is a fake that reports the same rows whatever the store filters
	// on — so adding `AND co_tai_khoan` to the register, which would hide every directory-only
	// person from the screen that exists to list them, would leave this whole file green.
	if err := dsMenhDeLa(q); err != nil {
		return nil, err
	}

	hg := &rowsDS{cot: cot}
	for _, h := range ra {
		dong := make([]driver.Value, len(cot))
		for i, c := range cot {
			dong[i] = h.giaTri(c)
		}
		hg.hang = append(hg.hang, dong)
	}
	return hg, nil
}

// dsMenhDeLa refuses a WHERE clause carrying a conjunct this engine does not implement.
//
// It strips the four it does implement and requires that nothing but `AND`s and whitespace is
// left. Listing what IS understood, rather than guessing at what is not, is what makes the check
// hold for a predicate nobody has thought of yet.
func dsMenhDeLa(q string) error {
	_, menh, co := strings.Cut(q, " WHERE ")
	if !co {
		return errors.New("driver giả: câu lệnh không có WHERE — mọi truy vấn phải ràng buộc xã")
	}
	if truoc, _, co := strings.Cut(menh, " ORDER BY "); co {
		menh = truoc
	}
	for _, hieu := range []*regexp.Regexp{
		regexp.MustCompile(`tenant_id = \$1`),
		regexp.MustCompile(`deleted_at IS NULL`),
		dsReBang,
		dsReCap,
	} {
		menh = hieu.ReplaceAllString(menh, "")
	}
	con := strings.TrimSpace(strings.ReplaceAll(menh, "AND", ""))
	if con != "" {
		return fmt.Errorf("driver giả: mệnh đề %q chưa được mô phỏng — "+
			"bổ sung engine thay vì bỏ qua, nếu không bài kiểm sẽ xanh trước một bộ lọc đã đổi", con)
	}
	return nil
}

func dsLoc(in []hangND, giu func(hangND) bool) []hangND {
	var ra []hangND
	for _, h := range in {
		if giu(h) {
			ra = append(ra, h)
		}
	}
	return ra
}

func dsSoSanh(a, b driver.Value) int {
	switch x := a.(type) {
	case time.Time:
		y, ok := b.(time.Time)
		if !ok {
			panic(fmt.Sprintf("driver giả: so thời gian với %T — con trỏ ràng buộc sai kiểu", b))
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
		y, ok := b.(string)
		if !ok {
			panic(fmt.Sprintf("driver giả: so chuỗi với %T — con trỏ ràng buộc sai kiểu", b))
		}
		return strings.Compare(x, y)
	default:
		panic(fmt.Sprintf("driver giả: kiểu %T không so sánh được", a))
	}
}

type ketNoiDS struct{ g *ghiLenhDS }

func (k ketNoiDS) Connect(context.Context) (driver.Conn, error) { return &connDS{g: k.g}, nil }
func (k ketNoiDS) Driver() driver.Driver                        { return trinhDS{} }

type trinhDS struct{}

func (trinhDS) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connDS struct{ g *ghiLenhDS }

func (c *connDS) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connDS) Close() error { return nil }
func (c *connDS) Begin() (driver.Tx, error) {
	return nil, errors.New("driver giả: không có giao dịch")
}

func (c *connDS) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.g.lenh = append(c.g.lenh, lenhDS{sql: q, args: gt})
	return dsChay(q, gt)
}

type rowsDS struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsDS) Columns() []string { return r.cot }
func (r *rowsDS) Close() error      { return nil }
func (r *rowsDS) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}
