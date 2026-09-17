package events

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

func TestDispatchRefusesMessageWithoutTenant(t *testing.T) {
	err := Dispatch(context.Background(), Envelope{Name: "petitions.received.v1"},
		func(context.Context, Envelope) error { return nil })
	if !errors.Is(err, ErrNoTenant) {
		t.Fatal("a message with no commune must be refused, never guessed")
	}
}

func TestDispatchPutsTenantInContext(t *testing.T) {
	want := tenant.ID("01J0000000000000000000000X")
	err := Dispatch(context.Background(),
		Envelope{Name: "petitions.received.v1", TenantID: want},
		func(ctx context.Context, _ Envelope) error {
			if got := tenant.MustFrom(ctx); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}
