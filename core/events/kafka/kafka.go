// Package kafka is the Kafka implementation of events.Publisher (ADR 0010: inter-service events travel on
// Kafka). Built on github.com/twmb/franz-go (owner decision 17/10/2026, TASK W-1).
//
// ONE RECORD PER ENVELOPE, AND THE SHAPE IS THE CONTRACT A CONSUMER READS:
//
//	topic   the envelope's Name — "comms.zalo_oa_event_received.v1". One topic per event and version, so
//	        a .v2 runs beside its .v1 during a migration (rule 2, invariant 4) without either consumer
//	        filtering the other's messages out
//	key     the envelope's TenantID — every event of one commune lands on one partition, so a consumer
//	        sees one commune's events in the order they were published, and no commune is ever keyless
//	value   the envelope as one JSON document: {"id","name","tenant_id","at","payload"}. The payload's
//	        bytes are copied VERBATIM — see encodeEnvelope for why encoding/json is not trusted with them
//
// SYNCHRONOUS: Publish returns once the brokers acknowledged the record (acks=all, idempotent producer —
// franz-go's defaults, kept). A caller that answers its own client only after Publish returns therefore
// never claims a delivery the broker did not take.
//
// THE TOPIC IS NOT CREATED HERE. Auto-creation would put partition count and retention — ops' decisions,
// and for a payload carrying personal data a Decree 13 decision — into whichever pod published first. A
// topic that does not exist makes Publish fail, and the caller answers 503.
package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/events"
)

// DeliveryTimeout bounds one Publish: a broker that never answers becomes an error the caller can turn
// into a 503, never a request held open until the client gives up.
const DeliveryTimeout = 10 * time.Second

var (
	// ErrNotConfigured is a broker with no address — the group is off (dev only).
	ErrNotConfigured = errors.New("kafka: KAFKA_EVENT_ADDRESS is not set")
	// ErrInvalidEnvelope is an envelope this publisher refuses to put on the wire: no id, no name, no
	// commune, or a payload that is not JSON.
	ErrInvalidEnvelope = errors.New("kafka: invalid envelope")
)

// Publisher publishes envelopes to Kafka. Safe for concurrent use.
type Publisher struct {
	cl *kgo.Client
}

var _ events.Publisher = (*Publisher)(nil)

// New builds the client. clientID names the service in the brokers' logs and quotas. No connection is
// opened here — franz-go dials on first use — so a broker that is down does not stop the service from
// starting; it makes Publish fail.
func New(clientID string, b config.KafkaEventBroker) (*Publisher, error) {
	opts, err := clientOptions(clientID, b)
	if err != nil {
		return nil, err
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("kafka: build the client: %w", err)
	}
	return &Publisher{cl: cl}, nil
}

// security is how the connection is protected: TLS on or off, and the SASL mechanism (nil = none).
type security struct {
	tls  bool
	sasl sasl.Mechanism
}

// securityOf decides the connection's protection from the configuration — the one decision the owner
// made explicitly (17/10/2026): credentials set ⇒ SASL over TLS; no credentials ⇒ neither.
func securityOf(b config.KafkaEventBroker) (security, error) {
	if !b.UsesSASL() {
		// @security-exception: owner decision 17/10/2026 — in-cluster broker without auth allowed;
		// SASL_SSL used whenever credentials are set. Rule 13 #1 breached on purpose: no TLS, no auth.
		// Said at every startup by config.CanhBao, never silent.
		return security{}, nil
	}
	user, pass := string(b.Username.Lo()), string(b.Password.Lo())
	var m sasl.Mechanism
	switch b.SASLMechanism {
	case config.KafkaSASLPlain:
		m = plain.Auth{User: user, Pass: pass}.AsMechanism()
	case config.KafkaSASLSCRAMSHA256:
		m = scram.Auth{User: user, Pass: pass}.AsSha256Mechanism()
	case config.KafkaSASLSCRAMSHA512:
		m = scram.Auth{User: user, Pass: pass}.AsSha512Mechanism()
	default:
		// core/config refuses this at Load; a direct caller handing a hand-built value is refused again
		// rather than falling back to plaintext with credentials in hand.
		return security{}, fmt.Errorf("kafka: unsupported SASL mechanism %q", b.SASLMechanism)
	}
	// SASL credentials never travel without TLS: PLAIN sends the password as-is.
	return security{tls: true, sasl: m}, nil
}

func clientOptions(clientID string, b config.KafkaEventBroker) ([]kgo.Opt, error) {
	if !b.Configured() {
		return nil, ErrNotConfigured
	}
	sec, err := securityOf(b)
	if err != nil {
		return nil, err
	}
	opts := []kgo.Opt{
		// EVERY broker of the list (rule 11 invariant 5): seeds, from which the client learns the rest.
		kgo.SeedBrokers(b.Addresses...),
		kgo.ClientID(clientID),
		kgo.RecordDeliveryTimeout(DeliveryTimeout),
		// Publish is synchronous and called once per webhook request: waiting to batch only adds latency.
		kgo.ProducerLinger(0),
	}
	if sec.tls {
		// Verified against the system roots — never InsecureSkipVerify (rule 13 #1).
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}
	if sec.sasl != nil {
		opts = append(opts, kgo.SASL(sec.sasl))
	}
	return opts, nil
}

// Publish sends e and waits for the brokers' acknowledgement.
func (p *Publisher) Publish(ctx context.Context, e events.Envelope) error {
	value, err := encodeEnvelope(e)
	if err != nil {
		return err
	}
	rec := &kgo.Record{Topic: e.Name, Key: []byte(e.TenantID), Value: value}
	if err := p.cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
		// The error names the topic, never the payload.
		return fmt.Errorf("kafka: publish to %q: %w", e.Name, err)
	}
	return nil
}

// Close flushes nothing (every Publish already waited) and releases the connections.
func (p *Publisher) Close() { p.cl.Close() }

// envelopeHead is events.Envelope without its payload — the same JSON names.
type envelopeHead struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	TenantID string    `json:"tenant_id"`
	At       time.Time `json:"at"`
}

// encodeEnvelope renders e as {"id","name","tenant_id","at","payload"} with the payload bytes VERBATIM.
//
// NOT json.Marshal(e): encoding/json compacts whatever a json.RawMessage returns, so the payload that
// arrives in Kafka would differ in whitespace from what the publisher was handed — and the payload of a
// forwarded webhook is somebody else's signed bytes, which a consumer may need to re-verify byte for byte.
func encodeEnvelope(e events.Envelope) ([]byte, error) {
	switch {
	case e.ID == "", e.Name == "":
		return nil, fmt.Errorf("%w: no id or no name", ErrInvalidEnvelope)
	case !e.TenantID.Valid():
		// A message without a commune is refused at the source too, not only by events.Dispatch.
		return nil, fmt.Errorf("%w: %q carries no commune", ErrInvalidEnvelope, e.Name)
	case !json.Valid(e.Payload):
		return nil, fmt.Errorf("%w: %q payload is not JSON", ErrInvalidEnvelope, e.Name)
	}
	head, err := json.Marshal(envelopeHead{ID: e.ID, Name: e.Name, TenantID: string(e.TenantID), At: e.At.UTC()})
	if err != nil {
		return nil, fmt.Errorf("kafka: encode the envelope: %w", err)
	}
	out := make([]byte, 0, len(head)+len(e.Payload)+12)
	out = append(out, head[:len(head)-1]...) // drop the closing brace
	out = append(out, `,"payload":`...)
	out = append(out, e.Payload...)
	out = append(out, '}')
	return out, nil
}
