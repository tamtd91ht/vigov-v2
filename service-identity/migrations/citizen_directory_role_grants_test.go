package migrations

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0034 (owner decision 16/10/2026: default grants of citizen.read /
// citizen.update per the prototype's matrix). Same weaker-half caveat as
// citizen_dossier_notice_permissions_test.go: it proves the statements are WRITTEN this way, not that
// PostgreSQL ran them.

const migration0034 = "0034_citizen_directory_role_grants.sql"

// grantPair matches one row of the `grant_rule` VALUES: ('<role code>', '<key>').
var grantPair = regexp.MustCompile(`\('([a-z-]+)',\s*'(citizen\.[a-z]+)'\)`)

// THE PAIRS ARE EXACTLY THE OWNER'S LIST MINUS THE ROLES 0028 ALREADY COVERS, with the prototype's
// `tiep-nhan` landing on `can-bo-mot-cua` (ADR 0055 §4 mapping, owner confirmed 16/10/2026 — 0034
// header). A pair added here without a person deciding it, or a leader role repeated (which would
// undo an administrator's withdrawal), must turn this red.
func TestMigration0034GrantsExactlyTheDecidedPairs(t *testing.T) {
	sql := maChay(t, migration0034)
	want := map[string]bool{
		"chanh-van-phong citizen.read":   true,
		"truong-bo-phan citizen.read":    true,
		"chuyen-vien citizen.read":       true,
		"can-bo-mot-cua citizen.read":    true,
		"truong-thon citizen.read":       true,
		"chanh-van-phong citizen.update": true,
		"can-bo-mot-cua citizen.update":  true,
		"truong-thon citizen.update":     true,
	}
	got := map[string]bool{}
	for _, m := range grantPair.FindAllStringSubmatch(sql, -1) {
		got[m[1]+" "+m[2]] = true
	}
	if len(got) == 0 {
		t.Fatal("no (role, key) pair found in 0034 — the parser went blind")
	}
	for k := range want {
		if !got[k] {
			t.Errorf("0034 no longer grants %s", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("0034 grants %s, which nobody decided", k)
		}
	}
	for _, leader := range []string{"chu-tich-ubnd", "pho-chu-tich-ubnd", "quan-tri-he-thong"} {
		if strings.Contains(sql, "'"+leader+"'") {
			t.Errorf("0034 names %s — 0028 granted it; a re-grant would silently undo a later withdrawal", leader)
		}
	}
}

// EVERY ROLE CODE EXISTS. A code copied from the prototype (`tiep-nhan`, which no template here
// carries) would grant nothing in any commune and nobody would notice. Read off the ONE
// place role codes are declared — domain.RoleTemplates — the way role_template_test.go reads the
// migrations: as text, since internal/domain imports this package and cannot be imported back.
func TestMigration0034RoleCodesExistInRoleTemplates(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "internal", "domain", "role_template.go"))
	if err != nil {
		t.Fatalf("đọc role_template.go: %v", err)
	}
	templates := string(b)
	sql := maChay(t, migration0034)
	seen := map[string]bool{}
	for _, m := range grantPair.FindAllStringSubmatch(sql, -1) {
		code := m[1]
		if seen[code] {
			continue
		}
		seen[code] = true
		if !strings.Contains(templates, `Code: "`+code+`"`) {
			t.Errorf("0034 grants to role %q, which domain.RoleTemplates does not declare — OUT-OF-SCOPE, not a guess", code)
		}
	}
	if len(seen) != 5 {
		t.Errorf("0034 grants to %d distinct roles, want 5", len(seen))
	}
}

// The grant goes to live roles BY CODE, idempotently, with its audit entry written from the grant's
// own RETURNING — same statement, same transaction (rule 6, invariant 3). No catalogue row, no
// removal, no rewrite.
func TestMigration0034GrantsLiveRolesByCodeWithAudit(t *testing.T) {
	sql := maChay(t, migration0034)
	for _, c := range []struct{ want, why string }{
		{"join grant_rule r on r.role_code = vt.ma where vt.deleted_at is null",
			"matched by stable code, live roles only"},
		{"on conflict (tenant_id, vai_tro_id, quyen_ma) do nothing returning tenant_id, vai_tro_id, quyen_ma",
			"idempotent: a key already held is neither re-granted nor re-audited"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),",
			"one system entry per role that gained a key, the Phân quyền verb, subject the role id"},
		{"from per_role;", "the entry is built from the INSERT's RETURNING"},
		{"'ly_do', 'migration 0034:", "the entry names this file, so REVERSAL can list its grants"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0034 does not contain %q — %s", c.want, c.why)
		}
	}
	if strings.Count(sql, "insert into vai_tro_quyen") != 1 {
		t.Error("0034 must grant in exactly one statement")
	}
	for _, bad := range []string{"insert into quyen", "drop ", "delete from", "truncate", "alter table", "update vai_tro", "update quyen"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0034 contains %q in its executable part", bad)
		}
	}
}
