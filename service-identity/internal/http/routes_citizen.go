package http

// Routes for the CITIZEN surface of the identity service — what the Zalo Mini App reads WITH a citizen
// session. Distinct from routes_cong_dan.go, which despite its name is the PUBLIC surface (no session,
// `?host=`): the two run behind different chains (cmd/server buildCitizenEdge vs dungBienCongKhai), and a
// route on the wrong mux either 404s or serves without the session it needs.
//
// A THIRD Register INTO A THIRD MUX, and a third Deps type, so no staff store and no public read is
// reachable from here (rule 4, invariant 5). Same template as service-petitions' routes_cong_dan.go:
//
//	authz.CitizenOnly()           WHO   — a citizen-channel principal, or 401. No RBAC (rule 5, inv. 6)
//	httpx.XaTuPhien*(…)           WHICH COMMUNE — from the session, or 401 (ADR 0022)
//
// both in the same statement; tools/apidoc refuses a citizen route missing either.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ActiveResidentialUnitLister is the citizen picker's read: the commune's units that are live AND in
// use, in the commune's order — the SAME predicate as ResolveActiveResidentialUnits, so a unit offered
// here is a unit petitions may write. *idstore.ThonToDanPhoStore satisfies it.
//
// NOT the staff list's interface (ThonToDanPho.DanhSach): that one returns out-of-use units, the counts
// and the head of the unit — a person — none of which may reach a citizen.
type ActiveResidentialUnitLister interface {
	ActiveUnits(ctx context.Context) ([]domain.ActiveResidentialUnit, error)
}

// DepsCitizen is everything the citizen routes may touch. Nothing else is reachable from them.
type DepsCitizen struct {
	ResidentialUnits ActiveResidentialUnitLister
	Log              *slog.Logger
}

// MyResidentialUnitsPath is the citizen picker's path, for cmd/server to mount on the citizen chain.
// The route below spells it as a literal because tools/apidoc reads the pattern from the source; the
// chain test in cmd/server requests THIS constant through the real chain, so the two cannot differ
// silently.
const MyResidentialUnitsPath = "/api/v1/my-residential-units"

// RegisterCitizen mounts the citizen routes onto their OWN mux — the one behind the citizen chain.
func RegisterCitizen(mux *http.ServeMux, d DepsCitizen) {
	if d.ResidentialUnits == nil {
		panic("identity/http: thiếu kho thôn / tổ dân phố đang dùng — GET /api/v1/my-residential-units sẽ panic khi người dân mở form phản ánh")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &handlerCitizen{d: d}

	// --- the commune's active residential units, for the petition form's picker (ADR 0088) ---------
	//
	// `my-residential-units`: the citizen-surface counterpart of the staff resource `residential-units`,
	// per kb/00-foundation/ubiquitous-language.md §Tiền tố `my-` and the precedent
	// `my-citizen-report-fields` (a commune catalogue on the citizen surface). A RESOURCE OF ITS OWN, so
	// tools/ingress routes it separately from the staff list, and the two never share a handler. NOT
	// `/api/v1/citizen/…`: ingress groups by the first segment, and `citizen` would become a resource
	// segment one service owns and the next citizen route of another service collides with.
	//
	// OWNER DECISION 09/10/2026 (ADR 0088 "Còn mở" #2): a new citizen route in identity, the owner of
	// `thon_to_dan_pho`.
	//
	// SESSION-ONLY: the commune comes from the citizen session, never from a parameter (rule 1,
	// forbidden #2; rule 4, invariant 2). No query parameter is read at all.
	//
	// XaTuPhienChiXem AND NOT XaTuPhien: the list is the commune's own register of places, the same for
	// every resident, and names nobody — and the petition form it feeds accepts a session without a
	// verified phone (ADR 0080), so that session must see the picker too. The handler takes no identity
	// from the principal and filters by none (ADR 0045 stop condition #6).
	//
	// ONLY `id` AND `name` LEAVE: no head of unit (a person, rule 3), no counts, no code, no type, no
	// `active` (every unit listed is active). Ordered by the commune's rank, then name.
	//
	// RATE LIMIT (rule 13, invariant 7): none declared — the SAME as the other session-scoped citizen
	// reads (petitions' GET my-citizen-reports, my-citizen-report-fields). The route is not
	// unauthenticated: a server-minted citizen session is required. core/ratelimit has no policy for
	// citizen READS, and choosing a number is a rule 13 stop condition for the owner.
	//
	// CACHE: `private, max-age=60` — per session (never a shared cache: the answer depends on the
	// Authorization header, hence also `Vary: Authorization`), short enough that a unit the commune
	// takes out of use leaves the picker within a minute.
	//
	// NO AUDIT ENTRY: the commune's own place register, no personal data, not a cross-commune read
	// (rule 6, invariant 7). NO idem.* DECLARATION: a GET changes no state.
	//
	// NO @consumer: tools/apidoc allows it on Public routes only — CitizenOnly already says who calls.
	//
	// @summary  Danh sách thôn / tổ dân phố ĐANG DÙNG của xã trong phiên công dân, cho ô chọn ở form gửi phản ánh trên Mini App — chỉ mã và tên, theo thứ tự của xã
	// NO @screen: docs/ui-ux/ has no Mini App section for the hamlet picker; naming one would be design
	// intent invented here.
	//
	// 200 WITH `items: []` is a commune that has entered no unit, or has taken every unit out of use.
	//
	// 401 is the three situations every citizen route folds together (ADR 0022): no bearer token · a
	// token that is not usable (unknown, expired, revoked, commune deactivated) · a session with no
	// commune yet. A staff cookie is not a citizen session and answers the same 401. No 403: there is no
	// permission, and the phone is not required here.
	//
	// 500 is a store failure, or a list over idstore.TranDanhSachThonToDanPho — refused, never truncated.
	//
	// 503 is the session registry or the commune check unreachable (httpx.CitizenEdge) — never a 401.
	//
	// @reply    200 myResidentialUnitsOut
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/my-residential-units",
		authz.CitizenOnly()(
			httpx.XaTuPhienChiXem("danh sách thôn / tổ dân phố là sổ địa bàn của xã, như nhau với mọi người dân và không chứa hồ sơ của ai — phiên chưa xác thực số vẫn gửi được phản ánh (ADR 0080) nên vẫn cần ô chọn thôn")(
				http.HandlerFunc(h.ListMyResidentialUnits))))
}

type handlerCitizen struct {
	d DepsCitizen
}

// myResidentialUnitOut is one unit on the citizen's picker. Two fields and no others, on purpose: what
// is not a field here cannot leave, whatever the store reads tomorrow.
type myResidentialUnitOut struct {
	ID   string `json:"id"`   // ULID — what the petition send carries back (`residential_unit_id`)
	Name string `json:"name"` // "Thôn Bình An" — today's name, for display
}

type myResidentialUnitsOut struct {
	Items []myResidentialUnitOut `json:"items"`
}

// ListMyResidentialUnits serves the session commune's active residential units.
// GET /api/v1/my-residential-units
func (h *handlerCitizen) ListMyResidentialUnits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	units, err := h.d.ResidentialUnits.ActiveUnits(ctx)
	if err != nil {
		// The commune id names no person and is what an operator can act on; the wrapped error never
		// reaches the client (rule 3, forbidden #3).
		if errors.Is(err, idstore.ErrQuaNhieuThonToDanPho) {
			h.d.Log.ErrorContext(ctx, "danh sách thôn/tổ dân phố cho công dân vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhSachThonToDanPho)
		} else {
			h.d.Log.ErrorContext(ctx, "danh sách thôn/tổ dân phố cho công dân: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := myResidentialUnitsOut{Items: make([]myResidentialUnitOut, 0, len(units))}
	for _, u := range units {
		if u.Name == "" {
			// `ten` is NOT NULL; an empty one is a broken row. Not offered — the same refusal
			// ResolveActiveResidentialUnits makes, so the picker and the write still agree.
			continue
		}
		out.Items = append(out.Items, myResidentialUnitOut{ID: u.ID, Name: u.Name})
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Header().Add("Vary", "Authorization")
	vietJSON(w, http.StatusOK, out)
}
