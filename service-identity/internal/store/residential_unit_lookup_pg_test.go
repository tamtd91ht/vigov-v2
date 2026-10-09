package store

import (
	"database/sql"
	"testing"
)

// The two reads behind ResolveActiveResidentialUnits and ResolveResidentialUnitNames, against a real
// PostgreSQL (harness in checker_pg_test.go). Skipped unless VIGOV_TEST_DSN is set.
//
// What each pins, all of it living in the SQL where no fake can disagree: the decision read refuses
// out-of-use AND soft-deleted units; the display read answers both (out of use as live, soft-deleted
// flagged); and both leave another commune's unit absent.

// seedResidentialUnits puts four units in xa (in use, out of use, soft deleted) and one in otherXa.
func seedResidentialUnits(t *testing.T, db *sql.DB, xa, otherXa string) {
	t.Helper()
	themThonToDanPho(t, db, xa, "tt-active", "thon-binh-an", "Thôn Bình An", "")
	themThonToDanPho(t, db, xa, "tt-off", "thon-cu", "Thôn Cũ", "")
	themThonToDanPho(t, db, xa, "tt-gone", "thon-da-xoa", "Thôn Đã Xoá", "")
	themThonToDanPho(t, db, otherXa, "tt-other", "thon-xa-khac", "Thôn Xã Khác", "")
	if _, err := db.Exec(`UPDATE thon_to_dan_pho SET dang_dung = false WHERE tenant_id = $1 AND id = $2`,
		xa, "tt-off"); err != nil {
		t.Fatalf("ngưng dùng thôn: %v", err)
	}
	if _, err := db.Exec(`UPDATE thon_to_dan_pho SET deleted_at = now(), deleted_by = 'CB-test', delete_reason = 'test'
		WHERE tenant_id = $1 AND id = $2`, xa, "tt-gone"); err != nil {
		t.Fatalf("xoá mềm thôn: %v", err)
	}
}

var residentialUnitAsk = []string{"tt-active", "tt-off", "tt-gone", "tt-other", "tt-none"}

func TestPgActiveResidentialUnitsOnlyInUseUnitsOfThisCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	seedResidentialUnits(t, db, xa, otherXa)

	got, err := dungThonToDanPhoStore(db).ActiveUnitsByID(ctxXa(xa), residentialUnitAsk)
	if err != nil {
		t.Fatalf("ActiveUnitsByID: %v", err)
	}
	if len(got) != 1 || got[0].ID != "tt-active" || got[0].Name != "Thôn Bình An" {
		t.Fatalf("đơn vị đang dùng = %+v, muốn chỉ tt-active (ngưng dùng / đã xoá / xã khác / không có phải vắng)", got)
	}
}

func TestPgResidentialUnitNamesIncludeOutOfUseAndRemovedAndStayInCommune(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	seedResidentialUnits(t, db, xa, otherXa)

	got, err := dungThonToDanPhoStore(db).UnitNamesByID(ctxXa(xa), residentialUnitAsk)
	if err != nil {
		t.Fatalf("UnitNamesByID: %v", err)
	}
	byID := map[string]bool{}
	for _, n := range got {
		byID[n.ID] = n.Live
		if n.Name == "" {
			t.Errorf("%s: tên rỗng", n.ID)
		}
	}
	if live, ok := byID["tt-active"]; !ok || !live {
		t.Errorf("tt-active = (%v, %v), muốn có mặt và còn hiệu lực", live, ok)
	}
	if live, ok := byID["tt-off"]; !ok || !live {
		t.Errorf("tt-off = (%v, %v), muốn CÓ MẶT và còn hiệu lực — ngưng dùng không phải đã gỡ (ADR 0059 §2)", live, ok)
	}
	if live, ok := byID["tt-gone"]; !ok || live {
		t.Errorf("tt-gone = (%v, %v), muốn CÓ MẶT và đánh dấu đã gỡ", live, ok)
	}
	if _, ok := byID["tt-other"]; ok {
		t.Error("THÔN CỦA XÃ KHÁC có trong câu trả lời — rò giữa hai xã (luật 1)")
	}
	if len(got) != 3 {
		t.Errorf("số dòng = %d, muốn 3", len(got))
	}
}

// The citizen picker's list (GET /api/v1/my-residential-units): the decision read's predicate over the
// whole commune — in use and live only, this commune only — in the commune's rank order, ids and names
// only.
func TestPgActiveUnitsListsOnlyInUseUnitsOfThisCommuneInRankOrder(t *testing.T) {
	db := moKetNoi(t)
	xa, otherXa := xaRieng(t)
	seedResidentialUnits(t, db, xa, otherXa)
	// A second in-use unit ranked BEFORE the first, so the order is the rank and not the name.
	themThonToDanPho(t, db, xa, "tt-first", "thon-xuan", "Thôn Xuân", "")
	if _, err := db.Exec(`UPDATE thon_to_dan_pho SET sort_order = CASE id WHEN 'tt-first' THEN 1 ELSE 2 END
		WHERE tenant_id = $1 AND id IN ('tt-first', 'tt-active')`, xa); err != nil {
		t.Fatalf("xếp hạng thôn: %v", err)
	}

	got, err := dungThonToDanPhoStore(db).ActiveUnits(ctxXa(xa))
	if err != nil {
		t.Fatalf("ActiveUnits: %v", err)
	}
	if len(got) != 2 || got[0].ID != "tt-first" || got[1].ID != "tt-active" || got[1].Name != "Thôn Bình An" {
		t.Fatalf("danh sách = %+v, muốn [tt-first, tt-active] theo hạng (ngưng dùng / đã xoá / xã khác phải vắng)", got)
	}
}
