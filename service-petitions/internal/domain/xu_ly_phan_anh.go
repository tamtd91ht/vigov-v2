package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// The STAFF half of the petition lifecycle — classification, assignment, moving along the main
// flow, and closing with a result the citizen can read.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). The
// nine statuses, the transition map and the two deadline rules live in phieu_phan_anh.go; what is
// here is the vocabulary and the shape checks the four staff acts need, expressed as pure
// functions so they can be tested without a database, a network or a commune's configuration.

// --- the classification ceiling (ADR 0035 §C, open question #26, decided 2026-09-22) ----------

// GioTranPhanLoai is how long a petition may sit unclassified, IN WORKING HOURS.
//
// ADR 0035 §C decided "MỘT NGÀY LÀM VIỆC kể từ khi tiếp nhận, đếm bằng GIỜ LÀM VIỆC qua
// identity.AdvanceWorkingHours (ADR 0007) — cùng đơn vị với mọi hạn khác, không phải ngày lịch".
// This constant is the translation of "one working day" into the unit that RPC speaks, and it is
// the one number in this file a reader should challenge.
//
// ⚠ EIGHT IS AN ASSUMPTION, AND IT IS STATED RATHER THAN HIDDEN. Neither the ADR nor the customer
// wrote a number of hours; what exists is `.claude/skills/petition-lifecycle` ("a working day is
// 8 hours") and docs/ui-ux/14-cau-hinh.md §8, whose `Tiếp nhận` column uses `8 giờ` for the
// one-day commitments. A commune whose `lich_lam_viec` adds up to 7.5 hours a day therefore gets
// a ceiling slightly longer than its own working day — which is the SAFE direction (it never
// shortens a commitment), and it is a finding for the customer, not a decision to bury.
//
// WHY IT IS NOT READ FROM THE `sla` TABLE: that table holds `gio_tiep_nhan` and `gio_xu_ly_xong`
// and has no third column. Adding one is service-identity's migration and ADR 0029's territory,
// not this service's, and inventing a per-commune source here would put one configuration
// screen's numbers in two places.
//
// WHY IT IS FIXED AND NOT PER COMMUNE: ADR 0035 §C set it as a floor under the indicator, i.e. a
// number that must mean the same thing when 200+ communes are compared. A commune able to raise
// its own ceiling is a commune able to make its own classification figure look better.
const GioTranPhanLoai uint32 = 8

// --- the restricted field -----------------------------------------------------------------------

// LinhVucHanChe is the one field code whose petitions are not visible to every reader.
//
// `can-bo` is "Thái độ / tác phong cán bộ" — a report ABOUT a member of staff (docs/ui-ux/09 §5 and
// §14.5). It needs its own key because the ordinary readers of this register are the colleagues of
// the person being reported on, and a channel a citizen does not trust stops carrying the reports a
// commune actually needs.
//
// # IT LIVES IN domain AND NOT IN internal/http, AND THAT MOVED FOR A REASON
//
// It began as a constant beside the read handler, which was right while one route consulted it.
// The LIST route made that wrong: excluding these petitions has to happen in the WHERE clause —
// filtering a page after it came back returns short pages and a cursor that has already advanced
// past the rows it dropped — so `internal/store` needs the value too, and a store may not import a
// handler package. One constant, in the layer both can see.
//
// THE PERMISSION THAT OPENS IT STAYS IN internal/http (`QuyenHanChe`), because an authz.Perm is a
// fact about the HTTP surface and domain imports nothing but the standard library.
//
// ADR 0035 §D adds the half this constant cannot express: this code NEVER LEAVES THE COMMUNE, in any
// figure at any level. Letting it into a district or province count would open a door that is still
// locked inside the commune itself.
const LinhVucHanChe = "can-bo"

// --- the nine labels --------------------------------------------------------------------------

// nhanTrangThai is the citizen-readable wording of each of the nine statuses.
//
// # THIS IS A COPY, IT IS DELIBERATE, AND phieu_phan_anh.go SAYS THE OPPOSITE FOR A REASON THAT HAS EXPIRED
//
// The note on TrangThai states the labels are "deliberately NOT copied here, because a second copy
// of a display string is a second copy that drifts". That was right while nothing in this service
// needed a label. It stopped being right the moment this service became the PUBLISHER of
// `petitions.status_changed.v1`: `CitizenMessage.status_label` carries the label on the wire, and
// proto/vigov/petitions/v1/events.proto explains at length why the wording must TRAVEL rather than
// be looked up by the consumer — a status → sentence table inside `comms` would be a copy of a list
// `petitions` owns sitting on the far side of a service boundary.
//
// So there is exactly ONE copy in this service, it is here, and the owning file is
// kb/00-foundation/ubiquitous-language.md §Chín trạng thái. Two things follow, and both are rules
// rather than advice:
//
//   - a commune may NOT rename these (ADR 0027, addendum of 2026-09-20). That is what makes the
//     label safe to put on a queue at all: it is the same word in all 200+ communes, so it carries
//     no local meaning and no personal data.
//   - a THIRD copy — in a handler, in a template, in web-admin — is the drift this comment is
//     trying to prevent. Ask this function.
var nhanTrangThai = map[TrangThai]string{
	DaTiepNhan:    "Đã tiếp nhận",
	DangPhanLoai:  "Đang phân loại",
	DaChuyenXuLy:  "Đã chuyển xử lý",
	DangXuLy:      "Đang xử lý",
	DaXuLy:        "Đã xử lý",
	ChoDanXacNhan: "Chờ dân xác nhận",
	DaDong:        "Đã đóng",
	KhongTiepNhan: "Không tiếp nhận",
	ChuyenCapTren: "Chuyển cấp trên",
}

// NhanTrangThai returns the label, or "" for a string that is not one of the nine.
//
// EMPTY RATHER THAN THE CODE ITSELF. A caller that falls back to the raw code would put
// `cho-dan-xac-nhan` in front of a citizen as though it were Vietnamese; the receiving side
// refuses an empty `status_label` outright (comms `domain.ThamSoThongBao.KiemTra`), which is a
// loud failure instead of a message nobody can read.
func NhanTrangThai(t TrangThai) string { return nhanTrangThai[t] }

// --- what the citizen is told -------------------------------------------------------------------

// loiNhanChoDan is THE table of which transitions owe the citizen a message, and what the message
// says. Each entry composes the PROCEDURAL sentence that travels as `CitizenMessage.next_step`.
//
// # SIX ENTRIES, DECIDED BY THE OWNER ON 2026-09-24 — AND THE THREE ABSENCES ARE PART OF THE DECISION
//
//	da-tiep-nhan       intake: the acknowledge deadline + the 113/114/115 reminder
//	da-chuyen-xu-ly    handed to a department: the resolve deadline
//	cho-dan-xac-nhan   the work is done: the citizen is invited to confirm and rate
//	da-dong            closed: where to read the result
//	khong-tiep-nhan    refused: where to read the reason (POST …/rejection)
//	chuyen-cap-tren    referred: where to read the reason and the receiving body (POST …/referral)
//
// `dang-phan-loai`, `dang-xu-ly` and `da-xu-ly` are NOT here. The first is internal; the second used to
// be here and was removed on purpose — the department-and-deadline message belongs to the moment the
// petition is HANDED OVER (`da-chuyen-xu-ly`), and saying it twice is how a channel becomes noise
// people mute; the third is followed at once by `cho-dan-xac-nhan`, which carries the message.
//
// An absent entry makes the event carry no `citizen_message`, and `comms` then writes no ledger row
// and sends nothing; that is a fact, not a failure, and its consumer says so at the point it checks.
//
// # WHAT THE SENTENCES MAY DRAW ON — AND WHY THE ARGUMENT IS THE WHOLE PETITION ANYWAY
//
// The function receives the petition because the two deadlines live on it. It may read EXACTLY two
// fields, `HanTiepNhan` and `HanXuLyXong`, and nothing else. Both are software-computed instants the
// commune has ALREADY COMMITTED TO by the time the message is composed — `han_tiep_nhan` at intake,
// `han_xu_ly_xong` at classification, which ADR 0028 decision E places strictly before any department
// is named — so naming them invents nothing (rule 10, forbidden #3 is about inventing a promise, not
// repeating one). Neither is personal data, and a deadline is on the list the owner allowed to reach
// ZNS (rule 3, invariant 6: code, status, short result, deadline).
//
// NEVER, IN ANY ENTRY: the reporter, the text of the petition, the address, a photograph, the officer's
// name or number, internal notes, routing history — and NOT THE RESULT TEXT `ket_qua_xu_ly` either.
// That text is free text a member of staff wrote about ONE case; it will eventually name the reporter
// or quote them, and the event contract (proto/vigov/petitions/v1/events.proto, CitizenMessage) forbids
// forwarding it. The citizen reads it with their lookup code behind an authenticated read.
//
// # THE DEPARTMENT IS NOT NAMED, AND THAT IS A CONTRACT GAP RATHER THAN A CHOICE
//
// This service stores `bo_phan_id` only; the NAME lives in service-identity. Asking identity for it
// inside the write transaction would hold the row lock for a network round trip, and the event
// contract has no field to carry a unit id for `comms` to resolve. So the sentence says "bộ phận
// chuyên môn" and the gap is reported, not papered over.
var loiNhanChoDan = map[TrangThai]func(p PhieuPhanAnh) string{
	DaTiepNhan: func(p PhieuPhanAnh) string {
		return "Xã đã tiếp nhận phản ánh. " +
			hanHoacKhong(p.HanTiepNhan, "Phản ánh sẽ được cán bộ xem trước %s. ",
				"Phản ánh sẽ được cán bộ xem trong thời hạn tiếp nhận của xã. ") +
			"Giữ mã tra cứu để theo dõi. " +
			"Việc khẩn cấp, xin gọi ngay 113 (công an), 114 (cứu nạn, cứu hộ, chữa cháy) " +
			"hoặc 115 (cấp cứu y tế)."
	},
	DaChuyenXuLy: func(p PhieuPhanAnh) string {
		return "Phản ánh đã được chuyển bộ phận chuyên môn xử lý. " +
			hanHoacKhong(p.HanXuLyXong, "Hạn xử lý xong: trước %s. ", "") +
			"Dùng mã tra cứu để xem tiến độ."
	},
	ChoDanXacNhan: func(PhieuPhanAnh) string {
		return "Xã đã xử lý xong phản ánh. " +
			"Mời ông/bà mở ứng dụng, dùng mã tra cứu để xem kết quả, xác nhận và đánh giá."
	},
	DaDong: func(PhieuPhanAnh) string {
		return "Phản ánh đã được đóng. " +
			"Dùng mã tra cứu để xem kết quả xử lý xã đã ghi."
	},
	// THE TWO BRANCHES POINT AT THE LOOKUP CODE AND CARRY NEITHER THE REASON NOR THE RECEIVING BODY.
	// Both are now stored (migration 0011, `ly_do_ket_thuc_nhanh` and `co_quan_nhan`) and both are
	// free text a member of staff typed about ONE case — the same standing as `ket_qua_xu_ly`, which
	// the event contract forbids forwarding (proto/vigov/petitions/v1/events.proto, CitizenMessage).
	// The citizen reads them behind the authenticated GET /api/v1/my-citizen-reports/{maTraCuu}.
	//
	// ⚠ STILL OPEN: the owner's decision also asked for WHERE TO GO NEXT. No column holds it; the
	// reason text is where staff write it today. A structured field is a decision, not a spelling.
	KhongTiepNhan: func(PhieuPhanAnh) string {
		return "Xã không tiếp nhận phản ánh này. " +
			"Dùng mã tra cứu để xem lý do và nơi ông/bà có thể liên hệ tiếp."
	},
	ChuyenCapTren: func(PhieuPhanAnh) string {
		return "Phản ánh đã được chuyển lên cơ quan cấp trên có thẩm quyền. " +
			"Dùng mã tra cứu để xem cơ quan tiếp nhận và việc ông/bà cần làm tiếp."
	},
}

// loiNhanHanChe is what a citizen is told about a petition in the RESTRICTED field `can-bo`, at any of
// the transitions above: the lookup code and the status label travel beside it, and nothing else.
//
// NO DEADLINE, NO DEPARTMENT, NO RESULT. The owner's decision of 2026-09-24 limits these to code +
// status. A report ABOUT a member of staff passes through a third party (ZNS) and a notification
// ledger read by the commune's own staff — the colleagues of the person reported on — and "which
// department took it" is exactly the routing detail that tells them who is handling the complaint.
const loiNhanHanChe = "Dùng mã tra cứu để xem tiến độ trong ứng dụng."

// muiGioChoDan is the zone a deadline is written in for a citizen.
//
// A FIXED +07:00 AND NOT time.LoadLocation("Asia/Ho_Chi_Minh"). The two agree for every instant since
// 1975 (Vietnam has no daylight saving), and the fixed zone cannot fail: LoadLocation reads a zone
// database the container image may not carry, and a failure there would either refuse a status change
// because a SENTENCE could not be formatted or fall back to UTC — seven hours off, in a promise told
// to a citizen. service-identity names the zone (`domain.MuiGioHanhChinh`) because it combines
// wall-clock SESSION times with dates; this only renders an instant, which needs the offset alone.
var muiGioChoDan = time.FixedZone("ICT", 7*3600)

// hanHoacKhong renders a deadline into `mau`, or returns `khongCo` when the deadline is the zero time.
//
// THE ZERO CASE IS UNREACHABLE on today's paths (the citizen channel always stores `han_tiep_nhan`,
// and `han_xu_ly_xong` is fixed before a department can be named) and is handled anyway: a zero here
// rendered as "trước 07:00 ngày 01/01/0001" would be a promise in the year 1, and an EMPTY sentence
// would drop the whole message at the consumer — a citizen silently not told.
func hanHoacKhong(han time.Time, mau, khongCo string) string {
	if han.IsZero() {
		return khongCo
	}
	return fmt.Sprintf(mau, han.In(muiGioChoDan).Format("15:04 ngày 02/01/2006"))
}

// ViecTiepTheo returns the sentence owed to the citizen when petition `p` enters status `moi`, or ""
// when that transition owes them nothing. `p` is the petition AFTER the change.
//
// THE RESTRICTED FIELD IS DECIDED HERE, NOT BY EACH CALLER, so no publisher can forget it: a `can-bo`
// petition that owes a message gets loiNhanHanChe and nothing more.
func ViecTiepTheo(p PhieuPhanAnh, moi TrangThai) string {
	soan, co := loiNhanChoDan[moi]
	if !co {
		return ""
	}
	if p.LinhVuc == LinhVucHanChe {
		return loiNhanHanChe
	}
	return soan(p)
}

// BaoChoDan reports whether this transition owes the citizen a message (rule 10, invariant 5).
//
// IT IS DERIVED FROM loiNhanChoDan AND NOT FROM A SECOND LIST. Two lists would be two answers, and
// the one that drifts is the one that quietly stops telling somebody — which from inside the system
// is indistinguishable from correct behaviour, because nothing errors when a message is not owed.
func BaoChoDan(t TrangThai) bool {
	_, co := loiNhanChoDan[t]
	return co
}

// --- moving along the main flow ------------------------------------------------------------------

// tienTrinhChinh is the part of the lifecycle that moves with NO extra business data attached.
//
// THREE ENTRIES, AND THE FOUR TRANSITIONS THAT ARE MISSING ARE MISSING ON PURPOSE. Each of those
// four carries a decision and a permission of its own, so folding them in here would let one route
// perform an act the commune granted a different right for:
//
//	da-tiep-nhan   -> dang-phan-loai   settles `linh_vuc` AND FIXES `han_xu_ly_xong` — feedback.classify
//	dang-phan-loai -> da-chuyen-xu-ly  names the department answerable for it       — feedback.assign
//	cho-dan-xac-nhan -> da-dong        records a result the citizen can read        — feedback.resolve
//	cho-dan-xac-nhan / da-dong -> dang-xu-ly   REOPENING, governed by three per-commune flags of
//	                                   ADR 0008 that no table in this repository holds yet
//
// The map in phieu_phan_anh.go is still the authority on what the lifecycle ALLOWS; this one is the
// narrower question "which move does the plain advance route perform", and ChuyenSangDuoc is
// consulted as well at every call site so the two can never disagree in the permissive direction.
var tienTrinhChinh = map[TrangThai]TrangThai{
	DaChuyenXuLy: DangXuLy,
	DangXuLy:     DaXuLy,
	DaXuLy:       ChoDanXacNhan,
}

// TienTrinhChinh returns the next status of the plain forward advance, and false when there is none.
func TienTrinhChinh(t TrangThai) (TrangThai, bool) {
	m, co := tienTrinhChinh[t]
	return m, co
}

// ErrPhanCongSaiLuc is returned when the petition is not at a point where naming a department means
// anything.
var ErrPhanCongSaiLuc = errors.New(
	"phan_anh: chưa phân công được — phiếu phải được phân loại trước, và phiếu đã đóng thì không phân công nữa")

// SauKhiPhanCong answers what naming a department does to the status, and whether it is allowed at
// all.
//
// # TWO CASES, AND THE SECOND IS THE ONE THE SPECIFICATION IS EXPLICIT ABOUT
//
//	dang-phan-loai        -> da-chuyen-xu-ly   the FIRST routing. It is the only edge into that
//	                                           status in the whole lifecycle map (ADR 0027), so the
//	                                           act that names a department IS that transition.
//	already being worked  -> unchanged         docs/ui-ux/09 §8.5 titles the block "Chuyển xử lý,
//	                                           KHÔNG đổi trạng thái". Moving a petition from one
//	                                           department to another mid-processing must not drag it
//	                                           backwards, which would re-open a step the register
//	                                           already recorded as passed.
//
// ⚠ RECONCILING THOSE TWO IS THIS SESSION'S READING, NOT A LINE OF THE SPECIFICATION, and it is
// reported as such. §8.5 says routing never changes the status; ADR 0027's map says `da-chuyen-xu-ly`
// is reachable only from `dang-phan-loai` and gives no other act that reaches it. Taken literally and
// separately, the two together leave `da-chuyen-xu-ly` unreachable — a status with no way in, which
// is the exact failure ADR 0027 §Quyết định B describes for a lifecycle nobody can complete.
//
// REFUSED FROM `da-tiep-nhan`, because assigning an unclassified petition would put a department in
// charge of work whose RESOLVE DEADLINE HAS NOT BEEN FIXED YET (ADR 0028 decision E) — the officer
// would be handed a commitment nobody has made. Classify first.
//
// REFUSED FROM `da-dong` AND FROM THE TWO TERMINAL STATUSES, because there is no commitment left to
// be answerable for, and editing a closed petition is editing an archival record (rule 7,
// forbidden #5).
func SauKhiPhanCong(t TrangThai) (TrangThai, error) {
	switch t {
	case DangPhanLoai:
		return DaChuyenXuLy, nil
	case DaChuyenXuLy, DangXuLy, DaXuLy, ChoDanXacNhan:
		return t, nil
	default:
		// `da-tiep-nhan`, `da-dong`, `khong-tiep-nhan`, `chuyen-cap-tren`, and any code that is not
		// one of the nine. FAIL CLOSED: an unknown status is refused, never let through as "probably
		// fine".
		return "", ErrPhanCongSaiLuc
	}
}

// ErrDongSaiLuc is returned when the petition is not at the point the lifecycle closes from.
var ErrDongSaiLuc = errors.New(
	"phan_anh: chưa đóng được — chỉ đóng phiếu đang ở bước chờ dân xác nhận")

// DongDuoc reports whether the petition may be closed now.
//
// `cho-dan-xac-nhan` AND NOTHING ELSE, which is exactly what the lifecycle map allows into `da-dong`
// (ADR 0027). Widening it — "close from anywhere, the officer knows best" — would let a petition be
// closed before the citizen was ever asked to confirm, and adding or removing an edge of that map is
// rule 10, stop condition #2.
func DongDuoc(t TrangThai) error {
	if t != ChoDanXacNhan {
		return ErrDongSaiLuc
	}
	return nil
}

// ErrKetThucNhanhSaiLuc is returned when a petition is not at the one point the two terminal branches
// leave from.
var ErrKetThucNhanhSaiLuc = errors.New(
	"phan_anh: chỉ từ chối tiếp nhận hoặc chuyển cấp trên được phiếu đang ở bước phân loại")

// KetThucNhanhDuoc reports whether petition status `t` may take the terminal branch `nhanh`
// (`khong-tiep-nhan` or `chuyen-cap-tren`).
//
// `dang-phan-loai` AND NOTHING ELSE, which is what chuyenDuocSang allows (docs/ui-ux/09 §6): that is
// the step where a human first reads the report and decides whether the commune takes it at all, and
// both branches are OUTCOMES OF THAT DECISION (ADR 0030 — which is why both routes are guarded by
// `feedback.classify`). A referral out of `dang-xu-ly` — work already begun, then handed up — is a
// different edge of the lifecycle that the owner has NOT decided (rule 10, stop condition #2), so it
// is refused here rather than let through by a wider check.
//
// BOTH CONDITIONS ARE CHECKED, and the second is not redundant: `t == DangPhanLoai` is this act's own
// narrowing, ChuyenSangDuoc is the lifecycle map. Checking only the first would let a target that is
// not a branch at all (a caller passing `da-dong`) through; checking only the second would widen the
// day somebody adds a branch edge from another status to the map.
func KetThucNhanhDuoc(t, nhanh TrangThai) error {
	if nhanh != KhongTiepNhan && nhanh != ChuyenCapTren {
		return ErrKetThucNhanhSaiLuc
	}
	if t != DangPhanLoai || !t.ChuyenSangDuoc(nhanh) {
		return ErrKetThucNhanhSaiLuc
	}
	return nil
}

// --- shape checks for the four staff acts ---------------------------------------------------------

const (
	// LinhVucToiDa bounds a tier-1 field code. Twelve codes exist today and the longest is
	// `an-toan-thuc-pham` (17); the bound is for a value that is not a code at all.
	LinhVucToiDa = 64

	// BoPhanToiDa and CanBoToiDa bound identity's `bo_phan.id` (a ULID) and a staff BUSINESS CODE
	// (`CB-00123`). Neither is validated against identity here — see KiemPhanCong.
	BoPhanToiDa = 64
	CanBoToiDa  = 64

	// KetQuaToiDa bounds the result written when a petition is closed. Long enough for a paragraph a
	// citizen reads, short enough that no closing carries a document.
	KetQuaToiDa = 2000

	// KetQuaToiThieu is the shortest string accepted as a RESULT.
	//
	// ⚠ THIS NUMBER IS THIS SESSION'S, NOT THE CUSTOMER'S, and it is the one place this file guesses.
	// Rule 10, invariant 6 says closing records "a result the citizen can read" and forbids closing
	// silently; it does not say how long. Ten runes refuses "ok", "xong", "đã xử lý" — the strings
	// that make a closing look recorded while telling the citizen nothing — and accepts any real
	// sentence. Raising it later is cheap; lowering it after communes have closed petitions is not.
	KetQuaToiThieu = 10

	// LyDoToiThieu and LyDoToiDa bound the reason a petition was refused or referred — the sentence
	// the citizen reads INSTEAD of a result (rule 10, invariant 6 in spirit: never end silently).
	//
	// THE SAME TWO NUMBERS AS THE CLOSING RESULT, BY DEFINITION RATHER THAN BY COINCIDENCE. Same kind
	// of text, same reader, same screen. The maximum is ALSO the database's: migration 0011's CHECK
	// `phieu_phan_anh_ly_do_ket_thuc_nhanh_toi_da` refuses more than 2000 characters, and validating
	// the same number here in RUNES is what turns an oversized reason into a 400 instead of a 500.
	// The minimum inherits KetQuaToiThieu's ⚠: it is this session's number, not the customer's.
	LyDoToiThieu = KetQuaToiThieu
	LyDoToiDa    = KetQuaToiDa

	// CoQuanNhanToiDa bounds the receiving body of a referral — the database's number (migration
	// 0011, `phieu_phan_anh_co_quan_nhan_toi_da`), repeated here for the same 400-not-500 reason.
	//
	// NO MINIMUM BEYOND NON-BLANK. It is the NAME of an authority, and real ones are short ("Công an
	// xã", "Điện lực"): a ten-rune floor would refuse a correct answer. That is an assumption of this
	// session and is reported.
	CoQuanNhanToiDa = 200
)

var (
	// ErrThieuLinhVuc — classification with no field code. The act EXISTS to settle it.
	ErrThieuLinhVuc = errors.New("phan_anh: `field` trống — phân loại phải chốt lĩnh vực")

	// ErrLinhVucSaiDang — not a kebab-case code.
	//
	// THIS IS A SHAPE CHECK AND NOT AN EXISTENCE CHECK, and the difference is a known hole rather
	// than an oversight — see KiemLinhVuc.
	ErrLinhVucSaiDang = errors.New("phan_anh: `field` không phải một mã lĩnh vực hợp lệ")

	// ErrThieuBoPhan — assignment with no department. "— Để bộ phận tự phân công —" is a real
	// answer for the OFFICER (docs/ui-ux/09 §8.5), never for the department: a petition assigned to
	// nobody is a petition nobody is answerable for, with a deadline already running.
	ErrThieuBoPhan  = errors.New("phan_anh: `unit` trống — phải có bộ phận nhận xử lý")
	ErrBoPhanQuaDai = errors.New("phan_anh: `unit` quá dài")
	ErrCanBoQuaDai  = errors.New("phan_anh: `assignee` quá dài")

	// ErrThieuKetQua / ErrKetQuaQuaNgan — closing with nothing the citizen can read. Rule 10,
	// invariant 6: "Đã xử lý" alone is not a result, say what was actually done.
	ErrThieuKetQua   = errors.New("phan_anh: `result` trống — không đóng phiếu mà không có kết quả cho người dân đọc")
	ErrKetQuaQuaNgan = errors.New("phan_anh: `result` quá ngắn — người dân phải đọc được xã đã làm gì")
	ErrKetQuaQuaDai  = errors.New("phan_anh: `result` quá dài")

	// ErrThieuLyDo / ErrLyDoQuaNgan / ErrLyDoQuaDai — refusing or referring with nothing the citizen
	// can read. A petition that leaves the commune with no reason is ended SILENTLY, which is exactly
	// what the database's CHECK (migration 0011) and rule 10, invariant 6 forbid.
	ErrThieuLyDo   = errors.New("phan_anh: `reason` trống — người dân phải đọc được vì sao")
	ErrLyDoQuaNgan = errors.New("phan_anh: `reason` quá ngắn — người dân phải đọc được vì sao")
	ErrLyDoQuaDai  = errors.New("phan_anh: `reason` quá dài")

	// ErrThieuCoQuanNhan / ErrCoQuanNhanQuaDai — a referral must name WHERE the petition went. A
	// citizen told "đã chuyển cấp trên" and not to whom has nobody to follow up with.
	ErrThieuCoQuanNhan  = errors.New("phan_anh: `receiving_body` trống — phải ghi cơ quan tiếp nhận")
	ErrCoQuanNhanQuaDai = errors.New("phan_anh: `receiving_body` quá dài")

	// ErrKhongConCamKet — the petition is in a terminal status, so there is no commitment left to
	// act on. Editing it would be editing an archival record (rule 7, forbidden #5).
	ErrKhongConCamKet = errors.New("phan_anh: phiếu đã kết thúc — không còn thao tác nào trên phiếu này")
)

// maKebab is the shape of every enum value in this system: Vietnamese without diacritics,
// kebab-case (ADR 0011). Written as a hand rolled scan rather than a regexp so this file keeps its
// standard-library-only rule with no cost at all.
func maKebab(s string) bool {
	if s == "" || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
		case ch == '-':
			if s[i-1] == '-' {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// KiemLinhVuc trims and SHAPE-CHECKS a tier-1 field code.
//
// # ⚠ IT DOES NOT CHECK THAT THE CODE EXISTS, AND THAT IS A KNOWN, MEASURED HOLE
//
// ADR 0026 requires the code to be checked ON WRITE against the tier-1 set owned by service
// `platform`, and ADR 0026 stop condition #2 says how `petitions` reads that set — a live gRPC call
// or a replica fed by events — needs its own ADR that nobody has written. Measured today:
// proto/vigov/platform/v1/platform.proto exposes ResolveHost, GetTenant, ListTenants and
// ResolveTenantSuccession, and nothing else. There is no RPC to ask.
//
// WHAT THE HOLE COSTS, said precisely rather than softened: a misspelt code reaches
// identity.ResolveDeadlines, which falls back to the DEFAULT `sla` row when no row matches the field
// (service-identity/internal/grpc/sla.go, `domain.DongTheoLinhVuc`). So a typo produces a
// PLAUSIBLE deadline from the wrong row, the petition is filed under a code no report groups by, and
// nothing anywhere errors.
//
// WHY THE TWELVE CODES ARE NOT WRITTEN HERE INSTEAD: that is ADR 0026 stop condition #1 — the code
// set belongs to `platform`, and a copy in this service is the second source the two-tier model
// exists to prevent. A copy would also be the thing a later session "kept in step" by hand.
func KiemLinhVuc(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuLinhVuc
	case utf8.RuneCountInString(s) > LinhVucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrLinhVucSaiDang, LinhVucToiDa)
	case !maKebab(s):
		// THE INPUT IS NOT ECHOED. A field code is not personal data, but this message travels to a
		// client and into logs, and every refusal in this service holds the same line (rule 3).
		return "", ErrLinhVucSaiDang
	}
	return s, nil
}

// KiemPhanCong trims and bounds the department and the optional officer.
//
// NEITHER IS VERIFIED AGAINST identity, and that is stated rather than implied: `bo_phan` and the
// staff directory live in service-identity, this service may not read its tables (rule 2, forbidden
// #2), and identity's contract has no "does this unit exist" RPC. So a department id that names
// nothing is storable today. It is visible the moment somebody opens the petition — the screen shows
// no unit — which is the failure mode to prefer over a fabricated check that looks like one.
//
// THE OFFICER IS OPTIONAL BECAUSE THE SCREEN SAYS SO: "— Để bộ phận phân công —" (docs/ui-ux/09 §8.5)
// is a real choice, and it means the department decides internally who takes it.
func KiemPhanCong(boPhan, canBo string) (string, string, error) {
	boPhan = strings.TrimSpace(boPhan)
	canBo = strings.TrimSpace(canBo)
	switch {
	case boPhan == "":
		return "", "", ErrThieuBoPhan
	case utf8.RuneCountInString(boPhan) > BoPhanToiDa:
		return "", "", fmt.Errorf("%w (tối đa %d ký tự)", ErrBoPhanQuaDai, BoPhanToiDa)
	case utf8.RuneCountInString(canBo) > CanBoToiDa:
		return "", "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCanBoQuaDai, CanBoToiDa)
	}
	return boPhan, canBo, nil
}

// KiemKetQua trims and bounds the closing result — the sentence rule 10, invariant 6 promises the
// citizen.
//
// TRIMMED BEFORE IT IS MEASURED, so a textarea holding nothing but newlines is refused as empty
// rather than stored as a closing with a blank result. That is the case this function exists for:
// a petition closed with no readable result is closed SILENTLY, which is what invariant 6 forbids,
// and the citizen who was told "đã đóng" and nothing else has no way to tell being helped from being
// dismissed.
func KiemKetQua(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuKetQua
	case utf8.RuneCountInString(s) < KetQuaToiThieu:
		return "", fmt.Errorf("%w (tối thiểu %d ký tự)", ErrKetQuaQuaNgan, KetQuaToiThieu)
	case utf8.RuneCountInString(s) > KetQuaToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrKetQuaQuaDai, KetQuaToiDa)
	}
	return s, nil
}

// KiemLyDoKetThucNhanh trims and bounds the reason of a refusal or a referral, in RUNES.
//
// Trimmed before it is measured for the reason KiemKetQua gives: a textarea of newlines is a petition
// ended silently. Counted in runes because Vietnamese diacritics are two or three bytes each, and the
// database's `char_length` counts characters — a byte count here would refuse a reason PostgreSQL
// accepts, and a smaller one would pass a reason it refuses with a 500.
func KiemLyDoKetThucNhanh(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch n := utf8.RuneCountInString(s); {
	case s == "":
		return "", ErrThieuLyDo
	case n < LyDoToiThieu:
		return "", fmt.Errorf("%w (tối thiểu %d ký tự)", ErrLyDoQuaNgan, LyDoToiThieu)
	case n > LyDoToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrLyDoQuaDai, LyDoToiDa)
	}
	return s, nil
}

// KiemCoQuanNhan trims and bounds the receiving body of a referral, in RUNES.
//
// FREE TEXT BY THE USER'S DECISION (migration 0011's header): since 7/2025 a transfer goes sideways
// as often as up, so there is no catalogue of "the level above" to validate against.
func KiemCoQuanNhan(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuCoQuanNhan
	case utf8.RuneCountInString(s) > CoQuanNhanToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCoQuanNhanQuaDai, CoQuanNhanToiDa)
	}
	return s, nil
}

// LaLoiXuLyPhanAnh reports whether this is a refusal of what the caller sent, as opposed to a
// failure of the system.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400 — the same discipline as LaLoiGuiPhanAnh, for the
// same measured reason: a default of "anything I do not recognise is the caller's fault" turns a
// database outage into a 400, and the officer then retypes the form forever while nobody is told the
// server is broken.
func LaLoiXuLyPhanAnh(err error) bool {
	for _, mot := range []error{
		ErrThieuLinhVuc, ErrLinhVucSaiDang,
		ErrThieuBoPhan, ErrBoPhanQuaDai, ErrCanBoQuaDai,
		ErrThieuKetQua, ErrKetQuaQuaNgan, ErrKetQuaQuaDai,
		ErrThieuLyDo, ErrLyDoQuaNgan, ErrLyDoQuaDai,
		ErrThieuCoQuanNhan, ErrCoQuanNhanQuaDai,
		// ErrPhanCongSaiLuc, ErrDongSaiLuc, ErrKetThucNhanhSaiLuc AND ErrKhongConCamKet ARE
		// DELIBERATELY NOT IN THIS LIST.
		// They are refusals about the STATE OF THE RECORD, not about what the caller typed, and the
		// handler answers them 409 — the same split service-documents makes. A 400 there would tell
		// an officer to fix a form that is perfectly correct.
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
