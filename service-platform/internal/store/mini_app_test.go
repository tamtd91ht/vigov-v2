package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Statement-level checks that run WITHOUT a database. The pg tests beside this file prove the
// behaviour; these pin the three clauses whose loss would turn nothing red on a one-row fixture.

func TestMiniAppQueryExcludesInactiveAndSoftDeleted(t *testing.T) {
	for _, clause := range []string{"m.is_active", "m.deleted_at IS NULL", "m.app_id = $1"} {
		if !strings.Contains(queryMiniApp, clause) {
			t.Errorf("truy vấn mini app thiếu %q — app tắt hoặc đã xoá mềm sẽ vẫn cấp xã:\n%s",
				clause, queryMiniApp)
		}
	}
	// Never follows a successor on its own (ADR 0045 §Chế độ).
	if strings.Contains(queryMiniApp, "tenant_succession") {
		t.Error("truy vấn mini app đi theo tenant_succession — app riêng không được tự chuyển xã")
	}
}

func TestCommuneProfileExcludesSoftDeletedAndScansInOrder(t *testing.T) {
	if !strings.Contains(communeProfileFilter, "deleted_at IS NULL") {
		t.Errorf("đọc hồ sơ hiển thị không loại dòng xoá mềm: %q", communeProfileFilter)
	}
	// The column list is positional — see Read.
	const want = `office_address, COALESCE(logo_url, ''), hotline, office_hours_text, introduction`
	if communeProfileColumns != want {
		t.Fatalf("thứ tự cột = %q, muốn %q — Read quét theo VỊ TRÍ", communeProfileColumns, want)
	}
}

func TestMiniAppEmptyAppIDRefusedBeforeDatabase(t *testing.T) {
	// A nil *sql.DB: reaching the database would panic, so a pass proves the refusal came first.
	d := &Directory{}
	_, err := d.MiniApp(context.Background(), "")
	if !errors.Is(err, domain.ErrAppIDEmpty) {
		t.Fatalf("err = %v, muốn ErrAppIDEmpty", err)
	}
}
