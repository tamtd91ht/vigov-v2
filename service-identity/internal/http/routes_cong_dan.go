package http

// Routes for the PUBLIC surface of the identity service — what the Zalo Mini App reads with no
// session at all (owner decision 2026-09-27).
//
// THE FILE NAME IS HISTORICAL: until 2026-09-27 this held the CITIZEN surface (authz.CitizenOnly +
// httpx.KhongThuocXa). The owner then made both routes here Public, and a Public route cannot live
// behind httpx.CitizenEdge — that edge refuses (500) any successful answer from a route that declared
// no commune class, and tools/apidoc forbids a commune class on a non-citizen route. So this surface
// now runs behind its own PUBLIC chain (cmd/server dungBienCongKhai): CORS, header stripping,
// recovery, and no session layer.
//
// A SECOND Register INTO A SECOND MUX, and a different Deps type, so no staff store is reachable from
// here (the arrangement rule 4, invariant 5 asks of citizen routes).
//
// THE COMMUNE IS NEVER ON THIS CHAIN. There is no `Host` to resolve (the Mini App calls the reserved
// API host, ADR 0046) and no session to read it from. A route that needs one names the commune's
// DOMAIN in `?host=`, and the handler resolves it through the platform registry, server-side
// (HandlerCongKhai.xaTheoHost). No tenant_id is ever read from the request (rule 1, forbidden #2).

import (
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
)

// DepsCongKhai is everything the public routes may touch. Nothing else is reachable from them.
type DepsCongKhai struct {
	// Xa is the platform registry read that turns `?host=` into a commune. Registry METADATA only
	// (ADR 0003).
	Xa TraXaTheoHost

	// DanhBa is the published-directory read, and the ONLY business read on this surface. It returns
	// domain.CanBoCongKhai, which has no field for anything the owner did not allow out.
	DanhBa DocDanhBaCongKhai

	Log *slog.Logger
}

// MauDanhMucXa and MauDanhBaCongKhai are the collections the public chain serves. Exported because
// cmd/server must register the SAME paths on the outer mux to route them to the public chain; two
// spellings of one path is a route that silently falls to the staff chain and 404s on the reserved
// API host.
//
// The routes below spell them as LITERALS because tools/apidoc reads the pattern from the source;
// danh_muc_xa_test.go and danh_ba_cong_khai_test.go assert the literal and the constant agree.
const (
	MauDanhMucXa      = "/api/v1/communes"
	MauDanhBaCongKhai = "/api/v1/commune-staff"
)

// RegisterCongKhai mounts the public routes onto their OWN mux — the one behind the public chain.
func RegisterCongKhai(mux *http.ServeMux, d DepsCongKhai) {
	if d.Xa == nil {
		panic("identity/http: thiếu kho tra xã theo tên miền — các tuyến công khai sẽ panic khi có người gọi")
	}
	if d.DanhBa == nil {
		panic("identity/http: thiếu kho danh bạ công khai — GET /api/v1/commune-staff sẽ panic khi có người gọi")
	}
	h := newHandlerCongKhai(d)

	// --- the commune a QR's domain belongs to, before the citizen confirms it -------------------
	//
	// `communes` IS THE SETTLED NOUN (kb/00-foundation/ubiquitous-language.md §Kênh công dân: "Danh
	// mục xã cho Mini App"). `?host=` IS A FILTER ON IT.
	//
	// PUBLIC SINCE 2026-09-27 (owner decision; it was CitizenOnly + KhongThuocXa in 2f075b9). The
	// confirmation screen must name the commune before any session exists. What that gives away is
	// small and already public: a commune's name and province for a domain the caller already holds —
	// the same fact the commune's own sign-in page prints at GET /api/v1/communes/current. No id, no
	// host, no activity state; unknown, reserved and inactive answer identically.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Tra xã theo tên miền trong mã QR, để Mini App hỏi "Làm việc với xã X?" trước khi có phiên nào — không trả mã xã
	// @consumer citizen-app
	// NO @screen: docs/ui-ux/ has no section for the Mini App's QR confirmation screen, and naming
	// one that does not describe it would be design intent invented here.
	//
	// 200 carries ZERO or ONE commune. `items: []` is the ONE answer for a domain no commune holds, a
	// domain reserved for the platform, and a commune that is no longer active — identical bytes.
	//
	// 400 is `host` missing, repeated, or not a bare lowercase hostname. The platform is not asked.
	//
	// 503 is the platform registry unreachable — never an empty list, never a remembered commune.
	//
	// @reply    200 danhMucXaRa
	// @reply    400 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/communes",
		authz.Public("Mini App hỏi \"Làm việc với xã X?\" TRƯỚC khi có phiên nào: tra tên miền in trên mã QR ra tên và tỉnh của xã — siêu dữ liệu công khai mà trang đăng nhập của chính xã ấy vốn đã hiện; không trả mã xã, không trả tên miền")(
			http.HandlerFunc(h.DanhMucXa)))

	// --- the commune's published staff directory ------------------------------------------------
	//
	// `commune-staff` — the owner's noun (2026-09-27). DISTINCT from `staff` (the register, admin.user)
	// and `staff-directory` (the assignee picker, AnyAuthenticated): three audiences, three nouns, and
	// three column lists that must never be merged.
	//
	// PUBLIC (owner decision 2026-09-27, following docs/ui-ux/12-danh-ba-can-bo.md:117 "công khai"). The
	// reason is the channel: a citizen opening the Mini App to ring the commune office has, and needs,
	// no account. What makes that safe is not the route but the ROW: only people an administrator
	// published one by one with their recorded consent (#12), not locked, not soft-deleted — enforced in
	// the store predicate and, for the consent, again by migration 0010's CHECKs.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Danh bạ cán bộ xã đã công khai trên Zalo Mini App, theo tên miền của xã — chỉ người đã đồng ý công khai
	// @screen   12-danh-ba-can-bo §8
	// @consumer citizen-app
	//
	// 200 is the whole published directory, ordered by display order then unit name. `items: []` for a
	// commune that published nobody AND for a domain no active commune holds — identical bytes.
	//
	// 400 is `host` missing, repeated, or not a bare lowercase hostname. The platform is not asked.
	//
	// 500 is a store failure, or a directory over idstore.TranDanhBaCongKhai — refused, never truncated.
	//
	// 503 is the platform registry unreachable.
	//
	// @reply    200 danhBaCongKhaiRa
	// @reply    400 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-staff",
		authz.Public("danh bạ cán bộ xã công bố cho người dân trên Zalo Mini App (docs/ui-ux/12-danh-ba-can-bo.md:117): người dân gọi điện cho xã không cần tài khoản; chỉ trả người được quản trị công khai từng người kèm đồng ý đã ghi nhận (#12)")(
			http.HandlerFunc(h.DanhBaCongKhai)))
}
