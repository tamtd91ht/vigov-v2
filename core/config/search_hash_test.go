package config

// config.SearchHash — SEARCH_HASH_KEY (ADR 0101 #8; owner 17/10/2026). Required in prod, base64 of
// at least 32 bytes, never echoed.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSearchHashKeyRequiredInProdWithItsHint(t *testing.T) {
	for _, env := range []string{EnvStaging, EnvProd} {
		clean(t, env, nil)
		_, err := Load("identity", Uses(SearchHash))
		if !errors.Is(err, ErrThieuBienMoiTruong) {
			t.Fatalf("%s: want ErrThieuBienMoiTruong, got %v", env, err)
		}
		for _, want := range []string{"SEARCH_HASH_KEY", "identity-secrets", "openssl rand -base64 32"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not say %q: %v", env, want, err)
			}
		}
	}
}

func TestSearchHashKeyDecodedAndNeverPrinted(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	enc := base64.StdEncoding.EncodeToString(raw)
	clean(t, EnvProd, map[string]string{"SEARCH_HASH_KEY": enc + "\n"})
	cfg, err := Load("identity", Uses(SearchHash))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got := cfg.SearchHashKey().Lo(); string(got) != string(raw) {
		t.Errorf("decoded key differs (%d bytes)", len(got))
	}
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, rendered := range []string{fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), string(js)} {
		if strings.Contains(rendered, enc) {
			t.Errorf("the key reaches a rendering: %s", rendered)
		}
	}
}

func TestSearchHashKeyRefusals(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 31))
	for name, v := range map[string]string{"not base64": "khong-phai-base64!!", "31 bytes": short} {
		clean(t, EnvDev, map[string]string{"SEARCH_HASH_KEY": v})
		_, err := Load("identity", Uses(SearchHash))
		if !errors.Is(err, ErrSearchHashKeyInvalid) {
			t.Errorf("%s: want ErrSearchHashKeyInvalid, got %v", name, err)
			continue
		}
		if strings.Contains(err.Error(), v) {
			t.Errorf("%s: the refusal echoes the value: %v", name, err)
		}
	}
}

func TestSearchHashKeyOffInDev(t *testing.T) {
	clean(t, EnvDev, nil)
	cfg, err := Load("identity", Uses(SearchHash))
	if err != nil {
		t.Fatalf("dev must start without the key: %v", err)
	}
	if !cfg.SearchHashKey().Rong() {
		t.Error("an unset key must read as empty")
	}
}
