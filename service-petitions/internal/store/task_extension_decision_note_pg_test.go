package store

import (
	"strings"
	"testing"
)

// Integration tests for migration 0031 — `de_nghi_lui_han.decision_note` is written only in the UPDATE
// that decides the request, and frozen afterwards.
//
// ⚠ THESE SKIP UNLESS VIGOV_TEST_DSN IS SET, and the package still prints `ok` — see the header of
// nhiem_vu_pg_test.go. The schema-text suite (migrations/task_extension_decision_note_test.go) proves
// the rule is WRITTEN; only these prove PostgreSQL enforces it.

// decisionNoteFixture files one pending request against a fresh task in a fresh commune.
func decisionNoteFixture(t *testing.T, taskID, taskCode, reqID string) string {
	t.Helper()
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, taskID, taskCode, "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if err := themDeNghi(db, xa, reqID, taskID, "cho-duyet", nil, nil); err != nil {
		t.Fatalf("gửi đề nghị: %v", err)
	}
	return xa
}

// TestPgDecisionNoteWrittenWithDecisionThenFrozen — the one legitimate write passes, for both outcomes;
// every write after it is refused by the trigger (P0001).
func TestPgDecisionNoteWrittenWithDecisionThenFrozen(t *testing.T) {
	moKetNoi(t) // skip at the parent, not only inside the subtests
	for i, outcome := range []string{"da-duyet", "tu-choi"} {
		t.Run(outcome, func(t *testing.T) {
			taskID := "nv-dnn-" + outcome
			xa := decisionNoteFixture(t, taskID, []string{"NV80", "NV81"}[i], "dnn-"+outcome)
			db := moKetNoi(t)

			if _, err := db.Exec(
				`UPDATE de_nghi_lui_han
				 SET trang_thai = $3, nguoi_duyet_ma = 'CB-00007', duyet_luc = now(), decision_note = $4
				 WHERE tenant_id = $1 AND id = $2`,
				xa, "dnn-"+outcome, outcome, "Đồng ý, cần báo cáo tiến độ hằng tuần."); err != nil {
				t.Fatalf("ghi quyết định kèm ghi chú bị chặn: %v", err)
			}

			for name, stmt := range map[string]string{
				"sửa ghi chú": `UPDATE de_nghi_lui_han SET decision_note = 'Viết lại.' WHERE tenant_id = $1 AND id = $2`,
				"xoá ghi chú": `UPDATE de_nghi_lui_han SET decision_note = NULL WHERE tenant_id = $1 AND id = $2`,
			} {
				_, err := db.Exec(stmt, xa, "dnn-"+outcome)
				canMaLoi(t, err, "P0001", name+" sau khi đã quyết định")
			}

			// A soft delete must still pass: it does not touch the note.
			if _, err := db.Exec(
				`UPDATE de_nghi_lui_han SET deleted_at = now(), deleted_by = 'CB-00123', delete_reason = 'nhập nhầm'
				 WHERE tenant_id = $1 AND id = $2`, xa, "dnn-"+outcome); err != nil {
				t.Fatalf("xoá mềm đề nghị đã có ghi chú bị chặn: %v", err)
			}
		})
	}
}

// TestPgDecisionNoteRefusedWhilePending — a note on a pending request (CHECK, 23514), a blank note, and
// a note over 5000 characters are refused; a decision without a note still passes (today's write path).
func TestPgDecisionNoteRefusedWhilePending(t *testing.T) {
	xa := decisionNoteFixture(t, "nv-dnn-p", "NV82", "dnn-p")
	db := moKetNoi(t)

	_, err := db.Exec(`UPDATE de_nghi_lui_han SET decision_note = 'Sớm.' WHERE tenant_id = $1 AND id = $2`,
		xa, "dnn-p")
	if err == nil {
		t.Fatal("ghi được ghi chú quyết định lên đề nghị còn chờ duyệt")
	}

	decide := `UPDATE de_nghi_lui_han
		SET trang_thai = 'da-duyet', nguoi_duyet_ma = 'CB-00007', duyet_luc = now(), decision_note = $3
		WHERE tenant_id = $1 AND id = $2`
	_, err = db.Exec(decide, xa, "dnn-p", "   ")
	canMaLoi(t, err, "23514", "ghi chú rỗng")
	_, err = db.Exec(decide, xa, "dnn-p", strings.Repeat("ạ", 5001))
	canMaLoi(t, err, "23514", "ghi chú quá 5000 ký tự")

	if _, err := db.Exec(
		`UPDATE de_nghi_lui_han SET trang_thai = 'tu-choi', nguoi_duyet_ma = 'CB-00007', duyet_luc = now()
		 WHERE tenant_id = $1 AND id = $2`, xa, "dnn-p"); err != nil {
		t.Fatalf("quyết định không ghi chú (đường ghi hiện tại) bị chặn: %v", err)
	}

	// The UPDATE above is refused by the trigger first; the CHECK is what refuses an INSERT that files
	// a pending request already carrying a note. The slot is free again now that dnn-p is decided.
	_, err = db.Exec(
		`INSERT INTO de_nghi_lui_han
		 (tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, han_moi, ly_do, trang_thai, thoi_diem, decision_note)
		 VALUES ($1, 'dnn-p2', 'nv-dnn-p', 'CB-00311', $2, 'Chờ số liệu.', 'cho-duyet', now(), 'Sớm.')`,
		xa, mocHanPg)
	canMaLoi(t, err, "23514", "đề nghị chờ duyệt mang sẵn ghi chú quyết định")
}
