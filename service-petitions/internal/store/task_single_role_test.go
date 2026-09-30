package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ADR 0065 NV5 (user decision 30/09/2026, migration 0025): `co_quan_chu_tri_id` and
// `chuyen_vien_theo_doi_ma` are RETIRED — never read, never written. The survivors are `bo_phan_id` and
// `nguoi_thuc_hien_ma`.
//
//	PROVED HERE   the register's column list, the create INSERT and the hand-over UPDATE name neither
//	              retired column, and the hand-over still writes both survivors.
//	NOT PROVED    the reads' predicates — task_list_scope_test.go (related scope) and
//	              org_unit_holdings_test.go (held-open count) each assert their own statement.

var retiredTaskRoleColumns = []string{"co_quan_chu_tri_id", "chuyen_vien_theo_doi_ma"}

func assertNoRetiredRoleColumn(t *testing.T, what, sqlText string) {
	t.Helper()
	for _, c := range retiredTaskRoleColumns {
		if strings.Contains(sqlText, c) {
			t.Errorf("%s còn nêu cột đã nghỉ %s: %s", what, c, sqlText)
		}
	}
}

func TestTaskColumnListOmitsRetiredRoles(t *testing.T) {
	assertNoRetiredRoleColumn(t, "cotNhiemVu", cotNhiemVu)
}

func runTaskWrite(t *testing.T, fn func(context.Context, *pkgstore.ScopedTx, *NhiemVuStore) error) sfStmt {
	t.Helper()
	d := &sfDB{affected: 1}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewNhiemVuStore(h)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return fn(ctx, tx, s)
	}); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d câu lệnh, muốn 1", len(d.stmts))
	}
	return d.stmts[0]
}

func TestTaskCreateWritesNoRetiredRole(t *testing.T) {
	st := runTaskWrite(t, func(ctx context.Context, tx *pkgstore.ScopedTx, s *NhiemVuStore) error {
		return s.Tao(ctx, tx, domain.NhiemVu{
			ID: "nv-001", Ma: "NV01", Loai: "co-ban", TieuDe: "Việc", TrangThai: domain.MoiGiao,
			NguonGiao: domain.NguonTrucTiep, BoPhanID: "bp-a", NguoiThucHienMa: "CB-00311",
			NguoiTaoMa: "CB-00123", UpdatedAt: sfAt,
		})
	})
	assertNoRetiredRoleColumn(t, "INSERT nhiem_vu", st.sql)
	// The column list and the placeholders must still agree after two columns left.
	if n := strings.Count(st.sql, "$"); n != len(st.args) {
		t.Errorf("%d chỗ giữ tham số, %d tham số", n, len(st.args))
	}
}

func TestTaskReassignWritesSurvivorsOnly(t *testing.T) {
	st := runTaskWrite(t, func(ctx context.Context, tx *pkgstore.ScopedTx, s *NhiemVuStore) error {
		return s.Reassign(ctx, tx, "nv-001", domain.DangThucHien, domain.NhiemVu{
			BoPhanID: "bp-b", NguoiThucHienMa: "CB-00999", TrangThai: domain.MoiGiao,
		})
	})
	assertNoRetiredRoleColumn(t, "UPDATE giao lại", st.sql)
	if !strings.Contains(st.sql, "bo_phan_id = $3, nguoi_thuc_hien_ma = $4, trang_thai = $5") {
		t.Fatalf("danh sách SET đổi chỗ; các vị trí dưới không còn đúng nghĩa: %s", st.sql)
	}
	if len(st.args) != 6 || st.args[2] != "bp-b" || st.args[3] != "CB-00999" ||
		st.args[4] != string(domain.MoiGiao) || st.args[5] != string(domain.DangThucHien) {
		t.Errorf("tham số = %v", st.args)
	}
}
