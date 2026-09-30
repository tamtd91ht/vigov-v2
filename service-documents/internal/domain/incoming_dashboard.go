package domain

// The leadership dashboard's view of SỔ VĂN BẢN ĐẾN (docs/ui-ux/01-tong-quan-dieu-hanh.md §4.2, §5).
//
// WHY THIS FILE IS IN domain/: every figure on /tong-quan has a drill-down list behind it (§4, "Mỗi ô
// là một button … mở danh sách … đã lọc sẵn"), and docs/ui-ux/13-bao-cao.md §10 forbids the two from
// disagreeing. The only way the row count of the list can equal the figure is that both are the SAME
// predicate. What each figure MEANS is decided here, once; store/ turns that meaning into one SQL
// fragment that both the count and the list use.

import (
	"errors"
	"time"

	// THE ZONE DATABASE IS EMBEDDED, and the reason is the period below rather than tidiness. Without it
	// time.LoadLocation reads the host's zoneinfo, which a distroless image does not carry — and the
	// failure would either refuse every dashboard load or, worse, tempt somebody into a UTC fallback
	// that moves a period's first seven hours into the previous day.
	_ "time/tzdata"
)

// AdministrativeZoneName is the zone a commune's calendar DATES are read in.
//
// NAMED EXPLICITLY, NEVER time.Local: `ngay_den` is a DATE, and "which dates does this period cover"
// has an answer only in a zone. A pod started with TZ=UTC must count the same documents as one
// started without it. service-identity names the same zone for the same reason
// (service-identity/internal/domain/tien_gio_lam_viec.go:37); this is a second spelling of the value,
// not of the decision, because rule 2 forbids importing that package.
const AdministrativeZoneName = "Asia/Ho_Chi_Minh"

// administrativeZone is loaded once from the embedded database. The error is kept rather than
// panicking at package load, so a broken build answers 500 on the dashboard instead of refusing to
// start the whole register.
var administrativeZone, administrativeZoneErr = time.LoadLocation(AdministrativeZoneName)

// IncomingMetric names one figure of the dashboard's incoming-document block.
//
// ENGLISH VALUES, AND THAT IS A READING OF ADR 0011 STATED RATHER THAN HIDDEN: these values are not a
// business enum stored in any column or archival record — each one is the JSON field name of the
// figure it selects on GET /api/v1/incoming-document-summary, so a client drills down with the same
// word it read the figure under. Nothing about a statute or a clock rides on the word.
type IncomingMetric string

const (
	// MetricArrived — "Đến trong kỳ": `ngay_den`, read as a date, falls in the period.
	MetricArrived IncomingMetric = "arrived"
	// MetricOpen — "Chưa xử lý xong" (the document half): NOT TrangThaiVanBanDen.DaKetThuc().
	MetricOpen IncomingMetric = "open"
	// MetricOverdue — "Quá hạn xử lý": VanBanDen.QuaHan(now).
	MetricOverdue IncomingMetric = "overdue"
)

// ParseIncomingMetric accepts exactly the three values above. Anything else is refused by the caller
// rather than ignored: a dropped selector returns the whole register to a screen that asked for one
// figure's rows.
func ParseIncomingMetric(s string) (IncomingMetric, bool) {
	switch m := IncomingMetric(s); m {
	case MetricArrived, MetricOpen, MetricOverdue:
		return m, true
	default:
		return "", false
	}
}

// IncomingStatuses is every state the incoming register has — the six codes of
// migrations/0004_so_van_ban.sql `van_ban_den_trang_thai_hop_le`.
func IncomingStatuses() []TrangThaiVanBanDen {
	return []TrangThaiVanBanDen{
		VanBanMoiVaoSo, VanBanDaPhanCong, VanBanDangXuLy,
		VanBanDaGiaiQuyet, VanBanChuyenCapTren, VanBanLuuKhongThuLy,
	}
}

// FinishedIncomingStatuses is the set DaKetThuc answers true for, DERIVED FROM DaKetThuc and not
// written out a second time. The SQL `open` predicate is built from this list, so a fourth closing
// state added to DaKetThuc reaches the figure, the drill-down and QuaHan in the same edit.
func FinishedIncomingStatuses() []TrangThaiVanBanDen {
	var out []TrangThaiVanBanDen
	for _, s := range IncomingStatuses() {
		if s.DaKetThuc() {
			out = append(out, s)
		}
	}
	return out
}

// ArrivalWindow is the inclusive range of calendar DATES a period covers, for comparison with
// `ngay_den`. Both ends are midnight UTC carrying the date only — the shape `ngay_den` already has
// when it is scanned (see KiemNgayDen), so nobody compares a date with an instant by accident.
type ArrivalWindow struct {
	FirstDate time.Time
	LastDate  time.Time
}

var (
	// ErrPeriodMissing — `from` or `to` absent. A period figure with a default period is a figure for a
	// period nobody chose.
	ErrPeriodMissing = errors.New("kỳ báo cáo: thiếu `from` hoặc `to`")
	// ErrPeriodInverted — `from` is not strictly before `to`. [from, to) is then empty or backwards.
	ErrPeriodInverted = errors.New("kỳ báo cáo: `from` phải trước `to`")
)

// ArrivalWindowFor turns the half-open instant period [from, to) into the dates it covers, read in
// AdministrativeZoneName.
//
// A DATE IS IN THE PERIOD WHEN ANY PART OF ITS DAY IS. The first date is the date `from` falls on;
// the last is the date of the last instant before `to`. So a period ending exactly at local midnight
// does NOT include the day that begins at that midnight — that is what "half-open" means for a
// dashboard whose next period starts there — while a period ending at 10:00 includes that day.
//
// ⚠ A PERIOD THAT DOES NOT START AND END ON LOCAL MIDNIGHT COUNTS WHOLE DAYS AT BOTH ENDS, because
// `ngay_den` has no time of day to compare with. That is the column's resolution, not a choice here.
func ArrivalWindowFor(from, to time.Time) (ArrivalWindow, error) {
	if from.IsZero() || to.IsZero() {
		return ArrivalWindow{}, ErrPeriodMissing
	}
	if !from.Before(to) {
		return ArrivalWindow{}, ErrPeriodInverted
	}
	if administrativeZoneErr != nil {
		return ArrivalWindow{}, administrativeZoneErr
	}
	f := from.In(administrativeZone)
	t := to.In(administrativeZone)

	first := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, time.UTC)
	last := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	startOfToDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, administrativeZone)
	if t.Equal(startOfToDay) {
		// `to` is exactly local midnight: the day it opens is outside [from, to). time.Date
		// normalises day 0 to the last day of the previous month.
		last = time.Date(t.Year(), t.Month(), t.Day()-1, 0, 0, 0, 0, time.UTC)
	}
	return ArrivalWindow{FirstDate: first, LastDate: last}, nil
}

// IncomingSummary is the three figures, as counts. Nothing else: there is no on-time ratio because
// the register does not store the instant a document was settled (see QuaHan), and there are no
// citizen letters because `don_thu` does not exist in this service yet.
type IncomingSummary struct {
	Arrived int
	Open    int
	Overdue int
}

// OverdueQueueMax is the largest number of items the "CẦN XỬ LÝ NGAY" block shows —
// docs/ui-ux/01-tong-quan-dieu-hanh.md §5, "Danh sách tối đa 10 mục".
const OverdueQueueMax = 10

// CriticalOverdueWorkingHours is how long past its deadline, in WORKING HOURS of the commune, an
// overdue document becomes `critical`.
//
// ⚠ THE FIGURE IS THE TASK'S, NOT THE SPECIFICATION'S: §5 of the dashboard chapter sorts by lateness
// and names no threshold. It is not an SLA (rule 10, forbidden #3 concerns the commitment itself) but
// it IS a judgement about urgency that a commune might want to set; it is a constant until the
// customer says otherwise. It is counted by identity's AdvanceWorkingHours, never by adding a
// duration here (rule 10, forbidden #2).
const CriticalOverdueWorkingHours uint32 = 48
