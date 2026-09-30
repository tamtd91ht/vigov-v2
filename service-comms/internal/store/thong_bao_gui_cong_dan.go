package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ThongBaoGuiCongDanStore is the only place that knows the SQL of the citizen notification
// ledger (`migrations/0004_thong_bao_gui_cong_dan.sql`).
//
// It is built from *store.DB and reaches the database only through Scoped — there is no field
// here holding a *sql.DB, and adding one would reopen the hole core/store exists to close
// (rule 1, invariant 5).
//
// THE TWO WRITE METHODS TAKE A *store.ScopedTx AND THE READ TAKES A CONTEXT, and that asymmetry
// is the shape rule 6 forces: the audit entry has to share the transaction with the write, so a
// write method that opened its own transaction would make that impossible for its caller. The
// transaction is opened one level up, in internal/app, where the business act is.
//
// NO audit.Write IN THIS FILE, AND THAT IS NOT THE OMISSION IT LOOKS LIKE — audit_guard warns on
// exactly this shape, so the answer is written down rather than rediscovered. Rule 6 requires the
// entry to share the TRANSACTION, which it does: app/SoThongBao opens one, calls the method
// below, and writes the entry inside the same *store.ScopedTx. What the entry cannot come from is
// here — `Actor` is who caused the write (a consumer's system principal, a member of staff, an
// IP) and `Action` is a business verb, and neither exists at the SQL layer. Putting audit.Write
// in this file would make the store invent an actor, which is how an unattributable entry is
// born. service-identity draws the same line: store/phien.go inserts, app/dang_nhap.go audits.
type ThongBaoGuiCongDanStore struct {
	db *store.DB
}

func NewThongBaoGuiCongDanStore(db *store.DB) *ThongBaoGuiCongDanStore {
	return &ThongBaoGuiCongDanStore{db: db}
}

// TranThongBaoMotHoSo bounds the ledger read for ONE business record.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so every
// list read is a shared resource and an unbounded one is forbidden (`skills/rest-api-design` §5,
// FORBIDDEN #4). This read returns the whole history of one record on purpose — a partial history
// of what a commune told a citizen is worse than none, because it reads as complete — so the
// bound cannot come from a `limit` parameter.
//
// 200 IS ABOUT TWO ORDERS OF MAGNITUDE ABOVE A REAL RECORD. A petition notifies at three points
// in its lifecycle (`skills/petition-lifecycle`: intake, processing, closing), and a reopening
// (ADR 0008 caps it at one by default) adds a few more. Anything past 200 is not a long-running
// case; it is a consumer in a loop, an import run twice, or a producer republishing — and the
// citizen whose phone those messages went to is already having a worse day than the operator.
const TranThongBaoMotHoSo = 200

// ErrQuaNhieuThongBao says the ceiling was reached. The caller refuses and raises an operational
// alarm rather than rendering a partial history.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list answers "what did we
// tell this citizen, and when", which is the question a complaint or an inspection opens with. A
// silently short answer is a commune stating it never sent something it did send. Between a wrong
// answer nobody notices and no answer somebody fixes, this system chooses the second (fail
// closed).
var ErrQuaNhieuThongBao = errors.New("thong_bao_gui_cong_dan: vượt trần một hồ sơ")

// ErrThongBaoKhongTonTai says the id addressed no live row OF THIS COMMUNE.
//
// The two causes are deliberately not distinguished: "no such notification" and "that
// notification belongs to another commune" must look identical from outside, or the error itself
// becomes a way to discover what another authority holds (rule 4, forbidden #2, applied to the
// commune axis).
var ErrThongBaoKhongTonTai = errors.New("thong_bao_gui_cong_dan: không có bản ghi")

// cotThongBao IS READ BY POSITION in the scan below. Several adjacent columns share a type —
// `doi_tuong_ma`/`moc`, `nguoi_nhan_ma`/`nguoi_nhan_che`, `mau_ma`/`trang_thai` — so swapping a
// pair here without swapping it there produces no error at all. It produces a ledger that says a
// message about record A went to record B's code.
const cotThongBao = `id, khoa_lan_gui, doi_tuong_loai, doi_tuong_ma, moc, lan, kenh, ` +
	`nguoi_nhan_ma, nguoi_nhan_che, mau_ma, tham_so, trang_thai, so_lan_thu, gui_luc, loi_ma, tao_luc`

// chenThongBao records the obligation. ON CONFLICT DO NOTHING is the idempotency.
//
// WHY `DO NOTHING` AND NOT `DO UPDATE`: the conflicting row is the SAME FACT, already recorded,
// possibly already delivered. Updating it would let a redelivery reset a `da-gui` row back to
// `cho-gui` and send the citizen a second copy — and the trigger in migration 0004 refuses that
// anyway, so `DO UPDATE` would turn a harmless duplicate into a failed transaction that rolls
// back a business write.
//
// The statement names `tenant_id` first, as every write in this system does: the commune is not
// an argument the caller chooses, it is bound from the transaction (rule 1, invariants 4 and 5).
const chenThongBao = `INSERT INTO thong_bao_gui_cong_dan
	(tenant_id, id, khoa_lan_gui, doi_tuong_loai, doi_tuong_ma, moc, lan, kenh,
	 nguoi_nhan_ma, mau_ma, tham_so, trang_thai)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	ON CONFLICT (tenant_id, khoa_lan_gui) DO NOTHING`

// Chen records that this commune owes this citizen one notification.
//
// IT RETURNS WHETHER A ROW WAS ACTUALLY CREATED, and that boolean is load-bearing rather than
// informational: the caller uses it to decide whether to write an audit entry. A redelivery must
// not add a second entry saying the commune promised something twice, and it must not fail
// either — queues deliver at least once (rule 2, invariant 5), so the second delivery is normal
// operation, not an error.
//
// `trang_thai` IS NOT A PARAMETER. A row is born `cho-gui` and nothing else: the obligation is
// recorded before any send is attempted, because sending is eventually consistent with the
// business write (ADR 0006, consequence 4). Letting the caller name the state would let a
// producer insert a row already marked `da-gui`, which is a claim about the citizen's phone that
// nothing in this service witnessed.
func (s *ThongBaoGuiCongDanStore) Chen(ctx context.Context, tx *store.ScopedTx,
	tb domain.ThongBaoGuiCongDan) (bool, error) {

	// Validated HERE as well as in the caller, because this is the last point before the row
	// exists. The CHECK constraint on `tham_so` would catch the missing keys, but it arrives as a
	// database error at the bottom of a transaction with no explanation of which rule was broken.
	thamSo, err := tb.ThamSo.JSON()
	if err != nil {
		return false, fmt.Errorf("thong_bao_gui_cong_dan: tham số: %w", err)
	}

	kq, err := tx.Exec(ctx, chenThongBao,
		string(tx.TenantID()), tb.ID, tb.KhoaLanGui, string(tb.DoiTuongLoai), tb.DoiTuongMa,
		tb.Moc, tb.Lan, string(tb.Kenh), tb.NguoiNhanMa, tb.MauMa, thamSo, string(domain.ChoGui))
	if err != nil {
		return false, fmt.Errorf("thong_bao_gui_cong_dan: ghi sổ: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("thong_bao_gui_cong_dan: đếm dòng đã ghi: %w", err)
	}
	return n > 0, nil
}

// ghiKetQuaThongBao records the outcome of one send attempt.
//
// THE PREDICATE CARRIES THE IDEMPOTENCY: `trang_thai <> 'da-gui'` means a second report of the
// same successful delivery matches no row and changes nothing, instead of hitting the trigger
// that refuses to move a delivered notification. `deleted_at IS NULL` is there for the same
// reason every read path has it (rule 7, invariant 2) — a soft-deleted row is not a row this
// service still acts on.
//
// `nguoi_nhan_che` USES COALESCE SO AN EMPTY VALUE DOES NOT ERASE ONE ALREADY RECORDED: a first
// attempt that reached the gateway knows which number it used; a later attempt that failed before
// resolving one does not, and must not overwrite the fact with nothing.
const ghiKetQuaThongBao = `UPDATE thong_bao_gui_cong_dan
	SET trang_thai = $3,
	    gui_luc = $4,
	    nguoi_nhan_che = COALESCE(NULLIF($5, ''), nguoi_nhan_che),
	    loi_ma = NULLIF($6, ''),
	    so_lan_thu = $7,
	    cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND trang_thai <> 'da-gui'`

// GhiKetQua records what the channel did with one notification.
//
// It returns whether a row moved. `false` means the notification was already delivered — which is
// a normal outcome of an at-least-once queue, not a failure — and the caller writes no second
// audit entry for it. A genuinely missing id is reported as ErrThongBaoKhongTonTai only when
// nothing matched AND the row is not already delivered; the two cases are separated by a second,
// narrow read rather than by guessing, because "already sent" and "does not exist" must lead to
// very different operator behaviour.
func (s *ThongBaoGuiCongDanStore) GhiKetQua(ctx context.Context, tx *store.ScopedTx,
	id string, kq domain.KetQuaGui) (bool, error) {

	if err := kq.KiemTra(); err != nil {
		return false, fmt.Errorf("thong_bao_gui_cong_dan: kết quả gửi: %w", err)
	}

	var guiLuc any
	if !kq.GuiLuc.IsZero() {
		guiLuc = kq.GuiLuc.UTC()
	}

	kqSQL, err := tx.Exec(ctx, ghiKetQuaThongBao,
		string(tx.TenantID()), id, string(kq.TrangThai), guiLuc,
		kq.NguoiNhanChe, kq.LoiMa, kq.SoLanThu)
	if err != nil {
		return false, fmt.Errorf("thong_bao_gui_cong_dan: ghi kết quả gửi: %w", err)
	}
	n, err := kqSQL.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("thong_bao_gui_cong_dan: đếm dòng đã đổi: %w", err)
	}
	if n > 0 {
		return true, nil
	}

	co, err := s.tonTai(ctx, tx, id)
	if err != nil {
		return false, err
	}
	if !co {
		return false, fmt.Errorf("%w: %s", ErrThongBaoKhongTonTai, id)
	}
	// The row exists and did not move: it is already `da-gui`. Nothing to do and nothing wrong.
	return false, nil
}

// demThongBaoTheoID answers whether this commune has a live row with this id. The commune is in
// the statement as `tenant_id = $1`, not as a filter the caller could forget.
const demThongBaoTheoID = `SELECT 1 FROM thong_bao_gui_cong_dan
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *ThongBaoGuiCongDanStore) tonTai(ctx context.Context, tx *store.ScopedTx, id string) (bool, error) {
	var mot int
	err := tx.Underlying().QueryRowContext(ctx, demThongBaoTheoID, string(tx.TenantID()), id).Scan(&mot)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("thong_bao_gui_cong_dan: tra bản ghi: %w", err)
	}
	return true, nil
}

// TheoDoiTuong reads everything this commune has told a citizen about ONE business record.
//
// NEWEST FIRST, because that is the question's real shape: the last thing the citizen was told is
// what they are holding when they ring. The tie-break is `id`, which carries UNIQUE
// (tenant_id, id), so the order is TOTAL — two calls cannot return the same rows in a different
// sequence, which is what stops a client-side diff from flickering and stops any test of this
// from comparing sets by accident.
//
// NOT PAGINATED, AND THAT IS A DECISION. The whole history of one record is the smallest unit
// that answers the question at all; a first page of it reads as complete and is not. The bound
// pagination would have provided is provided instead by TranThongBaoMotHoSo.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the ledger of the commune the request arrived in (rule 1,
// invariants 4 and 5). `doi_tuong_ma` is a business code and is matched exactly — it is never
// used as a prefix or a pattern, which would turn one lookup code into a way to enumerate others
// (rule 4, invariant 4).
func (s *ThongBaoGuiCongDanStore) TheoDoiTuong(ctx context.Context, loai domain.DoiTuongLoai,
	doiTuongMa string) ([]domain.ThongBaoGuiCongDan, error) {

	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete history
	// of that size — the truncation this read refuses to perform, performed by the bound meant to
	// prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotThongBao, "thong_bao_gui_cong_dan",
		`AND doi_tuong_loai = $2 AND doi_tuong_ma = $3 AND deleted_at IS NULL
		 ORDER BY tao_luc DESC, id DESC LIMIT $4`,
		string(loai), doiTuongMa, TranThongBaoMotHoSo+1)
	if err != nil {
		return nil, fmt.Errorf("thong_bao_gui_cong_dan: đọc sổ: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.ThongBaoGuiCongDan, 0, 8)
	for rows.Next() {
		tb, err := docMotThongBao(rows)
		if err != nil {
			return nil, err
		}
		ra = append(ra, tb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thong_bao_gui_cong_dan: duyệt kết quả: %w", err)
	}
	if len(ra) > TranThongBaoMotHoSo {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuThongBao
	}
	return ra, nil
}

// docMotThongBao scans one row. POSITIONAL — in lockstep with cotThongBao.
//
// THE THREE NULLABLE COLUMNS ARE NULLABLE FOR THREE DIFFERENT REASONS, and collapsing any of them
// to an empty string in the schema would destroy a distinction somebody needs:
//
//	nguoi_nhan_che  NULL until a send is attempted — the masked number is not known before then
//	gui_luc         NULL until the channel accepted it
//	loi_ma          NULL when nothing failed
//
// In Go they land as the zero value, which is correct for every reader: an empty masked number is
// "not known", not "the empty number".
func docMotThongBao(rows *sql.Rows) (domain.ThongBaoGuiCongDan, error) {
	var (
		tb     domain.ThongBaoGuiCongDan
		loai   string
		kenh   string
		trang  string
		thamSo []byte
		che    sql.NullString
		loiMa  sql.NullString
		guiLuc sql.NullTime
	)
	if err := rows.Scan(&tb.ID, &tb.KhoaLanGui, &loai, &tb.DoiTuongMa, &tb.Moc, &tb.Lan, &kenh,
		&tb.NguoiNhanMa, &che, &tb.MauMa, &thamSo, &trang, &tb.SoLanThu, &guiLuc, &loiMa,
		&tb.TaoLuc); err != nil {
		return domain.ThongBaoGuiCongDan{}, fmt.Errorf("thong_bao_gui_cong_dan: đọc dòng: %w", err)
	}
	if err := json.Unmarshal(thamSo, &tb.ThamSo); err != nil {
		return domain.ThongBaoGuiCongDan{}, fmt.Errorf("thong_bao_gui_cong_dan: đọc tham số: %w", err)
	}
	tb.DoiTuongLoai = domain.DoiTuongLoai(loai)
	tb.Kenh = domain.Kenh(kenh)
	tb.TrangThai = domain.TrangThaiGui(trang)
	tb.NguoiNhanChe = che.String
	tb.LoiMa = loiMa.String
	tb.GuiLuc = guiLuc.Time
	return tb, nil
}
