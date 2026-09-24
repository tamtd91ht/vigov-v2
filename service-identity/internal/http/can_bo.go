package http

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The two read routes of the staff register. GET /api/v1/staff and GET /api/v1/staff/{id}.
//
// THE WRITE ROUTES LIVE IN can_bo_ghi.go, added on 2026-09-22 once the customer answered #10,
// #13, #14, #15 and #16. They share this file's `canBoTomTat` and `raNgoai`, which is what keeps
// "what a staff record looks like on the wire" a single decision: a write route returning a shape
// of its own is how a screen ends up rendering two different versions of one row.
//
// ONE WRITE PATH IS STILL ABSENT ON PURPOSE — the soft delete of #10. It carries its own
// permission, and no key in the `quyen` table means it. See the header of can_bo_ghi.go.

// canBoTomTat is one staff record as it leaves the API. JSON field names are English
// (rest-api-design §1); the values are whatever the commune typed, in Vietnamese.
//
// WHAT IS NOT HERE IS PART OF THE CONTRACT: there is no password field of any kind, and there is
// no route by which one could appear — the store does not even SELECT mat_khau_hash, and
// domain.CanBoTomTat has nowhere to put it. That is three independent layers, which is what it
// takes for "a credential never leaves this service" to be a property rather than a habit.
type canBoTomTat struct {
	ID       string `json:"id"`   // ULID — what GET /api/v1/staff/{id} takes
	Code     string `json:"code"` // cb.Ma — the code the audit trail quotes
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Position string `json:"position"`

	// Ids, not labels. The screen already holds the department and role lists for its own filter
	// boxes; resolving the label here would mean a join, and the paged read walks one table.
	DepartmentID string `json:"department_id"`
	RoleID       string `json:"role_id"`

	// TWO NUMBERS, BECAUSE THEY ARE TWO KINDS OF DATA IN LAW (#16). Neither is masked on this
	// surface, and the reason is #11 rather than convenience — see soRaManHinhNoiBo.
	//
	//	phone   `dien_thoai_co_quan`, the office landline. Duty information.
	//	mobile  `di_dong_ca_nhan`, the person's own mobile. Personal data under Decree 13/2023.
	Phone  string `json:"phone"`
	Mobile string `json:"mobile"`

	// HasAccount separates "this person can sign in" from "this account is not locked". They are
	// two questions (migration 0003), and a client that reads one for the other reports a
	// directory entry as an active account — a figure a commune sends upward.
	HasAccount bool `json:"has_account"`
	Active     bool `json:"active"`

	// Null means never signed in. The screen shows "Chưa đăng nhập" for it; a zero timestamp
	// would render as some date in year 1.
	LastLoginAt *time.Time `json:"last_login_at"`

	CreatedAt time.Time `json:"created_at"`

	// HasZalo — "Có Zalo" under the mobile (migration 0010 §1). It describes `mobile`, so it is on
	// this surface for the same reason `mobile` is (#11).
	HasZalo bool `json:"has_zalo"`

	// The Mini App publication (#12). What the staff screen needs to draw the toggle and the
	// "đã ghi nhận đồng ý lúc …" line. Null on either nullable field is meaningful:
	//
	//	display_order        null = no explicit order (0 is a real position).
	//	consent_recorded_at  null = no consent on record — always the case when published is false.
	Published         bool       `json:"published"`
	DisplayOrder      *int       `json:"display_order"`
	ConsentRecordedAt *time.Time `json:"consent_recorded_at"`
}

// soRaManHinhNoiBo decides what a staff telephone number looks like on the way out of THIS
// surface — the commune's own staff screens.
//
// IT IS NOT MASKED, AND THAT IS A CUSTOMER DECISION TAKEN ON 2026-09-22, NOT A RELAXATION.
// Open question #11, answered in full: staff of the same commune are NOT shown a masked number,
// because they have to ring each other to do their work — mask it and they pass the number
// through a private channel instead, where the authority has neither a trail nor any control.
// The scope stays shut regardless: `Scoped` binds `tenant_id` (rule 1, invariant 5), so this is
// one commune's directory shown to that commune's own staff, behind `admin.user`.
//
// THE DECISION EXPLICITLY DID NOT CREATE A PERMISSION KEY ("KHÔNG ĐẶT KHOÁ MỚI nào"), which is
// why there is no Checker call here and why #27 is not blocking this route.
//
// WHAT #11 DID NOT OPEN, AND WHAT THIS FUNCTION'S NAME IS FOR: the same decision keeps the mask on
// EXCEL EXPORTS and on anything published outside the authority (rule 3, invariant 4), and #12
// keeps the mobile off the Mini App until that person's own consent is recorded (migration 0010;
// written by PUT /api/v1/staff/{id}/publication). Neither the export nor a public Mini App READ
// exists in this service today. When one is written
// it must NOT reuse this function; the name says which surface this is, so that reuse has to be a
// decision somebody takes rather than an import somebody copies.
//
// IT IS STILL A FUNCTION AND NOT A DELETED LINE, so there is exactly one place to change if the
// customer revisits #11 — and one place for a reader to find the argument.
func soRaManHinhNoiBo(so string) string { return so }

// raNgoai converts one record for the wire. IT IS THE ONLY EXIT — for the two read routes AND for
// the five write routes in can_bo_ghi.go — which is what makes the decision above enforceable:
// there is no second place that builds this shape, so a masking rule cannot hold on the list and
// be forgotten on the reply to an edit.
//
// ho_ten, chuc_vu and the department are NOT masked. `admin.user` — "Quản lý người dùng" — is an
// explicit permission that guards exactly this screen (14-cau-hinh.md §12.8), and a register of
// staff whose names are masked is not a register of staff. `email` is not masked either: it is a
// work address issued by the authority, the same reading identity/internal/app/
// dang_nhap.go:81 takes when it explains that a staff work address is not citizen personal data
// (what it guards there is an UNAUTHENTICATED, client-supplied string, which this is not).
func raNgoai(cb domain.CanBoTomTat) canBoTomTat {
	return canBoTomTat{
		ID:           cb.ID,
		Code:         cb.Ma,
		FullName:     cb.HoTen,
		Email:        cb.Email,
		Position:     cb.ChucVu,
		DepartmentID: cb.BoPhanID,
		RoleID:       cb.VaiTroID,
		Phone:        soRaManHinhNoiBo(cb.DienThoaiCoQuan),
		Mobile:       soRaManHinhNoiBo(cb.DiDongCaNhan),
		HasAccount:   cb.CoTaiKhoan,
		Active:       cb.DangHoatDong,
		LastLoginAt:  cb.DangNhapGanNhat,
		CreatedAt:    cb.TaoLuc,

		HasZalo:           cb.CoZalo,
		Published:         cb.HienTrenMiniApp,
		DisplayOrder:      cb.ThuTuDanhBa,
		ConsentRecordedAt: cb.DongYCongKhaiLuc,
	}
}

// DanhSachCanBo serves one page of the register. GET /api/v1/staff
//
// NO AUDIT ENTRY, and since 2026-09-22 THE ARGUMENT FOR THAT HAS CHANGED — read this before
// assuming it is still the old one.
//
// It used to be simple: rule 6, invariant 7 audits reading FULL personal data, and the number left
// here masked, so nothing full was read. Open question #11 ended that: a staff mobile is now shown
// UNMASKED to the commune's own staff, so this route does return full personal data.
//
// It still records nothing, and the reason is what invariant 7 is FOR. It audits the EXCEPTIONAL
// read — the moment somebody uses a privilege to see more than the screen normally shows. #11
// deliberately created no such privilege ("KHÔNG ĐẶT KHOÁ MỚI nào"): the unmasked view IS the
// ordinary state of this screen for every holder of `admin.user`, so there is no exceptional event
// to record. An entry per page of the directory would add thousands that say "somebody opened the
// staff list" to a ledger kept for years, and bury the entries that carry legal weight — who
// changed a role, who locked an account. A trail nobody can read answers no inspection.
//
// → STATED FOR THE CUSTOMER RATHER THAN DECIDED HERE: #11 settled the masking and said nothing
// about the trail. Rule 6, stop condition #1 is "a new operation where it is unclear whether it
// must be audited". This turn kept the existing behaviour — it did not choose it.
//
// TWO FILTERS ON THE URL SINCE 2026-09-24 (user decision): `unit` (a department id →
// `bo_phan_id`) and `published` (`true`/`false` → `hien_tren_mini_app`). Neither is personal data,
// so the URL is the right place for them. THE FREE-TEXT SEARCH IS DELIBERATELY NOT HERE — it is
// POST /api/v1/staff/searches, because the text is usually a name or a number (rule 3, forbidden #4).
func (h *Handler) DanhSachCanBo(w http.ResponseWriter, r *http.Request) {
	// The commune is fixed by httpx.TenantMiddleware from Host; page.Parse reads only
	// limit/cursor/sort/order, and a cursor carries no commune by construction. A client naming
	// its own commune is a client granting itself access — nothing here reads tenant_id from the
	// query string (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// Both filters are validated BEFORE the store is touched, like the page request below.
	loc := domain.LocCanBo{BoPhanID: thamSoLoc(thamSo, "unit")}
	if !kiemBoPhanLoc(w, loc.BoPhanID) {
		return
	}
	switch thamSoLoc(thamSo, "published") {
	case "":
	case "true":
		loc.CongKhai = ptrBool(true)
	case "false":
		loc.CongKhai = ptrBool(false)
	default:
		// ONLY the two literal spellings. strconv.ParseBool would also take "1", "t", "TRUE" — a
		// wider contract than the one published, and the next client would come to depend on it.
		// The value is not echoed back: nothing the client sent is repeated into an error.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tham số `published` chỉ nhận true hoặc false.", "")
		return
	}

	// Parsed BEFORE the store is touched: a rejected page request must run no statement at all.
	yc, err := page.Parse(thamSo, idstore.SapXepCanBo)
	if err != nil {
		// page.HTTPError owns the mapping so all eight services answer a bad cursor the same way.
		// It never echoes what the client sent: a cursor is opaque, and a rejected sort key is
		// often a probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	h.traTrangCanBo(w, r, "danh sách cán bộ", loc, yc)
}

// thamSoLoc reads one filter parameter off the query string, trimmed: url.Values' own single-value
// accessor plus a TrimSpace, indexing the map rather than calling that method.
//
// THE SAME SHAPE AS service-comms' thamSoLoc (internal/http/noi_dung_mini_app.go), for the same two
// reasons, and neither is cosmetic:
//
//   - rbac_guard and tenant_scope_guard both key on the NAME of url.Values' accessor method called
//     with a string literal, reading it as a mounted route and as a database call respectively.
//     Both are false alarms on a filter read; indexing the map avoids them without weakening either.
//   - tools/apidoc recognises a PACKAGE-LEVEL function taking url.Values and a string key that it
//     indexes the map with, and reads the parameter name from each CALL SITE — which is how `unit`
//     and `published` reach openapi.json (ledger `_chung/apidoc-ham-cap-goi-doc-tham-so`). A helper
//     taking *http.Request instead would drop both from the contract silently.
func thamSoLoc(q url.Values, ten string) string {
	v, co := q[ten]
	if !co || len(v) == 0 {
		return ""
	}
	return strings.TrimSpace(v[0])
}

// kiemBoPhanLoc validates a department id used as a FILTER, answering 400 itself.
//
// domain.KiemTraIDThamChieu IS THE REPO'S VALIDATOR FOR A CLIENT-SUPPLIED DEPARTMENT ID, and it is
// reused rather than a stricter one invented: it bounds the length, and nothing more, because no
// code path in this repository fixes the FORMAT of `bo_phan.id` — there is no ULID validator in
// core/, and no insert into `bo_phan` exists yet to say ids are ULIDs rather than slugs. A strict
// ULID check here would refuse real departments the day they are provisioned with another shape.
// The value reaches SQL only as a bound parameter, so its characters cannot matter to safety; a
// well-formed id that names no department simply matches nobody, which is the honest answer.
func kiemBoPhanLoc(w http.ResponseWriter, id string) bool {
	if err := domain.KiemTraIDThamChieu(id); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Mã bộ phận dùng để lọc không hợp lệ.", "")
		return false
	}
	return true
}

func ptrBool(v bool) *bool { return &v }

// traTrangCanBo runs one register read and writes the page. SHARED BY THE LIST AND THE SEARCH so
// the two cannot drift in the one place they must agree: the shape on the wire and what a failure
// is allowed to say.
func (h *Handler) traTrangCanBo(w http.ResponseWriter, r *http.Request, viec string, loc domain.LocCanBo, yc page.Request) {
	ctx := r.Context()
	kq, err := h.d.DanhBa.DanhSach(ctx, loc, yc)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT carry a name, an email or a
		// phone number, and it never reaches the client (rule 3, forbidden #3). `loc` is NOT
		// logged: on the search route it holds the text the administrator typed.
		h.d.Log.Error(viec+": lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on an empty commune, never
	// as null. A newly onboarded commune has an empty register, and a client that has to handle
	// both shapes handles one of them wrong.
	// page.Result[canBoTomTat] TRỰC TIẾP, không còn một struct ba trường chép lại nó.
	//
	// Bản sao ấy tồn tại vì tools/apidoc chưa có nhánh cho generic, nên một kiểu trả lời dạng
	// generic sẽ làm tuyến này rơi khỏi hợp đồng REST TRONG IM LẶNG — và web khi ấy gõ tay
	// hình dạng màn hình bằng cách đoán. Nay apidoc hiểu `page.Result[T]`, nên cả bản sao lẫn
	// bài test dùng reflection để ghim nó khỏi trôi đều không còn lý do tồn tại.
	ra := page.Result[canBoTomTat]{
		Items:      make([]canBoTomTat, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, cb := range kq.Items {
		ra.Items = append(ra.Items, raNgoai(cb))
	}
	vietJSON(w, http.StatusOK, ra)
}

// ChiTietCanBo serves one record. GET /api/v1/staff/{id}
//
// 404 FOR ANOTHER COMMUNE'S ID, NOT 403 — and it costs nothing to get right here, because the
// store cannot see the other commune's row at all: Scoped.Query binds the commune from the
// context to $1, so the id matches nothing and comes back as ErrCanBoKhongTonTai. An invented id,
// a soft-deleted person and another authority's staff member are one single answer, so none of
// them can be told apart by trying (rule 4, forbidden #2).
func (h *Handler) ChiTietCanBo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cb, err := h.d.DanhBa.ChiTiet(ctx, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, idstore.ErrCanBoKhongTonTai) {
			httpx.WriteError(w, http.StatusNotFound, "staff_not_found",
				"Không tìm thấy cán bộ.", "")
			return
		}
		h.d.Log.Error("chi tiết cán bộ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, raNgoai(cb))
}
