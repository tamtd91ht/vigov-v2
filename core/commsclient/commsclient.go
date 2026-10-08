// Package commsclient is how another service asks comms to do something over gRPC.
//
// Today there is one question and its callers are the automation runners (ADR 0058 §3): deliver
// staff notices into the header-bell inbox (DeliverStaffNotifications). The contract — the key
// recipes, what a title may say, the limits — is stated once, in proto/vigov/comms/v1/comms.proto,
// and not repeated here.
//
// Same shape as core/documentsclient, for the same reasons: grpc.NewClient connects lazily, the
// caller key and the commune travel as metadata set by the interceptors (rule 1, invariant 8), and
// the channel is plaintext by the owner's decision of 2026-09-28 (the `@security-exception` in Dial).
//
// NEVER LOG A REQUEST OR A NOTICE. The generated String() prints title and body in full; the
// wrappers here log counts and status codes only.
package commsclient

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
	"google.golang.org/protobuf/types/known/timestamppb"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
)

// CallTimeout bounds one call. Longer than the 3 s of the lookup clients: one call writes up to 100
// notices and one audit entry in one transaction.
const CallTimeout = 10 * time.Second

// MaxNoticesPerCall and MaxRecipientsPerCall are the contract's bounds (DeliverStaffNotificationsRequest).
// Deliver refuses more rather than splitting silently: the caller pages, and knows it did.
const (
	MaxNoticesPerCall    = 100
	MaxRecipientsPerCall = 1000
)

// ErrCommsUnavailable marks "comms did not answer, asking again may work" (UNAVAILABLE or
// DEADLINE_EXCEEDED). Nothing was written; retrying the SAME request is safe (the keys).
var ErrCommsUnavailable = errors.New("commsclient: comms tạm thời không trả lời — thử lại sau")

// Notice is one staff notice — the Go shape of commsv1.StaffNotification.
type Notice struct {
	IdempotencyKey string
	Kind           commsv1.StaffNotificationKind
	RecipientMa    []string
	Title          string
	Body           string
	Link           string
	// DueSoonItems is set on a due-soon notice only (StaffNotification.due_soon_items): the records
	// Body counts, each with its STORED deadline, so comms can apply the commune's Zalo lead to the
	// Zalo copy. Empty is the old producer's shape — comms then sends the bell's text unchanged.
	DueSoonItems []DueSoonItem
}

// DueSoonItem is one record of a due-soon notice: its business code and its deadline as stored. The
// producer copies the deadline; it never computes one here (rule 10, invariant 2).
type DueSoonItem struct {
	Code     string
	Deadline time.Time
}

// Delivery is comms' answer for one notice, mapped by key.
type Delivery struct {
	Created          uint32
	AlreadyDelivered uint32
}

// Client asks comms over gRPC. No cache.
type Client struct {
	cl   commsv1.CommsServiceClient
	conn *grpc.ClientConn // nil when the client was injected through New
	log  *slog.Logger
}

// Dial opens the connection to comms — the shape of documentsclient.Dial, for its reasons.
func Dial(addr string, khoa secret.Secret, log *slog.Logger) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("commsclient: thiếu địa chỉ COMMS_GRPC_ADDR")
	}
	conn, err := grpc.NewClient(addr,
		// @security-exception: service-to-service gRPC has no TLS yet — same channel as identityclient/documentsclient (tools/security_debt.json, expires 2026-12-28), converted together when internal TLS lands; carries staff codes and notice sentences that by contract hold no citizen personal data — but ALSO the deployment-wide GRPC_CALLER_KEY (x-vigov-caller-key metadata) and the commune id (x-tenant-id), both readable by anyone on the cluster network until TLS lands
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpcx.UnaryClientCallerAuth(khoa),
			grpcx.UnaryClientInterceptor(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("commsclient: mở kết nối tới %s: %w", addr, err)
	}
	c := New(commsv1.NewCommsServiceClient(conn), log)
	c.conn = conn
	return c, nil
}

// New wraps a generated client. The connection under it MUST carry grpcx.UnaryClientCallerAuth
// then grpcx.UnaryClientInterceptor (rule 1, invariant 8). Dial builds exactly that.
func New(cl commsv1.CommsServiceClient, log *slog.Logger) *Client {
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

// DeliverStaffNotifications delivers one page of notices into the bell inbox of staff of the
// commune the context carries, and returns comms' answer per idempotency key.
//
// REFUSED LOCALLY, before the wire: an empty page, more than MaxNoticesPerCall notices or
// MaxRecipientsPerCall recipients, and a key requested twice. Comms would refuse each of them with
// INVALID_ARGUMENT and write nothing; refusing here names the cause instead of a status code.
//
// errors.Is(err, ErrCommsUnavailable) → retry later with the same notices; anything else → the call
// did not happen or the contract broke. The map is nil beside an error.
func (c *Client) DeliverStaffNotifications(ctx context.Context, notices []Notice) (map[string]Delivery, error) {
	if len(notices) == 0 {
		return nil, errors.New("commsclient: DeliverStaffNotifications không có thông báo nào")
	}
	if len(notices) > MaxNoticesPerCall {
		return nil, fmt.Errorf("commsclient: %d thông báo trong một lần gọi, tối đa %d — bên gọi phải chia trang",
			len(notices), MaxNoticesPerCall)
	}
	req := &commsv1.DeliverStaffNotificationsRequest{Notifications: make([]*commsv1.StaffNotification, 0, len(notices))}
	seen := make(map[string]bool, len(notices))
	recipients := 0
	for _, n := range notices {
		if seen[n.IdempotencyKey] {
			return nil, errors.New("commsclient: một khoá chống trùng xuất hiện hai lần trong một lần gọi")
		}
		seen[n.IdempotencyKey] = true
		recipients += len(n.RecipientMa)
		req.Notifications = append(req.Notifications, &commsv1.StaffNotification{
			IdempotencyKey: n.IdempotencyKey,
			Kind:           n.Kind,
			RecipientMa:    n.RecipientMa,
			Title:          n.Title,
			Body:           n.Body,
			Link:           n.Link,
			DueSoonItems:   dueSoonItemsToWire(n.DueSoonItems),
		})
	}
	if recipients > MaxRecipientsPerCall {
		return nil, fmt.Errorf("commsclient: %d người nhận trong một lần gọi, tối đa %d — bên gọi phải chia trang",
			recipients, MaxRecipientsPerCall)
	}

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	resp, err := c.cl.DeliverStaffNotifications(ctx, req)
	if err != nil {
		// Counts and the code only — never the request (its String() prints every title and body).
		c.log.WarnContext(ctx, "CẢNH BÁO: comms không nhận thông báo cán bộ",
			"ma_loi", status.Code(err).String(), "so_thong_bao", len(notices), "so_nguoi_nhan", recipients)
		return nil, wrapCallError("DeliverStaffNotifications", err)
	}

	out := make(map[string]Delivery, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		if !seen[it.GetIdempotencyKey()] {
			// An answer about a key nobody sent: the reply is about some other request.
			return nil, errors.New("commsclient: DeliverStaffNotifications trả về khoá không được gửi — lỗi hợp đồng")
		}
		out[it.GetIdempotencyKey()] = Delivery{Created: it.GetCreated(), AlreadyDelivered: it.GetAlreadyDelivered()}
	}
	if len(out) != len(notices) {
		// The contract answers one item per notice. Fewer is not "some were skipped" — it is a
		// reply the caller cannot count, so it is refused rather than read as zero created.
		return nil, fmt.Errorf("commsclient: DeliverStaffNotifications trả %d mục cho %d thông báo — lỗi hợp đồng",
			len(out), len(notices))
	}
	return out, nil
}

// dueSoonItemsToWire maps items one to one, in the caller's order. Nil for none, so a notice without
// items is byte for byte the request an older producer sent.
func dueSoonItemsToWire(items []DueSoonItem) []*commsv1.DueSoonItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]*commsv1.DueSoonItem, 0, len(items))
	for _, it := range items {
		out = append(out, &commsv1.DueSoonItem{Code: it.Code, Deadline: timestamppb.New(it.Deadline)})
	}
	return out
}

func wrapCallError(method string, err error) error {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("commsclient: %s: %w: %w", method, ErrCommsUnavailable, err)
	default:
		return fmt.Errorf("commsclient: %s: %w", method, err)
	}
}
