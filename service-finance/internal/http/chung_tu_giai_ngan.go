package http

// The WRITE routes of the disbursement voucher register (docs/ui-ux/06-giai-ngan.md §8.2).
//
// SIX ROUTES, THREE PERMISSIONS, AND THE SPLIT IS THE SPECIFICATION'S OWN (06-giai-ngan.md:202):
// `budget.update` enters, corrects and removes (removal moved here from `budget.confirm` on
// 07/10/2026, following the prototype — routes.go says why), `budget.confirm` confirms and
// freezes. The seventh thing a screen needs — LISTING a project's vouchers — is
// GET /api/v1/investment-projects/{id}/disbursements (ListProjectVouchers, below; added 06/10/2026). The questions it was deferred over are answered
// there: every state, no paging (one PROJECT's vouchers, not a commune's year), a hard ceiling that
// refuses rather than truncates.
//
// THE LIFECYCLE IS A CHAIN AND THE ROUTES ARE ITS EDGES:
//
//	POST   /api/v1/disbursements                    -> Kế toán nhập
//	POST   /api/v1/disbursements/{id}/confirmation  -> Đã xác nhận
//	POST   /api/v1/disbursements/{id}/lockout       -> Đã khoá (from Kế toán nhập too: confirms and
//	                                                   locks in one call, 07/10/2026)
//	DELETE /api/v1/disbursements/{id}/lockout       -> back to Đã xác nhận, with a reason, by
//	                                                   somebody OTHER than the person who locked it
//
// `lockout` AND `confirmation` ARE NOMINALISED SUB-RESOURCES, NOT VERBS IN A PATH. `lock`,
// `unlock` and `confirm` are all verbs, which skills/rest-api-design forbids on a path and
// `rest_api_guard` reports. `lockout` is the noun this system already settled for exactly this
// shape — a STATE that POST creates and DELETE removes — on `POST`/`DELETE /api/v1/staff/{id}/
// lockout` (kb/00-foundation/ubiquitous-language.md:160). ⚠ THAT ROW IS ABOUT A STAFF ACCOUNT, NOT
// ABOUT A VOUCHER: the noun is being REUSED here by this session because the mapping table has no
// row for "khoá chứng từ", and ADR 0011 says to ask rather than translate on the spot. It is a
// finding in the hand-over, not a decision hidden in a route.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// chungTuRa is one voucher as it leaves the API.
//
// EVERY AMOUNT IS A JSON NUMBER OF ĐỒNG, never a formatted string — the same contract duAnRa
// states and for the same reason: a server sending formatted money makes every consumer parse it
// back before it can add two of them up, which is where a đồng goes missing.
//
// NOTHING HERE IS PERSONAL DATA (rule 3). `counterparty` is a company ("Công ty ABC") — 0004:283-284
// says so outright — and the four staff fields carry BUSINESS CODES (`CB-2026-7K3M9Q`), not names:
// resolving a code to a person is another route's job under another permission.
//
// THE FOUR UNLOCK FIELDS DESCRIBE THE LAST UNLOCK ONLY, and `unlock_count` is what says whether
// there were others. The history is in `audit_log`, which is append-only; these columns are what a
// screen shows beside the row, and the two answer different questions (0005:134-139).
type chungTuRa struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`

	// PaymentDate is the date of the PAYMENT, not the date the row was typed. They differ routinely
	// — a commune enters last week's vouchers on Monday — and every cumulative chart of §4 is
	// ordered by this one.
	PaymentDate string `json:"payment_date"` // YYYY-MM-DD
	Amount      int64  `json:"amount"`       // đồng, always > 0 (open question #30)
	Description string `json:"description"`

	Counterparty string `json:"counterparty,omitempty"`
	VoucherNo    string `json:"voucher_no,omitempty"`

	// FundingSourceID is `nguon_von.id` — which funding source this payment was drawn from (§8.2's
	// `NGUỒN VỐN` column, migration 0007).
	//
	// `omitempty`, AND ITS ABSENCE IS A MEANINGFUL ANSWER rather than a field the server forgot: a
	// voucher with no source is the state §13 rule 6 defines and §6 reports as "đã chi nhưng chưa
	// ghi rút từ nguồn nào". The screen draws `—` in that column, which is exactly what §8.2's own
	// sample rows show.
	//
	// THE NAME IS `funding_source_id` AND IT IS NOT A URL NOUN. ADR 0011 governs resource names in
	// paths and `kb/00-foundation/ubiquitous-language.md` has no row for `nguon_von`; this is a FIELD,
	// and `FundingSource` is the entity name migration 0007 already carries (`@entity: FundingSource`,
	// 0007:134). No CRUD route for the catalogue is created here — that noun still has to be asked for.
	FundingSourceID string `json:"funding_source_id,omitempty"`

	// FundingSourceName is the NAME of that source, for the `NGUỒN VỐN` column — SET BY THE PROJECT'S
	// VOUCHER LIST ONLY (GET /api/v1/investment-projects/{id}/disbursements). The write routes reply
	// without it: the client just chose the source and already holds its name, and reading the
	// catalogue again inside every write to echo a label would be a second read for no new fact.
	// Absent when the voucher names no source.
	FundingSourceName string `json:"funding_source_name,omitempty"`

	// Status is `ke-toan-nhap` | `da-xac-nhan` | `da-khoa` — Vietnamese without diacritics, which
	// is ADR 0011: only the surrounding contract is English. OUTPUT ONLY; a request carrying it is
	// refused with 400, because the state is what the four lifecycle routes are FOR.
	Status string `json:"status"`

	EnteredBy   string `json:"entered_by"`             // `nguoi_nhap_id` — a staff business code
	ConfirmedBy string `json:"confirmed_by,omitempty"` // `nguoi_xac_nhan_id`
	LockedBy    string `json:"locked_by,omitempty"`    // `nguoi_khoa_id`
	LockedAt    string `json:"locked_at,omitempty"`    // RFC 3339

	UnlockedBy   string `json:"unlocked_by,omitempty"`
	UnlockedAt   string `json:"unlocked_at,omitempty"`
	UnlockReason string `json:"unlock_reason,omitempty"`

	// UnlockCount is NOT `omitempty`. Zero is the answer for a voucher nobody has reopened, and it
	// is a fact rather than an absence (0005:145-148) — a field that vanished at zero would make a
	// client unable to tell "never unlocked" from "this server does not report it".
	UnlockCount int `json:"unlock_count"`
}

func lucRa(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func chungTuRaNgoai(c domain.ChungTuGiaiNgan) chungTuRa {
	return chungTuRa{
		ID:              c.ID,
		ProjectID:       c.DuAnID,
		PaymentDate:     ngayRa(c.NgayChi),
		Amount:          int64(c.SoTien),
		Description:     c.NoiDung,
		Counterparty:    c.DoiTac,
		VoucherNo:       c.SoChungTu,
		FundingSourceID: c.NguonVonID,
		Status:          string(c.TrangThai),
		EnteredBy:       c.NguoiNhapID,
		ConfirmedBy:     c.NguoiXacNhanID,
		LockedBy:        c.NguoiKhoaID,
		LockedAt:        lucRa(c.ThoiDiemKhoa),
		UnlockedBy:      c.NguoiMoKhoaID,
		UnlockedAt:      lucRa(c.ThoiDiemMoKhoa),
		UnlockReason:    c.LyDoMoKhoa,
		UnlockCount:     c.SoLanMoKhoa,
	}
}

// `omitempty` ON EVERY OPTIONAL INPUT FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field
// REQUIRED in kb/20-contracts/openapi.json unless it carries `omitempty`, so without it these
// bodies would tell every generated client that `status` MUST be sent — on routes that answer 400
// to exactly that.

// themChungTuVao is the body of POST /api/v1/disbursements.
//
// `Status` IS HERE ONLY SO IT CAN BE REFUSED. It is not written anywhere and never reaches the
// store — the INSERT writes `'ke-toan-nhap'` as a literal. Declaring it and answering 400 is the
// difference between a client learning that the lifecycle is not theirs to set and a client
// believing it just created a voucher already `Đã khoá`: a figure nobody confirmed, frozen against
// editing, counting toward the commune's disbursement total.
//
// THERE IS NO `entered_by` FIELD AND THERE MUST NEVER BE ONE. Who entered the voucher is the acting
// principal from the session; a field would be a client naming somebody else as the author of a
// financial record (rule 1, forbidden #2, applied to a person instead of a commune).
//
// `funding_source_id` IS OPTIONAL IN THE SCHEMA AND DECIDED BY THE PROJECT (user decision 06/10/2026,
// domain.CheckVoucherSource): on a project with live allocation lines it is required and must be one
// of them — 409 `source_required` / `source_not_allocated`; on a project with none it must be absent —
// 409 `source_not_allocated`. The schema cannot say "required when the project has lines", so the
// field stays `omitempty` and the 409 carries the rule.
type themChungTuVao struct {
	ProjectID       string `json:"project_id"`
	PaymentDate     string `json:"payment_date"` // YYYY-MM-DD
	Amount          int64  `json:"amount"`       // đồng
	Description     string `json:"description"`
	Counterparty    string `json:"counterparty,omitempty"`
	VoucherNo       string `json:"voucher_no,omitempty"`
	FundingSourceID string `json:"funding_source_id,omitempty"`

	Status *string `json:"status,omitempty"`
}

// suaChungTuVao is the body of PATCH /api/v1/disbursements/{id}.
//
// EVERY EDITABLE FIELD IS A POINTER, and that is the whole reason this is a PATCH and not a PUT:
// `counterparty` and `voucher_no` are optional, and their empty string is a MEANINGFUL value —
// "this voucher has no treasury number" is a statement, not an absence of one. A body of plain
// values cannot tell "not mentioned" from "cleared", so a dialog editing only the description would
// wipe the counterparty off a payment record.
//
// `ProjectID` IS REFUSED, NOT IGNORED. Moving a voucher between projects moves money between two
// reported totals with nothing on either screen saying so; the operation for one filed against the
// wrong project is to remove it with a reason and enter it again — two events, both audited.
//
// `funding_source_id` CARRIES THREE ANSWERS, which is the whole reason it is a pointer here too:
// absent leaves the source alone, `""` DETACHES the voucher — putting it back into §6's "đã chi
// nhưng chưa ghi rút từ nguồn nào" warning — and an id attaches it to that source. A body of plain
// values could not express the middle one, so a commune that attributed a payment to the wrong
// source would have no way to say "not this one" short of removing the voucher entirely. WHEN NAMED,
// the project's rule applies (decision 06/10/2026): `""` on a project with allocation lines is 409
// `source_required`, a source it does not draw on is 409 `source_not_allocated`. WHEN ABSENT, nothing
// is checked — see app.Sua for why a legacy NULL source is not forced.
//
// `null` READS AS ABSENT, NOT AS DETACH, because that is what a `*string` does in encoding/json and
// it is the convention `counterparty` and `voucher_no` already set on this very body. The clearing
// spelling is `""` for all three, so a client does not have to remember which field takes which.
type suaChungTuVao struct {
	PaymentDate     *string `json:"payment_date,omitempty"`
	Amount          *int64  `json:"amount,omitempty"`
	Description     *string `json:"description,omitempty"`
	Counterparty    *string `json:"counterparty,omitempty"`
	VoucherNo       *string `json:"voucher_no,omitempty"`
	FundingSourceID *string `json:"funding_source_id,omitempty"`

	ProjectID *string `json:"project_id,omitempty"`
	Status    *string `json:"status,omitempty"`
}

// goChungTuVao is the OPTIONAL body of DELETE /api/v1/disbursements/{id}.
//
// THE REASON IS OPTIONAL (task card G2, 07/10/2026, following the prototype): absent or blank, the
// row records domain.VoucherRemovalDefaultReason, so rule 7 invariant 1's `delete_reason` is never
// empty. When given it rides in a BODY rather than the query string, where free text about a public
// authority's spending would land in every access log and proxy cache. `omitempty` so the generated
// contract does not mark it required.
type goChungTuVao struct {
	Reason string `json:"reason,omitempty"`
}

// moKhoaVao is the body of DELETE /api/v1/disbursements/{id}/lockout.
//
// THE REASON IS MANDATORY, and this is the one body in this file where that is a DECISION rather
// than a rule somebody else made: migration 0005 settled open question #29 in this direction on
// 2026-09-22, by this project and not by the customer, because the unlocks that have ALREADY
// HAPPENED cannot be rebuilt from any source afterwards. `chung_tu_giai_ngan_mo_khoa_du_vet`
// refuses a blank one in the database; the use case refuses it first, in Vietnamese.
type moKhoaVao struct {
	Reason string `json:"reason"`
}

// ngayVao parses a YYYY-MM-DD payment date.
//
// `time.DateOnly` AND NOT RFC 3339: this is a DATE — the day the payment was made — and a client
// sending an instant would be choosing a timezone for a fact that has none. The column is `DATE`.
//
// THE ZERO VALUE IS RETURNED FOR AN EMPTY STRING rather than an error, so that PATCH can tell
// "absent" from "malformed" using its own pointer. domain.KiemTraNgayChi refuses the zero on the
// create path, where the field is required.
func ngayVao(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ThemChungTu records one payment against one project. POST /api/v1/disbursements
func (h *Handler) ThemChungTu(w http.ResponseWriter, r *http.Request) {
	var vao themChungTuVao
	if !docThan(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE. A body naming `status` is refused outright, so the caller learns that
	// the lifecycle is not theirs to set rather than watching the field disappear.
	if vao.Status != nil {
		h.traLoiLoiChungTu(w, r, "thêm", domain.ErrTrangThaiDoTuClient)
		return
	}
	ngay, ok := ngayVao(vao.PaymentDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`payment_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-07.", "")
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	moi, err := h.d.GhiChungTu.Them(r.Context(), app.YeuCauThemChungTu{
		DuAnID:     vao.ProjectID,
		NgayChi:    ngay,
		SoTien:     domain.Dong(vao.Amount),
		NoiDung:    vao.Description,
		DoiTac:     vao.Counterparty,
		SoChungTu:  vao.VoucherNo,
		NguonVonID: vao.FundingSourceID,
	}, nguoi)
	if err != nil {
		h.traLoiLoiChungTu(w, r, "thêm", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE ID AND NOT THE BODY: the
	// body would go into Redis, which is a cache and not a record store. A voucher has no business
	// code of its own — there is no `ma` column — so the id is the only stable handle there is.
	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, chungTuRaNgoai(moi))
}

// SuaChungTu corrects an unlocked voucher. PATCH /api/v1/disbursements/{id}
func (h *Handler) SuaChungTu(w http.ResponseWriter, r *http.Request) {
	var vao suaChungTuVao
	if !docThan(w, r, &vao) {
		return
	}
	if vao.Status != nil {
		h.traLoiLoiChungTu(w, r, "sửa", domain.ErrTrangThaiDoTuClient)
		return
	}
	if vao.ProjectID != nil {
		h.traLoiLoiChungTu(w, r, "sửa", domain.ErrDuAnBatBien)
		return
	}

	yc := app.YeuCauSuaChungTu{
		NoiDung:   vao.Description,
		DoiTac:    vao.Counterparty,
		SoChungTu: vao.VoucherNo,
		// THE POINTER IS PASSED THROUGH UNTOUCHED, including a pointer to "". Dereferencing it here to
		// "decide" whether the client meant it would collapse "leave alone" and "detach" into one
		// value at the only layer that can still tell them apart.
		NguonVonID: vao.FundingSourceID,
	}
	if vao.PaymentDate != nil {
		ngay, ok := ngayVao(*vao.PaymentDate)
		if !ok || ngay.IsZero() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`payment_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-07.", "")
			return
		}
		yc.NgayChi = &ngay
	}
	if vao.Amount != nil {
		so := domain.Dong(*vao.Amount)
		yc.SoTien = &so
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	sau, err := h.d.GhiChungTu.Sua(r.Context(), r.PathValue("id"), yc, nguoi)
	if err != nil {
		h.traLoiLoiChungTu(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, chungTuRaNgoai(sau))
}

// GoChungTu soft deletes one voucher. DELETE /api/v1/disbursements/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason` — but there is nothing the caller can do with it, and returning it would invite a
// client to display a voucher it has just taken off the screen.
func (h *Handler) GoChungTu(w http.ResponseWriter, r *http.Request) {
	// AN EMPTY BODY IS ACCEPTED HERE, unlike docThan — the same shape as XoaDuAn: the reason is
	// optional, and a client removing a voucher without one may send no body at all. Anything that IS
	// sent must still be valid JSON.
	var vao goChungTuVao
	r.Body = http.MaxBytesReader(w, r.Body, thanToiDa)
	if err := json.NewDecoder(r.Body).Decode(&vao); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiChungTu.Go(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiChungTu(w, r, "gỡ", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// XacNhanChungTu confirms one voucher. POST /api/v1/disbursements/{id}/confirmation
//
// NO REQUEST BODY: confirming carries no information beyond who did it and when, and both come from
// the session and the clock. A body would be a field somebody eventually fills.
func (h *Handler) XacNhanChungTu(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	sau, err := h.d.GhiChungTu.XacNhan(r.Context(), r.PathValue("id"), nguoi)
	if err != nil {
		h.traLoiLoiChungTu(w, r, "xác nhận", err)
		return
	}
	vietJSON(w, http.StatusOK, chungTuRaNgoai(sau))
}

// KhoaChungTu freezes one voucher. POST /api/v1/disbursements/{id}/lockout
func (h *Handler) KhoaChungTu(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	sau, err := h.d.GhiChungTu.Khoa(r.Context(), r.PathValue("id"), nguoi)
	if err != nil {
		h.traLoiLoiChungTu(w, r, "khoá", err)
		return
	}
	vietJSON(w, http.StatusOK, chungTuRaNgoai(sau))
}

// MoKhoaChungTu reopens one frozen voucher. DELETE /api/v1/disbursements/{id}/lockout
//
// IT RETURNS THE VOUCHER (200), unlike the soft delete above, because the caller's next act is on
// the same row: unlocking exists so that a figure can be corrected, and the client needs the state
// it has landed in — `Đã xác nhận` — to know which buttons to draw.
func (h *Handler) MoKhoaChungTu(w http.ResponseWriter, r *http.Request) {
	var vao moKhoaVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	sau, err := h.d.GhiChungTu.MoKhoa(r.Context(), r.PathValue("id"), vao.Reason, nguoi)
	if err != nil {
		h.traLoiLoiChungTu(w, r, "mở khoá", err)
		return
	}
	vietJSON(w, http.StatusOK, chungTuRaNgoai(sau))
}

// projectVouchersOut is the reply of GET /api/v1/investment-projects/{id}/disbursements.
//
// `items` IS NEVER null — a project with no voucher is the ordinary state of a new project, and `[]`
// says so. `count` is len(items): the list is never paged or truncated (fistore.MaxVouchersPerProject
// refuses instead), so the count IS the project's number of live vouchers.
type projectVouchersOut struct {
	ProjectID string      `json:"project_id"`
	Items     []chungTuRa `json:"items"`
	Count     int         `json:"count"`
}

// ListProjectVouchers serves one project's live vouchers, newest payment date first.
// GET /api/v1/investment-projects/{id}/disbursements
//
// WHY IT WAS DEFERRED AND WHY THAT NO LONGER HOLDS: the file header used to say a read route brings
// paging and "what does a commune with 4000 vouchers a year get". This route reads ONE PROJECT's
// vouchers — §8.2's tab — not a commune's year, so there is no page to choose; the ceiling refuses a
// project past any real size rather than truncating it. Nothing in that deferral was about personal
// data: `counterparty` is a company (0004:283-284) and the staff fields are business codes; the
// attachment column `tep_dinh_kem` is NOT read here at all.
//
// A PROJECT OF ANOTHER COMMUNE ANSWERS 404, the same as one that does not exist — the store cannot
// reach it (rule 4, forbidden #2, applied between communes). NO AUDIT ENTRY, for the reason
// DanhSachDuAn gives: the commune's own public-money figures, under `budget.read`.
func (h *Handler) ListProjectVouchers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_argument", "Thiếu mã dự án.", "")
		return
	}
	vouchers, err := h.d.DuAn.VouchersOfProject(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, fistore.ErrKhongThayDuAn):
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy dự án.", "")
		case errors.Is(err, fistore.ErrTooManyVouchers):
			h.d.Log.Error("chứng từ của dự án: vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)))
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		default:
			h.d.Log.Error("chứng từ của dự án: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		}
		return
	}
	out := projectVouchersOut{ProjectID: id, Items: make([]chungTuRa, 0, len(vouchers)), Count: len(vouchers)}
	for _, v := range vouchers {
		item := chungTuRaNgoai(v.Voucher)
		item.FundingSourceName = v.FundingSourceName
		out.Items = append(out.Items, item)
	}
	vietJSON(w, http.StatusOK, out)
}

// thieuChuThe answers a request that reached a guarded write route with no principal.
//
// A 500 AND NOT AN ANONYMOUS WRITE. These routes sit behind authz.RequirePermission, so there is
// always one; arriving here without one means the route was mounted wrong, or identity is older
// than the `ma` field and sent a principal with no business code. Rule 6 does not permit a business
// write whose trail cannot name who made it, and a fallback to the internal id would put two kinds
// of identifier into `audit_log.actor_id` one deployment window at a time, with every test green.
func (h *Handler) thieuChuThe(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi chứng từ giải ngân chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal",
		"Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// traLoiLoiChungTu maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL SIX ROUTES, because six copies of this mapping would drift and the copy that
// drifts is the one answering 500 where it meant 409 — which reads to an operator as a broken
// server rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR EVERY LIFECYCLE REFUSAL: the caller HOLDS the permission and is allowed
// to perform the operation. What is refused is this operation on THIS voucher, because of the state
// the voucher is in or who last acted on it. 403 would send an accountant to the Phân quyền screen
// to be granted a right they already have — and in the self-unlock case, a right that would change
// nothing, because the rule is about the person and not the permission.
//
// THE DOMAIN'S OWN SENTENCE IS RETURNED ON EVERY REFUSAL, deliberately. It names the operation and
// the way out ("phải mở khoá trước (quyền `budget.confirm`)"), holds no personal data and no
// internal detail, and a second sentence written here would drift from it. What must NEVER reach a
// client is the PostgreSQL exception underneath — its text is English, it names a constraint, and
// it says nothing an accountant in a commune can act on.
func (h *Handler) traLoiLoiChungTu(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, fistore.ErrKhongThayChungTu):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy chứng từ này.", "")
	case errors.Is(err, fistore.ErrKhongThayDuAnCuaChungTu):
		// 404 NAMING THE PROJECT, not the voucher: the voucher may well exist. A project of another
		// commune is indistinguishable from one that does not exist, because the query cannot reach
		// it at all (rule 4, forbidden #2, applied between communes).
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy dự án cho chứng từ này.", "")
	case errors.Is(err, fistore.ErrKhongThayNguonVonCuaChungTu):
		// 404 NAMING THE FIELD, exactly as the project case above and for both of its reasons. A
		// funding source of ANOTHER commune is indistinguishable from one that does not exist, because
		// the query cannot reach it at all (rule 1) — so this one answer must cover both, or the status
		// code itself would tell a caller which communes hold which sources.
		//
		// 404 RATHER THAN 400 even on PATCH, where the voucher itself exists: what is missing is a
		// resource the body names, which is the same shape as `project_id`. One rule for both fields
		// means a client does not have to learn which referenced id answers which status.
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy nguồn vốn này trong xã.", "")
	case errors.Is(err, domain.ErrChungTuDaKhoa),
		errors.Is(err, domain.ErrChungTuChuaKhoa),
		errors.Is(err, domain.ErrChungTuDaXacNhan),
		errors.Is(err, domain.ErrChungTuDaKhoaRoi),
		errors.Is(err, domain.ErrTuMoKhoaChungTuMinhVuaKhoa):
		httpx.WriteError(w, http.StatusConflict, "voucher_state", err.Error(), "")
	case errors.Is(err, domain.ErrSelfConfirmation):
		// ITS OWN CODE, the prototype's (`self_confirmation`, service.py:845), so a client can tell
		// "somebody else must confirm this" from "this voucher is in the wrong state". 409 and not 403
		// for the reason above: the caller holds `budget.confirm`; the rule is about the person.
		httpx.WriteError(w, http.StatusConflict, "self_confirmation", err.Error(), "")
	case errors.Is(err, domain.ErrSourceRequired):
		// 409, NOT 400 — the repository's convention for the allocation refusals (8245698b): the body
		// is well formed; what refuses it is the state of THIS project (it has allocation lines). The
		// sentence names the way out.
		httpx.WriteError(w, http.StatusConflict, "source_required",
			"`funding_source_id`: dự án này đã khai nguồn vốn, nên chứng từ phải ghi rõ rút từ nguồn nào "+
				"trong các nguồn đã phân bổ cho dự án.", "")
	case errors.Is(err, domain.ErrSourceNotAllocated):
		// ONE ANSWER for "the project draws on other sources" and "the project declared no source": both
		// say the same thing — this source is not one this project draws on — and the way out is the
		// same screen (the project's `funding_allocations`).
		httpx.WriteError(w, http.StatusConflict, "source_not_allocated",
			"`funding_source_id`: nguồn vốn này không được phân bổ cho dự án. Hãy chọn một nguồn đã phân "+
				"bổ cho dự án, hoặc bỏ trống nếu dự án chưa khai nguồn vốn nào.", "")
	case laLoiDauVaoChungTu(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on;
		// the amount and the description are NOT, because a log line travels into centralised
		// logging across every commune at once.
		h.d.Log.Error("chứng từ giải ngân: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoChungTu reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is the
// client's fault" turns a database outage into a 400, and a client that believes its input is wrong
// retries with different input forever while nobody is told the server is broken.
func laLoiDauVaoChungTu(err error) bool {
	for _, mot := range []error{
		domain.ErrThieuDuAn, domain.ErrDuAnBatBien, domain.ErrTrangThaiDoTuClient,
		domain.ErrSoTienKhongDuong, domain.ErrSoTienQuaLon,
		domain.ErrThieuNgayChi, domain.ErrNgayChiNgoaiLich,
		domain.ErrThieuNoiDung, domain.ErrNoiDungQuaDai,
		domain.ErrDoiTacQuaDai, domain.ErrSoChungTuQuaDai, domain.ErrNguonVonIDSai,
		domain.ErrThieuLyDoMoKhoa, domain.ErrLyDoMoKhoaQuaDai,
		domain.ErrThieuLyDoGo, domain.ErrLyDoGoQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
