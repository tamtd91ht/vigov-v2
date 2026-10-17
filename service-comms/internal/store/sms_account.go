package store

// A COMMUNE'S SMS ACCOUNTS — `sms_account`, `sms_template` (migration 0032; ADR 0099 #2). SQL, and nothing
// else.
//
// EVERY STATEMENT IS ONE COMMUNE'S: $1 is the commune, from the context or the transaction (rule 1,
// invariants 4 and 5). Both unique keys on sms_account (one live label, one default) are composite with
// tenant_id; nothing here reads another commune's row.
//
// THE SEALED COLUMNS (api_key_sealed, secret_key_sealed) are read only by Sealed / LockLive — the methods
// that hand them to the use case — and are never put in an error string. ListLive / Live — what the
// screen reads — cannot select them.
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
	// ErrSmsLabelTaken — another LIVE account of this commune already carries that label, case-insensitively
	// (0032 sms_account_live_label).
	ErrSmsLabelTaken = errors.New("sms_account: label already used by a live account of the commune")
	// ErrSmsAccountChanged — the row is gone, retired, or moved underneath this write (a concurrent save, or
	// a concurrent default change hitting sms_account_one_default_per_commune).
	ErrSmsAccountChanged = errors.New("sms_account: row changed underneath this write")
)

// ---- sms_account --------------------------------------------------------------------------------------

// SmsAccountStore is the only scoped path to sms_account.
type SmsAccountStore struct {
	db *store.DB
}

func NewSmsAccountStore(db *store.DB) *SmsAccountStore { return &SmsAccountStore{db: db} }

// SmsAccountSecrets is a live row with its sealed bytes. Neither is ever empty (NOT NULL).
type SmsAccountSecrets struct {
	Account         domain.SmsAccount
	APIKeySealed    []byte
	SecretKeySealed []byte
}

// smsAccountColumns IS READ BY POSITION in scanSmsAccount. last_balance is read as text: the exact
// decimal, never a float.
const smsAccountColumns = `id, provider, label, brandname, is_default, last_check_at, last_check_result,
	last_balance::text, set_at, set_by, created_at`

const smsAccountSealedColumns = `, api_key_sealed, secret_key_sealed`

func scanSmsAccount(scan func(...any) error, extra ...any) (domain.SmsAccount, error) {
	var (
		a        domain.SmsAccount
		checkAt  sql.NullTime
		checkRes sql.NullString
		balance  sql.NullString
	)
	dest := append([]any{&a.ID, &a.Provider, &a.Label, &a.Brandname, &a.IsDefault, &checkAt, &checkRes, &balance,
		&a.SetAt, &a.SetBy, &a.CreatedAt}, extra...)
	if err := scan(dest...); err != nil {
		return domain.SmsAccount{}, err
	}
	a.LastCheckAt, a.LastCheckResult, a.LastBalance = checkAt.Time, checkRes.String, balance.String
	return a, nil
}

// ListLive reads the commune's live accounts — metadata only — the default first, then oldest first.
func (s *SmsAccountStore) ListLive(ctx context.Context) ([]domain.SmsAccount, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, smsAccountColumns, "sms_account",
		"AND retired_at IS NULL ORDER BY is_default DESC, created_at, id")
	if err != nil {
		return nil, fmt.Errorf("sms_account: list: %w", err)
	}
	defer rows.Close()
	var out []domain.SmsAccount
	for rows.Next() {
		a, err := scanSmsAccount(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("sms_account: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sms_account: list: %w", err)
	}
	return out, nil
}

// Live reads one live account of this commune — metadata only. found=false: no such live row here.
func (s *SmsAccountStore) Live(ctx context.Context, id string) (domain.SmsAccount, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, smsAccountColumns, "sms_account", "AND id = $2 AND retired_at IS NULL", id)
	if err != nil {
		return domain.SmsAccount{}, false, fmt.Errorf("sms_account: read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.SmsAccount{}, false, rowsErr(rows, "sms_account: read")
	}
	a, err := scanSmsAccount(rows.Scan)
	if err != nil {
		return domain.SmsAccount{}, false, fmt.Errorf("sms_account: scan: %w", err)
	}
	return a, true, rows.Err()
}

// Sealed reads one live account WITH its sealed keys, outside any transaction — for an eSMS call.
// found=false: no such live row here.
func (s *SmsAccountStore) Sealed(ctx context.Context, id string) (SmsAccountSecrets, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, smsAccountColumns+smsAccountSealedColumns, "sms_account",
		"AND id = $2 AND retired_at IS NULL", id)
	if err != nil {
		return SmsAccountSecrets{}, false, fmt.Errorf("sms_account: read sealed: %w", err)
	}
	return firstSealedSmsAccount(rows)
}

// LockLive is Sealed inside the transaction, the row held until it ends — every write below is
// read-decide-write.
func (s *SmsAccountStore) LockLive(ctx context.Context, tx *store.ScopedTx, id string) (SmsAccountSecrets, bool, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, smsAccountColumns+smsAccountSealedColumns, "sms_account",
		"AND id = $2 AND retired_at IS NULL FOR UPDATE", id)
	if err != nil {
		return SmsAccountSecrets{}, false, fmt.Errorf("sms_account: lock: %w", err)
	}
	return firstSealedSmsAccount(rows)
}

func firstSealedSmsAccount(rows *sql.Rows) (SmsAccountSecrets, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return SmsAccountSecrets{}, false, rowsErr(rows, "sms_account: read sealed")
	}
	var out SmsAccountSecrets
	a, err := scanSmsAccount(rows.Scan, &out.APIKeySealed, &out.SecretKeySealed)
	if err != nil {
		return SmsAccountSecrets{}, false, fmt.Errorf("sms_account: scan sealed: %w", err)
	}
	out.Account = a
	return out, true, rows.Err()
}

// nullBalance is "" → NULL (0032 sms_account_balance_from_success: no balance without a successful check).
func nullBalance(b string) any {
	if b == "" {
		return nil
	}
	return b
}

// insertSmsAccount — the last check is the one that just answered for these keys (the use case refuses
// to save keys eSMS did not accept).
const insertSmsAccount = `INSERT INTO sms_account (tenant_id, id, provider, label, api_key_sealed, secret_key_sealed, brandname,
		is_default, last_check_at, last_check_result, last_balance, set_at, set_by, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $12, $12)`

// InsertLive adds a live account in the transaction's commune. a.IsDefault true needs ClearDefault first
// in the same transaction (the partial unique key is checked immediately). ErrSmsLabelTaken on the live
// label key; ErrSmsAccountChanged on the default key (a concurrent default change).
func (s *SmsAccountStore) InsertLive(ctx context.Context, tx *store.ScopedTx, a domain.SmsAccount, apiSealed, secretSealed []byte) error {
	var checkAt, checkRes any
	if !a.LastCheckAt.IsZero() {
		checkAt, checkRes = a.LastCheckAt.UTC(), a.LastCheckResult
	}
	if _, err := tx.Exec(ctx, insertSmsAccount, string(tx.TenantID()), a.ID, a.Provider, a.Label, apiSealed, secretSealed,
		a.Brandname, a.IsDefault, checkAt, checkRes, nullBalance(a.LastBalance), a.SetAt.UTC(), a.SetBy); err != nil {
		return namedSmsAccountKey(fmt.Errorf("sms_account: insert: %w", err))
	}
	return nil
}

// namedSmsAccountKey turns a violation of one of sms_account's two unique keys into its sentinel; any
// other error is returned as is.
func namedSmsAccountKey(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		switch pg.ConstraintName {
		case "sms_account_live_label":
			return ErrSmsLabelTaken
		case "sms_account_one_default_per_commune":
			return ErrSmsAccountChanged
		}
	}
	return err
}

const updateSmsAccountLabel = `UPDATE sms_account SET label = $3, updated_at = $4
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// UpdateLabel renames a live account. ErrSmsLabelTaken when another live account carries the name.
func (s *SmsAccountStore) UpdateLabel(ctx context.Context, tx *store.ScopedTx, id, label string, at time.Time) error {
	return namedSmsAccountKey(execOne(ctx, tx, updateSmsAccountLabel, "sms_account: update label", ErrSmsAccountChanged,
		string(tx.TenantID()), id, label, at.UTC()))
}

// updateSmsAccountCredentials — keys and brandname together, with the new set_at / set_by 0032's guard
// demands, and the check that just answered for them.
const updateSmsAccountCredentials = `UPDATE sms_account SET label = $3, brandname = $4, api_key_sealed = $5,
		secret_key_sealed = $6, set_at = $7, set_by = $8, last_check_at = $7, last_check_result = $9,
		last_balance = $10, updated_at = $7
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// UpdateCredentials writes new keys and / or brandname onto a live account (both sealed columns are
// written — the caller passes the existing bytes for a key it keeps), its label, who set them and the
// check that verified them. at must differ from the row's set_at (0032's guard).
func (s *SmsAccountStore) UpdateCredentials(ctx context.Context, tx *store.ScopedTx, id, label, brandname string,
	apiSealed, secretSealed []byte, by, checkResult, balance string, at time.Time) error {

	return namedSmsAccountKey(execOne(ctx, tx, updateSmsAccountCredentials, "sms_account: update credentials",
		ErrSmsAccountChanged, string(tx.TenantID()), id, label, brandname, apiSealed, secretSealed, at.UTC(), by,
		checkResult, nullBalance(balance)))
}

const clearSmsAccountDefault = `UPDATE sms_account SET is_default = false, updated_at = $2
	WHERE tenant_id = $1 AND is_default`

// ClearDefault takes the default mark off whichever account of the commune carries it (none is fine).
// Retired rows never carry it (0032 sms_account_retired_not_default), so the guard's freeze is not hit.
func (s *SmsAccountStore) ClearDefault(ctx context.Context, tx *store.ScopedTx, at time.Time) error {
	if _, err := tx.Exec(ctx, clearSmsAccountDefault, string(tx.TenantID()), at.UTC()); err != nil {
		return fmt.Errorf("sms_account: clear default: %w", err)
	}
	return nil
}

const setSmsAccountDefault = `UPDATE sms_account SET is_default = true, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// SetDefault makes a live account the commune's default: the old default is cleared FIRST, in the same
// transaction (0032: the partial unique key is checked immediately).
func (s *SmsAccountStore) SetDefault(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	if err := s.ClearDefault(ctx, tx, at); err != nil {
		return err
	}
	return namedSmsAccountKey(execOne(ctx, tx, setSmsAccountDefault, "sms_account: set default", ErrSmsAccountChanged,
		string(tx.TenantID()), id, at.UTC()))
}

const unsetSmsAccountDefault = `UPDATE sms_account SET is_default = false, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// UnsetDefault takes the default mark off one live account; the commune then has no default.
func (s *SmsAccountStore) UnsetDefault(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	return execOne(ctx, tx, unsetSmsAccountDefault, "sms_account: unset default", ErrSmsAccountChanged,
		string(tx.TenantID()), id, at.UTC())
}

const recordSmsAccountCheck = `UPDATE sms_account SET last_check_at = $3, last_check_result = $4, last_balance = $5,
		updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// RecordCheck stores the last "Kiểm tra kết nối" — a SmsCheck* class, never eSMS's text — and the balance
// it returned ("" for anything but thanh-cong, so a failed check never leaves an older balance standing).
func (s *SmsAccountStore) RecordCheck(ctx context.Context, tx *store.ScopedTx, id, result, balance string, at time.Time) error {
	return execOne(ctx, tx, recordSmsAccountCheck, "sms_account: record check", ErrSmsAccountChanged,
		string(tx.TenantID()), id, at.UTC(), result, nullBalance(balance))
}

// retireSmsAccount — the soft-delete trio, and is_default cleared in the SAME statement (0032
// sms_account_retired_not_default).
const retireSmsAccount = `UPDATE sms_account SET retired_at = $3, retired_by = $4, retire_reason = $5, is_default = false,
		updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND retired_at IS NULL`

// Retire takes a live account out of service ("ngừng dùng"). The row stays, frozen, as the record of an
// account campaigns went out through (rule 7). No other account becomes the default.
func (s *SmsAccountStore) Retire(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	return execOne(ctx, tx, retireSmsAccount, "sms_account: retire", ErrSmsAccountChanged,
		string(tx.TenantID()), id, at.UTC(), by, reason)
}

// ---- sms_template -------------------------------------------------------------------------------------

// SmsTemplateStore is the only scoped path to sms_template.
type SmsTemplateStore struct {
	db *store.DB
}

func NewSmsTemplateStore(db *store.DB) *SmsTemplateStore { return &SmsTemplateStore{db: db} }

// smsTemplateColumns IS READ BY POSITION in scanSmsTemplates.
const smsTemplateColumns = `id, account_id, provider_template_id, network_id, content, params, synced_at, vanished_at`

// storedSmsParam is 0032's `params` element: {name, max_length}.
type storedSmsParam struct {
	Name      string `json:"name"`
	MaxLength int    `json:"max_length"`
}

func encodeSmsParams(ps []domain.SmsTemplateParam) ([]byte, error) {
	out := make([]storedSmsParam, 0, len(ps))
	for _, p := range ps {
		out = append(out, storedSmsParam{Name: p.Name, MaxLength: p.MaxLen})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("sms_template: encode params: %w", err)
	}
	return b, nil
}

func decodeSmsParams(b []byte) ([]domain.SmsTemplateParam, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var in []storedSmsParam
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("sms_template: decode params: %w", err)
	}
	out := make([]domain.SmsTemplateParam, 0, len(in))
	for _, p := range in {
		out = append(out, domain.SmsTemplateParam{Name: p.Name, MaxLen: p.MaxLength})
	}
	return out, nil
}

func scanSmsTemplates(rows *sql.Rows, what string) ([]domain.SmsTemplate, error) {
	defer rows.Close()
	var out []domain.SmsTemplate
	for rows.Next() {
		var (
			t        domain.SmsTemplate
			network  int64
			params   []byte
			vanished sql.NullTime
		)
		if err := rows.Scan(&t.ID, &t.AccountID, &t.ProviderTemplateID, &network, &t.Content, &params, &t.SyncedAt,
			&vanished); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", what, err)
		}
		ps, err := decodeSmsParams(params)
		if err != nil {
			return nil, err
		}
		t.NetworkID, t.Params, t.VanishedAt = int(network), ps, vanished.Time
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return out, nil
}

// ListCurrentByAccount reads the templates eSMS still lists for one account of this commune (vanished
// rows excluded — 0032: a vanished template is not offered), by content then network.
func (s *SmsTemplateStore) ListCurrentByAccount(ctx context.Context, accountID string) ([]domain.SmsTemplate, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, smsTemplateColumns, "sms_template",
		"AND account_id = $2 AND vanished_at IS NULL ORDER BY content, network_id, provider_template_id", accountID)
	if err != nil {
		return nil, fmt.Errorf("sms_template: list: %w", err)
	}
	return scanSmsTemplates(rows, "sms_template: list")
}

// LockByAccount reads EVERY template row of one account, vanished included, held until the transaction
// ends — the sync's read-decide-write.
func (s *SmsTemplateStore) LockByAccount(ctx context.Context, tx *store.ScopedTx, accountID string) ([]domain.SmsTemplate, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, smsTemplateColumns, "sms_template",
		"AND account_id = $2 ORDER BY provider_template_id, network_id FOR UPDATE", accountID)
	if err != nil {
		return nil, fmt.Errorf("sms_template: lock: %w", err)
	}
	return scanSmsTemplates(rows, "sms_template: lock")
}

// upsertSmsTemplate — eSMS's columns rewritten; a row that reappears is no longer vanished. The conflict
// target is 0032's per-account unique key.
const upsertSmsTemplate = `INSERT INTO sms_template (tenant_id, id, account_id, provider_template_id, network_id, content,
		params, synced_at, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $8)
	ON CONFLICT (tenant_id, account_id, provider_template_id, network_id) DO UPDATE SET
		content = EXCLUDED.content, params = EXCLUDED.params, synced_at = EXCLUDED.synced_at, vanished_at = NULL,
		updated_at = EXCLUDED.updated_at`

// UpsertFromSync writes what eSMS answered for one template of t.AccountID. t.ID is the id to use IF the
// row is new (ignored on an existing one). The caller validated it (domain.ValidateSmsTemplate).
func (s *SmsTemplateStore) UpsertFromSync(ctx context.Context, tx *store.ScopedTx, t domain.SmsTemplate, syncedAt time.Time) error {
	params, err := encodeSmsParams(t.Params)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, upsertSmsTemplate, string(tx.TenantID()), t.ID, t.AccountID, t.ProviderTemplateID,
		t.NetworkID, t.Content, params, syncedAt.UTC()); err != nil {
		return fmt.Errorf("sms_template: upsert: %w", err)
	}
	return nil
}

const markSmsTemplateVanished = `UPDATE sms_template SET vanished_at = $3, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND vanished_at IS NULL`

// MarkVanished records a template eSMS no longer lists (0032: the row stays; campaigns name it).
func (s *SmsTemplateStore) MarkVanished(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	return execOne(ctx, tx, markSmsTemplateVanished, "sms_template: mark vanished", ErrSmsAccountChanged,
		string(tx.TenantID()), id, at.UTC())
}
