package http

// Wave 2 of the operator area, part 2 (ADR 0073 #5, owner 04/10/2026): the platform's SHARED Mini App
// declaration and the commune QR link built from it. Both read and write only this service's own
// tables — mini_app, tenant, tenant_domain, platform_audit_log.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// SharedMiniAppEditor is the shared-app store (*store.SharedMiniAppStore).
type SharedMiniAppEditor interface {
	SharedMiniApp(ctx context.Context) (domain.SharedMiniApp, error)
	DeclareSharedMiniApp(ctx context.Context, appID, reason string, by domain.OperatorActor) (domain.SharedMiniApp, bool, error)
	CommuneLaunchHost(ctx context.Context, communeID string) (host string, active bool, err error)
}

type sharedMiniAppView struct {
	AppID     string    `json:"app_id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// sharedMiniAppBody — first declaration and replacement are ONE act with one shape: "this App ID is
// the shared app from now on". The App ID being replaced is not named: there is at most one, and the
// store reads it under lock.
type sharedMiniAppBody struct {
	AppID  string `json:"app_id"`
	Reason string `json:"reason"`
}

// launchLinkView is the commune's QR link. The console renders the QR client-side from `url`;
// `domain` is shown beside it so the person printing it can read which commune it names (it may be
// "" on a `rieng` link for a commune with no primary host — that link does not carry it).
// `source` says which app the link opens, so the card can label it: "rieng" (the commune's own app)
// or "chung" (the shared app). Enum values Vietnamese snake_case (ADR 0011).
type launchLinkView struct {
	URL    string `json:"url"`
	Domain string `json:"domain"`
	AppID  string `json:"app_id"`
	Source string `json:"source"`
}

const (
	launchSourceOwn    = "rieng"
	launchSourceShared = "chung"
)

const msgSharedAppNotDeclared = "Chưa khai báo App ID của Mini App dùng chung. Người giữ quyền Mini App khai báo trước."

func toSharedMiniAppView(a domain.SharedMiniApp) sharedMiniAppView {
	return sharedMiniAppView{AppID: a.AppID, CreatedAt: a.CreatedAt.UTC(), CreatedBy: a.CreatedBy}
}

func (h *operatorHandlers) writeSharedAppError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNoSharedMiniApp):
		// 409: here it is a precondition of another resource. The singleton's own read answers 404.
		httpx.WriteError(w, http.StatusConflict, "shared_mini_app_not_declared", msgSharedAppNotDeclared, "")
	case errors.Is(err, store.ErrSharedMiniAppAmbiguous):
		httpx.WriteError(w, http.StatusConflict, "shared_mini_app_ambiguous",
			"Sổ Mini App đang có hơn một Mini App dùng chung chạy cùng lúc. Báo nhóm nền tảng, không tự chọn.", "")
	case errors.Is(err, errOwnMiniAppAmbiguous):
		h.d.Log.ErrorContext(r.Context(), "khu vận hành: xã có hơn một Mini App riêng đang chạy — không tạo liên kết QR", "err", err)
		httpx.WriteError(w, http.StatusConflict, "own_mini_app_ambiguous",
			"Xã đang có hơn một Mini App riêng chạy cùng lúc. Gỡ App ID thừa trước, không tự chọn.", "")
	case errors.Is(err, store.ErrCommuneNoPrimaryHost):
		httpx.WriteError(w, http.StatusConflict, "commune_no_primary_domain",
			"Xã chưa có tên miền chính, chưa tạo được liên kết mở app.", "")
	case errors.Is(err, domain.ErrLaunchHostInvalid):
		h.d.Log.ErrorContext(r.Context(), "khu vận hành: tên miền chính của xã không dùng được trong liên kết QR", "err", err)
		httpx.WriteError(w, http.StatusConflict, "commune_no_primary_domain",
			"Tên miền chính của xã không dùng được trong liên kết mở app. Báo nhóm nền tảng.", "")
	default:
		h.writeRegistryError(w, r, err)
	}
}

// getSharedMiniApp — the shared app as the registry holds it now.
func (h *operatorHandlers) getSharedMiniApp(w http.ResponseWriter, r *http.Request) {
	a, err := h.d.SharedApp.SharedMiniApp(r.Context())
	if errors.Is(err, store.ErrNoSharedMiniApp) {
		httpx.WriteError(w, http.StatusNotFound, "shared_mini_app_not_declared", msgSharedAppNotDeclared, "")
		return
	}
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSharedMiniAppView(a))
}

// declareSharedMiniApp sets or replaces the platform's shared app App ID (owner 04/10/2026). Effective
// at once for citizen sign-ins through the bridge (Directory.MiniApp has no cache); QR links already
// printed with the OLD App ID stop opening a session — that is the cost of replacing it, and why the
// reason is mandatory.
func (h *operatorHandlers) declareSharedMiniApp(w http.ResponseWriter, r *http.Request) {
	var b sharedMiniAppBody
	if !decodeBody(w, r, &b) {
		return
	}
	if err := domain.ValidateMiniAppID(b.AppID); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	a, _, err := h.d.SharedApp.DeclareSharedMiniApp(r.Context(), b.AppID, reason, actorOf(r))
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSharedMiniAppView(a))
}

// getLaunchLink builds the commune's QR link (ADR 0070 §Sửa đổi 05/10/2026 #4): from the commune's
// LIVE own app when it has one — `https://zalo.me/s/<its App ID>/?src=qr`, no `d=` — otherwise from
// the shared app and the commune's PRIMARY host, exactly as before. QRs already printed keep working
// either way: both links stay valid for as long as their App ID is live.
//
// NOT TRAILED, by the owner's decision (ADR 0048 §30/09 #9): issuing a QR is not a write. The price,
// stated there: a misprinted QR cannot be traced to whoever generated it. ops.qr.issue is the gate.
//
// The commune is the PATH's (rule 1): never a body, a query or a header.
func (h *operatorHandlers) getLaunchLink(w http.ResponseWriter, r *http.Request) {
	_, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	host, active, err := h.d.SharedApp.CommuneLaunchHost(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrCommuneNoPrimaryHost) {
		h.writeSharedAppError(w, r, err)
		return
	}
	// An inactive (merged, dissolved) commune is refused before anything else: a QR for it would send
	// citizens to an authority that no longer takes petitions (rule 7 invariant 6).
	if !active {
		h.writeRegistryError(w, r, store.ErrCommuneInactive)
		return
	}
	own, found, ownErr := h.liveOwnMiniApp(r.Context(), id)
	if ownErr != nil {
		h.writeSharedAppError(w, r, ownErr)
		return
	}
	if found {
		link, err := domain.OwnMiniAppLaunchLink(own)
		if err != nil {
			h.writeSharedAppError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, launchLinkView{URL: link, Domain: host, AppID: own, Source: launchSourceOwn})
		return
	}
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	app, err := h.d.SharedApp.SharedMiniApp(r.Context())
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	link, err := domain.MiniAppLaunchLink(app.AppID, host)
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, launchLinkView{URL: link, Domain: host, AppID: app.AppID, Source: launchSourceShared})
}

// errOwnMiniAppAmbiguous — the commune has more than one running own app. ADR 0070 #1 forbids it and
// every write path refuses to create it, but no database constraint does (migration 0006 left it
// open); choosing one would print a QR for an App ID nobody chose. Refused, like the shared-app case.
var errOwnMiniAppAmbiguous = errors.New("operator launch link: more than one running own mini app")

// liveOwnMiniApp is the commune's running, not-deleted `rieng` App ID, if any. Commune already
// excludes soft-deleted rows; a switched-off row (written before 05/10/2026) is not live either.
func (h *operatorHandlers) liveOwnMiniApp(ctx context.Context, id string) (string, bool, error) {
	_, apps, err := h.d.Registry.Commune(ctx, id)
	if err != nil {
		return "", false, err
	}
	var live []string
	for _, a := range apps {
		if a.Mode == domain.CheDoRieng && a.Active {
			live = append(live, a.AppID)
		}
	}
	switch len(live) {
	case 0:
		return "", false, nil
	case 1:
		return live[0], true, nil
	}
	return "", false, errOwnMiniAppAmbiguous
}
