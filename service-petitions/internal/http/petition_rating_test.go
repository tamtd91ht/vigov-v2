package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

// POST /api/v1/my-citizen-reports/{maTraCuu}/rating, over the REAL citizen chain (CitizenEdge ->
// CitizenPrincipal -> XaTuPhien -> idem), with the use case faked.
//
//	PROVED HERE   401 with no usable session, use case not reached · another citizen's code and another
//	              commune's code answer BYTE FOR BYTE what the GET answers for an unknown code · 409 on a
//	              status that cannot be rated · 400 on stars out of range and an over-long comment, with
//	              fixed sentences that echo nothing · 200 in the GET's shape, with the rating on it · the
//	              citizen and the commune reach the use case FROM THE SESSION, whatever the body says ·
//	              the rating shows on the citizen's GET and list card · the staff response carries
//	              rating, comment, rated_at and reopen_count · `rating_max` is validated and reaches the
//	              store as a bound filter.
//
//	NOT PROVED    the transaction, the reopen statement, the audit entry and the outbox row — those are
//	              internal/app/petition_rating_test.go, over the real store.

// ratingFake is the use case. IT APPLIES THE OWNERSHIP AND STATE RULES ITSELF, keyed by commune, so the
// isolation cases fail if the handler hands down the wrong identity or commune — a fake that answered
// everyone would make them pass while proving nothing. The real rules are proved in internal/app.
type ratingFake struct {
	petitions map[tenant.ID]map[string]domain.PhieuPhanAnh

	calls       int
	seenCitizen []audit.Actor
	seenTenant  []tenant.ID
	seenReq     []app.RatingRequest
	err         error
}

func newRatingFake() *ratingFake {
	return &ratingFake{petitions: map[tenant.ID]map[string]domain.PhieuPhanAnh{}}
}

func (f *ratingFake) Rate(ctx context.Context, ma string, req app.RatingRequest, citizen audit.Actor) (
	domain.PhieuPhanAnh, error) {

	f.calls++
	f.seenCitizen = append(f.seenCitizen, citizen)
	f.seenTenant = append(f.seenTenant, tenant.MustFrom(ctx))
	f.seenReq = append(f.seenReq, req)
	if f.err != nil {
		return domain.PhieuPhanAnh{}, f.err
	}
	stars, comment, err := domain.CheckRating(req.Stars, req.Comment)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	p, ok := f.petitions[tenant.MustFrom(ctx)][ma]
	if !ok || p.CongDanID != citizen.ID {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	if !domain.RatingOpen(p.TrangThai) {
		return domain.PhieuPhanAnh{}, domain.ErrRatingNotOpen
	}
	p.Rating, p.RatingComment, p.RatedAt = stars, comment, ratedAtHTTP
	if domain.RatingReopens(stars) {
		p.TrangThai, p.SoLanMoLai = domain.DangXuLy, p.SoLanMoLai+1
	}
	return p, nil
}

var ratedAtHTTP = time.Date(2026, 9, 28, 2, 17, 9, 0, time.UTC)

const (
	codeRateMine    = "PA-5RTK-8MWX-2HQD"
	codeRateOther   = "PA-6BNV-3CXZ-9KPT"
	codeRateWorking = "PA-7DFG-4JKL-2MNP"
	idemKeyRating   = "01JKHOACHAMSAOCONGDANTHU1"
)

func ratingFixture() *ratingFake {
	f := newRatingFake()
	f.petitions[xaA] = map[string]domain.PhieuPhanAnh{
		codeRateMine: {ID: "pa-r1", MaTraCuu: codeRateMine, Kenh: domain.KenhZaloMiniApp, CongDanID: idToi,
			NoiDung: "Đống rác ở đầu ngõ.", LinhVuc: "rac-thai", TrangThai: domain.ChoDanXacNhan,
			GocDemHan: mocGui, VaoSoLuc: mocVaoSo, HanTiepNhan: mocTiepNh, HanXuLyXong: mocXuLy,
			SoLanMoLai: 1, BoPhanID: "bp-001", CanBoXuLyID: "CB-00123"},
		codeRateOther: {ID: "pa-r2", MaTraCuu: codeRateOther, Kenh: domain.KenhZaloMiniApp,
			CongDanID: idNguoiKhac, TrangThai: domain.ChoDanXacNhan, GocDemHan: mocGui},
		codeRateWorking: {ID: "pa-r3", MaTraCuu: codeRateWorking, Kenh: domain.KenhZaloMiniApp,
			CongDanID: idToi, TrangThai: domain.DangXuLy, GocDemHan: mocGui},
	}
	f.petitions[xaB] = map[string]domain.PhieuPhanAnh{}
	return f
}

type ratingServer struct {
	h      http.Handler
	rating *ratingFake
}

func buildRatingServer(t *testing.T) *ratingServer {
	t.Helper()
	f := ratingFixture()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), Rating: f, NhanLinhVuc: nhanLinhVucMau(), Log: log,
	})
	// THE REAL CHAIN, in the order cmd/server builds it — idem innermost, as the intake suite does.
	var h http.Handler = mux
	h = idem.Middleware(khoIdemMoi(), log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(&soPhienCongDanGia{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &ratingServer{h: h, rating: f}
}

func ratingPath(code string) string { return "/api/v1/my-citizen-reports/" + code + "/rating" }

func (s *ratingServer) post(t *testing.T, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://"+hostMiniApp+path, strings.NewReader(body))
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, idemKeyRating)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func (s *ratingServer) get(t *testing.T, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+hostMiniApp+path, nil)
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

// --- 401 ------------------------------------------------------------------------------------------

func TestRatingNoSessionIs401AndUseCaseNotReached(t *testing.T) {
	for name, token := range map[string]string{
		"không có header Authorization": "",
		"token sổ phiên không nhận":     "token-khong-ai-cap-BAO-GIO",
	} {
		t.Run(name, func(t *testing.T) {
			s := buildRatingServer(t)
			w := s.post(t, ratingPath(codeRateMine), `{"stars":1}`, token)
			doiMa(t, w, http.StatusUnauthorized)
			if s.rating.calls != 0 {
				t.Errorf("use case bị gọi %d lần dù không có phiên", s.rating.calls)
			}
		})
	}
}

// --- 404: another citizen, another commune — identical to the GET's unknown code ------------------

func TestRatingOtherCitizenAndOtherCommuneAreTheSame404AsTheGet(t *testing.T) {
	s := buildRatingServer(t)
	unknown := s.get(t, duongCuaToi(maKhongTonTai), tokenCuaToi)
	doiMa(t, unknown, http.StatusNotFound)

	for name, c := range map[string]struct{ code, token string }{
		"mã của người khác, cùng xã":   {codeRateOther, tokenCuaToi},
		"mã của mình, phiên ở xã khác": {codeRateMine, tokenXaB},
		"mã không tồn tại":             {"PA-0000-0000-0000", tokenCuaToi},
	} {
		t.Run(name, func(t *testing.T) {
			w := s.post(t, ratingPath(c.code), `{"stars":1}`, c.token)
			doiMa(t, w, http.StatusNotFound)
			if w.Body.String() != unknown.Body.String() {
				t.Errorf("thân 404 khác GET mã không tồn tại (luật 4 cấm #2):\n  POST: %s\n  GET:  %s",
					w.Body.String(), unknown.Body.String())
			}
		})
	}
	// The commune of the session is what the use case saw — B for the B session, never A.
	var sawB bool
	for _, x := range s.rating.seenTenant {
		sawB = sawB || x == xaB
	}
	if !sawB {
		t.Errorf("phiên xã B không tới use case với xã B: %v", s.rating.seenTenant)
	}
}

// --- identity comes from the session --------------------------------------------------------------

func TestRatingIdentityFromSessionNotBody(t *testing.T) {
	s := buildRatingServer(t)
	// A body naming ANOTHER citizen and a status. Neither is a field of the request type.
	w := s.post(t, ratingPath(codeRateMine),
		`{"stars":4,"citizen_id":"`+idNguoiKhac+`","status":"da-dong","tenant":"xa-khac"}`, tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	if len(s.rating.seenCitizen) != 1 {
		t.Fatalf("use case được gọi %d lần", len(s.rating.seenCitizen))
	}
	who := s.rating.seenCitizen[0]
	if who.ID != idToi || who.Kind != "citizen" || who.IP == "" {
		t.Errorf("chủ thể = %+v, muốn công dân của phiên với IP", who)
	}
	if s.rating.seenTenant[0] != xaA {
		t.Errorf("xã = %v, muốn xã của phiên", s.rating.seenTenant[0])
	}
}

// --- 409 ------------------------------------------------------------------------------------------

func TestRatingWrongStatusIs409WithFixedSentence(t *testing.T) {
	s := buildRatingServer(t)
	w := s.post(t, ratingPath(codeRateWorking), `{"stars":2}`, tokenCuaToi)
	doiMa(t, w, http.StatusConflict)
	e := loiTra(t, w)
	if e.Code != "petition_state" {
		t.Errorf("mã lỗi = %q, muốn petition_state", e.Code)
	}
	if strings.Contains(w.Body.String(), "phan_anh:") || strings.Contains(w.Body.String(), string(xaA)) {
		t.Errorf("câu từ chối lộ tiền tố gói hoặc mã xã: %s", w.Body.String())
	}
}

// --- 400 ------------------------------------------------------------------------------------------

func TestRatingBadInputIs400(t *testing.T) {
	long := strings.Repeat("ồ", domain.RatingCommentMaxLen+1)
	for name, body := range map[string]string{
		"thiếu sao":         `{}`,
		"0 sao":             `{"stars":0}`,
		"6 sao":             `{"stars":6}`,
		"nhận xét quá dài":  `{"stars":4,"comment":"` + long + `"}`,
		"không phải JSON":   `{"stars":`,
		"sao không phải số": `{"stars":"năm"}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := buildRatingServer(t)
			w := s.post(t, ratingPath(codeRateMine), body, tokenCuaToi)
			doiMa(t, w, http.StatusBadRequest)
			if strings.Contains(w.Body.String(), "ồồồ") || strings.Contains(w.Body.String(), "phan_anh:") {
				t.Errorf("thân lỗi nhắc lại dữ liệu vào hoặc lộ tiền tố gói: %s", w.Body.String())
			}
		})
	}
}

// --- 200 ------------------------------------------------------------------------------------------

func TestRatingHighKeepsStatusInTheGetShape(t *testing.T) {
	s := buildRatingServer(t)
	w := s.post(t, ratingPath(codeRateMine), `{"stars":5,"comment":"Xã dọn nhanh."}`, tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.Status != string(domain.ChoDanXacNhan) || ra.Rating == nil || *ra.Rating != 5 ||
		ra.RatedAt == nil || !ra.RatedAt.Equal(ratedAtHTTP) {
		t.Errorf("200 sai: trạng thái=%s sao=%v lúc=%v", ra.Status, ra.Rating, ra.RatedAt)
	}
	if ra.FieldLabel != "Rác thải – Vệ sinh môi trường" {
		t.Errorf("nhãn lĩnh vực = %q", ra.FieldLabel)
	}
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	for _, k := range []string{"reopen_count", "so_lan_mo_lai", "rating_comment", "bo_phan_id", "assignee"} {
		if _, co := raw[k]; co {
			t.Errorf("trường %q lọt ra bề mặt công dân", k)
		}
	}
	if s.rating.seenReq[0].Comment != "Xã dọn nhanh." {
		t.Errorf("nhận xét không tới use case: %q", s.rating.seenReq[0].Comment)
	}
}

func TestRatingLowReturnsTheReopenedPetition(t *testing.T) {
	s := buildRatingServer(t)
	w := s.post(t, ratingPath(codeRateMine), `{"stars":1}`, tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.Status != string(domain.DangXuLy) || ra.Rating == nil || *ra.Rating != 1 {
		t.Errorf("mở lại: trạng thái=%s sao=%v", ra.Status, ra.Rating)
	}
	// The deadline the citizen was promised is on the response unchanged.
	if ra.ResolveDue == nil || !ra.ResolveDue.Equal(mocXuLy) {
		t.Errorf("resolve_due = %v, muốn %v không đổi", ra.ResolveDue, mocXuLy)
	}
}

// --- the citizen's own reads carry the rating -----------------------------------------------------

func TestCitizenResponsesCarryRatingOnlyWhenRated(t *testing.T) {
	rated := phieuCuaToiRaNgoai(domain.PhieuPhanAnh{MaTraCuu: "PA-X", TrangThai: domain.ChoDanXacNhan,
		Rating: 3, RatingComment: "ổn", RatedAt: ratedAtHTTP, SoLanMoLai: 2}, "")
	if rated.Rating == nil || *rated.Rating != 3 || rated.RatedAt == nil {
		t.Errorf("phiếu đã chấm không mang đánh giá: %+v", rated)
	}
	card := tomTatPhieuCuaToi(rated)
	if card.Rating == nil || *card.Rating != 3 || card.RatedAt == nil {
		t.Errorf("thẻ danh sách không mang đánh giá: %+v", card)
	}

	unrated := phieuCuaToiRaNgoai(domain.PhieuPhanAnh{MaTraCuu: "PA-Y", TrangThai: domain.DaXuLy}, "")
	b, _ := json.Marshal(unrated)
	if strings.Contains(string(b), `"rating"`) || strings.Contains(string(b), `"rated_at"`) {
		t.Errorf("phiếu chưa chấm lại mang khoá đánh giá: %s", b)
	}
}

// --- the staff response --------------------------------------------------------------------------

func TestStaffResponseCarriesRatingAndReopenCount(t *testing.T) {
	p := domain.PhieuPhanAnh{MaTraCuu: "PA-S", TrangThai: domain.DangXuLy, CongDanID: "cd-1",
		NoiDung: "Đống rác.", AnDanh: true, NguoiGuiHoTen: "Nguyễn Văn An", NguoiGuiDienThoai: "0900000000",
		Rating: 2, RatingComment: "Rác vẫn còn.", RatedAt: ratedAtHTTP, SoLanMoLai: 3}

	ra := phieuRaNgoai(p, "", false)
	if ra.Rating == nil || *ra.Rating != 2 || ra.RatedAt == nil || !ra.RatedAt.Equal(ratedAtHTTP) {
		t.Errorf("rating/rated_at = %v/%v", ra.Rating, ra.RatedAt)
	}
	// Same rule as `content`: shown, unmasked, on an ANONYMOUS petition too. Anonymity still hides the
	// reporter.
	if ra.RatingComment != "Rác vẫn còn." || ra.Content != "Đống rác." {
		t.Errorf("rating_comment = %q, content = %q", ra.RatingComment, ra.Content)
	}
	if ra.ReporterName != "" || ra.ReporterPhone != "" {
		t.Error("phiếu ẩn danh hiện người gửi")
	}
	if ra.ReopenCount == nil || *ra.ReopenCount != 3 {
		t.Errorf("reopen_count = %v, muốn 3", ra.ReopenCount)
	}

	// Never reopened: `0` travels as 0 — a meaningful answer, not an absent key. Unrated: no rating keys.
	fresh := phieuRaNgoai(domain.PhieuPhanAnh{MaTraCuu: "PA-T", TrangThai: domain.DaTiepNhan}, "", false)
	b, _ := json.Marshal(fresh)
	if !strings.Contains(string(b), `"reopen_count":0`) {
		t.Errorf("reopen_count 0 không đi trên dây: %s", b)
	}
	for _, k := range []string{`"rating"`, `"rating_comment"`, `"rated_at"`} {
		if strings.Contains(string(b), k) {
			t.Errorf("phiếu chưa chấm mang %s: %s", k, b)
		}
	}
}

// --- the low-rating filter on the staff list ------------------------------------------------------

func TestListRatingMaxReachesTheStore(t *testing.T) {
	for q, want := range map[string]int{"?rating_max=2": 2, "?rating_max=5": 5, "?rating_max=1": 1, "": 0} {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("feedback.read"))
		w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+q, canBoCuaXa(xaA), nil)
		doiMa(t, w, http.StatusOK)
		if m.danhSach.loc.RatingMax != want {
			t.Errorf("%q: RatingMax = %d, muốn %d", q, m.danhSach.loc.RatingMax, want)
		}
		// The restricted-field decision is still made alongside it (closed without the key).
		if m.danhSach.loc.ChoPhepHanChe {
			t.Errorf("%q: lọc đánh giá mở lĩnh vực hạn chế", q)
		}
	}
}

func TestListRatingMaxInvalidIs400(t *testing.T) {
	for _, q := range []string{"?rating_max=0", "?rating_max=6", "?rating_max=%2B2", "?rating_max=",
		"?rating_max=22", "?rating_max=hai", "?rating_max=%202"} {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("feedback.read"))
		w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+q, canBoCuaXa(xaA), nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q: mã = %d, muốn 400", q, w.Code)
		}
		if m.danhSach.goi != 0 {
			t.Errorf("%q: đã chạy truy vấn dù bộ lọc bị từ chối", q)
		}
	}
}
