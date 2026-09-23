package domain

// The WRITE rules of the task register — `docs/ui-ux/02-nhiem-vu.md` §5, §6, §7, and the four
// decisions ADR 0037 made about the sub-task tree.
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). Every
// rule below is a pure function over values, precisely so the expensive ones — the recursive
// completion check and the extension-approval rule of ADR 0038 — can be proved without a database,
// without a network and without a commune's configuration. There is no PostgreSQL reachable from
// this build environment, so a rule that could only be tested against one would not be tested.
//
// # WHAT IS DELIBERATELY NOT HERE
//
// The TRAVERSAL of the tree. Walking from a task to its descendants needs the store, so the loop
// lives in internal/app, inside the transaction that holds the rows. What lives here is the
// VERDICT over a set of rows already read — which is the half that carries the rule.

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// --- bounds ---------------------------------------------------------------------------------

const (
	// TieuDeNhiemVuToiDa bounds "Nội dung nhiệm vụ / Trích yếu văn bản" (§7.2). Long enough for a
	// document's subject line quoted in full, short enough that the column is not a document store.
	TieuDeNhiemVuToiDa = 500

	// MoTaNhiemVuToiDa bounds the free-text description of §7.1.
	MoTaNhiemVuToiDa = 5000

	// MaNhiemVuToiDa bounds the issued number — `NV19`. A register number a clerk types.
	MaNhiemVuToiDa = 32

	// LyDoNhiemVuToiDa bounds the mandatory reasons: the delete reason (rule 7, invariant 1) and
	// the extension reason (§5.8).
	LyDoNhiemVuToiDa = 1000

	// NoiDungNhatKyToiDa bounds one progress entry (§5.9).
	NoiDungNhatKyToiDa = 5000

	// TomTatKetQuaToiDa and GhiChuNhiemVuToiDa bound the two textareas of §5.4.
	TomTatKetQuaToiDa  = 5000
	GhiChuNhiemVuToiDa = 5000
)

// TienDoToiDa is the ceiling of "% tiến độ ghi nhận" (§5.3). The same 0–100 the schema's
// `nhiem_vu_tien_do_hop_le` CHECK admits — stated twice on purpose: the database is the floor and
// this layer is the sentence, and a drift between the two is a worse error message, never a hole.
const TienDoToiDa = 100

// --- refusals of what the client sent ----------------------------------------------------------

var (
	ErrThieuTieuDeNhiemVu  = errors.New("nhiệm vụ: thiếu nội dung nhiệm vụ")
	ErrTieuDeNhiemVuQuaDai = fmt.Errorf(
		"nhiệm vụ: nội dung nhiệm vụ quá dài (tối đa %d ký tự)", TieuDeNhiemVuToiDa)
	ErrMoTaNhiemVuQuaDai = fmt.Errorf(
		"nhiệm vụ: mô tả quá dài (tối đa %d ký tự)", MoTaNhiemVuToiDa)

	ErrThieuLoaiNhiemVu = errors.New(
		"nhiệm vụ: thiếu loại nhiệm vụ — biểu mẫu bắt buộc chọn, và danh mục loại nhiệm vụ của xã " +
			"phải có mã này")

	// ErrNguonGiaoKhongHopLe refuses a source of work that is not one of §3's four.
	//
	// A SEPARATE SENTINEL FROM ErrThieuLoaiNhiemVu, although both are "a code the form sent is not
	// usable": they name different boxes on the form, and one sentence covering both would leave the
	// person looking at the wrong one. The four codes are CLOSED — each names a different
	// originating register, so a fifth is a new integration rather than a new label.
	ErrNguonGiaoKhongHopLe = errors.New(
		"nhiệm vụ: nguồn giao việc không phải một trong bốn nguồn của sổ nhiệm vụ")

	ErrMaNhiemVuSaiDinhDang = errors.New(
		"nhiệm vụ: mã nhiệm vụ chỉ nhận chữ in hoa, chữ số, dấu gạch ngang và dấu gạch dưới")
	ErrMaNhiemVuQuaDai = fmt.Errorf(
		"nhiệm vụ: mã nhiệm vụ quá dài (tối đa %d ký tự)", MaNhiemVuToiDa)

	ErrTienDoNgoaiKhoang = fmt.Errorf(
		"nhiệm vụ: tiến độ phải nằm trong khoảng 0–%d", TienDoToiDa)

	ErrThieuLyDoXoaNhiemVu = errors.New(
		"nhiệm vụ: thiếu lý do xoá — sổ nhiệm vụ là hồ sơ lưu trữ, xoá mềm phải ghi ai xoá và vì sao")
	ErrLyDoNhiemVuQuaDai = fmt.Errorf(
		"nhiệm vụ: lý do quá dài (tối đa %d ký tự)", LyDoNhiemVuToiDa)

	ErrThieuNoiDungNhatKy  = errors.New("nhiệm vụ: thiếu nội dung nhật ký")
	ErrNoiDungNhatKyQuaDai = fmt.Errorf(
		"nhiệm vụ: nội dung nhật ký quá dài (tối đa %d ký tự)", NoiDungNhatKyToiDa)

	ErrTomTatKetQuaQuaDai = fmt.Errorf(
		"nhiệm vụ: tóm tắt kết quả quá dài (tối đa %d ký tự)", TomTatKetQuaToiDa)
	ErrGhiChuNhiemVuQuaDai = fmt.Errorf(
		"nhiệm vụ: ghi chú quá dài (tối đa %d ký tự)", GhiChuNhiemVuToiDa)
)

// LaLoiDauVaoNhiemVu reports whether this is a refusal of WHAT THE CLIENT SENT, as opposed to a
// failure or a refusal about the state of the record.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, the same discipline laLoiDauVao uses for the
// catalogues: a default of "anything I do not recognise is the client's fault" turns a database
// outage into a 400, and a client that believes its input is wrong retries with different input
// for ever while nobody is told the server is broken.
func LaLoiDauVaoNhiemVu(err error) bool {
	for _, mot := range []error{
		ErrThieuTieuDeNhiemVu, ErrTieuDeNhiemVuQuaDai, ErrMoTaNhiemVuQuaDai,
		ErrThieuLoaiNhiemVu, ErrNguonGiaoKhongHopLe,
		ErrMaNhiemVuSaiDinhDang, ErrMaNhiemVuQuaDai,
		ErrTienDoNgoaiKhoang,
		ErrThieuLyDoXoaNhiemVu, ErrLyDoNhiemVuQuaDai,
		ErrThieuNoiDungNhatKy, ErrNoiDungNhatKyQuaDai,
		ErrTomTatKetQuaQuaDai, ErrGhiChuNhiemVuQuaDai,
		ErrTrangThaiNhiemVuKhongBiet, ErrThieuLyDoLuiHan, ErrHanMoiKhongLui,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}

// --- field checks ------------------------------------------------------------------------------

// KiemTieuDeNhiemVu trims and bounds the one mandatory text of the form.
//
// MEASURED IN RUNES AND NOT BYTES, here and in every check below. Vietnamese is three bytes per
// accented character in UTF-8, so a byte bound would cut a Vietnamese title at a third of the
// length it cuts an English one — and the person who hits it has no way to tell why.
func KiemTieuDeNhiemVu(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuTieuDeNhiemVu
	case len([]rune(s)) > TieuDeNhiemVuToiDa:
		return "", ErrTieuDeNhiemVuQuaDai
	}
	return s, nil
}

// KiemVanBanTuyChon trims and bounds an OPTIONAL free-text field, returning the caller's own
// sentinel when it is too long.
//
// ONE FUNCTION FOR THE FOUR OPTIONAL TEXTS, with the error passed in rather than derived: each
// field names itself in its own sentence, and a shared "văn bản quá dài" would leave the person
// on the form guessing which box to shorten.
func KiemVanBanTuyChon(s string, tran int, quaDai error) (string, error) {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > tran {
		return "", quaDai
	}
	return s, nil
}

// KiemMaNhiemVu checks a code the clerk typed instead of letting the register mint one (§7.1's
// `Tự sinh mã` checkbox, unticked).
//
// THE CHARACTER SET IS NARROW ON PURPOSE. This code is printed in the Sổ theo dõi, quoted in
// meeting minutes and carried in a URL path segment (`GET /api/v1/tasks/{ma}`), so a space, a
// slash or a Vietnamese diacritic in it is a number that cannot be typed back, cannot be searched
// for reliably, and reaches the register as a different string depending on who encoded it.
//
// IT IS NEVER REISSUED, so this check is the LAST moment the value can be refused: migration
// 0006's trigger refuses every later change to `ma`, and `UNIQUE (tenant_id, ma)` counts
// soft-deleted rows.
func KiemMaNhiemVu(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrMaNhiemVuSaiDinhDang
	}
	if len([]rune(s)) > MaNhiemVuToiDa {
		return "", ErrMaNhiemVuQuaDai
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return "", ErrMaNhiemVuSaiDinhDang
		}
	}
	return s, nil
}

// KiemTienDo bounds "% tiến độ ghi nhận".
func KiemTienDo(n int) error {
	if n < 0 || n > TienDoToiDa {
		return ErrTienDoNgoaiKhoang
	}
	return nil
}

// KiemLyDoXoaNhiemVu refuses a soft delete that records no reason (rule 7, invariant 1).
func KiemLyDoXoaNhiemVu(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuLyDoXoaNhiemVu
	case len([]rune(s)) > LyDoNhiemVuToiDa:
		return "", ErrLyDoNhiemVuQuaDai
	}
	return s, nil
}

// --- minting the register number -----------------------------------------------------------------

// TienToMaNhiemVu is the series §7.1 names: `NV01, NV02…`, minted PER COMMUNE.
const TienToMaNhiemVu = "NV"

// SoTrongMaNhiemVu reads the number out of a minted code, and says so when the code is not one of
// the minted ones.
//
// A COMMUNE MAY TYPE ITS OWN CODES (§7.1 offers the checkbox), and those are not in the series.
// `KH-2026-07` is a perfectly good task number and contributes nothing to "what is the next NV".
func SoTrongMaNhiemVu(ma string) (int, bool) {
	if !strings.HasPrefix(ma, TienToMaNhiemVu) {
		return 0, false
	}
	so := ma[len(TienToMaNhiemVu):]
	if so == "" {
		return 0, false
	}
	n := 0
	for _, r := range so {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			// Not a series number any commune produced. Refusing beats overflowing into a negative.
			return 0, false
		}
	}
	return n, true
}

// MaNhiemVuTiepTheo mints the next code of the series from the highest one already issued.
//
// ZERO-PADDED TO TWO DIGITS, WHICH IS §7.1'S OWN SPELLING (`NV01, NV02…`), AND THE PADDING STOPS
// THERE. `NV100` follows `NV99` — widening the pad to three would renumber nothing (the issued
// codes are immutable) but would produce `NV099` beside `NV99`, two spellings of one position in
// one register.
//
// ⚠ IT IS SEQUENTIAL AND THAT IS CORRECT HERE, which is worth saying because the neighbouring
// register forbids exactly that: `phieu_phan_anh.ma_tra_cuu` is handed to a CITIZEN and sits on a
// lookup path, so a guessable code reads other people's petitions (rule 4, invariant 4). A task
// number never leaves the staff surface.
func MaNhiemVuTiepTheo(soLonNhat int) string {
	return fmt.Sprintf("%s%02d", TienToMaNhiemVu, soLonNhat+1)
}

// --- the tree (ADR 0037) ---------------------------------------------------------------------------

// NhiemVuTomTat is the narrow shape a tree walk reads: enough to decide the two tree rules and to
// NAME the rows in the refusal, and nothing else.
//
// IT CARRIES THE CODE AND NOT ONLY THE COUNT, because ADR 0037 decision 4 asks the refusal to
// "liệt kê việc con còn lại". A message saying "3 việc con chưa xong" sends an officer hunting; one
// saying which three is a message they can act on. The code is a staff-facing register number and
// carries no personal data (rule 3).
type NhiemVuTomTat struct {
	ID        string
	Ma        string
	TrangThai TrangThaiNhiemVu
}

// TranDuyetCayNhiemVu bounds ONE tree walk — total nodes visited.
//
// # IT IS A SAFETY NET AND NOT A BUSINESS LIMIT, AND THE DIFFERENCE MATTERS
//
// ADR 0037 decision 1 is "nhiều tầng, KHÔNG GIỚI HẠN", and this is not a depth limit smuggled back
// in: a real commune's task tree is tens of rows, and 5000 is three orders of magnitude past any
// of them. What it bounds is the pathological case — a cycle that slipped past the write path, or
// an import that built something nobody meant — so that the failure is a refusal a person reads
// rather than a request that never returns while holding row locks on a government register.
const TranDuyetCayNhiemVu = 5000

// TranBacCayNhiemVu bounds the walk UPWARD when a parent is being set: how many ancestors are
// followed before the attempt is refused.
//
// SMALLER THAN THE DOWNWARD BOUND ON PURPOSE. Downward a walk legitimately visits every descendant;
// upward it visits one chain, and a chain 200 deep is not an administrative structure — it is the
// shape a cycle makes when the visited-set check has been removed.
const TranBacCayNhiemVu = 200

var (
	// ErrChuTrinhCayNhiemVu is THE FIFTH RULE — the one that is in no specification.
	//
	// `A → B → C → A` is representable because the tree has no depth limit (ADR 0037 decision 1),
	// and migration 0008's `CHECK (nhiem_vu_cha_id IS DISTINCT FROM id)` only stops a row being its
	// own parent: a CHECK sees one row and cannot see the row above it. A cycle makes EVERY tree
	// walk run for ever — including the completion check of decision 4 — so it is refused at the
	// write path, which is the only place that can walk upward inside the transaction.
	ErrChuTrinhCayNhiemVu = errors.New(
		"nhiệm vụ: không đặt được nhiệm vụ cha — nhiệm vụ cha nằm trong nhánh con của chính " +
			"nhiệm vụ này, và một vòng như thế làm mọi phép duyệt cây chạy vĩnh viễn")

	// ErrCayNhiemVuQuaLon is the safety net firing. It is NOT "too many sub-tasks are allowed": see
	// TranDuyetCayNhiemVu.
	ErrCayNhiemVuQuaLon = fmt.Errorf(
		"nhiệm vụ: cây nhiệm vụ con vượt quá %d bản ghi nên không duyệt hết được — nhiều khả năng "+
			"dữ liệu có vòng lặp, hãy báo quản trị", TranDuyetCayNhiemVu)

	// ErrConChuaXoa refuses the soft delete of a task that still has live children (ADR 0037
	// decision 3). LoiConChuaXoa wraps it with the count the decision asks for.
	ErrConChuaXoa = errors.New("nhiệm vụ: còn nhiệm vụ con chưa xoá")

	// ErrConChuaXong refuses `hoan-thanh` on a task whose tree still holds unfinished work (ADR
	// 0037 decision 4). LoiConChuaXong wraps it with the codes.
	ErrConChuaXong = errors.New("nhiệm vụ: còn nhiệm vụ con chưa hoàn thành")

	// ErrChaKhongTonTai means the parent named on the request is not a live task of this commune.
	ErrChaKhongTonTai = errors.New(
		"nhiệm vụ: không tìm thấy nhiệm vụ cha trong sổ nhiệm vụ của xã")
)

// LoiConChuaXoa is decision 3's refusal, and it says HOW MANY.
//
// THE COUNT IS THE DECISION'S OWN WORDING — "TỪ CHỐI, kèm câu nói rõ còn mấy việc con". Without it
// the officer is told they may not delete and has no idea what is in the way; with it they know
// exactly how much work stands between them and the act.
func LoiConChuaXoa(n int) error {
	return fmt.Errorf("%w: còn %d việc con chưa xoá — xử lý hoặc xoá các việc con trước", ErrConChuaXoa, n)
}

// LoiConChuaXong is decision 4's refusal, and it LISTS the unfinished work.
//
// THE LIST IS BOUNDED AT MuoiMaDauTien codes so one message cannot become a page of register
// numbers; the count is always exact, so nothing is hidden by the bound.
func LoiConChuaXong(ma []string) error {
	hien := ma
	if len(hien) > MuoiMaDauTien {
		hien = hien[:MuoiMaDauTien]
		return fmt.Errorf("%w: còn %d việc con (%s, …) — hoàn thành hết việc con rồi mới hoàn thành việc cha",
			ErrConChuaXong, len(ma), strings.Join(hien, ", "))
	}
	return fmt.Errorf("%w: còn %d việc con (%s) — hoàn thành hết việc con rồi mới hoàn thành việc cha",
		ErrConChuaXong, len(ma), strings.Join(hien, ", "))
}

// MuoiMaDauTien bounds how many register numbers one refusal quotes.
const MuoiMaDauTien = 10

// ConChuaXong is DECISION 4's VERDICT over a tree that has already been read.
//
// # WHAT COUNTS AS "XONG" IS `hoan-thanh` AND NOTHING ELSE, AND THAT HAS A COST WORTH STATING
//
// §11.4 says it in the specification's own words: "Nhiệm vụ cha hoàn thành chỉ khi toàn bộ nhiệm vụ
// con đã HOÀN THÀNH". So `chuyen-tiep` — the other terminal status — does NOT satisfy it, and a
// parent with a child that was forwarded to another department cannot be completed here.
//
// THAT IS FAIL-CLOSED AND IT IS REPORTED RATHER THAN SOFTENED. §6 says a forwarded task "sinh bản
// ghi liên kết", and NO COLUMN LINKS THE TWO ROWS (migration 0006 states the absence, and this pass
// did not invent one) — so this service cannot tell whether the forwarded work was ever finished
// somewhere else. Treating `chuyen-tiep` as done would let a parent be completed on the strength of
// a link nobody can follow, and the completion figure that reaches leadership would count it.
//
// THE ORDER OF THE RETURNED CODES IS THE ORDER THE CALLER READ THE TREE IN, so a refusal listing
// three codes lists the same three every time — a message that reshuffles itself reads as two
// different problems.
func ConChuaXong(con []NhiemVuTomTat) []string {
	var chua []string
	for _, c := range con {
		if c.TrangThai != HoanThanh {
			chua = append(chua, c.Ma)
		}
	}
	return chua
}

// --- the lifecycle, as the write path needs it -------------------------------------------------

// ErrTrangThaiNhiemVuKhongBiet refuses a target status that is not one of the seven.
//
// FAIL CLOSED: an unknown code can only come from a caller that made one up, and a task allowed to
// land in a state the lifecycle map does not have is work that stops moving, silently, while a
// deadline keeps running against it.
var ErrTrangThaiNhiemVuKhongBiet = errors.New(
	"nhiệm vụ: trạng thái không phải một trong bảy trạng thái của nhiệm vụ")

// ErrChuyenTrangThaiNhiemVuSaiLuc refuses a move the lifecycle of §6 does not have.
var ErrChuyenTrangThaiNhiemVuSaiLuc = errors.New(
	"nhiệm vụ: vòng đời không có bước chuyển này từ trạng thái hiện tại")

// ErrTiepTucKhongBietTrangThaiTruoc refuses resuming a paused task when the log cannot say what it
// was paused FROM.
//
// # WHY THIS IS A REFUSAL AND NOT A DEFAULT
//
// §6 says a paused task resumes into "(trạng thái TRƯỚC)". There is no `trang_thai_truoc` column
// and there must not be one — the fact is already in `nhat_ky_nhiem_vu`, and a second copy is what
// rule 9's one-line test forbids. When the log holds no state before the pause, the honest answer
// is that this task cannot be resumed automatically; picking `dang-thuc-hien` "because it is the
// usual one" would move work into a state nobody put it in, and the timeline would then say so.
var ErrTiepTucKhongBietTrangThaiTruoc = errors.New(
	"nhiệm vụ: nhật ký không ghi trạng thái trước lần tạm dừng nên chưa xác định được bước tiếp tục")

// MocNhatKy is one timeline row as the resume rule needs it: the state the task was in at that
// moment, newest first.
//
// A NARROWER TYPE THAN NhatKyNhiemVu ON PURPOSE. The resume rule needs ONE column, and a function
// taking the whole row would be a function a later edit could make depend on the author, the
// department or the text — none of which decides where a paused task resumes to.
type MocNhatKy struct {
	TrangThai TrangThaiNhiemVu
}

// NhatKyNhiemVu is one entry of "Nhật ký & Trao đổi" (§5.9) — a BUSINESS record the drawer renders,
// written in the same transaction as the act it describes.
//
// IT IS NOT THE AUDIT TRAIL AND DOES NOT REPLACE IT (migration 0006 says so at length): `audit_log`
// answers "who changed what" for the whole service and is invisible to a commune, while this is the
// timeline an officer reads and writes. Both are written, for the same act, in one transaction.
//
// APPEND-ONLY — migration 0006 enforces it with a trigger, so there is no update path and no
// soft-delete column here. To correct an entry, write another entry carrying the correction.
type NhatKyNhiemVu struct {
	ID        string
	NhiemVuID string

	// NguoiMa is the author, as a STAFF BUSINESS CODE (rule 6, invariant 8).
	NguoiMa string

	ThoiDiem time.Time

	// TrangThaiTaiThoiDiem is the state the task was in AT THIS MOMENT — the chip §5.9 draws on the
	// timeline row. STORED rather than derived: the task's current state is one value, and the
	// timeline needs the state at each step.
	//
	// THE CONVENTION IS "THE STATE THE ACT LANDED IT IN", and it is what TrangThaiTruocTamDung
	// reads. Writing the state BEFORE the act instead would make every resume read one row too far
	// back, which no test of a single transition could show.
	TrangThaiTaiThoiDiem TrangThaiNhiemVu

	// Who was holding the task at this step. Both empty when this entry changed no assignment —
	// §5.9 renders them only "khi có thay đổi phân công".
	BoPhanID        string
	NguoiPhuTrachMa string

	NoiDung string
}

// KiemNoiDungNhatKy trims and bounds one entry. MANDATORY: the schema's
// `nhat_ky_nhiem_vu_noi_dung_khong_rong` refuses an empty one, and an entry with nothing in it is a
// row nobody can act on in a table that is never edited afterwards.
func KiemNoiDungNhatKy(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuNoiDungNhatKy
	case len([]rune(s)) > NoiDungNhatKyToiDa:
		return "", ErrNoiDungNhatKyQuaDai
	}
	return s, nil
}

// NoiDungChuyenTrangThai is the entry written when an officer moves a task and types nothing.
//
// # WHY A SENTENCE IS GENERATED RATHER THAN THE ENTRY SKIPPED
//
// §6's lifecycle and §5.9's timeline are the same story told twice, and a status change with no
// timeline row leaves a gap an officer reads as "nothing happened here". The schema also refuses an
// empty `noi_dung`, so "write the row with no text" is not available either.
//
// IT QUOTES THE CODES AND NOT THE LABELS. The LABELS belong to the commune (open question #21, ADR
// 0035 §C) and a copy of a display string frozen into a timeline row would still be there after the
// commune re-worded it — two spellings of one status in one drawer.
func NoiDungChuyenTrangThai(tu, sang TrangThaiNhiemVu) string {
	return fmt.Sprintf("Chuyển trạng thái: %s → %s", tu, sang)
}

// TrangThaiTruocTamDung derives §6's "(trạng thái trước)" from the timeline.
//
// # THE DERIVATION, AND WHY IT IS A FUNCTION OVER ROWS RATHER THAN A QUERY
//
// `nhat_ky` arrives NEWEST FIRST. The rows at the head are the pause itself (and any entry written
// while paused, which carries `tam-dung` too); the first row that is NOT `tam-dung` is the state
// the task was paused from. Written here, the rule is testable against a handful of rows; written
// as SQL it would be testable only against a PostgreSQL this build environment does not have.
//
// THE ANSWER IS STILL CHECKED AGAINST TamDungVeDuoc by the caller: a timeline can hold
// `hoan-thanh` (a task completed, reopened by an administrator in some future pass), and resuming a
// pause into `hoan-thanh` would be finished work nobody did.
func TrangThaiTruocTamDung(nhatKy []MocNhatKy) (TrangThaiNhiemVu, bool) {
	for _, m := range nhatKy {
		if m.TrangThai == TamDung {
			continue
		}
		return m.TrangThai, true
	}
	return "", false
}

// ChuyenTrangThaiDuoc is the whole of §6's shape check for ONE move.
//
// `truocTamDung` IS ONLY CONSULTED WHEN THE TASK IS PAUSED, and the caller passes "" when it is
// not. That asymmetry is the rule itself: from every other status the map decides alone, while from
// `tam-dung` the legal target is a fact about THIS TASK'S HISTORY that no map can hold — see
// TrangThaiNhiemVu.ChuyenSangDuoc, which is deliberately loose for exactly this case.
func ChuyenTrangThaiDuoc(hienTai, moi, truocTamDung TrangThaiNhiemVu) error {
	if !moi.HopLe() {
		return ErrTrangThaiNhiemVuKhongBiet
	}
	if hienTai == TamDung {
		if truocTamDung == "" {
			return ErrTiepTucKhongBietTrangThaiTruoc
		}
		if !TamDungVeDuoc(truocTamDung) || moi != truocTamDung {
			return fmt.Errorf("%w: việc đang tạm dừng chỉ tiếp tục về đúng trạng thái trước đó",
				ErrChuyenTrangThaiNhiemVuSaiLuc)
		}
		return nil
	}
	if !hienTai.ChuyenSangDuoc(moi) {
		return ErrChuyenTrangThaiNhiemVuSaiLuc
	}
	return nil
}

// --- the extension request (§5.8, ADR 0038) -------------------------------------------------------

// TrangThaiDeNghi is where an extension request stands. THREE CODES, closed, and they are the
// three the schema's `de_nghi_lui_han_trang_thai_hop_le` CHECK admits.
type TrangThaiDeNghi string

const (
	ChoDuyetLuiHan TrangThaiDeNghi = "cho-duyet"
	DaDuyetLuiHan  TrangThaiDeNghi = "da-duyet"
	TuChoiLuiHan   TrangThaiDeNghi = "tu-choi"
)

func (t TrangThaiDeNghi) HopLe() bool {
	switch t {
	case ChoDuyetLuiHan, DaDuyetLuiHan, TuChoiLuiHan:
		return true
	}
	return false
}

// DeNghiLuiHan is one request to move a task's deadline, and the decision on it.
//
// `HanMoi` IS WHAT WAS ASKED FOR AND NEVER WHAT WAS GRANTED. Approving it writes `nhiem_vu.
// han_xu_ly` and leaves this row's own figure untouched — migration 0006's
// `de_nghi_lui_han_bat_bien` trigger refuses to change it afterwards, so the record of what a
// leader actually approved cannot be rewritten later.
type DeNghiLuiHan struct {
	ID        string
	NhiemVuID string

	// The two people, as STAFF BUSINESS CODES (`CB-00123`) — rule 6, invariant 8. NguoiDuyetMa is
	// empty until somebody decides.
	NguoiDeNghiMa string
	NguoiDuyetMa  string

	HanMoi time.Time
	LyDo   string

	TrangThai TrangThaiDeNghi

	ThoiDiem time.Time
	DuyetLuc time.Time
}

var (
	ErrThieuLyDoLuiHan = errors.New(
		"nhiệm vụ: thiếu lý do lùi hạn — lãnh đạo giao việc không quyết được một đề nghị không nói vì sao")

	// ErrNhiemVuChuaCoHan refuses an extension request against a task that has no deadline.
	//
	// # IT IS A SCHEMA FACT, NOT A PREFERENCE
	//
	// `nhiem_vu_hai_han_cung_co_cung_khong` requires `han_xu_ly` and `han_ban_dau` to be set or
	// absent TOGETHER, and `han_ban_dau` may never be written after creation (the
	// `nhiem_vu_bat_bien` trigger). So approving an extension on a task with no deadline could only
	// produce a row the database refuses. Saying it here, before anything is written, turns a
	// constraint error into a sentence naming what is missing.
	ErrNhiemVuChuaCoHan = errors.New(
		"nhiệm vụ: nhiệm vụ này chưa có hạn xử lý nên không có gì để lùi — hạn chỉ ấn định được lúc tạo việc")

	// ErrHanMoiKhongLui refuses a "lùi hạn" that does not move the deadline later.
	//
	// SHORTENING A COMMITMENT IS A DIFFERENT ACT and nobody has specified it: §5.8 is headed "Đề
	// nghị lùi hạn" and promises "Hạn gốc vẫn được giữ lại". A request that pulls the date forward
	// would travel through the same approval and move `han_xu_ly` backwards, which no screen would
	// label as what it is.
	ErrHanMoiKhongLui = errors.New(
		"nhiệm vụ: hạn mới phải muộn hơn hạn hiện tại — đây là đề nghị LÙI hạn")

	// ErrDaCoDeNghiChoDuyet mirrors `UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)` in a sentence.
	ErrDaCoDeNghiChoDuyet = errors.New(
		"nhiệm vụ: đã có một đề nghị lùi hạn đang chờ duyệt cho nhiệm vụ này")

	// ErrDeNghiDaQuyetDinh refuses a second decision on one request.
	ErrDeNghiDaQuyetDinh = errors.New(
		"nhiệm vụ: đề nghị lùi hạn này đã được quyết định rồi")

	// ErrChuaGhiLanhDaoGiaoViec is ADR 0038's OPEN QUESTION, failing CLOSED.
	//
	// The ADR says it in as many words: `lanh_dao_giao_viec_ma` is nullable, a task with nobody
	// named has no approver, and until the owner decides otherwise the answer is to REFUSE with a
	// sentence naming what is missing. It deliberately does NOT fall back to `nguoi_tao_ma` — the
	// clerk who typed the row on somebody else's behalf is precisely the person who must not decide
	// it, which is why migration 0006 put the two codes in two columns.
	ErrChuaGhiLanhDaoGiaoViec = errors.New(
		"nhiệm vụ: nhiệm vụ này chưa ghi lãnh đạo giao việc nên chưa ai duyệt được đề nghị lùi hạn — " +
			"hãy bổ sung lãnh đạo giao việc cho nhiệm vụ")

	// ErrKhongPhaiLanhDaoGiaoViec is ADR 0038's SECOND LAYER — the one the permission key cannot
	// express, because rule 5 checks `(tenant_id, role, permission)` and has no "which record"
	// dimension.
	ErrKhongPhaiLanhDaoGiaoViec = errors.New(
		"nhiệm vụ: chỉ lãnh đạo giao việc ghi trên nhiệm vụ này mới duyệt được đề nghị lùi hạn")

	// ErrTuDuyetDeNghiCuaMinh is the independent second rule of ADR 0038.
	ErrTuDuyetDeNghiCuaMinh = errors.New(
		"nhiệm vụ: không ai duyệt được đề nghị lùi hạn của chính mình")
)

// KiemLyDoLuiHan trims and bounds §5.8's mandatory reason.
func KiemLyDoLuiHan(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuLyDoLuiHan
	case len([]rune(s)) > LyDoNhiemVuToiDa:
		return "", ErrLyDoNhiemVuQuaDai
	}
	return s, nil
}

// DuocDeNghiLuiHan decides whether THIS task can carry an extension request at all.
//
// IT DOES NOT ASK WHO IS ASKING. §5.8 puts the box on the drawer of the officer doing the work, and
// the route's key (`task.update`) is what decides whether this account may touch the task's
// progress at all. Narrowing it further to "the named assignee" would silence the ordinary case the
// specification describes — a department holding a task with nobody named yet (§11.1) — and ADR
// 0038 decides the APPROVAL side, deliberately saying nothing about who may ask.
func DuocDeNghiLuiHan(n NhiemVu, hanMoi time.Time) error {
	if n.HanXuLy.IsZero() {
		return ErrNhiemVuChuaCoHan
	}
	if !hanMoi.After(n.HanXuLy) {
		return ErrHanMoiKhongLui
	}
	return nil
}

// DuocDuyetLuiHan IS ADR 0038 IN ONE FUNCTION — the layer a permission key cannot hold.
//
// # THE TWO LAYERS, AND WHY THIS ONE CANNOT BE THE ROUTE'S
//
//	task.extend                         may this ACCOUNT touch extensions at all   — the gate,
//	                                    authz.RequirePermission, in routes.go
//	Ma == n.LanhDaoGiaoViecMa           is this THAT PERSON'S task                 — here
//
// Drop the gate and anybody may call the route. Drop this and every leader holding the key approves
// every task in the commune, including those of a department they have nothing to do with. ADR 0038
// keeps both, in that order: the gate answers 403 to somebody unrelated, and this answers a business
// refusal to somebody who holds the right but was not the one who gave the work out.
//
// # THE COMPARISON IS BETWEEN TWO STAFF BUSINESS CODES, AND THAT IS THE WHOLE CORRECTNESS OF IT
//
// `lanh_dao_giao_viec_ma` holds `CB-…` (migration 0006 says so in the column name), and
// authz.Principal.Ma holds the same kind of value. Comparing an internal id against this column
// compares two DIFFERENT KINDS of identifier: it never matches, every leader falls into the "not
// the named leader" branch, and the feature simply does not work — with nothing red anywhere,
// because both values are non-empty strings that look plausible (rule 6, invariant 8; measured in
// this repository on 2026-09-22).
//
// # AN EMPTY CODE MATCHES NOBODY, AND BOTH EMPTIES ARE CHECKED EXPLICITLY
//
// Without the two guards below, `"" == ""` would be true and EVERY account holding `task.extend`
// could approve every extension on every task that has no leader named — the widest possible
// failure, produced by the narrowest possible omission.
func DuocDuyetLuiHan(n NhiemVu, dn DeNghiLuiHan, nguoiMa string) error {
	if dn.TrangThai != ChoDuyetLuiHan {
		return ErrDeNghiDaQuyetDinh
	}
	if nguoiMa == "" {
		// A principal with no business code cannot be compared with a column that holds one, and
		// rule 6 does not permit a business write whose trail cannot name who made it.
		return ErrKhongPhaiLanhDaoGiaoViec
	}
	if n.LanhDaoGiaoViecMa == "" {
		return ErrChuaGhiLanhDaoGiaoViec
	}
	if nguoiMa != n.LanhDaoGiaoViecMa {
		return ErrKhongPhaiLanhDaoGiaoViec
	}
	// THE SECOND RULE, INDEPENDENT OF THE FIRST. It still bites when the leader who gave the work
	// out is also the one who asked for more time: ADR 0038 keeps it precisely for that case, and
	// `../vigov-require` has the same rule (service.py:719-723).
	if dn.NguoiDeNghiMa == nguoiMa {
		return ErrTuDuyetDeNghiCuaMinh
	}
	return nil
}

// LaLoiThamQuyenLuiHan reports whether this refusal is about WHO is acting rather than about the
// state of the record. The HTTP layer answers 403 for these and 409 for the rest — see the note on
// the mapping, and ErrChuaGhiLanhDaoGiaoViec's own reason for being in this group.
func LaLoiThamQuyenLuiHan(err error) bool {
	return errors.Is(err, ErrKhongPhaiLanhDaoGiaoViec) ||
		errors.Is(err, ErrTuDuyetDeNghiCuaMinh) ||
		errors.Is(err, ErrChuaGhiLanhDaoGiaoViec)
}
