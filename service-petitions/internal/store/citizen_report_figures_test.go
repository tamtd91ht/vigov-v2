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

// The /phan-anh statistics reads — citizen_report_figures.go.
//
//	PROVED HERE   the count and the points run THE LIST's WHERE (registerFilter: commune $1 from the
//	              context, soft-deleted rows out, the restricted field out unless opened, every filter a
//	              bound parameter) · the points select lat/lng/status ONLY, located rows only, LIMIT =
//	              ceiling + 1, and refuse with NO rows past it · the breakdown reuses the /tong-quan
//	              tile's predicates byte for byte, groups by field and by residential unit with the grand
//	              row, drops a place row that shows nothing, refuses a missing grand row · the handling
//	              read binds BOTH tables to $1, reads hand-overs from the timeline at or before the finish,
//	              and maps a missing hand-over to the empty unit with no instant.
//	NOT PROVED    what PostgreSQL does with the statements (no VIGOV_TEST_DSN here) — in particular the
//	              "since" subquery of the handling read and GROUPING SETS' NULLs are asserted as text only.

// --- count and points: the list's filter ---------------------------------------------------------------

func TestCountRunsTheListFilter(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(7)}}
	s := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d)))
	loc := LocPhieu{TrangThai: "dang-xu-ly", ThonID: "thon-1", RatingMax: 2}
	n, err := s.CountCitizenReports(ctxXa(xaThu), loc)
	if err != nil || n != 7 {
		t.Fatalf("CountCitizenReports = %d, %v", n, err)
	}
	where, args := locPhieuThanhSQL(loc)
	l := d.stmts[0]
	if l.sql != "SELECT count(*) FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL"+where {
		t.Errorf("statement = %q — not the list's WHERE", l.sql)
	}
	if l.args[0] != string(xaThu) || len(l.args) != len(args)+1 {
		t.Errorf("args = %v", l.args)
	}
	if !strings.Contains(l.sql, dieuKienHanChe) {
		t.Error("the restricted field is counted for a reader without the key")
	}
	for _, v := range []string{"dang-xu-ly", "thon-1"} {
		if strings.Contains(l.sql, v) {
			t.Errorf("%q concatenated into the statement", v)
		}
	}
}

func TestCountRefusesAnInvalidMetricBeforeSQL(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(1)}}
	s := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d)))
	if _, err := s.CountCitizenReports(ctxXa(xaThu), LocPhieu{Metric: "khong-co"}); !errors.Is(err, ErrCitizenReportMetricInvalid) {
		t.Errorf("err = %v", err)
	}
	if len(d.stmts) != 0 {
		t.Error("ran SQL for a refused filter")
	}
}

// THE LIST AND THE COUNT READ ONE WHERE: DanhSach's filter is registerFilter's.
func TestListAndCountShareTheFilter(t *testing.T) {
	loc := LocPhieu{TrangThai: "da-tiep-nhan", ChiTreHan: true, ChoPhepHanChe: true}
	l := chayDanhSach(t, loc)
	filter, _, err := registerFilter(loc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(l.sql, filter) {
		t.Errorf("list statement %q does not carry registerFilter %q", l.sql, filter)
	}
}

func TestPointsSelectCoordinatesAndStatusOnly(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		{"lat": 15.512345, "lng": 108.234567, "trang_thai": "dang-xu-ly"},
	}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))
	pts, err := s.CitizenReportPoints(ctxXa(xaThu), LocPhieu{LinhVuc: "rac-thai"})
	if err != nil {
		t.Fatalf("CitizenReportPoints: %v", err)
	}
	if len(pts) != 1 || pts[0].Lat != 15.512345 || pts[0].Lng != 108.234567 || pts[0].Status != domain.DangXuLy {
		t.Errorf("points = %+v", pts)
	}
	l := k.lenh[0]
	if !strings.HasPrefix(l.sql, "SELECT lat, lng, trang_thai FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL") {
		t.Errorf("statement = %q", l.sql)
	}
	for _, frag := range []string{"lat IS NOT NULL AND lng IS NOT NULL", dieuKienHanChe, "ORDER BY id LIMIT $3"} {
		if !strings.Contains(l.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, l.sql)
		}
	}
	if l.args[len(l.args)-1] != int64(CitizenReportPointsCeiling+1) {
		t.Errorf("LIMIT = %v, want the ceiling PLUS ONE", l.args[len(l.args)-1])
	}
	for _, leak := range []string{"ma_tra_cuu", "noi_dung", "nguoi_gui", "dia_chi"} {
		if strings.Contains(l.sql[:strings.Index(l.sql, " FROM ")], leak) {
			t.Errorf("points select %q", leak)
		}
	}
}

func TestPointsPastTheCeilingRefuseWithNoRows(t *testing.T) {
	rows := make([]map[string]driver.Value, 0, CitizenReportPointsCeiling+1)
	for i := 0; i <= CitizenReportPointsCeiling; i++ {
		rows = append(rows, map[string]driver.Value{"lat": 15.5, "lng": 108.2, "trang_thai": "da-dong"})
	}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(&khoGia{hangTheoCot: rows})))
	pts, err := s.CitizenReportPoints(ctxXa(xaThu), LocPhieu{})
	if !errors.Is(err, ErrTooManyCitizenReportPoints) || pts != nil {
		t.Errorf("= %d points, %v — want none and the refusal", len(pts), err)
	}

	// Exactly at the ceiling is a complete answer.
	s = NewPhieuPhanAnhStore(store.New(moKhoGia(&khoGia{hangTheoCot: rows[:CitizenReportPointsCeiling]})))
	if pts, err := s.CitizenReportPoints(ctxXa(xaThu), LocPhieu{}); err != nil || len(pts) != CitizenReportPointsCeiling {
		t.Errorf("at the ceiling: %d, %v", len(pts), err)
	}
}

// --- the breakdown ----------------------------------------------------------------------------------------

func TestBreakdownReusesTheTilePredicates(t *testing.T) {
	cols := citizenReportBreakdownColumns()
	for _, m := range []domain.CitizenReportMetric{domain.CitizenReportReceived, domain.CitizenReportOnTimeSample,
		domain.CitizenReportOnTime, domain.CitizenReportLate, domain.CitizenReportRatingSample} {
		frag := "count(*) FILTER (WHERE " + citizenReportMetricCondition(m, "$2", "$3") + ")"
		if !strings.Contains(cols, frag) {
			t.Errorf("breakdown does not use the tile's %s predicate", m)
		}
		if !strings.Contains(citizenReportSummaryColumns(), frag) {
			t.Errorf("the tile's %s predicate changed text", m)
		}
	}
	if !strings.Contains(cols, citizenReportRatingSumColumn) {
		t.Error("rating sum is not the tile's")
	}
	// The overdue stock is the queue's two predicates, as of now(), never a flag column.
	for _, frag := range []string{citizenReportClassificationOverdue, citizenReportResolutionOverdue} {
		if !strings.Contains(cols, frag) {
			t.Errorf("overdue lacks the queue predicate %q", frag)
		}
	}
	// Residential units: received WITHOUT khong-tiep-nhan.
	if !strings.Contains(cols, "AND trang_thai <> 'khong-tiep-nhan'") {
		t.Error("residential-unit received includes declined reports")
	}
}

var brAsOf = time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)

// row builds one positional answer: groupedField, groupedPlace, field, place, then the seven counts
// (received, received-accepted, finished, sample, on-time, late, rating sample) + rating sum + overdue.
func brRow(gf, gp int64, field, place any, c ...int64) []driver.Value {
	v := []driver.Value{gf, gp, field, place}
	for i, n := range c {
		if i == 7 && n < 0 {
			v = append(v, nil) // NULL rating sum
			continue
		}
		v = append(v, n)
	}
	return append(v, brAsOf)
}

func TestBreakdownStatementAndRows(t *testing.T) {
	d := &countDriver{rows: [][]driver.Value{
		brRow(0, 1, "", nil, 2, 2, 0, 1, 0, 1, 0, -1, 1),
		brRow(0, 1, "rac-thai", nil, 11, 10, 8, 8, 6, 2, 5, 17, 3),
		brRow(1, 0, nil, "", 1, 1, 0, 0, 0, 0, 0, -1, 0),
		brRow(1, 0, nil, "thon-1", 12, 11, 8, 9, 6, 3, 5, 17, 4),
		brRow(1, 0, nil, "thon-2", 0, 0, 1, 0, 0, 0, 0, -1, 0), // in scope only by `finished`: not a row
		brRow(1, 1, nil, nil, 13, 12, 8, 9, 6, 3, 5, 17, 4),
	}}
	s := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d)))
	got, err := s.CitizenReportBreakdown(ctxXa(xaThu), periodA, false)
	if err != nil {
		t.Fatalf("CitizenReportBreakdown: %v", err)
	}
	if !got.AsOf.Equal(brAsOf) {
		t.Errorf("AsOf = %v", got.AsOf)
	}
	if got.Totals.OnTimeSample != 9 || got.Totals.OnTime != 6 || got.Totals.Late != 3 || got.Totals.Overdue != 4 {
		t.Errorf("totals = %+v", got.Totals)
	}
	if len(got.Fields) != 2 || got.Fields[0].FieldCode != "" || got.Fields[0].RatingSum != 0 ||
		got.Fields[1] != (domain.CitizenReportFieldFigures{FieldCode: "rac-thai", Received: 11, Finished: 8,
			OnTimeSample: 8, OnTime: 6, Late: 2, RatingSample: 5, RatingSum: 17, Overdue: 3}) {
		t.Errorf("fields = %+v", got.Fields)
	}
	if len(got.ResidentialUnits) != 2 || got.ResidentialUnits[0].ResidentialUnitID != "" ||
		got.ResidentialUnits[1] != (domain.CitizenReportResidentialUnitFigures{ResidentialUnitID: "thon-1", Received: 11, Overdue: 4}) {
		t.Errorf("residential units = %+v — the accepted count, the null-place row kept, empty rows dropped", got.ResidentialUnits)
	}
	if got.Units == nil {
		t.Error("Units nil — the use case fills it, the route must never see null")
	}

	l := d.stmts[0]
	if !strings.HasPrefix(l.sql, "SELECT "+citizenReportBreakdownColumns()+" FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL") {
		t.Errorf("statement does not bind the commune / exclude deleted rows: %q", l.sql)
	}
	if !strings.Contains(l.sql, dieuKienHanChe) {
		t.Error("restricted field counted without the key")
	}
	if !strings.HasSuffix(l.sql, " GROUP BY GROUPING SETS ((COALESCE(linh_vuc, '')), (COALESCE(thon_id, '')), ()) ORDER BY 1, 2, 3, 4") {
		t.Errorf("grouping: %q", l.sql)
	}
	if len(l.args) != 3 || l.args[0] != string(xaThu) || !periodArgsMatch(l.args, 1, 2) {
		t.Errorf("args = %v", l.args)
	}

	// With the key, the exclusion is gone.
	d2 := &countDriver{rows: [][]driver.Value{brRow(1, 1, nil, nil, 0, 0, 0, 0, 0, 0, 0, -1, 0)}}
	if _, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d2))).CitizenReportBreakdown(ctxXa(xaThu), periodA, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d2.stmts[0].sql, dieuKienHanChe) {
		t.Error("restricted field still excluded for a holder of the key")
	}
}

// ADR 0053 §C5: "Chưa xác định địa bàn" is a row even when nothing falls in it.
func TestBreakdownNoPlaceRowAlwaysPresent(t *testing.T) {
	d := &countDriver{rows: [][]driver.Value{
		brRow(1, 0, nil, "thon-1", 3, 3, 0, 0, 0, 0, 0, -1, 1),
		brRow(1, 1, nil, nil, 3, 3, 0, 0, 0, 0, 0, -1, 1),
	}}
	got, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CitizenReportBreakdown(ctxXa(xaThu), periodA, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ResidentialUnits) != 2 || got.ResidentialUnits[0] != (domain.CitizenReportResidentialUnitFigures{}) {
		t.Errorf("residential units = %+v — want the zero \"no place\" row first", got.ResidentialUnits)
	}
	d = &countDriver{rows: [][]driver.Value{brRow(1, 1, nil, nil, 0, 0, 0, 0, 0, 0, 0, -1, 0)}}
	got, _ = NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CitizenReportBreakdown(ctxXa(xaThu), periodA, false)
	if len(got.ResidentialUnits) != 1 {
		t.Errorf("empty period: %+v", got.ResidentialUnits)
	}
}

func TestBreakdownRefusals(t *testing.T) {
	d := &countDriver{rows: [][]driver.Value{brRow(0, 1, "rac-thai", nil, 1, 1, 0, 0, 0, 0, 0, -1, 0)}}
	if _, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CitizenReportBreakdown(ctxXa(xaThu), periodA, false); err == nil {
		t.Error("no grand row, yet an answer")
	}
	d = &countDriver{}
	if _, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CitizenReportBreakdown(ctxXa(xaThu), domain.Period{}, false); !errors.Is(err, domain.ErrPeriodInvalid) || len(d.stmts) != 0 {
		t.Errorf("empty period: %v, %d statements", err, len(d.stmts))
	}
	cause := errors.New("mất kết nối")
	if _, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(&countDriver{err: cause}))).CitizenReportBreakdown(ctxXa(xaThu), periodA, false); !errors.Is(err, cause) {
		t.Errorf("driver error not wrapped: %v", err)
	}
}

// --- the handling read -----------------------------------------------------------------------------------

func TestHandlingStatementBindsBothTablesAndReadsTheTimeline(t *testing.T) {
	since := time.Date(2026, 9, 25, 2, 0, 0, 0, time.UTC)
	finish := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	d := &countDriver{rows: [][]driver.Value{
		{"bp-moi-truong", since, finish},
		{"", nil, finish},
	}}
	got, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CitizenReportHandling(ctxXa(xaThu), periodA, false)
	if err != nil {
		t.Fatalf("CitizenReportHandling: %v", err)
	}
	if len(got) != 2 || got[0] != (domain.CitizenReportHandling{OrgUnitID: "bp-moi-truong", AssignedAt: since, FinishedAt: finish}) ||
		got[1].OrgUnitID != "" || !got[1].AssignedAt.IsZero() {
		t.Errorf("rows = %+v", got)
	}
	q := d.stmts[0].sql
	for _, frag := range []string{
		"FROM phieu_phan_anh\n\tWHERE tenant_id = $1 AND deleted_at IS NULL", dieuKienHanChe,
		"xu_ly_xong_luc >= $2 AND xu_ly_xong_luc < $3",
		"WHERE l.tenant_id = $1 AND l.hanh_vi = 'phan-cong' AND l.thoi_diem <= d.xu_ly_xong_luc",
		"ORDER BY pid, at DESC, lid DESC",
		// The span starts at the LAST assignment (owner decision 09/10/2026), the holder row itself.
		"SELECT COALESCE(h.unit, ''), h.at, d.xu_ly_xong_luc",
	} {
		if !strings.Contains(q, frag) {
			t.Errorf("handling statement lacks %q:\n%s", frag, q)
		}
	}
	// The petition's CURRENT holder is never read: a later re-assignment must not take credit.
	if strings.Contains(q, "bo_phan_id") && !strings.Contains(q, "l.bo_phan_id") {
		t.Error("the handling read uses the petition's current bo_phan_id")
	}
	if a := d.stmts[0].args; len(a) != 3 || a[0] != string(xaThu) || !periodArgsMatch(a, 1, 2) {
		t.Errorf("args = %v", a)
	}
}
