package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the dates the commune works although the week says otherwise — the Prime
// Minister's annual swap days (migration 0006).
//
// ⚠ THE PATH IS NOT REGISTERED YET — see the head of lich_lam_viec.go. The name of this URL
// resource has no row in kb/00-foundation/ubiquitous-language.md and is being asked, not guessed
// (ADR 0011).
//
// THERE IS NO WRITE ROUTE: who may edit a commune's calendar has not been asked.

// caLamBuRa is one swap-day working session as it leaves the API.
//
// ONE ROW IS A SESSION, NOT A DAY, exactly as in the weekly calendar: a swap day with a lunch
// break is two rows with a gap between them. That is why this object carries both a date AND a
// pair of times, while a holiday carries only a date — a swap day has to say which hours were
// worked, because the weekday it falls on usually has no sessions at all (migration 0006:245).
//
// NOTHING HERE IS PERSONAL DATA (rule 3).
type caLamBuRa struct {
	ID string `json:"id"` // ULID — what a later edit would reference

	// Date is the calendar date as `YYYY-MM-DD` — see ngayNghiLeRa.Date for why it is not a
	// timestamp.
	Date string `json:"date"`

	// Start and End are wall-clock times as HH:MM:SS — same shape and same reasoning as
	// caLamViecRa.Start, so one client formatter serves both routes.
	Start string `json:"start"`
	End   string `json:"end"`

	// Name is the announcement this row implements — "Làm bù nghỉ Tết theo Thông báo số …". An
	// inspection asking why a deadline ran through a Saturday reads exactly this string, which is
	// why it leaves the service rather than staying internal.
	Name string `json:"name"`
}

// vanDeLamBuRa is one problem found in the swap days THIS RESPONSE CARRIES: two sessions on one
// date that overlap, so those hours are counted twice.
//
// THE SAME DERIVED-ON-READ DISCIPLINE AS vanDeLichRa, and the same absent constraint underneath:
// `ngay_lam_bu` carries UNIQUE (tenant_id, ngay, bat_dau), which stops two sessions STARTING at the
// same minute and nothing more (migration 0006:283).
//
// NO `empty` KIND HERE, AND THAT ABSENCE IS THE DESIGN. A year with no swap day at all is ordinary
// — most years, most communes — while a weekly calendar with no session means the commune has no
// working hours. Reporting "empty" here would train a client to treat the two as the same state.
type vanDeLamBuRa struct {
	// Kind is the same stable identifier the weekly calendar publishes for the same defect, so a
	// client writes one branch: `overlapping_sessions`.
	Kind string `json:"kind"`

	// Date is the day the two sessions sit on, `YYYY-MM-DD`. Never null here — an overlap always
	// belongs to a date, unlike the weekly calendar's `empty_calendar`, which belongs to no
	// weekday.
	Date string `json:"date"`

	// SessionIDs names the two rows involved. Ids and not times: the times are on `items` in this
	// same response (rule 9).
	SessionIDs []string `json:"session_ids"`

	// Message is the Vietnamese sentence the screen shows. It names no person (rule 3).
	Message string `json:"message"`
}

// danhSachCaLamBuRa wraps the list in an OBJECT rather than a bare JSON array — same reasoning as
// the two routes beside it, and the same absence of `next_cursor` / `has_more`: this route returns
// the whole year or it fails.
type danhSachCaLamBuRa struct {
	Items    []caLamBuRa    `json:"items"`
	Problems []vanDeLamBuRa `json:"problems"`
}

func caLamBuRaNgoai(c domain.CaLamBu) caLamBuRa {
	return caLamBuRa{
		ID:    c.ID,
		Date:  c.Ngay,
		Start: c.BatDau.Chuoi(),
		End:   c.KetThuc.Chuoi(),
		Name:  c.Ten,
	}
}

func vanDeLamBuRaNgoai(v domain.VanDeLamBu) vanDeLamBuRa {
	ca := v.CaID
	if ca == nil {
		// [] and never null, same as every other list on this contract.
		ca = []string{}
	}
	return vanDeLamBuRa{
		Kind:       string(domain.VanDeCaChongNhau),
		Date:       v.Ngay,
		SessionIDs: ca,
		Message: "Hai ca làm bù ngày " + v.Ngay +
			" chồng giờ nhau — số giờ trong khoảng chồng bị tính hai lần.",
	}
}

// DanhSachCaLamBu serves the commune's swap-day sessions for one year.
//
// NO AUDIT ENTRY: not personal data, not a cross-commune read (rule 6, invariant 7).
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachCaLamBu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The same year parser as the holiday route, deliberately shared — see docNamTruyVan. Two
	// screens showing "the calendar for 2026" must mean one window.
	nam, ok := docNamTruyVan(w, r)
	if !ok {
		return
	}

	ds, err := h.d.NgayLamBu.TheoNam(ctx, nam)
	if err != nil {
		var xungDot *domain.LoiNgayVuaNghiVuaLamBu
		if errors.As(err, &xungDot) {
			// REFUSED ON THIS SIDE TOO, and that is the point of refusing at all: a conflict
			// answered only by the holiday route would leave this screen looking healthy, and the
			// commune would fix nothing. Neither table gets to be the winner.
			h.traLoiXungDotLich(w, r, xungDot)
			return
		}
		if errors.Is(err, idstore.ErrQuaNhieuNgayLamBu) {
			// REFUSED, NOT TRUNCATED. A silently short list of swap days counts a day the office
			// WAS open as closed, which lengthens every deadline crossing it — the commune looks
			// slower than it was, on the days its backlog is largest.
			h.d.Log.Error("danh sách ngày làm bù vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "nam", nam,
				"tran", idstore.TranNgayLamBuMotNam)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("ngày làm bù: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "nam", nam, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice on BOTH lists: `items` and `problems` must marshal as
	// [] and never as null.
	//
	// THE ORDER IS THE STORE'S — by date, then by start time — and is not touched here.
	ra := danhSachCaLamBuRa{
		Items:    make([]caLamBuRa, 0, len(ds)),
		Problems: make([]vanDeLamBuRa, 0, 1),
	}
	for _, mot := range ds {
		ra.Items = append(ra.Items, caLamBuRaNgoai(mot))
	}
	// Computed from the rows this response carries, in this same handler, so the two halves can
	// never describe two different instants. The rule itself lives in the domain package: it is a
	// property of a working calendar, not of HTTP.
	for _, vd := range domain.CaLamBuChongNhau(ds) {
		ra.Problems = append(ra.Problems, vanDeLamBuRaNgoai(vd))
	}
	vietJSON(w, http.StatusOK, ra)
}
