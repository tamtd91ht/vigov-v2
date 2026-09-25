package config

// LISTEN_ADDR — the REST port, one default for every service (owner decision 2026-09-25).

import "testing"

func TestListenAddrMacDinh8080ChoMoiDichVu(t *testing.T) {
	// EVERY service, not one. Until 2026-09-25 five of them passed their own default through a
	// per-service helper (:8083, :8084, :8086, :8087, :8088), and a pod whose manifest lacked
	// LISTEN_ADDR listened where neither the httpGet probe nor the NetworkPolicy (both 8080)
	// looked. The service name is in the loop so that a future per-service branch in Load turns
	// this red instead of reintroducing the split silently.
	for _, dv := range []string{"identity", "platform", "documents", "petitions", "finance", "comms", "reporting"} {
		t.Run(dv, func(t *testing.T) {
			datMoiTruong(t, map[string]string{
				"DATABASE_DSN": dsnGia,
				"ENV":          EnvDev,
				"LISTEN_ADDR":  "",
			})

			cfg, err := Load(dv)
			if err != nil {
				t.Fatalf("Load lỗi: %v", err)
			}
			if cfg.ListenAddr != ":8080" {
				t.Errorf("ListenAddr = %q, muốn :8080", cfg.ListenAddr)
			}
		})
	}
}

func TestListenAddrChiKhoangTrangVanLa8080(t *testing.T) {
	// A value of only whitespace is "unset", not an address: passing "  " to net.Listen fails at
	// startup with an error that names the port rather than the manifest line.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN": dsnGia,
		"ENV":          EnvDev,
		"LISTEN_ADDR":  "  \n",
	})

	cfg, err := Load("comms")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, muốn :8080", cfg.ListenAddr)
	}
}

func TestListenAddrDocTuMoiTruongCoCatKhoangTrang(t *testing.T) {
	// Running several services on one machine is done by setting LISTEN_ADDR per process; the
	// value must arrive intact apart from a trailing newline pasted out of a ConfigMap.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN": dsnGia,
		"ENV":          EnvDev,
		"LISTEN_ADDR":  " :8087\n",
	})

	cfg, err := Load("comms")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.ListenAddr != ":8087" {
		t.Errorf("ListenAddr = %q, muốn :8087", cfg.ListenAddr)
	}
}
