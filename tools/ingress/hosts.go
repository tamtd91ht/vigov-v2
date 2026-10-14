package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// hostsFilePath is the ONE place public hostnames are written (owner, 01/10/2026). Every host
// this generator emits — the Ingress, its overlays, and the host constants of the reserved-domain
// checks and the Mini App — is read from it. Before that date the same four values were typed in
// five places (this package, core/config, service-platform's domain, platform-admin, citizen-app)
// and nothing compared them.
const hostsFilePath = "deploy/hosts.yaml"

// hostsFile is the exact shape of deploy/hosts.yaml. Decoded with KnownFields: a misspelt key
// (`api_sufix`) is an error, never a field silently left empty.
type hostsFile struct {
	Environments       []hostsEnvironment `yaml:"environments"`
	MiniAppEnvironment string             `yaml:"mini_app_environment"`
}

type hostsEnvironment struct {
	Name           string `yaml:"name"`
	WebHost        string `yaml:"web_host"`
	APISuffix      string `yaml:"api_suffix"`
	APILabelSuffix string `yaml:"api_label_suffix"`
	WebTLSSecret   string `yaml:"web_tls_secret"`
	APITLSSecret   string `yaml:"api_tls_secret"`
	Retired        bool   `yaml:"retired"`
}

// hostPlan is deploy/hosts.yaml after validation. Environments keeps the file's order: the first
// entry is what deploy/base carries.
//
// Retired holds the environments that no longer run (owner, 14/10/2026: staging). They get no
// overlay, but their roots and suffixes stay in the reserved-domain constants: a host a retired
// environment once served must never become a commune's.
type hostPlan struct {
	Environments []moiTruong
	Retired      []moiTruong
	MiniApp      moiTruong
}

// allEnvironments is live then retired — every host family the reserved-domain rule must cover.
func (p hostPlan) allEnvironments() []moiTruong {
	return append(append([]moiTruong{}, p.Environments...), p.Retired...)
}

// webRoot is the parent of an environment's commune web hosts: "*.vigov.vn" → "vigov.vn".
func (m moiTruong) webRoot() string { return strings.TrimPrefix(m.HostWeb, "*.") }

var (
	// envNameRe — the name is also an overlay directory and a Jenkins MT value.
	envNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	// k8sNameRe — a TLS Secret name (DNS-1123 label; the existing names fit, nothing looser is needed).
	k8sNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
	// apiLabelSuffixRe — appended to a service name inside ONE DNS label, so it starts with the
	// separator and carries nothing a label cannot.
	apiLabelSuffixRe = regexp.MustCompile(`^-[a-z0-9]+$`)
)

// loadHostPlan reads and validates deploy/hosts.yaml under root.
func loadHostPlan(root string) (hostPlan, error) {
	raw, err := os.ReadFile(filepath.Join(root, hostsFilePath))
	if err != nil {
		return hostPlan{}, fmt.Errorf("đọc %s: %w", hostsFilePath, err)
	}
	return parseHostPlan(raw)
}

// parseHostPlan is FAIL CLOSED end to end: an unknown key, a missing or empty field, a duplicate
// environment, a host that is not a plain lower-case DNS name, two environments building the same
// API host, or a mini_app_environment the live list does not hold stops the generator, and nothing is written. Every value here becomes an Ingress
// host, a TLS SAN or a reserved-domain rule; "probably meant" is not a value any of them can take.
func parseHostPlan(raw []byte) (hostPlan, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var f hostsFile
	if err := dec.Decode(&f); err != nil {
		return hostPlan{}, fmt.Errorf("%s: %w", hostsFilePath, err)
	}
	// A second document would be ignored by Decode; ignoring half a file is not reading it.
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return hostPlan{}, fmt.Errorf("%s: chỉ được có MỘT tài liệu YAML", hostsFilePath)
	}
	if len(f.Environments) == 0 {
		return hostPlan{}, fmt.Errorf("%s: `environments` rỗng hoặc thiếu", hostsFilePath)
	}

	var plan hostPlan
	seenName := map[string]bool{}
	// seenSecret: a TLS Secret name → where it was first used. seenAPIHost: api_suffix +
	// api_label_suffix → the environment owning that API host family.
	type secretUse struct{ env, kind, apiSuffix string }
	seenSecret := map[string]secretUse{}
	seenAPIHost := map[string]string{}
	for i, e := range f.Environments {
		where := fmt.Sprintf("%s: environments[%d]", hostsFilePath, i)
		required := []struct{ key, val string }{{"name", e.Name}, {"web_host", e.WebHost}, {"api_suffix", e.APISuffix}}
		if !e.Retired {
			required = append(required, struct{ key, val string }{"web_tls_secret", e.WebTLSSecret},
				struct{ key, val string }{"api_tls_secret", e.APITLSSecret})
		}
		for _, fld := range required {
			if strings.TrimSpace(fld.val) == "" {
				return hostPlan{}, fmt.Errorf("%s: thiếu hoặc rỗng `%s`", where, fld.key)
			}
		}
		if e.Retired {
			// A retired entry serves nothing. A TLS Secret or label suffix on it means somebody thinks
			// it still runs — refuse rather than guess which of the two they meant.
			if e.WebTLSSecret != "" || e.APITLSSecret != "" || e.APILabelSuffix != "" {
				return hostPlan{}, fmt.Errorf("%s: môi trường nghỉ hưu %q không được có web_tls_secret/api_tls_secret/api_label_suffix", where, e.Name)
			}
			if i == 0 {
				return hostPlan{}, fmt.Errorf("%s: môi trường đầu là thứ base mang — không được là môi trường nghỉ hưu", where)
			}
		}
		if !envNameRe.MatchString(e.Name) {
			return hostPlan{}, fmt.Errorf("%s: name %q không phải một nhãn [a-z0-9-]", where, e.Name)
		}
		if seenName[e.Name] {
			return hostPlan{}, fmt.Errorf("%s: môi trường %q khai hai lần", where, e.Name)
		}
		seenName[e.Name] = true
		root, ok := strings.CutPrefix(e.WebHost, "*.")
		if !ok || !isPlainHost(root) {
			return hostPlan{}, fmt.Errorf("%s: web_host %q phải có dạng `*.<tên miền>` (một ký tự đại diện, đúng một nhãn)", where, e.WebHost)
		}
		if !isPlainHost(e.APISuffix) {
			return hostPlan{}, fmt.Errorf("%s: api_suffix %q không phải tên miền thường, viết thường", where, e.APISuffix)
		}
		if e.APILabelSuffix != "" && !apiLabelSuffixRe.MatchString(e.APILabelSuffix) {
			return hostPlan{}, fmt.Errorf("%s: api_label_suffix %q phải rỗng hoặc có dạng `-[a-z0-9]+`", where, e.APILabelSuffix)
		}
		// Two environments on one api_suffix with one label suffix would build the SAME host for
		// every service — one environment's pods answering under the other's name.
		apiKey := e.APILabelSuffix + "." + e.APISuffix
		if prev, dup := seenAPIHost[apiKey]; dup {
			return hostPlan{}, fmt.Errorf("%s: host API `<dịch vụ>%s.%s` trùng với môi trường %s — đặt api_label_suffix khác",
				where, e.APILabelSuffix, e.APISuffix, prev)
		}
		seenAPIHost[apiKey] = e.Name
		for _, u := range []struct{ name, kind string }{{e.WebTLSSecret, "web"}, {e.APITLSSecret, "api"}} {
			if u.name == "" { // retired, checked above
				continue
			}
			if !k8sNameRe.MatchString(u.name) {
				return hostPlan{}, fmt.Errorf("%s: tên Secret TLS %q không hợp lệ", where, u.name)
			}
			// One Secret for two wildcards means one of them is served a certificate that does not
			// cover it — HTTPS breaks on that family and nothing else does. The one exception: two
			// API Secrets for the SAME api_suffix hold the same `*.<api_suffix>` certificate (each
			// namespace keeps its own copy under one name — owner, 14/10/2026).
			prev, dup := seenSecret[u.name]
			if dup && !(prev.kind == "api" && u.kind == "api" && prev.apiSuffix == e.APISuffix) {
				return hostPlan{}, fmt.Errorf("%s: Secret TLS %q đã dùng ở %s", where, u.name, prev.env)
			}
			if !dup {
				seenSecret[u.name] = secretUse{env: e.Name, kind: u.kind, apiSuffix: e.APISuffix}
			}
		}
		m := moiTruong{
			Ten: e.Name, HostWeb: e.WebHost, DuoiAPI: e.APISuffix, NhanAPI: e.APILabelSuffix,
			TLSWeb: e.WebTLSSecret, TLSAPI: e.APITLSSecret,
		}
		if e.Retired {
			plan.Retired = append(plan.Retired, m)
		} else {
			plan.Environments = append(plan.Environments, m)
		}
	}

	if f.MiniAppEnvironment == "" {
		return hostPlan{}, fmt.Errorf("%s: thiếu `mini_app_environment`", hostsFilePath)
	}
	found := false
	for _, m := range plan.Environments {
		if m.Ten == f.MiniAppEnvironment {
			plan.MiniApp, found = m, true
		}
	}
	if !found {
		return hostPlan{}, fmt.Errorf("%s: mini_app_environment %q không có trong `environments` (hoặc đã nghỉ hưu)", hostsFilePath, f.MiniAppEnvironment)
	}
	return plan, nil
}

// isPlainHost: at least two DNS labels, lower-case, no wildcard, no port, no trailing dot. Every
// generated file embeds these values in a string literal, so this is also what makes them safe
// to write there without escaping.
func isPlainHost(h string) bool {
	labels := strings.Split(h, ".")
	if len(labels) < 2 || len(h) > 253 {
		return false
	}
	for _, l := range labels {
		if !k8sNameRe.MatchString(l) {
			return false
		}
	}
	return true
}
