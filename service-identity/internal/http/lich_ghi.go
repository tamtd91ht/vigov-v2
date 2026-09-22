package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The WRITE routes behind the commune's working calendar (migration 0006, 14-cau-hinh.md §8:318).
//
//	POST   /api/v1/working-hours              add one session to the ordinary week
//	PATCH  /api/v1/working-hours/{id}         move a session
//	DELETE /api/v1/working-hours/{id}         soft delete, reason mandatory
//	POST   /api/v1/working-hours/defaults     sow Mon–Fri, morning and afternoon
//	POST   /api/v1/public-holidays            add one closure date
//	PATCH  /api/v1/public-holidays/{id}
//	DELETE /api/v1/public-holidays/{id}
//	POST   /api/v1/public-holidays/defaults   sow the FIXED-DATE holidays of one year
//	POST   /api/v1/swap-working-days          add one swap-day session
//	PATCH  /api/v1/swap-working-days/{id}
//	DELETE /api/v1/swap-working-days/{id}
//
// =================================================================================================
// ALL ELEVEN DECLARE `admin.sla`, AND NO KEY IS INVENTED.
//
// `admin.sla` — "Cấu hình thời hạn xử lý" — already exists in the `quyen` table (migration
// 0001:277). Rule 5, invariant 3c: a key no migration seeds is a right no administrator can grant,
// so a route declaring an invented key answers 403 to EVERY account forever while its tests stay
// green, because a fake checker grants any string.
//
// WHY THIS KEY AND NOT A NEW `admin.calendar`. The calendar is not a separate subject: a deadline
// is counted in WORKING HOURS (ADR 0007), and 14-cau-hinh.md §8 — the "Thời hạn xử lý" tab — names
// these three tables at :318 as what "giờ làm việc" means. Somebody configuring how long the
// commune has to answer a citizen and somebody configuring when the commune is open are the same
// person doing the same job, and rule 5, invariant 3b is explicit that these rights are not a
// Cartesian product to be generated. A second key would also have to be SEEDED, which is a
// migration on the permission catalogue and a finding for open question #27 — not something a route
// may decide.
//
// THE READS STAY AnyAuthenticated AND THAT ASYMMETRY IS DELIBERATE, not an oversight to tidy. Office
// hours and holidays sit under every deadline PRINTED ON A SCREEN — the due date on a task, the
// "còn mấy ngày" chip — so a configuration permission on the READ would empty those screens for
// everybody who is not an administrator (routes.go states the trade-off in full). Writing is the
// opposite: it is the act that moves a commune's commitments, and exactly one job does it.
//
// =================================================================================================
// WHAT THESE ROUTES DO NOT DO, AND MUST NEVER DO.
//
// They do not soften a single refusal on the deadline path. An empty calendar still makes
// grpc.AdvanceWorkingHours answer FAILED_PRECONDITION; a calendar that contradicts itself still
// refuses; a swap day on a working weekday still refuses. What these routes add is a way for the
// calendar to STOP BEING EMPTY. A fallback wired anywhere near the deadline path would re-create
// exactly the defect migration 0006:56 spends a paragraph refusing — a commitment invented by
// software and then told to a citizen (rule 10, forbidden #3).
//
// They do not touch a deadline already issued. `han_tiep_nhan` and `han_xu_ly_xong` are columns on
// records in `service-petitions` and `service-documents`, fixed once at the act that set them
// (rule 10, invariant 2; ADR 0028), and this service cannot write to those schemas at all (rule 2,
// forbidden #2). 14-cau-hinh.md:289 says the same from the specification's side.

// thanLichToiDa bounds every request body on this surface. 4 KiB is far past a weekday and two
// times and far short of anything worth streaming: an unbounded body is memory a client chooses, on
// a process serving 200+ communes.
const thanLichToiDa = 4 << 10

// docThanLich decodes a JSON body, answering 400 itself on failure.
//
// THE DECODER'S OWN MESSAGE NEVER REACHES THE CLIENT: it quotes the offending input (rule 3,
// forbidden #3). The sentence returned says what to fix without repeating what was sent.
func docThanLich(w http.ResponseWriter, r *http.Request, vao any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanLichToiDa)
	if err := json.NewDecoder(r.Body).Decode(vao); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// docGioBatBuoc and docGioTuyChon parse a time of day off the wire.
//
// THE SAME PARSER FOR BOTH, AND IT IS THE DOMAIN'S (domain.DocGioTrongNgay). A second parser here
// would accept a shape the domain refuses, or refuse one it accepts, and the disagreement would
// show up as a session the commune saved and the deadline function could not read.
func docGioBatBuoc(s string) (domain.GioTrongNgay, error) { return domain.DocGioTrongNgay(s) }

func docGioTuyChon(s *string) (*domain.GioTrongNgay, error) {
	if s == nil {
		return nil, nil
	}
	g, err := domain.DocGioTrongNgay(*s)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// xoaLichVao is the body of every DELETE on this surface.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where it would land in every access log and proxy cache as a
// free-text sentence somebody typed about a government record. Same shape as
// DELETE /api/v1/document-types/{id} in service-documents.
type xoaLichVao struct {
	Reason string `json:"reason"`
}

// gieoLichRa is what one seeding run did. See app.KetQuaGieoLich for why three counts and not a
// boolean.
type gieoLichRa struct {
	// Seeded is how many rows this request actually wrote.
	Seeded int `json:"seeded"`

	// Kept is how many seed rows the commune ALREADY HAD — left exactly as they were, including any
	// hours the commune edited and any row the commune deliberately removed. `kept` rather than
	// `skipped` on purpose: the screen has to be able to say "chúng tôi không đụng vào 10 dòng của
	// xã".
	Kept int `json:"kept"`

	// Skipped is how many seed rows were NOT written because writing them would have broken the
	// calendar — a session overlapping one the commune already has, or a holiday on a date the
	// commune has declared a swap working day. It is a separate count from `kept` because the
	// screen owes the person a different sentence: `kept` means "you already had it", `skipped`
	// means "we did not write it, and you should look at why".
	Skipped int `json:"skipped"`
}

func gieoLichRaNgoai(kq app.KetQuaGieoLich) gieoLichRa {
	return gieoLichRa{Seeded: kq.DaGieo, Kept: kq.DaCo, Skipped: kq.BoQua}
}

// ---------------------------------------------------------------------------------------------
// lich_lam_viec — the ordinary week.
// ---------------------------------------------------------------------------------------------

// themCaLamViecVao is the body of POST /api/v1/working-hours.
//
// THE FIELD NAMES MATCH caLamViecRa EXACTLY — `weekday`, `start`, `end`, `note` — so a client reads
// a row, edits it and sends it back without a translation table. `weekday` is ISO 8601: 1 = Monday
// … 7 = Sunday, which is NOT what JavaScript's `getDay()` returns; the refusal sentence says so,
// because that off-by-one moves every session by a day and produces no error anywhere.
type themCaLamViecVao struct {
	Weekday int    `json:"weekday"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Note    string `json:"note"`
}

// suaCaLamViecVao is a PARTIAL edit: an ABSENT field means "leave it alone", which is why every
// field is a pointer. With plain values this could not tell "not mentioned" from "sent zero", and
// zero is a legal time (midnight) and an illegal weekday.
type suaCaLamViecVao struct {
	Weekday *int    `json:"weekday"`
	Start   *string `json:"start"`
	End     *string `json:"end"`
	Note    *string `json:"note"`
}

// ThemCaLamViec adds one session to the commune's week. POST /api/v1/working-hours
//
// 201 WITH THE ROW, because this request DOES create one addressable resource and the client needs
// the id to edit or remove it.
func (h *Handler) ThemCaLamViec(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao themCaLamViecVao
	if !docThanLich(w, r, &vao) {
		return
	}
	batDau, err := docGioBatBuoc(vao.Start)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ca làm việc", err)
		return
	}
	ketThuc, err := docGioBatBuoc(vao.End)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ca làm việc", err)
		return
	}

	c, err := h.d.GhiLichLamViec.ThemCa(r.Context(), app.YeuCauThemCa{
		Thu: vao.Weekday, BatDau: batDau, KetThuc: ketThuc, GhiChu: vao.Note,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ca làm việc", err)
		return
	}
	vietJSON(w, http.StatusCreated, caLamViecRaNgoai(c))
}

// SuaCaLamViec moves one session. PATCH /api/v1/working-hours/{id}
//
// IT CHANGES NO DEADLINE ALREADY ISSUED — 14-cau-hinh.md:289, rule 10 invariant 2. Nothing in this
// service can reach `han_tiep_nhan` or `han_xu_ly_xong`.
func (h *Handler) SuaCaLamViec(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao suaCaLamViecVao
	if !docThanLich(w, r, &vao) {
		return
	}
	// EVERY FIELD ABSENT IS REFUSED RATHER THAN TREATED AS A NO-OP. `{}` means the client sent a
	// form it failed to read, and answering 200 would tell the person their edit was saved.
	if vao.Weekday == nil && vao.Start == nil && vao.End == nil && vao.Note == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có trường nào được gửi lên để sửa.", "")
		return
	}
	batDau, err := docGioTuyChon(vao.Start)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ca làm việc", err)
		return
	}
	ketThuc, err := docGioTuyChon(vao.End)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ca làm việc", err)
		return
	}

	c, err := h.d.GhiLichLamViec.SuaCa(r.Context(), r.PathValue("id"), app.YeuCauSuaCa{
		Thu: vao.Weekday, BatDau: batDau, KetThuc: ketThuc, GhiChu: vao.Note,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ca làm việc", err)
		return
	}
	vietJSON(w, http.StatusOK, caLamViecRaNgoai(c))
}

// XoaCaLamViec soft deletes one session. DELETE /api/v1/working-hours/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason` — but there is nothing the caller can do with it, and returning it would invite a
// client to display a row it has just removed from the screen.
func (h *Handler) XoaCaLamViec(w http.ResponseWriter, r *http.Request) {
	h.xoaMotDongLich(w, r, "xoá ca làm việc", func(id, lyDo string, nguoi app.NguoiThucHien) error {
		return h.d.GhiLichLamViec.XoaCa(r.Context(), id, lyDo, nguoi)
	})
}

// GieoCaLamViecMacDinh sows the ordinary week. POST /api/v1/working-hours/defaults
//
// THE HOURS ARE domain.BoGieoCaLamViec's AND THE ARGUMENT FOR THEM IS OWNED THERE — including the
// fact that NO SPECIFICATION STATES THEM and that they are an INITIAL VALUE the commune edits, not
// a constant of the software. Not restated here: a second copy would drift (rule 9).
//
// 200 AND NOT 201, INCLUDING ON THE FIRST RUN. The request creates no single addressable resource —
// it brings a commune's calendar to a known state — and the body says how much of it moved. A 201
// would owe a Location, and there is no one URL to point at.
func (h *Handler) GieoCaLamViecMacDinh(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	kq, err := h.d.GhiLichLamViec.GieoTuanMacDinh(r.Context(), nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "gieo lịch làm việc mặc định", err)
		return
	}
	vietJSON(w, http.StatusOK, gieoLichRaNgoai(kq))
}

// ---------------------------------------------------------------------------------------------
// ngay_nghi_le — the closure dates.
// ---------------------------------------------------------------------------------------------

// themNgayNghiLeVao mirrors ngayNghiLeRa: `date` is `YYYY-MM-DD`, a DAY with no instant and no
// zone. An ISO-8601 timestamp is refused, and the difference is a day rather than a formatting
// taste: "2026-09-02T00:00:00Z" rendered anywhere west of London is the 1st, and a holiday that
// shifts by one day is a deadline counted through a day the office was shut.
type themNgayNghiLeVao struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

type suaNgayNghiLeVao struct {
	Date *string `json:"date"`
	Name *string `json:"name"`
}

// gieoNgayNghiLeVao is the body of POST /api/v1/public-holidays/defaults.
//
// THE YEAR IS MANDATORY AND "this year" IS NOT A DEFAULT THIS CODE GETS TO PICK — the same refusal
// docNamTruyVan makes on the read routes, for the same reason: on 31/12 a screen asking to seed
// "the holidays" would silently switch year at midnight, and a client that never sent the parameter
// would be seeding a different window every January with no line of code changing.
type gieoNgayNghiLeVao struct {
	// A pointer, so "absent" is distinguishable from "0". A bare int would make `{}` mean the year
	// 0, which the range check would then refuse with a message about the range rather than about
	// the missing field.
	Year *int `json:"year"`
}

// ThemNgayNghiLe adds one closure date. POST /api/v1/public-holidays
func (h *Handler) ThemNgayNghiLe(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao themNgayNghiLeVao
	if !docThanLich(w, r, &vao) {
		return
	}
	n, err := h.d.GhiNgayNghiLe.ThemNgayNghi(r.Context(), app.YeuCauThemNgayNghi{
		Ngay: vao.Date, Ten: vao.Name,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ngày nghỉ lễ", err)
		return
	}
	vietJSON(w, http.StatusCreated, ngayNghiLeRaNgoai(n))
}

// SuaNgayNghiLe edits one closure date. PATCH /api/v1/public-holidays/{id}
func (h *Handler) SuaNgayNghiLe(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao suaNgayNghiLeVao
	if !docThanLich(w, r, &vao) {
		return
	}
	if vao.Date == nil && vao.Name == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có trường nào được gửi lên để sửa.", "")
		return
	}
	n, err := h.d.GhiNgayNghiLe.SuaNgayNghi(r.Context(), r.PathValue("id"), app.YeuCauSuaNgayNghi{
		Ngay: vao.Date, Ten: vao.Name,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ngày nghỉ lễ", err)
		return
	}
	vietJSON(w, http.StatusOK, ngayNghiLeRaNgoai(n))
}

// XoaNgayNghiLe soft deletes one closure date. DELETE /api/v1/public-holidays/{id}
func (h *Handler) XoaNgayNghiLe(w http.ResponseWriter, r *http.Request) {
	h.xoaMotDongLich(w, r, "xoá ngày nghỉ lễ", func(id, lyDo string, nguoi app.NguoiThucHien) error {
		return h.d.GhiNgayNghiLe.XoaNgayNghi(r.Context(), id, lyDo, nguoi)
	})
}

// GieoNgayNghiLeMacDinh sows one year's FIXED-DATE public holidays.
// POST /api/v1/public-holidays/defaults
//
// ⚠ FOUR DAYS, NOT ELEVEN, AND THE SCREEN MUST SAY SO. domain.BoGieoNgayNghiLe owns the argument:
// Tết Nguyên đán and Giỗ Tổ Hùng Vương are LUNAR — a hand-written lunar conversion that is one day
// out is a deadline counted through a day the office was shut — and the day beside 02/9 is chosen
// by the Prime Minister each year between 01/9 and 03/9. The commune enters all three from the
// annual announcement, through POST /api/v1/public-holidays.
//
// `seeded: 4` ON ITS OWN READS AS "DONE". Whoever builds this screen owes the person a sentence
// naming what is still missing; the API cannot say it in a count.
func (h *Handler) GieoNgayNghiLeMacDinh(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao gieoNgayNghiLeVao
	if !docThanLich(w, r, &vao) {
		return
	}
	if vao.Year == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần nêu rõ năm cần gieo ngày nghỉ lễ, ví dụ {\"year\": 2026}.", "")
		return
	}
	// THE SAME BOUND THE READ ROUTES USE. An input-sanity bound, not a business rule: the year
	// reaches `make_date($2, 1, 1)`, which happily accepts 99999 and would then scan a window no
	// index can help with. The input is NOT echoed back (rule 3, forbidden #3).
	if *vao.Year < NamNhoNhat || *vao.Year > NamLonNhat {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Năm không hợp lệ — hãy nêu một năm trong khoảng "+
				strconv.Itoa(NamNhoNhat)+"–"+strconv.Itoa(NamLonNhat)+".", "")
		return
	}

	kq, err := h.d.GhiNgayNghiLe.GieoNgayNghiLeMacDinh(r.Context(), *vao.Year, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "gieo ngày nghỉ lễ mặc định", err)
		return
	}
	vietJSON(w, http.StatusOK, gieoLichRaNgoai(kq))
}

// ---------------------------------------------------------------------------------------------
// ngay_lam_bu — the swap working days.
// ---------------------------------------------------------------------------------------------

// themNgayLamBuVao mirrors caLamBuRa. THE HOURS ARE ON THE ROW and are not taken from the weekly
// calendar, precisely because the weekday a swap day falls on usually has no session at all.
type themNgayLamBuVao struct {
	Date  string `json:"date"`
	Start string `json:"start"`
	End   string `json:"end"`
	Name  string `json:"name"`
}

type suaNgayLamBuVao struct {
	Date  *string `json:"date"`
	Start *string `json:"start"`
	End   *string `json:"end"`
	Name  *string `json:"name"`
}

// ThemNgayLamBu adds one swap-day session. POST /api/v1/swap-working-days
//
// THERE IS NO SEED ROUTE FOR THIS TABLE, and that is an answer rather than a gap: a swap day exists
// only because the Prime Minister announced one for a particular year, so there is no fixed set to
// sow (domain.KhongCoNgayLamBuMacDinh). Seeding a guess would make every deadline crossing that
// date come out earlier than the commune's real hours.
func (h *Handler) ThemNgayLamBu(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao themNgayLamBuVao
	if !docThanLich(w, r, &vao) {
		return
	}
	batDau, err := docGioBatBuoc(vao.Start)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ngày làm bù", err)
		return
	}
	ketThuc, err := docGioBatBuoc(vao.End)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ngày làm bù", err)
		return
	}

	c, err := h.d.GhiNgayLamBu.ThemLamBu(r.Context(), app.YeuCauThemLamBu{
		Ngay: vao.Date, BatDau: batDau, KetThuc: ketThuc, Ten: vao.Name,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "thêm ngày làm bù", err)
		return
	}
	vietJSON(w, http.StatusCreated, caLamBuRaNgoai(c))
}

// SuaNgayLamBu edits one swap-day session. PATCH /api/v1/swap-working-days/{id}
func (h *Handler) SuaNgayLamBu(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao suaNgayLamBuVao
	if !docThanLich(w, r, &vao) {
		return
	}
	if vao.Date == nil && vao.Start == nil && vao.End == nil && vao.Name == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có trường nào được gửi lên để sửa.", "")
		return
	}
	batDau, err := docGioTuyChon(vao.Start)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ngày làm bù", err)
		return
	}
	ketThuc, err := docGioTuyChon(vao.End)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ngày làm bù", err)
		return
	}

	c, err := h.d.GhiNgayLamBu.SuaLamBu(r.Context(), r.PathValue("id"), app.YeuCauSuaLamBu{
		Ngay: vao.Date, BatDau: batDau, KetThuc: ketThuc, Ten: vao.Name,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiLich(w, r, "sửa ngày làm bù", err)
		return
	}
	vietJSON(w, http.StatusOK, caLamBuRaNgoai(c))
}

// XoaNgayLamBu soft deletes one swap-day session. DELETE /api/v1/swap-working-days/{id}
func (h *Handler) XoaNgayLamBu(w http.ResponseWriter, r *http.Request) {
	h.xoaMotDongLich(w, r, "xoá ngày làm bù", func(id, lyDo string, nguoi app.NguoiThucHien) error {
		return h.d.GhiNgayLamBu.XoaLamBu(r.Context(), id, lyDo, nguoi)
	})
}

// ---------------------------------------------------------------------------------------------
// The pieces all eleven routes share.
// ---------------------------------------------------------------------------------------------

// xoaMotDongLich is the one shape of a soft-delete handler on this surface.
//
// ONE FUNCTION FOR THREE ROUTES because three copies of "read the actor, read the reason, call the
// use case, answer 204" is three places for one of them to forget the reason — and a delete with no
// reason is a removal nobody can be asked about (rule 7, invariant 1).
func (h *Handler) xoaMotDongLich(w http.ResponseWriter, r *http.Request, viec string,
	goi func(id, lyDo string, nguoi app.NguoiThucHien) error) {

	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var vao xoaLichVao
	if !docThanLich(w, r, &vao) {
		return
	}
	if err := goi(r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiGhiLich(w, r, viec, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// traLoiLoiGhiLich maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ELEVEN ROUTES: eleven copies of this mapping would drift, and the copy that
// drifts answers 500 where it meant 409 — which reads to an operator as a broken server rather than
// as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR EVERY STATE REFUSAL HERE: the caller HOLDS `admin.sla` and is allowed to
// configure the calendar. What is refused is this operation on THIS calendar, because of the state
// the calendar is in. 403 would send an administrator to the Phân quyền screen to be granted a
// right they already have.
func (h *Handler) traLoiLoiGhiLich(w http.ResponseWriter, r *http.Request, viec string, err error) {
	// A DATE THAT IS BOTH A CLOSURE AND A WORKING DAY. The same type, the same 409 and the same
	// sentence the two READ routes already answer with — one fault, one spelling, so the screen
	// that shows the conflict and the screen that refused the write do not describe one state with
	// two words.
	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if errors.As(err, &xungDot) {
		h.traLoiXungDotLich(w, r, xungDot)
		return
	}

	// ADR 0007 DECISION 9 — a swap day on a weekday that already works. The domain's own sentence
	// is returned: it names the date and the weekday, holds no personal data, and says which of the
	// two readings the system refuses to choose between.
	var khongTinhDuoc *domain.LoiKhongTinhDuocHan
	if errors.As(err, &khongTinhDuoc) {
		httpx.WriteError(w, http.StatusConflict, string(khongTinhDuoc.Loai), khongTinhDuoc.Error(), "")
		return
	}

	// TWO SESSIONS THAT OVERLAP. The database cannot refuse this (the EXCLUDE constraint needs
	// `btree_gist` — migration 0006:109), so this refusal is the only thing standing between the
	// commune and a calendar whose overlapping hours are counted twice. The sentence names the row
	// to go and look at, because the two rows look perfectly reasonable one at a time.
	var chongCa *domain.LoiCaChongCaKhac
	if errors.As(err, &chongCa) {
		httpx.WriteError(w, http.StatusConflict, string(domain.VanDeCaChongNhau), chongCa.Error(), "")
		return
	}

	switch {
	case errors.Is(err, idstore.ErrCaLamViecKhongTonTai),
		errors.Is(err, idstore.ErrNgayNghiLeKhongTonTai),
		errors.Is(err, idstore.ErrNgayLamBuKhongTonTai):
		// 404 FOR ANOTHER COMMUNE'S ID, NOT 403. Every statement is scoped, so the id matches no
		// row; an invented id, a soft-deleted row and another authority's row are one single answer,
		// so none can be told apart by trying (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "calendar_row_not_found",
			"Không tìm thấy dòng lịch làm việc này.", "")

	case errors.Is(err, idstore.ErrCaLamViecTrungGioMo):
		// THE SENTENCE SAYS WHY A SESSION THAT IS NOWHERE ON THE SCREEN IS NONETHELESS IN THE WAY.
		// `UNIQUE (tenant_id, thu, bat_dau)` deliberately carries no `WHERE deleted_at IS NULL` — a
		// partial unique index is what lets an issued value be reissued, which rule 7, invariant 3
		// forbids — so a REMOVED session still holds its opening minute. Without this sentence the
		// refusal reads as a bug.
		httpx.WriteError(w, http.StatusConflict, "session_start_taken",
			"Xã đã có một ca bắt đầu đúng giờ này trong thứ này — kể cả khi ca đó đã bị xoá. "+
				"Hãy chọn giờ bắt đầu khác, hoặc sửa ca đang có.", "")

	case errors.Is(err, idstore.ErrNgayNghiLeTrungNgay):
		httpx.WriteError(w, http.StatusConflict, "holiday_date_taken",
			"Xã đã có dòng cho ngày này — kể cả khi dòng đó đã bị xoá. Hãy sửa dòng đang có.", "")

	case errors.Is(err, idstore.ErrNgayLamBuTrungGioMo):
		httpx.WriteError(w, http.StatusConflict, "swap_session_start_taken",
			"Xã đã có một ca làm bù bắt đầu đúng giờ này trong ngày này — kể cả khi ca đó đã bị xoá. "+
				"Hãy chọn giờ bắt đầu khác, hoặc sửa ca đang có.", "")

	case errors.Is(err, idstore.ErrQuaNhieuCaLamViec),
		errors.Is(err, idstore.ErrQuaNhieuNgayNghiLe),
		errors.Is(err, idstore.ErrQuaNhieuNgayLamBu):
		// REFUSED, NOT TRUNCATED, on the write path as well as the read: a decision taken against a
		// partial calendar is a session the check never saw.
		h.d.Log.Error("lịch làm việc vượt trần — TỪ CHỐI ghi thay vì quyết định trên danh sách thiếu",
			"xa", string(tenant.MustFrom(r.Context())), "viec", viec)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")

	case domain.LaLoiDauVaoLich(err):
		// The domain's own sentence is returned: it names the rule, holds no personal data and no
		// internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")

	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is what an operator can act on.
		h.d.Log.Error("ghi lịch làm việc: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
