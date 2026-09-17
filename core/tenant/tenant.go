// Package tenant carries the commune identity for one request.
//
// WHY A PACKAGE AND NOT A PARAMETER: tenant_id is a DATA DIMENSION, not an argument. If a
// business function takes it as a parameter it can be passed wrong, and eventually it will
// be. Here it rides in context.Context from the edge down to the store, and no business
// function ever names it.
//
// The whole point of MustFrom panicking: failing loudly in development beats failing
// silently in production, where silence means serving every commune's data at once.
package tenant

import (
	"context"
	"errors"
	"fmt"
)

// ID is the opaque, immutable identifier of a commune (ULID).
//
// It is deliberately NOT the administrative code, the domain name, or the commune name.
// Vietnam reorganises commune-level units periodically; an identifier carrying meaning would
// force rewriting foreign keys across archival records at the first merger, which the law
// does not permit. See kb/10-decisions/0004-shard-by-tenant.md.
type ID string

func (t ID) String() string { return string(t) }
func (t ID) Valid() bool    { return len(t) == 26 } // ULID length

type ctxKey struct{}

var ErrNoTenant = errors.New("tenant: no commune in context")

// Into returns a context carrying the commune. Called exactly once, at the edge.
func Into(ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From reports the commune, if any. Prefer MustFrom in service code.
func From(ctx context.Context) (ID, bool) {
	id, ok := ctx.Value(ctxKey{}).(ID)
	return id, ok && id.Valid()
}

// ctxKeyDay carries the WHOLE Tenant, not just the id.
//
// HAI KHOÁ, KHÔNG PHẢI MỘT, và lý do đáng nêu: `tenant_id` đến được bằng nhiều đường, còn
// tên xã thì không. Biên HTTP phân giải `Host` nên nó biết cả tên; biên gRPC chỉ nhận
// `tenant_id` trong metadata, và một consumer sự kiện cũng vậy. Nếu ép một khoá duy nhất
// mang cả Tenant thì hai đường sau phải BỊA ra một Tenant rỗng tên — và một tên rỗng đi tới
// màn hình của một cơ quan nhà nước thì tệ hơn hẳn một lỗi.
//
// Nên: `From`/`MustFrom` luôn có (mọi đường đều đặt id), còn `Current` chỉ có sau biên HTTP.
// Chỗ nào cần tên xã thì tự khắc phải chạy sau biên ấy, và trình biên dịch không nói hộ được
// điều đó — `MustCurrent` nói, bằng cách panic.
type ctxKeyDay struct{}

// IntoFull returns a context carrying the whole commune. Called exactly once, at the HTTP edge.
//
// Nó đặt CẢ hai khoá: mọi thứ đọc `From` vẫn chạy y nguyên.
func IntoFull(ctx context.Context, t Tenant) context.Context {
	return context.WithValue(Into(ctx, t.ID), ctxKeyDay{}, t)
}

// Current reports the whole commune, if the HTTP edge put it there.
//
// `false` KHÔNG phải lỗi: trên đường gRPC và trên đường sự kiện, chỉ có `tenant_id` được
// truyền đi, nên ở đó câu trả lời đúng là "không biết tên xã" chứ không phải một tên bịa.
func Current(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(ctxKeyDay{}).(Tenant)
	return t, ok && t.ID.Valid()
}

// MustCurrent returns the whole commune or panics.
//
// Cùng lý do với MustFrom: một handler cần TÊN xã mà không có nó thì hoặc hiện tên rỗng,
// hoặc hiện tên của xã trước đó còn sót trong một biến nào đó. Cả hai đều là một cơ quan nhà
// nước hiển thị sai tên mình, và cả hai đều không có gì đỏ. Dừng lại to tiếng rẻ hơn nhiều.
func MustCurrent(ctx context.Context) Tenant {
	t, ok := Current(ctx)
	if !ok {
		panic("tenant: không có thông tin xã trong context — handler này chạy ngoài biên HTTP")
	}
	return t
}

// MustFrom returns the commune or panics.
//
// Panicking is the point. A query without a commune returns rows from EVERY commune and
// raises no error; that failure is invisible until somebody complains. A panic in a handler
// is recovered into a 500 and shows up immediately in development.
func MustFrom(ctx context.Context) ID {
	id, ok := From(ctx)
	if !ok {
		panic(fmt.Sprintf("tenant: %v — a query would have crossed commune boundaries", ErrNoTenant))
	}
	return id
}

// Directory resolves an incoming Host to a commune.
//
// Implementations must be cached with a short TTL: at 200+ communes this is on the path of
// every single request, and the mapping changes only when a commune is renamed or its domain
// is reassigned. Invalidate on those events, never poll.
type Directory interface {
	// ByHost returns the commune for a Host header.
	// A Host matching nothing must return ok=false — never a fallback commune.
	ByHost(ctx context.Context, host string) (Tenant, bool)
}

// Tenant is the platform's view of a commune. Business services never store this; they hold
// only the ID and read the rest through the platform service when they need to display it.
type Tenant struct {
	ID     ID
	Host   string
	Name   string // display name at this moment in time, not an identifier
	Active bool
}
