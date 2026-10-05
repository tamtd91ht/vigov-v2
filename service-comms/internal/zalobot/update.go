package zalobot

import (
	"bytes"
	"encoding/json"
	"errors"
)

// ErrMalformedUpdate is every payload ParseUpdate cannot read. ONE error, carrying nothing of the
// payload: the body holds a staff member's message and chat id (personal data, ADR 0074), and a JSON
// decoder's error can quote a fragment of it.
var ErrMalformedUpdate = errors.New("zalobot: update payload is neither the flat nor the result-wrapped shape")

// maxUpdateBytes bounds what ParseUpdate reads; the webhook route caps the body before calling it too.
const maxUpdateBytes = 64 << 10

// Update is the part of one webhook update comms uses. chat_id IS PERSONAL DATA (ADR 0074): never
// logged, never returned to a client, never in a cache key — the webhook route keys a DIGEST of it.
type Update struct {
	// EventName is Zalo's event, e.g. "message.text.received". Empty when the payload carries none.
	EventName string
	// ChatID is the chat the message came from — what a pairing links to, and what sendMessage sends
	// to. Text whatever JSON type Zalo sends.
	ChatID string
	// MessageID is Zalo's id of the message, "" when absent. For de-duplicating a redelivery.
	MessageID string
	// Text is what the person typed ("" for a non-text event). A pairing code, or anything else.
	Text string
}

// ParseUpdate reads one webhook payload in EITHER shape (ADR 0074: "bộ đọc nhận cả gói phẳng lẫn gói
// bọc `result`"; ../vigov-require 08-tich-hop-ngoai.md: Zalo's documentation shows the wrapped shape,
// real deliveries arrive flat):
//
//	flat     {"event_name": "...", "message": {"chat": {"id": ...}, "text": "...", "message_id": ...}}
//	wrapped  {"ok": true, "result": {"event_name": "...", "message": {...}}}
//
// An update without a message or without a chat id is ErrMalformedUpdate: there is nothing to answer
// and nobody to answer.
func ParseUpdate(body []byte) (Update, error) {
	if len(body) == 0 || len(body) > maxUpdateBytes {
		return Update{}, ErrMalformedUpdate
	}
	var outer struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		return Update{}, ErrMalformedUpdate
	}
	payload := body
	if r := bytes.TrimSpace(outer.Result); len(r) > 0 && r[0] == '{' {
		payload = r
	}
	var u struct {
		EventName string `json:"event_name"`
		Message   *struct {
			MessageID json.RawMessage `json:"message_id"`
			Text      *string         `json:"text"`
			Chat      *struct {
				ID json.RawMessage `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	}
	if err := json.Unmarshal(payload, &u); err != nil {
		return Update{}, ErrMalformedUpdate
	}
	if u.Message == nil || u.Message.Chat == nil {
		return Update{}, ErrMalformedUpdate
	}
	chatID, ok := scalarText(u.Message.Chat.ID)
	if !ok || chatID == "" || len(chatID) > 128 {
		return Update{}, ErrMalformedUpdate
	}
	out := Update{EventName: u.EventName, ChatID: chatID}
	if id, ok := scalarText(u.Message.MessageID); ok {
		out.MessageID = id
	}
	if u.Message.Text != nil {
		out.Text = *u.Message.Text
	}
	return out, nil
}
