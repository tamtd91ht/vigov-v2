package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The READ routes behind the disbursement tracking screen (docs/ui-ux/06-giai-ngan.md §7, §8).
//
// THERE IS NO WRITE ROUTE IN THIS FILE, and the absence is a decision. "Nhập giải ngân" (§10) is
// an all-or-nothing Excel import, and "Ghi nhận khoản chi" (§8.2) starts a voucher lifecycle whose
// unlock rule needs `budget.confirm`. Both write archival records, so both need the business write
// and its audit entry in ONE transaction (rule 6, invariant 3) — that belongs in internal/app,
// which this service has no use case in yet. A read route written today decides nothing a write
// route will later have to undo.

// duAnRa is one project as the list and the detail screens receive it.
//
// EVERY AMOUNT IS A JSON NUMBER OF ĐỒNG, never a formatted string like "100.000.000 đ": the
// formatting is the screen's job and the locale is the screen's to know. A server sending
// formatted money makes every consumer parse it back before it can add two of them up, which is
// where a đồng goes missing.
//
// INT64 IN JSON IS A KNOWN SHARP EDGE, stated rather than hidden: JavaScript's `number` is a
// float64, exact only up to 2^53 ≈ 9 × 10^15. A commune's whole annual plan is 3,3 × 10^10 (§14),
// five orders of magnitude inside that, so no figure this system produces can lose a đồng on the
// way through a browser. If a national-level total ever came through here that would stop being
// true, and the field would have to become a string — a change to make deliberately, not discover.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a project name, a code, and amounts of public money. The
// two id fields are internal ids of an org unit and a staff member, not names — resolving them to
// people is another route's job under another permission.
type duAnRa struct {
	ID   string `json:"id"`
	Code string `json:"code"` // `ma` — issued once, never reissued
	Year int    `json:"year"` // `nam` — each budget year is its own set of projects (§13 rule 8)

	CategoryID  string `json:"category_id"` // `hang_muc_id`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	PlannedAmount   int64 `json:"planned_amount"`   // `ke_hoach_von_nam`, đồng
	ApprovedAmount  int64 `json:"approved_amount"`  // §9's rule for a blank field already applied
	DisbursedAmount int64 `json:"disbursed_amount"` // DERIVED from the vouchers, stored nowhere

	// RemainingAmount CAN BE NEGATIVE — §13 rule 2 says a ratio above 100% is shown, not blocked.
	// Clamping it would hide an over-disbursement, which is the figure somebody needs to see.
	RemainingAmount int64 `json:"remaining_amount"`

	// DisbursedRatio and DelayScore are in HUNDREDTHS OF A PERCENT, the unit §3 prints: 1033 reads
	// as 10,33%, 3136 as "chậm 31,36 điểm". Integers, so nothing rounds between the server's
	// comparison and the screen's rendering.
	//
	// BOTH ARE NULL WHEN THE PROJECT HAS NO ALLOCATION, and that is the contract: 0% and "not
	// applicable" are different statements, and a screen printing "0%" for a project nobody has
	// allocated money to reports it as the commune's worst performer. *int64 forces the client to
	// decide what to show rather than receiving a plausible number by accident.
	DisbursedRatio *int64 `json:"disbursed_ratio"`
	DelayScore     *int64 `json:"delay_score"`

	// IsDelayed is DERIVED on every read from the clock and the threshold (§3), never stored. Same
	// shape rule 10 requires of `overdue`: a column a job maintains is wrong the moment the job is
	// late, and the stale value is the one that reaches the report.
	IsDelayed bool `json:"is_delayed"`

	OrgUnitID      string `json:"org_unit_id,omitempty"`
	AssigneeID     string `json:"assignee_id,omitempty"`
	StartDate      string `json:"start_date,omitempty"`
	CompletionDate string `json:"completion_date,omitempty"`

	// DisbursementDeadline is the date THIS YEAR'S MONEY must be disbursed by. §9 is explicit that
	// it is not the completion date: works finished in March may still have to be disbursed before
	// 31/12. Two different dates, deliberately two different fields.
	DisbursementDeadline string `json:"disbursement_deadline"`

	// DelayThreshold is the threshold `is_delayed` was decided against, in hundredths of a percent,
	// and DelayThresholdSource says WHO CHOSE IT — `xa` or `mac-dinh`.
	//
	// THE SECOND FIELD IS THE POINT AND THE FIRST ONE ALONE WOULD BE A LIE BY OMISSION. Migration
	// 0005 gave the threshold a home per commune and per budget year (§13 rule 5), and 0004's header
	// had already named the failure that follows a table nobody writes: the API "reads as 'already
	// configurable' while every commune silently gets the default". A number with no provenance
	// cannot tell a commune that has chosen 10 points from one that has chosen nothing — and only
	// the second needs to be told. `mac-dinh` is the honest answer for every commune today.
	//
	// THE LIST ROUTE ALSO CARRIES THESE TWO AT THE TOP LEVEL, and that is not two sources for one
	// fact (rule 9): the handler reads the threshold ONCE per request and fans the same value out,
	// so nothing inside one response can disagree with itself. The top-level pair is kept because a
	// client already reads it (`delay_threshold`); the per-project pair exists because the DETAIL
	// route returns a bare duAnRa and has nowhere else to put it — apidoc refuses embedded structs,
	// so wrapping that reply would rename a field web-admin already builds against.
	DelayThreshold       int64  `json:"delay_threshold"`
	DelayThresholdSource string `json:"delay_threshold_source"`
}

type danhSachDuAnRa struct {
	Items []duAnRa `json:"items"`

	// Year is echoed back because the client asked for one explicitly, and a reply that did not say
	// which year it carries is indistinguishable from a reply about another one.
	Year int `json:"year"`

	// DelayThreshold is the threshold the server actually applied, in hundredths of a percent.
	// SENT BACK RATHER THAN ASSUMED BY THE CLIENT: it is per commune and per budget year
	// (`cau_hinh_giai_ngan`, migration 0005), and a client holding its own copy would keep labelling
	// projects by the old figure with nothing saying so.
	DelayThreshold int64 `json:"delay_threshold"`

	// DelayThresholdSource is `xa` when the commune set this figure for this budget year, and
	// `mac-dinh` when nobody has and the software's 10 points (§13 rule 5) are in force.
	//
	// "TOMORROW IT IS PER COMMUNE" IS NOW TODAY, and this field is what stops that from being
	// invisible. No screen writes `cau_hinh_giai_ngan` yet — that is the web's work and needs a
	// permission decision — so every commune is on `mac-dinh`, and the honest thing is to say so
	// rather than to present the vendor's number as the commune's own choice.
	DelayThresholdSource string `json:"delay_threshold_source"`
}

func ngayRa(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// duAnRaNgoai converts one project, applying every derived rule in ONE place.
//
// `nay` IS PASSED IN RATHER THAN READ FROM time.Now() HERE: the delay score depends on the clock,
// and a function that reads the clock itself cannot be tested at a chosen moment — which would
// leave the arithmetic deciding "chậm 31,36 điểm" untested at exactly the dates that matter, the
// first and last days of a budget year.
// `nguong` IS THE WHOLE domain.NguongCanhBaoCham AND NOT A BARE domain.PhanVan, and the type change
// is the fix rather than a detail. The bare figure made "the commune chose 10 points" and "nobody
// has chosen anything" the same value, so no caller COULD report the difference; carrying the
// provenance means every caller has to handle it and the API cannot quietly drop it.
func duAnRaNgoai(t domain.TienDoDuAn, nay time.Time, nguong domain.NguongCanhBaoCham) duAnRa {
	ra := duAnRa{
		ID:                   t.DuAn.ID,
		Code:                 t.DuAn.Ma,
		Year:                 t.DuAn.Nam,
		CategoryID:           t.DuAn.HangMucID,
		Name:                 t.DuAn.Ten,
		Description:          t.DuAn.MoTa,
		PlannedAmount:        int64(t.DuAn.KeHoachVonNam),
		ApprovedAmount:       int64(t.DuAn.TongMucHieuLuc()),
		DisbursedAmount:      int64(t.DaGiaiNgan),
		RemainingAmount:      int64(t.ConPhaiGiaiNgan()),
		IsDelayed:            domain.LaCham(t, nay, nguong.Gia),
		DelayThreshold:       int64(nguong.Gia),
		DelayThresholdSource: string(nguong.Nguon),
		OrgUnitID:            t.DuAn.DonViThucHienID,
		AssigneeID:           t.DuAn.CanBoPhuTrachID,
		StartDate:            ngayRa(t.DuAn.NgayKhoiCong),
		CompletionDate:       ngayRa(t.DuAn.NgayHoanThanh),
		DisbursementDeadline: ngayRa(t.DuAn.ThoiHanGiaiNgan),
	}
	if ty, ok := t.TyLeGiaiNgan(); ok {
		v := int64(ty)
		ra.DisbursedRatio = &v
	}
	if diem, ok := domain.DiemCham(t, nay); ok {
		v := int64(diem)
		ra.DelayScore = &v
	}
	return ra
}

// DanhSachDuAn serves one commune's projects for one budget year.
// GET /api/v1/investment-projects?year=2026&category=<id>
//
// NO AUDIT ENTRY. Rule 6, invariant 7 audits reading FULL personal data and reading ACROSS
// communes; this is neither — public money inside the commune the request arrived in, read by a
// member of staff holding budget.read. An entry per list refresh would bury the entries that carry
// legal weight under thousands that carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachDuAn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The commune is fixed by httpx.TenantMiddleware from Host and reaches the store through the
	// context; the two parameters read here select a budget year and filter by category, INSIDE
	// that commune. Nothing below reads tenant_id from the query string (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// THE BUDGET YEAR IS REQUIRED AND IS NOT DEFAULTED TO THE CURRENT ONE. A default here decides
	// which money gets reported, and decides it invisibly: a client that forgot the parameter on
	// 02/01 would show last year's plan under this year's heading, with every figure on the page
	// internally consistent and wrong (§13 rule 8).
	//
	// `year` IS NOT A SCOPE and carries no tenant_id — see the block above. Validated BEFORE the
	// store is touched: a rejected request must run no statement at all.
	nam, err := strconv.Atoi(thamSo.Get("year"))
	if err != nil || nam < 2000 || nam > 2100 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument",
			"Thiếu hoặc sai năm ngân sách. Ví dụ: ?year=2026", "")
		return
	}

	ds, err := h.d.DuAn.DanhSach(ctx, fistore.LocDuAn{
		Nam: nam,
		// `category` filters WITHIN the commune; tenant_id still comes only from the context, and
		// the store binds it from there (rule 1, invariant 5).
		HangMucID: thamSo.Get("category"),
	})
	if err != nil {
		if errors.Is(err, fistore.ErrQuaNhieuDuAn) {
			// REFUSED, NOT TRUNCATED: this list is totalled on the screen, so a short list is a
			// total that is simply too small and looks entirely normal.
			h.d.Log.Error("danh sách dự án vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "nam", nam, "tran", fistore.TranDuAnMotNam)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh sách dự án: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "nam", nam, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE THRESHOLD IS THE COMMUNE'S, READ FOR THIS BUDGET YEAR, and the reply says both the figure
	// and who chose it. It used to be domain.NguongCanhBaoChamMacDinh — a constant in the vendor's
	// source deciding whether a commune's KPI card reads "29 dự án chậm" or "0" (rule 1, invariant
	// 10 read backwards, open question #31). Migration 0005 gave it a home; this is the read.
	//
	// READ ONCE PER REQUEST AND FANNED OUT, not read per project: one value for one budget year, so
	// nothing inside one response can disagree with itself.
	//
	// A FAILURE HERE IS A 500 AND NOT A FALL BACK TO THE DEFAULT. Falling back would answer with a
	// plausible number and label it `mac-dinh`, which is indistinguishable from the ordinary case —
	// so a commune whose stored threshold is unreadable or out of range would be told it had never
	// set one. Fail closed: refuse, and name the commune in the log.
	nguong, err := h.d.Nguong.NguongCanhBaoCham(ctx, nam)
	if err != nil {
		h.d.Log.Error("ngưỡng cảnh báo chậm: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "nam", nam, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	nay := h.nay()

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null. A client
	// that has to handle both handles one of them wrong — and an empty year is the ordinary state
	// here, since no commune has a project until somebody enters one.
	ra := danhSachDuAnRa{
		Items:                make([]duAnRa, 0, len(ds)),
		Year:                 nam,
		DelayThreshold:       int64(nguong.Gia),
		DelayThresholdSource: string(nguong.Nguon),
	}
	for _, mot := range ds {
		ra.Items = append(ra.Items, duAnRaNgoai(mot, nay, nguong))
	}
	vietJSON(w, http.StatusOK, ra)
}

// ChiTietDuAn serves one project of the commune in the context.
// GET /api/v1/investment-projects/{id}
//
// A PROJECT OF ANOTHER COMMUNE ANSWERS 404, THE SAME AS ONE THAT DOES NOT EXIST — and it does so
// because the store cannot reach it at all, not because this handler compares anything. Two
// different answers would tell a caller that a record exists inside an authority they have no
// business knowing about (rule 4, forbidden #2, applied between communes).
func (h *Handler) ChiTietDuAn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument", "Thiếu mã dự án.", "")
		return
	}

	mot, err := h.d.DuAn.ChiTiet(ctx, id)
	if err != nil {
		if errors.Is(err, fistore.ErrKhongThayDuAn) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy dự án.", "")
			return
		}
		h.d.Log.Error("chi tiết dự án: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE YEAR COMES FROM THE PROJECT, NOT FROM A PARAMETER, and that is the only correct source
	// here: §13 rule 8 makes each budget year its own set of projects, and the threshold is stored
	// per year. Reading the current calendar year instead would judge a 2025 project against a
	// figure the commune chose for 2026 — silently re-flagging closed work.
	nguong, err := h.d.Nguong.NguongCanhBaoCham(ctx, mot.DuAn.Nam)
	if err != nil {
		h.d.Log.Error("ngưỡng cảnh báo chậm: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "nam", mot.DuAn.Nam, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	vietJSON(w, http.StatusOK, duAnRaNgoai(mot, h.nay(), nguong))
}
