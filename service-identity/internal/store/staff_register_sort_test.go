package store

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The six screen columns of the staff register as sorts (owner decision 08/10/2026), the search
// widened to email and department name, and the filtered count — against the SAME fake engine as
// can_bo_danh_sach_test.go, which executes the keyset rather than recording it.
//
// What the fake proves: every page boundary of every new sort, in both directions, visits each person
// exactly once and in the stated order; the cursor never carries a personal value. What it does NOT
// prove — the SQL of the derived relations (COALESCE sentinels, the department JOIN, the collation) —
// is proven against PostgreSQL in staff_register_sort_pg_test.go.
//
// THE ORDERS BELOW ARE THE FAKE'S BYTE ORDER (strings.Compare): "Đỗ" sorts after "Trần" here because
// "Đ" is a two-byte rune. PostgreSQL orders names by the database collation; the pg suite uses names
// whose order is the same under both.
//
// dsDuLieu, as these cases read it (commune A, live rows only):
//
//	       full_name       position            department (live)   phone        last sign-in  active
//	nd-01  Nguyễn Văn A    Chủ tịch UBND xã    Văn phòng UBND      0900000001   dsMoc         yes
//	nd-02  Trần Thị B      Trưởng thôn         Bộ phận Một cửa     0900000002   never         yes
//	nd-03  Lê Văn C        Kế toán             Văn phòng UBND      0900000003   dsMoc − 1h    NO
//	nd-05  Đỗ Văn E        ""                  (bp-003, deleted)   0900000005   never         yes

func TestStaffSortWalksEveryNewSortInOrder(t *testing.T) {
	// MUTATIONS THAT MUST TURN THIS RED:
	//   - read the department/last_login/status anchor from the domain row instead of the scanned key
	//     (the sentinel rows then anchor wrong and page two repeats or loses them);
	//   - swap the two department prefixes, or the two last-login sentinels, in staffSortRelation
	//     (the fake models the intended order, the pg suite the SQL);
	//   - declare full_name/position/phone as KindText (the cursor then carries the value — see the
	//     next case).
	b := moBanThuDS(t)
	for sortOrder, want := range map[string][]string{
		"sort=full_name&order=asc":      {"nd-03", "nd-01", "nd-02", "nd-05"},
		"sort=full_name&order=desc":     {"nd-05", "nd-02", "nd-01", "nd-03"},
		"sort=position&order=asc":       {"nd-05", "nd-01", "nd-03", "nd-02"},
		"sort=position&order=desc":      {"nd-02", "nd-03", "nd-01", "nd-05"},
		"sort=phone&order=asc":          {"nd-01", "nd-02", "nd-03", "nd-05"},
		"sort=phone&order=desc":         {"nd-05", "nd-03", "nd-02", "nd-01"},
		"sort=department&order=asc":     {"nd-02", "nd-01", "nd-03", "nd-05"},
		"sort=department&order=desc":    {"nd-03", "nd-01", "nd-02", "nd-05"},
		"sort=last_login_at&order=asc":  {"nd-03", "nd-01", "nd-02", "nd-05"},
		"sort=last_login_at&order=desc": {"nd-01", "nd-03", "nd-05", "nd-02"},
		"sort=status&order=asc":         {"nd-03", "nd-01", "nd-02", "nd-05"},
		"sort=status&order=desc":        {"nd-05", "nd-02", "nd-01", "nd-03"},
	} {
		for _, limit := range []string{"1", "2", "3", "100"} {
			q := sortOrder + "&limit=" + limit
			t.Run(q, func(t *testing.T) {
				got := b.duyetHet(t, xaMau, q)
				if strings.Join(got, ",") != strings.Join(want, ",") {
					t.Fatalf("walked %v, want %v — a row repeated, lost or out of order", got, want)
				}
			})
		}
	}
}

// THE EMPTY VALUES COME LAST IN BOTH DIRECTIONS, ACROSS A PAGE BOUNDARY: a person who has never signed
// in, and a person whose department is gone, are exactly the rows an administrator opens this screen to
// find. With a plain `(col, id) > (…)` on a NULL column they would vanish from page two onward.
func TestStaffSortEmptyValuesAreOnALaterPageNotLost(t *testing.T) {
	b := moBanThuDS(t)
	for _, tc := range []struct {
		query string
		page2 []string
	}{
		{"sort=last_login_at&order=asc&limit=2", []string{"nd-02", "nd-05"}},
		{"sort=last_login_at&order=desc&limit=2", []string{"nd-05", "nd-02"}},
		{"sort=department&order=asc&limit=3", []string{"nd-05"}},
		{"sort=department&order=desc&limit=3", []string{"nd-05"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			first, err := b.trang(t, xaMau, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if !first.HasMore || first.NextCursor == "" {
				t.Fatalf("page one claims to be the last: %+v", first)
			}
			second, err := b.trang(t, xaMau, tc.query+"&cursor="+url.QueryEscape(first.NextCursor))
			if err != nil {
				t.Fatalf("page two: %v", err)
			}
			var got []string
			for _, cb := range second.Items {
				got = append(got, cb.ID)
			}
			if strings.Join(got, ",") != strings.Join(tc.page2, ",") {
				t.Fatalf("page two = %v, want %v", got, tc.page2)
			}
		})
	}
}

// RULE 3, FORBIDDEN #4: `next_cursor` travels in a URL, an access log and a browser history. Sorting
// by a person's name, position or office number must not put that value in it — page.KindRef carries
// the anchor row's id alone and the key is looked up server-side.
//
// MUTATION THAT MUST TURN THIS RED: declare `full_name` (or `position`, `phone`) as page.KindText.
func TestStaffSortCursorNeverCarriesThePersonalValue(t *testing.T) {
	b := moBanThuDS(t)
	var secrets []string
	for _, h := range dsDuLieu {
		for _, v := range []string{h.hoTen, h.chucVu, h.dienThoaiCoQuan, h.diDongCaNhan, h.email} {
			if v != "" {
				secrets = append(secrets, v)
			}
		}
	}

	for _, sortKey := range []string{"full_name", "position", "phone", "department", "last_login_at", "status"} {
		for _, dir := range []string{"asc", "desc"} {
			cursor := ""
			for i := 0; i < 10; i++ {
				q := "sort=" + sortKey + "&order=" + dir + "&limit=1"
				if cursor != "" {
					q += "&cursor=" + url.QueryEscape(cursor)
				}
				res, err := b.trang(t, xaMau, q)
				if err != nil {
					t.Fatalf("%s %s: %v", sortKey, dir, err)
				}
				if !res.HasMore {
					break
				}
				cursor = res.NextCursor
				raw, err := base64.RawURLEncoding.DecodeString(cursor)
				if err != nil {
					t.Fatalf("cursor is not base64url: %v", err)
				}
				for _, s := range secrets {
					if strings.Contains(string(raw), s) {
						t.Fatalf("%s %s: next_cursor carries a personal value of a fixture row: %s",
							sortKey, dir, raw)
					}
				}
			}
		}
	}
}

// Email and mobile STAY UNSORTABLE: they are personal data the owner did not list, and the
// prototype's single "Điện thoại" column is the office number.
func TestStaffSortRefusesEmailAndMobile(t *testing.T) {
	for _, col := range []string{"email", "mobile", "di_dong_ca_nhan", "ho_ten", "dang_nhap_gan_nhat"} {
		if _, err := page.New(SapXepCanBo, col, "", "", ""); err == nil {
			t.Errorf("sort=%s accepted", col)
		}
	}
}

// Every derived relation binds the commune ITSELF — QueryPage binds only the outer relation — and the
// department JOIN admits only live units of that commune.
func TestStaffSortRelationsBindTheCommune(t *testing.T) {
	for _, param := range []string{"department", "last_login_at", "status"} {
		for _, dir := range []page.Dir{page.Asc, page.Desc} {
			rel, key := staffSortRelation(param, dir)
			if rel == "nguoi_dung" || key == "" {
				t.Fatalf("%s %s: no derived relation", param, dir)
			}
			if !strings.Contains(rel, "WHERE nguoi_dung.tenant_id = $1") {
				t.Errorf("%s %s: relation does not bind the commune: %s", param, dir, rel)
			}
			if !strings.HasSuffix(rel, ") AS nguoi_dung") {
				t.Errorf("%s %s: relation must be aliased nguoi_dung so the search's correlation resolves: %s",
					param, dir, rel)
			}
			if strings.Contains(rel, "mat_khau") {
				t.Errorf("%s %s: relation names the credential column: %s", param, dir, rel)
			}
		}
	}
	for _, dir := range []page.Dir{page.Asc, page.Desc} {
		rel, _ := staffSortRelation("department", dir)
		for _, must := range []string{"b.tenant_id = $1", "b.deleted_at IS NULL"} {
			if !strings.Contains(rel, must) {
				t.Errorf("department %s: JOIN lacks %q: %s", dir, must, rel)
			}
		}
	}
	for _, param := range []string{"code", "created_at", "full_name", "position", "phone"} {
		if rel, key := staffSortRelation(param, page.Asc); rel != "nguoi_dung" || key != "" {
			t.Errorf("%s must page the plain table (KindRef looks its key up there): %q, %q", param, rel, key)
		}
	}
}

// --- the widened search ---------------------------------------------------------------------------

func TestStaffSearchMatchesEmail(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "b@example", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "b@example"}, "limit=100"), "nd-02")
	kiemIDs(t, "EXAMPLE.GOV", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "EXAMPLE.GOV"}, "limit=100"),
		"nd-01", "nd-02", "nd-03", "nd-05")
	// The soft-deleted nd-04 has an address too; it stays out.
	kiemIDs(t, "d@example (soft-deleted)", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "d@example"}, "limit=100"))
}

// A SOFT-DELETED DEPARTMENT'S NAME DOES NOT MATCH. The screen resolves a department name from the live
// unit list (the prototype's unitName prints "—" for an unknown id), so a row in a deleted unit shows no
// department; a search hit on a name the screen does not show would look like a wrong result.
func TestStaffSearchMatchesLiveDepartmentNameOnly(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "một cửa", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "một cửa"}, "limit=100"), "nd-02")
	kiemIDs(t, "văn phòng ubnd", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "văn phòng ubnd"}, "limit=100"),
		"nd-01", "nd-03")
	kiemIDs(t, "Lưu trữ (deleted unit)", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Lưu trữ"}, "limit=100"))

	l := b.lenhCuoi(t)
	if !strings.Contains(l.sql, "bp.tenant_id = $1") {
		t.Errorf("department subquery does not bind the commune: %q", l.sql)
	}
}

// RULE 1: commune B's `bp-001` carries the same id as commune A's. Its name must never find A's people,
// and A's search must never find B's.
func TestStaffSearchDepartmentNameStaysInItsCommune(t *testing.T) {
	b := moBanThuDS(t)
	kiemIDs(t, "Bí Mật from A", b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Bí Mật"}, "limit=100"))
	kiemIDs(t, "Bí Mật from B", b.timHet(t, dsXaB, domain.LocCanBo{TuKhoa: "Bí Mật"}, "limit=100"), "ndb-01")
	kiemIDs(t, "Văn phòng from B", b.timHet(t, dsXaB, domain.LocCanBo{TuKhoa: "Văn phòng"}, "limit=100"))
}

// The search and every new sort compose: the filter's placeholders come before the anchor's, on the
// derived relations as on the plain table.
func TestStaffSearchUnderEveryNewSortVisitsEachMatchOnce(t *testing.T) {
	b := moBanThuDS(t)
	for _, sortKey := range []string{"full_name", "position", "phone", "department", "last_login_at", "status"} {
		for _, dir := range []string{"asc", "desc"} {
			q := "sort=" + sortKey + "&order=" + dir + "&limit=1"
			kiemIDs(t, "Văn "+q, b.timHet(t, xaMau, domain.LocCanBo{TuKhoa: "Văn"}, q), "nd-01", "nd-03", "nd-05")
			kiemIDs(t, "bp-001 "+q, b.timHet(t, xaMau, domain.LocCanBo{BoPhanID: "bp-001"}, q), "nd-01", "nd-03")
		}
	}
}

// --- the filtered count --------------------------------------------------------------------------

// CountMatching IS THE LIST'S OWN PREDICATE, counted: for every filter it equals the number of rows a
// full walk of the list returns, soft-deleted excluded and commune bound.
func TestStaffCountMatchingEqualsTheWalk(t *testing.T) {
	b := moBanThuDS(t)
	for _, tc := range []struct {
		tenant string
		loc    domain.LocCanBo
	}{
		{xaMau, domain.LocCanBo{}},
		{xaMau, domain.LocCanBo{TuKhoa: "Văn"}},
		{xaMau, domain.LocCanBo{TuKhoa: "một cửa"}},
		{xaMau, domain.LocCanBo{TuKhoa: "example.gov"}},
		{xaMau, domain.LocCanBo{TuKhoa: "0900", BoPhanID: "bp-001"}},
		{xaMau, domain.LocCanBo{CongKhai: boolP(false)}},
		{xaMau, domain.LocCanBo{TuKhoa: "Phạm Thị D"}}, // soft-deleted only → 0
		{dsXaB, domain.LocCanBo{}},
	} {
		want := len(b.timHet(t, tc.tenant, tc.loc, "limit=100"))
		got, err := b.kho.CountMatching(ctxXa(tc.tenant), tc.loc)
		if err != nil {
			t.Fatalf("CountMatching(%+v): %v", tc.loc, err)
		}
		if got != want {
			t.Errorf("CountMatching(%s, %+v) = %d, the list walks %d", tc.tenant, tc.loc, got, want)
		}
	}

	if got, _ := b.kho.CountMatching(ctxXa(xaMau), domain.LocCanBo{}); got != 4 {
		t.Errorf("whole register of commune A = %d, want 4 (nd-04 is soft-deleted)", got)
	}
	l := b.lenhCuoi(t)
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") || l.args[0] != xaMau {
		t.Errorf("count does not bind the commune to $1: %q %v", l.sql, l.args)
	}
	for _, banned := range []string{"ORDER BY", "LIMIT"} {
		if strings.Contains(l.sql, banned) {
			t.Errorf("count carries %s: %q", banned, l.sql)
		}
	}
}
