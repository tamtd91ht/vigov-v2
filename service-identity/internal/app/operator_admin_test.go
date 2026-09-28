package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

func TestOperatorAdminRefusesWithoutTicketBeforeAnyTransaction(t *testing.T) {
	h := newOpHarness(t)
	code, _ := h.create(t)
	ctx := context.Background()
	before := len(h.store.txs)
	for _, ticket := range []string{"", "   ", "OPS 1", "OPS-\x01", strings.Repeat("x", 65)} {
		errs := map[string]error{}
		_, errs["create"] = h.admin.CreateOperator(ctx, "second@example.test", "Second", ticket)
		errs["grant"] = h.admin.Grant(ctx, code, "ops.qr.issue", ticket)
		errs["revoke"] = h.admin.Revoke(ctx, code, "ops.qr.issue", ticket)
		errs["disable"] = h.admin.Disable(ctx, code, "reason", ticket)
		errs["enable"] = h.admin.Enable(ctx, code, "reason", ticket)
		_, errs["reset-mfa"] = h.admin.ResetMFA(ctx, code, ticket)
		for name, err := range errs {
			if !errors.Is(err, ErrTicketRequired) && !errors.Is(err, ErrInvalidTicket) {
				t.Errorf("ticket %q, %s: err = %v", ticket, name, err)
			}
		}
	}
	if len(h.store.txs) != before {
		t.Fatal("an act without a valid ticket opened a transaction")
	}
}

func TestOperatorCreateAuditsWithoutPersonalData(t *testing.T) {
	h := newOpHarness(t)
	ctx := context.Background()
	c, err := h.admin.CreateOperator(ctx, "  "+opEmail+" ", " "+opName+" ", "OPS-11")
	if err != nil {
		t.Fatal(err)
	}
	if c.Code != "VH-00001" || len(c.TemporaryPassword) < 16 {
		t.Fatalf("created = %s, %d-char password", c.Code, len(c.TemporaryPassword))
	}
	acc := h.store.account(t, c.Code)
	if acc.Email != opEmail || !acc.MustChangePassword || acc.CreatedBy != domain.SystemActor {
		t.Fatalf("account = %+v", acc)
	}
	if err := password.KiemTra(string(c.TemporaryPassword.Lo()), h.store.state.creds[acc.ID].passwordHash); err != nil {
		t.Fatal("stored hash does not match the returned temporary password")
	}
	tx := h.store.lastTx()
	if !slices.Equal(tx.ops, []string{"CreateAccount", "AppendAudit:operator.account_created"}) || !tx.committed {
		t.Fatalf("create ops: %v", tx.ops)
	}
	e := h.store.state.audit[0]
	if e.Actor != domain.SystemActor || e.Subject != c.Code || e.Reason != "ticket:OPS-11" {
		t.Fatalf("entry = %+v", e)
	}
	if s := fmt.Sprint(e); strings.Contains(s, "example.test") || strings.Contains(s, opName) {
		t.Fatalf("personal data in the audit entry: %s", s)
	}

	if _, err := h.admin.CreateOperator(ctx, strings.ToUpper(opEmail), "Other", "OPS-12"); !errors.Is(err, operatorstore.ErrEmailTaken) {
		t.Fatalf("duplicate email: %v", err)
	}
	for _, bad := range []string{"", "no-at-sign", "a@b", "a b@example.test", "@example.test", "a@@example.test"} {
		if _, err := h.admin.CreateOperator(ctx, bad, "Name", "OPS-13"); !errors.Is(err, ErrInvalidOperatorEmail) {
			t.Errorf("email %q: %v", bad, err)
		}
	}
	if _, err := h.admin.CreateOperator(ctx, "b@example.test", "  ", "OPS-14"); !errors.Is(err, ErrInvalidOperatorName) {
		t.Errorf("blank name: %v", err)
	}
}

func TestOperatorGrantRevokeAuditAndValidation(t *testing.T) {
	h := newOpHarness(t)
	code, _ := h.create(t)
	ctx := context.Background()

	if err := h.admin.Grant(ctx, code, "ops.upload_policy.manage", "OPS-20"); err != nil {
		t.Fatal(err)
	}
	tx := h.store.lastTx()
	if !slices.Equal(tx.ops, []string{"ByCode", "Grant", "AppendAudit:operator.permission_granted"}) {
		t.Fatalf("grant ops: %v", tx.ops)
	}
	e := h.store.state.audit[len(h.store.state.audit)-1]
	if e.Actor != domain.SystemActor || e.Reason != "ticket:OPS-20" || e.After["permission_key"] != "ops.upload_policy.manage" {
		t.Fatalf("grant entry = %+v", e)
	}
	if err := h.admin.Grant(ctx, code, "ops.upload_policy.manage", "OPS-21"); !errors.Is(err, operatorstore.ErrAlreadyGranted) {
		t.Fatalf("double grant: %v", err)
	}
	if err := h.admin.Revoke(ctx, code, "ops.upload_policy.manage", "OPS-22"); err != nil {
		t.Fatal(err)
	}
	e = h.store.state.audit[len(h.store.state.audit)-1]
	if e.Action != domain.OperatorAuditPermissionRevoked || e.Before["permission_key"] != "ops.upload_policy.manage" {
		t.Fatalf("revoke entry = %+v", e)
	}
	if err := h.admin.Revoke(ctx, code, "ops.upload_policy.manage", "OPS-23"); !errors.Is(err, operatorstore.ErrNotGranted) {
		t.Fatalf("revoke of nothing: %v", err)
	}

	before := len(h.store.txs)
	for _, key := range []string{"task.extend", "OPS.QR.ISSUE", "ops.qr.issue ", "ops.everything"} {
		err := h.admin.Grant(ctx, code, key, "OPS-24")
		if key == "ops.qr.issue " { // surrounding blanks are trimmed; the key itself is exact
			if err != nil {
				t.Errorf("trimmed key refused: %v", err)
			}
			continue
		}
		if !errors.Is(err, domain.ErrUnknownOperatorPermission) {
			t.Errorf("key %q: %v", key, err)
		}
	}
	if len(h.store.txs) != before+1 {
		t.Fatal("an off-list key reached the store")
	}
	if err := h.admin.Grant(ctx, "VH-99999", "ops.tenant.manage", "OPS-25"); !errors.Is(err, ErrOperatorNotFound) {
		t.Fatalf("unknown code: %v", err)
	}
	if err := h.admin.Grant(ctx, "01JINTERNALID0000000000000", "ops.tenant.manage", "OPS-26"); !errors.Is(err, ErrOperatorNotFound) {
		t.Fatalf("internal id as code: %v", err)
	}
}

func TestOperatorDisableEnable(t *testing.T) {
	h := newOpHarness(t)
	code, _ := h.create(t)
	ctx := context.Background()
	if err := h.admin.Disable(ctx, code, "", "OPS-30"); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("no reason: %v", err)
	}
	if err := h.admin.Disable(ctx, code, "left the team", "OPS-30"); err != nil {
		t.Fatal(err)
	}
	acc := h.store.account(t, code)
	if !acc.Disabled() || acc.DisabledBy != domain.SystemActor || acc.DisabledReason != "ticket:OPS-30 left the team" {
		t.Fatalf("disabled account = %+v", acc)
	}
	if err := h.admin.Disable(ctx, code, "again", "OPS-31"); !errors.Is(err, ErrOperatorAlreadyDisabled) {
		t.Fatalf("second disable: %v", err)
	}
	if err := h.admin.Enable(ctx, code, "back", "OPS-32"); err != nil {
		t.Fatal(err)
	}
	if err := h.admin.Enable(ctx, code, "back", "OPS-33"); !errors.Is(err, ErrOperatorNotDisabled) {
		t.Fatalf("second enable: %v", err)
	}
	acts := h.store.auditActions()
	if !slices.Contains(acts, domain.OperatorAuditAccountDisabled) || !slices.Contains(acts, domain.OperatorAuditAccountEnabled) {
		t.Fatalf("actions = %v", acts)
	}
}

func TestOperatorListMasksEmail(t *testing.T) {
	h := newOpHarness(t)
	code, _, _, _ := h.enrolled(t)
	ctx := context.Background()
	if err := h.admin.Grant(ctx, code, "ops.qr.issue", "OPS-40"); err != nil {
		t.Fatal(err)
	}
	second, err := h.admin.CreateOperator(ctx, "second@example.test", "Second Operator", "OPS-41")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.admin.Disable(ctx, second.Code, "not yet", "OPS-42"); err != nil {
		t.Fatal(err)
	}
	list, err := h.admin.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("%d rows", len(list))
	}
	if list[0].Code != code || list[0].MaskedEmail != "o***@example.test" || list[0].Status != OperatorStatusActive ||
		!slices.Equal(list[0].Permissions, []domain.OperatorPermission{domain.OperatorPermissionQRIssue}) {
		t.Fatalf("row 0 = %+v", list[0])
	}
	if list[1].Status != OperatorStatusDisabled || list[1].MaskedEmail != "s***@example.test" {
		t.Fatalf("row 1 = %+v", list[1])
	}
	for _, r := range list {
		if strings.Contains(r.MaskedEmail, "operator.one") || strings.Contains(r.MaskedEmail, "second@") {
			t.Fatalf("unmasked email: %+v", r)
		}
	}

	h2 := newOpHarness(t)
	h2.create(t)
	l2, _ := h2.admin.List(ctx)
	if l2[0].Status != OperatorStatusEnrollmentRequired {
		t.Fatalf("fresh account status = %s", l2[0].Status)
	}
}
