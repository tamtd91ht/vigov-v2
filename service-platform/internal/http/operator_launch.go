package http

// Wave 2 of the operator area, part 2 (ADR 0073 #5, owner 04/10/2026): the platform's SHARED Mini App
// declaration and the commune QR link built from it. Both read and write only this service's own
// tables — mini_app, tenant, tenant_domain, platform_audit_log.

import (
	"context"
	"errors"
	"log/slog"
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
	CommuneLaunchTarget(ctx context.Context, communeID string) (store.LaunchTarget, error)
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
// `domain` is the commune's PRIMARY host, shown beside it so the person printing it can read which
// commune it names — the link itself never carries it ("" for a commune with no primary host).
// `source` says which app: "chung" (the shared app, `x=<launch code>`) or "rieng" (the commune's own
// app). Enum values Vietnamese snake_case (ADR 0011). `app_id` is kept for the console's existing
// shape; the owner's decision of 13/10/2026 is that operators are not SHOWN the shared app's identity.
type launchLinkView struct {
	URL    string `json:"url"`
	Domain string `json:"domain"`
	AppID  string `json:"app_id"`
	Source string `json:"source"`
}

// launchLinksView is the commune's QR link — AT MOST ONE since 13/10/2026 (chooseLaunchLink), in an
// array so platform-admin's existing shape still reads it — and the own-app refusal that made the
// shared link the choice, if any. Both arrays are always present.
type launchLinksView struct {
	Links       []launchLinkView        `json:"links"`
	Unavailable []launchLinkUnavailable `json:"unavailable"`
}

// launchLinkUnavailable names a link the commune would have but cannot get now, with the same code
// and text the route used to refuse with — so the console explains the gap instead of hiding it.
type launchLinkUnavailable struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	launchSourceOwn    = "rieng"
	launchSourceShared = "chung"
)

const msgSharedAppNotDeclared = "Chưa khai báo App ID của Mini App dùng chung. Người giữ quyền Mini App khai báo trước."

func toSharedMiniAppView(a domain.SharedMiniApp) sharedMiniAppView {
	return sharedMiniAppView{AppID: a.AppID, CreatedAt: a.CreatedAt.UTC(), CreatedBy: a.CreatedBy}
}

// launchRefusal is how one launch-link refusal reads to the console. `log` is set where the cause is a
// state the platform team must repair, not one the operator can.
type launchRefusal struct {
	status     int
	code, text string
	log        string
}

// launchRefusalOf maps the refusals that concern ONE link of a commune (the shared app's, or the own
// app's). Both QR routes answer them with these codes and texts; the operator route also lists the
// own-app one as unavailable when the shared link is given instead. Anything else (unknown commune, an
// inactive one, a store failure) is not a property of one link: ok is false.
func launchRefusalOf(err error) (launchRefusal, bool) {
	switch {
	case err == nil:
		return launchRefusal{}, false
	case errors.Is(err, store.ErrNoSharedMiniApp):
		// 409: here it is a precondition of another resource. The singleton's own read answers 404.
		return launchRefusal{status: http.StatusConflict, code: "shared_mini_app_not_declared", text: msgSharedAppNotDeclared}, true
	case errors.Is(err, store.ErrSharedMiniAppAmbiguous):
		return launchRefusal{status: http.StatusConflict, code: "shared_mini_app_ambiguous",
			text: "Sổ Mini App đang có hơn một Mini App dùng chung chạy cùng lúc. Báo nhóm nền tảng, không tự chọn."}, true
	case errors.Is(err, errOwnMiniAppAmbiguous):
		return launchRefusal{status: http.StatusConflict, code: "own_mini_app_ambiguous",
			text: "Xã đang có hơn một Mini App riêng chạy cùng lúc. Gỡ App ID thừa trước, không tự chọn.",
			log:  "liên kết QR: xã có hơn một Mini App riêng đang chạy — không tạo liên kết QR app riêng"}, true
	case errors.Is(err, domain.ErrLaunchCodeInvalid):
		// Not reachable through the registry (0024: NOT NULL + shape CHECK); a row edited by hand.
		return launchRefusal{status: http.StatusConflict, code: "commune_launch_code_invalid",
			text: "Mã mở app của xã không đúng dạng. Báo nhóm nền tảng.",
			log:  "liên kết QR: mã mở app (launch_code) của xã không đúng dạng — không tạo liên kết QR x="}, true
	}
	return launchRefusal{}, false
}

// logLaunchRefusal logs a refusal the platform team must repair. Nothing but the error: no commune
// data, no principal (rule 3).
func logLaunchRefusal(ctx context.Context, log *slog.Logger, rf launchRefusal, err error) {
	if rf.log != "" {
		log.ErrorContext(ctx, rf.log, "err", err)
	}
}

func (h *operatorHandlers) writeSharedAppError(w http.ResponseWriter, r *http.Request, err error) {
	if rf, ok := launchRefusalOf(err); ok {
		logLaunchRefusal(r.Context(), h.d.Log, rf, err)
		httpx.WriteError(w, rf.status, rf.code, rf.text, "")
		return
	}
	h.writeRegistryError(w, r, err)
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

// getLaunchLink returns the commune's ONE QR link (owner 13/10/2026), chosen by chooseLaunchLink — the
// same rule the commune's own staff see on web-admin (GET /api/v1/commune-launch-link), so the two
// screens never print different QRs for one commune: the own app's link when exactly one is live, else
// the shared app's `x=<launch code>` link. The `d=<domain>` form is gone. This replaces "every link,
// the operator chooses" of 06/10/2026. In the dev cluster either link also carries `pf=<profile code>`
// (MINI_APP_PROFILE_CODE, ADR 0046 §Sửa đổi 14/10/2026 #9; launch_link.go).
//
// `links` holds that one link (the array is the console's existing shape). `unavailable` lists the own
// app's ambiguity when it is what made the shared link the choice. No link at all is refused, with the
// own-app ambiguity first, else the shared link's reason. An inactive commune is refused before any
// link is considered.
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
	target, err := h.d.SharedApp.CommuneLaunchTarget(r.Context(), id)
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	// An inactive (merged, dissolved) commune is refused before anything else: a QR for it would send
	// citizens to an authority that no longer takes petitions (rule 7 invariant 6).
	if !target.Active {
		h.writeRegistryError(w, r, store.ErrCommuneInactive)
		return
	}
	live, unpublished, err := h.liveOwnMiniApps(r.Context(), id)
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	link, ownErr, err := chooseLaunchLink(r.Context(), h.d.SharedApp, live, target.LaunchCode, h.d.MiniAppProfileCode)
	if err != nil {
		h.writeSharedAppError(w, r, err)
		return
	}
	out := launchLinksView{
		Links:       []launchLinkView{{URL: link.url, Domain: target.Host, AppID: link.appID, Source: link.source}},
		Unavailable: []launchLinkUnavailable{},
	}
	if ownErr != nil {
		rf, _ := launchRefusalOf(ownErr)
		logLaunchRefusal(r.Context(), h.d.Log, rf, ownErr)
		out.Unavailable = append(out.Unavailable, launchLinkUnavailable{Source: launchSourceOwn, Code: rf.code, Message: rf.text})
	}
	// The commune HAS a running own app, but no operator has marked it published, so the shared link was
	// given. Said to the OPERATOR only — the one who can mark it, and who would otherwise read the shared
	// QR as a fault. The staff route never says it (owner 13/10/2026: staff see one link, nothing else).
	// Not logged: it is a state the operator repairs, not the platform team.
	if link.source == launchSourceShared && ownErr == nil && unpublished > 0 {
		out.Unavailable = append(out.Unavailable, launchLinkUnavailable{Source: launchSourceOwn,
			Code: codeOwnMiniAppUnpublished, Message: msgOwnMiniAppUnpublished})
	}
	writeJSON(w, http.StatusOK, out)
}

const (
	codeOwnMiniAppUnpublished = "own_mini_app_unpublished"
	msgOwnMiniAppUnpublished  = "Mini App riêng của xã chưa được đánh dấu đã phát hành trên Zalo, nên QR dùng app chung. " +
		"Bản chưa phát hành chỉ mở được bằng tài khoản quản trị/phát triển của app. Đánh dấu đã phát hành khi Zalo đã phát hành app."
)

// errOwnMiniAppAmbiguous — the commune has more than one running own app. ADR 0070 #1 forbids it and
// every write path refuses to create it, but no database constraint does (migration 0006 left it
// open); choosing one would print a QR for an App ID nobody chose. Refused, like the shared-app case.
var errOwnMiniAppAmbiguous = errors.New("launch link: more than one running own mini app")

// liveOwnMiniApps is the commune's running, not-deleted `rieng` App IDs an operator has marked
// PUBLISHED on Zalo — the same set store.CommuneLaunchStore gives the staff route (ADR 0070 bổ sung
// 13/10/2026) — and how many running ones are NOT marked. Commune already excludes soft-deleted rows; a
// switched-off row (written before 05/10/2026) is not live either.
func (h *operatorHandlers) liveOwnMiniApps(ctx context.Context, id string) (live []string, unpublished int, err error) {
	_, apps, err := h.d.Registry.Commune(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	for _, a := range apps {
		if a.Mode != domain.CheDoRieng || !a.Active {
			continue
		}
		if a.Published() {
			live = append(live, a.AppID)
		} else {
			unpublished++
		}
	}
	return live, unpublished, nil
}
