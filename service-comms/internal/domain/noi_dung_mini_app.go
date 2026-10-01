package domain

// MINI APP CONTENT (`docs/ui-ux/11-noi-dung-mini-app.md`) — what a commune publishes to its own
// residents inside the Zalo Mini App.
//
// READ THIS BEFORE REACHING FOR A NAME IN THIS PACKAGE. Three files here now model something a
// Vietnamese speaker would call `thông báo`, and they are three different things with three
// different audiences (kb/00-foundation/ubiquitous-language.md:151):
//
//	thong_bao_noi_bo.go        a commune tells its OWN STAFF something. Table `thong_bao`.
//	thong_bao_gui_cong_dan.go  a message LEAVES the commune to ONE CITIZEN. Table of the same name.
//	this file                  an ARTICLE on a public channel, readable by anybody who opens the
//	                           Mini App — and `thong-bao` is one of its six `loai` values, §5.
//
// Every identifier here therefore carries `NoiDung`, `DanhMuc…MiniApp` or an unambiguous prefix.
// A name collision resolved by "whichever compiles" is how an internal staff notice ends up on a
// public channel — which on this surface is a disclosure, not a bug.
//
// WHAT IS MODELLED HERE AND WHAT IS NOT. The types below carry an item of content as a RECORD: what
// it says, which category it sits in, where it came from, whether residents can see it. They do NOT
// carry the portal synchronisation of §3 and §10 — there is no `core/crypto` to hold the portal's
// API key (ADR 0009, decision 7), no scheduler for §3's `Mỗi 6 giờ`, and no outbound HTTP adapter.
// Migration 0006 names all three absences in full. What survives from that half is PROVENANCE, which
// is a property of the row rather than of the job: `Nguon`, `NguonURL`, `NguonIDNgoai`, `DaSuaTay`.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// LoaiNoiDung is one of §5's six tabs. The codes are §5's own `Mã` column, character for character —
// they were read, not translated.
//
// THE LIST IS CLOSED AND THE CHECK CONSTRAINT IN MIGRATION 0006 ADMITS NO SEVENTH. A commune cannot
// add a type: §5 is a fixed set of six tabs in the interface, and a seventh value would be a tab no
// screen draws, holding rows nobody can find.
type LoaiNoiDung string

const (
	LoaiTinTuc      LoaiNoiDung = "tin-tuc"      // bài viết thường
	LoaiSuKien      LoaiNoiDung = "su-kien"      // có ngày diễn ra
	LoaiThongBao    LoaiNoiDung = "thong-bao"    // thông báo cho DÂN — not the internal staff module
	LoaiTruyenThanh LoaiNoiDung = "truyen-thanh" // bản tin audio của loa xã
	LoaiVideo       LoaiNoiDung = "video"
	LoaiBanner      LoaiNoiDung = "banner" // ảnh quảng bá trên đầu Mini App
)

// LoaiNoiDungHopLe reports whether a string is one of the six.
//
// A SWITCH AND NOT A MAP, deliberately: a map would be package-level mutable state, and the one
// thing worse than an unknown `loai` is an unknown `loai` that some other file added at runtime.
func LoaiNoiDungHopLe(l string) bool {
	switch LoaiNoiDung(l) {
	case LoaiTinTuc, LoaiSuKien, LoaiThongBao, LoaiTruyenThanh, LoaiVideo, LoaiBanner:
		return true
	}
	return false
}

// TrangThaiNoiDung is §6's chip.
//
// `TrangThaiChoDuyet` HAS NO WRITER IN THIS REPOSITORY and that is deliberate rather than
// unfinished: §10.2 puts an item here only when the PORTAL SYNC runs in `chờ duyệt` mode, and the
// sync is not built. The value exists because §6 draws the chip, and because leaving it out of the
// schema would make the sync's first pass a migration on live data.
type TrangThaiNoiDung string

const (
	// TrangThaiAn — composed and not published. §7: "Chưa bật 'Đăng lên Mini App' thì bà con chưa
	// thấy — soạn trước, đăng sau được." This is the state a new item is born in.
	TrangThaiAn TrangThaiNoiDung = "an"
	// TrangThaiChoDuyet — waiting for approval. Sync only (§10.2).
	TrangThaiChoDuyet TrangThaiNoiDung = "cho-duyet"
	// TrangThaiDangHien — live on the Mini App.
	TrangThaiDangHien TrangThaiNoiDung = "dang-hien"
)

// NguonNoiDung is §8's `nguon`: how this row came to exist. IT IS NEVER TAKEN FROM A REQUEST — see
// YeuCauThemNoiDung — and migration 0006 makes it immutable once written.
type NguonNoiDung string

const (
	NguonThuCong    NguonNoiDung = "thu-cong"     // soạn tay trong ViGov
	NguonDongBoCong NguonNoiDung = "dong-bo-cong" // đồng bộ từ Cổng thông tin điện tử của xã
)

// The bounds. Every one of them is a REFUSAL rather than a truncation: an article silently cut is an
// article whose meaning changed on the way into the database, on a channel residents read.
//
// WHY THEY ARE NOT `TieuDeToiDa` AND `NoiDungToiDaThongBao` FROM thong_bao_noi_bo.go, although two
// of the four numbers coincide: those bound an INTERNAL NOTICE typed by one member of staff into a
// textarea. These bound a PUBLISHED ARTICLE, most of which will arrive from a portal that has its
// own limits, and the day one of the two has to move it must move alone. Sharing the constant would
// couple a staff memo's length to a commune's news.
const (
	// TieuDeNoiDungToiDa — §6 prints the title in bold on one line and §7 collects it in a text
	// input. 300 runes is roughly ten times any real headline.
	TieuDeNoiDungToiDa = 300

	// TomTatNoiDungToiDa — §6's second line under the title, "tóm tắt cắt 1 dòng". 2.000 runes is
	// far past a summary and short of an article.
	TomTatNoiDungToiDa = 2000

	// ThanNoiDungToiDa — §7's body, HTML per §8. 200.000 runes is about a hundred pages: past that
	// it is a document with attachments, which this chapter does not have.
	ThanNoiDungToiDa = 200000

	// URLNoiDungToiDa bounds `anh_dai_dien_url` and `nguon_url`. 2.000 is the length past which a
	// URL stops being one in every browser anybody uses.
	URLNoiDungToiDa = 2000

	// TenDanhMucToiDa — §7's `Danh mục` select shows this. Same order as a catalogue label.
	TenDanhMucToiDa = 200

	// SlugDanhMucToiDa bounds the category's business code.
	SlugDanhMucToiDa = 64

	// ThuTuDanhMucToiDa bounds the display order of a category.
	ThuTuDanhMucToiDa = 9999

	// EventPlaceMaxRunes is migration 0011's `noi_dung_mini_app_event_place_valid` bound, in CHARACTERS
	// (the CHECK uses char_length, so diacritics do not shorten it). A vendor bound against abuse, not
	// a customer number: raising it is one migration, lowering it after staff wrote longer text is not.
	EventPlaceMaxRunes = 500
)

var (
	// ErrLoaiNoiDungKhongHopLe — a `loai` outside §5's six. Refused HERE as well as by the CHECK
	// constraint, because the constraint answers with a PostgreSQL exception and this answers with a
	// sentence naming the field.
	ErrLoaiNoiDungKhongHopLe = errors.New("noi_dung_mini_app: `type` không phải một trong sáu loại nội dung")

	// ErrTieuDeNoiDungTrong / ErrTieuDeNoiDungQuaDai — §7 marks the title required. An item with no
	// title is a row nobody can pick out of §6's table.
	ErrTieuDeNoiDungTrong  = errors.New("noi_dung_mini_app: thiếu `title`")
	ErrTieuDeNoiDungQuaDai = errors.New("noi_dung_mini_app: `title` quá dài")

	ErrTomTatQuaDai      = errors.New("noi_dung_mini_app: `summary` quá dài")
	ErrThanNoiDungQuaDai = errors.New("noi_dung_mini_app: `body` quá dài")

	// ErrURLKhongHopLe — a URL that is not http(s), or is longer than any real one.
	//
	// THE SCHEME CHECK IS THE POINT AND IT IS NOT COSMETIC. `anh_dai_dien_url` is rendered as the
	// `src` of an image and `nguon_url` as an `href`, both inside an application residents open on
	// their phones. `javascript:…` in an href is script execution on the citizen channel, and
	// `data:…` is an arbitrary payload served from the commune's own screen. An allowlist of two
	// schemes is the only form of this check that cannot be worked around by spelling.
	ErrURLKhongHopLe = errors.New("noi_dung_mini_app: địa chỉ phải bắt đầu bằng http:// hoặc https://")
	ErrURLQuaDai     = errors.New("noi_dung_mini_app: địa chỉ quá dài")

	// ErrTenDanhMucTrong / ErrTenDanhMucQuaDai — §8 gives the category a `ten`, and a category with
	// no name is an empty line in §7's select.
	ErrTenDanhMucTrong  = errors.New("danh_muc_mini_app: thiếu `name`")
	ErrTenDanhMucQuaDai = errors.New("danh_muc_mini_app: `name` quá dài")

	// ErrSlugDanhMucTrong / ErrSlugDanhMucSaiDinhDang / ErrSlugDanhMucQuaDai — the category's
	// business code, in the form ADR 0011 fixes.
	ErrSlugDanhMucTrong       = errors.New("danh_muc_mini_app: thiếu `slug`")
	ErrSlugDanhMucSaiDinhDang = errors.New(
		"danh_muc_mini_app: `slug` chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ `chuyen-doi-so`")
	ErrSlugDanhMucQuaDai = errors.New("danh_muc_mini_app: `slug` quá dài")

	// ErrThuTuDanhMucNgoaiKhoang — the display order of a category.
	ErrThuTuDanhMucNgoaiKhoang = errors.New("danh_muc_mini_app: `order` ngoài khoảng cho phép")

	// ErrDanhMucTuLamCha — §8's tree. Refused here so the caller gets a sentence rather than the
	// CHECK constraint's exception.
	ErrDanhMucTuLamCha = errors.New("danh_muc_mini_app: một danh mục không thể là cha của chính nó")

	// The per-type fields of migration 0011 (ADR 0047 §6). Each refusal mirrors one CHECK there, so a
	// member of staff reads a sentence naming the field instead of a PostgreSQL exception and a 500.
	//
	// ErrEventFieldsOnlyForEvent / ErrVideoURLOnlyForVideo — `..._event_only_for_su_kien` and
	// `..._video_url_only_for_video`. REFUSED rather than silently dropped when the client SENT a
	// value: a date typed on a news article and quietly thrown away is a date the author believes is
	// published. (A type CHANGE clears the old type's fields — that is NoiDungMiniApp.WithoutOtherTypeFields.)
	ErrEventFieldsOnlyForEvent = errors.New(
		"noi_dung_mini_app: `event_starts_at`, `event_ends_at` và `event_place` chỉ dùng cho loại su-kien")
	ErrVideoURLOnlyForVideo = errors.New("noi_dung_mini_app: `video_url` chỉ dùng cho loại video")

	// ErrEventEndsWithoutStart / ErrEventEndsBeforeStart — `..._event_window_valid`. An end with no
	// start is not a window, and an event that ends before it starts is a typo a resident would read.
	ErrEventEndsWithoutStart = errors.New("noi_dung_mini_app: có `event_ends_at` thì phải có `event_starts_at`")
	ErrEventEndsBeforeStart  = errors.New("noi_dung_mini_app: `event_ends_at` không được sớm hơn `event_starts_at`")

	// ErrEventPlaceTooLong / ErrEventPlaceInvalid — `..._event_place_valid`.
	ErrEventPlaceTooLong = errors.New("noi_dung_mini_app: `event_place` quá dài")
	ErrEventPlaceInvalid = errors.New("noi_dung_mini_app: `event_place` không được chứa ký tự điều khiển")

	// ErrEventTimeInvalid — an event instant that is not RFC 3339. An instant with no offset would make
	// the server pick a time zone for the commune, which is exactly the guess this refuses.
	ErrEventTimeInvalid = errors.New(
		"noi_dung_mini_app: thời điểm phải theo RFC 3339 và có múi giờ, ví dụ 2026-10-01T08:00:00+07:00")
)

// ParseEventInstant reads one event instant off the wire. "" means "none" and returns the zero time.
//
// `field` is the contract name, put in the sentence so the author knows which of the two boxes is
// wrong. The value itself is never echoed.
func ParseEventInstant(field, s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w (`%s`)", ErrEventTimeInvalid, field)
	}
	return t.UTC(), nil
}

// NormalizeEventPlace trims and validates §7's event place. "" is valid and means "no place" — stored
// as NULL, because the CHECK refuses a blank string.
func NormalizeEventPlace(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len([]rune(s)) > EventPlaceMaxRunes {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrEventPlaceTooLong, EventPlaceMaxRunes)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrEventPlaceInvalid
		}
	}
	return s, nil
}

// NormalizeVideoURL validates the external video link (ADR 0047 §6 (3)) with the same http(s)
// allowlist as every other link here (ChuanHoaURL), then LOWER-CASES THE SCHEME ONLY.
//
// Why the scheme and nothing else: migration 0011 asks the write path to store it lower-cased, and a
// scheme is case-insensitive by RFC 3986 — `HTTPS://x` and `https://x` are the same URL, so this is not
// rewriting what staff typed. The host and path are left alone: their case can matter.
//
// ChuanHoaURL's bound (URLNoiDungToiDa, in bytes) is tighter than the CHECK's 2048 characters, so a
// value that passes here always passes the database.
func NormalizeVideoURL(s string) (string, error) {
	u, err := ChuanHoaURL(s)
	if err != nil {
		return "", fmt.Errorf("%w (`video_url`)", err)
	}
	if u == "" {
		return "", nil
	}
	i := strings.Index(u, "://")
	return strings.ToLower(u[:i]) + u[i:], nil
}

// CheckEventWindow is `..._event_window_valid`: no end without a start, and no end before the start.
// Zero times mean "not set".
func CheckEventWindow(startsAt, endsAt time.Time) error {
	if endsAt.IsZero() {
		return nil
	}
	if startsAt.IsZero() {
		return ErrEventEndsWithoutStart
	}
	if endsAt.Before(startsAt) {
		return ErrEventEndsBeforeStart
	}
	return nil
}

// checkTypeFields is the three per-type CHECKs of migration 0011 on one set of values.
func checkTypeFields(loai LoaiNoiDung, startsAt, endsAt time.Time, place, videoURL string) error {
	if loai != LoaiSuKien && (!startsAt.IsZero() || !endsAt.IsZero() || place != "") {
		return ErrEventFieldsOnlyForEvent
	}
	if loai != LoaiVideo && videoURL != "" {
		return ErrVideoURLOnlyForVideo
	}
	return CheckEventWindow(startsAt, endsAt)
}

// ChuanHoaTieuDeNoiDung trims and validates an article title.
//
// IT CARRIES VIETNAMESE WITH DIACRITICS — it is a headline a resident reads — so nothing here
// restricts the character set beyond refusing control characters, which corrupt a screen and a log
// line alike.
func ChuanHoaTieuDeNoiDung(tieuDe string) (string, error) {
	tieuDe = strings.TrimSpace(tieuDe)
	switch {
	case tieuDe == "":
		return "", ErrTieuDeNoiDungTrong
	case len([]rune(tieuDe)) > TieuDeNoiDungToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTieuDeNoiDungQuaDai, TieuDeNoiDungToiDa)
	}
	for _, r := range tieuDe {
		if unicode.IsControl(r) {
			return "", ErrTieuDeNoiDungTrong
		}
	}
	return tieuDe, nil
}

// ChuanHoaVanBanDai trims and bounds a multi-line field — the summary and the body.
//
// NEWLINES, CARRIAGE RETURNS AND TABS ARE ALLOWED and every other control character is not. A
// browser on Windows sends CRLF, and refusing it would reject an ordinary form submission.
//
// ⚠ IT DOES NOT SANITISE HTML, AND THAT LIMIT IS WRITTEN HERE RATHER THAN LEFT TO BE ASSUMED. §8
// says `noi_dung` is HTML and §7 wants a rich-text editor, so the body is STORED AS GIVEN. There is
// no HTML sanitiser in this repository, and writing one from scratch is how sanitisers get written
// wrongly. What follows, stated so it is a known risk rather than a surprise: a member of staff with
// `content.update` can put arbitrary markup — including script — into something every resident of
// the commune opens. The permission is the only control today. The fix is a vetted sanitiser at the
// point the Mini App renders it, or at this boundary, and it is a decision with an owner rather than
// a line somebody adds here.
func ChuanHoaVanBanDai(s string, tran int, quaDai error) (string, error) {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > tran {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", quaDai, tran)
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", fmt.Errorf("%w", quaDai)
		}
	}
	return s, nil
}

// ChuanHoaURL trims and validates a link. AN EMPTY STRING IS VALID and means "no link".
//
// See ErrURLKhongHopLe for why the scheme allowlist is not cosmetic.
func ChuanHoaURL(u string) (string, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", nil
	}
	if len(u) > URLNoiDungToiDa {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrURLQuaDai, URLNoiDungToiDa)
	}
	// LOWER-CASED FOR THE COMPARISON ONLY, never for storage: `HTTPS://…` is a valid URL somebody
	// may have pasted, and rewriting what they typed is what rule 7 keeps us from doing to stored
	// values. `url.Parse` is deliberately not used — it accepts a scheme-less string as a relative
	// path, which is exactly the shape this check exists to refuse.
	thap := strings.ToLower(u)
	if !strings.HasPrefix(thap, "http://") && !strings.HasPrefix(thap, "https://") {
		return "", ErrURLKhongHopLe
	}
	for _, r := range u {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrURLKhongHopLe
		}
	}
	return u, nil
}

// ChuanHoaTenDanhMuc trims and validates a category's display name.
func ChuanHoaTenDanhMuc(ten string) (string, error) {
	ten = strings.TrimSpace(ten)
	switch {
	case ten == "":
		return "", ErrTenDanhMucTrong
	case len([]rune(ten)) > TenDanhMucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTenDanhMucQuaDai, TenDanhMucToiDa)
	}
	for _, r := range ten {
		if unicode.IsControl(r) {
			return "", ErrTenDanhMucTrong
		}
	}
	return ten, nil
}

// ChuanHoaSlugDanhMuc trims and validates a category's business code.
//
// THE RULE IS THE SAME AS ChuanHoaMa's AND THE FUNCTION IS NOT, deliberately. `ChuanHoaMa` answers
// with sentences naming `code`, which is the contract field of the map-asset-type catalogue; this
// contract calls the field `slug`, because §8 does. An error message naming a field the client never
// sent is a message that sends somebody looking at the wrong input. The repository already accepts
// this trade explicitly — see the note in danh_muc_ba_tang.go on why the same file exists in five
// services.
//
// NO CASE FOLDING. `Chuyen-Doi-So` is REFUSED rather than lower-cased: silently changing a code the
// person typed means the code stored is not the code they saw, and rule 7, invariant 3 never lets us
// correct it afterwards.
func ChuanHoaSlugDanhMuc(slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	switch {
	case slug == "":
		return "", ErrSlugDanhMucTrong
	case len(slug) > SlugDanhMucToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrSlugDanhMucQuaDai, SlugDanhMucToiDa)
	}
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return "", ErrSlugDanhMucSaiDinhDang
	}
	truocLaGach := false
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			truocLaGach = false
		case c == '-':
			if truocLaGach {
				// `chuyen--doi-so` reads as one code and sorts as another. Refuse it while it is
				// still a typo rather than after every article is filed under it.
				return "", ErrSlugDanhMucSaiDinhDang
			}
			truocLaGach = true
		default:
			return "", ErrSlugDanhMucSaiDinhDang
		}
	}
	return slug, nil
}

// KiemTraThuTuDanhMuc bounds a category's display order. NEGATIVE IS REFUSED, not clamped — see
// KiemTraThuTu next door for the argument.
func KiemTraThuTuDanhMuc(thuTu int) error {
	if thuTu < 0 || thuTu > ThuTuDanhMucToiDa {
		return fmt.Errorf("%w (0..%d)", ErrThuTuDanhMucNgoaiKhoang, ThuTuDanhMucToiDa)
	}
	return nil
}

// --- the records -------------------------------------------------------------------------------

// DanhMucMiniApp is ONE category of the commune's own Mini App filing tree — the `ContentCategory`
// entity of migration 0006.
type DanhMucMiniApp struct {
	ID string // ULID, internal

	// Ten is a NAME the commune gave a category of its own, so the contract field is `name` and not
	// `label` — kb/00-foundation/ubiquitous-language.md draws that line at the schema, and this
	// column is `ten`, next to `bo_phan.ten`.
	Ten string

	// Slug is the business code: the stable handle an audit entry and an export have on this row.
	Slug string

	// ChaID is "" at the root. §3's sample tree is two levels (`Danh mục › Chuyển đổi số`); nothing
	// in the chapter says two is the limit, so the type expresses a tree.
	ChaID string

	ThuTu  int
	TaoLuc time.Time
}

// NoiDungMiniApp is ONE item of content — the `ContentItem` entity of migration 0006.
//
// `TepDinhKem` IS NOT A FIELD HERE, although migration 0006 creates the column §8 declares. Nothing
// in this pass writes it, and §6's `Tệp đính kèm` column is `🔗 Có ảnh` / `—`, which is derived from
// `AnhDaiDienURL` — see CoAnh below. A field read from a column no path fills is a field every layer
// above has to carry and no screen can trust.
type NoiDungMiniApp struct {
	ID string // ULID, internal

	Loai LoaiNoiDung

	// DanhMucID is "" for §7's `— Chưa xếp danh mục —`. That is an ordinary state, not a missing
	// value: an article synchronised before the commune built its tree has nowhere to sit.
	DanhMucID string

	TieuDe  string
	TomTat  string
	NoiDung string // HTML, stored as given — see ChuanHoaVanBanDai for the sanitisation gap

	AnhDaiDienURL string

	// NgayDang is §6's `Ngày đăng`, a DATE. It differs from TaoLuc for an article carried over from
	// the portal with the portal's own publication date.
	NgayDang time.Time

	// LuotXem is §6's `👁 {n}`. NOTHING INCREMENTS IT IN THIS PASS: the only thing that legitimately
	// would is a resident opening the article, and the citizen-facing read of §9 is not built.
	LuotXem int

	TrangThai TrangThaiNoiDung

	// PublishedAt is ADR 0047 §6 G1: the instant of the FIRST publish, never changed afterwards. The
	// zero time means NULL — never published, OR published before migration 0011 recorded the instant
	// (no backfill, on purpose: 0011 explains why `ngay_dang` cannot be copied into it). Readers fall
	// back to NgayDang when it is zero. Migration 0011's trigger refuses every change once it is set.
	PublishedAt time.Time

	// EventStartsAt, EventEndsAt and EventPlace exist only on `su-kien` (migration 0011's CHECKs); zero
	// / "" means NULL. EventPlace is free text that can name a household ("sân nhà ông …"): same
	// standing as TieuDe — never logged, never in the audit delta.
	EventStartsAt time.Time
	EventEndsAt   time.Time
	EventPlace    string

	// VideoURL exists only on `video`: an EXTERNAL http(s) link opened outside the Mini App (ADR 0047
	// §6 (3)). "" means NULL.
	VideoURL string

	// CoverImageFileID is migration 0011's `cover_image_file_id`: this service's own `stored_file` row
	// holding the UPLOADED cover (ADR 0047 §6 (1)). "" = NULL. Distinct from AnhDaiDienURL, which is a
	// link to a file some other system serves and is never published by this service.
	CoverImageFileID string

	// Nguon, NguonURL, NguonIDNgoai and DaSuaTay are the provenance half of §8. They describe the
	// ROW rather than the sync job, which is why they are here while the job is not — migration 0006
	// argues it in full.
	Nguon        NguonNoiDung
	NguonURL     string
	NguonIDNgoai string
	DaSuaTay     bool

	// NguoiTaoMa is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id — rule 6,
	// invariant 8. §8 models it as a uuid; the rule wins, and migration 0006 argues it.
	NguoiTaoMa string

	TaoLuc     time.Time
	CapNhatLuc time.Time
}

// CoAnh answers §6's `Tệp đính kèm` column: `🔗 Có ảnh` when there is an image, `—` when there is
// not.
//
// DERIVED AND NOT STORED. A boolean column beside the URL would be two representations of one fact,
// and the stale one is what a screen would read (rule 9). Either image counts: the legacy link or the
// uploaded cover (migration 0011).
func (n NoiDungMiniApp) CoAnh() bool { return n.AnhDaiDienURL != "" || n.CoverImageFileID != "" }

// HienChoDan reports whether residents can see this item at all.
//
// IT IS NOT `TrangThai == TrangThaiDangHien` SPELLED OUT AT EVERY CALL SITE, and that is the point:
// `cho-duyet` and `an` are both invisible to a resident for different reasons, and a place that
// asked the narrow question by hand would eventually ask it wrongly — by testing for `an` alone and
// publishing everything waiting for approval.
func (n NoiDungMiniApp) HienChoDan() bool { return n.TrangThai == TrangThaiDangHien }

// CheckTypeFields reports whether the record satisfies migration 0011's three per-type CHECKs. It is
// asked of the MERGED row on an edit, because a PATCH naming only `event_ends_at` is valid or not
// depending on the start already stored.
func (n NoiDungMiniApp) CheckTypeFields() error {
	return checkTypeFields(n.Loai, n.EventStartsAt, n.EventEndsAt, n.EventPlace, n.VideoURL)
}

// WithoutOtherTypeFields clears the columns the record's CURRENT type cannot carry.
//
// `loai` is editable (0006:454), and 0011's CHECKs refuse an UPDATE that moves an article away from
// `su-kien` / `video` while leaving that type's columns set. Clearing them in the same UPDATE is the
// intended behaviour: an event window left on a news article is a date a citizen reads.
func (n NoiDungMiniApp) WithoutOtherTypeFields() NoiDungMiniApp {
	if n.Loai != LoaiSuKien {
		n.EventStartsAt, n.EventEndsAt, n.EventPlace = time.Time{}, time.Time{}, ""
	}
	if n.Loai != LoaiVideo {
		n.VideoURL = ""
	}
	return n
}

// --- the requests ------------------------------------------------------------------------------

// YeuCauThemNoiDung is one new item as the use case receives it — §7's modal, field for field.
//
// THERE IS NO `TrangThai` FIELD AND THERE MUST NEVER BE ONE. §7 collects a CHECKBOX, `Đăng lên Mini
// App`, and the state is decided from it by the use case. A request that could name its own state
// could post an item already marked `cho-duyet` — a state §10.2 reserves for the sync — or publish
// one straight past whatever approval a commune later introduces.
//
// THERE IS NO `Nguon`, `NguonURL` OR `NguonIDNgoai` FIELD EITHER. Provenance decides what may later
// be done to the row: §10.4 protects a hand-edited synced article from the next run, and a client
// that could claim `dong-bo-cong` could take a row it composed out of the commune's own hands. The
// store writes the value as a LITERAL for the same reason.
//
// AND THERE IS NO `NguoiTaoMa`. The author is the PRINCIPAL of the request, taken from the session at
// the handler (rule 6, invariant 8). A field here is a field a body could fill, which is one member
// of staff publishing over a colleague's name.
//
// THERE IS NO `NgayDang` EITHER, and that one is simply §7: the modal has no date field. The use case
// dates a hand-composed item today. The column is editable in the schema because a future sync must
// carry the portal's publication date, not because a form may set it.
type YeuCauThemNoiDung struct {
	Loai string

	// DanhMucID is "" for `— Chưa xếp danh mục —`.
	DanhMucID string

	TieuDe        string
	TomTat        string
	NoiDung       string
	AnhDaiDienURL string

	// EventStartsAt, EventEndsAt, EventPlace — `su-kien` only. Zero / "" = not given.
	EventStartsAt time.Time
	EventEndsAt   time.Time
	EventPlace    string

	// VideoURL — `video` only. "" = not given.
	VideoURL string

	// CoverImageFileID — an uploaded, completed cover (`ready`) issued WITHOUT an article id; the new
	// article takes the id that upload minted (migration 0011: the modal uploads before `Lưu`). "" = none.
	CoverImageFileID string

	// DangLenMiniApp is §7's checkbox. true -> `dang-hien`, false -> `an`.
	//
	// THERE IS NO `PublishedAt` FIELD, for the reason there is no `TrangThai`: G1's instant is fixed by
	// the ACT of publishing, which the use case performs. A request that could name it could backdate a
	// public authority's first publication.
	DangLenMiniApp bool
}

// KiemTra normalises and validates the request, returning the cleaned values.
func (y YeuCauThemNoiDung) KiemTra() (YeuCauThemNoiDung, error) {
	var ra YeuCauThemNoiDung
	var err error

	if !LoaiNoiDungHopLe(y.Loai) {
		return YeuCauThemNoiDung{}, ErrLoaiNoiDungKhongHopLe
	}
	ra.Loai = y.Loai

	if ra.TieuDe, err = ChuanHoaTieuDeNoiDung(y.TieuDe); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	if ra.TomTat, err = ChuanHoaVanBanDai(y.TomTat, TomTatNoiDungToiDa, ErrTomTatQuaDai); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	if ra.NoiDung, err = ChuanHoaVanBanDai(y.NoiDung, ThanNoiDungToiDa, ErrThanNoiDungQuaDai); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	if ra.AnhDaiDienURL, err = ChuanHoaURL(y.AnhDaiDienURL); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	// The category id is a ULID minted by this service. It is validated for SHAPE only — that it is
	// one token and not a paragraph; whether the category EXISTS is the foreign key's answer, and
	// asking it twice would be a check that can disagree with the constraint underneath it.
	ra.DanhMucID = strings.TrimSpace(y.DanhMucID)
	if len(ra.DanhMucID) > MaCanBoToiDa {
		return YeuCauThemNoiDung{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrMaCanBoQuaDai, MaCanBoToiDa)
	}

	if ra.EventPlace, err = NormalizeEventPlace(y.EventPlace); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	if ra.VideoURL, err = NormalizeVideoURL(y.VideoURL); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	if ra.CoverImageFileID, err = cleanFileID(y.CoverImageFileID); err != nil {
		return YeuCauThemNoiDung{}, err
	}
	ra.EventStartsAt, ra.EventEndsAt = y.EventStartsAt.UTC(), y.EventEndsAt.UTC() // zero stays zero
	// On a create the type is known here, so the per-type CHECKs are answered before any transaction.
	if err := checkTypeFields(LoaiNoiDung(ra.Loai), ra.EventStartsAt, ra.EventEndsAt,
		ra.EventPlace, ra.VideoURL); err != nil {
		return YeuCauThemNoiDung{}, err
	}

	ra.DangLenMiniApp = y.DangLenMiniApp
	return ra, nil
}

// YeuCauSuaNoiDung is a PARTIAL edit — §6's `✎` reopening §7's modal. A nil pointer means "leave
// this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Every field here has a meaningful zero: an empty summary
// is "no summary", an empty category is `— Chưa xếp danh mục —`, and `DangLenMiniApp` false is
// "take it off the Mini App". A struct of plain values cannot tell "the client did not mention this"
// from "the client cleared it", so a screen that edits only the title would silently unpublish the
// article and drop it out of its category.
//
// THE SAME THREE FIELDS ARE ABSENT AS ON THE CREATE, for the same reasons, plus one that only
// applies here: `DaSuaTay` is not a field. §10.4's flag records that a member of STAFF edited a
// SYNCED item, which is a fact about the act rather than a value in it — the use case sets it, and
// migration 0006 refuses to clear it.
type YeuCauSuaNoiDung struct {
	Loai           *string
	DanhMucID      *string
	TieuDe         *string
	TomTat         *string
	NoiDung        *string
	AnhDaiDienURL  *string
	DangLenMiniApp *bool

	// The per-type fields of migration 0011. nil = leave alone; a pointer to the ZERO time / "" =
	// clear. Whether a value may be set depends on the type AFTER the edit, which only the merged row
	// knows — see SoanNoiDungMiniApp.Sua.
	EventStartsAt *time.Time
	EventEndsAt   *time.Time
	EventPlace    *string
	VideoURL      *string

	// CoverImageFileID — nil = leave alone; "" = DETACH (the column goes NULL; the file row stays, rule
	// 7); an id = attach that file, which must have been uploaded for THIS article and be `ready`.
	CoverImageFileID *string
}

// SetsEventField reports whether the request gives an event field a VALUE (clearing one does not
// count): such a request on an article that will not be `su-kien` is refused, never silently dropped.
func (y YeuCauSuaNoiDung) SetsEventField() bool {
	return (y.EventStartsAt != nil && !y.EventStartsAt.IsZero()) ||
		(y.EventEndsAt != nil && !y.EventEndsAt.IsZero()) ||
		(y.EventPlace != nil && *y.EventPlace != "")
}

// SetsVideoURL is SetsEventField for `video_url`.
func (y YeuCauSuaNoiDung) SetsVideoURL() bool { return y.VideoURL != nil && *y.VideoURL != "" }

// KiemTra validates whatever the request actually mentioned, and returns the cleaned values in the
// same shape.
func (y YeuCauSuaNoiDung) KiemTra() (YeuCauSuaNoiDung, error) {
	ra := YeuCauSuaNoiDung{DangLenMiniApp: y.DangLenMiniApp}

	if y.Loai != nil {
		if !LoaiNoiDungHopLe(*y.Loai) {
			return YeuCauSuaNoiDung{}, ErrLoaiNoiDungKhongHopLe
		}
		l := *y.Loai
		ra.Loai = &l
	}
	if y.TieuDe != nil {
		v, err := ChuanHoaTieuDeNoiDung(*y.TieuDe)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.TieuDe = &v
	}
	if y.TomTat != nil {
		v, err := ChuanHoaVanBanDai(*y.TomTat, TomTatNoiDungToiDa, ErrTomTatQuaDai)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.TomTat = &v
	}
	if y.NoiDung != nil {
		v, err := ChuanHoaVanBanDai(*y.NoiDung, ThanNoiDungToiDa, ErrThanNoiDungQuaDai)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.NoiDung = &v
	}
	if y.AnhDaiDienURL != nil {
		v, err := ChuanHoaURL(*y.AnhDaiDienURL)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.AnhDaiDienURL = &v
	}
	if y.DanhMucID != nil {
		v := strings.TrimSpace(*y.DanhMucID)
		if len(v) > MaCanBoToiDa {
			return YeuCauSuaNoiDung{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrMaCanBoQuaDai, MaCanBoToiDa)
		}
		ra.DanhMucID = &v
	}
	if y.EventPlace != nil {
		v, err := NormalizeEventPlace(*y.EventPlace)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.EventPlace = &v
	}
	if y.VideoURL != nil {
		v, err := NormalizeVideoURL(*y.VideoURL)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.VideoURL = &v
	}
	if y.CoverImageFileID != nil {
		v, err := cleanFileID(*y.CoverImageFileID)
		if err != nil {
			return YeuCauSuaNoiDung{}, err
		}
		ra.CoverImageFileID = &v
	}
	if y.EventStartsAt != nil {
		v := y.EventStartsAt.UTC()
		ra.EventStartsAt = &v
	}
	if y.EventEndsAt != nil {
		v := y.EventEndsAt.UTC()
		ra.EventEndsAt = &v
	}
	return ra, nil
}

// YeuCauThemDanhMuc is one new category — §6's `⊞ Danh mục tin`.
type YeuCauThemDanhMuc struct {
	Ten   string
	Slug  string
	ChaID string
	ThuTu int
}

// KiemTra normalises and validates the request.
func (y YeuCauThemDanhMuc) KiemTra() (YeuCauThemDanhMuc, error) {
	var ra YeuCauThemDanhMuc
	var err error

	if ra.Ten, err = ChuanHoaTenDanhMuc(y.Ten); err != nil {
		return YeuCauThemDanhMuc{}, err
	}
	if ra.Slug, err = ChuanHoaSlugDanhMuc(y.Slug); err != nil {
		return YeuCauThemDanhMuc{}, err
	}
	if err := KiemTraThuTuDanhMuc(y.ThuTu); err != nil {
		return YeuCauThemDanhMuc{}, err
	}
	ra.ThuTu = y.ThuTu

	ra.ChaID = strings.TrimSpace(y.ChaID)
	if len(ra.ChaID) > MaCanBoToiDa {
		return YeuCauThemDanhMuc{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrMaCanBoQuaDai, MaCanBoToiDa)
	}
	return ra, nil
}
