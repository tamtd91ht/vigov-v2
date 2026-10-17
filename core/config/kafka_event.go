package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/secret"
)

// KAFKA_EVENT_ADDRESS · KAFKA_EVENT_USERNAME · KAFKA_EVENT_PASSWORD · KAFKA_EVENT_SASL_MECHANISM — the
// Kafka that carries BUSINESS EVENTS between services (ADR 0010: inter-service events are Kafka's, never
// RabbitMQ's). Group KafkaEvent. First declared by service-comms on 17/10/2026, to forward every Zalo OA
// webhook event (owner decision, TASK W-1).
//
// THE NAME IS THE ROLE, NOT THE CLUSTER (rule 11 invariant 3): `KAFKA_EVENT_*` is "the Kafka that carries
// events". Which physical cluster that is, is the VALUE the operator puts in common-config — moving the
// workload is one value and a restart, never a release.
//
// THE ADDRESS IS CLUSTER-SHAPED FROM THE FIRST LINE (rule 11 invariant 5): a comma-separated host:port
// list, handed WHOLE to the client as its seed brokers. Never split to "the" host: that is correct for as
// long as there is one broker and silently wrong on the day of the second.
//
// CREDENTIALS ARE OPTIONAL — OWNER DECISION 17/10/2026. Username + password set: SASL over TLS
// (SASL_SSL) with the named mechanism. Neither set: the client connects to the address with NO
// authentication and NO encryption, which the owner accepted for an in-cluster broker; the plaintext
// branch carries its @security-exception in core/events/kafka. Exactly one of the two set, or a mechanism
// without credentials, is refused: the usual cause is a typo'd key, and the result would be a client that
// silently connects in plaintext while the operator believes it authenticates.
//
// REQUIRED: the ADDRESS only, in staging/prod, for a service that declares the group (ADR 0057 — used ⇒
// required). In dev an empty address is "off": the route that publishes answers 503 at use, never a
// silent drop. The credentials and the mechanism are never required — their absence IS the owner's
// plaintext mode, so requiring them would contradict the decision.

// Kafka SASL mechanisms the client supports. The strings are Kafka's own names, compared upper-case.
const (
	KafkaSASLPlain       = "PLAIN"
	KafkaSASLSCRAMSHA256 = "SCRAM-SHA-256"
	KafkaSASLSCRAMSHA512 = "SCRAM-SHA-512"
)

// ErrKafkaEventInvalid is a malformed KAFKA_EVENT_* group. Load refuses, by variable name, never with a
// credential in the message.
var ErrKafkaEventInvalid = errors.New("config: KAFKA_EVENT_* is invalid")

// KafkaEventBroker is the event broker connection, consumed by core/events/kafka.New.
type KafkaEventBroker struct {
	// Addresses are the seed brokers, every one kept, in the order written.
	Addresses []string
	// Username and Password are both set or both nil. secret.Secret: the username is not a credential on
	// its own, but it sits in the same Secret as the password and is printed by nothing.
	Username secret.Secret
	Password secret.Secret
	// SASLMechanism is one of the KafkaSASL* constants when credentials are set, "" otherwise.
	SASLMechanism string
}

// Configured reports whether an address was given. false = the group is off (dev only).
func (k KafkaEventBroker) Configured() bool { return len(k.Addresses) > 0 }

// UsesSASL reports whether the connection authenticates (and therefore runs over TLS).
func (k KafkaEventBroker) UsesSASL() bool { return !k.Username.Rong() }

// parseKafkaEvent validates the four trimmed values. All empty = the zero KafkaEventBroker (off).
func parseKafkaEvent(addrRaw, userRaw, passRaw, mechRaw string) (KafkaEventBroker, error) {
	var k KafkaEventBroker
	for i, a := range strings.Split(addrRaw, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if err := kafkaBrokerShape(a); err != nil {
			// The address is not a credential: quoting it points the operator at the exact entry.
			return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_ADDRESS entry %d (%q) %v — "+
				"expected host:port[,host:port], no scheme", ErrKafkaEventInvalid, i+1, a, err)
		}
		k.Addresses = append(k.Addresses, a)
	}

	switch {
	case userRaw != "" && passRaw == "":
		return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_USERNAME is set but KAFKA_EVENT_PASSWORD is empty — "+
			"set both (SASL over TLS) or neither (plaintext, owner decision 17/10/2026)", ErrKafkaEventInvalid)
	case userRaw == "" && passRaw != "":
		return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_PASSWORD is set but KAFKA_EVENT_USERNAME is empty — "+
			"set both (SASL over TLS) or neither (plaintext, owner decision 17/10/2026)", ErrKafkaEventInvalid)
	}
	mech := strings.ToUpper(mechRaw)
	if userRaw == "" {
		if mech != "" {
			return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_SASL_MECHANISM is set but there are no credentials — "+
				"a mechanism without KAFKA_EVENT_USERNAME/PASSWORD would connect in plaintext while it reads as "+
				"authenticated; remove it, or set the credentials", ErrKafkaEventInvalid)
		}
		return k, nil
	}
	switch mech {
	case KafkaSASLPlain, KafkaSASLSCRAMSHA256, KafkaSASLSCRAMSHA512:
	case "":
		// No default: which mechanism the broker speaks is the broker's fact, and a guessed one is an
		// authentication failure on every publish that names the wrong cause.
		return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_USERNAME/PASSWORD are set but KAFKA_EVENT_SASL_MECHANISM "+
			"is empty — set %s, %s or %s, as the broker is configured", ErrKafkaEventInvalid,
			KafkaSASLPlain, KafkaSASLSCRAMSHA256, KafkaSASLSCRAMSHA512)
	default:
		return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_SASL_MECHANISM %q is not supported — %s, %s or %s",
			ErrKafkaEventInvalid, mechRaw, KafkaSASLPlain, KafkaSASLSCRAMSHA256, KafkaSASLSCRAMSHA512)
	}
	if len(k.Addresses) == 0 {
		// Credentials in a process that never connects: the usual cause is a typo'd address key.
		return KafkaEventBroker{}, fmt.Errorf("%w: KAFKA_EVENT_USERNAME/PASSWORD are set but KAFKA_EVENT_ADDRESS is empty",
			ErrKafkaEventInvalid)
	}
	k.Username = secret.Secret(userRaw)
	k.Password = secret.Secret(passRaw)
	k.SASLMechanism = mech
	return k, nil
}

// kafkaBrokerShape: host:port, port 1–65535, no scheme, no path, no whitespace.
func kafkaBrokerShape(a string) error {
	if strings.Contains(a, "://") || strings.ContainsAny(a, "/ \t") {
		return errors.New("is not a bare host:port")
	}
	host, port, err := net.SplitHostPort(a)
	if err != nil || host == "" {
		return errors.New("is not host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("has no valid port")
	}
	return nil
}
