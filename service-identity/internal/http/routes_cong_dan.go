package http

// Routes for the CITIZEN surface of the identity service.
//
// A SECOND Register INTO A SECOND MUX, the shape service-petitions/internal/http/routes_cong_dan.go
// set: the citizen surface runs behind a different edge chain (commune from the SESSION, not from
// `Host` — ADR 0022), and it takes a different Deps type, so no staff store is reachable from here
// (rule 4, invariant 5).
//
// Every citizen route declares BOTH axes in the same statement — authz.CitizenOnly() for WHO, and
// httpx.XaTuPhien() / XaTuPhienChiXem(reason) / KhongThuocXa(reason) for WHICH COMMUNE. tools/apidoc
// refuses a citizen route missing either.

import (
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
)

// DepsCongDan is everything the citizen routes may touch. Nothing else is reachable from them.
type DepsCongDan struct {
	// Xa is the platform registry read behind GET /api/v1/communes. Registry METADATA only (ADR
	// 0003) — the one thing a KhongThuocXa route may reach (core/httpx.KhongThuocXa, second wall).
	Xa  TraXaTheoHost
	Log *slog.Logger
}

// MauDanhMucXa is the collection the citizen chain serves in this service. Exported because
// cmd/server must register the SAME path on the outer mux to route it to the citizen chain; two
// spellings of one path is a route that silently falls to the staff chain and 404s.
//
// The route below spells it as a LITERAL because tools/apidoc reads the pattern from the source;
// routes_cong_dan_test.go asserts the literal and this constant agree.
const MauDanhMucXa = "/api/v1/communes"

// RegisterCongDan mounts the citizen routes onto their OWN mux — the one behind the citizen chain.
func RegisterCongDan(mux *http.ServeMux, d DepsCongDan) {
	if d.Xa == nil {
		panic("identity/http: thiếu kho tra xã theo tên miền — GET /api/v1/communes sẽ panic khi có người gọi")
	}
	h := newHandlerCongDan(d)

	// --- the commune a QR's domain belongs to, before the citizen confirms it -------------------
	//
	// `communes` IS THE SETTLED NOUN (kb/00-foundation/ubiquitous-language.md §Kênh công dân: "Danh
	// mục xã cho Mini App", class KhongThuocXa). `?host=` IS A FILTER ON IT and is NOT in that table
	// yet — reported as a finding rather than written into kb/ from here.
	//
	// KhongThuocXa, NOT XaTuPhienChiXem: the commune asked about is, by construction, NOT the
	// session's — the citizen is deciding whether to enter it. Taking the session's commune would
	// answer the wrong question; there is no commune in the context and none is needed, because the
	// only thing reachable is the platform registry.
	//
	// A SESSION WITHOUT A VERIFIED PHONE IS ACCEPTED, and that is ADR 0045 applied, not waived: such a
	// session may VIEW, and this route only views public registry metadata. KhongThuocXa never checks
	// the phone, and authz.CitizenOnly checks only that a citizen session exists. Nothing here filters
	// by citizen identity, so an empty CitizenID has nothing to widen.
	//
	// WHY A SESSION AT ALL for public metadata: every Mini App launch opens one through the bridge
	// before any screen, so requiring it costs the app nothing, and it keeps this lookup off the
	// unauthenticated internet — a route anyone can call is a free domain-to-commune oracle. Public()
	// would need its own reason and an owner's decision (rule 5, stop condition #2); none was asked for.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Tra xã theo tên miền trong mã QR, để Mini App hỏi "Làm việc với xã X?" trước khi công dân xác nhận — không trả mã xã
	// NO @screen: docs/ui-ux/ has no section for the Mini App's QR confirmation screen, and naming
	// one that does not describe it would be design intent invented here.
	//
	// 200 carries ZERO or ONE commune. `items: []` is the ONE answer for a domain no commune holds, a
	// domain reserved for the platform, and a commune that is no longer active — identical bytes.
	//
	// 400 is `host` missing, repeated, or not a bare lowercase hostname. The platform is not asked.
	//
	// 401 is no usable citizen session. There is no 403: citizens hold no permissions.
	//
	// 503 is the platform registry unreachable — never an empty list, never a remembered commune.
	//
	// @reply    200 danhMucXaRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/communes",
		authz.CitizenOnly()(
			httpx.KhongThuocXa("xác nhận xã trước khi vào: công dân quét QR mang tên miền của một xã CHƯA thuộc phiên, tuyến chỉ đọc tên và tỉnh của xã ấy từ sổ đăng ký nền tảng")(
				http.HandlerFunc(h.DanhMucXa))))
}
