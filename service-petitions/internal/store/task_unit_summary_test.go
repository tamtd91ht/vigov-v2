package store

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The per-department task table (/bao-cao) — TaskUnitSummary in task_summary.go.
//
// THE PROPERTY THESE TESTS EXIST FOR: the table's completed / on_time_sample / on_time / overdue are
// the SAME predicate text as the /tong-quan tiles, so the department rows add up to the tiles. Asserted
// as substrings of the real statement. Semantic correctness: task_unit_summary_pg_test.go.

var unitAsOf = time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)

// TestTaskUnitSummaryReusesTheTilePredicates — every reused figure appears as
// `count(*) FILTER (WHERE <taskMetricCondition>)`, byte for byte.
func TestTaskUnitSummaryReusesTheTilePredicates(t *testing.T) {
	cols := taskUnitSummaryColumns()
	for _, m := range []domain.TaskMetric{domain.TaskCompleted, domain.TaskOnTimeSample, domain.TaskOnTime,
		domain.TaskOverdue} {
		if !strings.Contains(cols, "count(*) FILTER (WHERE "+taskMetricCondition(m, "$2", "$3")+")") {
			t.Errorf("bảng theo bộ phận không dùng đúng điều kiện của ô %s", m)
		}
		if !strings.Contains(taskSummaryColumns(), "count(*) FILTER (WHERE "+taskMetricCondition(m, "$2", "$3")+")") {
			t.Errorf("ô %s của tổng quan không còn cùng chữ", m)
		}
	}
	if !strings.Contains(cols, "count(*) FILTER (WHERE "+taskInHandCondition("$2", "$3")+")") {
		t.Error("cột total không dùng taskInHandCondition")
	}
}

// TestTaskInHandCondition — created before `to`, and not completed or completed at/after `from`.
func TestTaskInHandCondition(t *testing.T) {
	c := taskInHandCondition("$2", "$3")
	want := "(tao_luc < $3 AND (trang_thai <> 'hoan-thanh' OR ngay_hoan_thanh >= $2))"
	if c != want {
		t.Errorf("= %q, muốn %q", c, want)
	}
}

// TestTaskUnitSummaryStatement — ONE statement, commune $1, soft-deleted rows excluded, the period as
// $2/$3, grouped by the COALESCEd department with the empty grouping set, rows read in column order,
// the grand row read for as_of only.
func TestTaskUnitSummaryStatement(t *testing.T) {
	d := &countDriver{rows: [][]driver.Value{
		{int64(0), "", int64(3), int64(1), int64(1), int64(0), int64(2), unitAsOf},
		{int64(0), "bp-01", int64(11), int64(7), int64(6), int64(5), int64(4), unitAsOf},
		{int64(1), nil, int64(14), int64(8), int64(7), int64(5), int64(6), unitAsOf},
	}}
	s := NewNhiemVuStore(store.New(sql.OpenDB(d)))

	got, err := s.TaskUnitSummary(ctxXa(xaThu), periodA)
	if err != nil {
		t.Fatalf("TaskUnitSummary: %v", err)
	}
	if !got.AsOf.Equal(unitAsOf) {
		t.Errorf("AsOf = %v", got.AsOf)
	}
	want := []domain.TaskUnitFigures{
		{OrgUnitID: "", Total: 3, Completed: 1, OnTimeSample: 1, OnTime: 0, Overdue: 2},
		{OrgUnitID: "bp-01", Total: 11, Completed: 7, OnTimeSample: 6, OnTime: 5, Overdue: 4},
	}
	if len(got.Units) != len(want) {
		t.Fatalf("= %+v — dòng tổng không được thành một bộ phận", got.Units)
	}
	for i := range want {
		if got.Units[i] != want[i] {
			t.Errorf("dòng %d = %+v, muốn %+v", i, got.Units[i], want[i])
		}
	}

	if len(d.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1 — không đếm từng bộ phận một", len(d.stmts))
	}
	l := d.stmts[0]
	if !strings.HasPrefix(l.sql, "SELECT "+taskUnitSummaryColumns()+" FROM nhiem_vu WHERE tenant_id = $1 AND deleted_at IS NULL AND (") {
		t.Errorf("câu đếm không buộc xã và loại dòng đã xoá: %q", l.sql)
	}
	if !strings.HasSuffix(l.sql, " GROUP BY GROUPING SETS ((COALESCE(bo_phan_id, '')), ()) ORDER BY 1, 2") {
		t.Errorf("câu đếm không gom theo bộ phận kèm dòng tổng: %q", l.sql)
	}
	// The WHERE is the union of the figures — the overdue stock included, so a department whose only
	// work is overdue from before the period still appears.
	for _, part := range []string{taskInHandCondition("$2", "$3"), taskMetricCondition(domain.TaskCompleted, "$2", "$3"),
		taskMetricCondition(domain.TaskOverdue, "", "")} {
		if !strings.Contains(taskUnitSummaryTail(), part) {
			t.Errorf("điều kiện phạm vi thiếu %q", part)
		}
	}
	if len(l.args) != 3 || l.args[0] != string(xaThu) || !periodArgsMatch(l.args, 1, 2) {
		t.Errorf("tham số = %v, muốn [xã, from, to]", l.args)
	}
}

// TestTaskUnitSummaryNothingInScope — only the grand row: no unit, as_of still read, Units is [] and
// not nil.
func TestTaskUnitSummaryNothingInScope(t *testing.T) {
	d := &countDriver{rows: [][]driver.Value{
		{int64(1), nil, int64(0), int64(0), int64(0), int64(0), int64(0), unitAsOf},
	}}
	got, err := NewNhiemVuStore(store.New(sql.OpenDB(d))).TaskUnitSummary(ctxXa(xaThu), periodA)
	if err != nil {
		t.Fatal(err)
	}
	if got.Units == nil || len(got.Units) != 0 || !got.AsOf.Equal(unitAsOf) {
		t.Errorf("= %+v", got)
	}
}

// TestTaskUnitSummaryRefusesMissingGrandRow — the statement always yields the grand row; its absence
// is a broken statement, never "no as_of".
func TestTaskUnitSummaryRefusesMissingGrandRow(t *testing.T) {
	d := &countDriver{rows: [][]driver.Value{
		{int64(0), "bp-01", int64(1), int64(0), int64(0), int64(0), int64(0), unitAsOf},
	}}
	if _, err := NewNhiemVuStore(store.New(sql.OpenDB(d))).TaskUnitSummary(ctxXa(xaThu), periodA); err == nil {
		t.Error("thiếu dòng tổng mà vẫn trả kết quả")
	}
}

func TestTaskUnitSummaryRefusesEmptyPeriod(t *testing.T) {
	d := &countDriver{}
	s := NewNhiemVuStore(store.New(sql.OpenDB(d)))
	if _, err := s.TaskUnitSummary(ctxXa(xaThu), domain.Period{}); !errors.Is(err, domain.ErrPeriodInvalid) {
		t.Errorf("err = %v, muốn ErrPeriodInvalid", err)
	}
	if len(d.stmts) != 0 {
		t.Error("kỳ rỗng mà vẫn chạy câu đếm")
	}
}

func TestTaskUnitSummaryWrapsDriverError(t *testing.T) {
	cause := errors.New("mất kết nối")
	s := NewNhiemVuStore(store.New(sql.OpenDB(&countDriver{err: cause})))
	if _, err := s.TaskUnitSummary(ctxXa(xaThu), periodA); !errors.Is(err, cause) {
		t.Errorf("lỗi không được bọc bằng %%w: %v", err)
	}
}
