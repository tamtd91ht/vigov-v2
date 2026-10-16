package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The citizen contact directory against a real PostgreSQL (harness in checker_pg_test.go). Skipped
// unless VIGOV_TEST_DSN is set — read the skip line before believing it ran.
//
// What each test pins lives in the SQL, where no fake can disagree: the keyset walk visits every row
// once with a tie-break; Count walks the list's own predicate; the fold finds "Nguyễn" from "nguyen"
// and a digit finds any part of the phone; a membership re-added is a SECOND row; the tag strip
// counts live rows; and the two invariants ADR 0097 §3 names for the Zalo column.

// directoryCounter makes every contact and group id of this suite distinct without the clock.
var directoryCounter atomic.Int64

// citizenContactFixture inserts one live contact through the store, in a committed transaction.
func citizenContactFixture(t *testing.T, db *sql.DB, xa string, c domain.CitizenContact) string {
	t.Helper()
	if c.ID == "" {
		c.ID = fmt.Sprintf("CT%024d", directoryCounter.Add(1)) // 26 characters — CHECK citizen_contact_id_ulid
	}
	if c.Source == "" {
		c.Source = domain.CitizenContactSourceManual
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	ctx := ctxXa(xa)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, c)
	}); err != nil {
		t.Fatalf("chèn người dân: %v", err)
	}
	return c.ID
}

// citizenGroupFixture inserts one live group through the store.
func citizenGroupFixture(t *testing.T, db *sql.DB, xa, name string, order int) string {
	t.Helper()
	id := fmt.Sprintf("CG%024d", directoryCounter.Add(1))
	ctx := ctxXa(xa)
	g := NewCitizenGroupStore(pkgstore.New(db))
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return g.Insert(ctx, tx, domain.CitizenGroup{ID: id, Name: name, SortOrder: order, CreatedAt: time.Now()})
	}); err != nil {
		t.Fatalf("chèn nhóm: %v", err)
	}
	return id
}

// identityPhoneFixture is a fake national number unique to this suite ("09…", rule 3 invariant 5)
// and the matching dinh_danh_cong_dan row holding its IDENTITY form ("84…" — the form
// crosstenant.DinhDanhStore.TimHoacTao stores, domain.CitizenPhoneIdentityForm). Returns the
// national number and the citizen id.
func identityPhoneFixture(t *testing.T, db *sql.DB) (national, citizenID string) {
	t.Helper()
	n := demCongDan.Add(1)
	national = fmt.Sprintf("09%08d", n)
	citizenID = fmt.Sprintf("CZ%024d", n)
	if _, err := db.Exec(`INSERT INTO dinh_danh_cong_dan (id, so_dien_thoai) VALUES ($1, $2)`,
		citizenID, domain.CitizenPhoneIdentityForm(national)); err != nil {
		t.Fatalf("thêm định danh công dân: %v", err)
	}
	return national, citizenID
}

func inTx(t *testing.T, db *sql.DB, xa string, fn func(ctx context.Context, tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	ctx := ctxXa(xa)
	return pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return fn(ctx, tx) })
}

func walkDirectory(t *testing.T, s *CitizenDirectoryStore, xa string, f CitizenContactFilter, limit int) []CitizenContactListItem {
	t.Helper()
	var all []CitizenContactListItem
	cursor := ""
	for pages := 0; pages < 50; pages++ {
		req, err := page.New(CitizenContactSorts, "", "", fmt.Sprint(limit), cursor)
		if err != nil {
			t.Fatalf("page.New: %v", err)
		}
		kq, err := s.List(ctxXa(xa), f, req)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		all = append(all, kq.Items...)
		if !kq.HasMore {
			if kq.NextCursor != "" {
				t.Error("has_more = false but next_cursor is set")
			}
			return all
		}
		if len(kq.Items) != limit {
			t.Errorf("a page with has_more must be full: %d rows, limit %d", len(kq.Items), limit)
		}
		cursor = kq.NextCursor
	}
	t.Fatal("the walk never ended — a cursor that repeats itself")
	return nil
}

// KEYSET: five rows, two sharing created_at exactly on a page boundary at limit 2; the walk hands
// out every row once, newest first, and never a row of the other commune. Count agrees with the
// walk under the same filter.
func TestPgCitizenDirectoryKeysetWalkIsTotalAndCountMatches(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	base := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)

	ids := map[string]bool{}
	for i, c := range []struct {
		name  string
		phone string
		at    time.Time
	}{
		{"Nguyễn Văn A", "0900000001", base.Add(1 * time.Minute)},
		{"Trần Thị B", "0900000002", base.Add(2 * time.Minute)},
		{"Lê Văn C", "0900000003", base.Add(2 * time.Minute)}, // same instant as B — the tie-break
		{"Phạm Thị D", "0900000004", base.Add(3 * time.Minute)},
		{"Hoàng Văn E", "0900000005", base.Add(4 * time.Minute)},
	} {
		id := citizenContactFixture(t, db, xa, domain.CitizenContact{
			FullName: c.name, Phone: c.phone, CreatedAt: c.at, IsActive: i != 4, Tags: []string{"nhãn chung"},
		})
		ids[id] = true
	}
	citizenContactFixture(t, db, otherXa, domain.CitizenContact{FullName: "Xã Khác", Phone: "0900000001", IsActive: true})

	all := walkDirectory(t, s, xa, CitizenContactFilter{}, 2)
	if len(all) != 5 {
		t.Fatalf("walked %d rows, want 5", len(all))
	}
	seen := map[string]bool{}
	for i, r := range all {
		if !ids[r.ID] {
			t.Fatalf("row %s is not of this commune — rò giữa hai xã (luật 1)", r.ID)
		}
		if seen[r.ID] {
			t.Fatalf("row %s handed out twice — the keyset is not a total order", r.ID)
		}
		seen[r.ID] = true
		if i > 0 && all[i-1].CreatedAt.Before(r.CreatedAt) {
			t.Errorf("row %d is newer than row %d — newest first", i, i-1)
		}
		if r.Tags == nil || r.GroupIDs == nil {
			t.Errorf("%s: Tags / GroupIDs must be non-nil slices", r.ID)
		}
	}
	if all[0].FullName != "Hoàng Văn E" {
		t.Errorf("first row = %q, want the newest", all[0].FullName)
	}

	n, err := s.Count(ctxXa(xa), CitizenContactFilter{})
	if err != nil || n != 5 {
		t.Errorf("Count = %d, %v — want 5", n, err)
	}
	active := CitizenContactFilter{ActiveOnly: true}
	if got := walkDirectory(t, s, xa, active, 2); len(got) != 4 {
		t.Errorf("ActiveOnly walk = %d rows, want 4", len(got))
	}
	if n, err := s.Count(ctxXa(xa), active); err != nil || n != 4 {
		t.Errorf("Count(ActiveOnly) = %d, %v — want 4, the list's own predicate", n, err)
	}
	if n, err := s.Count(ctxXa(otherXa), CitizenContactFilter{}); err != nil || n != 1 {
		t.Errorf("Count in the other commune = %d, %v — want 1", n, err)
	}
}

// KEYWORD: the fold finds "Nguyễn" from "nguyen" and "NGUYỄN"; the address is folded too; a term
// with a digit matches ANY part of the phone (ADR 0097 D11); a term without one never touches it.
func TestPgCitizenDirectoryKeywordFoldsNameAndAddressAndMatchesPartOfPhone(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	a := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Nguyễn Văn A", Phone: "0900000011", Address: "Số 1 đường Lê Lợi", IsActive: true})
	b := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "Trần Thị B", Phone: "0911000022", Address: "Thôn Bình An", IsActive: true})

	for term, want := range map[string][]string{
		"nguyen":   {a},
		"NGUYỄN":   {a},
		"le loi":   {a},
		"binh an":  {b},
		"0000":     {a, b}, // any part of the number: both hold "0000"
		"1100":     {b},
		"văn 0900": {a}, // name OR phone: "văn" and "0900" both point at A
		"không ai": nil,
	} {
		f := CitizenContactFilter{Keyword: term}
		got := walkDirectory(t, s, xa, f, 10)
		gotIDs := map[string]bool{}
		for _, r := range got {
			gotIDs[r.ID] = true
		}
		if len(got) != len(want) {
			t.Errorf("%q: %d rows, want %d", term, len(got), len(want))
		}
		for _, id := range want {
			if !gotIDs[id] {
				t.Errorf("%q: missing %s", term, id)
			}
		}
		if n, err := s.Count(ctxXa(xa), f); err != nil || n != len(want) {
			t.Errorf("%q: Count = %d, %v — want %d", term, n, err, len(want))
		}
	}
}

// GET / READ PHONE / UPDATE / SOFT DELETE / PHONE TAKEN, and the one-answer rule for another
// commune's id.
func TestPgCitizenDirectoryRowLifecycle(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	themThonToDanPho(t, db, xa, "tt-"+xa[:20], "thon-1", "Thôn 1", "")
	unit := "tt-" + xa[:20]

	id := citizenContactFixture(t, db, xa, domain.CitizenContact{
		FullName: "Nguyễn Văn A", Phone: "0900000031", ResidentialUnitID: &unit,
		Address: "Số 1", Note: "ghi chú", Tags: []string{"hộ nghèo"}, IsActive: true,
	})

	got, found, err := s.Get(ctxXa(xa), id)
	if err != nil || !found {
		t.Fatalf("Get: %v, found %v", err, found)
	}
	if got.Address != "Số 1" || got.Note != "ghi chú" || got.ResidentialUnitID == nil || *got.ResidentialUnitID != unit ||
		len(got.Tags) != 1 || got.Source != domain.CitizenContactSourceManual || got.HouseholdCode != nil || got.ImportBatchID != nil {
		t.Errorf("Get round-trip = %+v", got)
	}
	if _, found, err := s.Get(ctxXa(otherXa), id); err != nil || found {
		t.Errorf("another commune's Get: found %v, %v — must be not found, never 403", found, err)
	}

	// The page carries the unit's NAME and no address or note.
	rows := walkDirectory(t, s, xa, CitizenContactFilter{ResidentialUnitID: unit}, 10)
	if len(rows) != 1 || rows[0].ResidentialUnitName != "Thôn 1" {
		t.Errorf("page by unit = %+v, want one row named Thôn 1", rows)
	}

	// Full number only inside a transaction (the caller's audit entry goes beside it).
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		phone, found, err := s.ReadPhone(ctx, tx, id)
		if err != nil || !found || phone != "0900000031" {
			t.Errorf("ReadPhone = %q, %v, %v", phone, found, err)
		}
		if _, found, err := s.ReadPhone(ctx, tx, "CT00000000000000000000NONE"); err != nil || found {
			t.Errorf("ReadPhone unknown id: found %v, %v", found, err)
		}
		taken, err := s.PhoneTaken(ctx, tx, "0900000031", "")
		if err != nil || !taken {
			t.Errorf("PhoneTaken = %v, %v — want true", taken, err)
		}
		taken, err = s.PhoneTaken(ctx, tx, "0900000031", id)
		if err != nil || taken {
			t.Errorf("PhoneTaken excluding the row itself = %v, %v — want false", taken, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := inTx(t, db, otherXa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		_, found, err := s.ReadPhone(ctx, tx, id)
		if err != nil || found {
			t.Errorf("another commune's ReadPhone: found %v, %v", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A second live row with the same number is refused by the key, with the sentinel.
	err = inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.CitizenContact{ID: fmt.Sprintf("CT%024d", directoryCounter.Add(1)),
			FullName: "Trùng số", Phone: "0900000031", Source: domain.CitizenContactSourceManual, CreatedAt: time.Now()})
	})
	if !errors.Is(err, ErrCitizenPhoneTaken) {
		t.Errorf("duplicate number: %v, want ErrCitizenPhoneTaken", err)
	}
	// Another commune's unit is refused by the composite foreign key.
	themThonToDanPho(t, db, otherXa, "tt-"+otherXa[:20], "thon-x", "Thôn X", "")
	foreign := "tt-" + otherXa[:20]
	err = inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.CitizenContact{ID: fmt.Sprintf("CT%024d", directoryCounter.Add(1)),
			FullName: "Thôn lạ", Phone: "0900000032", ResidentialUnitID: &foreign,
			Source: domain.CitizenContactSourceManual, CreatedAt: time.Now()})
	})
	if !errors.Is(err, ErrCitizenContactUnitUnknown) {
		t.Errorf("other commune's unit: %v, want ErrCitizenContactUnitUnknown", err)
	}

	// Update rewrites the editable columns; source stays.
	got.FullName = "Nguyễn Văn A (sửa)"
	got.Phone = "0900000033"
	got.Address = ""
	got.Tags = []string{}
	got.IsActive = false
	got.UpdatedAt = time.Now()
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error { return s.Update(ctx, tx, got) }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _, _ := s.Get(ctxXa(xa), id)
	if after.FullName != "Nguyễn Văn A (sửa)" || after.Phone != "0900000033" || after.Address != "" || len(after.Tags) != 0 || after.IsActive {
		t.Errorf("after Update = %+v", after)
	}

	// Soft delete: gone from every read, a second removal is not-found, the row is kept.
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, id, "CB-00001", "Chuyển đi nơi khác", time.Now())
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, found, _ := s.Get(ctxXa(xa), id); found {
		t.Error("a removed row is still returned by Get")
	}
	if n, _ := s.Count(ctxXa(xa), CitizenContactFilter{}); n != 0 {
		t.Errorf("Count after removal = %d, want 0", n)
	}
	err = inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, id, "CB-00001", "lần hai", time.Now())
	})
	if !errors.Is(err, ErrCitizenContactNotFound) {
		t.Errorf("second removal: %v, want ErrCitizenContactNotFound", err)
	}
	var kept int
	if err := db.QueryRow(`SELECT count(*) FROM citizen_contact WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL AND delete_reason = 'Chuyển đi nơi khác'`, xa, id).Scan(&kept); err != nil || kept != 1 {
		t.Errorf("removed row kept with its reason: %d, %v", kept, err)
	}
	// The number is free again for a new live row (migration 0033: a phone is not an issued code).
	batch := fmt.Sprintf("IB%024d", directoryCounter.Add(1))
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.CitizenContact{ID: fmt.Sprintf("CT%024d", directoryCounter.Add(1)),
			FullName: "Quay lại", Phone: "0900000033", Source: domain.CitizenContactSourceExcel,
			ImportBatchID: &batch, CreatedAt: time.Now()})
	}); err != nil {
		t.Errorf("re-adding a removed number: %v", err)
	}
}

// MEMBERSHIP: removed and added back is a SECOND row (0031 shape); GroupIDsOf drops a removed
// group; MemberCounts counts live contacts; the group filter walks the membership.
func TestPgCitizenDirectoryMembershipReAddIsANewRow(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	g := NewCitizenGroupStore(pkgstore.New(db))
	g1 := citizenGroupFixture(t, db, xa, "Hộ kinh doanh", 1)
	g2 := citizenGroupFixture(t, db, xa, "Tổ trưởng dân phố", 2)
	c1 := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "A", Phone: "0900000041", IsActive: true})
	c2 := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "B", Phone: "0900000042", IsActive: true})

	set := func(contact string, groups ...string) {
		t.Helper()
		if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
			return s.SetGroups(ctx, tx, contact, groups, "CB-00001", time.Now())
		}); err != nil {
			t.Fatalf("SetGroups(%v): %v", groups, err)
		}
	}
	set(c1, g1, g2, g1) // the repeat is dropped
	set(c2, g1)
	set(c1, g2)     // g1 removed
	set(c1, g2, g1) // g1 added back — a new row

	var total, live int
	if err := db.QueryRow(`SELECT count(*), count(*) FILTER (WHERE deleted_at IS NULL) FROM citizen_contact_group
		WHERE tenant_id = $1 AND contact_id = $2 AND group_id = $3`, xa, c1, g1).Scan(&total, &live); err != nil {
		t.Fatal(err)
	}
	if total != 2 || live != 1 {
		t.Errorf("(c1, g1) rows = %d total, %d live — want 2 and 1: a re-add is a new row, never an edit", total, live)
	}
	var reason string
	if err := db.QueryRow(`SELECT delete_reason FROM citizen_contact_group WHERE tenant_id = $1 AND contact_id = $2 AND group_id = $3 AND deleted_at IS NOT NULL`,
		xa, c1, g1).Scan(&reason); err != nil || reason != CitizenGroupMembershipRemovedReason {
		t.Errorf("removed row reason = %q, %v", reason, err)
	}

	ids, err := s.GroupIDsOf(ctxXa(xa), []string{c1, c2, "CT00000000000000000000NONE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids[c1]) != 2 || ids[c1][0] != g2 || ids[c1][1] != g1 || len(ids[c2]) != 1 || ids[c2][0] != g1 || len(ids) != 2 {
		t.Errorf("GroupIDsOf = %v — want c1:[g2 g1] (membership order), c2:[g1], unknown absent", ids)
	}
	counts, err := s.MemberCounts(ctxXa(xa))
	if err != nil || counts[g1] != 2 || counts[g2] != 1 {
		t.Errorf("MemberCounts = %v, %v — want g1:2 g2:1", counts, err)
	}
	if rows := walkDirectory(t, s, xa, CitizenContactFilter{GroupID: g1}, 10); len(rows) != 2 {
		t.Errorf("group filter = %d rows, want 2", len(rows))
	}
	if n, _ := s.Count(ctxXa(xa), CitizenContactFilter{GroupID: g2}); n != 1 {
		t.Errorf("Count(GroupID g2) = %d, want 1", n)
	}

	// A removed group names nobody; a removed contact is in no count.
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return g.SoftDelete(ctx, tx, g2, "CB-00001", "", time.Now())
	}); err != nil {
		t.Fatalf("group SoftDelete with no reason: %v", err)
	}
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, c2, "CB-00001", "lý do", time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	ids, _ = s.GroupIDsOf(ctxXa(xa), []string{c1})
	if len(ids[c1]) != 1 || ids[c1][0] != g1 {
		t.Errorf("after removing g2: GroupIDsOf(c1) = %v, want [g1]", ids[c1])
	}
	counts, _ = s.MemberCounts(ctxXa(xa))
	if counts[g1] != 1 || counts[g2] != 1 {
		t.Errorf("MemberCounts after removals = %v — g1 loses the removed contact; g2's rows are kept", counts)
	}
	// Another commune's group is refused by the key.
	_, otherXa := xaRieng(t)
	foreign := citizenGroupFixture(t, db, otherXa, "Nhóm xã khác", 1)
	err = inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SetGroups(ctx, tx, c1, []string{foreign}, "CB-00001", time.Now())
	})
	if !errors.Is(err, ErrCitizenGroupNotFound) {
		t.Errorf("other commune's group: %v, want ErrCitizenGroupNotFound", err)
	}
}

func TestPgCitizenDirectoryTagCounts(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "A", Phone: "0900000051", Tags: []string{"hộ nghèo", "cựu chiến binh"}, IsActive: true})
	citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "B", Phone: "0900000052", Tags: []string{"hộ nghèo"}, IsActive: false})
	gone := citizenContactFixture(t, db, xa, domain.CitizenContact{FullName: "C", Phone: "0900000053", Tags: []string{"hộ nghèo", "đã gỡ"}, IsActive: true})
	citizenContactFixture(t, db, otherXa, domain.CitizenContact{FullName: "X", Phone: "0900000051", Tags: []string{"xã khác"}, IsActive: true})
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, gone, "CB-00001", "lý do", time.Now())
	}); err != nil {
		t.Fatal(err)
	}

	got, err := s.TagCounts(ctxXa(xa))
	if err != nil {
		t.Fatal(err)
	}
	want := []TagCount{{"hộ nghèo", 2}, {"cựu chiến binh", 1}}
	if len(got) != len(want) {
		t.Fatalf("TagCounts = %v, want %v (removed row and other commune excluded, inactive counted)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("TagCounts[%d] = %v, want %v (by count desc, then tag)", i, got[i], want[i])
		}
	}
}

// ADR 0097 §3 — THE TWO MANDATORY TESTS, and the rest of the invariant: a number in commune A's
// directory with a LIVE session in commune B reads "—" in A; a REVOKED or EXPIRED session in A reads
// "—"; a live session in A reads "Có"; the batch answers exactly the phones asked, a platform-unknown
// number as false; the page's column and the Zalo filter agree with the batch.
func TestPgOwnAppSessionIsPerCommuneAndLiveOnly(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	s := NewCitizenDirectoryStore(pkgstore.New(db))
	sessions := soPhienCongDan(db)

	phoneB, citizenB := identityPhoneFixture(t, db) // live session in B only
	phoneRevoked, citizenRevoked := identityPhoneFixture(t, db)
	phoneExpired, citizenExpired := identityPhoneFixture(t, db)
	phoneLive, citizenLive := identityPhoneFixture(t, db)
	phoneNobody := "0900000099" // in A's directory, never signed in anywhere

	for _, p := range []string{phoneB, phoneRevoked, phoneExpired, phoneLive, phoneNobody} {
		citizenContactFixture(t, db, xaA, domain.CitizenContact{FullName: "Người dân", Phone: p, IsActive: true})
	}

	moPhienCongDan(t, db, xaB, citizenB)
	revoked, _ := moPhienCongDan(t, db, xaA, citizenRevoked)
	if err := inTx(t, db, xaA, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return sessions.ThuHoi(ctx, tx, revoked, "công dân đăng xuất")
	}); err != nil {
		t.Fatal(err)
	}
	expired, _ := moPhienCongDan(t, db, xaA, citizenExpired)
	// Both columns moved back — CHECK (het_han_luc > tao_luc).
	if _, err := db.Exec(`UPDATE phien_cong_dan SET tao_luc = now() - interval '2 hours',
		het_han_luc = now() - interval '1 minute' WHERE tenant_id = $1 AND id = $2`, xaA, expired); err != nil {
		t.Fatal(err)
	}
	moPhienCongDan(t, db, xaA, citizenLive)

	ask := []string{
		domain.CitizenPhoneIdentityForm(phoneB), domain.CitizenPhoneIdentityForm(phoneRevoked),
		domain.CitizenPhoneIdentityForm(phoneExpired), domain.CitizenPhoneIdentityForm(phoneLive),
		domain.CitizenPhoneIdentityForm(phoneNobody), "", domain.CitizenPhoneIdentityForm(phoneLive),
	}
	var got map[string]bool
	if err := inTx(t, db, xaA, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.OwnAppSessionByPhone(ctx, tx, ask)
		return err
	}); err != nil {
		t.Fatalf("OwnAppSessionByPhone: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("map holds %d phones, want exactly the 5 distinct non-empty ones asked: %v", len(got), got)
	}
	for name, c := range map[string]struct {
		phone string
		want  bool
	}{
		"live session in commune B only (§3 test 1)": {phoneB, false},
		"revoked session in A (§3 test 2)":           {phoneRevoked, false},
		"expired session in A (§3 test 2)":           {phoneExpired, false},
		"never signed in — unknown to the platform":  {phoneNobody, false},
		"live session in A":                          {phoneLive, true},
	} {
		key := domain.CitizenPhoneIdentityForm(c.phone)
		if v, ok := got[key]; !ok || v != c.want {
			t.Errorf("%s: got %v (present %v), want %v", name, v, ok, c.want)
		}
	}

	// The empty ask makes no statement and answers an empty map.
	if err := inTx(t, db, xaA, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		m, err := s.OwnAppSessionByPhone(ctx, tx, []string{"", ""})
		if err != nil || len(m) != 0 {
			t.Errorf("empty ask = %v, %v", m, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The page's column agrees with the batch, row by row — and commune B, which holds the only live
	// session of phoneB, has no directory row at all, so it sees nothing.
	rows := walkDirectory(t, s, xaA, CitizenContactFilter{}, 10)
	if len(rows) != 5 {
		t.Fatalf("A's directory = %d rows, want 5", len(rows))
	}
	for _, r := range rows {
		if want := got[domain.CitizenPhoneIdentityForm(r.Phone)]; r.HasOwnAppSession != want {
			t.Errorf("page column for a row = %v, batch says %v", r.HasOwnAppSession, want)
		}
	}
	if only := walkDirectory(t, s, xaA, CitizenContactFilter{HasOwnAppSession: boolPtr(true)}, 10); len(only) != 1 || only[0].Phone != phoneLive {
		t.Errorf("filter Zalo = true → %d rows, want the one live-in-A number", len(only))
	}
	if n, _ := s.Count(ctxXa(xaA), CitizenContactFilter{HasOwnAppSession: boolPtr(false)}); n != 4 {
		t.Errorf("Count(Zalo = false) = %d, want 4", n)
	}
	if rowsB := walkDirectory(t, s, xaB, CitizenContactFilter{HasOwnAppSession: boolPtr(true)}, 10); len(rowsB) != 0 {
		t.Errorf("commune B lists %d rows — it has no directory row; nothing of A's may appear", len(rowsB))
	}
}

// GROUPS: live list in rank order; the folded name is unique and resolvable from any spelling; a
// removed group's name may be used again.
func TestPgCitizenGroupsFoldedNameAndOrder(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	g := NewCitizenGroupStore(pkgstore.New(db))
	second := citizenGroupFixture(t, db, xa, "Hộ nghèo", 2)
	first := citizenGroupFixture(t, db, xa, "Tổ trưởng dân phố", 1)
	citizenGroupFixture(t, db, otherXa, "Hộ nghèo", 1)

	list, err := g.List(ctxXa(xa))
	if err != nil || len(list) != 2 || list[0].ID != first || list[1].ID != second {
		t.Fatalf("List = %+v, %v — want [first second] by sort_order, this commune only", list, err)
	}
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		for name, want := range map[string]bool{"ho ngheo": true, "HỘ NGHÈO": true, "Hộ cận nghèo": false, "": false} {
			taken, err := g.NameTaken(ctx, tx, name, "")
			if err != nil || taken != want {
				t.Errorf("NameTaken(%q) = %v, %v — want %v", name, taken, err, want)
			}
		}
		if taken, _ := g.NameTaken(ctx, tx, "Hộ nghèo", second); taken {
			t.Error("NameTaken must exclude the row being edited")
		}
		err := g.Insert(ctx, tx, domain.CitizenGroup{ID: fmt.Sprintf("CG%024d", directoryCounter.Add(1)), Name: "HO NGHEO", CreatedAt: time.Now()})
		if !errors.Is(err, ErrCitizenGroupNameTaken) {
			t.Errorf("duplicate folded name: %v, want ErrCitizenGroupNameTaken", err)
		}
		return errors.New("rollback the failed insert")
	}); err == nil {
		t.Fatal("expected the deliberate rollback")
	}

	ids, err := g.IDsByNames(ctxXa(xa), []string{"ho ngheo", "Tổ trưởng dân phố", "Không có", "", "ho ngheo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids["ho ngheo"] != second || ids["Tổ trưởng dân phố"] != first {
		t.Errorf("IDsByNames = %v — keyed by the name as asked, unknown absent", ids)
	}
	if _, found, _ := g.Get(ctxXa(otherXa), first); found {
		t.Error("another commune's Get found the group")
	}

	// Update, then remove, then the name is free again.
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return g.Update(ctx, tx, domain.CitizenGroup{ID: second, Name: "Hộ nghèo 2026", Description: "mô tả", SortOrder: 3, UpdatedAt: time.Now()})
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, found, _ := g.Get(ctxXa(xa), second); !found || got.Name != "Hộ nghèo 2026" || got.Description != "mô tả" || got.SortOrder != 3 {
		t.Errorf("after Update = %+v", got)
	}
	if err := inTx(t, db, xa, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return g.SoftDelete(ctx, tx, second, "CB-00001", "  ", time.Now())
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, found, _ := g.Get(ctxXa(xa), second); found {
		t.Error("removed group still found")
	}
	citizenGroupFixture(t, db, xa, "Hộ nghèo 2026", 5) // the name is free again (live_key)
	if list, _ := g.List(ctxXa(xa)); len(list) != 2 {
		t.Errorf("List after remove + re-create = %d, want 2", len(list))
	}
}
