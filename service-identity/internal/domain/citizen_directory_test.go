package domain

import (
	"errors"
	"strings"
	"testing"
)

// identityFormFake is the "84…" spelling `dinh_danh_cong_dan.so_dien_thoai` holds for fakeMobile —
// the observed Zalo envelope of zalo/wire.go:76, which zalo/wire.go normalizePhone stores unchanged.
// Pinned as a literal on purpose: this package cannot import internal/zalo (standard library only),
// so the agreement is proven against the documented form, not computed from the other side.
const identityFormFake = "84900000000"

func TestNormalizeCitizenPhone_OneFormForEverySpelling(t *testing.T) {
	// The owner's card lists "0905 123 456"-style spellings; the SAME shapes are exercised on the
	// agreed fake number (rule 3, invariant 5) — only the digits differ.
	for _, raw := range []string{
		fakeMobile,
		"0900 000 000",
		"+84900000000",
		identityFormFake,
		"0900.000.000",
		"(+84) 900-000-000",
		"+84 900 000 000",
		"0084900000000",
		" 0900000000 ",
		"84 900 000 000",
	} {
		got, err := NormalizeCitizenPhone(raw)
		if err != nil || got != fakeMobile {
			t.Errorf("NormalizeCitizenPhone(%q) = %q, %v; muốn %q", raw, got, err, fakeMobile)
		}
	}
}

func TestNormalizeCitizenPhone_KeepsALeading084(t *testing.T) {
	// "084…" is a real Vinaphone prefix: a number that already starts with 0 must never have its "84"
	// read as the country code. Built by concatenation so no literal looks like a real subscriber.
	national := "084" + "0000000"
	if got, err := NormalizeCitizenPhone(national); err != nil || got != national {
		t.Errorf("số bắt đầu 084 bị đổi: %q, %v", got, err)
	}
	// "84" + "84…": the country code followed by an 084 number folds to the 084 number.
	if got, err := NormalizeCitizenPhone("84" + national[1:]); err != nil || got != national {
		t.Errorf("84 + số 084 phải về số 084: %q, %v", got, err)
	}
}

func TestNormalizeCitizenPhone_Refusals(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"trống", "", ErrCitizenPhoneMissing},
		{"chỉ khoảng trắng", "   ", ErrCitizenPhoneMissing},
		{"chữ", "abc", ErrCitizenPhoneInvalid},
		{"chữ lẫn số", "0900000000a", ErrCitizenPhoneInvalid},
		{"dấu + ở giữa", "0900+000000", ErrCitizenPhoneInvalid},
		{"quá ngắn", "0900", ErrCitizenPhoneNotNational},
		{"không có số 0 đầu", "900000000", ErrCitizenPhoneNotNational},
		{"quá 15 chữ số", "0" + strings.Repeat("9", 15), ErrCitizenPhoneNotNational},
		{"quá 32 ký tự", strings.Repeat("0 ", 17), ErrCitizenPhoneTooLong},
		{"chỉ dấu", "+()-.", ErrCitizenPhoneNotNational},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeCitizenPhone(c.raw)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if got != "" {
				t.Errorf("bị từ chối mà vẫn trả giá trị %q", got)
			}
			if typed := strings.TrimSpace(c.raw); typed != "" && strings.Contains(err.Error(), typed) {
				t.Errorf("lỗi lặp lại giá trị bị từ chối: %q", err)
			}
		})
	}
}

func TestCitizenPhoneIdentityForm_RoundTripBothWays(t *testing.T) {
	// national → identity: the exact string dinh_danh_cong_dan holds.
	if got := CitizenPhoneIdentityForm(fakeMobile); got != identityFormFake {
		t.Errorf("IdentityForm(%q) = %q, muốn %q", fakeMobile, got, identityFormFake)
	}
	// identity → national: what Zalo stored normalises back to the directory's form.
	if got, err := NormalizeCitizenPhone(identityFormFake); err != nil || got != fakeMobile {
		t.Errorf("Normalize(%q) = %q, %v; muốn %q", identityFormFake, got, err, fakeMobile)
	}
	// Both compositions are identities on their own domain.
	if got, err := NormalizeCitizenPhone(CitizenPhoneIdentityForm(fakeMobile)); err != nil || got != fakeMobile {
		t.Errorf("Normalize(IdentityForm(n)) = %q, %v", got, err)
	}
	if got := CitizenPhoneIdentityForm(mustNormalizePhone(t, identityFormFake)); got != identityFormFake {
		t.Errorf("IdentityForm(Normalize(i)) = %q", got)
	}
	// Every spelling a person may type lands on the same identity form, i.e. the Zalo join is an
	// equality whatever was typed.
	for _, raw := range []string{"+84 900 000 000", "0900.000.000", "(0900) 000 000"} {
		if got := CitizenPhoneIdentityForm(mustNormalizePhone(t, raw)); got != identityFormFake {
			t.Errorf("IdentityForm(Normalize(%q)) = %q", raw, got)
		}
	}
}

func TestCitizenPhoneIdentityForm_FailsClosed(t *testing.T) {
	// Anything that is not a national number answers "", which equals no stored number.
	for _, bad := range []string{"", identityFormFake, "+84900000000", "900000000", "0900", "0900 000 000", "abc"} {
		if got := CitizenPhoneIdentityForm(bad); got != "" {
			t.Errorf("IdentityForm(%q) = %q, muốn rỗng", bad, got)
		}
	}
}

func mustNormalizePhone(t *testing.T, raw string) string {
	t.Helper()
	n, err := NormalizeCitizenPhone(raw)
	if err != nil {
		t.Fatalf("NormalizeCitizenPhone(%q): %v", raw, err)
	}
	return n
}

func TestCitizenContactSource(t *testing.T) {
	if !CitizenContactSourceManual.Valid() || !CitizenContactSourceExcel.Valid() {
		t.Error("hai giá trị của CHECK 0033 phải hợp lệ")
	}
	if CitizenContactSource("manual").Valid() || CitizenContactSource("").Valid() || CitizenContactSource("mini-app").Valid() {
		t.Error("giá trị ngoài CHECK 0033 phải bị từ chối — giá trị thứ ba là điều kiện dừng ADR 0097 #4")
	}
	if string(CitizenContactSourceManual) != "nhap-tay" || string(CitizenContactSourceExcel) != "nhap-excel" {
		t.Error("giá trị lưu phải đúng chữ của CHECK citizen_contact_source_valid")
	}
}

func TestCheckCitizenContactInput_Normalises(t *testing.T) {
	unit := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	got, err := CheckCitizenContactInput(CitizenContactInput{
		FullName:          "  Nguyễn   Văn\tA ",
		Phone:             "+84 900 000 000",
		ResidentialUnitID: " " + unit + " ",
		Address:           "  Số 1, đường Lê Lợi  ",
		Note:              " ghi chú ",
		Tags:              []string{" hộ nghèo ", "Hộ Nghèo", "", "tổ  trưởng"},
		IsActive:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.FullName != "Nguyễn Văn A" || got.Phone != fakeMobile || got.ResidentialUnitID != unit ||
		got.Address != "Số 1, đường Lê Lợi" || got.Note != "ghi chú" || !got.IsActive {
		t.Errorf("chuẩn hoá sai: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "hộ nghèo" || got.Tags[1] != "tổ trưởng" {
		t.Errorf("nhãn: %q", got.Tags)
	}

	// Optional fields empty: valid, and the tags come back as an empty non-nil slice.
	minimal, err := CheckCitizenContactInput(CitizenContactInput{FullName: "A", Phone: fakeMobile})
	if err != nil || minimal.Tags == nil || len(minimal.Tags) != 0 || minimal.ResidentialUnitID != "" {
		t.Errorf("tối thiểu: %+v, %v", minimal, err)
	}
}

func TestCheckCitizenContactInput_Refusals(t *testing.T) {
	ok := CitizenContactInput{FullName: "A", Phone: fakeMobile}
	cases := []struct {
		name string
		mod  func(*CitizenContactInput)
		want error
	}{
		{"thiếu họ tên", func(in *CitizenContactInput) { in.FullName = "  " }, ErrCitizenContactFullNameMissing},
		{"họ tên quá dài", func(in *CitizenContactInput) { in.FullName = strings.Repeat("ă", MaxCitizenContactFullName+1) }, ErrCitizenContactFullNameTooLong},
		{"họ tên vừa trần", func(in *CitizenContactInput) { in.FullName = strings.Repeat("ă", MaxCitizenContactFullName) }, nil},
		{"thiếu số", func(in *CitizenContactInput) { in.Phone = "" }, ErrCitizenPhoneMissing},
		{"số sai", func(in *CitizenContactInput) { in.Phone = "09x" }, ErrCitizenPhoneInvalid},
		{"mã thôn sai dạng", func(in *CitizenContactInput) { in.ResidentialUnitID = "thon-1" }, ErrCitizenContactUnitRefInvalid},
		{"mã thôn chữ thường", func(in *CitizenContactInput) { in.ResidentialUnitID = strings.ToLower("01ARZ3NDEKTSV4RRFFQ69G5FAV") }, ErrCitizenContactUnitRefInvalid},
		{"mã thôn có chữ cấm của Crockford", func(in *CitizenContactInput) { in.ResidentialUnitID = "01ARZ3NDEKTSV4RRFFQ69G5FAI" }, ErrCitizenContactUnitRefInvalid},
		{"địa chỉ quá dài", func(in *CitizenContactInput) { in.Address = strings.Repeat("ê", MaxCitizenContactAddress+1) }, ErrCitizenContactAddressTooLong},
		{"ghi chú quá dài", func(in *CitizenContactInput) { in.Note = strings.Repeat("ơ", MaxCitizenContactNote+1) }, ErrCitizenContactNoteTooLong},
		{"nhãn có dấu phẩy", func(in *CitizenContactInput) { in.Tags = []string{"a, b"} }, ErrCitizenContactTagHasComma},
		{"nhãn quá dài", func(in *CitizenContactInput) { in.Tags = []string{strings.Repeat("ư", MaxCitizenContactTagLen+1)} }, ErrCitizenContactTagTooLong},
		{"quá nhiều nhãn", func(in *CitizenContactInput) {
			for i := 0; i <= MaxCitizenContactTags; i++ {
				in.Tags = append(in.Tags, "nhãn "+strings.Repeat("x", i))
			}
		}, ErrCitizenContactTooManyTags},
		{"nhãn trùng không tính vào trần", func(in *CitizenContactInput) {
			for i := 0; i <= MaxCitizenContactTags; i++ {
				in.Tags = append(in.Tags, "Cùng một nhãn")
			}
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := ok
			in.Tags = nil
			c.mod(&in)
			_, err := CheckCitizenContactInput(in)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, muốn %v", err, c.want)
			}
		})
	}
}

func TestSplitCitizenContactTags(t *testing.T) {
	if got := SplitCitizenContactTags("  "); got != nil {
		t.Errorf("ô trống phải là nil: %q", got)
	}
	tags, err := NormalizeCitizenContactTags(SplitCitizenContactTags("hộ nghèo, tổ trưởng ,, hộ kinh doanh"))
	if err != nil || len(tags) != 3 || tags[2] != "hộ kinh doanh" {
		t.Errorf("tách nhãn: %q, %v", tags, err)
	}
}

func TestCheckCitizenGroupInput(t *testing.T) {
	got, err := CheckCitizenGroupInput(CitizenGroupInput{Name: "  Tổ trưởng   dân phố ", Description: " mô tả ", SortOrder: 3})
	if err != nil || got.Name != "Tổ trưởng dân phố" || got.Description != "mô tả" || got.SortOrder != 3 {
		t.Errorf("chuẩn hoá: %+v, %v", got, err)
	}
	cases := []struct {
		name string
		in   CitizenGroupInput
		want error
	}{
		{"thiếu tên", CitizenGroupInput{Name: " "}, ErrCitizenGroupNameMissing},
		{"tên quá dài", CitizenGroupInput{Name: strings.Repeat("ă", MaxCitizenGroupName+1)}, ErrCitizenGroupNameTooLong},
		{"mô tả quá dài", CitizenGroupInput{Name: "A", Description: strings.Repeat("ê", MaxCitizenGroupDescription+1)}, ErrCitizenGroupDescriptionTooLong},
		{"thứ tự âm", CitizenGroupInput{Name: "A", SortOrder: -1}, ErrCitizenGroupSortOrderRange},
		{"thứ tự quá trần", CitizenGroupInput{Name: "A", SortOrder: MaxCitizenGroupSortOrder + 1}, ErrCitizenGroupSortOrderRange},
		{"thứ tự 0", CitizenGroupInput{Name: "A"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := CheckCitizenGroupInput(c.in); !errors.Is(err, c.want) {
				t.Errorf("err = %v, muốn %v", err, c.want)
			}
		})
	}
}

func TestDeleteReasons_MandatoryForContactOptionalForGroup(t *testing.T) {
	if _, err := NormalizeCitizenContactDeleteReason("  "); !errors.Is(err, ErrCitizenDirectoryReasonMissing) {
		t.Errorf("gỡ người dân không lý do phải bị từ chối: %v", err)
	}
	if got, err := NormalizeCitizenContactDeleteReason("  đã chuyển đi  "); err != nil || got != "đã chuyển đi" {
		t.Errorf("lý do người dân: %q, %v", got, err)
	}
	if got, err := NormalizeCitizenGroupDeleteReason("  "); err != nil || got != "" {
		t.Errorf("gỡ nhóm không lý do phải được (ADR 0097 D7): %q, %v", got, err)
	}
	if got, err := NormalizeCitizenGroupDeleteReason(" dọn danh mục "); err != nil || got != "dọn danh mục" {
		t.Errorf("lý do nhóm: %q, %v", got, err)
	}
	long := strings.Repeat("ă", CitizenDirectoryDeleteReasonMaxLen+1)
	if _, err := NormalizeCitizenContactDeleteReason(long); !errors.Is(err, ErrCitizenDirectoryReasonTooLong) {
		t.Errorf("lý do người dân quá dài: %v", err)
	}
	if _, err := NormalizeCitizenGroupDeleteReason(long); !errors.Is(err, ErrCitizenDirectoryReasonTooLong) {
		t.Errorf("lý do nhóm quá dài: %v", err)
	}
}

// NO SENTINEL CARRIES A VALUE, and no wrapped refusal of a personal-data field echoes the input
// (rule 3, forbidden #3).
func TestCitizenDirectoryErrors_NeverEchoTheValue(t *testing.T) {
	const secret = "Nguyễn Văn Bí Mật"
	_, err := CheckCitizenContactInput(CitizenContactInput{FullName: strings.Repeat("x", MaxCitizenContactFullName) + secret, Phone: fakeMobile})
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "xxx") {
		t.Errorf("lỗi họ tên lặp lại giá trị: %v", err)
	}
	_, err = NormalizeCitizenPhone(fakeMobile + "9x")
	if err == nil || strings.Contains(err.Error(), fakeMobile) {
		t.Errorf("lỗi số điện thoại lặp lại giá trị: %v", err)
	}
	for _, e := range []error{
		ErrCitizenContactFullNameMissing, ErrCitizenContactFullNameTooLong, ErrCitizenPhoneMissing,
		ErrCitizenPhoneInvalid, ErrCitizenPhoneTooLong, ErrCitizenPhoneNotNational,
		ErrCitizenContactAddressTooLong, ErrCitizenContactNoteTooLong, ErrCitizenContactTooManyTags,
		ErrCitizenContactTagTooLong, ErrCitizenContactTagHasComma, ErrCitizenContactUnitRefInvalid,
		ErrCitizenGroupNameMissing, ErrCitizenGroupNameTooLong, ErrCitizenGroupDescriptionTooLong,
		ErrCitizenGroupSortOrderRange, ErrCitizenDirectoryReasonMissing, ErrCitizenDirectoryReasonTooLong,
	} {
		if !strings.HasPrefix(e.Error(), "danh bạ người dân: ") {
			t.Errorf("sentinel thiếu tiền tố nhà: %q", e)
		}
	}
}

func TestTagSensitiveWarning_IsTheAgreedSentence(t *testing.T) {
	// ADR 0097 D4: the fixed sentence under the tag field, word for word.
	if TagSensitiveWarning != "Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị" {
		t.Errorf("câu cấm nhãn lệch ADR 0097 D4: %q", TagSensitiveWarning)
	}
}
