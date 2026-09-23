package http

// The STAFF READ surface of the meeting-minutes register (docs/ui-ux/04-bien-ban-hop.md §2, §5).
//
//	GET /api/v1/meetings   task.read
//
// # THE URL NOUN IS `meetings`, AND IT IS NOT IN THE MAPPING TABLE YET
//
// ADR 0011 puts URL segments in ENGLISH and says a new resource name is looked up in
// kb/00-foundation/ubiquitous-language.md rather than translated on the spot. THAT TABLE HAS NO ROW
// FOR `bien_ban_hop` — so this name is not read from it, and it is not invented here either:
// `meetings` (with `conclusions` nested inside it) is what the RUNNING sibling implementation
// serves, `apps/api/app/modules/tasks/router.py:45`, read under the project owner's instruction of
// 2026-09-23 to consult that repository where this one has no answer. Reported as a finding so the
// row is added by the session that owns that file (rule 9, invariant 2) — not written from here.
//
// `bien-ban` AND `ket-luan` ARE BLOCKED SEGMENTS in hooks/rest_api_guard.py, which is the same
// decision expressed as a gate: the specification's own §6 sketch (`/api/bien-ban`) could not ship.
//
// # `task.read` AND NOT A KEY OF ITS OWN
//
// The `quyen` table has no `meeting.*` key (service-identity/migrations/0001_init.sql:273-305), and
// NO KEY IS INVENTED HERE (rule 5, invariant 3c): a key no migration seeds is a right no
// administrator can grant, so the route would answer 403 to every account forever while the tests
// stayed green. `task.read` ("Xem nhiệm vụ", :304) is the honest fit rather than the nearest one —
// this screen lives at `/nhiem-vu/bien-ban` inside the task module, everything it shows is either
// the origin of a task or a count OF tasks, and anybody who may not read the register has no use
// for a page whose badges are made of it.
//
// ⚠ WHETHER A COMMUNE WANTS TO SEPARATE "may read meeting minutes" FROM "may read tasks" IS A
// QUESTION FOR THE CUSTOMER (open question #27), not for this file. Splitting it later is a seeded
// key plus one literal here.
//
// # THERE IS NO WRITE ROUTE IN THIS FILE, AND THAT IS THIS PASS'S BOUNDARY
//
// Recording minutes, adding a conclusion and SPLITTING A CONCLUSION INTO A TASK (§3) are the next
// pass. The third one is additionally blocked on what already blocks POST /api/v1/tasks — creating
// a task FIXES a deadline counted in working hours and identity publishes no contract that returns
// them (ADR 0029 §118) — and §3 makes that sharper rather than softer, because it wants the deadline
// GUESSED from a date inside the sentence ("báo cáo trước ngày 20/8").

import (
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// bienBanRa is one meeting's minutes as they leave the API to a member of staff.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017).
//
// THE MINUTES BODY, THE ATTENDEE LIST AND THE ATTACHMENTS ARE NOT ON THIS RESPONSE. §2's card does
// not draw them, and the full text of every meeting on a page would dwarf everything else on the
// wire. A client must not read their absence as "this meeting has none" — there is no field, and
// the detail route that would carry them is a later pass.
type bienBanRa struct {
	// ID is the internal id, and it is on the wire for ONE reason: §3's split flow sends the
	// CONCLUSION's id back as `nguon_id`, so ids on this surface are load-bearing rather than
	// decorative. The minutes have no business code of their own — `reference_no` is typed by hand,
	// is optional, and repeats across years (migration 0007).
	ID string `json:"id"`

	Title string `json:"title"`

	// HeldOn is a CALENDAR DAY, `2026-08-05`, NOT an instant. The column is DATE; sending it as an
	// RFC 3339 timestamp would attach a midnight in some time zone to it, and a browser one zone
	// away would render the previous day on a card whose whole content is that date.
	HeldOn string `json:"held_on"`

	// ReferenceNo is `31/BB-UBND`. EMPTY IS ORDINARY — §2 drops the segment from the meta line when
	// it is missing, rather than showing a placeholder.
	ReferenceNo string `json:"reference_no"`
	Location    string `json:"location"`

	// ChairedBy is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id (rule 6, invariant
	// 8). No name is joined in: this service does not own the staff directory (rule 2), and a name is
	// not this response's to hold.
	ChairedBy string `json:"chaired_by"`

	// Conclusions are IN THE ORDER THE CIRCLES ARE DRAWN (by `thu_tu`, fixed in the store's ORDER
	// BY). Empty is a real state, not an error: §7.3 saves minutes with no conclusions.
	//
	// make(…, 0, …) AND NEVER nil — see the note in the handler on `items`.
	Conclusions []ketLuanRa `json:"conclusions"`

	// TaskCount and TaskDoneCount are the card header's `{x}/{y} nhiệm vụ xong` — §7.5: the SUM over
	// every conclusion of this meeting. Derived here from the conclusions, by domain.TienDoNhiemVu,
	// and stored nowhere (migration 0007 says why there is no counter column).
	TaskCount     int `json:"task_count"`
	TaskDoneCount int `json:"task_done_count"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ketLuanRa is one numbered conclusion (§2's `①` rows).
type ketLuanRa struct {
	// ID is what §3 sends back as `nguon_id` when a conclusion is split into a task. It is the whole
	// reason the back-link can be traced afterwards (§1).
	ID string `json:"id"`

	// Ordinal is the number in the circle, from 1. IT IS NOT THE ARRAY INDEX: §7.2 appends and never
	// renumbers, so a removed conclusion leaves a gap the screen must show as it is.
	Ordinal int    `json:"ordinal"`
	Content string `json:"content"`

	// The per-conclusion `{x}/{y} nhiệm vụ đã hoàn thành` of §2. `task_count == 0` is the `Chưa tách
	// thành nhiệm vụ nào` line, which is a DIFFERENT statement from `0/3` — see
	// domain.KetLuanHop.ChuaTachNhiemVu. No boolean is sent for it: the client has both numbers and
	// a second representation of one fact is what rule 9 refuses.
	TaskCount     int `json:"task_count"`
	TaskDoneCount int `json:"task_done_count"`

	CreatedAt time.Time `json:"created_at"`
}

// ngayHopRa formats the meeting day as a calendar date.
//
// ONE FUNCTION, ONE FORMAT. `2006-01-02` is ISO 8601, which is what a JSON contract carries; the
// `5/8/2026` of §2 is a RENDERING and belongs to the screen (ADR 0017). Doing it in two places is
// how one of them ends up with the browser's time zone in it.
func ngayHopRa(t time.Time) string { return t.Format("2006-01-02") }

// bienBanRaNgoai builds the response.
//
// NO MASKING BRANCH AND NO PERMISSION ARGUMENT, unlike the petition register's equivalent: every
// person named on these minutes is a MEMBER OF STAFF, by business code. The day a citizen's name
// appears on this record, this function needs the branch phieuRaNgoai has — and migration 0007 says
// the same thing about the columns.
func bienBanRaNgoai(b domain.BienBanHop) bienBanRa {
	xong, tong := b.TienDoNhiemVu()
	ra := bienBanRa{
		ID:            b.ID,
		Title:         b.TenCuocHop,
		HeldOn:        ngayHopRa(b.NgayHop),
		ReferenceNo:   b.SoHieu,
		Location:      b.DiaDiem,
		ChairedBy:     b.ChuTriMa,
		Conclusions:   make([]ketLuanRa, 0, len(b.KetLuan)),
		TaskCount:     tong,
		TaskDoneCount: xong,
		CreatedBy:     b.NguoiTaoMa,
		CreatedAt:     b.TaoLuc,
	}
	for _, k := range b.KetLuan {
		ra.Conclusions = append(ra.Conclusions, ketLuanRa{
			ID:            k.ID,
			Ordinal:       k.ThuTu,
			Content:       k.NoiDung,
			TaskCount:     k.SoNhiemVu,
			TaskDoneCount: k.SoNhiemVuXong,
			CreatedAt:     k.TaoLuc,
		})
	}
	return ra
}

// DanhSachBienBan serves one page of the commune's meeting minutes. GET /api/v1/meetings
//
// # IT TAKES NO FILTER, AND THAT IS THE SPECIFICATION RATHER THAN AN OMISSION
//
// §2 draws one vertical list of cards, newest first, with no filter bar, no search box and no tabs.
// Accepting a parameter the screen does not offer would be a surface nobody asked for and nobody
// tests; the page cursor is the only thing this route reads from the query string.
//
// # NO AUDIT ENTRY
//
// Rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes. This is
// neither: these minutes carry no citizen personal data, and the query cannot leave the commune the
// request arrived in. An audit ledger that grew a row per screen opened would bury the disclosures
// it exists to make findable.
func (h *Handler) DanhSachBienBan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING HERE READS `tenant_id`
	// FROM THE QUERY STRING — a client naming its own commune is a client granting itself access
	// (rule 1, forbidden #2).
	yc, err := page.Parse(r.URL.Query(), petstore.SapXepBienBan)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	kq, err := h.d.DanhSachBienBan.DanhSach(ctx, yc)
	if err != nil {
		// The wrapped error carries the store failure. IT DOES NOT REACH THE CLIENT — and on this
		// route that is more than a habit: the statement behind it quotes no row, but an error text
		// from a register whose columns hold minutes would be the easiest way for a case to reach a
		// log (rule 3, forbidden #3).
		h.d.Log.Error("danh sách biên bản họp: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// page.Result[T] DIRECTLY — tools/apidoc understands it, so there is no second three-field struct
	// copying it and no way for the two to drift.
	//
	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune that has
	// recorded no meeting, never as null. A newly onboarded commune has exactly that, and a client
	// that has to handle both shapes handles one of them wrong. The same holds for `conclusions` on
	// every card — see bienBanRaNgoai.
	ra := page.Result[bienBanRa]{
		Items:      make([]bienBanRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, b := range kq.Items {
		ra.Items = append(ra.Items, bienBanRaNgoai(b))
	}
	vietJSON(w, http.StatusOK, ra)
}

// QuyenDocBienBan guards the read route. It is `task.read` — the key seeded at
// service-identity/migrations/0001_init.sql:304 — and NOT a `meeting.*` key, which does not exist in
// the `quyen` table and is not invented here (rule 5, invariant 3c). See the note at the top.
//
// It is a constant because the handler's own reasoning refers to it; the ROUTE declaration in
// routes.go spells the key out as a literal, because tools/apidoc refuses anything there that is not
// one.
const QuyenDocBienBan authz.Perm = "task.read"
