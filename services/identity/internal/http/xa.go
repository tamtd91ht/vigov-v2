package http

import (
	"net/http"
	"strings"

	"github.com/vihat/vigov/pkg/httpx"
	"github.com/vihat/vigov/pkg/tenant"
)

// The read route behind the sign-in screen. GET /api/v1/commune
//
// It is the ONE public route in this service that returns data, and every decision below exists
// because of that. Anything it returns is readable by anybody who can reach the domain.

// thongTinXa is the commune as the sign-in screen sees it.
//
// THERE IS NO `id` FIELD, AND ADDING ONE IS THE MISTAKE THIS COMMENT EXISTS TO PREVENT. The
// value the handler reads it from — tenant.Tenant — carries `ID` right next to `Name`, so the
// field will be sitting there in front of whoever edits this next. It stays out because:
//
//   - the web does not need it. The commune is derived server-side from `Host` on EVERY request
//     (rule 1, invariant 3), so a client has nothing to do with the value;
//   - a client that sent one back would be ignored anyway — naming your own commune is granting
//     yourself access (rule 1, forbidden #2). So it can only ever be decoration;
//   - it is the opaque identifier the whole isolation model rests on, and it is what prefixes
//     cache keys, queue messages, realtime rooms and file paths (rule 1, invariant 7).
//     Publishing it on an UNAUTHENTICATED route hands that prefix to anyone with curl, in
//     exchange for nothing.
//
// THERE IS NO `active` FIELD EITHER, and that is a different reason: it could only ever be
// `true` here. httpx.TenantMiddleware answers 404 for a deactivated commune before any handler
// runs (pkg/httpx/edge.go:25), so this code is unreachable for one. A field that is constant by
// construction invites the web to build a "this commune has merged" branch that never executes —
// and a merged commune IS a real state (rule 1, invariant 6: marked inactive, never deleted).
// Answering it properly means the edge saying something other than a bare 404, which is a change
// in pkg/httpx that every service shares. STATED, not silently half-done here.
//
// THERE IS NO `parent_authority` FIELD, although the admin web already asks for one
// (apps/commune-admin/src/components/cau-hinh-xa.tsx reads `parentAuthority` — "Thành phố Đà
// Nẵng" under the commune name, docs/ui-ux/15 §3). THE DATA DOES NOT EXIST: pkg/tenant.Tenant is
// {ID, Host, Name, Active} and `message Tenant` in proto/vigov/platform/v1 is {id, host,
// display_name}. Adding it is a change to the contract BETWEEN services, which is the
// contract-designer's surface and is generated from .proto, never written by hand (rule 2,
// invariant 7). Returning "" would be worse than returning nothing: the screen would print an
// empty line under the commune name and nobody would know whether the authority has no parent or
// the field was never filled in.
type thongTinXa struct {
	// Name is the display name AT THIS MOMENT, not an identifier — a commune can be renamed by
	// an administrative reorganisation while its tenant_id stays put, which is the whole point of
	// the id being opaque (rule 1, invariant 2).
	Name string `json:"name"`

	// Host is the commune's canonical domain as the registry holds it. It is what the caller
	// should be on; it is not necessarily the string the caller typed.
	Host string `json:"host"`
}

// ThongTinXa serves the commune this request's Host resolves to. GET /api/v1/commune
//
// THE COMMUNE IS NEVER TAKEN FROM THE REQUEST. It is the one the EDGE resolved from `Host` and
// put in the context; the query string, the body and any client header are not read here at all,
// and pkg/httpx.StripTenantHeaders has already removed the latter (rule 1, forbidden #2).
//
// NO AUDIT ENTRY: nothing is written, nothing personal is read, and nothing crosses a commune —
// the two cases rule 6, invariant 7 asks for. An entry per hit on the sign-in screen would bury
// the entries that carry legal weight under noise from unauthenticated traffic.
func (h *Handler) ThongTinXa(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	xa := tenant.MustFrom(ctx)

	// A SECOND LOOKUP, AGAINST THE SAME DIRECTORY THE EDGE USED — deliberately the same value
	// (services/identity/cmd/server/main.go), never a second source. The edge puts only the
	// commune's ID into the context, and this route needs its name; asking the platform registry
	// is the only way to learn one (rule 2: this service does not read the registry's tables).
	t, ok := h.d.Xa.ByHost(ctx, hostYeuCau(r))
	switch {
	case !ok:
		// The edge resolved this Host moments ago, so reaching here means the platform service
		// became unreachable in between, or the cache entry expired into an outage. FAIL CLOSED:
		// no name is served rather than a stale or guessed one.
		h.d.Log.Error("cấu hình xã: không đọc được tên xã cho host này — dịch vụ nền tảng có vấn đề",
			"xa", string(xa), "host", hostYeuCau(r))

	case t.ID != xa:
		// One Host answering with two different communes inside one request. A domain reassigned
		// between two lookups microseconds apart is the innocent reading and it is vanishingly
		// rare; either way the only safe answer is to serve neither name. Printing commune B's
		// name on commune A's domain is a breach between two authorities, on a public route.
		h.d.Log.Warn("CẢNH BÁO: cùng một Host phân giải ra hai xã khác nhau trong cùng một yêu cầu",
			"xa_theo_bien", string(xa), "xa_theo_tra_lai", string(t.ID), "host", hostYeuCau(r))

	default:
		vietJSON(w, http.StatusOK, thongTinXa{Name: t.Name, Host: t.Host})
		return
	}

	// 503 AND NOT 500: this service is healthy and the request was well formed — what failed is
	// a dependency, and the sign-in screen should retry rather than send somebody to read
	// identity's logs. Not 404 either: the edge has just established that this commune exists, so
	// saying it does not would be a lie a client caches.
	httpx.WriteError(w, http.StatusServiceUnavailable, "tenant_unavailable",
		"Chưa đọc được thông tin xã. Vui lòng thử lại.", "")
}

// hostYeuCau normalises `Host` exactly the way the edge does — lower-cased, port removed
// (pkg/httpx/edge.go:22).
//
// IT IS A COPY OF A NORMALISATION RULE, which is normally how two places drift apart. What makes
// it safe here is the comparison in ThongTinXa above: a Host normalised differently resolves to
// a different commune, or to none, and both of those are refused. Drift can therefore only ever
// produce a 503 — never one commune's name served on another commune's domain.
//
// THE CLEAN FIX IS ONE RESOLUTION, NOT TWO: the edge carrying the whole tenant.Tenant in the
// context instead of only its id, so no handler ever resolves anything. That is a change inside
// pkg/httpx, which all eight services share — outside this service, and stated rather than done.
func hostYeuCau(r *http.Request) string {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.ToLower(host)
}
