package config

// MINI_APP_PROFILE_CODE (mini_app_profile.go): never required, shape-checked, refused when set in
// staging/prod. Fake codes only.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const profileCodeFake = "AbCdEfGhIjKlMnOpQrStUvWx"

func TestMiniAppProfileCodeUnsetIsOffInEveryEnvironment(t *testing.T) {
	for _, env := range []string{EnvDev, EnvStaging, EnvProd} {
		t.Run(env, func(t *testing.T) {
			clean(t, env, nil)
			cfg, err := Load("platform", Uses(MiniAppProfile))
			if err != nil {
				t.Fatalf("unset must never be refused: %v", err)
			}
			if !cfg.MiniAppProfileCode().Rong() {
				t.Error("unset must read as empty — links carry no pf")
			}
		})
	}
}

func TestMiniAppProfileCodeSetInDevIsKept(t *testing.T) {
	// Surrounding blanks are trimmed (a newline pasted into a Secret).
	clean(t, EnvDev, map[string]string{"MINI_APP_PROFILE_CODE": " " + profileCodeFake + "\n"})
	cfg, err := Load("platform", Uses(MiniAppProfile))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(cfg.MiniAppProfileCode().Lo()); got != profileCodeFake {
		t.Errorf("got %q", got)
	}
}

// Set in staging or prod: every printed QR would open the dev cluster. Refused, and the refusal says
// what the variable is and where to remove it from — never the value.
func TestMiniAppProfileCodeSetInStrictEnvIsRefused(t *testing.T) {
	for _, env := range []string{EnvStaging, EnvProd} {
		t.Run(env, func(t *testing.T) {
			clean(t, env, map[string]string{"MINI_APP_PROFILE_CODE": profileCodeFake})
			_, err := Load("platform", Uses(MiniAppProfile))
			if !errors.Is(err, ErrMiniAppProfileCodeInStrictEnv) {
				t.Fatalf("want ErrMiniAppProfileCodeInStrictEnv, got %v", err)
			}
			msg := err.Error()
			for _, want := range []string{"MINI_APP_PROFILE_CODE", "ENV=" + env, "platform-secrets",
				"vigov-dev", "rollout restart deploy/vigov-service-platform", "pf="} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal does not say %q: %s", want, msg)
				}
			}
			if strings.Contains(msg, profileCodeFake) {
				t.Error("the refusal prints the code")
			}
		})
	}
}

// Even a MALFORMED value in prod is refused for being set at all — the operator's fix is the same:
// remove the key.
func TestMiniAppProfileCodeMalformedInProdIsRefusedAsSet(t *testing.T) {
	clean(t, EnvProd, map[string]string{"MINI_APP_PROFILE_CODE": "short"})
	if _, err := Load("platform", Uses(MiniAppProfile)); !errors.Is(err, ErrMiniAppProfileCodeInStrictEnv) {
		t.Fatalf("got %v", err)
	}
}

func TestMiniAppProfileCodeBadShapeIsRefused(t *testing.T) {
	for _, bad := range []string{
		"AbCdEfGhIjKlMnOpQrS",          // 19: one short
		strings.Repeat("a", 129),       // one long
		"AbCdEfGhIjKlMnOpQrStUv Wx",    // inner space
		"AbCdEfGhIjKlMnOpQrStUvWx&x=1", // would break out of the query
		"AbCdEfGhIjKlMnOpQrStUvWx%41",  // percent-escape
		"ÀbCdEfGhIjKlMnOpQrStUvWx",     // non-ASCII
	} {
		t.Run(bad, func(t *testing.T) {
			clean(t, EnvDev, map[string]string{"MINI_APP_PROFILE_CODE": bad})
			_, err := Load("platform", Uses(MiniAppProfile))
			if !errors.Is(err, ErrMiniAppProfileCodeInvalid) {
				t.Fatalf("want ErrMiniAppProfileCodeInvalid, got %v", err)
			}
			msg := err.Error()
			for _, want := range []string{"MINI_APP_PROFILE_CODE", "VIGOV_DEV_PROFILE_CODE", "platform-secrets", "20–128"} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal does not say %q: %s", want, msg)
				}
			}
			if strings.Contains(msg, bad) {
				t.Error("the refusal prints the value")
			}
		})
	}
	// The bounds themselves are accepted.
	for _, ok := range []string{strings.Repeat("A", 20), strings.Repeat("z", 128), "a-b_c-d_e-f_g-h_i-j_k"} {
		clean(t, EnvDev, map[string]string{"MINI_APP_PROFILE_CODE": ok})
		if _, err := Load("platform", Uses(MiniAppProfile)); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
}

// Undeclared, it is not read at all: a prod pod of another service that happens to see the key (a
// misplaced value in a shared object) is not stopped by it.
func TestMiniAppProfileCodeUndeclaredIsIgnored(t *testing.T) {
	clean(t, EnvProd, map[string]string{"MINI_APP_PROFILE_CODE": profileCodeFake})
	if _, err := Load("reporting", Uses()); err != nil {
		t.Fatalf("an undeclared MINI_APP_PROFILE_CODE stopped reporting: %v", err)
	}
}

// The code never reaches a rendering of the config.
func TestMiniAppProfileCodeNeverRendered(t *testing.T) {
	clean(t, EnvDev, map[string]string{"MINI_APP_PROFILE_CODE": profileCodeFake})
	cfg, err := Load("platform", Uses(MiniAppProfile))
	if err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	slog.New(slog.NewTextHandler(&b, nil)).Info("boot", "cfg", cfg)
	for name, out := range map[string]string{"%+v": fmt.Sprintf("%+v", cfg), "%#v": fmt.Sprintf("%#v", cfg),
		"json": string(j), "slog": b.String()} {
		if strings.Contains(out, profileCodeFake) {
			t.Errorf("%s prints the profile code", name)
		}
	}
}
