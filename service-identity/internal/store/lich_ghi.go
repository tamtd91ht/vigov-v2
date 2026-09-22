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

// The WRITE paths of the commune's working calendar — the three tables of migration 0006.
//
// =================================================================================================
// WHAT CHANGED SINCE THE THREE READ STORES EACH SAID "NO WRITE PATH", AND WHY THAT IS NOT A STOP
// CONDITION BEING ROUTED AROUND.
//
// lich_lam_viec.go, ngay_nghi_le.go and ngay_lam_bu.go each carry the sentence *"who may edit a
// commune's working calendar has not been asked — it is the sibling of open question #21"*. That
// sentence was about the PERMISSION, and the permission now has an answer that invents nothing:
// `admin.sla` — "Cấu hình thời hạn xử lý" — already exists in the `quyen` table (migration
// 0001:277) and is the key the deadline configuration screen already uses. The calendar is the
// other half of that same screen: 14-cau-hinh.md §8 says the deadline is counted in working hours
// and names these three tables at :318 as what "giờ làm việc" means. One job, one person, one key.
// No new key is created here — rule 5, invariant 3c: a key no migration seeds is a right no
// administrator can grant, so the route would answer 403 to every account forever while its tests
// stayed green.
//
// =================================================================================================
// FIVE THINGS HOLD ACROSS EVERY METHOD IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID(), which took it from the context
//     (rule 1, invariants 4 and 5). It is a parameter of no method here, so no caller can name
//     another commune's row. It matters as much as on any table in this service: a calendar row
//     read or written across the boundary would make one commune's promise to its citizens be
//     computed from another commune's office hours.
//  2. NOTHING HERE OPENS A TRANSACTION. Every method takes the *store.ScopedTx the use case opened
//     and writes its audit entry in (rule 6, invariant 3). There is no signature that would let the
//     business write and its trail land in two transactions.
//  3. THERE IS NO HARD DELETE. Every removal is an UPDATE setting `deleted_at`, `deleted_by` and
//     `delete_reason` (rule 7, invariant 1). A calendar row is the BASIS OF AN ISSUED COMMITMENT:
//     when an inspection asks why a petition received on 30/04 was due on 05/05, the answer is the
//     calendar as it stood that day (migration 0006:41).
//  4. NO UPDATE NAMES A SOFT-DELETE COLUMN except the soft-delete statement itself, so no edit can
//     resurrect a removed row and no edit can silently remove a live one.
//  5. THE `…DeGhi` READS RETURN SOFT-DELETED ROWS TOO, flagged. That is not an oversight and it is
//     explained at DaXoa below: it is what keeps the SEEDING run from colliding with a row the
//     commune deliberately removed.

// ---------------------------------------------------------------------------------------------
// The sentinels. Each one is the same answer for an invented id, a soft-deleted row and another
// commune's row — every statement here is scoped, so all three genuinely produce no match, and
// telling them apart would let a caller learn which ids exist elsewhere (rule 4, forbidden #2,
// applied to configuration).
// ---------------------------------------------------------------------------------------------

var (
	ErrCaLamViecKhongTonTai  = errors.New("lich_lam_viec: không tìm thấy ca làm việc")
	ErrNgayNghiLeKhongTonTai = errors.New("ngay_nghi_le: không tìm thấy ngày nghỉ lễ")
	ErrNgayLamBuKhongTonTai  = errors.New("ngay_lam_bu: không tìm thấy ca làm bù")

	// ErrCaLamViecTrungGioMo is `UNIQUE (tenant_id, thu, bat_dau)` refusing a second session that
	// starts at the same minute on the same weekday.
	//
	// ⚠ IT FIRES FOR A SOFT-DELETED ROW TOO, AND THAT IS THE SCHEMA RATHER THAN A CHOICE MADE HERE.
	// The unique key deliberately carries no `WHERE deleted_at IS NULL` — a partial unique index is
	// what lets an issued code be reissued, which rule 7, invariant 3 forbids and
	// tools/check_khoa_duy_nhat.py blocks at the migration. The consequence on THIS table is real
	// and is stated rather than hidden: a commune that removes the Monday 07:30 session cannot add
	// another session starting at 07:30 on a Monday, ever. What they can do is EDIT the remaining
	// rows, which reaches every configuration they might want. The handler's sentence says so.
	ErrCaLamViecTrungGioMo = errors.New("lich_lam_viec: xã đã có ca bắt đầu đúng giờ này trong thứ này")

	// ErrNgayNghiLeTrungNgay is `UNIQUE (tenant_id, ngay)` — one date, one holiday row. Same
	// soft-delete consequence as above.
	ErrNgayNghiLeTrungNgay = errors.New("ngay_nghi_le: xã đã có dòng cho ngày này")

	// ErrNgayLamBuTrungGioMo is `UNIQUE (tenant_id, ngay, bat_dau)`.
	ErrNgayLamBuTrungGioMo = errors.New("ngay_lam_bu: xã đã có ca làm bù bắt đầu đúng giờ này trong ngày này")
)

// ---------------------------------------------------------------------------------------------
// lich_lam_viec — the ordinary week.
// ---------------------------------------------------------------------------------------------

// CaLamViecDeGhi is one weekly session AS THE WRITE PATH SEES IT: the row, plus whether it is
// soft-deleted.
//
// =================================================================================================
// WHY THE WRITE PATH NEEDS THE DELETED ROWS AND THE READ PATH DOES NOT — the one paragraph in this
// file worth reading twice.
//
// Two different questions are asked of this list and they need two different sets of rows:
//
//	"does this new session OVERLAP anything?"   LIVE rows only. A soft-deleted session is not a
//	                                            session (rule 7, invariant 2), and counting it would
//	                                            refuse a perfectly good save.
//	"does the SEED already have this row?"      LIVE AND DELETED. The unique key counts deleted rows
//	                                            (see ErrCaLamViecTrungGioMo), so a seeding run that
//	                                            treated a removed Monday morning as "missing" would
//	                                            INSERT it, hit the constraint, and roll the whole
//	                                            seeding transaction back — a button that fails for
//	                                            a commune whose only sin was tidying their calendar.
//	                                            Skipping it is also the right BUSINESS answer: the
//	                                            commune removed that row on purpose, and a seed that
//	                                            puts it back is a seed that overwrites a decision.
//
// ONE QUERY ANSWERS BOTH because a second query would be a second instant, and the seeding decision
// is read-decide-write inside one transaction.
// =================================================================================================
type CaLamViecDeGhi struct {
	Ca    domain.CaLamViec
	DaXoa bool
}

// cotLichLamViecDeGhi is cotLichLamViec PLUS the soft-delete flag, in the same order.
//
// THE FLAG IS LAST, so the first five positions are identical to the read path's and a reader
// comparing the two lists compares them line for line. The two adjacent time columns carry the same
// warning cotLichLamViec does: swapping them produces no error, only a week that runs from closing
// time to opening time.
const cotLichLamViecDeGhi = cotLichLamViec + `, (deleted_at IS NOT NULL) AS da_xoa`

// DanhSachDeGhi reads this commune's whole weekly calendar INSIDE the transaction, deleted rows
// included.
//
// NO `FOR UPDATE`, AND THAT IS NOT AN OVERSIGHT — the same argument SLAStore.DanhSachDeGhi makes:
// the rows a seeding decision is about are the ones that DO NOT EXIST, and PostgreSQL cannot lock
// an absent row. What serialises two concurrent seeding runs is `UNIQUE (tenant_id, thu, bat_dau)`:
// the loser's INSERT is refused and its whole transaction rolls back, leaving no duplicate and no
// half-seeded week.
//
// THE CEILING IS THE READ PATH'S, and it counts deleted rows too. A week whose rows — live and
// removed — have passed TranLichLamViec has stopped being a working week, which is exactly what
// that constant means (an import run twice, a fixture on a live database), and refusing the WRITE
// as well as the read is the fail-closed reading.
func (s *LichLamViecStore) DanhSachDeGhi(ctx context.Context, tx *store.ScopedTx) ([]CaLamViecDeGhi, error) {
	const stmt = `SELECT ` + cotLichLamViecDeGhi + ` FROM lich_lam_viec ` +
		`WHERE tenant_id = $1 ORDER BY thu, bat_dau LIMIT $2`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), TranLichLamViec+1)
	if err != nil {
		return nil, fmt.Errorf("lich_lam_viec: đọc lịch để ghi: %w", err)
	}
	defer rows.Close()

	ra := make([]CaLamViecDeGhi, 0, 16)
	for rows.Next() {
		m, err := quetMotCaLamViec(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("lich_lam_viec: đọc dòng để ghi: %w", err)
		}
		ra = append(ra, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lich_lam_viec: duyệt kết quả để ghi: %w", err)
	}
	if len(ra) > TranLichLamViec {
		// Rows already read are DROPPED rather than trimmed, exactly as on the read path: handing
		// back a list the caller might use anyway is how a refusal turns into a silent truncation.
		return nil, ErrQuaNhieuCaLamViec
	}
	return ra, nil
}

// TheoIDDeGhi reads one LIVE session inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD. The edit is a read-decide-write: read the row, apply
// the partial change, validate the RESULT, write it back. Without the lock two administrators
// editing one session both read the old state and the second write silently discards the first.
func (s *LichLamViecStore) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CaLamViec, error) {
	if id == "" {
		// Fail closed rather than compare against the empty string, which would match whatever row
		// happens to carry it.
		return domain.CaLamViec{}, ErrCaLamViecKhongTonTai
	}
	const stmt = `SELECT ` + cotLichLamViecDeGhi + ` FROM lich_lam_viec ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	m, err := quetMotCaLamViec(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CaLamViec{}, ErrCaLamViecKhongTonTai
	}
	if err != nil {
		return domain.CaLamViec{}, fmt.Errorf("lich_lam_viec: đọc ca để ghi: %w", err)
	}
	return m.Ca, nil
}

// quetMotCaLamViec is the one Scan for both write reads — same column list, same order, one place.
func quetMotCaLamViec(quet func(...any) error) (CaLamViecDeGhi, error) {
	var m CaLamViecDeGhi
	var batDau, ketThuc int
	if err := quet(&m.Ca.ID, &m.Ca.Thu, &batDau, &ketThuc, &m.Ca.GhiChu, &m.DaXoa); err != nil {
		return CaLamViecDeGhi{}, err
	}
	m.Ca.BatDau = domain.GioTrongNgay(batDau)
	m.Ca.KetThuc = domain.GioTrongNgay(ketThuc)
	return m, nil
}

// Chen writes one weekly session.
//
// THE ID IS MINTED BY THE CALLER AND PASSED IN, not generated here: the use case pins it in tests,
// and a store that minted its own ids would make every assertion about which row was written an
// assertion about randomness.
//
// NO `ON CONFLICT`. Same reasoning as SLAStore.Chen: `DO NOTHING` makes "this row already existed"
// indistinguishable from "this row was written" at the call site, and the seeding use case has to
// report the difference — and must audit only when something actually changed.
//
// THE TIMES GO IN AS `make_time`, NOT AS A STRING. A `TIME` parameter sent as text is parsed by the
// server against its own DateStyle; three integers cannot be misread, and they come from
// GioTrongNgay, which is already seconds since midnight. 24:00:00 is expressible — `make_time`
// accepts an hour of 24 with zero minutes and seconds, which is the value the schema permits for a
// session closing at midnight.
func (s *LichLamViecStore) Chen(ctx context.Context, tx *store.ScopedTx, c domain.CaLamViec) error {
	const stmt = `INSERT INTO lich_lam_viec (tenant_id, id, thu, bat_dau, ket_thuc, ghi_chu)
		VALUES ($1,$2,$3,make_time($4,$5,$6),make_time($7,$8,$9),$10)`

	gio1, phut1, giay1 := baPhanCuaGio(c.BatDau)
	gio2, phut2, giay2 := baPhanCuaGio(c.KetThuc)
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Thu,
		gio1, phut1, giay1, gio2, phut2, giay2, c.GhiChu)
	if err != nil {
		return fmt.Errorf("lich_lam_viec: chèn ca: %w", dichLoiGhiLich(err))
	}
	return nil
}

// CapNhat writes the four editable columns of one session.
//
// `id`, `tenant_id` AND EVERY SOFT-DELETE COLUMN ARE ABSENT FROM THE SET CLAUSE, so this statement
// cannot move a row to another commune, resurrect a removed one, or change which row it is.
//
// `cap_nhat_luc = now()` IS IN THE SAME STATEMENT. A second UPDATE for the timestamp would be a
// second statement that can fail on its own, leaving a row whose hours changed and whose "last
// edited" says otherwise.
func (s *LichLamViecStore) CapNhat(ctx context.Context, tx *store.ScopedTx, c domain.CaLamViec) error {
	if c.ID == "" {
		return ErrCaLamViecKhongTonTai
	}
	const stmt = `UPDATE lich_lam_viec SET
			thu = $3, bat_dau = make_time($4,$5,$6), ket_thuc = make_time($7,$8,$9),
			ghi_chu = $10, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	gio1, phut1, giay1 := baPhanCuaGio(c.BatDau)
	gio2, phut2, giay2 := baPhanCuaGio(c.KetThuc)
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Thu,
		gio1, phut1, giay1, gio2, phut2, giay2, c.GhiChu)
	if err != nil {
		return fmt.Errorf("lich_lam_viec: cập nhật ca: %w", dichLoiGhiLich(err))
	}
	return doiMotDongLich(kq, ErrCaLamViecKhongTonTai, "cập nhật ca làm việc")
}

// XoaMem soft deletes one session. THERE IS NO HARD DELETE IN THIS FILE (rule 7, forbidden #1).
//
// `boi` IS THE STAFF CODE, never the internal id — the same value rule 6, invariant 8 puts in
// `audit_log.actor_id`, and for the same reason: `deleted_by` is read years later by somebody
// handling an inspection, and `CB-00123` names a person with no lookup still alive.
func (s *LichLamViecStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemLich("lich_lam_viec"), string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("lich_lam_viec: xoá mềm: %w", err)
	}
	return doiMotDongLich(kq, ErrCaLamViecKhongTonTai, "xoá ca làm việc")
}

// ---------------------------------------------------------------------------------------------
// ngay_nghi_le — the closure dates.
// ---------------------------------------------------------------------------------------------

// NgayNghiLeDeGhi is one closure date as the write path sees it. See CaLamViecDeGhi for why the
// deleted rows travel with the live ones.
type NgayNghiLeDeGhi struct {
	Ngay  domain.NgayNghiLe
	DaXoa bool
}

// cotNgayNghiLeDeGhi renders the date as TEXT, exactly as the read path does and for the same
// reason: `ngay` is a DATE — a day in the commune's own calendar, with no instant and no zone — and
// a time.Time would arrive as midnight UTC and move to the PREVIOUS DAY the first time it were
// formatted in local time (domain.NgayNghiLe.Ngay).
const cotNgayNghiLeDeGhi = `id, to_char(ngay, 'YYYY-MM-DD') AS ngay, ten, (deleted_at IS NOT NULL) AS da_xoa`

// TheoNamDeGhi reads one commune's closure dates for one year INSIDE the transaction, deleted rows
// included.
//
// BY YEAR AND NOT WHOLE, for the reason TranNgayNghiLeMotNam states: this table grows with TIME
// rather than with how the commune is organised, so "all of it" is a list with no upper bound.
//
// THE YEAR IS A RANGE, NOT `EXTRACT(YEAR FROM ngay) = $2`: a function on the column cannot use the
// index migration 0006:231 creates, and the range form reads the same rows.
//
// NO CROSS-TABLE JOIN HERE, UNLIKE THE READ PATH. The read refuses a whole year when any date is
// both a holiday and a swap day; the write path checks that conflict FOR THE ONE DATE BEING
// WRITTEN, because refusing every edit in a year that already contains a conflict would leave the
// commune unable to fix the conflict.
func (s *NgayNghiLeStore) TheoNamDeGhi(ctx context.Context, tx *store.ScopedTx, nam int) ([]NgayNghiLeDeGhi, error) {
	const stmt = `SELECT ` + cotNgayNghiLeDeGhi + ` FROM ngay_nghi_le
		WHERE tenant_id = $1 AND ngay >= make_date($2, 1, 1) AND ngay < make_date($2 + 1, 1, 1)
		ORDER BY ngay LIMIT $3`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), nam, TranNgayNghiLeMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("ngay_nghi_le: đọc ngày nghỉ lễ năm %d để ghi: %w", nam, err)
	}
	defer rows.Close()

	ra := make([]NgayNghiLeDeGhi, 0, 32)
	for rows.Next() {
		var m NgayNghiLeDeGhi
		if err := rows.Scan(&m.Ngay.ID, &m.Ngay.Ngay, &m.Ngay.Ten, &m.DaXoa); err != nil {
			return nil, fmt.Errorf("ngay_nghi_le: đọc dòng để ghi: %w", err)
		}
		ra = append(ra, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngay_nghi_le: duyệt kết quả để ghi: %w", err)
	}
	if len(ra) > TranNgayNghiLeMotNam {
		return nil, ErrQuaNhieuNgayNghiLe
	}
	return ra, nil
}

func (s *NgayNghiLeStore) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.NgayNghiLe, error) {
	if id == "" {
		return domain.NgayNghiLe{}, ErrNgayNghiLeKhongTonTai
	}
	const stmt = `SELECT ` + cotNgayNghiLeDeGhi + ` FROM ngay_nghi_le ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	var m NgayNghiLeDeGhi
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).
		Scan(&m.Ngay.ID, &m.Ngay.Ngay, &m.Ngay.Ten, &m.DaXoa)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NgayNghiLe{}, ErrNgayNghiLeKhongTonTai
	}
	if err != nil {
		return domain.NgayNghiLe{}, fmt.Errorf("ngay_nghi_le: đọc ngày để ghi: %w", err)
	}
	return m.Ngay, nil
}

// Chen writes one closure date.
//
// THE DATE GOES IN AS `$3::date` AND NOT AS A PARSED time.Time, for the same reason it comes OUT as
// text: the value is a day, and the moment it becomes an instant it acquires a zone that can move
// it by one. The string has already been checked against `YYYY-MM-DD` by domain.ChuanHoaNgay, so
// PostgreSQL's parse of it is unambiguous whatever DateStyle is set to.
func (s *NgayNghiLeStore) Chen(ctx context.Context, tx *store.ScopedTx, n domain.NgayNghiLe) error {
	const stmt = `INSERT INTO ngay_nghi_le (tenant_id, id, ngay, ten) VALUES ($1,$2,$3::date,$4)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), n.ID, n.Ngay, n.Ten)
	if err != nil {
		return fmt.Errorf("ngay_nghi_le: chèn ngày: %w", dichLoiGhiLich(err))
	}
	return nil
}

func (s *NgayNghiLeStore) CapNhat(ctx context.Context, tx *store.ScopedTx, n domain.NgayNghiLe) error {
	if n.ID == "" {
		return ErrNgayNghiLeKhongTonTai
	}
	const stmt = `UPDATE ngay_nghi_le SET ngay = $3::date, ten = $4, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), n.ID, n.Ngay, n.Ten)
	if err != nil {
		return fmt.Errorf("ngay_nghi_le: cập nhật ngày: %w", dichLoiGhiLich(err))
	}
	return doiMotDongLich(kq, ErrNgayNghiLeKhongTonTai, "cập nhật ngày nghỉ lễ")
}

func (s *NgayNghiLeStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemLich("ngay_nghi_le"), string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("ngay_nghi_le: xoá mềm: %w", err)
	}
	return doiMotDongLich(kq, ErrNgayNghiLeKhongTonTai, "xoá ngày nghỉ lễ")
}

// ---------------------------------------------------------------------------------------------
// ngay_lam_bu — the swap working days.
// ---------------------------------------------------------------------------------------------

// CaLamBuDeGhi is one swap-day session as the write path sees it.
type CaLamBuDeGhi struct {
	Ca    domain.CaLamBu
	DaXoa bool
}

const cotNgayLamBuDeGhi = `id, to_char(ngay, 'YYYY-MM-DD') AS ngay, ` +
	`date_part('epoch', bat_dau)::int AS bat_dau, date_part('epoch', ket_thuc)::int AS ket_thuc, ` +
	`ten, (deleted_at IS NOT NULL) AS da_xoa`

// TheoNamDeGhi reads one commune's swap-day sessions for one year inside the transaction.
func (s *NgayLamBuStore) TheoNamDeGhi(ctx context.Context, tx *store.ScopedTx, nam int) ([]CaLamBuDeGhi, error) {
	const stmt = `SELECT ` + cotNgayLamBuDeGhi + ` FROM ngay_lam_bu
		WHERE tenant_id = $1 AND ngay >= make_date($2, 1, 1) AND ngay < make_date($2 + 1, 1, 1)
		ORDER BY ngay, bat_dau LIMIT $3`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), nam, TranNgayLamBuMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("ngay_lam_bu: đọc ngày làm bù năm %d để ghi: %w", nam, err)
	}
	defer rows.Close()

	ra := make([]CaLamBuDeGhi, 0, 16)
	for rows.Next() {
		m, err := quetMotCaLamBu(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("ngay_lam_bu: đọc dòng để ghi: %w", err)
		}
		ra = append(ra, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngay_lam_bu: duyệt kết quả để ghi: %w", err)
	}
	if len(ra) > TranNgayLamBuMotNam {
		return nil, ErrQuaNhieuNgayLamBu
	}
	return ra, nil
}

func (s *NgayLamBuStore) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CaLamBu, error) {
	if id == "" {
		return domain.CaLamBu{}, ErrNgayLamBuKhongTonTai
	}
	const stmt = `SELECT ` + cotNgayLamBuDeGhi + ` FROM ngay_lam_bu ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	m, err := quetMotCaLamBu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CaLamBu{}, ErrNgayLamBuKhongTonTai
	}
	if err != nil {
		return domain.CaLamBu{}, fmt.Errorf("ngay_lam_bu: đọc ca làm bù để ghi: %w", err)
	}
	return m.Ca, nil
}

// quetMotCaLamBu — POSITIONAL, in lockstep with cotNgayLamBuDeGhi. TWO PAIRS OF ADJACENT SAME-TYPED
// VALUES sit in that list: `bat_dau` / `ket_thuc` (both int) and `ngay` / `ten` (both text).
// Swapping either pair produces no error at all.
func quetMotCaLamBu(quet func(...any) error) (CaLamBuDeGhi, error) {
	var m CaLamBuDeGhi
	var batDau, ketThuc int
	if err := quet(&m.Ca.ID, &m.Ca.Ngay, &batDau, &ketThuc, &m.Ca.Ten, &m.DaXoa); err != nil {
		return CaLamBuDeGhi{}, err
	}
	m.Ca.BatDau = domain.GioTrongNgay(batDau)
	m.Ca.KetThuc = domain.GioTrongNgay(ketThuc)
	return m, nil
}

func (s *NgayLamBuStore) Chen(ctx context.Context, tx *store.ScopedTx, c domain.CaLamBu) error {
	const stmt = `INSERT INTO ngay_lam_bu (tenant_id, id, ngay, bat_dau, ket_thuc, ten)
		VALUES ($1,$2,$3::date,make_time($4,$5,$6),make_time($7,$8,$9),$10)`
	gio1, phut1, giay1 := baPhanCuaGio(c.BatDau)
	gio2, phut2, giay2 := baPhanCuaGio(c.KetThuc)
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Ngay,
		gio1, phut1, giay1, gio2, phut2, giay2, c.Ten)
	if err != nil {
		return fmt.Errorf("ngay_lam_bu: chèn ca làm bù: %w", dichLoiGhiLich(err))
	}
	return nil
}

func (s *NgayLamBuStore) CapNhat(ctx context.Context, tx *store.ScopedTx, c domain.CaLamBu) error {
	if c.ID == "" {
		return ErrNgayLamBuKhongTonTai
	}
	const stmt = `UPDATE ngay_lam_bu SET
			ngay = $3::date, bat_dau = make_time($4,$5,$6), ket_thuc = make_time($7,$8,$9),
			ten = $10, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	gio1, phut1, giay1 := baPhanCuaGio(c.BatDau)
	gio2, phut2, giay2 := baPhanCuaGio(c.KetThuc)
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Ngay,
		gio1, phut1, giay1, gio2, phut2, giay2, c.Ten)
	if err != nil {
		return fmt.Errorf("ngay_lam_bu: cập nhật ca làm bù: %w", dichLoiGhiLich(err))
	}
	return doiMotDongLich(kq, ErrNgayLamBuKhongTonTai, "cập nhật ca làm bù")
}

func (s *NgayLamBuStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemLich("ngay_lam_bu"), string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("ngay_lam_bu: xoá mềm: %w", err)
	}
	return doiMotDongLich(kq, ErrNgayLamBuKhongTonTai, "xoá ca làm bù")
}

// ---------------------------------------------------------------------------------------------
// The pieces all three tables share.
// ---------------------------------------------------------------------------------------------

// baPhanCuaGio splits seconds-since-midnight into the three arguments `make_time` wants.
//
// WHY make_time AND NOT A STRING OR AN INTERVAL: a `TIME` parameter sent as text is parsed by the
// server against its own DateStyle, and a session's opening minute is not a value worth handing to
// a parser whose behaviour depends on a session setting. Three numbers cannot be misread.
// `make_time` also accepts an hour of 24 with zero minutes and seconds, which is the value the
// schema permits for a session closing at midnight (domain.GiayTrongNgay) and which `time.Time`
// cannot hold at all.
//
// THE SECONDS ARE float64 BECAUSE `make_time`'S THIRD ARGUMENT IS `double precision`. Passing an
// int makes PostgreSQL look for make_time(int, int, int), which does not exist, and the statement
// fails at the commune's first save rather than here.
func baPhanCuaGio(g domain.GioTrongNgay) (int, int, float64) {
	return int(g) / 3600, (int(g) / 60) % 60, float64(int(g) % 60)
}

// xoaMemLich builds the one soft-delete statement shape all three tables use.
//
// THE TABLE NAME IS INTERPOLATED AND EVERY VALUE IS A PARAMETER. The name comes from the three
// literals in this file and never from a caller, so there is no path by which anything a client
// sent reaches the statement text.
//
// `deleted_at IS NULL` IN THE PREDICATE makes a second delete answer "not found" rather than
// silently rewriting `delete_reason` — the first removal is the one that happened, and overwriting
// its reason would edit a record of an act (rule 7, forbidden #5).
func xoaMemLich(bang string) string {
	return `UPDATE ` + bang + ` SET deleted_at = now(), deleted_by = $3, delete_reason = $4, ` +
		`cap_nhat_luc = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
}

// doiMotDongLich turns "the UPDATE matched nothing" into the table's own not-found sentinel.
//
// IT CANNOT NORMALLY HAPPEN on an edit — the caller has already read the row FOR UPDATE inside the
// same transaction. It is checked anyway, because the alternative is a write path reporting success
// having changed nothing, and the one thing that produces it is the predicate here drifting away
// from the predicate in TheoIDDeGhi.
func doiMotDongLich(kq sql.Result, khongCo error, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("lịch làm việc: %s: đếm dòng đã ghi: %w", viec, err)
	}
	if n == 0 {
		return khongCo
	}
	return nil
}

// dichLoiGhiLich turns a unique-key violation into a sentinel the use case can decide on.
//
// IT MATCHES ON THE CONSTRAINT NAME AS A SUBSTRING, following dichLoiGhiSLA, AND IT MATCHES TWO
// SPELLINGS OF EACH KEY. All three tables are PARTITION BY HASH, so the violation PostgreSQL
// reports names the PARTITION's index (`lich_lam_viec_p07_tenant_id_thu_bat_dau_key`) rather than
// the parent constraint (`lich_lam_viec_khong_trung_gio_mo`) — a match on the parent name alone
// would therefore never fire on any real database while passing every test that does not have one.
// Both are listed so neither spelling can slip past.
//
// THE UNRECOGNISED ERROR IS RETURNED UNCHANGED, never swallowed into a sentinel that reads as a
// business refusal: a connection failure reported as "this commune already has that row" sends an
// administrator to look for a row that is not there.
func dichLoiGhiLich(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "thu_bat_dau_key"), strings.Contains(s, "lich_lam_viec_khong_trung_gio_mo"):
		return ErrCaLamViecTrungGioMo
	case strings.Contains(s, "ngay_bat_dau_key"), strings.Contains(s, "ngay_lam_bu_khong_trung_gio_mo"):
		return ErrNgayLamBuTrungGioMo
	case strings.Contains(s, "tenant_id_ngay_key"), strings.Contains(s, "ngay_nghi_le_mot_dong_moi_ngay"):
		return ErrNgayNghiLeTrungNgay
	}
	return err
}
