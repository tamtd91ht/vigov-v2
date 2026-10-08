package domain

// StaffNotification — one notice in one staff member's header bell (docs/ui-ux/08-thong-bao.md §8),
// the entity of migrations/0010_staff_notification.sql, fed by CommsService.DeliverStaffNotifications
// (ADR 0058 §3).
//
// THIS FILE HOLDS THE CONTRACT'S LIMITS AS PURE FUNCTIONS. The table in comms.proto (1–100 notices,
// ≤ 1 000 recipients per call, key 1–200 printable ASCII, title 1–200, body ≤ 500, link ≤ 300 and
// relative) is enforced HERE, before any transaction opens, so a refused batch writes nothing — the
// contract's "all or nothing". The migration's CHECKs are the floor under it.
//
// THE REFUSAL MESSAGES NAME THE FIELD AND THE INDEX, NEVER THE VALUE. `title` and `body` are free
// text another service composed; echoing one into a gRPC status would carry it into that service's
// logs (rule 3, forbidden #3).

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The four OLD kinds of 0010, stored Vietnamese without diacritics (ADR 0011). comms.proto names them
// in English on the wire; the mapping lives in internal/grpc. They stay valid: producers send them until
// they move to the per-domain kinds (comms.proto StaffNotificationKind 5–16, the ZaloKind* constants of
// zalo_link.go), and 0021's CHECK admits both sets — nothing else.
const (
	StaffNotificationDueSoon      = "sap-den-han"
	StaffNotificationOverdue      = "qua-han"
	StaffNotificationEscalation   = "leo-thang"
	StaffNotificationWeeklyDigest = "ban-tin-tuan"
)

// StaffNotificationDisbursementMention — mentioned in a disbursement project's discussion (comms.proto
// STAFF_NOTIFICATION_KIND_DISBURSEMENT_MENTION, migration 0023, ADR 0081 #5). `<domain>.<shape>` like
// 0021's kinds, but BELL ONLY: it is in BellOnlyKinds, never in ZaloReminderKinds.
const StaffNotificationDisbursementMention = "giai-ngan.nhac-ten"

// BellOnlyKinds are kinds the bell stores and Zalo never carries (ADR 0081 #5: "không gửi Zalo"). A
// commune cannot select them (0021's settings CHECK does not admit them) and the Zalo enqueue never
// writes a row for them (ZaloQueueableKinds excludes them; 0023 leaves zalo_delivery.kind without them,
// so a row slipping past Go is refused by the database too).
var BellOnlyKinds = []string{StaffNotificationDisbursementMention}

// The contract's bounds (comms.proto, DeliverStaffNotificationsRequest and StaffNotification).
const (
	MaxNotificationsPerCall = 100
	MaxRecipientsPerCall    = 1000
	MaxRecipientsPerNotice  = 200
	MaxIdempotencyKeyLen    = 200
	MaxNotificationTitleLen = 200
	MaxNotificationBodyLen  = 500
	MaxNotificationLinkLen  = 300
	MaxRecipientCodeLen     = 64 // not in the proto: the point past which a "code" is not one
	notificationKeyMinPrint = 0x20
	notificationKeyMaxPrint = 0x7e

	// StaffNotification.due_soon_items (comms.proto, ADR 0079 lô 5 Q13).
	MaxDueSoonItems       = 500
	MaxDueSoonItemCodeLen = 100
)

// DueSoonItem is one record a due-soon digest names: its business code and its STORED deadline, copied by
// the producer as read (rule 10, invariant 2). Comms never derives a deadline or an overdue state from it;
// it only compares Deadline with the commune's Zalo lead (ZaloDueSoonCutoff).
type DueSoonItem struct {
	Code     string
	Deadline time.Time
}

// DueSoonKinds are the kinds a due_soon_items list may ride on (comms.proto: DUE_SOON 1, TASK_DUE_SOON 5,
// DOCUMENT_DUE_SOON 9, PETITION_DUE_SOON 13 — stored values here). Migration 0024's
// zalo_delivery_due_soon_items_shape CHECK names the same four.
var DueSoonKinds = []string{StaffNotificationDueSoon, ZaloKindTaskDueSoon, ZaloKindDocumentDueSoon,
	ZaloKindPetitionDueSoon}

// IsDueSoonKind reports whether kind is one of DueSoonKinds.
func IsDueSoonKind(kind string) bool { return slices.Contains(DueSoonKinds, kind) }

// StaffNotification is one row as the store reads it.
type StaffNotification struct {
	ID             string
	RecipientCode  string
	IdempotencyKey string
	Kind           string
	Title          string
	Body           string
	Link           string
	// ReadAt is the zero time while unread.
	ReadAt    time.Time
	CreatedAt time.Time
}

// Read reports whether the recipient has read it. Derived from ReadAt — never a second column.
func (n StaffNotification) Read() bool { return !n.ReadAt.IsZero() }

// NotificationDelivery is one notice of one DeliverStaffNotifications call, already validated:
// recipients trimmed and de-duplicated in first-seen order.
type NotificationDelivery struct {
	IdempotencyKey string
	Kind           string
	RecipientCodes []string
	Title          string
	Body           string
	Link           string
	// DueSoonItems shape the ZALO copy only (comms.proto StaffNotification.due_soon_items); the bell row
	// never sees them. Empty = an old producer: today's Zalo copy, never "drop it".
	DueSoonItems []DueSoonItem
}

// DeliveryOutcome is what one notice produced: rows created, and recipients who already had the key.
type DeliveryOutcome struct {
	IdempotencyKey   string
	Created          int
	AlreadyDelivered int
}

// ErrInvalidDelivery wraps every refusal of a batch. The gRPC layer maps it to INVALID_ARGUMENT and
// sends the message — which names a field and an index and nothing the caller wrote.
var ErrInvalidDelivery = errors.New("staff_notification: invalid delivery")

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDelivery, fmt.Sprintf(format, a...))
}

// ValidateDeliveries checks a whole call and returns it normalised, or the FIRST refusal. Nothing is
// partially accepted: one bad notice refuses the call.
func ValidateDeliveries(in []NotificationDelivery) ([]NotificationDelivery, error) {
	switch {
	case len(in) == 0:
		return nil, invalid("notifications is empty")
	case len(in) > MaxNotificationsPerCall:
		return nil, invalid("notifications has %d items, at most %d", len(in), MaxNotificationsPerCall)
	}
	out := make([]NotificationDelivery, 0, len(in))
	seenKey := make(map[string]int, len(in))
	total := 0
	for i, n := range in {
		if err := validateIdempotencyKey(n.IdempotencyKey); err != nil {
			return nil, invalid("notifications[%d].idempotency_key %s", i, err.Error())
		}
		if first, dup := seenKey[n.IdempotencyKey]; dup {
			return nil, invalid("notifications[%d].idempotency_key repeats notifications[%d]", i, first)
		}
		seenKey[n.IdempotencyKey] = i

		if !knownKind(n.Kind) {
			return nil, invalid("notifications[%d].kind is unspecified or unknown", i)
		}

		codes, err := normaliseRecipients(n.RecipientCodes)
		if err != nil {
			return nil, invalid("notifications[%d].recipient_ma %s", i, err.Error())
		}
		total += len(codes)
		if total > MaxRecipientsPerCall {
			return nil, invalid("the call addresses more than %d recipients in total", MaxRecipientsPerCall)
		}

		title := strings.TrimSpace(n.Title)
		switch {
		case title == "":
			return nil, invalid("notifications[%d].title is empty", i)
		case utf8.RuneCountInString(title) > MaxNotificationTitleLen:
			return nil, invalid("notifications[%d].title is longer than %d characters", i, MaxNotificationTitleLen)
		case !utf8.ValidString(title) || notificationHasControl(title, false):
			return nil, invalid("notifications[%d].title holds a control character or invalid UTF-8", i)
		}

		body := strings.TrimSpace(n.Body)
		switch {
		case utf8.RuneCountInString(body) > MaxNotificationBodyLen:
			return nil, invalid("notifications[%d].body is longer than %d characters", i, MaxNotificationBodyLen)
		case !utf8.ValidString(body) || notificationHasControl(body, true):
			return nil, invalid("notifications[%d].body holds a control character or invalid UTF-8", i)
		}

		if err := validateLink(n.Link); err != nil {
			return nil, invalid("notifications[%d].link %s", i, err.Error())
		}

		items, err := validateDueSoonItems(n.Kind, n.DueSoonItems)
		if err != nil {
			return nil, invalid("notifications[%d].due_soon_items%s", i, err.Error())
		}

		out = append(out, NotificationDelivery{
			IdempotencyKey: n.IdempotencyKey,
			Kind:           n.Kind,
			RecipientCodes: codes,
			Title:          title,
			Body:           body,
			Link:           n.Link,
			DueSoonItems:   items,
		})
	}
	return out, nil
}

// knownKind is the set 0023's staff_notification_kind_with_bell_only admits: the four old kinds, the twelve
// per-domain ones (ZaloReminderKinds, which also holds the weekly digest) and the bell-only kinds. One
// list for both, so the bell and the Zalo selection can never disagree on what a kind is.
func knownKind(k string) bool {
	return slices.Contains(ZaloQueueableKinds(), k) || slices.Contains(BellOnlyKinds, k)
}

// validateDueSoonItems is comms.proto's VALIDATION for StaffNotification.due_soon_items: none, or — on a
// due-soon kind only — at most 500 items, each code 1–100 printable characters and unique within the
// notice, each deadline set. The error text starts with the item's index or a space and names no code:
// a code is the producer's text, and the status message lands in the producer's logs.
//
// Codes are NOT trimmed or rewritten: the code is what the bell's body names, and a changed one would name
// a record that does not exist. Deadlines are kept as sent (UTC) — compared, never recomputed.
func validateDueSoonItems(kind string, in []DueSoonItem) ([]DueSoonItem, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if !IsDueSoonKind(kind) {
		return nil, errors.New(" is set on a kind that is not due soon")
	}
	if len(in) > MaxDueSoonItems {
		return nil, fmt.Errorf(" has %d items, at most %d", len(in), MaxDueSoonItems)
	}
	out := make([]DueSoonItem, 0, len(in))
	seen := make(map[string]int, len(in))
	for j, it := range in {
		n := utf8.RuneCountInString(it.Code)
		switch {
		case n == 0:
			return nil, fmt.Errorf("[%d].code is empty", j)
		case n > MaxDueSoonItemCodeLen:
			return nil, fmt.Errorf("[%d].code is longer than %d characters", j, MaxDueSoonItemCodeLen)
		case !utf8.ValidString(it.Code) || !allPrintable(it.Code):
			return nil, fmt.Errorf("[%d].code holds a character that is not printable", j)
		}
		if first, dup := seen[it.Code]; dup {
			return nil, fmt.Errorf("[%d].code repeats [%d]", j, first)
		}
		seen[it.Code] = j
		if it.Deadline.IsZero() {
			return nil, fmt.Errorf("[%d].deadline is not set", j)
		}
		out = append(out, DueSoonItem{Code: it.Code, Deadline: it.Deadline.UTC()})
	}
	return out, nil
}

// allPrintable: every rune is a graphic character or the ASCII space (unicode.IsPrint).
func allPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// validateIdempotencyKey: 1–200 printable ASCII characters, NOT trimmed — the key is opaque, and a
// key changed by this service is a key the caller's retry no longer matches.
func validateIdempotencyKey(k string) error {
	if k == "" {
		return errors.New("is empty")
	}
	if len(k) > MaxIdempotencyKeyLen {
		return fmt.Errorf("is longer than %d characters", MaxIdempotencyKeyLen)
	}
	for i := 0; i < len(k); i++ {
		if k[i] < notificationKeyMinPrint || k[i] > notificationKeyMaxPrint {
			return errors.New("holds a character outside printable ASCII")
		}
	}
	return nil
}

// normaliseRecipients trims, refuses blanks and malformed codes, and collapses duplicates keeping
// first-seen order ("duplicates collapsed", comms.proto).
func normaliseRecipients(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, errors.New("is empty")
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for j, raw := range in {
		c := strings.TrimSpace(raw)
		if c == "" {
			return nil, fmt.Errorf("[%d] is empty", j)
		}
		if len(c) > MaxRecipientCodeLen {
			return nil, fmt.Errorf("[%d] is longer than %d characters", j, MaxRecipientCodeLen)
		}
		for i := 0; i < len(c); i++ {
			if c[i] <= 0x20 || c[i] > notificationKeyMaxPrint {
				return nil, fmt.Errorf("[%d] is not a staff code", j)
			}
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) > MaxRecipientsPerNotice {
		return nil, fmt.Errorf("has %d distinct codes, at most %d", len(out), MaxRecipientsPerNotice)
	}
	return out, nil
}

// validateLink accepts "" or a RELATIVE path starting with a single "/". Anything with a scheme, a
// host, "//" or a backslash is refused: browsers read "/\evil" as "//evil", and a stored absolute URL
// is an open redirect in every recipient's bell (comms.proto, StaffNotification.link).
func validateLink(l string) error {
	if l == "" {
		return nil
	}
	switch {
	case len(l) > MaxNotificationLinkLen:
		return fmt.Errorf("is longer than %d characters", MaxNotificationLinkLen)
	case !strings.HasPrefix(l, "/") || strings.HasPrefix(l, "//"):
		return errors.New("is not a relative path starting with a single \"/\"")
	case strings.ContainsAny(l, "\\") || hasSpaceOrControl(l):
		return errors.New("holds a backslash, a space or a control character")
	}
	u, err := url.Parse(l)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return errors.New("is not a relative path")
	}
	return nil
}

func notificationHasControl(s string, allowNewline bool) bool {
	for _, r := range s {
		if allowNewline && r == '\n' {
			continue
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func hasSpaceOrControl(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}
