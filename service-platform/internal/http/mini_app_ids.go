package http

// GET /api/v1/mini-app-ids — the PUBLIC, read-only answer to "which Zalo App ID does this deploy push
// to" (owner, option A, 06/10/2026): citizen-app/scripts/deploy.mjs reads the App ID from this
// service's `mini_app` table — the source of truth — instead of a hand-kept file that drifts from it.
//
// WHY IT IS PUBLIC: an App ID is not a secret — every printed QR (`zalo.me/s/<App ID>/…`) carries
// one — and the caller is a deploy run on a developer machine or Jenkins, which holds no staff session.
// The owner approved the unauthenticated route (rule 13 / rule 5 stop condition answered 06/10/2026).
//
// WHY IT IS NOT ON THE COMMUNE CHAIN: the request names no commune by its Host — it is addressed to
// the platform's own API host, which TenantMiddleware would answer 404. The question is a registry
// read keyed by a domain in the query, exactly like Directory.MiniApp / ResolveHost: it runs before
// any commune is known. cmd/server mounts RegisterMiniAppIDs beside /healthz, outside both chains.
//
// WHAT IT NEVER SAYS: no commune id, no commune name, no list. One App ID and which kind of app it is.
// Every absence — unknown domain, reserved host, inactive or merged commune, commune with no own app,
// shared app not declared — is ONE 404 body, so a prober learns nothing about which domains are
// communes. NO AUDIT ENTRY: a read of non-personal registry data (rule 6 audits writes, full personal
// data and cross-commune reads of business data; this is none of them).

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// SharedMiniAppReader is the shared-app read (*store.SharedMiniAppStore).
type SharedMiniAppReader interface {
	SharedMiniApp(ctx context.Context) (domain.SharedMiniApp, error)
}

// OwnMiniAppResolver answers a host's commune's live own app (*store.Directory).
type OwnMiniAppResolver interface {
	LiveOwnMiniAppID(ctx context.Context, host string) (string, error)
}

// MiniAppIDDeps are what the public lookup needs. All required.
type MiniAppIDDeps struct {
	Shared  SharedMiniAppReader
	Own     OwnMiniAppResolver
	Limiter *ratelimit.Limiter // ratelimit.MiniAppIDLookup
	Log     *slog.Logger
}

// miniAppIDOut is the whole answer. `source` is the enum the QR link card already uses (operator
// launch links): "chung" the shared app, "rieng" the commune's own (ADR 0011, Vietnamese snake_case).
type miniAppIDOut struct {
	AppID  string `json:"app_id"`
	Source string `json:"source"`
}

// sharedAppSelector is the only value `app` takes: the ViHAT shared app (ADR 0044 "app chung").
const sharedAppSelector = "vihat"

const (
	msgMiniAppIDNotFound = "Không tìm thấy Mini App đang chạy cho yêu cầu này."
	msgMiniAppIDQuery    = "Cần đúng một trong hai tham số: app=vihat, hoặc host=<tên miền của xã>."
)

// RegisterMiniAppIDs mounts the lookup. PANICS on a missing dependency, at startup — a nil limiter
// would be an unauthenticated route with no bound (rule 13 invariant 7).
func RegisterMiniAppIDs(mux *http.ServeMux, d MiniAppIDDeps) {
	if d.Shared == nil || d.Own == nil || d.Limiter == nil || d.Log == nil {
		panic("platform/http: RegisterMiniAppIDs thiếu phụ thuộc — Shared, Own, Limiter và Log đều bắt buộc")
	}
	h := &miniAppIDHandler{d: d}

	// RATE LIMIT — ratelimit.MiniAppIDLookup: 30 requests per minute per client network (provisional,
	// see the constant), counted BEFORE the query is read, so malformed probes spend the budget too.
	// Fails CLOSED: Redis down → 503 `rate_limit_unavailable`.
	//
	// Cache-Control: no-store on EVERY answer: an App ID replaced in the console is effective at once
	// (ADR 0070), and a cached old one would push a build into the app just retired.
	//
	// 400 `invalid_query`: neither or both of app/host, a repeated parameter, app other than `vihat`,
	// or a host that is not a bare domain name. 404 is one body for every absence (file comment).
	// 409: more than one live app matches — never picked.
	//
	// @summary  App ID của Mini App đang chạy — app chung (app=vihat) hoặc app riêng của xã giữ tên miền (host=…); cho lệnh đẩy Mini App
	// @screen   *(không có màn hình — lệnh đẩy citizen-app/scripts/deploy.mjs gọi; chủ dự án chọn phương án A 06/10/2026)*
	// @consumer citizen-app
	// @reply    200 miniAppIDOut
	// @reply    400 httpx.Error invalid_query
	// @reply    404 httpx.Error mini_app_id_not_found
	// @reply    409 httpx.Error shared_mini_app_ambiguous own_mini_app_ambiguous
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error internal
	// @reply    503 httpx.Error rate_limit_unavailable
	mux.Handle("GET /api/v1/mini-app-ids",
		authz.Public("lệnh đẩy Mini App (citizen-app/scripts/deploy.mjs, máy dev hoặc Jenkins, không có phiên cán bộ) đọc App ID từ sổ mini_app của nền tảng thay cho tệp chép tay (chủ dự án chọn phương án A 06/10/2026); App ID không phải bí mật — mọi mã QR in ra đều mang nó; chỉ trả App ID và loại app, không mã xã, không tên xã, không danh sách; giới hạn 30 lần/phút theo mạng của máy gọi")(
			http.HandlerFunc(h.lookup)))
}

type miniAppIDHandler struct{ d MiniAppIDDeps }

func (h *miniAppIDHandler) lookup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !ratelimit.Gate(w, r, h.d.Limiter, ratelimit.PlatformIPKey(httpx.ClientIP(r)), h.d.Log) {
		return
	}
	q := r.URL.Query()
	apps, hosts := q["app"], q["host"]
	switch {
	case len(apps) == 1 && len(hosts) == 0:
		if apps[0] != sharedAppSelector {
			h.invalidQuery(w)
			return
		}
		h.shared(w, r)
	case len(hosts) == 1 && len(apps) == 0:
		// The registry's own host check: lower-case LDH, no port, no scheme. A reserved platform host
		// is well-formed and simply names no commune — the 404, not a 400 that would say "reserved".
		host, err := domain.ParseCommuneHost(hosts[0], "")
		switch {
		case errors.Is(err, domain.ErrCommuneHostReserved):
			h.notFound(w)
			return
		case err != nil:
			h.invalidQuery(w)
			return
		}
		h.own(w, r, host)
	default:
		h.invalidQuery(w)
	}
}

func (h *miniAppIDHandler) shared(w http.ResponseWriter, r *http.Request) {
	a, err := h.d.Shared.SharedMiniApp(r.Context())
	switch {
	case errors.Is(err, store.ErrNoSharedMiniApp):
		h.notFound(w)
	case err != nil:
		h.refuse(w, r, err)
	default:
		writeJSON(w, http.StatusOK, miniAppIDOut{AppID: a.AppID, Source: launchSourceShared})
	}
}

func (h *miniAppIDHandler) own(w http.ResponseWriter, r *http.Request, host string) {
	appID, err := h.d.Own.LiveOwnMiniAppID(r.Context(), host)
	switch {
	case errors.Is(err, store.ErrKhongCoMiniApp):
		h.notFound(w)
	case errors.Is(err, store.ErrOwnMiniAppAmbiguous):
		// The same code and sentence the operator console's QR card refuses with (operator_launch.go).
		h.refuse(w, r, errOwnMiniAppAmbiguous)
	case err != nil:
		h.refuse(w, r, err)
	default:
		writeJSON(w, http.StatusOK, miniAppIDOut{AppID: appID, Source: launchSourceOwn})
	}
}

// refuse answers the ambiguity refusals with operator_launch.go's codes and sentences, and anything
// else as a 500 whose cause stays in the log.
func (h *miniAppIDHandler) refuse(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrSharedMiniAppAmbiguous) || errors.Is(err, errOwnMiniAppAmbiguous) {
		rf, _ := launchRefusalOf(err)
		h.d.Log.ErrorContext(r.Context(), "mini-app-ids: hơn một Mini App đang chạy khớp — không tự chọn", "code", rf.code)
		httpx.WriteError(w, rf.status, rf.code, rf.text, "")
		return
	}
	h.d.Log.ErrorContext(r.Context(), "mini-app-ids: lỗi hệ thống", "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

func (h *miniAppIDHandler) notFound(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "mini_app_id_not_found", msgMiniAppIDNotFound, "")
}

func (h *miniAppIDHandler) invalidQuery(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_query", msgMiniAppIDQuery, "")
}
