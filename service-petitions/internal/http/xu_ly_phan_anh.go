package http

// The STAFF processing surface of the petition register — the list, and the four acts that move a
// petition through its lifecycle (docs/ui-ux/09 §2, §4, §8).
//
// FIVE ROUTES, FOUR PERMISSIONS, AND EVERY KEY ALREADY EXISTS IN `quyen`:
//
//	GET    /api/v1/citizen-reports                        feedback.read
//	POST   /api/v1/citizen-reports/{maTraCuu}/classification  feedback.classify
//	POST   /api/v1/citizen-reports/{maTraCuu}/assignment      feedback.assign
//	POST   /api/v1/citizen-reports/{maTraCuu}/status          feedback.read    + the holding rule
//	POST   /api/v1/citizen-reports/{maTraCuu}/closure         feedback.resolve
//	POST   /api/v1/citizen-reports/{maTraCuu}/rejection       feedback.classify  (-> khong-tiep-nhan)
//	POST   /api/v1/citizen-reports/{maTraCuu}/referral        feedback.classify  (-> chuyen-cap-tren)
//
// `rejection` and `referral` are nouns the USER chose on 25/09/2026 (ubiquitous-language.md), and the
// key is `feedback.classify` because both branches leave only from `dang-phan-loai` — they are the
// other outcomes of classification (ADR 0030).
//
// The first five keys are seeded at service-identity/migrations/0001_init.sql:288-294 and the two
// ADR 0030 added at 0007_quyen_phan_loai_va_xem_day_du.sql:58-59. NO KEY WAS INVENTED (rule 5,
// invariant 3c) — `tools/check_quyen.py` scans the whole repository against the table on every
// `make check`.
//
// # THE HOLDING RULE, DECIDED BY THE OWNER ON 2026-09-23 ("theo require")
//
// `…/status` declares `feedback.read` and the condition that decides lives in
// app.duocTienTrangThai: `feedback.resolve` OR being the officer this petition was assigned to. A
// hamlet leader handed one petition must be able to move it, and granting them the commune-wide key
// for that would let them close the neighbouring hamlet's petitions as well.
//
// ⚠ `…/closure` WAS NOT WIDENED WITH IT, AND THE TWO LINES ABOVE DIFFERING IS THE POINT. Closing
// records a result the citizen reads (rule 10, invariant 6) and is exactly what open question #7
// settled on 2026-09-16: "`feedback.resolve` quyết định ai đóng được". The five routes the other
// repository lowered are all WORKING routes; none of them is the closing.
//
// # THE RESTRICTED FIELD, ON ALL FOUR WRITE ROUTES SINCE 2026-09-23
//
// A petition in `can-bo` — a report ABOUT a member of staff — is refused to a caller without
// `feedback.restricted`, AND THE ACT IS REFUSED, not just the response body: the real problem was a
// colleague of the person being reported on classifying, assigning and CLOSING the complaint about
// them. The refusal answers 404, identical to an unknown code, for the reason DocPhieuPhanAnh gives.
//
// Each of the four routes reads ONE fact here — `coQuyenHanChe` — and app.duocChamPhieuHanChe
// decides, inside the transaction, on the locked row. THE ROUTE PERMISSIONS ABOVE ARE UNCHANGED, and
// no key was added: `feedback.restricted` is seeded at service-identity/migrations/0001_init.sql:294.
//
// ⚠ STILL A FINDING, AND UNCHANGED BY THE ABOVE: the `quyen` table has no key meaning "move the work
// along". Open question #27 is still the place that is answered, and inventing a key here would
// produce a route that answers 403 to EVERY account forever while every test stayed green, because a
// fake checker grants any string.
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
	"context"
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

// QuyenXuLyCaXa is the COMMUNE-WIDE right to work on any petition of this commune. It is the key
// `…/closure` is guarded by at the route, and the key `…/status` consults as ONE HALF of the holding
// rule — see app.duocTienTrangThai for the other half.
//
// The key exists in `quyen` and is not invented here — service-identity/migrations/0001_init.sql:293
// seeds it, and the label an administrator reads on the Phân quyền screen is "KẾT THÚC xử lý phản
// ánh". That wording is worth reading twice: the key the customer named is about ENDING the work, so
// gating "move the work along" with it was a mismatch between the string and what the tick box says
// it grants. The holding rule narrows the gap without inventing a key.
//
// It is a constant rather than a literal because it is used with Checker.Allows and
// not inside authz.RequirePermission: tools/apidoc reads the ROUTE declarations and refuses anything
// there that is not a string literal, which is why routes.go spells its keys out and this does not.
const QuyenXuLyCaXa authz.Perm = "feedback.resolve"

// coQuyenHanChe answers the ONE question the four write use cases ask about the caller: does this
// account hold `feedback.restricted`, the key that opens the field `can-bo`?
//
// IT ANSWERS AND DECIDES NOTHING. Which petitions that fact refuses is app.duocChamPhieuHanChe's,
// inside the transaction, on the row read under the lock — the field is a property of the ROW, and a
// decision made here would be made against a row this layer has not read and cannot lock.
//
// FAIL CLOSED: no principal in the context means `false`, never `true`. There is always one behind
// authz.RequirePermission, so this is a precondition rather than a case — but the safe value of a
// permission fact is the one that grants nothing.
func (h *Handler) coQuyenHanChe(ctx context.Context) app.QuyenXemHanChe {
	principal, ok := authz.From(ctx)
	if !ok {
		return false
	}
	return app.QuyenXemHanChe(h.d.Checker.Allows(ctx, principal, QuyenHanChe))
}

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

// khongTiepNhanVao is the body of POST …/{maTraCuu}/rejection.
//
// `Reason` IS MANDATORY AND THE SERVER ENFORCES IT: it is what the citizen reads instead of a result,
// and migration 0011 refuses the status without it.
type khongTiepNhanVao struct {
	Reason string `json:"reason"`
}

// chuyenCapTrenVao is the body of POST …/{maTraCuu}/referral. BOTH fields are mandatory: a citizen
// told their report was passed on must be told WHY and TO WHOM.
type chuyenCapTrenVao struct {
	Reason string `json:"reason"`
	// ReceivingBody is free text — "Điện lực …", "Công an …" — by the user's decision: transfers go
	// sideways as often as up, so there is no catalogue to pick from.
	ReceivingBody string `json:"receiving_body"`
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

	loc, chiGiaoChoToi, err := locPhieuTuQuery(thamSo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	// "GIAO CHO TÔI" — THE CODE COMES FROM THE SESSION, NEVER FROM THE REQUEST. The query said only
	// "my petitions"; WHO "my" is, is `Principal.Ma`, the same business code the holding rule compares
	// against `can_bo_xu_ly_id` (app.duocTienTrangThai).
	//
	// FAIL CLOSED ON AN EMPTY CODE: dropping the filter would answer "Giao cho tôi" with the whole
	// register, a screen showing every petition under a tab that claims to be one officer's workload.
	// An empty `Ma` is identity older than the field — a deployment fault, answered as the write
	// routes answer it (thieuChuTheXuLy), and the store is not reached.
	if chiGiaoChoToi {
		p, ok := authz.From(ctx)
		if !ok || p.Ma == "" {
			h.thieuChuTheXuLy(w, r)
			return
		}
		loc.CanBoXuLyID = p.Ma
	}
	// THE SAME FACT THE FOUR WRITE ROUTES HAND DOWN, read through the same helper (rule 9,
	// invariant 2). Two readings of one permission are two readings that can end up consulting two
	// keys, and the one that would drift is whichever is edited second.
	loc.ChoPhepHanChe = bool(h.coQuyenHanChe(ctx))

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
//
// `scope` IS A SWITCH AND CARRIES NO IDENTITY. `mine` returns chiGiaoChoToi=true and the HANDLER
// fills the code from the session; this function never sets LocPhieu.CanBoXuLyID and reads no key
// that could name an officer, so `?assignee=CB-…` or any other invented parameter never reaches the
// store. UNLIKE THE TASK LIST, which accepts `assignee` as a plain filter: this route exposes no
// "pick an officer" filter, and adding one is a product decision, not a spelling. Absent or `all`
// means the whole commune ("Toàn xã"). Any other value is refused: silently widening a tab labelled
// "Giao cho tôi" to the whole register is the dropped-filter failure described above.
func locPhieuTuQuery(q map[string][]string) (loc petstore.LocPhieu, chiGiaoChoToi bool, err error) {
	lay := func(k string) string {
		if v, ok := q[k]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}

	if s := lay("status"); s != "" {
		if !domain.TrangThai(s).HopLe() {
			return loc, false, errTrangThaiPhieuKhongHopLe
		}
		loc.TrangThai = s
	}
	if s := lay("channel"); s != "" {
		if !domain.KenhTiepNhan(s).HopLe() {
			return loc, false, errKenhKhongHopLe
		}
		loc.Kenh = s
	}
	loc.LinhVuc = lay("field")
	loc.ThonID = lay("hamlet")
	loc.BoPhanID = lay("unit")
	loc.Tim = lay("q")
	if len(loc.Tim) > petstore.TimPhieuToiDa {
		return loc, false, petstore.ErrTimPhieuQuaDai
	}

	// `late=true` IS THE ONLY ACCEPTED SPELLING, and anything else is refused rather than read as
	// false. A checkbox whose value arrived as `1` or `yes` and was silently dropped shows an officer
	// the whole register while the box on their screen is ticked.
	if s := lay("late"); s != "" {
		if s != "true" {
			return loc, false, errLocTreHanKhongHopLe
		}
		loc.ChiTreHan = true
	}

	// §4's scope tabs, SPELLED AS GET /api/v1/tasks SPELLS THEM (locNhiemVuTuQuery): one screen
	// family, one vocabulary, so the web client does not learn two words for "Giao cho tôi". `all` is
	// the default and needs no predicate.
	switch s := lay("scope"); s {
	case "", "all":
	case "mine":
		chiGiaoChoToi = true
	case "related":
		// REFUSED, WITH THE REASON. Who counts as "related" to a petition — and whether a related
		// person may only log and not act — is undecided with the customer. Answering with a guess
		// would put petitions on an officer's screen under a rule nobody made.
		return loc, false, errPhamViLienQuanChuaCo
	default:
		return loc, false, errPhamViKhongHopLe
	}
	return loc, chiGiaoChoToi, nil
}

var (
	errTrangThaiPhieuKhongHopLe = errors.New(
		"`status` không phải một trong chín trạng thái của phiếu phản ánh")
	errKenhKhongHopLe = errors.New(
		"`channel` không phải một trong bốn kênh tiếp nhận")
	errLocTreHanKhongHopLe = errors.New(
		"`late` chỉ nhận giá trị `true`; bỏ hẳn tham số nếu không lọc theo trễ hạn")
	// errPhamViKhongHopLe (the unknown-`scope` refusal) is SHARED with the task list — nhiem_vu.go —
	// because the accepted values are the same two words.
	errPhamViLienQuanChuaCo = errors.New(
		"`scope=related` chưa dùng được với phiếu phản ánh: thế nào là \"liên quan\" và người liên quan " +
			"được làm gì chưa được chốt")
)

// --- the four acts ---------------------------------------------------------------------------------

// PhanLoaiPhieu settles the field and fixes the resolve deadline.
// POST /api/v1/citizen-reports/{maTraCuu}/classification
//
// THE RESTRICTED FACT GOES DOWN WITH EVERY ONE OF THE FOUR ACTS, and this one is the act where its
// absence bit hardest: without it, an account holding `feedback.classify` could re-file a report
// about a member of staff under any other field and make it visible to the whole register.
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
	ctx := r.Context()
	sau, err := h.d.XuLyPhieu.ChotLinhVuc(ctx, r.PathValue("maTraCuu"),
		app.YeuCauChotLinhVuc{LinhVuc: vao.Field}, nguoi, h.coQuyenHanChe(ctx))
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
	ctx := r.Context()
	sau, err := h.d.XuLyPhieu.PhanCong(ctx, r.PathValue("maTraCuu"),
		app.YeuCauPhanCong{BoPhan: vao.Unit, CanBo: vao.Assignee}, nguoi, h.coQuyenHanChe(ctx))
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
//
// # THIS HANDLER ANSWERS ONE QUESTION AND DECIDES NOTHING
//
// The route's gate already refused anyone without `feedback.read`. What is read here is whether the
// caller ALSO holds `feedback.resolve`, the commune-wide right, and that single fact is handed to the
// use case. Whether the act is permitted — the commune-wide right OR being the named assignee — is
// app.duocTienTrangThai's, inside the transaction, on the row read under the lock. Deciding it here
// would mean deciding against an assignee read before the lock, and would leave the rule unreachable
// to every caller that is not an HTTP request.
//
// FAIL CLOSED: no principal in the context means `false`, never `true`. There is always one behind
// authz.RequirePermission, so this is a precondition rather than a case — but the safe value of a
// permission fact is the one that grants nothing.
func (h *Handler) TienTrangThaiPhieu(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	var quyen app.QuyenXuLyCaXa
	if principal, co := authz.From(ctx); co {
		quyen = app.QuyenXuLyCaXa(h.d.Checker.Allows(ctx, principal, QuyenXuLyCaXa))
	}
	sau, err := h.d.XuLyPhieu.TienTrangThai(ctx, r.PathValue("maTraCuu"), nguoi, quyen,
		h.coQuyenHanChe(ctx))
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
	ctx := r.Context()
	sau, err := h.d.XuLyPhieu.Dong(ctx, r.PathValue("maTraCuu"), vao.Result, nguoi,
		h.coQuyenHanChe(ctx))
	if err != nil {
		h.traLoiLoiXuLy(w, r, "đóng phiếu", err)
		return
	}
	h.traPhieu(w, r, sau)
}

// KhongTiepNhanPhieu refuses the petition with a reason the citizen can read.
// POST /api/v1/citizen-reports/{maTraCuu}/rejection
func (h *Handler) KhongTiepNhanPhieu(w http.ResponseWriter, r *http.Request) {
	var vao khongTiepNhanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	sau, err := h.d.XuLyPhieu.KhongTiepNhan(ctx, r.PathValue("maTraCuu"), vao.Reason, nguoi,
		h.coQuyenHanChe(ctx))
	if err != nil {
		h.traLoiLoiXuLy(w, r, "không tiếp nhận", err)
		return
	}
	h.traPhieu(w, r, sau)
}

// ChuyenCapTrenPhieu refers the petition to another body, naming it and saying why.
// POST /api/v1/citizen-reports/{maTraCuu}/referral
func (h *Handler) ChuyenCapTrenPhieu(w http.ResponseWriter, r *http.Request) {
	var vao chuyenCapTrenVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	sau, err := h.d.XuLyPhieu.ChuyenCapTren(ctx, r.PathValue("maTraCuu"),
		app.YeuCauChuyenCapTren{LyDo: vao.Reason, CoQuanNhan: vao.ReceivingBody}, nguoi,
		h.coQuyenHanChe(ctx))
	if err != nil {
		h.traLoiLoiXuLy(w, r, "chuyển cấp trên", err)
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
//
// # THE RESTRICTED FIELD IS CHECKED HERE TOO, AND THIS IS THE SECOND LAYER, NOT THE FIRST
//
// app.duocChamPhieuHanChe refuses the ACT, inside the transaction, on the locked row, so for the four
// routes above this branch is unreachable today: a caller without `feedback.restricted` never gets a
// petition back to render. It is written anyway because a FIFTH write route added next year can
// forget to hand the fact down — and on that day this is the only thing left standing between a
// report about a member of staff and one of that person's colleagues.
//
// WHAT IT CAN AND CANNOT DO, SAID PLAINLY RATHER THAN IMPLIED: it withholds the RECORD, not the act.
// A route that forgot to pass the fact down has already committed its write by the time execution
// reaches here. This is damage limitation and is never a substitute for the check in internal/app.
//
// IT ALSO FIRES ON ONE REAL PATH: an officer without the key who CLASSIFIES a petition INTO
// `can-bo`. That act is allowed — see app.duocChamPhieuHanChe — and it succeeds; the answer is 404
// because the record the response would carry is one that officer may no longer read.
func (h *Handler) traPhieu(w http.ResponseWriter, r *http.Request, p domain.PhieuPhanAnh) {
	ctx := r.Context()
	if p.LinhVuc == LinhVucHanChe && !h.coQuyenHanChe(ctx) {
		h.khongTimThay(w)
		return
	}
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
	case errors.Is(err, app.ErrPhieuHanChe):
		// 404 AND NOT 403 — THE FIFTH CAUSE FOLDED INTO THE ONE ANSWER, and it is the same sentence
		// the read route already says (internal/http/phieu_phan_anh.go, on DocPhieuPhanAnh): a 403
		// here would confirm that a report about a member of staff exists under this code, TO A
		// COLLEAGUE OF THAT PERSON, and the existence of such a report is precisely what the
		// restriction protects. Fail closed, and answer what an unknown code answers.
		//
		// IT IS DELIBERATELY NOT GROUPED WITH ErrKhongPhaiNguoiDuocGiao BELOW, which is a 403. That
		// one refuses a caller who may SEE the petition and may not move it, so naming the refusal
		// discloses nothing new. This one refuses a caller who may not know the petition is there.
		h.khongTimThay(w)
	case errors.Is(err, petstore.ErrPhieuDaChuyenTrang):
		httpx.WriteError(w, http.StatusConflict, "petition_state",
			"Phiếu đã chuyển sang trạng thái khác trong lúc bạn đang mở màn hình. "+
				"Hãy tải lại phiếu rồi thao tác lại.", "")
	case errors.Is(err, domain.ErrPhanCongSaiLuc):
		// THAM SỐ THỨ NĂM LÀ `traceID`, KHÔNG PHẢI TÊN TRƯỜNG. Chỗ này từng truyền `"unit"`, nên
		// chuỗi ấy đi ra dây trong `trace_id` — trường người trực dùng để tìm lại một yêu cầu
		// trong nhật ký lúc có sự cố. Một `trace_id` bằng `"unit"` không tìm được gì, và tệ hơn
		// là nó TRÔNG NHƯ một trace id thật nên người tìm sẽ tin rồi đi tìm.
		//
		// `httpx.Error` hiện KHÔNG có trường `field`. Muốn trả tên trường cho biểu mẫu thì đó là
		// một thay đổi ở `core/httpx` cho cả tám dịch vụ, không phải một đối số truyền lén ở đây.
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "")
	case errors.Is(err, domain.ErrDongSaiLuc):
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "")
	case errors.Is(err, domain.ErrKetThucNhanhSaiLuc):
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "")
	case errors.Is(err, domain.ErrKhongConCamKet):
		httpx.WriteError(w, http.StatusConflict, "petition_state", err.Error(), "")
	case errors.Is(err, app.ErrKhongPhaiNguoiDuocGiao):
		// 403 AND NOT 409, WHICH IS THE OPPOSITE CALL FROM EVERY CASE AROUND IT. The three above
		// refuse an act by somebody who HOLDS the right; this one refuses the caller themselves. A 409
		// would tell an officer to reload a petition they were never allowed to move, and would hide a
		// permission problem behind a sentence about state.
		//
		// THE SENTENCE NAMES NEITHER THE ASSIGNEE NOR THE STATUS. Who is holding a petition is
		// routing information, and telling a caller who was refused exactly who has it invites going
		// round the refusal by asking that person — the point of the rule is that the record decides.
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Phiếu này không được phân công cho bạn, và tài khoản của bạn không có quyền xử lý "+
				"phản ánh của cả xã.", "")
	case errors.Is(err, app.ErrChuaAnDinhDuocHanXuLy):
		// 409, AND THE SENTENCE NAMES THE SCREEN THAT FIXES IT. This is the ordinary answer in every
		// commune today: `sla` is empty everywhere and the onboarding step that fills it does not
		// exist in this repository. A fallback deadline is refused outright (rule 10, forbidden #3),
		// so the honest thing is to say which configuration is missing.
		httpx.WriteError(w, http.StatusConflict, "sla_chua_cau_hinh",
			"Xã chưa cấu hình thời hạn xử lý cho lĩnh vực này, nên chưa phân loại được. "+
				// `""` chứ không `"field"` — tham số thứ năm là `traceID`, xem ghi chú ở nhánh
				// `ErrPhanCongSaiLuc` bên trên.
				"Vào Cấu hình → Thời hạn xử lý để đặt số giờ, rồi phân loại lại.", "")
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
