package store

// A COMMUNE'S ZALO OA — `zalo_commune_oa`, `zalo_oa_authorization`, `zalo_oa_template` (migration 0031;
// ADR 0098, block A). SQL, and nothing else.
//
// EVERY STATEMENT IS ONE COMMUNE'S: $1 is the commune, from the context or the transaction (rule 1,
// invariants 4 and 5). The two platform-wide unique keys (one live place per OA, one state hash) are
// the DATABASE's checks; nothing here reads another commune's row, and the only error they raise is
// turned into one generic sentence that names no commune (ErrZaloOATaken). The ONE cross-commune read —
// the callback's lookup by state hash — lives in crosstenant/zalo_oa.go, not here.
//
// THE SEALED COLUMNS (access token, refresh token, code verifier) are read only by the methods that
// exist to hand them to the use case (SealedByID / LockLiveByID / LockLiveByZaloOAID / OpenByID /
// LockOpenByID) and are never put in an error string. ListLive / LiveByID and the template reads — what
// the screen reads — cannot select them.
//
// SEVERAL LIVE OAs PER COMMUNE (ADR 0099 #1, migration 0032): every read of an OA is BY ROW ID (or by
// Zalo's oa_id for the callback); there is no "the commune's live OA" read.
//
// NO audit.Write HERE: internal/app opens the transaction and writes the entry inside it.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

var (
	// ErrZaloOATaken — the OA is live in ANOTHER commune (0031's zalo_commune_oa_one_live_per_oa floor).
	// WHICH commune is never said.
	ErrZaloOATaken = errors.New("zalo_commune_oa: the OA is live elsewhere")
	// ErrMilestoneTaken — another template of this commune already announces that milestone (0031's
	// zalo_oa_template_one_per_milestone; ADR 0098 #8 "một mốc một mẫu").
	ErrMilestoneTaken = errors.New("zalo_oa_template: the milestone is already attached to another template")
)

// ---- zalo_commune_oa ----------------------------------------------------------------------------------

// ZaloCommuneOAStore is the only scoped path to zalo_commune_oa.
type ZaloCommuneOAStore struct {
	db *store.DB
}

func NewZaloCommuneOAStore(db *store.DB) *ZaloCommuneOAStore { return &ZaloCommuneOAStore{db: db} }

// CommuneOASecrets is the live row with its sealed bytes. Neither is ever empty (NOT NULL).
type CommuneOASecrets struct {
	OA                 domain.CommuneZaloOA
	AccessTokenSealed  []byte
	RefreshTokenSealed []byte
}

// communeOAColumns IS READ BY POSITION in scanCommuneOA.
const communeOAColumns = `id, oa_id, oa_name, access_expires_at, refresh_issued_at, token_refreshed_at,
	authorized_at, authorized_by, last_check_at, last_check_result,
	needs_reauthorization_since, needs_reauthorization_reason, retired_at, retired_by, retire_reason`

const communeOASealedColumns = `, access_token_sealed, refresh_token_sealed`

func scanCommuneOA(scan func(...any) error, extra ...any) (domain.CommuneZaloOA, error) {
	var (
		o                            domain.CommuneZaloOA
		checkAt, reauthAt, retiredAt sql.NullTime
		checkRes, reauthReason       sql.NullString
		retiredBy, retireReason      sql.NullString
	)
	dest := append([]any{&o.ID, &o.OAID, &o.OAName, &o.AccessExpiresAt, &o.RefreshIssuedAt, &o.TokenRefreshedAt,
		&o.AuthorizedAt, &o.AuthorizedBy, &checkAt, &checkRes, &reauthAt, &reauthReason, &retiredAt, &retiredBy,
		&retireReason}, extra...)
	if err := scan(dest...); err != nil {
		return domain.CommuneZaloOA{}, err
	}
	o.LastCheckAt, o.LastCheckResult = checkAt.Time, checkRes.String
	o.NeedsReauthorizationSince, o.NeedsReauthorizationReason = reauthAt.Time, reauthReason.String
	o.RetiredAt, o.RetiredBy, o.RetireReason = retiredAt.Time, retiredBy.String, retireReason.String
	return o, nil
}

// ListLive reads EVERY live OA of the commune — metadata only, oldest first (0032's
// zalo_commune_oa_live_by_commune). ADR 0099 #1: a commune holds several OAs at once, so there is no
// "the live OA" any more; every other read below is BY ROW ID. Reading "the first live row" here would
// answer an arbitrary OA the day a commune has two — the screen describing one, the sender using another
// (0032 header, READ PATHS THAT CHANGE MEANING).
func (s *ZaloCommuneOAStore) ListLive(ctx context.Context) ([]domain.CommuneZaloOA, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, communeOAColumns, "zalo_commune_oa",
		"AND retired_at IS NULL ORDER BY created_at, id")
	if err != nil {
		return nil, fmt.Errorf("zalo_commune_oa: list: %w", err)
	}
	defer rows.Close()
	var out []domain.CommuneZaloOA
	for rows.Next() {
		o, err := scanCommuneOA(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("zalo_commune_oa: scan: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_commune_oa: list: %w", err)
	}
	return out, nil
}

// LiveByID reads one live OA of this commune — metadata only. found=false: no such row HERE, or it is
// retired. Another commune's id is not found: the scope is the context's commune.
func (s *ZaloCommuneOAStore) LiveByID(ctx context.Context, id string) (domain.CommuneZaloOA, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, communeOAColumns, "zalo_commune_oa", "AND id = $2 AND retired_at IS NULL", id)
	if err != nil {
		return domain.CommuneZaloOA{}, false, fmt.Errorf("zalo_commune_oa: read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.CommuneZaloOA{}, false, rowsErr(rows, "zalo_commune_oa: read")
	}
	o, err := scanCommuneOA(rows.Scan)
	if err != nil {
		return domain.CommuneZaloOA{}, false, fmt.Errorf("zalo_commune_oa: scan: %w", err)
	}
	return o, true, rows.Err()
}

// SealedByID reads one live OA WITH its sealed bytes, outside any transaction — for a Zalo call.
// found=false: no such live row in this commune.
func (s *ZaloCommuneOAStore) SealedByID(ctx context.Context, id string) (CommuneOASecrets, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, communeOAColumns+communeOASealedColumns, "zalo_commune_oa",
		"AND id = $2 AND retired_at IS NULL", id)
	if err != nil {
		return CommuneOASecrets{}, false, fmt.Errorf("zalo_commune_oa: read sealed: %w", err)
	}
	return firstSealedOA(rows)
}

// LockLiveByID reads one live OA with its sealed bytes and holds it until the transaction ends — every
// write below is read-decide-write. found=false: no such live row in this commune.
func (s *ZaloCommuneOAStore) LockLiveByID(ctx context.Context, tx *store.ScopedTx, id string) (CommuneOASecrets, bool, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, communeOAColumns+communeOASealedColumns, "zalo_commune_oa",
		"AND id = $2 AND retired_at IS NULL FOR UPDATE", id)
	if err != nil {
		return CommuneOASecrets{}, false, fmt.Errorf("zalo_commune_oa: lock: %w", err)
	}
	return firstSealedOA(rows)
}

// LockLiveByZaloOAID locks the live row of THIS commune carrying Zalo's oa_id — the callback's question
// "is this very OA already connected here?" (ADR 0099 #1 keeps ADR 0098 #11 for the SAME OA: a
// re-authorization retires the old row and inserts a new one). At most one row: 0031's
// zalo_commune_oa_one_live_per_oa is platform-wide. found=false: not live here (it may be live in another
// commune — that is the insert's 23505, never a read of the other commune).
func (s *ZaloCommuneOAStore) LockLiveByZaloOAID(ctx context.Context, tx *store.ScopedTx, oaID string) (CommuneOASecrets, bool, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, communeOAColumns+communeOASealedColumns, "zalo_commune_oa",
		"AND oa_id = $2 AND retired_at IS NULL FOR UPDATE", oaID)
	if err != nil {
		return CommuneOASecrets{}, false, fmt.Errorf("zalo_commune_oa: lock by oa_id: %w", err)
	}
	return firstSealedOA(rows)
}

func firstSealedOA(rows *sql.Rows) (CommuneOASecrets, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return CommuneOASecrets{}, false, rowsErr(rows, "zalo_commune_oa: read sealed")
	}
	var out CommuneOASecrets
	o, err := scanCommuneOA(rows.Scan, &out.AccessTokenSealed, &out.RefreshTokenSealed)
	if err != nil {
		return CommuneOASecrets{}, false, fmt.Errorf("zalo_commune_oa: scan sealed: %w", err)
	}
	out.OA = o
	return out, true, rows.Err()
}

// insertCommuneOA — token_refreshed_at starts equal to authorized_at (the connecting member of staff is
// the first writer of the sealed columns). last_check_* is the getoa that answered at connection, if any.
const insertCommuneOA = `INSERT INTO zalo_commune_oa (tenant_id, id, oa_id, oa_name, access_token_sealed, refresh_token_sealed,
		access_expires_at, refresh_issued_at, token_refreshed_at, authorized_at, authorized_by,
		last_check_at, last_check_result, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10, $11, $12, $9, $9)`

// InsertLive connects an OA: a new live row in the transaction's commune. ErrZaloOATaken when the OA is
// live elsewhere (0031's one_live_per_oa index, 23505 naming it) — the caller retired this commune's own
// live row of the same OA first (LockLiveByZaloOAID), so what is left is another commune. 0032 dropped the
// one-live-per-commune index (ADR 0099 #1): a second, different OA is simply another row. The transaction
// is then aborted; the caller rolls it back.
func (s *ZaloCommuneOAStore) InsertLive(ctx context.Context, tx *store.ScopedTx, o domain.CommuneZaloOA,
	accessSealed, refreshSealed []byte) error {

	var checkAt any
	var checkRes any
	if !o.LastCheckAt.IsZero() {
		checkAt, checkRes = o.LastCheckAt.UTC(), o.LastCheckResult
	}
	if _, err := tx.Exec(ctx, insertCommuneOA, string(tx.TenantID()), o.ID, o.OAID, o.OAName, accessSealed, refreshSealed,
		o.AccessExpiresAt.UTC(), o.RefreshIssuedAt.UTC(), o.AuthorizedAt.UTC(), o.AuthorizedBy, checkAt, checkRes); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "zalo_commune_oa_one_live_per_oa" {
			return ErrZaloOATaken
		}
		return fmt.Errorf("zalo_commune_oa: insert: %w", err)
	}
	return nil
}

// resealCommuneOA — both tokens together (Zalo voids the old pair), with the new token_refreshed_at the
// guard demands. Guarded on the token_refreshed_at the caller READ: a concurrent refresh that already
// moved it means the presented refresh token was spent by the other writer, and this one must not win.
const resealCommuneOA = `UPDATE zalo_commune_oa SET access_token_sealed = $3, refresh_token_sealed = $4,
		access_expires_at = $5, refresh_issued_at = $6, token_refreshed_at = $7, updated_at = $7
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL AND token_refreshed_at = $8`

// Reseal writes a fresh token pair onto the live row. seenRefreshedAt is the token_refreshed_at the
// caller read before calling Zalo; a row that moved meanwhile is ErrZaloRowChanged.
func (s *ZaloCommuneOAStore) Reseal(ctx context.Context, tx *store.ScopedTx, id string, accessSealed, refreshSealed []byte,
	accessExpiresAt, refreshIssuedAt, refreshedAt, seenRefreshedAt time.Time) error {

	return execOne(ctx, tx, resealCommuneOA, "zalo_commune_oa: reseal", ErrZaloRowChanged,
		string(tx.TenantID()), id, accessSealed, refreshSealed, accessExpiresAt.UTC(), refreshIssuedAt.UTC(),
		refreshedAt.UTC(), seenRefreshedAt.UTC())
}

const recordCommuneOACheck = `UPDATE zalo_commune_oa SET last_check_at = $3, last_check_result = $4, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// RecordCheck stores the last "Kiểm tra kết nối" — a ZaloCall* class, never Zalo's text.
func (s *ZaloCommuneOAStore) RecordCheck(ctx context.Context, tx *store.ScopedTx, id, result string, at time.Time) error {
	return execOne(ctx, tx, recordCommuneOACheck, "zalo_commune_oa: record check", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC(), result)
}

const setCommuneOAName = `UPDATE zalo_commune_oa SET oa_name = $3, updated_at = $4
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// SetName stores the display name getoa answered.
func (s *ZaloCommuneOAStore) SetName(ctx context.Context, tx *store.ScopedTx, id, name string, at time.Time) error {
	return execOne(ctx, tx, setCommuneOAName, "zalo_commune_oa: set name", ErrZaloRowChanged,
		string(tx.TenantID()), id, name, at.UTC())
}

// markNeedsReauthorization — only an unmarked row; a second marking would move `since`, which is the
// instant the screen and the ledger report.
const markNeedsReauthorization = `UPDATE zalo_commune_oa SET needs_reauthorization_since = $3,
		needs_reauthorization_reason = $4, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL AND needs_reauthorization_since IS NULL`

// MarkNeedsReauthorization flags the live row (ADR 0098 #6). reason is a Reauthorization* value. A row
// already flagged, or gone, is ErrZaloRowChanged.
func (s *ZaloCommuneOAStore) MarkNeedsReauthorization(ctx context.Context, tx *store.ScopedTx, id, reason string,
	at time.Time) error {

	return execOne(ctx, tx, markNeedsReauthorization, "zalo_commune_oa: mark needs reauthorization", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC(), reason)
}

const retireCommuneOA = `UPDATE zalo_commune_oa SET retired_at = $3, retired_by = $4, retire_reason = $5, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// Retire takes the live OA out of service (the row stays, frozen, with its sealed tokens as history —
// rule 7; 0031). The caller detaches the OA's templates in the same transaction (ADR 0098 #11).
func (s *ZaloCommuneOAStore) Retire(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	return execOne(ctx, tx, retireCommuneOA, "zalo_commune_oa: retire", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC(), by, reason)
}

// ---- zalo_oa_authorization ----------------------------------------------------------------------------

// ZaloOAAuthorizationStore is the only scoped path to zalo_oa_authorization. The callback's FIRST step —
// state hash → commune — is crosstenant.ZaloOA; everything after it comes here, in that commune's
// context.
type ZaloOAAuthorizationStore struct {
	db *store.DB
}

func NewZaloOAAuthorizationStore(db *store.DB) *ZaloOAAuthorizationStore {
	return &ZaloOAAuthorizationStore{db: db}
}

// OpenAuthorization is one request with its sealed verifier — what the callback needs to call Zalo.
type OpenAuthorization struct {
	Auth               domain.ZaloOAAuthorization
	CodeVerifierSealed []byte
}

// authorizationColumns IS READ BY POSITION in scanAuthorization.
const authorizationColumns = `id, requested_at, requested_by, expires_at, used_at, outcome, oa_row_id, code_verifier_sealed`

func scanAuthorization(scan func(...any) error) (OpenAuthorization, error) {
	var (
		out     OpenAuthorization
		usedAt  sql.NullTime
		outcome sql.NullString
		oaRowID sql.NullString
	)
	a := &out.Auth
	if err := scan(&a.ID, &a.RequestedAt, &a.RequestedBy, &a.ExpiresAt, &usedAt, &outcome, &oaRowID,
		&out.CodeVerifierSealed); err != nil {
		return OpenAuthorization{}, err
	}
	a.UsedAt, a.Outcome, a.OARowID = usedAt.Time, outcome.String, oaRowID.String
	return out, nil
}

const insertAuthorization = `INSERT INTO zalo_oa_authorization (tenant_id, id, state_hash, code_verifier_sealed,
		requested_at, requested_by, expires_at, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $5)`

// Insert records a new link handed out. stateHash is the SHA-256 of the state (32 bytes — 0031's CHECK),
// verifierSealed the sealed PKCE verifier.
func (s *ZaloOAAuthorizationStore) Insert(ctx context.Context, tx *store.ScopedTx, a domain.ZaloOAAuthorization,
	stateHash, verifierSealed []byte) error {

	if _, err := tx.Exec(ctx, insertAuthorization, string(tx.TenantID()), a.ID, stateHash, verifierSealed,
		a.RequestedAt.UTC(), a.RequestedBy, a.ExpiresAt.UTC()); err != nil {
		return fmt.Errorf("zalo_oa_authorization: insert: %w", err)
	}
	return nil
}

// OpenByID reads one request of this commune by id, outside any transaction — the callback's read before
// it calls Zalo. found=false: no such row, or already used. Expiry is the caller's check (its clock).
func (s *ZaloOAAuthorizationStore) OpenByID(ctx context.Context, id string) (OpenAuthorization, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, authorizationColumns, "zalo_oa_authorization", "AND id = $2 AND used_at IS NULL", id)
	if err != nil {
		return OpenAuthorization{}, false, fmt.Errorf("zalo_oa_authorization: read: %w", err)
	}
	return firstAuthorization(rows)
}

// LockOpenByID is OpenByID inside the transaction, held until it ends — the row is closed under this lock.
func (s *ZaloOAAuthorizationStore) LockOpenByID(ctx context.Context, tx *store.ScopedTx, id string) (OpenAuthorization, bool, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, authorizationColumns, "zalo_oa_authorization", "AND id = $2 AND used_at IS NULL FOR UPDATE", id)
	if err != nil {
		return OpenAuthorization{}, false, fmt.Errorf("zalo_oa_authorization: lock: %w", err)
	}
	return firstAuthorization(rows)
}

func firstAuthorization(rows *sql.Rows) (OpenAuthorization, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return OpenAuthorization{}, false, rowsErr(rows, "zalo_oa_authorization: read")
	}
	out, err := scanAuthorization(rows.Scan)
	if err != nil {
		return OpenAuthorization{}, false, fmt.Errorf("zalo_oa_authorization: scan: %w", err)
	}
	return out, true, rows.Err()
}

const closeAuthorization = `UPDATE zalo_oa_authorization SET used_at = $3, outcome = $4, oa_row_id = $5
	WHERE tenant_id = $1 AND id = $2 AND used_at IS NULL`

// Close spends the request with its outcome (0031: used whatever the outcome). oaRowID is "" unless the
// outcome is thanh-cong, where it names the OA row produced (0031's CHECK holds both ways).
func (s *ZaloOAAuthorizationStore) Close(ctx context.Context, tx *store.ScopedTx, id string, usedAt time.Time,
	outcome, oaRowID string) error {

	var row any
	if oaRowID != "" {
		row = oaRowID
	}
	return execOne(ctx, tx, closeAuthorization, "zalo_oa_authorization: close", ErrZaloRowChanged,
		string(tx.TenantID()), id, usedAt.UTC(), outcome, row)
}

// expireOpenAuthorizations — the sweep's write (0031 zalo_oa_authorization_open_by_expiry).
const expireOpenAuthorizations = `UPDATE zalo_oa_authorization SET used_at = $3, outcome = $4
	WHERE tenant_id = $1 AND used_at IS NULL AND expires_at < $2
	RETURNING id`

// ExpireOpen closes every open request of this commune whose expiry is before `before`, as het-han, and
// returns their ids for the caller's one audit entry.
func (s *ZaloOAAuthorizationStore) ExpireOpen(ctx context.Context, tx *store.ScopedTx, before, usedAt time.Time) ([]string, error) {
	// THE TENANT SCOPE HERE IS MANUAL: tx.Underlying() bypasses ScopedTx's own binding (an UPDATE …
	// RETURNING is neither its Query nor its Exec), so `tenant_id = $1` in the statement and
	// tx.TenantID() as $1 below are what keep this to the transaction's commune. Keep both.
	rows, err := tx.Underlying().QueryContext(ctx, expireOpenAuthorizations, string(tx.TenantID()), before.UTC(), usedAt.UTC(),
		domain.AuthorizationExpired)
	if err != nil {
		return nil, fmt.Errorf("zalo_oa_authorization: expire: %w", err)
	}
	return scanIDs(rows, "zalo_oa_authorization: expire")
}

func scanIDs(rows *sql.Rows, what string) ([]string, error) {
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", what, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return ids, nil
}

// ---- zalo_oa_template ---------------------------------------------------------------------------------

// ZaloOATemplateStore is the only scoped path to zalo_oa_template.
type ZaloOATemplateStore struct {
	db *store.DB
}

func NewZaloOATemplateStore(db *store.DB) *ZaloOATemplateStore { return &ZaloOATemplateStore{db: db} }

// templateColumns IS READ BY POSITION in scanTemplate. No sealed column exists on this table.
const templateColumns = `id, oa_row_id, template_id, template_name, zalo_status, zalo_quality, zalo_tag, params, params_match,
	milestone, milestone_set_at, milestone_set_by, synced_at`

// storedParam is 0031's `params` element: {name, required, type, max_length}.
type storedParam struct {
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	Type      string `json:"type"`
	MaxLength int    `json:"max_length"`
}

func encodeParams(ps []domain.TemplateParam) ([]byte, error) {
	out := make([]storedParam, 0, len(ps))
	for _, p := range ps {
		out = append(out, storedParam{Name: p.Name, Required: p.Required, Type: p.Type, MaxLength: p.MaxLength})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("zalo_oa_template: encode params: %w", err)
	}
	return b, nil
}

func decodeParams(b []byte) ([]domain.TemplateParam, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var in []storedParam
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("zalo_oa_template: decode params: %w", err)
	}
	out := make([]domain.TemplateParam, 0, len(in))
	for _, p := range in {
		out = append(out, domain.TemplateParam{Name: p.Name, Required: p.Required, Type: p.Type, MaxLength: p.MaxLength})
	}
	return out, nil
}

func scanTemplate(scan func(...any) error, extra ...any) (domain.ZaloOATemplate, error) {
	var (
		t         domain.ZaloOATemplate
		params    []byte
		milestone sql.NullString
		setAt     sql.NullTime
		setBy     sql.NullString
	)
	dest := append([]any{&t.ID, &t.OARowID, &t.TemplateID, &t.TemplateName, &t.ZaloStatus, &t.ZaloQuality, &t.ZaloTag,
		&params, &t.ParamsMatch, &milestone, &setAt, &setBy, &t.SyncedAt}, extra...)
	if err := scan(dest...); err != nil {
		return domain.ZaloOATemplate{}, err
	}
	ps, err := decodeParams(params)
	if err != nil {
		return domain.ZaloOATemplate{}, err
	}
	t.Params = ps
	t.Milestone, t.MilestoneSetAt, t.MilestoneSetBy = milestone.String, setAt.Time, setBy.String
	return t, nil
}

func scanTemplates(rows *sql.Rows, what string) ([]domain.ZaloOATemplate, error) {
	defer rows.Close()
	var out []domain.ZaloOATemplate
	for rows.Next() {
		t, err := scanTemplate(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", what, err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return out, nil
}

// ListByOA reads the templates loaded from one OA row of this commune, attached first, then by name.
func (s *ZaloOATemplateStore) ListByOA(ctx context.Context, oaRowID string) ([]domain.ZaloOATemplate, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, templateColumns, "zalo_oa_template",
		"AND oa_row_id = $2 ORDER BY milestone IS NULL, template_name, template_id", oaRowID)
	if err != nil {
		return nil, fmt.Errorf("zalo_oa_template: list: %w", err)
	}
	return scanTemplates(rows, "zalo_oa_template: list")
}

// LockByOA is ListByOA inside the transaction, rows held until it ends — the sync's read-decide-write.
func (s *ZaloOATemplateStore) LockByOA(ctx context.Context, tx *store.ScopedTx, oaRowID string) ([]domain.ZaloOATemplate, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, templateColumns, "zalo_oa_template", "AND oa_row_id = $2 ORDER BY template_id FOR UPDATE", oaRowID)
	if err != nil {
		return nil, fmt.Errorf("zalo_oa_template: lock: %w", err)
	}
	return scanTemplates(rows, "zalo_oa_template: lock")
}

// Get reads one template row of this commune. found=false: no such row here.
func (s *ZaloOATemplateStore) Get(ctx context.Context, id string) (domain.ZaloOATemplate, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, templateColumns, "zalo_oa_template", "AND id = $2", id)
	if err != nil {
		return domain.ZaloOATemplate{}, false, fmt.Errorf("zalo_oa_template: read: %w", err)
	}
	return firstTemplate(rows)
}

// GetByZaloTemplateID reads the template Zalo calls templateID under one OA row of this commune — the
// sender's read (ADR 0099 #8: a campaign names the OA and Zalo's template id). 0031's unique key
// (tenant_id, oa_row_id, template_id) makes it at most one row. found=false: not loaded from that OA here.
func (s *ZaloOATemplateStore) GetByZaloTemplateID(ctx context.Context, oaRowID, templateID string) (domain.ZaloOATemplate, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, templateColumns, "zalo_oa_template", "AND oa_row_id = $2 AND template_id = $3",
		oaRowID, templateID)
	if err != nil {
		return domain.ZaloOATemplate{}, false, fmt.Errorf("zalo_oa_template: read by template id: %w", err)
	}
	return firstTemplate(rows)
}

// TemplateCounts is how many templates were loaded from one OA row, and how many carry a milestone.
type TemplateCounts struct {
	Templates int
	Attached  int
}

// CountsByOA counts this commune's templates per OA row in ONE read — the OA list shows the counts of
// every live OA (ADR 0099 #1), and a read per OA would grow with the list. Retired OAs' rows are counted
// too; the caller keeps the ones it lists.
func (s *ZaloOATemplateStore) CountsByOA(ctx context.Context) (map[string]TemplateCounts, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, "oa_row_id, count(*), count(milestone)", "zalo_oa_template", "GROUP BY oa_row_id")
	if err != nil {
		return nil, fmt.Errorf("zalo_oa_template: count: %w", err)
	}
	defer rows.Close()
	out := map[string]TemplateCounts{}
	for rows.Next() {
		var id string
		var c TemplateCounts
		if err := rows.Scan(&id, &c.Templates, &c.Attached); err != nil {
			return nil, fmt.Errorf("zalo_oa_template: count: scan: %w", err)
		}
		out[id] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_oa_template: count: %w", err)
	}
	return out, nil
}

// LockByID is Get inside the transaction, held until it ends.
func (s *ZaloOATemplateStore) LockByID(ctx context.Context, tx *store.ScopedTx, id string) (domain.ZaloOATemplate, bool, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, templateColumns, "zalo_oa_template", "AND id = $2 FOR UPDATE", id)
	if err != nil {
		return domain.ZaloOATemplate{}, false, fmt.Errorf("zalo_oa_template: lock: %w", err)
	}
	return firstTemplate(rows)
}

func firstTemplate(rows *sql.Rows) (domain.ZaloOATemplate, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return domain.ZaloOATemplate{}, false, rowsErr(rows, "zalo_oa_template: read")
	}
	t, err := scanTemplate(rows.Scan)
	if err != nil {
		return domain.ZaloOATemplate{}, false, fmt.Errorf("zalo_oa_template: scan: %w", err)
	}
	return t, true, rows.Err()
}

// upsertTemplate — Zalo's columns rewritten, the commune's milestone* untouched (0031: "milestone* is
// the commune's and survives syncs"). The conflict target is the per-OA-row unique key.
const upsertTemplate = `INSERT INTO zalo_oa_template (tenant_id, id, oa_row_id, template_id, template_name, zalo_status,
		zalo_quality, zalo_tag, params, params_match, synced_at, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $11)
	ON CONFLICT (tenant_id, oa_row_id, template_id) DO UPDATE SET
		template_name = EXCLUDED.template_name, zalo_status = EXCLUDED.zalo_status, zalo_quality = EXCLUDED.zalo_quality,
		zalo_tag = EXCLUDED.zalo_tag, params = EXCLUDED.params, params_match = EXCLUDED.params_match,
		synced_at = EXCLUDED.synced_at, updated_at = EXCLUDED.updated_at`

// UpsertFromSync writes what Zalo answered for one template of oaRowID. t.ID is the id to use IF the row
// is new (ignored on an existing row); t.Milestone* is never written here. The caller computed
// t.ParamsMatch (ADR 0098 #9) and validated status / quality / tag against 0031's CHECKs.
func (s *ZaloOATemplateStore) UpsertFromSync(ctx context.Context, tx *store.ScopedTx, oaRowID string, t domain.ZaloOATemplate,
	syncedAt time.Time) error {

	params, err := encodeParams(t.Params)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, upsertTemplate, string(tx.TenantID()), t.ID, oaRowID, t.TemplateID, t.TemplateName,
		t.ZaloStatus, t.ZaloQuality, t.ZaloTag, params, t.ParamsMatch, syncedAt.UTC()); err != nil {
		return fmt.Errorf("zalo_oa_template: upsert: %w", err)
	}
	return nil
}

const markTemplateDeleted = `UPDATE zalo_oa_template SET zalo_status = $3, synced_at = $4, updated_at = $4
	WHERE tenant_id = $1 AND id = $2`

// MarkDeleted records a template Zalo no longer lists as zalo_status = 'DELETE' (0031: "a template that
// vanished at Zalo is zalo_status = 'DELETE'"). The row and its milestone stay.
func (s *ZaloOATemplateStore) MarkDeleted(ctx context.Context, tx *store.ScopedTx, id string, syncedAt time.Time) error {
	return execOne(ctx, tx, markTemplateDeleted, "zalo_oa_template: mark deleted", ErrZaloRowChanged,
		string(tx.TenantID()), id, domain.ZaloTemplateStatusDeleted, syncedAt.UTC())
}

// setTemplateMilestone — attach / re-attach / detach in one statement: the trio moves together (0031
// milestone_complete), and milestone_set_at always moves (the guard).
const setTemplateMilestone = `UPDATE zalo_oa_template SET milestone = $3, milestone_set_at = $4, milestone_set_by = $5, updated_at = $6
	WHERE tenant_id = $1 AND id = $2`

// SetMilestone attaches (milestone non-nil) or detaches (nil) one template. by is the staff code — stored
// on attach, NULL on detach (0031's trio is all or none; the detaching actor is the audit entry's).
// ErrMilestoneTaken when another template of the commune already carries that milestone.
func (s *ZaloOATemplateStore) SetMilestone(ctx context.Context, tx *store.ScopedTx, id string, milestone *string,
	at time.Time, by string) error {

	var m, setAt, setBy any
	if milestone != nil {
		m, setAt, setBy = *milestone, at.UTC(), by
	}
	res, err := tx.Exec(ctx, setTemplateMilestone, string(tx.TenantID()), id, m, setAt, setBy, at.UTC())
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "zalo_oa_template_one_per_milestone" {
			return ErrMilestoneTaken
		}
		return fmt.Errorf("zalo_oa_template: set milestone: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("zalo_oa_template: set milestone: %w", err)
	}
	if n == 0 {
		return ErrZaloRowChanged
	}
	return nil
}

// detachAllOfOA — every attached template of one OA row (ADR 0098 #11 "gỡ mọi mốc đã gắn", in the same
// transaction as the retire; 0031: the next OA's template cannot take the milestone otherwise).
const detachAllOfOA = `UPDATE zalo_oa_template SET milestone = NULL, milestone_set_at = NULL, milestone_set_by = NULL, updated_at = $3
	WHERE tenant_id = $1 AND oa_row_id = $2 AND milestone IS NOT NULL
	RETURNING id`

// DetachAllOfOA detaches every milestone of oaRowID's templates and returns the detached rows' ids for
// the caller's audit entry.
func (s *ZaloOATemplateStore) DetachAllOfOA(ctx context.Context, tx *store.ScopedTx, oaRowID string, at time.Time) ([]string, error) {
	// THE TENANT SCOPE HERE IS MANUAL (see ExpireOpen): `tenant_id = $1` in the statement and
	// tx.TenantID() as $1 keep this UPDATE … RETURNING to the transaction's commune.
	rows, err := tx.Underlying().QueryContext(ctx, detachAllOfOA, string(tx.TenantID()), oaRowID, at.UTC())
	if err != nil {
		return nil, fmt.Errorf("zalo_oa_template: detach all: %w", err)
	}
	return scanIDs(rows, "zalo_oa_template: detach all")
}

// AttachedTemplate is MauTinXa's read: the template attached to a milestone AND the OA row it belongs to,
// so the resolver can refuse on the OA's state as well as the template's (0031 question 4). With several
// OAs per commune (ADR 0099 #1) the OA row is also WHICH OA sends the milestone's notice: Template.OARowID
// = OA.ID. The milestone stays ONE template per commune (zalo_oa_template_one_per_milestone kept).
type AttachedTemplate struct {
	Template domain.ZaloOATemplate
	OA       domain.CommuneZaloOA
}

// attachedForMilestone — a JOIN, so BOTH tables are constrained to $1 (core/store.QueryJoin's contract):
// a template row joined to an OA row by id alone would match another commune's OA wherever ids collide.
const attachedForMilestone = `SELECT t.` + "id, t.oa_row_id, t.template_id, t.template_name, t.zalo_status, t.zalo_quality, t.zalo_tag, t.params, t.params_match, t.milestone, t.milestone_set_at, t.milestone_set_by, t.synced_at" + `,
		o.id, o.oa_id, o.oa_name, o.access_expires_at, o.refresh_issued_at, o.token_refreshed_at,
		o.authorized_at, o.authorized_by, o.last_check_at, o.last_check_result,
		o.needs_reauthorization_since, o.needs_reauthorization_reason, o.retired_at, o.retired_by, o.retire_reason
	FROM zalo_oa_template t
	JOIN zalo_commune_oa o ON o.tenant_id = $1 AND o.id = t.oa_row_id
	WHERE t.tenant_id = $1 AND t.milestone = $2`

// AttachedForMilestone reads the one template of this commune attached to milestone, with its OA row.
// found=false: nothing attached. The OA may be retired or flagged — the caller decides.
func (s *ZaloOATemplateStore) AttachedForMilestone(ctx context.Context, milestone string) (AttachedTemplate, bool, error) {
	// Scoped.QueryJoin binds the context's commune as $1; both tables above are constrained to it.
	rows, err := s.db.For(ctx).QueryJoin(ctx, attachedForMilestone, milestone)
	if err != nil {
		return AttachedTemplate{}, false, fmt.Errorf("zalo_oa_template: attached for milestone: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return AttachedTemplate{}, false, rowsErr(rows, "zalo_oa_template: attached for milestone")
	}
	var (
		out                          AttachedTemplate
		params                       []byte
		milestoneCol, setBy          sql.NullString
		setAt                        sql.NullTime
		checkAt, reauthAt, retiredAt sql.NullTime
		checkRes, reauthReason       sql.NullString
		retiredBy, retireReason      sql.NullString
	)
	t, o := &out.Template, &out.OA
	if err := rows.Scan(&t.ID, &t.OARowID, &t.TemplateID, &t.TemplateName, &t.ZaloStatus, &t.ZaloQuality, &t.ZaloTag,
		&params, &t.ParamsMatch, &milestoneCol, &setAt, &setBy, &t.SyncedAt,
		&o.ID, &o.OAID, &o.OAName, &o.AccessExpiresAt, &o.RefreshIssuedAt, &o.TokenRefreshedAt,
		&o.AuthorizedAt, &o.AuthorizedBy, &checkAt, &checkRes, &reauthAt, &reauthReason, &retiredAt, &retiredBy,
		&retireReason); err != nil {
		return AttachedTemplate{}, false, fmt.Errorf("zalo_oa_template: scan attached: %w", err)
	}
	ps, err := decodeParams(params)
	if err != nil {
		return AttachedTemplate{}, false, err
	}
	t.Params = ps
	t.Milestone, t.MilestoneSetAt, t.MilestoneSetBy = milestoneCol.String, setAt.Time, setBy.String
	o.LastCheckAt, o.LastCheckResult = checkAt.Time, checkRes.String
	o.NeedsReauthorizationSince, o.NeedsReauthorizationReason = reauthAt.Time, reauthReason.String
	o.RetiredAt, o.RetiredBy, o.RetireReason = retiredAt.Time, retiredBy.String, retireReason.String
	return out, true, rows.Err()
}
