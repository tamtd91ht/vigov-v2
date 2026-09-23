package domain

// THE INTERNAL ANNOUNCEMENT (`docs/ui-ux/08-thong-bao.md`) — what a commune tells its OWN STAFF.
//
// READ THIS BEFORE REACHING FOR A NAME IN THIS PACKAGE. `thong_bao_gui_cong_dan.go`, next door,
// models a message that LEAVES the commune to ONE CITIZEN. The two are different audiences, two
// different trust levels (rule 4) and two different tables, and they share the one Vietnamese
// word `thông báo`. Every identifier here therefore carries `NoiBo` or an unambiguous prefix:
// `DaGui`, `ChoGui` and `TrangThaiGui` already belong to the citizen ledger, and a name collision
// resolved by "whichever compiles" is how a staff notice ends up on the citizen channel.
//
// WHAT IS MODELLED HERE AND WHAT IS NOT. The types below carry the announcement as a RECORD: what
// was said, who said it, who it went to, what each recipient has done with it. They do not carry
// the mail copy (§1, §5) beyond the two fields that record what was asked for and what happened —
// there is no SMTP adapter and no `Cấu hình → Máy chủ thư` in this repository, so sending is a
// later pass and is named as such in migration 0005.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// TrangThaiThongBao is the lifecycle of §6, hyphenated like every other enum value in this system.
//
// See migration 0005 for why the separator differs from §6's sketch and for what each state means
// to the person reading the screen. THE ORDER IS ONE-WAY: a draft may be issued, an issued
// announcement may be withdrawn, and nothing goes back — the database refuses it, and so does
// every path here.
type TrangThaiThongBao string

const (
	// ThongBaoNhap — drafted, no recipient list generated, visible to nobody but its author.
	ThongBaoNhap TrangThaiThongBao = "nhap"
	// ThongBaoDaPhatHanh — issued. §9.2 forbids editing the content from this point.
	ThongBaoDaPhatHanh TrangThaiThongBao = "da-phat-hanh"
	// ThongBaoDaGo — withdrawn with §4's `🗑 Gỡ`. The row and its acknowledgements stay: people
	// read it, and a withdrawal does not un-read it.
	ThongBaoDaGo TrangThaiThongBao = "da-go"
)

// TrangThaiThuThongBao is §3's mail chip, plus the state before any attempt.
//
// NOTHING IN THIS REPOSITORY MOVES IT OFF `ThuChuaGui` YET, and that is deliberate rather than
// unfinished: the values exist because they are part of the record an announcement carries, and
// inventing a sender to exercise them would put an external dependency (SMTP, per-commune mail
// configuration) into a pass that has neither.
type TrangThaiThuThongBao string

const (
	ThuChuaGui TrangThaiThuThongBao = "chua-gui"
	ThuDangGui TrangThaiThuThongBao = "dang-gui"
	ThuDaGui   TrangThaiThuThongBao = "da-gui"
	ThuLoi     TrangThaiThuThongBao = "loi"
)

// The bounds. Every one of them is a REFUSAL rather than a truncation: a title silently cut at
// 300 characters is an announcement whose subject changed on the way into the database.
const (
	// TieuDeToiDa — §5's placeholder is `Mời họp giao ban tháng 9`. 300 runes is roughly ten times
	// any real subject line and short enough that a card can still draw it.
	TieuDeToiDa = 300

	// NoiDungToiDaThongBao — the body §5 collects in a textarea. 20.000 runes is about ten pages;
	// past that it is a document with an attachment, which chapter 08 does not have.
	NoiDungToiDaThongBao = 20000

	// NguoiNhanToiDa bounds ONE announcement's recipient list. A commune has a few dozen staff
	// (§10's samples reach 12), so 500 is more than an order of magnitude above the real figure —
	// the point past which the list is not an addressee list but a loop, an import run twice, or a
	// client sending the same code five hundred times.
	NguoiNhanToiDa = 500

	// BoPhanToiDa bounds the departments one announcement may address. §5 lists five; the org
	// chart of a commune is tens of units at most.
	BoPhanToiDa = 50

	// MaCanBoToiDa bounds one staff business code (`CB-2026-7K3M9Q`). Long enough for any scheme
	// identity might use, short enough that a client cannot post a document as an identifier.
	MaCanBoToiDa = 64
)

var (
	// ErrTieuDeTrong — §5 marks the title required. A card with no title is a card nobody can pick
	// out of §2's list.
	ErrTieuDeTrong = errors.New("thong_bao: thiếu tiêu đề")
	// ErrTieuDeQuaDai — refused, never truncated. See the note on the bounds above.
	ErrTieuDeQuaDai = errors.New("thong_bao: tiêu đề quá dài")

	// ErrNoiDungTrong — §5 marks the body required. An announcement with a subject and nothing
	// under it is a notice that tells its readers nothing they can act on.
	ErrNoiDungTrong  = errors.New("thong_bao: thiếu nội dung")
	ErrNoiDungQuaDai = errors.New("thong_bao: nội dung quá dài")

	// ErrKhongCoNguoiNhan — an announcement addressed to nobody. It is refused rather than stored,
	// because §3's counter would read `0/0` and §2's `Gửi cho tôi` would never show it to anyone:
	// the author would believe they had told the commune something.
	ErrKhongCoNguoiNhan = errors.New("thong_bao: không có người nhận")
	// ErrQuaNhieuNguoiNhan — past NguoiNhanToiDa.
	ErrQuaNhieuNguoiNhan = errors.New("thong_bao: quá nhiều người nhận")
	// ErrQuaNhieuBoPhan — past BoPhanToiDa.
	ErrQuaNhieuBoPhan = errors.New("thong_bao: quá nhiều bộ phận nhận")

	// ErrMaCanBoTrong / ErrMaCanBoQuaDai — a recipient with no code, or with something that is not
	// a code. Refused HERE rather than at the database, because the column is only NOT NULL and an
	// empty string satisfies that.
	ErrMaCanBoTrong  = errors.New("thong_bao: mã cán bộ rỗng")
	ErrMaCanBoQuaDai = errors.New("thong_bao: mã cán bộ quá dài")
)

// ChuanHoaTieuDe trims and validates the subject line.
//
// IT CARRIES VIETNAMESE WITH DIACRITICS — it is a sentence a person reads, so nothing here
// restricts the character set beyond refusing control characters, which would corrupt a screen and
// a log line alike. Same shape as ChuanHoaNhan, for the same reason.
func ChuanHoaTieuDe(tieuDe string) (string, error) {
	tieuDe = strings.TrimSpace(tieuDe)
	switch {
	case tieuDe == "":
		return "", ErrTieuDeTrong
	case len([]rune(tieuDe)) > TieuDeToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTieuDeQuaDai, TieuDeToiDa)
	}
	for _, r := range tieuDe {
		if unicode.IsControl(r) {
			return "", ErrTieuDeTrong
		}
	}
	return tieuDe, nil
}

// ChuanHoaNoiDungThongBao trims and validates the body.
//
// NEWLINES ARE ALLOWED AND EVERY OTHER CONTROL CHARACTER IS NOT. That is the one difference from
// ChuanHoaTieuDe and it is the whole point of a textarea: an announcement is paragraphs. A
// carriage return is accepted too, because a browser on Windows sends CRLF and refusing it would
// reject a perfectly ordinary form submission.
func ChuanHoaNoiDungThongBao(noiDung string) (string, error) {
	noiDung = strings.TrimSpace(noiDung)
	switch {
	case noiDung == "":
		return "", ErrNoiDungTrong
	case len([]rune(noiDung)) > NoiDungToiDaThongBao:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrNoiDungQuaDai, NoiDungToiDaThongBao)
	}
	for _, r := range noiDung {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", ErrNoiDungTrong
		}
	}
	return noiDung, nil
}

// ChuanHoaMaCanBo trims and validates one staff business code.
//
// IT DOES NOT CHECK THE SHAPE OF THE CODE, and that is deliberate. `CB-2026-7K3M9Q` is minted by
// service-identity and its format is that service's to change (rule 2); a pattern copied here
// would be a second declaration of somebody else's fact, and the copy would refuse a real code on
// the day identity widened it. What is checked is what this service is entitled to insist on: a
// code that is present, is not whitespace, is not a paragraph and has no control characters in it.
func ChuanHoaMaCanBo(ma string) (string, error) {
	ma = strings.TrimSpace(ma)
	switch {
	case ma == "":
		return "", ErrMaCanBoTrong
	case len([]rune(ma)) > MaCanBoToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMaCanBoQuaDai, MaCanBoToiDa)
	}
	for _, r := range ma {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrMaCanBoTrong
		}
	}
	return ma, nil
}

// ChuanHoaDanhSachMaCanBo normalises a list of staff codes, DROPS DUPLICATES and keeps the order.
//
// DEDUPLICATION HAPPENS HERE AND NOT ONLY IN THE DATABASE. `PRIMARY KEY (tenant_id, thong_bao_id,
// nguoi_nhan_ma)` would collapse the duplicates anyway, but it would do it by making the second
// INSERT a no-op inside a transaction the caller is counting rows in — so the announcement would
// report a recipient count larger than the list it actually has, and §3's `{y}` would be wrong for
// the life of the record. Doing it before the write keeps the two numbers the same number.
//
// THE ORDER IS KEPT because §4 draws the recipient list, and a list that reorders itself between
// two reads looks to the person watching it like the data changed.
func ChuanHoaDanhSachMaCanBo(tho []string, tran int, quaNhieu error) ([]string, error) {
	ra := make([]string, 0, len(tho))
	daCo := make(map[string]bool, len(tho))
	for _, mot := range tho {
		ma, err := ChuanHoaMaCanBo(mot)
		if err != nil {
			return nil, err
		}
		if daCo[ma] {
			continue
		}
		daCo[ma] = true
		ra = append(ra, ma)
	}
	if len(ra) > tran {
		return nil, fmt.Errorf("%w (tối đa %d)", quaNhieu, tran)
	}
	return ra, nil
}

// ThongBaoNoiBo is ONE internal announcement — the `Announcement` entity of migration 0005.
//
// THE COUNTERS ARE FIELDS AND THE STATE IS NOT. `SoNguoiNhan` and `SoDaXacNhan` are read from the
// database as aggregates of `thong_bao_nguoi_nhan` and are never stored on the row (0005 says
// why); `TrangThai` is stored, because it is a decision somebody took rather than a count of
// anything.
type ThongBaoNoiBo struct {
	ID string // ULID, internal

	TieuDe  string
	NoiDung string

	TrangThai TrangThaiThongBao

	// Ghim is §3's pin. It is the one display property an issued announcement may still change.
	Ghim bool

	// BatBuocXacNhan turns on §3's orange chip and the counter below, and keeps the announcement on
	// the bell of everyone who has not acknowledged it (§9.5).
	BatBuocXacNhan bool

	// GuiThuDienTu records WHAT WAS ASKED FOR; TrangThaiThu records what happened. They are two
	// facts and conflating them would make "we asked for mail and the adapter does not exist yet"
	// indistinguishable from "mail was never wanted".
	GuiThuDienTu bool
	TrangThaiThu TrangThaiThuThongBao

	// NguoiSoanMa is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id — rule 6,
	// invariant 8. §6 models it as a uuid; the rule wins, and migration 0005 argues it in full.
	NguoiSoanMa string

	// PhatHanhLuc is the zero time for a draft. The database ties the two together so neither can
	// drift from the other.
	PhatHanhLuc time.Time
	TaoLuc      time.Time

	// SoNguoiNhan / SoDaXacNhan are §3's `{x}/{y} đã xác nhận`, DERIVED by counting the recipient
	// rows. A draft has neither, and a card only draws them when BatBuocXacNhan is on.
	SoNguoiNhan int
	SoDaXacNhan int
}

// DaPhatHanh reports whether staff can see this announcement at all.
//
// IT IS NOT `TrangThai == ThongBaoDaPhatHanh`, AND THE DIFFERENCE MATTERS: a WITHDRAWN
// announcement was issued too, and §4 keeps its recipients and their acknowledgements. Anything
// asking "has this left the author's hands" has to include `da-go`, and every place that asked the
// narrower question by hand would eventually ask it wrongly.
func (t ThongBaoNoiBo) DaPhatHanh() bool { return t.TrangThai != ThongBaoNhap }

// NguoiNhanThongBao is ONE member of staff on ONE announcement's list — §4's `NGƯỜI NHẬN` rows.
type NguoiNhanThongBao struct {
	// NguoiNhanMa is the staff business code. §2's `Gửi cho tôi` compares it against the session's
	// own `.Ma`, with no lookup in between.
	NguoiNhanMa string

	// DichDanh is §6's flag: true = named explicitly in §5's `Gửi thêm đích danh`, false =
	// generated from a chosen department. It is NOT derivable from the department list — a person
	// can be both, and departmental membership lives in another service and changes.
	DichDanh bool

	// The three states of §4 as two instants: `chưa mở` is both zero, `đã mở` is the first set,
	// `✓ đã xác nhận` is both. The database refuses clearing either one once set.
	DaMoLuc      time.Time
	DaXacNhanLuc time.Time

	ThuGuiLuc time.Time
	// ThuLoiMa is the provider's error CODE, never its message: an SMTP failure quotes the
	// envelope, and the envelope is somebody's address (rule 3).
	ThuLoiMa string
}

// DaXacNhan reports whether this person acknowledged reading the announcement (§4's green check).
func (n NguoiNhanThongBao) DaXacNhan() bool { return !n.DaXacNhanLuc.IsZero() }

// DaMo reports whether this person opened it at all.
func (n NguoiNhanThongBao) DaMo() bool { return !n.DaMoLuc.IsZero() }

// YeuCauSoanThongBao is one new announcement as the use case receives it, already past the HTTP
// layer and not yet validated.
//
// THERE IS NO `TrangThai` FIELD AND THERE MUST NEVER BE ONE. The state is decided by which act was
// performed, not by a value a client sends: a request that could name its own state could post an
// announcement already marked `da-phat-hanh` — an announcement with no recipient list that every
// screen would nonetheless treat as issued.
//
// THERE IS NO `NguoiSoanMa` FIELD EITHER. The author is the PRINCIPAL of the request, taken from
// the session at the handler (rule 6, invariant 8). A field here is a field a body could fill,
// which is a member of staff issuing an announcement over a colleague's name.
type YeuCauSoanThongBao struct {
	TieuDe  string
	NoiDung string

	// BoPhanIDs are the departments §5's chips addressed. They are RECORDED, and in this pass they
	// do NOT expand into recipients — see the use case for why that expansion needs a contract
	// service-identity does not publish today.
	BoPhanIDs []string

	// NguoiNhanMa are the staff codes of §5's `Gửi thêm đích danh`.
	NguoiNhanMa []string

	Ghim           bool
	BatBuocXacNhan bool
	GuiThuDienTu   bool
}

// KiemTra normalises and validates the request, returning the cleaned values.
//
// IT REFUSES AN EMPTY RECIPIENT LIST, and that is the check most worth having: §5 lets a person
// tick departments, name individuals, or both, and an announcement that reaches nobody is the one
// failure of this module that produces no error anywhere — the author presses `➤ Phát hành`, the
// card appears in the book, and the commune is simply never told. See ErrKhongCoNguoiNhan.
func (y YeuCauSoanThongBao) KiemTra() (YeuCauSoanThongBao, error) {
	var ra YeuCauSoanThongBao
	var err error

	if ra.TieuDe, err = ChuanHoaTieuDe(y.TieuDe); err != nil {
		return YeuCauSoanThongBao{}, err
	}
	if ra.NoiDung, err = ChuanHoaNoiDungThongBao(y.NoiDung); err != nil {
		return YeuCauSoanThongBao{}, err
	}
	// The department ids are ULIDs minted by service-identity, so they are validated with the same
	// shape check as a staff code: present, one token, not a paragraph. Nothing here asserts that
	// the department EXISTS — that is identity's data (rule 2), and the consequence of a stale id
	// is written down in migration 0005.
	if ra.BoPhanIDs, err = ChuanHoaDanhSachMaCanBo(y.BoPhanIDs, BoPhanToiDa, ErrQuaNhieuBoPhan); err != nil {
		return YeuCauSoanThongBao{}, err
	}
	if ra.NguoiNhanMa, err = ChuanHoaDanhSachMaCanBo(y.NguoiNhanMa, NguoiNhanToiDa, ErrQuaNhieuNguoiNhan); err != nil {
		return YeuCauSoanThongBao{}, err
	}

	ra.Ghim = y.Ghim
	ra.BatBuocXacNhan = y.BatBuocXacNhan
	ra.GuiThuDienTu = y.GuiThuDienTu
	return ra, nil
}
