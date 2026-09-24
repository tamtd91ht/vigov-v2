package app

import (
	"errors"
	"strings"
	"testing"
)

// A CYCLE RUNNING THROUGH A SOFT-DELETED ANCESTOR IS STILL A CYCLE.
//
// X (root) ← D (soft-deleted, under X) ← P (live, under D). Moving X under P makes X its own
// ancestor through D. store.KhoaBoPhan SELECTS the deleted flag instead of filtering on it precisely
// so the walk can follow D; a walk that treated a deleted ancestor as the end of the chain would find
// "no cycle", commit, and leave an org chart no tree read terminates on.
//
// MUTATION THAT MUST TURN THIS RED: in kiemVongLap, `if daXoa { return nil }` past the first step.
func TestSuaBoPhanVongQuaToTienDaXoaMemVanBiTuChoi(t *testing.T) {
	k := &khoBoPhanGia{hang: []hangBoPhan{
		{xa: xaBoPhan, id: "bp-x", ma: "x", ten: "X"},
		{xa: xaBoPhan, id: "bp-d", ma: "d", ten: "D", cha: "bp-x", daXoa: true},
		{xa: xaBoPhan, id: "bp-p", ma: "p", ten: "P", cha: "bp-d"},
	}}
	uc, ctx := dungUseCaseBoPhan(t, k)

	_, err := uc.Sua(ctx, "bp-x", YeuCauSuaBoPhan{ChaID: chuoi("bp-p")}, nguoiBoPhan())
	if !errors.Is(err, ErrCayBoPhanVongLap) {
		t.Fatalf("lỗi = %v, muốn ErrCayBoPhanVongLap — vòng đi qua bộ phận đã xoá mềm bị bỏ sót", err)
	}
	if len(k.cau("UPDATE bo_phan")) != 0 || k.daCommit != 0 {
		t.Error("vòng lặp mà vẫn UPDATE hoặc commit")
	}
}

// EVERY STATEMENT OF AN EDIT BINDS THE CONTEXT'S COMMUNE AT $1, AND THE UPDATE IS BOUNDED ON
// (commune, id, live). TestThemBoPhanMoiCauMangXaCuaNguCanh covers the create path only; the edit
// path adds the ancestor walk and the UPDATE, and the fake driver does not evaluate an UPDATE's WHERE
// — so a predicate dropped from it would pass every other test here.
//
// MUTATION THAT MUST TURN THIS RED: drop `AND deleted_at IS NULL` from BoPhanStore.CapNhat.
func TestSuaBoPhanMoiCauMangXaVaUpdateBoTrenDongSong(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	if _, err := uc.Sua(ctx, "bp-c", YeuCauSuaBoPhan{Ten: chuoi("TỔ MỘT CỬA LIÊN THÔNG"), ChaID: chuoi("bp-a")}, nguoiBoPhan()); err != nil {
		t.Fatal(err)
	}
	if len(k.lenh) < 4 {
		t.Fatalf("chỉ %d câu lệnh — phép kiểm không kiểm gì", len(k.lenh))
	}
	for _, l := range k.lenh {
		if len(l.args) == 0 || l.args[0] != string(xaBoPhan) {
			t.Errorf("câu lệnh không mang xã ở $1: %s (args %v)", l.sql, l.args)
		}
	}
	up := k.cau("UPDATE bo_phan")
	if len(up) != 1 {
		t.Fatalf("có %d UPDATE, muốn 1", len(up))
	}
	gon := strings.Join(strings.Fields(up[0].sql), " ")
	if !strings.Contains(gon, "WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL") {
		t.Errorf("UPDATE không bó theo (xã, id, dòng sống): %s", gon)
	}
}
