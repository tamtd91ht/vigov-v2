package http

// THE SOFT DELETE OF A COMMUNE — owner only (ADR 0048 §Sửa đổi 13/10/2026 #2, #7, #8; migration 0026).
//
// POST /api/v1/communes/{id}/deletion, not DELETE /api/v1/communes/{id}: the act is a nominalised
// sub-resource carrying a confirmation and a reason (skills/rest-api-design §3), and the commune row is
// NOT removed — it is marked, and stays what every archival record of the commune points at (rule 7).
// It also keeps TestNoDomainRemovalOrRepoint's guard meaningful: no DELETE on the registry exists.
//
// EVERY CHECK IS THE SERVER'S, in one transaction under the row lock (store.RegistryWriter.DeleteCommune):
// the console's own checks are a convenience, never the gate (rule 5 forbidden #1).

import (
	"errors"
	"net/http"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// communeDeletionBody — the two layers the owner decided are confirm_domain (one of the commune's
// domains, re-typed) and the store's 7-day wait; reason is the trail's.
type communeDeletionBody struct {
	ConfirmDomain string `json:"confirm_domain"`
	Reason        string `json:"reason"`
}

// communeDeletionView is what the deletion wrote. Not communeDetailView: the commune is no longer
// readable through GET /communes/{id} the moment this answers.
type communeDeletionView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DeletedAt time.Time `json:"deleted_at"`
	// DeletedBy is the operator's VH- code.
	DeletedBy string `json:"deleted_by"`
}

// communeDeletionRefusalView is the 409 body: httpx.Error's three fields, plus — for
// commune_inactive_too_recent only — WHEN the deletion becomes possible, so the console can show the
// date instead of a bare "not yet". null for commune_active.
type communeDeletionRefusalView struct {
	Code               string     `json:"code"`
	Message            string     `json:"message"`
	TraceID            string     `json:"trace_id"`
	EarliestDeletionAt *time.Time `json:"earliest_deletion_at"`
}

const (
	msgCommuneActive = "Xã đang hoạt động. Chỉ xoá được xã đã ngừng hoạt động liên tục đủ 7 ngày."
	// The mismatch message names NO domain: the refusal must not become a way to read them.
	msgConfirmDomainMismatch = "Tên miền xác nhận không khớp với tên miền nào của xã."
)

// vietnamTime renders the earliest-deletion moment for a person. A fixed offset, not
// time.LoadLocation: Viet Nam has no daylight saving, and a zone database missing from the image would
// otherwise turn a refusal into a 500.
var vietnamTime = time.FixedZone("ICT", 7*60*60)

func (h *operatorHandlers) deleteCommune(w http.ResponseWriter, r *http.Request) {
	ctx, _, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b communeDeletionBody
	if !decodeBody(w, r, &b) {
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	del, hosts, err := h.d.Writer.DeleteCommune(ctx, b.ConfirmDomain, reason, actorOf(r))
	if err != nil {
		h.writeDeletionError(w, r, err)
		return
	}
	// The hosts stop resolving at once in this process; other services keep a cached positive answer
	// for up to one TENANT_CACHE_TTL — but the commune was already inactive for 7 days, so every one of
	// them has been refusing it since.
	for _, host := range hosts {
		h.d.Forget(host)
	}
	writeJSON(w, http.StatusCreated, communeDeletionView{ID: del.ID, Name: del.Name,
		DeletedAt: del.DeletedAt.UTC(), DeletedBy: del.DeletedBy})
}

func (h *operatorHandlers) writeDeletionError(w http.ResponseWriter, r *http.Request, err error) {
	var tooRecent *store.InactiveTooRecentError
	switch {
	case errors.As(err, &tooRecent):
		at := tooRecent.EarliestAt.UTC()
		writeJSON(w, http.StatusConflict, communeDeletionRefusalView{
			Code: "commune_inactive_too_recent",
			Message: "Xã chưa ngừng hoạt động đủ 7 ngày. Có thể xoá từ " +
				at.In(vietnamTime).Format("15:04 ngày 02/01/2006") + " (giờ Việt Nam).",
			EarliestDeletionAt: &at,
		})
	case errors.Is(err, store.ErrCommuneActive):
		writeJSON(w, http.StatusConflict, communeDeletionRefusalView{Code: "commune_active", Message: msgCommuneActive})
	case errors.Is(err, domain.ErrDeletionConfirmMismatch):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "confirm_domain_mismatch", msgConfirmDomainMismatch, "")
	default:
		h.writeRegistryError(w, r, err)
	}
}
