package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Fake KEK material — the text says so (rule 8, forbidden #1). Each is exactly 32 bytes.
var (
	kekFake = base64.StdEncoding.EncodeToString([]byte("kek-FAKE-NOT-A-REAL-KEY-32-byte!"))
	kekOld  = base64.StdEncoding.EncodeToString([]byte("kek-OLD-FAKE-NOT-A-REAL-KEY-32b!"))
)

func TestSecretEncryptionAbsentIsNotAnError(t *testing.T) {
	datMoiTruong(t, operatorBase())
	cfg, err := Load("comms")
	if err != nil {
		t.Fatalf("absent SECRET_ENCRYPTION_KEYS must not stop the service: %v", err)
	}
	if cfg.SecretEncryptionConfigured() {
		t.Fatal("nothing set, yet SecretEncryptionConfigured is true")
	}
}

func TestSecretEncryptionLoadsDecodedInOrder(t *testing.T) {
	env := operatorBase()
	env["SECRET_ENCRYPTION_KEYS"] = " " + kekFake + " , " + kekOld + ","
	datMoiTruong(t, env)
	cfg, err := Load("comms")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SecretEncryptionKeys) != 2 ||
		string(cfg.SecretEncryptionKeys[0].Lo()) != "kek-FAKE-NOT-A-REAL-KEY-32-byte!" {
		t.Fatal("KEKs must be held DECODED, in order — the FIRST entry is the one that wraps")
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x"} {
		if strings.Contains(fmt.Sprintf(verb, cfg), "kek-FAKE") {
			t.Fatalf("%s printed KEK material", verb)
		}
	}
}

func TestSecretEncryptionMalformedIsRefused(t *testing.T) {
	for name, raw := range map[string]string{
		"not base64": "not*base64*at*all",
		"undecoded":  "kek-FAKE-NOT-A-REAL-KEY-32-byte!",
		"16 bytes":   base64.StdEncoding.EncodeToString([]byte("only-sixteen-byt")),
		"duplicate":  kekFake + "," + kekFake,
	} {
		t.Run(name, func(t *testing.T) {
			env := operatorBase()
			env["SECRET_ENCRYPTION_KEYS"] = raw
			datMoiTruong(t, env)
			_, err := Load("comms")
			if !errors.Is(err, ErrSecretEncryptionKeysInvalid) {
				t.Fatalf("want ErrSecretEncryptionKeysInvalid, got %v", err)
			}
			if strings.Contains(err.Error(), raw) {
				t.Fatal("the error printed key material (rule 8)")
			}
		})
	}
}
