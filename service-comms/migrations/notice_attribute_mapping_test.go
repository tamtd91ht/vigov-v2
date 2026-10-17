package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0034 (attribute mapping — ADR 0101 #12, TASK E-2). They assert the CHECKs
// and the guard are WRITTEN; PostgreSQL enforcing them needs VIGOV_TEST_DSN (store's pg suite).

const file0034 = "0034_notice_campaign_attribute_mapping.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: the column given a default (a table rewrite, and a value nobody
// chose); the mapping allowed on typed / imported audiences; a staff audience mapping anything but full_name;
// a value not shaped like a code (a value stored by mistake); more than 20 entries; the guard no longer
// freezing the mapping; a row rewrite or a dropped column.
func TestMigration0034AttributeMapping(t *testing.T) {
	sql := executableSQL(t, file0034)
	for _, c := range []struct{ want, why string }{
		{"alter table notice_campaign add column if not exists attribute_mapping jsonb;", "nullable, no default"},
		{"jsonb_typeof(attribute_mapping) = 'object'", "an object"},
		{"jsonb_array_length(jsonb_path_query_array(attribute_mapping, '$.keyvalue()')) between 1 and 20", "1..20 entries"},
		{`'$.* ? (@.type() != "string" || !(@ like_regex "^[a-z_][a-z0-9_]{1,31}$"))'`, "every value a code-shaped string"},
		{"check (attribute_mapping is null or audience_kind not in ('danh-sach-so', 'tep-nhap'))", "directory audiences only"},
		{"or not jsonb_path_exists(attribute_mapping, '$.* ? (@ != \"full_name\")')", "staff: full_name only"},
		{"audience_kind not in ('can-bo-tat-ca', 'can-bo-theo-bo-phan')", "the two staff audiences"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0034 lacks %q — %s", c.want, c.why)
		}
	}
	body := functionBody(t, sql, "notice_campaign_guard")
	for _, want := range []string{
		"or new.attribute_mapping is distinct from old.attribute_mapping",
		// 0033's frozen columns must survive the replacement.
		"or new.recipient_mapping is distinct from old.recipient_mapping",
		"or new.params is distinct from old.params",
		"or new.created_by is distinct from old.created_by",
		"if tg_op = 'delete' then raise exception",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("notice_campaign_guard lacks %q", want)
		}
	}
	noDestruction(t, file0034, sql)
}

// 0034 declares no new entity: it only widens 0032's notice_campaign.
func TestMigration0034DeclaresNoEntity(t *testing.T) {
	b, err := fs.ReadFile(FS, file0034)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "-- @entity:") {
		t.Error("0034 adds an @entity mark")
	}
}
