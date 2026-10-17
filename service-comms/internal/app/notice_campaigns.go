package app

// NOTICE CAMPAIGNS (migration 0032; ADR 0099 #4–#8) — the use case behind "Gửi tin ZNS / SMS"
// (/api/v1/notice-templates, /notice-campaign-previews, /notice-campaigns; notice.send): the templates to
// compose from, the preview, the send, and the history. The SENDING is notice_sender.go.
//
// THE COMMUNE IS THE CONTEXT'S, always (rule 1, invariant 4). Every source (OA, eSMS account, template)
// is read through its own use case, scoped to the context's commune; an id of another commune reads as
// "not found", never as "exists elsewhere".
//
// THE PHONE NUMBER NEVER REACHES THIS FILE IN FULL (ADR 0099 #5). The preview asks identity for COUNTS
// only (count_only); the send asks for refs + MASKED numbers and writes exactly those. The full number is
// asked per batch by the sender, at send time.
//
// THE SEND IS ONE TRANSACTION (ADR 0099 #8; transaction-boundaries gui_dot_tin_zns_sms (2)): the campaign
// row, EVERY recipient row and the audit entry commit together, or nothing does. identity is asked BEFORE
// the transaction, and if it cannot be asked nothing is written — never an empty campaign "sent" to
// nobody.
//
// THE ESTIMATE IS NEVER A GATE (ADR 0099 #7): ZNS is priced from the template's Zalo tag (the table in
// internal/domain), SMS by eSMS's price estimate; an estimate that cannot be had is stored as NULL and the
// send goes ahead.
//
// TYPED / IMPORTED NUMBERS (audience danh-sach-so / tep-nhap; ADR 0099 §Sửa đổi S1–S7) are notice_typed.go:
// no identity call at all, the numbers normalised here, each owed row's number and message SEALED under the
// commune's DEK and written in the same transaction as the campaign.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// NoticeAudienceSource is identity's audience pair (ADR 0099 #5). *identityclient.Client satisfies it.
type NoticeAudienceSource interface {
	CountNoticeAudience(ctx context.Context, a identityclient.NoticeAudience) (identityclient.NoticeAudienceCounts, error)
	ResolveNoticeAudience(ctx context.Context, a identityclient.NoticeAudience) ([]identityclient.NoticeAudienceMember,
		identityclient.NoticeAudienceCounts, error)
}

// NoticeOASource is what a campaign needs of the commune's OAs. *CommuneZaloOAs satisfies it.
type NoticeOASource interface {
	List(ctx context.Context) (CommuneZaloOAList, error)
	ListTemplates(ctx context.Context, oaRowID string) ([]domain.ZaloOATemplate, error)
	TemplateForSend(ctx context.Context, oaRowID, templateID string) (domain.ZaloOATemplate, error)
	OpenAccessToken(ctx context.Context, oaRowID string) (secret.Secret, string, error)
}

// NoticeSmsSource is what a campaign needs of the commune's eSMS accounts. *SmsAccounts satisfies it.
type NoticeSmsSource interface {
	ListAccounts(ctx context.Context) ([]domain.SmsAccount, error)
	ListTemplates(ctx context.Context, accountID string) ([]SmsTemplateGroup, error)
	TemplateForSend(ctx context.Context, accountID, templateRowID string) (domain.SmsAccount, domain.SmsTemplate, error)
	OpenForSend(ctx context.Context, accountID string) (domain.SmsAccount, esms.Credentials, error)
}

// NoticeSmsPricer is eSMS's price estimate. *esms.Client satisfies it.
type NoticeSmsPricer interface {
	EstimateCSKH(ctx context.Context, creds esms.Credentials, brandname, content string, isUnicode bool) (string, esms.Outcome)
}

// NoticeSealer seals the typed rows' numbers and messages under the commune's DEK (ADR 0099 S6). The
// commune envelope (*crypto.Envelope) satisfies it — a nil one refuses with crypto.ErrNotConfigured.
type NoticeSealer interface {
	SealMany(ctx context.Context, plaintexts []secret.Secret, aads [][]byte) ([][]byte, error)
}

// NoticeCampaignRepo is the campaign store's half this use case writes and reads.
// *docstore.NoticeCampaignStore satisfies it; every method reads through db.For(ctx) — the context's
// commune is tenant_id = $1.
type NoticeCampaignRepo interface {
	InsertCampaign(ctx context.Context, tx *store.ScopedTx, c domain.NoticeCampaign) error
	InsertRecipients(ctx context.Context, tx *store.ScopedTx, campaignID string, rs []domain.NoticeRecipient, at time.Time) error
	List(ctx context.Context, after domain.NoticeCursor, limit int) ([]docstore.NoticeCampaignView, error)
	Get(ctx context.Context, id string) (docstore.NoticeCampaignView, bool, error)
	ListRecipients(ctx context.Context, campaignID, afterID string, limit int) ([]domain.NoticeRecipientView, error)
}

var (
	// ErrNoticeSourceMissing — the OA / account is not a live one of this commune (404).
	ErrNoticeSourceMissing = errors.New("notice_campaign: no such live OA / SMS account in the commune")
	// ErrNoticeSourceUnavailable — the OA needs re-authorization: nothing can go out through it until the
	// OA admin acts (409).
	ErrNoticeSourceUnavailable = errors.New("notice_campaign: the source cannot send now")
	// ErrNoticeTemplateMissing — no such template under that source (404).
	ErrNoticeTemplateMissing = errors.New("notice_campaign: no such template under the source")
	// ErrNoticeTemplateNotSendable — the template exists but the provider would refuse it (ZBS not
	// ENABLE; eSMS no longer lists it) (409).
	ErrNoticeTemplateNotSendable = errors.New("notice_campaign: the template cannot be sent")
	// ErrNoticeNoRecipients — the audience reaches nobody (409); nothing is written.
	ErrNoticeNoRecipients = errors.New("notice_campaign: the audience reaches no recipient")
	// ErrNoticeIdentityUnavailable — identity could not be asked; nothing is written (503).
	ErrNoticeIdentityUnavailable = errors.New("notice_campaign: identity could not be asked")
	// ErrNoticeCampaignNotFound — no such campaign in this commune (404).
	ErrNoticeCampaignNotFound = errors.New("notice_campaign: no such campaign in the commune")
)

// NoticeMissingParamsError — a send with template parameters still empty (422). Missing names the keys
// (ZNS parameter names, SMS slot positions) — never a value.
type NoticeMissingParamsError struct{ Missing []string }

func (e *NoticeMissingParamsError) Error() string {
	return "notice_campaign: missing template parameters: " + strings.Join(e.Missing, ", ")
}

// NoticeInput is one preview or send. The commune is the context's; no field names one.
type NoticeInput struct {
	Channel      string
	OARowID      string
	SmsAccountID string
	TemplateRef  string
	AudienceKind string
	AudienceIDs  []string
	AudienceTags []string
	Params       map[string]string
	Title        string // send only
	// Typed / imported audiences only (danh-sach-so, tep-nhap): the rows, the SMS content mode and, for
	// SmsContentShared, the shared text. Refused for a directory audience.
	Recipients     []NoticeRecipientInput
	SmsContentMode string
	Content        string
	// Mapping — tep-nhap only: which file column fed what (stored on the campaign). Refused otherwise.
	Mapping *domain.NoticeRecipientMapping
}

// NoticeRecipientInput is one typed / imported row as the browser sent it. Phone is the cell as typed — it
// is normalised here, never trusted. Content is the row's own SMS text (SmsContentPerRow); Values the
// row's mapped columns ({{variable_i}} for SMS, ZBS parameter names for ZNS). Never logged.
type NoticeRecipientInput struct {
	Phone   string
	Content string
	Values  map[string]string
}

// NoticePreview is POST notice-campaign-previews. Content is nil for ZNS (Zalo keeps the template body;
// comms has none); for SMS to typed numbers it is the FIRST valid row's text, a sample. UnitPrice /
// EstimatedCost are "" when no estimate could be had. InvalidCount / DuplicateCount: typed rows whose
// number did not normalise / normalised duplicates dropped (0 for a directory audience).
type NoticePreview struct {
	Content        *string
	RecipientCount int
	SkippedCount   int
	InvalidCount   int
	DuplicateCount int
	UnitPrice      string
	EstimatedCost  string
	MissingParams  []string
}

// ZnsNoticeTemplate is one approved (ENABLE) ZBS template of one live OA, as the composer lists it.
type ZnsNoticeTemplate struct {
	OARowID                string
	OAName                 string
	OANeedsReauthorization bool
	TemplateID             string
	TemplateName           string
	Tag                    string
	Params                 []domain.TemplateParam
	UnitPrice              string
}

// SmsNoticeTemplate is one template TEXT of one live eSMS account, as the composer lists it. Its slots are
// filled by POSITION (params key "1", "2", …). The price comes with the preview (eSMS prices a filled text).
type SmsNoticeTemplate struct {
	SmsAccountID string
	AccountLabel string
	Brandname    string
	TemplateRef  string
	Content      string
	Params       []domain.SmsTemplateParam
}

// NoticeTemplates is GET notice-templates.
type NoticeTemplates struct {
	ZNS []ZnsNoticeTemplate
	SMS []SmsNoticeTemplate
}

// NoticeCampaignItem is one campaign as the history shows it.
type NoticeCampaignItem struct {
	Campaign domain.NoticeCampaign
	// SourceName — the OA's name (ZNS), or "label · Brandname" (SMS).
	SourceName string
	// Skipped — identity's skipped count at the send, PLUS (once finished) the rows the sender skipped
	// because identity no longer delivered them: "Bỏ qua" on the screen (ADR 0099 #5). While the
	// campaign runs, only the first part is known without counting rows.
	Skipped int
}

// NoticeCampaigns owns the composer's reads, the preview and the send.
type NoticeCampaigns struct {
	db       *store.DB
	repo     NoticeCampaignRepo
	audience NoticeAudienceSource
	oas      NoticeOASource
	sms      NoticeSmsSource
	pricer   NoticeSmsPricer
	sealer   NoticeSealer

	newID func() (string, error)
	now   func() time.Time
}

// NewNoticeCampaigns panics on a missing dependency — at construction, not on the first request.
// sealer may be a nil *crypto.Envelope (no SECRET_ENCRYPTION_KEYS): a send to typed numbers then answers
// crypto.ErrNotConfigured by name, and everything else still works.
func NewNoticeCampaigns(db *store.DB, repo NoticeCampaignRepo, audience NoticeAudienceSource, oas NoticeOASource,
	sms NoticeSmsSource, pricer NoticeSmsPricer, sealer NoticeSealer) *NoticeCampaigns {

	if db == nil || repo == nil || audience == nil || oas == nil || sms == nil || pricer == nil || sealer == nil {
		panic("app: NewNoticeCampaigns thiếu phụ thuộc")
	}
	return &NoticeCampaigns{db: db, repo: repo, audience: audience, oas: oas, sms: sms, pricer: pricer, sealer: sealer,
		newID: ulid.Moi, now: time.Now}
}

func (uc *NoticeCampaigns) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// ---- the composer's templates ---------------------------------------------------------------------------

// Templates lists what a campaign can be composed from (ADR 0099 #4): every ENABLE ZBS template of every
// live OA, and every template text eSMS still lists for every live account. A ZBS template for a campaign
// need NOT carry the three milestone parameters (ADR 0098 #9 is for milestone notices only).
func (uc *NoticeCampaigns) Templates(ctx context.Context) (NoticeTemplates, error) {
	out := NoticeTemplates{ZNS: []ZnsNoticeTemplate{}, SMS: []SmsNoticeTemplate{}}
	// Scoped: CommuneZaloOAs reads through db.For(ctx) — the context's commune is tenant_id = $1.
	oas, err := uc.oas.List(ctx)
	if err != nil {
		return NoticeTemplates{}, fmt.Errorf("notice_campaign: list OAs: %w", err)
	}
	for _, it := range oas.Items {
		ts, err := uc.oas.ListTemplates(ctx, it.OA.ID)
		if errors.Is(err, ErrCommuneZaloOAMissing) {
			continue // disconnected between the two reads
		}
		if err != nil {
			return NoticeTemplates{}, fmt.Errorf("notice_campaign: list OA templates: %w", err)
		}
		for _, t := range ts {
			if t.ZaloStatus != domain.ZaloTemplateStatusEnabled {
				continue
			}
			params := t.Params
			if params == nil {
				params = []domain.TemplateParam{}
			}
			out.ZNS = append(out.ZNS, ZnsNoticeTemplate{OARowID: it.OA.ID, OAName: it.OA.OAName,
				OANeedsReauthorization: it.OA.NeedsReauthorization(), TemplateID: t.TemplateID, TemplateName: t.TemplateName,
				Tag: t.ZaloTag, Params: params, UnitPrice: domain.ZnsUnitPrice(t.ZaloTag)})
		}
	}
	accounts, err := uc.sms.ListAccounts(ctx)
	if err != nil {
		return NoticeTemplates{}, fmt.Errorf("notice_campaign: list SMS accounts: %w", err)
	}
	for _, a := range accounts {
		groups, err := uc.sms.ListTemplates(ctx, a.ID)
		if errors.Is(err, ErrSmsAccountNotFound) {
			continue // retired between the two reads
		}
		if err != nil {
			return NoticeTemplates{}, fmt.Errorf("notice_campaign: list SMS templates: %w", err)
		}
		for _, g := range groups {
			out.SMS = append(out.SMS, SmsNoticeTemplate{SmsAccountID: a.ID, AccountLabel: a.Label, Brandname: a.Brandname,
				TemplateRef: g.TemplateRowID, Content: g.Content, Params: g.Params})
		}
	}
	return out, nil
}

// ---- preview and send ---------------------------------------------------------------------------------

// composed is a request checked against its source and template.
type composed struct {
	channel, oaRowID, smsAccountID, templateRef, templateName, templateContent string
	sourceName                                                                 string
	audience                                                                   domain.NoticeAudience
	params                                                                     map[string]string // only the template's own
	missing                                                                    []string
	content                                                                    *string
	unitPrice                                                                  string // "" = no estimate
	// Typed / imported audiences only (notice_typed.go).
	smsContentMode   string
	rows             []typedRow // every input row except normalised duplicates, in input order
	valid            int
	invalid          int
	duplicate        int
	missingVariables bool // c.missing names the shared SMS text's variables, not template parameters
	mapping          *domain.NoticeRecipientMapping
}

// compose validates the request alone, then against its source and template — the checks Preview and
// Send share. The SMS estimate needs a filled text, so it is asked only when nothing is missing.
func (uc *NoticeCampaigns) compose(ctx context.Context, in NoticeInput) (composed, error) {
	audience, err := domain.NormalizeNoticeAudience(in.AudienceKind, in.AudienceIDs, in.AudienceTags)
	if err != nil {
		return composed{}, err
	}
	params, err := domain.NormalizeNoticeParams(in.Params)
	if err != nil {
		return composed{}, err
	}
	if audience.Typed() {
		return uc.composeTyped(ctx, in, audience, params)
	}
	// A directory audience: the rows come from identity, so none may come from the client.
	switch {
	case len(in.Recipients) > 0:
		return composed{}, domain.ErrNoticeRecipients
	case in.SmsContentMode != "":
		return composed{}, domain.ErrNoticeContentMode
	case in.Content != "":
		return composed{}, domain.ErrNoticeContent
	case in.Mapping != nil:
		return composed{}, domain.ErrNoticeMapping
	}
	if err := domain.ValidateNoticeSource(in.Channel, in.OARowID, in.SmsAccountID, in.TemplateRef); err != nil {
		return composed{}, err
	}
	c := composed{channel: in.Channel, oaRowID: in.OARowID, smsAccountID: in.SmsAccountID, templateRef: in.TemplateRef,
		audience: audience}
	if in.Channel == domain.NoticeChannelZNS {
		return uc.composeZNS(ctx, c, params)
	}
	return uc.composeSMS(ctx, c, params)
}

// znsTemplate reads the ZBS template of the campaign's OA, mapping the source's errors onto the campaign's.
func (uc *NoticeCampaigns) znsTemplate(ctx context.Context, oaRowID, templateRef string) (domain.ZaloOATemplate, error) {
	t, err := uc.oas.TemplateForSend(ctx, oaRowID, templateRef)
	switch {
	case errors.Is(err, ErrCommuneZaloOAMissing):
		return domain.ZaloOATemplate{}, ErrNoticeSourceMissing
	case errors.Is(err, ErrZaloOANeedsReauthorization):
		return domain.ZaloOATemplate{}, ErrNoticeSourceUnavailable
	case errors.Is(err, ErrZaloOATemplateMissing):
		return domain.ZaloOATemplate{}, ErrNoticeTemplateMissing
	case errors.Is(err, ErrZaloOATemplateNotSendable):
		return domain.ZaloOATemplate{}, ErrNoticeTemplateNotSendable
	case err != nil:
		return domain.ZaloOATemplate{}, fmt.Errorf("notice_campaign: read the ZBS template: %w", err)
	}
	return t, nil
}

func (uc *NoticeCampaigns) composeZNS(ctx context.Context, c composed, params map[string]string) (composed, error) {
	t, err := uc.znsTemplate(ctx, c.oaRowID, c.templateRef)
	if err != nil {
		return composed{}, err
	}
	missing, err := domain.MissingZnsParams(t.Params, params)
	if err != nil {
		return composed{}, err
	}
	c.templateName = t.TemplateName
	c.params = domain.ZnsTemplateData(t.Params, params)
	c.missing = missing
	c.unitPrice = domain.ZnsUnitPrice(t.ZaloTag)
	c.sourceName = uc.oaName(ctx, c.oaRowID)
	return c, nil
}

// oaName is the OA's display name for the reply (the history reads it through its own join).
func (uc *NoticeCampaigns) oaName(ctx context.Context, oaRowID string) string {
	// Scoped: CommuneZaloOAs reads through db.For(ctx) — the context's commune is tenant_id = $1.
	list, err := uc.oas.List(ctx)
	if err != nil {
		return ""
	}
	for _, it := range list.Items {
		if it.OA.ID == oaRowID {
			return it.OA.OAName
		}
	}
	return ""
}

func (uc *NoticeCampaigns) composeSMS(ctx context.Context, c composed, params map[string]string) (composed, error) {
	a, t, err := uc.sms.TemplateForSend(ctx, c.smsAccountID, c.templateRef)
	switch {
	case errors.Is(err, ErrSmsAccountNotFound):
		return composed{}, ErrNoticeSourceMissing
	case errors.Is(err, ErrSmsTemplateMissing):
		return composed{}, ErrNoticeTemplateMissing
	case errors.Is(err, ErrSmsTemplateNotSendable):
		return composed{}, ErrNoticeTemplateNotSendable
	case err != nil:
		return composed{}, fmt.Errorf("notice_campaign: read the SMS template: %w", err)
	}
	content, missing, err := domain.PreviewSmsContent(t.Content, params)
	if err != nil {
		return composed{}, err
	}
	c.templateName = smsTemplateName(t.Content)
	c.templateContent = t.Content
	c.params = domain.SmsTemplateData(t.Content, params)
	c.missing = missing
	c.content = &content
	c.sourceName = a.Label + " · " + a.Brandname
	if len(missing) == 0 {
		price, err := uc.estimateSMS(ctx, a.ID, content)
		if err != nil {
			return composed{}, err
		}
		c.unitPrice = price
	}
	return c, nil
}

// smsTemplateName — an eSMS template has no name, only its text; the campaign's template_name (≤ 255
// characters, 0032) is the text's beginning, so the history can say which text went out.
func smsTemplateName(content string) string {
	const max = 120
	if utf8.RuneCountInString(content) <= max {
		return content
	}
	return string([]rune(content)[:max-1]) + "…"
}

// estimateSMS asks eSMS what ONE message of the filled text costs (internal/esms EstimateProbePhone — the
// agreed fake number, never a recipient's). An eSMS refusal or outage is "no estimate" (""), never an
// error: ADR 0099 #7. A deployment that cannot open the keys at all (no SECRET_ENCRYPTION_KEYS) IS an
// error — the campaign could never be sent, so the preview says so now.
func (uc *NoticeCampaigns) estimateSMS(ctx context.Context, accountID, filled string) (string, error) {
	a, creds, err := uc.sms.OpenForSend(ctx, accountID)
	switch {
	case errors.Is(err, ErrSmsAccountNotFound):
		return "", ErrNoticeSourceMissing
	case errors.Is(err, crypto.ErrNotConfigured):
		return "", err
	case err != nil:
		return "", fmt.Errorf("notice_campaign: open the SMS account: %w", err)
	}
	price, o := uc.pricer.EstimateCSKH(ctx, creds, a.Brandname, filled, domain.SmsContentIsUnicode(filled))
	clear(creds.APIKey)
	clear(creds.SecretKey)
	if o != esms.OutcomeOK {
		return "", nil
	}
	if _, ok := domain.ParseMoney(price); !ok {
		return "", nil
	}
	return price, nil
}

// noticeAudienceKinds — THE ONE TABLE between 0032's ADR 0011 values and identity's enum (the migration
// header's "one table": five audiences here, the two recipient kinds below). It lives in this package, not
// in internal/domain, because domain imports nothing but the standard library.
var noticeAudienceKinds = map[string]identityclient.NoticeAudienceKind{
	domain.NoticeAudienceCitizenAll:   identityclient.NoticeAudienceCitizenAll,
	domain.NoticeAudienceCitizenGroup: identityclient.NoticeAudienceCitizenGroup,
	domain.NoticeAudienceCitizenTag:   identityclient.NoticeAudienceCitizenTag,
	domain.NoticeAudienceStaffAll:     identityclient.NoticeAudienceStaffAll,
	domain.NoticeAudienceStaffUnit:    identityclient.NoticeAudienceStaffUnit,
}

// noticeRecipientKinds — identity's recipient kind → 0032's value.
var noticeRecipientKinds = map[identityclient.NoticeRecipientKind]string{
	identityclient.NoticeRecipientCitizen: domain.NoticeRecipientCitizen,
	identityclient.NoticeRecipientStaff:   domain.NoticeRecipientStaff,
}

// identityRecipientKind is noticeRecipientKinds read backwards; 0 for a value outside it.
func identityRecipientKind(kind string) identityclient.NoticeRecipientKind {
	for k, v := range noticeRecipientKinds {
		if v == kind {
			return k
		}
	}
	return 0
}

// identityAudience is the stored audience in identity's shape. The kind was validated by the domain, so
// the map always answers.
func identityAudience(a domain.NoticeAudience) identityclient.NoticeAudience {
	out := identityclient.NoticeAudience{Kind: noticeAudienceKinds[a.Kind]}
	switch a.Kind {
	case domain.NoticeAudienceCitizenGroup:
		out.GroupIDs = a.IDs
	case domain.NoticeAudienceCitizenTag:
		out.Tags = a.IDs
	case domain.NoticeAudienceStaffUnit:
		out.OrgUnitIDs = a.IDs
	}
	return out
}

// Preview — "Xem trước" (ADR 0099 #7): nothing is written, nothing audited. identity is asked for COUNTS
// only. Missing parameters are LISTED, not refused — the staff member is still typing.
func (uc *NoticeCampaigns) Preview(ctx context.Context, in NoticeInput) (NoticePreview, error) {
	c, err := uc.compose(ctx, in)
	if err != nil {
		return NoticePreview{}, err
	}
	if c.audience.Typed() {
		return typedPreview(c), nil
	}
	counts, err := uc.audience.CountNoticeAudience(ctx, identityAudience(c.audience))
	if err != nil {
		return NoticePreview{}, fmt.Errorf("%w: %w", ErrNoticeIdentityUnavailable, err)
	}
	out := NoticePreview{Content: c.content, RecipientCount: counts.Recipients,
		SkippedCount: counts.SkippedInactive + counts.SkippedNoPhone, MissingParams: c.missing}
	if c.unitPrice != "" {
		if unit, cost, ok := domain.EstimateCost(c.unitPrice, counts.Recipients); ok {
			out.UnitPrice, out.EstimatedCost = unit, cost
		}
	}
	return out, nil
}

// Send — "Gửi" (ADR 0099 #8). The checks of Preview, then: no parameter missing, the whole audience from
// identity (refs + MASKED numbers, every page, all or nothing), and ONE transaction with the campaign,
// every recipient row ('cho-gui') and the audit entry. The sender takes it from there.
func (uc *NoticeCampaigns) Send(ctx context.Context, in NoticeInput, actor audit.Actor) (NoticeCampaignItem, error) {
	if actor.ID == "" {
		return NoticeCampaignItem{}, ErrNoActor
	}
	title, err := domain.NormalizeNoticeTitle(in.Title)
	if err != nil {
		return NoticeCampaignItem{}, err
	}
	c, err := uc.compose(ctx, in)
	if err != nil {
		return NoticeCampaignItem{}, err
	}
	if len(c.missing) > 0 {
		if c.missingVariables {
			return NoticeCampaignItem{}, &domain.NoticeMissingVariablesError{Missing: c.missing}
		}
		return NoticeCampaignItem{}, &NoticeMissingParamsError{Missing: c.missing}
	}
	if c.audience.Typed() {
		return uc.sendTyped(ctx, c, title, actor)
	}

	// identity, OUTSIDE any transaction (transaction-boundaries gui_dot_tin_zns_sms (2)).
	members, counts, err := uc.audience.ResolveNoticeAudience(ctx, identityAudience(c.audience))
	if err != nil {
		return NoticeCampaignItem{}, fmt.Errorf("%w: %w", ErrNoticeIdentityUnavailable, err)
	}
	wantKind := c.audience.RecipientKind()
	rows := make([]domain.NoticeRecipient, 0, len(members))
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		if seen[m.Ref] {
			continue // 0032's (campaign, ref) key: one message per directory row
		}
		kind := noticeRecipientKinds[m.Kind]
		// A member of the wrong directory, an empty ref or an unmasked number is identity breaking its
		// contract; refused before the transaction (0032's CHECK would refuse it inside), naming nothing.
		if kind != wantKind || m.Ref == "" || len(m.Ref) > 64 || !domain.ValidMaskedPhone(m.MaskedPhone) {
			return NoticeCampaignItem{}, fmt.Errorf("%w: identity returned a recipient outside the contract",
				ErrNoticeIdentityUnavailable)
		}
		seen[m.Ref] = true
		id, err := uc.newID()
		if err != nil {
			return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: new recipient id: %w", err)
		}
		rows = append(rows, domain.NoticeRecipient{ID: id, Ref: m.Ref, Kind: kind, PhoneMasked: m.MaskedPhone})
	}
	if len(rows) == 0 {
		return NoticeCampaignItem{}, ErrNoticeNoRecipients
	}

	id, err := uc.newID()
	if err != nil {
		return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: new id: %w", err)
	}
	at := uc.clock()
	camp := domain.NoticeCampaign{ID: id, Title: title, Channel: c.channel, OARowID: c.oaRowID, SmsAccountID: c.smsAccountID,
		TemplateRef: c.templateRef, TemplateName: c.templateName, TemplateContent: c.templateContent, Params: c.params,
		Audience: c.audience, RecipientCount: len(rows), SkippedCount: counts.SkippedInactive + counts.SkippedNoPhone,
		Status: domain.NoticeCampaignSending, CreatedAt: at, CreatedBy: actor.ID}
	if c.unitPrice != "" {
		if unit, cost, ok := domain.EstimateCost(c.unitPrice, len(rows)); ok {
			camp.UnitPrice, camp.EstimatedCost = unit, cost
		}
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.repo.InsertCampaign(ctx, tx, camp); err != nil {
			return err
		}
		if err := uc.repo.InsertRecipients(ctx, tx, camp.ID, rows, at); err != nil {
			return err
		}
		return writeNoticeCampaignAudit(ctx, tx, actor, camp, at)
	})
	if err != nil {
		return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: save: %w", err)
	}
	return NoticeCampaignItem{Campaign: camp, SourceName: c.sourceName, Skipped: camp.SkippedCount}, nil
}

// writeNoticeCampaignAudit is the send's entry: what went out, through what, to whom (as chosen — kind and
// ids), how many, the estimate. NO NUMBER AND NO REF: the recipient rows hold those, masked. The parameter
// values ARE recorded — staff typed them for the whole campaign, they are what was said (ADR 0099 #6).
func writeNoticeCampaignAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, c domain.NoticeCampaign,
	at time.Time) error {

	fields := map[string]any{
		"title": c.Title, "channel": c.Channel, "oa_row_id": c.OARowID, "sms_account_id": c.SmsAccountID,
		"template_ref": c.TemplateRef, "template_name": c.TemplateName, "params": c.Params,
		"audience_kind": c.Audience.Kind, "audience_ids": c.Audience.IDs, "recipient_count": c.RecipientCount,
		"skipped_count": c.SkippedCount, "unit_price": c.UnitPrice, "estimated_cost": c.EstimatedCost,
	}
	if c.Audience.Typed() {
		// COUNTS ONLY for typed / imported numbers: the shared text and the fixed parameters stay on the
		// campaign row (the archival record); each row's own message is sealed and gone at its outcome, and
		// must not survive in the trail instead (S6).
		delete(fields, "template_name")
		delete(fields, "params")
		fields["sms_content_mode"] = c.SmsContentMode
		fields["invalid_count"] = c.InvalidCount
		fields["recipient_mapping"] = c.RecipientMapping // column HEADERS only, never a value
	}
	delta, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("notice_campaign: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionSendNoticeCampaign, Subject: c.Subject(),
		At: at, Delta: delta})
}

// ---- history --------------------------------------------------------------------------------------------

func noticeItem(v docstore.NoticeCampaignView) NoticeCampaignItem {
	c := v.Campaign
	name := v.OAName
	if c.Channel == domain.NoticeChannelSMS {
		name = v.AccountLabel + " · " + v.Brandname
	}
	skipped := c.SkippedCount
	if c.Finished() {
		if rest := c.RecipientCount - c.SentCount - c.FailedCount; rest > 0 {
			skipped += rest // rows the sender left 'bo-qua'
		}
	}
	return NoticeCampaignItem{Campaign: c, SourceName: name, Skipped: skipped}
}

// List reads one page of the commune's campaigns, newest first. next is "" on the last page.
func (uc *NoticeCampaigns) List(ctx context.Context, cursor string) ([]NoticeCampaignItem, string, error) {
	after, err := domain.DecodeNoticeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	// Scoped: the store reads through db.For(ctx) — the context's commune is tenant_id = $1.
	views, err := uc.repo.List(ctx, after, domain.NoticeListPageSize)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(views) > domain.NoticeListPageSize {
		views = views[:domain.NoticeListPageSize]
		last := views[len(views)-1].Campaign
		next = domain.NoticeCursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	}
	out := make([]NoticeCampaignItem, 0, len(views))
	for _, v := range views {
		out = append(out, noticeItem(v))
	}
	return out, next, nil
}

// Recipients reads one page of one campaign's per-number history (S5): masked number, outcome, the
// provider's scrubbed message. The campaign must be this commune's (ErrNoticeCampaignNotFound otherwise —
// the same for another commune's id). next is "" on the last page.
func (uc *NoticeCampaigns) Recipients(ctx context.Context, campaignID, cursor string) ([]domain.NoticeRecipientView, string, error) {
	after, err := domain.DecodeNoticeRecipientCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	// Scoped: the store reads through db.For(ctx) — the context's commune is tenant_id = $1.
	if _, found, err := uc.repo.Get(ctx, campaignID); err != nil {
		return nil, "", err
	} else if !found {
		return nil, "", ErrNoticeCampaignNotFound
	}
	rows, err := uc.repo.ListRecipients(ctx, campaignID, after, domain.NoticeRecipientPageSize)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > domain.NoticeRecipientPageSize {
		rows = rows[:domain.NoticeRecipientPageSize]
		next = domain.EncodeNoticeRecipientCursor(rows[len(rows)-1].ID)
	}
	return rows, next, nil
}

// Get reads one campaign of this commune (ErrNoticeCampaignNotFound — the same for another commune's id).
func (uc *NoticeCampaigns) Get(ctx context.Context, id string) (NoticeCampaignItem, error) {
	// Scoped: the store reads through db.For(ctx) — the context's commune is tenant_id = $1.
	v, found, err := uc.repo.Get(ctx, id)
	if err != nil {
		return NoticeCampaignItem{}, err
	}
	if !found {
		return NoticeCampaignItem{}, ErrNoticeCampaignNotFound
	}
	return noticeItem(v), nil
}
