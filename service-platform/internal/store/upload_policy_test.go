package store

// Checks on migration 0008 and on the read statement that run WITHOUT a database. The pg tests
// beside this file prove the behaviour; these pin what a one-row fixture cannot: the seed values the
// owner confirmed, the link between the seed and core/storage's allow-list, and the clauses whose
// loss would turn nothing red.
//
// WHY THE SEED IS CHECKED AGAINST core/storage HERE, IN service-platform: the seed is platform's
// data and core/storage is the allow-list a policy may only narrow (platform.proto). A MIME typo in
// the SQL ("image/jpg") would not fail the migration — it would ship a policy whose every upload is
// refused at step (c), discovered by a commune. Checking the SQL text, rather than a Go copy of the
// seed, means the file that is applied is the file that is checked.

import (
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-platform/migrations"
)

const uploadPolicyMigration = "0008_upload_policy.sql"

func readUploadPolicyMigration(t *testing.T) string {
	t.Helper()
	b, err := fs.ReadFile(migrations.FS, uploadPolicyMigration)
	if err != nil {
		t.Fatalf("read %s: %v", uploadPolicyMigration, err)
	}
	return string(b)
}

type seedRow struct {
	purpose  string
	maxBytes int64
	mimes    []string
	maxFiles string // "NULL" or a number, as written
}

var seedRowPattern = regexp.MustCompile(
	`\('([a-z-]+)',\s*(\d+),\s*ARRAY\[([^\]]*)\],\s*(NULL|\d+),\s*'system',\s*'system'\)`)

func parseSeed(t *testing.T, sql string) []seedRow {
	t.Helper()
	var out []seedRow
	for _, m := range seedRowPattern.FindAllStringSubmatch(sql, -1) {
		n, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			t.Fatalf("max_bytes %q: %v", m[2], err)
		}
		var mimes []string
		for _, q := range strings.Split(m[3], ",") {
			mimes = append(mimes, strings.Trim(strings.TrimSpace(q), "'"))
		}
		out = append(out, seedRow{purpose: m[1], maxBytes: n, mimes: mimes, maxFiles: m[4]})
	}
	return out
}

// The owner's values of 2026-09-28, exactly.
func TestUploadPolicySeedMatchesOwnerDecision(t *testing.T) {
	images := []string{"image/jpeg", "image/png", "image/webp", "image/heic"}
	want := map[string]seedRow{
		"content-video":      {maxBytes: 2147483648, mimes: []string{"video/mp4", "video/quicktime"}, maxFiles: "NULL"},
		"content-image":      {maxBytes: 10485760, mimes: images, maxFiles: "NULL"},
		"tenant-logo":        {maxBytes: 10485760, mimes: images, maxFiles: "NULL"},
		"petition-photo":     {maxBytes: 10485760, mimes: images, maxFiles: "5"},
		"document-scan":      {maxBytes: 52428800, mimes: []string{"application/pdf", "image/jpeg", "image/png"}, maxFiles: "NULL"},
		"content-attachment": {maxBytes: 52428800, mimes: []string{"application/pdf"}, maxFiles: "NULL"},
	}
	rows := parseSeed(t, readUploadPolicyMigration(t))
	if len(rows) != len(want) {
		t.Fatalf("seed rows = %d, want %d — the pattern or the seed changed", len(rows), len(want))
	}
	for _, r := range rows {
		w, ok := want[r.purpose]
		if !ok {
			t.Errorf("unexpected seed purpose %q", r.purpose)
			continue
		}
		if r.maxBytes != w.maxBytes || !slices.Equal(r.mimes, w.mimes) || r.maxFiles != w.maxFiles {
			t.Errorf("%s = %+v, want %+v", r.purpose, r, w)
		}
		delete(want, r.purpose)
	}
	for p := range want {
		t.Errorf("purpose %q not seeded", p)
	}
}

// Every seeded MIME value is one core/storage can sniff; every seeded purpose is one core/storage
// holds AND one the proto enum can name.
func TestUploadPolicySeedWithinStorageAllowList(t *testing.T) {
	for _, r := range parseSeed(t, readUploadPolicyMigration(t)) {
		for _, m := range r.mimes {
			if _, ok := storage.ExtForMIME(m); !ok {
				t.Errorf("%s: MIME %q is not in core/storage's allow-list (core/storage/mime.go)", r.purpose, m)
			}
		}
		if !slices.Contains(storage.Purposes(), storage.Purpose(r.purpose)) {
			t.Errorf("seed purpose %q is not a core/storage purpose", r.purpose)
		}
		if v, ok := purposeEnumOf(r.purpose); !ok {
			t.Errorf("seed purpose %q has no UploadPurpose value (got %v)", r.purpose, v)
		}
	}
}

// purposeEnumOf applies platform.proto's derivation rule in reverse, independently of
// internal/grpc — so this test and that package cannot share one mistake.
func purposeEnumOf(p string) (int32, bool) {
	v, ok := platformv1.UploadPurpose_value["UPLOAD_PURPOSE_"+strings.ToUpper(strings.ReplaceAll(p, "-", "_"))]
	return v, ok && v != 0
}

var (
	purposeCheckDefine = regexp.MustCompile(`(?s)upload_policy_purpose_known CHECK \(purpose IN \((.*?)\)\)`)
	purposeCheckDrop   = regexp.MustCompile(`DROP CONSTRAINT (?:IF EXISTS )?upload_policy_purpose_known\b`)
	sqlLineComment     = regexp.MustCompile(`(?m)--.*$`)
)

// currentPurposeCheck returns the value list of upload_policy_purpose_known as the LAST migration
// touching it leaves it, and that file's name. Every migration is read in filename order (the order
// pkg/migrate applies them), comments stripped so reversal prose cannot pose as a definition; within
// a file the last DROP or definition wins. A trailing DROP with no re-definition fails the test: the
// database would then hold no copy of the list at all.
//
// WHY NOT A FIXED FILE NAME: the constraint is re-defined by a new migration every time core/storage
// gains a purpose (0008 is immutable once applied). A test pinned to 0008 would compare storage with
// a list the database no longer holds — red for the right change, or green for the wrong one.
func currentPurposeCheck(t *testing.T) (list, file string) {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	dropped := false
	for _, e := range entries { // fs.ReadDir sorts by filename
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		sql := sqlLineComment.ReplaceAllString(string(b), "")
		lastDefine, lastDrop := -1, -1
		var defineList string
		if ms := purposeCheckDefine.FindAllStringSubmatchIndex(sql, -1); ms != nil {
			m := ms[len(ms)-1]
			lastDefine, defineList = m[0], sql[m[2]:m[3]]
		}
		if ms := purposeCheckDrop.FindAllStringIndex(sql, -1); ms != nil {
			lastDrop = ms[len(ms)-1][0]
		}
		switch {
		case lastDefine > lastDrop:
			list, file, dropped = defineList, e.Name(), false
		case lastDrop > lastDefine:
			file, dropped = e.Name(), true
		}
	}
	if dropped {
		t.Fatalf("%s drops upload_policy_purpose_known without re-defining it", file)
	}
	if file == "" {
		t.Fatal("purpose CHECK not found in any migration")
	}
	return list, file
}

// The CHECK list on upload_policy.purpose is the database's copy of the closed list; it must equal
// core/storage's, or the operator can store a purpose nobody can use (or cannot store one they need).
func TestUploadPolicyPurposeCheckMatchesStorage(t *testing.T) {
	list, file := currentPurposeCheck(t)
	var got []string
	for _, q := range strings.Split(list, ",") {
		got = append(got, strings.Trim(strings.TrimSpace(q), "'"))
	}
	var want []string
	for _, p := range storage.Purposes() {
		want = append(want, string(p))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("CHECK purposes (%s) = %v, core/storage = %v — add a migration re-defining the CHECK", file, got, want)
	}
}

func TestUploadPolicyMigrationConstraintsAndTriggers(t *testing.T) {
	sql := readUploadPolicyMigration(t)
	for _, clause := range []string{
		"-- @entity: UploadPolicy",
		"-- @entity: PlatformAuditEntry",
		"CHECK (max_bytes > 0 AND max_bytes <= 5368709120)",
		"CHECK (cardinality(allowed_mime_types) > 0)",
		"CHECK (max_files_per_subject IS NULL OR max_files_per_subject >= 1)",
		"upload_policy_soft_delete_complete",
		"BEFORE DELETE ON upload_policy",
		"BEFORE UPDATE OR DELETE ON platform_audit_log",
		"FOR EACH ROW EXECUTE FUNCTION platform_audit_log_append_only()",
		"BEFORE TRUNCATE ON platform_audit_log",
		"FOR EACH STATEMENT EXECUTE FUNCTION platform_audit_log_append_only()",
		// The trail is written in the same statement as the seed, from what was inserted.
		"ON CONFLICT (purpose) DO NOTHING",
		"RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject",
		"'system', 'upload_policy.seeded'",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("migration 0008 lacks %q", clause)
		}
	}
	// Rule 1, forbidden #1: no made-up commune on the platform trail.
	auditTable := sql[strings.Index(sql, "CREATE TABLE IF NOT EXISTS platform_audit_log"):]
	auditTable = auditTable[:strings.Index(auditTable, ");")]
	if strings.Contains(auditTable, "tenant_id") {
		t.Error("platform_audit_log carries tenant_id — the owner decided it has none")
	}
	if strings.Count(sql, "@scope:  platform") != 2 {
		t.Error("both tables must declare @scope: platform")
	}
}

// The read excludes soft-deleted rows — a withdrawn policy must read as not configured.
func TestListUploadPoliciesExcludesSoftDeleted(t *testing.T) {
	if !strings.Contains(listUploadPolicies, "WHERE deleted_at IS NULL") {
		t.Errorf("read does not exclude soft-deleted rows:\n%s", listUploadPolicies)
	}
}
