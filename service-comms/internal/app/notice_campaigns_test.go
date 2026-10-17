package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The campaign use case over fakes for every source and for identity, the recording fake driver for the
// transaction and the audit entry. What is proved: the checks against source and template, the preview
// writes nothing, the send asks identity OUTSIDE the transaction and writes campaign + rows + entry in ONE,
// nothing is written when identity fails, and no full number is ever handled.

// The production types satisfy the interfaces this file fakes.
var (
	_ NoticeOASource       = (*CommuneZaloOAs)(nil)
	_ NoticeSmsSource      = (*SmsAccounts)(nil)
	_ NoticeSmsPricer      = (*esms.Client)(nil)
	_ NoticeCampaignRepo   = (*docstore.NoticeCampaignStore)(nil)
	_ NoticeAudienceSource = (*identityclient.Client)(nil)
)

var noticeClock = time.Date(2026, 10, 17, 5, 0, 0, 0, time.UTC)

const (
	noticeOAID   = "01JNOTICEOA000000000000001"
	noticeAccID  = "01JNOTICEACC00000000000001"
	noticeTplRow = "01JNOTICETPL00000000000001"
	noticeSmsTpl = "Xa {P1,30} moi ba con hop luc {P2,10}"
	noticeActor  = "CB-00123"
)

type fakeNoticeOAs struct {
	templates   []domain.ZaloOATemplate
	needsReauth bool
	sendErr     error
}

func (f *fakeNoticeOAs) List(context.Context) (CommuneZaloOAList, error) {
	o := domain.CommuneZaloOA{ID: noticeOAID, OAID: "4321", OAName: "UBND Xã Mẫu"}
	if f.needsReauth {
		o.NeedsReauthorizationSince = noticeClock
	}
	return CommuneZaloOAList{Items: []CommuneZaloOAItem{{OA: o}}}, nil
}

func (f *fakeNoticeOAs) ListTemplates(_ context.Context, id string) ([]domain.ZaloOATemplate, error) {
	if id != noticeOAID {
		return nil, ErrCommuneZaloOAMissing
	}
	return f.templates, nil
}

func (f *fakeNoticeOAs) TemplateForSend(_ context.Context, oa, tpl string) (domain.ZaloOATemplate, error) {
	switch {
	case f.sendErr != nil:
		return domain.ZaloOATemplate{}, f.sendErr
	case oa != noticeOAID:
		return domain.ZaloOATemplate{}, ErrCommuneZaloOAMissing
	case f.needsReauth:
		return domain.ZaloOATemplate{}, ErrZaloOANeedsReauthorization
	}
	for _, t := range f.templates {
		if t.TemplateID == tpl {
			if t.ZaloStatus != domain.ZaloTemplateStatusEnabled {
				return domain.ZaloOATemplate{}, ErrZaloOATemplateNotSendable
			}
			return t, nil
		}
	}
	return domain.ZaloOATemplate{}, ErrZaloOATemplateMissing
}

func (f *fakeNoticeOAs) OpenAccessToken(context.Context, string) (secret.Secret, string, error) {
	return secret.Secret("FAKE-access-token-NOT-REAL"), "4321", nil
}

type fakeNoticeSms struct {
	opened    []esms.Credentials
	openErr   error
	vanished  bool
	listCalls int
}

func noticeAccount() domain.SmsAccount {
	return domain.SmsAccount{ID: noticeAccID, Provider: "esms", Label: "eSMS chính", Brandname: "XaMau"}
}

func (f *fakeNoticeSms) ListAccounts(context.Context) ([]domain.SmsAccount, error) {
	return []domain.SmsAccount{noticeAccount()}, nil
}

func (f *fakeNoticeSms) ListTemplates(context.Context, string) ([]SmsTemplateGroup, error) {
	f.listCalls++
	params, _ := domain.ParseSmsTemplateParams(noticeSmsTpl)
	return []SmsTemplateGroup{{TemplateRowID: noticeTplRow, Content: noticeSmsTpl, Params: params}}, nil
}

func (f *fakeNoticeSms) TemplateForSend(_ context.Context, acc, row string) (domain.SmsAccount, domain.SmsTemplate, error) {
	switch {
	case acc != noticeAccID:
		return domain.SmsAccount{}, domain.SmsTemplate{}, ErrSmsAccountNotFound
	case row != noticeTplRow:
		return domain.SmsAccount{}, domain.SmsTemplate{}, ErrSmsTemplateMissing
	case f.vanished:
		return domain.SmsAccount{}, domain.SmsTemplate{}, ErrSmsTemplateNotSendable
	}
	return noticeAccount(), domain.SmsTemplate{ID: noticeTplRow, AccountID: acc, Content: noticeSmsTpl}, nil
}

func (f *fakeNoticeSms) OpenForSend(context.Context, string) (domain.SmsAccount, esms.Credentials, error) {
	if f.openErr != nil {
		return domain.SmsAccount{}, esms.Credentials{}, f.openErr
	}
	c := esms.Credentials{APIKey: secret.Secret("FAKE-api-NOT-REAL"), SecretKey: secret.Secret("FAKE-secret-NOT-REAL")}
	f.opened = append(f.opened, c)
	return noticeAccount(), c, nil
}

type fakePricer struct {
	price   string
	outcome esms.Outcome
	content []string
}

func (p *fakePricer) EstimateCSKH(_ context.Context, _ esms.Credentials, _, content string, _ bool) (string, esms.Outcome) {
	p.content = append(p.content, content)
	return p.price, p.outcome
}

type fakeAudience struct {
	f              *sqlFake
	counts         identityclient.NoticeAudienceCounts
	members        []identityclient.NoticeAudienceMember
	err            error
	asked          []identityclient.NoticeAudience
	begunWhenAsked []int
}

func (a *fakeAudience) note(x identityclient.NoticeAudience) {
	a.asked = append(a.asked, x)
	a.f.mu.Lock()
	a.begunWhenAsked = append(a.begunWhenAsked, a.f.begun)
	a.f.mu.Unlock()
}

func (a *fakeAudience) CountNoticeAudience(_ context.Context, x identityclient.NoticeAudience) (identityclient.NoticeAudienceCounts, error) {
	a.note(x)
	return a.counts, a.err
}

func (a *fakeAudience) ResolveNoticeAudience(_ context.Context, x identityclient.NoticeAudience) ([]identityclient.NoticeAudienceMember,
	identityclient.NoticeAudienceCounts, error) {
	a.note(x)
	if a.err != nil {
		return nil, identityclient.NoticeAudienceCounts{}, a.err
	}
	return a.members, a.counts, nil
}

type fakeCampaignRepo struct {
	campaigns  []domain.NoticeCampaign
	recipients []domain.NoticeRecipient
	views      []docstore.NoticeCampaignView
	listAfter  domain.NoticeCursor
	insertErr  error
}

func (r *fakeCampaignRepo) InsertCampaign(_ context.Context, tx *store.ScopedTx, c domain.NoticeCampaign) error {
	if tx == nil || tx.TenantID() != xaA {
		return errors.New("not in the commune's transaction")
	}
	r.campaigns = append(r.campaigns, c)
	return r.insertErr
}

func (r *fakeCampaignRepo) InsertRecipients(_ context.Context, tx *store.ScopedTx, _ string, rs []domain.NoticeRecipient, _ time.Time) error {
	if tx == nil {
		return errors.New("outside a transaction")
	}
	r.recipients = append(r.recipients, rs...)
	return nil
}

func (r *fakeCampaignRepo) List(_ context.Context, after domain.NoticeCursor, _ int) ([]docstore.NoticeCampaignView, error) {
	r.listAfter = after
	return r.views, nil
}

func (r *fakeCampaignRepo) ListRecipients(_ context.Context, campaignID, afterID string, limit int) ([]domain.NoticeRecipientView, error) {
	r.listAfter = domain.NoticeCursor{ID: afterID}
	out := []domain.NoticeRecipientView{}
	for _, x := range r.recipients {
		if x.ID > afterID && len(out) < limit+1 {
			out = append(out, domain.NoticeRecipientView{ID: x.ID, PhoneMasked: x.PhoneMasked, Status: x.Status})
		}
	}
	return out, nil
}

func (r *fakeCampaignRepo) Get(_ context.Context, id string) (docstore.NoticeCampaignView, bool, error) {
	for _, v := range r.views {
		if v.Campaign.ID == id {
			return v, true, nil
		}
	}
	return docstore.NoticeCampaignView{}, false, nil
}

type noticeRig struct {
	f        *sqlFake
	oas      *fakeNoticeOAs
	sms      *fakeNoticeSms
	pricer   *fakePricer
	audience *fakeAudience
	repo     *fakeCampaignRepo
	env      *crypto.Envelope
	uc       *NoticeCampaigns
	ctx      context.Context
}

func newNoticeRig(t *testing.T) *noticeRig {
	t.Helper()
	f := &sqlFake{}
	r := &noticeRig{f: f, ctx: tenant.Into(context.Background(), xaA),
		oas: &fakeNoticeOAs{templates: []domain.ZaloOATemplate{
			{TemplateID: "1001", TemplateName: "Thông báo họp", ZaloStatus: domain.ZaloTemplateStatusEnabled, ZaloTag: "CUSTOMER_CARE",
				Params: []domain.TemplateParam{{Name: "su_kien", MaxLength: 30}, {Name: "thoi_gian"}}},
			{TemplateID: "1002", TemplateName: "Đang duyệt", ZaloStatus: domain.ZaloTemplateStatusPendingReview},
		}},
		sms:    &fakeNoticeSms{},
		pricer: &fakePricer{price: "350", outcome: esms.OutcomeOK},
		audience: &fakeAudience{f: f, counts: identityclient.NoticeAudienceCounts{Recipients: 2, SkippedInactive: 1, SkippedNoPhone: 1},
			members: []identityclient.NoticeAudienceMember{
				{Kind: identityclient.NoticeRecipientCitizen, Ref: "C1", MaskedPhone: "09****0001"},
				{Kind: identityclient.NoticeRecipientCitizen, Ref: "C2", MaskedPhone: "09****0002"},
			}},
		repo: &fakeCampaignRepo{},
	}
	r.env = testEnvelope(t)
	r.uc = NewNoticeCampaigns(store.New(sql.OpenDB(f)), r.repo, r.audience, r.oas, r.sms, r.pricer, r.env)
	n := 0
	r.uc.newID = func() (string, error) { n++; return fmt.Sprintf("01JNOTICEID%015d", n), nil }
	r.uc.now = func() time.Time { return noticeClock }
	return r
}

func smsInput(params map[string]string) NoticeInput {
	return NoticeInput{Channel: domain.NoticeChannelSMS, SmsAccountID: noticeAccID, TemplateRef: noticeTplRow,
		AudienceKind: domain.NoticeAudienceCitizenAll, Params: params, Title: "Họp dân tháng 10"}
}

func znsInput(params map[string]string) NoticeInput {
	return NoticeInput{Channel: domain.NoticeChannelZNS, OARowID: noticeOAID, TemplateRef: "1001",
		AudienceKind: domain.NoticeAudienceCitizenGroup, AudienceIDs: []string{"G1"}, Params: params, Title: "Họp dân"}
}

func noticeStaff() audit.Actor { return audit.Actor{ID: noticeActor, Kind: "staff", IP: "10.0.0.7"} }

func TestNoticeTemplatesListsEnabledZBSAndEverySmsText(t *testing.T) {
	r := newNoticeRig(t)
	got, err := r.uc.Templates(r.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ZNS) != 1 || got.ZNS[0].TemplateID != "1001" || got.ZNS[0].OAName != "UBND Xã Mẫu" ||
		got.ZNS[0].UnitPrice != "200.00" || len(got.ZNS[0].Params) != 2 {
		t.Fatalf("zns = %+v", got.ZNS)
	}
	if len(got.SMS) != 1 || got.SMS[0].TemplateRef != noticeTplRow || got.SMS[0].Brandname != "XaMau" || len(got.SMS[0].Params) != 2 {
		t.Fatalf("sms = %+v", got.SMS)
	}
}

func TestNoticePreviewZnsListsMissingAndWritesNothing(t *testing.T) {
	r := newNoticeRig(t)
	p, err := r.uc.Preview(r.ctx, znsInput(map[string]string{"su_kien": "Họp dân"}))
	if err != nil {
		t.Fatal(err)
	}
	if p.Content != nil || p.RecipientCount != 2 || p.SkippedCount != 2 || p.UnitPrice != "200.00" || p.EstimatedCost != "400.00" ||
		strings.Join(p.MissingParams, ",") != "thoi_gian" {
		t.Fatalf("preview = %+v", p)
	}
	if r.f.begun != 0 || len(r.f.stmts) != 0 {
		t.Fatalf("the preview wrote: %d tx, %d stmts", r.f.begun, len(r.f.stmts))
	}
	if a := r.audience.asked[0]; a.Kind != identityclient.NoticeAudienceCitizenGroup || len(a.GroupIDs) != 1 || a.GroupIDs[0] != "G1" {
		t.Fatalf("identity asked %+v", a)
	}
}

func TestNoticePreviewSmsEstimatesOnlyAFilledText(t *testing.T) {
	r := newNoticeRig(t)
	p, err := r.uc.Preview(r.ctx, smsInput(map[string]string{"1": "Mẫu"}))
	if err != nil {
		t.Fatal(err)
	}
	if p.Content == nil || *p.Content != "Xa Mẫu moi ba con hop luc {P2,10}" || p.UnitPrice != "" ||
		strings.Join(p.MissingParams, ",") != "2" || len(r.pricer.content) != 0 {
		t.Fatalf("preview = %+v content %v; priced %v", p, p.Content, r.pricer.content)
	}
	p, err = r.uc.Preview(r.ctx, smsInput(map[string]string{"1": "Mẫu", "2": "8h"}))
	if err != nil || p.UnitPrice != "350.00" || p.EstimatedCost != "700.00" || len(p.MissingParams) != 0 {
		t.Fatalf("preview = %+v %v", p, err)
	}
	if r.pricer.content[0] != "Xa Mẫu moi ba con hop luc 8h" {
		t.Fatalf("priced %q", r.pricer.content[0])
	}
	// The keys were cleared once eSMS answered.
	for _, b := range r.sms.opened[0].APIKey {
		if b != 0 {
			t.Fatal("ApiKey not cleared")
		}
	}
	// eSMS refusing the estimate is "no estimate", never an error (ADR 0099 #7).
	r.pricer.outcome = esms.OutcomeUnavailable
	if p, err := r.uc.Preview(r.ctx, smsInput(map[string]string{"1": "Mẫu", "2": "8h"})); err != nil || p.UnitPrice != "" || p.EstimatedCost != "" {
		t.Fatalf("estimate down: %+v %v", p, err)
	}
	// No key in the deployment IS an error: the campaign could never be sent.
	r.sms.openErr = crypto.ErrNotConfigured
	if _, err := r.uc.Preview(r.ctx, smsInput(map[string]string{"1": "Mẫu", "2": "8h"})); !errors.Is(err, crypto.ErrNotConfigured) {
		t.Fatalf("no envelope: %v", err)
	}
}

func TestNoticePreviewRefusals(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*noticeRig, *NoticeInput)
		want error
	}{
		{"oa of another commune", func(_ *noticeRig, in *NoticeInput) { in.OARowID = "01JOTHERCOMMUNEOA000000001" }, ErrNoticeSourceMissing},
		{"oa needs reauth", func(r *noticeRig, _ *NoticeInput) { r.oas.needsReauth = true }, ErrNoticeSourceUnavailable},
		{"template pending", func(_ *noticeRig, in *NoticeInput) { in.TemplateRef = "1002" }, ErrNoticeTemplateNotSendable},
		{"template unknown", func(_ *noticeRig, in *NoticeInput) { in.TemplateRef = "9999" }, ErrNoticeTemplateMissing},
		{"both sources", func(_ *noticeRig, in *NoticeInput) { in.SmsAccountID = noticeAccID }, domain.ErrNoticeInvalid},
		{"tags for a group kind", func(_ *noticeRig, in *NoticeInput) { in.AudienceTags = []string{"x"} }, domain.ErrNoticeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newNoticeRig(t)
			in := znsInput(nil)
			tc.mut(r, &in)
			if _, err := r.uc.Preview(r.ctx, in); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, want %v", err, tc.want)
			}
			if len(r.audience.asked) != 0 {
				t.Error("identity was asked for a request that was refused")
			}
		})
	}
	r := newNoticeRig(t)
	r.sms.vanished = true
	if _, err := r.uc.Preview(r.ctx, smsInput(nil)); !errors.Is(err, ErrNoticeTemplateNotSendable) {
		t.Fatalf("vanished sms template: %v", err)
	}
}

func TestNoticeSendWritesCampaignRowsAndEntryInOneTransaction(t *testing.T) {
	r := newNoticeRig(t)
	got, err := r.uc.Send(r.ctx, smsInput(map[string]string{"1": "Mẫu", "2": "8h", "9": "stray"}), noticeStaff())
	if err != nil {
		t.Fatal(err)
	}
	if r.audience.begunWhenAsked[0] != 0 {
		t.Fatal("identity was asked inside a transaction")
	}
	if r.f.begun != 1 || r.f.commits != 1 {
		t.Fatalf("transactions: begun %d commits %d", r.f.begun, r.f.commits)
	}
	if len(r.repo.campaigns) != 1 {
		t.Fatal("no campaign row")
	}
	c := r.repo.campaigns[0]
	if c.CreatedBy != noticeActor || c.Status != domain.NoticeCampaignSending || c.RecipientCount != 2 || c.SkippedCount != 2 ||
		c.TemplateContent != noticeSmsTpl || c.UnitPrice != "350.00" || c.EstimatedCost != "700.00" ||
		len(c.Params) != 2 || c.Params["9"] != "" || c.Audience.Kind != domain.NoticeAudienceCitizenAll {
		t.Fatalf("campaign = %+v", c)
	}
	if len(r.repo.recipients) != 2 || r.repo.recipients[0].PhoneMasked != "09****0001" ||
		r.repo.recipients[0].Kind != domain.NoticeRecipientCitizen {
		t.Fatalf("recipients = %+v", r.repo.recipients)
	}
	aud := r.f.with("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("%d audit entries", len(aud))
	}
	entry := fmt.Sprint(aud[0].args)
	if !strings.Contains(entry, domain.ActionSendNoticeCampaign) || !strings.Contains(entry, noticeActor) ||
		!strings.Contains(entry, "notice-campaign/"+c.ID) {
		t.Fatalf("entry = %s", entry)
	}
	if strings.Contains(entry, "09****") || strings.Contains(entry, "C1") {
		t.Fatalf("the entry carries a recipient: %s", entry)
	}
	if got.SourceName != "eSMS chính · XaMau" || got.Campaign.ID != c.ID {
		t.Fatalf("reply = %+v", got)
	}
}

func TestNoticeSendRefusesWithoutWriting(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*noticeRig, *NoticeInput)
		want func(error) bool
	}{
		{"missing params", func(_ *noticeRig, in *NoticeInput) { in.Params = map[string]string{"1": "Mẫu"} },
			func(err error) bool {
				var m *NoticeMissingParamsError
				return errors.As(err, &m) && strings.Join(m.Missing, ",") == "2"
			}},
		{"identity down", func(r *noticeRig, _ *NoticeInput) { r.audience.err = identityclient.ErrIdentityUnavailable },
			func(err error) bool { return errors.Is(err, ErrNoticeIdentityUnavailable) }},
		{"nobody", func(r *noticeRig, _ *NoticeInput) { r.audience.members = nil },
			func(err error) bool { return errors.Is(err, ErrNoticeNoRecipients) }},
		{"identity returns a full number", func(r *noticeRig, _ *NoticeInput) {
			r.audience.members = []identityclient.NoticeAudienceMember{{Kind: identityclient.NoticeRecipientCitizen, Ref: "C1",
				MaskedPhone: "0900000000"}}
		}, func(err error) bool { return errors.Is(err, ErrNoticeIdentityUnavailable) }},
		{"staff member for a citizen audience", func(r *noticeRig, _ *NoticeInput) {
			r.audience.members = []identityclient.NoticeAudienceMember{{Kind: identityclient.NoticeRecipientStaff, Ref: "CB-1",
				MaskedPhone: "09****0001"}}
		}, func(err error) bool { return errors.Is(err, ErrNoticeIdentityUnavailable) }},
		{"no title", func(_ *noticeRig, in *NoticeInput) { in.Title = "  " },
			func(err error) bool { return errors.Is(err, domain.ErrNoticeTitle) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newNoticeRig(t)
			in := smsInput(map[string]string{"1": "Mẫu", "2": "8h"})
			tc.mut(r, &in)
			if _, err := r.uc.Send(r.ctx, in, noticeStaff()); !tc.want(err) {
				t.Fatalf("err = %v", err)
			}
			if r.f.begun != 0 || len(r.repo.campaigns) != 0 {
				t.Fatal("something was written")
			}
		})
	}
	r := newNoticeRig(t)
	if _, err := r.uc.Send(r.ctx, smsInput(map[string]string{"1": "a", "2": "b"}), audit.Actor{}); !errors.Is(err, ErrNoActor) {
		t.Fatalf("no actor: %v", err)
	}
}

func TestNoticeSendRollsBackWhenTheInsertFails(t *testing.T) {
	r := newNoticeRig(t)
	r.repo.insertErr = errors.New("boom")
	if _, err := r.uc.Send(r.ctx, smsInput(map[string]string{"1": "a", "2": "b"}), noticeStaff()); err == nil {
		t.Fatal("no error")
	}
	if r.f.commits != 0 || r.f.rollbacks != 1 || len(r.f.with("INSERT INTO audit_log")) != 0 {
		t.Fatalf("commits %d rollbacks %d", r.f.commits, r.f.rollbacks)
	}
}

func TestNoticeListPagesAndDerivesSkipped(t *testing.T) {
	r := newNoticeRig(t)
	for i := 0; i < domain.NoticeListPageSize+1; i++ {
		c := domain.NoticeCampaign{ID: fmt.Sprintf("C%03d", i), Channel: domain.NoticeChannelZNS, CreatedAt: noticeClock.Add(-time.Duration(i) * time.Minute),
			Status: domain.NoticeCampaignSending, RecipientCount: 10, SkippedCount: 1}
		if i == 0 {
			c.Status, c.SentCount, c.FailedCount = domain.NoticeCampaignPartlySent, 6, 2
		}
		r.repo.views = append(r.repo.views, docstore.NoticeCampaignView{Campaign: c, OAName: "UBND Xã Mẫu"})
	}
	items, next, err := r.uc.List(r.ctx, "")
	if err != nil || len(items) != domain.NoticeListPageSize || next == "" {
		t.Fatalf("%d items, next %q, %v", len(items), next, err)
	}
	// Finished: identity's 1 + the 2 rows neither sent nor failed. Running: identity's only.
	if items[0].Skipped != 3 || items[1].Skipped != 1 || items[0].SourceName != "UBND Xã Mẫu" {
		t.Fatalf("items[0] = %+v, items[1] = %+v", items[0], items[1])
	}
	r.repo.views = r.repo.views[:1]
	if _, _, err := r.uc.List(r.ctx, next); err != nil || r.repo.listAfter.ID != fmt.Sprintf("C%03d", domain.NoticeListPageSize-1) {
		t.Fatalf("cursor not passed: %+v %v", r.repo.listAfter, err)
	}
	if _, _, err := r.uc.List(r.ctx, "!!"); !errors.Is(err, domain.ErrNoticeInvalid) {
		t.Fatalf("bad cursor: %v", err)
	}
	if _, err := r.uc.Get(r.ctx, "nope"); !errors.Is(err, ErrNoticeCampaignNotFound) {
		t.Fatalf("get: %v", err)
	}
}
