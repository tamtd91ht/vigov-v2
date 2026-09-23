package store

// The WRITE half of the task register — the locking read, the tree reads, the six statements that
// change a task, its progress log, and the extension requests filed against it. SQL, and nothing
// else.
//
// SIX THINGS HOLD ACROSS EVERY METHOD IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID() (rule 1, invariants 4 and 5). It is
//     never a parameter here, so no caller can write into another commune's register.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may. Every method below takes
//     *store.ScopedTx, which is what makes "write the record now, write the trail afterwards if it
//     works" impossible to express.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the locking read, the tree
//     walk, and the WHERE clause of every UPDATE.
//  4. EVERY UPDATE THAT MOVES THE LIFECYCLE CARRIES THE EXPECTED STATUS in its WHERE clause, so two
//     officers acting on one task at the same moment cannot both succeed. The second matches no row
//     and the caller sees a refusal, never a silent overwrite.
//  5. `ma`, `han_ban_dau`, `nguoi_tao_ma`, `nguon_giao` AND `nguon_id` APPEAR IN NO UPDATE HERE.
//     The first two are refused by the `nhiem_vu_bat_bien` trigger underneath; the last three are
//     facts about how the row came into being. Their absence is what keeps that floor unreachable.
//  6. THERE IS NO `DELETE` STATEMENT IN THIS FILE AND THERE MUST NEVER BE ONE. A task is an
//     administrative record carrying the commune's own progress log (rule 7; §11.5 says so in the
//     specification's own words), and the trigger refuses a hard delete anyway.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

var (
	// ErrNhiemVuDaChuyenTrang means the task was not in the status the caller believed it was in.
	//
	// A SEPARATE SENTINEL FROM ErrNhiemVuKhongTonTai, AND THE DIFFERENCE IS WHAT THE OFFICER IS
	// TOLD. "Không tìm thấy nhiệm vụ" (404) sends them looking for a lost record; this one is a 409
	// saying somebody moved it while their screen was open, which is a true statement they can act
	// on by reloading. Folding the two together would make a normal race look like data loss.
	ErrNhiemVuDaChuyenTrang = errors.New("nhiem_vu: nhiệm vụ không còn ở trạng thái vừa đọc")

	// ErrMaNhiemVuDaTonTai — the register number is already taken in this commune, SOFT-DELETED
	// ROWS INCLUDED.
	//
	// Counting deleted rows is not strictness for its own sake: `UNIQUE (tenant_id, ma)` counts them
	// too, deliberately and with the reasoning written into migration 0006. `NV19` is written on
	// paper minutes and quoted in meeting conclusions; two tasks that have ever carried one number
	// cannot be told apart afterwards (rule 7, invariant 3).
	ErrMaNhiemVuDaTonTai = errors.New("nhiem_vu: mã nhiệm vụ đã được dùng trong xã này")

	// ErrDeNghiKhongTonTai — no live extension request with that id, on that task, in this commune.
	ErrDeNghiKhongTonTai = errors.New("de_nghi_lui_han: không có đề nghị lùi hạn")
)

// --- the locking read -----------------------------------------------------------------------------

// TheoMaDeSua reads one live task INSIDE the caller's transaction and holds it until the
// transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Every write below is a
// read-decide-write: read the task, work out whether the lifecycle and the tree rules admit the
// act, refuse or apply. Without the lock two officers acting on one task both read the old status
// and both decide against it — and the pair that costs the most is the completion check of ADR
// 0037 decision 4, where the second write would complete a parent whose last child was finished by
// nobody.
//
// IT TAKES THE ISSUED NUMBER AND NOT THE INTERNAL id, because that is what the URL carries, what
// the Sổ theo dõi prints and what the audit entry's Subject is. One identifier end to end is one
// identifier nobody can mix up.
func (s *NhiemVuStore) TheoMaDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (
	domain.NhiemVu, error) {

	const stmt = `SELECT ` + cotNhiemVu + ` FROM nhiem_vu
		WHERE tenant_id = $1 AND ma = $2 AND deleted_at IS NULL FOR UPDATE`

	n, err := quetNhiemVu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NhiemVu{}, ErrNhiemVuKhongTonTai
	}
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc nhiệm vụ để sửa: %w", err)
	}
	return n, nil
}

// TheoIDDeSua is TheoMaDeSua keyed by the INTERNAL id.
//
// A SECOND METHOD RATHER THAN A FLAG, and it exists for exactly one caller: the parent named on a
// create or a re-parent request arrives as an id, not as a register number. Locking it too is what
// stops the parent being soft-deleted in the window between "this parent is live" and the INSERT
// that hangs a child off it — which would produce the orphan ADR 0037 decision 3 exists to prevent.
func (s *NhiemVuStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (
	domain.NhiemVu, error) {

	const stmt = `SELECT ` + cotNhiemVu + ` FROM nhiem_vu
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	n, err := quetNhiemVu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NhiemVu{}, ErrNhiemVuKhongTonTai
	}
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc nhiệm vụ cha để sửa: %w", err)
	}
	return n, nil
}

// --- the tree (ADR 0037) ---------------------------------------------------------------------------

// ConTrucTiep reads the DIRECT children of one task.
//
// ONE LEVEL, AND THE RECURSION IS THE CALLER'S — which looks like the wrong place for it and is
// not. A recursive CTE would do the walk in one statement, and it would be UNTESTABLE from this
// repository: there is no PostgreSQL reachable here (VIGOV_TEST_DSN is unset), so the rule ADR 0037
// decision 4 is about would be carried entirely by a statement nothing ever executes. Walking in Go
// puts the rule where the fake driver can drive it, and the cost — one round trip per level, inside
// a transaction that already holds the row — is paid on a tree of tens of rows.
//
// IT EXCLUDES SOFT-DELETED CHILDREN (rule 7, invariant 2), which is also what makes decision 3
// expressible: "còn con chưa xoá" is exactly what this returns.
//
// THE ORDER IS STABLE (`ma`), so a refusal listing three register numbers lists the same three in
// the same order every time. A message that reshuffles itself reads as two different problems.
func (s *NhiemVuStore) ConTrucTiep(ctx context.Context, tx *store.ScopedTx, chaID string) (
	[]domain.NhiemVuTomTat, error) {

	const stmt = `SELECT id, ma, trang_thai FROM nhiem_vu
		WHERE tenant_id = $1 AND nhiem_vu_cha_id = $2 AND deleted_at IS NULL
		ORDER BY ma`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), chaID)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu: đọc việc con: %w", err)
	}
	defer rows.Close()

	var ra []domain.NhiemVuTomTat
	for rows.Next() {
		var (
			c  domain.NhiemVuTomTat
			tt string
		)
		if err := rows.Scan(&c.ID, &c.Ma, &tt); err != nil {
			return nil, fmt.Errorf("nhiem_vu: đọc dòng việc con: %w", err)
		}
		c.TrangThai = domain.TrangThaiNhiemVu(tt)
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhiem_vu: duyệt việc con: %w", err)
	}
	return ra, nil
}

// ChaCua reads ONE step UPWARD — the parent id of a task, empty when it is a root.
//
// THIS IS THE STATEMENT THE CYCLE CHECK IS BUILT FROM, and it is why the check can exist at all:
// migration 0008's `CHECK (nhiem_vu_cha_id IS DISTINCT FROM id)` sees one row and cannot see the
// row above it, so `A → B → C → A` is invisible to the schema. Walking up is the only way to find
// the caller in its own ancestry, and it has to happen in the SAME transaction that holds the rows,
// or another officer re-parents something in the window and the walk decides against a tree that no
// longer exists.
//
// A SOFT-DELETED ANCESTOR ENDS THE WALK, and that is correct rather than convenient: decision 3
// refuses to delete a task that still has live children, so a deleted row cannot be in the middle
// of a live chain. Ending there also keeps the walk from being lengthened by rows no screen shows.
func (s *NhiemVuStore) ChaCua(ctx context.Context, tx *store.ScopedTx, id string) (string, error) {
	const stmt = `SELECT nhiem_vu_cha_id FROM nhiem_vu
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var cha sql.NullString
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan(&cha)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("nhiem_vu: đọc nhiệm vụ cha: %w", err)
	}
	return cha.String, nil
}

// --- minting the register number -------------------------------------------------------------------

// SoLonNhatDaCap reads the highest number already issued in this commune's `NV…` series.
//
// # WHY `max()` IS SAFE HERE AND IS NOT SAFE FOR THE DOCUMENT REGISTER
//
// service-documents mints from a COUNTER ROW and says why in its own file: "a number computed from
// rows goes DOWN when a row is removed". That is true of a register rows can leave — and rows never
// leave this one. A task is soft-deleted, never deleted (the `nhiem_vu_bat_bien` trigger refuses a
// hard delete outright), and THIS STATEMENT DELIBERATELY DOES NOT FILTER `deleted_at`: a
// soft-deleted `NV19` still owns the number 19, exactly as `UNIQUE (tenant_id, ma)` says it does.
// So the maximum is a high-water mark that cannot go backwards.
//
// # WHAT IT STILL DOES NOT GIVE, SAID PLAINLY RATHER THAN DISCOVERED
//
// THERE IS NO ROW TO LOCK. Two clerks creating a task in the same instant both read the same
// maximum, both mint the same code, and the second INSERT is refused by `UNIQUE (tenant_id, ma)` —
// which is SAFE (no duplicate number can exist, and the commitment rule 7 makes is kept) but
// reaches that clerk as "please try again" rather than as the next number. Migration 0006 chose
// this trade deliberately ("Minting belongs to the write path, inside the transaction, against
// UNIQUE (tenant_id, ma) — which is what actually guarantees it") because a PostgreSQL sequence is
// per-table and would hand commune B the number after commune A's. A counter row of the shape
// `day_so_van_ban` has would close the gap and needs a migration; it is reported as a finding
// rather than smuggled in here.
//
// THE PATTERN MATCH IS PART OF THE ANSWER. §7.1 lets a clerk type a code of their own, so
// `KH-2026-07` is a perfectly good task number that contributes nothing to the `NV…` series.
// Excluding it in SQL keeps a hand-typed code from being read as a series position.
func (s *NhiemVuStore) SoLonNhatDaCap(ctx context.Context, tx *store.ScopedTx) (int, error) {
	// `substring(ma from 3)` skips the `NV` prefix; the `~` pattern is what guarantees the rest is
	// digits, so the cast cannot fail on a hand-typed code.
	const stmt = `SELECT COALESCE(MAX(substring(ma from 3)::bigint), 0) FROM nhiem_vu
		WHERE tenant_id = $1 AND ma ~ '^NV[0-9]+$'`

	var so int64
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&so); err != nil {
		return 0, fmt.Errorf("nhiem_vu: đọc số lớn nhất của dãy mã: %w", err)
	}
	return int(so), nil
}

// MaDaDung reports whether this commune has already issued that register number.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE — the same split
// LoaiNhiemVuStore.MaDaDung makes, and for the same reason. Two concurrent creates of one code can
// both pass this check and the second will hit `UNIQUE (tenant_id, ma)` and roll the whole
// transaction back: no duplicate row, an unhelpful 500. This turns the ordinary case into a
// sentence somebody can act on.
//
// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. An issued number is never reissued (rule 7,
// invariant 3), so a soft-deleted row still owns its code.
func (s *NhiemVuStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	const stmt = `SELECT count(*) FROM nhiem_vu WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("nhiem_vu: kiểm tra mã nhiệm vụ trùng: %w", err)
	}
	return n > 0, nil
}

// --- the writes ---------------------------------------------------------------------------------------

// Tao inserts one task INSIDE the caller's transaction.
//
// THE CALLER WRITES THE AUDIT ENTRY AND THE FIRST LOG LINE IN THAT SAME TRANSACTION. This method
// deliberately does not: the audit entry needs the actor and the IP, which are facts about the
// REQUEST and not about the record, and a store that invented them would produce a trail naming
// nobody.
//
// # THE TWO DEADLINE COLUMNS ARE WRITTEN FROM ONE VALUE, HERE, AND NEVER AGAIN
//
// `han_xu_ly` and `han_ban_dau` both take `n.HanXuLy` at creation, which is the act that FIXES the
// commitment (rule 10, invariant 2; ADR 0028). From this statement onward `han_ban_dau` is
// IMMUTABLE — migration 0006's trigger refuses every change to it — so granting an extension moves
// `han_xu_ly` alone and §11.3's on-time ratio keeps its denominator.
//
// A ZERO time.Time BECOMES SQL NULL, and `nhiem_vu_hai_han_cung_co_cung_khong` requires the pair to
// arrive and leave together — which they do, because they come from one field.
//
// ⚠ NO ARITHMETIC ANYWHERE NEAR THIS FILE. A deadline in this system is counted in WORKING HOURS
// and has exactly one implementation, in service `identity` (rule 10, forbidden #2; ADR 0007). What
// this column receives is the instant the form carried — see the note on the use case.
func (s *NhiemVuStore) Tao(ctx context.Context, tx *store.ScopedTx, n domain.NhiemVu) error {
	const stmt = `INSERT INTO nhiem_vu (
		tenant_id, id, ma, loai, khoi, tieu_de, mo_ta, trang_thai, muc_uu_tien,
		nguon_giao, nguon_id, bo_phan_id, nguoi_thuc_hien_ma, lanh_dao_giao_viec_ma,
		co_quan_chu_tri_id, chuyen_vien_theo_doi_ma,
		han_xu_ly, han_ban_dau, tien_do, tom_tat_ket_qua, ghi_chu,
		nhiem_vu_cha_id, nguoi_tao_ma)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	_, err := tx.Exec(ctx, stmt,
		string(tx.TenantID()), n.ID, n.Ma, n.Loai, rongThanhNull(n.Khoi), n.TieuDe,
		rongThanhNull(n.MoTa), string(n.TrangThai), rongThanhNull(n.MucUuTien),
		string(n.NguonGiao), rongThanhNull(n.NguonID), rongThanhNull(n.BoPhanID),
		rongThanhNull(n.NguoiThucHienMa), rongThanhNull(n.LanhDaoGiaoViecMa),
		rongThanhNull(n.CoQuanChuTriID), rongThanhNull(n.ChuyenVienTheoDoiMa),
		// ONE VALUE, TWO COLUMNS. See the note above before separating them.
		khongThanhNull(n.HanXuLy), khongThanhNull(n.HanXuLy),
		n.TienDo, rongThanhNull(n.TomTatKetQua), rongThanhNull(n.GhiChu),
		rongThanhNull(n.NhiemVuChaID), n.NguoiTaoMa)
	if err != nil {
		return fmt.Errorf("nhiem_vu: ghi nhiệm vụ: %w", err)
	}
	return nil
}

// SuaNhiemVu is the set of fields PATCH /api/v1/tasks/{ma} may move.
//
// # WHAT IS NOT IN THIS STRUCT IS THE DESIGN, AND EACH ABSENCE IS A DIFFERENT RULE
//
//	ma, han_ban_dau        the trigger refuses them, and rule 7 is why: an issued number is never
//	                       renumbered, and the original commitment is the denominator of §11.3.
//	han_xu_ly              IT MOVES THROUGH THE EXTENSION FLOW AND NOWHERE ELSE. A PATCH able to
//	                       rewrite a deadline would make `de_nghi_lui_han` decorative — the leader's
//	                       approval could be walked around by the person who wanted more time.
//	trang_thai,            the lifecycle is §6's and moves through its own route, which writes a
//	ngay_hoan_thanh        timeline row and checks the tree (ADR 0037 decision 4).
//	lanh_dao_giao_viec_ma  ADR 0038 makes that column the APPROVER of extensions. A PATCH guarded by
//	                       `task.update` that could rewrite it would let anybody holding that key
//	                       name themselves the approver of their own extension requests — the
//	                       escalation the two-layer rule exists to prevent.
//	bo_phan_id,            assignment is `task.assign`'s act (§6, §10), with a route of its own that
//	nguoi_thuc_hien_ma     this pass did not build. Folding it in here would grant assignment to
//	                       every holder of `task.update`.
//	nguon_giao, nguon_id   facts about how the row came into being.
//
// EVERY FIELD IS A POINTER, so "not mentioned" and "set to empty" are distinguishable. Three of
// them have a meaningful zero — a cleared note, a progress of 0, an unticked box — and PUT-shaped
// replacement cannot tell those from "the client did not send this field".
type SuaNhiemVu struct {
	Khoi         *string
	TieuDe       *string
	MoTa         *string
	MucUuTien    *string
	TienDo       *int
	TomTatKetQua *string
	GhiChu       *string

	LanhDaoPheDuyetHoanThanh *bool
	CapTrenCongNhanHoanThanh *bool

	// NhiemVuChaID re-parents the task. An empty string DETACHES it into a root task, which is why
	// this is a pointer to a string rather than a string: "" and "not mentioned" are two different
	// requests.
	//
	// ⚠ THIS IS THE FIELD THE CYCLE CHECK EXISTS FOR. A task created under a parent cannot close a
	// cycle — a brand-new row has no descendants — so `A → B → C → A` can only ever be built by
	// MOVING an existing task under one of its own descendants, i.e. here.
	NhiemVuChaID *string

	// VanBan is §5.4's document block — the three dynamic lists of §7.2, sent WHOLE.
	//
	// # A POINTER TO A SLICE, AND THE DOUBLE INDIRECTION IS THE RULE ITSELF
	//
	// nil          the client did not send the block. It is left exactly as it is.
	// &[]{}        the client sent an EMPTY block. Every line is removed — that is three `✕`
	//              clicks followed by Save, and it must be expressible.
	//
	// A plain slice could not tell those apart, and the one it would collapse into the other is the
	// one that silently keeps lines a member of staff deleted.
	//
	// ⚠ IT IS REPLACE-BY-SET AND NOT AN APPEND. The lines the block does not name are removed, which
	// is how `✕` reaches the server at all: the line simply stops being sent. domain.SoSanhVanBan
	// decides what that means against the rows already stored, and it is the one place the `thu_tu`
	// rule lives.
	//
	// IT IS NOT FOLDED BY Apdung AND IS NOT COUNTED BY CoGiDoi. Both of those work on a
	// domain.NhiemVu, whose scalar columns are one UPDATE; this block is a set of rows and needs the
	// stored lines read under the task's lock before anything about it can be decided. The use case
	// therefore asks domain.SoSanhVanBan separately — see app.GhiNhiemVu.Sua.
	VanBan *[]domain.VanBanNhiemVuVao
}

// CoGiDoi reports whether anything would actually change.
//
// THE CALLER USES IT TO WRITE NOTHING AT ALL when the answer is no — no UPDATE and no audit entry.
// That is what makes `idem.KhongCan` on the PATCH route a property rather than a hope: the same
// request sent twice leaves one row in one state and one entry in the ledger. Remove this and that
// declaration becomes a lie, and the second request files an entry saying nothing changed.
func (s SuaNhiemVu) CoGiDoi(n domain.NhiemVu) bool {
	switch {
	case s.Khoi != nil && *s.Khoi != n.Khoi,
		s.TieuDe != nil && *s.TieuDe != n.TieuDe,
		s.MoTa != nil && *s.MoTa != n.MoTa,
		s.MucUuTien != nil && *s.MucUuTien != n.MucUuTien,
		s.TienDo != nil && *s.TienDo != n.TienDo,
		s.TomTatKetQua != nil && *s.TomTatKetQua != n.TomTatKetQua,
		s.GhiChu != nil && *s.GhiChu != n.GhiChu,
		s.LanhDaoPheDuyetHoanThanh != nil && *s.LanhDaoPheDuyetHoanThanh != n.LanhDaoPheDuyetHoanThanh,
		s.CapTrenCongNhanHoanThanh != nil && *s.CapTrenCongNhanHoanThanh != n.CapTrenCongNhanHoanThanh,
		s.NhiemVuChaID != nil && *s.NhiemVuChaID != n.NhiemVuChaID:
		return true
	}
	return false
}

// Apdung folds the patch onto a task that has been read, so the caller holds the AFTER state
// without reading the row back.
//
// ONE FUNCTION, USED FOR BOTH THE UPDATE AND THE AUDIT DELTA. Two separate foldings would be two
// answers to "what did this act change", and the one that drifts is whichever is edited second —
// which is the one an inspection reads.
func (s SuaNhiemVu) Apdung(n domain.NhiemVu) domain.NhiemVu {
	if s.Khoi != nil {
		n.Khoi = *s.Khoi
	}
	if s.TieuDe != nil {
		n.TieuDe = *s.TieuDe
	}
	if s.MoTa != nil {
		n.MoTa = *s.MoTa
	}
	if s.MucUuTien != nil {
		n.MucUuTien = *s.MucUuTien
	}
	if s.TienDo != nil {
		n.TienDo = *s.TienDo
	}
	if s.TomTatKetQua != nil {
		n.TomTatKetQua = *s.TomTatKetQua
	}
	if s.GhiChu != nil {
		n.GhiChu = *s.GhiChu
	}
	if s.LanhDaoPheDuyetHoanThanh != nil {
		n.LanhDaoPheDuyetHoanThanh = *s.LanhDaoPheDuyetHoanThanh
	}
	if s.CapTrenCongNhanHoanThanh != nil {
		n.CapTrenCongNhanHoanThanh = *s.CapTrenCongNhanHoanThanh
	}
	if s.NhiemVuChaID != nil {
		n.NhiemVuChaID = *s.NhiemVuChaID
	}
	return n
}

// Sua writes the patched task.
//
// EVERY COLUMN IS WRITTEN FROM THE ALREADY-FOLDED VALUE rather than through a `COALESCE($n, col)`
// per field. The caller read the row under the lock in this same transaction and folded the patch
// onto it, so there is one statement, one shape, and no per-column branch in SQL that a later
// reader has to prove is equivalent to the branch in Go.
//
// THE WHERE CLAUSE CARRIES NEITHER A STATUS NOR A VERSION, and that is deliberate: this act does
// not move the lifecycle, so there is nothing it could race with that the row lock does not already
// hold. What it does carry is `deleted_at IS NULL` — editing a task somebody removed a second ago
// is a refusal, not a resurrection.
func (s *NhiemVuStore) Sua(ctx context.Context, tx *store.ScopedTx, id string, n domain.NhiemVu) error {
	const stmt = `UPDATE nhiem_vu
		SET khoi = $3, tieu_de = $4, mo_ta = $5, muc_uu_tien = $6, tien_do = $7,
		    tom_tat_ket_qua = $8, ghi_chu = $9,
		    lanh_dao_phe_duyet_hoan_thanh = $10, cap_tren_cong_nhan_hoan_thanh = $11,
		    nhiem_vu_cha_id = $12, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id,
		rongThanhNull(n.Khoi), n.TieuDe, rongThanhNull(n.MoTa), rongThanhNull(n.MucUuTien),
		n.TienDo, rongThanhNull(n.TomTatKetQua), rongThanhNull(n.GhiChu),
		n.LanhDaoPheDuyetHoanThanh, n.CapTrenCongNhanHoanThanh,
		rongThanhNull(n.NhiemVuChaID))
	if err != nil {
		return fmt.Errorf("nhiem_vu: sửa nhiệm vụ: %w", err)
	}
	return doiMotDongNhiemVu(kq, "sửa nhiệm vụ")
}

// DoiTrangThai moves the task along §6's lifecycle and records the instant it was finished when
// that is the step being taken.
//
// `ngay_hoan_thanh` IS SET HERE AND NOWHERE ELSE, and it is passed as a zero time for every other
// step. The schema ties it to the status with a BICONDITIONAL (`nhiem_vu_hoan_thanh_co_ngay`), so
// the two are written by one statement or the row is refused: a task in `hoan-thanh` with no
// instant is a row §11.3's on-time ratio cannot classify, and it would drop out of the denominator
// silently.
//
// LEAVING `hoan-thanh` IS NOT A PATH THE LIFECYCLE HAS — §6 draws no arrow out of it — so this
// statement never has to clear the instant, and deliberately cannot: there is no expression here
// that writes NULL into it.
//
// THE WHERE CLAUSE CARRIES THE EXPECTED STATUS. Two officers moving one task at the same moment
// cannot both succeed; the second matches no row and is told to reload.
func (s *NhiemVuStore) DoiTrangThai(ctx context.Context, tx *store.ScopedTx, id string,
	tu, sang domain.TrangThaiNhiemVu, ngayHoanThanh time.Time) error {

	const stmt = `UPDATE nhiem_vu
		SET trang_thai = $3, ngay_hoan_thanh = $4, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $5 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id,
		string(sang), khongThanhNull(ngayHoanThanh), string(tu))
	if err != nil {
		return fmt.Errorf("nhiem_vu: đổi trạng thái nhiệm vụ: %w", err)
	}
	return doiMotDongNhiemVu(kq, "đổi trạng thái")
}

// DoiHanXuLy moves the CURRENT commitment, and only it.
//
// # THIS IS THE ONLY STATEMENT IN THE SERVICE THAT MOVES A TASK'S DEADLINE, AND IT NAMES ONE COLUMN
//
// §5.8 promises it on the screen — "Hạn gốc vẫn được giữ lại để báo cáo đúng hạn không bị lùi
// theo" — and this statement is what makes the promise true rather than intended: `han_ban_dau` is
// not in the SET list, so there is no value any caller could pass that would move it. Migration
// 0006's trigger refuses it underneath as well; two walls, because the one that reaches a person
// first is the sentence and the one that cannot be argued with is the trigger.
//
// THE WHERE CLAUSE CARRIES THE DEADLINE THE APPROVER SAW. Between the leader reading the request
// and pressing approve, a second extension could have been approved elsewhere; matching on the old
// value means the loser of that race writes nothing and is told so, rather than silently moving a
// commitment twice.
func (s *NhiemVuStore) DoiHanXuLy(ctx context.Context, tx *store.ScopedTx, id string,
	hanCu, hanMoi time.Time) error {

	const stmt = `UPDATE nhiem_vu
		SET han_xu_ly = $3, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND han_xu_ly = $4 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, hanMoi, hanCu)
	if err != nil {
		return fmt.Errorf("nhiem_vu: lùi hạn xử lý: %w", err)
	}
	return doiMotDongNhiemVu(kq, "lùi hạn xử lý")
}

// XoaMem removes the task from every read path and keeps the record (rule 7, invariant 1).
//
// THE THREE COLUMNS ARE WRITTEN TOGETHER OR NOT AT ALL — `nhiem_vu_xoa_mem_day_du` refuses a
// partial one — so a soft delete always carries WHO and WHY. `nguoiMa` is a STAFF BUSINESS CODE
// (rule 6, invariant 8): `deleted_by` is read years later by somebody handling a complaint, and a
// ULID there names nobody.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND DELETE HARMLESS: it cannot overwrite who deleted
// the task or why, so a double-clicked button leaves the first reason standing. That is also what
// the route's `idem.KhongCan` declaration rests on.
//
// ⚠ IT DOES NOT LOOK AT THE CHILDREN. ADR 0037 decision 3 is the CALLER's, checked on the tree read
// under the lock — a store method that also counted children would be a rule expressed twice, and
// the copy that drifts is the one nothing tests.
func (s *NhiemVuStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, nguoiMa, lyDo string,
	luc time.Time) error {

	const stmt = `UPDATE nhiem_vu
		SET deleted_at = $3, deleted_by = $4, delete_reason = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, luc, nguoiMa, lyDo)
	if err != nil {
		// NOT the reason text: it is free text somebody typed about a government record, and an
		// error message travels into centralised logging (rule 3, forbidden #3).
		return fmt.Errorf("nhiem_vu: xoá mềm nhiệm vụ: %w", err)
	}
	return doiMotDongNhiemVu(kq, "xoá mềm")
}

// doiMotDongNhiemVu turns "no row matched" into the sentinel the caller answers 409 for.
//
// ONE FUNCTION FOR EVERY WRITE ABOVE, because copies of this drift and the copy that drifts is the
// one that treats zero rows as success — an officer's click that silently did nothing, on a screen
// that then shows the old state as though it were the new one.
func doiMotDongNhiemVu(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("nhiem_vu: %s, đếm dòng: %w", viec, err)
	}
	if n == 0 {
		// The caller read the row under FOR UPDATE a moment ago, so it exists and belongs to this
		// commune. Zero rows therefore means the row moved between the two statements.
		return ErrNhiemVuDaChuyenTrang
	}
	return nil
}

// --- the progress log (§5.9) -----------------------------------------------------------------------

// GhiNhatKy appends one timeline entry INSIDE the caller's transaction.
//
// ONE METHOD, NO UPDATE AND NO DELETE, because the table is APPEND-ONLY and migration 0006 enforces
// it with a trigger (rule 7, forbidden #5). There is no signature here that could edit an entry, so
// "correct the timeline" can only ever mean writing another entry that carries the correction.
//
// IT IS WRITTEN IN THE SAME TRANSACTION AS THE ACT IT DESCRIBES. A status change that committed
// without its timeline row would leave a gap an officer reads as "nothing happened here", and the
// resume rule of §6 — which derives the state before a pause FROM this table — would then answer
// from a history with a hole in it.
func (s *NhiemVuStore) GhiNhatKy(ctx context.Context, tx *store.ScopedTx, e domain.NhatKyNhiemVu) error {
	const stmt = `INSERT INTO nhat_ky_nhiem_vu (
		tenant_id, id, nhiem_vu_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem,
		bo_phan_id, nguoi_phu_trach_ma, noi_dung)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), e.ID, e.NhiemVuID, e.ThoiDiem, e.NguoiMa,
		string(e.TrangThaiTaiThoiDiem), rongThanhNull(e.BoPhanID), rongThanhNull(e.NguoiPhuTrachMa),
		e.NoiDung)
	if err != nil {
		// NOT the entry text: an officer writes free text here about the work, and an INSERT error
		// can quote the whole row on some drivers (rule 3).
		return fmt.Errorf("nhat_ky_nhiem_vu: ghi nhật ký: %w", err)
	}
	return nil
}

// TranDocNhatKy bounds how far back the resume rule looks.
//
// # WHY THE WINDOW IS BOUNDED AT ALL
//
// §6's "(trạng thái trước)" is the row before the pause, and in every real timeline it is within a
// handful of entries. Reading the whole log to find it would make the cost of resuming a task grow
// with how much was written on it, inside a transaction holding the row lock. When the answer is
// not in the window the use case REFUSES and says so — it does not guess, which is the one thing
// that would put work into a state nobody chose.
const TranDocNhatKy = 50

// NhatKyGanNhat reads the newest entries of one task, NEWEST FIRST.
//
// IT RETURNS domain.MocNhatKy AND NOT THE WHOLE ROW, and the narrowness is the point: the only
// caller is the resume rule, which needs one column. A method returning the full entries would be a
// method a later edit could make the rule depend on the author or the text of — neither of which
// decides where a paused task resumes to.
func (s *NhiemVuStore) NhatKyGanNhat(ctx context.Context, tx *store.ScopedTx, nhiemVuID string,
	n int) ([]domain.MocNhatKy, error) {

	const stmt = `SELECT trang_thai_tai_thoi_diem FROM nhat_ky_nhiem_vu
		WHERE tenant_id = $1 AND nhiem_vu_id = $2
		ORDER BY thoi_diem DESC, id DESC
		LIMIT $3`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), nhiemVuID, n)
	if err != nil {
		return nil, fmt.Errorf("nhat_ky_nhiem_vu: đọc nhật ký gần nhất: %w", err)
	}
	defer rows.Close()

	var ra []domain.MocNhatKy
	for rows.Next() {
		var tt string
		if err := rows.Scan(&tt); err != nil {
			return nil, fmt.Errorf("nhat_ky_nhiem_vu: đọc dòng nhật ký: %w", err)
		}
		ra = append(ra, domain.MocNhatKy{TrangThai: domain.TrangThaiNhiemVu(tt)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhat_ky_nhiem_vu: duyệt nhật ký: %w", err)
	}
	return ra, nil
}

// --- the extension requests (§5.8) --------------------------------------------------------------------

// DeNghiLuiHanStore is the only path to `de_nghi_lui_han` (migration 0006).
//
// A SEPARATE STORE FROM THE REGISTER, unlike the progress log which sits on NhiemVuStore, and the
// difference is what the two tables ARE. The log has no state of its own: it is append-only and
// exists only as the trace of acts on a task. An extension request is a RECORD WITH ITS OWN
// LIFECYCLE — filed, then approved or rejected, by a different person under a different permission
// — so it has a locking read and a decision statement that have no counterpart on the log.
type DeNghiLuiHanStore struct {
	db *store.DB
}

func NewDeNghiLuiHanStore(db *store.DB) *DeNghiLuiHanStore { return &DeNghiLuiHanStore{db: db} }

const cotDeNghi = `id, nhiem_vu_id, nguoi_de_nghi_ma, nguoi_duyet_ma, han_moi, ly_do,
	trang_thai, thoi_diem, duyet_luc`

// Tao files one request INSIDE the caller's transaction.
//
// `trang_thai` IS A LITERAL. Read that as the property it is: there is no $n for it, so no layer
// above can file a request that is already approved — the decision columns are NULL by construction
// and `de_nghi_lui_han_quyet_dinh_day_du` requires exactly that of a pending row.
//
// AT MOST ONE PENDING REQUEST PER TASK is enforced by `UNIQUE (tenant_id, nhiem_vu_id,
// moc_cho_duyet)` underneath — two pending requests carrying two different dates is a leader with
// two buttons and no way to tell what approving either one means. The caller checks first so the
// ordinary case is a sentence rather than a constraint error.
func (s *DeNghiLuiHanStore) Tao(ctx context.Context, tx *store.ScopedTx, d domain.DeNghiLuiHan) error {
	const stmt = `INSERT INTO de_nghi_lui_han (
		tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, han_moi, ly_do, trang_thai, thoi_diem)
		VALUES ($1,$2,$3,$4,$5,$6,'cho-duyet',$7)`

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), d.ID, d.NhiemVuID, d.NguoiDeNghiMa,
		d.HanMoi, d.LyDo, d.ThoiDiem)
	if err != nil {
		// NOT the reason text (rule 3, forbidden #3).
		return fmt.Errorf("de_nghi_lui_han: ghi đề nghị: %w", err)
	}
	return nil
}

// DangChoDuyet reports whether this task already has a request awaiting a decision.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE — the same split MaDaDung makes.
func (s *DeNghiLuiHanStore) DangChoDuyet(ctx context.Context, tx *store.ScopedTx, nhiemVuID string) (
	bool, error) {

	const stmt = `SELECT count(*) FROM de_nghi_lui_han
		WHERE tenant_id = $1 AND nhiem_vu_id = $2 AND trang_thai = 'cho-duyet' AND deleted_at IS NULL`

	var n int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), nhiemVuID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("de_nghi_lui_han: kiểm tra đề nghị đang chờ: %w", err)
	}
	return n > 0, nil
}

// TheoIDDeSua reads one live request of ONE TASK, under the lock.
//
// IT TAKES BOTH IDS AND MATCHES BOTH. The request id alone would be enough to find the row, and
// matching the task as well is what stops a request of task A being decided through task B's URL —
// where the leader named on B would be compared against a request filed on A. The two-layer rule of
// ADR 0038 reads the leader from the TASK, so the pair has to be checked in the query that fetches
// the request.
func (s *DeNghiLuiHanStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx,
	nhiemVuID, id string) (domain.DeNghiLuiHan, error) {

	const stmt = `SELECT ` + cotDeNghi + ` FROM de_nghi_lui_han
		WHERE tenant_id = $1 AND id = $2 AND nhiem_vu_id = $3 AND deleted_at IS NULL FOR UPDATE`

	d, err := quetDeNghi(tx.Underlying().QueryRowContext(ctx, stmt,
		string(tx.TenantID()), id, nhiemVuID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DeNghiLuiHan{}, ErrDeNghiKhongTonTai
	}
	if err != nil {
		return domain.DeNghiLuiHan{}, err
	}
	return d, nil
}

// QuyetDinh records the decision — and the decision ONLY.
//
// `han_moi`, `ly_do`, `nguoi_de_nghi_ma` AND `thoi_diem` ARE NOT IN THE SET LIST, and migration
// 0006's `de_nghi_lui_han_bat_bien` trigger refuses them underneath: rewriting what was ASKED FOR
// after a decision would leave an approval attached to something the approver never read.
//
// THE WHERE CLAUSE CARRIES `trang_thai = 'cho-duyet'`, so a second decision on one request matches
// no row. One request, one answer, one audit entry.
func (s *DeNghiLuiHanStore) QuyetDinh(ctx context.Context, tx *store.ScopedTx, id string,
	sang domain.TrangThaiDeNghi, nguoiDuyetMa string, luc time.Time) error {

	const stmt = `UPDATE de_nghi_lui_han
		SET trang_thai = $3, nguoi_duyet_ma = $4, duyet_luc = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = 'cho-duyet' AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, string(sang), nguoiDuyetMa, luc)
	if err != nil {
		return fmt.Errorf("de_nghi_lui_han: ghi quyết định: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("de_nghi_lui_han: ghi quyết định, đếm dòng: %w", err)
	}
	if n == 0 {
		// The caller read the row under FOR UPDATE a moment ago, so it exists. Zero rows means
		// somebody decided it first.
		return domain.ErrDeNghiDaQuyetDinh
	}
	return nil
}

// quetDeNghi reads one row of cotDeNghi. POSITIONAL, IN LOCKSTEP WITH IT.
func quetDeNghi(r quangKiem) (domain.DeNghiLuiHan, error) {
	var (
		d        domain.DeNghiLuiHan
		tt       string
		duyet    sql.NullString
		duyetLuc sql.NullTime
	)
	if err := r.Scan(&d.ID, &d.NhiemVuID, &d.NguoiDeNghiMa, &duyet, &d.HanMoi, &d.LyDo,
		&tt, &d.ThoiDiem, &duyetLuc); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DeNghiLuiHan{}, err
		}
		return domain.DeNghiLuiHan{}, fmt.Errorf("de_nghi_lui_han: đọc dòng: %w", err)
	}
	d.TrangThai = domain.TrangThaiDeNghi(tt)
	d.NguoiDuyetMa = duyet.String
	d.DuyetLuc = duyetLuc.Time
	return d, nil
}
