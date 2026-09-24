package store

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The org chart's store, in two halves.
//
// THE FIRST TEST RUNS EVERYWHERE and pins the three traps of the count query in its TEXT — the ones
// truyVanBoPhanKemSoCanBo names. It is a weak check (a string is not a plan), stated as such; it
// exists so that the one-word edits that break the count silently cannot happen with no test red on
// a machine without PostgreSQL, which is every machine this has been built on so far.
//
// THE REST NEED A REAL PostgreSQL (VIGOV_TEST_DSN; harness in checker_pg_test.go) and skip without
// one: the non-partial unique key, the foreign key on the parent, the partition-named constraint the
// error translation matches, and the counts themselves.

func TestTruyVanBoPhanDemDungCach(t *testing.T) {
	q := truyVanBoPhanKemSoCanBo
	for _, can := range []string{
		"count(nd.id)",                 // not count(*): a LEFT JOIN row of NULLs must count 0
		"nd.tenant_id  = bp.tenant_id", // the join repeats the commune (rule 1)
		"AND nd.deleted_at IS NULL",    // in the JOIN …
		"AND nd.dang_hoat_dong",        // … with the lock flag
		"WHERE bp.tenant_id = $1",      // the commune from the context
		"AND bp.deleted_at IS NULL",    // soft-deleted units excluded (rule 7, invariant 2)
		"ORDER BY bp.thu_tu, bp.ten",
	} {
		if !strings.Contains(q, can) {
			t.Errorf("câu đếm thiếu %q", can)
		}
	}
	where := q[strings.Index(q, "WHERE"):]
	if strings.Contains(where, "nd.") {
		t.Errorf("điều kiện trên nguoi_dung nằm trong WHERE — bộ phận không có ai sẽ biến mất khỏi sơ đồ: %s", where)
	}
	if strings.Contains(q, "count(*)") {
		t.Error("count(*) trên LEFT JOIN đếm dòng NULL thành 1")
	}
}

func themBoPhanPg(t *testing.T, db *sql.DB, xa, id, ma, cha string, daXoa bool) {
	t.Helper()
	var c any
	if cha != "" {
		c = cha
	}
	_, err := db.Exec(`INSERT INTO bo_phan (tenant_id, id, ten, ma, cha_id, deleted_at)
		VALUES ($1,$2,$3,$4,$5, CASE WHEN $6 THEN now() END)`, xa, id, strings.ToUpper(ma), ma, c, daXoa)
	if err != nil {
		t.Fatalf("thêm bộ phận: %v", err)
	}
}

func themNguoiBoPhanPg(t *testing.T, db *sql.DB, xa, id, boPhanID string, dangHoatDong, daXoa bool) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, bo_phan_id, dang_hoat_dong, deleted_at)
		VALUES ($1,$2,$3,'Nguyễn Văn A',$4,$5,$6, CASE WHEN $7 THEN now() END)`,
		xa, id, "CB-"+id, id+"@xa.gov.vn", boPhanID, dangHoatDong, daXoa)
	if err != nil {
		t.Fatalf("thêm cán bộ: %v", err)
	}
}

func TestPgBoPhanDemCanBoDangLamViec(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	themBoPhanPg(t, db, xa, "bp-1", "van-phong", "", false)
	themBoPhanPg(t, db, xa, "bp-2", "to-trong", "", false)
	themBoPhanPg(t, db, xa, "bp-x", "da-xoa", "", true)
	themNguoiBoPhanPg(t, db, xa, "nd-1", "bp-1", true, false)
	themNguoiBoPhanPg(t, db, xa, "nd-2", "bp-1", true, false)
	themNguoiBoPhanPg(t, db, xa, "nd-khoa", "bp-1", false, false) // locked — retired/transferred
	themNguoiBoPhanPg(t, db, xa, "nd-xoa", "bp-1", true, true)    // soft-deleted
	// Same unit id in another commune, with staff: must not be counted here.
	themBoPhanPg(t, db, xaKhac, "bp-1", "van-phong", "", false)
	themNguoiBoPhanPg(t, db, xaKhac, "nd-k", "bp-1", true, false)

	ds, err := NewBoPhanStore(pkgstore.New(db)).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatal(err)
	}
	dem := map[string]int{}
	for _, bp := range ds {
		dem[bp.ID] = bp.SoCanBo
	}
	if len(ds) != 2 || dem["bp-1"] != 2 || dem["bp-2"] != 0 {
		t.Fatalf("đếm = %v (%d bộ phận), muốn bp-1:2 bp-2:0 và không có bộ phận đã xoá", dem, len(ds))
	}
}

func TestPgBoPhanGhi(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewBoPhanStore(kho)
	themBoPhanPg(t, db, xa, "bp-1", "van-phong", "", false)
	themBoPhanPg(t, db, xa, "bp-x", "da-xoa", "", true)
	themBoPhanPg(t, db, xaKhac, "bp-k", "van-phong-2", "", false)

	err := kho.For(ctxXa(xa)).Tx(ctxXa(xa), func(tx *pkgstore.ScopedTx) error {
		// The non-partial key: a soft-deleted unit's code is still taken.
		if err := s.Chen(ctxXa(xa), tx, domain.BoPhan{ID: "bp-2", Ten: "X", Ma: "da-xoa"}); !errors.Is(err, ErrMaBoPhanDaDung) {
			t.Errorf("chèn trùng mã của dòng đã xoá mềm: %v, muốn ErrMaBoPhanDaDung", err)
		}
		return errors.New("rollback")
	})
	if err == nil {
		t.Fatal("giao dịch không rollback")
	}

	err = kho.For(ctxXa(xa)).Tx(ctxXa(xa), func(tx *pkgstore.ScopedTx) error {
		// A parent id that exists only in another commune fails the composite foreign key.
		if err := s.Chen(ctxXa(xa), tx, domain.BoPhan{ID: "bp-3", Ten: "Y", Ma: "y", ChaID: "bp-k"}); !errors.Is(err, ErrBoPhanChaKhongTonTai) {
			t.Errorf("cha ở xã khác: %v, muốn ErrBoPhanChaKhongTonTai", err)
		}
		return errors.New("rollback")
	})
	if err == nil {
		t.Fatal("giao dịch không rollback")
	}

	err = kho.For(ctxXa(xa)).Tx(ctxXa(xa), func(tx *pkgstore.ScopedTx) error {
		if _, _, err := s.KhoaBoPhan(ctxXa(xa), tx, "bp-k"); !errors.Is(err, ErrKhongTimThayBoPhan) {
			t.Errorf("khoá dòng của xã khác: %v", err)
		}
		ma, err := s.MaCungGoc(ctxXa(xa), tx, "van-phong")
		if err != nil || len(ma) != 1 || ma[0] != "van-phong" {
			t.Errorf("MaCungGoc = %v, %v — chỉ mã của xã này", ma, err)
		}
		if err := s.Chen(ctxXa(xa), tx, domain.BoPhan{ID: "bp-4", Ten: "Z", Ma: "z", ChaID: "bp-1", ThuTu: 2}); err != nil {
			return err
		}
		bp, xoa, err := s.KhoaBoPhan(ctxXa(xa), tx, "bp-4")
		if err != nil || xoa || bp.ChaID != "bp-1" || bp.ThuTu != 2 {
			t.Errorf("đọc lại: %+v %v %v", bp, xoa, err)
		}
		bp.ChaID = ""
		return s.CapNhat(ctxXa(xa), tx, bp)
	})
	if err != nil {
		t.Fatal(err)
	}
}
