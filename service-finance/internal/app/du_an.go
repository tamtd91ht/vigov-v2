package app

// The use cases behind the WRITE surface of the investment project register
// (docs/ui-ux/06-giai-ngan.md §9 the "Thêm dự án" modal, §8's `[✎ Sửa dự án]`, §13).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and
// nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THAT EVERY WRITE HERE SPANS MORE THAN ONE STATEMENT. Creating a project
// writes the project AND its funding allocation lines; removing one removes the lines and the
// project. Those have to land together or not at all — a project created with half its allocation
// is a §6 card that is wrong with nothing on the screen saying so — and "together or not at all"
// is one transaction, in one place.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here:
//
//	UNIQUE (tenant_id, ma), soft-deleted rows INCLUDED   0004:216-221
//	CHECK (ke_hoach_von_nam >= 0)                        0004:224-229
//	CHECK (nam BETWEEN 2000 AND 2100)                    0004:231-233
//	hard DELETE refused outright                         0004, `ho_so_luu_tru_cam_xoa_cung`
//	CHECK (so_tien_phan_bo >= 0)                         0007, phan_bo_nguon_von
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves: PostgreSQL answers with an exception whose text is
// English, names a constraint, and tells an accountant in a commune nothing they can act on. This
// layer refuses FIRST, in Vietnamese, naming the operation and the way out. A drift between the two
// is therefore a worse error message, never a hole — the constraint still runs last, and the whole
// transaction rolls back with the audit entry inside it.
//
// ---------------------------------------------------------------------------
// TWO QUESTIONS THIS FILE DELIBERATELY DOES NOT ANSWER, because they are the customer's:
//
//	the auto-generated code    §9's `☑ Tự sinh mã` is NOT implemented. The specification gives two
//	                          incompatible formats for one column and no scope for the sequence —
//	                          domain.ErrThieuMaDuAn sets out the whole argument. A project code is an
//	                          ISSUED CODE (rule 7, invariant 3 and forbidden #4), so a guessed format
//	                          is a commune whose codes can never be corrected.
//	removing a project that    refused, and store.ErrDuAnConChungTu states why refusal is the only
//	  still has vouchers       direction that writes nothing and can be loosened later with one
//	                          branch. ADR 0037 settled the same SHAPE for the task tree; that is a
//	                          different record and its answer does not carry over.
//
// Both are findings for the user, not decisions taken quietly here.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// KhoDuAn is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the project in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the allocation lines land with the project — provable without a
// PostgreSQL. There is none reachable from this repository's build environment, so a test that
// needed one would be a test that never runs.
type KhoDuAn interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.DuAn, error)
	MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error)
	HangMucConSong(ctx context.Context, tx *store.ScopedTx, hangMucID string) error
	NguonVonConSongTrongNam(ctx context.Context, tx *store.ScopedTx, nguonVonID string, nam int) error
	DemChungTuConSong(ctx context.Context, tx *store.ScopedTx, duAnID string) (int, error)
	Chen(ctx context.Context, tx *store.ScopedTx, d domain.DuAn) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, d domain.DuAn) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	ChenPhanBo(ctx context.Context, tx *store.ScopedTx, pb domain.PhanBoNguonVon) error
	XoaMemPhanBoCuaDuAn(ctx context.Context, tx *store.ScopedTx, duAnID, boi, lyDo string) (int, error)
}

// DuAn owns entering, correcting and removing one commune's investment projects.
type DuAn struct {
	db  *store.DB
	kho KhoDuAn

	// sinhID is injected so a test can pin the ids. In production it is ulid.Moi.
	//
	// IT IS CALLED ONCE PER ROW, INCLUDING ONCE PER ALLOCATION LINE. A single id reused across the
	// project and its lines would make `phan_bo_nguon_von.id` collide with `du_an.id`, which the
	// separate PRIMARY KEYs permit and which nothing would notice until somebody joined the two.
	sinhID func() (string, error)
}

func NewDuAn(db *store.DB, kho KhoDuAn) *DuAn {
	return &DuAn{db: db, kho: kho, sinhID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `them_chung_tu_giai_ngan`): an inspection reads these strings,
// and a function name would tell them nothing.
const (
	HanhViThemDuAn = "them_du_an"
	HanhViSuaDuAn  = "sua_du_an"
	HanhViXoaDuAn  = "xoa_du_an"
)

// YeuCauThemDuAn is one new project, as §9's modal supplies it.
//
// THE FIELD ORDER FOLLOWS §9's OWN, AND THAT ORDER IS BUSINESS RATHER THAN LAYOUT: the year's
// allocated amount comes FIRST and the split across sources SECOND, because the split is checked
// against the amount and not the other way round. Nothing here enforces that check — §9 calls it a
// WARNING ("cảnh báo khi thiếu hoặc vượt"), §11 turns the same two numbers into the
// `Đủ`/`Chưa đủ`/`Chưa gắn nguồn` chip, and both are things a SCREEN says. See PhanBo.
//
// THERE IS NO `Ma` DEFAULT AND NO GENERATOR. §9 offers `☑ Tự sinh mã`; domain.ErrThieuMaDuAn is the
// refusal and carries the whole argument for why guessing the format is the expensive move.
type YeuCauThemDuAn struct {
	// Ma is REQUIRED — see domain.ErrThieuMaDuAn.
	Ma  string
	Nam int

	HangMucID string
	Ten       string
	MoTa      string

	KeHoachVonNam    domain.Dong
	TongMucDuocDuyet domain.Dong // zero means "the same as this year's plan" (§9)

	DonViThucHienID string
	CanBoPhuTrachID string

	NgayKhoiCong  time.Time
	NgayHoanThanh time.Time

	// ThoiHanGiaiNgan is optional on the way in. A zero value becomes 31/12 of the budget year
	// (§11's "mặc định 31/12"), applied in ONE place by domain.HanGiaiNganMacDinh.
	ThoiHanGiaiNgan time.Time

	// PhanBo is §9's dynamic `Nguồn vốn` list, and it is OPTIONAL: *"Chưa gắn nguồn nào. Xã theo dõi
	// kế hoạch vốn theo hạng mục thì để trống cũng được."* A project with no line is a NORMAL project
	// and §11 names that state (`Chưa gắn nguồn`).
	//
	// ⚠ THE TOTAL IS NEVER COMPARED AGAINST KeHoachVonNam HERE, AND THAT MUST NOT CHANGE. §9 says the
	// system *warns* when the sources come to less or more than the allocated amount. A warning
	// refuses nothing. Turning it into a constraint would refuse the entry at the only moment the
	// modal is open — while the commune is still working the figures out — and §13 rule 6 requires
	// every rule here to survive a project with NO allocation at all.
	PhanBo []domain.DongPhanBoMoi
}

// YeuCauSuaDuAn is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Several fields have a meaningful zero — a description
// cleared to "", an officer unassigned back to "Chưa phân công", a plan revised down to 0 — and a
// struct of plain values cannot tell "the client did not mention this" from "the client cleared it".
// A dialog editing only the name would silently unassign the officer in charge.
//
// `Ma` AND `Nam` ARE ABSENT AND THAT IS NOT AN OVERSIGHT — the handler refuses a body naming either,
// with domain.ErrMaDuAnBatBien and domain.ErrNamBatBien, and `capNhatDuAn` has no column for them.
//
// `PhanBo` IS ABSENT TOO, AND THAT IS A STATED GAP RATHER THAN A DECISION. §9 puts the allocation
// list in the CREATE modal; §8's detail screen shows allocations read-only and offers no editor for
// them. Editing them means answering migration 0007's open question (b) — may one project hold two
// lines naming one source — which 0007 says outright is the customer's call and not this
// repository's. Creating is safe from that question because a brand-new project has no existing
// line to collide with, so ChuanHoaPhanBoMoi's per-request check settles it completely.
type YeuCauSuaDuAn struct {
	HangMucID *string
	Ten       *string
	MoTa      *string

	KeHoachVonNam    *domain.Dong
	TongMucDuocDuyet *domain.Dong

	DonViThucHienID *string
	CanBoPhuTrachID *string

	NgayKhoiCong    *time.Time
	NgayHoanThanh   *time.Time
	ThoiHanGiaiNgan *time.Time
}

// KetQuaThemDuAn is the project together with the allocation lines written beside it, so the caller
// can echo back exactly what landed without a second read.
type KetQuaThemDuAn struct {
	DuAn   domain.DuAn
	PhanBo []domain.PhanBoNguonVon
}

// Them records one new investment project, with its funding allocation lines if §9's list was filled
// in.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
func (uc *DuAn) Them(ctx context.Context, yc YeuCauThemDuAn,
	nguoi audit.Actor) (KetQuaThemDuAn, error) {

	ma, err := domain.ChuanHoaMaDuAn(yc.Ma)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	if err := domain.KiemTraNamDuAn(yc.Nam); err != nil {
		return KetQuaThemDuAn{}, err
	}
	hangMucID, err := domain.ChuanHoaHangMucID(yc.HangMucID)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	ten, err := domain.ChuanHoaTenDuAn(yc.Ten)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	moTa, err := domain.ChuanHoaMoTaDuAn(yc.MoTa)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	if err := domain.KiemTraKeHoachVon(yc.KeHoachVonNam); err != nil {
		return KetQuaThemDuAn{}, err
	}
	if err := domain.KiemTraTongMuc(yc.TongMucDuocDuyet); err != nil {
		return KetQuaThemDuAn{}, err
	}
	donVi, err := domain.ChuanHoaThamChieu(yc.DonViThucHienID)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	canBo, err := domain.ChuanHoaThamChieu(yc.CanBoPhuTrachID)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	for _, ngay := range []time.Time{yc.NgayKhoiCong, yc.NgayHoanThanh, yc.ThoiHanGiaiNgan} {
		if err := domain.KiemTraNgayDuAn(ngay); err != nil {
			return KetQuaThemDuAn{}, err
		}
	}
	phanBo, err := domain.ChuanHoaPhanBoMoi(yc.PhanBo)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return KetQuaThemDuAn{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return KetQuaThemDuAn{}, fmt.Errorf("du_an: sinh mã: %w", err)
	}

	// §11's "mặc định 31/12", APPLIED HERE AND NOWHERE ELSE. The column is `DATE NOT NULL` with no
	// database default, and a database default could only be derived from `now()` — which on
	// 02/01/2027 would stamp a 2026 project with a 2027 deadline.
	hanGiaiNgan := yc.ThoiHanGiaiNgan
	if hanGiaiNgan.IsZero() {
		hanGiaiNgan = domain.HanGiaiNganMacDinh(yc.Nam)
	}

	moi := domain.DuAn{
		ID:               id,
		Ma:               ma,
		Nam:              yc.Nam,
		HangMucID:        hangMucID,
		Ten:              ten,
		MoTa:             moTa,
		KeHoachVonNam:    yc.KeHoachVonNam,
		TongMucDuocDuyet: yc.TongMucDuocDuyet,
		DonViThucHienID:  donVi,
		CanBoPhuTrachID:  canBo,
		NgayKhoiCong:     yc.NgayKhoiCong,
		NgayHoanThanh:    yc.NgayHoanThanh,
		ThoiHanGiaiNgan:  hanGiaiNgan,
	}

	// THE IDS OF THE ALLOCATION LINES ARE MINTED OUTSIDE THE TRANSACTION, on purpose: generating an
	// id can fail, and a failure inside the transaction would roll back a project for a reason that
	// has nothing to do with the project.
	dongPhanBo := make([]domain.PhanBoNguonVon, 0, len(phanBo))
	for _, mot := range phanBo {
		pbID, err := uc.sinhID()
		if err != nil {
			return KetQuaThemDuAn{}, fmt.Errorf("du_an: sinh mã dòng phân bổ: %w", err)
		}
		dongPhanBo = append(dongPhanBo, domain.PhanBoNguonVon{
			ID:         pbID,
			DuAnID:     moi.ID,
			NguonVonID: mot.NguonVonID,
			SoTien:     mot.SoTien,
		})
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE CODE FIRST, BECAUSE IT IS THE REFUSAL THE COMMUNE CAN ACT ON WITHOUT KNOWING ANYTHING
		// ELSE. A taken code is a condition of ONE value the person just typed; a missing category is
		// a condition of the commune's configuration.
		daDung, err := uc.kho.MaDaDung(ctx, tx, moi.Ma)
		if err != nil {
			return err
		}
		if daDung {
			return fistore.ErrMaDuAnDaTonTai
		}

		// INSIDE THE TRANSACTION, because with no foreign key underneath (0004:182-190) this check IS
		// the constraint. A project classified under a category that does not exist is money counted
		// in §3's KPI card and missing from every row of §5's table — two totals on one screen that
		// disagree, with no row looking wrong.
		if err := uc.kho.HangMucConSong(ctx, tx, moi.HangMucID); err != nil {
			return err
		}

		// EVERY SOURCE IS CHECKED AGAINST THE PROJECT'S OWN YEAR. Not the calendar's, and not one the
		// client sent: `moi.Nam` is the year this project belongs to, and §13 rule 8 makes each year
		// its own set. There is no foreign key here either (0007:102-113), so this is the constraint.
		for _, pb := range dongPhanBo {
			if err := uc.kho.NguonVonConSongTrongNam(ctx, tx, pb.NguonVonID, moi.Nam); err != nil {
				return err
			}
		}

		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}
		// THE LINES LAND IN THE SAME TRANSACTION AS THE PROJECT. A project created with half its
		// allocation is a §6 card that is wrong — one source short — with nothing on any screen
		// saying so, and it would be indistinguishable from a commune that meant to allocate less.
		for _, pb := range dongPhanBo {
			if err := uc.kho.ChenPhanBo(ctx, tx, pb); err != nil {
				return err
			}
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatDuAn(moi, dongPhanBo)})
		if err != nil {
			return fmt.Errorf("du_an: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides which
		// commune the entry belongs to.
		//
		// THE SUBJECT IS THE PROJECT'S BUSINESS CODE, never the internal id (rule 6, invariant 8).
		// `DA-2026-…` names a record to somebody handling an inspection years later with no lookup
		// still alive; a ULID names nobody, and the row it points at may by then be gone.
		//
		// NOTHING IN THE DELTA IS PERSONAL DATA (rule 3): a project name, amounts of public money, and
		// two ids of rows owned by another service — not the names behind them.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViThemDuAn,
			Subject: moi.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no project, no allocation line, no trail. The states agree.
		return KetQuaThemDuAn{}, bocDuAn(ctx, "thêm", err)
	}
	return KetQuaThemDuAn{DuAn: moi, PhanBo: dongPhanBo}, nil
}

// Sua corrects one project — §8's `[✎ Sửa dự án]`.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a project the values it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// ⚠ A PROJECT WITH CONFIRMED OR LOCKED VOUCHERS IS EDITED NORMALLY, AND THAT IS NOT AN OVERSIGHT.
// ADR 0036 decided that a CONFIRMED VOUCHER returns to `Kế toán nhập` when ITS OWN figures move,
// because the leader confirmed those figures. A project has no `trang_thai` column and no
// confirmation on it, so there is nothing here for that rule to act on — carrying it across would be
// inventing a lifecycle the specification never gave this record. What the edit DOES do is change
// the denominator of every ratio on §3 and §8, which is exactly why the before/after pair below is
// the whole point of the entry.
func (uc *DuAn) Sua(ctx context.Context, id string, yc YeuCauSuaDuAn,
	nguoi audit.Actor) (domain.DuAn, error) {

	if id == "" {
		return domain.DuAn{}, fistore.ErrKhongThayDuAn
	}

	// Shape first, outside the transaction, for the same reason as Them.
	var hangMucID, ten, moTa, donVi, canBo string
	var err error
	if yc.HangMucID != nil {
		if hangMucID, err = domain.ChuanHoaHangMucID(*yc.HangMucID); err != nil {
			return domain.DuAn{}, err
		}
	}
	if yc.Ten != nil {
		if ten, err = domain.ChuanHoaTenDuAn(*yc.Ten); err != nil {
			return domain.DuAn{}, err
		}
	}
	if yc.MoTa != nil {
		if moTa, err = domain.ChuanHoaMoTaDuAn(*yc.MoTa); err != nil {
			return domain.DuAn{}, err
		}
	}
	if yc.DonViThucHienID != nil {
		if donVi, err = domain.ChuanHoaThamChieu(*yc.DonViThucHienID); err != nil {
			return domain.DuAn{}, err
		}
	}
	if yc.CanBoPhuTrachID != nil {
		if canBo, err = domain.ChuanHoaThamChieu(*yc.CanBoPhuTrachID); err != nil {
			return domain.DuAn{}, err
		}
	}
	if yc.KeHoachVonNam != nil {
		if err := domain.KiemTraKeHoachVon(*yc.KeHoachVonNam); err != nil {
			return domain.DuAn{}, err
		}
	}
	if yc.TongMucDuocDuyet != nil {
		if err := domain.KiemTraTongMuc(*yc.TongMucDuocDuyet); err != nil {
			return domain.DuAn{}, err
		}
	}
	for _, ngay := range []*time.Time{yc.NgayKhoiCong, yc.NgayHoanThanh, yc.ThoiHanGiaiNgan} {
		if ngay == nil {
			continue
		}
		if err := domain.KiemTraNgayDuAn(*ngay); err != nil {
			return domain.DuAn{}, err
		}
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.DuAn{}, err
	}

	var sau domain.DuAn
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		if yc.HangMucID != nil {
			sau.HangMucID = hangMucID
		}
		if yc.Ten != nil {
			sau.Ten = ten
		}
		if yc.MoTa != nil {
			sau.MoTa = moTa
		}
		if yc.KeHoachVonNam != nil {
			sau.KeHoachVonNam = *yc.KeHoachVonNam
		}
		if yc.TongMucDuocDuyet != nil {
			sau.TongMucDuocDuyet = *yc.TongMucDuocDuyet
		}
		if yc.DonViThucHienID != nil {
			sau.DonViThucHienID = donVi
		}
		if yc.CanBoPhuTrachID != nil {
			sau.CanBoPhuTrachID = canBo
		}
		if yc.NgayKhoiCong != nil {
			sau.NgayKhoiCong = *yc.NgayKhoiCong
		}
		if yc.NgayHoanThanh != nil {
			sau.NgayHoanThanh = *yc.NgayHoanThanh
		}
		if yc.ThoiHanGiaiNgan != nil {
			// A CLEARED DEADLINE FALLS BACK TO 31/12, NOT TO THE ZERO TIME. The column is NOT NULL, so
			// there is no "unset" to go back to; §11's default is what "no particular date" means for
			// this field, and it is applied by the same function the create path uses.
			if yc.ThoiHanGiaiNgan.IsZero() {
				sau.ThoiHanGiaiNgan = domain.HanGiaiNganMacDinh(truoc.Nam)
			} else {
				sau.ThoiHanGiaiNgan = *yc.ThoiHanGiaiNgan
			}
		}

		if khongDoiDuAn(truoc, sau) {
			return nil
		}

		// CHECKED ONLY WHEN THE CATEGORY ACTUALLY MOVES. Correcting a project's name must not fail
		// because the category it has always been classified under was removed from the catalogue
		// afterwards — the row is the historical fact, and refusing to edit anything else would strand
		// it. What IS refused is reclassifying into a category that does not exist in this commune.
		if sau.HangMucID != truoc.HangMucID {
			if err := uc.kho.HangMucConSong(ctx, tx, sau.HangMucID); err != nil {
				return err
			}
		}
		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		//
		// `ke_hoach_von_nam` IS THE FIELD THIS DELTA EXISTS FOR. It is the denominator of the
		// disbursement ratio on §7.2 and of the delay score on §3, so revising it silently moves a
		// project from "chậm 31,36 điểm" to "bám sát tiến độ" without one đồng having moved. The pair
		// of numbers in an append-only ledger is the only thing that tells that apart afterwards.
		delta, err := json.Marshal(map[string]any{
			"du_an_id": sau.ID,
			"truoc":    tomTatDoiDuAn(truoc, sau, true),
			"sau":      tomTatDoiDuAn(truoc, sau, false),
		})
		if err != nil {
			return fmt.Errorf("du_an: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaDuAn,
			Subject: truoc.Ma, // the business code; `ma` cannot change, so before and after agree
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.DuAn{}, bocDuAn(ctx, "sửa", err)
	}
	return sau, nil
}

// Xoa soft deletes one project together with its funding allocation lines.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its CODE STAYS
// TAKEN FOREVER — `UNIQUE (tenant_id, ma)` counts removed rows, which is exactly what §9 promises
// about a project "đã rút khỏi danh sách". `ho_so_luu_tru_cam_xoa_cung` refuses a hard DELETE
// outright.
//
// ⚠ A PROJECT THAT STILL HAS LIVE VOUCHERS IS REFUSED, AND THAT IS AN OPEN QUESTION ANSWERED IN THE
// ONLY DIRECTION THAT WRITES NOTHING. store.ErrDuAnConChungTu sets out the three candidate answers
// and why the other two are one-way. It is a finding for the user, not a rule this repository is
// entitled to fix permanently.
func (uc *DuAn) Xoa(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return fistore.ErrKhongThayDuAn
	}
	lyDo, err := domain.ChuanHoaLyDoXoaDuAn(lyDoTho)
	if err != nil {
		return err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		// COUNTED UNDER THE PROJECT'S ROW LOCK, which is what makes the answer still true when the
		// UPDATE runs. Without it a voucher entered between the count and the removal would land on a
		// project that is being removed, and its money would be stranded exactly as this refusal
		// exists to prevent.
		soChungTu, err := uc.kho.DemChungTuConSong(ctx, tx, truoc.ID)
		if err != nil {
			return err
		}
		if soChungTu > 0 {
			return fistore.ErrDuAnConChungTu
		}

		// THE ALLOCATION LINES GO FIRST, and the order is not arbitrary: they are read through
		// `du_an_id`, and removing the project first would leave a window — inside this transaction,
		// but visible to the statement that follows — where a line points at a project no read path
		// returns. Both statements are in one transaction so nothing outside ever sees either state,
		// and the order keeps the code honest for a reader.
		//
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE on both, the same value the entry's actor holds.
		// Two kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		soDongPhanBo, err := uc.kho.XoaMemPhanBoCuaDuAn(ctx, tx, truoc.ID, nguoi.ID, lyDo)
		if err != nil {
			return err
		}
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication rule 9
		// forbids: the column is the current state of the row and can only ever hold the FIRST
		// removal, while the entry is the append-only record of the act. They answer two different
		// questions and neither can be derived from the other.
		//
		// `so_dong_phan_bo` IS RECORDED BECAUSE NOTHING ELSE CAN REBUILD IT. Once the lines carry
		// `deleted_at`, "how many sources was this project drawing on when it was withdrawn" has no
		// other source — and it is the figure that explains why a §6 card's "đã phân bổ" dropped.
		delta, err := json.Marshal(map[string]any{
			"du_an_id":        truoc.ID,
			"truoc":           tomTatDuAn(truoc, nil),
			"ly_do":           lyDo,
			"so_dong_phan_bo": soDongPhanBo,
			"xoa_mem":         true,
		})
		if err != nil {
			return fmt.Errorf("du_an: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViXoaDuAn,
			Subject: truoc.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return bocDuAn(ctx, "xoá", err)
	}
	return nil
}

// bocDuAn wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No amount, no project name, no description: an error travels into centralised logging across every
// commune at once, and a project's description is somebody's typing about a public authority's
// spending. The commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this code is taken" and "the database is down" into one 500.
func bocDuAn(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("du_an: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}

// tomTatDuAn is the audit delta's view of one project.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a code, a name, amounts of public money, dates, and two
// ids of rows owned by another service — not the names behind them. Resolving `don_vi_thuc_hien_id`
// or `can_bo_phu_trach_id` to a person is another service's job under another permission, and doing
// it here would put a staff member's name into an append-only ledger nobody can edit afterwards.
//
// `so_tien_phan_bo` IS SUMMED AND THE LINES ARE LISTED, both: the total is what §6 and §11 compare
// against the plan, and the per-source breakdown is what an inspection asks for by name when a
// source's card does not add up.
func tomTatDuAn(d domain.DuAn, phanBo []domain.PhanBoNguonVon) map[string]any {
	ra := map[string]any{
		"du_an_id":            d.ID,
		"ma":                  d.Ma,
		"nam":                 d.Nam,
		"hang_muc_id":         d.HangMucID,
		"ten":                 d.Ten,
		"ke_hoach_von_nam":    int64(d.KeHoachVonNam),
		"tong_muc_duoc_duyet": int64(d.TongMucDuocDuyet),
		"thoi_han_giai_ngan":  d.ThoiHanGiaiNgan.Format("2006-01-02"),
	}
	if len(phanBo) == 0 {
		return ra
	}
	var tong domain.Dong
	dong := make([]map[string]any, 0, len(phanBo))
	for _, pb := range phanBo {
		tong += pb.SoTien
		dong = append(dong, map[string]any{
			"nguon_von_id": pb.NguonVonID,
			"so_tien":      int64(pb.SoTien),
		})
	}
	ra["phan_bo_nguon_von"] = dong
	ra["tong_phan_bo"] = int64(tong)
	return ra
}

// tomTatDoiDuAn returns only the fields that actually moved, from whichever side is asked for.
func tomTatDoiDuAn(truoc, sau domain.DuAn, ben bool) map[string]any {
	ra := map[string]any{}
	if truoc.HangMucID != sau.HangMucID {
		ra["hang_muc_id"] = chon(ben, truoc.HangMucID, sau.HangMucID)
	}
	if truoc.Ten != sau.Ten {
		ra["ten"] = chon(ben, truoc.Ten, sau.Ten)
	}
	if truoc.MoTa != sau.MoTa {
		ra["mo_ta"] = chon(ben, truoc.MoTa, sau.MoTa)
	}
	if truoc.KeHoachVonNam != sau.KeHoachVonNam {
		ra["ke_hoach_von_nam"] = int64(chon(ben, truoc.KeHoachVonNam, sau.KeHoachVonNam))
	}
	if truoc.TongMucDuocDuyet != sau.TongMucDuocDuyet {
		ra["tong_muc_duoc_duyet"] = int64(chon(ben, truoc.TongMucDuocDuyet, sau.TongMucDuocDuyet))
	}
	if truoc.DonViThucHienID != sau.DonViThucHienID {
		ra["don_vi_thuc_hien_id"] = chon(ben, truoc.DonViThucHienID, sau.DonViThucHienID)
	}
	if truoc.CanBoPhuTrachID != sau.CanBoPhuTrachID {
		ra["can_bo_phu_trach_id"] = chon(ben, truoc.CanBoPhuTrachID, sau.CanBoPhuTrachID)
	}
	if !truoc.NgayKhoiCong.Equal(sau.NgayKhoiCong) {
		ra["ngay_khoi_cong"] = ngayDelta(chon(ben, truoc.NgayKhoiCong, sau.NgayKhoiCong))
	}
	if !truoc.NgayHoanThanh.Equal(sau.NgayHoanThanh) {
		ra["ngay_hoan_thanh"] = ngayDelta(chon(ben, truoc.NgayHoanThanh, sau.NgayHoanThanh))
	}
	if !truoc.ThoiHanGiaiNgan.Equal(sau.ThoiHanGiaiNgan) {
		ra["thoi_han_giai_ngan"] = ngayDelta(chon(ben, truoc.ThoiHanGiaiNgan, sau.ThoiHanGiaiNgan))
	}
	return ra
}

// ngayDelta renders a date for the trail, spelling "not set" as an empty string rather than as
// 01/01/0001 — which reads as a real date to somebody reading the ledger years later.
func ngayDelta(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// khongDoiDuAn reports whether the edit would change nothing.
//
// ⚠ EVERY EDITABLE FIELD MUST BE LISTED HERE, AND A MISSING ONE FAILS IN SILENCE. Sua returns early
// when this says "nothing moved" — so a field that is editable but unlisted is a field whose edit
// writes NO row and leaves NO audit entry. The request answers 200 with the OLD values and nothing
// anywhere is red. `ma` and `nam` are absent because no path through Sua can change them.
func khongDoiDuAn(truoc, sau domain.DuAn) bool {
	return truoc.HangMucID == sau.HangMucID &&
		truoc.Ten == sau.Ten &&
		truoc.MoTa == sau.MoTa &&
		truoc.KeHoachVonNam == sau.KeHoachVonNam &&
		truoc.TongMucDuocDuyet == sau.TongMucDuocDuyet &&
		truoc.DonViThucHienID == sau.DonViThucHienID &&
		truoc.CanBoPhuTrachID == sau.CanBoPhuTrachID &&
		truoc.NgayKhoiCong.Equal(sau.NgayKhoiCong) &&
		truoc.NgayHoanThanh.Equal(sau.NgayHoanThanh) &&
		truoc.ThoiHanGiaiNgan.Equal(sau.ThoiHanGiaiNgan)
}
