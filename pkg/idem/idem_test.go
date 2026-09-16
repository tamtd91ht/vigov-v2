package idem

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/authz"
	"github.com/vihat/vigov/pkg/tenant"
)

// Two real-shaped commune ids. ULIDs are opaque: they are NOT administrative codes.
const (
	xaA = "01J0000000000000000000000A"
	xaB = "01J0000000000000000000000B"
)

// khoaKhachHang is a client-generated key of the shape a Mini App would send.
const khoaKhachHang = "01J8ZQG7QY5V3N4C6K2D1F0R7T"

// Two clerks of the SAME commune.
const (
	canBo1 = "01J8ZQG7QY5V3N4C6K2D1CANB1"
	canBo2 = "01J8ZQG7QY5V3N4C6K2D1CANB2"
)

const duongDan = "/api/v1/citizen-reports"

// keyGia builds the key the middleware would build for the route used throughout this file.
func keyGia(tid, chuThe string) string {
	return Key(tenant.ID(tid), chuThe, http.MethodPost, duongDan, khoaKhachHang)
}

// storeGia is an in-memory Store. The whole point of the Store interface: every test in this
// file runs with no Redis, the way tenant.Directory lets the edge be tested without the
// platform service.
type storeGia struct {
	mu   sync.Mutex
	data map[string]string
	loi  error // when set, every call fails — stands in for an unreachable Redis
	nhat []string
}

func newStoreGia() *storeGia { return &storeGia{data: map[string]string{}} }

func (s *storeGia) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loi != nil {
		return false, s.loi
	}
	s.nhat = append(s.nhat, "claim "+key)
	if _, co := s.data[key]; co {
		return false, nil
	}
	s.data[key] = dauDangChay
	return true, nil
}

func (s *storeGia) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loi != nil {
		return "", s.loi
	}
	return s.data[key], nil
}

func (s *storeGia) Complete(_ context.Context, key, value string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loi != nil {
		return s.loi
	}
	s.data[key] = value
	return nil
}

func (s *storeGia) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loi != nil {
		return s.loi
	}
	delete(s.data, key)
	return nil
}

func (s *storeGia) doc(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key]
}

func (s *storeGia) soKhoa() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}

func logIm() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// chuTheMacDinh is the actor component of the key for every request `goi` sends.
const chuTheMacDinh = "staff:" + canBo1

// goi sends one request from the DEFAULT CLERK of the commune.
//
// It is not anonymous, and cannot be: Required refuses a request with no principal, because one
// shared actor is one shared key space for every anonymous sender of a commune. The anonymous
// case has its own file — an_danh_test.go — where the refusal is the property under test.
func goi(t *testing.T, h http.Handler, s Store, tid, khoa string) *httptest.ResponseRecorder {
	t.Helper()
	return goiNhu(t, h, s, tid, khoa, canBo(tid, canBo1))
}

// goiNhu sends one request through the middleware chain, with the commune already resolved and
// the principal already in the context — exactly the state httpx.TenantMiddleware and authz
// leave behind at the edge, since authz wraps OUTSIDE idem.
func goiNhu(t *testing.T, h http.Handler, s Store, tid, khoa string,
	p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, duongDan, strings.NewReader("{}"))
	if khoa != "" {
		r.Header.Set(Header, khoa)
	}
	ctx := tenant.Into(r.Context(), tenant.ID(tid))
	if p != nil {
		ctx = authz.Into(ctx, *p)
	}
	ctx = Into(ctx, s, logIm())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r.WithContext(ctx))
	return w
}

func canBo(tid, id string) *authz.Principal {
	return &authz.Principal{ID: id, Kind: "staff", TenantID: tenant.ID(tid)}
}

// handlerTao stands in for an intake handler: it reports its lookup code and answers 201.
func handlerTao(ma string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RecordCode(r.Context(), ma)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"lookup_code":"` + ma + `"}`))
	})
}

func TestLanDauDiQuaRoiPhatLaiDungMaVaStatus(t *testing.T) {
	// The replay must carry the lookup code. A bare flag would answer 409 and the citizen
	// would never learn their code while the petition sits in the system (rule 10, invariant 1).
	var chay int
	h := Required(MoKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chay++
		RecordCode(r.Context(), "PA-7F3K9Q")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"lookup_code":"PA-7F3K9Q"}`))
	}))
	s := newStoreGia()

	w1 := goi(t, h, s, xaA, khoaKhachHang)
	if w1.Code != http.StatusCreated {
		t.Fatalf("lần đầu: status = %d, muốn 201", w1.Code)
	}
	if chay != 1 {
		t.Fatalf("handler chạy %d lần ở lần gửi đầu", chay)
	}

	w2 := goi(t, h, s, xaA, khoaKhachHang)
	if chay != 1 {
		t.Fatalf("handler chạy lại ở lần gửi thứ hai (%d lần) — phiếu bị tạo trùng", chay)
	}
	if w2.Code != http.StatusCreated {
		t.Errorf("phát lại: status = %d, muốn 201", w2.Code)
	}
	if w2.Header().Get(HeaderPhatLai) != "true" {
		t.Errorf("thiếu header %s", HeaderPhatLai)
	}
	var pl PhatLai
	if err := json.Unmarshal(w2.Body.Bytes(), &pl); err != nil {
		t.Fatalf("thân phát lại không phải JSON: %v", err)
	}
	if pl.Code != "PA-7F3K9Q" || !pl.Replayed {
		t.Errorf("phát lại = %+v, muốn mã PA-7F3K9Q", pl)
	}
}

func TestKhongLuuThanPhanHoi(t *testing.T) {
	// Rule 3: the body holds names, phone numbers and the text of the petition. Putting it in
	// Redis builds a personal-data store outside PostgreSQL — no audit trail, no soft delete.
	h := Required(MoKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RecordCode(r.Context(), "PA-7F3K9Q")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ho_ten":"Nguyễn Văn A","so_dien_thoai":"0900000000"}`))
	}))
	s := newStoreGia()
	goi(t, h, s, xaA, khoaKhachHang)

	giaTri := s.doc(keyGia(xaA, chuTheMacDinh))
	if giaTri != "2:201:PA-7F3K9Q" {
		t.Fatalf("giá trị lưu = %q, muốn đúng \"2:<status>:<mã>\"", giaTri)
	}
	for _, cam := range []string{"Nguyễn", "0900000000", "ho_ten"} {
		if strings.Contains(giaTri, cam) {
			t.Errorf("dữ liệu cá nhân lọt vào Redis: %q", giaTri)
		}
	}
}

func TestHaiXaCungMotKhoaKhachHangKhongDungNhau(t *testing.T) {
	// THE case this package exists for. Without the t:<tenant_id> prefix, commune B is served
	// commune A's result — a breach between two public authorities, through a cache key
	// (rule 1, invariant 7).
	s := newStoreGia()
	hA := Required(DongKhiHong)(handlerTao("PA-AAAA11"))
	hB := Required(DongKhiHong)(handlerTao("PA-BBBB22"))

	wA := goi(t, hA, s, xaA, khoaKhachHang)
	if wA.Code != http.StatusCreated {
		t.Fatalf("xã A: status = %d", wA.Code)
	}
	wB := goi(t, hB, s, xaB, khoaKhachHang)
	if wB.Code != http.StatusCreated {
		t.Fatalf("xã B bị chặn bởi khoá của xã A: status = %d — RÒ RỈ GIỮA HAI XÃ", wB.Code)
	}
	if wB.Header().Get(HeaderPhatLai) != "" {
		t.Fatal("xã B nhận bản phát lại của xã A — RÒ RỈ GIỮA HAI XÃ")
	}
	if s.soKhoa() != 2 {
		t.Fatalf("có %d khoá, muốn 2 — mỗi xã một khoá riêng", s.soKhoa())
	}
	if s.doc(keyGia(xaA, chuTheMacDinh)) ==
		s.doc(keyGia(xaB, chuTheMacDinh)) {
		t.Fatal("hai xã chia sẻ một giá trị")
	}

	// And the replay stays inside its own commune.
	wA2 := goi(t, hA, s, xaA, khoaKhachHang)
	var pl PhatLai
	_ = json.Unmarshal(wA2.Body.Bytes(), &pl)
	if pl.Code != "PA-AAAA11" {
		t.Errorf("xã A phát lại mã %q, muốn PA-AAAA11", pl.Code)
	}
}

func TestHaiCanBoCungXaCungMotKhoaKhachHangKhongDungNhau(t *testing.T) {
	// The pair of the test above, one level in. The commune prefix stops a leak BETWEEN two
	// authorities; the actor stops the same leak INSIDE one. The key is supplied by the client,
	// so two clerks of the same commune can present the same one — and the second would be
	// replayed the first one's lookup code: a business code for something they did not do,
	// while their own petition was never created.
	s := newStoreGia()
	h1 := Required(DongKhiHong)(handlerTao("PA-CANBO1"))
	h2 := Required(DongKhiHong)(handlerTao("PA-CANBO2"))

	w1 := goiNhu(t, h1, s, xaA, khoaKhachHang, canBo(xaA, canBo1))
	if w1.Code != http.StatusCreated {
		t.Fatalf("cán bộ 1: status = %d", w1.Code)
	}
	w2 := goiNhu(t, h2, s, xaA, khoaKhachHang, canBo(xaA, canBo2))
	if w2.Code != http.StatusCreated {
		t.Fatalf("cán bộ 2 bị chặn bởi khoá của cán bộ 1: status = %d", w2.Code)
	}
	if w2.Header().Get(HeaderPhatLai) != "" {
		t.Fatal("cán bộ 2 nhận bản phát lại của cán bộ 1 — đọc kết quả của người khác")
	}
	if !strings.Contains(w2.Body.String(), "PA-CANBO2") {
		t.Errorf("cán bộ 2 nhận thân %s, muốn phiếu của chính mình", w2.Body.String())
	}
	if s.soKhoa() != 2 {
		t.Fatalf("có %d khoá, muốn 2 — mỗi chủ thể một khoá riêng", s.soKhoa())
	}

	// Each one replays only their own.
	var pl PhatLai
	_ = json.Unmarshal(goiNhu(t, h1, s, xaA, khoaKhachHang, canBo(xaA, canBo1)).Body.Bytes(), &pl)
	if pl.Code != "PA-CANBO1" {
		t.Errorf("cán bộ 1 phát lại mã %q, muốn PA-CANBO1", pl.Code)
	}
	pl = PhatLai{}
	_ = json.Unmarshal(goiNhu(t, h2, s, xaA, khoaKhachHang, canBo(xaA, canBo2)).Body.Bytes(), &pl)
	if pl.Code != "PA-CANBO2" {
		t.Errorf("cán bộ 2 phát lại mã %q, muốn PA-CANBO2", pl.Code)
	}
}

func TestChuThe(t *testing.T) {
	if got := ChuThe(context.Background()); got != ChuTheAnDanh {
		t.Errorf("không có Principal: ChuThe = %q, muốn %q", got, ChuTheAnDanh)
	}
	ctx := authz.Into(context.Background(), authz.Principal{ID: canBo1, Kind: "staff"})
	if got := ChuThe(ctx); got != "staff:"+canBo1 {
		t.Errorf("ChuThe = %q", got)
	}
	// Kind must be part of it: staff and citizen ids come from two different tables.
	ctx = authz.Into(context.Background(), authz.Principal{ID: canBo1, Kind: "citizen"})
	if got := ChuThe(ctx); got != "citizen:"+canBo1 {
		t.Errorf("ChuThe = %q", got)
	}
}

func TestKeyMangTienToXa(t *testing.T) {
	k := keyGia(xaA, chuTheMacDinh)
	if !strings.HasPrefix(k, "t:"+xaA+":idem:") {
		t.Fatalf("key = %q, thiếu tiền tố t:<tenant_id>:idem:", k)
	}
	if strings.Contains(k, khoaKhachHang) {
		t.Error("khoá do client sinh nằm nguyên trong key — phải băm")
	}
	// A key reused on another route must not replay the wrong result.
	if k == Key(tenant.ID(xaA), chuTheMacDinh, http.MethodPost, "/api/v1/disbursements", khoaKhachHang) {
		t.Error("hai path khác nhau cho ra cùng một key")
	}
	if k == Key(tenant.ID(xaA), chuTheMacDinh, http.MethodPut, "/api/v1/citizen-reports", khoaKhachHang) {
		t.Error("hai method khác nhau cho ra cùng một key")
	}
	// Same commune, same route, same client key, two different people.
	if keyGia(xaA, "staff:"+canBo1) == keyGia(xaA, "staff:"+canBo2) {
		t.Error("hai cán bộ cùng xã cho ra cùng một key")
	}
	// Kind is part of the actor: staff ids and citizen ids come from two different tables.
	if keyGia(xaA, "staff:"+canBo1) == keyGia(xaA, "citizen:"+canBo1) {
		t.Error("cán bộ và công dân cùng id cho ra cùng một key")
	}
}

func TestDangChayThiTraVe409(t *testing.T) {
	s := newStoreGia()
	// A claim left by a request still in flight.
	key := keyGia(xaA, chuTheMacDinh)
	s.data[key] = dauDangChay

	var chay int
	h := Required(MoKhiHong)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { chay++ }))
	w := goi(t, h, s, xaA, khoaKhachHang)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, muốn 409", w.Code)
	}
	if w.Header().Get("Retry-After") != "1" {
		t.Error("thiếu Retry-After")
	}
	if chay != 0 {
		t.Error("handler chạy trong khi lần gửi trước còn đang xử lý")
	}
}

func TestHeaderKhongHopLe(t *testing.T) {
	cases := []struct {
		ten  string
		khoa string
		muon int
	}{
		{"thiếu header", "", http.StatusBadRequest},
		{"quá ngắn", "abc", http.StatusBadRequest},
		{"quá dài", strings.Repeat("a", KeyToiDa+1), http.StatusBadRequest},
		{"ký tự lạ", "khoa nay co dau cach", http.StatusBadRequest},
		{"ký tự điều khiển", "abcdefgh\nijkl", http.StatusBadRequest},
		{"hợp lệ", khoaKhachHang, http.StatusCreated},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			s := newStoreGia()
			h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))
			w := goi(t, h, s, xaA, c.khoa)
			if w.Code != c.muon {
				t.Fatalf("status = %d, muốn %d", w.Code, c.muon)
			}
			if c.muon != http.StatusBadRequest {
				return
			}
			// The message must say what to send, or the integrator guesses.
			if !strings.Contains(w.Body.String(), Header) {
				t.Errorf("thông báo không nhắc tên header: %s", w.Body.String())
			}
			// A rejected request must not consume a key.
			if s.soKhoa() != 0 {
				t.Errorf("yêu cầu bị từ chối vẫn chiếm %d khoá", s.soKhoa())
			}
		})
	}
}

func TestMaLoiTiengAnhThongBaoTiengViet(t *testing.T) {
	// httpx.Error is the ONE failure shape in the system (pkg/httpx/edge.go). Two naming
	// conventions inside one shape is exactly the drift rule 9 exists to prevent:
	//   code    machine-readable identifier, English snake_case
	//   message read by a person, Vietnamese
	docLoi := func(w *httptest.ResponseRecorder) (string, string) {
		t.Helper()
		var e struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			TraceID string `json:"trace_id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
			t.Fatalf("thân lỗi không phải hình dạng httpx.Error: %v (%s)", err, w.Body.String())
		}
		return e.Code, e.Message
	}

	h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))

	// thiếu header
	code, msg := docLoi(goi(t, h, newStoreGia(), xaA, ""))
	if code != "missing_idempotency_key" {
		t.Errorf("code = %q", code)
	}
	if msg == "" {
		t.Error("thông báo rỗng")
	}

	// khoá sai định dạng
	if code, _ = docLoi(goi(t, h, newStoreGia(), xaA, "abc")); code != "invalid_idempotency_key" {
		t.Errorf("code = %q", code)
	}

	// đang chạy dở
	s := newStoreGia()
	s.data[keyGia(xaA, chuTheMacDinh)] = dauDangChay
	if code, _ = docLoi(goi(t, h, s, xaA, khoaKhachHang)); code != "request_in_progress" {
		t.Errorf("code = %q", code)
	}

	// Store hỏng, chế độ đóng
	hong := &storeGia{data: map[string]string{}, loi: errors.New("connection refused")}
	if code, _ = docLoi(goi(t, h, hong, xaA, khoaKhachHang)); code != "idempotency_unavailable" {
		t.Errorf("code = %q", code)
	}
}

func TestKhongCanBoQuaHoanToan(t *testing.T) {
	s := newStoreGia()
	var chay int
	h := KhongCan("xoá một phiên đã xoá cho ra cùng kết quả")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			chay++
			w.WriteHeader(http.StatusNoContent)
		}))

	for i := 0; i < 3; i++ {
		// No header at all, and it must still pass.
		if w := goi(t, h, s, xaA, ""); w.Code != http.StatusNoContent {
			t.Fatalf("lần %d: status = %d, muốn 204", i, w.Code)
		}
	}
	if chay != 3 {
		t.Errorf("handler chạy %d lần, muốn 3", chay)
	}
	if s.soKhoa() != 0 {
		t.Errorf("KhongCan vẫn chạm vào Store: %d khoá", s.soKhoa())
	}
}

func TestKhongCanThieuLyDoThiPanic(t *testing.T) {
	// Must fail at route construction, not at request time: a missing reason stops a
	// deployment, it does not surprise a citizen.
	defer func() {
		if recover() == nil {
			t.Fatal("KhongCan(\"\") phải panic — miễn trừ không lý do là miễn trừ không ai dám gỡ")
		}
	}()
	KhongCan("")
}

func TestRequiredCheDoKhongHopLeThiPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Required với chế độ zero phải panic — không được có giá trị mặc định")
		}
	}()
	Required(CheDoHong(0))
}

func TestStoreHong(t *testing.T) {
	cases := []struct {
		ten    string
		cheDo  CheDoHong
		store  Store
		muon   int
		chayHl bool
	}{
		{"MoKhiHong + Redis lỗi", MoKhiHong, &storeGia{data: map[string]string{},
			loi: errors.New("dial tcp: connection refused")}, http.StatusCreated, true},
		{"DongKhiHong + Redis lỗi", DongKhiHong, &storeGia{data: map[string]string{},
			loi: errors.New("dial tcp: connection refused")}, http.StatusServiceUnavailable, false},
		{"MoKhiHong + chưa cấu hình Redis", MoKhiHong, nil, http.StatusCreated, true},
		{"DongKhiHong + chưa cấu hình Redis", DongKhiHong, nil, http.StatusServiceUnavailable, false},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			var chay bool
			h := Required(c.cheDo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				chay = true
				RecordCode(r.Context(), "PA-7F3K9Q")
				w.WriteHeader(http.StatusCreated)
			}))
			w := goi(t, h, c.store, xaA, khoaKhachHang)
			if w.Code != c.muon {
				t.Fatalf("status = %d, muốn %d", w.Code, c.muon)
			}
			if chay != c.chayHl {
				t.Errorf("handler chạy = %v, muốn %v", chay, c.chayHl)
			}
			if c.muon == http.StatusServiceUnavailable && w.Header().Get("Retry-After") == "" {
				t.Error("503 phải kèm Retry-After")
			}
		})
	}
}

func TestHandlerLoiThiNhaKhoa(t *testing.T) {
	// The write did not happen, so the client must be able to correct the request and send it
	// again with the same key. Holding the claim would refuse them for the whole TTL.
	cases := []int{http.StatusBadRequest, http.StatusUnprocessableEntity,
		http.StatusInternalServerError}
	for _, status := range cases {
		t.Run(http.StatusText(status), func(t *testing.T) {
			s := newStoreGia()
			h := Required(DongKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			if w := goi(t, h, s, xaA, khoaKhachHang); w.Code != status {
				t.Fatalf("status = %d, muốn %d", w.Code, status)
			}
			if s.soKhoa() != 0 {
				t.Fatalf("handler lỗi mà khoá còn lại %d — lần gửi lại sẽ bị từ chối oan", s.soKhoa())
			}
			// And the corrected request goes through.
			h2 := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))
			if w := goi(t, h2, s, xaA, khoaKhachHang); w.Code != http.StatusCreated {
				t.Fatalf("gửi lại sau khi sửa: status = %d, muốn 201", w.Code)
			}
		})
	}
}

func TestHandlerPanicThiNhaKhoa(t *testing.T) {
	// A handler that dies mid-request must not leave its claim behind: every retry would be
	// refused although nothing was ever created.
	s := newStoreGia()
	h := Required(MoKhiHong)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("hỏng giữa chừng")
	}))

	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic phải tiếp tục lan lên httpx.Recover, không được nuốt")
			}
		}()
		goi(t, h, s, xaA, khoaKhachHang)
	}()

	if s.soKhoa() != 0 {
		t.Fatalf("panic để lại %d khoá — mọi lần gửi lại bị từ chối suốt TTL", s.soKhoa())
	}
}

// storeHetHan makes the first Claim report "taken" and the following Get find nothing — the
// key expired between the two calls, or the first attempt released it.
type storeHetHan struct {
	storeGia
	soLanClaim int
}

func (s *storeHetHan) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	s.soLanClaim++
	if s.soLanClaim == 1 {
		return false, nil
	}
	return s.storeGia.Claim(ctx, key, ttl)
}

func TestKhoaBienMatGiuaChungThiChiemLai(t *testing.T) {
	// Refusing here would refuse a request that nothing is actually protecting against: no
	// value is stored, so no first attempt is on record.
	s := &storeHetHan{storeGia: storeGia{data: map[string]string{}}}
	h := Required(MoKhiHong)(handlerTao("PA-7F3K9Q"))

	if w := goi(t, h, s, xaA, khoaKhachHang); w.Code != http.StatusCreated {
		t.Fatalf("status = %d, muốn 201", w.Code)
	}
	if s.soLanClaim != 2 {
		t.Errorf("Claim gọi %d lần, muốn 2 (lần đầu thua, đọc thấy trống, chiếm lại)", s.soLanClaim)
	}
	if got := s.doc(keyGia(xaA, chuTheMacDinh)); got != "2:201:PA-7F3K9Q" {
		t.Errorf("giá trị = %q", got)
	}
}

func TestTachGiaTri(t *testing.T) {
	cases := []struct {
		vao    string
		status int
		ma     string
		ok     bool
	}{
		{"2:201:PA-7F3K9Q", 201, "PA-7F3K9Q", true},
		{"2:200:", 200, "", true},
		{"1", 0, "", false},
		{"2:abc:PA-1", 0, "", false},
		{"2:999:PA-1", 0, "", false},
		{"", 0, "", false},
	}
	for _, c := range cases {
		st, ma, ok := TachGiaTri(c.vao)
		if st != c.status || ma != c.ma || ok != c.ok {
			t.Errorf("TachGiaTri(%q) = (%d, %q, %v), muốn (%d, %q, %v)",
				c.vao, st, ma, ok, c.status, c.ma, c.ok)
		}
	}
}

func TestRecordCodeLocKyTu(t *testing.T) {
	// The code goes into the stored value; a newline or a colon storm must not reshape it.
	s := newStoreGia()
	h := Required(MoKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RecordCode(r.Context(), "PA:7F\n3K 9Q")
		w.WriteHeader(http.StatusCreated)
	}))
	goi(t, h, s, xaA, khoaKhachHang)

	giaTri := s.doc(keyGia(xaA, chuTheMacDinh))
	if giaTri != "2:201:PA7F3K9Q" {
		t.Fatalf("giá trị = %q, muốn mã đã lọc", giaTri)
	}
	if st, ma, ok := TachGiaTri(giaTri); !ok || st != 201 || ma != "PA7F3K9Q" {
		t.Errorf("không đọc lại được: (%d, %q, %v)", st, ma, ok)
	}
}

func TestRecordCodeNgoaiLuongLaKhongLam(t *testing.T) {
	// A handler may call it unconditionally, including on a route declared KhongCan.
	RecordCode(context.Background(), "PA-7F3K9Q")
}

func TestKhongCoXaThiPanic(t *testing.T) {
	// tenant.MustFrom panics by design; httpx.Recover turns it into a traceable 500. A key
	// built without a commune is a key two communes can share.
	defer func() {
		if recover() == nil {
			t.Fatal("thiếu xã trong context phải panic, không được dựng key không tiền tố")
		}
	}()
	h := Required(MoKhiHong)(handlerTao("PA-7F3K9Q"))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/citizen-reports", nil)
	r.Header.Set(Header, khoaKhachHang)
	r = r.WithContext(Into(r.Context(), newStoreGia(), logIm()))
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestMiddlewareGanStoreVaoContext(t *testing.T) {
	s := newStoreGia()
	var h http.Handler = Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))
	h = Middleware(s, logIm())(h)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/citizen-reports", nil)
	r.Header.Set(Header, khoaKhachHang)
	ctx := tenant.Into(r.Context(), tenant.ID(xaA))
	// A principal, because authz wraps outside idem and Required refuses a request without one.
	ctx = authz.Into(ctx, *canBo(xaA, canBo1))
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, muốn 201", w.Code)
	}
	if s.soKhoa() != 1 {
		t.Errorf("Middleware không gắn được Store: %d khoá", s.soKhoa())
	}
}
