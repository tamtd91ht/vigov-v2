package domain

import (
	"os"
	"regexp"
	"testing"
)

// reservedHostMigrationFile is the migration that carries the SQL half of the rule — the CHECK's BODY.
// 0012 renamed the constraint to tenant_domain_not_reserved and left the body alone (a rename does
// not touch it), so 0007 is still where the body the database enforces is written. A migration that
// ever REDEFINES the CHECK must move this constant to itself. Read from disk by a
// relative path rather than through the embedded migrations package: domain/ imports nothing but
// the standard library, tests included, and the file on disk IS what gets embedded.
const reservedHostMigrationFile = "../../migrations/0007_tenant_domain_khong_danh_rieng.sql"

// checkBlockPattern captures the body of the CHECK, and notMatchPattern every `host !~* '<pattern>'` in it.
var (
	checkBlockPattern = regexp.MustCompile(
		`(?s)ADD CONSTRAINT tenant_domain_khong_danh_rieng CHECK \((.*?)\)\s*NOT VALID;`)
	notMatchPattern = regexp.MustCompile(`host !~\* '([^']*)'`)
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
func TestReservedHostRuleGoAndSQLAgree(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(reservedHostMigrationFile)
	if err != nil {
		t.Fatalf("đọc migration: %v", err)
	}
	block := checkBlockPattern.FindSubmatch(content)
	if block == nil {
		t.Fatalf("không tìm thấy CHECK tenant_domain_khong_danh_rieng ... NOT VALID trong %s — "+
			"đổi hình dạng câu lệnh thì đổi cả test này", reservedHostMigrationFile)
	}
	matches := notMatchPattern.FindAllSubmatch(block[1], -1)
	// Pinned: a third clause in some other shape (`<>`, `LIKE`) would be enforced by PostgreSQL and
	// invisible here, so the count is part of the contract.
	if len(matches) != 2 {
		t.Fatalf("CHECK có %d mẫu `host !~* '...'`, muốn 2 — test này chỉ hiểu hình dạng đó:\n%s",
			len(matches), block[1])
	}
	var patterns []*regexp.Regexp
	for _, m := range matches {
		patterns = append(patterns, regexp.MustCompile(`(?i)`+string(m[1])))
	}
	sqlRefuses := func(host string) bool {
		for _, r := range patterns {
			if r.MatchString(host) {
				return true
			}
		}
		return false
	}

	for _, c := range reservedHostCases {
		goSays, sqlSays := IsReservedHost(c.Host), sqlRefuses(c.Host)
		if goSays != sqlSays {
			t.Errorf("%q: Go nói danh riêng=%v, CHECK nói %v — hai nửa của một luật đã lệch nhau",
				c.Host, goSays, sqlSays)
		}
		if sqlSays != c.Reserved {
			t.Errorf("%q: CHECK nói danh riêng=%v, bảng muốn %v", c.Host, sqlSays, c.Reserved)
		}
	}
}
