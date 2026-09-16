package config

// GRPC_LISTEN_ADDR — the inter-service port.
//
// A separate file rather than an addition to config_test.go so the two do not collide while
// both are being worked on. The helper and the fake DSN come from config_test.go, same
// package.

import "testing"

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
