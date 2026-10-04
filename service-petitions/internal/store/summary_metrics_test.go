package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The overview figures, the overdue queues and the drill-down filter — task_summary.go and
// citizen_report_summary.go.
//
// THE PROPERTY THESE TESTS EXIST FOR: a figure and its drill-down list are the SAME predicate text.
// Asserted as a substring of both real statements, so a later edit to one spelling and not the other
// turns this red — which is the only way "the count equals the list" can hold without a database.
//
// NOT PROVED HERE: that PostgreSQL evaluates the predicates to the right rows. The task half has a
// PostgreSQL suite (summary_metrics_pg_test.go) that SKIPS without VIGOV_TEST_DSN; the petition half
// has none yet — see that file.

// --- a fake that answers ONE aggregate row -----------------------------------------------------------

// countDriver records every statement and answers a single row of `row`, whatever the SELECT list
// says. The shared fake (driver_gia_test.go) builds rows BY COLUMN NAME from a comma-split SELECT
// list, which cannot represent `count(*) FILTER (WHERE trang_thai IN ('a', 'b'))` — the commas inside
// the IN list would split one column into several.
type countDriver struct {
	stmts []lenhGia
	row   []driver.Value
	// rows, when set, is answered INSTEAD of row — for the grouped statement, which yields one row per
	// department plus the grand row.
	rows [][]driver.Value
	err  error
}

func (d *countDriver) Connect(context.Context) (driver.Conn, error) { return &countConn{d: d}, nil }
func (d *countDriver) Driver() driver.Driver                        { return trinhGia{} }

type countConn struct{ d *countDriver }

func (c *countConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("không hỗ trợ Prepare")
}
func (c *countConn) Close() error              { return nil }
func (c *countConn) Begin() (driver.Tx, error) { return nil, errors.New("không có giao dịch") }

func (c *countConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.stmts = append(c.d.stmts, lenhGia{sql: q, args: vals})
	if c.d.err != nil {
		return nil, c.d.err
	}
	if c.d.rows != nil {
		return &multiRows{rows: c.d.rows}, nil
	}
	return &countRows{row: c.d.row}, nil
}

// multiRows answers several rows of the same width.
type multiRows struct {
	rows [][]driver.Value
	i    int
}

func (r *multiRows) Columns() []string {
	cols := make([]string, len(r.rows[0]))
	for i := range cols {
		cols[i] = "c"
	}
	return cols
}
func (r *multiRows) Close() error { return nil }
func (r *multiRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

type countRows struct {
	row  []driver.Value
	done bool
}

func (r *countRows) Columns() []string {
	cols := make([]string, len(r.row))
	for i := range cols {
		cols[i] = "c"
	}
	return cols
}
func (r *countRows) Close() error { return nil }
func (r *countRows) Next(dest []driver.Value) error {
	// A nil row means "no rows at all" — the shape a queue with nothing overdue returns.
	if r.done || r.row == nil {
		return io.EOF
	}
	copy(dest, r.row)
	r.done = true
	return nil
}

var (
	periodA = domain.Period{
		From: time.Date(2026, 9, 21, 17, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 28, 17, 0, 0, 0, time.UTC),
	}
	// ONE assertion helper over both registers.
	periodArgsMatch = func(args []driver.Value, from, to int) bool {
		f, ok1 := args[from].(time.Time)
		t, ok2 := args[to].(time.Time)
		return ok1 && ok2 && f.Equal(periodA.From) && t.Equal(periodA.To)
	}
)

// --- tasks: one predicate for the figure and the list ------------------------------------------------

// TestTaskMetricFigureAndListShareOnePredicate — for every figure, the text inside the summary's
// FILTER and the text ANDed into the list's WHERE clause are the same bytes. With no other filter the
// list binds its period as $2/$3, exactly as the summary does, so the comparison needs no rewriting.
func TestTaskMetricFigureAndListShareOnePredicate(t *testing.T) {
	summary := taskSummaryColumns()
	for _, m := range domain.TaskMetrics {
		t.Run(string(m), func(t *testing.T) {
			cond := taskMetricCondition(m, "$2", "$3")
			if !strings.Contains(summary, "count(*) FILTER (WHERE "+cond+")") {
				t.Errorf("câu đếm không chứa đúng điều kiện của %s", m)
			}
			where, args := locNhiemVuThanhSQL(LocNhiemVu{Metric: m, Period: periodA})
			if where != " AND "+cond {
				t.Errorf("danh sách lọc %s bằng\n  %q\nmuốn\n  %q", m, where, " AND "+cond)
			}
			if m.PeriodBound() {
				if len(args) != 2 || !periodArgsMatch(args2values(args), 0, 1) {
					t.Errorf("chỉ số theo kỳ %s phải buộc đúng [from, to): %v", m, args)
				}
			} else if len(args) != 0 {
				// A stock figure given period arguments would be a statement with more arguments than
				// placeholders — PostgreSQL refuses it.
				t.Errorf("chỉ số hiện trạng %s không được buộc tham số kỳ: %v", m, args)
			}
		})
	}
}

func args2values(a []any) []driver.Value {
	out := make([]driver.Value, len(a))
	for i, v := range a {
		out[i] = v
	}
	return out
}

// TestTaskMetricAfterOtherFiltersShiftsPlaceholders — with other filters in front, the period moves
// to the next free placeholders and the other filters keep theirs.
func TestTaskMetricAfterOtherFiltersShiftsPlaceholders(t *testing.T) {
	where, args := locNhiemVuThanhSQL(LocNhiemVu{
		TrangThai: "hoan-thanh", Loai: "theo-van-ban", Metric: domain.TaskCompleted, Period: periodA,
	})
	if !strings.HasSuffix(where, " AND "+taskMetricCondition(domain.TaskCompleted, "$4", "$5")) {
		t.Errorf("kỳ không dời sang $4/$5 sau hai bộ lọc: %q", where)
	}
	if len(args) != 4 || args[0] != "hoan-thanh" || args[1] != "theo-van-ban" ||
		!periodArgsMatch(args2values(args), 2, 3) {
		t.Errorf("thứ tự tham số sai: %v", args)
	}
}

// TestTaskOverdueIsOpenWorkOnly — the tile counts work still OWED: not a task finished late
// (dieuKienTreHan's meaning, kept for the register's `late=true`), not `tam-dung`. A legacy
// `chuyen-tiep` row IS open work since P13 (28/09/2026) and so is counted, overdue included.
func TestTaskOverdueIsOpenWorkOnly(t *testing.T) {
	c := taskMetricCondition(domain.TaskOverdue, "", "")
	if strings.Contains(c, "ngay_hoan_thanh") {
		t.Error("quá hạn của tổng quan xét cả việc đã hoàn thành — việc xong trễ tháng trước không phải việc đang chờ")
	}
	if strings.Contains(c, dieuKienTreHan) {
		t.Error("quá hạn của tổng quan dùng lại dieuKienTreHan — điều kiện ấy khớp cả việc xong trễ mãi mãi")
	}
	for _, s := range []domain.TrangThaiNhiemVu{domain.MoiGiao, domain.DaTiepNhanNV, domain.DangThucHien,
		domain.ChoDuyet, domain.ChuyenTiep} {
		if !strings.Contains(c, "'"+string(s)+"'") {
			t.Errorf("thiếu trạng thái %s", s)
		}
	}
	for _, s := range []domain.TrangThaiNhiemVu{domain.TamDung, domain.HoanThanh} {
		if strings.Contains(c, "'"+string(s)+"'") {
			t.Errorf("trạng thái %s lọt vào quá hạn", s)
		}
	}
	if !strings.Contains(c, "han_xu_ly IS NOT NULL AND han_xu_ly < now()") || strings.Contains(c, "han_ban_dau") {
		t.Errorf("quá hạn phải đo theo han_xu_ly hiện hành với đồng hồ CSDL: %q", c)
	}
}

// TestTaskInProgressCountsLegacyForwarded (P13) — the in-progress figure and its drill-down share one
// predicate, and a `chuyen-tiep` row is in it; `tam-dung` and `hoan-thanh` stay out.
func TestTaskInProgressCountsLegacyForwarded(t *testing.T) {
	c := taskMetricCondition(domain.TaskInProgress, "", "")
	if !strings.Contains(c, "'"+string(domain.ChuyenTiep)+"'") {
		t.Errorf("đang thực hiện không đếm việc chuyen-tiep cũ: %q", c)
	}
	for _, s := range []domain.TrangThaiNhiemVu{domain.TamDung, domain.HoanThanh} {
		if strings.Contains(c, "'"+string(s)+"'") {
			t.Errorf("trạng thái %s lọt vào đang thực hiện", s)
		}
	}
	where, _ := locNhiemVuThanhSQL(LocNhiemVu{Metric: domain.TaskInProgress})
	if !strings.Contains(where, c) {
		t.Errorf("danh sách bấm vào không dùng cùng điều kiện với con số: %q", where)
	}
}

// TestTaskOnTimeMeasuredAgainstOriginalDeadline — §11.3: `ngay_hoan_thanh <= han_ban_dau`. Measured
// against `han_xu_ly`, every granted extension would read as a deadline met.
func TestTaskOnTimeMeasuredAgainstOriginalDeadline(t *testing.T) {
	c := taskMetricCondition(domain.TaskOnTime, "$2", "$3")
	if !strings.Contains(c, "ngay_hoan_thanh <= han_ban_dau") || strings.Contains(c, "han_xu_ly") {
		t.Errorf("đúng hạn phải so với han_ban_dau: %q", c)
	}
	if !strings.Contains(taskMetricCondition(domain.TaskOnTimeSample, "$2", "$3"), "han_ban_dau IS NOT NULL") {
		t.Error("mẫu đúng hạn gồm cả việc không có hạn")
	}
	// Half-open [from, to).
	if !strings.Contains(c, "ngay_hoan_thanh >= $2") || !strings.Contains(c, "ngay_hoan_thanh < $3") {
		t.Errorf("kỳ không phải nửa mở [from, to): %q", c)
	}
}

func TestTaskMetricUnknownIsFalse(t *testing.T) {
	if c := taskMetricCondition(domain.TaskMetric("bogus"), "$2", "$3"); c != "(FALSE)" {
		t.Errorf("metric lạ sinh %q — phải là FALSE, không bao giờ rỗng", c)
	}
}

// TestTaskListRefusesInvalidMetricBeforeQuery — the store floor under the handler.
func TestTaskListRefusesInvalidMetricBeforeQuery(t *testing.T) {
	for name, loc := range map[string]LocNhiemVu{
		"metric lạ":        {Metric: "bogus"},
		"theo kỳ, kỳ rỗng": {Metric: domain.TaskCompleted},
		"theo kỳ, kỳ ngược": {Metric: domain.TaskOnTime,
			Period: domain.Period{From: periodA.To, To: periodA.From}},
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoGia{}
			s := NewNhiemVuStore(store.New(moKhoGia(k)))
			yc, _ := page.Parse(url.Values{}, SapXepNhiemVu)
			if _, err := s.DanhSach(ctxXa(xaThu), loc, yc); !errors.Is(err, ErrTaskMetricInvalid) {
				t.Errorf("err = %v, muốn ErrTaskMetricInvalid", err)
			}
			if len(k.lenh) != 0 {
				t.Error("đã chạy câu lệnh cho một metric bị từ chối")
			}
		})
	}
}

// TestTaskSummaryStatement — one statement, commune $1 from the context, soft-deleted rows excluded,
// the period bound as $2/$3, and the six counts read back IN ORDER.
func TestTaskSummaryStatement(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(11), int64(12), int64(13), int64(14), int64(15), int64(16)}}
	s := NewNhiemVuStore(store.New(sql.OpenDB(d)))

	got, err := s.TaskSummary(ctxXa(xaThu), periodA)
	if err != nil {
		t.Fatalf("TaskSummary: %v", err)
	}
	want := domain.TaskSummary{InProgress: 11, Overdue: 12, Suspended: 13, Completed: 14, OnTimeSample: 15, OnTime: 16}
	if got != want {
		t.Errorf("= %+v, muốn %+v — thứ tự đọc lệch với thứ tự cột", got, want)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(d.stmts))
	}
	l := d.stmts[0]
	if !strings.Contains(l.sql, " FROM nhiem_vu WHERE tenant_id = $1 AND deleted_at IS NULL") {
		t.Errorf("câu đếm không buộc xã và loại dòng đã xoá: %q", l.sql)
	}
	if len(l.args) != 3 || l.args[0] != string(xaThu) || !periodArgsMatch(l.args, 1, 2) {
		t.Errorf("tham số = %v, muốn [xã, from, to]", l.args)
	}
	if !strings.HasPrefix(l.sql, "SELECT "+taskSummaryColumns()+" FROM") {
		t.Error("câu đếm không dùng taskSummaryColumns")
	}
}

func TestTaskSummaryRefusesEmptyPeriod(t *testing.T) {
	d := &countDriver{}
	s := NewNhiemVuStore(store.New(sql.OpenDB(d)))
	if _, err := s.TaskSummary(ctxXa(xaThu), domain.Period{}); !errors.Is(err, domain.ErrPeriodInvalid) {
		t.Errorf("err = %v, muốn ErrPeriodInvalid", err)
	}
	if len(d.stmts) != 0 {
		t.Error("kỳ rỗng mà vẫn chạy câu đếm — kỳ rỗng không phải \"mọi thời gian\"")
	}
}

func TestTaskSummaryWrapsDriverError(t *testing.T) {
	cause := errors.New("mất kết nối")
	s := NewNhiemVuStore(store.New(sql.OpenDB(&countDriver{err: cause})))
	if _, err := s.TaskSummary(ctxXa(xaThu), periodA); !errors.Is(err, cause) {
		t.Errorf("lỗi không được bọc bằng %%w: %v", err)
	}
}

// TestOverdueTasksStatement — the overdue FIGURE's predicate, oldest missed deadline first, bounded,
// and no title on the wire.
func TestOverdueTasksStatement(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		{"ma": "NV07", "loai": "theo-van-ban", "han_xu_ly": time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)},
	}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	items, err := s.OverdueTasks(ctxXa(xaThu), 7)
	if err != nil {
		t.Fatalf("OverdueTasks: %v", err)
	}
	if len(items) != 1 || items[0].Code != "NV07" || items[0].CategoryCode != "theo-van-ban" ||
		items[0].Kind != domain.DeadlineTask || !items[0].MissedDeadline.Equal(time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("dòng đọc sai: %+v", items)
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "AND deleted_at IS NULL AND "+taskMetricCondition(domain.TaskOverdue, "", "")+" ORDER BY han_xu_ly ASC, ma ASC LIMIT $2") {
		t.Errorf("hàng đợi không dùng đúng điều kiện của ô quá hạn: %q", l.sql)
	}
	if l.args[0] != string(xaThu) || l.args[1] != int64(7) {
		t.Errorf("tham số = %v", l.args)
	}
	if strings.Contains(l.sql, "tieu_de") || strings.Contains(l.sql, "mo_ta") {
		t.Error("hàng đợi đọc tiêu đề/mô tả — những chữ ấy hay trích lời người dân")
	}
}

func TestOverdueQueuesRefuseLimitOutsideBounds(t *testing.T) {
	for _, n := range []int{0, -1, OverdueQueueMax + 1} {
		k := &khoGia{}
		if _, err := NewNhiemVuStore(store.New(moKhoGia(k))).OverdueTasks(ctxXa(xaThu), n); !errors.Is(err, ErrOverdueQueueLimit) {
			t.Errorf("limit %d: err = %v", n, err)
		}
		if _, err := NewPhieuPhanAnhStore(store.New(moKhoGia(k))).OverdueCitizenReports(ctxXa(xaThu), n, false); !errors.Is(err, ErrOverdueQueueLimit) {
			t.Errorf("limit %d: err = %v", n, err)
		}
		if len(k.lenh) != 0 {
			t.Errorf("limit %d vẫn chạy câu lệnh", n)
		}
	}
}

// --- citizen reports ----------------------------------------------------------------------------------

func TestCitizenReportMetricFigureAndListShareOnePredicate(t *testing.T) {
	summary := citizenReportSummaryColumns()
	for _, m := range domain.CitizenReportMetrics {
		t.Run(string(m), func(t *testing.T) {
			cond := citizenReportMetricCondition(m, "$2", "$3")
			if !strings.Contains(summary, "count(*) FILTER (WHERE "+cond+")") {
				t.Errorf("câu đếm không chứa đúng điều kiện của %s", m)
			}
			// ChoPhepHanChe true, so the restricted exclusion is not in front and the metric is the
			// whole predicate — the exclusion is asserted on its own below.
			where, args := locPhieuThanhSQL(LocPhieu{ChoPhepHanChe: true, Metric: m, Period: periodA})
			if where != " AND "+cond {
				t.Errorf("danh sách lọc %s bằng\n  %q\nmuốn\n  %q", m, where, " AND "+cond)
			}
			if m.PeriodBound() != (len(args) == 2) {
				t.Errorf("số tham số kỳ của %s = %d", m, len(args))
			}
		})
	}
}

// TestCitizenReportRestrictedExclusionIsShared — the figure and the list exclude `can-bo` with the
// same text, decided by the same boolean. A figure that counted rows its list hides would come up
// short on click.
func TestCitizenReportRestrictedExclusionIsShared(t *testing.T) {
	where, _ := locPhieuThanhSQL(LocPhieu{Metric: domain.CitizenReportInProgress})
	if !strings.HasPrefix(where, restrictedFieldExclusion) {
		t.Errorf("danh sách đóng không loại can-bo trước: %q", where)
	}
	if !strings.Contains(restrictedFieldExclusion, "'"+domain.LinhVucHanChe+"'") {
		t.Error("hằng loại trừ không nhắc đúng mã lĩnh vực hạn chế")
	}

	for _, restricted := range []bool{false, true} {
		d := &countDriver{row: []driver.Value{int64(1), int64(2), int64(3), int64(4), int64(5),
			int64(6), int64(7), int64(8), int64(9)}}
		s := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d)))
		if _, err := s.CitizenReportSummary(ctxXa(xaThu), periodA, restricted); err != nil {
			t.Fatalf("CitizenReportSummary: %v", err)
		}
		has := strings.Contains(d.stmts[0].sql, restrictedFieldExclusion)
		if has == restricted {
			t.Errorf("restricted=%v: câu đếm loại can-bo = %v", restricted, has)
		}
	}
}

func TestCitizenReportSummaryStatement(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(21), int64(22), int64(23), int64(24), int64(25),
		int64(26), int64(27), int64(28), int64(101)}}
	s := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d)))
	got, err := s.CitizenReportSummary(ctxXa(xaThu), periodA, false)
	if err != nil {
		t.Fatalf("CitizenReportSummary: %v", err)
	}
	want := domain.CitizenReportSummary{Received: 21, InProgress: 22, OnTimeSample: 23, OnTime: 24, Late: 25,
		RatingSample: 26, LowRating: 27, PublicationPending: 28, RatingSum: 101}
	if got != want {
		t.Errorf("= %+v, muốn %+v", got, want)
	}
	l := d.stmts[0]
	if !strings.Contains(l.sql, " FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL") {
		t.Errorf("câu đếm không buộc xã và loại dòng đã xoá: %q", l.sql)
	}
	if len(l.args) != 3 || l.args[0] != string(xaThu) || !periodArgsMatch(l.args, 1, 2) {
		t.Errorf("tham số = %v", l.args)
	}
}

// TestCitizenReportOnTimeSampleIsAUnionB — open question #26 as decided: the sample is A ∪ B, late is
// late-A ∪ B, on time is on-time-A minus B. Asserted on the text because the property is that the
// three figures are BUILT from the same two pieces; semantic correctness needs PostgreSQL.
func TestCitizenReportOnTimeSampleIsAUnionB(t *testing.T) {
	sample := citizenReportMetricCondition(domain.CitizenReportOnTimeSample, "$2", "$3")
	onTime := citizenReportMetricCondition(domain.CitizenReportOnTime, "$2", "$3")
	late := citizenReportMetricCondition(domain.CitizenReportLate, "$2", "$3")

	// B — the ceiling branch — appears in all three, and on time NEGATES it.
	const ceiling = "han_phan_loai IS NOT NULL AND han_phan_loai >= $2 AND han_phan_loai < $3"
	for name, c := range map[string]string{"sample": sample, "on_time": onTime, "late": late} {
		if !strings.Contains(c, ceiling) {
			t.Errorf("%s không chứa nhánh B (trần phân loại trong kỳ)", name)
		}
	}
	if !strings.Contains(onTime, "AND NOT (han_phan_loai IS NOT NULL") {
		t.Error("đúng hạn không trừ nhánh B — phiếu trễ trần phân loại bị tính đúng hạn")
	}
	// B excludes the two terminal branches.
	if !strings.Contains(sample, "trang_thai NOT IN ('khong-tiep-nhan', 'chuyen-cap-tren')") {
		t.Error("nhánh B không loại hai nhánh kết thúc")
	}
	// A is measured on the settling instant, against the resolve deadline.
	if !strings.Contains(onTime, "xu_ly_xong_luc <= han_xu_ly_xong") || !strings.Contains(late, "xu_ly_xong_luc > han_xu_ly_xong") {
		t.Error("nhánh A không so thời điểm xử lý xong với han_xu_ly_xong")
	}
	// NULL-SAFETY — the guard that keeps `NOT B` from being NULL for an unclassified row.
	if !strings.Contains(sample, "(phan_loai_luc IS NOT NULL AND phan_loai_luc > han_phan_loai)") ||
		!strings.Contains(sample, "(phan_loai_luc IS NULL AND han_phan_loai < now())") {
		t.Error("nhánh B so phan_loai_luc mà không chặn NULL — NOT B sẽ thành NULL và rơi mất phiếu đúng hạn")
	}
	if !strings.Contains(sample, "xu_ly_xong_luc IS NOT NULL") {
		t.Error("nhánh A so xu_ly_xong_luc mà không chặn NULL")
	}
}

func TestCitizenReportInProgressAndReceived(t *testing.T) {
	ip := citizenReportMetricCondition(domain.CitizenReportInProgress, "", "")
	for _, s := range []domain.TrangThai{domain.DaDong, domain.KhongTiepNhan, domain.ChuyenCapTren} {
		if !strings.Contains(ip, "'"+string(s)+"'") {
			t.Errorf("đang xử lý không loại %s", s)
		}
	}
	if !strings.HasPrefix(ip, "(trang_thai NOT IN (") {
		t.Errorf("đang xử lý phải là \"mọi trạng thái trừ ba kết cục\" — phiếu mở lại vẫn được đếm: %q", ip)
	}
	rc := citizenReportMetricCondition(domain.CitizenReportReceived, "$2", "$3")
	if rc != "(vao_so_luc >= $2 AND vao_so_luc < $3)" {
		t.Errorf("tiếp nhận trong kỳ = %q — đo theo vao_so_luc, nửa mở, gồm cả khong-tiep-nhan", rc)
	}
}

// TestOverdueCitizenReportsStatement — both overdue branches, the restricted exclusion, ordering on
// the missed deadline, and no content or reporter column anywhere in the statement.
func TestOverdueCitizenReportsStatement(t *testing.T) {
	// The comma-split fake cannot read this SELECT list (CASE … IN (…)), so it answers no rows — the
	// statement is what is asserted.
	d := &countDriver{row: nil}
	s := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d)))
	if _, err := s.OverdueCitizenReports(ctxXa(xaThu), 10, false); err != nil {
		t.Fatalf("OverdueCitizenReports: %v", err)
	}
	l := d.stmts[0].sql
	for _, want := range []string{
		" FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL" + restrictedFieldExclusion,
		citizenReportClassificationOverdue,
		citizenReportResolutionOverdue,
		"ORDER BY 4 ASC, id ASC LIMIT $2",
	} {
		if !strings.Contains(l, want) {
			t.Errorf("thiếu %q trong\n%s", want, l)
		}
	}
	for _, cam := range []string{"noi_dung", "nguoi_gui_ho_ten", "nguoi_gui_dien_thoai", "dia_chi", "lat", "lng"} {
		if strings.Contains(l, cam) {
			t.Errorf("hàng đợi đọc cột dữ liệu cá nhân %q (luật 3)", cam)
		}
	}
	// Resolution overdue is by STATUS — a reopened petition keeps its first xu_ly_xong_luc.
	if strings.Contains(citizenReportResolutionOverdue, "xu_ly_xong_luc") {
		t.Error("quá hạn xử lý xét xu_ly_xong_luc — phiếu mở lại quá hạn sẽ biến mất khỏi hàng đợi")
	}
	if strings.Contains(citizenReportResolutionOverdue, "'"+string(domain.DaXuLy)+"'") ||
		strings.Contains(citizenReportResolutionOverdue, "'"+string(domain.ChoDanXacNhan)+"'") {
		t.Error("phiếu đã xử lý xong (chờ dân xác nhận) bị tính là việc trễ")
	}

	d2 := &countDriver{row: nil}
	if _, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d2))).OverdueCitizenReports(ctxXa(xaThu), 10, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d2.stmts[0].sql, restrictedFieldExclusion) {
		t.Error("có feedback.restricted mà hàng đợi vẫn loại can-bo")
	}
}

// --- the three stock figures of 2026-10-02 --------------------------------------------------------------

// TestCitizenReportSummaryExistingFiguresUnchanged — the five figures that existed before keep their
// SQL text BYTE FOR BYTE and their positions; the new ones are appended after them.
func TestCitizenReportSummaryExistingFiguresUnchanged(t *testing.T) {
	var before string
	for i, m := range domain.CitizenReportMetrics[:5] {
		if i > 0 {
			before += ", "
		}
		before += "count(*) FILTER (WHERE " + citizenReportMetricCondition(m, "$2", "$3") + ")"
	}
	if !strings.HasPrefix(citizenReportSummaryColumns(), before+", ") {
		t.Error("năm chỉ số cũ không còn đứng đầu, nguyên văn, đúng thứ tự")
	}
	if got := domain.CitizenReportMetrics[:5]; got[0] != domain.CitizenReportReceived ||
		got[1] != domain.CitizenReportInProgress || got[2] != domain.CitizenReportOnTimeSample ||
		got[3] != domain.CitizenReportOnTime || got[4] != domain.CitizenReportLate {
		t.Errorf("thứ tự năm chỉ số cũ đổi: %v", got)
	}
	if !strings.HasSuffix(citizenReportSummaryColumns(),
		", sum(diem_hai_long) FILTER (WHERE (diem_hai_long IS NOT NULL))") {
		t.Errorf("cột tổng số sao không phải cột cuối, lọc đúng mẫu rating_sample: %s", citizenReportSummaryColumns())
	}
}

// TestLowRatingFigureIsTheRatingMaxFilter — the card "n phiếu bị đánh giá thấp" and the register's
// "Bị đánh giá thấp" filter (rating_max=2) are ONE predicate.
func TestLowRatingFigureIsTheRatingMaxFilter(t *testing.T) {
	where, args := locPhieuThanhSQL(LocPhieu{ChoPhepHanChe: true, RatingMax: domain.LowRatingMaxStars})
	if where != " AND "+ratingAtMostCondition("$2") || len(args) != 1 || args[0] != 2 {
		t.Fatalf("bộ lọc rating_max = %q %v", where, args)
	}
	fig := citizenReportMetricCondition(domain.CitizenReportLowRating, "", "")
	if fig != "("+strings.Replace(ratingAtMostCondition("$2"), "$2", "2", 1)+")" {
		t.Errorf("chỉ số low_rating = %q — không cùng câu với bộ lọc rating_max=2", fig)
	}
	if !strings.Contains(fig, "diem_hai_long IS NOT NULL") {
		t.Error("phiếu chưa chấm sao bị tính là đánh giá thấp")
	}
	// The filter text itself did not move.
	if ratingAtMostCondition("$%d") != "diem_hai_long IS NOT NULL AND diem_hai_long <= $%d" {
		t.Errorf("câu lọc rating_max đổi chữ: %q", ratingAtMostCondition("$%d"))
	}
}

func TestPublicationPendingNeverCountsStaffConduct(t *testing.T) {
	c := citizenReportMetricCondition(domain.CitizenReportPublicationPending, "", "")
	if c != "(publication_status = 'cho-duyet' AND linh_vuc IS DISTINCT FROM 'can-bo')" {
		t.Errorf("chờ kiểm duyệt = %q — phải là cho-duyet và loại can-bo kể cả với người có feedback.restricted", c)
	}
	for _, m := range []domain.CitizenReportMetric{domain.CitizenReportRatingSample, domain.CitizenReportLowRating,
		domain.CitizenReportPublicationPending} {
		if m.PeriodBound() {
			t.Errorf("%s phải là chỉ số hiện trạng (không theo kỳ) — danh sách nó mở không lọc theo kỳ", m)
		}
		if strings.Contains(citizenReportMetricCondition(m, "$2", "$3"), "$") {
			t.Errorf("%s dùng tham số kỳ", m)
		}
	}
}

// TestCitizenReportSummaryNothingRatedSumIsZero — SQL's sum over no row is NULL; with a 0 sample it is
// read as 0, and the client shows a dash.
func TestCitizenReportSummaryNothingRatedSumIsZero(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(1), int64(0), int64(0), int64(0), int64(0),
		int64(0), int64(0), int64(0), nil}}
	got, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CitizenReportSummary(ctxXa(xaThu), periodA, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.RatingSum != 0 || got.RatingSample != 0 {
		t.Errorf("= %+v", got)
	}
}

// TestOverdueCitizenReportsReadsTheStatementNow — column 5 is now() itself, so the working time late
// is measured up to the instant the overdue predicate used; column 4 stays the ORDER BY key.
func TestOverdueCitizenReportsReadsTheStatementNow(t *testing.T) {
	if !strings.HasSuffix(overdueCitizenReportColumns, " AS missed, now() AS as_of") {
		t.Errorf("cột cuối không phải now(): %s", overdueCitizenReportColumns)
	}
	if !strings.Contains(citizenReportResolutionOverdue, "han_xu_ly_xong < now()") ||
		!strings.Contains(citizenReportClassificationOverdue, "han_phan_loai < now()") {
		t.Error("vị từ quá hạn không còn so với now() của chính câu lệnh")
	}
}
