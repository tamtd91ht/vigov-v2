package http

// THE HEADER BELL — the signed-in staff member's OWN inbox (docs/ui-ux/08-thong-bao.md §8,
// docs/ui-ux/15-phu-luc-giao-dien-chung.md §3.2). Routes and their declarations are in
// routes_staff_notification.go.
//
// # ONE PERSON'S INBOX, AND THE PERSON COMES FROM THE SESSION
//
// Every read and every write here is filtered by (commune, authz.Principal.Ma). There is no query
// parameter, path segment or body field naming a recipient: a staff member cannot read, count or
// mark another staff member's notices, because no input reaches that filter but the session. Another
// person's notice id answers 404 exactly like an id that does not exist (rule 4, forbidden #2,
// applied between staff).
//
// WHAT THE BELL DOES NOT SHOW YET: issued announcements (§8's first row). Those live in the
// announcement book of migration 0005 and are not copied into this inbox by app.SoanThongBaoNoiBo —
// a follow-up in the same service, not a contract question.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// notificationOut is one bell item as it leaves the API.
//
// NOTHING HERE IS MASKED: the contract forbids citizen personal data in `title` and `body`
// (comms.proto, "WHAT A NOTICE MAY SAY"), and the recipient is the reader. The two strings are still
// never logged on the way out.
type notificationOut struct {
	ID string `json:"id"`
	// Kind is `sap-den-han` · `qua-han` · `leo-thang` · `ban-tin-tuan` (ADR 0011) — what the bell
	// draws as the icon.
	Kind  string `json:"kind"`
	Title string `json:"title"`
	// Body is the second line; "" draws none.
	Body string `json:"body"`
	// Link is a RELATIVE web-admin path, or "" when the item leads nowhere. Validated relative at
	// delivery and by a CHECK in the table; a client still navigates with the router, never as a URL.
	Link string `json:"link"`
	// Read drives the blue dot. ReadAt is null while unread.
	Read      bool       `json:"read"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func notificationToOut(n domain.StaffNotification) notificationOut {
	out := notificationOut{
		ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body, Link: n.Link,
		Read: n.Read(), CreatedAt: n.CreatedAt,
	}
	if n.Read() {
		at := n.ReadAt
		out.ReadAt = &at
	}
	return out
}

// unreadCountOut is the badge: `Thông báo — {unread} thông báo chưa đọc`.
type unreadCountOut struct {
	Unread int `json:"unread"`
}

// markReadIn is the body of both PATCH routes. `read` MUST be true: marking a notice unread again is
// refused — `read_at` is one-way (migration 0010), so the badge never rises with no event behind it.
type markReadIn struct {
	Read *bool `json:"read"`
}

// markAllReadOut says how many notices `Đọc hết` moved.
type markAllReadOut struct {
	Marked int `json:"marked"`
}

// inboxOwner returns the signed-in STAFF member, or answers and returns false.
//
//	not staff      403 — the bell belongs to staff accounts. The staff chain never carries a citizen
//	               principal; refusing here keeps that true if a route is ever mounted elsewhere.
//	no staff code  500 — an identity older than the `ma` field. NO FALLBACK to the internal id: the
//	               inbox is addressed by code, and the trail needs the code (rule 6, invariant 8).
func (h *Handler) inboxOwner(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok {
		// AnyAuthenticated already answered 401; reaching here is a mounting fault.
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
			"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
		return audit.Actor{}, false
	}
	if p.Kind != "staff" {
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Hộp thông báo chỉ dành cho tài khoản cán bộ.", "")
		return audit.Actor{}, false
	}
	if p.Ma == "" {
		h.d.Log.Error("hộp thông báo: chủ thể không có mã cán bộ — không đọc được hộp thư theo mã",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.Ma, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}

// ListMyNotifications serves GET /api/v1/notifications — one page of the caller's own bell, newest
// first. NO AUDIT ENTRY: a person reading notices addressed to them is neither full personal data
// nor a cross-commune read (rule 6, invariant 7).
func (h *Handler) ListMyNotifications(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.inboxOwner(w, r)
	if !ok {
		return
	}
	req, err := page.Parse(r.URL.Query(), commsstore.SortStaffNotifications)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	res, err := h.d.StaffInbox.ListOwn(r.Context(), owner.ID, req)
	if err != nil {
		h.inboxFailure(w, r, "đọc hộp thông báo", err)
		return
	}
	out := page.Result[notificationOut]{
		Items:      make([]notificationOut, 0, len(res.Items)),
		NextCursor: res.NextCursor,
		HasMore:    res.HasMore,
	}
	for _, n := range res.Items {
		out.Items = append(out.Items, notificationToOut(n))
	}
	vietJSON(w, http.StatusOK, out)
}

// CountMyUnreadNotifications serves GET /api/v1/notifications/unread-count — the badge.
func (h *Handler) CountMyUnreadNotifications(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.inboxOwner(w, r)
	if !ok {
		return
	}
	n, err := h.d.StaffInbox.CountUnread(r.Context(), owner.ID)
	if err != nil {
		h.inboxFailure(w, r, "đếm thông báo chưa đọc", err)
		return
	}
	vietJSON(w, http.StatusOK, unreadCountOut{Unread: n})
}

// MarkMyNotificationRead serves PATCH /api/v1/notifications/{id} with {"read": true}.
func (h *Handler) MarkMyNotificationRead(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.inboxOwner(w, r)
	if !ok {
		return
	}
	if !readMarkBody(w, r) {
		return
	}
	n, err := h.d.WriteStaffInbox.MarkRead(r.Context(), r.PathValue("id"), owner)
	if err != nil {
		if errors.Is(err, commsstore.ErrStaffNotificationNotFound) {
			// Another person's notice and no notice are one answer — see the file header.
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy thông báo này.", "")
			return
		}
		h.inboxFailure(w, r, "đánh dấu đã đọc", err)
		return
	}
	vietJSON(w, http.StatusOK, notificationToOut(n))
}

// MarkAllMyNotificationsRead serves PATCH /api/v1/notifications with {"read": true} — `Đọc hết`.
func (h *Handler) MarkAllMyNotificationsRead(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.inboxOwner(w, r)
	if !ok {
		return
	}
	if !readMarkBody(w, r) {
		return
	}
	n, err := h.d.WriteStaffInbox.MarkAllRead(r.Context(), owner)
	if err != nil {
		h.inboxFailure(w, r, "đọc hết", err)
		return
	}
	vietJSON(w, http.StatusOK, markAllReadOut{Marked: n})
}

// readMarkBody decodes {"read": true}, answering 400 itself otherwise.
func readMarkBody(w http.ResponseWriter, r *http.Request) bool {
	var in markReadIn
	if !docThan(w, r, &in) {
		return false
	}
	if in.Read == nil || !*in.Read {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Chỉ đánh dấu ĐÃ ĐỌC được: gửi {\"read\": true}. Thông báo đã đọc không đánh dấu lại là chưa đọc.", "")
		return false
	}
	return true
}

// inboxFailure logs the commune — never a title, a body or a code — and answers 500.
func (h *Handler) inboxFailure(w http.ResponseWriter, r *http.Request, what string, err error) {
	if errors.Is(err, app.ErrNoActor) {
		h.d.Log.Error("hộp thông báo: "+what+" không có chủ thể — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(r.Context())))
	} else {
		h.d.Log.Error("hộp thông báo: "+what+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
