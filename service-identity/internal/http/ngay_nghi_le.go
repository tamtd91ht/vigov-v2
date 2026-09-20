package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the dates the commune does not work. GET /api/v1/public-holidays?year=
//
// THE NOUN IS SETTLED (user, 2026-09-20) AND IS NOT DECIDED IN THIS FILE, including the objection
// that was raised against it and answered. The name and the whole argument behind it are owned by
// kb/00-foundation/ubiquitous-language.md §"Lịch làm việc của xã" (ADR 0011); nothing of it is
// restated here, because a second copy drifts and then neither copy can be trusted (rule 9).
//
// THERE IS NO WRITE ROUTE: who may edit a commune's calendar has not been asked.

// NamNhoNhat and NamLonNhat bound the `year` this route accepts.
//
// AN INPUT-SANITY BOUND, NOT A BUSINESS RULE, and it is stated so nobody reads it as one. The year
// reaches `make_date($2, 1, 1)`, which happily accepts 99999 and would then scan a window no index
// can help with. The range covers every year this system can hold a calendar for, several times
// over; a request outside it is a client bug and is answered as one.
const (
	NamNhoNhat = 2000
	NamLonNhat = 2100
)

// docNamTruyVan reads the mandatory `year` query parameter. It writes the 400 itself and returns
// false when the parameter is missing, unreadable or out of range.
//
// ---------------------------------------------------------------------------------------------
// THE PARAMETER IS MANDATORY, AND "THIS YEAR" IS NOT A DEFAULT THIS CODE GETS TO PICK. Defaulting
// looks harmless and is not: on 31/12 a screen asking for "the holidays" would silently switch
// year at midnight, and a client that never sends the parameter would be reading a different
// window every January without a line of code changing. Refusing names the gap at the caller,
// where it can be fixed, which is the same reading as every other fail-closed decision here.
//
// WHY BY YEAR AT ALL: `ngay_nghi_le` and `ngay_lam_bu` grow with TIME, not with how the commune is
// organised, so "all of them" is a list without an upper bound — see TranNgayNghiLeMotNam. A year
// is also the unit the configuration screen shows and the unit the Prime Minister's annual
// announcement arrives in.
// ---------------------------------------------------------------------------------------------
//
// Shared with the swap-day route in ngay_lam_bu.go. It lives here because this was the first route
// that needed it; the two must stay one function, or the two screens end up accepting two
// different notions of a year.
func docNamTruyVan(w http.ResponseWriter, r *http.Request) (int, bool) {
	// THE COMMUNE IS NOT IN THIS QUERY STRING AND NEVER MAY BE. It is fixed by
	// httpx.TenantMiddleware from Host and travels in the context; the only thing read here is a
	// year. A client naming its own commune is a client granting itself access (rule 1,
	// forbidden #2).
	//
	// THE RAW SLICE AND NOT url.Values.Get, WHICH IS WHY THIS IS FOUR LINES INSTEAD OF ONE. `Get`
	// returns the FIRST value and drops the rest, so `?year=2026&year=2027` silently reads one of
	// two years the client asked for — a winner picked in silence, which is the very family of
	// failure this calendar refuses elsewhere. Exactly one value, or refuse.
	//
	// Nothing below reads tenant_id from the client, and nothing added here may.
	gt := r.URL.Query()["year"]
	if len(gt) != 1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query",
			"Cần đúng một tham số year — hãy nêu rõ năm cần xem, ví dụ year=2026.", "")
		return 0, false
	}
	nam, err := strconv.Atoi(strings.TrimSpace(gt[0]))
	// ONE ANSWER FOR "not a number" AND "out of range": both are the same mistake from the caller's
	// side, and telling them apart tells a prober nothing useful and a client nothing it can act
	// on differently. The input is NOT echoed into the response (rule 3, forbidden #3) — the
	// message names the parameter, never what was sent.
	if err != nil || nam < NamNhoNhat || nam > NamLonNhat {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query",
			"Tham số year không hợp lệ — hãy nêu một năm trong khoảng "+
				strconv.Itoa(NamNhoNhat)+"–"+strconv.Itoa(NamLonNhat)+".", "")
		return 0, false
	}
	return nam, true
}

// traLoiXungDotLich answers a calendar that contradicts itself: a date recorded BOTH as a holiday
// and as a swap working day.
//
// 409 AND NOT 500, AND NOT A QUIETLY RESOLVED WINNER. Nothing failed — the store did exactly what
// it was asked — and nothing about the request is wrong either, so neither 500 nor 400 describes
// it. What is wrong is the commune's own configuration, and 409 is the status that says "the state
// on the server conflicts": the screen can show the offending dates and the person can fix one of
// the two rows. Picking a winner instead would make one of two visible configuration rows do
// nothing, and nobody would ever see which (migration 0006:254).
//
// THE MESSAGE NAMES THE DATES. A date says when an authority is open; it is not personal data
// (rule 3), and a refusal that does not say which day to look at is a refusal nobody can act on.
// The dates keep the `YYYY-MM-DD` spelling the `date` field of this contract uses, rather than
// being reformatted for prose, so the string in the message and the string in the data are the
// same string.
//
// SHARED BY BOTH DATE ROUTES on purpose: refusing on one side only would leave the other screen
// looking healthy, and the commune would fix nothing.
func (h *Handler) traLoiXungDotLich(w http.ResponseWriter, r *http.Request, e *domain.LoiNgayVuaNghiVuaLamBu) {
	ngay := e.Ngay
	them := ""
	if len(ngay) > domain.SoNgayXungDotNeuTen {
		them = " và " + strconv.Itoa(len(ngay)-domain.SoNgayXungDotNeuTen) + " ngày khác"
		ngay = ngay[:domain.SoNgayXungDotNeuTen]
	}
	h.d.Log.Error("lịch của xã mâu thuẫn — ngày vừa nghỉ lễ vừa làm bù, TỪ CHỐI thay vì chọn bên",
		"xa", string(tenant.MustFrom(r.Context())), "so_ngay", len(e.Ngay))
	httpx.WriteError(w, http.StatusConflict, "calendar_conflict",
		"Cấu hình lịch của xã đang mâu thuẫn: ngày "+strings.Join(ngay, ", ")+them+
			" vừa được khai là ngày nghỉ lễ vừa được khai là ngày làm bù. "+
			"Hệ thống không tự chọn bên nào — vui lòng sửa một trong hai.", "")
}

// ngayNghiLeRa is one closure date as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): the row says when the authority is closed.
type ngayNghiLeRa struct {
	ID string `json:"id"` // ULID — what a later edit would reference

	// Date is the calendar date as `YYYY-MM-DD` — a DAY, with no instant and no zone.
	//
	// NOT AN ISO-8601 TIMESTAMP, and the difference is a day rather than a formatting taste: a
	// timestamp carries a zone, and "2026-09-02T00:00:00Z" rendered in Asia/Ho_Chi_Minh is the 2nd
	// while the same instant rendered anywhere west of London is the 1st. A holiday that shifts by
	// one day is a deadline counted through a day the office was shut.
	Date string `json:"date"`

	// Name is what a person reads: "Quốc khánh", "Lễ hội đình làng". `name` and not `label`: the
	// column is `ten` and a holiday is a thing with a name, not a label put on a code — the
	// boundary kb/00-foundation/ubiquitous-language.md draws at the SCHEMA.
	Name string `json:"name"`
}

// danhSachNgayNghiLeRa wraps the list in an OBJECT rather than a bare JSON array.
//
// NO `year` FIELD ECHOING THE REQUEST: the caller chose the window and already knows it, and a
// second copy of one fact is a second copy to drift (rule 9).
type danhSachNgayNghiLeRa struct {
	Items []ngayNghiLeRa `json:"items"`
}

func ngayNghiLeRaNgoai(n domain.NgayNghiLe) ngayNghiLeRa {
	return ngayNghiLeRa{ID: n.ID, Date: n.Ngay, Name: n.Ten}
}

// DanhSachNgayNghiLe serves the commune's holidays for one year.
//
// NO AUDIT ENTRY: not personal data, not a cross-commune read (rule 6, invariant 7).
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachNgayNghiLe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nam, ok := docNamTruyVan(w, r)
	if !ok {
		return
	}

	ds, err := h.d.NgayNghiLe.TheoNam(ctx, nam)
	if err != nil {
		var xungDot *domain.LoiNgayVuaNghiVuaLamBu
		if errors.As(err, &xungDot) {
			h.traLoiXungDotLich(w, r, xungDot)
			return
		}
		if errors.Is(err, idstore.ErrQuaNhieuNgayNghiLe) {
			// REFUSED, NOT TRUNCATED. A silently short list of holidays counts a closed day as a
			// working day and shortens every deadline crossing it.
			h.d.Log.Error("danh sách ngày nghỉ lễ vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "nam", nam,
				"tran", idstore.TranNgayNghiLeMotNam)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("ngày nghỉ lễ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "nam", nam, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] and never as null. An empty
	// year is a legitimate answer here — and today it is the only one, since migration 0006 seeds
	// nothing — but it is NOT the same statement as an empty weekly calendar: a commune with no
	// holidays entered still has working hours.
	//
	// THE ORDER IS THE STORE'S, ascending by date, and is not touched here.
	ra := danhSachNgayNghiLeRa{Items: make([]ngayNghiLeRa, 0, len(ds))}
	for _, mot := range ds {
		ra.Items = append(ra.Items, ngayNghiLeRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
