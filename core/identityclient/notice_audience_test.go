package identityclient

// What these tests defend on THIS side of the wire: the page walk is all-or-nothing and follows the
// token, count_only sends no token and returns counts only, Reveal maps only asked refs, an outage is
// an error (never "nobody" / never "bo-qua"), the commune rides in metadata, and no number reaches a
// log or an error.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

const noticePhone = "0900000000" // the agreed fake number (rule 3, invariant 5)

type noticeServer struct {
	identityv1.UnimplementedIdentityServiceServer

	pages    map[string]*identityv1.ResolveNoticeAudienceResponse // by page_token
	failAt   string                                               // a token whose call fails
	items    []*identityv1.NoticeRecipientPhone
	err      error
	resolved []*identityv1.ResolveNoticeAudienceRequest
	revealed []*identityv1.RevealNoticeRecipientPhonesRequest
	seenXa   []string
	attrs    []*identityv1.NoticeAttribute
	listed   []*identityv1.ListNoticeAttributesRequest
}

func (s *noticeServer) ResolveNoticeAudience(ctx context.Context, in *identityv1.ResolveNoticeAudienceRequest) (
	*identityv1.ResolveNoticeAudienceResponse, error) {
	s.resolved = append(s.resolved, in)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.seenXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.failAt != "" && in.GetPageToken() == s.failAt {
		return nil, status.Error(codes.Unavailable, "identity down")
	}
	return s.pages[in.GetPageToken()], nil
}

func (s *noticeServer) RevealNoticeRecipientPhones(ctx context.Context, in *identityv1.RevealNoticeRecipientPhonesRequest) (
	*identityv1.RevealNoticeRecipientPhonesResponse, error) {
	s.revealed = append(s.revealed, in)
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.RevealNoticeRecipientPhonesResponse{Items: s.items}, nil
}

func member(ref string) *identityv1.NoticeAudienceMember {
	return &identityv1.NoticeAudienceMember{
		Kind: identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN, Ref: ref, MaskedPhone: "09****0000",
	}
}

func twoPages() map[string]*identityv1.ResolveNoticeAudienceResponse {
	return map[string]*identityv1.ResolveNoticeAudienceResponse{
		"": {Recipients: []*identityv1.NoticeAudienceMember{member("01A"), member("01B")},
			RecipientCount: 3, SkippedInactiveCount: 1, NextPageToken: "t1"},
		"t1": {Recipients: []*identityv1.NoticeAudienceMember{member("01C")},
			RecipientCount: 3, SkippedInactiveCount: 1},
	}
}

func TestResolveNoticeAudienceWalksEveryPage(t *testing.T) {
	srv := &noticeServer{pages: twoPages()}
	c := moMay(t, srv)

	got, counts, err := c.ResolveNoticeAudience(ngucCanh(), NoticeAudience{Kind: NoticeAudienceCitizenGroup, GroupIDs: []string{"G1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Ref != "01A" || got[2].Ref != "01C" || got[0].Kind != NoticeRecipientCitizen ||
		got[0].MaskedPhone != "09****0000" {
		t.Errorf("members = %+v", got)
	}
	if counts != (NoticeAudienceCounts{Recipients: 3, SkippedInactive: 1}) {
		t.Errorf("counts = %+v", counts)
	}
	if len(srv.resolved) != 2 || srv.resolved[1].GetPageToken() != "t1" ||
		srv.resolved[0].GetAudience().GetKind() != identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP ||
		srv.resolved[0].GetAudience().GetGroupIds()[0] != "G1" {
		t.Errorf("requests = %v", srv.resolved)
	}
	if len(srv.seenXa) != 1 || srv.seenXa[0] != string(xaA) {
		t.Errorf("x-tenant-id = %v, want %q", srv.seenXa, string(xaA))
	}
}

// ALL OR NOTHING: a failed second page returns no member — a campaign is never written with half its
// audience — and the transient code carries ErrIdentityUnavailable.
func TestResolveNoticeAudienceFailedPageReturnsNothing(t *testing.T) {
	c := moMay(t, &noticeServer{pages: twoPages(), failAt: "t1"})
	got, _, err := c.ResolveNoticeAudience(ngucCanh(), NoticeAudience{Kind: NoticeAudienceCitizenAll})
	if !errors.Is(err, ErrIdentityUnavailable) || got != nil {
		t.Fatalf("got %d members, err %v — want none and ErrIdentityUnavailable", len(got), err)
	}
}

func TestResolveNoticeAudienceRepeatedTokenIsAnError(t *testing.T) {
	pages := map[string]*identityv1.ResolveNoticeAudienceResponse{
		"":   {NextPageToken: "t1"},
		"t1": {NextPageToken: "t1"},
	}
	c := moMay(t, &noticeServer{pages: pages})
	if _, _, err := c.ResolveNoticeAudience(ngucCanh(), NoticeAudience{Kind: NoticeAudienceStaffAll}); err == nil {
		t.Fatal("a token that never ends was walked as an answer")
	}
}

func TestCountNoticeAudienceSendsCountOnly(t *testing.T) {
	srv := &noticeServer{pages: map[string]*identityv1.ResolveNoticeAudienceResponse{
		"": {RecipientCount: 9, SkippedInactiveCount: 2, SkippedNoPhoneCount: 1},
	}}
	c := moMay(t, srv)
	counts, err := c.CountNoticeAudience(ngucCanh(), NoticeAudience{Kind: NoticeAudienceStaffUnit, OrgUnitIDs: []string{"U1"}})
	if err != nil {
		t.Fatal(err)
	}
	if counts != (NoticeAudienceCounts{Recipients: 9, SkippedInactive: 2, SkippedNoPhone: 1}) {
		t.Errorf("counts = %+v", counts)
	}
	if !srv.resolved[0].GetCountOnly() || srv.resolved[0].GetPageToken() != "" {
		t.Errorf("request = %v, want count_only without a token", srv.resolved[0])
	}
}

func TestRevealNoticeRecipientPhonesMapsAskedRefsOnly(t *testing.T) {
	srv := &noticeServer{items: []*identityv1.NoticeRecipientPhone{
		{Ref: "CB-00001", Phone: noticePhone},
		{Ref: "CB-99999", Phone: "0900000009"}, // never asked — dropped
	}}
	c := moMay(t, srv)
	got, err := c.RevealNoticeRecipientPhones(ngucCanh(), NoticeRecipientStaff, []string{"CB-00001", "CB-00002"}, "NC-1", "CB-00123")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["CB-00001"] != noticePhone {
		t.Errorf("got %v, want only CB-00001", got)
	}
	r := srv.revealed[0]
	if r.GetKind() != identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF || r.GetCampaignRef() != "NC-1" ||
		r.GetActorCode() != "CB-00123" || len(r.GetRefs()) != 2 {
		t.Errorf("request = kind %v campaign %q actor %q refs %d", r.GetKind(), r.GetCampaignRef(), r.GetActorCode(), len(r.GetRefs()))
	}
}

// An outage is an error with its code, never an empty map (which the caller would read as "bo-qua"),
// and the log line names the code only.
func TestRevealNoticeRecipientPhonesOutageKeepsCodeAndLogsNothingPersonal(t *testing.T) {
	c := moMay(t, &noticeServer{err: status.Error(codes.Unavailable, "identity down")})
	var buf bytes.Buffer
	c.log = slog.New(slog.NewTextHandler(&buf, nil))

	got, err := c.RevealNoticeRecipientPhones(ngucCanh(), NoticeRecipientCitizen, []string{"01A"}, "NC-1", "CB-00123")
	if status.Code(err) != codes.Unavailable || !errors.Is(err, ErrIdentityUnavailable) || got != nil {
		t.Fatalf("got %v, err %v — want nil and Unavailable", got, err)
	}
	for _, leak := range []string{"01A", "CB-00123", noticePhone} {
		if strings.Contains(buf.String(), leak) || strings.Contains(err.Error(), leak) {
			t.Errorf("%q leaked: log=%q err=%v", leak, buf.String(), err)
		}
	}
}

func TestRevealNoticeRecipientPhonesRefusesLocally(t *testing.T) {
	over := make([]string, MaxNoticeRevealRefs+1)
	for name, call := range map[string]func(c *Client) error{
		"over the ceiling": func(c *Client) error {
			_, err := c.RevealNoticeRecipientPhones(ngucCanh(), NoticeRecipientCitizen, over, "NC-1", "CB-1")
			return err
		},
		"no campaign": func(c *Client) error {
			_, err := c.RevealNoticeRecipientPhones(ngucCanh(), NoticeRecipientCitizen, []string{"x"}, "", "CB-1")
			return err
		},
		"no actor": func(c *Client) error {
			_, err := c.RevealNoticeRecipientPhones(ngucCanh(), NoticeRecipientCitizen, []string{"x"}, "NC-1", "")
			return err
		},
		"no kind": func(c *Client) error {
			_, err := c.RevealNoticeRecipientPhones(ngucCanh(), 0, []string{"x"}, "NC-1", "CB-1")
			return err
		},
	} {
		srv := &noticeServer{}
		if err := call(moMay(t, srv)); err == nil {
			t.Errorf("%s: no error", name)
		}
		if len(srv.revealed) != 0 {
			t.Errorf("%s: reached the server", name)
		}
	}
	// No refs: an empty map, no call.
	srv := &noticeServer{}
	got, err := moMay(t, srv).RevealNoticeRecipientPhones(ngucCanh(), NoticeRecipientCitizen, nil, "NC-1", "CB-1")
	if err != nil || len(got) != 0 || len(srv.revealed) != 0 {
		t.Errorf("empty refs = %v, %v, %d calls — want empty, no call", got, err, len(srv.revealed))
	}
}

// NO COMMUNE, NO CALL.
func TestNoticeAudienceWithoutCommuneIsRefusedLocally(t *testing.T) {
	srv := &noticeServer{}
	_, err := moMay(t, srv).CountNoticeAudience(context.Background(), NoticeAudience{Kind: NoticeAudienceCitizenAll})
	if status.Code(err) != codes.InvalidArgument || len(srv.resolved) != 0 {
		t.Errorf("code = %v, calls %d — want InvalidArgument before the wire", status.Code(err), len(srv.resolved))
	}
}

func (s *noticeServer) ListNoticeAttributes(_ context.Context, in *identityv1.ListNoticeAttributesRequest) (
	*identityv1.ListNoticeAttributesResponse, error) {
	s.listed = append(s.listed, in)
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ListNoticeAttributesResponse{Attributes: s.attrs}, nil
}

// ADR 0101 #12: the codes go on the wire; only REQUESTED codes come back, one entry each ("" when the
// server sent none), whatever else the server added.
func TestRevealNoticeRecipientsCarriesRequestedAttributesOnly(t *testing.T) {
	srv := &noticeServer{items: []*identityv1.NoticeRecipientPhone{
		{Ref: "01A", Phone: noticePhone, Attributes: map[string]string{"full_name": "Nguyễn Văn A", "address": "không hỏi"}},
	}}
	got, err := moMay(t, srv).RevealNoticeRecipients(ngucCanh(), NoticeRecipientCitizen, []string{"01A"}, "NC-1", "CB-1",
		[]string{"full_name", "hongheo"})
	if err != nil {
		t.Fatal(err)
	}
	r := got["01A"]
	if r.Phone != noticePhone || len(r.Attributes) != 2 || r.Attributes["full_name"] != "Nguyễn Văn A" || r.Attributes["hongheo"] != "" {
		t.Errorf("got %+v", r)
	}
	if strings.Join(srv.revealed[0].GetAttributeCodes(), ",") != "full_name,hongheo" {
		t.Errorf("attribute_codes sent = %v", srv.revealed[0].GetAttributeCodes())
	}
	over := make([]string, MaxNoticeAttributeCodes+1)
	if _, err := moMay(t, &noticeServer{}).RevealNoticeRecipients(ngucCanh(), NoticeRecipientCitizen, []string{"01A"},
		"NC-1", "CB-1", over); err == nil {
		t.Error("21 attribute codes were sent")
	}
}

func TestListNoticeAttributes(t *testing.T) {
	srv := &noticeServer{attrs: []*identityv1.NoticeAttribute{{Code: "full_name", Label: "Họ và tên"}}}
	got, err := moMay(t, srv).ListNoticeAttributes(ngucCanh(), NoticeRecipientStaff)
	if err != nil || len(got) != 1 || got[0] != (NoticeAttribute{Code: "full_name", Label: "Họ và tên"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if srv.listed[0].GetKind() != identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF {
		t.Errorf("kind sent = %v", srv.listed[0].GetKind())
	}
	if _, err := moMay(t, &noticeServer{err: status.Error(codes.Unavailable, "down")}).ListNoticeAttributes(ngucCanh(),
		NoticeRecipientCitizen); !errors.Is(err, ErrIdentityUnavailable) {
		t.Errorf("outage: %v, want ErrIdentityUnavailable", err)
	}
	if _, err := moMay(t, &noticeServer{}).ListNoticeAttributes(ngucCanh(), 0); err == nil {
		t.Error("no kind: no error")
	}
}
