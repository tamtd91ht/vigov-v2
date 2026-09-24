package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// NhanTrangThaiNhiemVuStore reads and writes a commune's OVERRIDES of the seven task-status labels
// (`nhan_trang_thai_nhiem_vu`, migration 0010; open question #21, ADR 0035 §C).
//
// WHAT A MISSING ROW MEANS: the commune has not re-worded that status, so the screen shows the
// DEFAULT (domain.MacDinhTrangThaiNhiemVu). Merging is the domain's job, not this file's: the store
// returns the rows that exist and nothing it made up.
//
// THERE IS NO DELETE AND NO SOFT DELETE HERE. The table has no `deleted_at` (the migration argues
// why) and its trigger refuses DELETE; "back to the default" is an upsert of the default wording,
// audited like any other write.
type NhanTrangThaiNhiemVuStore struct {
	db *store.DB
}

// NewNhanTrangThaiNhiemVuStore takes *store.DB and nothing else — no constructor from a raw *sql.DB,
// so there is no way to build one that can query without a commune (rule 1, invariant 5).
func NewNhanTrangThaiNhiemVuStore(db *store.DB) *NhanTrangThaiNhiemVuStore {
	return &NhanTrangThaiNhiemVuStore{db: db}
}

// TranNhanTrangThai is the most rows one commune can hold: one per status code, enforced by
// PRIMARY KEY (tenant_id, ma) together with the CHECK on `ma`. Selecting one more than that is what
// makes "the schema no longer says what this file believes" detectable.
const TranNhanTrangThai = 7

// ErrQuaNhieuNhanTrangThai — more rows than status codes. Impossible under migration 0010; reaching
// it means the schema changed under this file. Refused, not truncated: a truncated set silently
// drops a commune's wording for a status.
var ErrQuaNhieuNhanTrangThai = errors.New("nhan_trang_thai_nhiem_vu: nhiều dòng hơn số mã trạng thái")

// cotNhanTrangThai IS READ BY POSITION. `ma` / `nhan` / `cap_nhat_boi` are all TEXT: swapping two
// here or in a Scan compiles and puts a staff code where a Kanban header belongs.
const cotNhanTrangThai = `ma, nhan, thu_tu, cap_nhat_boi`

func quetNhanTrangThai(quet func(...any) error) (domain.NhanTrangThaiNhiemVu, error) {
	var n domain.NhanTrangThaiNhiemVu
	var ma string
	if err := quet(&ma, &n.Nhan, &n.ThuTu, &n.CapNhatBoi); err != nil {
		return domain.NhanTrangThaiNhiemVu{}, err
	}
	n.Ma = domain.TrangThaiNhiemVu(ma)
	return n, nil
}

// DanhSach reads every override of the commune the request arrived in.
//
// THE COMMUNE IS NOT A PARAMETER: Scoped.Query binds it to $1 from the context (rule 1,
// invariant 4). ORDER BY ma only makes the statement deterministic; the DISPLAY order is decided by
// domain.GopNhanTrangThai over the merged seven, which is the one place that knows the defaults.
func (s *NhanTrangThaiNhiemVuStore) DanhSach(ctx context.Context) ([]domain.NhanTrangThaiNhiemVu, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNhanTrangThai, "nhan_trang_thai_nhiem_vu",
		`ORDER BY ma LIMIT $2`, TranNhanTrangThai+1)
	if err != nil {
		return nil, fmt.Errorf("nhan_trang_thai_nhiem_vu: đọc: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.NhanTrangThaiNhiemVu, 0, TranNhanTrangThai)
	for rows.Next() {
		n, err := quetNhanTrangThai(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("nhan_trang_thai_nhiem_vu: đọc dòng: %w", err)
		}
		ra = append(ra, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhan_trang_thai_nhiem_vu: duyệt kết quả: %w", err)
	}
	if len(ra) > TranNhanTrangThai {
		return nil, ErrQuaNhieuNhanTrangThai
	}
	return ra, nil
}

// TheoMaDeSua reads one override inside the transaction, holding it until the transaction ends.
// The second result is false when the commune has no row for this code (it shows the default).
//
// ⚠ `FOR UPDATE` LOCKS ONLY A ROW THAT EXISTS. Two first-ever writes of the same code in the same
// commune both see "no row", both upsert, and ON CONFLICT makes the second an update — so the table
// ends correct, but the second audit entry's "before" says default where it was the first writer's
// value. Accepted: the table holds display configuration, both entries name their author and their
// "after", and closing the window would need a lock on a row that does not exist yet.
func (s *NhanTrangThaiNhiemVuStore) TheoMaDeSua(ctx context.Context, tx *store.ScopedTx,
	ma domain.TrangThaiNhiemVu) (domain.NhanTrangThaiNhiemVu, bool, error) {

	const stmt = `SELECT ` + cotNhanTrangThai + ` FROM nhan_trang_thai_nhiem_vu ` +
		`WHERE tenant_id = $1 AND ma = $2 FOR UPDATE`

	n, err := quetNhanTrangThai(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), string(ma)).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NhanTrangThaiNhiemVu{}, false, nil
	}
	if err != nil {
		return domain.NhanTrangThaiNhiemVu{}, false, fmt.Errorf("nhan_trang_thai_nhiem_vu: đọc dòng để sửa: %w", err)
	}
	return n, true, nil
}

// ghiNhanTrangThai UPSERTS ON THE PRIMARY KEY and writes BOTH `nhan` and `thu_tu` every time: the
// migration makes both NOT NULL, so a first write that changes only the label still has to store
// the effective position (the default) beside it.
//
// `tenant_id` and `ma` APPEAR IN NO SET CLAUSE. The trigger refuses changing either; their absence
// here is what keeps that floor unreachable from this service.
const ghiNhanTrangThai = `INSERT INTO nhan_trang_thai_nhiem_vu (tenant_id, ma, nhan, thu_tu, cap_nhat_boi)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (tenant_id, ma) DO UPDATE
	SET nhan = EXCLUDED.nhan, thu_tu = EXCLUDED.thu_tu,
	    cap_nhat_boi = EXCLUDED.cap_nhat_boi, cap_nhat_luc = now()`

// Ghi stores one commune's wording and position for one code. The commune is $1, from the
// transaction, which took it from the context — never a method parameter.
func (s *NhanTrangThaiNhiemVuStore) Ghi(ctx context.Context, tx *store.ScopedTx, n domain.NhanTrangThaiNhiemVu) error {
	if _, err := tx.Exec(ctx, ghiNhanTrangThai, string(tx.TenantID()),
		string(n.Ma), n.Nhan, n.ThuTu, n.CapNhatBoi); err != nil {
		return fmt.Errorf("nhan_trang_thai_nhiem_vu: ghi: %w", err)
	}
	return nil
}
