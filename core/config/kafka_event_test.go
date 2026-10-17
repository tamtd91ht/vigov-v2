package config

// config.KafkaEvent — KAFKA_EVENT_* (ADR 0010; owner 17/10/2026). The address is required in prod; the
// credentials are optional, both-or-neither, and a mechanism comes only with them.

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Fake values. The password carries the word so nobody mistakes it for a real one.
const (
	fakeKafkaUser     = "vigov-comms"
	fakeKafkaPassword = "mat-khau-kafka-GIA-KHONG-PHAI-THAT"
)

func TestKafkaEventAddressRequiredInProdWithItsHint(t *testing.T) {
	for _, env := range []string{EnvStaging, EnvProd} {
		clean(t, env, nil)
		_, err := Load("comms", Uses(KafkaEvent))
		if !errors.Is(err, ErrThieuBienMoiTruong) {
			t.Fatalf("%s: want ErrThieuBienMoiTruong, got %v", env, err)
		}
		for _, want := range []string{"KAFKA_EVENT_ADDRESS", "common-config", "ADR 0010"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not say %q: %v", env, want, err)
			}
		}
		// The credentials are the owner's optional half: never named by the refusal.
		list := err.Error()[:strings.Index(err.Error(), " (service")]
		for _, v := range []string{"KAFKA_EVENT_USERNAME", "KAFKA_EVENT_PASSWORD", "KAFKA_EVENT_SASL_MECHANISM"} {
			if strings.Contains(list, v) {
				t.Errorf("%s: %s is optional by decision, yet the refusal names it: %v", env, v, err)
			}
		}
	}
}

// Credentials absent: plaintext, no SASL — the owner's in-cluster mode. Every broker of the list kept.
func TestKafkaEventWithoutCredentialsIsPlaintext(t *testing.T) {
	clean(t, EnvProd, map[string]string{"KAFKA_EVENT_ADDRESS": " kafka-0:9092, kafka-1:9092 ,kafka-2:9092,\n"})
	cfg, err := Load("comms", Uses(KafkaEvent))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	k := cfg.KafkaEventBroker()
	if want := []string{"kafka-0:9092", "kafka-1:9092", "kafka-2:9092"}; !reflect.DeepEqual(k.Addresses, want) {
		t.Errorf("addresses %v, want every broker %v", k.Addresses, want)
	}
	if k.UsesSASL() || k.SASLMechanism != "" || !k.Username.Rong() || !k.Password.Rong() {
		t.Errorf("no credentials must mean no SASL: %+v", k)
	}
	if !k.Configured() {
		t.Error("an address must read as configured")
	}
	// Said at startup, never silent.
	if w := strings.Join(cfg.CanhBao(), "|"); !strings.Contains(w, "KHÔNG mã hoá") {
		t.Errorf("the plaintext mode must be warned about at startup: %v", cfg.CanhBao())
	}
}

// Credentials present: SASL (over TLS, decided by core/events/kafka from UsesSASL), mechanism normalised.
func TestKafkaEventWithCredentialsUsesSASL(t *testing.T) {
	for _, mech := range []string{"PLAIN", "scram-sha-256", " SCRAM-SHA-512 "} {
		clean(t, EnvProd, map[string]string{
			"KAFKA_EVENT_ADDRESS":        "kafka-0:9093",
			"KAFKA_EVENT_USERNAME":       fakeKafkaUser,
			"KAFKA_EVENT_PASSWORD":       fakeKafkaPassword,
			"KAFKA_EVENT_SASL_MECHANISM": mech,
		})
		cfg, err := Load("comms", Uses(KafkaEvent))
		if err != nil {
			t.Errorf("%q refused: %v", mech, err)
			continue
		}
		k := cfg.KafkaEventBroker()
		if !k.UsesSASL() || k.SASLMechanism != strings.ToUpper(strings.TrimSpace(mech)) {
			t.Errorf("%q: got %+v", mech, k)
		}
		if string(k.Password.Lo()) != fakeKafkaPassword || string(k.Username.Lo()) != fakeKafkaUser {
			t.Errorf("%q: the credentials did not survive Load intact", mech)
		}
		for _, w := range cfg.CanhBao() {
			if strings.Contains(w, "KAFKA") {
				t.Errorf("%q: SASL mode must not warn about plaintext: %s", mech, w)
			}
		}
		// Neither credential reaches any rendering of the config.
		for _, out := range []string{fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg)} {
			if strings.Contains(out, fakeKafkaPassword) || strings.Contains(out, fakeKafkaUser) {
				t.Fatalf("a Kafka credential was printed: %s", out)
			}
		}
	}
}

// Malformed is refused by name, in every environment, and never with the password in the message.
func TestKafkaEventRefusals(t *testing.T) {
	cases := map[string]map[string]string{
		"bad mechanism": {"KAFKA_EVENT_ADDRESS": "kafka-0:9093", "KAFKA_EVENT_USERNAME": fakeKafkaUser,
			"KAFKA_EVENT_PASSWORD": fakeKafkaPassword, "KAFKA_EVENT_SASL_MECHANISM": "GSSAPI"},
		"credentials without mechanism": {"KAFKA_EVENT_ADDRESS": "kafka-0:9093", "KAFKA_EVENT_USERNAME": fakeKafkaUser,
			"KAFKA_EVENT_PASSWORD": fakeKafkaPassword},
		"mechanism without credentials": {"KAFKA_EVENT_ADDRESS": "kafka-0:9092", "KAFKA_EVENT_SASL_MECHANISM": "PLAIN"},
		"username only":                 {"KAFKA_EVENT_ADDRESS": "kafka-0:9092", "KAFKA_EVENT_USERNAME": fakeKafkaUser},
		"password only":                 {"KAFKA_EVENT_ADDRESS": "kafka-0:9092", "KAFKA_EVENT_PASSWORD": fakeKafkaPassword},
		"credentials without address": {"KAFKA_EVENT_USERNAME": fakeKafkaUser, "KAFKA_EVENT_PASSWORD": fakeKafkaPassword,
			"KAFKA_EVENT_SASL_MECHANISM": "PLAIN"},
		"scheme in address": {"KAFKA_EVENT_ADDRESS": "kafka://kafka-0:9092"},
		"no port":           {"KAFKA_EVENT_ADDRESS": "kafka-0:9092,kafka-1"},
		"bad port":          {"KAFKA_EVENT_ADDRESS": "kafka-0:99999"},
	}
	for name, env := range cases {
		for _, e := range []string{EnvDev, EnvProd} {
			clean(t, e, env)
			_, err := Load("comms", Uses(KafkaEvent))
			if name == "credentials without address" && e == EnvProd {
				// Prod refuses it earlier, as a missing required variable — also by name.
				if !errors.Is(err, ErrThieuBienMoiTruong) {
					t.Errorf("%s/%s: want ErrThieuBienMoiTruong, got %v", name, e, err)
				}
				continue
			}
			if !errors.Is(err, ErrKafkaEventInvalid) {
				t.Errorf("%s/%s: want ErrKafkaEventInvalid, got %v", name, e, err)
				continue
			}
			if strings.Contains(err.Error(), fakeKafkaPassword) {
				t.Errorf("%s/%s: the refusal printed the password: %v", name, e, err)
			}
		}
	}
}

// Dev with nothing set: the group is off, and nothing warns about a broker nobody configured.
func TestKafkaEventOffInDev(t *testing.T) {
	clean(t, EnvDev, nil)
	cfg, err := Load("comms", Uses(KafkaEvent))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KafkaEventBroker().Configured() {
		t.Error("empty must read as off")
	}
	if w := cfg.CanhBao(); len(w) != 0 {
		t.Errorf("an unconfigured broker must not warn about plaintext: %v", w)
	}
}
