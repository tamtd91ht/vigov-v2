package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The two reads behind the petition ↔ directory link (ADR 0102) against a real PostgreSQL (harness in
// checker_pg_test.go). Skipped unless VIGOV_TEST_DSN is set — read the skip line before believing it ran.
//
// What is pinned lives in the SQL: a live row of THIS commune is found by its stored number; the same
// number in another commune, a soft-deleted row and an inactive row each answer as the contract says; and
// the actor check refuses a soft-deleted staff code that StaffCodeExists would accept.

func TestPgLiveIDByPhoneSeesOnlyThisCommunesLiveRows(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	n := directoryCounter.Add(1)
	live := fmt.Sprintf("09%08d", n)
	removed := fmt.Sprintf("08%08d", n)
	inactive := fmt.Sprintf("07%08d", n)
	elsewhere := fmt.Sprintf("03%08d", n)

	liveID := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Nguyễn Văn A", Phone: live, IsActive: true})
	removedID := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Nguyễn Văn B", Phone: removed, IsActive: true})
	inactiveID := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Nguyễn Văn C", Phone: inactive, IsActive: false})
	// The same number as the live row, in ANOTHER commune — it must never be the answer here.
	otherID := citizenContactFixture(t, db, otherXa, domain.CitizenContact{FullName: "Nguyễn Văn D", Phone: live, IsActive: true})
	_ = citizenContactFixture(t, db, otherXa, domain.CitizenContact{FullName: "Nguyễn Văn E", Phone: elsewhere, IsActive: true})

	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, removedID, "CB-00001", "kiểm thử", time.Now())
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	ctx := ctxXa(xa)
	for number, want := range map[string]string{
		live:      liveID,     // live, this commune
		inactive:  inactiveID, // "ngừng liên hệ" is still that person's row (identity.proto)
		removed:   "",         // soft-deleted: never found (rule 7, invariant 2)
		elsewhere: "",         // another commune's only
		"":        "",         // never reaches SQL
	} {
		id, found, err := s.LiveIDByPhone(ctx, number)
		if err != nil {
			t.Fatalf("LiveIDByPhone: %v", err)
		}
		if id != want || found != (want != "") {
			t.Errorf("LiveIDByPhone(row %q) = %q, %v; want %q", want, id, found, want)
		}
	}
	// And the other commune finds ITS row for the shared number, not ours.
	if id, found, err := s.LiveIDByPhone(ctxXa(otherXa), live); err != nil || !found || id != otherID {
		t.Errorf("other commune: %q, %v, %v; want its own row", id, found, err)
	}

	// The full-number read GetCitizenContactMatchPhone uses: same live predicate, keyed by id.
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		for id, want := range map[string]string{liveID: live, inactiveID: inactive, removedID: "", otherID: ""} {
			p, found, err := s.ReadPhone(ctx, tx, id)
			if err != nil {
				return err
			}
			if p != want || found != (want != "") {
				t.Errorf("ReadPhone(%s): found %v — want found %v", id, found, want != "")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPgLiveStaffCodeExistsRefusesRemovedAndOtherCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	n := directoryCounter.Add(1)
	liveCode := fmt.Sprintf("CB-M%05d", n)
	removedCode := fmt.Sprintf("CB-R%05d", n)
	otherCode := fmt.Sprintf("CB-O%05d", n)
	for _, r := range []struct {
		xa, id, code string
		deleted      bool
	}{
		{xa, fmt.Sprintf("NM%024d", n), liveCode, false},
		{xa, fmt.Sprintf("NR%024d", n), removedCode, true},
		{otherXa, fmt.Sprintf("NO%024d", n), otherCode, false},
	} {
		if err := themCanBoMa(db, r.xa, r.id, r.code, r.deleted); err != nil {
			t.Fatalf("themCanBoMa(%s): %v", r.code, err)
		}
	}

	s := NewCanBoStore(pkgstore.New(db))
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		for code, want := range map[string]bool{liveCode: true, removedCode: false, otherCode: false, "": false} {
			ok, err := s.LiveStaffCodeExists(ctx, tx, code)
			if err != nil {
				return err
			}
			if ok != want {
				t.Errorf("LiveStaffCodeExists(%q) = %v, want %v", code, ok, want)
			}
		}
		// The contrast that justifies a second method: the notice check counts the removed code.
		if ok, err := s.StaffCodeExists(ctx, tx, removedCode); err != nil || !ok {
			t.Errorf("StaffCodeExists(removed) = %v, %v — the two checks are meant to differ", ok, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
