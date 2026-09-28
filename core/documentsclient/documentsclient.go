// Package documentsclient is how another service asks documents a question over gRPC.
//
// Today there is one question and one caller: identity, before it soft-deletes an org unit, asks
// how many open incoming documents that unit still holds (CountOrgUnitHoldings — the predicate is
// stated once, in proto/vigov/documents/v1/documents.proto, and not repeated here).
//
// Same shape as core/petitionsclient, for the same reasons, including the two read there first:
// the new identity → documents edge is not a startup cycle (grpc.NewClient connects lazily), and
// there is NO Dial yet because a new plaintext channel is a rule 13 STOP condition for the owner.
package documentsclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
)

// CallTimeout bounds one call — the same number as every other inter-service client.
const CallTimeout = 3 * time.Second

// ErrDocumentsUnavailable marks "documents did not answer, asking again may work" (UNAVAILABLE or
// DEADLINE_EXCEEDED). IT IS NEVER AN ANSWER — never "holds nothing".
var ErrDocumentsUnavailable = errors.New("documentsclient: documents tạm thời không trả lời — thử lại sau")

// OrgUnitHoldings is what one org unit still holds open in documents.
type OrgUnitHoldings struct {
	OpenIncomingDocuments uint32
}

// Any reports whether the unit holds anything open here. A field added to the contract is added to
// the struct AND to this sum in the same change: a kind not read is a kind that reads as zero.
func (h OrgUnitHoldings) Any() bool {
	return h.OpenIncomingDocuments > 0
}

// Client asks documents over gRPC. No cache.
type Client struct {
	cl  documentsv1.DocumentsServiceClient
	log *slog.Logger
}

// New wraps a generated client. The connection under it MUST carry grpcx.UnaryClientCallerAuth
// then grpcx.UnaryClientInterceptor (rule 1, invariant 8).
func New(cl documentsv1.DocumentsServiceClient, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{cl: cl, log: log}
}

// OrgUnitHoldings asks how many open incoming documents the unit holds in the commune the context
// carries. Any() true → refuse the delete; an error → refuse the delete too (503, retryable when
// errors.Is(err, ErrDocumentsUnavailable)). The zero value beside an error means nothing.
func (c *Client) OrgUnitHoldings(ctx context.Context, orgUnitID string) (OrgUnitHoldings, error) {
	if orgUnitID == "" {
		return OrgUnitHoldings{}, fmt.Errorf(
			"documentsclient: OrgUnitHoldings với mã bộ phận rỗng — bên gọi phải truyền bo_phan.id")
	}

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	resp, err := c.cl.CountOrgUnitHoldings(ctx, &documentsv1.CountOrgUnitHoldingsRequest{OrgUnitId: orgUnitID})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không đếm được hồ sơ bộ phận đang giữ ở documents",
			"ma_loi", status.Code(err).String(), "err", err)
		return OrgUnitHoldings{}, wrapCallError("CountOrgUnitHoldings", err)
	}
	return OrgUnitHoldings{OpenIncomingDocuments: resp.GetOpenIncomingDocuments()}, nil
}

func wrapCallError(method string, err error) error {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("documentsclient: %s: %w: %w", method, ErrDocumentsUnavailable, err)
	default:
		return fmt.Errorf("documentsclient: %s: %w", method, err)
	}
}
