package http

// ZALO GROUP CHATS, COMMUNE SIDE (ADR 0092 #1–#3, #8) — Cấu hình → Kênh Zalo, the same key as the rest of
// the tab: `admin.zalo` — "Cấu hình kênh Zalo" — by the owner's decision of 15/10/2026 (tester bug
// sheet web-admin-141026 row 22), replacing the `admin.lookup` of ADR 0074 #6 / ADR 0092 #3; seeded by
// service-identity migration 0032 and pre-granted there to every live role holding `admin.lookup`:
//
//	POST   /api/v1/zalo-group-pairing-codes  admin.zalo  a one-time code for one scope (201, shown once)
//	GET    /api/v1/zalo-groups               admin.zalo  the commune's live groups
//	PATCH  /api/v1/zalo-groups/{id}          admin.zalo  edit a group's scope and/or kinds
//	DELETE /api/v1/zalo-groups/{id}          admin.zalo  unlink a group, with a reason (the ONLY way)
//
// NO chat_id LEAVES HERE (ADR 0092 §Hệ quả): no type below has a field for it.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ZaloGroupAdmin is the use case. *app.ZaloGroups satisfies it. Every write opens its own transaction and
// writes its audit entry inside it (rule 6, invariant 3).
type ZaloGroupAdmin interface {
	IssueGroupPairingCode(ctx context.Context, in app.GroupPairingCodeInput, actor audit.Actor) (app.GroupPairingCodeView, error)
	Groups(ctx context.Context) ([]app.ZaloGroupView, error)
	EditGroup(ctx context.Context, id string, p app.ZaloGroupPatch, actor audit.Actor) error
	UnlinkGroup(ctx context.Context, id, reason string, actor audit.Actor) error
}

// ZaloGroupDeps is everything the four routes touch.
type ZaloGroupDeps struct {
	Checker authz.Checker
	Groups  ZaloGroupAdmin
	Log     *slog.Logger
}

type zaloGroupHandler struct{ d ZaloGroupDeps }

// groupPairingCodeIn is the body of POST zalo-group-pairing-codes. org_unit_id null or absent = the whole
// commune; kinds a non-empty subset of the group kinds the commune has switched on.
type groupPairingCodeIn struct {
	OrgUnitID *string  `json:"org_unit_id"`
	Kinds     []string `json:"kinds"`
}

// groupPairingCodeOut — the code is shown ONCE; the server keeps only its hash.
type groupPairingCodeOut struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// zaloGroupOut is one live group. org_unit_id null = the whole commune. org_unit_name "" when identity holds
// no name; org_unit_removed: the unit left the org chart. linked_by is the code ISSUER's business code.
type zaloGroupOut struct {
	ID             string    `json:"id"`
	OrgUnitID      *string   `json:"org_unit_id"`
	OrgUnitName    string    `json:"org_unit_name"`
	OrgUnitRemoved bool      `json:"org_unit_removed"`
	Kinds          []string  `json:"kinds"`
	LinkedAt       time.Time `json:"linked_at"`
	LinkedBy       string    `json:"linked_by"`
	LinkedByName   string    `json:"linked_by_name"`
}

type zaloGroupList struct {
	Items []zaloGroupOut `json:"items"`
	// SupportedKinds is the list a group may tick (0029's fourteen), in display order.
	SupportedKinds []string `json:"supported_kinds"`
}

// zaloGroupPatchIn is the body of PATCH zalo-groups/{id}. An ABSENT (or null) field keeps its value;
// org_unit_id "" moves the group to the whole commune — the repo's PATCH convention (operator
// petition-fields: absent = keep, "" = clear). Not a json.RawMessage tri-state: tools/apidoc cannot describe
// a type outside the repo, and an undocumented shape is one the web builds against by guess. At least one
// of the two.
type zaloGroupPatchIn struct {
	OrgUnitID *string  `json:"org_unit_id"`
	Kinds     []string `json:"kinds"`
}

// zaloGroupUnlinkIn is the body of DELETE — rule 7: a soft delete carries its reason.
type zaloGroupUnlinkIn struct {
	Reason string `json:"reason"`
}

// RegisterZaloGroups mounts the four routes on the staff mux. Refuses incomplete wiring at construction.
func RegisterZaloGroups(mux *http.ServeMux, d ZaloGroupDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến nhóm Zalo sẽ không kiểm được quyền")
	case d.Groups == nil:
		panic("comms/http: thiếu use case nhóm Zalo — /api/v1/zalo-groups sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloGroupHandler{d: d}

	// 201: a new resource. idem.KhongCan, AND WHY NOT idem.Required: a replay would keep a LIVE CREDENTIAL
	// in the cache for a day. A duplicate request issues a second code for the same scope instead; each
	// expires in ten minutes and works once, so the cost is one unused code and one more entry.
	//
	//	409 zalo_bot_not_configured  no bot can serve the commune — a code for it would link nothing
	//	409 zalo_channel_disabled    the commune's channel is off: it has no kind to give a group
	//	422 <code>                   the unit or a kind is refused — the code and sentence say which
	//	503 org_units_unavailable    identity could not be asked whether the unit is live
	//
	// @summary  Tạo mã gắn nhóm Zalo 8 ký tự cho một phạm vi — một bộ phận đang hoạt động của xã hoặc cả xã (org_unit_id null), các loại tin là tập con các loại dành cho nhóm mà xã đang bật; dùng một lần, sống 10 phút, chỉ lưu bản băm; gõ mã vào nhóm Zalo đã thêm bot (có thể kèm @tên bot); có ghi vết (không ghi mã)
	// @screen   ADR 0092 #3 — Cấu hình → tab Kênh Zalo
	// @request  groupPairingCodeIn
	// @reply    201 groupPairingCodeOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error zalo_bot_not_configured zalo_channel_disabled
	// @reply    422 httpx.Error org_unit_invalid org_unit_not_live kinds_required unknown_kind kind_not_enabled
	// @reply    503 httpx.Error org_units_unavailable
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-group-pairing-codes",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("lần gửi thứ hai cấp thêm một mã gắn nhóm cho cùng phạm vi; mỗi mã sống 10 phút và chỉ dùng một lần, nên cái giá là một mã bỏ không; lưu câu trả lời để phát lại là giữ một mã còn sống trong bộ đệm")(
				http.HandlerFunc(h.CreateGroupPairingCode))))

	// NO idem.* DECLARATION: a GET. No audit entry: the commune's own configuration, read in it.
	//
	//	503 org_units_unavailable / staff_names_unavailable — identity could not be asked; never a page of blanks
	//
	// @summary  Các nhóm Zalo đang gắn với xã — phạm vi (bộ phận hoặc cả xã, kèm tên bộ phận và cờ bộ phận đã gỡ), các loại tin, gắn lúc nào, mã và tên cán bộ đã tạo mã; supported_kinds là danh sách loại được chọn cho nhóm; không bao giờ trả chat_id
	// @screen   ADR 0092 #1 — Cấu hình → tab Kênh Zalo
	// @reply    200 zaloGroupList
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error org_units_unavailable staff_names_unavailable
	mux.Handle("GET /api/v1/zalo-groups",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			http.HandlerFunc(h.ListGroups)))

	// PATCH AND NOT PUT: an edit of the kinds alone must not move the scope. idem.KhongCan: an edit equal to
	// what is stored writes and files nothing, so the same request twice leaves one entry.
	//
	//	404 not_found  no live group by that id in this commune (unknown, ended, another commune's: one answer)
	//
	// @summary  Sửa phạm vi (bộ phận hoặc cả xã) và/hoặc các loại tin của một nhóm Zalo đang gắn — loại tin phải là loại dành cho nhóm mà xã đang bật; có ghi vết trước/sau
	// @screen   ADR 0092 #1 #2 — Cấu hình → tab Kênh Zalo
	// @request  zaloGroupPatchIn
	// @reply    204 -
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error zalo_channel_disabled
	// @reply    422 httpx.Error nothing_to_change org_unit_invalid org_unit_not_live kinds_required unknown_kind kind_not_enabled
	// @reply    503 httpx.Error org_units_unavailable
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/zalo-groups/{id}",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("sửa là ghi đè phạm vi và loại tin bằng đúng giá trị trong thân; trùng giá trị đang lưu thì không ghi và không có vết, nên lần gửi thứ hai để lại đúng một vết")(
				http.HandlerFunc(h.EditGroup))))

	// A SOFT DELETE with a mandatory reason, IN THE BODY. idem.KhongCan: a second DELETE finds no live group
	// (404) — the locked read and the UPDATE carry `unlinked_at IS NULL`, so who unlinked it and why stand.
	//
	// @summary  Gỡ một nhóm Zalo khỏi xã — kết thúc mềm liên kết (giữ làm lịch sử), cần lý do, có ghi vết; đây là cách DUY NHẤT gỡ nhóm (/dung trong nhóm không có tác dụng)
	// @screen   ADR 0092 #8 — Cấu hình → tab Kênh Zalo
	// @request  zaloGroupUnlinkIn
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    422 httpx.Error reason_required
	// @reply    400 httpx.Error invalid_request
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/zalo-groups/{id}",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("lần gỡ thứ hai không còn nhóm đang gắn nên trả 404, không ghi gì và không có vết")(
				http.HandlerFunc(h.UnlinkGroup))))
}

// actor is the signed-in member of staff, by BUSINESS CODE (rule 6, invariant 8) — never a fallback.
func (h *zaloGroupHandler) actor(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	a, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến nhóm Zalo chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return a, ok
}

// CreateGroupPairingCode — POST /api/v1/zalo-group-pairing-codes
func (h *zaloGroupHandler) CreateGroupPairingCode(w http.ResponseWriter, r *http.Request) {
	var in groupPairingCodeIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	unit := ""
	if in.OrgUnitID != nil {
		unit = *in.OrgUnitID
	}
	v, err := h.d.Groups.IssueGroupPairingCode(r.Context(), app.GroupPairingCodeInput{OrgUnitID: unit, Kinds: in.Kinds}, actor)
	if err != nil {
		h.fail(w, r, "tạo mã gắn nhóm", err)
		return
	}
	// A credential: never cached by anything between here and the browser.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusCreated, groupPairingCodeOut{Code: v.Code, ExpiresAt: v.ExpiresAt})
}

// ListGroups — GET /api/v1/zalo-groups
func (h *zaloGroupHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	list, err := h.d.Groups.Groups(r.Context())
	if err != nil {
		h.fail(w, r, "liệt kê nhóm", err)
		return
	}
	out := zaloGroupList{Items: make([]zaloGroupOut, 0, len(list)),
		SupportedKinds: append([]string(nil), domain.ZaloGroupKinds...)}
	for _, g := range list {
		o := zaloGroupOut{ID: g.ID, OrgUnitName: g.OrgUnitName, OrgUnitRemoved: g.OrgUnitRemoved, Kinds: g.Kinds,
			LinkedAt: g.LinkedAt, LinkedBy: g.LinkedBy, LinkedByName: g.LinkedByName}
		if g.OrgUnitID != "" {
			u := g.OrgUnitID
			o.OrgUnitID = &u
		}
		if o.Kinds == nil {
			o.Kinds = []string{}
		}
		out.Items = append(out.Items, o)
	}
	vietJSON(w, http.StatusOK, out)
}

// EditGroup — PATCH /api/v1/zalo-groups/{id}
func (h *zaloGroupHandler) EditGroup(w http.ResponseWriter, r *http.Request) {
	var in zaloGroupPatchIn
	if !docThan(w, r, &in) {
		return
	}
	p := app.ZaloGroupPatch{Kinds: in.Kinds}
	if in.OrgUnitID != nil {
		p.OrgUnitSet = true
		p.OrgUnitID = *in.OrgUnitID // "" = the whole commune
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.d.Groups.EditGroup(r.Context(), r.PathValue("id"), p, actor); err != nil {
		h.fail(w, r, "sửa nhóm", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnlinkGroup — DELETE /api/v1/zalo-groups/{id}
func (h *zaloGroupHandler) UnlinkGroup(w http.ResponseWriter, r *http.Request) {
	var in zaloGroupUnlinkIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.d.Groups.UnlinkGroup(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.fail(w, r, "gỡ nhóm", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// fail maps a use-case error to its reply. The wrapped error never reaches the client (rule 3).
func (h *zaloGroupHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	xa := string(tenant.MustFrom(r.Context()))
	var ge *domain.ZaloGroupError
	switch {
	case errors.As(err, &ge):
		httpx.WriteError(w, http.StatusUnprocessableEntity, ge.Code, ge.Message, "")
	case errors.Is(err, commsstore.ErrZaloGroupNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy nhóm Zalo đang gắn này.", "")
	case errors.Is(err, app.ErrZaloBotNotConfigured):
		httpx.WriteError(w, http.StatusConflict, "zalo_bot_not_configured",
			"Chưa có bot Zalo nào phục vụ xã. Vui lòng báo quản trị nền tảng hoặc cấu hình bot riêng của xã.", "")
	case errors.Is(err, app.ErrZaloChannelOff):
		httpx.WriteError(w, http.StatusConflict, "zalo_channel_disabled",
			"Xã chưa bật kênh nhắc việc qua Zalo. Hãy bật kênh và chọn loại tin trước khi gắn nhóm.", "")
	case errors.Is(err, app.ErrOrgUnitsUnavailable):
		h.d.Log.Warn("nhóm Zalo: "+op+" — không hỏi được danh sách bộ phận", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "org_units_unavailable",
			"Tạm thời không kiểm được bộ phận của xã. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrStaffNamesUnavailable):
		h.d.Log.Warn("nhóm Zalo: "+op+" — không tra được tên cán bộ", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "staff_names_unavailable",
			"Tạm thời không tải được họ tên cán bộ. Vui lòng thử lại sau ít phút.", "")
	default:
		h.d.Log.Error("nhóm Zalo: "+op+" lỗi hệ thống", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
