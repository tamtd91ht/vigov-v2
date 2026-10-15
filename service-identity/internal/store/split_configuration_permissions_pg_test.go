package store

import (
	"context"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/migrations"
)

// The pre-grant of migration 0032 (owner decision 15/10/2026), against a real PostgreSQL. Skipped
// unless VIGOV_TEST_DSN is set. The statement is read FROM THE EMBEDDED FILE and run again on rows
// written after the migration ran, so this exercises the SQL that ships (the 0031 test's shape).
//
// MUTATIONS THAT MUST TURN THIS RED: drop `vt.deleted_at IS NULL` (the deleted role gains keys);
// give admin.org the four keys (role B gains three too many); drop ON CONFLICT DO NOTHING (the rerun
// fails on the primary key); write the entry outside the RETURNING (the rerun writes a second entry).
func TestPgMigration0032PreGrantsByHeldKey(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themVaiTro(t, db, xa, "vt-lookup", "vt-lookup", "Giữ danh mục", false)
	capQuyenCho(t, db, xa, "vt-lookup", "admin.lookup")
	themVaiTro(t, db, xa, "vt-org", "vt-org", "Giữ sơ đồ tổ chức", false)
	capQuyenCho(t, db, xa, "vt-org", "admin.org")
	themVaiTro(t, db, xa, "vt-none", "vt-none", "Không liên quan", false)
	capQuyenCho(t, db, xa, "vt-none", "task.read")
	// Already holds one of the four: gains only the other three, and its entry says so.
	themVaiTro(t, db, xa, "vt-partial", "vt-partial", "Đã có một khoá", false)
	capQuyenCho(t, db, xa, "vt-partial", "admin.lookup")
	capQuyenCho(t, db, xa, "vt-partial", "admin.mail_server")
	// Soft-deleted: the commune's decision, untouched.
	themVaiTro(t, db, xa, "vt-deleted", "vt-deleted", "Đã xoá", false)
	capQuyenCho(t, db, xa, "vt-deleted", "admin.lookup")
	if _, err := db.Exec(`UPDATE vai_tro SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'thử'
	                      WHERE tenant_id = $1 AND id = 'vt-deleted'`, xa); err != nil {
		t.Fatal(err)
	}

	raw, err := fs.ReadFile(migrations.FS, "0032_split_configuration_permissions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	start := strings.Index(sql, "WITH grant_rule (held, granted) AS (")
	end := strings.Index(sql, "FROM per_role;")
	if start < 0 || end < start {
		t.Fatal("không tìm thấy câu cấp sẵn trong 0032")
	}
	stmt := sql[start : end+len("FROM per_role;")]
	// Twice: the second run must insert nothing and audit nothing (idempotent).
	for i := 0; i < 2; i++ {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("chạy câu cấp sẵn lần %d: %v", i+1, err)
		}
	}

	held := func(role string) []string {
		t.Helper()
		rows, err := db.Query(`SELECT quyen_ma FROM vai_tro_quyen WHERE tenant_id = $1 AND vai_tro_id = $2`, xa, role)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				t.Fatal(err)
			}
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	for role, want := range map[string][]string{
		"vt-lookup":  {"admin.lookup", "admin.mail_server", "admin.residential_unit", "admin.system_message", "admin.zalo_channel"},
		"vt-org":     {"admin.org", "admin.residential_unit"},
		"vt-none":    {"task.read"},
		"vt-partial": {"admin.lookup", "admin.mail_server", "admin.residential_unit", "admin.system_message", "admin.zalo_channel"},
		"vt-deleted": {"admin.lookup"},
	} {
		if got := held(role); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s giữ %v, muốn %v", role, got, want)
		}
	}

	var by string
	if err := db.QueryRow(`SELECT cap_boi FROM vai_tro_quyen
	                        WHERE tenant_id = $1 AND vai_tro_id = 'vt-org' AND quyen_ma = 'admin.residential_unit'`, xa).
		Scan(&by); err != nil {
		t.Fatal(err)
	}
	if by != "system" {
		t.Errorf("cap_boi = %q, muốn system", by)
	}

	// One entry per role that gained something — three roles, despite two runs.
	entries := map[string]string{}
	rows, err := db.Query(`SELECT subject, delta->>'them' FROM audit_log
	                        WHERE tenant_id = $1 AND action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
	                          AND delta->>'ly_do' LIKE 'migration 0032:%'`, xa)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var subject, them string
		if err := rows.Scan(&subject, &them); err != nil {
			t.Fatal(err)
		}
		if _, dup := entries[subject]; dup {
			t.Errorf("vai trò %s có hai vết — lần chạy lại đã ghi thêm", subject)
		}
		entries[subject] = them
	}
	if len(entries) != 3 {
		t.Fatalf("có %d vết cấp sẵn, muốn 3 (vt-lookup, vt-org, vt-partial): %v", len(entries), entries)
	}
	if got := entries["vt-partial"]; got != `["admin.residential_unit", "admin.system_message", "admin.zalo_channel"]` {
		t.Errorf("vết vt-partial ghi them = %s, muốn đúng ba khoá chưa có", got)
	}
}
