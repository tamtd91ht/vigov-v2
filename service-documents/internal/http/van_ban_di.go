package http

// The routes of SỔ VĂN BẢN ĐI.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER — see migration 0004. The four routes mirror the
// incoming register's, minus routing (an outgoing document is not sent between departments; it
// leaves the commune) and minus the deadline (there is no commitment to meet: issuing IS the act).
//
// FOUR ROUTES, TWO PERMISSIONS, THE SAME KEYS AS THE INCOMING REGISTER: `document.create` issues and
// corrects, `document.read` lists. No key was invented (rule 5, invariant 3c), and the same finding
// applies — there is no `document.delete`, so removing an entry is guarded by `document.create`.
//
// THE URL RESOURCE IS `outgoing-documents`, settled at kb/00-foundation/ubiquitous-language.md:143
// and not translated on the spot (ADR 0011).
//
// THE ERROR MAPPING IS traLoiLoiVanBan, SHARED WITH THE INCOMING REGISTER. One mapping, because the
// two books must answer the same question the same way; the sentences differ where the books differ,
// which is what the two separate `ErrKhongThay…` sentinels are for.

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// vanBanDiRa is one outgoing document as it leaves the API.
//
// ⚠ `Recipient` MAY NAME A CITIZEN (rule 3) — "Ông Nguyễn Văn A, thôn Bình Trị" is what a commune
// writes on a reply. It is returned unmasked to that commune's own staff, which is what the register
// is for; what is guaranteed is that it is never logged and never appears in an error message.
//
// THERE IS NO `status` AND NO `due_at`. An outgoing document has no lifecycle and no commitment in
// any source — see domain.VanBanDi. Inventing either would invent a workflow a commune then has to
// follow, or a promise nobody made.
type vanBanDiRa struct {
	ID string `json:"id"`

	// Number and Year are the ISSUED NUMBER — the one that is printed on the document, sealed and
	// sent out. Output only: a request carrying either is refused with 400.
	Number int `json:"number"`
	Year   int `json:"year"`

	DocumentDate string `json:"document_date"` // YYYY-MM-DD, the day it was signed and issued
	DocumentType string `json:"document_type"` // the catalogue CODE
	Summary      string `json:"summary"`
	Recipient    string `json:"recipient"`
	Signer       string `json:"signer,omitempty"`

	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func vanBanDiRaNgoai(v domain.VanBanDi) vanBanDiRa {
	return vanBanDiRa{
		ID:           v.ID,
		Number:       v.SoDi,
		Year:         v.Nam,
		DocumentDate: ngayRa(v.NgayVanBan),
		DocumentType: v.LoaiVanBan,
		Summary:      v.TrichYeu,
		Recipient:    v.NoiNhan,
		Signer:       v.NguoiKy,
		CreatedBy:    v.NguoiTaoMa,
		CreatedAt:    lucRa(v.TaoLuc),
		UpdatedAt:    lucRa(v.CapNhatLuc),
	}
}

// capSoVanBanDiVao is the body of POST /api/v1/outgoing-documents.
//
// `Number` IS HERE ONLY SO IT CAN BE REFUSED, and on this register that refusal is the heaviest one
// in the file: the number goes onto paper, under a seal, out of the building. A client that could
// name it could put a number already carried by a signed document onto a second one.
type capSoVanBanDiVao struct {
	DocumentDate string `json:"document_date"` // YYYY-MM-DD
	DocumentType string `json:"document_type"`
	Summary      string `json:"summary"`
	Recipient    string `json:"recipient"`
	Signer       string `json:"signer,omitempty"`

	Number *int `json:"number,omitempty"`
}

// suaVanBanDiVao is the body of PATCH /api/v1/outgoing-documents/{id}. Pointers for the same reason
// as the incoming register: `signer` is optional and its empty value is a statement.
type suaVanBanDiVao struct {
	DocumentDate *string `json:"document_date,omitempty"`
	DocumentType *string `json:"document_type,omitempty"`
	Summary      *string `json:"summary,omitempty"`
	Recipient    *string `json:"recipient,omitempty"`
	Signer       *string `json:"signer,omitempty"`

	Number *int `json:"number,omitempty"`
}

// CapSoVanBanDi issues one outgoing document number. POST /api/v1/outgoing-documents
func (h *Handler) CapSoVanBanDi(w http.ResponseWriter, r *http.Request) {
	var vao capSoVanBanDiVao
	if !docThan(w, r, &vao) {
		return
	}
	if err := khongDoTuClient(vao.Number, nil, nil); err != nil {
		h.traLoiLoiVanBan(w, r, "cấp số", err)
		return
	}

	ngay, ok := ngayVao(vao.DocumentDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-22.", "document_date")
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	moi, err := h.d.GhiVanBanDi.CapSo(r.Context(), app.YeuCauCapSoVanBanDi{
		NgayVanBan: ngay,
		LoaiVanBan: vao.DocumentType,
		TrichYeu:   vao.Summary,
		NoiNhan:    vao.Recipient,
		NguoiKy:    vao.Signer,
	}, nguoi)
	if err != nil {
		h.traLoiLoiVanBan(w, r, "cấp số", err)
		return
	}

	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, vanBanDiRaNgoai(moi))
}

// SuaVanBanDi corrects one issued entry. PATCH /api/v1/outgoing-documents/{id}
func (h *Handler) SuaVanBanDi(w http.ResponseWriter, r *http.Request) {
	var vao suaVanBanDiVao
	if !docThan(w, r, &vao) {
		return
	}
	if err := khongDoTuClient(vao.Number, nil, nil); err != nil {
		h.traLoiLoiVanBan(w, r, "sửa", err)
		return
	}

	yc := app.YeuCauSuaVanBanDi{
		LoaiVanBan: vao.DocumentType,
		TrichYeu:   vao.Summary,
		NoiNhan:    vao.Recipient,
		NguoiKy:    vao.Signer,
	}
	if vao.DocumentDate != nil {
		t, ok := ngayVao(*vao.DocumentDate)
		if !ok || t.IsZero() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-22.", "document_date")
			return
		}
		yc.NgayVanBan = &t
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	sau, err := h.d.GhiVanBanDi.Sua(r.Context(), r.PathValue("id"), yc, nguoi)
	if err != nil {
		h.traLoiLoiVanBan(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, vanBanDiRaNgoai(sau))
}

// GoVanBanDi soft deletes one entry. DELETE /api/v1/outgoing-documents/{id}
//
// 204 AND NO BODY, and the row stays with its three removal columns. THE NUMBER STAYS TAKEN — the
// document was issued under it and has left the commune; taking the row off the screen cannot take
// the number off the paper.
func (h *Handler) GoVanBanDi(w http.ResponseWriter, r *http.Request) {
	var vao goVanBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiVanBanDi.Go(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiVanBan(w, r, "gỡ", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DanhSachVanBanDi serves one page of the outgoing register. GET /api/v1/outgoing-documents
func (h *Handler) DanhSachVanBanDi(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING BELOW READS `tenant_id`
	// from the query string — a client naming its own commune grants itself access (rule 1, #2).
	thamSo := r.URL.Query()

	yc, err := page.Parse(thamSo, docstore.SapXepVanBanDi)
	if err != nil {
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	loc, err := locVanBanDiTuQuery(thamSo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	kq, err := h.d.VanBanDi.DanhSach(ctx, loc, yc)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT carry a summary or a recipient,
		// and it never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("danh sách văn bản đi: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	ra := page.Result[vanBanDiRa]{
		Items:      make([]vanBanDiRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, v := range kq.Items {
		ra.Items = append(ra.Items, vanBanDiRaNgoai(v))
	}
	vietJSON(w, http.StatusOK, ra)
}

// The two filter refusals, shared with the incoming register so that one mistake reads the same on
// both screens. A second wording is a second answer to one question.
var (
	errNamKhongHopLe       = errors.New("`year` phải là một năm hợp lệ, ví dụ 2026")
	errTimQuaDai           = errors.New("`q` quá dài")
	errTrangThaiKhongHopLe = errors.New("`status` không phải một trạng thái của sổ văn bản đến")
)

func locVanBanDiTuQuery(q map[string][]string) (docstore.LocVanBanDi, error) {
	lay := func(k string) string {
		if v, ok := q[k]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}

	var loc docstore.LocVanBanDi
	if s := lay("year"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2000 || n > 2200 {
			return loc, errNamKhongHopLe
		}
		loc.Nam = n
	}
	loc.LoaiVanBan = lay("document_type")
	loc.Tim = lay("q")
	if len(loc.Tim) > 200 {
		return loc, errTimQuaDai
	}
	return loc, nil
}
