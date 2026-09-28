package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

func TestParseArgsRequiresTicketOnEveryMutatingCommand(t *testing.T) {
	for _, args := range [][]string{
		{"create", "--email", "a@example.test", "--name", "A"},
		{"grant", "--code", "VH-00001", "--permission", "ops.qr.issue"},
		{"revoke", "--code", "VH-00001", "--permission", "ops.qr.issue"},
		{"disable", "--code", "VH-00001", "--reason", "left"},
		{"enable", "--code", "VH-00001", "--reason", "back"},
		{"reset-mfa", "--code", "VH-00001"},
		{"reset-mfa", "--code", "VH-00001", "--ticket", "   "},
	} {
		_, err := parseArgs(args)
		if !errors.Is(err, errUsage) || !strings.Contains(err.Error(), "--ticket is required") {
			t.Errorf("%v: err = %v, want the missing ticket named", args, err)
		}
	}
}

func TestParseArgsValidation(t *testing.T) {
	bad := map[string][]string{
		"no command":         {},
		"unknown command":    {"delete", "--code", "VH-00001", "--ticket", "OPS-1"},
		"unknown flag":       {"list", "--all"},
		"ticket on list":     {"list", "--ticket", "OPS-1"},
		"stray argument":     {"grant", "--code", "VH-00001", "--permission", "ops.qr.issue", "--ticket", "OPS-1", "extra"},
		"missing email":      {"create", "--name", "A", "--ticket", "OPS-1"},
		"missing reason":     {"disable", "--code", "VH-00001", "--ticket", "OPS-1"},
		"missing code":       {"reset-mfa", "--ticket", "OPS-1"},
		"missing permission": {"grant", "--code", "VH-00001", "--ticket", "OPS-1"},
	}
	for name, args := range bad {
		if _, err := parseArgs(args); !errors.Is(err, errUsage) {
			t.Errorf("%s: err = %v, want a usage error", name, err)
		}
	}
	c, err := parseArgs([]string{"create", "--email", "a@example.test", "--name", "Operator A", "--ticket", "OPS-7"})
	if err != nil || c.verb != "create" || c.email != "a@example.test" || c.displayName != "Operator A" || c.ticket != "OPS-7" {
		t.Fatalf("create parsed as %+v, %v", c, err)
	}
	if c, err := parseArgs([]string{"list"}); err != nil || c.verb != "list" {
		t.Fatalf("list: %+v %v", c, err)
	}
}

// A usage error exits 2 WITHOUT reading configuration or opening a database: run returns before
// config.Load, which would otherwise fail on this machine's empty environment with exit 1.
func TestRunRefusesMissingTicketBeforeTouchingAnything(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"grant", "--code", "VH-00001", "--permission", "ops.qr.issue"}, &out, &errOut)
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "--ticket is required") {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

type fakeAdmin struct {
	calls []string
	temp  secret.Secret
}

func (f *fakeAdmin) CreateOperator(_ context.Context, email, name, ticket string) (app.OperatorCreated, error) {
	f.calls = append(f.calls, "create "+email+"|"+name+"|"+ticket)
	return app.OperatorCreated{Code: "VH-00001", TemporaryPassword: f.temp}, nil
}
func (f *fakeAdmin) Grant(_ context.Context, code, perm, ticket string) error {
	f.calls = append(f.calls, "grant "+code+"|"+perm+"|"+ticket)
	return nil
}
func (f *fakeAdmin) Revoke(_ context.Context, code, perm, ticket string) error {
	f.calls = append(f.calls, "revoke "+code+"|"+perm+"|"+ticket)
	return nil
}
func (f *fakeAdmin) Disable(_ context.Context, code, reason, ticket string) error {
	f.calls = append(f.calls, "disable "+code+"|"+reason+"|"+ticket)
	return nil
}
func (f *fakeAdmin) Enable(_ context.Context, code, reason, ticket string) error {
	f.calls = append(f.calls, "enable "+code+"|"+reason+"|"+ticket)
	return nil
}
func (f *fakeAdmin) ResetMFA(_ context.Context, code, ticket string) (secret.Secret, error) {
	f.calls = append(f.calls, "reset-mfa "+code+"|"+ticket)
	return f.temp, nil
}
func (f *fakeAdmin) List(context.Context) ([]app.OperatorListing, error) {
	return []app.OperatorListing{{Code: "VH-00001", MaskedEmail: "o***@example.test", DisplayName: "Operator One",
		Status: app.OperatorStatusActive, Permissions: []domain.OperatorPermission{domain.OperatorPermissionQRIssue}}}, nil
}

func TestExecutePassesArgumentsInPlaceAndPrintsSecretOnce(t *testing.T) {
	const temp = "TEMP-FAKE-FAKE-FAKE"
	f := &fakeAdmin{temp: secret.Secret(temp)}
	ctx := context.Background()
	for _, args := range [][]string{
		{"create", "--email", "a@example.test", "--name", "Operator A", "--ticket", "OPS-1"},
		{"grant", "--code", "VH-00001", "--permission", "ops.qr.issue", "--ticket", "OPS-2"},
		{"revoke", "--code", "VH-00001", "--permission", "ops.qr.issue", "--ticket", "OPS-3"},
		{"disable", "--code", "VH-00001", "--reason", "left", "--ticket", "OPS-4"},
		{"enable", "--code", "VH-00001", "--reason", "back", "--ticket", "OPS-5"},
		{"reset-mfa", "--code", "VH-00001", "--ticket", "OPS-6"},
	} {
		c, err := parseArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := execute(ctx, c, f, &out); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(out.String(), temp); (args[0] == "create" || args[0] == "reset-mfa") != (n == 1) || n > 1 {
			t.Errorf("%s: temporary password printed %d times", args[0], n)
		}
	}
	want := []string{
		"create a@example.test|Operator A|OPS-1",
		"grant VH-00001|ops.qr.issue|OPS-2",
		"revoke VH-00001|ops.qr.issue|OPS-3",
		"disable VH-00001|left|OPS-4",
		"enable VH-00001|back|OPS-5",
		"reset-mfa VH-00001|OPS-6",
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}

	var out bytes.Buffer
	c, _ := parseArgs([]string{"list"})
	if err := execute(ctx, c, f, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "o***@example.test") || !strings.Contains(out.String(), "ops.qr.issue") {
		t.Fatalf("list output: %s", out.String())
	}
}
