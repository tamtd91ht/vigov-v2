package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// The mini-app verbs (ADR 0066 decision row 3): the command line, the secret from stdin, and the
// commune put into context. The acts themselves are tested in internal/app/mini_app_secret_test.go.

const (
	testTenant = "01J9XA0000000000000000000A"
	testApp    = "1234567890123456789"
	fakeSecret = "FAKE-APP-SECRET-0000"
)

type fakeMiniApp struct {
	calls  []string
	secret string
}

func (f *fakeMiniApp) SetSecret(ctx context.Context, appID string, v secret.Secret, ticket string) error {
	f.secret = string(v.Lo())
	f.calls = append(f.calls, "set "+string(tenant.MustFrom(ctx))+"|"+appID+"|"+ticket)
	return nil
}

func (f *fakeMiniApp) SetDemo(ctx context.Context, appID string, on bool, ticket string) error {
	state := "off"
	if on {
		state = "on"
	}
	f.calls = append(f.calls, "demo-"+state+" "+string(tenant.MustFrom(ctx))+"|"+appID+"|"+ticket)
	return nil
}

func (f *fakeMiniApp) Retire(ctx context.Context, appID, ticket string) error {
	f.calls = append(f.calls, "retire "+string(tenant.MustFrom(ctx))+"|"+appID+"|"+ticket)
	return nil
}

func miniArgs(verb, sub string, extra ...string) []string {
	return append([]string{verb, sub, "--tenant", testTenant, "--app-id", testApp}, extra...)
}

func TestMiniAppParseArgs(t *testing.T) {
	bad := map[string][]string{
		"no sub-command":      {"mini-app-secret"},
		"wrong sub":           miniArgs("mini-app-secret", "on", "--ticket", "OPS-1"),
		"wrong demo sub":      miniArgs("mini-app-demo", "set", "--ticket", "OPS-1"),
		"no ticket":           miniArgs("mini-app-secret", "set"),
		"blank ticket":        miniArgs("mini-app-demo", "on", "--ticket", "  "),
		"no tenant":           {"mini-app-demo", "on", "--app-id", testApp, "--ticket", "OPS-1"},
		"tenant not an id":    {"mini-app-demo", "on", "--tenant", "thangbinh-danang.vigov.vn", "--app-id", testApp, "--ticket", "OPS-1"},
		"no app id":           {"mini-app-secret", "retire", "--tenant", testTenant, "--ticket", "OPS-1"},
		"secret on argv":      miniArgs("mini-app-secret", "set", "--ticket", "OPS-1", "--secret", fakeSecret),
		"secret as stray arg": miniArgs("mini-app-secret", "set", "--ticket", "OPS-1", fakeSecret),
	}
	for name, args := range bad {
		if _, err := parseArgs(args); !errors.Is(err, errUsage) {
			t.Errorf("%s: err = %v, want a usage error", name, err)
		}
	}
	c, err := parseArgs(miniArgs("mini-app-demo", "off", "--ticket", "OPS-3"))
	if err != nil || c.verb != "mini-app-demo" || c.sub != "off" || c.tenant != testTenant || c.appID != testApp || c.ticket != "OPS-3" {
		t.Fatalf("parsed %+v, %v", c, err)
	}
}

// A usage error exits 2 before configuration is read — the secret on stdin is never consumed.
func TestMiniAppRunRefusesMissingTicketBeforeTouchingAnything(t *testing.T) {
	var out, errOut bytes.Buffer
	in := strings.NewReader(fakeSecret)
	if code := run(miniArgs("mini-app-secret", "set"), in, &out, &errOut); code != exitUsage {
		t.Fatalf("exit = %d", code)
	}
	if in.Len() != len(fakeSecret) {
		t.Fatal("stdin was read before the command line was validated")
	}
}

func TestMiniAppExecuteReadsSecretFromStdinOnly(t *testing.T) {
	ctx := context.Background()
	f := &fakeMiniApp{}
	for _, tc := range []struct {
		args  []string
		stdin string
	}{
		{miniArgs("mini-app-secret", "set", "--ticket", "OPS-1"), "  " + fakeSecret + "\r\n"},
		{miniArgs("mini-app-demo", "on", "--ticket", "OPS-2"), ""},
		{miniArgs("mini-app-demo", "off", "--ticket", "OPS-3"), ""},
		{miniArgs("mini-app-secret", "retire", "--ticket", "OPS-4"), ""},
	} {
		c, err := parseArgs(tc.args)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := executeMiniApp(ctx, c, f, strings.NewReader(tc.stdin), &out); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if strings.Contains(out.String(), fakeSecret) {
			t.Fatalf("%v: the secret was printed", tc.args)
		}
	}
	if f.secret != fakeSecret {
		t.Fatalf("secret passed = %q, want the trimmed stdin", f.secret)
	}
	want := []string{
		"set " + testTenant + "|" + testApp + "|OPS-1",
		"demo-on " + testTenant + "|" + testApp + "|OPS-2",
		"demo-off " + testTenant + "|" + testApp + "|OPS-3",
		"retire " + testTenant + "|" + testApp + "|OPS-4",
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
}

// The deploy job's pod has no stdin: the secret comes from a mounted file, and stdin is not read.
func TestMiniAppSecretFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(path, []byte(fakeSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := parseArgs(miniArgs("mini-app-secret", "set", "--ticket", "OPS-1", "--secret-file", path))
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeMiniApp{}
	var out bytes.Buffer
	if err := executeMiniApp(context.Background(), c, f, strings.NewReader("NOT-THIS"), &out); err != nil {
		t.Fatal(err)
	}
	if f.secret != fakeSecret || strings.Contains(out.String(), fakeSecret) {
		t.Fatalf("secret = %q, out = %q", f.secret, out.String())
	}

	c.secretFile = filepath.Join(t.TempDir(), "missing")
	f = &fakeMiniApp{}
	if err := executeMiniApp(context.Background(), c, f, strings.NewReader(fakeSecret), &bytes.Buffer{}); err == nil || len(f.calls) != 0 {
		t.Fatalf("missing file: err = %v, calls = %v", err, f.calls)
	}

	// Only `set` takes a secret.
	for _, sub := range []string{"retire"} {
		if _, err := parseArgs(miniArgs("mini-app-secret", sub, "--ticket", "OPS-1", "--secret-file", path)); !errors.Is(err, errUsage) {
			t.Errorf("%s --secret-file: err = %v, want a usage error", sub, err)
		}
	}
	if _, err := parseArgs(miniArgs("mini-app-demo", "on", "--ticket", "OPS-1", "--secret-file", path)); !errors.Is(err, errUsage) {
		t.Errorf("demo on --secret-file: err = %v, want a usage error", err)
	}
}

func TestMiniAppSecretStdinRefusals(t *testing.T) {
	c, err := parseArgs(miniArgs("mini-app-secret", "set", "--ticket", "OPS-1"))
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]string{
		"empty":      "",
		"whitespace": " \n\t",
		"oversized":  strings.Repeat("a", maxSecretInput+1),
	} {
		f := &fakeMiniApp{}
		err := executeMiniApp(context.Background(), c, f, strings.NewReader(in), &bytes.Buffer{})
		if err == nil || len(f.calls) != 0 {
			t.Errorf("%s: err = %v, calls = %v — want refused before the act", name, err, f.calls)
		}
	}
}
