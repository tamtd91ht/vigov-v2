package kafka

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

const testCommune = tenant.ID("01J0000000000000000000000X")

// Credentials absent: no TLS, no SASL — the owner's in-cluster mode (17/10/2026).
func TestNoCredentialsMeansNoSASLNoTLS(t *testing.T) {
	sec, err := securityOf(config.KafkaEventBroker{Addresses: []string{"kafka-0:9092"}})
	if err != nil {
		t.Fatal(err)
	}
	if sec.tls || sec.sasl != nil {
		t.Errorf("no credentials must mean plaintext without SASL, got %+v", sec)
	}
}

// Credentials present: SASL with the configured mechanism, and ALWAYS over TLS.
func TestCredentialsMeanSASLOverTLS(t *testing.T) {
	for _, mech := range []string{config.KafkaSASLPlain, config.KafkaSASLSCRAMSHA256, config.KafkaSASLSCRAMSHA512} {
		sec, err := securityOf(config.KafkaEventBroker{
			Addresses: []string{"kafka-0:9093"}, Username: secret.Secret("u"), Password: secret.Secret("p-FAKE"),
			SASLMechanism: mech,
		})
		if err != nil {
			t.Fatalf("%s: %v", mech, err)
		}
		if !sec.tls {
			t.Errorf("%s: SASL credentials must never travel without TLS", mech)
		}
		if sec.sasl == nil || sec.sasl.Name() != mech {
			t.Errorf("%s: mechanism %v", mech, sec.sasl)
		}
	}
}

// A hand-built value with credentials and an unknown mechanism is refused — never plaintext instead.
func TestUnknownMechanismRefused(t *testing.T) {
	_, err := securityOf(config.KafkaEventBroker{
		Addresses: []string{"kafka-0:9093"}, Username: secret.Secret("u"), Password: secret.Secret("p"),
		SASLMechanism: "GSSAPI",
	})
	if err == nil {
		t.Fatal("an unknown mechanism must be refused")
	}
}

func TestNewRefusesNoAddress(t *testing.T) {
	if _, err := New("comms", "dev", config.KafkaEventBroker{}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("want ErrNotConfigured, got %v", err)
	}
}

// A missing profile is refused at startup: on the shared cluster an unprefixed topic is prod's topic.
func TestNewRefusesNoProfile(t *testing.T) {
	for _, p := range []string{"", "production"} {
		if _, err := New("comms", p, config.KafkaEventBroker{Addresses: []string{"127.0.0.1:1"}}); !errors.Is(err, events.ErrNoProfile) {
			t.Errorf("profile %q: want ErrNoProfile, got %v", p, err)
		}
	}
}

// New dials nothing: a broker that is down must not stop the service from starting.
func TestNewDoesNotDial(t *testing.T) {
	p, err := New("comms", "dev", config.KafkaEventBroker{Addresses: []string{"127.0.0.1:1", "127.0.0.1:2"}})
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
}

// The payload reaches the wire byte for byte — whitespace included — inside a valid JSON envelope.
func TestEncodeKeepsPayloadVerbatim(t *testing.T) {
	raw := []byte("{ \"app_id\" : \"1\",\n  \"event_name\":\"user_send_text\" }")
	at := time.Date(2026, 10, 17, 3, 4, 5, 0, time.UTC)
	out, err := encodeEnvelope(events.Envelope{ID: "01J0000000000000000000000E", Name: "comms.zalo_oa_event_received.v1",
		TenantID: testCommune, At: at, Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, append([]byte(`"payload":`), raw...)) {
		t.Fatalf("payload not verbatim: %s", out)
	}
	var back struct {
		ID       string          `json:"id"`
		Name     string          `json:"name"`
		TenantID string          `json:"tenant_id"`
		At       time.Time       `json:"at"`
		Payload  json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("not one JSON document: %v (%s)", err, out)
	}
	if back.TenantID != string(testCommune) || back.Name != "comms.zalo_oa_event_received.v1" || !back.At.Equal(at) ||
		back.ID != "01J0000000000000000000000E" {
		t.Errorf("envelope head lost: %+v", back)
	}
	if !bytes.Equal(back.Payload, raw) {
		t.Errorf("payload changed: %q", back.Payload)
	}
}

func TestEncodeRefusesBadEnvelopes(t *testing.T) {
	valid := events.Envelope{ID: "x", Name: "comms.zalo_oa_event_received.v1", TenantID: testCommune, Payload: []byte(`{}`)}
	cases := map[string]func(e *events.Envelope){
		"no commune":    func(e *events.Envelope) { e.TenantID = "" },
		"bad commune":   func(e *events.Envelope) { e.TenantID = "commune-a" },
		"no name":       func(e *events.Envelope) { e.Name = "" },
		"no id":         func(e *events.Envelope) { e.ID = "" },
		"not JSON":      func(e *events.Envelope) { e.Payload = []byte("{") },
		"empty payload": func(e *events.Envelope) { e.Payload = nil },
	}
	for name, mutate := range cases {
		e := valid
		mutate(&e)
		if _, err := encodeEnvelope(e); !errors.Is(err, ErrInvalidEnvelope) {
			t.Errorf("%s: want ErrInvalidEnvelope, got %v", name, err)
		}
	}
	if _, err := encodeEnvelope(valid); err != nil {
		t.Errorf("a valid envelope was refused: %v", err)
	}
}
