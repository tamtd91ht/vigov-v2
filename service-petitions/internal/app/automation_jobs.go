package app

// The three jobs (ADR 0058 §4, §8; ADR 0029 §Bổ sung 29/09) and the delivery of their notices.
//
// A JOB FIRST BUILDS A PLAN, THEN DELIVERS IT. Every identity question is asked while planning; a
// failure there (outage, contract fault) stops the run with NOTHING delivered, and the next run
// redoes it under the same idempotency keys. FAILED_PRECONDITION on one field (the commune has no
// usable `sla` row for it) drops THAT field's notices only — nothing is ever sent from a default —
// and the run is recorded CONFIGURATION_MISSING, so the commune sees it on the configuration screen.
//
// WHO IS TOLD, in one chain for every job, fail closed at each step:
//
//	the named holder  →  the people in the holding unit who hand out its work
//	                     (`task.assign` / `feedback.assign`, ResolveOrgUnitPermissionHolders)
//	                  →  the leadership (ResolveLeadershipStaff) — the contract's fallback for work with
//	                     neither a holder nor a unit
//
// A RESTRICTED PETITION (`can-bo`, ADR 0030) is announced only to its named holder, or to unit
// holders who ALSO hold `feedback.restricted` (asked as a second key — the contract's "ask a second
// time"), and NEVER to the leadership list, which carries no permission fact. A report about a
// member of staff must not reach a colleague through the bell. Nobody left → counted as without
// recipient, the one number the commune must see.
//
// ALL LATENESS IS IDENTITY'S INSTANTS compared with `claimed_at` (rule 10, forbidden #2 and
// invariant 3). No deadline is moved or stored; no hour is added here.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The permission keys that mean "hands out this unit's work" in this service's two domains, and the
// key that lets a person see the restricted field. All three are rows of `quyen` (identity.proto,
// ResolveOrgUnitPermissionHolders; ADR 0030).
const (
	permTaskAssign         = "task.assign"
	permFeedbackAssign     = "feedback.assign"
	permFeedbackRestricted = "feedback.restricted"
)

// maxRecipientsPerNotice is comms' bound on one notice (1 to 200); a longer list is split into
// several notices with the SAME key, in different calls — comms deduplicates per recipient.
const maxRecipientsPerNotice = 200

// automationPlan is what one run decided to say.
type automationPlan struct {
	notices  []domain.StaffNotice
	examined int
	// withoutRecipient holds the ids of records that were due a notice and had nobody to receive it.
	withoutRecipient map[string]bool
	// digestWithoutRecipient counts the records a digest summarised when there was no leadership.
	digestWithoutRecipient int
	configMissing          bool
}

func (p *automationPlan) nobody(id string) {
	if p.withoutRecipient == nil {
		p.withoutRecipient = map[string]bool{}
	}
	p.withoutRecipient[id] = true
}

func (p automationPlan) withoutRecipientCount() int {
	return len(p.withoutRecipient) + p.digestWithoutRecipient
}

// loadRecords reads the kind's open records. A store failure is a dependency failure.
func (r *AutomationRunner) loadRecords(ctx context.Context, kind domain.AutomationWorkKind) ([]domain.AutomationRecord, error) {
	var (
		recs []domain.AutomationRecord
		err  error
	)
	if kind == domain.AutomationCitizenReport {
		recs, err = r.d.CitizenReports.OpenCitizenReportsForAutomation(ctx)
	} else {
		recs, err = r.d.Tasks.OpenTasksForAutomation(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errAutomationStore, err)
	}
	return recs, nil
}

// --- sla_reminders ---------------------------------------------------------------------------------

func (r *AutomationRunner) slaReminders(ctx context.Context, kind domain.AutomationWorkKind, asOf time.Time) (
	automationPlan, error) {

	var p automationPlan
	recs, err := r.loadRecords(ctx, kind)
	if err != nil {
		return p, err
	}
	p.examined = len(recs)
	wk := identityWorkKind(kind)

	// 1. THE DUE-SOON CUTOFF, ONCE PER SLA ROW KEY (one key, one threshold — identity.proto): a
	// petition's field, a task's priority (ADR 0079 lô 2 Q4 b; "" = no priority, the default row).
	// Thresholds only — every task deadline stays the one the assigner typed (rule 10).
	cutoffs := map[string]time.Time{}
	for _, f := range distinctFields(recs, func(x domain.AutomationRecord) bool {
		return x.HasDeadline() && x.Deadline.After(asOf)
	}) {
		c, err := r.d.Identity.DueSoonCutoff(ctx, wk, f, asOf)
		if errors.Is(err, identityclient.ErrDueSoonNotConfigured) {
			p.configMissing = true
			continue
		}
		if err != nil {
			return p, err
		}
		cutoffs[f] = c
	}

	// 2. THE UNASSIGNED-HOLD INSTANTS, per SLA row key, in pages of 500 distinct starts.
	holdDue := map[string]map[time.Time]time.Time{}
	held := groupByField(recs, domain.AutomationRecord.HeldUnassigned)
	for _, f := range sortedKeys(held) {
		due := map[time.Time]time.Time{}
		disabled := false
		for _, page := range pageInstants(held[f], func(x domain.AutomationRecord) time.Time { return x.HoldStartedAt }) {
			off, got, err := r.d.Identity.UnassignedHoldInstants(ctx, wk, f, page)
			if errors.Is(err, identityclient.ErrAutomationNotConfigured) {
				p.configMissing = true
				disabled = true
				break
			}
			if err != nil {
				return p, err
			}
			if off { // the commune's NULL: do not report this field — an answer, not a fault
				disabled = true
				break
			}
			for k, v := range got {
				due[k] = v
			}
		}
		if !disabled {
			holdDue[f] = due
		}
	}

	// 3. WHAT IS DUE A NOTICE.
	var soon, late, unassigned []domain.AutomationRecord
	for _, x := range recs {
		if c, ok := cutoffs[x.SLARowKey()]; ok && x.DueSoonAt(asOf, c) {
			soon = append(soon, x)
		} else if x.PastDeadlineAt(asOf) {
			late = append(late, x)
		}
		if x.HeldUnassigned() {
			if at, ok := holdDue[x.SLARowKey()][x.HoldStartedAt.UTC()]; ok && !at.After(asOf) {
				unassigned = append(unassigned, x)
			}
		}
	}

	// 4. WHO — one batched lookup per unit set, leadership once. Only records with no usable named
	// holder need their unit asked about.
	var unnamed []domain.AutomationRecord
	for _, x := range concat(soon, late, unassigned) {
		if len(domain.CleanRecipients([]string{x.AssigneeMa})) == 0 {
			unnamed = append(unnamed, x)
		}
	}
	book, err := r.newRecipientBook(ctx, kind, unnamed, len(unnamed) > 0)
	if err != nil {
		return p, err
	}
	p.configMissing = p.configMissing || book.leadersMissing

	// 5. THE NOTICES.
	day := domain.LocalDay(asOf)
	perPerson := map[string][]string{}
	for _, x := range soon {
		to := book.chain(x, true)
		if len(to) == 0 {
			p.nobody(x.ID)
			continue
		}
		for _, ma := range to {
			perPerson[ma] = append(perPerson[ma], x.Code)
		}
	}
	for _, ma := range sortedKeys(perPerson) {
		p.notices = append(p.notices, domain.DueSoonNotice(kind, day, ma, perPerson[ma]))
	}
	for _, x := range late {
		to := book.chain(x, true)
		if len(to) == 0 {
			p.nobody(x.ID)
			continue
		}
		p.notices = append(p.notices, domain.OverdueNotice(kind, x, day, to))
	}
	for _, x := range unassigned {
		to := book.chain(x, true)
		if len(to) == 0 {
			p.nobody(x.ID)
			continue
		}
		p.notices = append(p.notices, domain.UnassignedNotice(kind, x, to))
	}
	return p, nil
}

// --- escalation ------------------------------------------------------------------------------------

// escalation tells the unit head, then the leadership, once each, when a late record's lateness
// crosses the commune's two thresholds. Petitions escalate on `han_xu_ly_xong` only (user decision) —
// the only deadline OpenCitizenReportsForAutomation reads.
func (r *AutomationRunner) escalation(ctx context.Context, kind domain.AutomationWorkKind, asOf time.Time) (
	automationPlan, error) {

	var p automationPlan
	recs, err := r.loadRecords(ctx, kind)
	if err != nil {
		return p, err
	}
	p.examined = len(recs)
	wk := identityWorkKind(kind)

	lateByField := groupByField(recs, func(x domain.AutomationRecord) bool { return x.PastDeadlineAt(asOf) })
	instants := map[string]map[time.Time]identityclient.EscalationInstants{}
	for _, f := range sortedKeys(lateByField) {
		got := map[time.Time]identityclient.EscalationInstants{}
		missing := false
		for _, page := range pageInstants(lateByField[f], func(x domain.AutomationRecord) time.Time { return x.Deadline }) {
			ans, err := r.d.Identity.EscalationInstants(ctx, wk, f, page)
			if errors.Is(err, identityclient.ErrAutomationNotConfigured) {
				p.configMissing = true
				missing = true
				break
			}
			if err != nil {
				return p, err
			}
			for k, v := range ans {
				got[k] = v
			}
		}
		if !missing {
			instants[f] = got
		}
	}

	var late []domain.AutomationRecord
	for _, f := range sortedKeys(instants) {
		late = append(late, lateByField[f]...)
	}
	book, err := r.newRecipientBook(ctx, kind, late, len(late) > 0)
	if err != nil {
		return p, err
	}
	p.configMissing = p.configMissing || book.leadersMissing

	for _, x := range late {
		in := instants[x.SLARowKey()][x.Deadline.UTC()]
		// THE TWO LEVELS ARE EVALUATED INDEPENDENTLY — a row saved before the Y >= X rule may answer
		// the chairman's instant first (identity.proto, EscalationInstants.chairman_due_at).
		if !in.UnitHeadDueAt.IsZero() && !in.UnitHeadDueAt.After(asOf) {
			if to := book.unitHolders(x); len(to) > 0 {
				p.notices = append(p.notices, domain.EscalationNotice(kind, x, domain.EscalationUnitHead, to))
			} else {
				p.nobody(x.ID)
			}
		}
		if !in.ChairmanDueAt.IsZero() && !in.ChairmanDueAt.After(asOf) {
			if to := book.leadershipFor(x); len(to) > 0 {
				p.notices = append(p.notices, domain.EscalationNotice(kind, x, domain.EscalationChairman, to))
			} else {
				p.nobody(x.ID)
			}
		}
	}
	return p, nil
}

// --- weekly_digest ---------------------------------------------------------------------------------

// weeklyDigest sends the leadership this service's half of the Monday summary (ADR 0058 §8).
func (r *AutomationRunner) weeklyDigest(ctx context.Context, kind domain.AutomationWorkKind, asOf time.Time) (
	automationPlan, error) {

	var p automationPlan
	book, err := r.newRecipientBook(ctx, kind, nil, true)
	if err != nil {
		return p, err
	}
	p.configMissing = book.leadersMissing
	week := domain.ISOWeek(asOf)

	var (
		n          domain.StaffNotice
		summarised int
	)
	if kind == domain.AutomationCitizenReport {
		// "In the week" = the seven calendar days before the run, at the same local clock time.
		c, err := r.d.CitizenReports.CitizenReportDigestCounts(ctx, asOf, domain.SameClockDaysLater(asOf, -7))
		if err != nil {
			return p, fmt.Errorf("%w: %w", errAutomationStore, err)
		}
		p.examined = c.Examined
		summarised = c.Hot
		n = domain.CitizenReportDigestNotice(week, c, book.leaders)
	} else {
		recs, err := r.loadRecords(ctx, kind)
		if err != nil {
			return p, err
		}
		p.examined = len(recs)
		weekEnd := domain.SameClockDaysLater(asOf, 7)
		var pastDeadline, dueThisWeek, awaiting int
		for _, x := range recs {
			counted := false
			if x.PastDeadlineAt(asOf) {
				pastDeadline++
				counted = true
			} else if x.DueSoonAt(asOf, weekEnd) {
				dueThisWeek++
				counted = true
			}
			if x.Status == string(domain.ChoDuyet) {
				awaiting++
				counted = true
			}
			if counted {
				summarised++
			}
		}
		n = domain.TaskDigestNotice(week, pastDeadline, dueThisWeek, awaiting, book.leaders)
	}
	if len(book.leaders) == 0 {
		// A commune that flagged no leadership role has nobody to tell — never a fallback to
		// "everyone" (identity.proto, ResolveLeadershipStaff). Each record it would have named is late
		// or pending work nobody is being told about.
		p.digestWithoutRecipient = summarised
		return p, nil
	}
	p.notices = append(p.notices, n)
	return p, nil
}

// --- recipients ------------------------------------------------------------------------------------

// recipientBook holds, for ONE run, every recipient lookup it needs — asked once, batched.
type recipientBook struct {
	units          map[string][]string        // unit -> holders of the domain's assign key
	restrictedOK   map[string]map[string]bool // unit -> holders of feedback.restricted
	leaders        []string
	leadersMissing bool
}

func (r *AutomationRunner) newRecipientBook(ctx context.Context, kind domain.AutomationWorkKind,
	recs []domain.AutomationRecord, withLeaders bool) (*recipientBook, error) {

	b := &recipientBook{units: map[string][]string{}, restrictedOK: map[string]map[string]bool{}}
	assignKey := permTaskAssign
	if kind == domain.AutomationCitizenReport {
		assignKey = permFeedbackAssign
	}
	var units, restrictedUnits []string
	seen, seenR := map[string]bool{}, map[string]bool{}
	for _, x := range recs {
		if x.OrgUnitID == "" {
			continue
		}
		if !seen[x.OrgUnitID] {
			seen[x.OrgUnitID] = true
			units = append(units, x.OrgUnitID)
		}
		if x.Restricted && !seenR[x.OrgUnitID] {
			seenR[x.OrgUnitID] = true
			restrictedUnits = append(restrictedUnits, x.OrgUnitID)
		}
	}
	sort.Strings(units)
	sort.Strings(restrictedUnits)
	for _, page := range pageStrings(units, identityclient.MaxOrgUnitsPerCall) {
		got, err := r.d.Identity.OrgUnitPermissionHolders(ctx, page, assignKey)
		if err != nil {
			return nil, err
		}
		for u, codes := range got {
			b.units[u] = domain.CleanRecipients(codes)
		}
	}
	for _, page := range pageStrings(restrictedUnits, identityclient.MaxOrgUnitsPerCall) {
		got, err := r.d.Identity.OrgUnitPermissionHolders(ctx, page, permFeedbackRestricted)
		if err != nil {
			return nil, err
		}
		for u, codes := range got {
			set := map[string]bool{}
			for _, c := range codes {
				set[c] = true
			}
			b.restrictedOK[u] = set
		}
	}
	if withLeaders {
		leaders, err := r.d.Identity.LeadershipStaff(ctx)
		switch {
		case errors.Is(err, identityclient.ErrAutomationNotConfigured):
			// More than 50 flagged accounts: refused by identity rather than truncated. Nobody is told
			// from a guessed list; the run says the commune must fix its roles.
			b.leadersMissing = true
		case err != nil:
			return nil, err
		default:
			b.leaders = domain.CleanRecipients(leaders)
		}
	}
	return b, nil
}

// unitHolders are the people in the record's unit who hand out its work — for a restricted petition,
// only those who also hold `feedback.restricted`.
func (b *recipientBook) unitHolders(x domain.AutomationRecord) []string {
	if x.OrgUnitID == "" {
		return nil
	}
	all := b.units[x.OrgUnitID]
	if !x.Restricted {
		return all
	}
	ok := b.restrictedOK[x.OrgUnitID]
	var out []string
	for _, c := range all {
		if ok[c] {
			out = append(out, c)
		}
	}
	return out
}

// leadershipFor is the leadership, except for a restricted petition (see the file header).
func (b *recipientBook) leadershipFor(x domain.AutomationRecord) []string {
	if x.Restricted {
		return nil
	}
	return b.leaders
}

// chain is the file header's order: named holder → unit holders → leadership (if allowed).
func (b *recipientBook) chain(x domain.AutomationRecord, leadershipFallback bool) []string {
	if to := domain.CleanRecipients([]string{x.AssigneeMa}); len(to) > 0 {
		return to
	}
	if to := b.unitHolders(x); len(to) > 0 {
		return to
	}
	if leadershipFallback {
		return b.leadershipFor(x)
	}
	return nil
}

// --- delivery --------------------------------------------------------------------------------------

// deliver sends the plan in pages the contract accepts, in a stable order, and sums what comms
// CREATED (already-delivered duplicates are not counted — RecordAutomationRunOutcomeRequest).
//
// THE FIRST FAILED PAGE STOPS DELIVERY. Pages already accepted stay delivered, and the next run
// sends the same keys again, so nothing is doubled and nothing is lost.
func (r *AutomationRunner) deliver(ctx context.Context, notices []domain.StaffNotice) (uint32, error) {
	pages, err := automationPages(notices)
	if err != nil {
		return 0, err
	}
	var created uint32
	for _, page := range pages {
		got, err := r.d.Comms.DeliverStaffNotifications(ctx, page)
		if err != nil {
			return created, err
		}
		for _, d := range got {
			created += d.Created
		}
	}
	return created, nil
}

// automationPages turns the plan into comms pages: ≤ 100 notices, ≤ 1 000 recipients, ≤ 200 per
// notice, and a key at most once per page. Sorted by key, so a retried run sends the same requests.
func automationPages(notices []domain.StaffNotice) ([][]commsclient.Notice, error) {
	sorted := append([]domain.StaffNotice(nil), notices...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var parts []commsclient.Notice
	for _, n := range sorted {
		if !domain.ValidNoticeKey(n.Key) {
			return nil, fmt.Errorf("việc nền: khoá chống trùng không hợp lệ")
		}
		kind, ok := commsKind(n.Work, n.Kind)
		if !ok {
			return nil, fmt.Errorf("việc nền: loại thông báo không biết")
		}
		to := domain.CleanRecipients(n.Recipients)
		for start := 0; start < len(to); start += maxRecipientsPerNotice {
			end := min(start+maxRecipientsPerNotice, len(to))
			parts = append(parts, commsclient.Notice{IdempotencyKey: n.Key, Kind: kind,
				RecipientMa: to[start:end], Title: n.Title, Body: n.Body, Link: n.Link})
		}
	}

	var (
		pages      [][]commsclient.Notice
		cur        []commsclient.Notice
		keys       map[string]bool
		recipients int
	)
	flush := func() {
		if len(cur) > 0 {
			pages = append(pages, cur)
		}
		cur, keys, recipients = nil, map[string]bool{}, 0
	}
	flush()
	for _, part := range parts {
		if len(cur) == commsclient.MaxNoticesPerCall || keys[part.IdempotencyKey] ||
			recipients+len(part.RecipientMa) > commsclient.MaxRecipientsPerCall {
			flush()
		}
		cur = append(cur, part)
		keys[part.IdempotencyKey] = true
		recipients += len(part.RecipientMa)
	}
	flush()
	return pages, nil
}

// commsKind names the wire kind by the work the notice speaks about: a task's is a TASK kind (5–8), a
// petition's a PETITION kind (13–16), so a commune can switch each domain's reminders apart (ADR 0079
// lô 2 Q3); never the legacy 1–3. A comms build older than 5–16 refuses the batch: the run fails loudly
// and the next one resends the SAME keys. The kind is not part of comms' dedup (commune, key,
// recipient), so a notice delivered under the legacy kind earlier the same day is not delivered again.
//
// The weekly digest stays WEEKLY_DIGEST: comms does not split it per domain; the key tells the digests
// apart. An unknown work kind is refused, not defaulted to either domain.
func commsKind(work domain.AutomationWorkKind, k domain.NoticeKind) (commsv1.StaffNotificationKind, bool) {
	if k == domain.NoticeWeeklyDigest && (work == domain.AutomationTask || work == domain.AutomationCitizenReport) {
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST, true
	}
	switch work {
	case domain.AutomationTask:
		switch k {
		case domain.NoticeDueSoon:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_DUE_SOON, true
		case domain.NoticeOverdue:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_OVERDUE, true
		case domain.NoticeUnassigned:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_UNASSIGNED, true
		case domain.NoticeEscalation:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ESCALATION, true
		}
	case domain.AutomationCitizenReport:
		switch k {
		case domain.NoticeDueSoon:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_DUE_SOON, true
		case domain.NoticeOverdue:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_OVERDUE, true
		case domain.NoticeUnassigned:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_UNASSIGNED, true
		case domain.NoticeEscalation:
			return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ESCALATION, true
		}
	}
	return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED, false
}

// --- small helpers ---------------------------------------------------------------------------------

// distinctFields and groupByField key by SLARowKey — the code identity reads the `sla` row by.
func distinctFields(recs []domain.AutomationRecord, keep func(domain.AutomationRecord) bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range recs {
		if k := x.SLARowKey(); keep(x) && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func groupByField(recs []domain.AutomationRecord, keep func(domain.AutomationRecord) bool) map[string][]domain.AutomationRecord {
	out := map[string][]domain.AutomationRecord{}
	for _, x := range recs {
		if keep(x) {
			out[x.SLARowKey()] = append(out[x.SLARowKey()], x)
		}
	}
	return out
}

// pageInstants collects the DISTINCT instants of a group (UTC) in pages of the contract's 500.
func pageInstants(recs []domain.AutomationRecord, at func(domain.AutomationRecord) time.Time) [][]time.Time {
	seen := map[time.Time]bool{}
	var all []time.Time
	for _, x := range recs {
		t := at(x).UTC()
		if !seen[t] {
			seen[t] = true
			all = append(all, t)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Before(all[j]) })
	var pages [][]time.Time
	for start := 0; start < len(all); start += identityclient.MaxInstantsPerCall {
		pages = append(pages, all[start:min(start+identityclient.MaxInstantsPerCall, len(all))])
	}
	return pages
}

func pageStrings(s []string, size int) [][]string {
	var pages [][]string
	for start := 0; start < len(s); start += size {
		pages = append(pages, s[start:min(start+size, len(s))])
	}
	return pages
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func concat(lists ...[]domain.AutomationRecord) []domain.AutomationRecord {
	var out []domain.AutomationRecord
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}
