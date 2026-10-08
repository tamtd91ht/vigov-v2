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
// THE AUTO-ISSUED CODE (§9's `☑ Tự sinh mã`) WAS DECIDED BY THE USER ON 06/10/2026: a blank `code`
// is issued the next number of ONE per-commune series, DA01, DA02 … DA100 (domain.FormatProjectSerial,
// the prototype's format). See issueProjectSerial for how "never reissued" holds.
//
// ONE QUESTION THIS FILE DELIBERATELY DOES NOT ANSWER, because it is the customer's:
//
//	removing a project that    refused, and store.ErrDuAnConChungTu states why refusal is the only
//	  still has vouchers       direction that writes nothing and can be loosened later with one
//	                          branch. ADR 0037 settled the same SHAPE for the task tree; that is a
//	                          different record and its answer does not carry over.

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
	LockProjectSerial(ctx context.Context, tx *store.ScopedTx) (int64, error)
	AdvanceProjectSerial(ctx context.Context, tx *store.ScopedTx, next int64) error
	HangMucConSong(ctx context.Context, tx *store.ScopedTx, hangMucID string) error
	NguonVonConSongTrongNam(ctx context.Context, tx *store.ScopedTx, nguonVonID string) error
	DemChungTuConSong(ctx context.Context, tx *store.ScopedTx, duAnID string) (int, error)
	Chen(ctx context.Context, tx *store.ScopedTx, d domain.DuAn) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, d domain.DuAn) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	ChenPhanBo(ctx context.Context, tx *store.ScopedTx, pb domain.PhanBoNguonVon) error
	XoaMemPhanBoCuaDuAn(ctx context.Context, tx *store.ScopedTx, duAnID, boi, lyDo string) (int, error)

	// Editing the allocation set (PATCH `funding_allocations`, decision 06/10/2026).
	AllocationLinesForEdit(ctx context.Context, tx *store.ScopedTx, projectID string) ([]domain.StoredAllocationLine, error)
	VoucherCountBySource(ctx context.Context, tx *store.ScopedTx, projectID string) (map[string]int, error)
	UpdateAllocationAmount(ctx context.Context, tx *store.ScopedTx, lineID string, amount domain.Dong) error
	ReviveAllocation(ctx context.Context, tx *store.ScopedTx, lineID string, amount domain.Dong) error
	SoftDeleteAllocation(ctx context.Context, tx *store.ScopedTx, lineID, by, reason string) error
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
type YeuCauThemDuAn struct {
	// Ma is OPTIONAL: blank (after trimming) means "issue the next code of this commune's series"
	// (§9's `☑ Tự sinh mã`). A typed code must never have been used, removed projects included.
	Ma  string
	Nam int

	HangMucID string
	Ten       string
	MoTa      string

	KeHoachVonNam    domain.Dong
	TongMucDuocDuyet domain.Dong // zero means "the same as this year's plan" (§9)

	DonViThucHienID string
	CanBoPhuTrachID string

	// ImplementingUnit is "Đơn vị thực hiện" as typed (0016). Optional: blank = not named.
	ImplementingUnit string

	NgayKhoiCong  time.Time
	NgayHoanThanh time.Time

	// ThoiHanGiaiNgan is optional on the way in. A zero value becomes 31/12 of the budget year
	// (§11's "mặc định 31/12"), applied in ONE place by domain.HanGiaiNganMacDinh.
	ThoiHanGiaiNgan time.Time

	// PhanBo is §9's dynamic `Nguồn vốn` list, and it is OPTIONAL: *"Chưa gắn nguồn nào. Xã theo dõi
	// kế hoạch vốn theo hạng mục thì để trống cũng được."* A project with no line is a NORMAL project
	// and §11 names that state (`Chưa gắn nguồn`).
	//
	// ITS TOTAL MAY NOT EXCEED KeHoachVonNam (decision 06/10/2026, prototype service.py:607-610) —
	// domain.CheckAllocationWithinPlan. Less is allowed: the remainder is the chip's shortfall.
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
// `PhanBo` IS THE FULL REPLACEMENT SET OF ALLOCATION LINES (decision 06/10/2026): nil = leave the lines
// alone; non-nil = these lines and no others (an empty slice removes every line). Kept sources are
// UPDATED IN PLACE, re-added sources REVIVE their old row, removed ones are soft deleted — never
// soft-delete-then-reinsert, because `UNIQUE (tenant_id, du_an_id, nguon_von_id)` (0013) counts
// soft-deleted rows. See domain.PlanAllocationReplacement.
type YeuCauSuaDuAn struct {
	HangMucID *string
	Ten       *string
	MoTa      *string

	KeHoachVonNam    *domain.Dong
	TongMucDuocDuyet *domain.Dong

	DonViThucHienID *string
	CanBoPhuTrachID *string

	// ImplementingUnit: nil = leave alone, a pointer to "" (after trimming) = clear to NULL.
	ImplementingUnit *string

	NgayKhoiCong    *time.Time
	NgayHoanThanh   *time.Time
	ThoiHanGiaiNgan *time.Time

	// AtRisk: nil = leave alone; otherwise tick / untick "nguy cơ không giải ngân hết" (ADR 0080 #2,
	// migration 0018). Same permission as every other field of this edit — budget.update.
	AtRisk *bool

	PhanBo *[]domain.DongPhanBoMoi
}

// ProjectEditResult is the project after an edit together with its LIVE allocation lines after it, so
// the PATCH reply echoes what the project now draws on without a second read.
type ProjectEditResult struct {
	DuAn        domain.DuAn
	Allocations []domain.PhanBoNguonVon
}

// allocationRemovedReason is `delete_reason` on a line an edit drops. FIXED TEXT: the PATCH body has
// no reason field (the prototype's has none either), and rule 7 invariant 1 still wants the column
// filled. The act itself — who, when, which lines before and after — is the `sua_du_an` audit entry.
const allocationRemovedReason = "gỡ nguồn vốn khỏi dự án khi sửa phân bổ nguồn vốn"

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

	// A BLANK CODE IS A REQUEST FOR THE NEXT SERIAL, not a missing field (user decision 06/10/2026).
	// The number is chosen INSIDE the transaction, under the counter's row lock; a typed code is
	// validated here, before anything opens.
	autoCode := strings.TrimSpace(yc.Ma) == ""
	var ma string
	if !autoCode {
		var err error
		if ma, err = domain.ChuanHoaMaDuAn(yc.Ma); err != nil {
			return KetQuaThemDuAn{}, err
		}
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
	implementingUnit, err := domain.NormaliseImplementingUnit(yc.ImplementingUnit)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	for _, ngay := range []time.Time{yc.NgayKhoiCong, yc.NgayHoanThanh, yc.ThoiHanGiaiNgan} {
		if err := domain.KiemTraNgayDuAn(ngay); err != nil {
			return KetQuaThemDuAn{}, err
		}
	}
	if err := domain.CheckProjectDateOrder(yc.NgayKhoiCong, yc.NgayHoanThanh); err != nil {
		return KetQuaThemDuAn{}, err
	}
	phanBo, err := domain.ChuanHoaPhanBoMoi(yc.PhanBo)
	if err != nil {
		return KetQuaThemDuAn{}, err
	}
	if err := domain.CheckAllocationWithinPlan(yc.KeHoachVonNam, totalOfNewLines(phanBo)); err != nil {
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
		ImplementingUnit: implementingUnit,
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
		if autoCode {
			issued, err := uc.issueProjectSerial(ctx, tx)
			if err != nil {
				return err
			}
			moi.Ma = issued
		} else {
			daDung, err := uc.kho.MaDaDung(ctx, tx, moi.Ma)
			if err != nil {
				return err
			}
			if daDung {
				return fistore.ErrMaDuAnDaTonTai
			}
		}

		// INSIDE THE TRANSACTION, because with no foreign key underneath (0004:182-190) this check IS
		// the constraint. A project classified under a category that does not exist is money counted
		// in §3's KPI card and missing from every row of §5's table — two totals on one screen that
		// disagree, with no row looking wrong.
		if err := uc.kho.HangMucConSong(ctx, tx, moi.HangMucID); err != nil {
			return err
		}

		// EVERY SOURCE MUST BE A LIVE SOURCE OF THIS COMMUNE'S CATALOGUE. Not of a year: since
		// migration 0013 a source is shared by every budget year (user decision 06/10/2026), and the
		// year this allocation counts in is `moi.Nam`, which the §6 read takes from the project. There
		// is no foreign key here either (0007:102-113), so this is the constraint.
		for _, pb := range dongPhanBo {
			if err := uc.kho.NguonVonConSongTrongNam(ctx, tx, pb.NguonVonID); err != nil {
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

		// `auto_code` SAYS WHO CHOSE THE CODE. A system-issued code and a typed one look identical in
		// the row; years later the question "why is this project DA07" has two different answers.
		themDelta := map[string]any{"sau": tomTatDuAn(moi, dongPhanBo)}
		if autoCode {
			themDelta["auto_code"] = true
		}
		delta, err := json.Marshal(themDelta)
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

// issueProjectSerial picks the next free code of this commune's series, inside the create transaction.
//
// WHY "NEVER REISSUED" HOLDS (rule 7, invariant 3): each candidate is checked with MaDaDung, which
// counts SOFT-DELETED projects, so a code a removed project carries is stepped over exactly like a
// live one — and `UNIQUE (tenant_id, ma)` is the floor under that check. The counter only decides
// where to START; it is not what makes the answer correct.
//
// WHY TWO CLERKS DO NOT GET THE SAME NUMBER: LockProjectSerial holds the commune's counter row until
// this transaction ends, so a concurrent auto-coded create waits, then starts from the advanced value.
// (A concurrent create with a TYPED code equal to the candidate is not serialised by this lock; the
// unique key refuses one of the two, which rolls that one back.)
//
// CODES TYPED BY HAND ARE STEPPED OVER, not jumped to: a commune that typed `DA05` gets DA01..DA04,
// then DA06. Bounded by domain.ProjectSerialSkipLimit because it runs under a row lock.
func (uc *DuAn) issueProjectSerial(ctx context.Context, tx *store.ScopedTx) (string, error) {
	n, err := uc.kho.LockProjectSerial(ctx, tx)
	if err != nil {
		return "", err
	}
	for i := 0; i < domain.ProjectSerialSkipLimit; i, n = i+1, n+1 {
		candidate := domain.FormatProjectSerial(n)
		taken, err := uc.kho.MaDaDung(ctx, tx, candidate)
		if err != nil {
			return "", err
		}
		if taken {
			continue
		}
		if err := uc.kho.AdvanceProjectSerial(ctx, tx, n+1); err != nil {
			return "", err
		}
		return candidate, nil
	}
	return "", domain.ErrProjectSerialExhausted
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
	nguoi audit.Actor) (ProjectEditResult, error) {

	if id == "" {
		return ProjectEditResult{}, fistore.ErrKhongThayDuAn
	}

	// Shape first, outside the transaction, for the same reason as Them.
	var hangMucID, ten, moTa, donVi, canBo, implementingUnit string
	var err error
	if yc.ImplementingUnit != nil {
		if implementingUnit, err = domain.NormaliseImplementingUnit(*yc.ImplementingUnit); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.HangMucID != nil {
		if hangMucID, err = domain.ChuanHoaHangMucID(*yc.HangMucID); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.Ten != nil {
		if ten, err = domain.ChuanHoaTenDuAn(*yc.Ten); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.MoTa != nil {
		if moTa, err = domain.ChuanHoaMoTaDuAn(*yc.MoTa); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.DonViThucHienID != nil {
		if donVi, err = domain.ChuanHoaThamChieu(*yc.DonViThucHienID); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.CanBoPhuTrachID != nil {
		if canBo, err = domain.ChuanHoaThamChieu(*yc.CanBoPhuTrachID); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.KeHoachVonNam != nil {
		if err := domain.KiemTraKeHoachVon(*yc.KeHoachVonNam); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if yc.TongMucDuocDuyet != nil {
		if err := domain.KiemTraTongMuc(*yc.TongMucDuocDuyet); err != nil {
			return ProjectEditResult{}, err
		}
	}
	for _, ngay := range []*time.Time{yc.NgayKhoiCong, yc.NgayHoanThanh, yc.ThoiHanGiaiNgan} {
		if ngay == nil {
			continue
		}
		if err := domain.KiemTraNgayDuAn(*ngay); err != nil {
			return ProjectEditResult{}, err
		}
	}
	// THE REPLACEMENT SET IS VALIDATED HERE (shape, duplicates, bounds); WHETHER IT FITS THE PLAN is
	// decided inside the transaction, against the MERGED plan — the stored one, or the one this same
	// PATCH revises.
	var wanted []domain.DongPhanBoMoi
	if yc.PhanBo != nil {
		if wanted, err = domain.ChuanHoaPhanBoMoi(*yc.PhanBo); err != nil {
			return ProjectEditResult{}, err
		}
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return ProjectEditResult{}, err
	}
	// Ids for lines this edit may INSERT, minted outside the transaction for the reason Them gives.
	// One per wanted line; the ones a kept or revived source does not need are simply not used.
	newIDs := make([]string, 0, len(wanted))
	for range wanted {
		pbID, err := uc.sinhID()
		if err != nil {
			return ProjectEditResult{}, fmt.Errorf("du_an: sinh mã dòng phân bổ: %w", err)
		}
		newIDs = append(newIDs, pbID)
	}

	var kq ProjectEditResult
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		sau := truoc
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
		if yc.ImplementingUnit != nil {
			sau.ImplementingUnit = implementingUnit
		}
		if yc.AtRisk != nil {
			sau.AtRisk = *yc.AtRisk
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

		// AGAINST THE MERGED ROW, because a PATCH may name only one of the two dates and the other one
		// is whatever is stored. CHECKED ONLY WHEN A DATE IS NAMED: a legacy row already out of order
		// must still accept a name correction, the same reasoning as the category check below.
		if yc.NgayKhoiCong != nil || yc.NgayHoanThanh != nil {
			if err := domain.CheckProjectDateOrder(sau.NgayKhoiCong, sau.NgayHoanThanh); err != nil {
				return err
			}
		}

		// THE LINES, LIVE AND REMOVED, read under the project's lock — every writer of a project's
		// lines holds that lock, so what is read here is still true at the writes below. Read on every
		// edit, so the reply can echo what the project draws on after it.
		stored, err := uc.kho.AllocationLinesForEdit(ctx, tx, truoc.ID)
		if err != nil {
			return err
		}
		lineBefore := domain.LiveAllocationLines(stored)
		lineAfter := lineBefore
		var repl domain.AllocationReplacement
		if yc.PhanBo != nil {
			repl = domain.PlanAllocationReplacement(stored, wanted)
			lineAfter = linesAfterReplacement(stored, wanted, truoc.ID, newIDs)
		}

		// OVER-ALLOCATION IS REFUSED (decision 06/10/2026) on the two edits that can produce it: a new
		// allocation set, and a plan revised DOWN below what is already allocated. Neither named — a
		// name correction on a legacy row already over its plan — is not refused: that row is the
		// historical fact, the same reasoning as the category check below.
		if yc.PhanBo != nil || sau.KeHoachVonNam < truoc.KeHoachVonNam {
			if err := domain.CheckAllocationWithinPlan(sau.KeHoachVonNam,
				domain.TotalOfLines(lineAfter)); err != nil {
				return err
			}
		}

		projectMoved := !khongDoiDuAn(truoc, sau)
		if !projectMoved && repl.Empty() {
			kq = ProjectEditResult{DuAn: truoc, Allocations: lineBefore}
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

		if !repl.Empty() {
			if err := uc.applyAllocationReplacement(ctx, tx, truoc.ID, repl, lineAfter, nguoi); err != nil {
				return err
			}
		}
		if projectMoved {
			if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
				return err
			}
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		//
		// `ke_hoach_von_nam` IS THE FIELD THIS DELTA EXISTS FOR. It is the denominator of the
		// disbursement ratio on §7.2 and of the delay score on §3, so revising it silently moves a
		// project from "chậm 31,36 điểm" to "bám sát tiến độ" without one đồng having moved. The pair
		// of numbers in an append-only ledger is the only thing that tells that apart afterwards.
		//
		// THE ALLOCATION LINES, WHEN THEY MOVED, ARE LISTED IN FULL ON BOTH SIDES — source id and
		// amount, plus the total — because "which sources did this project draw on before the edit"
		// has no other source once a line is soft deleted or revived.
		deltaBefore := tomTatDoiDuAn(truoc, sau, true)
		deltaAfter := tomTatDoiDuAn(truoc, sau, false)
		if !repl.Empty() {
			deltaBefore["phan_bo_nguon_von"], deltaBefore["tong_phan_bo"] = allocationDelta(lineBefore)
			deltaAfter["phan_bo_nguon_von"], deltaAfter["tong_phan_bo"] = allocationDelta(lineAfter)
		}
		delta, err := json.Marshal(map[string]any{
			"du_an_id": sau.ID,
			"truoc":    deltaBefore,
			"sau":      deltaAfter,
		})
		if err != nil {
			return fmt.Errorf("du_an: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaDuAn,
			Subject: truoc.Ma, // the business code; `ma` cannot change, so before and after agree
			Delta:   delta,
		}); err != nil {
			return err
		}
		kq = ProjectEditResult{DuAn: sau, Allocations: lineAfter}
		return nil
	})
	if err != nil {
		return ProjectEditResult{}, bocDuAn(ctx, "sửa", err)
	}
	return kq, nil
}

// applyAllocationReplacement writes one replacement, inside the edit's transaction.
//
// THE VOUCHER GUARD COMES FIRST AND WRITES NOTHING: a source that already has live vouchers of THIS
// project cannot be dropped (domain.ErrSourceHasDisbursements). The count is asked only when the edit
// actually removes a source, so an edit that only moves amounts never touches the voucher table.
//
// EVERY SOURCE THE PROJECT GAINS — inserted or revived — MUST BE A LIVE SOURCE OF THIS COMMUNE'S
// CATALOGUE, the same check the create path makes (no foreign key underneath, 0007:102-113). Kept
// sources are not re-checked, for the category reasoning: the line is the historical fact.
func (uc *DuAn) applyAllocationReplacement(ctx context.Context, tx *store.ScopedTx, projectID string,
	repl domain.AllocationReplacement, lineAfter []domain.PhanBoNguonVon, nguoi audit.Actor) error {

	if len(repl.Remove) > 0 {
		counts, err := uc.kho.VoucherCountBySource(ctx, tx, projectID)
		if err != nil {
			return err
		}
		for _, r := range repl.Remove {
			if counts[r.NguonVonID] > 0 {
				return domain.ErrSourceHasDisbursements
			}
		}
	}
	for _, r := range repl.Revive {
		if err := uc.kho.NguonVonConSongTrongNam(ctx, tx, r.NguonVonID); err != nil {
			return err
		}
	}
	for _, w := range repl.Insert {
		if err := uc.kho.NguonVonConSongTrongNam(ctx, tx, w.NguonVonID); err != nil {
			return err
		}
	}

	// `deleted_by` IS THE STAFF BUSINESS CODE, the value the entry's actor carries (rule 6, inv. 8).
	for _, r := range repl.Remove {
		if err := uc.kho.SoftDeleteAllocation(ctx, tx, r.ID, nguoi.ID, allocationRemovedReason); err != nil {
			return err
		}
	}
	for _, u := range repl.Update {
		if err := uc.kho.UpdateAllocationAmount(ctx, tx, u.ID, u.SoTien); err != nil {
			return err
		}
	}
	for _, r := range repl.Revive {
		if err := uc.kho.ReviveAllocation(ctx, tx, r.ID, r.SoTien); err != nil {
			return err
		}
	}
	inserted := make(map[string]struct{}, len(repl.Insert))
	for _, w := range repl.Insert {
		inserted[w.NguonVonID] = struct{}{}
	}
	for _, l := range lineAfter {
		if _, ok := inserted[l.NguonVonID]; !ok {
			continue
		}
		if err := uc.kho.ChenPhanBo(ctx, tx, l); err != nil {
			return err
		}
	}
	return nil
}

// linesAfterReplacement is the project's live line set once the replacement lands, in the order the
// request listed the sources. A stored row keeps its id (updated or revived); a new source takes the
// next pre-minted id.
func linesAfterReplacement(stored []domain.StoredAllocationLine, wanted []domain.DongPhanBoMoi,
	projectID string, newIDs []string) []domain.PhanBoNguonVon {

	bySource := make(map[string]domain.StoredAllocationLine, len(stored))
	for _, s := range stored {
		bySource[s.NguonVonID] = s
	}
	out := make([]domain.PhanBoNguonVon, 0, len(wanted))
	next := 0
	for _, w := range wanted {
		line := domain.PhanBoNguonVon{DuAnID: projectID, NguonVonID: w.NguonVonID, SoTien: w.SoTien}
		if s, ok := bySource[w.NguonVonID]; ok {
			line.ID = s.ID
		} else {
			line.ID = newIDs[next]
			next++
		}
		out = append(out, line)
	}
	return out
}

// allocationDelta is the audit view of a line set: each source with its amount, and the total. No
// personal data (rule 3) — ids of catalogue rows and amounts of public money.
func allocationDelta(lines []domain.PhanBoNguonVon) ([]map[string]any, int64) {
	out := make([]map[string]any, 0, len(lines))
	for _, l := range lines {
		out = append(out, map[string]any{"nguon_von_id": l.NguonVonID, "so_tien": int64(l.SoTien)})
	}
	return out, int64(domain.TotalOfLines(lines))
}

// totalOfNewLines totals the lines of a create request (saturating — domain.SumAllocations).
func totalOfNewLines(lines []domain.DongPhanBoMoi) domain.Dong {
	amounts := make([]domain.Dong, 0, len(lines))
	for _, l := range lines {
		amounts = append(amounts, l.SoTien)
	}
	return domain.SumAllocations(amounts...)
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
	if d.ImplementingUnit != "" {
		ra["implementing_unit"] = d.ImplementingUnit
	}
	if d.AtRisk {
		ra["at_risk"] = true // a removed project's flag is part of what was removed (0018)
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
	// Free text of the same class as a voucher's `doi_tac` (0016's header): fine in the delta, which is
	// the commune's own record of who changed what; never in a log line (rule 3).
	if truoc.ImplementingUnit != sau.ImplementingUnit {
		ra["implementing_unit"] = chon(ben, truoc.ImplementingUnit, sau.ImplementingUnit)
	}
	// A leader's judgement on the project (ADR 0080 #2): who ticked or unticked it, and when, is the
	// whole point of the trail — the flag itself carries no reason.
	if truoc.AtRisk != sau.AtRisk {
		ra["at_risk"] = chon(ben, truoc.AtRisk, sau.AtRisk)
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
		truoc.ImplementingUnit == sau.ImplementingUnit &&
		truoc.AtRisk == sau.AtRisk &&
		truoc.NgayKhoiCong.Equal(sau.NgayKhoiCong) &&
		truoc.NgayHoanThanh.Equal(sau.NgayHoanThanh) &&
		truoc.ThoiHanGiaiNgan.Equal(sau.ThoiHanGiaiNgan)
}
