package store

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Tests that need no database, for the pairing code. The pg suite next door SKIPS without
// VIGOV_TEST_DSN while the package still prints `ok`, so everything decidable without a server
// — that the code is hashed before it is bound, that the TTL is computed by the database, that
// the single-use predicate is in the statement, that a refusing predicate is reported as a
// refusal — is decided here.
//
// WHAT THESE CANNOT DECIDE, and the pg suite must: that PostgreSQL really refuses a TTL over
// 120 seconds, that two racing redeems really serialise, that the foreign key really refuses a
// session from another commune. The fake driver executes no SQL; it records it.

const maGhepThu = "MG-01"

// mauHetHan is the one row `INSERT ... RETURNING het_han_luc` hands back. The fake driver reads
// the column names out of the RETURNING clause, so this is pinned to the real statement.
func mauHetHan() []map[string]driver.Value {
	return []map[string]driver.Value{{"het_han_luc": mocKhai.Add(ThoiHanMaGhep)}}
}

// khoGhep wires ONE *sql.DB over the fake, so the transaction the test opens and the store that
// writes inside it are the same handle — which is what production does, and what makes the
// recorded transaction number mean anything.
func khoGhep(k *khoKC) (*pkgstore.DB, *GhepPhienStore) {
	db := dbGia(k)
	return db, NewGhepPhienStore(db)
}

func taoMaGhep(t *testing.T, k *khoKC, ctx context.Context) (MaGhepDaTao, error) {
	t.Helper()
	var ra MaGhepDaTao
	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		ra, err = s.Tao(ctx, tx, MaGhepMoi{IP: "10.0.0.9", ThietBi: "man-hinh-mot-cua-01"})
		return err
	})
	return ra, err
}

// --- creating a code ----------------------------------------------------------------------------

func TestGhepPhienTaoBuocXaTuGiaoDichChuKhongPhaiThamSo(t *testing.T) {
	// RULE 1, INVARIANT 4. MaGhepMoi has no commune field: the commune comes from the ScopedTx,
	// which got it from the context. A caller holding commune A cannot mint a code for B.
	k := khoMoi()
	k.hang = mauHetHan()

	if _, err := taoMaGhep(t, k, ctxXa(xaKCD)); err != nil {
		t.Fatalf("Tao lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "INSERT INTO ghep_phien") {
		t.Fatalf("câu lệnh không phải chèn mã ghép: %q", l.sql)
	}
	if len(l.args) < 1 || l.args[0] != xaKCD {
		t.Fatalf("$1 = %v, muốn xã của giao dịch %q", l.args, xaKCD)
	}
	if l.tx == 0 {
		// Rule 6, invariant 3: the write has to be inside the caller's transaction, because
		// that is the transaction the audit entry shares. A write that ran outside it can
		// succeed while the entry fails.
		t.Error("chèn mã ghép chạy NGOÀI giao dịch của lời gọi")
	}
}

func TestGhepPhienTaoLuuMaDaBAMChuKhongLuuMa(t *testing.T) {
	// THE MOST IMPORTANT ASSERTION IN THIS FILE. The code is a bearer secret for two minutes:
	// whoever can read it can have a shared screen issued a session in somebody else's name.
	// Stored raw it would sit in every database backup and every read-only reporting account.
	k := khoMoi()
	k.hang = mauHetHan()

	ra, err := taoMaGhep(t, k, ctxXa(xaKCD))
	if err != nil {
		t.Fatalf("Tao lỗi: %v", err)
	}
	if ra.Ma == "" {
		t.Fatal("không trả mã nào cho màn hình")
	}
	l := k.cuoi()
	for i, a := range l.args {
		if s, ok := a.(string); ok && s == ra.Ma {
			t.Fatalf("mã thô đi vào tham số $%d — phải là mã băm", i+1)
		}
	}
	if strings.Contains(l.sql, ra.Ma) {
		t.Fatal("mã thô nằm trong chính câu lệnh")
	}
	tong := sha256.Sum256([]byte(ra.Ma))
	if len(l.args) < 3 || l.args[2] != hex.EncodeToString(tong[:]) {
		t.Errorf("$3 không phải SHA-256 hex của mã: %v", l.args[2])
	}
}

func TestGhepPhienTaoDeCoSoDuLieuTinhHanDung(t *testing.T) {
	// ADR 0019, invariant 1 fixes the TTL at 120 seconds and the schema enforces it
	// (ghep_phien_ttl_toi_da_120s). The expiry is computed FROM now() INSIDE the statement, so
	// tao_luc and het_han_luc share one clock — the database's. A Go-side timestamp would make
	// an app server running a few seconds fast unable to create a pairing at all, and only that
	// one server.
	k := khoMoi()
	k.hang = mauHetHan()

	if _, err := taoMaGhep(t, k, ctxXa(xaKCD)); err != nil {
		t.Fatalf("Tao lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "now() + make_interval(secs => $5)") {
		t.Errorf("hạn dùng không do cơ sở dữ liệu tính: %q", l.sql)
	}
	if len(l.args) < 5 || l.args[4] != 120.0 {
		t.Errorf("$5 = %v, muốn 120 giây (ADR 0019 bất biến 1)", l.args[4])
	}
	// The expiry handed back is the one the DATABASE stored, read through RETURNING, so the
	// screen counts down against the same clock the redeem compares with.
	if !strings.Contains(l.sql, "RETURNING het_han_luc") {
		t.Errorf("không đọc lại hạn dùng từ cơ sở dữ liệu: %q", l.sql)
	}
}

func TestGhepPhienTaoSinhBonKyTuDoiChieuKhongGayNhamMat(t *testing.T) {
	// ADR 0019, invariant 4: several identical-looking screens stand side by side at a one-stop
	// counter, and these four characters are the only thing a person can compare by eye. I, L,
	// O and U are out of the alphabet because a citizen who cannot tell O from 0 cannot perform
	// the one check this step exists for.
	k := khoMoi()
	k.hang = mauHetHan()

	ra, err := taoMaGhep(t, k, ctxXa(xaKCD))
	if err != nil {
		t.Fatalf("Tao lỗi: %v", err)
	}
	if len(ra.KyTuDoiChieu) != 4 {
		t.Fatalf("ký tự đối chiếu = %q, muốn đúng 4 ký tự", ra.KyTuDoiChieu)
	}
	for _, c := range ra.KyTuDoiChieu {
		if !strings.ContainsRune(chuCaiDoiChieu, c) {
			t.Errorf("ký tự %q ngoài bảng chữ đối chiếu", string(c))
		}
		if strings.ContainsRune("ILOU", c) {
			t.Errorf("ký tự %q dễ nhìn nhầm — không được nằm trong bảng", string(c))
		}
	}
	// The comparison characters are stored RAW and that is correct: they are not the secret.
	// They must, however, actually reach the row — both devices display them.
	if len(k.cuoi().args) < 4 || k.cuoi().args[3] != ra.KyTuDoiChieu {
		t.Errorf("$4 = %v, muốn ký tự đối chiếu %q", k.cuoi().args[3], ra.KyTuDoiChieu)
	}
}

func TestGhepPhienTaoMoiLanMotMaKhac(t *testing.T) {
	// A predictable code is a pairing anyone can take over. This does not measure entropy — it
	// catches the failure that actually happens, a generator wired to a constant or to a seed.
	k1, k2 := khoMoi(), khoMoi()
	k1.hang, k2.hang = mauHetHan(), mauHetHan()

	a, err := taoMaGhep(t, k1, ctxXa(xaKCD))
	if err != nil {
		t.Fatalf("Tao lỗi: %v", err)
	}
	b, err := taoMaGhep(t, k2, ctxXa(xaKCD))
	if err != nil {
		t.Fatalf("Tao lỗi: %v", err)
	}
	if a.Ma == b.Ma || a.ID == b.ID {
		t.Fatalf("hai lần tạo ra cùng một mã hoặc cùng một định danh")
	}
}

func TestGhepPhienTaoKhongCoGiaoDichThiTuChoi(t *testing.T) {
	// The audit entry (ADR 0019, invariant 6) has to share this write's transaction, so there
	// is no signature that writes without one.
	k := khoMoi()
	if _, err := NewGhepPhienStore(dbGia(k)).Tao(context.Background(), nil, MaGhepMoi{}); !errors.Is(err, ErrThieuGiaoDichGhep) {
		t.Fatalf("lỗi = %v, muốn ErrThieuGiaoDichGhep", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("không có giao dịch mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

// --- redeeming ------------------------------------------------------------------------------------

func TestGhepPhienDungMangDuBonDieuKienCuaMotLanDuyNhat(t *testing.T) {
	// SINGLE USE IS THE DATABASE'S JOB, and these four clauses are how it is asked to do it.
	// Two screens racing for one code serialise on the row lock and the loser matches nothing —
	// which a check-then-write in Go could not achieve.
	k := khoMoi()
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Dung(ctx, tx, maGhepThu, congDan1, "SID-MH-01")
	})
	if err != nil {
		t.Fatalf("Dung lỗi: %v", err)
	}
	l := k.cuoi()
	for _, manh := range []string{
		"UPDATE ghep_phien", "tenant_id = $1", "id = $2",
		"dung_luc IS NULL", "huy_luc IS NULL", "het_han_luc > now()",
	} {
		if !strings.Contains(l.sql, manh) {
			t.Errorf("câu lệnh dùng mã thiếu %q: %q", manh, l.sql)
		}
	}
	if l.args[0] != xaKCD || l.args[1] != maGhepThu || l.args[2] != congDan1 || l.args[3] != "SID-MH-01" {
		t.Errorf("tham số sai chỗ: %v", l.args)
	}
	if l.tx == 0 {
		t.Error("dùng mã chạy NGOÀI giao dịch của lời gọi")
	}
}

func TestGhepPhienDungKhongTrungDongNaoThiBaoKhongDungDuoc(t *testing.T) {
	// Zero rows is the predicate refusing, never an infrastructure failure — already used,
	// cancelled, expired, or another commune's code. The five cases are deliberately not told
	// apart here: distinguishing them needs a read before the write, which is the shape that
	// loses the race.
	k := khoMoi()
	k.soDong = 0
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Dung(ctx, tx, maGhepThu, congDan1, "SID-MH-01")
	})
	if !errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Fatalf("lỗi = %v, muốn ErrMaGhepKhongDungDuoc", err)
	}
}

func TestGhepPhienDungTrungNhieuDongThiKeuTo(t *testing.T) {
	// The predicate names the primary key, so two rows is impossible unless the key changed —
	// and a code that can be spent twice by one statement is the one thing this table exists to
	// prevent. It fails with an error that is NOT the ordinary sentinel, so a caller handling
	// "code no longer usable" cannot swallow it.
	k := khoMoi()
	k.soDong = 2
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Dung(ctx, tx, maGhepThu, congDan1, "SID-MH-01")
	})
	if err == nil {
		t.Fatal("hai dòng bị đổi mà không báo lỗi")
	}
	if errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Fatalf("lỗi = %v, không được nhận nhầm là mã hết dùng được", err)
	}
}

func TestGhepPhienDungThieuThamSoThiTuChoiTruocKhiChamCoSoDuLieu(t *testing.T) {
	// Checked before the transaction is touched, which is what lets these refusals be tested on
	// a machine with no database — the machine this repository is usually built on.
	k := khoMoi()
	db, s := khoGhep(k)
	ctx := ctxXa(xaKCD)

	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := s.Dung(ctx, tx, "", congDan1, "SID-MH-01"); !errors.Is(err, ErrThieuMaGhep) {
			t.Errorf("thiếu mã: lỗi = %v, muốn ErrThieuMaGhep", err)
		}
		if err := s.Dung(ctx, tx, maGhepThu, "", "SID-MH-01"); !errors.Is(err, ErrThieuPhienRa) {
			t.Errorf("thiếu công dân: lỗi = %v, muốn ErrThieuPhienRa", err)
		}
		if err := s.Dung(ctx, tx, maGhepThu, congDan1, ""); !errors.Is(err, ErrThieuPhienRa) {
			t.Errorf("thiếu phiên: lỗi = %v, muốn ErrThieuPhienRa", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("giao dịch lỗi: %v", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("tham số thiếu mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

// --- cancelling ------------------------------------------------------------------------------------

func TestGhepPhienHuyKhongDongToiMaDaDung(t *testing.T) {
	// `dung_luc IS NULL` IS THE LOAD-BEARING CLAUSE. Without it a code that has already produced
	// a session could be marked cancelled, and the row — which ADR 0019, invariant 6 keeps as
	// evidence that a citizen authorised a specific screen at a specific time — would say the
	// pairing never happened.
	k := khoMoi()
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Huy(ctx, tx, maGhepThu, "màn hình đóng phiên ghép")
	})
	if err != nil {
		t.Fatalf("Huy lỗi: %v", err)
	}
	l := k.cuoi()
	for _, manh := range []string{
		"UPDATE ghep_phien", "tenant_id = $1", "id = $2", "huy_luc IS NULL", "dung_luc IS NULL",
	} {
		if !strings.Contains(l.sql, manh) {
			t.Errorf("câu lệnh huỷ thiếu %q: %q", manh, l.sql)
		}
	}
	if l.tx == 0 {
		t.Error("huỷ mã chạy NGOÀI giao dịch của lời gọi")
	}
}

func TestGhepPhienHuyPhaiCoLyDo(t *testing.T) {
	// The same discipline as revoking a session: the audit entry the caller writes beside this
	// is only as good as what it can say about why. The schema allows NULL here; this layer
	// does not.
	k := khoMoi()
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := s.Huy(ctx, tx, maGhepThu, "   "); !errors.Is(err, ErrThieuLyDoHuy) {
			t.Errorf("lỗi = %v, muốn ErrThieuLyDoHuy", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("giao dịch lỗi: %v", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("huỷ không lý do mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func TestGhepPhienHuyKhongTrungDongNaoThiBaoKhongHuyDuoc(t *testing.T) {
	// Already used, already cancelled, or not this commune's code. It is an error rather than a
	// silent no-op because of the first case: a screen that has already been issued a session
	// is NOT un-paired by cancelling the code, and a caller told "cancelled" would report to a
	// citizen that a screen was logged out when it was not.
	k := khoMoi()
	k.soDong = 0
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Huy(ctx, tx, maGhepThu, "màn hình đóng phiên ghép")
	})
	if !errors.Is(err, ErrMaGhepKhongHuyDuoc) {
		t.Fatalf("lỗi = %v, muốn ErrMaGhepKhongHuyDuoc", err)
	}
}

// --- failures ---------------------------------------------------------------------------------------

func TestGhepPhienLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := khoMoi()
	k.loi = goc
	ctx := ctxXa(xaKCD)

	db, s := khoGhep(k)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Dung(ctx, tx, maGhepThu, congDan1, "SID-MH-01")
	})
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrMaGhepKhongDungDuoc) {
		t.Error("lỗi kho bị nhận nhầm là mã hết dùng được")
	}
}

func TestGhepPhienTaoKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY: a pairing code written without a commune would belong to no
	// authority, and the screen it pairs would be in no commune either.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("tạo mã ghép khi context không có xã mà không panic")
		}
	}()
	k := khoMoi()
	k.hang = mauHetHan()
	_, _ = taoMaGhep(t, k, context.Background())
}
