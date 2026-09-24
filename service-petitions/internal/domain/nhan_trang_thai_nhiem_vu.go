package domain

// A commune's own LABEL and ORDER for the seven task statuses — entity `TaskStatusLabel`,
// table `nhan_trang_thai_nhiem_vu` (migration 0010), URL resource `task-statuses`.
//
// OPEN QUESTION #21, DECIDED (ADR 0035 §C): a commune changes the LABEL and the ORDER of a status,
// never the LIST OF CODES, and this group has NO `Tắt`. So everything here is keyed on the closed
// set of TrangThaiNhiemVu codes in nhiem_vu.go, and nothing here can add, remove or disable one.
//
// THE MODEL IS AN OVERRIDE, the same one `nhan_linh_vuc` uses (ADR 0026 tier 2): a code with no row
// is perfectly valid and shows the DEFAULT below. A reader that treated "no row" as "unknown
// status" would blank out every Kanban column of every commune — because today no commune has a
// single row.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// VaiTroTrangThai is where a status sits in the lifecycle diagram of docs/ui-ux/02-nhiem-vu.md §6.
// The values are Vietnamese without diacritics — enum VALUES are never translated (ADR 0011).
type VaiTroTrangThai string

const (
	// VaiTroChinh — one of the five main states; the Kanban board draws a column for it (§4.1).
	VaiTroChinh VaiTroTrangThai = "chinh"
	// VaiTroReNhanh — `tam-dung` / `chuyen-tiep`, the two branch states with no column of their own.
	VaiTroReNhanh VaiTroTrangThai = "re-nhanh"
)

// VaiTro derives the role from LaTrangThaiChinh — one answer to "is this a main state", not two.
func (t TrangThaiNhiemVu) VaiTro() VaiTroTrangThai {
	if t.LaTrangThaiChinh() {
		return VaiTroChinh
	}
	return VaiTroReNhanh
}

// MacDinhTrangThai is the wording and position the SOFTWARE ships for one status.
type MacDinhTrangThai struct {
	Ma    TrangThaiNhiemVu
	Nhan  string
	ThuTu int
}

// macDinhTrangThaiNhiemVu IS THE ONE PLACE THE DEFAULT WORDING LIVES IN CODE — transcribed from
// docs/ui-ux/02-nhiem-vu.md §6 (the table at :216-224), in that table's order.
//
// WHY HERE AND NOT IN THE WEB CLIENT: the read route merges a commune's overrides with these
// defaults and answers all seven codes, so a screen never needs its own copy. A second copy on the
// screen is a copy that drifts, and the one that drifts is the one a member of staff reads.
//
// `moi-giao` IS "Mới giao", the §6 name. The Kanban board's "Chưa thực hiện" (§6, in italics) is
// exactly the kind of commune-specific wording this table exists to let a commune set for itself;
// shipping it as the default would pick one of the specification's two names silently.
//
// THE SET OF CODES MUST EQUAL THE LIFECYCLE'S SET (chuyenDuocSangNhiemVu) and migration 0010's CHECK.
// nhan_trang_thai_nhiem_vu_test.go asserts the first; migrations/nhan_trang_thai_nhiem_vu_test.go
// the second. An eighth entry here would be a label for a state nothing can reach.
var macDinhTrangThaiNhiemVu = [...]MacDinhTrangThai{
	{MoiGiao, "Mới giao", 1},
	{DaTiepNhanNV, "Đã tiếp nhận", 2},
	{DangThucHien, "Đang thực hiện", 3},
	{ChoDuyet, "Chờ duyệt", 4},
	{HoanThanh, "Hoàn thành", 5},
	{TamDung, "Tạm dừng", 6},
	{ChuyenTiep, "Chuyển tiếp", 7},
}

// MacDinhTrangThaiNhiemVu returns a COPY of the defaults, in default order. A copy, because a
// caller holding the array itself could rewrite the shipped wording for every commune at once.
func MacDinhTrangThaiNhiemVu() []MacDinhTrangThai {
	ra := make([]MacDinhTrangThai, len(macDinhTrangThaiNhiemVu))
	copy(ra, macDinhTrangThaiNhiemVu[:])
	return ra
}

// TimMacDinhTrangThai returns the default for one code, and false for a code outside the seven.
// FAIL CLOSED: the caller turns false into "not found", never into a label it made up.
func TimMacDinhTrangThai(ma string) (MacDinhTrangThai, bool) {
	for _, md := range macDinhTrangThaiNhiemVu {
		if string(md.Ma) == ma {
			return md, true
		}
	}
	return MacDinhTrangThai{}, false
}

// NhanTrangThaiNhiemVu is ONE OVERRIDE ROW as stored: a commune's wording and position for one code.
type NhanTrangThaiNhiemVu struct {
	Ma    TrangThaiNhiemVu
	Nhan  string
	ThuTu int

	// CapNhatBoi is the STAFF BUSINESS CODE (`CB-00123`) of whoever last set it — never the internal
	// id (rule 6, invariant 8). A convenience for the screen; the evidence is the audit entry.
	CapNhatBoi string
}

// TrangThaiHienThi is one status as a commune SEES it: the override when there is one, else the
// default — with the default carried beside it so a configuration screen can show both.
type TrangThaiHienThi struct {
	Ma    TrangThaiNhiemVu
	Nhan  string
	ThuTu int

	VaiTro VaiTroTrangThai

	NhanMacDinh  string
	ThuTuMacDinh int

	// DaTuyChinh is DERIVED: the effective wording or position differs from the default. It is NOT
	// "a row exists" — "back to the default" is a WRITE of the default values (migration 0010 refuses
	// DELETE), and after that write the row still exists while nothing is customised any more. A flag
	// meaning "row exists" would keep saying "đã tuỳ chỉnh" on a status showing the shipped wording.
	DaTuyChinh bool
}

// ErrMaTrangThaiLa — an override row for a code outside the seven. Migration 0010's CHECK makes that
// impossible to STORE, so reaching this means the schema and this file disagree. Refused rather than
// skipped: a skipped row is a commune's setting silently not applied (fail closed).
var ErrMaTrangThaiLa = errors.New("nhan_trang_thai_nhiem_vu: mã trạng thái không thuộc bảy mã của vòng đời")

// HienThiMot merges ONE code's default with its override (nil = no row).
func HienThiMot(md MacDinhTrangThai, ghiDe *NhanTrangThaiNhiemVu) TrangThaiHienThi {
	ra := TrangThaiHienThi{
		Ma: md.Ma, Nhan: md.Nhan, ThuTu: md.ThuTu, VaiTro: md.Ma.VaiTro(),
		NhanMacDinh: md.Nhan, ThuTuMacDinh: md.ThuTu,
	}
	if ghiDe != nil {
		ra.Nhan, ra.ThuTu = ghiDe.Nhan, ghiDe.ThuTu
	}
	ra.DaTuyChinh = ra.Nhan != md.Nhan || ra.ThuTu != md.ThuTu
	return ra
}

// GopNhanTrangThai answers ALL SEVEN codes — never fewer — sorted by effective position.
//
// TIES ARE BROKEN BY THE DEFAULT POSITION, which is total (seven distinct numbers), so two calls can
// never return the same statuses in a different order. Migration 0010 deliberately has no
// UNIQUE (tenant_id, thu_tu) — partial overrides and two-step swaps both need ties to be legal — so
// the tie-break has to live on the read, and here is the one read.
func GopNhanTrangThai(ghiDe []NhanTrangThaiNhiemVu) ([]TrangThaiHienThi, error) {
	theoMa := make(map[TrangThaiNhiemVu]*NhanTrangThaiNhiemVu, len(ghiDe))
	for i := range ghiDe {
		if _, ok := TimMacDinhTrangThai(string(ghiDe[i].Ma)); !ok {
			return nil, ErrMaTrangThaiLa
		}
		theoMa[ghiDe[i].Ma] = &ghiDe[i]
	}

	ra := make([]TrangThaiHienThi, 0, len(macDinhTrangThaiNhiemVu))
	for _, md := range macDinhTrangThaiNhiemVu {
		ra = append(ra, HienThiMot(md, theoMa[md.Ma]))
	}
	sort.SliceStable(ra, func(i, j int) bool {
		if ra[i].ThuTu != ra[j].ThuTu {
			return ra[i].ThuTu < ra[j].ThuTu
		}
		return ra[i].ThuTuMacDinh < ra[j].ThuTuMacDinh
	})
	return ra, nil
}

// --- validation of what a client may supply ---------------------------------------------------

// NhanTrangThaiToiDa is migration 0010's `char_length(nhan) <= 100`, restated so a client sees a 400
// rather than the CHECK's 500. Deliberately NOT NhanToiDa (200, the catalogues' bound): this table's
// CHECK is 100, and a validator looser than the CHECK is a validator that lets a 500 through.
const NhanTrangThaiToiDa = 100

// ErrKhongTatDuocTrangThai — the request tried to take a status out of use. ADR 0035 §C: this group
// has no `Tắt`, because tasks sitting in a disabled status would drop out of every filter.
var ErrKhongTatDuocTrangThai = errors.New("trang_thai_nhiem_vu: nhóm trạng thái nhiệm vụ không có `Tắt` — chỉ sửa được `label` và `order`")

// ChuanHoaNhanTrangThai trims and validates a status label.
//
// COUNTED IN CHARACTERS (runes), NOT BYTES, matching `char_length` in the CHECK. A Vietnamese letter
// with diacritics is 2-3 bytes in UTF-8, so a byte count would refuse a 40-letter Vietnamese label
// that the database would accept — and accept nothing the database refuses in exchange.
func ChuanHoaNhanTrangThai(nhan string) (string, error) {
	nhan = strings.TrimSpace(nhan)
	switch {
	case nhan == "":
		return "", ErrNhanTrong
	case utf8.RuneCountInString(nhan) > NhanTrangThaiToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNhanQuaDai, NhanTrangThaiToiDa)
	}
	for _, r := range nhan {
		if unicode.IsControl(r) {
			// A control character corrupts a Kanban header and a log line alike.
			return "", ErrNhanTrong
		}
	}
	return nhan, nil
}

// KiemTraThuTuTrangThai bounds the position: 1 first (migration 0010, `thu_tu >= 1`).
//
// THE UPPER BOUND IS ThuTuToiDa, the catalogues' own, and it exists for the column type rather than
// for the business: an INT overflow from a JSON number would otherwise come back as a 500.
func KiemTraThuTuTrangThai(thuTu int) error {
	if thuTu < 1 || thuTu > ThuTuToiDa {
		return fmt.Errorf("%w (1..%d)", ErrThuTuNgoaiKhoang, ThuTuToiDa)
	}
	return nil
}
