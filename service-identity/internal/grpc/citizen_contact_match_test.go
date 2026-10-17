package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
)

// The two handlers' status-code tables (identity.proto, FindCitizenContactByPhone and
// GetCitizenContactMatchPhone). The fold, the transaction and the audit entry are the use case's, tested
// in internal/app; the live / commune predicate is the store's, tested against PostgreSQL.

const (
	contactIDSample = "CT01JCONTACTSAMPLE000000000"
	actorCodeSample = "CB-00123"
)

type contactMatchFake struct {
	// Find
	findID    string
	findErr   error
	findCalls int
	findRaw   string
	// Match
	matchPhone string
	matchErr   error
	matchCalls int
	matchID    string
	matchActor string
}

func (f *contactMatchFake) FindByPhone(_ context.Context, raw string) (string, bool, error) {
	f.findCalls++
	f.findRaw = raw
	if f.findErr != nil {
		return "", false, f.findErr
	}
	return f.findID, f.findID != "", nil
}

func (f *contactMatchFake) MatchPhone(_ context.Context, id, actor string) (string, bool, error) {
	f.matchCalls++
	f.matchID, f.matchActor = id, actor
	if f.matchErr != nil {
		return "", false, f.matchErr
	}
	return f.matchPhone, f.matchPhone != "", nil
}

func TestFindContactFoundReturnsIDOnly(t *testing.T) {
	f := &contactMatchFake{findID: contactIDSample}
	s, log := may(t, func(d *Deps) { d.ContactMatch = f })

	resp, err := s.FindCitizenContactByPhone(ctxXa(xaA), &identityv1.FindCitizenContactByPhoneRequest{Phone: phoneSample})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetFound() || resp.GetContactId() != contactIDSample {
		t.Fatalf("resp = found %v, id %q", resp.GetFound(), resp.GetContactId())
	}
	// The number goes to the use case AS SENT — identity folds it, never the caller.
	if f.findRaw != phoneSample {
		t.Errorf("use case got %q", f.findRaw)
	}
	if strings.Contains(log.String(), phoneSample) {
		t.Fatalf("the number reached the log: %s", log.String())
	}
}

func TestFindContactAbsentIsOKNotFoundAndUnlogged(t *testing.T) {
	f := &contactMatchFake{} // finds nothing
	s, log := may(t, func(d *Deps) { d.ContactMatch = f })

	resp, err := s.FindCitizenContactByPhone(ctxXa(xaA), &identityv1.FindCitizenContactByPhoneRequest{Phone: phoneSample})
	if err != nil {
		t.Fatalf("err = %v (%v), want OK — absence is never NOT_FOUND", err, status.Code(err))
	}
	if resp.GetFound() || resp.GetContactId() != "" {
		t.Fatalf("resp = found %v, id %q — want empty", resp.GetFound(), resp.GetContactId())
	}
	if log.Len() != 0 {
		t.Errorf("an ordinary negative was logged: %s", log.String())
	}
}

func TestFindContactEmptyPhoneIsInvalidArgument(t *testing.T) {
	f := &contactMatchFake{findErr: errors.Join(app.ErrCitizenContactMatchInvalid, errors.New("thiếu số điện thoại"))}
	s, _ := may(t, func(d *Deps) { d.ContactMatch = f })

	_, err := s.FindCitizenContactByPhone(ctxXa(xaA), &identityv1.FindCitizenContactByPhoneRequest{Phone: "  "})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestFindContactFailureIsInternalAndLogsNoNumber(t *testing.T) {
	f := &contactMatchFake{findErr: fmt.Errorf("tim_danh_ba_theo_so: xã %s: %w", xaA, loiKho)}
	s, log := may(t, func(d *Deps) { d.ContactMatch = f })

	_, err := s.FindCitizenContactByPhone(ctxXa(xaA), &identityv1.FindCitizenContactByPhoneRequest{Phone: phoneSample})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal — an outage is not 'not in the directory'", status.Code(err))
	}
	if strings.Contains(log.String(), phoneSample) || strings.Contains(err.Error(), phoneSample) {
		t.Error("the number leaked into the log or the error")
	}
}

func TestFindContactWithoutCommuneDoesNotRead(t *testing.T) {
	f := &contactMatchFake{findID: contactIDSample}
	s, _ := may(t, func(d *Deps) { d.ContactMatch = f })

	_, err := s.FindCitizenContactByPhone(context.Background(), &identityv1.FindCitizenContactByPhoneRequest{Phone: phoneSample})
	if err == nil || f.findCalls != 0 {
		t.Fatalf("err = %v, calls = %d — want a refusal and no read", err, f.findCalls)
	}
}

func TestMatchPhoneFoundPassesKeysThroughUnlogged(t *testing.T) {
	f := &contactMatchFake{matchPhone: phoneSample}
	s, log := may(t, func(d *Deps) { d.ContactMatch = f })

	resp, err := s.GetCitizenContactMatchPhone(ctxXa(xaA), &identityv1.GetCitizenContactMatchPhoneRequest{
		ContactId: contactIDSample, ActorCode: actorCodeSample})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetFound() || resp.GetPhone() != phoneSample {
		t.Fatalf("resp = found %v", resp.GetFound())
	}
	if f.matchID != contactIDSample || f.matchActor != actorCodeSample {
		t.Errorf("use case got (%q, %q) — the two keys must not be swapped", f.matchID, f.matchActor)
	}
	if strings.Contains(log.String(), phoneSample) {
		t.Fatalf("the number reached the log: %s", log.String())
	}
}

func TestMatchPhoneAbsentIsOKEmpty(t *testing.T) {
	f := &contactMatchFake{} // no live row
	s, log := may(t, func(d *Deps) { d.ContactMatch = f })

	resp, err := s.GetCitizenContactMatchPhone(ctxXa(xaA), &identityv1.GetCitizenContactMatchPhoneRequest{
		ContactId: contactIDSample, ActorCode: actorCodeSample})
	if err != nil {
		t.Fatalf("err = %v, want OK", err)
	}
	if resp.GetFound() || resp.GetPhone() != "" {
		t.Fatal("an absent row answered a number")
	}
	if log.Len() != 0 {
		t.Errorf("an ordinary negative was logged: %s", log.String())
	}
}

func TestMatchPhoneInvalidIsInvalidArgument(t *testing.T) {
	f := &contactMatchFake{matchErr: errors.Join(app.ErrCitizenContactMatchInvalid,
		errors.New("actor_code không phải mã cán bộ còn hiệu lực của xã này"))}
	s, _ := may(t, func(d *Deps) { d.ContactMatch = f })

	_, err := s.GetCitizenContactMatchPhone(ctxXa(xaA), &identityv1.GetCitizenContactMatchPhoneRequest{
		ContactId: contactIDSample, ActorCode: "CB-GONE"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
	if strings.Contains(err.Error(), contactIDSample) || strings.Contains(err.Error(), "CB-GONE") {
		t.Errorf("the refusal quotes a key: %v", err)
	}
}

func TestMatchPhoneFailureIsInternalAndLogsNoIdentifier(t *testing.T) {
	f := &contactMatchFake{matchErr: fmt.Errorf("mo_so_xem_phan_anh_nguoi_dan: xã %s: %w", xaA, loiKho)}
	s, log := may(t, func(d *Deps) { d.ContactMatch = f })

	_, err := s.GetCitizenContactMatchPhone(ctxXa(xaA), &identityv1.GetCitizenContactMatchPhoneRequest{
		ContactId: contactIDSample, ActorCode: actorCodeSample})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	for _, v := range []string{phoneSample, contactIDSample} {
		if strings.Contains(log.String(), v) || strings.Contains(err.Error(), v) {
			t.Errorf("%q leaked into the log or the error", v)
		}
	}
}
