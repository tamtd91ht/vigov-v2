package domain

// Staff notices fixed by ONE ACT of a person (ADR 0086 A1–A3; comms.proto StaffNotificationKind 18–24):
// who is told, the idempotency key, and the Vietnamese sentences. No I/O — the outbox row is written by
// internal/app inside the act's own transaction, and the relay delivers it later.
//
// THE THREE COMMON RULES OF ADR 0086 A3, and where each one lives:
//
//	1. the actor is never a recipient       DropActor, applied by app.writeStaffNotice to EVERY kind
//	2. the key is the ACT, not the record   ActNoticeKey: `<stored value>:<id of the row the act wrote>`
//	3. only business codes and staff titles  the builders below take a task's title (typed by staff), a
//	                                         register number, a lookup code and dates — never a
//	                                         petition's content, a citizen's rating comment, a letter's
//	                                         summary or an extension REASON (rule 3; ADR 0086 stop #3)
//
// THE ONE STAFF FREE TEXT THAT IS CARRIED is the mention's body: the first 280 characters of the log
// entry the colleague was mentioned in (TaskMentionNotice). It was asked for by the main session's
// design of this pass; it is staff text, not a petition's content, and it already lives in
// `nhat_ky_nhiem_vu`. It is stated here because it is the one place a sentence typed by an officer
// leaves this service.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ActNoticeKind is the STORED value comms keeps for the kind (comms.proto, the `→` beside each value).
// Vietnamese, as enum values are (ADR 0011). app maps it onto the wire enum.
type ActNoticeKind string

const (
	ActNoticeTaskAssigned           ActNoticeKind = "nhiem-vu.giao-moi"
	ActNoticeTaskExtensionRequested ActNoticeKind = "nhiem-vu.de-nghi-lui-han"
	ActNoticeTaskApprovalRequested  ActNoticeKind = "nhiem-vu.cho-duyet"
	ActNoticeTaskMention            ActNoticeKind = "nhiem-vu.nhac-ten"
	ActNoticePetitionAssigned       ActNoticeKind = "phan-anh.phan-cong"
	ActNoticePetitionReopened       ActNoticeKind = "phan-anh.mo-lai"
)

// EventName is the outbox row's `name` — the fact WITH ITS VERSION (rule 2, invariant 4), so the day a
// broker replaces the relay the rows already carry a name a consumer can subscribe to.
func (k ActNoticeKind) EventName() string {
	switch k {
	case ActNoticeTaskAssigned:
		return "tasks.assigned.v1"
	case ActNoticeTaskExtensionRequested:
		return "tasks.extension_requested.v1"
	case ActNoticeTaskApprovalRequested:
		return "tasks.approval_requested.v1"
	case ActNoticeTaskMention:
		return "tasks.mentioned.v1"
	case ActNoticePetitionAssigned:
		return "petitions.assigned.v1"
	case ActNoticePetitionReopened:
		return "petitions.reopened.v1"
	}
	return ""
}

// ActNotice is one notice an act owes, before the actor is dropped.
type ActNotice struct {
	Kind ActNoticeKind
	// ActRowID is the id of the row THE ACT wrote — a timeline row, a log entry, an extension request.
	// The key is built from it (A3 #2), never from the record: handing the same task back to the same
	// person after it went round is a new decision and must notify again.
	ActRowID   string
	Recipients []string
	Title      string
	Body       string
	Link       string
}

// Key is `<stored value>:<act row id>` (comms.proto, "THE KEY IS THE ACT, NOT THE RECORD").
func (n ActNotice) Key() string { return string(n.Kind) + ":" + n.ActRowID }

// MaxRecipientsPerStaffNotice is comms' bound on one notice's recipients (StaffNotification.recipient_ma:
// 1 to 200). A longer list is split by SplitRecipients into several outbox rows.
const MaxRecipientsPerStaffNotice = 200

// DropActor removes the acting staff member from the recipients (A3 #1: "người vừa bấm nút thì không
// báo"), then de-duplicates, drops codes comms would refuse, and sorts — so the row a retry would write
// is the row the first attempt wrote. An empty actor drops nothing (a citizen's act names no staff).
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

// SplitRecipients cuts a cleaned, sorted list into parts comms accepts, each with its own key: the first
// part keeps the act's key, the next ones append `:2`, `:3`. Deterministic because the list is sorted,
// so deduplication per (commune, key, recipient) still delivers each person once. In practice one part:
// the only list that could pass 200 is a unit's `task.assign` holders.
func SplitRecipients(key string, recipients []string) (keys []string, parts [][]string) {
	for i, start := 0, 0; start < len(recipients); i, start = i+1, start+MaxRecipientsPerStaffNotice {
		end := min(start+MaxRecipientsPerStaffNotice, len(recipients))
		k := key
		if i > 0 {
			k = fmt.Sprintf("%s:%d", key, i+1)
		}
		keys = append(keys, k)
		parts = append(parts, recipients[start:end])
	}
	return keys, parts
}

// --- recipients -----------------------------------------------------------------------------------

// TaskLeaderOrCreator is the recipient of kinds 19 and 20: the task's assigning leader, else its
// creator (ADR 0086 A2; user decision 09/10/2026 for 20). Both are staff business codes on the row.
func TaskLeaderOrCreator(n NhiemVu) []string {
	if strings.TrimSpace(n.LanhDaoGiaoViecMa) != "" {
		return []string{n.LanhDaoGiaoViecMa}
	}
	if strings.TrimSpace(n.NguoiTaoMa) != "" {
		return []string{n.NguoiTaoMa}
	}
	return nil
}

// TaskAssignedRecipients is kind 18's recipient: the named assignee; with none, the holders of
// `task.assign` in the task's unit — asked of identity BEFORE the transaction and handed in by unit id.
// A unit the caller could not resolve is simply absent from the map, and then nobody is told.
func TaskAssignedRecipients(n NhiemVu, unitHolders map[string][]string) []string {
	if strings.TrimSpace(n.NguoiThucHienMa) != "" {
		return []string{n.NguoiThucHienMa}
	}
	if n.BoPhanID == "" {
		return nil
	}
	return unitHolders[n.BoPhanID]
}

// --- the sentences --------------------------------------------------------------------------------

// formatLocalDate is "dd/mm/yyyy" in Asia/Ho_Chi_Minh — DISPLAY only, never arithmetic on a deadline.
func formatLocalDate(t time.Time) string { return t.In(AutomationZone).Format("02/01/2006") }

// TaskAssignedNotice — kind 18. "Bạn được giao nhiệm vụ: {tiêu đề}", body the deadline or nothing.
func TaskAssignedNotice(n NhiemVu, actRowID string, recipients []string) ActNotice {
	body := ""
	if !n.HanXuLy.IsZero() {
		body = "Hạn " + formatLocalDate(n.HanXuLy)
	}
	return ActNotice{
		Kind: ActNoticeTaskAssigned, ActRowID: actRowID, Recipients: recipients,
		Title: clip("Bạn được giao nhiệm vụ: "+n.TieuDe, noticeTitleMax),
		Body:  body,
		Link:  AutomationTask.RecordLink(n.Ma),
	}
}

// TaskExtensionRequestedNotice — kind 19. The requested date, NEVER the reason (ADR 0086 stop #3).
func TaskExtensionRequestedNotice(n NhiemVu, requestID string, newDue time.Time) ActNotice {
	return ActNotice{
		Kind: ActNoticeTaskExtensionRequested, ActRowID: requestID, Recipients: TaskLeaderOrCreator(n),
		Title: clip("Đề nghị lùi hạn: "+n.TieuDe, noticeTitleMax),
		Body:  "Đề nghị hạn mới " + formatLocalDate(newDue),
		Link:  AutomationTask.RecordLink(n.Ma),
	}
}

// TaskApprovalRequestedNotice — kind 20, on a move INTO `cho-duyet` only (the caller decides that).
func TaskApprovalRequestedNotice(n NhiemVu, statusRowID string) ActNotice {
	return ActNotice{
		Kind: ActNoticeTaskApprovalRequested, ActRowID: statusRowID, Recipients: TaskLeaderOrCreator(n),
		Title: clip("Nhiệm vụ chờ duyệt: "+n.TieuDe, noticeTitleMax),
		Link:  AutomationTask.RecordLink(n.Ma),
	}
}

// MentionBodyMax is how much of the log entry the mention notice carries (main session design 09/10).
const MentionBodyMax = 280

// TaskMentionNotice — kind 21. One notice per entry, every mentioned code (the actor is dropped later).
func TaskMentionNotice(n NhiemVu, logEntryID, entryText string, mentioned []string) ActNotice {
	return ActNotice{
		Kind: ActNoticeTaskMention, ActRowID: logEntryID, Recipients: mentioned,
		Title: clip("Bạn được nhắc trong nhiệm vụ: "+n.TieuDe, noticeTitleMax),
		Body:  clip(entryText, MentionBodyMax),
		Link:  AutomationTask.RecordLink(n.Ma),
	}
}

// PetitionAssignedNotice — kind 23. The lookup code only: no field label (a restricted petition, ADR
// 0030, would otherwise announce what it is about), no content.
func PetitionAssignedNotice(p PhieuPhanAnh, logRowID string) ActNotice {
	return ActNotice{
		Kind: ActNoticePetitionAssigned, ActRowID: logRowID, Recipients: []string{p.CanBoXuLyID},
		Title: clip("Bạn được giao xử lý phản ánh "+p.MaTraCuu, noticeTitleMax),
		Link:  AutomationCitizenReport.RecordLink(p.MaTraCuu),
	}
}

// PetitionReopenedNotice — kind 24. NEVER the citizen's comment, never the stars' text beyond the fact.
func PetitionReopenedNotice(p PhieuPhanAnh, logRowID string) ActNotice {
	return ActNotice{
		Kind: ActNoticePetitionReopened, ActRowID: logRowID, Recipients: []string{p.CanBoXuLyID},
		Title: clip("Phản ánh "+p.MaTraCuu+" bị mở lại do đánh giá thấp", noticeTitleMax),
		Link:  AutomationCitizenReport.RecordLink(p.MaTraCuu),
	}
}

// --- mentions in a task log entry -----------------------------------------------------------------

// MentionsMax bounds the mentioned colleagues of one entry — the bound finance uses for its discussion
// (service-finance domain.MentionsMax), so the two pickers behave alike.
const MentionsMax = 20

var (
	ErrMentionsTooMany = fmt.Errorf("nhật ký nhiệm vụ: `mentioned_staff_codes` nhắc quá nhiều người (tối đa %d)", MentionsMax)
	ErrMentionInvalid  = errors.New("nhật ký nhiệm vụ: `mentioned_staff_codes` có phần tử rỗng hoặc không phải mã cán bộ")
)

// NormaliseMentions trims, checks the SHAPE of and de-duplicates the mentioned staff codes, keeping the
// order picked. nil and [] both become an empty slice. Whether each code is a staff member of THIS
// commune is app's question to identity (the shape of finance's NormaliseMentions, plus that check).
func NormaliseMentions(codes []string) ([]string, error) {
	out := make([]string, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		if !ValidRecipientCode(code) || strings.ContainsFunc(code, unicode.IsSpace) {
			return nil, ErrMentionInvalid
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) > MentionsMax {
		return nil, ErrMentionsTooMany
	}
	return out, nil
}
