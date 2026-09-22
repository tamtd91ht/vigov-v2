package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// PhienHienTai is what the middleware learned about the current session, for the handlers that
// need it.
//
// WHY NOT ON authz.Principal: Principal answers "who is acting and where", which every service
// needs. Which session they are acting through is a detail only the identity service uses —
// sign-out, the session list, revoking a single device. Widening a shared type for one
// service's need is how a shared type stops meaning anything.
type PhienHienTai struct {
	// Sid is the session this request arrived on. Compared against a client-supplied sid before
	// revoking anything: a member of staff may end their OWN session, nobody else's.
	Sid string

	// MaCanBo is the BUSINESS code, the one the audit trail records (rule 6, invariant 3).
	// Deliberately not the same value as authz.Principal.ID — see the comment there.
	MaCanBo string

	// HetHanLuc is when this session expires, as the REGISTRY has it — not as the token claims
	// it. The registry is what revocation and expiry are decided on, so it is also what
	// GET /api/v1/sessions/current must report; a client told the token's own expiry would show
	// a session that was revoked an hour ago as still running.
	HetHanLuc time.Time

	// HoTen and ChucVu are the two display fields the header and GET /api/v1/sessions/current
	// show. They are copied here at step 5 below, where the account has just been read, so the
	// route costs no second query on a call that happens on every page load.
	//
	// THE WHOLE domain.CanBo IS DELIBERATELY NOT CARRIED. It holds MatKhauHash, and a credential
	// sitting in the request context is one `%+v` away from a log line that cannot be recalled
	// (rule 3, forbidden #1; rule 8). Two strings can leak nothing a response already shows.
	HoTen  string
	ChucVu string
}

type ctxKeyPhien struct{}

// PhienTu reports the current session, if the request carried a valid one.
func PhienTu(ctx context.Context) (PhienHienTai, bool) {
	p, ok := ctx.Value(ctxKeyPhien{}).(PhienHienTai)
	return p, ok
}

// XacThuc rebuilds the request's principal from its session cookie.
//
// IT RUNS ON EVERY REQUEST, not only on guarded ones (skills/session-and-token, required #3).
// A token that is merely signed cannot be taken back; checking the session registry on every
// request is the price of being able to revoke one at all — an account locked, a role changed
// or a password changed must stop working immediately, not when the token happens to expire.
//
// NO PRINCIPAL IS NOT AN ERROR HERE. A missing or unusable cookie lets the request continue
// without one, and authz decides: a Public route still serves, a guarded route answers 401.
// Refusing here instead would mean the sign-in page itself could not be reached once a cookie
// went stale.
//
// THE ONE EXCEPTION, AND THE ORDER THAT MAKES IT WORK: a token whose commune does not match
// the commune resolved from Host is refused on the spot, BEFORE any database read. See below.
func XacThuc(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Precondition, not one of the steps: this middleware is mounted INSIDE
			// httpx.TenantMiddleware. If it ever is not, panicking here is the loud failure —
			// the alternative is a session lookup with no commune, which reads every commune.
			xa := tenant.MustFrom(ctx)

			// 1. No cookie: carry on with no principal.
			c, err := r.Cookie(CookiePhien)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 2. Unreadable or expired token: clear it and carry on with no principal.
			claims, err := d.Signer.Giai(c.Value)
			if err != nil {
				xoaCookiePhien(w)
				next.ServeHTTP(w, r)
				return
			}

			// 3. COMMUNE CHECK — HERE, BEFORE ANY DATABASE READ, AND THE ORDER IS THE POINT.
			//
			// A browser does not send a cookie across hosts. A token issued for commune A
			// arriving at commune B's domain is therefore not a user mistake: it is a
			// deliberate probe or a stolen token, and it is a SECURITY SIGNAL, not a 4xx to
			// count in a dashboard (skills/session-and-token, §2).
			//
			// Doing it at the token layer also means the detection needs no cross-commune
			// query: nothing is looked up in commune B for a sid issued in commune A, so there
			// is no `// @cross-tenant:` to justify and no path by which one commune's request
			// touches another commune's rows.
			if claims.TenantID != xa {
				d.Log.Warn("CẢNH BÁO AN NINH: token của xã khác gửi tới tên miền này",
					"xa_trong_token", string(claims.TenantID),
					"xa_theo_host", string(xa),
					"ip", ipTu(r),
					"path", r.URL.Path,
					// A fingerprint, never the sid itself and never the token: an alert that
					// carries a working credential is a second copy of that credential, in the
					// log pipeline, where it cannot be recalled (rule 8).
					"sid_van_tay", vanTay(claims.Sid))
				xoaCookiePhien(w)
				httpx.WriteError(w, http.StatusUnauthorized, "tenant_mismatch",
					"Phiên đăng nhập không hợp lệ trên tên miền này.", "")
				return
			}

			// 4. Session registry: revoked, expired or unknown sid means no principal.
			ph, err := d.Phien.KiemTra(ctx, claims.Sid)
			if err != nil {
				xoaCookiePhien(w)
				next.ServeHTTP(w, r)
				return
			}

			// 5. The account itself: locked or soft-deleted accounts are not returned by
			// TheoID, so a person locked out mid-session stops being a principal on the very
			// next request.
			cb, err := d.CanBo.TheoID(ctx, ph.NguoiDungID)
			if err != nil {
				xoaCookiePhien(w)
				next.ServeHTTP(w, r)
				return
			}

			// 6. Build the principal.
			//
			// ID IS THE INTERNAL id, NOT cb.Ma. store.Checker queries `nd.id = $2` with
			// Principal.ID (identity/internal/store/checker.go:38). Putting the
			// business code here matches no row, so EVERY permission check answers false and
			// every guarded route returns 403 — with nothing in the response, the logs or a
			// test to point at the cause.
			//
			// Ma CARRIES THAT BUSINESS CODE INSTEAD, on its own field. It is what the audit
			// trail records (rule 6, invariant 2) and it decides nothing. PhienHienTai.MaCanBo
			// still holds the same value for identity's own header and /sessions/current; the
			// two are filled from one `cb` in one place, so there is nothing here that can
			// drift.
			//
			// Roles is left empty ON PURPOSE. Permissions are read from the database on every
			// request by store.Checker. Carrying them on the principal would mean a role change
			// takes effect only when the session ends (skills/session-and-token, FORBIDDEN).
			p := authz.Principal{
				ID:       cb.ID,
				Ma:       cb.Ma,
				Kind:     "staff",
				TenantID: xa,
			}
			ctx = authz.Into(ctx, p)
			ctx = context.WithValue(ctx, ctxKeyPhien{}, PhienHienTai{
				Sid:     claims.Sid,
				MaCanBo: cb.Ma,
				// ph.HetHanLuc, not claims.ExpiresAt: the registry is what expiry and revocation
				// are decided on, and the two can differ.
				HetHanLuc: ph.HetHanLuc,
				HoTen:     cb.HoTen,
				ChucVu:    cb.ChucVu,
			})

			// 7. Last-seen stamp. Deliberately outside any transaction and its failure ignored:
			// it is a diagnostic column, and failing a real request because a timestamp could
			// not be written trades an operation for a nicety.
			d.Phien.GhiNhanDung(ctx, claims.Sid)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// vanTay is a short, one-way fingerprint of a secret, so two log lines can be correlated
// without the log holding anything that can be replayed.
func vanTay(s string) string {
	tong := sha256.Sum256([]byte(s))
	return hex.EncodeToString(tong[:])[:12]
}

// ipTu reports the client address recorded on the audit trail.
//
// X-Forwarded-For IS DELIBERATELY NOT TRUSTED. Any client can set it, and a forged address in
// an archival record is worse than a proxy's address: the trail then states, with the authority
// of a government record, that somebody acted from an address they never used. When a trusted
// reverse proxy is actually in place, this is the single function to change, and the trust
// boundary has to be configured — not assumed.
//
// IT DELEGATES RATHER THAN REPEATING, and the reason is the sentence just above. `core/httpx`
// now holds the same logic for the four services that send this address to identity over gRPC.
// Two copies of one trust boundary is a boundary that gets widened in one copy: somebody adds
// X-Forwarded-For where a proxy was deployed, misses the other, and the audit trail then
// disagrees with itself about where one person acted. One function to change, as this comment
// already promised — it now has to be true across the repository, not just in this file.
func ipTu(r *http.Request) string { return httpx.ClientIP(r) }
