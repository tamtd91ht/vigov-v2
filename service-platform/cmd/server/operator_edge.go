package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/operatortoken"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/ulid"
	svchttp "github.com/vihat/vigov/service-platform/internal/http"
	"github.com/vihat/vigov/service-platform/internal/opauth"
)

// THE SECOND EDGE (ADR 0048 §28/09 #2 + #4): one host, OPERATOR_HOST, gets its own handler chain and
// its own routes; every other Host keeps the commune chain exactly as it was. The two never share a
// mux, so a commune route cannot be reached on the operator host and an operator route cannot be
// reached on a commune host — the separation is which handler a request ENTERS, decided once, here.

// buildOperatorEdge is the chain for Host == OPERATOR_HOST.
//
//	StripTenantHeaders  the operator realm has no commune; a client naming one is granting itself scope
//	Recover             a panic becomes a traceable 500
//	request guard       inside svchttp.OperatorHandler: Origin + JSON on every non-GET (CSRF —
//	                    admin.vigov.vn and every <xa>.vigov.vn are the SAME site, so SameSite=Strict
//	                    does not stop a commune page's script)
//	(no TenantMiddleware) — OPERATOR_HOST is a reserved host that resolves to no commune, and must not:
//	                      TenantMiddleware would 404 it, and resolving it to a commune would make the
//	                      vendor surface and that commune's surface one surface (ADR 0046, 0048 #6)
//
// The operator guards (internal/opauth) sit on each route, not around the mux: the sign-in routes
// are the ones that create a session, so a mux-wide guard would have to exempt them by path — the
// shape where an exemption list quietly grows.
func buildOperatorEdge(d svchttp.OperatorDeps) http.Handler {
	h := svchttp.OperatorHandler(d)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)
	return h
}

// requestHost is the Host header lower-cased and stripped of its port, the same reading
// TenantMiddleware makes, so both edges agree on which host a request is for.
func requestHost(r *http.Request) string {
	h := strings.ToLower(strings.TrimSpace(r.Host))
	if i := strings.LastIndexByte(h, ':'); i >= 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	return h
}

// buildOuter is the outermost mux: /healthz (outside every chain, see main.go), then the host split.
//
// operator == nil, or operatorHost == "", means the operator area is OFF: every request — the
// operator host's included — goes to the commune chain, where OPERATOR_HOST is a reserved host and
// answers 404 like any unknown host (ADR 0048: "vắng → cả khu 404"). The operator routes are not
// mounted anywhere in that case.
func buildOuter(commune, operator http.Handler, operatorHost string) *http.ServeMux {
	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if operator == nil || operatorHost == "" {
		outer.Handle("/", commune)
		return outer
	}
	outer.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestHost(r) == operatorHost {
			operator.ServeHTTP(w, r)
			return
		}
		commune.ServeHTTP(w, r)
	}))
	return outer
}

// failingCounter is the rate-limit store when REDIS_DSN is unset (dev only — staging/prod refuse to
// start without it, config.Redis). It fails every count, which ratelimit.Middleware answers with
// 503: the sign-in routes stay CLOSED rather than becoming unbounded (ADR 0048 §01/10 #3).
type failingCounter struct{}

func (failingCounter) Incr(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return 0, 0, errors.New("REDIS_DSN chưa đặt")
}

// hostHolder is the one registry question the startup check asks (*store.Directory).
type hostHolder interface {
	HostHeld(ctx context.Context, host string) (bool, error)
}

// refuseOperatorHostCollision stops the service from starting when OPERATOR_HOST is a host some
// commune row holds in tenant_domain — ACTIVE OR INACTIVE.
//
// WHY AT STARTUP AND NOT ONLY AT THE LOOKUP: the Directory already refuses to resolve
// OPERATOR_HOST, so no request would reach that commune through it. But the row would still say the
// vendor's cross-commune host is a commune's address: the operator console lists it, GetTenant can
// hand it out as the commune's primary host, and switching OPERATOR_HOST off later would leave a row
// nobody remembers is reserved. One host naming both surfaces is ADR 0048 stop condition #6, and
// refusing to start is the only answer that cannot be missed. An inactive commune counts: rule 7
// keeps its rows, and reactivating it would put the collision back.
//
// THE ERROR NAMES THE VARIABLE, NEVER THE COMMUNE: startup logs travel further than the registry,
// and the operator fixing this needs to know which setting to change, not which commune to look at
// — they can find the row with the host they set.
//
// A lookup failure also refuses to start (fail closed): "could not check" is not "no collision".
func refuseOperatorHostCollision(ctx context.Context, dir hostHolder, operatorHost string) error {
	if operatorHost == "" {
		return nil
	}
	held, err := dir.HostHeld(ctx, operatorHost)
	if err != nil {
		return fmt.Errorf("platform: không kiểm được OPERATOR_HOST với tenant_domain — từ chối khởi động: %w", err)
	}
	if held {
		return errors.New("platform: OPERATOR_HOST trùng một tên miền đang có trong tenant_domain (của một xã, " +
			"đang hoạt động hoặc đã ngừng) — khu vận hành không bao giờ dùng chung host với một xã (ADR 0048 điều " +
			"kiện dừng #6). Đặt OPERATOR_HOST khác, hoặc xin chủ dự án quyết dòng tenant_domain đó; không tự sửa dòng")
	}
	return nil
}

// operatorWiring is what run() needs to build and later close the operator edge.
type operatorWiring struct {
	handler http.Handler
	close   func()
}

// wireOperatorEdge builds the operator edge, or returns a nil handler when OPERATOR_HOST is unset —
// in which case NOTHING operator-related is dialled or opened.
//
// IN DEV a missing dependency degrades to a refusal at use (no signing keys → 503 on every guarded
// route; no Redis → 503 on every sign-in step). A missing IDENTITY_OPERATOR_GRPC_ADDR with OPERATOR_HOST set
// refuses to START: an operator edge that can resolve no session is an area that is "on" and answers
// 503 for a reason only the startup log could have named.
func wireOperatorEdge(cfg config.Config, reg svchttp.CommuneReader, w svchttp.CommuneWriter,
	forget func(string), log *slog.Logger) (operatorWiring, error) {
	host := cfg.OperatorHost()
	signingKeys := cfg.OperatorSessionSigningKeys()
	// identity's OPERATOR listener, not IDENTITY_GRPC_ADDR's staff port: OperatorService is
	// registered on that listener only (user decision 01/10/2026).
	identityAddr := cfg.IdentityOperatorGRPCAddr()
	redisDSN := cfg.RedisDSN()
	if host == "" {
		log.Info("khu vận hành TẮT — OPERATOR_HOST trống, không gắn tuyến vận hành nào")
		return operatorWiring{close: func() {}}, nil
	}

	var verifier opauth.TokenVerifier
	signer, err := operatortoken.NewSigner(signingKeys)
	switch {
	case errors.Is(err, operatortoken.ErrNotConfigured):
		// Reported by cfg.CanhBao too; every guarded operator route answers 503.
	case err != nil:
		return operatorWiring{}, fmt.Errorf("platform: OPERATOR_SESSION_SIGNING_KEYS: %w", err)
	default:
		verifier = signer
	}

	identity, err := operatorclient.Dial(identityAddr, cfg.GRPCCallerKey(), log)
	if err != nil {
		return operatorWiring{}, fmt.Errorf("platform: khu vận hành bật (OPERATOR_HOST) nhưng không nối được identity: %w", err)
	}

	var counter ratelimit.Counter = failingCounter{}
	closeCounter := func() {}
	if dsn := redisDSN.Lo(); dsn != "" {
		rc, err := ratelimit.NewRedisCounter(dsn)
		if err != nil {
			_ = identity.Close()
			return operatorWiring{}, fmt.Errorf("platform: %w", err)
		}
		counter, closeCounter = rc, func() { _ = rc.Close() }
	} else {
		log.Warn("CẢNH BÁO CẤU HÌNH: REDIS_DSN trống — mọi bước đăng nhập vận hành trả 503 (giới hạn tần suất đóng kín)")
	}
	limiter, err := ratelimit.New(counter, ratelimit.OperatorSignIn)
	if err != nil {
		_ = identity.Close()
		closeCounter()
		return operatorWiring{}, fmt.Errorf("platform: %w", err)
	}

	h := buildOperatorEdge(svchttp.OperatorDeps{
		Auth:         opauth.NewAuth(verifier, identity, log),
		Identity:     identity,
		Limiter:      limiter,
		Registry:     reg,
		Writer:       w,
		OperatorHost: host,
		NewID:        ulid.Moi,
		Forget:       forget,
		Log:          log,
	})
	log.Info("khu vận hành BẬT", "host", host)
	return operatorWiring{handler: h, close: func() { _ = identity.Close(); closeCounter() }}, nil
}
