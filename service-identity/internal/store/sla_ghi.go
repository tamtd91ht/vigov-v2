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

// The WRITE paths of the commune's processing-deadline table (migration 0008, ADR 0029).
//
// =================================================================================================
// WHERE A FIELD CODE ENTERS, AND WHO CHECKED IT.
//
// Chen writes either a row of domain.BoGieoSLA() (a fixed list in source) or a field's own row added
// through POST /api/v1/sla (ADR 0079 lô 2 Q4). THE SECOND IS CHECKED BEFORE IT GETS HERE, by the use
// case, against the list's owner — app.SLA.AddFieldRule. This file does not, and must not, decide
// whether a code is real: it only knows SQL (rule 5 of the service pattern). CapNhatGio still names
// no `linh_vuc`, so an existing row cannot be re-pointed at another field.
//
// =================================================================================================
// FOUR THINGS HOLD ACROSS BOTH METHODS, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context
//     (rule 1, invariants 4 and 5). It is a parameter of neither method, so no caller can name
//     another commune's row. It matters more here than on most tables: a deadline row read or
//     written across the boundary would make one commune's promise to its citizens be computed
//     from another commune's policy.
//  2. NOTHING HERE OPENS A TRANSACTION. Both take the *store.ScopedTx the use case opened and
//     writes its audit entry in (rule 6, invariant 3). There is no signature that would let the
//     business write and its trail land in two transactions.
//  3. NEITHER STATEMENT NAMES `loai_viec` OR `linh_vuc` IN A SET CLAUSE. What a row applies to is
//     fixed when the row is created; changing it would silently re-point a commitment at a
//     different field, and the unique key would then be wrong with nothing on screen to say so.
//  4. THERE IS NO HARD DELETE. SoftDelete writes the three columns rule 7 names, and only on a
//     field's own row — the default row is refused in SQL as well as in the use case.

// ErrDongSLAKhongTonTai is an id that matches no live row of this commune.
//
// IT IS THE SAME ANSWER FOR AN INVENTED ID, A SOFT-DELETED ROW AND ANOTHER COMMUNE'S ROW, and that
// is not laziness: every statement here is scoped, so all three genuinely produce no match. Telling
// them apart would let a caller learn which ids exist elsewhere (rule 4, forbidden #2, applied to
// configuration).
var ErrDongSLAKhongTonTai = errors.New("sla: không tìm thấy dòng thời hạn xử lý")

// ErrDongSLATrungKhoa is `UNIQUE (tenant_id, loai_viec, linh_vuc_khoa)` refusing a second row for a
// pair this commune already has.
//
// THE SEEDING PATH TREATS IT AS A LOST RACE, NOT AS A BUG. Two administrators pressing the seed
// button at the same moment both read an empty table and both insert; one transaction wins and the
// other is refused by the constraint, which is exactly the outcome that must happen — no duplicate
// row, and no half-seeded table, because the loser rolls back whole.
var ErrDongSLATrungKhoa = errors.New("sla: xã đã có dòng thời hạn cho loại việc và lĩnh vực này")

// DanhSachDeGhi reads this commune's whole live deadline table INSIDE the transaction.
//
// IT EXISTS SO THE SEEDING DECISION IS NOT MADE ON A STALE READ. Seeding is read-decide-write —
// "which of the fifteen rows does this commune not have" — and answering that question outside the
// transaction leaves a gap through which a concurrent run inserts the same rows. DanhSach (sla.go)
// cannot serve it: it reads through *store.Scoped, which is a different connection from the
// transaction, so it would see the table as it was before this transaction started.
//
// NO `FOR UPDATE`, AND THAT IS NOT AN OVERSIGHT. There is nothing to lock: the rows this decision
// is about are the ones that DO NOT EXIST, and PostgreSQL cannot lock an absent row. What actually
// serialises two concurrent seeding runs is `UNIQUE (tenant_id, loai_viec, linh_vuc_khoa)` — the
// loser's INSERT is refused and its whole transaction rolls back, leaving no duplicate and no
// half-seeded table. A `FOR UPDATE` here would lock the rows already present, which are precisely
// the ones nothing is going to touch, and would read as a protection that is not the one holding.
//
// NO CEILING CHECK, UNLIKE DanhSach. TranSLA guards a list on its way to a screen, where a
// truncated read silently answers with the default deadline instead of a field's own
// (DongTheoLinhVuc falls back). Here the result is only ever asked "does this pair exist", and a
// truncated answer would cause a duplicate INSERT that the unique key then refuses — loud, not
// silent. Ordering is not needed either, for the same reason: the caller builds a set.
func (s *SLAStore) DanhSachDeGhi(ctx context.Context, tx *store.ScopedTx) ([]domain.DongSLA, error) {
	const stmt = `SELECT ` + cotSLA + ` FROM sla ` +
		`WHERE tenant_id = $1 AND deleted_at IS NULL`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()))
	if err != nil {
		return nil, fmt.Errorf("sla: đọc bảng thời hạn để ghi: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.DongSLA, 0, 16)
	for rows.Next() {
		d, err := quetMotDongSLA(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("sla: đọc dòng để ghi: %w", err)
		}
		ra = append(ra, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sla: duyệt kết quả để ghi: %w", err)
	}
	return ra, nil
}

// TheoIDDeGhi reads one live deadline row inside the transaction and holds it until the transaction
// ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD. The edit is a read-decide-write: read the row, apply
// the partial change to it, validate the RESULT, write it back. Without the lock two administrators
// editing the same row both read the old state and the second write silently overwrites the first —
// and on this table the value overwritten is a commitment a public authority makes to its citizens.
//
// IT RETURNS domain.DongSLA AND USES cotSLA — THE SAME COLUMN LIST AND THE SAME SCAN ORDER AS THE
// READ PATH, on purpose. cotSLA carries the warning about the five adjacent integer columns whose
// silent transposition changes the promise without producing an error; a second list here would be
// a second place for that transposition to happen.
func (s *SLAStore) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.DongSLA, error) {
	if id == "" {
		// Fail closed rather than compare against the empty string, which would match whatever row
		// happens to carry it. Named rather than turned into "not found" by accident.
		return domain.DongSLA{}, ErrDongSLAKhongTonTai
	}

	const stmt = `SELECT ` + cotSLA + ` FROM sla ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	d, err := quetMotDongSLA(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DongSLA{}, ErrDongSLAKhongTonTai
	}
	if err != nil {
		return domain.DongSLA{}, fmt.Errorf("sla: đọc dòng để ghi: %w", err)
	}
	return d, nil
}

// quetMotDongSLA is DanhSach's Scan for a QueryRow — same list, same order, one place.
//
// THE NULL FOLD IS THE SAME ONE: `linh_vuc IS NULL` is the default row and becomes "" in Go. The
// schema refuses the empty string outright (sla_linh_vuc_khong_rong), so "" here can only have come
// from a NULL.
func quetMotDongSLA(quet func(...any) error) (domain.DongSLA, error) {
	var d domain.DongSLA
	var loaiViec string
	var linhVuc sql.NullString
	// NULL = "do not report"; folded to 0, which the schema's CHECK makes unambiguous (migration 0016).
	var unassigned sql.NullInt64
	err := quet(&d.ID, &loaiViec, &linhVuc,
		&d.GioTiepNhan, &d.GioXuLyXong, &d.GioSapDenHan,
		&d.GioBaoLanhDao, &d.GioBaoChuTich, &unassigned)
	if err != nil {
		return domain.DongSLA{}, err
	}
	d.UnassignedHoldHours = int(unassigned.Int64)
	d.LoaiViec = domain.LoaiViec(loaiViec)
	if !d.LoaiViec.HopLe() {
		// Same refusal as the list read, for the same reason: a row whose kind of work the software
		// does not know is a number nobody can say what it promises. The value is NOT put in the
		// error — it came from the database and an error travels into logs (rule 3, forbidden #3).
		return domain.DongSLA{}, fmt.Errorf("sla: dòng %s: %w", d.ID, ErrLoaiViecLa)
	}
	d.LinhVuc = linhVuc.String
	return d, nil
}

// CapNhatGio writes the five hour figures of one row.
//
// FIVE COLUMNS AND NOTHING ELSE. `loai_viec`, `linh_vuc`, `id` and every soft-delete column are
// absent from the SET clause, so this statement cannot re-point a row at another field, resurrect a
// deleted one, or move it to another commune whatever it is passed.
//
// IT DOES NOT TOUCH ANY DEADLINE ALREADY ISSUED, AND THAT IS THE WHOLE POINT OF THE ROUTE ABOVE IT.
// `han_tiep_nhan` and `han_xu_ly_xong` are columns on the PETITION and the DOCUMENT, in other
// services' schemas, fixed once at the act that set them and stored (rule 10, invariant 2; ADR 0028;
// 14-cau-hinh.md:289 — *"Thay đổi chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu"*). This
// service cannot reach those columns even if somebody wanted to, and nothing anywhere recomputes a
// deadline from this table. That is not an accident of layering to be tidied away: a recomputation
// would silently move a commitment already made to a named citizen.
//
// `cap_nhat_luc = now()` IS IN THE SAME STATEMENT. A second UPDATE for the timestamp would be a
// second statement that can fail on its own, leaving a row whose numbers changed and whose "last
// edited" says otherwise.
func (s *SLAStore) CapNhatGio(ctx context.Context, tx *store.ScopedTx, d domain.DongSLA) error {
	if d.ID == "" {
		return ErrDongSLAKhongTonTai
	}

	const stmt = `UPDATE sla SET
			gio_tiep_nhan = $3, gio_xu_ly_xong = $4, gio_sap_den_han = $5,
			gio_bao_lanh_dao = $6, gio_bao_chu_tich = $7, unassigned_hold_hours = $8,
			cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), d.ID,
		d.GioTiepNhan, d.GioXuLyXong, d.GioSapDenHan, d.GioBaoLanhDao, d.GioBaoChuTich,
		optionalHoursArg(d.UnassignedHoldHours))
	if err != nil {
		return fmt.Errorf("sla: cập nhật số giờ: %w", dichLoiGhiSLA(err))
	}
	return doiMotDongSLA(kq, "cập nhật số giờ")
}

// Chen writes one row: a seed row, or a field's own row whose code the use case has already checked
// with the list's owner (app.SLA.AddFieldRule).
//
// THE ID IS MINTED BY THE CALLER AND PASSED IN, not generated here: the use case pins it in tests,
// and a store that minted its own ids would make every assertion about which row was written an
// assertion about randomness.
//
// `linh_vuc` GOES IN AS NULL FOR THE DEFAULT ROW — the fold DanhSach performs on the way out,
// performed here on the way in, and in the same one place. The schema refuses the empty string
// (sla_linh_vuc_khong_rong), so passing "" through unfolded would not create a second default row;
// it would fail the whole seeding transaction, which is a worse way to find out.
//
// NO `ON CONFLICT`. It would be the obvious way to make seeding idempotent and it is deliberately
// not used: `ON CONFLICT DO NOTHING` makes "this row already existed" indistinguishable from "this
// row was written" at the call site, and the seeding use case has to report the difference — and
// must audit only when something actually changed. The unique key still does its job; a collision
// arrives as ErrDongSLATrungKhoa and the caller decides.
func (s *SLAStore) Chen(ctx context.Context, tx *store.ScopedTx, d domain.DongSLA) error {
	const stmt = `INSERT INTO sla
			(tenant_id, id, loai_viec, linh_vuc,
			 gio_tiep_nhan, gio_xu_ly_xong, gio_sap_den_han,
			 gio_bao_lanh_dao, gio_bao_chu_tich, unassigned_hold_hours)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	var linhVuc any
	if d.LinhVuc != "" {
		linhVuc = d.LinhVuc
	}

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), d.ID, string(d.LoaiViec), linhVuc,
		d.GioTiepNhan, d.GioXuLyXong, d.GioSapDenHan, d.GioBaoLanhDao, d.GioBaoChuTich,
		optionalHoursArg(d.UnassignedHoldHours))
	if err != nil {
		return fmt.Errorf("sla: chèn dòng thời hạn: %w", dichLoiGhiSLA(err))
	}
	return nil
}

// SoftDelete removes one field's own row: the three columns rule 7, invariant 1 names, in one
// statement. `by` is the remover's STAFF CODE (rule 6, invariant 8), never the internal id.
//
// `linh_vuc IS NOT NULL` IS IN THE PREDICATE: the default row is refused here too, not only by the use
// case — removing it leaves every field of that kind with nothing to fall back on. A default row
// matched by id therefore affects zero rows and reads as not found.
//
// `deleted_at IS NULL` makes a second delete a 404 rather than a rewrite of who removed the row and
// why (rule 7, forbidden #5). Setting `deleted_at` turns the generated `linh_vuc_khoa` NULL (0008:213),
// which is what lets the commune add a row for the same field again later.
func (s *SLAStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	if id == "" {
		return ErrDongSLAKhongTonTai
	}
	const stmt = `UPDATE sla SET deleted_at = now(), deleted_by = $3, delete_reason = $4,
			cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND linh_vuc IS NOT NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("sla: xoá mềm dòng thời hạn riêng: %w", err)
	}
	return doiMotDongSLA(kq, "xoá mềm")
}

// optionalHoursArg writes the optional threshold: 0 in Go is NULL in the column ("do not report"),
// never a stored 0 — the schema refuses one (sla_unassigned_hold_hours_positive, migration 0016).
func optionalHoursArg(hours int) any {
	if hours == 0 {
		return nil
	}
	return hours
}

// doiMotDongSLA turns "the UPDATE matched nothing" into ErrDongSLAKhongTonTai.
//
// IT CANNOT NORMALLY HAPPEN — the caller has already read the row FOR UPDATE inside the same
// transaction, so it is there and held. Checked anyway, because the alternative is a write path
// reporting success having changed nothing, and the one thing that produces it is the predicate
// here drifting away from the predicate in TheoIDDeGhi.
func doiMotDongSLA(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("sla: %s: đếm dòng đã ghi: %w", viec, err)
	}
	if n == 0 {
		return ErrDongSLAKhongTonTai
	}
	return nil
}

// dichLoiGhiSLA turns a constraint violation into a sentinel the use case can decide on.
//
// IT MATCHES ON THE CONSTRAINT NAME AS A SUBSTRING, following dichLoiGhiCanBo. `sla` is PARTITION
// BY HASH, so the violation PostgreSQL reports names the PARTITION's constraint
// (`sla_p07_tenant_id_loai_viec_linh_vuc_khoa_key`) rather than the parent's — a match on the exact
// parent name would therefore never fire, on any real database, while passing every test that does
// not have one.
//
// THE UNRECOGNISED ERROR IS RETURNED UNCHANGED, never swallowed into a sentinel that reads as a
// business refusal: a connection failure reported as "this commune already has that row" sends an
// administrator to look for a row that is not there.
func dichLoiGhiSLA(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "linh_vuc_khoa_key") {
		return ErrDongSLATrungKhoa
	}
	return err
}
