package store

import (
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration tests for migration 0016 — `han_ban_dau` follows a deadline correction only while the
// task has had no approved extension.
//
// ⚠ THESE SKIP UNLESS VIGOV_TEST_DSN IS SET, and the package still prints `ok` — see the header of
// nhiem_vu_pg_test.go. None of the assertions below has been executed in the environment they were
// written in. The fake-driver suite (internal/app/task_deadline_correction_test.go) proves which
// statement the write path sends; only these prove the trigger honours the rule.

var (
	dueCorrectedPg = time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	dueExtendedPg  = time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
)

// addApprovedExtension files one APPROVED extension request against a task, soft-deleted when asked.
func addApprovedExtension(t *testing.T, xa, taskID string, deleted bool) {
	t.Helper()
	db := moKetNoi(t)
	if _, err := db.Exec(
		`INSERT INTO de_nghi_lui_han (tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, nguoi_duyet_ma,
		   han_moi, ly_do, trang_thai, thoi_diem, duyet_luc)
		 VALUES ($1, $2, $3, 'CB-00311', 'CB-00007', $4, 'Chờ số liệu.', 'da-duyet', now(), now())`,
		xa, "dn-"+taskID, taskID, dueExtendedPg); err != nil {
		t.Fatalf("thêm đề nghị đã duyệt: %v", err)
	}
	if deleted {
		if _, err := db.Exec(
			`UPDATE de_nghi_lui_han SET deleted_at = now(), deleted_by = 'CB-00123', delete_reason = 'nhập nhầm'
			 WHERE tenant_id = $1 AND id = $2`, xa, "dn-"+taskID); err != nil {
			t.Fatalf("xoá mềm đề nghị: %v", err)
		}
	}
}

func correct(t *testing.T, xa, id string, old time.Time, withOriginal bool) error {
	t.Helper()
	kho := pkgstore.New(moKetNoi(t))
	s := NewNhiemVuStore(kho)
	ctx := ctxXa(tenant.ID(xa))
	return kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.CorrectDeadline(ctx, tx, id, old, dueCorrectedPg, withOriginal)
	})
}

func deadlines(t *testing.T, xa, id string) (current, original time.Time) {
	t.Helper()
	if err := moKetNoi(t).QueryRow(
		`SELECT han_xu_ly, han_ban_dau FROM nhiem_vu WHERE tenant_id = $1 AND id = $2`, xa, id,
	).Scan(&current, &original); err != nil {
		t.Fatalf("đọc hạn: %v", err)
	}
	return current, original
}

// TestPgCorrectionBeforeExtensionCarriesOriginal — branch 1: no approved extension, both move.
func TestPgCorrectionBeforeExtensionCarriesOriginal(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-dc-1", "NV01", "theo-van-ban", "moi-giao",
		mocHanPg, mocHanPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	if err := correct(t, xa, "nv-dc-1", mocHanPg, true); err != nil {
		t.Fatalf("sửa hạn trước khi có gia hạn bị chặn: %v", err)
	}
	cur, orig := deadlines(t, xa, "nv-dc-1")
	if !cur.Equal(dueCorrectedPg) || !orig.Equal(dueCorrectedPg) {
		t.Errorf("hạn %v / gốc %v, muốn cả hai %v", cur, orig, dueCorrectedPg)
	}
}

// TestPgCorrectionAfterExtensionFreezesOriginal — branch 2: after an approved extension — SOFT-DELETED
// OR NOT — the trigger refuses carrying `han_ban_dau`, and a correction of `han_xu_ly` alone passes.
func TestPgCorrectionAfterExtensionFreezesOriginal(t *testing.T) {
	// SKIP AT THE PARENT, not only inside the subtests: otherwise the parent reports PASS over two
	// skipped children, a green that proves nothing.
	moKetNoi(t)
	for name, deleted := range map[string]bool{"đề nghị còn hiệu lực": false, "đề nghị đã xoá mềm": true} {
		t.Run(name, func(t *testing.T) {
			db := moKetNoi(t)
			xa, _ := xaRieng(t)
			danhMucChoXa(t, db, xa)
			if err := themNhiemVu(db, xa, "nv-dc-2", "NV02", "theo-van-ban", "dang-thuc-hien",
				mocHanGocPg, mocHanGocPg, nil); err != nil {
				t.Fatalf("thêm nhiệm vụ: %v", err)
			}
			addApprovedExtension(t, xa, "nv-dc-2", deleted)
			if _, err := db.Exec(`UPDATE nhiem_vu SET han_xu_ly = $3 WHERE tenant_id = $1 AND id = $2`,
				xa, "nv-dc-2", dueExtendedPg); err != nil {
				t.Fatalf("áp gia hạn: %v", err)
			}

			canMaLoi(t, correct(t, xa, "nv-dc-2", dueExtendedPg, true), "P0001",
				"đã có gia hạn được duyệt mà hạn ban đầu vẫn đi theo — §11.3 mất mẫu số")

			if err := correct(t, xa, "nv-dc-2", dueExtendedPg, false); err != nil {
				t.Fatalf("sửa riêng hạn xử lý bị chặn: %v", err)
			}
			cur, orig := deadlines(t, xa, "nv-dc-2")
			if !cur.Equal(dueCorrectedPg) || !orig.Equal(mocHanGocPg) {
				t.Errorf("hạn %v / gốc %v, muốn %v / %v", cur, orig, dueCorrectedPg, mocHanGocPg)
			}
		})
	}
}

// TestPgOriginalNeverTakesAValueOfItsOwn — even with no extension, `han_ban_dau` may only FOLLOW
// `han_xu_ly`; the approved count is checked through the real store as well, soft-deleted included.
func TestPgOriginalNeverTakesAValueOfItsOwn(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	danhMucChoXa(t, db, xa)
	if err := themNhiemVu(db, xa, "nv-dc-3", "NV03", "theo-van-ban", "moi-giao",
		mocHanPg, mocHanPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ: %v", err)
	}
	_, err := db.Exec(`UPDATE nhiem_vu SET han_xu_ly = $3, han_ban_dau = $4 WHERE tenant_id = $1 AND id = $2`,
		xa, "nv-dc-3", dueCorrectedPg, dueExtendedPg)
	canMaLoi(t, err, "P0001", "hạn ban đầu nhận một giá trị riêng, không theo hạn xử lý")

	addApprovedExtension(t, xa, "nv-dc-3", true)
	kho := pkgstore.New(db)
	ctx := ctxXa(tenant.ID(xa))
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		n, err := NewDeNghiLuiHanStore(kho).ApprovedCount(ctx, tx, "nv-dc-3")
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("đếm đề nghị đã duyệt = %d, muốn 1 (kể cả đã xoá mềm)", n)
		}
		return nil
	}); err != nil {
		t.Fatalf("đếm: %v", err)
	}
}
