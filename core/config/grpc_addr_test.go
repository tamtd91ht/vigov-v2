package config

// GRPC_LISTEN_ADDR — the inter-service port.
//
// A separate file rather than an addition to config_test.go so the two do not collide while
// both are being worked on. The helper and the fake DSN come from config_test.go, same
// package.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestGRPCListenAddrMacDinh(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":     dsnGia,
		"ENV":              EnvDev,
		"GRPC_LISTEN_ADDR": "",
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.GRPCListenAddr != ":9090" {
		t.Errorf("GRPCListenAddr = %q, muốn :9090", cfg.GRPCListenAddr)
	}
	// The whole point of a second variable: the two surfaces must not end up on one listener.
	// gRPC needs HTTP/2 and the REST surface is served over HTTP/1.1.
	if cfg.GRPCListenAddr == cfg.ListenAddr {
		t.Errorf("gRPC và HTTP mặc định trùng cổng %q", cfg.ListenAddr)
	}
}

func TestGRPCListenAddrDocTuMoiTruong(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":     dsnGia,
		"ENV":              EnvDev,
		"GRPC_LISTEN_ADDR": "127.0.0.1:19090",
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.GRPCListenAddr != "127.0.0.1:19090" {
		t.Errorf("GRPCListenAddr = %q", cfg.GRPCListenAddr)
	}
}

func TestPlatformGRPCAddrKhongCoMacDinh(t *testing.T) {
	// NO DEFAULT, ON PURPOSE. This is the address of the registry that answers "which commune",
	// so every service edge resolves its commune through it. A guessed default would either
	// refuse every request or resolve communes against something that is not the registry —
	// rule 1, forbidden #1, a default on the isolation path.
	//
	// Empty is allowed HERE because the platform service holds the registry and calls nobody.
	// Refusing to start belongs to the services that need it: platformclient.Dial("") fails by
	// name so the operator reads which variable is missing, not "404 for every domain".
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":       dsnGia,
		"ENV":                EnvDev,
		"PLATFORM_GRPC_ADDR": "",
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.PlatformGRPCAddr != "" {
		t.Errorf("PlatformGRPCAddr = %q, phải để trống chứ không đoán", cfg.PlatformGRPCAddr)
	}
}

func TestPlatformGRPCAddrDocTuMoiTruong(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":       dsnGia,
		"ENV":                EnvDev,
		"PLATFORM_GRPC_ADDR": "  platform.noi-bo:9090  ",
	})

	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	// Trimmed: a trailing space pasted from a deployment manifest turns into a dial error that
	// reads like the platform is down.
	if cfg.PlatformGRPCAddr != "platform.noi-bo:9090" {
		t.Errorf("PlatformGRPCAddr = %q", cfg.PlatformGRPCAddr)
	}
}

func TestKhoaGoiNoiBoDocTuMoiTruong(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":    dsnGia,
		"ENV":             EnvDev,
		"GRPC_CALLER_KEY": "  " + khoaGoiNoiBoGia + "\n",
	})

	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	// Trimmed. A trailing newline pasted out of a k8s secret compares unequal at the far end,
	// and the refusal it produces reads "unauthenticated" — which sends the reader hunting for
	// a missing variable instead of an invisible character.
	if string(cfg.GRPCCallerKey) != khoaGoiNoiBoGia {
		t.Errorf("GRPCCallerKey đọc sai: %q", string(cfg.GRPCCallerKey))
	}
}

func TestKhoaGoiNoiBoThieuThiChanKhoiDongOMoiMoiTruong(t *testing.T) {
	// NO DEV EXEMPTION, unlike SESSION_SIGNING_KEYS (ADR 0025, invariant 3). A signing key that
	// is missing in dev means this process cannot read a session — annoying, local, visible. A
	// CALLER KEY that is missing means a gRPC port that accepts anything reaching it, and
	// "works fine locally" is exactly how that configuration reaches a cluster.
	//
	// The message must name the variable. "Unauthenticated on every call" sends the reader to
	// the network; "thiếu GRPC_CALLER_KEY" sends them to the deployment manifest.
	for _, env := range []string{EnvDev, EnvStaging, EnvProd} {
		t.Run(env, func(t *testing.T) {
			datMoiTruong(t, map[string]string{
				"DATABASE_DSN":         dsnGia,
				"ENV":                  env,
				"SESSION_SIGNING_KEYS": khoaGia,
			})
			t.Setenv("GRPC_CALLER_KEY", "")

			_, err := Load("comms")
			if !errors.Is(err, ErrThieuBienMoiTruong) {
				t.Fatalf("muốn ErrThieuBienMoiTruong, nhận %v", err)
			}
			if !strings.Contains(err.Error(), "GRPC_CALLER_KEY") {
				t.Errorf("thông báo phải nói rõ thiếu biến nào: %v", err)
			}
		})
	}
}

func TestKhoaGoiNoiBoKhongTuHienRaKhiGhiLog(t *testing.T) {
	// One `slog.Info("boot", "cfg", cfg)` written while debugging would put the key that guards
	// the whole inter-service surface into centralised logging, backups and a third-party
	// monitoring vendor at once — from where it cannot be recalled (rule 8, invariant 1). And
	// there is exactly ONE key, so a leak is a leak for every service on the deployment.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaGia,
		"GRPC_CALLER_KEY":      khoaGoiNoiBoGia,
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatal(err)
	}

	tho, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{
		fmt.Sprintf("%v", cfg.GRPCCallerKey),
		fmt.Sprintf("%s", cfg.GRPCCallerKey),
		fmt.Sprintf("%d", cfg.GRPCCallerKey),
		fmt.Sprintf("%#v", cfg.GRPCCallerKey),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%v", cfg.Redacted()),
		string(tho),
	} {
		if strings.Contains(r, khoaGoiNoiBoGia) {
			t.Errorf("khoá gọi nội bộ lọt ra khi in: %q", r)
		}
	}

	// ...but it must still be reachable for the interceptor.
	if string(cfg.GRPCCallerKey.Lo()) != khoaGoiNoiBoGia {
		t.Error("Lo() không trả về khoá thật — interceptor sẽ từ chối mọi lời gọi")
	}
}

func TestIdentityGRPCAddrKhongCoMacDinh(t *testing.T) {
	// NO DEFAULT, for the same reason as PLATFORM_GRPC_ADDR above. This is the address of the
	// service that answers "who is holding this session", so every guarded staff route in four
	// services depends on it. A guessed default does not fail closed in a readable way: every
	// staff request answers 503 while identity is perfectly healthy.
	//
	// Empty is allowed HERE because identity builds its own principal and platform calls nobody.
	// Refusing to start belongs to the services that need it: identityclient.Dial("") fails by
	// name, so the operator reads which variable is missing.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":       dsnGia,
		"ENV":                EnvDev,
		"IDENTITY_GRPC_ADDR": "",
	})

	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.IdentityGRPCAddr != "" {
		t.Errorf("IdentityGRPCAddr = %q, phải để trống chứ không đoán", cfg.IdentityGRPCAddr)
	}
}

func TestIdentityGRPCAddrDocTuMoiTruong(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":       dsnGia,
		"ENV":                EnvDev,
		"IDENTITY_GRPC_ADDR": "  identity.noi-bo:9090\n",
	})

	cfg, err := Load("documents")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	// Trimmed: a trailing newline pasted from a deployment manifest turns into a dial error that
	// reads like identity is down.
	if cfg.IdentityGRPCAddr != "identity.noi-bo:9090" {
		t.Errorf("IdentityGRPCAddr = %q", cfg.IdentityGRPCAddr)
	}
}

func TestHaiDiaChiGRPCKhongLanNhau(t *testing.T) {
	// Two addresses, two variables, and a reader has to be able to tell which is which. They were
	// briefly one field's worth of typing apart in the struct literal, and a swap there would point
	// every session resolution at the registry and every Host resolution at identity — both fail,
	// neither says why.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":       dsnGia,
		"ENV":                EnvDev,
		"PLATFORM_GRPC_ADDR": "platform.noi-bo:9090",
		"IDENTITY_GRPC_ADDR": "identity.noi-bo:9090",
	})

	cfg, err := Load("petitions")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.PlatformGRPCAddr != "platform.noi-bo:9090" {
		t.Errorf("PlatformGRPCAddr = %q", cfg.PlatformGRPCAddr)
	}
	if cfg.IdentityGRPCAddr != "identity.noi-bo:9090" {
		t.Errorf("IdentityGRPCAddr = %q", cfg.IdentityGRPCAddr)
	}
}
