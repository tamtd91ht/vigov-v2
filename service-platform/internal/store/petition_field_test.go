package store

// Checks on migration 0011 (petition_field, tier 1 of ADR 0026) and on its read statement, WITHOUT a
// database. petition_field_pg_test.go proves the behaviour; these pin what a fixture cannot: the
// exact code set, and the clauses whose loss would turn nothing red.

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/migrations"
)

const petitionFieldMigration = "0011_petition_field.sql"

type fieldSeed struct {
	code, label string
	order       int
	icon, tone  string
	active      string
	file        string
}

var fieldSeedPattern = regexp.MustCompile(
	`\('([a-z0-9-]+)',\s*'([^']*)',\s*(\d+),\s*'([^']*)',\s*'([^']*)',\s*(true|false),\s*'system',\s*'system'\)`)

// allFieldSeeds returns the petition_field seed rows of EVERY migration, comments stripped, so a code
// added by a later file is pinned too — and a code seeded twice (the second a silent ON CONFLICT
// no-op) fails.
func allFieldSeeds(t *testing.T) []fieldSeed {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	var out []fieldSeed
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, m := range fieldSeedPattern.FindAllStringSubmatch(sqlLineComment.ReplaceAllString(string(b), ""), -1) {
			n, _ := strconv.Atoi(m[3])
			if prev, dup := seen[m[1]]; dup {
				t.Errorf("code %q seeded in %s and again in %s — the second is a no-op", m[1], prev, e.Name())
			}
			seen[m[1]] = e.Name()
			out = append(out, fieldSeed{code: m[1], label: m[2], order: n, icon: m[4], tone: m[5],
				active: m[6], file: e.Name()})
		}
	}
	return out
}

// THE TWELVE CODES, EXACTLY — and they are the same twelve that key service-identity's SLA seed
// (service-identity/internal/domain/sla_gieo.go:141-152, pinned there by sla_gieo_test.go). The two
// modules cannot import each other (rule 2, forbidden #1), so each pins the SAME literal set; a code
// changed on one side only turns that side's test red here or there. A mismatch is not cosmetic: a
// code identity has an SLA row for but platform does not list is refused at intake, and the reverse
// silently gets the default deadline (identity.proto, ResolveDeadlines).
//
// Labels and order: docs/ui-ux/09-phan-anh-nguoi-dan.md §5. Icons, tones: ADR 0060 §5.
func TestPetitionFieldSeedIsTheTwelveCodes(t *testing.T) {
	want := []fieldSeed{
		{code: "rac-thai", label: "Rác thải – Vệ sinh môi trường", order: 1, icon: "Trash2", tone: "orange"},
		{code: "giao-thong", label: "Hạ tầng giao thông", order: 2, icon: "TrafficCone", tone: "blue"},
		{code: "cap-thoat-nuoc", label: "Cấp thoát nước", order: 3, icon: "Droplets", tone: "cyan"},
		{code: "dien", label: "Điện", order: 4, icon: "Zap", tone: "orange"},
		{code: "trat-tu-do-thi", label: "Trật tự đô thị – lấn chiếm vỉa hè", order: 5, icon: "Construction", tone: "purple"},
		{code: "an-ninh", label: "An ninh trật tự", order: 6, icon: "ShieldAlert", tone: "red"},
		{code: "xay-dung", label: "Xây dựng không phép", order: 7, icon: "Hammer", tone: "cyan"},
		{code: "o-nhiem", label: "Ô nhiễm (tiếng ồn, khí thải, nước thải)", order: 8, icon: "Factory", tone: "green"},
		{code: "y-te-giao-duc", label: "Y tế – Giáo dục", order: 9, icon: "Stethoscope", tone: "blue"},
		{code: "can-bo", label: "Thái độ / tác phong cán bộ", order: 10, icon: "UserRoundX", tone: "purple"},
		{code: "an-toan-thuc-pham", label: "An toàn thực phẩm", order: 11, icon: "Utensils", tone: "green"},
		{code: "khac", label: "Khác", order: 12, icon: "MessageSquare", tone: "blue"},
	}
	got := allFieldSeeds(t)
	if len(got) != len(want) {
		t.Fatalf("seed rows = %d, want %d — the pattern or the seed changed", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		w.active, w.file = "true", petitionFieldMigration
		if g != w {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
}

// The spec's dead code must never come back as a thirteenth field (ADR 0026 §2; sla_gieo.go:155).
func TestPetitionFieldSeedHasNoRetiredSpecCode(t *testing.T) {
	for _, r := range allFieldSeeds(t) {
		if r.code == "ve-sinh-moi-truong" {
			t.Errorf("%s seeds ve-sinh-moi-truong — the old code of rac-thai", r.file)
		}
	}
}

func TestPetitionFieldMigrationClauses(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, petitionFieldMigration)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, clause := range []string{
		"-- @entity: PetitionField",
		"-- @scope:  platform",
		"code          TEXT        PRIMARY KEY",
		// No default on active: every write states it.
		"active        BOOLEAN     NOT NULL,",
		"CHECK (tone IN ('', 'blue', 'green', 'orange', 'purple', 'cyan', 'red'))",
		"length(code) <= 64",
		// Never deleted, never renamed.
		"BEFORE UPDATE OR DELETE ON petition_field",
		"IF TG_OP = 'DELETE' THEN",
		"IF NEW.code IS DISTINCT FROM OLD.code THEN",
		// The trail is written in the same statement, from what was inserted.
		"ON CONFLICT (code) DO NOTHING",
		"RETURNING code, default_label, sort_order, icon, tone, active",
		"INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)",
		"'system', 'petition_field.seeded'",
		"'migration " + petitionFieldMigration + ":",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration 0011 lacks %q", clause)
		}
	}
	table := sql[strings.Index(sql, "CREATE TABLE IF NOT EXISTS petition_field"):]
	table = sqlLineComment.ReplaceAllString(table[:strings.Index(table, ");")], "")
	// No commune (ADR 0026) and no soft delete (ADR 0060 §4: a soft-deleted code would drop old
	// petitions' labels from every read path).
	for _, forbidden := range []string{"tenant_id", "deleted_at"} {
		if strings.Contains(table, forbidden) {
			t.Errorf("petition_field declares %s", forbidden)
		}
	}
}

// The read returns RETIRED codes too, in a total order.
func TestListPetitionFieldsReadsEveryCode(t *testing.T) {
	q := strings.ToLower(listPetitionFields)
	if strings.Contains(q, "where") {
		t.Errorf("read filters rows — retired codes must still be returned:\n%s", listPetitionFields)
	}
	if !strings.Contains(q, "order by sort_order, code") {
		t.Errorf("read order is not total:\n%s", listPetitionFields)
	}
}
