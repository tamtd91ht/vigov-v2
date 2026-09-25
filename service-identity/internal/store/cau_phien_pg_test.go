package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// Integration tests for the Mini App bridge's SQL (migration 0011, ADR 0045), against a real
// PostgreSQL. THEY SKIP WITHOUT VIGOV_TEST_DSN AND THE PACKAGE STILL PRINTS `ok` — look at the
// duration (see .env.example) before believing these ran.

var demZalo int

func maZaloPg() string {
	demZalo++
	return fmt.Sprintf("zalo-user-GIA-%010d-%d", demZalo, time.Now().UnixNano())
}

func taiKhoanPg(t *testing.T, db *sql.DB, xa, maZalo string) crosstenant.TaiKhoanZalo {
	t.Helper()
	var tk crosstenant.TaiKhoanZalo
	ctx := ctxXa(xa)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		tk, _, err = crosstenant.NewTaiKhoanZaloStore(db).TimHoacTao(ctx, tx.Underlying(), "app-1", maZalo)
		return err
	})
	if err != nil {
		t.Fatalf("tạo tài khoản Zalo: %v", err)
	}
	return tk
}

func moPhienCauPg(t *testing.T, db *sql.DB, xa string, p PhienCauMoi) (sid, token string) {
	t.Helper()
	ctx := ctxXa(xa)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		sid, token, _, err = soPhienCongDan(db).TaoQuaCau(ctx, tx, p)
		return err
	})
	if err != nil {
		t.Fatalf("mở phiên qua cầu: %v", err)
	}
	return sid, token
}

func TestPgPhienCauChuaCoSoTraCuuDuocVoiCongDanRong(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	tk := taiKhoanPg(t, db, xaA, maZaloPg())

	sid, token := moPhienCauPg(t, db, xaA, PhienCauMoi{TaiKhoanZaloID: tk.ID, ThoiHan: time.Hour})

	p, ok := soPhienCongDan(db).TraCuu(context.Background(), token)
	if !ok {
		t.Fatal("phiên chưa có số không tra cứu được — NULL cong_dan_id làm hỏng quetPhien")
	}
	if p.ID != sid || p.CitizenID != "" || string(p.TenantID) != xaA {
		t.Fatalf("phiên = %+v", p)
	}
}

func TestPgPhienKhongThuocAiBiCSDLTuChoi(t *testing.T) {
	// CHECK phien_cong_dan_co_chu_the: neither a citizen nor an account.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	_, err := db.Exec(`INSERT INTO phien_cong_dan (tenant_id, id, bam_token, nguon, het_han_luc)
		VALUES ($1, $2, $3, 'app', now() + interval '1 hour')`, xaA, "SID-KHONG-THUOC-AI", "bam-khong-thuoc-ai")
	if err == nil {
		t.Fatal("CSDL nhận một phiên không thuộc công dân nào lẫn tài khoản Zalo nào")
	}
}

func TestPgThuHoiTheoTaiKhoanZaloVuotXaChiPhienCuaTaiKhoanAy(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	cuaToi := taiKhoanPg(t, db, xaA, maZaloPg())
	nguoiKhac := taiKhoanPg(t, db, xaA, maZaloPg())

	_, tokCu := moPhienCauPg(t, db, xaA, PhienCauMoi{TaiKhoanZaloID: cuaToi.ID, ThoiHan: time.Hour})
	_, tokKhac := moPhienCauPg(t, db, xaA, PhienCauMoi{TaiKhoanZaloID: nguoiKhac.ID, ThoiHan: time.Hour})

	// The switch runs in commune B's transaction and must reach the session in commune A.
	var n int64
	ctx := ctxXa(xaB)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		n, err = soPhienCongDan(db).ThuHoiCuaTaiKhoanZalo(ctx, tx, cuaToi.ID, "đổi xã")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("thu hồi %d phiên, muốn 1", n)
	}
	if _, ok := soPhienCongDan(db).TraCuu(context.Background(), tokCu); ok {
		t.Fatal("phiên ở xã cũ vẫn dùng được sau khi đổi xã")
	}
	if _, ok := soPhienCongDan(db).TraCuu(context.Background(), tokKhac); !ok {
		t.Fatal("thu hồi theo tài khoản Zalo chạm cả phiên của NGƯỜI KHÁC")
	}
}

func TestPgTaiKhoanZaloTimHoacTaoNhoXaVaLienKet(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	so := crosstenant.NewTaiKhoanZaloStore(db)
	ma := maZaloPg()

	if _, co, err := so.Doc(context.Background(), "app-1", ma); err != nil || co {
		t.Fatalf("tài khoản chưa có mà Doc trả co=%v err=%v", co, err)
	}

	congDan := themCongDan(t, db)
	ctx := ctxXa(xaA)
	var id string
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		tk, moi, err := so.TimHoacTao(ctx, tx.Underlying(), "app-1", ma)
		if err != nil || !moi {
			return fmt.Errorf("lần đầu: moi=%v err=%w", moi, err)
		}
		id = tk.ID
		if err := so.TroToiDinhDanh(ctx, tx.Underlying(), tk.ID, congDan); err != nil {
			return err
		}
		return so.NhoXaCuaGiaoDich(ctx, tx, tk.ID)
	})
	if err != nil {
		t.Fatal(err)
	}

	tk, co, err := so.Doc(context.Background(), "app-1", ma)
	if err != nil || !co || tk.ID != id || tk.CongDanID != congDan || string(tk.XaDaNho) != xaA {
		t.Fatalf("đọc lại = %+v co=%v err=%v", tk, co, err)
	}
	// The raw Zalo id must not be in the table at all.
	var dem int
	if err := db.QueryRow(`SELECT count(*) FROM tai_khoan_zalo WHERE bam_zalo_user_id = $1`, ma).Scan(&dem); err != nil {
		t.Fatal(err)
	}
	if dem != 0 {
		t.Fatal("mã Zalo thô nằm trong cột băm")
	}
	// Same key through another app is another account (UNKNOWN #2 not measured).
	if _, co, _ := so.Doc(context.Background(), "app-2", ma); co {
		t.Fatal("cùng mã Zalo qua app khác trả về cùng tài khoản")
	}
}
