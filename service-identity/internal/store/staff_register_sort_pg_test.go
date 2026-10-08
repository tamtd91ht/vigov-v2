package store

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The staff register's new sorts, widened search and filtered count against a REAL PostgreSQL — the
// one place the derived relations' COALESCE sentinels, the department JOIN's commune binding, the
// KindRef lookup and the collation are checked by the engine that runs them. Skipped without
// VIGOV_TEST_DSN (harness in checker_pg_test.go). The fake-engine twin is staff_register_sort_test.go.
//
// The names are plain ASCII on purpose, so the expected order is the same under the C collation and
// any linguistic one — what is being tested is the keyset, not the collation.
//
//	      full_name    position     department        phone        last sign-in  active
//	p1    Anh Ba       Ke toan      PHONG B           0900000003   t0            yes
//	p2    Binh Chi     Chu tich     PHONG A           0900000001   never         NO
//	p3    Cuong Dung   Truong thon  (none)            0900000002   t0 − 1h       yes
//	p4    Dung Em      Van thu      PHONG C (deleted) 0900000004   never         yes
//	p5    Em Giang     (soft-deleted person)
//	B:    q1 in commune B's `d1` — the same id as A's PHONG A — named "PHONG Z"
func TestStaffRegisterSortSearchCountPg(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	t0 := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	seedDepartment := func(tenantID, id, name string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO bo_phan (tenant_id, id, ten, ma) VALUES ($1,$2,$3,$4)`,
			tenantID, id, name, "ma-"+id); err != nil {
			t.Fatalf("seed department: %v", err)
		}
	}
	seedDepartment(xaA, "d1", "PHONG A")
	seedDepartment(xaA, "d2", "PHONG B")
	seedDepartment(xaA, "d3", "PHONG C")
	seedDepartment(xaB, "d1", "PHONG Z")
	if _, err := db.Exec(`UPDATE bo_phan SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`, xaA, "d3"); err != nil {
		t.Fatalf("soft-delete department: %v", err)
	}

	seedPerson := func(tenantID, id, name, position string, department sql.NullString, phone string,
		signedIn *time.Time, active bool) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, chuc_vu, bo_phan_id, dien_thoai_co_quan, dang_hoat_dong, dang_nhap_gan_nhat)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			tenantID, id, "CB-"+id, name, id+"@example.gov.vn", position, department, phone, active, signedIn); err != nil {
			t.Fatalf("seed person %s: %v", id, err)
		}
	}
	dept := func(id string) sql.NullString { return sql.NullString{String: id, Valid: id != ""} }
	earlier := t0.Add(-time.Hour)
	seedPerson(xaA, "p1", "Anh Ba", "Ke toan", dept("d2"), "0900000003", &t0, true)
	seedPerson(xaA, "p2", "Binh Chi", "Chu tich", dept("d1"), "0900000001", nil, false)
	seedPerson(xaA, "p3", "Cuong Dung", "Truong thon", dept(""), "0900000002", &earlier, true)
	seedPerson(xaA, "p4", "Dung Em", "Van thu", dept("d3"), "0900000004", nil, true)
	seedPerson(xaA, "p5", "Em Giang", "Pho chu tich", dept("d1"), "0900000005", nil, true)
	seedPerson(xaB, "q1", "Giang Hoa", "Ke toan", dept("d1"), "0900000009", nil, true)
	if _, err := db.Exec(`UPDATE nguoi_dung SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`, xaA, "p5"); err != nil {
		t.Fatalf("soft-delete person: %v", err)
	}

	staff := NewCanBoStore(pkgstore.New(db))
	walk := func(tenantID string, loc domain.LocCanBo, query string) []string {
		t.Helper()
		var ids []string
		cursor := ""
		for i := 0; i < 20; i++ {
			q, err := url.ParseQuery(query)
			if err != nil {
				t.Fatal(err)
			}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			yc, err := page.Parse(q, SapXepCanBo)
			if err != nil {
				t.Fatalf("%s: %v", query, err)
			}
			res, err := staff.DanhSach(ctxXa(tenantID), loc, yc)
			if err != nil {
				t.Fatalf("%s: %v", query, err)
			}
			for _, cb := range res.Items {
				ids = append(ids, cb.ID)
			}
			if !res.HasMore {
				return ids
			}
			cursor = res.NextCursor
		}
		t.Fatalf("%s: the cursor does not advance", query)
		return nil
	}

	for query, want := range map[string]string{
		"sort=full_name&order=asc":      "p1,p2,p3,p4",
		"sort=full_name&order=desc":     "p4,p3,p2,p1",
		"sort=position&order=asc":       "p2,p1,p3,p4",
		"sort=position&order=desc":      "p4,p3,p1,p2",
		"sort=phone&order=asc":          "p2,p3,p1,p4",
		"sort=phone&order=desc":         "p4,p1,p3,p2",
		"sort=department&order=asc":     "p2,p1,p3,p4", // PHONG A, PHONG B, then none + deleted by id
		"sort=department&order=desc":    "p1,p2,p4,p3", // PHONG B, PHONG A, then none + deleted by id DESC
		"sort=last_login_at&order=asc":  "p3,p1,p2,p4",
		"sort=last_login_at&order=desc": "p1,p3,p4,p2",
		"sort=status&order=asc":         "p2,p1,p3,p4",
		"sort=status&order=desc":        "p4,p3,p1,p2",
	} {
		for _, limit := range []string{"1", "3", "100"} {
			if got := strings.Join(walk(xaA, domain.LocCanBo{}, query+"&limit="+limit), ","); got != want {
				t.Errorf("%s limit=%s: %s, want %s", query, limit, got, want)
			}
		}
	}

	search := func(tenantID, text string) string {
		t.Helper()
		return strings.Join(walk(tenantID, domain.LocCanBo{TuKhoa: text}, "limit=100"), ",")
	}
	for _, tc := range []struct{ tenant, text, want string }{
		{xaA, "p3@example", "p3"}, // email
		{xaA, "phong a", "p2"},    // live department name, case-insensitive
		{xaA, "PHONG C", ""},      // a soft-deleted department's name does not match
		{xaA, "PHONG Z", ""},      // commune B's name on the same department id
		{xaB, "PHONG Z", "q1"},    // ... found in its own commune
		{xaA, "Em Giang", ""},     // soft-deleted person
		{xaA, "PHONG", "p1,p2"},   // both live units, nothing else
		{xaA, "example.gov.vn", "p1,p2,p3,p4"},
	} {
		if got := search(tc.tenant, tc.text); got != tc.want {
			t.Errorf("search %q in %s: %q, want %q", tc.text, tc.tenant, got, tc.want)
		}
	}

	for _, tc := range []struct {
		tenant string
		loc    domain.LocCanBo
		want   int
	}{
		{xaA, domain.LocCanBo{}, 4},
		{xaA, domain.LocCanBo{TuKhoa: "PHONG"}, 2},
		{xaA, domain.LocCanBo{TuKhoa: "0900", BoPhanID: "d1"}, 1},
		{xaB, domain.LocCanBo{}, 1},
	} {
		got, err := staff.CountMatching(ctxXa(tc.tenant), tc.loc)
		if err != nil {
			t.Fatalf("CountMatching(%+v): %v", tc.loc, err)
		}
		if got != tc.want {
			t.Errorf("CountMatching(%s, %+v) = %d, want %d", tc.tenant, tc.loc, got, tc.want)
		}
	}
}
