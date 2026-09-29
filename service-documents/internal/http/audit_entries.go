package http

import (
	"context"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
)

// AuditLogReader reads THIS service's own `audit_log` and records the read (ADR 0054).
// *audit.Log satisfies it; declared here so the route's refusals are testable without PostgreSQL.
type AuditLogReader interface {
	Read(ctx context.Context, reader audit.Actor, q audit.Query) (page.Result[audit.EntryView], error)
}

// ListAuditEntries serves GET /api/v1/documents-audit-entries.
//
// THE QUERY PARAMETERS ARE NAMED HERE, NOT IN core/audit: tools/apidoc reads them out of the
// handler's own package, and a name read only inside core would be missing from the contract the
// admin web builds against. Validation, the order and the read's own audit entry are core/audit's.
//
// NO `tenant_id` IS READ FROM THE QUERY — the commune is the context's, fixed from Host (rule 1).
func (h *Handler) ListAuditEntries(w http.ResponseWriter, r *http.Request) {
	reader, ok := actorFrom(r)
	if !ok {
		h.d.Log.Error("nhật ký hệ thống: không dựng được người đọc",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	q := r.URL.Query()
	res, err := h.d.AuditLog.Read(r.Context(), reader, audit.Query{
		From:    q.Get("from"),
		To:      q.Get("to"),
		Actor:   q.Get("actor"),
		Action:  q.Get("action"),
		Subject: q.Get("subject"),
		Limit:   q.Get("limit"),
		Cursor:  q.Get("cursor"),
	})
	if err != nil {
		status, code, msg := audit.HTTPError(err)
		if status == http.StatusInternalServerError {
			h.d.Log.Error("nhật ký hệ thống: lỗi hệ thống",
				"xa", string(tenant.MustFrom(r.Context())), "err", err)
		}
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	writeJSON(w, http.StatusOK, res)
}
