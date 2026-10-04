package store

import (
	"database/sql/driver"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the Sổ tay lãnh đạo's server-side filters (ADR 0071) at the SQL they become:
// `scope=assigned-by-me` (LocNhiemVu.AssignedByStaffCode), `incomplete=true` (LocNhiemVu.Incomplete)
// and the pending-extension count (DeNghiLuiHanStore.CountPending).
//
//	PROVED HERE   assigned-by-me is ONE bound code over EXACTLY the creator and assigner columns, in
//	              one parenthesised OR, numbered after the filters before it, never spliced into the
//	              text, and carrying none of `related`'s other clauses · incomplete excludes ONLY
//	              `hoan-thanh` and binds nothing · the list and the per-status count run the same
//	              predicate with the same values · the extension count runs the queue's own filter
//	              over the queue's own relation, commune-bound, in one statement, and returns its number.
//
//	NOT PROVED    PostgreSQL's reading of the predicates and the cross-commune exclusion on real rows —
//	              TestPgLeaderNotebookFilters, which SKIPS without VIGOV_TEST_DSN.

func TestTaskListAssignedBySQL(t *testing.T) {
	cond, args := locNhiemVuThanhSQL(LocNhiemVu{Loai: "co-ban", AssignedByStaffCode: "CB-00123"})

	want := " AND (nguoi_tao_ma = $3 OR lanh_dao_giao_viec_ma = $3)"
	if !strings.Contains(cond, want) {
		t.Fatalf("điều kiện = %q\nmuốn chứa %q", cond, want)
	}
	// NOT `related`: no assignee, no timeline, no unit (ADR 0071 names two columns and no more).
	for _, extra := range []string{"nguoi_thuc_hien_ma", "nhat_ky_nhiem_vu", "bo_phan_id"} {
		if strings.Contains(cond, extra) {
			t.Errorf("phạm vi 'tôi đã giao' kéo theo vế %s: %s", extra, cond)
		}
	}
	if strings.Contains(cond, "CB-00123") {
		t.Errorf("mã cán bộ bị ghép thẳng vào câu lệnh: %s", cond)
	}
	if len(args) != 2 || args[0] != "co-ban" || args[1] != "CB-00123" {
		t.Errorf("tham số = %v, muốn [co-ban CB-00123]", args)
	}
}

func TestTaskListIncompleteExcludesOnlyCompleted(t *testing.T) {
	cond, args := locNhiemVuThanhSQL(LocNhiemVu{Incomplete: true})

	if cond != " AND trang_thai <> '"+string(domain.HoanThanh)+"'" {
		t.Errorf("điều kiện = %q, muốn đúng một vế loại hoan-thanh", cond)
	}
	if len(args) != 0 {
		t.Errorf("tham số = %v, muốn không có", args)
	}
	// ⚠ NOT the overview's open set: `tam-dung` must not be excluded (ADR 0071).
	if strings.Contains(cond, string(domain.TamDung)) {
		t.Errorf("bộ lọc chưa hoàn thành loại cả tạm dừng: %s", cond)
	}
}

// TestLeaderNotebookAssignedListAndCountShareThePredicate — the Sổ tay's "Việc tôi đã giao" badge and
// the rows under it are one predicate with one set of values (ADR 0071: counts use the list's own).
func TestLeaderNotebookAssignedListAndCountShareThePredicate(t *testing.T) {
	loc := LocNhiemVu{AssignedByStaffCode: "CB-00123", Incomplete: true}
	predicate, args := locNhiemVuThanhSQL(loc)

	list := chayDanhSachNhiemVu(t, loc)
	if !strings.Contains(list.sql, "AND deleted_at IS NULL"+predicate) {
		t.Errorf("câu danh sách không dùng đúng điều kiện:\n%q\nmuốn chứa %q", list.sql, predicate)
	}

	k := &khoGia{hangTheoCot: []map[string]driver.Value{{"trang_thai": "moi-giao", "count(*)": int64(4)}}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))
	counts, err := s.CountByStatus(ctxXa(xaThu), loc)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts[domain.MoiGiao] != 4 {
		t.Errorf("số đếm = %v", counts)
	}
	count := k.lenh[0]
	if !strings.Contains(count.sql, "AND deleted_at IS NULL"+predicate+" GROUP BY trang_thai") {
		t.Errorf("câu đếm không dùng đúng điều kiện:\n%q\nmuốn chứa %q", count.sql, predicate)
	}
	want := append([]driver.Value{string(xaThu)}, toDriverValues(args)...)
	if !reflect.DeepEqual(count.args, want) {
		t.Errorf("tham số câu đếm = %v, muốn %v", count.args, want)
	}
	if !reflect.DeepEqual(list.args[:len(want)], want) {
		t.Errorf("tham số câu danh sách = %v, muốn bắt đầu bằng %v", list.args, want)
	}
}

func toDriverValues(in []any) []driver.Value {
	out := make([]driver.Value, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	return out
}

// TestCountPendingSharesTheQueuePredicate — the "Duyệt lùi hạn" badge counts the queue's own relation
// under the queue's own filter: same text, same values, commune bound.
func TestCountPendingSharesTheQueuePredicate(t *testing.T) {
	loc := LocDeNghiChoDuyet{LanhDaoGiaoViecMa: "CB-00123", TaskCode: "NV19"}
	filter, args := pendingExtensionFilter(loc)

	k := &khoGia{hangTheoCot: []map[string]driver.Value{{"count(*)": int64(7)}}}
	s := NewDeNghiLuiHanStore(store.New(moKhoGia(k)))
	n, err := s.CountPending(ctxXa(xaThu), loc)
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if n != 7 {
		t.Errorf("số đếm = %d, muốn 7", n)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	c := k.lenh[0]
	if c.sql != "SELECT count(*) FROM "+bangDeNghiChoDuyet+" WHERE tenant_id = $1 "+filter {
		t.Errorf("câu đếm = %q", c.sql)
	}
	want := append([]driver.Value{string(xaThu)}, toDriverValues(args)...)
	if !reflect.DeepEqual(c.args, want) {
		t.Errorf("tham số = %v, muốn %v", c.args, want)
	}
	if filter != " AND lanh_dao_giao_viec_ma = $3 AND nhiem_vu_ma = $4" {
		t.Errorf("điều kiện = %q", filter)
	}

	// The page runs the same filter text, so badge and rows are one set.
	kp := &khoGia{}
	sp := NewDeNghiLuiHanStore(store.New(moKhoGia(kp)))
	yc, _ := page.Parse(url.Values{}, SapXepDeNghiChoDuyet)
	if _, err := sp.ChoDuyet(ctxXa(xaThu), loc, yc); err != nil {
		t.Fatalf("ChoDuyet: %v", err)
	}
	if !strings.Contains(kp.lenh[0].sql, filter) {
		t.Errorf("câu trang không chứa điều kiện của câu đếm: %q", kp.lenh[0].sql)
	}
}

func TestCountPendingWithoutRowIsAnError(t *testing.T) {
	k := &khoGia{}
	s := NewDeNghiLuiHanStore(store.New(moKhoGia(k)))
	if _, err := s.CountPending(ctxXa(xaThu), LocDeNghiChoDuyet{}); err == nil {
		t.Error("câu đếm không trả dòng nào mà không lỗi — huy hiệu sẽ hiện 0 thay vì báo lỗi")
	}
}
