package main

import (
	"testing"

	"github.com/vihat/vigov/core/config"
)

// TestConfigUsesMatchReads keeps configUses equal to what this package reads from config.Config.
//
// An accessor read without its group declared panics at startup in every environment; a group
// declared and never read makes staging and prod refuse to start without variables this service
// does not use. Both are red here first (core/config.CheckUses).
func TestConfigUsesMatchReads(t *testing.T) {
	problems, err := config.CheckUses(".", configUses)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// TestDeclaresCommsClient: the mention notice (ADR 0081 #5) needs comms, so staging/prod must refuse
// to start without COMMS_GRPC_ADDR (rule 11, invariant 8). Undeclared, an unset address in prod would
// mean every mention silently notifies nobody.
func TestDeclaresCommsClient(t *testing.T) {
	for _, g := range configUses.Groups() {
		if g == config.CommsClient {
			return
		}
	}
	t.Fatal("configUses does not declare config.CommsClient")
}
