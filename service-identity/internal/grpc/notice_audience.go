package grpc

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ResolveNoticeAudience and RevealNoticeRecipientPhones — WHO a notice campaign reaches, and the full
// numbers its send job needs (ADR 0099 #5). The contract — two RPCs not one with a flag, the predicate,
// deduplication by number, the 10 000 page and the 500 batch, the audit entry — is on the two RPCs in
// identity.proto; this file implements their status-code tables.
//
// NEVER LOG EITHER REQUEST OR RESPONSE. The generated String() prints `phone` in full and
// `debug_redact` is a no-op in protobuf-go v1.36.12; tags are typed by a commune; a ref turns into a
// person for anyone holding the directory. The one line this file can emit is Server.loi's, whose
// error carries the commune and no value (rule 3).

// NoticeAudienceService is the use case behind both RPCs — *app.NoticeAudience in production. A USE
// CASE, not a store: Reveal's read and its audit entry share one transaction, and the numbers leave
// only after the entry commits (rule 6, invariants 3 and 7).
type NoticeAudienceService interface {
	Resolve(ctx context.Context, a domain.NoticeAudience, after string, countOnly bool) (app.NoticeAudiencePage, error)
	Reveal(ctx context.Context, kind domain.NoticeRecipientKind, refs []string, campaignRef, actorCode string) ([]domain.NoticeRecipient, error)
}

func (s *Server) ResolveNoticeAudience(ctx context.Context, req *identityv1.ResolveNoticeAudienceRequest) (
	*identityv1.ResolveNoticeAudienceResponse, error) {

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	aud, err := domain.CheckNoticeAudience(audienceFromProto(req.GetAudience()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.GetCountOnly() && req.GetPageToken() != "" {
		return nil, status.Error(codes.InvalidArgument, "count_only không đi cùng page_token")
	}
	after := ""
	if t := req.GetPageToken(); t != "" {
		if after, err = decodeNoticePageToken(t, aud); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	page, err := s.d.NoticeAudience.Resolve(ctx, aud, after, req.GetCountOnly())
	if err != nil {
		if errors.Is(err, domain.ErrNoticeAudienceInvalid) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		// INTERNAL, never "nobody": the caller answers 503 and writes NO campaign (identity.proto).
		return nil, s.loi(ctx, err, "ResolveNoticeAudience")
	}

	kind := recipientKindToProto(aud.Kind.RecipientKind())
	out := &identityv1.ResolveNoticeAudienceResponse{
		RecipientCount:       uint32(page.RecipientCount),
		SkippedInactiveCount: uint32(page.SkippedInactive),
		SkippedNoPhoneCount:  uint32(page.SkippedNoPhone),
	}
	if req.GetCountOnly() {
		return out, nil // counts only — never a member, never a token
	}
	out.Recipients = make([]*identityv1.NoticeAudienceMember, 0, len(page.Members))
	for _, m := range page.Members {
		// Field by field: the masked number and the ref, nothing a mapper could widen.
		out.Recipients = append(out.Recipients, &identityv1.NoticeAudienceMember{
			Kind: kind, Ref: m.Ref, MaskedPhone: m.MaskedPhone,
		})
	}
	if page.NextAfter != "" {
		out.NextPageToken = encodeNoticePageToken(aud, page.NextAfter)
	}
	return out, nil
}

func (s *Server) RevealNoticeRecipientPhones(ctx context.Context, req *identityv1.RevealNoticeRecipientPhonesRequest) (
	*identityv1.RevealNoticeRecipientPhonesResponse, error) {

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}
	kind := recipientKindFromProto(req.GetKind())
	// The ceiling on what was SENT, before duplicates collapse — refused, never truncated.
	if n := len(req.GetRefs()); n > domain.MaxNoticeRevealRefs {
		return nil, status.Errorf(codes.InvalidArgument,
			"refs có %d mục, vượt trần %d — bên gọi phải chia lô", n, domain.MaxNoticeRevealRefs)
	}

	items, err := s.d.NoticeAudience.Reveal(ctx, kind, req.GetRefs(), req.GetCampaignRef(), req.GetActorCode())
	if err != nil {
		if errors.Is(err, app.ErrNoticeRevealInvalid) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		// INTERNAL: nothing disclosed; the batch stays `cho-gui` and the job retries (identity.proto).
		return nil, s.loi(ctx, err, "RevealNoticeRecipientPhones")
	}
	out := &identityv1.RevealNoticeRecipientPhonesResponse{
		Items: make([]*identityv1.NoticeRecipientPhone, 0, len(items)),
	}
	for _, it := range items {
		out.Items = append(out.Items, &identityv1.NoticeRecipientPhone{Ref: it.Ref, Phone: it.Phone})
	}
	return out, nil
}

func audienceFromProto(a *identityv1.NoticeAudience) domain.NoticeAudience {
	var k domain.NoticeAudienceKind
	switch a.GetKind() {
	case identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_ALL:
		k = domain.NoticeAudienceCitizenAll
	case identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP:
		k = domain.NoticeAudienceCitizenGroup
	case identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_TAG:
		k = domain.NoticeAudienceCitizenTag
	case identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_ALL:
		k = domain.NoticeAudienceStaffAll
	case identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_UNIT:
		k = domain.NoticeAudienceStaffUnit
	}
	return domain.NoticeAudience{
		Kind: k, GroupIDs: a.GetGroupIds(), Tags: a.GetTags(), OrgUnitIDs: a.GetOrgUnitIds(),
	}
}

func recipientKindFromProto(k identityv1.NoticeRecipientKind) domain.NoticeRecipientKind {
	switch k {
	case identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN:
		return domain.NoticeRecipientCitizen
	case identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF:
		return domain.NoticeRecipientStaff
	}
	return domain.NoticeRecipientUnspecified
}

func recipientKindToProto(k domain.NoticeRecipientKind) identityv1.NoticeRecipientKind {
	switch k {
	case domain.NoticeRecipientCitizen:
		return identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN
	case domain.NoticeRecipientStaff:
		return identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF
	}
	return identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_UNSPECIFIED
}

// The page token is a keyset POSITION — a ref, never a number — bound to the audience it was minted
// for by the audience's one-way fingerprint, so a token replayed against another audience is refused
// instead of silently skipping that audience's first refs. Not a secret and not signed: a forged token
// can only skip ahead inside the caller's own audience, in the caller's own commune.
const noticePageTokenPrefix = "na1."

var errNoticePageToken = errors.New("page_token không do máy chủ này cấp cho đối tượng này")

func encodeNoticePageToken(a domain.NoticeAudience, after string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(noticePageTokenPrefix + a.Fingerprint() + "." + after))
}

func decodeNoticePageToken(tok string, a domain.NoticeAudience) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return "", errNoticePageToken
	}
	rest, ok := strings.CutPrefix(string(raw), noticePageTokenPrefix)
	if !ok {
		return "", errNoticePageToken
	}
	fp, after, ok := strings.Cut(rest, ".")
	if !ok || fp != a.Fingerprint() || after == "" {
		return "", errNoticePageToken
	}
	return after, nil
}
