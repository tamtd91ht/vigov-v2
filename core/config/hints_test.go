package config

// The operator hints printed with ErrThieuBienMoiTruong (hints.go).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// refusableVars is every variable Load can refuse when empty, derived from the source rather than
// listed: the r.read calls in config.go whose requirement is requiredEverywhere or requiredInProd.
// A list written here would be one more list to forget; the r.read call is where the decision is.
func refusableVars(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "config.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 3 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "read" {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "r" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Errorf("r.read with a non-literal name at %v — the hint check cannot see it", call.Pos())
			return true
		}
		req, ok := call.Args[2].(*ast.Ident)
		if !ok {
			t.Errorf("r.read(%s) with a non-identifier requirement", lit.Value)
			return true
		}
		name, _ := strconv.Unquote(lit.Value)
		if req.Name == "requiredEverywhere" || req.Name == "requiredInProd" {
			seen[name] = true
		}
		return true
	})
	var out []string
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	// Guard against a parse that silently finds nothing (and so passes everything).
	if len(out) < 20 || !seen["TRUSTED_PROXY_CIDRS"] || !seen["DATABASE_DSN"] {
		t.Fatalf("refusableVars found %d variables %v — the r.read parse is broken", len(out), out)
	}
	return out
}

// A variable cannot become required without saying what it is, how to get it and where it goes.
func TestEveryRefusableVariableHasAHint(t *testing.T) {
	refusable := map[string]bool{}
	for _, name := range refusableVars(t) {
		refusable[name] = true
		h, ok := envHints[name]
		if !ok {
			t.Errorf("%s can stop a pod but has no entry in envHints (hints.go) — the operator would see a bare name", name)
			continue
		}
		if h.meaning == "" || h.source == "" || h.shape == "" {
			t.Errorf("%s: hint has an empty field: %+v", name, h)
		}
		if h.place < inConfigMap || h.place > inDeploymentEnv {
			t.Errorf("%s: hint has no placement", name)
		}
	}
	for name := range envHints {
		if !refusable[name] {
			t.Errorf("envHints has %s, which Load never refuses — a stale entry", name)
		}
	}
}

// Every missing variable is printed with its whole hint, in every refusal Load can produce.
func TestRefusalPrintsEveryMissingHint(t *testing.T) {
	var all []Group
	for g := Group(1); g < groupEnd; g++ {
		all = append(all, g)
	}
	clean(t, EnvProd, map[string]string{"DATABASE_DSN": ""})
	_, prodErr := Load("svc-test", Uses(all...))
	// ENV is only missing when it is empty, which is dev's rule.
	clean(t, "", nil)
	_, devErr := Load("svc-test", Uses())

	for _, name := range refusableVars(t) {
		err := prodErr
		if name == "ENV" {
			err = devErr
		}
		if err == nil {
			t.Fatalf("expected a refusal naming %s", name)
		}
		h := envHints[name]
		for _, want := range []string{"\n\n" + name + "\n", h.meaning, h.source, h.place.describe("svc-test"), h.shape} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not print %q", name, want)
			}
		}
	}
}

// The exact message the platform pod printed on 29/09/2026, now with its explanation. Pinned
// whole: this is the text an operator reads, and a change to it should be a decision, not drift.
func TestTrustedProxyRefusalMessage(t *testing.T) {
	clean(t, EnvProd, map[string]string{"GRPC_CALLER_KEY": khoaGoiNoiBoGia})
	_, err := Load("platform", Uses(HTTPServer, GRPCServer, TenantCache))
	if err == nil {
		t.Fatal("platform in prod without TRUSTED_PROXY_CIDRS must be refused")
	}
	want := "config: thiếu biến môi trường bắt buộc: TRUSTED_PROXY_CIDRS (service platform, ENV=prod)\n" +
		"\n" +
		"TRUSTED_PROXY_CIDRS\n" +
		"  Là gì:        Dải IP Pod (Pod CIDR) của ingress-nginx và web-admin — các trạm chuyển tiếp được tin khi báo IP người dùng qua X-Forwarded-For. " +
		"Thiếu thì vết kiểm toán ghi IP của pod thay vì IP người thao tác.\n" +
		"  Lấy giá trị:  Dải Pod của từng node: kubectl get nodes -o jsonpath='{range .items[*]}{.spec.podCIDR}{\"\\n\"}{end}' ; " +
		"IP pod ingress và web-admin: kubectl get pods -A -o wide | grep -E 'ingress|web-admin' (RKE2 chạy ingress-nginx trong kube-system). " +
		"RKE2/k3s thường là 10.42.0.0/16. Nếu ingress-nginx chạy hostNetwork thì thêm dải IP node (kubectl get nodes -o wide). " +
		"Không bao giờ 0.0.0.0/0 hay ::/0 — tin mọi địa chỉ là tin X-Forwarded-For giả của bất kỳ ai, pod từ chối khởi động.\n" +
		"  Đặt ở:        ConfigMap `common-config` — dùng chung cho mọi pod Go\n" +
		"  Dạng:         10.42.0.0/16 — nhiều dải cách nhau dấu phẩy\n" +
		"\n" +
		"Đặt xong: kubectl -n <namespace> rollout restart deploy/vigov-service-platform. Bảng đầy đủ: deploy/cau-hinh/README.md."
	if got := err.Error(); got != want {
		t.Errorf("TRUSTED_PROXY_CIDRS refusal changed.\n got: %s\nwant: %s", got, want)
	}
}

// The refusal names variables, never values: a present credential must not ride along with the
// explanation of a missing one (rule 8).
func TestRefusalPrintsNoValue(t *testing.T) {
	present := map[string]string{
		"GRPC_CALLER_KEY":             khoaGoiNoiBoGia,
		"REDIS_DSN":                   redisGia,
		"CITIZEN_SESSION_BRIDGE_KEYS": khoaCauGia,
		"TRUSTED_PROXY_CIDRS":         "10.77.0.0/16",
	}
	clean(t, EnvProd, present)
	_, err := Load("identity", Uses(HTTPServer, GRPCServer, Redis, CitizenBridge, StaffSessionSigning))
	if err == nil {
		t.Fatal("expected a refusal: SESSION_SIGNING_KEYS and the bridge address are missing")
	}
	msg := err.Error()
	for name, v := range present {
		if strings.Contains(msg, v) {
			t.Errorf("the refusal prints the value of %s", name)
		}
	}
	if strings.Contains(msg, dsnGia) || strings.Contains(msg, "khong-phai-mat-khau-that") {
		t.Error("the refusal prints the DATABASE_DSN value")
	}
}

// The hint and the operator's README must agree on WHICH OBJECT holds each variable: two answers
// to "where does it go" is how a pod stops twice. Section 4 (Không đặt) is skipped: those variables
// are not set by anybody yet, so the README has no object to disagree with.
func TestHintPlacementMatchesREADME(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "cau-hinh", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("^\\|\\s*`([A-Z0-9_]+)`\\s*\\|")
	section := placement(0)
	found := map[string]placement{}
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "## 1."):
			section = inSecret
		case strings.HasPrefix(line, "## 2."):
			section = inConfigMap
		case strings.HasPrefix(line, "## 3."):
			section = inDeploymentEnv
		case strings.HasPrefix(line, "## "):
			section = 0
		}
		if m := row.FindStringSubmatch(line); m != nil && section != 0 {
			found[m[1]] = section
		}
	}
	if len(found) < 20 {
		t.Fatalf("parsed only %d README rows — the README layout changed, fix this test", len(found))
	}
	for name, h := range envHints {
		p, ok := found[name]
		if !ok {
			continue
		}
		if p != h.place {
			t.Errorf("%s: hints.go places it in %q, README section says %q", name, h.place.describe("<dịch vụ>"), p.describe("<dịch vụ>"))
		}
	}
}

// The ConfigMap and Secret names printed by describe() must be the ones the manifests really read
// through envFrom — a hint naming an object no Deployment reads is a pod that stops twice.
func TestObjectNamesMatchManifests(t *testing.T) {
	for _, svc := range []string{"platform", "identity", "documents", "finance", "petitions", "comms", "reporting"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "base", svc, "deployment.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		s := string(raw)
		checks := map[string]*regexp.Regexp{
			"configMapRef " + configMapName: regexp.MustCompile(`configMapRef:\s*\{\s*name:\s*` + regexp.QuoteMeta(configMapName) + `\s*\}`),
			"secretRef " + secretName(svc):  regexp.MustCompile(`secretRef:\s*\{\s*name:\s*` + regexp.QuoteMeta(secretName(svc)) + `\s*\}`),
		}
		for what, re := range checks {
			if !re.MatchString(s) {
				t.Errorf("deploy/base/%s/deployment.yaml does not read %s — hints.go would send the operator to the wrong object", svc, what)
			}
		}
	}
}
