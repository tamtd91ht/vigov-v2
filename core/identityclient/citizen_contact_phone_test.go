package identityclient

// What these tests defend on THIS side of the wire: an empty answer stays an empty answer, an outage
// stays an error (never "no phone"), the commune rides in metadata, and the number never reaches a log.

import (
	"bytes"
	"context"
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
	contactSessionID = "01JE1SESSIONAAAAAAAAAAAAAA"
	contactCitizenID = "01JE1CITIZENBBBBBBBBBBBBBB"
	// The agreed fake number (rule 3, invariant 5).
	contactPhone = "0900000000"
)

// contactPhoneServer is identity answering ResolveCitizenContactPhone, and nothing else.
type contactPhoneServer struct {
	identityv1.UnimplementedIdentityServiceServer

	phone string
	err   error

	calls  int
	seen   *identityv1.ResolveCitizenContactPhoneRequest
	seenXa []string
}

func (s *contactPhoneServer) ResolveCitizenContactPhone(ctx context.Context,
	in *identityv1.ResolveCitizenContactPhoneRequest) (*identityv1.ResolveCitizenContactPhoneResponse, error) {
	s.calls++
	s.seen = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.seenXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveCitizenContactPhoneResponse{VerifiedPhone: s.phone}, nil
}

func TestResolveCitizenContactPhoneReturnsNumberAndCarriesCommune(t *testing.T) {
	srv := &contactPhoneServer{phone: contactPhone}
	c := moMay(t, srv)

	got, err := c.ResolveCitizenContactPhone(ngucCanh(), contactSessionID, contactCitizenID)
	if err != nil {
		t.Fatalf("ResolveCitizenContactPhone: %v", err)
	}
	if got != contactPhone {
		t.Errorf("phone = %q, want the number identity answered", got)
	}
	if srv.seen.GetSessionId() != contactSessionID || srv.seen.GetCitizenId() != contactCitizenID {
		t.Errorf("request = session %q citizen %q, want both passed verbatim",
			srv.seen.GetSessionId(), srv.seen.GetCitizenId())
	}
	if len(srv.seenXa) != 1 || srv.seenXa[0] != string(xaA) {
		t.Errorf("x-tenant-id on the wire = %v, want exactly %q — this RPC is never commune-exempt",
			srv.seenXa, string(xaA))
	}
}

// "" IS A REAL ANSWER: no live session of this citizen. It must come back as "", nil — the caller
// maps it to 401 — and never be turned into an error (that would be a 503 for a dead session).
func TestResolveCitizenContactPhoneEmptyAnswerIsNotAnError(t *testing.T) {
	c := moMay(t, &contactPhoneServer{phone: ""})

	got, err := c.ResolveCitizenContactPhone(ngucCanh(), contactSessionID, contactCitizenID)
	if err != nil {
		t.Fatalf("err = %v, want nil — an empty answer is an answer", err)
	}
	if got != "" {
		t.Errorf("phone = %q, want empty", got)
	}
}

// AN OUTAGE IS AN ERROR WITH ITS CODE KEPT, never "no phone". And the warning names the code only.
func TestResolveCitizenContactPhoneUnavailableKeepsCodeAndLogsNoIdentifiers(t *testing.T) {
	c := moMay(t, &contactPhoneServer{err: status.Error(codes.Unavailable, "identity down")})
	var buf bytes.Buffer
	c.log = slog.New(slog.NewTextHandler(&buf, nil))

	got, err := c.ResolveCitizenContactPhone(ngucCanh(), contactSessionID, contactCitizenID)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code = %v, want Unavailable (err: %v)", status.Code(err), err)
	}
	if got != "" {
		t.Errorf("phone = %q alongside an error", got)
	}
	for _, leak := range []string{contactSessionID, contactCitizenID, contactPhone} {
		if strings.Contains(buf.String(), leak) || strings.Contains(err.Error(), leak) {
			t.Errorf("%q leaked into the log or the error: log=%q err=%q", leak, buf.String(), err)
		}
	}
}

// A call that can only fail never leaves the process.
func TestResolveCitizenContactPhoneRefusesEmptyArgumentsLocally(t *testing.T) {
	for name, args := range map[string][2]string{
		"no session": {"", contactCitizenID},
		"no citizen": {contactSessionID, ""},
	} {
		t.Run(name, func(t *testing.T) {
			srv := &contactPhoneServer{phone: contactPhone}
			c := moMay(t, srv)
			if _, err := c.ResolveCitizenContactPhone(ngucCanh(), args[0], args[1]); err == nil {
				t.Fatal("no error")
			}
			if srv.calls != 0 {
				t.Errorf("server called %d times for a request that can only fail", srv.calls)
			}
		})
	}
}

// NO COMMUNE, NO CALL: the client interceptor refuses with InvalidArgument before the wire.
func TestResolveCitizenContactPhoneWithoutCommuneIsRefusedLocally(t *testing.T) {
	srv := &contactPhoneServer{phone: contactPhone}
	c := moMay(t, srv)

	_, err := c.ResolveCitizenContactPhone(context.Background(), contactSessionID, contactCitizenID)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument from grpcx.UnaryClientInterceptor (err: %v)",
			status.Code(err), err)
	}
	if srv.calls != 0 {
		t.Errorf("reached the server %d times without a commune", srv.calls)
	}
}
