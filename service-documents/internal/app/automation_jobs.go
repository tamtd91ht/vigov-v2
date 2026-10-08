package app

// The three jobs over SỔ VĂN BẢN ĐẾN (ADR 0058 §4, §8; ADR 0029 §Bổ sung 29/09) and the delivery of
// their notices. Same design as service-petitions' internal/app/automation_jobs.go.
//
// A JOB FIRST BUILDS A PLAN, THEN DELIVERS IT. Every identity question is asked while planning; a
// failure there (outage, contract fault) stops the run with NOTHING delivered, and the next run redoes
// it under the same idempotency keys. FAILED_PRECONDITION (the commune has no usable `sla` row for
// `van-ban-den`) drops the notices that depended on it — nothing is ever sent from a default — and the
// run is recorded CONFIGURATION_MISSING, so the commune sees it on the configuration screen.
//
// ONE FIELD, "": the register has no `lĩnh vực` (app.VanBanDen.hanXuLyXong), so every identity
// question reads the commune's DEFAULT `van-ban-den` row — the same row the deadline was fixed from.
//
// WHO IS TOLD, in one chain for every job, fail closed at each step:
//
//	the named holder (`can_bo_xu_ly_ma`)
//	  →  the people in the holding unit who route its documents (`document.route`,
//	     ResolveOrgUnitPermissionHolders — the key the contract names for incoming documents)
//	  →  the leadership (ResolveLeadershipStaff) — the contract's fallback for work with neither
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
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// permDocumentRoute means "hands out this unit's documents" — a row of `quyen`
// (service-identity/migrations/0001_init.sql:296), named by identity.proto
// ResolveOrgUnitPermissionHolders for incoming documents.
const permDocumentRoute = "document.route"

// incomingWorkKind is the contract value every identity question of this runner carries.
const incomingWorkKind = identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN

// defaultSLARow is the `linh_vuc` of every question: the register has none (file header).
const defaultSLARow = ""

// maxRecipientsPerNotice is comms' bound on one notice (1 to 200); a longer list is split into
// several notices with the SAME key, in different calls — comms deduplicates per recipient.
const maxRecipientsPerNotice = 200

// automationPlan is what one run decided to say.
type automationPlan struct {
	notices  []domain.StaffNotice
	examined int
	// withoutRecipient holds the ids of documents that were due a notice and had nobody to receive it.
	withoutRecipient map[string]bool
	// digestWithoutRecipient counts the documents a digest summarised when there was no leadership.
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

// loadRecords reads the open documents. A store failure is a dependency failure.
func (r *AutomationRunner) loadRecords(ctx context.Context) ([]domain.AutomationRecord, error) {
	recs, err := r.d.Incoming.OpenIncomingForAutomation(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errAutomationStore, err)
	}
	return recs, nil
}

// --- sla_reminders ---------------------------------------------------------------------------------

func (r *AutomationRunner) slaReminders(ctx context.Context, asOf time.Time) (automationPlan, error) {
	var p automationPlan
	recs, err := r.loadRecords(ctx)
	if err != nil {
		return p, err
	}
	p.examined = len(recs)

	// 1. THE DUE-SOON CUTOFF — asked only when some deadline is still ahead.
	var (
		cutoff    time.Time
		hasCutoff bool
	)
	for _, x := range recs {
		if x.Deadline.After(asOf) {
			c, err := r.d.Identity.DueSoonCutoff(ctx, incomingWorkKind, defaultSLARow, asOf)
			switch {
			case errors.Is(err, identityclient.ErrDueSoonNotConfigured):
				p.configMissing = true
			case err != nil:
				return p, err
			default:
				cutoff, hasCutoff = c, true
			}
			break
		}
	}

	// 2. THE UNASSIGNED-HOLD INSTANTS, in pages of 500 distinct starts.
	var held []domain.AutomationRecord
	for _, x := range recs {
		if x.HeldUnassigned() {
			held = append(held, x)
		}
	}
	holdDue := map[time.Time]time.Time{}
	holdReported := len(held) > 0
	for _, page := range pageInstants(held, func(x domain.AutomationRecord) time.Time { return x.HoldStartedAt }) {
		off, got, err := r.d.Identity.UnassignedHoldInstants(ctx, incomingWorkKind, defaultSLARow, page)
		if errors.Is(err, identityclient.ErrAutomationNotConfigured) {
			p.configMissing = true
			holdReported = false
			break
		}
		if err != nil {
			return p, err
		}
		if off { // the commune's NULL: do not report — an answer, not a fault
			holdReported = false
			break
		}
		for k, v := range got {
			holdDue[k] = v
		}
	}

	// 3. WHAT IS DUE A NOTICE.
	var soon, late, unassigned []domain.AutomationRecord
	for _, x := range recs {
		if hasCutoff && x.DueSoonAt(asOf, cutoff) {
			soon = append(soon, x)
		} else if x.PastDeadlineAt(asOf) {
			late = append(late, x)
		}
		if holdReported && x.HeldUnassigned() {
			if at, ok := holdDue[x.HoldStartedAt.UTC()]; ok && !at.After(asOf) {
				unassigned = append(unassigned, x)
			}
		}
	}

	// 4. WHO — one batched lookup per unit set, leadership once. Only documents with no usable named
	// holder need their unit asked about.
	var unnamed []domain.AutomationRecord
	for _, l := range [][]domain.AutomationRecord{soon, late, unassigned} {
		for _, x := range l {
			if len(domain.CleanRecipients([]string{x.AssigneeMa})) == 0 {
				unnamed = append(unnamed, x)
			}
		}
	}
	book, err := r.newRecipientBook(ctx, unnamed, len(unnamed) > 0)
	if err != nil {
		return p, err
	}
	p.configMissing = p.configMissing || book.leadersMissing

	// 5. THE NOTICES.
	day := domain.LocalDay(asOf)
	perPerson := map[string][]domain.AutomationRecord{}
	for _, x := range soon {
		to := book.chain(x)
		if len(to) == 0 {
			p.nobody(x.ID)
			continue
		}
		for _, ma := range to {
			perPerson[ma] = append(perPerson[ma], x)
		}
	}
	for _, ma := range sortedKeys(perPerson) {
		p.notices = append(p.notices, domain.DueSoonNotice(day, ma, perPerson[ma]))
	}
	for _, x := range late {
		to := book.chain(x)
		if len(to) == 0 {
			p.nobody(x.ID)
			continue
		}
		p.notices = append(p.notices, domain.OverdueNotice(x, day, to))
	}
	for _, x := range unassigned {
		to := book.chain(x)
		if len(to) == 0 {
			p.nobody(x.ID)
			continue
		}
		p.notices = append(p.notices, domain.UnassignedNotice(x, to))
	}
	return p, nil
}

// --- escalation ------------------------------------------------------------------------------------

// escalation tells the unit head, then the leadership, once each, when a late document's lateness
// crosses the commune's two thresholds, counted from `han_xu_ly_xong` (the register's one clock).
//
// ⚠ ADR 0058 open question #3 (which kinds escalate) is still the user's; the contract allows every
// WorkKind and the caller's card asked for documents. A commune that does not want it switches the
// `escalation` job off — the setting is per job, so that also stops the petitions half.
func (r *AutomationRunner) escalation(ctx context.Context, asOf time.Time) (automationPlan, error) {
	var p automationPlan
	recs, err := r.loadRecords(ctx)
	if err != nil {
		return p, err
	}
	p.examined = len(recs)

	var late []domain.AutomationRecord
	for _, x := range recs {
		if x.PastDeadlineAt(asOf) {
			late = append(late, x)
		}
	}
	instants := map[time.Time]identityclient.EscalationInstants{}
	for _, page := range pageInstants(late, func(x domain.AutomationRecord) time.Time { return x.Deadline }) {
		ans, err := r.d.Identity.EscalationInstants(ctx, incomingWorkKind, defaultSLARow, page)
		if errors.Is(err, identityclient.ErrAutomationNotConfigured) {
			p.configMissing = true
			return p, nil // nothing escalates from a default
		}
		if err != nil {
			return p, err
		}
		for k, v := range ans {
			instants[k] = v
		}
	}

	book, err := r.newRecipientBook(ctx, late, len(late) > 0)
	if err != nil {
		return p, err
	}
	p.configMissing = p.configMissing || book.leadersMissing

	for _, x := range late {
		in := instants[x.Deadline.UTC()]
		// THE TWO LEVELS ARE EVALUATED INDEPENDENTLY — a row saved before the Y >= X rule may answer
		// the chairman's instant first (identity.proto, EscalationInstants.chairman_due_at).
		if !in.UnitHeadDueAt.IsZero() && !in.UnitHeadDueAt.After(asOf) {
			if to := book.unitHolders(x); len(to) > 0 {
				p.notices = append(p.notices, domain.EscalationNotice(x, domain.EscalationUnitHead, to))
			} else {
				p.nobody(x.ID)
			}
		}
		if !in.ChairmanDueAt.IsZero() && !in.ChairmanDueAt.After(asOf) {
			if len(book.leaders) > 0 {
				p.notices = append(p.notices, domain.EscalationNotice(x, domain.EscalationChairman, book.leaders))
			} else {
				p.nobody(x.ID)
			}
		}
	}
	return p, nil
}

// --- weekly_digest ---------------------------------------------------------------------------------

// weeklyDigest sends the leadership this service's part of the Monday summary (ADR 0058 §8, two
// parts by owning service). Documents part: overdue, and due within the next 7 calendar days. The
// đơn thư line the caller's card named is absent because the register is (domain/automation.go).
func (r *AutomationRunner) weeklyDigest(ctx context.Context, asOf time.Time) (automationPlan, error) {
	var p automationPlan
	book, err := r.newRecipientBook(ctx, nil, true)
	if err != nil {
		return p, err
	}
	p.configMissing = book.leadersMissing

	recs, err := r.loadRecords(ctx)
	if err != nil {
		return p, err
	}
	p.examined = len(recs)
	weekEnd := domain.SameClockDaysLater(asOf, 7)
	var pastDeadline, dueThisWeek int
	for _, x := range recs {
		if x.PastDeadlineAt(asOf) {
			pastDeadline++
		} else if x.DueSoonAt(asOf, weekEnd) {
			dueThisWeek++
		}
	}
	if len(book.leaders) == 0 {
		// A commune that flagged no leadership role has nobody to tell — never a fallback to
		// "everyone" (identity.proto, ResolveLeadershipStaff).
		p.digestWithoutRecipient = pastDeadline + dueThisWeek
		return p, nil
	}
	p.notices = append(p.notices, domain.IncomingDigestNotice(domain.ISOWeek(asOf), pastDeadline, dueThisWeek, book.leaders))
	return p, nil
}

// --- recipients ------------------------------------------------------------------------------------

// recipientBook holds, for ONE run, every recipient lookup it needs — asked once, batched.
type recipientBook struct {
	units          map[string][]string // unit -> holders of document.route
	leaders        []string
	leadersMissing bool
}

func (r *AutomationRunner) newRecipientBook(ctx context.Context, recs []domain.AutomationRecord,
	withLeaders bool) (*recipientBook, error) {

	b := &recipientBook{units: map[string][]string{}}
	var units []string
	seen := map[string]bool{}
	for _, x := range recs {
		if x.OrgUnitID != "" && !seen[x.OrgUnitID] {
			seen[x.OrgUnitID] = true
			units = append(units, x.OrgUnitID)
		}
	}
	sort.Strings(units)
	for _, page := range pageStrings(units, identityclient.MaxOrgUnitsPerCall) {
		got, err := r.d.Identity.OrgUnitPermissionHolders(ctx, page, permDocumentRoute)
		if err != nil {
			return nil, err
		}
		for u, codes := range got {
			b.units[u] = domain.CleanRecipients(codes)
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

// unitHolders are the people in the document's unit who route its documents.
func (b *recipientBook) unitHolders(x domain.AutomationRecord) []string {
	if x.OrgUnitID == "" {
		return nil
	}
	return b.units[x.OrgUnitID]
}

// chain is the file header's order: named holder → unit holders → leadership.
func (b *recipientBook) chain(x domain.AutomationRecord) []string {
	if to := domain.CleanRecipients([]string{x.AssigneeMa}); len(to) > 0 {
		return to
	}
	if to := b.unitHolders(x); len(to) > 0 {
		return to
	}
	return b.leaders
}

// --- delivery --------------------------------------------------------------------------------------

// deliver sends the plan in pages the contract accepts, in a stable order, and sums what comms
// CREATED (already-delivered duplicates are not counted). THE FIRST FAILED PAGE STOPS DELIVERY; pages
// already accepted stay delivered, and the next run sends the same keys again.
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
		kind, ok := commsKind(n.Kind)
		if !ok {
			return nil, fmt.Errorf("việc nền: loại thông báo không biết")
		}
		to := domain.CleanRecipients(n.Recipients)
		items := commsDueSoonItems(n.DueSoonItems)
		for start := 0; start < len(to); start += maxRecipientsPerNotice {
			end := min(start+maxRecipientsPerNotice, len(to))
			// A due-soon notice has one recipient, so it is never split; were it split, every part
			// speaks for the same documents and carries the same items.
			parts = append(parts, commsclient.Notice{IdempotencyKey: n.Key, Kind: kind,
				RecipientMa: to[start:end], Title: n.Title, Body: n.Body, Link: n.Link, DueSoonItems: items})
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

// commsDueSoonItems copies the plan's items as they are — code and STORED deadline, in the plan's
// order (domain.DueSoonNotice already de-duplicated, sorted and capped them). Nil for none.
func commsDueSoonItems(items []domain.DueSoonItem) []commsclient.DueSoonItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]commsclient.DueSoonItem, 0, len(items))
	for _, it := range items {
		out = append(out, commsclient.DueSoonItem{Code: it.Code, Deadline: it.Deadline})
	}
	return out
}

// commsKind names the wire kind. Every notice of this service is a DOCUMENT kind (9–12), so a commune
// can switch documents' reminders apart from tasks' and petitions' (ADR 0079 lô 2 Q3); never the
// legacy 1–3. A comms build older than 9–12 refuses the batch: the run fails loudly and the next one
// resends the SAME keys. The kind is not part of comms' dedup (commune, key, recipient), so a notice
// delivered under the legacy kind earlier the same day is not delivered again.
//
// The weekly digest stays WEEKLY_DIGEST: comms does not split it per domain; the key tells the digests apart.
func commsKind(k domain.NoticeKind) (commsv1.StaffNotificationKind, bool) {
	switch k {
	case domain.NoticeDueSoon:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_DUE_SOON, true
	case domain.NoticeOverdue:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_OVERDUE, true
	case domain.NoticeUnassigned:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_UNASSIGNED, true
	case domain.NoticeEscalation:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ESCALATION, true
	case domain.NoticeWeeklyDigest:
		return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST, true
	}
	return commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_UNSPECIFIED, false
}

// --- small helpers ---------------------------------------------------------------------------------

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
