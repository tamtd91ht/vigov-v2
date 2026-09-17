package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// VÌ SAO TỆP NÀY TỒN TẠI
//
// Rìa công dân là chỗ MỌI yêu cầu của MỌI công dân đi qua, và nó là rìa duy nhất trong hệ
// thống không có `Host` để suy ra xã. Hỏng ở đây không gây lỗi: một yêu cầu rơi qua mà không
// có xã vẫn được phục vụ bình thường, và thứ duy nhất khác đi là dữ liệu của xã này hiện trên
// màn hình của công dân xã kia. Không có gì đỏ để ai nhìn thấy.
//
// Bạn đọc tệp này cùng với kb/10-decisions/0022-ria-kenh-cong-dan.md.

const (
	tokenA    = "token-cua-cong-dan-xa-a"
	tokenChua = "token-chua-chon-xa"
	apiHost   = "api.vigov.vn" // MỘT host cho mọi xã — nó không ứng với xã nào (ADR 0005)
)

type soPhienGia map[string]CitizenSession

func (m soPhienGia) TraCuu(_ context.Context, tok string) (CitizenSession, bool) {
	p, ok := m[tok]
	return p, ok
}

func soPhienMau() soPhienGia {
	return soPhienGia{
		tokenA: {ID: "phien-1", CitizenID: "cd-1", TenantID: xaA},
		// Công dân đã có phiên nhưng CHƯA chọn xã — lớp "khám phá" của ADR 0005. Đây là câu
		// trả lời thật, không phải dữ liệu hỏng.
		tokenChua: {ID: "phien-2", CitizenID: "cd-2"},
	}
}

// goiCongDan chạy một yêu cầu qua đúng chuỗi rìa công dân thật.
func goiCongDan(h http.Handler, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://"+apiHost+"/api/v1/bat-ky", nil)
	r.Host = apiHost
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func riaCongDan(trong http.Handler) http.Handler {
	return CitizenEdge(soPhienMau())(trong)
}

func TestKhongCoPhienThiTuyenNghiepVuBiTuChoi(t *testing.T) {
	var chay bool
	h := riaCongDan(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		chay = true
	})))

	w := goiCongDan(h, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mã = %d, muốn 401", w.Code)
	}
	if chay {
		t.Fatal("handler ĐÃ CHẠY khi không có phiên — mọi truy vấn sau đó không có xã để giới hạn")
	}
}

// Ba trạng thái phải cho CÙNG một câu trả lời: không có phiên, phiên không tra được, phiên
// chưa gắn xã. Tách chúng ra là nói cho người đang dò biết họ đã tới bước nào.
func TestBaTrangThaiThieuXaTraLoiGiongHetNhau(t *testing.T) {
	h := riaCongDan(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))

	khong := goiCongDan(h, "")
	sai := goiCongDan(h, "token-khong-co-that")
	chuaChon := goiCongDan(h, tokenChua)

	for ten, w := range map[string]*httptest.ResponseRecorder{"token sai": sai, "chưa chọn xã": chuaChon} {
		if w.Code != khong.Code || w.Body.String() != khong.Body.String() {
			t.Fatalf("RÒ RỈ: %q trả %d %s, còn không có phiên trả %d %s — chênh lệch này đủ để "+
				"dò ra trạng thái phiên", ten, w.Code, w.Body.String(), khong.Code, khong.Body.String())
		}
	}
}

func TestPhienHopLeThiXaVaoContext(t *testing.T) {
	var thay tenant.ID
	h := riaCongDan(XaTuPhien()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		thay = tenant.MustFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})))

	w := goiCongDan(h, tokenA)

	if w.Code != http.StatusNoContent {
		t.Fatalf("mã = %d, muốn 204", w.Code)
	}
	if thay != xaA {
		t.Fatalf("xã trong context = %q, muốn %q", thay, xaA)
	}
}

// BÀI QUAN TRỌNG NHẤT TỆP NÀY — luật 1 cấm #2.
//
// Client tự khai xã là client tự cấp quyền. Ở rìa này nó nặng hơn đường cán bộ: không có `Host`
// để đối chiếu, nên một giá trị do client gửi lọt qua thì KHÔNG có phép kiểm nào sau đó mâu
// thuẫn với nó. Tham số `t=` của deep link dẫn giao diện (ADR 0005), nó không chạm tới rìa.
func TestXaKhongBaoGioDenTuClient(t *testing.T) {
	var thay tenant.ID
	var conLai []string
	h := riaCongDan(XaTuPhien()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		thay = tenant.MustFrom(r.Context())
		for k := range r.Header {
			if strings.HasPrefix(strings.ToLower(k), "x-tenant") {
				conLai = append(conLai, k)
			}
		}
	})))

	r := httptest.NewRequest("GET", "https://"+apiHost+"/api/v1/bat-ky?t="+xaB, nil)
	r.Host = apiHost
	r.Header.Set("Authorization", "Bearer "+tokenA)
	r.Header.Set("X-Tenant-Id", xaB)
	r.Header.Set("x-tenant-host", "binhduong.vigov.vn")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if len(conLai) > 0 {
		t.Errorf("header xã do client gửi còn sót: %v — rìa công dân phải tự gỡ, không chờ "+
			"StripTenantHeaders được mắc thêm", conLai)
	}
	if thay != xaA {
		t.Fatalf("xã = %q, muốn %q — xã phải đến từ phiên, không từ header hay query của client",
			thay, xaA)
	}
}

// Phiên đựng ở Authorization, KHÔNG phải cookie. Một host phục vụ mọi xã nghĩa là cookie ở đó
// được gửi kèm lưu lượng của mọi xã theo đúng cấu tạo — hình dạng luật 1 cấm #3.
func TestPhienTrongCookieKhongDuocCoi(t *testing.T) {
	h := riaCongDan(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))

	r := httptest.NewRequest("GET", "https://"+apiHost+"/api/v1/bat-ky", nil)
	r.Host = apiHost
	r.AddCookie(&http.Cookie{Name: "phien", Value: tokenA})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mã = %d, muốn 401 — cookie không phải nơi đựng phiên công dân", w.Code)
	}
}

// Thiếu khai lớp là DENY, không phải allow (luật 5 bất biến 2).
//
// apidoc từ chối tuyến như thế lúc DỰNG; đây là hàng rào lúc chạy, cho tuyến đăng ký ngoài cây
// mà bộ sinh quét.
func TestTuyenKhongKhaiLopXaThiKhongTraLoiDuoc(t *testing.T) {
	h := riaCongDan(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"bi_mat":"danh sach ho so"}`))
	}))

	w := goiCongDan(h, tokenA)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("mã = %d, muốn 500 — tuyến chưa khai lớp xã không được trả lời", w.Code)
	}
	if strings.Contains(w.Body.String(), "bi_mat") {
		t.Fatalf("thân của handler đã lọt ra ngoài: %s", w.Body.String())
	}
}

// Handler không ghi gì thì net/http trả 200 rỗng — tức một tuyến chưa khai lớp vẫn "thành
// công". Nhánh này không tự lộ ra trong bài trên.
func TestTuyenKhongKhaiLopXaVaKhongGhiGiVanBiTuChoi(t *testing.T) {
	h := riaCongDan(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	w := goiCongDan(h, tokenA)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("mã = %d, muốn 500", w.Code)
	}
}

// Phản hồi TỪ CHỐI của một guard nằm ngoài lớp xã phải đi qua nguyên vẹn.
//
// authz.CitizenOnly nằm NGOÀI lớp xã (ADR 0022), nên một yêu cầu không có phiên bị từ chối
// trước khi middleware lớp kịp chạy. Nếu hàng rào "chưa khai lớp" chặn cả phản hồi ấy thì mọi
// 401 của kênh công dân thành 500, và người sửa sau đi tìm lỗi máy chủ không tồn tại.
func TestTuChoiCuaGuardNgoaiKhongBiDoiThanh500(t *testing.T) {
	guardNgoai := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "Phiên không hợp lệ.", "")
		})
	}
	h := riaCongDan(guardNgoai(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))))

	w := goiCongDan(h, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mã = %d, muốn 401 — từ chối của guard ngoài bị đổi thành lỗi máy chủ", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unauthorized") {
		t.Fatalf("thân phản hồi = %s, muốn giữ nguyên lỗi của guard ngoài", w.Body.String())
	}
}

// Bức tường thứ nhất của ADR 0022: handler KhongThuocXa KHÔNG có xã trong context, kể cả khi
// phiên có xã. Không có xã nghĩa là store.For(ctx) panic — nó không thể chạm dữ liệu nghiệp vụ.
func TestKhongThuocXaThiHandlerKhongCoXaDuTrongPhienCo(t *testing.T) {
	var coXa bool
	var mustFromPanic bool
	h := riaCongDan(KhongThuocXa("danh mục xã để công dân chọn: lời gọi TẠO RA lựa chọn đó")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, coXa = tenant.From(r.Context())
			func() {
				defer func() { mustFromPanic = recover() != nil }()
				_ = tenant.MustFrom(r.Context()) // chính là cái store.For(ctx) gọi
			}()
			w.WriteHeader(http.StatusOK)
		})))

	w := goiCongDan(h, tokenA) // phiên NÀY có xã

	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d, muốn 200", w.Code)
	}
	if coXa {
		t.Fatal("handler KhongThuocXa THẤY xã trong context — nó đọc được dữ liệu nghiệp vụ " +
			"của xã đó mà không tuyến nào khai điều ấy")
	}
	if !mustFromPanic {
		t.Fatal("tenant.MustFrom không panic — bức tường thứ nhất của ADR 0022 không còn")
	}
}

// Lý do là BẮT BUỘC và phải cụ thể, đúng kỷ luật authz.Public(reason) — luật 5 cấm #4. Panic
// lúc dựng máy chủ, chứ không phải một miễn trừ không ai giải thích được sáu tháng sau.
func TestKhongThuocXaKhongCoLyDoThiDungNgayLucDung(t *testing.T) {
	for _, lyDo := range []string{"", "   "} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("KhongThuocXa(%q) không dừng — một miễn trừ không lý do đã lọt vào chuỗi", lyDo)
				}
			}()
			KhongThuocXa(lyDo)
		}()
	}
}

// tenant.MustCurrent PANIC trong handler công dân là HÀNH VI ĐÃ GHI, không phải lỗi.
//
// Rìa này không phân giải `Host` nên nó không biết TÊN xã, chỉ biết tenant_id từ phiên (ADR
// 0022). Ép một Tenant vào context thì phải bịa một bản ghi rỗng tên, và một tên rỗng đi tới
// màn hình của một cơ quan nhà nước thì tệ hơn hẳn một lỗi. Tên xã là dữ liệu Mini App lấy
// bằng một lời gọi riêng.
//
// Bài này tồn tại để người sửa sau không "vá" bằng cách đổi Into thành IntoFull.
func TestMustCurrentPanicTrongHandlerCongDanLaYDo(t *testing.T) {
	var coTenXa bool
	var daPanic bool
	h := riaCongDan(XaTuPhien()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, coTenXa = tenant.Current(r.Context())
		func() {
			defer func() { daPanic = recover() != nil }()
			_ = tenant.MustCurrent(r.Context())
		}()
		w.WriteHeader(http.StatusOK)
	})))

	goiCongDan(h, tokenA)

	if coTenXa {
		t.Fatal("rìa công dân đặt CẢ Tenant vào context — nó không phân giải Host nên tên xã " +
			"ở đó chỉ có thể là bịa (ADR 0022)")
	}
	if !daPanic {
		t.Fatal("tenant.MustCurrent không panic — hành vi này ADR 0022 ghi là ý đồ")
	}
}
