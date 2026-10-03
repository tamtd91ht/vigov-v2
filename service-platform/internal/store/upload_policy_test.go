package store

// Checks on the upload_policy migrations (0008 and every later seed) and on the read statement that
// run WITHOUT a database. The pg tests beside this file prove the behaviour; these pin what a one-row
// fixture cannot: the seed values the
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
	file     string // the migration that seeds it; set by allSeeds
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

// allSeeds returns the seed rows of EVERY migration, in the order pkg/migrate applies them, each
// tagged with its file. Comments are stripped first, so reversal prose cannot pose as a seed.
//
// WHY EVERY FILE AND NOT 0008: a purpose gains its policy in a later migration whenever its limits
// are decided after 0008 was applied (0010 for task-attachment). A test reading only 0008 would
// never see that row — a MIME typo there would ship green. A purpose seeded twice fails the test:
// ON CONFLICT DO NOTHING makes the second row a silent no-op, so the pinned value would describe a
// row the database never holds.
func allSeeds(t *testing.T) []seedRow {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	var out []seedRow
	seen := map[string]string{}
	for _, e := range entries { // fs.ReadDir sorts by filename
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, r := range parseSeed(t, sqlLineComment.ReplaceAllString(string(b), "")) {
			if prev, dup := seen[r.purpose]; dup {
				t.Errorf("purpose %q seeded in %s and again in %s — the second is a no-op", r.purpose, prev, e.Name())
			}
			seen[r.purpose] = e.Name()
			r.file = e.Name()
			out = append(out, r)
		}
	}
	return out
}

// The owner's values, exactly: six of 2026-09-28 (0008), task-attachment of 2026-09-29 (0010),
// content-audio of 2026-10-01 (0013, ADR 0067 §4), the two staff petition purposes of 2026-10-02 (0015).
// These are the SEEDED values as written in each file. content-image's seed is later rewritten by
// 0012 (TestUploadPolicyContentImageCoverChange pins that) and petition-photo's by 0014
// (TestUploadPolicyPetitionPhotoNoHEICChange, G3: no HEIC), so their effective values are not these.
func TestUploadPolicySeedMatchesOwnerDecision(t *testing.T) {
	images := []string{"image/jpeg", "image/png", "image/webp", "image/heic"}
	want := map[string]seedRow{
		"content-video":      {maxBytes: 2147483648, mimes: []string{"video/mp4", "video/quicktime"}, maxFiles: "NULL"},
		"content-image":      {maxBytes: 10485760, mimes: images, maxFiles: "NULL"},
		"tenant-logo":        {maxBytes: 10485760, mimes: images, maxFiles: "NULL"},
		"petition-photo":     {maxBytes: 10485760, mimes: images, maxFiles: "5"},
		"document-scan":      {maxBytes: 52428800, mimes: []string{"application/pdf", "image/jpeg", "image/png"}, maxFiles: "NULL"},
		"content-attachment": {maxBytes: 52428800, mimes: []string{"application/pdf"}, maxFiles: "NULL"},
		// Người dùng chốt 29/09/2026: the document-scan values.
		"task-attachment": {maxBytes: 52428800, mimes: []string{"application/pdf", "image/jpeg", "image/png"}, maxFiles: "NULL", file: "0010_upload_policy_seed_task_attachment.sql"},
		// Chủ dự án chốt 01/10/2026 (ADR 0067 §4): 30 MiB, MP3/M4A, one broadcast file per item.
		"content-audio": {maxBytes: 31457280, mimes: []string{"audio/mpeg", "audio/mp4"}, maxFiles: "1", file: "0013_upload_policy_content_audio.sql"},
		// Chủ dự án chốt 02/10/2026 that both kinds exist (C, B); the VALUES were set by precedent the
		// same day (0015 header): the photo = petition-photo after 0014 (10 MiB, no HEIC, 5), the log
		// attachment = task-attachment of 0010.
		"petition-verification-photo": {maxBytes: 10485760, mimes: []string{"image/jpeg", "image/png", "image/webp"}, maxFiles: "5", file: "0015_upload_policy_petition_staff_files.sql"},
		"petition-log-attachment":     {maxBytes: 52428800, mimes: []string{"application/pdf", "image/jpeg", "image/png"}, maxFiles: "NULL", file: "0015_upload_policy_petition_staff_files.sql"},
		// Chủ dự án chốt 02/10/2026 (ADR 0069 #5): banner web-admin 2 MiB, PNG/WebP/JPEG, no count limit.
		// tenant-logo's seed above is rewritten by 0016 (TestUploadPolicyTenantLogoBannerChange).
		"tenant-banner": {maxBytes: 2097152, mimes: []string{"image/png", "image/webp", "image/jpeg"}, maxFiles: "NULL", file: tenantLogoBannerMigration},
		// Chủ dự án chốt 03/10/2026 (ADR 0067 amendment): body images of a Mini App article — the cover's
		// values after 0012 (TestUploadPolicyContentBodyImageCopiesCover pins the link), at most 20 per article.
		"content-body-image": {maxBytes: 52428800, mimes: []string{"image/jpeg", "image/png", "image/webp"}, maxFiles: "20", file: contentBodyImageMigration},
	}
	for p, w := range want {
		if w.file == "" {
			w.file = uploadPolicyMigration
			want[p] = w
		}
	}
	rows := allSeeds(t)
	if len(rows) != len(want) {
		t.Fatalf("seed rows = %d, want %d — the pattern or the seed changed", len(rows), len(want))
	}
	for _, r := range rows {
		w, ok := want[r.purpose]
		if !ok {
			t.Errorf("unexpected seed purpose %q", r.purpose)
			continue
		}
		if r.maxBytes != w.maxBytes || !slices.Equal(r.mimes, w.mimes) || r.maxFiles != w.maxFiles || r.file != w.file {
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
	rows := allSeeds(t)
	if len(rows) == 0 {
		t.Fatal("no seed rows parsed — the pattern or the seed changed")
	}
	for _, r := range rows {
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

// Every migration that seeds a policy writes its trail entry the way 0008 does: in the same
// statement, from what the INSERT returned, so a re-run writes no second entry — and the entry's
// reason names the file that wrote it.
func TestUploadPolicySeedFilesWriteTrail(t *testing.T) {
	files := map[string]bool{}
	for _, r := range allSeeds(t) {
		files[r.file] = true
	}
	for f := range files {
		b, err := fs.ReadFile(migrations.FS, f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		sql := sqlLineComment.ReplaceAllString(string(b), "")
		for _, clause := range []string{
			"WITH seeded AS (",
			"ON CONFLICT (purpose) DO NOTHING",
			"RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject",
			"INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)",
			"'system', 'upload_policy.seeded'",
			"FROM seeded s;",
			"'migration " + f + ":",
		} {
			if !strings.Contains(sql, clause) {
				t.Errorf("%s seeds a policy but lacks %q", f, clause)
			}
		}
	}
}

const contentImageCoverMigration = "0012_upload_policy_content_image_cover.sql"

// Người dùng chốt 01/10/2026: the Mini App cover image is 50 MiB, JPEG/PNG/WebP, no HEIC
// (docs/ui-ux/11-noi-dung-mini-app.md:126,201). 0012 rewrites 0008's content-image row to that —
// but ONLY while the row still holds 0008's seed, signed 'system' and live, so a limit an operator
// set is never overwritten; and it writes the trail from what the UPDATE returned, in the same
// statement. Each clause below is one whose loss would turn no other test red.
func TestUploadPolicyContentImageCoverChange(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, contentImageCoverMigration)
	if err != nil {
		t.Fatalf("read %s: %v", contentImageCoverMigration, err)
	}
	sql := sqlLineComment.ReplaceAllString(string(b), "")
	for _, clause := range []string{
		// The new values.
		"SET max_bytes          = 52428800",
		"allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp']",
		"updated_by         = 'system'",
		// The filter: one purpose, untouched seed state only (rule 7 forbidden #2; never overwrite).
		"WHERE u.purpose = 'content-image'",
		"AND u.deleted_at IS NULL",
		"AND u.updated_by = 'system'",
		"AND u.max_bytes = 10485760",
		"AND u.allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic']::text[]",
		"AND u.max_files_per_subject IS NULL",
		// The trail, same statement, before and after from the row itself (rule 6 inv 3, 5, 6).
		"INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)",
		"'system', 'upload_policy.changed'",
		"FROM changed c;",
		"'migration " + contentImageCoverMigration + ":",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("%s lacks %q", contentImageCoverMigration, clause)
		}
	}
	// The values set must stay inside core/storage's allow-list, and HEIC must be gone.
	set := regexp.MustCompile(`allowed_mime_types = ARRAY\[([^\]]*)\],`).FindStringSubmatch(sql)
	if set == nil {
		t.Fatal("new MIME list not found")
	}
	for _, q := range strings.Split(set[1], ",") {
		m := strings.Trim(strings.TrimSpace(q), "'")
		if m == storage.MIMEHEIC {
			t.Error("content-image still allows HEIC — the user decided against it on 2026-10-01")
		}
		if _, ok := storage.ExtForMIME(m); !ok {
			t.Errorf("MIME %q is not in core/storage's allow-list", m)
		}
	}
}

const petitionPhotoNoHEICMigration = "0014_upload_policy_petition_photo_no_heic.sql"

// Chủ dự án chốt G3 (ADR 0047:255, 30/09/2026, nhắc lại 02/10/2026): scene photos are JPEG/PNG/WebP
// only, no HEIC; 10 MiB and 5 files stay. 0014 rewrites 0008's petition-photo row to that — ONLY
// while the row still holds 0008's seed, signed 'system' and live — and writes the trail from what
// the UPDATE returned, in the same statement. Each clause is one whose loss would turn no other test red.
func TestUploadPolicyPetitionPhotoNoHEICChange(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, petitionPhotoNoHEICMigration)
	if err != nil {
		t.Fatalf("read %s: %v", petitionPhotoNoHEICMigration, err)
	}
	sql := sqlLineComment.ReplaceAllString(string(b), "")
	for _, clause := range []string{
		// The new value; size and count are deliberately not SET (G3 says nothing about them).
		"SET allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp'],",
		"updated_by         = 'system'",
		// The filter: one purpose, untouched seed state only (rule 7 forbidden #2; never overwrite).
		"WHERE u.purpose = 'petition-photo'",
		"AND u.deleted_at IS NULL",
		"AND u.updated_by = 'system'",
		"AND u.max_bytes = 10485760",
		"AND u.allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic']::text[]",
		"AND u.max_files_per_subject = 5",
		// The trail, same statement, before and after from the row itself (rule 6 inv 3, 5, 6).
		"INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)",
		"'system', 'upload_policy.changed'",
		"FROM changed c;",
		"'migration " + petitionPhotoNoHEICMigration + ":",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("%s lacks %q", petitionPhotoNoHEICMigration, clause)
		}
	}
	for _, forbidden := range []string{"max_bytes          =", "max_files_per_subject ="} {
		if strings.Contains(sql[:strings.Index(sql, "FROM upload_policy AS prev")], forbidden) {
			t.Errorf("%s SETs %q — G3 keeps 10 MiB and 5 files", petitionPhotoNoHEICMigration, forbidden)
		}
	}
	set := regexp.MustCompile(`SET allowed_mime_types = ARRAY\[([^\]]*)\],`).FindStringSubmatch(sql)
	if set == nil {
		t.Fatal("new MIME list not found")
	}
	var got []string
	for _, q := range strings.Split(set[1], ",") {
		got = append(got, strings.Trim(strings.TrimSpace(q), "'"))
	}
	if want := []string{storage.MIMEJPEG, storage.MIMEPNG, storage.MIMEWebP}; !slices.Equal(got, want) {
		t.Errorf("petition-photo MIME list = %v, want %v (G3; no HEIC)", got, want)
	}
}

const tenantLogoBannerMigration = "0016_upload_policy_tenant_logo_banner.sql"

// Chủ dự án chốt 02/10/2026 (ADR 0069 #4): the commune logo is PNG/WebP/JPEG ≤ 2 MiB, no HEIC. 0016
// rewrites 0008's tenant-logo row to that — ONLY while the row still holds 0008's seed, signed
// 'system' and live — and writes the trail from what the UPDATE returned, in the same statement. Each
// clause is one whose loss would turn no other test red.
func TestUploadPolicyTenantLogoBannerChange(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, tenantLogoBannerMigration)
	if err != nil {
		t.Fatalf("read %s: %v", tenantLogoBannerMigration, err)
	}
	sql := sqlLineComment.ReplaceAllString(string(b), "")
	for _, clause := range []string{
		"SET max_bytes          = 2097152",
		"allowed_mime_types = ARRAY['image/png', 'image/webp', 'image/jpeg'],",
		"updated_by         = 'system'",
		// The filter: one purpose, untouched seed state only (rule 7 forbidden #2; never overwrite).
		"WHERE u.purpose = 'tenant-logo'",
		"AND u.deleted_at IS NULL",
		"AND u.updated_by = 'system'",
		"AND u.max_bytes = 10485760",
		"AND u.allowed_mime_types = ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic']::text[]",
		"AND u.max_files_per_subject IS NULL",
		// The trail, same statement, before and after from the row itself (rule 6 inv 3, 5, 6).
		"INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)",
		"'system', 'upload_policy.changed'",
		"FROM changed c;",
		"'migration " + tenantLogoBannerMigration + ":",
	} {
		if !strings.Contains(sql, clause) {
			t.Errorf("%s lacks %q", tenantLogoBannerMigration, clause)
		}
	}
	set := regexp.MustCompile(`SET max_bytes          = \d+,\s*allowed_mime_types = ARRAY\[([^\]]*)\],`).FindStringSubmatch(sql)
	if set == nil {
		t.Fatal("new tenant-logo MIME list not found")
	}
	var got []string
	for _, q := range strings.Split(set[1], ",") {
		got = append(got, strings.Trim(strings.TrimSpace(q), "'"))
	}
	if want := []string{storage.MIMEPNG, storage.MIMEWebP, storage.MIMEJPEG}; !slices.Equal(got, want) {
		t.Errorf("tenant-logo MIME list = %v, want %v (ADR 0069 #4; no HEIC)", got, want)
	}
}

const contentBodyImageMigration = "0018_upload_policy_content_body_image.sql"

// Chủ dự án chốt 03/10/2026: body images take the COVER's size and types, only the count differs. The
// size and MIME list are read from BOTH files — 0012's SET and 0018's seed — so a later edit of either
// value in one place turns this red instead of silently splitting the two image purposes.
func TestUploadPolicyContentBodyImageCopiesCover(t *testing.T) {
	cb, err := fs.ReadFile(migrations.FS, contentImageCoverMigration)
	if err != nil {
		t.Fatalf("read %s: %v", contentImageCoverMigration, err)
	}
	cover := regexp.MustCompile(`SET max_bytes\s+= (\d+),\s*allowed_mime_types = ARRAY\[([^\]]*)\],`).
		FindStringSubmatch(sqlLineComment.ReplaceAllString(string(cb), ""))
	if cover == nil {
		t.Fatalf("%s: cover values not found", contentImageCoverMigration)
	}
	var coverMimes []string
	for _, q := range strings.Split(cover[2], ",") {
		coverMimes = append(coverMimes, strings.Trim(strings.TrimSpace(q), "'"))
	}

	var body *seedRow
	for _, r := range allSeeds(t) {
		if r.purpose == "content-body-image" {
			found := r
			body = &found
		}
	}
	if body == nil {
		t.Fatal("content-body-image not seeded")
	}
	if body.file != contentBodyImageMigration {
		t.Errorf("content-body-image seeded in %s, want %s", body.file, contentBodyImageMigration)
	}
	if strconv.FormatInt(body.maxBytes, 10) != cover[1] || !slices.Equal(body.mimes, coverMimes) {
		t.Errorf("content-body-image = %d %v, cover (0012) = %s %v — the owner decided they are the same",
			body.maxBytes, body.mimes, cover[1], coverMimes)
	}
	if body.maxFiles != "20" {
		t.Errorf("content-body-image max_files_per_subject = %s, want 20 (owner, 03/10/2026)", body.maxFiles)
	}
}

// The read excludes soft-deleted rows — a withdrawn policy must read as not configured.
func TestListUploadPoliciesExcludesSoftDeleted(t *testing.T) {
	if !strings.Contains(listUploadPolicies, "WHERE deleted_at IS NULL") {
		t.Errorf("read does not exclude soft-deleted rows:\n%s", listUploadPolicies)
	}
}
