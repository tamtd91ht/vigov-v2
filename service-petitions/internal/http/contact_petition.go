package http

// The two READ routes of the petition ↔ citizen-directory link (ADR 0102). The link is DERIVED on every
// call from (tenant_id, sender_phone_normalized) — nothing here stores, caches or returns a number:
//
//	GET /api/v1/citizen-reports/{maTraCuu}/directory-match          "Đồng bộ danh bạ" on a petition
//	GET /api/v1/citizen-report-lists/by-contact/{contactId}          "Xem phản ánh" on a directory row
//
// ⚠ PERSONAL DATA (rule 3, ADR 0102 #8). The petition's match key and the number identity hands back are
// held in local variables for one call and dropped. Neither is logged, put in an error, returned in any
// field, cached, or sent anywhere but the one identity call / the one query it keys. The log lines below
// carry the commune id and an error whose text never quotes either.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// SenderDirectoryLookup is what the two routes ask identity (ADR 0102 #2). *identityclient.Client.
type SenderDirectoryLookup interface {
	FindCitizenContactByPhone(ctx context.Context, phoneNumber string) (found bool, contactID string, err error)
	GetCitizenContactMatchPhone(ctx context.Context, contactID, actorCode string) (found bool, matchPhone string, err error)
}

// ContactPetitionReader is the register's side of the link. *petstore.PhieuPhanAnhStore.
type ContactPetitionReader interface {
	SenderMatchKey(ctx context.Context, petitionID string) (string, error)
	ContactPetitions(ctx context.Context, matchPhone string, includeRestricted bool, limit, offset int) (
		[]domain.ContactPetition, error)
	CountContactPetitions(ctx context.Context, matchPhone string, includeRestricted bool) (int, error)
}

// The three answers of the match. Strings the web switches on (web-admin petition-directory-link.ts).
const (
	directoryMatchFound         = "found"
	directoryMatchNotFound      = "not_found"
	directoryMatchNotApplicable = "not_applicable"
)

// directoryMatchOut is the match's 200. ContactID is null unless Status is "found".
//
// PhoneUnverified is domain.ContactUnverified (ADR 0080): the number was typed by hand in a session with
// no verified phone. The web shows its warning in "Thêm người dân" (ADR 0102 #6); it decides nothing here.
type directoryMatchOut struct {
	Status          string  `json:"status"`
	ContactID       *string `json:"contact_id"`
	PhoneUnverified bool    `json:"phone_unverified"`
}

// contactPetitionOut is one row of "Xem phản ánh": lookup code, title, status, date — and NOTHING about
// the reporter (ADR 0102 #8). `status` is one of the nine (ADR 0027); the web labels it as the register does.
type contactPetitionOut struct {
	LookupCode string    `json:"lookup_code"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	ReceivedAt time.Time `json:"received_at"`
}

// contactPetitionPageOut is one page. `page` is 1-based; `page_size` is the size APPLIED (a request
// above ContactPetitionPageSizeMax is served at the cap).
type contactPetitionPageOut struct {
	Items    []contactPetitionOut `json:"items"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// writeDirectoryUnavailable is the ONE answer when identity could not be asked. NEVER not_found, never an
// empty list: either would offer to add a person already in the directory, or show a person with no
// petitions (transaction-boundaries.json lien_ket_phan_anh_danh_ba_nguoi_dan, (a) and (b)).
func writeDirectoryUnavailable(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusServiceUnavailable, "citizen_directory_unavailable",
		"Chưa đối chiếu được với danh bạ người dân. Vui lòng thử lại sau ít phút.", "")
}

// DirectoryMatch answers whether a petition's sender has a row in the commune's citizen directory.
// GET /api/v1/citizen-reports/{maTraCuu}/directory-match
//
// ORDER, and why each step is where it is:
//  1. the petition, through the SAME read the detail uses — unknown, another commune's, soft-deleted,
//     and `can-bo` without `feedback.restricted` all answer the detail's one 404;
//  2. ANONYMOUS → not_applicable, and identity is NOT CALLED (ADR 0102 #4; the RPC's contract: "must not
//     call, rather than call and discard"). Decided before the key is even read;
//  3. no stored key (no usable number, or a row the backfill has not reached) → not_applicable, no call;
//  4. identity, with the stored key. Any failure → 503, never a guess.
//
// NO AUDIT ENTRY: the answer is a yes/no and an opaque id — no personal data leaves (the RPC is not
// audited in identity either, for the same reason). NO idem.*: a GET.
func (h *Handler) DirectoryMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return
	}
	p, err := h.d.Phieu.TheoMaTraCuu(ctx, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			h.khongTimThay(w)
			return
		}
		h.d.Log.Error("đối chiếu danh bạ: đọc phiếu lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	if p.LinhVuc == LinhVucHanChe {
		principal, ok := authz.From(ctx)
		if !ok || !h.d.Checker.Allows(ctx, principal, QuyenHanChe) {
			h.khongTimThay(w)
			return
		}
	}
	out := directoryMatchOut{Status: directoryMatchNotApplicable, PhoneUnverified: p.ContactUnverified()}
	if p.AnDanh {
		vietJSON(w, http.StatusOK, out)
		return
	}
	key, err := h.d.ContactPetitions.SenderMatchKey(ctx, p.ID)
	if err != nil {
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			// Deleted between the two reads: the detail's answer.
			h.khongTimThay(w)
			return
		}
		h.d.Log.Error("đối chiếu danh bạ: đọc khoá đối chiếu lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	if key == "" {
		vietJSON(w, http.StatusOK, out)
		return
	}
	found, contactID, err := h.d.SenderDirectory.FindCitizenContactByPhone(ctx, key)
	if err != nil {
		h.d.Log.Warn("CẢNH BÁO: đối chiếu danh bạ trả 503 — identity không trả lời",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		writeDirectoryUnavailable(w)
		return
	}
	out.Status = directoryMatchNotFound
	if found {
		out.Status = directoryMatchFound
		out.ContactID = &contactID
	}
	vietJSON(w, http.StatusOK, out)
}

// contactIDMaxLen bounds the path segment before anything is sent: a directory row id is a 26-character
// ULID; anything far longer cannot be one and has no business on the wire.
const contactIDMaxLen = 64

// contactPetitionMaxPage bounds `page` so page × page_size stays a small OFFSET (and cannot overflow).
const contactPetitionMaxPage = 10000

// ContactPetitionsOf lists one directory row's petitions, newest booked first.
// GET /api/v1/citizen-report-lists/by-contact/{contactId}?page=&page_size=
//
//  1. paging parameters checked — 400 before identity is asked;
//  2. identity turns the row id into its number (one audit entry there, naming Principal.Ma — rule 6,
//     invariant 8). Not found → 200 EMPTY: the same answer as a person with no petitions, so the route
//     does not tell a deleted or another commune's row from a live one;
//  3. this service's own query on the key (anonymous and deleted excluded; `can-bo` unless the reader
//     holds `feedback.restricted`), the key then dropped.
//
// The number is never returned, logged or cached (ADR 0102 #8, stop condition #2). NO AUDIT ENTRY HERE:
// the disclosure is identity's, audited there; the rows carry no personal data. NO idem.*: a GET.
func (h *Handler) ContactPetitionsOf(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, pageSize, ok := parseContactPetitionPage(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_page",
			"Trang hoặc số dòng mỗi trang không hợp lệ.", "")
		return
	}
	empty := contactPetitionPageOut{Items: []contactPetitionOut{}, Page: page, PageSize: pageSize}

	contactID := strings.TrimSpace(r.PathValue("contactId"))
	if contactID == "" || len(contactID) > contactIDMaxLen {
		vietJSON(w, http.StatusOK, empty)
		return
	}
	principal, okP := authz.From(ctx)
	if !okP || principal.Ma == "" {
		// NO FALLBACK TO Principal.ID (rule 6, invariant 8): identity's entry must name a business code.
		h.d.Log.Error("xem phản ánh theo danh bạ: chủ thể không có mã cán bộ — không thể ghi vết lộ số",
			"xa", string(tenant.MustFrom(ctx)))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	found, matchPhone, err := h.d.SenderDirectory.GetCitizenContactMatchPhone(ctx, contactID, principal.Ma)
	if err != nil {
		h.d.Log.Warn("CẢNH BÁO: xem phản ánh theo danh bạ trả 503 — identity không trả lời",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		writeDirectoryUnavailable(w)
		return
	}
	if !found || matchPhone == "" {
		vietJSON(w, http.StatusOK, empty)
		return
	}

	restricted := h.d.Checker.Allows(ctx, principal, QuyenHanChe)
	total, err := h.d.ContactPetitions.CountContactPetitions(ctx, matchPhone, restricted)
	if err != nil {
		h.d.Log.Error("xem phản ánh theo danh bạ: đếm lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	rows := []domain.ContactPetition{}
	if offset := (page - 1) * pageSize; offset < total {
		rows, err = h.d.ContactPetitions.ContactPetitions(ctx, matchPhone, restricted, pageSize, offset)
		if err != nil {
			h.d.Log.Error("xem phản ánh theo danh bạ: đọc lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
	}

	out := contactPetitionPageOut{Items: make([]contactPetitionOut, 0, len(rows)), Total: total,
		Page: page, PageSize: pageSize}
	for _, p := range rows {
		out.Items = append(out.Items, contactPetitionOut{
			LookupCode: p.LookupCode,
			Title:      domain.ContactPetitionTitle(p.Content),
			Status:     string(p.Status),
			ReceivedAt: p.ReceivedAt,
		})
	}
	// The rows are a person's petitions: no shared cache may keep them (the web's reads are no-store too).
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusOK, out)
}

// parseContactPetitionPage reads `page` (1-based, default 1) and `page_size` (default
// ContactPetitionPageSizeDefault, served at most at ContactPetitionPageSizeMax). Not a positive integer,
// or a page past contactPetitionMaxPage → false.
func parseContactPetitionPage(r *http.Request) (page, pageSize int, ok bool) {
	q := r.URL.Query()
	page, pageSize = 1, domain.ContactPetitionPageSizeDefault
	if v := firstQueryValue(q, "page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > contactPetitionMaxPage {
			return 0, 0, false
		}
		page = n
	}
	if v := firstQueryValue(q, "page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return 0, 0, false
		}
		pageSize = min(n, domain.ContactPetitionPageSizeMax)
	}
	return page, pageSize, true
}

// firstQueryValue is the first value of one query parameter, "" when absent. Spelled out rather than
// through the url package's accessor, which rbac_guard reads as a route declaration.
func firstQueryValue(q url.Values, key string) string {
	if v := q[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}
