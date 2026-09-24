package http

// The WRITE routes of the staff register — the first write surface this service exposes that is
// not a session.
//
// SEPARATE ROUTES AND NOT ONE FORM. The specification draws a single dialog carrying name,
// position, department, role and status (14-cau-hinh.md §3), and this deliberately does NOT
// mirror it:
//
//	POST   /api/v1/staff                   add a directory entry
//	PATCH  /api/v1/staff/{id}              correct the profile — no authority changes hands
//	POST   /api/v1/staff/{id}/lockout      the person has retired or transferred (#10)
//	DELETE /api/v1/staff/{id}/lockout      they are back
//	PUT    /api/v1/staff/{id}/role         move them to a role (#13, #14)
//	PUT    /api/v1/staff/{id}/publication  the Mini App directory (#12), `content.update`
//	DELETE /api/v1/staff/{id}              soft-delete a DUPLICATED row (#10), `admin.user.delete`
//
// WHY THE SPLIT COSTS THE WEB A SECOND REQUEST AND IS STILL RIGHT. The customer settled on
// 2026-09-22 that locking somebody and deleting them are DIFFERENT operations with DIFFERENT
// permissions (#10), that the last administrator may not be removed by any of three paths (#13),
// and that nobody may act on their own account or hand out a permission they do not hold (#14).
// Every one of those is a rule about ONE operation. Folded into a single PATCH they would become
// conditionals inside one handler, sharing one audit verb — and a ledger where "the telephone
// number was corrected" and "this person was moved into the role that runs the commune" are the
// same entry cannot answer an inspection without somebody interpreting every delta.
//
// THE SOFT DELETE OF #10 CARRIES ITS OWN KEY, `admin.user.delete`. #10 says the delete is a
// different operation from the lock, with a different permission; `admin.user` ("Quản lý người
// dùng") for both would be exactly the shape #10 refused. The key was absent from the `quyen`
// table until migration 0010 §4 seeded it (ADR 0035, #27: "seed a key only when a real route needs
// it"), and until then the route was deliberately not written — rule 5 invariant 3c: a key no
// migration seeds is a right no administrator can grant.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// thanCanBoToiDa bounds the request body. 16 KiB is far past any of these bodies — six short text
// fields — and far short of anything worth streaming: the point is that an unbounded body is
// memory a client chooses, on a process serving 200+ communes.
const thanCanBoToiDa = 16 << 10

// themCanBoVao is the create body. JSON field names are English (rest-api-design §1); the values
// are whatever the commune typed, in Vietnamese.
//
// WHAT IS ABSENT IS THE CONTRACT, and each absence is a decision:
//
//	code         #15 — the system mints it (domain.SinhMaCanBo). Neither form in the
//	             specification draws the box, and a client-chosen code is a code somebody can point
//	             at an existing person's archival records.
//	role_id      assigning a role is PUT /api/v1/staff/{id}/role, the route that carries #14's two
//	             guards. A role here would be the way around both of them.
//	password,    #9/#17/#18 — issuing an account is a different flow. The INSERT writes
//	has_account  `co_tai_khoan = false` as a literal, so there is no value any layer could pass.
//	active       a row that has just been created is not locked. Locking is its own route, and it
//	             is the route that leaves the entry saying who shut the account.
type themCanBoVao struct {
	FullName    string `json:"full_name"`
	Position    string `json:"position"`
	Email       string `json:"email"`
	OrgUnitID   string `json:"org_unit_id"`
	OfficePhone string `json:"office_phone"`
	Mobile      string `json:"mobile"`
}

// suaCanBoVao is a PARTIAL edit: an ABSENT field means "leave this alone", and that is why every
// field is a pointer. Four of the six have a meaningful empty value — clearing a position, a
// department or either telephone number is a legitimate edit — so a struct of plain strings could
// not tell "not mentioned" from "cleared", and a screen editing only the position would wipe both
// telephone numbers off a government directory with nothing reporting it.
type suaCanBoVao struct {
	FullName    *string `json:"full_name"`
	Position    *string `json:"position"`
	Email       *string `json:"email"`
	OrgUnitID   *string `json:"org_unit_id"`
	OfficePhone *string `json:"office_phone"`
	Mobile      *string `json:"mobile"`

	// HasZalo — "Có Zalo" (migration 0010 §1). Contact information about the mobile, so it is a
	// profile correction under `admin.user`, and it publishes nothing: publication is
	// PUT .../publication under `content.update`. null/absent = unchanged, like every field here.
	HasZalo *bool `json:"has_zalo"`
}

// datCongKhaiVao is the whole body of PUT /api/v1/staff/{id}/publication — the Mini App
// publication state of ONE person (open question #12).
//
//	published          REQUIRED. A pointer so that an absent field is refused rather than read as
//	                   false: an accidental unpublish is not harmless — it clears the consent
//	                   marks, and the person then has to be asked again.
//	consent_confirmed  the administrator's confirmation, for THIS request, that the person was
//	                   asked and agreed. Must be true when published is true; ignored otherwise.
//	display_order      position in the directory, >= 0; null or absent = no explicit order. PUT
//	                   carries the WHOLE state, so omitting it clears a position that was set.
type datCongKhaiVao struct {
	Published        *bool `json:"published"`
	ConsentConfirmed bool  `json:"consent_confirmed"`
	DisplayOrder     *int  `json:"display_order"`
}

// datVaiTroVao is the whole body of the role route. PUT, so it carries the WHOLE state of the
// relationship rather than an instruction — `""` means "no role", which is a legitimate
// destination (`nguoi_dung.vai_tro_id` is nullable, and a person can sit in the org chart holding
// nothing).
type datVaiTroVao struct {
	RoleID string `json:"role_id"`
}

// xoaCanBoVao is the body of DELETE /api/v1/staff/{id}.
//
// A BODY ON A DELETE, the convention DELETE /api/v1/tasks/{ma} set in service-petitions: the reason
// is mandatory (rule 7, invariant 1), and a query string would put free text about a government
// record into every access log and proxy cache. Trimmed, required and bounded by the use case
// (domain.ChuanHoaLyDoXoa), so the rule has one owner.
type xoaCanBoVao struct {
	Reason string `json:"reason"`
}

// docThanCanBo decodes a JSON body, answering 400 itself on failure.
//
// THE DECODER'S OWN MESSAGE NEVER REACHES THE CLIENT. It quotes the offending input, which on this
// surface is a person's name, work address or telephone number (rule 3, forbidden #3). The
// sentence returned says what to fix without repeating what was sent.
func docThanCanBo(w http.ResponseWriter, r *http.Request, vao any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanCanBoToiDa)
	if err := json.NewDecoder(r.Body).Decode(vao); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// nguoiThucHienCanBo builds the actor of the change from the request.
//
// THE TRAIL RECORDS THE BUSINESS CODE `p.Ma`, NEVER THE INTERNAL `p.ID` (rule 6, invariant 8).
// `audit_log.actor_id` is read years later by somebody handling a complaint or an inspection:
// `CB-2026-7K3M9Q` names a person to them with no lookup still alive, a ULID names nobody. The
// policy was written at app/dang_nhap.go:151 long before anything enforced it; what reads it now is
// `.claude/hooks/audit_actor_guard.py` and `tools/check_audit_actor.py`.
//
// BOTH IDENTIFIERS ARE CARRIED, AND THAT IS NOT REDUNDANT. The use case needs `p.ID` for two
// decisions open question #14 settled — "is the target the caller themselves" and "what does the
// caller actually hold" — and neither can be answered from a staff code, because that is not what
// the grants join on. See app.NguoiThucHien.
//
// AN EMPTY `Ma` REFUSES THE WRITE AND NEVER FALLS BACK TO `ID`. Empty happens only when this
// service is talking to an identity older than the `ma` field; a substitute here would put
// internal ids back into the column silently, one deployment window at a time, with every test
// still green.
//
// THE IP COMES FROM THIS PROCESS'S OWN SOCKET. httpx.ClientIP does not trust X-Forwarded-For, and
// rule 6, invariant 2 wants the address the request really arrived from — not one the caller chose
// to name.
//
// A MISSING PRINCIPAL IS A FAILURE, NOT AN ANONYMOUS ENTRY. These routes sit behind
// authz.RequirePermission, so there is always one; reaching here without one means the route was
// mounted wrong, and an entry attributed to nobody is what rule 6 exists to prevent.
func nguoiThucHienCanBo(r *http.Request) (app.NguoiThucHien, bool) {
	p, ok := authz.From(r.Context())
	if !ok || p.ID == "" || p.Ma == "" || p.Kind != "staff" {
		return app.NguoiThucHien{}, false
	}
	return app.NguoiThucHien{
		ID:  p.ID,
		Vet: audit.Actor{ID: p.Ma, Kind: p.Kind, IP: ipTu(r)},
	}, true
}

// ThemCanBo adds one directory entry. POST /api/v1/staff
func (h *Handler) ThemCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than themCanBoVao
	if !docThanCanBo(w, r, &than) {
		return
	}

	cb, err := h.d.GhiDanhBa.Them(r.Context(), app.YeuCauThemCanBo{
		HoTen:           than.FullName,
		ChucVu:          than.Position,
		Email:           than.Email,
		BoPhanID:        than.OrgUnitID,
		DienThoaiCoQuan: than.OfficePhone,
		DiDongCaNhan:    than.Mobile,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiCanBo(w, r, "thêm cán bộ", err)
		return
	}
	vietJSON(w, http.StatusCreated, raNgoai(cb))
}

// SuaCanBo corrects one profile. PATCH /api/v1/staff/{id}
func (h *Handler) SuaCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than suaCanBoVao
	if !docThanCanBo(w, r, &than) {
		return
	}

	cb, err := h.d.GhiDanhBa.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaCanBo{
		HoTen:           than.FullName,
		ChucVu:          than.Position,
		Email:           than.Email,
		BoPhanID:        than.OrgUnitID,
		DienThoaiCoQuan: than.OfficePhone,
		DiDongCaNhan:    than.Mobile,
		CoZalo:          than.HasZalo,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiCanBo(w, r, "sửa hồ sơ cán bộ", err)
		return
	}
	vietJSON(w, http.StatusOK, raNgoai(cb))
}

// KhoaCanBo shuts one account. POST /api/v1/staff/{id}/lockout
func (h *Handler) KhoaCanBo(w http.ResponseWriter, r *http.Request) { h.datKhoaCanBo(w, r, true) }

// MoKhoaCanBo opens it again. DELETE /api/v1/staff/{id}/lockout
func (h *Handler) MoKhoaCanBo(w http.ResponseWriter, r *http.Request) { h.datKhoaCanBo(w, r, false) }

// datKhoaCanBo is both directions. ONE FUNCTION because the two differ in exactly one boolean, and
// two copies would be two places for the #13 refusal to be edited out of.
//
// NO REQUEST BODY IN EITHER DIRECTION, and that is a stated gap rather than a simplification:
// #10 makes the lock the answer to "this person retired or transferred", and the schema has no
// column to record WHICH of the two it was, nor any other reason. Accepting a free-text reason
// that lands only in the audit delta would put an explanation in a place no screen can show, so
// the question "why is this person locked" would have one answer for an inspection and none for
// the commune. Reported to the user instead of half-built.
func (h *Handler) datKhoaCanBo(w http.ResponseWriter, r *http.Request, khoa bool) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}

	viec := "mở khoá tài khoản"
	if khoa {
		viec = "khoá tài khoản"
	}
	cb, err := h.d.GhiDanhBa.DatKhoa(r.Context(), r.PathValue("id"), khoa, nguoi)
	if err != nil {
		h.traLoiLoiGhiCanBo(w, r, viec, err)
		return
	}
	// 200 WITH THE RECORD, NOT 204, INCLUDING ON THE DELETE. The screen redraws the row it just
	// changed; a 204 would make every lock two requests, and the second one could show a row
	// somebody else had edited in between — which reads as the first request having done something
	// it did not.
	vietJSON(w, http.StatusOK, raNgoai(cb))
}

// DoiVaiTroCanBo moves one person to a role. PUT /api/v1/staff/{id}/role
func (h *Handler) DoiVaiTroCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than datVaiTroVao
	if !docThanCanBo(w, r, &than) {
		return
	}

	cb, err := h.d.GhiDanhBa.DoiVaiTro(r.Context(), r.PathValue("id"), than.RoleID, nguoi)
	if err != nil {
		h.traLoiLoiGhiCanBo(w, r, "đổi vai trò cán bộ", err)
		return
	}
	vietJSON(w, http.StatusOK, raNgoai(cb))
}

// DatCongKhaiCanBo publishes one person to the Mini App directory, or takes them off it.
// PUT /api/v1/staff/{id}/publication
//
// THE ACTOR IS BUILT BY nguoiThucHienCanBo LIKE EVERY OTHER WRITE HERE, and for this route that is
// the whole of rule 6 invariant 8 AND of #12's "who recorded the consent": the use case writes
// nguoi.Vet.ID — the staff code p.Ma — into `dong_y_cong_khai_ghi_boi`. An empty p.Ma answers 500
// before the use case runs; there is no fallback to p.ID.
func (h *Handler) DatCongKhaiCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than datCongKhaiVao
	if !docThanCanBo(w, r, &than) {
		return
	}
	if than.Published == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu trường published: phải nói rõ công khai (true) hay không công khai (false).", "")
		return
	}

	cb, err := h.d.GhiDanhBa.DatCongKhai(r.Context(), r.PathValue("id"), app.YeuCauCongKhai{
		CongKhai:       *than.Published,
		DaXacNhanDongY: than.ConsentConfirmed,
		ThuTu:          than.DisplayOrder,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiCanBo(w, r, "đặt công khai Mini App", err)
		return
	}
	vietJSON(w, http.StatusOK, raNgoai(cb))
}

// XoaCanBo soft-deletes one duplicated directory row. DELETE /api/v1/staff/{id}
//
// 204 AND NO BODY: the row is gone from every read path, so there is nothing for the screen to
// redraw — the same answer DELETE /api/v1/tasks/{ma} gives.
func (h *Handler) XoaCanBo(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var than xoaCanBoVao
	if !docThanCanBo(w, r, &than) {
		return
	}

	if err := h.d.GhiDanhBa.Xoa(r.Context(), r.PathValue("id"), than.Reason, nguoi); err != nil {
		h.traLoiLoiGhiCanBo(w, r, "xoá dòng danh bạ nhập trùng", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// thieuNguoiThucHien answers a request whose principal cannot audit a write.
//
// 500 AND NOT 401: these routes sit behind authz.RequirePermission, which has already refused
// every request without a principal, so arriving here means the chain was built wrong or this
// service is talking to an identity that does not send `ma`. Rule 6 does not permit a business
// write whose trail cannot name who made it, and answering 401 would send a member of staff to
// sign in again for a fault that is not theirs.
func (h *Handler) thieuNguoiThucHien(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("ghi danh bạ: không dựng được chủ thể vết kiểm toán",
		"xa", string(tenant.MustFrom(r.Context())))
	httpx.WriteError(w, http.StatusInternalServerError, "internal",
		"Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// traLoiLoiGhiCanBo maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR EVERY WRITE ROUTE: seven copies of this mapping would drift, and the copy that
// drifts answers 500 where it meant 409 — which reads to an operator as a broken server rather
// than as a rule doing its job.
func (h *Handler) traLoiLoiGhiCanBo(w http.ResponseWriter, r *http.Request, viec string, err error) {
	var thieuQuyen *app.LoiTraoQuyenKhongCam

	switch {
	case errors.Is(err, idstore.ErrCanBoKhongTonTai):
		// 404 FOR ANOTHER COMMUNE'S ID, NOT 403, and it costs nothing to get right: every read
		// here is scoped, so the id matches no row. An invented id, a soft-deleted person and
		// another authority's staff member are one single answer, so none of them can be told
		// apart by trying (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "staff_not_found",
			"Không tìm thấy cán bộ.", "")

	case errors.Is(err, app.ErrTuThaoTacChinhMinh):
		// 403, AND THE SENTENCE HAS TO SAY IT IS NOT ABOUT A PERMISSION. Open question #14: the
		// holder of `admin.user` may not act on their own account, because otherwise that one key
		// stands above every other — its holder can put themselves into the strongest role in the
		// commune, and nothing in the system would record that as unusual. Granting the caller
		// another permission would not change this answer, so the message must not send them to
		// the Phân quyền screen.
		httpx.WriteError(w, http.StatusForbidden, "self_target_forbidden",
			"Không thao tác được lên chính tài khoản của mình. "+
				"Hãy nhờ một người quản trị khác của xã thực hiện.", "")

	case errors.As(err, &thieuQuyen):
		// 403 AND THE KEYS ARE NAMED. Permission keys are not personal data — they are the same
		// strings the Phân quyền screen prints — and naming them turns "you may not do this" into
		// something the administrator can act on: these are the rights they must be granted first,
		// or the role is one only somebody stronger may assign.
		httpx.WriteError(w, http.StatusForbidden, "permission_escalation",
			"Vai trò này mang quyền mà tài khoản của bạn không có, nên bạn không gán được: "+
				strings.Join(thieuQuyen.Thieu, ", ")+". Hãy nhờ người có đủ quyền thực hiện.", "")

	case errors.Is(err, app.ErrQuanTriCuoiCung):
		// 409 AND NOT 403: the caller holds `admin.user` and is allowed to manage users. What is
		// refused is this operation against the STATE of the commune — it would leave nobody able
		// to administer it, and ADR 0003 gives the vendor no way back in, so it is a procedural
		// dead end rather than an incident with a hot fix (#13). The same caller may do it the
		// moment a second administrator exists, which is exactly what 409 means.
		httpx.WriteError(w, http.StatusConflict, "last_admin",
			"Xã phải luôn còn ít nhất một người quản trị. Hãy cấp quyền quản trị cho một cán bộ "+
				"khác trước, rồi thực hiện lại thao tác này.", "")

	case errors.Is(err, app.ErrVaiTroKhongTonTai), errors.Is(err, idstore.ErrVaiTroKhongTonTai):
		httpx.WriteError(w, http.StatusBadRequest, "role_not_found",
			"Vai trò được chọn không còn trong xã. Hãy tải lại danh sách vai trò.", "")

	case errors.Is(err, idstore.ErrBoPhanKhongTonTai):
		httpx.WriteError(w, http.StatusBadRequest, "org_unit_not_found",
			"Bộ phận được chọn không còn trong xã. Hãy tải lại sơ đồ tổ chức.", "")

	case errors.Is(err, app.ErrChuaXacNhanDongY):
		// 400 WITH ITS OWN CODE, so the screen can point at the consent checkbox rather than at the
		// form in general. The sentence names the requirement and its legal basis (#12).
		httpx.WriteError(w, http.StatusBadRequest, "consent_required",
			"Chưa xác nhận đã hỏi ý và được chính người này đồng ý. Công khai số điện thoại lên "+
				"Zalo Mini App là công khai dữ liệu cá nhân (Nghị định 13/2023/NĐ-CP), cần có sự "+
				"đồng ý của người đó cho từng lần công khai.", "")

	case errors.Is(err, app.ErrCanBoCoTaiKhoan):
		// 409 AND NOT 403, for the same reason as last_admin: the caller holds the key. What is
		// refused is the operation against the STATE of this row. The sentence names BOTH ways
		// forward, because the person pressing the button is in one of two situations and the
		// answer differs: a retirement is a lock (the row stays), a genuine duplicate needs its
		// account revoked first — an act this system does not offer yet (`admin.user.revoke`,
		// ADR 0035), which is said plainly rather than sending them to look for it.
		httpx.WriteError(w, http.StatusConflict, "staff_has_account",
			"Cán bộ này đang có tài khoản đăng nhập nên không xoá được — xoá chỉ dành cho dòng danh bạ "+
				"nhập trùng không có tài khoản. Nếu người này nghỉ hưu hoặc chuyển công tác, hãy khoá "+
				"tài khoản thay vì xoá. Nếu đây đúng là dòng nhập trùng, cần thu hồi tài khoản trước; "+
				"chức năng thu hồi tài khoản chưa có trên hệ thống.", "")

	case errors.Is(err, idstore.ErrEmailDaDung):
		httpx.WriteError(w, http.StatusConflict, "email_taken",
			"Thư điện tử này đã được dùng cho một cán bộ khác trong xã.", "")

	case laLoiDauVaoCanBo(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")

	default:
		// The wrapped error carries the store failure. It does NOT carry a name, an e-mail or a
		// telephone number, and it never reaches the client (rule 3, forbidden #3). The commune is
		// logged because it is the only thing an operator can act on.
		h.d.Log.Error("ghi danh bạ cán bộ: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoCanBo reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is
// the client's fault" turns a database outage into a 400, and a client that believes its input is
// wrong retries with different input forever while nobody is told the server is broken.
func laLoiDauVaoCanBo(err error) bool {
	for _, mot := range []error{
		domain.ErrThieuHoTen, domain.ErrHoTenQuaDai, domain.ErrChucVuQuaDai,
		domain.ErrThieuEmail, domain.ErrEmailSaiDinhDang, domain.ErrEmailQuaDai,
		domain.ErrSoDienThoaiSai, domain.ErrSoDienThoaiQuaDai,
		domain.ErrIDThamChieuQuaDai,
		domain.ErrThuTuDanhBaAm, domain.ErrThuTuDanhBaQuaLon,
		domain.ErrThieuLyDoXoa, domain.ErrLyDoXoaQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
