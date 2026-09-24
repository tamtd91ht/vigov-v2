package domain

// The batches of revenue/expenditure recorded against one leaf budget line — the `⇄ Các đợt thu, chi`
// dialog (docs/ui-ux/07-thu-chi-ngan-sach.md §5) and the `entries` calculation mode (§4.2) that sums
// them. Storage: migration 0008 (`dot_thu_chi`, `gia_tri_dot`). User decision 25/09/2026, ledger
// service-finance/thu-chi-ngan-sach-82.
//
// WHAT THE USER DECIDED AND THIS FILE ENCODES:
//
//   - A LEAF line chooses `manual` or `entries` (KiemTraCachTinhChon). `children` is never chosen: it
//     follows from the tree (CachTinhTheoCay).
//   - Writing a batch does NOT switch the mode. The line's DISPLAYED figure comes from the batches
//     only while its mode is `entries` (BangDayDu.GiaTri).
//   - A line in `entries` mode that still has live batches may not gain a child
//     (ErrKhoanMucTheoDotConDot) — decided by the main session under the user's "follow the
//     recommendations", 25/09/2026. Without live batches it becomes `children` as before.
//   - A batch is never edited. A wrong batch is removed (soft delete, reason) and recorded again.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// DotThuChi is one batch (table `dot_thu_chi`) with its amounts (table `gia_tri_dot`).
//
// tenant_id IS NOT A FIELD, for the reason BangNganSach gives. `deleted_*` are not fields either: a
// removed batch never leaves the store.
type DotThuChi struct {
	ID         string
	KhoanMucID string
	Ngay       time.Time // a DATE — the day the money moved, as the commune states it
	NoiDung    string

	// DonViCaNhan is "Đơn vị, cá nhân" — "" when not stated. PERSONAL DATA WHEN IT NAMES A PERSON
	// (migration 0008's header): never in a log line, an error message or an audit delta. The audit
	// entry records THAT it was stated and its length, never the text (rule 3; rule 6 forbidden #4).
	DonViCaNhan string

	SoChungTu  string    // "" when not stated
	NguoiGhiMa string    // staff business code (`CB-00123`), never the internal id — rule 6 inv. 8
	TaoLuc     time.Time // zero on a batch just built in memory, before the database stamped it

	// GiaTri is cotID -> amount in đồng, and ONLY STATED AMOUNTS APPEAR. An absent key is an empty
	// amount (§9 rule 4), exactly as in BangDayDu.Gia.
	GiaTri map[string]Dong
}

// DotCuaKhoanMuc is what the `⇄` dialog reads: the line, the sheet's live columns (so the client can
// draw one amount per NUMBER column), and the line's live batches newest first.
type DotCuaKhoanMuc struct {
	KhoanMuc KhoanMucNganSach
	Cot      []CotNganSach
	Dot      []DotThuChi
}

var (
	ErrThieuNgayDot        = errors.New("ngan_sach: thiếu `date` của đợt (YYYY-MM-DD)")
	ErrNgayDotNgoaiLich    = errors.New("ngan_sach: `date` của đợt ngoài khoảng hợp lệ (2000-01-01..2100-12-31)")
	ErrThieuNoiDungDot     = errors.New("ngan_sach: thiếu `content` của đợt")
	ErrNoiDungDotQuaDai    = errors.New("ngan_sach: `content` của đợt quá dài")
	ErrDonViCaNhanQuaDai   = errors.New("ngan_sach: `counterparty` quá dài")
	ErrSoChungTuDotQuaDai  = errors.New("ngan_sach: `document_no` quá dài")
	ErrDotKhongCoSoTienNao = errors.New(
		"ngan_sach: đợt phải có ít nhất một số tiền ở một cột số — một đợt không có số không cộng vào đâu cả")

	// ErrDotChiGhiVaoLa — a batch on a line that has children. The line's figure is the sum of its
	// children (customer decision 06/09/2026); a batch there would sit in the table counting nowhere.
	ErrDotChiGhiVaoLa = errors.New(
		"ngan_sach: khoản mục có dòng con thì không ghi đợt — số của nó luôn cộng từ các dòng con")

	// ErrKhoanMucTheoDotKhongGoThang — a figure typed into a line in `entries` mode. §4.2: the cells
	// become read-only. A typed figure there would be stored and never displayed, and would silently
	// reappear only if somebody later switched modes — which overwrites it anyway (§9.1).
	ErrKhoanMucTheoDotKhongGoThang = errors.New(
		"ngan_sach: khoản mục đang cộng theo đợt — số lấy từ tổng các đợt; đổi sang `manual` trước khi gõ số")

	// ErrKhoanMucTheoDotConDot — adding a child under a line in `entries` mode that still has live
	// batches. Accepted, the line would become `children` and every batch would silently stop counting
	// while still listed in the dialog.
	ErrKhoanMucTheoDotConDot = errors.New(
		"ngan_sach: khoản mục đang cộng theo đợt và còn đợt chưa gỡ — gỡ các đợt hoặc đổi sang `manual` trước khi thêm dòng con")

	// ErrKhoanMucChaKhongDoiCachTinh — `method` sent for a line that has children. Its mode is
	// `children` and follows the tree; neither `manual` nor `entries` can be chosen for it.
	ErrKhoanMucChaKhongDoiCachTinh = errors.New(
		"ngan_sach: khoản mục có dòng con thì luôn cộng từ dòng con — không đổi được cách tính")

	// ErrKhongThayDot — "no such live batch IN THIS COMMUNE". A batch of another commune, a removed
	// batch, and a batch whose line or sheet was removed are one answer: the query reaches none of them.
	ErrKhongThayDot = errors.New("ngan_sach: không có đợt thu chi này trong xã")

	// ErrTongDotVuotMuc — the sum of a line's batches in one column does not fit in int64 đồng. NOT
	// REACHABLE WITH REAL DATA (each amount is bounded by GiaTriToiDa); it exists so that an
	// impossible sum is a refusal the operator sees, never a wrapped-around figure on a report.
	ErrTongDotVuotMuc = errors.New("ngan_sach: tổng các đợt của một khoản mục vượt phạm vi số nguyên")
)

// KiemTraCachTinhChon accepts the two modes a CLIENT may choose for a leaf: `manual` and `entries`.
// `children` is refused here with the same sentence as before — it follows the tree.
func KiemTraCachTinhChon(c CachTinh) error {
	if c != TinhTay && c != TinhTheoDot {
		return ErrCachTinhDoTuClient
	}
	return nil
}

// KiemTraNgayDot — required, and inside the window migration 0008's `dot_thu_chi_ngay_hop_le`
// admits. Checked here so a typo year is a 400 with a sentence and not a 500 from the CHECK.
func KiemTraNgayDot(t time.Time) error {
	if t.IsZero() {
		return ErrThieuNgayDot
	}
	if n := t.Year(); n < NamChungTuSom || n > NamChungTuMuon {
		return ErrNgayDotNgoaiLich
	}
	return nil
}

// ChuanHoaNoiDungDot — required. The cap is the voucher's (NoiDungChungTuToiDa) because migration
// 0008 reuses it: one limit for one kind of text across the module. COUNTED IN RUNES, as
// `char_length` counts: Vietnamese diacritics are 2-3 bytes each.
func ChuanHoaNoiDungDot(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuNoiDungDot
	case utf8.RuneCountInString(s) > NoiDungChungTuToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNoiDungDotQuaDai, NoiDungChungTuToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrThieuNoiDungDot
	}
	return s, nil
}

// ChuanHoaDonViCaNhan — OPTIONAL; "" means not stated and is stored as NULL. The error names the
// field and the limit, NEVER the value: this text may be a citizen's name (rule 3, forbidden #3).
func ChuanHoaDonViCaNhan(s string) (string, error) {
	return chuanHoaTuyChonDot(s, DoiTacToiDa, ErrDonViCaNhanQuaDai)
}

// ChuanHoaSoChungTuDot — OPTIONAL; "" is stored as NULL.
func ChuanHoaSoChungTuDot(s string) (string, error) {
	return chuanHoaTuyChonDot(s, SoChungTuToiDa, ErrSoChungTuDotQuaDai)
}

func chuanHoaTuyChonDot(s string, toiDa int, quaDai error) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case utf8.RuneCountInString(s) > toiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", quaDai, toiDa)
	case coKyTuDieuKhien(s):
		return "", quaDai
	}
	return s, nil
}
