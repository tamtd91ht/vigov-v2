package store

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The reads of petition_merge.go and the duplicate thresholds, on driver_gia_test.go's fake driver.
//
//	PROVED HERE   the link columns are scanned by NAME (a merged row reads back its main, when and who) ·
//	              the detail's link read is ONE statement scoped to $1, live rows only, both halves told
//	              apart · the duplicate search binds every value, carries the partial index's predicate,
//	              the four open statuses, NEVER `can-bo`, the field only when classified, and is bounded ·
//	              "no settings row" answers 50 m / 7 days, a stored row is read as stored, two rows refuse.
//	NOT PROVED    the writes (they take a transaction — internal/app's fake driver proves their SQL and
//	              their order) and anything PostgreSQL does — the trigger and the CHECKs of migration 0037
//	              are the floor, checked as text in migrations/petition_merge_test.go.

var mergedAtSample = time.Date(2026, 10, 9, 3, 4, 5, 0, time.UTC)

func TestScanReadsTheMergeLink(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
		"merged_into": "01JMAINPETITION", "merged_at": mergedAtSample, "merged_by": "CB-00123"})}}
	p, err := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k))).TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatal(err)
	}
	if p.MergedInto != "01JMAINPETITION" || !p.MergedAt.Equal(mergedAtSample) || p.MergedBy != "CB-00123" {
		t.Errorf("link = %q %v %q", p.MergedInto, p.MergedAt, p.MergedBy)
	}
}

func TestMergeLinksOneScopedStatement(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		{"ma_tra_cuu": "PA-MAIN", "id = $3": true},
		{"ma_tra_cuu": "PA-CHILD-1", "id = $3": false},
		{"ma_tra_cuu": "PA-CHILD-2", "id = $3": false},
	}}
	s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
	links, err := s.MergeLinks(ctxXa(xaThu), domain.PhieuPhanAnh{ID: "01JSELF", MergedInto: "01JMAIN"})
	if err != nil {
		t.Fatal(err)
	}
	if links.MainCode != "PA-MAIN" || strings.Join(links.ChildCodes, ",") != "PA-CHILD-1,PA-CHILD-2" {
		t.Errorf("links = %+v", links)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("%d statements, want ONE", len(k.lenh))
	}
	l := k.lenh[0]
	for _, frag := range []string{"FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL",
		"(merged_into = $2 OR id = $3)", "LIMIT $4"} {
		if !strings.Contains(l.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, l.sql)
		}
	}
	if l.args[0] != string(xaThu) || l.args[1] != "01JSELF" || l.args[2] != "01JMAIN" {
		t.Errorf("args = %v", l.args)
	}
}

func TestMergeLinksRefusesPastTheCeiling(t *testing.T) {
	rows := make([]map[string]driver.Value, 0, MergedPetitionsCeiling+1)
	for i := 0; i <= MergedPetitionsCeiling; i++ {
		rows = append(rows, map[string]driver.Value{"ma_tra_cuu": "PA-X", "id = $3": false})
	}
	_, err := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(&khoGia{hangTheoCot: rows}))).
		MergeLinks(ctxXa(xaThu), domain.PhieuPhanAnh{ID: "01JSELF"})
	if !errors.Is(err, ErrTooManyMergedPetitions) {
		t.Errorf("err = %v — a list with petitions silently missing", err)
	}
}

func TestDuplicateCandidatesStatement(t *testing.T) {
	reported := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	q := DuplicateQuery{ExcludeID: "01JSELF", Box: domain.GeoBox{MinLat: 15.1, MaxLat: 15.2, MinLng: 108.1, MaxLng: 108.2},
		ReportedAt: reported, WindowDays: 7, Field: "rac-thai", Limit: 100}
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	got, err := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k))).DuplicateCandidates(ctxXa(xaThu), q)
	if err != nil || len(got) != 1 {
		t.Fatalf("= %d, %v", len(got), err)
	}
	l := k.lenh[0]
	for _, frag := range []string{
		"FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL AND merged_into IS NULL" +
			" AND lat IS NOT NULL AND lng IS NOT NULL",
		"id <> $2",
		"goc_dem_han >= $3::timestamptz - make_interval(days => $4::int)",
		"goc_dem_han <= $3::timestamptz + make_interval(days => $4::int)",
		"lat BETWEEN $5 AND $6 AND lng BETWEEN $7 AND $8",
		"trang_thai IN ('da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly')",
		"(linh_vuc IS NULL OR linh_vuc <> 'can-bo')",
		"(linh_vuc IS NULL OR linh_vuc = $9)",
		"LIMIT $10",
	} {
		if !strings.Contains(l.sql, frag) {
			t.Errorf("statement lacks %q:\n%s", frag, l.sql)
		}
	}
	want := []driver.Value{string(xaThu), "01JSELF", reported, int64(7), 15.1, 15.2, 108.1, 108.2, "rac-thai", int64(100)}
	if len(l.args) != len(want) {
		t.Fatalf("args = %v", l.args)
	}
	for i := range want {
		if l.args[i] != want[i] {
			t.Errorf("arg %d = %v, want %v", i, l.args[i], want[i])
		}
	}
	for _, leak := range []string{"rac-thai", "15.1", "01JSELF"} {
		if strings.Contains(l.sql, leak) {
			t.Errorf("%q concatenated into the statement", leak)
		}
	}

	// Unclassified: no field predicate at all — any field may be the same incident.
	k = &khoGia{}
	q.Field = ""
	if _, err := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k))).DuplicateCandidates(ctxXa(xaThu), q); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(k.lenh[0].sql, "linh_vuc = $") || !strings.Contains(k.lenh[0].sql, "LIMIT $9") {
		t.Errorf("unclassified statement: %s", k.lenh[0].sql)
	}
}

func TestDuplicateCandidatesRefusesUnboundedBeforeSQL(t *testing.T) {
	k := &khoGia{}
	s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
	for _, q := range []DuplicateQuery{{Limit: 0, WindowDays: 7}, {Limit: 10, WindowDays: 0}} {
		if _, err := s.DuplicateCandidates(ctxXa(xaThu), q); err == nil {
			t.Errorf("%+v accepted", q)
		}
	}
	if len(k.lenh) != 0 {
		t.Error("ran SQL for a refused query")
	}
}

func TestDuplicateThresholds(t *testing.T) {
	k := &khoGia{}
	got, err := newPetitionSettingsStore(k).DuplicateThresholds(ctxXa(xaThu))
	if err != nil || got != (DuplicateThresholds{RadiusMeters: 50, WindowDays: 7}) {
		t.Errorf("no row = %+v, %v — want the decided defaults 50 m / 7 days (migration 0038)", got, err)
	}
	if l := k.lenh[0]; !strings.Contains(l.sql, " FROM petition_settings WHERE tenant_id = $1 ") || len(l.args) != 1 {
		t.Errorf("statement = %q %v", l.sql, l.args)
	}

	row := map[string]driver.Value{"duplicate_radius_meters": int64(120), "duplicate_window_days": int64(3)}
	k = &khoGia{hangTheoCot: []map[string]driver.Value{row}}
	if got, err := newPetitionSettingsStore(k).DuplicateThresholds(ctxXa(xaThu)); err != nil ||
		got != (DuplicateThresholds{RadiusMeters: 120, WindowDays: 3}) {
		t.Errorf("stored = %+v, %v", got, err)
	}

	k = &khoGia{hangTheoCot: []map[string]driver.Value{row, row}}
	if _, err := newPetitionSettingsStore(k).DuplicateThresholds(ctxXa(xaThu)); !errors.Is(err, ErrPetitionSettingsDuplicate) {
		t.Errorf("two rows: %v", err)
	}
	cause := errors.New("mất kết nối")
	if _, err := newPetitionSettingsStore(&khoGia{loi: cause}).DuplicateThresholds(ctxXa(xaThu)); !errors.Is(err, cause) {
		t.Errorf("driver failure answered with a default: %v", err)
	}
}
