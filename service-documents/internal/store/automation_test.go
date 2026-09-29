package store

import (
	"strings"
	"testing"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

// What this defends WITHOUT a database: the automation read binds the commune as $1 on every table of
// the statement, excludes soft-deleted rows, carries no clock of its own, never selects free text, and
// uses the dashboard's own "open" predicate. The behaviour against real rows is automation_pg_test.go.

func TestAutomationIncomingQueryIsScopedLiveAndClockless(t *testing.T) {
	q, args := automationIncomingQuery()
	if strings.Contains(strings.ToLower(q), "now()") {
		t.Error("dùng now() — `now` của lượt là claimed_at, không phải đồng hồ CSDL")
	}
	if !strings.Contains(q, "v.tenant_id = $1") || !strings.Contains(q, "v.deleted_at IS NULL") {
		t.Error("thiếu ràng buộc xã hoặc loại dòng đã xoá")
	}
	tables := strings.Count(strings.ReplaceAll(q, "DISTINCT FROM", ""), "FROM ")
	if strings.Count(q, "tenant_id = $1") < tables {
		t.Errorf("có bảng trong câu không bị ràng buộc xã (%d bảng)", tables)
	}
	finished := domain.FinishedIncomingStatuses()
	if len(args) != len(finished) {
		t.Fatalf("tham số = %v, muốn đúng các trạng thái kết thúc %v", args, finished)
	}
	for i, s := range finished {
		if args[i] != string(s) {
			t.Errorf("tham số %d = %v, muốn %s", i, args[i], s)
		}
	}
}

func TestAutomationIncomingQueryReadsNoFreeText(t *testing.T) {
	q, _ := automationIncomingQuery()
	for _, col := range []string{"trich_yeu", "co_quan_ban_hanh", "so_ky_hieu", "noi_dung"} {
		if strings.Contains(q, col) {
			t.Errorf("câu đọc cho việc nền chọn cột %s", col)
		}
	}
}
