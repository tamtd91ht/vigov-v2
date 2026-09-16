package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/pkg/store"
)

func phienStore(db *sql.DB) *PhienStore { return NewPhienStore(pkgstore.New(db)) }

// moPhien opens a session for a staff member the caller has already created.
func moPhien(t *testing.T, db *sql.DB, xa, nguoiID string) (sid, refresh string) {
	t.Helper()
	s := phienStore(db)
	err := pkgstore.New(db).For(ctxXa(xa)).Tx(ctxXa(xa), func(tx *pkgstore.ScopedTx) error {
		var err error
		sid, refresh, err = s.Tao(ctxXa(xa), tx, nguoiID, "10.0.0.1", "test-agent")
		return err
	})
	if err != nil {
		t.Fatalf("mở phiên: %v", err)
	}
	return sid, refresh
}

func TestPhienMoRoiKiemTraDuoc(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})

	sid, refresh := moPhien(t, db, xaA, "nd-1")
	if sid == "" || refresh == "" {
		t.Fatal("Tao trả về mã rỗng")
	}
	if sid == refresh {
		t.Error("sid và refresh token giống nhau — phải là hai giá trị độc lập")
	}

	p, err := phienStore(db).KiemTra(ctxXa(xaA), sid)
	if err != nil {
		t.Fatalf("KiemTra lỗi: %v", err)
	}
	if p.NguoiDungID != "nd-1" {
		t.Errorf("phiên thuộc về %q, muốn nd-1", p.NguoiDungID)
	}
}

func TestPhienCuaXaNayKhongDungDuocOXaKhac(t *testing.T) {
	// A session opened against commune A's domain must be worthless at commune B's. This is the
	// property the whole session table exists to guarantee, and the one an attacker probes for.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	dungXa(t, db, xaB, "vt-1", "nd-1", []string{"task.read"})

	sid, _ := moPhien(t, db, xaA, "nd-1")

	if _, err := phienStore(db).KiemTra(ctxXa(xaB), sid); err == nil {
		t.Error("RÒ RỈ: phiên của xã A dùng được ở xã B")
	}
}

func TestPhienThuHoiThiHetHieuLuc(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	sid, _ := moPhien(t, db, xaA, "nd-1")

	s := phienStore(db)
	err := pkgstore.New(db).For(ctxXa(xaA)).Tx(ctxXa(xaA), func(tx *pkgstore.ScopedTx) error {
		return s.ThuHoi(ctxXa(xaA), tx, sid, "đăng xuất")
	})
	if err != nil {
		t.Fatalf("ThuHoi lỗi: %v", err)
	}

	if _, err := s.KiemTra(ctxXa(xaA), sid); !errors.Is(err, ErrPhienKhongTonTai) {
		t.Errorf("phiên đã thu hồi mà vẫn dùng được, lỗi = %v", err)
	}
}

func TestThuHoiCuaCanBoDongMoiPhien(t *testing.T) {
	// Required #7: locking an account, changing a role or changing a password must end EVERY
	// open session. Leaving one open means the old permissions keep working, which is the same
	// as not having made the change.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})

	sid1, _ := moPhien(t, db, xaA, "nd-1")
	sid2, _ := moPhien(t, db, xaA, "nd-1")
	sid3, _ := moPhien(t, db, xaA, "nd-1")

	s := phienStore(db)
	err := pkgstore.New(db).For(ctxXa(xaA)).Tx(ctxXa(xaA), func(tx *pkgstore.ScopedTx) error {
		return s.ThuHoiCuaCanBo(ctxXa(xaA), tx, "nd-1", "khoá tài khoản")
	})
	if err != nil {
		t.Fatalf("ThuHoiCuaCanBo lỗi: %v", err)
	}

	for i, sid := range []string{sid1, sid2, sid3} {
		if _, err := s.KiemTra(ctxXa(xaA), sid); err == nil {
			t.Errorf("phiên thứ %d vẫn còn hiệu lực sau khi khoá tài khoản", i+1)
		}
	}
}

func TestThuHoiKhongChamPhienCuaNguoiKhac(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	dungXa(t, db, xaA, "vt-2", "nd-2", []string{"task.read"})

	sidCuaA, _ := moPhien(t, db, xaA, "nd-1")
	sidCuaB, _ := moPhien(t, db, xaA, "nd-2")

	s := phienStore(db)
	err := pkgstore.New(db).For(ctxXa(xaA)).Tx(ctxXa(xaA), func(tx *pkgstore.ScopedTx) error {
		return s.ThuHoiCuaCanBo(ctxXa(xaA), tx, "nd-1", "khoá tài khoản")
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.KiemTra(ctxXa(xaA), sidCuaA); err == nil {
		t.Error("phiên của nd-1 phải bị thu hồi")
	}
	if _, err := s.KiemTra(ctxXa(xaA), sidCuaB); err != nil {
		t.Errorf("phiên của nd-2 bị thu hồi oan: %v", err)
	}
}

func TestSidKhongDoanDuoc(t *testing.T) {
	// A guessable sid is a session anyone can take over, and with it every permission its owner
	// holds. 256 bits of randomness, base64url-encoded, is 43 characters.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})

	thay := make(map[string]bool)
	for range 20 {
		sid, refresh := moPhien(t, db, xaA, "nd-1")
		if len(sid) < 40 {
			t.Fatalf("sid chỉ dài %d ký tự — quá ngắn để chống đoán", len(sid))
		}
		if thay[sid] {
			t.Fatal("sid lặp lại giữa hai phiên")
		}
		thay[sid] = true
		if thay[refresh] {
			t.Fatal("refresh token trùng với một giá trị đã cấp")
		}
		thay[refresh] = true
	}
}

func TestRefreshTokenKhongLuuNguyenVan(t *testing.T) {
	// A leaked backup must not hand over working sessions: only the SHA-256 of the refresh
	// token is stored.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	dungXa(t, db, xaA, "vt-1", "nd-1", []string{"task.read"})
	_, refresh := moPhien(t, db, xaA, "nd-1")

	var n int
	err := db.QueryRow(
		`SELECT count(*) FROM phien WHERE tenant_id = $1 AND refresh_hash = $2`,
		xaA, refresh).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("refresh token được lưu nguyên văn trong cơ sở dữ liệu")
	}

	// The hash of it, on the other hand, must be there.
	err = db.QueryRow(
		`SELECT count(*) FROM phien WHERE tenant_id = $1 AND refresh_hash = $2`,
		xaA, bamRefresh(refresh)).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("không tìm thấy phiên theo mã băm của refresh token")
	}
}

func TestPhienLaKhoaNgoaiToiCanBo(t *testing.T) {
	// A session pointing at a staff member who does not exist in this commune is a session that
	// could be minted for anyone. The composite foreign key is what prevents it.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)

	_, err := db.Exec(
		`INSERT INTO phien (tenant_id, id, nguoi_dung_id, refresh_hash, het_han_luc)
		 VALUES ($1,$2,$3,$4, now() + interval '1 hour')`,
		xaA, "sid-bia", "nd-khong-ton-tai", "bam")
	if err == nil {
		t.Error("mở được phiên cho một cán bộ không tồn tại")
	}
}

var _ = context.Background
