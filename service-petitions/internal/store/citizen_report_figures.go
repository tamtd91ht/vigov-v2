package store

// The /phan-anh statistics reads (owner decisions of 09/10/2026, batch A). SQL, and nothing else.
//
//	CountCitizenReports        the register total under the LIST's filters
//	CitizenReportPoints        the heat map under the LIST's filters — coordinates and status only
//	CitizenReportBreakdown     the period figures by field and by residential unit, plus the totals
//	CitizenReportHandling      the petitions finished in the period, with the unit holding each and since when
//
// ONE PREDICATE PER QUESTION, SHARED WITH THE SCREEN IT FEEDS. The count and the points go through
// registerFilter — the very WHERE the register list pages through — so the total over a list and the
// dots on the map are the same rows the list shows. The breakdown's on-time / late columns are
// citizenReportMetricCondition, the /tong-quan tile's text, so its field rows add up to the tile.
//
// Commune $1 from the context (rule 1, invariant 5); soft-deleted rows excluded (rule 7, invariant 2);
// the restricted field `can-bo` excluded unless the reader holds `feedback.restricted` — by the SAME
// constant the list uses (restrictedFieldExclusion).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// registerFilter is the WHERE tail of the register list: live rows, then the validated filters. THE
// ONE SPELLING — DanhSach, CountCitizenReports and CitizenReportPoints all read through it, so a
// filter added to the list reaches the total and the map in the same edit.
func registerFilter(loc LocPhieu) (string, []any, error) {
	if err := validateCitizenReportMetric(loc); err != nil {
		return "", nil, err
	}
	predicate, args := locPhieuThanhSQL(loc)
	// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2).
	return `AND deleted_at IS NULL` + predicate, args, nil
}

// CountCitizenReports is the number of rows GET /api/v1/citizen-reports would page through under `loc`.
// Paging is not part of `loc`, so it cannot change the total.
func (s *PhieuPhanAnhStore) CountCitizenReports(ctx context.Context, loc LocPhieu) (int, error) {
	tail, args, err := registerFilter(loc)
	if err != nil {
		return 0, err
	}
	rows, err := s.db.For(ctx).Query(ctx, "count(*)", "phieu_phan_anh", tail, args...)
	if err != nil {
		return 0, fmt.Errorf("phieu_phan_anh: đếm theo bộ lọc: %w", err)
	}
	defer rows.Close()
	var n int64
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("phieu_phan_anh: đếm theo bộ lọc: %w", err)
		}
		return 0, errors.New("phieu_phan_anh: đếm theo bộ lọc: câu đếm không trả dòng nào")
	}
	if err := rows.Scan(&n); err != nil {
		return 0, fmt.Errorf("phieu_phan_anh: đếm theo bộ lọc: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("phieu_phan_anh: đếm theo bộ lọc: %w", err)
	}
	return int(n), nil
}

// CitizenReportPointsCeiling bounds the heat map. The map draws the whole filter at once and is not
// paginated (service-comms' map-asset-points precedent, the same 5000); past it the route refuses and
// the officer narrows the filter, rather than a map with petitions silently missing.
const CitizenReportPointsCeiling = 5000

// ErrTooManyCitizenReportPoints — the filter matches more than CitizenReportPointsCeiling located rows.
var ErrTooManyCitizenReportPoints = fmt.Errorf("phieu_phan_anh: quá %d điểm trên bản đồ", CitizenReportPointsCeiling)

// citizenReportPointColumns is read by position in lockstep with the Scan below. `lat, lng` and the
// status — NOTHING ELSE (domain.CitizenReportPoint says why).
const citizenReportPointColumns = `lat, lng, trang_thai`

// CitizenReportPoints reads the located rows of the register list under `loc`. Rows without BOTH
// coordinates are not on the map and not counted against the ceiling.
//
// LIMIT IS THE CEILING PLUS ONE, so "too many" is detectable rather than indistinguishable from a
// complete set of exactly that size; the rows already read are DROPPED on refusal, never returned
// trimmed. `ORDER BY id` only makes two reads of one register agree; the map has no order.
func (s *PhieuPhanAnhStore) CitizenReportPoints(ctx context.Context, loc LocPhieu) ([]domain.CitizenReportPoint, error) {
	tail, args, err := registerFilter(loc)
	if err != nil {
		return nil, err
	}
	args = append(args, CitizenReportPointsCeiling+1)
	tail += ` AND lat IS NOT NULL AND lng IS NOT NULL ORDER BY id LIMIT $` + strconv.Itoa(len(args)+1)

	rows, err := s.db.For(ctx).Query(ctx, citizenReportPointColumns, "phieu_phan_anh", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: đọc điểm bản đồ: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CitizenReportPoint, 0, 64)
	for rows.Next() {
		var (
			p      domain.CitizenReportPoint
			status string
		)
		if err := rows.Scan(&p.Lat, &p.Lng, &status); err != nil {
			// NOT the coordinates: a location of a citizen's report is personal data (rule 3).
			return nil, fmt.Errorf("phieu_phan_anh: đọc điểm: %w", err)
		}
		p.Status = domain.TrangThai(status)
		out = append(out, p)
		if len(out) > CitizenReportPointsCeiling {
			return nil, ErrTooManyCitizenReportPoints
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: duyệt điểm: %w", err)
	}
	return out, nil
}

// --- the breakdown ------------------------------------------------------------------------------------

// citizenReportOverdueNow is "overdue as of now" — the overdue queue's own two predicates
// (OverdueCitizenReports), so the breakdown's stock figure and the "Cần xử lý ngay" panel are one set.
var citizenReportOverdueNow = `((` + citizenReportClassificationOverdue + `) OR (` +
	citizenReportResolutionOverdue + `))`

// citizenReportFinished is "the work was finished in the period" — the instant domain.QuaHan compares
// once the work is done (the resolve clock stops at `xu_ly_xong_luc`, not at closing).
func citizenReportFinished(from, to string) string {
	return `(xu_ly_xong_luc IS NOT NULL AND xu_ly_xong_luc >= ` + from + ` AND xu_ly_xong_luc < ` + to + `)`
}

// citizenReportReceivedAccepted is `received` without the reports the commune declined
// (`khong-tiep-nhan`) — the residential-unit figure, where a declined report is not a problem located there.
func citizenReportReceivedAccepted(from, to string) string {
	return `(` + citizenReportMetricCondition(domain.CitizenReportReceived, from, to) +
		` AND trang_thai <> '` + string(domain.KhongTiepNhan) + `')`
}

// The two grouping keys. COALESCE so "no field" / "no place" is ONE row each, never NULL and ” split.
const (
	breakdownFieldKey = `COALESCE(linh_vuc, '')`
	breakdownPlaceKey = `COALESCE(thon_id, '')`
)

// citizenReportBreakdownColumns is the SELECT list, read by position in lockstep with the Scan.
//
// THE ON-TIME COLUMNS ARE citizenReportMetricCondition'S TEXT, NOT A RESPELLING — so the field rows sum
// to the /tong-quan tile for the same period, and the grand row IS the tile.
func citizenReportBreakdownColumns() string {
	cond := func(m domain.CitizenReportMetric) string { return citizenReportMetricCondition(m, "$2", "$3") }
	return "GROUPING(" + breakdownFieldKey + "), GROUPING(" + breakdownPlaceKey + "), " +
		breakdownFieldKey + ", " + breakdownPlaceKey +
		", count(*) FILTER (WHERE " + cond(domain.CitizenReportReceived) + ")" +
		", count(*) FILTER (WHERE " + citizenReportReceivedAccepted("$2", "$3") + ")" +
		", count(*) FILTER (WHERE " + citizenReportFinished("$2", "$3") + ")" +
		", count(*) FILTER (WHERE " + cond(domain.CitizenReportOnTimeSample) + ")" +
		", count(*) FILTER (WHERE " + cond(domain.CitizenReportOnTime) + ")" +
		", count(*) FILTER (WHERE " + cond(domain.CitizenReportLate) + ")" +
		", count(*) FILTER (WHERE " + cond(domain.CitizenReportRatingSample) + ")" +
		", " + citizenReportRatingSumColumn +
		", count(*) FILTER (WHERE " + citizenReportOverdueNow + ")" +
		", now()"
}

// citizenReportBreakdownTail scopes, restricts to the rows some figure counts, then groups.
//
// scopeFilter IS THE ONE SCOPE OF EVERY FIGURE READ (summary, queue, breakdown): live rows, and the
// restricted field unless the reader holds the key. A later batch that excludes merged ("phụ")
// petitions from the figures adds its clause THERE, once, for all of them.
//
// THREE GROUPING SETS IN ONE STATEMENT — by field, by residential unit, and `()` — so every figure is
// read at ONE instant and the empty set yields the grand row (section (a) and `now()`) even when no
// row matches. The WHERE is the union of the figures (on_time / late are subsets of the sample, and
// received-accepted of received).
func citizenReportBreakdownTail(restricted bool) string {
	return scopeFilter(restricted) + ` AND (` +
		citizenReportMetricCondition(domain.CitizenReportReceived, "$2", "$3") + ` OR ` +
		citizenReportFinished("$2", "$3") + ` OR ` +
		citizenReportMetricCondition(domain.CitizenReportOnTimeSample, "$2", "$3") + ` OR ` +
		citizenReportMetricCondition(domain.CitizenReportRatingSample, "", "") + ` OR ` +
		citizenReportOverdueNow + `)` +
		` GROUP BY GROUPING SETS ((` + breakdownFieldKey + `), (` + breakdownPlaceKey + `), ()) ORDER BY 1, 2, 3, 4`
}

// CitizenReportBreakdown counts the field and residential-unit figures of one period in ONE statement.
// `restricted` is the reader's `feedback.restricted` fact; false — the zero value — excludes `can-bo`.
// The Units section is not here: it needs identity's calendar (app.CitizenReportBreakdown).
func (s *PhieuPhanAnhStore) CitizenReportBreakdown(ctx context.Context, p domain.Period, restricted bool) (
	domain.CitizenReportBreakdown, error) {

	if _, err := domain.NewPeriod(p.From, p.To); err != nil {
		return domain.CitizenReportBreakdown{}, fmt.Errorf("phieu_phan_anh: thống kê theo kỳ: %w", err)
	}
	rows, err := s.db.For(ctx).Query(ctx, citizenReportBreakdownColumns(), "phieu_phan_anh",
		citizenReportBreakdownTail(restricted), p.From, p.To)
	if err != nil {
		return domain.CitizenReportBreakdown{}, fmt.Errorf("phieu_phan_anh: thống kê theo kỳ: %w", err)
	}
	defer rows.Close()

	out := domain.CitizenReportBreakdown{
		Fields:           []domain.CitizenReportFieldFigures{},
		ResidentialUnits: []domain.CitizenReportResidentialUnitFigures{},
		Units:            []domain.CitizenReportUnitFigures{},
	}
	var sawGrand bool
	for rows.Next() {
		var (
			groupedField, groupedPlace int64
			field, place               sql.NullString
			c                          [7]int64
			ratingSum                  sql.NullInt64
			overdue                    int64
			asOf                       time.Time
		)
		if err := rows.Scan(&groupedField, &groupedPlace, &field, &place,
			&c[0], &c[1], &c[2], &c[3], &c[4], &c[5], &c[6], &ratingSum, &overdue, &asOf); err != nil {
			return domain.CitizenReportBreakdown{}, fmt.Errorf("phieu_phan_anh: thống kê theo kỳ: đọc dòng: %w", err)
		}
		fig := domain.CitizenReportFieldFigures{
			FieldCode: field.String, Received: int(c[0]), Finished: int(c[2]),
			OnTimeSample: int(c[3]), OnTime: int(c[4]), Late: int(c[5]),
			RatingSample: int(c[6]), RatingSum: int(ratingSum.Int64), Overdue: int(overdue),
		}
		switch {
		case groupedField == 1 && groupedPlace == 1:
			// The `()` set: section (a), and the statement's now().
			sawGrand = true
			fig.FieldCode = ""
			out.Totals, out.AsOf = fig, asOf
		case groupedField == 0:
			out.Fields = append(out.Fields, fig)
		default:
			// A place in scope only through a figure it does not show (finished, rating) is not a row —
			// except "no place", which ADR 0053 §C5 keeps ALWAYS (added below when absent).
			if place.String != "" && c[1] == 0 && overdue == 0 {
				continue
			}
			out.ResidentialUnits = append(out.ResidentialUnits, domain.CitizenReportResidentialUnitFigures{
				ResidentialUnitID: place.String, Received: int(c[1]), Overdue: int(overdue),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return domain.CitizenReportBreakdown{}, fmt.Errorf("phieu_phan_anh: thống kê theo kỳ: %w", err)
	}
	if !sawGrand {
		// The empty grouping set always yields a row; none means the statement is not what was written.
		return domain.CitizenReportBreakdown{}, errors.New("phieu_phan_anh: thống kê theo kỳ: thiếu dòng tổng của câu đếm")
	}
	// "Chưa xác định địa bàn" IS ALWAYS A ROW, zero included (ADR 0053 §C5): an absent row reads as
	// "every report had a place", which is a claim, while a 0 is a count.
	hasNoPlace := false
	for _, ru := range out.ResidentialUnits {
		if ru.ResidentialUnitID == "" {
			hasNoPlace = true
			break
		}
	}
	if !hasNoPlace {
		out.ResidentialUnits = append([]domain.CitizenReportResidentialUnitFigures{{}}, out.ResidentialUnits...)
	}
	return out, nil
}

// citizenReportHandlingStmt reads, for every petition finished in the period, the unit holding it at
// `xu_ly_xong_luc` and the instant of the LAST assignment before that (owner decision 09/10/2026, ADR
// 0053 §C4 open item 1: "từ LẦN GIAO CUỐI cho bộ phận đang giữ phiếu lúc xu_ly_xong_luc").
//
//	done     the petitions finished in [from, to), scoped like every figure (scopeFilter)
//	steps    their `phan-cong` timeline rows AT OR BEFORE the finishing instant — the processing log
//	         (migration 0013) is the only record of who held a petition WHEN; the petition's own
//	         `bo_phan_id` says who holds it NOW and would credit a later re-assignment with old work
//	holder   the latest of those rows (ties in one transaction broken by id): its unit is the unit at
//	         finishing, its `thoi_diem` the start of the measured span. A re-assignment inside the same
//	         unit is a later `phan-cong` row and so restarts the span — that is what "lần giao cuối" says
//
// No `phan-cong` row before finishing (a petition assigned before migration 0013) leaves holder NULL:
// the petition is counted as finished under "" and left out of the handling sample — never credited to
// the unit that holds it today.
//
// EVERY TABLE IS BOUND TO $1 (core/store QueryJoin's contract): the log is joined on its own
// `tenant_id = $1`, never on the petition id alone.
func citizenReportHandlingStmt(restricted bool) string {
	return `WITH done AS (
	SELECT id, xu_ly_xong_luc FROM phieu_phan_anh
	WHERE tenant_id = $1 ` + scopeFilter(restricted) + ` AND ` + citizenReportFinished("$2", "$3") + `
), steps AS (
	SELECT l.phieu_phan_anh_id AS pid, l.bo_phan_id AS unit, l.thoi_diem AS at, l.id AS lid
	FROM nhat_ky_phan_anh l JOIN done d ON d.id = l.phieu_phan_anh_id
	WHERE l.tenant_id = $1 AND l.hanh_vi = '` + string(domain.NhatKyPhanCong) + `' AND l.thoi_diem <= d.xu_ly_xong_luc
), holder AS (
	SELECT DISTINCT ON (pid) pid, unit, at FROM steps ORDER BY pid, at DESC, lid DESC
)
SELECT COALESCE(h.unit, ''), h.at, d.xu_ly_xong_luc
FROM done d LEFT JOIN holder h ON h.pid = d.id
ORDER BY d.id`
}

// CitizenReportHandling reads one row per petition finished in the period — see citizenReportHandlingStmt.
// NO CODE, NO CONTENT: a unit, two instants.
func (s *PhieuPhanAnhStore) CitizenReportHandling(ctx context.Context, p domain.Period, restricted bool) (
	[]domain.CitizenReportHandling, error) {

	if _, err := domain.NewPeriod(p.From, p.To); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: thời gian xử lý theo bộ phận: %w", err)
	}
	// $1 is the commune (QueryJoin), and BOTH tables carry `tenant_id = $1` in the statement.
	rows, err := s.db.For(ctx).QueryJoin(ctx, citizenReportHandlingStmt(restricted), p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: thời gian xử lý theo bộ phận: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CitizenReportHandling, 0, 64)
	for rows.Next() {
		var (
			h     domain.CitizenReportHandling
			since sql.NullTime
		)
		if err := rows.Scan(&h.OrgUnitID, &since, &h.FinishedAt); err != nil {
			return nil, fmt.Errorf("phieu_phan_anh: thời gian xử lý theo bộ phận: đọc dòng: %w", err)
		}
		if since.Valid && h.OrgUnitID != "" {
			h.AssignedAt = since.Time
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: thời gian xử lý theo bộ phận: %w", err)
	}
	return out, nil
}
