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

// launchLinkView is one of the commune's QR links. The console renders the QR client-side from `url`;
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

// launchLinksView is every QR link the commune has (owner 06/10/2026), shared app first, and the
// links that could not be built with the reason each was refused. Both arrays are always present.
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
// app's) — the ones getLaunchLink reports per link instead of refusing the commune. Anything else
// (unknown commune, an inactive one, a store failure) is not a property of one link: ok is false.
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
			log:  "khu vận hành: xã có hơn một Mini App riêng đang chạy — không tạo liên kết QR app riêng"}, true
	case errors.Is(err, store.ErrCommuneNoPrimaryHost):
		return launchRefusal{status: http.StatusConflict, code: "commune_no_primary_domain",
			text: "Xã chưa có tên miền chính, chưa tạo được liên kết mở app."}, true
	case errors.Is(err, domain.ErrLaunchHostInvalid):
		return launchRefusal{status: http.StatusConflict, code: "commune_no_primary_domain",
			text: "Tên miền chính của xã không dùng được trong liên kết mở app. Báo nhóm nền tảng.",
			log:  "khu vận hành: tên miền chính của xã không dùng được trong liên kết QR"}, true
	}
	return launchRefusal{}, false
}

func (h *operatorHandlers) logLaunchRefusal(r *http.Request, rf launchRefusal, err error) {
	if rf.log != "" {
		h.d.Log.ErrorContext(r.Context(), rf.log, "err", err)
	}
}

func (h *operatorHandlers) writeSharedAppError(w http.ResponseWriter, r *http.Request, err error) {
	if rf, ok := launchRefusalOf(err); ok {
		h.logLaunchRefusal(r, rf, err)
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

// getLaunchLink returns EVERY QR link the commune has, so the operator chooses which to print (owner
// 06/10/2026: "QR từ đâu thì mở app từ đó" — this replaces ADR 0070 §Sửa đổi 05/10/2026 #4's "own app
// first"). Shared app first: `https://zalo.me/s/<shared App ID>/?d=<primary host>&src=qr`, present when
// a shared app is declared and the commune has a usable primary host — for EVERY active commune, own
// app or not, because communes borrow the shared app while their own waits for Zalo's approval. Then
// the own app: `https://zalo.me/s/<its App ID>/?src=qr`, no `d=`, present when exactly one is live.
//
// A link that cannot be built is listed in `unavailable` with the code and text it was refused with
// before, so one link's problem never hides the other. Only when NEITHER link exists is the request
// refused, with the answer it had before 06/10/2026 (own-app ambiguity first, else the shared link's
// reason). An inactive commune is refused before any link is considered.
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
	host, active, hostErr := h.d.SharedApp.CommuneLaunchHost(r.Context(), id)
	if hostErr != nil && !errors.Is(hostErr, store.ErrCommuneNoPrimaryHost) {
		h.writeSharedAppError(w, r, hostErr)
		return
	}
	// An inactive (merged, dissolved) commune is refused before anything else: a QR for it would send
	// citizens to an authority that no longer takes petitions (rule 7 invariant 6).
	if !active {
		h.writeRegistryError(w, r, store.ErrCommuneInactive)
		return
	}
	shared, sharedErr := h.sharedLaunchLink(r.Context(), host, hostErr)
	own, ownFound, ownErr := h.ownLaunchLink(r.Context(), id, host)
	// A failure that is not a property of one link (store down, commune vanished) refuses the request:
	// listing it as "unavailable" would hide an outage behind a half-empty card.
	for _, err := range []error{ownErr, sharedErr} {
		if _, perLink := launchRefusalOf(err); err != nil && !perLink {
			h.writeSharedAppError(w, r, err)
			return
		}
	}
	out := launchLinksView{Links: []launchLinkView{}, Unavailable: []launchLinkUnavailable{}}
	if sharedErr == nil {
		out.Links = append(out.Links, shared)
	}
	if ownFound && ownErr == nil {
		out.Links = append(out.Links, own)
	}
	if len(out.Links) == 0 {
		if ownErr != nil {
			h.writeSharedAppError(w, r, ownErr)
		} else {
			h.writeSharedAppError(w, r, sharedErr)
		}
		return
	}
	for _, u := range []struct {
		source string
		err    error
	}{{launchSourceShared, sharedErr}, {launchSourceOwn, ownErr}} {
		if u.err == nil {
			continue
		}
		rf, _ := launchRefusalOf(u.err)
		h.logLaunchRefusal(r, rf, u.err)
		out.Unavailable = append(out.Unavailable, launchLinkUnavailable{Source: u.source, Code: rf.code, Message: rf.text})
	}
	writeJSON(w, http.StatusOK, out)
}

// sharedLaunchLink is the shared app's link for the commune's primary host. hostErr is the host read's
// own refusal (no primary host), reported first, as it was before 06/10/2026.
func (h *operatorHandlers) sharedLaunchLink(ctx context.Context, host string, hostErr error) (launchLinkView, error) {
	if hostErr != nil {
		return launchLinkView{}, hostErr
	}
	app, err := h.d.SharedApp.SharedMiniApp(ctx)
	if err != nil {
		return launchLinkView{}, err
	}
	link, err := domain.MiniAppLaunchLink(app.AppID, host)
	if err != nil {
		return launchLinkView{}, err
	}
	return launchLinkView{URL: link, Domain: host, AppID: app.AppID, Source: launchSourceShared}, nil
}

// ownLaunchLink is the commune's live own app's link. found is false, with no error, when the commune
// has none — an absence, not a refusal, so it is not listed as unavailable.
func (h *operatorHandlers) ownLaunchLink(ctx context.Context, id, host string) (launchLinkView, bool, error) {
	appID, found, err := h.liveOwnMiniApp(ctx, id)
	if err != nil || !found {
		return launchLinkView{}, false, err
	}
	link, err := domain.OwnMiniAppLaunchLink(appID)
	if err != nil {
		return launchLinkView{}, true, err
	}
	return launchLinkView{URL: link, Domain: host, AppID: appID, Source: launchSourceOwn}, true, nil
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
