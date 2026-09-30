package http

// The three routes of the `⇄ Các đợt thu, chi` dialog (docs/ui-ux/07-thu-chi-ngan-sach.md §5,
// migration 0008). Declared in routes.go with their permissions; mapped to errors by
// traLoiLoiNganSach, the one mapping for the whole budget board.
//
// AMOUNTS are JSON numbers of đồng, `null` for an empty amount (§9 rule 4) — the contract bangDayDuRa
// states for the sheet.
//
// `counterparty` ("Đơn vị, cá nhân", `don_vi_ca_nhan`) MAY NAME A PERSON (migration 0008), so it
// LEAVES THE API MASKED with core/privacy.MaskName (rule 3, invariant 3) — on the list and on the 201
// alike. No permission grants the full value: showing it would need an explicit full-view key, which
// the `quyen` table does not have (rule 3 stop condition #1; open question #27 — never an INSERT).
//
// NOT THE VOUCHER'S PRECEDENT, deliberately: chungTuRa emits its `counterparty` raw because that field
// is declared a COMPANY, never a person (domain/chung_tu_giai_ngan.go ChuanHoaDoiTac, 0004:283-284).
// 0008 declares the opposite for this column.
//
// THE COSTS, stated: (1) an ORGANISATION name is masked too ("Công ty TNHH ABC" -> "Công t. T. A.") —
// the text cannot tell a household from a company, and failing closed is the rule; (2) MaskName leaves
// a ONE-WORD value unchanged, which is the shared helper's behaviour and not re-decided here.
// Never logged and never put in an error message by anything in this file.

import (
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// dotRa is one batch.
type dotRa struct {
	ID     string `json:"id"`
	LineID string `json:"line_id"`
	Date   string `json:"date"` // YYYY-MM-DD
	// Content is "Nội dung".
	Content string `json:"content"`
	// Counterparty is "Đơn vị, cá nhân", MASKED (privacy.MaskName) — omitted when not stated.
	Counterparty string `json:"counterparty,omitempty"`
	DocumentNo   string `json:"document_no,omitempty"`
	RecordedAt   string `json:"recorded_at,omitempty"` // RFC 3339; absent on the 201 of a create

	// Values is columnID -> đồng. ON THE LIST, EVERY LIVE NUMBER COLUMN OF THE SHEET IS A KEY, `null`
	// when this batch left it empty — the same shape dongRa.Values has. On the 201 of a create only
	// the stated amounts appear; the client refetches the list, as it refetches the sheet.
	Values map[string]*int64 `json:"values"`

	// UnavailableReasons is columnID -> sentence, for an amount STORED before the ceiling was lowered
	// (domain.GiaTriToiDa, 25/09/2026) that the browser could not read exactly. Such an amount is
	// `null` in Values and keyed here, so the accountant can find the batch and remove it. Omitted in
	// the ordinary case.
	UnavailableReasons map[string]string `json:"unavailable_reasons,omitempty"`
}

// danhSachDotRa is the dialog's list.
type danhSachDotRa struct {
	LineID string `json:"line_id"`

	// Method is the line's calculation mode. The batches move the line's DISPLAYED figure only when
	// this is `entries` (§4.2); the screen can say so instead of leaving the accountant to wonder why
	// a recorded batch changed nothing.
	Method string `json:"method"`

	Entries []dotRa `json:"entries"` // newest first: date DESC, then recording time DESC
}

// ghiDotVao is the body of POST /api/v1/budget-lines/{id}/entries.
//
// THERE IS NO `line_id`: the line is the path segment, and a second copy in the body is a second
// answer that could disagree. `values` is `map[string]*int64`: a number is an amount in đồng, `null`
// is "left empty" and stores nothing. At least one amount must be a number.
type ghiDotVao struct {
	Date         string            `json:"date"` // YYYY-MM-DD
	Content      string            `json:"content"`
	Counterparty string            `json:"counterparty,omitempty"`
	DocumentNo   string            `json:"document_no,omitempty"`
	Values       map[string]*int64 `json:"values"`
}

func dotRaNgoai(d domain.DotThuChi, cot []domain.CotNganSach) dotRa {
	ra := dotRa{
		ID: d.ID, LineID: d.KhoanMucID, Date: ngayRa(d.Ngay), Content: d.NoiDung,
		Counterparty: privacy.MaskName(d.DonViCaNhan), DocumentNo: d.SoChungTu, RecordedAt: lucRa(d.TaoLuc),
		Values: map[string]*int64{},
	}
	for _, c := range cot {
		if c.Kieu != domain.CotSo {
			continue // a percentage column carries no amount (§9 rule 3)
		}
		ra.Values[c.ID] = nil
	}
	for cotID, g := range d.GiaTri {
		if err := domain.KiemTraGiaTriDaLuu(g); err != nil {
			ra.Values[cotID] = nil
			if ra.UnavailableReasons == nil {
				ra.UnavailableReasons = map[string]string{}
			}
			ra.UnavailableReasons[cotID] = err.Error()
			continue
		}
		v := int64(g)
		ra.Values[cotID] = &v
	}
	return ra
}

// DocDotThuChi returns one line's live batches. GET /api/v1/budget-lines/{id}/entries
func (h *Handler) DocDotThuChi(w http.ResponseWriter, r *http.Request) {
	ds, err := h.d.NganSach.DotCuaKhoanMuc(r.Context(), r.PathValue("id"))
	if err != nil {
		h.traLoiLoiNganSach(w, r, "đọc đợt", err)
		return
	}
	ra := danhSachDotRa{
		LineID: ds.KhoanMuc.ID, Method: string(ds.KhoanMuc.CachTinh),
		Entries: make([]dotRa, 0, len(ds.Dot)),
	}
	for _, d := range ds.Dot {
		ra.Entries = append(ra.Entries, dotRaNgoai(d, ds.Cot))
	}
	vietJSON(w, http.StatusOK, ra)
}

// GhiDotThuChi records one batch. POST /api/v1/budget-lines/{id}/entries
func (h *Handler) GhiDotThuChi(w http.ResponseWriter, r *http.Request) {
	var vao ghiDotVao
	if !docThan(w, r, &vao) {
		return
	}
	ngay, ok := ngayVao(vao.Date)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`date` phải theo dạng YYYY-MM-DD, ví dụ 2026-08-25.", "")
		return
	}
	gia := make(map[string]*domain.Dong, len(vao.Values))
	for cotID, v := range vao.Values {
		if v == nil {
			gia[cotID] = nil
			continue
		}
		g := domain.Dong(*v)
		gia[cotID] = &g
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	moi, err := h.d.GhiNganSach.GhiDot(r.Context(), app.YeuCauGhiDot{
		KhoanMucID: r.PathValue("id"), Ngay: ngay, NoiDung: vao.Content,
		DonViCaNhan: vao.Counterparty, SoChungTu: vao.DocumentNo, GiaTri: gia,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "ghi đợt", err)
		return
	}

	// The batch id, never the body: the body carries `counterparty` and would land in Redis, which is
	// a cache and not a record store.
	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, dotRaNgoai(moi, nil))
}

// GoDotThuChi soft deletes one batch. DELETE /api/v1/budget-entries/{id}
func (h *Handler) GoDotThuChi(w http.ResponseWriter, r *http.Request) {
	var vao goVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiNganSach.GoDot(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiNganSach(w, r, "gỡ đợt", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
