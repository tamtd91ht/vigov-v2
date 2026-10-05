package config

// config.ZaloBotWebhook — ZALO_BOT_WEBHOOK_HOST (ADR 0074 #5, owner 05/10/2026).

import (
	"errors"
	"strings"
	"testing"
)

func TestZaloBotWebhookHostRequiredInProdWithItsHint(t *testing.T) {
	clean(t, EnvProd, nil)
	_, err := Load("comms", Uses(ZaloBotWebhook))
	if !errors.Is(err, ErrThieuBienMoiTruong) || !strings.Contains(err.Error(), "ZALO_BOT_WEBHOOK_HOST") {
		t.Fatalf("want a refusal naming ZALO_BOT_WEBHOOK_HOST, got %v", err)
	}
	// The refusal explains itself: what it is, where it comes from, where it goes.
	for _, want := range []string{"zalo-bot-updates", "bot.api.vigov.vn", "vigov-service-comms"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestZaloBotWebhookHostAccepted(t *testing.T) {
	for _, h := range []string{"bot.api.vigov.vn", "bot.api.stg.vigov.vn", " bot.api.vigov.vn\n", "bot.example.com"} {
		clean(t, EnvProd, map[string]string{"ZALO_BOT_WEBHOOK_HOST": h})
		cfg, err := Load("comms", Uses(ZaloBotWebhook))
		if err != nil {
			t.Errorf("%q refused: %v", h, err)
			continue
		}
		if cfg.ZaloBotWebhookHost() != strings.TrimSpace(h) {
			t.Errorf("%q loaded as %q", h, cfg.ZaloBotWebhookHost())
		}
	}
}

// Malformed or commune-shaped is FATAL in every environment, never repaired.
func TestZaloBotWebhookHostRefused(t *testing.T) {
	for _, h := range []string{
		"https://bot.api.vigov.vn",                 // scheme
		"bot.api.vigov.vn/api/v1/zalo-bot-updates", // path
		"bot.api.vigov.vn:443",                     // port
		"Bot.api.vigov.vn",                         // upper-case
		"bot.api.vigov.vn.",                        // trailing dot
		"10.0.0.5",                                 // IP literal
		"localhost",                                // not qualified
		"vigov.vn",                                 // the root
		"stg.vigov.vn",                             // the staging root
		"thangbinh-danang.vigov.vn",                // a commune's web host
		"thangbinh.stg.vigov.vn",                   // a commune's staging host
		"bot..vigov.vn",                            // empty label
	} {
		clean(t, EnvDev, map[string]string{"ZALO_BOT_WEBHOOK_HOST": h})
		_, err := Load("comms", Uses(ZaloBotWebhook))
		if !errors.Is(err, ErrZaloBotWebhookHostInvalid) {
			t.Errorf("%q: want ErrZaloBotWebhookHostInvalid, got %v", h, err)
			continue
		}
		if !strings.Contains(err.Error(), "comms") {
			t.Errorf("%q: the refusal does not name the service: %v", h, err)
		}
	}
}

// Only comms declares the group: a malformed value in the shared environment stops nobody else, and
// no other service can read it.
func TestZaloBotWebhookHostIgnoredWhenUndeclared(t *testing.T) {
	clean(t, EnvProd, map[string]string{"ZALO_BOT_WEBHOOK_HOST": "https://X:1/", "TRUSTED_PROXY_CIDRS": "10.42.0.0/16"})
	cfg, err := Load("platform", Uses(HTTPServer))
	if err != nil {
		t.Fatalf("an undeclared ZALO_BOT_WEBHOOK_HOST stopped platform: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("platform read ZALO_BOT_WEBHOOK_HOST without declaring ZaloBotWebhook")
		}
	}()
	_ = cfg.ZaloBotWebhookHost()
}

func TestZaloBotWebhookHostEmptyInDevIsOff(t *testing.T) {
	clean(t, EnvDev, nil)
	cfg, err := Load("comms", Uses(ZaloBotWebhook))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ZaloBotWebhookHost() != "" {
		t.Fatal("empty must read as off, never as a guess")
	}
}
