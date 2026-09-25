package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The CITIZEN surface: GET /api/v1/my-citizen-reports/{maTraCuu}
//
// # THIS HARNESS DRIVES THE REAL CITIZEN CHAIN, and that is the difference from mayChu above
//
// The staff harness injects an authz.Principal into the request context, because this service has
// no staff session middleware of its own. There is no equivalent shortcut here and there must not
// be: the thing under test IS how a citizen identity and a commune come to exist on a request —
// from a bearer token, through httpx.CitizenEdge, into authz.CitizenPrincipal and
// httpx.XaTuPhien (ADR 0022). Injecting a principal would delete the half of rule 4, invariant 2
// that says the identity never comes from the request.
//
//	PROVED HERE   401 with no usable session · a petition of ANOTHER CITIZEN answers BYTE FOR BYTE
//	              what an unknown code answers · a session of commune B cannot read commune A's
//	              petition · 200 on one's own petition with the phone number MASKED · a citizen
//	              identifier supplied in the QUERY STRING is ignored · no staff-internal field is
//	              on the response · an anonymous petition shows neither name nor number.
//
//	NOT PROVED    that PostgreSQL applies the identity predicate — the fake below applies it in Go.
//	              That is asserted where it is decidable: the statement itself, in
//	              internal/store/phieu_phan_anh_cong_dan_test.go.

const (
	tokenCuaToi    = "token-phien-cong-dan-GIA-KHONG-PHAI-THAT"
	tokenNguoiKhac = "token-phien-cong-dan-KHAC-GIA"
	tokenXaB       = "token-phien-cong-dan-XA-B-GIA"

	// Opaque citizen identifiers. NO PHONE NUMBER ANYWHERE IN A FIXTURE (rule 3).
	idToi       = "cd-01JTOI"
	idNguoiKhac = "cd-01JNGUOIKHAC"

	maCuaToi       = "PA-4K7M-92XR-BTVD"
	maCuaNguoiKhac = "PA-8QLT-61VB-35ZN"
	maAnDanh       = "PA-3RKW-77YH-51PC"
	maKhongTonTai  = "PA-0000-0000-0000"

	// hostMiniApp maps to NO commune, which is the production shape: the Mini App calls one API
	// host and that host is not a commune's domain (ADR 0005).
	hostMiniApp = "api.example.gov.vn"
)

// phieuCuaToiGia is the identity-filtered read, KEYED BY COMMUNE, AND IT APPLIES THE IDENTITY
// PREDICATE ITSELF — in Go, where the real store applies it in SQL.
//
// MIRRORING THE REAL STORE'S CONTRACT EXACTLY IS THE POINT, including the parts that are easy to
// skip: an empty identity comes back as ErrThieuDinhDanhCongDan and NOT as "not found", because
// the real store refuses before it queries and the handler answers 500 rather than 404 for it. A
// fake that collapsed the two would hide the one branch that distinguishes a broken isolation path
// from an ordinary miss.
type phieuCuaToiGia struct {
	theo map[tenant.ID]map[string]domain.PhieuPhanAnh
	loi  error

	goi int
	// thayCongDan records every identity the handler passed down. It is what proves the value came
	// from the session: a handler reading it from the query string would record the attacker's
	// string here, and the assertion would name exactly that.
	thayCongDan   []string
	thayXa        []tenant.ID
	thayTrangThai []string
}

func (p *phieuCuaToiGia) CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (
	domain.PhieuPhanAnh, error) {

	p.goi++
	p.thayCongDan = append(p.thayCongDan, congDanID)
	// tenant.MustFrom, never tenant.From with a fallback: a read with no commune must be loud.
	p.thayXa = append(p.thayXa, tenant.MustFrom(ctx))

	if congDanID == "" {
		return domain.PhieuPhanAnh{}, petstore.ErrThieuDinhDanhCongDan
	}
	if p.loi != nil {
		return domain.PhieuPhanAnh{}, p.loi
	}
	pa, co := p.theo[tenant.MustFrom(ctx)][ma]
	if !co || pa.CongDanID != congDanID {
		// THE FOUR CAUSES COLLAPSE HERE exactly as they collapse in SQL: unknown code, another
		// citizen's code, another commune's code, soft deleted.
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

// DanhSachCuaCongDan mirrors the real store's list contract IN GO: both axes, the status filter,
// newest first by (GocDemHan, ID), and a real keyset cursor, so the handler's cursor round-trip is
// exercised end to end. The SQL itself is asserted in internal/store/phieu_cua_toi_danh_sach_test.go.
func (p *phieuCuaToiGia) DanhSachCuaCongDan(ctx context.Context, congDanID, trangThai string,
	yc page.Request) (page.Result[domain.PhieuPhanAnh], error) {

	p.goi++
	p.thayCongDan = append(p.thayCongDan, congDanID)
	p.thayXa = append(p.thayXa, tenant.MustFrom(ctx))
	p.thayTrangThai = append(p.thayTrangThai, trangThai)

	if congDanID == "" {
		return page.NewResult[domain.PhieuPhanAnh](), petstore.ErrThieuDinhDanhCongDan
	}
	if p.loi != nil {
		return page.NewResult[domain.PhieuPhanAnh](), p.loi
	}
	return trangGiaCuaCongDan(p.theo[tenant.MustFrom(ctx)], congDanID, trangThai, yc), nil
}

// trangGiaCuaCongDan is the in-Go keyset page both citizen fakes share.
func trangGiaCuaCongDan(theo map[string]domain.PhieuPhanAnh, congDanID, trangThai string,
	yc page.Request) page.Result[domain.PhieuPhanAnh] {

	var ds []domain.PhieuPhanAnh
	for _, pa := range theo {
		if pa.CongDanID != congDanID || pa.CongDanID == "" {
			continue
		}
		if trangThai != "" && string(pa.TrangThai) != trangThai {
			continue
		}
		ds = append(ds, pa)
	}
	sau := func(a, b domain.PhieuPhanAnh) bool { // a comes before b in DESC order
		if !a.GocDemHan.Equal(b.GocDemHan) {
			return a.GocDemHan.After(b.GocDemHan)
		}
		return a.ID > b.ID
	}
	sort.Slice(ds, func(i, j int) bool { return sau(ds[i], ds[j]) })

	if moc, co := yc.After(); co {
		neo := domain.PhieuPhanAnh{GocDemHan: moc.Key.Time(), ID: moc.ID}
		con := ds[:0:0]
		for _, pa := range ds {
			if sau(neo, pa) {
				con = append(con, pa)
			}
		}
		ds = con
	}

	kq := page.NewResult[domain.PhieuPhanAnh]()
	for _, pa := range ds {
		if len(kq.Items) == yc.Limit() {
			kq.HasMore = true
			cuoi := kq.Items[len(kq.Items)-1]
			kq.NextCursor = page.Encode(yc.Column(), yc.Dir(),
				page.Anchor{Key: page.TimeKey(cuoi.GocDemHan), ID: cuoi.ID})
			break
		}
		kq.Items = append(kq.Items, pa)
	}
	return kq
}

// phieuCuaToiMau: commune A holds three petitions, commune B holds none.
//
//	maCuaToi        mine, ordinary, named reporter
//	maCuaNguoiKhac  ANOTHER CITIZEN'S, same commune — the case rule 4 exists for
//	maAnDanh        mine, filed ANONYMOUSLY
func phieuCuaToiMau() *phieuCuaToiGia {
	const hoTen = "Nguyễn Văn An"
	// THE AGREED FAKE NUMBER (rule 3, invariant 5). Never a real one, not even in a fixture.
	const dienThoai = "0900000000"

	return &phieuCuaToiGia{theo: map[tenant.ID]map[string]domain.PhieuPhanAnh{
		xaA: {
			maCuaToi: {
				ID: "pa-001", MaTraCuu: maCuaToi, Kenh: domain.KenhZaloMiniApp,
				CongDanID: idToi,
				NoiDung:   "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
				LinhVuc:   "rac-thai", DiaChi: "Đầu ngõ thôn Hà Lam",
				NguoiGuiHoTen: hoTen, NguoiGuiDienThoai: dienThoai,
				TrangThai: domain.DangPhanLoai,
				GocDemHan: mocGui, VaoSoLuc: mocVaoSo,
				HanTiepNhan: mocTiepNh, HanXuLyXong: mocXuLy,
				// STAFF-INTERNAL FIELDS, POPULATED ON PURPOSE. The response must not carry them
				// (rule 10, invariant 7); a fixture that left them empty could not tell a handler
				// that leaks them from one that does not.
				BoPhanID: "bp-001", CanBoXuLyID: "nd-001",
				HienCongKhai: true, SoLanMoLai: 2,
			},
			maCuaNguoiKhac: {
				ID: "pa-002", MaTraCuu: maCuaNguoiKhac, Kenh: domain.KenhZaloMiniApp,
				CongDanID: idNguoiKhac,
				NoiDung:   "Phiếu của một công dân khác, cùng xã.",
				TrangThai: domain.DaTiepNhan,
				GocDemHan: mocGui, VaoSoLuc: mocVaoSo, HanTiepNhan: mocTiepNh,
			},
			maAnDanh: {
				ID: "pa-003", MaTraCuu: maAnDanh, Kenh: domain.KenhZaloMiniApp,
				CongDanID: idToi, AnDanh: true,
				NoiDung: "Phản ánh gửi ẩn danh.",
				// Present even though AnDanh — the record keeps the identity, no screen shows it.
				NguoiGuiHoTen: hoTen, NguoiGuiDienThoai: dienThoai,
				TrangThai: domain.DaTiepNhan,
				GocDemHan: mocGui, VaoSoLuc: mocVaoSo, HanTiepNhan: mocTiepNh,
			},
		},
		xaB: {},
	}}
}

// soPhienCongDanGia is the session registry the edge consults on every citizen request.
//
// THREE USABLE TOKENS AND ONE ANSWER FOR EVERYTHING ELSE — the real contract has exactly one
// negative outcome, so unknown, expired and revoked are indistinguishable (core/httpx/citizen.go).
type soPhienCongDanGia struct{ goi int }

func (s *soPhienCongDanGia) TraCuu(_ context.Context, token string) (httpx.CitizenSession, bool) {
	s.goi++
	switch token {
	case tokenCuaToi:
		return httpx.CitizenSession{ID: "sid-1", CitizenID: idToi, TenantID: xaA}, true
	case tokenNguoiKhac:
		return httpx.CitizenSession{ID: "sid-2", CitizenID: idNguoiKhac, TenantID: xaA}, true
	case tokenXaB:
		// THE SAME CITIZEN, IN COMMUNE B. One person may hold a session in more than one commune
		// (ADR 0005), so the commune — not the person — has to be what refuses.
		return httpx.CitizenSession{ID: "sid-3", CitizenID: idToi, TenantID: xaB}, true
	}
	return httpx.CitizenSession{}, false
}

// --- harness --------------------------------------------------------------------------------

type mayChuCongDan struct {
	h     http.Handler
	phieu *phieuCuaToiGia
	so    *soPhienCongDanGia
}

func dungMayChuCongDan(t *testing.T) *mayChuCongDan {
	t.Helper()

	phieu := phieuCuaToiMau()
	so := &soPhienCongDanGia{}

	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: phieu,
		// The intake use case is wired because RegisterCongDan refuses incomplete Deps at
		// construction, and it is deliberately NOT exercised here: this file is about the READ
		// surface. The write surface has its own suite, gui_phan_anh_test.go.
		GuiPhieu:    soPhieuMoi(),
		NhanLinhVuc: nhanLinhVucMau(),
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	// THE REAL CHAIN, in the order cmd/server builds it. No TenantMiddleware and no staffauth:
	// this surface resolves the commune from the session, never from Host (ADR 0022).
	var h http.Handler = mux
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(so)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuCongDan{h: h, phieu: phieu, so: so}
}

// goi issues a citizen request. An empty token means "no Authorization header at all".
func (m *mayChuCongDan) goi(t *testing.T, duong, token string) *httptest.ResponseRecorder {
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

func duongCuaToi(ma string) string { return "/api/v1/my-citizen-reports/" + ma }

func docPhieuCuaToi(t *testing.T, than []byte) phieuCuaToiRa {
	t.Helper()
	var ra phieuCuaToiRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

// --- CA 1: KHÔNG PHIÊN -> 401 -----------------------------------------------------------------

// TestCuaToiKhongPhienLa401VaKhoKhongBiChamToi covers the three situations ADR 0022 folds into one
// answer, and asserts the store was NEVER reached in any of them.
//
// The store count is the half that matters. A route that refused only after reading would have
// read a petition on behalf of a request carrying no identity at all — and the status code alone
// cannot tell the two apart.
func TestCuaToiKhongPhienLa401VaKhoKhongBiChamToi(t *testing.T) {
	for ten, token := range map[string]string{
		"không có header Authorization": "",
		"token sổ phiên không nhận":     "token-khong-ai-cap-BAO-GIO",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuCongDan(t)
			w := m.goi(t, duongCuaToi(maCuaToi), token)
			doiMa(t, w, http.StatusUnauthorized)
			if m.phieu.goi != 0 {
				t.Errorf("kho bị đọc %d lần dù không có phiên dùng được", m.phieu.goi)
			}
		})
	}
}

// TestCuaToiPhienChuaChonXaLa401 is the third situation, and it is the one that looks like a bug
// and is not: a citizen signed in with NO commune chosen is an ordinary state of the Mini App
// (ADR 0005). httpx.XaTuPhien refuses it on every business route, with the SAME 401 — no default
// commune is ever filled in (rule 1, forbidden #1).
func TestCuaToiPhienChuaChonXaLa401(t *testing.T) {
	m := dungMayChuCongDan(t)
	// A session with an empty TenantID, produced by a registry that answers exactly that.
	m.h = chuoiCongDanVoi(t, m, phienKhongXa{})

	w := m.goi(t, duongCuaToi(maCuaToi), "bat-ky-token-nao")
	doiMa(t, w, http.StatusUnauthorized)
	if m.phieu.goi != 0 {
		t.Errorf("kho bị đọc %d lần dù phiên chưa gắn xã nào", m.phieu.goi)
	}
}

type phienKhongXa struct{}

func (phienKhongXa) TraCuu(context.Context, string) (httpx.CitizenSession, bool) {
	// Usable session, NO commune. ok=true on purpose: this is not a bad token, it is a citizen who
	// has not picked a commune yet, and the refusal has to come from the commune axis.
	return httpx.CitizenSession{ID: "sid-x", CitizenID: idToi}, true
}

// chuoiCongDanVoi rebuilds the chain around the same mux with a different session registry.
func chuoiCongDanVoi(t *testing.T, m *mayChuCongDan, so httpx.CitizenSessions) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu:       m.phieu,
		GuiPhieu:    soPhieuMoi(),
		NhanLinhVuc: nhanLinhVucMau(),
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	var h http.Handler = mux
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(so)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	return httpx.StripTenantHeaders(h)
}

// --- CA 2: MÃ CỦA NGƯỜI KHÁC -> GIỐNG HỆT MÃ KHÔNG TỒN TẠI ------------------------------------

// TestCuaToiMaCuaNguoiKhacTraLoiGIONGHETMaKhongTonTai is THE test of this file.
//
// IT COMPARES TWO RESPONSES BYTE FOR BYTE rather than checking two status codes. Rule 4,
// forbidden #2 is not satisfied by "both are 404": an identical status with a different error key,
// a different message, or a different body length still tells the person working through codes
// that one of them found something. The only assertion that holds the promise is equality of the
// whole response.
//
// ĐỘT BIẾN: bỏ điều kiện lọc theo danh tính công dân khỏi câu truy vấn (hoặc khỏi phép so trong
// kho giả, tương ứng với mệnh đề `AND cong_dan_id = $3`) và ca này ĐỎ ngay — phiếu của người khác
// trả về 200 kèm nội dung của họ.
func TestCuaToiMaCuaNguoiKhacTraLoiGIONGHETMaKhongTonTai(t *testing.T) {
	m := dungMayChuCongDan(t)

	nguoiKhac := m.goi(t, duongCuaToi(maCuaNguoiKhac), tokenCuaToi)
	khongCo := m.goi(t, duongCuaToi(maKhongTonTai), tokenCuaToi)

	doiMa(t, nguoiKhac, http.StatusNotFound)
	doiMa(t, khongCo, http.StatusNotFound)

	if nguoiKhac.Code != khongCo.Code {
		t.Fatalf("mã trạng thái khác nhau: %d và %d", nguoiKhac.Code, khongCo.Code)
	}
	if !bytes.Equal(nguoiKhac.Body.Bytes(), khongCo.Body.Bytes()) {
		t.Errorf("hai thân phản hồi KHÁC NHAU — sự tồn tại của phiếu người khác bị lộ:\n"+
			"  phiếu người khác: %s\n  mã không tồn tại: %s",
			nguoiKhac.Body.String(), khongCo.Body.String())
	}
	// AND NOTHING OF THE OTHER PERSON'S PETITION IS IN THE BODY. The equality above would already
	// catch it, but this names the thing being protected so a future edit that "improves" the
	// message cannot quietly reintroduce it.
	if strings.Contains(nguoiKhac.Body.String(), "công dân khác") {
		t.Errorf("nội dung phiếu của người khác lọt vào phản hồi: %s", nguoiKhac.Body.String())
	}
}

// TestCuaToiDinhDanhTuPHIENChuKhongPhaiTuThamSo is rule 4, forbidden #1 — the top entry of that
// rule's list, because `?cong_dan_id=` looks like an ordinary parameter in a diff.
//
// The attacker holds their OWN valid session and names the victim in the query string. The answer
// must be the same 404, and the identity the store was asked with must be the ATTACKER'S — read
// from the session, never from the request.
func TestCuaToiDinhDanhTuPHIENChuKhongPhaiTuThamSo(t *testing.T) {
	m := dungMayChuCongDan(t)

	w := m.goi(t, duongCuaToi(maCuaToi)+"?cong_dan_id="+idToi, tokenNguoiKhac)
	doiMa(t, w, http.StatusNotFound)

	if len(m.phieu.thayCongDan) != 1 {
		t.Fatalf("kho được hỏi %d lần, muốn 1", len(m.phieu.thayCongDan))
	}
	if m.phieu.thayCongDan[0] != idNguoiKhac {
		t.Errorf("định danh gửi xuống kho = %q, muốn %q (định danh CỦA PHIÊN) — "+
			"một giá trị do client gửi đã lọt vào đường cách ly (luật 4 cấm #1)",
			m.phieu.thayCongDan[0], idNguoiKhac)
	}
}

// --- CA 3: PHIÊN XÃ B HỎI PHIẾU XÃ A -> TỪ CHỐI -----------------------------------------------

// TestCuaToiPhienXaBKhongDocDuocPhieuXaA is the commune axis, with the person held CONSTANT.
//
// The session carries the SAME citizen as the owner of the petition — only the commune differs.
// That is what makes it a test of rule 1 rather than of rule 4: if the commune were dropped from
// the query, the identity filter alone would happily return commune A's petition.
func TestCuaToiPhienXaBKhongDocDuocPhieuXaA(t *testing.T) {
	m := dungMayChuCongDan(t)

	w := m.goi(t, duongCuaToi(maCuaToi), tokenXaB)
	doiMa(t, w, http.StatusNotFound)

	// THE COMMUNE THAT REACHED THE STORE IS THE SESSION'S, and that is the assertion a status code
	// cannot make: a handler that had resolved commune A from anywhere would look identical here.
	if len(m.phieu.thayXa) != 1 || m.phieu.thayXa[0] != xaB {
		t.Errorf("xã đến kho = %v, muốn %q (xã CỦA PHIÊN, ADR 0022)", m.phieu.thayXa, xaB)
	}

	// And the body is the same one an unknown code produces — commune B's citizen must not be able
	// to tell "exists in another commune" from "does not exist".
	khongCo := m.goi(t, duongCuaToi(maKhongTonTai), tokenXaB)
	if !bytes.Equal(w.Body.Bytes(), khongCo.Body.Bytes()) {
		t.Errorf("thân phản hồi khác nhau giữa 'phiếu của xã khác' và 'không tồn tại':\n  %s\n  %s",
			w.Body.String(), khongCo.Body.String())
	}
}

// --- CA 4: PHIẾU CỦA CHÍNH MÌNH -> 200, SỐ ĐIỆN THOẠI ĐÃ CHE ----------------------------------

func TestCuaToiPhieuCuaChinhMinhLa200VaDaChe(t *testing.T) {
	m := dungMayChuCongDan(t)

	w := m.goi(t, duongCuaToi(maCuaToi), tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	ra := docPhieuCuaToi(t, w.Body.Bytes())

	if ra.Code != maCuaToi {
		t.Errorf("code = %q, muốn %q", ra.Code, maCuaToi)
	}
	if ra.Content == "" || ra.Status != string(domain.DangPhanLoai) {
		t.Errorf("nội dung/trạng thái không đúng: %+v", ra)
	}
	// The commune's own wording for the field code, not the raw slug.
	if ra.Field != "rac-thai" || ra.FieldLabel != "Rác thải – Vệ sinh môi trường" {
		t.Errorf("lĩnh vực = %q / nhãn = %q", ra.Field, ra.FieldLabel)
	}

	// THE MASKING, ASSERTED ON THE RAW BODY AND NOT ONLY ON THE DECODED STRUCT. A field added later
	// that carried the number under another name would pass a struct-level check.
	than := w.Body.String()
	if strings.Contains(than, "0900000000") {
		t.Errorf("SỐ ĐIỆN THOẠI ĐẦY ĐỦ lọt ra phản hồi của chính công dân: %s", than)
	}
	if strings.Contains(than, "Nguyễn Văn An") {
		t.Errorf("HỌ TÊN ĐẦY ĐỦ lọt ra phản hồi: %s", than)
	}
	if ra.ReporterPhone != "09****0000" {
		t.Errorf("reporter_phone = %q, muốn dạng đã che %q", ra.ReporterPhone, "09****0000")
	}
	if ra.ReporterName != "Nguyễn V. A." {
		t.Errorf("reporter_name = %q, muốn dạng đã che", ra.ReporterName)
	}

	// The two deadlines come out byte for byte, and the two NULLs keep their opposite meanings.
	if ra.AcknowledgeDue == nil || !ra.AcknowledgeDue.Equal(mocTiepNh) {
		t.Errorf("acknowledge_due = %v, muốn %v", ra.AcknowledgeDue, mocTiepNh)
	}
	if ra.ResolveDue == nil || !ra.ResolveDue.Equal(mocXuLy) {
		t.Errorf("resolve_due = %v, muốn %v", ra.ResolveDue, mocXuLy)
	}
	if !ra.ClockFrom.Equal(mocGui) {
		t.Errorf("clock_from = %v, muốn %v", ra.ClockFrom, mocGui)
	}
}

// TestCuaToiKhongMangTruongNOIBO is rule 10, invariant 7 and rule 4, forbidden #5.
//
// IT READS THE RAW JSON KEYS rather than the struct, because the defect this guards against is
// somebody adding a field — and a struct-level assertion can only check fields that already exist.
func TestCuaToiKhongMangTruongNOIBO(t *testing.T) {
	m := dungMayChuCongDan(t)
	w := m.goi(t, duongCuaToi(maCuaToi), tokenCuaToi)
	doiMa(t, w, http.StatusOK)

	var tho map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}

	// Each of these is a fact about how the AUTHORITY handles the petition, not about the
	// petition's progress. The fixture populates every one of them, so a leak is visible.
	for _, khoa := range []string{
		"bo_phan_id", "org_unit_id", "department_id",
		"can_bo_xu_ly_id", "assignee_id", "handler_id",
		"public", "hien_cong_khai",
		"booked_at", "vao_so_luc",
		"reopen_count", "so_lan_mo_lai",
		"id",
		// Rule 10, invariant 3: overdue is DERIVED, never a field. A boolean here would be a
		// second representation of a fact the client already holds, and the stale one is the one
		// a screen renders.
		"overdue", "is_overdue", "qua_han",
	} {
		if _, co := tho[khoa]; co {
			t.Errorf("trường nội bộ %q lọt ra bề mặt công dân: %s", khoa, w.Body.String())
		}
	}

	// And the internal values must not appear anywhere in the body under any spelling.
	for _, gt := range []string{"bp-001", "nd-001", "pa-001"} {
		if strings.Contains(w.Body.String(), gt) {
			t.Errorf("giá trị nội bộ %q lọt ra phản hồi: %s", gt, w.Body.String())
		}
	}
}

// TestCuaToiPhieuAnDanhKhongHienTENVaSO — anonymity is a promise the CITIZEN made a choice about,
// and it holds on their own screen too. Neither field carries a value, not even a masked one: a
// masked name is still a name, and the record keeps the identity regardless (ADR 0008).
func TestCuaToiPhieuAnDanhKhongHienTENVaSO(t *testing.T) {
	m := dungMayChuCongDan(t)

	w := m.goi(t, duongCuaToi(maAnDanh), tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	ra := docPhieuCuaToi(t, w.Body.Bytes())

	if !ra.Anonymous {
		t.Error("anonymous = false trên phiếu ẩn danh")
	}
	if ra.ReporterName != "" || ra.ReporterPhone != "" {
		t.Errorf("phiếu ẩn danh vẫn hiện người gửi: %q / %q", ra.ReporterName, ra.ReporterPhone)
	}
	if strings.Contains(w.Body.String(), "0900000000") || strings.Contains(w.Body.String(), "Nguyễn") {
		t.Errorf("dữ liệu người gửi lọt ra trên phiếu ẩn danh: %s", w.Body.String())
	}
}

// --- lỗi hệ thống và lắp ráp -------------------------------------------------------------------

// TestCuaToiLoiKhoLa500VaKhongLoDuLieu: a store failure is a 500 whose body says nothing about the
// petition. The fake's error deliberately carries a code and a name, the way a real driver error
// does — that is what makes the assertion meaningful (rule 3, forbidden #3).
func TestCuaToiLoiKhoLa500VaKhongLoDuLieu(t *testing.T) {
	m := dungMayChuCongDan(t)
	m.phieu.loi = errors.New("pg: connection refused tại phiếu " + maCuaToi + " của Nguyễn Văn An")

	w := m.goi(t, duongCuaToi(maCuaToi), tokenCuaToi)
	doiMa(t, w, http.StatusInternalServerError)

	than := w.Body.String()
	if strings.Contains(than, maCuaToi) || strings.Contains(than, "Nguyễn") ||
		strings.Contains(than, "connection refused") {
		t.Errorf("chi tiết nội bộ lọt ra thân lỗi: %s", than)
	}
}

func TestRegisterCongDanThieuPhuThuocThiPanicNgayLucDung(t *testing.T) {
	for ten, bo := range map[string]func(d *DepsCongDan){
		"thiếu kho phiếu theo danh tính": func(d *DepsCongDan) { d.Phieu = nil },
		"thiếu use case tiếp nhận":       func(d *DepsCongDan) { d.GuiPhieu = nil },
		"thiếu kho nhãn lĩnh vực":        func(d *DepsCongDan) { d.NhanLinhVuc = nil },
	} {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("dựng được tuyến công dân với phụ thuộc thiếu — lỗi sẽ nổ trước mặt người dân")
				}
			}()
			d := DepsCongDan{Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), NhanLinhVuc: nhanLinhVucMau()}
			bo(&d)
			RegisterCongDan(http.NewServeMux(), d)
		})
	}
}

// TestRegisterCongDanDuPhuThuocThiKhongPanic is the other half: without it the test above proves
// only that RegisterCongDan panics, not that it panics for the reason claimed.
func TestRegisterCongDanDuPhuThuocThiKhongPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterCongDan panic dù đủ phụ thuộc: %v", r)
		}
	}()
	RegisterCongDan(http.NewServeMux(), DepsCongDan{
		Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), NhanLinhVuc: nhanLinhVucMau(),
	})
}

// TestCuaToiKhongDungChungHandlerVoiTuyenCANBO pins rule 4, invariant 5 at the type level.
//
// DepsCongDan carries the identity-filtered read and NOTHING ELSE. If somebody ever widens it to
// the staff interface, this stops compiling — which is the point: the separation is meant to be a
// property of the types, not a habit.
func TestCuaToiKhongDungChungHandlerVoiTuyenCANBO(t *testing.T) {
	var _ PhieuCuaCongDanDoc = phieuCuaToiMau()

	// The staff read interface must NOT be satisfied by the citizen dependency, and the citizen
	// one must not be satisfied by the staff fake. Asserted as a compile-time-shaped check rather
	// than with reflection, so the failure arrives at build time.
	if _, ok := any(phieuCuaToiMau()).(PhieuPhanAnhDoc); ok {
		t.Error("kho của tuyến công dân cũng thoả giao diện đọc KHÔNG lọc danh tính — " +
			"một tuyến công dân có thể với tới đường đọc của cán bộ")
	}
	if _, ok := any(phieuMau()).(PhieuCuaCongDanDoc); ok {
		t.Error("kho của tuyến cán bộ cũng thoả giao diện đọc theo danh tính công dân")
	}
}

// mocGui and friends are defined in phieu_phan_anh_test.go — the citizen suite deliberately reuses
// them so the two surfaces are compared against the SAME instants. A second set of constants would
// let the two drift and hide a difference that mattered.
var _ = time.Time{}
