package app

// THE NOTICE CAMPAIGN SENDER — sends the owed `notice_recipient` rows (migration 0032; ADR 0099 #8). The
// campaign was written with every recipient row in one transaction (notice_campaigns.go); this job sends
// them.
//
// THE SHAPE IS THE ZALO DISPATCHER'S (zalo_dispatcher.go): an in-process ticker, ONE advisory lock per
// tick so one replica sends, the communes with owed rows listed as identifiers by
// internal/store/crosstenant, each commune IN ITS OWN CONTEXT behind its own recover, a merged commune left
// untouched (rule 7, invariant 6).
//
// PER COMMUNE, THREE STEPS, AND THE NETWORK IS NEVER INSIDE A TRANSACTION:
//
//  1. one transaction: claim ≤ 500 owed rows (SKIP LOCKED), each counted as one attempt and leased
//  2. outside any transaction, per campaign of the batch: the source checked and opened ONCE (the OA's
//     access token / the account's keys), the full numbers asked of identity ONCE per recipient kind
//     (RevealNoticeRecipientPhones — load-data-once, never per person), then one provider call per row
//     with RequestId / tracking_id = the row id
//  3. one transaction: every row's outcome, every touched campaign recounted (and closed when no row is
//     owed), ONE audit entry by the system principal (rule 6, invariant 6)
//
// A pass that dies between 1 and 3 leaves its rows leased; the lease runs out and they are claimed again —
// at least once. The provider's de-duplication on the row id (eSMS refuses a repeated RequestId for 24
// hours — answered 124, recorded as accepted) is the second layer.
//
// THE FULL NUMBERS (rule 3; ADR 0099 #5): held in ONE local map per (campaign, kind) for the length of the
// provider calls, cleared right after, never logged, never stored, never put in an error, an audit delta or
// a log line. The same for the OA token and the eSMS keys. Log lines carry the commune, counts and error
// classes — nothing else.
//
// TYPED / IMPORTED ROWS (recipient_kind 'so-nhap', migration 0033; ADR 0099 S6): identity is NOT asked —
// the row's own sealed number and message are opened (ONE OpenMany per campaign of the batch, the commune's
// DEK read once), used for the one provider call, wiped; the outcome UPDATE empties both sealed columns.
// A refused send records the provider's message, scrubbed (domain.ScrubProviderMessage — no 7-digit run).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalooa"
)

const (
	// NoticeSendTickInterval — the card's 15 seconds. A tick with nothing owed is one identifiers-only
	// query.
	NoticeSendTickInterval = 15 * time.Second
	// noticeSendTickBudget bounds one tick and so the scheduler lock's hold. 500 provider calls at well
	// under a second each fit; a slower provider leaves the rest to the next tick (see sendBatch).
	noticeSendTickBudget = 10 * time.Minute
	// noticeSendLease outlives one tick's budget, so a row being sent is never claimed by the next tick.
	noticeSendLease = 15 * time.Minute
	// noticeRecordTimeout bounds step 3, on a context the tick's time budget cannot cut: the sends already
	// happened, and their record must still land.
	noticeRecordTimeout = 30 * time.Second
)

// NoticeSendLocks is *crosstenant.NoticeSend.
type NoticeSendLocks interface {
	CommunesWithDueNoticeRecipients(ctx context.Context, now time.Time) ([]tenant.ID, error)
	TryLockNoticeSendScheduler(ctx context.Context) (func(), bool, error)
}

// NoticeSendRepo is the campaign store's sender half. *docstore.NoticeCampaignStore satisfies it; every
// method is scoped to the context's / transaction's commune (tenant_id = $1).
type NoticeSendRepo interface {
	ClaimDue(ctx context.Context, tx *store.ScopedTx, now, leaseUntil time.Time, limit int) ([]domain.DueNoticeRecipient, error)
	CampaignsByID(ctx context.Context, ids []string) (map[string]domain.NoticeCampaign, error)
	MarkSent(ctx context.Context, tx *store.ScopedTx, id, messageID string, at time.Time) error
	Retry(ctx context.Context, tx *store.ScopedTx, id string, next, at time.Time) error
	Fail(ctx context.Context, tx *store.ScopedTx, id, class, errorMessage string, at time.Time) error
	Skip(ctx context.Context, tx *store.ScopedTx, id, class string, at time.Time) error
	Settle(ctx context.Context, tx *store.ScopedTx, campaignID string, at time.Time) (string, error)
}

// NoticePhoneRevealer is identity's reveal (ADR 0099 #5). *identityclient.Client satisfies it.
type NoticePhoneRevealer interface {
	RevealNoticeRecipientPhones(ctx context.Context, kind identityclient.NoticeRecipientKind, refs []string,
		campaignRef, actorCode string) (map[string]string, error)
}

// NoticeOASender is what the sender needs of the commune's OAs. *CommuneZaloOAs satisfies it.
type NoticeOASender interface {
	TemplateForSend(ctx context.Context, oaRowID, templateID string) (domain.ZaloOATemplate, error)
	OpenAccessToken(ctx context.Context, oaRowID string) (secret.Secret, string, error)
}

// NoticeSmsOpener opens an account's keys. *SmsAccounts satisfies it.
type NoticeSmsOpener interface {
	OpenForSend(ctx context.Context, accountID string) (domain.SmsAccount, esms.Credentials, error)
}

// ZnsMessenger sends one ZBS template message; the third result is Zalo's own message on a refusal (never
// logged). *zalooa.Client satisfies it.
type ZnsMessenger interface {
	SendTemplateMessageReport(ctx context.Context, accessToken secret.Secret, phone, templateID string,
		templateData map[string]string, trackingID string) (string, zalooa.Outcome, string)
}

// SmsMessenger sends one CSKH SMS; the third result is eSMS's ErrorMessage on a refusal (never logged).
// *esms.Client satisfies it.
type SmsMessenger interface {
	SendCSKHReport(ctx context.Context, creds esms.Credentials, m esms.Message) (string, esms.Outcome, string)
}

// NoticeOpener opens typed rows' sealed numbers and messages (ADR 0099 S6). The commune envelope
// (*crypto.Envelope) satisfies it; a nil one answers crypto.ErrNotConfigured and the rows stay owed.
type NoticeOpener interface {
	OpenMany(ctx context.Context, sealed, aads [][]byte) ([]secret.Secret, error)
}

// NoticeSenderDeps wires the sender. Registry tells a merged commune (left untouched) from an active one.
type NoticeSenderDeps struct {
	DB       *store.DB
	Repo     NoticeSendRepo
	Locks    NoticeSendLocks
	Registry PortalCommuneRegistry
	Phones   NoticePhoneRevealer
	OAs      NoticeOASender
	Accounts NoticeSmsOpener
	Zns      ZnsMessenger
	Sms      SmsMessenger
	Opener   NoticeOpener
	Log      *slog.Logger
}

// NoticeSender sends owed campaign messages. Run blocks until ctx is done.
type NoticeSender struct {
	d          NoticeSenderDeps
	interval   time.Duration
	tickBudget time.Duration
	now        func() time.Time
}

// NewNoticeSender builds the sender.
func NewNoticeSender(d NoticeSenderDeps) (*NoticeSender, error) {
	if d.DB == nil || d.Repo == nil || d.Locks == nil || d.Registry == nil || d.Phones == nil || d.OAs == nil ||
		d.Accounts == nil || d.Zns == nil || d.Sms == nil || d.Opener == nil {
		return nil, errors.New("gui_dot_tin: việc nền thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &NoticeSender{d: d, interval: NoticeSendTickInterval, tickBudget: noticeSendTickBudget,
		now: func() time.Time { return time.Now().UTC() }}, nil
}

// Run ticks until ctx is cancelled — once at start, then every interval.
func (s *NoticeSender) Run(ctx context.Context) {
	s.Tick(ctx)
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

// Tick does one pass over every commune with owed rows.
func (s *NoticeSender) Tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	release, ok, err := s.d.Locks.TryLockNoticeSendScheduler(ctx)
	if err != nil {
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá việc nền gửi đợt tin, bỏ nhịp này", "service", "comms", "err", err)
		return
	}
	if !ok {
		return // another replica sends
	}
	defer release()
	tctx, cancel := context.WithTimeout(ctx, s.tickBudget)
	defer cancel()

	ids, err := s.d.Locks.CommunesWithDueNoticeRecipients(tctx, s.now())
	if err != nil {
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã có đợt tin chờ gửi", "service", "comms", "err", err)
		return
	}
	for i, id := range ids {
		if tctx.Err() != nil {
			s.d.Log.WarnContext(ctx, "CẢNH BÁO: nhịp gửi đợt tin hết thời gian, các xã còn lại chờ nhịp sau",
				"service", "comms", "con_lai", len(ids)-i)
			return
		}
		s.sendCommune(tctx, id)
	}
}

// noticeResult is one row's outcome of this pass.
type noticeResult struct {
	kind      string // the domain.NoticeRecipient* status it moves to; NoticeRecipientPending = retry
	class     string // failure class (fail / skip), or the retryable fault's class (retry)
	messageID string // sent only
	errorText string // the provider's message, SCRUBBED (domain.ScrubProviderMessage); fail only
}

// withProviderText attaches a provider's refusal text, scrubbed, to a failure (nothing else keeps it).
func withProviderText(r noticeResult, text string) noticeResult {
	if r.kind == domain.NoticeRecipientFailed || r.kind == domain.NoticeRecipientPending {
		r.errorText = domain.ScrubProviderMessage(text)
	}
	return r
}

func sentResult(id string) noticeResult {
	return noticeResult{kind: domain.NoticeRecipientSent, messageID: id}
}
func failResult(class string) noticeResult {
	return noticeResult{kind: domain.NoticeRecipientFailed, class: class}
}
func retryResult(class string) noticeResult {
	return noticeResult{kind: domain.NoticeRecipientPending, class: class}
}
func skipResult(class string) noticeResult {
	return noticeResult{kind: domain.NoticeRecipientSkipped, class: class}
}

// sendCommune runs the three steps for one commune. Nothing it does can stop the next commune.
func (s *NoticeSender) sendCommune(ctx context.Context, id tenant.ID) {
	defer func() {
		if p := recover(); p != nil {
			s.d.Log.ErrorContext(ctx, "LỖI: gửi đợt tin của một xã bị panic, chuyển sang xã kế", "service", "comms",
				"xa", string(id), "panic", fmt.Sprint(p))
		}
	}()
	cctx := tenant.Into(ctx, id)
	t, ok, err := s.d.Registry.XaTrongNguCanh(cctx)
	if err != nil {
		s.d.Log.WarnContext(cctx, "CẢNH BÁO: không đọc được trạng thái xã, bỏ xã này ở nhịp này", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if !ok || !t.Active {
		return // a merged or unknown commune keeps its rows unchanged (rule 7, invariant 6)
	}

	// 1. claim.
	now := s.now().Truncate(time.Microsecond)
	var due []domain.DueNoticeRecipient
	err = s.d.DB.For(cctx).Tx(cctx, func(tx *store.ScopedTx) (err error) {
		due, err = s.d.Repo.ClaimDue(cctx, tx, now, now.Add(noticeSendLease), docstore.MaxNoticeClaim)
		return err
	})
	if err != nil {
		s.d.Log.WarnContext(cctx, "CẢNH BÁO: không nhận được lô đợt tin của xã — thử lại nhịp sau", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if len(due) == 0 {
		return
	}

	// 2. send, outside any transaction.
	results := s.sendBatch(cctx, due)

	// 3. record — on a context the tick's time budget cannot cut (the sends happened; their record must land).
	rctx, cancel := context.WithTimeout(context.WithoutCancel(cctx), noticeRecordTimeout)
	defer cancel()
	if err := s.record(rctx, due, results); err != nil {
		// The sends happened; the record did not land. The leases run out and those rows are claimed again:
		// eSMS answers 124 for a repeated RequestId (recorded as accepted); Zalo is not known to de-duplicate
		// on tracking_id, so a ZNS message may arrive twice — at least once, never silently lost.
		s.d.Log.WarnContext(cctx, "CẢNH BÁO: không ghi được kết quả gửi đợt tin — các dòng sẽ được gửi lại sau hạn giữ",
			"service", "comms", "xa", string(id), "so_dong", len(due), "err", err)
	}
}

// sendBatch decides every claimed row: per campaign, the source opened once, the numbers revealed once per
// kind, then one provider call per row. A row the pass did not reach (the tick ran out) is a retry.
func (s *NoticeSender) sendBatch(ctx context.Context, due []domain.DueNoticeRecipient) map[string]noticeResult {
	results := make(map[string]noticeResult, len(due))
	order := []string{}
	byCampaign := map[string][]domain.DueNoticeRecipient{}
	for _, d := range due {
		if _, ok := byCampaign[d.CampaignID]; !ok {
			order = append(order, d.CampaignID)
		}
		byCampaign[d.CampaignID] = append(byCampaign[d.CampaignID], d)
	}
	campaigns, err := s.d.Repo.CampaignsByID(ctx, order)
	if err != nil {
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: không đọc được đợt tin của lô — thử lại sau", "service", "comms", "err", err)
		for _, d := range due {
			results[d.ID] = retryResult(domain.NoticeFailUnavailable)
		}
		return results
	}
	for _, cid := range order {
		rows := byCampaign[cid]
		c, ok := campaigns[cid]
		if !ok || c.Finished() {
			// A row owed to a campaign that is gone or closed cannot be: the campaign closes only when no row
			// is owed. Ended rather than retried for ever.
			setAll(results, rows, failResult(domain.NoticeFailRejected))
			continue
		}
		var typed, directory []domain.DueNoticeRecipient
		for _, d := range rows {
			if d.Kind == domain.NoticeRecipientTyped {
				typed = append(typed, d)
			} else {
				directory = append(directory, d)
			}
		}
		if len(directory) > 0 {
			if c.Channel == domain.NoticeChannelZNS {
				s.sendZNS(ctx, c, directory, results)
			} else {
				s.sendSMS(ctx, c, directory, results)
			}
		}
		if len(typed) > 0 {
			s.sendTyped(ctx, c, typed, results)
		}
	}
	return results
}

func setAll(results map[string]noticeResult, rows []domain.DueNoticeRecipient, r noticeResult) {
	for _, d := range rows {
		results[d.ID] = r
	}
}

// byKind splits one campaign's rows by recipient kind — one reveal per kind.
func byKind(rows []domain.DueNoticeRecipient) map[string][]domain.DueNoticeRecipient {
	out := map[string][]domain.DueNoticeRecipient{}
	for _, d := range rows {
		out[d.Kind] = append(out[d.Kind], d)
	}
	return out
}

// reveal asks identity for the full numbers of one kind's rows ONCE. ok=false: identity did not answer —
// every row is a retry (transaction-boundaries gui_dot_tin_zns_sms (b)), NEVER bo-qua.
func (s *NoticeSender) reveal(ctx context.Context, c domain.NoticeCampaign, kind string, rows []domain.DueNoticeRecipient,
	results map[string]noticeResult) (map[string]string, bool) {

	ik := identityRecipientKind(kind)
	if ik == 0 {
		setAll(results, rows, failResult(domain.NoticeFailRejected))
		return nil, false
	}
	refs := make([]string, 0, len(rows))
	for _, d := range rows {
		refs = append(refs, d.Ref)
	}
	phones, err := s.d.Phones.RevealNoticeRecipientPhones(ctx, ik, refs, c.ID, c.CreatedBy)
	if err != nil {
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: identity không mở được số cho lô đợt tin — giữ chờ gửi", "service", "comms",
			"xa", string(tenant.MustFrom(ctx)), "dot", c.ID, "so_dong", len(rows), "err", err)
		setAll(results, rows, retryResult(domain.NoticeFailUnavailable))
		return nil, false
	}
	return phones, true
}

func (s *NoticeSender) sendZNS(ctx context.Context, c domain.NoticeCampaign, rows []domain.DueNoticeRecipient,
	results map[string]noticeResult) {

	// The source BEFORE any number is revealed: a campaign that cannot go out discloses nothing.
	if _, err := s.d.OAs.TemplateForSend(ctx, c.OARowID, c.TemplateRef); err != nil {
		setAll(results, rows, oaSourceResult(err))
		return
	}
	token, _, err := s.d.OAs.OpenAccessToken(ctx, c.OARowID)
	if err != nil {
		setAll(results, rows, oaSourceResult(err))
		return
	}
	defer clear(token)
	for kind, part := range byKind(rows) {
		phones, ok := s.reveal(ctx, c, kind, part, results)
		if !ok {
			continue
		}
		for _, d := range part {
			phone, deliverable := phones[d.Ref]
			switch {
			case !deliverable:
				results[d.ID] = skipResult(domain.NoticeFailRecipientInactive)
			case ctx.Err() != nil:
				results[d.ID] = retryResult(domain.NoticeFailUnavailable)
			default:
				msgID, o, text := s.d.Zns.SendTemplateMessageReport(ctx, token, phone, c.TemplateRef, c.Params, d.ID)
				results[d.ID] = withProviderText(znsResult(msgID, o), text)
			}
		}
		clear(phones)
	}
}

func (s *NoticeSender) sendSMS(ctx context.Context, c domain.NoticeCampaign, rows []domain.DueNoticeRecipient,
	results map[string]noticeResult) {

	values, err := domain.SmsSlotValues(c.TemplateContent, c.Params)
	if err != nil {
		setAll(results, rows, failResult(domain.NoticeFailTemplateRejected))
		return
	}
	content, err := domain.FillSmsTemplate(c.TemplateContent, values)
	if err != nil {
		setAll(results, rows, failResult(domain.NoticeFailTemplateRejected))
		return
	}
	account, creds, err := s.d.Accounts.OpenForSend(ctx, c.SmsAccountID)
	switch {
	case errors.Is(err, ErrSmsAccountNotFound):
		setAll(results, rows, failResult(domain.NoticeFailSourceRetired))
		return
	case err != nil:
		// No key in this deployment, a broken seal, the database: a fault of OURS — retried, and after the
		// last attempt that-bai khong-kha-dung.
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: không mở được tài khoản SMS cho đợt tin — giữ chờ gửi", "service", "comms",
			"xa", string(tenant.MustFrom(ctx)), "dot", c.ID, "err", err)
		setAll(results, rows, retryResult(domain.NoticeFailUnavailable))
		return
	}
	defer clear(creds.APIKey)
	defer clear(creds.SecretKey)
	unicode := domain.SmsContentIsUnicode(content)
	for kind, part := range byKind(rows) {
		phones, ok := s.reveal(ctx, c, kind, part, results)
		if !ok {
			continue
		}
		for _, d := range part {
			phone, deliverable := phones[d.Ref]
			switch {
			case !deliverable:
				results[d.ID] = skipResult(domain.NoticeFailRecipientInactive)
			case ctx.Err() != nil:
				results[d.ID] = retryResult(domain.NoticeFailUnavailable)
			default:
				msgID, o, text := s.d.Sms.SendCSKHReport(ctx, creds, esms.Message{Phone: phone, Content: content,
					Brandname: account.Brandname, IsUnicode: unicode, RequestID: d.ID, CampaignID: c.ID})
				results[d.ID] = withProviderText(smsResult(d.ID, msgID, o), text)
			}
		}
		clear(phones)
	}
}

// oaSourceResult maps an OA that cannot send onto every row of the campaign.
func oaSourceResult(err error) noticeResult {
	switch {
	case errors.Is(err, ErrCommuneZaloOAMissing):
		return failResult(domain.NoticeFailSourceRetired)
	case errors.Is(err, ErrZaloOANeedsReauthorization):
		return failResult(domain.NoticeFailOANeedsReauth)
	case errors.Is(err, ErrZaloOATemplateMissing), errors.Is(err, ErrZaloOATemplateNotSendable):
		return failResult(domain.NoticeFailTemplateRejected)
	}
	// Zalo unavailable during a due refresh, no key in this deployment, the database: retried.
	return retryResult(domain.NoticeFailUnavailable)
}

// znsResult maps zalooa's class onto the row (the failure-class table lives here, not in domain, because
// domain imports nothing but the standard library).
func znsResult(msgID string, o zalooa.Outcome) noticeResult {
	switch o {
	case zalooa.OutcomeOK:
		return sentResult(msgID)
	case zalooa.OutcomeTokenRejected:
		return failResult(domain.NoticeFailOANeedsReauth)
	case zalooa.OutcomeRateLimited:
		return retryResult(domain.NoticeFailRateLimited)
	case zalooa.OutcomeUnavailable:
		return retryResult(domain.NoticeFailUnavailable)
	case zalooa.OutcomeRecipientRejected:
		return failResult(domain.NoticeFailInvalidNumber)
	case zalooa.OutcomeMalformedResponse:
		return failResult(domain.NoticeFailMalformed)
	}
	return failResult(domain.NoticeFailRejected)
}

// smsResult maps internal/esms's class onto the row. 124 (RequestId already held) is ACCEPTED — the earlier
// ambiguous attempt reached eSMS — and carries no SMSID, so the row records DuplicateRequestMessageID.
func smsResult(rowID, msgID string, o esms.Outcome) noticeResult {
	switch o {
	case esms.OutcomeOK:
		return sentResult(msgID)
	case esms.OutcomeDuplicateRequest:
		return sentResult(domain.DuplicateRequestMessageID(rowID))
	case esms.OutcomeKeyRejected:
		return failResult(domain.NoticeFailKeyRejected)
	case esms.OutcomeBrandnameInactive:
		return failResult(domain.NoticeFailBrandnameInactive)
	case esms.OutcomeTemplateMismatch:
		return failResult(domain.NoticeFailTemplateRejected)
	case esms.OutcomeUnavailable:
		return retryResult(domain.NoticeFailUnavailable)
	case esms.OutcomeMalformedResponse:
		return failResult(domain.NoticeFailMalformed)
	}
	return failResult(domain.NoticeFailRejected)
}

// record writes every row's outcome, recounts every touched campaign, and writes ONE system entry — one
// transaction. A retryable fault on a row's last attempt ends it that-bai with that fault's class.
func (s *NoticeSender) record(ctx context.Context, due []domain.DueNoticeRecipient, results map[string]noticeResult) error {
	done := s.now().Truncate(time.Microsecond)
	return s.d.DB.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		tally := map[string][]string{}
		touched := []string{}
		seen := map[string]bool{}
		for _, d := range due {
			r, ok := results[d.ID]
			if !ok {
				r = retryResult(domain.NoticeFailUnavailable)
			}
			var err error
			label := r.kind
			switch r.kind {
			case domain.NoticeRecipientSent:
				err = s.d.Repo.MarkSent(ctx, tx, d.ID, r.messageID, done)
			case domain.NoticeRecipientSkipped:
				err = s.d.Repo.Skip(ctx, tx, d.ID, r.class, done)
				label += ":" + r.class
			case domain.NoticeRecipientPending:
				if d.Attempts >= domain.MaxNoticeSendAttempts {
					err = s.d.Repo.Fail(ctx, tx, d.ID, r.class, r.errorText, done)
					label = domain.NoticeRecipientFailed + ":" + r.class
				} else {
					err = s.d.Repo.Retry(ctx, tx, d.ID, done.Add(domain.NoticeRetryBackoff(d.Attempts)), done)
					label = "thu_lai:" + r.class
				}
			default:
				err = s.d.Repo.Fail(ctx, tx, d.ID, r.class, r.errorText, done)
				label += ":" + r.class
			}
			if errors.Is(err, docstore.ErrNoticeRowChanged) {
				continue // settled by another pass after the lease ran out
			}
			if err != nil {
				return err
			}
			tally[label] = append(tally[label], d.ID)
			if !seen[d.CampaignID] {
				seen[d.CampaignID] = true
				touched = append(touched, d.CampaignID)
			}
		}
		finished := map[string]string{}
		for _, cid := range touched {
			status, err := s.d.Repo.Settle(ctx, tx, cid, done)
			if err != nil {
				return err
			}
			if status != "" && status != domain.NoticeCampaignSending {
				finished[cid] = status
			}
		}
		return writeNoticeBatchAudit(ctx, tx, tally, finished, done)
	})
}

// writeNoticeBatchAudit is one entry per batch, by the system principal (rule 6, invariant 6): the row ids
// under each outcome and the campaigns the batch closed. Never a number, a ref, a content or a key.
func writeNoticeBatchAudit(ctx context.Context, tx *store.ScopedTx, tally map[string][]string, finished map[string]string,
	at time.Time) error {

	if len(tally) == 0 {
		return nil
	}
	delta, err := json.Marshal(map[string]any{"ket_qua": tally, "dot_ket_thuc": finished})
	if err != nil {
		return fmt.Errorf("gui_dot_tin: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: systemPrincipal, Action: domain.ActionNoticeSendBatch,
		Subject: "notice-recipient/" + at.Format("2006-01-02"), At: at, Delta: delta})
}
