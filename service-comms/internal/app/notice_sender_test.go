package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/zalooa"
)

// The campaign sender over fakes. What is proved: one reveal per (campaign, kind) — never per person; the
// source is checked BEFORE any number is revealed; each outcome class lands on the right row status; a
// retryable fault backs off and ends that-bai on the last attempt; identity down keeps rows owed; one
// system entry per batch; and the full number reaches nothing but the provider call.

var (
	_ NoticeSendRepo      = (*docstore.NoticeCampaignStore)(nil)
	_ NoticeSendLocks     = (*crosstenant.NoticeSend)(nil)
	_ NoticePhoneRevealer = (*identityclient.Client)(nil)
	_ NoticeOASender      = (*CommuneZaloOAs)(nil)
	_ NoticeSmsOpener     = (*SmsAccounts)(nil)
	_ ZnsMessenger        = (*zalooa.Client)(nil)
	_ SmsMessenger        = (*esms.Client)(nil)
)

const senderCampaign = "01JSENDCAMP000000000000001"

// The agreed fake numbers (rule 3, invariant 5).
const (
	senderPhone1 = "0900000000"
	senderPhone2 = "0900000001"
)

type senderLocks struct {
	communes []tenant.ID
	locked   bool
}

func (l *senderLocks) CommunesWithDueNoticeRecipients(context.Context, time.Time) ([]tenant.ID, error) {
	return l.communes, nil
}

func (l *senderLocks) TryLockNoticeSendScheduler(context.Context) (func(), bool, error) {
	return func() {}, !l.locked, nil
}

type senderRepo struct {
	due      []domain.DueNoticeRecipient
	campaign domain.NoticeCampaign
	claimed  int
	calls    []string // "op:id:arg"
	settled  []string
	status   string
}

func (r *senderRepo) ClaimDue(_ context.Context, tx *store.ScopedTx, now, lease time.Time, limit int) ([]domain.DueNoticeRecipient, error) {
	if tx.TenantID() != xaA || limit != docstore.MaxNoticeClaim || !lease.After(now) {
		return nil, errors.New("bad claim")
	}
	r.claimed++
	return r.due, nil
}

func (r *senderRepo) CampaignsByID(_ context.Context, ids []string) (map[string]domain.NoticeCampaign, error) {
	out := map[string]domain.NoticeCampaign{}
	for _, id := range ids {
		if id == r.campaign.ID {
			out[id] = r.campaign
		}
	}
	return out, nil
}

func (r *senderRepo) MarkSent(_ context.Context, _ *store.ScopedTx, id, msg string, _ time.Time) error {
	r.calls = append(r.calls, "sent:"+id+":"+msg)
	return nil
}

func (r *senderRepo) Retry(_ context.Context, _ *store.ScopedTx, id string, next, at time.Time) error {
	r.calls = append(r.calls, "retry:"+id+":"+next.Sub(at).String())
	return nil
}

func (r *senderRepo) Fail(_ context.Context, _ *store.ScopedTx, id, class string, _ time.Time) error {
	r.calls = append(r.calls, "fail:"+id+":"+class)
	return nil
}

func (r *senderRepo) Skip(_ context.Context, _ *store.ScopedTx, id, class string, _ time.Time) error {
	r.calls = append(r.calls, "skip:"+id+":"+class)
	return nil
}

func (r *senderRepo) Settle(_ context.Context, _ *store.ScopedTx, id string, _ time.Time) (string, error) {
	r.settled = append(r.settled, id)
	return r.status, nil
}

type senderPhones struct {
	phones map[string]string
	err    error
	calls  []string // "kind|refs|campaign|actor"
}

func (p *senderPhones) RevealNoticeRecipientPhones(_ context.Context, kind identityclient.NoticeRecipientKind, refs []string,
	campaignRef, actorCode string) (map[string]string, error) {
	p.calls = append(p.calls, fmt.Sprintf("%d|%s|%s|%s", kind, strings.Join(refs, ","), campaignRef, actorCode))
	if p.err != nil {
		return nil, p.err
	}
	out := map[string]string{}
	for _, r := range refs {
		if v, ok := p.phones[r]; ok {
			out[r] = v
		}
	}
	return out, nil
}

type senderOAs struct{ templateErr, tokenErr error }

func (o *senderOAs) TemplateForSend(context.Context, string, string) (domain.ZaloOATemplate, error) {
	return domain.ZaloOATemplate{}, o.templateErr
}

func (o *senderOAs) OpenAccessToken(context.Context, string) (secret.Secret, string, error) {
	if o.tokenErr != nil {
		return nil, "", o.tokenErr
	}
	return secret.Secret("FAKE-access-token-NOT-REAL"), "4321", nil
}

type senderAccounts struct{ err error }

func (a *senderAccounts) OpenForSend(context.Context, string) (domain.SmsAccount, esms.Credentials, error) {
	if a.err != nil {
		return domain.SmsAccount{}, esms.Credentials{}, a.err
	}
	return noticeAccount(), esms.Credentials{APIKey: secret.Secret("FAKE-api-NOT-REAL"), SecretKey: secret.Secret("FAKE-secret-NOT-REAL")}, nil
}

type senderZns struct {
	outcomes map[string]zalooa.Outcome // by phone
	sent     []string                  // "phone|template|tracking|param"
}

func (z *senderZns) SendTemplateMessage(_ context.Context, _ secret.Secret, phone, tpl string, data map[string]string, tracking string) (string, zalooa.Outcome) {
	z.sent = append(z.sent, phone+"|"+tpl+"|"+tracking+"|"+data["su_kien"])
	if o, ok := z.outcomes[phone]; ok {
		return "", o
	}
	return "zmsg-" + tracking, zalooa.OutcomeOK
}

type senderSms struct {
	outcomes map[string]esms.Outcome
	messages []esms.Message
}

func (s *senderSms) SendCSKH(_ context.Context, _ esms.Credentials, m esms.Message) (string, esms.Outcome) {
	s.messages = append(s.messages, m)
	if o, ok := s.outcomes[m.Phone]; ok {
		return "", o
	}
	return "SMS-" + m.RequestID, esms.OutcomeOK
}

type senderRegistry struct{ inactive bool }

func (g senderRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) { // vi-name-ok: the method of the existing PortalCommuneRegistry interface
	return tenant.Tenant{ID: tenant.MustFrom(ctx), Active: !g.inactive}, true, nil
}

type senderRig struct {
	f        *sqlFake
	locks    *senderLocks
	repo     *senderRepo
	phones   *senderPhones
	oas      *senderOAs
	accounts *senderAccounts
	zns      *senderZns
	sms      *senderSms
	registry senderRegistry
	logs     *bytes.Buffer
}

func newSenderRig(channel string) *senderRig {
	c := domain.NoticeCampaign{ID: senderCampaign, Channel: channel, Status: domain.NoticeCampaignSending, CreatedBy: noticeActor,
		RecipientCount: 3}
	if channel == domain.NoticeChannelZNS {
		c.OARowID, c.TemplateRef, c.Params = noticeOAID, "1001", map[string]string{"su_kien": "Hop dan"}
	} else {
		c.SmsAccountID, c.TemplateRef, c.TemplateContent = noticeAccID, noticeTplRow, noticeSmsTpl
		c.Params = map[string]string{"1": "Mau", "2": "8h"}
	}
	return &senderRig{
		f:     &sqlFake{},
		locks: &senderLocks{communes: []tenant.ID{xaA}},
		repo: &senderRepo{campaign: c, status: domain.NoticeCampaignSending, due: []domain.DueNoticeRecipient{
			{ID: "R1", CampaignID: senderCampaign, Ref: "C1", Kind: domain.NoticeRecipientCitizen, Attempts: 1},
			{ID: "R2", CampaignID: senderCampaign, Ref: "C2", Kind: domain.NoticeRecipientCitizen, Attempts: 1},
			{ID: "R3", CampaignID: senderCampaign, Ref: "C3", Kind: domain.NoticeRecipientCitizen, Attempts: 1},
		}},
		phones:   &senderPhones{phones: map[string]string{"C1": senderPhone1, "C2": senderPhone2}},
		oas:      &senderOAs{},
		accounts: &senderAccounts{},
		zns:      &senderZns{},
		sms:      &senderSms{},
		logs:     &bytes.Buffer{},
	}
}

func (r *senderRig) tick(t *testing.T) {
	t.Helper()
	s, err := NewNoticeSender(NoticeSenderDeps{DB: store.New(sql.OpenDB(r.f)), Repo: r.repo, Locks: r.locks, Registry: r.registry,
		Phones: r.phones, OAs: r.oas, Accounts: r.accounts, Zns: r.zns, Sms: r.sms,
		Log: slog.New(slog.NewTextHandler(r.logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return noticeClock }
	s.Tick(context.Background())
}

func (r *senderRig) assertNoPhoneEscaped(t *testing.T) {
	t.Helper()
	for _, p := range []string{senderPhone1, senderPhone2, "84900000000"} {
		if strings.Contains(r.logs.String(), p) {
			t.Fatalf("a full number reached the log: %s", r.logs.String())
		}
		for _, st := range r.f.stmts {
			if strings.Contains(fmt.Sprint(st.args), p) {
				t.Fatalf("a full number reached the database: %s %v", st.sql, st.args)
			}
		}
		if strings.Contains(strings.Join(r.repo.calls, " "), p) {
			t.Fatalf("a full number reached the store: %v", r.repo.calls)
		}
	}
}

func TestNoticeSenderSmsBatch(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelSMS)
	r.sms.outcomes = map[string]esms.Outcome{senderPhone2: esms.OutcomeUnavailable}
	r.tick(t)

	// ONE reveal for the whole batch, with the campaign and the sender's staff code (load-data-once).
	if len(r.phones.calls) != 1 || r.phones.calls[0] != fmt.Sprintf("%d|C1,C2,C3|%s|%s",
		identityclient.NoticeRecipientCitizen, senderCampaign, noticeActor) {
		t.Fatalf("reveals = %v", r.phones.calls)
	}
	if len(r.sms.messages) != 2 {
		t.Fatalf("%d SMS sent", len(r.sms.messages))
	}
	m := r.sms.messages[0]
	if m.Phone != senderPhone1 || m.Content != "Xa Mau moi ba con hop luc 8h" || m.Brandname != "XaMau" || m.RequestID != "R1" ||
		m.CampaignID != senderCampaign {
		t.Fatalf("message = %+v", m)
	}
	want := []string{"sent:R1:SMS-R1", "retry:R2:1m0s", "skip:R3:" + domain.NoticeFailRecipientInactive}
	if strings.Join(r.repo.calls, " ") != strings.Join(want, " ") {
		t.Fatalf("outcomes = %v, want %v", r.repo.calls, want)
	}
	if len(r.repo.settled) != 1 || r.repo.settled[0] != senderCampaign {
		t.Fatalf("settled = %v", r.repo.settled)
	}
	aud := r.f.with("INSERT INTO audit_log")
	if len(aud) != 1 || !strings.Contains(fmt.Sprint(aud[0].args), domain.ActionNoticeSendBatch) ||
		!strings.Contains(fmt.Sprint(aud[0].args), "system") {
		t.Fatalf("audit = %v", aud)
	}
	// Two transactions: the claim, then the record.
	if r.f.begun != 2 || r.f.commits != 2 {
		t.Fatalf("transactions: %d / %d", r.f.begun, r.f.commits)
	}
	r.assertNoPhoneEscaped(t)
}

func TestNoticeSenderLastAttemptEndsFailed(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelSMS)
	for i := range r.repo.due {
		r.repo.due[i].Attempts = domain.MaxNoticeSendAttempts
	}
	r.sms.outcomes = map[string]esms.Outcome{senderPhone1: esms.OutcomeUnavailable, senderPhone2: esms.OutcomeDuplicateRequest}
	r.tick(t)
	want := []string{"fail:R1:" + domain.NoticeFailUnavailable, "sent:R2:request:R2", "skip:R3:" + domain.NoticeFailRecipientInactive}
	if strings.Join(r.repo.calls, " ") != strings.Join(want, " ") {
		t.Fatalf("outcomes = %v", r.repo.calls)
	}
}

func TestNoticeSenderIdentityDownKeepsRowsOwed(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelSMS)
	r.phones.err = identityclient.ErrIdentityUnavailable
	r.tick(t)
	if len(r.sms.messages) != 0 {
		t.Fatal("sent without numbers")
	}
	for _, c := range r.repo.calls {
		if !strings.HasPrefix(c, "retry:") {
			t.Fatalf("identity down must never skip or fail: %v", r.repo.calls)
		}
	}
}

func TestNoticeSenderZnsOANeedsReauthRevealsNothing(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelZNS)
	r.oas.tokenErr = ErrZaloOANeedsReauthorization
	r.tick(t)
	if len(r.phones.calls) != 0 {
		t.Fatal("numbers revealed for a campaign that cannot go out")
	}
	for _, c := range r.repo.calls {
		if !strings.HasSuffix(c, ":"+domain.NoticeFailOANeedsReauth) {
			t.Fatalf("outcomes = %v", r.repo.calls)
		}
	}
}

func TestNoticeSenderZnsClasses(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelZNS)
	r.zns.outcomes = map[string]zalooa.Outcome{senderPhone2: zalooa.OutcomeRecipientRejected}
	r.tick(t)
	if len(r.zns.sent) != 2 || r.zns.sent[0] != senderPhone1+"|1001|R1|Hop dan" {
		t.Fatalf("sent = %v", r.zns.sent)
	}
	want := []string{"sent:R1:zmsg-R1", "fail:R2:" + domain.NoticeFailInvalidNumber, "skip:R3:" + domain.NoticeFailRecipientInactive}
	if strings.Join(r.repo.calls, " ") != strings.Join(want, " ") {
		t.Fatalf("outcomes = %v", r.repo.calls)
	}
	r.assertNoPhoneEscaped(t)
}

func TestNoticeSenderRetiredAccountFailsTheBatch(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelSMS)
	r.accounts.err = ErrSmsAccountNotFound
	r.tick(t)
	if len(r.phones.calls) != 0 || len(r.repo.calls) != 3 || !strings.HasSuffix(r.repo.calls[0], domain.NoticeFailSourceRetired) {
		t.Fatalf("reveals %v outcomes %v", r.phones.calls, r.repo.calls)
	}
}

func TestNoticeSenderLeavesMergedCommuneAndOtherReplicaAlone(t *testing.T) {
	r := newSenderRig(domain.NoticeChannelSMS)
	r.registry.inactive = true
	r.tick(t)
	if r.repo.claimed != 0 {
		t.Fatal("claimed rows of a merged commune")
	}
	r = newSenderRig(domain.NoticeChannelSMS)
	r.locks.locked = true
	r.tick(t)
	if r.repo.claimed != 0 {
		t.Fatal("sent while another replica holds the lock")
	}
}
