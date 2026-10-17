package domain

// A ZALO OA WEBHOOK EVENT (owner decision 17/10/2026, TASK W-1). Zalo posts every event of every OA
// authorised to the platform's ONE shared app (ADR 0098) to ONE webhook URL; comms verifies it, finds the
// commune whose live OA it concerns, and forwards the WHOLE body unchanged to Kafka as
// comms.zalo_oa_event_received.v1.
//
// This file holds only the pure rules: read the few fields the decision needs, verify the signature,
// name the OA. Nothing here logs, and nothing here keeps the body.

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// ZaloOAEventSignatureHeader carries Zalo's signature: `mac=<hex sha256>`.
const ZaloOAEventSignatureHeader = "X-ZEvent-Signature"

// ErrZaloOAEventMalformed is a body that is not a JSON object, or lacks app_id / timestamp — no signature
// can be computed for it, so it is refused like a wrong signature.
var ErrZaloOAEventMalformed = errors.New("comms: zalo oa event: malformed body")

// ZaloOAEvent is what the webhook reads from an event: the two fields the signature covers, the event
// name, and the OA it concerns. Nothing about the user, the message or its text.
type ZaloOAEvent struct {
	AppID     string
	Timestamp string
	EventName string
	// OAID is the OA the event belongs to, by the owner's rule (ZaloOAEventOAID); "" when none applies.
	OAID string
}

// zaloOAIDShape is zalo_commune_oa.oa_id's CHECK (migration 0031): digits, 1–32. Anything else cannot be
// a live OA, so it is never sent to the database.
var zaloOAIDShape = regexp.MustCompile(`^[0-9]{1,32}$`)

type zaloOAEventWire struct {
	AppID     json.RawMessage `json:"app_id"`
	Timestamp json.RawMessage `json:"timestamp"`
	EventName string          `json:"event_name"`
	OAID      json.RawMessage `json:"oa_id"`
	Sender    struct {
		ID json.RawMessage `json:"id"`
	} `json:"sender"`
	Recipient struct {
		ID json.RawMessage `json:"id"`
	} `json:"recipient"`
}

// ParseZaloOAEvent reads the fields above from the raw body. The body itself is not kept.
func ParseZaloOAEvent(body []byte) (ZaloOAEvent, error) {
	var w zaloOAEventWire
	if err := json.Unmarshal(body, &w); err != nil {
		return ZaloOAEvent{}, ErrZaloOAEventMalformed
	}
	e := ZaloOAEvent{AppID: jsonScalar(w.AppID), Timestamp: jsonScalar(w.Timestamp), EventName: w.EventName}
	if e.AppID == "" || e.Timestamp == "" {
		return ZaloOAEvent{}, ErrZaloOAEventMalformed
	}
	e.OAID = zaloOAEventOAID(w)
	return e, nil
}

// zaloOAEventOAID is the OWNER'S RULE (17/10/2026), in this order:
//
//	top-level `oa_id`                       when present
//	`recipient.id`                          for an event whose name starts with `user_` (user → OA)
//	`sender.id`                             for an event whose name starts with `oa_`   (OA → user)
//	none                                    otherwise — the event is answered 200 and NOT published
//
// A value that is not OA-id-shaped is "none": fail closed, never a guess at another field.
func zaloOAEventOAID(w zaloOAEventWire) string {
	var id string
	switch {
	case jsonScalar(w.OAID) != "":
		id = jsonScalar(w.OAID)
	case strings.HasPrefix(w.EventName, "user_"):
		id = jsonScalar(w.Recipient.ID)
	case strings.HasPrefix(w.EventName, "oa_"):
		id = jsonScalar(w.Sender.ID)
	}
	if !zaloOAIDShape.MatchString(id) {
		return ""
	}
	return id
}

// jsonScalar is a JSON string's value, or a JSON number's literal text; "" for anything else. Zalo sends
// ids and the timestamp as strings, but the signature is over the TEXT, so a number is taken verbatim.
func jsonScalar(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return ""
		}
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) != nil {
		return ""
	}
	return n.String()
}

// VerifyZaloOAEventSignature reports whether header is Zalo's signature of body:
//
//	header = "mac=" + hex(sha256(app_id + body + timestamp + OA secret key))
//
// where app_id and timestamp are the body's own fields (Zalo's documented scheme). Compared in constant
// time. An empty secret never verifies anything.
func VerifyZaloOAEventSignature(header string, e ZaloOAEvent, body, secretKey []byte) bool {
	if len(secretKey) == 0 {
		return false
	}
	given, ok := strings.CutPrefix(strings.TrimSpace(header), "mac=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(strings.TrimSpace(given))
	if err != nil || len(got) != sha256.Size {
		return false
	}
	h := sha256.New()
	h.Write([]byte(e.AppID))
	h.Write(body)
	h.Write([]byte(e.Timestamp))
	h.Write(secretKey)
	return subtle.ConstantTimeCompare(got, h.Sum(nil)) == 1
}

// zaloOAEventNameShape bounds what of an event name may reach a log line: Zalo's names are lower-case
// words joined by underscores. Anything else is logged as a fixed word, never echoed.
var zaloOAEventNameShape = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// LoggableZaloOAEventName is the event name if it has Zalo's shape, "unrecognised" otherwise.
func LoggableZaloOAEventName(name string) string {
	if zaloOAEventNameShape.MatchString(name) {
		return name
	}
	return "unrecognised"
}
