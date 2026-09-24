package domain

import (
	"errors"
	"time"
)

// A task — `nhiem_vu`, entity `Task`, URL resource `tasks`.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). The
// rules below are expressed as pure functions over values precisely so they can be tested without
// a database, without a network and without a commune's configuration.

// --- the seven statuses ----------------------------------------------------------------------

// TrangThaiNhiemVu is where a task currently stands.
//
// THE LIST IS CLOSED — open question #21, DECIDED on 2026-09-22 (ADR 0035 §C): a commune may
// change the LABEL and the ORDER, never the LIST OF CODES. Two consequences, both load-bearing:
//
//   - the lifecycle below may name the codes DIRECTLY in source. That is only true because the
//     list is fixed; it would be wrong for a list a commune could edit, and the code signal
//     attached to question #21 says exactly that.
//   - changing one of these strings is now a MIGRATION OVER RECORDS A COMMUNE HAS CLOSED, not a
//     rename. ADR 0035 §C calls the reverse direction — pulling commune-invented codes back into a
//     shared set — something that cannot be done at all.
//
// ADR 0035 §C also decided there is NO `Tắt` for this group, which is why there is no `dang_dung`
// anywhere near these codes: disabling a status that has tasks sitting in it drops those tasks out
// of every filter, and nothing on the screen would say so.
//
// The codes are Vietnamese without diacritics, kebab-case: enum VALUES are never translated into
// English (ADR 0011). The LABELS are not on these constants — the commune owns them (#21). The
// shipped DEFAULT wording and order live in ONE table, nhan_trang_thai_nhiem_vu.go, transcribed from
// `docs/ui-ux/02-nhiem-vu.md` §6; a commune's own wording is a row of migration 0010.
type TrangThaiNhiemVu string

const (
	MoiGiao      TrangThaiNhiemVu = "moi-giao"
	DaTiepNhanNV TrangThaiNhiemVu = "da-tiep-nhan"
	DangThucHien TrangThaiNhiemVu = "dang-thuc-hien"
	ChoDuyet     TrangThaiNhiemVu = "cho-duyet"
	HoanThanh    TrangThaiNhiemVu = "hoan-thanh"
	TamDung      TrangThaiNhiemVu = "tam-dung"
	ChuyenTiep   TrangThaiNhiemVu = "chuyen-tiep"
)

// DaTiepNhanNV CARRIES A SUFFIX AND THE PETITION CONSTANT DOES NOT, because both lifecycles have a
// step spelled `da-tiep-nhan` and Go constants share one package namespace. The suffix is on the
// TASK one on purpose: `DaTiepNhan` was there first, it is referenced across the petition use
// cases, and renaming it to make this file read prettier would edit code that is already right.

// chuyenDuocSangNhiemVu is the whole lifecycle of `docs/ui-ux/02-nhiem-vu.md` §6, and the ABSENCES
// are as deliberate as the entries.
//
// THE NAME CARRIES A SUFFIX FOR THE REASON THE `DaTiepNhanNV` CONSTANT DOES: the petition lifecycle
// already owns the unsuffixed name in this package, and two lifecycles cannot share one map name.
// The compiler says so loudly, which is the right place to find it.
//
// THE MAIN FLOW is the diagram at :227 read left to right. The two branch states are reachable
// from each of the four unfinished main states, which is what the fan-in at :229-231 draws.
//
// `hoan-thanh` AND `chuyen-tiep` ARE TERMINAL, and they appear as keys with EMPTY lists rather
// than being left out, so "a state with no way out" is visibly different from "a state nobody
// wrote down". Neither is a guess dressed up as a rule:
//
//	hoan-thanh   §6 draws no arrow leaving it, and §5.4's two approval tick boxes say in the
//	             interface itself that they "không làm đổi trạng thái nhiệm vụ".
//	chuyen-tiep  §6 says it "sinh bản ghi liên kết" — the work continues as ANOTHER task in
//	             another department, and this row stops here. ⚠ NO COLUMN LINKS THE TWO ROWS, and
//	             migration 0006 does not invent one: which end holds the link, and whether the new
//	             task inherits the deadline, are the same unanswered questions the sub-task tree
//	             raises. Reported as a finding.
//
// `tam-dung` IS THE ONE ENTRY THAT IS NOT A PLAIN LOOKUP — see the note on TamDungVeDuoc.
var chuyenDuocSangNhiemVu = map[TrangThaiNhiemVu][]TrangThaiNhiemVu{
	MoiGiao:      {DaTiepNhanNV, TamDung, ChuyenTiep},
	DaTiepNhanNV: {DangThucHien, TamDung, ChuyenTiep},
	DangThucHien: {ChoDuyet, TamDung, ChuyenTiep},
	ChoDuyet:     {HoanThanh, TamDung, ChuyenTiep},
	TamDung:      {MoiGiao, DaTiepNhanNV, DangThucHien, ChoDuyet},
	HoanThanh:    {},
	ChuyenTiep:   {},
}

// ErrChuyenTrangThaiNhiemVuKhongHopLe is returned for a move the lifecycle does not have.
//
// FAIL CLOSED: a status the map does not know is refused, never allowed through as "probably
// fine". An unknown code can only come from data written before a migration or from a caller that
// made one up — and a task allowed to land in a state with no way out is work that stops moving,
// silently, while a deadline keeps running against it.
var ErrChuyenTrangThaiNhiemVuKhongHopLe = errors.New("nhiem_vu: chuyển trạng thái không hợp lệ")

// HopLe reports whether the string is one of the seven.
func (t TrangThaiNhiemVu) HopLe() bool {
	_, co := chuyenDuocSangNhiemVu[t]
	return co
}

// ChuyenSangDuoc reports whether the lifecycle HAS this move — which is a question about SHAPE and
// not about permission.
//
// ⚠ IT IS DELIBERATELY LOOSE FOR `tam-dung`, AND READING THAT AS A BUG WOULD BE THE MISTAKE. §6
// says a paused task resumes at "(trạng thái trước)" — the state it was paused FROM — so the legal
// target is a fact about this task's history and not about the status alone. This map cannot know
// it, and a map that pretended to would be wrong for three of the four cases.
//
// WHERE THE MISSING HALF LIVES: `nhat_ky_nhiem_vu.trang_thai_tai_thoi_diem` records the state at
// every step (migration 0006), so the state before the pause is READ from the log, in the write use
// case, inside the transaction that holds the row. It is NOT a column on the task: that would be a
// second copy of a fact the log already holds, which is rule 9's one-line test, and the copy that
// drifts is the one a screen reads.
//
// The same split the petition lifecycle makes: a transition being SHAPED correctly and a transition
// being PERMITTED are two questions, and conflating them puts business rules in a lookup table.
func (t TrangThaiNhiemVu) ChuyenSangDuoc(m TrangThaiNhiemVu) bool {
	for _, cho := range chuyenDuocSangNhiemVu[t] {
		if cho == m {
			return true
		}
	}
	return false
}

// KetThuc reports whether the lifecycle has no way out of this status.
func (t TrangThaiNhiemVu) KetThuc() bool {
	return t.HopLe() && len(chuyenDuocSangNhiemVu[t]) == 0
}

// TamDungVeDuoc reports whether a task PAUSED from `tu` may resume into it.
//
// A SEPARATE FUNCTION RATHER THAN A LOOSER MAP, so the caller has to have gone and found the
// previous state before it can ask. `ChuyenSangDuoc(TamDung, X)` answers "is X a shape the pause
// can return to"; this answers "is X the state this task was actually paused from", and only the
// second is a rule. A paused task that resumed into `hoan-thanh` would be finished work nobody did.
func TamDungVeDuoc(tu TrangThaiNhiemVu) bool {
	return tu == MoiGiao || tu == DaTiepNhanNV || tu == DangThucHien || tu == ChoDuyet
}

// LaTrangThaiChinh reports whether this is one of the five columns the Kanban board draws.
//
// §4.1: the two branch states have no column of their own. It is a PRESENTATION fact expressed
// here rather than in the handler because both read paths need it and two copies would disagree.
func (t TrangThaiNhiemVu) LaTrangThaiChinh() bool {
	return t == MoiGiao || t == DaTiepNhanNV || t == DangThucHien || t == ChoDuyet || t == HoanThanh
}

// --- the four sources of work ------------------------------------------------------------------

// NguonGiao is where the task came from (§3, §4.2).
//
// FOUR CODES, CLOSED: each names a DIFFERENT originating register, so a fifth is a new integration
// rather than a new label a commune might want.
type NguonGiao string

const (
	NguonTrucTiep   NguonGiao = "truc-tiep"
	NguonKetLuanHop NguonGiao = "ket-luan-hop"
	NguonVanBanDen  NguonGiao = "van-ban-den"
	NguonPhanAnh    NguonGiao = "phan-anh"
)

func (n NguonGiao) HopLe() bool {
	switch n {
	case NguonTrucTiep, NguonKetLuanHop, NguonVanBanDen, NguonPhanAnh:
		return true
	}
	return false
}

// --- the record ---------------------------------------------------------------------------------

// NhiemVu is one task as the business sees it.
//
// THERE IS NO OVERDUE FIELD ON THIS STRUCT AND THERE MUST NEVER BE ONE (rule 10, invariant 3). See
// TreHan and HoanThanhDungHanBanDau, and read why they are two different questions before using
// either in a report.
//
// IT CARRIES NO CITIZEN PERSONAL DATA. Every person named on it is a member of staff, by business
// code. A task born from a petition points at that petition through NguonID and copies nothing
// across; the day a reporter's name appears on this struct is the day rule 3 is broken by a field
// somebody added "for the list screen".
type NhiemVu struct {
	ID string

	// Ma is the issued number — `NV19`. Sequential per commune, and that is correct here: unlike a
	// petition's lookup code it never leaves the staff surface, so it sits on no enumerable path.
	Ma string

	// Loai and MucUuTien hold CATALOGUE CODES of this service's own tables (migration 0003), and
	// the database holds a real foreign key on both. MucUuTien is empty when the commune has not
	// set one — the form defaults it, the schema does not.
	Loai      string
	MucUuTien string

	// Khoi holds a `khoi_nhiem_vu` code AS A VALUE. That catalogue lives in service `identity`
	// (ADR 0024), so there is no foreign key and no JOIN — and empty is a real answer, the
	// "— Chưa xác định —" of §7.1.
	Khoi string

	TieuDe string
	MoTa   string

	TrangThai TrangThaiNhiemVu

	NguonGiao NguonGiao
	// NguonID is the id of the originating record — a meeting conclusion, an incoming letter or a
	// petition. Empty for `truc-tiep`, which the schema enforces.
	NguonID string

	// BoPhanID is identity's `bo_phan` id. The four `…Ma` fields are STAFF BUSINESS CODES
	// (`CB-2026-7K3M9Q`), never internal ids — rule 6, invariant 8, and the column names say so.
	BoPhanID            string
	NguoiThucHienMa     string
	LanhDaoGiaoViecMa   string
	CoQuanChuTriID      string
	ChuyenVienTheoDoiMa string

	// HanXuLy is the CURRENT commitment and HanBanDau is the commitment AS FIRST MADE. The second
	// never moves — migration 0006's trigger refuses it — because §11.3's on-time ratio is measured
	// against it. Both are zero together or set together; the schema enforces that too.
	//
	// A ZERO VALUE MEANS "NO DEADLINE WAS SET", which §4.1 renders as `Hạn —`. It does not mean
	// "not loaded" and it does not mean "overdue".
	HanXuLy       time.Time
	HanBanDau     time.Time
	NgayHoanThanh time.Time

	TienDo int

	// NhiemVuChaID is the task this one hangs off — §5.10's "Nhiệm vụ con", the column migration
	// 0008 added after ADR 0037 answered the four questions a self-referencing tree forces.
	//
	// EMPTY IS A ROOT TASK and is the ordinary case. The tree has NO DEPTH LIMIT (decision 1), which
	// is what makes a CYCLE representable and is why the write path walks upward before accepting a
	// parent — see domain.ErrChuTrinhCayNhiemVu.
	//
	// ⚠ THERE IS NO INHERITED DEADLINE ANYWHERE NEAR THIS FIELD (decision 2). A sub-task has a
	// deadline OF ITS OWN or none at all, so a child may fall overdue while its parent has not —
	// which is correct business and is stated in ADR 0037 rather than discovered from a screen.
	NhiemVuChaID string

	TomTatKetQua string
	GhiChu       string

	// The two manual approval marks of §5.4. §179 states the rule they carry in the interface
	// itself: they change no status. Nothing in this package reads them for that reason.
	LanhDaoPheDuyetHoanThanh bool
	CapTrenCongNhanHoanThanh bool

	NguoiTaoMa string

	TaoLuc time.Time

	// VanBan is §5.4's "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO" — the three lists of referenced documents
	// (migration 0009, and nhiem_vu_van_ban.go in this package for what a line is).
	//
	// ⚠ nil MEANS "NOT LOADED ON THIS SURFACE", NOT "THIS TASK HAS NO DOCUMENTS", and the two are
	// different statements. The register LIST does not read this block — §4's card does not draw it,
	// and loading three lists for every row of every page would be payload nobody renders — so every
	// item of a page comes back with nil here. Only the DETAIL read and the two write acts fill it,
	// and they fill it with a NON-nil slice, empty when the task really has no lines.
	//
	// Reading a nil here as "no documents" is therefore how a screen would report an empty block for
	// a task that has three. The HTTP layer carries the same distinction onto the wire and says so.
	VanBan []NhiemVuVanBan
}

// TreHan DERIVES whether the CURRENT commitment was missed. It is never stored (rule 10, invariant
// 3): a stored flag is wrong the moment a job is late or a holiday is entered, and the stale copy
// is the one that reaches the report.
//
// A TASK FINISHED LATE STAYS LATE. Once NgayHoanThanh is set the answer stops depending on `now`
// and compares the two recorded instants instead — otherwise a commune's late work would quietly
// stop being late as soon as it was done, and last quarter's figures would change every time
// somebody opened the screen.
//
// NO DEADLINE MEANS NOT LATE, and that is a statement about the COMMITMENT rather than about the
// work: §4.1 renders such a task as `Hạn —`, and nothing was promised to anybody.
func (n NhiemVu) TreHan(now time.Time) bool {
	if n.HanXuLy.IsZero() {
		return false
	}
	if !n.NgayHoanThanh.IsZero() {
		return n.NgayHoanThanh.After(n.HanXuLy)
	}
	return now.After(n.HanXuLy)
}

// HoanThanhDungHanBanDau answers §11.3's question, and it is NOT the negation of TreHan.
//
// ⚠ READ THE DIFFERENCE BEFORE USING EITHER IN A FIGURE, because using the wrong one produces a
// number that is plausible, wrong, and reported upward:
//
//	TreHan                  measured against HanXuLy   — the commitment as it stands TODAY, i.e.
//	                        after any extension. This is what the register's red chip and the
//	                        "Chỉ việc quá hạn" filter mean: is this piece of work late NOW.
//	HoanThanhDungHanBanDau  measured against HanBanDau — the commitment as FIRST made. §11.3 says
//	                        the on-time ratio is "ngay_hoan_thanh ≤ han_ban_dau", and §5.8 promises
//	                        the citizen-facing reason on the screen itself: "Hạn gốc vẫn được giữ
//	                        lại để báo cáo đúng hạn không bị lùi theo". Measuring the ratio against
//	                        HanXuLy would make every granted extension read as a deadline met.
//
// A TASK THAT IS NOT FINISHED IS NOT ON TIME AND NOT LATE — it is not yet a sample. The second
// return value says so, so a caller cannot fold "still running" into either bucket by accident;
// the on-time ratio must EXCLUDE those rows from both numerator and denominator.
func (n NhiemVu) HoanThanhDungHanBanDau() (dungHan, daLaMauDo bool) {
	if n.NgayHoanThanh.IsZero() || n.HanBanDau.IsZero() {
		return false, false
	}
	return !n.NgayHoanThanh.After(n.HanBanDau), true
}

// DaGiaHan reports whether the commitment has been moved from the one first made.
//
// DERIVED FROM THE TWO STORED INSTANTS, not from a counter and not from the request table: the two
// columns already hold the fact, and the request table answers a different question (who asked, and
// what the answer was).
func (n NhiemVu) DaGiaHan() bool {
	return !n.HanBanDau.IsZero() && !n.HanXuLy.IsZero() && !n.HanXuLy.Equal(n.HanBanDau)
}

// ChuaPhanCong reports the `Chưa phân công` state §4.1 and §4.2 render.
//
// IT ASKS ABOUT THE PERSON, NOT THE DEPARTMENT, and the two are different states: §11.1 makes
// "handed to a department with nobody named, for too long" the case the system has to REPORT, so a
// predicate that collapsed them would hide exactly the rows that rule exists to find.
func (n NhiemVu) ChuaPhanCong() bool { return n.NguoiThucHienMa == "" }
