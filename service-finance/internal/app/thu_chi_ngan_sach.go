package app

// The use cases behind the WRITE surface of the commune's budget board
// (docs/ui-ux/07-thu-chi-ngan-sach.md §4, §6, §7).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and nothing
// else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE TREE. Every decision on this screen is about the SHAPE of the sheet, not
// about the row being written: whether the line has children (the customer's 06/09/2026 rule),
// what its depth is, whether marking it leaves exactly one marked row, whether its parent lost its
// last child. Those need the whole tree AND the transaction at once, which is exactly one place —
// here. Each write therefore locks the sheet, reads the tree under the lock, asks internal/domain,
// and applies or refuses.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE — said once, here:
//
//	the two sheet kinds, the six roles   migration 0006's CHECK constraints
//	`manual` | `entries` | `children`    migration 0008 (widened 0006's CHECK)
//	one live sheet per year+kind         `UNIQUE (tenant_id, nam, loai, lan)` plus CoBangConSong
//	hard DELETE refused outright         `ho_so_luu_tru_cam_xoa_cung`
//
// AND ONE PROPERTY WHOSE FLOOR IS MISSING, said plainly rather than implied: "at most one row is
// marked as the total" has NO database constraint (migration 0006 sets out why a partial unique
// index cannot be verified in this build environment). What holds it is DatDongTong's statement
// pair under the sheet's row lock — and, because this will not be the table's only writer forever,
// domain.BangDayDu.DongTong REFUSES to produce a total when it finds more than one rather than
// summing or choosing.
//
// ---------------------------------------------------------------------------
// WHAT IS NOT BUILT HERE, deliberately:
//
//	the Excel import (§6)   `nguon_tep` / `nap_luc` are the columns it will fill, and TaoBang takes
//	                        the column set as data for exactly that reason — the parser is a turn of
//	                        its own and it is what fills them.
//	editing a COLUMN       a sheet's columns come from the Phòng Tài chính's file. Renaming one, or
//	                        moving a role from one column to another, changes which figure an
//	                        indicator reads — which is ADR 0035 §A territory and needs its own
//	                        decision about what happens to the periods already reported.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// KhoNganSach is the store, declared at the point of use.
//
// EVERY WRITE METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the sheet in
// one transaction and the audit entry in another: there is no signature here that would let you. It
// is also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the star is a radio — provable without a PostgreSQL. There is none
// reachable from this repository's build environment, so a test that needed one would never run.
type KhoNganSach interface {
	BangTheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.BangNganSach, error)
	BangDayDuTrongGiaoDich(ctx context.Context, tx *store.ScopedTx,
		bang domain.BangNganSach) (domain.BangDayDu, error)
	BangIDCuaKhoanMuc(ctx context.Context, tx *store.ScopedTx, khoanMucID string) (string, error)
	CoBangConSong(ctx context.Context, tx *store.ScopedTx, nam int, loai domain.LoaiBang) (bool, error)
	LanKeTiep(ctx context.Context, tx *store.ScopedTx, nam int, loai domain.LoaiBang) (int, error)

	ChenBang(ctx context.Context, tx *store.ScopedTx, b domain.BangNganSach) error
	ChenCot(ctx context.Context, tx *store.ScopedTx, c domain.CotNganSach) error
	XoaMemBang(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	CapNhatBang(ctx context.Context, tx *store.ScopedTx, b domain.BangNganSach) error

	ChenKhoanMuc(ctx context.Context, tx *store.ScopedTx, k domain.KhoanMucNganSach) error
	CapNhatKhoanMuc(ctx context.Context, tx *store.ScopedTx, k domain.KhoanMucNganSach) error
	DatCachTinh(ctx context.Context, tx *store.ScopedTx, id string, ct domain.CachTinh) error
	DatDongTong(ctx context.Context, tx *store.ScopedTx, bangID, id string) error
	XoaMemKhoanMuc(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	GhiGiaTri(ctx context.Context, tx *store.ScopedTx, khoanMucID, cotID string, gia *domain.Dong) error

	// The batches (migration 0008) — dot_thu_chi.go in this package.
	CoDotConSong(ctx context.Context, tx *store.ScopedTx, khoanMucID string) (bool, error)
	DotTheoIDTrongGiaoDich(ctx context.Context, tx *store.ScopedTx, id string) (domain.DotThuChi, error)
	ChenDot(ctx context.Context, tx *store.ScopedTx, d domain.DotThuChi) error
	ChenSoTienDot(ctx context.Context, tx *store.ScopedTx, dotID, cotID string, gia domain.Dong) error
	XoaMemDot(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// NganSach owns creating, removing, and editing one commune's budget board.
type NganSach struct {
	db  *store.DB
	kho KhoNganSach

	// sinhID is injected so a test can pin the id. In production it is ulid.Moi.
	sinhID func() (string, error)
}

func NewNganSach(db *store.DB, kho KhoNganSach) *NganSach {
	return &NganSach{db: db, kho: kho, sinhID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes: an inspection reads these strings, and a function name would tell them
// nothing.
//
// SIX VERBS AND NOT ONE `sua_ngan_sach`, because they are six administrative acts with six
// consequences. `dat_dong_tong_ngan_sach` in particular is the one worth searching for by name — it
// is the record of somebody choosing which row the commune's reported total comes from, and ADR
// 0035 §A is the reason that choice is a person's and not the software's.
const (
	HanhViTaoBangNganSach     = "tao_bang_ngan_sach"
	HanhViGoBangNganSach      = "go_bang_ngan_sach"
	HanhViSuaBangNganSach     = "sua_bang_ngan_sach"
	HanhViThemKhoanMuc        = "them_khoan_muc_ngan_sach"
	HanhViSuaKhoanMuc         = "sua_khoan_muc_ngan_sach"
	HanhViGoKhoanMuc          = "go_khoan_muc_ngan_sach"
	HanhViDatDongTongNganSach = "dat_dong_tong_ngan_sach"
	HanhViGhiDotThuChi        = "ghi_dot_thu_chi"
	HanhViGoDotThuChi         = "go_dot_thu_chi"
)

// --- creating a sheet ------------------------------------------------------------------------------

// YeuCauTaoBang is one new sheet together with its columns.
//
// THE COLUMNS ARRIVE WITH THE SHEET AND NOT AFTERWARDS, and that is not a convenience. §3's closing
// note makes columns DATA, and ADR 0035 §A makes two of them load-bearing: a sheet that existed for
// a while with no `Dự toán TP giao` column is a sheet whose indicator was absent for that while,
// with figures typed into it meanwhile. One act, one transaction, one audit entry describing the
// shape the commune actually created.
//
// THERE IS NO `Ma`, NO `Lan` AND NO `Id` FIELD. All three are the system's: `lan` comes from what is
// already in the table and `ma` is built from it. A client-supplied code on a table whose uniqueness
// counts soft-deleted rows is a client that can permanently burn a code.
type YeuCauTaoBang struct {
	Nam       int
	Loai      domain.LoaiBang
	TieuDe    string
	DonViTinh string    // one of the three wire codes — domain.KiemTraDonViTinh
	LuyKeDen  time.Time // zero when the commune has not stated it

	// Cot carries Ten, ThuTu, Kieu, CongThuc and VaiTro. ID and BangID are filled here.
	Cot []domain.CotNganSach
}

// TaoBang creates one sheet and its columns.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
func (uc *NganSach) TaoBang(ctx context.Context, yc YeuCauTaoBang,
	nguoi audit.Actor) (domain.BangNganSach, error) {

	if err := domain.KiemTraLoaiBang(yc.Loai); err != nil {
		return domain.BangNganSach{}, err
	}
	if err := domain.KiemTraNamNganSach(yc.Nam); err != nil {
		return domain.BangNganSach{}, err
	}
	tieuDe, err := domain.ChuanHoaTieuDeBang(yc.TieuDe)
	if err != nil {
		return domain.BangNganSach{}, err
	}
	donVi, err := domain.KiemTraDonViTinh(yc.DonViTinh)
	if err != nil {
		return domain.BangNganSach{}, err
	}

	cot := make([]domain.CotNganSach, 0, len(yc.Cot))
	for _, c := range yc.Cot {
		ten, err := domain.ChuanHoaTenCot(c.Ten)
		if err != nil {
			return domain.BangNganSach{}, err
		}
		congThuc, err := domain.ChuanHoaCongThuc(c.CongThuc)
		if err != nil {
			return domain.BangNganSach{}, err
		}
		if err := domain.KiemTraThuTuNganSach(c.ThuTu); err != nil {
			return domain.BangNganSach{}, err
		}
		cot = append(cot, domain.CotNganSach{
			Ten: ten, ThuTu: c.ThuTu, Kieu: c.Kieu, CongThuc: congThuc, VaiTro: c.VaiTro,
		})
	}
	// THE WHOLE SET AT ONCE, because the duplicate-role check cannot be made per column: two columns
	// both marked `Thu xã hưởng` is a sheet where the `Cân đối` cell has two candidate answers.
	if err := domain.KiemTraBoCot(yc.Loai, cot); err != nil {
		return domain.BangNganSach{}, err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.BangNganSach{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.BangNganSach{}, fmt.Errorf("ngan_sach: sinh mã bảng: %w", err)
	}

	moi := domain.BangNganSach{
		ID: id, Nam: yc.Nam, Loai: yc.Loai,
		// The LABEL is what the column stores — see domain.DonViTinh for why code and stored text differ.
		TieuDe: tieuDe, DonViTinh: donVi.Nhan(), LuyKeDen: yc.LuyKeDen,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// REFUSED RATHER THAN STACKED. §6's `🗑 Gỡ` is how a year's sheet is replaced; a second live
		// sheet for one year and kind would give that year two answers with nothing on either screen
		// saying which the report was built from.
		daCo, err := uc.kho.CoBangConSong(ctx, tx, yc.Nam, yc.Loai)
		if err != nil {
			return err
		}
		if daCo {
			return fistore.ErrBangDaTonTai
		}

		lan, err := uc.kho.LanKeTiep(ctx, tx, yc.Nam, yc.Loai)
		if err != nil {
			return err
		}
		moi.Lan = lan
		moi.Ma = maBang(yc.Nam, yc.Loai, lan)

		if err := uc.kho.ChenBang(ctx, tx, moi); err != nil {
			return err
		}
		for i := range cot {
			cotID, err := uc.sinhID()
			if err != nil {
				return fmt.Errorf("ngan_sach: sinh mã cột: %w", err)
			}
			cot[i].ID = cotID
			cot[i].BangID = moi.ID
			if err := uc.kho.ChenCot(ctx, tx, cot[i]); err != nil {
				return err
			}
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatBang(moi, cot)})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERTS (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4).
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViTaoBangNganSach, Subject: moi.Ma, Delta: delta,
		})
	})
	if err != nil {
		// Nothing was committed: no sheet, no columns, no trail. The states agree.
		return domain.BangNganSach{}, bocNganSach(ctx, "tạo bảng", err)
	}
	return moi, nil
}

// maBang builds the sheet's business code — `NS-2026-CHI-01`.
//
// IT IS BUILT AND NOT TYPED, and the two halves are why. It has to be UNIQUE FOREVER, counting
// removed sheets (rule 7, invariant 3), which a person cannot be asked to guarantee; and it is what
// `audit_log.subject` holds for every write on this board, so it has to be readable years later by
// somebody handling an inspection (rule 6, invariant 8). `NS-2026-CHI-01` names the thing; a ULID
// names nobody.
func maBang(nam int, loai domain.LoaiBang, lan int) string {
	return fmt.Sprintf("NS-%d-%s-%02d", nam, strings.ToUpper(string(loai)), lan)
}

// GoBang soft deletes one whole sheet — §6's `🗑 Gỡ`, the button whose own dialog warns it cannot be
// undone.
//
// "CANNOT BE UNDONE" IS TRUE ON THE SCREEN AND FALSE IN THE DATABASE, on purpose. The row stays with
// `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1) and its whole tree stays with
// it; what the commune loses is the sheet being ON any screen or in any total, which is what they
// asked for. Nothing is destroyed, and `ho_so_luu_tru_cam_xoa_cung` refuses a hard delete outright.
//
// THE REASON IS MANDATORY. A year's budget figures that vanished from every report with no reason
// attached is a question somebody will ask and nobody can answer — and the rows are still there, so
// it WILL be asked.
func (uc *NganSach) GoBang(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return fistore.ErrKhongThayBangNganSach
	}
	lyDo, err := domain.ChuanHoaLyDoXoaNganSach(lyDoTho)
	if err != nil {
		return err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.BangTheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds. Two
		// kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		if err := uc.kho.XoaMemBang(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"bang_id": truoc.ID,
			"truoc":   tomTatBang(truoc, nil),
			"ly_do":   lyDo,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViGoBangNganSach, Subject: truoc.Ma, Delta: delta,
		})
	})
	if err != nil {
		return bocNganSach(ctx, "gỡ bảng", err)
	}
	return nil
}

// YeuCauSuaBang is a PARTIAL edit of a sheet's header: a nil pointer means "leave this alone".
//
// THREE FIELDS AND NO MORE. `Nam`, `Loai`, `Ma`, `Lan` decide which report the figures belong to and
// the code the trail is filed under; the columns decide which figure an indicator reads (ADR 0035
// §A). None of them is an edit of this act.
type YeuCauSuaBang struct {
	TieuDe *string

	// LuyKeDen non-nil and ZERO clears the cut-off date — "the commune has not stated it" is a real
	// state of this column (store.ngayHoacNil writes it as NULL).
	LuyKeDen *time.Time

	// DonViTinh is one of the three wire codes. CHANGING IT CHANGES ONLY HOW THE FIGURES ARE
	// DISPLAYED: every stored figure is đồng and none is rescaled here (domain.DonViTinh).
	DonViTinh *string
}

// SuaBang edits one sheet's title, display unit and cut-off date.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING, the same property SuaKhoanMuc holds and for the same
// two reasons: an entry saying nothing changed buries the ones carrying legal weight, and it is what
// makes the route's idem.KhongCan declaration true.
//
// THE UNIT IS COMPARED AS THE STORED LABEL. A legacy sheet holding free text ("tr.đồng") that is set
// to `trieu-dong` DOES change — its column becomes the canonical label — and that is recorded, with
// the old text in `truoc`, because the screen will print its figures differently from that moment.
func (uc *NganSach) SuaBang(ctx context.Context, id string, yc YeuCauSuaBang,
	nguoi audit.Actor) (domain.BangNganSach, error) {

	if id == "" {
		return domain.BangNganSach{}, fistore.ErrKhongThayBangNganSach
	}
	// Shape first, outside the transaction: a request that fails its shape must never hold the
	// sheet's row lock while doing so.
	var tieuDe, donVi string
	var err error
	if yc.TieuDe != nil {
		if tieuDe, err = domain.ChuanHoaTieuDeBang(*yc.TieuDe); err != nil {
			return domain.BangNganSach{}, err
		}
	}
	if yc.DonViTinh != nil {
		d, err := domain.KiemTraDonViTinh(*yc.DonViTinh)
		if err != nil {
			return domain.BangNganSach{}, err
		}
		donVi = d.Nhan()
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.BangNganSach{}, err
	}

	var sau domain.BangNganSach
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// FOR UPDATE, and it excludes soft-deleted rows: a removed sheet and another commune's sheet
		// both answer ErrKhongThayBangNganSach, which the handler turns into one 404 body.
		truoc, err := uc.kho.BangTheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		sau = truoc
		if yc.TieuDe != nil {
			sau.TieuDe = tieuDe
		}
		if yc.DonViTinh != nil {
			sau.DonViTinh = donVi
		}
		if yc.LuyKeDen != nil {
			sau.LuyKeDen = *yc.LuyKeDen
		}

		if truoc.TieuDe == sau.TieuDe && truoc.DonViTinh == sau.DonViTinh &&
			truoc.LuyKeDen.Equal(sau.LuyKeDen) {
			return nil
		}
		if err := uc.kho.CapNhatBang(ctx, tx, sau); err != nil {
			return err
		}

		// BEFORE AND AFTER OF ALL THREE FIELDS (rule 6, invariant 5). Three short values, none of
		// them personal data; recording all three rather than only the moved ones lets an inspector
		// read the sheet's whole header at that instant from one entry.
		delta, err := json.Marshal(map[string]any{
			"bang_id": truoc.ID,
			"truoc":   dauBang(truoc),
			"sau":     dauBang(sau),
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViSuaBangNganSach, Subject: truoc.Ma, Delta: delta,
		})
	})
	if err != nil {
		return domain.BangNganSach{}, bocNganSach(ctx, "sửa bảng", err)
	}
	return sau, nil
}

// dauBang is the audit delta's view of a sheet's editable header.
func dauBang(b domain.BangNganSach) map[string]any {
	var luyKe any
	if !b.LuyKeDen.IsZero() {
		luyKe = b.LuyKeDen.Format(time.DateOnly)
	}
	return map[string]any{"tieu_de": b.TieuDe, "don_vi_tinh": b.DonViTinh, "luy_ke_den": luyKe}
}

// --- the lines -------------------------------------------------------------------------------------

// YeuCauThemKhoanMuc is one new line of the tree.
//
// THERE IS NO `CachTinh`, NO `Cap` AND NO `LaDongTong` FIELD, AND THERE MUST NEVER BE ONE.
//
//	CachTinh    follows from the tree (domain.CachTinhTheoCay). A field here is a field a request
//	            body can fill, and what that buys is a PARENT marked `manual` — the direct entry the
//	            customer's 06/09/2026 decision exists to forbid.
//	Cap         is the parent's depth plus one. A field is a second answer to a question the tree
//	            already answers, and the screen indents by it.
//	LaDongTong  is the star. Creating a line already marked would be a second marked row arriving
//	            with no act recorded, which is exactly the double count ADR 0035 §A turns on.
type YeuCauThemKhoanMuc struct {
	BangID string
	ChaID  string // "" at the top level
	TT     string
	Ten    string
	ThuTu  int
}

// ThemKhoanMuc adds one line — `＋ Thêm khoản mục con` and `⊞ Thêm khoản mục cấp cao nhất` (§4.3).
//
// IT MAY CHANGE ITS PARENT TOO, and that is the customer's rule taking effect: a leaf that gains its
// first child stops being typed into and starts summing. The flip is written in the SAME
// transaction, because a tree in which a parent still says `manual` while having a child is a tree
// whose figures are read from a locked cell.
//
// THE PARENT'S OWN CELLS ARE LEFT WHERE THEY ARE. §9 rule 1: switching to a computed mode LOCKS the
// typed figures, it does not erase them — and that is what makes the reverse move (GoKhoanMuc's last
// child) able to give the commune something back rather than a blank row.
func (uc *NganSach) ThemKhoanMuc(ctx context.Context, yc YeuCauThemKhoanMuc,
	nguoi audit.Actor) (domain.KhoanMucNganSach, error) {

	if yc.BangID == "" {
		return domain.KhoanMucNganSach{}, domain.ErrThieuBang
	}
	ten, err := domain.ChuanHoaTenKhoanMuc(yc.Ten)
	if err != nil {
		return domain.KhoanMucNganSach{}, err
	}
	tt, err := domain.ChuanHoaTT(yc.TT)
	if err != nil {
		return domain.KhoanMucNganSach{}, err
	}
	if err := domain.KiemTraThuTuNganSach(yc.ThuTu); err != nil {
		return domain.KhoanMucNganSach{}, err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.KhoanMucNganSach{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.KhoanMucNganSach{}, fmt.Errorf("ngan_sach: sinh mã khoản mục: %w", err)
	}

	moi := domain.KhoanMucNganSach{
		ID: id, BangID: yc.BangID, ChaID: yc.ChaID, TT: tt, Ten: ten, ThuTu: yc.ThuTu,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bang, day, err := uc.khoaVaDocCay(ctx, tx, yc.BangID)
		if err != nil {
			return err
		}

		var cha domain.KhoanMucNganSach
		coCha := yc.ChaID != ""
		if coCha {
			var thay bool
			cha, thay = day.TheoID(yc.ChaID)
			if !thay {
				// A parent that is not in THIS sheet's live tree: it does not exist, it was removed,
				// or it belongs to another sheet. One answer for all three — accepting it would put
				// one year's line under another year's tree and total them together.
				return domain.ErrChaKhongCungBang
			}
			// A PARENT IN `entries` MODE WITH LIVE BATCHES IS REFUSED, not flipped (decided 25/09/2026
			// under the user's "follow the recommendations"; migration 0008 question 4b left it open).
			// Flipped, it would become `children` and every batch in its dialog would silently stop
			// counting. Without live batches it flips below exactly like a `manual` leaf.
			if cha.CachTinh == domain.TinhTheoDot {
				conDot, err := uc.kho.CoDotConSong(ctx, tx, cha.ID)
				if err != nil {
					return err
				}
				if conDot {
					return domain.ErrKhoanMucTheoDotConDot
				}
			}
		}
		// A NEW LINE IS ALWAYS A LEAF, so its calculation mode is `manual` and its depth is the
		// parent's plus one. Both derived here and never taken from the request.
		moi.Cap = domain.CapTheoCha(cha, coCha)
		moi.CachTinh = domain.CachTinhTheoCay(false)

		if err := uc.kho.ChenKhoanMuc(ctx, tx, moi); err != nil {
			return err
		}

		chaDoiCachTinh := false
		if coCha && cha.CachTinh != domain.TinhTheoCon {
			if err := uc.kho.DatCachTinh(ctx, tx, cha.ID, domain.TinhTheoCon); err != nil {
				return err
			}
			chaDoiCachTinh = true
		}

		than := map[string]any{"sau": tomTatKhoanMuc(moi)}
		if chaDoiCachTinh {
			// RECORDED BECAUSE IT IS A CHANGE THE COMMUNE DID NOT ASK FOR. A parent that silently
			// stopped accepting typed figures is the single most confusing thing this screen can do,
			// and the entry is what lets somebody answer "why did that row stop being editable".
			than["cha_chuyen_sang_cong_tu_dong_con"] = map[string]any{
				"khoan_muc_id": cha.ID, "ten": cha.Ten, "cach_tinh_truoc": string(cha.CachTinh),
			}
		}
		delta, err := json.Marshal(than)
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViThemKhoanMuc, Subject: bang.Ma, Delta: delta,
		})
	})
	if err != nil {
		return domain.KhoanMucNganSach{}, bocNganSach(ctx, "thêm khoản mục", err)
	}
	return moi, nil
}

// YeuCauSuaKhoanMuc is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. `TT` is optional and its empty string is a MEANINGFUL
// value — §4.1's own sample has a row with no reference at all — so a struct of plain values cannot
// tell "the client did not mention this" from "the client cleared it", and a dialog editing only the
// name would wipe the reference off a budget line.
//
// `BangID`, `ChaID`, `Cap` AND `LaDongTong` ARE ABSENT AND NONE IS AN OVERSIGHT. Moving a line
// between sheets or under another parent moves its figure between two totals that have already been
// read off a screen; the other two are derived or have their own act.
type YeuCauSuaKhoanMuc struct {
	TT    *string
	Ten   *string
	ThuTu *int

	// CachTinh is the LEAF's choice between `manual` and `entries` (user decision 25/09/2026, §4.2).
	// `children` is never accepted — it follows the tree — and a line WITH children refuses both.
	//
	// WHAT A SWITCH DOES TO THE STORED FIGURES (§9.1), stated because the two directions differ:
	//
	//	manual  -> entries   the typed cells are LEFT UNTOUCHED and simply stop being displayed.
	//	entries -> manual    every number cell is OVERWRITTEN with the batch sum that was on the
	//	                     screen (NULL where the sum was empty), in the same transaction, audited.
	//
	// So the typed figures from before an entries period never come back: switching back to manual
	// shows what the batches added up to, which is §9.1's "giữ giá trị vừa tính làm giá trị khởi đầu".
	CachTinh *domain.CachTinh

	// GiaTri is cotID -> figure, and a nil VALUE means "clear this cell" (§9 rule 4: empty is a state
	// the screen draws as `—`, not the absence of a record). A cotID that is absent from the map is
	// a cell this request does not mention.
	GiaTri map[string]*domain.Dong
}

// SuaKhoanMuc edits one line's text and its figures.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a line the values it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// A FIGURE TYPED INTO A PARENT IS REFUSED — the customer's decision of 06/09/2026, enforced here and
// not only on the screen, because a screen-only rule holds until somebody calls the API directly.
// domain.ErrKhoanMucChaKhongGoThang carries the sentence, and migration 0006 carries the price the
// customer accepted with it.
func (uc *NganSach) SuaKhoanMuc(ctx context.Context, id string, yc YeuCauSuaKhoanMuc,
	nguoi audit.Actor) (domain.KhoanMucNganSach, error) {

	if id == "" {
		return domain.KhoanMucNganSach{}, domain.ErrKhongThayKhoanMuc
	}
	// Shape first, outside the transaction, for the same reason as ThemKhoanMuc.
	var ten, tt string
	var err error
	if yc.Ten != nil {
		if ten, err = domain.ChuanHoaTenKhoanMuc(*yc.Ten); err != nil {
			return domain.KhoanMucNganSach{}, err
		}
	}
	if yc.TT != nil {
		if tt, err = domain.ChuanHoaTT(*yc.TT); err != nil {
			return domain.KhoanMucNganSach{}, err
		}
	}
	if yc.ThuTu != nil {
		if err := domain.KiemTraThuTuNganSach(*yc.ThuTu); err != nil {
			return domain.KhoanMucNganSach{}, err
		}
	}
	for _, g := range yc.GiaTri {
		if g == nil {
			continue
		}
		if err := domain.KiemTraGiaTri(*g); err != nil {
			return domain.KhoanMucNganSach{}, err
		}
	}
	if yc.CachTinh != nil {
		if err := domain.KiemTraCachTinhChon(*yc.CachTinh); err != nil {
			return domain.KhoanMucNganSach{}, err
		}
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.KhoanMucNganSach{}, err
	}

	var sau domain.KhoanMucNganSach
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bangID, err := uc.kho.BangIDCuaKhoanMuc(ctx, tx, id)
		if err != nil {
			return err
		}
		bang, day, err := uc.khoaVaDocCay(ctx, tx, bangID)
		if err != nil {
			return err
		}
		truoc, thay := day.TheoID(id)
		if !thay {
			return domain.ErrKhongThayKhoanMuc
		}

		sau = truoc
		if yc.TT != nil {
			sau.TT = tt
		}
		if yc.Ten != nil {
			sau.Ten = ten
		}
		if yc.ThuTu != nil {
			sau.ThuTu = *yc.ThuTu
		}

		// THE MODE, DECIDED BEFORE ANY WRITE so every refusal below leaves nothing behind.
		cachTinhDoi := yc.CachTinh != nil && *yc.CachTinh != truoc.CachTinh
		if yc.CachTinh != nil && day.CoCon(truoc.ID) {
			return domain.ErrKhoanMucChaKhongDoiCachTinh
		}
		if cachTinhDoi {
			sau.CachTinh = *yc.CachTinh
		}
		// §4.2: an `entries` line's cells are read-only. Judged on the mode AFTER this request, so
		// "switch to manual and type a figure" in one PATCH is accepted and "switch to entries and
		// type a figure" is not.
		if len(yc.GiaTri) > 0 && sau.CachTinh == domain.TinhTheoDot && !day.CoCon(truoc.ID) {
			return domain.ErrKhoanMucTheoDotKhongGoThang
		}

		var soTuDot []map[string]any
		if cachTinhDoi {
			if truoc.CachTinh == domain.TinhTheoDot && sau.CachTinh == domain.TinhTay {
				if day.Gia == nil {
					day.Gia = map[string]map[string]domain.Dong{}
				}
				if soTuDot, err = uc.chepTongDotVaoO(ctx, tx, day, truoc.ID); err != nil {
					return err
				}
			}
			if err := uc.kho.DatCachTinh(ctx, tx, truoc.ID, sau.CachTinh); err != nil {
				return err
			}
		}

		// THE FIGURES, AND THE CUSTOMER'S RULE IN FRONT OF THEM. The check is on the LINE, not on
		// each cell: a parent is a parent for every column at once.
		oDoi, err := uc.ghiCacO(ctx, tx, day, truoc, yc.GiaTri)
		if err != nil {
			return err
		}

		vanBanDoi := truoc.TT != sau.TT || truoc.Ten != sau.Ten || truoc.ThuTu != sau.ThuTu
		if !vanBanDoi && len(oDoi) == 0 && !cachTinhDoi {
			return nil
		}
		if vanBanDoi {
			if err := uc.kho.CapNhatKhoanMuc(ctx, tx, sau); err != nil {
				return err
			}
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column of every cell on every edit makes the one figure somebody actually changed
		// impossible to find in a ledger that is never deleted — and on this screen the figure IS the
		// record: it is what goes into the document sent to the higher authority.
		than := map[string]any{"khoan_muc_id": truoc.ID}
		if vanBanDoi {
			than["truoc"] = tomTatDoiKhoanMuc(truoc, sau, true)
			than["sau"] = tomTatDoiKhoanMuc(truoc, sau, false)
		}
		if len(oDoi) > 0 {
			than["o"] = oDoi
		}
		if cachTinhDoi {
			// THE SWITCH IS RECORDED WITH WHAT IT DID TO THE CELLS. entries -> manual rewrites the
			// line's figures from the batch sums, and "why did these cells change" is answered only here.
			than["cach_tinh"] = map[string]any{
				"truoc": string(truoc.CachTinh), "sau": string(sau.CachTinh),
			}
			if len(soTuDot) > 0 {
				than["o_lay_tu_tong_dot"] = soTuDot
			}
		}
		delta, err := json.Marshal(than)
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViSuaKhoanMuc, Subject: bang.Ma, Delta: delta,
		})
	})
	if err != nil {
		return domain.KhoanMucNganSach{}, bocNganSach(ctx, "sửa khoản mục", err)
	}
	return sau, nil
}

// chepTongDotVaoO is §9.1's entries -> manual hand-over: every NUMBER cell of the line becomes the
// batch sum that was on the screen a moment ago — the sum, or NULL where the sum was empty.
//
// NULL AND NOT "LEAVE THE OLD TYPED FIGURE": a typed figure from before the entries period would
// otherwise reappear in a column the screen showed as `—`, and the commune would see a number nobody
// entered for this period. Only cells that actually change are written.
//
// It UPDATES day.Gia for the line, so a `values` map in the same request is compared against the
// figures the line holds after the hand-over, not before.
func (uc *NganSach) chepTongDotVaoO(ctx context.Context, tx *store.ScopedTx, day domain.BangDayDu,
	khoanMucID string) ([]map[string]any, error) {

	// day.Gia is non-nil (the caller guarantees it), so this inner map is shared with the caller's copy.
	if day.Gia[khoanMucID] == nil {
		day.Gia[khoanMucID] = map[string]domain.Dong{}
	}
	var doi []map[string]any
	for _, cot := range day.Cot {
		if cot.Kieu != domain.CotSo {
			continue
		}
		tong, coTong := day.GiaDot[khoanMucID][cot.ID]
		var moi *domain.Dong
		if coTong {
			// The typo guard every typed cell passes. A batch sum beyond it is not a budget figure,
			// and copying it would put into a manual cell what no person could have typed there.
			if err := domain.KiemTraGiaTri(tong); err != nil {
				return nil, err
			}
			g := tong
			moi = &g
		}
		cu, coCu := day.Gia[khoanMucID][cot.ID]
		if khongDoiO(cu, coCu, moi) {
			continue
		}
		if err := uc.kho.GhiGiaTri(ctx, tx, khoanMucID, cot.ID, moi); err != nil {
			return nil, err
		}
		doi = append(doi, map[string]any{
			"cot_id": cot.ID, "cot": cot.Ten,
			"truoc": soHoacNil(cu, coCu), "sau": soHoacNilCon(moi),
		})
		if coTong {
			day.Gia[khoanMucID][cot.ID] = tong
		} else {
			delete(day.Gia[khoanMucID], cot.ID)
		}
	}
	return doi, nil
}

// ghiCacO writes the cells this request mentions and returns a before/after record of the ones that
// actually moved.
//
// THREE REFUSALS, ALL BEFORE ANY WRITE:
//
//	the line has children     domain.ChoGhiGiaTri — the customer's 06/09/2026 decision.
//	the column is not this     a figure filed against another sheet's column is a figure on no
//	  sheet's                  screen, sitting in the table looking healthy.
//	the column is `phan_tram`  §9 rule 3: a percentage is computed at render, never stored. A stored
//	                           one is a second home for a derivable number, and the stale copy is the
//	                           one that reaches the report.
func (uc *NganSach) ghiCacO(ctx context.Context, tx *store.ScopedTx, day domain.BangDayDu,
	k domain.KhoanMucNganSach, o map[string]*domain.Dong) ([]map[string]any, error) {

	if len(o) == 0 {
		return nil, nil
	}
	if err := domain.ChoGhiGiaTri(day.CoCon(k.ID)); err != nil {
		return nil, err
	}

	// SORTED BY THE SHEET'S OWN COLUMN ORDER rather than by map iteration, so the audit delta of one
	// edit reads the same way twice and two runs of a test compare equal.
	var doi []map[string]any
	for _, cot := range day.Cot {
		moi, duocNhac := o[cot.ID]
		if !duocNhac {
			continue
		}
		if cot.Kieu != domain.CotSo {
			return nil, domain.ErrCotKhongPhaiCotSo
		}
		cu, coCu := day.Gia[k.ID][cot.ID]
		if khongDoiO(cu, coCu, moi) {
			continue
		}
		if err := uc.kho.GhiGiaTri(ctx, tx, k.ID, cot.ID, moi); err != nil {
			return nil, err
		}
		doi = append(doi, map[string]any{
			"cot_id": cot.ID,
			"cot":    cot.Ten,
			"truoc":  soHoacNil(cu, coCu),
			"sau":    soHoacNilCon(moi),
		})
	}
	// EVERY MENTIONED COLUMN MUST BE ONE OF THIS SHEET'S. Counting what was matched is how a column
	// id belonging to another sheet is caught — the loop above simply never sees it, and a silent
	// "wrote nothing" would tell the client its figure was saved.
	if len(o) != demOTrongBang(day, o) {
		return nil, domain.ErrCotKhongThuocBang
	}
	return doi, nil
}

func demOTrongBang(day domain.BangDayDu, o map[string]*domain.Dong) int {
	n := 0
	for _, cot := range day.Cot {
		if _, co := o[cot.ID]; co {
			n++
		}
	}
	return n
}

func khongDoiO(cu domain.Dong, coCu bool, moi *domain.Dong) bool {
	if moi == nil {
		return !coCu
	}
	return coCu && cu == *moi
}

func soHoacNil(g domain.Dong, co bool) any {
	if !co {
		return nil
	}
	return int64(g)
}

func soHoacNilCon(g *domain.Dong) any {
	if g == nil {
		return nil
	}
	return int64(*g)
}

// GoKhoanMuc soft deletes one line — `🗑 Gỡ khoản mục` (§4.1).
//
// A LINE WITH CHILDREN IS REFUSED rather than cascaded (domain.ErrConGiuKhoanMucCon). A cascade
// takes a whole branch off every total in one click, and the rows are archival: they stay in the
// table carrying `deleted_at`, so "undo" is not a button, it is re-entering them one by one. Making
// the commune remove the children first makes the size of the act visible while it is still being
// decided.
//
// REMOVING THE LAST CHILD GIVES THE PARENT ITS FIGURES BACK, and that is §9 rule 1's second half:
// "đổi ngược lại thì giữ giá trị vừa tính làm giá trị khởi đầu". The parent returns to `manual` and
// each of its numeric cells is written with the total that was on the screen a moment ago —
// otherwise the row would go blank, and blank on this screen means "nobody has entered this", which
// is not what happened.
func (uc *NganSach) GoKhoanMuc(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return domain.ErrKhongThayKhoanMuc
	}
	lyDo, err := domain.ChuanHoaLyDoXoaNganSach(lyDoTho)
	if err != nil {
		return err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bangID, err := uc.kho.BangIDCuaKhoanMuc(ctx, tx, id)
		if err != nil {
			return err
		}
		bang, day, err := uc.khoaVaDocCay(ctx, tx, bangID)
		if err != nil {
			return err
		}
		truoc, thay := day.TheoID(id)
		if !thay {
			return domain.ErrKhongThayKhoanMuc
		}
		if err := domain.ChoGo(day.CoCon(id)); err != nil {
			return err
		}

		// COMPUTED BEFORE THE REMOVAL, because after it the parent has no children and the figure
		// this is trying to preserve no longer exists anywhere.
		var traLai []map[string]any
		chaVeTinhTay := truoc.ChaID != "" && len(day.ConTrucTiep(truoc.ChaID)) == 1
		giuLai := map[string]domain.Dong{}
		if chaVeTinhTay {
			for _, cot := range day.Cot {
				if cot.Kieu != domain.CotSo {
					continue
				}
				if g, co := day.GiaTri(truoc.ChaID, cot.ID); co {
					giuLai[cot.ID] = g
				}
			}
		}

		if err := uc.kho.XoaMemKhoanMuc(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		if chaVeTinhTay {
			if err := uc.kho.DatCachTinh(ctx, tx, truoc.ChaID, domain.TinhTay); err != nil {
				return err
			}
			for _, cot := range day.Cot {
				g, co := giuLai[cot.ID]
				if !co {
					continue
				}
				giuG := g
				if err := uc.kho.GhiGiaTri(ctx, tx, truoc.ChaID, cot.ID, &giuG); err != nil {
					return err
				}
				traLai = append(traLai, map[string]any{"cot_id": cot.ID, "gia_tri": int64(g)})
			}
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication rule 9
		// forbids: the column is the current state of the row and can only ever hold the FIRST
		// removal, while the entry is the append-only record of the act.
		than := map[string]any{
			"khoan_muc_id": truoc.ID,
			"truoc":        tomTatKhoanMuc(truoc),
			"ly_do":        lyDo,
			"xoa_mem":      true,
		}
		if chaVeTinhTay {
			than["cha_ve_nhap_tay"] = map[string]any{
				"khoan_muc_id": truoc.ChaID,
				"giu_lai":      traLai,
			}
		}
		delta, err := json.Marshal(than)
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViGoKhoanMuc, Subject: bang.Ma, Delta: delta,
		})
	})
	if err != nil {
		return bocNganSach(ctx, "gỡ khoản mục", err)
	}
	return nil
}

// DatDongTong marks one line as the sheet's total row — the `☆` of §4.1.
//
// THE MOST CONSEQUENTIAL ROUTE ON THIS SCREEN, and the shortest. It decides which row every summary
// cell and both indicators are read from, which decides the figures the commune puts in a document
// sent upward. ADR 0035 §A is why it is a person's act at all: no ordering, no depth and no name
// pattern can tell the thu sheet's two nested top-level rows from the chi sheet's `Tổng số` sitting
// beside A…E, and getting it wrong double-counts on one form while looking right on the other.
//
// IT IS A RADIO AND THE STORE MAKES IT ONE: DatDongTong clears every other row of the sheet and sets
// this one, both inside this transaction and under the sheet's row lock. THERE IS NO "UNMARK"
// ROUTE — §4.1 draws the star as something that MOVES ("bấm ngôi sao ở đầu một dòng khác để đổi"),
// and a sheet that had a total and then deliberately had none is a state nothing on that screen
// asks for. A sheet loses its total only by the marked row being removed, which releases the flag.
func (uc *NganSach) DatDongTong(ctx context.Context, id string,
	nguoi audit.Actor) (domain.KhoanMucNganSach, error) {

	if id == "" {
		return domain.KhoanMucNganSach{}, domain.ErrKhongThayKhoanMuc
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.KhoanMucNganSach{}, err
	}

	var sau domain.KhoanMucNganSach
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bangID, err := uc.kho.BangIDCuaKhoanMuc(ctx, tx, id)
		if err != nil {
			return err
		}
		bang, day, err := uc.khoaVaDocCay(ctx, tx, bangID)
		if err != nil {
			return err
		}
		truoc, thay := day.TheoID(id)
		if !thay {
			return domain.ErrKhongThayKhoanMuc
		}

		// WHICH ROW HELD IT BEFORE, read BEFORE the write and recorded in the entry. "The commune's
		// reported total moved from this row to that one" is the whole content of this act, and after
		// the write the previous answer is gone from the table.
		var truocDongTong string
		for _, k := range day.KhoanMuc {
			if k.LaDongTong {
				truocDongTong = k.ID
			}
		}

		if err := uc.kho.DatDongTong(ctx, tx, bangID, truoc.ID); err != nil {
			return err
		}
		sau = truoc
		sau.LaDongTong = true

		delta, err := json.Marshal(map[string]any{
			"khoan_muc_id": truoc.ID,
			"truoc":        map[string]any{"dong_tong_id": truocDongTong},
			"sau":          map[string]any{"dong_tong_id": truoc.ID, "ten": truoc.Ten, "tt": truoc.TT},
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViDatDongTongNganSach, Subject: bang.Ma, Delta: delta,
		})
	})
	if err != nil {
		return domain.KhoanMucNganSach{}, bocNganSach(ctx, "đặt dòng tổng", err)
	}
	return sau, nil
}

// khoaVaDocCay is the opening move of every line write: lock the sheet, then read its tree under the
// lock.
//
// ONE FUNCTION AND NOT FOUR COPIES, because the part that is identical is the part that is easy to
// get subtly wrong in one copy — reading the tree BEFORE taking the lock, which gives a decision
// made against a sheet that has since moved.
func (uc *NganSach) khoaVaDocCay(ctx context.Context, tx *store.ScopedTx,
	bangID string) (domain.BangNganSach, domain.BangDayDu, error) {

	bang, err := uc.kho.BangTheoIDDeSua(ctx, tx, bangID)
	if err != nil {
		return domain.BangNganSach{}, domain.BangDayDu{}, err
	}
	day, err := uc.kho.BangDayDuTrongGiaoDich(ctx, tx, bang)
	if err != nil {
		return domain.BangNganSach{}, domain.BangDayDu{}, err
	}
	return bang, day, nil
}

// bocNganSach wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No figure, no line name, no title: an error travels into centralised logging across every commune
// at once, and a budget line's text is a public authority's own wording about its spending. The
// commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this line has children" and "the database is down" into one 500.
func bocNganSach(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("ngan_sach: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}

// tomTatBang is the audit delta's view of one sheet. NOTHING HERE IS PERSONAL DATA (rule 3): a
// budget sheet carries figures, a title and column headings, and no person appears on it.
func tomTatBang(b domain.BangNganSach, cot []domain.CotNganSach) map[string]any {
	ra := map[string]any{
		"bang_id":     b.ID,
		"ma":          b.Ma,
		"nam":         b.Nam,
		"loai":        string(b.Loai),
		"lan":         b.Lan,
		"tieu_de":     b.TieuDe,
		"don_vi_tinh": b.DonViTinh,
	}
	if len(cot) == 0 {
		return ra
	}
	// THE COLUMN SET IS IN THE ENTRY FOR THE CREATE, and the roles with it. Which column carries
	// `Thu xã hưởng` decides the `Cân đối` cell (ADR 0035 #32), so "which columns did this sheet have
	// when it was created" is a question an inspection can genuinely need — and the columns are
	// editable by nothing, so this entry is the only record of the answer.
	var tom []map[string]any
	for _, c := range cot {
		tom = append(tom, map[string]any{
			"cot_id": c.ID, "ten": c.Ten, "thu_tu": c.ThuTu,
			"kieu": string(c.Kieu), "vai_tro": string(c.VaiTro),
		})
	}
	ra["cot"] = tom
	return ra
}

// tomTatKhoanMuc is the audit delta's view of one line.
func tomTatKhoanMuc(k domain.KhoanMucNganSach) map[string]any {
	return map[string]any{
		"khoan_muc_id": k.ID,
		"bang_id":      k.BangID,
		"cha_id":       k.ChaID,
		"tt":           k.TT,
		"ten":          k.Ten,
		"thu_tu":       k.ThuTu,
		"cach_tinh":    string(k.CachTinh),
		"cap":          k.Cap,
		"la_dong_tong": k.LaDongTong,
	}
}

// tomTatDoiKhoanMuc returns only the text fields that actually moved, from whichever side is asked
// for. The figures are not here: they travel in their own `o` list, with the column named, because a
// cell is identified by a pair and not by a field name.
func tomTatDoiKhoanMuc(truoc, sau domain.KhoanMucNganSach, ben bool) map[string]any {
	ra := map[string]any{}
	if truoc.TT != sau.TT {
		ra["tt"] = chon(ben, truoc.TT, sau.TT)
	}
	if truoc.Ten != sau.Ten {
		ra["ten"] = chon(ben, truoc.Ten, sau.Ten)
	}
	if truoc.ThuTu != sau.ThuTu {
		ra["thu_tu"] = chon(ben, truoc.ThuTu, sau.ThuTu)
	}
	return ra
}
