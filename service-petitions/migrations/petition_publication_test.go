package migrations

import (
	"os"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0017 (three-state `publication_status`, optional `rating_comment`,
// ADR 0050 points 2 and 8). Same standing as ket_thuc_nhanh_phan_anh_test.go: it reads the SQL this
// binary embeds and asserts the columns, the backfill and the constraints are WRITTEN. It does NOT
// prove PostgreSQL applies the file or evaluates the CHECKs; that needs VIGOV_TEST_DSN and a pg suite
// in internal/store. It exists so that a widened list, a lost never-public CHECK or a lossy drop goes
// red somewhere that always runs.

const tep0017 = "0017_petition_publication_and_rating_comment.sql"

// TestMigration0017ColumnsAndConstraints — THE MUTATIONS THAT MUST TURN THIS RED: a default other
// than 'cho-duyet' (every petition would be born public or hidden); a fourth value, or a lost one, in
// the closed list; dropping the never-public CHECK or writing it with `<>` on `linh_vuc` (NULL would
// then pass as NULL, not TRUE); a cap other than 1000 characters, or counted in bytes; a comment
// allowed without stars.
func TestMigration0017ColumnsAndConstraints(t *testing.T) {
	sql := maChay(t, tep0017)
	for _, c := range []struct{ can, vi string }{
		{"add column if not exists publication_status text not null default 'cho-duyet';",
			"trạng thái công khai phải có, NOT NULL, mặc định chờ duyệt"},
		{"add column if not exists rating_comment text;", "nhận xét kèm sao phải có và NULLABLE"},
		{"check (publication_status in ('cho-duyet', 'cong-khai', 'an'))",
			"danh sách đóng đúng ba giá trị"},
		{"check (linh_vuc is distinct from 'can-bo' or publication_status <> 'cong-khai')",
			"phiếu tác phong cán bộ không bao giờ công khai — cho mọi đường ghi"},
		{"check (rating_comment is null or (btrim(rating_comment) <> '' and char_length(rating_comment) <= 1000))",
			"nhận xét: không rỗng, trần 1000 KÝ TỰ"},
		{"check (rating_comment is null or diem_hai_long is not null)",
			"nhận xét chỉ đi kèm một lần chấm sao"},
		{"comment on column phieu_phan_anh.hien_cong_khai is",
			"cột cũ phải được đánh dấu bị thay, không bị xoá"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0017 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0017BackfillIsAuditedAndResumable — both UPDATEs match only rows still at the default
// (a retry writes and audits nothing twice), the staff-conduct one runs FIRST, the carry-over excludes
// staff conduct, and each writes its audit entry in the same statement with the system principal and
// the business code as subject.
func TestMigration0017BackfillIsAuditedAndResumable(t *testing.T) {
	sql := maChay(t, tep0017)
	hidden := "update phieu_phan_anh set publication_status = 'an', cap_nhat_luc = now() " +
		"where linh_vuc = 'can-bo' and publication_status = 'cho-duyet' returning tenant_id, ma_tra_cuu"
	published := "update phieu_phan_anh set publication_status = 'cong-khai', cap_nhat_luc = now() " +
		"where hien_cong_khai = true and linh_vuc is distinct from 'can-bo' and publication_status = 'cho-duyet' " +
		"returning tenant_id, ma_tra_cuu"
	iH, iP := strings.Index(sql, hidden), strings.Index(sql, published)
	if iH < 0 {
		t.Errorf("0017 thiếu backfill tác phong cán bộ → 'an' chỉ trên dòng còn mặc định: %q", hidden)
	}
	if iP < 0 {
		t.Errorf("0017 thiếu backfill hien_cong_khai → 'cong-khai' loại trừ can-bo: %q", published)
	}
	if iH >= 0 && iP >= 0 && iH > iP {
		t.Error("0017: backfill 'an' phải chạy TRƯỚC — nếu không, một phiếu can-bo có thể lên 'cong-khai'")
	}
	audit := "insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) " +
		"select tenant_id, 'system', 'system', '', 'dat_trang_thai_cong_khai', ma_tra_cuu, now(),"
	if n := strings.Count(sql, audit); n != 2 {
		t.Errorf("0017 có %d lệnh ghi vết hệ thống, mong 2 — mỗi backfill một, cùng câu lệnh (luật 6)", n)
	}
	// The never-public CHECK must come after the backfill it validates.
	if iC := strings.Index(sql, "add constraint phieu_phan_anh_staff_conduct_never_public"); iC < 0 || iC < iP {
		t.Error("0017: ràng buộc không-công-khai phải thêm SAU backfill")
	}
}

// TestMigration0017FieldCodeMatchesDomain — the CHECK and the backfill name the staff-conduct field
// by literal; the Go side names it domain.LinhVucHanChe. If that constant ever changes, the CHECK
// silently guards a code nobody writes.
func TestMigration0017FieldCodeMatchesDomain(t *testing.T) {
	b, err := os.ReadFile("../internal/domain/xu_ly_phan_anh.go")
	if err != nil {
		t.Fatalf("đọc domain: %v", err)
	}
	if !strings.Contains(string(b), `const LinhVucHanChe = "can-bo"`) {
		t.Error("domain.LinhVucHanChe không còn là \"can-bo\" — ràng buộc và backfill của 0017 đang canh sai mã")
	}
}

// TestMigration0017IsNotLossy — nothing is dropped, retyped or emptied; the archival guards of 0004
// and 0011 are not replaced. The executable text is checked (comments stripped by maChay), so the
// REVERSAL prose does not trip it.
func TestMigration0017IsNotLossy(t *testing.T) {
	sql := maChay(t, tep0017)
	for _, c := range []struct{ cam, vi string }{
		{"drop ", "xoá một đối tượng — lệnh đảo ngược chỉ nằm trong chú thích REVERSAL"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"delete ", "xoá dòng"},
		{"set hien_cong_khai", "ghi vào cột đã bị thay"},
		{"function ho_so_luu_tru_bat_bien", "thay hàm canh lưu trữ của 0004"},
		{"function phieu_phan_anh_ket_thuc_nhanh_bat_bien", "thay hàm canh của 0011"},
		{"so_lan_mo_lai_toi_da", "đọc trần mở lại đã bị ADR 0050 điểm 2 bỏ"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0017 chứa %q — %s", c.cam, c.vi)
		}
	}
}
