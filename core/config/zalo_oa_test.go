package config

// config.ZaloOA — ZALO_OA_APP_ID, ZALO_OA_APP_SECRET, ZALO_OA_CALLBACK_HOST (ADR 0098 #2, #3; owner
// 16/10/2026). Shaped like zalo_bot_test.go: the two hosts share one check and must be refused alike.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	zaloOAAppIDGia = "4000000000000000001"
	// Not a real key: a fake with the word in it so a reader never mistakes it for one.
	zaloOAAppSecretGia = "secret-app-zalo-GIA-KHONG-PHAI-KHOA-THAT"
)

func zaloOAEnvGia(host string) map[string]string {
	return map[string]string{
		"ZALO_OA_APP_ID":        zaloOAAppIDGia,
		"ZALO_OA_APP_SECRET":    zaloOAAppSecretGia,
		"ZALO_OA_CALLBACK_HOST": host,
	}
}

// All three are required in staging/prod, and the refusal explains each one: what it is, where the
// value comes from, which object it goes in — never a value.
func TestZaloOARequiredInProdWithItsHint(t *testing.T) {
	for _, env := range []string{EnvStaging, EnvProd} {
		clean(t, env, nil)
		_, err := Load("comms", Uses(ZaloOA))
		if !errors.Is(err, ErrThieuBienMoiTruong) {
			t.Fatalf("%s: want ErrThieuBienMoiTruong, got %v", env, err)
		}
		for _, want := range []string{
			"ZALO_OA_APP_ID", "ZALO_OA_APP_SECRET", "ZALO_OA_CALLBACK_HOST",
			"zalo-oa-authorizations", "developers.zalo.me", "ADR 0098",
			"common-config", "comms-secrets", "vigov-service-comms",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not say %q: %v", env, want, err)
			}
		}
	}
}

func TestZaloOAAccepted(t *testing.T) {
	for _, h := range []string{"bot.api.vigov.vn", "oa.api.vigov.vn", " bot.api.vigov.vn\n", "oa.example.com"} {
		clean(t, EnvProd, zaloOAEnvGia(h))
		cfg, err := Load("comms", Uses(ZaloOA))
		if err != nil {
			t.Errorf("%q refused: %v", h, err)
			continue
		}
		if cfg.ZaloOACallbackHost() != strings.TrimSpace(h) {
			t.Errorf("%q loaded as %q", h, cfg.ZaloOACallbackHost())
		}
		if cfg.ZaloOAAppID() != zaloOAAppIDGia {
			t.Errorf("app id loaded as %q", cfg.ZaloOAAppID())
		}
		if string(cfg.ZaloOAAppSecret().Lo()) != zaloOAAppSecretGia {
			t.Error("the secret did not survive Load intact")
		}
	}
}

// The app id is trimmed like every variable, then held to its shape: a pasted token, name or URL is
// refused rather than carried into every authorisation link the communes open.
func TestZaloOAAppIDShape(t *testing.T) {
	clean(t, EnvProd, zaloOAEnvGia("bot.api.vigov.vn"))
	// Trimmed, accepted.
	for _, id := range []string{"1", " 4000000000000000001\n", strings.Repeat("9", 32)} {
		clean(t, EnvDev, map[string]string{"ZALO_OA_APP_ID": id})
		cfg, err := Load("comms", Uses(ZaloOA))
		if err != nil {
			t.Errorf("%q refused: %v", id, err)
			continue
		}
		if cfg.ZaloOAAppID() != strings.TrimSpace(id) {
			t.Errorf("%q loaded as %q", id, cfg.ZaloOAAppID())
		}
	}
	// Refused, in every environment.
	for _, id := range []string{
		"abc",                   // a name
		"123abc",                // digits then letters
		"12.5",                  // not an integer
		"-1",                    // sign
		"4000 0000",             // inner space
		strings.Repeat("9", 33), // too long to be Zalo's long
		"https://developers.zalo.me/app/4000000000000000001", // the URL instead of the id
	} {
		clean(t, EnvDev, map[string]string{"ZALO_OA_APP_ID": id})
		_, err := Load("comms", Uses(ZaloOA))
		if !errors.Is(err, ErrZaloOAAppIDInvalid) {
			t.Errorf("%q: want ErrZaloOAAppIDInvalid, got %v", id, err)
			continue
		}
		if !strings.Contains(err.Error(), "comms") {
			t.Errorf("%q: the refusal does not name the service: %v", id, err)
		}
	}
}

// Malformed or commune-shaped is FATAL in every environment, never repaired — the ZALO_BOT_WEBHOOK_HOST
// list, verbatim, so the two hosts can never drift apart in what they refuse (ADR 0098 #3). The error
// is this variable's own: a refusal must name the host that is wrong, not the other one.
func TestZaloOACallbackHostRefused(t *testing.T) {
	for _, h := range []string{
		"https://bot.api.vigov.vn",                       // scheme
		"bot.api.vigov.vn/api/v1/zalo-oa-authorizations", // path
		"bot.api.vigov.vn:443",                           // port
		"Bot.api.vigov.vn",                               // upper-case
		"bot.api.vigov.vn.",                              // trailing dot
		"10.0.0.5",                                       // IP literal
		"localhost",                                      // not qualified
		"vigov.vn",                                       // the root
		"stg.vigov.vn",                                   // the staging root
		"thangbinh-danang.vigov.vn",                      // a commune's web host
		"thangbinh.stg.vigov.vn",                         // a commune's staging host
		"dev.vigov.vn",                                   // the dev root
		"thangbinh.dev.vigov.vn",                         // a commune's dev host
		"bot..vigov.vn",                                  // empty label
	} {
		clean(t, EnvDev, map[string]string{"ZALO_OA_CALLBACK_HOST": h})
		_, err := Load("comms", Uses(ZaloOA))
		if !errors.Is(err, ErrZaloOACallbackHostInvalid) {
			t.Errorf("%q: want ErrZaloOACallbackHostInvalid, got %v", h, err)
			continue
		}
		if errors.Is(err, ErrZaloBotWebhookHostInvalid) {
			t.Errorf("%q: the refusal blames ZALO_BOT_WEBHOOK_HOST: %v", h, err)
		}
		if !strings.Contains(err.Error(), "comms") {
			t.Errorf("%q: the refusal does not name the service: %v", h, err)
		}
	}
}

// Only comms declares the group: a malformed value in the shared environment stops nobody else, and
// no other service can read any of the three.
func TestZaloOAIgnoredWhenUndeclared(t *testing.T) {
	clean(t, EnvProd, map[string]string{
		"ZALO_OA_APP_ID":        "not-a-number",
		"ZALO_OA_APP_SECRET":    zaloOAAppSecretGia,
		"ZALO_OA_CALLBACK_HOST": "https://X:1/",
		"TRUSTED_PROXY_CIDRS":   "10.42.0.0/16",
	})
	cfg, err := Load("platform", Uses(HTTPServer))
	if err != nil {
		t.Fatalf("an undeclared ZaloOA value stopped platform: %v", err)
	}
	for name, read := range map[string]func(){
		"ZaloOAAppID":        func() { _ = cfg.ZaloOAAppID() },
		"ZaloOAAppSecret":    func() { _ = cfg.ZaloOAAppSecret() },
		"ZaloOACallbackHost": func() { _ = cfg.ZaloOACallbackHost() },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("platform read %s without declaring ZaloOA", name)
				}
			}()
			read()
		}()
	}
}

// All three empty in dev = the group is OFF: "" and nil, never a guess.
func TestZaloOAEmptyInDevIsOff(t *testing.T) {
	clean(t, EnvDev, nil)
	cfg, err := Load("comms", Uses(ZaloOA))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ZaloOAAppID() != "" || cfg.ZaloOACallbackHost() != "" || !cfg.ZaloOAAppSecret().Rong() {
		t.Fatal("empty must read as off, never as a guess")
	}
}

// The secret never renders — not through the accessor, not through the Config view (rule 8).
func TestZaloOAAppSecretNeverPrints(t *testing.T) {
	clean(t, EnvDev, zaloOAEnvGia("bot.api.vigov.vn"))
	cfg, err := Load("comms", Uses(ZaloOA))
	if err != nil {
		t.Fatal(err)
	}
	for what, s := range map[string]string{
		"%v":       fmt.Sprintf("%v", cfg),
		"%+v":      fmt.Sprintf("%+v", cfg),
		"%#v":      fmt.Sprintf("%#v", cfg),
		"%d":       fmt.Sprintf("%d", cfg.ZaloOAAppSecret()),
		"String":   cfg.ZaloOAAppSecret().String(),
		"LogValue": cfg.LogValue().String(),
	} {
		if strings.Contains(s, zaloOAAppSecretGia) {
			t.Errorf("%s renders ZALO_OA_APP_SECRET", what)
		}
	}
	// ...while the non-secret halves do render, so a startup line can say which app is in use.
	if s := fmt.Sprintf("%+v", cfg); !strings.Contains(s, zaloOAAppIDGia) || !strings.Contains(s, "bot.api.vigov.vn") {
		t.Errorf("the view hides the app id or the callback host: %s", s)
	}
}
