package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for the pairing code, against a real PostgreSQL.
//
// WHY A REAL DATABASE: the three properties this store leans on are all in the schema, and a
// fake driver can only agree with what the Go code believes about them —
//
//	single use        UPDATE ... WHERE dung_luc IS NULL, with two calls racing for one row
//	the 120s cap      CONSTRAINT ghep_phien_ttl_toi_da_120s
//	commune matching  FOREIGN KEY (tenant_id, phien_id) — ADR 0019, invariant 8 backed by a
//	                  constraint rather than only by a check in Go
//
// THEY SKIP WITHOUT VIGOV_TEST_DSN AND THE PACKAGE STILL PRINTS `ok`. Green here does NOT mean
// these assertions ran. The harness lives in checker_pg_test.go and phien_cong_dan_pg_test.go.

func soGhepPhien(db *sql.DB) *GhepPhienStore {
	return NewGhepPhienStore(pkgstore.New(db))
}

// taoMaGhepThat creates one pairing code through the store, in a committed transaction — the
// way a use case would, with its audit entry alongside.
func taoMaGhepThat(t *testing.T, db *sql.DB, xa string) MaGhepDaTao {
	t.Helper()
	ctx := ctxXa(xa)
	var ra MaGhepDaTao
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		ra, err = soGhepPhien(db).Tao(ctx, tx, MaGhepMoi{IP: "10.0.0.9", ThietBi: "man-hinh-01"})
		return err
	})
	if err != nil {
		t.Fatalf("tạo mã ghép: %v", err)
	}
	return ra
}

// dungMaGhep redeems through the store and returns the error, WITHOUT failing the test: every
// interesting case here is a refusal.
func dungMaGhep(db *sql.DB, xa, id, congDanID, phienID string) error {
	ctx := ctxXa(xa)
	return pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return soGhepPhien(db).Dung(ctx, tx, id, congDanID, phienID)
	})
}

func huyMaGhep(db *sql.DB, xa, id, lyDo string) error {
	ctx := ctxXa(xa)
	return pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return soGhepPhien(db).Huy(ctx, tx, id, lyDo)
	})
}

func TestPgGhepPhienLuuBamVaHanDungDoCoSoDuLieuTinh(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	ra := taoMaGhepThat(t, db, xa)

	var bam, kyTu string
	var giay float64
	var dung, huy *time.Time
	err := db.QueryRow(
		`SELECT bam_ma, ky_tu_doi_chieu, extract(epoch FROM het_han_luc - tao_luc),
		        dung_luc, huy_luc
		 FROM ghep_phien WHERE tenant_id = $1 AND id = $2`, xa, ra.ID).
		Scan(&bam, &kyTu, &giay, &dung, &huy)
	if err != nil {
		t.Fatalf("đọc lại mã ghép: %v", err)
	}

	// THE CODE IS STORED HASHED — the single most important property of this table.
	tong := sha256.Sum256([]byte(ra.Ma))
	if bam != hex.EncodeToString(tong[:]) {
		t.Error("cột bam_ma không phải SHA-256 của mã đã phát")
	}
	if bam == ra.Ma {
		t.Fatal("mã được lưu THÔ — mọi bản sao lưu cầm được quyền ghép phiên")
	}
	if len(kyTu) != 4 {
		t.Errorf("ký tự đối chiếu %q, muốn đúng 4 ký tự", kyTu)
	}
	if giay != ThoiHanMaGhep.Seconds() {
		t.Errorf("hạn dùng dài %v giây, muốn %v (ADR 0019 bất biến 1)", giay, ThoiHanMaGhep.Seconds())
	}
	if dung != nil || huy != nil {
		t.Error("mã vừa tạo mà đã mang dấu đã dùng hoặc đã huỷ")
	}
	if ra.HetHanLuc.IsZero() {
		t.Error("không đọc lại được hạn dùng từ cơ sở dữ liệu")
	}
}

func TestPgGhepPhienChiDungDuocMotLan(t *testing.T) {
	// SINGLE USE IS THE DATABASE'S JOB. Two screens racing for one code serialise on the row
	// lock and the loser matches nothing — which is what this asserts, one call after the other.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cd := themCongDan(t, db)
	sid, _ := moPhienCongDan(t, db, xa, cd)

	ra := taoMaGhepThat(t, db, xa)

	if err := dungMaGhep(db, xa, ra.ID, cd, sid); err != nil {
		t.Fatalf("lần dùng đầu tiên phải thành công: %v", err)
	}
	if err := dungMaGhep(db, xa, ra.ID, cd, sid); !errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Fatalf("lần dùng thứ hai: lỗi = %v, muốn ErrMaGhepKhongDungDuoc", err)
	}

	// All three facts landed together — CONSTRAINT ghep_phien_dung_thi_du_ba_thu makes
	// "redeemed by nobody" impossible, and this checks the row really carries them.
	var congDan, phien string
	if err := db.QueryRow(
		`SELECT cong_dan_id, phien_id FROM ghep_phien WHERE tenant_id = $1 AND id = $2`,
		xa, ra.ID).Scan(&congDan, &phien); err != nil {
		t.Fatalf("đọc lại mã đã dùng: %v", err)
	}
	if congDan != cd || phien != sid {
		t.Errorf("mã đã dùng ghi (%q, %q), muốn (%q, %q)", congDan, phien, cd, sid)
	}
}

func TestPgGhepPhienDaDungThiKhongHuyDuocVaNguocLai(t *testing.T) {
	// A screen that has already been issued a session is NOT un-paired by cancelling the code,
	// and the row is evidence that a citizen authorised that screen (ADR 0019, invariant 6).
	// Marking it cancelled would rewrite that evidence.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cd := themCongDan(t, db)
	sid, _ := moPhienCongDan(t, db, xa, cd)

	daDung := taoMaGhepThat(t, db, xa)
	if err := dungMaGhep(db, xa, daDung.ID, cd, sid); err != nil {
		t.Fatalf("dùng mã: %v", err)
	}
	if err := huyMaGhep(db, xa, daDung.ID, "màn hình đóng"); !errors.Is(err, ErrMaGhepKhongHuyDuoc) {
		t.Errorf("huỷ mã đã dùng: lỗi = %v, muốn ErrMaGhepKhongHuyDuoc", err)
	}

	daHuy := taoMaGhepThat(t, db, xa)
	if err := huyMaGhep(db, xa, daHuy.ID, "công dân bỏ giữa chừng"); err != nil {
		t.Fatalf("huỷ mã: %v", err)
	}
	if err := dungMaGhep(db, xa, daHuy.ID, cd, sid); !errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Errorf("dùng mã đã huỷ: lỗi = %v, muốn ErrMaGhepKhongDungDuoc", err)
	}
}

func TestPgGhepPhienKhongDungDuocMaCuaXaKhac(t *testing.T) {
	// RULE 1. The code belongs to commune A; commune B holding its id still matches no row,
	// because the UPDATE binds tenant_id from the transaction. This is the last of the three
	// layers that keep a pairing inside one commune — the other two are the foreign key below
	// and the comparison the use case makes (ADR 0019, invariant 8).
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	cd := themCongDan(t, db)
	sidB, _ := moPhienCongDan(t, db, xaB, cd)

	ra := taoMaGhepThat(t, db, xaA)

	if err := dungMaGhep(db, xaB, ra.ID, cd, sidB); !errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Fatalf("xã B dùng được mã của xã A: lỗi = %v", err)
	}
}

func TestPgGhepPhienKhoaNgoaiTuChoiPhienCuaXaKhac(t *testing.T) {
	// ADR 0019, INVARIANT 8 BACKED BY A CONSTRAINT. Even inside commune A's own transaction, a
	// session id belonging to commune B has no target for FOREIGN KEY (tenant_id, phien_id), so
	// the database itself refuses the pairing. It is NOT the ordinary sentinel: nothing about
	// the code was wrong, and a caller must not report "code no longer usable".
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	cd := themCongDan(t, db)
	sidB, _ := moPhienCongDan(t, db, xaB, cd)

	ra := taoMaGhepThat(t, db, xaA)

	err := dungMaGhep(db, xaA, ra.ID, cd, sidB)
	if err == nil {
		t.Fatal("ghép được phiên của xã khác — khoá ngoại không giữ")
	}
	if errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Errorf("lỗi khoá ngoại bị nhận nhầm là mã hết dùng được: %v", err)
	}
}

func TestPgGhepPhienMaHetHanThiKhongDungDuoc(t *testing.T) {
	// "EXPIRED" IS A COMPARISON, NEVER A COLUMN (rule 10, invariant 3). The row is written with
	// a tao_luc in the past — the only way to reach this state without waiting two minutes — and
	// the redeem's `het_han_luc > now()` is what refuses it.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	cd := themCongDan(t, db)
	sid, _ := moPhienCongDan(t, db, xa, cd)

	_, err := db.Exec(
		`INSERT INTO ghep_phien (tenant_id, id, bam_ma, ky_tu_doi_chieu, tao_luc, het_han_luc)
		 VALUES ($1,$2,$3,'K7QM', now() - interval '10 minutes',
		         now() - interval '10 minutes' + interval '120 seconds')`,
		xa, "GP-HET-HAN", hex.EncodeToString([]byte("bam-gia-het-han-0000000000000000")))
	if err != nil {
		t.Fatalf("thêm mã đã hết hạn: %v", err)
	}

	if err := dungMaGhep(db, xa, "GP-HET-HAN", cd, sid); !errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Fatalf("dùng được mã đã hết hạn: lỗi = %v", err)
	}
}

func TestPgGhepPhienCoSoDuLieuTuChoiTTLQua120Giay(t *testing.T) {
	// The cap is enforced where it cannot be forgotten by a caller. ADR 0019 decided the number,
	// so unlike the paired SESSION's TTL (still open, §CÒN MỞ #1) it belongs in the schema.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO ghep_phien (tenant_id, id, bam_ma, ky_tu_doi_chieu, het_han_luc)
		 VALUES ($1,'GP-QUA-HAN',$2,'K7QM', now() + interval '121 seconds')`,
		xa, hex.EncodeToString([]byte("bam-gia-qua-han-0000000000000000")))
	if err == nil {
		t.Error("cơ sở dữ liệu nhận mã ghép sống quá 120 giây")
	}
}
