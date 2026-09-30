package http

// GET /api/v1/tasks/register-export — the Sổ theo dõi of docs/ui-ux/02-nhiem-vu.md §4.3 as an .xlsx,
// under EXACTLY the filters and sort of GET /api/v1/tasks. `task.read` (user decision 28/09/2026).
//
// THE PATH CANNOT COLLIDE WITH `tasks/{ma}`: a register number is upper-case letters, digits, `-` and
// `_` only (domain.KiemMaNhiemVu), so the lower-case `register-export` is never a task's number — and
// the literal pattern is the more specific one for net/http's mux anyway.
//
// WHAT THE HANDLER DOES: HTTP translation, and the rendering callback. Reading, name resolution, the
// row cap and the audit entry are app.TaskRegisterExport's, in the order that file states.
//
// `limit` AND `cursor` ARE NOT READ: an export is the whole filtered register, never one page of it.

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// xlsxContentType is the media type of an Office Open XML workbook.
const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// registerExportFileName carries NO personal data and no commune name (rule 3, forbidden #4): the
// file is saved on a desktop and mailed around, and its name travels with it.
const registerExportFileName = "so-theo-doi-nhiem-vu.xlsx"

// ExportTaskRegister serves the file.
func (h *Handler) ExportTaskRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// THE LIST'S SORT, validated against the list's one allowlist before anything is read.
	sortParam, order := registerSortFromQuery(q)
	if _, err := page.New(petstore.SapXepNhiemVu, sortParam, order, "", ""); err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	// THE LIST'S FILTERS — one parser for the list, the counts and this file, identity-backed
	// `soon` / `scope=related` included (409 / 503 exactly as there).
	loc, ok := h.taskFilterFromRequest(w, r, q)
	if !ok {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("xuất sổ theo dõi: chủ thể không có mã cán bộ — SAI CẤU HÌNH ROUTE",
			"xa", string(tenant.MustFrom(ctx)))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	file, _, err := h.d.TaskRegisterExport.Export(ctx,
		app.RegisterExportRequest{Filter: loc, Sort: sortParam, Order: order}, actor, renderTaskRegister)
	switch {
	case errors.Is(err, app.ErrRegisterExportTooLarge):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "register_export_too_large",
			"Sổ theo dõi khớp quá 5.000 nhiệm vụ nên không xuất được một tệp. Hãy thu hẹp bộ lọc rồi xuất lại.", "")
		return
	case errors.Is(err, app.ErrRegisterNamesUnavailable):
		h.d.Log.Warn("CẢNH BÁO: không xuất được sổ theo dõi vì chưa tra được tên",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "register_names_unavailable",
			"Chưa tra được tên bộ phận hoặc cán bộ nên chưa xuất được sổ theo dõi. Vui lòng thử lại sau ít phút.", "")
		return
	case err != nil:
		h.d.Log.Error("xuất sổ theo dõi: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+registerExportFileName+`"`)
	// NOT CACHED ANYWHERE ON THE WAY: the file carries staff full names.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(file); err != nil {
		// The trail is already written, correctly: the export happened; the client left mid-transfer.
		h.d.Log.Warn("xuất sổ theo dõi: ngắt khi đang gửi tệp", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
}

// registerSortFromQuery reads the list's two sort parameters, by map index as locNhiemVuTuQuery reads
// its filters. The first value wins, as url.Values.Get would.
func registerSortFromQuery(q map[string][]string) (string, string) {
	first := func(v []string) string {
		if len(v) > 0 {
			return v[0]
		}
		return ""
	}
	return first(q["sort"]), first(q["order"])
}

// registerColumns are §4.3's columns IN §4.3'S ORDER (docs/ui-ux/02-nhiem-vu.md:117-126 — "Xuất
// Excel của bảng này phải giữ đúng thứ tự cột", :128). The `☐` selection column is a screen control
// and is not exported. The two approval marks are §4.3's "các ô tick phê duyệt".
//
// §4.3's "Cơ quan chủ trì tham mưu" and "Chuyên viên VP tham mưu / theo dõi" ARE NOT COLUMNS ANY MORE
// (ADR 0065 NV5, user decision 30/09/2026): the lead unit IS the unit and the monitor IS the assignee,
// both printed in "Đơn vị thực hiện". Two columns repeating that cell would be two copies of one fact.
var registerColumns = []string{
	"Mã",
	"Nội dung nhiệm vụ / Trích yếu văn bản",
	"Đơn vị thực hiện",
	"Văn bản cấp trên giao",
	"Văn bản chỉ đạo của Đảng uỷ",
	"Văn bản sản phẩm đầu ra",
	"Hạn xử lý",
	"Tóm tắt kết quả",
	"Ghi chú",
	"Lãnh đạo phê duyệt",
	"Cấp trên công nhận",
}

var registerColumnWidths = []float64{10, 60, 30, 44, 44, 44, 18, 44, 34, 12, 12}

// registerZone renders deadlines and document dates: a FIXED +07:00, for the reason domain's
// muiGioChoDan gives (Vietnam has no DST, and a zone database the image may lack must not make a
// printed deadline seven hours off).
var registerZone = time.FixedZone("ICT", 7*3600)

// renderTaskRegister builds the workbook. Every cell is written as a STRING value, never a formula:
// excelize stores it as an inline/shared string, so a title beginning with `=` is text, not a formula a
// spreadsheet would run (the "CSV injection" class does not apply to typed xlsx cells).
func renderTaskRegister(d app.TaskRegisterData) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "So theo doi"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	head, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	body, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	if err != nil {
		return nil, err
	}

	for i, title := range registerColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(sheet, cell, title); err != nil {
			return nil, err
		}
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(sheet, col, col, registerColumnWidths[i]); err != nil {
			return nil, err
		}
	}
	last, _ := excelize.CoordinatesToCellName(len(registerColumns), 1)
	if err := f.SetCellStyle(sheet, "A1", last, head); err != nil {
		return nil, err
	}

	for r, n := range d.Tasks {
		values := registerRow(n, d)
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	if len(d.Tasks) > 0 {
		end, _ := excelize.CoordinatesToCellName(len(registerColumns), len(d.Tasks)+1)
		if err := f.SetCellStyle(sheet, "A2", end, body); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// registerRow is one task's 11 cells, in registerColumns' order.
func registerRow(n domain.NhiemVu, d app.TaskRegisterData) []string {
	content := []string{n.TieuDe}
	if n.MoTa != "" {
		content = append(content, n.MoTa)
	}
	if n.Khoi != "" {
		content = append(content, blocLabel(n.Khoi, d))
	}
	unitLine := []string{unitName(n.BoPhanID, d)}
	if n.NguoiThucHienMa != "" {
		unitLine = append(unitLine, staffName(n.NguoiThucHienMa, d))
	}
	return []string{
		n.Ma,
		strings.Join(content, "\n"),
		strings.Join(unitLine, "\n"),
		documentsCell(n.VanBan, domain.VanBanCapTrenGiao),
		documentsCell(n.VanBan, domain.VanBanChiDaoDangUy),
		documentsCell(n.VanBan, domain.VanBanSanPhamRa),
		deadlineCell(n.HanXuLy),
		n.TomTatKetQua,
		n.GhiChu,
		mark(n.LanhDaoPheDuyetHoanThanh),
		mark(n.CapTrenCongNhanHoanThanh),
	}
}

// unitName renders a unit id. "" is "no unit" and prints `—`, as the screen does. An id identity did
// not answer prints "Không rõ bộ phận" — NEVER the ULID, which names nothing to a reader.
func unitName(id string, d app.TaskRegisterData) string {
	if id == "" {
		return "—"
	}
	u, ok := d.Units[id]
	if !ok {
		return "Không rõ bộ phận"
	}
	if u.Standing == identityv1.RecordStanding_RECORD_STANDING_REMOVED {
		return u.Name + " (đã gỡ)"
	}
	return u.Name
}

// staffName renders a staff business code as the person's name. An unanswered code prints the CODE:
// unlike a ULID it names somebody to a reader with the directory, and a blank would read as "nobody".
func staffName(code string, d app.TaskRegisterData) string {
	if code == "" {
		return "—"
	}
	s, ok := d.Staff[code]
	if !ok {
		return code
	}
	if s.TrangThai == identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_REMOVED_FROM_DIRECTORY {
		return s.HoTen + " (đã gỡ khỏi danh bạ)"
	}
	return s.HoTen
}

// blocLabel renders a bloc code; an unanswered code prints the code itself (a readable slug).
func blocLabel(code string, d app.TaskRegisterData) string {
	b, ok := d.Blocs[code]
	if !ok {
		return code
	}
	if b.Standing == identityv1.RecordStanding_RECORD_STANDING_REMOVED {
		return b.Label + " (đã gỡ)"
	}
	return b.Label
}

// documentsCell renders one of the three document groups: per line `<số ký hiệu> · <ngày>` (or
// `Không số`, §4.3), then the summary on the next line; lines separated by a blank line. An empty group
// is an empty cell. The lines arrive already in position order (store: ORDER BY nhom, thu_tu).
func documentsCell(lines []domain.NhiemVuVanBan, group domain.NhomVanBanNhiemVu) string {
	var parts []string
	for _, v := range lines {
		if v.Nhom != group {
			continue
		}
		head := v.SoKyHieu
		if head == "" {
			head = "Không số"
		}
		if !v.NgayVanBan.IsZero() {
			head += " · " + v.NgayVanBan.Format("02/01/2006")
		}
		parts = append(parts, head+"\n"+v.TrichYeu)
	}
	return strings.Join(parts, "\n\n")
}

// deadlineCell renders the CURRENT commitment (`han_xu_ly`) in Vietnam time, or `—` for none (§4.1).
func deadlineCell(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.In(registerZone).Format("02/01/2006 15:04")
}

func mark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}
