package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the commune's ordinary working week. GET /api/v1/working-hours
//
// THE NOUN IS SETTLED (user, 2026-09-20) AND IS NOT DECIDED IN THIS FILE. The name and the
// argument behind it — including what was rejected — are owned by
// kb/00-foundation/ubiquitous-language.md §"Lịch làm việc của xã" (ADR 0011). Not restated here:
// a second copy drifts, and then neither copy can be trusted (rule 9, invariant 2).
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Who may edit a commune's
// working calendar has not been asked; it is the sibling of open question #21. A working calendar
// is the BASIS OF AN ISSUED COMMITMENT — when an inspection asks why a petition received on 30/04
// was due on 05/05, the answer is the calendar as it stood that day (migration 0006:41) — so a
// half-written write path here is heavier than a half-written catalogue write path.

// caLamViecRa is one working session as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): office hours describe when an authority is open, not a
// person. They are printed on the noticeboard in the lobby.
type caLamViecRa struct {
	ID string `json:"id"` // ULID — what a later edit would reference

	// Weekday is ISO 8601: 1 = Monday … 7 = Sunday.
	//
	// THE ISO NUMBER AND NOT THE VIETNAMESE ORDINAL ("thứ Hai" = 2 with Sunday unnumbered). It is
	// what the column holds, what PostgreSQL's EXTRACT(ISODOW …) returns and what JavaScript's
	// `getDay()` does NOT return — a client converting 0-is-Sunday to this without noticing is
	// exactly the off-by-one migration 0006:98 warns about, and this field name is the only place
	// the contract can say so.
	Weekday int `json:"weekday"`

	// Start and End are wall-clock times as HH:MM:SS, NOT instants and NOT durations.
	//
	// FIXED WIDTH, SECONDS ALWAYS PRESENT — one shape on the wire, sortable as text. The reason
	// they are not ISO-8601 timestamps is that the schema deliberately holds no date and no zone:
	// "07:30" is an instruction to a commune's staff, and the instant is produced once, in the
	// deadline function, by combining it with a date in Asia/Ho_Chi_Minh (migration 0006:104).
	Start string `json:"start"`
	End   string `json:"end"`

	// Note is the commune's own label for the session — "Buổi sáng". Never parsed, never matched
	// on; it exists for the person reading the configuration screen.
	Note string `json:"note"`
}

// vanDeLichRa is one problem found in the calendar THIS RESPONSE CARRIES.
//
// ---------------------------------------------------------------------------------------------
// WHY THE READ ROUTE PUBLISHES PROBLEMS AT ALL, AND WHY THEY ARE NOT STORED. Two states of this
// table are invisible to the database and fatal to anything computed from it:
//
//	empty_calendar         a commune with NO session has no working hours, so a deadline counted
//	                       in working hours never arrives. The screen must be able to say "chưa
//	                       cấu hình" — today that is every commune, because migration 0006 seeds
//	                       nothing — and no caller may read the empty list as ordinary hours.
//	                       A default here ("Mon–Fri 08:00–17:00") is a commitment invented by
//	                       software and told to a citizen (rule 10; migration 0006:56).
//	overlapping_sessions   two sessions on one weekday DOUBLE-COUNT those hours. The database
//	                       cannot refuse it: the EXCLUDE constraint needs `btree_gist`, and a
//	                       migration that fails for a missing extension stops the service
//	                       (ADR 0013; migration 0006:109). So the read side SURFACES it rather than
//	                       letting the hours be counted twice in silence.
//
// DERIVED ON EVERY READ, NEVER A COLUMN. Same discipline as rule 10, invariant 3: a stored flag is
// wrong the moment a row changes, and the stale copy is the one that reaches the screen.
//
// THE ROUTE STILL ANSWERS 200. The rows ARE the commune's configuration and the screen that has to
// fix them is the one asking; refusing would leave the only screen that can repair the calendar
// unable to load. Refusal belongs to whatever COMPUTES a deadline — and that is another service,
// reading this over gRPC in a later turn, with `problems` non-empty meaning "refuse".
// ---------------------------------------------------------------------------------------------
type vanDeLichRa struct {
	// Kind is a stable English identifier a client branches on: `empty_calendar` or
	// `overlapping_sessions`. Same split as httpx.Error — `code` is for a machine, `message` is a
	// sentence a person reads.
	Kind string `json:"kind"`

	// Weekday is the ISO weekday the problem sits on, `null` when the problem is about the whole
	// calendar (`empty_calendar`). NULL AND NOT 0: 0 is not an ISO weekday, and a client rendering
	// "thứ 0" would be publishing a day nobody configured.
	Weekday *int `json:"weekday"`

	// SessionIDs names the rows involved — [] for `empty_calendar`, two ids for an overlap. Ids
	// and not times: the times are on `items` in this same response, and a second copy is a second
	// copy to drift (rule 9).
	SessionIDs []string `json:"session_ids"`

	// Message is the Vietnamese sentence the screen shows. It names no person and no personal
	// data (rule 3).
	Message string `json:"message"`
}

// danhSachCaLamViecRa wraps the list in an OBJECT rather than a bare JSON array — same reasoning as
// danhSachBoPhanRa, and the same absence of `next_cursor` / `has_more`: this route returns the
// WHOLE week or it fails. A calendar is only correct whole.
type danhSachCaLamViecRa struct {
	Items    []caLamViecRa `json:"items"`
	Problems []vanDeLichRa `json:"problems"`
}

func caLamViecRaNgoai(c domain.CaLamViec) caLamViecRa {
	return caLamViecRa{
		ID:      c.ID,
		Weekday: c.Thu,
		Start:   c.BatDau.Chuoi(),
		End:     c.KetThuc.Chuoi(),
		Note:    c.GhiChu,
	}
}

// tenThu names an ISO weekday in Vietnamese, for the problem sentences only.
//
// DISPLAY ONLY, AND NOTHING KEYS ON IT. The contract carries the NUMBER; this is prose inside a
// message, so a commune reading "hai ca sáng thứ Ba chồng nhau" does not have to translate an ISO
// index in their head to find the row to fix.
func tenThu(thu int) string {
	switch thu {
	case 1:
		return "thứ Hai"
	case 2:
		return "thứ Ba"
	case 3:
		return "thứ Tư"
	case 4:
		return "thứ Năm"
	case 5:
		return "thứ Sáu"
	case 6:
		return "thứ Bảy"
	case 7:
		return "Chủ nhật"
	default:
		// Unreachable while the CHECK constraint holds (thu BETWEEN 1 AND 7). Saying so beats
		// printing a bare number that reads like a real weekday.
		return "ngày không hợp lệ"
	}
}

func vanDeLichRaNgoai(v domain.VanDeLich) vanDeLichRa {
	ra := vanDeLichRa{
		Kind:       string(v.Loai),
		SessionIDs: v.CaID,
	}
	if ra.SessionIDs == nil {
		// [] and never null: a client that has to handle both handles one of them wrong.
		ra.SessionIDs = []string{}
	}
	switch v.Loai {
	case domain.VanDeLichTrong:
		ra.Message = "Xã chưa cấu hình giờ làm việc. Chưa tính được hạn xử lý nào cho tới khi có ít nhất một ca làm việc."
	case domain.VanDeCaChongNhau:
		thu := v.Thu
		ra.Weekday = &thu
		ra.Message = "Hai ca làm việc " + tenThu(v.Thu) + " chồng giờ nhau — số giờ trong khoảng chồng bị tính hai lần."
	default:
		// A kind was added to the domain and not here. The identifier still travels, so the client
		// is not left guessing what the empty sentence meant.
		ra.Message = "Lịch làm việc có vấn đề chưa được mô tả: " + string(v.Loai)
	}
	return ra
}

// DanhSachCaLamViec serves the commune's working week.
//
// NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data and reading ACROSS
// communes. This is neither — the commune's own office hours, read inside the commune the request
// arrived in.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachCaLamViec(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.LichLamViec.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuCaLamViec) {
			// REFUSED, NOT TRUNCATED. A silently short week is a session missing from the
			// calendar, and every deadline computed afterwards is longer than the commitment the
			// commune actually made. The log names the commune because that is what an operator
			// can act on.
			h.d.Log.Error("lịch làm việc vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranLichLamViec)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("lịch làm việc: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice on BOTH lists: `items` and `problems` must marshal as
	// [] and never as null.
	ra := danhSachCaLamViecRa{
		Items:    make([]caLamViecRa, 0, len(ds)),
		Problems: make([]vanDeLichRa, 0, 2),
	}
	for _, mot := range ds {
		ra.Items = append(ra.Items, caLamViecRaNgoai(mot))
	}
	// THE PROBLEMS ARE COMPUTED FROM THE ROWS THIS RESPONSE CARRIES, in this same handler, so the
	// two halves can never describe two different instants. The rule itself lives in the domain
	// package — it is a property of a working calendar, not of HTTP, and the deadline function
	// will need the same rule over gRPC without re-implementing it.
	for _, vd := range domain.VanDeCuaLich(ds) {
		ra.Problems = append(ra.Problems, vanDeLichRaNgoai(vd))
	}

	// An empty calendar is logged at WARN, once per read, because it is the state in which this
	// commune can be promised nothing: no deadline can be computed until somebody configures it.
	// It is not an error — nothing failed — and it must not be one, or the screen that fixes it
	// could not load.
	if len(ds) == 0 {
		h.d.Log.Warn("xã chưa cấu hình lịch làm việc — chưa tính được hạn xử lý nào",
			"xa", string(tenant.MustFrom(ctx)))
	}
	vietJSON(w, http.StatusOK, ra)
}
