package migrations

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0032 (`phieu_phan_anh.zalo_account_id`, ADR 0080). Same standing as
// task_extension_decision_note_test.go: it reads the SQL this binary embeds and asserts the column, the
// CHECKs, the freeze and the photo-floor change are WRITTEN. It does NOT prove PostgreSQL enforces
// them — that needs VIGOV_TEST_DSN (tools/schema-smoke, and a pg suite in internal/store).

const file0032 = "0032_petition_zalo_account_owner.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the column made NOT NULL or given a default; a blank string
// admitted as an owner; both owner kinds allowed on one row; a Zalo owner on a channel other than the
// Mini App; rating or reopen allowed on a Zalo-owned row; the index not led by tenant_id; a second
// "unverified" flag column added beside the owner.
func TestMigration0032ColumnConstraintsIndex(t *testing.T) {
	sql := maChay(t, file0032)
	for _, c := range []struct{ want, why string }{
		{"alter table phieu_phan_anh add column if not exists zalo_account_id text;",
			"cột chủ phiếu tài khoản Zalo phải có, NULLABLE, không mặc định"},
		{"check (zalo_account_id is null or btrim(zalo_account_id) <> '')",
			"chủ phiếu rỗng không phải là chủ"},
		{"check (not (cong_dan_id is not null and zalo_account_id is not null))",
			"một phiếu nhiều nhất một loại chủ"},
		{"check (zalo_account_id is null or kenh_tiep_nhan = 'zalo-mini-app')",
			"phiếu chủ Zalo chỉ đến từ Mini App (ADR 0080 điều kiện dừng #3)"},
		{"check (zalo_account_id is null or (diem_hai_long is null and danh_gia_luc is null and rating_comment is null and so_lan_mo_lai = 0))",
			"phiếu chưa xác thực không đánh giá, không mở lại (ADR 0080 #8)"},
		{"on phieu_phan_anh (tenant_id, zalo_account_id, vao_so_luc desc) where zalo_account_id is not null",
			"chỉ mục đếm ngưỡng 10/ngày, tenant_id đứng đầu (luật 1)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0032 thiếu %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"drop column", "tệp này không xoá cột — lệnh đảo ngược chỉ nằm trong chú thích REVERSAL"},
		{"drop constraint", "không gỡ ràng buộc có sẵn"},
		{"drop index", "không gỡ chỉ mục có sẵn"},
		{"alter column", "đổi kiểu hoặc NOT NULL của cột có sẵn"},
		{"delete from", "không xoá dòng nào"},
		{"truncate", "không làm rỗng bảng nào"},
		{"update phieu_phan_anh", "không backfill"},
		{"update stored_file", "không ghi dữ liệu"},
		{"insert into", "không ghi dữ liệu"},
		{"create table", "không có bảng mới"},
		{"create or replace function ho_so_luu_tru_bat_bien", "hàm canh lưu trữ của 0004 không phải của tệp này"},
		{"drop trigger if exists stored_file_petition_check", "trigger sàn ảnh của 0026 giữ nguyên, chỉ thay hàm"},
		{"unverified boolean", "không thêm cờ thứ hai — chủ phiếu là sự thật duy nhất (ADR 0080 #5)"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0032 chứa %q — %s", c.banned, c.why)
		}
	}
}

// The owner is frozen after intake by its own trigger. Mutation: the trigger removed, or the comparison
// weakened so that clearing (or setting) the owner passes.
func TestMigration0032OwnerFrozen(t *testing.T) {
	sql := maChay(t, file0032)
	for _, want := range []string{
		"create or replace function phieu_phan_anh_zalo_owner_frozen()",
		"if new.zalo_account_id is distinct from old.zalo_account_id then raise exception",
		"before update on phieu_phan_anh for each row execute function phieu_phan_anh_zalo_owner_frozen()",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0032 thiếu %q — chủ phiếu phải đóng băng sau khi tiếp nhận", want)
		}
	}
}

// photoFn returns the normalised body of stored_file_petition_check as defined in one file.
func photoFn(t *testing.T, file string) string {
	t.Helper()
	sql := maChay(t, file)
	const head = "create or replace function stored_file_petition_check()"
	i := strings.Index(sql, head)
	if i < 0 {
		t.Fatalf("%s không định nghĩa stored_file_petition_check", file)
	}
	j := strings.Index(sql[i:], "end $$;")
	if j < 0 {
		t.Fatalf("%s: không tìm thấy cuối hàm stored_file_petition_check", file)
	}
	return sql[i : i+j]
}

// The replaced photo floor is 0026's body with ONE condition widened. Everything before and after the
// owner check — subject filter, stored-set entry, commune and soft-delete filter, lock, 5-photo count —
// must be 0026's text after the two mechanical substitutions. Mutation: any other edit in the body.
func TestMigration0032PhotoFloorIs0026PlusOwner(t *testing.T) {
	oldFn, newFn := photoFn(t, tep0026), photoFn(t, file0032)
	const cut = "if new.uploaded_by = 'cong-dan' and ("
	const tail = "if new.purpose = 'petition-photo'"
	split := func(s, name string) (string, string) {
		a, b := strings.Index(s, cut), strings.Index(s, tail)
		if a < 0 || b < 0 || b < a {
			t.Fatalf("%s: không tách được thân hàm quanh kiểm chủ phiếu", name)
		}
		return s[:a], s[b:]
	}
	oldPre, oldPost := split(oldFn, tep0026)
	newPre, newPost := split(newFn, file0032)

	wantPre := strings.NewReplacer(
		"petition_citizen text; live_photos int;",
		"petition_citizen text; petition_zalo text; live_photos int;",
		"select p.cong_dan_id into petition_citizen",
		"select p.cong_dan_id, p.zalo_account_id into petition_citizen, petition_zalo",
	).Replace(oldPre)
	if newPre != wantPre {
		t.Errorf("phần đầu hàm sàn ảnh lệch 0026 ngoài hai thay thế đã nêu:\n got %s\nwant %s", newPre, wantPre)
	}
	if newPost != oldPost {
		t.Errorf("phần đếm 5 ảnh lệch 0026:\n got %s\nwant %s", newPost, oldPost)
	}

	// Fail closed: refused only when BOTH owners are empty. Mutation: `or` between the two, or one
	// owner dropped from the test.
	const owner = "if new.uploaded_by = 'cong-dan' and (petition_citizen is null or btrim(petition_citizen) = '') " +
		"and (petition_zalo is null or btrim(petition_zalo) = '') then raise exception"
	if !strings.Contains(newFn, owner) {
		t.Errorf("0032 thiếu kiểm %q — ảnh công dân cần phiếu có chủ là phiên công dân, không chủ thì từ chối", owner)
	}
}

// Whichever migration defines stored_file_petition_check LAST is the one PostgreSQL runs. A later file
// that rewrites the floor and forgets the Zalo owner (or the citizen owner, or the limit) turns this red.
func TestLatestPhotoFloorAdmitsBothOwners(t *testing.T) {
	names, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatalf("liệt kê migration: %v", err)
	}
	sort.Strings(names)
	latest := ""
	for _, n := range names {
		if strings.Contains(maChay(t, n), "create or replace function stored_file_petition_check()") {
			latest = n
		}
	}
	if latest < file0032 {
		t.Fatalf("định nghĩa cuối của stored_file_petition_check là %q, phải từ 0032 trở đi", latest)
	}
	fn := photoFn(t, latest)
	for _, want := range []string{
		"select p.cong_dan_id, p.zalo_account_id into petition_citizen, petition_zalo",
		"and (petition_zalo is null or btrim(petition_zalo) = '') then raise exception",
		"if live_photos >= 5 then raise exception",
		"where p.tenant_id = new.tenant_id and p.id = new.subject_id and p.deleted_at is null for update",
	} {
		if !strings.Contains(fn, want) {
			t.Errorf("định nghĩa cuối của stored_file_petition_check (%s) thiếu %q", latest, want)
		}
	}
}
