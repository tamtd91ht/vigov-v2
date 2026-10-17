package store

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The notice-audience reads (ADR 0099 #5) against a real PostgreSQL (harness in checker_pg_test.go).
// Skipped unless VIGOV_TEST_DSN is set — read the skip line before believing it ran.
//
// What each test pins lives in the SQL: the commune bound in every table of the statement, removed
// rows excluded, live memberships of live groups only, ANY of the tags, direct members of a live unit
// only, the keyset by id, and the reveal reads re-reporting is_active / locked NOW.

func refsOf(cs []domain.NoticeCandidate) string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Ref)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestPgNoticeCitizenAudience(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	ctx := ctxXa(xa)

	a := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Người A", Phone: "0900000001", IsActive: true, Tags: []string{"hộ nghèo"}})
	b := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Người B", Phone: "0900000002", IsActive: false, Tags: []string{"hộ nghèo"}})
	c := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Người C", Phone: "0900000003", IsActive: true, Tags: []string{"cựu chiến binh"}})
	gone := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Người D", Phone: "0900000004", IsActive: true, Tags: []string{"hộ nghèo"}})
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, gone, "CB-00001", "thử", time.Now())
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	// The other commune: same tag, same group name — must never be counted here (rule 1).
	other := citizenContactFixture(t, db, otherXa, domain.CitizenContact{FullName: "Xã khác", Phone: "0900000001", IsActive: true, Tags: []string{"hộ nghèo"}})
	otherGroup := citizenGroupFixture(t, db, otherXa, "Nhóm", 1)

	group := citizenGroupFixture(t, db, xa, "Nhóm", 1)
	for _, m := range []struct {
		xa, contact string
		groups      []string
	}{{xa, a, []string{group}}, {xa, b, []string{group}}, {otherXa, other, []string{otherGroup}}} {
		if err := inTx(t, db, m.xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			return s.SetGroups(ctx, tx, m.contact, m.groups, "CB-00001", time.Now())
		}); err != nil {
			t.Fatalf("SetGroups: %v", err)
		}
	}

	for name, tc := range map[string]struct {
		a              domain.NoticeAudience
		active, paused int
		page           string
	}{
		"all":                 {domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenAll}, 2, 1, sortedJoin(a, c)},
		"tag":                 {domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenTag, Tags: []string{"hộ nghèo"}}, 1, 1, a},
		"tags union":          {domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenTag, Tags: []string{"hộ nghèo", "cựu chiến binh"}}, 2, 1, sortedJoin(a, c)},
		"group":               {domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenGroup, GroupIDs: []string{group}}, 1, 1, a},
		"other commune group": {domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenGroup, GroupIDs: []string{otherGroup}}, 0, 0, ""},
	} {
		act, paused, err := s.NoticeAudienceCounts(ctx, tc.a)
		if err != nil || act != tc.active || paused != tc.paused {
			t.Errorf("%s: counts = %d/%d, %v — want %d/%d", name, act, paused, err, tc.active, tc.paused)
		}
		page, err := s.NoticeAudiencePage(ctx, tc.a, "", 100)
		if err != nil || refsOf(page) != tc.page {
			t.Errorf("%s: page = %s, %v — want %s", name, refsOf(page), err, tc.page)
		}
	}

	// KEYSET: limit 1 walks a then c by id, never twice.
	all := domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenAll}
	p1, _ := s.NoticeAudiencePage(ctx, all, "", 1)
	if len(p1) != 1 {
		t.Fatalf("page 1 = %d rows", len(p1))
	}
	p2, _ := s.NoticeAudiencePage(ctx, all, p1[0].Ref, 1)
	if len(p2) != 1 || p2[0].Ref <= p1[0].Ref {
		t.Fatalf("page 2 = %v after %s — want the next id", p2, p1[0].Ref)
	}

	// REVEAL: live rows of THIS commune only; is_active travels; the removed row and the other
	// commune's id are absent.
	var got []domain.NoticeCandidate
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.NoticeRevealCandidates(ctx, tx, []string{a, b, gone, other})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if refsOf(got) != sortedJoin(a, b) {
		t.Fatalf("reveal = %s, want %s", refsOf(got), sortedJoin(a, b))
	}
	for _, g := range got {
		if g.Ref == b && !g.Inactive || g.Ref == a && g.Inactive {
			t.Errorf("%s: Inactive = %v", g.Ref, g.Inactive)
		}
	}
}

func TestPgNoticeStaffAudience(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	s := NewCanBoStore(pkgstore.New(db))
	ctx := ctxXa(xa)

	themBoPhanPg(t, db, xa, "BP"+xa[:24], "bp-a", "", false)
	themBoPhanPg(t, db, xa, "BQ"+xa[:24], "bp-gone", "", true)
	themBoPhanPg(t, db, xa, "BS"+xa[:24], "bp-sub", "BP"+xa[:24], false)
	unit, goneUnit, subUnit := "BP"+xa[:24], "BQ"+xa[:24], "BS"+xa[:24]

	add := func(t *testing.T, x, code, unitID, mobile string, account, active, deleted bool) {
		t.Helper()
		var u any
		if unitID != "" {
			u = unitID
		}
		hash := ""
		if account {
			hash = bamMauPg
		}
		if _, err := db.Exec(`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, bo_phan_id, di_dong_ca_nhan,
			co_tai_khoan, mat_khau_hash, dang_hoat_dong, deleted_at)
			VALUES ($1,$2,$3,'Nguyễn Văn A',$4,$5,$6,$7,$8,$9, CASE WHEN $10 THEN now() END)`,
			x, "ND"+code+x[:16], code, strings.ToLower(code)+"@xa.gov.vn", u, mobile, account, hash, active, deleted); err != nil {
			t.Fatalf("thêm cán bộ %s: %v", code, err)
		}
	}
	add(t, xa, "CB-N0001", unit, "0900000001", true, true, false)     // account, active
	add(t, xa, "CB-N0002", unit, "0900000002", true, false, false)    // account, LOCKED
	add(t, xa, "CB-N0003", unit, "0900000003", false, false, false)   // directory-only: not locked
	add(t, xa, "CB-N0004", subUnit, "0900000004", true, true, false)  // sub-unit: not a direct member
	add(t, xa, "CB-N0005", goneUnit, "0900000005", true, true, false) // removed unit
	add(t, xa, "CB-N0006", unit, "0900000006", true, true, true)      // soft-deleted person
	add(t, otherXa, "CB-N0007", "", "0900000007", true, true, false)  // other commune

	cands, err := s.NoticeAudienceCandidates(ctx, domain.NoticeAudience{Kind: domain.NoticeAudienceStaffUnit, OrgUnitIDs: []string{unit, goneUnit}})
	if err != nil {
		t.Fatal(err)
	}
	if refsOf(cands) != "CB-N0001,CB-N0002,CB-N0003" {
		t.Fatalf("unit audience = %s, want the three direct live members of the live unit", refsOf(cands))
	}
	for _, c := range cands {
		if (c.Ref == "CB-N0002") != c.Inactive {
			t.Errorf("%s: Inactive = %v — locked means an account switched off, nothing else", c.Ref, c.Inactive)
		}
	}
	all, err := s.NoticeAudienceCandidates(ctx, domain.NoticeAudience{Kind: domain.NoticeAudienceStaffAll})
	if err != nil || refsOf(all) != "CB-N0001,CB-N0002,CB-N0003,CB-N0004,CB-N0005" {
		t.Errorf("staff all = %s, %v", refsOf(all), err)
	}

	err = inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		got, err := s.NoticeRevealCandidates(ctx, tx, []string{"CB-N0001", "CB-N0006", "CB-N0007"})
		if err != nil {
			return err
		}
		if refsOf(got) != "CB-N0001" {
			t.Errorf("reveal = %s, want CB-N0001 only (deleted and other commune absent)", refsOf(got))
		}
		for code, want := range map[string]bool{"CB-N0001": true, "CB-N0006": true, "CB-N0007": false, "": false} {
			ok, err := s.StaffCodeExists(ctx, tx, code)
			if err != nil || ok != want {
				t.Errorf("StaffCodeExists(%q) = %v, %v — want %v (soft-deleted counts, other commune does not)", code, ok, err, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func sortedJoin(refs ...string) string {
	sort.Strings(refs)
	return strings.Join(refs, ",")
}
