package app

// The OPERATOR path of mini_app_secret (ADR 0070 §"Bổ sung 02/10/2026" #6/#7): SetSecretAsOperator and
// RetireAsOperator, on the same fakes as mini_app_secret_test.go. What is pinned here: the version row
// and the trail entry name the operator's VH- code with actor_kind operator, in the same transaction;
// the reason is the operator's own; every shape refusal is decided before any lookup; and the
// operatorctl path through the same change() still signs as system with the ticket.

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

const (
	testOperatorCode = "VH-00001"
	testOperatorIP   = "198.51.100.7"
	testMainApp      = "5550000000000000001"
)

var testOperator = MiniAppOperator{Code: testOperatorCode, IP: testOperatorIP}

// auditArgs returns the arguments of every audit_log insert carrying action:
// tenant, actor_id, actor_kind, actor_ip, action, subject, at, delta (core/audit.Write).
func (rig adminRig) auditArgs(action string) [][]driver.Value {
	rig.g.mu.Lock()
	defer rig.g.mu.Unlock()
	var out [][]driver.Value
	for _, l := range rig.g.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") && len(l.args) == 8 && l.args[4] == action {
			out = append(out, l.args)
		}
	}
	return out
}

func deltaOf(args []driver.Value) string {
	switch d := args[7].(type) {
	case []byte:
		return string(d)
	case string:
		return d
	}
	return ""
}

func TestMiniAppSecretOperatorSetIsTheOperatorsAct(t *testing.T) {
	rig := newAdminRig(t)
	set, err := rig.a.SetSecretAsOperator(rig.ctx, testOperator, " "+testOwnApp+" ", secret.Secret(testAppSecret),
		"  Đổi App ID theo yêu cầu của xã\nphiếu 12  ")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := rig.r.live(testOwnApp)
	if !ok || v.SetBy != testOperatorCode || v.Sealed == nil {
		t.Fatalf("live version = %+v, want set_by %s", v, testOperatorCode)
	}
	want := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if set.AppID != testOwnApp || set.Version != v.ID || set.SetBy != testOperatorCode || !set.SetAt.Equal(want) {
		t.Fatalf("result = %+v, want the live version's metadata", set)
	}

	rows := rig.auditArgs(ActionSetOwnAppSecret)
	if len(rows) != 1 {
		t.Fatalf("%d entries, want 1", len(rows))
	}
	a := rows[0]
	if a[0] != string(xaThu) || a[1] != testOperatorCode || a[2] != audit.KindOperator || a[3] != testOperatorIP || a[5] != testOwnApp {
		t.Fatalf("entry = %v — want the target commune, actor %s, kind %s, the operator's IP, subject the App ID",
			a[:6], testOperatorCode, audit.KindOperator)
	}
	d := deltaOf(a)
	if !strings.Contains(d, `"ly_do":"Đổi App ID theo yêu cầu của xã\nphiếu 12"`) {
		t.Errorf("delta = %s, want the operator's reason, trimmed", d)
	}
	if strings.Contains(d, testAppSecret) || strings.Contains(d, "ticket:") {
		t.Errorf("delta = %s — no secret, no ticket", d)
	}
	if rig.g.soGiaoDich() != 1 || rig.g.ketThucCua(1) != "commit" {
		t.Fatalf("transactions = %d, end %q — want the version and the entry in ONE committed tx",
			rig.g.soGiaoDich(), rig.g.ketThucCua(1))
	}
}

func TestMiniAppSecretOperatorReplaceRetiresTheOldAsTheOperator(t *testing.T) {
	rig := newAdminRig(t)
	if err := rig.a.SetSecret(rig.ctx, testOwnApp, secret.Secret(testAppSecret), "OPS-1"); err != nil {
		t.Fatal(err)
	}
	first, _ := rig.r.live(testOwnApp)
	if _, err := rig.a.SetSecretAsOperator(rig.ctx, testOperator, testOwnApp, secret.Secret("FAKE-APP-SECRET-0001"), "thay khoá"); err != nil {
		t.Fatal(err)
	}
	if rig.r.by[first.ID] != testOperatorCode || rig.r.dead[first.ID] != "thay khoá" {
		t.Fatalf("old version retired by %q for %q, want %s with the operator's reason",
			rig.r.by[first.ID], rig.r.dead[first.ID], testOperatorCode)
	}
}

func TestMiniAppSecretOperatorRetire(t *testing.T) {
	rig := newAdminRig(t)
	if _, err := rig.a.RetireAsOperator(rig.ctx, testOperator, testOwnApp, "gỡ App ID"); !errors.Is(err, ErrMiniAppSettingsNotFound) {
		t.Fatalf("retire with nothing live: %v, want ErrMiniAppSettingsNotFound", err)
	}
	if len(rig.auditArgs(ActionRetireOwnAppConfig)) != 0 {
		t.Fatal("a refused retire wrote an entry")
	}
	if _, err := rig.a.SetSecretAsOperator(rig.ctx, testOperator, testOwnApp, secret.Secret(testAppSecret), "đặt khoá"); err != nil {
		t.Fatal(err)
	}
	live, _ := rig.r.live(testOwnApp)

	// Retire needs NO binding: the App ID was moved off this commune since.
	rig.a.binding = bindingFake{apps: map[string]platformclient.MiniApp{}}
	r, err := rig.a.RetireAsOperator(rig.ctx, testOperator, testOwnApp, "gỡ App ID")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rig.r.live(testOwnApp); ok {
		t.Fatal("still live after retire")
	}
	if r.AppID != testOwnApp || r.Version != live.ID || r.RetiredBy != testOperatorCode || r.RetiredAt.IsZero() {
		t.Fatalf("result = %+v, want the retired version %s by %s", r, live.ID, testOperatorCode)
	}
	if rig.r.by[live.ID] != testOperatorCode {
		t.Fatalf("deleted_by = %q", rig.r.by[live.ID])
	}
	rows := rig.auditArgs(ActionRetireOwnAppConfig)
	if len(rows) != 1 || rows[0][1] != testOperatorCode || rows[0][2] != audit.KindOperator {
		t.Fatalf("entries = %v", rows)
	}
	// Another commune's settings are out of reach by construction: every statement binds the commune
	// of ctx as $1 (store/mini_app_secret.go), pinned by store/mini_app_secret_pg_test.go.
}

func TestMiniAppSecretOperatorRefusals(t *testing.T) {
	rig := newAdminRig(t)
	rig.a.binding = bindingFake{apps: map[string]platformclient.MiniApp{
		testOwnApp:  {CheDo: platformclient.CheDoAppRieng, XaRieng: xaThu},
		testMainApp: {CheDo: platformclient.CheDoAppChinh},
	}}
	ok := secret.Secret(testAppSecret)
	ctxB := tenant.Into(context.Background(), xaKhac)
	cases := map[string]struct {
		err  error
		want error
	}{}
	run := func(name string, want error, f func() error) { cases[name] = struct{ err, want error }{f(), want} }
	set := func(ctx context.Context, op MiniAppOperator, id string, v secret.Secret, reason string) func() error {
		return func() error { _, err := rig.a.SetSecretAsOperator(ctx, op, id, v, reason); return err }
	}
	ret := func(op MiniAppOperator, id, reason string) func() error {
		return func() error { _, err := rig.a.RetireAsOperator(rig.ctx, op, id, reason); return err }
	}
	run("no operator code", ErrMiniAppNoOperatorCode, set(rig.ctx, MiniAppOperator{IP: testOperatorIP}, testOwnApp, ok, "x"))
	run("retire no operator code", ErrMiniAppNoOperatorCode, ret(MiniAppOperator{Code: " "}, testOwnApp, "x"))
	run("app id letters", ErrMiniAppIDInvalid, set(rig.ctx, testOperator, "abc", ok, "x"))
	run("app id 33 digits", ErrMiniAppIDInvalid, set(rig.ctx, testOperator, strings.Repeat("1", 33), ok, "x"))
	run("app id empty", ErrMiniAppIDInvalid, ret(testOperator, "", "x"))
	run("secret empty", ErrMiniAppSecretInvalid, set(rig.ctx, testOperator, testOwnApp, nil, "x"))
	run("secret space", ErrMiniAppSecretInvalid, set(rig.ctx, testOperator, testOwnApp, secret.Secret("a b"), "x"))
	run("secret 257", ErrMiniAppSecretInvalid, set(rig.ctx, testOperator, testOwnApp, secret.Secret(strings.Repeat("a", 257)), "x"))
	run("secret control", ErrMiniAppSecretInvalid, set(rig.ctx, testOperator, testOwnApp, secret.Secret("ab\x01"), "x"))
	run("reason empty", ErrMiniAppReasonInvalid, set(rig.ctx, testOperator, testOwnApp, ok, " \n "))
	run("reason 501", ErrMiniAppReasonInvalid, set(rig.ctx, testOperator, testOwnApp, ok, strings.Repeat("ạ", 501)))
	run("reason NUL", ErrMiniAppReasonInvalid, ret(testOperator, testOwnApp, "a\x00b"))
	run("reason not utf8", ErrMiniAppReasonInvalid, ret(testOperator, testOwnApp, "a\xffb"))
	run("other commune", ErrMiniAppNotOwnApp, set(ctxB, testOperator, testOwnApp, ok, "x"))
	run("main app", ErrMiniAppNotOwnApp, set(rig.ctx, testOperator, testMainApp, ok, "x"))
	run("unregistered", ErrMiniAppNotOwnApp, set(rig.ctx, testOperator, "777", ok, "x"))
	for name, c := range cases {
		if !errors.Is(c.err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, c.err, c.want)
		}
	}
	if len(rig.r.rows) != 0 || rig.g.soGiaoDich() != 0 {
		t.Fatalf("a refused act wrote: rows %d, transactions %d", len(rig.r.rows), rig.g.soGiaoDich())
	}
	// 500 runes exactly is accepted.
	if _, err := rig.a.SetSecretAsOperator(rig.ctx, testOperator, testOwnApp, ok, strings.Repeat("ạ", 500)); err != nil {
		t.Fatalf("500-rune reason refused: %v", err)
	}
}

// The operatorctl path is unchanged by the operator variant: same change(), system principal, ticket.
func TestMiniAppSecretOperatorctlPathStillSystemAndTicket(t *testing.T) {
	rig := newAdminRig(t)
	if err := rig.a.SetSecret(rig.ctx, testOwnApp, secret.Secret(testAppSecret), "OPS-7"); err != nil {
		t.Fatal(err)
	}
	v, _ := rig.r.live(testOwnApp)
	if v.SetBy != domain.SystemActor {
		t.Fatalf("set_by = %q, want %q", v.SetBy, domain.SystemActor)
	}
	if err := rig.a.Retire(rig.ctx, testOwnApp, "OPS-8"); err != nil {
		t.Fatal(err)
	}
	if rig.r.by[v.ID] != domain.SystemActor || rig.r.dead[v.ID] != "ticket:OPS-8" {
		t.Fatalf("retired by %q for %q, want system / ticket:OPS-8", rig.r.by[v.ID], rig.r.dead[v.ID])
	}
	for _, action := range []string{ActionSetOwnAppSecret, ActionRetireOwnAppConfig} {
		rows := rig.auditArgs(action)
		if len(rows) != 1 || rows[0][1] != domain.SystemActor || rows[0][2] != "system" || rows[0][3] != "" {
			t.Fatalf("%s entries = %v, want one system entry with no IP", action, rows)
		}
	}
}
