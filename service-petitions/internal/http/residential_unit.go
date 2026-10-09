package http

// The residential unit (thôn / tổ dân phố, the glossary's `ResidentialUnit`) on the petition surfaces —
// ADR 0088 §1. The wire name is `residential_unit_id` / `residential_unit_name`, the glossary's word and
// the one GET /api/v1/citizen-report-breakdown already uses; never `hamlet` (half the meaning — a tổ dân
// phố is not a hamlet; kb/00-foundation/ubiquitous-language.md). The list FILTER keeps its older `hamlet`
// spelling: renaming a published query parameter is a separate change.
//
// THE PETITION STORES THE ID; THE NAME IS identity's, READ ON DISPLAY. A retired unit still answers its
// name (ResolveResidentialUnitNames includes them), so an old petition keeps showing where it was.

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
)

// errResidentialUnitNamesNotWired — Deps.ResidentialUnitNames is nil and a row needs a name.
var errResidentialUnitNamesNotWired = errors.New("petitions/http: chưa nối dây tra tên thôn / tổ dân phố")

// residentialUnitNames resolves every non-empty id in ONE batched lookup (skills/load-data-once). No id,
// no call. An error means the lookup did not happen — the caller decides whether that fails the read.
func (h *Handler) residentialUnitNames(ctx context.Context, ids []string) (
	map[string]identityclient.ResidentialUnitName, error) {

	asked := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			asked = append(asked, id)
		}
	}
	if len(asked) == 0 {
		return map[string]identityclient.ResidentialUnitName{}, nil
	}
	if h.d.ResidentialUnitNames == nil {
		return nil, errResidentialUnitNamesNotWired
	}
	return h.d.ResidentialUnitNames.ResidentialUnitNamesInBatches(ctx, asked)
}

// nameOneResidentialUnit is residentialUnitNames for a single response AFTER A COMMITTED WRITE: the act
// happened, so a failed lookup must not turn into an error the officer would retry — the name goes out
// absent (the id is still there) and the failure is logged, the field-label precedent of traPhieu.
func (h *Handler) nameOneResidentialUnit(ctx context.Context, ra *phieuPhanAnhRa) {
	if ra.ResidentialUnitID == "" {
		return
	}
	names, err := h.residentialUnitNames(ctx, []string{ra.ResidentialUnitID})
	if err != nil {
		h.d.Log.Warn("đọc tên thôn / tổ dân phố sau khi ghi: không tra được — trả phiếu không tên thôn",
			"xa", string(tenant.MustFrom(ctx)), "ma_tra_cuu", ra.Code, "err", err)
		return
	}
	ra.ResidentialUnitName = names[ra.ResidentialUnitID].Name
}

// writeResidentialUnitNotOffered is the ONE answer for a picked unit identity does not confirm as an
// active unit of this commune — unknown, retired, removed or another commune's. One code and one
// sentence, so the answer says nothing about which ids exist elsewhere (rule 1); the client's move is
// the same for all: reload the list and pick again. The `field_not_offered` precedent.
func writeResidentialUnitNotOffered(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "residential_unit_not_offered",
		"Thôn, tổ dân phố đã chọn hiện không có trong danh sách của xã. Vui lòng chọn lại thôn, tổ dân phố.", "")
}

// writeResidentialUnitCheckUnavailable — identity could not be asked, so NOTHING was written and no code
// was issued (ADR 0088 stop condition #1: never written unchecked, never dropped). Clears by itself.
func writeResidentialUnitCheckUnavailable(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusServiceUnavailable, "residential_unit_check_unavailable",
		"Chưa kiểm tra được thôn, tổ dân phố đã chọn nên phiếu CHƯA được ghi. Vui lòng thử lại sau ít phút.", "")
}

// writeResidentialUnitNamesUnavailable — a read that shows place names could not get them. Refused
// rather than shown with blank places, which a reader takes as fact (identityclient.ResidentialUnitNames).
func writeResidentialUnitNamesUnavailable(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusServiceUnavailable, "residential_unit_names_unavailable",
		"Chưa tra được tên thôn, tổ dân phố. Vui lòng thử lại sau ít phút.", "")
}
