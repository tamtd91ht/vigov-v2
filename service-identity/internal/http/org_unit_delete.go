package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
)

// orgUnitDeleteIn is the body of DELETE /api/v1/org-units/{id}. OPTIONAL since 10/10/2026 (owner
// decision — the screen drops the reason box): absent, "" or no body = the fixed sentence
// domain.OrgUnitDeleteDefaultReason. Trimmed and bounded by the use case
// (domain.NormalizeOrgUnitDeleteReason), so the rule has one owner.
type orgUnitDeleteIn struct {
	Reason string `json:"reason,omitempty"`
}

// orgUnitHoldingsOut counts what the unit still holds, by kind. EVERY FIELD IS PRINTED, zeros
// included: the screen lists what to move, and a missing field reads as "unknown", not "none".
type orgUnitHoldingsOut struct {
	Staff                 int `json:"staff"`
	ChildUnits            int `json:"child_units"`
	OpenPetitions         int `json:"open_petitions"`
	OpenTasks             int `json:"open_tasks"`
	OpenIncomingDocuments int `json:"open_incoming_documents"`
}

// orgUnitInUseOut is the 409. The three fields of httpx.Error, so a client reading `code` and
// `message` on every error keeps working, plus the counts.
type orgUnitInUseOut struct {
	Code     string             `json:"code"`
	Message  string             `json:"message"`
	TraceID  string             `json:"trace_id"`
	Holdings orgUnitHoldingsOut `json:"holdings"`
}

// DeleteOrgUnit soft-deletes one unit. DELETE /api/v1/org-units/{id}
//
// 204 AND NO BODY, like DELETE /api/v1/staff/{id}: the unit is gone from the list, and the screen
// reloads it.
func (h *Handler) DeleteOrgUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in orgUnitDeleteIn
	if !readOptionalDeleteBody(w, r, thanBoPhanToiDa, &in) {
		return
	}
	// Scoped by the request context: the use case reads the commune from it (rule 1, invariant 4).
	if err := h.d.GhiBoPhan.Remove(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeOrgUnitDeleteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeOrgUnitDeleteError adds the three answers only the delete has, then falls through to the
// mapping every org-chart write shares (404, 400, 500).
func (h *Handler) writeOrgUnitDeleteError(w http.ResponseWriter, r *http.Request, err error) {
	var inUse *app.OrgUnitInUseError
	switch {
	case errors.As(err, &inUse):
		// 409 AND NOT 403: the caller holds `admin.org`; what is refused is the operation against
		// what the unit still holds — the same caller may do it once those are moved (§12.4).
		hd := inUse.Holdings
		vietJSON(w, http.StatusConflict, orgUnitInUseOut{
			Code:    "org_unit_in_use",
			Message: hd.Sentence(),
			Holdings: orgUnitHoldingsOut{
				Staff: hd.Staff, ChildUnits: hd.ChildUnits, OpenPetitions: hd.OpenPetitions,
				OpenTasks: hd.OpenTasks, OpenIncomingDocuments: hd.OpenIncomingDocuments,
			},
		})

	case errors.Is(err, app.ErrOrgUnitHoldingsUnavailable):
		// 503 AND NEVER A DELETE: an unanswered question is not "holds nothing". The wrapped error
		// (gRPC code included) is logged for the operator; the client gets a retry sentence.
		h.d.Log.Warn("xoá bộ phận: không hỏi được phản ánh/văn bản — TỪ CHỐI xoá",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "org_unit_delete_unavailable",
			"Chưa kiểm được hồ sơ bộ phận đang giữ ở phân hệ phản ánh hoặc văn bản, nên chưa xoá. Vui lòng thử lại sau ít phút.", "")

	case errors.Is(err, app.ErrOrgUnitDeleteNotConfigured):
		// 503 WITH ITS OWN CODE AND AN HONEST SENTENCE: retrying will not help, an operator must set
		// PETITIONS_GRPC_ADDR / DOCUMENTS_GRPC_ADDR. Logged at Error for that reason.
		h.d.Log.Error("xoá bộ phận: chưa cấu hình PETITIONS_GRPC_ADDR hoặc DOCUMENTS_GRPC_ADDR",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusServiceUnavailable, "org_unit_delete_not_configured",
			"Chức năng xoá bộ phận chưa được bật trên hệ thống này. Hãy báo quản trị kỹ thuật.", "")

	default:
		h.traLoiLoiGhiBoPhan(w, r, "xoá bộ phận", err)
	}
}
