package store

// The merge link between petitions (migration 0037, ADR 0087) — SQL, and nothing else.
//
// THE SAME FIVE PROPERTIES AS xu_ly_phan_anh.go: the commune is $1 in every statement; nothing here
// opens a transaction (the use case does, and writes the audit entry inside it); every read excludes
// soft-deleted rows — except the one chain check that must see them, said where it happens; every UPDATE
// carries what the caller read under the lock, so a race is a refusal and never an overwrite; and the
// trigger `phieu_phan_anh_merge_guard` is the floor under all of it.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ByIDForUpdate is TheoMaTraCuuDeSua by INTERNAL id — for the one caller that holds an id and no code:
// unmerge, which reaches the main petition through the merged one's `merged_into` and must lock it
// FIRST (the lock order every act on a main petition uses — main, then its merged petitions).
func (s *PhieuPhanAnhStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (
	domain.PhieuPhanAnh, error) {

	const stmt = `SELECT ` + cotPhieu + ` FROM phieu_phan_anh
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	p, err := quetPhieu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	if err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc phiếu chính để sửa: %w", err)
	}
	return p, nil
}

// HasMergedPetitionsTx reports whether any petition — SOFT-DELETED ONES INCLUDED — is merged into `id`.
//
// NOT FILTERED ON deleted_at, on purpose and like the trigger's own check: a soft-deleted merged petition
// still points at its main, so a "no chains" check that skipped it would let a chain form through a row
// nobody sees (fail closed). The partial index `phieu_phan_anh_merged_children` serves it.
func (s *PhieuPhanAnhStore) HasMergedPetitionsTx(ctx context.Context, tx *store.ScopedTx, id string) (bool, error) {
	// ScopedTx.Query prefixes `WHERE tenant_id = $1`, bound from the transaction's commune.
	rows, err := tx.Query(ctx, "id", "phieu_phan_anh", "AND merged_into = $2 LIMIT 1", id)
	if err != nil {
		return false, fmt.Errorf("phieu_phan_anh: kiểm phiếu đã gộp vào: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("phieu_phan_anh: kiểm phiếu đã gộp vào: %w", err)
	}
	return found, nil
}

// MovePetitionDeadlineForMerge writes the MAIN petition's `han_xu_ly_xong` the merge decided
// (domain.CheckMerge) — BEFORE the link, which the trigger requires.
//
// THE SECOND WRITER OF `han_xu_ly_xong` after classification, and the statement itself can only move it
// EARLIER or out of "chưa có": `to` must be non-zero and below the stored value, or the stored value NULL
// (ADR 0087 stop condition #2). The value the caller read under the lock is in the WHERE clause (`IS NOT
// DISTINCT FROM`), so a classification racing in between is a refusal, never an overwrite.
func (s *PhieuPhanAnhStore) MovePetitionDeadlineForMerge(ctx context.Context, tx *store.ScopedTx,
	id string, from, to time.Time) error {

	if to.IsZero() {
		return errors.New("phieu_phan_anh: dời hạn khi gộp: hạn mới rỗng")
	}
	const stmt = `UPDATE phieu_phan_anh SET han_xu_ly_xong = $3, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND han_xu_ly_xong IS NOT DISTINCT FROM $4
		  AND (han_xu_ly_xong IS NULL OR $3 < han_xu_ly_xong)
		  AND merged_into IS NULL AND deleted_at IS NULL`

	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, to, khongThanhNull(from))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: dời hạn phiếu chính khi gộp: %w", err)
	}
	return doiMotDongPhieu(res, "dời hạn phiếu chính khi gộp")
}

// LinkMerge writes the link: the merged petition now points at `mainID`, made `at` by `by` (a staff
// BUSINESS code). The WHERE clause carries "not linked" and the status read under the lock.
func (s *PhieuPhanAnhStore) LinkMerge(ctx context.Context, tx *store.ScopedTx,
	id, mainID string, status domain.TrangThai, at time.Time, by string) error {

	const stmt = `UPDATE phieu_phan_anh
		SET merged_into = $3, merged_at = $4, merged_by = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND merged_into IS NULL AND trang_thai = $6 AND deleted_at IS NULL`

	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, mainID, at, by, string(status))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: gộp phiếu: %w", err)
	}
	return doiMotDongPhieu(res, "gộp phiếu")
}

// UnlinkMerge clears the link — who and when go with it (CHECK `phieu_phan_anh_merge_link_complete`);
// the history keeps them. The WHERE clause carries the main petition and the status read under the lock.
func (s *PhieuPhanAnhStore) UnlinkMerge(ctx context.Context, tx *store.ScopedTx,
	id, mainID string, status domain.TrangThai) error {

	const stmt = `UPDATE phieu_phan_anh
		SET merged_into = NULL, merged_at = NULL, merged_by = NULL, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND merged_into = $3 AND trang_thai = $4 AND deleted_at IS NULL`

	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, mainID, string(status))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: tách phiếu: %w", err)
	}
	return doiMotDongPhieu(res, "tách phiếu")
}

// MergedPetitionsCeiling bounds the merged petitions one main petition is read with — the follow-along
// step and the staff detail. With no chains a merged petition is one report of one incident; hundreds of
// reports of ONE pothole is not a shape this register has. Past it the read REFUSES rather than handing
// back a list with petitions silently missing — which for the follow-along would leave a citizen behind.
const MergedPetitionsCeiling = 200

// ErrTooManyMergedPetitions — more than MergedPetitionsCeiling live petitions are merged into one main.
var ErrTooManyMergedPetitions = fmt.Errorf("phieu_phan_anh: quá %d phiếu gộp vào một phiếu chính", MergedPetitionsCeiling)

// MergedPetitionsForUpdate reads every LIVE petition merged into `mainID`, under lock, in one statement
// — the follow-along step's batch (load-data-once: one read, whatever their number). The caller holds the
// main petition's lock already: main first, then its merged petitions, the order every act uses.
func (s *PhieuPhanAnhStore) MergedPetitionsForUpdate(ctx context.Context, tx *store.ScopedTx, mainID string) (
	[]domain.PhieuPhanAnh, error) {

	// ScopedTx.Query prefixes `WHERE tenant_id = $1`, bound from the transaction's commune.
	rows, err := tx.Query(ctx, cotPhieu, "phieu_phan_anh",
		"AND merged_into = $2 AND deleted_at IS NULL ORDER BY vao_so_luc, id LIMIT $3 FOR UPDATE",
		mainID, MergedPetitionsCeiling+1)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: đọc phiếu đã gộp: %w", err)
	}
	defer rows.Close()
	var out []domain.PhieuPhanAnh
	for rows.Next() {
		p, err := quetPhieu(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		if len(out) > MergedPetitionsCeiling {
			return nil, ErrTooManyMergedPetitions
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: đọc phiếu đã gộp: %w", err)
	}
	return out, nil
}

// FollowMain moves ONE merged petition along with its main petition (ADR 0087 §2) — into
// `cho-dan-xac-nhan`, or into `da-dong` with the MAIN petition's result (the same sentence, ADR 0087 §2).
//
// TWO LITERAL STATEMENTS chosen by `to`, the ownerFilter shape: the status is never a parameter, so no
// caller can move a merged petition anywhere else. `xu_ly_xong_luc` keeps the merged petition's OWN
// instant when it has one, else takes the main's — the work on the incident was finished then, and a
// closed petition with no finishing instant would read as overdue for ever (domain.QuaHan).
//
// THE WHERE CLAUSE CARRIES THE LINK AND THE STATUS read under the lock: a petition unmerged or moved
// meanwhile is a refusal, never moved by an act on a main petition it no longer belongs to.
func (s *PhieuPhanAnhStore) FollowMain(ctx context.Context, tx *store.ScopedTx, id, mainID string,
	from, to domain.TrangThai, workDoneAt time.Time, result string, closedAt time.Time) error {

	var (
		res sql.Result
		err error
	)
	switch to {
	case domain.ChoDanXacNhan:
		const stmt = `UPDATE phieu_phan_anh
			SET trang_thai = 'cho-dan-xac-nhan', xu_ly_xong_luc = COALESCE(xu_ly_xong_luc, $3),
			    cap_nhat_luc = now()
			WHERE tenant_id = $1 AND id = $2 AND merged_into = $4 AND trang_thai = $5 AND deleted_at IS NULL`
		res, err = tx.Exec(ctx, stmt, string(tx.TenantID()), id, khongThanhNull(workDoneAt), mainID, string(from))
	case domain.DaDong:
		const stmt = `UPDATE phieu_phan_anh
			SET trang_thai = 'da-dong', ket_qua_xu_ly = $3, dong_luc = $4,
			    xu_ly_xong_luc = COALESCE(xu_ly_xong_luc, $5), cap_nhat_luc = now()
			WHERE tenant_id = $1 AND id = $2 AND merged_into = $6 AND trang_thai = $7 AND deleted_at IS NULL`
		res, err = tx.Exec(ctx, stmt, string(tx.TenantID()), id, result, closedAt, khongThanhNull(workDoneAt),
			mainID, string(from))
	default:
		return fmt.Errorf("phieu_phan_anh: phiếu gộp không đi theo phiếu chính vào %q", to)
	}
	if err != nil {
		// NOT the result text: free text about one case (rule 3, forbidden #3).
		return fmt.Errorf("phieu_phan_anh: phiếu gộp đi theo phiếu chính: %w", err)
	}
	return doiMotDongPhieu(res, "phiếu gộp đi theo phiếu chính")
}

// MergeLinks reads the CODES the staff detail shows for `p`: its main petition's when it is merged, and
// the live petitions merged into it. ONE statement for both halves. A soft-deleted main petition is not
// named (rule 7, invariant 2) — the link is then shown without a code, which an officer can see is odd.
func (s *PhieuPhanAnhStore) MergeLinks(ctx context.Context, p domain.PhieuPhanAnh) (domain.MergeLinks, error) {
	out := domain.MergeLinks{}
	// $3 = the main petition's id, or '' on a main petition — which matches no row (ids are never blank).
	rows, err := s.db.For(ctx).Query(ctx, "ma_tra_cuu, id = $3", "phieu_phan_anh",
		"AND deleted_at IS NULL AND (merged_into = $2 OR id = $3) ORDER BY vao_so_luc, id LIMIT $4",
		p.ID, p.MergedInto, MergedPetitionsCeiling+2)
	if err != nil {
		return out, fmt.Errorf("phieu_phan_anh: đọc liên kết gộp: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			code   string
			isMain bool
		)
		if err := rows.Scan(&code, &isMain); err != nil {
			return out, fmt.Errorf("phieu_phan_anh: đọc liên kết gộp: %w", err)
		}
		if isMain {
			out.MainCode = code
			continue
		}
		out.ChildCodes = append(out.ChildCodes, code)
		if len(out.ChildCodes) > MergedPetitionsCeiling {
			return domain.MergeLinks{}, ErrTooManyMergedPetitions
		}
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("phieu_phan_anh: đọc liên kết gộp: %w", err)
	}
	return out, nil
}

// --- the suspected-duplicate search (ADR 0087 §6) ------------------------------------------------------

// DuplicateQuery is the pre-filter of the suspected-duplicate search: the box around the petition's place
// and the window around when it was reported. Built by app.DuplicateCandidates; every value is BOUND.
type DuplicateQuery struct {
	ExcludeID  string
	Box        domain.GeoBox
	ReportedAt time.Time // the petition's own `goc_dem_han`
	WindowDays int       // the commune's `duplicate_window_days`, CALENDAR days between reports
	Field      string    // "" = the petition is unclassified: any field
	Limit      int
}

// duplicateOpenStatuses is domain.MergeOpen as SQL — the four statuses, the only petitions a merge can
// take. Built from the domain constants so a renamed code is a compile error, not a silent miss.
var duplicateOpenStatuses = `trang_thai IN ('` + string(domain.DaTiepNhan) + `', '` + string(domain.DangPhanLoai) +
	`', '` + string(domain.DaChuyenXuLy) + `', '` + string(domain.DangXuLy) + `')`

// DuplicateCandidates reads the petitions in the box and the window that a merge could take: live, not
// themselves merged, located, unresolved, NEVER `can-bo` (ADR 0087 §5 — a staff-conduct petition is never
// merged, so suggesting it is a suggestion nobody can act on), and of the same field when both are
// classified. The partial index `phieu_phan_anh_duplicate_candidates` carries the first four.
//
// THE WINDOW IS CALENDAR DAYS OF REPORTS, not a processing deadline — migration 0038's header: working
// hours (ADR 0007) do not apply to "reported within D days". Computed by PostgreSQL from the stored instant.
//
// ORDERED, BOUNDED: the closest in time first, `Limit` rows at most. The caller refines by distance.
func (s *PhieuPhanAnhStore) DuplicateCandidates(ctx context.Context, q DuplicateQuery) ([]domain.PhieuPhanAnh, error) {
	if q.Limit < 1 || q.WindowDays < 1 {
		return nil, errors.New("phieu_phan_anh: tìm phiếu nghi trùng: giới hạn hoặc khoảng ngày không hợp lệ")
	}
	tail := `AND deleted_at IS NULL AND merged_into IS NULL AND lat IS NOT NULL AND lng IS NOT NULL` +
		` AND id <> $2` +
		` AND goc_dem_han >= $3::timestamptz - make_interval(days => $4::int)` +
		` AND goc_dem_han <= $3::timestamptz + make_interval(days => $4::int)` +
		` AND lat BETWEEN $5 AND $6 AND lng BETWEEN $7 AND $8` +
		` AND ` + duplicateOpenStatuses +
		` AND (linh_vuc IS NULL OR linh_vuc <> '` + domain.LinhVucHanChe + `')`
	args := []any{q.ExcludeID, q.ReportedAt, q.WindowDays, q.Box.MinLat, q.Box.MaxLat, q.Box.MinLng, q.Box.MaxLng}
	if q.Field != "" {
		args = append(args, q.Field)
		tail += ` AND (linh_vuc IS NULL OR linh_vuc = $` + strconv.Itoa(len(args)+1) + `)`
	}
	args = append(args, q.Limit)
	tail += ` ORDER BY abs(extract(epoch FROM goc_dem_han - $3::timestamptz)), id LIMIT $` + strconv.Itoa(len(args)+1)

	rows, err := s.db.For(ctx).Query(ctx, cotPhieu, "phieu_phan_anh", tail, args...)
	if err != nil {
		// NOT the coordinates: a located report is personal data (rule 3).
		return nil, fmt.Errorf("phieu_phan_anh: tìm phiếu nghi trùng: %w", err)
	}
	defer rows.Close()
	out := []domain.PhieuPhanAnh{}
	for rows.Next() {
		p, err := quetPhieu(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: tìm phiếu nghi trùng: %w", err)
	}
	return out, nil
}

// --- the history -----------------------------------------------------------------------------------------

// PetitionMergeEventStore is the only path to `petition_merge_event` (migration 0037): APPEND-ONLY, so it
// has one write and it takes the transaction — the history row, the link and the audit entry commit
// together or not at all (rule 6, invariant 3). There is no update and no delete method; the table's
// trigger refuses both anyway.
type PetitionMergeEventStore struct {
	db *store.DB
}

func NewPetitionMergeEventStore(db *store.DB) *PetitionMergeEventStore {
	return &PetitionMergeEventStore{db: db}
}

// Append records one merge or unmerge INSIDE the caller's transaction. A zero deadline is SQL NULL
// ("chưa có"); an empty reason is NULL — the CHECK refuses a blank one.
func (s *PetitionMergeEventStore) Append(ctx context.Context, tx *store.ScopedTx, e domain.PetitionMergeEvent) error {
	const stmt = `INSERT INTO petition_merge_event (tenant_id, id, petition_id, main_petition_id, kind,
		performed_at, performed_by, reason, main_deadline_before, main_deadline_after)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), e.ID, e.PetitionID, e.MainPetitionID, string(e.Kind),
		e.PerformedAt, e.PerformedBy, rongThanhNull(e.Reason),
		khongThanhNull(e.MainDeadlineBefore), khongThanhNull(e.MainDeadlineAfter))
	if err != nil {
		// NOT the reason: staff free text that may quote a report (rule 3).
		return fmt.Errorf("petition_merge_event: ghi: %w", err)
	}
	return nil
}
