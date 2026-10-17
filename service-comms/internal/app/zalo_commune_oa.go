package app

// A COMMUNE'S ZALO OAs (migration 0031; ADR 0098, block A; SEVERAL per commune since ADR 0099 #1 and
// migration 0032) — the administration of them (CommuneZaloOAs, behind /api/v1/zalo-oas,
// /api/v1/zalo-oas/{id}/… and /api/v1/zalo-oa-templates/{id}, admin.zalo, and the public callback
// GET /api/v1/zalo-oa-authorizations on the platform's callback host), the token lifecycle the refresh
// job drives, and the two reads the campaign sender uses (OpenAccessToken, TemplateForSend).
//
// EVERY ACT NAMES ITS OA BY ROW ID (ADR 0099 #1). There is no "the commune's OA": a commune holds several
// at once, the staff member picks one on the screen and in the campaign. An id that is not a live OA of
// the context's commune — unknown, retired, or another commune's — is ONE answer, ErrCommuneZaloOAMissing,
// so a guessed id tells nothing about another commune (rule 1; rule 4 forbidden #2 in spirit).
//
// THE COMMUNE IS THE CONTEXT'S, always (rule 1, invariant 4) — with ONE declared exception:
// CompleteAuthorization is called from Zalo's callback, which has no commune Host. It finds the commune
// FROM `state` through crosstenant.ZaloOA (ADR 0098 #4), puts it into the context, and from that line on
// every read and write is scoped like everything else. Not found, expired, used: one error, naming no
// commune. Nothing here takes a commune as an argument.
//
// THE TOKENS AND THE CODE VERIFIER ARE WRITE-ONLY. Sealed under the COMMUNE's data key (ADR 0098 #5),
// with additional data naming the SECRET, the COMMUNE and the ROW (0031 "OWED BY GO"). No return value
// carries a token; no audit delta carries a sealed column; Zalo failures are CLASSES (internal/zalooa),
// never text (ADR 0098 #13).
//
// THE ORDER OF EVERY ACT is the commune bot's (zalo_commune_bot.go): validate the request alone → the
// Zalo call OUTSIDE any transaction → seal OUTSIDE the transaction → ONE transaction with the change and
// its audit entry. Two writers of the sealed columns — the connecting member of staff and the refresh job
// (a system principal, rule 6 invariant 6) — hence 0031's token_refreshed_at (ADR 0098 §"Hai cột thời
// điểm").

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalooa"
)

// The 0031 bindings, verbatim ("OWED BY GO"). Bound to the ROW, so bytes copied from a retired row onto
// the live one do not open; to the commune, so they do not open in another; to the SECRET's name, so a
// refresh token does not open as an access token.
func communeOAAccessAAD(ctx context.Context, rowID string) []byte {
	return []byte("zalo_commune_oa/access_token/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

func communeOARefreshAAD(ctx context.Context, rowID string) []byte {
	return []byte("zalo_commune_oa/refresh_token/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

func oaAuthorizationVerifierAAD(ctx context.Context, rowID string) []byte {
	return []byte("zalo_oa_authorization/code_verifier/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

// ZaloOACallbackPath is the resource Zalo calls back (ADR 0098 #7), on the platform's callback host.
const ZaloOACallbackPath = "/api/v1/zalo-oa-authorizations"

// zaloOAStateBytes — ADR 0098 #4: "`state` (CSPRNG, 32 byte, một lần)".
const zaloOAStateBytes = 32

// MaxSyncedTemplates bounds one sync (5 pages of 100). A commune's OA has tens of templates; past this
// the sync is cut and says so, rather than paging for ever against a misbehaving answer.
const MaxSyncedTemplates = 500

// ZaloOAAPI is what this use case needs of internal/zalooa. *zalooa.Client satisfies it; tests fake it.
type ZaloOAAPI interface {
	ExchangeCode(ctx context.Context, appID string, appSecret secret.Secret, code string, verifier secret.Secret) (zalooa.Tokens, zalooa.Outcome)
	Refresh(ctx context.Context, appID string, appSecret, refreshToken secret.Secret) (zalooa.Tokens, zalooa.Outcome)
	GetOA(ctx context.Context, accessToken secret.Secret) (zalooa.OAInfo, zalooa.Outcome)
	ListTemplates(ctx context.Context, accessToken secret.Secret, offset, limit int) ([]zalooa.TemplateSummary, int, zalooa.Outcome)
	TemplateInfo(ctx context.Context, accessToken secret.Secret, templateID string) (zalooa.TemplateDetail, zalooa.Outcome)
}

// ZaloOAAuthorizationResolver is the callback's one cross-commune read (crosstenant.ZaloOA).
type ZaloOAAuthorizationResolver interface {
	OpenAuthorizationByStateHash(ctx context.Context, hash []byte) (tenant.ID, string, bool, error)
}

// ZaloOAConfig is the platform's shared Zalo App (config group ZaloOA, ADR 0098 #2, #3). All three empty
// in dev = the group is off: StartAuthorization and every Zalo call refuse by name.
type ZaloOAConfig struct {
	AppID        string
	AppSecret    secret.Secret
	CallbackHost string
}

func (c ZaloOAConfig) configured() bool {
	return c.AppID != "" && !c.AppSecret.Rong() && c.CallbackHost != ""
}

// CallbackURL is the URL comms registers with Zalo: https, the platform host, the fixed path.
func (c ZaloOAConfig) CallbackURL() string { return "https://" + c.CallbackHost + ZaloOACallbackPath }

var (
	// ErrZaloOANotConfigured — ZALO_OA_APP_ID / _APP_SECRET / _CALLBACK_HOST not set (dev): the OA
	// feature is off by name (503 at the edge).
	ErrZaloOANotConfigured = errors.New("zalo oa: the platform's Zalo App is not configured")
	// ErrCommuneZaloOAMissing — the id names no live OA of this commune: unknown, retired, or another
	// commune's — one error for all three (404 at the edge).
	ErrCommuneZaloOAMissing = errors.New("zalo oa: no such live OA in this commune")
	// ErrCommuneZaloOAChanged — the OA changed between the read and the write (a concurrent act).
	ErrCommuneZaloOAChanged = errors.New("zalo oa: the commune's OA changed meanwhile")
	// ErrZaloOANeedsReauthorization — the live OA's refresh token was refused or aged out; nothing
	// sends or syncs until the OA admin authorizes again (ADR 0098 #6).
	ErrZaloOANeedsReauthorization = errors.New("zalo oa: the OA needs re-authorization")
	// ErrAuthorizationNotFound — the callback's state matches no open, unexpired request. ONE sentence
	// for unknown, used and expired alike, naming no commune (ADR 0098 #4; 404 at the edge).
	ErrAuthorizationNotFound = errors.New("zalo oa: no open authorization request for this state")
	// ErrAuthorizationMalformed — the callback carried a state that found a request, but no code or an
	// oa_id outside Zalo's shape; the request is spent as sai-khuon (400 at the edge).
	ErrAuthorizationMalformed = errors.New("zalo oa: the callback is not the shape Zalo documents")
	// ErrAuthorizationDenied — the OA admin pressed Huỷ/Từ chối at Zalo: the callback came back with
	// `error` instead of a code. The request is spent as zalo-tu-choi; nothing is exchanged.
	ErrAuthorizationDenied = errors.New("zalo oa: the OA admin denied the authorization")
	// ErrZaloOATemplateMissing — no such template row of a LIVE OA of this commune (unknown, another
	// commune's, or its OA retired — one error).
	ErrZaloOATemplateMissing = errors.New("zalo oa: no such template")
	// ErrZaloOATemplateNotSendable — the template exists under the OA but Zalo does not have it ENABLE:
	// Zalo would refuse the send, so the sender does not try (TemplateForSend).
	ErrZaloOATemplateNotSendable = errors.New("zalo oa: the template is not ENABLE at Zalo")
)

// ZaloOACallError — Zalo did not accept a call. Class only (ADR 0098 #13); Step names which call.
type ZaloOACallError struct{ Step, Class string }

func (e *ZaloOACallError) Error() string { return "zalo oa: " + e.Step + " failed: " + e.Class }

// CommuneZaloOAItem is one live OA of the commune with its template counts.
type CommuneZaloOAItem struct {
	OA domain.CommuneZaloOA
	// TemplateCount / AttachedCount: templates loaded from THIS OA, and how many carry a milestone.
	TemplateCount int
	AttachedCount int
}

// CommuneZaloOAList is GET zalo-oas: every live OA of the commune, oldest first (ADR 0099 #1).
type CommuneZaloOAList struct {
	Items []CommuneZaloOAItem
	// Configured: the platform's Zalo App is set up — the "Kết nối OA" button is usable.
	Configured bool
}

// AuthorizationLink is POST zalo-oas/authorization-links: the Zalo link the OA admin opens.
type AuthorizationLink struct {
	URL       string
	ExpiresAt time.Time
}

// CompleteAuthorizationInput is what Zalo's GET callback carries, plus the browser's IP for the trail.
type CompleteAuthorizationInput struct {
	State    string // base64url, as placed in the link
	Code     string
	OAID     string
	ClientIP string
	// Denied: Zalo came back with `error` (the OA admin refused). With a valid state the request is
	// spent as zalo-tu-choi; the code, if any, is never exchanged.
	Denied bool
}

// CompleteAuthorizationResult — the commune the request belonged to and where to send the browser.
type CompleteAuthorizationResult struct {
	Tenant tenant.ID
	// Host is the commune's web host for the redirect back to Cấu hình › Kênh Zalo; "" when the registry
	// does not know it (the edge then answers a plain page — never a guessed host).
	Host string
	// Outcome is the domain.Authorization* value the request was spent with, on success AND on every
	// failure that spent it — so the edge sends the browser back with the result the screen names. ""
	// when nothing was spent (no commune found, or a fault before the request could be closed).
	Outcome string
	OA      domain.CommuneZaloOA
}

// ZaloOACheckOutput is POST zalo-oas/{id}/check.
type ZaloOACheckOutput struct {
	Outcome   string
	CheckedAt time.Time
	OAName    string // set only on thanh-cong
}

// DisconnectZaloOAOutput is DELETE zalo-oas/{id}.
type DisconnectZaloOAOutput struct {
	Retired               bool
	DetachedTemplateCount int
	// RevokeNotice is said whenever a row was retired (ADR 0098 #11).
	RevokeNotice string
}

// TemplateSyncResult is POST zalo-oas/{id}/templates/sync.
type TemplateSyncResult struct {
	Loaded   int // templates Zalo listed and this sync recorded
	Enabled  int
	Matching int // ENABLE and the three parameters present
	// ChangedTemplateIDs: templates whose zalo_status differs from the previous sync (new ones included).
	ChangedTemplateIDs []string
	// SkippedTemplateIDs: listed by Zalo but not recorded — detail unreadable, or a value outside 0031's
	// CHECKs. Named so the screen can say which.
	SkippedTemplateIDs []string
	// Truncated: Zalo listed more than MaxSyncedTemplates; the rest were not read.
	Truncated bool
	SyncedAt  time.Time
}

// CommuneZaloOAs owns the acts on the commune's OA.
type CommuneZaloOAs struct {
	db        *store.DB
	oas       *docstore.ZaloCommuneOAStore
	auths     *docstore.ZaloOAAuthorizationStore
	templates *docstore.ZaloOATemplateStore
	resolver  ZaloOAAuthorizationResolver
	envelope  *crypto.Envelope
	api       ZaloOAAPI
	cfg       ZaloOAConfig
	registry  PortalCommuneRegistry // the commune's host, for the callback's redirect
	log       *slog.Logger          // the SECURITY LOG (skills/security-logging) — never the audit trail

	newID    func() (string, error)
	now      func() time.Time
	newState func() ([]byte, error)
	newPKCE  func() (secret.Secret, string, error)
}

// NewCommuneZaloOAs panics on a missing dependency. envelope may be nil (no SECRET_ENCRYPTION_KEYS):
// every act that seals or opens then refuses with crypto.ErrNotConfigured.
func NewCommuneZaloOAs(db *store.DB, resolver ZaloOAAuthorizationResolver, envelope *crypto.Envelope, api ZaloOAAPI,
	cfg ZaloOAConfig, registry PortalCommuneRegistry, log *slog.Logger) *CommuneZaloOAs {

	if db == nil || resolver == nil || api == nil || registry == nil {
		panic("app: NewCommuneZaloOAs thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &CommuneZaloOAs{db: db, oas: docstore.NewZaloCommuneOAStore(db), auths: docstore.NewZaloOAAuthorizationStore(db),
		templates: docstore.NewZaloOATemplateStore(db), resolver: resolver, envelope: envelope, api: api, cfg: cfg,
		registry: registry, log: log, newID: ulid.Moi, now: time.Now, newState: newZaloOAState, newPKCE: zalooa.NewPKCE}
}

func (uc *CommuneZaloOAs) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// newZaloOAState is 32 bytes of CSPRNG output (rule 13 #2).
func newZaloOAState() ([]byte, error) {
	b := make([]byte, zaloOAStateBytes)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("zalo oa: generate state: %w", err)
	}
	return b, nil
}

// The security-log events of the commune OA (TCVN 14423 §6.8.2.1, "configuration changed"; ADR 0098
// #12). The audit trail already holds who did what; these are the operational stream an incident
// responder watches — a commune's OA is where its citizens' notices come from.
const (
	SecEventCommuneOALinkCreated   = "zalo_commune_oa.link_created"
	SecEventCommuneOAConnected     = "zalo_commune_oa.connected"
	SecEventCommuneOAAuthFailed    = "zalo_commune_oa.authorization_failed"
	SecEventCommuneOAChecked       = "zalo_commune_oa.checked"
	SecEventCommuneOARefreshed     = "zalo_commune_oa.token_refreshed"
	SecEventCommuneOAReauthNeeded  = "zalo_commune_oa.needs_reauthorization"
	SecEventCommuneOATemplatesSync = "zalo_commune_oa.templates_synced"
	SecEventCommuneOAMilestone     = "zalo_commune_oa.milestone"
	SecEventCommuneOADisconnected  = "zalo_commune_oa.disconnected"
)

// securityEvent writes one security-log line: event, outcome, commune, the actor's BUSINESS CODE, the
// client IP, and the OA's id (not personal data). NEVER a token, a state, a code or a Zalo URL
// (ADR 0098 #13; rule 3; rule 8).
func (uc *CommuneZaloOAs) securityEvent(ctx context.Context, event, outcome string, actor audit.Actor, oaID string) {
	uc.log.InfoContext(ctx, "NHẬT KÝ AN NINH: thay đổi cấu hình OA Zalo của xã",
		"event", event, "outcome", outcome, "xa", string(tenant.MustFrom(ctx)), "actor", actor.ID, "ip", actor.IP,
		"oa_id", oaID, "at", uc.clock())
}

// ---- reads --------------------------------------------------------------------------------------------

// List reads every live OA of the commune — no sealed column, no secret — with each one's template
// counts (ADR 0099 #1). Two reads whatever the number of OAs: the OAs, then one grouped count.
func (uc *CommuneZaloOAs) List(ctx context.Context) (CommuneZaloOAList, error) {
	out := CommuneZaloOAList{Configured: uc.cfg.configured()}
	oas, err := uc.oas.ListLive(ctx)
	if err != nil {
		return CommuneZaloOAList{}, err
	}
	if len(oas) == 0 {
		return out, nil
	}
	counts, err := uc.templates.CountsByOA(ctx)
	if err != nil {
		return CommuneZaloOAList{}, err
	}
	out.Items = make([]CommuneZaloOAItem, 0, len(oas))
	for _, o := range oas {
		c := counts[o.ID]
		out.Items = append(out.Items, CommuneZaloOAItem{OA: o, TemplateCount: c.Templates, AttachedCount: c.Attached})
	}
	return out, nil
}

// ListTemplates is GET zalo-oas/{id}/templates: the templates loaded from one live OA of the commune.
// An id that is not a live OA here is ErrCommuneZaloOAMissing — never an empty list, which would read as
// "this OA has no template".
func (uc *CommuneZaloOAs) ListTemplates(ctx context.Context, oaRowID string) ([]domain.ZaloOATemplate, error) {
	o, found, err := uc.oas.LiveByID(ctx, oaRowID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrCommuneZaloOAMissing
	}
	return uc.templates.ListByOA(ctx, o.ID)
}

// ---- the authorization ----------------------------------------------------------------------------------

// StartAuthorization issues one authorization link (ADR 0098 #4, #7): a CSPRNG state (stored only as its
// SHA-256), a PKCE pair (the verifier sealed), ONE transaction with the request row and its entry. Every
// call is a new link; older open links stay valid until their 10 minutes pass.
func (uc *CommuneZaloOAs) StartAuthorization(ctx context.Context, actor audit.Actor) (AuthorizationLink, error) {
	if actor.ID == "" {
		return AuthorizationLink{}, ErrNoActor
	}
	if !uc.cfg.configured() {
		return AuthorizationLink{}, ErrZaloOANotConfigured
	}
	if uc.envelope == nil {
		return AuthorizationLink{}, crypto.ErrNotConfigured
	}
	state, err := uc.newState()
	if err != nil {
		return AuthorizationLink{}, err
	}
	if len(state) != zaloOAStateBytes {
		return AuthorizationLink{}, errors.New("zalo oa: state is not 32 bytes")
	}
	verifier, challenge, err := uc.newPKCE()
	if err != nil {
		return AuthorizationLink{}, fmt.Errorf("zalo oa: generate PKCE: %w", err)
	}
	defer clear(verifier)
	id, err := uc.newID()
	if err != nil {
		return AuthorizationLink{}, fmt.Errorf("zalo oa: new id: %w", err)
	}
	sealed, err := uc.envelope.Seal(ctx, verifier, oaAuthorizationVerifierAAD(ctx, id))
	if err != nil {
		return AuthorizationLink{}, fmt.Errorf("zalo oa: seal the code verifier: %w", err)
	}
	hash := sha256.Sum256(state)
	at := uc.clock()
	req := domain.ZaloOAAuthorization{ID: id, RequestedAt: at, RequestedBy: actor.ID, ExpiresAt: at.Add(domain.AuthorizationTTL)}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.auths.Insert(ctx, tx, req, hash[:], sealed); err != nil {
			return err
		}
		// The delta names the request and its window — never the state, the hash or the verifier.
		return writeCommuneOAAudit(ctx, tx, actor, domain.ActionCreateZaloOAAuthorizationLink, req.Subject(), at, nil,
			map[string]any{"requested_at": at, "requested_by": actor.ID, "expires_at": req.ExpiresAt}, nil)
	})
	if err != nil {
		return AuthorizationLink{}, fmt.Errorf("zalo oa: store the authorization request: %w", err)
	}
	uc.securityEvent(ctx, SecEventCommuneOALinkCreated, "issued", actor, "")
	link := zalooa.PermissionURL(uc.cfg.AppID, uc.cfg.CallbackURL(), challenge, base64.RawURLEncoding.EncodeToString(state))
	return AuthorizationLink{URL: link, ExpiresAt: req.ExpiresAt}, nil
}

// CompleteAuthorization is Zalo's callback (GET zalo-oa-authorizations, public, on the callback host).
// ctx carries NO commune: the commune is found from the state (ADR 0098 #4), and the rest runs in it.
//
//	state unreadable, unknown, used, expired   ErrAuthorizationNotFound — one sentence, no commune
//	Zalo came back with `error` (Denied)       request spent as zalo-tu-choi; ErrAuthorizationDenied
//	code missing / oa_id not Zalo's shape      request spent as sai-khuon; ErrAuthorizationMalformed
//	Zalo refuses the exchange                  request spent with the class; *ZaloOACallError
//	getoa answers another oa_id                request spent as phan-hoi-sai-dang; *ZaloOACallError
//	the OA is live in another commune          request spent as oa-dang-o-xa-khac; ErrZaloOATaken
//	the SAME OA is already live in this        that row retired and its milestones detached (ADR 0098
//	commune (re-authorization)                 #11, kept by ADR 0099 #1), the new row inserted, the
//	                                           request closed thanh-cong — ONE transaction; two entries
//	                                           (retire, connect) attributed to who asked for the link
//	a DIFFERENT OA                             the new row inserted beside the commune's other OAs —
//	                                           nothing retired (ADR 0099 #1); one entry
//
// getoa not answering is NOT a refusal (ADR 0098 #13 "lỗi Zalo là lớp"): the OA is connected with an
// empty name and last_check carrying the class; "Kiểm tra" fills the name later. getoa answering a
// DIFFERENT oa_id than the callback is refused: the row must never carry an id its token does not open.
func (uc *CommuneZaloOAs) CompleteAuthorization(ctx context.Context, in CompleteAuthorizationInput) (CompleteAuthorizationResult, error) {
	if !uc.cfg.configured() {
		return CompleteAuthorizationResult{}, ErrZaloOANotConfigured
	}
	state, err := base64.RawURLEncoding.DecodeString(in.State)
	if err != nil || len(state) != zaloOAStateBytes {
		return CompleteAuthorizationResult{}, ErrAuthorizationNotFound
	}
	hash := sha256.Sum256(state)
	tid, reqID, found, err := uc.resolver.OpenAuthorizationByStateHash(ctx, hash[:])
	if err != nil {
		return CompleteAuthorizationResult{}, fmt.Errorf("zalo oa: find the authorization request: %w", err)
	}
	if !found {
		return CompleteAuthorizationResult{}, ErrAuthorizationNotFound
	}
	// FROM HERE ON, THE COMMUNE IS IN THE CONTEXT and every access is scoped (rule 1, invariant 5).
	cctx := tenant.Into(ctx, tid)
	open, found, err := uc.auths.OpenByID(cctx, reqID)
	if err != nil {
		return CompleteAuthorizationResult{}, err
	}
	now := uc.clock()
	if !found || !open.Auth.Open(now) {
		return CompleteAuthorizationResult{}, ErrAuthorizationNotFound
	}
	// The callback is an act of somebody outside any session: the trail's "who" is the staff member who
	// asked for the link; the IP is the OA admin's browser (ADR 0098 #12).
	actor := audit.Actor{ID: open.Auth.RequestedBy, Kind: "staff", IP: in.ClientIP}
	// The commune's host, for sending the browser back to Cấu hình › Kênh Zalo with the outcome — failures
	// included, so the OA admin reads what happened where they started. Unknown = "": the edge shows a
	// plain page; it never guesses a host.
	result := CompleteAuthorizationResult{Tenant: tid, Host: uc.communeHost(cctx)}

	if in.Denied {
		if err := uc.closeAuthorization(cctx, reqID, now, domain.AuthorizationZaloRefused, actor,
			map[string]any{"reason": "denied_at_zalo"}); err != nil {
			return result, err
		}
		uc.securityEvent(cctx, SecEventCommuneOAAuthFailed, "denied_at_zalo", actor, "")
		result.Outcome = domain.AuthorizationZaloRefused
		return result, ErrAuthorizationDenied
	}
	if in.Code == "" || !domain.ValidZaloOAID(in.OAID) {
		if err := uc.closeAuthorization(cctx, reqID, now, domain.AuthorizationMalformedCallback, actor, nil); err != nil {
			return result, err
		}
		uc.securityEvent(cctx, SecEventCommuneOAAuthFailed, domain.AuthorizationMalformedCallback, actor, "")
		result.Outcome = domain.AuthorizationMalformedCallback
		return result, ErrAuthorizationMalformed
	}
	if uc.envelope == nil {
		return result, crypto.ErrNotConfigured
	}
	verifier, err := uc.envelope.Open(cctx, open.CodeVerifierSealed, oaAuthorizationVerifierAAD(cctx, reqID))
	if err != nil {
		return result, fmt.Errorf("zalo oa: open the code verifier: %w", err)
	}
	defer clear(verifier)

	// 1. Zalo, outside any transaction: the exchange, then getoa with the fresh access token.
	tokens, o := uc.api.ExchangeCode(cctx, uc.cfg.AppID, uc.cfg.AppSecret, in.Code, verifier)
	if o != zalooa.OutcomeOK {
		class := oaOutcomeValue(o)
		outcome := domain.AuthorizationOutcomeOf(class)
		if err := uc.closeAuthorization(cctx, reqID, now, outcome, actor, map[string]any{"zalo_class": class}); err != nil {
			return result, err
		}
		uc.securityEvent(cctx, SecEventCommuneOAAuthFailed, outcome, actor, in.OAID)
		result.Outcome = outcome
		return result, &ZaloOACallError{Step: "exchange", Class: class}
	}
	defer clear(tokens.Access)
	defer clear(tokens.Refresh)
	info, oo := uc.api.GetOA(cctx, tokens.Access)
	checkClass := oaOutcomeValue(oo)
	name := ""
	if oo == zalooa.OutcomeOK {
		if info.OAID != in.OAID {
			if err := uc.closeAuthorization(cctx, reqID, now, domain.AuthorizationMalformedResponse, actor,
				map[string]any{"zalo_class": domain.ZaloCallMalformedResponse, "reason": "oa_id_mismatch"}); err != nil {
				return result, err
			}
			uc.securityEvent(cctx, SecEventCommuneOAAuthFailed, "oa_id_mismatch", actor, in.OAID)
			result.Outcome = domain.AuthorizationMalformedResponse
			return result, &ZaloOACallError{Step: "getoa", Class: domain.ZaloCallMalformedResponse}
		}
		name = domain.ClipZaloOAName(info.Name)
	}

	// 2. The row id is chosen BEFORE sealing: the binding names it.
	rowID, err := uc.newID()
	if err != nil {
		return result, fmt.Errorf("zalo oa: new id: %w", err)
	}
	accessSealed, refreshSealed, err := uc.sealPair(cctx, rowID, tokens)
	if err != nil {
		return result, err
	}
	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = domain.AccessTokenTTL // Zalo's documented 25 hours when the answer carried none
	}
	at := uc.clock()
	after := domain.CommuneZaloOA{ID: rowID, OAID: in.OAID, OAName: name, AccessExpiresAt: at.Add(expiresIn),
		RefreshIssuedAt: at, TokenRefreshedAt: at, AuthorizedAt: at, AuthorizedBy: actor.ID,
		LastCheckAt: at, LastCheckResult: checkClass}

	// 3. ONE transaction: retire + detach (only when THIS OA is already live here), insert, close,
	// entries. The commune's other OAs are left alone: ADR 0099 #1 — a commune holds several at once.
	var replaced *domain.CommuneZaloOA
	err = uc.db.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		if _, stillOpen, err := uc.auths.LockOpenByID(cctx, tx, reqID); err != nil {
			return err
		} else if !stillOpen {
			return ErrAuthorizationNotFound // spent meanwhile: a replayed state never connects twice
		}
		cur, found, err := uc.oas.LockLiveByZaloOAID(cctx, tx, in.OAID)
		if err != nil {
			return err
		}
		if found {
			// ADR 0098 #11 "dòng mới": the same OA again is a new row — authorized_* is immutable and the
			// new tokens are bound to the new row id. The old row's milestones go with it, as under 0098;
			// the commune re-attaches after "Nạp mẫu".
			if _, err := uc.retireInTx(cctx, tx, cur.OA, actor, domain.CommuneZaloOAReauthorizedReason, at,
				map[string]any{"replaced_by_row_id": rowID}); err != nil {
				return err
			}
			replaced = &cur.OA
		}
		if err := uc.oas.InsertLive(cctx, tx, after, accessSealed, refreshSealed); err != nil {
			return err
		}
		if err := uc.auths.Close(cctx, tx, reqID, at, domain.AuthorizationSucceeded, rowID); err != nil {
			return err
		}
		return writeCommuneOAAudit(cctx, tx, actor, domain.ActionConnectZaloOA, after.Subject(), at,
			communeOADeltaPtr(replaced), communeOADelta(after),
			map[string]any{"authorization_id": reqID, "replaced_oa": replaced != nil, "getoa": checkClass})
	})
	if err != nil {
		if errors.Is(err, docstore.ErrZaloOATaken) {
			// The floor refused it: spend the request, say nothing about which commune holds the OA.
			if cerr := uc.closeAuthorization(cctx, reqID, at, domain.AuthorizationOALiveElsewhere, actor, nil); cerr != nil {
				return result, cerr
			}
			uc.securityEvent(cctx, SecEventCommuneOAAuthFailed, domain.AuthorizationOALiveElsewhere, actor, in.OAID)
			result.Outcome = domain.AuthorizationOALiveElsewhere
			return result, docstore.ErrZaloOATaken
		}
		if errors.Is(err, ErrAuthorizationNotFound) {
			return result, ErrAuthorizationNotFound
		}
		return result, fmt.Errorf("zalo oa: connect the OA: %w", err)
	}
	result.OA, result.Outcome = after, domain.AuthorizationSucceeded
	outcome := "connected"
	if replaced != nil {
		outcome = "reauthorized"
	}
	uc.securityEvent(cctx, SecEventCommuneOAConnected, outcome, actor, in.OAID)
	return result, nil
}

// communeHost is the commune's web host from the platform registry; "" when unknown, inactive or the
// registry did not answer — never a guessed host.
func (uc *CommuneZaloOAs) communeHost(ctx context.Context) string {
	if t, ok, err := uc.registry.XaTrongNguCanh(ctx); err == nil && ok && t.Active {
		return t.Host
	}
	return ""
}

// closeAuthorization spends the request with outcome and writes its entry, in one transaction. A request
// spent meanwhile is left alone (nothing to record twice).
func (uc *CommuneZaloOAs) closeAuthorization(ctx context.Context, reqID string, at time.Time, outcome string,
	actor audit.Actor, facts map[string]any) error {

	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		open, found, err := uc.auths.LockOpenByID(ctx, tx, reqID)
		if err != nil || !found {
			return err
		}
		if err := uc.auths.Close(ctx, tx, reqID, at, outcome, ""); err != nil {
			return err
		}
		return writeCommuneOAAudit(ctx, tx, actor, domain.ActionZaloOAAuthorizationFailed, open.Auth.Subject(), at, nil,
			map[string]any{"used_at": at, "outcome": outcome}, facts)
	})
	if err != nil {
		return fmt.Errorf("zalo oa: close the authorization request: %w", err)
	}
	return nil
}

// sealPair seals both tokens under the row's two bindings.
func (uc *CommuneZaloOAs) sealPair(ctx context.Context, rowID string, t zalooa.Tokens) (access, refresh []byte, err error) {
	if uc.envelope == nil {
		return nil, nil, crypto.ErrNotConfigured
	}
	if access, err = uc.envelope.Seal(ctx, t.Access, communeOAAccessAAD(ctx, rowID)); err != nil {
		return nil, nil, fmt.Errorf("zalo oa: seal the access token: %w", err)
	}
	if refresh, err = uc.envelope.Seal(ctx, t.Refresh, communeOARefreshAAD(ctx, rowID)); err != nil {
		return nil, nil, fmt.Errorf("zalo oa: seal the refresh token: %w", err)
	}
	return access, refresh, nil
}

// retireInTx retires one live row and detaches its templates' milestones, with the entry — the shared
// half of a re-authorization of the same OA and of "Ngắt OA" (ADR 0098 #11; ADR 0099 #1).
// Returns how many milestones were detached. Everything runs on tx: a read on the pool here would wait
// for the connection this transaction holds.
func (uc *CommuneZaloOAs) retireInTx(ctx context.Context, tx *store.ScopedTx, o domain.CommuneZaloOA, actor audit.Actor,
	reason string, at time.Time, facts map[string]any) (int, error) {

	if err := uc.oas.Retire(ctx, tx, o.ID, actor.ID, reason, at); err != nil {
		return 0, err
	}
	detached, err := uc.templates.DetachAllOfOA(ctx, tx, o.ID, at)
	if err != nil {
		return 0, err
	}
	m := map[string]any{"detached_template_count": len(detached), "detached_template_row_ids": detached}
	for k, v := range facts {
		m[k] = v
	}
	return len(detached), writeCommuneOAAudit(ctx, tx, actor, domain.ActionDisconnectZaloOA, o.Subject(), at, communeOADelta(o),
		map[string]any{"retired_at": at, "retired_by": actor.ID, "retire_reason": reason}, m)
}

// ---- the token lifecycle ---------------------------------------------------------------------------------

// openAccess reads ONE live OA of the commune by row id and opens its ACCESS token — refreshing first when
// it is due (ADR 0098 #6: never a call with an expired token). actor is who the refresh is attributed to:
// the staff member behind "Kiểm tra" / "Nạp mẫu", or the system principal from the job and the sender.
// The caller clears the token.
//
//	no live OA with that id here             ErrCommuneZaloOAMissing
//	flagged needs-reauthorization            ErrZaloOANeedsReauthorization
//	refresh due, refresh token > 3 months    flagged refresh-token-het-han (one transaction + entry),
//	                                         ErrZaloOANeedsReauthorization — Zalo is not called
//	refresh due, Zalo refuses the token      flagged refresh-token-bi-tu-choi, same
//	refresh due, Zalo unavailable            *ZaloOACallError; nothing written; next tick retries
//	refresh due, Zalo answers                both tokens resealed in ONE transaction with the entry
func (uc *CommuneZaloOAs) openAccess(ctx context.Context, oaRowID string, actor audit.Actor) (docstore.CommuneOASecrets,
	secret.Secret, bool, error) {

	s, found, err := uc.oas.SealedByID(ctx, oaRowID)
	if err != nil {
		return docstore.CommuneOASecrets{}, nil, false, err
	}
	if !found {
		return docstore.CommuneOASecrets{}, nil, false, ErrCommuneZaloOAMissing
	}
	if s.OA.NeedsReauthorization() {
		return s, nil, false, ErrZaloOANeedsReauthorization
	}
	if uc.envelope == nil {
		return s, nil, false, crypto.ErrNotConfigured
	}
	now := uc.clock()
	if !s.OA.AccessRefreshDue(now) {
		access, err := uc.envelope.Open(ctx, s.AccessTokenSealed, communeOAAccessAAD(ctx, s.OA.ID))
		if err != nil {
			return s, nil, false, fmt.Errorf("zalo oa: open the access token: %w", err)
		}
		return s, access, false, nil
	}
	if s.OA.RefreshTokenExpired(now) {
		return s, nil, false, uc.flagReauthorization(ctx, s.OA, domain.ReauthorizationRefreshTokenExpired, actor, now)
	}
	if !uc.cfg.configured() {
		return s, nil, false, ErrZaloOANotConfigured
	}
	refresh, err := uc.envelope.Open(ctx, s.RefreshTokenSealed, communeOARefreshAAD(ctx, s.OA.ID))
	if err != nil {
		return s, nil, false, fmt.Errorf("zalo oa: open the refresh token: %w", err)
	}
	defer clear(refresh)

	tokens, o := uc.api.Refresh(ctx, uc.cfg.AppID, uc.cfg.AppSecret, refresh)
	switch {
	case o == zalooa.OutcomeTokenRejected:
		return s, nil, false, uc.flagReauthorization(ctx, s.OA, domain.ReauthorizationRefreshTokenRejected, actor, now)
	case o != zalooa.OutcomeOK:
		return s, nil, false, &ZaloOACallError{Step: "refresh", Class: oaOutcomeValue(o)}
	}
	defer clear(tokens.Refresh)
	accessSealed, refreshSealed, err := uc.sealPair(ctx, s.OA.ID, tokens)
	if err != nil {
		clear(tokens.Access)
		return s, nil, false, err
	}
	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = domain.AccessTokenTTL
	}
	at := uc.clock()
	after := s.OA
	after.AccessExpiresAt, after.RefreshIssuedAt, after.TokenRefreshedAt = at.Add(expiresIn), at, at
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.oas.LockLiveByID(ctx, tx, s.OA.ID)
		if err != nil {
			return err
		}
		if !found || !cur.OA.TokenRefreshedAt.Equal(s.OA.TokenRefreshedAt) {
			return ErrCommuneZaloOAChanged
		}
		if err := uc.oas.Reseal(ctx, tx, s.OA.ID, accessSealed, refreshSealed, after.AccessExpiresAt, after.RefreshIssuedAt,
			after.TokenRefreshedAt, s.OA.TokenRefreshedAt); err != nil {
			if errors.Is(err, docstore.ErrZaloRowChanged) {
				return ErrCommuneZaloOAChanged
			}
			return err
		}
		return writeCommuneOAAudit(ctx, tx, actor, domain.ActionRefreshZaloOAToken, s.OA.Subject(), at,
			map[string]any{"access_expires_at": s.OA.AccessExpiresAt, "refresh_issued_at": s.OA.RefreshIssuedAt,
				"token_refreshed_at": s.OA.TokenRefreshedAt},
			map[string]any{"access_expires_at": after.AccessExpiresAt, "refresh_issued_at": after.RefreshIssuedAt,
				"token_refreshed_at": after.TokenRefreshedAt}, nil)
	})
	if err != nil {
		clear(tokens.Access)
		// Zalo has already rotated the pair; a record that did not land leaves the row holding a refresh
		// token Zalo voided. The next attempt is refused by Zalo → needs re-authorization — visible, not
		// silent. The security log says it happened here.
		uc.securityEvent(ctx, SecEventCommuneOARefreshed, "record_failed", actor, s.OA.OAID)
		return s, nil, false, fmt.Errorf("zalo oa: record the refreshed tokens: %w", err)
	}
	s.OA = after
	uc.securityEvent(ctx, SecEventCommuneOARefreshed, "refreshed", actor, s.OA.OAID)
	return s, tokens.Access, true, nil
}

// flagReauthorization marks one live row and writes its entry in one transaction; a row already flagged
// by another writer is accepted as such.
func (uc *CommuneZaloOAs) flagReauthorization(ctx context.Context, o domain.CommuneZaloOA, reason string, actor audit.Actor,
	at time.Time) error {

	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.oas.LockLiveByID(ctx, tx, o.ID)
		if err != nil {
			return err
		}
		if !found {
			return ErrCommuneZaloOAChanged
		}
		if cur.OA.NeedsReauthorization() {
			return nil
		}
		if !cur.OA.TokenRefreshedAt.Equal(o.TokenRefreshedAt) {
			// Another writer refreshed meanwhile: the token we presented was the one it spent. Not a
			// refusal of the OA's grant — nothing to flag.
			return ErrCommuneZaloOAChanged
		}
		if err := uc.oas.MarkNeedsReauthorization(ctx, tx, o.ID, reason, at); err != nil {
			return err
		}
		return writeCommuneOAAudit(ctx, tx, actor, domain.ActionZaloOANeedsReauthorization, o.Subject(), at, nil,
			map[string]any{"needs_reauthorization_since": at, "needs_reauthorization_reason": reason}, nil)
	})
	if err != nil {
		return fmt.Errorf("zalo oa: flag re-authorization: %w", err)
	}
	uc.securityEvent(ctx, SecEventCommuneOAReauthNeeded, reason, actor, o.OAID)
	return ErrZaloOANeedsReauthorization
}

// RefreshIfDue is the refresh job's call for one commune (system principal): EVERY live OA of the
// commune whose access token is due is refreshed, flagged when Zalo refuses or the refresh token aged out
// (ADR 0099 #1 — several OAs, each with its own token pair). refreshed=true when at least one new pair was
// recorded. One OA failing does not stop the next: the failures are joined and returned, and the job
// retries them next tick. A flagged OA, or one retired or changed meanwhile, is not an error here.
func (uc *CommuneZaloOAs) RefreshIfDue(ctx context.Context) (refreshed bool, err error) {
	oas, err := uc.oas.ListLive(ctx)
	if err != nil {
		return false, err
	}
	now := uc.clock()
	var errs []error
	for _, o := range oas {
		if o.NeedsReauthorization() || !o.AccessRefreshDue(now) {
			continue
		}
		_, access, did, err := uc.openAccess(ctx, o.ID, systemPrincipal)
		clear(access)
		switch {
		case err == nil:
			refreshed = refreshed || did
		case errors.Is(err, ErrCommuneZaloOAMissing), errors.Is(err, ErrZaloOANeedsReauthorization),
			errors.Is(err, ErrCommuneZaloOAChanged):
		default:
			errs = append(errs, fmt.Errorf("oa row %s: %w", o.ID, err))
		}
	}
	return refreshed, errors.Join(errs...)
}

// ExpireOpenAuthorizations closes this commune's open requests past their expiry as het-han, with one
// system entry (the sweep's half of the job). Returns how many.
func (uc *CommuneZaloOAs) ExpireOpenAuthorizations(ctx context.Context, now time.Time) (int, error) {
	n := 0
	at := now.UTC().Truncate(time.Microsecond)
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		ids, err := uc.auths.ExpireOpen(ctx, tx, at, at)
		if err != nil || len(ids) == 0 {
			return err
		}
		n = len(ids)
		delta, err := json.Marshal(map[string]any{"outcome": domain.AuthorizationExpired, "used_at": at, "authorization_ids": ids})
		if err != nil {
			return fmt.Errorf("zalo oa: encode delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{Actor: systemPrincipal, Action: domain.ActionZaloOAAuthorizationExpired,
			Subject: "zalo-oa-authorization/" + at.Format("2006-01-02"), At: at, Delta: delta})
	})
	if err != nil {
		return 0, fmt.Errorf("zalo oa: expire authorization requests: %w", err)
	}
	return n, nil
}

// ---- the sender's two reads (ADR 0099 #8, block D-5) ------------------------------------------------------

// OpenAccessToken opens the ACCESS token of one live OA of the commune for a send, refreshing it first
// when it is due — the same path "Kiểm tra" takes, attributed to the system principal (the sender is a
// background job; rule 6 invariant 6). Returns the token and Zalo's oa_id. THE CALLER CLEARS THE TOKEN
// (clear(tok)) as soon as the call to Zalo returns, and never logs, stores or returns it.
//
//	id not a live OA of this commune     ErrCommuneZaloOAMissing (final for this campaign)
//	OA needs re-authorization            ErrZaloOANeedsReauthorization (final until the OA admin acts)
//	refresh due and Zalo unavailable     *ZaloOACallError (transient: retry later)
//	no SECRET_ENCRYPTION_KEYS            crypto.ErrNotConfigured
func (uc *CommuneZaloOAs) OpenAccessToken(ctx context.Context, oaRowID string) (secret.Secret, string, error) {
	s, access, _, err := uc.openAccess(ctx, oaRowID, systemPrincipal)
	if err != nil {
		clear(access)
		return nil, "", err
	}
	return access, s.OA.OAID, nil
}

// TemplateForSend reads the template Zalo calls templateID under one live OA of the commune, for a send:
// it must be ENABLE at Zalo. The returned Params are what Zalo declared — the campaign's parameters are
// checked against THEIR names by the caller. No milestone condition: a campaign template is any approved
// template of the OA, not one of the three-parameter milestone templates (ADR 0098 #9 is for milestones).
//
//	id not a live OA of this commune             ErrCommuneZaloOAMissing
//	OA needs re-authorization                    ErrZaloOANeedsReauthorization
//	template not loaded from that OA             ErrZaloOATemplateMissing
//	template not ENABLE (incl. DELETE at Zalo)   ErrZaloOATemplateNotSendable
func (uc *CommuneZaloOAs) TemplateForSend(ctx context.Context, oaRowID, templateID string) (domain.ZaloOATemplate, error) {
	o, found, err := uc.oas.LiveByID(ctx, oaRowID)
	if err != nil {
		return domain.ZaloOATemplate{}, err
	}
	if !found {
		return domain.ZaloOATemplate{}, ErrCommuneZaloOAMissing
	}
	if o.NeedsReauthorization() {
		return domain.ZaloOATemplate{}, ErrZaloOANeedsReauthorization
	}
	t, found, err := uc.templates.GetByZaloTemplateID(ctx, o.ID, templateID)
	if err != nil {
		return domain.ZaloOATemplate{}, err
	}
	if !found {
		return domain.ZaloOATemplate{}, ErrZaloOATemplateMissing
	}
	if t.ZaloStatus != domain.ZaloTemplateStatusEnabled {
		return domain.ZaloOATemplate{}, ErrZaloOATemplateNotSendable
	}
	return t, nil
}

// ---- the acts behind the buttons ------------------------------------------------------------------------

// Check — getoa with one OA's access token (refreshed first if due), then the result and the name
// recorded with the entry in one transaction (POST zalo-oas/{id}/check).
func (uc *CommuneZaloOAs) Check(ctx context.Context, oaRowID string, actor audit.Actor) (ZaloOACheckOutput, error) {
	if actor.ID == "" {
		return ZaloOACheckOutput{}, ErrNoActor
	}
	s, access, _, err := uc.openAccess(ctx, oaRowID, actor)
	if err != nil {
		return ZaloOACheckOutput{}, err
	}
	defer clear(access)
	info, o := uc.api.GetOA(ctx, access)
	result := oaOutcomeValue(o)
	name := s.OA.OAName
	if o == zalooa.OutcomeOK {
		name = domain.ClipZaloOAName(info.Name)
	}
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.oas.LockLiveByID(ctx, tx, s.OA.ID)
		if err != nil {
			return err
		}
		if !found {
			return ErrCommuneZaloOAChanged
		}
		if err := uc.oas.RecordCheck(ctx, tx, s.OA.ID, result, at); err != nil {
			return err
		}
		facts := map[string]any{"result": result, "checked_at": at}
		if name != cur.OA.OAName {
			if err := uc.oas.SetName(ctx, tx, s.OA.ID, name, at); err != nil {
				return err
			}
			facts["oa_name_before"], facts["oa_name_after"] = cur.OA.OAName, name
		}
		return writeCommuneOAAudit(ctx, tx, actor, domain.ActionCheckZaloOA, s.OA.Subject(), at, nil, facts, nil)
	})
	if err != nil {
		if errors.Is(err, docstore.ErrZaloRowChanged) || errors.Is(err, ErrCommuneZaloOAChanged) {
			return ZaloOACheckOutput{}, ErrCommuneZaloOAChanged
		}
		return ZaloOACheckOutput{}, fmt.Errorf("zalo oa: record the check: %w", err)
	}
	uc.securityEvent(ctx, SecEventCommuneOAChecked, result, actor, s.OA.OAID)
	out := ZaloOACheckOutput{Outcome: result, CheckedAt: at}
	if o == zalooa.OutcomeOK {
		out.OAName = name
	}
	return out, nil
}

// Disconnect — "Ngắt OA" (ADR 0098 #11) on ONE OA of the commune: that row retired with the staff
// member's reason and every milestone of its templates detached, in one transaction with the entry. The
// commune's other OAs are untouched (ADR 0099 #1). An id that is not a live OA here — another commune's,
// unknown, or already retired by an earlier DELETE — is ErrCommuneZaloOAMissing and writes nothing. The
// reply carries the revoke notice.
func (uc *CommuneZaloOAs) Disconnect(ctx context.Context, oaRowID, rawReason string, actor audit.Actor) (DisconnectZaloOAOutput, error) {
	if actor.ID == "" {
		return DisconnectZaloOAOutput{}, ErrNoActor
	}
	reason, err := domain.NormalizeCommuneZaloOARetireReason(rawReason)
	if err != nil {
		return DisconnectZaloOAOutput{}, err
	}
	at := uc.clock()
	var out DisconnectZaloOAOutput
	var retired string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.oas.LockLiveByID(ctx, tx, oaRowID)
		if err != nil {
			return err
		}
		if !found {
			return ErrCommuneZaloOAMissing
		}
		retired = cur.OA.OAID
		n, err := uc.retireInTx(ctx, tx, cur.OA, actor, reason, at, nil)
		if err != nil {
			return err
		}
		out = DisconnectZaloOAOutput{Retired: true, DetachedTemplateCount: n, RevokeNotice: domain.CommuneZaloOARevokeNotice}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCommuneZaloOAMissing) {
			return DisconnectZaloOAOutput{}, err
		}
		return DisconnectZaloOAOutput{}, fmt.Errorf("zalo oa: disconnect the OA: %w", err)
	}
	uc.securityEvent(ctx, SecEventCommuneOADisconnected, "retired", actor, retired)
	return out, nil
}

// SyncTemplates — "Nạp mẫu từ OA" (ADR 0098 #8) for ONE OA of the commune: template/all paged to the end
// (at most MaxSyncedTemplates), template/info for each, then ONE transaction that upserts every template,
// marks the ones Zalo no longer lists DELETE, and writes the entry. Zalo's columns are rewritten;
// milestones are kept — a template attached to a milestone that is no longer ENABLE stays attached (0031
// VENDOR CHOICES) and the screen shows the warning; MauTinXa refuses it at read time.
//
// A template whose detail cannot be read (Rejected / Malformed) or whose values fall outside 0031's
// CHECKs is SKIPPED and named in the result. A token refusal, a rate limit or an outage aborts the
// whole sync with the class — nothing is written on a partial read.
func (uc *CommuneZaloOAs) SyncTemplates(ctx context.Context, oaRowID string, actor audit.Actor) (TemplateSyncResult, error) {
	if actor.ID == "" {
		return TemplateSyncResult{}, ErrNoActor
	}
	s, access, _, err := uc.openAccess(ctx, oaRowID, actor)
	if err != nil {
		return TemplateSyncResult{}, err
	}
	defer clear(access)

	// 1. Zalo, outside any transaction.
	var listed []zalooa.TemplateSummary
	truncated := false
	for offset := 0; ; {
		page, total, o := uc.api.ListTemplates(ctx, access, offset, zalooa.MaxTemplatePageSize)
		if o != zalooa.OutcomeOK {
			uc.securityEvent(ctx, SecEventCommuneOATemplatesSync, "list_failed:"+oaOutcomeValue(o), actor, s.OA.OAID)
			return TemplateSyncResult{}, &ZaloOACallError{Step: "template/all", Class: oaOutcomeValue(o)}
		}
		listed = append(listed, page...)
		offset += len(page)
		if len(page) == 0 || (total >= 0 && offset >= total) {
			break
		}
		if offset >= MaxSyncedTemplates {
			truncated = true
			break
		}
	}
	var details []zalooa.TemplateDetail
	var skipped []string
	for _, sum := range listed {
		d, o := uc.api.TemplateInfo(ctx, access, sum.ID)
		switch {
		case o == zalooa.OutcomeOK:
		case o == zalooa.OutcomeRejected || o == zalooa.OutcomeMalformedResponse:
			skipped = append(skipped, sum.ID)
			continue
		default:
			uc.securityEvent(ctx, SecEventCommuneOATemplatesSync, "info_failed:"+oaOutcomeValue(o), actor, s.OA.OAID)
			return TemplateSyncResult{}, &ZaloOACallError{Step: "template/info", Class: oaOutcomeValue(o)}
		}
		if d.Status == "" {
			d.Status = sum.Status
		}
		if d.Quality == "" {
			d.Quality = sum.Quality
		}
		if d.Quality == "" {
			d.Quality = domain.ZaloTemplateQualityUndefined
		}
		if d.Name == "" {
			d.Name = sum.Name
		}
		if !domain.ValidZaloTemplateID(sum.ID) || !domain.ValidZaloTemplateStatus(d.Status) ||
			!domain.ValidZaloTemplateQuality(d.Quality) || len(d.Tag) > domain.MaxZaloTemplateTagLen {
			skipped = append(skipped, sum.ID)
			continue
		}
		d.ID = sum.ID // the id the list gave is the row key; a detail answering another id is not trusted over it
		details = append(details, d)
	}

	// 2. ONE transaction.
	at := uc.clock()
	out := TemplateSyncResult{SkippedTemplateIDs: skipped, Truncated: truncated, SyncedAt: at}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		_, found, err := uc.oas.LockLiveByID(ctx, tx, s.OA.ID)
		if err != nil {
			return err
		}
		if !found {
			return ErrCommuneZaloOAChanged
		}
		existing, err := uc.templates.LockByOA(ctx, tx, s.OA.ID)
		if err != nil {
			return err
		}
		before := make(map[string]domain.ZaloOATemplate, len(existing))
		for _, t := range existing {
			before[t.TemplateID] = t
		}
		seen := make(map[string]bool, len(details))
		for _, d := range details {
			seen[d.ID] = true
			params := make([]domain.TemplateParam, 0, len(d.Params))
			for _, p := range d.Params {
				params = append(params, domain.TemplateParam{Name: p.Name, Required: p.Required, Type: p.Type, MaxLength: p.MaxLength})
			}
			t := domain.ZaloOATemplate{TemplateID: d.ID, TemplateName: domain.ClipZaloOAName(d.Name), ZaloStatus: d.Status,
				ZaloQuality: d.Quality, ZaloTag: d.Tag, Params: params, ParamsMatch: len(domain.MissingTemplateParams(params)) == 0}
			prev, known := before[d.ID]
			if known {
				t.ID = prev.ID
			} else if t.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("zalo oa: new id: %w", err)
			}
			if err := uc.templates.UpsertFromSync(ctx, tx, s.OA.ID, t, at); err != nil {
				return err
			}
			out.Loaded++
			if t.ZaloStatus == domain.ZaloTemplateStatusEnabled {
				out.Enabled++
				if t.ParamsMatch {
					out.Matching++
				}
			}
			if !known || prev.ZaloStatus != t.ZaloStatus {
				out.ChangedTemplateIDs = append(out.ChangedTemplateIDs, t.TemplateID)
			}
		}
		if !truncated {
			// Not listed any more = gone at Zalo. Only when the whole list was read: a cut list says
			// nothing about the rest.
			for _, t := range existing {
				if seen[t.TemplateID] || t.ZaloStatus == domain.ZaloTemplateStatusDeleted {
					continue
				}
				if err := uc.templates.MarkDeleted(ctx, tx, t.ID, at); err != nil {
					return err
				}
				out.ChangedTemplateIDs = append(out.ChangedTemplateIDs, t.TemplateID)
			}
		}
		// Counts and template ids only — never a parameter list, never a name (ADR 0098 #12).
		return writeCommuneOAAudit(ctx, tx, actor, domain.ActionSyncZaloOATemplates, s.OA.Subject(), at, nil,
			map[string]any{"loaded": out.Loaded, "enabled": out.Enabled, "matching": out.Matching, "synced_at": at},
			map[string]any{"changed_template_ids": out.ChangedTemplateIDs, "skipped_template_ids": skipped, "truncated": truncated})
	})
	if err != nil {
		if errors.Is(err, ErrCommuneZaloOAChanged) {
			return TemplateSyncResult{}, err
		}
		return TemplateSyncResult{}, fmt.Errorf("zalo oa: record the templates: %w", err)
	}
	uc.securityEvent(ctx, SecEventCommuneOATemplatesSync, "synced", actor, s.OA.OAID)
	return out, nil
}

// SetMilestone — PATCH zalo-oa-templates/{id} (ADR 0098 #8, #9): attach (milestone non-nil) only an
// ENABLE template declaring the three parameters — the refusal names the missing ones; detach (nil)
// always. The template is found by its row id IN THIS COMMUNE, and its OA must still be live (a retired
// OA's template is not offered). The milestone stays ONE template in the whole commune whatever the
// number of OAs (ADR 0099 #1: the notice goes out through the OA of the attached template). One
// transaction with the entry. The same milestone already set is a no-op with no entry.
//
// THE LOCK ORDER IS THE OA's ROW, THEN THE TEMPLATE's — the order retireInTx and SyncTemplates take. The
// template is read unlocked first only to learn its OA; it is re-read under lock and must still belong to
// that OA. Locking the template first would deadlock against a concurrent "Ngắt OA".
func (uc *CommuneZaloOAs) SetMilestone(ctx context.Context, templateRowID string, milestone *string, actor audit.Actor) (
	domain.ZaloOATemplate, error) {

	if actor.ID == "" {
		return domain.ZaloOATemplate{}, ErrNoActor
	}
	if milestone != nil && !domain.ValidMilestone(*milestone) {
		return domain.ZaloOATemplate{}, domain.ErrZaloOAMilestoneUnknown
	}
	seen, found, err := uc.templates.Get(ctx, templateRowID)
	if err != nil {
		return domain.ZaloOATemplate{}, err
	}
	if !found {
		return domain.ZaloOATemplate{}, ErrZaloOATemplateMissing
	}
	at := uc.clock()
	var out domain.ZaloOATemplate
	var oaID string
	changed := false
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.oas.LockLiveByID(ctx, tx, seen.OARowID)
		if err != nil {
			return err
		}
		if !found {
			return ErrZaloOATemplateMissing // a retired OA's template is not a live OA's
		}
		oaID = cur.OA.OAID
		t, found, err := uc.templates.LockByID(ctx, tx, templateRowID)
		if err != nil {
			return err
		}
		if !found || t.OARowID != cur.OA.ID {
			return ErrZaloOATemplateMissing
		}
		after := t
		if milestone == nil {
			if !t.Attached() {
				out = t
				return nil
			}
			after.Milestone, after.MilestoneSetAt, after.MilestoneSetBy = "", time.Time{}, ""
		} else {
			if t.ZaloStatus != domain.ZaloTemplateStatusEnabled {
				return domain.ErrZaloOATemplateNotEnabled
			}
			if missing := domain.MissingTemplateParams(t.Params); len(missing) > 0 || !t.ParamsMatch {
				if len(missing) == 0 {
					missing = domain.RequiredTemplateParams
				}
				return &domain.TemplateParamsMissingError{Missing: missing}
			}
			if t.Milestone == *milestone {
				out = t
				return nil
			}
			after.Milestone, after.MilestoneSetAt, after.MilestoneSetBy = *milestone, at, actor.ID
		}
		if err := uc.templates.SetMilestone(ctx, tx, t.ID, milestone, at, actor.ID); err != nil {
			return err
		}
		changed = true
		out = after
		action := domain.ActionAttachZaloOATemplateMilestone
		if milestone == nil {
			action = domain.ActionDetachZaloOATemplateMilestone
		}
		return writeCommuneOAAudit(ctx, tx, actor, action, t.Subject(), at, templateMilestoneDelta(t), templateMilestoneDelta(after),
			map[string]any{"oa_id": cur.OA.OAID, "oa_row_id": cur.OA.ID, "zalo_status": t.ZaloStatus, "params_match": t.ParamsMatch})
	})
	if err != nil {
		if errors.Is(err, docstore.ErrMilestoneTaken) || errors.Is(err, domain.ErrCommuneZaloOAInvalid) ||
			errors.Is(err, ErrZaloOATemplateMissing) {
			return domain.ZaloOATemplate{}, err
		}
		return domain.ZaloOATemplate{}, fmt.Errorf("zalo oa: set the milestone: %w", err)
	}
	if changed {
		outcome := "attached"
		if milestone == nil {
			outcome = "detached"
		}
		uc.securityEvent(ctx, SecEventCommuneOAMilestone, outcome, actor, oaID)
	}
	return out, nil
}

// ---- deltas and the entry ---------------------------------------------------------------------------------

// communeOADelta is the audit view of an OA row — NEVER a sealed column (0031: "before/after never carry
// a *_sealed column"). Nothing here is personal data: an OA's id and name are an organisation's.
func communeOADelta(o domain.CommuneZaloOA) map[string]any {
	return map[string]any{
		"oa_id": o.OAID, "oa_name": o.OAName, "authorized_at": o.AuthorizedAt, "authorized_by": o.AuthorizedBy,
		"access_expires_at": o.AccessExpiresAt, "refresh_issued_at": o.RefreshIssuedAt, "token_refreshed_at": o.TokenRefreshedAt,
	}
}

func communeOADeltaPtr(o *domain.CommuneZaloOA) any {
	if o == nil {
		return nil
	}
	return communeOADelta(*o)
}

// templateMilestoneDelta is the commune's part of a template row — the milestone trio only.
func templateMilestoneDelta(t domain.ZaloOATemplate) map[string]any {
	m := map[string]any{"template_id": t.TemplateID, "milestone": nil}
	if t.Attached() {
		m["milestone"], m["milestone_set_at"], m["milestone_set_by"] = t.Milestone, t.MilestoneSetAt, t.MilestoneSetBy
	}
	return m
}

func writeCommuneOAAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action, subject string,
	at time.Time, before, after any, facts map[string]any) error {

	m := map[string]any{"truoc": before, "sau": after}
	for k, v := range facts {
		m[k] = v
	}
	delta, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("zalo oa: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: subject, At: at, Delta: delta})
}

// oaOutcomeValue maps the adapter's class onto the stored vocabulary (ADR 0011; 0022 / 0031 columns).
func oaOutcomeValue(o zalooa.Outcome) string {
	switch o {
	case zalooa.OutcomeOK:
		return domain.ZaloCallOK
	case zalooa.OutcomeTokenRejected:
		return domain.ZaloCallTokenRejected
	case zalooa.OutcomeRateLimited:
		return domain.ZaloCallRateLimited
	case zalooa.OutcomeUnavailable:
		return domain.ZaloCallUnavailable
	case zalooa.OutcomeRejected:
		return domain.ZaloCallRejected
	}
	return domain.ZaloCallMalformedResponse
}
