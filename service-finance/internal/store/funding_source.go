package store

// Funding sources: the reads §6's block needs beyond TienDoTheoNguon, and the WRITE path of the
// catalogue and its per-year granted amounts (migration 0013, user decisions 06/10/2026).
//
// THE PROPERTIES chung_tu_giai_ngan.go lists hold here unchanged: the commune is $1 in every statement,
// taken from the context or the transaction and never from a parameter (rule 1, invariants 4 and 5);
// nothing here opens a transaction (internal/app does, and writes the audit entry inside it — rule 6,
// invariant 3); every read excludes soft-deleted rows; there is no hard delete.
//
// WHAT IS ABSENT ON PURPOSE: no rename and no remove of a source (user decision 06/10/2026, prototype
// behaviour), and no DELETE of a granted amount — "nothing granted" is the figure 0, written by an
// audited UPDATE (0013's note on the table).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

var (
	// ErrFundingSourceNotFound — no LIVE source with this id in THIS commune. A source of another
	// commune is indistinguishable from one that does not exist, because every statement binds
	// tenant_id = $1; the caller answers 404 either way (rule 4, forbidden #2, applied between communes).
	ErrFundingSourceNotFound = errors.New("nguon_von: không có nguồn vốn này trong xã")

	// ErrFundingSourceNameTaken — the commune already has a source by this name. The check COUNTS
	// SOFT-DELETED ROWS, because `nguon_von_name_unique` does (0013's header, accepted by the user).
	ErrFundingSourceNameTaken = errors.New("nguon_von: xã đã có nguồn vốn mang tên này")

	// ErrFundingSourceCatalogueFull — the commune holds TranNguonVonMotNam live sources. Refused at the
	// write so the read never has to refuse: every §6 read refuses past that ceiling rather than
	// truncate (ErrQuaNhieuNguonVon), so a create that crossed it would break the commune's whole block.
	ErrFundingSourceCatalogueFull = errors.New("nguon_von: danh mục nguồn vốn của xã đã đủ trần")
)

// --- reads ---------------------------------------------------------------------------------------

// UnattributedDisbursed is §13 rule 6's warning for one budget year — "Còn … đã chi nhưng chưa ghi rút
// từ nguồn nào": the live vouchers of the year's live projects that name NO source.
//
// THE SAME COUNTING RULE AS THE PROJECT'S "đã giải ngân", and that is what makes the warning
// reconcile: store/du_an.go tongChungTu (:80-95) sums EVERY state including `ke-toan-nhap` (§11
// "WHERE trạng thái ≥ kế toán nhập", the first state) and excludes soft-deleted vouchers; the year
// comes from the PROJECT (`da.nam`), as tongChungTuTheoNguon takes it. So this figure plus the
// cards' "đã giải ngân" equals the commune's disbursed total for the year — and a status filter added
// here alone would break that equality with every number still looking plausible.
//
// tenant_id = $1 ON BOTH TABLES, for the reason tongPhanBoTheoNguon gives.
func (s *NguonVonStore) UnattributedDisbursed(ctx context.Context, year int) (domain.Dong, error) {
	if year == 0 {
		return 0, ErrThieuNamNganSach
	}
	const stmt = `SELECT COALESCE(SUM(ct.so_tien), 0)
		FROM chung_tu_giai_ngan ct
		JOIN du_an da
		  ON da.tenant_id = ct.tenant_id AND da.tenant_id = $1 AND da.id = ct.du_an_id
		WHERE ct.tenant_id = $1 AND ct.deleted_at IS NULL AND ct.nguon_von_id IS NULL
		  AND da.deleted_at IS NULL AND da.nam = $2`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, year)
	if err != nil {
		return 0, fmt.Errorf("nguon_von: đọc chi chưa ghi nguồn: %w", err)
	}
	total, err := scanCount(rows)
	if err != nil {
		return 0, fmt.Errorf("nguon_von: đọc chi chưa ghi nguồn: %w", err)
	}
	return domain.Dong(total), nil
}

// sourceProjects lists the year's live projects holding an allocation line for ONE source, each with
// that line and the vouchers it drew FROM THAT SOURCE.
//
// MEMBERSHIP IS THE ALLOCATION LINE (prototype repository.list_items, `source_id` filter: "dự án có
// nguồn ấy trong bảng phân bổ"). Since 0013 a project holds at most one line per source, so a project
// appears at most once and no SUM over lines is needed.
//
// THE VOUCHER SUBQUERY IS tongChungTuTheoNguon's RULE, per project: every state counts, soft-deleted
// vouchers do not, and only vouchers naming this source — so the rows add up to the card's figure,
// minus DisbursedWithoutAllocation.
//
// ORDER BY da.ma — unique per commune, so the order is total. LIMIT is the projects ceiling plus one
// (a source can fund at most every project of the year).
const sourceProjects = `SELECT da.id, da.ma, da.ten, da.ke_hoach_von_nam, pb.so_tien_phan_bo,
		       COALESCE(ct.da_giai_ngan, 0)
		FROM phan_bo_nguon_von pb
		JOIN du_an da
		  ON da.tenant_id = pb.tenant_id AND da.tenant_id = $1 AND da.id = pb.du_an_id
		LEFT JOIN (SELECT c.tenant_id, c.du_an_id, SUM(c.so_tien) AS da_giai_ngan
		             FROM chung_tu_giai_ngan c
		            WHERE c.tenant_id = $1 AND c.deleted_at IS NULL AND c.nguon_von_id = $2
		            GROUP BY c.tenant_id, c.du_an_id) ct
		  ON ct.tenant_id = da.tenant_id AND ct.du_an_id = da.id
		WHERE pb.tenant_id = $1 AND pb.nguon_von_id = $2 AND pb.deleted_at IS NULL
		  AND da.deleted_at IS NULL AND da.nam = $3
		ORDER BY da.ma, da.id
		LIMIT $4`

// disbursedWithoutAllocation — money drawn from the source this year by projects with NO line for it.
// See domain.FundingSourceProjects.DisbursedWithoutAllocation for why the breakdown needs it.
const disbursedWithoutAllocation = `SELECT COALESCE(SUM(ct.so_tien), 0)
		FROM chung_tu_giai_ngan ct
		JOIN du_an da
		  ON da.tenant_id = ct.tenant_id AND da.tenant_id = $1 AND da.id = ct.du_an_id
		WHERE ct.tenant_id = $1 AND ct.deleted_at IS NULL AND ct.nguon_von_id = $2
		  AND da.deleted_at IS NULL AND da.nam = $3
		  AND NOT EXISTS (SELECT 1 FROM phan_bo_nguon_von pb
		                   WHERE pb.tenant_id = $1 AND pb.du_an_id = da.id
		                     AND pb.nguon_von_id = $2 AND pb.deleted_at IS NULL)`

// SourceProjects reads the breakdown behind one source's card for one budget year (prototype
// SourceItemsDialog). A source that is not a live source of THIS commune is ErrFundingSourceNotFound —
// checked first, so an unknown id is a 404 rather than an empty, plausible-looking list.
//
// THREE STATEMENTS, NOT ONE TRANSACTION — the same footing as every other read in this service (the
// project list reads its threshold in a second statement). A voucher committed between two of them
// can make one response's figures disagree by that voucher until the next read; nothing is stored
// from it.
func (s *NguonVonStore) SourceProjects(ctx context.Context, sourceID string,
	year int) (domain.FundingSourceProjects, error) {

	if year == 0 {
		return domain.FundingSourceProjects{}, ErrThieuNamNganSach
	}
	rows, err := s.db.For(ctx).Query(ctx, `id, ten, thu_tu`, "nguon_von",
		`AND id = $2 AND deleted_at IS NULL`, sourceID)
	if err != nil {
		return domain.FundingSourceProjects{}, fmt.Errorf("nguon_von: đọc nguồn vốn: %w", err)
	}
	src, err := scanOneSource(rows)
	if err != nil {
		return domain.FundingSourceProjects{}, err
	}
	src.Nam = year

	rows, err = s.db.For(ctx).QueryJoin(ctx, sourceProjects, sourceID, year, TranDuAnMotNam+1)
	if err != nil {
		return domain.FundingSourceProjects{}, fmt.Errorf("nguon_von: đọc dự án theo nguồn: %w", err)
	}
	defer rows.Close()

	out := domain.FundingSourceProjects{Source: src, Projects: make([]domain.FundingSourceProject, 0, 8)}
	for rows.Next() {
		var (
			p                             domain.FundingSourceProject
			planned, allocated, disbursed int64
		)
		if err := rows.Scan(&p.ProjectID, &p.Code, &p.Name, &planned, &allocated, &disbursed); err != nil {
			return domain.FundingSourceProjects{}, fmt.Errorf("nguon_von: đọc dòng dự án theo nguồn: %w", err)
		}
		p.PlannedAmount, p.AllocatedAmount, p.DisbursedAmount =
			domain.Dong(planned), domain.Dong(allocated), domain.Dong(disbursed)
		out.Projects = append(out.Projects, p)
	}
	if err := rows.Err(); err != nil {
		return domain.FundingSourceProjects{}, fmt.Errorf("nguon_von: duyệt dự án theo nguồn: %w", err)
	}
	if len(out.Projects) > TranDuAnMotNam {
		// Refused, not truncated: these rows are added up against the card on the same screen.
		return domain.FundingSourceProjects{}, ErrQuaNhieuDuAn
	}

	rest, err := s.db.For(ctx).QueryJoin(ctx, disbursedWithoutAllocation, sourceID, year)
	if err != nil {
		return domain.FundingSourceProjects{}, fmt.Errorf("nguon_von: đọc chi ngoài phân bổ: %w", err)
	}
	without, err := scanCount(rest)
	if err != nil {
		return domain.FundingSourceProjects{}, fmt.Errorf("nguon_von: đọc chi ngoài phân bổ: %w", err)
	}
	out.DisbursedWithoutAllocation = domain.Dong(without)
	return out, nil
}

// scanOneSource reads (id, ten, thu_tu) of one catalogue row; no row is ErrFundingSourceNotFound.
func scanOneSource(rows *sql.Rows) (domain.NguonVon, error) {
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NguonVon{}, fmt.Errorf("nguon_von: đọc nguồn vốn: %w", err)
		}
		return domain.NguonVon{}, ErrFundingSourceNotFound
	}
	var nv domain.NguonVon
	if err := rows.Scan(&nv.ID, &nv.Ten, &nv.ThuTu); err != nil {
		return domain.NguonVon{}, fmt.Errorf("nguon_von: đọc dòng nguồn vốn: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.NguonVon{}, fmt.Errorf("nguon_von: duyệt nguồn vốn: %w", err)
	}
	return nv, nil
}

// scanCount reads the single integer a COUNT / SUM statement answers.
func scanCount(rows *sql.Rows) (int64, error) {
	defer rows.Close()
	var n int64
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, err
		}
	}
	return n, rows.Err()
}

// --- writes --------------------------------------------------------------------------------------

// FundingSourceWriteStore writes one commune's funding source catalogue and its granted amounts.
// EVERY METHOD TAKES THE CALLER'S TRANSACTION — there is no signature here that could write a source
// in one transaction and its audit entry in another. Every statement binds the commune as $1 from
// tx.TenantID(), never from a parameter.
type FundingSourceWriteStore struct {
	db *store.DB
}

func NewFundingSourceWriteStore(db *store.DB) *FundingSourceWriteStore {
	return &FundingSourceWriteStore{db: db}
}

// nameTaken has NO `deleted_at` PREDICATE, because `nguon_von_name_unique` has none: the question is
// the key's question, asked first so the refusal arrives as a sentence rather than a constraint
// violation.
const nameTaken = `SELECT count(*) FROM nguon_von WHERE tenant_id = $1 AND ten = $2`

// NameTaken reports whether the commune has EVER had a source by this exact name.
func (s *FundingSourceWriteStore) NameTaken(ctx context.Context, tx *store.ScopedTx, name string) (bool, error) {
	rows, err := tx.Underlying().QueryContext(ctx, nameTaken, string(tx.TenantID()), name)
	if err != nil {
		return false, fmt.Errorf("nguon_von: kiểm tên trùng: %w", err)
	}
	n, err := scanCount(rows)
	if err != nil {
		return false, fmt.Errorf("nguon_von: kiểm tên trùng: %w", err)
	}
	return n > 0, nil
}

const countLiveSources = `SELECT count(*) FROM nguon_von WHERE tenant_id = $1 AND deleted_at IS NULL`

// CountLive counts the commune's live sources — what the §6 reads are bounded by.
func (s *FundingSourceWriteStore) CountLive(ctx context.Context, tx *store.ScopedTx) (int, error) {
	rows, err := tx.Underlying().QueryContext(ctx, countLiveSources, string(tx.TenantID()))
	if err != nil {
		return 0, fmt.Errorf("nguon_von: đếm nguồn vốn: %w", err)
	}
	n, err := scanCount(rows)
	if err != nil {
		return 0, fmt.Errorf("nguon_von: đếm nguồn vốn: %w", err)
	}
	return int(n), nil
}

// insertSource appends the source AT THE END of the commune's order: `thu_tu` = the commune's highest
// + 1 (1 for the first). The create body carries no order (the decided body shape); appending keeps
// every existing card where the commune put it. Two concurrent creates may take the same `thu_tu` —
// harmless, the read orders by (thu_tu, ten) and `ten` is unique.
//
// Every operand is cast, so PostgreSQL never has to guess a parameter's type inside INSERT … SELECT.
const insertSource = `INSERT INTO nguon_von (tenant_id, id, ten, thu_tu)
	SELECT $1::text, $2::text, $3::text, COALESCE(MAX(thu_tu), 0) + 1
	  FROM nguon_von WHERE tenant_id = $1::text
	RETURNING thu_tu`

// InsertSource records one new catalogue row and returns the order it was given.
//
// A UNIQUE VIOLATION IS ErrFundingSourceNameTaken: NameTaken has already been asked inside the same
// transaction, so the only way here is a concurrent create of the same name committing first — and
// "the commune already has a source by this name" is then the truth.
func (s *FundingSourceWriteStore) InsertSource(ctx context.Context, tx *store.ScopedTx,
	id, name string) (int, error) {

	var order int
	err := tx.Underlying().QueryRowContext(ctx, insertSource, string(tx.TenantID()), id, name).Scan(&order)
	if isUniqueViolation(err) {
		return 0, fmt.Errorf("nguon_von: thêm nguồn vốn: %w", ErrFundingSourceNameTaken)
	}
	if err != nil {
		return 0, fmt.Errorf("nguon_von: thêm nguồn vốn: %w", err)
	}
	return order, nil
}

// sourceForUpdate — FOR UPDATE on the parent of a partitioned table locks the row in its partition.
const sourceForUpdate = `SELECT id, ten, thu_tu FROM nguon_von
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

// SourceForUpdate reads one live source of this commune and LOCKS it until the transaction ends.
//
// THE LOCK IS WHAT MAKES THE YEAR-AMOUNT UPSERT SAFE: "read the year's row, then insert or update it"
// is a read-decide-write on a row that may not exist yet, which no lock on that row can hold. Every
// writer of a source's amounts locks the SOURCE first, so two of them queue instead of both inserting
// (the second would hit the unique key as a 500) or both updating from the same "before".
func (s *FundingSourceWriteStore) SourceForUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.NguonVon, error) {

	rows, err := tx.Underlying().QueryContext(ctx, sourceForUpdate, string(tx.TenantID()), id)
	if err != nil {
		return domain.NguonVon{}, fmt.Errorf("nguon_von: khoá nguồn vốn: %w", err)
	}
	return scanOneSource(rows)
}

const annualAmount = `SELECT id, granted_amount FROM funding_source_annual_amounts
	WHERE tenant_id = $1 AND funding_source_id = $2 AND year = $3`

// AnnualAmount reads the amount granted to one source for one year; found = false when the commune
// has entered none (which every reader treats as 0 — 0013's note on the table).
func (s *FundingSourceWriteStore) AnnualAmount(ctx context.Context, tx *store.ScopedTx,
	sourceID string, year int) (domain.FundingSourceAnnualAmount, bool, error) {

	rows, err := tx.Underlying().QueryContext(ctx, annualAmount, string(tx.TenantID()), sourceID, year)
	if err != nil {
		return domain.FundingSourceAnnualAmount{}, false, fmt.Errorf("nguon_von: đọc vốn được giao: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.FundingSourceAnnualAmount{}, false, fmt.Errorf("nguon_von: đọc vốn được giao: %w", err)
		}
		return domain.FundingSourceAnnualAmount{}, false, nil
	}
	a := domain.FundingSourceAnnualAmount{FundingSourceID: sourceID, Year: year}
	var amount int64
	if err := rows.Scan(&a.ID, &amount); err != nil {
		return domain.FundingSourceAnnualAmount{}, false, fmt.Errorf("nguon_von: đọc dòng vốn được giao: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.FundingSourceAnnualAmount{}, false, fmt.Errorf("nguon_von: duyệt vốn được giao: %w", err)
	}
	a.GrantedAmount = domain.Dong(amount)
	return a, true, nil
}

const insertAnnualAmount = `INSERT INTO funding_source_annual_amounts
	(tenant_id, id, funding_source_id, year, granted_amount) VALUES ($1, $2, $3, $4, $5)`

// InsertAnnualAmount records the first granted amount of one source for one year.
func (s *FundingSourceWriteStore) InsertAnnualAmount(ctx context.Context, tx *store.ScopedTx,
	a domain.FundingSourceAnnualAmount) error {

	_, err := tx.Exec(ctx, insertAnnualAmount, string(tx.TenantID()),
		a.ID, a.FundingSourceID, a.Year, int64(a.GrantedAmount))
	if err != nil {
		return fmt.Errorf("nguon_von: ghi vốn được giao: %w", err)
	}
	return nil
}

// updateAnnualAmount — the only UPDATE of this table: one figure of one row, filtered by commune AND id.
const updateAnnualAmount = `UPDATE funding_source_annual_amounts
	SET granted_amount = $3, updated_at = now()
	WHERE tenant_id = $1 AND id = $2`

// UpdateAnnualAmount corrects the granted amount of an existing (source, year) row. Zero rows affected
// is an error, never a silent success: the row was read under the source's lock a moment ago.
func (s *FundingSourceWriteStore) UpdateAnnualAmount(ctx context.Context, tx *store.ScopedTx,
	id string, amount domain.Dong) error {

	res, err := tx.Exec(ctx, updateAnnualAmount, string(tx.TenantID()), id, int64(amount))
	if err != nil {
		return fmt.Errorf("nguon_von: sửa vốn được giao: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("nguon_von: sửa vốn được giao: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("nguon_von: sửa vốn được giao: %d dòng đổi, muốn 1", n)
	}
	return nil
}
