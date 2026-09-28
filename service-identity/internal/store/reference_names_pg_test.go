package store

import (
	"sort"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The three reads behind ResolveOrgUnitNames, ResolveTaskBlocLabels and ResolveLiveOrgUnitCodes,
// against a real PostgreSQL (harness in checker_pg_test.go). Skipped unless VIGOV_TEST_DSN is set.
//
// What each pins: the display reads ANSWER removed rows, flagged; the decision read does NOT; and all
// three leave another commune's key absent even when the key is the same string (the normal case —
// every commune seeds the same bloc codes and similar unit slugs).

func TestPgOrgUnitNamesIncludeRemovedAndStayInCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	themBoPhanPg(t, db, xa, "bp-live", "van-phong", "", false)
	themBoPhanPg(t, db, xa, "bp-gone", "da-xoa", "", true)
	themBoPhanPg(t, db, otherXa, "bp-other", "van-phong", "", false)

	got, err := NewBoPhanStore(pkgstore.New(db)).NamesByID(ctxXa(xa),
		[]string{"bp-live", "bp-gone", "bp-other", "bp-none"})
	if err != nil {
		t.Fatalf("NamesByID: %v", err)
	}
	byID := map[string]bool{}
	for _, n := range got {
		byID[n.ID] = n.Live
		if n.Name == "" {
			t.Errorf("%s: tên rỗng", n.ID)
		}
	}
	if live, ok := byID["bp-live"]; !ok || !live {
		t.Errorf("bp-live = (%v, %v), muốn có mặt và còn hiệu lực", live, ok)
	}
	if live, ok := byID["bp-gone"]; !ok || live {
		t.Errorf("bp-gone = (%v, %v), muốn CÓ MẶT và đánh dấu đã gỡ — sổ in phải ghi đúng điều hồ sơ giữ", live, ok)
	}
	if _, ok := byID["bp-other"]; ok {
		t.Error("BỘ PHẬN CỦA XÃ KHÁC có trong câu trả lời — rò giữa hai xã (luật 1)")
	}
	if len(got) != 2 {
		t.Errorf("số dòng = %d, muốn 2", len(got))
	}
}

func TestPgLiveOrgUnitCodesOnlyLiveUnitsOfThisCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	themBoPhanPg(t, db, xa, "bp-live", "van-phong", "", false)
	themBoPhanPg(t, db, xa, "bp-gone", "da-xoa", "", true)
	themBoPhanPg(t, db, otherXa, "bp-other", "chi-o-xa-khac", "", false)

	got, err := NewBoPhanStore(pkgstore.New(db)).LiveIDsByCode(ctxXa(xa),
		[]string{"van-phong", "da-xoa", "chi-o-xa-khac", "khong-co"})
	if err != nil {
		t.Fatalf("LiveIDsByCode: %v", err)
	}
	if len(got) != 1 || got[0].Ma != "van-phong" || got[0].ID != "bp-live" {
		t.Fatalf("khớp mã = %+v, muốn chỉ van-phong→bp-live (đã xoá / xã khác / không có phải vắng)", got)
	}
}

func TestPgTaskBlocLabelsIncludeRemovedAndStayInCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	themKhoiNhiemVu(t, db, xa, "k1-"+xa, "khoi-uy-ban", "Khối Uỷ ban", 1)
	themKhoiNhiemVu(t, db, xa, "k2-"+xa, "khoi-cu", "Khối cũ", 2)
	if _, err := db.Exec(`UPDATE khoi_nhiem_vu SET deleted_at = now(), deleted_by = 'CB-test', delete_reason = 'test'
		WHERE tenant_id = $1 AND id = $2`, xa, "k2-"+xa); err != nil {
		t.Fatalf("xoá mềm khối: %v", err)
	}
	themKhoiNhiemVu(t, db, otherXa, "k3-"+otherXa, "chi-o-xa-khac", "Khối xã khác", 1)

	got, err := NewKhoiNhiemVuStore(pkgstore.New(db)).LabelsByCode(ctxXa(xa),
		[]string{"khoi-uy-ban", "khoi-cu", "chi-o-xa-khac", "khong-co"})
	if err != nil {
		t.Fatalf("LabelsByCode: %v", err)
	}
	var seen []string
	for _, l := range got {
		seen = append(seen, l.Ma)
		switch l.Ma {
		case "khoi-uy-ban":
			if !l.Live || l.Label != "Khối Uỷ ban" {
				t.Errorf("khoi-uy-ban = %+v", l)
			}
		case "khoi-cu":
			if l.Live {
				t.Error("khối đã xoá mềm bị đánh dấu còn hiệu lực")
			}
		}
	}
	sort.Strings(seen)
	if strings.Join(seen, ",") != "khoi-cu,khoi-uy-ban" {
		t.Errorf("mã trả về = %v, muốn khoi-cu,khoi-uy-ban (xã khác / không có phải vắng)", seen)
	}
}
