package store

// ZALO GROUP CHATS — `zalo_group_pairing_code`, `zalo_group_link`, `zalo_group_delivery` (migration 0029,
// ADR 0092). SQL, and nothing else.
//
// EVERY STATEMENT HERE IS ONE COMMUNE'S: $1 is the commune, from the context or the transaction (rule 1,
// invariants 4 and 5). The statements that must cross communes — finding a group code by its hash for the
// shared bot, asking whether a chat is live as a group anywhere — are in internal/store/crosstenant, each with
// its `// @cross-tenant:`.
//
// chat_id IS PERSONAL DATA (ADR 0092 §Hệ quả). It is written only by InsertGroupLink, read only by the
// SEND (ClaimDueGroupDeliveries) and by the webhook's own-chat lookup as a bind parameter, never put into an
// error string.
//
// NO audit.Write HERE: internal/app opens the transaction and writes the entry inside it.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloGroupStore is the only scoped path to the three commune tables of 0029.
type ZaloGroupStore struct {
	db *store.DB
}

func NewZaloGroupStore(db *store.DB) *ZaloGroupStore { return &ZaloGroupStore{db: db} }

var (
	// ErrZaloGroupNotFound — no LIVE group link with that id in this commune (unknown, ended, another
	// commune's: one answer).
	ErrZaloGroupNotFound = errors.New("zalo_group_link: not found")
	// ErrZaloGroupChatLinked — the chat is live as a group already (0029 zalo_group_link_one_live_per_chat).
	ErrZaloGroupChatLinked = errors.New("zalo_group_link: the chat is live as a group already")
	// ErrZaloGroupChatPaired — the chat is live as a PERSONAL link (0029 zalo_chat_one_link_kind).
	ErrZaloGroupChatPaired = errors.New("zalo_group_link: the chat is paired to a person")
	// ErrZaloGroupBotNotLive — the bot named is not the one serving the commune (0022's
	// zalo_link_commune_bot_live, attached to zalo_group_link by 0029).
	ErrZaloGroupBotNotLive = errors.New("zalo_group_link: the bot is not the commune's live bot")
)

// MaxZaloGroupsListed bounds a commune's live groups read at once. A commune has a handful (one per unit,
// plus a few commune-wide); past this the list is refused, never truncated — a truncated list would leave
// groups out of the enqueue.
const MaxZaloGroupsListed = 500

// ---- group pairing codes ---------------------------------------------------------------------------

// GroupPairingCode is one locked `zalo_group_pairing_code` row.
type GroupPairingCode struct {
	ID             string
	IssuedBy       string
	OrgUnitID      string // "" = the whole commune
	Kinds          []string
	ExpiresAt      time.Time
	FailedAttempts int
}

const insertGroupPairingCode = `INSERT INTO zalo_group_pairing_code
		(tenant_id, id, issued_by, org_unit_id, kinds, code_hash, expires_at, created_at, updated_at)
	VALUES ($1, $2, $3, NULLIF($4, ''), string_to_array($5, ','), $6, $7, $8, $8)`

// InsertGroupPairingCode stores the HASH of a new group code with its scope. ErrPairingCodeTaken: that
// hash was issued in this commune before, and the caller draws again — inside a SAVEPOINT so the refusal
// aborts nothing else of the caller's transaction.
func (s *ZaloGroupStore) InsertGroupPairingCode(ctx context.Context, tx *store.ScopedTx, id, issuedBy, orgUnitID string,
	kinds []string, hash []byte, expiresAt, at time.Time) error {

	// A savepoint INSIDE this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `SAVEPOINT zalo_group_pairing_code_insert`); err != nil {
		return fmt.Errorf("zalo_group_pairing_code: savepoint: %w", err)
	}
	if _, err := tx.Exec(ctx, insertGroupPairingCode, string(tx.TenantID()), id, issuedBy, orgUnitID,
		strings.Join(kinds, ","), hash, expiresAt.UTC(), at.UTC()); err != nil {
		// Back to the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT zalo_group_pairing_code_insert`); rbErr != nil {
			return fmt.Errorf("zalo_group_pairing_code: insert: %w (rollback to savepoint: %v)", err, rbErr)
		}
		if isUniqueViolation(err) {
			return ErrPairingCodeTaken
		}
		return fmt.Errorf("zalo_group_pairing_code: insert: %w", err)
	}
	// Release the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT zalo_group_pairing_code_insert`); err != nil {
		return fmt.Errorf("zalo_group_pairing_code: release savepoint: %w", err)
	}
	return nil
}

// OpenGroupPairingCodeByHash finds an OPEN group code of THIS commune by its hash — a commune bot's webhook,
// whose commune comes from Host. found=false: none open here (0029 never issues a hash twice in a commune).
func (s *ZaloGroupStore) OpenGroupPairingCodeByHash(ctx context.Context, hash []byte) (string, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, "id", "zalo_group_pairing_code",
		"AND code_hash = $2 AND used_at IS NULL AND cancelled_at IS NULL LIMIT 2", hash)
	if err != nil {
		return "", false, fmt.Errorf("zalo_group_pairing_code: find by hash: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", false, fmt.Errorf("zalo_group_pairing_code: find by hash: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", false, fmt.Errorf("zalo_group_pairing_code: find by hash: %w", err)
	}
	if len(ids) != 1 {
		return "", false, nil // none, or (impossible by 0029) two: no match, fail closed
	}
	return ids[0], true, nil
}

// LockOpenGroupPairingCode reads one OPEN group code of this commune, FOR UPDATE — two groups sending the
// same code at once cannot both use it. ErrPairingCodeNotOpen when it is used, cancelled or absent.
func (s *ZaloGroupStore) LockOpenGroupPairingCode(ctx context.Context, tx *store.ScopedTx, id string) (GroupPairingCode, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, "id, issued_by, COALESCE(org_unit_id, ''), array_to_string(kinds, ','), expires_at, failed_attempts",
		"zalo_group_pairing_code", "AND id = $2 AND used_at IS NULL AND cancelled_at IS NULL FOR UPDATE", id)
	if err != nil {
		return GroupPairingCode{}, fmt.Errorf("zalo_group_pairing_code: lock: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return GroupPairingCode{}, fmt.Errorf("zalo_group_pairing_code: lock: %w", err)
		}
		return GroupPairingCode{}, ErrPairingCodeNotOpen
	}
	var (
		c     GroupPairingCode
		kinds string
	)
	if err := rows.Scan(&c.ID, &c.IssuedBy, &c.OrgUnitID, &kinds, &c.ExpiresAt, &c.FailedAttempts); err != nil {
		return GroupPairingCode{}, fmt.Errorf("zalo_group_pairing_code: scan: %w", err)
	}
	c.Kinds = splitKinds(kinds)
	return c, rows.Err()
}

const markGroupPairingCodeUsed = `UPDATE zalo_group_pairing_code SET used_at = $3, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND used_at IS NULL AND cancelled_at IS NULL`

// MarkGroupPairingCodeUsed closes the code as used — once (0029's guard refuses re-opening it).
func (s *ZaloGroupStore) MarkGroupPairingCodeUsed(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	return execOne(ctx, tx, markGroupPairingCodeUsed, "zalo_group_pairing_code: mark used", ErrPairingCodeNotOpen,
		string(tx.TenantID()), id, at.UTC())
}

// ---- group links -------------------------------------------------------------------------------------

const zaloGroupLinkColumns = "id, bot_ref, COALESCE(org_unit_id, ''), array_to_string(kinds, ','), linked_at, linked_by"

func scanGroupLinks(rows *sql.Rows) ([]domain.ZaloGroupLink, error) {
	defer rows.Close()
	out := []domain.ZaloGroupLink{}
	for rows.Next() {
		var (
			l     domain.ZaloGroupLink
			kinds string
		)
		if err := rows.Scan(&l.ID, &l.BotRef, &l.OrgUnitID, &kinds, &l.LinkedAt, &l.LinkedBy); err != nil {
			return nil, fmt.Errorf("zalo_group_link: scan: %w", err)
		}
		l.Kinds = splitKinds(kinds)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_group_link: rows: %w", err)
	}
	return out, nil
}

func splitKinds(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

// LiveGroupLinks lists the commune's live group links, oldest first. Refuses past MaxZaloGroupsListed.
func (s *ZaloGroupStore) LiveGroupLinks(ctx context.Context) ([]domain.ZaloGroupLink, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, zaloGroupLinkColumns, "zalo_group_link",
		"AND unlinked_at IS NULL ORDER BY linked_at, id LIMIT $2", MaxZaloGroupsListed+1)
	if err != nil {
		return nil, fmt.Errorf("zalo_group_link: list: %w", err)
	}
	return boundGroupLinks(scanGroupLinks(rows))
}

// LiveGroupTargets reads the commune's live groups inside the delivering transaction — the enqueue's read
// (0029 zalo_group_link_live_by_unit). Refuses past MaxZaloGroupsListed.
func (s *ZaloGroupStore) LiveGroupTargets(ctx context.Context, tx *store.ScopedTx) ([]domain.ZaloGroupTarget, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, zaloGroupLinkColumns, "zalo_group_link",
		"AND unlinked_at IS NULL ORDER BY id LIMIT $2", MaxZaloGroupsListed+1)
	if err != nil {
		return nil, fmt.Errorf("zalo_group_link: read targets: %w", err)
	}
	links, err := boundGroupLinks(scanGroupLinks(rows))
	if err != nil {
		return nil, err
	}
	out := make([]domain.ZaloGroupTarget, 0, len(links))
	for _, l := range links {
		out = append(out, domain.ZaloGroupTarget{ID: l.ID, OrgUnitID: l.OrgUnitID, Kinds: l.Kinds})
	}
	return out, nil
}

func boundGroupLinks(links []domain.ZaloGroupLink, err error) ([]domain.ZaloGroupLink, error) {
	if err != nil {
		return nil, err
	}
	if len(links) > MaxZaloGroupsListed {
		return nil, fmt.Errorf("zalo_group_link: more than %d live groups in one commune", MaxZaloGroupsListed)
	}
	return links, nil
}

// LockLiveGroupLink locks one live group link of this commune. ErrZaloGroupNotFound: none live by that id.
func (s *ZaloGroupStore) LockLiveGroupLink(ctx context.Context, tx *store.ScopedTx, id string) (domain.ZaloGroupLink, error) {
	// ScopedTx.Query adds `WHERE tenant_id = $1` and binds the transaction's commune.
	rows, err := tx.Query(ctx, zaloGroupLinkColumns, "zalo_group_link",
		"AND id = $2 AND unlinked_at IS NULL FOR UPDATE", id)
	if err != nil {
		return domain.ZaloGroupLink{}, fmt.Errorf("zalo_group_link: lock: %w", err)
	}
	links, err := scanGroupLinks(rows)
	if err != nil {
		return domain.ZaloGroupLink{}, err
	}
	if len(links) == 0 {
		return domain.ZaloGroupLink{}, ErrZaloGroupNotFound
	}
	return links[0], nil
}

// LiveGroupLinkOfChat reports whether one chat of one bot is live as a group IN THIS COMMUNE — a commune
// bot's group links live in its commune alone (0022 trigger), so this is the whole answer for that bot.
func (s *ZaloGroupStore) LiveGroupLinkOfChat(ctx context.Context, botRef, chatID string) (bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, "id", "zalo_group_link",
		"AND bot_ref = $2 AND chat_id = $3 AND unlinked_at IS NULL LIMIT 1", botRef, chatID)
	if err != nil {
		return false, fmt.Errorf("zalo_group_link: find by chat: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("zalo_group_link: find by chat: %w", err)
	}
	return found, nil
}

const insertGroupLinkStmt = `INSERT INTO zalo_group_link
		(tenant_id, id, bot_ref, chat_id, org_unit_id, kinds, pairing_code_id, linked_at, linked_by, created_at, updated_at)
	VALUES ($1, $2, $3, $4, NULLIF($5, ''), string_to_array($6, ','), $7, $8, $9, $8, $8)`

// InsertGroupLink creates a live group link through botRef, copying the code's scope. The three refusals
// PostgreSQL decides are mapped to sentinels, so the webhook answers each with its own sentence: the chat
// live as a group (any commune), the chat paired to a person, the bot not the commune's live one. No
// savepoint: each of them aborts the linking as a whole, which is the caller's rollback.
func (s *ZaloGroupStore) InsertGroupLink(ctx context.Context, tx *store.ScopedTx, id, botRef, chatID string,
	c GroupPairingCode, at time.Time) error {

	// tenant_id = $1 is the transaction's commune (ScopedTx).
	_, err := tx.Exec(ctx, insertGroupLinkStmt, string(tx.TenantID()), id, botRef, chatID, c.OrgUnitID,
		strings.Join(c.Kinds, ","), c.ID, at.UTC(), c.IssuedBy)
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch {
		case pg.Code == "23505" && pg.ConstraintName == "zalo_group_link_one_live_per_chat":
			return ErrZaloGroupChatLinked
		case pg.Code == "P0001" && strings.Contains(pg.Message, "paired to a person"):
			return ErrZaloGroupChatPaired
		case pg.Code == "P0001":
			return ErrZaloGroupBotNotLive
		}
	}
	return fmt.Errorf("zalo_group_link: insert: %w", err)
}

const updateGroupLinkScope = `UPDATE zalo_group_link SET org_unit_id = NULLIF($3, ''), kinds = string_to_array($4, ','),
	updated_at = $5 WHERE tenant_id = $1 AND id = $2 AND unlinked_at IS NULL`

// UpdateGroupLinkScope rewrites a live link's scope and kinds (0029: the only editable columns).
func (s *ZaloGroupStore) UpdateGroupLinkScope(ctx context.Context, tx *store.ScopedTx, id, orgUnitID string,
	kinds []string, at time.Time) error {

	return execOne(ctx, tx, updateGroupLinkScope, "zalo_group_link: update scope", ErrZaloGroupNotFound,
		string(tx.TenantID()), id, orgUnitID, strings.Join(kinds, ","), at.UTC())
}

const endGroupLink = `UPDATE zalo_group_link SET unlinked_at = $3, unlinked_by = $4, unlink_reason = $5, updated_at = $3
	WHERE tenant_id = $1 AND id = $2 AND unlinked_at IS NULL`

// EndGroupLink ends one live group link (the `unlinked_*` trio is its soft delete). by is a business code.
func (s *ZaloGroupStore) EndGroupLink(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	return execOne(ctx, tx, endGroupLink, "zalo_group_link: end", ErrZaloGroupNotFound,
		string(tx.TenantID()), id, at.UTC(), by, reason)
}

const endLiveGroupLinksOfBot = `UPDATE zalo_group_link SET unlinked_at = $3, unlinked_by = $4, unlink_reason = $5,
		updated_at = $3
	WHERE tenant_id = $1 AND bot_ref = $2 AND unlinked_at IS NULL
	RETURNING ` + zaloGroupLinkColumns

// EndLiveGroupLinksOfBot ends every live group link of botRef in this commune and returns them, for one
// audit entry each — the bot switch (0029: the deferred adopt/retire checks refuse a commit that leaves one).
func (s *ZaloGroupStore) EndLiveGroupLinksOfBot(ctx context.Context, tx *store.ScopedTx, botRef, by, reason string,
	at time.Time) ([]domain.ZaloGroupLink, error) {

	// THE TENANT SCOPE HERE IS MANUAL: tx.Underlying() bypasses ScopedTx's own binding (an UPDATE …
	// RETURNING is neither its Query nor its Exec), so `tenant_id = $1` in the statement and tx.TenantID()
	// as $1 below are what keep this to the transaction's commune. Keep both.
	rows, err := tx.Underlying().QueryContext(ctx, endLiveGroupLinksOfBot, string(tx.TenantID()), botRef, at.UTC(), by, reason)
	if err != nil {
		return nil, fmt.Errorf("zalo_group_link: end the bot's group links: %w", err)
	}
	return scanGroupLinks(rows)
}

// CountLiveGroupLinksOfBot counts this commune's live group links through botRef — what a switch of bot
// would end, beside the staff count.
func (s *ZaloGroupStore) CountLiveGroupLinksOfBot(ctx context.Context, botRef string) (int, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, "count(*)", "zalo_group_link", "AND bot_ref = $2 AND unlinked_at IS NULL", botRef)
	if err != nil {
		return 0, fmt.Errorf("zalo_group_link: count: %w", err)
	}
	defer rows.Close()
	n := 0
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, fmt.Errorf("zalo_group_link: count: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("zalo_group_link: count: %w", err)
	}
	return n, nil
}

// ---- the group outbox ----------------------------------------------------------------------------------

// GroupDeliveryRow is one message owed to one group, as the enqueue writes it.
type GroupDeliveryRow struct {
	ID, GroupLinkID, IdempotencyKey, Kind string
	Title, Body, Link                     string
}

// insertGroupDeliveries writes the batch through unnest — one statement for the call. ON CONFLICT on 0029's
// (tenant_id, group_link_id, idempotency_key): first delivery wins (ADR 0092 #5), a retry is a no-op.
const insertGroupDeliveries = `INSERT INTO zalo_group_delivery
		(tenant_id, id, group_link_id, idempotency_key, kind, title, body, link, status, next_attempt_at,
		 created_at, updated_at)
	SELECT $1, r.id, r.group_link_id, r.idempotency_key, r.kind, r.title, r.body, r.link, 'cho-gui', $9, $9, $9
	FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[], $8::text[])
		AS r(id, group_link_id, idempotency_key, kind, title, body, link)
	ON CONFLICT (tenant_id, group_link_id, idempotency_key) DO NOTHING`

// EnqueueGroupDeliveries reads the commune's live groups, lets match choose the rows, and writes them —
// INSIDE ITS OWN SAVEPOINT, beside zalo_delivery's (transaction boundary gui_tin_nhom_zalo): a failure is
// rolled back to the savepoint and returned, leaving the caller's transaction — the bell notices and their
// entry — intact. Returns how many rows were written.
func (s *ZaloGroupStore) EnqueueGroupDeliveries(ctx context.Context, tx *store.ScopedTx,
	match func([]domain.ZaloGroupTarget) ([]GroupDeliveryRow, error), at time.Time) (int, error) {

	// A savepoint INSIDE this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `SAVEPOINT zalo_group_delivery_enqueue`); err != nil {
		return 0, fmt.Errorf("zalo_group_delivery: savepoint: %w", err)
	}
	n, err := s.enqueueGroupDeliveries(ctx, tx, match, at)
	if err != nil {
		// Back to the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT zalo_group_delivery_enqueue`); rbErr != nil {
			return 0, fmt.Errorf("zalo_group_delivery: enqueue: %w (rollback to savepoint: %v)", err, rbErr)
		}
		return 0, fmt.Errorf("zalo_group_delivery: enqueue: %w", err)
	}
	// Release the savepoint of this commune's ScopedTx (tenant_id is bound by the transaction).
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT zalo_group_delivery_enqueue`); err != nil {
		return 0, fmt.Errorf("zalo_group_delivery: release savepoint: %w", err)
	}
	return n, nil
}

func (s *ZaloGroupStore) enqueueGroupDeliveries(ctx context.Context, tx *store.ScopedTx,
	match func([]domain.ZaloGroupTarget) ([]GroupDeliveryRow, error), at time.Time) (int, error) {

	groups, err := s.LiveGroupTargets(ctx, tx)
	if err != nil || len(groups) == 0 {
		return 0, err
	}
	rows, err := match(groups)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	cols := make([][]string, 7)
	for _, r := range rows {
		for i, v := range []string{r.ID, r.GroupLinkID, r.IdempotencyKey, r.Kind, r.Title, r.Body, r.Link} {
			cols[i] = append(cols[i], v)
		}
	}
	res, err := tx.Exec(ctx, insertGroupDeliveries, string(tx.TenantID()), cols[0], cols[1], cols[2], cols[3], cols[4],
		cols[5], cols[6], at.UTC())
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// DueZaloGroupDelivery is one owed group message with its link's state at claim time. ChatID is personal
// data: never logged.
type DueZaloGroupDelivery struct {
	ID          string
	GroupLinkID string
	Kind        string
	Attempts    int
	Title       string
	Body        string
	Link        string
	// The link, read in the same statement: live, through which bot, its kinds now, its chat.
	LinkLive   bool
	BotRef     string
	GroupKinds []string
	ChatID     string
}

// claimDueGroupDeliveries — the joined link is constrained to the SAME commune ($1) in its ON clause, as
// core/store.QueryJoin requires of every joined table.
const claimDueGroupDeliveries = `SELECT d.id, d.group_link_id, d.kind, d.attempts, d.title, d.body, d.link,
		g.unlinked_at IS NULL, g.bot_ref, array_to_string(g.kinds, ','), g.chat_id
	FROM zalo_group_delivery d
	JOIN zalo_group_link g ON g.tenant_id = $1 AND g.id = d.group_link_id
	WHERE d.tenant_id = $1 AND d.status = 'cho-gui' AND d.deleted_at IS NULL AND d.next_attempt_at <= $2
	ORDER BY d.next_attempt_at, d.id
	LIMIT $3
	FOR UPDATE OF d SKIP LOCKED`

// ClaimDueGroupDeliveries locks the commune's owed group rows that are due, oldest first, SKIP LOCKED so two
// replicas never take the same row (zalo_delivery's shape; 0029 has no lease column either).
func (s *ZaloGroupStore) ClaimDueGroupDeliveries(ctx context.Context, tx *store.ScopedTx, now time.Time, limit int) (
	[]DueZaloGroupDelivery, error) {

	if limit <= 0 || limit > MaxZaloDeliveryBatch {
		limit = MaxZaloDeliveryBatch
	}
	// THE TENANT SCOPE HERE IS MANUAL: a join with FOR UPDATE OF, on the transaction's connection —
	// `d.tenant_id = $1` and `g.tenant_id = $1` with tx.TenantID() as $1. Keep both.
	rows, err := tx.Underlying().QueryContext(ctx, claimDueGroupDeliveries, string(tx.TenantID()), now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("zalo_group_delivery: claim: %w", err)
	}
	defer rows.Close()
	var out []DueZaloGroupDelivery
	for rows.Next() {
		var (
			d     DueZaloGroupDelivery
			kinds string
		)
		if err := rows.Scan(&d.ID, &d.GroupLinkID, &d.Kind, &d.Attempts, &d.Title, &d.Body, &d.Link,
			&d.LinkLive, &d.BotRef, &kinds, &d.ChatID); err != nil {
			return nil, fmt.Errorf("zalo_group_delivery: scan: %w", err)
		}
		d.GroupKinds = splitKinds(kinds)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zalo_group_delivery: claim: %w", err)
	}
	return out, nil
}

const (
	skipGroupDelivery = `UPDATE zalo_group_delivery SET status = 'bo-qua', skip_reason = $3, next_attempt_at = NULL,
		updated_at = $4 WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	postponeGroupDelivery = `UPDATE zalo_group_delivery SET next_attempt_at = $3, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	startGroupDeliveryAttempt = `UPDATE zalo_group_delivery SET attempts = attempts + 1, next_attempt_at = $3,
		updated_at = $4 WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	markGroupDeliverySent = `UPDATE zalo_group_delivery SET status = 'da-gui', sent_at = $3, next_attempt_at = NULL,
		error_class = NULL, updated_at = $3 WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	retryGroupDelivery = `UPDATE zalo_group_delivery SET error_class = $3, next_attempt_at = $4, updated_at = $5
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	failGroupDelivery = `UPDATE zalo_group_delivery SET status = 'that-bai', error_class = $3, next_attempt_at = NULL,
		updated_at = $4 WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
)

// SkipGroupDelivery ends an owed group row as 'bo-qua' with its reason.
func (s *ZaloGroupStore) SkipGroupDelivery(ctx context.Context, tx *store.ScopedTx, id, reason string, at time.Time) error {
	return execOne(ctx, tx, skipGroupDelivery, "zalo_group_delivery: skip", ErrZaloRowChanged,
		string(tx.TenantID()), id, reason, at.UTC())
}

// PostponeGroupDelivery moves an owed group row's next attempt (quiet hours) without counting an attempt.
func (s *ZaloGroupStore) PostponeGroupDelivery(ctx context.Context, tx *store.ScopedTx, id string, next, at time.Time) error {
	return execOne(ctx, tx, postponeGroupDelivery, "zalo_group_delivery: postpone", ErrZaloRowChanged,
		string(tx.TenantID()), id, next.UTC(), at.UTC())
}

// StartGroupAttempt counts one attempt and leases the row until leaseUntil (next_attempt_at is the lease,
// as on zalo_delivery): a pass that dies leaves it to expire and the row is sent again — at least once.
func (s *ZaloGroupStore) StartGroupAttempt(ctx context.Context, tx *store.ScopedTx, id string, leaseUntil, at time.Time) error {
	return execOne(ctx, tx, startGroupDeliveryAttempt, "zalo_group_delivery: start attempt", ErrZaloRowChanged,
		string(tx.TenantID()), id, leaseUntil.UTC(), at.UTC())
}

// MarkGroupDeliverySent ends an owed group row as 'da-gui'.
func (s *ZaloGroupStore) MarkGroupDeliverySent(ctx context.Context, tx *store.ScopedTx, id string, at time.Time) error {
	return execOne(ctx, tx, markGroupDeliverySent, "zalo_group_delivery: mark sent", ErrZaloRowChanged,
		string(tx.TenantID()), id, at.UTC())
}

// RetryGroupDelivery keeps an owed group row owed after a retryable failure.
func (s *ZaloGroupStore) RetryGroupDelivery(ctx context.Context, tx *store.ScopedTx, id, errorClass string, next,
	at time.Time) error {

	return execOne(ctx, tx, retryGroupDelivery, "zalo_group_delivery: retry", ErrZaloRowChanged,
		string(tx.TenantID()), id, errorClass, next.UTC(), at.UTC())
}

// FailGroupDelivery ends an owed group row as 'that-bai' with its class. It does NOT end the link: unlinking
// is a staff act on ViGov (ADR 0092 #8).
func (s *ZaloGroupStore) FailGroupDelivery(ctx context.Context, tx *store.ScopedTx, id, errorClass string, at time.Time) error {
	return execOne(ctx, tx, failGroupDelivery, "zalo_group_delivery: fail", ErrZaloRowChanged,
		string(tx.TenantID()), id, errorClass, at.UTC())
}
