package store

// The commune's revenue/expenditure budget board — reads and writes
// (docs/ui-ux/07-thu-chi-ngan-sach.md, migration 0006).
//
// FIVE THINGS HOLD ACROSS EVERY METHOD BELOW, and each one is a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, taken from the context through db.For(ctx) or
//     tx.TenantID() (rule 1, invariants 4 and 5). It is never a parameter of any method here.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS. There is no method here that can see one, and the one
//     place that deliberately counts them is LanKeTiep — see the reason there.
//  4. THERE IS NO HARD DELETE, and `ho_so_luu_tru_cam_xoa_cung` refuses one even if somebody writes
//     it (rule 7, forbidden #1). CLEARING A CELL SETS `gia_tri` TO NULL; it does not remove the row.
//  5. `is_headline` IS WRITTEN IN EXACTLY TWO STATEMENTS — the pair inside DatDongTong, and the soft
//     delete (which releases it). That is load-bearing rather than tidy: migration 0006 states why
//     the database carries no partial unique index for "at most one marked row", so what keeps the
//     property is that no other statement in this package can set the column.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// NganSachStore reads and writes one commune's budget board.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call, so
// there is no constructor, no field and no method here that could produce a statement without one
// (rule 1, invariant 5).
type NganSachStore struct {
	db *store.DB
}

func NewNganSachStore(db *store.DB) *NganSachStore { return &NganSachStore{db: db} }

var (
	// ErrKhongThayBangNganSach is "no such live sheet IN THIS COMMUNE" — the two halves are one
	// answer on purpose. A sheet of another commune is indistinguishable from one that does not
	// exist, because the query cannot reach it at all; the caller answers 404 either way and so
	// leaks nothing about what another authority holds.
	ErrKhongThayBangNganSach = errors.New("ngan_sach: không có bảng ngân sách này trong xã")

	// ErrBangDaTonTai — this commune already has a LIVE sheet for that year and kind. §6's `🗑 Gỡ`
	// is the way to replace one; creating a second alongside it would give the year two answers and
	// no way to tell which the report was built from.
	ErrBangDaTonTai = errors.New("ngan_sach: xã đã có bảng cho năm và loại này — gỡ bảng cũ trước khi nạp bảng mới")

	// ErrQuaNhieuKhoanMuc says the ceiling was reached. The caller answers 500 and REFUSES rather
	// than truncating: this tree is totalled into the figures that go upward, and a silently short
	// tree produces totals that are simply too small and look entirely normal.
	ErrQuaNhieuKhoanMuc = errors.New("ngan_sach: bảng vượt trần số khoản mục")
)

// TranKhoanMucMotBang is the hard upper bound on one sheet's lines.
//
// THE REAL FIGURE IS 59 (§10, the 2026 chi sheet) and 52 for thu. 5000 is roughly eighty times
// that: it is not an estimate of how many there might be, it is the point past which the thing
// being read is no longer one commune's budget report — an import run twice, a fixture on a live
// database. When a real commune approaches it the answer is paging plus server-side totals, NOT a
// bigger constant; a tree this long stopped being readable long before it stopped being loadable.
const TranKhoanMucMotBang = 5000

// --- reads ---------------------------------------------------------------------------------------

const cotBang = `id, ma, nam, loai, lan, tieu_de, don_vi_tinh, luy_ke_den, ` +
	`COALESCE(nguon_tep, ''), nap_luc`

func quetBang(quet func(...any) error) (domain.BangNganSach, error) {
	var (
		b      domain.BangNganSach
		loai   string
		luyKe  sql.NullTime
		napLuc sql.NullTime
	)
	err := quet(&b.ID, &b.Ma, &b.Nam, &loai, &b.Lan, &b.TieuDe, &b.DonViTinh,
		&luyKe, &b.NguonTep, &napLuc)
	if err != nil {
		return domain.BangNganSach{}, err
	}
	b.Loai = domain.LoaiBang(loai)
	if luyKe.Valid {
		b.LuyKeDen = luyKe.Time
	}
	if napLuc.Valid {
		b.NapLuc = napLuc.Time
	}
	return b, nil
}

// BangTheoNamLoai reads the commune's CURRENT sheet for one budget year and kind.
//
// `ORDER BY lan DESC LIMIT 1` AND NOT A BARE SINGLE-ROW READ: migration 0006 gives each `🗑 Gỡ` +
// reload cycle its own revision, so a year+kind can hold several rows of which at most one is live.
// The ordering makes the answer TOTAL even if a future writer ever leaves two live — it takes the
// newest rather than whichever the planner happened to return first.
func (s *NganSachStore) BangTheoNamLoai(ctx context.Context, nam int,
	loai domain.LoaiBang) (domain.BangNganSach, error) {

	rows, err := s.db.For(ctx).Query(ctx, cotBang, "bang_ngan_sach",
		`AND deleted_at IS NULL AND nam = $2 AND loai = $3 ORDER BY lan DESC LIMIT 1`,
		nam, string(loai))
	if err != nil {
		return domain.BangNganSach{}, fmt.Errorf("ngan_sach: đọc bảng: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.BangNganSach{}, fmt.Errorf("ngan_sach: đọc bảng: %w", err)
		}
		return domain.BangNganSach{}, ErrKhongThayBangNganSach
	}
	b, err := quetBang(rows.Scan)
	if err != nil {
		return domain.BangNganSach{}, fmt.Errorf("ngan_sach: đọc dòng bảng: %w", err)
	}
	return b, nil
}

// BangDayDu reads one whole sheet: its columns, every live line, and every filled cell.
//
// FOUR STATEMENTS AND NOT ONE JOIN, deliberately. A single join of lines × values would return one
// row per cell and repeat every line's text once per column — for the specification's own 59 lines
// and 6 columns that is 354 rows carrying 59 copies of each name. More importantly the shapes are
// different: a line with no cell at all must still appear (it is drawn with `—`), which an inner
// join drops and an outer join re-creates with NULLs nobody can tell from an empty cell.
func (s *NganSachStore) BangDayDu(ctx context.Context, nam int,
	loai domain.LoaiBang) (domain.BangDayDu, error) {

	bang, err := s.BangTheoNamLoai(ctx, nam, loai)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	return s.bangDayDuTheoBang(ctx, bang)
}

func (s *NganSachStore) bangDayDuTheoBang(ctx context.Context,
	bang domain.BangNganSach) (domain.BangDayDu, error) {

	cot, err := s.CotCuaBang(ctx, bang.ID)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	khoanMuc, err := s.KhoanMucCuaBang(ctx, bang.ID)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	gia, err := s.GiaTriCuaBang(ctx, bang.ID)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, tongDotCuaBang, bang.ID, string(domain.TinhTheoDot))
	if err != nil {
		return domain.BangDayDu{}, fmt.Errorf("ngan_sach: đọc tổng đợt: %w", err)
	}
	giaDot, vuot, err := quetTongDot(rows)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	return domain.BangDayDu{Bang: bang, Cot: cot, KhoanMuc: khoanMuc, Gia: gia,
		GiaDot: giaDot, GiaDotVuotMuc: vuot}, nil
}

const cotCot = `id, bang_id, ten, thu_tu, kieu, COALESCE(cong_thuc, ''), COALESCE(vai_tro, '')`

// CotCuaBang reads one sheet's columns in display order.
//
// NO LIMIT CLAUSE AND A BOUND ALL THE SAME: domain.SoCotToiDa is enforced on the WRITE path, where
// the whole set arrives at once and can be refused with a sentence. A ceiling here would truncate a
// column set, and a report missing a column is a report missing a figure.
func (s *NganSachStore) CotCuaBang(ctx context.Context, bangID string) ([]domain.CotNganSach, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotCot, "cot_ngan_sach",
		`AND deleted_at IS NULL AND bang_id = $2 ORDER BY thu_tu, id`, bangID)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc cột: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CotNganSach, 0, 8)
	for rows.Next() {
		var (
			c    domain.CotNganSach
			kieu string
			vai  string
		)
		if err := rows.Scan(&c.ID, &c.BangID, &c.Ten, &c.ThuTu, &kieu, &c.CongThuc, &vai); err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng cột: %w", err)
		}
		c.Kieu = domain.KieuCot(kieu)
		c.VaiTro = domain.VaiTroCot(vai)
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt cột: %w", err)
	}
	return ra, nil
}

const cotKhoanMuc = `id, bang_id, COALESCE(cha_id, ''), tt, ten, thu_tu, cach_tinh, cap, is_headline`

// KhoanMucCuaBang reads one sheet's whole tree, in display order.
//
// ORDER BY thu_tu, id: `thu_tu` is what the screen draws by and `id` makes the order TOTAL, so two
// lines the commune gave the same order cannot swap between two reads. An unstable order makes a
// client-side diff flicker on every reload and makes any test of this compare sets by accident.
//
// LIMIT IS THE CEILING PLUS ONE, which is what makes "there are too many" detectable at all.
// Selecting exactly the ceiling returns a full page indistinguishable from a complete tree of
// exactly that size — the truncation this read refuses to perform, performed by the bound meant to
// prevent it.
func (s *NganSachStore) KhoanMucCuaBang(ctx context.Context,
	bangID string) ([]domain.KhoanMucNganSach, error) {

	rows, err := s.db.For(ctx).Query(ctx, cotKhoanMuc, "khoan_muc_ngan_sach",
		`AND deleted_at IS NULL AND bang_id = $2 ORDER BY thu_tu, id LIMIT $3`,
		bangID, TranKhoanMucMotBang+1)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc khoản mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.KhoanMucNganSach, 0, 64)
	for rows.Next() {
		var (
			k        domain.KhoanMucNganSach
			cachTinh string
		)
		err := rows.Scan(&k.ID, &k.BangID, &k.ChaID, &k.TT, &k.Ten, &k.ThuTu,
			&cachTinh, &k.Cap, &k.LaDongTong)
		if err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng khoản mục: %w", err)
		}
		k.CachTinh = domain.CachTinh(cachTinh)
		ra = append(ra, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt khoản mục: %w", err)
	}
	if len(ra) > TranKhoanMucMotBang {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a tree the
		// caller might total anyway is how a refusal turns back into a silent truncation one careless
		// `if err != nil { log }` later.
		return nil, ErrQuaNhieuKhoanMuc
	}
	return ra, nil
}

// GiaTriCuaBang reads every FILLED cell of one sheet.
//
// `AND g.gia_tri IS NOT NULL` IS THE HALF THAT CARRIES §9 RULE 4. A cleared cell keeps its row with
// a NULL (there is no hard delete here), and the domain distinguishes "empty" from "0" by whether
// the key is PRESENT in the map. Letting a NULL through would put a zero into the map and collapse
// the two — and the collapse is invisible: the screen would print `0` for a section the commune has
// not entered, and that zero would be totalled into a figure sent upward.
//
// `tenant_id = $1` IS ON BOTH SIDES OF THE JOIN. Joining on `khoan_muc_id` alone would match another
// commune's row wherever ids collide, and no test of a single commune would ever show it (rule 1).
func (s *NganSachStore) GiaTriCuaBang(ctx context.Context,
	bangID string) (map[string]map[string]domain.Dong, error) {

	const stmt = `SELECT g.khoan_muc_id, g.cot_id, g.gia_tri
		FROM gia_tri_khoan_muc g
		JOIN khoan_muc_ngan_sach k
		  ON k.tenant_id = g.tenant_id AND k.id = g.khoan_muc_id
		WHERE g.tenant_id = $1 AND k.bang_id = $2
		  AND k.deleted_at IS NULL AND g.gia_tri IS NOT NULL`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, bangID)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc giá trị: %w", err)
	}
	defer rows.Close()

	ra := map[string]map[string]domain.Dong{}
	for rows.Next() {
		var khoanMucID, cotID string
		var gia int64
		if err := rows.Scan(&khoanMucID, &cotID, &gia); err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng giá trị: %w", err)
		}
		if ra[khoanMucID] == nil {
			ra[khoanMucID] = map[string]domain.Dong{}
		}
		ra[khoanMucID][cotID] = domain.Dong(gia)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt giá trị: %w", err)
	}
	return ra, nil
}

// --- reads INSIDE a transaction --------------------------------------------------------------------

// BangTheoIDDeSua reads one live sheet inside the transaction and holds it until the transaction
// ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. It is the serialisation point
// for the WHOLE sheet: every write below — a line, a cell, the star — takes this lock first, so two
// members of staff editing one budget board cannot both read the tree, both decide against it, and
// both write. The case that actually costs something is the star: marking a row is "clear every
// other row, then set this one", and two of those interleaved leave TWO marked rows, which is the
// double count `is_headline` exists to prevent and which the database here cannot refuse (migration
// 0006 says why).
func (s *NganSachStore) BangTheoIDDeSua(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.BangNganSach, error) {

	const stmt = `SELECT ` + cotBang + ` FROM bang_ngan_sach ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	b, err := quetBang(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BangNganSach{}, ErrKhongThayBangNganSach
	}
	if err != nil {
		return domain.BangNganSach{}, fmt.Errorf("ngan_sach: đọc bảng để sửa: %w", err)
	}
	return b, nil
}

// BangDayDuTrongGiaoDich reads the whole sheet through the transaction, after BangTheoIDDeSua has
// locked it.
//
// WHY THE WHOLE SHEET FOR EVERY LINE WRITE, when the write touches one row: every decision this
// module makes is about the TREE, not about the row. Whether a line has children (the customer's
// 06/09/2026 rule), what its depth is, whether a parent is in this sheet, whether marking this row
// leaves exactly one marked — none of them can be answered from the row alone, and each answered by
// its own small query would be several round trips whose answers could disagree with one another.
// The specification's own sheet is 59 lines; reading it whole is cheaper than the queries it
// replaces and leaves the decisions in domain, where they are tested without a database.
func (s *NganSachStore) BangDayDuTrongGiaoDich(ctx context.Context, tx *store.ScopedTx,
	bang domain.BangNganSach) (domain.BangDayDu, error) {

	cot, err := s.cotTrongGiaoDich(ctx, tx, bang.ID)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	khoanMuc, err := s.khoanMucTrongGiaoDich(ctx, tx, bang.ID)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	gia, err := s.giaTriTrongGiaoDich(ctx, tx, bang.ID)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	// THE BATCH SUMS ARE READ UNDER THE SAME LOCK AS THE TREE, because the entries -> manual switch
	// copies them into the line's cells (§9.1): a sum read outside the transaction could be one batch
	// behind the figure the screen showed.
	//
	// AN OVERSIZED SUM DOES NOT FAIL THIS READ (quetTongDot): every write reads the sheet here, so a
	// failure would lock them all, GoDot — the remedy — included.
	rows, err := tx.Underlying().QueryContext(ctx, tongDotCuaBang, string(tx.TenantID()), bang.ID,
		string(domain.TinhTheoDot))
	if err != nil {
		return domain.BangDayDu{}, fmt.Errorf("ngan_sach: đọc tổng đợt trong giao dịch: %w", err)
	}
	giaDot, vuot, err := quetTongDot(rows)
	if err != nil {
		return domain.BangDayDu{}, err
	}
	return domain.BangDayDu{Bang: bang, Cot: cot, KhoanMuc: khoanMuc, Gia: gia,
		GiaDot: giaDot, GiaDotVuotMuc: vuot}, nil
}

func (s *NganSachStore) cotTrongGiaoDich(ctx context.Context, tx *store.ScopedTx,
	bangID string) ([]domain.CotNganSach, error) {

	const stmt = `SELECT ` + cotCot + ` FROM cot_ngan_sach ` +
		`WHERE tenant_id = $1 AND deleted_at IS NULL AND bang_id = $2 ORDER BY thu_tu, id`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), bangID)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc cột trong giao dịch: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CotNganSach, 0, 8)
	for rows.Next() {
		var (
			c    domain.CotNganSach
			kieu string
			vai  string
		)
		if err := rows.Scan(&c.ID, &c.BangID, &c.Ten, &c.ThuTu, &kieu, &c.CongThuc, &vai); err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng cột: %w", err)
		}
		c.Kieu = domain.KieuCot(kieu)
		c.VaiTro = domain.VaiTroCot(vai)
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt cột: %w", err)
	}
	return ra, nil
}

func (s *NganSachStore) khoanMucTrongGiaoDich(ctx context.Context, tx *store.ScopedTx,
	bangID string) ([]domain.KhoanMucNganSach, error) {

	const stmt = `SELECT ` + cotKhoanMuc + ` FROM khoan_muc_ngan_sach ` +
		`WHERE tenant_id = $1 AND deleted_at IS NULL AND bang_id = $2 ORDER BY thu_tu, id LIMIT $3`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), bangID,
		TranKhoanMucMotBang+1)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc khoản mục trong giao dịch: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.KhoanMucNganSach, 0, 64)
	for rows.Next() {
		var (
			k        domain.KhoanMucNganSach
			cachTinh string
		)
		err := rows.Scan(&k.ID, &k.BangID, &k.ChaID, &k.TT, &k.Ten, &k.ThuTu,
			&cachTinh, &k.Cap, &k.LaDongTong)
		if err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng khoản mục: %w", err)
		}
		k.CachTinh = domain.CachTinh(cachTinh)
		ra = append(ra, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt khoản mục: %w", err)
	}
	if len(ra) > TranKhoanMucMotBang {
		return nil, ErrQuaNhieuKhoanMuc
	}
	return ra, nil
}

func (s *NganSachStore) giaTriTrongGiaoDich(ctx context.Context, tx *store.ScopedTx,
	bangID string) (map[string]map[string]domain.Dong, error) {

	const stmt = `SELECT g.khoan_muc_id, g.cot_id, g.gia_tri
		FROM gia_tri_khoan_muc g
		JOIN khoan_muc_ngan_sach k
		  ON k.tenant_id = g.tenant_id AND k.id = g.khoan_muc_id
		WHERE g.tenant_id = $1 AND k.bang_id = $2
		  AND k.deleted_at IS NULL AND g.gia_tri IS NOT NULL`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), bangID)
	if err != nil {
		return nil, fmt.Errorf("ngan_sach: đọc giá trị trong giao dịch: %w", err)
	}
	defer rows.Close()

	ra := map[string]map[string]domain.Dong{}
	for rows.Next() {
		var khoanMucID, cotID string
		var gia int64
		if err := rows.Scan(&khoanMucID, &cotID, &gia); err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng giá trị: %w", err)
		}
		if ra[khoanMucID] == nil {
			ra[khoanMucID] = map[string]domain.Dong{}
		}
		ra[khoanMucID][cotID] = domain.Dong(gia)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt giá trị: %w", err)
	}
	return ra, nil
}

// LanKeTiep is the revision number a new sheet for this year and kind gets.
//
// IT COUNTS SOFT-DELETED ROWS ON PURPOSE — the ONE read in this file that does, and the reason is
// rule 7, invariant 3. `ma` is `NS-2026-CHI-<lan>` and a code that has been issued is never
// reissued; restricting this to live rows would hand revision 1 back out after a `🗑 Gỡ`, and two
// sheets would then carry one code in a table where the old one is still there.
//
// IT MUST RUN INSIDE THE TRANSACTION THAT INSERTS, or two concurrent creations both read the same
// MAX and both write it. `UNIQUE (tenant_id, nam, loai, lan)` is the floor that refuses the second
// one whatever happens here.
func (s *NganSachStore) LanKeTiep(ctx context.Context, tx *store.ScopedTx,
	nam int, loai domain.LoaiBang) (int, error) {

	const stmt = `SELECT COALESCE(MAX(lan), 0) + 1 FROM bang_ngan_sach
		WHERE tenant_id = $1 AND nam = $2 AND loai = $3`

	var lan int
	err := tx.Underlying().QueryRowContext(ctx, stmt,
		string(tx.TenantID()), nam, string(loai)).Scan(&lan)
	if err != nil {
		return 0, fmt.Errorf("ngan_sach: đọc lần kế tiếp: %w", err)
	}
	return lan, nil
}

// CoBangConSong reports whether the commune already has a LIVE sheet for this year and kind, inside
// the transaction that is about to create one.
func (s *NganSachStore) CoBangConSong(ctx context.Context, tx *store.ScopedTx,
	nam int, loai domain.LoaiBang) (bool, error) {

	const stmt = `SELECT EXISTS (SELECT 1 FROM bang_ngan_sach
		WHERE tenant_id = $1 AND nam = $2 AND loai = $3 AND deleted_at IS NULL)`

	var co bool
	err := tx.Underlying().QueryRowContext(ctx, stmt,
		string(tx.TenantID()), nam, string(loai)).Scan(&co)
	if err != nil {
		return false, fmt.Errorf("ngan_sach: kiểm bảng còn sống: %w", err)
	}
	return co, nil
}

// --- writes ----------------------------------------------------------------------------------------

const chenBang = `INSERT INTO bang_ngan_sach
	(tenant_id, id, ma, nam, loai, lan, tieu_de, don_vi_tinh, luy_ke_den, nguon_tep, nap_luc)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

// ChenBang adds one sheet.
func (s *NganSachStore) ChenBang(ctx context.Context, tx *store.ScopedTx,
	b domain.BangNganSach) error {

	_, err := tx.Exec(ctx, chenBang, string(tx.TenantID()),
		b.ID, b.Ma, b.Nam, string(b.Loai), b.Lan, b.TieuDe, b.DonViTinh,
		ngayHoacNil(b.LuyKeDen), rongThanhNil(b.NguonTep), ngayHoacNil(b.NapLuc))
	if err != nil {
		return fmt.Errorf("ngan_sach: chèn bảng: %w", err)
	}
	return nil
}

// chenCot — `vai_tro` AND `cong_thuc` GO IN AS NULL WHEN EMPTY, not as ”. The uniqueness of a role
// per sheet is `UNIQUE (tenant_id, bang_id, vai_tro)`, and SQL lets any number of NULLs coexist
// while two ordinary columns both storing ” would COLLIDE — every sheet would then be limited to
// one column without a role, which is every sheet.
const chenCot = `INSERT INTO cot_ngan_sach
	(tenant_id, id, bang_id, ten, thu_tu, kieu, cong_thuc, vai_tro)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

func (s *NganSachStore) ChenCot(ctx context.Context, tx *store.ScopedTx,
	c domain.CotNganSach) error {

	_, err := tx.Exec(ctx, chenCot, string(tx.TenantID()),
		c.ID, c.BangID, c.Ten, c.ThuTu, string(c.Kieu),
		rongThanhNil(c.CongThuc), rongThanhNil(string(c.VaiTro)))
	if err != nil {
		return fmt.Errorf("ngan_sach: chèn cột: %w", err)
	}
	return nil
}

// xoaMemBang writes all THREE columns rule 7, invariant 1 names, in one statement, so none of them
// can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND REMOVAL A 404 rather than a silent rewrite of who
// removed the sheet and why. The first removal is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
//
// THE LINES AND CELLS ARE NOT TOUCHED, and that is deliberate rather than lazy: every read path in
// this file reaches them THROUGH the sheet, so a removed sheet takes its whole tree off every
// screen and out of every total without a single row being destroyed. Stamping `deleted_at` on
// 59 lines as well would be 59 more writes recording the same one act, and the act is recorded once,
// in `audit_log`.
const xoaMemBang = `UPDATE bang_ngan_sach
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *NganSachStore) XoaMemBang(ctx context.Context, tx *store.ScopedTx,
	id, boi, lyDo string) error {

	kq, err := tx.Exec(ctx, xoaMemBang, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("ngan_sach: xoá mềm bảng: %w", err)
	}
	return doiMotDongNganSach(kq, "xoá mềm bảng", ErrKhongThayBangNganSach)
}

// capNhatBang writes the three editable fields of a sheet's header.
//
// `nam`, `loai`, `lan`, `ma`, `nguon_tep` AND `nap_luc` APPEAR NOWHERE IN THIS STATEMENT. The first
// four identify which report the figures belong to and the code the audit trail is filed under;
// the last two record an import that happened. None of them is an edit.
//
// `AND deleted_at IS NULL` MAKES A REMOVED SHEET A 404 rather than a silent rewrite of a record
// that is off every screen (rule 7, forbidden #5).
const capNhatBang = `UPDATE bang_ngan_sach
	SET tieu_de = $3, don_vi_tinh = $4, luy_ke_den = $5, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhatBang updates one sheet's title, display unit and cut-off date.
func (s *NganSachStore) CapNhatBang(ctx context.Context, tx *store.ScopedTx,
	b domain.BangNganSach) error {

	kq, err := tx.Exec(ctx, capNhatBang, string(tx.TenantID()),
		b.ID, b.TieuDe, b.DonViTinh, ngayHoacNil(b.LuyKeDen))
	if err != nil {
		return fmt.Errorf("ngan_sach: cập nhật bảng: %w", err)
	}
	return doiMotDongNganSach(kq, "cập nhật bảng", ErrKhongThayBangNganSach)
}

// chenKhoanMuc — `is_headline` IS ABSENT AND TAKES ITS DEFAULT false.
//
// Read that as the property it is, not as a shortcut. A new line is not the commune's total until
// somebody stars it: there is no $n for the flag, so there is no value a handler could pass and no
// field a client could fill. Turning it into a parameter is the one edit that would let a client
// create a second marked row — which is the double count migration 0006 was written to end, and
// which the database here cannot refuse.
//
// `cach_tinh` AND `cap` ARE PARAMETERS BUT ARE NEVER CLIENT-SUPPLIED: internal/app derives both
// from the tree (domain.CachTinhTheoCay, domain.CapTheoCha) and refuses a request body that names
// either.
const chenKhoanMuc = `INSERT INTO khoan_muc_ngan_sach
	(tenant_id, id, bang_id, cha_id, tt, ten, thu_tu, cach_tinh, cap)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

func (s *NganSachStore) ChenKhoanMuc(ctx context.Context, tx *store.ScopedTx,
	k domain.KhoanMucNganSach) error {

	_, err := tx.Exec(ctx, chenKhoanMuc, string(tx.TenantID()),
		k.ID, k.BangID, rongThanhNil(k.ChaID), k.TT, k.Ten, k.ThuTu, string(k.CachTinh), k.Cap)
	if err != nil {
		return fmt.Errorf("ngan_sach: chèn khoản mục: %w", err)
	}
	return nil
}

// capNhatKhoanMuc — `bang_id`, `cha_id`, `is_headline` AND `cap` APPEAR NOWHERE IN THIS STATEMENT.
//
// `bang_id` and `cha_id` because moving a line to another sheet or under another parent moves its
// figure between two totals that have already been read off a screen, leaving ONE entry that reads
// as an ordinary edit. The operation for a line in the wrong place is to remove it with a reason and
// enter it again — two events, both audited.
//
// `is_headline` because the star has its own statement pair (DatDongTong), which is what keeps "at
// most one marked row" true without a database constraint.
//
// `cap` because it is derived from the parent, which this statement cannot change.
const capNhatKhoanMuc = `UPDATE khoan_muc_ngan_sach
	SET tt = $3, ten = $4, thu_tu = $5, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *NganSachStore) CapNhatKhoanMuc(ctx context.Context, tx *store.ScopedTx,
	k domain.KhoanMucNganSach) error {

	kq, err := tx.Exec(ctx, capNhatKhoanMuc, string(tx.TenantID()), k.ID, k.TT, k.Ten, k.ThuTu)
	if err != nil {
		return fmt.Errorf("ngan_sach: cập nhật khoản mục: %w", err)
	}
	return doiMotDongNganSach(kq, "cập nhật khoản mục", domain.ErrKhongThayKhoanMuc)
}

// datCachTinh moves one line between `manual` and `children`.
//
// ITS OWN STATEMENT AND NOT PART OF THE EDIT ABOVE, because it is never something a client asks
// for: it follows from the line gaining its first child or losing its last (domain.CachTinhTheoCay).
// Merged into capNhatKhoanMuc it would become a field on the edit path, and a field on the edit path
// is a field a request body can eventually fill — which is a parent marked `manual`, the exact row
// the customer's 06/09/2026 rule says cannot exist.
const datCachTinh = `UPDATE khoan_muc_ngan_sach
	SET cach_tinh = $3, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *NganSachStore) DatCachTinh(ctx context.Context, tx *store.ScopedTx,
	id string, ct domain.CachTinh) error {

	kq, err := tx.Exec(ctx, datCachTinh, string(tx.TenantID()), id, string(ct))
	if err != nil {
		return fmt.Errorf("ngan_sach: đặt cách tính: %w", err)
	}
	return doiMotDongNganSach(kq, "đặt cách tính", domain.ErrKhongThayKhoanMuc)
}

// boDanhDauDongTongKhac and datDongTong are the star, and they are TWO STATEMENTS ON PURPOSE.
//
// THE MARK IS A RADIO, NOT A TOGGLE — §4.1: "Bấm ngôi sao ở đầu một dòng khác để đổi". Clearing
// every other row of the sheet first is what makes "at most one marked row" hold, and it has to be
// in the SAME transaction as the set: between them the sheet has no total at all, which is a state
// no reader may see.
//
// WHY THIS IS THE PROPERTY AND NOT A DATABASE CONSTRAINT: migration 0006 sets out why a partial
// unique index cannot be verified in this build environment. So these two statements, under the
// sheet's FOR UPDATE lock, are the enforcement — and the read path refuses to produce a total when
// it finds more than one anyway, because this is not the only writer the table will ever have.
const boDanhDauDongTongKhac = `UPDATE khoan_muc_ngan_sach
	SET is_headline = false, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND bang_id = $2 AND id <> $3 AND is_headline`

const datDongTong = `UPDATE khoan_muc_ngan_sach
	SET is_headline = true, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// DatDongTong marks one line as the sheet's total row and unmarks every other line of that sheet.
func (s *NganSachStore) DatDongTong(ctx context.Context, tx *store.ScopedTx,
	bangID, id string) error {

	if _, err := tx.Exec(ctx, boDanhDauDongTongKhac, string(tx.TenantID()), bangID, id); err != nil {
		return fmt.Errorf("ngan_sach: bỏ đánh dấu dòng tổng cũ: %w", err)
	}
	kq, err := tx.Exec(ctx, datDongTong, string(tx.TenantID()), id)
	if err != nil {
		return fmt.Errorf("ngan_sach: đánh dấu dòng tổng: %w", err)
	}
	return doiMotDongNganSach(kq, "đánh dấu dòng tổng", domain.ErrKhongThayKhoanMuc)
}

// xoaMemKhoanMuc writes rule 7 invariant 1's three columns AND CLEARS `is_headline` in the same
// statement.
//
// THE `is_headline = false` IS NOT TIDINESS. A removed line is off every read path, so a sheet whose
// marked row was removed would have no total — correctly — but the mark would still be sitting on a
// dead row. Two things then break at once: the commune cannot see why its total vanished, and a
// future "at most one marked row" constraint would find the slot permanently occupied by a row
// nobody can reach. Releasing it here is what lets the commune star another line and get its figure
// back.
const xoaMemKhoanMuc = `UPDATE khoan_muc_ngan_sach
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4,
	    is_headline = false, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *NganSachStore) XoaMemKhoanMuc(ctx context.Context, tx *store.ScopedTx,
	id, boi, lyDo string) error {

	kq, err := tx.Exec(ctx, xoaMemKhoanMuc, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("ngan_sach: xoá mềm khoản mục: %w", err)
	}
	return doiMotDongNganSach(kq, "xoá mềm khoản mục", domain.ErrKhongThayKhoanMuc)
}

// ghiGiaTri writes one cell, creating it if it is not there.
//
// ON CONFLICT DO UPDATE AND NOT "read then INSERT or UPDATE": under the sheet's row lock both would
// be correct today, and the upsert stays correct on the day somebody removes the lock. It is also
// one round trip instead of two for the operation an accountant performs several hundred times in
// an afternoon.
//
// `gia_tri` MAY BE NULL AND THAT IS HOW A CELL IS CLEARED. There is no DELETE here and there must
// not be one: `ho_so_luu_tru_cam_xoa_cung` refuses a hard delete on this table, and §9 rule 4 makes
// the empty cell a STATE the screen draws as `—` rather than the absence of a record.
const ghiGiaTri = `INSERT INTO gia_tri_khoan_muc (tenant_id, khoan_muc_id, cot_id, gia_tri)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (tenant_id, khoan_muc_id, cot_id)
	DO UPDATE SET gia_tri = EXCLUDED.gia_tri, cap_nhat_luc = now()`

// GhiGiaTri writes one cell. A nil `gia` clears it.
func (s *NganSachStore) GhiGiaTri(ctx context.Context, tx *store.ScopedTx,
	khoanMucID, cotID string, gia *domain.Dong) error {

	var v any
	if gia != nil {
		v = int64(*gia)
	}
	if _, err := tx.Exec(ctx, ghiGiaTri, string(tx.TenantID()), khoanMucID, cotID, v); err != nil {
		return fmt.Errorf("ngan_sach: ghi giá trị: %w", err)
	}
	return nil
}

// doiMotDongNganSach turns "nothing was updated" into the caller's own not-found error.
//
// IT TAKES THE ERROR RATHER THAN RETURNING ONE, unlike doiMotDongChungTu next door, because this
// file writes to three different tables and a single sentence would tell somebody removing a budget
// line that a sheet could not be found.
func doiMotDongNganSach(kq sql.Result, viec string, khongThay error) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("ngan_sach: %s: đọc số dòng: %w", viec, err)
	}
	if n == 0 {
		return khongThay
	}
	return nil
}

// ngayHoacNil writes a zero time as NULL.
//
// WHY IT MATTERS ON THESE TWO COLUMNS SPECIFICALLY: `luy_ke_den` and `nap_luc` are both genuinely
// absent on an ordinary sheet — a commune that has not stated its cut-off date, a sheet typed in by
// hand rather than loaded from Excel. Writing Go's zero time instead would store the year 1 as a
// fact, and `bang_ngan_sach_nguon_tep_du` would then accept a sheet claiming to have been imported
// at an instant that is not a date anybody could have imported at.
func ngayHoacNil(t interface{ IsZero() bool }) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t
}

// BangIDCuaKhoanMuc answers which sheet one live line belongs to, inside the transaction.
//
// IT EXISTS BECAUSE THE LOCK IS ON THE SHEET AND THE REQUEST NAMES A LINE. Every line write locks
// the whole sheet (BangTheoIDDeSua) so that the tree a decision is made against cannot move under
// it — but the caller only has a line id, and `FOR UPDATE` cannot be taken through a join without
// naming which table it applies to. So: one unlocked read to learn the sheet, then the lock, then
// the tree is read AGAIN under it. The unlocked read can be stale in exactly one way — the line was
// removed in between — and that is caught by the line not being in the locked tree, which answers
// "not found", which is the truth.
func (s *NganSachStore) BangIDCuaKhoanMuc(ctx context.Context, tx *store.ScopedTx,
	khoanMucID string) (string, error) {

	const stmt = `SELECT bang_id FROM khoan_muc_ngan_sach
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var bangID string
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), khoanMucID).Scan(&bangID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrKhongThayKhoanMuc
	}
	if err != nil {
		return "", fmt.Errorf("ngan_sach: đọc bảng của khoản mục: %w", err)
	}
	return bangID, nil
}
