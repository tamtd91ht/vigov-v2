package domain

// The automation jobs' business rules that need no I/O (ADR 0058): which records a job speaks
// about, the idempotency keys of the notices, and the Vietnamese sentences the bell shows.
//
// NOTHING HERE COMPUTES A DEADLINE OR A LATENESS. Every threshold is an instant identity answered
// (ResolveDueSoonCutoff, ResolveEscalationInstants, ResolveUnassignedHoldInstants) and every test
// here is a comparison of two instants — rule 10, forbidden #2 and invariant 3: overdue is derived
// from the stored deadline and the run's `claimed_at`, never stored, never counted in wall-clock
// hours.
//
// NO CITIZEN PERSONAL DATA IN A NOTICE (rule 3; comms.proto "WHAT A NOTICE MAY SAY"). A notice names
// the record by its business code (a task's `ma`, a petition's lookup code) and a short label — never
// a petition's content, a reporter's name, number or address, and not a task's free-text title
// either (domain.OverdueItem gives the reason the title stays off such surfaces). The recipient
// follows the link to a screen that checks their permission.

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AutomationWorkKind is the kind of work a run covers, spelled as identity's `sla.loai_viec` values
// (ADR 0011: enum values stay Vietnamese) so the idempotency key names the same vocabulary as the
// contract.
type AutomationWorkKind string

const (
	AutomationTask          AutomationWorkKind = "nhiem-vu"
	AutomationCitizenReport AutomationWorkKind = "phan-anh"
)

// AutomationZone is Asia/Ho_Chi_Minh for the dates in idempotency keys and in the sentences.
// FIXED +07:00, the precedent of the register export (internal/http/task_register_export.go): Viet
// Nam has no daylight saving, and a fixed zone needs no tzdata in the image. Used for DISPLAY and
// for DATE KEYS only — never to count time towards a deadline.
var AutomationZone = time.FixedZone("ICT", 7*3600)

// AutomationRecord is the runner's view of one open record — codes, instants and holders only.
type AutomationRecord struct {
	// ID is the internal id: key material for the idempotency key, never shown.
	ID string
	// Code is what the notice shows: a task's `ma`, a petition's `ma_tra_cuu`.
	Code string
	// Field is the petition's `linh_vuc`; "" for tasks, which carry none.
	Field string
	// Priority is the task's `muc_uu_tien` (ADR 0079 lô 2 Q4 b); "" for petitions and for a task filed
	// with no priority. Identity reads the task's `sla` row by it, falling back to the default row.
	Priority string
	Status   string
	// Deadline is the stored commitment (`han_xu_ly`, `han_xu_ly_xong`); zero when there is none.
	Deadline time.Time
	// OrgUnitID is identity's `bo_phan.id` holding the record; "" when none.
	OrgUnitID string
	// AssigneeMa is the staff business code of the named holder; "" when nobody is named.
	AssigneeMa string
	// HoldStartedAt is when the unit began holding the record with nobody named — zero unless
	// OrgUnitID is set and AssigneeMa is empty. Derived by the store from the timeline (see there).
	HoldStartedAt time.Time
	// Restricted is a petition in the `can-bo` field (ADR 0030): it is never announced to anybody
	// who might not hold `feedback.restricted`.
	Restricted bool
}

// SLARowKey is the code identity looks the record's `sla` row up by: a petition's field, a task's
// priority (ADR 0079 lô 2 Q4 b). The store fills at most one of the two, so this never has to choose.
// "" asks for the default row — identity's DongTheoLinhVuc also falls back to it when the code has no
// row of its own. Reminder thresholds only: no deadline is computed from it here (rule 10).
func (r AutomationRecord) SLARowKey() string {
	if r.Priority != "" {
		return r.Priority
	}
	return r.Field
}

// HasDeadline reports whether a commitment is stored.
func (r AutomationRecord) HasDeadline() bool { return !r.Deadline.IsZero() }

// PastDeadlineAt is the DERIVED overdue test: deadline at or before asOf (identity.proto: "A deadline
// AT OR BEFORE as_of is OVERDUE"). Never stored.
func (r AutomationRecord) PastDeadlineAt(asOf time.Time) bool {
	return r.HasDeadline() && !r.Deadline.After(asOf)
}

// DueSoonAt is identity's comparison: asOf < deadline <= cutoff.
func (r AutomationRecord) DueSoonAt(asOf, cutoff time.Time) bool {
	return r.HasDeadline() && r.Deadline.After(asOf) && !r.Deadline.After(cutoff)
}

// HeldUnassigned reports whether a unit holds the record with nobody named.
func (r AutomationRecord) HeldUnassigned() bool {
	return r.OrgUnitID != "" && r.AssigneeMa == "" && !r.HoldStartedAt.IsZero()
}

// CitizenReportDigestCounts are the petition half of the weekly digest (ADR 0058 §8).
type CitizenReportDigestCounts struct {
	// Examined is every live petition counted over (restricted field excluded).
	Examined int
	// PastDeadline: open petitions whose resolve deadline is at or before the run's instant.
	PastDeadline int
	// LowRated: rated 1–2 stars within the window.
	LowRated int
	// Hot is "phản ánh nóng" — past deadline OR low-rated, each petition once.
	Hot int
}

// EscalationLevel names who is told, as the key recipe spells it.
type EscalationLevel string

const (
	EscalationUnitHead EscalationLevel = "unit_head"
	EscalationChairman EscalationLevel = "chairman"
)

// NoticeKind mirrors comms' StaffNotificationKind without importing it (domain imports only the
// standard library). On the wire every kind but the weekly digest is split by the notice's Work — a
// TASK_* or PETITION_* kind (app.commsKind, ADR 0079 lô 2 Q3); comms does not split the digest.
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
	Key  string
	Kind NoticeKind
	// Work is the kind of work the notice speaks about; with Kind it names the wire kind.
	Work       AutomationWorkKind
	Recipients []string
	Title      string
	Body       string
	Link       string
	// DueSoonItems is set on a due-soon notice only (comms.proto StaffNotification.due_soon_items):
	// the records Body counts, each with its STORED deadline, so comms can apply the commune's Zalo
	// lead to the Zalo copy. Built from the very records Body is built from — never a second
	// selection that could name a record the recipient may not see (ADR 0030).
	DueSoonItems []DueSoonItem
}

// DueSoonItem is one record of a due-soon digest: its business code and its deadline as stored.
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
// NOT A DEADLINE AND NOT A LATENESS — it bounds the weekly digest's windows ("đến hạn trong 7 ngày
// tới", "bị đánh giá trong 7 ngày qua"), which are calendar windows a leader reads as such. Built by
// time.Date's normalisation of the local date, so a window never gains or loses an hour. Every
// commitment is still compared as stored; nothing here moves one.
func SameClockDaysLater(t time.Time, days int) time.Time {
	l := t.In(AutomationZone)
	return time.Date(l.Year(), l.Month(), l.Day()+days, l.Hour(), l.Minute(), l.Second(), l.Nanosecond(),
		AutomationZone).UTC()
}

// ValidRecipientCode is the shape comms can take: a non-empty printable-ASCII staff code. A code
// failing it would make comms refuse the WHOLE page (INVALID_ARGUMENT, all or nothing), so the
// runner drops it instead and counts the record as without recipient if nobody else remains.
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
	sortStrings(out)
	return out
}

// --- idempotency keys: comms.proto's recipes, built from the BUSINESS ITEM, never the run -------

// DueSoonKey is one digest per person per day. The recipe is
// `sla_reminders:due_soon:<work_kind>:<YYYY-MM-DD>`; the recipient's code is APPENDED because each
// person's digest carries a different count, and one call may not repeat a key. Deduplication is per
// (commune, key, recipient) either way, so the promise — once per person per day — is unchanged.
func DueSoonKey(kind AutomationWorkKind, day, recipient string) string {
	return "sla_reminders:due_soon:" + string(kind) + ":" + day + ":" + recipient
}

// OverdueKey is once per record per day late.
func OverdueKey(kind AutomationWorkKind, recordID, day string) string {
	return "sla_reminders:overdue:" + string(kind) + ":" + recordID + ":" + day
}

// UnassignedKey is once per HOLD EPISODE: a record handed back to a unit unassigned later starts a
// new episode and a new key. The contract lists no recipe for it; this follows the others' shape.
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

// --- wording ---------------------------------------------------------------------------------------

func (k AutomationWorkKind) noun() string {
	if k == AutomationCitizenReport {
		return "phiếu phản ánh"
	}
	return "nhiệm vụ"
}

func (k AutomationWorkKind) listPath() string {
	if k == AutomationCitizenReport {
		return "/phan-anh"
	}
	return "/nhiem-vu"
}

// RecordLink leads to the register filtered on the record's code — a relative path (comms refuses
// anything with a scheme or a host). The code is a business code, not personal data.
func (k AutomationWorkKind) RecordLink(code string) string {
	return k.listPath() + "?q=" + url.QueryEscape(code)
}

// DueSoonLink leads to the list the digest counts. Tasks have the `soon=true` filter; the petition
// register has no such filter yet, so the link is the register itself.
func (k AutomationWorkKind) DueSoonLink() string {
	if k == AutomationTask {
		return "/nhiem-vu?soon=true"
	}
	return k.listPath()
}

// DigestLink leads to where the digest's figures drill down.
func (k AutomationWorkKind) DigestLink() string {
	if k == AutomationTask {
		return "/nhiem-vu?metric=overdue"
	}
	return k.listPath()
}

// FormatLocalInstant is "17:00 ngày 26/09/2026" in Asia/Ho_Chi_Minh.
func FormatLocalInstant(t time.Time) string {
	return t.In(AutomationZone).Format("15:04 ngày 02/01/2006")
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// DueSoonNotice is the per-person digest: "Bạn có 3 nhiệm vụ sắp đến hạn", listing the codes. recs are
// the records the runner chose for THIS recipient; title and body count every one of them, and the
// items are the same records — so the restricted-petition rule the runner applied holds for both.
func DueSoonNotice(kind AutomationWorkKind, day, recipient string, recs []AutomationRecord) StaffNotice {
	codes := make([]string, 0, len(recs))
	for _, x := range recs {
		codes = append(codes, x.Code)
	}
	sortStrings(codes)
	return StaffNotice{
		Key:          DueSoonKey(kind, day, recipient),
		Kind:         NoticeDueSoon,
		Work:         kind,
		Recipients:   []string{recipient},
		Title:        fmt.Sprintf("Bạn có %d %s sắp đến hạn xử lý", len(codes), kind.noun()),
		Body:         codeList("Gồm: ", codes),
		Link:         kind.DueSoonLink(),
		DueSoonItems: dueSoonItems(recs),
	}
}

// dueSoonItems copies each record's code and STORED deadline (rule 10, invariant 2 — nothing is
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

// OverdueNotice is one late record, once per day late.
func OverdueNotice(kind AutomationWorkKind, r AutomationRecord, day string, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        OverdueKey(kind, r.ID, day),
		Kind:       NoticeOverdue,
		Work:       kind,
		Recipients: recipients,
		Title:      clip(fmt.Sprintf("%s %s đã quá hạn xử lý", capitalise(kind.noun()), r.Code), noticeTitleMax),
		Body:       "Hạn xử lý: " + FormatLocalInstant(r.Deadline) + ".",
		Link:       kind.RecordLink(r.Code),
	}
}

// UnassignedNotice reports a unit holding the record with nobody named past the commune's threshold.
// Its own kind (TASK_UNASSIGNED / PETITION_UNASSIGNED on the wire); the key is the one it had when it
// was sent as OVERDUE, so a run straddling the switch does not tell anybody twice.
func UnassignedNotice(kind AutomationWorkKind, r AutomationRecord, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        UnassignedKey(kind, r.ID, r.HoldStartedAt),
		Kind:       NoticeUnassigned,
		Work:       kind,
		Recipients: recipients,
		Title:      clip(fmt.Sprintf("%s %s chưa được phân công người thực hiện", capitalise(kind.noun()), r.Code), noticeTitleMax),
		Body: "Bộ phận nhận từ " + FormatLocalInstant(r.HoldStartedAt) +
			" nhưng chưa phân công người thực hiện, đã quá thời gian xã quy định.",
		Link: kind.RecordLink(r.Code),
	}
}

// EscalationNotice is one late record crossing one threshold, once per level.
func EscalationNotice(kind AutomationWorkKind, r AutomationRecord, level EscalationLevel, recipients []string) StaffNotice {
	who := "trưởng bộ phận"
	if level == EscalationChairman {
		who = "lãnh đạo"
	}
	return StaffNotice{
		Key:        EscalationKey(kind, r.ID, level),
		Kind:       NoticeEscalation,
		Work:       kind,
		Recipients: recipients,
		Title:      clip(fmt.Sprintf("Báo cáo việc trễ hạn: %s %s", kind.noun(), r.Code), noticeTitleMax),
		Body: "Quá hạn xử lý từ " + FormatLocalInstant(r.Deadline) +
			", đã vượt ngưỡng báo " + who + " theo quy định của xã.",
		Link: kind.RecordLink(r.Code),
	}
}

// TaskDigestNotice is the task half of the Monday summary.
func TaskDigestNotice(week string, pastDeadline, dueThisWeek, awaitingApproval int, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        WeeklyDigestKey(AutomationTask, week),
		Kind:       NoticeWeeklyDigest,
		Work:       AutomationTask,
		Recipients: recipients,
		Title:      "Bản tin đầu tuần: nhiệm vụ",
		Body: fmt.Sprintf("%d nhiệm vụ quá hạn, %d nhiệm vụ đến hạn trong 7 ngày tới, %d nhiệm vụ chờ duyệt.",
			pastDeadline, dueThisWeek, awaitingApproval),
		Link: AutomationTask.DigestLink(),
	}
}

// CitizenReportDigestNotice is the petition half — "phản ánh nóng" = past deadline OR rated 1–2
// stars in the week (user, 2026-09-29).
func CitizenReportDigestNotice(week string, c CitizenReportDigestCounts, recipients []string) StaffNotice {
	return StaffNotice{
		Key:        WeeklyDigestKey(AutomationCitizenReport, week),
		Kind:       NoticeWeeklyDigest,
		Work:       AutomationCitizenReport,
		Recipients: recipients,
		Title:      "Bản tin đầu tuần: phản ánh",
		Body: fmt.Sprintf("%d phản ánh nóng: %d phiếu quá hạn xử lý, %d phiếu bị đánh giá 1–2 sao trong 7 ngày qua.",
			c.Hot, c.PastDeadline, c.LowRated),
		Link: AutomationCitizenReport.DigestLink(),
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
		rest := len(codes) - i
		tail := fmt.Sprintf(" và %d mục khác.", rest)
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

// sortStrings orders codes so a retried run builds the same request.
func sortStrings(s []string) { sort.Strings(s) }
