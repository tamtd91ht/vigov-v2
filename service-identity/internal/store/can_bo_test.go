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
)

// THE DEFECT THIS FILE EXISTS TO CATCH: cotCanBo names the columns, motCanBo scans them BY
// POSITION, and since migration 0003 two of them — co_tai_khoan and dang_hoat_dong — are
// adjacent BOOLEANs. Swap them in either list and nothing fails: the driver scans a bool into a
// bool, both lists still have eleven entries, every existing test still passes, and the two
// values simply trade places for the rest of the system's life. What comes out is a
// directory-only person reported as an account and a locked account reported as open.
//
// A test using (true, true) or (false, false) cannot see a swap. Every assertion below therefore
// uses an ASYMMETRIC pair, and reads the column names out of the REAL statement rather than
// restating them — a copy of the list would agree with a swapped list just as happily.
//
// No PostgreSQL: the pg suites next door skip themselves without VIGOV_TEST_DSN, and a check
// nobody runs is a check that does not exist. The driver below returns each column by NAME, in
// the order the statement asked for them, which is exactly what PostgreSQL does and the only
// part of it this property depends on.

// hangMau is the row the fake driver serves, keyed by column name.
//
// co_tai_khoan = false WITH dang_hoat_dong = true is a combination the production query filters
// out — that is the point. It is the pair that makes a swap visible, and this test is about how
// a row is DECODED, not about which rows come back. The predicates are asserted separately, at
// the bottom of this file.
var hangMau = map[string]driver.Value{
	"id":                 "nd-01JINTERNALIDCUACANBO",
	"ma":                 "CB-2026-7K3M9Q",
	"ho_ten":             "Nguyễn Văn A",
	"email":              "canbo.a@example.gov.vn",
	"chuc_vu":            "Công chức Văn phòng",
	"bo_phan_id":         "bp-001",
	"vai_tro_id":         "vt-001",
	"dien_thoai_co_quan": "0900000000", // the agreed fake number (rule 3, invariant 5)
	"mat_khau_hash":      "$argon2id$gia$KHONG-PHAI-HASH-THAT",
	// phai_doi_mat_khau = true against co_tai_khoan = false: a THIRD asymmetric value, so a
	// positional slip involving this column cannot hide behind the other two. It also sits
	// between two TEXT columns in cotCanBo on purpose — see the note there.
	"phai_doi_mat_khau": true,
	"co_tai_khoan":      false,
	"dang_hoat_dong":    true,
}

const xaMau = "01J0000000000000000000000A"

// --- fake driver ------------------------------------------------------------------------------

type ghiCot struct {
	sql string
	gia map[string]driver.Value
}

type ketNoiCot struct{ g *ghiCot }

func (k ketNoiCot) Connect(context.Context) (driver.Conn, error) { return &connCot{g: k.g}, nil }
func (k ketNoiCot) Driver() driver.Driver                        { return trinhCot{} }

type trinhCot struct{}

func (trinhCot) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connCot struct{ g *ghiCot }

func (c *connCot) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connCot) Close() error              { return nil }
func (c *connCot) Begin() (driver.Tx, error) { return nil, errors.New("không cần giao dịch") }

func (c *connCot) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	c.g.sql = q
	ten, err := cotTrongCauLenh(q)
	if err != nil {
		return nil, err
	}
	hang := make([]driver.Value, len(ten))
	for i, t := range ten {
		v, co := c.g.gia[t]
		if !co {
			// A column was added to cotCanBo and not to hangMau. Failing here is deliberate:
			// scanning a missing column as nil would "pass" while proving nothing.
			return nil, fmt.Errorf("driver giả: không có giá trị mẫu cho cột %q", t)
		}
		hang[i] = v
	}
	return &rowsCot{cot: ten, hang: hang}, nil
}

type rowsCot struct {
	cot  []string
	hang []driver.Value
	xong bool
}

func (r *rowsCot) Columns() []string { return r.cot }
func (r *rowsCot) Close() error      { return nil }
func (r *rowsCot) Next(dest []driver.Value) error {
	if r.xong {
		return io.EOF
	}
	copy(dest, r.hang)
	r.xong = true
	return nil
}

// cotTrongCauLenh reads the SELECT list out of the statement the store actually built, so this
// test is pinned to cotCanBo itself rather than to a copy of it.
func cotTrongCauLenh(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := ketThucDanhSachCot(q)
	if i < 0 || j <= i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var ten []string
	for _, bieu := range tachDauPhay(q[i+len("SELECT ") : j]) {
		ten = append(ten, tenCot(bieu))
	}
	return ten, nil
}

// ketThucDanhSachCot finds the FROM that ends the SELECT list: the first one at parenthesis
// DEPTH ZERO.
//
// IT USED TO BE strings.Index(q, " FROM ") AND THAT WAS TWO TRAPS IN ONE LINE. A statement whose
// FROM starts a line — which every multi-line query in this package does — was not found at all,
// because the character before FROM is a newline rather than a space. And a statement carrying a
// derived table (`LEFT JOIN (SELECT … FROM …)`) or an `EXTRACT(EPOCH FROM x)` in its column list
// matched the INNER one, so the "column list" this returned stopped in the middle of an expression
// and the fake driver reported a missing sample value for a column nobody had written.
func ketThucDanhSachCot(q string) int {
	muc := 0
	for i := 0; i < len(q); i++ {
		switch q[i] {
		case '(':
			muc++
		case ')':
			muc--
		}
		if muc != 0 || i == 0 || !khoangTrang(q[i-1]) || !strings.HasPrefix(q[i:], "FROM") {
			continue
		}
		if i+4 < len(q) && khoangTrang(q[i+4]) {
			return i
		}
	}
	return -1
}

func khoangTrang(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}

// tachDauPhay splits on commas AT DEPTH ZERO. `coalesce(bo_phan_id,”)` carries a comma of its
// own, and a naive split turns one column into two — which would make this test report a
// column-count mismatch that is not there.
func tachDauPhay(s string) []string {
	var ra []string
	sau, muc := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			muc++
		case ')':
			muc--
		case ',':
			if muc == 0 {
				ra = append(ra, s[sau:i])
				sau = i + 1
			}
		}
	}
	return append(ra, s[sau:])
}

// tenCot reduces one SELECT-list expression to THE NAME POSTGRESQL WOULD GIVE THAT OUTPUT COLUMN.
// That is the whole contract: the fake driver builds rows by looking these names up, so it can
// only assert something real if it names columns the same way the server does.
//
// Three forms, in the order PostgreSQL resolves them:
//
//	`… AS ten`        an explicit alias wins over everything. Statements that compute a value —
//	                  `date_part('epoch', bat_dau)::int AS bat_dau` — MUST carry one, or the server
//	                  names the column `date_part`, twice, and nothing lines up with the Scan.
//	`nl.ngay`         a qualified column is named by its last segment, exactly as the server names
//	                  it. Without this, a joined statement would need sample rows keyed "nl.ngay",
//	                  which is a name no database ever returns.
//	`coalesce(x, '')` named after the column inside, which is what the pre-existing single-table
//	                  statements in this package rely on.
func tenCot(bieu string) string {
	if truong := strings.Fields(bieu); len(truong) >= 2 &&
		strings.EqualFold(truong[len(truong)-2], "AS") {
		return truong[len(truong)-1]
	}
	s := strings.Join(strings.Fields(bieu), "")
	if rest, co := strings.CutPrefix(s, "coalesce("); co {
		if i := strings.Index(rest, ","); i >= 0 {
			s = rest[:i]
		} else {
			s = strings.TrimSuffix(rest, ")")
		}
	}
	// The qualifier is dropped only on a bare column reference: an expression carrying a '(' is a
	// function call, and its dots belong to whatever is inside it.
	if !strings.Contains(s, "(") {
		if i := strings.LastIndex(s, "."); i >= 0 {
			return s[i+1:]
		}
	}
	return s
}

func moStoreGia(t *testing.T) (*CanBoStore, *ghiCot) {
	t.Helper()
	g := &ghiCot{gia: map[string]driver.Value{}}
	for k, v := range hangMau {
		g.gia[k] = v
	}
	db := sql.OpenDB(ketNoiCot{g: g})
	t.Cleanup(func() { db.Close() })
	return NewCanBoStore(pkgstore.New(db)), g
}

// --- the order ---------------------------------------------------------------------------------

func TestDocDungCapCoTaiKhoanVaDangHoatDong(t *testing.T) {
	// The two orders that must agree: the SELECT list (cotCanBo) and the Scan targets (motCanBo).
	// Each case feeds an asymmetric pair and states which way round it must come back.
	cases := map[string]struct {
		coTaiKhoan   bool
		dangHoatDong bool
	}{
		// A person in the public staff directory who was never given an account. Read as
		// (true, false) instead, they become an account that somebody merely locked.
		"chỉ có trong danh bạ": {coTaiKhoan: false, dangHoatDong: true},
		// A real account, currently locked. Read the other way round, a former employee reads as
		// somebody who simply never had an account — and the lock silently reads as open.
		"tài khoản đã bị khoá": {coTaiKhoan: true, dangHoatDong: false},
	}

	for ten, tc := range cases {
		t.Run(ten, func(t *testing.T) {
			s, g := moStoreGia(t)
			g.gia["co_tai_khoan"] = tc.coTaiKhoan
			g.gia["dang_hoat_dong"] = tc.dangHoatDong

			cb, err := s.TheoID(ctxXa(xaMau), "nd-01JINTERNALIDCUACANBO")
			if err != nil {
				t.Fatalf("đọc cán bộ lỗi: %v", err)
			}
			if cb.CoTaiKhoan != tc.coTaiKhoan || cb.DangHoatDong != tc.dangHoatDong {
				t.Fatalf("đọc ra (CoTaiKhoan=%v, DangHoatDong=%v), muốn (%v, %v) — "+
					"hai cột bool cạnh nhau bị hoán đổi giữa cotCanBo và motCanBo",
					cb.CoTaiKhoan, cb.DangHoatDong, tc.coTaiKhoan, tc.dangHoatDong)
			}
		})
	}
}

func TestMoiTruongCanBoVeDungChoCuaNo(t *testing.T) {
	// The wider version of the same defect: any two same-typed neighbours can trade places
	// silently. Every value here is distinct, so a misplacement anywhere in the list is named.
	s, _ := moStoreGia(t)

	cb, err := s.TheoEmail(ctxXa(xaMau), "canbo.a@example.gov.vn")
	if err != nil {
		t.Fatalf("đọc cán bộ lỗi: %v", err)
	}
	for ten, cap := range map[string][2]string{
		"ID":              {cb.ID, hangMau["id"].(string)},
		"Ma":              {cb.Ma, hangMau["ma"].(string)},
		"HoTen":           {cb.HoTen, hangMau["ho_ten"].(string)},
		"Email":           {cb.Email, hangMau["email"].(string)},
		"ChucVu":          {cb.ChucVu, hangMau["chuc_vu"].(string)},
		"BoPhanID":        {cb.BoPhanID, hangMau["bo_phan_id"].(string)},
		"VaiTroID":        {cb.VaiTroID, hangMau["vai_tro_id"].(string)},
		"DienThoaiCoQuan": {cb.DienThoaiCoQuan, hangMau["dien_thoai_co_quan"].(string)},
		"MatKhauHash":     {cb.MatKhauHash, hangMau["mat_khau_hash"].(string)},
	} {
		if cap[0] != cap[1] {
			t.Errorf("%s = %q, muốn %q — sai thứ tự cột", ten, cap[0], cap[1])
		}
	}

	// The bool among the strings, checked separately because the map above is typed for strings.
	// This is the assertion that turns red if `phai_doi_mat_khau` is moved next to the other two
	// booleans and then trades places with one of them.
	if cb.PhaiDoiMatKhau != hangMau["phai_doi_mat_khau"].(bool) {
		t.Errorf("PhaiDoiMatKhau = %v, muốn %v — sai thứ tự cột",
			cb.PhaiDoiMatKhau, hangMau["phai_doi_mat_khau"])
	}
}

func TestCoTaiKhoanDungNgaySatTruocDangHoatDong(t *testing.T) {
	// Structural, and the cheapest guard of the three: whatever else changes in cotCanBo, these
	// two stay adjacent and in this order, because motCanBo's last two Scan targets are in this
	// order. Stated here so a future edit that reorders the list is told why it turned red.
	s, g := moStoreGia(t)
	if _, err := s.TheoID(ctxXa(xaMau), "nd-01JINTERNALIDCUACANBO"); err != nil {
		t.Fatal(err)
	}
	ten, err := cotTrongCauLenh(g.sql)
	if err != nil {
		t.Fatal(err)
	}
	n := len(ten)
	if n < 2 || ten[n-2] != "co_tai_khoan" || ten[n-1] != "dang_hoat_dong" {
		t.Fatalf("hai cột cuối của cotCanBo là %v, muốn [co_tai_khoan dang_hoat_dong] — "+
			"motCanBo quét hai đích cuối theo đúng thứ tự này", ten[max(0, n-2):])
	}
}

// --- the predicates -----------------------------------------------------------------------------

func TestDuongDangNhapLocDuCaBaDieuKien(t *testing.T) {
	// Three conditions, three different failures if one goes missing:
	//   deleted_at IS NULL — a soft-deleted record, kept for the audit trail (rule 7), signs in.
	//   co_tai_khoan       — a public directory entry that was never given an account signs in,
	//                        and the response-timing side channel it opens hands out the
	//                        commune's staff directory (see the note on TheoEmail).
	//   dang_hoat_dong     — a locked-out former employee signs in.
	// They are ADDITIVE. Neither of the last two replaces the other: locked and never-granted are
	// different states, and a row can be in one without being in the other.
	goi := map[string]func(*CanBoStore) error{
		"TheoEmail": func(s *CanBoStore) error {
			_, err := s.TheoEmail(ctxXa(xaMau), "canbo.a@example.gov.vn")
			return err
		},
		"TheoID": func(s *CanBoStore) error {
			_, err := s.TheoID(ctxXa(xaMau), "nd-01JINTERNALIDCUACANBO")
			return err
		},
	}
	for ten, chay := range goi {
		t.Run(ten, func(t *testing.T) {
			s, g := moStoreGia(t)
			if err := chay(s); err != nil {
				t.Fatal(err)
			}
			i := strings.Index(g.sql, " WHERE ")
			if i < 0 {
				t.Fatalf("câu lệnh không có WHERE: %q", g.sql)
			}
			// Only the predicate is inspected: the SELECT list names both columns whatever the
			// filter does, so searching the whole statement would report a filter that had been
			// deleted.
			dk := g.sql[i:]
			for _, can := range []string{"tenant_id = $1", "deleted_at IS NULL",
				"co_tai_khoan", "dang_hoat_dong"} {
				if !strings.Contains(dk, can) {
					t.Errorf("thiếu %q trong điều kiện: %q", can, dk)
				}
			}
		})
	}
}
