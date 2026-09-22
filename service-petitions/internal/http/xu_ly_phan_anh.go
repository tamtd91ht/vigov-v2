package http

// The STAFF processing surface of the petition register — the list, and the four acts that move a
// petition through its lifecycle (docs/ui-ux/09 §2, §4, §8).
//
// FIVE ROUTES, FOUR PERMISSIONS, AND EVERY KEY ALREADY EXISTS IN `quyen`:
//
//	GET    /api/v1/citizen-reports                        feedback.read
//	POST   /api/v1/citizen-reports/{maTraCuu}/classification  feedback.classify
//	POST   /api/v1/citizen-reports/{maTraCuu}/assignment      feedback.assign
//	POST   /api/v1/citizen-reports/{maTraCuu}/status          feedback.resolve
//	POST   /api/v1/citizen-reports/{maTraCuu}/closure         feedback.resolve
//
// The first five keys are seeded at service-identity/migrations/0001_init.sql:288-294 and the two
// ADR 0030 added at 0007_quyen_phan_loai_va_xem_day_du.sql:58-59. NO KEY WAS INVENTED (rule 5,
// invariant 3c) — `tools/check_quyen.py` scans the whole repository against the table on every
// `make check`.
//
// ⚠ FINDING, NOT A DECISION: `feedback.resolve` GUARDS TWO ROUTES. The `quyen` table has no key for
// "move the work along", so advancing a petition and closing it are guarded by the same right. A
// commune may well want the officer who processes a petition to be unable to CLOSE one. Inventing a
// key here would produce a route that answers 403 to EVERY account forever while every test stayed
// green, because a fake checker grants any string. It is raised for open question #27 instead.
//
// # THE THREE URL NOUNS, AND WHERE EACH COMES FROM
//
//	closure          ALREADY SETTLED — kb/00-foundation/ubiquitous-language.md maps `dong_phieu` to
//	                 `closure`. Not translated on the spot.
//	classification   } NOT in that table. Both are the nominalisation `.claude/hooks/rest_api_guard`
//	assignment       } itself names (`classify` -> `classification`, `assign` -> `assignment`), so
//	                 the repository's own brain already answers what the mapping table does not.
//	                 ADR 0011 says ASK rather than translate on the spot, so this is a FINDING for
//	                 the owner of that table, exactly as `routings` was for service-documents.
//	                 Changing them is free today and stops being free the day a commune runs.
//	status           a noun, and the state of the resource. docs/ui-ux/09 §13 proposes
//	                 `/trang-thai`, which is the same thing in Vietnamese — a path segment ADR 0011
//	                 forbids.

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// --- request bodies ---------------------------------------------------------------------------

// phanLoaiVao is the body of POST …/{maTraCuu}/classification.
//
// ONE FIELD, AND THE ABSENCES ARE REFUSALS RATHER THAN OMISSIONS. There is no `due_at`: the deadline
// this act fixes is the commune's SLA against the commune's calendar, computed by identity, and a
// client choosing it is a client choosing how long the authority may take. There is no `status`: the
// act moves the petition to `dang-phan-loai` and nowhere else.
type phanLoaiVao struct {
	// Field is the tier-1 field code — `rac-thai`, `an-ninh`, … (docs/ui-ux/09 §5).
	//
	// ⚠ ITS EXISTENCE IS NOT CHECKED. See domain.KiemLinhVuc for the full, measured statement of what
	// that costs and why ADR 0026 stop condition #2 is the thing blocking it.
	Field string `json:"field"`
}

// phanCongVao is the body of POST …/{maTraCuu}/assignment — the "Chuyển xử lý" block of §8.5.
type phanCongVao struct {
	Unit string `json:"unit"`
	// Assignee is optional: "— Để bộ phận phân công —" is a real choice on that screen.
	Assignee string `json:"assignee,omitempty"`
}

// dongPhieuVao is the body of POST …/{maTraCuu}/closure.
//
// `Result` IS MANDATORY AND THE SERVER ENFORCES IT (rule 10, invariant 6). It is the sentence the
// citizen reads when they look their petition up, and the one thing that distinguishes a petition
// that was handled from one that was quietly filed away.
type dongPhieuVao struct {
	Result string `json:"result"`
}

// --- the register list --------------------------------------------------------------------------

// DanhSachPhieu serves one page of the commune's petition register. GET /api/v1/citizen-reports
//
// # THE RESTRICTED FIELD IS DECIDED HERE, AND IT IS DECIDED BEFORE THE QUERY
//
// Petitions in `can-bo` — reports ABOUT a member of staff — are excluded from the page unless the
// caller holds `feedback.restricted` (docs/ui-ux/09 §5 and §14.5). The decision reaches the store as
// a FILTER rather than as a check afterwards, and that is not an optimisation: filtering a page in Go
// after it came back returns short pages (a page of 20 rendering 14) while the cursor has already
// advanced past the rows that were dropped — so an officer sees gaps in a register and has no way to
// know why. The count an officer derives by paging is wrong in the same way.
//
// FAIL CLOSED: the filter's zero value is the CLOSED one. An account with no principal at all, or one
// whose permission lookup fails, gets the narrow register.
//
// # THE REPORTER IS ALWAYS MASKED ON THIS ROUTE, EVEN WITH `feedback.unmask`
//
// That key opens ONE petition at a time on the detail route, and every such read writes an audit
// entry naming the officer and the record (rule 6, invariant 7; ADR 0030). A list cannot honour that
// shape: one request would disclose twenty reporters, and either the ledger gets twenty rows for one
// screen — burying the disclosures it exists to make findable — or it gets one row that does not say
// which citizens were exposed. Neither is an audit trail.
//
// SO THE COST IS STATED RATHER THAN GLOSSED: an officer who needs a reporter's number opens the
// petition. That is one extra click, and it is what makes "who read whose details, and when"
// answerable.
func (h *Handler) DanhSachPhieu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to $1
	// from the context on every statement (rule 1, invariant 5). page.Parse reads only
	// limit/cursor/sort/order, and a cursor carries no commune by construction. NOTHING HERE READS
	// `tenant_id` FROM THE QUERY STRING — a client naming its own commune is a client granting itself
	// access (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// Parsed BEFORE the store is touched: a rejected page request must run no statement at all.
	yc, err := page.Parse(thamSo, petstore.SapXepPhieu)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	loc, err := locPhieuTuQuery(thamSo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	if principal, ok := authz.From(ctx); ok {
		loc.ChoPhepHanChe = h.d.Checker.Allows(ctx, principal, QuyenHanChe)
	}

	kq, err := h.d.DanhSachPhieu.DanhSach(ctx, loc, yc)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT carry the search pattern, the
		// content or a lookup code, and it never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("danh sách phiếu phản ánh: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE LABELS ARE READ ONCE FOR THE WHOLE PAGE, not once per row. The override set is closed at
	// twelve (ADR 0026), so one read and a scan beats twenty queries — and a per-row read would be a
	// second read path to keep in step with the detail route's.
	nhan, err := h.d.NhanLinhVuc.DanhSach(ctx)
	if err != nil {
		h.d.Log.Error("đọc nhãn lĩnh vực cho danh sách: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	theoMa := make(map[string]string, len(nhan))
	for _, n := range nhan {
		theoMa[n.Ma] = n.Nhan
	}

	// page.Result[T] DIRECTLY — tools/apidoc understands it, so there is no second three-field struct
	// copying it and no way for the two to drift.
	//
	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune whose register is
	// empty, never as null. A newly onboarded commune has exactly that, and a client that has to
	// handle both shapes handles one of them wrong.
	ra := page.Result[phieuPhanAnhRa]{
		Items:      make([]phieuPhanAnhRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, p := range kq.Items {
		// `false` IS THE MASKING DECISION AND IT IS A LITERAL. There is no branch here that could
		// become `true` — see the note on this handler.
		ra.Items = append(ra.Items, phieuRaNgoai(p, theoMa[p.LinhVuc], false))
	}
	vietJSON(w, http.StatusOK, ra)
}

// locPhieuTuQuery validates the filters. EVERY VALUE BECOMES A BOUND PARAMETER in the store; nothing
// here builds SQL.
//
// AN UNKNOWN `status` OR `channel` IS REFUSED RATHER THAN IGNORED. A filter silently dropped returns
// the WHOLE register to a screen that asked for one slice of it, and the screen has no way to know —
// which on this surface means an officer believing they are looking at every petition of one kind.
//
// `field`, `hamlet` AND `unit` ARE NOT VALIDATED AGAINST ANY LIST, deliberately: the field code set
// lives in service `platform` and the other two in `identity`, and neither is readable from here
// (ADR 0026 stop condition #2; rule 2, forbidden #2). A code that matches nothing returns an empty
// page, which is a true answer.
func locPhieuTuQuery(q map[string][]string) (petstore.LocPhieu, error) {
	lay := func(k string) string {
		if v, ok := q[k]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}

	var loc petstore.LocPhieu
	if s := lay("status"); s != "" {
		if !domain.TrangThai(s).HopLe() {
			return loc, errTrangThaiPhieuKhongHopLe
		}
		loc.TrangThai = s
	}
	if s := lay("channel"); s != "" {
		if !domain.KenhTiepNhan(s).HopLe() {
			return loc, errKenhKhongHopLe
		}
		loc.Kenh = s
	}
	loc.LinhVuc = lay("field")
	loc.ThonID = lay("hamlet")
	loc.BoPhanID = lay("unit")
	loc.Tim = lay("q")
	if len(loc.Tim) > petstore.TimPhieuToiDa {
		return loc, petstore.ErrTimPhieuQuaDai
	}

	// `late=true` IS THE ONLY ACCEPTED SPELLING, and anything else is refused rather than read as
	// false. A checkbox whose value arrived as `1` or `yes` and was silently dropped shows an officer
	// the whole register while the box on their screen is ticked.
	if s := lay("late"); s != "" {
		if s != "true" {
			return loc, errLocTreHanKhongHopLe
		}
		loc.ChiTreHan = true
	}
	return loc, nil
}

var (
	errTrangThaiPhieuKhongHopLe = errors.New(
		"`status` không phải một trong chín trạng thái của phiếu phản ánh")
	errKenhKhongHopLe = errors.New(
		"`channel` không phải một trong bốn kênh tiếp nhận")
	errLocTreHanKhongHopLe = errors.New(
		"`late` chỉ nhận giá trị `true`; bỏ hẳn tham số nếu không lọc theo trễ hạn")
)

// --- the four acts ---------------------------------------------------------------------------------

// PhanLoaiPhieu settles the field and fixes the resolve deadline.
// POST /api/v1/citizen-reports/{maTraCuu}/classification
func (h *Handler) PhanLoaiPhieu(w http.ResponseWriter, r *http.Request) {
	var vao phanLoaiVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	sau, err := h.d.XuLyPhieu.ChotLinhVuc(r.Context(), r.PathValue("maTraCuu"),
		app.YeuCauChotLinhVuc{LinhVuc: vao.Field}, nguoi)
	if err != nil {
		h.traLoiLoiXuLy(w, r, "phân loại", err)
		return
	}
	h.traPhieu(w, r, sau)
}

// PhanCongPhieu names the department answerable for the petition.
// POST /api/v1/citizen-reports/{maTraCuu}/assignment
func (h *Handler) PhanCongPhieu(w http.ResponseWriter, r *http.Request) {
	var vao phanCongVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	sau, err := h.d.XuLyPhieu.PhanCong(r.Context(), r.PathValue("maTraCuu"),
		app.YeuCauPhanCong{BoPhan: vao.Unit, CanBo: vao.Assignee}, nguoi)
	if err != nil {
		h.traLoiLoiXuLy(w, r, "phân công", err)
		return
	}
	h.traPhieu(w, r, sau)
}

// TienTrangThaiPhieu advances the petition ONE step along the main flow.
// POST /api/v1/citizen-reports/{maTraCuu}/status
//
// NO BODY, AND THAT IS THE DESIGN. A target status on the wire is a client able to skip steps — to
// jump a petition to `da-xu-ly` without anybody working on it — and the intermediate states would
// then be optional in practice while looking mandatory in the lifecycle map. The server holds the
// map; the caller says "advance".
func (h *Handler) TienTrangThaiPhieu(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	sau, err := h.d.XuLyPhieu.TienTrangThai(r.Context(), r.PathValue("maTraCuu"), nguoi)
	if err != nil {
		h.traLoiLoiXuLy(w, r, "chuyển trạng thái", err)
		return
	}
	h.traPhieu(w, r, sau)
}

// DongPhieu closes the petition with a result the citizen can read.
// POST /api/v1/citizen-reports/{maTraCuu}/closure
func (h *Handler) DongPhieu(w http.ResponseWriter, r *http.Request) {
	var vao dongPhieuVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	sau, err := h.d.XuLyPhieu.Dong(r.Context(), r.PathValue("maTraCuu"), vao.Result, nguoi)
	if err != nil {
		h.traLoiLoiXuLy(w, r, "đóng phiếu", err)
		return
	}
	h.traPhieu(w, r, sau)
}

// --- shared -------------------------------------------------------------------------------------

// traPhieu writes the petition back after a successful act.
//
// IT RETURNS THE RECORD (200), because the caller's next act is on the same row: the screen needs the
// status it has landed in and the deadline that was just fixed to know which buttons to draw and what
// to show in the "HẠN XỬ LÝ" box.
//
// THE REPORTER IS MASKED, ALWAYS, ON ALL FOUR WRITE ROUTES. `feedback.unmask` is a right to READ a
// citizen's details, and ADR 0030 attaches an audit entry to every exercise of it — a write route
// returning them unmasked would be a disclosure with no entry, on a path nobody would think to look
// at. An officer who needs the number opens the petition.
func (h *Handler) traPhieu(w http.ResponseWriter, r *http.Request, p domain.PhieuPhanAnh) {
	ctx := r.Context()
	nhan := ""
	if p.LinhVuc != "" {
		var err error
		nhan, err = h.nhanCuaLinhVuc(ctx, p.LinhVuc)
		if err != nil {
			// THE ACT ALREADY COMMITTED. Answering 500 here would tell an officer the classification
			// failed when it did not, and they would do it again — which the second time answers 409
			// because the petition has moved. The label is cosmetic, so the response goes out without
			// it and the failure is logged for an operator.
			h.d.Log.Error("đọc nhãn lĩnh vực sau khi ghi: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
		}
	}
	vietJSON(w, http.StatusOK, phieuRaNgoai(p, nhan, false))
}

// traLoiLoiXuLy maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL FOUR ROUTES, because four copies of this mapping would drift and the copy that
// drifts is the one answering 500 where it meant 409 — which reads to an operator as a broken server
// rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR A STATE REFUSAL: the caller HOLDS the permission and is allowed to perform
// the act. What is refused is this act on THIS petition, because of the state it is in. 403 would
// send an officer to the Phân quyền screen to be granted a right they already have.
func (h *Handler) traLoiLoiXuLy(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, petstore.ErrPhieuKhongTonTai):
		// THE SAME ANSWER AS AN UNKNOWN CODE, AN OTHER COMMUNE'S CODE AND A SOFT-DELETED PETITION —
		// see Handler.khongTimThay. Telling them apart tells somebody trying codes how close they are.
		h.khongTimThay(w)
	case errors.Is(err, petstore.ErrPhieuDaChuyenTrang):
		httpx.WriteError(w, http.StatusConflict, "petition_state",
			"Phiếu đã chuyển sang trạng thái khác trong lúc bạn đang mở màn hình. "+
				"Hãy tải lại phiếu rồi thao tác lại.", "")
	case errors.Is(err, domain.ErrPhanCongSaiLuc):
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "unit")
	case errors.Is(err, domain.ErrDongSaiLuc):
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "")
	case errors.Is(err, domain.ErrKhongConCamKet):
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "")
	case errors.Is(err, app.ErrChuaAnDinhDuocHanXuLy):
		// 409, AND THE SENTENCE NAMES THE SCREEN THAT FIXES IT. This is the ordinary answer in every
		// commune today: `sla` is empty everywhere and the onboarding step that fills it does not
		// exist in this repository. A fallback deadline is refused outright (rule 10, forbidden #3),
		// so the honest thing is to say which configuration is missing.
		httpx.WriteError(w, http.StatusConflict, "sla_chua_cau_hinh",
			"Xã chưa cấu hình thời hạn xử lý cho lĩnh vực này, nên chưa phân loại được. "+
				"Vào Cấu hình → Thời hạn xử lý để đặt số giờ, rồi phân loại lại.", "field")
	case domain.LaLoiXuLyPhanAnh(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		// The wrapped error carries the store or identity failure and never reaches the client (rule
		// 3, forbidden #3). The commune is logged because it is the only thing an operator can act on.
		h.d.Log.Error("xử lý phiếu phản ánh: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// thieuChuTheXuLy answers a request that reached a guarded write route with no principal, or with one
// carrying no business code.
//
// A 500 AND NOT AN ANONYMOUS WRITE. These routes sit behind authz.RequirePermission, so there is
// always a principal; arriving here without one means the route was mounted wrong, or identity is
// older than the `ma` field and sent a principal with no business code. Rule 6 does not permit a
// business write whose trail cannot name who made it, and a fallback to the internal id would put two
// kinds of identifier into `audit_log.actor_id` one deployment window at a time, with every test green
// (rule 6, invariant 8 — measured on 2026-09-22).
func (h *Handler) thieuChuTheXuLy(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến xử lý phiếu chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal",
		"Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
