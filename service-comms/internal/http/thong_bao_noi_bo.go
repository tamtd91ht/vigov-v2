package http

// THE STAFF SURFACE OF THE INTERNAL ANNOUNCEMENT BOOK (`docs/ui-ux/08-thong-bao.md`).
//
//	GET  /api/v1/announcements   announcement.create
//	POST /api/v1/announcements   announcement.create
//
// # THE URL NOUN WAS LOOKED UP, NOT TRANSLATED
//
// `announcements` is the settled mapping for this meaning of `thông báo`
// (kb/00-foundation/ubiquitous-language.md, the row that splits the one Vietnamese word into
// `announcements` · `public-notices` · `notifications`). §7 of the specification sketches
// `/api/thong-bao`, which cannot ship twice over: ADR 0011 puts path segments in English, and
// `thong-bao` is a blocked segment in hooks/rest_api_guard.py.
//
// # THE PERMISSION, AND THE PART OF IT THAT IS A STOP CONDITION
//
// §9.1 names the key for composing and issuing: `announcement.create`. It EXISTS and is seeded —
// service-identity/migrations/0001_init.sql:286, group `THÔNG BÁO`, "Soạn và gửi thông báo" — so
// the write route below is guarded by the key the specification itself names, and no key was
// invented (rule 5, invariant 3c).
//
// ⚠ THE READ ROUTE IS GUARDED BY THE SAME KEY, AND THAT IS A KNOWN GAP RATHER THAN A CHOICE. §2
// offers two filters: `Gửi cho tôi` (announcements I am a recipient of) and `Cả sổ thông báo`
// (everything, "cần quyền xem"). THERE IS NO READ KEY IN THE `quyen` TABLE — `announcement.create`
// is the ONLY key in the `THÔNG BÁO` group, and the table holds 33 keys where the specification's
// own heading counts 43 (open question #27). So:
//
//	SHIPPED      `Cả sổ thông báo`, under `announcement.create`. Whoever may compose and send
//	             announcements may read the book of them — that is a strict UNDER-grant, not an
//	             over-grant, and under-granting is the direction this system fails in.
//	NOT SHIPPED  `Gửi cho tôi`. Every member of staff needs to read the announcements addressed to
//	             THEM, and the only ways to express that today are a key that does not exist
//	             (→ #27, never an INSERT) or authz.AnyAuthenticated, which is rule 5's own first
//	             stop condition. Both are the customer's call. Until one is taken, an ordinary
//	             account gets 403 here — visibly, which is the point.
//
// # WHAT IS ABSENT FROM THIS FILE, so the absence is not read as unfinished work
//
//	no detail route      §7's GET /:id. The list already carries the body and the counters; what a
//	                     detail route would ADD is §4's recipient list and department chips with
//	                     PEOPLE'S NAMES on them, and this service does not own the staff directory
//	                     (rule 2) — that is an identity RPC, not a SQL join.
//	no `phat-hanh`,      the draft/publish split of §7, the withdrawal of §4, and §4's per-person
//	no `go`, no          acknowledgement. The write route below issues in one act; withdrawal and
//	`xac-nhan`/`da-mo`   acknowledgement each change a record staff have already read, and §9.2/§9.5
//	                     describe them but nothing here decides them.
//	no pinned-first      §3 puts pinned announcements at the head of the list. core/page carries ONE
//	  ordering           sort column plus the `id` tie-break, so `ORDER BY ghim DESC, tao_luc DESC`
//	                     is not a cursor this repository can express. `pinned` is on the response and
//	                     the client can raise those cards; approximating it in the cursor would make
//	                     page two repeat and skip rows, silently.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// QuyenThongBao guards both routes. It is `announcement.create`, seeded at
// service-identity/migrations/0001_init.sql:286, and NOT a `announcement.read` key — which does not
// exist in the `quyen` table and is not invented here (rule 5, invariant 3c). See the note at the
// top for what that costs and what it is waiting on.
//
// It is a constant because the reasoning above refers to it; the ROUTE declarations in routes.go
// spell the key out as a LITERAL, because tools/apidoc refuses anything there that is not one.
const QuyenThongBao authz.Perm = "announcement.create"

// thongBaoRa is one announcement as it leaves the API to a member of staff.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017).
//
// NOTHING HERE IS MASKED AND NOTHING HERE NEEDS TO BE (rule 3): every person on this record is a
// member of STAFF, by business code. `body`, however, is free text a colleague typed and a
// commune's announcement can quote a case — it must not be logged, put in a file name, a URL or a
// cache key on the way to a screen.
type thongBaoRa struct {
	// ID is the internal id. It is on the wire because every act §7 lists afterwards — acknowledge,
	// withdraw, mark opened — addresses the announcement by it.
	ID string `json:"id"`

	Title string `json:"title"`

	// Body is the WHOLE text. §2's card truncates it to two lines and §2's right-hand panel renders
	// it in full, both from this one field — there is no detail route in this pass, so a truncated
	// value here would leave that panel showing a cut-off notice with nothing saying so.
	Body string `json:"body"`

	// Status is `nhap` · `da-phat-hanh` · `da-go` — a Vietnamese value without diacritics, which is
	// ADR 0011: only the surrounding contract is English.
	Status string `json:"status"`

	// Pinned is §3's pin. THE LIST IS NOT SORTED BY IT — see the note at the top of this file.
	Pinned bool `json:"pinned"`

	// AckRequired turns on §3's orange chip and makes the two counters below meaningful. A client
	// draws `{x}/{y}` only when it is true; the numbers are sent regardless, because a second
	// representation of "should I draw this" is what rule 9 refuses.
	AckRequired bool `json:"ack_required"`

	// EmailRequested is what the author ASKED FOR; EmailStatus is what happened. They are two facts
	// and the second is `chua-gui` on every row today: this repository has no SMTP adapter, so
	// §3's mail chip never appears. Conflating them would make "not sent yet" indistinguishable
	// from "mail was never wanted".
	EmailRequested bool   `json:"email_requested"`
	EmailStatus    string `json:"email_status"`

	// AuthorCode is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id (rule 6,
	// invariant 8). No name is joined in: this service does not own the staff directory (rule 2),
	// and a name is not this response's to hold.
	AuthorCode string `json:"author_code"`

	// RecipientCount and AckCount are §3's `{x}/{y} đã xác nhận`, DERIVED by counting the recipient
	// rows and stored nowhere (migration 0005 says why there is no counter column).
	RecipientCount int `json:"recipient_count"`
	AckCount       int `json:"ack_count"`

	// IssuedAt is NULL-able in the database and is therefore a POINTER here: a draft has never been
	// issued, and `0001-01-01T00:00:00Z` on the wire is a date a client will render.
	IssuedAt  *time.Time `json:"issued_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func thongBaoRaNgoai(t domain.ThongBaoNoiBo) thongBaoRa {
	ra := thongBaoRa{
		ID:             t.ID,
		Title:          t.TieuDe,
		Body:           t.NoiDung,
		Status:         string(t.TrangThai),
		Pinned:         t.Ghim,
		AckRequired:    t.BatBuocXacNhan,
		EmailRequested: t.GuiThuDienTu,
		EmailStatus:    string(t.TrangThaiThu),
		AuthorCode:     t.NguoiSoanMa,
		RecipientCount: t.SoNguoiNhan,
		AckCount:       t.SoDaXacNhan,
		CreatedAt:      t.TaoLuc,
	}
	if !t.PhatHanhLuc.IsZero() {
		luc := t.PhatHanhLuc
		ra.IssuedAt = &luc
	}
	return ra
}

// DanhSachThongBao serves one page of the commune's announcement book.
// GET /api/v1/announcements
//
// # IT TAKES NO `pham_vi` PARAMETER
//
// §7 sketches `?pham_vi=gui-cho-toi|ca-so`. This route answers the SECOND only, and it does not
// accept the parameter at all rather than accepting it and ignoring one value — a parameter that
// is read and silently has no effect is how a client ends up showing every announcement on a
// screen labelled `Gửi cho tôi`. Why the first scope is not here: the note at the top of this file.
//
// # NO AUDIT ENTRY
//
// Rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes. This is
// neither: these announcements carry no citizen personal data, and the query cannot leave the
// commune the request arrived in. An audit ledger that grew a row per screen opened would bury the
// disclosures it exists to make findable.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachThongBao(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING HERE READS `tenant_id`
	// FROM THE QUERY STRING — a client naming its own commune is a client granting itself access
	// (rule 1, forbidden #2).
	yc, err := page.Parse(r.URL.Query(), commsstore.SapXepThongBaoNoiBo)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	kq, err := h.d.ThongBao.DanhSach(ctx, yc)
	if err != nil {
		// The wrapped error carries the store failure. IT DOES NOT REACH THE CLIENT — and on this
		// route that is more than a habit: the statement behind it quotes no row, but an error text
		// from a table whose columns hold announcement bodies would be the easiest way for a case to
		// reach a log (rule 3, forbidden #3).
		h.d.Log.Error("sổ thông báo nội bộ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// page.Result[T] DIRECTLY — tools/apidoc understands it, so there is no second three-field
	// struct copying it and no way for the two to drift.
	//
	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune that has issued
	// no announcement, never as null. EVERY commune is in that case today, and a client that has to
	// handle both shapes handles one of them wrong.
	ra := page.Result[thongBaoRa]{
		Items:      make([]thongBaoRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, t := range kq.Items {
		ra.Items = append(ra.Items, thongBaoRaNgoai(t))
	}
	vietJSON(w, http.StatusOK, ra)
}

// --- the write route ---------------------------------------------------------------------------

// phatHanhThongBaoVao is the body of POST /api/v1/announcements.
//
// `omitempty` ON EVERY OPTIONAL FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field REQUIRED
// in kb/20-contracts/openapi.json unless it carries `omitempty`, so without it this body would tell
// every generated client that the three flags MUST be sent.
//
// THERE IS NO `status` FIELD AND THERE MUST NEVER BE ONE. The state is decided by which act was
// performed; a client that could name it could post an announcement already marked issued with no
// recipient list behind it. Same for `author_code`: the author is the session's principal (rule 6,
// invariant 8), and a field here is a member of staff issuing a notice over a colleague's name.
//
// Unknown fields are IGNORED rather than refused, which is deliberate: a screen that reads a row
// and posts it back carries `id` and `created_at`, and rejecting that would make the obvious client
// wrong for no benefit.
type phatHanhThongBaoVao struct {
	Title string `json:"title"`
	Body  string `json:"body"`

	// OrgUnitIDs is §5's department chips. IT IS ACCEPTED AND THEN REFUSED, on purpose: a client
	// that sends it is told the capability is missing, rather than watching the field disappear and
	// believing five departments were notified. See app.ErrGuiTheoBoPhanChuaCo.
	OrgUnitIDs []string `json:"org_unit_ids,omitempty"`

	// RecipientCodes is §5's `Gửi thêm đích danh`, as STAFF BUSINESS CODES. Codes and not internal
	// ids, for the reason every other staff reference in this repository gives (rule 6, invariant 8)
	// and one that is specific here: `Gửi cho tôi` compares this value against the session's own
	// `.Ma`, so a second kind of identifier would make that filter impossible to write.
	RecipientCodes []string `json:"recipient_codes,omitempty"`

	Pinned         bool `json:"pinned,omitempty"`
	AckRequired    bool `json:"ack_required,omitempty"`
	EmailRequested bool `json:"email_requested,omitempty"`
}

// PhatHanhThongBao issues one announcement. POST /api/v1/announcements
//
// 201 AND THE WHOLE ROW BACK. §2's list re-reads itself after the modal closes, but the response
// carries what was actually stored — in particular `recipient_count`, which is the number the
// author most needs to see and the one thing they cannot check from the form they just submitted.
func (h *Handler) PhatHanhThongBao(w http.ResponseWriter, r *http.Request) {
	var vao phatHanhThongBaoVao
	if !docThan(w, r, &vao) {
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến phát hành thông báo chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	moi, err := h.d.GhiThongBao.PhatHanh(r.Context(), domain.YeuCauSoanThongBao{
		TieuDe:         vao.Title,
		NoiDung:        vao.Body,
		BoPhanIDs:      vao.OrgUnitIDs,
		NguoiNhanMa:    vao.RecipientCodes,
		Ghim:           vao.Pinned,
		BatBuocXacNhan: vao.AckRequired,
		GuiThuDienTu:   vao.EmailRequested,
	}, nguoi)
	if err != nil {
		h.traLoiLoiThongBao(w, r, err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE ID AND NOT THE BODY: the
	// body would go into Redis, which is a cache and not a record store — and an announcement body
	// is free text about the commune's business. An announcement has no business code, so the id is
	// what there is.
	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, thongBaoRaNgoai(moi))
}

// traLoiLoiThongBao maps one use-case failure onto a status and a sentence.
//
// THE FIFTH ARGUMENT OF httpx.WriteError IS `traceID`, NOT A FIELD NAME. httpx.Error carries
// `code`, `message` and `trace_id` and has no `field` — sixteen call sites in two other services
// passed a field name there and were cleaned up on 2026-09-23. Where a field is at fault, its name
// goes in the SENTENCE, which is where a person reads it.
func (h *Handler) traLoiLoiThongBao(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrGuiTheoBoPhanChuaCo):
		// 501 AND NOT 400 OR 409. The body is well-formed and the caller holds the permission; what
		// is missing is a CAPABILITY OF THE SERVER — expanding a department into the staff inside it
		// needs a service-identity RPC that does not exist. 400 would tell the author their form is
		// wrong, which would send them looking for a mistake they did not make; 409 would claim a
		// conflict with a state, and there is none.
		//
		// THE SENTENCE NAMES THE WORKAROUND, because an error a person cannot act on is an error
		// they report to somebody else.
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented",
			"Chưa gửi được thông báo theo bộ phận: hệ thống chưa lấy được danh sách cán bộ của "+
				"một bộ phận. Hãy chọn từng người ở mục \"Gửi thêm đích danh\".", "")
	case errors.Is(err, domain.ErrKhongCoNguoiNhan):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thông báo phải có ít nhất một người nhận. Hãy chọn người nhận ở mục "+
				"\"Gửi thêm đích danh\".", "")
	case laLoiDauVaoThongBao(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	case errors.Is(err, app.ErrThieuNguoiSoan):
		// The principal reached the handler but carries no staff code — this service is talking to an
		// identity older than the `ma` field. 500 is the honest answer: rule 6 does not permit a
		// business write whose trail cannot name who made it, and there is NO FALLBACK to an id.
		h.d.Log.Error("phát hành thông báo: chủ thể không có mã cán bộ — vết kiểm toán sẽ vô danh",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on —
		// NOT the title and NOT the body.
		h.d.Log.Error("phát hành thông báo: lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoThongBao reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, for the reason laLoiDauVao gives next door: a
// default of "anything I do not recognise is the client's fault" turns a database outage into a
// 400, and a client that believes its input is wrong retries with different input forever while
// nobody is told the server is broken.
func laLoiDauVaoThongBao(err error) bool {
	for _, mot := range []error{
		domain.ErrTieuDeTrong, domain.ErrTieuDeQuaDai,
		domain.ErrNoiDungTrong, domain.ErrNoiDungQuaDai,
		domain.ErrQuaNhieuNguoiNhan, domain.ErrQuaNhieuBoPhan,
		domain.ErrMaCanBoTrong, domain.ErrMaCanBoQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
