package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Điều duy nhất endpoint này hứa: LUÔN trả 200. Nó không đọc, không lưu, không xác minh gì.
//
// Vì sao phải có test cho một hàm hai dòng: lời hứa "luôn 200" là thứ Zalo đo. Một lần sửa
// tưởng vô hại — thêm `switch r.Method`, thêm một phép kiểm chữ ký nửa vời, thêm một lần đọc
// body rồi trả 400 khi JSON hỏng — đều biến webhook thành endpoint Zalo sẽ TẮT sau vài lần
// thử lại, và không có gì báo cho ai biết ngoài việc webhook lặng đi.
func TestWebhookZaloLuonTraVe200(t *testing.T) {
	mux := http.NewServeMux()
	MountWebhookZalo(mux)

	// Zalo có thể thăm dò bằng GET lúc lưu URL rồi mới gửi sự kiện bằng POST. Cả hai phải 200.
	cac_ca := []struct {
		ten    string
		phuong string
		than   string
		kieu   string
	}{
		{"GET không thân — Console thăm dò lúc lưu URL", http.MethodGet, "", ""},
		{"POST JSON hợp lệ", http.MethodPost, `{"event_name":"test"}`, "application/json"},
		{"POST JSON HỎNG — vẫn phải 200, không được 400", http.MethodPost, `{ khong phai json`, "application/json"},
		{"POST thân rỗng", http.MethodPost, "", "application/json"},
		{"POST kiểu nội dung lạ", http.MethodPost, "abc", "text/plain"},
		{"HEAD", http.MethodHead, "", ""},
		{"PUT — phương thức không ai dùng, vẫn không được 405", http.MethodPut, "{}", "application/json"},
	}

	for _, ca := range cac_ca {
		t.Run(ca.ten, func(t *testing.T) {
			r := httptest.NewRequest(ca.phuong, DuongDanWebhookZalo, strings.NewReader(ca.than))
			if ca.kieu != "" {
				r.Header.Set("Content-Type", ca.kieu)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("mong 200, nhận %d", w.Code)
			}
		})
	}
}

// Thân yêu cầu phải còn NGUYÊN sau khi xử lý: endpoint không được đọc nó.
//
// Đây không phải phép kiểm hình thức. Đọc thân là bước đầu tiên của việc ghi nó ra log, và một
// dòng gỡ lỗi in trọn payload sẽ đẩy định danh người dùng vào log tập trung, bản sao lưu và
// dịch vụ giám sát của bên thứ ba — nơi không thu hồi lại được (luật 3).
func TestWebhookZaloKhongDocThan(t *testing.T) {
	mux := http.NewServeMux()
	MountWebhookZalo(mux)

	than := `{"event_name":"user_send_text","sender":{"id":"0900000000"}}`
	doc := &demDoc{Reader: strings.NewReader(than)}

	r := httptest.NewRequest(http.MethodPost, DuongDanWebhookZalo, doc)
	mux.ServeHTTP(httptest.NewRecorder(), r)

	if doc.so_byte != 0 {
		t.Fatalf("endpoint đã đọc %d byte của thân yêu cầu; nó phải đọc 0", doc.so_byte)
	}
}

// Đường dẫn KHÔNG được nằm dưới /api/v1: tiền tố ấy là bề mặt có xã, còn endpoint này không có
// xã nào. Một đường dẫn trông như có xã là đường dẫn người sau sẽ gắn lại vào sau chuỗi xã, và
// khi ấy Zalo nhận 404.
func TestDuongDanWebhookNamNgoaiBeMatCoXa(t *testing.T) {
	if strings.HasPrefix(DuongDanWebhookZalo, "/api/") {
		t.Fatalf("đường dẫn %q nằm dưới /api/ — xem chú thích MountWebhookZalo", DuongDanWebhookZalo)
	}
	if !strings.HasPrefix(DuongDanWebhookZalo, "/") {
		t.Fatalf("đường dẫn %q phải bắt đầu bằng /", DuongDanWebhookZalo)
	}
}

type demDoc struct {
	Reader  *strings.Reader
	so_byte int
}

func (d *demDoc) Read(p []byte) (int, error) {
	n, err := d.Reader.Read(p)
	d.so_byte += n
	return n, err
}
