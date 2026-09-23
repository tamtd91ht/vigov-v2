package store

// The WRITE path of the investment project register (`du_an`) and the funding allocation lines a
// project is created with (`phan_bo_nguon_von`).
//
// FIVE THINGS HOLD ACROSS EVERY METHOD BELOW, and each one is a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, taken from tx.TenantID() which took it from the
//     context (rule 1, invariants 4 and 5). It is never a parameter of any method here, so a
//     caller cannot name another commune's row even by mistake.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS — except MaDaDung, which must see them, and says why.
//  4. THERE IS NO HARD DELETE, and `ho_so_luu_tru_cam_xoa_cung` (0004:325-328) refuses one even
//     if somebody writes it (rule 7, forbidden #1).
//  5. `ma` AND `nam` APPEAR IN NO UPDATE STATEMENT. An issued code is never renumbered (rule 7,
//     forbidden #4) and a project never moves between budget years (§13 rule 8). Both are refused
//     in the domain with a sentence; here they are refused by not existing as a column anybody can
//     reach, which is the half a future edit cannot argue with.
//
// THIS FILE ADDS NO MIGRATION AND NEEDS NONE. Every column it writes was declared by 0004 and 0007,
// including `phan_bo_nguon_von` in full — 0007 shipped the table and deliberately shipped no write
// path, and this is that write path.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// DuAnGhiStore writes one commune's investment projects.
//
// A SECOND TYPE BESIDE DuAnStore RATHER THAN MORE METHODS ON IT, and the reason is the transaction.
// Every method here takes the caller's *store.ScopedTx; DuAnStore reads through db.For(ctx), which
// is a connection of its own. Behind one type a future caller would reach for whichever method was
// nearest and could end up writing a project outside the transaction its audit entry lives in —
// the exact defect core/audit was shaped to make impossible.
//
// It holds *store.DB and never a *sql.DB, for the same reason DuAnStore does (rule 1, invariant 5).
type DuAnGhiStore struct {
	db *store.DB
}

func NewDuAnGhiStore(db *store.DB) *DuAnGhiStore { return &DuAnGhiStore{db: db} }

var (
	// ErrMaDuAnDaTonTai — the code is already taken in this commune, SOFT-DELETED ROWS INCLUDED.
	//
	// §9 states it in the modal's own footnote: *"Mã tự nhập phải chưa từng được dùng, KỂ CẢ BỞI DỰ
	// ÁN ĐÃ RÚT KHỎI DANH SÁCH."* Rule 7, invariant 3 says the same thing for the whole system. The
	// message the handler returns has to explain why a code that is nowhere on the screen is
	// nonetheless taken, or this reads as a bug.
	ErrMaDuAnDaTonTai = errors.New("du_an: mã dự án đã được dùng trong xã này")

	// ErrKhongThayHangMuc — the capital plan category a project is classified under does not exist
	// in this commune, or has been removed from its catalogue.
	//
	// WHY THE APPLICATION HAS TO ASK AT ALL: migration 0004 declares NO foreign key from
	// `du_an.hang_muc_id` to `hang_muc_ke_hoach_von`, and it says so outright rather than leaving
	// the absence to be discovered (0004:182-190) — a real FK between two HASH-partitioned tables is
	// accepted by PostgreSQL 13, but no PostgreSQL is reachable from this repository's build
	// environment and a migration that fails at startup stops the service.
	//
	// SO THIS CHECK IS THE ONLY ONE THERE IS, AND IT GUARDS TWO DIFFERENT FAILURES AT ONCE:
	//
	//	an id that matches nothing   the project's plan would be counted in §3's "KẾ HOẠCH VỐN NĂM"
	//	                             and missing from every row of §5's "Tiến độ theo hạng mục" — two
	//	                             totals on one screen that disagree, with no row looking wrong.
	//	a category of ANOTHER commune  rule 1. The statement binds tenant_id = $1, so such an id is
	//	                             indistinguishable from one that does not exist — which is the
	//	                             answer this error carries, and it leaks nothing about what
	//	                             another authority holds.
	ErrKhongThayHangMuc = errors.New("du_an: không có hạng mục kế hoạch vốn này trong xã")

	// ErrKhongThayNguonVonPhanBo — the funding source an allocation line names does not exist in this
	// commune, or belongs to a DIFFERENT budget year.
	//
	// THE YEAR IS PART OF THE CHECK AND THAT IS NOT AN EXTRA. `nguon_von` carries `nam` (0007) and
	// §13 rule 8 makes each budget year its own set of data; §6's cards are drawn per year. A 2026
	// project allocated against a 2027 source would put its money on a card of the wrong year — where
	// it inflates one year's "đã phân bổ" and is missing from the other's, on two screens that are
	// each internally consistent.
	ErrKhongThayNguonVonPhanBo = errors.New("du_an: không có nguồn vốn này trong xã ở năm ngân sách của dự án")

	// ErrDuAnConChungTu — a project still carrying LIVE disbursement vouchers cannot be removed.
	//
	// ⚠ THIS IS A DECISION THIS REPOSITORY IS NOT ENTITLED TO MAKE PERMANENTLY, AND IT IS FLAGGED AS
	// SUCH. The specification says nothing about removing a project that has vouchers filed against
	// it: refuse, cascade, or leave the vouchers behind are three different answers with three
	// different consequences, and ADR 0037 settled the same shape for the TASK TREE — a different
	// record, whose answer does not carry over.
	//
	// REFUSAL IS THE ONLY DIRECTION THAT WRITES NOTHING. The other two are one-way:
	//
	//	leave them   the vouchers stay live while every read path drops the project (rule 7,
	//	             invariant 2), so their money disappears from §3's "ĐÃ GIẢI NGÂN" and from §5's
	//	             category totals while still sitting in `chung_tu_giai_ngan`. The commune's own
	//	             figures then disagree with the sum of its own vouchers and no row looks wrong.
	//	cascade      soft-deleting somebody else's payment records as a side effect of removing a
	//	             project — including CONFIRMED and LOCKED ones, which `chung_tu_da_khoa` refuses
	//	             outright for a reason (a locked voucher is a figure somebody has signed off).
	//
	// Loosening this the day the customer answers costs one branch. Tightening it afterwards means
	// every removal already made was made under a rule nobody chose.
	//
	// SOFT-DELETED VOUCHERS DO NOT COUNT. They are out of every read path already, so nothing is
	// stranded by removing the project they pointed at.
	ErrDuAnConChungTu = errors.New("du_an: dự án này còn chứng từ giải ngân nên chưa xoá được")
)

// cotDuAnGhi IS READ BY POSITION in the scan below, and the list has pairs that swap with no error
// whatsoever — the same pairs cotDuAn names, for the same reason:
//
//	ke_hoach_von_nam, tong_muc_duoc_duyet   two amounts. A swap changes every ratio on every screen.
//	ngay_khoi_cong, ngay_hoan_thanh,        three dates. A swap puts the disbursement deadline on the
//	  thoi_han_giai_ngan                    wrong clock, which §9 warns is NOT the completion date.
//
// IT IS NOT cotDuAn AND CANNOT BE. That constant prefixes every column with `da.` for the join
// behind the read path; this statement reads one table with no alias. Two constants, because a
// single one carrying the alias would silently attach this write path to the shape of a read.
const cotDuAnGhi = `id, ma, nam, hang_muc_id, ten, COALESCE(mo_ta, ''), ` +
	`ke_hoach_von_nam, COALESCE(tong_muc_duoc_duyet, 0), ` +
	`COALESCE(don_vi_thuc_hien_id, ''), COALESCE(can_bo_phu_trach_id, ''), ` +
	`ngay_khoi_cong, ngay_hoan_thanh, thoi_han_giai_ngan`

func docMotDongDuAn(quet func(...any) error) (domain.DuAn, error) {
	var (
		d             domain.DuAn
		khoiCong      sql.NullTime
		hoanThanh     sql.NullTime
		thoiHan       time.Time
		keHoach, tong int64
	)
	err := quet(&d.ID, &d.Ma, &d.Nam, &d.HangMucID, &d.Ten, &d.MoTa,
		&keHoach, &tong, &d.DonViThucHienID, &d.CanBoPhuTrachID,
		&khoiCong, &hoanThanh, &thoiHan)
	if err != nil {
		return domain.DuAn{}, err
	}
	d.KeHoachVonNam = domain.Dong(keHoach)
	d.TongMucDuocDuyet = domain.Dong(tong)
	d.ThoiHanGiaiNgan = thoiHan
	if khoiCong.Valid {
		d.NgayKhoiCong = khoiCong.Time
	}
	if hoanThanh.Valid {
		d.NgayHoanThanh = hoanThanh.Time
	}
	return d, nil
}

// TheoIDDeSua reads one live project inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Both writes that use it are
// read-decide-write: read the project, work out what the edit would produce, refuse or apply.
// Without the lock two members of staff editing the same project both read the old figures and both
// write over each other, and the audit trail records two edits whose `truoc` values disagree about
// what the row held — so neither entry can be believed afterwards.
//
// IT IS ALSO WHAT MAKES THE REMOVAL CHECK MEAN ANYTHING. Go counts the project's live vouchers and
// refuses if there are any; read without a lock, a voucher entered between the count and the UPDATE
// would land on a project that is being removed in another transaction, and its money would be
// stranded exactly as ErrDuAnConChungTu exists to prevent.
func (s *DuAnGhiStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.DuAn, error) {

	const stmt = `SELECT ` + cotDuAnGhi + ` FROM du_an ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	d, err := docMotDongDuAn(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		// THE SAME ERROR THE READ PATH USES, deliberately: "no such project" and "a project of
		// another commune" are one answer, because the query cannot reach another commune's row at
		// all. The handler answers 404 either way and so leaks nothing about what another authority
		// holds.
		return domain.DuAn{}, ErrKhongThayDuAn
	}
	if err != nil {
		return domain.DuAn{}, fmt.Errorf("du_an: đọc dự án để sửa: %w", err)
	}
	return d, nil
}

// MaDaDung reports whether this project code has EVER been issued in this commune.
//
// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE, and this is the line to read twice. §9:
// *"Mã tự nhập phải chưa từng được dùng, kể cả bởi dự án đã rút khỏi danh sách."* Rule 7, invariant
// 3 says the same for the whole system. Restrict this to live rows and a withdrawn project's code
// can be reissued to an unrelated project — after which the vouchers of the first one read as
// belonging to the second, and the audit entries filed under that code name two different projects.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of one code
// can both pass this check, and the second then hits `UNIQUE (tenant_id, ma)` (0004:216-221) and
// rolls the whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade:
// the constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *DuAnGhiStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	const stmt = `SELECT count(*) FROM du_an WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("du_an: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// HangMucConSong refuses a category that is not a live category OF THIS COMMUNE.
//
// IT LIVES HERE AND NOT ON HangMucKeHoachVonStore, AND THE REASON IS THE TRANSACTION — the same one
// ChungTuGiaiNganStore.NguonVonConSong gives. Every method of this file takes the caller's
// *store.ScopedTx; the catalogue store reads through db.For(ctx), which is a connection of its own,
// so the answer could already be stale by the time the INSERT runs.
//
// NOT `FOR UPDATE`: this reads a fact about a row nothing here writes, and locking the catalogue row
// would serialise every project the commune enters behind whoever last touched a category.
//
// `dang_dung` IS NOT IN THE PREDICATE, AND THAT IS DELIBERATE. A category may be switched off (`Tắt`)
// while projects already classified under it stay where they are — §5 still totals them. What is
// refused is a category that does not exist or has been removed; refusing a disabled one as well
// would mean a commune that tidies its catalogue can no longer enter a project against last year's
// classification, which is precisely what the `chuyen-tiep` and `keo-dai` categories of §5 are for.
func (s *DuAnGhiStore) HangMucConSong(ctx context.Context, tx *store.ScopedTx,
	hangMucID string) error {

	const stmt = `SELECT 1 FROM hang_muc_ke_hoach_von ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var mot int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), hangMucID).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrKhongThayHangMuc
	}
	if err != nil {
		return fmt.Errorf("du_an: kiểm hạng mục kế hoạch vốn: %w", err)
	}
	return nil
}

// NguonVonConSongTrongNam refuses a funding source that is not a live source of this commune IN THIS
// BUDGET YEAR.
//
// THE YEAR IS BOUND AS $3 AND IS THE PROJECT'S OWN — never a client's, and never the calendar's. A
// source of the wrong year would put a project's allocation on a card §6 draws for a different year.
//
// NOT `FOR UPDATE`, for the same reason HangMucConSong is not.
//
// IT RETURNS ONLY AN ERROR. There is no `ma` column on `nguon_von` (§11 gives it a name and no
// code), so there is no business code to hand back; the audit subject stays the PROJECT's code and
// the source id is named inside the delta.
func (s *DuAnGhiStore) NguonVonConSongTrongNam(ctx context.Context, tx *store.ScopedTx,
	nguonVonID string, nam int) error {

	const stmt = `SELECT 1 FROM nguon_von ` +
		`WHERE tenant_id = $1 AND id = $2 AND nam = $3 AND deleted_at IS NULL`

	var mot int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), nguonVonID, nam).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrKhongThayNguonVonPhanBo
	}
	if err != nil {
		return fmt.Errorf("du_an: kiểm nguồn vốn phân bổ: %w", err)
	}
	return nil
}

// DemChungTuConSong counts the LIVE disbursement vouchers filed against one project.
//
// IT IS WHAT ErrDuAnConChungTu IS DECIDED ON, and it counts live rows only: a soft-deleted voucher
// is already out of every read path and out of `da_giai_ngan`, so nothing is stranded by removing
// the project it pointed at.
func (s *DuAnGhiStore) DemChungTuConSong(ctx context.Context, tx *store.ScopedTx,
	duAnID string) (int, error) {

	const stmt = `SELECT count(*) FROM chung_tu_giai_ngan ` +
		`WHERE tenant_id = $1 AND du_an_id = $2 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), duAnID).Scan(&n); err != nil {
		return 0, fmt.Errorf("du_an: đếm chứng từ còn sống: %w", err)
	}
	return n, nil
}

// chenDuAn — `tenant_id` IS $1 AND EVERY OPTIONAL TEXT COLUMN GOES THROUGH rongThanhNil.
//
// `thoi_han_giai_ngan` IS NOT NULLABLE AND IS NOT OPTIONAL HERE. §9 lets the commune leave the box
// blank and §11 says the default is 31/12; domain.HanGiaiNganMacDinh applies that in ONE place and
// the caller has already done so, so this statement always receives a real date. A NULL would be
// refused by the column and a zero time would store 01/01/0001 — a deadline every project on the
// screen is past.
//
// `tong_muc_duoc_duyet` IS WRITTEN AS NULL WHEN IT IS ZERO, and that is the migration's own design
// (0004:199-203): NULL means "the same as this year's plan", and a COPIED value silently stops
// following the plan the day the plan is revised, with nothing on the screen saying which of the two
// figures is stale. domain.DuAn.TongMucHieuLuc reads the rule back out.
//
// `deleted_at`, `deleted_by`, `delete_reason` ARE ABSENT AND TAKE THEIR NULL DEFAULTS. A new project
// has not been removed, which is a fact rather than an absence.
const chenDuAn = `INSERT INTO du_an
	(tenant_id, id, ma, nam, hang_muc_id, ten, mo_ta, ke_hoach_von_nam, tong_muc_duoc_duyet,
	 don_vi_thuc_hien_id, can_bo_phu_trach_id, ngay_khoi_cong, ngay_hoan_thanh, thoi_han_giai_ngan)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

// Chen adds one project.
func (s *DuAnGhiStore) Chen(ctx context.Context, tx *store.ScopedTx, d domain.DuAn) error {
	_, err := tx.Exec(ctx, chenDuAn, string(tx.TenantID()),
		d.ID, d.Ma, d.Nam, d.HangMucID, d.Ten, rongThanhNil(d.MoTa),
		int64(d.KeHoachVonNam), khongThanhNil(d.TongMucDuocDuyet),
		rongThanhNil(d.DonViThucHienID), rongThanhNil(d.CanBoPhuTrachID),
		ngayThanhNil(d.NgayKhoiCong), ngayThanhNil(d.NgayHoanThanh), d.ThoiHanGiaiNgan)
	if err != nil {
		return fmt.Errorf("du_an: chèn: %w", err)
	}
	return nil
}

// capNhatDuAn — `ma` AND `nam` APPEAR NOWHERE IN THIS STATEMENT, and that is the half of the rule a
// future edit cannot argue with.
//
// `ma` because an issued code is never renumbered (rule 7, forbidden #4): it is printed on §7.2's
// row, quoted in the disbursement decisions filed against the project, and carried as the `subject`
// of every audit entry the project's vouchers have ever written. Changing it detaches a paper trail
// from the record it describes, in one UPDATE, leaving one entry that reads as an ordinary edit.
//
// `nam` because §13 rule 8 makes each budget year its own set of projects. Moving a project between
// years moves its whole plan AND every voucher filed against it out of one year's totals and into
// another's — §3's KPI cards, §5's category table and §4's chart, all of which have already been
// read off a screen. The domain refuses it with a sentence (ErrNamBatBien); this refuses it by
// having no column to reach.
//
// THE OTHER TEN FIELDS ARE ALL EDITABLE, which is §8's `[✎ Sửa dự án]` button. A LOCKED or CONFIRMED
// VOUCHER DOES NOT FREEZE THEM: ADR 0036 decided that a CONFIRMED VOUCHER goes back to draft when its
// own figures move, and that decision is about the voucher's own confirmation. A project has no
// `trang_thai` column and no confirmation to lose, so there is nothing here for that rule to act on
// — and inventing an equivalent would be deciding a question the specification has not asked.
const capNhatDuAn = `UPDATE du_an
	SET hang_muc_id = $3, ten = $4, mo_ta = $5,
	    ke_hoach_von_nam = $6, tong_muc_duoc_duyet = $7,
	    don_vi_thuc_hien_id = $8, can_bo_phu_trach_id = $9,
	    ngay_khoi_cong = $10, ngay_hoan_thanh = $11, thoi_han_giai_ngan = $12,
	    cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the ten fields a member of staff may correct. The caller has already read the row
// with TheoIDDeSua.
func (s *DuAnGhiStore) CapNhat(ctx context.Context, tx *store.ScopedTx, d domain.DuAn) error {
	kq, err := tx.Exec(ctx, capNhatDuAn, string(tx.TenantID()),
		d.ID, d.HangMucID, d.Ten, rongThanhNil(d.MoTa),
		int64(d.KeHoachVonNam), khongThanhNil(d.TongMucDuocDuyet),
		rongThanhNil(d.DonViThucHienID), rongThanhNil(d.CanBoPhuTrachID),
		ngayThanhNil(d.NgayKhoiCong), ngayThanhNil(d.NgayHoanThanh), d.ThoiHanGiaiNgan)
	if err != nil {
		return fmt.Errorf("du_an: cập nhật: %w", err)
	}
	return doiMotDongDuAn(kq, "cập nhật")
}

// xoaMemDuAn writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by` and
// `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND REMOVAL A 404 rather than a silent rewrite of who
// removed the project and why. The first removal is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
//
// THE ROW STAYS, AND SO DOES ITS CODE. `ho_so_luu_tru_cam_xoa_cung` refuses a hard DELETE on this
// table (0004:325-328), and `UNIQUE (tenant_id, ma)` counts the removed row — so the code it was
// issued is never available again, which is exactly what §9 promises about a project "đã rút khỏi
// danh sách".
const xoaMemDuAn = `UPDATE du_an
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *DuAnGhiStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemDuAn, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("du_an: xoá mềm: %w", err)
	}
	return doiMotDongDuAn(kq, "xoá mềm")
}

// chenPhanBo adds one allocation line — how much of a project's year plan is drawn from which
// source (§9's `+ Thêm nguồn vốn`, §11's `phan_bo_nguon_von`).
const chenPhanBo = `INSERT INTO phan_bo_nguon_von
	(tenant_id, id, du_an_id, nguon_von_id, so_tien_phan_bo)
	VALUES ($1, $2, $3, $4, $5)`

// ChenPhanBo writes one allocation line. It is called INSIDE the project's own transaction, so a
// line that cannot be written takes the project and its audit entry down with it — a project half
// allocated is a §6 card that is wrong with nothing saying so.
func (s *DuAnGhiStore) ChenPhanBo(ctx context.Context, tx *store.ScopedTx,
	pb domain.PhanBoNguonVon) error {

	_, err := tx.Exec(ctx, chenPhanBo, string(tx.TenantID()),
		pb.ID, pb.DuAnID, pb.NguonVonID, int64(pb.SoTien))
	if err != nil {
		return fmt.Errorf("phan_bo_nguon_von: chèn: %w", err)
	}
	return nil
}

// xoaMemPhanBoCuaDuAn soft deletes every live allocation line of one project.
//
// WHY THE LINES GO WHEN THE PROJECT GOES, while its VOUCHERS block the removal entirely: they are
// different kinds of record. An allocation line is a PLAN — a statement about where this project's
// money is supposed to come from — and it has no meaning apart from the project. Left live, it would
// keep counting toward §6's "đã phân bổ" and toward that source's "Số dự án" for a project no screen
// can show. A voucher is a PAYMENT that really happened, which is why removing one is its own
// audited act and why a project still holding any is refused outright (ErrDuAnConChungTu).
//
// NO `RowsAffected` CHECK, AND THAT IS THE DIFFERENCE FROM EVERY OTHER SOFT DELETE HERE: zero rows is
// the ORDINARY case. §9 says a project with no allocation is normal ("Xã theo dõi kế hoạch vốn theo
// hạng mục thì để trống cũng được"), so treating "nothing to remove" as "row not found" would refuse
// the removal of exactly the projects §9 calls ordinary.
const xoaMemPhanBoCuaDuAn = `UPDATE phan_bo_nguon_von
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND du_an_id = $2 AND deleted_at IS NULL`

// XoaMemPhanBoCuaDuAn removes a project's allocation lines, in the project's own transaction. It
// returns how many lines it removed, so the audit delta can say — a number nobody can rebuild once
// the rows carry `deleted_at`.
func (s *DuAnGhiStore) XoaMemPhanBoCuaDuAn(ctx context.Context, tx *store.ScopedTx,
	duAnID, boi, lyDo string) (int, error) {

	kq, err := tx.Exec(ctx, xoaMemPhanBoCuaDuAn, string(tx.TenantID()), duAnID, boi, lyDo)
	if err != nil {
		return 0, fmt.Errorf("phan_bo_nguon_von: xoá mềm theo dự án: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("phan_bo_nguon_von: xoá mềm theo dự án: đọc số dòng: %w", err)
	}
	return int(n), nil
}

// doiMotDongDuAn turns "nothing was updated" into ErrKhongThayDuAn.
//
// IT IS A THIRD COPY OF doiMotDong ON PURPOSE, and the difference is the error it returns: the
// catalogue's answers ErrDanhMucKhongTonTai and the voucher's answers ErrKhongThayChungTu, each of
// which a handler turns into a sentence about that kind of record. Sharing one would put "Không tìm
// thấy mục danh mục này." in front of somebody who was removing an investment project.
func doiMotDongDuAn(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("du_an: %s: đọc số dòng: %w", viec, err)
	}
	if n == 0 {
		return ErrKhongThayDuAn
	}
	return nil
}

// khongThanhNil writes a zero amount as NULL rather than as 0.
//
// IT IS FOR `tong_muc_duoc_duyet` AND FOR NOTHING ELSE, and it must never be pointed at
// `ke_hoach_von_nam`. On that column zero is a REAL FIGURE — a project entered before its allocation
// is decided (0004:224-229) — and writing it as NULL would violate `NOT NULL`. On this one, NULL is
// the migration's own spelling of "the commune left it blank, read it as this year's plan"
// (0004:199-203); a stored 0 would read as "this project is approved for nothing at all".
func khongThanhNil(so domain.Dong) any {
	if so == 0 {
		return nil
	}
	return int64(so)
}

// ngayThanhNil writes a zero time as NULL rather than as 01/01/0001.
//
// WITHOUT IT AN UNSET DATE BECOMES A REAL ONE. `ngay_khoi_cong` and `ngay_hoan_thanh` are both
// nullable and both optional in §9; a zero `time.Time` sent to PostgreSQL stores year 1, which sorts
// to the front of every chart, prints as a date on §8, and makes "chưa xác định" indistinguishable
// from a project that started two millennia ago. The reads COALESCE nothing here — they scan through
// sql.NullTime — so NULL is the only spelling of "not set" they can report.
func ngayThanhNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
