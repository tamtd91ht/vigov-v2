package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// This file holds the check that pays for the whole tool: the generated Ingress and the REST
// contract must still agree. It reads the FILE ON DISK — not the model the generator just
// built — because the failure being guarded against is a file that stopped matching: a rule
// deleted by hand, or a route added in Go and never regenerated here.
//
// Delete one service host from deploy/base/mang/ingress.yaml and both
// TestTepSinhRaKhopVoiHopDong and TestMoiTuyenHopDongCoDungMotLuatIngress turn red. The
// prefix-level correspondence (which service owns which path) lives in sinhts_test.go now,
// against the TS table the admin web actually routes by (ADR 0043).

// ingressTep is a deliberately independent reader of the generated file. It shares no code
// with the renderer, so a test passing means two separate pieces of code agree about the
// content — not that one piece agrees with itself.
type ingressTep struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		IngressClassName string `yaml:"ingressClassName"`
		TLS              []any  `yaml:"tls"`
		Rules            []struct {
			Host string `yaml:"host"`
			HTTP struct {
				Paths []struct {
					Path     string `yaml:"path"`
					PathType string `yaml:"pathType"`
					Backend  struct {
						Service struct {
							Name string `yaml:"name"`
							Port struct {
								Name string `yaml:"name"`
							} `yaml:"port"`
						} `yaml:"service"`
					} `yaml:"backend"`
				} `yaml:"paths"`
			} `yaml:"http"`
		} `yaml:"rules"`
	} `yaml:"spec"`
}

func goc(t *testing.T) string {
	t.Helper()
	root, err := timGoc()
	if err != nil {
		t.Fatalf("không tìm được gốc kho: %v", err)
	}
	return root
}

func docTepSinh(t *testing.T) ingressTep {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), duongTepSinh))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongTepSinh, err)
	}
	var ing ingressTep
	if err := yaml.Unmarshal(raw, &ing); err != nil {
		t.Fatalf("%s không phải YAML hợp lệ: %v", duongTepSinh, err)
	}
	if ing.Kind != "Ingress" || ing.Metadata.Name != "vigov" {
		t.Fatalf("%s: mong Ingress tên vigov, có %q/%q", duongTepSinh, ing.Kind, ing.Metadata.Name)
	}
	return ing
}

func docTuyenHopDong(t *testing.T) []tuyenHopDong {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), duongHopDong))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongHopDong, err)
	}
	tuyens, err := docHopDong(raw)
	if err != nil {
		t.Fatalf("hợp đồng REST không phân giải được chủ sở hữu: %v", err)
	}
	return tuyens
}

