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
