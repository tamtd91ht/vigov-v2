package identityclient

// The two ADR 0102 wrappers: found / not found / outage kept apart, the commune and the actor code on the
// wire, nothing that can only fail ever sent, and neither the number nor the contact id in a log or error.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

const (
	matchContactID = "01JE1CONTACTCCCCCCCCCCCCCC"
	matchActorCode = "CB-00123"
	// The agreed fake number (rule 3, invariant 5).
	matchPhone = "0900000000"
)

type contactMatchServer struct {
	identityv1.UnimplementedIdentityServiceServer

	found     bool
	contactID string
	phone     string
	err       error

	findCalls, getCalls int
	seenFind            *identityv1.FindCitizenContactByPhoneRequest
	seenGet             *identityv1.GetCitizenContactMatchPhoneRequest
	seenXa              []string
}

func (s *contactMatchServer) FindCitizenContactByPhone(ctx context.Context,
	in *identityv1.FindCitizenContactByPhoneRequest) (*identityv1.FindCitizenContactByPhoneResponse, error) {
	s.findCalls++
	s.seenFind = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.seenXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.FindCitizenContactByPhoneResponse{Found: s.found, ContactId: s.contactID}, nil
}

func (s *contactMatchServer) GetCitizenContactMatchPhone(ctx context.Context,
	in *identityv1.GetCitizenContactMatchPhoneRequest) (*identityv1.GetCitizenContactMatchPhoneResponse, error) {
	s.getCalls++
	s.seenGet = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.seenXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.GetCitizenContactMatchPhoneResponse{Found: s.found, Phone: s.phone}, nil
}

func TestFindCitizenContactByPhoneFoundCarriesCommune(t *testing.T) {
	srv := &contactMatchServer{found: true, contactID: matchContactID}
	c := moMay(t, srv)

	found, id, err := c.FindCitizenContactByPhone(ngucCanh(), matchPhone)
	if err != nil || !found || id != matchContactID {
		t.Fatalf("= %v %q %v, want found with the row id", found, id, err)
	}
	if srv.seenFind.GetPhone() != matchPhone {
		t.Error("the number was not sent as given")
	}
	if len(srv.seenXa) != 1 || srv.seenXa[0] != string(xaA) {
		t.Errorf("x-tenant-id = %v, want exactly %q", srv.seenXa, string(xaA))
	}
}

func TestFindCitizenContactByPhoneNotFoundIsAnAnswer(t *testing.T) {
	c := moMay(t, &contactMatchServer{found: false})
	found, id, err := c.FindCitizenContactByPhone(ngucCanh(), matchPhone)
	if err != nil || found || id != "" {
		t.Fatalf("= %v %q %v, want not found and no error", found, id, err)
	}
}

func TestFindCitizenContactByPhoneFoundWithoutIDIsAContractFault(t *testing.T) {
	c := moMay(t, &contactMatchServer{found: true})
	if found, _, err := c.FindCitizenContactByPhone(ngucCanh(), matchPhone); err == nil || found {
		t.Fatalf("found=%v err=%v, want an error — never a found without a row id", found, err)
	}
}

func TestFindCitizenContactByPhoneOutageIsUnavailableAndLeaksNothing(t *testing.T) {
	c := moMay(t, &contactMatchServer{err: status.Error(codes.Unavailable, "down")})
	var buf bytes.Buffer
	c.log = slog.New(slog.NewTextHandler(&buf, nil))

	found, _, err := c.FindCitizenContactByPhone(ngucCanh(), matchPhone)
	if found || !errors.Is(err, ErrIdentityUnavailable) || status.Code(err) != codes.Unavailable {
		t.Fatalf("found=%v err=%v, want ErrIdentityUnavailable with the code kept", found, err)
	}
	if strings.Contains(buf.String(), matchPhone) || strings.Contains(err.Error(), matchPhone) {
		t.Errorf("number leaked: log=%q err=%q", buf.String(), err)
	}
}

func TestFindCitizenContactByPhoneRefusesBlankLocally(t *testing.T) {
	srv := &contactMatchServer{found: true, contactID: matchContactID}
	c := moMay(t, srv)
	if _, _, err := c.FindCitizenContactByPhone(ngucCanh(), "  "); err == nil {
		t.Fatal("no error for a blank number")
	}
	if srv.findCalls != 0 {
		t.Errorf("server called %d times for a request that can only fail", srv.findCalls)
	}
}

func TestFindCitizenContactByPhoneWithoutCommuneIsRefusedLocally(t *testing.T) {
	srv := &contactMatchServer{found: true, contactID: matchContactID}
	c := moMay(t, srv)
	_, _, err := c.FindCitizenContactByPhone(context.Background(), matchPhone)
	if status.Code(err) != codes.InvalidArgument || srv.findCalls != 0 {
		t.Errorf("code = %v, calls = %d — want InvalidArgument before the wire", status.Code(err), srv.findCalls)
	}
}

func TestGetCitizenContactMatchPhoneFoundSendsActorCode(t *testing.T) {
	srv := &contactMatchServer{found: true, phone: matchPhone}
	c := moMay(t, srv)

	found, got, err := c.GetCitizenContactMatchPhone(ngucCanh(), matchContactID, matchActorCode)
	if err != nil || !found || got != matchPhone {
		t.Fatalf("= %v %v, want found with the number", found, err)
	}
	if srv.seenGet.GetContactId() != matchContactID || srv.seenGet.GetActorCode() != matchActorCode {
		t.Errorf("request = %q / %q, want the row id and the business code verbatim",
			srv.seenGet.GetContactId(), srv.seenGet.GetActorCode())
	}
	if len(srv.seenXa) != 1 || srv.seenXa[0] != string(xaA) {
		t.Errorf("x-tenant-id = %v, want exactly %q", srv.seenXa, string(xaA))
	}
}

func TestGetCitizenContactMatchPhoneNotFoundIsAnAnswer(t *testing.T) {
	c := moMay(t, &contactMatchServer{found: false})
	found, got, err := c.GetCitizenContactMatchPhone(ngucCanh(), matchContactID, matchActorCode)
	if err != nil || found || got != "" {
		t.Fatalf("= %v (number returned: %v) %v, want not found and no error", found, got != "", err)
	}
}

// A key outside the national form would match nothing — a silent short list. It is refused, unquoted.
func TestGetCitizenContactMatchPhoneRefusesUnnormalisedKey(t *testing.T) {
	const typed = "+84 900 000 000"
	c := moMay(t, &contactMatchServer{found: true, phone: typed})
	var buf bytes.Buffer
	c.log = slog.New(slog.NewTextHandler(&buf, nil))

	found, got, err := c.GetCitizenContactMatchPhone(ngucCanh(), matchContactID, matchActorCode)
	if err == nil || found || got != "" {
		t.Fatalf("found=%v err=%v, want a contract error and no number", found, err)
	}
	if strings.Contains(err.Error(), "900") || strings.Contains(buf.String(), "900") {
		t.Errorf("number leaked: log=%q err=%q", buf.String(), err)
	}
}

func TestGetCitizenContactMatchPhoneOutageIsUnavailable(t *testing.T) {
	c := moMay(t, &contactMatchServer{err: status.Error(codes.DeadlineExceeded, "slow")})
	var buf bytes.Buffer
	c.log = slog.New(slog.NewTextHandler(&buf, nil))
	_, _, err := c.GetCitizenContactMatchPhone(ngucCanh(), matchContactID, matchActorCode)
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("err = %v, want ErrIdentityUnavailable", err)
	}
	if strings.Contains(buf.String(), matchContactID) || strings.Contains(err.Error(), matchContactID) {
		t.Errorf("contact id leaked: log=%q err=%q", buf.String(), err)
	}
}

func TestGetCitizenContactMatchPhoneRefusesBlankArgumentsLocally(t *testing.T) {
	for name, args := range map[string][2]string{
		"no contact": {"", matchActorCode},
		"no actor":   {matchContactID, ""},
	} {
		t.Run(name, func(t *testing.T) {
			srv := &contactMatchServer{found: true, phone: matchPhone}
			c := moMay(t, srv)
			if _, _, err := c.GetCitizenContactMatchPhone(ngucCanh(), args[0], args[1]); err == nil {
				t.Fatal("no error")
			}
			if srv.getCalls != 0 {
				t.Errorf("server called %d times for a request that can only fail", srv.getCalls)
			}
		})
	}
}
