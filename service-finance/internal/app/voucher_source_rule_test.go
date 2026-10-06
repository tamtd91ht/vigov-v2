package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The funding-source rule of decision 06/10/2026 (domain.CheckVoucherSource), through the REAL store
// over the fake driver. Every refusal must leave NOTHING committed: no voucher row, no audit entry.

const unallocatedSourceID = "01JNGUONVONKHONGPHANBO000" // a live source of this commune the project does not draw on

func storeWithAllocatedProject(row *hangCT) *khoCTGia {
	return &khoCTGia{
		maDuAn: maDuAnMau, hang: row,
		nguonVonCo: map[string]bool{idNguonVonCu: true, idNguonVonMoi: true, unallocatedSourceID: true},
		allocated:  []string{idNguonVonCu, idNguonVonMoi},
	}
}

func assertVoucherNothingWritten(t *testing.T, k *khoCTGia) {
	t.Helper()
	if k.coCau("INSERT INTO chung_tu_giai_ngan") || k.coCau("UPDATE chung_tu_giai_ngan") {
		t.Error("refused, yet the voucher was written")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("refused, yet an audit entry was written")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — want 0/1", k.daCommit, k.daRollback)
	}
}

func TestCreateVoucher_AllocatedProjectWithoutSource_SourceRequired(t *testing.T) {
	k := storeWithAllocatedProject(nil)
	uc, ctx := dungUseCaseChungTu(t, k)

	_, err := uc.Them(ctx, themChungTuMau(), canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSourceRequired) {
		t.Fatalf("err = %v, want ErrSourceRequired", err)
	}
	assertVoucherNothingWritten(t, k)
}

func TestCreateVoucher_AllocatedProjectOtherSource_SourceNotAllocated(t *testing.T) {
	k := storeWithAllocatedProject(nil)
	uc, ctx := dungUseCaseChungTu(t, k)

	req := themChungTuMau()
	req.NguonVonID = unallocatedSourceID
	_, err := uc.Them(ctx, req, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSourceNotAllocated) {
		t.Fatalf("err = %v, want ErrSourceNotAllocated", err)
	}
	assertVoucherNothingWritten(t, k)
}

func TestCreateVoucher_UnallocatedProjectWithSource_SourceNotAllocated(t *testing.T) {
	k := storeWithAllocatedProject(nil)
	k.allocated = nil
	uc, ctx := dungUseCaseChungTu(t, k)

	req := themChungTuMau()
	req.NguonVonID = idNguonVonCu
	_, err := uc.Them(ctx, req, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSourceNotAllocated) {
		t.Fatalf("err = %v, want ErrSourceNotAllocated", err)
	}
	assertVoucherNothingWritten(t, k)
}

func TestCreateVoucher_AllocatedSource_WritesUnderProjectShareLock(t *testing.T) {
	k := storeWithAllocatedProject(nil)
	uc, ctx := dungUseCaseChungTu(t, k)

	req := themChungTuMau()
	req.NguonVonID = idNguonVonMoi
	got, err := uc.Them(ctx, req, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if got.NguonVonID != idNguonVonMoi || k.daCommit != 1 {
		t.Fatalf("source %q, commit %d", got.NguonVonID, k.daCommit)
	}
	lockAt, insertAt := statementIndex(k, "FOR SHARE"), statementIndex(k, "INSERT INTO chung_tu_giai_ngan")
	if lockAt < 0 {
		t.Fatal("the project row is not share-locked — an allocation edit can interleave (race of 8245698b)")
	}
	if lockAt > insertAt {
		t.Fatal("the project share lock is taken AFTER the insert — it protects nothing")
	}
	if !strings.Contains(k.lenh[lockAt].sql, "FROM du_an") {
		t.Fatalf("FOR SHARE is on another statement: %s", k.lenh[lockAt].sql)
	}
}

func TestEditVoucher_AllocatedProjectClearsSource_SourceRequired(t *testing.T) {
	row := hangOTrangThai(domain.ChungTuKeToanNhap)
	row.nguonVonID = idNguonVonCu
	k := storeWithAllocatedProject(row)
	uc, ctx := dungUseCaseChungTu(t, k)

	empty := ""
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &empty}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSourceRequired) {
		t.Fatalf("err = %v, want ErrSourceRequired", err)
	}
	assertVoucherNothingWritten(t, k)
}

func TestEditVoucher_AllocatedProjectMovesToOtherSource_SourceNotAllocated(t *testing.T) {
	row := hangOTrangThai(domain.ChungTuKeToanNhap)
	row.nguonVonID = idNguonVonCu
	k := storeWithAllocatedProject(row)
	uc, ctx := dungUseCaseChungTu(t, k)

	other := unallocatedSourceID
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &other}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSourceNotAllocated) {
		t.Fatalf("err = %v, want ErrSourceNotAllocated", err)
	}
	assertVoucherNothingWritten(t, k)
}

func TestEditVoucher_UnallocatedProjectAttachesSource_SourceNotAllocated(t *testing.T) {
	k := storeWithAllocatedProject(hangOTrangThai(domain.ChungTuKeToanNhap))
	k.allocated = nil
	uc, ctx := dungUseCaseChungTu(t, k)

	src := idNguonVonMoi
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &src}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSourceNotAllocated) {
		t.Fatalf("err = %v, want ErrSourceNotAllocated", err)
	}
	assertVoucherNothingWritten(t, k)
}

func TestEditVoucher_SourceNotNamed_LegacyNullSourceStillEditable(t *testing.T) {
	// THE STATED CHOICE IN app.Sua: a voucher entered before its project declared sources keeps its
	// NULL source through a correction that does not name the field.
	k := storeWithAllocatedProject(hangOTrangThai(domain.ChungTuKeToanNhap)) // source ""
	uc, ctx := dungUseCaseChungTu(t, k)

	amount := domain.Dong(31_000_000)
	got, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{SoTien: &amount}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if got.NguonVonID != "" || got.SoTien != amount || k.daCommit != 1 {
		t.Fatalf("source %q, amount %d, commit %d", got.NguonVonID, got.SoTien, k.daCommit)
	}
	if !k.coCau("FOR SHARE") {
		t.Error("an edit did not share-lock the project row")
	}
}

func TestEditVoucher_MovesBetweenAllocatedSources(t *testing.T) {
	row := hangOTrangThai(domain.ChungTuKeToanNhap)
	row.nguonVonID = idNguonVonCu
	k := storeWithAllocatedProject(row)
	uc, ctx := dungUseCaseChungTu(t, k)

	next := idNguonVonMoi
	got, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &next}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if got.NguonVonID != idNguonVonMoi || k.daCommit != 1 {
		t.Fatalf("source %q, commit %d", got.NguonVonID, k.daCommit)
	}
}

func TestEditVoucher_LockedVoucherStillRefusesSourceChange(t *testing.T) {
	// Rule 4 of the card: the lock is checked BEFORE the source rule, so a locked voucher answers
	// voucher_state, never source_*.
	k := storeWithAllocatedProject(hangDaKhoa(maLanhDao))
	uc, ctx := dungUseCaseChungTu(t, k)

	empty := ""
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &empty}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrChungTuDaKhoa) {
		t.Fatalf("err = %v, want ErrChungTuDaKhoa", err)
	}
	assertVoucherNothingWritten(t, k)
}

// statementIndex is the index of the first recorded statement containing `part`, or -1.
func statementIndex(k *khoCTGia, part string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	for i, l := range k.lenh {
		if strings.Contains(l.sql, part) {
			return i
		}
	}
	return -1
}
