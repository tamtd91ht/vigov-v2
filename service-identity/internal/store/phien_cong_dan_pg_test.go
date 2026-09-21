package store

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Integration tests for the citizen session registry, against a real PostgreSQL.
//
// WHY A REAL DATABASE: everything that matters here is in the SQL — the single WHERE that makes
// unknown, expired and revoked indistinguishable, the comparison against now() that is not a
// stored flag, and the fact that a revocation in commune A does not touch commune B. A mock
// would agree with whatever the query says.
//
// THEY SKIP WITHOUT VIGOV_TEST_DSN AND THE PACKAGE STILL PRINTS `ok`. Green here does NOT mean
// these assertions ran. Read the skip lines before believing this path is covered.

func soPhienCongDan(db *sql.DB) *PhienCongDanStore {
	return NewPhienCongDanStore(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// demCongDan makes every citizen in this suite distinct WITHOUT depending on the clock.
//
// dinh_danh_cong_dan is unique on the phone number PLATFORM-WIDE and these tests share one
// schema, so a value derived from time.Now() collides whenever two calls land inside one clock
// tick — which on Windows is a millisecond, not a nanosecond. A counter cannot.
var demCongDan atomic.Int64

// themCongDan inserts one citizen identity and returns its id.
//
// The number is fake and obviously so (rule 3, invariant 5). The agreed constant 0900000000 is
// not used directly because the uniqueness above needs one number per citizen; every value here
// stays in the same obviously-fake range.
func themCongDan(t *testing.T, db *sql.DB) string {
	t.Helper()
	n := demCongDan.Add(1)
	id := fmt.Sprintf("CD%024d", n) // 26 characters — CHECK dinh_danh_cong_dan_id_la_ulid
	if _, err := db.Exec(
		`INSERT INTO dinh_danh_cong_dan (id, so_dien_thoai) VALUES ($1,$2)`,
		id, fmt.Sprintf("09%08d", n)); err != nil {
		t.Fatalf("thêm định danh công dân: %v", err)
	}
	return id
}

// moPhienCongDan opens a session for a citizen in one commune and returns the sid and token.
func moPhienCongDan(t *testing.T, db *sql.DB, xa, congDanID string) (sid, token string) {
	t.Helper()
	s := soPhienCongDan(db)
	ctx := ctxXa(xa)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		sid, token, err = s.Tao(ctx, tx, PhienMoi{
			CongDanID: congDanID,
			Nguon:     NguonApp,
			ThoiHan:   time.Hour,
			IP:        "10.0.0.1",
			ThietBi:   "test-agent",
		})
		return err
	})
	if err != nil {
		t.Fatalf("mở phiên công dân: %v", err)
	}
	return sid, token
}

func TestPhienCongDanTraCuuDuocBangToken(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)

	sid, token := moPhienCongDan(t, db, xaA, congDan)
	if sid == "" || token == "" {
		t.Fatal("Tao trả về giá trị rỗng")
	}
	if sid == token {
		t.Error("sid và token giống nhau — phải là hai giá trị độc lập")
	}

	p, ok := soPhienCongDan(db).TraCuu(context.Background(), token)
	if !ok {
		t.Fatal("phiên vừa mở mà tra cứu không ra")
	}
	if p.ID != sid {
		t.Errorf("ID = %q, muốn %q", p.ID, sid)
	}
	if p.CitizenID != congDan {
		t.Errorf("CitizenID = %q, muốn %q", p.CitizenID, congDan)
	}
	if string(p.TenantID) != xaA {
		t.Errorf("TenantID = %q, muốn %q", p.TenantID, xaA)
	}
}

func TestTraCuuKhongCanXaTrongNguCanh(t *testing.T) {
	// THE PROPERTY THE WHOLE CITIZEN EDGE RESTS ON (ADR 0022): the commune is DERIVED from the
	// session, so the lookup must work on a context that carries no commune at all. If this ever
	// needs a commune, something has started taking it from the request — which rule 1,
	// forbidden #2 closes.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)
	_, token := moPhienCongDan(t, db, xaA, congDan)

	p, ok := soPhienCongDan(db).TraCuu(context.Background(), token)
	if !ok || string(p.TenantID) != xaA {
		t.Errorf("tra cứu không có xã trong ngữ cảnh: ok=%v, xã=%q", ok, p.TenantID)
	}
}

func TestTokenKhongLuuNguyenVan(t *testing.T) {
	// A leaked backup must not hand over working sessions. Only the SHA-256 of the token is
	// stored, exactly as the staff registry does with its refresh token.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)
	_, token := moPhienCongDan(t, db, xaA, congDan)

	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM phien_cong_dan WHERE tenant_id = $1 AND bam_token = $2`,
		xaA, token).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("token được lưu nguyên văn trong cơ sở dữ liệu")
	}

	if err := db.QueryRow(
		`SELECT count(*) FROM phien_cong_dan WHERE tenant_id = $1 AND bam_token = $2`,
		xaA, bamRefresh(token)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("không tìm thấy phiên theo mã băm của token")
	}
}

func TestPhienHetHanKhongTraCuuDuoc(t *testing.T) {
	// "Đã hết hạn" is a comparison against the clock, never a column (rule 10, invariant 3).
	// The expiry is moved into the past directly so the test does not depend on the app clock
	// and the database clock agreeing.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)
	sid, token := moPhienCongDan(t, db, xaA, congDan)

	// CẢ HAI cột lùi về quá khứ, không chỉ `het_han_luc`. Lược đồ giữ
	// `CHECK (het_han_luc > tao_luc)` (`migrations/0004_kenh_cong_dan.sql:372`), nên một phiên
	// "hết hạn" phải là phiên ĐƯỢC TẠO SỚM HƠN NỮA — không phải phiên có hạn nằm trước lúc tạo,
	// thứ không đường mã sản xuất nào tạo ra được (thu hồi dùng cột `thu_hoi_luc` riêng).
	// Mẫu thử cũ vi phạm chính ràng buộc ấy, và nó sống sót vì ca này chưa từng gặp PostgreSQL.
	if _, err := db.Exec(
		`UPDATE phien_cong_dan
		    SET tao_luc     = now() - interval '2 hours',
		        het_han_luc = now() - interval '1 minute'
		 WHERE tenant_id = $1 AND id = $2`, xaA, sid); err != nil {
		t.Fatal(err)
	}

	if _, ok := soPhienCongDan(db).TraCuu(context.Background(), token); ok {
		t.Error("phiên đã hết hạn mà vẫn tra cứu ra")
	}
}

func TestPhienThuHoiKhongTraCuuDuoc(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)
	sid, token := moPhienCongDan(t, db, xaA, congDan)

	s := soPhienCongDan(db)
	ctx := ctxXa(xaA)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.ThuHoi(ctx, tx, sid, "công dân đăng xuất")
	})
	if err != nil {
		t.Fatalf("ThuHoi lỗi: %v", err)
	}

	if _, ok := s.TraCuu(context.Background(), token); ok {
		t.Error("phiên đã thu hồi mà vẫn dùng được")
	}
}

func TestThuHoiCuaCongDanChiDongTrongXaCuaMinh(t *testing.T) {
	// A citizen deals with several communes at once (ADR 0002). Commune B ending their own
	// screen session must not end the citizen's session with commune A — that would be one
	// public authority acting on another's relationship with a citizen.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	congDan := themCongDan(t, db)

	_, tokenA := moPhienCongDan(t, db, xaA, congDan)
	_, tokenB1 := moPhienCongDan(t, db, xaB, congDan)
	_, tokenB2 := moPhienCongDan(t, db, xaB, congDan)

	s := soPhienCongDan(db)
	ctx := ctxXa(xaB)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.ThuHoiCuaCongDan(ctx, tx, congDan, "đăng xuất màn hình dùng chung")
	})
	if err != nil {
		t.Fatalf("ThuHoiCuaCongDan lỗi: %v", err)
	}

	for i, tok := range []string{tokenB1, tokenB2} {
		if _, ok := s.TraCuu(context.Background(), tok); ok {
			t.Errorf("phiên thứ %d ở xã B vẫn còn hiệu lực sau khi thu hồi", i+1)
		}
	}
	if _, ok := s.TraCuu(context.Background(), tokenA); !ok {
		t.Error("RÒ RỈ NGƯỢC: xã B thu hồi phiên mà phiên của công dân ở xã A cũng tắt")
	}
}

func TestPhienChuaChonXaDungDuocNhungKhongMangXa(t *testing.T) {
	// ADR 0005 separates discovery from session: the citizen signs in, then chooses a commune.
	// The registry must issue and resolve that session; core/httpx.XaTuPhien is what refuses it
	// on any business route.
	db := moKetNoi(t)
	congDan := themCongDan(t, db)
	s := soPhienCongDan(db)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	sid, token, err := s.TaoChuaChonXa(context.Background(), tx, PhienMoi{
		CongDanID: congDan, Nguon: NguonApp, ThoiHan: time.Hour,
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("TaoChuaChonXa lỗi: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	p, ok := s.TraCuu(context.Background(), token)
	if !ok {
		t.Fatal("phiên chưa chọn xã tra cứu không ra")
	}
	if p.ID != sid || p.CitizenID != congDan {
		t.Errorf("phiên trả về sai: ID=%q CitizenID=%q", p.ID, p.CitizenID)
	}
	if p.TenantID.Valid() {
		t.Errorf("TenantID = %q — phiên chưa chọn xã phải rỗng", p.TenantID)
	}

	// And it must be closable, or TaoChuaChonXa is a one-way door.
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ThuHoiChuaChonXa(context.Background(), tx2, sid, "đã chọn xã, phát hành phiên mới"); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("ThuHoiChuaChonXa lỗi: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.TraCuu(context.Background(), token); ok {
		t.Error("phiên chưa chọn xã đã thu hồi mà vẫn dùng được")
	}
}

func TestGiaoDichHuyThiKhongCoPhien(t *testing.T) {
	// Rule 6, invariant 3: the session row and the audit entry for the sign-in live or die
	// together. A session that committed on its own would be a citizen logged in with no trail
	// saying when or from where.
	db := moKetNoi(t)
	congDan := themCongDan(t, db)
	s := soPhienCongDan(db)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.TaoChuaChonXa(context.Background(), tx, PhienMoi{
		CongDanID: congDan, Nguon: NguonApp, ThoiHan: time.Hour,
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	if _, ok := s.TraCuu(context.Background(), token); ok {
		t.Error("phiên vẫn dùng được sau khi giao dịch của lời gọi bị huỷ")
	}
}

func TestTokenKhongDoanDuoc(t *testing.T) {
	// A guessable token is a session anyone can take over, and a citizen session is the key to
	// one person's own administrative files. 256 bits, base64url, is 43 characters.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)

	thay := make(map[string]bool)
	for range 20 {
		sid, token := moPhienCongDan(t, db, xaA, congDan)
		if len(token) < 40 {
			t.Fatalf("token chỉ dài %d ký tự — quá ngắn để chống đoán", len(token))
		}
		if thay[token] || thay[sid] {
			t.Fatal("giá trị lặp lại giữa hai phiên")
		}
		thay[token] = true
		thay[sid] = true
	}
}

func TestGhiNhanDungCapNhatDungGanNhat(t *testing.T) {
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)
	sid, token := moPhienCongDan(t, db, xaA, congDan)

	s := soPhienCongDan(db)
	p, ok := s.TraCuu(context.Background(), token)
	if !ok {
		t.Fatal("tra cứu không ra")
	}
	s.GhiNhanDung(context.Background(), p)

	var dung *time.Time
	if err := db.QueryRow(
		`SELECT dung_gan_nhat FROM phien_cong_dan WHERE tenant_id = $1 AND id = $2`,
		xaA, sid).Scan(&dung); err != nil {
		t.Fatal(err)
	}
	if dung == nil {
		t.Error("dung_gan_nhat vẫn rỗng sau khi ghi nhận")
	}
}

func TestNguonKhongHopLeKhongBaoGioChamCSDL(t *testing.T) {
	// The CHECK constraint is the backstop, not the gate: a constraint violation arrives as a
	// driver error no caller can act on, and this field decides how much a session is trusted.
	db := moKetNoi(t)
	xaA, _ := xaRieng(t)
	congDan := themCongDan(t, db)

	s := soPhienCongDan(db)
	ctx := ctxXa(xaA)
	err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, _, err := s.Tao(ctx, tx, PhienMoi{
			CongDanID: congDan, Nguon: "web", ThoiHan: time.Hour,
		})
		return err
	})
	if err == nil {
		t.Error("nguồn phiên không hợp lệ mà vẫn tạo được phiên")
	}
}
