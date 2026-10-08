package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The handler's status-code table (identity.proto, ResolveCitizenContactPhone). The transaction and
// the audit entry are the use case's, tested in internal/app; the predicate is the store's.

const (
	phoneSample   = "0900000000" // the agreed fake number (rule 3, invariant 5)
	sidSample     = "SID-01JSESSIONSAMPLE0000000"
	citizenSample = "CD-01JCITIZENSAMPLE00000000"
)

type contactPhoneFake struct {
	phone string
	err   error
	calls int
	sid   string
	cit   string
}

func (f *contactPhoneFake) Reveal(_ context.Context, sid, citizen string) (string, error) {
	f.calls++
	f.sid, f.cit = sid, citizen
	if f.err != nil {
		return "", f.err
	}
	if f.phone == "" {
		return "", idstore.ErrNoContactPhone
	}
	return f.phone, nil
}

func TestContactPhoneEmptyFieldsAreInvalidArgument(t *testing.T) {
	f := &contactPhoneFake{phone: phoneSample}
	s, _ := may(t, func(d *Deps) { d.ContactPhones = f })

	for _, req := range []*identityv1.ResolveCitizenContactPhoneRequest{
		{CitizenId: citizenSample},
		{SessionId: sidSample},
		{},
	} {
		_, err := s.ResolveCitizenContactPhone(ctxXa(xaA), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", status.Code(err))
		}
	}
	if f.calls != 0 {
		t.Errorf("the use case ran %d times for an empty key", f.calls)
	}
}

// Every "nothing to disclose" case reaches the handler as ONE sentinel, and leaves as OK + "" —
// never NotFound, never a log line.
func TestContactPhoneNoUsableSessionIsOKEmpty(t *testing.T) {
	f := &contactPhoneFake{} // answers ErrNoContactPhone
	s, log := may(t, func(d *Deps) { d.ContactPhones = f })

	resp, err := s.ResolveCitizenContactPhone(ctxXa(xaA), &identityv1.ResolveCitizenContactPhoneRequest{
		SessionId: sidSample, CitizenId: citizenSample})
	if err != nil {
		t.Fatalf("err = %v, want OK", err)
	}
	if resp.GetVerifiedPhone() != "" {
		t.Fatalf("verified_phone = %q, want empty", resp.GetVerifiedPhone())
	}
	if log.Len() != 0 {
		t.Errorf("an ordinary negative was logged: %s", log.String())
	}
}

func TestContactPhoneDisclosedPassesThroughUnlogged(t *testing.T) {
	f := &contactPhoneFake{phone: phoneSample}
	s, log := may(t, func(d *Deps) { d.ContactPhones = f })

	resp, err := s.ResolveCitizenContactPhone(ctxXa(xaA), &identityv1.ResolveCitizenContactPhoneRequest{
		SessionId: sidSample, CitizenId: citizenSample})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetVerifiedPhone() != phoneSample {
		t.Fatalf("verified_phone = %q", resp.GetVerifiedPhone())
	}
	if f.sid != sidSample || f.cit != citizenSample {
		t.Errorf("use case got (%q, %q) — the two keys must not be swapped", f.sid, f.cit)
	}
	if strings.Contains(log.String(), phoneSample) {
		t.Fatalf("the number reached the log: %s", log.String())
	}
}

// A failure is Internal, never "no phone", and the log carries none of the identifiers.
func TestContactPhoneFailureIsInternalAndLogsNoIdentifier(t *testing.T) {
	f := &contactPhoneFake{err: fmt.Errorf("mo_so_dien_thoai_cong_dan: xã %s: %w", xaA, loiKho)}
	s, log := may(t, func(d *Deps) { d.ContactPhones = f })

	_, err := s.ResolveCitizenContactPhone(ctxXa(xaA), &identityv1.ResolveCitizenContactPhoneRequest{
		SessionId: sidSample, CitizenId: citizenSample})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	for _, v := range []string{phoneSample, sidSample, citizenSample} {
		if strings.Contains(log.String(), v) || strings.Contains(err.Error(), v) {
			t.Errorf("%q leaked into the log or the error", v)
		}
	}
}

func TestContactPhoneWithoutCommuneIsInternalNotPanic(t *testing.T) {
	f := &contactPhoneFake{phone: phoneSample}
	s, _ := may(t, func(d *Deps) { d.ContactPhones = f })

	_, err := s.ResolveCitizenContactPhone(context.Background(), &identityv1.ResolveCitizenContactPhoneRequest{
		SessionId: sidSample, CitizenId: citizenSample})
	if status.Code(err) != codes.Internal || f.calls != 0 {
		t.Fatalf("code = %v, calls = %d — want Internal and no read", status.Code(err), f.calls)
	}
}
