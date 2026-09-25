package domain

// The three lists of referenced documents that hang off a task — `docs/ui-ux/02-nhiem-vu.md` §5.4
// ("SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO") and §7.2 ("danh sách động"), table `nhiem_vu_van_ban`
// (migration 0009).
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). The one
// rule that actually costs something — what a write request does to the lines already stored — is a
// PURE FUNCTION over values, precisely so it can be proved without a database. There is no
// PostgreSQL reachable from this build environment (VIGOV_TEST_DSN unset), so a rule expressed only
// as SQL would be a rule nothing ever executes.
//
// # ⚠ THE NAME SAYS "văn bản" AND NOTHING HERE TOUCHES service-documents
//
// A line is FREE TEXT A CLERK TYPED, not a reference to a row in the document register. §7.2's
// input is a textarea whose placeholder is a Vietnamese sentence, and the documents it names are
// usually a SUPERIOR BODY's ("Thông báo số 90-TB/TU của Thành uỷ") — which never enters a commune's
// own incoming register, so there is nothing over there to point at. Migration 0009 states the
// whole argument; do not "complete" this with a cross-service id.

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// --- the three groups --------------------------------------------------------------------------

// NhomVanBanNhiemVu is which of §5.4's three boxes a line belongs to.
//
// THE LIST IS CLOSED, and for a reason that is different from the task lifecycle's: each code names
// a FIXED BOX ON THE SCREEN with its own label and its own placeholder, written out in §5.4 and
// §7.2. A commune cannot add a fourth group because there is nowhere to draw it — so this is a
// CHECK in the schema, never a `danh_muc` table with a `Tắt` button.
//
// The codes are Vietnamese without diacritics, kebab-case: enum VALUES are never translated into
// English (ADR 0011). The LABELS are deliberately not copied here — they live on the screen, and a
// second copy of a display string is a second copy that drifts.
type NhomVanBanNhiemVu string

const (
	VanBanCapTrenGiao  NhomVanBanNhiemVu = "cap-tren-giao"
	VanBanChiDaoDangUy NhomVanBanNhiemVu = "chi-dao-dang-uy"
	VanBanSanPhamRa    NhomVanBanNhiemVu = "san-pham-dau-ra"
)

// HopLe reports whether the string is one of the three.
//
// FAIL CLOSED: an unknown code can only come from a caller that made one up, and a line filed under
// a group no box renders is a line nobody ever sees again — on a record that is never hard-deleted.
func (n NhomVanBanNhiemVu) HopLe() bool {
	switch n {
	case VanBanCapTrenGiao, VanBanChiDaoDangUy, VanBanSanPhamRa:
		return true
	}
	return false
}

// --- the record ---------------------------------------------------------------------------------

// NhiemVuVanBan is ONE line of one of the three lists, as it is stored.
//
// IT IS A FIELD VALUE OF A TASK AND NOT A RECORD OF ITS OWN — migration 0009 sets out the evidence
// (§9 gives it no author and no instant, while `nhat_ky_nhiem_vu` next to it has both). That is why
// there is no `NguoiTaoMa` and no business timestamp on this struct: adding one would assert a fact
// the specification does not record and no screen shows.
type NhiemVuVanBan struct {
	ID string

	// NhiemVuID is the task this line is part of. The schema holds a real composite foreign key on
	// it, so a line cannot hang off another commune's task.
	NhiemVuID string

	Nhom NhomVanBanNhiemVu

	// SoKyHieu (`1742-CV/BTCTU`) and NgayVanBan are the STRUCTURED half of §5.4's rendering, and
	// both are EMPTY/ZERO in the ordinary case today: §7.2 offers one textarea and does not split
	// them out. Nothing in this package parses TrichYeu to fill them — guessing a reference number
	// off a Vietnamese sentence prints a document number that does not exist.
	SoKyHieu   string
	NgayVanBan time.Time

	// TrichYeu is the text of the line. MANDATORY: an empty line in a dynamic list is a row nobody
	// can read and nobody can act on, and `✕` is how a line is removed.
	//
	// ⚠ IT IS SENSITIVE BUSINESS TEXT. An administrative document's subject line routinely names a
	// citizen's case ("về việc giải quyết đơn của hộ ông…"), so it must not reach a log line, an
	// error message returned to a client, or a file name (rule 3).
	TrichYeu string

	// ThuTu is the position WITHIN ITS GROUP — an ISSUED NUMBER, not an array index.
	//
	// ⚠ IT IS NEVER RECOMPUTED FROM THE ORDER A CLIENT SENT. See SoSanhVanBan, and migration 0009's
	// own block on this: renumbering by array position both loses a recorded fact and collides with
	// `UNIQUE (tenant_id, nhiem_vu_id, nhom, thu_tu)`, which counts soft-deleted rows.
	ThuTu int
}

// VanBanNhiemVuVao is ONE line as a WRITE REQUEST carries it.
//
// # IT HAS NO `ThuTu` AND NO `NhiemVuID`, AND BOTH ABSENCES ARE THE RULE RATHER THAN AN OVERSIGHT
//
// A position is MINTED BY THE REGISTER (max + 1 within the group) and a line's task comes from the
// URL. A request able to carry either could place a line at a number another line already holds, or
// hang a line off a task the caller never named — and a struct with the field is a struct somebody
// will eventually fill in.
//
// AN EMPTY ID MEANS A NEW LINE. That is the whole of `+ Thêm văn bản`.
type VanBanNhiemVuVao struct {
	ID         string
	Nhom       NhomVanBanNhiemVu
	SoKyHieu   string
	NgayVanBan time.Time
	TrichYeu   string
}

// --- bounds and refusals -------------------------------------------------------------------------

const (
	// SoKyHieuVanBanToiDa bounds `1742-CV/BTCTU`. Long enough for the longest Vietnamese reference
	// number anybody types, short enough that the column is not a second summary field.
	SoKyHieuVanBanToiDa = 100

	// TrichYeuVanBanToiDa bounds the line's text. It is the sentence §7.2's placeholder shows
	// ("Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo…"), not a document body.
	TrichYeuVanBanToiDa = 1000

	// VanBanNhiemVuToiDa bounds how many lines ONE REQUEST may carry, across all three groups.
	//
	// IT IS A SAFETY NET AND NOT A BUSINESS LIMIT. §5.4's sample task carries one line per group and
	// no commune types dozens; what this stops is a request that would open a transaction on a
	// government register and then run a few thousand statements inside it while holding the task's
	// row lock. The refusal names the bound, so a commune that really hits it is told why rather
	// than left with a request that never returns.
	VanBanNhiemVuToiDa = 100
)

var (
	ErrNhomVanBanKhongHopLe = errors.New(
		"nhiệm vụ: nhóm văn bản không phải một trong ba nhóm của sổ theo dõi văn bản chỉ đạo")

	ErrThieuTrichYeuVanBan = errors.New(
		"nhiệm vụ: thiếu nội dung dòng văn bản — dòng trống thì gỡ hẳn khỏi danh sách")
	ErrTrichYeuVanBanQuaDai = fmt.Errorf(
		"nhiệm vụ: nội dung dòng văn bản quá dài (tối đa %d ký tự)", TrichYeuVanBanToiDa)
	ErrSoKyHieuVanBanQuaDai = fmt.Errorf(
		"nhiệm vụ: số ký hiệu văn bản quá dài (tối đa %d ký tự)", SoKyHieuVanBanToiDa)

	ErrQuaNhieuVanBan = fmt.Errorf(
		"nhiệm vụ: một lần gửi chỉ nhận tối đa %d dòng văn bản", VanBanNhiemVuToiDa)

	// ErrVanBanKhongThuocNhiemVu means the request named a line id that is not a LIVE line of this
	// task.
	//
	// REFUSED, NEVER CREATED UNDER THE SENT ID. Accepting it would let a client choose a line's
	// internal id — and the id of a line of ANOTHER task, or of a line somebody removed a second
	// ago, would silently become a line of this one.
	ErrVanBanKhongThuocNhiemVu = errors.New(
		"nhiệm vụ: dòng văn bản này không thuộc nhiệm vụ đang sửa — hãy tải lại rồi thao tác lại")

	// ErrVanBanTrungTrongYeuCau refuses one line named twice in one request.
	//
	// It is not pedantry: the two entries would carry two different texts, and whichever was applied
	// second would win silently — a screen showing one of the two with no way to tell which.
	ErrVanBanTrungTrongYeuCau = errors.New(
		"nhiệm vụ: một dòng văn bản được gửi hai lần trong cùng một yêu cầu")

	// ErrDoiNhomVanBan refuses moving a stored line from one of the three boxes to another.
	//
	// NOBODY HAS SPECIFIED THAT ACT. §5.4 and §7.2 draw three independent lists with an `✕` and a
	// `+ Thêm văn bản` each, and no control that moves a line between them. Allowing it silently
	// would also break the numbering: `thu_tu` is unique within (task, group) and counts
	// soft-deleted rows, so a line arriving in a group where its number is taken is a constraint
	// error inside the business transaction. Removing the line and adding it in the other group is
	// the act that already exists, and it is honest about what happened.
	ErrDoiNhomVanBan = errors.New(
		"nhiệm vụ: không chuyển được một dòng văn bản sang nhóm khác — gỡ dòng ấy rồi thêm lại ở " +
			"nhóm mới")
)

// LaLoiDauVaoVanBanNhiemVu reports whether this is a refusal of WHAT THE CLIENT SENT, as opposed to
// a failure or a refusal about the state of the record.
//
// LISTED EXPLICITLY, the same discipline LaLoiDauVaoNhiemVu uses and for the same reason: a default
// of "anything I do not recognise is the client's fault" turns a database outage into a 400, and a
// client that believes its input is wrong retries for ever while nobody is told the server broke.
//
// ⚠ ErrVanBanKhongThuocNhiemVu AND ErrDoiNhomVanBan ARE DELIBERATELY NOT IN THIS LIST. They are
// about the STATE OF THE RECORD — the line is not there any more, or it sits in another group — so
// the HTTP layer answers 409 and tells the officer to reload, which is a true statement they can act
// on. Folding them in here would tell them to fix their form.
func LaLoiDauVaoVanBanNhiemVu(err error) bool { return LoiDauVaoVanBanNhiemVuGoc(err) != nil }

// LoiDauVaoVanBanNhiemVuGoc returns the SENTINEL such a refusal wraps, or nil — the sentence the
// HTTP layer answers with, never the wrapped chain (see LoiDauVaoNhiemVuGoc).
func LoiDauVaoVanBanNhiemVuGoc(err error) error {
	for _, mot := range []error{
		ErrNhomVanBanKhongHopLe,
		ErrThieuTrichYeuVanBan, ErrTrichYeuVanBanQuaDai, ErrSoKyHieuVanBanQuaDai,
		ErrQuaNhieuVanBan, ErrVanBanTrungTrongYeuCau,
	} {
		if errors.Is(err, mot) {
			return mot
		}
	}
	return nil
}

// --- field checks ---------------------------------------------------------------------------------

// KiemVanBanNhiemVuVao trims and bounds ONE line of a write request.
//
// MEASURED IN RUNES AND NOT BYTES, here as everywhere in this package: Vietnamese is three bytes per
// accented character in UTF-8, so a byte bound cuts a Vietnamese sentence at a third of the length
// it cuts an English one, and the person who hits it has no way to tell why.
//
// THE DATE IS NOT VALIDATED AGAINST ANYTHING. A document may legitimately be dated years back (a
// 2019 directive still being implemented) and, less often, ahead of today (a document signed with a
// future effective date, typed in before it arrives). Refusing either would refuse real
// configuration — the lesson written into service-identity/migrations/0008_sla.sql:237-242.
func KiemVanBanNhiemVuVao(v VanBanNhiemVuVao) (VanBanNhiemVuVao, error) {
	if !v.Nhom.HopLe() {
		return VanBanNhiemVuVao{}, ErrNhomVanBanKhongHopLe
	}

	v.TrichYeu = strings.TrimSpace(v.TrichYeu)
	switch {
	case v.TrichYeu == "":
		return VanBanNhiemVuVao{}, ErrThieuTrichYeuVanBan
	case len([]rune(v.TrichYeu)) > TrichYeuVanBanToiDa:
		return VanBanNhiemVuVao{}, ErrTrichYeuVanBanQuaDai
	}

	v.SoKyHieu = strings.TrimSpace(v.SoKyHieu)
	if len([]rune(v.SoKyHieu)) > SoKyHieuVanBanToiDa {
		return VanBanNhiemVuVao{}, ErrSoKyHieuVanBanQuaDai
	}

	v.ID = strings.TrimSpace(v.ID)
	return v, nil
}

// KiemDanhSachVanBanNhiemVu checks and trims a whole request block.
//
// IT RUNS BEFORE THE TRANSACTION OPENS, which is why it is separate from SoSanhVanBan: a rejected
// form must hold no row lock on a government register.
//
// THE RETURNED SLICE IS NEVER nil WHEN THE INPUT WAS NON-nil, so a caller can tell "the client sent
// an empty list" (remove everything) from "the client did not send the block at all" (leave it
// alone) by the pointer it holds, not by the length of this result.
func KiemDanhSachVanBanNhiemVu(ds []VanBanNhiemVuVao) ([]VanBanNhiemVuVao, error) {
	if len(ds) > VanBanNhiemVuToiDa {
		return nil, ErrQuaNhieuVanBan
	}
	ra := make([]VanBanNhiemVuVao, 0, len(ds))
	for _, v := range ds {
		sach, err := KiemVanBanNhiemVuVao(v)
		if err != nil {
			return nil, err
		}
		ra = append(ra, sach)
	}
	return ra, nil
}

// --- the one rule that costs something ------------------------------------------------------------

// ThayDoiVanBan is what a request does to the lines already stored: the three sets, decided once.
//
// ONE STRUCT RATHER THAN THREE RETURN VALUES, so a caller cannot apply two of the three and forget
// the third — which on this block would look exactly like a successful save that quietly kept a
// removed line.
type ThayDoiVanBan struct {
	// Them are NEW lines. THEIR `ThuTu` IS ZERO HERE ON PURPOSE: the position is minted against the
	// store's own high-water mark inside the transaction, and a value invented in this package would
	// be a second place the numbering rule lives.
	Them []VanBanNhiemVuVao

	// Sua are stored lines whose text the request changed. `ThuTu` and `Nhom` are carried from the
	// STORED row and never from the request — see the note on the function below.
	Sua []NhiemVuVanBan

	// Xoa are stored lines the request did not name. They are SOFT deleted; the row and its number
	// stay.
	Xoa []NhiemVuVanBan
}

// CoGiDoi reports whether this block would change anything at all.
//
// THE CALLER USES IT TO WRITE NOTHING when the answer is no — no statement and no audit entry. A
// PATCH that re-sent the same three lines unchanged must not file an entry in an append-only ledger
// saying an act happened that changed nothing, because an inspection counting acts would count it.
func (t ThayDoiVanBan) CoGiDoi() bool {
	return len(t.Them) > 0 || len(t.Sua) > 0 || len(t.Xoa) > 0
}

// SoSanhVanBan decides what a request does to the lines already stored. THIS IS THE RULE OF §7.2'S
// DYNAMIC LIST, and it is a pure function so it can be proved without a database.
//
// # `thu_tu` OF AN EXISTING LINE IS READ FROM THE STORED ROW AND NEVER FROM THE ARRAY POSITION
//
// That single sentence is the whole correctness of this function, and getting it wrong is the
// reflex rather than the exception — "the client sent the list in display order, so renumber from
// the index" runs, looks right on screen, and is wrong twice over:
//
//   - IT DESTROYS A RECORDED FACT. `thu_tu` says where the clerk put the line. Derived from the
//     last request, it makes a client that sends the array in a different order silently REORDER a
//     list on an administrative record, with nothing red anywhere.
//   - IT COLLIDES WITH THE UNIQUE KEY, in the commonest case of all. `UNIQUE (tenant_id,
//     nhiem_vu_id, nhom, thu_tu)` counts SOFT-DELETED rows (migration 0009 says why), so removing
//     line ② and renumbering the survivors 1,2 reissues a number the removed row still holds.
//
// A gap after a removal is the CORRECT outcome, exactly as it is for `ket_luan_hop`'s ordinals.
//
// # `nhom` OF AN EXISTING LINE IS ALSO THE STORED ONE, AND A REQUEST THAT DISAGREES IS REFUSED
//
// Not silently corrected: a client sending a different group either has stale data or is trying an
// act nobody specified, and both deserve a sentence rather than a line that stays where it was while
// the screen shows it somewhere else. An EMPTY group on an existing line means "not stated" and is
// accepted — it is the shape a client sends when it is only correcting the text.
//
// # THE ORDER OF EVERY RETURNED SLICE IS DETERMINISTIC
//
// `Them` follows the request; `Sua` and `Xoa` follow the order of `dangCo`, which the store returns
// sorted. A function that reshuffled them would make one audit delta read as two different acts
// across two identical requests.
func SoSanhVanBan(dangCo []NhiemVuVanBan, yeuCau []VanBanNhiemVuVao) (ThayDoiVanBan, error) {
	var ra ThayDoiVanBan

	theoID := make(map[string]NhiemVuVanBan, len(dangCo))
	for _, c := range dangCo {
		theoID[c.ID] = c
	}

	// giu records which stored lines the request named. Everything not in it is removed — that is
	// how `✕` reaches the server: the line simply stops being sent.
	giu := make(map[string]bool, len(yeuCau))

	for _, v := range yeuCau {
		if v.ID == "" {
			ra.Them = append(ra.Them, v)
			continue
		}
		cu, co := theoID[v.ID]
		if !co {
			// NOT created under the id the client chose. A line id names a row of THIS task, and
			// accepting an unknown one lets a client pick internal ids.
			return ThayDoiVanBan{}, ErrVanBanKhongThuocNhiemVu
		}
		if giu[v.ID] {
			return ThayDoiVanBan{}, ErrVanBanTrungTrongYeuCau
		}
		giu[v.ID] = true

		if v.Nhom != "" && v.Nhom != cu.Nhom {
			return ThayDoiVanBan{}, ErrDoiNhomVanBan
		}

		// THE STORED ROW IS THE BASE AND ONLY THE THREE CONTENT FIELDS ARE FOLDED ONTO IT. `ID`,
		// `NhiemVuID`, `Nhom` and `ThuTu` therefore cannot be moved by any request, because there is
		// no assignment here that could move them.
		moi := cu
		moi.SoKyHieu = v.SoKyHieu
		moi.NgayVanBan = v.NgayVanBan
		moi.TrichYeu = v.TrichYeu

		if moi.SoKyHieu != cu.SoKyHieu ||
			!moi.NgayVanBan.Equal(cu.NgayVanBan) ||
			moi.TrichYeu != cu.TrichYeu {
			ra.Sua = append(ra.Sua, moi)
		}
	}

	for _, c := range dangCo {
		if !giu[c.ID] {
			ra.Xoa = append(ra.Xoa, c)
		}
	}
	return ra, nil
}

// ThuTuVanBanTiepTheo mints the next position in one group from the highest already issued.
//
// A FUNCTION RATHER THAN `+ 1` AT THE CALL SITE, for the reason `MaNhiemVuTiepTheo` is one: the
// numbering rule of §7.2 ("thêm văn bản" appends) has exactly one expression, and the store's
// high-water mark deliberately counts soft-deleted rows so this can never hand back a number a
// removed line still holds.
//
// ZERO MEANS THE GROUP IS EMPTY, which this turns into 1 — the first number the schema's
// `nhiem_vu_van_ban_thu_tu_tu_mot` admits.
func ThuTuVanBanTiepTheo(lonNhat int) int { return lonNhat + 1 }
