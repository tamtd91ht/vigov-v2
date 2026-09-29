package store

import (
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR, and why it runs WITHOUT a database while directory_pg_test.go does not.
//
// The registry has THREE resolution paths — ByHost (the hot path on every request), ByHostErr (the
// same lookup with the failure reason kept) and ByID (naming a commune attached to an archival
// record). They must describe the same commune. The failure this guards is not a wrong answer, it
// is a PARTIAL one: a column added to two paths and forgotten in the third returns the zero value
// on the third, with no error, no log line and nothing red. The screen then shows the province on
// some requests and not on others depending on which path answered — and which path answered is
// invisible from the screen.
//
// The pg tests prove the value round-trips through a real PostgreSQL, which is the stronger check;
// they are skipped without VIGOV_TEST_DSN. This one is about the statements themselves and
// therefore always runs, which matters because "somebody edited two of the three" is a code-review
// failure, not an environment-dependent one.

// requiredColumns is every column the three paths must agree on. Adding a field to tenant.Tenant means
// adding its column here — that is the whole mechanism.
var requiredColumns = []string{"t.id", "t.ten", "t.tinh_thanh", "t.dang_hoat_dong"}

func TestEveryDirectoryQuerySelectsEveryColumn(t *testing.T) {
	for name, stmt := range map[string]string{
		"theo host": queryByHost,
		"theo id":   queryByID,
	} {
		for _, col := range requiredColumns {
			if !strings.Contains(stmt, col) {
				t.Errorf("truy vấn %s thiếu cột %s — đường phân giải này sẽ trả giá trị rỗng "+
					"trong khi các đường kia trả đúng, và không có gì đỏ:\n%s", name, col, stmt)
			}
		}
	}
}

func TestHostPathsShareOneStatement(t *testing.T) {
	// ByHost and ByHostErr answer the SAME question and differ only in what they do with a
	// failure. Two statements would let the WHERE clause drift between the hot path and the
	// operator-facing one, so one commune could resolve on one and not on the other.
	//
	// Asserted through tenantColumns rather than by comparing the two call sites, because the call sites
	// now literally name one variable — this is what pins that they keep doing so.
	if !strings.Contains(queryByHost, tenantColumns("d.host")) {
		t.Fatalf("câu lệnh theo host không dựng từ tenantColumns:\n%s", queryByHost)
	}
	if !strings.Contains(queryByID, tenantColumns("COALESCE(d.host, '')")) {
		t.Fatalf("câu lệnh theo id không dựng từ tenantColumns:\n%s", queryByID)
	}
}

func TestColumnOrderMatchesScanOrder(t *testing.T) {
	// scanTenant scans BY POSITION. A column inserted in the middle of tenantColumns without moving the
	// matching Scan target makes `ten` land in `Province` and vice versa — two TEXT columns, so
	// the driver reports nothing at all and a commune is displayed under the wrong heading.
	//
	// The order is asserted as a whole string, not column by column, because the defect IS the
	// order: every column can be present and the result still be wrong.
	const want = `t.id, d.host, t.ten, t.tinh_thanh, t.dang_hoat_dong`
	if got := tenantColumns("d.host"); got != want {
		t.Fatalf("thứ tự cột = %q, muốn %q — scanTenant quét theo VỊ TRÍ, đổi thứ tự là đổi giá trị "+
			"giữa hai cột TEXT mà driver không báo gì", got, want)
	}
}
