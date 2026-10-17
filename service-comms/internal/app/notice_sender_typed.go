package app

// THE SENDER'S PATH FOR TYPED / IMPORTED ROWS (recipient_kind 'so-nhap'; migration 0033; ADR 0099 §Sửa đổi
// S6). The directory path (notice_sender.go) asks identity for the numbers; these rows carry their own,
// SEALED under the commune's DEK, with their own message beside it. Per campaign of the batch:
//
//  1. the source checked and opened ONCE, BEFORE anything is unsealed — a campaign that cannot go out
//     opens no number (the directory path's rule);
//  2. every row's number and message opened in ONE OpenMany (the commune's DEK read once);
//  3. one provider call per row with RequestId / tracking_id = the row id, as for every row;
//  4. the opened bytes wiped. The outcome UPDATE (record, store) empties both sealed columns.
//
// A row whose seal does not open (or whose message is not the JSON the campaign wrote) is a fault of OURS:
// retried, and after the last attempt that-bai khong-kha-dung — the same answer the directory path gives a
// key it cannot open. Nothing here logs a number, a message or a provider text.

import (
	"context"
	"errors"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
)

func (s *NoticeSender) sendTyped(ctx context.Context, c domain.NoticeCampaign, rows []domain.DueNoticeRecipient,
	results map[string]noticeResult) {

	// 1. The source, before any seal is opened.
	var (
		token   secret.Secret
		creds   esms.Credentials
		account domain.SmsAccount
	)
	if c.Channel == domain.NoticeChannelZNS {
		if _, err := s.d.OAs.TemplateForSend(ctx, c.OARowID, c.TemplateRef); err != nil {
			setAll(results, rows, oaSourceResult(err))
			return
		}
		t, _, err := s.d.OAs.OpenAccessToken(ctx, c.OARowID)
		if err != nil {
			setAll(results, rows, oaSourceResult(err))
			return
		}
		token = t
		defer clear(token)
	} else {
		a, k, err := s.d.Accounts.OpenForSend(ctx, c.SmsAccountID)
		switch {
		case errors.Is(err, ErrSmsAccountNotFound):
			setAll(results, rows, failResult(domain.NoticeFailSourceRetired))
			return
		case err != nil:
			s.d.Log.WarnContext(ctx, "CẢNH BÁO: không mở được tài khoản SMS cho đợt tin — giữ chờ gửi", "service", "comms",
				"xa", string(tenant.MustFrom(ctx)), "dot", c.ID, "err", err)
			setAll(results, rows, retryResult(domain.NoticeFailUnavailable))
			return
		}
		account, creds = a, k
		defer clear(creds.APIKey)
		defer clear(creds.SecretKey)
	}

	// 2. Every row's number and message, one DEK read.
	sealed := make([][]byte, 0, 2*len(rows))
	aads := make([][]byte, 0, 2*len(rows))
	for _, d := range rows {
		sealed = append(sealed, d.PhoneSealed, d.PayloadSealed)
		aads = append(aads, noticePhoneAAD(ctx, d.ID), noticePayloadAAD(ctx, d.ID))
	}
	opened, err := s.d.Opener.OpenMany(ctx, sealed, aads)
	if err != nil {
		// No key in this deployment, no DEK, the database: ours — the rows stay owed.
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: không mở được số đã niêm của đợt tin — giữ chờ gửi", "service", "comms",
			"xa", string(tenant.MustFrom(ctx)), "dot", c.ID, "so_dong", len(rows), "err", err)
		setAll(results, rows, retryResult(domain.NoticeFailUnavailable))
		return
	}
	defer func() {
		for _, o := range opened {
			clear(o)
		}
	}()

	// 3. One provider call per row.
	broken := 0
	for i, d := range rows {
		phone, raw := opened[2*i], opened[2*i+1]
		if phone == nil || raw == nil {
			broken++
			results[d.ID] = retryResult(domain.NoticeFailUnavailable)
			continue
		}
		payload, err := decodeNoticePayload(raw)
		if err != nil || (c.Channel == domain.NoticeChannelSMS && payload.Text == "") {
			broken++
			results[d.ID] = retryResult(domain.NoticeFailUnavailable)
			continue
		}
		if ctx.Err() != nil {
			results[d.ID] = retryResult(domain.NoticeFailUnavailable)
			continue
		}
		if c.Channel == domain.NoticeChannelZNS {
			msgID, o, text := s.d.Zns.SendTemplateMessageReport(ctx, token, string(phone.Lo()), c.TemplateRef, payload.Params, d.ID)
			results[d.ID] = withProviderText(znsResult(msgID, o), text)
		} else {
			msgID, o, text := s.d.Sms.SendCSKHReport(ctx, creds, esms.Message{Phone: string(phone.Lo()), Content: payload.Text,
				Brandname: account.Brandname, IsUnicode: domain.SmsContentIsUnicode(payload.Text), RequestID: d.ID,
				CampaignID: c.ID})
			results[d.ID] = withProviderText(smsResult(d.ID, msgID, o), text)
		}
		clear(payload.Params)
	}
	if broken > 0 {
		s.d.Log.WarnContext(ctx, "CẢNH BÁO: có dòng số đã niêm không mở được — giữ chờ gửi", "service", "comms",
			"xa", string(tenant.MustFrom(ctx)), "dot", c.ID, "so_dong", broken)
	}
}
