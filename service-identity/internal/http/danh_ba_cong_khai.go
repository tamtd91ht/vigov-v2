package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// GET /api/v1/commune-staff?host= — the commune's staff directory as published on the Zalo Mini App
// (docs/ui-ux/12-danh-ba-can-bo.md:117; owner decision 2026-09-27: public, no sign-in).

// DocDanhBaCongKhai is the one store read behind the route. *idstore.CanBoStore satisfies it. The
// commune comes from the CONTEXT (rule 1, invariant 4), put there by DanhBaCongKhai below.
type DocDanhBaCongKhai interface {
	DanhBaCongKhai(ctx context.Context) ([]domain.CanBoCongKhai, error)
}

// danhBaCongKhaiRa is an OBJECT, not a bare array — the shape of every list in this service. NO
// next_cursor, NO has_more: the whole published directory or a refusal (idstore.TranDanhBaCongKhai).
type danhBaCongKhaiRa struct {
	Items []canBoCongKhaiRa `json:"items"`
}

// canBoCongKhaiRa is one published person on the wire.
//
// THE JSON NAMES ARE canBoTomTat's where the meaning is the same (`full_name`, `position`, `phone` for
// the office line, `mobile` for the personal one, `has_zalo`), so the web and the Mini App read one
// vocabulary. `department_name` is new: the register sends `department_id` and resolves it on screen,
// and a public reader has no org chart to resolve it against.
//
// WHAT IS ABSENT IS THE CONTRACT (owner decision 2026-09-27): no `id`, no `code`, no `email`, no
// `has_account` / `active`, no `published` / `consent_recorded_at` / `display_order`. The test asserts
// the exact key set.
type canBoCongKhaiRa struct {
	FullName       string `json:"full_name"`
	Position       string `json:"position"`
	DepartmentName string `json:"department_name"` // "" when the person sits in no unit
	Phone          string `json:"phone"`           // office line — duty information (#16)
	Mobile         string `json:"mobile"`          // personal mobile — published under #12 consent

	// HasZalo describes Mobile (migration 0010 §1) and is sent only WITH a mobile: "Có Zalo" under an
	// empty number is a statement about a number nobody published.
	HasZalo bool `json:"has_zalo"`
}

// DanhBaCongKhai serves the published directory of the commune a domain belongs to.
//
// THE COMMUNE: `host` is validated (400 before any RPC), resolved by the PLATFORM (xaTheoHost), and
// only then put into the context with tenant.Into — the same place the staff edge puts the commune it
// resolved from `Host`. The store reads it from there; no tenant_id is accepted from the request.
//
// UNKNOWN, RESERVED OR INACTIVE DOMAIN → 200 `{"items": []}`, byte-identical to an active commune that
// has published nobody. A different answer would make this route a probe for which domains are
// communes — the same reason GET /api/v1/communes folds them.
//
// NOT MASKED, AND THAT IS THE DECISION, not an oversight of rule 3 invariant 3: each number here was
// published by an administrator, per person, with that person's recorded consent (#12), so that a
// citizen can ring it. Masking it would defeat the consented act; the consent is the "explicit
// permission" the invariant names.
//
// NO AUDIT ENTRY: nothing is written, and rule 6 invariant 7 audits reading FULL personal data behind a
// permission or ACROSS communes. This reads only what the commune already chose to publish, inside the
// one commune the host resolved to. NOTHING PERSONAL IS LOGGED on any path (rule 3): the logs name the
// commune id and the host, never a row.
func (h *HandlerCongKhai) DanhBaCongKhai(w http.ResponseWriter, r *http.Request) {
	gia := r.URL.Query()["host"]
	if len(gia) != 1 || !domain.HopLeTenMienXa(gia[0]) {
		viet400Host(w)
		return
	}

	xa, co, ok := h.xaTheoHost(w, r, gia[0], "danh bạ công khai")
	if !ok {
		return
	}
	ra := danhBaCongKhaiRa{Items: []canBoCongKhaiRa{}} // `[]`, never `null`
	if !co {
		vietJSON(w, http.StatusOK, ra)
		return
	}

	ctx := tenant.Into(r.Context(), xa.ID)
	ds, err := h.d.DanhBa.DanhBaCongKhai(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuCanBoCongKhai) {
			h.d.Log.ErrorContext(ctx, "danh bạ công khai vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(xa.ID), "tran", idstore.TranDanhBaCongKhai)
		} else {
			// The wrapped error carries the store failure, never a name or a number, and never reaches
			// the client (rule 3, forbidden #3).
			h.d.Log.ErrorContext(ctx, "danh bạ công khai: lỗi hệ thống", "xa", string(xa.ID), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	for _, cb := range ds {
		ra.Items = append(ra.Items, canBoCongKhaiRa{
			FullName:       cb.HoTen,
			Position:       cb.ChucVu,
			DepartmentName: cb.TenBoPhan,
			Phone:          cb.DienThoaiCoQuan,
			Mobile:         cb.DiDongCaNhan,
			HasZalo:        cb.CoZalo && cb.DiDongCaNhan != "",
		})
	}
	vietJSON(w, http.StatusOK, ra)
}
