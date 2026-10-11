package store

import (
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/textfold"
)

// Integration tests for migration 0041 (accent-insensitive register search, bug sheet 10/10 row 8) and
// for `scope=related` on the petition register (row 57), against a REAL PostgreSQL.
//
// ⚠ THEY SKIP UNLESS VIGOV_TEST_DSN IS SET, AND THE PACKAGE STILL PRINTS `ok`. No PostgreSQL was
// reachable when these were written (DSN unset, Docker daemon down). The fake-driver suites prove the
// statement TEXT; only these prove PostgreSQL computes the generated columns, accepts the U&'' literal,
// and that the SQL fold equals the Go fold.

// TestPgSQLFoldEqualsGoFold — the database's fold of each sample equals textfold.Fold, decomposed input
// included. If it does not, the stored column and any Go-side comparison disagree.
func TestPgSQLFoldEqualsGoFold(t *testing.T) {
	db := moKetNoi(t)
	for _, s := range []string{
		"Nhà văn hoá", "NHÀ VĂN HOÁ", "Đường Đỗ Quang Đẩu", "Thuỷ lợi Ứng Hoà", "Ðường",
		"Nhà văn hoá", "Nguyễn Thị Ánh Tuyết", "NV19 50% (đã xử lý)",
	} {
		var got string
		if err := db.QueryRow(`SELECT `+textfold.SQL("$1::text"), s).Scan(&got); err != nil {
			t.Fatalf("gập %q: %v", s, err)
		}
		if got != textfold.Fold(s) {
			t.Errorf("SQL gập %q thành %q, Go thành %q", s, got, textfold.Fold(s))
		}
	}
}

func taskCodesFor(t *testing.T, s *NhiemVuStore, commune, term string) string {
	t.Helper()
	yc, err := page.Parse(url.Values{}, SapXepNhiemVu)
	if err != nil {
		t.Fatal(err)
	}
	kq, err := s.DanhSach(ctxXa(tenant.ID(commune)), LocNhiemVu{Tim: term}, yc)
	if err != nil {
		t.Fatalf("DanhSach(%q): %v", term, err)
	}
	var codes []string
	for _, n := range kq.Items {
		codes = append(codes, n.Ma)
	}
	sort.Strings(codes)
	return strings.Join(codes, ",")
}

// TestPgTaskSearchIsAccentInsensitive — "nha van hoa" finds "Nhà văn hoá"; so do the accented and
// upper-case spellings, and the register number still matches. Another commune's row never does.
func TestPgTaskSearchIsAccentInsensitive(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, other)
	insert := func(xa, id, code, title string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO nhiem_vu
			 (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao, tien_do, nguoi_tao_ma)
			 VALUES ($1,$2,$3,'theo-van-ban',$4,'moi-giao','truc-tiep',0,'CB-00123')`,
			xa, xa[:10]+id, code, title); err != nil {
			t.Fatalf("thêm %s: %v", code, err)
		}
	}
	insert(commune, "t1", "NV01", "Sửa chữa Nhà văn hoá thôn 3")
	insert(commune, "t2", "NV02", "Rà soát hộ nghèo")
	insert(other, "t1", "NV01", "Nhà văn hoá xã bên")

	s := NewNhiemVuStore(pkgstore.New(db))
	for term, want := range map[string]string{
		"nha van hoa": "NV01", "Nhà văn hoá": "NV01", "NHÀ VĂN HOÁ": "NV01", "nhà van hoa": "NV01",
		"ra soat": "NV02", "nv02": "NV02", "đường": "",
	} {
		if got := taskCodesFor(t, s, commune, term); got != want {
			t.Errorf("tìm %q = %q, muốn %q", term, got, want)
		}
	}
}

// TestPgCitizenReportRelatedScope — `scope=related` returns exactly: assigned to me · a timeline entry
// of mine · held by one of my units. Another officer's petition, and the same code's rows in another
// commune, are not returned.
func TestPgCitizenReportRelatedScope(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	const me = "CB-00123"
	at := time.Date(2026, 9, 9, 7, 20, 0, 0, time.UTC)
	add := func(xa, id, code, assignee, unit string) {
		t.Helper()
		if err := themPhieu(db, xa, xa[:10]+id, code, "can-bo-nhap-ho", "dien", "da-tiep-nhan",
			at, at, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", code, err)
		}
		if _, err := db.Exec(`UPDATE phieu_phan_anh SET can_bo_xu_ly_id = $3, bo_phan_id = $4
			WHERE tenant_id = $1 AND id = $2`, xa, xa[:10]+id, nullHoac(assignee), nullHoac(unit)); err != nil {
			t.Fatalf("giao %s: %v", code, err)
		}
	}
	add(commune, "p1", "PA-REL1-0001-AAAA", me, "")             // assigned to me
	add(commune, "p2", "PA-REL2-0002-AAAA", "CB-00999", "")     // I wrote on its timeline
	add(commune, "p3", "PA-REL3-0003-AAAA", "", "bp-vpdu")      // held by my unit
	add(commune, "p4", "PA-REL4-0004-AAAA", "CB-00999", "bp-x") // not mine
	add(other, "p1", "PA-REL5-0005-AAAA", me, "bp-vpdu")        // another commune
	if _, err := db.Exec(`INSERT INTO nhat_ky_phan_anh (tenant_id, id, phieu_phan_anh_id, thoi_diem, nguoi_ma,
		hanh_vi, trang_thai_tai_thoi_diem, noi_dung) VALUES ($1,$2,$3,$4,$5,'ghi-chu','da-tiep-nhan','Đã gọi.')`,
		commune, commune[:10]+"nk1", commune[:10]+"p2", at, me); err != nil {
		t.Fatalf("thêm nhật ký: %v", err)
	}

	s := NewPhieuPhanAnhStore(pkgstore.New(db))
	yc, err := page.Parse(url.Values{}, SapXepPhieu)
	if err != nil {
		t.Fatal(err)
	}
	kq, err := s.DanhSach(ctxXa(tenant.ID(commune)), LocPhieu{ChoPhepHanChe: true,
		Related: &CitizenReportRelatedScope{StaffCode: me, OrgUnits: []string{"bp-vpdu"}}}, yc)
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	var codes []string
	for _, p := range kq.Items {
		codes = append(codes, p.MaTraCuu)
	}
	sort.Strings(codes)
	if got := strings.Join(codes, ","); got != "PA-REL1-0001-AAAA,PA-REL2-0002-AAAA,PA-REL3-0003-AAAA" {
		t.Errorf("liên quan đến tôi = %s", got)
	}
}

// TestPgPetitionSearchIsAccentInsensitive — the text and the location fold; a NULL location matches
// nothing on its own.
func TestPgPetitionSearchIsAccentInsensitive(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	at := time.Date(2026, 9, 9, 7, 20, 0, 0, time.UTC)
	if err := themPhieu(db, commune, commune[:10]+"p1", "PA-FOLD-0001-AAAA", "can-bo-nhap-ho", "dien",
		"da-tiep-nhan", at, at, nil, nil); err != nil {
		t.Fatalf("thêm phiếu: %v", err)
	}
	if _, err := db.Exec(`UPDATE phieu_phan_anh SET noi_dung = $3, dia_chi = $4 WHERE tenant_id = $1 AND id = $2`,
		commune, commune[:10]+"p1", "Đèn đường trước Nhà văn hoá bị hỏng", "Đầu ngõ thôn Hà Lam"); err != nil {
		t.Fatalf("đặt nội dung: %v", err)
	}
	s := NewPhieuPhanAnhStore(pkgstore.New(db))
	for term, want := range map[string]int{"nha van hoa": 1, "DEN DUONG": 1, "ha lam": 1, "đầu ngõ": 1, "chợ": 0} {
		n, err := s.CountCitizenReports(ctxXa(tenant.ID(commune)), LocPhieu{Tim: term})
		if err != nil {
			t.Fatalf("đếm %q: %v", term, err)
		}
		if n != want {
			t.Errorf("tìm %q = %d phiếu, muốn %d", term, n, want)
		}
	}
}
