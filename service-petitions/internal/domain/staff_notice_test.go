package domain

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// The pure rules of ADR 0086's act notices: who, the key, the sentences, the mentions' shape.

func TestDropActorCleansAndRemovesTheActor(t *testing.T) {
	got := DropActor([]string{"CB-2", "CB-1", "CB-2", "", "CB 3", "CB-ACT"}, "CB-ACT")
	if strings.Join(got, ",") != "CB-1,CB-2" {
		t.Errorf("DropActor = %v", got)
	}
	if got := DropActor([]string{"CB-1"}, ""); len(got) != 1 {
		t.Errorf("không có người bấm (công dân) mà vẫn bỏ người: %v", got)
	}
	if got := DropActor([]string{"CB-ACT"}, "CB-ACT"); len(got) != 0 {
		t.Errorf("chỉ còn người bấm mà vẫn có người nhận: %v", got)
	}
}

func TestSplitRecipientsKeepsTheActKeyFirst(t *testing.T) {
	to := make([]string, 401)
	for i := range to {
		to[i] = "CB-" + strings.Repeat("x", i%5+1)
	}
	keys, parts := SplitRecipients("k", to)
	if len(keys) != 3 || keys[0] != "k" || keys[1] != "k:2" || keys[2] != "k:3" ||
		len(parts[0]) != 200 || len(parts[2]) != 1 {
		t.Errorf("chia = %v / %d,%d,%d", keys, len(parts[0]), len(parts[1]), len(parts[2]))
	}
	if keys, _ := SplitRecipients("k", []string{"CB-1"}); len(keys) != 1 || keys[0] != "k" {
		t.Errorf("một phần mà đổi khoá: %v", keys)
	}
}

func TestEveryActKindHasAVersionedName(t *testing.T) {
	for _, k := range []ActNoticeKind{ActNoticeTaskAssigned, ActNoticeTaskExtensionRequested,
		ActNoticeTaskApprovalRequested, ActNoticeTaskMention, ActNoticePetitionAssigned, ActNoticePetitionReopened} {
		if n := k.EventName(); !strings.HasSuffix(n, ".v1") {
			t.Errorf("%s: tên sự kiện %q không mang phiên bản", k, n)
		}
	}
	if ActNoticeKind("khac").EventName() != "" {
		t.Error("loại lạ có tên")
	}
}

func TestTaskRecipientRules(t *testing.T) {
	if got := TaskLeaderOrCreator(NhiemVu{LanhDaoGiaoViecMa: "CB-LD", NguoiTaoMa: "CB-TAO"}); got[0] != "CB-LD" {
		t.Errorf("lãnh đạo giao việc trước: %v", got)
	}
	if got := TaskLeaderOrCreator(NhiemVu{NguoiTaoMa: "CB-TAO"}); got[0] != "CB-TAO" {
		t.Errorf("không có lãnh đạo thì người tạo: %v", got)
	}
	if got := TaskLeaderOrCreator(NhiemVu{}); got != nil {
		t.Errorf("không ai: %v", got)
	}
	holders := map[string][]string{"bp-1": {"CB-H"}}
	if got := TaskAssignedRecipients(NhiemVu{NguoiThucHienMa: "CB-X", BoPhanID: "bp-1"}, holders); got[0] != "CB-X" {
		t.Errorf("người thực hiện được nêu tên trước: %v", got)
	}
	if got := TaskAssignedRecipients(NhiemVu{BoPhanID: "bp-1"}, holders); got[0] != "CB-H" {
		t.Errorf("chỉ bộ phận thì người giao việc của bộ phận: %v", got)
	}
	if got := TaskAssignedRecipients(NhiemVu{BoPhanID: "bp-2"}, holders); got != nil {
		t.Errorf("bộ phận không tra được: %v", got)
	}
}

func TestActNoticeSentences(t *testing.T) {
	long := strings.Repeat("Đ", 400)
	n := NhiemVu{Ma: "NV07", TieuDe: long, HanXuLy: time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC)}
	a := TaskAssignedNotice(n, "nk-1", []string{"CB-1"})
	if utf8.RuneCountInString(a.Title) > noticeTitleMax || a.Key() != "nhiem-vu.giao-moi:nk-1" || a.Link != "/nhiem-vu?q=NV07" {
		t.Errorf("giao việc = %q (%d) %q", a.Key(), utf8.RuneCountInString(a.Title), a.Link)
	}
	// 31/12 18:00 UTC is already 01/01 in Viet Nam.
	if a.Body != "Hạn 01/01/2027" {
		t.Errorf("hạn theo giờ Việt Nam = %q", a.Body)
	}
	if b := TaskAssignedNotice(NhiemVu{Ma: "NV08", TieuDe: "x"}, "nk", nil); b.Body != "" {
		t.Errorf("không có hạn mà vẫn ghi: %q", b.Body)
	}
	m := TaskMentionNotice(NhiemVu{Ma: "NV07", TieuDe: "x"}, "nk-2", strings.Repeat("a", 500), []string{"CB-1"})
	if utf8.RuneCountInString(m.Body) > MentionBodyMax {
		t.Errorf("nội dung nhắc tên dài %d", utf8.RuneCountInString(m.Body))
	}
	p := PetitionAssignedNotice(PhieuPhanAnh{MaTraCuu: "PA-ABC", CanBoXuLyID: "CB-9", LinhVuc: LinhVucHanChe}, "nk-3")
	if p.Title != "Bạn được giao xử lý phản ánh PA-ABC" || strings.Contains(p.Title, LinhVucHanChe) || p.Recipients[0] != "CB-9" {
		t.Errorf("phân công phiếu = %+v", p)
	}
	r := PetitionReopenedNotice(PhieuPhanAnh{MaTraCuu: "PA-ABC", CanBoXuLyID: "CB-9"}, "nk-4")
	if r.Title != "Phản ánh PA-ABC bị mở lại do đánh giá thấp" || r.Body != "" || r.Link != "/phan-anh?q=PA-ABC" {
		t.Errorf("mở lại = %+v", r)
	}
}

func TestNormaliseMentions(t *testing.T) {
	got, err := NormaliseMentions([]string{" CB-2 ", "CB-1", "CB-2"})
	if err != nil || strings.Join(got, ",") != "CB-2,CB-1" {
		t.Errorf("= %v %v", got, err)
	}
	if got, err := NormaliseMentions(nil); err != nil || got == nil || len(got) != 0 {
		t.Errorf("nil = %v %v", got, err)
	}
	for _, bad := range [][]string{{""}, {"CB 1"}, {"CB\t1"}, {strings.Repeat("x", 65)}} {
		if _, err := NormaliseMentions(bad); err != ErrMentionInvalid {
			t.Errorf("%q: %v", bad, err)
		}
	}
	many := make([]string, MentionsMax+1)
	for i := range many {
		many[i] = "CB-" + string(rune('A'+i))
	}
	if _, err := NormaliseMentions(many); err != ErrMentionsTooMany {
		t.Errorf("quá nhiều: %v", err)
	}
}

func TestReportReadyNoticeOmitsMissingFigures(t *testing.T) {
	start := time.Date(2026, 9, 27, 17, 0, 0, 0, time.UTC)
	n := ReportReadyNotice(AutomationTask, ReportWeek, start, nil, []string{"CB-LD"})
	if n.Body != "" || n.Key != "scheduled_reports:nhiem-vu:week:2026-09-28" || n.Kind != NoticeReportReady {
		t.Errorf("= %+v", n)
	}
	n = ReportReadyNotice(AutomationCitizenReport, ReportMonth, start,
		[]ReportFigure{{Label: "Phản ánh trễ hạn trong kỳ", Value: 0}}, []string{"CB-LD"})
	// A REAL zero (counted) is printed; only a figure with no data source is omitted.
	if n.Body != "Phản ánh trễ hạn trong kỳ: 0" || n.Title != "Báo cáo điều hành tháng đã sẵn sàng" {
		t.Errorf("= %+v", n)
	}
}
