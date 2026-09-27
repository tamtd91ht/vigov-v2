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

// TraXaTheoHost is the one platform read this route makes. *platformclient.Directory satisfies it.
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

// xaCongKhai is one commune as a citizen who does not belong to it yet may see it.
//
// THERE IS NO `id` FIELD — the reason is on thongTinXa (xa.go) and on core/grpcx's
// methodsWithoutTenant: the tenant ULID prefixes every cache key, queue, room and file path (rule 1,
// invariant 7), and a citizen channel has nothing to do with it. The app sends the DOMAIN back to
// the session bridge when the citizen confirms; the bridge resolves it again on the server side.
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

// HandlerCongDan serves the citizen routes of this service. A SEPARATE TYPE from Handler, with its
// own Deps, so a citizen route cannot reach a staff store even by typing it (rule 4, invariant 5 —
// the same arrangement service-petitions uses).
type HandlerCongDan struct {
	d DepsCongDan
}

// DanhMucXa answers which commune, if any, a domain belongs to. GET /api/v1/communes?host=
//
// ORDER IS THE DESIGN: the shape of the host is checked BEFORE the platform is asked. A malformed
// value is the caller's mistake and costs nothing; sending it on would spend a platform call per
// junk request on a route any holder of a citizen session can reach. The shape is
// domain.HopLeTenMienXa — the SAME function the session bridge applies to `commune_host_hint`, so
// this screen can never name a commune for a value the bridge then refuses.
//
// ONE ANSWER FOR UNKNOWN, RESERVED AND INACTIVE: 200 `{"items": []}`. service-platform already
// answers reserved Hosts with the same NotFound as unclaimed ones (internal/grpc/server.go:96-98);
// inactive is folded in HERE. Telling them apart would let whoever holds a session map which
// domains are the platform's and which communes have merged away — the probe core/httpx/edge.go:26-34
// refuses on the staff edge, for the same reason. A merged commune's QR landing screen is a
// configuration matter (domain redirect), not something this route guesses at.
//
// PLATFORM UNREACHABLE: 503. Never an empty list (that reads as "this QR is wrong"), and never a
// cached or remembered commune (rule 1, forbidden #1).
//
// NO CACHE: the answer is keyed by nothing tenant-shaped and would be cheap to cache, but a cache
// here is a second place where a commune's deactivation arrives late, and the platform directory
// read is one indexed lookup. Add one only after it is measured.
//
// NO AUDIT ENTRY: nothing is written, nothing personal is read (a commune's name and province are
// public metadata, not rule 3 data), and nothing of any commune's business data is touched — rule 6,
// invariant 7 asks for neither case. The KhongThuocXa class guarantees the last point mechanically:
// no commune is in the context, so store.For(ctx) would panic.
//
// NO PERSONAL DATA IN THE LOG LINE: the Host is logged on an outage because it is a domain, not a
// person; it has passed domain.HopLeTenMienXa, so it cannot carry a log-injection payload either.
func (h *HandlerCongDan) DanhMucXa(w http.ResponseWriter, r *http.Request) {
	gia := r.URL.Query()["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		// The body never echoes what was sent.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_host",
			"Tên miền của xã không hợp lệ.", "")
		return
	}
	host := gia[0]

	xa, ok, err := h.d.Xa.XaTheoHost(r.Context(), host)
	if err != nil {
		h.d.Log.WarnContext(r.Context(), "danh mục xã: không hỏi được dịch vụ nền tảng", "host", host, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
			"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
		return
	}
	ra := danhMucXaRa{Items: []xaCongKhai{}} // `[]`, never `null`: one shape for the client
	if ok && xa.Active {
		ra.Items = append(ra.Items, xaCongKhai{Name: xa.Name, Province: xa.Province})
	}
	vietJSON(w, http.StatusOK, ra)
}

// newHandlerCongDan defaults the logger the same way NewHandler does.
func newHandlerCongDan(d DepsCongDan) *HandlerCongDan {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &HandlerCongDan{d: d}
}
