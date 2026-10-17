package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR: the template table is a fixed list in source that GRANTS AUTHORITY in every
// commune that presses the button. A typo is a right nobody can hold (rule 5, invariant 3c — the
// foreign key would refuse it at a commune's first run); an extra key is a right granted by nobody in
// particular. Both are caught here, against the keys the migrations of this service REALLY seed —
// read off the SQL files, never off a second list in Go.

// seededKeys reads the permission keys of every `INSERT INTO quyen` in the given migration files.
func seededKeys(t *testing.T, files ...string) map[string]bool {
	t.Helper()
	row := regexp.MustCompile(`\(\s*'([a-z][a-z._]*)'\s*,`)
	out := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("đọc migration %s: %v", f, err)
		}
		src := string(b)
		for {
			i := strings.Index(src, "INSERT INTO quyen")
			if i < 0 {
				break
			}
			src = src[i:]
			end := strings.Index(src, ";")
			if end < 0 {
				t.Fatalf("%s: INSERT INTO quyen không có dấu ; kết thúc", f)
			}
			for _, m := range row.FindAllStringSubmatch(src[:end], -1) {
				out[m[1]] = true
			}
			src = src[end:]
		}
	}
	// FAIL CLOSED: a parser that finds nothing would make every assertion below vacuously true.
	if len(out) == 0 {
		t.Fatalf("không đọc được khoá quyền nào từ %v — phép kiểm mất nguồn chuẩn", files)
	}
	return out
}

func allMigrations(t *testing.T) []string {
	t.Helper()
	fs, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(fs) == 0 {
		t.Fatalf("không tìm thấy migration: %v", err)
	}
	return fs
}

func TestRoleTemplatesAreTheEightOfSpec41WithTheirCounts(t *testing.T) {
	want := []struct {
		order    int
		code     string
		name     string
		isLeader bool
		count    int
	}{
		// Migration 0036 (owner 17/10/2026, ADR 0101 #5): admin.citizen_field to both admin.zalo holders: 44→45, 23→24.
		{1, "chu-tich-ubnd", "Chủ tịch UBND", true, 45},
		{2, "pho-chu-tich-ubnd", "Phó Chủ tịch UBND", true, 29},
		// Migration 0034 (owner 16/10/2026) moved five of these: 20→22, 12→13, 8→9, 6→8, 4→6.
		{3, "chanh-van-phong", "Chánh Văn phòng", false, 24},
		{4, "truong-bo-phan", "Trưởng bộ phận", false, 13},
		{5, "chuyen-vien", "Chuyên viên chuyên môn", false, 9},
		{6, "ke-toan", "Kế toán", false, 6},
		{7, "can-bo-mot-cua", "Cán bộ một cửa", false, 8},
		{8, "truong-thon", "Trưởng thôn, Tổ trưởng dân phố", false, 6},
	}
	got := RoleTemplates()
	if len(got) != len(want) {
		t.Fatalf("có %d vai trò mẫu, muốn %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Order != w.order || g.Code != w.code || g.Name != w.name || g.IsLeader != w.isLeader {
			t.Errorf("vai trò #%d = {%d %q %q %v}, muốn {%d %q %q %v}",
				i, g.Order, g.Code, g.Name, g.IsLeader, w.order, w.code, w.name, w.isLeader)
		}
		if len(g.Permissions) != w.count {
			t.Errorf("%s: %d quyền, muốn %d", g.Code, len(g.Permissions), w.count)
		}
	}
}

func TestRoleTemplatesHaveNoDuplicateCodeOrKey(t *testing.T) {
	codes := map[string]bool{}
	for _, r := range RoleTemplates() {
		if codes[r.Code] {
			t.Errorf("mã vai trò %q lặp lại — UNIQUE (tenant_id, ma) sẽ hỏng cả lượt gieo", r.Code)
		}
		codes[r.Code] = true
		keys := map[string]bool{}
		for _, k := range r.Permissions {
			if keys[k] {
				t.Errorf("%s: quyền %q lặp lại — khoá chính vai_tro_quyen sẽ hỏng cả lượt gieo", r.Code, k)
			}
			keys[k] = true
		}
	}
}

// EVERY KEY EXISTS IN `quyen` (rule 5, invariant 3c). Read off every migration of this service.
func TestRoleTemplateKeysAllExistInQuyen(t *testing.T) {
	seeded := seededKeys(t, allMigrations(t)...)
	for _, r := range RoleTemplates() {
		for _, k := range r.Permissions {
			if !seeded[k] {
				t.Errorf("%s: quyền %q không có trong bảng quyen — không xã nào cấp được", r.Code, k)
			}
		}
	}
}

// CHỦ TỊCH HOLDS EXACTLY THE 33 KEYS OF MIGRATION 0001 PLUS THE SIX OF 0028 AND THE FOUR OF 0032 —
// not the catalogue. The six were added by user decision 2026-10-09, the four by owner decision
// 15/10/2026 (role_template.go file comment); another key
// arriving here without a person deciding it must turn this red.
func TestChairmanHoldsExactlyTheKeysOf0001(t *testing.T) {
	of0001 := seededKeys(t, filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if len(of0001) != 33 {
		t.Fatalf("migration 0001 gieo %d khoá, phép kiểm này viết cho 33 — đọc lại quyết định trước khi sửa", len(of0001))
	}
	of0028 := seededKeys(t, filepath.Join("..", "..", "migrations", "0028_citizen_dossier_notice_permissions.sql"))
	if len(of0028) != 6 {
		t.Fatalf("migration 0028 gieo %d khoá, phép kiểm này viết cho 6 — đọc lại quyết định trước khi sửa", len(of0028))
	}
	for k := range of0028 {
		of0001[k] = true
	}
	// Migration 0032 (owner decision 15/10/2026): four NEW configuration keys. Its INSERTs also
	// re-rank admin.org/role/sla/user, which 0001 already seeds — so count only what 0001 lacks.
	of0032 := seededKeys(t, filepath.Join("..", "..", "migrations", "0032_split_configuration_permissions.sql"))
	added0032 := 0
	for k := range of0032 {
		if !of0001[k] {
			added0032++
			of0001[k] = true
		}
	}
	if added0032 != 4 {
		t.Fatalf("migration 0032 thêm %d khoá mới, phép kiểm này viết cho 4 — đọc lại quyết định trước khi sửa", added0032)
	}
	// Migration 0035 (ADR 0099 #3, owner 17/10/2026): admin.sms, granted wherever admin.zalo is.
	of0035 := seededKeys(t, filepath.Join("..", "..", "migrations", "0035_sms_channel_permission.sql"))
	if len(of0035) != 1 || !of0035["admin.sms"] {
		t.Fatalf("migration 0035 gieo %v, phép kiểm này viết cho đúng admin.sms — đọc lại quyết định trước khi sửa", of0035)
	}
	of0001["admin.sms"] = true
	// Migration 0036 (ADR 0101 #5, owner 17/10/2026): admin.citizen_field, granted wherever admin.zalo is.
	of0036 := seededKeys(t, filepath.Join("..", "..", "migrations", "0036_citizen_field.sql"))
	if len(of0036) != 1 || !of0036["admin.citizen_field"] {
		t.Fatalf("migration 0036 gieo %v, phép kiểm này viết cho đúng admin.citizen_field — đọc lại quyết định trước khi sửa", of0036)
	}
	of0001["admin.citizen_field"] = true
	chair := RoleTemplates()[0]
	got := map[string]bool{}
	for _, k := range chair.Permissions {
		got[k] = true
	}
	var missing, extra []string
	for k := range of0001 {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !of0001[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("Chủ tịch UBND: thiếu %v, thừa %v so với 33 khoá của 0001 + 6 khoá của 0028 + 4 khoá của 0032 + admin.sms của 0035 + admin.citizen_field của 0036", missing, extra)
	}
}

// MIGRATION 0034'S EIGHT (role, key) PAIRS ARE IN THE TEMPLATES — read off the SQL, so a commune
// seeded after 0034 and one seeded before end in the same state (0034 header). And NO other
// template holds either key beyond those pairs and the two leader roles of 0028 (ADR 0097 D12).
func TestRoleTemplatesCarryMigration0034Grants(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0034_citizen_directory_role_grants.sql"))
	if err != nil {
		t.Fatalf("đọc migration 0034: %v", err)
	}
	pair := regexp.MustCompile(`\('([a-z-]+)',\s*'(citizen\.[a-z]+)'\)`)
	want := map[string]map[string]bool{}
	for _, m := range pair.FindAllStringSubmatch(string(b), -1) {
		if want[m[1]] == nil {
			want[m[1]] = map[string]bool{}
		}
		want[m[1]][m[2]] = true
	}
	if len(want) != 5 {
		t.Fatalf("0034 names %d roles, phép kiểm này viết cho 5 — đọc lại quyết định trước khi sửa", len(want))
	}
	leaders := map[string]bool{"chu-tich-ubnd": true, "pho-chu-tich-ubnd": true}
	for _, r := range RoleTemplates() {
		held := map[string]bool{}
		for _, k := range r.Permissions {
			if strings.HasPrefix(k, "citizen.") {
				held[k] = true
			}
		}
		for k := range want[r.Code] {
			if !held[k] {
				t.Errorf("%s: 0034 cấp %q mà bộ mẫu không có — xã gieo sau 0034 sẽ thiếu khoá", r.Code, k)
			}
		}
		for k := range held {
			if !want[r.Code][k] && !leaders[r.Code] {
				t.Errorf("%s: bộ mẫu cấp %q mà 0034 không cấp — ngoài D12", r.Code, k)
			}
		}
	}
}

// THE THREE LATER KEYS ARE GRANTED BY NO TEMPLATE, and `quan-tri-he-thong` is not a template.
func TestRoleTemplatesNeverGrantTheNarrowKeysNorTheAdminRole(t *testing.T) {
	forbidden := []string{"feedback.classify", "feedback.unmask", "admin.user.delete"}
	for _, r := range RoleTemplates() {
		if r.Code == "quan-tri-he-thong" {
			t.Errorf("vai trò quản trị hệ thống nằm trong bộ mẫu — lượt gieo không được đụng tới nó")
		}
		for _, k := range r.Permissions {
			for _, f := range forbidden {
				if k == f {
					t.Errorf("%s cấp %q — khoá này không thuộc bộ mẫu (quyết định 2026-09-28)", r.Code, k)
				}
			}
			if strings.ContainsAny(k, "*%") {
				t.Errorf("%s: %q là mẫu đại diện — mọi khoá phải viết tường minh", r.Code, k)
			}
		}
	}
}

// THE UNION IS WHAT THE CALLER MUST HOLD — and, because Chủ tịch holds all 45 (0001's 33 + 0028's
// six + 0032's four + 0035's admin.sms + 0036's admin.citizen_field), it is exactly those 45.
func TestRoleTemplatePermissionsIsTheUnion(t *testing.T) {
	u := RoleTemplatePermissions()
	if len(u) != 45 {
		t.Fatalf("hợp các quyền của bộ mẫu có %d khoá, muốn 45", len(u))
	}
	in := map[string]bool{}
	for _, k := range u {
		if in[k] {
			t.Errorf("khoá %q lặp trong hợp", k)
		}
		in[k] = true
	}
	for _, r := range RoleTemplates() {
		for _, k := range r.Permissions {
			if !in[k] {
				t.Errorf("%s cấp %q mà hợp không có — người gọi sẽ không bị kiểm khoá này", r.Code, k)
			}
		}
	}
}

// A caller editing the returned slice must not change what the next call returns.
func TestRoleTemplatesReturnFreshSlices(t *testing.T) {
	a := RoleTemplates()
	a[0].Permissions[0] = "bi-sua"
	a[1].Code = "bi-sua"
	b := RoleTemplates()
	if b[0].Permissions[0] == "bi-sua" || b[1].Code == "bi-sua" {
		t.Fatal("RoleTemplates trả về dữ liệu dùng chung — một lượt gieo sửa được bộ mẫu của xã sau")
	}
}
