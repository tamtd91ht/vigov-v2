package store

import (
	"database/sql"
	"net/url"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration test for DanhSachCuaCongDan against the real schema. SKIPS unless VIGOV_TEST_DSN is
// set (harness in danh_muc_nhiem_vu_pg_test.go), and the package still prints `ok` — read the note
// at the top of phieu_phan_anh_pg_test.go before believing a green run.
//
// It is the half the fake driver cannot prove: that PostgreSQL applies both predicates, excludes
// the soft-deleted row and the staff-booked one, and pages newest first across the cursor.

func themPhieuCuaCongDan(t *testing.T, db *sql.DB, xa, id, ma, kenh string, congDan any,
	linhVuc any, goc time.Time, hanTiepNhan any) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO phieu_phan_anh
		 (tenant_id, id, ma_tra_cuu, kenh_tiep_nhan, cong_dan_id, noi_dung, linh_vuc, trang_thai,
		  goc_dem_han, vao_so_luc, han_tiep_nhan, han_xu_ly_xong)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'da-tiep-nhan',$8,$8,$9,NULL)`,
		xa, id, ma, kenh, congDan, "Nội dung phản ánh dùng cho phép kiểm.", linhVuc, goc, hanTiepNhan,
	); err != nil {
		t.Fatalf("thêm phiếu %s: %v", id, err)
	}
}

func TestPgDanhSachCuaCongDanChiPhieuCuaMinhTrongXaCuaPhien(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	goc := time.Now().UTC().Add(-5 * time.Hour).Truncate(time.Microsecond)
	const toi, nguoiKhac = "cd-pg-toi", "cd-pg-khac"

	themPhieuCuaCongDan(t, db, xaA, "pa-ds-1", "PA-DS01-0000-0001", "zalo-mini-app", toi, nil, goc, goc.Add(8*time.Hour))
	themPhieuCuaCongDan(t, db, xaA, "pa-ds-2", "PA-DS02-0000-0002", "zalo-mini-app", toi, nil, goc.Add(time.Hour), goc.Add(9*time.Hour))
	// Newest of mine, then soft-deleted: must not appear.
	themPhieuCuaCongDan(t, db, xaA, "pa-ds-3", "PA-DS03-0000-0003", "zalo-mini-app", toi, nil, goc.Add(2*time.Hour), goc.Add(10*time.Hour))
	if _, err := db.Exec(`UPDATE phieu_phan_anh SET deleted_at = now(), deleted_by = 'nd-test',
		delete_reason = 'phép kiểm' WHERE tenant_id = $1 AND id = $2`, xaA, "pa-ds-3"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}
	themPhieuCuaCongDan(t, db, xaA, "pa-ds-4", "PA-DS04-0000-0004", "zalo-mini-app", nguoiKhac, nil, goc, goc.Add(8*time.Hour))
	// Staff-booked: no citizen account, field required, no acknowledge deadline (migration 0004).
	themPhieuCuaCongDan(t, db, xaA, "pa-ds-5", "PA-DS05-0000-0005", "can-bo-nhap-ho", nil, "dien", goc, nil)
	// The SAME citizen id, in commune B.
	themPhieuCuaCongDan(t, db, xaB, "pa-ds-6", "PA-DS06-0000-0006", "zalo-mini-app", toi, nil, goc, goc.Add(8*time.Hour))

	s := NewPhieuPhanAnhStore(pkgstore.New(db))
	ctxA := ctxXa(tenant.ID(xaA))

	var thay []string
	conTro := ""
	for i := 0; i < 5; i++ {
		q := url.Values{"limit": {"1"}}
		if conTro != "" {
			q.Set("cursor", conTro)
		}
		kq, err := s.DanhSachCuaCongDan(ctxA, toi, "", trangCuaToi(t, q))
		if err != nil {
			t.Fatalf("trang %d: %v", i+1, err)
		}
		for _, p := range kq.Items {
			thay = append(thay, p.ID)
		}
		if !kq.HasMore {
			break
		}
		conTro = kq.NextCursor
	}
	if len(thay) != 2 || thay[0] != "pa-ds-2" || thay[1] != "pa-ds-1" {
		t.Fatalf("xã A, công dân của phiên thấy %v — muốn [pa-ds-2 pa-ds-1]: chỉ phiếu của mình, còn sống, "+
			"không phiếu nhập hộ, mới nhất trước", thay)
	}

	kq, err := s.DanhSachCuaCongDan(ctxXa(tenant.ID(xaB)), toi, "", trangCuaToi(t, url.Values{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 1 || kq.Items[0].ID != "pa-ds-6" {
		t.Fatalf("cùng công dân trong xã B thấy %d mục — muốn đúng phiếu của xã B", len(kq.Items))
	}
}
