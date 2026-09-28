package domain

import (
	"errors"
	"strings"
	"testing"
)

// ADR 0050 point 2, as pure rules. The database's two CHECKs (0004 stars 1..5, 0017 comment ≤ 1000
// and non-blank) are the floor; these are the 400s in front of them.

func TestCheckRatingStarsOneToFive(t *testing.T) {
	for stars := RatingMinStars; stars <= RatingMaxStars; stars++ {
		if got, _, err := CheckRating(stars, ""); err != nil || got != stars {
			t.Errorf("%d sao bị từ chối: %v", stars, err)
		}
	}
	for _, stars := range []int{0, -1, 6, 100} {
		if _, _, err := CheckRating(stars, ""); !errors.Is(err, ErrRatingStarsOutOfRange) {
			t.Errorf("%d sao: lỗi = %v, muốn ErrRatingStarsOutOfRange", stars, err)
		}
	}
}

// TestCheckRatingCommentCountsRunes — 1000 Vietnamese characters are 1000, not 3000 bytes. A byte count
// would refuse a comment PostgreSQL's char_length accepts.
func TestCheckRatingCommentCountsRunes(t *testing.T) {
	full := strings.Repeat("ồ", RatingCommentMaxLen)
	if _, got, err := CheckRating(4, full); err != nil || got != full {
		t.Errorf("nhận xét đúng %d ký tự có dấu bị từ chối: %v", RatingCommentMaxLen, err)
	}
	_, _, err := CheckRating(4, full+"ồ")
	if !errors.Is(err, ErrRatingCommentTooLong) {
		t.Fatalf("nhận xét %d ký tự: lỗi = %v, muốn ErrRatingCommentTooLong", RatingCommentMaxLen+1, err)
	}
	if strings.Contains(err.Error(), "ồồ") {
		t.Error("lỗi nhắc lại nội dung người dân gõ (luật 3 cấm #3)")
	}
}

// TestCheckRatingBlankCommentIsNone — a textarea of spaces is "no comment", never a blank string the
// migration 0017 CHECK would refuse with a 500.
func TestCheckRatingBlankCommentIsNone(t *testing.T) {
	if _, got, err := CheckRating(3, "  \n\t "); err != nil || got != "" {
		t.Errorf("nhận xét trắng = %q, %v; muốn \"\" và không lỗi", got, err)
	}
	if _, got, _ := CheckRating(3, "  Tốt  "); got != "Tốt" {
		t.Errorf("nhận xét không được cắt khoảng trắng: %q", got)
	}
}

// TestRatingOpenOnlyAtTheTwoStatuses — ADR 0050 point 2: "khi phiếu ở 'đã xử lý' / 'chờ dân xác nhận'".
func TestRatingOpenOnlyAtTheTwoStatuses(t *testing.T) {
	for st := range chuyenDuocSang {
		want := st == DaXuLy || st == ChoDanXacNhan
		if RatingOpen(st) != want {
			t.Errorf("RatingOpen(%s) = %v, muốn %v", st, !want, want)
		}
	}
	if RatingOpen("received") || RatingOpen("") {
		t.Error("mã lạ được coi là chấm được — phải từ chối (fail closed)")
	}
}

// TestRatingReopenThresholdIsTwo — the fixed number of ADR 0050, not configuration.
func TestRatingReopenThresholdIsTwo(t *testing.T) {
	for stars, want := range map[int]bool{1: true, 2: true, 3: false, 4: false, 5: false, 0: false} {
		if RatingReopens(stars) != want {
			t.Errorf("RatingReopens(%d) = %v, muốn %v", stars, !want, want)
		}
	}
}

// TestReopenByRatingAllowedFollowsTheMap — both reopen edges from the two rateable statuses are in the
// lifecycle map (the `da-xu-ly` one added 28/09/2026), and nothing else passes the gate.
func TestReopenByRatingAllowedFollowsTheMap(t *testing.T) {
	for st := range chuyenDuocSang {
		want := st == DaXuLy || st == ChoDanXacNhan
		if ReopenByRatingAllowed(st) != want {
			t.Errorf("ReopenByRatingAllowed(%s) = %v, muốn %v", st, !want, want)
		}
	}
}

// TestStaffAdvanceNeverReopens — the staff advance route moves ONLY along tienTrinhChinh, and from
// `da-xu-ly` that is `cho-dan-xac-nhan`. Adding `da-xu-ly -> dang-xu-ly` to the lifecycle map must not
// have given the staff route a way back.
//
// THE MUTATION THAT MUST TURN THIS RED: `DaXuLy: DangXuLy` in tienTrinhChinh.
func TestStaffAdvanceNeverReopens(t *testing.T) {
	for tu, sang := range tienTrinhChinh {
		if sang == DangXuLy && (tu == DaXuLy || tu == ChoDanXacNhan || tu == DaDong) {
			t.Errorf("tuyến tiến của cán bộ mở lại phiếu %s -> %s — chỉ dân chấm sao mới mở lại", tu, sang)
		}
	}
	if next, _ := TienTrinhChinh(DaXuLy); next != ChoDanXacNhan {
		t.Errorf("tiến từ da-xu-ly = %q, muốn cho-dan-xac-nhan", next)
	}
}

// TestReopenNextStepIsTheOnlyReopenSentence — the reopening owes the citizen a word (ADR 0041:53,
// ADR 0050), the restricted field gets code + status only, and the sentence carries no deadline and
// no threshold.
func TestReopenNextStepIsTheOnlyReopenSentence(t *testing.T) {
	msg := ReopenNextStep(PhieuPhanAnh{LinhVuc: "rac-thai", HanXuLyXong: hanXong})
	if msg == "" {
		t.Fatal("mở lại không có lời báo — luật 10 bất biến 5")
	}
	for _, cam := range []string{"sao", "2", "hạn", "trước"} {
		if strings.Contains(msg, cam) {
			t.Errorf("lời báo mở lại chứa %q — không nói ngưỡng, không hứa hạn: %q", cam, msg)
		}
	}
	if got := ReopenNextStep(PhieuPhanAnh{LinhVuc: LinhVucHanChe}); got != loiNhanHanChe {
		t.Errorf("phiếu can-bo được báo %q, muốn chỉ câu hạn chế", got)
	}
	// The staff table is untouched: entering dang-xu-ly by the staff advance still owes nothing.
	if BaoChoDan(DangXuLy) {
		t.Error("dang-xu-ly vào bảng báo — bước tiến của cán bộ sẽ nhắn dân (ADR 0041 §Không báo)")
	}
}
