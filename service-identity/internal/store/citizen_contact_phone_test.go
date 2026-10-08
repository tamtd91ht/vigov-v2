package store

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The refusals ContactPhoneForReveal makes BEFORE the database — testable on every machine.
func TestContactPhoneForRevealRefusesEmptyKeysAndNoTx(t *testing.T) {
	s := &PhienCongDanStore{}
	ctx := context.Background()
	for name, c := range map[string][2]string{
		"no session": {"", "CD-1"},
		"no citizen": {"SID-1", ""},
	} {
		if _, err := s.ContactPhoneForReveal(ctx, nil, c[0], c[1]); !errors.Is(err, ErrNoContactPhone) {
			t.Errorf("%s: err = %v, want ErrNoContactPhone — an empty key must never reach the query", name, err)
		}
	}
	if _, err := s.ContactPhoneForReveal(ctx, nil, "SID-1", "CD-1"); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("no transaction: err = %v, want ErrThieuGiaoDich", err)
	}
}

// The predicate against a REAL PostgreSQL: the six "nothing to disclose" cases of the contract and
// the one that discloses. Skipped without VIGOV_TEST_DSN — read the skip line before believing it ran.
func TestContactPhoneForRevealPg(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	s := soPhienCongDan(db)

	read := func(xa, sid, citizen string) (string, error) {
		t.Helper()
		ctx := ctxXa(xa)
		var phone string
		err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			var err error
			phone, err = s.ContactPhoneForReveal(ctx, tx, sid, citizen)
			return err
		})
		return phone, err
	}

	citizen := themCongDan(t, db)
	other := themCongDan(t, db)
	var want string
	if err := db.QueryRow(`SELECT so_dien_thoai FROM dinh_danh_cong_dan WHERE id = $1`, citizen).Scan(&want); err != nil {
		t.Fatal(err)
	}

	live, _ := moPhienCongDan(t, db, xaA, citizen)
	if got, err := read(xaA, live, citizen); err != nil || got != want {
		t.Fatalf("live session of the citizen: got %q, err %v — want the verified number", got, err)
	}

	revoked, _ := moPhienCongDan(t, db, xaA, citizen)
	ctx := ctxXa(xaA)
	if err := pkgstore.New(db).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.ThuHoi(ctx, tx, revoked, "công dân đăng xuất")
	}); err != nil {
		t.Fatal(err)
	}

	expired, _ := moPhienCongDan(t, db, xaA, citizen)
	// Both columns moved back — CHECK (het_han_luc > tao_luc), see TestPhienHetHanKhongTraCuuDuoc.
	if _, err := db.Exec(`UPDATE phien_cong_dan SET tao_luc = now() - interval '2 hours',
		het_han_luc = now() - interval '1 minute' WHERE tenant_id = $1 AND id = $2`, xaA, expired); err != nil {
		t.Fatal(err)
	}

	inB, _ := moPhienCongDan(t, db, xaB, citizen)

	tk := taiKhoanPg(t, db, xaA, maZaloPg())
	unverified, _ := moPhienCauPg(t, db, xaA, PhienCauMoi{TaiKhoanZaloID: tk.ID, ThoiHan: time.Hour})

	for name, c := range map[string][2]string{
		"unknown session":                {"SID-KHONG-TON-TAI", citizen},
		"revoked session":                {revoked, citizen},
		"expired session":                {expired, citizen},
		"another commune's session":      {inB, citizen},
		"session with no verified phone": {unverified, citizen},
		"session of another citizen":     {live, other},
	} {
		got, err := read(xaA, c[0], c[1])
		if !errors.Is(err, ErrNoContactPhone) || got != "" {
			t.Errorf("%s: got %q, err %v — want ErrNoContactPhone and nothing", name, got, err)
		}
	}
}
