package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0022 (petition_field.description_placeholder). The pg suites SKIP
// without VIGOV_TEST_DSN.
//
// THE MUTATIONS THAT MUST TURN THIS RED: the column made NOT NULL or given a default (a sentence every
// commune would see, chosen by nobody); a seed of the prototype's sentences, which name one commune's
// places (ADR 0060 §4, ADR 0079) — until the owner picks the text, a seed is its own migration; the
// shape CHECK removed; a code touched; anything dropped or deleted.
func TestMigration0022PetitionFieldDescriptionPlaceholder(t *testing.T) {
	const file = "0022_petition_field_description_placeholder.sql"
	sql := executableSQL(t, file)
	for _, c := range []struct{ want, why string }{
		{"alter table petition_field add column if not exists description_placeholder text;", "nullable, no default"},
		{"check (description_placeholder is null or (btrim(description_placeholder) <> '' and char_length(description_placeholder) <= 500", "NULL or a bounded, non-blank sentence"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0022 lacks %q — %s", c.want, c.why)
		}
	}
	for _, c := range []struct{ banned, why string }{
		{"update petition_field", "no seed until the owner picks the text (header)"},
		{"insert into", "writes rows"},
		{"hà lam", "a commune's place name at tier 1"},
		{"nguyễn thành", "a commune's place name at tier 1"},
		{"tiểu la", "a commune's place name at tier 1"},
		{"default", "a default is a sentence nobody chose, on every commune"},
		{"drop column", "drops a column"},
		{"drop table", "drops a table"},
		{"delete from", "deletes rows"},
		{"alter column", "retypes a column"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0022 contains %q — %s", c.banned, c.why)
		}
	}
}
