package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Integration tests for the citizen↔commune relationship, against a real PostgreSQL.
//
// WHY A REAL DATABASE: everything asserted here lives in the SQL and in the schema — that the
// commune predicate really separates two communes' rows, that `coalesce` really turns NULL into
// the empty string, that the CHECK constraints really refuse a rejection with no reason and a
// review with no reviewer. A fake driver agrees with whatever the Go code believes; only a
// server can contradict it.
//
// THEY SKIP WITHOUT VIGOV_TEST_DSN AND THE PACKAGE STILL PRINTS `ok`. Green here does NOT mean
// these assertions ran. Read the skip lines before believing this path is covered. The harness
// (TestMain, moKetNoi, xaRieng) lives in checker_pg_test.go; the DSN carries a password and
// lives only in the environment (rule 8).

func soQuanHe(db *sql.DB) *QuanHeCongDanXaStore {
	return NewQuanHeCongDanXaStore(pkgstore.New(db))
}

// themQuanHe inserts one relationship row, filling exactly the columns the CHECK constraints
// require for the state asked for.
func themQuanHe(t *testing.T, db *sql.DB, xa, congDanID string,
	khai domain.KhaiCuTru, tt domain.TrangThaiXacThuc, khaiLuc time.Time) {
	t.Helper()

	var boi, lyDo any
	var luc any
	if tt != domain.ChoXacThuc {
		boi, luc = "CB-XET-01", khaiLuc.Add(time.Hour)
	}
	if tt == domain.TuChoi {
		lyDo = "Địa chỉ khai không thuộc địa bàn xã."
	}
	_, err := db.Exec(
		`INSERT INTO quan_he_cong_dan_xa
		   (tenant_id, cong_dan_id, khai_cu_tru, nguon_khai, khai_luc,
		    trang_thai_xac_thuc, xet_duyet_boi, xet_duyet_luc, ly_do_tu_choi)
		 VALUES ($1,$2,$3,'cong_dan',$4,$5,$6,$7,$8)`,
		xa, congDanID, string(khai), khaiLuc, string(tt), boi, luc, lyDo)
	if err != nil {
		t.Fatalf("thêm quan hệ: %v", err)
	}
}

func TestPgQuanHeChiThayDongCuaXaMinh(t *testing.T) {
	// RULE 1. One citizen, two communes, two different declarations. Commune A must never see
	// B's row — and the shape that would leak it, a query without the commune predicate, still
	// returns rows and turns no test red anywhere else.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	cd := themCongDan(t, db)
	moc := time.Now().UTC().Add(-time.Hour)

	themQuanHe(t, db, xaA, cd, domain.KhaiThuongTru, domain.DaXacThuc, moc)
	themQuanHe(t, db, xaB, cd, domain.KhaiTamTru, domain.ChoXacThuc, moc)

	a, err := soQuanHe(db).TheoPhienCongDan(ctxCongDanTrongXa(t, xaA, cd))
	if err != nil {
		t.Fatalf("đọc quan hệ xã A: %v", err)
	}
	if a.Khai != domain.KhaiThuongTru || a.TrangThai != domain.DaXacThuc {
		t.Errorf("xã A đọc ra (%q, %q)", a.Khai, a.TrangThai)
	}

	b, err := soQuanHe(db).TheoPhienCongDan(ctxCongDanTrongXa(t, xaB, cd))
	if err != nil {
		t.Fatalf("đọc quan hệ xã B: %v", err)
	}
	if b.Khai != domain.KhaiTamTru || b.TrangThai != domain.ChoXacThuc {
		t.Errorf("xã B đọc ra (%q, %q)", b.Khai, b.TrangThai)
	}
}

func TestPgQuanHeCoalesceBienNullThanhChuoiRong(t *testing.T) {
	// The fake driver cannot decide this: it does not evaluate SQL, so `coalesce(xet_duyet_boi,
	// '')` is only really applied here. A NULL arriving in a plain string field is a scan error
	// at runtime — on the rows that make up the whole verification queue.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cd := themCongDan(t, db)
	themQuanHe(t, db, xa, cd, domain.KhaiChuaKhai, domain.ChoXacThuc, time.Now().UTC())

	q, err := soQuanHe(db).TheoPhienCongDan(ctxCongDanTrongXa(t, xa, cd))
	if err != nil {
		t.Fatalf("đọc quan hệ: %v", err)
	}
	if q.XetDuyetBoi != "" || q.LyDoTuChoi != "" {
		t.Errorf("NULL không thành chuỗi rỗng: %+v", q)
	}
	// xet_duyet_luc is NOT coalesced: a zero time.Time would read as 01/01/0001 on a screen.
	if q.XetDuyetLuc != nil {
		t.Errorf("XetDuyetLuc = %v, muốn nil", q.XetDuyetLuc)
	}
}

func TestPgQuanHeBoQuaDongDaXoaMem(t *testing.T) {
	// RULE 7, INVARIANT 2. Soft-deleted means kept and invisible; a read path that forgot the
	// predicate shows a relationship the commune has retired.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cd := themCongDan(t, db)
	themQuanHe(t, db, xa, cd, domain.KhaiTamTru, domain.ChoXacThuc, time.Now().UTC())

	if _, err := db.Exec(
		`UPDATE quan_he_cong_dan_xa SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND cong_dan_id = $2`,
		xa, cd, "CB-XOA-01", "kiểm thử"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	_, err := soQuanHe(db).TheoPhienCongDan(ctxCongDanTrongXa(t, xa, cd))
	if !errors.Is(err, ErrKhongCoQuanHe) {
		t.Fatalf("dòng đã xoá mềm vẫn đọc được, err = %v", err)
	}
}

func TestPgHangChoChiLayDongDangCho(t *testing.T) {
	// The queue is what an officer works from, oldest first. A verified or rejected row still in
	// it is work done twice; a waiting row missing from it is a citizen nobody answers.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cu, moi := themCongDan(t, db), themCongDan(t, db)
	xong := themCongDan(t, db)
	goc := time.Now().UTC().Add(-2 * time.Hour)

	themQuanHe(t, db, xa, moi, domain.KhaiTamTru, domain.ChoXacThuc, goc.Add(time.Hour))
	themQuanHe(t, db, xa, cu, domain.KhaiThuongTru, domain.ChoXacThuc, goc)
	themQuanHe(t, db, xa, xong, domain.KhaiThuongTru, domain.DaXacThuc, goc)

	ra, err := soQuanHe(db).HangChoXacThuc(ctxXa(xa), 10)
	if err != nil {
		t.Fatalf("đọc hàng chờ: %v", err)
	}
	if len(ra.Muc) != 2 {
		t.Fatalf("hàng chờ có %d dòng, muốn 2", len(ra.Muc))
	}
	if ra.Muc[0].CongDanID != cu {
		t.Errorf("dòng đầu là %q, muốn công dân chờ lâu nhất %q", ra.Muc[0].CongDanID, cu)
	}
	for _, m := range ra.Muc {
		if m.TrangThai != domain.ChoXacThuc {
			t.Errorf("hàng chờ lọt dòng trạng thái %q", m.TrangThai)
		}
	}
}

func TestPgQuanHeRangBuocGiuHaiSuThatNhatQuan(t *testing.T) {
	// THE SCHEMA IS THE LAST LINE OF DEFENCE, and these two constraints are the ones that stop a
	// dead end reaching a citizen: a rejection always carries a reason they can read (ADR 0023
	// §A2), and a row that has left the queue always names the officer who took it out.
	//
	// Asserted with raw INSERTs on purpose: the Go layer has no write path yet (open questions
	// #19 and #20), so the only thing that could enforce this today is the database — and
	// whether it does is exactly what a test on a real server can answer.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cd := themCongDan(t, db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO quan_he_cong_dan_xa
		   (tenant_id, cong_dan_id, khai_cu_tru, nguon_khai, trang_thai_xac_thuc,
		    xet_duyet_boi, xet_duyet_luc)
		 VALUES ($1,$2,'tam_tru','cong_dan','tu_choi','CB-01', now())`, xa, cd)
	if err == nil {
		t.Error("từ chối KHÔNG có lý do mà cơ sở dữ liệu vẫn nhận")
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO quan_he_cong_dan_xa
		   (tenant_id, cong_dan_id, khai_cu_tru, nguon_khai, trang_thai_xac_thuc)
		 VALUES ($1,$2,'tam_tru','cong_dan','da_xac_thuc')`, xa, cd)
	if err == nil {
		t.Error("đã xác thực mà KHÔNG có người xét duyệt, cơ sở dữ liệu vẫn nhận")
	}
}
