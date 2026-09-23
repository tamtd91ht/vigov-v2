package store

import (
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// Integration tests for the WRITE path of `chung_tu_giai_ngan.nguon_von_id` — against a real
// PostgreSQL.
//
// WHY A REAL DATABASE, WHEN internal/app ALREADY EXERCISES THIS STORE OVER A FAKE DRIVER: the fake
// agrees with whatever the statement says. The two properties below are properties of the SCHEMA,
// and each one is a defect that a green fake-driver run would report as healthy:
//
//	`CHECK (nguon_von_id IS NULL OR btrim(…) <> '')` (0007:274-277) refuses an EMPTY STRING outright,
//	so a write path that sent '' for "no funding source" would fail the INSERT — and with it the
//	audit entry sharing the transaction (rule 6, invariant 3). The fake driver accepts any argument.
//
//	`NULL` is what §6's "đã chi nhưng chưa ghi rút từ nguồn nào" counts. '' would drop the voucher out
//	of that warning while attaching it to no source either: money missing from BOTH sides of one
//	screen, with every row looking filled in.
//
// The SCHEMA side of the same column — its nullability, the '' refusal, and the `chung_tu_da_khoa`
// trigger refusing to let a LOCKED voucher change source — is asserted in nguon_von_pg_test.go and
// is not repeated here. This file is about what THIS PACKAGE writes.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, and that is not a footnote. On a machine without it every
// test here SKIPS while the package still prints `ok` — a green run means the code COMPILES. Nothing
// in this file may be described as verified until somebody sets the DSN and says what came out.
//
// TestMain, moKetNoi, xaRieng and the fixtures live beside the other pg tests of this package: ONE
// way to build the schema, not two.

// chungTuMauDeGhi is the voucher the cases below write through the real store.
func chungTuMauDeGhi(id, duAnID, nguonVonID string) domain.ChungTuGiaiNgan {
	return domain.ChungTuGiaiNgan{
		ID:         id,
		DuAnID:     duAnID,
		NgayChi:    time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		SoTien:     30_000_000,
		NoiDung:    "Thanh toán đợt 3",
		NguonVonID: nguonVonID,

		NguoiNhapID: "CB-00123",
	}
}

// nguonVonCuaChungTu reads the column back as a *string, so NULL and ” are two different answers
// here — which is the entire point of every case in this file.
func nguonVonCuaChungTu(t *testing.T, xa, id string) *string {
	t.Helper()
	var ra *string
	err := moKetNoi(t).QueryRow(
		`SELECT nguon_von_id FROM chung_tu_giai_ngan WHERE tenant_id = $1 AND id = $2`,
		xa, id).Scan(&ra)
	if err != nil {
		t.Fatalf("đọc nguon_von_id của %q: %v", id, err)
	}
	return ra
}

func TestPgChenChungTuChuaGanNguonThiGhiNULL(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))

	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 100_000_000)

	kho := pkgstore.New(db)
	s := NewChungTuGiaiNganStore(kho)
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Chen(ctx, tx, chungTuMauDeGhi("ct-khong-nguon", "da-1", ""))
	}); err != nil {
		// THIS IS THE FAILURE THE TEST EXISTS FOR. If `rongThanhNil` were ever dropped from the INSERT,
		// the CHECK refuses the row and a commune recording a payment with no source — the state §13
		// rule 6 DEFINES — gets a 500 for an operation the specification permits.
		t.Fatalf("chèn chứng từ chưa gắn nguồn bị từ chối: %v", err)
	}

	if got := nguonVonCuaChungTu(t, xa, "ct-khong-nguon"); got != nil {
		t.Fatalf("nguon_von_id = %q, muốn NULL — '' rơi khỏi cảnh báo §6 mà cũng không thuộc nguồn nào",
			*got)
	}
}

func TestPgCapNhatGoChungTuKhoiNguonThiCotVeNULL(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))

	themDuAnToiThieu(t, db, xa, "da-1", "DA01", 2026, 100_000_000)
	themNguonVon(t, db, xa, "nv-xa", "Ngân sách xã, phường", 2026, 1, 9_200_000_000)

	kho := pkgstore.New(db)
	s := NewChungTuGiaiNganStore(kho)

	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Chen(ctx, tx, chungTuMauDeGhi("ct-1", "da-1", "nv-xa"))
	}); err != nil {
		t.Fatalf("chèn: %v", err)
	}
	if got := nguonVonCuaChungTu(t, xa, "ct-1"); got == nil || *got != "nv-xa" {
		t.Fatalf("nguon_von_id sau khi chèn = %v, muốn nv-xa", got)
	}

	// DETACHING IS A REAL CORRECTION: the accountant attributed a payment to the wrong source and the
	// right one is not yet known. It must land as NULL, which is what §6's warning counts.
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.CapNhat(ctx, tx, chungTuMauDeGhi("ct-1", "da-1", ""))
	}); err != nil {
		t.Fatalf("cập nhật gỡ khỏi nguồn bị từ chối: %v", err)
	}
	if got := nguonVonCuaChungTu(t, xa, "ct-1"); got != nil {
		t.Fatalf("nguon_von_id sau khi gỡ = %q, muốn NULL", *got)
	}
}

func TestPgNguonVonConSongKhongThayNguonCuaXaKhac(t *testing.T) {
	// RULE 1, AND IT IS THE CHECK THAT REPLACES A FOREIGN KEY. 0007 declares none (0007:102-113 says
	// why), so nothing in the database stops a voucher from naming another commune's source. This
	// statement binds tenant_id = $1, so such an id is indistinguishable from one that never existed —
	// and that single answer is what keeps the refusal from telling a caller what another authority
	// holds.
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)
	ctxA := ctxXa(tenant.ID(xaA))

	themNguonVon(t, db, xaB, "nv-cua-xa-b", "Nguồn xã hội hoá", 2026, 1, 1_100_000_000)
	themNguonVon(t, db, xaA, "nv-cua-xa-a", "Ngân sách xã, phường", 2026, 1, 9_200_000_000)

	kho := pkgstore.New(db)
	s := NewChungTuGiaiNganStore(kho)

	var loiCheo, loiNha error
	if err := kho.For(ctxA).Tx(ctxA, func(tx *pkgstore.ScopedTx) error {
		loiCheo = s.NguonVonConSong(ctxA, tx, "nv-cua-xa-b")
		loiNha = s.NguonVonConSong(ctxA, tx, "nv-cua-xa-a")
		return nil
	}); err != nil {
		t.Fatalf("giao dịch: %v", err)
	}
	if loiCheo != ErrKhongThayNguonVonCuaChungTu {
		t.Fatalf("nguồn của xã khác trả %v, muốn ErrKhongThayNguonVonCuaChungTu — đây là ca luật 1 "+
			"sinh ra để chặn", loiCheo)
	}
	if loiNha != nil {
		t.Fatalf("nguồn của chính xã mình bị từ chối: %v", loiNha)
	}
}

func TestPgNguonVonConSongKhongThayNguonDaXoaMem(t *testing.T) {
	// A SOFT-DELETED SOURCE IS NOT A SOURCE TO ATTACH NEW MONEY TO (rule 7, invariant 2: every read
	// path excludes deleted rows). The row stays — the commune's history keeps naming it — but a
	// payment recorded today may not be filed under a source the commune has withdrawn.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(xa))

	themNguonVon(t, db, xa, "nv-cu", "Nguồn đã rút", 2026, 1, 1_000_000_000)
	if _, err := db.Exec(
		`UPDATE nguon_von SET deleted_at = now(), deleted_by = 'CB-00123', delete_reason = 'khai nhầm'
		  WHERE tenant_id = $1 AND id = 'nv-cu'`, xa); err != nil {
		t.Fatalf("xoá mềm nguồn vốn: %v", err)
	}

	kho := pkgstore.New(db)
	s := NewChungTuGiaiNganStore(kho)
	var loi error
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		loi = s.NguonVonConSong(ctx, tx, "nv-cu")
		return nil
	}); err != nil {
		t.Fatalf("giao dịch: %v", err)
	}
	if loi != ErrKhongThayNguonVonCuaChungTu {
		t.Fatalf("nguồn đã xoá mềm trả %v, muốn ErrKhongThayNguonVonCuaChungTu", loi)
	}
}
