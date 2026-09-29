package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// "⬆ Nhập từ Excel" on Cấu hình → Người dùng (14-cau-hinh.md §3; user decision 2026-09-29, ADR 0059
// §1) — three routes, all `admin.user`, the key POST /api/v1/staff declares. A file with ANY non-empty
// Vai trò cell additionally needs `admin.role`; the use case checks it and this layer answers 403.
//
//	GET  /api/v1/staff/import-template  the .xlsx to fill in, with two dropdowns (Bộ phận, Vai trò)
//	POST /api/v1/staff/import-previews  check a filled file — WRITES NOTHING, mints nothing
//	POST /api/v1/staff/imports          write it, all or nothing; 201 carries the temporary passwords ONCE
//
// PERSONAL DATA ON THE WIRE (rule 3). The preview echoes back what the administrator's own file holds,
// so they can check it — EXCEPT the personal mobile (#16, Decree 13), which is MASKED (privacy.MaskPhone).
// Error messages name the row and the column only. The 201 carries no telephone number at all.
//
// THE PASSWORDS (#9, ADR 0059 §1). The 201 body is the ONLY place they ever appear. Both responses carry
// `Cache-Control: no-store`. A retry with the same Idempotency-Key is answered by core/idem from its
// cache, which never holds a body: the replay is `{"code":"<batch_id>","replayed":true}` — the batch
// code this handler records, and NO password. An administrator who lost the first response resets each
// person through PUT /api/v1/staff/{id}/password, one new value and one new entry each.

// staffSheet is the data sheet's tab name — a UI string (rule 12, invariant 2).
const staffSheet = "Cán bộ"

type staffImportErrorOut struct {
	Row     int    `json:"row"`    // dòng trong bảng tính (tiêu đề là dòng 1); 0 = lỗi của cả tệp
	Column  string `json:"column"` // tên cột như trên tiêu đề; "" = lỗi của cả dòng hoặc cả tệp
	Message string `json:"message"`
}

// staffImportPlannedOut is one person the import would create. `mobile` is MASKED.
type staffImportPlannedOut struct {
	Row           int    `json:"row"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"` // "" = no address → no account
	Position      string `json:"position"`
	OrgUnitCode   string `json:"org_unit_code"`
	OrgUnitName   string `json:"org_unit_name"`
	RoleCode      string `json:"role_code"`
	RoleName      string `json:"role_name"`
	OfficePhone   string `json:"office_phone"`
	Mobile        string `json:"mobile"` // masked (rule 3, #16)
	IssuesAccount bool   `json:"issues_account"`
}

type staffImportPreviewOut struct {
	Valid  bool                    `json:"valid"`
	People []staffImportPlannedOut `json:"people"`
	Errors []staffImportErrorOut   `json:"errors"`
}

// staffImportCreatedPersonOut is one person created. `login` is the address the person signs in with
// ("" when none — and then no account). NO TELEPHONE NUMBER: the file for the administrator to hand
// out credentials needs none.
type staffImportCreatedPersonOut struct {
	Row           int    `json:"row"`
	ID            string `json:"id"`
	Code          string `json:"code"`
	FullName      string `json:"full_name"`
	Login         string `json:"login"`
	OrgUnitCode   string `json:"org_unit_code"`
	RoleCode      string `json:"role_code"`
	AccountIssued bool   `json:"account_issued"`

	// TemporaryPassword is plaintext, returned exactly once, absent when no account was issued. THE
	// SAME DECLARED EXEMPTION as capTaiKhoanRa.TemporaryPassword (tai_khoan_can_bo.go), multiplied by N
	// by the user's decision of 2026-09-29 (ADR 0059 §1).
	TemporaryPassword string `json:"temporary_password,omitempty" apidoc:"bi-mat-co-chu-y:Mật khẩu tạm dùng MỘT LẦN cho từng người được cấp tài khoản, người dùng chốt 29/09/2026 (ADR 0059 §1, câu mở #9): trả duy nhất trong phản hồi này, không lưu bản trần, không có trong vết, lần gửi lại cùng Idempotency-Key chỉ trả mã lô. Bắt buộc đổi ở lần đăng nhập đầu."`
}

type staffImportCreatedOut struct {
	// BatchID is `lo_nhap` on every audit entry of this file — also what an idempotent replay returns
	// as `code`.
	BatchID string                        `json:"batch_id"`
	Created []staffImportCreatedPersonOut `json:"created"`
}

type staffImportRejectedOut struct {
	Code    string                `json:"code"`
	Message string                `json:"message"`
	TraceID string                `json:"trace_id"`
	Errors  []staffImportErrorOut `json:"errors"`
}

func staffErrorsOut(in []domain.StaffImportError) []staffImportErrorOut {
	out := make([]staffImportErrorOut, 0, len(in))
	for _, e := range in {
		out = append(out, staffImportErrorOut{Row: e.Row, Column: e.Column, Message: e.Message})
	}
	return out
}

func staffPlannedOut(in []app.ImportedStaff) []staffImportPlannedOut {
	out := make([]staffImportPlannedOut, 0, len(in))
	for _, p := range in {
		mobile := ""
		if p.Staff.DiDongCaNhan != "" {
			mobile = privacy.MaskPhone(p.Staff.DiDongCaNhan)
		}
		out = append(out, staffImportPlannedOut{
			Row: p.Row, FullName: p.Staff.HoTen, Email: p.Staff.Email, Position: p.Staff.ChucVu,
			OrgUnitCode: p.OrgUnitCode, OrgUnitName: p.OrgUnitName, RoleCode: p.RoleCode, RoleName: p.RoleName,
			OfficePhone: p.Staff.DienThoaiCoQuan, Mobile: mobile, IssuesAccount: p.AccountIssued,
		})
	}
	return out
}

func staffCreatedOut(in []app.ImportedStaff) []staffImportCreatedPersonOut {
	out := make([]staffImportCreatedPersonOut, 0, len(in))
	for _, p := range in {
		out = append(out, staffImportCreatedPersonOut{
			Row: p.Row, ID: p.Staff.ID, Code: p.Staff.Ma, FullName: p.Staff.HoTen, Login: p.Staff.Email,
			OrgUnitCode: p.OrgUnitCode, RoleCode: p.RoleCode, AccountIssued: p.AccountIssued,
			TemporaryPassword: p.TemporaryPassword,
		})
	}
	return out
}

// staffTemplateSpec builds the template. NO EXAMPLE ROW ON THE DATA SHEET: an example left in place is
// imported as a real person with a permanent code AND an account. The guide's example number is the
// agreed fake (rule 3, invariant 5) — not the realistic one in ../vigov-require (ADR 0059 §1).
func staffTemplateSpec(units []domain.StaffImportOrgUnitChoice, roles []domain.StaffImportRoleChoice) xlsx.TemplateSpec {
	unitValues := make([]string, 0, len(units))
	for _, u := range units {
		unitValues = append(unitValues, domain.StaffChoiceLabel(u.Code, u.Name))
	}
	roleValues := make([]string, 0, len(roles))
	for _, r := range roles {
		roleValues = append(roleValues, domain.StaffChoiceLabel(r.Code, r.Name))
	}
	return xlsx.TemplateSpec{
		SheetName: staffSheet,
		Header:    domain.StaffImportColumns(),
		Guide: []string{
			"Mỗi dòng dưới dòng tiêu đề là MỘT cán bộ MỚI. Không sửa, không đổi thứ tự dòng tiêu đề. Mã cán bộ do hệ thống tự sinh — tệp không có cột Mã.",
			"Họ và tên: bắt buộc. Hai người trùng họ tên vẫn nhập được — mã cán bộ phân biệt họ.",
			"Thư điện tử công vụ: là TÊN ĐĂNG NHẬP. Có thư điện tử thì người đó được cấp tài khoản với một mật khẩu tạm, bắt buộc đổi ở lần đăng nhập đầu. Để trống thì chỉ thêm vào danh bạ, không cấp tài khoản. Không trùng thư điện tử đã có trong danh bạ của xã (kể cả dòng đã xoá) và không trùng nhau trong tệp.",
			"Bộ phận, Vai trò: chọn trong danh sách (hệ thống đọc MÃ đứng trước dấu \"·\"); để trống nếu chưa có. Tệp có cột Vai trò không trống thì người nhập cần thêm quyền Phân quyền (admin.role), và không trao được vai trò mang quyền mình không giữ.",
			"Điện thoại cơ quan: số máy bàn công vụ. Di động cá nhân: dữ liệu cá nhân (Nghị định 13/2023) — chỉ ghi khi người đó đồng ý. Ví dụ định dạng: 0900000000.",
			"Tệp được nhập TOÀN BỘ HOẶC KHÔNG GÌ CẢ: chỉ cần một dòng lỗi thì không ai được tạo. Hãy bấm Kiểm tra trước khi Nhập.",
			"Sau khi nhập, màn hình trả về mật khẩu tạm của từng người ĐÚNG MỘT LẦN. Hãy tải xuống, phát cho từng người và xoá tệp đó đi. Mất tệp thì đặt lại mật khẩu từng người.",
		},
		Choices: map[int]xlsx.Choices{
			3: {Values: unitValues, Title: "Bộ phận", Message: "Không phải bộ phận của xã."},
			4: {Values: roleValues, Title: "Vai trò", Message: "Không phải vai trò của xã."},
		},
		DropdownRows: domain.MaxStaffImportRows,
	}
}

// StaffImportTemplate serves GET /api/v1/staff/import-template.
func (h *Handler) StaffImportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// The store reads through store.DB.For(ctx): tenant_id comes from the context, never a parameter.
	units, roles, err := h.d.StaffImports.TemplateChoices(ctx)
	if err != nil {
		h.d.Log.Error("mẫu nhập cán bộ: đọc danh sách chọn lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	b, err := xlsx.Template(staffTemplateSpec(units, roles))
	if err != nil {
		h.d.Log.Error("mẫu nhập cán bộ: dựng tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	w.Header().Set("Content-Type", mimeXLSX)
	w.Header().Set("Content-Disposition", `attachment; filename="mau-nhap-can-bo.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(b); err != nil {
		h.d.Log.Warn("mẫu nhập cán bộ: gửi tệp lỗi", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
}

func (h *Handler) readStaffRows(w http.ResponseWriter, r *http.Request) ([]domain.StaffImportRow, []domain.StaffImportError, bool) {
	data, ok := h.readImportUpload(w, r)
	if !ok {
		return nil, nil, false
	}
	cells, ok := h.readImportSheet(w, r, data, "nhập cán bộ")
	if !ok {
		return nil, nil, false
	}
	rows, errs := domain.StaffRowsFromSheet(cells)
	return rows, errs, true
}

func writeRoleImportForbidden(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusForbidden, "role_permission_required",
		"Tệp có cột Vai trò không trống — gán vai trò cần thêm quyền Phân quyền (admin.role). Hãy để trống cột Vai trò, hoặc nhờ người có quyền này nhập tệp.", "")
}

// PreviewStaffImport serves POST /api/v1/staff/import-previews. 200 whether or not the file is valid.
func (h *Handler) PreviewStaffImport(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	rows, sheetErrs, ok := h.readStaffRows(w, r)
	if !ok {
		return
	}
	// The body echoes names and addresses from the file: never cached in between.
	w.Header().Set("Cache-Control", "no-store")
	if len(sheetErrs) > 0 {
		vietJSON(w, http.StatusOK, staffImportPreviewOut{People: []staffImportPlannedOut{}, Errors: staffErrorsOut(sheetErrs)})
		return
	}
	// The use case opens store.DB.For(ctx): tenant_id comes from the Host-derived context.
	res, err := h.d.StaffImports.Preview(r.Context(), rows, actor)
	if err != nil {
		if errors.Is(err, app.ErrRoleAssignmentNotPermitted) {
			writeRoleImportForbidden(w)
			return
		}
		h.d.Log.Error("nhập cán bộ: xem trước lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, staffImportPreviewOut{
		Valid:  len(res.Errors) == 0,
		People: staffPlannedOut(res.People),
		Errors: staffErrorsOut(res.Errors),
	})
}

// ImportStaff serves POST /api/v1/staff/imports.
func (h *Handler) ImportStaff(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	rows, sheetErrs, ok := h.readStaffRows(w, r)
	if !ok {
		return
	}
	if len(sheetErrs) > 0 {
		writeStaffImportRejected(w, sheetErrs)
		return
	}
	res, err := h.d.StaffImports.Import(r.Context(), rows, actor)
	if err != nil {
		var rej *app.StaffImportRejected
		switch {
		case errors.As(err, &rej):
			writeStaffImportRejected(w, rej.Errors)
		case errors.Is(err, app.ErrRoleAssignmentNotPermitted):
			writeRoleImportForbidden(w)
		case errors.Is(err, idstore.ErrEmailDaDung), errors.Is(err, idstore.ErrBoPhanKhongTonTai),
			errors.Is(err, idstore.ErrVaiTroKhongTonTai), errors.Is(err, app.ErrStaffImportChanged):
			// The commune changed between the plan and the write. The whole file was rolled back.
			httpx.WriteError(w, http.StatusConflict, "staff_changed",
				"Danh bạ, bộ phận hoặc vai trò của xã vừa thay đổi. Chưa ai được tạo. Hãy kiểm tra lại tệp rồi nhập lại.", "")
		default:
			// The error never carries a password: nothing in app/ or store/ puts one in an error.
			h.d.Log.Error("nhập cán bộ: lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Chưa ai được tạo. Vui lòng thử lại.", "")
		}
		return
	}
	// A retry with the same key replays THIS code and nothing else — never a password (core/idem).
	idem.RecordCode(r.Context(), res.Batch)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	vietJSON(w, http.StatusCreated, staffImportCreatedOut{BatchID: res.Batch, Created: staffCreatedOut(res.People)})
}

// writeStaffImportRejected is 400 with every error — like writeResidentialUnitImportRejected.
func writeStaffImportRejected(w http.ResponseWriter, errs []domain.StaffImportError) {
	vietJSON(w, http.StatusBadRequest, staffImportRejectedOut{
		Code:    "import_invalid",
		Message: "Tệp có lỗi nên chưa cán bộ nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
		Errors:  staffErrorsOut(errs),
	})
}
