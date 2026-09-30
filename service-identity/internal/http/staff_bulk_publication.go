package http

// POST /api/v1/staff/publications — publishing MANY people to the Zalo Mini App directory in one
// request (user decision 2026-09-30, option A). The route's reasoning is on its mux.Handle in
// routes.go; this file translates HTTP and nothing else.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-identity/internal/app"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// bulkPublicationBodyMax bounds the body. One item is under 100 bytes, so 200 items (the cap) are
// ~20 KiB — past thanCanBoToiDa (16 KiB), which is sized for one person's profile. 64 KiB leaves
// room for whitespace and refuses a payload.
const bulkPublicationBodyMax = 64 << 10

// bulkPublicationIn is the body of POST /api/v1/staff/publications.
type bulkPublicationIn struct {
	Items []bulkPublicationItemIn `json:"items"`
}

// bulkPublicationItemIn is one row.
//
//	id                 the person, as the single route's {id}.
//	consent_confirmed  the administrator confirms, FOR THIS ROW, "đã hỏi ý và người này đồng ý".
//	                   Absent = false = the row is skipped (consent_required) — never published.
//	display_order      explicit position, >= 0. ABSENT OR NULL LEAVES the current position; this
//	                   route turns people on, it does not reset the order of those already on.
type bulkPublicationItemIn struct {
	ID               string `json:"id"`
	ConsentConfirmed bool   `json:"consent_confirmed"`
	DisplayOrder     *int   `json:"display_order,omitempty"`
}

// bulkPublicationOut answers every item, in request order.
type bulkPublicationOut struct {
	Items []bulkPublicationItemOut `json:"items"`
}

// bulkPublicationItemOut is what happened to one row. NO NAME, NO TELEPHONE NUMBER (rule 3): the
// screen already holds the register, and joins on `id`.
//
//	result       "published" — on the Mini App after this request (freshly, or already was);
//	             "skipped"   — nothing written for this row.
//	reason_code  only when skipped: consent_required · staff_locked · staff_not_found — the codes
//	             PUT /api/v1/staff/{id}/publication answers for the same refusals.
type bulkPublicationItemOut struct {
	ID         string `json:"id"`
	Result     string `json:"result"`
	ReasonCode string `json:"reason_code,omitempty"`
}

const (
	bulkResultPublished = "published"
	bulkResultSkipped   = "skipped"
)

// PublishStaffBulk publishes many people to the Mini App directory. POST /api/v1/staff/publications
//
// The actor is built by nguoiThucHienCanBo like every staff write: the staff code p.Ma becomes both
// the audit actor and the consent recorder of every row; an empty p.Ma answers 500 first.
func (h *Handler) PublishStaffBulk(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var body bulkPublicationIn
	r.Body = http.MaxBytesReader(w, r.Body, bulkPublicationBodyMax)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// The decoder's own message is not returned: it would quote the input.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}

	items := make([]app.BulkPublishItem, len(body.Items))
	for i, it := range body.Items {
		items[i] = app.BulkPublishItem{
			ID:               it.ID,
			ConsentConfirmed: it.ConsentConfirmed,
			DisplayOrder:     it.DisplayOrder,
		}
	}
	outcomes, err := h.d.GhiDanhBa.PublishMany(r.Context(), items, actor)
	if err != nil {
		h.traLoiLoiGhiCanBo(w, r, "công khai Mini App hàng loạt", err)
		return
	}

	out := bulkPublicationOut{Items: make([]bulkPublicationItemOut, len(outcomes))}
	for i, o := range outcomes {
		out.Items[i] = bulkPublicationItemOut{ID: o.ID, Result: bulkResultPublished}
		if o.Published {
			continue
		}
		code, known := bulkReasonCode(o.Refusal)
		if !known {
			// The use case promises one of three sentinels. Anything else is a defect; answering
			// "skipped" with an invented code would tell the administrator something false.
			h.traLoiLoiGhiCanBo(w, r, "công khai Mini App hàng loạt", o.Refusal)
			return
		}
		out.Items[i].Result = bulkResultSkipped
		out.Items[i].ReasonCode = code
	}
	vietJSON(w, http.StatusOK, out)
}

// bulkReasonCode maps a per-row refusal onto the code the single route answers for it.
func bulkReasonCode(err error) (string, bool) {
	switch {
	case errors.Is(err, app.ErrChuaXacNhanDongY):
		return "consent_required", true
	case errors.Is(err, app.ErrStaffLocked):
		return "staff_locked", true
	case errors.Is(err, idstore.ErrCanBoKhongTonTai):
		// Another commune's id, a soft-deleted person and an invented id are one answer (rule 4,
		// forbidden #2) — TheoIDDeGhi is scoped and excludes deleted rows.
		return "staff_not_found", true
	}
	return "", false
}
