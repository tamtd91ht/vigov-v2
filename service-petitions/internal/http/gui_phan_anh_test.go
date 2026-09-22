package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The CITIZEN intake: POST /api/v1/my-citizen-reports
//
// # THIS HARNESS DRIVES THE REAL CITIZEN CHAIN, INCLUDING idem
//
// There is no shortcut that injects a principal, for the reason phieu_cua_toi_test.go gives: the
// thing under test IS how a citizen identity and a commune come to exist on a request. The
// idempotency middleware is in the chain as well, because the route DECLARES duplicate protection
// and a declaration nothing exercises is a claim, not a property.
//
//	PROVED HERE   401 with no usable session and with a session that has chosen no commune, and the
//	              use case is NOT reached in either · a body naming `cong_dan_id` (or any other fact
//	              the client does not decide) is 400 and the use case is NOT reached · the citizen
//	              id handed to the use case is the SESSION'S, even when the body and the query name
//	              somebody else · the commune handed down is the SESSION'S · a commune with no
//	              deadline configuration answers 503 and NO LOOKUP CODE LEAVES · 201 carries the
//	              code, and THAT CODE IS READABLE BACK THROUGH THE EXISTING GET · the response body
//	              is masked · two requests with one Idempotency-Key produce ONE petition and the
//	              second is told the same code · a missing Idempotency-Key is 400.
//
//	NOT PROVED    that the row and its audit entry share a transaction, and what the entry holds.
//	              The use case owns that and internal/app/gui_phan_anh_test.go asserts it against a
//	              driver that records whether a statement ran inside one.

// --- an in-memory petition register that BOTH citizen routes talk to --------------------------
//
// ONE OBJECT BEHIND BOTH Deps FIELDS, deliberately: the assertion that matters most in this file is
// that the code handed back by the POST opens the petition through the GET, and two disconnected
// fakes could not express it.
type soPhieuGia struct {
	mu   sync.Mutex
	theo map[tenant.ID]map[string]domain.PhieuPhanAnh

	// loi is what Gui returns instead of accepting. It stands in for identity refusing.
	loi error

	demGui      int
	thayCongDan []string
	thayXa      []tenant.ID
	thayYeuCau  []app.YeuCauGuiPhanAnh
	thayKind    []string
	thayIP      []string
	dem         int
}

func soPhieuMoi() *soPhieuGia {
	return &soPhieuGia{theo: map[tenant.ID]map[string]domain.PhieuPhanAnh{}}
}

// Gui mirrors the real use case's CONTRACT rather than its implementation: it mints a code only on
// the success path, so the "no code on a refused intake" assertion is about the route and not about
// this fake being lenient.
func (s *soPhieuGia) Gui(ctx context.Context, yc app.YeuCauGuiPhanAnh, congDan audit.Actor) (
	domain.PhieuPhanAnh, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	s.demGui++
	s.thayCongDan = append(s.thayCongDan, congDan.ID)
	s.thayKind = append(s.thayKind, congDan.Kind)
	s.thayIP = append(s.thayIP, congDan.IP)
	s.thayYeuCau = append(s.thayYeuCau, yc)
	// tenant.MustFrom, never tenant.From with a fallback: a write with no commune must be loud.
	xa := tenant.MustFrom(ctx)
	s.thayXa = append(s.thayXa, xa)

	if s.loi != nil {
		return domain.PhieuPhanAnh{}, s.loi
	}

	s.dem++
	// A DIFFERENT CODE EVERY TIME. Were it constant, the duplicate-request test would pass even if
	// the handler ran twice.
	p := domain.PhieuPhanAnh{
		ID:                fmt.Sprintf("pa-%03d", s.dem),
		MaTraCuu:          fmt.Sprintf("PA-4K7M-92XR-BTV%d", s.dem),
		Kenh:              domain.KenhZaloMiniApp,
		CongDanID:         congDan.ID,
		NoiDung:           yc.NoiDung,
		DiaChi:            yc.DiaChi,
		NguoiGuiHoTen:     yc.HoTen,
		NguoiGuiDienThoai: yc.DienThoai,
		AnDanh:            yc.AnDanh,
		TrangThai:         domain.DaTiepNhan,
		GocDemHan:         mocGuiThuGui,
		VaoSoLuc:          mocGuiThuGui,
		HanTiepNhan:       mocHanThuGui,
	}
	if s.theo[xa] == nil {
		s.theo[xa] = map[string]domain.PhieuPhanAnh{}
	}
	s.theo[xa][p.MaTraCuu] = p
	return p, nil
}

// CuaCongDanTheoMaTraCuu is the SAME read the citizen GET route uses, with the same two filters.
func (s *soPhieuGia) CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (
	domain.PhieuPhanAnh, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	if congDanID == "" {
		return domain.PhieuPhanAnh{}, petstore.ErrThieuDinhDanhCongDan
	}
	p, co := s.theo[tenant.MustFrom(ctx)][ma]
	if !co || p.CongDanID != congDanID {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return p, nil
}

var (
	mocGuiThuGui = time.Date(2026, 9, 22, 7, 14, 3, 0, time.UTC)
	mocHanThuGui = time.Date(2026, 9, 23, 2, 30, 0, 0, time.UTC)
)

// --- an in-memory idempotency store -----------------------------------------------------------
//
// The whole point of the idem.Store interface: this suite runs with no Redis, the way
// tenant.Directory lets the edge be tested without the platform service.
type khoIdemGia struct {
	mu   sync.Mutex
	data map[string]string
}

func khoIdemMoi() *khoIdemGia { return &khoIdemGia{data: map[string]string{}} }

func (k *khoIdemGia) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, co := k.data[key]; co {
		return false, nil
	}
	k.data[key] = "1"
	return true, nil
}

func (k *khoIdemGia) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.data[key], nil
}

func (k *khoIdemGia) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.data[key] = value
	return nil
}

func (k *khoIdemGia) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.data, key)
	return nil
}

// --- harness -----------------------------------------------------------------------------------

type mayChuGui struct {
	h   http.Handler
	so  *soPhieuGia
	kho *khoIdemGia
}

func dungMayChuGui(t *testing.T) *mayChuGui {
	t.Helper()

	so := soPhieuMoi()
	kho := khoIdemMoi()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu:       so,
		GuiPhieu:    so,
		NhanLinhVuc: nhanLinhVucMau(),
		Log:         log,
	})

	// THE REAL CHAIN, in the order cmd/server builds it — including idem.Middleware innermost, where
	// cmd/server puts it, because the commune the key is prefixed with arrives per route via
	// httpx.XaTuPhien and not on the chain (ADR 0022).
	var h http.Handler = mux
	h = idem.Middleware(kho, log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(&soPhienCongDanGia{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuGui{h: h, so: so, kho: kho}
}

const duongTapCongDan = "/api/v1/my-citizen-reports"

// thanThu is a well-formed submission. THE NUMBER IS THE AGREED FAKE ONE (rule 3, invariant 5).
const thanThu = `{"content":"Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",` +
	`"address":"Đầu ngõ thôn Hà Lam","reporter_name":"Nguyễn Văn An",` +
	`"reporter_phone":"0900000000"}`

// gui issues one intake request. An empty token means no Authorization header; an empty key means
// no Idempotency-Key header.
func (m *mayChuGui) gui(t *testing.T, than, token, khoa string) *httptest.ResponseRecorder {
	t.Helper()
	return m.guiDuong(t, duongTapCongDan, than, token, khoa)
}

func (m *mayChuGui) guiDuong(t *testing.T, duong, than, token, khoa string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://"+hostMiniApp+duong, strings.NewReader(than))
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if khoa != "" {
		r.Header.Set(idem.Header, khoa)
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func (m *mayChuGui) doc(t *testing.T, duong, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+hostMiniApp+duong, nil)
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

const khoaThu = "01JIDEMPOTENCYKEYCUAKHACHHANG"

// --- CA 1: KHÔNG PHIÊN -> 401, VÀ USE CASE KHÔNG BỊ CHẠM -------------------------------------

// TestGuiKhongPhienLa401VaKhongTiepNhanGi covers the three situations ADR 0022 folds into one
// answer, and asserts the use case was NEVER reached in any of them.
//
// THE SECOND HALF IS THE ONE THAT MATTERS. A route that refused only after running the use case
// would have opened a transaction and asked identity for a commitment on behalf of a request
// carrying no identity at all — and the status code alone cannot tell the two apart.
func TestGuiKhongPhienLa401VaKhongTiepNhanGi(t *testing.T) {
	for ten, token := range map[string]string{
		"không có header Authorization": "",
		"token sổ phiên không nhận":     "token-khong-ai-cap-BAO-GIO",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuGui(t)
			w := m.gui(t, thanThu, token, khoaThu)
			doiMa(t, w, http.StatusUnauthorized)
			if m.so.demGui != 0 {
				t.Errorf("use case bị gọi %d lần dù không có phiên dùng được", m.so.demGui)
			}
		})
	}
}

// TestGuiPhienChuaChonXaLa401 is the third situation, and it is the one that looks like a bug and is
// not: a citizen signed in with NO commune chosen is an ordinary state of the Mini App (ADR 0005).
// httpx.XaTuPhien refuses it with the SAME 401 — no default commune is ever filled in (rule 1,
// forbidden #1), because filing a petition into a guessed commune is filing it with the wrong
// authority.
func TestGuiPhienChuaChonXaLa401(t *testing.T) {
	m := dungMayChuGui(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: m.so, GuiPhieu: m.so, NhanLinhVuc: nhanLinhVucMau(), Log: log,
	})
	var h http.Handler = mux
	h = idem.Middleware(m.kho, log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(phienKhongXa{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	m.h = httpx.StripTenantHeaders(h)

	w := m.gui(t, thanThu, "bat-ky-token-nao", khoaThu)
	doiMa(t, w, http.StatusUnauthorized)
	if m.so.demGui != 0 {
		t.Errorf("use case bị gọi %d lần dù phiên chưa gắn xã nào", m.so.demGui)
	}
}

// --- CA 2: THÂN MANG cong_dan_id -> 400 ------------------------------------------------------

// TestGuiThanMangDinhDanhCongDanLa400 is rule 4, forbidden #1, refused at the edge of the handler.
//
// FOUR SPELLINGS AND TWO LANGUAGES, because a refusal that catches only the English name catches
// only the integrator who read the contract. Every one of them must be a 400 AND must stop before
// the use case: a field that is silently dropped is a field a client keeps sending, believing it
// works, until the day somebody "wires it up".
//
// ĐỘT BIẾN BẮT BUỘC (c): lấy cong_dan_id từ thân yêu cầu — tức bỏ truongKhongPhaiCuaClient và đọc
// vao.CongDanID vào YeuCauGuiPhanAnh — và ca này ĐỎ, cùng với
// TestGuiDinhDanhTuPHIENChuKhongPhaiTuThan bên dưới.
func TestGuiThanMangDinhDanhCongDanLa400(t *testing.T) {
	than := map[string]string{
		"cong_dan_id":    `{"content":"x","cong_dan_id":"cd-01JNGUOIKHAC"}`,
		"citizen_id":     `{"content":"x","citizen_id":"cd-01JNGUOIKHAC"}`,
		"lĩnh vực":       `{"content":"x","field":"an-ninh-trat-tu"}`,
		"linh_vuc":       `{"content":"x","linh_vuc":"an-ninh-trat-tu"}`,
		"kênh":           `{"content":"x","channel":"can-bo-nhap-ho"}`,
		"mã tra cứu":     `{"content":"x","code":"PA-0000-0000-0000"}`,
		"trạng thái":     `{"content":"x","status":"da-dong"}`,
		"gốc đếm hạn":    `{"content":"x","clock_from":"2020-01-01T00:00:00Z"}`,
		"hạn tiếp nhận":  `{"content":"x","acknowledge_due":"2099-01-01T00:00:00Z"}`,
		"hạn xử lý xong": `{"content":"x","resolve_due":"2099-01-01T00:00:00Z"}`,
	}
	for ten, b := range than {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuGui(t)
			w := m.gui(t, b, tokenCuaToi, khoaThu)
			doiMa(t, w, http.StatusBadRequest)
			if m.so.demGui != 0 {
				t.Errorf("use case bị gọi %d lần dù thân yêu cầu đòi quyết một việc "+
					"không phải của client", m.so.demGui)
			}
		})
	}
}

// TestGuiDinhDanhTuPHIENChuKhongPhaiTuThan is the other half of the same rule, and the sharper one:
// even with a body the handler ACCEPTS, the identity that reaches the use case must be the session's.
//
// The attacker holds their OWN valid session and names the victim in the query string — the one
// place left once the body fields are refused.
func TestGuiDinhDanhTuPHIENChuKhongPhaiTuThan(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.guiDuong(t, duongTapCongDan+"?cong_dan_id="+idToi, thanThu, tokenNguoiKhac, khoaThu)
	doiMa(t, w, http.StatusCreated)

	if len(m.so.thayCongDan) != 1 {
		t.Fatalf("use case được gọi %d lần, muốn 1", len(m.so.thayCongDan))
	}
	if m.so.thayCongDan[0] != idNguoiKhac {
		t.Errorf("định danh gửi xuống use case = %q, muốn %q (định danh CỦA PHIÊN) — "+
			"một giá trị do client gửi đã lọt vào đường cách ly (luật 4 cấm #1)",
			m.so.thayCongDan[0], idNguoiKhac)
	}
	// Kind and IP are what the audit entry is built from (rule 6, invariant 2). A citizen's act
	// recorded as a staff one makes the whole ledger unable to answer "who did this".
	if m.so.thayKind[0] != "citizen" {
		t.Errorf("kind = %q, muốn %q", m.so.thayKind[0], "citizen")
	}
	if m.so.thayIP[0] != "10.0.0.9" {
		t.Errorf("IP = %q, muốn địa chỉ socket của tiến trình này, không phải X-Forwarded-For",
			m.so.thayIP[0])
	}
	// THE COMMUNE IS THE SESSION'S TOO (ADR 0022). Asserted separately from the identity: a handler
	// that got one right and the other wrong would still leak across an authority boundary.
	if m.so.thayXa[0] != xaA {
		t.Errorf("xã đến use case = %q, muốn %q", m.so.thayXa[0], xaA)
	}
}

// --- CA 3: XÃ CHƯA CẤU HÌNH SLA -> TỪ CHỐI, KHÔNG CẤP MÃ -------------------------------------

// TestGuiXaChuaCauHinhHanLa503VaKhongCoMaTraCuu is TODAY'S ANSWER FOR EVERY COMMUNE: the deadline
// table is seeded by nothing, so identity refuses with FAILED_PRECONDITION and this route refuses
// the intake.
//
// THREE ASSERTIONS, THREE DIFFERENT PROMISES:
//
//	503 and not 500   nothing is broken; a human fills in a configuration screen
//	no lookup code    rule 7, invariant 3 — an issued code is never reissued, so a code handed out
//	                  for a petition that does not exist can never be cleaned up
//	no internal word  rule 3, forbidden #3 — the citizen reads a sentence they can act on
//
// ĐỘT BIẾN BẮT BUỘC (b) sống ở tầng use case; đây là nửa HTTP của nó.
func TestGuiXaChuaCauHinhHanLa503VaKhongCoMaTraCuu(t *testing.T) {
	m := dungMayChuGui(t)
	m.so.loi = fmt.Errorf("%w cho xã %s: rpc error: code = FailedPrecondition desc = "+
		"xã chưa cấu hình bảng thời hạn xử lý", app.ErrChuaAnDinhDuocHan, xaA)

	w := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusServiceUnavailable)

	than := w.Body.String()
	// NO LOOKUP CODE, ASSERTED ON THE RAW BODY. The `code` KEY is present — httpx.Error uses it for
	// the machine-readable error name — so the assertion has to be about the VALUE shape a lookup
	// code has, not about the key. Checking the key would be a test that passes for the wrong
	// reason, or fails for one.
	if strings.Contains(than, domain.TienToMaTraCuu+"-") {
		t.Errorf("MỘT MÃ TRA CỨU rời khỏi một lượt tiếp nhận đã hỏng: %s", than)
	}
	if len(m.so.theo[xaA]) != 0 {
		t.Errorf("ghi %d phiếu dù lượt tiếp nhận đã hỏng", len(m.so.theo[xaA]))
	}
	// The citizen must be told plainly that nothing was recorded — a vague "try again" leaves them
	// believing the commune has their report.
	if !strings.Contains(than, "CHƯA được ghi nhận") {
		t.Errorf("thông điệp không nói rõ phiếu CHƯA được ghi nhận: %s", than)
	}
	for _, bi := range []string{"sla", "SLA", "identity", "rpc", "FailedPrecondition", "grpc"} {
		if strings.Contains(than, bi) {
			t.Errorf("chi tiết nội bộ %q lọt ra thân trả lời cho công dân: %s", bi, than)
		}
	}
	// AND NO Retry-After: retrying in a second cannot help, and a number here would be a second
	// promise nobody can keep.
	if w.Header().Get("Retry-After") != "" {
		t.Errorf("Retry-After = %q — hứa một mốc thử lại mà không ai giữ được",
			w.Header().Get("Retry-After"))
	}
}

// TestGuiLoiHeThongLa500VaKhongLoDuLieu — a failure that is not a configuration gap is a 500 whose
// body says nothing about the petition or the sender.
func TestGuiLoiHeThongLa500VaKhongLoDuLieu(t *testing.T) {
	m := dungMayChuGui(t)
	m.so.loi = fmt.Errorf("pg: connection refused khi ghi phiếu của Nguyễn Văn An 0900000000")

	w := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusInternalServerError)

	than := w.Body.String()
	for _, bi := range []string{"Nguyễn", "0900000000", "connection refused"} {
		if strings.Contains(than, bi) {
			t.Errorf("chi tiết nội bộ %q lọt ra thân lỗi: %s", bi, than)
		}
	}
}

// --- CA 4: GỬI ĐƯỢC -> 201 KÈM MÃ, VÀ MÃ ẤY TRA LẠI ĐƯỢC -------------------------------------

// TestGuiThanhCongLa201VaMaTraCuuTraLaiDuoc is rule 10, invariant 1 end to end.
//
// THE SECOND HALF IS THE ONE WORTH HAVING. A 201 carrying a code proves the response; reading that
// same code back through the EXISTING GET route proves the code is the one the commune actually
// stored — which is the whole of what the citizen was promised. A handler that returned a code it
// never wrote would pass the first assertion and fail this one.
func TestGuiThanhCongLa201VaMaTraCuuTraLaiDuoc(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.Code == "" {
		t.Fatal("201 KHÔNG kèm mã tra cứu — mã là toàn bộ cam kết trả cho công dân (luật 10 bất biến 1)")
	}
	if !strings.HasPrefix(ra.Code, domain.TienToMaTraCuu+"-") {
		t.Errorf("mã tra cứu = %q, không mang tiền tố %q", ra.Code, domain.TienToMaTraCuu)
	}
	if ra.Status != string(domain.DaTiepNhan) {
		t.Errorf("status = %q, muốn %q", ra.Status, domain.DaTiepNhan)
	}
	if ra.Channel != string(domain.KenhZaloMiniApp) {
		t.Errorf("channel = %q, muốn %q — kênh là hằng trong use case, không lấy từ thân",
			ra.Channel, domain.KenhZaloMiniApp)
	}
	// THE TWO NULLS KEEP THEIR OPPOSITE MEANINGS on the very first response: the acknowledge
	// commitment EXISTS, the resolve one does not yet (ADR 0028 decision E).
	if ra.AcknowledgeDue == nil || !ra.AcknowledgeDue.Equal(mocHanThuGui) {
		t.Errorf("acknowledge_due = %v, muốn %v — cam kết đã ấn định phải trả về ngay",
			ra.AcknowledgeDue, mocHanThuGui)
	}
	if ra.ResolveDue != nil {
		t.Errorf("resolve_due = %v — hạn xử lý xong chỉ được ấn định lúc cán bộ chốt lĩnh vực",
			ra.ResolveDue)
	}
	if ra.Field != "" || ra.FieldLabel != "" {
		t.Errorf("lĩnh vực = %q / nhãn = %q — công dân không chọn lĩnh vực", ra.Field, ra.FieldLabel)
	}

	// THE CODE OPENS THE PETITION THROUGH THE ROUTE THE CITIZEN WILL ACTUALLY USE.
	doc := m.doc(t, duongTapCongDan+"/"+ra.Code, tokenCuaToi)
	doiMa(t, doc, http.StatusOK)
	lai := docPhieuCuaToi(t, doc.Body.Bytes())
	if lai.Code != ra.Code {
		t.Errorf("tra lại mã %q ra phiếu %q", ra.Code, lai.Code)
	}

	// AND THE TWO BODIES ARE THE SAME SHAPE — one fact, one representation (rule 9). A Mini App that
	// renders the submit result and the lookup result from one type is a Mini App that cannot show a
	// citizen two different versions of their own petition.
	if !bytes.Equal(w.Body.Bytes(), doc.Body.Bytes()) {
		t.Errorf("thân 201 và thân 200 khác nhau:\n  201: %s\n  200: %s",
			w.Body.String(), doc.Body.String())
	}
}

// TestGuiPhanHoiDaCheDuLieuCaNhan — the contact details come back masked even though the citizen
// just typed them. The two reasons are on phieuCuaToiRa.ReporterPhone and they hold on this surface
// for the same reason: there is no permission on a citizen route that could open them, and
// "no permission to check" resolves to masked, never to open.
func TestGuiPhanHoiDaCheDuLieuCaNhan(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	than := w.Body.String()
	if strings.Contains(than, "0900000000") {
		t.Errorf("SỐ ĐIỆN THOẠI ĐẦY ĐỦ lọt ra phản hồi tiếp nhận: %s", than)
	}
	if strings.Contains(than, "Nguyễn Văn An") {
		t.Errorf("HỌ TÊN ĐẦY ĐỦ lọt ra phản hồi tiếp nhận: %s", than)
	}

	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.ReporterPhone != "09****0000" || ra.ReporterName != "Nguyễn V. A." {
		t.Errorf("người gửi chưa che đúng: %q / %q", ra.ReporterName, ra.ReporterPhone)
	}

	// No internal field, and no derived-overdue field (rule 10, invariant 3).
	var tho map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", than)
	}
	for _, khoa := range []string{"id", "bo_phan_id", "can_bo_xu_ly_id", "vao_so_luc",
		"hien_cong_khai", "overdue", "is_overdue", "qua_han"} {
		if _, co := tho[khoa]; co {
			t.Errorf("trường nội bộ %q lọt ra phản hồi tiếp nhận: %s", khoa, than)
		}
	}
}

// --- CA 5: HAI LẦN GỬI CÙNG MỘT KHOÁ -> MỘT PHIẾU --------------------------------------------

// TestGuiHaiLanCungKhoaChiRaMotPhieu is the promise the route's idem declaration makes.
//
// A double-tapped `Gửi` is the ordinary case, not an attack, and rule 7 makes its consequence
// permanent: the duplicate petition can only be soft-deleted and the code it consumed is never
// reissued.
//
// THE SECOND RESPONSE MUST STILL CARRY THE CODE. A bare 409 would leave the citizen without the one
// string they were promised while their petition sits in the system — rule 10, invariant 1 broken
// by the very mechanism meant to protect it (core/idem.RecordCode exists for exactly this).
//
// ĐỘT BIẾN: bỏ `idem.Required(idem.MoKhiHong)` khỏi câu lệnh route và ca này ĐỎ với HAI phiếu.
func TestGuiHaiLanCungKhoaChiRaMotPhieu(t *testing.T) {
	m := dungMayChuGui(t)

	mot := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, mot, http.StatusCreated)
	ma := docPhieuCuaToi(t, mot.Body.Bytes()).Code

	hai := m.gui(t, thanThu, tokenCuaToi, khoaThu)

	if m.so.demGui != 1 {
		t.Fatalf("use case chạy %d lần với cùng một Idempotency-Key — một lần bấm hai lần "+
			"thành HAI phiếu, và xoá mềm là cách duy nhất gỡ (luật 7)", m.so.demGui)
	}
	if hai.Header().Get(idem.HeaderPhatLai) != "true" {
		t.Errorf("phản hồi thứ hai không đánh dấu phát lại: %v", hai.Header())
	}
	if !strings.Contains(hai.Body.String(), ma) {
		t.Errorf("phản hồi phát lại KHÔNG mang mã tra cứu %q — công dân không bao giờ biết mã "+
			"của phiếu đang nằm trong hệ thống: %s", ma, hai.Body.String())
	}
}

// TestGuiHaiKhoaKhacNhauRaHaiPhieu is the half that makes the test above mean something: without it
// a handler that simply refused every second request would pass.
func TestGuiHaiKhoaKhacNhauRaHaiPhieu(t *testing.T) {
	m := dungMayChuGui(t)

	doiMa(t, m.gui(t, thanThu, tokenCuaToi, khoaThu), http.StatusCreated)
	doiMa(t, m.gui(t, thanThu, tokenCuaToi, khoaThu+"-KHAC"), http.StatusCreated)

	if m.so.demGui != 2 {
		t.Errorf("use case chạy %d lần với hai khoá khác nhau, muốn 2 — hai lần phản ánh thật "+
			"của cùng một người bị nuốt mất một", m.so.demGui)
	}
}

// TestGuiHaiCONGDANCungKhoaRaHaiPhieu is the isolation half of the idempotency key, and the reason
// idem.Required refuses a request with no principal: the key is CLIENT-GENERATED, so two citizens
// can present the same string. If the actor were not in the hash, the second would be handed the
// FIRST ONE'S lookup code — one citizen reading another citizen's petition through a cache key.
func TestGuiHaiCongDanCungKhoaRaHaiPhieu(t *testing.T) {
	m := dungMayChuGui(t)

	mot := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	hai := m.gui(t, thanThu, tokenNguoiKhac, khoaThu)

	doiMa(t, mot, http.StatusCreated)
	doiMa(t, hai, http.StatusCreated)
	if m.so.demGui != 2 {
		t.Fatalf("use case chạy %d lần cho HAI công dân dùng cùng một khoá, muốn 2", m.so.demGui)
	}

	maMot := docPhieuCuaToi(t, mot.Body.Bytes()).Code
	maHai := docPhieuCuaToi(t, hai.Body.Bytes()).Code
	if maMot == maHai {
		t.Errorf("hai công dân nhận CÙNG một mã tra cứu %q — người thứ hai tra ra phiếu của "+
			"người thứ nhất (luật 4 bất biến 1)", maMot)
	}
}

// TestGuiThieuKhoaChongTrungLa400 — the header is mandatory on this route, and the message says what
// to send. Without it there is nothing for the middleware to key on and the protection the route
// declares would be a claim rather than a property.
func TestGuiThieuKhoaChongTrungLa400(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, thanThu, tokenCuaToi, "")
	doiMa(t, w, http.StatusBadRequest)
	if m.so.demGui != 0 {
		t.Errorf("use case chạy %d lần dù thiếu khoá chống trùng", m.so.demGui)
	}
	if !strings.Contains(w.Body.String(), idem.Header) {
		t.Errorf("thông điệp không nói thiếu header nào: %s", w.Body.String())
	}
}

// --- CA 6: ĐẦU VÀO SAI --------------------------------------------------------------------------

func TestGuiNoiDungRongLa400(t *testing.T) {
	m := dungMayChuGui(t)
	m.so.loi = domain.ErrNoiDungTrong

	w := m.gui(t, `{"content":"   "}`, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusBadRequest)
}

func TestGuiThanKhongPhaiJSONLa400(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, `{"content":`, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusBadRequest)
	if m.so.demGui != 0 {
		t.Errorf("use case chạy %d lần với thân hỏng", m.so.demGui)
	}
}

// --- lắp ráp -------------------------------------------------------------------------------------

// TestRegisterCongDanThieuUseCaseGuiThiPanic — a nil here does not crash a screen, it crashes the
// act rule 10 exists for, in front of a member of the public who has just typed out a complaint.
func TestRegisterCongDanThieuUseCaseGuiThiPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("dựng được tuyến công dân mà không có use case tiếp nhận — " +
				"POST sẽ panic trước mặt người dân")
		}
	}()
	RegisterCongDan(http.NewServeMux(), DepsCongDan{
		Phieu: phieuCuaToiMau(), NhanLinhVuc: nhanLinhVucMau(),
	})
}
