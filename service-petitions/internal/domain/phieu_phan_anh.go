package domain

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

// A petition — `phieu_phan_anh`, entity `Petition`, URL resource `citizen-reports`.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). The
// rules below are the ones ADR 0027 and ADR 0028 settled with the customer on 2026-09-20, and
// they are expressed here — as pure functions over values — precisely so they can be tested
// without a database, without a network and without a commune's configuration.

// --- the nine statuses ---------------------------------------------------------------------

// TrangThai is where a petition currently stands.
//
// THE LIST IS CLOSED AND THE CUSTOMER APPROVED THE NINE STRINGS VERBATIM on 2026-09-20
// (ADR 0027, and the §Bổ sung at the end of it). Two consequences, both load-bearing:
//
//   - the state machine below may name the codes DIRECTLY in source. That is only true because
//     the list is fixed; it would be wrong for a list a commune could edit.
//   - changing one of these strings is now a MIGRATION OF ARCHIVAL RECORDS (rule 7), not a
//     rename. The cheap moment to re-read them has been used.
//
// The codes are Vietnamese without diacritics, kebab-case: enum VALUES are never translated
// into English (ADR 0011). Codes and their Vietnamese labels have one owner,
// kb/00-foundation/ubiquitous-language.md §Chín trạng thái — the labels are deliberately NOT
// copied here, because a second copy of a display string is a second copy that drifts.
type TrangThai string

const (
	DaTiepNhan    TrangThai = "da-tiep-nhan"
	DangPhanLoai  TrangThai = "dang-phan-loai"
	DaChuyenXuLy  TrangThai = "da-chuyen-xu-ly"
	DangXuLy      TrangThai = "dang-xu-ly"
	DaXuLy        TrangThai = "da-xu-ly"
	ChoDanXacNhan TrangThai = "cho-dan-xac-nhan"
	DaDong        TrangThai = "da-dong"
	KhongTiepNhan TrangThai = "khong-tiep-nhan"
	ChuyenCapTren TrangThai = "chuyen-cap-tren"
)

// chuyenDuocSang is the whole lifecycle, and the ABSENCES are as deliberate as the entries.
//
// It follows docs/ui-ux/09-phan-anh-nguoi-dan.md §6 exactly, including the fact that the two
// branch statuses leave from `dang-phan-loai` AND FROM NOWHERE ELSE. That is not an omission in
// the specification: `dang-phan-loai` is the step where a human first reads the report and
// decides whether the commune takes it at all (§8.2, "Đang xem phiếu thuộc lĩnh vực nào, có
// tiếp nhận không"). Letting a petition be refused after it has already been assigned and
// worked on would be a different lifecycle, and rule 10, stop condition #2 puts that with the
// customer.
//
// `khong-tiep-nhan` and `chuyen-cap-tren` are TERMINAL. They appear as keys with empty lists
// rather than being left out, so the difference between "a state with no way out" and "a state
// nobody wrote down" is visible in the source.
//
// REOPENING (`cho-dan-xac-nhan` and `da-dong` back to `dang-xu-ly`) is in the map because §6
// puts it there: a 1–2 star rating reopens the petition. WHETHER it may is per-commune
// configuration — `cho_phep_mo_lai`, `nguong_sao_mo_lai`, `so_lan_mo_lai_toi_da` (ADR 0008) —
// and this map does not answer that. A transition being SHAPED correctly and a transition being
// PERMITTED are two questions; conflating them would hard-code a flag ADR 0008 created to be a
// flag.
//
// `da-xu-ly -> da-dong` WAS ADDED BY THE OWNER'S DECISION OF 2026-09-24: a petition with NOBODY WHO
// CAN CONFIRM IT — no citizen account behind it (`cong_dan_id` empty: staff-booked, `can-bo-nhap-ho`)
// — may be closed straight from `da-xu-ly`, with the closing result as the record. The edge being
// SHAPED here does not make it PERMITTED for every petition: domain.DongDuoc allows it only when
// `cong_dan_id` is empty, and a petition WITH a citizen still closes only from `cho-dan-xac-nhan`.
var chuyenDuocSang = map[TrangThai][]TrangThai{
	DaTiepNhan:    {DangPhanLoai},
	DangPhanLoai:  {DaChuyenXuLy, KhongTiepNhan, ChuyenCapTren},
	DaChuyenXuLy:  {DangXuLy},
	DangXuLy:      {DaXuLy},
	DaXuLy:        {ChoDanXacNhan, DaDong},
	ChoDanXacNhan: {DaDong, DangXuLy},
	DaDong:        {DangXuLy},
	KhongTiepNhan: {},
	ChuyenCapTren: {},
}

// ErrChuyenTrangThaiKhongHopLe is returned for a move the lifecycle does not have.
//
// FAIL CLOSED: a status the map does not know is refused, never allowed through as "probably
// fine". An unknown code can only come from data written before a migration, or from a caller
// that made one up — and a petition allowed to land in a state with no way out is a commitment
// to a citizen that stops moving, silently, forever.
var ErrChuyenTrangThaiKhongHopLe = errors.New("phieu_phan_anh: chuyển trạng thái không hợp lệ")

// HopLe reports whether the string is one of the nine.
func (t TrangThai) HopLe() bool {
	_, co := chuyenDuocSang[t]
	return co
}

// ChuyenSangDuoc reports whether this petition may move to m.
func (t TrangThai) ChuyenSangDuoc(m TrangThai) bool {
	for _, cho := range chuyenDuocSang[t] {
		if cho == m {
			return true
		}
	}
	return false
}

// KetThuc reports whether the lifecycle has no way out of this status.
func (t TrangThai) KetThuc() bool {
	return t.HopLe() && len(chuyenDuocSang[t]) == 0
}

// --- the four intake channels --------------------------------------------------------------

// KenhTiepNhan is where the petition came in (docs/ui-ux/09 §12).
type KenhTiepNhan string

const (
	KenhZaloMiniApp KenhTiepNhan = "zalo-mini-app"
	KenhZaloOA      KenhTiepNhan = "zalo-oa"
	KenhWebXa       KenhTiepNhan = "web-xa"
	KenhCanBoNhapHo KenhTiepNhan = "can-bo-nhap-ho"
)

func (k KenhTiepNhan) HopLe() bool {
	switch k {
	case KenhZaloMiniApp, KenhZaloOA, KenhWebXa, KenhCanBoNhapHo:
		return true
	}
	return false
}

// CoLinhVucLucVaoSo reports whether this channel's FORM carries the field at booking.
//
// THE RULE IS WRITTEN BY FORM SHAPE, NOT BY A LIST OF CHANNELS, and ADR 0028 §Quyết định E says
// why in one sentence: a fifth channel must not force the deadline rule to be rewritten. Read
// it as the ADR states it —
//
//	a channel whose form carries the field at booking fixes BOTH deadlines there;
//	a channel whose form does not defers `han_xu_ly_xong` to the first classification.
//
// Today only the staff-booked modal carries it (docs/ui-ux/09 §11, "Lĩnh vực" is a required
// field). The citizen channels do not, and deliberately will not: letting the citizen pick the
// field would let every pothole be filed as `An ninh trật tự` and take that field's 16-hour
// commitment, which is open question #23, closed in the other direction.
//
// ADDING A FIFTH CHANNEL WITHOUT ANSWERING THIS QUESTION IS A STOP CONDITION (ADR 0028, stop
// condition #6). A channel that falls through to `false` by default gets the deferred deadline
// by accident rather than by decision.
func (k KenhTiepNhan) CoLinhVucLucVaoSo() bool {
	return k == KenhCanBoNhapHo
}

// HanTiepNhanApDung reports whether the "somebody has read it" clock means anything here.
//
// It does NOT on the staff-booked channel: the officer IS the reader, so the interval being
// measured does not exist. The column is then NULL, and NULL means "KHÔNG ÁP DỤNG" — never
// `0 giờ`, because zero is a valid sample and would drag a commune's average acknowledge time
// toward nothing. Reports must EXCLUDE those rows, never COALESCE them (ADR 0028, decision F #5
// and its stop conditions #2 and #3).
func (k KenhTiepNhan) HanTiepNhanApDung() bool {
	return k != KenhCanBoNhapHo
}

// --- the record ------------------------------------------------------------------------------

// PhieuPhanAnh is one petition as the business sees it.
//
// THE TWO DEADLINE FIELDS ARE ZERO WHEN THE COLUMN IS NULL, AND THE TWO NULLS MEAN OPPOSITE
// THINGS. Do not read either of them with `IsZero()` at a call site; use the two predicates
// below, which say which question is being asked:
//
//	HanTiepNhanKhongApDung()  han_tiep_nhan IS NULL — "KHÔNG ÁP DỤNG", staff-booked
//	ChuaChotHanXuLy()         han_xu_ly_xong IS NULL — "CHƯA CÓ", not classified yet
//	TranPhanLoaiKhongApDung() han_phan_loai IS NULL — "KHÔNG ÁP DỤNG", staff-booked
//
// THERE IS NO OVERDUE FIELD ON THIS STRUCT AND THERE MUST NEVER BE ONE (rule 10, invariant 3).
// See QuaHan and QuaHanTiepNhan.
type PhieuPhanAnh struct {
	ID       string
	MaTraCuu string

	Kenh KenhTiepNhan

	// CongDanID is the opaque citizen identifier, empty when a member of staff booked the
	// petition with no citizen account behind it. IT IS STORED EVEN FOR AN ANONYMOUS PETITION —
	// see AnDanh.
	CongDanID string

	NoiDung string

	// LinhVuc holds a TIER-1 code AS A VALUE (ADR 0026): no foreign key, no JOIN across the
	// service boundary. Empty until an officer settles it, which on the citizen channels is also
	// the act that fixes HanXuLyXong.
	LinhVuc string

	DiaChi string
	ThonID string
	Lat    *float64
	Lng    *float64

	// PERSONAL DATA (rule 3, Decree 13/2023). Never logged, never in an error message, never in
	// a file name or a cache key. Masked on the way out unless the caller holds an explicit
	// full-view permission.
	NguoiGuiHoTen     string
	NguoiGuiDienThoai string

	// AnDanh hides the reporter from staff screens and from the public page. It does NOT mean
	// the identity was not recorded (ADR 0008): CongDanID is stored either way, because without
	// it there is no anti-spam, the citizen cannot find their own petition, and a defamatory
	// report becomes untraceable.
	AnDanh bool

	TrangThai TrangThai

	BoPhanID    string
	CanBoXuLyID string

	// GocDemHan is the instant BOTH clocks are counted from — when the citizen pressed send
	// (ADR 0027, decision D). Fixing a deadline LATER does not move its origin: the wait before
	// classification is charged against the resolve deadline, by design.
	GocDemHan time.Time

	// VaoSoLuc is when the row was created. On the staff-booked channel it can be up to seven
	// days after GocDemHan, and that gap is exactly how long the citizen has already waited.
	VaoSoLuc time.Time

	HanTiepNhan time.Time
	HanXuLyXong time.Time

	// HanPhanLoai is the CLASSIFICATION CEILING — ADR 0035 §C, which closed open question #26 on
	// 2026-09-22. It is a THIRD stored deadline, fixed at the same act as HanTiepNhan (the row being
	// created) and counted in working hours from the same GocDemHan, from GioTranPhanLoai.
	//
	// WHY IT IS STORED AND NOT DERIVED FROM HanTiepNhan: it is a commitment fixed by an act, exactly
	// like the other two (rule 10, invariant 2). ADR 0035 §C says so outright — "đổi trần về sau chỉ
	// áp cho phiếu nhận từ lúc đổi trở đi" — and a value recomputed on read would move every
	// already-received petition's ceiling the day the constant changed.
	//
	// IT IS ZERO (SQL NULL) ON A STAFF-BOOKED PETITION, with the same meaning as HanTiepNhan NULL:
	// "KHÔNG ÁP DỤNG". That channel's form carries the field at booking, so the petition is never
	// unclassified and there is no interval to bound.
	HanPhanLoai time.Time

	PhanLoaiLuc time.Time
	XuLyXongLuc time.Time
	DongLuc     time.Time

	// KetQuaXuLy is the result a CITIZEN reads, written when the petition is closed (rule 10,
	// invariant 6). It is NOT a staff note: staff notes and routing history stay internal (rule 4,
	// forbidden #5), and this one string is deliberately the only free text that crosses to the
	// citizen surface.
	//
	// IT NEVER TRAVELS ON THE EVENT. proto/vigov/petitions/v1/events.proto says why in full: free
	// text written about a specific case will eventually name the reporter or quote their complaint,
	// and a queue is persisted, replicated and backed up. The citizen reaches it with their lookup
	// code, behind an authenticated read.
	KetQuaXuLy string

	// THE TWO TERMINAL BRANCHES — `khong-tiep-nhan` and `chuyen-cap-tren` (migration 0011, user
	// decisions 24-25/09/2026). Both require LyDoKetThucNhanh, a reason the CITIZEN reads (same
	// standing as KetQuaXuLy: not a staff note, never on the event). `chuyen-cap-tren` also requires
	// CoQuanNhan, the receiving body, free text because transfers go sideways as often as up.
	// KetThucNhanhLuc is the instant of the branch act — deliberately NOT DongLuc, which would make a
	// refused petition count as a resolved one. All three are empty on every other status, and the
	// database refuses otherwise (CHECK `phieu_phan_anh_ket_thuc_nhanh_du_truong`).
	//
	// Written by store.KhongTiepNhan / store.ChuyenCapTren (POST …/rejection, …/referral) and read by
	// every SELECT of cotPhieu.
	LyDoKetThucNhanh string
	CoQuanNhan       string
	KetThucNhanhLuc  time.Time

	HienCongKhai bool
	SoLanMoLai   int

	// TaoLuc is when the ROW was created, and it is filled ONLY by the paginated read — the cursor
	// offers it as a sort column and nothing else in this service uses it.
	//
	// IT IS NOT VaoSoLuc AND THE TWO MUST NOT BE SHOWN AS ONE. `vao_so_luc` is a business fact (when
	// the commune took the report into its register) and `tao_luc` is a row-lifecycle fact. On the
	// staff-booked channel they can differ, and the gap is not something any screen should explain.
	TaoLuc time.Time
}

// HanTiepNhanKhongApDung reports that there is no acknowledge commitment on this petition at
// all — not that one is missing.
func (p PhieuPhanAnh) HanTiepNhanKhongApDung() bool { return p.HanTiepNhan.IsZero() }

// ChuaChotHanXuLy reports that the resolve commitment has not been fixed yet, because nobody
// has classified the petition.
//
// A REPORT MUST NOT COUNT THESE ROWS IN AN ON-TIME RATIO, and must show how many it left out:
// a commune that classifies slowly would otherwise have a BETTER on-time figure than one that
// classifies promptly. That is open question #26, and it is the customer's — this predicate
// exists so the exclusion is expressible, not so somebody can pick a classification deadline.
func (p PhieuPhanAnh) ChuaChotHanXuLy() bool { return p.HanXuLyXong.IsZero() }

// QuaHan DERIVES whether the resolve commitment was missed. It is never stored (rule 10,
// invariant 3): a stored flag is wrong the moment a job is late or a holiday is entered, and
// the stale copy is the one that reaches the report.
//
// A PETITION FINISHED LATE STAYS LATE. Once XuLyXongLuc is set the answer stops depending on
// `now` and compares the two recorded instants instead — otherwise a commune's late work would
// quietly stop being late as soon as it was done, and last quarter's figures would change every
// time somebody opened the screen.
//
// NO RESOLVE DEADLINE MEANS NOT OVERDUE, and that is a statement about the COMMITMENT, not about
// the work: an unclassified petition has had no resolve date promised to anyone, so there is
// nothing to have missed. The clock that IS running on it in the meantime is the acknowledge
// clock — see QuaHanTiepNhan.
func (p PhieuPhanAnh) QuaHan(now time.Time) bool {
	if p.ChuaChotHanXuLy() {
		return false
	}
	if !p.XuLyXongLuc.IsZero() {
		return p.XuLyXongLuc.After(p.HanXuLyXong)
	}
	return now.After(p.HanXuLyXong)
}

// QuaHanTiepNhan DERIVES whether the acknowledge commitment was missed — whether a human read
// the petition in time.
//
// THE CLOCK STOPS AT THE FIRST HUMAN ACT, not at the creation of the row (ADR 0027, decision D
// #3). Measuring from the send to the insert would be measuring the software against itself:
// that interval is always near zero, so every commune would score perfectly on a figure that
// says nothing about whether anybody read anything.
func (p PhieuPhanAnh) QuaHanTiepNhan(now time.Time) bool {
	if p.HanTiepNhanKhongApDung() {
		return false
	}
	if !p.PhanLoaiLuc.IsZero() {
		return p.PhanLoaiLuc.After(p.HanTiepNhan)
	}
	return now.After(p.HanTiepNhan)
}

// TranPhanLoaiKhongApDung reports that this petition has no classification ceiling at all — not
// that one is missing. Same shape and same meaning as HanTiepNhanKhongApDung, and for the same
// reason: on the staff-booked channel the petition arrives already classified.
func (p PhieuPhanAnh) TranPhanLoaiKhongApDung() bool { return p.HanPhanLoai.IsZero() }

// QuaHanPhanLoai DERIVES whether the classification ceiling was missed — whether the petition sat
// unclassified longer than ADR 0035 §C allows. It is never stored (rule 10, invariant 3).
//
// THE CEILING IS WHAT PUTS AN UNCLASSIFIED PETITION INTO THE DENOMINATOR, which is the whole point
// of question #26 and of the decision that closed it: without it, a commune that classifies slowly
// has a BETTER on-time figure than one that classifies promptly, because its slow petitions are not
// counted at all. A commune must be hurt where it is slow.
//
// THE CLOCK STOPS AT PhanLoaiLuc, the same instant that stops the acknowledge clock — settling the
// field IS the classification. A petition classified late STAYS late, for the reason QuaHan gives:
// otherwise last quarter's figures would change every time somebody opened the screen.
func (p PhieuPhanAnh) QuaHanPhanLoai(now time.Time) bool {
	if p.TranPhanLoaiKhongApDung() {
		return false
	}
	if !p.PhanLoaiLuc.IsZero() {
		return p.PhanLoaiLuc.After(p.HanPhanLoai)
	}
	return now.After(p.HanPhanLoai)
}

// --- the deadline rules ----------------------------------------------------------------------

// HanSomHon returns the EARLIER of a commitment already promised and one newly computed.
//
// ADR 0027, decision C: changing the field at classification may only SHORTEN the deadline,
// never extend it. The reason the customer gave, in their own terms: the deadline is a thing
// already SAID to a citizen, with a lookup code in their hand so they can read it back.
// Extending it is quietly withdrawing a promise, and the first person to notice is always the
// one who was told the old number.
//
// IT APPLIES FROM THE SECOND SETTLING ONWARD. The FIRST time an officer settles the field is the
// act that FIXES the deadline, not a change to one — there is nothing to compare against, and
// `daHua` is zero. That distinction is what ADR 0028 decision E bought: applying `min` against a
// zero value would silently re-create the 56-hour ceiling the ADR removed, and the code would
// look entirely correct.
func HanSomHon(daHua, moi time.Time) time.Time {
	if daHua.IsZero() {
		return moi
	}
	if moi.IsZero() {
		return daHua
	}
	if moi.Before(daHua) {
		return moi
	}
	return daHua
}

// SoNgayNhoLai bounds how far back an officer may place the origin of the clock on a
// staff-booked petition.
//
// @sla-ok: ADR 0028 quyết định F #3 — bảy NGÀY LỊCH, không phải giờ làm việc. Đây không phải một
// hạn xử lý: nó là khoảng thời gian người dân còn nhớ được chính xác việc mình đã phản ánh khi
// nào ("dân gọi điện tuần trước, trưởng thôn ghi sổ tay rồi mới nhập"). Hạn xử lý vẫn do
// identity.AdvanceWorkingHours tính bằng giờ làm việc và không đi qua hằng số này.
//
// ĐÃ ĐO: dấu này KHÔNG phải thứ cho `AddDate(0, 0, -SoNgayNhoLai)` ở KiemGocDemHan đi qua
// `citizen_commitment_guard` hôm nay — gỡ cả hai dấu trong tệp thì rào vẫn im, vì cửa sổ hai
// dòng quanh dòng ấy không có chữ nào thuộc ngữ cảnh hạn. Giữ dấu vì nó ghi VÌ SAO ngày lịch là
// đúng ở đây, nhưng đừng đọc nó như một phép miễn trừ đang có hiệu lực: một dấu được tin là
// đang canh trong khi nó đã chết là hạng lỗi đắt nhất trong kho này.
const SoNgayNhoLai = 7

// ErrGocDemHanNgoaiKhoang is returned for an origin outside the permitted window.
var ErrGocDemHanNgoaiKhoang = errors.New("phieu_phan_anh: mốc khởi động đồng hồ nằm ngoài khoảng cho phép")

// KiemGocDemHan refuses an origin the officer typed that is in the future or too far in the
// past.
//
// REFUSED, NEVER CLAMPED TO THE BOUNDARY — the same discipline as ADR 0007, decision 9. Clamping
// is software silently changing what a person just typed, and the person who typed it then has
// no idea which mark the deadline was counted from.
//
// WHY IT IS BOUNDED AT ALL: an arbitrary past mark is the ability to manufacture an
// already-overdue petition for somebody else, or to hide a late one by moving its origin back.
// Both falsify a figure through a form field, with no software fault involved.
func KiemGocDemHan(goc, vaoSo time.Time) error {
	if goc.IsZero() {
		return fmt.Errorf("%w: chưa có mốc", ErrGocDemHanNgoaiKhoang)
	}
	if goc.After(vaoSo) {
		return fmt.Errorf("%w: muộn hơn lúc vào sổ", ErrGocDemHanNgoaiKhoang)
	}
	// @sla-ok: xem SoNgayNhoLai ngay trên — bảy NGÀY LỊCH của trí nhớ người dân, không phải hạn.
	if goc.Before(vaoSo.AddDate(0, 0, -SoNgayNhoLai)) {
		return fmt.Errorf("%w: sớm hơn %d ngày trước lúc vào sổ", ErrGocDemHanNgoaiKhoang, SoNgayNhoLai)
	}
	return nil
}

// --- the lookup code ---------------------------------------------------------------------------

// chuCaiMaTraCuu is the alphabet a lookup code is drawn from.
//
// THIRTY CHARACTERS, AND THE SIX THAT ARE MISSING ARE THE POINT: `0` `O`, `1` `I` `L`, and `U`
// are gone. This code is printed on a slip, read down a telephone and typed back in by a
// citizen who may be elderly — a code containing both `0` and `O` produces a lookup failure that
// the person reads as "the commune lost my report".
//
// `U` is dropped as well, following Crockford's base32: it is the one letter whose absence stops
// a random string from spelling an obscenity on a government document.
const chuCaiMaTraCuu = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// soKyTuMaTraCuu is how many characters of that alphabet a code carries.
//
// TWELVE gives 30^12 ≈ 5.3e17 possibilities, about 59 bits. The requirement is not "hard to
// brute-force offline" — it is that a code CANNOT BE ENUMERATED (rule 4, invariant 4; rule 10,
// forbidden #5). A sequential or short code on a lookup path reads every other citizen's
// petition, one increment at a time, and the specification's display code `PA-2026-0021` is
// exactly that shape, which is why it is not this value.
const soKyTuMaTraCuu = 12

// TienToMaTraCuu keeps the code recognisable as a petition when somebody reads it aloud or finds
// it written on a slip with no context.
const TienToMaTraCuu = "PA"

// SinhMaTraCuu mints the code the citizen is handed the moment the petition is received (rule
// 10, invariant 1).
//
// crypto/rand, NOT math/rand: a code drawn from a seeded generator is a code whose neighbours can
// be computed, which is the enumeration this function exists to prevent — and the failure would
// be invisible, since the strings look equally random.
//
// REJECTION SAMPLING, and the discarded draws matter. The alphabet has 30 characters and a byte
// has 256 values; `b % 30` would make the first 16 characters of the alphabet measurably more
// likely than the last 14. That bias is not a cosmetic flaw at scale: it shrinks the effective
// search space of every code the system will ever issue.
//
// AN ERROR IS NEVER SWALLOWED AND NEVER PAPERED OVER WITH A FALLBACK. If the system cannot draw
// randomness it must refuse the intake: a predictable code handed to a citizen cannot be
// withdrawn, because the code is never reissued (rule 7, invariant 3) and the slip is already in
// their hand.
func SinhMaTraCuu() (string, error) {
	var b strings.Builder
	b.Grow(len(TienToMaTraCuu) + 1 + soKyTuMaTraCuu + 2)
	b.WriteString(TienToMaTraCuu)

	const nguong = 256 - (256 % len(chuCaiMaTraCuu)) // 240: draws at or above this are discarded

	dem := make([]byte, soKyTuMaTraCuu)
	for i := 0; i < soKyTuMaTraCuu; {
		if _, err := rand.Read(dem); err != nil {
			return "", fmt.Errorf("phieu_phan_anh: sinh mã tra cứu: %w", err)
		}
		for _, v := range dem {
			if i >= soKyTuMaTraCuu {
				break
			}
			if int(v) >= nguong {
				continue
			}
			// Grouped in fours — `PA-4K7M-92XR-BTVD`. A person reading a twelve-character run
			// down a telephone loses their place; four is the group size a telephone number uses
			// for the same reason.
			if i > 0 && i%4 == 0 {
				b.WriteByte('-')
			} else if i == 0 {
				b.WriteByte('-')
			}
			b.WriteByte(chuCaiMaTraCuu[int(v)%len(chuCaiMaTraCuu)])
			i++
		}
	}
	return b.String(), nil
}
