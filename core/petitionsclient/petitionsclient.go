// Package petitionsclient is how another service asks petitions a question over gRPC.
//
// Today there is one question and one caller: identity, before it soft-deletes an org unit, asks
// how many open records of petitions that unit still holds (CountOrgUnitHoldings — the predicate
// is stated once, in proto/vigov/petitions/v1/petitions.proto, and not repeated here).
//
// WHY IT LIVES IN core/ AND NOT INSIDE service-identity: the same reason as identityclient. The
// transport and the reading of a failure are written once; a second caller would otherwise write
// a second client with a second idea of what an outage means.
//
// # THE DEPENDENCY DIRECTION IS NEW, AND IT IS NOT A CYCLE AT STARTUP
//
// petitions already calls identity (core/identityclient). With this package identity calls
// petitions too. That is two edges, not a startup deadlock: grpc.NewClient connects lazily, so
// neither service needs the other up in order to start, and each call only reaches the other when
// a request makes it. What the two edges DO create is mutual runtime coupling on the org-unit
// delete — that act now fails while petitions is down, deliberately (fail closed, below).
//
// # THE CHANNEL IS PLAINTEXT, BY THE OWNER'S DECISION OF 2026-09-28
//
// A new unencrypted channel is a rule 13 STOP condition; the owner answered it: the same transport
// as identityclient and platformclient, marked with `@security-exception` on the one line in Dial
// rather than a second ledger entry, and converted together with those two when internal TLS lands.
package petitionsclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
)

// CallTimeout bounds one call. The same number as identityclient.HanGoi, deliberately: two
// deadlines on two hops would make the observed timeout depend on which dependency was slow.
const CallTimeout = 3 * time.Second

// ErrPetitionsUnavailable marks "petitions did not answer, asking again may work" — the gRPC code
// was UNAVAILABLE or DEADLINE_EXCEEDED. The caller answers 503 with a retry sentence.
//
// IT IS NEVER AN ANSWER. Not "holds nothing". Any other failure (INVALID_ARGUMENT, the caller key,
// INTERNAL) is a plain error — also a refusal, but one a retry does not fix. The original status is
// wrapped either way, so status.Code(err) still reads it.
var ErrPetitionsUnavailable = errors.New("petitionsclient: petitions tạm thời không trả lời — thử lại sau")

// OrgUnitHoldings is what one org unit still holds open in petitions.
type OrgUnitHoldings struct {
	OpenPetitions uint32
	OpenTasks     uint32
}

// Any reports whether the unit holds anything open here — i.e. whether the delete must be refused.
//
// EVERY FIELD OF THE STRUCT IS IN THIS SUM, and a field added to the contract must be added to the
// struct AND here in the same change. A kind the caller does not read is a kind that reads as zero,
// which is the one direction that silently lets a delete through.
func (h OrgUnitHoldings) Any() bool {
	return h.OpenPetitions > 0 || h.OpenTasks > 0
}

// Client asks petitions over gRPC. NO CACHE: the answer is a point in time and is read once per
// delete.
type Client struct {
	cl   petitionsv1.PetitionsServiceClient
	conn *grpc.ClientConn // nil when the client was injected through New, e.g. in a test
	log  *slog.Logger
}

// Dial opens the connection to petitions. Built exactly like identityclient.Dial: grpc.NewClient
// connects lazily (identity must start while petitions is down — the delete then refuses, it does
// not hang startup), and the caller key goes FIRST, then the commune.
//
// khoa is the deployment's caller key (config.GRPCCallerKey); an empty one panics inside
// grpcx.UnaryClientCallerAuth at construction, for the reason identityclient.Dial gives.
func Dial(addr string, khoa secret.Secret, log *slog.Logger) (*Client, error) {
	if addr == "" {
		// Fail closed and BY NAME. The caller decides what an unconfigured owner means (identity:
		// the org-unit delete answers 503 "not configured"); this package never guesses an address.
		return nil, fmt.Errorf("petitionsclient: thiếu địa chỉ PETITIONS_GRPC_ADDR")
	}
	conn, err := grpc.NewClient(addr,
		// @security-exception: service-to-service gRPC has no TLS yet — same channel as identityclient/platformclient (tools/security_debt.json, expires 2026-12-28), converted together when internal TLS lands; carries one org-unit id and two counts, no personal data — but ALSO the deployment-wide GRPC_CALLER_KEY (x-vigov-caller-key metadata, one key shared by every service) and the commune id (x-tenant-id), both readable by anyone on the cluster network until TLS lands
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpcx.UnaryClientCallerAuth(khoa),
			grpcx.UnaryClientInterceptor(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("petitionsclient: mở kết nối tới %s: %w", addr, err)
	}
	c := New(petitionsv1.NewPetitionsServiceClient(conn), log)
	c.conn = conn
	return c, nil
}

// New wraps a generated client. The connection under it MUST carry, in this order,
// grpcx.UnaryClientCallerAuth and grpcx.UnaryClientInterceptor — the commune travels in metadata
// and nowhere else (rule 1, invariant 8). Dial builds exactly that.
func New(cl petitionsv1.PetitionsServiceClient, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{cl: cl, log: log}
}

// Close releases the connection. Safe on a client built with New.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// OrgUnitHoldings asks how many open petitions and tasks the unit holds in the commune the context
// carries.
//
// # THE CALLER DECIDES FROM THIS, AND AN ERROR IS NEVER ZERO
//
// Any() true → refuse the delete, naming the counts. An error → refuse the delete too (503,
// retryable when errors.Is(err, ErrPetitionsUnavailable)). The zero value returned beside an error
// means nothing and must not be read.
//
// An id in, two counts out; nothing personal. The id is not logged either — it is not personal
// data, but it is not needed to tell an outage from a misconfiguration, and the code is.
func (c *Client) OrgUnitHoldings(ctx context.Context, orgUnitID string) (OrgUnitHoldings, error) {
	if orgUnitID == "" {
		// A wiring fault in the caller. Refused locally: the server answers INVALID_ARGUMENT for
		// exactly this, and a blank id sent anyway would ask about rows whose column is empty.
		return OrgUnitHoldings{}, fmt.Errorf(
			"petitionsclient: OrgUnitHoldings với mã bộ phận rỗng — bên gọi phải truyền bo_phan.id")
	}

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	resp, err := c.cl.CountOrgUnitHoldings(ctx, &petitionsv1.CountOrgUnitHoldingsRequest{OrgUnitId: orgUnitID})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không đếm được hồ sơ bộ phận đang giữ ở petitions",
			"ma_loi", status.Code(err).String(), "err", err)
		return OrgUnitHoldings{}, wrapCallError("CountOrgUnitHoldings", err)
	}
	return OrgUnitHoldings{
		OpenPetitions: resp.GetOpenPetitions(),
		OpenTasks:     resp.GetOpenTasks(),
	}, nil
}

func wrapCallError(method string, err error) error {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("petitionsclient: %s: %w: %w", method, ErrPetitionsUnavailable, err)
	default:
		return fmt.Errorf("petitionsclient: %s: %w", method, err)
	}
}
