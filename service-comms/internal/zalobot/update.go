package zalobot

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
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
	// ChatType is message.chat.chat_type. Anything but ChatPrivate — GROUP, absent, unrecognised — is NOT a
	// one-to-one chat, and a personal pairing code is never acted on there (fail closed).
	ChatType ChatType
	// FromBot is message.from.is_bot. True only for a JSON true: an absent or odd value reads as a person,
	// which costs nothing — a group answers a person's free text with silence anyway.
	FromBot bool
}

// ChatType is the kind of chat an update came from (docs.zaloplatforms.com/docs/BOT/webhook:
// message.chat.chat_type is "PRIVATE" or "GROUP", the latter in Beta).
type ChatType int

const (
	// ChatTypeUnknown is the ZERO value on purpose: an Update built without a chat type is not private.
	ChatTypeUnknown ChatType = iota
	ChatTypePrivate
	ChatTypeGroup
)

// String is the type's name for a log line; the chat type is not personal data.
func (c ChatType) String() string {
	switch c {
	case ChatTypePrivate:
		return "private"
	case ChatTypeGroup:
		return "group"
	}
	return "unknown"
}

// fromIsBot reads message.from.is_bot: true only for a JSON true. is_bot ONLY — the sender's id and
// display_name are personal data nothing here needs, so they are never decoded.
func fromIsBot(raw json.RawMessage) bool {
	var f struct {
		IsBot json.RawMessage `json:"is_bot"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return false
	}
	return string(bytes.TrimSpace(f.IsBot)) == "true"
}

// parseChatType reads chat_type. Case and surrounding space are tolerated; any other value is Unknown.
func parseChatType(raw json.RawMessage) ChatType {
	s, ok := scalarText(raw)
	if !ok {
		return ChatTypeUnknown
	}
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PRIVATE":
		return ChatTypePrivate
	case "GROUP":
		return ChatTypeGroup
	}
	return ChatTypeUnknown
}

// ParseUpdate reads one webhook payload in EITHER shape (ADR 0074: "bộ đọc nhận cả gói phẳng lẫn gói
// bọc `result`"; ../vigov-require 08-tich-hop-ngoai.md: Zalo's documentation shows the wrapped shape,
// real deliveries arrive flat):
//
//	flat     {"event_name": "...", "message": {"chat": {"id": ..., "chat_type": "PRIVATE"|"GROUP"},
//	          "from": {"is_bot": false, ...}, "text": "...", "message_id": ...}}
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
				ID       json.RawMessage `json:"id"`
				ChatType json.RawMessage `json:"chat_type"`
			} `json:"chat"`
			// From is decoded on its own (fromIsBot): an odd "from" must not make an otherwise readable
			// update unreadable — it was not read at all before chat types mattered.
			From json.RawMessage `json:"from"`
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
	out := Update{EventName: u.EventName, ChatID: chatID, ChatType: parseChatType(u.Message.Chat.ChatType)}
	out.FromBot = fromIsBot(u.Message.From)
	if id, ok := scalarText(u.Message.MessageID); ok {
		out.MessageID = id
	}
	if u.Message.Text != nil {
		out.Text = *u.Message.Text
	}
	return out, nil
}
