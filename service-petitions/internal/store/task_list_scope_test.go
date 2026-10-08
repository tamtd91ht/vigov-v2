package store

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
)

// Tests for the two identity-backed filters of the task list — `soon=true` (DueSoon*) and
// `scope=related` (Related) — at the SQL they become.
//
//	PROVED HERE   both are bound parameters with the right numbering after other filters · the
//	              due-soon window is (from, until] over `han_xu_ly` and unfinished rows only · the
//	              related clause ORs every clause of §3 under ONE bound staff code, with the unit
//	              clauses present only when units exist · a half window, an inverted window and a
//	              codeless related scope are refused before any statement · the list and the count
//	              share the builder.
//
//	NOT PROVED    PostgreSQL's reading of the subquery and the `id IN` binding under the due-sort
//	              derived table — TestPgTaskListSoonAndRelated, which SKIPS without VIGOV_TEST_DSN.

var (
	soonFrom  = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	soonUntil = time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
)

func TestTaskListSoonWindowSQL(t *testing.T) {
	cond, args := locNhiemVuThanhSQL(LocNhiemVu{TrangThai: "moi-giao", DueSoonFrom: soonFrom, DueSoonUntil: soonUntil})

	want := " AND han_xu_ly IS NOT NULL AND ngay_hoan_thanh IS NULL AND han_xu_ly > $3 AND han_xu_ly <= $4"
	if !strings.Contains(cond, want) {
		t.Fatalf("điều kiện = %q\nmuốn chứa %q", cond, want)
	}
	if len(args) != 3 || args[1] != soonFrom || args[2] != soonUntil {
		t.Errorf("tham số = %v, muốn trạng thái rồi hai đầu cửa sổ", args)
	}
}

// Each priority level's own upper end (ADR 0079 lô 2 Q4 b), bound, in code order, with the default's
// as ELSE — so a task with no priority (NULL) keeps the default row's threshold.
func TestTaskListSoonWindowPerPrioritySQL(t *testing.T) {
	khan, cao := soonUntil.Add(24*time.Hour), soonUntil.Add(-time.Hour)
	cond, args := locNhiemVuThanhSQL(LocNhiemVu{TrangThai: "moi-giao", DueSoonFrom: soonFrom, DueSoonUntil: soonUntil,
		DueSoonByPriority: &TaskDueSoonByPriority{Until: map[string]time.Time{"khan": khan, "cao": cao}}})

	want := " AND han_xu_ly > $3 AND han_xu_ly <= (CASE muc_uu_tien WHEN $5::text THEN $6::timestamptz" +
		" WHEN $7::text THEN $8::timestamptz ELSE $4::timestamptz END)"
	if !strings.Contains(cond, want) {
		t.Fatalf("điều kiện = %q\nmuốn chứa %q", cond, want)
	}
	if len(args) != 7 || args[1] != soonFrom || args[2] != soonUntil || args[3] != "cao" || args[4] != cao ||
		args[5] != "khan" || args[6] != khan {
		t.Errorf("tham số = %v", args)
	}
	if strings.Contains(cond, "khan") || strings.Contains(cond, "'cao'") {
		t.Errorf("mã mức ưu tiên bị ghép thẳng vào câu lệnh: %s", cond)
	}
}

func TestTaskListRelatedSQL(t *testing.T) {
	cond, args := locNhiemVuThanhSQL(LocNhiemVu{
		Loai:    "co-ban",
		Related: &TaskRelatedScope{StaffCode: "CB-00123", OrgUnits: []string{"bp-1", "bp-2"}},
	})

	for _, want := range []string{
		"nguoi_thuc_hien_ma = $3", "lanh_dao_giao_viec_ma = $3", "nguoi_tao_ma = $3",
		"id IN (SELECT nk.nhiem_vu_id FROM nhat_ky_nhiem_vu nk WHERE nk.tenant_id = $1 AND nk.nguoi_ma = $3)",
		"bo_phan_id IN ($4, $5)",
	} {
		if !strings.Contains(cond, want) {
			t.Errorf("điều kiện thiếu %q: %s", want, cond)
		}
	}
	// ADR 0065 NV5: the retired columns are never read — "tôi theo dõi" IS nguoi_thuc_hien_ma and
	// "bộ phận chủ trì" IS bo_phan_id.
	for _, retired := range []string{"chuyen_vien_theo_doi_ma", "co_quan_chu_tri_id"} {
		if strings.Contains(cond, retired) {
			t.Errorf("điều kiện còn đọc cột đã nghỉ %s: %s", retired, cond)
		}
	}
	if !strings.Contains(cond, " AND (nguoi_thuc_hien_ma = $3 OR ") || strings.Count(cond, " OR ") != 4 {
		t.Errorf("các vế phải nằm trong MỘT nhóm OR (5 vế): %s", cond)
	}
	if strings.Contains(cond, "CB-00123") || strings.Contains(cond, "bp-1") {
		t.Errorf("giá trị bị ghép thẳng vào câu lệnh: %s", cond)
	}
	if len(args) != 4 || args[1] != "CB-00123" || args[2] != "bp-1" || args[3] != "bp-2" {
		t.Errorf("tham số = %v", args)
	}
}

// TestTaskListRelatedWithoutUnitsHasNoUnitClause — no unit means the unit clause matches nothing,
// which is expressed by its ABSENCE, never by an empty `IN ()` or by dropping to every unit.
func TestTaskListRelatedWithoutUnitsHasNoUnitClause(t *testing.T) {
	cond, args := locNhiemVuThanhSQL(LocNhiemVu{Related: &TaskRelatedScope{StaffCode: "CB-00123"}})

	if strings.Contains(cond, "bo_phan_id") {
		t.Errorf("có vế bộ phận dù không có bộ phận nào: %s", cond)
	}
	if len(args) != 1 {
		t.Errorf("tham số = %v, muốn đúng mã cán bộ", args)
	}
}

func TestTaskListScopeRefusedBeforeAnyStatement(t *testing.T) {
	yc, _ := page.Parse(url.Values{}, SapXepNhiemVu)
	for name, loc := range map[string]LocNhiemVu{
		"thiếu đầu trên":     {DueSoonFrom: soonFrom},
		"thiếu đầu dưới":     {DueSoonUntil: soonUntil},
		"cửa sổ bị ngược":    {DueSoonFrom: soonUntil, DueSoonUntil: soonFrom},
		"liên quan không mã": {Related: &TaskRelatedScope{OrgUnits: []string{"bp-1"}}},
		"mức ưu tiên không cửa sổ": {DueSoonByPriority: &TaskDueSoonByPriority{
			Until: map[string]time.Time{"khan": soonUntil}}},
		"mức ưu tiên bị ngược": {DueSoonFrom: soonFrom, DueSoonUntil: soonUntil, DueSoonByPriority: &TaskDueSoonByPriority{
			Until: map[string]time.Time{"khan": soonFrom.Add(-time.Hour)}}},
		"mức ưu tiên mã rỗng": {DueSoonFrom: soonFrom, DueSoonUntil: soonUntil, DueSoonByPriority: &TaskDueSoonByPriority{
			Until: map[string]time.Time{"": soonUntil}}},
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoGia{}
			s := NewNhiemVuStore(store.New(moKhoGia(k)))

			_, errList := s.DanhSach(ctxXa(xaThu), loc, yc)
			_, errCount := s.CountByStatus(ctxXa(xaThu), loc)

			if !errors.Is(errList, ErrTaskDueSoonWindow) && !errors.Is(errList, ErrTaskRelatedNoStaff) {
				t.Errorf("danh sách: err = %v", errList)
			}
			if !errors.Is(errCount, ErrTaskDueSoonWindow) && !errors.Is(errCount, ErrTaskRelatedNoStaff) {
				t.Errorf("đếm: err = %v", errCount)
			}
			if len(k.lenh) != 0 {
				t.Errorf("chạy %d câu lệnh dù bộ lọc bị từ chối", len(k.lenh))
			}
		})
	}
}

// TestTaskCountSharesSoonAndRelated — the count runs the same builder, so both predicates appear in
// its statement with the same numbering.
func TestTaskCountSharesSoonAndRelated(t *testing.T) {
	k := &khoGia{}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	if _, err := s.CountByStatus(ctxXa(xaThu), LocNhiemVu{
		DueSoonFrom: soonFrom, DueSoonUntil: soonUntil,
		Related: &TaskRelatedScope{StaffCode: "CB-00123"},
	}); err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	q := k.lenh[0].sql
	for _, want := range []string{"han_xu_ly > $2 AND han_xu_ly <= $3", "nguoi_ma = $4"} {
		if !strings.Contains(q, want) {
			t.Errorf("câu đếm thiếu %q: %s", want, q)
		}
	}
}
