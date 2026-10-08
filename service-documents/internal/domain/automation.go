package domain

// The automation jobs' business rules that need no I/O (ADR 0058): which incoming documents a job
// speaks about, the idempotency keys of the notices, and the Vietnamese sentences the bell shows.
// Same design as service-petitions/internal/domain/automation.go; this is the documents half.
//
// NOTHING HERE COMPUTES A DEADLINE OR A LATENESS. Every threshold is an instant identity answered
// (ResolveDueSoonCutoff, ResolveEscalationInstants, ResolveUnassignedHoldInstants) and every test here
// is a comparison of two instants — rule 10, forbidden #2 and invariant 3.
//
// NO PERSONAL DATA AND NO FREE TEXT IN A NOTICE (rule 3; comms.proto "WHAT A NOTICE MAY SAY"). A notice
// names the document by its register code (MaVanBanDen, "VB-DEN-2026-0007") — never `trich_yeu` or
// `co_quan_ban_hanh`: both are free text a clerk typed about a government matter, and a document
// forwarding a citizen's letter carries that citizen's name in its summary. The recipient follows the
// link to a screen that checks their permission.
//
// ĐƠN THƯ IS NOT HERE, and that is a finding, not an omission: `don_thu` does not exist in this
// service (no table, no deadline column — migration 0004 has only the two document registers). The
// contract admits WORK_KIND_DON_THU for this runner, but there is no stored commitment to compare with,
// and inventing one is rule 10's stop condition #1. The runner therefore does not claim DON_THU scopes
// (app.automationKinds says why claiming them would be worse than not).

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AutomationWorkKind is the kind of work a run covers, spelled as identity's `sla.loai_viec` value
// (ADR 0011: enum values stay Vietnamese) so the idempotency key names the same vocabulary as the
// contract.
type AutomationWorkKind string

// AutomationIncomingDocument is `van-ban-den`, the one kind this runner executes today.
const AutomationIncomingDocument AutomationWorkKind = "van-ban-den"

// AutomationZone is Asia/Ho_Chi_Minh for the dates in idempotency keys and in the sentences. FIXED
// +07:00, the petitions runner's choice, so neither runner depends on tzdata for a key: Viet Nam has no
// daylight saving. Used for DISPLAY and DATE KEYS only — never to count time towards a deadline.
var AutomationZone = time.FixedZone("ICT", 7*3600)

// AutomationRecord is the runner's view of one OPEN incoming document — codes, instants and holders.
type AutomationRecord struct {
	// ID is the internal id: key material for the idempotency key, never shown.
	ID string
	// Code is what the notice shows: MaVanBanDen(nam, so_vao_so).
	Code string
	// Deadline is the stored commitment `han_xu_ly_xong` (NOT NULL in migration 0004).
	Deadline time.Time
	// OrgUnitID is identity's `bo_phan.id` holding the document (`bo_phan_dang_giu_id`); "" when none.
	OrgUnitID string
	// AssigneeMa is `can_bo_xu_ly_ma`, a staff business code; "" when nobody is named.
	AssigneeMa string
	// HoldStartedAt is when the unit began holding the document with nobody named — zero unless
	// OrgUnitID is set and AssigneeMa is empty. Derived by the store from the routing timeline.
	HoldStartedAt time.Time
}

// PastDeadlineAt is the DERIVED overdue test of THIS register: now strictly after the deadline — the
// same comparison as VanBanDen.QuaHan and the dashboard's lateAgainstPredicate, so a reminder speaks
// about exactly the documents /tong-quan and /van-ban?metric=overdue count. The store reads open
// documents only, so the "finished is never overdue" half of QuaHan is already applied. Never stored.
func (r AutomationRecord) PastDeadlineAt(asOf time.Time) bool {
	return !r.Deadline.IsZero() && asOf.After(r.Deadline)
}

// DueSoonAt is identity's comparison: asOf < deadline <= cutoff (identity.proto,
// ResolveDueSoonCutoffResponse). A deadline exactly at asOf is neither due soon nor (by QuaHan)
// overdue for that one instant; the next run, at least five minutes later, reports it as overdue.
func (r AutomationRecord) DueSoonAt(asOf, cutoff time.Time) bool {
	return !r.Deadline.IsZero() && r.Deadline.After(asOf) && !r.Deadline.After(cutoff)
}

// HeldUnassigned reports whether a unit holds the document with nobody named.
func (r AutomationRecord) HeldUnassigned() bool {
	return r.OrgUnitID != "" && r.AssigneeMa == "" && !r.HoldStartedAt.IsZero()
}

// EscalationLevel names who is told, as the key recipe spells it.
type EscalationLevel string

const (
	EscalationUnitHead EscalationLevel = "unit_head"
	EscalationChairman EscalationLevel = "chairman"
)

// NoticeKind mirrors comms' StaffNotificationKind without importing it (domain imports only the
// standard library). Every kind here is a DOCUMENT one on the wire (app.commsKind, ADR 0079 lô 2 Q3)
// except the weekly digest, which comms does not split per domain.
type NoticeKind int

const (
	NoticeDueSoon NoticeKind = iota + 1
	NoticeOverdue
	NoticeEscalation
	NoticeWeeklyDigest
	// NoticeUnassigned is the holding unit having named nobody past the commune's threshold — its own
	// kind since comms split it out of OVERDUE, so a commune can switch it separately.
	NoticeUnassigned
)

// StaffNotice is one notice before delivery.
type StaffNotice struct {
	Key        string
	Kind       NoticeKind
	Recipients []string
	Title      string
	Body       string
	Link       string
	// DueSoonItems is set on a due-soon notice only (comms.proto StaffNotification.due_soon_items):
	// the documents Body counts, each with its STORED `han_xu_ly_xong`, so comms can apply the
	// commune's Zalo lead to the Zalo copy. Built from the very records Body is built from — never a
	// second selection.
	DueSoonItems []DueSoonItem
}

// DueSoonItem is one document of a due-soon digest: its register code and its deadline as stored.
type DueSoonItem struct {
	Code     string
	Deadline time.Time
}

// Contract bounds of a notice (comms.proto StaffNotification).
const (
	noticeTitleMax = 200
	noticeBodyMax  = 500
	noticeKeyMax   = 200
	// MaxDueSoonItems is comms' bound on due_soon_items. Past it the EARLIEST deadlines are sent —
	// the ones a Zalo lead narrower than the bell window keeps first (comms.proto).
	MaxDueSoonItems = 500
)

// LocalDay is the Asia/Ho_Chi_Minh date of t, "2006-01-02" — the date the key recipes name.
func LocalDay(t time.Time) string { return t.In(AutomationZone).Format("2006-01-02") }

// ISOWeek is the ISO week of t's local date, "2006-W01".
func ISOWeek(t time.Time) string {
	y, w := t.In(AutomationZone).ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

// SameClockDaysLater is the instant `days` CALENDAR days after t, at the same local clock time.
//
// NOT A DEADLINE AND NOT A LATENESS — it bounds the weekly digest's window ("đến hạn trong 7 ngày
// tới"), a calendar window a leader reads as such. Built by time.Date's normalisation of the local
// date. Every commitment is still compared as stored; nothing here moves one.
func SameClockDaysLater(t time.Time, days int) time.Time {
	l := t.In(AutomationZone)
	return time.Date(l.Year(), l.Month(), l.Day()+days, l.Hour(), l.Minute(), l.Second(), l.Nanosecond(),
		AutomationZone).UTC()
}

// ValidRecipientCode is the shape comms can take: a non-empty printable-ASCII staff code of at most
// 64 bytes. A code failing it would make comms refuse the WHOLE page (all or nothing), so the runner
// drops it and counts the document as without recipient if nobody else remains.
func ValidRecipientCode(ma string) bool {
	if ma == "" || len(ma) > 64 {
		return false
	}
	for i := 0; i < len(ma); i++ {
		if ma[i] < 0x21 || ma[i] > 0x7e {
			return false
		}
	}
	return true
}

// CleanRecipients de-duplicates, drops invalid codes and sorts — so a retried run sends the same
// request, byte for byte.
func CleanRecipients(codes []string) []string {
	seen := make(map[string]bool, len(codes))
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if ValidRecipientCode(c) && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// --- idempotency keys: comms.proto's recipes, built from the BUSINESS ITEM, never the run -----------

// DueSoonKey is one digest per person per day; the recipient is appended because each person's digest
// carries a different list and one call may not repeat a key (the petitions runner's reading).
func DueSoonKey(kind AutomationWorkKind, day, recipient string) string {
	return "sla_reminders:due_soon:" + string(kind) + ":" + day + ":" + recipient
}

// OverdueKey is once per document per day late.
func OverdueKey(kind AutomationWorkKind, recordID, day string) string {
	return "sla_reminders:overdue:" + string(kind) + ":" + recordID + ":" + day
}

// UnassignedKey is once per HOLD EPISODE: a document routed back to a unit unassigned later starts a
// new episode and a new key.
func UnassignedKey(kind AutomationWorkKind, recordID string, holdStartedAt time.Time) string {
	return "sla_reminders:unassigned:" + string(kind) + ":" + recordID + ":" +
		strconv.FormatInt(holdStartedAt.UTC().Unix(), 10)
}

// EscalationKey is once per level, ever.
func EscalationKey(kind AutomationWorkKind, recordID string, level EscalationLevel) string {
	return "escalation:" + string(kind) + ":" + recordID + ":" + string(level)
}

// WeeklyDigestKey is once per ISO week.
func WeeklyDigestKey(kind AutomationWorkKind, week string) string {
	return "weekly_digest:" + string(kind) + ":" + week
}

// ValidNoticeKey is comms' key rule: 1–200 printable ASCII characters.
func ValidNoticeKey(k string) bool {
	if k == "" || len(k) > noticeKeyMax {
		return false
	}
	for i := 0; i < len(k); i++ {
		if k[i] < 0x20 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

// --- links: relative paths into web-admin's /van-ban (comms refuses a scheme or a host) -------------
//
// /van-ban reads only the drill-down parameters (web-admin/src/lib/drill-down.ts: `metric` of
// incoming-documents is arrived | open | overdue, and open/overdue take no period). It has no
// per-record query parameter, so a one-document notice links to the list that contains it and the
// title carries the code to find it by.

const (
	incomingRegisterLink = "/van-ban"
	incomingOverdueLink  = "/van-ban?metric=overdue"
	incomingOpenLink     = "/van-ban?metric=open"
)

// FormatLocalInstant is "17:00 ngày 26/09/2026" in Asia/Ho_Chi_Minh.
func FormatLocalInstant(t time.Time) string {
	return t.In(AutomationZone).Format("15:04 ngày 02/01/2006")
}

// DueSoonNotice is the per-person digest: "Bạn có 3 văn bản đến sắp đến hạn xử lý", listing codes.
// recs are the documents the runner chose for THIS recipient; title and body count every one of them,
// and the items are the same records.
func DueSoonNotice(day, recipient string, recs []AutomationRecord) StaffNotice {
	codes := make([]string, 0, len(recs))
	for _, x := range recs {
		codes = append(codes, x.Code)
	}
	sort.Strings(codes)
	return StaffNotice{
		Key:          DueSoonKey(AutomationIncomingDocument, day, recipient),
		Kind:         NoticeDueSoon,
		Recipients:   []string{recipient},
		Title:        fmt.Sprintf("Bạn có %d văn bản đến sắp đến hạn xử lý", len(codes)),
		Body:         codeList("Gồm: ", codes),
		Link:         incomingOpenLink,
		DueSoonItems: dueSoonItems(recs),
	}
}

// dueSoonItems copies each document's code and STORED deadline (rule 10, invariant 2 — nothing is
// computed), one item per code (comms refuses a repeated code; a repeat keeps its earliest deadline),
// earliest deadline first, at most MaxDueSoonItems. The order is total (deadline, then code), so a
// retried run sends the same request.
func dueSoonItems(recs []AutomationRecord) []DueSoonItem {
	byCode := make(map[string]time.Time, len(recs))
	for _, x := range recs {
		if d, ok := byCode[x.Code]; !ok || x.Deadline.Before(d) {
			byCode[x.Code] = x.Deadline
		}
	}
	out := make([]DueSoonItem, 0, len(byCode))
	for c, d := range byCode {
		out = append(out, DueSoonItem{Code: c, Deadline: d})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Deadline.Equal(out[j].Deadline) {
			return out[i].Deadline.Before(out[j].Deadline)
		}
		return out[i].Code < out[j].Code
	})
	if len(out) > MaxDueSoonItems {
		out = out[:MaxDueSoonItems]
	}
	return out
}

// OverdueNotice is one late document, once per day late.
func OverdueNotice(r AutomationRecord, day string, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        OverdueKey(AutomationIncomingDocument, r.ID, day),
		Kind:       NoticeOverdue,
		Recipients: recipients,
		Title:      clip("Văn bản đến "+r.Code+" đã quá hạn xử lý", noticeTitleMax),
		Body:       "Hạn xử lý: " + FormatLocalInstant(r.Deadline) + ".",
		Link:       incomingOverdueLink,
	}
}

// UnassignedNotice reports a unit holding the document with nobody named past the commune's
// threshold. Its own kind (DOCUMENT_UNASSIGNED on the wire); the key is the one it had when it was
// sent as OVERDUE, so a run straddling the switch does not tell anybody twice.
func UnassignedNotice(r AutomationRecord, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        UnassignedKey(AutomationIncomingDocument, r.ID, r.HoldStartedAt),
		Kind:       NoticeUnassigned,
		Recipients: recipients,
		Title:      clip("Văn bản đến "+r.Code+" chưa được phân công người xử lý", noticeTitleMax),
		Body: "Bộ phận nhận từ " + FormatLocalInstant(r.HoldStartedAt) +
			" nhưng chưa phân công người xử lý, đã quá thời gian xã quy định.",
		Link: incomingRegisterLink,
	}
}

// EscalationNotice is one late document crossing one threshold, once per level.
func EscalationNotice(r AutomationRecord, level EscalationLevel, recipients []string) StaffNotice {
	who := "trưởng bộ phận"
	if level == EscalationChairman {
		who = "lãnh đạo"
	}
	return StaffNotice{
		Key:        EscalationKey(AutomationIncomingDocument, r.ID, level),
		Kind:       NoticeEscalation,
		Recipients: recipients,
		Title:      clip("Báo cáo việc trễ hạn: văn bản đến "+r.Code, noticeTitleMax),
		Body: "Quá hạn xử lý từ " + FormatLocalInstant(r.Deadline) +
			", đã vượt ngưỡng báo " + who + " theo quy định của xã.",
		Link: incomingOverdueLink,
	}
}

// IncomingDigestNotice is the documents half of the Monday summary (ADR 0058 §8, two parts).
func IncomingDigestNotice(week string, pastDeadline, dueThisWeek int, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        WeeklyDigestKey(AutomationIncomingDocument, week),
		Kind:       NoticeWeeklyDigest,
		Recipients: recipients,
		Title:      "Bản tin đầu tuần: văn bản đến",
		Body: fmt.Sprintf("%d văn bản đến quá hạn xử lý, %d văn bản đến sắp đến hạn trong 7 ngày tới.",
			pastDeadline, dueThisWeek),
		Link: incomingOverdueLink,
	}
}

// codeList renders "Gồm: A, B, C và N mục khác." within the body bound.
func codeList(prefix string, codes []string) string {
	if len(codes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(prefix)
	for i, c := range codes {
		sep := ""
		if i > 0 {
			sep = ", "
		}
		tail := fmt.Sprintf(" và %d mục khác.", len(codes)-i)
		if len([]rune(b.String()+sep+c))+len([]rune(tail)) > noticeBodyMax {
			b.WriteString(tail)
			return b.String()
		}
		b.WriteString(sep + c)
	}
	b.WriteString(".")
	return b.String()
}

func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
