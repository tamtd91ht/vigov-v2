package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

// Who a notice campaign ("Gửi tin ZNS / SMS", ADR 0099 #5) reaches, and the rules that turn a
// directory into a recipient list. The contract — the predicate of each audience, why duplicates are
// merged by number, why the counts are of the whole audience — is on ResolveNoticeAudience and
// RevealNoticeRecipientPhones in proto/vigov/identity/v1/identity.proto. This file holds the parts a
// machine can decide without a database: the shape of an audience and the selection over candidates.

// NoticeAudienceKind is the `Gửi cho ai` choice. Zero is "unspecified" and is refused.
type NoticeAudienceKind int

const (
	NoticeAudienceUnspecified NoticeAudienceKind = iota
	NoticeAudienceCitizenAll
	NoticeAudienceCitizenGroup
	NoticeAudienceCitizenTag
	NoticeAudienceStaffAll
	NoticeAudienceStaffUnit
)

// NoticeRecipientKind is the directory a recipient ref points into. Zero is "unspecified" and is
// refused.
type NoticeRecipientKind int

const (
	NoticeRecipientUnspecified NoticeRecipientKind = iota
	// NoticeRecipientCitizen — the ref is `citizen_contact.id`.
	NoticeRecipientCitizen
	// NoticeRecipientStaff — the ref is `nguoi_dung.ma`, the staff BUSINESS code.
	NoticeRecipientStaff
)

// The ceilings the contract fixes. Exceeding one is a refusal, never a truncation: a truncated audience
// sends to some people and not others with nothing reporting which.
const (
	// MaxNoticeAudienceListEntries bounds group_ids / tags / org_unit_ids, counted as SENT.
	MaxNoticeAudienceListEntries = 50
	// MaxNoticeAudiencePage bounds one ResolveNoticeAudience page.
	MaxNoticeAudiencePage = 10000
	// MaxNoticeRevealRefs bounds one RevealNoticeRecipientPhones call, counted as SENT.
	MaxNoticeRevealRefs = 500
	// MaxNoticeCampaignRefLen bounds campaign_ref, the audit entry's subject.
	MaxNoticeCampaignRefLen = 64
)

// ErrNoticeAudienceInvalid marks every request shape the contract answers INVALID_ARGUMENT. Wrapped
// with the reason; the reason never carries a value from the request (tags may be typed by a commune).
var ErrNoticeAudienceInvalid = errors.New("đối tượng gửi tin không hợp lệ")

// NoticeAudience is one audience. Exactly the list its Kind uses is non-empty.
type NoticeAudience struct {
	Kind       NoticeAudienceKind
	GroupIDs   []string
	Tags       []string
	OrgUnitIDs []string
}

// RecipientKind is the directory the audience reads: CITIZEN_* → citizen, STAFF_* → staff. One audience
// never mixes the two.
func (k NoticeAudienceKind) RecipientKind() NoticeRecipientKind {
	switch k {
	case NoticeAudienceCitizenAll, NoticeAudienceCitizenGroup, NoticeAudienceCitizenTag:
		return NoticeRecipientCitizen
	case NoticeAudienceStaffAll, NoticeAudienceStaffUnit:
		return NoticeRecipientStaff
	}
	return NoticeRecipientUnspecified
}

// CheckNoticeAudience applies the contract's shape rules and returns the NORMALISED audience: blanks
// dropped, duplicates collapsed, every list sorted (so two spellings of one audience have one
// Fingerprint), tags in the form NormalizeCitizenContactTags stores (inner whitespace collapsed).
//
// A list the kind REQUIRES that is empty after blanks are dropped is refused — `group_ids: [""]` must
// never be read as "no filter", which would widen a group send to the whole commune.
func CheckNoticeAudience(a NoticeAudience) (NoticeAudience, error) {
	for _, l := range [][]string{a.GroupIDs, a.Tags, a.OrgUnitIDs} {
		if len(l) > MaxNoticeAudienceListEntries {
			return NoticeAudience{}, errors.Join(ErrNoticeAudienceInvalid,
				errors.New("một danh sách của đối tượng vượt trần 50 mục"))
		}
	}
	out := NoticeAudience{
		Kind:       a.Kind,
		GroupIDs:   collapseSorted(a.GroupIDs, strings.TrimSpace),
		Tags:       collapseSorted(a.Tags, gomKhoangTrang),
		OrgUnitIDs: collapseSorted(a.OrgUnitIDs, strings.TrimSpace),
	}
	var need, groups, tags, units bool
	switch a.Kind {
	case NoticeAudienceCitizenAll, NoticeAudienceStaffAll:
		need = true
	case NoticeAudienceCitizenGroup:
		need, groups = len(out.GroupIDs) > 0, true
	case NoticeAudienceCitizenTag:
		need, tags = len(out.Tags) > 0, true
	case NoticeAudienceStaffUnit:
		need, units = len(out.OrgUnitIDs) > 0, true
	default:
		return NoticeAudience{}, errors.Join(ErrNoticeAudienceInvalid, errors.New("thiếu loại đối tượng"))
	}
	if !need {
		return NoticeAudience{}, errors.Join(ErrNoticeAudienceInvalid,
			errors.New("danh sách loại đối tượng này cần đang trống"))
	}
	// A list the kind does NOT use, non-empty, is refused rather than ignored: a caller that sent tags
	// with CITIZEN_ALL believed it was narrowing the send.
	if (!groups && len(a.GroupIDs) > 0) || (!tags && len(a.Tags) > 0) || (!units && len(a.OrgUnitIDs) > 0) {
		return NoticeAudience{}, errors.Join(ErrNoticeAudienceInvalid,
			errors.New("có danh sách mà loại đối tượng này không dùng"))
	}
	return out, nil
}

// collapseSorted cleans each entry, drops blanks and duplicates, and sorts.
func collapseSorted(in []string, clean func(string) string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = clean(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// Fingerprint names a NORMALISED audience in a page token, so a token minted for one audience is
// refused for another. A one-way digest: the token is opaque to the caller and must not carry the
// tags a commune typed.
func (a NoticeAudience) Fingerprint() string {
	var b strings.Builder
	b.WriteByte(byte('0' + a.Kind))
	for _, l := range [][]string{a.GroupIDs, a.Tags, a.OrgUnitIDs} {
		b.WriteByte(0x1e)
		b.WriteString(strings.Join(l, "\x1f"))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:16]
}

// NoticeCandidate is one directory row of an audience BEFORE selection: its ref, the number as stored
// (national form for a contact, as typed for staff), and whether it is switched off / locked.
//
// Phone IS PERSONAL DATA. Never logged; never leaves the use case unmasked except through Reveal.
type NoticeCandidate struct {
	Ref      string
	Phone    string
	Inactive bool
}

// NoticeRecipient is one deliverable recipient: the ref and the FULL number in national form.
type NoticeRecipient struct {
	Ref   string
	Phone string
}

// NoticeSelection is the outcome of SelectNoticeRecipients.
type NoticeSelection struct {
	// Recipients ordered by ref ascending, one per number.
	Recipients      []NoticeRecipient
	SkippedInactive int
	SkippedNoPhone  int
}

// SelectNoticeRecipients applies the contract's predicate to candidates: inactive rows are skipped and
// counted, rows whose number does not normalise (NormalizeCitizenPhone) are skipped and counted, and
// rows sharing one normalised number become ONE recipient under the SMALLEST ref. A duplicate merged is
// neither a skip nor a count — it is the same message (identity.proto, "ONE NUMBER, ONE MESSAGE").
//
// An inactive row is counted inactive whatever its number: "switched off" is the reason a person reads
// on the preview, and it is true.
func SelectNoticeRecipients(cands []NoticeCandidate) NoticeSelection {
	sorted := make([]NoticeCandidate, len(cands))
	copy(sorted, cands)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Ref < sorted[j].Ref })

	var sel NoticeSelection
	seenRef := make(map[string]bool, len(sorted))
	seenPhone := make(map[string]bool, len(sorted))
	for _, c := range sorted {
		if c.Ref == "" || seenRef[c.Ref] {
			continue
		}
		seenRef[c.Ref] = true
		if c.Inactive {
			sel.SkippedInactive++
			continue
		}
		phone, err := NormalizeCitizenPhone(c.Phone)
		if err != nil {
			sel.SkippedNoPhone++
			continue
		}
		if seenPhone[phone] {
			continue // the smaller ref, met first in ref order, keeps it
		}
		seenPhone[phone] = true
		sel.Recipients = append(sel.Recipients, NoticeRecipient{Ref: c.Ref, Phone: phone})
	}
	return sel
}
