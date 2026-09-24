package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// Schema-TEXT checks for migration 0008 (budget batches + the `entries` calculation mode, user
// decision 25/09/2026).
//
// WHAT THIS IS AND IS NOT. It reads the SQL this binary embeds and asserts the constraints and
// triggers are WRITTEN. It does NOT prove PostgreSQL accepts the file, that the DROP/ADD of the
// CHECK recurses into the 32 partitions, or that the triggers refuse what they claim to — that needs
// VIGOV_TEST_DSN. It exists so that a widened or deleted CHECK, or a removed trigger, goes red
// somewhere that always runs. It is the weaker half, and it is written down as the weaker half.

const (
	tep0006 = "0006_thu_chi_ngan_sach.sql"
	tep0008 = "0008_dot_thu_chi.sql"
)

// maChay returns the executable part of a migration: `--` comments removed, whitespace collapsed,
// lower-cased. Comments go FIRST because 0008's header and REVERSAL write rejected and down-path SQL
// out in prose — a test matching comments would read the explanation as the DDL.
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

// TestMigration0008MoRongCachTinhKhongMatGiaTri — the widening is LOSSLESS and matches Go.
//
// THE MUTATIONS THAT MUST TURN THIS RED: drop `manual` or `children` from 0008's list (every
// existing line then fails re-validation, or worse, a later narrowing slips in as a "widening");
// add a fourth code 0008 has no table for; rename a Go constant's value.
func TestMigration0008MoRongCachTinhKhongMatGiaTri(t *testing.T) {
	cu := danhSachMaTrongCheck(t, maChay(t, tep0006), "khoan_muc_ngan_sach_cach_tinh_hop_le", "cach_tinh")
	moi := danhSachMaTrongCheck(t, maChay(t, tep0008), "khoan_muc_ngan_sach_cach_tinh_hop_le", "cach_tinh")

	if strings.Join(cu, ",") != "children,manual" {
		t.Fatalf("0006 khai %v, mong [children manual] — bộ phân tích đã mù hoặc 0006 đã bị sửa", cu)
	}
	coMoi := map[string]bool{}
	for _, m := range moi {
		coMoi[m] = true
	}
	for _, m := range cu {
		if !coMoi[m] {
			t.Errorf("0008 bỏ mất mã %q của 0006 — mở rộng phải là tập cha, không thì dòng có sẵn hỏng khi kiểm lại", m)
		}
	}

	goConst := []string{string(domain.TinhTay), string(domain.TinhTheoDot), string(domain.TinhTheoCon)}
	sort.Strings(goConst)
	if strings.Join(moi, ",") != strings.Join(goConst, ",") {
		t.Errorf("CHECK của 0008 %v KHÁC hằng Go %v — một mã chỉ có ở một bên là một chế độ không ai đọc", moi, goConst)
	}
}

// TestMigration0008DoiRangBuocCoCanh — the DROP of the old CHECK happens only inside the guarded
// block, pinned to the PARENT table, and is immediately followed by the ADD in the same block.
func TestMigration0008DoiRangBuocCoCanh(t *testing.T) {
	sql := maChay(t, tep0008)

	for _, c := range []struct{ can, vi string }{
		{"where conrelid = 'khoan_muc_ngan_sach'::regclass and conname = 'khoan_muc_ngan_sach_cach_tinh_hop_le'",
			"tra cứu ràng buộc phải ghim vào bảng CHA — tên trùng ở 32 mảnh con"},
		{"if dinh_nghia is null or position('entries' in dinh_nghia) = 0 then if dinh_nghia is not null then " +
			"alter table khoan_muc_ngan_sach drop constraint khoan_muc_ngan_sach_cach_tinh_hop_le; end if; " +
			"alter table khoan_muc_ngan_sach add constraint khoan_muc_ngan_sach_cach_tinh_hop_le",
			"DROP phải có canh, và ADD phải theo ngay trong cùng khối — không cửa sổ nào thiếu CHECK"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0008 thiếu %q — %s", c.can, c.vi)
		}
	}
	if n := strings.Count(sql, "drop constraint"); n != 1 {
		t.Errorf("0008 có %d câu DROP CONSTRAINT, mong đúng 1 (ràng buộc cach_tinh)", n)
	}
}

// TestMigration0008BangDotVaGiaTri — keys composite with tenant_id, partitioned like the siblings,
// the caps, the all-or-none soft delete, and the three triggers.
//
// THE MUTATIONS THAT MUST TURN THIS RED: drop tenant_id from a primary key; remove a partition loop;
// drop the `IS NOT NULL` guards from the soft-delete CHECK (a CHECK treats NULL as passed); remove
// any trigger; turn the whole-row comparison back into a column list.
func TestMigration0008BangDotVaGiaTri(t *testing.T) {
	sql := maChay(t, tep0008)

	for _, c := range []struct{ can, vi string }{
		{"create table if not exists dot_thu_chi (", "bảng đợt"},
		{"create table if not exists gia_tri_dot (", "bảng giá trị đợt"},
		{"primary key (tenant_id, id)", "khoá chính đợt hợp thành với tenant_id (luật 1 bất biến 6)"},
		{"primary key (tenant_id, dot_id, cot_id)", "khoá chính giá trị hợp thành với tenant_id"},
		{"partition of dot_thu_chi ' 'for values with (modulus 32, remainder %s)", "32 mảnh cho dot_thu_chi"},
		{"partition of gia_tri_dot ' 'for values with (modulus 32, remainder %s)", "32 mảnh cho gia_tri_dot"},
		{"ngay date not null", "ngày bắt buộc"},
		{"noi_dung text not null", "nội dung bắt buộc"},
		{"don_vi_ca_nhan text,", "đơn vị, cá nhân NULLABLE"},
		{"so_chung_tu text,", "số chứng từ NULLABLE"},
		{"nguoi_ghi_ma text not null", "người ghi là MÃ cán bộ, bắt buộc (luật 6 bất biến 8)"},
		{"gia_tri bigint,", "tiền là BIGINT đồng, NULL = trống"},
		{"check (char_length(noi_dung) <= 1000)", "trần nội dung theo KÝ TỰ"},
		{"check (char_length(don_vi_ca_nhan) <= 300)", "trần đơn vị, cá nhân theo KÝ TỰ"},
		{"check (char_length(so_chung_tu) <= 100)", "trần số chứng từ theo KÝ TỰ"},
		{"check (char_length(delete_reason) <= 500)", "trần lý do xoá"},
		{"(deleted_at is null and deleted_by is null and delete_reason is null) or (deleted_at is not null " +
			"and deleted_by is not null and btrim(deleted_by) <> '' and delete_reason is not null and btrim(delete_reason) <> '')",
			"xoá mềm đủ ba hoặc không gì"},
		{"before delete on dot_thu_chi for each row execute function ho_so_luu_tru_cam_xoa_cung()",
			"xoá cứng đợt phải bị chặn bằng hàm lưu trữ dùng chung"},
		{"before update on dot_thu_chi for each row execute function dot_thu_chi_bat_bien()",
			"đợt đã ghi không được sửa"},
		{"(to_jsonb(new) - 'deleted_at' - 'deleted_by' - 'delete_reason') is distinct from " +
			"(to_jsonb(old) - 'deleted_at' - 'deleted_by' - 'delete_reason')",
			"so cả dòng trừ bộ ba — cột thêm sau tự động bị khoá"},
		{"if old.deleted_at is not null and (new.deleted_at is distinct from old.deleted_at",
			"đã xoá thì không khôi phục, không viết lại lý do"},
		{"before update or delete on gia_tri_dot for each row execute function gia_tri_dot_bat_bien()",
			"giá trị đợt không sửa, không xoá"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0008 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0008KhongPhaHuy — this file only ADDS (the constraint swap aside, checked above). It
// must not drop or retype a column, drop a table, replace 0004's archival guard, or put money in a
// floating type or in jsonb.
func TestMigration0008KhongPhaHuy(t *testing.T) {
	sql := maChay(t, tep0008)
	for _, c := range []struct{ cam, vi string }{
		{"drop column", "xoá cột trên hồ sơ lưu trữ"},
		{"drop table", "xoá bảng"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"create or replace function ho_so_luu_tru_cam_xoa_cung","thay hàm canh lưu trữ của 0004 — bảng khác đang dùng"},
		{"jsonb,", "tiền trong jsonb — xem đầu tệp: không kiểu, không khoá"},
		{"numeric", "tiền không phải BIGINT đồng"},
		{"double precision", "tiền dấu phẩy động"},
		{"real,", "tiền dấu phẩy động"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0008 chứa %q — %s", c.cam, c.vi)
		}
	}
	if regexp.MustCompile(`unique[^;]*where`).MatchString(sql) {
		t.Error("0008 có khoá duy nhất từng phần — mã đã cấp sẽ cấp lại được (luật 7 bất biến 3)")
	}
}
