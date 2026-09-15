package audit

import (
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/tenant"
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
