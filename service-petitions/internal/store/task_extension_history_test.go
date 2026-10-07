package store

import (
	"database/sql/driver"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The per-task extension HISTORY read, over the fake driver: the ONE statement it builds and the rows it
// scans. What PostgreSQL does with it is task_extension_history_pg_test.go (SKIPS without VIGOV_TEST_DSN).

var (
	historyFiledAt   = time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	historyDecidedAt = time.Date(2026, 9, 29, 4, 0, 0, 0, time.UTC)
	// A WEEK from the filing instant, so a positional swap between the two TIMESTAMPTZs is wrong data.
	historyNewDueAt = time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
)

func historyRows() []map[string]driver.Value {
	return []map[string]driver.Value{
		{
			"id": "dn-002", "nhiem_vu_id": "nv-001", "nguoi_de_nghi_ma": "CB-00311",
			"nguoi_duyet_ma": "CB-00123", "han_moi": historyNewDueAt, "ly_do": "Thiếu nhân lực.",
			"trang_thai": "tu-choi", "thoi_diem": historyFiledAt, "duyet_luc": historyDecidedAt,
			"decision_note": "Chưa đủ căn cứ.",
		},
		{
			// PENDING: the three decision columns are NULL and must scan to "" / zero, never an error.
			"id": "dn-001", "nhiem_vu_id": "nv-001", "nguoi_de_nghi_ma": "CB-00311",
			"nguoi_duyet_ma": nil, "han_moi": historyNewDueAt, "ly_do": "Chờ số liệu.",
			"trang_thai": "cho-duyet", "thoi_diem": historyFiledAt, "duyet_luc": nil,
			"decision_note": nil,
		},
	}
}

func parseHistory(t *testing.T, q url.Values) page.Request {
	t.Helper()
	req, err := page.Parse(q, TaskExtensionHistorySort)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	return req
}

// TestTaskHistoryScopedLiveNewestFirst — commune $1 from the context, the task as bound $2, soft-deleted
// requests excluded, NO status predicate (every status), newest first with the id tie-break, and the row
// scanned by name including the note.
func TestTaskHistoryScopedLiveNewestFirst(t *testing.T) {
	commune := tenant.ID("01JA" + strings.Repeat("A", 22))
	k := &khoGia{hangTheoCot: historyRows()}
	s := NewDeNghiLuiHanStore(pkgstore.New(moKhoGia(k)))

	out, err := s.TaskHistory(ctxXa(commune), "nv-001", parseHistory(t, url.Values{}))
	if err != nil {
		t.Fatalf("TaskHistory: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	if len(l.args) != 3 || l.args[0] != string(commune) || l.args[1] != "nv-001" {
		t.Fatalf("tham số = %v — muốn (xã từ ngữ cảnh, id nội bộ nhiệm vụ, limit+1)", l.args)
	}
	for _, want := range []string{
		"FROM de_nghi_lui_han",
		"WHERE tenant_id = $1",
		"AND nhiem_vu_id = $2",
		"AND deleted_at IS NULL",
		"decision_note",
		"ORDER BY thoi_diem DESC, id DESC",
		"LIMIT $3",
	} {
		if !strings.Contains(l.sql, want) {
			t.Errorf("câu lệnh thiếu %q: %s", want, l.sql)
		}
	}
	// EVERY status: the history must not inherit the queue's pending-only predicate.
	if strings.Contains(l.sql, "trang_thai =") || strings.Contains(l.sql, "JOIN") {
		t.Errorf("lịch sử lọc theo trạng thái hoặc join như hàng chờ: %s", l.sql)
	}

	if len(out.Items) != 2 {
		t.Fatalf("đọc %d dòng, muốn 2", len(out.Items))
	}
	decided, pending := out.Items[0], out.Items[1]
	if decided.ID != "dn-002" || decided.TrangThai != domain.TuChoiLuiHan || decided.NguoiDuyetMa != "CB-00123" ||
		!decided.DuyetLuc.Equal(historyDecidedAt) || !decided.HanMoi.Equal(historyNewDueAt) ||
		!decided.ThoiDiem.Equal(historyFiledAt) || decided.DecisionNote != "Chưa đủ căn cứ." ||
		decided.LyDo != "Thiếu nhân lực." || decided.NguoiDeNghiMa != "CB-00311" {
		t.Errorf("dòng đã quyết quét sai: %+v", decided)
	}
	if pending.NguoiDuyetMa != "" || !pending.DuyetLuc.IsZero() || pending.DecisionNote != "" ||
		pending.TrangThai != domain.ChoDuyetLuiHan {
		t.Errorf("dòng chờ duyệt quét sai (NULL phải thành rỗng/zero): %+v", pending)
	}
}

// TestTaskHistoryCursorIsDescendingAnchor — one row more than the limit gives a cursor, and the cursor
// comes back as the `(thoi_diem, id) < (…)` anchor in the next statement.
func TestTaskHistoryCursorIsDescendingAnchor(t *testing.T) {
	commune := tenant.ID("01JA" + strings.Repeat("A", 22))
	k := &khoGia{hangTheoCot: historyRows()}
	s := NewDeNghiLuiHanStore(pkgstore.New(moKhoGia(k)))

	first, err := s.TaskHistory(ctxXa(commune), "nv-001", parseHistory(t, url.Values{"limit": {"1"}}))
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	if !first.HasMore || first.NextCursor == "" || len(first.Items) != 1 {
		t.Fatalf("trang 1: %d dòng, has_more=%v cursor=%q", len(first.Items), first.HasMore, first.NextCursor)
	}

	k.lenh = nil
	if _, err := s.TaskHistory(ctxXa(commune), "nv-001",
		parseHistory(t, url.Values{"limit": {"1"}, "cursor": {first.NextCursor}})); err != nil {
		t.Fatalf("trang 2: %v", err)
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "AND (thoi_diem, id) < ($3, $4)") || !strings.Contains(l.sql, "LIMIT $5") {
		t.Errorf("trang 2 thiếu mốc con trỏ giảm dần: %s", l.sql)
	}
	if len(l.args) != 5 || l.args[1] != "nv-001" || l.args[3] != "dn-002" {
		t.Errorf("tham số trang 2 = %v, muốn $2 = nv-001, $4 = dn-002", l.args)
	}
}
