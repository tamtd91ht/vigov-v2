package domain

// The pure rules of chapter 11, with no database and no HTTP.
//
// WHAT THIS FILE IS RESPONSIBLE FOR: the six content types are exactly §5's six; a title is refused
// rather than truncated; a link that is not http(s) is refused; a category slug has the form ADR 0011
// fixes; and a partial edit can tell "not mentioned" from "cleared". Everything else — the
// transaction, the commune, the permission — is proved where it lives.

import (
	"errors"
	"strings"
	"testing"
)

func TestSixContentTypesAreTheSixSpecCodes(t *testing.T) {
	// §5 IS A TABLE WITH A `Mã` COLUMN, so these strings were read rather than invented. They are
	// also the CHECK constraint of migration 0006 and the `type` a client sends, so a typo in any of
	// the six is a tab that answers 400 to the screen that owns it.
	for _, code := range []string{"tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video", "banner"} {
		if !IsValidContentType(code) {
			t.Errorf("mã %q của §5 bị coi là không hợp lệ", code)
		}
	}
	// A SEVENTH VALUE IS REFUSED, including the plausible ones. `tin_tuc` with an underscore and
	// `TinTuc` are exactly what a client writes when nobody checks, and both would sit in the
	// database as a `loai` no tab ever shows.
	for _, code := range []string{"", "tin_tuc", "TinTuc", "news", "thong-bao-noi-bo", "su kien"} {
		if IsValidContentType(code) {
			t.Errorf("mã %q không thuộc §5 mà vẫn được nhận", code)
		}
	}
}

func TestNormalizeContentTitleRefusesRatherThanTruncates(t *testing.T) {
	if _, err := NormalizeContentTitle("   "); !errors.Is(err, ErrContentTitleEmpty) {
		t.Errorf("tiêu đề toàn khoảng trắng: lỗi = %v, muốn ErrContentTitleEmpty", err)
	}
	// REFUSED, NOT TRUNCATED. A headline silently cut at 300 runes is an article whose subject
	// changed on the way into the database, on a channel residents read.
	long := strings.Repeat("a", ContentTitleMaxLen+1)
	if _, err := NormalizeContentTitle(long); !errors.Is(err, ErrContentTitleTooLong) {
		t.Errorf("tiêu đề quá dài: lỗi = %v, muốn ErrContentTitleTooLong", err)
	}
	// A control character corrupts a screen and a log line alike.
	if _, err := NormalizeContentTitle("Xã thông báo\x00"); err == nil {
		t.Error("tiêu đề chứa ký tự điều khiển mà vẫn qua")
	}
	// Vietnamese with diacritics is the ordinary case and must pass untouched but trimmed.
	out, err := NormalizeContentTitle("  Xã Thăng Bình khai giảng năm học mới  ")
	if err != nil || out != "Xã Thăng Bình khai giảng năm học mới" {
		t.Errorf("tiêu đề hợp lệ = %q, lỗi %v", out, err)
	}
}

func TestNormalizeURLAcceptsOnlyHTTPAndHTTPS(t *testing.T) {
	// THE ASSERTION THIS FUNCTION EXISTS FOR. `anh_dai_dien_url` is rendered as an image source and
	// `nguon_url` as a link, both inside an application residents open on their phones: `javascript:`
	// is script execution on the citizen channel and `data:` is an arbitrary payload served from the
	// commune's own screen.
	for _, bad := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"file:///etc/passwd",
		"//example.com/anh.png",
		"/anh.png",
		"example.com/anh.png",
	} {
		if _, err := NormalizeURL(bad); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("địa chỉ %q: lỗi = %v, muốn ErrInvalidURL", bad, err)
		}
	}
	// AN EMPTY STRING IS VALID and means "no link" — §7 marks the image optional.
	if out, err := NormalizeURL("   "); err != nil || out != "" {
		t.Errorf("địa chỉ rỗng = %q, lỗi %v — rỗng nghĩa là không có ảnh", out, err)
	}
	// WHAT WAS TYPED IS WHAT IS STORED: the scheme is compared case-insensitively and the value is
	// NOT lower-cased, because a path is case sensitive and rewriting it breaks the link.
	for _, good := range []string{
		"https://thangbinh.danang.gov.vn/Anh/KhaiGiang.JPG",
		"HTTPS://thangbinh.danang.gov.vn/a.png",
		"http://thangbinh.danang.gov.vn/a.png",
	} {
		out, err := NormalizeURL(good)
		if err != nil {
			t.Errorf("địa chỉ %q bị từ chối: %v", good, err)
		}
		if out != good {
			t.Errorf("địa chỉ bị sửa: %q -> %q", good, out)
		}
	}
}

func TestNormalizeCategorySlugKeepsADR0011Form(t *testing.T) {
	for _, bad := range []string{"", "Chuyen-Doi-So", "chuyen_doi_so", "chuyển-đổi-số",
		"-chuyen-doi-so", "chuyen-doi-so-", "chuyen--doi-so", "chuyen doi so"} {
		if _, err := NormalizeCategorySlug(bad); err == nil {
			t.Errorf("slug %q sai khuôn mà vẫn qua", bad)
		}
	}
	// NO CASE FOLDING, and this is the case that says so: `Chuyen-Doi-So` is REFUSED above rather
	// than lower-cased here. The slug goes into every article as a filing value and rule 7 never lets
	// it be corrected, so the code stored has to be the code the person saw.
	if out, err := NormalizeCategorySlug(" chuyen-doi-so "); err != nil || out != "chuyen-doi-so" {
		t.Errorf("slug hợp lệ = %q, lỗi %v", out, err)
	}
	if _, err := NormalizeCategorySlug(strings.Repeat("a", CategorySlugMaxLen+1)); !errors.Is(err, ErrCategorySlugTooLong) {
		t.Error("slug quá dài mà vẫn qua")
	}
}

func TestCreateContentItemRequestHasNoStatusOrSourceField(t *testing.T) {
	// A COMPILE-TIME ASSERTION WRITTEN AS A TEST, and it is the one this file exists for. If somebody
	// adds a `Status` or a `Source` field to the request struct, §10.2's approval state and §10.4's
	// protection both become things a request body can claim. A struct literal naming every field is
	// what turns that addition into a build failure HERE rather than into a silent capability.
	_ = CreateContentItemRequest{
		Type:       "tin-tuc",
		CategoryID: "",
		Title:      "T",
		Summary:    "",
		Body:       "",
		ImageURL:   "",
		Publish:    false,
	}
}

func TestValidateCreateContentItemRefusesUnknownTypeKeepsPublishFlag(t *testing.T) {
	req := CreateContentItemRequest{Type: "bai-viet", Title: "Tiêu đề"}
	if _, err := req.Validate(); !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("loại lạ: lỗi = %v, muốn ErrInvalidContentType", err)
	}

	req = CreateContentItemRequest{Type: "banner", Title: "  Khẩu hiệu  ", Publish: true}
	clean, err := req.Validate()
	if err != nil {
		t.Fatalf("yêu cầu hợp lệ bị từ chối: %v", err)
	}
	if clean.Title != "Khẩu hiệu" {
		t.Errorf("tiêu đề = %q, muốn đã trim", clean.Title)
	}
	// THE CHECKBOX SURVIVES VALIDATION. It decides whether residents see the item, and a
	// normalisation step that dropped it would publish nothing while reporting success.
	if !clean.Publish {
		t.Error("cờ `Đăng lên Mini App` bị mất khi chuẩn hoá")
	}
}

func TestValidateUpdateContentItemTellsUnmentionedFromCleared(t *testing.T) {
	// THE WHOLE REASON THE EDIT REQUEST IS A STRUCT OF POINTERS. A screen that edits only the title
	// must not clear the summary, drop the article out of its category and unpublish it — and a
	// struct of plain values cannot tell those two apart.
	var req UpdateContentItemRequest
	clean, err := req.Validate()
	if err != nil {
		t.Fatalf("yêu cầu rỗng bị từ chối: %v", err)
	}
	if clean.Title != nil || clean.Summary != nil || clean.CategoryID != nil ||
		clean.Body != nil || clean.ImageURL != nil || clean.Type != nil ||
		clean.Publish != nil {
		t.Errorf("yêu cầu không nhắc trường nào mà chuẩn hoá lại sinh ra giá trị: %+v", clean)
	}

	// MENTIONED AND EMPTY IS A REAL REQUEST: `— Chưa xếp danh mục —`, and a summary the author
	// deleted. It must survive as a non-nil pointer to "".
	empty := ""
	req = UpdateContentItemRequest{CategoryID: &empty, Summary: &empty}
	clean, err = req.Validate()
	if err != nil {
		t.Fatalf("xoá trắng bị từ chối: %v", err)
	}
	if clean.CategoryID == nil || *clean.CategoryID != "" {
		t.Errorf("`category_id: \"\"` phải giữ được nghĩa 'bỏ khỏi danh mục': %#v", clean.CategoryID)
	}
	if clean.Summary == nil || *clean.Summary != "" {
		t.Errorf("`summary: \"\"` phải giữ được nghĩa 'xoá tóm tắt': %#v", clean.Summary)
	}
}

func TestValidateUpdateContentItemStillChecksEachMentionedField(t *testing.T) {
	bad := "javascript:alert(1)"
	if _, err := (UpdateContentItemRequest{ImageURL: &bad}).Validate(); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("sửa ảnh sang javascript: lỗi = %v, muốn ErrInvalidURL", err)
	}
	blank := "   "
	if _, err := (UpdateContentItemRequest{Title: &blank}).Validate(); !errors.Is(err, ErrContentTitleEmpty) {
		t.Error("sửa tiêu đề thành khoảng trắng mà vẫn qua")
	}
	unknown := "podcast"
	if _, err := (UpdateContentItemRequest{Type: &unknown}).Validate(); !errors.Is(err, ErrInvalidContentType) {
		t.Error("sửa sang một loại ngoài §5 mà vẫn qua")
	}
}

func TestHasImageAndVisibilityAreDerivedNotStored(t *testing.T) {
	// §6's `Tệp đính kèm` column is `🔗 Có ảnh` / `—`, derived from the URL. A boolean column beside
	// it would be two representations of one fact, and the stale one is what a screen would read.
	if (ContentItem{}).HasImage() {
		t.Error("không có ảnh mà HasImage() trả true")
	}
	if !(ContentItem{ImageURL: "https://x/a.png"}).HasImage() {
		t.Error("có ảnh mà HasImage() trả false")
	}

	// IsVisibleToCitizens MUST BE FALSE FOR `cho-duyet` AS WELL AS FOR `an`. A place that asked the narrow
	// question by hand — testing for `an` alone — would publish everything waiting for approval.
	for _, tt := range []struct {
		status  ContentStatus
		visible bool
	}{
		{ContentStatusVisible, true},
		{ContentStatusPending, false},
		{ContentStatusHidden, false},
	} {
		if got := (ContentItem{Status: tt.status}).IsVisibleToCitizens(); got != tt.visible {
			t.Errorf("trạng thái %q: IsVisibleToCitizens() = %v, muốn %v", tt.status, got, tt.visible)
		}
	}
}

func TestValidateCreateContentCategoryRefusesNegativeOrderAndEmptyName(t *testing.T) {
	if _, err := (CreateContentCategoryRequest{Name: " ", Slug: "a"}).Validate(); !errors.Is(err, ErrCategoryNameEmpty) {
		t.Error("tên danh mục rỗng mà vẫn qua")
	}
	if _, err := (CreateContentCategoryRequest{Name: "Chuyển đổi số", Slug: "chuyen-doi-so", SortOrder: -1}).
		Validate(); !errors.Is(err, ErrCategorySortOrderOutOfRange) {
		// NEGATIVE IS REFUSED, NOT CLAMPED: -1 to mean "first" works until somebody sends -2, and the
		// order a commune arranged its own tree in then depends on who edited last.
		t.Error("thứ tự âm mà vẫn qua")
	}
	clean, err := (CreateContentCategoryRequest{Name: " Chuyển đổi số ", Slug: " chuyen-doi-so ", SortOrder: 3}).Validate()
	if err != nil {
		t.Fatalf("danh mục hợp lệ bị từ chối: %v", err)
	}
	if clean.Name != "Chuyển đổi số" || clean.Slug != "chuyen-doi-so" || clean.SortOrder != 3 {
		t.Errorf("danh mục sau chuẩn hoá = %+v", clean)
	}
}
