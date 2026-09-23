package domain

// The WRITE rules of the meeting-minutes register — `docs/ui-ux/04-bien-ban-hop.md` §3, §4, §7.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). Every
// rule below is a pure function over values, so the two that actually carry a business decision —
// the ordinal a new conclusion gets (§7.2) and what may be typed into a conclusion at all — can be
// proved without a database. There is no PostgreSQL reachable from this build environment
// (VIGOV_TEST_DSN unset), so a rule that could only be tested against one would not be tested.
//
// # WHAT IS DELIBERATELY NOT HERE
//
//   - NO RULE ABOUT EDITING A CONCLUSION. Whether a conclusion that tasks have already been split
//     from may still have its text changed is a STOP CONDITION (migration 0007 says so in its own
//     words and deliberately declared no immutability trigger). This pass builds no edit route, so
//     no default has been baked in — and a check written here "ready for later" would be exactly
//     that default, in the layer everything else would then be built against.
//   - NO CEILING ON HOW MANY TASKS ONE CONCLUSION MAY PRODUCE. §3 says a conclusion splits into
//     MANY tasks and counts them as `x/y`; the schema constrains nothing. A limit invented here
//     would be a new business rule nobody asked for, refusing a real meeting.
//   - NO UNIQUENESS RULE ON THE MINUTES. `so_hieu` is typed by hand, optional, and restarts each
//     year — migration 0007 refuses a unique key on it for that reason, and this layer does not
//     re-introduce one by the back door.

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// --- bounds -------------------------------------------------------------------------------------

const (
	// TenCuocHopToiDa bounds §4's "Tên cuộc họp" — `Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026`.
	// Long enough for a full Vietnamese meeting title, short enough that the column is not a body.
	TenCuocHopToiDa = 300

	// SoHieuBienBanToiDa bounds the reference number of the written minutes, `31/BB-UBND`.
	SoHieuBienBanToiDa = 64

	DiaDiemBienBanToiDa = 300

	// NoiDungBienBanToiDa bounds §4's "Nội dung biên bản" — the minutes IN FULL, which is what that
	// field is for. It is the largest text this service accepts anywhere, deliberately.
	NoiDungBienBanToiDa = 50000

	// NoiDungKetLuanToiDa bounds ONE conclusion (§2's ① rows). The same ceiling as a task's
	// description, because a conclusion routinely becomes one.
	NoiDungKetLuanToiDa = 5000

	// KetLuanMoiLanToiDa bounds how many conclusions ONE request may carry — §4's dynamic list on
	// the create form, and nothing else. It is NOT a ceiling on how many conclusions a meeting may
	// hold: `POST .../conclusions` appends one at a time and is not counted against it.
	//
	// WHY A BOUND AT ALL: every conclusion in the array becomes an INSERT inside one transaction, so
	// an unbounded array is an unbounded transaction on a government register.
	KetLuanMoiLanToiDa = 50

	// ThanhPhanToiDa bounds §4's "Thành phần tham dự" list, and ThanhPhanMotDongToiDa bounds one
	// entry of it. A commune's meeting has tens of attendees, not thousands.
	ThanhPhanToiDa        = 200
	ThanhPhanMotDongToiDa = 200

	// ChuTriMaToiDa bounds the chair's STAFF BUSINESS CODE (`CB-2026-7K3M9Q`) — the same shape the
	// task register's staff-code columns hold.
	ChuTriMaToiDa = 32
)

// --- refusals of what the client sent -------------------------------------------------------------

var (
	ErrThieuTenCuocHop = errors.New(
		"biên bản họp: thiếu tên cuộc họp — thẻ không có tiêu đề là thẻ không ai nhận ra trong danh sách")
	ErrTenCuocHopQuaDai = fmt.Errorf(
		"biên bản họp: tên cuộc họp quá dài (tối đa %d ký tự)", TenCuocHopToiDa)

	// ErrThieuNgayHop refuses minutes with no meeting day. §4 marks "Ngày họp" required, and the
	// column is NOT NULL: a zero date reaching the column would be the year 1 on the card.
	ErrThieuNgayHop = errors.New("biên bản họp: thiếu ngày họp")

	// ErrNgayHopKhongDocDuoc refuses a date the wire format cannot read. It is a CALENDAR DAY
	// (`2026-08-05`) and not an instant — see the note on the HTTP layer's ngayHopVao.
	ErrNgayHopKhongDocDuoc = errors.New(
		"biên bản họp: ngày họp phải là ngày lịch dạng `2026-08-05`")

	ErrSoHieuBienBanQuaDai = fmt.Errorf(
		"biên bản họp: số hiệu biên bản quá dài (tối đa %d ký tự)", SoHieuBienBanToiDa)
	ErrDiaDiemBienBanQuaDai = fmt.Errorf(
		"biên bản họp: địa điểm quá dài (tối đa %d ký tự)", DiaDiemBienBanToiDa)
	ErrNoiDungBienBanQuaDai = fmt.Errorf(
		"biên bản họp: nội dung biên bản quá dài (tối đa %d ký tự)", NoiDungBienBanToiDa)
	ErrChuTriQuaDai = fmt.Errorf(
		"biên bản họp: mã cán bộ chủ trì quá dài (tối đa %d ký tự)", ChuTriMaToiDa)

	ErrThieuNoiDungKetLuan = errors.New(
		"biên bản họp: thiếu nội dung kết luận — một kết luận rỗng là một dòng không ai thi hành được")
	ErrNoiDungKetLuanQuaDai = fmt.Errorf(
		"biên bản họp: nội dung kết luận quá dài (tối đa %d ký tự)", NoiDungKetLuanToiDa)

	ErrQuaNhieuKetLuan = fmt.Errorf(
		"biên bản họp: một lần nhập tối đa %d kết luận — thêm tiếp bằng nút `+ Thêm kết luận`",
		KetLuanMoiLanToiDa)

	ErrQuaNhieuThanhPhan = fmt.Errorf(
		"biên bản họp: thành phần tham dự tối đa %d dòng", ThanhPhanToiDa)
	ErrThanhPhanQuaDai = fmt.Errorf(
		"biên bản họp: một dòng thành phần tham dự quá dài (tối đa %d ký tự)", ThanhPhanMotDongToiDa)
)

// LaLoiDauVaoBienBan reports whether this is a refusal of WHAT THE CLIENT SENT, as opposed to a
// failure or a refusal about the state of the record.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, the same discipline LaLoiDauVaoNhiemVu uses: a
// default of "anything I do not recognise is the client's fault" turns a database outage into a 400,
// and a client that believes its input is wrong retries with different input for ever while nobody
// is told the server is broken.
func LaLoiDauVaoBienBan(err error) bool {
	for _, mot := range []error{
		ErrThieuTenCuocHop, ErrTenCuocHopQuaDai,
		ErrThieuNgayHop, ErrNgayHopKhongDocDuoc,
		ErrSoHieuBienBanQuaDai, ErrDiaDiemBienBanQuaDai, ErrNoiDungBienBanQuaDai,
		ErrChuTriQuaDai,
		ErrThieuNoiDungKetLuan, ErrNoiDungKetLuanQuaDai, ErrQuaNhieuKetLuan,
		ErrQuaNhieuThanhPhan, ErrThanhPhanQuaDai,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}

// --- field checks ----------------------------------------------------------------------------------

// KiemTenCuocHop trims and bounds the one mandatory text of §4's form.
//
// MEASURED IN RUNES AND NOT BYTES, here and in every check below. Vietnamese is three bytes per
// accented character in UTF-8, so a byte bound would cut a Vietnamese title at a third of the length
// it cuts an English one — and the person who hits it has no way to tell why.
func KiemTenCuocHop(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuTenCuocHop
	case len([]rune(s)) > TenCuocHopToiDa:
		return "", ErrTenCuocHopQuaDai
	}
	return s, nil
}

// KiemNoiDungKetLuan trims and bounds ONE conclusion. MANDATORY: the schema's
// `ket_luan_hop_noi_dung_khong_rong` refuses an empty one, and a conclusion with nothing in it is a
// row a task would point back at for ever while saying nothing.
func KiemNoiDungKetLuan(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuNoiDungKetLuan
	case len([]rune(s)) > NoiDungKetLuanToiDa:
		return "", ErrNoiDungKetLuanQuaDai
	}
	return s, nil
}

// ChuanHoaNgayHop refuses a missing meeting day and keeps the value a CALENDAR DAY.
//
// # WHY THE TIME OF DAY IS CUT OFF HERE RATHER THAN LEFT TO THE COLUMN
//
// `ngay_hop` is DATE (migration 0007), so PostgreSQL would drop the time part anyway — but it would
// drop it AFTER converting through the session's time zone, which can move the day. Cutting it here,
// in one place, means the day that reaches the column is the day the form sent and the day the card
// renders. Nothing in this system COUNTS from this value: a deadline is working-hours arithmetic and
// belongs to identity (rule 10, invariant 4; ADR 0007), and this is a calendar fact.
//
// ⚠ THE CALENDAR DAY IS READ IN THE VALUE'S OWN LOCATION AND THEN PINNED TO UTC MIDNIGHT — it is NOT
// converted to UTC first. `2026-08-05 01:00 +07` means the fifth of August to the person who typed
// it; converting first would store the fourth, and the card would show a meeting held the day before
// it was. On the real path the value arrives from `time.Parse("2006-01-02", …)`, which is already
// UTC midnight, so this branch protects a caller that has not gone through the wire format.
func ChuanHoaNgayHop(t time.Time) (time.Time, error) {
	if t.IsZero() {
		return time.Time{}, ErrThieuNgayHop
	}
	nam, thang, ngay := t.Date()
	return time.Date(nam, thang, ngay, 0, 0, 0, 0, time.UTC), nil
}

// KiemThanhPhan trims §4's attendee list and drops blank lines.
//
// ⚠ IT IS A LIST OF STAFF, NOT OF CITIZENS. Migration 0007 says so on the column: the day somebody
// writes a reporter's name and phone number in here, this table has become a personal-data store
// (rule 3). Nothing in this file can enforce that — a free-text line accepts anything — which is
// exactly why it is written down at every layer that touches the column.
func KiemThanhPhan(ds []string) ([]string, error) {
	if len(ds) > ThanhPhanToiDa {
		return nil, ErrQuaNhieuThanhPhan
	}
	// NEVER nil: the column is `NOT NULL DEFAULT '[]'`, and a reader must not have to tell "nobody
	// recorded the attendees" from "column not set".
	ra := make([]string, 0, len(ds))
	for _, s := range ds {
		s = strings.TrimSpace(s)
		if s == "" {
			// A BLANK LINE IS DROPPED, NOT REFUSED. §4's control is a multi-select plus free text, so
			// an empty row is the widget's own leftover rather than something the clerk meant.
			continue
		}
		if len([]rune(s)) > ThanhPhanMotDongToiDa {
			return nil, ErrThanhPhanQuaDai
		}
		ra = append(ra, s)
	}
	return ra, nil
}

// ThuTuKetLuanTiepTheo is §7.2 in one line: "đánh số liên tục từ 1 trong phạm vi một biên bản; thêm
// mới thì nối tiếp".
//
// # IT COUNTS FROM THE HIGHEST NUMBER EVER ISSUED, NOT FROM HOW MANY ROWS ARE LIVE
//
// A soft-deleted conclusion KEEPS its number: `UNIQUE (tenant_id, bien_ban_id, thu_tu)` deliberately
// counts deleted rows (migration 0007), the printed minutes still say "②", and the tasks split from
// it still point at it. So a gap after a removal is the correct outcome, and `len(live) + 1` — the
// obvious-looking alternative — would mint a SECOND ② whose tasks are indistinguishable from the
// first's on every screen that shows the ordinal.
func ThuTuKetLuanTiepTheo(lonNhatDaCap int) int {
	if lonNhatDaCap < 0 {
		// Defensive, and it costs nothing: a negative maximum can only come from a store failure
		// scanned into a zero value, and `0` is the answer that keeps ① as the first number.
		lonNhatDaCap = 0
	}
	return lonNhatDaCap + 1
}
