package http

// The incoming register's half of the leadership dashboard /tong-quan
// (docs/ui-ux/01-tong-quan-dieu-hanh.md §4.2 and §5), plus the drill-down filter on the list route.
//
// THREE SURFACES, ONE PREDICATE PER FIGURE. The summary counts, the drill-down lists, and the queue
// reads — and all three reach the same store.metricPredicate, so the row count behind a figure is the
// figure (docs/ui-ux/13-bao-cao.md §10: /tong-quan and the lists must not disagree).

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// incomingSummaryOut is GET /api/v1/incoming-document-summary.
//
// COUNTS ONLY. No on-time ratio: the register does not store when a document was settled
// (domain.VanBanDen.QuaHan), so "đúng hạn" cannot be computed without inventing that instant. No
// citizen letters: `don_thu` is not in this service yet.
//
// `as_of` IS THE INSTANT `open` AND `overdue` WERE MEASURED AT. Those two are stock figures — they
// describe the register now, not the period — and a screen that renders them beside a period should
// be able to say when "now" was.
type incomingSummaryOut struct {
	From    string `json:"from"`  // RFC 3339, echoed
	To      string `json:"to"`    // RFC 3339, echoed — the period is [from, to)
	AsOf    string `json:"as_of"` // RFC 3339
	Arrived int    `json:"arrived"`
	Open    int    `json:"open"`
	Overdue int    `json:"overdue"`
}

// overdueQueueItemOut is one row of "CẦN XỬ LÝ NGAY".
//
// `code` IS THE REGISTER CODE (domain.MaVanBanDen, "VB-DEN-2026-0007"), CHOSEN OVER THE SUMMARY ON
// PURPOSE. §5 draws the row with a title, but `trich_yeu` is free text that may name a citizen
// (rule 3), and this block sits on the first screen every leader opens — the most widely seen,
// most often screenshotted surface in the product. The code carries the number and year and nothing
// personal; the summary is one click away, in the drawer, behind `document.read`.
//
// `kind` IS "van-ban-den": §5's block merges several modules, and the value is Vietnamese without
// diacritics because it names a business record type (ADR 0011).
//
// `due_at` IS THE MISSED DEADLINE, RFC 3339, the same field name the register's own rows use.
type overdueQueueItemOut struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Code        string `json:"code"`
	DueAt       string `json:"due_at"`
	Critical    bool   `json:"critical"`
	HoldingUnit string `json:"holding_unit,omitempty"` // identity's `bo_phan.id`
}

type overdueQueueOut struct {
	Items []overdueQueueItemOut `json:"items"`
	AsOf  string                `json:"as_of"` // RFC 3339 — the instant "overdue" and "critical" were judged at
}

// overdueQueueKind is the `kind` value of this register's rows: an English NAME holding the ADR 0011
// enum VALUE.
const overdueQueueKind = "van-ban-den"

// now is the handler's clock. Deps.Clock is a seam for tests; nil is the real clock.
func (h *Handler) now() time.Time {
	if h.d.Clock == nil {
		return time.Now().UTC()
	}
	return h.d.Clock().UTC()
}

var (
	errPeriodFormat   = errors.New("`from` và `to` phải theo dạng RFC 3339, ví dụ 2026-09-01T00:00:00+07:00")
	errMetricUnknown  = errors.New("`metric` chỉ nhận arrived, open hoặc overdue")
	errPeriodNoMetric = errors.New("`from` / `to` chỉ dùng cùng `metric=arrived`")
	errQueueLimit     = errors.New("`limit` phải là số nguyên dương")
)

// parsePeriod reads the REQUIRED half-open period [from, to). There is no default period: a figure
// for a period nobody chose is a figure somebody will read as the one they meant.
func parsePeriod(q url.Values) (from, to time.Time, window domain.ArrivalWindow, err error) {
	fs, ts := q.Get("from"), q.Get("to")
	if fs == "" || ts == "" {
		return from, to, window, domain.ErrPeriodMissing
	}
	if from, err = time.Parse(time.RFC3339, fs); err != nil {
		return from, to, window, errPeriodFormat
	}
	if to, err = time.Parse(time.RFC3339, ts); err != nil {
		return from, to, window, errPeriodFormat
	}
	window, err = domain.ArrivalWindowFor(from, to)
	return from, to, window, err
}

// isPeriodRefusal reports whether err is the caller's period being wrong (400) rather than a failure.
func isPeriodRefusal(err error) bool {
	return errors.Is(err, domain.ErrPeriodMissing) || errors.Is(err, domain.ErrPeriodInverted) ||
		errors.Is(err, errPeriodFormat)
}

// metricFromQuery reads the drill-down selector of GET /api/v1/incoming-documents.
//
// `from`/`to` WITHOUT `metric=arrived` IS REFUSED, not ignored: a list that silently dropped the
// period would be read as the period's rows.
func metricFromQuery(q url.Values, now time.Time) (docstore.IncomingMetricFilter, error) {
	hasPeriod := q.Get("from") != "" || q.Get("to") != ""
	s := q.Get("metric")
	if s == "" {
		if hasPeriod {
			return docstore.IncomingMetricFilter{}, errPeriodNoMetric
		}
		return docstore.IncomingMetricFilter{}, nil
	}
	m, ok := domain.ParseIncomingMetric(s)
	if !ok {
		return docstore.IncomingMetricFilter{}, errMetricUnknown
	}
	switch m {
	case domain.MetricArrived:
		_, _, w, err := parsePeriod(q)
		if err != nil {
			return docstore.IncomingMetricFilter{}, err
		}
		return docstore.IncomingMetricFilter{Metric: m, Window: w}, nil
	case domain.MetricOverdue:
		if hasPeriod {
			return docstore.IncomingMetricFilter{}, errPeriodNoMetric
		}
		return docstore.IncomingMetricFilter{Metric: m, Now: now}, nil
	default: // MetricOpen — a stock figure: no period, no instant
		if hasPeriod {
			return docstore.IncomingMetricFilter{}, errPeriodNoMetric
		}
		return docstore.IncomingMetricFilter{Metric: m}, nil
	}
}

// IncomingDocumentSummary serves GET /api/v1/incoming-document-summary.
func (h *Handler) IncomingDocumentSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	from, to, window, err := parsePeriod(q)
	if err != nil {
		if isPeriodRefusal(err) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
			return
		}
		h.d.Log.Error("tổng quan văn bản đến: không dựng được kỳ",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	now := h.now()
	sum, err := h.d.IncomingSummary.CountIncomingSummary(ctx, window, now)
	if err != nil {
		h.d.Log.Error("tổng quan văn bản đến: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, incomingSummaryOut{
		From: lucRa(from), To: lucRa(to), AsOf: lucRa(now),
		Arrived: sum.Arrived, Open: sum.Open, Overdue: sum.Overdue,
	})
}

// IncomingDocumentOverdueQueue serves GET /api/v1/incoming-document-overdue-queue.
//
// `limit` DEFAULTS TO AND IS CAPPED AT 10 — §5's "tối đa 10 mục". Above the cap is served the cap, not
// refused (skills/rest-api-design §5 #2); below 1 or not a number is 400.
func (h *Handler) IncomingDocumentOverdueQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	limit := domain.OverdueQueueMax
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", errQueueLimit.Error(), "")
			return
		}
		limit = min(n, domain.OverdueQueueMax)
	}

	now := h.now()
	items, err := h.d.OverdueQueue.OverdueQueue(ctx, now, limit)
	if err != nil {
		if errors.Is(err, app.ErrWorkingCalendarUnavailable) {
			// 503: the register is fine, the calendar that decides `critical` is not reachable. A queue
			// with every row `critical: false` would be the wrong answer, not a degraded one.
			h.d.Log.Warn("hàng đợi quá hạn văn bản đến: không hỏi được lịch làm việc",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusServiceUnavailable, "working_calendar_unavailable",
				"Chưa xác định được mức khẩn vì không đọc được lịch làm việc của xã. Vui lòng thử lại sau.", "")
			return
		}
		h.d.Log.Error("hàng đợi quá hạn văn bản đến: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	out := overdueQueueOut{Items: make([]overdueQueueItemOut, 0, len(items)), AsOf: lucRa(now)}
	for _, it := range items {
		v := it.Document
		out.Items = append(out.Items, overdueQueueItemOut{
			Kind:        overdueQueueKind,
			ID:          v.ID,
			Code:        domain.MaVanBanDen(v.Nam, v.SoVaoSo),
			DueAt:       lucRa(v.HanXuLyXong),
			Critical:    it.Critical,
			HoldingUnit: v.BoPhanDangGiu,
		})
	}
	vietJSON(w, http.StatusOK, out)
}
