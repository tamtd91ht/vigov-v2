package store

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The campaign SQL (migration 0032) against a REAL PostgreSQL: the campaign + recipients insert, the list
// join and keyset, the claim (lease + attempt), the four outcomes, the settle, and isolation between two
// communes. SKIPPED WITHOUT VIGOV_TEST_DSN — a green run without the DSN means this file COMPILES,
// nothing more.

var noticeAt = time.Date(2026, 10, 17, 4, 0, 0, 0, time.UTC)

func TestPgNoticeCampaignLifecycle(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	h := pkgstore.New(db)
	ctx1, ctx2 := ctxXa(tenant.ID(c1)), ctxXa(tenant.ID(c2))
	accounts, campaigns := NewSmsAccountStore(h), NewNoticeCampaignStore(h)
	run := func(ctx context.Context, fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx).Tx(ctx, fn) }
	const accID, campID = "01JNOTICEPGACC000000000001", "01JNOTICEPGCAMP00000000001"

	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return accounts.InsertLive(ctx1, tx, smsAccountRow(accID, "eSMS chính", true), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("account: %v", err)
	}
	c := domain.NoticeCampaign{ID: campID, Title: "Họp dân", Channel: domain.NoticeChannelSMS, SmsAccountID: accID,
		TemplateRef: "TPL1", TemplateName: "Mẫu họp", TemplateContent: "Moi {P1,20} hop", Params: map[string]string{"1": "ba con"},
		Audience:       domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenAll, IDs: []string{}},
		RecipientCount: 3, SkippedCount: 1, UnitPrice: "350.00", EstimatedCost: "1050.00", CreatedAt: noticeAt, CreatedBy: "CB-00123"}
	rs := []domain.NoticeRecipient{
		{ID: "01JNOTICEPGROW000000000001", Ref: "C1", Kind: domain.NoticeRecipientCitizen, PhoneMasked: "09****0001"},
		{ID: "01JNOTICEPGROW000000000002", Ref: "C2", Kind: domain.NoticeRecipientCitizen, PhoneMasked: "09****0002"},
		{ID: "01JNOTICEPGROW000000000003", Ref: "C3", Kind: domain.NoticeRecipientCitizen, PhoneMasked: "09****0003"},
	}
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		if err := campaigns.InsertCampaign(ctx1, tx, c); err != nil {
			return err
		}
		return campaigns.InsertRecipients(ctx1, tx, campID, rs, noticeAt)
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// A FULL number is refused by the database (0032's mask CHECK), whatever Go does.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return campaigns.InsertRecipients(ctx1, tx, campID, []domain.NoticeRecipient{{ID: "01JNOTICEPGROW000000000009",
			Ref: "C9", Kind: domain.NoticeRecipientCitizen, PhoneMasked: "0900000000"}}, noticeAt)
	}); err == nil {
		t.Fatal("a full number was stored")
	}

	page, err := campaigns.List(ctx1, domain.NoticeCursor{}, 50)
	if err != nil || len(page) != 1 || page[0].AccountLabel != "eSMS chính" || page[0].Campaign.UnitPrice != "350.00" ||
		page[0].Campaign.Params["1"] != "ba con" {
		t.Fatalf("list = %+v %v", page, err)
	}
	if other, _ := campaigns.List(ctx2, domain.NoticeCursor{}, 50); len(other) != 0 {
		t.Fatal("commune 2 lists commune 1's campaign")
	}
	if _, found, _ := campaigns.Get(ctx2, campID); found {
		t.Fatal("commune 2 reads commune 1's campaign")
	}

	var due []domain.DueNoticeRecipient
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) (err error) {
		due, err = campaigns.ClaimDue(ctx1, tx, noticeAt, noticeAt.Add(15*time.Minute), 500)
		return err
	}); err != nil || len(due) != 3 || due[0].Attempts != 1 {
		t.Fatalf("claim = %+v %v", due, err)
	}
	// Leased: a second claim before the lease ends takes nothing.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		again, err := campaigns.ClaimDue(ctx1, tx, noticeAt.Add(time.Minute), noticeAt.Add(16*time.Minute), 500)
		if err == nil && len(again) != 0 {
			t.Errorf("claimed a leased row: %+v", again)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		if err := campaigns.MarkSent(ctx1, tx, rs[0].ID, "SMS123", noticeAt); err != nil {
			return err
		}
		if err := campaigns.Fail(ctx1, tx, rs[1].ID, domain.NoticeFailRejected, noticeAt); err != nil {
			return err
		}
		if err := campaigns.Retry(ctx1, tx, rs[2].ID, noticeAt.Add(time.Minute), noticeAt); err != nil {
			return err
		}
		var err error
		status, err = campaigns.Settle(ctx1, tx, campID, noticeAt)
		return err
	}); err != nil || status != domain.NoticeCampaignSending {
		t.Fatalf("first settle = %q %v", status, err)
	}
	// A finished row keeps its outcome.
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		return campaigns.Fail(ctx1, tx, rs[0].ID, domain.NoticeFailRejected, noticeAt)
	}); !errors.Is(err, ErrNoticeRowChanged) {
		t.Fatalf("re-settling a sent row: %v", err)
	}
	if err := run(ctx1, func(tx *pkgstore.ScopedTx) error {
		if err := campaigns.Skip(ctx1, tx, rs[2].ID, domain.NoticeFailRecipientInactive, noticeAt); err != nil {
			return err
		}
		var err error
		status, err = campaigns.Settle(ctx1, tx, campID, noticeAt.Add(time.Minute))
		return err
	}); err != nil || status != domain.NoticeCampaignPartlySent {
		t.Fatalf("final settle = %q %v", status, err)
	}
	got, found, err := campaigns.Get(ctx1, campID)
	if err != nil || !found || got.Campaign.SentCount != 1 || got.Campaign.FailedCount != 1 || got.Campaign.FinishedAt.IsZero() {
		t.Fatalf("after = %+v %v %v", got, found, err)
	}
	if byID, err := campaigns.CampaignsByID(ctx1, []string{campID, "nope"}); err != nil || len(byID) != 1 {
		t.Fatalf("by id = %v %v", byID, err)
	}
}
