package domain

// Funding sources, and the allocation of a project's year plan across them (§6, §9, §11).
//
// EVERYTHING IN THIS FILE IS DERIVED AND NOTHING IS STORED. `tong_nguon` is a ceiling the commune
// types; every other figure on §6's card — allocated, remaining, disbursed, and the three ratios —
// is computed from rows on each read. The reasoning is the one 0004 gives for refusing a
// `da_giai_ngan` column and rule 10 gives for refusing `is_overdue`: two homes for one number
// drift, and the stale one is what reaches the report going upward.

// NguonVon is one funding source of one commune in one budget year (table `nguon_von`).
//
// COLUMNS DELIBERATELY ABSENT, the same two as DuAn: `tenant_id` rides in context.Context and is
// bound by the scoped repository (rule 1, invariant 4), and `deleted_at` never reaches a reader
// because a soft-deleted row never leaves the store (rule 7, invariant 2).
type NguonVon struct {
	ID string // ULID

	// Ten is what §6 prints on the card ("Ngân sách xã, phường"). There is no code column: §11
	// gives this table a name and no `ma`, so nothing here is an issued business code.
	Ten string

	// Nam is the budget year this source belongs to. A source of 2026 and the same-named source of
	// 2027 are two rows with two ceilings (§13 rule 8).
	Nam int

	ThuTu int

	// TongNguon is the CEILING of the source, not a balance. Nothing decrements it as money is
	// allocated or spent — TienDoNguonVon computes both of those from rows.
	TongNguon Dong
}

// PhanBoNguonVon is one line of a project's funding plan: how much of this project's year plan is
// drawn from this source (table `phan_bo_nguon_von`).
//
// A PLAN, NOT A PAYMENT. Which source a payment was actually drawn from is
// `chung_tu_giai_ngan.nguon_von_id`, and §6 shows both precisely because they may disagree.
type PhanBoNguonVon struct {
	ID         string // ULID
	DuAnID     string
	NguonVonID string
	SoTien     Dong
}

// TrangThaiGanNguon is the chip §11 defines for a project row: `Đủ · N nguồn`, `Chưa đủ`, or
// `Chưa gắn nguồn`.
//
// THREE STATES AND NOT A BOOLEAN, because "the commune has not allocated this project's plan to any
// source" and "the commune has allocated less than the plan" are different situations with
// different next actions, and §9 says the first one is perfectly normal ("Xã theo dõi kế hoạch vốn
// theo hạng mục thì để trống cũng được"). Collapsing them would show an orange warning on a project
// nobody has done anything wrong to.
type TrangThaiGanNguon string

const (
	// ChuaGanNguon — no allocation row at all. §11: "không có bản ghi ⇒ Chưa gắn nguồn".
	ChuaGanNguon TrangThaiGanNguon = "chua-gan-nguon"
	// ChuaDuNguon — allocated, but the total is below the project's year plan.
	ChuaDuNguon TrangThaiGanNguon = "chua-du"
	// DuNguon — the allocated total reaches or exceeds the year plan.
	DuNguon TrangThaiGanNguon = "du"
)

// TinhTrangGanNguon is the chip together with the two figures it is computed from, so a caller that
// has to render "Đủ · 3 nguồn" never recomputes either.
type TinhTrangGanNguon struct {
	TrangThai TrangThaiGanNguon

	// SoNguon counts DISTINCT sources, not rows. That is load-bearing while
	// migration 0007 leaves `UNIQUE (tenant_id, du_an_id, nguon_von_id)` undeclared: two rows
	// naming one source must read as one source on the chip, or "3 nguồn" silently means "2 sources,
	// one of them entered twice" and nothing on the screen says so.
	SoNguon int

	// TongPhanBo is the sum of the allocation lines — it can exceed the plan, and it is not
	// clamped. §13 rule 2 refuses to hide over-disbursement for the same reason: the figure somebody
	// needs to see is exactly the one a clamp would remove.
	TongPhanBo Dong
}

// GanNguon computes the chip of §11 for one project from its live allocation lines.
//
// THE RULE LIVES HERE AND NOWHERE ELSE. §11 states it in one sentence — "Đủ khi
// SUM(so_tien_phan_bo) ≥ ke_hoach_von_nam, ngược lại Chưa đủ; không có bản ghi ⇒ Chưa gắn nguồn" —
// and a rule that short is exactly the kind that gets re-implemented in a handler and in a template
// until three screens disagree about one project.
//
// THE THRESHOLD IS `≥`, matching the specification's own symbol. A project allocated exactly its
// plan is fully funded; reading it as "chưa đủ" would put an orange chip on the most ordinary row
// in the commune.
//
// A PROJECT WITH NO PLAN (KeHoachVonNam == 0) AND SOME ALLOCATION READS AS `Đủ`, because 0 ≥ 0 is
// what the specification's own condition says. Stated rather than special-cased: there is no
// sentence anywhere granting this function permission to invent a fourth state for it.
func GanNguon(keHoachVonNam Dong, phanBo []PhanBoNguonVon) TinhTrangGanNguon {
	if len(phanBo) == 0 {
		return TinhTrangGanNguon{TrangThai: ChuaGanNguon}
	}

	var tong Dong
	rieng := make(map[string]struct{}, len(phanBo))
	for _, p := range phanBo {
		tong += p.SoTien
		rieng[p.NguonVonID] = struct{}{}
	}

	tt := ChuaDuNguon
	if tong >= keHoachVonNam {
		tt = DuNguon
	}
	return TinhTrangGanNguon{TrangThai: tt, SoNguon: len(rieng), TongPhanBo: tong}
}

// TienDoNguonVon is one funding source together with the figures DERIVED from rows that name it —
// the card of §6, with its three progress bars.
//
// A SEPARATE TYPE FROM NguonVon, for the reason TienDoDuAn is separate from DuAn: NguonVon is what
// the table holds, this is what a screen is shown. Merging them would put computed totals on the
// entity, which is the first step toward somebody storing one.
type TienDoNguonVon struct {
	NguonVon NguonVon

	// DaPhanBo is SUM(so_tien_phan_bo) over the source's LIVE allocation lines.
	DaPhanBo Dong

	// DaGiaiNgan is SUM(so_tien) over LIVE vouchers naming this source, in every state —
	// `ke-toan-nhap` counts (§11). Vouchers with no source are in NO card's figure; they are the
	// warning of §13 rule 6 and are counted separately.
	DaGiaiNgan Dong

	// SoDuAn counts DISTINCT projects drawing on this source — "Số dự án" in §6's table. Distinct
	// for the same reason TinhTrangGanNguon.SoNguon is.
	SoDuAn int
}

// ConChuaPhanBo is the "còn 9,1 tỷ chưa phân bổ" of §6.
//
// IT CAN BE NEGATIVE — a commune that has allocated more than the source holds. Not clamped: that
// is an over-allocation somebody has to see and resolve, and a zero in its place would hide it on
// every screen while the underlying rows stayed wrong.
func (t TienDoNguonVon) ConChuaPhanBo() Dong {
	return t.NguonVon.TongNguon - t.DaPhanBo
}

// The three ratios of §6's card, in parts per ten thousand.
//
// EACH RETURNS ok = false WHEN ITS DENOMINATOR IS ZERO, and that second return value is the whole
// point — the same reasoning TyLeGiaiNgan gives. §6's own sample data contains the case: "Chương
// trình mục tiêu quốc gia, 10,4 tỷ, đã phân bổ 0 đ, 0 dự án". Printing "0%" for the middle bar
// there would report a source nobody has allocated from as the worst performing one in the commune,
// which is a statement about the commune's work rather than about missing data.
//
// THE MULTIPLICATION IS SAFE IN int64 at any figure this domain can hold: the largest denominator
// in sight is a commune's whole capital plan (§14: 3,3 × 10^10 đồng), and `× 10000` leaves it eight
// orders of magnitude inside int64. It is the same shape as TyLeGiaiNgan and overflows at the same
// point — around 9,2 × 10^14 đồng, which is four orders of magnitude above a province.

// TyLeDaPhanBo is allocated / total — the first bar.
func (t TienDoNguonVon) TyLeDaPhanBo() (PhanVan, bool) {
	if t.NguonVon.TongNguon <= 0 {
		return 0, false
	}
	return PhanVan(int64(t.DaPhanBo) * 10000 / int64(t.NguonVon.TongNguon)), true
}

// TyLeGiaiNganTrenPhanBo is disbursed / allocated — the second bar. This is the one that says how
// the money actually committed to this source is moving.
func (t TienDoNguonVon) TyLeGiaiNganTrenPhanBo() (PhanVan, bool) {
	if t.DaPhanBo <= 0 {
		return 0, false
	}
	return PhanVan(int64(t.DaGiaiNgan) * 10000 / int64(t.DaPhanBo)), true
}

// TyLeGiaiNganTrenTongNguon is disbursed / total — the third bar.
func (t TienDoNguonVon) TyLeGiaiNganTrenTongNguon() (PhanVan, bool) {
	if t.NguonVon.TongNguon <= 0 {
		return 0, false
	}
	return PhanVan(int64(t.DaGiaiNgan) * 10000 / int64(t.NguonVon.TongNguon)), true
}
