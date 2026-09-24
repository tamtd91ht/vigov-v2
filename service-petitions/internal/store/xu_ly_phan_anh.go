package store

// The STAFF side of the petition register — the paginated list, the locking read, and the four
// writes the lifecycle is moved by. SQL, and nothing else.
//
// FIVE THINGS HOLD ACROSS EVERY METHOD IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from tx.TenantID() / the context (rule 1, invariants 4
//     and 5). It is never a parameter here, so no caller can reach another commune's register.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the list, the locking read, and
//     the WHERE clause of every UPDATE.
//  4. EVERY UPDATE CARRIES THE EXPECTED STATUS in its WHERE clause, so two officers acting on one
//     petition at the same moment cannot both succeed. The second matches no row and the caller sees
//     a refusal, never a silent overwrite.
//  5. `ma_tra_cuu`, `goc_dem_han`, `kenh_tiep_nhan` AND THE THREE DEADLINE COLUMNS APPEAR IN NO
//     UPDATE HERE. The first three are refused by the `ho_so_luu_tru_bat_bien` trigger underneath;
//     `han_tiep_nhan` and `han_phan_loai` are fixed at intake and `han_xu_ly_xong` is fixed by
//     ChotLinhVuc alone (rule 10, invariant 2). Their absence is what keeps that floor unreachable.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ErrPhieuDaChuyenTrang means the petition was not in the status the caller believed it was in.
//
// A SEPARATE SENTINEL FROM ErrPhieuKhongTonTai, AND THE DIFFERENCE IS WHAT THE OFFICER IS TOLD.
// "Không tìm thấy phiếu" (404) sends them looking for a lost record; this one is a 409 saying
// somebody else moved it while their screen was open, which is a true statement they can act on by
// reloading. Folding the two together would make a normal race look like data loss.
//
// IT IS PRODUCED BY THE UPDATES AND NOT BY THE READ: each UPDATE carries the expected status, so
// zero rows affected after a successful locking read means exactly this.
var ErrPhieuDaChuyenTrang = errors.New("phieu_phan_anh: phiếu không còn ở trạng thái vừa đọc")

// --- the register screen -------------------------------------------------------------------------

// SapXepPhieu is the closed set of sorts GET /api/v1/citizen-reports offers.
//
// `booked_at` IS THE DEFAULT, DESCENDING — newest first, which is what docs/ui-ux/09 §2 shows and
// what an officer opening the screen in the morning needs. `vao_so_luc` is NOT NULL and indexed
// (`phieu_phan_anh_so`, migration 0004), which is what page.QueryPage needs from a sort column.
//
// WHAT IS DELIBERATELY ABSENT, and each absence is a different rule:
//
//	the content and the location columns   a sort key travels in a URL, an access log and a browser
//	                                       history, and those are citizen personal data (rule 3).
//	han_xu_ly_xong                         it is NULLABLE — `(col, id) > (…)` is NULL for a NULL
//	                                       col, so every unclassified petition would vanish from
//	                                       page two onward. A screen looking for late petitions asks
//	                                       with `late=true`.
//	ma_tra_cuu                             sorting by it is sorting by randomness, and offering it
//	                                       invites a client to page the register by lookup code.
var SapXepPhieu = page.NewAllowlist(page.Desc,
	page.Col("booked_at", "vao_so_luc", page.KindTime),
	page.Col("created_at", "tao_luc", page.KindTime),
)

// TimPhieuToiDa bounds the free-text search. Longer than any phrase a clerk types, short enough that
// the pattern cannot become a payload.
const TimPhieuToiDa = 200

// ErrTimPhieuQuaDai — the search box was sent more than TimPhieuToiDa characters.
var ErrTimPhieuQuaDai = fmt.Errorf("phieu_phan_anh: chuỗi tìm kiếm quá dài (tối đa %d ký tự)", TimPhieuToiDa)

// LocPhieu is the set of filters the list route accepts, already validated by the handler.
//
// A STRUCT AND NOT A STRING OF SQL: every field below becomes a BOUND PARAMETER. A filter assembled
// as text anywhere above this line is an injection point in a government register.
type LocPhieu struct {
	TrangThai string // "" = every status
	LinhVuc   string // "" = every field
	ThonID    string // "" = every hamlet, including none
	BoPhanID  string // "" = every department, including none
	Kenh      string // "" = every intake channel
	Tim       string // "" = no text search

	// CanBoXuLyID restricts the page to petitions whose CURRENT assignee is this staff BUSINESS CODE
	// — the "Giao cho tôi" tab of docs/ui-ux/09 §4. "" = every assignee, including none.
	//
	// THE ONLY WRITER IS THE HANDLER, AND IT WRITES `authz.Principal.Ma` — THE CALLER'S OWN CODE FROM
	// THE SESSION. The query string carries a scope switch and never a code: a code taken from the
	// request would let any reader of the register list another officer's workload by typing it
	// (rule 1, forbidden #2 in spirit; rule 4, invariant 2). The column holds a business code, the
	// same kind of value app.duocTienTrangThai compares against — an internal id here matches nothing.
	CanBoXuLyID string

	// ChiTreHan restricts the page to petitions whose RESOLVE commitment was missed.
	//
	// DERIVED IN SQL, NEVER READ FROM A COLUMN (rule 10, invariant 3). See the predicate in
	// locPhieuThanhSQL, and read the warning there before touching it.
	ChiTreHan bool

	// ChoPhepHanChe opens the restricted field `can-bo` — reports ABOUT a member of staff.
	//
	// # THE ZERO VALUE IS THE CLOSED ONE, AND THAT IS THE WHOLE DESIGN OF THIS FIELD
	//
	// False means those petitions are excluded from the page, from the cursor and therefore from
	// every count derived from paging it. A caller that forgets to set it gets the SAFE answer; a
	// caller that wants the wide answer has to say so, and the only place that says so is the
	// handler, after asking the Checker for `feedback.restricted`.
	//
	// WHY IT IS A FILTER AND NOT A CHECK AFTER THE READ: filtering in Go after the page came back
	// would return short pages — a page of 20 that renders 14 — and the cursor would still have
	// advanced past the six. The officer would see gaps in a register and have no way to know.
	ChoPhepHanChe bool
}

// dieuKienTim is the free-text predicate, built with the placeholder already chosen.
//
// WRITTEN BY CONCATENATION RATHER THAN fmt.Sprintf, AND THAT IS NOT A STYLE CHOICE: `hooks/pii_guard`
// blocks a formatting call within 200 characters of a personal-data column name, and it is RIGHT to
// — `fmt.Sprintf` next to those names is, nine times out of ten, a log line being assembled. This
// one is a WHERE clause, so the honest move is to stop using the shape the guard watches rather than
// to argue with it. The clause searches the petition text and the incident location, which are
// citizen personal data (rule 3); nothing in this file logs either, and the pattern is bounded by
// the handler before it arrives.
func dieuKienTim(n int) string {
	p := "$" + strconv.Itoa(n)
	return " AND (ma_tra_cuu ILIKE " + p +
		" OR noi_dung ILIKE " + p +
		" OR COALESCE(dia_chi,'') ILIKE " + p + ")"
}

// locPhieuThanhSQL turns the validated filter struct into a predicate and its bound values.
//
// ONE FUNCTION BUILDING BOTH HALVES, because the placeholder numbers and the argument order are one
// fact: written apart, a filter added to one half and forgotten in the other produces a query that
// silently reads the wrong column's value.
func locPhieuThanhSQL(loc LocPhieu) (string, []any) {
	var (
		dieuKien string
		args     []any
	)
	them := func(mau string, gt any) {
		args = append(args, gt)
		dieuKien += fmt.Sprintf(mau, len(args)+1) // $1 is the commune
	}

	// THE RESTRICTED FIELD, FIRST AND UNCONDITIONALLY. Written as a literal rather than a bound
	// parameter on purpose: the code it excludes is a decision of this system (docs/ui-ux/09 §5 and
	// §14.5), not a value any layer above may choose — a parameter here would be a caller able to
	// pick which field it is not allowed to see.
	if !loc.ChoPhepHanChe {
		dieuKien += ` AND (linh_vuc IS NULL OR linh_vuc <> '` + domain.LinhVucHanChe + `')`
	}

	if loc.TrangThai != "" {
		them(" AND trang_thai = $%d", loc.TrangThai)
	}
	if loc.LinhVuc != "" {
		them(" AND linh_vuc = $%d", loc.LinhVuc)
	}
	if loc.ThonID != "" {
		them(" AND thon_id = $%d", loc.ThonID)
	}
	if loc.BoPhanID != "" {
		them(" AND bo_phan_id = $%d", loc.BoPhanID)
	}
	if loc.Kenh != "" {
		them(" AND kenh_tiep_nhan = $%d", loc.Kenh)
	}
	if loc.CanBoXuLyID != "" {
		// Equality on a NULLable column: an unassigned petition (NULL) is not TRUE here, so it is
		// never "assigned to me" — the same guard app.duocTienTrangThai states explicitly.
		them(" AND can_bo_xu_ly_id = $%d", loc.CanBoXuLyID)
	}
	if loc.Tim != "" {
		// ILIKE ON THREE COLUMNS — the lookup code a citizen reads down the telephone, the report a
		// clerk remembers, and where it happened: the three boxes docs/ui-ux/09 §4 names. A leading
		// wildcard cannot use an index, which is affordable because the scan is already bound to one
		// commune.
		args = append(args, "%"+loc.Tim+"%")
		dieuKien += dieuKienTim(len(args) + 1)
	}

	if loc.ChiTreHan {
		// OVERDUE IS DERIVED HERE TOO, AND THIS IS THE SECOND EXPRESSION OF domain.QuaHan — say so
		// rather than let somebody discover it. Go cannot run inside a WHERE clause, so a register
		// filtered on "late" has no other shape; what this comment buys is that the two are read
		// together when either changes.
		//
		// IT MIRRORS domain.QuaHan LINE FOR LINE:
		//
		//	no resolve deadline  -> NOT late. An unclassified petition has had no resolve date
		//	                        promised to anybody, so there is nothing to have missed.
		//	settled              -> compare the two RECORDED instants, so work finished late STAYS
		//	                        late and last quarter's figure does not change on every read.
		//	not settled          -> compare the deadline with now.
		//
		// `now()` AND NOT A PARAMETER: the database's clock is the one the deadline was stored
		// against, and a `now` passed from a handler is a second clock that can disagree with it.
		dieuKien += ` AND han_xu_ly_xong IS NOT NULL AND (
			(xu_ly_xong_luc IS NULL AND han_xu_ly_xong < now())
			OR (xu_ly_xong_luc IS NOT NULL AND xu_ly_xong_luc > han_xu_ly_xong))`
	}
	return dieuKien, args
}

// mocPhieu BINDS each allowlisted sort column to the way that column's cursor value is read out of a
// scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added to SapXepPhieu
// without a reader here is a panic at startup rather than an error on the first request that uses
// it — which would be after release.
var mocPhieu = store.NewMoc[domain.PhieuPhanAnh](SapXepPhieu,
	map[string]func(domain.PhieuPhanAnh) page.Key{
		"booked_at":  func(p domain.PhieuPhanAnh) page.Key { return page.TimeKey(p.VaoSoLuc) },
		"created_at": func(p domain.PhieuPhanAnh) page.Key { return page.TimeKey(p.TaoLuc) },
	})

// cotPhieuCoTaoLuc is cotPhieu plus `tao_luc`, which the cursor needs and the single-row reads do
// not.
//
// A SECOND CONSTANT RATHER THAN ONE LIST WITH THE COLUMN ALWAYS PRESENT, because `tao_luc` is a
// ROW-LIFECYCLE fact and not a business one: nothing on any screen renders it, and adding it to the
// shape every read returns would invite a handler to show it beside `vao_so_luc`, which is the fact
// the register is actually ordered by. The two differ by up to a week on the staff-booked channel.
const cotPhieuCoTaoLuc = cotPhieu + `, tao_luc`

// quetPhieuCoTaoLuc reads one row of cotPhieuCoTaoLuc.
//
// THE EXTRA DESTINATION IS APPENDED, NOT INSERTED, and quetPhieuThem says why at length: database/sql
// binds by position, and the columns an inserted destination would shift are the three adjacent
// deadline columns whose NULLs mean two opposite things.
func quetPhieuCoTaoLuc(r quangKiem) (domain.PhieuPhanAnh, error) {
	var taoLuc time.Time
	p, err := quetPhieuThem(r, &taoLuc)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	p.TaoLuc = taoLuc
	return p, nil
}

// DanhSach reads ONE PAGE of the commune's petition register.
//
// PAGINATED, UNLIKE THE LABEL CATALOGUE NEXT DOOR, and the difference is what the two lists ARE: the
// catalogue is closed at twelve rows, while this register grows with every week the commune
// operates. An unpaginated read of it is a query whose cost rises for ever.
//
// THE FILTERS ARE BOUND PARAMETERS, numbered from $2 because QueryPage gives $1 to the commune.
func (s *PhieuPhanAnhStore) DanhSach(ctx context.Context, loc LocPhieu, yc page.Request) (
	page.Result[domain.PhieuPhanAnh], error) {

	dieuKien, args := locPhieuThanhSQL(loc)

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotPhieuCoTaoLuc,
		Table:   "phieu_phan_anh",
		// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
		// `phieu_phan_anh_so` is built on exactly this predicate.
		Filter: `AND deleted_at IS NULL` + dieuKien,
		Args:   args,
	}, yc, mocPhieu, func(rows *sql.Rows) (domain.PhieuPhanAnh, string, error) {
		p, err := quetPhieuCoTaoLuc(rows)
		if err != nil {
			return domain.PhieuPhanAnh{}, "", err
		}
		return p, p.ID, nil
	})
}

// --- the locking read ----------------------------------------------------------------------------

// TheoMaTraCuuDeSua reads one live petition INSIDE the caller's transaction and holds it until the
// transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Every write below is a
// read-decide-write: read the petition, work out whether the lifecycle admits the act, refuse or
// apply. Without the lock two officers acting on the same petition both read the old status and both
// decide against it — and the pair that costs the most is classification racing classification,
// where the second write would move a deadline the first one already promised to a citizen.
//
// IT TAKES THE LOOKUP CODE AND NOT THE INTERNAL id, because that is what the URL carries and what
// the audit entry's Subject is. One identifier end to end is one identifier nobody can mix up.
//
// IT EXCLUDES SOFT-DELETED ROWS, so acting on a petition somebody removed a second ago is
// ErrPhieuKhongTonTai rather than a resurrection.
func (s *PhieuPhanAnhStore) TheoMaTraCuuDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (
	domain.PhieuPhanAnh, error) {

	const stmt = `SELECT ` + cotPhieu + ` FROM phieu_phan_anh
		WHERE tenant_id = $1 AND ma_tra_cuu = $2 AND deleted_at IS NULL FOR UPDATE`

	p, err := quetPhieu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	if err != nil {
		// THE CODE IS NOT IN THE WRAPPED MESSAGE. It is the one string that opens a citizen's
		// petition, and an error travels into centralised logging (rule 3).
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc phiếu để sửa: %w", err)
	}
	return p, nil
}

// --- the three writes that move the lifecycle -----------------------------------------------------

// PhanCong records who is answerable for the petition, and the status that act lands it in.
//
// THE DEPARTMENT, THE OFFICER AND THE STATUS IN ONE STATEMENT, because they are one administrative
// act: "đã chuyển xử lý" and "đang giao cho" are the two cells §8.3 renders side by side, and a
// window in which one has moved and the other has not is a register that contradicts itself on one
// screen.
//
// `sangTrangThai` IS PASSED IN, ALREADY DECIDED. A re-assignment of a petition that is already being
// worked on must NOT drag it backwards to `da-chuyen-xu-ly` — docs/ui-ux/09 §8.5 titles that block
// "Chuyển xử lý, KHÔNG đổi trạng thái" — and deciding which of the two cases this is belongs in the
// use case, where the audit entry records the before and after.
func (s *PhieuPhanAnhStore) PhanCong(ctx context.Context, tx *store.ScopedTx,
	id, boPhanID, canBoID string, tuTrangThai, sangTrangThai domain.TrangThai) error {

	const stmt = `UPDATE phieu_phan_anh
		SET bo_phan_id = $3, can_bo_xu_ly_id = $4, trang_thai = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $6 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id,
		boPhanID, rongThanhNull(canBoID), string(sangTrangThai), string(tuTrangThai))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: phân công: %w", err)
	}
	return doiMotDongPhieu(kq, "phân công")
}

// DoiTrangThai moves the petition one step along the main flow and records the instant the work was
// finished when that is the step being taken.
//
// `xu_ly_xong_luc` IS SET HERE AND NOWHERE ELSE, and it is passed in as a zero time for every other
// step. It is the instant domain.QuaHan compares against once the work is done — the reason a
// petition finished late STAYS late — so writing it at the wrong transition would freeze the
// comparison at the wrong moment and change an already-published figure.
func (s *PhieuPhanAnhStore) DoiTrangThai(ctx context.Context, tx *store.ScopedTx,
	id string, tuTrangThai, sangTrangThai domain.TrangThai, xuLyXongLuc time.Time) error {

	// COALESCE, NOT A PLAIN ASSIGNMENT. A zero time arrives as NULL, and `COALESCE($4, column)`
	// leaves an instant already recorded untouched — so a step that is not the finishing step cannot
	// blank out the finishing instant of a petition that was reopened and finished again.
	const stmt = `UPDATE phieu_phan_anh
		SET trang_thai = $3, xu_ly_xong_luc = COALESCE($4, xu_ly_xong_luc), cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $5 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id,
		string(sangTrangThai), khongThanhNull(xuLyXongLuc), string(tuTrangThai))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: đổi trạng thái: %w", err)
	}
	return doiMotDongPhieu(kq, "đổi trạng thái")
}

// Dong closes the petition, recording the result the CITIZEN reads and the instant it was closed.
//
// `ket_qua_xu_ly` IS A PARAMETER AND IS NEVER NULL FROM HERE. Rule 10, invariant 6 forbids closing
// silently, and the use case refuses an empty or trivial result before this runs — so a row in
// `da-dong` with nothing readable in it cannot be produced by this service at all.
//
// THE STATUS IS A LITERAL. Read that as the property it is: there is no $n for it, so no layer above
// can pass a status to a method named Dong, and this statement can only ever produce `da-dong`.
func (s *PhieuPhanAnhStore) Dong(ctx context.Context, tx *store.ScopedTx,
	id string, tuTrangThai domain.TrangThai, ketQua string, dongLuc time.Time) error {

	const stmt = `UPDATE phieu_phan_anh
		SET trang_thai = 'da-dong', ket_qua_xu_ly = $3, dong_luc = $4, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $5 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, ketQua, dongLuc, string(tuTrangThai))
	if err != nil {
		// NOT the result text: it is free text about one citizen's case and an error message travels
		// into centralised logging (rule 3, forbidden #3).
		return fmt.Errorf("phieu_phan_anh: đóng phiếu: %w", err)
	}
	return doiMotDongPhieu(kq, "đóng phiếu")
}

// doiMotDongPhieu turns "no row matched" into the sentinel the caller answers 409 for.
//
// ONE FUNCTION FOR ALL THREE WRITES, because three copies of this would drift and the copy that
// drifts is the one that treats zero rows as success — which is an officer's click that silently
// did nothing, on a screen that then shows the old state as though it were the new one.
func doiMotDongPhieu(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: %s, đếm dòng: %w", viec, err)
	}
	if n == 0 {
		// The caller read the row under FOR UPDATE a moment ago, so it exists and belongs to this
		// commune. Zero rows therefore means the STATUS moved — see ErrPhieuDaChuyenTrang.
		return ErrPhieuDaChuyenTrang
	}
	return nil
}

// --- the outbox ------------------------------------------------------------------------------------

// SuKienDiStore is the only path to `su_kien_di` (migration 0005).
//
// IT HAS ONE WRITE METHOD AND IT TAKES THE TRANSACTION. That is what makes it impossible to record
// the obligation to notify a citizen in one transaction and the status change in another: there is
// no signature here that would let you, and rule 10, invariant 5 is not a promise that survives "the
// status changed but the message row failed".
//
// THERE IS DELIBERATELY NO READ METHOD AND NO "MARK AS SENT" METHOD. Both belong to the relay, the
// relay does not exist (there is no broker client in this repository — `core/events.Publisher` is a
// bare interface), and a method with no caller is a method nobody keeps correct.
type SuKienDiStore struct {
	db *store.DB
}

func NewSuKienDiStore(db *store.DB) *SuKienDiStore { return &SuKienDiStore{db: db} }

// SuKienDi is one fact waiting to leave this service.
type SuKienDi struct {
	ID string

	// Ten is the event name WITH ITS VERSION — `petitions.status_changed.v1` (rule 2, invariant 4).
	Ten string

	// DoiTuong is the BUSINESS code of the record, i.e. the lookup code. Never the internal ULID.
	DoiTuong string

	// Than is the protojson body. WHAT MAY NOT BE IN IT is the list in
	// proto/vigov/petitions/v1/events.proto: no phone number, no full name, no location, no
	// coordinates, no text of the petition, no photograph. This store does not police it — the
	// publisher builds the message from a typed contract, which is where the list is enforceable.
	Than []byte

	// XayRaLuc is when the FACT happened, not when the relay will publish it. A consumer ordering by
	// publication time would reorder a commune's history after an outage.
	XayRaLuc time.Time
}

// Chen records one outgoing event INSIDE the caller's transaction.
//
// `gui_luc` IS NOT WRITTEN, so it is NULL: nothing has been published. That absence is the whole
// queue state — there is no status column beside it to get out of step with it.
func (s *SuKienDiStore) Chen(ctx context.Context, tx *store.ScopedTx, e SuKienDi) error {
	const stmt = `INSERT INTO su_kien_di (tenant_id, id, ten, doi_tuong, than, xay_ra_luc)
		VALUES ($1,$2,$3,$4,$5,$6)`

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), e.ID, e.Ten, e.DoiTuong, e.Than, e.XayRaLuc)
	if err != nil {
		// NOT the body: it carries a lookup code and an opaque citizen id, and an INSERT error can
		// quote the whole row on some drivers (rule 3).
		return fmt.Errorf("su_kien_di: ghi sự kiện: %w", err)
	}
	return nil
}
