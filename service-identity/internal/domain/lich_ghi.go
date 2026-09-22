package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The rules a change to the commune's working calendar must satisfy, in the one place that knows
// them (migration 0006, ADR 0007).
//
// THEY LIVE IN domain/ AND NOT IN THE HANDLER for the same reason domain/sla_ghi.go gives: the
// seeding path and the three edit paths both have to hold them, and a copy in each is a copy that
// drifts. domain/ imports nothing but the standard library.
//
// =================================================================================================
// WHAT THE DATABASE CANNOT CHECK, AND THEREFORE WHAT THIS FILE OWES.
//
// Migration 0006:109 hands the write path one obligation in writing and it is the expensive one:
//
//	"UNIQUE keeps two sessions from starting at the same minute, and the write path — which does
//	 not exist yet — must reject an overlap and carry a test for it."
//
// The EXCLUDE constraint that would do it in SQL needs `btree_gist`, and a migration that fails for
// a missing extension does not degrade — it stops the service (ADR 0013). So two sessions of
// 07:30–11:30 and 09:00–12:00 on one Monday are accepted by every constraint in the schema, and the
// two overlapping hours are then counted TWICE by the deadline function: the deadline comes out
// EARLIER than the commune's real hours, which is the direction that reports an authority late when
// it was not.
//
// THE OVERLAP CHECK HERE CALLS TimChongNhau, THE SAME FUNCTION THE READ PATH PUBLISHES `problems`
// FROM. That is the whole point of routing it through one function: a second implementation would
// drift, and the day it did, the write path would accept a shape the read path then reports as
// broken — with the person looking at the screen unable to tell why their save was allowed.
//
// =================================================================================================
// WHAT THIS FILE DELIBERATELY DOES NOT DO: it does not loosen a single refusal on the deadline path.
//
// An empty calendar still means FAILED_PRECONDITION out of grpc.AdvanceWorkingHours, and a commune
// with no session still cannot have a deadline computed. What the write path adds is a way for the
// calendar to STOP BEING EMPTY. Nothing here is a fallback, and nothing here may become one
// (rule 10, forbidden #3; migration 0006:56).

// TenLichToiDa bounds the free text on a calendar row — a session's `ghi_chu`, a holiday's `ten`, a
// swap day's `ten`.
//
// 200 RUNES IS FAR PAST "Làm bù nghỉ Tết theo Thông báo số 6046/VPCP-KGVX" AND FAR SHORT OF A
// DOCUMENT. The column is TEXT and takes anything; the bound exists because one process serves 200+
// communes and a field with no ceiling is memory a client chooses. It is counted in RUNES, not
// bytes: a Vietnamese sentence is roughly three bytes a character, and a byte bound would cut a
// commune's own language off at a third of the room it gives English.
const TenLichToiDa = 200

// LyDoXoaLichToiDa bounds the mandatory reason on a soft delete (rule 7, invariant 1).
const LyDoXoaLichToiDa = 500

var (
	// ErrThuKhongHopLe — ISO 8601 weekday, 1 = Monday … 7 = Sunday. The same range the CHECK
	// constraint admits (lich_lam_viec_thu_hop_le), refused here so the person reading the screen
	// gets a sentence instead of a constraint name.
	//
	// THE MESSAGE SPELLS OUT WHICH CONVENTION, because the caller most likely to send 0 is a
	// JavaScript client using `getDay()`, where Sunday is 0 and Monday is 1 — an off-by-one that
	// moves every session of the week by a day and produces no error anywhere (migration 0006:98).
	ErrThuKhongHopLe = errors.New("lịch làm việc: thứ phải từ 1 (thứ Hai) đến 7 (Chủ nhật) theo ISO 8601")

	// ErrGioKhongDocDuoc is a time of day that is not HH:MM or HH:MM:SS.
	ErrGioKhongDocDuoc = errors.New("lịch làm việc: giờ phải theo khuôn HH:MM hoặc HH:MM:SS, ví dụ 07:30")

	// ErrCaKhongCoDoDai — a session ending before or at its start. The same CHECK the schema carries
	// (lich_lam_viec_co_do_dai, ngay_lam_bu_co_do_dai); refused here so the sentence names the rule.
	ErrCaKhongCoDoDai = errors.New("lịch làm việc: giờ kết thúc phải sau giờ bắt đầu")

	// ErrNgayKhongDocDuoc is a date that is not `YYYY-MM-DD`.
	//
	// THE FORM IS EXACT AND `2026-9-2` IS REFUSED. It is the spelling the column renders, the
	// spelling the read contract publishes and the spelling the deadline function matches on
	// (caCuaNgay formats "2006-01-02" and looks the date up as a string). One shape, or the lookup
	// silently misses and a commune's holiday does nothing.
	ErrNgayKhongDocDuoc = errors.New("lịch làm việc: ngày phải theo khuôn YYYY-MM-DD, ví dụ 2026-09-02")

	// ErrThieuTenLich — a holiday or a swap day with no name.
	//
	// THE SCHEMA REFUSES IT TOO (ngay_nghi_le_co_ten, ngay_lam_bu_co_ten) AND THAT IS NOT DUPLICATION
	// WORTH REMOVING: the CHECK is the floor that holds against a restore or a hand-written INSERT,
	// this is the sentence a person reads. The name is what an inspection asking why a deadline ran
	// through a Saturday actually reads.
	ErrThieuTenLich = errors.New("lịch làm việc: phải có tên — đây là dòng chữ người đọc lịch nhìn thấy")

	// ErrTenLichQuaDai — see TenLichToiDa.
	ErrTenLichQuaDai = errors.New("lịch làm việc: tên quá dài")

	// ErrThieuLyDoXoaLich — rule 7, invariant 1 names three columns, and `delete_reason` with
	// nothing in it is a removal nobody can be asked about. A calendar row is the BASIS OF AN ISSUED
	// COMMITMENT (migration 0006:41): when an inspection asks why a petition received on 30/04 was
	// due on 05/05, the answer is the calendar as it stood that day — including why a row left it.
	ErrThieuLyDoXoaLich = errors.New("lịch làm việc: phải nêu lý do xoá")

	// ErrLyDoXoaLichQuaDai — see LyDoXoaLichToiDa.
	ErrLyDoXoaLichQuaDai = errors.New("lịch làm việc: lý do xoá quá dài")
)

// LaLoiDauVaoLich reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, following LaLoiDauVaoSLA: a default of "anything
// I do not recognise is the client's fault" turns a database outage into a 400, and a client that
// believes its input is wrong retries with different input forever while nobody is told the server
// is broken.
func LaLoiDauVaoLich(err error) bool {
	for _, mot := range []error{
		ErrThuKhongHopLe, ErrGioKhongDocDuoc, ErrCaKhongCoDoDai, ErrNgayKhongDocDuoc,
		ErrThieuTenLich, ErrTenLichQuaDai, ErrThieuLyDoXoaLich, ErrLyDoXoaLichQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}

// DocGioTrongNgay parses a wall-clock time of day as the contract spells it: HH:MM or HH:MM:SS.
//
// BOTH SHAPES ARE ACCEPTED ON THE WAY IN AND EXACTLY ONE IS PRODUCED ON THE WAY OUT
// (GioTrongNgay.Chuoi always writes HH:MM:SS). That asymmetry is deliberate: a person typing office
// hours writes "07:30", while a contract that carried two shapes would make every client parse both
// — and a client that handles only one handles the other wrong.
//
// 24:00:00 IS LEGAL AND IS NOT AN OFF-BY-ONE. PostgreSQL's TIME accepts it and the schema's
// `ket_thuc > bat_dau` permits a session that closes at midnight; GioTrongNgay is seconds since
// midnight precisely so this value has somewhere to go (see GiayTrongNgay). Nothing past it is
// accepted: 24:00:01 is not a time of day.
//
// NO time.Parse, AND THE REASON IS THE 24:00:00 CASE: time.Parse("15:04:05", "24:00:00") fails,
// because a time.Time cannot hold an hour of 24. Hand-parsing three integers is longer and is the
// version that accepts every value the column does.
func DocGioTrongNgay(s string) (GioTrongNgay, error) {
	phan := strings.Split(strings.TrimSpace(s), ":")
	if len(phan) != 2 && len(phan) != 3 {
		return 0, ErrGioKhongDocDuoc
	}
	// SECONDS DEFAULT TO ZERO AND NEVER TO "whatever was there before". A missing :SS means the
	// minute's start, which is what a person typing 07:30 means.
	so := [3]int{0, 0, 0}
	for i, p := range phan {
		// EXACTLY TWO DIGITS PER PART. strconv.Atoi alone would accept "7", "+7" and " 7", so
		// "7:3" would become 07:03 — a time the commune did not type, differing from what they
		// meant by 27 minutes, with nothing on screen to say so.
		if len(p) != 2 || p[0] < '0' || p[0] > '9' || p[1] < '0' || p[1] > '9' {
			return 0, ErrGioKhongDocDuoc
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, ErrGioKhongDocDuoc
		}
		so[i] = n
	}
	if so[1] > 59 || so[2] > 59 {
		return 0, ErrGioKhongDocDuoc
	}
	g := GioTrongNgay(so[0]*3600 + so[1]*60 + so[2])
	if g > GiayTrongNgay {
		return 0, ErrGioKhongDocDuoc
	}
	return g, nil
}

// ChuanHoaNgay refuses anything that is not exactly `YYYY-MM-DD`, and returns the date unchanged.
//
// THE ROUND TRIP IS THE CHECK. time.Parse accepts "2026-02-30" by rolling it into 2 March, so a
// commune entering a date that does not exist would get a holiday on a different day from the one
// they typed. Re-formatting the parsed value and comparing it against the input is what catches
// that — and it catches "2026-9-2" in the same line, which matters because the deadline function
// looks a date up as a STRING (caCuaNgay formats "2006-01-02"), so a second spelling is a holiday
// that silently does nothing.
func ChuanHoaNgay(s string) (string, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02", s)
	if err != nil || t.Format("2006-01-02") != s {
		return "", ErrNgayKhongDocDuoc
	}
	return s, nil
}

// ThuCuaNgay is the ISO weekday of a `YYYY-MM-DD` date: 1 = Monday … 7 = Sunday.
//
// IT GOES THROUGH thuISO, THE ONE WEEKDAY CONVERSION IN THIS SERVICE (tien_gio_lam_viec.go:342). A
// second conversion written here is exactly where an off-by-one lives for a year before anybody
// notices the deadline is a day out — and this one decides whether a swap day is refused under
// ADR 0007 decision 9.
//
// NO ZONE IS INVOLVED AND NONE IS NEEDED: which weekday a calendar date falls on is the same in
// every zone. The instant is produced elsewhere, once, in the deadline function.
func ThuCuaNgay(ngay string) (int, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(ngay))
	if err != nil {
		return 0, ErrNgayKhongDocDuoc
	}
	return thuISO(t), nil
}

// ChuanHoaTenLich trims and bounds the free text of a calendar row.
func ChuanHoaTenLich(ten string) (string, error) {
	ten = strings.TrimSpace(ten)
	switch {
	case ten == "":
		return "", ErrThieuTenLich
	case len([]rune(ten)) > TenLichToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTenLichQuaDai, TenLichToiDa)
	}
	return ten, nil
}

// ChuanHoaGhiChuCa trims and bounds a session's note. UNLIKE ChuanHoaTenLich IT ACCEPTS THE EMPTY
// STRING: `ghi_chu` has `DEFAULT ”` in the schema and is a label a commune writes for itself, not
// a name an inspection reads. A session with no note is an ordinary session; a holiday with no name
// is a row nobody can identify.
func ChuanHoaGhiChuCa(ghiChu string) (string, error) {
	ghiChu = strings.TrimSpace(ghiChu)
	if len([]rune(ghiChu)) > TenLichToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTenLichQuaDai, TenLichToiDa)
	}
	return ghiChu, nil
}

// ChuanHoaLyDoXoaLich refuses a soft delete with no reason (rule 7, invariant 1).
func ChuanHoaLyDoXoaLich(lyDo string) (string, error) {
	lyDo = strings.TrimSpace(lyDo)
	switch {
	case lyDo == "":
		return "", ErrThieuLyDoXoaLich
	case len([]rune(lyDo)) > LyDoXoaLichToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrLyDoXoaLichQuaDai, LyDoXoaLichToiDa)
	}
	return lyDo, nil
}

// KiemTraCaLamViec refuses one weekly session on its own — weekday and length.
//
// THE WHOLE ROW IS CHECKED EVEN WHEN ONLY ONE FIELD WAS EDITED, the same discipline as
// KiemTraDongSLA: the edit path applies the change to the row it read and validates the RESULT, so
// a row that was already bad cannot be committed again untouched by a screen that only meant to
// move the closing time.
func KiemTraCaLamViec(c CaLamViec) error {
	if c.Thu < 1 || c.Thu > 7 {
		return ErrThuKhongHopLe
	}
	return kiemTraDoDaiCa(c.BatDau, c.KetThuc)
}

// KiemTraCaLamBu refuses one swap-day session on its own — date, length, name.
func KiemTraCaLamBu(c CaLamBu) error {
	if _, err := ChuanHoaNgay(c.Ngay); err != nil {
		return err
	}
	if err := kiemTraDoDaiCa(c.BatDau, c.KetThuc); err != nil {
		return err
	}
	_, err := ChuanHoaTenLich(c.Ten)
	return err
}

// KiemTraNgayNghiLe refuses one closure date on its own — date and name.
func KiemTraNgayNghiLe(n NgayNghiLe) error {
	if _, err := ChuanHoaNgay(n.Ngay); err != nil {
		return err
	}
	_, err := ChuanHoaTenLich(n.Ten)
	return err
}

func kiemTraDoDaiCa(batDau, ketThuc GioTrongNgay) error {
	switch {
	case batDau < 0 || batDau > GiayTrongNgay || ketThuc < 0 || ketThuc > GiayTrongNgay:
		return ErrGioKhongDocDuoc
	case ketThuc <= batDau:
		return ErrCaKhongCoDoDai
	}
	return nil
}

// IDCaDangXet is the placeholder id the overlap check gives the session being written.
//
// IT EXISTS BECAUSE A SESSION BEING ADDED HAS NO ID YET, and TimChongNhau reports pairs by id. A
// constant that cannot be a ULID (26 characters, refused by `lich_lam_viec_id_la_ulid`) is safe to
// compare against: no stored row can ever carry it.
const IDCaDangXet = "ca-dang-xet"

// LoiCaChongCaKhac is one session overlapping another that this commune already has.
//
// A TYPE AND NOT A SENTINEL, because the sentence has to name the row somebody must go and look at.
// A refusal that says only "overlap" leaves a person staring at a week of sessions with no idea
// which two are the problem — and the whole reason this check exists is that the two rows look
// perfectly reasonable one at a time.
type LoiCaChongCaKhac struct {
	// CaID is the id of the EXISTING session, the one already stored. Never the id of the row being
	// written: that one may not exist yet, and pointing at it would send the person to fix the thing
	// they are currently typing.
	CaID string

	// Nhom is the weekday (as an ISO number) for the weekly calendar, or the `YYYY-MM-DD` date for a
	// swap day — whichever grouping the overlap was found in.
	Nhom string

	// LaNgay says which of the two Nhom is, so the sentence reads as a person would say it.
	LaNgay bool
}

func (e *LoiCaChongCaKhac) Error() string {
	// NEITHER A WEEKDAY, A DATE NOR A ROW ID IS PERSONAL DATA (rule 3): they say when an authority
	// is open. That is what makes it safe for this sentence to reach a screen.
	if e.LaNgay {
		return fmt.Sprintf("lịch làm việc: ca này chồng giờ với một ca đã có trong ngày %s (mã %s) — "+
			"giờ trong khoảng chồng sẽ bị tính hai lần", e.Nhom, e.CaID)
	}
	thu, err := strconv.Atoi(e.Nhom)
	if err != nil {
		thu = 0
	}
	return fmt.Sprintf("lịch làm việc: ca này chồng giờ với một ca đã có của %s (mã %s) — "+
		"giờ trong khoảng chồng sẽ bị tính hai lần", TenThuISO(thu), e.CaID)
}

// TenThuISO names an ISO weekday in Vietnamese, for refusal sentences.
//
// EXPORTED HERE RATHER THAN LEFT IN internal/http, where `tenThu` already says the same thing, and
// that duplication is the thing to remove rather than accept — but not in this turn's scope to
// rewrite the read route. STATED: two copies of one mapping is a rule 9 problem, and the one to
// keep is this one, because a refusal sentence is built in the domain where HTTP cannot reach.
func TenThuISO(thu int) string {
	switch thu {
	case 1:
		return "thứ Hai"
	case 2:
		return "thứ Ba"
	case 3:
		return "thứ Tư"
	case 4:
		return "thứ Năm"
	case 5:
		return "thứ Sáu"
	case 6:
		return "thứ Bảy"
	case 7:
		return "Chủ nhật"
	default:
		return "ngày không hợp lệ"
	}
}

// KhongChongCaNao refuses a session that overlaps one of the sessions already in its group.
//
// =================================================================================================
// IT CALLS TimChongNhau — THE SAME FUNCTION domain.VanDeCuaLich PUBLISHES `problems` FROM — AND
// THAT IS THE LOAD-BEARING DECISION OF THIS FILE. A second overlap implementation would drift from
// the first, and on the day it did, the write path would accept a shape the read path reports as
// broken: the commune sees a red problem on a row the server just told them was fine.
//
// HALF-OPEN INTERVALS COME WITH IT: a morning ending 11:30 and an afternoon starting 11:30 do NOT
// overlap. That is the ordinary shape of a working day and refusing it would make the lunch break
// — the thing this table exists to express — unconfigurable.
//
// `dangCo` MUST HOLD ONLY LIVE ROWS AND MUST EXCLUDE THE ROW BEING EDITED. A soft-deleted session
// is not a session (rule 7, invariant 2), and a row compared against itself overlaps itself
// perfectly — which would make every edit impossible.
// =================================================================================================
func KhongChongCaNao(moi Khoang, dangCo []Khoang) error {
	ks := make([]Khoang, 0, len(dangCo)+1)
	moi.ID = IDCaDangXet
	ks = append(ks, moi)
	for _, k := range dangCo {
		if k.ID == IDCaDangXet {
			// Unreachable while callers pass stored rows, whose ids are ULIDs. Dropped rather than
			// compared, because a fixture carrying the placeholder would make this function report
			// a row against itself and the failure would read as a genuine overlap.
			continue
		}
		ks = append(ks, k)
	}

	for _, cap := range TimChongNhau(ks) {
		khac := ""
		switch {
		case cap.A == IDCaDangXet:
			khac = cap.B
		case cap.B == IDCaDangXet:
			khac = cap.A
		default:
			// Two EXISTING rows overlap each other. Not this write's fault and not this write's
			// business: refusing here would make a commune unable to add a Tuesday session until
			// they had fixed a Monday one. The read route already names it as a problem.
			continue
		}
		return &LoiCaChongCaKhac{CaID: khac, Nhom: cap.Nhom, LaNgay: strings.Contains(cap.Nhom, "-")}
	}
	return nil
}

// KhoangCuaCaLamViec and KhoangCuaCaLamBu turn a row into the shape the overlap check compares.
//
// THE GROUP IS THE WEEKDAY FOR ONE AND THE DATE FOR THE OTHER, exactly as VanDeCuaLich and
// CaLamBuChongNhau already build them. Written here as functions so the write path cannot pick a
// different grouping from the read path — a swap day grouped by weekday would report two different
// Saturdays as clashing.
func KhoangCuaCaLamViec(c CaLamViec) Khoang {
	return Khoang{ID: c.ID, Nhom: strconv.Itoa(c.Thu), BatDau: c.BatDau, KetThuc: c.KetThuc}
}

func KhoangCuaCaLamBu(c CaLamBu) Khoang {
	return Khoang{ID: c.ID, Nhom: c.Ngay, BatDau: c.BatDau, KetThuc: c.KetThuc}
}

// LoiLamBuVaoNgayDaLamViec builds ADR 0007 decision 9's refusal for the WRITE path.
//
// =================================================================================================
// ONE FAULT, ONE VOCABULARY. The compute path already refuses this state — caCuaNgay returns
// *LoiKhongTinhDuocHan{Loai: LoiLamBuTrungNgayDaLam} — and this function builds THE SAME TYPE with
// THE SAME Loai, so a commune meeting the rule at the moment they save and a service meeting it at
// the moment it computes a deadline are told the same thing in the same words.
//
// WHY IT IS REFUSED AT ALL RATHER THAN RESOLVED (ADR 0007 decision 9): a swap day on a date whose
// weekday already has sessions reads two opposite ways — the swap day REPLACES those hours, or it
// ADDS to them. They differ by exactly the overlap; nothing decides between them; and loosening a
// refusal later is additive, while un-inventing a deadline computed from double-counted hours is
// not, because that deadline has already been told to a citizen (rule 10, invariant 2).
//
// CHECKING IT AT WRITE TIME IS AN ADDITION AND NOT A SUBSTITUTE. The compute-path refusal stays
// exactly where it is: this one cannot see a session added to the weekly calendar AFTER the swap
// day was written, which is the other direction of the same conflict (see the note on
// app.Lich.ThemCa).
// =================================================================================================
func LoiLamBuVaoNgayDaLamViec(ngay string, thu int) error {
	return &LoiKhongTinhDuocHan{
		Loai: LoiLamBuTrungNgayDaLam,
		ChiTiet: fmt.Sprintf(
			"lịch làm việc: ngày %s là %s, mà %s vốn đã có ca làm việc trong lịch tuần — "+
				"ngày làm bù chỉ dành cho ngày xã vốn KHÔNG làm việc. Hệ thống không tự chọn giữa "+
				"THAY và CỘNG THÊM giờ (ADR 0007 quyết định 9), vui lòng sửa cấu hình",
			ngay, TenThuISO(thu), TenThuISO(thu)),
	}
}
