package http

// EXTERNAL CONTACTS — `liên hệ ngoài bộ máy xã` (migration 0014; owner comms, user decision 03/10/2026):
// the bodies OUTSIDE the commune's apparatus that residents call — the health station, the commune
// police, the power company. Staff edit them in a tab of /danh-ba; residents read them on the Mini App
// directory.
//
//	GET    /api/v1/external-contacts                  content.read    the whole list, staff
//	POST   /api/v1/external-contacts                  content.update  add one
//	PATCH  /api/v1/external-contacts/{id}             content.update  edit one
//	DELETE /api/v1/external-contacts/{id}             content.update  soft delete, reason mandatory
//	GET    /api/v1/commune-external-contacts?host=    Public          the whole list, residents
//
// THE NOUNS ARE THE USER'S (03/10/2026): `external-contacts` for staff, `commune-external-contacts` for
// the public read — the same split as `content-items` / `commune-news`.
//
// THE KEYS ARE THE USER'S TOO, and both exist: `content.read` / `content.update` are seeded at
// service-identity/migrations/0001_init.sql:292-293 — "Xem / Sửa nội dung và danh bạ Mini App", which
// names exactly this list. NO KEY WAS INVENTED (rule 5, invariant 3c). The DELETE carries
// `content.update` because that is what DELETE /api/v1/content-items/{id} carries; there is no
// separate delete key in the `quyen` table for this group.
//
// A SEPARATE REGISTRATION WITH ITS OWN DEPS (RegisterExternalContacts / RegisterPublicExternalContacts),
// not more fields on Deps / DepsCongKhai: the five routes share nothing with the rest of the service but
// the checker, the platform lookup and the public limiter, all passed in. cmd/server mounts them on the
// same two muxes.
//
// PERSONAL DATA (rule 3). `phone` and `address` are treated as possibly personal (0014, PERSONAL DATA):
// never logged, never in an error, masked in the audit delta (app/external_contact.go). They are
// returned UNMASKED on both surfaces, because being shown is the purpose of the row — the commune typed
// them for residents to call — and staff must see what they edit.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// PublicExternalContactsPath is the public collection. Exported because cmd/server must route the SAME
// path to the public chain on the outer mux (the reasoning at MauTinXa); the route below spells it as a
// LITERAL because tools/apidoc reads the pattern from the source, and a test asserts the two agree.
const PublicExternalContactsPath = "/api/v1/commune-external-contacts"

// ExternalContactsReader is the READ half — one live list, the same rows for staff and residents (no
// publish flag, 0014). *commsstore.ExternalContactStore satisfies it.
type ExternalContactsReader interface {
	List(ctx context.Context) ([]domain.ExternalContact, error)
}

// ExternalContactsWriter is the WRITE half. A second interface for the reason GhiLoaiTaiNguyen gives:
// each of these opens a TRANSACTION and writes its audit entry inside it (rule 6, invariant 3).
// *app.ExternalContacts satisfies it.
type ExternalContactsWriter interface {
	Create(ctx context.Context, in app.ExternalContactInput, actor audit.Actor) (domain.ExternalContact, error)
	Update(ctx context.Context, id string, p app.ExternalContactPatch, actor audit.Actor) (domain.ExternalContact, error)
	Delete(ctx context.Context, id, reason string, actor audit.Actor) error
}

// ExternalContactDeps is everything the four staff routes touch.
type ExternalContactDeps struct {
	Checker authz.Checker
	Reader  ExternalContactsReader
	Writer  ExternalContactsWriter
	Log     *slog.Logger
}

// PublicExternalContactDeps is everything the public route touches — nothing else is reachable from it.
type PublicExternalContactDeps struct {
	// Xa is the SAME platform lookup the public news routes use (TraXaTheoHost): an outage is a 503,
	// never an empty list that reads as "this commune has no contacts".
	Xa       TraXaTheoHost
	Contacts ExternalContactsReader

	// Limiter is ratelimit.PublicNewsRead — the EXISTING policy, by the user's decision of 03/10/2026, and
	// in production the SAME *ratelimit.Limiter instance the news routes hold. The key is the same
	// (ratelimit.PublicHostIPKey, `t:<tenant_id>:rl:public-news:host:…:ip:…`), so news reads and contact
	// reads from one client network share ONE budget of 120 per minute per host.
	Limiter *ratelimit.Limiter

	Log *slog.Logger
}

type externalContactHandler struct {
	d ExternalContactDeps
}

// externalContactOut is one contact as it leaves the API, on both surfaces.
type externalContactOut struct {
	// ID is the row's ULID — the handle the staff routes take, and a React key on the Mini App. Random,
	// so it enumerates nothing; not personal data.
	ID string `json:"id"`

	Name string `json:"name"`

	// Category is the group heading, free text: rows sharing the EXACT text are one group.
	Category string `json:"category"`

	// Phone is dial characters only (digits, `+`, space, `.`, `-`, parentheses) — ready for a `tel:` link.
	Phone string `json:"phone"`

	// Address is ABSENT when the commune entered none.
	Address string `json:"address,omitempty"`

	// DisplayOrder is ABSENT when no position was given; such rows come LAST, then by id.
	DisplayOrder *int `json:"display_order,omitempty"`
}

// externalContactListOut wraps the list in an object; no cursor — the whole list or a refusal, like
// map-field-schemas. Bounded by commsstore.ExternalContactCeiling.
type externalContactListOut struct {
	Items []externalContactOut `json:"items"`
}

// createExternalContactIn is the body of POST. `omitempty` on the optional fields so tools/apidoc does
// not mark them required.
type createExternalContactIn struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Phone        string `json:"phone"`
	Address      string `json:"address,omitempty"`
	DisplayOrder *int   `json:"display_order,omitempty"`
}

// updateExternalContactIn is the body of PATCH. ABSENT = leave alone. `address: ""` CLEARS the address.
// `display_order` cannot be cleared once set (a JSON null reads as absent through a pointer — the limit
// content-items' `display_order` states); a number sets it.
type updateExternalContactIn struct {
	Name         *string `json:"name,omitempty"`
	Category     *string `json:"category,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	Address      *string `json:"address,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

// deleteExternalContactIn — the reason is mandatory (rule 7, invariant 1) and travels in the body, never
// the query string, where free text would land in every access log.
type deleteExternalContactIn struct {
	Reason string `json:"reason"`
}

func externalContactToOut(c domain.ExternalContact) externalContactOut {
	return externalContactOut{
		ID: c.ID, Name: c.Name, Category: c.Category, Phone: c.Phone, Address: c.Address,
		DisplayOrder: c.DisplayOrder,
	}
}

// RegisterExternalContacts mounts the four STAFF routes. Refuses incomplete wiring at construction,
// like Register: a route without its store would answer every caller with a recovered panic.
func RegisterExternalContacts(mux *http.ServeMux, d ExternalContactDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — bốn tuyến liên hệ ngoài bộ máy xã sẽ không kiểm được quyền")
	case d.Reader == nil:
		panic("comms/http: thiếu kho liên hệ ngoài bộ máy xã — GET /api/v1/external-contacts sẽ panic khi có người gọi")
	case d.Writer == nil:
		panic("comms/http: thiếu use case ghi liên hệ ngoài bộ máy xã — POST/PATCH/DELETE /api/v1/external-contacts sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &externalContactHandler{d: d}

	// NOT PAGINATED ON PURPOSE: a few dozen rows per commune (0014), returned whole in display order —
	// the order the staff tab and the Mini App both draw. Past commsstore.ExternalContactCeiling the
	// route REFUSES with a 500 rather than truncating: a silently short list is a number nobody finds.
	//
	// NO idem.* DECLARATION: a GET changes no state. No audit entry: the commune's own published list,
	// read inside that commune (rule 6, invariant 7 asks for neither case).
	//
	// @summary  Liên hệ ngoài bộ máy xã (trạm y tế, công an, điện lực…) — cả danh sách của xã theo thứ tự hiển thị, cho tab trong Danh bạ
	// @screen   12-danh-ba-can-bo §2
	// @reply    200 externalContactListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/external-contacts",
		authz.RequirePermission(d.Checker, "content.read")(
			http.HandlerFunc(h.ListExternalContacts)))

	// idem.Required(DongKhiHong), THE CONTENT-ITEM CHOICE AND NOT THE CATEGORY'S: there is NO unique key
	// underneath (0014 — two rows may share a name), so with MoKhiHong a cache outage would let a
	// double-submitted form put the same number on every resident's directory twice. The idempotency key
	// is the ONLY layer, so it cannot fail open; refusing while the cache is down costs one button.
	//
	// 409 `catalogue_full` past commsstore.ExternalContactCeiling.
	//
	// @summary  Thêm một liên hệ ngoài bộ máy xã — hiện ngay trên danh bạ Zalo Mini App (không có cờ công khai)
	// @screen   12-danh-ba-can-bo §2
	// @request  createExternalContactIn
	// @reply    201 externalContactOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/external-contacts",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.CreateExternalContact))))

	// PATCH AND NOT PUT: `address` and `display_order` are optional, and a dialog editing only the phone
	// must not clear them. 404 is ONE answer for no such id, another commune's id and a deleted row.
	//
	// idem.KhongCan: app.ExternalContacts.Update writes and audits nothing when nothing moved, so the same
	// request twice leaves one state and one entry.
	//
	// @summary  Sửa tên, nhóm, số điện thoại, địa chỉ hoặc thứ tự của một liên hệ ngoài bộ máy xã
	// @screen   12-danh-ba-can-bo §2
	// @request  updateExternalContactIn
	// @reply    200 externalContactOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/external-contacts/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; use case không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateExternalContact))))

	// A SOFT DELETE with a mandatory reason, IN THE BODY — the shape of DELETE /api/v1/content-items/{id}
	// (`{ "reason": "…" }`, 204). The row leaves the Mini App in the same statement: a live row is a
	// public row.
	//
	// idem.KhongCan — a second delete is a 404: the locked read and the UPDATE carry
	// `AND deleted_at IS NULL`, so it cannot overwrite who deleted it or why.
	//
	// @summary  Xoá mềm một liên hệ ngoài bộ máy xã, kèm lý do bắt buộc — gỡ khỏi danh bạ Mini App ngay trong cùng lần ghi
	// @screen   12-danh-ba-can-bo §2
	// @request  deleteExternalContactIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/external-contacts/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("xoá một liên hệ đã xoá trả 404: lượt đọc khoá dòng và câu UPDATE đều mang `AND deleted_at IS NULL`, nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteExternalContact))))
}

// RegisterPublicExternalContacts mounts the PUBLIC route onto the public mux (cmd/server muxCongKhai).
func RegisterPublicExternalContacts(mux *http.ServeMux, d PublicExternalContactDeps) {
	switch {
	case d.Xa == nil:
		panic("comms/http: thiếu kho tra xã theo tên miền — tuyến liên hệ ngoài bộ máy của xã sẽ panic khi có người gọi")
	case d.Contacts == nil:
		panic("comms/http: thiếu kho liên hệ ngoài bộ máy xã — tuyến công khai sẽ panic khi có người gọi")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất tuyến công khai (ratelimit.PublicNewsRead)")
	}
	// THE PUBLIC NEWS HANDLER, built with only what its host resolution reads (Xa, Limiter, Log), so
	// PublicExternalContacts goes through the SAME xaTheoHost the three news routes use — host check,
	// platform lookup, `t:<tenant_id>` limiter key, 429, 503 — not a copy of it that could drift.
	h := newHandlerCongKhai(DepsCongKhai{Xa: d.Xa, Limiter: d.Limiter, Log: d.Log})
	p := &publicExternalContacts{h: h, contacts: d.Contacts}

	// PUBLIC (user decision 03/10/2026): the commune typed these institutions' numbers FOR its residents,
	// and a resident opening the Mini App has no account and needs none to read them. Every live row is
	// public — there is no publish flag (0014).
	//
	// RATE LIMIT — ratelimit.PublicNewsRead (rule 13, invariant 7; the user chose the EXISTING policy):
	// 120 requests per minute per (host, client network), counted after the platform resolved the host,
	// the key `t:<tenant_id>:…` for a commune. Over it: 429 `rate_limited` + Retry-After. A Redis outage
	// SERVES (the policy's fail-open exception) with one security warning per minute.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Liên hệ ngoài bộ máy xã (trạm y tế, công an, điện lực…) theo tên miền của xã — cho danh bạ trên Zalo Mini App, nhóm theo `category`
	// @screen   12-danh-ba-can-bo §8
	// @consumer citizen-app
	//
	// 200 is the whole list, never paginated, in display order (unordered last). `{"items":[]}` for a
	// commune with no contact AND for a domain no active commune holds — identical bytes.
	//
	// 400 is `host` missing, repeated or malformed (the platform is not asked). 500 is a store failure or
	// the ceiling exceeded. 503 is the platform registry unreachable. 429 is the rate limit above.
	//
	// @reply    200 externalContactListOut
	// @reply    400 httpx.Error
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-external-contacts",
		authz.Public("danh bạ liên hệ ngoài bộ máy xã (trạm y tế, công an, điện lực…) mà xã nhập để người dân gọi, hiện trên Zalo Mini App (người dùng quyết định 03/10/2026): người dân đọc không cần tài khoản; chỉ trả dòng chưa xoá của đúng xã mà nền tảng phân giải từ tên miền; giới hạn 120 lần/phút theo tên miền và mạng của máy gọi")(
			http.HandlerFunc(p.PublicExternalContacts)))
}

// ListExternalContacts — GET /api/v1/external-contacts
func (h *externalContactHandler) ListExternalContacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Scoped in the store: ExternalContactStore.List reads through db.For(ctx).Query.
	list, err := h.d.Reader.List(ctx)
	if err != nil {
		h.listError(ctx, w, err)
		return
	}
	writeExternalContactList(w, list, func(s string) string { return s })
}

func (h *externalContactHandler) listError(ctx context.Context, w http.ResponseWriter, err error) {
	if errors.Is(err, commsstore.ErrTooManyExternalContacts) {
		h.d.Log.Error("liên hệ ngoài bộ máy xã vượt trần — TỪ CHỐI thay vì cắt bớt",
			"xa", string(tenant.MustFrom(ctx)), "tran", commsstore.ExternalContactCeiling)
	} else {
		h.d.Log.Error("liên hệ ngoài bộ máy xã: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// writeExternalContactList writes `{"items":[…]}` — `[]`, never `null`. text is applied to the free-text
// fields (identity on the staff surface, plain text on the public one).
func writeExternalContactList(w http.ResponseWriter, list []domain.ExternalContact, text func(string) string) {
	out := externalContactListOut{Items: make([]externalContactOut, 0, len(list))}
	for _, c := range list {
		o := externalContactToOut(c)
		o.Name, o.Category, o.Address = text(o.Name), text(o.Category), text(o.Address)
		out.Items = append(out.Items, o)
	}
	vietJSON(w, http.StatusOK, out)
}

// CreateExternalContact — POST /api/v1/external-contacts
func (h *externalContactHandler) CreateExternalContact(w http.ResponseWriter, r *http.Request) {
	var in createExternalContactIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.ExternalContacts opens db.For(ctx).Tx.
	row, err := h.d.Writer.Create(r.Context(), app.ExternalContactInput{
		Name: in.Name, Category: in.Category, Phone: in.Phone, Address: in.Address, DisplayOrder: in.DisplayOrder,
	}, actor)
	if err != nil {
		h.writeError(w, r, "thêm", err)
		return
	}
	// The row's id, not the body: the body (a phone number) would go into Redis, a cache.
	idem.RecordCode(r.Context(), row.ID)
	vietJSON(w, http.StatusCreated, externalContactToOut(row))
}

// UpdateExternalContact — PATCH /api/v1/external-contacts/{id}
func (h *externalContactHandler) UpdateExternalContact(w http.ResponseWriter, r *http.Request) {
	var in updateExternalContactIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.ExternalContacts opens db.For(ctx).Tx.
	row, err := h.d.Writer.Update(r.Context(), r.PathValue("id"), app.ExternalContactPatch{
		Name: in.Name, Category: in.Category, Phone: in.Phone, Address: in.Address, DisplayOrder: in.DisplayOrder,
	}, actor)
	if err != nil {
		h.writeError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, externalContactToOut(row))
}

// DeleteExternalContact — DELETE /api/v1/external-contacts/{id}. Soft delete; 204, no body.
func (h *externalContactHandler) DeleteExternalContact(w http.ResponseWriter, r *http.Request) {
	var in deleteExternalContactIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.ExternalContacts opens db.For(ctx).Tx.
	if err := h.d.Writer.Delete(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeError(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// missingPrincipal — a write route reached with no principal carrying a business code. Behind
// RequirePermission, so this is a wiring fault or an identity older than `ma`; rule 6 does not permit a
// write whose trail cannot name who made it — and there is NO fallback to the internal id.
func (h *externalContactHandler) missingPrincipal(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi liên hệ ngoài bộ máy xã chạy mà không có chủ thể mang mã cán bộ",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// writeError maps a use-case failure onto a status: 404 for a row not live in this commune, 409 for the
// ceiling, 400 for the fixed sentences below, 500 for everything else — never a default 400. The wrapped
// error is logged, never sent (rule 3, forbidden #3); it names a rule and a field, never a value.
func (h *externalContactHandler) writeError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, commsstore.ErrExternalContactNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy liên hệ này.", "")
	case errors.Is(err, commsstore.ErrExternalContactsFull):
		httpx.WriteError(w, http.StatusConflict, "catalogue_full",
			fmt.Sprintf("Xã đã có %d liên hệ ngoài bộ máy — mức tối đa. Hãy xoá bớt liên hệ không dùng.",
				commsstore.ExternalContactCeiling), "")
	default:
		if msg, ok := refusalMessage(externalContactRefusals, err); ok {
			h.d.Log.Info("liên hệ ngoài bộ máy xã: từ chối "+op, "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
			return
		}
		h.d.Log.Error("liên hệ ngoài bộ máy xã: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// externalContactRefusals — the 400s of the write routes, one fixed sentence per domain sentinel (the
// split refusal{} explains). Bounds are read from domain, never retyped.
var externalContactRefusals = []refusal{
	{domain.ErrExternalContactNameEmpty, "Tên liên hệ đang trống hoặc chứa ký tự không hợp lệ."},
	{domain.ErrExternalContactNameTooLong, fmt.Sprintf("Tên liên hệ quá dài (tối đa %d ký tự).", domain.ExternalContactNameMaxLen)},
	{domain.ErrExternalContactCategoryEmpty, "Nhóm liên hệ đang trống hoặc chứa ký tự không hợp lệ."},
	{domain.ErrExternalContactCategoryTooLong, fmt.Sprintf("Nhóm liên hệ quá dài (tối đa %d ký tự).", domain.ExternalContactCategoryMaxLen)},
	{domain.ErrExternalContactPhoneEmpty, "Chưa nhập số điện thoại."},
	{domain.ErrExternalContactPhoneShape, fmt.Sprintf("Số điện thoại chỉ gồm chữ số và các dấu + . - ( ) cùng dấu cách, "+
		"có từ %d đến %d chữ số, tối đa %d ký tự. Số máy lẻ hãy ghi vào địa chỉ.",
		domain.ExternalContactPhoneMinDigits, domain.ExternalContactPhoneMaxDigits, domain.ExternalContactPhoneMaxLen)},
	{domain.ErrExternalContactAddressInvalid, "Địa chỉ chứa ký tự không hợp lệ."},
	{domain.ErrExternalContactAddressTooLong, fmt.Sprintf("Địa chỉ quá dài (tối đa %d ký tự).", domain.ExternalContactAddressMaxLen)},
	{domain.ErrExternalContactOrderInvalid, "Thứ tự hiển thị phải là số nguyên không âm."},
	{domain.ErrThieuLyDoXoa, "Hãy nhập lý do xoá. Liên hệ đã hiện cho người dân là hồ sơ lưu trữ, xoá phải ghi rõ vì sao."},
	{domain.ErrLyDoXoaQuaDai, fmt.Sprintf("Lý do xoá quá dài (tối đa %d ký tự).", domain.LyDoXoaToiDa)},
}

// publicExternalContacts serves the public route through the news surface's host resolution.
type publicExternalContacts struct {
	h        *HandlerCongKhai
	contacts ExternalContactsReader
}

// PublicExternalContacts serves the commune's live external contacts to the Mini App.
// GET /api/v1/commune-external-contacts?host=
//
// THE SAME NEGATIVES AS GET /api/v1/commune-news: `host` validated (domain.HopLeTenMienXa) before the
// platform is asked; an unknown, reserved or inactive domain is `{"items":[]}` with 200, byte-identical
// to an active commune with no contact; an unreachable platform is 503; past the limit 429.
//
// EVERY TEXT FIELD IS PLAIN TEXT (domain.VanBanThuanChoDan), like every text on this surface: the staff
// form accepts any characters. `phone` needs no such pass — its character set is closed (0014).
//
// NO AUDIT ENTRY: nothing is written; this is what the commune entered for its residents, read inside
// the one commune the host resolved to.
func (p *publicExternalContacts) PublicExternalContacts(w http.ResponseWriter, r *http.Request) {
	gia := r.URL.Query()["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}

	xa, co, ok := p.h.xaTheoHost(w, r, gia[0], "liên hệ ngoài bộ máy của xã")
	if !ok {
		return
	}
	if !co {
		vietJSON(w, http.StatusOK, externalContactListOut{Items: []externalContactOut{}})
		return
	}

	ctx := tenant.Into(r.Context(), xa.ID)
	// Scoped in the store: ExternalContactStore.List reads through db.For(ctx).Query, live rows only.
	list, err := p.contacts.List(ctx)
	if err != nil {
		if errors.Is(err, commsstore.ErrTooManyExternalContacts) {
			p.h.d.Log.ErrorContext(ctx, "liên hệ ngoài bộ máy của xã: vượt trần",
				"xa", string(xa.ID), "tran", commsstore.ExternalContactCeiling)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		p.h.loi500(ctx, w, "liên hệ ngoài bộ máy của xã", err)
		return
	}
	writeExternalContactList(w, list, domain.VanBanThuanChoDan)
}
