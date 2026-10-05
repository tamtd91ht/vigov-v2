package store

// THE COMMUNE SIDE OF THE ZALO BOT CHANNEL — `zalo_channel_setting`, `zalo_pairing_code`, `zalo_link`,
// `zalo_delivery` (migration 0018, ADR 0074). SQL, and nothing else.
//
// EVERY STATEMENT HERE IS ONE COMMUNE'S: $1 is the commune, from the context or the transaction (rule 1,
// invariants 4 and 5). The statements that must cross communes — finding a pairing code or a link by what
// a chat sent, ending a chat's link in ANOTHER commune, listing the communes with deliveries due — are in
// internal/store/crosstenant/zalo_bot.go, each with its `// @cross-tenant:`.
//
// chat_id IS PERSONAL DATA (ADR 0074). It is read here only by the method the SEND needs (LiveChatsOf),
// written only by InsertLink, and never put into an error string.
//
// NO audit.Write HERE: internal/app opens the transaction and writes the entry inside it.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloLinkStore is the only scoped path to the four commune tables of 0018.
type ZaloLinkStore struct {
	db *store.DB
}

func NewZaloLinkStore(db *store.DB) *ZaloLinkStore { return &ZaloLinkStore{db: db} }

var (
	// ErrPairingCodeTaken — the hash of a freshly drawn code is already in this commune's history (0018:
	// a hash is never issued twice in a commune). The caller draws again.
	ErrPairingCodeTaken = errors.New("zalo_pairing_code: hash already issued in this commune")
	// ErrPairingCodeNotOpen — the code addressed is used, cancelled or absent in this commune.
	ErrPairingCodeNotOpen = errors.New("zalo_pairing_code: not open")
	// ErrZaloRowChanged — a link or delivery moved underneath a guarded update.
	ErrZaloRowChanged = errors.New("zalo: row changed underneath this write")
)

// MaxZaloLinksListed bounds the commune's linked-staff list. A commune has tens of staff; this is the
// point past which the list is not one any screen can show, refused rather than truncated.
const MaxZaloLinksListed = 2000

// ---- channel settings ------------------------------------------------------------------------------

const zaloSettingColumns = `is_enabled, array_to_string(kinds, ','), to_char(quiet_start, 'HH24:MI'),
	to_char(quiet_end, 'HH24:MI'), overdue_start_after_days, overdue_repeat_every_days, updated_at, updated_by`

func scanZaloSetting(scan func(...any) error) (domain.ZaloChannelSetting, error) {
	var (
		s                   domain.ZaloChannelSetting
		kinds, qs, qe       string
		startDays, everyDay sql.NullInt64
	)
	if err := scan(&s.IsEnabled, &kinds, &qs, &qe, &startDays, &everyDay, &s.UpdatedAt, &s.UpdatedBy); err != nil {
		return domain.ZaloChannelSetting{}, err
	}
	s.Kinds = []string{}
	if kinds != "" {
		s.Kinds = strings.Split(kinds, ",")
	}
	var ok1, ok2 bool
	s.QuietStartMinute, ok1 = domain.ParseClock(qs)
	s.QuietEndMinute, ok2 = domain.ParseClock(qe)
	if !ok1 || !ok2 {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo_channel_setting: unreadable quiet window")
	}
	if startDays.Valid {
		v := int(startDays.Int64)
		s.OverdueStartAfterDays = &v
	}
	if everyDay.Valid {
		v := int(everyDay.Int64)
		s.OverdueRepeatEveryDays = &v
	}
	s.Saved = true
	return s, nil
}

// ChannelSetting reads the commune's row; no row = domain.DefaultZaloChannelSetting (Saved=false: OFF).
func (s *ZaloLinkStore) ChannelSetting(ctx context.Context) (domain.ZaloChannelSetting, error) {
	rows, err := s.db.For(ctx).Query(ctx, zaloSettingColumns, "zalo_channel_setting", "")
	if err != nil {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo_channel_setting: read: %w", err)
	}
	defer rows.Close()
	return firstZaloSetting(rows)
}

// LockChannelSetting reads the row inside the transaction, FOR UPDATE, so two saves cannot both write an
// entry whose "before" is the same state.
func (s *ZaloLinkStore) LockChannelSetting(ctx context.Context, tx *store.ScopedTx) (domain.ZaloChannelSetting, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, zaloSettingColumns, "zalo_channel_setting", "FOR UPDATE")
	if err != nil {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo_channel_setting: lock: %w", err)
	}
	defer rows.Close()
	return firstZaloSetting(rows)
}

func firstZaloSetting(rows *sql.Rows) (domain.ZaloChannelSetting, error) {
	out := domain.DefaultZaloChannelSetting()
	if rows.Next() {
		v, err := scanZaloSetting(rows.Scan)
		if err != nil {
			return domain.ZaloChannelSetting{}, fmt.Errorf("zalo_channel_setting: scan: %w", err)
		}
		out = v
	}
	if err := rows.Err(); err != nil {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo_channel_setting: rows: %w", err)
	}
	return out, nil
}

// saveZaloSetting overwrites the commune's one row in place (0018: no soft delete on this table).
const saveZaloSetting = `INSERT INTO zalo_channel_setting (tenant_id, is_enabled, kinds, quiet_start, quiet_end,
		overdue_start_after_days, overdue_repeat_every_days, created_at, updated_at, updated_by)
	VALUES ($1, $2, string_to_array($3, ','), $4::time, $5::time, $6, $7, $8, $8, $9)
	ON CONFLICT (tenant_id) DO UPDATE SET is_enabled = EXCLUDED.is_enabled, kinds = EXCLUDED.kinds,
		quiet_start = EXCLUDED.quiet_start, quiet_end = EXCLUDED.quiet_end,
		overdue_start_after_days = EXCLUDED.overdue_start_after_days,
		overdue_repeat_every_days = EXCLUDED.overdue_repeat_every_days,
		updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`

// SaveChannelSetting writes the commune's settings row. by is the saver's business code.
func (s *ZaloLinkStore) SaveChannelSetting(ctx context.Context, tx *store.ScopedTx, v domain.ZaloChannelSetting,
	by string, at time.Time) error {

	if _, err := tx.Exec(ctx, saveZaloSetting, string(tx.TenantID()), v.IsEnabled, strings.Join(v.Kinds, ","),
		domain.FormatClock(v.QuietStartMinute), domain.FormatClock(v.QuietEndMinute),
		nullableInt(v.OverdueStartAfterDays), nullableInt(v.OverdueRepeatEveryDays), at.UTC(), by); err != nil {
		return fmt.Errorf("zalo_channel_setting: save: %w", err)
	}
	return nil
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}

// ---- pairing codes ---------------------------------------------------------------------------------

// PairingCode is one locked `zalo_pairing_code` row.
type PairingCode struct {
	ID             string
	StaffCode      string
	ExpiresAt      time.Time
	FailedAttempts int
}

const cancelOpenPairingCodes = `UPDATE zalo_pairing_code SET cancelled_at = $3, updated_at = $3
	WHERE tenant_id = $1 AND staff_code = $2 AND used_at IS NULL AND cancelled_at IS NULL`

// CancelOpenPairingCodes closes every open code of one member of staff — expired or not (0018: expiry
// cannot sit in the one-open-per-staff index predicate, so issuing CANCELS the previous one).
func (s *ZaloLinkStore) CancelOpenPairingCodes(ctx context.Context, tx *store.ScopedTx, staffCode string,
	at time.Time) (int, error) {

	res, err := tx.Exec(ctx, cancelOpenPairingCodes, string(tx.TenantID()), staffCode, at.UTC())
	if err != nil {
		return 0, fmt.Errorf("zalo_pairing_code: cancel open: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("zalo_pairing_code: cancel open: %w", err)
	}
	return int(n), nil
}

const insertPairingCode = `INSERT INTO zalo_pairing_code (tenant_id, id, staff_code, code_hash, expires_at, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $6)`

// InsertPairingCode stores the HASH of a new code. ErrPairingCodeTaken: that hash was issued in this
// commune before, and the caller draws again — the INSERT runs inside a SAVEPOINT so the refusal aborts
// nothing else of the caller's transaction (its cancel of the previous code, above all).
func (s *ZaloLinkStore) InsertPairingCode(ctx context.Context, tx *store.ScopedTx, id, staffCode string,
	hash []byte, expiresAt, at time.Time) error {

	// A savepoint INSIDE this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `SAVEPOINT zalo_pairing_code_insert`); err != nil {
		return fmt.Errorf("zalo_pairing_code: savepoint: %w", err)
	}
	if _, err := tx.Exec(ctx, insertPairingCode, string(tx.TenantID()), id, staffCode, hash, expiresAt.UTC(),
		at.UTC()); err != nil {
		// Back to the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT zalo_pairing_code_insert`); rbErr != nil {
			return fmt.Errorf("zalo_pairing_code: insert: %w (rollback to savepoint: %v)", err, rbErr)
		}
		if isUniqueViolation(err) {
			return ErrPairingCodeTaken
		}
		return fmt.Errorf("zalo_pairing_code: insert: %w", err)
	}
	// Release the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT zalo_pairing_code_insert`); err != nil {
		return fmt.Errorf("zalo_pairing_code: release savepoint: %w", err)
	}
	return nil
}

// LockOpenPairingCode reads one OPEN code of this commune, FOR UPDATE — two chats sending the same code at
// once cannot both use it. ErrPairingCodeNotOpen when it is used, cancelled or absent.
func (s *ZaloLinkStore) LockOpenPairingCode(ctx context.Context, tx *store.ScopedTx, id string) (PairingCode, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, "id, staff_code, expires_at, failed_attempts", "zalo_pairing_code",
		"AND id = $2 AND used_at IS NULL AND cancelled_at IS NULL FOR UPDATE", id)
	if err != nil {
		return PairingCode{}, fmt.Errorf("zalo_pairing_code: lock: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return PairingCode{}, fmt.Errorf("zalo_pairing_code: lock: %w", err)
		}
		return PairingCode{}, ErrPairingCodeNotOpen
	}
	var c PairingCode
	if err := rows.Scan(&c.ID, &c.StaffCode, &c.ExpiresAt, &c.FailedAttempts); err != nil {
		return PairingCode{}, fmt.Errorf("zalo_pairing_code: scan: %w", err)
	}
	return c, rows.Err()
}

const markPairingCodeUsed = `UPDATE zalo_pairing_code SET used_at = $3, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND used_at IS NULL AND cancelled_at IS NULL`

// MarkPairingCodeUsed closes the code as used — once (the 0018 guard refuses re-opening it).
func (s *ZaloLinkStore) MarkPairingCodeUsed(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	return execOne(ctx, tx, markPairingCodeUsed, "zalo_pairing_code: mark used", ErrPairingCodeNotOpen,
		string(tx.TenantID()), id, at.UTC())
}

// ---- links -------------------------------------------------------------------------------------------

const zaloLinkColumns = "id, staff_code, linked_at"

func scanLinks(rows *sql.Rows) ([]domain.ZaloLink, error) {
	defer rows.Close()
	out := []domain.ZaloLink{}
	for rows.Next() {
		var l domain.ZaloLink
		if err := rows.Scan(&l.ID, &l.StaffCode, &l.LinkedAt); err != nil {
			return nil, fmt.Errorf("zalo_link: scan: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_link: rows: %w", err)
	}
	return out, nil
}

func firstLink(rows *sql.Rows, err error) (domain.ZaloLink, bool, error) {
	if err != nil {
		return domain.ZaloLink{}, false, fmt.Errorf("zalo_link: read: %w", err)
	}
	links, err := scanLinks(rows)
	if err != nil || len(links) == 0 {
		return domain.ZaloLink{}, false, err
	}
	return links[0], true, nil
}

// LiveLink reads one member of staff's live link. found=false: not linked.
func (s *ZaloLinkStore) LiveLink(ctx context.Context, staffCode string) (domain.ZaloLink, bool, error) {
	return firstLink(s.db.For(ctx).Query(ctx, zaloLinkColumns, "zalo_link",
		"AND staff_code = $2 AND unlinked_at IS NULL", staffCode))
}

// LiveLinks lists the commune's live links, oldest first. Refuses past MaxZaloLinksListed.
func (s *ZaloLinkStore) LiveLinks(ctx context.Context) ([]domain.ZaloLink, error) {
	rows, err := s.db.For(ctx).Query(ctx, zaloLinkColumns, "zalo_link",
		"AND unlinked_at IS NULL ORDER BY linked_at, id LIMIT $2", MaxZaloLinksListed+1)
	if err != nil {
		return nil, fmt.Errorf("zalo_link: list: %w", err)
	}
	out, err := scanLinks(rows)
	if err != nil {
		return nil, err
	}
	if len(out) > MaxZaloLinksListed {
		return nil, fmt.Errorf("zalo_link: more than %d live links in one commune", MaxZaloLinksListed)
	}
	return out, nil
}

// LockLiveLinkOfStaff locks one member of staff's live link. found=false: none.
func (s *ZaloLinkStore) LockLiveLinkOfStaff(ctx context.Context, tx *store.ScopedTx, staffCode string) (
	domain.ZaloLink, bool, error) {

	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	return firstLink(tx.Query(ctx, zaloLinkColumns, "zalo_link",
		"AND staff_code = $2 AND unlinked_at IS NULL FOR UPDATE", staffCode))
}

// LockLiveLinkOfChat locks the live link of one chat IN THIS COMMUNE (the shared bot). found=false: none
// here — a live link elsewhere is crosstenant's to end.
func (s *ZaloLinkStore) LockLiveLinkOfChat(ctx context.Context, tx *store.ScopedTx, chatID string) (
	domain.ZaloLink, bool, error) {

	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	return firstLink(tx.Query(ctx, zaloLinkColumns, "zalo_link",
		"AND bot_ref = $2 AND chat_id = $3 AND unlinked_at IS NULL FOR UPDATE", domain.SharedZaloBotRef, chatID))
}

const endZaloLink = `UPDATE zalo_link SET unlinked_at = $3, unlinked_by = $4, unlink_reason = $5, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND unlinked_at IS NULL`

// EndLink ends one live link (the `unlinked_*` trio is this table's soft delete, 0018). by is a business
// code or core/audit.SystemActor.
func (s *ZaloLinkStore) EndLink(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	return execOne(ctx, tx, endZaloLink, "zalo_link: end", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC(), by, reason)
}

const insertZaloLink = `INSERT INTO zalo_link (tenant_id, id, staff_code, bot_ref, chat_id, linked_at, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $6, $6)`

// InsertLink creates a live link of the shared bot. The caller has ended every live link of this member
// of staff and of this chat first (the two partial unique indexes of 0018).
func (s *ZaloLinkStore) InsertLink(ctx context.Context, tx *store.ScopedTx, id, staffCode, chatID string,
	at time.Time) error {

	if _, err := tx.Exec(ctx, insertZaloLink, string(tx.TenantID()), id, staffCode, domain.SharedZaloBotRef,
		chatID, at.UTC()); err != nil {
		return fmt.Errorf("zalo_link: insert: %w", err)
	}
	return nil
}

// LinkedChat is where a message to one member of staff goes. ChatID is personal data: never logged.
type LinkedChat struct {
	LinkID string
	ChatID string
}

// LiveChatsOf reads the live chats of several members of staff, keyed by staff code.
func (s *ZaloLinkStore) LiveChatsOf(ctx context.Context, tx *store.ScopedTx, staffCodes []string) (map[string]LinkedChat, error) {
	out := map[string]LinkedChat{}
	if len(staffCodes) == 0 {
		return out, nil
	}
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, "staff_code, id, chat_id", "zalo_link",
		"AND bot_ref = $2 AND staff_code = ANY($3) AND unlinked_at IS NULL", domain.SharedZaloBotRef, staffCodes)
	if err != nil {
		return nil, fmt.Errorf("zalo_link: read chats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var c LinkedChat
		if err := rows.Scan(&code, &c.LinkID, &c.ChatID); err != nil {
			return nil, fmt.Errorf("zalo_link: scan chats: %w", err)
		}
		out[code] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_link: read chats: %w", err)
	}
	return out, nil
}

// ---- the outbox -------------------------------------------------------------------------------------

// enqueueZaloDeliveries writes ONE zalo_delivery row per bell notice this call created — the notices are
// found by (idempotency key, the call's own created_at), so a notice an earlier call created is never
// queued twice, and the unique (tenant_id, notification_id) makes a retry a no-op anyway.
//
// THE ROW'S id IS THE NOTICE'S id. A delivery repeats exactly one notice (0018's unique key), so the
// notice's ULID is already a unique, internal, non-business identifier for it — and it lets one INSERT …
// SELECT do the whole batch. Test messages (no notice) get fresh ULIDs.
//
// The three filters that can be decided HERE, from this commune's own rows, are decided here, in 0018's
// order, so a skip is recorded the moment the notice exists ("mỗi lần bỏ qua ghi lý do"):
//
//	kenh-tat       no settings row, or switched off
//	loai-tat       this kind not selected
//	chua-lien-ket  the recipient has no live link
//
// Everything else — the bot's configuration (platform scope), quiet hours, the send — is the sender's,
// which re-checks all three at send time as well.
const enqueueZaloDeliveries = `INSERT INTO zalo_delivery
		(tenant_id, id, notification_id, staff_code, kind, status, skip_reason, next_attempt_at, created_at, updated_at)
	SELECT c.tenant_id, c.id, c.id, c.recipient_code, c.kind,
		CASE WHEN c.reason IS NULL THEN 'cho-gui' ELSE 'bo-qua' END,
		c.reason,
		CASE WHEN c.reason IS NULL THEN $3::timestamptz END,
		$3, $3
	FROM (
		SELECT n.tenant_id, n.id, n.recipient_code, n.kind,
			CASE
				WHEN s.tenant_id IS NULL OR NOT s.is_enabled THEN 'kenh-tat'
				WHEN NOT (n.kind = ANY (s.kinds)) THEN 'loai-tat'
				WHEN NOT EXISTS (SELECT 1 FROM zalo_link l
				                 WHERE l.tenant_id = n.tenant_id AND l.staff_code = n.recipient_code
				                   AND l.unlinked_at IS NULL) THEN 'chua-lien-ket'
			END AS reason
		FROM staff_notification n
		LEFT JOIN zalo_channel_setting s ON s.tenant_id = n.tenant_id
		WHERE n.tenant_id = $1 AND n.idempotency_key = ANY ($2) AND n.created_at = $3 AND n.deleted_at IS NULL
	) c
	ON CONFLICT (tenant_id, notification_id) DO NOTHING`

// EnqueueZaloDeliveries runs enqueueZaloDeliveries for the keys of one DeliverStaffNotifications call.
// Returns how many rows were written (queued and skipped together).
//
// INSIDE A SAVEPOINT, AND THAT IS THE POINT: "Zalo là kênh thêm … người chưa ghép nối vẫn phải nhận đủ"
// (ADR 0074 #1). A failure here is rolled back to the savepoint and returned, leaving the caller's
// transaction — the bell notices and their audit entry — intact and committable. Without it, one bad
// Zalo row would take every bell notice of the call down with it.
func (s *ZaloLinkStore) EnqueueZaloDeliveries(ctx context.Context, tx *store.ScopedTx, keys []string,
	createdAt time.Time) (int, error) {

	if len(keys) == 0 {
		return 0, nil
	}
	// A savepoint INSIDE this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `SAVEPOINT zalo_delivery_enqueue`); err != nil {
		return 0, fmt.Errorf("zalo_delivery: savepoint: %w", err)
	}
	res, err := tx.Exec(ctx, enqueueZaloDeliveries, string(tx.TenantID()), keys, createdAt.UTC())
	var n int64
	if err == nil {
		n, err = res.RowsAffected()
	}
	if err != nil {
		// Back to the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT zalo_delivery_enqueue`); rbErr != nil {
			return 0, fmt.Errorf("zalo_delivery: enqueue: %w (rollback to savepoint: %v)", err, rbErr)
		}
		return 0, fmt.Errorf("zalo_delivery: enqueue: %w", err)
	}
	// Release the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT zalo_delivery_enqueue`); err != nil {
		return 0, fmt.Errorf("zalo_delivery: release savepoint: %w", err)
	}
	return int(n), nil
}

const insertTestDelivery = `INSERT INTO zalo_delivery (tenant_id, id, notification_id, staff_code, kind, status, link_id,
		next_attempt_at, created_at, updated_at)
	VALUES ($1, $2, NULL, $3, 'thu-nghiem', 'cho-gui', $4, $5, $6, $6)`

// InsertTestDelivery records the test message of `POST zalo-links/current/test-messages` as owed, with
// its link already chosen. next is when the SENDER may pick it up should the direct send never record an
// outcome (a crash between the two): far enough ahead that the sender never races the direct send.
func (s *ZaloLinkStore) InsertTestDelivery(ctx context.Context, tx *store.ScopedTx, id, staffCode, linkID string,
	next, at time.Time) error {

	if _, err := tx.Exec(ctx, insertTestDelivery, string(tx.TenantID()), id, staffCode, linkID,
		next.UTC(), at.UTC()); err != nil {
		return fmt.Errorf("zalo_delivery: insert test: %w", err)
	}
	return nil
}

// DueZaloDelivery is one owed message, with what it says. Title/Body/Link are the bell notice's ("" for
// a test message).
type DueZaloDelivery struct {
	ID        string
	StaffCode string
	Kind      string
	Attempts  int
	Title     string
	Body      string
	Link      string
}

// MaxZaloDeliveryBatch bounds one commune's claim: the rows locked in one transaction.
const MaxZaloDeliveryBatch = 50

// claimDueDeliveries — the joined notice is constrained to the SAME commune ($1) in its ON clause, as
// core/store.QueryJoin requires of every joined table.
const claimDueDeliveries = `SELECT d.id, d.staff_code, d.kind, d.attempts,
		COALESCE(n.title, ''), COALESCE(n.body, ''), COALESCE(n.link, '')
	FROM zalo_delivery d
	LEFT JOIN staff_notification n ON n.tenant_id = $1 AND n.id = d.notification_id
	WHERE d.tenant_id = $1 AND d.status = 'cho-gui' AND d.deleted_at IS NULL AND d.next_attempt_at <= $2
	ORDER BY d.next_attempt_at, d.id
	LIMIT $3
	FOR UPDATE OF d SKIP LOCKED`

// ClaimDueDeliveries locks the commune's owed rows that are due, oldest first, SKIP LOCKED so two
// replicas never take the same row.
func (s *ZaloLinkStore) ClaimDueDeliveries(ctx context.Context, tx *store.ScopedTx, now time.Time, limit int) (
	[]DueZaloDelivery, error) {

	if limit <= 0 || limit > MaxZaloDeliveryBatch {
		limit = MaxZaloDeliveryBatch
	}
	rows, err := tx.Underlying().QueryContext(ctx, claimDueDeliveries, string(tx.TenantID()), now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("zalo_delivery: claim: %w", err)
	}
	defer rows.Close()
	var out []DueZaloDelivery
	for rows.Next() {
		var d DueZaloDelivery
		if err := rows.Scan(&d.ID, &d.StaffCode, &d.Kind, &d.Attempts, &d.Title, &d.Body, &d.Link); err != nil {
			return nil, fmt.Errorf("zalo_delivery: scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_delivery: claim: %w", err)
	}
	return out, nil
}

const (
	skipDelivery = `UPDATE zalo_delivery SET status = 'bo-qua', skip_reason = $3, next_attempt_at = NULL, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	postponeDelivery = `UPDATE zalo_delivery SET next_attempt_at = $3, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	startDeliveryAttempt = `UPDATE zalo_delivery SET attempts = attempts + 1, link_id = $3, next_attempt_at = $4,
		updated_at = $5 WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	markDeliverySent = `UPDATE zalo_delivery SET status = 'da-gui', sent_at = $3, next_attempt_at = NULL,
		error_class = NULL, updated_at = $3
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui' AND link_id IS NOT NULL`
	retryDelivery = `UPDATE zalo_delivery SET error_class = $3, next_attempt_at = $4, updated_at = $5
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	failDelivery = `UPDATE zalo_delivery SET status = 'that-bai', error_class = $3, next_attempt_at = NULL, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
)

// SkipDelivery ends an owed row as 'bo-qua' with its reason.
func (s *ZaloLinkStore) SkipDelivery(ctx context.Context, tx *store.ScopedTx, id, reason string, at time.Time) error {
	return execOne(ctx, tx, skipDelivery, "zalo_delivery: skip", ErrZaloRowChanged,
		string(tx.TenantID()), id, reason, at.UTC())
}

// PostponeDelivery moves an owed row's next attempt (quiet hours) without counting an attempt.
func (s *ZaloLinkStore) PostponeDelivery(ctx context.Context, tx *store.ScopedTx, id string, next, at time.Time) error {
	return execOne(ctx, tx, postponeDelivery, "zalo_delivery: postpone", ErrZaloRowChanged,
		string(tx.TenantID()), id, next.UTC(), at.UTC())
}

// StartAttempt counts one attempt, records the link it goes through, and leases the row until leaseUntil
// so no other pass sends it meanwhile. A pass that dies leaves the lease to expire and the row is sent
// again — at least once, never silently lost.
func (s *ZaloLinkStore) StartAttempt(ctx context.Context, tx *store.ScopedTx, id, linkID string, leaseUntil,
	at time.Time) error {

	return execOne(ctx, tx, startDeliveryAttempt, "zalo_delivery: start attempt", ErrZaloRowChanged,
		string(tx.TenantID()), id, linkID, leaseUntil.UTC(), at.UTC())
}

// MarkDeliverySent ends an owed row as 'da-gui'.
func (s *ZaloLinkStore) MarkDeliverySent(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	return execOne(ctx, tx, markDeliverySent, "zalo_delivery: mark sent", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC())
}

// RetryDelivery keeps an owed row owed after a retryable failure, with the class and the next attempt.
func (s *ZaloLinkStore) RetryDelivery(ctx context.Context, tx *store.ScopedTx, id, errorClass string, next,
	at time.Time) error {

	return execOne(ctx, tx, retryDelivery, "zalo_delivery: retry", ErrZaloRowChanged,
		string(tx.TenantID()), id, errorClass, next.UTC(), at.UTC())
}

// FailDelivery ends an owed row as 'that-bai' with its class (never Zalo's text: the token is in the URL).
func (s *ZaloLinkStore) FailDelivery(ctx context.Context, tx *store.ScopedTx, id, errorClass string, at time.Time) error {
	return execOne(ctx, tx, failDelivery, "zalo_delivery: fail", ErrZaloRowChanged,
		string(tx.TenantID()), id, errorClass, at.UTC())
}

// execOne runs a guarded single-row write of this commune's ScopedTx; zero rows is notFound. Every
// statement passed here carries `tenant_id = $1` with the transaction's commune as $1.
func execOne(ctx context.Context, tx *store.ScopedTx, stmt, what string, notFound error, args ...any) error {
	res, err := tx.Exec(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}
