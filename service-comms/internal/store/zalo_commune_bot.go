package store

// A COMMUNE'S OWN ZALO BOT — `zalo_commune_bot` (migration 0022; ADR 0079 Q1 #1–#5). SQL, and nothing else.
//
// EVERY STATEMENT IS ONE COMMUNE'S: $1 is the commune, from the context or the transaction (rule 1,
// invariants 4 and 5). The platform-wide unique key on live bot accounts is the DATABASE's check; nothing
// here reads another commune's row, and the error it raises is turned into one generic sentence that
// names no commune (ErrCommuneZaloBotAccountTaken).
//
// THE SEALED COLUMNS are read only by the methods that exist to hand them to the use case (Sealed /
// LockLive) and are never put in an error string. Live() — what the screen reads — cannot select them.
//
// NO audit.Write HERE: internal/app opens the transaction and writes the entry inside it.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloCommuneBotStore is the only scoped path to zalo_commune_bot.
type ZaloCommuneBotStore struct {
	db *store.DB
}

func NewZaloCommuneBotStore(db *store.DB) *ZaloCommuneBotStore { return &ZaloCommuneBotStore{db: db} }

// ErrCommuneZaloBotAccountTaken — the bot account is live somewhere else on the platform: another
// commune's own bot, or the shared bot (0022's unique key / account trigger). WHICH one is never said.
var ErrCommuneZaloBotAccountTaken = errors.New("zalo_commune_bot: the bot account is live elsewhere")

// CommuneBotSecrets is the live row with its sealed bytes. Token is never empty (NOT NULL); the two
// webhook slots are nil when empty.
type CommuneBotSecrets struct {
	Bot                 domain.CommuneZaloBot
	TokenSealed         []byte
	WebhookSecretSealed []byte
	PendingSecretSealed []byte
}

// communeBotColumns IS READ BY POSITION in scanCommuneBot.
const communeBotColumns = `id, bot_account_id, bot_name, chat_url, set_at, set_by, last_check_at, last_check_result,
	webhook_set_at, webhook_set_by, webhook_secret_pending_sealed IS NOT NULL`

const communeBotSealedColumns = `, token_sealed, webhook_secret_sealed, webhook_secret_pending_sealed`

func scanCommuneBot(scan func(...any) error, extra ...any) (domain.CommuneZaloBot, error) {
	var (
		b        domain.CommuneZaloBot
		checkAt  sql.NullTime
		checkRes sql.NullString
		hookAt   sql.NullTime
		hookBy   sql.NullString
	)
	dest := append([]any{&b.ID, &b.BotAccountID, &b.BotName, &b.ChatURL, &b.SetAt, &b.SetBy, &checkAt, &checkRes,
		&hookAt, &hookBy, &b.HasPendingWebhook}, extra...)
	if err := scan(dest...); err != nil {
		return domain.CommuneZaloBot{}, err
	}
	b.LastCheckAt, b.LastCheckResult = checkAt.Time, checkRes.String
	b.WebhookSetAt, b.WebhookSetBy = hookAt.Time, hookBy.String
	return b, nil
}

// Live reads the commune's live bot — metadata only. found=false: the commune uses the shared bot.
func (s *ZaloCommuneBotStore) Live(ctx context.Context) (domain.CommuneZaloBot, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, communeBotColumns, "zalo_commune_bot", "AND retired_at IS NULL")
	if err != nil {
		return domain.CommuneZaloBot{}, false, fmt.Errorf("zalo_commune_bot: read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.CommuneZaloBot{}, false, rowsErr(rows, "zalo_commune_bot: read")
	}
	b, err := scanCommuneBot(rows.Scan)
	if err != nil {
		return domain.CommuneZaloBot{}, false, fmt.Errorf("zalo_commune_bot: scan: %w", err)
	}
	return b, true, rows.Err()
}

// Sealed reads the live bot WITH its sealed bytes, outside any transaction — for a send, a reply, or the
// webhook's authentication. found=false: no live own bot.
func (s *ZaloCommuneBotStore) Sealed(ctx context.Context) (CommuneBotSecrets, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, communeBotColumns+communeBotSealedColumns, "zalo_commune_bot",
		"AND retired_at IS NULL")
	if err != nil {
		return CommuneBotSecrets{}, false, fmt.Errorf("zalo_commune_bot: read sealed: %w", err)
	}
	return firstSealed(rows)
}

// LockLive reads the live bot with its sealed bytes and holds it until the transaction ends — every
// write below is read-decide-write. found=false: no live own bot.
func (s *ZaloCommuneBotStore) LockLive(ctx context.Context, tx *store.ScopedTx) (CommuneBotSecrets, bool, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, communeBotColumns+communeBotSealedColumns, "zalo_commune_bot",
		"AND retired_at IS NULL FOR UPDATE")
	if err != nil {
		return CommuneBotSecrets{}, false, fmt.Errorf("zalo_commune_bot: lock: %w", err)
	}
	return firstSealed(rows)
}

func firstSealed(rows *sql.Rows) (CommuneBotSecrets, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return CommuneBotSecrets{}, false, rowsErr(rows, "zalo_commune_bot: read sealed")
	}
	var out CommuneBotSecrets
	b, err := scanCommuneBot(rows.Scan, &out.TokenSealed, &out.WebhookSecretSealed, &out.PendingSecretSealed)
	if err != nil {
		return CommuneBotSecrets{}, false, fmt.Errorf("zalo_commune_bot: scan sealed: %w", err)
	}
	out.Bot = b
	return out, true, rows.Err()
}

func rowsErr(rows *sql.Rows, what string) error {
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

// insertCommuneBot — the last check is the getMe that just answered for this token (the shared bot's
// SaveToken records the same).
const insertCommuneBot = `INSERT INTO zalo_commune_bot (tenant_id, id, bot_account_id, token_sealed, bot_name, chat_url,
		set_at, set_by, last_check_at, last_check_result, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7, 'thanh-cong', $7, $7)`

// InsertLive adopts a bot: a new live row in the transaction's commune. ErrCommuneZaloBotAccountTaken
// when the account is live elsewhere — the unique key (23505) or the shared-bot trigger (P0001, the only
// exception an INSERT can raise here: 0022's other triggers fire on UPDATE/DELETE or at COMMIT). The
// transaction is then aborted; the caller rolls it back.
func (s *ZaloCommuneBotStore) InsertLive(ctx context.Context, tx *store.ScopedTx, b domain.CommuneZaloBot,
	tokenSealed []byte) error {

	if _, err := tx.Exec(ctx, insertCommuneBot, string(tx.TenantID()), b.ID, b.BotAccountID, tokenSealed,
		b.BotName, b.ChatURL, b.SetAt.UTC(), b.SetBy); err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && (pg.Code == "23505" || pg.Code == "P0001") {
			return ErrCommuneZaloBotAccountTaken
		}
		return fmt.Errorf("zalo_commune_bot: insert: %w", err)
	}
	return nil
}

// updateCommuneBotToken — a new token for the SAME bot account: in place, with a new set_at (0022's
// guard refuses a new token without one).
const updateCommuneBotToken = `UPDATE zalo_commune_bot SET token_sealed = $3, bot_name = $4, chat_url = $5,
		set_at = $6, set_by = $7, last_check_at = $6, last_check_result = 'thanh-cong', updated_at = $6
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// UpdateToken replaces the live bot's token (same account), name and chat link.
func (s *ZaloCommuneBotStore) UpdateToken(ctx context.Context, tx *store.ScopedTx, id string, tokenSealed []byte,
	name, chatURL, by string, at time.Time) error {

	return execOne(ctx, tx, updateCommuneBotToken, "zalo_commune_bot: update token", ErrZaloRowChanged,
		string(tx.TenantID()), id, tokenSealed, name, chatURL, at.UTC(), by)
}

const updateCommuneBotMeta = `UPDATE zalo_commune_bot SET bot_name = $3, chat_url = $4, updated_at = $5
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// UpdateMeta changes the live bot's name and chat link only (no new token).
func (s *ZaloCommuneBotStore) UpdateMeta(ctx context.Context, tx *store.ScopedTx, id, name, chatURL string,
	at time.Time) error {

	return execOne(ctx, tx, updateCommuneBotMeta, "zalo_commune_bot: update", ErrZaloRowChanged,
		string(tx.TenantID()), id, name, chatURL, at.UTC())
}

const recordCommuneBotCheck = `UPDATE zalo_commune_bot SET last_check_at = $3, last_check_result = $4, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// RecordCheck stores the last "Kiểm tra kết nối" — a ZaloCall* class, never Zalo's text.
func (s *ZaloCommuneBotStore) RecordCheck(ctx context.Context, tx *store.ScopedTx, id, result string, at time.Time) error {
	return execOne(ctx, tx, recordCommuneBotCheck, "zalo_commune_bot: record check", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC(), result)
}

const setCommuneBotPending = `UPDATE zalo_commune_bot SET webhook_secret_pending_sealed = $3,
		webhook_secret_pending_at = $4, updated_at = $4
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL AND webhook_secret_pending_sealed IS NULL`

// SetPendingWebhookSecret fills the EMPTY pending slot (0018's pending-then-promote, 0022). A slot that
// filled meanwhile is ErrZaloRowChanged.
func (s *ZaloCommuneBotStore) SetPendingWebhookSecret(ctx context.Context, tx *store.ScopedTx, id string,
	sealed []byte, at time.Time) error {

	return execOne(ctx, tx, setCommuneBotPending, "zalo_commune_bot: set pending secret", ErrZaloRowChanged,
		string(tx.TenantID()), id, sealed, at.UTC())
}

const promoteCommuneBotPending = `UPDATE zalo_commune_bot SET webhook_secret_sealed = webhook_secret_pending_sealed,
		webhook_set_at = $4, webhook_set_by = $5,
		webhook_secret_pending_sealed = NULL, webhook_secret_pending_at = NULL, updated_at = $4
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL AND webhook_secret_pending_sealed = $3`

// PromotePendingWebhookSecret makes the pending secret the one in force — only if the slot still holds
// pendingSealed (ErrZaloRowChanged otherwise). by is the staff code.
func (s *ZaloCommuneBotStore) PromotePendingWebhookSecret(ctx context.Context, tx *store.ScopedTx, id string,
	pendingSealed []byte, by string, at time.Time) error {

	return execOne(ctx, tx, promoteCommuneBotPending, "zalo_commune_bot: promote secret", ErrZaloRowChanged,
		string(tx.TenantID()), id, pendingSealed, at.UTC(), by)
}

const clearCommuneBotPending = `UPDATE zalo_commune_bot SET webhook_secret_pending_sealed = NULL,
		webhook_secret_pending_at = NULL, updated_at = $4
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL AND webhook_secret_pending_sealed = $3`

// ClearPendingWebhookSecret empties the slot after a DEFINITE refusal by Zalo — only if it still holds
// pendingSealed (ErrZaloRowChanged otherwise).
func (s *ZaloCommuneBotStore) ClearPendingWebhookSecret(ctx context.Context, tx *store.ScopedTx, id string,
	pendingSealed []byte, at time.Time) error {

	return execOne(ctx, tx, clearCommuneBotPending, "zalo_commune_bot: clear pending secret", ErrZaloRowChanged,
		string(tx.TenantID()), id, pendingSealed, at.UTC())
}

// retireCommuneBot writes the soft-delete trio and clears the pending slot in the same statement — 0022
// refuses a retired row holding a pending secret the webhook might still accept.
const retireCommuneBot = `UPDATE zalo_commune_bot SET retired_at = $3, retired_by = $4, retire_reason = $5,
		webhook_secret_pending_sealed = NULL, webhook_secret_pending_at = NULL, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// Retire takes the live bot out of service (the row stays, frozen, as the record of which bot served the
// commune — rule 7). The caller ends the bot's live links in the same transaction (0022's deferred check).
func (s *ZaloCommuneBotStore) Retire(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	return execOne(ctx, tx, retireCommuneBot, "zalo_commune_bot: retire", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC(), by, reason)
}
