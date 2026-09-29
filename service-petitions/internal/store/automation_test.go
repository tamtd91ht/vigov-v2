package store

import (
	"strings"
	"testing"
)

// What this defends WITHOUT a database: the automation reads bind the commune as $1 on every table of
// the join, exclude soft-deleted rows, carry no clock of their own, and never select a column that
// holds citizen personal data or free text. The behaviour against real rows is automation_pg_test.go.

func TestAutomationQueriesAreScopedLiveAndClockless(t *testing.T) {
	for name, q := range map[string]string{
		"tasks":   automationTaskQuery,
		"reports": automationCitizenReportQuery,
		"digest":  citizenReportDigestColumns,
	} {
		if strings.Contains(strings.ToLower(q), "now()") {
			t.Errorf("%s: dùng now() — `now` của lượt là claimed_at, không phải đồng hồ CSDL", name)
		}
		if name == "digest" {
			continue // single table: Scoped.Query adds the commune and the tail adds deleted_at
		}
		if !strings.Contains(q, ".tenant_id = $1") || !strings.Contains(q, ".deleted_at IS NULL") {
			t.Errorf("%s: thiếu ràng buộc xã hoặc loại dòng đã xoá", name)
		}
		// Every table of the join is constrained to the commune (core/store.QueryJoin's contract).
		tables := strings.Count(strings.ReplaceAll(q, "DISTINCT FROM", ""), "FROM ")
		if strings.Count(q, "tenant_id = $1") < tables {
			t.Errorf("%s: có bảng trong câu không bị ràng buộc xã", name)
		}
	}
}

func TestAutomationQueriesReadNoPersonalData(t *testing.T) {
	for _, q := range []string{automationTaskQuery, automationCitizenReportQuery, citizenReportDigestColumns} {
		for _, col := range []string{"noi_dung", "nguoi_gui_ho_ten", "nguoi_gui_dien_thoai", "dia_chi", "cong_dan_id",
			"tieu_de", "mo_ta", "ghi_chu"} {
			if strings.Contains(q, col) {
				t.Errorf("câu đọc cho việc nền chọn cột %s", col)
			}
		}
	}
}
