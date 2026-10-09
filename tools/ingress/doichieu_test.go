package main

import (
	"bytes"
	"errors"
	"io"
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
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
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

// pinnedIngressNames — the objects the file must hold, in order. Literal, not read from the
// generator: `vigov` is the name both overlay kustomizations already target.
var pinnedIngressNames = []string{"vigov", "vigov-no-upload"}

// readIngressObjects reads every document of the generated file and checks it holds exactly the
// pinned Ingress objects, in order.
func readIngressObjects(t *testing.T) []ingressTep {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), duongTepSinh))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongTepSinh, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var out []ingressTep
	for {
		var ing ingressTep
		err := dec.Decode(&ing)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("%s không phải YAML hợp lệ: %v", duongTepSinh, err)
		}
		out = append(out, ing)
	}
	if len(out) != len(pinnedIngressNames) {
		t.Fatalf("%s có %d tài liệu, mong %d Ingress %v", duongTepSinh, len(out), len(pinnedIngressNames), pinnedIngressNames)
	}
	for i, ing := range out {
		if ing.Kind != "Ingress" || ing.Metadata.Name != pinnedIngressNames[i] {
			t.Fatalf("%s tài liệu %d: mong Ingress tên %s, có %q/%q", duongTepSinh, i, pinnedIngressNames[i], ing.Kind, ing.Metadata.Name)
		}
	}
	return out
}

// docTepSinh is the HOST-LEVEL view of the generated file: the rules of every Ingress object
// concatenated in file order. `vigov` comes first and its rule 0 is the web host, so Rules[0] is
// the web host and Rules[1:] are the service hosts. Per-object checks (indices the overlays patch,
// annotations) use readIngressObjects instead.
func docTepSinh(t *testing.T) ingressTep {
	t.Helper()
	objs := readIngressObjects(t)
	merged := objs[0]
	for _, o := range objs[1:] {
		merged.Spec.Rules = append(merged.Spec.Rules, o.Spec.Rules...)
	}
	return merged
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
