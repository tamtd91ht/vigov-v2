package page

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

// WHAT THIS FILE IS FOR: pkg/page decides where "the next page" starts for every list route in
// eight services. Three of its failures are SILENT — a cursor accepted from a different sort, a
// limit that is not actually capped, and a sort key taken from a client string — and none of them
// turn anything red on their own. The fourth, a malformed cursor, must be an ordinary 400 and
// never a panic, because it arrives from a URL that anybody can edit.
//
// The keyset walk itself (no row repeated, none skipped) is proved in pkg/store/page_test.go,
// where the statement actually runs.

var (
	cotNgayTao = Col("created_at", "ngay_tao", KindTime)
	cotMa      = Col("code", "ma", KindText)
	cotSo      = Col("arrival_no", "so_den", KindInt)
	danhSach   = NewAllowlist(Desc, cotNgayTao, cotMa, cotSo)
)

func doc(t *testing.T, raw string) (Request, error) {
	t.Helper()
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("chuỗi truy vấn hỏng trong chính bài kiểm: %v", err)
	}
	return Parse(q, danhSach)
}

// --- defaults and bounds ------------------------------------------------------------------------

func TestKhongThamSoThiDungMacDinhCuaTuyen(t *testing.T) {
	r, err := doc(t, "")
	if err != nil {
		t.Fatalf("không tham số mà vẫn lỗi: %v", err)
	}
	if r.Column().SQL != "ngay_tao" {
		t.Errorf("cột = %q, muốn ngay_tao — mặc định của tuyến", r.Column().SQL)
	}
	if r.Dir() != Desc {
		t.Errorf("chiều = %q, muốn desc", r.Dir())
	}
	if r.Limit() != DefaultLimit {
		t.Errorf("limit = %d, muốn %d", r.Limit(), DefaultLimit)
	}
	if _, co := r.After(); co {
		t.Error("trang đầu mà lại có mốc để đi tiếp")
	}
}

func TestLimitVuotTranThiKepChuKhongBaoLoi(t *testing.T) {
	// §5 #2. A client asking for 10.000 is served the cap AND a cursor to continue with, so the
	// clamp loses nothing. This is deliberately not the BatchGetStaff rule, where a silent clamp is
	// a silent wrong answer because the caller has no way to fetch the remainder.
	for _, raw := range []string{"limit=101", "limit=10000", "limit=999999999"} {
		r, err := doc(t, raw)
		if err != nil {
			t.Fatalf("%s: xin quá trần phải được kẹp, không phải lỗi: %v", raw, err)
		}
		if r.Limit() != MaxLimit {
			t.Errorf("%s: limit = %d, muốn kẹp về %d", raw, r.Limit(), MaxLimit)
		}
	}
}

func TestLimitTrongKhoangThiGiuNguyen(t *testing.T) {
	r, err := doc(t, "limit=50")
	if err != nil {
		t.Fatal(err)
	}
	if r.Limit() != 50 {
		t.Errorf("limit = %d, muốn 50", r.Limit())
	}
}

func TestLimitKhongDuongHoacKhongPhaiSoThiTuChoi(t *testing.T) {
	// The asymmetry with the case above, stated as a test: "nhiều hơn mức cho phép" is a request
	// the server can answer; "0 bản ghi" and "-5 bản ghi" are not requests for anything, and
	// silently turning them into 20 would hide the bug in whatever built the URL.
	for _, raw := range []string{"limit=0", "limit=-5", "limit=abc", "limit=2.5", "limit=%20"} {
		if _, err := doc(t, raw); !errors.Is(err, ErrLimit) {
			t.Errorf("%s: muốn ErrLimit, nhận %v", raw, err)
		}
	}
}

// --- the allowlist --------------------------------------------------------------------------------

func TestCotSapXepLayTuDanhSachTrangChuKhongPhaiChuoiKhachGui(t *testing.T) {
	r, err := doc(t, "sort=code&order=asc")
	if err != nil {
		t.Fatal(err)
	}
	if r.Column().SQL != "ma" || r.Column().Kind != KindText {
		t.Errorf("cột = %+v, muốn ma/text — tên cột phải đến từ khai báo trong mã nguồn", r.Column())
	}
	if r.Dir() != Asc {
		t.Errorf("chiều = %q, muốn asc", r.Dir())
	}
}

func TestCotNgoaiDanhSachTrangBiTuChoi(t *testing.T) {
	// The injection-shaped ones are in here on purpose: they must be refused by NOT BEING ON THE
	// LIST, long before anything considers putting them in a statement.
	for _, raw := range []string{
		"sort=ho_ten",
		"sort=dien_thoai",
		"sort=" + url.QueryEscape("ngay_tao; DROP TABLE phan_anh"),
		"sort=" + url.QueryEscape("ngay_tao--"),
		"sort=" + url.QueryEscape("(SELECT 1)"),
	} {
		if _, err := doc(t, raw); !errors.Is(err, ErrSort) {
			t.Errorf("%s: muốn ErrSort, nhận %v", raw, err)
		}
	}
}

func TestChieuSapXepNgoaiAscDescBiTuChoi(t *testing.T) {
	for _, raw := range []string{"order=ASC", "order=xuong", "order=1", "order=" + url.QueryEscape("asc;--")} {
		if _, err := doc(t, raw); !errors.Is(err, ErrSort) {
			t.Errorf("%s: muốn ErrSort, nhận %v", raw, err)
		}
	}
}

func TestColTuChoiTenCotKhongPhaiDinhDanh(t *testing.T) {
	// The last line of defence: even if a client string reached an allowlist declaration, it never
	// becomes SQL. This fails at wiring time, which is the cheapest place to find it.
	for _, ten := range []string{"ngay_tao; DROP TABLE phan_anh", "ngay tao", "NgayTao", "", "1col", "ma--"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Col chấp nhận tên cột %q — đó là đường đi của SQL injection", ten)
				}
			}()
			_ = Col("x", ten, KindText)
		}()
	}
}

func TestDanhSachTrangTrungThamSoThiPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("hai cột cùng tham số mà không báo — một trong hai sẽ không bao giờ dùng được")
		}
	}()
	_ = NewAllowlist(Asc, cotNgayTao, Col("created_at", "cap_nhat_luc", KindTime))
}

// --- the cursor ------------------------------------------------------------------------------------

func TestConTroDiVeDuCaBaKieuKhoa(t *testing.T) {
	moc := map[string]struct {
		cot Column
		a   Anchor
	}{
		"thời gian": {cotNgayTao, Anchor{Key: TimeKey(time.Date(2026, 3, 14, 9, 30, 0, 123456000, time.UTC)), ID: "01J0000000000000000000000X"}},
		"chuỗi":     {cotMa, Anchor{Key: TextKey("PA-2026-0042"), ID: "01J0000000000000000000000Y"}},
		"số":        {cotSo, Anchor{Key: IntKey(4212), ID: "01J0000000000000000000000Z"}},
	}
	for ten, tc := range moc {
		t.Run(ten, func(t *testing.T) {
			s := Encode(tc.cot, Asc, tc.a)
			ve, err := Decode(s, tc.cot, Asc)
			if err != nil {
				t.Fatalf("giải mã lỗi: %v", err)
			}
			if !ve.Key.Equal(tc.a.Key) {
				t.Errorf("khoá = %v, muốn %v", ve.Key, tc.a.Key)
			}
			if ve.ID != tc.a.ID {
				t.Errorf("id = %q, muốn %q", ve.ID, tc.a.ID)
			}
			if ve.Key.Kind() != tc.cot.Kind {
				t.Errorf("kiểu = %q, muốn %q — giá trị phải về đúng kiểu để RÀNG BUỘC, không phải nối chuỗi",
					ve.Key.Kind(), tc.cot.Kind)
			}
		})
	}
}

func TestConTroLaChuoiMoKhongPhaiOffset(t *testing.T) {
	// If a cursor were readable as a number, a client would send "cursor=500" and choose where to
	// start reading. Opaque is the point; base64url is so it survives a query string untouched.
	s := Encode(cotNgayTao, Desc, Anchor{Key: TimeKey(time.Now()), ID: "01J0000000000000000000000X"})
	if s == "" {
		t.Fatal("con trỏ rỗng")
	}
	if s != url.QueryEscape(s) {
		t.Errorf("con trỏ %q phải đi qua query string mà không cần thoát ký tự", s)
	}
	if strings.ContainsAny(s, "+/=") {
		t.Errorf("con trỏ %q dùng base64 chuẩn — phải là base64url không đệm", s)
	}
}

func TestConTroKhongBaoGioMangMaXa(t *testing.T) {
	// Rule 1, forbidden #2: a cursor carrying tenant_id is a client naming its own commune, and it
	// would survive being pasted into another commune's domain. The commune comes from the context
	// and is bound to $1 by pkg/store — there is no second source.
	xa := "01J0000000000000000000000A"
	s := Encode(cotNgayTao, Desc, Anchor{Key: TimeKey(time.Now()), ID: "01J0000000000000000000000X"})
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	var truong map[string]any
	if err := json.Unmarshal(raw, &truong); err != nil {
		t.Fatal(err)
	}
	for k, v := range truong {
		if strings.Contains(strings.ToLower(k), "tenant") || strings.Contains(strings.ToLower(k), "xa") {
			t.Errorf("con trỏ mang trường %q = %v — xã không bao giờ nằm trong con trỏ", k, v)
		}
		if s, ok := v.(string); ok && s == xa {
			t.Errorf("con trỏ mang mã xã ở trường %q", k)
		}
	}

	// And a cursor that has one BOLTED ON must be refused, not silently ignored: silently ignoring
	// it means the field is there, in logs and in bug reports, looking like it works.
	them, _ := json.Marshal(map[string]any{
		"v": 1, "c": "created_at", "d": "desc", "t": "time",
		"k": time.Now().UTC().Format(time.RFC3339Nano), "i": "01J0000000000000000000000X",
		"tenant_id": xa,
	})
	if _, err := Decode(base64.RawURLEncoding.EncodeToString(them), cotNgayTao, Desc); !errors.Is(err, ErrCursor) {
		t.Errorf("con trỏ có trường lạ vẫn được nhận: %v", err)
	}
}

func TestConTroRacBiTuChoiRoRangVaKhongPanic(t *testing.T) {
	hopLe := func(m map[string]any) string {
		b, _ := json.Marshal(m)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	cases := map[string]string{
		"không phải base64":        "!!!khong-phai-base64!!!",
		"base64 có đệm":            base64.StdEncoding.EncodeToString([]byte(`{"v":1}`)),
		"không phải json":          base64.RawURLEncoding.EncodeToString([]byte("khong phai json")),
		"json rỗng":                base64.RawURLEncoding.EncodeToString([]byte(`{}`)),
		"mảng json":                base64.RawURLEncoding.EncodeToString([]byte(`[1,2,3]`)),
		"phiên bản khác":           hopLe(map[string]any{"v": 99, "c": "created_at", "d": "desc", "t": "time", "k": "2026-03-14T09:30:00Z", "i": "x"}),
		"thiếu khoá phá hoà":       hopLe(map[string]any{"v": 1, "c": "created_at", "d": "desc", "t": "time", "k": "2026-03-14T09:30:00Z", "i": ""}),
		"mốc không phải thời gian": hopLe(map[string]any{"v": 1, "c": "created_at", "d": "desc", "t": "time", "k": "hom qua", "i": "x"}),
		"kiểu khoá sai cột":        hopLe(map[string]any{"v": 1, "c": "created_at", "d": "desc", "t": "text", "k": "abc", "i": "x"}),
		"tiêm sql trong mốc":       hopLe(map[string]any{"v": 1, "c": "created_at", "d": "desc", "t": "time", "k": "2026-03-14T09:30:00Z') OR 1=1 --", "i": "x"}),
		"tiêm sql trong id":        hopLe(map[string]any{"v": 1, "c": "created_at", "d": "desc", "t": "text", "k": "abc", "i": "x'); DROP TABLE phan_anh --"}),
	}
	for ten, s := range cases {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("PANIC trên chuỗi từ URL: %v — con trỏ hỏng là lỗi 400 bình thường", r)
				}
			}()
			if _, err := Decode(s, cotNgayTao, Desc); !errors.Is(err, ErrCursor) {
				t.Fatalf("muốn ErrCursor, nhận %v", err)
			}
		})
	}
}

func TestConTroCuaCachSapXepKhacBiTuChoi(t *testing.T) {
	// A cursor made while sorting by ngay_tao, replayed against a sort by ma, anchors the walk at a
	// position that means nothing in the new order: the client gets a list that skips records and
	// repeats others, with no error anywhere. One restart is cheaper than a list nobody can trust.
	s := Encode(cotNgayTao, Desc, Anchor{Key: TimeKey(time.Now()), ID: "01J0000000000000000000000X"})

	if _, err := Decode(s, cotMa, Desc); !errors.Is(err, ErrCursor) {
		t.Errorf("con trỏ của cột khác vẫn được nhận: %v", err)
	}
	if _, err := Decode(s, cotNgayTao, Asc); !errors.Is(err, ErrCursor) {
		t.Errorf("con trỏ của chiều khác vẫn được nhận: %v", err)
	}
	if _, err := Decode(s, cotNgayTao, Desc); err != nil {
		t.Errorf("đúng cột đúng chiều mà bị từ chối: %v", err)
	}
}

func TestParseTuChoiConTroRacTruocKhiChamCSDL(t *testing.T) {
	r, err := doc(t, "cursor=khong-phai-con-tro")
	if !errors.Is(err, ErrCursor) {
		t.Fatalf("muốn ErrCursor, nhận %v", err)
	}
	if r.Column().SQL != "" || r.Limit() != 0 {
		t.Error("Parse lỗi mà vẫn trả về một yêu cầu dùng được — người gọi sẽ chạy câu lệnh với nó")
	}
}

// --- the error shape ------------------------------------------------------------------------------

func TestMaLoiTiengAnhThongBaoTiengViet(t *testing.T) {
	cases := map[error]string{
		ErrCursor: "invalid_cursor",
		ErrSort:   "invalid_sort",
		ErrLimit:  "invalid_limit",
	}
	for err, ma := range cases {
		st, code, msg := HTTPError(err)
		if st != 400 {
			t.Errorf("%v: status = %d, muốn 400", err, st)
		}
		if code != ma {
			t.Errorf("%v: code = %q, muốn %q", err, code, ma)
		}
		if msg == "" || msg == code {
			t.Errorf("%v: thông báo cho người đọc không được rỗng", err)
		}
		if !coDauTiengViet(msg) {
			t.Errorf("%v: thông báo %q không phải tiếng Việt có dấu", err, msg)
		}
	}
}

func coDauTiengViet(s string) bool {
	for _, ky := range []string{"ă", "â", "ê", "ô", "ơ", "ư", "đ", "ả", "ợ", "ụ", "ố", "ỏ", "ệ"} {
		if strings.Contains(s, ky) {
			return true
		}
	}
	return false
}

// --- the response shape ---------------------------------------------------------------------------

func TestPhanHoiCoBaTruongVaKhongCoTong(t *testing.T) {
	// §5 #4 and requirement 7: a keyset read never computes a total, so any `total` here would come
	// from a COUNT(*) over a 32-way partitioned table on every list request, or from an invention.
	kq := NewResult[string]()
	kq.Items = append(kq.Items, "a")
	kq.NextCursor = "abc"
	kq.HasMore = true

	b, err := json.Marshal(kq)
	if err != nil {
		t.Fatal(err)
	}
	var truong map[string]json.RawMessage
	if err := json.Unmarshal(b, &truong); err != nil {
		t.Fatal(err)
	}
	if len(truong) != 3 {
		t.Errorf("phản hồi có %d trường: %s", len(truong), b)
	}
	for _, ten := range []string{"items", "next_cursor", "has_more"} {
		if _, co := truong[ten]; !co {
			t.Errorf("thiếu trường %q: %s", ten, b)
		}
	}
	for _, cam := range []string{"total", "page", "page_count", "count", "offset"} {
		if _, co := truong[cam]; co {
			t.Errorf("phản hồi có trường %q — hình dạng con trỏ không biết con số đó", cam)
		}
	}
}

func TestDanhSachRongLaMangRongChuKhongPhaiNull(t *testing.T) {
	b, err := json.Marshal(NewResult[string]())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"items":[]`) {
		t.Errorf("danh sách rỗng mã hoá thành %s — client phải lặp được mà không kiểm null", b)
	}
}
