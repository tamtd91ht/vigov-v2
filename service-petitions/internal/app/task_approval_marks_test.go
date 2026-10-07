package app

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// PATCH /api/v1/tasks/{ma} and §5.4's two approval marks (user decision 07/10/2026): both marks are
// in the audit entry's before/after, each mark that MOVES writes one timeline row in the same
// transaction, a mark sent with its current value writes none, and neither touches the status.
//
// The fixture row (dongNhiemVuGia) holds leader_approved = TRUE and superior_acknowledged = FALSE,
// so "untick the leader, tick the superior" exercises both directions in one act.

func approvalDelta(t *testing.T, k *khoNhiemVuGia) (before, after map[string]any, raw string) {
	t.Helper()
	raw = chuoiDelta(t, vetKiemToan(t, k))
	var d struct {
		Before map[string]any `json:"truoc"`
		After  map[string]any `json:"sau"`
	}
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("delta không phải JSON: %v", err)
	}
	return d.Before, d.After, raw
}

func TestTaskEdit_ApprovalMarksBothDirections(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	off, on := false, true
	after, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{
		LanhDaoPheDuyetHoanThanh: &off, CapTrenCongNhanHoanThanh: &on,
	}, canBoThu())
	if err != nil {
		t.Fatalf("sửa dấu duyệt: %v", err)
	}
	chiGhiTrongGiaoDich(t, k)

	if after.LanhDaoPheDuyetHoanThanh || !after.CapTrenCongNhanHoanThanh {
		t.Errorf("bản ghi trả về: lãnh đạo=%v cấp trên=%v, muốn false/true",
			after.LanhDaoPheDuyetHoanThanh, after.CapTrenCongNhanHoanThanh)
	}
	// THE MARKS CHANGE NO STATUS (migration 0006:302-304).
	if after.TrangThai != domain.DangThucHien {
		t.Errorf("trạng thái = %q, dấu duyệt không được đổi trạng thái", after.TrangThai)
	}

	logs := k.cau("INSERT INTO nhat_ky_nhiem_vu")
	if len(logs) != 2 {
		t.Fatalf("ghi %d dòng nhật ký, muốn 2 (mỗi dấu đổi một dòng)", len(logs))
	}
	want := []string{
		"Bỏ đánh dấu: " + leaderApprovedLabel,
		"Đánh dấu: " + superiorAcknowledgedLabel,
	}
	for i, w := range want {
		if !containsArg(logs[i].args, w) {
			t.Errorf("dòng nhật ký %d không mang %q", i+1, w)
		}
		if !containsArg(logs[i].args, string(domain.DangThucHien)) {
			t.Errorf("dòng nhật ký %d không ghi trạng thái hiện tại %q", i+1, domain.DangThucHien)
		}
	}

	before, afterD, _ := approvalDelta(t, k)
	if before["lanh_dao_phe_duyet"] != true || before["cap_tren_cong_nhan"] != false {
		t.Errorf("truoc: lanh_dao_phe_duyet=%v cap_tren_cong_nhan=%v, muốn true/false",
			before["lanh_dao_phe_duyet"], before["cap_tren_cong_nhan"])
	}
	if afterD["lanh_dao_phe_duyet"] != false || afterD["cap_tren_cong_nhan"] != true {
		t.Errorf("sau: lanh_dao_phe_duyet=%v cap_tren_cong_nhan=%v, muốn false/true",
			afterD["lanh_dao_phe_duyet"], afterD["cap_tren_cong_nhan"])
	}
}

// A mark sent with the value it already holds writes no timeline row; the act that carries it still
// records both marks (unchanged) in the audit entry.
func TestTaskEdit_UnchangedMarkWritesNoTimelineRow(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	on := true // leader's mark already TRUE on the fixture row
	title := "Tiêu đề mới của nhiệm vụ"
	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{
		TieuDe: &title, LanhDaoPheDuyetHoanThanh: &on,
	}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	chiGhiTrongGiaoDich(t, k)
	if n := len(k.cau("INSERT INTO nhat_ky_nhiem_vu")); n != 0 {
		t.Errorf("ghi %d dòng nhật ký dù không dấu nào đổi, muốn 0", n)
	}
	before, after, _ := approvalDelta(t, k)
	if before["lanh_dao_phe_duyet"] != true || after["lanh_dao_phe_duyet"] != true ||
		before["cap_tren_cong_nhan"] != false || after["cap_tren_cong_nhan"] != false {
		t.Errorf("delta dấu duyệt sai: truoc=%v sau=%v", before, after)
	}
}

// Sending ONLY a mark with its current value is a no-op: no UPDATE, no timeline row, no audit entry.
func TestTaskEdit_OnlyUnchangedMarkWritesNothing(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	off := false // superior's mark already FALSE
	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{CapTrenCongNhanHoanThanh: &off},
		canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "UPDATE") || strings.HasPrefix(l.sql, "INSERT") {
			t.Errorf("ghi dù không có gì đổi: %q", l.sql)
		}
	}
}

// The result summary and the note reach the audit entry as LENGTHS, never as text.
func TestTaskEdit_AuditCarriesSummaryAndNoteLengthOnly(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	summary := "Đã hoàn thành rà soát, gửi báo cáo về văn phòng."
	note := "Chờ số liệu bổ sung từ chi bộ."
	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{TomTatKetQua: &summary, GhiChu: &note},
		canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	chiGhiTrongGiaoDich(t, k)

	before, after, raw := approvalDelta(t, k)
	for _, text := range []string{summary, note} {
		if strings.Contains(raw, text) {
			t.Errorf("delta mang nguyên văn văn bản tự do: %s", raw)
		}
	}
	if before["do_dai_tom_tat_ket_qua"] != float64(0) || before["do_dai_ghi_chu"] != float64(0) {
		t.Errorf("truoc: độ dài = %v / %v, muốn 0 / 0",
			before["do_dai_tom_tat_ket_qua"], before["do_dai_ghi_chu"])
	}
	if got, want := after["do_dai_tom_tat_ket_qua"], float64(utf8.RuneCountInString(summary)); got != want {
		t.Errorf("sau.do_dai_tom_tat_ket_qua = %v, muốn %v", got, want)
	}
	if got, want := after["do_dai_ghi_chu"], float64(utf8.RuneCountInString(note)); got != want {
		t.Errorf("sau.do_dai_ghi_chu = %v, muốn %v", got, want)
	}
	if n := len(k.cau("INSERT INTO nhat_ky_nhiem_vu")); n != 0 {
		t.Errorf("ghi %d dòng nhật ký cho tóm tắt/ghi chú, muốn 0", n)
	}
}

func TestApprovalMarkLogLines(t *testing.T) {
	n := func(l, s bool) domain.NhiemVu {
		return domain.NhiemVu{LanhDaoPheDuyetHoanThanh: l, CapTrenCongNhanHoanThanh: s}
	}
	cases := []struct {
		name          string
		before, after domain.NhiemVu
		want          []string
	}{
		{"không đổi", n(true, false), n(true, false), nil},
		{"đánh dấu lãnh đạo", n(false, false), n(true, false), []string{"Đánh dấu: " + leaderApprovedLabel}},
		{"bỏ dấu cấp trên", n(false, true), n(false, false), []string{"Bỏ đánh dấu: " + superiorAcknowledgedLabel}},
		{"cả hai", n(false, false), n(true, true),
			[]string{"Đánh dấu: " + leaderApprovedLabel, "Đánh dấu: " + superiorAcknowledgedLabel}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := approvalMarkLogLines(c.before, c.after)
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("= %q, muốn %q", got, c.want)
			}
		})
	}
}
