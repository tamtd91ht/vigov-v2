package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The two handlers' status-code tables (identity.proto, ResolveNoticeAudience and
// RevealNoticeRecipientPhones). Selection, paging and the audit transaction are the use case's
// (internal/app/notice_audience_test.go); one test here runs the REAL use case over fake stores so the
// wire answer is pinned end to end for deduplication and skips.

type noticeAudienceFake struct {
	page        app.NoticeAudiencePage
	items       []domain.NoticeRecipient
	err         error
	resolves    int
	reveals     int
	gotAfter    string
	gotCount    bool
	gotAudience domain.NoticeAudience
	gotCodes    []string
	attrs       []domain.NoticeAttribute
	gotKind     domain.NoticeRecipientKind
}

func (f *noticeAudienceFake) Resolve(_ context.Context, a domain.NoticeAudience, after string, countOnly bool) (app.NoticeAudiencePage, error) {
	f.resolves++
	f.gotAudience, f.gotAfter, f.gotCount = a, after, countOnly
	return f.page, f.err
}

func (f *noticeAudienceFake) Reveal(_ context.Context, _ domain.NoticeRecipientKind, _ []string, _, _ string,
	codes []string) ([]domain.NoticeRecipient, error) {
	f.reveals++
	f.gotCodes = codes
	return f.items, f.err
}

func (f *noticeAudienceFake) Attributes(_ context.Context, kind domain.NoticeRecipientKind) ([]domain.NoticeAttribute, error) {
	f.gotKind = kind
	return f.attrs, f.err
}

type fieldsFake struct{}

func (fieldsFake) List(context.Context, bool) ([]domain.CitizenField, error) { return nil, nil }

// staffDirectoryFake is the staff store for the end-to-end case; Resolve never opens a transaction.
type staffDirectoryFake struct{ rows []domain.NoticeCandidate }

func (f staffDirectoryFake) NoticeAudienceCandidates(context.Context, domain.NoticeAudience) ([]domain.NoticeCandidate, error) {
	return f.rows, nil
}
func (staffDirectoryFake) NoticeRevealCandidates(context.Context, *pkgstore.ScopedTx, []string) ([]domain.NoticeCandidate, error) {
	return nil, errors.New("not used")
}
func (staffDirectoryFake) NoticeRevealAttributeSources(context.Context, *pkgstore.ScopedTx, []string) (map[string]domain.NoticeAttributeSource, error) {
	return nil, errors.New("not used")
}
func (staffDirectoryFake) StaffCodeExists(context.Context, *pkgstore.ScopedTx, string) (bool, error) {
	return false, errors.New("not used")
}

type citizenDirectoryFake struct{}

func (citizenDirectoryFake) NoticeAudienceCounts(context.Context, domain.NoticeAudience) (int, int, error) {
	return 0, 0, nil
}
func (citizenDirectoryFake) NoticeAudiencePage(context.Context, domain.NoticeAudience, string, int) ([]domain.NoticeCandidate, error) {
	return nil, nil
}
func (citizenDirectoryFake) NoticeRevealCandidates(context.Context, *pkgstore.ScopedTx, []string) ([]domain.NoticeCandidate, error) {
	return nil, nil
}
func (citizenDirectoryFake) NoticeRevealAttributeSources(context.Context, *pkgstore.ScopedTx, []string) (map[string]domain.NoticeAttributeSource, error) {
	return nil, nil
}

func staffAll() *identityv1.NoticeAudience {
	return &identityv1.NoticeAudience{Kind: identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_ALL}
}

// End to end through the real use case: two rows with one number are ONE recipient under the smaller
// ref, the locked and the number-less rows are counted not returned, every member is masked and of
// kind STAFF, and the full number appears nowhere in the response.
func TestNoticeResolveDedupAndSkipsOnTheWire(t *testing.T) {
	uc := app.NewNoticeAudience(pkgstore.New(nil), citizenDirectoryFake{}, staffDirectoryFake{rows: []domain.NoticeCandidate{
		{Ref: "CB-00002", Phone: phoneSample},
		{Ref: "CB-00001", Phone: phoneSample},
		{Ref: "CB-00003", Phone: "0900000003", Inactive: true},
		{Ref: "CB-00004", Phone: "không có"},
	}}, fieldsFake{})
	s, logs := may(t, func(d *Deps) { d.NoticeAudience = uc })

	resp, err := s.ResolveNoticeAudience(ctxXa(xaA), &identityv1.ResolveNoticeAudienceRequest{Audience: staffAll()})
	if err != nil {
		t.Fatalf("ResolveNoticeAudience: %v", err)
	}
	if resp.GetRecipientCount() != 1 || resp.GetSkippedInactiveCount() != 1 || resp.GetSkippedNoPhoneCount() != 1 {
		t.Fatalf("counts = %d/%d/%d, want 1/1/1", resp.GetRecipientCount(),
			resp.GetSkippedInactiveCount(), resp.GetSkippedNoPhoneCount())
	}
	if len(resp.GetRecipients()) != 1 {
		t.Fatalf("%d recipients, want 1", len(resp.GetRecipients()))
	}
	m := resp.GetRecipients()[0]
	if m.GetRef() != "CB-00001" || m.GetKind() != identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF ||
		m.GetMaskedPhone() != "09****0000" {
		t.Errorf("member = %v, want CB-00001, STAFF, 09****0000", m)
	}
	if resp.GetNextPageToken() != "" {
		t.Error("a one-page audience carries a next_page_token")
	}
	if strings.Contains(resp.String(), phoneSample) || strings.Contains(logs.String(), phoneSample) {
		t.Fatal("the full number reached the response or the log")
	}
}

// count_only answers the counts alone: no member, no token — even when the use case had members.
func TestNoticeResolveCountOnlyCarriesNoMemberAndNoToken(t *testing.T) {
	f := &noticeAudienceFake{page: app.NoticeAudiencePage{
		Members:        []app.NoticeAudienceMember{{Ref: "CB-00001", MaskedPhone: "09****0000"}},
		RecipientCount: 7, SkippedInactive: 2, NextAfter: "CB-00001",
	}}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })

	resp, err := s.ResolveNoticeAudience(ctxXa(xaA), &identityv1.ResolveNoticeAudienceRequest{
		Audience: staffAll(), CountOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !f.gotCount {
		t.Error("count_only was not passed to the use case")
	}
	if len(resp.GetRecipients()) != 0 || resp.GetNextPageToken() != "" || resp.GetRecipientCount() != 7 ||
		resp.GetSkippedInactiveCount() != 2 {
		t.Errorf("response = %v, want counts only", resp)
	}
}

// The token round-trips to the same keyset position, and is refused for another audience or forged.
func TestNoticeResolvePageTokenRoundTripAndRefusals(t *testing.T) {
	f := &noticeAudienceFake{page: app.NoticeAudiencePage{
		Members: []app.NoticeAudienceMember{{Ref: "CB-00001", MaskedPhone: "09****0000"}}, NextAfter: "CB-00001",
	}}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })
	ctx := ctxXa(xaA)

	units := &identityv1.NoticeAudience{
		Kind:       identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_UNIT,
		OrgUnitIds: []string{"BP-B", "BP-A"},
	}
	first, err := s.ResolveNoticeAudience(ctx, &identityv1.ResolveNoticeAudienceRequest{Audience: units})
	if err != nil || first.GetNextPageToken() == "" {
		t.Fatalf("first page: %v, token %q", err, first.GetNextPageToken())
	}
	if strings.Contains(first.GetNextPageToken(), "CB-00001") {
		t.Error("the token is not opaque")
	}
	// The same audience spelled in another order is the same audience.
	same := &identityv1.NoticeAudience{Kind: units.Kind, OrgUnitIds: []string{"BP-A", "BP-B", "BP-A"}}
	if _, err := s.ResolveNoticeAudience(ctx, &identityv1.ResolveNoticeAudienceRequest{
		Audience: same, PageToken: first.GetNextPageToken(),
	}); err != nil {
		t.Fatalf("second page: %v", err)
	}
	if f.gotAfter != "CB-00001" {
		t.Errorf("after = %q, want the keyset position CB-00001", f.gotAfter)
	}

	calls := f.resolves
	for name, req := range map[string]*identityv1.ResolveNoticeAudienceRequest{
		"another audience": {Audience: staffAll(), PageToken: first.GetNextPageToken()},
		"forged":           {Audience: units, PageToken: "bm90LWEtdG9rZW4"},
		"not base64":       {Audience: units, PageToken: "%%%"},
		"with count_only":  {Audience: units, PageToken: first.GetNextPageToken(), CountOnly: true},
	} {
		if _, err := s.ResolveNoticeAudience(ctx, req); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: code = %v, want InvalidArgument", name, status.Code(err))
		}
	}
	if f.resolves != calls {
		t.Error("the use case ran for a refused token")
	}
}

func TestNoticeResolveAudienceShapeIsInvalidArgument(t *testing.T) {
	f := &noticeAudienceFake{}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })
	many := make([]string, domain.MaxNoticeAudienceListEntries+1)
	for i := range many {
		many[i] = fmt.Sprintf("G%02d", i)
	}
	k := func(v identityv1.NoticeAudienceKind) identityv1.NoticeAudienceKind { return v }
	for name, a := range map[string]*identityv1.NoticeAudience{
		"missing audience":         nil,
		"kind unspecified":         {},
		"group without ids":        {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP)},
		"group with blank ids":     {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP), GroupIds: []string{"", " "}},
		"tag without tags":         {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_TAG)},
		"unit without ids":         {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_UNIT)},
		"all with tags":            {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_ALL), Tags: []string{"x"}},
		"group with unit ids":      {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP), GroupIds: []string{"G"}, OrgUnitIds: []string{"U"}},
		"staff all with group ids": {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_ALL), GroupIds: []string{"G"}},
		"51 group ids":             {Kind: k(identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP), GroupIds: many},
	} {
		_, err := s.ResolveNoticeAudience(ctxXa(xaA), &identityv1.ResolveNoticeAudienceRequest{Audience: a})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: code = %v, want InvalidArgument", name, status.Code(err))
		}
	}
	if f.resolves != 0 {
		t.Errorf("the use case ran %d times for refused audiences", f.resolves)
	}
}

// A store failure is INTERNAL — never "nobody" — and the cause stays in this service's log.
func TestNoticeResolveFailureIsInternal(t *testing.T) {
	f := &noticeAudienceFake{err: errors.New("db down")}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })
	_, err := s.ResolveNoticeAudience(ctxXa(xaA), &identityv1.ResolveNoticeAudienceRequest{Audience: staffAll()})
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "db down") {
		t.Errorf("err = %v, want Internal without the cause", err)
	}
}

func TestNoticeRevealMapsItemsAndNeverLogsThem(t *testing.T) {
	f := &noticeAudienceFake{items: []domain.NoticeRecipient{{Ref: "CB-00001", Phone: phoneSample}}}
	s, logs := may(t, func(d *Deps) { d.NoticeAudience = f })

	resp, err := s.RevealNoticeRecipientPhones(ctxXa(xaA), &identityv1.RevealNoticeRecipientPhonesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF, Refs: []string{"CB-00001"},
		CampaignRef: "NC-1", ActorCode: "CB-00123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetRef() != "CB-00001" || resp.GetItems()[0].GetPhone() != phoneSample {
		t.Errorf("items = %v", resp.GetItems())
	}
	if strings.Contains(logs.String(), phoneSample) {
		t.Error("the number reached the log")
	}
}

func TestNoticeRevealRefusals(t *testing.T) {
	over := make([]string, domain.MaxNoticeRevealRefs+1)
	for i := range over {
		over[i] = fmt.Sprintf("R%03d", i)
	}
	f := &noticeAudienceFake{}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })
	_, err := s.RevealNoticeRecipientPhones(ctxXa(xaA), &identityv1.RevealNoticeRecipientPhonesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN, Refs: over, CampaignRef: "NC-1", ActorCode: "CB-1",
	})
	if status.Code(err) != codes.InvalidArgument || f.reveals != 0 {
		t.Errorf("over the ceiling: code %v, use case ran %d times — want InvalidArgument before it", status.Code(err), f.reveals)
	}

	// The use case's shape refusal (kind, campaign_ref, actor_code) maps to InvalidArgument.
	f.err = errors.Join(app.ErrNoticeRevealInvalid, errors.New("thiếu loại người nhận"))
	_, err = s.RevealNoticeRecipientPhones(ctxXa(xaA), &identityv1.RevealNoticeRecipientPhonesRequest{Refs: []string{"x"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", status.Code(err))
	}

	// Anything else is Internal: nothing disclosed, the batch is retried.
	f.err = errors.New("audit write failed")
	_, err = s.RevealNoticeRecipientPhones(ctxXa(xaA), &identityv1.RevealNoticeRecipientPhonesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN, Refs: []string{"x"}, CampaignRef: "NC-1", ActorCode: "CB-1",
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("code = %v, want Internal", status.Code(err))
	}
}

// ADR 0101 #12: the requested codes reach the use case as sent, and each item's attributes reach the wire
// as the use case formatted them — never logged.
func TestNoticeRevealCarriesAttributes(t *testing.T) {
	f := &noticeAudienceFake{items: []domain.NoticeRecipient{{Ref: "01A", Phone: phoneSample,
		Attributes: map[string]string{"full_name": "Nguyễn Văn A", "hongheo": "Hộ nghèo"}}}}
	s, logs := may(t, func(d *Deps) { d.NoticeAudience = f })

	resp, err := s.RevealNoticeRecipientPhones(ctxXa(xaA), &identityv1.RevealNoticeRecipientPhonesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN, Refs: []string{"01A"},
		CampaignRef: "NC-1", ActorCode: "CB-00123", AttributeCodes: []string{"full_name", "hongheo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.gotCodes, ",") != "full_name,hongheo" {
		t.Errorf("codes passed = %v", f.gotCodes)
	}
	got := resp.GetItems()[0].GetAttributes()
	if len(got) != 2 || got["full_name"] != "Nguyễn Văn A" || got["hongheo"] != "Hộ nghèo" {
		t.Errorf("attributes = %v", got)
	}
	if strings.Contains(logs.String(), "Nguyễn Văn A") {
		t.Error("an attribute value reached the log")
	}
}

func TestNoticeRevealRefusesTooManyAttributeCodes(t *testing.T) {
	f := &noticeAudienceFake{}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })
	over := make([]string, domain.MaxNoticeAttributeCodes+1)
	for i := range over {
		over[i] = fmt.Sprintf("attr%02dx", i)
	}
	_, err := s.RevealNoticeRecipientPhones(ctxXa(xaA), &identityv1.RevealNoticeRecipientPhonesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN, Refs: []string{"01A"},
		CampaignRef: "NC-1", ActorCode: "CB-1", AttributeCodes: over,
	})
	if status.Code(err) != codes.InvalidArgument || f.reveals != 0 {
		t.Errorf("code %v, use case ran %d times — want InvalidArgument before it", status.Code(err), f.reveals)
	}
}

func TestListNoticeAttributes(t *testing.T) {
	f := &noticeAudienceFake{attrs: []domain.NoticeAttribute{{Code: "full_name", Label: "Họ và tên"}, {Code: "hongheo", Label: "Hộ nghèo"}}}
	s, _ := may(t, func(d *Deps) { d.NoticeAudience = f })

	resp, err := s.ListNoticeAttributes(ctxXa(xaA), &identityv1.ListNoticeAttributesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotKind != domain.NoticeRecipientCitizen || len(resp.GetAttributes()) != 2 ||
		resp.GetAttributes()[1].GetCode() != "hongheo" || resp.GetAttributes()[1].GetLabel() != "Hộ nghèo" {
		t.Errorf("kind %v, attributes %v", f.gotKind, resp.GetAttributes())
	}

	if _, err := s.ListNoticeAttributes(ctxXa(xaA), &identityv1.ListNoticeAttributesRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("kind unspecified: code %v, want InvalidArgument", status.Code(err))
	}
	f.err = errors.New("db down")
	if _, err := s.ListNoticeAttributes(ctxXa(xaA), &identityv1.ListNoticeAttributesRequest{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF,
	}); status.Code(err) != codes.Internal {
		t.Errorf("store failure: code %v, want Internal", status.Code(err))
	}
}
