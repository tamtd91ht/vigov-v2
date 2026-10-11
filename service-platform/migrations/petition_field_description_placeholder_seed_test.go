package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0023 (seed of petition_field.description_placeholder). The pg suites
// SKIP without VIGOV_TEST_DSN.
//
// THE MUTATIONS THAT MUST TURN THIS RED: a place name from the prototype (or the word "xã") reaching a
// tier-1 sentence every commune sees (ADR 0060 §4, ADR 0079); the UPDATE losing its NULL filter (it would
// overwrite an operator's sentence) or its code join (every row); the trail entry no longer written in
// the same statement (rule 6, invariant 3); a code renamed, a row inserted or deleted.
func TestMigration0023PetitionFieldPlaceholderSeed(t *testing.T) {
	const file = "0023_petition_field_description_placeholder_seed.sql"
	sql := executableSQL(t, file)
	for _, c := range []struct{ want, why string }{
		{"('rac-thai', 'ví dụ:", "rac-thai seeded"},
		{"('giao-thong', 'ví dụ:", "giao-thong seeded"},
		{"('cap-thoat-nuoc', 'ví dụ:", "cap-thoat-nuoc seeded"},
		{"('trat-tu-do-thi', 'ví dụ:", "trat-tu-do-thi seeded"},
		{"('khac', 'ví dụ:", "khac seeded"},
		{"where f.code = s.code and f.description_placeholder is null", "only the five codes, only rows still NULL"},
		{"returning f.code, f.description_placeholder ) insert into platform_audit_log (actor, action, subject, before, after, reason)", "trail entry in the same statement"},
		{"select 'system', 'petition_field.changed', c.code,", "the operator path's action name, actor system"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0023 lacks %q — %s", c.want, c.why)
		}
	}
	if n := strings.Count(sql, "update petition_field"); n != 1 {
		t.Errorf("0023 has %d UPDATEs of petition_field, want exactly the one filtered statement", n)
	}
	if n := strings.Count(sql, "'ví dụ:"); n != 5 {
		t.Errorf("0023 seeds %d sentences, want 5", n)
	}
	for _, c := range []struct{ banned, why string }{
		{"hà lam", "a commune's place name"},
		{"nguyễn thành", "a commune's place name"},
		{"tiểu la", "a commune's place name"},
		{"quốc lộ", "a commune's place name"},
		{"1a", "a commune's road"},
		{"thăng bình", "a commune's name"},
		{"kiệt", "a commune's alley / regional word"},
		{"số nhà", "a specific address"},
		{" xã", "wrong on a ward's form — use cán bộ / chính quyền"},
		{"set code", "a code is never renamed"},
		{"insert into petition_field", "no new code"},
		{"delete from", "deletes rows"},
		{"drop ", "drops something"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0023 contains %q — %s", c.banned, c.why)
		}
	}
}
