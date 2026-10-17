package domain

// The commune's CITIZEN CONTACT DIRECTORY ("Danh bạ người dân", ADR 0097; migration 0033): the two
// entities, the shape rules of one row as it arrives on a write route, and the ONE phone
// normalisation every path in and out of the directory goes through.
//
// STANDARD LIBRARY ONLY (doc.go). No SQL, no HTTP, no commune: `tenant_id` is not a field of any type
// here because it travels in context.Context (rule 1, invariant 4) and the store scopes every query.
//
// THIS IS NOT `dinh_danh_cong_dan` (ADR 0002): that is a citizen's platform-wide identity, created by
// the citizen signing in to the Mini App. A directory row is created BY STAFF, about a person who may
// never open the app, and belongs to one commune. The two meet in exactly one place — the "Zalo"
// column (ADR 0097 §3) — and CitizenPhoneIdentityForm is the bridge between their two spellings of a
// number.
//
// NO ERROR MESSAGE HERE EVER QUOTES THE VALUE IT REFUSED (rule 3, forbidden #3). Every string this file
// inspects is personal data: a name, a phone number, a home address, a note about a person. The
// sentences name the FIELD and the RULE, which is what the person filling the form needs, and they are
// safe to log, to return and to read out.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// CitizenContactSource is `citizen_contact.source` — how a row entered the directory. Stored in
// Vietnamese (ADR 0011); the CHECK of migration 0033 admits exactly these two. A THIRD value (the
// citizen's own registration through the Mini App) is deliberately absent — ADR 0097 D3, stop
// condition #4.
type CitizenContactSource string

const (
	// CitizenContactSourceManual — typed by staff on the "Thêm người dân" form.
	CitizenContactSourceManual CitizenContactSource = "nhap-tay"
	// CitizenContactSourceExcel — one row of an Excel import; the row carries its ImportBatchID.
	CitizenContactSourceExcel CitizenContactSource = "nhap-excel"
)

// Valid reports whether the value is one of the two.
func (s CitizenContactSource) Valid() bool {
	return s == CitizenContactSourceManual || s == CitizenContactSourceExcel
}

// CitizenContact is one row of `citizen_contact`: a person the commune can reach.
//
// Phone is the NATIONAL form ("0…", NormalizeCitizenPhone) and is personal data (rule 3) — masked
// by privacy.MaskPhone before it leaves the API, revealed only through "Xem số đầy đủ" (ADR 0097 D2).
// FullName and Address are personal data too. The folded name is a GENERATED column the store never
// selects, so it is not here.
type CitizenContact struct {
	ID                string
	FullName          string
	Phone             string
	ResidentialUnitID *string // thon_to_dan_pho of the same commune; nil = not entered
	Address           string
	Note              string
	Tags              []string
	Source            CitizenContactSource
	IsActive          bool
	// Email is the static e-mail column (ADR 0101 #6), lower case, "" = none. Personal data: masked at
	// the API (privacy.MaskEmail), revealed only through the audited route.
	Email string
	// Attributes are the commune's dynamic attribute values, {code: JSON value} (ADR 0101; migration
	// 0036). Values of RETIRED fields are kept here and hidden on the way out (LiveAttributes).
	Attributes map[string]json.RawMessage
	// HouseholdCode is RESERVED (owner, 16/10/2026) — carried so a row round-trips, never set by
	// this service.
	HouseholdCode *string
	ImportBatchID *string

	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	DeletedBy    string // staff business code (rule 6, invariant 8)
	DeleteReason string
}

// CitizenGroup is one row of `citizen_group`: a named group the commune defines.
type CitizenGroup struct {
	ID          string
	Name        string
	Description string
	SortOrder   int

	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	DeletedBy    string
	DeleteReason string
}

// TagSensitiveWarning is the fixed sentence under the tag field (ADR 0097 D4, §5). It cannot be
// switched off. A machine cannot tell whether "hộ nghèo" is sensitive data under Decree 13/2023
// Article 2(4), so the sentence is the reminder AT THE PLACE A PERSON TYPES — and the evidence that
// the commune was told, should an inspection ask. Kept here so the import template's guide sheet
// and the web form say the same words.
const TagSensitiveWarning = "Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị"

// Length ceilings, counted in RUNES (Vietnamese is up to three bytes per accented character; a byte
// bound would cut a sentence at a third of its length). Every column is unbounded TEXT; a field with no
// ceiling is a field a client chooses the size of, on a process serving many communes, in a row that is
// never deleted. The numbers refuse a payload, never honest input.
const (
	// MaxCitizenContactFullName — the directory's bound; a Vietnamese full name fits in a fifth of it.
	MaxCitizenContactFullName = 255
	// MaxCitizenContactAddress — the citizen-letter register's sender-address bound (service-documents
	// MaxSenderAddress), the same kind of field.
	MaxCitizenContactAddress = 500
	// MaxCitizenContactNote — a two-line textarea on the form (chapter 16 §5); a thousand runes is a
	// paragraph, not a note.
	MaxCitizenContactNote = 1000
	// MaxCitizenContactTags bounds how many free labels one row carries. The tag strip on the list
	// screen shows every distinct tag of the commune; twenty per person is already more than a strip
	// can show.
	MaxCitizenContactTags = 20
	// MaxCitizenContactTagLen bounds one label. "cựu chiến binh" is 14.
	MaxCitizenContactTagLen = 50
	// MaxCitizenGroupName — a catalogue name, the org-chart unit-name bound (TranTenBoPhan).
	MaxCitizenGroupName = TranTenBoPhan
	// MaxCitizenGroupDescription — "Hộ kinh doanh cá thể, nhận thông báo thuế" fits in a tenth of it.
	MaxCitizenGroupDescription = 500
	// MaxCitizenGroupSortOrder — the same rank bound as every ordered catalogue of this service.
	MaxCitizenGroupSortOrder = TranThuTuBoPhan
	// CitizenDirectoryDeleteReasonMaxLen is the CHECK on `delete_reason` (migration 0033, both tables).
	CitizenDirectoryDeleteReasonMaxLen = 500

	// The `phone ~ '^[0-9]{9,15}$'` CHECK of migration 0033, as two bounds. The schema refuses what
	// the application missed; the application refuses first, with a sentence.
	minCitizenPhoneDigits = 9
	maxCitizenPhoneDigits = 15
)

// The refusals. Sentinels, so the HTTP layer maps them to 400 by identity rather than by matching on
// text. Each sentence is shown to the person as is, so it names the rule.
var (
	ErrCitizenContactFullNameMissing  = errors.New("danh bạ người dân: thiếu họ và tên")
	ErrCitizenContactFullNameTooLong  = errors.New("danh bạ người dân: họ và tên quá dài")
	ErrCitizenPhoneMissing            = errors.New("danh bạ người dân: thiếu số điện thoại")
	ErrCitizenPhoneInvalid            = errors.New("danh bạ người dân: số điện thoại chỉ gồm chữ số, dấu cách, dấu +, -, . và ngoặc; +84 hoặc 84 ở đầu được quy về 0")
	ErrCitizenPhoneTooLong            = errors.New("danh bạ người dân: số điện thoại quá dài")
	ErrCitizenPhoneNotNational        = errors.New("danh bạ người dân: số điện thoại phải bắt đầu bằng 0 (hoặc +84 / 84) và có từ 9 đến 15 chữ số")
	ErrCitizenContactAddressTooLong   = errors.New("danh bạ người dân: địa chỉ quá dài")
	ErrCitizenContactNoteTooLong      = errors.New("danh bạ người dân: ghi chú quá dài")
	ErrCitizenContactTooManyTags      = errors.New("danh bạ người dân: quá nhiều nhãn")
	ErrCitizenContactTagTooLong       = errors.New("danh bạ người dân: nhãn quá dài")
	ErrCitizenContactTagHasComma      = errors.New("danh bạ người dân: nhãn không được chứa dấu phẩy — dấu phẩy là ký tự ngăn cách giữa các nhãn")
	ErrCitizenContactUnitRefInvalid   = errors.New("danh bạ người dân: mã thôn / tổ dân phố không đúng dạng")
	ErrCitizenContactSourceUnknown    = errors.New("danh bạ người dân: nguồn dòng không hợp lệ (nhap-tay hoặc nhap-excel)")
	ErrCitizenGroupNameMissing        = errors.New("danh bạ người dân: thiếu tên nhóm")
	ErrCitizenGroupNameTooLong        = errors.New("danh bạ người dân: tên nhóm quá dài")
	ErrCitizenGroupDescriptionTooLong = errors.New("danh bạ người dân: mô tả nhóm quá dài")
	ErrCitizenGroupSortOrderRange     = errors.New("danh bạ người dân: thứ tự nhóm phải từ 0 đến 9999")
	ErrCitizenDirectoryReasonMissing  = errors.New("danh bạ người dân: phải nêu lý do gỡ người dân khỏi danh bạ")
	ErrCitizenDirectoryReasonTooLong  = errors.New("danh bạ người dân: lý do xoá quá dài")
)

// NormalizeCitizenPhone brings one typed number to the NATIONAL form the directory stores: digits
// only, leading 0 — "0900000000".
//
// WHAT IT ACCEPTS is exactly what the staff directory accepts (ChuanHoaSoDienThoai: digits, space,
// +, -, ., brackets; at most 32 characters) — one answer in this service to "what may a telephone
// number contain". What it then DOES differs, and migration 0033 says why: the directory's unique key
// is (tenant_id, phone, live_key), so "0900 000 000" and "+84900000000" stored as typed would be one
// person twice. The staff directory keeps the text as typed because its columns carry no such key.
//
// THE FOLD, in order: separators removed; a leading "+" dropped; a leading "00" (the international
// dialling prefix, "0084…") dropped; then a number that does NOT start with 0 and starts with the
// country code 84 gets its 84 replaced by 0. A number that already starts with 0 is never touched —
// "084…" is a real Vinaphone prefix, and eating its 84 would turn one subscriber into another.
//
// WHY THE NATIONAL FORM AND NOT THE "84…" FORM `dinh_danh_cong_dan` holds (chapter 16 §8; owner's
// card): it is what staff type and read, and what privacy.MaskPhone renders as "09****0000". The
// identity form is derived from it by CitizenPhoneIdentityForm, for the one query that needs it.
//
// WHY A NUMBER WITHOUT A LEADING 0 IS REFUSED rather than given one: "900000000" may be a number whose
// 0 was dropped, or a number from another country; guessing writes a wrong number into a government
// record the commune will later use to reach that person. Refusing costs the typist one character.
//
// Neither the argument nor the result ever enters an error (rule 3, forbidden #3).
func NormalizeCitizenPhone(raw string) (string, error) {
	typed, err := ChuanHoaSoDienThoai(raw)
	switch {
	case errors.Is(err, ErrSoDienThoaiQuaDai):
		return "", ErrCitizenPhoneTooLong
	case err != nil:
		return "", ErrCitizenPhoneInvalid
	case typed == "":
		return "", ErrCitizenPhoneMissing
	}

	// Separators go first, so "(+84) 900-000-000" is judged as "+84900000000". A "+" means
	// "international prefix follows" and only makes sense before the first digit; anywhere else it is
	// a typo the charset above let through, and dropping it silently would join two numbers into one.
	compact := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "", ".", "").Replace(typed)
	if i := strings.LastIndex(compact, "+"); i > 0 {
		return "", ErrCitizenPhoneInvalid
	}
	d := strings.TrimPrefix(strings.TrimPrefix(compact, "+"), "00")
	if !strings.HasPrefix(d, "0") && strings.HasPrefix(d, "84") {
		d = "0" + d[2:]
	}
	if !isNationalPhone(d) {
		return "", ErrCitizenPhoneNotNational
	}
	return d, nil
}

// isNationalPhone is the stored shape: digits only, leading 0, within the 0033 CHECK's bounds.
func isNationalPhone(d string) bool {
	if len(d) < minCitizenPhoneDigits || len(d) > maxCitizenPhoneDigits || d[0] != '0' {
		return false
	}
	for i := 0; i < len(d); i++ {
		if d[i] < '0' || d[i] > '9' {
			return false
		}
	}
	return true
}

// CitizenPhoneIdentityForm is the exact string `dinh_danh_cong_dan.so_dien_thoai` holds for a
// national number: the leading 0 replaced by the country code — "0900000000" → "84900000000".
//
// THE EVIDENCE for that form, so the "Zalo" column's join is an EQUALITY and not a LIKE: the column is
// written only by crosstenant.DinhDanhStore.TimHoacTao (dinh_danh_cong_dan.go:124-128), with the number
// the use case was handed in YeuCauMoPhienCau.SoDaXacThuc (cau_phien_cong_dan.go:347). That number
// comes from zalo.Client.Phone, which normalises Zalo's answer through zalo/wire.go normalizePhone
// (wire.go:103-119: separators removed, "+" dropped, a leading 0 replaced by 84, digits only) — the
// observed envelope at wire.go:76 is `"number": "84900000000"`. The gRPC bridge (grpc/cau_phien.go:73)
// passes the sibling repository's number through the same field, and wire.go:96 records that both
// sides normalise alike. The app layer only trims (cau_phien_cong_dan.go:221).
//
// FAILS CLOSED: anything that is not a national number (NormalizeCitizenPhone's output) answers "",
// and "" equals no stored number — a malformed input can never match somebody's identity. Callers
// that must distinguish "no number" from "bad number" call NormalizeCitizenPhone first.
func CitizenPhoneIdentityForm(national string) string {
	if !isNationalPhone(national) {
		return ""
	}
	return "84" + national[1:]
}

// CitizenContactInput is one contact as a write route receives it, and — after CheckCitizenContactInput —
// as the store writes it. ResidentialUnitID "" means none (stored NULL).
type CitizenContactInput struct {
	FullName          string
	Phone             string
	ResidentialUnitID string
	Address           string
	Note              string
	Tags              []string
	IsActive          bool
	// Email "" = none (ADR 0101 #6). Attribute values are checked by CheckCitizenAttributes against the
	// commune's definitions, which only the use case can read.
	Email string
}

// CheckCitizenContactInput applies every shape rule and returns the NORMALISED input: name with
// whitespace collapsed, phone in national form, address and note trimmed, tags cleaned
// (NormalizeCitizenContactTags), residential unit id trimmed. The first refusal is returned; the
// Excel reader applies the same rules column by column so it can name the column.
//
// IT DOES NOT CHECK THAT THE RESIDENTIAL UNIT EXISTS in this commune — that is the composite foreign
// key's job inside the transaction (migration 0033 citizen_contact_residential_unit_fkey), where the
// answer cannot go stale. What is checked is the SHAPE, so a megabyte of text never reaches a key
// comparison.
func CheckCitizenContactInput(in CitizenContactInput) (CitizenContactInput, error) {
	out := in
	var err error
	if out.FullName, err = NormalizeCitizenContactFullName(in.FullName); err != nil {
		return CitizenContactInput{}, err
	}
	if out.Phone, err = NormalizeCitizenPhone(in.Phone); err != nil {
		return CitizenContactInput{}, err
	}
	if out.ResidentialUnitID, err = NormalizeCitizenContactUnitRef(in.ResidentialUnitID); err != nil {
		return CitizenContactInput{}, err
	}
	if out.Address, err = NormalizeCitizenContactAddress(in.Address); err != nil {
		return CitizenContactInput{}, err
	}
	if out.Note, err = NormalizeCitizenContactNote(in.Note); err != nil {
		return CitizenContactInput{}, err
	}
	if out.Tags, err = NormalizeCitizenContactTags(in.Tags); err != nil {
		return CitizenContactInput{}, err
	}
	if out.Email, err = NormalizeCitizenContactEmail(in.Email); err != nil {
		return CitizenContactInput{}, err
	}
	return out, nil
}

// NormalizeCitizenContactFullName trims and collapses whitespace (the ChuanHoaHoTen treatment — an
// Excel paste leaves doubled spaces, and "Nguyễn  Văn A" would search as a different person).
// REQUIRED: the 0033 CHECK refuses a blank name; refusing here produces a sentence instead.
func NormalizeCitizenContactFullName(raw string) (string, error) {
	name := gomKhoangTrang(raw)
	if name == "" {
		return "", ErrCitizenContactFullNameMissing
	}
	if utf8.RuneCountInString(name) > MaxCitizenContactFullName {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenContactFullNameTooLong, MaxCitizenContactFullName)
	}
	return name, nil
}

// NormalizeCitizenContactAddress trims and bounds the OPTIONAL address. Internal whitespace is kept:
// this is prose nothing matches on exactly (the search folds it in SQL).
func NormalizeCitizenContactAddress(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > MaxCitizenContactAddress {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenContactAddressTooLong, MaxCitizenContactAddress)
	}
	return s, nil
}

// NormalizeCitizenContactNote trims and bounds the OPTIONAL note.
func NormalizeCitizenContactNote(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > MaxCitizenContactNote {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenContactNoteTooLong, MaxCitizenContactNote)
	}
	return s, nil
}

// NormalizeCitizenContactTags cleans the free labels of one row: each trimmed with whitespace
// collapsed, blanks dropped, repeats dropped (case-insensitively — "Hộ nghèo" and "hộ nghèo" are one
// label, the first spelling kept), the count and each length bounded. Empty in, empty (non-nil) out,
// so the store always writes an array and never NULL.
//
// A COMMA INSIDE A TAG IS REFUSED: the form and the Excel column both separate labels with commas
// (chapter 16 §5, §7), and a stored label holding one would be read back as two.
func NormalizeCitizenContactTags(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, r := range raw {
		tag := gomKhoangTrang(r)
		if tag == "" {
			continue
		}
		if strings.Contains(tag, ",") {
			return nil, ErrCitizenContactTagHasComma
		}
		if utf8.RuneCountInString(tag) > MaxCitizenContactTagLen {
			return nil, fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenContactTagTooLong, MaxCitizenContactTagLen)
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
		if len(out) > MaxCitizenContactTags {
			return nil, fmt.Errorf("%w (tối đa %d)", ErrCitizenContactTooManyTags, MaxCitizenContactTags)
		}
	}
	return out, nil
}

// SplitCitizenContactTags turns the comma-separated text of the form's tag box or the Excel "Nhãn"
// cell into the list NormalizeCitizenContactTags takes. Splitting is the ONLY thing it does, so a
// caller cannot skip the rules by choosing this entry point.
func SplitCitizenContactTags(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return strings.Split(text, ",")
}

// NormalizeCitizenContactUnitRef trims a residential-unit id the client supplies: "" (none) or
// ULID-shaped — 26 characters of Crockford's base32, the alphabet core/ulid and migration 0033's
// `length(id) = 26` CHECK agree on. Whether it names a live unit of THIS commune is the transaction's
// question (CheckCitizenContactInput).
func NormalizeCitizenContactUnitRef(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", nil
	}
	if !isULIDShaped(id) {
		return "", ErrCitizenContactUnitRefInvalid
	}
	return id, nil
}

// ulidAlphabet is Crockford's base32 as core/ulid writes it: digits and upper-case letters without
// I, L, O and U. Lower case is refused on purpose — no writer in this repository produces it, and an
// id that differs only in case from a stored one would miss the key and be reported as "unknown unit",
// which is the right answer for a value nothing here minted.
const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func isULIDShaped(s string) bool {
	if len(s) != 26 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(ulidAlphabet, rune(s[i])) {
			return false
		}
	}
	return true
}

// CitizenGroupInput is one group as the "Lập nhóm" form sends it.
type CitizenGroupInput struct {
	Name        string
	Description string
	SortOrder   int
}

// CheckCitizenGroupInput applies the group's shape rules and returns the normalised input. Name
// uniqueness within the commune (on the folded name, live rows) is the store's unique key, inside the
// transaction.
func CheckCitizenGroupInput(in CitizenGroupInput) (CitizenGroupInput, error) {
	out := in
	out.Name = gomKhoangTrang(in.Name)
	switch {
	case out.Name == "":
		return CitizenGroupInput{}, ErrCitizenGroupNameMissing
	case utf8.RuneCountInString(out.Name) > MaxCitizenGroupName:
		return CitizenGroupInput{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenGroupNameTooLong, MaxCitizenGroupName)
	}
	out.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(out.Description) > MaxCitizenGroupDescription {
		return CitizenGroupInput{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenGroupDescriptionTooLong, MaxCitizenGroupDescription)
	}
	if in.SortOrder < 0 || in.SortOrder > MaxCitizenGroupSortOrder {
		return CitizenGroupInput{}, ErrCitizenGroupSortOrderRange
	}
	return out, nil
}

// NormalizeCitizenContactDeleteReason trims and bounds the reason stored beside a contact's soft
// delete. MANDATORY (ADR 0097 D7; the 0033 CHECK refuses a blank one): removing a person from the
// directory is the commune ceasing to contact a citizen, and "why" must be answerable. Free text —
// the owner asked for no catalogue of reasons.
func NormalizeCitizenContactDeleteReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", ErrCitizenDirectoryReasonMissing
	}
	return boundCitizenDirectoryReason(reason)
}

// NormalizeCitizenGroupDeleteReason trims and bounds the reason beside a group's soft delete.
// OPTIONAL (ADR 0097 D7 — removing a group only tidies a catalogue): "" is returned as "" and the
// store writes NULL, which the 0033 CHECK admits; a reason that IS typed is bounded like a contact's.
func NormalizeCitizenGroupDeleteReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", nil
	}
	return boundCitizenDirectoryReason(reason)
}

func boundCitizenDirectoryReason(reason string) (string, error) {
	if utf8.RuneCountInString(reason) > CitizenDirectoryDeleteReasonMaxLen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCitizenDirectoryReasonTooLong, CitizenDirectoryDeleteReasonMaxLen)
	}
	return reason, nil
}
