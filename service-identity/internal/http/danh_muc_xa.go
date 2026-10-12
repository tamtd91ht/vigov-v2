package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The Mini App's commune confirmation. GET /api/v1/communes?host=<domain> | ?code=<launch code>
//
// The shared Mini App opens from a QR whose parameter carries a commune's DOMAIN (owner's decision
// 2026-09-27). Before the citizen confirms "Làm việc với xã X?", the app needs X's display name.
// This route answers that and nothing more.
//
// THE DOMAIN IS A LOOKUP KEY AND NOTHING ELSE. It is not stored, it opens no session, and it grants
// nothing: the commune enters a session only by the citizen's explicit act through the session
// bridge (ADR 0005 · 0019 · 0022), never because this route answered. A QR parameter steers the
// interface; it is client-supplied data (rule 1, forbidden #2).
//
// THE LAUNCH CODE IS THE SAME KIND OF KEY (owner decision 2026-10-12). Commune QR links now carry
// `?x=<launch code>` instead of the domain, so the link does not show the domain. The Mini App resolves
// the code ONCE through `?code=` to the commune's name, province and PRIMARY host, then uses that host
// everywhere it used the QR's domain before. The code grants nothing either: no session, no tenant id.

// TraXaTheoHost is the one platform read the public surface makes. *platformclient.Directory
// satisfies it.
//
// A NARROW INTERFACE, NOT tenant.Directory: that one's bool folds an outage into "unknown Host",
// which is right for the Host edge and wrong here — see platformclient.XaTheoHost.
//
// CommuneByHost rather than XaTheoHost: the same lookup with the commune's launch code kept, which
// the `?host=` answer publishes. ONE host method, so every public route resolves a host the same way.
type TraXaTheoHost interface {
	CommuneByHost(ctx context.Context, host string) (platformclient.Commune, bool, error)
	// CommuneByLaunchCode: same three outcomes; Host is the commune's PRIMARY host, "" if reserved.
	CommuneByLaunchCode(ctx context.Context, code string) (tenant.Tenant, bool, error)
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
// `host` ONLY IN THE `?code=` ANSWER. Asked by host, the caller already holds the domain, and echoing
// the registry's canonical form would hand out which of a commune's domains is the primary one —
// something that screen does not need. Asked by code, the caller holds NO domain, and the primary host is
// exactly what it needs: every later public read (`commune-profiles`, `commune-staff`, the session
// bridge) is keyed by host (owner decision 2026-10-12, "only the link changes"). omitempty keeps the
// `?host=` answer byte-identical to what it was; the code path never emits "" (see communeByCode).
//
// `launch_code` ONLY IN THE `?host=` ANSWER (owner decision 2026-10-12, ADR 0047 §Sửa đổi 12/10/2026
// row 5). The QR CLI (citizen-app/scripts/commune-qr.mjs, `npm run qr --domain=`) learns from it which
// code to print as `?x=`. Asked by code, the caller already holds the code, so it is not echoed. The
// code is public — it is printed on the QR — and grants nothing (proto Tenant.launch_code). omitempty:
// a platform that sends none (an older service-platform) gives no key, never an invented code; the
// CLI treats its absence as "write the legacy d= QR and warn".
//
// NO `active` FIELD: an inactive commune is never returned (see DanhMucXa), so it could only ever be
// true.
type xaCongKhai struct {
	// Name is the display name at this moment — a commune can be renamed while its id stays put.
	Name string `json:"name"`

	// Province: "" means not declared; the screen renders nothing (same contract as thongTinXa).
	Province string `json:"province"`

	// Host: the commune's primary host, `?code=` answers only. Absent from `?host=` answers.
	Host string `json:"host,omitempty"`

	// LaunchCode: tenant.launch_code, `?host=` answers only. Absent from `?code=` answers, and absent
	// when the platform sent "".
	LaunchCode string `json:"launch_code,omitempty"`
}

// HandlerCongKhai serves the PUBLIC routes of this service. A SEPARATE TYPE from Handler, with its own
// Deps, so a public route cannot reach a staff store even by typing it — the arrangement rule 4,
// invariant 5 asks of citizen routes, applied to routes that are even less trusted.
type HandlerCongKhai struct {
	d DepsCongKhai
	// limiter guards POST /api/v1/citizen-sessions only (rate_limit.go). One per mux, so one per pod.
	limiter *ipRateLimiter
}

// DanhMucXa answers which commune, if any, a domain or a launch code belongs to.
// GET /api/v1/communes?host= | ?code=
//
// EXACTLY ONE OF `host` / `code`, each at most once. Both, or neither, is 400: the caller is never
// answered by picking one of the two for it — the same rule service-platform's ResolveHost applies.
//
// ORDER IS THE DESIGN: the shape of the host (or code) is checked BEFORE the platform is asked. A malformed
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
	q := r.URL.Query()
	gia, hasHost := q["host"]
	codes, hasCode := q["code"]
	switch {
	case hasHost && hasCode:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query",
			"Chỉ gửi một trong hai tham số: tên miền hoặc mã xã.", "")
		return
	case hasCode:
		h.communeByCode(w, r, codes)
		return
	}
	// Neither parameter falls through to here and is refused as a missing host — the answer it got
	// before `code` existed.
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}

	xa, co, ok := h.communeByHost(w, r, gia[0], "danh mục xã")
	if !ok {
		return
	}
	ra := danhMucXaRa{Items: []xaCongKhai{}} // `[]`, never `null`: one shape for the client
	if co {
		ra.Items = append(ra.Items, xaCongKhai{Name: xa.Name, Province: xa.Province, LaunchCode: xa.LaunchCode})
	}
	vietJSON(w, http.StatusOK, ra)
}

// communeByCode is the `?code=` half of DanhMucXa: the commune a QR's launch code names, with its
// primary host.
//
// THE SAME NEGATIVE AS `?host=`: unknown, inactive, and a commune whose primary host the platform blanked
// (reserved) are all `{"items": []}`, byte for byte. A blank host is "no commune" rather than a commune
// with no host, because a commune the Mini App cannot address by host is one it cannot open — and it
// must never fall back to another of the commune's hosts (rule 1, forbidden #1).
func (h *HandlerCongKhai) communeByCode(w http.ResponseWriter, r *http.Request, codes []string) {
	if len(codes) != 1 || !validLaunchCode(codes[0]) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_code", "Mã xã không hợp lệ.", "")
		return
	}
	code := codes[0]
	xa, found, err := h.d.Xa.CommuneByLaunchCode(r.Context(), code)
	if err != nil {
		// The code is a public, opaque 12-hex key that passed validLaunchCode — not personal data and
		// not a log-injection payload.
		h.d.Log.WarnContext(r.Context(), "danh mục xã theo mã: không hỏi được dịch vụ nền tảng", "code", code, "err", err)
		viet503Platform(w)
		return
	}
	ra := danhMucXaRa{Items: []xaCongKhai{}} // `[]`, never `null`
	if found && xa.Active && xa.ID.Valid() && xa.Host != "" {
		if domain.HopLeTenMienXa(xa.Host) {
			ra.Items = append(ra.Items, xaCongKhai{Name: xa.Name, Province: xa.Province, Host: xa.Host})
		} else {
			// The registry handed back a primary host the session bridge would refuse. Answering it
			// would open a commune the next request cannot reach: fail closed, and say so.
			h.d.Log.WarnContext(r.Context(), "danh mục xã theo mã: nền tảng trả tên miền chính sai hình dạng", "code", code)
		}
	}
	vietJSON(w, http.StatusOK, ra)
}

// validLaunchCode is the launch code's shape, `^[0-9a-f]{12}$` (proto ResolveHostRequest.launch_code).
// Checked here so junk never costs a platform call.
func validLaunchCode(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
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
	xa, co, ok := h.communeByHost(w, r, host, viec)
	return xa.Tenant, co, ok
}

// communeByHost is xaTheoHost with the commune's launch code kept — the body both share, so the three
// outcomes cannot drift between the catalogue answer and the routes that read business data.
func (h *HandlerCongKhai) communeByHost(w http.ResponseWriter, r *http.Request, host, viec string) (platformclient.Commune, bool, bool) {
	xa, co, err := h.d.Xa.CommuneByHost(r.Context(), host)
	if err != nil {
		h.d.Log.WarnContext(r.Context(), viec+": không hỏi được dịch vụ nền tảng", "host", host, "err", err)
		viet503Platform(w)
		return platformclient.Commune{}, false, false
	}
	if !co || !xa.Active || !xa.ID.Valid() {
		// An invalid id cannot come back from platformclient (it refuses non-ULIDs); checked anyway
		// because tenant.Into with an empty id would be a scoped read of "no commune" (fail closed).
		return platformclient.Commune{}, false, true
	}
	return xa, true, true
}

// viet503Platform is the ONE answer for an unreachable platform registry on the public surface.
func viet503Platform(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
		"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
}

// newHandlerCongKhai defaults the logger the same way NewHandler does.
func newHandlerCongKhai(d DepsCongKhai) *HandlerCongKhai {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &HandlerCongKhai{d: d,
		limiter: newIPRateLimiter(citizenSessionLimit, citizenSessionWindow, rateLimitMaxKeys, time.Now)}
}
