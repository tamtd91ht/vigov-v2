package store

import (
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// SoftDelete and re-adding a field's own row (ADR 0079 lô 2 Q4) against a real PostgreSQL. Skipped
// unless VIGOV_TEST_DSN is set — the harness of checker_pg_test.go.

// A FIELD ROW SOFT-DELETED BY SoftDelete FREES ITS SLOT: the same field can be added again (0008:213),
// the removed row keeps its three soft-delete columns, and the default row is untouched by SQL even
// when asked by id.
func TestPgSLASoftDeleteFreesFieldAndSparesDefault(t *testing.T) {
	db := moKetNoi(t)
	xa, _ := xaRieng(t)
	themSLABuoc(t, db, xa, "sla-fr-md", "phan-anh", "", 8, 56, 24, 25, 49)
	themSLABuoc(t, db, xa, "sla-fr-an", "phan-anh", "an-ninh", 2, 16, 4, 8, 16)

	kho := pkgstore.New(db)
	s := NewSLAStore(kho)
	ctx := ctxXa(xa)
	err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, idSLA("sla-fr-an"), "CB-0042", "gộp vào mặc định")
	})
	if err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	var by, reason string
	if err := db.QueryRow(`SELECT deleted_by, delete_reason FROM sla WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NOT NULL`,
		xa, idSLA("sla-fr-an")).Scan(&by, &reason); err != nil || by != "CB-0042" || reason != "gộp vào mặc định" {
		t.Errorf("dòng đã xoá: by %q reason %q err %v", by, reason, err)
	}

	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.Chen(ctx, tx, domain.DongSLA{ID: idSLA("sla-fr-an-2"), LoaiViec: domain.LoaiViecPhanAnh,
			LinhVuc: "an-ninh", GioTiepNhan: 2, GioXuLyXong: 12, GioSapDenHan: 4, GioBaoLanhDao: 8, GioBaoChuTich: 16})
	})
	if err != nil {
		t.Errorf("thêm lại lĩnh vực sau khi xoá mềm: %v", err)
	}

	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.SoftDelete(ctx, tx, idSLA("sla-fr-md"), "CB-0042", "thử")
	})
	if !errors.Is(err, ErrDongSLAKhongTonTai) {
		t.Errorf("xoá dòng mặc định: lỗi = %v, muốn ErrDongSLAKhongTonTai (SQL không chạm dòng mặc định)", err)
	}
}
