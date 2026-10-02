package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Unit tests for PetitionSettingsStore on driver_gia_test.go's fake driver. PROVED: the commune is $1
// and comes from the context; no row answers the decided default TRUE; a stored row is read as stored
// (both values, so neither can be the one the code assumes); two rows and a driver failure refuse.
// NOT PROVED: anything PostgreSQL does — the table, its key and its guard are migration 0028's, checked
// as text in migrations/petition_settings_test.go.

func newPetitionSettingsStore(k *khoGia) *PetitionSettingsStore {
	return NewPetitionSettingsStore(pkgstore.New(moKhoGia(k)))
}

func settingsRow(required bool) map[string]driver.Value {
	return map[string]driver.Value{"verification_photo_required": required}
}

func TestVerificationPhotoRequiredNoRowIsTrue(t *testing.T) {
	k := &khoGia{}
	got, err := newPetitionSettingsStore(k).VerificationPhotoRequired(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("no row: %v", err)
	}
	if !got {
		t.Fatal("a commune with no row must get the decided default TRUE (ADR 0008 decision 3)")
	}
	if len(k.lenh) != 1 {
		t.Fatalf("ran %d statements, want 1", len(k.lenh))
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, " FROM petition_settings WHERE tenant_id = $1 ") {
		t.Errorf("statement not scoped to the commune: %q", l.sql)
	}
	if len(l.args) != 1 || l.args[0] != string(xaThu) {
		t.Errorf("args = %v, want only the commune from the context %q", l.args, xaThu)
	}
}

func TestVerificationPhotoRequiredReadsStoredValue(t *testing.T) {
	for _, want := range []bool{false, true} {
		k := &khoGia{hangTheoCot: []map[string]driver.Value{settingsRow(want)}}
		got, err := newPetitionSettingsStore(k).VerificationPhotoRequired(ctxXa(xaThu))
		if err != nil {
			t.Fatalf("stored %v: %v", want, err)
		}
		if got != want {
			t.Errorf("stored %v, read %v", want, got)
		}
	}
}

// The commune follows the context: a second commune is bound as $1, never the first one cached.
func TestVerificationPhotoRequiredFollowsContextCommune(t *testing.T) {
	other := tenant.ID("01JB" + strings.Repeat("B", 22))
	k := &khoGia{}
	s := newPetitionSettingsStore(k)
	if _, err := s.VerificationPhotoRequired(ctxXa(xaThu)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerificationPhotoRequired(ctxXa(other)); err != nil {
		t.Fatal(err)
	}
	if k.lenh[1].args[0] != string(other) {
		t.Errorf("second read bound %v, want %q", k.lenh[1].args[0], other)
	}
}

func TestVerificationPhotoRequiredTwoRowsRefuses(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{settingsRow(false), settingsRow(true)}}
	_, err := newPetitionSettingsStore(k).VerificationPhotoRequired(ctxXa(xaThu))
	if !errors.Is(err, ErrPetitionSettingsDuplicate) {
		t.Fatalf("two rows: err = %v, want ErrPetitionSettingsDuplicate", err)
	}
}

// A driver failure is an error, NEVER the default: the caller is the closing act.
func TestVerificationPhotoRequiredDriverErrorIsWrapped(t *testing.T) {
	boom := errors.New("connection reset")
	k := &khoGia{loi: boom}
	_, err := newPetitionSettingsStore(k).VerificationPhotoRequired(ctxXa(xaThu))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the driver error wrapped", err)
	}
}

func TestVerificationPhotoRequiredNoCommunePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("read with no commune in the context did not panic")
		}
	}()
	_, _ = newPetitionSettingsStore(&khoGia{}).VerificationPhotoRequired(context.Background())
}
