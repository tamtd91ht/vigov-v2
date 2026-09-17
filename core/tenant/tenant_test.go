package tenant

import (
	"context"
	"testing"
)

func TestMustFromPanicsWithoutTenant(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustFrom must panic on a context with no commune — " +
				"silently returning would let a query cross commune boundaries")
		}
	}()
	MustFrom(context.Background())
}

func TestRoundTrip(t *testing.T) {
	id := ID("01J0000000000000000000000X")
	got := MustFrom(Into(context.Background(), id))
	if got != id {
		t.Fatalf("got %q, want %q", got, id)
	}
}

func TestInvalidIDIsNotAccepted(t *testing.T) {
	if _, ok := From(Into(context.Background(), ID("short"))); ok {
		t.Fatal("a malformed tenant id must not be accepted")
	}
}
