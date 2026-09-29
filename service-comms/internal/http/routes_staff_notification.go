package http

// The header-bell routes — the signed-in staff member's OWN inbox (docs/ui-ux/08-thong-bao.md §8).
// Called from Register; in their own file so the four declarations sit together with their reasons.
//
// `notifications` IS THE SETTLED NOUN: the third meaning of `thông báo` in
// kb/00-foundation/ubiquitous-language.md:151 (`announcements` · `public-notices` · `notifications`).
//
// # AnyAuthenticated, AND THE REASON IS THAT THE DATA IS THE CALLER'S OWN
//
// The `quyen` table has no key for "read my own bell", and none is invented (rule 5, invariant 3c).
// The alternative keys would each be wrong: any existing key would hide the bell from every staff
// member who lacks it, while the notices in it are addressed to them BY NAME by another service.
// ../vigov-require reached the same answer (apps/api/app/modules/notifications/router.py:23-35: "No
// permission gate: a notification is addressed to one person and the query is scoped to that
// person, so there is nothing to withhold").
//
// WHAT AnyAuthenticated WAIVES AND WHAT IT DOES NOT: it waives the permission check only. The commune
// check stays (a session of another commune is 401), and every statement behind these routes is
// filtered by the session's own staff code — so "every signed-in account" can read exactly ONE
// inbox, its own. There is deliberately no 403-for-a-missing-key line: there is no key to miss. 403
// is answered to a non-staff principal (internal/http/staff_notification.go, inboxOwner).

import (
	"context"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// StaffInboxReader is the READ half — two store calls, each filtered by the recipient code the
// handler takes from the session. *store.StaffNotificationStore satisfies it.
type StaffInboxReader interface {
	ListOwn(ctx context.Context, recipientCode string, req page.Request) (page.Result[domain.StaffNotification], error)
	CountUnread(ctx context.Context, recipientCode string) (int, error)
}

// StaffInboxWriter is the WRITE half — each opens a transaction and files its audit entry inside it
// (rule 6, invariant 3). The actor IS the recipient. *app.StaffNotifications satisfies it.
type StaffInboxWriter interface {
	MarkRead(ctx context.Context, id string, actor audit.Actor) (domain.StaffNotification, error)
	MarkAllRead(ctx context.Context, actor audit.Actor) (int, error)
}

// registerStaffNotificationRoutes mounts the four bell routes. Register has already refused a nil
// StaffInbox / WriteStaffInbox.
func registerStaffNotificationRoutes(mux *http.ServeMux, h *Handler) {
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Hộp thông báo ở chuông của CHÍNH cán bộ đang đăng nhập — mới nhất trước, có con trỏ trang
	// @screen   15-phu-luc-giao-dien-chung §3.2
	// @reply    200 page.Result[notificationOut]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/notifications",
		authz.AnyAuthenticated("hộp chuông là thông báo gửi ĐÍCH DANH cho chính cán bộ đang đăng nhập; mọi câu đọc lọc theo mã cán bộ của phiên nên mỗi tài khoản chỉ đọc được hộp thư của mình, và bảng quyen không có khoá nào cho việc này — đòi một khoá có sẵn sẽ giấu thông báo khỏi đúng người được gửi")(
			http.HandlerFunc(h.ListMyNotifications)))

	// The badge, asked on every page load. Its own route so it stays one indexed count.
	//
	// @summary  Số thông báo chưa đọc trong hộp chuông của chính cán bộ đang đăng nhập — cho huy hiệu đỏ
	// @screen   15-phu-luc-giao-dien-chung §3.2
	// @reply    200 unreadCountOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/notifications/unread-count",
		authz.AnyAuthenticated("huy hiệu chuông đếm thông báo chưa đọc của chính cán bộ đang đăng nhập, lọc theo mã cán bộ của phiên; không có khoá quyền nào cho việc này trong bảng quyen")(
			http.HandlerFunc(h.CountMyUnreadNotifications)))

	// Opening one item. PATCH with {"read": true}: `read` is the only field, and false is refused.
	//
	// idem.KhongCan: marking a read notice read writes nothing and files nothing (app.MarkRead), so the
	// second request leaves one `read_at` and one entry.
	//
	// 404 covers "no such notice", "another person's" and "another commune's", indistinguishably.
	//
	// @summary  Đánh dấu đã đọc một thông báo trong hộp chuông của chính mình
	// @screen   15-phu-luc-giao-dien-chung §3.2
	// @request  markReadIn
	// @reply    200 notificationOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/notifications/{id}",
		authz.AnyAuthenticated("cán bộ đánh dấu đã đọc thông báo gửi cho chính mình; câu ghi lọc theo mã cán bộ của phiên nên không chạm được hộp thư của người khác")(
			idem.KhongCan("đánh dấu một thông báo đã đọc lần thứ hai không ghi gì và không có vết: read_at chỉ đặt một lần")(
				http.HandlerFunc(h.MarkMyNotificationRead))))

	// `Đọc hết`. PATCH on the collection with {"read": true}: every unread notice of the caller.
	//
	// idem.KhongCan: the second request finds nothing unread, writes nothing and files nothing.
	//
	// @summary  Đọc hết — đánh dấu đã đọc mọi thông báo chưa đọc trong hộp chuông của chính mình
	// @screen   15-phu-luc-giao-dien-chung §3.2
	// @request  markReadIn
	// @reply    200 markAllReadOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/notifications",
		authz.AnyAuthenticated("nút Đọc hết của chuông chỉ đánh dấu thông báo gửi cho chính cán bộ đang đăng nhập, lọc theo mã cán bộ của phiên")(
			idem.KhongCan("lần thứ hai không còn thông báo chưa đọc nào nên không ghi gì và không có vết")(
				http.HandlerFunc(h.MarkAllMyNotificationsRead))))
}
