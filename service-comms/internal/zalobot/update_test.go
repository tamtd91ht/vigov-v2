package zalobot

import (
	"errors"
	"strings"
	"testing"
)

func TestParseUpdateReadsBothShapes(t *testing.T) {
	cases := map[string]string{
		"flat": `{"event_name":"message.text.received","message":{"message_id":"m-1",` +
			`"chat":{"id":"chat-ABC","chat_type":"PRIVATE"},"text":"K7QM2XPA","date":1749538250568}}`,
		"wrapped": `{"ok":true,"result":{"event_name":"message.text.received","message":{"message_id":"m-1",` +
			`"chat":{"id":"chat-ABC"},"text":"K7QM2XPA"}}}`,
	}
	for name, body := range cases {
		u, err := ParseUpdate([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if u.EventName != "message.text.received" || u.ChatID != "chat-ABC" || u.MessageID != "m-1" || u.Text != "K7QM2XPA" {
			t.Errorf("%s: = %+v", name, u)
		}
	}
}

func TestParseUpdateNumericIDsAndNoText(t *testing.T) {
	u, err := ParseUpdate([]byte(`{"event_name":"message.sticker.received","message":{"message_id":42,"chat":{"id":123456}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if u.ChatID != "123456" || u.MessageID != "42" || u.Text != "" {
		t.Fatalf("= %+v", u)
	}
}

// Every unreadable payload is the ONE sentinel, and its text quotes nothing of the body — the body is a
// staff member's message (personal data).
func TestParseUpdateRefusesWithoutQuoting(t *testing.T) {
	const marker = "NOI-DUNG-RIENG-0900000000"
	for _, body := range []string{
		``,
		`not json ` + marker,
		`{"event_name":"x","message":{"text":"` + marker + `"}}`, // no chat
		`{"event_name":"x"}`, // no message
		`{"ok":true,"result":{"message":{"chat":{"id":null},"text":"` + marker + `"}}}`,
		`{"message":{"chat":{"id":{"nested":1}}}}`,
		`{"message":{"chat":{"id":"` + strings.Repeat("9", 129) + `"}}}`,
		strings.Repeat(" ", maxUpdateBytes+1),
	} {
		_, err := ParseUpdate([]byte(body))
		if !errors.Is(err, ErrMalformedUpdate) {
			t.Errorf("%.40q: err = %v", body, err)
			continue
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("the error quotes the payload: %v", err)
		}
	}
}

// chat_type decides whether a personal pairing code may be acted on: only PRIVATE is private. GROUP, an
// absent value, an unrecognised one and a non-string one all read as NOT private (fail closed).
func TestParseUpdateChatType(t *testing.T) {
	const pre = `{"event_name":"message.text.received","message":{"message_id":"m-1","chat":{"id":"chat-ABC"`
	const post = `},"text":"K7QM2XPA"}}`
	cases := map[string]ChatType{
		`,"chat_type":"PRIVATE"`:  ChatTypePrivate,
		`,"chat_type":" private"`: ChatTypePrivate,
		`,"chat_type":"GROUP"`:    ChatTypeGroup,
		``:                        ChatTypeUnknown,
		`,"chat_type":null`:       ChatTypeUnknown,
		`,"chat_type":""`:         ChatTypeUnknown,
		`,"chat_type":"CHANNEL"`:  ChatTypeUnknown,
		`,"chat_type":1`:          ChatTypeUnknown,
		`,"chat_type":{"x":1}`:    ChatTypeUnknown,
	}
	for frag, want := range cases {
		u, err := ParseUpdate([]byte(pre + frag + post))
		if err != nil {
			t.Fatalf("%q: %v", frag, err)
		}
		if u.ChatType != want {
			t.Errorf("%q → %v, want %v", frag, u.ChatType, want)
		}
		if u.ChatID != "chat-ABC" || u.Text != "K7QM2XPA" {
			t.Errorf("%q: = %+v", frag, u)
		}
	}
	// The wrapped shape carries it too.
	u, err := ParseUpdate([]byte(`{"ok":true,"result":{"event_name":"message.text.received","message":` +
		`{"chat":{"id":"g-1","chat_type":"GROUP"},"from":{"id":"u-1","display_name":"X","is_bot":false},"text":"hi"}}}`))
	if err != nil || u.ChatType != ChatTypeGroup || u.FromBot {
		t.Fatalf("wrapped GROUP → %+v %v", u, err)
	}
	if ChatTypeUnknown != 0 {
		t.Fatal("an Update built without a chat type must not be private: Unknown is the zero value")
	}
}

func TestParseUpdateFromIsBot(t *testing.T) {
	const pre = `{"event_name":"message.text.received","message":{"chat":{"id":"g-1","chat_type":"GROUP"},"text":"x"`
	cases := map[string]bool{
		`,"from":{"id":"b-1","display_name":"Bot","is_bot":true}}}`:    true,
		`,"from":{"id":"u-1","display_name":"Người","is_bot":false}}}`: false,
		`,"from":{"id":"u-1"}}}`:      false,
		`,"from":{"is_bot":"true"}}}`: false,
		`}}`:                          false,
		// An odd "from" never makes the update unreadable: it was not read at all before.
		`,"from":"weird"}}`: false,
	}
	for frag, want := range cases {
		u, err := ParseUpdate([]byte(pre + frag))
		if err != nil {
			t.Fatalf("%q: %v", frag, err)
		}
		if u.FromBot != want {
			t.Errorf("%q → FromBot %v, want %v", frag, u.FromBot, want)
		}
	}
}
