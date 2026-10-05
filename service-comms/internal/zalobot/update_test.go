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
