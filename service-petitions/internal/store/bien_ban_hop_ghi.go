package store

// The WRITE half of the meeting-minutes register — recording minutes (§4), appending a conclusion
// (§2's last row), and the two locking reads the rules above them decide on. SQL, and nothing else.
//
// FIVE THINGS HOLD IN EVERY STATEMENT IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1, from tx.TenantID() or from *store.Scoped (rule 1, invariants 4 and 5). It
//     is never a parameter here, so no caller can write into another commune's register.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may. Every write below takes a
//     *store.ScopedTx, which is what makes "write the record now, write the trail afterwards if it
//     works" impossible to express.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — WITH ONE DELIBERATE EXCEPTION,
//     ThuTuLonNhat, which counts them on purpose. Its own comment says why.
//  4. THERE IS NO `DELETE` AND NO `UPDATE` IN THIS FILE. Minutes and conclusions are archival
//     records; a hard delete is refused by the `ho_so_luu_tru_cam_xoa_cung` trigger anyway, and
//     whether a conclusion may be EDITED once tasks point at it is an open question migration 0007
//     deliberately left unanswered. Writing an UPDATE here would answer it.
//  5. `thu_tu` IS NEVER RECOMPUTED. §7.2 appends and never renumbers, and rule 7, invariant 3 is the
//     reason: an issued number is not reissued, even after a soft delete.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

var (
	// ErrBienBanKhongTonTai — no live minutes with that id in this commune.
	//
	// ONE SENTINEL FOR THREE CAUSES — no such id, another commune's id, and a soft-deleted record —
	// and the handler answers 404 to all three. Telling them apart tells a caller which minutes exist
	// in a register they are not reading.
	ErrBienBanKhongTonTai = errors.New("bien_ban_hop: không có biên bản họp")

	// ErrKetLuanKhongTonTai — that meeting has no live conclusion with that ordinal.
	//
	// A SOFT-DELETED CONCLUSION TAKES THIS ANSWER TOO, and it is the correct one: the number stays
	// taken for ever (it is still printed on the minutes and still pointed at by the tasks split from
	// it), but nothing new may be hung off a conclusion that has been removed from the register.
	ErrKetLuanKhongTonTai = errors.New("ket_luan_hop: không có kết luận này trong biên bản")
)

// cotKetLuan IS READ BY POSITION in quetKetLuan below.
//
// THE TWO COUNTERS ARE NOT IN IT. `so_nhiem_vu` / `so_nhiem_vu_xong` are `count(*)` over the task
// register (see cauKetLuanKemDem), not columns — migration 0007 refuses a counter column, and a read
// that returned zeros for them here would look exactly like a conclusion nobody has split.
const cotKetLuan = `id, bien_ban_id, thu_tu, noi_dung, tao_luc`

// --- the locking read -----------------------------------------------------------------------------

// BienBanTheoIDDeSua reads one live meeting INSIDE the caller's transaction and holds it until the
// transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Appending a conclusion is a
// read-decide-write: read the highest ordinal, mint the next one, insert. Two clerks typing a
// conclusion into the same minutes at the same moment would otherwise both read ③ and both mint ④ —
// and `UNIQUE (tenant_id, bien_ban_id, thu_tu)` would refuse the second with a constraint error
// rather than giving it ⑤. Holding the PARENT row serialises them on the meeting, which is the only
// row both acts have in common.
//
// It also closes the window in which the minutes could be removed between "these minutes exist" and
// the INSERT that hangs a conclusion off them.
//
// THE MINUTES BODY AND THE ATTENDEES ARE NOT SELECTED (cotBienBan). Nothing in the write path needs
// them, and the full text of a meeting has no business being loaded into a transaction that is about
// to write one short row.
func (s *BienBanHopStore) BienBanTheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (
	domain.BienBanHop, error) {

	const stmt = `SELECT ` + cotBienBan + ` FROM bien_ban_hop
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	b, err := quetBienBan(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BienBanHop{}, ErrBienBanKhongTonTai
	}
	if err != nil {
		// NOT the id and NOT any column of the row: an error travels into centralised logging across
		// every commune at once, and this register's columns hold minutes (rule 3).
		return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc biên bản để ghi: %w", err)
	}
	return b, nil
}

// KetLuanTheoThuTu reads ONE live conclusion of one meeting, BY ITS ORDINAL — the ① ② ③ of §2, which
// is what the split route carries in its path.
//
// # IT IS THE ONLY READ IN THIS FILE THAT RUNS OUTSIDE A TRANSACTION, AND THAT IS DELIBERATE
//
// Its caller (app.TachKetLuanThanhNhiemVu) uses it to check that the conclusion being split really
// exists in THIS commune before delegating to the task-creation use case — which opens its own
// transaction and writes the task, the timeline row and the audit entry inside it. Locking the
// conclusion here would hold a row lock across a second transaction, which is worse than the window
// it would close; and the window itself is empty today, because no route in this service can
// soft-delete a conclusion at all. The consequence if that changes is stated where the caller is.
//
// THE PAIR IS MATCHED, not just the ordinal: `(bien_ban_id, thu_tu)` is what `UNIQUE (tenant_id,
// bien_ban_id, thu_tu)` guarantees, and an ordinal alone means nothing — every meeting has a ①.
func (s *BienBanHopStore) KetLuanTheoThuTu(ctx context.Context, bienBanID string, thuTu int) (
	domain.KetLuanHop, error) {

	rows, err := s.db.For(ctx).Query(ctx, cotKetLuan, "ket_luan_hop",
		`AND bien_ban_id = $2 AND thu_tu = $3 AND deleted_at IS NULL`, bienBanID, thuTu)
	if err != nil {
		return domain.KetLuanHop{}, fmt.Errorf("ket_luan_hop: đọc kết luận: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.KetLuanHop{}, fmt.Errorf("ket_luan_hop: đọc kết luận: %w", err)
		}
		return domain.KetLuanHop{}, ErrKetLuanKhongTonTai
	}
	k, err := quetKetLuan(rows)
	if err != nil {
		return domain.KetLuanHop{}, err
	}
	return k, nil
}

// ThuTuLonNhat reads the HIGHEST ordinal ever issued inside one meeting.
//
// # IT DELIBERATELY DOES NOT FILTER `deleted_at`, AND THAT IS THE WHOLE POINT
//
// A soft-deleted conclusion keeps its number: the unique key counts deleted rows (migration 0007
// says why in full), the printed minutes still say "②", and the tasks split from it still point at
// it. Counting only live rows would mint a SECOND ② whose tasks are indistinguishable from the
// first's on every screen that shows the ordinal. A gap after a removal is the correct outcome
// (§7.2: conclusions are APPENDED, never renumbered).
//
// ZERO MEANS "NO CONCLUSION HAS EVER BEEN RECORDED", which domain.ThuTuKetLuanTiepTheo turns into ①.
func (s *BienBanHopStore) ThuTuLonNhat(ctx context.Context, tx *store.ScopedTx, bienBanID string) (
	int, error) {

	const stmt = `SELECT COALESCE(MAX(thu_tu), 0) FROM ket_luan_hop
		WHERE tenant_id = $1 AND bien_ban_id = $2`

	var so int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), bienBanID).Scan(&so)
	if err != nil {
		return 0, fmt.Errorf("ket_luan_hop: đọc số thứ tự lớn nhất: %w", err)
	}
	return so, nil
}

// --- the writes ----------------------------------------------------------------------------------

// TaoBienBan inserts one meeting's minutes INSIDE the caller's transaction.
//
// THE CALLER WRITES THE AUDIT ENTRY IN THAT SAME TRANSACTION. This method deliberately does not: the
// entry needs the actor and the IP, which are facts about the REQUEST and not about the record, and
// a store that invented them would produce a trail naming nobody.
//
// `dinh_kem` IS NOT IN THE COLUMN LIST, so the column takes its `'[]'` default. There is no file
// store in this repository, so there is nothing to put in it — and a column written with an empty
// array by hand would look like a decision rather than an absence.
//
// `deleted_at`, `tao_luc` AND `cap_nhat_luc` ARE NOT WRITTEN EITHER: the first must be NULL on a new
// row, and the other two have defaults in the schema, which keeps one clock for the whole register.
func (s *BienBanHopStore) TaoBienBan(ctx context.Context, tx *store.ScopedTx,
	b domain.BienBanHop) error {

	const stmt = `INSERT INTO bien_ban_hop (
		tenant_id, id, ten_cuoc_hop, ngay_hop, so_hieu, dia_diem, chu_tri_ma,
		thanh_phan, noi_dung, nguoi_tao_ma)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)`

	thanhPhan, err := jsonDanhSach(b.ThanhPhan)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, stmt,
		string(tx.TenantID()), b.ID, b.TenCuocHop, b.NgayHop,
		rongThanhNull(b.SoHieu), rongThanhNull(b.DiaDiem), rongThanhNull(b.ChuTriMa),
		thanhPhan, rongThanhNull(b.NoiDung), b.NguoiTaoMa)
	if err != nil {
		// NOT the title and NOT the body. An INSERT error can quote the whole row on some drivers,
		// and this register's columns hold a commune's minutes (rule 3, forbidden #1).
		return fmt.Errorf("bien_ban_hop: ghi biên bản: %w", err)
	}
	return nil
}

// TaoKetLuan appends ONE numbered conclusion INSIDE the caller's transaction.
//
// THE ORDINAL ARRIVES ALREADY MINTED, from domain.ThuTuKetLuanTiepTheo over ThuTuLonNhat, read under
// the meeting's lock. It is not computed here: a store that minted it would be a second place the
// numbering rule lives, and `UNIQUE (tenant_id, bien_ban_id, thu_tu)` is what actually guarantees it
// either way.
func (s *BienBanHopStore) TaoKetLuan(ctx context.Context, tx *store.ScopedTx,
	k domain.KetLuanHop) error {

	const stmt = `INSERT INTO ket_luan_hop (tenant_id, id, bien_ban_id, thu_tu, noi_dung)
		VALUES ($1,$2,$3,$4,$5)`

	_, err := tx.Exec(ctx, stmt,
		string(tx.TenantID()), k.ID, k.BienBanID, k.ThuTu, k.NoiDung)
	if err != nil {
		// NOT the content: a conclusion routinely quotes a case (migration 0007).
		return fmt.Errorf("ket_luan_hop: ghi kết luận: %w", err)
	}
	return nil
}

// --- shared --------------------------------------------------------------------------------------

// jsonDanhSach renders a list of strings for a JSONB column.
//
// # IT PASSES A STRING AND THE STATEMENT CASTS IT WITH `::jsonb`, WHICH IS NOT WHAT `su_kien_di` AND
// `audit_log` DO
//
// Those two hand a `[]byte` to their JSONB column. A `[]byte` parameter is the one shape a driver
// may legitimately read as `bytea` — lib/pq encodes it as a hex literal — and a hex literal is not
// valid JSON. Which way round it lands cannot be verified from this build environment: there is no
// PostgreSQL reachable here (VIGOV_TEST_DSN unset). So this file takes the shape that is unambiguous
// to every driver rather than the shape that happens to be next door, and the difference is REPORTED
// rather than silently propagated into a third place.
//
// A nil list becomes `[]`, never `null`: the column is `NOT NULL DEFAULT '[]'` and a reader must not
// have to tell "nobody recorded the attendees" from "column not set".
func jsonDanhSach(ds []string) (string, error) {
	if ds == nil {
		ds = []string{}
	}
	b, err := json.Marshal(ds)
	if err != nil {
		// The values are strings a clerk typed; json.Marshal cannot fail on them. Returning the error
		// rather than ignoring it keeps the impossible case loud instead of writing `null`.
		return "", fmt.Errorf("bien_ban_hop: mã hoá thành phần tham dự: %w", err)
	}
	return string(b), nil
}

// quetKetLuan reads one row of cotKetLuan.
//
// POSITIONAL, IN LOCKSTEP WITH cotKetLuan — database/sql binds by POSITION, so a destination
// inserted or removed anywhere but the tail silently shifts every column after it.
//
// THE TWO COUNTERS ARE LEFT AT ZERO and that is honest: this read does not ask the task register
// anything. A caller that needs `x/y` uses the list read, which aggregates them in SQL.
func quetKetLuan(r quangKiem) (domain.KetLuanHop, error) {
	var k domain.KetLuanHop
	if err := r.Scan(&k.ID, &k.BienBanID, &k.ThuTu, &k.NoiDung, &k.TaoLuc); err != nil {
		return domain.KetLuanHop{}, fmt.Errorf("ket_luan_hop: đọc dòng: %w", err)
	}
	return k, nil
}
