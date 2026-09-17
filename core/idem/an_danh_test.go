package idem

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
)

// THE DEFECT CLASS THIS FILE CLOSES: a route declared `authz.Public(...)` together with
// `idem.Required(...)`.
//
// The pair looks right on a citizen intake route — rule 10 demands a lookup code returned
// immediately, rule 7 forbids deleting the duplicate, so intake is precisely where duplicate
// protection matters most. But with no principal the actor component of the key is one shared
// value, so every anonymous sender of one commune shares one key space:
//
//	citizen A: Idempotency-Key aaaaaaaa -> petition created, code PA-2026-0001
//	citizen B: Idempotency-Key aaaaaaaa -> replayed: B is handed A's lookup code
//
// B can then look up A's petition (rule 4, invariant 1) AND B's own petition was never created
// while B believes it was (rule 10, invariant 1). Both failures are silent, and the minimum key
// length does not help: a key derived from the form contents or a device id collides on purpose,
// not by chance.

// goiLog sends one request through the middleware with a chosen principal and a readable log,
// so both the answer and the line written about it can be asserted on.
func goiLog(t *testing.T, h http.Handler, s Store, tid, khoa string,
	p *authz.Principal) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	r := httptest.NewRequest(http.MethodPost, duongDan, strings.NewReader("{}"))
	if khoa != "" {
		r.Header.Set(Header, khoa)
	}
	ctx := tenant.Into(r.Context(), tenant.ID(tid))
	if p != nil {
		ctx = authz.Into(ctx, *p)
	}
	ctx = Into(ctx, s, log)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r.WithContext(ctx))
	return w, buf.String()
}

func TestHaiNguoiAnDanhCungXaCungKhoaKhongAiNhanKetQuaCuaAi(t *testing.T) {
	// The headline case. Two citizens of the SAME commune present the SAME client-generated key.
	// Nobody may be served, and above all nobody may be served SOMEBODY ELSE'S lookup code.
	s := newStoreGia()
	var chay int
	h := Required(MoKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chay++
		RecordCode(r.Context(), "PA-CUA-NGUOI-A")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"lookup_code":"PA-CUA-NGUOI-A"}`))
	}))

	wA, _ := goiLog(t, h, s, xaA, khoaKhachHang, nil)
	wB, _ := goiLog(t, h, s, xaA, khoaKhachHang, nil)

	// The leak first, because it is the consequence: what person B is handed.
	if wB.Header().Get(HeaderPhatLai) != "" {
		t.Error("người B nhận BẢN PHÁT LẠI của người A — lộ hồ sơ giữa hai công dân (luật 4)")
	}
	if strings.Contains(wB.Body.String(), "PA-CUA-NGUOI-A") {
		t.Error("người B nhận MÃ TRA CỨU của người A — tra được nội dung phản ánh của người khác, " +
			"trong khi phản ánh của chính B chưa từng được tạo (luật 10 bất biến 1)")
	}
	if chay != 0 {
		t.Errorf("handler chạy %d lần cho yêu cầu không có chủ thể — mọi người gửi ẩn danh của xã "+
			"dùng CHUNG một không gian khoá", chay)
	}
	for ten, w := range map[string]*httptest.ResponseRecorder{"người A": wA, "người B": wB} {
		if w.Code != http.StatusInternalServerError {
			t.Errorf("%s: mã = %d, muốn 500 — route khai Required sau Public là sai cấu hình",
				ten, w.Code)
		}
	}
	if s.soKhoa() != 0 {
		t.Errorf("yêu cầu bị từ chối vẫn chiếm %d khoá", s.soKhoa())
	}
}

func TestAnDanhBiTuChoiThiNoiRoLaLoiCauHinhChuKhongPhaiLoiNguoiGui(t *testing.T) {
	// Whoever meets this 500 must reach the cause in one step: the route declares Required after
	// Public. Without that, the next person goes hunting for a bug inside idem.
	s := newStoreGia()
	h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))

	w, ra := goiLog(t, h, s, xaA, khoaKhachHang, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("mã = %d, muốn 500", w.Code)
	}
	var e struct{ Code, Message string }
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải hình dạng httpx.Error: %v (%s)", err, w.Body.String())
	}
	if e.Code != "idempotency_misconfigured" {
		t.Errorf("code = %q, muốn idempotency_misconfigured", e.Code)
	}
	if !strings.Contains(e.Message, "không phải do bạn") {
		t.Errorf("thông báo không nói rõ đây không phải lỗi người gửi: %q", e.Message)
	}
	if !strings.Contains(e.Message, "CHƯA được xử lý") {
		t.Errorf("thông báo không nói rõ yêu cầu chưa được xử lý — người gửi không biết có phải "+
			"gửi lại hay không: %q", e.Message)
	}

	// The log line is the operator's half of the same message.
	if !strings.Contains(ra, "level=ERROR") {
		t.Errorf("sai cấu hình route chỉ được ghi ở mức ERROR:\n%s", ra)
	}
	if !strings.Contains(ra, "xa="+xaA) {
		t.Errorf("dòng log thiếu xã — 200+ xã ghi chung một luồng log:\n%s", ra)
	}
	if !strings.Contains(ra, duongDan) {
		t.Errorf("dòng log không chỉ ra route nào sai cấu hình:\n%s", ra)
	}
}

func TestSaiCauHinhRouteDuocBaoTruocLoiCuaNguoiGui(t *testing.T) {
	// A misconfigured route with a missing or malformed header must report the CONFIGURATION, not
	// the header. Answering 400 would send the integrator to fix a header that was never the
	// problem, while the defect — a route that cannot protect anybody — stays hidden.
	h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))

	for ten, khoa := range map[string]string{"thiếu header": "", "khoá quá ngắn": "abc"} {
		t.Run(ten, func(t *testing.T) {
			w, _ := goiLog(t, h, newStoreGia(), xaA, khoa, nil)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("mã = %d, muốn 500 — lỗi cấu hình route xếp trên lỗi của người gửi", w.Code)
			}
		})
	}

	// With a principal, the same requests are ordinary client errors again.
	for _, khoa := range []string{"", "abc"} {
		w, _ := goiLog(t, h, newStoreGia(), xaA, khoa, canBo(xaA, canBo1))
		if w.Code != http.StatusBadRequest {
			t.Errorf("khoá %q từ một chủ thể hợp lệ: mã = %d, muốn 400", khoa, w.Code)
		}
	}
}

func TestCoChuTheThiVanPhucVuBinhThuong(t *testing.T) {
	// The guard must refuse the anonymous case and nothing else: a signed-in clerk still gets
	// duplicate protection exactly as before.
	s := newStoreGia()
	h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))

	w, _ := goiLog(t, h, s, xaA, khoaKhachHang, canBo(xaA, canBo1))
	if w.Code != http.StatusCreated {
		t.Fatalf("mã = %d, muốn 201", w.Code)
	}
	w2, _ := goiLog(t, h, s, xaA, khoaKhachHang, canBo(xaA, canBo1))
	if w2.Header().Get(HeaderPhatLai) != "true" {
		t.Error("cùng một người gửi lại cùng khoá phải được phát lại kết quả của CHÍNH MÌNH")
	}
}

func TestKhongCanVanPhucVuYeuCauAnDanh(t *testing.T) {
	// The sign-in route is Public and declares KhongCan. It must keep working: refusing it would
	// lock every member of staff out of the system, which is the more serious failure.
	s := newStoreGia()
	var chay int
	h := KhongCan("đăng nhập lần hai mở một phiên thứ hai, thu hồi được")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			chay++
			w.WriteHeader(http.StatusCreated)
		}))

	w, _ := goiLog(t, h, s, xaA, "", nil)
	if w.Code != http.StatusCreated || chay != 1 {
		t.Fatalf("KhongCan + ẩn danh: mã = %d, handler chạy %d lần, muốn 201 và 1", w.Code, chay)
	}
}

func TestTuChoiAnDanhTruocKhiChamVaoStore(t *testing.T) {
	// A misconfigured route must not consume anything, and must answer the same way whether or
	// not Redis is reachable: it is not a Redis problem, and a "Store unavailable" answer would
	// send the next person to the wrong place entirely.
	for ten, s := range map[string]Store{
		"Store bình thường": newStoreGia(),
		"Store hỏng":        &storeGia{data: map[string]string{}, loi: errors.New("connection refused")},
		"chưa cấu hình":     nil,
	} {
		t.Run(ten, func(t *testing.T) {
			for _, cheDo := range []CheDoHong{MoKhiHong, DongKhiHong} {
				h := Required(cheDo)(handlerTao("PA-7F3K9Q"))
				w, _ := goiLog(t, h, s, xaA, khoaKhachHang, nil)
				if w.Code != http.StatusInternalServerError {
					t.Errorf("chế độ %v: mã = %d, muốn 500", cheDo, w.Code)
				}
			}
			if g, ok := s.(*storeGia); ok && g.soKhoa() != 0 {
				t.Errorf("đã chạm vào Store: %d khoá", g.soKhoa())
			}
		})
	}
}

func TestDongLogCuaIdemLuonMangXa(t *testing.T) {
	// One process serves 200+ communes into one log stream. The heaviest line is the MoKhiHong
	// one: it says duplicate protection was SKIPPED, and without the commune an operator cannot
	// act on it. A commune id is an opaque ULID, not personal data (rule 3).
	hong := &storeGia{data: map[string]string{}, loi: errors.New("dial tcp: connection refused")}

	_, raMo := goiLog(t, Required(MoKhiHong)(handlerTao("PA-7F3K9Q")), hong, xaB,
		khoaKhachHang, canBo(xaB, canBo1))
	if !strings.Contains(raMo, "xa="+xaB) {
		t.Errorf("dòng 'bỏ qua chống trùng' thiếu xã — không biết xã nào đã đi qua không được bảo vệ:\n%s", raMo)
	}

	_, raDong := goiLog(t, Required(DongKhiHong)(handlerTao("PA-7F3K9Q")), hong, xaB,
		khoaKhachHang, canBo(xaB, canBo1))
	if !strings.Contains(raDong, "xa="+xaB) {
		t.Errorf("dòng 'từ chối vì Store không dùng được' thiếu xã:\n%s", raDong)
	}

	// And nothing in either line is the client-supplied key or the built Redis key: one is
	// client-controlled input in a log, the other is a value that identifies a stored result.
	for _, ra := range []string{raMo, raDong} {
		if strings.Contains(ra, khoaKhachHang) {
			t.Errorf("khoá do client gửi lọt vào log:\n%s", ra)
		}
	}
}
