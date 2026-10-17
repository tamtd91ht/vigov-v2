package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0046 (normalised sender phone + its index, ADR 0102 #3). It reads the
// SQL this binary embeds; it does NOT prove PostgreSQL accepts it or plans with the index — that is
// tools/schema-smoke, which needs VIGOV_TEST_DSN.

const file0046 = "0046_petition_sender_phone_normalized.sql"

// TestMigration0046IsAdditiveOnly — rule 7: the file adds a nullable column and an index and writes no
// row. Above all it holds NO normalisation logic: the phone rule lives once, in core (ADR 0102 #3), and a
// SQL copy would drift from it silently — a short contact list with no error anywhere.
func TestMigration0046IsAdditiveOnly(t *testing.T) {
	sql := maChay(t, file0046)
	for _, c := range []struct{ banned, why string }{
		{"drop ", "tệp này không xoá gì — lệnh đảo ngược chỉ nằm trong chú thích REVERSAL"},
		{"alter column", "không đổi kiểu hay NOT NULL của cột có sẵn"},
		{"delete from", "không xoá dòng nào"},
		{"truncate", "không làm rỗng bảng nào"},
		{"update ", "không ghi dữ liệu — việc điền lại là job Go dùng hàm chuẩn hoá của core"},
		{"insert into", "không ghi dữ liệu"},
		{"generated always", "không cột sinh — cần luật chuẩn hoá bằng SQL, là bản sao thứ hai"},
		{"regexp_replace", "không chuẩn hoá số bằng SQL — một luật, một chỗ (core)"},
		{"translate(", "không chuẩn hoá số bằng SQL — một luật, một chỗ (core)"},
		{"not null default", "cột phải nullable, không mặc định — dòng cũ chưa điền lại là NULL, không phải ''"},
		{"nguoi_gui_dien_thoai =", "cột thô giữ nguyên (luật 7)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0046 chứa %q — %s", c.banned, c.why)
		}
	}
}

// TestMigration0046IndexServesTheLinkQuery — tenant first (rule 1), anonymous and soft-deleted rows out
// (ADR 0102 #4, rule 7 invariant 2), keyset order matching QueryPage's default register sort.
func TestMigration0046IndexServesTheLinkQuery(t *testing.T) {
	sql := maChay(t, file0046)
	for _, c := range []struct{ want, why string }{
		{"alter table phieu_phan_anh add column if not exists sender_phone_normalized text constraint",
			"cột mới, nullable, trên bảng cha phân vùng (lan xuống 8 phân vùng)"},
		{"check (sender_phone_normalized is null or sender_phone_normalized <> '')",
			"chuỗi rỗng sẽ nối hai người gửi để trống ô với nhau"},
		{"create index if not exists phieu_phan_anh_by_sender_phone on phieu_phan_anh " +
			"(tenant_id, sender_phone_normalized, vao_so_luc desc, id desc)",
			"tenant_id đứng đầu (luật 1); thứ tự sắp khớp booked_at mặc định của sổ"},
		{"where an_danh = false and deleted_at is null and sender_phone_normalized is not null;",
			"ẩn danh không bao giờ khớp (ADR 0102 #4); dòng xoá mềm bị loại (luật 7)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0046 thiếu %q — %s", c.want, c.why)
		}
	}
}
