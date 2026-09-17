package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// VÌ SAO TỆP NÀY TỒN TẠI
//
// `TenantMiddleware` là nơi luật 1 bất biến 3 được thực thi: xã suy từ `Host` ở RÌA NGOÀI
// CÙNG, trước mọi handler, và không phân giải được thì từ chối. Mọi tuyến của cả tám dịch vụ
// đi qua đúng hàm này.
//
// Nó không có một bài test nào cho tới 2026-09-17. Điều đó đáng nói không phải vì thiếu sót
// nói chung, mà vì hỏng ở đây KHÔNG gây lỗi: một biên rơi về xã mặc định vẫn phục vụ bình
// thường, mọi màn hình vẫn chạy, và thứ duy nhất khác đi là dữ liệu xã này hiện trên tên miền
// xã kia. Không có gì đỏ để ai nhìn thấy.

const (
	xaA   = "01J0000000000000000000000A"
	xaB   = "01J0000000000000000000000B"
	hostA = "thangbinh.vigov.vn"
	hostB = "binhduong.vigov.vn"
	hostC = "dasapnhap.vigov.vn" // đã ngừng hoạt động
)

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

func thuMucMau() thuMucGia {
	return thuMucGia{
		hostA: {ID: xaA, Host: hostA, Name: "Xã Thăng Bình", Active: true},
		hostB: {ID: xaB, Host: hostB, Name: "Xã Bình Dương", Active: true},
		// Xã sáp nhập: dữ liệu còn nguyên (luật 7 bất biến 6), nhưng không phục vụ tiếp.
		hostC: {ID: "01J0000000000000000000000C", Host: hostC, Name: "Xã Đã Sáp Nhập", Active: false},
	}
}

// goi chạy một yêu cầu qua đúng chuỗi biên thật.
func goi(h http.Handler, host string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://"+host+"/bat-ky", nil)
	r.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func bien(sau http.HandlerFunc) http.Handler {
	return TenantMiddleware(thuMucMau())(sau)
}

func TestHostKhongThuocXaNaoThiTuChoi(t *testing.T) {
	var chay bool
	h := bien(func(http.ResponseWriter, *http.Request) { chay = true })

	w := goi(h, "khong-ai-biet.example.gov.vn")
	if w.Code != http.StatusNotFound {
		t.Fatalf("mã = %d, muốn 404", w.Code)
	}
	if chay {
		t.Fatal("handler ĐÃ CHẠY cho một Host không thuộc xã nào — mọi truy vấn sau đó sẽ " +
			"không có xã để giới hạn")
	}
}

// BÀI QUAN TRỌNG NHẤT TỆP NÀY.
//
// Xã đã sáp nhập và tên miền chưa bao giờ tồn tại phải nhận CÙNG MỘT câu trả lời, CÙNG một mã
// lỗi. Trả khác nhau nghe tử tế hơn với xã sáp nhập, nhưng nó chính là chỗ rò: người gõ thử
// một loạt tên miền sẽ phân biệt được "chưa từng có" với "từng có", tức dò ra danh sách xã
// trên nền tảng — một danh sách không ai được phép lấy bằng cách đoán.
func TestXaNgungHoatDongTraLoiGiongHetTenMienKhongTonTai(t *testing.T) {
	h := bien(func(http.ResponseWriter, *http.Request) {})

	daSapNhap := goi(h, hostC)
	khongTonTai := goi(h, "khong-ai-biet.example.gov.vn")

	if daSapNhap.Code != khongTonTai.Code {
		t.Fatalf("RÒ RỈ: xã đã sáp nhập trả %d còn tên miền không tồn tại trả %d — chênh lệch "+
			"này đủ để dò ra xã nào từng tồn tại", daSapNhap.Code, khongTonTai.Code)
	}
	if daSapNhap.Body.String() != khongTonTai.Body.String() {
		t.Fatalf("RÒ RỈ: thân phản hồi khác nhau\n  đã sáp nhập: %s\n  không tồn tại: %s",
			daSapNhap.Body.String(), khongTonTai.Body.String())
	}
	if strings.Contains(daSapNhap.Body.String(), "Sáp Nhập") {
		t.Fatalf("RÒ RỈ: tên xã đã sáp nhập lọt vào phản hồi: %s", daSapNhap.Body.String())
	}
}

// Hợp đồng REST khai httpx.Error cho MỌI lỗi. Một 404 trả text/plain buộc mọi máy khách phải
// có thêm một nhánh cho riêng nó — và nhánh đó là nơi thông báo lỗi biến thành chuỗi rỗng.
func TestTuChoiDungHinhDangLoiCuaHopDong(t *testing.T) {
	h := bien(func(http.ResponseWriter, *http.Request) {})
	w := goi(h, "khong-ai-biet.example.gov.vn")

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, muốn application/json", ct)
	}
	var e Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân không phải httpx.Error: %v — %s", err, w.Body.String())
	}
	if e.Code != "tenant_not_found" {
		t.Errorf("code = %q, muốn tenant_not_found", e.Code)
	}
	if e.Message == "" {
		t.Error("message rỗng — người dùng không có gì để đọc")
	}
}

// Biên phải đặt CẢ tenant.Tenant, không chỉ id.
//
// Nếu chỉ có id thì handler nào cần tên xã phải phân giải `Host` lần thứ hai, và khi đó nó
// cũng phải chép lại phép chuẩn hoá Host ở đây. Hai lần phân giải là hai câu trả lời có thể
// lệch nhau, và cái lệch ấy hiện ra dưới dạng tên xã khác trên màn hình.
func TestBienDatCaThongTinXaVaoContext(t *testing.T) {
	var thay tenant.Tenant
	var thayID tenant.ID
	h := bien(func(_ http.ResponseWriter, r *http.Request) {
		thay = tenant.MustCurrent(r.Context())
		thayID = tenant.MustFrom(r.Context())
	})

	goi(h, hostB)

	if thay.ID != xaB || thay.Name != "Xã Bình Dương" || thay.Host != hostB {
		t.Fatalf("Tenant trong context = %+v, muốn xã B đầy đủ", thay)
	}
	// `From` phải vẫn chạy y nguyên: đường gRPC và đường sự kiện chỉ đặt id, và mọi kho dữ
	// liệu đọc qua `From`. Nếu khoá thứ hai làm hỏng khoá thứ nhất thì mọi truy vấn mất xã.
	if thayID != xaB {
		t.Fatalf("From = %q, muốn %q", thayID, xaB)
	}
}

// Một Host viết hoa kèm cổng vẫn phải tới đúng xã. Trình duyệt gửi `Host` y như người ta gõ,
// và một cổng phi chuẩn (staging, cụm nội bộ) là chuyện thường. Thiếu phép chuẩn hoá này thì
// mọi người gõ tên miền viết hoa nhận 404 — màn hình đăng nhập hỏng mà không có gì đỏ.
func TestChuanHoaHostHoaVaCong(t *testing.T) {
	var thay tenant.ID
	h := bien(func(_ http.ResponseWriter, r *http.Request) {
		thay = tenant.MustFrom(r.Context())
	})

	goi(h, strings.ToUpper(hostA)+":8443")

	if thay != xaA {
		t.Fatalf("xã = %q, muốn %q — Host viết hoa kèm cổng phải chuẩn hoá về cùng một xã",
			thay, xaA)
	}
}

// Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Reverse proxy lẽ ra đã gỡ những
// header này; đây là lớp thứ hai, vì "lẽ ra đã" chính là cách sự cách ly bị mất.
func TestGoHeaderXaDoClientGui(t *testing.T) {
	var conLai []string
	h := StripTenantHeaders(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		for k := range r.Header {
			if strings.HasPrefix(strings.ToLower(k), "x-tenant") {
				conLai = append(conLai, k)
			}
		}
	}))

	r := httptest.NewRequest("GET", "https://"+hostA+"/bat-ky", nil)
	r.Header.Set("X-Tenant-Id", xaB)
	r.Header.Set("x-tenant-host", hostB)
	r.Header.Set("X-TENANT-OVERRIDE", "bat-ky")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if len(conLai) > 0 {
		t.Fatalf("header do client gửi còn sót: %v — client tự khai xã là client tự cấp quyền",
			conLai)
	}
}

func TestXaPhanGiaiDuocThiHandlerChay(t *testing.T) {
	var chay bool
	h := bien(func(w http.ResponseWriter, _ *http.Request) {
		chay = true
		w.WriteHeader(http.StatusNoContent)
	})

	w := goi(h, hostA)
	if !chay || w.Code != http.StatusNoContent {
		t.Fatalf("handler chạy = %v, mã = %d — muốn chạy và 204", chay, w.Code)
	}
}
