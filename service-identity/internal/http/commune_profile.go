package http

import (
	"context"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// GET /api/v1/commune-profiles?host= — the commune's display profile for the Mini App's commune screen
// (SRS M6.1; user decision 2026-09-29).
//
// THE DATA IS THE PLATFORM'S, NOT THIS SERVICE'S: `ho_so_hien_thi_xa` is owned by service-platform (ADR
// 0045 decision 5) and read here over gRPC, GetTenantProfile, with the commune in "x-tenant-id" (rule 2,
// invariant 3). Nothing is stored or cached here.
//
// WHY ON IDENTITY'S PUBLIC CHAIN (coordinator's placement decision 2026-09-29): this chain already
// resolves `?host=` to a commune through the platform registry and already dials the platform; a
// second public edge in service-platform would duplicate the host resolution, the CORS allow-list and
// the one-shape-for-every-negative discipline in a second place.

// TenantProfileReader is the one profile read. *platformclient.Directory satisfies it. The commune is
// the one in ctx — put there by CommuneProfiles after the platform resolved the host.
type TenantProfileReader interface {
	TenantProfile(ctx context.Context) (platformclient.TenantProfile, bool, error)
}

// communeProfilesOut is ZERO or ONE profile — the same collection shape as danhMucXaRa, so an unknown,
// reserved or inactive domain is `{"items": []}` here exactly as it is on GET /api/v1/communes.
type communeProfilesOut struct {
	Items []communeProfileOut `json:"items"`
}

// communeProfileOut is what anybody who knows a commune's domain may read about it.
//
// NAME comes from the registry (the same value GET /api/v1/communes returns); the other three from the
// commune's declared profile, "" when not declared — the screen renders nothing, never a default.
//
// WHAT IS ABSENT IS THE CONTRACT:
//   - no `id`, no `host` — same reasons as xaCongKhai.
//   - NO LOGO. platform.proto TenantProfile.logo_url is "a URL to an image served elsewhere — not bytes
//     and not a storage key", i.e. not an approved derivative in ViGov's public bucket (ADR 0052 §2).
//     Forwarding it would make every Mini App fetch whatever host was typed. It arrives when logos are
//     stored as public-bucket objects.
//   - no `introduction` — outside the fields this route was approved for (2026-09-29).
type communeProfileOut struct {
	Name          string `json:"name"`
	OfficeAddress string `json:"office_address"`

	// Hotline is the commune's OFFICIAL line — public-service information, not personal data (#16).
	Hotline string `json:"hotline"`

	// OfficeHoursText is DISPLAY TEXT. The working-hours calendar deadlines count against is this
	// service's own (ADR 0007); a client must never parse this.
	OfficeHoursText string `json:"office_hours_text"`
}

// CommuneProfiles serves the display profile of the commune a domain belongs to.
//
// ORDER: host shape (400, nothing asked) → registry (503 if unreachable) → profile (503 if
// unreachable). An unknown, reserved or inactive domain stops at the registry with `{"items": []}` —
// byte-identical to GET /api/v1/communes for the same domain, so this route answers no existence
// question that one does not already answer.
//
// AN ACTIVE COMMUNE WITH NO DECLARED PROFILE is one item carrying the name and three "": the name is
// already public on /communes, and an empty list here would read as "no such commune".
//
// NO AUDIT ENTRY: nothing is written and nothing personal is read (rule 6, invariant 7).
func (h *HandlerCongKhai) CommuneProfiles(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()["host"]
	if len(values) != 1 || !domain.HopLeTenMienXa(values[0]) {
		viet400Host(w)
		return
	}

	commune, found, ok := h.xaTheoHost(w, r, values[0], "hồ sơ hiển thị xã")
	if !ok {
		return
	}
	out := communeProfilesOut{Items: []communeProfileOut{}} // `[]`, never `null`
	if !found {
		vietJSON(w, http.StatusOK, out)
		return
	}

	ctx := tenant.Into(r.Context(), commune.ID)
	profile, _, err := h.d.Profile.TenantProfile(ctx)
	// ok=false (not declared) needs no branch: the zero TenantProfile is exactly "" in every field.
	if err != nil {
		// The commune id and host only — a profile holds no personal data, and none is logged.
		h.d.Log.WarnContext(ctx, "hồ sơ hiển thị xã: không hỏi được dịch vụ nền tảng",
			"xa", string(commune.ID), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
			"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
		return
	}
	out.Items = append(out.Items, communeProfileOut{
		Name:            commune.Name,
		OfficeAddress:   profile.OfficeAddress,
		Hotline:         profile.Hotline,
		OfficeHoursText: profile.OfficeHoursText,
	})
	vietJSON(w, http.StatusOK, out)
}
