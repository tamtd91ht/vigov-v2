package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Tests for the spreadsheet import's row rules (P11).
//
//	PROVED HERE   the heading row must be the template's exactly · a deadline needs a date AND a time,
//	              read in +07:00; a date alone and an unreadable value are refused with their own
//	              sentences · every refusal names its row and column and never quotes the cell · a
//	              document cell becomes ONE line, stored whole, never parsed · the approval marks accept
//	              `x`/`có` and refuse anything else · unit or assignee is required.

func importRow(over map[int]string) []string {
	cells := make([]string, len(TaskImportHeadings))
	cells[importColTitle] = "Rà soát quỹ đất công ích"
	cells[importColUnit] = "dia-chinh"
	for i, v := range over {
		cells[i] = v
	}
	return cells
}

func TestCheckTaskImportHeadings(t *testing.T) {
	good := append([]string(nil), TaskImportHeadings...)
	if !CheckTaskImportHeadings(good) {
		t.Fatal("hàng tiêu đề đúng mẫu bị từ chối")
	}
	if !CheckTaskImportHeadings(append(append([]string(nil), good...), "", "  ")) {
		t.Error("cột trống thừa ở cuối không được làm hỏng mẫu")
	}
	swapped := append([]string(nil), good...)
	swapped[2], swapped[3] = swapped[3], swapped[2]
	for name, cells := range map[string][]string{
		"đổi chỗ hai cột": swapped,
		"thiếu cột":       good[:len(good)-1],
		"thêm cột có chữ": append(append([]string(nil), good...), "Cột lạ"),
	} {
		if CheckTaskImportHeadings(cells) {
			t.Errorf("%s: nhận một mẫu khác — giá trị sẽ vào nhầm trường", name)
		}
	}
}

// TestTaskImportRetiredColumns — ADR 0065 NV5: the template no longer carries "cơ quan chủ trì" /
// "chuyên viên theo dõi", and a heading row that still names either, at any position, is recognised as
// the old template so the refusal can say why.
func TestTaskImportRetiredColumns(t *testing.T) {
	for _, h := range TaskImportRetiredHeadings {
		for _, current := range TaskImportHeadings {
			if current == h {
				t.Fatalf("mẫu hiện hành còn cột đã bỏ %q", h)
			}
		}
	}
	if TaskImportCarriesRetiredColumns(TaskImportHeadings) {
		t.Fatal("mẫu hiện hành bị coi là mẫu cũ")
	}
	// The pre-NV5 layout: the two columns sat after "Khối nhiệm vụ (mã)".
	old := append([]string(nil), TaskImportHeadings[:8]...)
	old = append(old, " "+TaskImportRetiredHeadings[0]+" ", TaskImportRetiredHeadings[1])
	old = append(old, TaskImportHeadings[8:]...)
	if !TaskImportCarriesRetiredColumns(old) {
		t.Error("mẫu cũ (còn hai cột đã bỏ) không được nhận ra")
	}
	if CheckTaskImportHeadings(old) {
		t.Error("mẫu cũ lọt qua kiểm tra hàng tiêu đề")
	}
	onlyMonitor := append(append([]string(nil), TaskImportHeadings...), TaskImportRetiredHeadings[1])
	if !TaskImportCarriesRetiredColumns(onlyMonitor) {
		t.Error("còn riêng cột chuyên viên theo dõi ở cuối mà không được nhận ra")
	}
}

func TestParseImportDeadline(t *testing.T) {
	want := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC) // 17:00 in Vietnam
	for _, s := range []string{"30/09/2026 17:00", "30/9/2026 17:00", "2026-09-30 17:00", " 30/09/2026   17:00:00 "} {
		got, err := ParseImportDeadline(s)
		if err != nil || !got.Equal(want) {
			t.Errorf("%q = %v, %v — muốn %v (giờ Việt Nam)", s, got, err, want)
		}
	}
	for _, s := range []string{"30/09/2026", "2026-09-30"} {
		if _, err := ParseImportDeadline(s); !errors.Is(err, ErrImportDeadlineNeedsTime) {
			t.Errorf("%q: lỗi = %v, muốn ErrImportDeadlineNeedsTime — không được tự chọn giờ", s, err)
		}
	}
	for _, s := range []string{"cuối tháng", "31/02/2026 17:00", "30/09/2026 25:00"} {
		if _, err := ParseImportDeadline(s); !errors.Is(err, ErrImportDeadlineUnreadable) {
			t.Errorf("%q: lỗi = %v, muốn ErrImportDeadlineUnreadable", s, err)
		}
	}
	if got, err := ParseImportDeadline(""); err != nil || !got.IsZero() {
		t.Errorf("ô trống phải là không có hạn: %v, %v", got, err)
	}
}

func TestParseTaskImportRowDocumentsStoredWhole(t *testing.T) {
	const upper = "Công văn số 0000-CV/XX ngày 01/09/2026 của cơ quan cấp trên"
	r, errs := ParseTaskImportRow(2, importRow(map[int]string{
		importColDocUpper:       "  " + upper + "  ",
		importColDocOutput:      "Báo cáo kết quả",
		importColDue:            "30/09/2026 17:00",
		importColLeaderApproved: "x", importColSuperiorAcknowledged: "Có",
	}))
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	if len(r.Documents) != 2 || r.Documents[0].TrichYeu != upper || r.Documents[0].SoKyHieu != "" ||
		!r.Documents[0].NgayVanBan.IsZero() || r.Documents[0].Nhom != VanBanCapTrenGiao ||
		r.Documents[1].Nhom != VanBanSanPhamRa {
		t.Errorf("văn bản = %+v — mỗi ô một dòng, nguyên văn, không tách số hay ngày", r.Documents)
	}
	if !r.LeaderApproved || !r.SuperiorAcknowledged {
		t.Errorf("dấu duyệt không đọc được: %v %v", r.LeaderApproved, r.SuperiorAcknowledged)
	}
}

func TestParseTaskImportRowRefusalsNameRowAndColumnNeverTheValue(t *testing.T) {
	const secret = "Nguyễn Văn Bí Mật"
	_, errs := ParseTaskImportRow(7, importRow(map[int]string{
		importColTitle:          "",
		importColUnit:           "",
		importColDue:            "15/10/2026",
		importColLeaderApproved: secret,
	}))
	cols := map[string]bool{}
	for _, e := range errs {
		if e.Row != 7 {
			t.Errorf("lỗi mang dòng %d, muốn 7", e.Row)
		}
		if strings.Contains(e.Message, secret) || strings.Contains(e.Message, "15/10/2026") {
			t.Errorf("thông báo nhắc lại giá trị của ô: %q", e.Message)
		}
		cols[e.Column] = true
	}
	for _, c := range []int{importColTitle, importColUnit, importColDue, importColLeaderApproved} {
		if !cols[TaskImportHeadings[c]] {
			t.Errorf("thiếu lỗi ở cột %q: %+v", TaskImportHeadings[c], errs)
		}
	}
}

func TestTaskImportRowBlank(t *testing.T) {
	if !TaskImportRowBlank([]string{"", "  ", "\t"}) || TaskImportRowBlank([]string{"", "x"}) {
		t.Error("nhận diện dòng trống sai")
	}
}

// --- Bug sheet row 19: "báo lỗi mặc dù các dữ liệu là hợp lệ" -----------------------------------------
//
// The customer's file is not available, so these are the shapes a VALID file takes once Excel and a
// Vietnamese keyboard have touched the template. Each was refused before the fix.

// TestTaskImportHeadingsSurviveExcelEditing — a heading row that READS as the template's is the
// template's: letter case, a line break typed with Alt+Enter, doubled or non-breaking spaces, and the
// two tone placements of the same syllable ("uỷ" / "ủy") change nothing a person can see.
func TestTaskImportHeadingsSurviveExcelEditing(t *testing.T) {
	for name, edit := range map[string]func([]string){
		"chữ hoa":            func(h []string) { h[0] = "TÊN NHIỆM VỤ" },
		"xuống dòng trong ô": func(h []string) { h[5] = "Hạn hoàn thành\n(ngày giờ)" },
		"hai dấu cách":       func(h []string) { h[2] = "Đơn vị  thực hiện (mã)" },
		"dấu cách không ngắt": func(h []string) {
			h[13] = "Ghi chú "
		},
		"bỏ dấu kiểu mới": func(h []string) { h[9] = "Văn bản chỉ đạo của Đảng ủy" },
	} {
		head := append([]string(nil), TaskImportHeadings...)
		edit(head)
		if !CheckTaskImportHeadings(head) {
			t.Errorf("%s: hàng tiêu đề đọc giống hệt mẫu nhưng bị từ chối", name)
		}
	}
}

// TestLocateTaskImportHeadingsBelowTitleRows — a clerk adds the commune's name and a title above the
// heading row (often merged across the sheet). The heading is found below them and the rows above are
// never read as tasks; a sheet whose heading is not in the first rows is still refused.
func TestLocateTaskImportHeadingsBelowTitleRows(t *testing.T) {
	sheet := [][]string{
		{"UBND XÃ MẪU", "", "", "CỘNG HOÀ XÃ HỘI CHỦ NGHĨA VIỆT NAM"},
		{"DANH SÁCH NHIỆM VỤ GIAO"},
		{},
		append([]string(nil), TaskImportHeadings...),
		importRow(nil),
	}
	if at, ok := LocateTaskImportHeadings(sheet); !ok || at != 3 {
		t.Fatalf("hàng tiêu đề ở chỉ số %d (%v), muốn 3", at, ok)
	}
	if at, ok := LocateTaskImportHeadings([][]string{append([]string(nil), TaskImportHeadings...)}); !ok || at != 0 {
		t.Errorf("hàng tiêu đề ở dòng 1: %d, %v", at, ok)
	}
	deep := make([][]string, TaskImportHeadingSearchRows)
	deep = append(deep, append([]string(nil), TaskImportHeadings...))
	if _, ok := LocateTaskImportHeadings(deep); ok {
		t.Error("hàng tiêu đề nằm quá sâu vẫn được nhận — các dòng phía trên có thể là nhiệm vụ")
	}
}

// TestDiagnoseTaskImportHeadingsNamesTheCell — a refused heading row says WHICH cell differs and what
// the template has there, so the clerk can fix one cell instead of retyping the file.
func TestDiagnoseTaskImportHeadingsNamesTheCell(t *testing.T) {
	head := append([]string(nil), TaskImportHeadings...)
	head[5] = "Hạn hoàn thành"
	p := DiagnoseTaskImportHeadings([][]string{{"DANH SÁCH"}, head, importRow(nil)})
	if p.Row != 2 || p.Column != 5 {
		t.Fatalf("chẩn đoán = %+v, muốn dòng 2 cột F", p)
	}
	s := p.Sentence()
	for _, want := range []string{"F2", TaskImportHeadings[5]} {
		if !strings.Contains(s, want) {
			t.Errorf("câu %q thiếu %q", s, want)
		}
	}

	extra := append(append([]string(nil), TaskImportHeadings...), "Cột thêm")
	if p := DiagnoseTaskImportHeadings([][]string{extra}); p.Row != 1 || p.Column != len(TaskImportHeadings) ||
		!strings.Contains(p.Sentence(), "O1") {
		t.Errorf("cột ngoài mẫu: %+v %q", p, p.Sentence())
	}
	if p := DiagnoseTaskImportHeadings([][]string{{"STT", "Nội dung"}, {"1", "Việc"}}); p.Row != 0 ||
		!strings.Contains(p.Sentence(), "không tìm thấy hàng tiêu đề") {
		t.Errorf("không có hàng tiêu đề: %+v %q", p, p.Sentence())
	}
}

// TestParseImportDeadlineVietnameseSpellings — the same date AND time, written the ways Vietnamese
// offices write it. A date with no time is still refused (user decision 28/09/2026), and so is a time
// with no date.
func TestParseImportDeadlineVietnameseSpellings(t *testing.T) {
	want := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC) // 17:00 in Vietnam
	for _, s := range []string{
		"17:00 30/09/2026", "17h00 30/09/2026", "30/09/2026 17h00", "30/09/2026 17h", "30/09/2026 17H00",
		"30-09-2026 17:00", "30.09.2026 17:00", "2026-09-30T17:00:00", "17:00 ngày 30/09/2026",
		"30/09/2026 lúc 17:00",
	} {
		got, err := ParseImportDeadline(s)
		if err != nil || !got.Equal(want) {
			t.Errorf("%q = %v, %v — muốn %v", s, got, err, want)
		}
	}
	for _, s := range []string{"30-09-2026", "30.09.2026", "ngày 30/09/2026"} {
		if _, err := ParseImportDeadline(s); !errors.Is(err, ErrImportDeadlineNeedsTime) {
			t.Errorf("%q: lỗi = %v, muốn ErrImportDeadlineNeedsTime", s, err)
		}
	}
	for _, s := range []string{"17:00", "30/09/26 17:00", "17:00 17:00", "30/09/2026 30/09/2026 17:00", "30/09/2026 17:5"} {
		if _, err := ParseImportDeadline(s); !errors.Is(err, ErrImportDeadlineUnreadable) {
			t.Errorf("%q: lỗi = %v, muốn ErrImportDeadlineUnreadable", s, err)
		}
	}
}

// TestParseTaskImportRowFoldsCodeCase — unit, priority, type and bloc codes are lower-case only by
// their own validators (identity ChuanHoaMaBoPhan / ChuanHoaMaDanhMuc, petitions ChuanHoaMa), so
// "Dia-Chinh" can only mean "dia-chinh". Staff codes carry no such guarantee and are kept exactly.
func TestParseTaskImportRowFoldsCodeCase(t *testing.T) {
	r, errs := ParseTaskImportRow(2, importRow(map[int]string{
		importColUnit: "Dia-Chinh", importColPriority: "CAO", importColType: "Theo-Van-Ban",
		importColBloc: "Khoi-Uy-Ban", importColAssignee: "CB-2026-7K3M9Q",
	}))
	if len(errs) != 0 {
		t.Fatalf("lỗi: %+v", errs)
	}
	if r.UnitCode != "dia-chinh" || r.PriorityCode != "cao" || r.TypeCode != "theo-van-ban" ||
		r.BlocCode != "khoi-uy-ban" || r.AssigneeCode != "CB-2026-7K3M9Q" {
		t.Errorf("mã sau chuẩn hoá = %+v", r)
	}
}

// TestParseTaskImportRowExplicitNoMark — Excel turns a typed FALSE into a boolean cell (read as "0"),
// and clerks write "không" / "chưa". All mean "not approved", like an empty cell.
func TestParseTaskImportRowExplicitNoMark(t *testing.T) {
	for _, v := range []string{"0", "FALSE", "false", "Không", "chưa"} {
		r, errs := ParseTaskImportRow(2, importRow(map[int]string{importColLeaderApproved: v}))
		if len(errs) != 0 || r.LeaderApproved {
			t.Errorf("%q: %+v, đã duyệt = %v", v, errs, r.LeaderApproved)
		}
	}
	if r, errs := ParseTaskImportRow(2, importRow(map[int]string{importColLeaderApproved: "TRUE"})); len(errs) != 0 || !r.LeaderApproved {
		t.Errorf("TRUE: %+v %v", errs, r.LeaderApproved)
	}
}
