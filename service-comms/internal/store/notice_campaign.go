package store

// NOTICE CAMPAIGNS — `notice_campaign`, `notice_recipient` (migration 0032; ADR 0099 #8). SQL, and nothing
// else.
//
// EVERY STATEMENT IS ONE COMMUNE'S: $1 is the commune, from the context or the transaction (rule 1,
// invariants 4 and 5). Joined tables are constrained to $1 in their ON clause (core/store.QueryJoin).
//
// THE FULL PHONE NUMBER IS NEVER BOUND IN CLEAR HERE. notice_recipient holds the directory ref and the
// MASKED number (0032's CHECK refuses anything else); a typed / imported row's number and message arrive
// already SEALED by internal/app (0033, ADR 0099 S6) and every final outcome UPDATE empties them.
//
// NO audit.Write HERE: internal/app opens the transaction and writes the entry inside it. NO DELETE: 0032's
// guards refuse it, and nothing here tries.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ErrNoticeRowChanged — the recipient row is no longer owed (settled by another pass after its lease ran
// out): the write found nothing to change.
var ErrNoticeRowChanged = errors.New("notice_recipient: row no longer owed")

// MaxNoticeClaim bounds one claim: the rows one batch reveals and sends. It is identity's ceiling on one
// RevealNoticeRecipientPhones call (identityclient.MaxNoticeRevealRefs), so a batch is at most one reveal
// per (campaign, recipient kind) and never a split one.
const MaxNoticeClaim = 500

// maxRecipientsPerInsert bounds one multi-row INSERT: 3 + 1 000 × 7 bind parameters stays far under
// PostgreSQL's 65 535, and one statement's size stays reasonable.
const maxRecipientsPerInsert = 1000

// NoticeCampaignStore is the only scoped path to notice_campaign and notice_recipient.
type NoticeCampaignStore struct {
	db *store.DB
}

func NewNoticeCampaignStore(db *store.DB) *NoticeCampaignStore { return &NoticeCampaignStore{db: db} }

// NoticeCampaignView is a campaign with its source's display names (the OA's name, or the account's label
// and Brandname) — read through a join, because a retired OA / account keeps its row (rule 7) and the
// history must still name it.
type NoticeCampaignView struct {
	Campaign     domain.NoticeCampaign
	OAName       string
	AccountLabel string
	Brandname    string
}

// noticeCampaignColumns IS READ BY POSITION in scanNoticeCampaign. Money is read as text: exact decimals,
// never floats. Prefixed `c.` because the list joins.
const noticeCampaignColumns = `c.id, c.title, c.channel, COALESCE(c.oa_row_id, ''), COALESCE(c.sms_account_id, ''),
	COALESCE(c.template_ref, ''), c.template_name, COALESCE(c.template_content, ''), c.params::text, c.audience_kind,
	c.audience_ids::text, c.recipient_count, c.skipped_count, COALESCE(c.unit_price::text, ''),
	COALESCE(c.estimated_cost::text, ''), c.status, c.sent_count, c.failed_count, c.created_at, c.created_by,
	c.finished_at, COALESCE(c.sms_content_mode, ''), c.invalid_count, COALESCE(c.recipient_mapping::text, ''),
	COALESCE(c.attribute_mapping::text, '')`

func scanNoticeCampaign(scan func(...any) error, extra ...any) (domain.NoticeCampaign, error) {
	var (
		c        domain.NoticeCampaign
		params   string
		audience string
		finished sql.NullTime
		mapping  string
		attrMap  string
	)
	dest := append([]any{&c.ID, &c.Title, &c.Channel, &c.OARowID, &c.SmsAccountID, &c.TemplateRef, &c.TemplateName,
		&c.TemplateContent, &params, &c.Audience.Kind, &audience, &c.RecipientCount, &c.SkippedCount, &c.UnitPrice,
		&c.EstimatedCost, &c.Status, &c.SentCount, &c.FailedCount, &c.CreatedAt, &c.CreatedBy, &finished,
		&c.SmsContentMode, &c.InvalidCount, &mapping, &attrMap}, extra...)
	if err := scan(dest...); err != nil {
		return domain.NoticeCampaign{}, err
	}
	if err := json.Unmarshal([]byte(params), &c.Params); err != nil {
		return domain.NoticeCampaign{}, fmt.Errorf("notice_campaign: decode params: %w", err)
	}
	if err := json.Unmarshal([]byte(audience), &c.Audience.IDs); err != nil {
		return domain.NoticeCampaign{}, fmt.Errorf("notice_campaign: decode audience_ids: %w", err)
	}
	if mapping != "" {
		c.RecipientMapping = &domain.NoticeRecipientMapping{}
		if err := json.Unmarshal([]byte(mapping), c.RecipientMapping); err != nil {
			return domain.NoticeCampaign{}, fmt.Errorf("notice_campaign: decode recipient_mapping: %w", err)
		}
	}
	if attrMap != "" {
		if err := json.Unmarshal([]byte(attrMap), &c.AttributeMapping); err != nil {
			return domain.NoticeCampaign{}, fmt.Errorf("notice_campaign: decode attribute_mapping: %w", err)
		}
	}
	c.FinishedAt = finished.Time
	return c, nil
}

const insertNoticeCampaign = `INSERT INTO notice_campaign (tenant_id, id, title, channel, oa_row_id, sms_account_id,
		template_ref, template_name, template_content, params, audience_kind, audience_ids, recipient_count,
		skipped_count, unit_price, estimated_cost, status, created_at, created_by, sms_content_mode, invalid_count,
		recipient_mapping, attribute_mapping)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15::numeric, $16::numeric, 'dang-gui', $17, $18,
		$19, $20, $21::jsonb, $22::jsonb)`

// InsertCampaign writes the campaign row ('dang-gui') in the transaction's commune. The recipient rows go
// in with InsertRecipients in the SAME transaction (ADR 0099 #8).
func (s *NoticeCampaignStore) InsertCampaign(ctx context.Context, tx *store.ScopedTx, c domain.NoticeCampaign) error {
	p := c.Params
	if p == nil {
		p = map[string]string{}
	}
	params, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("notice_campaign: encode params: %w", err)
	}
	ids := c.Audience.IDs
	if ids == nil {
		ids = []string{}
	}
	audience, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("notice_campaign: encode audience_ids: %w", err)
	}
	var mapping any // NULL unless tep-nhap (0033 notice_campaign_recipient_mapping_shape)
	if c.RecipientMapping != nil {
		b, err := json.Marshal(c.RecipientMapping)
		if err != nil {
			return fmt.Errorf("notice_campaign: encode recipient_mapping: %w", err)
		}
		mapping = string(b)
	}
	var attrMap any // NULL unless a directory campaign mapped an attribute (0034)
	if len(c.AttributeMapping) > 0 {
		b, err := json.Marshal(c.AttributeMapping)
		if err != nil {
			return fmt.Errorf("notice_campaign: encode attribute_mapping: %w", err)
		}
		attrMap = string(b)
	}
	if _, err := tx.Exec(ctx, insertNoticeCampaign, string(tx.TenantID()), c.ID, c.Title, c.Channel, nullText(c.OARowID),
		nullText(c.SmsAccountID), nullText(c.TemplateRef), c.TemplateName, nullText(c.TemplateContent), params, c.Audience.Kind,
		audience, c.RecipientCount, c.SkippedCount, nullText(c.UnitPrice), nullText(c.EstimatedCost), c.CreatedAt.UTC(),
		c.CreatedBy, nullText(c.SmsContentMode), c.InvalidCount, mapping, attrMap); err != nil {
		return fmt.Errorf("notice_campaign: insert: %w", err)
	}
	return nil
}

// insertNoticeRecipientsHead is the multi-row INSERT's fixed half. $1 is the commune, $2 the campaign, $3
// the time — shared by every row; each row adds its seven parameters (noticeRecipientValues).
const insertNoticeRecipientsHead = `INSERT INTO notice_recipient (tenant_id, id, campaign_id, recipient_ref,
		recipient_kind, phone_masked, status, phone_sealed, payload_sealed, created_at, updated_at) VALUES `

// noticeRecipientValues renders the VALUES list of n rows: ($1, $k, $2, $k+1, …, $k+6, $3, $3), …
func noticeRecipientValues(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		k := 4 + i*7
		b.WriteString("($1, $" + strconv.Itoa(k) + ", $2")
		for j := 1; j < 7; j++ {
			b.WriteString(", $" + strconv.Itoa(k+j))
		}
		b.WriteString(", $3, $3)")
	}
	return b.String()
}

// InsertRecipients writes every recipient row of one campaign in the transaction's commune, ≤ 1 000 rows
// per statement: 'cho-gui' (no lease) unless the row says 'khong-hop-le' (0033). The ref, the MASKED number
// and — typed rows only — the SEALED number and message; 0032 / 0033's CHECKs refuse anything else.
func (s *NoticeCampaignStore) InsertRecipients(ctx context.Context, tx *store.ScopedTx, campaignID string,
	rs []domain.NoticeRecipient, at time.Time) error {

	for start := 0; start < len(rs); start += maxRecipientsPerInsert {
		chunk := rs[start:min(start+maxRecipientsPerInsert, len(rs))]
		args := make([]any, 0, 3+len(chunk)*7)
		args = append(args, string(tx.TenantID()), campaignID, at.UTC())
		for _, r := range chunk {
			status := r.Status
			if status == "" {
				status = domain.NoticeRecipientPending
			}
			args = append(args, r.ID, r.Ref, r.Kind, r.PhoneMasked, status, nullBytes(r.PhoneSealed), nullBytes(r.PayloadSealed))
		}
		if _, err := tx.Exec(ctx, insertNoticeRecipientsHead+noticeRecipientValues(len(chunk)), args...); err != nil {
			return fmt.Errorf("notice_recipient: insert: %w", err)
		}
	}
	return nil
}

// listNoticeCampaignsHead — the two source tables are constrained to the SAME commune ($1) in their ON
// clauses, as core/store.QueryJoin requires. Newest first, keyset on (created_at, id) — 0032
// notice_campaign_by_time.
const listNoticeCampaignsHead = `SELECT ` + noticeCampaignColumns + `,
		COALESCE(o.oa_name, ''), COALESCE(a.label, ''), COALESCE(a.brandname, '')
	FROM notice_campaign c
	LEFT JOIN zalo_commune_oa o ON o.tenant_id = $1 AND o.id = c.oa_row_id
	LEFT JOIN sms_account a ON a.tenant_id = $1 AND a.id = c.sms_account_id
	WHERE c.tenant_id = $1 `

// List reads one page of the commune's campaigns, newest first, strictly after `after` (zero = first
// page); limit+1 rows are asked so the caller knows whether a next page exists.
func (s *NoticeCampaignStore) List(ctx context.Context, after domain.NoticeCursor, limit int) ([]NoticeCampaignView, error) {
	var (
		rows *sql.Rows
		err  error
	)
	// Scoped.QueryJoin binds the context's commune as $1 (listNoticeCampaignsHead carries tenant_id).
	if after.ID == "" {
		rows, err = s.db.For(ctx).QueryJoin(ctx, listNoticeCampaignsHead+`ORDER BY c.created_at DESC, c.id DESC LIMIT $2`,
			limit+1)
	} else {
		rows, err = s.db.For(ctx).QueryJoin(ctx, listNoticeCampaignsHead+
			`AND (c.created_at, c.id) < ($2, $3) ORDER BY c.created_at DESC, c.id DESC LIMIT $4`,
			after.CreatedAt.UTC(), after.ID, limit+1)
	}
	if err != nil {
		return nil, fmt.Errorf("notice_campaign: list: %w", err)
	}
	return scanNoticeCampaignViews(rows, "notice_campaign: list")
}

// Get reads one campaign of this commune. found=false: no such campaign HERE (another commune's id reads
// the same — rule 1).
func (s *NoticeCampaignStore) Get(ctx context.Context, id string) (NoticeCampaignView, bool, error) {
	// Scoped.QueryJoin binds the context's commune as $1 (listNoticeCampaignsHead carries tenant_id).
	rows, err := s.db.For(ctx).QueryJoin(ctx, listNoticeCampaignsHead+`AND c.id = $2`, id)
	if err != nil {
		return NoticeCampaignView{}, false, fmt.Errorf("notice_campaign: read: %w", err)
	}
	out, err := scanNoticeCampaignViews(rows, "notice_campaign: read")
	if err != nil || len(out) == 0 {
		return NoticeCampaignView{}, false, err
	}
	return out[0], true, nil
}

func scanNoticeCampaignViews(rows *sql.Rows, what string) ([]NoticeCampaignView, error) {
	defer rows.Close()
	var out []NoticeCampaignView
	for rows.Next() {
		var v NoticeCampaignView
		c, err := scanNoticeCampaign(rows.Scan, &v.OAName, &v.AccountLabel, &v.Brandname)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", what, err)
		}
		v.Campaign = c
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return out, nil
}

// campaignsByID — one table, aliased `c` for the shared column list.
const campaignsByID = `SELECT ` + noticeCampaignColumns + ` FROM notice_campaign c
	WHERE c.tenant_id = $1 AND c.id = ANY($2)`

// CampaignsByID reads the named campaigns of this commune — the sender's read for one batch. Missing ids
// are simply absent from the map.
func (s *NoticeCampaignStore) CampaignsByID(ctx context.Context, ids []string) (map[string]domain.NoticeCampaign, error) {
	out := map[string]domain.NoticeCampaign{}
	if len(ids) == 0 {
		return out, nil
	}
	// Scoped.QueryJoin binds the context's commune as $1 (campaignsByID carries tenant_id).
	// pgx binds a []string to a text[] directly (service-identity can_bo_ten.go precedent).
	rows, err := s.db.For(ctx).QueryJoin(ctx, campaignsByID, ids)
	if err != nil {
		return nil, fmt.Errorf("notice_campaign: read batch: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanNoticeCampaign(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("notice_campaign: scan batch: %w", err)
		}
		out[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notice_campaign: read batch: %w", err)
	}
	return out, nil
}

// claimDueNoticeRecipients — the rows still owed whose lease (or back-off) has run out, oldest first,
// SKIP LOCKED so two replicas never take the same row; each counted as one more attempt and leased.
// 0032 notice_recipient_pending serves the WHERE.
const claimDueNoticeRecipients = `WITH due AS (
		SELECT id FROM notice_recipient
		WHERE tenant_id = $1 AND status = 'cho-gui' AND (lease_until IS NULL OR lease_until <= $2)
		ORDER BY created_at, id
		LIMIT $3
		FOR UPDATE SKIP LOCKED)
	UPDATE notice_recipient r SET attempts = r.attempts + 1, lease_until = $4, updated_at = $2
	FROM due
	WHERE r.tenant_id = $1 AND r.id = due.id
	RETURNING r.id, r.campaign_id, r.recipient_ref, r.recipient_kind, r.attempts, r.phone_sealed, r.payload_sealed`

// ClaimDue locks up to limit owed rows of the transaction's commune, counts the attempt and leases each
// until leaseUntil. A pass that dies leaves the lease to run out and the rows are claimed again — at least
// once; the provider's RequestId / tracking_id = row id is the second layer against a double send.
func (s *NoticeCampaignStore) ClaimDue(ctx context.Context, tx *store.ScopedTx, now, leaseUntil time.Time, limit int) (
	[]domain.DueNoticeRecipient, error) {

	if limit <= 0 || limit > MaxNoticeClaim {
		limit = MaxNoticeClaim
	}
	// A CTE + UPDATE … RETURNING is one statement the one-table ScopedTx.Query cannot express; it carries
	// tenant_id = $1 in both halves, bound to the transaction's commune.
	rows, err := tx.Underlying().QueryContext(ctx, claimDueNoticeRecipients, string(tx.TenantID()), now.UTC(), limit,
		leaseUntil.UTC())
	if err != nil {
		return nil, fmt.Errorf("notice_recipient: claim: %w", err)
	}
	defer rows.Close()
	var out []domain.DueNoticeRecipient
	for rows.Next() {
		var d domain.DueNoticeRecipient
		if err := rows.Scan(&d.ID, &d.CampaignID, &d.Ref, &d.Kind, &d.Attempts, &d.PhoneSealed, &d.PayloadSealed); err != nil {
			return nil, fmt.Errorf("notice_recipient: claim scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notice_recipient: claim: %w", err)
	}
	return out, nil
}

// EVERY FINAL OUTCOME EMPTIES THE SEALED PAIR IN THE SAME UPDATE (0033 notice_recipient_sealed_exactly_
// while_owed; ADR 0099 S6) — a no-op on a directory row, which never had them. A retry keeps them.
const (
	markNoticeSent = `UPDATE notice_recipient SET status = 'da-gui', provider_message_id = $3, sent_at = $4,
		lease_until = NULL, failure_class = NULL, phone_sealed = NULL, payload_sealed = NULL, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	retryNotice = `UPDATE notice_recipient SET lease_until = $3, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	failNotice = `UPDATE notice_recipient SET status = 'that-bai', failure_class = $3, error_message = $5,
		lease_until = NULL, phone_sealed = NULL, payload_sealed = NULL, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
	skipNotice = `UPDATE notice_recipient SET status = 'bo-qua', failure_class = $3, lease_until = NULL,
		phone_sealed = NULL, payload_sealed = NULL, updated_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'cho-gui'`
)

// MarkSent ends an owed row as 'da-gui' with the provider's message id.
func (s *NoticeCampaignStore) MarkSent(ctx context.Context, tx *store.ScopedTx, id, messageID string, at time.Time) error {
	return execOne(ctx, tx, markNoticeSent, "notice_recipient: mark sent", ErrNoticeRowChanged,
		string(tx.TenantID()), id, messageID, at.UTC())
}

// Retry keeps an owed row owed, not to be claimed again before next.
func (s *NoticeCampaignStore) Retry(ctx context.Context, tx *store.ScopedTx, id string, next, at time.Time) error {
	return execOne(ctx, tx, retryNotice, "notice_recipient: retry", ErrNoticeRowChanged,
		string(tx.TenantID()), id, next.UTC(), at.UTC())
}

// Fail ends an owed row as 'that-bai' with its class (domain.NoticeFail*) and the provider's message
// ("" = none; the caller passes it through domain.ScrubProviderMessage — 0033 refuses a 7-digit run).
func (s *NoticeCampaignStore) Fail(ctx context.Context, tx *store.ScopedTx, id, class, errorMessage string, at time.Time) error {
	return execOne(ctx, tx, failNotice, "notice_recipient: fail", ErrNoticeRowChanged,
		string(tx.TenantID()), id, class, at.UTC(), nullText(errorMessage))
}

// Skip ends an owed row as 'bo-qua' with its class — deliberately not sent.
func (s *NoticeCampaignStore) Skip(ctx context.Context, tx *store.ScopedTx, id, class string, at time.Time) error {
	return execOne(ctx, tx, skipNotice, "notice_recipient: skip", ErrNoticeRowChanged,
		string(tx.TenantID()), id, class, at.UTC())
}

// settleNoticeCampaign recounts a running campaign from its rows and, when none is owed any more, closes
// it with its terminal status (domain.NoticeCampaignOutcome, spelled in SQL) — in the caller's transaction,
// after the row outcomes. Counts only grow (0032's guard): rows that left 'cho-gui' never return.
// finished_at is never before created_at (0032 notice_campaign_finished_after_created), whatever the
// writing replica's clock says.
const settleNoticeCampaign = `UPDATE notice_campaign c SET sent_count = s.sent, failed_count = s.failed,
		status = CASE WHEN s.pending > 0 THEN 'dang-gui'
		              WHEN s.sent >= c.recipient_count AND c.invalid_count = 0 THEN 'da-gui'
		              WHEN s.sent = 0 THEN 'that-bai'
		              ELSE 'gui-mot-phan' END,
		finished_at = CASE WHEN s.pending > 0 THEN NULL ELSE GREATEST($3::timestamptz, c.created_at) END
	FROM (SELECT count(*) FILTER (WHERE status = 'cho-gui') AS pending,
	             count(*) FILTER (WHERE status = 'da-gui') AS sent,
	             count(*) FILTER (WHERE status = 'that-bai') AS failed
	      FROM notice_recipient WHERE tenant_id = $1 AND campaign_id = $2) s
	WHERE c.tenant_id = $1 AND c.id = $2 AND c.status = 'dang-gui'
	RETURNING c.status`

// Settle recounts one running campaign of the transaction's commune and returns its status afterwards
// ("" when the campaign was not running — already finished, or not here).
func (s *NoticeCampaignStore) Settle(ctx context.Context, tx *store.ScopedTx, campaignID string, at time.Time) (string, error) {
	// An UPDATE … FROM … RETURNING carries tenant_id = $1 on both tables, bound to the transaction's commune.
	var status string
	err := tx.Underlying().QueryRowContext(ctx, settleNoticeCampaign, string(tx.TenantID()), campaignID, at.UTC()).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("notice_campaign: settle: %w", err)
	}
	return status, nil
}

// noticeRecipientViewColumns IS READ BY POSITION in ListRecipients. Never the sealed pair, never the ref.
const noticeRecipientViewColumns = `id, phone_masked, status, COALESCE(failure_class, ''), COALESCE(error_message, ''),
	sent_at, created_at`

// ListRecipients reads one page of one campaign's rows in this commune, by row id, strictly after afterID
// ("" = first page); limit+1 rows are asked so the caller knows whether a next page exists. 0033
// notice_recipient_by_campaign serves it.
func (s *NoticeCampaignStore) ListRecipients(ctx context.Context, campaignID, afterID string, limit int) (
	[]domain.NoticeRecipientView, error) {

	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, noticeRecipientViewColumns, "notice_recipient",
		"AND campaign_id = $2 AND id > $3 ORDER BY id LIMIT $4", campaignID, afterID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("notice_recipient: list: %w", err)
	}
	defer rows.Close()
	out := []domain.NoticeRecipientView{}
	for rows.Next() {
		var (
			v    domain.NoticeRecipientView
			sent sql.NullTime
		)
		if err := rows.Scan(&v.ID, &v.PhoneMasked, &v.Status, &v.FailureClass, &v.ErrorMessage, &sent, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("notice_recipient: list scan: %w", err)
		}
		v.SentAt = sent.Time
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notice_recipient: list: %w", err)
	}
	return out, nil
}

// nullBytes — an empty byte slice is SQL NULL (the sealed pair of a directory or invalid row).
func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// ---- sms_template, for the campaign ----------------------------------------------------------------------

// TemplateOfAccount reads one template row of one account of this commune, VANISHED ROWS INCLUDED — the
// caller tells "no such template" (404) from "eSMS no longer lists it" (not sendable) by VanishedAt.
// found=false: no such row under that account here.
func (s *SmsTemplateStore) TemplateOfAccount(ctx context.Context, accountID, id string) (domain.SmsTemplate, bool, error) {
	// Scoped.Query adds `WHERE tenant_id = $1` and binds the context's commune.
	rows, err := s.db.For(ctx).Query(ctx, smsTemplateColumns, "sms_template", "AND account_id = $2 AND id = $3", accountID, id)
	if err != nil {
		return domain.SmsTemplate{}, false, fmt.Errorf("sms_template: read: %w", err)
	}
	out, err := scanSmsTemplates(rows, "sms_template: read")
	if err != nil || len(out) == 0 {
		return domain.SmsTemplate{}, false, err
	}
	return out[0], true, nil
}
