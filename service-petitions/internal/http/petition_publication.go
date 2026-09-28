package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// publicationIn is the body of PUT /api/v1/citizen-reports/{maTraCuu}/publication.
//
// ONE FIELD. `cong-khai` ("Cho hiện công khai") or `an` ("Ẩn khỏi trang công khai") — the two buttons
// of the requirement's drawer (`FeedbackDetailDrawer.tsx:265-309`). `cho-duyet` is refused: it means
// nobody has decided, and a staff act cannot record that (domain.ErrPublicationStatusInvalid).
//
// NO `note`. The requirement's schema has an optional note stored on the row (`moderation_note`), but
// its screen never sends one and this register has no column for it; who and when are in the audit
// entry.
type publicationIn struct {
	Status string `json:"status"`
}

// SetPublication records a staff moderation decision on the public page.
// PUT /api/v1/citizen-reports/{maTraCuu}/publication
//
// IT MOVES NO LIFECYCLE STATUS and notifies nobody (user decision 28/09/2026; not in ADR 0041's table).
// The answer is the petition as it stands, through traPhieu — so the restricted-field second layer
// and the masking of the reporter are the same as on every other write route.
func (h *Handler) SetPublication(w http.ResponseWriter, r *http.Request) {
	var in publicationIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	after, err := h.d.XuLyPhieu.SetPublication(ctx, r.PathValue("maTraCuu"), in.Status, actor,
		h.coQuyenHanChe(ctx))
	switch {
	case err == nil:
		h.traPhieu(w, r, after)
	case errors.Is(err, domain.ErrPublicationStatusInvalid):
		// Fixed sentences, never err.Error(): the chain carries this service's wrapping and, inside the
		// transaction, the commune id (the argument cacCauTuChoiPhieu makes).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Trạng thái công khai chỉ nhận `cong-khai` (cho hiện công khai) hoặc `an` (ẩn khỏi trang công khai).", "")
	case errors.Is(err, domain.ErrNeverPublic):
		// 409 AND NOT 400: the request is well formed and the caller holds the right; it is THIS record
		// that can never be public. The code is the requirement's own, `never_public`
		// (`service.py:781`). THE SENTENCE IS THE COMMUNE'S "Lời hệ thống" wording of
		// `feedback.never_public` (ADR 0024), the shipped default when it has none or when the wording
		// cannot be read — the 409 and the code do not depend on it.
		httpx.WriteError(w, http.StatusConflict, "never_public",
			h.systemMessage(ctx, domain.KeyFeedbackNeverPublic), "")
	default:
		// 404 (unknown / other commune / soft deleted / restricted without the key), 409 on a lost
		// race, 500 otherwise — the one mapping every write route shares.
		h.traLoiLoiXuLy(w, r, "đặt trạng thái công khai", err)
	}
}
