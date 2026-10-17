package app

// NOTICE CAMPAIGNS TO TYPED / IMPORTED NUMBERS (migration 0033; ADR 0099 §Sửa đổi 17/10/2026, S1–S7):
// audience 'danh-sach-so' (typed or pasted, ≤ 1 000) and 'tep-nhap' (the rows of a .xlsx / .csv the BROWSER
// read — the file itself never reaches this service and is not kept, S2).
//
// WHAT DIFFERS FROM A DIRECTORY CAMPAIGN, and why:
//
//   - NO identity CALL. These numbers are in no directory; identity has nothing to count, skip or reveal.
//     "Ngừng liên hệ" is not checked for them (S1, owner's words).
//   - THE SERVER RE-CHECKS EVERY ROW (S2): each number is normalised here (core/phone), never trusted as the
//     browser sent it. A number that does not normalise is KEPT as a 'khong-hop-le' row with a masked
//     value (S5: "lưu lịch sử từng số kể cả số không hợp lệ"); a normalised duplicate is dropped.
//   - THE FULL NUMBER AND THE ROW'S OWN MESSAGE ARE SEALED under the commune's DEK in the row (S6), with
//     additional data naming the field, the commune and the row; the sender opens them at send time and
//     the outcome UPDATE empties both (0033's CHECK refuses a finished row that still holds them).
//   - SMS HAS NO TEMPLATE (S3): either each row's own text (cot-trong-tep) or one shared text with optional
//     {{variable_i}} that each row's values fill (noi-dung-chung). eSMS refuses a text matching no
//     registered CSKH template per row; that row ends that-bai with eSMS's message (S5).
//   - ZNS: the template and OA as for a directory campaign; the typed `params` are the fixed values, and a
//     row's values[<param>] overrides the fixed value FOR THAT ROW (S4).
//
// THE SEND IS STILL ONE TRANSACTION (ADR 0099 #8): campaign + every row (owed AND invalid) + the audit
// entry. The sealing happens BEFORE it — a commune's first seal may create its DEK in its own short
// transaction (sms_accounts.go's order).
//
// PERSONAL DATA (rule 3): numbers and messages live in local variables and sealed bytes only. Nothing here
// logs, and no error carries a number, a row's text or a value.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/phone"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// noticePhoneAAD / noticePayloadAAD — 0033's bindings, verbatim: bound to the ROW (bytes copied onto
// another row do not open), to the commune, and to the FIELD (a number does not open as a message).
func noticePhoneAAD(ctx context.Context, rowID string) []byte {
	return []byte("notice_recipient/phone/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

func noticePayloadAAD(ctx context.Context, rowID string) []byte {
	return []byte("notice_recipient/payload/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

// typedRow is one typed / imported row after the server's check. phone == "" means the number did not
// normalise (a 'khong-hop-le' row). Never logged, never formatted.
type typedRow struct {
	phone  string            // national form, "" = invalid
	masked string            // what the history shows
	text   string            // SMS: the final text of this row
	params map[string]string // ZNS: the fixed values with this row's overrides
}

// composeTyped checks a typed / imported request against its source, then every row.
func (uc *NoticeCampaigns) composeTyped(ctx context.Context, in NoticeInput, audience domain.NoticeAudience,
	params map[string]string) (composed, error) {

	if n := len(in.Recipients); n == 0 || n > domain.MaxTypedRecipientsFor(audience.Kind) {
		return composed{}, domain.ErrNoticeRecipients
	}
	values := make([]map[string]string, len(in.Recipients))
	for i, r := range in.Recipients {
		v, err := domain.NormalizeNoticeRowValues(r.Values)
		if err != nil {
			return composed{}, err
		}
		values[i] = v
	}
	c := composed{channel: in.Channel, oaRowID: in.OARowID, smsAccountID: in.SmsAccountID, templateRef: in.TemplateRef,
		audience: audience}

	var (
		shared   string
		declared []domain.TemplateParam
		keep     map[string]bool
	)
	switch in.Channel {
	case domain.NoticeChannelSMS:
		if err := domain.ValidateTypedSmsSource(in.OARowID, in.SmsAccountID, in.TemplateRef, in.SmsContentMode); err != nil {
			return composed{}, err
		}
		if len(params) > 0 {
			return composed{}, domain.ErrNoticeParams // no template, so no template parameter
		}
		account, err := uc.liveAccount(ctx, in.SmsAccountID)
		if err != nil {
			return composed{}, err
		}
		c.smsContentMode = in.SmsContentMode
		c.sourceName = account.Label + " · " + account.Brandname
		if in.SmsContentMode == domain.SmsContentShared {
			text, ok := domain.NormalizeNoticeMessageText(in.Content)
			if !ok {
				return composed{}, domain.ErrNoticeContent
			}
			shared = text
			c.templateContent = text
			c.templateName = smsTemplateName(text)
			c.missing = domain.MissingNoticeVariables(text, values)
			c.missingVariables = true
			keep = make(map[string]bool, len(c.missing))
			for _, m := range c.missing {
				keep[m] = true
			}
		} else if in.Content != "" {
			return composed{}, domain.ErrNoticeContent
		}
		c.params = map[string]string{}
	case domain.NoticeChannelZNS:
		if err := domain.ValidateNoticeSource(in.Channel, in.OARowID, in.SmsAccountID, in.TemplateRef); err != nil {
			return composed{}, err
		}
		if in.SmsContentMode != "" {
			return composed{}, domain.ErrNoticeContentMode
		}
		if in.Content != "" {
			return composed{}, domain.ErrNoticeContent
		}
		t, err := uc.znsTemplate(ctx, in.OARowID, in.TemplateRef)
		if err != nil {
			return composed{}, err
		}
		declared = t.Params
		c.templateName = t.TemplateName
		c.params = domain.ZnsTemplateData(declared, params)
		c.unitPrice = domain.ZnsUnitPrice(t.ZaloTag)
		c.sourceName = uc.oaName(ctx, in.OARowID)
		// A parameter is missing when it has no fixed value AND is not mapped to a column AND no row carries
		// a value for it. The web sends a mapped parameter in `params` as "" (dropped above as empty) with the
		// real value in each row's values; a mapped column whose cells are all empty is still "mapped" — the
		// rows with an empty cell are refused by Zalo one by one, with Zalo's message (S5).
		provided := make(map[string]string, len(declared))
		for k, v := range c.params {
			provided[k] = v
		}
		for _, p := range declared {
			if _, ok := provided[p.Name]; ok {
				continue
			}
			if in.Mapping != nil {
				if _, mapped := in.Mapping.Params[p.Name]; mapped {
					provided[p.Name] = ""
					continue
				}
			}
			for _, v := range values {
				if v[p.Name] != "" {
					provided[p.Name] = ""
					break
				}
			}
		}
		missing, err := domain.MissingZnsParams(declared, provided)
		if err != nil {
			return composed{}, err
		}
		c.missing = missing
	default:
		return composed{}, domain.ErrNoticeChannel
	}
	mapping, err := domain.NormalizeNoticeMapping(in.Mapping, audience.Kind, in.Channel, in.SmsContentMode, shared, declared)
	if err != nil {
		return composed{}, err
	}
	c.mapping = mapping

	seen := make(map[string]bool, len(in.Recipients))
	c.rows = make([]typedRow, 0, len(in.Recipients))
	for i, r := range in.Recipients {
		national, ok := "", false
		if !domain.RawPhoneTooLong(r.Phone) {
			national, ok = phone.NormalizeVNMobile(r.Phone)
		}
		if !ok {
			c.rows = append(c.rows, typedRow{masked: domain.MaskInvalidNoticePhone(r.Phone)})
			c.invalid++
			continue
		}
		if seen[national] {
			c.duplicate++
			continue
		}
		seen[national] = true
		row := typedRow{phone: national, masked: privacy.MaskPhone(national)}
		switch {
		case in.Channel == domain.NoticeChannelZNS:
			merged := make(map[string]string, len(declared))
			for k, v := range c.params {
				merged[k] = v
			}
			for _, p := range declared {
				v := values[i][p.Name]
				if v == "" {
					continue // an empty cell keeps the fixed value (or none)
				}
				if p.MaxLength > 0 && utf8.RuneCountInString(v) > p.MaxLength {
					return composed{}, domain.ErrNoticeParamLong
				}
				merged[p.Name] = v
			}
			row.params = merged
		case c.smsContentMode == domain.SmsContentShared:
			text, ok := domain.NormalizeNoticeMessageText(domain.RenderNoticeContent(shared, values[i], keep))
			if !ok {
				return composed{}, &domain.NoticeRowContentError{Row: i + 1}
			}
			row.text = text
		default:
			text, ok := domain.NormalizeNoticeMessageText(r.Content)
			if !ok {
				return composed{}, &domain.NoticeRowContentError{Row: i + 1}
			}
			row.text = text
		}
		c.rows = append(c.rows, row)
		c.valid++
	}

	if in.Channel == domain.NoticeChannelSMS {
		longest := ""
		for _, r := range c.rows {
			if r.phone == "" {
				continue
			}
			if c.content == nil {
				sample := r.text
				c.content = &sample
			}
			if utf8.RuneCountInString(r.text) > utf8.RuneCountInString(longest) {
				longest = r.text
			}
		}
		// THE ESTIMATE PRICES THE LONGEST TEXT (vendor choice): eSMS prices one text, and rows differ in
		// length; the longest is the one that may need the most segments, so the estimate errs high rather
		// than low. Never a gate (ADR 0099 #7).
		if len(c.missing) == 0 && longest != "" {
			price, err := uc.estimateSMS(ctx, in.SmsAccountID, longest)
			if err != nil {
				return composed{}, err
			}
			c.unitPrice = price
		}
	}
	return c, nil
}

// liveAccount finds a live eSMS account of this commune by id (ErrNoticeSourceMissing otherwise — the same
// for another commune's id).
func (uc *NoticeCampaigns) liveAccount(ctx context.Context, id string) (domain.SmsAccount, error) {
	// Scoped: SmsAccounts reads through db.For(ctx) — the context's commune is tenant_id = $1.
	accounts, err := uc.sms.ListAccounts(ctx)
	if err != nil {
		return domain.SmsAccount{}, fmt.Errorf("notice_campaign: list SMS accounts: %w", err)
	}
	for _, a := range accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return domain.SmsAccount{}, ErrNoticeSourceMissing
}

// typedPreview — "Xem trước" of a typed / imported campaign: counts from the rows, nothing written.
func typedPreview(c composed) NoticePreview {
	out := NoticePreview{Content: c.content, RecipientCount: c.valid, InvalidCount: c.invalid, DuplicateCount: c.duplicate,
		MissingParams: c.missing}
	if c.unitPrice != "" {
		if unit, cost, ok := domain.EstimateCost(c.unitPrice, c.valid); ok {
			out.UnitPrice, out.EstimatedCost = unit, cost
		}
	}
	return out
}

// sendTyped writes a typed / imported campaign: the owed rows sealed, the invalid rows final, the campaign
// and the audit entry — ONE transaction.
func (uc *NoticeCampaigns) sendTyped(ctx context.Context, c composed, title string, actor audit.Actor) (NoticeCampaignItem, error) {
	if c.valid == 0 {
		return NoticeCampaignItem{}, ErrNoticeNoRecipients // only unreadable numbers: nothing to send
	}
	rows := make([]domain.NoticeRecipient, len(c.rows))
	plain := make([]secret.Secret, 0, 2*c.valid)
	aads := make([][]byte, 0, 2*c.valid)
	defer func() {
		for _, p := range plain {
			clear(p)
		}
	}()
	for i, r := range c.rows {
		id, err := uc.newID()
		if err != nil {
			return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: new recipient id: %w", err)
		}
		rows[i] = domain.NoticeRecipient{ID: id, Ref: id, Kind: domain.NoticeRecipientTyped, PhoneMasked: r.masked}
		if r.phone == "" {
			rows[i].Status = domain.NoticeRecipientInvalid
			continue
		}
		rows[i].Status = domain.NoticeRecipientPending
		payload, err := json.Marshal(domain.NoticePayload{Text: r.text, Params: r.params})
		if err != nil {
			return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: encode a row's message: %w", err)
		}
		plain = append(plain, secret.Secret(r.phone), secret.Secret(payload))
		aads = append(aads, noticePhoneAAD(ctx, id), noticePayloadAAD(ctx, id))
	}
	// Sealed OUTSIDE the transaction: a commune's first seal may create its DEK in its own transaction.
	sealed, err := uc.sealer.SealMany(ctx, plain, aads)
	if err != nil {
		return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: seal the typed numbers: %w", err)
	}
	k := 0
	for i := range rows {
		if rows[i].Status != domain.NoticeRecipientPending {
			continue
		}
		rows[i].PhoneSealed, rows[i].PayloadSealed = sealed[k], sealed[k+1]
		k += 2
	}

	id, err := uc.newID()
	if err != nil {
		return NoticeCampaignItem{}, fmt.Errorf("notice_campaign: new id: %w", err)
	}
	at := uc.clock()
	ref := c.templateRef
	if c.channel == domain.NoticeChannelSMS {
		ref = "" // no template (S3) — stored NULL, 0033 notice_campaign_template_ref_by_audience
	}
	camp := domain.NoticeCampaign{ID: id, Title: title, Channel: c.channel, OARowID: c.oaRowID, SmsAccountID: c.smsAccountID,
		TemplateRef: ref, TemplateName: c.templateName, TemplateContent: c.templateContent, SmsContentMode: c.smsContentMode,
		RecipientMapping: c.mapping, Params: c.params, Audience: domain.NoticeAudience{Kind: c.audience.Kind, IDs: []string{}}, RecipientCount: c.valid,
		InvalidCount: c.invalid, Status: domain.NoticeCampaignSending, CreatedAt: at, CreatedBy: actor.ID}
	if c.unitPrice != "" {
		if unit, cost, ok := domain.EstimateCost(c.unitPrice, c.valid); ok {
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
	return NoticeCampaignItem{Campaign: camp, SourceName: c.sourceName}, nil
}

// errNoticePayload — a row's sealed message opened but is not the JSON sendTyped wrote. Our fault, never a
// provider's; the row is retried and ends khong-kha-dung like any other fault of ours.
var errNoticePayload = errors.New("notice_recipient: unreadable payload")

// decodeNoticePayload reads a row's opened message.
func decodeNoticePayload(b []byte) (domain.NoticePayload, error) {
	var p domain.NoticePayload
	if err := json.Unmarshal(b, &p); err != nil {
		return domain.NoticePayload{}, errNoticePayload
	}
	return p, nil
}
