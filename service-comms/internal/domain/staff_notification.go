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
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The four kinds, stored Vietnamese without diacritics (ADR 0011). comms.proto names them in English
// on the wire; the mapping lives in internal/grpc. The CHECK on the table admits no fifth.
const (
	StaffNotificationDueSoon      = "sap-den-han"
	StaffNotificationOverdue      = "qua-han"
	StaffNotificationEscalation   = "leo-thang"
	StaffNotificationWeeklyDigest = "ban-tin-tuan"
)

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
)

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

		out = append(out, NotificationDelivery{
			IdempotencyKey: n.IdempotencyKey,
			Kind:           n.Kind,
			RecipientCodes: codes,
			Title:          title,
			Body:           body,
			Link:           n.Link,
		})
	}
	return out, nil
}

func knownKind(k string) bool {
	switch k {
	case StaffNotificationDueSoon, StaffNotificationOverdue, StaffNotificationEscalation,
		StaffNotificationWeeklyDigest:
		return true
	}
	return false
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
