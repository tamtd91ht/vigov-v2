package operatorclient

import (
	"strings"
	"testing"
)

// An empty IDENTITY_OPERATOR_GRPC_ADDR is refused by name, never dialled as "" (which gRPC would
// accept and then fail on every call, far from the cause). It must NOT name IDENTITY_GRPC_ADDR: that
// is the staff port, and an operator who sets it from this message points platform at a port that no
// longer serves OperatorService.
func TestDialRefusesEmptyAddress(t *testing.T) {
	_, err := Dial("", callerKeyFake, nil)
	if err == nil || !strings.Contains(err.Error(), "IDENTITY_OPERATOR_GRPC_ADDR") {
		t.Fatalf("Dial(\"\") err = %v, want a refusal naming IDENTITY_OPERATOR_GRPC_ADDR", err)
	}
	if strings.Contains(strings.ReplaceAll(err.Error(), "IDENTITY_OPERATOR_GRPC_ADDR", ""), "IDENTITY_GRPC_ADDR") {
		t.Errorf("the refusal names the staff-port variable: %v", err)
	}
}

// grpc.NewClient is lazy, so a well-formed address builds a client without any server; Close then
// releases it. An injected client (New) closes as a no-op.
func TestDialBuildsAndCloses(t *testing.T) {
	c, err := Dial("identity:9093", callerKeyFake, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := New(nil, nil).Close(); err != nil {
		t.Fatalf("Close on injected client: %v", err)
	}
}
