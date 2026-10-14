package http

// THE COMMUNE'S ONE QR LINK (owner 13/10/2026). Commune staff on web-admin (Cấu hình) and operators on
// the commune detail of platform-admin see exactly ONE link, chosen by ONE rule — chooseLaunchLink — so
// the two screens can never print different QRs for the same commune:
//
//  1. the commune has exactly one live own app (mini_app che_do 'rieng', running, not deleted) THAT AN
//     OPERATOR HAS MARKED PUBLISHED on Zalo (published_at, migration 0025; ADR 0070 bổ sung 13/10/2026)
//     → `https://zalo.me/s/<its App ID>/?src=qr` (domain.OwnMiniAppLaunchLink). An unpublished App ID
//     opens only for its Zalo admins/developers, so it is never put on a QR. Both callers filter on the
//     mark BEFORE calling chooseLaunchLink: liveOwn below holds published App IDs only;
//  2. otherwise → the shared app's `https://zalo.me/s/<shared App ID>/?src=qr&x=<launch code>`
//     (domain.MiniAppLaunchLink).
//
// The `d=<domain>` form is withdrawn from both routes by the same decision.
//
// THE DEV CLUSTER'S LINKS CARRY `pf` (owner 14/10/2026, ADR 0046 §Sửa đổi 14/10/2026 #9): when
// MINI_APP_PROFILE_CODE is set — vigov-dev only; core/config refuses it in staging/prod — both forms gain
// `pf=<code>`, so the QR opens the Mini App against the dev hosts exactly as `npm run qr -- --profile=dev`
// does. The real queries, keys sorted by url.Values: own `?pf=<code>&src=qr`, shared
// `?pf=<code>&src=qr&x=<launch code>`. Unset (prod) = the two forms above, unchanged. Both routes go
// through chooseLaunchLink, so both screens get the same `pf` or none.
//
// NOT TRAILED, either route (ADR 0048 §30/09 #9): building a link writes nothing. The price, stated
// there: a misprinted QR cannot be traced to whoever generated it.

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// SharedAppReader is the one read of the platform's shared Mini App the QR routes need
// (*store.SharedMiniAppStore). The row has no commune (migration 0006), so it takes none.
type SharedAppReader interface {
	SharedMiniApp(ctx context.Context) (domain.SharedMiniApp, error)
}

// CommuneLaunchReader reads the commune IN CTX's launch facts (*store.CommuneLaunchStore). It takes no
// commune: the commune is the one the edge resolved from Host (rule 1, invariant 4).
type CommuneLaunchReader interface {
	CommuneLaunch(ctx context.Context) (store.CommuneLaunch, error)
}

// chosenLaunchLink is the one link chooseLaunchLink picked.
type chosenLaunchLink struct {
	url, appID, source string
}

// chooseLaunchLink applies the owner's rule of 13/10/2026 (file comment). liveOwn is the commune's
// running own App IDs marked published (two or more = ambiguous); launchCode its tenant.launch_code;
// profileCode MINI_APP_PROFILE_CODE (empty outside the dev cluster), appended as `pf` to either link.
//
// ownErr is the own app's refusal that made the shared link the choice (errOwnMiniAppAmbiguous), nil
// when there was none — the operator console lists it as unavailable, staff never see it. err refuses
// the request: no link could be built. Among per-link refusals the own app's ambiguity wins, as before
// 13/10/2026; a failure that is not a property of one link (a store outage) always wins, so an outage
// is never reported as "no link".
//
// TWO OWN APPS NEVER FALL BACK SILENTLY TO NOTHING: the shared link is still given — a citizen can open
// the commune through it — and the ambiguity is reported. Choosing one of the two App IDs would print a
// QR for an app nobody chose (ADR 0070 #1).
func chooseLaunchLink(ctx context.Context, shared SharedAppReader, liveOwn []string, launchCode string,
	profileCode secret.Secret) (link chosenLaunchLink, ownErr, err error) {

	// The ONE place the code leaves secret.Secret: the URL it is printed into (staff see it on the QR).
	pf := string(profileCode.Lo())
	switch len(liveOwn) {
	case 0:
	case 1:
		u, err := domain.OwnMiniAppLaunchLink(liveOwn[0], pf)
		if err != nil {
			return chosenLaunchLink{}, nil, err
		}
		return chosenLaunchLink{url: u, appID: liveOwn[0], source: launchSourceOwn}, nil, nil
	default:
		ownErr = errOwnMiniAppAmbiguous
	}
	app, err := shared.SharedMiniApp(ctx)
	if err == nil {
		var u string
		if u, err = domain.MiniAppLaunchLink(app.AppID, launchCode, pf); err == nil {
			return chosenLaunchLink{url: u, appID: app.AppID, source: launchSourceShared}, ownErr, nil
		}
	}
	if _, perLink := launchRefusalOf(err); perLink && ownErr != nil {
		return chosenLaunchLink{}, ownErr, ownErr
	}
	return chosenLaunchLink{}, ownErr, err
}

// communeLaunchLinkOut is the commune's QR link as STAFF see it: the URL to render and which kind of app
// it opens. No App ID and no domain (owner 13/10/2026: staff see one link and nothing about the shared
// app); `source` is "rieng" or "chung" (Vietnamese enum values, ADR 0011) so the screen can word the
// card, nothing more.
type communeLaunchLinkOut struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

type communeLaunchHandlers struct {
	launch      CommuneLaunchReader
	shared      SharedAppReader
	profileCode secret.Secret
	log         *slog.Logger
}

// get answers GET /api/v1/commune-launch-link for the commune of the Host. Read-only, not trailed.
func (h *communeLaunchHandlers) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := h.launch.CommuneLaunch(ctx)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	// An inactive (merged, dissolved) commune gets no QR: it would send citizens to an authority that no
	// longer takes petitions (rule 7 invariant 6). The edge refuses such a Host already; this holds if it
	// ever stops doing so. Same code and text as the operator route.
	if !c.Active {
		httpx.WriteError(w, http.StatusConflict, "commune_inactive", "Xã đã ngừng hoạt động.", "")
		return
	}
	link, _, err := chooseLaunchLink(ctx, h.shared, c.LiveOwnAppIDs, c.LaunchCode, h.profileCode)
	if err != nil {
		if rf, ok := launchRefusalOf(err); ok {
			logLaunchRefusal(ctx, h.log, rf, err)
			httpx.WriteError(w, rf.status, rf.code, rf.text, "")
			return
		}
		h.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, communeLaunchLinkOut{URL: link.url, Source: link.source})
}

// internal logs a failure with the commune id only — no body, no principal (rule 3) — and answers 500.
func (h *communeLaunchHandlers) internal(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "liên kết mở Mini App của xã: đọc thất bại",
		"xa", string(tenant.MustFrom(r.Context())), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", msgInternal, "")
}
