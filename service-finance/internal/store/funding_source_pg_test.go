package store

import (
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// Integration tests for funding_source.go against a real PostgreSQL — the reads behind §6's warning
// and breakdown, and the catalogue write path. SKIPPED UNLESS VIGOV_TEST_DSN IS SET (see
// nguon_von_pg_test.go's header): on a machine without it a green run means the code COMPILES.

func TestPgUnattributedDisbursedCountsOnlySourcelessVouchersOfTheYear(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)

	themNguonVon(t, db, commune, "nv-xa", "Ngân sách xã, phường", 1)
	themDuAnToiThieu(t, db, commune, "da-2026", "DA2026", 2026, 100_000_000)
	themDuAnToiThieu(t, db, commune, "da-2027", "DA2027", 2027, 100_000_000)
	// Counted: no source, a 2026 project, state `ke-toan-nhap` (the default — the FIRST state counts).
	themChungTu(t, db, commune, "ct-null", "da-2026", 3_400_000, nil)
	// Not counted: names a source; another year's project; another commune.
	themChungTu(t, db, commune, "ct-nguon", "da-2026", 1_000_000, strp("nv-xa"))
	themChungTu(t, db, commune, "ct-2027", "da-2027", 5_000_000, nil)
	themDuAnToiThieu(t, db, other, "da-2026", "DA2026", 2026, 100_000_000)
	themChungTu(t, db, other, "ct-khac", "da-2026", 9_000_000, nil)

	got, err := dungNguonVonStore(db).UnattributedDisbursed(ctxXa(tenant.ID(commune)), 2026)
	if err != nil {
		t.Fatalf("UnattributedDisbursed: %v", err)
	}
	if got != 3_400_000 {
		t.Fatalf("= %d, muốn 3.400.000 — chỉ chứng từ không nguồn của dự án năm 2026 trong xã này", got)
	}
}

func TestPgSourceProjectsIsAllocationMembershipYearScopedAndReconciles(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	ctx := ctxXa(tenant.ID(commune))

	themNguonVon(t, db, commune, "nv-xa", "Ngân sách xã, phường", 1)
	themNguonVon(t, db, commune, "nv-tp", "Ngân sách thành phố hỗ trợ", 2)
	themDuAnToiThieu(t, db, commune, "da-1", "DA01", 2026, 60_000_000)
	themDuAnToiThieu(t, db, commune, "da-2", "DA02", 2026, 40_000_000)
	themDuAnToiThieu(t, db, commune, "da-3", "DA03", 2026, 10_000_000) // no line on nv-xa, but pays from it
	themDuAnToiThieu(t, db, commune, "da-old", "DA00", 2025, 10_000_000)
	themPhanBo(t, db, commune, "pb-1", "da-1", "nv-xa", 30_000_000)
	themPhanBo(t, db, commune, "pb-1b", "da-1", "nv-tp", 30_000_000)
	themPhanBo(t, db, commune, "pb-2", "da-2", "nv-xa", 40_000_000)
	themPhanBo(t, db, commune, "pb-old", "da-old", "nv-xa", 10_000_000)
	themChungTu(t, db, commune, "ct-1", "da-1", 13_200_000, strp("nv-xa"))
	themChungTu(t, db, commune, "ct-1b", "da-1", 7_000_000, strp("nv-tp")) // other source: not in nv-xa's rows
	themChungTu(t, db, commune, "ct-3", "da-3", 2_000_000, strp("nv-xa"))
	// Another commune with the SAME ids.
	themNguonVon(t, db, other, "nv-xa", "Ngân sách xã, phường", 1)
	themDuAnToiThieu(t, db, other, "da-1", "DA01", 2026, 60_000_000)
	themPhanBo(t, db, other, "pb-1", "da-1", "nv-xa", 999)
	themChungTu(t, db, other, "ct-1", "da-1", 999, strp("nv-xa"))

	got, err := dungNguonVonStore(db).SourceProjects(ctx, "nv-xa", 2026)
	if err != nil {
		t.Fatalf("SourceProjects: %v", err)
	}
	if got.Source.ID != "nv-xa" || got.Source.Nam != 2026 || len(got.Projects) != 2 {
		t.Fatalf("= %+v — muốn đúng hai dự án 2026 có dòng phân bổ trên nv-xa", got)
	}
	p1, p2 := got.Projects[0], got.Projects[1]
	if p1.Code != "DA01" || p1.AllocatedAmount != 30_000_000 || p1.DisbursedAmount != 13_200_000 || p1.PlannedAmount != 60_000_000 {
		t.Errorf("DA01 = %+v", p1)
	}
	if p2.Code != "DA02" || p2.AllocatedAmount != 40_000_000 || p2.DisbursedAmount != 0 {
		t.Errorf("DA02 = %+v", p2)
	}
	if got.DisbursedWithoutAllocation != 2_000_000 {
		t.Errorf("chi ngoài phân bổ = %d, muốn 2.000.000 (DA03)", got.DisbursedWithoutAllocation)
	}

	// The rows plus the remainder ARE the card's figure.
	cards, err := dungNguonVonStore(db).TienDoTheoNguon(ctx, 2026)
	if err != nil {
		t.Fatalf("TienDoTheoNguon: %v", err)
	}
	var card domain.TienDoNguonVon
	for _, c := range cards {
		if c.NguonVon.ID == "nv-xa" {
			card = c
		}
	}
	if sum := p1.DisbursedAmount + p2.DisbursedAmount + got.DisbursedWithoutAllocation; sum != card.DaGiaiNgan {
		t.Errorf("dòng + phần ngoài phân bổ = %d, thẻ = %d — hai số trên một màn không khớp", sum, card.DaGiaiNgan)
	}
	if p1.AllocatedAmount+p2.AllocatedAmount != card.DaPhanBo {
		t.Errorf("tổng phân bổ dòng = %d, thẻ = %d", p1.AllocatedAmount+p2.AllocatedAmount, card.DaPhanBo)
	}

	if _, err := dungNguonVonStore(db).SourceProjects(ctxXa(tenant.ID(other)), "nv-tp", 2026); !errors.Is(err, ErrFundingSourceNotFound) {
		t.Fatalf("nguồn của xã khác: lỗi = %v, muốn ErrFundingSourceNotFound", err)
	}
}

func TestPgFundingSourceWritePath(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	ctx := ctxXa(tenant.ID(commune))
	scoped := pkgstore.New(db)
	w := NewFundingSourceWriteStore(scoped)

	themNguonVon(t, db, commune, "nv-cu", "Ngân sách xã, phường", 7)

	err := scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if taken, err := w.NameTaken(ctx, tx, "Ngân sách xã, phường"); err != nil || !taken {
			t.Errorf("NameTaken(tên đã có) = %v, %v", taken, err)
		}
		if n, err := w.CountLive(ctx, tx); err != nil || n != 1 {
			t.Errorf("CountLive = %d, %v", n, err)
		}
		order, err := w.InsertSource(ctx, tx, "nv-moi", "Nguồn xã hội hoá")
		if err != nil {
			return err
		}
		if order != 8 {
			t.Errorf("thu_tu = %d, muốn 8 (cuối danh sách)", order)
		}
		a := domain.FundingSourceAnnualAmount{ID: "fa-1", FundingSourceID: "nv-moi", Year: 2026, GrantedAmount: 1_100_000_000}
		if err := w.InsertAnnualAmount(ctx, tx, a); err != nil {
			return err
		}
		if _, err := w.SourceForUpdate(ctx, tx, "nv-moi"); err != nil {
			return err
		}
		got, found, err := w.AnnualAmount(ctx, tx, "nv-moi", 2026)
		if err != nil || !found || got.GrantedAmount != 1_100_000_000 {
			t.Errorf("AnnualAmount = %+v, %v, %v", got, found, err)
		}
		if err := w.UpdateAnnualAmount(ctx, tx, "fa-1", 1_200_000_000); err != nil {
			return err
		}
		if _, found, _ := w.AnnualAmount(ctx, tx, "nv-moi", 2027); found {
			t.Error("năm 2027 chưa nhập mà đọc ra có số")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("giao dịch: %v", err)
	}

	// The unique key, reached through the write path, is the sentence's error — not a bare 23505.
	err = scoped.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, err := w.InsertSource(ctx, tx, "nv-trung", "Nguồn xã hội hoá")
		return err
	})
	if !errors.Is(err, ErrFundingSourceNameTaken) {
		t.Fatalf("tên trùng: lỗi = %v, muốn ErrFundingSourceNameTaken", err)
	}

	cards, err := dungNguonVonStore(db).DanhSach(ctx, 2026)
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(cards) != 2 || cards[1].ID != "nv-moi" || cards[1].TongNguon != 1_200_000_000 {
		t.Fatalf("sau khi ghi: %+v", cards)
	}
}
