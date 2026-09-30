package domain

// The commune's revenue/expenditure budget board (docs/ui-ux/07-thu-chi-ngan-sach.md).
//
// WHERE THE RULES REALLY LIVE, said first so nothing here is mistaken for the enforcement:
//
//	the two sheet kinds                 migrations/0006, `bang_ngan_sach_loai_hop_le`
//	the two column kinds                the same file, `cot_ngan_sach_kieu_hop_le`
//	the SIX column roles                the same file, `cot_ngan_sach_vai_tro_hop_le`
//	`manual` | `entries` | `children`   migrations/0008 (widened 0006's `khoan_muc_ngan_sach_cach_tinh_hop_le`)
//	hard DELETE refused outright        the same file, `ho_so_luu_tru_cam_xoa_cung`
//
// Those are the floor and they hold against every writer. What they cannot do is explain themselves
// to an accountant in a commune, and two of them cannot be expressed in one table at all — whether
// a role belongs on THIS sheet's kind needs `loai`, which is on another table. That is this file.
//
// ---------------------------------------------------------------------------
// THREE THINGS IN THIS FILE ARE DECISIONS SOMEBODY PAID FOR. They are not style, and none of them
// may be "simplified" into an inference:
//
//  1. THE TOTAL ROW IS MARKED BY A PERSON (`is_headline`). Never the first row, never the shallowest,
//     never the one whose `tt` looks like a total. §5 rule 5 says "mặc định dòng đầu tiên" and that
//     half is the trap: the thu sheet has TWO nested top-level rows and the chi sheet has `Tổng số`
//     as a SIBLING of A…E, so adding or picking by position double-counts on one form and is right
//     on the other, with nothing reporting it. Measured by `../vigov-require` (their migration 0035);
//     recorded at kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md §M3.
//
//  2. THE INDICATOR COLUMNS ARE MARKED BY A PERSON (`vai_tro`). Columns are DATA (§3), so the only
//     alternative is matching the heading text — the same trap, word for word. ADR 0035 §A fixes
//     which column each figure comes from, and the gap between the two candidate revenue columns in
//     the specification's own sample is over a million units: enough to flip the sign of the
//     `Cân đối thu - chi` cell.
//
//  3. NOT ENOUGH DATA MEANS NO FIGURE AND A SENTENCE — never 0, never NaN, never a default. ADR 0035
//     §A: "một ô trống kèm lý do là một việc cần làm; một con số sai là một văn bản phải đính chính".
//     Every function below that can fail to produce a number returns the reason with it.
//
// THIS PACKAGE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4, and doc.go).

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// --- the code sets ------------------------------------------------------------------------------
//
// NAMED string TYPES AND NOT BARE ONES, so a sheet kind and a column role cannot be passed to each
// other's parameter. The VALUES are Vietnamese without diacritics (ADR 0011) and are exactly what
// the CHECK constraints admit — a third spelling here is a row PostgreSQL refuses, discovered on
// somebody's first save.

type LoaiBang string

const (
	BangThu LoaiBang = "thu"
	BangChi LoaiBang = "chi"
)

type KieuCot string

const (
	CotSo       KieuCot = "so"
	CotPhanTram KieuCot = "phan_tram"
)

// VaiTroCot says WHICH FIGURE a column holds, for the two indicators that leave this screen and go
// into a report. The empty value means "an ordinary column" and is the case for most of them.
//
// SIX ROLES AND NOT A ROLE PER COLUMN: only the columns ADR 0035 §A names need to be found by
// meaning. Everything else is a heading the commune wrote and the screen prints.
type VaiTroCot string

const (
	// The four columns of a `thu` sheet that matter (§3.2).
	//
	// `VaiTroDuToanTPGiao` IS THE DENOMINATOR OF `Thu đạt dự toán` — ADR 0035 #33, chosen over
	// `Dự toán Xã giao` because it is the figure the commune is JUDGED against: a target set by the
	// city/district, comparable between communes. A commune's own target is its own ruler, and
	// targets nobody shares cannot be added up.
	VaiTroDuToanTPGiao VaiTroCot = "du-toan-tp-giao"
	VaiTroDuToanXaGiao VaiTroCot = "du-toan-xa-giao"

	// `VaiTroThuNSNN` is the NUMERATOR of `Thu đạt dự toán` (§9 rule 6, unchanged by ADR 0035 —
	// that ADR moved the denominator only).
	VaiTroThuNSNN VaiTroCot = "thu-nsnn"

	// `VaiTroThuXaHuong` IS `Tổng thu` FOR THE `Cân đối thu - chi` CELL — ADR 0035 #32, and this is
	// the one most likely to be "corrected" by somebody reading §5 rule 5 on its own. A commune
	// balances on what it is ENTITLED TO KEEP under the allocation rules, not on what arose in its
	// territory: most of what arises is regulated upward and THE COMMUNE MAY NOT SPEND IT. Built on
	// the wrong column, that cell answers "how much money did this territory produce" — a real
	// question, and not the one the person reading the cell is asking.
	VaiTroThuXaHuong VaiTroCot = "thu-xa-huong"

	// The two columns of a `chi` sheet that matter (§3.1).
	VaiTroDuToanNam   VaiTroCot = "du-toan-nam"
	VaiTroChiNganSach VaiTroCot = "chi-ngan-sach"
)

// vaiTroCuaLoai is which roles are legal on which kind of sheet. The database cannot check this —
// `loai` lives on `bang_ngan_sach` and `vai_tro` on `cot_ngan_sach` — so this map is the only place
// it is decided, and KiemTraVaiTro is the only reader.
var vaiTroCuaLoai = map[LoaiBang][]VaiTroCot{
	BangThu: {VaiTroDuToanTPGiao, VaiTroDuToanXaGiao, VaiTroThuNSNN, VaiTroThuXaHuong},
	BangChi: {VaiTroDuToanNam, VaiTroChiNganSach},
}

// CachTinh is how a line's figure is produced.
//
// THREE VALUES, as §4.2 draws. `entries` — "Cộng theo đợt", the `⇄` dialog of §5 — sums the live
// batches of `dot_thu_chi` (migration 0008, user decision 25/09/2026).
//
// A LINE WITH CHILDREN IS ALWAYS `children` and that is not chosen by a client: it follows from the
// tree (customer decision 06/09/2026, CachTinhTheoCay). A LEAF chooses `manual` or `entries`
// (KiemTraCachTinhChon, dot_thu_chi.go) — through PATCH /api/v1/budget-lines/{id}, never on create.
//
// THE READER AND THE WRITER OF `entries` LANDED TOGETHER: BangDayDu.GiaTri reads GiaDot for a leaf
// in that mode, and internal/app.SuaKhoanMuc is the only writer of the value. One without the other
// is a line whose figures all read empty while looking like a working feature.
type CachTinh string

const (
	TinhTay     CachTinh = "manual"
	TinhTheoDot CachTinh = "entries"
	TinhTheoCon CachTinh = "children"
)

// CachTinhTheoCay is the ONE place the derivation lives. A second copy is how a row ends up marked
// `manual` while carrying children, which is a row the customer's rule says cannot exist.
func CachTinhTheoCay(coCon bool) CachTinh {
	if coCon {
		return TinhTheoCon
	}
	return TinhTay
}

// --- the rows -----------------------------------------------------------------------------------

// BangNganSach is one tab × one budget year (table `bang_ngan_sach`).
//
// COLUMNS DELIBERATELY ABSENT:
//
//	tenant_id           never a field. It rides in context.Context and is bound by the scoped
//	                    repository (rule 1, invariant 4).
//	deleted_at          a soft-deleted sheet never leaves the store (rule 7, invariant 2).
//	khoan_muc_tong_id   §7 puts the pointer to the total row here; this schema puts a FLAG on the
//	                    row instead (migration 0006 explains the measurement). A pointer cannot
//	                    express "two rows claim to be the total" — it would hand back a plausible
//	                    single answer for a sheet that has no single answer.
type BangNganSach struct {
	ID   string // ULID
	Ma   string // "NS-2026-CHI-01" — issued once, never reissued
	Nam  int
	Loai LoaiBang

	// Lan is the revision: each `🗑 Gỡ` + reload of this year+kind is its own sheet. Migration 0006
	// says why a revision number is needed at all — without it, rule 7's "a code is never reissued"
	// and §6's ordinary reload cannot both hold.
	Lan int

	TieuDe    string
	DonViTinh string

	LuyKeDen time.Time // zero when the commune has not stated it
	NguonTep string    // "" for a sheet entered by hand
	NapLuc   time.Time // zero for a sheet entered by hand
}

// CotNganSach is one column of one sheet (table `cot_ngan_sach`).
type CotNganSach struct {
	ID     string
	BangID string
	Ten    string
	ThuTu  int
	Kieu   KieuCot

	// CongThuc is stored for a `phan_tram` column and IS NOT EVALUATED HERE. §9 rule 3: a percentage
	// column is computed at render and never stored, so it never becomes a second home for something
	// derivable. The two indicators that DO go into a report are computed from VaiTro, never from
	// this string.
	CongThuc string

	// VaiTro is empty for an ordinary column. See the block at the top of this file.
	VaiTro VaiTroCot
}

// KhoanMucNganSach is one row of the tree (table `khoan_muc_ngan_sach`).
type KhoanMucNganSach struct {
	ID     string
	BangID string
	ChaID  string // "" at the top level

	// TT is the reference the commune's own file carries: "A", "I", "1.1", "-", or nothing. TEXT and
	// never a number — parsing it loses "A" and orders "1.10" before "1.2".
	TT  string
	Ten string

	ThuTu    int
	CachTinh CachTinh
	Cap      int

	// LaDongTong is `is_headline`. See the block at the top of this file; nothing infers it.
	LaDongTong bool
}

// --- the refusals -------------------------------------------------------------------------------

var (
	ErrThieuBang             = errors.New("ngan_sach: thiếu bảng ngân sách cho khoản mục")
	ErrKhongThayBang         = errors.New("ngan_sach: xã chưa có bảng ngân sách cho năm và loại này")
	ErrLoaiBangSai           = errors.New("ngan_sach: `kind` phải là `thu` hoặc `chi`")
	ErrNamNgoaiLich          = errors.New("ngan_sach: `year` ngoài khoảng năm hợp lệ (2000..2100)")
	ErrKieuCotSai            = errors.New("ngan_sach: `type` của cột phải là `so` hoặc `phan_tram`")
	ErrVaiTroSai             = errors.New("ngan_sach: `role` của cột không thuộc bộ mã đã chốt")
	ErrVaiTroSaiLoaiBang     = errors.New("ngan_sach: `role` này không thuộc loại bảng đang thao tác")
	ErrVaiTroTrenCotPhanTram = errors.New("ngan_sach: cột `phan_tram` không mang `role` — chỉ số đọc từ cột số")
	ErrThieuCongThuc         = errors.New("ngan_sach: cột `phan_tram` phải có `formula`")
	ErrThuaCongThuc          = errors.New("ngan_sach: cột `so` không mang `formula`")
	ErrVaiTroTrungTrongBang  = errors.New("ngan_sach: một vai trò chỉ gán được cho MỘT cột trong bảng")

	ErrThieuTieuDe    = errors.New("ngan_sach: thiếu `title` của bảng")
	ErrTieuDeQuaDai   = errors.New("ngan_sach: `title` quá dài")
	ErrThieuDonViTinh = errors.New("ngan_sach: thiếu `unit`")
	ErrDonViTinhSai   = errors.New("ngan_sach: `unit` phải là `dong`, `nghin-dong` hoặc `trieu-dong`")
	ErrThieuTenCot    = errors.New("ngan_sach: thiếu `name` của cột")
	ErrTenCotQuaDai   = errors.New("ngan_sach: `name` của cột quá dài")
	ErrCongThucQuaDai = errors.New("ngan_sach: `formula` quá dài")
	ErrKhongCoCotNao  = errors.New("ngan_sach: bảng phải có ít nhất một cột")
	ErrQuaNhieuCot    = errors.New("ngan_sach: bảng vượt số cột tối đa")

	ErrThieuTenKhoanMuc   = errors.New("ngan_sach: thiếu `name` của khoản mục")
	ErrTenKhoanMucQuaDai  = errors.New("ngan_sach: `name` của khoản mục quá dài")
	ErrTTQuaDai           = errors.New("ngan_sach: `no` của khoản mục quá dài")
	ErrThuTuNgoaiKhoangNS = errors.New("ngan_sach: `order` ngoài khoảng cho phép")
	ErrGiaTriQuaLon       = errors.New("ngan_sach: giá trị vượt mức một dòng ngân sách cấp xã có thể có")

	// ErrTongVuotMuc — a parent's sum (or the balance) leaves GiaTriToiDa or int64. A READ-SIDE
	// reason, never a write refusal: the cell is shown unavailable with this sentence instead of a
	// wrapped or clamped figure. The sentence never quotes a figure (it travels into logs).
	ErrTongVuotMuc = errors.New(
		"ngan_sach: tổng cộng ra vượt mức một con số ngân sách hiển thị chính xác được — không tính được; kiểm tra các số quá lớn ở các dòng bên dưới")

	// ErrGiaTriDaLuuVuotMuc — a value STORED before GiaTriToiDa was lowered (25/09/2026) lies past
	// it. Read and flagged, never refused on read.
	ErrGiaTriDaLuuVuotMuc = errors.New(
		"ngan_sach: số đang lưu vượt mức một con số ngân sách hiển thị chính xác được — gõ lại số đúng hoặc gỡ đợt ghi nhầm")

	// ErrTruongBangKhongSua — a PATCH on a sheet named a field that identifies it. REFUSED RATHER THAN
	// IGNORED: `year`/`kind` decide which report the figures belong to and `code` is the handle the
	// audit trail is filed under; a client watching them vanish silently would believe it had moved a
	// year's budget. Columns are not editable at all (see the header of internal/app's budget file).
	ErrTruongBangKhongSua = errors.New(
		"ngan_sach: `year`, `kind`, `code` và `columns` của bảng không sửa được — sai năm hoặc loại thì gỡ bảng và tạo lại")

	ErrThieuLyDoXoaNganSach  = errors.New("ngan_sach: thiếu lý do gỡ")
	ErrLyDoXoaNganSachQuaDai = errors.New("ngan_sach: lý do gỡ quá dài")

	// ErrCachTinhDoTuClient — a request tried to set `cach_tinh` to something a client may not choose:
	// any value on CREATE (a new line is always a `manual` leaf), or anything but `manual` / `entries`
	// on an edit.
	//
	// REFUSED RATHER THAN IGNORED, and the difference is what the client learns. `children` follows
	// from whether the line has children (CachTinhTheoCay), which is the customer's decision of
	// 06/09/2026 expressed as a property. A client that could set it would be a client that could
	// mark a parent `manual` — which is precisely the direct entry the decision forbids. Choosing
	// between `manual` and `entries` on a LEAF is the user's decision of 25/09/2026 (§4.2).
	ErrCachTinhDoTuClient = errors.New(
		"ngan_sach: `method` không đặt khi thêm khoản mục; khi sửa chỉ nhận `manual` hoặc `entries` cho khoản mục lá — `children` suy ra từ cây")

	// ErrCapDoTuClient — a request tried to set `cap`. It is derived from the parent.
	ErrCapDoTuClient = errors.New("ngan_sach: `level` không do client đặt — độ sâu suy ra từ khoản mục cha")

	// ErrDongTongQuaTuyenRieng — a request tried to set `is_headline` on a create or an edit.
	//
	// REFUSED RATHER THAN IGNORED. The flag is reachable from no layer above the store except the
	// star's own statement pair, so ignoring it would be safe — and would leave the client believing
	// it had just made this row the commune's reported total. ADR 0035 §A is why that act has a route
	// of its own: which row the total comes from decides the figures in a document sent to a higher
	// authority, and it is one act, with one audit entry naming the row it moved from.
	ErrDongTongQuaTuyenRieng = errors.New(
		"ngan_sach: `is_headline` không đặt kèm khi thêm/sửa — dùng tuyến đánh dấu dòng tổng riêng")

	// ErrKhoanMucChaKhongGoThang — THE CUSTOMER'S DECISION OF 06/09/2026 (anh Hà), carried over from
	// `../vigov-require` commit `502d6f4` and blocked HERE, in the business layer, rather than only
	// on the screen.
	//
	// THE PRICE THE CUSTOMER ACCEPTED, and it is written into the refusal itself so that whoever
	// meets it knows this is a decision and not a defect: real forms have rows where the parent is
	// NOT the sum of its children — khoản mục ngoài cân đối, and the `Trong đó:` lines that restate
	// part of the row above. On those rows the number on the screen WILL DIFFER FROM THE PAPER THE
	// COMMUNE SIGNED.
	ErrKhoanMucChaKhongGoThang = errors.New(
		"ngan_sach: khoản mục có dòng con thì không gõ số thẳng — số của nó luôn cộng từ các dòng con")

	// ErrConGiuKhoanMucCon — removing a line that still has children.
	//
	// REFUSED RATHER THAN CASCADED. A cascade would take rows off every total in one click, and the
	// rows are archival: they stay in the table carrying `deleted_at`, so "undo" is not a button, it
	// is re-entering them. Making the commune remove the children first makes the size of the act
	// visible while it is still being decided.
	ErrConGiuKhoanMucCon = errors.New(
		"ngan_sach: khoản mục còn dòng con thì chưa gỡ được — gỡ các dòng con trước")

	// ErrChaKhongCungBang — a parent in another sheet. Silently accepted, it would put one year's
	// line under another year's tree and total them together.
	ErrChaKhongCungBang = errors.New("ngan_sach: khoản mục cha phải nằm trong cùng một bảng")

	// ErrChaLaChinhNo / ErrChaTaoVongLap — a cycle in the tree. Nothing in the database stops it
	// (migration 0006 states the cost of having no foreign key), so it is refused here, inside the
	// transaction, and the read path below is written so that a cycle that somehow exists cannot
	// hang a request.
	ErrChaLaChinhNo  = errors.New("ngan_sach: khoản mục không thể là cha của chính nó")
	ErrChaTaoVongLap = errors.New("ngan_sach: khoản mục cha nằm bên dưới chính khoản mục này — cây sẽ thành vòng")

	// ErrCotKhongThuocBang / ErrCotKhongPhaiCotSo — writing a figure into a column that is not this
	// sheet's, or into a percentage column. §9 rule 3: a percentage is computed at render, never
	// stored; a stored one is a second home for a derivable number.
	ErrCotKhongThuocBang = errors.New("ngan_sach: cột không thuộc bảng của khoản mục này")
	ErrCotKhongPhaiCotSo = errors.New("ngan_sach: cột `phan_tram` không nhận giá trị — phần trăm tính khi hiển thị")

	// ErrKhongThayKhoanMuc / ErrKhongThayCot are "no such live row IN THIS COMMUNE" — the two halves
	// are one answer on purpose. A row of another commune is indistinguishable from one that does not
	// exist, because the query cannot reach it at all.
	ErrKhongThayKhoanMuc = errors.New("ngan_sach: không có khoản mục này trong xã")
	ErrKhongThayCot      = errors.New("ngan_sach: không có cột này trong xã")
)

// The bounds. Not business rules and not pretending to be: they are the point past which a value
// stops being a budget field and starts being a mistake or an attack.
const (
	TieuDeBangToiDa      = 300
	TenCotToiDa          = 200
	CongThucToiDa        = 300
	TenKhoanMucToiDa     = 500
	TTKhoanMucToiDa      = 30
	LyDoXoaNganSachToiDa = 500

	// ThuTuNganSachToiDa bounds `thu_tu`. The chi sheet of §10 has 59 lines; 100 000 is five
	// thousand times that, which leaves room for an import that numbers by 100 and none for a value
	// that is really a year or a timestamp.
	ThuTuNganSachToiDa = 100_000

	// SoCotToiDa — the thu tab draws six columns and the chi tab four (§3). Thirty is five times the
	// larger, and the point past which the thing being loaded is not a budget report.
	SoCotToiDa = 30

	// GiaTriToiDa is 2^53 − 1 đồng (≈ 9,007 × 10^15) in absolute value — JavaScript's
	// Number.MAX_SAFE_INTEGER, the largest integer the browser reads from JSON EXACTLY. It bounds
	// every typed cell, every batch amount, AND every figure this service computes (a parent's sum,
	// an entries leaf's batch sum, the balance): a computed figure past it comes back UNAVAILABLE
	// with a reason (BangDayDu.GiaTri), never clamped and never sent.
	//
	// WHY THIS NUMBER AND NOT 10^17 (the value until 25/09/2026): above 2^53 a JSON number silently
	// rounds to the nearest representable double in the browser — 9 007 199 254 740 993 arrives as
	// …992 — so a figure the server holds exactly would be shown one đồng off, and the web client
	// fails closed on such values rather than draw them. A ceiling the server accepts but the only
	// client cannot display is a cell nobody can see. Real figures are far below it: the
	// specification's whole commune plans 3,99 × 10^12 đồng of revenue a year (§3.2), three orders
	// of magnitude under. It is still a TYPO GUARD first — a figure pasted with its separators
	// stripped, or triệu đồng typed as đồng — exactly as SoTienToiDa is for a voucher.
	//
	// ROWS STORED UNDER THE OLD CEILING ARE STILL READ: a stored value past this bound is shown as
	// unavailable with ErrGiaTriDaLuuVuotMuc, never refused on read — refusing would lock the whole
	// sheet, including the edit that fixes the cell.
	//
	// IT APPLIES IN BOTH DIRECTIONS because a budget value may legitimately be NEGATIVE (§9 rule 4),
	// unlike a disbursement voucher.
	GiaTriToiDa Dong = 9_007_199_254_740_991
)

// --- validation ---------------------------------------------------------------------------------

func KiemTraNamNganSach(nam int) error {
	if nam < NamChungTuSom || nam > NamChungTuMuon {
		return ErrNamNgoaiLich
	}
	return nil
}

func KiemTraLoaiBang(l LoaiBang) error {
	if l != BangThu && l != BangChi {
		return ErrLoaiBangSai
	}
	return nil
}

// KiemTraCot validates one column declaration against the sheet it is being added to.
//
// ALL FOUR RULES IN ONE PLACE, because three of them are pairwise: the kind decides whether a
// formula is required, the kind decides whether a role is allowed, and the sheet's kind decides
// WHICH roles are allowed. Checked separately they drift, and the drift is a column that carries a
// `thu` role on a `chi` sheet — an indicator reading a figure that is not what it is named after.
func KiemTraCot(loai LoaiBang, c CotNganSach) error {
	if c.Kieu != CotSo && c.Kieu != CotPhanTram {
		return ErrKieuCotSai
	}
	switch {
	case c.Kieu == CotPhanTram && c.CongThuc == "":
		return ErrThieuCongThuc
	case c.Kieu == CotSo && c.CongThuc != "":
		return ErrThuaCongThuc
	}
	if c.VaiTro == "" {
		return nil
	}
	if c.Kieu != CotSo {
		return ErrVaiTroTrenCotPhanTram
	}
	return KiemTraVaiTro(loai, c.VaiTro)
}

// KiemTraVaiTro reports whether this role is one of the six AND belongs on this kind of sheet.
func KiemTraVaiTro(loai LoaiBang, v VaiTroCot) error {
	hop, biet := vaiTroCuaLoai[loai]
	if !biet {
		return ErrLoaiBangSai
	}
	for _, m := range hop {
		if m == v {
			return nil
		}
	}
	// A role that exists but belongs to the other kind gets its OWN sentence, because the two
	// mistakes need two different corrections: one is a typo, the other is the wrong tab.
	for _, moi := range vaiTroCuaLoai {
		for _, m := range moi {
			if m == v {
				return fmt.Errorf("%w (`%s`)", ErrVaiTroSaiLoaiBang, v)
			}
		}
	}
	return fmt.Errorf("%w (`%s`)", ErrVaiTroSai, v)
}

// KiemTraBoCot validates the whole column set of a new sheet at once.
//
// THE DUPLICATE-ROLE CHECK CANNOT BE DONE PER COLUMN, which is why it is here: `UNIQUE (tenant_id,
// bang_id, vai_tro)` in migration 0006 is the floor, and this is the sentence that arrives first —
// two columns both marked `Thu xã hưởng` is a sheet where the `Cân đối` cell has two candidate
// answers and no way to choose.
func KiemTraBoCot(loai LoaiBang, cot []CotNganSach) error {
	switch {
	case len(cot) == 0:
		return ErrKhongCoCotNao
	case len(cot) > SoCotToiDa:
		return fmt.Errorf("%w (tối đa %d cột)", ErrQuaNhieuCot, SoCotToiDa)
	}
	daCo := map[VaiTroCot]struct{}{}
	for _, c := range cot {
		if err := KiemTraCot(loai, c); err != nil {
			return err
		}
		if c.VaiTro == "" {
			continue
		}
		if _, trung := daCo[c.VaiTro]; trung {
			return fmt.Errorf("%w (`%s`)", ErrVaiTroTrungTrongBang, c.VaiTro)
		}
		daCo[c.VaiTro] = struct{}{}
	}
	return nil
}

func ChuanHoaTieuDeBang(s string) (string, error) {
	return chuanHoaBatBuoc(s, TieuDeBangToiDa, ErrThieuTieuDe, ErrTieuDeQuaDai)
}

// --- the sheet's unit: a CLOSED list (the customer's decision of 25/09/2026) ------------------------
//
// DonViTinh is the unit the SCREEN prints the sheet's figures in, and NOTHING ELSE. Every stored
// figure is BIGINT đồng (migration 0006's MONEY block) and stays đồng whatever this says: the web
// converts what the accountant types into đồng before sending, and divides by the factor when it
// draws. Changing a sheet's unit therefore changes how its numbers are DISPLAYED and never rescales
// one of them — a unit that rewrote stored figures would silently multiply a public authority's
// budget by a thousand.
//
// THE WIRE CODE AND THE STORED TEXT ARE DIFFERENT ON PURPOSE. The wire carries the ASCII code
// (kebab-case, like the column roles); the column `don_vi_tinh` keeps the Vietnamese label, because
// that is what every row written before 25/09/2026 already holds (the column default is
// 'Triệu đồng'), and writing the label keeps the column one shape instead of two. Validation lives
// HERE and not in a CHECK: 0006 cannot be edited (its checksum is recorded), and a new CHECK would
// first have to decide what happens to legacy free-text rows — a rewrite of populated archival data
// that is not this change's to make.
type DonViTinh string

const (
	DonViDong      DonViTinh = "dong"
	DonViNghinDong DonViTinh = "nghin-dong"
	DonViTrieuDong DonViTinh = "trieu-dong"
)

// nhanDonViTinh is the label written to `don_vi_tinh` and printed on the screen. "Triệu đồng" is
// spelled exactly as migration 0006's default, so a legacy row holding the default and a new row are
// the same bytes.
var nhanDonViTinh = map[DonViTinh]string{
	DonViDong:      "Đồng",
	DonViNghinDong: "Nghìn đồng",
	DonViTrieuDong: "Triệu đồng",
}

// Nhan is the label for this unit, "" for a value outside the closed list.
func (d DonViTinh) Nhan() string { return nhanDonViTinh[d] }

// KiemTraDonViTinh accepts EXACTLY one of the three codes on a write. A label ("Triệu đồng") is
// refused here even though the read path recognises it: one input vocabulary is a contract, two
// is a guessing game that grows a third spelling every time a client is written.
func KiemTraDonViTinh(ma string) (DonViTinh, error) {
	if strings.TrimSpace(ma) == "" {
		return "", ErrThieuDonViTinh
	}
	d := DonViTinh(ma)
	if _, co := nhanDonViTinh[d]; !co {
		return "", ErrDonViTinhSai
	}
	return d, nil
}

// donViTinhCu maps the free text a sheet created before 25/09/2026 may hold onto a code, after
// lower-casing and collapsing whitespace. EVERY ENTRY NAMES ONE SCALE WITHOUT DOUBT — "ngàn" is the
// southern spelling of "nghìn", "1.000 đồng" is how Vietnamese budget forms write the thousand unit,
// "trđ" is the standard abbreviation of triệu đồng. Anything else ("tỷ đồng", "triệu", a typo) is NOT
// guessed: DocDonViTinhDaLuu answers false and the screen warns, because a wrong guess here makes
// every figure on the sheet display a thousand times too large or too small.
var donViTinhCu = map[string]DonViTinh{
	"đồng": DonViDong, "vnđ": DonViDong, "vnd": DonViDong, "đ": DonViDong, "dong": DonViDong,
	"nghìn đồng": DonViNghinDong, "ngàn đồng": DonViNghinDong,
	"1.000 đồng": DonViNghinDong, "1000 đồng": DonViNghinDong, "nghin-dong": DonViNghinDong,
	"triệu đồng": DonViTrieuDong, "trđ": DonViTrieuDong,
	"1.000.000 đồng": DonViTrieuDong, "trieu-dong": DonViTrieuDong,
}

// DocDonViTinhDaLuu reads what `don_vi_tinh` holds back into a code. false means the stored text is
// not one of the three units unambiguously — the caller reports it, it never substitutes a default.
// A text that is not NFC-normalised will not match and is reported too: that is the safe direction.
func DocDonViTinhDaLuu(luu string) (DonViTinh, bool) {
	d, co := donViTinhCu[strings.Join(strings.Fields(strings.ToLower(luu)), " ")]
	return d, co
}

func ChuanHoaTenCot(s string) (string, error) {
	return chuanHoaBatBuoc(s, TenCotToiDa, ErrThieuTenCot, ErrTenCotQuaDai)
}

func ChuanHoaTenKhoanMuc(s string) (string, error) {
	return chuanHoaBatBuoc(s, TenKhoanMucToiDa, ErrThieuTenKhoanMuc, ErrTenKhoanMucQuaDai)
}

func ChuanHoaLyDoXoaNganSach(s string) (string, error) {
	return chuanHoaBatBuoc(s, LyDoXoaNganSachToiDa, ErrThieuLyDoXoaNganSach, ErrLyDoXoaNganSachQuaDai)
}

// ChuanHoaCongThuc — OPTIONAL and never evaluated. A `so` column must not carry one; a `phan_tram`
// column must (KiemTraCot decides which), and the string is stored verbatim for the client to
// render §9 rule 3 with.
func ChuanHoaCongThuc(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > CongThucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCongThucQuaDai, CongThucToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrCongThucQuaDai
	}
	return s, nil
}

// ChuanHoaTT — OPTIONAL. §4.1: "A", "I", "1.1", "-" and blank are all real, and a blank one is what
// the `Đầu tư cho các DA…` row of §4.1's own sample carries.
func ChuanHoaTT(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > TTKhoanMucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTTQuaDai, TTKhoanMucToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrTTQuaDai
	}
	return s, nil
}

func KiemTraThuTuNganSach(t int) error {
	if t < 0 || t > ThuTuNganSachToiDa {
		return fmt.Errorf("%w (0..%d)", ErrThuTuNgoaiKhoangNS, ThuTuNganSachToiDa)
	}
	return nil
}

// KiemTraGiaTri bounds one cell. ZERO AND NEGATIVE ARE BOTH LEGAL (§9 rule 4) — the only thing
// refused is a magnitude that is not a budget figure. Do not "fix" this into `> 0`: that rule
// belongs to `chung_tu_giai_ngan`, where a negative is a refund pretending to be a payment
// (ADR 0035 §B), and it is a different table with a different meaning.
func KiemTraGiaTri(g Dong) error {
	if g > GiaTriToiDa || g < -GiaTriToiDa {
		return fmt.Errorf("%w (|giá trị| tối đa %d đồng)", ErrGiaTriQuaLon, int64(GiaTriToiDa))
	}
	return nil
}

func chuanHoaBatBuoc(s string, toiDa int, thieu, quaDai error) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", thieu
	case len([]rune(s)) > toiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", quaDai, toiDa)
	case coKyTuDieuKhien(s):
		return "", thieu
	}
	return s, nil
}

// --- the whole sheet, as a screen sees it ---------------------------------------------------------

// BangDayDu is one sheet with everything drawn from it: its columns, every live line, and the value
// of every filled cell.
//
// `Gia` HOLDS ONLY FILLED CELLS, AND THAT IS THE WHOLE POINT OF THE MAP. §9 rule 4 makes "trống"
// and "0" two different statements — the screen draws `—` for one and `0` for the other — so an
// absent key means empty and a present key means a figure the commune stated, including zero. A
// `map[string]Dong` with a zero default would collapse the two, and the collapse is invisible.
type BangDayDu struct {
	Bang     BangNganSach
	Cot      []CotNganSach      // ordered by thu_tu
	KhoanMuc []KhoanMucNganSach // ordered by thu_tu

	// Gia is khoanMucID -> cotID -> value. Only cells with a non-NULL `gia_tri` appear.
	Gia map[string]map[string]Dong

	// GiaDot is khoanMucID -> cotID -> the SUM of that line's LIVE batches (migration 0008). Same
	// convention as Gia: a key appears only when at least one live batch stated an amount there, so
	// an absent key is EMPTY (`—`) and never 0 (§9 rule 4). Read for a leaf in `entries` mode only;
	// a line in any other mode keeps its batches in the table and they count nowhere.
	GiaDot map[string]map[string]Dong

	// GiaDotVuotMuc is khoanMucID -> cotID -> true where the batch SUM does not fit int64 at all
	// (SUM(BIGINT) is NUMERIC in PostgreSQL and can exceed it). The sum cannot sit in GiaDot, and
	// failing the whole sheet read for it would lock every write — GoDot included, which is the only
	// way to remove the offending batch. So the fact is carried here and GiaTri turns that ONE cell
	// into a figure with a reason.
	GiaDotVuotMuc map[string]map[string]bool
}

// ConTrucTiep returns the DIRECT children of one line, in display order.
//
// DIRECT ONLY, AND §9 RULE 2 SAYS WHY: a `children` line sums its direct children, each of which
// may itself be a `children` line that has already summed ITS children. Summing grandchildren as
// well would count every level below twice over.
func (b BangDayDu) ConTrucTiep(chaID string) []KhoanMucNganSach {
	var ra []KhoanMucNganSach
	for _, k := range b.KhoanMuc {
		if k.ChaID == chaID && k.ID != chaID {
			ra = append(ra, k)
		}
	}
	sort.SliceStable(ra, func(i, j int) bool { return ra[i].ThuTu < ra[j].ThuTu })
	return ra
}

// CoCon reports whether this line has at least one live child — the single question the customer's
// 06/09/2026 decision turns on.
func (b BangDayDu) CoCon(id string) bool {
	for _, k := range b.KhoanMuc {
		if k.ChaID == id && k.ID != id {
			return true
		}
	}
	return false
}

// GiaTri is the figure of one cell as the screen shows it: typed in for a `manual` leaf, the sum of
// its live batches for an `entries` leaf, summed from the direct children for a parent — so a parent
// over an `entries` leaf picks the batch sum up through the same recursion.
//
// AN EMPTY CELL IS Co == false, never a zero — §9 rule 4 again. A parent all of whose children are empty is EMPTY, not 0: a commune that has
// not yet entered a section must not see that section reported as nil spending.
//
// THE RECURSION CANNOT HANG. Nothing in the database stops a cycle in `cha_id` (migration 0006
// states that cost), and a read must survive one that somehow exists — so the walk carries the set
// of lines already visited and treats a revisit as an empty branch. It refuses rather than looping,
// which is a wrong figure the screen can show; looping is a request that never returns.
//
// THREE OUTCOMES, NOT TWO, in one SoTien:
//
//	Co == true                 a figure
//	Co == false, LyDo == ""    an EMPTY cell (`—`)
//	Co == false, LyDo != ""    a figure that CANNOT BE COMPUTED — shown as the reason, never as a
//	                           number and never as empty
//
// The third exists because a sum can leave what int64 holds, or what the browser reads exactly
// (GiaTriToiDa). `tong += g` unchecked wraps to a plausible negative figure on a parent row; a
// clamp prints a number nobody entered. Both reach a report as a healthy-looking figure. A figure
// with a reason is a job for the commune ("gỡ đợt nhập sai"), and it is ISOLATED: only the cells
// that depend on it — the line and its ancestors in that column — are unavailable; the rest of the
// sheet reads and every write still works (the write that fixes it included).
func (b BangDayDu) GiaTri(khoanMucID, cotID string) SoTien {
	g, co, err := b.giaTri(khoanMucID, cotID, map[string]struct{}{})
	switch {
	case err != nil:
		return soTienKhong(err)
	case co:
		return soTienCo(g)
	default:
		return SoTien{}
	}
}

// giaTri returns (figure, present, why-not-computable). err != nil means UNAVAILABLE and wins over
// everything: a parent with one unavailable child is unavailable, because summing the others would
// print a total that silently leaves a line out.
func (b BangDayDu) giaTri(khoanMucID, cotID string, daQua map[string]struct{}) (Dong, bool, error) {
	if _, lap := daQua[khoanMucID]; lap {
		return 0, false, nil
	}
	daQua[khoanMucID] = struct{}{}

	con := b.ConTrucTiep(khoanMucID)
	if len(con) == 0 {
		// THE MODE DECIDES THE SOURCE, and only for a leaf: a line with children sums them whatever
		// its stored mode says (the tree wins, CachTinhTheoCay). The typed figures of an `entries`
		// leaf stay stored and are NOT shown — §9.1's "giá trị nhập tay bị khoá".
		k, coK := b.TheoID(khoanMucID)
		if coK && k.CachTinh == TinhTheoDot {
			if b.GiaDotVuotMuc[khoanMucID][cotID] {
				return 0, false, fmt.Errorf("%w (dòng %q)", ErrTongDotVuotMuc, k.Ten)
			}
			g, coSo := b.GiaDot[khoanMucID][cotID]
			if coSo && vuotGiaTriToiDa(g) {
				return 0, false, fmt.Errorf("%w (dòng %q)", ErrTongDotVuotMuc, k.Ten)
			}
			return g, coSo, nil
		}
		g, co := b.Gia[khoanMucID][cotID]
		if co && vuotGiaTriToiDa(g) {
			// A row stored under the old ceiling (10^17). READ, never refused: refusing would fail the
			// whole sheet and lock the very PATCH that corrects this cell.
			return 0, false, fmt.Errorf("%w (dòng %q)", ErrGiaTriDaLuuVuotMuc, k.Ten)
		}
		return g, co, nil
	}

	var tong Dong
	var coGiNao bool
	for _, c := range con {
		g, co, err := b.giaTri(c.ID, cotID, daQua)
		if err != nil {
			// The child's own reason, unchanged: it names the line that has to be fixed, which is
			// the only line where fixing is possible. Every ancestor repeats it rather than
			// stacking "dòng con của dòng con của…".
			return 0, false, err
		}
		if !co {
			continue
		}
		moi, tran := congKiemTra(tong, g)
		if tran {
			return 0, false, b.loiTongVuotMuc(khoanMucID)
		}
		tong = moi
		coGiNao = true
	}
	// Checked at the END and not per step: a positive and a negative child may legitimately pass
	// through a larger partial sum on the way to a total that is fine. Overflow of int64 itself is
	// caught per step above, because past that point the partial sum is no longer a number at all.
	if coGiNao && vuotGiaTriToiDa(tong) {
		return 0, false, b.loiTongVuotMuc(khoanMucID)
	}
	return tong, coGiNao, nil
}

func (b BangDayDu) loiTongVuotMuc(khoanMucID string) error {
	k, _ := b.TheoID(khoanMucID)
	return fmt.Errorf("%w (dòng %q)", ErrTongVuotMuc, k.Ten)
}

// vuotGiaTriToiDa — past the bound every figure on this board must respect, in either direction.
func vuotGiaTriToiDa(g Dong) bool { return g > GiaTriToiDa || g < -GiaTriToiDa }

// congKiemTra adds two amounts and reports int64 overflow instead of wrapping. Two operands of the
// same sign whose sum has the other sign are exactly the wrapped cases.
func congKiemTra(a, b Dong) (Dong, bool) {
	s := a + b
	if (a > 0 && b > 0 && s < 0) || (a < 0 && b < 0 && s >= 0) {
		return 0, true
	}
	return s, false
}

// KiemTraGiaTriDaLuu classifies a STORED amount on the way OUT (a batch amount on the `⇄` list).
// Past GiaTriToiDa it is not sent as a number — the browser would round it — but flagged with
// ErrGiaTriDaLuuVuotMuc so the accountant can find the batch and remove it.
func KiemTraGiaTriDaLuu(g Dong) error {
	if vuotGiaTriToiDa(g) {
		return ErrGiaTriDaLuuVuotMuc
	}
	return nil
}

// TheoID finds one line of this sheet.
func (b BangDayDu) TheoID(id string) (KhoanMucNganSach, bool) {
	for _, k := range b.KhoanMuc {
		if k.ID == id {
			return k, true
		}
	}
	return KhoanMucNganSach{}, false
}

// CotTheoVaiTro finds the column the commune marked with this role.
func (b BangDayDu) CotTheoVaiTro(v VaiTroCot) (CotNganSach, bool) {
	for _, c := range b.Cot {
		if c.VaiTro == v {
			return c, true
		}
	}
	return CotNganSach{}, false
}

// --- the total row ---------------------------------------------------------------------------------

var (
	// ErrChuaDanhDauDongTong — nobody has starred a row yet. A SENTENCE AND NOT A ZERO: ADR 0035 §A
	// forbids a "temporary" default here, and the first row is exactly the guess that is right on
	// one form and wrong on the next.
	ErrChuaDanhDauDongTong = errors.New(
		"ngan_sach: chưa đánh dấu dòng nào là dòng tổng — bấm ngôi sao ở đầu dòng tổng của biểu này")

	// ErrNhieuDongTong — two or more rows claim to be the total.
	//
	// THE APPLICATION MAKES THE MARK A RADIO, so this state is not reachable through any route in
	// this service. It is refused anyway, and that is the point: the database carries no partial
	// unique index for it (migration 0006 says why it cannot be verified here), so the state can
	// arrive from an import, a psql session, or a future writer. Answering with the first one, or
	// with their sum, is the double count `../vigov-require` measured — the very thing `is_headline`
	// was introduced to end.
	ErrNhieuDongTong = errors.New(
		"ngan_sach: có nhiều hơn một dòng được đánh dấu là dòng tổng — cộng cả hai là đếm đôi; giữ đúng một dòng")
)

// DongTong is the row the summary cells are read from.
//
// IT IS THE MARKED ROW OR NOTHING. Not the first row, not the shallowest, not the one whose `tt` or
// name looks like a total. The three sentences at the top of this file are all in this one function.
func (b BangDayDu) DongTong() (KhoanMucNganSach, error) {
	var thay []KhoanMucNganSach
	for _, k := range b.KhoanMuc {
		if k.LaDongTong {
			thay = append(thay, k)
		}
	}
	switch len(thay) {
	case 0:
		return KhoanMucNganSach{}, ErrChuaDanhDauDongTong
	case 1:
		return thay[0], nil
	default:
		return KhoanMucNganSach{}, fmt.Errorf("%w (%d dòng)", ErrNhieuDongTong, len(thay))
	}
}

// SoTong is one summary figure: the value of the marked row in the column carrying this role.
//
// FOUR WAYS TO HAVE NO NUMBER, AND EACH GETS ITS OWN SENTENCE, because each needs a different act
// from the commune: star a row · keep only one · mark the column · type the figure in. A single
// "không có dữ liệu" would leave all four looking like a broken screen.
func (b BangDayDu) SoTong(v VaiTroCot) (Dong, bool, error) {
	dong, err := b.DongTong()
	if err != nil {
		return 0, false, err
	}
	cot, co := b.CotTheoVaiTro(v)
	if !co {
		return 0, false, fmt.Errorf("%w (`%s`)", ErrChuaGanVaiTroCot, v)
	}
	g, coSo, err := b.giaTri(dong.ID, cot.ID, map[string]struct{}{})
	if err != nil {
		// A fifth way to have no number: the figure exists but cannot be computed. Its own sentence
		// names the line to fix; an indicator built on it would be as wrong as the figure.
		return 0, false, err
	}
	if !coSo {
		return 0, false, fmt.Errorf("%w (cột `%s`)", ErrDongTongChuaCoSo, cot.Ten)
	}
	return g, true, nil
}

var (
	// ErrChuaGanVaiTroCot — no column of this sheet carries the role the figure is defined in terms
	// of. ADR 0035 §A makes `Dự toán TP giao` A COLUMN THAT MUST HAVE DATA: leave it out and the
	// indicator DISAPPEARS, "và điều đó phải hiện ra thành một câu, không thành một ô trống".
	ErrChuaGanVaiTroCot = errors.New(
		"ngan_sach: chưa có cột nào của bảng được gán vai trò này — chỉ số không tính được")

	// ErrDongTongChuaCoSo — the marked row has no figure in that column. Same rule, other half:
	// an empty cell is not a zero and must not be reported as one.
	ErrDongTongChuaCoSo = errors.New(
		"ngan_sach: dòng tổng chưa có số ở cột này — chỉ số không tính được")

	// ErrMauSoBangKhong — the denominator is zero. §9 rule 3 says a percentage with a zero
	// denominator is shown BLANK, and the same holds for an indicator: a division by zero is not
	// infinity and is not 100%, it is an absent measurement.
	ErrMauSoBangKhong = errors.New(
		"ngan_sach: mẫu số bằng 0 nên tỷ lệ không có nghĩa — để trống thay vì hiện một con số")
)

// --- the figures that leave this screen ------------------------------------------------------------

// SoTien is one money figure with its provenance: either a number, or the reason there is none.
//
// A STRUCT AND NOT A `(Dong, error)` PAIR, because these travel to a handler that has to put the
// REASON on the screen beside an empty cell. A pair invites `if err != nil { return }`, and what
// reaches the commune is then a blank box (ADR 0035 §A: an empty box with a reason is a job to do;
// an empty box alone is a screen that looks broken).
type SoTien struct {
	Gia  Dong
	Co   bool
	LyDo string // "" when Co is true
}

func soTienCo(g Dong) SoTien       { return SoTien{Gia: g, Co: true} }
func soTienKhong(err error) SoTien { return SoTien{LyDo: err.Error()} }

// TyLe is one ratio with its provenance. Same shape and the same reason as SoTien.
type TyLe struct {
	Gia  PhanVan
	Co   bool
	LyDo string
}

func tyLeKhong(err error) TyLe { return TyLe{LyDo: err.Error()} }

// ChiSoDatDuToan is `Thu đạt dự toán` on a thu sheet and `Chi đạt dự toán` on a chi sheet (§9 rule 6).
//
// THE DENOMINATOR OF THE REVENUE ONE IS `Dự toán TP giao` — ADR 0035 #33, decided against
// `Dự toán Xã giao` which §9 rule 6 names. THE SPECIFICATION CONTRADICTS ITSELF and the ADR settles
// it: the summary cells of §3.2 produce 108,1% and only the city target does that, while the prose
// at §9 rule 6 matches no cell in its own file. A commune is judged on the target its superior set,
// and targets a commune sets for itself cannot be compared between communes.
//
// DO NOT "FIX" THIS TOWARD `Dự toán Xã giao`. Overturning it is the customer's to do, and ADR 0035's
// closing table says what it costs after the first reporting period: a correction to every report
// already sent upward.
func ChiSoDatDuToan(b BangDayDu) (string, TyLe) {
	var ten string
	var tuVaiTro, mauVaiTro VaiTroCot
	switch b.Bang.Loai {
	case BangThu:
		ten, tuVaiTro, mauVaiTro = "Thu đạt dự toán", VaiTroThuNSNN, VaiTroDuToanTPGiao
	case BangChi:
		ten, tuVaiTro, mauVaiTro = "Chi đạt dự toán", VaiTroChiNganSach, VaiTroDuToanNam
	default:
		return "", tyLeKhong(ErrLoaiBangSai)
	}

	tu, _, err := b.SoTong(tuVaiTro)
	if err != nil {
		return ten, tyLeKhong(err)
	}
	mau, _, err := b.SoTong(mauVaiTro)
	if err != nil {
		return ten, tyLeKhong(err)
	}
	if mau == 0 {
		return ten, tyLeKhong(ErrMauSoBangKhong)
	}
	return ten, TyLe{Gia: TyLeDong(tu, mau), Co: true}
}

// TyLeDong is a ratio of two amounts in parts per ten thousand, rounded to nearest.
//
// INTEGER ARITHMETIC THROUGHOUT, and the multiplication happens FIRST: `tu * 10000 / mau` keeps
// every digit the unit can express, while dividing first would throw away everything below a whole
// multiple of the denominator. Rounding is to nearest rather than toward zero so that 108,14% and
// 108,16% are not the same printed figure — the specification prints two decimals of a percentage.
//
// OVERFLOW IS NOT REACHABLE HERE: SoTong never hands out a figure past GiaTriToiDa (≈ 9 × 10^15),
// and 9 × 10^15 × 10^4 = 9 × 10^19 would overflow int64 — so the multiplication is done in a wider
// path by dividing the numerator when it is large.
// Written out because the same class of bug was caught by a test in du_an.go, where `nay.Sub(dau) *
// 10000` overflowed on nanoseconds.
func TyLeDong(tu, mau Dong) PhanVan {
	if mau == 0 {
		return 0
	}
	// 9,22 × 10^18 / 10^4 = 9,22 × 10^14. Above that the straight multiplication would overflow, so
	// scale both sides down first — losing at most a hundred đồng on a figure of a hundred thousand
	// billion, which is far below the unit being reported.
	const antoan = 9_000_000_000_000
	if tu > antoan || tu < -antoan {
		tu /= 1000
		mau /= 1000
		if mau == 0 {
			return 0
		}
	}
	tuSo := int64(tu) * 10_000
	mauSo := int64(mau)
	// Round half away from zero, on the sign of the QUOTIENT rather than of the numerator.
	nua := mauSo / 2
	if (tuSo < 0) != (mauSo < 0) {
		nua = -nua
	}
	return PhanVan((tuSo + nua) / mauSo)
}

// CanDoiThuChi is the `Cân đối thu - chi` cell of §9 rule 6: total revenue minus total expenditure
// for one budget year.
//
// `Tổng thu` IS `Thu xã hưởng` AND NOT `Thu ngân sách NSNN` — ADR 0035 #32. The reason is business
// and not cosmetic: a commune balances on the revenue it is ENTITLED TO KEEP under the allocation
// rules. Most of what arises in its territory is regulated upward and the commune may not spend it,
// so a balance built on the arising figure answers a real question that is not the one being asked,
// and in the specification's own sample it is over a million units larger — enough to show a surplus
// while the commune is short of money.
//
// IT TAKES BOTH SHEETS, so the caller has to have found them both. A missing sheet is a reason, not
// a zero: "the commune has not entered its expenditure sheet" and "the commune spent nothing" are
// different statements and only one of them is ever true.
func CanDoiThuChi(thu, chi BangDayDu) SoTien {
	if thu.Bang.Loai != BangThu {
		return soTienKhong(fmt.Errorf("%w (thiếu bảng thu)", ErrKhongThayBang))
	}
	if chi.Bang.Loai != BangChi {
		return soTienKhong(fmt.Errorf("%w (thiếu bảng chi)", ErrKhongThayBang))
	}
	tongThu, _, err := thu.SoTong(VaiTroThuXaHuong)
	if err != nil {
		return soTienKhong(fmt.Errorf("bảng thu: %w", err))
	}
	tongChi, _, err := chi.SoTong(VaiTroChiNganSach)
	if err != nil {
		return soTienKhong(fmt.Errorf("bảng chi: %w", err))
	}
	// Both operands are within ±GiaTriToiDa (SoTong guarantees it), so the difference fits int64 —
	// but it can reach twice the bound, past what the browser reads exactly. Unavailable, not clamped.
	canDoi := tongThu - tongChi
	if vuotGiaTriToiDa(canDoi) {
		return soTienKhong(fmt.Errorf("cân đối thu - chi: %w", ErrTongVuotMuc))
	}
	return soTienCo(canDoi)
}

// --- the write-path rules ---------------------------------------------------------------------------

// ChoGhiGiaTri reports whether a figure may be typed into this line.
//
// THE CUSTOMER'S DECISION OF 06/09/2026, ENFORCED IN THE BUSINESS LAYER AND NOT ONLY ON THE SCREEN
// (`../vigov-require` commit `502d6f4`). A screen-only rule is a rule that holds until somebody
// calls the API directly, which on a budget board is an import script written next year.
func ChoGhiGiaTri(coCon bool) error {
	if coCon {
		return ErrKhoanMucChaKhongGoThang
	}
	return nil
}

// ChoGo reports whether this line may be removed.
func ChoGo(coCon bool) error {
	if coCon {
		return ErrConGiuKhoanMucCon
	}
	return nil
}

// CapTheoCha is the depth of a line under a given parent. Derived here and nowhere else, so a value
// from a request body can never become a row's `cap`.
func CapTheoCha(cha KhoanMucNganSach, coCha bool) int {
	if !coCha {
		return 0
	}
	return cha.Cap + 1
}
