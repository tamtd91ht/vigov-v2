package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The Mini App's commune confirmation. GET /api/v1/communes?host=<domain>
//
// The shared Mini App opens from a QR whose parameter carries a commune's DOMAIN (owner's decision
// 2026-09-27). Before the citizen confirms "Làm việc với xã X?", the app needs X's display name.
// This route answers that and nothing more.
//
// THE DOMAIN IS A LOOKUP KEY AND NOTHING ELSE. It is not stored, it opens no session, and it grants
// nothing: the commune enters a session only by the citizen's explicit act through the session
// bridge (ADR 0005 · 0019 · 0022), never because this route answered. A QR parameter steers the
// interface; it is client-supplied data (rule 1, forbidden #2).

// TraXaTheoHost is the one platform read the public surface makes. *platformclient.Directory
// satisfies it.
//
// A NARROW INTERFACE, NOT tenant.Directory: that one's bool folds an outage into "unknown Host",
// which is right for the Host edge and wrong here — see platformclient.XaTheoHost.
type TraXaTheoHost interface {
	XaTheoHost(ctx context.Context, host string) (tenant.Tenant, bool, error)
}

// danhMucXaRa is the catalogue answer: ZERO or ONE commune.
//
// A LIST AND NOT ONE OBJECT, because `communes` is the collection (kb/00-foundation/
// ubiquitous-language.md §Kênh công dân, "Danh mục xã cho Mini App") and `host` is a filter on it.
// The list also gives the three negatives their one shape for free: unknown, reserved and inactive
// are all `{"items": []}`, byte for byte.
//
// NO CURSOR, and skills/rest-api-design REQUIRED #7 is still met: the filter is an exact match on a
// Host, which the registry holds once, so the list is bounded at one by construction. The day the
// filter becomes optional (the commune picker), pagination arrives with it.
type danhMucXaRa struct {
	Items []xaCongKhai `json:"items"`
}

// xaCongKhai is one commune as anybody who knows its domain may see it.
//
// THERE IS NO `id` FIELD — the reason is on thongTinXa (xa.go) and on core/grpcx's
// methodsWithoutTenant: the tenant ULID prefixes every cache key, queue, room and file path (rule 1,
// invariant 7), and a public reader has nothing to do with it. The app sends the DOMAIN back to the
// session bridge when the citizen confirms; the bridge resolves it again on the server side.
//
// THERE IS NO `host` FIELD EITHER, unlike thongTinXa: the caller already holds the domain it asked
// about, and echoing the registry's canonical form would hand out, per query, which of a commune's
// domains is the primary one — something the confirmation screen does not need.
//
// NO `active` FIELD: an inactive commune is never returned (see DanhMucXa), so it could only ever be
// true.
type xaCongKhai struct {
	// Name is the display name at this moment — a commune can be renamed while its id stays put.
	Name string `json:"name"`

	// Province: "" means not declared; the screen renders nothing (same contract as thongTinXa).
	Province string `json:"province"`
}

// HandlerCongKhai serves the PUBLIC routes of this service. A SEPARATE TYPE from Handler, with its own
// Deps, so a public route cannot reach a staff store even by typing it — the arrangement rule 4,
// invariant 5 asks of citizen routes, applied to routes that are even less trusted.
type HandlerCongKhai struct {
	d DepsCongKhai
}

// DanhMucXa answers which commune, if any, a domain belongs to. GET /api/v1/communes?host=
//
// ORDER IS THE DESIGN: the shape of the host is checked BEFORE the platform is asked. A malformed
// value is the caller's mistake and costs nothing; sending it on would spend a platform call per junk
// request on a route anybody on the internet can reach. The shape is domain.HopLeTenMienXa — the SAME
// function the session bridge applies to `commune_host_hint`, so this screen can never name a commune
// for a value the bridge then refuses.
//
// ONE ANSWER FOR UNKNOWN, RESERVED AND INACTIVE: 200 `{"items": []}`. service-platform already
// answers reserved Hosts with the same NotFound as unclaimed ones (internal/grpc/server.go:96-98);
// inactive is folded in HERE (xaTheoHost). Telling them apart would let anybody map which domains are
// the platform's and which communes have merged away — the probe core/httpx/edge.go:26-34 refuses on
// the staff edge, for the same reason.
//
// PLATFORM UNREACHABLE: 503. Never an empty list (that reads as "this QR is wrong"), and never a
// cached or remembered commune (rule 1, forbidden #1).
//
// NO CACHE: a cache here is a second place where a commune's deactivation arrives late, and the
// platform directory read is one indexed lookup. Add one only after it is measured.
//
// NO AUDIT ENTRY: nothing is written, nothing personal is read (a commune's name and province are
// public metadata, not rule 3 data), and no commune's business data is touched — rule 6, invariant 7
// asks for neither case. This handler never puts the commune into the context, so store.For(ctx)
// would panic if anything here tried.
func (h *HandlerCongKhai) DanhMucXa(w http.ResponseWriter, r *http.Request) {
	gia := r.URL.Query()["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}

	xa, co, ok := h.xaTheoHost(w, r, gia[0], "danh mục xã")
	if !ok {
		return
	}
	ra := danhMucXaRa{Items: []xaCongKhai{}} // `[]`, never `null`: one shape for the client
	if co {
		ra.Items = append(ra.Items, xaCongKhai{Name: xa.Name, Province: xa.Province})
	}
	vietJSON(w, http.StatusOK, ra)
}

// viet400Host is the ONE refusal of a malformed `host`, shared by every public route so the Mini App
// meets one wording. The body never echoes what was sent.
func viet400Host(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_host", "Tên miền của xã không hợp lệ.", "")
}

// xaTheoHost resolves an ALREADY-VALIDATED host to the commune the platform says holds it — the one
// place the public surface learns which commune a request is about.
//
// THREE OUTCOMES, and the caller must tell them apart:
//
//	ok=false          503 already written: the platform could not be asked. Never "no commune".
//	ok=true, co=false no ACTIVE commune holds this host — unknown, reserved and inactive alike. The
//	                  caller answers its own "nothing here" shape, identical for all three.
//	ok=true, co=true  xa is the active commune; a route that reads business data puts xa.ID into the
//	                  context with tenant.Into and reads through a scoped store.
//
// THE COMMUNE COMES FROM THE PLATFORM REGISTRY, SERVER-SIDE, NEVER FROM THE CLIENT. The host is a
// public name printed on a QR and on the commune's own website; resolving it is exactly what the staff
// edge does with `Host` (rule 1, invariant 3), just read from a parameter because the Mini App has no
// domain of its own. No tenant_id is ever accepted from the request (rule 1, forbidden #2).
//
// NO PERSONAL DATA IN THE LOG LINE: the host is a domain, not a person, and it has passed
// domain.HopLeTenMienXa, so it cannot carry a log-injection payload either.
func (h *HandlerCongKhai) xaTheoHost(w http.ResponseWriter, r *http.Request, host, viec string) (tenant.Tenant, bool, bool) {
	xa, co, err := h.d.Xa.XaTheoHost(r.Context(), host)
	if err != nil {
		h.d.Log.WarnContext(r.Context(), viec+": không hỏi được dịch vụ nền tảng", "host", host, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
			"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
		return tenant.Tenant{}, false, false
	}
	if !co || !xa.Active || !xa.ID.Valid() {
		// An invalid id cannot come back from platformclient (it refuses non-ULIDs); checked anyway
		// because tenant.Into with an empty id would be a scoped read of "no commune" (fail closed).
		return tenant.Tenant{}, false, true
	}
	return xa, true, true
}

// newHandlerCongKhai defaults the logger the same way NewHandler does.
func newHandlerCongKhai(d DepsCongKhai) *HandlerCongKhai {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &HandlerCongKhai{d: d}
}
