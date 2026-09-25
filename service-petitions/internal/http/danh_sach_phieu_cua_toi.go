package http

import (
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The CITIZEN list of their own petitions — "Phản ánh của tôi". GET /api/v1/my-citizen-reports
//
// A SIBLING OF phieu_cua_toi.go AND BOUND BY EVERYTHING WRITTEN THERE: same handler type, same Deps,
// same identity helper, same citizen-safe response builder. What this file adds is the page and the
// list-card shape; it adds no new source of identity, commune or field.

// trichNoiDungToiDa bounds the content excerpt on a list card, in RUNES — Vietnamese text is
// multi-byte, and a byte cut would split a character and put an invalid string on the screen.
//
// FIXED IN SOURCE and not a query parameter: a client-chosen length is a client choosing to receive
// the whole text through the list, which is the one thing the excerpt exists not to do.
const trichNoiDungToiDa = 140

// phieuCuaToiTomTatRa is one petition on the citizen's LIST CARD.
//
// A STRICT SUBSET OF phieuCuaToiRa, AND BUILT FROM IT (see tomTatPhieuCuaToi), so no field can
// appear here that the by-code route does not already hand the same citizen. Every field keeps the
// same JSON name and the same null semantics as there, so the Mini App parses one vocabulary.
//
// WHAT IS LEFT OFF, beyond what phieuCuaToiRa already withholds (read that list first):
//
//	reporter_name / reporter_phone   even masked, twenty of them on one screen serve nobody — the
//	                                 card is about the report, and the detail route still shows them
//	address · result · reason · …    detail-screen facts; the card opens the detail
//	channel · anonymous              nothing on the card is decided by them
//
// THE TWO NULLS STILL MEAN OPPOSITE THINGS — see phieuCuaToiRa. They are pointers WITHOUT omitempty
// exactly as there: `null` is an answer ("không áp dụng" / "chưa có"), not an absent field.
type phieuCuaToiTomTatRa struct {
	Code       string `json:"code"`
	Status     string `json:"status"`
	Field      string `json:"field"`
	FieldLabel string `json:"field_label"`

	// ContentExcerpt is the first trichNoiDungToiDa runes of what the citizen themself wrote, with
	// `…` in the last position when it was cut. The detail route carries the whole text.
	ContentExcerpt string `json:"content_excerpt"`

	ClockFrom      time.Time  `json:"clock_from"`
	AcknowledgeDue *time.Time `json:"acknowledge_due"`
	ResolveDue     *time.Time `json:"resolve_due"`
}

// tomTatPhieuCuaToi derives the card FROM the citizen-safe detail response, never from the domain
// record directly. That is what makes "a strict subset of phieuCuaToiRa" a fact about the code: a
// staff-internal field would have to be added to phieuCuaToiRa first, where its list of deliberate
// absences stands in the way.
func tomTatPhieuCuaToi(ra phieuCuaToiRa) phieuCuaToiTomTatRa {
	return phieuCuaToiTomTatRa{
		Code:           ra.Code,
		Status:         ra.Status,
		Field:          ra.Field,
		FieldLabel:     ra.FieldLabel,
		ContentExcerpt: trichNoiDung(ra.Content),
		ClockFrom:      ra.ClockFrom,
		AcknowledgeDue: ra.AcknowledgeDue,
		ResolveDue:     ra.ResolveDue,
	}
}

// trichNoiDung cuts to at most trichNoiDungToiDa runes, ellipsis included.
func trichNoiDung(s string) string {
	r := []rune(s)
	if len(r) <= trichNoiDungToiDa {
		return s
	}
	return strings.TrimRightFunc(string(r[:trichNoiDungToiDa-1]), unicode.IsSpace) + "…"
}

// DanhSachPhieuCuaToi serves one page of the SESSION citizen's own petitions, in the SESSION
// commune, newest first. GET /api/v1/my-citizen-reports
//
// # NOTHING FROM THE REQUEST NAMES A PERSON OR A COMMUNE
//
// The identity is danhTinhTuPhien (the bearer token, via authz.CitizenPrincipal); the commune is
// httpx.XaTuPhien's. The query string is read for four keys only — limit, cursor, order and status —
// and none of them can widen the page: the cursor carries no commune by construction (core/page), and
// the store binds the commune to $1 and the citizen to $2 whatever the cursor says. A citizen id, a
// phone number or a commune id in the query string is ignored because nothing here reads it (rule 4,
// forbidden #1; rule 1, forbidden #2).
//
// # AN EMPTY LIST IS 200 WITH `items: []`
//
// There is no 403 and no 404 on this route. A citizen with no petitions, a citizen whose petitions
// are all in another commune and a citizen with only soft-deleted petitions get the SAME answer, so
// the list reveals nothing about records that are not theirs to see (rule 4, forbidden #2).
//
// # ORDER IS FIXED: NEWEST FIRST
//
// `?order=desc` is accepted as a no-op; any other value is refused 400 rather than silently ignored,
// the same stance locPhieuTuQuery takes on the staff list — a screen that asked for a different
// order and silently got this one would show the citizen a list it misreads.
//
// # NO AUDIT ENTRY, FOR THE SAME REASON AS THE BY-CODE ROUTE
//
// Nothing is written and nothing leaves unmasked: the card carries no reporter field at all. See
// HandlerCongDan.PhieuCuaToi for the condition under which that stops holding.
func (h *HandlerCongDan) DanhSachPhieuCuaToi(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	congDanID, ok := h.danhTinhTuPhien(ctx)
	if !ok {
		// A WIRING FAULT, answered exactly as PhieuCuaToi answers it — see the note there.
		h.d.Log.Error("tuyến công dân chạy mà không có danh tính trong phiên — thiếu authz.CitizenOnly " +
			"hoặc authz.CitizenPrincipal trên chuỗi rìa")
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	thamSo := r.URL.Query()

	if o := thamSo.Get(page.ParamOrder); o != "" && o != string(page.Desc) {
		status, ma, thongBao := page.HTTPError(page.ErrSort)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}
	// Parsed BEFORE the store is touched: a rejected page request runs no statement at all.
	yc, err := page.Parse(thamSo, petstore.SapXepPhieuCuaToi)
	if err != nil {
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	// `status` IS CHECKED AGAINST THE CLOSED NINE (ADR 0027) and an unknown value is refused, never
	// dropped — a dropped filter answers "Đang xử lý" with every petition the citizen has.
	trangThai := ""
	if v := thamSo["status"]; len(v) > 0 {
		trangThai = v[0]
	}
	if trangThai != "" && !domain.TrangThai(trangThai).HopLe() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", errTrangThaiPhieuKhongHopLe.Error(), "")
		return
	}

	kq, err := h.d.Phieu.DanhSachCuaCongDan(ctx, congDanID, trangThai, yc)
	if err != nil {
		// Neither the citizen identifier nor any lookup code is logged (rule 3). The commune is not
		// personal data and is what an operator needs.
		h.d.Log.Error("danh sách phiếu của công dân: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE LABELS ARE READ ONCE FOR THE PAGE, and not at all when no petition on it is classified —
	// the ordinary case for a citizen whose reports are still waiting to be read.
	theoMa := map[string]string{}
	for _, p := range kq.Items {
		if p.LinhVuc == "" {
			continue
		}
		nhan, err := h.d.NhanLinhVuc.DanhSach(ctx)
		if err != nil {
			h.d.Log.Error("đọc nhãn lĩnh vực cho danh sách của công dân: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		for _, n := range nhan {
			theoMa[n.Ma] = n.Nhan
		}
		break
	}

	// make(..., 0, ...): `items` marshals as [] — never null — for a citizen with nothing filed.
	ra := page.Result[phieuCuaToiTomTatRa]{
		Items:      make([]phieuCuaToiTomTatRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, p := range kq.Items {
		ra.Items = append(ra.Items, tomTatPhieuCuaToi(phieuCuaToiRaNgoai(p, theoMa[p.LinhVuc])))
	}
	vietJSON(w, http.StatusOK, ra)
}
