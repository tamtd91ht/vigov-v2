package searchhash

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
)

func key(t *testing.T, fill byte) Key {
	t.Helper()
	k, err := NewKey(secret.Secret(bytes.Repeat([]byte{fill}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSubstringsAreExactlyLengths3To10(t *testing.T) {
	got := Substrings("12345")
	want := []string{"123", "234", "345", "1234", "2345", "12345"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Substrings = %v, want %v", got, want)
	}
	all := Substrings("0123456789")
	if len(all) != 36 { // 8+7+6+5+4+3+2+1, all distinct
		t.Errorf("10 distinct digits: %d substrings, want 36", len(all))
	}
	for _, g := range all {
		if len(g) < 3 || len(g) > 10 {
			t.Errorf("substring %q outside 3..10", g)
		}
	}
	if Substrings("12") != nil {
		t.Error("shorter than 3: want none")
	}
}

func TestSameInputSameHashDifferentKeyDifferentHash(t *testing.T) {
	a, b := key(t, 1), key(t, 2)
	if !reflect.DeepEqual(a.PhoneGrams("0900000000"), a.PhoneGrams("0900000000")) {
		t.Error("the same number must hash the same under the same key")
	}
	ha, _ := a.PhoneQuery("900")
	hb, _ := b.PhoneQuery("900")
	if ha == hb {
		t.Error("two keys must give two hashes")
	}
	if a.Fingerprint() == b.Fingerprint() || len(a.Fingerprint()) != 16 {
		t.Errorf("fingerprints %q / %q", a.Fingerprint(), b.Fingerprint())
	}
	if len(ha) != 32 {
		t.Errorf("hash length %d, want 32 hex", len(ha))
	}
}

func TestQueryHashIsInDocumentHashes(t *testing.T) {
	k := key(t, 7)
	doc := map[string]bool{}
	for _, h := range k.PhoneGrams("0900000001") {
		doc[h] = true
	}
	for _, q := range []string{"090", "0001", "900000", "0900000001"} {
		h, ok := k.PhoneQuery(q)
		if !ok || !doc[h] {
			t.Errorf("query %q not found among the number's hashes", q)
		}
	}
	if h, ok := k.PhoneQuery("123"); ok && doc[h] {
		t.Error("a substring the number does not contain must not match")
	}
	if _, ok := k.PhoneQuery("12"); ok {
		t.Error("two digits must not make a clause")
	}
	if _, ok := k.PhoneQuery("09000000012"); ok {
		t.Error("eleven digits must not make a gram clause")
	}
}

func TestEmailGramsAndExact(t *testing.T) {
	k := key(t, 9)
	grams, exact := k.EmailGrams(" Nguyen.Van.Anh.Long@Example.VN ")
	set := map[string]bool{}
	for _, g := range grams {
		set[g] = true
	}
	// Only the first 10 runes of the local part are cut: "nguyen.van".
	if len(grams) != len(Substrings("nguyen.van")) {
		t.Errorf("%d grams, want %d", len(grams), len(Substrings("nguyen.van")))
	}
	if g, _ := k.EmailQuery("NGUYEN"); !set[g] {
		t.Error("a fragment of the local part must match")
	}
	if g, _ := k.EmailQuery("anh.long"); set[g] {
		t.Error("beyond the 10th rune must not be indexed as grams")
	}
	if _, e := k.EmailQuery("nguyen.van.anh.long@example.vn"); e != exact || e == "" {
		t.Error("the whole address must match the exact hash")
	}
	// A phone substring and an e-mail gram of the same digits never collide (domain separation).
	pg, _ := k.PhoneQuery("0900")
	eg, _ := k.EmailQuery("0900")
	if pg == eg {
		t.Error("phone and e-mail hashes of the same text must differ")
	}
}

func TestShortKeyRefused(t *testing.T) {
	if _, err := NewKey(secret.Secret(strings.Repeat("x", 31))); err != ErrKeyTooShort {
		t.Errorf("err = %v", err)
	}
	var zero Key
	if zero.PhoneGrams("0900000000") != nil {
		t.Error("the zero key must hash nothing")
	}
}
