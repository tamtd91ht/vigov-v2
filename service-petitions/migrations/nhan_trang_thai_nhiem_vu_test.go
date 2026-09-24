package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0010 (open question #21, ADR 0035 §C).
//
// WHAT THIS IS AND IS NOT. It reads the SQL this binary embeds and asserts the constraints are
// written there. It does NOT prove PostgreSQL enforces them — that is
// internal/store/nhan_trang_thai_nhiem_vu_pg_test.go, which needs VIGOV_TEST_DSN and SKIPS without
// it. This file exists because a pg suite that skips on most machines turns green whatever the SQL
// says: a widened or deleted CHECK must go red somewhere that always runs, and this is that
// somewhere. It is the weaker half, and it is written down as the weaker half.

const (
	tep0006 = "0006_nhiem_vu.sql"
	tep0010 = "0010_nhan_trang_thai_nhiem_vu.sql"
)

// maChay returns the executable part of a migration: `--` comments removed, whitespace collapsed,
// lower-cased. Comments are removed FIRST because 0010's header writes the rejected
// `UNIQUE (tenant_id, thu_tu)` and the absent `dang_dung` out in prose — a test matching comments
// would read the explanation as the DDL.
func maChay(t *testing.T, ten string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, ten)
	if err != nil {
		t.Fatalf("đọc %s từ FS nhúng: %v", ten, err)
	}
	var dong []string
	for _, d := range strings.Split(string(b), "\n") {
		if i := strings.Index(d, "--"); i >= 0 {
			d = d[:i]
		}
		dong = append(dong, d)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(dong, " ")), " "))
}

// danhSachMaTrongCheck returns the quoted codes of `CONSTRAINT <ten> CHECK (<cot> IN (...))`,
// sorted. It FAILS when the constraint is not found: a parser that went blind must not read as
// "both lists are empty, therefore equal".
func danhSachMaTrongCheck(t *testing.T, sql, ten, cot string) []string {
	t.Helper()
	m := regexp.MustCompile(`constraint ` + regexp.QuoteMeta(ten) + ` check \(` +
		regexp.QuoteMeta(cot) + ` in \(([^)]*)\)\)`).FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("không tìm thấy ràng buộc %s CHECK (%s IN (...))", ten, cot)
	}
	var ma []string
	for _, x := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(m[1], -1) {
		ma = append(ma, x[1])
	}
	sort.Strings(ma)
	return ma
}

// TestMigration0010MaTrungKhopMayTrangThai IS THE ASSERTION #21 RESTS ON.
//
// A label row for a code the state machine does not have is a state with no way in and no way out
// (open question #21). The two lists — 0006's status CHECK and 0010's label CHECK — must be the
// SAME SET, and exactly seven.
//
// THE MUTATION THAT MUST TURN THIS RED: add an eighth code to `nhan_trang_thai_nhiem_vu_ma_hop_le`
// in 0010 (or drop one). Also red: a later migration widening 0006's list without widening this.
func TestMigration0010MaTrungKhopMayTrangThai(t *testing.T) {
	mayTrangThai := danhSachMaTrongCheck(t, maChay(t, tep0006), "nhiem_vu_trang_thai_hop_le", "trang_thai")
	nhan := danhSachMaTrongCheck(t, maChay(t, tep0010), "nhan_trang_thai_nhiem_vu_ma_hop_le", "ma")

	if len(mayTrangThai) != 7 {
		t.Fatalf("0006 khai %d mã trạng thái, mong 7 (§6) — bộ phân tích đã mù hoặc 0006 đã đổi: %v",
			len(mayTrangThai), mayTrangThai)
	}
	if strings.Join(nhan, ",") != strings.Join(mayTrangThai, ",") {
		t.Errorf("mã trong CHECK của 0010 KHÁC máy trạng thái 0006:\n  0010: %v\n  0006: %v\n"+
			"một nhãn cho mã không có trong máy trạng thái là một trạng thái không lối vào, không lối ra (#21)",
			nhan, mayTrangThai)
	}
}

// TestMigration0010KhoaVaRangBuoc — the key is composite with tenant_id (rule 1, invariant 6), the
// table is hash-partitioned like its siblings, and the label / order / author CHECKs are there.
func TestMigration0010KhoaVaRangBuoc(t *testing.T) {
	sql := maChay(t, tep0010)

	for _, c := range []struct{ can, vi string }{
		{"primary key (tenant_id, ma)", "khoá chính phải hợp thành với tenant_id (luật 1 bất biến 6)"},
		{") partition by hash (tenant_id);", "bảng phải phân mảnh hash theo tenant_id như các bảng anh em (ADR 0010)"},
		{"partition of nhan_trang_thai_nhiem_vu ' 'for values with (modulus 32, remainder %s)",
			"thiếu vòng tạo 32 mảnh — bảng phân mảnh không mảnh từ chối mọi INSERT"},
		{"constraint nhan_trang_thai_nhiem_vu_nhan_khong_rong check (btrim(nhan) <> '')",
			"nhãn rỗng là một cột Kanban không tên"},
		{"constraint nhan_trang_thai_nhiem_vu_nhan_toi_da check (char_length(nhan) <= 100)",
			"nhãn phải có trần, đếm theo KÝ TỰ (char_length), không theo byte"},
		{"constraint nhan_trang_thai_nhiem_vu_thu_tu_tu_mot check (thu_tu >= 1)", "thứ tự bắt đầu từ 1"},
		{"constraint nhan_trang_thai_nhiem_vu_cap_nhat_boi_khong_rong check (btrim(cap_nhat_boi) <> '')",
			"người sửa phải có mã cán bộ (luật 6 bất biến 8)"},
		{"cap_nhat_boi text not null", "cap_nhat_boi phải NOT NULL — CHECK coi NULL là đạt"},
		{"before update or delete on nhan_trang_thai_nhiem_vu for each row execute function nhan_trang_thai_nhiem_vu_bat_bien()",
			"trigger chặn xoá cứng / đổi mã / đổi xã đã mất"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0010 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0010KhongCoNutTatKhongXoaMem states three ABSENCES as assertions, so each decision
// cannot be undone by a "tidy-up" that looks harmless in a diff.
//
//	dang_dung            ADR 0035 §C: no `Tắt` — a disabled status drops its tasks out of every filter
//	deleted_at           configuration, not an archival record; "reset" is a write, not a delete
//	unique (…, thu_tu)   partial overrides and swaps; order is display-only (0010's header)
func TestMigration0010KhongCoNutTatKhongXoaMem(t *testing.T) {
	sql := maChay(t, tep0010)
	for _, c := range []struct{ cam, vi string }{
		{"dang_dung", "có cột dang_dung — #21 / ADR 0035 §C: nhóm này KHÔNG có nút Tắt"},
		{"deleted_at", "có cột deleted_at — bảng cấu hình ghi đè tại chỗ; 'về mặc định' là một lần GHI"},
		{"unique (tenant_id, thu_tu)", "có khoá duy nhất trên thu_tu — chặn mọi lần đổi chỗ hai trạng thái"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Error("0010 " + c.vi)
		}
	}
}
