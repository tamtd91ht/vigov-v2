package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The two use cases behind ResolveNoticeAudience and RevealNoticeRecipientPhones (ADR 0099 #5,
// identity.proto). Resolve discloses no full number and writes nothing; Reveal discloses, and writes
// ONE audit entry per call in the read's own transaction.

// ActionRevealNoticePhones is the audit verb for handing full numbers to the job sending a notice
// campaign — "mở số cho đợt gửi". The VALUE follows the house convention (ADR 0011, Vietnamese
// snake_case like `xem_email_can_bo`); the identifier is English (rule 12).
const ActionRevealNoticePhones = "mo_so_cho_dot_gui"

// The directory reads, declared here at the point of use. *idstore.CitizenDirectoryStore and
// *idstore.CanBoStore satisfy them.
type (
	CitizenNoticeReader interface {
		NoticeAudienceCounts(ctx context.Context, a domain.NoticeAudience) (active, inactive int, err error)
		NoticeAudiencePage(ctx context.Context, a domain.NoticeAudience, after string, limit int) ([]domain.NoticeCandidate, error)
		NoticeRevealCandidates(ctx context.Context, tx *store.ScopedTx, refs []string) ([]domain.NoticeCandidate, error)
		NoticeRevealAttributeSources(ctx context.Context, tx *store.ScopedTx, refs []string) (map[string]domain.NoticeAttributeSource, error)
	}
	StaffNoticeReader interface {
		NoticeAudienceCandidates(ctx context.Context, a domain.NoticeAudience) ([]domain.NoticeCandidate, error)
		NoticeRevealCandidates(ctx context.Context, tx *store.ScopedTx, codes []string) ([]domain.NoticeCandidate, error)
		NoticeRevealAttributeSources(ctx context.Context, tx *store.ScopedTx, codes []string) (map[string]domain.NoticeAttributeSource, error)
		StaffCodeExists(ctx context.Context, tx *store.ScopedTx, code string) (bool, error)
	}
	// NoticeFieldReader is the commune's attribute definitions (ADR 0101). *idstore.CitizenFieldStore.
	NoticeFieldReader interface {
		List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error)
	}
)

// ErrNoticeRevealInvalid marks every Reveal request the contract answers INVALID_ARGUMENT. Wrapped with
// the reason; the reason never carries a ref, a code or a number.
var ErrNoticeRevealInvalid = errors.New("yêu cầu mở số cho đợt gửi không hợp lệ")

// NoticeAudienceMember is one recipient as Resolve hands it out: the ref and the MASKED number. The
// full number never leaves Resolve.
type NoticeAudienceMember struct {
	Ref         string
	MaskedPhone string
}

// NoticeAudiencePage is one page of an audience plus the counts of the WHOLE audience.
type NoticeAudiencePage struct {
	Members         []NoticeAudienceMember
	RecipientCount  int
	SkippedInactive int
	SkippedNoPhone  int
	// NextAfter is the last ref of this page when another page exists; "" on the last page.
	NextAfter string
}

// NoticeAudience serves ResolveNoticeAudience and RevealNoticeRecipientPhones.
type NoticeAudience struct {
	db       *store.DB
	citizens CitizenNoticeReader
	staff    StaffNoticeReader
	fields   NoticeFieldReader
	// pageSize is domain.MaxNoticeAudiencePage; a field only so a test can walk pages of two.
	pageSize int
}

func NewNoticeAudience(db *store.DB, citizens CitizenNoticeReader, staff StaffNoticeReader, fields NoticeFieldReader) *NoticeAudience {
	if db == nil || citizens == nil || staff == nil || fields == nil {
		panic("app.NewNoticeAudience: db, citizens, staff and fields are required")
	}
	return &NoticeAudience{db: db, citizens: citizens, staff: staff, fields: fields, pageSize: domain.MaxNoticeAudiencePage}
}

// Attributes lists what a campaign of `kind` may map a template parameter to (ListNoticeAttributes, ADR
// 0101 #12): configuration only — nothing personal is read, nothing audited.
func (uc *NoticeAudience) Attributes(ctx context.Context, kind domain.NoticeRecipientKind) ([]domain.NoticeAttribute, error) {
	switch kind {
	case domain.NoticeRecipientStaff:
		return domain.NoticeAttributesFor(kind, nil), nil
	case domain.NoticeRecipientCitizen:
		// Scoped: CitizenFieldStore reads through db.For(ctx) — the context's commune is tenant_id = $1.
		defs, err := uc.fields.List(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("thuoc_tinh_dot_gui: xã %s: %w", tenant.MustFrom(ctx), err)
		}
		return domain.NoticeAttributesFor(kind, defs), nil
	}
	return nil, errors.Join(ErrNoticeRevealInvalid, errors.New("thiếu loại người nhận"))
}

// noticeAttributePlan is a checked attribute_codes request: the codes, the definitions by code (live and
// retired) and the commune's zone. Zero when no attribute was asked.
type noticeAttributePlan struct {
	codes []string
	defs  map[string]domain.CitizenField
	loc   *time.Location
}

// planAttributes checks attribute_codes BEFORE the transaction: the definitions are configuration, read
// once per call, never per recipient.
func (uc *NoticeAudience) planAttributes(ctx context.Context, kind domain.NoticeRecipientKind, codes []string) (noticeAttributePlan, error) {
	if len(codes) > domain.MaxNoticeAttributeCodes {
		return noticeAttributePlan{}, errors.Join(ErrNoticeRevealInvalid,
			fmt.Errorf("quá %d mã thuộc tính trong một lần mở", domain.MaxNoticeAttributeCodes))
	}
	nonBlank := false
	for _, c := range codes {
		nonBlank = nonBlank || strings.TrimSpace(c) != ""
	}
	if !nonBlank {
		return noticeAttributePlan{}, nil
	}
	var defs []domain.CitizenField
	if kind == domain.NoticeRecipientCitizen {
		var err error
		// Live AND retired: the mapping was fixed at POST (identity.proto). Scoped through db.For(ctx).
		if defs, err = uc.fields.List(ctx, true); err != nil {
			return noticeAttributePlan{}, fmt.Errorf("mo_so_cho_dot_gui: đọc cấu hình thuộc tính: %w", err)
		}
	}
	checked, err := domain.CheckNoticeAttributeCodes(kind, codes, defs)
	if err != nil {
		return noticeAttributePlan{}, errors.Join(ErrNoticeRevealInvalid, err)
	}
	// The commune's zone for dateTime — an error, never a fallback to UTC (domain.MuiGio).
	loc, err := domain.MuiGio()
	if err != nil {
		return noticeAttributePlan{}, err
	}
	byCode := make(map[string]domain.CitizenField, len(defs))
	for _, d := range defs {
		byCode[d.Code] = d
	}
	return noticeAttributePlan{codes: checked, defs: byCode, loc: loc}, nil
}

// Resolve answers one page of a CHECKED audience (domain.CheckNoticeAudience) after the ref `after`
// ("" = first page). countOnly answers the counts alone — no member, no NextAfter, no page read.
//
// NOTHING IS WRITTEN AND NOTHING IS AUDITED: no full number leaves (identity.proto). Masked here, so a
// caller of this use case cannot reach the full number even by mistake.
func (uc *NoticeAudience) Resolve(ctx context.Context, a domain.NoticeAudience, after string, countOnly bool) (NoticeAudiencePage, error) {
	switch a.Kind.RecipientKind() {
	case domain.NoticeRecipientCitizen:
		return uc.resolveCitizens(ctx, a, after, countOnly)
	case domain.NoticeRecipientStaff:
		return uc.resolveStaff(ctx, a, after, countOnly)
	}
	return NoticeAudiencePage{}, errors.Join(domain.ErrNoticeAudienceInvalid, errors.New("thiếu loại đối tượng"))
}

// resolveCitizens: counts in one statement, the page in another, keyset by contact id. A live number is
// unique per commune, so the count of active rows IS the count after deduplication.
func (uc *NoticeAudience) resolveCitizens(ctx context.Context, a domain.NoticeAudience, after string, countOnly bool) (NoticeAudiencePage, error) {
	active, inactive, err := uc.citizens.NoticeAudienceCounts(ctx, a)
	if err != nil {
		return NoticeAudiencePage{}, fmt.Errorf("doi_tuong_gui_tin: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	out := NoticeAudiencePage{RecipientCount: active, SkippedInactive: inactive}
	if countOnly {
		return out, nil
	}
	cands, err := uc.citizens.NoticeAudiencePage(ctx, a, after, uc.pageSize+1)
	if err != nil {
		return NoticeAudiencePage{}, fmt.Errorf("doi_tuong_gui_tin: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	more := len(cands) > uc.pageSize
	if more {
		cands = cands[:uc.pageSize]
	}
	sel := domain.SelectNoticeRecipients(cands)
	out.Members = maskMembers(sel.Recipients)
	if more {
		// The keyset is the last ROW read, not the last member kept: a row dropped by the selection
		// must not be read again on the next page.
		out.NextAfter = cands[len(cands)-1].Ref
	}
	return out, nil
}

// resolveStaff: the whole staff audience in one statement, selected and deduplicated over all of it,
// then paged in memory by ref.
func (uc *NoticeAudience) resolveStaff(ctx context.Context, a domain.NoticeAudience, after string, countOnly bool) (NoticeAudiencePage, error) {
	cands, err := uc.staff.NoticeAudienceCandidates(ctx, a)
	if err != nil {
		return NoticeAudiencePage{}, fmt.Errorf("doi_tuong_gui_tin: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	sel := domain.SelectNoticeRecipients(cands)
	out := NoticeAudiencePage{
		RecipientCount:  len(sel.Recipients),
		SkippedInactive: sel.SkippedInactive,
		SkippedNoPhone:  sel.SkippedNoPhone,
	}
	if countOnly {
		return out, nil
	}
	rest := sel.Recipients
	if after != "" {
		i := sort.Search(len(rest), func(i int) bool { return rest[i].Ref > after })
		rest = rest[i:]
	}
	if len(rest) > uc.pageSize {
		rest = rest[:uc.pageSize]
		out.NextAfter = rest[len(rest)-1].Ref
	}
	out.Members = maskMembers(rest)
	return out, nil
}

func maskMembers(rs []domain.NoticeRecipient) []NoticeAudienceMember {
	out := make([]NoticeAudienceMember, 0, len(rs))
	for _, r := range rs {
		out = append(out, NoticeAudienceMember{Ref: r.Ref, MaskedPhone: privacy.MaskPhone(r.Phone)})
	}
	return out
}

// Reveal returns the FULL national number of each deliverable ref, and records the disclosure.
//
// THE READ AND THE ENTRY SHARE ONE TRANSACTION, and the numbers leave only after the entry has
// committed (the ordering of StaffEmailReveal, ADR 0082). A failed entry rolls back and nothing is
// disclosed. A call that discloses nothing writes nothing.
//
// THE ACTOR IS THE STAFF MEMBER WHO PRESSED SEND, by business code (rule 6, invariant 8) — the code
// comms captured from authz.Principal.Ma at POST. It must be a `nguoi_dung.ma` of this commune
// (soft-deleted included), checked inside the same transaction; anything else is
// ErrNoticeRevealInvalid. IP is EMPTY: identity observes no browser on this hop (identity.proto).
//
// ATTRIBUTES (ADR 0101 #12): attributeCodes, when given, are read for the SAME recipients in the SAME
// transaction and recorded on the SAME entry — the codes, never a value.
func (uc *NoticeAudience) Reveal(ctx context.Context, kind domain.NoticeRecipientKind, refs []string,
	campaignRef, actorCode string, attributeCodes []string) ([]domain.NoticeRecipient, error) {

	switch {
	case kind != domain.NoticeRecipientCitizen && kind != domain.NoticeRecipientStaff:
		return nil, errors.Join(ErrNoticeRevealInvalid, errors.New("thiếu loại người nhận"))
	case len(refs) > domain.MaxNoticeRevealRefs:
		return nil, errors.Join(ErrNoticeRevealInvalid,
			fmt.Errorf("quá %d ref trong một lần mở số — bên gọi phải chia lô", domain.MaxNoticeRevealRefs))
	case strings.TrimSpace(campaignRef) == "" || len(campaignRef) > domain.MaxNoticeCampaignRefLen:
		return nil, errors.Join(ErrNoticeRevealInvalid,
			fmt.Errorf("campaign_ref trống hoặc dài quá %d ký tự", domain.MaxNoticeCampaignRefLen))
	case strings.TrimSpace(actorCode) == "":
		// NEVER a fallback actor: an entry naming nobody, or the system, answers "who sent this to the
		// whole commune" with nobody (identity.proto).
		return nil, errors.Join(ErrNoticeRevealInvalid, errors.New("thiếu actor_code"))
	}
	plan, err := uc.planAttributes(ctx, kind, attributeCodes)
	if err != nil {
		if errors.Is(err, ErrNoticeRevealInvalid) {
			return nil, err
		}
		return nil, fmt.Errorf("mo_so_cho_dot_gui: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	wanted := collapseRefs(refs)
	if len(wanted) == 0 {
		return nil, nil // empty answers empty — never "everyone"
	}

	var out []domain.NoticeRecipient
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		ok, err := uc.staff.StaffCodeExists(ctx, tx, actorCode)
		if err != nil {
			return err
		}
		if !ok {
			return errors.Join(ErrNoticeRevealInvalid, errors.New("actor_code không phải mã cán bộ của xã này"))
		}

		var cands []domain.NoticeCandidate
		if kind == domain.NoticeRecipientCitizen {
			cands, err = uc.citizens.NoticeRevealCandidates(ctx, tx, wanted)
		} else {
			cands, err = uc.staff.NoticeRevealCandidates(ctx, tx, wanted)
		}
		if err != nil {
			return err
		}
		// Only refs that were asked for, even if the read returned another; then the predicate NOW,
		// and duplicates by number inside this batch (the smallest ref keeps it).
		asked := make(map[string]bool, len(wanted))
		for _, r := range wanted {
			asked[r] = true
		}
		kept := cands[:0:0]
		for _, c := range cands {
			if asked[c.Ref] {
				kept = append(kept, c)
			}
		}
		sel := domain.SelectNoticeRecipients(kept)
		if len(sel.Recipients) == 0 {
			return nil // nothing disclosed, nothing written
		}

		if len(plan.codes) > 0 {
			if err := uc.attachAttributes(ctx, tx, kind, plan, sel.Recipients); err != nil {
				return err
			}
		}

		fields := map[string]any{
			// The FIELD opened, the kind and the COUNT — never a number, never a ref list (rule 6,
			// forbidden #4).
			"field": "phone",
			"kind":  recipientKindLabel(kind),
			"count": len(sel.Recipients),
		}
		if len(plan.codes) > 0 {
			fields["attributes"] = plan.codes // the CODES opened (ADR 0101 #12) — never a value
		}
		delta, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("mo_so_cho_dot_gui: delta: %w", err)
		}
		// TenantID left unset: audit.Write takes it from the transaction, which took it from the
		// context — one source for the commune (rule 1, invariant 4).
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: actorCode, Kind: "staff", IP: ""},
			Action:  ActionRevealNoticePhones,
			Subject: campaignRef,
			Delta:   delta,
		}); err != nil {
			return err
		}
		out = sel.Recipients
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNoticeRevealInvalid) {
			return nil, err
		}
		// Never a number, a ref or a code in the message: the commune is what an operator needs.
		return nil, fmt.Errorf("mo_so_cho_dot_gui: xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return out, nil
}

// attachAttributes reads the attribute sources of the selected recipients in the Reveal transaction and
// sets each recipient's formatted values — one entry per requested code, "" when absent.
func (uc *NoticeAudience) attachAttributes(ctx context.Context, tx *store.ScopedTx, kind domain.NoticeRecipientKind,
	plan noticeAttributePlan, rs []domain.NoticeRecipient) error {

	refs := make([]string, 0, len(rs))
	for _, r := range rs {
		refs = append(refs, r.Ref)
	}
	var (
		srcs map[string]domain.NoticeAttributeSource
		err  error
	)
	if kind == domain.NoticeRecipientCitizen {
		srcs, err = uc.citizens.NoticeRevealAttributeSources(ctx, tx, refs)
	} else {
		srcs, err = uc.staff.NoticeRevealAttributeSources(ctx, tx, refs)
	}
	if err != nil {
		return err
	}
	for i := range rs {
		rs[i].Attributes = domain.FormatNoticeAttributes(plan.codes, plan.defs, srcs[rs[i].Ref], plan.loc)
	}
	return nil
}

// recipientKindLabel is the delta's spelling of the recipient kind.
func recipientKindLabel(k domain.NoticeRecipientKind) string {
	if k == domain.NoticeRecipientStaff {
		return "staff"
	}
	return "citizen"
}

// collapseRefs drops blanks and duplicates. NOT trimmed: the caller maps the answer by the ref it
// sent, so a ref is matched exactly as sent or not at all.
func collapseRefs(refs []string) []string {
	seen := make(map[string]bool, len(refs))
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if strings.TrimSpace(r) == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}
