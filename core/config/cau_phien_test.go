package config

// The citizen-session bridge variables (ADR 0045 §Cấu hình) and the citizen session TTL (owner's
// answer to CÒN MỞ #4).

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// khoaCauGia / khoaCauGia2 are fake bridge keys; the text says so (rule 8, forbidden #1).
const (
	khoaCauGia  = "khoa-cau-phien-GIA-KHONG-PHAI-KHOA-THAT-1"
	khoaCauGia2 = "khoa-cau-phien-GIA-KHONG-PHAI-KHOA-THAT-2"
)

func TestCauPhienMacDinhLaTat(t *testing.T) {
	// Neither variable: the bridge is simply not started, and identity still serves staff. This
	// is every service except identity, and identity on any machine that is not doing Mini App work.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                       dsnGia,
		"ENV":                                EnvDev,
		"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": "",
		"CITIZEN_SESSION_BRIDGE_KEYS":        "",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.CauPhienBat() {
		t.Error("cầu phiên bật khi không khai biến nào")
	}
}

func TestCauPhienDocDuHaiNua(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                       dsnGia,
		"ENV":                                EnvDev,
		"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": " :9091\n",
		// A trailing comma and whitespace, as a hand-edited Secret produces them.
		"CITIZEN_SESSION_BRIDGE_KEYS": " " + khoaCauGia + " , " + khoaCauGia2 + ",",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if !cfg.CauPhienBat() {
		t.Fatal("cầu phiên không bật khi đã khai đủ hai nửa")
	}
	if cfg.CitizenSessionBridgeListenAddr != ":9091" {
		t.Errorf("địa chỉ cầu = %q", cfg.CitizenSessionBridgeListenAddr)
	}
	if len(cfg.CitizenSessionBridgeKeys) != 2 ||
		string(cfg.CitizenSessionBridgeKeys[0].Lo()) != khoaCauGia ||
		string(cfg.CitizenSessionBridgeKeys[1].Lo()) != khoaCauGia2 {
		t.Errorf("đọc sai danh sách khoá cầu: %d khoá", len(cfg.CitizenSessionBridgeKeys))
	}
}

func TestCauPhienNuaVoiThiTuChoiKhoiDong(t *testing.T) {
	for ten, cap := range map[string]map[string]string{
		"co dia chi, thieu khoa": {"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": ":9091", "CITIZEN_SESSION_BRIDGE_KEYS": ""},
		"co khoa, thieu dia chi": {"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": "", "CITIZEN_SESSION_BRIDGE_KEYS": khoaCauGia},
		"khoa chi toan dau phay": {"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": ":9091", "CITIZEN_SESSION_BRIDGE_KEYS": " , ,"},
	} {
		t.Run(ten, func(t *testing.T) {
			cap["DATABASE_DSN"] = dsnGia
			cap["ENV"] = EnvDev
			datMoiTruong(t, cap)
			_, err := Load("identity")
			if !errors.Is(err, ErrCauPhienNuaVoi) {
				t.Fatalf("lỗi = %v, muốn ErrCauPhienNuaVoi", err)
			}
		})
	}
}

func TestLoiCauPhienKhongMangGiaTriKhoa(t *testing.T) {
	// The refusal names the variable, never the value: the value is a credential (rule 8).
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                       dsnGia,
		"ENV":                                EnvDev,
		"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": "",
		"CITIZEN_SESSION_BRIDGE_KEYS":        khoaCauGia,
	})
	_, err := Load("identity")
	if err == nil {
		t.Fatal("Load nhận khoá cầu không kèm địa chỉ")
	}
	if strings.Contains(err.Error(), khoaCauGia) {
		t.Fatal("thông điệp lỗi chứa giá trị khoá cầu")
	}
}

func TestKhoaCauKhongTuHienRaKhiGhiLog(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                       dsnGia,
		"ENV":                                EnvDev,
		"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": ":9091",
		"CITIZEN_SESSION_BRIDGE_KEYS":        khoaCauGia,
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	for _, s := range []string{fmt.Sprint(cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg)} {
		if strings.Contains(s, khoaCauGia) {
			t.Fatal("in cấu hình ra chuỗi làm lộ khoá cầu")
		}
	}
}

func TestThoiHanPhienCongDanMacDinhBaMuoiNgay(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":        dsnGia,
		"ENV":                 EnvDev,
		"CITIZEN_SESSION_TTL": "",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.CitizenSessionTTL != 30*24*time.Hour {
		t.Errorf("TTL mặc định = %v, muốn 720h (quyết định của chủ dự án 25/09/2026)", cfg.CitizenSessionTTL)
	}
}

func TestThoiHanPhienCongDanDocTuMoiTruong(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":        dsnGia,
		"ENV":                 EnvDev,
		"CITIZEN_SESSION_TTL": " 168h\n",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.CitizenSessionTTL != 168*time.Hour {
		t.Errorf("TTL = %v, muốn 168h", cfg.CitizenSessionTTL)
	}
}

func TestThoiHanPhienCongDanSaiThiTuChoiKhongLuiVeMacDinh(t *testing.T) {
	// "30d" is the typo that matters: not Go syntax, and a silent fallback would hide it.
	for _, v := range []string{"30d", "abc", "0s", "-1h"} {
		t.Run(v, func(t *testing.T) {
			datMoiTruong(t, map[string]string{
				"DATABASE_DSN":        dsnGia,
				"ENV":                 EnvDev,
				"CITIZEN_SESSION_TTL": v,
			})
			if _, err := Load("identity"); !errors.Is(err, ErrThoiHanPhienHong) {
				t.Fatalf("CITIZEN_SESSION_TTL=%q: lỗi = %v, muốn ErrThoiHanPhienHong", v, err)
			}
		})
	}
}
