package app

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
)

// The decision writes its note onto the request row (migration 0031), IN THE SAME UPDATE that decides
// — 0031's trigger refuses it anywhere else — and a blank note is NULL, never ''. The real store on
// the fake driver; the trigger and the CHECK themselves are proved in
// internal/store/task_extension_history_pg_test.go (SKIPS without VIGOV_TEST_DSN).

const decisionNoteText = "Đồng ý, báo cáo tiến độ hằng tuần."

func decideWithNote(t *testing.T, approve bool, note string) *khoNhiemVuGia {
	t.Helper()
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	leader := audit.Actor{ID: maLanhDao, Kind: "staff", IP: "10.0.0.8"}
	after, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: approve, GhiChu: note}, leader)
	if err != nil {
		t.Fatalf("quyết định: %v", err)
	}
	if want := strings.TrimSpace(note); after.DecisionNote != want {
		t.Errorf("DecisionNote trả về = %q, muốn %q", after.DecisionNote, want)
	}
	chiGhiTrongGiaoDich(t, k)
	return k
}

// decisionUpdate is the ONE deciding statement; the note must be in it, not in a second UPDATE.
func decisionUpdate(t *testing.T, k *khoNhiemVuGia) lenhPhieu {
	t.Helper()
	updates := k.cau("UPDATE de_nghi_lui_han")
	if len(updates) != 1 {
		t.Fatalf("chạy %d câu UPDATE de_nghi_lui_han, muốn đúng 1 — ghi chú phải đi cùng câu quyết định", len(updates))
	}
	if !strings.Contains(updates[0].sql, "decision_note = $6") {
		t.Fatalf("câu quyết định không ghi decision_note: %s", updates[0].sql)
	}
	return updates[0]
}

func TestDecisionWritesNoteInTheDecidingUpdate(t *testing.T) {
	for name, approve := range map[string]bool{"duyệt": true, "từ chối": false} {
		t.Run(name, func(t *testing.T) {
			k := decideWithNote(t, approve, "  "+decisionNoteText+"  ")
			u := decisionUpdate(t, k)
			if u.args[5] != decisionNoteText {
				t.Errorf("$6 = %#v, muốn ghi chú đã cắt khoảng trắng", u.args[5])
			}
			// The timeline entry keeps receiving the note exactly as before 0031.
			if logs := k.cau("INSERT INTO nhat_ky_nhiem_vu"); len(logs) != 1 || !containsArg(logs[0].args, decisionNoteText) {
				t.Error("nhật ký nhiệm vụ không còn mang ghi chú quyết định")
			}
		})
	}
}

func TestDecisionBlankNoteIsNull(t *testing.T) {
	for name, note := range map[string]string{"rỗng": "", "toàn khoảng trắng": "   \t "} {
		t.Run(name, func(t *testing.T) {
			u := decisionUpdate(t, decideWithNote(t, false, note))
			if u.args[5] != nil {
				t.Errorf("$6 = %#v, muốn NULL — '' không phải \"không có ghi chú\"", u.args[5])
			}
		})
	}
}

// The audit entry records the note's LENGTH, not its text: audit_log is append-only and never deleted,
// and the note is free text that may name people (rule 6, forbidden #4) — the `do_dai_ly_do` convention.
func TestDecisionAuditCarriesNoteLengthNotText(t *testing.T) {
	k := decideWithNote(t, true, decisionNoteText)
	delta := chuoiDelta(t, vetKiemToan(t, k))
	if strings.Contains(delta, decisionNoteText) {
		t.Errorf("delta mang nguyên văn ghi chú: %s", delta)
	}
	var d struct {
		After map[string]any `json:"sau"`
	}
	if err := json.Unmarshal([]byte(delta), &d); err != nil {
		t.Fatalf("delta không phải JSON: %v", err)
	}
	if got, want := d.After["do_dai_ghi_chu"], float64(len([]rune(decisionNoteText))); got != want {
		t.Errorf("sau.do_dai_ghi_chu = %v, muốn %v", got, want)
	}
}

func containsArg(args []driver.Value, want string) bool {
	for _, a := range args {
		if s, ok := a.(string); ok && s == want {
			return true
		}
	}
	return false
}
