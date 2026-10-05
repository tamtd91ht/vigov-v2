// Package platformstore is service-comms' store for its PLATFORM-SCOPE tables (migration 0018, ADR
// 0074): `platform_data_encryption_key`, `zalo_bot_shared`, `platform_operator_audit_log` — and the two
// statements that cross every commune because the shared bot does: ending every live staff link on a
// change of bot account, and the per-commune counts of the operator screen.
//
// WHY IT IS UNSCOPED. core/store.For(ctx) panics without a commune, and these tables belong to no
// commune BY DESIGN (ADR 0074 #4: "không dùng một tenant_id giả"). Inventing one to satisfy core/store
// would be a default on the isolation path (rule 1, forbidden #1). So this package takes the raw
// *sql.DB — the service-identity operatorstore precedent — and every statement carries its own
// `// @cross-tenant:` mark (rule 1, forbidden #6).
//
// WHY A SIBLING OF crosstenant AND NOT A FILE IN IT: crosstenant/portal_sync.go admits exactly commune
// IDENTIFIERS and advisory locks, "never a read returning business rows of more than one commune". The
// platform tables are not about any commune; the two statements below that touch commune tables are
// the shared bot's, and keeping them here keeps both packages' lists short and countable.
//
// WHAT CROSSES COMMUNES, EXACTLY:
//
//   - Tx.EndAllLiveLinks — a WRITE across every commune's zalo_link rows of the shared bot, owner-decided
//     (05/10/2026): a different bot account ends every link. Each ended link is audited IN ITS OWN
//     COMMUNE's audit_log, in the same statement, attributed to the operator (0018 header: "unlinking
//     (including the links a bot-account change ends) … are entries in the existing comms audit_log").
//   - Tx.CountLiveLinks — a count, no row.
//   - Tx.CommuneStats — per-commune counts and a switch; tenant ids, never a person.
//
// THE HANDLE DOES NOT LEAVE. Store hands out only *Tx, whose methods are the statements below.
//
// AUDIT IN THE SAME TRANSACTION (rule 6, invariant 3): every write here is a method on *Tx, and
// Tx.AppendOperatorAudit writes platform_operator_audit_log inside that same transaction. The use
// case decides WHICH entry; this package guarantees the two cannot commit apart. The platform DEK's
// own writes (PlatformDEKStore) open their own short transaction and write their own entry in it,
// attributed to the operator the caller put in the context (WithOperator) — refused without one.
//
// NEVER IN AN ERROR, A LOG OR AN ENTRY: a sealed column, a wrapped DEK, a chat_id.
package platformstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

var (
	// ErrNoOperator — a platform DEK write without an operator in the context: there would be no one to
	// name in the trail (rule 6 invariant 8; the table's CHECK refuses anything but a VH- code).
	ErrNoOperator = errors.New("platformstore: no operator in the context for the platform data key's trail")
	// ErrNotConfigured — the shared bot row does not exist.
	ErrNotConfigured = errors.New("platformstore: the shared Zalo Bot is not configured")
	// ErrChanged — a compare-and-swap found the row changed underneath (another operator act got there
	// first). Nothing written; retrying is safe.
	ErrChanged = errors.New("platformstore: the shared Zalo Bot changed underneath this write")
	// ErrWriteWithoutTrail — a transaction wrote and appended no operator trail entry. Rolled back: the
	// change and its entry commit together or not at all (rule 6, invariant 3).
	ErrWriteWithoutTrail = errors.New("platformstore: a platform write without its operator trail entry was rolled back")
)

// Operator is the person a platform-scope write is attributed to.
type Operator struct {
	Code string // VH- business code (rule 6, invariant 8)
	IP   string // as domain.NormalizeActorIP returned it
}

type operatorKey struct{}

// WithOperator puts the acting operator into ctx for the platform DEK store's trail. The DEK is created
// by the FIRST seal, inside an operator's act; crypto.PlatformDEKStore carries only a context.
func WithOperator(ctx context.Context, op Operator) context.Context {
	return context.WithValue(ctx, operatorKey{}, op)
}

func operatorFrom(ctx context.Context) (Operator, bool) {
	op, ok := ctx.Value(operatorKey{}).(Operator)
	return op, ok && domain.ValidOperatorCode(op.Code) && op.IP != ""
}

// Store holds the pool and hands out nothing but *Tx.
type Store struct{ db *sql.DB }

// New wraps the process pool.
func New(db *sql.DB) *Store { return &Store{db: db} }

var _ crypto.PlatformDEKStore = (*Store)(nil)

// Tx is one transaction on the platform tables.
//
// IT KNOWS WHETHER IT WROTE AND WHETHER IT WAS AUDITED. Every write method marks `wrote`;
// AppendOperatorAudit marks `audited`; InTx refuses to COMMIT a transaction that wrote without an
// entry (ErrWriteWithoutTrail). The use case decides which entry — only it knows the business fact —
// and this type makes forgetting it a rollback instead of an unattributed change.
type Tx struct {
	tx      *sql.Tx
	wrote   bool
	audited bool
}

// InTx runs fn in one transaction. A panic rolls back and keeps propagating (core/store.Tx's rule).
func (s *Store) InTx(ctx context.Context, fn func(*Tx) error) error {
	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("platformstore: begin: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			_ = sqlTx.Rollback()
			panic(r)
		}
	}()
	t := &Tx{tx: sqlTx}
	err = fn(t)
	if err == nil && t.wrote && !t.audited {
		err = ErrWriteWithoutTrail
	}
	if err != nil {
		if rbErr := sqlTx.Rollback(); rbErr != nil {
			return fmt.Errorf("platformstore: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("platformstore: commit: %w", err)
	}
	return nil
}

// ---- the shared bot row ---------------------------------------------------------------------------

const sharedBotColumns = `bot_account_id, bot_name, chat_url, set_at, set_by, last_check_at, last_check_result,
	webhook_set_at, webhook_set_by, webhook_secret_pending_sealed IS NOT NULL`

func scanSharedBot(row interface{ Scan(...any) error }) (domain.SharedZaloBot, error) {
	var (
		b                      domain.SharedZaloBot
		checkAt, webhookAt     sql.NullTime
		checkResult, webhookBy sql.NullString
	)
	if err := row.Scan(&b.BotAccountID, &b.BotName, &b.ChatURL, &b.SetAt, &b.SetBy, &checkAt, &checkResult,
		&webhookAt, &webhookBy, &b.HasPendingWebhook); err != nil {
		return domain.SharedZaloBot{}, err
	}
	b.LastCheckAt, b.LastCheckResult = checkAt.Time, checkResult.String
	b.WebhookSetAt, b.WebhookSetBy = webhookAt.Time, webhookBy.String
	return b, nil
}

// SharedBot reads the row as metadata. found=false: never configured.
func (s *Store) SharedBot(ctx context.Context) (domain.SharedZaloBot, bool, error) {
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). One row by construction
	// (bot_ref = 'shared'); metadata only, never a sealed column.
	row := s.db.QueryRowContext(ctx, `SELECT `+sharedBotColumns+` FROM zalo_bot_shared WHERE bot_ref = $1`,
		domain.SharedZaloBotRef)
	b, err := scanSharedBot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SharedZaloBot{}, false, nil
	}
	if err != nil {
		return domain.SharedZaloBot{}, false, fmt.Errorf("platformstore: read zalo_bot_shared: %w", err)
	}
	return b, true, nil
}

// SharedBotToken returns the SEALED token — the input of PlatformEnvelope.OpenPlatform and nothing
// else. found=false: never configured.
func (s *Store) SharedBotToken(ctx context.Context) ([]byte, bool, error) {
	var sealed []byte
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). One row, one sealed
	// column, handed to the opener and never rendered.
	err := s.db.QueryRowContext(ctx, `SELECT token_sealed FROM zalo_bot_shared WHERE bot_ref = $1`,
		domain.SharedZaloBotRef).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("platformstore: read the sealed token: %w", err)
	}
	return sealed, true, nil
}

// PendingWebhookSecretRead returns the sealed pending webhook secret OUTSIDE any transaction — read
// before one opens, so it can be opened without holding the lock; the transaction then checks the slot
// did not move (Tx.PendingWebhookSecret). nil when there is none, or no row.
func (s *Store) PendingWebhookSecretRead(ctx context.Context) ([]byte, error) {
	var sealed []byte
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). One sealed column.
	err := s.db.QueryRowContext(ctx, `SELECT webhook_secret_pending_sealed FROM zalo_bot_shared WHERE bot_ref = $1`,
		domain.SharedZaloBotRef).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("platformstore: read the pending webhook secret: %w", err)
	}
	return sealed, nil
}

// WebhookSecretsSealed returns BOTH sealed webhook secrets — the one in force and the pending one — for
// the webhook's authentication (ADR 0074 #5). During a switch Zalo may sign with either (0018
// PENDING-THEN-PROMOTE). nil for a slot that is empty; both nil when no row or no webhook was ever set.
func (s *Store) WebhookSecretsSealed(ctx context.Context) (current, pending []byte, err error) {
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). Two sealed columns, handed
	// to the opener and never rendered.
	err = s.db.QueryRowContext(ctx, `SELECT webhook_secret_sealed, webhook_secret_pending_sealed
		FROM zalo_bot_shared WHERE bot_ref = $1`, domain.SharedZaloBotRef).Scan(&current, &pending)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("platformstore: read the webhook secrets: %w", err)
	}
	return current, pending, nil
}

// sharedBotLockKey serialises every operator write on the shared bot, including the FIRST one, when
// there is no row yet for FOR UPDATE to lock. FNV-1a names a lock; it protects nothing.
func sharedBotLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("vigov:comms:zalo-bot-shared"))
	return int64(h.Sum64())
}

// LockSharedBot takes the transaction's lock on the shared bot and reads the row under it: the sealed
// token as stored and the metadata. found=false: no row yet.
func (t *Tx) LockSharedBot(ctx context.Context) (domain.SharedZaloBot, []byte, bool, error) {
	// @cross-tenant: an advisory lock names the one shared bot; it reads no table (ADR 0058 §1 shape).
	if _, err := t.tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, sharedBotLockKey()); err != nil {
		return domain.SharedZaloBot{}, nil, false, fmt.Errorf("platformstore: lock the shared bot: %w", err)
	}
	var sealed []byte
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row, locked.
	row := t.tx.QueryRowContext(ctx, `SELECT token_sealed, `+sharedBotColumns+`
		FROM zalo_bot_shared WHERE bot_ref = $1 FOR UPDATE`, domain.SharedZaloBotRef)
	var (
		b                      domain.SharedZaloBot
		checkAt, webhookAt     sql.NullTime
		checkResult, webhookBy sql.NullString
	)
	err := row.Scan(&sealed, &b.BotAccountID, &b.BotName, &b.ChatURL, &b.SetAt, &b.SetBy, &checkAt, &checkResult,
		&webhookAt, &webhookBy, &b.HasPendingWebhook)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SharedZaloBot{}, nil, false, nil
	}
	if err != nil {
		return domain.SharedZaloBot{}, nil, false, fmt.Errorf("platformstore: read zalo_bot_shared for update: %w", err)
	}
	b.LastCheckAt, b.LastCheckResult = checkAt.Time, checkResult.String
	b.WebhookSetAt, b.WebhookSetBy = webhookAt.Time, webhookBy.String
	return b, sealed, true, nil
}

// SavedToken is what SaveToken wrote.
type SavedToken struct {
	BotAccountID, BotName, ChatURL, SetBy string
	Sealed                                []byte
}

// upsertToken replaces the one row in place. set_at and the last check are the DATABASE clock of this
// transaction: the 0018 guard requires a new set_at with a new token, and the account-change trigger
// compares links' linked_at with it.
const upsertToken = `INSERT INTO zalo_bot_shared
	(bot_ref, token_sealed, bot_account_id, bot_name, chat_url, set_at, set_by, last_check_at, last_check_result)
	VALUES ($1, $2, $3, $4, $5, now(), $6, now(), $7)
	ON CONFLICT (bot_ref) DO UPDATE SET
		token_sealed = EXCLUDED.token_sealed, bot_account_id = EXCLUDED.bot_account_id,
		bot_name = EXCLUDED.bot_name, chat_url = EXCLUDED.chat_url,
		set_at = EXCLUDED.set_at, set_by = EXCLUDED.set_by,
		last_check_at = EXCLUDED.last_check_at, last_check_result = EXCLUDED.last_check_result,
		updated_at = now()
	RETURNING set_at`

// SaveToken writes the token, its account and display fields, and records the check as (now, OK) — the
// getMe the use case ran before was a check of this token (zalo_bot_operator.proto).
func (t *Tx) SaveToken(ctx context.Context, in SavedToken) (time.Time, error) {
	t.wrote = true
	var setAt time.Time
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row.
	err := t.tx.QueryRowContext(ctx, upsertToken, domain.SharedZaloBotRef, in.Sealed, in.BotAccountID,
		in.BotName, in.ChatURL, in.SetBy, domain.ZaloCallOK).Scan(&setAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("platformstore: save the shared bot token: %w", err)
	}
	return setAt, nil
}

// RecordCheck records (now, result) as the last check — only if the stored token is still the one the
// check used (compare-and-swap on the sealed bytes), else ErrChanged.
func (t *Tx) RecordCheck(ctx context.Context, checkedSealed []byte, result string) (time.Time, error) {
	t.wrote = true
	var at time.Time
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row.
	err := t.tx.QueryRowContext(ctx, `UPDATE zalo_bot_shared SET last_check_at = now(), last_check_result = $2,
		updated_at = now() WHERE bot_ref = $1 AND token_sealed = $3 RETURNING last_check_at`,
		domain.SharedZaloBotRef, result, checkedSealed).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrChanged
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("platformstore: record the check: %w", err)
	}
	return at, nil
}

// SetPendingWebhookSecret stores a new secret_token as PENDING beside the current one.
func (t *Tx) SetPendingWebhookSecret(ctx context.Context, sealed []byte) (time.Time, error) {
	t.wrote = true
	var at time.Time
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row.
	err := t.tx.QueryRowContext(ctx, `UPDATE zalo_bot_shared SET webhook_secret_pending_sealed = $2,
		webhook_secret_pending_at = now(), updated_at = now() WHERE bot_ref = $1
		RETURNING webhook_secret_pending_at`, domain.SharedZaloBotRef, sealed).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNotConfigured
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("platformstore: store the pending webhook secret: %w", err)
	}
	return at, nil
}

// PendingWebhookSecret returns the sealed pending secret under the lock, nil when there is none.
func (t *Tx) PendingWebhookSecret(ctx context.Context) ([]byte, error) {
	var sealed []byte
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row.
	err := t.tx.QueryRowContext(ctx, `SELECT webhook_secret_pending_sealed FROM zalo_bot_shared WHERE bot_ref = $1`,
		domain.SharedZaloBotRef).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("platformstore: read the pending webhook secret: %w", err)
	}
	return sealed, nil
}

// PromotePendingWebhookSecret makes THIS pending secret current — only if the pending column still
// holds exactly these bytes, else ErrChanged — and retires every other secret: the old current is
// overwritten and the pending slot cleared (0018 holds one pending slot).
func (t *Tx) PromotePendingWebhookSecret(ctx context.Context, pendingSealed []byte, by string) (time.Time, error) {
	t.wrote = true
	var at time.Time
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row.
	err := t.tx.QueryRowContext(ctx, `UPDATE zalo_bot_shared SET
		webhook_secret_sealed = webhook_secret_pending_sealed, webhook_set_at = now(), webhook_set_by = $3,
		webhook_secret_pending_sealed = NULL, webhook_secret_pending_at = NULL, updated_at = now()
		WHERE bot_ref = $1 AND webhook_secret_pending_sealed = $2 RETURNING webhook_set_at`,
		domain.SharedZaloBotRef, pendingSealed, by).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrChanged
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("platformstore: promote the webhook secret: %w", err)
	}
	return at, nil
}

// ClearPendingWebhookSecret drops THIS pending secret (Zalo definitely refused it). A pending slot that
// no longer holds these bytes is left alone and reported as ErrChanged.
func (t *Tx) ClearPendingWebhookSecret(ctx context.Context, pendingSealed []byte) error {
	t.wrote = true
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). The one row.
	res, err := t.tx.ExecContext(ctx, `UPDATE zalo_bot_shared SET webhook_secret_pending_sealed = NULL,
		webhook_secret_pending_at = NULL, updated_at = now()
		WHERE bot_ref = $1 AND webhook_secret_pending_sealed = $2`, domain.SharedZaloBotRef, pendingSealed)
	if err != nil {
		return fmt.Errorf("platformstore: clear the pending webhook secret: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("platformstore: clear the pending webhook secret: %w", err)
	} else if n == 0 {
		return ErrChanged
	}
	return nil
}

// ---- the two statements across communes -----------------------------------------------------------

// CountLiveLinks counts every live link of the shared bot, all communes.
func (t *Tx) CountLiveLinks(ctx context.Context) (int, error) {
	var n int
	// @cross-tenant: ADR 0074 frontmatter (owner 05/10/2026) — the confirmation dialog of a bot-account
	// change shows how many staff of ALL communes must pair again. One number, no row, no person.
	err := t.tx.QueryRowContext(ctx, `SELECT count(*) FROM zalo_link WHERE bot_ref = $1 AND unlinked_at IS NULL`,
		domain.SharedZaloBotRef).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("platformstore: count live links: %w", err)
	}
	return n, nil
}

// endAllLiveLinks ends every live link of the shared bot and writes, IN THE SAME STATEMENT, one entry
// per ended link into THAT link's commune audit_log (tenant_id is each ended row's own), attributed to
// the operator with actor_kind 'operator' (core/audit.KindOperator: withheld from the commune's own
// screen). Returns the per-commune counts.
const endAllLiveLinks = `WITH ended AS (
		UPDATE zalo_link SET unlinked_at = now(), unlinked_by = $2, unlink_reason = $3, updated_at = now()
		WHERE bot_ref = $1 AND unlinked_at IS NULL
		RETURNING tenant_id, staff_code
	), trail AS (
		INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
		SELECT tenant_id, $2, $4, $5, $6, staff_code, now(), $7::jsonb FROM ended
		RETURNING tenant_id
	)
	SELECT tenant_id, count(*) FROM trail GROUP BY tenant_id ORDER BY tenant_id`

// EndAllLiveLinks ends every live link of the shared bot, across communes, with unlinked_by = the
// operator's code and the fixed reason. The per-commune counts are returned for the platform trail.
func (t *Tx) EndAllLiveLinks(ctx context.Context, op Operator) (map[string]int, int, error) {
	t.wrote = true
	delta, err := json.Marshal(map[string]any{
		"ly_do":   domain.ZaloLinkEndedByBotChange,
		"nguon":   "doi_tai_khoan_zalo_bot",
		"bot_ref": domain.SharedZaloBotRef,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("platformstore: encode the link-end delta: %w", err)
	}
	// @cross-tenant: ADR 0074 frontmatter (owner 05/10/2026) — a different bot account ends EVERY live
	// staff link of the shared bot, every commune, in the same transaction as the token, each ended link
	// audited in its own commune (tenant_id of the row it ends), never in another's.
	rows, err := t.tx.QueryContext(ctx, endAllLiveLinks, domain.SharedZaloBotRef, op.Code,
		domain.ZaloLinkEndedByBotChange, audit.KindOperator, op.IP, domain.ActionEndZaloLink, delta)
	if err != nil {
		return nil, 0, fmt.Errorf("platformstore: end live links: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	total := 0
	for rows.Next() {
		var tid string
		var n int
		if err := rows.Scan(&tid, &n); err != nil {
			return nil, 0, fmt.Errorf("platformstore: end live links: %w", err)
		}
		out[tid] = n
		total += n
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("platformstore: end live links: %w", err)
	}
	return out, total, nil
}

// communeStats — a commune appears when it has a settings row or a live link of the shared bot.
const communeStats = `SELECT tenant_id, bool_or(enabled), sum(links)::bigint FROM (
		SELECT tenant_id, is_enabled AS enabled, 0::bigint AS links FROM zalo_channel_setting
		UNION ALL
		SELECT tenant_id, false, count(*) FROM zalo_link
		WHERE bot_ref = $1 AND unlinked_at IS NULL GROUP BY tenant_id
	) s GROUP BY tenant_id ORDER BY tenant_id`

// CommuneStats reads every commune's switch and live-link count. Inside a Tx because the read is
// audited in the same transaction (rule 6, invariant 7).
func (t *Tx) CommuneStats(ctx context.Context) ([]domain.ZaloBotCommuneStat, error) {
	// @cross-tenant: GET zalo-bots/shared/communes (ADR 0074 §"Tài nguyên URL", user 05/10/2026) — the
	// uptake of the ONE shared bot across every commune. Counts and a switch, never a person; audited.
	rows, err := t.tx.QueryContext(ctx, communeStats, domain.SharedZaloBotRef)
	if err != nil {
		return nil, fmt.Errorf("platformstore: commune stats: %w", err)
	}
	defer rows.Close()
	var out []domain.ZaloBotCommuneStat
	for rows.Next() {
		var s domain.ZaloBotCommuneStat
		var n int64
		if err := rows.Scan(&s.TenantID, &s.ChannelEnabled, &n); err != nil {
			return nil, fmt.Errorf("platformstore: commune stats: %w", err)
		}
		s.LinkedStaffCount = int(n)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platformstore: commune stats: %w", err)
	}
	return out, nil
}

// ---- the platform operator trail ------------------------------------------------------------------

// OperatorAudit is one platform_operator_audit_log entry. Before/After are the significant fields,
// masked — NEVER a sealed column, a token, a secret or a chat_id.
type OperatorAudit struct {
	Operator Operator
	Action   string
	Target   string
	Before   any
	After    any
}

// AppendOperatorAudit writes one entry in THIS transaction.
func (t *Tx) AppendOperatorAudit(ctx context.Context, e OperatorAudit) error {
	if err := appendOperatorAudit(ctx, t.tx, e); err != nil {
		return err
	}
	t.audited = true
	return nil
}

func appendOperatorAudit(ctx context.Context, tx *sql.Tx, e OperatorAudit) error {
	if !domain.ValidOperatorCode(e.Operator.Code) || e.Operator.IP == "" {
		return ErrNoOperator
	}
	before, err := jsonOrNull(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNull(e.After)
	if err != nil {
		return err
	}
	// @cross-tenant: platform-scope trail, no commune by design (0018 platform_operator_audit_log).
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_operator_audit_log (actor_code, ip, action, target, before, after)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb)`, e.Operator.Code, e.Operator.IP, e.Action, e.Target, before, after)
	if err != nil {
		return fmt.Errorf("platformstore: append the operator trail: %w", err)
	}
	return nil
}

func jsonOrNull(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("platformstore: encode the trail delta: %w", err)
	}
	return string(b), nil
}

// ---- crypto.PlatformDEKStore -----------------------------------------------------------------------

// GetPlatformDEK returns the wrapped platform DEK, or crypto.ErrPlatformDEKNotFound.
func (s *Store) GetPlatformDEK(ctx context.Context) (crypto.WrappedDEK, error) {
	var w crypto.WrappedDEK
	// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). One row, scope 'platform'.
	err := s.db.QueryRowContext(ctx, `SELECT kek_id, wrapped FROM platform_data_encryption_key WHERE scope = $1`,
		crypto.PlatformScope).Scan(&w.KEKID, &w.Wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return crypto.WrappedDEK{}, crypto.ErrPlatformDEKNotFound
	}
	if err != nil {
		return crypto.WrappedDEK{}, fmt.Errorf("platformstore: read the platform DEK: %w", err)
	}
	return w, nil
}

// CreatePlatformDEK inserts the first platform DEK, or reports crypto.ErrPlatformDEKExists. Never
// overwrites. Audited in its own transaction, attributed to the operator in ctx (WithOperator) — the
// person whose act needed the first seal. No operator → refused before anything is written.
func (s *Store) CreatePlatformDEK(ctx context.Context, dek crypto.WrappedDEK) error {
	op, ok := operatorFrom(ctx)
	if !ok {
		return ErrNoOperator
	}
	var inserted int64
	err := s.InTx(ctx, func(t *Tx) error {
		// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). One row.
		res, err := t.tx.ExecContext(ctx, `INSERT INTO platform_data_encryption_key (scope, kek_id, wrapped)
			VALUES ($1, $2, $3) ON CONFLICT (scope) DO NOTHING`, crypto.PlatformScope, dek.KEKID, dek.Wrapped)
		if err != nil {
			return fmt.Errorf("platformstore: insert the platform DEK: %w", err)
		}
		if inserted, err = res.RowsAffected(); err != nil {
			return fmt.Errorf("platformstore: insert the platform DEK: %w", err)
		}
		if inserted == 0 {
			return nil
		}
		// The KEK id only: 8 hex characters derived by HMAC, which say nothing about any key.
		t.wrote = true
		return t.AppendOperatorAudit(ctx, OperatorAudit{Operator: op, Action: domain.ActionPlatformDataKeyCreated,
			Target: domain.PlatformDataKeyTarget, After: map[string]string{"kek_id": dek.KEKID}})
	})
	if err != nil {
		return err
	}
	if inserted == 0 {
		return crypto.ErrPlatformDEKExists
	}
	return nil
}

// ReplacePlatformDEK swaps oldDEK for newDEK only if the row still equals oldDEK, else
// crypto.ErrPlatformDEKChanged. Audited like CreatePlatformDEK.
func (s *Store) ReplacePlatformDEK(ctx context.Context, oldDEK, newDEK crypto.WrappedDEK) error {
	if oldDEK.KEKID == newDEK.KEKID && bytes.Equal(oldDEK.Wrapped, newDEK.Wrapped) {
		return nil
	}
	op, ok := operatorFrom(ctx)
	if !ok {
		return ErrNoOperator
	}
	return s.InTx(ctx, func(t *Tx) error {
		// @cross-tenant: platform-scope table, no commune by design (ADR 0074 #4). Compare-and-swap.
		res, err := t.tx.ExecContext(ctx, `UPDATE platform_data_encryption_key SET kek_id = $4, wrapped = $5,
			updated_at = now() WHERE scope = $1 AND kek_id = $2 AND wrapped = $3`,
			crypto.PlatformScope, oldDEK.KEKID, oldDEK.Wrapped, newDEK.KEKID, newDEK.Wrapped)
		if err != nil {
			return fmt.Errorf("platformstore: re-wrap the platform DEK: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("platformstore: re-wrap the platform DEK: %w", err)
		}
		if n == 0 {
			return crypto.ErrPlatformDEKChanged
		}
		t.wrote = true
		return t.AppendOperatorAudit(ctx, OperatorAudit{Operator: op, Action: domain.ActionPlatformDataKeyRewrapped,
			Target: domain.PlatformDataKeyTarget, Before: map[string]string{"kek_id": oldDEK.KEKID},
			After: map[string]string{"kek_id": newDEK.KEKID}})
	})
}
