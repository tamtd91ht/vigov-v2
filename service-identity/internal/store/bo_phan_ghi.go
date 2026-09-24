package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The WRITE paths of the org chart (14-cau-hinh.md §1): add a unit, rename it, move it, re-rank it.
//
// THREE THINGS HOLD ACROSS EVERY METHOD, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context
//     (rule 1, invariants 4 and 5). No method takes a commune, so no caller can name another
//     commune's unit — as a target, as a parent, or as an ancestor on the cycle walk.
//  2. NOTHING HERE OPENS A TRANSACTION. Every method takes the *store.ScopedTx the use case opened
//     and writes its audit entry in (rule 6, invariant 3).
//  3. `ma` APPEARS IN NO UPDATE. The code is an issued identifier; it is written once by Chen and
//     never again (rule 7, invariant 3).
//
// THERE IS NO DELETE. Refusing to remove a unit that still holds staff OR records in documents,
// petitions or comms needs a cross-service contract that does not exist (rule 2, stop condition #2;
// user decision 2026-09-24). The soft-delete columns exist; no method here writes them.

var (
	// ErrKhongTimThayBoPhan — the unit NAMED IN THE PATH matches no live row of this commune. The
	// same answer for an invented id, a soft-deleted unit and another commune's unit, so none can
	// be told apart by trying (rule 4, forbidden #2 applied to configuration). 404.
	ErrKhongTimThayBoPhan = errors.New("bo_phan: không tìm thấy bộ phận")

	// ErrBoPhanChaKhongTonTai — the PARENT named in the body is not a live unit of this commune.
	// 400 and not 404: the resource addressed by the request exists (or is being created); what is
	// wrong is a value the client sent. Same reading as `org_unit_not_found` on the staff routes.
	ErrBoPhanChaKhongTonTai = errors.New("bo_phan: bộ phận cha không tồn tại trong xã")

	// ErrMaBoPhanDaDung — `UNIQUE (tenant_id, ma)` refused the code. That key is NOT partial, so a
	// code once issued stays taken after a soft delete (rule 7, invariant 3). 409.
	ErrMaBoPhanDaDung = errors.New("bo_phan: mã bộ phận đã được dùng trong xã")
)

// cotKhoaBoPhan IS READ BY POSITION in KhoaBoPhan. `ma` and `ten` are adjacent TEXT columns.
//
// THE DELETED FLAG IS SELECTED, NOT FILTERED, because the one query serves two callers that want
// different answers: the target of an edit and a new parent must be live, while an ancestor on the
// cycle walk must be FOLLOWED whatever its state — a walk that stopped at a soft-deleted ancestor
// would miss a cycle running through it.
const cotKhoaBoPhan = `id, ma, ten, coalesce(cha_id,''), thu_tu, deleted_at IS NOT NULL`

// KhoaBoPhan reads one unit of this commune and holds its row until the transaction ends.
//
// `FOR UPDATE` IS THE POINT. Every write here is read-decide-write: an edit reads the row and
// writes it back, and a move walks the ancestors of the new parent and decides "no cycle". Without
// the locks two administrators moving A under B and B under A at the same instant both walk a tree
// in which neither move has happened, both find no cycle, and both commit — the org chart then holds
// a loop no read path terminates on. With them the second walk waits for the first commit and sees
// its result (or PostgreSQL aborts one of the two as a deadlock, which is a refusal, not a loop).
func (s *BoPhanStore) KhoaBoPhan(ctx context.Context, tx *store.ScopedTx, id string) (domain.BoPhan, bool, error) {
	if id == "" {
		// Fail closed rather than compare against the empty string.
		return domain.BoPhan{}, false, ErrKhongTimThayBoPhan
	}
	const stmt = `SELECT ` + cotKhoaBoPhan + ` FROM bo_phan ` +
		`WHERE tenant_id = $1 AND id = $2 FOR UPDATE`

	var bp domain.BoPhan
	var daXoa bool
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).
		Scan(&bp.ID, &bp.Ma, &bp.Ten, &bp.ChaID, &bp.ThuTu, &daXoa)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BoPhan{}, false, ErrKhongTimThayBoPhan
	}
	if err != nil {
		return domain.BoPhan{}, false, fmt.Errorf("bo_phan: khoá dòng: %w", err)
	}
	return bp, daXoa, nil
}

// tranMaCungGoc bounds MaCungGoc. A commune with this many units sharing one slug base is not an
// org chart (TranDanhMucBoPhan is 500 for the whole commune).
const tranMaCungGoc = 1000

// MaCungGoc lists the codes of this commune equal to `goc` or of the form `goc-…`, SOFT-DELETED ROWS
// INCLUDED — the unique key counts them, so "free" must count them too, or the suffix chosen here is
// one the INSERT then refuses.
//
// Read inside the transaction. It does not LOCK anything — an absent code cannot be locked — so two
// concurrent creations of one name can still pick the same candidate; the unique key refuses the
// loser, whose request answers 409, and a retry picks the next suffix.
//
// `goc` IS A VALIDATED SLUG ([a-z0-9-] only), so it carries neither `%` nor `_` and needs no
// escaping inside LIKE. The caller guarantees that by construction (domain.SinhMaBoPhan /
// domain.ChuanHoaMaBoPhan).
func (s *BoPhanStore) MaCungGoc(ctx context.Context, tx *store.ScopedTx, goc string) ([]string, error) {
	const stmt = `SELECT ma FROM bo_phan WHERE tenant_id = $1 AND (ma = $2 OR ma LIKE $3) LIMIT $4`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), goc, goc+"-%", tranMaCungGoc)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: đọc mã cùng gốc: %w", err)
	}
	defer rows.Close()
	var ra []string
	for rows.Next() {
		var ma string
		if err := rows.Scan(&ma); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc mã: %w", err)
		}
		ra = append(ra, ma)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt mã: %w", err)
	}
	return ra, nil
}

// Chen writes one new unit. The id is minted by the caller, so tests can name the row.
//
// `cha_id` GOES IN AS NULL AT THE ROOT (nullif): the column is nullable and the read path folds NULL
// to "", so this is the same fold on the way in. An empty string would fail the composite foreign key
// (tenant_id, cha_id) → bo_phan and read as "parent not found".
func (s *BoPhanStore) Chen(ctx context.Context, tx *store.ScopedTx, bp domain.BoPhan) error {
	const stmt = `INSERT INTO bo_phan (tenant_id, id, ten, ma, cha_id, thu_tu)
		VALUES ($1,$2,$3,$4,nullif($5,''),$6)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), bp.ID, bp.Ten, bp.Ma, bp.ChaID, bp.ThuTu)
	if err != nil {
		return dichLoiGhiBoPhan("chèn bộ phận", err)
	}
	return nil
}

// CapNhat writes the three editable columns of one live unit: name, parent, rank.
//
// `ma`, `id` AND EVERY SOFT-DELETE COLUMN ARE ABSENT FROM THE SET CLAUSE, so this statement cannot
// renumber an issued code, resurrect a deleted unit or move a row to another commune whatever it is
// passed. `cap_nhat_luc = now()` is in the same statement for the reason sla_ghi.go gives.
func (s *BoPhanStore) CapNhat(ctx context.Context, tx *store.ScopedTx, bp domain.BoPhan) error {
	if bp.ID == "" {
		return ErrKhongTimThayBoPhan
	}
	const stmt = `UPDATE bo_phan SET ten = $3, cha_id = nullif($4,''), thu_tu = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), bp.ID, bp.Ten, bp.ChaID, bp.ThuTu)
	if err != nil {
		return dichLoiGhiBoPhan("cập nhật bộ phận", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("bo_phan: đếm dòng đã ghi: %w", err)
	}
	if n == 0 {
		// Cannot normally happen — the caller holds the row FOR UPDATE. Checked because a write path
		// reporting success having changed nothing is the one failure nobody notices.
		return ErrKhongTimThayBoPhan
	}
	return nil
}

// dichLoiGhiBoPhan turns a constraint violation into a sentinel. SUBSTRING match on the constraint
// name, because `bo_phan` is PARTITION BY HASH and PostgreSQL names the partition's copy
// (`bo_phan_p07_tenant_id_ma_key`) — see dichLoiGhiCanBo. Anything unrecognised is wrapped and
// passed on, never flattened into a business refusal.
func dichLoiGhiBoPhan(viec string, err error) error {
	switch tho := err.Error(); {
	case strings.Contains(tho, "tenant_id_ma_key"):
		return ErrMaBoPhanDaDung
	case strings.Contains(tho, "cha_id_fkey"):
		return ErrBoPhanChaKhongTonTai
	default:
		return fmt.Errorf("bo_phan: %s: %w", viec, err)
	}
}
