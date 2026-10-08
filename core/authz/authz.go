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
//
// # TWO IDENTIFIERS FOR A STAFF PRINCIPAL, AND THE QUESTION "WHY NOT ONE" IS ANSWERED HERE
//
// ID was made to carry the business code instead of the internal id on 2026-09-22 and the change
// was REJECTED, on a measurement rather than a preference. ID is the value every access decision
// is made with, in three places that all match on the internal id:
//
//	service-identity/internal/store/checker.go:124   `nd.id = $2`, the grant query itself
//	core/staffauth/staffauth.go:294                  binds one request's key set to one person
//	core/idem/idem.go:506                            the idempotency key's owner
//
// A business code in ID matches no row in the first, so EVERY permission check answers false and
// EVERY guarded route in four services returns 403 — with nothing in the response, the logs or a
// test to point at the cause, because a fake checker in a test grants whatever it is given.
//
// So the two identifiers answer two questions, and neither can answer the other's:
//
//	ID   DECIDES ACCESS.          Internal, opaque, joins to `nguoi_dung.id`. Never on the trail.
//	Ma   RECORDS RESPONSIBILITY.  The business code. Never compared, never joined, never a filter.
//
// CARRYING BOTH COSTS A CHOICE THE NEXT WRITER HAS TO MAKE CORRECTLY, and that cost is real — it
// is the whole reason `.claude/hooks/audit_actor_guard.py` and `tools/check_audit_actor.py` exist.
// The rejected alternative costs more: one field named `ID` holding a business code is a naming
// trap that reads as correct at every call site, and the failure it produces is a silent 403 for
// everybody rather than a guard saying which field to use.
type Principal struct {
	// ID is the INTERNAL identifier. For staff it is `nguoi_dung.id` (a ULID); for a citizen it
	// is the opaque citizen id. It is what authorisation is decided with — see the type comment.
	//
	// IT IS NOT WHAT AN AUDIT ENTRY RECORDS for a staff actor. Use Ma. A ULID on an archival
	// record means nothing to the person reading it years later during an inspection, and the
	// lookup that could translate it may no longer hold the row.
	ID string

	// Ma is the BUSINESS CODE of a staff member — `CB-00123`, `nguoi_dung.ma`, the string the
	// Danh bạ cán bộ screen shows. It is the ONE value `audit_log.actor_id` holds for a staff
	// actor (rule 6, invariant 2; the policy was written at
	// service-identity/internal/app/dang_nhap.go:151 long before anything enforced it).
	//
	// EMPTY FOR A CITIZEN PRINCIPAL, and that is not an omission: a citizen has no staff code,
	// and a citizen's audit entry records Principal.ID — an opaque citizen id — with
	// Actor.Kind = "citizen" (a Zalo-account principal likewise: its account id, with Kind
	// KindZaloAccount, ADR 0080). The kinds are told apart by Kind, never by guessing.
	//
	// EMPTY IS NEVER PAPERED OVER WITH ID. A write path that finds this empty must REFUSE the
	// write (core/audit.Entry.validate already refuses an empty actor). Falling back to ID would
	// put the internal id back in the column and recreate exactly the defect measured on
	// 2026-09-22, silently, with every test still green.
	Ma string

	Kind     string // "staff" | KindCitizen | KindZaloAccount
	TenantID tenant.ID
	Roles    []string
}

// KindCitizen is the Kind of a principal whose session carries a VERIFIED phone: ID is the opaque
// citizen id (`dinh_danh_cong_dan`), the one value a citizen-owned record stores as its owner.
const KindCitizen = "citizen"

// KindZaloAccount is the Kind of a principal whose session has NO verified phone but was opened by a
// Zalo account through the Mini App bridge: ID is `tai_khoan_zalo.id` (ADR 0080).
//
// A DISTINCT KIND AND NOT "citizen" WITH ANOTHER ID, on purpose: every citizen path written before
// ADR 0080 tests `Kind == "citizen"` before it stores Principal.ID as a citizen id (gui_phan_anh.go,
// petition_photo.go, petition_rating.go, phieu_cua_toi.go in service-petitions). Under one shared
// Kind a Zalo account id would pass those tests and land in `cong_dan_id` — a petition owned by a
// citizen who does not exist, readable by nobody, or by the wrong person. A new Kind makes each of
// them refuse it with no edit, and only code written for ADR 0080 accepts it.
//
// It is also audit.KindZaloAccount, the actor_kind of the entries such a principal writes; the two
// constants are pinned equal by a test.
const KindZaloAccount = "zalo-account"

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
	return requireOneOf(c, []Perm{perm})
}

// RequireAnyPermission guards a route that a holder of ANY of the listed keys may call.
//
// WHY IT EXISTS: one route can serve two screens whose specifications name different keys. The
// capital-plan category writes are the case that created it (owner decision, 07/10/2026): the
// Cấu hình tab manages the catalogue under `admin.lookup`, while the Giải ngân screen's Hạng mục
// dialog writes the same rows under `budget.update`, as the prototype shows. Picking one key locks
// the other screen out with a 403 nobody can grant around; duplicating the route makes two
// contracts for one resource.
//
// IT IS AN OR OF WHOLE KEYS, NEVER A WIDENING: each key is still a flat key checked as a whole
// (Perm's comment), the commune is still checked first and identically (xacNhanXa), and a missing
// principal is still 401. A holder of neither key gets the same 403 RequirePermission gives.
//
// Fewer than two keys, or an empty key, panics at wiring time. An empty list must never mean
// "every signed-in account" — that is AnyAuthenticated, which demands a reason — and one key is
// RequirePermission's spelling; two spellings of one declaration are two things to audit.
func RequireAnyPermission(c Checker, perms ...Perm) func(http.Handler) http.Handler {
	if len(perms) < 2 {
		panic("authz: RequireAnyPermission needs at least two keys — one key is RequirePermission")
	}
	for _, p := range perms {
		if p == "" {
			panic("authz: RequireAnyPermission given an empty key")
		}
	}
	return requireOneOf(c, append([]Perm(nil), perms...))
}

// requireOneOf is the single body behind both permission guards, so the 401/403 and commune
// semantics cannot drift between them.
func requireOneOf(c Checker, perms []Perm) func(http.Handler) http.Handler {
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
			for _, perm := range perms {
				if c.Allows(ctx, p, perm) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.WriteError(w, http.StatusForbidden, "forbidden",
				"Tài khoản của bạn không có quyền thực hiện thao tác này.", "")
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
//
// IT ADMITS BOTH CITIZEN-CHANNEL KINDS, KindCitizen AND KindZaloAccount, and that admits NO session
// it did not admit before ADR 0080: a session without a verified phone already passed here, as Kind
// "citizen" with an empty ID. What changed is only that such a session now names its Zalo account
// instead of nobody. The verified-phone wall was never this guard — it is the class: XaTuPhien still
// refuses that session, XaTuPhienChiXem still serves it, and only CommuneFromSessionOrZaloAccount
// lets it own a record. Refusing KindZaloAccount here would instead turn every view-only route (the
// field catalogue the unverified petition form needs) into a 401, and a 401 makes the Mini App reopen
// a session, get the same one back, and loop.
func CitizenOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := From(r.Context())
			if !ok || (p.Kind != KindCitizen && p.Kind != KindZaloAccount) {
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
//
// WHICH IDENTIFIER BECOMES THE PRINCIPAL (ADR 0080):
//
//	CitizenID set                        KindCitizen,     ID = CitizenID (wins over a Zalo account)
//	CitizenID empty, ZaloAccountID set   KindZaloAccount, ID = ZaloAccountID
//	both empty                           KindCitizen,     ID = "" — unchanged from before ADR 0080,
//	                                     and every owner check already refuses an empty ID
func CitizenPrincipal() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := httpx.CitizenSessionFrom(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(Into(r.Context(), principalOfSession(p))))
		})
	}
}

func principalOfSession(s httpx.CitizenSession) Principal {
	if strings.TrimSpace(s.CitizenID) == "" && strings.TrimSpace(s.ZaloAccountID) != "" {
		return Principal{ID: s.ZaloAccountID, Kind: KindZaloAccount, TenantID: s.TenantID}
	}
	return Principal{ID: s.CitizenID, Kind: KindCitizen, TenantID: s.TenantID}
}

// CitizenChannelOwner returns the principal that OWNS what a citizen-channel request writes or reads,
// and which kind of owner it is: Kind is KindCitizen (ID = citizen id) or KindZaloAccount (ID =
// `tai_khoan_zalo.id`, ADR 0080). ok=false means there is no owner — no principal, a staff principal,
// or an empty ID — and the caller must REFUSE; nothing may be put in the owner's place.
//
// The caller switches on Kind. A handler serving CommuneFromSessionOrZaloAccount that does not switch
// is a handler that stores a Zalo account id as a citizen id.
func CitizenChannelOwner(ctx context.Context) (Principal, bool) {
	p, ok := From(ctx)
	if !ok || strings.TrimSpace(p.ID) == "" {
		return Principal{}, false
	}
	if p.Kind != KindCitizen && p.Kind != KindZaloAccount {
		return Principal{}, false
	}
	return p, true
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
