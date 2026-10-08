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

// The letter read (ADR 0079 lô 5 Q18): the commune on every table, live rows only, no clock, no free
// text or personal data, the OPEN statuses bound from domain's own transition table, and only letters
// carrying a deadline.
func TestAutomationLetterQueryIsScopedLiveClocklessAndNamesNobody(t *testing.T) {
	q, args := automationLetterQuery()
	if strings.Contains(strings.ToLower(q), "now()") {
		t.Error("dùng now() — `now` của lượt là claimed_at")
	}
	if !strings.Contains(q, "c.tenant_id = $1") || !strings.Contains(q, "c.deleted_at IS NULL") {
		t.Error("thiếu ràng buộc xã hoặc loại dòng đã xoá")
	}
	tables := strings.Count(strings.ReplaceAll(q, "DISTINCT FROM", ""), "FROM ")
	if strings.Count(q, "tenant_id = $1") < tables {
		t.Errorf("có bảng trong câu không bị ràng buộc xã (%d bảng)", tables)
	}
	if !strings.Contains(q, "processing_due_at IS NOT NULL OR c.resolution_due_at IS NOT NULL") {
		t.Error("đọc cả đơn không có hạn — không bao giờ nhắc từ một hạn mặc định")
	}
	for _, col := range []string{"sender_name", "sender_phone", "sender_address", "summary", "letter_type", "content", "result_"} {
		if strings.Contains(q, col) {
			t.Errorf("câu đọc cho việc nền chọn cột %s", col)
		}
	}
	if len(args) == 0 || args[0] != string(domain.LetterLogRouting) {
		t.Fatalf("$2 phải là loại dòng luân chuyển, được %v", args)
	}
	var open []any
	for _, s := range domain.LetterStatuses {
		if !s.Finished() {
			open = append(open, string(s))
		}
	}
	if len(args[1:]) != len(open) {
		t.Fatalf("trạng thái mở = %v, muốn %v", args[1:], open)
	}
	for i, s := range open {
		if args[i+1] != s {
			t.Errorf("tham số %d = %v, muốn %v", i+1, args[i+1], s)
		}
	}
}
