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
