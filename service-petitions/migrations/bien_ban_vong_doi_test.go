package migrations

import (
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Schema-TEXT checks for migration 0012 (meeting-minutes lifecycle, user decisions 25/09/2026).
//
// WHAT THIS IS AND IS NOT — same standing as ket_thuc_nhanh_phan_anh_test.go. It reads the SQL this
// binary embeds and asserts the constraints, triggers and index are WRITTEN. It does NOT prove
// PostgreSQL accepts the file, evaluates the CASE, fires the triggers on every partition, or that
// FOR SHARE closes the signing race; that needs VIGOV_TEST_DSN. It exists so that a widened or
// deleted guard goes red somewhere that always runs.

const tep0012 = "0012_bien_ban_vong_doi.sql"

// TestMigration0012TrangThaiKhopDomain — the status CHECK and the Go constants are the SAME SET.
// A third code added on one side only is a state the other side cannot reach or cannot leave.
func TestMigration0012TrangThaiKhopDomain(t *testing.T) {
	sql := maChay(t, tep0012)
	trongSQL := danhSachMaTrongCheck(t, sql, "bien_ban_hop_trang_thai_hop_le", "trang_thai")
	trongGo := []string{domain.TrangThaiBienBanDuThao, domain.TrangThaiBienBanDaKy}
	sort.Strings(trongGo)
	if strings.Join(trongSQL, ",") != strings.Join(trongGo, ",") {
		t.Fatalf("0012 khai %v, domain khai %v — hai danh sách trạng thái biên bản phải trùng", trongSQL, trongGo)
	}
	if !strings.Contains(sql, "add column if not exists trang_thai text not null default '"+
		domain.TrangThaiBienBanDuThao+"';") {
		t.Error("trang_thai phải NOT NULL DEFAULT 'du-thao' — dòng có sẵn thành dự thảo, không bịa người ký")
	}
}

// TestMigration0012RangBuoc — the column bindings.
//
// THE MUTATIONS THAT MUST TURN THIS RED: delete the ELSE arm of the signing CHECK (a signer could sit
// on a draft); drop `is not null` before a `btrim` (a CHECK treats NULL as passed); drop `tenant_id`
// from the supplementary foreign key (a supplement could point into another commune); lose the
// both-or-none shape of the notice or of the no-task mark.
func TestMigration0012RangBuoc(t *testing.T) {
	sql := maChay(t, tep0012)
	for _, c := range []struct{ can, vi string }{
		{"when 'da-ky' then ky_luc is not null and ky_boi_ma is not null and btrim(ky_boi_ma) <> '' " +
			"else ky_luc is null and ky_boi_ma is null end", "đã ký ⇔ đủ người ký + thời điểm; dự thảo ⇒ cả hai NULL"},
		{"check (char_length(ky_boi_ma) <= 32)", "mã người ký phải có trần theo KÝ TỰ"},
		{"check (thu_ky_ma is null or (btrim(thu_ky_ma) <> '' and char_length(thu_ky_ma) <= 32))",
			"thư ký: NULL hoặc mã không rỗng, trần như chu_tri_ma"},
		{"(tb_so_ky_hieu is null and tb_ngay is null) or (tb_so_ky_hieu is not null and btrim(tb_so_ky_hieu) <> '' and tb_ngay is not null)",
			"số và ngày Thông báo kết luận: có cả hai hoặc không có gì"},
		{"check (char_length(tb_so_ky_hieu) <= 64)", "số TB phải có trần như so_hieu"},
		{"check (bo_sung_cho_id is null or (btrim(bo_sung_cho_id) <> '' and bo_sung_cho_id <> id))",
			"biên bản bổ sung không trỏ về chính nó"},
		{"foreign key (tenant_id, bo_sung_cho_id) references bien_ban_hop (tenant_id, id)",
			"khoá ngoại bổ sung phải hợp thành với tenant_id"},
		{"add column if not exists khong_phat_sinh boolean not null default false;", "dấu không phát sinh: NOT NULL DEFAULT false"},
		{"(khong_phat_sinh and khong_phat_sinh_luc is not null and khong_phat_sinh_boi_ma is not null and btrim(khong_phat_sinh_boi_ma) <> '') " +
			"or (not khong_phat_sinh and khong_phat_sinh_luc is null and khong_phat_sinh_boi_ma is null)",
			"dấu không phát sinh đi cùng ai + lúc nào; bỏ dấu thì xoá cả hai"},
		{"check (char_length(khong_phat_sinh_boi_ma) <= 32)", "mã người đặt dấu phải có trần"},
		{"create index if not exists bien_ban_hop_theo_ngay_hop on bien_ban_hop (tenant_id, ngay_hop desc, tao_luc desc, id) where deleted_at is null",
			"chỉ mục sắp theo ngày họp, bắt đầu bằng tenant_id, loại dòng xoá mềm"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0012 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0012TriggerBatBien — the two immutability triggers.
//
// THE MUTATIONS THAT MUST TURN THIS RED: add 'deleted_at' to the allowed columns (signed minutes
// could then be soft deleted); switch to a column allow-list; drop the set-once guard on the notice;
// drop FOR SHARE from the parent read (the signing race reopens); drop INSERT from the conclusion
// trigger (signed minutes could gain a conclusion).
func TestMigration0012TriggerBatBien(t *testing.T) {
	sql := maChay(t, tep0012)
	for _, c := range []struct{ can, vi string }{
		{"if old.trang_thai = 'da-ky' then if (to_jsonb(new) - 'tb_so_ky_hieu' - 'tb_ngay' - 'cap_nhat_luc') is distinct from (to_jsonb(old) - 'tb_so_ky_hieu' - 'tb_ngay' - 'cap_nhat_luc')",
			"so NGUYÊN DÒNG trừ đúng ba cột được phép — cột thêm sau tự bị khoá"},
		{"if (old.tb_so_ky_hieu is not null or old.tb_ngay is not null) and (new.tb_so_ky_hieu is distinct from old.tb_so_ky_hieu or new.tb_ngay is distinct from old.tb_ngay)",
			"số/ngày TB chỉ ghi MỘT lần sau ký"},
		{"before update on bien_ban_hop for each row execute function bien_ban_hop_da_ky_bat_bien()",
			"trigger khoá biên bản đã ký trên bảng CHA phân mảnh"},
		{"where tenant_id = old.tenant_id and id = old.bien_ban_id for share",
			"đọc biên bản cha cùng xã, FOR SHARE để chặn đua với thao tác ký"},
		// The INSERT path and the move path carry the same probe text, so each is anchored to its
		// own branch — a bare substring let a FOR SHARE removed from ONE of them pass (measured).
		{"if tg_op = 'insert' then select trang_thai into tt_moi from bien_ban_hop where tenant_id = new.tenant_id and id = new.bien_ban_id for share;",
			"INSERT kết luận: đọc cha cùng xã, FOR SHARE"},
		{"new.bien_ban_id is distinct from old.bien_ban_id then select trang_thai into tt_moi from bien_ban_hop where tenant_id = new.tenant_id and id = new.bien_ban_id for share;",
			"chuyển kết luận sang biên bản khác: đọc cha đích cùng xã, FOR SHARE"},
		{"(to_jsonb(new) - 'cap_nhat_luc') is distinct from (to_jsonb(old) - 'cap_nhat_luc')",
			"kết luận của biên bản đã ký: khoá nguyên dòng, kể cả xoá mềm"},
		{"before insert or update on ket_luan_hop for each row execute function ket_luan_hop_da_ky_bat_bien()",
			"trigger kết luận phải chặn cả INSERT"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0012 thiếu %q — %s", c.can, c.vi)
		}
	}
	for _, cam := range []string{"- 'deleted_at'", "- 'deleted_by'", "- 'delete_reason'"} {
		if strings.Contains(sql, cam) {
			t.Errorf("0012 loại %q khỏi phép so — biên bản đã ký sẽ xoá mềm được", cam)
		}
	}
}

// TestMigration0012KhongPhaHuy — this file only ADDS: no drop, no retype, no new NOT NULL column
// without a default, no sequence and no unique key (the notice number is transcribed, never minted).
func TestMigration0012KhongPhaHuy(t *testing.T) {
	sql := maChay(t, tep0012)
	for _, c := range []struct{ cam, vi string }{
		{"drop column", "xoá cột trên hồ sơ lưu trữ"},
		{"drop constraint", "gỡ một ràng buộc"},
		{"drop index", "gỡ chỉ mục đang có người đọc"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"timestamptz not null", "cột thời điểm mới NOT NULL — bịa giá trị cho dòng có sẵn"},
		{"date not null", "cột ngày mới NOT NULL — bịa giá trị cho dòng có sẵn"},
		{"sequence", "tự cấp số — số TB do văn thư cấp, chép tay"},
		{"unique", "khoá duy nhất trên số chép tay — số bắt đầu lại mỗi năm"},
		{"concurrently", "CONCURRENTLY bị từ chối trên bảng cha phân mảnh và trong giao dịch"},
		{"function ho_so_luu_tru_cam_xoa_cung", "thay hàm canh xoá cứng của 0007"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0012 chứa %q — %s", c.cam, c.vi)
		}
	}
}
