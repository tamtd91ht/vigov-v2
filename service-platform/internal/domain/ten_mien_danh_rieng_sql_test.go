package domain

import (
	"os"
	"regexp"
	"testing"
)

// tepMigrationDanhRieng is the migration that carries the SQL half of the rule. Read from disk by a
// relative path rather than through the embedded migrations package: domain/ imports nothing but
// the standard library, tests included, and the file on disk IS what gets embedded.
const tepMigrationDanhRieng = "../../migrations/0007_tenant_domain_khong_danh_rieng.sql"

// khoiRangBuoc captures the body of the CHECK, and mauKhongKhop every `host !~* '<pattern>'` in it.
var (
	khoiRangBuoc = regexp.MustCompile(
		`(?s)ADD CONSTRAINT tenant_domain_khong_danh_rieng CHECK \((.*?)\)\s*NOT VALID;`)
	mauKhongKhop = regexp.MustCompile(`host !~\* '([^']*)'`)
)

// THE LOCK-STEP TEST. The Go refusal (what stops resolution) and the CHECK (what stops insertion)
// are two spellings of one rule. If they disagree, a host is either refused at insert yet served
// if it got in some other way, or accepted into the table yet never resolved — the second is a
// commune onboarded onto an address that silently 404s.
//
// The CHECK is `NOT (host ~* p1) AND NOT (host ~* p2)`, so a host is reserved exactly when ANY
// pattern matches. The patterns are evaluated with Go regexp, case-insensitively as `~*` does; they
// use only anchors, alternation, groups, `?`, `*` and escaped dots, which PostgreSQL's ARE and RE2
// read identically.
func TestLuatTenMienDanhRiengGoVaSQLKhop(t *testing.T) {
	t.Parallel()

	noiDung, err := os.ReadFile(tepMigrationDanhRieng)
	if err != nil {
		t.Fatalf("đọc migration: %v", err)
	}
	khoi := khoiRangBuoc.FindSubmatch(noiDung)
	if khoi == nil {
		t.Fatalf("không tìm thấy CHECK tenant_domain_khong_danh_rieng ... NOT VALID trong %s — "+
			"đổi hình dạng câu lệnh thì đổi cả test này", tepMigrationDanhRieng)
	}
	cacMau := mauKhongKhop.FindAllSubmatch(khoi[1], -1)
	// Pinned: a third clause in some other shape (`<>`, `LIKE`) would be enforced by PostgreSQL and
	// invisible here, so the count is part of the contract.
	if len(cacMau) != 2 {
		t.Fatalf("CHECK có %d mẫu `host !~* '...'`, muốn 2 — test này chỉ hiểu hình dạng đó:\n%s",
			len(cacMau), khoi[1])
	}
	var mau []*regexp.Regexp
	for _, m := range cacMau {
		mau = append(mau, regexp.MustCompile(`(?i)`+string(m[1])))
	}
	sqlTuChoi := func(host string) bool {
		for _, r := range mau {
			if r.MatchString(host) {
				return true
			}
		}
		return false
	}

	for _, c := range bangTenMienDanhRieng {
		goNoi, sqlNoi := LaTenMienDanhRieng(c.Host), sqlTuChoi(c.Host)
		if goNoi != sqlNoi {
			t.Errorf("%q: Go nói danh riêng=%v, CHECK nói %v — hai nửa của một luật đã lệch nhau",
				c.Host, goNoi, sqlNoi)
		}
		if sqlNoi != c.DanhRieng {
			t.Errorf("%q: CHECK nói danh riêng=%v, bảng muốn %v", c.Host, sqlNoi, c.DanhRieng)
		}
	}
}
