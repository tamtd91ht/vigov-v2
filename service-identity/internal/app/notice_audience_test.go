package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The notice-campaign use cases (ADR 0099 #5): Resolve masks, pages and counts the WHOLE audience and
// writes nothing; Reveal reads and writes ONE audit entry in one committed transaction, on the fake
// driver of dang_nhap_giao_dich_test.go. The SQL predicates are the store's (store/notice_audience_pg_test.go).

const (
	noticeCampaign = "NC-01JCAMPAIGNSAMPLE0000000"
	noticeActor    = "CB-00123"
)

// Fake numbers only (rule 3, invariant 5): the agreed one and its neighbours.
func fakePhone(n int) string { return fmt.Sprintf("09000000%02d", n) }

type citizenNoticeFake struct {
	rows     []domain.NoticeCandidate // ALL live rows of the audience, inactive included
	reveal   []domain.NoticeCandidate
	err      error
	pageArgs []string
}

func (f *citizenNoticeFake) NoticeAudienceCounts(_ context.Context, _ domain.NoticeAudience) (int, int, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	a, i := 0, 0
	for _, r := range f.rows {
		if r.Inactive {
			i++
		} else {
			a++
		}
	}
	return a, i, nil
}

func (f *citizenNoticeFake) NoticeAudiencePage(_ context.Context, _ domain.NoticeAudience, after string, limit int) ([]domain.NoticeCandidate, error) {
	f.pageArgs = append(f.pageArgs, after)
	var out []domain.NoticeCandidate
	for _, r := range f.rows { // rows are kept in ref order by the tests
		if !r.Inactive && r.Ref > after && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *citizenNoticeFake) NoticeRevealCandidates(_ context.Context, tx *store.ScopedTx, refs []string) ([]domain.NoticeCandidate, error) {
	if tx == nil {
		return nil, errors.New("read outside the transaction")
	}
	return f.reveal, f.err
}

type staffNoticeFake struct {
	rows      []domain.NoticeCandidate
	reveal    []domain.NoticeCandidate
	codes     map[string]bool
	revealArg []string
}

func (f *staffNoticeFake) NoticeAudienceCandidates(context.Context, domain.NoticeAudience) ([]domain.NoticeCandidate, error) {
	return f.rows, nil
}

func (f *staffNoticeFake) NoticeRevealCandidates(_ context.Context, tx *store.ScopedTx, codes []string) ([]domain.NoticeCandidate, error) {
	if tx == nil {
		return nil, errors.New("read outside the transaction")
	}
	f.revealArg = codes
	return f.reveal, nil
}

func (f *staffNoticeFake) StaffCodeExists(_ context.Context, tx *store.ScopedTx, code string) (bool, error) {
	if tx == nil {
		return false, errors.New("read outside the transaction")
	}
	return f.codes[code], nil
}

func noticeHarness(t *testing.T, c *citizenNoticeFake, s *staffNoticeFake) (*NoticeAudience, *ghiChep) {
	t.Helper()
	db, g := moDB(t)
	if s.codes == nil {
		s.codes = map[string]bool{noticeActor: true}
	}
	return NewNoticeAudience(db, c, s), g
}

// STAFF: one number shared by two rows is ONE recipient under the smaller ref; a locked account and a
// row without a usable mobile are skipped and counted; a directory-only row is delivered. Paged by ref
// over the deduplicated set, counts identical on every page, and nothing is written.
func TestNoticeResolveStaffDedupSkipsAndPages(t *testing.T) {
	staff := &staffNoticeFake{rows: []domain.NoticeCandidate{
		{Ref: "CB-00005", Phone: fakePhone(1)},                 // duplicate of CB-00001 — merged
		{Ref: "CB-00001", Phone: "+84 900 000 001"},            // typed with prefix and spaces — normalises
		{Ref: "CB-00002", Phone: fakePhone(2), Inactive: true}, // locked
		{Ref: "CB-00003", Phone: ""},                           // no mobile
		{Ref: "CB-00004", Phone: fakePhone(4)},                 // directory-only row: delivered
		{Ref: "CB-00006", Phone: fakePhone(6)},
	}}
	uc, g := noticeHarness(t, &citizenNoticeFake{}, staff)
	uc.pageSize = 2
	ctx := ctxXa(xaThu)
	aud := domain.NoticeAudience{Kind: domain.NoticeAudienceStaffAll}

	var refs []string
	after := ""
	for pages := 0; pages < 5; pages++ {
		p, err := uc.Resolve(ctx, aud, after, false)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if p.RecipientCount != 3 || p.SkippedInactive != 1 || p.SkippedNoPhone != 1 {
			t.Fatalf("counts = %d/%d/%d, want 3 recipients, 1 inactive, 1 no phone on EVERY page",
				p.RecipientCount, p.SkippedInactive, p.SkippedNoPhone)
		}
		for _, m := range p.Members {
			refs = append(refs, m.Ref)
			if strings.Contains(m.MaskedPhone, "000000") || !strings.Contains(m.MaskedPhone, "*") {
				t.Errorf("member %s carries %q — must be masked", m.Ref, m.MaskedPhone)
			}
		}
		if p.NextAfter == "" {
			break
		}
		after = p.NextAfter
	}
	if got := strings.Join(refs, ","); got != "CB-00001,CB-00004,CB-00006" {
		t.Errorf("walked %s, want CB-00001,CB-00004,CB-00006 (dedup keeps the smaller ref, ordered)", got)
	}
	if g.soGiaoDich() != 0 || g.tim("audit_log") != nil {
		t.Error("Resolve opened a transaction or wrote an entry — it must write nothing")
	}
}

// CITIZENS: the counts come from the count read, the page walks the keyset by the last ROW read, and
// count_only reads no page at all.
func TestNoticeResolveCitizensKeysetAndCountOnly(t *testing.T) {
	cit := &citizenNoticeFake{rows: []domain.NoticeCandidate{
		{Ref: "01A", Phone: fakePhone(1)},
		{Ref: "01B", Phone: fakePhone(2), Inactive: true},
		{Ref: "01C", Phone: fakePhone(3)},
		{Ref: "01D", Phone: fakePhone(4)},
	}}
	uc, _ := noticeHarness(t, cit, &staffNoticeFake{})
	uc.pageSize = 2
	ctx := ctxXa(xaThu)
	aud := domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenAll}

	p, err := uc.Resolve(ctx, aud, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Members) != 0 || p.NextAfter != "" || p.RecipientCount != 3 || p.SkippedInactive != 1 {
		t.Fatalf("count_only = %+v — want counts only", p)
	}
	if len(cit.pageArgs) != 0 {
		t.Fatal("count_only read a page")
	}

	p1, _ := uc.Resolve(ctx, aud, "", false)
	if len(p1.Members) != 2 || p1.NextAfter != "01C" {
		t.Fatalf("page 1 = %+v, want 01A,01C then next after 01C", p1)
	}
	p2, _ := uc.Resolve(ctx, aud, p1.NextAfter, false)
	if len(p2.Members) != 1 || p2.Members[0].Ref != "01D" || p2.NextAfter != "" {
		t.Fatalf("page 2 = %+v, want 01D and no next", p2)
	}
}

func TestNoticeRevealOneEntryInTheReadsCommittedTransaction(t *testing.T) {
	staff := &staffNoticeFake{reveal: []domain.NoticeCandidate{
		{Ref: "CB-00002", Phone: fakePhone(1)}, // same number as CB-00001 in this batch — absent
		{Ref: "CB-00001", Phone: fakePhone(1)},
		{Ref: "CB-00003", Phone: fakePhone(3), Inactive: true}, // locked since POST — absent
		{Ref: "CB-00009", Phone: fakePhone(9)},                 // never asked for — absent
	}}
	uc, g := noticeHarness(t, &citizenNoticeFake{}, staff)

	got, err := uc.Reveal(ctxXa(xaThu), domain.NoticeRecipientStaff,
		[]string{"CB-00001", "CB-00002", "CB-00003", "CB-00001", ""}, noticeCampaign, noticeActor)
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if len(got) != 1 || got[0].Ref != "CB-00001" || got[0].Phone != fakePhone(1) {
		t.Fatalf("got %+v, want only CB-00001 with its full number", got)
	}
	if strings.Join(staff.revealArg, ",") != "CB-00001,CB-00002,CB-00003" {
		t.Errorf("refs read = %v, want blanks and duplicates collapsed", staff.revealArg)
	}
	if n := countStatements(g, "INSERT INTO audit_log"); n != 1 {
		t.Fatalf("%d audit entries, want exactly one per call", n)
	}
	entry := g.tim("INSERT INTO audit_log")
	if entry.tx == 0 || g.ketThucCua(entry.tx) != "commit" {
		t.Fatal("the entry was not written in a committed transaction")
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if entry.args[0] != string(xaThu) || entry.args[1] != noticeActor || entry.args[2] != "staff" ||
		entry.args[3] != "" || entry.args[4] != ActionRevealNoticePhones || entry.args[5] != noticeCampaign {
		t.Errorf("entry %v — want commune, the CB- code, staff, NO IP, the verb, the campaign", entry.args[:6])
	}
	raw := entry.args[7].([]byte)
	for _, leak := range []string{fakePhone(1), "CB-00001", "CB-00002"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("the delta carries %q: %s", leak, raw)
		}
	}
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatal(err)
	}
	if delta["field"] != "phone" || delta["kind"] != "staff" || delta["count"] != float64(1) {
		t.Errorf("delta = %v, want field phone, kind staff, count 1", delta)
	}
}

func TestNoticeRevealNothingDeliverableWritesNothing(t *testing.T) {
	cit := &citizenNoticeFake{reveal: []domain.NoticeCandidate{{Ref: "01A", Phone: fakePhone(1), Inactive: true}}}
	uc, g := noticeHarness(t, cit, &staffNoticeFake{})

	got, err := uc.Reveal(ctxXa(xaThu), domain.NoticeRecipientCitizen, []string{"01A", "01Z"}, noticeCampaign, noticeActor)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v — want nothing, no error", got, err)
	}
	if g.tim("INSERT INTO audit_log") != nil {
		t.Error("nothing was disclosed, yet an entry was written")
	}
}

func TestNoticeRevealUnknownActorIsInvalidAndUnaudited(t *testing.T) {
	cit := &citizenNoticeFake{reveal: []domain.NoticeCandidate{{Ref: "01A", Phone: fakePhone(1)}}}
	uc, g := noticeHarness(t, cit, &staffNoticeFake{})

	got, err := uc.Reveal(ctxXa(xaThu), domain.NoticeRecipientCitizen, []string{"01A"}, noticeCampaign, "CB-99999")
	if !errors.Is(err, ErrNoticeRevealInvalid) || got != nil {
		t.Fatalf("got %v, %v — want ErrNoticeRevealInvalid", got, err)
	}
	if g.tim("INSERT INTO audit_log") != nil {
		t.Error("an entry was written for an actor that is not a staff code of this commune")
	}
}

func TestNoticeRevealAuditFailureDisclosesNothing(t *testing.T) {
	cit := &citizenNoticeFake{reveal: []domain.NoticeCandidate{{Ref: "01A", Phone: fakePhone(1)}}}
	uc, g := noticeHarness(t, cit, &staffNoticeFake{})
	g.loiTheo["audit_log"] = errors.New("audit write failed")

	got, err := uc.Reveal(ctxXa(xaThu), domain.NoticeRecipientCitizen, []string{"01A"}, noticeCampaign, noticeActor)
	if err == nil || got != nil {
		t.Fatalf("audit failed and numbers were returned: %v, %v", got, err)
	}
	if strings.Contains(err.Error(), fakePhone(1)) || strings.Contains(err.Error(), "01A") {
		t.Errorf("the error carries a number or a ref: %v", err)
	}
	entry := g.tim("INSERT INTO audit_log")
	if entry == nil || g.ketThucCua(entry.tx) != "rollback" {
		t.Fatal("a disclosure with no trail was committed")
	}
}

// Every shape refusal happens before a transaction opens.
func TestNoticeRevealShapeRefusalsTouchNothing(t *testing.T) {
	uc, g := noticeHarness(t, &citizenNoticeFake{}, &staffNoticeFake{})
	many := make([]string, domain.MaxNoticeRevealRefs+1)
	for i := range many {
		many[i] = fmt.Sprintf("R%03d", i)
	}
	for name, c := range map[string]struct {
		kind     domain.NoticeRecipientKind
		refs     []string
		campaign string
		actor    string
	}{
		"kind unspecified":  {domain.NoticeRecipientUnspecified, []string{"01A"}, noticeCampaign, noticeActor},
		"over 500 refs":     {domain.NoticeRecipientCitizen, many, noticeCampaign, noticeActor},
		"no campaign":       {domain.NoticeRecipientCitizen, []string{"01A"}, "", noticeActor},
		"campaign too long": {domain.NoticeRecipientCitizen, []string{"01A"}, strings.Repeat("x", 65), noticeActor},
		"no actor":          {domain.NoticeRecipientCitizen, []string{"01A"}, noticeCampaign, ""},
	} {
		if _, err := uc.Reveal(ctxXa(xaThu), c.kind, c.refs, c.campaign, c.actor); !errors.Is(err, ErrNoticeRevealInvalid) {
			t.Errorf("%s: err = %v, want ErrNoticeRevealInvalid", name, err)
		}
	}
	// Empty refs: empty answer, not "everyone", and no transaction.
	if got, err := uc.Reveal(ctxXa(xaThu), domain.NoticeRecipientCitizen, []string{"", ""}, noticeCampaign, noticeActor); err != nil || got != nil {
		t.Errorf("empty refs = %v, %v — want empty", got, err)
	}
	if n := g.soGiaoDich(); n != 0 {
		t.Errorf("%d transactions opened for refused requests", n)
	}
}
