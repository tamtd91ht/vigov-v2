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
//   - NO PURE FUNCTION FOR "a conclusion with live tasks is locked" (decision 3, 25/09/2026). The
//     rule needs a COUNT of live tasks read under the meeting's lock, so it lives in the use case
//     (internal/app/bien_ban_hop_sua.go); what lives here is only its sentence, ErrKetLuanDaCoNhiemVu.
//     Migration 0012's header says why no trigger holds it either.
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

	// --- migration 0012's fields (user decisions 25/09/2026) ------------------------------------

	ErrThuKyQuaDai = fmt.Errorf(
		"biên bản họp: mã cán bộ thư ký quá dài (tối đa %d ký tự)", ChuTriMaToiDa)

	// The conclusion notice travels as a PAIR (CHECK `bien_ban_hop_thong_bao_du_truong`): half a
	// reference is a citation nobody can find in the office's register.
	ErrThieuSoThongBao = errors.New(
		"biên bản họp: thiếu số, ký hiệu Thông báo kết luận — số và ngày đi cùng nhau")
	ErrSoThongBaoQuaDai = fmt.Errorf(
		"biên bản họp: số, ký hiệu Thông báo kết luận quá dài (tối đa %d ký tự)", SoHieuBienBanToiDa)
	ErrThieuNgayThongBao = errors.New(
		"biên bản họp: thiếu ngày Thông báo kết luận — số và ngày đi cùng nhau")
	ErrNgayThongBaoKhongDocDuoc = errors.New(
		"biên bản họp: ngày Thông báo kết luận phải là ngày lịch dạng `2026-08-12`")

	ErrThieuLyDoXoaBienBan = errors.New(
		"biên bản họp: thiếu lý do xoá — biên bản là hồ sơ lưu trữ, xoá mềm phải ghi ai xoá và vì sao")
	ErrLyDoXoaBienBanQuaDai = fmt.Errorf(
		"biên bản họp: lý do xoá quá dài (tối đa %d ký tự)", LyDoXoaBienBanToiDa)

	// ErrBoSungChoBanNhap refuses supplementary minutes pointing at a DRAFT. A draft is corrected by
	// editing it; supplementary minutes exist because a SIGNED record may not change by one word
	// (decision 1). Migration 0012 leaves this check to the use case on purpose (its header).
	ErrBoSungChoBanNhap = errors.New(
		"biên bản họp: biên bản bổ sung chỉ lập cho biên bản ĐÃ KÝ — biên bản còn nháp thì sửa trực tiếp")
)

// --- refusals about the STATE of the record (409), never about what the client sent -------------
//
// NOT IN LaLoiDauVaoBienBan, deliberately: the caller holds the right and sent a well-formed request;
// what is refused is this act on THIS record, because of the state it is in. A 400 would tell the
// clerk to fix a form that has nothing wrong with it.
var (
	// ErrBienBanDaKy — signed minutes are locked (decision 1): content, conclusions, chair, secretary,
	// the no-task mark and the soft delete. The ONE thing still writable is the conclusion notice,
	// once. The sentence says what to do instead, because "locked" alone sends the clerk hunting.
	ErrBienBanDaKy = errors.New(
		"biên bản họp đã ký — nội dung, kết luận, chủ trì, thư ký đều bị khoá và không xoá được. " +
			"Sai sót sau khi ký thì lập biên bản bổ sung trỏ về biên bản này")

	// ErrDaCoThongBao — the notice of SIGNED minutes is recorded once (trigger
	// `bien_ban_hop_da_ky_bat_bien`). A correction is an act on paper, not an edit here.
	ErrDaCoThongBao = errors.New(
		"biên bản họp: số và ngày Thông báo kết luận đã ghi — sau khi ký chỉ ghi được một lần")

	// ErrKetLuanDaCoNhiemVu — decision 3: a conclusion tasks were split from is locked even in draft.
	// Its text is what those tasks quote; rewording it would make every task point at a sentence
	// nobody assigned.
	ErrKetLuanDaCoNhiemVu = errors.New(
		"kết luận đã được tách thành nhiệm vụ — nội dung bị khoá, không xoá được và không đánh dấu " +
			"'không phát sinh nhiệm vụ' được. Hãy xử lý các nhiệm vụ đó trước")

	// ErrBienBanConNhiemVu — removing minutes whose conclusions still have live tasks. Wrapped by
	// LoiBienBanConNhiemVu with the count, because "you may not" without "what is in the way" sends a
	// clerk hunting.
	ErrBienBanConNhiemVu = errors.New("biên bản họp còn nhiệm vụ đang trỏ về kết luận")

	// ErrKetLuanKhongPhatSinh — decision 4 read from the other side: while the mark "không phát sinh
	// nhiệm vụ" is set, splitting the conclusion into a task would make the record say two opposite
	// things at once.
	ErrKetLuanKhongPhatSinh = errors.New(
		"kết luận đang được đánh dấu 'không phát sinh nhiệm vụ' — bỏ dấu trước rồi mới tách thành nhiệm vụ")
)

// LoiConNhiemVu is ErrBienBanConNhiemVu carrying the count. A TYPE rather than a wrapped string so the
// HTTP layer can return exactly this sentence — the wrapping the use case adds on the way out names
// the commune, which is an operator's detail, not the clerk's.
type LoiConNhiemVu struct{ SoNhiemVu int }

func (e *LoiConNhiemVu) Error() string {
	return fmt.Sprintf("%s: còn %d nhiệm vụ — hãy xử lý (hoặc xoá) các nhiệm vụ đó trước rồi mới xoá biên bản",
		ErrBienBanConNhiemVu, e.SoNhiemVu)
}

func (e *LoiConNhiemVu) Is(dich error) bool { return dich == ErrBienBanConNhiemVu }

// LoiBienBanConNhiemVu names how many live tasks stand in the way of removing the minutes.
func LoiBienBanConNhiemVu(n int) error { return &LoiConNhiemVu{SoNhiemVu: n} }

// LyDoXoaBienBanToiDa bounds the reason recorded beside a soft delete of minutes or of one conclusion
// — the same ceiling the catalogues use (LyDoXoaToiDa).
const LyDoXoaBienBanToiDa = 500

// LaLoiDauVaoBienBan reports whether this is a refusal of WHAT THE CLIENT SENT, as opposed to a
// failure or a refusal about the state of the record.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, the same discipline LaLoiDauVaoNhiemVu uses: a
// default of "anything I do not recognise is the client's fault" turns a database outage into a 400,
// and a client that believes its input is wrong retries with different input for ever while nobody
// is told the server is broken.
func LaLoiDauVaoBienBan(err error) bool { return LoiDauVaoBienBanGoc(err) != nil }

// LoiDauVaoBienBanGoc returns the SENTINEL an input refusal wraps, or nil. The HTTP layer answers
// with the sentinel's own sentence: a refusal raised inside the transaction arrives wrapped with the
// commune and the operation (app.bocBienBan), which belong in the operator's log and not on the wire.
func LoiDauVaoBienBanGoc(err error) error {
	for _, mot := range []error{
		ErrThieuTenCuocHop, ErrTenCuocHopQuaDai,
		ErrThieuNgayHop, ErrNgayHopKhongDocDuoc,
		ErrSoHieuBienBanQuaDai, ErrDiaDiemBienBanQuaDai, ErrNoiDungBienBanQuaDai,
		ErrChuTriQuaDai,
		ErrThieuNoiDungKetLuan, ErrNoiDungKetLuanQuaDai, ErrQuaNhieuKetLuan,
		ErrQuaNhieuThanhPhan, ErrThanhPhanQuaDai,
		ErrThuKyQuaDai,
		ErrThieuSoThongBao, ErrSoThongBaoQuaDai, ErrThieuNgayThongBao, ErrNgayThongBaoKhongDocDuoc,
		ErrThieuLyDoXoaBienBan, ErrLyDoXoaBienBanQuaDai,
		ErrBoSungChoBanNhap,
	} {
		if errors.Is(err, mot) {
			return mot
		}
	}
	return nil
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

// KiemLyDoXoaBienBan refuses a soft delete that records no reason (rule 7, invariant 1) — for the
// minutes and for one conclusion alike.
func KiemLyDoXoaBienBan(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuLyDoXoaBienBan
	case len([]rune(s)) > LyDoXoaBienBanToiDa:
		return "", ErrLyDoXoaBienBanQuaDai
	}
	return s, nil
}

// KiemThongBaoKetLuan checks the conclusion notice (Thông báo kết luận) as a PAIR: a non-blank
// reference number and a calendar day, both or the request is refused. Neither is MINTED here — the
// office clerk numbers the notice in the register (Decree 30/2020, Art. 15); this is a transcription.
//
// The day is pinned to a calendar day by ChuanHoaNgayHop's rule, for ChuanHoaNgayHop's reason.
func KiemThongBaoKetLuan(so string, ngay time.Time) (string, time.Time, error) {
	so = strings.TrimSpace(so)
	switch {
	case so == "":
		return "", time.Time{}, ErrThieuSoThongBao
	case len([]rune(so)) > SoHieuBienBanToiDa:
		return "", time.Time{}, ErrSoThongBaoQuaDai
	case ngay.IsZero():
		return "", time.Time{}, ErrThieuNgayThongBao
	}
	nam, thang, d := ngay.Date()
	return so, time.Date(nam, thang, d, 0, 0, 0, 0, time.UTC), nil
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
