// Package authz declares who may call what.
//
// WHY EXPLICIT DECLARATIONS: the global guard rejects users who are not logged in. It does
// NOT check permissions. A route with no declaration is therefore callable by EVERY staff
// role, including roles with nothing to do with that subsystem — and nothing reports it, no
// test turns red. Rule 5 exists because that failure is silent.
//
// Permissions are always evaluated WITHIN one commune. A check that forgets the commune is
// cross-commune privilege escalation, not a lesser bug.
package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// Perm is a permission key: "<nhóm>.<việc>", for example "task.extend".
//
// WHY A FLAT STRING AND NOT (subsystem, action): the administrative permissions this system
// grants are not a Cartesian product. "task.approve" (duyệt hoàn thành) and "task.extend"
// (duyệt gia hạn) are both approvals and are deliberately separate rights — one closes a
// commitment to a citizen, the other moves its deadline. A fixed action set collapses them,
// and a role holding one would silently hold the other.
//
// The keys are the ones the Phân quyền screen shows, so a permission in code, a row in
// `quyen`, and a tick box a commune administrator sees are all the same string. A translation
// layer in between is a place for them to drift.
type Perm string

func (p Perm) String() string { return string(p) }

// Nhom returns the part before the dot: "task.extend" -> "task". Used for grouping on the
// permission matrix, never for deciding access — access is decided by the whole key.
func Nhom(p Perm) string {
	s := string(p)
	if i := strings.IndexByte(s, '.'); i > 0 {
		return s[:i]
	}
	return s
}

// Principal is whoever is making the request.
type Principal struct {
	ID       string
	Kind     string // "staff" | "citizen"
	TenantID tenant.ID
	Roles    []string
}

type ctxKey struct{}

func Into(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// Checker answers whether a principal holds a permission.
//
// The commune is not a parameter: it rides in the context, and an implementation that ignores
// it grants one commune's roles inside another commune (rule 1, invariant 3).
type Checker interface {
	Allows(ctx context.Context, p Principal, perm Perm) bool
}

// xacNhanXa reports whether the principal was issued for the commune this request arrived at.
//
// ONE FUNCTION, NOT A LINE REPEATED PER GUARD: the same invariant asserted in two places drifts,
// and the copy that gets forgotten is the one nobody notices — a missing commune check is
// cross-commune privilege escalation that no test of a single commune can produce (rule 5,
// invariant 3).
//
// A browser does not send a cookie across hosts, so a mismatch is never an ordinary user error:
// it is a deliberate probe or a stolen token.
//
// tenant.MustFrom panics when there is no commune, which is deliberate: a guard that ran without
// one would compare against nothing and let every commune through. It is a precondition of being
// mounted inside httpx.TenantMiddleware, not a case to handle.
func xacNhanXa(ctx context.Context, p Principal) bool {
	return p.TenantID == tenant.MustFrom(ctx)
}

// RequirePermission guards a route. This is the normal case.
func RequirePermission(c Checker, perm Perm) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			p, ok := From(ctx)
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
					"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
				return
			}
			// The commune in the token must match the commune resolved from Host — see
			// xacNhanXa.
			if !xacNhanXa(ctx, p) {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
					"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
				return
			}
			if !c.Allows(ctx, p, perm) {
				httpx.WriteError(w, http.StatusForbidden, "forbidden",
					"Tài khoản của bạn không có quyền thực hiện thao tác này.", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CitizenOnly guards a citizen route. Citizens have no roles; they are isolated by identity
// (rule 4), not by RBAC.
//
// THIS GUARD DOES NOT COMPARE THE COMMUNE, and ADR 0022 is why — it is not an oversight left
// for the next person to close.
//
// The citizen channel is the Zalo Mini App, which has NO DOMAIN (see CLAUDE.md, STACK). There is
// no Host to derive a commune from, so there is nothing to compare the session against: the
// session IS the only source of the commune (httpx.CitizenEdge), and a single source cannot be
// reconciled with itself. Comparing it with anything the client sends would rebuild the door
// rule 1, forbidden #2 closed, in the shape of a check that looks stricter. The equivalent
// comparison lives IN THE FLOW, not at the edge: ADR 0019, invariant 8.
//
// So this guard answers ONE axis — WHO. The commune axis is answered by the class declaration
// on the route, httpx.XaTuPhien() or httpx.KhongThuocXa(reason), and a citizen route declares
// both or apidoc refuses it. A citizen route mounted WITHOUT a commune resolved ahead of it is
// therefore normal and intended for the KhongThuocXa class (the commune picker), not a gap.
//
// Still open, and still the customer's to answer: what happens to a citizen acting with more
// than one commune (rule 4, stop condition #3).
func CitizenOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := From(r.Context())
			if !ok || p.Kind != "citizen" {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
					"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CitizenPrincipal turns the session httpx.CitizenEdge resolved into a Principal.
//
// MỘT LẦN TRA PHIÊN CHO CẢ HAI TRỤC. Rìa công dân đã tra sổ phiên một lần để biết xã; tra lại
// ở đây để biết danh tính là hai lần chạm sổ trên đường nóng của mọi yêu cầu công dân, và hai
// câu trả lời có thể lệch nhau nếu phiên bị thu hồi ở giữa. Một lần tra, hai trục đọc chung.
//
// Nó KHÔNG từ chối khi không có phiên: từ chối là việc của CitizenOnly, và một guard thứ hai
// trả cùng một lỗi ở cùng một chuỗi là hai chỗ để câu trả lời lệch nhau.
//
// Principal ở đây chỉ mang định danh mờ — không số điện thoại, không tên (luật 3). Xem
// httpx.CitizenSession.
func CitizenPrincipal() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := httpx.CitizenSessionFrom(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(Into(r.Context(), Principal{
				ID:       p.CitizenID,
				Kind:     "citizen",
				TenantID: p.TenantID,
			})))
		})
	}
}

// AnyAuthenticated opens a route to every signed-in account. The reason is mandatory and is
// kept in the binary so it can be audited: six months on, nobody dares remove an unexplained
// exemption.
//
// "Every signed-in account" means every account OF THIS COMMUNE. Dropping the permission check
// does not drop the commune check: waiving what a person may do never waives where they may do
// it (rule 5, invariant 3).
func AnyAuthenticated(reason string) func(http.Handler) http.Handler {
	if reason == "" {
		panic("authz: AnyAuthenticated requires a reason")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			p, ok := From(ctx)
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
					"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
				return
			}
			if !xacNhanXa(ctx, p) {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
					"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Public opens a route with no authentication. The reason is mandatory for the same reason.
func Public(reason string) func(http.Handler) http.Handler {
	if reason == "" {
		panic("authz: Public requires a specific reason")
	}
	return func(next http.Handler) http.Handler { return next }
}
