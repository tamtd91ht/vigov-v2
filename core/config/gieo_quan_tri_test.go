package config

// IDENTITY_ADMIN_SEED_PASSWORD — the default-administrator password (owner's decision, 2026-09-26).

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// matKhauGieoGia is a fake seed password; the text says so (rule 8, forbidden #1).
const matKhauGieoGia = "mat-khau-gieo-GIA-KHONG-PHAI-THAT"

func TestMatKhauGieoQuanTriTrongLaTat(t *testing.T) {
	// Unset must not stop the service: the variable is optional (rule 11, invariant 8), and empty
	// is how the feature is switched off.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                 dsnGia,
		"ENV":                          EnvProd,
		"SESSION_SIGNING_KEYS":         khoaGia,
		"IDENTITY_ADMIN_SEED_PASSWORD": "",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi khi biến trống: %v", err)
	}
	if !cfg.IdentityAdminSeedPassword.Rong() {
		t.Error("biến trống mà mật khẩu gieo có giá trị")
	}
}

func TestMatKhauGieoQuanTriDocVaCatKhoangTrang(t *testing.T) {
	// A trailing newline, as a value pasted into a k8s Secret carries it. Left in, the typed
	// password would never match and the refusal would read as an ordinary wrong password.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                 dsnGia,
		"ENV":                          EnvDev,
		"IDENTITY_ADMIN_SEED_PASSWORD": " " + matKhauGieoGia + "\n",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if got := string(cfg.IdentityAdminSeedPassword.Lo()); got != matKhauGieoGia {
		t.Errorf("mật khẩu gieo đọc sai (dài %d)", len(got))
	}
}

func TestMatKhauGieoQuanTriKhongTuHienRaKhiGhiLog(t *testing.T) {
	// The value opens the highest-privileged account of every commune not yet seeded. A config
	// printed at startup, marshalled into a debug endpoint or %+v'd into a log line must not carry it.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                 dsnGia,
		"ENV":                          EnvDev,
		"IDENTITY_ADMIN_SEED_PASSWORD": matKhauGieoGia,
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for ten, ra := range map[string]string{
		"%v":   fmt.Sprintf("%v", cfg),
		"%+v":  fmt.Sprintf("%+v", cfg),
		"%#v":  fmt.Sprintf("%#v", cfg),
		"json": string(js),
	} {
		if strings.Contains(ra, matKhauGieoGia) {
			t.Errorf("mật khẩu gieo lộ qua %s", ten)
		}
	}
}
