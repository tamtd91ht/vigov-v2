package store

// The statements behind POST /api/v1/map-asset-types/defaults — sowing the eleven groups of ADR 0072
// §3 into one commune's loai_tai_nguyen_ban_do. Same four properties as the catalogue's write path
// (loai_tai_nguyen_ban_do.go): the commune is $1 from tx.TenantID(), nothing here opens a transaction.
//
// THIS IS THE ONE PATH THAT WRITES `nguon = 'he-thong'`, and it writes it AS A LITERAL. Migration 0003
// says where system rows come from ("seed sown PER COMMUNE … COMMUNE ONBOARDING") and ADR 0072 §3 makes
// this explicit, audited action that step for this catalogue. The rows ship with the software, so they
// are tier 2 (0003: "TIER 2 FOR THE SHIPPED CODES"): a commune relabels or disables them, never
// deletes them. `ma_nguon_re_nhanh` stays false — no code branches on any of the eleven.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// LockCatalogueForSeed serialises two seeds of the SAME commune: without it both read "absent" and the
// second dies on UNIQUE (tenant_id, ma) with a 500. A transaction-scoped advisory lock keyed on the
// commune — released at commit or rollback, never held across requests, never touching another commune.
func (s *LoaiTaiNguyenBanDoStore) LockCatalogueForSeed(ctx context.Context, tx *store.ScopedTx) error {
	const stmt = `SELECT pg_advisory_xact_lock(hashtext('loai_tai_nguyen_ban_do:defaults:' || $1))`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID())); err != nil {
		return fmt.Errorf("loai_tai_nguyen_ban_do: khoá để nạp mặc định: %w", err)
	}
	return nil
}

// CodeStates reports, for each code already present in this commune — SOFT-DELETED ROWS INCLUDED —
// whether it is deleted. A code absent from the map is free. Deleted rows count because UNIQUE
// (tenant_id, ma) counts them (rule 7, invariant 3), and a seed never revives one (ADR 0055 §2).
func (s *LoaiTaiNguyenBanDoStore) CodeStates(ctx context.Context, tx *store.ScopedTx, codes []string) (map[string]bool, error) {
	out := make(map[string]bool, len(codes))
	if len(codes) == 0 {
		return out, nil
	}
	tail := "AND ma IN ("
	args := make([]any, 0, len(codes))
	for i, c := range codes {
		if i > 0 {
			tail += ", "
		}
		tail += fmt.Sprintf("$%d", i+2)
		args = append(args, c)
	}
	tail += ")"
	// ScopedTx.Query binds tenant_id = $1 itself.
	rows, err := tx.Query(ctx, `ma, deleted_at IS NOT NULL`, "loai_tai_nguyen_ban_do", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("loai_tai_nguyen_ban_do: đọc mã đã có: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			code    string
			deleted bool
		)
		if err := rows.Scan(&code, &deleted); err != nil {
			return nil, fmt.Errorf("loai_tai_nguyen_ban_do: đọc mã đã có: %w", err)
		}
		out[code] = deleted
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_tai_nguyen_ban_do: duyệt mã đã có: %w", err)
	}
	return out, nil
}

// insertSystemRow — `nguon = 'he-thong'`, `ma_nguon_re_nhanh = false`, `la_mac_dinh = false` and
// `dang_dung = true` ARE LITERALS. Nothing a request carries can reach them; the only parameters are
// the commune, a fresh id and the code / label / order of domain.DefaultMapAssetTypes.
const insertSystemRow = `INSERT INTO loai_tai_nguyen_ban_do
	(tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh)
	VALUES ($1, $2, $3, $4, $5, false, true, 'he-thong', false)`

// InsertSystemRow adds one shipped group to this commune's catalogue.
func (s *LoaiTaiNguyenBanDoStore) InsertSystemRow(ctx context.Context, tx *store.ScopedTx, id string, d domain.MapAssetTypeDefault) error {
	if _, err := tx.Exec(ctx, insertSystemRow, string(tx.TenantID()), id, d.Code, d.Label, d.Order); err != nil {
		return fmt.Errorf("loai_tai_nguyen_ban_do: chèn nhóm mặc định: %w", err)
	}
	return nil
}
