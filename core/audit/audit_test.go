package audit

import (
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

func TestValidateRejectsUnattributableEntries(t *testing.T) {
	base := Entry{
		TenantID: tenant.ID("01J0000000000000000000000X"),
		Actor:    Actor{ID: "CB-001", Kind: "staff"},
		Action:   "accept_petition",
		Subject:  "PA-2026-0001",
		At:       time.Now(),
	}
	if err := base.validate(); err != nil {
		t.Fatalf("a complete entry must validate: %v", err)
	}

	for name, mutate := range map[string]func(*Entry){
		"no commune": func(e *Entry) { e.TenantID = "" },
		"no actor":   func(e *Entry) { e.Actor.ID = "" },
		"no action":  func(e *Entry) { e.Action = "" },
		"no subject": func(e *Entry) { e.Subject = "" },
	} {
		e := base
		mutate(&e)
		if err := e.validate(); err == nil {
			t.Errorf("%s: must be rejected — an entry nobody can trace is worse than none", name)
		}
	}
}

// ADR 0080 #9: an act by a session without a verified phone is attributed to its Zalo account, with
// its own actor_kind. The entry is accepted with the account id, and refused without one — there is no
// fallback to the session id or to anything else.
func TestValidateZaloAccountActor(t *testing.T) {
	e := Entry{
		TenantID: tenant.ID("01J0000000000000000000000X"),
		Actor:    Actor{ID: "tkz-1", Kind: KindZaloAccount},
		Action:   "gui_phan_anh",
		Subject:  "PA-2026-0001",
	}
	if err := e.validate(); err != nil {
		t.Fatalf("a zalo-account entry with an account id must validate: %v", err)
	}
	e.Actor.ID = ""
	if err := e.validate(); err == nil {
		t.Fatal("a zalo-account entry with no account id must be refused")
	}
}
