package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidOperatorCode(t *testing.T) {
	for _, ok := range []string{"VH-00001", "VH-123456789"} {
		if !ValidOperatorCode(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "VH-0001", "vh-00001", "CB-00001", "VH-0000a", "system",
		"01JD8ZQK9M3NPXR7TVWYB2C4EF", "VH-" + strings.Repeat("1", 62), " VH-00001"} {
		if ValidOperatorCode(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestNormalizeActorIP(t *testing.T) {
	for in, want := range map[string]string{"": UnknownActorIP, " 10.0.0.5 ": "10.0.0.5", "::ffff:10.0.0.5": "10.0.0.5", "2001:db8::1": "2001:db8::1"} {
		got, err := NormalizeActorIP(in)
		if err != nil || got != want {
			t.Errorf("%q: = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"unknown", "10.0.0.5:443", "localhost"} {
		if _, err := NormalizeActorIP(bad); !errors.Is(err, ErrZaloBotInvalidArgument) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestNormalizeZaloBotName(t *testing.T) {
	if got, err := NormalizeZaloBotName("  Bot ViGov  "); err != nil || got != "Bot ViGov" {
		t.Fatalf("= %q, %v", got, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("ạ", 101), "Bot\nViGov", "\xff"} {
		if _, err := NormalizeZaloBotName(bad); !errors.Is(err, ErrZaloBotName) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
	if _, err := NormalizeZaloBotName(strings.Repeat("ạ", 100)); err != nil {
		t.Errorf("100 characters refused: %v", err)
	}
}

func TestValidateZaloChatURL(t *testing.T) {
	if err := ValidateZaloChatURL("https://zalo.me/1234567890"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "http://zalo.me/x", "zalo.me/x", "https://user:pw@zalo.me/x", "https:///x",
		"https://zalo.me/" + strings.Repeat("x", 290), " https://zalo.me/x", "https://zalo.me/a b", "javascript:alert(1)"} {
		if err := ValidateZaloChatURL(bad); !errors.Is(err, ErrZaloBotChatURL) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestZaloWebhookURL(t *testing.T) {
	if got := ZaloWebhookURL("bot.api.vigov.vn"); got != "https://bot.api.vigov.vn/api/v1/zalo-bot-updates" {
		t.Fatalf("= %q", got)
	}
	if ZaloWebhookURL("") != "" {
		t.Fatal("no host must give no URL, never a guess")
	}
}

// The unlink reason fits zalo_link_unlink_reason_length (≤ 200 characters).
func TestUnlinkReasonFitsItsColumn(t *testing.T) {
	if n := len([]rune(ZaloLinkEndedByBotChange)); n == 0 || n > 200 {
		t.Fatalf("%d characters", n)
	}
}
