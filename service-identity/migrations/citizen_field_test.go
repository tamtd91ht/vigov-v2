package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0036 (ADR 0101, owner 17/10/2026). The weaker half: they prove the
// statements are WRITTEN this way, not that PostgreSQL ran them — that needs VIGOV_TEST_DSN.

const migration0036 = "0036_citizen_field.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a code or type rule loosened, number_kind / date_pattern /
// options allowed on the wrong type, the immutability or no-remove trigger dropped, the key seeded under
// another band or label, the grant keyed on anything but admin.zalo or split from its audit entry,
// anything removed or rewritten.
func TestMigration0036CitizenFieldShape(t *testing.T) {
	sql := maChay(t, migration0036)
	for _, c := range []struct{ want, why string }{
		{"primary key (tenant_id, code)", "one code per commune, composite with tenant_id (rule 1 inv 6)"},
		{"check (code ~ '^[a-z][a-z0-9]{5,7}$')", "ADR 0101 #1: 6–8 characters, a letter first"},
		{"check (field_type in ('singletext', 'multitext', 'combobox', 'singlebox', 'number', 'datetime'))", "the six types"},
		{"(field_type = 'number' and number_kind in ('integer', 'decimal')) or (field_type <> 'number' and number_kind is null)", "number_kind only for number"},
		{"(field_type = 'datetime' and date_pattern in ('dd/mm/yyyy', 'dd/mm/yyyy hh:mm', 'mm/yyyy', 'yyyy')) or (field_type <> 'datetime' and date_pattern is null)", "date_pattern only for dateTime, fixed list"},
		{"(field_type in ('combobox', 'singlebox') and options is not null and citizen_field_options_valid(options)) or (field_type not in ('combobox', 'singlebox') and options is null)", "options only for the two choice types"},
		{"(e->>'value') !~ '^[a-z0-9]+(-[a-z0-9]+)*$'", "option value shape"},
		{"raise exception 'citizen_field: hard delete refused'", "no hard delete"},
		{"raise exception 'citizen_field: `code` is immutable'", "code immutable"},
		{"raise exception 'citizen_field: `field_type` is immutable'", "type immutable"},
		{"raise exception 'citizen_field: an option value cannot be removed'", "an option is retired, never removed"},
		{"before update or delete on citizen_field", "the guard is attached"},
		{"alter table citizen_contact add column if not exists email text;", "email, nullable"},
		{"alter table citizen_contact add column if not exists attributes jsonb not null default '{}'::jsonb;", "attributes, constant default (no rewrite)"},
		{"check (jsonb_typeof(attributes) = 'object')", "attributes is an object"},
		{"insert into quyen (ma, nhom, nhan, thu_tu) values ('admin.citizen_field', 'quản trị', 'cấu hình thuộc tính danh bạ người dân', 53) on conflict (ma) do update set", "one key, QUẢN TRỊ band, the owner's label, a free rank"},
		{"values ('admin.zalo', 'admin.citizen_field')", "granted to exactly the holders of admin.zalo"},
		{"where vt.deleted_at is null", "live roles only"},
		{"on conflict (tenant_id, vai_tro_id, quyen_ma) do nothing returning tenant_id, vai_tro_id, quyen_ma", "idempotent grant"},
		{"insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),", "one system entry per role that gained the key"},
		{"partition by hash (tenant_id)", "event_outbox partitioned (ADR 0096)"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0036 does not contain %q — %s", c.want, c.why)
		}
	}
	// Built from parts so the words never sit together in this file's text.
	for _, bad := range []string{"dr" + "op table", "dr" + "op column", "del" + "ete from", "trunc" + "ate", "update vai_tro", "update quyen", "update citizen_contact"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0036 contains %q in its executable part", bad)
		}
	}
}

const migration0038 = "0038_citizen_field_display_mode.sql"

// THE MUTATIONS THAT MUST TURN THIS RED: a column made nullable or given a non-constant default (a
// rewrite), a backfill's filter widened or dropped (an UPDATE over every row — rule 7), a mode allowed on
// a type without options, a third mode or width, or the immutability trigger redefined to cover either.
func TestMigration0038CitizenFieldDisplayMode(t *testing.T) {
	sql := maChay(t, migration0038)
	for _, c := range []struct{ want, why string }{
		{"alter table citizen_field add column if not exists display_mode text not null default '';", "additive, constant default (no rewrite)"},
		{"update citizen_field set display_mode = 'full' where field_type in ('combobox', 'singlebox') and display_mode = '';", "backfill filtered to the rows the CHECK would refuse"},
		{"check ((field_type in ('combobox', 'singlebox') and display_mode in ('full', 'dropdown')) or (field_type not in ('combobox', 'singlebox') and display_mode = ''))", "full|dropdown for the two choice types, '' for the rest"},
		{"alter table citizen_field add column if not exists layout_width text not null default 'half';", "additive, constant default (no rewrite)"},
		{"update citizen_field set layout_width = 'full' where field_type = 'multitext' and layout_width = 'half';", "backfill filtered to multiText rows"},
		{"check (layout_width in ('half', 'full'))", "two widths of the two-column form"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0038 does not contain %q — %s", c.want, c.why)
		}
	}
	if n := strings.Count(sql, "update "); n != 2 {
		t.Errorf("0038 has %d UPDATE statements, want exactly the two filtered backfills", n)
	}
	for _, bad := range []string{"dr" + "op table", "dr" + "op column", "del" + "ete from", "trunc" + "ate", "citizen_field_keep"} {
		if strings.Contains(sql, bad) {
			t.Errorf("0038 contains %q in its executable part", bad)
		}
	}
}
