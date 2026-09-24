package store

import (
	"database/sql"
	"strings"
	"testing"
)

// Migration 0010 against a real PostgreSQL: the Mini App publication columns (open question #12)
// and the `admin.user.delete` seed (ADR 0035).
//
// WHY A REAL DATABASE: every property below is a CHECK or a DEFAULT, and a CHECK has one behaviour
// no text test can show — it PASSES on NULL. Only the server can say whether a published row with
// no recorder is refused.
//
// Skipped unless VIGOV_TEST_DSN is set (rule 8). The always-running half is the schema-text test
// in migrations/danh_ba_mini_app_test.go. Shared harness: checker_pg_test.go.

// themDongDanhBa inserts one directory-only row (no account) with the given Mini App state and
// RETURNS THE ERROR: most cases here are about which rows the server refuses.
func themDongDanhBa(db *sql.DB, xa, id string, hien bool, dongYLuc any, ghiBoi string, thuTu any) error {
	_, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash,
		      hien_tren_mini_app, dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi, thu_tu_danh_ba)
		 VALUES ($1,$2,$3,$4,$5,false,'',$6,$7,$8,$9)`,
		xa, id, "CB-2026-"+strings.ToUpper(id[:6]), "Nguyễn Văn A", id+"@xa.danang.gov.vn",
		hien, dongYLuc, ghiBoi, thuTu)
	return err
}

func phaiBiTuChoiBoi(t *testing.T, err error, rangBuoc, vuot string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s — ràng buộc %s (migration 0010 §3) không còn hiệu lực", vuot, rangBuoc)
	}
	if !strings.Contains(err.Error(), rangBuoc) && !strings.Contains(err.Error(), "23514") {
		t.Fatalf("bị từ chối, nhưng không phải vì %s: %v", rangBuoc, err)
	}
}

// THE PROPERTY #12 RESTS ON. Three shapes of "published without consent", and each must be refused.
// The third is the reason the recorder column is NOT NULL: with a nullable column the CHECK would
// evaluate to NULL and PASS.
//
// THE MUTATION THAT MUST TURN THIS RED: delete `nguoi_dung_cong_khai_phai_co_dong_y` from 0010 §3.
func TestPgCongKhaiKhongCoDongYBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	phaiBiTuChoiBoi(t, themDongDanhBa(db, xa, "kdy001"+xa[20:], true, nil, "", nil),
		"nguoi_dung_cong_khai_phai_co_dong_y", "CÔNG KHAI KHÔNG CÓ ĐỒNG Ý được ghi vào")
	phaiBiTuChoiBoi(t, themDongDanhBa(db, xa, "kdy002"+xa[20:], true, "2026-09-24T08:00:00Z", "", nil),
		"nguoi_dung_cong_khai_phai_co_dong_y", "CÔNG KHAI KHÔNG GHI AI XÁC NHẬN được ghi vào")
	phaiBiTuChoiBoi(t, themDongDanhBa(db, xa, "kdy003"+xa[20:], true, nil, "CB-2026-GHI001", nil),
		"nguoi_dung_cong_khai_phai_co_dong_y", "CÔNG KHAI KHÔNG CÓ THỜI ĐIỂM ĐỒNG Ý được ghi vào")

	// A NULL recorder must be refused too — by NOT NULL rather than by the CHECK, which is the point.
	if err := themDongDanhBaGhiBoiNull(db, xa, "kdy004"+xa[20:]); err == nil {
		t.Fatal("CÔNG KHAI VỚI NGƯỜI GHI NULL được ghi vào — CHECK coi NULL là đạt; " +
			"dong_y_cong_khai_ghi_boi phải NOT NULL (migration 0010 §2)")
	}
}

func themDongDanhBaGhiBoiNull(db *sql.DB, xa, id string) error {
	_, err := db.Exec(
		`INSERT INTO nguoi_dung
		     (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash,
		      hien_tren_mini_app, dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi)
		 VALUES ($1,$2,$3,$4,$5,false,'',true,now(),NULL)`,
		xa, id, "CB-2026-"+strings.ToUpper(id[:6]), "Nguyễn Văn A", id+"@xa.danang.gov.vn")
	return err
}

// The other direction, so a constraint that refused every publication cannot pass the case above.
func TestPgCongKhaiCoDongYGhiDuoc(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	if err := themDongDanhBa(db, xa, "cdy001"+xa[20:], true, "2026-09-24T08:00:00Z", "CB-2026-GHI001", 3); err != nil {
		t.Fatalf("CÔNG KHAI CÓ ĐỦ ĐỒNG Ý BỊ TỪ CHỐI: %v", err)
	}
}

// "Unpublishing clears the consent marks; the next publish must ask again" (#12, user decision
// 2026-09-24). A stale consent left on an unpublished row is refused, so it can never be reused.
//
// THE MUTATION THAT MUST TURN THIS RED: delete `nguoi_dung_rut_cong_khai_xoa_dong_y` from 0010 §3.
func TestPgRutCongKhaiConDauDongYBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	id := "rut001" + xa[20:]
	if err := themDongDanhBa(db, xa, id, true, "2026-09-24T08:00:00Z", "CB-2026-GHI001", nil); err != nil {
		t.Fatalf("thêm dòng đã công khai: %v", err)
	}

	_, err := db.Exec(`UPDATE nguoi_dung SET hien_tren_mini_app = false WHERE tenant_id = $1 AND id = $2`, xa, id)
	phaiBiTuChoiBoi(t, err, "nguoi_dung_rut_cong_khai_xoa_dong_y",
		"RÚT CÔNG KHAI MÀ GIỮ LẠI DẤU ĐỒNG Ý được ghi vào")

	_, err = db.Exec(
		`UPDATE nguoi_dung
		    SET hien_tren_mini_app = false, dong_y_cong_khai_luc = NULL, dong_y_cong_khai_ghi_boi = ''
		  WHERE tenant_id = $1 AND id = $2`, xa, id)
	if err != nil {
		t.Fatalf("RÚT CÔNG KHAI ĐÚNG CÁCH (xoá dấu đồng ý) BỊ TỪ CHỐI: %v", err)
	}
}

// Defaults are the fail-closed ones: nobody published, no consent, no explicit order, no Zalo.
func TestPgMacDinhDanhBaMiniApp(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	id := "md0010" + xa[20:]
	if _, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash)
		 VALUES ($1,$2,$3,$4,$5,false,'')`,
		xa, id, "CB-2026-"+strings.ToUpper(id[:6]), "Nguyễn Văn A", id+"@xa.danang.gov.vn"); err != nil {
		t.Fatalf("thêm dòng: %v", err)
	}

	var (
		coZalo, hien bool
		luc          sql.NullTime
		ghiBoi       string
		thuTu        sql.NullInt64
	)
	if err := db.QueryRow(
		`SELECT co_zalo, hien_tren_mini_app, dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi, thu_tu_danh_ba
		   FROM nguoi_dung WHERE tenant_id = $1 AND id = $2`, xa, id).
		Scan(&coZalo, &hien, &luc, &ghiBoi, &thuTu); err != nil {
		t.Fatalf("đọc lại: %v", err)
	}
	if hien {
		t.Error("hien_tren_mini_app mặc định TRUE — mọi cán bộ bị công khai mà không ai được hỏi (#12)")
	}
	if coZalo || luc.Valid || ghiBoi != "" || thuTu.Valid {
		t.Errorf("mặc định sai: co_zalo=%v luc=%v ghi_boi=%q thu_tu=%v", coZalo, luc, ghiBoi, thuTu)
	}
}

func TestPgThuTuDanhBaAmBiTuChoi(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	phaiBiTuChoiBoi(t, themDongDanhBa(db, xa, "tta001"+xa[20:], false, nil, "", -1),
		"nguoi_dung_thu_tu_danh_ba_khong_am", "THỨ TỰ ÂM được ghi vào")
}

// The key TASK-04's route checks exists, in QUẢN TRỊ (rule 5, invariant 3c).
func TestPgKhoaAdminUserDeleteDaNap(t *testing.T) {
	db := moKetNoi(t)
	var nhom string
	if err := db.QueryRow(`SELECT nhom FROM quyen WHERE ma = 'admin.user.delete'`).Scan(&nhom); err != nil {
		t.Fatalf("khoá admin.user.delete không có trong bảng quyen: %v", err)
	}
	if nhom != "QUẢN TRỊ" {
		t.Errorf("admin.user.delete ở nhóm %q, muốn QUẢN TRỊ", nhom)
	}
}
