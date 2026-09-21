package domain

import "sort"

// LoaiViec is the kind of work an SLA row applies to (@entity ProcessingDeadline, migration
// 0008).
//
// THREE VALUES, AND THEY ARE THREE BUSINESS DOMAINS RATHER THAN THREE LABELS: `van-ban-den` is
// documents' work, `phan-anh` and `nhiem-vu` are petitions'. That is the whole reason the table
// belongs to neither of those services and lives here (ADR 0029). A FOURTH VALUE IS A DECISION,
// not a deployment — ADR 0029, stop condition #1.
//
// THE VALUES ARE THE STRINGS THE DATABASE STORES, spelled exactly as the CHECK constraint
// admits them (migration 0008). A second spelling in Go would be a second name for one thing.
type LoaiViec string

const (
	LoaiViecVanBanDen LoaiViec = "van-ban-den"
	LoaiViecPhanAnh   LoaiViec = "phan-anh"
	LoaiViecNhiemVu   LoaiViec = "nhiem-vu"
)

// HopLe reports whether v is one of the three admitted values.
//
// IT EXISTS FOR THE WRITE PATH THAT DOES NOT YET EXIST, and for a read that finds a value the
// CHECK constraint should have refused — a database restored from elsewhere, a column altered by
// hand. A caller that meets false must refuse: an SLA row for an unknown kind of work is a
// number nobody can say what it promises.
func (v LoaiViec) HopLe() bool {
	switch v {
	case LoaiViecVanBanDen, LoaiViecPhanAnh, LoaiViecNhiemVu:
		return true
	}
	return false
}

// DongSLA is one row of a commune's processing-deadline table.
//
// EVERY NUMBER HERE IS A COUNT OF WORKING HOURS (ADR 0007), AND THIS TYPE DELIBERATELY OFFERS NO
// WAY TO TURN ONE INTO AN INSTANT. Converting hours into a deadline needs the commune's
// calendar, and there is exactly one implementation of that — TienGioLamViec, published as
// AdvanceWorkingHours. A second one would be a second answer to the same question, and the two
// would disagree only on nights, weekends, ngay_nghi_le and ngay_lam_bu: the moments a citizen
// notices. This type supplies the NUMBER; it never does the arithmetic.
//
// THERE IS NO OVERDUE FIELD AND THERE MUST NEVER BE ONE. Overdue is derived from the stored
// deadline against now (rule 10, invariant 3).
type DongSLA struct {
	// ID is the ULID of the row.
	ID string

	LoaiViec LoaiViec

	// LinhVuc is the field code this row applies to, EMPTY for the default row.
	//
	// EMPTY MEANS DEFAULT AND NOTHING ELSE. The column is NULL for the default row and the
	// schema refuses the empty string outright (sla_linh_vuc_khong_rong, migration 0008), so the
	// two cannot both arrive and mean two things.
	//
	// IT IS A VALUE, NOT A KEY: the field codes are a closed platform-level code set owned by
	// service `platform` (ADR 0026), so nothing here can verify that the code still exists. The
	// specification already carries the consequence — 14-cau-hinh.md:308 holds an SLA row
	// pointing at a code that was removed, and the screen renders it raw.
	LinhVuc string

	// GioTiepNhan fixes `han_tiep_nhan`, at the act that CREATES the record (ADR 0028,
	// decision E). On the citizen channels the field is not known yet, so the caller reads this
	// from the DEFAULT row — see DongMacDinh.
	GioTiepNhan int

	// GioXuLyXong fixes `han_xu_ly_xong`, at the act that SETTLES THE FIELD — not at insert
	// time. Reading it when the row is created rebuilds the 56-hour ceiling ADR 0028 removed,
	// and is stop condition #1 of that ADR.
	GioXuLyXong int

	// GioSapDenHan is the number of working hours REMAINING at which the record starts counting
	// as "sắp đến hạn". It drives three things at once and they must not drift apart: when the
	// reminder is sent, what the "Sắp đến hạn" filter selects, and the figure in the bell
	// (docs/ui-ux/14-cau-hinh.md §8, the second sentence that section requires be kept).
	GioSapDenHan int

	// GioBaoLanhDao and GioBaoChuTich carry the escalation figures — AND THEIR ANCHOR IS NOT
	// DECIDED. The specification says it two ways: §8 heads the columns "sau 24 giờ" without
	// saying after what, while §9's escalation job counts from the deadline being missed and
	// DOUBLES the figure for the president instead of reading a second column.
	//
	// NOTHING MAY COMPUTE AN ESCALATION FROM THESE TWO UNTIL SOMEBODY ANSWERS "after what". A
	// reader that guesses notifies a commune's leadership on a basis nobody chose, and the
	// commune cannot tell from the message which basis was used.
	GioBaoLanhDao int
	GioBaoChuTich int
}

// LaDongMacDinh reports whether this is the row that applies to every field without one of its
// own (`linh_vuc IS NULL`, migration 0008).
func (d DongSLA) LaDongMacDinh() bool { return d.LinhVuc == "" }

// LoaiVanDeSLA names a way a commune's deadline table cannot answer the question put to it.
//
// THE VALUES ARE PART OF THE CONTRACT — a configuration screen shows them — so they are stable
// English identifiers, in the same spelling a client would branch on. Same discipline as
// LoaiVanDeLich.
type LoaiVanDeSLA string

const (
	// VanDeSLATrong: the commune has no SLA row at all.
	//
	// AN EMPTY TABLE IS NOT A DEFAULT, AND THIS CONSTANT EXISTS SO NOBODY CAN TREAT IT AS ONE.
	// Nothing may fall back to "24 hours", and nothing may fall back to the sixteen rows of
	// docs/ui-ux/14-cau-hinh.md §8 either: those came from ONE commune's prototype, and a
	// commitment invented by software is still told to a citizen as though the authority made it
	// (rule 10, forbidden #3; migration 0008, §NOTHING IS SEEDED).
	//
	// It is REPORTED rather than raised as an error, for the same reason VanDeLichTrong is: the
	// configuration screen that would fix the problem has to be able to load. Today this is
	// every commune — migration 0008 seeds nothing and the onboarding step does not exist.
	VanDeSLATrong LoaiVanDeSLA = "empty_sla"

	// VanDeThieuDongMacDinh: a kind of work has rows but no default row.
	//
	// THE HOLE THIS NAMES IS INVISIBLE ON THE SCREEN. A commune that configured `an-ninh-trat-tu`
	// and nothing else looks configured; a petition arriving in any other field has nothing to
	// fall back on, and on the citizen channels there is a second victim — ADR 0028 decision E
	// reads `han_tiep_nhan` from the DEFAULT row for every petition a citizen sends, because the
	// field is not known at that moment. Without the default row, not one of them gets a
	// deadline.
	VanDeThieuDongMacDinh LoaiVanDeSLA = "missing_default_row"
)

// VanDeSLA is one problem found in a commune's deadline table.
//
// DERIVED ON EVERY READ, NEVER STORED — the same discipline rule 10 invariant 3 applies to the
// overdue state. A column recording "this configuration is incomplete" would be written by
// something, and whatever wrote it would be stale the moment a row changed.
type VanDeSLA struct {
	Loai LoaiVanDeSLA

	// LoaiViec is the kind of work the problem sits on, empty when the problem is about the
	// whole table — VanDeSLATrong is not about one kind of work.
	LoaiViec LoaiViec
}

// VanDeCuaSLA reports everything that makes this deadline table unusable. An empty result means
// every kind of work present in it can be read from.
//
// IT DOES NOT DEMAND ALL THREE KINDS OF WORK, and that omission is deliberate rather than an
// oversight: a commune that does not use the task module has no `nhiem-vu` row, and calling that
// a defect would be this function deciding a commune's administrative practice. What it does
// check is the shape the specification itself fixes — that a kind of work which HAS rows also
// has the row the others fall back on.
func VanDeCuaSLA(ds []DongSLA) []VanDeSLA {
	if len(ds) == 0 {
		return []VanDeSLA{{Loai: VanDeSLATrong}}
	}

	coDong := map[LoaiViec]bool{}
	coMacDinh := map[LoaiViec]bool{}
	for _, d := range ds {
		coDong[d.LoaiViec] = true
		if d.LaDongMacDinh() {
			coMacDinh[d.LoaiViec] = true
		}
	}

	thieu := make([]string, 0, len(coDong))
	for lv := range coDong {
		if !coMacDinh[lv] {
			thieu = append(thieu, string(lv))
		}
	}
	// SORTED, because map iteration order is random in Go and a list of problems that reorders
	// between two loads of the same screen reads as "something changed" when nothing did.
	sort.Strings(thieu)

	ra := make([]VanDeSLA, 0, len(thieu))
	for _, lv := range thieu {
		ra = append(ra, VanDeSLA{Loai: VanDeThieuDongMacDinh, LoaiViec: LoaiViec(lv)})
	}
	if len(ra) == 0 {
		return nil
	}
	return ra
}

// DongMacDinh returns the default row for one kind of work — the row ADR 0028 decision E reads
// `gio_tiep_nhan` from for every petition a citizen sends, because at that instant nobody knows
// the field yet.
//
// IT DOES NOT FALL BACK TO ANYTHING. There is nothing below the default row; false means the
// commune has not configured this kind of work and the caller must REFUSE to compute a deadline.
func DongMacDinh(ds []DongSLA, loai LoaiViec) (DongSLA, bool) {
	for _, d := range ds {
		if d.LoaiViec == loai && d.LaDongMacDinh() {
			return d, true
		}
	}
	return DongSLA{}, false
}

// DongTheoLinhVuc returns the row that applies to one field of one kind of work, FALLING BACK TO
// THE DEFAULT ROW when that field has no row of its own.
//
// THE FALLBACK IS THE SPECIFICATION'S, NOT THIS FUNCTION'S: `linh_vuc = null` is labelled "Mặc
// định cho mọi lĩnh vực" (docs/ui-ux/14-cau-hinh.md:311, :316), which is what the default row is
// for. Without it a commune would have to type sixteen rows before a single deadline could be
// computed.
//
// AN EMPTY linhVuc ASKS FOR THE DEFAULT ROW, because empty is what the default row's field code
// is (LaDongMacDinh). A caller holding a field code it has not validated should not reach here
// with "" and expect a field-specific answer — the codes are a closed platform-level set
// (ADR 0026) and validating one is the WRITE path's job.
//
// FALSE MEANS REFUSE. Not "use zero", not "use the other kind of work's row": a deadline is a
// commitment, and the one thing worse than refusing to make one is inventing one (rule 10).
func DongTheoLinhVuc(ds []DongSLA, loai LoaiViec, linhVuc string) (DongSLA, bool) {
	if linhVuc != "" {
		for _, d := range ds {
			if d.LoaiViec == loai && d.LinhVuc == linhVuc {
				return d, true
			}
		}
	}
	return DongMacDinh(ds, loai)
}
