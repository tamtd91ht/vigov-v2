package migrations

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0030 (one category tree per content type, owner 14/10/2026). Like the
// other files here they read the SQL this binary embeds; they do not prove PostgreSQL enforces it.

const file0030 = "0030_content_category_per_type.sql"

// categoryTypes is the ONE list a category may carry: 0006's item types minus `banner` (owner: a banner
// has no category).
var categoryTypes = []string{"'tin-tuc'", "'su-kien'", "'thong-bao'", "'truyen-thanh'", "'video'"}

// THE MUTATIONS THAT MUST TURN THIS RED: the column left nullable or given a DEFAULT; `banner` admitted or a
// type dropped from the CHECK; the CHECK added NOT VALID; NOT NULL / CHECK placed before the backfill; the
// backfill losing its `loai IS NULL` filter, counting deleted items or banners, or falling back to anything
// but `tin-tuc`; the audit entry leaving the statement; any write to noi_dung_mini_app; the slug key, the
// old tree index or a column touched; the new index not leading with tenant_id.
func TestMigration0030CategoryPerType(t *testing.T) {
	sql := executableSQL(t, file0030)
	for _, c := range []struct{ want, why string }{
		{"alter table danh_muc_mini_app add column if not exists loai text;", "added nullable, backfilled, then tightened"},
		{"alter table danh_muc_mini_app alter column loai set not null;", "every category belongs to one tree"},
		{"add constraint danh_muc_mini_app_type_known check (loai in ('tin-tuc', 'su-kien', 'thong-bao', 'truyen-thanh', 'video'));", "five category-bearing types, no banner"},
		{"where i.danh_muc_id is not null and i.deleted_at is null and i.loai <> 'banner'", "only live, non-banner items decide a category's type"},
		{"case when t.type_count = 1 then t.only_type else 'tin-tuc' end", "exactly one item type, else tin-tuc"},
		{"from danh_muc_mini_app c left join item_types t on t.tenant_id = c.tenant_id and t.danh_muc_id = c.id where c.loai is null", "only untyped categories, joined within one commune"},
		{"update danh_muc_mini_app d set loai = t.new_type, cap_nhat_luc = now() from target t where d.tenant_id = t.tenant_id and d.id = t.id and d.loai is null", "filtered UPDATE (rule 7 forbidden #2), retry-safe"},
		{"logged as ( insert into audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta) select tenant_id, 'system', 'system', '', 'sua_danh_muc_mini_app', slug, now(),", "audited in the same statement, system principal, slug as subject (rule 6)"},
		{"if audited <> written then raise exception", "no typed row without its entry"},
		{"on danh_muc_mini_app (tenant_id, loai, cha_id, thu_tu) where deleted_at is null", "per-type tree index leads with tenant_id"},
	} {
		if !strings.Contains(sql, c.want) {
			t.Errorf("0030 lacks %q — %s", c.want, c.why)
		}
	}

	// Backfill first, then NOT NULL, then the CHECK: tightening before the backfill fails on any populated
	// table, which is exactly the table the owner says does not exist and this file does not assume away.
	backfill := strings.Index(sql, "update danh_muc_mini_app d set loai")
	notNull := strings.Index(sql, "alter column loai set not null")
	check := strings.Index(sql, "add constraint danh_muc_mini_app_type_known")
	if backfill < 0 || notNull < backfill || check < notNull {
		t.Errorf("order backfill(%d) < not null(%d) < check(%d) broken", backfill, notNull, check)
	}

	for _, c := range []struct{ banned, why string }{
		{"update noi_dung_mini_app", "items are never refiled by this migration"},
		{"drop table", "drops a table"},
		{"drop column", "drops a column"},
		{"drop index", "the 0006 tree index is kept (question 4)"},
		{"drop constraint", "the slug key and every 0006 constraint stay"},
		{"delete from", "deletes rows"},
		{"not valid", "NOT VALID on a partitioned table"},
		{"set default", "a DEFAULT would file a category under a type nobody chose"},
		{"loai text default", "a DEFAULT would file a category under a type nobody chose"},
		{"foreign key", "item -> category type is the application's rule, under the tree lock (header)"},
		{"unique", "names-per-(type, parent) is the write path's; the slug key stays per commune"},
	} {
		if strings.Contains(sql, c.banned) {
			t.Errorf("0030 contains %q — %s", c.banned, c.why)
		}
	}
}

// THE MUTATION THAT MUST TURN THIS RED: the category list drifting from 0006's item list (a new item type
// added there and not here, or a category type the items cannot have), or `banner` creeping in.
func TestMigration0030TypesAreItemTypesWithoutBanner(t *testing.T) {
	sql := executableSQL(t, file0030)
	i := strings.Index(sql, "constraint danh_muc_mini_app_type_known check (")
	if i < 0 {
		t.Fatal("0030 lacks danh_muc_mini_app_type_known")
	}
	got := quoted.FindAllString(sql[i:i+strings.Index(sql[i:], ");")], -1)
	sort.Strings(got)
	want := append([]string{}, categoryTypes...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("category types = %v, want exactly %v", got, want)
	}

	items := executableSQL(t, "0006_noi_dung_mini_app.sql")
	j := strings.Index(items, "constraint noi_dung_mini_app_loai_hop_le check (")
	if j < 0 {
		t.Fatal("0006's item type CHECK not found")
	}
	var itemTypes []string
	for _, v := range quoted.FindAllString(items[j:j+strings.Index(items[j:], "),")], -1) {
		if v != "'banner'" {
			itemTypes = append(itemTypes, v)
		}
	}
	sort.Strings(itemTypes)
	if strings.Join(itemTypes, ",") != strings.Join(want, ",") {
		t.Errorf("0006 item types without banner = %v, category types = %v — the lists drifted", itemTypes, want)
	}
}

// The Vietnamese column name is deliberate (header) and must carry its rule-12 marker on the line above,
// where hooks/english_identifier_guard.py looks for it.
func TestMigration0030ColumnNameMarked(t *testing.T) {
	b, err := fs.ReadFile(FS, file0030)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	add := strings.Index(src, "ALTER TABLE danh_muc_mini_app ADD COLUMN IF NOT EXISTS loai TEXT;")
	if add < 0 {
		t.Fatal("ADD COLUMN loai not found")
	}
	above := src[:add]
	above = above[strings.LastIndex(strings.TrimRight(above, "\n"), "\n")+1:]
	if !strings.HasPrefix(above, "-- vi-name-ok: ") {
		t.Errorf("the line above ADD COLUMN loai is %q, want a `-- vi-name-ok:` marker", above)
	}
}
