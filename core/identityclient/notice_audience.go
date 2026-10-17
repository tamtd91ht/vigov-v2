package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// The notice-campaign pair (ADR 0099 #5): ResolveNoticeAudience — who a campaign reaches, refs and
// MASKED numbers — and RevealNoticeRecipientPhones — the FULL numbers of refs a campaign already holds,
// audited by identity. The contract (why two RPCs, the predicate, the ceilings) is on the two RPCs in
// proto/vigov/identity/v1/identity.proto. ONE INTENDED CALLER: service-comms.
//
// Plain Go types in and out, so the caller never touches the generated messages — whose String()
// prints the full number.
//
// NEVER LOGGED, AT EITHER END (rule 3). Neither request nor answer is passed to a logger and no ref,
// tag or number is put into an error; the warnings below carry the gRPC code and nothing else.

// NoticeAudienceKind is the `Gửi cho ai` choice. Zero is invalid.
type NoticeAudienceKind int

const (
	NoticeAudienceCitizenAll NoticeAudienceKind = iota + 1
	NoticeAudienceCitizenGroup
	NoticeAudienceCitizenTag
	NoticeAudienceStaffAll
	NoticeAudienceStaffUnit
)

// NoticeRecipientKind is the directory a ref points into. Zero is invalid.
type NoticeRecipientKind int

const (
	// NoticeRecipientCitizen — the ref is a citizen contact id.
	NoticeRecipientCitizen NoticeRecipientKind = iota + 1
	// NoticeRecipientStaff — the ref is a staff business code (CB-…).
	NoticeRecipientStaff
)

// NoticeAudience is one audience. Exactly the list its Kind uses is non-empty (GroupIDs for
// CITIZEN_GROUP, Tags for CITIZEN_TAG, OrgUnitIDs for STAFF_UNIT); at most 50 entries each.
type NoticeAudience struct {
	Kind       NoticeAudienceKind
	GroupIDs   []string
	Tags       []string
	OrgUnitIDs []string
}

// NoticeAudienceCounts are the counts of the WHOLE audience. Recipients is after deduplication by
// number: the messages that will be sent.
type NoticeAudienceCounts struct {
	Recipients      int
	SkippedInactive int
	SkippedNoPhone  int
}

// NoticeAudienceMember is one recipient: the ref RevealNoticeRecipientPhones takes and the MASKED
// number — the only form of the number comms may store.
type NoticeAudienceMember struct {
	Kind        NoticeRecipientKind
	Ref         string
	MaskedPhone string
}

// MaxNoticeRevealRefs is the contract's ceiling on one RevealNoticeRecipientPhones call.
const MaxNoticeRevealRefs = 500

// maxNoticeAudiencePages bounds the page walk: 1 000 pages of 10 000 is ten million recipients, far
// past any commune. A walk that reaches it is a server handing out tokens that never end.
const maxNoticeAudiencePages = 1000

// CountNoticeAudience answers the preview's three counts (count_only) — nothing else crosses the wire.
//
// err != nil is "the call did not happen" (ErrIdentityUnavailable for the transient codes), NEVER zero:
// the preview says the count could not be loaded.
func (c *Client) CountNoticeAudience(ctx context.Context, a NoticeAudience) (NoticeAudienceCounts, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	resp, err := c.cl.ResolveNoticeAudience(ctx, &identityv1.ResolveNoticeAudienceRequest{
		Audience: audienceToProto(a), CountOnly: true,
	})
	if err != nil {
		c.warnNotice(ctx, "ResolveNoticeAudience", err)
		return NoticeAudienceCounts{}, wrapCallError("ResolveNoticeAudience", err, nil)
	}
	return countsOf(resp), nil
}

// ResolveNoticeAudience walks EVERY page of the audience and returns all members, ordered by ref, with
// the counts of the whole audience — what POST notice-campaigns writes in one transaction.
//
// ALL OR NOTHING: a failure on any page is an error and no member is returned, so the caller never
// writes a campaign holding part of its audience. Zero members with a nil error is an ordinary answer
// (an empty group) — never widened to "everyone".
func (c *Client) ResolveNoticeAudience(ctx context.Context, a NoticeAudience) ([]NoticeAudienceMember, NoticeAudienceCounts, error) {
	var (
		members []NoticeAudienceMember
		counts  NoticeAudienceCounts
		token   string
	)
	for page := 0; ; page++ {
		if page == maxNoticeAudiencePages {
			return nil, NoticeAudienceCounts{}, fmt.Errorf("identityclient: ResolveNoticeAudience: quá %d trang — "+
				"máy chủ trả con trỏ trang không dứt", maxNoticeAudiencePages)
		}
		resp, err := c.resolveNoticePage(ctx, a, token)
		if err != nil {
			c.warnNotice(ctx, "ResolveNoticeAudience", err)
			return nil, NoticeAudienceCounts{}, wrapCallError("ResolveNoticeAudience", err, nil)
		}
		if page == 0 {
			counts = countsOf(resp)
			members = make([]NoticeAudienceMember, 0, counts.Recipients)
		}
		for _, m := range resp.GetRecipients() {
			members = append(members, NoticeAudienceMember{
				Kind: recipientKindFromProto(m.GetKind()), Ref: m.GetRef(), MaskedPhone: m.GetMaskedPhone(),
			})
		}
		next := resp.GetNextPageToken()
		if next == "" {
			return members, counts, nil
		}
		if next == token {
			return nil, NoticeAudienceCounts{}, fmt.Errorf("identityclient: ResolveNoticeAudience: " +
				"máy chủ trả lại đúng con trỏ trang vừa gửi")
		}
		token = next
	}
}

// resolveNoticePage is one page, under the client's per-call deadline.
func (c *Client) resolveNoticePage(ctx context.Context, a NoticeAudience, token string) (*identityv1.ResolveNoticeAudienceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	return c.cl.ResolveNoticeAudience(ctx, &identityv1.ResolveNoticeAudienceRequest{
		Audience: audienceToProto(a), PageToken: token,
	})
}

// RevealNoticeRecipientPhones returns the FULL national number ("0…") of each deliverable ref, keyed by
// ref. Identity has already written the audit entry when this returns. A ref ABSENT from the map is not
// deliverable now (removed, switched off, locked, no number, duplicate) — the caller marks it `bo-qua`.
//
// err != nil is "the call did not happen": the batch stays `cho-gui` and is retried — NEVER `bo-qua`.
//
// HOLD THE NUMBERS ONLY FOR THE PROVIDER CALL: never stored, never logged, never in an error, a metric
// label, an outbox row or a dead letter (ADR 0099 #5).
func (c *Client) RevealNoticeRecipientPhones(ctx context.Context, kind NoticeRecipientKind, refs []string,
	campaignRef, actorCode string) (map[string]string, error) {

	// REFUSED LOCALLY: identity answers INVALID_ARGUMENT for each, and an operator reading that would
	// inspect the contract for a fault in this caller's batching.
	switch {
	case len(refs) > MaxNoticeRevealRefs:
		return nil, fmt.Errorf("identityclient: RevealNoticeRecipientPhones: %d ref, vượt trần %d — chia lô",
			len(refs), MaxNoticeRevealRefs)
	case campaignRef == "" || actorCode == "":
		return nil, fmt.Errorf("identityclient: RevealNoticeRecipientPhones: thiếu campaign_ref hoặc actor_code")
	case kind != NoticeRecipientCitizen && kind != NoticeRecipientStaff:
		return nil, fmt.Errorf("identityclient: RevealNoticeRecipientPhones: thiếu loại người nhận")
	case len(refs) == 0:
		return map[string]string{}, nil // nothing asked, nothing disclosed, no call
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	resp, err := c.cl.RevealNoticeRecipientPhones(ctx, &identityv1.RevealNoticeRecipientPhonesRequest{
		Kind: recipientKindToProto(kind), Refs: refs, CampaignRef: campaignRef, ActorCode: actorCode,
	})
	if err != nil {
		c.warnNotice(ctx, "RevealNoticeRecipientPhones", err)
		return nil, wrapCallError("RevealNoticeRecipientPhones", err, nil)
	}
	asked := make(map[string]bool, len(refs))
	for _, r := range refs {
		asked[r] = true
	}
	out := make(map[string]string, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		// Only refs this call asked for, whatever the server sent (identity.proto: a SUBSET of refs).
		if asked[it.GetRef()] && it.GetPhone() != "" {
			out[it.GetRef()] = it.GetPhone()
		}
	}
	return out, nil
}

// warnNotice: Warn, not Error — this service is healthy. The code only: no ref, no tag, no number.
func (c *Client) warnNotice(ctx context.Context, method string, err error) {
	c.log.WarnContext(ctx, "CẢNH BÁO: không gọi được identity cho đợt gửi tin",
		"rpc", method, "ma_loi", status.Code(err).String())
}

func countsOf(r *identityv1.ResolveNoticeAudienceResponse) NoticeAudienceCounts {
	return NoticeAudienceCounts{
		Recipients:      int(r.GetRecipientCount()),
		SkippedInactive: int(r.GetSkippedInactiveCount()),
		SkippedNoPhone:  int(r.GetSkippedNoPhoneCount()),
	}
}

func audienceToProto(a NoticeAudience) *identityv1.NoticeAudience {
	var k identityv1.NoticeAudienceKind
	switch a.Kind {
	case NoticeAudienceCitizenAll:
		k = identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_ALL
	case NoticeAudienceCitizenGroup:
		k = identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_GROUP
	case NoticeAudienceCitizenTag:
		k = identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_CITIZEN_TAG
	case NoticeAudienceStaffAll:
		k = identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_ALL
	case NoticeAudienceStaffUnit:
		k = identityv1.NoticeAudienceKind_NOTICE_AUDIENCE_KIND_STAFF_UNIT
	}
	return &identityv1.NoticeAudience{Kind: k, GroupIds: a.GroupIDs, Tags: a.Tags, OrgUnitIds: a.OrgUnitIDs}
}

func recipientKindToProto(k NoticeRecipientKind) identityv1.NoticeRecipientKind {
	switch k {
	case NoticeRecipientCitizen:
		return identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN
	case NoticeRecipientStaff:
		return identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF
	}
	return identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_UNSPECIFIED
}

func recipientKindFromProto(k identityv1.NoticeRecipientKind) NoticeRecipientKind {
	switch k {
	case identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_CITIZEN:
		return NoticeRecipientCitizen
	case identityv1.NoticeRecipientKind_NOTICE_RECIPIENT_KIND_STAFF:
		return NoticeRecipientStaff
	}
	return 0
}
