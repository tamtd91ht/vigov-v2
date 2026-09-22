package store

// The WRITE paths of the commune's staff register.
//
// FIVE THINGS HOLD ACROSS EVERY METHOD HERE, and each one is a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, taken from tx.TenantID() which took it from the
//     context (rule 1, invariants 4 and 5). It is never a parameter of any method here, so no
//     caller can name another commune's row even by mistake.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3). Every mutating method takes the *store.ScopedTx, so there is no
//     signature that would let the business write and its trail land in two transactions.
//  3. `ma`, `co_tai_khoan` AND `mat_khau_hash` APPEAR IN NO UPDATE. An issued staff code is never
//     renumbered (rule 7, invariant 3) and the archival records that name a person hold it as a
//     value; the other two belong to the account flow, which is a different set of questions
//     (#9/#17/#18) and must not be reachable from the directory screen.
//  4. EVERY READ EXCLUDES SOFT-DELETED ROWS. A person kept for the audit trail is not a person a
//     screen may edit, lock or reassign.
//  5. THE THREE READS THE GUARDS DEPEND ON ALL RUN INSIDE THE TRANSACTION, and two of them take
//     `FOR UPDATE`. Read-decide-write with the read outside the transaction is a check with a gap
//     in the middle — see QuanTriDangHoatDong, where the gap is what open question #13 is about.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The refusals this layer translates out of PostgreSQL. Sentinels, so the use case decides the
// policy and the handler decides the status code — neither of them by matching on driver text.
var (
	// ErrMaCanBoDaDung is `UNIQUE (tenant_id, ma)` refusing a code THAT MAY WELL BE INVISIBLE:
	// the key is deliberately not partial, so a soft-deleted person keeps their code for good
	// (rule 7, invariant 3; migration 0009 §4). The caller's answer is to mint another code and
	// try again — never to look for a free one, which is two statements with a gap in between.
	ErrMaCanBoDaDung = errors.New("can_bo: mã cán bộ đã được dùng trong xã")

	// ErrEmailDaDung is `UNIQUE (tenant_id, email)`. UNLIKE the code, this one is about a value a
	// person typed, so it reaches the screen as a sentence rather than a retry.
	ErrEmailDaDung = errors.New("can_bo: thư điện tử đã được dùng trong xã")

	// ErrBoPhanKhongTonTai / ErrVaiTroKhongTonTai are the two composite foreign keys of
	// `nguoi_dung` (migration 0001:186-187). They fire when a client names a unit or a role that
	// does not exist IN THIS COMMUNE — which is also what happens when it names another commune's
	// id, because the key carries `tenant_id` (rule 1, invariant 6).
	ErrBoPhanKhongTonTai = errors.New("can_bo: bộ phận không tồn tại trong xã")
	ErrVaiTroKhongTonTai = errors.New("can_bo: vai trò không tồn tại trong xã")
)

// QuyenQuanTriNguoiDung is the permission key open question #13 is counted on.
//
// IT IS A CONSTANT AND NOT A PARAMETER, and the SQL below is built from this very identifier
// rather than binding it: there is exactly ONE key whose disappearance locks a commune out of its
// own system, and a bound parameter is a value some caller could get wrong — quietly counting the
// holders of a different permission and reporting "there is another administrator" when there is
// not.
//
// THE KEY EXISTS IN THE `quyen` TABLE (migration 0001:278, "Quản lý người dùng"). Rule 5,
// invariant 3c: a key no migration seeds is a key no administrator can grant.
const QuyenQuanTriNguoiDung = "admin.user"

// TheoIDDeGhi reads one live staff row inside the transaction and holds it until the transaction
// ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Every write above it is a
// read-decide-write: read the row, decide whether the rules permit the change, apply it. Without
// the lock two administrators editing the same person both read the old state, both decide
// against it, and the second write silently overwrites the first — including the case where one
// was locking the account and the other was moving it to a different role.
//
// IT EXCLUDES SOFT-DELETED ROWS, so "edit somebody a colleague deleted a second ago" is a 404
// rather than a resurrection.
//
// IT RETURNS domain.CanBoTomTat — THE SAME TYPE AND THE SAME COLUMN LIST AS THE READ PATH, on
// purpose. A second column list would be a second positional Scan to keep in lockstep, and
// cotTomTat already carries the warning about the two pairs of adjacent same-typed columns. It
// also means the row a guard decides on is exactly the row the screen was showing.
func (s *CanBoStore) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBoTomTat, error) {
	if id == "" {
		// Fail closed rather than run a comparison against the empty string, which would match
		// whatever row happens to carry it. Not an error a caller can act on, but it is a defect,
		// so it is named instead of being turned into "not found" by accident.
		return domain.CanBoTomTat{}, ErrCanBoKhongTonTai
	}

	const stmt = `SELECT ` + cotTomTat + ` FROM nguoi_dung ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	cb, err := quetMotDong(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CanBoTomTat{}, ErrCanBoKhongTonTai
	}
	if err != nil {
		return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc dòng để ghi: %w", err)
	}
	return cb, nil
}

// quetMotDong is quetTomTat's shape for a QueryRow — same list, same order, one place.
func quetMotDong(quet func(...any) error) (domain.CanBoTomTat, error) {
	var cb domain.CanBoTomTat
	err := quet(&cb.ID, &cb.Ma, &cb.HoTen, &cb.Email, &cb.ChucVu,
		&cb.BoPhanID, &cb.VaiTroID, &cb.DienThoaiCoQuan, &cb.DiDongCaNhan,
		&cb.CoTaiKhoan, &cb.DangHoatDong,
		&cb.DangNhapGanNhat, &cb.TaoLuc)
	return cb, err
}

// chenCanBo — `co_tai_khoan`, `mat_khau_hash`, `dang_hoat_dong` AND `vai_tro_id` ARE LITERALS AND
// ARE NOT PARAMETERS. Read that as the security property it is, not as a shortcut:
//
//	co_tai_khoan = false   THIS ROUTE CREATES A DIRECTORY ENTRY, NEVER AN ACCOUNT. How a member of
//	mat_khau_hash = ''     staff gets their first credential is open questions #9/#17/#18 and a
//	                       separate flow (kb/90-ephemeral/tien-do, `cap-tai-khoan-can-bo`). With no
//	                       parameter for either, no handler can be talked into minting one, and the
//	                       CHECK constraint `nguoi_dung_co_tai_khoan_co_mat_khau` (migration 0009
//	                       §3) is satisfied by construction rather than by remembering.
//	dang_hoat_dong         left to its DEFAULT true. It says nothing on a row with no account.
//	vai_tro_id = NULL      A NEW PERSON HOLDS NOTHING. Assigning a role is its own route, because
//	                       it is the route carrying the two guards of open question #14 — and a
//	                       role parameter here would be the way around both of them.
//
// `nullif($n,”)` ON THE TWO NULLABLE FOREIGN KEYS: the Go side has one spelling of "none" (the
// empty string) and the schema has another (NULL). Writing ” into `bo_phan_id` would not be
// "no department", it would be a reference to a department whose id is the empty string — which
// the foreign key refuses, with a message about a constraint rather than about a choice.
const chenCanBo = `INSERT INTO nguoi_dung
	(tenant_id, id, ma, ho_ten, email, chuc_vu, bo_phan_id,
	 dien_thoai_co_quan, di_dong_ca_nhan,
	 vai_tro_id, co_tai_khoan, mat_khau_hash)
	VALUES ($1, $2, $3, $4, $5, $6, nullif($7,''), $8, $9, NULL, false, '')`

// Chen adds one directory entry. The caller has already minted the code with domain.SinhMaCanBo
// and will mint another if this comes back ErrMaCanBoDaDung.
func (s *CanBoStore) Chen(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error {
	_, err := tx.Exec(ctx, chenCanBo, string(tx.TenantID()),
		cb.ID, cb.Ma, cb.HoTen, cb.Email, cb.ChucVu, cb.BoPhanID,
		cb.DienThoaiCoQuan, cb.DiDongCaNhan)
	if err != nil {
		return dichLoiGhiCanBo("chèn", err)
	}
	return nil
}

// capNhatHoSoCanBo — WHAT IS ABSENT IS THE CONTRACT. `ma` (never renumbered), `co_tai_khoan` and
// `mat_khau_hash` (the account flow), `vai_tro_id` (its own route, its own guards) and
// `dang_hoat_dong` (its own route, its own guard) appear nowhere in this statement, so the profile
// screen cannot reach any of them however its request body is shaped.
const capNhatHoSoCanBo = `UPDATE nguoi_dung
	SET ho_ten = $3, email = $4, chuc_vu = $5, bo_phan_id = nullif($6,''),
	    dien_thoai_co_quan = $7, di_dong_ca_nhan = $8, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhatHoSo writes the six fields the Sửa thông tin cán bộ form owns.
func (s *CanBoStore) CapNhatHoSo(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error {
	kq, err := tx.Exec(ctx, capNhatHoSoCanBo, string(tx.TenantID()),
		cb.ID, cb.HoTen, cb.Email, cb.ChucVu, cb.BoPhanID,
		cb.DienThoaiCoQuan, cb.DiDongCaNhan)
	if err != nil {
		return dichLoiGhiCanBo("cập nhật hồ sơ", err)
	}
	return doiMotDong(kq, "cập nhật hồ sơ")
}

// DatKhoa opens or shuts one account. `dang_hoat_dong` and NOTHING else.
//
// THE COLUMN IS THE LOCK, AND OPEN QUESTION #10 IS WHY THIS IS NOT A DELETE: a member of staff who
// retires or transfers is LOCKED and stays in the directory, because their name is what makes
// years of administrative records readable. Deleting them is a different act, for a different
// situation (a duplicated row), with a different permission.
func (s *CanBoStore) DatKhoa(ctx context.Context, tx *store.ScopedTx, id string, dangHoatDong bool) error {
	const stmt = `UPDATE nguoi_dung SET dang_hoat_dong = $3, cap_nhat_luc = now()
	              WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, dangHoatDong)
	if err != nil {
		return dichLoiGhiCanBo("đặt khoá", err)
	}
	return doiMotDong(kq, "đặt khoá")
}

// DatVaiTro moves one person to a role, or to no role at all (”).
func (s *CanBoStore) DatVaiTro(ctx context.Context, tx *store.ScopedTx, id, vaiTroID string) error {
	const stmt = `UPDATE nguoi_dung SET vai_tro_id = nullif($3,''), cap_nhat_luc = now()
	              WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, vaiTroID)
	if err != nil {
		return dichLoiGhiCanBo("đặt vai trò", err)
	}
	return doiMotDong(kq, "đặt vai trò")
}

// truyVanQuanTriDeGhi lists — and LOCKS — every account that currently holds `admin.user` in this
// commune.
//
// THIS QUERY IS THE WHOLE OF OPEN QUESTION #13, so it is worth reading slowly.
//
// WHAT IT COUNTS. The six conditions are `dieuKienGiuQuyen`, SHARED WITH checker.go rather than
// spelled out again: the set of people who can actually exercise `admin.user` today is decided by
// that predicate, and a second copy of it here would drift. Drift in either direction is a defect
// with no error attached — looser here and the system believes there is another administrator when
// there is not, so it permits the lock-out; stricter and it refuses an operation that was safe,
// which reads to a commune as the software being broken.
//
// WHY IT SELECTS ROWS INSTEAD OF COUNTING THEM. Two reasons and both are load-bearing:
// PostgreSQL refuses `FOR UPDATE` together with an aggregate, and the caller needs to know whether
// the person being acted on is IN the set — a count cannot answer that.
//
// `FOR UPDATE` IS WHAT CLOSES THE GAP, and the gap is real: two administrators locking each other
// at the same instant both read "there is somebody else", both proceed, and the commune ends with
// nobody. Under READ COMMITTED, the second transaction blocks on the rows the first one holds;
// when the first commits, PostgreSQL RE-EVALUATES this query's conditions against the new version
// of each locked row and drops the ones that no longer satisfy them. So the second transaction
// sees the freshly locked administrator gone, counts one, and refuses. The lock has to cover the
// rows the ANSWER depends on — not just the row being written — which is why the set is locked and
// not merely read.
//
// IT LOCKS ROWS IN `vai_tro_quyen` TOO, and that is deliberate rather than a side effect of the
// join: `FOR UPDATE` with no `OF` locks the row of every table in the FROM list, so the grant that
// makes these people administrators is held as well. That is the FOURTH path #13 names — clearing
// `admin.user` from the last role that carries it, on the Phân quyền screen, without touching a
// single person. The write route for that screen does not exist yet; when it is written it must
// take the same lock, and this comment is where it will be looked for.
//
// STATED LIMIT, because a guard that looks complete and is not is worse than none: a transaction
// that INSERTS a brand-new administrator concurrently is not blocked by these locks — a row that
// does not exist cannot be locked — but it does not need to be. It can only ever make the set
// BIGGER, and the refusal is about the set becoming empty.
//
// ORDERED BY id SO EVERY CALLER TAKES THE LOCKS IN THE SAME SEQUENCE. Two transactions acquiring
// the same rows in opposite orders deadlock; PostgreSQL detects it and aborts one, which is safe
// but reaches a commune as an unexplained failure.
const truyVanQuanTriDeGhi = `
SELECT nd.id
FROM nguoi_dung nd
JOIN vai_tro       vt ON vt.tenant_id = nd.tenant_id AND vt.id         = nd.vai_tro_id
JOIN vai_tro_quyen vq ON vq.tenant_id = nd.tenant_id AND vq.vai_tro_id = vt.id
WHERE nd.tenant_id = $1
  AND vq.quyen_ma = '` + QuyenQuanTriNguoiDung + `'` + dieuKienGiuQuyen + `
ORDER BY nd.id
FOR UPDATE`

// QuanTriDangHoatDong returns the internal ids of everybody who can exercise `admin.user` in this
// commune right now, and holds every row the answer rests on until the transaction ends.
func (s *CanBoStore) QuanTriDangHoatDong(ctx context.Context, tx *store.ScopedTx) ([]string, error) {
	rows, err := tx.Underlying().QueryContext(ctx, truyVanQuanTriDeGhi, string(tx.TenantID()))
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc danh sách quản trị viên: %w", err)
	}
	defer rows.Close()

	var ra []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng quản trị viên: %w", err)
		}
		ra = append(ra, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt danh sách quản trị viên: %w", err)
	}
	return ra, nil
}

// QuyenCuaVaiTro lists the permission keys one role grants, and says whether the role is there at
// all.
//
// THE BOOL IS NOT A CONVENIENCE. "This role grants nothing" and "there is no such role" are two
// different answers and the second one must refuse the write: the foreign key would accept a
// SOFT-DELETED role (rule 7 keeps the row), and a person moved onto one holds no permission while
// every screen shows them as having a role. An empty list cannot be told from that, so the caller
// is handed both facts.
//
// IT IS READ INSIDE THE TRANSACTION because open question #14's second constraint compares it with
// what the actor holds, and a comparison whose two halves come from two moments is a comparison of
// two different worlds.
func (s *CanBoStore) QuyenCuaVaiTro(ctx context.Context, tx *store.ScopedTx, vaiTroID string) ([]string, bool, error) {
	if vaiTroID == "" {
		// "No role" is a legitimate destination and grants nothing. It is not a missing role.
		return nil, true, nil
	}

	const stmtTonTai = `SELECT 1 FROM vai_tro
	                    WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	var mot int
	err := tx.Underlying().QueryRowContext(ctx, stmtTonTai, string(tx.TenantID()), vaiTroID).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("can_bo: kiểm vai trò tồn tại: %w", err)
	}

	const stmt = `SELECT quyen_ma FROM vai_tro_quyen
	              WHERE tenant_id = $1 AND vai_tro_id = $2 ORDER BY quyen_ma`
	ds, err := docCotChuoi(ctx, tx, stmt, vaiTroID)
	if err != nil {
		return nil, false, fmt.Errorf("can_bo: đọc quyền của vai trò: %w", err)
	}
	return ds, true, nil
}

// QuyenDangGiu lists the permission keys one person can exercise right now.
//
// SAME PREDICATE AS checker.go, FOR THE SAME REASON AS QuanTriDangHoatDong: this answers "what may
// the person doing this actually do", and open question #14 forbids handing out more than that. A
// looser predicate here would let an account that is locked, or has no account at all, pass on a
// permission the checker would refuse it.
func (s *CanBoStore) QuyenDangGiu(ctx context.Context, tx *store.ScopedTx, canBoID string) ([]string, error) {
	if canBoID == "" {
		return nil, nil
	}
	ds, err := docCotChuoi(ctx, tx, truyVanQuyen1Nguoi, canBoID)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc quyền đang giữ: %w", err)
	}
	return ds, nil
}

// truyVanQuyen1Nguoi is truyVanDanhSachQuyen, run inside a transaction instead of through Scoped.
// The text is the one const in checker.go; only the handle differs.
const truyVanQuyen1Nguoi = truyVanDanhSachQuyen

func docCotChuoi(ctx context.Context, tx *store.ScopedTx, stmt string, args ...any) ([]string, error) {
	rows, err := tx.Underlying().QueryContext(ctx,
		stmt, append([]any{string(tx.TenantID())}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ra []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		ra = append(ra, v)
	}
	return ra, rows.Err()
}

// doiMotDong turns "the UPDATE matched nothing" into ErrCanBoKhongTonTai.
//
// IT CANNOT NORMALLY HAPPEN — every caller has already read the row FOR UPDATE inside the same
// transaction, so the row is there and is held. It is checked anyway because the alternative is a
// write path that reports success having changed nothing, and the one situation that produces it
// is a predicate here drifting away from the predicate in TheoIDDeGhi.
func doiMotDong(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("can_bo: %s: đếm dòng đã ghi: %w", viec, err)
	}
	if n == 0 {
		return ErrCanBoKhongTonTai
	}
	return nil
}

// dichLoiGhiCanBo turns a constraint violation into a sentinel the use case can decide on.
//
// IT MATCHES ON THE CONSTRAINT NAME AS A SUBSTRING, DELIBERATELY. `nguoi_dung` is PARTITION BY
// HASH with 32 partitions (ADR 0010), so the server names the PARTITION's copy of the key in the
// message — `nguoi_dung_p16_tenant_id_ma_key`, not `nguoi_dung_tenant_id_ma_key`. Matching the
// full parent name would compile, pass every unit test, and never once fire on a real server;
// matching the stable tail fires on both. Pinned against a real PostgreSQL in
// can_bo_ghi_pg_test.go, which is the only place this can be proven.
//
// ANYTHING UNRECOGNISED IS WRAPPED AND PASSED ON, never flattened into one of the sentinels. A
// disk failure reported as "that e-mail is taken" sends a person to change a field that was never
// the problem.
func dichLoiGhiCanBo(viec string, err error) error {
	switch tho := err.Error(); {
	case strings.Contains(tho, "tenant_id_ma_key"):
		return ErrMaCanBoDaDung
	case strings.Contains(tho, "tenant_id_email_key"):
		return ErrEmailDaDung
	case strings.Contains(tho, "bo_phan_id_fkey"):
		return ErrBoPhanKhongTonTai
	case strings.Contains(tho, "vai_tro_id_fkey"):
		return ErrVaiTroKhongTonTai
	default:
		return fmt.Errorf("can_bo: %s: %w", viec, err)
	}
}
