package http

// The STAFF surface of the petition processing logbook (migration 0013; docs/ui-ux/09 §8.7).
//
//	GET  /api/v1/citizen-reports/{maTraCuu}/log-entries   feedback.read (+ feedback.restricted on `can-bo`)
//	POST /api/v1/citizen-reports/{maTraCuu}/log-entries   feedback.read + app.duocGhiChu
//
// STAFF-INTERNAL. Nothing here is mounted on the citizen mux, and nothing here may be (rule 4,
// forbidden #5): the timeline IS the routing history and the staff notes.
//
// # NO `actor_name` ON THE WIRE, AND THE ABSENCE IS THE PRECEDENT'S
//
// The row names its author by staff BUSINESS CODE (`CB-00123`, rule 6 invariant 8). The task logbook
// this copies (`nhat_ky_nhiem_vu`) resolves no names through identity anywhere in this service, so
// neither does this. A screen that wants the name resolves the codes of one page with one batched
// call (skills/load-data-once) — adding that here is a decision about a dependency on identity for a
// read path, and it is reported, not made.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The two other commune-wide petition keys the note rule reads (app.QuyenGhiChuCaXa). Constants and
// not literals for the reason QuyenXuLyCaXa gives: they are used with Checker.Allows, not inside
// authz.RequirePermission. Both are seeded (service-identity/migrations/0001_init.sql and
// 0007_quyen_phan_loai_va_xem_day_du.sql); none is invented (rule 5, invariant 3c).
const (
	QuyenPhanCongPhieu authz.Perm = "feedback.assign"
	QuyenPhanLoaiPhieu authz.Perm = "feedback.classify"
)

// ghiChuPhieuVao is the body of POST …/{maTraCuu}/log-entries. `note` is MANDATORY (no omitempty):
// a note row with nothing in it is refused by migration 0013 and could never be corrected.
//
// ⚠ PERSONAL DATA MAY BE IN IT (rule 3). At most domain.GhiChuToiDa characters.
type ghiChuPhieuVao struct {
	Note string `json:"note"`
}

// nhatKyPhieuRa is one timeline row as it leaves the API to a member of staff.
type nhatKyPhieuRa struct {
	// ID is the row's internal id — the tie-break of the page order, and what a screen keys a list
	// item on. It names nothing outside this service.
	ID string `json:"id"`

	// At is the instant of the act, from the same clock as the act itself.
	At time.Time `json:"at"`

	// ActorCode is the STAFF BUSINESS CODE of who acted (`CB-00123`), never an internal id.
	ActorCode string `json:"actor_code"`

	// Action is one of the seven codes of migration 0013: `phan-loai`, `phan-cong`,
	// `chuyen-trang-thai`, `dong-phieu`, `khong-tiep-nhan`, `chuyen-cap-tren`, `ghi-chu`.
	Action string `json:"action"`

	// Status is the status the petition stood in at this moment — AFTER the act, for an act that
	// moves it. One of the nine codes.
	Status string `json:"status"`

	// Unit and Assignee are who the petition was handed to AT THIS STEP — set on `phan-cong` only,
	// "" on every other row. Assignee is "" on a `phan-cong` row too when the department was left to
	// choose ("— Để bộ phận phân công —"). A staff business code, never an internal id.
	Unit     string `json:"unit"`
	Assignee string `json:"assignee"`

	// Note is the staff note: mandatory on `ghi-chu`, optional on every other act, "" when none.
	// ⚠ PERSONAL DATA MAY BE IN IT (rule 3) — staff-internal.
	Note string `json:"note"`

	// Attachments is ALWAYS [] today: there is no file store in this repository, and migration 0013
	// reserves the column. The element shape is deliberately unpublished until that store exists.
	Attachments []any `json:"attachments"`
}

func nhatKyRaNgoai(e domain.NhatKyPhanAnh) nhatKyPhieuRa {
	return nhatKyPhieuRa{
		ID: e.ID, At: e.ThoiDiem, ActorCode: e.NguoiMa, Action: string(e.HanhVi),
		Status: string(e.TrangThai), Unit: e.BoPhanID, Assignee: e.CanBoXuLyMa, Note: e.NoiDung,
		Attachments: []any{},
	}
}

// DocNhatKyPhieu serves one page of one petition's timeline, newest first.
// GET /api/v1/citizen-reports/{maTraCuu}/log-entries
//
// THE PETITION IS READ FIRST, THROUGH THE SAME READER AS THE DETAIL ROUTE, and that is the check: the
// logbook is keyed by the petition's internal id, which this handler has only after that read — so a
// soft-deleted petition, another commune's code and an unknown code all stop at the one 404, and a
// `can-bo` petition without `feedback.restricted` stops there too, identical to an unknown code (the
// reasoning is on DocPhieuPhanAnh). NO NAME RESOLUTION — see the file header.
func (h *Handler) DocNhatKyPhieu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return
	}

	// Parsed BEFORE any store is touched: a rejected page request runs no statement at all.
	yc, err := page.Parse(r.URL.Query(), petstore.SapXepNhatKyPhieu)
	if err != nil {
		status, maLoi, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, maLoi, thongBao, "")
		return
	}

	p, err := h.d.Phieu.TheoMaTraCuu(ctx, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			h.khongTimThay(w)
			return
		}
		h.d.Log.Error("đọc phiếu cho nhật ký xử lý: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	if p.LinhVuc == LinhVucHanChe && !h.coQuyenHanChe(ctx) {
		h.khongTimThay(w)
		return
	}

	kq, err := h.d.NhatKyPhieu.NhatKyCuaPhieu(ctx, p.ID, yc)
	if err != nil {
		// The wrapped error carries the store failure — never a note, never the lookup code (rule 3).
		h.d.Log.Error("đọc nhật ký xử lý phiếu: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) so an empty timeline marshals as [] and never as null.
	ra := page.Result[nhatKyPhieuRa]{
		Items:      make([]nhatKyPhieuRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, e := range kq.Items {
		ra.Items = append(ra.Items, nhatKyRaNgoai(e))
	}
	vietJSON(w, http.StatusOK, ra)
}

// GhiChuPhieu appends one manual internal note. POST /api/v1/citizen-reports/{maTraCuu}/log-entries
//
// THIS HANDLER ANSWERS TWO QUESTIONS AND DECIDES NOTHING — the shape of TienTrangThaiPhieu. It reads
// whether the caller holds `feedback.restricted`, and whether they hold ANY of the three commune-wide
// petition keys; whether the note is allowed is app.duocGhiChu's, on the row read FOR UPDATE.
func (h *Handler) GhiChuPhieu(w http.ResponseWriter, r *http.Request) {
	var vao ghiChuPhieuVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	dong, err := h.d.XuLyPhieu.GhiChuNoiBo(ctx, r.PathValue("maTraCuu"), vao.Note, nguoi,
		h.coQuyenGhiChuCaXa(ctx), h.coQuyenHanChe(ctx))
	if err != nil {
		h.traLoiLoiXuLy(w, r, "ghi chú", err)
		return
	}
	// What a retry carrying the same Idempotency-Key is told: the row's id, never the note.
	idem.RecordCode(ctx, dong.ID)
	vietJSON(w, http.StatusCreated, nhatKyRaNgoai(dong))
}

// coQuyenGhiChuCaXa: does this account hold feedback.resolve, feedback.assign OR feedback.classify?
// FAIL CLOSED — no principal is `false`.
func (h *Handler) coQuyenGhiChuCaXa(ctx context.Context) app.QuyenGhiChuCaXa {
	principal, ok := authz.From(ctx)
	if !ok {
		return false
	}
	for _, k := range []authz.Perm{QuyenXuLyCaXa, QuyenPhanCongPhieu, QuyenPhanLoaiPhieu} {
		if h.d.Checker.Allows(ctx, principal, k) {
			return true
		}
	}
	return false
}
