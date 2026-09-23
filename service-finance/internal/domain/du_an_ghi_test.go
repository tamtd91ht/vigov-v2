package domain

// The refusals of the investment project write path (docs/ui-ux/06-giai-ngan.md §9, §13).
//
// EVERY CASE HERE IS ABOUT THE SENTENCE, NOT THE ENFORCEMENT. The floor is in migration 0004 and
// 0007 — `UNIQUE (tenant_id, ma)`, `CHECK (ke_hoach_von_nam >= 0)`, `CHECK (so_tien_phan_bo >= 0)` —
// and it holds against every writer. What this package adds is a refusal an accountant in a commune
// can act on, arriving first and in Vietnamese.

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestChuanHoaMaDuAn(t *testing.T) {
	for _, tc := range []struct {
		ten, vao, ra string
		loi          error
	}{
		// §11's own sample, IN CAPITALS. ChuanHoaMa in danh_muc_ba_tang.go admits lower case only,
		// because a catalogue code is a slug this system mints; a PROJECT code is a value the commune
		// types off a paper decision, and folding the case would store something different from it.
		{ten: "mã của đặc tả, giữ nguyên chữ hoa",
			vao: "DA-2026-be-tong-hoa-duong-ngo-xo-2", ra: "DA-2026-be-tong-hoa-duong-ngo-xo-2"},
		// §9's other spelling. Both formats have to pass, because the specification uses both and
		// this package is not the place that decides between them.
		{ten: "mã dãy số của §9", vao: "DA01", ra: "DA01"},
		{ten: "cắt khoảng trắng hai đầu", vao: "  DA01  ", ra: "DA01"},

		// ⚠ THE REFUSAL THAT IS A FINDING, NOT A BUG. §9 offers `☑ Tự sinh mã`; this service does not
		// generate one, because the specification gives two incompatible formats and no scope for the
		// sequence, and a project code is an ISSUED code that rule 7 forbids renumbering.
		{ten: "trống thì từ chối, KHÔNG tự sinh", vao: "", loi: ErrThieuMaDuAn},
		{ten: "chỉ khoảng trắng cũng là trống", vao: "   ", loi: ErrThieuMaDuAn},

		{ten: "gạch nối đầu", vao: "-DA01", loi: ErrMaDuAnSaiDinhDang},
		{ten: "gạch nối cuối", vao: "DA01-", loi: ErrMaDuAnSaiDinhDang},
		// `DA--01` reads as one code and sorts as another. Refused while it is still a typo rather
		// than after it is the code on an archival record.
		{ten: "hai gạch nối liền", vao: "DA--01", loi: ErrMaDuAnSaiDinhDang},
		{ten: "khoảng trắng giữa", vao: "DA 01", loi: ErrMaDuAnSaiDinhDang},
		// A code carrying diacritics would be a code nobody can type twice the same way, and it lands
		// in file names and export columns.
		{ten: "dấu tiếng Việt", vao: "DA-bê-tông", loi: ErrMaDuAnSaiDinhDang},
		{ten: "quá dài", vao: strings.Repeat("A", MaDuAnToiDa+1), loi: ErrMaDuAnQuaDai},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			ra, err := ChuanHoaMaDuAn(tc.vao)
			if tc.loi != nil {
				if !errors.Is(err, tc.loi) {
					t.Fatalf("lỗi = %v, muốn %v", err, tc.loi)
				}
				return
			}
			if err != nil {
				t.Fatalf("lỗi bất ngờ: %v", err)
			}
			if ra != tc.ra {
				t.Errorf("= %q, muốn %q", ra, tc.ra)
			}
		})
	}
}

// Zero is a REAL STATE (a project entered before its allocation is decided, 0004:224-229); negative
// is a sign error that would drag the commune's headline "KẾ HOẠCH VỐN NĂM" below the truth with no
// row looking wrong.
func TestKiemTraKeHoachVon(t *testing.T) {
	for _, tc := range []struct {
		ten string
		so  Dong
		loi error
	}{
		{ten: "không đồng là trạng thái thật", so: 0},
		{ten: "số thường", so: 100_000_000},
		{ten: "âm thì từ chối", so: -1, loi: ErrKeHoachVonAm},
		// The typo guard, shared with a voucher on purpose: both numbers are đồng typed by the same
		// accountant on the same screen, and a ceiling that differed would let a figure through in one
		// box that is refused in the other.
		{ten: "vượt trần chống gõ nhầm", so: SoTienToiDa + 1, loi: ErrKeHoachVonQuaLon},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			err := KiemTraKeHoachVon(tc.so)
			if tc.loi == nil && err != nil {
				t.Fatalf("lỗi bất ngờ: %v", err)
			}
			if tc.loi != nil && !errors.Is(err, tc.loi) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.loi)
			}
		})
	}
}

// §11's "mặc định 31/12". A FUNCTION rather than a database default, because the schema does not
// know the year: a `now()`-derived default would stamp a 2026 project with a 2027 deadline on
// 02/01/2027.
func TestHanGiaiNganMacDinhLa3112CuaNamNganSach(t *testing.T) {
	got := HanGiaiNganMacDinh(2026)
	muon := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(muon) {
		t.Errorf("= %v, muốn %v", got, muon)
	}
	// UTC BECAUSE THE COLUMN IS A `DATE`. A date has no timezone; taking a caller's location would
	// let a clock in UTC+7 produce 30/12 for the same year.
	if got.Location() != time.UTC {
		t.Errorf("múi giờ = %v, muốn UTC", got.Location())
	}
}

// §9 makes the allocation list optional and §11 names the state it produces (`Chưa gắn nguồn`). A
// nil list is a NORMAL project, not an incomplete one.
func TestChuanHoaPhanBoMoiKhongKhaiGiThiKhongLoi(t *testing.T) {
	ra, err := ChuanHoaPhanBoMoi(nil)
	if err != nil {
		t.Fatalf("lỗi bất ngờ: %v", err)
	}
	if len(ra) != 0 {
		t.Errorf("= %v, muốn rỗng", ra)
	}
}

func TestChuanHoaPhanBoMoi(t *testing.T) {
	for _, tc := range []struct {
		ten string
		vao []DongPhanBoMoi
		loi error
	}{
		{
			ten: "hai nguồn khác nhau",
			vao: []DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: 40}, {NguonVonID: "nv-tp", SoTien: 60}},
		},
		{
			// Zero is admitted for the same reason `CHECK (so_tien_phan_bo >= 0)` admits it (0007): a
			// source attached before its figure is agreed is a real intermediate state a commune types.
			ten: "số tiền bằng không là trạng thái thật",
			vao: []DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: 0}},
		},
		{
			// ⚠ THIS DOES NOT ANSWER MIGRATION 0007's OPEN QUESTION (b). What is refused is one REQUEST
			// naming a source twice — a malformed body — not the business state "a project holds two
			// lines for one source", which 0007 says outright is the customer's call. No constraint was
			// added to the schema.
			ten: "một nguồn khai hai dòng trong CÙNG một lần tạo",
			vao: []DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: 40}, {NguonVonID: "nv-xa", SoTien: 60}},
			loi: ErrPhanBoTrungNguon,
		},
		{
			// '' IS NOT "no source" ON THIS TABLE, IT IS A BROKEN REFERENCE — unlike
			// `chung_tu_giai_ngan.nguon_von_id`, where a blank is the meaningful state §13 rule 6
			// defines. `phan_bo_nguon_von.nguon_von_id` is NOT NULL with `CHECK (btrim(...) <> '')`.
			ten: "dòng không nêu nguồn nào",
			vao: []DongPhanBoMoi{{NguonVonID: "  ", SoTien: 40}},
			loi: ErrThieuNguonVonPhanBo,
		},
		{
			ten: "số tiền âm",
			vao: []DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: -1}},
			loi: ErrPhanBoAm,
		},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			_, err := ChuanHoaPhanBoMoi(tc.vao)
			if tc.loi == nil {
				if err != nil {
					t.Fatalf("lỗi bất ngờ: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.loi) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.loi)
			}
		})
	}
}

// ⚠ THE CASE THAT KEEPS §9's WARNING FROM BECOMING A CONSTRAINT. §9: the system "đối chiếu tổng các
// nguồn với số ấy và CẢNH BÁO khi thiếu hoặc vượt". This function validates lines and MUST NOT look
// at the project's plan at all — it does not even receive it, and that signature is the guarantee.
func TestChuanHoaPhanBoMoiKhongDoiChieuVoiKeHoachVon(t *testing.T) {
	// Far above and far below any plausible plan. Both must pass: the comparison belongs to the
	// screen, which draws §9's warning and §11's chip from the two raw figures.
	for _, so := range []Dong{1, SoTienToiDa} {
		if _, err := ChuanHoaPhanBoMoi([]DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: so}}); err != nil {
			t.Fatalf("số tiền %d bị từ chối — §9 là CẢNH BÁO chứ không phải ràng buộc: %v", so, err)
		}
	}
}

// §13 rule 8 lives in the store (the year is bound into the source lookup); what the domain owns is
// refusing a year that is a typo — one that would make the project invisible on every year-filtered
// screen while it sits in the table looking healthy.
func TestKiemTraNamDuAn(t *testing.T) {
	for _, tc := range []struct {
		ten string
		nam int
		loi error
	}{
		{ten: "năm thường", nam: 2026},
		{ten: "biên dưới", nam: NamDuAnSom},
		{ten: "biên trên", nam: NamDuAnMuon},
		{ten: "không có năm", nam: 0, loi: ErrThieuNamDuAn},
		{ten: "gõ nhầm 1026", nam: 1026, loi: ErrNamDuAnNgoaiLich},
		{ten: "gõ nhầm 20226", nam: 20226, loi: ErrNamDuAnNgoaiLich},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			err := KiemTraNamDuAn(tc.nam)
			if tc.loi == nil && err != nil {
				t.Fatalf("lỗi bất ngờ: %v", err)
			}
			if tc.loi != nil && !errors.Is(err, tc.loi) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.loi)
			}
		})
	}
}

// Rule 7, invariant 1 names `delete_reason` and it is not optional.
func TestChuanHoaLyDoXoaDuAnBatBuoc(t *testing.T) {
	if _, err := ChuanHoaLyDoXoaDuAn("   "); !errors.Is(err, ErrThieuLyDoXoaDuAn) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoaDuAn", err)
	}
	ra, err := ChuanHoaLyDoXoaDuAn("  xã rút khỏi kế hoạch vốn  ")
	if err != nil {
		t.Fatalf("lỗi bất ngờ: %v", err)
	}
	if ra != "xã rút khỏi kế hoạch vốn" {
		t.Errorf("= %q, muốn đã cắt khoảng trắng", ra)
	}
}

// §9: "Để trống thì lấy bằng số tiền bố trí năm nay." The rule lives in ONE place so three copies of
// a default cannot drift — and the drift would be invisible, because a plausible number still
// appears.
func TestTongMucHieuLucApDungLuatCuaMuc9(t *testing.T) {
	d := DuAn{KeHoachVonNam: 100_000_000}
	if got := d.TongMucHieuLuc(); got != 100_000_000 {
		t.Errorf("để trống: = %d, muốn bằng kế hoạch vốn năm", got)
	}
	d.TongMucDuocDuyet = 250_000_000
	if got := d.TongMucHieuLuc(); got != 250_000_000 {
		t.Errorf("có khai: = %d, muốn 250000000", got)
	}
}
