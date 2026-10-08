package domain

// The COMMUNE side of the Zalo Bot reminder channel (ADR 0074; migrations/0018_zalo_bot.sql): the pairing
// code a member of staff types into the shared bot, the link it produces, the commune's channel settings,
// and the outbox rows that repeat a bell notice on Zalo. Standard library only.
//
// The platform side (the one shared bot, its token and webhook secret) is zalo_bot.go.

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ---- the pairing code ------------------------------------------------------------------------------

// The pairing-code rules ADR 0074 adopted from the spec ("mã ghép 8 ký tự không có ký tự dễ nhầm, sống 10
// phút, một lần, chỉ lưu bản băm; sai 3 lần huỷ mã"). 0018 keeps the FACTS (expiry, use, attempts); these
// are the policy, and they live here, once.
const (
	PairingCodeLength = 8
	// PairingCodeAlphabet has no look-alikes: no 0/O, no 1/I/L. 31 symbols, so 31^8 ≈ 8.5 × 10^11 codes —
	// against 5 tries per chat per hour (ratelimit.ZaloBotPairing) and a 10-minute life.
	PairingCodeAlphabet      = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	PairingCodeTTL           = 10 * time.Minute
	PairingMaxFailedAttempts = 3
)

// NormalizePairingCode reads what a person typed into the bot as a pairing code. Case-insensitive (the
// alphabet is upper-case only), surrounding space trimmed, and one space or hyphen in the middle tolerated
// ("ABCD EFGH", "abcd-efgh" — how a code read aloud gets typed). ok=false: not code-shaped at all, which is
// not a wrong code — nothing is counted for it.
func NormalizePairingCode(text string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(text))
	if len(s) == PairingCodeLength+1 && (s[4] == ' ' || s[4] == '-') {
		s = s[:4] + s[5:]
	}
	if len(s) != PairingCodeLength {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(PairingCodeAlphabet, rune(s[i])) {
			return "", false
		}
	}
	return s, true
}

// PairingCodeHash is what zalo_pairing_code.code_hash stores: SHA-256 of the NORMALISED code, 32 bytes
// (0018's CHECK). Never the code itself.
//
// PLAIN SHA-256, NOT HMAC: an HMAC needs a new platform secret, which is a rule 8 stop condition nobody has
// answered. What a stolen table would expose is a code that lives ten minutes and works once.
func PairingCodeHash(normalized string) []byte {
	sum := sha256.Sum256([]byte(normalized))
	return sum[:]
}

// ---- what a chat says to the bot -------------------------------------------------------------------

// ZaloCommand is a slash command typed into the shared bot.
type ZaloCommand int

const (
	ZaloCommandNone ZaloCommand = iota
	ZaloCommandHelp
	ZaloCommandStop
)

// ParseZaloCommand reads /trogiup /help /batdau /start (help) and /dung /stop (end this chat's link).
// Case-insensitive; anything after the command word is ignored (Zalo may append a start payload).
func ParseZaloCommand(text string) ZaloCommand {
	word, _, _ := strings.Cut(strings.TrimSpace(text), " ")
	switch strings.ToLower(word) {
	case "/trogiup", "/help", "/batdau", "/start":
		return ZaloCommandHelp
	case "/dung", "/stop":
		return ZaloCommandStop
	}
	return ZaloCommandNone
}

// The bot's replies — FIXED IN CODE for wave 1 (ADR 0074 #8). None carries personal data: not the staff
// member's name, not their code, not the commune (decision #7 allowed a name; nothing needs it).
const (
	ZaloReplyHelp = "Đây là kênh nhắc việc của ViGov cho cán bộ. Để ghép nối: đăng nhập ViGov, mở trang Cá nhân, " +
		"bấm \"Lấy mã ghép nối\" rồi gửi mã 8 ký tự vào đây. Gõ /dung để ngừng nhận tin."
	// ZaloReplyPairingRefused is ONE sentence for every refusal of a code-shaped message — wrong, expired,
	// used, cancelled, ambiguous between communes: telling them apart would tell a guesser which codes exist.
	ZaloReplyPairingRefused = "Mã ghép nối không đúng hoặc đã hết hạn. Vui lòng lấy mã mới trên ViGov rồi gửi lại."
	ZaloReplyPairingLimited = "Bạn đã thử quá nhiều lần. Vui lòng thử lại sau một giờ."
	ZaloReplyPaired         = "Đã ghép nối thành công. Từ nay bạn sẽ nhận nhắc việc của ViGov tại đây. Gõ /dung để ngừng."
	ZaloReplyStopped        = "Đã ngừng gửi nhắc việc tới chat này. Muốn nhận lại, hãy ghép nối lại trên ViGov."
	ZaloReplyNotLinked      = "Chat này chưa ghép nối với tài khoản ViGov nào."
	ZaloReplyUnsupported    = "Bot chỉ đọc được tin nhắn chữ. Gửi mã ghép nối 8 ký tự, hoặc gõ /trogiup."
	ZaloReplyUnavailable    = "Hệ thống tạm thời không xử lý được. Vui lòng thử lại sau ít phút."
	// ZaloTestMessageText is the fixed test message of `POST zalo-links/current/test-messages`.
	ZaloTestMessageText = "Tin thử từ ViGov: kênh nhắc việc qua Zalo của bạn đang hoạt động."
)

// Zalo's event names this service acts on (../vigov-require 08-tich-hop-ngoai.md).
const (
	ZaloEventTextReceived        = "message.text.received"
	ZaloEventUnsupportedReceived = "message.unsupported.received"
)

// ---- links -----------------------------------------------------------------------------------------

// ZaloLink is one live `zalo_link` row as the staff side reads it. chat_id is NOT a field: it is personal
// data (ADR 0074), and the only reader that needs it is the dispatcher's send, which reads it there.
type ZaloLink struct {
	ID        string
	StaffCode string
	LinkedAt  time.Time
	// BotRef is the bot the link was made through: SharedZaloBotRef, or a commune bot's Ref (0022).
	BotRef string
}

// The `zalo_link.unlink_reason` sentences (≤ 200 characters, 0018 CHECK) — fixed, like
// ZaloLinkEndedByBotChange.
const (
	ZaloLinkEndedByStaff     = "Cán bộ gỡ ghép nối Zalo trên ViGov"
	ZaloLinkEndedByRepair    = "Cán bộ ghép nối lại — liên kết cũ kết thúc"
	ZaloLinkEndedByStop      = "Cán bộ gõ /dung trong chat Zalo"
	ZaloLinkEndedByChatTaken = "Chat Zalo này đã được ghép cho một tài khoản khác"
)

// The commune audit_log verbs of this file (values, ADR 0011). ActionEndZaloLink is in zalo_bot.go.
const (
	ActionIssueZaloPairingCode   = "tao_ma_ghep_zalo"
	ActionPairZaloLink           = "ghep_lien_ket_zalo"
	ActionSaveZaloChannelSetting = "luu_cau_hinh_kenh_zalo"
	ActionSendZaloTestMessage    = "gui_tin_thu_zalo"
	ActionDispatchZaloReminders  = "gui_nhac_viec_zalo"
)

// ---- the commune's channel settings ----------------------------------------------------------------

// The per-domain reminder kinds of migration 0021 (ADR 0079 Q3 phase 1: every kind that ALREADY has a
// producer). VALUES, Vietnamese without diacritics, `<domain>.<shape>` (ADR 0011, 0051). The web maps
// each onto spec Cấu hình 11's event code where one exists (ADR 0079 Q1 #6).
const (
	ZaloKindTaskDueSoon        = "nhiem-vu.sap-den-han"
	ZaloKindTaskOverdue        = "nhiem-vu.qua-han"
	ZaloKindTaskUnassigned     = "nhiem-vu.chua-cu-nguoi"
	ZaloKindTaskEscalation     = "nhiem-vu.leo-thang"
	ZaloKindDocumentDueSoon    = "van-ban.sap-den-han"
	ZaloKindDocumentOverdue    = "van-ban.qua-han"
	ZaloKindDocumentUnassigned = "van-ban.chua-cu-nguoi"
	ZaloKindDocumentEscalation = "van-ban.leo-thang"
	ZaloKindPetitionDueSoon    = "phan-anh.sap-den-han"
	ZaloKindPetitionOverdue    = "phan-anh.qua-han"
	ZaloKindPetitionUnassigned = "phan-anh.chua-cu-nguoi"
	ZaloKindPetitionEscalation = "phan-anh.leo-thang"
	ZaloKindWeeklyDigest       = StaffNotificationWeeklyDigest // unchanged value: one digest event
)

// ZaloReminderKinds is what a commune may SELECT and SAVE, in the canonical order the settings are
// stored and shown in: 0021's twelve per-domain kinds, then the weekly digest. The four OLD values of
// 0010/0018 are not here: they stay valid in the database and are mapped at read (ZaloLegacyKindMap).
var ZaloReminderKinds = []string{
	ZaloKindTaskDueSoon, ZaloKindTaskOverdue, ZaloKindTaskUnassigned, ZaloKindTaskEscalation,
	ZaloKindDocumentDueSoon, ZaloKindDocumentOverdue, ZaloKindDocumentUnassigned, ZaloKindDocumentEscalation,
	ZaloKindPetitionDueSoon, ZaloKindPetitionOverdue, ZaloKindPetitionUnassigned, ZaloKindPetitionEscalation,
	ZaloKindWeeklyDigest,
}

// zaloOverdueKinds need the commune's overdue cadence (0021 zalo_channel_setting_overdue_kinds_need_cadence).
// Unassigned and escalation notices are one-shot per hold / per level and need none.
var zaloOverdueKinds = []string{ZaloKindTaskOverdue, ZaloKindDocumentOverdue, ZaloKindPetitionOverdue}

// ZaloLegacyKindMap is 0021's READ-TIME MAP — ONE table, used by the settings read, the dispatcher's kind
// test and the enqueue statement: an OLD value (stored in a settings row, or carried by a notice the
// producers still send until comms.proto changes) means exactly these per-domain kinds. `qua-han`
// includes the three unassigned kinds because unassigned notices were SENT as qua-han: a commune that
// ticked qua-han was receiving them, and the map keeps exactly what it was getting.
var ZaloLegacyKindMap = map[string][]string{
	StaffNotificationDueSoon: {ZaloKindTaskDueSoon, ZaloKindDocumentDueSoon, ZaloKindPetitionDueSoon},
	StaffNotificationOverdue: {ZaloKindTaskOverdue, ZaloKindDocumentOverdue, ZaloKindPetitionOverdue,
		ZaloKindTaskUnassigned, ZaloKindDocumentUnassigned, ZaloKindPetitionUnassigned},
	StaffNotificationEscalation:   {ZaloKindTaskEscalation, ZaloKindDocumentEscalation, ZaloKindPetitionEscalation},
	StaffNotificationWeeklyDigest: {ZaloKindWeeklyDigest},
}

// zaloKindMeanings is the set of per-domain kinds one value stands for: its expansion when it is an old
// value, itself when it is a selectable kind, nothing otherwise.
func zaloKindMeanings(kind string) []string {
	if m, ok := ZaloLegacyKindMap[kind]; ok {
		return m
	}
	if slices.Contains(ZaloReminderKinds, kind) {
		return []string{kind}
	}
	return nil
}

// ExpandZaloKinds maps a stored selection onto per-domain kinds: old values become what they mean,
// duplicates collapse, the order is ZaloReminderKinds'. A value that is neither (the CHECK admits none)
// is dropped — it can select nothing.
func ExpandZaloKinds(kinds []string) []string {
	want := map[string]bool{}
	for _, k := range kinds {
		for _, m := range zaloKindMeanings(k) {
			want[m] = true
		}
	}
	out := make([]string, 0, len(want))
	for _, k := range ZaloReminderKinds {
		if want[k] {
			out = append(out, k)
		}
	}
	return out
}

// ZaloKindSelected reports whether a notice of noticeKind passes a selection. A notice of an OLD kind
// (what producers send until the proto card) passes when ANY kind it maps to is selected; a selection
// still holding old values is read through the same map.
func ZaloKindSelected(selected []string, noticeKind string) bool {
	sel := ExpandZaloKinds(selected)
	for _, m := range zaloKindMeanings(noticeKind) {
		if slices.Contains(sel, m) {
			return true
		}
	}
	return false
}

// ZaloLegacyKindPairs flattens ZaloLegacyKindMap into two parallel lists — (old, per-domain) pairs — for
// the enqueue statement's unnest, so SQL reads the same table Go does instead of a copy of it.
func ZaloLegacyKindPairs() (legacy, perDomain []string) {
	for _, old := range []string{StaffNotificationDueSoon, StaffNotificationOverdue, StaffNotificationEscalation,
		StaffNotificationWeeklyDigest} {
		for _, k := range ZaloLegacyKindMap[old] {
			legacy = append(legacy, old)
			perDomain = append(perDomain, k)
		}
	}
	return legacy, perDomain
}

// The defaults a commune that never saved is SHOWN. Only the quiet window has one, and it MIRRORS 0018's
// column DEFAULT ('21:00' / '06:00', "giờ yên tĩnh mặc định 21h–6h giờ Việt Nam", ADR 0074);
// store/zalo_link_pg_test.go asserts the two agree. Kinds and the overdue cadence have NONE: the commune
// chooses them (0018: "a vendor value here would decide it for them").
const (
	DefaultQuietStartMinute = 21 * 60
	DefaultQuietEndMinute   = 6 * 60
)

// ZaloChannelSetting is one commune's `zalo_channel_setting`. Quiet hours are minutes after midnight,
// Vietnam local time. Saved=false: no row — the channel is OFF (0018: fail closed).
type ZaloChannelSetting struct {
	IsEnabled              bool
	Kinds                  []string
	QuietStartMinute       int
	QuietEndMinute         int
	OverdueStartAfterDays  *int
	OverdueRepeatEveryDays *int
	UpdatedAt              time.Time
	UpdatedBy              string
	Saved                  bool
	// LegacyKindsStored: the stored row still holds one of the four OLD values, and Kinds is its
	// read-time expansion (0021). A save is then never a no-op, so the next save writes per-domain values.
	LegacyKindsStored bool
}

// ReadStoredZaloKinds is the settings read's half of 0021: the stored kinds, expanded, and whether any
// stored value was an old one.
func ReadStoredZaloKinds(stored []string) (kinds []string, legacy bool) {
	for _, k := range stored {
		if _, old := ZaloLegacyKindMap[k]; old && k != ZaloKindWeeklyDigest {
			legacy = true
		}
	}
	return ExpandZaloKinds(stored), legacy
}

// DefaultZaloChannelSetting is what a commune with no row is shown: off, no kind, the default quiet window.
func DefaultZaloChannelSetting() ZaloChannelSetting {
	return ZaloChannelSetting{QuietStartMinute: DefaultQuietStartMinute, QuietEndMinute: DefaultQuietEndMinute,
		Kinds: []string{}}
}

// KindEnabled reports whether a notice of kind passes the commune's selection — through 0021's map, so
// a notice of an OLD kind passes when any per-domain kind it stands for is selected.
func (s ZaloChannelSetting) KindEnabled(kind string) bool { return ZaloKindSelected(s.Kinds, kind) }

// Same reports whether two settings carry the same values (the audit-free no-op save).
func (s ZaloChannelSetting) Same(o ZaloChannelSetting) bool {
	return s.IsEnabled == o.IsEnabled && slices.Equal(s.Kinds, o.Kinds) &&
		s.QuietStartMinute == o.QuietStartMinute && s.QuietEndMinute == o.QuietEndMinute &&
		eqIntPtr(s.OverdueStartAfterDays, o.OverdueStartAfterDays) &&
		eqIntPtr(s.OverdueRepeatEveryDays, o.OverdueRepeatEveryDays)
}

func eqIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ZaloSettingError is a refusal of a channel-settings save: a machine code and ONE Vietnamese sentence the
// screen shows as is. Each mirrors one 0018 CHECK, so the database never has to be the one to say no.
type ZaloSettingError struct {
	Code    string
	Message string
}

func (e *ZaloSettingError) Error() string { return "zalo_channel_setting: " + e.Code }

// ErrZaloSettingInvalid is matched by errors.Is on every *ZaloSettingError.
var ErrZaloSettingInvalid = errors.New("zalo_channel_setting: invalid")

func (e *ZaloSettingError) Is(target error) bool { return target == ErrZaloSettingInvalid }

func settingErr(code, msg string) error { return &ZaloSettingError{Code: code, Message: msg} }

// The bounds of 0018's range CHECKs ("sanity bounds, not policy").
const (
	MaxOverdueStartAfterDays  = 365
	MaxOverdueRepeatEveryDays = 365
)

// NormalizeZaloChannelSetting validates a save and returns it with the kinds de-duplicated in the
// canonical order. It refuses exactly what 0018/0021's CHECKs refuse.
//
// AN OLD VALUE IS ACCEPTED AND WRITTEN AS WHAT IT MEANS (0021's map): a client built before 0021 keeps
// working, and the row it saves holds per-domain values only — never an old one again.
func NormalizeZaloChannelSetting(in ZaloChannelSetting) (ZaloChannelSetting, error) {
	out := in
	out.LegacyKindsStored = false
	for _, k := range in.Kinds {
		if zaloKindMeanings(k) == nil {
			return ZaloChannelSetting{}, settingErr("unknown_kind",
				"Loại nhắc việc không hợp lệ. Hãy chọn trong danh sách sự kiện mà kênh Zalo hỗ trợ.")
		}
	}
	out.Kinds = ExpandZaloKinds(in.Kinds)
	if out.IsEnabled && len(out.Kinds) == 0 {
		return ZaloChannelSetting{}, settingErr("enabled_without_kind",
			"Bật kênh Zalo thì phải chọn ít nhất một loại nhắc việc.")
	}
	if !validMinute(out.QuietStartMinute) || !validMinute(out.QuietEndMinute) {
		return ZaloChannelSetting{}, settingErr("invalid_quiet_time", "Giờ yên tĩnh phải có dạng HH:MM, từ 00:00 đến 23:59.")
	}
	if out.QuietStartMinute == out.QuietEndMinute {
		return ZaloChannelSetting{}, settingErr("empty_quiet_window",
			"Giờ bắt đầu và giờ kết thúc yên tĩnh không được trùng nhau.")
	}
	if (out.OverdueStartAfterDays == nil) != (out.OverdueRepeatEveryDays == nil) {
		return ZaloChannelSetting{}, settingErr("overdue_cadence_incomplete",
			"Nhịp nhắc việc quá hạn cần đủ hai số: bắt đầu sau bao nhiêu ngày, và nhắc lại mỗi bao nhiêu ngày.")
	}
	if out.overdueKindSelected() && out.OverdueStartAfterDays == nil {
		return ZaloChannelSetting{}, settingErr("overdue_cadence_required",
			"Đã chọn nhắc việc quá hạn thì phải đặt nhịp nhắc: bắt đầu sau bao nhiêu ngày và nhắc lại mỗi bao nhiêu ngày.")
	}
	if d := out.OverdueStartAfterDays; d != nil && (*d < 0 || *d > MaxOverdueStartAfterDays) {
		return ZaloChannelSetting{}, settingErr("overdue_start_out_of_range",
			fmt.Sprintf("Số ngày bắt đầu nhắc việc quá hạn phải từ 0 đến %d.", MaxOverdueStartAfterDays))
	}
	if d := out.OverdueRepeatEveryDays; d != nil && (*d < 1 || *d > MaxOverdueRepeatEveryDays) {
		return ZaloChannelSetting{}, settingErr("overdue_repeat_out_of_range",
			fmt.Sprintf("Số ngày nhắc lại việc quá hạn phải từ 1 đến %d.", MaxOverdueRepeatEveryDays))
	}
	return out, nil
}

// overdueKindSelected — any of the three per-domain overdue kinds (0021's cadence CHECK).
func (s ZaloChannelSetting) overdueKindSelected() bool {
	for _, k := range zaloOverdueKinds {
		if slices.Contains(s.Kinds, k) {
			return true
		}
	}
	return false
}

func validMinute(m int) bool { return m >= 0 && m < 24*60 }

// ParseClock reads "HH:MM" (24-hour, two digits each) into minutes after midnight.
func ParseClock(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 || s[0] == '+' || s[3] == '+' {
		return 0, false
	}
	return h*60 + m, true
}

// FormatClock writes minutes after midnight as "HH:MM".
func FormatClock(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }

// ---- quiet hours -------------------------------------------------------------------------------------

// VietnamTime is Asia/Ho_Chi_Minh. A FIXED +07:00 ZONE rather than time.LoadLocation: Vietnam has kept
// UTC+7 without daylight saving since 1975, and a fixed zone needs no tzdata in the container — a missing
// tzdata is a quiet-hours check that fails at runtime on the night somebody is messaged at 2 a.m.
var VietnamTime = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

// QuietUntil reports whether `at` falls in the commune's quiet window (Vietnam local time; a window with
// start > end wraps midnight) and, if so, the instant the window ends — when a postponed message is due.
func QuietUntil(at time.Time, startMinute, endMinute int) (time.Time, bool) {
	local := at.In(VietnamTime)
	now := local.Hour()*60 + local.Minute()
	var quiet bool
	if startMinute < endMinute {
		quiet = now >= startMinute && now < endMinute
	} else {
		quiet = now >= startMinute || now < endMinute
	}
	if !quiet {
		return time.Time{}, false
	}
	end := time.Date(local.Year(), local.Month(), local.Day(), endMinute/60, endMinute%60, 0, 0, VietnamTime)
	if !end.After(local) {
		end = end.AddDate(0, 0, 1)
	}
	return end.UTC(), true
}

// ---- the outbox ----------------------------------------------------------------------------------------

// `zalo_delivery` values (0018, ADR 0011).
const (
	ZaloKindTest = "thu-nghiem"

	ZaloDeliveryPending = "cho-gui"
	ZaloDeliverySent    = "da-gui"
	ZaloDeliverySkipped = "bo-qua"
	ZaloDeliveryFailed  = "that-bai"

	ZaloSkipNotLinked        = "chua-lien-ket"
	ZaloSkipChannelOff       = "kenh-tat"
	ZaloSkipKindOff          = "loai-tat"
	ZaloSkipBotNotConfigured = "bot-chua-cau-hinh"
)

// The sender's retry policy — VENDOR choices, not customer figures (ADR 0074 fixes only WHICH failures are
// retried: 429 / 408 / 5xx). Six attempts with doubling back-off from one minute cover a Zalo outage of
// about an hour; a reminder later than that has lost its point.
const (
	ZaloDeliveryMaxAttempts = 6
	zaloBackoffBase         = time.Minute
	zaloBackoffCap          = 30 * time.Minute
)

// ZaloDeliveryBackoff is the wait before attempt n+1 after n failed attempts (n ≥ 1).
func ZaloDeliveryBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	d := zaloBackoffBase
	for i := 1; i < attempts && d < zaloBackoffCap; i++ {
		d *= 2
	}
	return min(d, zaloBackoffCap)
}

// ZaloReminderText is the message a bell notice becomes on Zalo (ADR 0074 #7: title + body; #8: fixed
// shape). link is ABSOLUTE or "": the bell's link is a web-admin path, which a phone cannot open without
// the commune's host in front of it. The adapter clips to Zalo's 2000 characters.
func ZaloReminderText(title, body, link string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(title))
	if s := strings.TrimSpace(body); s != "" {
		b.WriteString("\n")
		b.WriteString(s)
	}
	if link != "" {
		b.WriteString("\n")
		b.WriteString(link)
	}
	if !utf8.ValidString(b.String()) {
		return strings.ToValidUTF8(b.String(), "�")
	}
	return b.String()
}

// AbsoluteStaffLink joins the commune's staff host and the bell's RELATIVE link. "" when either is
// missing or the link is not a plain absolute path (0010 validates it relative; this does not trust that).
func AbsoluteStaffLink(host, link string) string {
	if host == "" || link == "" || !strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") ||
		strings.ContainsAny(link, " \t\r\n\\") {
		return ""
	}
	return "https://" + host + link
}
