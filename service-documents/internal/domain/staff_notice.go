package domain

// Staff notices fixed by ONE ACT of a person (ADR 0086 A1–A3; comms.proto StaffNotificationKind 22,
// `van-ban.chuyen-toi`): who is told, the idempotency key, and the Vietnamese sentences. No I/O — the
// outbox row is written by internal/app inside the act's own transaction, and the relay delivers it
// later. Same design as service-petitions/internal/domain/staff_notice.go.
//
// THE THREE COMMON RULES OF ADR 0086 A3, and where each one lives:
//
//	1. the actor is never a recipient       DropActor, applied by app.writeStaffNotice to every notice
//	2. the key is the ACT, not the record   ActNotice.Key: `van-ban.chuyen-toi:<id of the row the act
//	                                        wrote>` — the routing-timeline row of an incoming document,
//	                                        the log row of a citizen letter
//	3. only business codes                  the builders below take a register number, a year and a
//	                                        stored deadline — never `trich_yeu`, a letter's summary,
//	                                        its sender, or a routing reason (rule 3; ADR 0086 stop #3)
//
// WHY NO SUMMARY IN THE BODY, EVEN FOR AN INCOMING DOCUMENT: the prototype puts `summary[:280]` there
// for both registers (vigov-require apps/api/app/modules/documents/service.py:231), and comms.proto
// names exactly that line as one not to copy. An incoming document's `trich_yeu` is free text, and a
// document forwarding a citizen's letter carries that citizen's name in it (automation.go header) —
// and this notice leaves for Zalo, an outside service (rule 3, stop condition 2). The officer follows
// the link to a screen that checks their permission.

import (
	"strconv"
	"time"
)

// ActNoticeKind is the STORED value comms keeps for the kind (comms.proto, the `→` beside each value).
// Vietnamese, as enum values are (ADR 0011). app maps it onto the wire enum.
type ActNoticeKind string

// ActNoticeDocumentAssigned — an incoming document OR a citizen letter handed to a named officer. ONE
// family for both registers (comms.proto: "one family, as for 9–12").
const ActNoticeDocumentAssigned ActNoticeKind = "van-ban.chuyen-toi"

// The outbox row's `name` — the fact WITH ITS VERSION (rule 2, invariant 4), so the day a broker
// replaces the relay the rows already carry a name a consumer can subscribe to. Two names for one
// wire kind: the register is a fact a consumer may care about; the Zalo box is not.
const (
	EventIncomingDocumentAssigned = "incoming_documents.assigned.v1"
	EventCitizenLetterAssigned    = "citizen_letters.assigned.v1"
)

// ActNotice is one notice an act owes, before the actor is dropped.
type ActNotice struct {
	Kind ActNoticeKind
	// Event is the outbox row's `name`.
	Event string
	// ActRowID is the id of the row THE ACT wrote. The key is built from it (A3 #2), never from the
	// record: handing the same document back to the same officer after it went round is a new decision
	// and must notify again.
	ActRowID   string
	Recipients []string
	Title      string
	Body       string
	Link       string
}

// Key is `<stored value>:<act row id>` (comms.proto, "THE KEY IS THE ACT, NOT THE RECORD").
func (n ActNotice) Key() string { return string(n.Kind) + ":" + n.ActRowID }

// DropActor removes the acting staff member from the recipients (A3 #1: "người vừa bấm nút thì không
// báo"), then de-duplicates, drops codes comms would refuse, and sorts — so the row a retry would write
// is the row the first attempt wrote.
func DropActor(recipients []string, actor string) []string {
	clean := CleanRecipients(recipients)
	out := clean[:0]
	for _, c := range clean {
		if c != actor {
			out = append(out, c)
		}
	}
	return out
}

// registerNumber is "{số}/{năm}" — the prototype's `arrival_no/book_year`. The year is part of it
// because each register restarts its series every year: "số 7" alone names a document of every year.
func registerNumber(number, year int) string {
	return strconv.Itoa(number) + "/" + strconv.Itoa(year)
}

// IncomingAssignedNotice — kind 22 for an incoming document routed with a named officer. `routingRowID`
// is the `lich_su_chuyen_van_ban` row the act wrote. The body is the STORED processing deadline —
// business data the overdue notice already carries — never the summary (file header).
func IncomingAssignedNotice(v VanBanDen, routingRowID, officer string) ActNotice {
	body := ""
	if !v.HanXuLyXong.IsZero() {
		body = "Hạn xử lý: " + FormatLocalInstant(v.HanXuLyXong) + "."
	}
	return ActNotice{
		Kind: ActNoticeDocumentAssigned, Event: EventIncomingDocumentAssigned, ActRowID: routingRowID,
		Recipients: []string{officer},
		Title:      clip("Bạn được giao xử lý văn bản đến số "+registerNumber(v.SoVaoSo, v.Nam), noticeTitleMax),
		Link:       incomingRegisterLink,
		Body:       body,
	}
}

// LetterAssignedNotice — kind 22 for a citizen letter routed with a named officer. `logRowID` is the
// `citizen_letter_log` row the act wrote. BODY EMPTY, ALWAYS: no summary, no sender, not even the
// letter's type — a denunciation must not announce itself (ADR 0078 #4; rule 3).
func LetterAssignedNotice(l CitizenLetter, logRowID, officer string) ActNotice {
	return ActNotice{
		Kind: ActNoticeDocumentAssigned, Event: EventCitizenLetterAssigned, ActRowID: logRowID,
		Recipients: []string{officer},
		Title:      clip("Bạn được giao xử lý đơn thư số "+registerNumber(l.Number, l.Year), noticeTitleMax),
		Link:       letterRegisterLink,
	}
}

// --- scheduled_reports (ADR 0086 B1) ---------------------------------------------------------------

// ReportPeriod is the period a scheduled_reports run reports, as identity decided it (ADR 0086 B2) and
// as the key recipe spells it.
type ReportPeriod string

const (
	ReportWeek  ReportPeriod = "week"
	ReportMonth ReportPeriod = "month"
)

// ReportPeriodStart is the first day of the CURRENT period containing asOf, in Asia/Ho_Chi_Minh
// (ADR 0086 B1: "kỳ HIỆN TẠI vừa bắt đầu", day boundaries in Vietnamese time, ADR 0053 §3): the Monday
// of asOf's ISO week, or the 1st of its month. "2006-01-02". The second result is false for a period
// this code does not know — the caller refuses rather than picks one.
//
// A CALENDAR DATE, NOT A DEADLINE: it names the period in the idempotency key and nothing is counted
// from it (rule 10 concerns commitments; this is a label).
func ReportPeriodStart(p ReportPeriod, asOf time.Time) (string, bool) {
	l := asOf.In(AutomationZone)
	switch p {
	case ReportWeek:
		back := (int(l.Weekday()) + 6) % 7 // Monday = 0
		return time.Date(l.Year(), l.Month(), l.Day()-back, 0, 0, 0, 0, AutomationZone).Format("2006-01-02"), true
	case ReportMonth:
		return time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, AutomationZone).Format("2006-01-02"), true
	}
	return "", false
}

// ScheduledReportKey is comms.proto's recipe for kind 25:
// `scheduled_reports:<work_kind>:<week|month>:<YYYY-MM-DD of the period start>`. `<work_kind>` is
// spelled as in every other key of this service (`van-ban-den`, AutomationWorkKind), so one register
// has one name across all its keys.
func ScheduledReportKey(kind AutomationWorkKind, p ReportPeriod, periodStart string) string {
	return "scheduled_reports:" + string(kind) + ":" + string(p) + ":" + periodStart
}

// ScheduledReportNotice is this service's part of the scheduled report: ONE figure, the incoming
// register's backlog of overdue documents (ADR 0086 B1 — "documents (văn bản đến quá hạn)"). A backlog,
// not a count within the period (ADR 0086 §Hệ quả). Zero is printed: it is a measured figure of a
// register that exists, not a figure without data (ADR 0053 §6 drops only the latter).
//
// THE TITLE IS THE SHIPPED DEFAULT of system messages `report.notification.week|month`
// (service-reporting/internal/domain/system_message.go). A commune's own rewording lives in
// service-reporting's database, which this service may not read (rule 2) and which no RPC exposes —
// so a commune that reworded it still sees the default here. Stated, not hidden.
func ScheduledReportNotice(p ReportPeriod, periodStart string, overdue int, recipients []string) StaffNotice {
	title := "Báo cáo điều hành tuần đã sẵn sàng"
	if p == ReportMonth {
		title = "Báo cáo điều hành tháng đã sẵn sàng"
	}
	return StaffNotice{
		Key:        ScheduledReportKey(AutomationIncomingDocument, p, periodStart),
		Kind:       NoticeReportReady,
		Recipients: recipients,
		Title:      title,
		Body:       "Văn bản đến quá hạn: " + strconv.Itoa(overdue),
		Link:       incomingOverdueLink,
	}
}
