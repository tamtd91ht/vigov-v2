package crosstenant

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Integration tests for the two cross-commune reads, against a real PostgreSQL.
//
// WHY A REAL DATABASE, and it is the whole point of this file: what these two queries claim is
// that they reach rows of MORE THAN ONE COMMUNE — one citizen's relationships wherever they
// are (ADR 0002), and a pairing code belonging to a commune the caller has not named
// (ADR 0019, invariant 8). Neither claim can be checked by a driver that returns whatever the
// test handed it: the fake would agree just as happily with a query that had a commune
// predicate in it.
//
// THEY SKIP WITHOUT VIGOV_TEST_DSN AND THE PACKAGE STILL PRINTS `ok`. The harness (TestMain,
// moKetNoi, soGiaRieng, trongGiaoDich) lives in dinh_danh_cong_dan_pg_test.go.

// xaRiengCT returns two commune ids unique to this test, so tests sharing one schema cannot see
// each other's rows.
func xaRiengCT(t *testing.T) (string, string) {
	t.Helper()
	n := time.Now().UnixNano()
	return fmt.Sprintf("%026d", n)[:26], fmt.Sprintf("%026d", n+1)[:26]
}

// congDanThat creates a real identity row and returns its id.
func congDanThat(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	trongGiaoDich(t, db, func(tx *sql.Tx) error {
		dd, _, err := NewDinhDanhStore().TimHoacTao(context.Background(), tx, soGiaRieng(t))
		id = dd.ID
		return err
	})
	return id
}

func themQuanHeCT(t *testing.T, db *sql.DB, xa, congDanID string, khaiLuc time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO quan_he_cong_dan_xa
		   (tenant_id, cong_dan_id, khai_cu_tru, nguon_khai, khai_luc, trang_thai_xac_thuc)
		 VALUES ($1,$2,'tam_tru','cong_dan',$3,'cho_xac_thuc')`,
		xa, congDanID, khaiLuc); err != nil {
		t.Fatalf("thêm quan hệ: %v", err)
	}
}

func TestPgXaCuaCongDanThayDuHaiXa(t *testing.T) {
	// ADR 0002: a citizen is many-to-many with communes, and the Mini App has to show them the
	// list. A scoped query cannot answer this — it would have to name a commune in order to find
	// out which communes there are.
	db := moKetNoi(t)
	xaA, xaB := xaRiengCT(t)
	cd := congDanThat(t, db)
	moc := time.Now().UTC().Add(-time.Hour)

	themQuanHeCT(t, db, xaA, cd, moc)                  // older
	themQuanHeCT(t, db, xaB, cd, moc.Add(time.Minute)) // newer

	ra, err := NewQuanHeStore(db).XaCuaCongDanTrongPhien(ctxKhamPha(t, httpx.CitizenSession{
		ID: "SID-PG", CitizenID: cd, TenantID: "",
	}))
	if err != nil {
		t.Fatalf("đọc danh sách xã: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d xã, muốn 2", len(ra))
	}
	// ORDER BY khai_luc DESC: most recent first.
	if ra[0].XaID != tenant.ID(xaB) || ra[1].XaID != tenant.ID(xaA) {
		t.Errorf("thứ tự sai: %q rồi %q", ra[0].XaID, ra[1].XaID)
	}
	for _, q := range ra {
		if q.Khai != domain.KhaiTamTru || q.TrangThai != domain.ChoXacThuc {
			t.Errorf("hai sự thật đọc sai: (%q, %q)", q.Khai, q.TrangThai)
		}
	}
}

func TestPgXaCuaCongDanChiThayCuaChinhMinh(t *testing.T) {
	// RULE 4, INVARIANT 1. Two citizens, one commune. The query is keyed on the identity in the
	// session, so the second citizen's row must not appear — and this is the assertion that
	// separates a sanctioned cross-commune read from the rollup doc.go forbids.
	db := moKetNoi(t)
	xa, _ := xaRiengCT(t)
	toi, nguoiKhac := congDanThat(t, db), congDanThat(t, db)
	moc := time.Now().UTC()

	themQuanHeCT(t, db, xa, toi, moc)
	themQuanHeCT(t, db, xa, nguoiKhac, moc)

	ra, err := NewQuanHeStore(db).XaCuaCongDanTrongPhien(ctxKhamPha(t, httpx.CitizenSession{
		ID: "SID-PG", CitizenID: toi, TenantID: "",
	}))
	if err != nil {
		t.Fatalf("đọc danh sách xã: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn đúng 1 — dòng của chính mình", len(ra))
	}
}

func TestPgXaCuaCongDanBoQuaDongDaXoaMem(t *testing.T) {
	// RULE 7, INVARIANT 2 — including on the citizen's own screen.
	db := moKetNoi(t)
	xaA, xaB := xaRiengCT(t)
	cd := congDanThat(t, db)
	moc := time.Now().UTC()

	themQuanHeCT(t, db, xaA, cd, moc)
	themQuanHeCT(t, db, xaB, cd, moc)
	if _, err := db.Exec(
		`UPDATE quan_he_cong_dan_xa SET deleted_at = now(), deleted_by = 'CB-01',
		        delete_reason = 'kiểm thử'
		 WHERE tenant_id = $1 AND cong_dan_id = $2`, xaB, cd); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ra, err := NewQuanHeStore(db).XaCuaCongDanTrongPhien(ctxKhamPha(t, httpx.CitizenSession{
		ID: "SID-PG", CitizenID: cd, TenantID: "",
	}))
	if err != nil {
		t.Fatalf("đọc danh sách xã: %v", err)
	}
	if len(ra) != 1 || ra[0].XaID != tenant.ID(xaA) {
		t.Fatalf("dòng đã xoá mềm vẫn hiện: %v", ra)
	}
}

// --- the pairing code, found without naming a commune -------------------------------------------

func themMaGhepCT(t *testing.T, db *sql.DB, xa, id, ma string) {
	t.Helper()
	tong := sha256.Sum256([]byte(ma))
	if _, err := db.Exec(
		`INSERT INTO ghep_phien (tenant_id, id, bam_ma, ky_tu_doi_chieu, het_han_luc)
		 VALUES ($1,$2,$3,'K7QM', now() + interval '120 seconds')`,
		xa, id, hex.EncodeToString(tong[:])); err != nil {
		t.Fatalf("thêm mã ghép: %v", err)
	}
}

func TestPgTheoMaTimThayMaCuaMotXaChuaAiNeuTen(t *testing.T) {
	// ADR 0019, INVARIANT 8, AND THE REASON THIS QUERY IS UNSCOPED. The caller does not know the
	// commune yet; it has to LEARN it from the row in order to compare — because a mismatch must
	// raise an alert, and an alert needs "this code exists elsewhere" rather than "no such
	// code". Scoping the lookup would have made the two indistinguishable.
	db := moKetNoi(t)
	xa, _ := xaRiengCT(t)
	ma := "ma-ghep-gia-pg-" + fmt.Sprint(time.Now().UnixNano())
	themMaGhepCT(t, db, xa, "GP-PG-01", ma)

	m, ok, err := NewGhepPhienStore(db).TheoMa(context.Background(), ma)
	if err != nil || !ok {
		t.Fatalf("TheoMa = (%v, %v), muốn tìm thấy", ok, err)
	}
	if string(m.XaID) != xa {
		t.Errorf("XaID = %q, muốn %q", m.XaID, xa)
	}
	if m.ID != "GP-PG-01" || m.KyTuDoiChieu != "K7QM" {
		t.Errorf("đọc sai cột: id=%q, ký tự=%q", m.ID, m.KyTuDoiChieu)
	}
	if ra := m.TrangThai(time.Now().UTC()); ra != domain.ChoGhep {
		t.Errorf("trạng thái = %q, muốn %q", ra, domain.ChoGhep)
	}
}

func TestPgTheoMaSuyTrangThaiTuDongThat(t *testing.T) {
	// The state has no column: it is derived from three timestamps read out of the row. Marking
	// the row used must change the answer without any column called `trang_thai` existing.
	db := moKetNoi(t)
	xa, _ := xaRiengCT(t)
	ma := "ma-ghep-gia-pg-" + fmt.Sprint(time.Now().UnixNano())
	themMaGhepCT(t, db, xa, "GP-PG-02", ma)

	// CANCELLING RATHER THAN REDEEMING, on purpose: a redeem needs a real session for the
	// foreign key, and that path is asserted in the store's own pg suite. What THIS test is
	// about is the derivation — the row gains a timestamp, and the answer changes without any
	// column called `trang_thai` existing.
	if _, err := db.Exec(
		`UPDATE ghep_phien SET huy_luc = now(), huy_ly_do = 'công dân bỏ giữa chừng'
		 WHERE tenant_id = $1 AND id = $2`, xa, "GP-PG-02"); err != nil {
		t.Fatalf("huỷ mã: %v", err)
	}

	m, ok, err := NewGhepPhienStore(db).TheoMa(context.Background(), ma)
	if err != nil || !ok {
		t.Fatalf("TheoMa = (%v, %v)", ok, err)
	}
	if ra := m.TrangThai(time.Now().UTC()); ra != domain.DaHuy {
		t.Errorf("trạng thái = %q, muốn %q", ra, domain.DaHuy)
	}
}

func TestPgTheoMaKhongCoThiOkFalse(t *testing.T) {
	db := moKetNoi(t)

	m, ok, err := NewGhepPhienStore(db).TheoMa(context.Background(), "ma-khong-ton-tai-o-dau-ca")
	if err != nil {
		t.Fatalf("không tìm thấy không phải lỗi, nhận: %v", err)
	}
	if ok {
		t.Fatalf("tìm thấy một mã không tồn tại: %v", m)
	}
}
