package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the three routes of Cấu hình → Thời hạn xử lý. Rule 5, invariant 7 asks
// four cases of every one of them —
//
//	401  no token
//	403  a signed-in account holding the WRONG permission
//	403  the RIGHT permission, in the WRONG COMMUNE
//	2xx  both correct
//
// THE THIRD CASE ONLY PROVES ANYTHING BECAUSE checkerGia IS KEYED BY COMMUNE (routes_test.go). The
// harness's default checker grants `admin.user` and NOT `admin.sla`, so every case here rebuilds the
// chain with a checker granting `admin.sla` in commune A and nothing in commune B — which means the
// "wrong permission" case below is not a contrivance either: it is the shipped default.
//
// WHAT IS DELIBERATELY NOT HERE: the seeding and no-overwrite properties. They are decisions of the
// use case, proved against a real transaction in app/sla_test.go, and asserting them again through a
// fake use case would assert that the fake returns what it was told to. What IS asserted here is
// what the handler itself owns — the permission, the status, the wire shape, and which of a
// principal's two identifiers reaches the audit trail.

// --- the fakes ------------------------------------------------------------------------------------

// slaGia stands in for *idstore.SLAStore on the READ field.
type slaGia struct {
	mu  sync.Mutex
	goi int

	ds  []domain.DongSLA
	loi error
}

// slaMau is a commune with a configured table: one default row and one field row, with DIFFERENT
// figures in every one of the five positions so a transposition in the response mapping shows up.
func slaMau() *slaGia {
	return &slaGia{ds: []domain.DongSLA{
		{
			ID: "01JSLAMACDINH000000000000", LoaiViec: domain.LoaiViecPhanAnh, LinhVuc: "",
			GioTiepNhan: 8, GioXuLyXong: 56, GioSapDenHan: 24, GioBaoLanhDao: 25, GioBaoChuTich: 48,
		},
		{
			ID: idDongSLAThu, LoaiViec: domain.LoaiViecPhanAnh, LinhVuc: "an-ninh",
			GioTiepNhan: 2, GioXuLyXong: 16, GioSapDenHan: 4, GioBaoLanhDao: 8, GioBaoChuTich: 17,
		},
	}}
}

const idDongSLAThu = "01JSLAANNINH00000000000XX"

func (s *slaGia) DanhSach(ctx context.Context) ([]domain.DongSLA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.goi++
	_ = tenant.MustFrom(ctx) // the route must be reached with a commune in context, or this panics
	return s.ds, s.loi
}

func (s *slaGia) soLanGoi() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.goi
}

// ghiSLAGia stands in for *app.SLA on the WRITE field.
//
// IT RECORDS THE ACTOR, and that is the point of `nguoiCuoi`: the handler is the one layer that
// decides which of a principal's two identifiers reaches the audit trail, and a fake that dropped it
// would let `p.ID` be written there with nothing turning red.
type ghiSLAGia struct {
	mu sync.Mutex

	goi       int
	nguoiCuoi app.NguoiThucHien
	xaCuoi    tenant.ID
	idCuoi    string
	suaCuoi   app.YeuCauSuaSLA

	kq   domain.DongSLA
	gieo app.KetQuaGieo
	loi  error
}

func ghiSLAMau() *ghiSLAGia {
	return &ghiSLAGia{
		kq: domain.DongSLA{
			ID: idDongSLAThu, LoaiViec: domain.LoaiViecPhanAnh, LinhVuc: "an-ninh",
			GioTiepNhan: 2, GioXuLyXong: 12, GioSapDenHan: 4, GioBaoLanhDao: 8, GioBaoChuTich: 17,
		},
		gieo: app.KetQuaGieo{DaGieo: 15, DaCo: 0},
	}
}

func (g *ghiSLAGia) ghiNhan(ctx context.Context, nguoi app.NguoiThucHien) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goi++
	g.nguoiCuoi = nguoi
	g.xaCuoi = tenant.MustFrom(ctx)
}

func (g *ghiSLAGia) soLanGoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.goi
}

func (g *ghiSLAGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaSLA,
	nguoi app.NguoiThucHien) (domain.DongSLA, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.suaCuoi = id, yc
	return g.kq, g.loi
}

func (g *ghiSLAGia) GieoMacDinh(ctx context.Context, nguoi app.NguoiThucHien) (app.KetQuaGieo, error) {
	g.ghiNhan(ctx, nguoi)
	return g.gieo, g.loi
}

// --- harness --------------------------------------------------------------------------------------

// tuyenSLA is every route of this screen with a body that satisfies it. ONE TABLE, so a case written
// once cannot be added to one route and forgotten on another — which is exactly how a list route
// gets a permission check and its write route does not.
type tuyenSLA struct {
	ten    string
	method string
	duong  string
	than   string
	ok     int
}

func moiTuyenSLA() []tuyenSLA {
	return []tuyenSLA{
		{"đọc bảng thời hạn", "GET", "/api/v1/sla", "", http.StatusOK},
		{"sửa một dòng", "PATCH", "/api/v1/sla/" + idDongSLAThu, `{"resolve_hours":12}`, http.StatusOK},
		{"gieo mặc định", "POST", "/api/v1/sla/defaults", "", http.StatusOK},
	}
}

// dungMayChuSLA builds the chain with a checker that grants `admin.sla` in commune A and nothing in
// commune B. The harness default grants `admin.user`, which is what makes the "wrong permission"
// case below use the shipped default rather than an invented one.
func dungMayChuSLA(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.sla"): true}},
			xaB: {},
		}}
	})
	return m
}

func (m *mayChu) goiSLA(t *testing.T, tg tuyenSLA, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	return m.goi(t, tg.method, host, tg.duong, tg.than, tok)
}

// --- rule 5, invariant 7: four cases, every route --------------------------------------------------

func TestSLA_401KhongToken(t *testing.T) {
	m := dungMayChuSLA(t)

	for _, tg := range moiTuyenSLA() {
		w := m.goiSLA(t, tg, hostA, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.sla.soLanGoi() + m.ghiSLA.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà tầng dưới đã chạy %d lần", n)
	}
}

// 403 WITH THE WRONG PERMISSION. The account holds `admin.user` — a real key, and the one the rest
// of this service's administration routes use — which has nothing to do with configuring deadlines.
// That `admin.sla` is a SEPARATE key is ADR 0029's deliberate choice, and this is what holds it.
func TestSLA_403SaiQuyen(t *testing.T) {
	m := dungMayChu(t) // the shipped default: quyenThu = admin.user in commune A

	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range moiTuyenSLA() {
		w := m.goiSLA(t, tg, hostA, tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — `admin.user` KHÔNG được mở màn hình thời hạn xử lý "+
				"(ADR 0029: khoá khác, cố ý). Thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.sla.soLanGoi() + m.ghiSLA.soLanGoi(); n != 0 {
		t.Errorf("sai quyền mà tầng dưới đã chạy %d lần — phép kiểm quyền phải chặn TRƯỚC", n)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE — the case that catches a permission crossing a
// commune boundary (rule 5, invariant 3). The same person, holding `admin.sla` in commune A,
// properly signed in at commune B: nothing about the request is malformed, the grant simply does not
// exist there.
//
// IT MATTERS MORE ON THIS TABLE THAN ON MOST. A deadline row written across the boundary would make
// one commune's promise to its citizens be computed from another commune's policy.
func TestSLA_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChuSLA(t)

	tok := m.tokenCho(t, xaB, sidB)
	for _, tg := range moiTuyenSLA() {
		w := m.goiSLA(t, tg, hostB, tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.sla.soLanGoi() + m.ghiSLA.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà tầng dưới đã chạy %d lần", n)
	}
}

func TestSLA_2xxDuCaHai(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, tg := range moiTuyenSLA() {
		w := m.goiSLA(t, tg, hostA, tok)
		if w.Code != tg.ok {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", tg.ten, w.Code, tg.ok, w.Body.String())
		}
	}
	if m.ghiSLA.xaCuoi != xaA {
		t.Errorf("use case ghi được gọi với xã %q, muốn %q", m.ghiSLA.xaCuoi, xaA)
	}
}

// --- rule 6, invariant 8: which identifier reaches the trail ----------------------------------------

// THE HANDLER PASSES THE STAFF CODE AS THE AUDIT ACTOR AND THE INTERNAL ID AS THE DECIDER. This is
// the one layer that chooses between a principal's two identifiers, and the wrong choice is
// invisible: a ULID is a perfectly valid string in `audit_log.actor_id`.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestSLAChuTheVetLaMaCanBo(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, tg := range moiTuyenSLA()[1:] { // the two write routes
		m.ghiSLA.nguoiCuoi = app.NguoiThucHien{}
		if w := m.goiSLA(t, tg, hostA, tok); w.Code != tg.ok {
			t.Fatalf("%s: mã = %d — thân: %s", tg.ten, w.Code, w.Body.String())
		}
		n := m.ghiSLA.nguoiCuoi
		if n.Vet.ID != maCanBo {
			t.Errorf("%s: chủ thể vết = %q, muốn MÃ CÁN BỘ %q", tg.ten, n.Vet.ID, maCanBo)
		}
		if n.Vet.ID == idNoiBo {
			t.Errorf("%s: chủ thể vết đang là ID nội bộ — luật 6 bất biến 8", tg.ten)
		}
		if n.ID != idNoiBo {
			t.Errorf("%s: định danh quyết định = %q, muốn id nội bộ %q", tg.ten, n.ID, idNoiBo)
		}
	}
}

// --- the read: shape, problems, and the empty table -------------------------------------------------

// THE FIVE FIGURES REACH THE WIRE IN THE RIGHT FIELDS. The fixture gives all five DIFFERENT values
// for exactly this reason: a transposition in dongSLARaNgoai produces no error anywhere, only a
// different promise, and there is no constraint downstream that could tell the five apart.
//
// MUTATION THAT MUST TURN THIS RED: swap AcknowledgeHours and DueSoonHours in dongSLARaNgoai.
func TestDocSLATraDungNamConSoVaCoDongMacDinh(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goi(t, "GET", hostA, "/api/v1/sla", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}

	var ra struct {
		Items []struct {
			ID        string `json:"id"`
			WorkKind  string `json:"work_kind"`
			Field     string `json:"field"`
			IsDefault bool   `json:"is_default"`
			Ack       int    `json:"acknowledge_hours"`
			Resolve   int    `json:"resolve_hours"`
			DueSoon   int    `json:"due_soon_hours"`
			Leader    int    `json:"escalate_leader_hours"`
			President int    `json:"escalate_president_hours"`
		} `json:"items"`
		Problems []struct {
			Kind string `json:"kind"`
		} `json:"problems"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không đọc được: %v — %s", err, w.Body.String())
	}
	if len(ra.Items) != 2 {
		t.Fatalf("có %d dòng, muốn 2", len(ra.Items))
	}

	mac := ra.Items[0]
	if !mac.IsDefault || mac.Field != "" {
		t.Errorf("dòng mặc định: is_default=%v field=%q, muốn true và \"\"", mac.IsDefault, mac.Field)
	}
	an := ra.Items[1]
	if an.IsDefault || an.Field != "an-ninh" || an.WorkKind != "phan-anh" {
		t.Errorf("dòng lĩnh vực: %+v", an)
	}
	// 2 · 16 · 4 · 8 · 17 — five distinct values, in the order the fixture set them.
	if an.Ack != 2 || an.Resolve != 16 || an.DueSoon != 4 || an.Leader != 8 || an.President != 17 {
		t.Errorf("năm con số = %d/%d/%d/%d/%d, muốn 2/16/4/8/17 "+
			"(tiếp nhận · xử lý xong · sắp đến hạn · báo lãnh đạo · báo chủ tịch)",
			an.Ack, an.Resolve, an.DueSoon, an.Leader, an.President)
	}
	if len(ra.Problems) != 0 {
		t.Errorf("bảng đủ dòng mặc định mà vẫn báo %d vấn đề: %+v", len(ra.Problems), ra.Problems)
	}
}

// AN EMPTY TABLE IS 200 WITH A NAMED PROBLEM, NOT AN ERROR — and this is the case that would be
// easiest to "improve" into a 409 by somebody reasoning that an unconfigured commune is broken. It
// is: but THE SCREEN THAT FIXES IT READS THROUGH THIS ROUTE, so refusing the read locks away the
// only repair. The refusal belongs where a DEADLINE IS COMPUTED — grpc.ResolveDeadlines answers
// FAILED_PRECONDITION and `service-documents` turns that into 409, and neither is softened by this.
//
// TODAY THIS IS EVERY COMMUNE: migration 0008 seeds nothing.
func TestDocSLABangRongVan200VaNeuTenVanDe(t *testing.T) {
	m := dungMayChuSLA(t)
	m.sla.ds = nil

	w := m.goi(t, "GET", hostA, "/api/v1/sla", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d, muốn 200 — màn hình sửa cấu hình phải nạp được. Thân: %s", w.Code, w.Body.String())
	}

	var ra struct {
		Items    []json.RawMessage `json:"items"`
		Problems []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"problems"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không đọc được: %v", err)
	}
	if ra.Items == nil {
		t.Error("`items` marshal thành null — phải là [] (client nào phải xử lý cả hai sẽ xử lý sai một)")
	}
	if len(ra.Problems) != 1 || ra.Problems[0].Kind != string(domain.VanDeSLATrong) {
		t.Fatalf("vấn đề = %+v, muốn đúng một `%s`", ra.Problems, domain.VanDeSLATrong)
	}
	if ra.Problems[0].Message == "" {
		t.Error("vấn đề không có câu tiếng Việt nào cho người đọc màn hình")
	}
}

// THE CEILING IS REFUSED, NOT TRUNCATED. A dropped row makes DongTheoLinhVuc fall back to the
// default and quietly answer with a different promise than the commune made.
func TestDocSLAVuotTranTraLoi500ChuKhongCatBot(t *testing.T) {
	m := dungMayChuSLA(t)
	m.sla.loi = idstore.ErrQuaNhieuDongSLA

	w := m.goi(t, "GET", hostA, "/api/v1/sla", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("mã = %d, muốn 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "tran") || strings.Contains(w.Body.String(), "sla:") {
		t.Errorf("chi tiết nội bộ lọt ra ngoài (luật 3 cấm #3): %s", w.Body.String())
	}
}

// --- the edit ---------------------------------------------------------------------------------------

// THE FIVE JSON NAMES MAP ONTO THE FIVE REQUEST FIELDS, and an absent one stays nil. A mapping
// defect here is invisible: the edit succeeds and the wrong figure moves.
func TestSuaSLAAnhXaDungNamTruongVaBoQuaTruongVang(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)

	than := `{"acknowledge_hours":3,"resolve_hours":12,"due_soon_hours":5,` +
		`"escalate_leader_hours":9,"escalate_president_hours":18}`
	if w := m.goi(t, "PATCH", hostA, "/api/v1/sla/"+idDongSLAThu, than, tok); w.Code != http.StatusOK {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	yc := m.ghiSLA.suaCuoi
	for ten, cap := range map[string][2]any{
		"acknowledge_hours":        {yc.GioTiepNhan, 3},
		"resolve_hours":            {yc.GioXuLyXong, 12},
		"due_soon_hours":           {yc.GioSapDenHan, 5},
		"escalate_leader_hours":    {yc.GioBaoLanhDao, 9},
		"escalate_president_hours": {yc.GioBaoChuTich, 18},
	} {
		p, _ := cap[0].(*int)
		if p == nil {
			t.Errorf("%s: không tới được use case", ten)
			continue
		}
		if *p != cap[1].(int) {
			t.Errorf("%s = %d, muốn %d", ten, *p, cap[1].(int))
		}
	}
	if m.ghiSLA.idCuoi != idDongSLAThu {
		t.Errorf("id = %q, muốn %q", m.ghiSLA.idCuoi, idDongSLAThu)
	}

	// Only one figure mentioned: the other four stay nil, which is what "leave this alone" is.
	if w := m.goi(t, "PATCH", hostA, "/api/v1/sla/"+idDongSLAThu, `{"resolve_hours":20}`, tok); w.Code != http.StatusOK {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	yc = m.ghiSLA.suaCuoi
	if yc.GioXuLyXong == nil || *yc.GioXuLyXong != 20 {
		t.Errorf("resolve_hours không tới nơi: %v", yc.GioXuLyXong)
	}
	if yc.GioTiepNhan != nil || yc.GioSapDenHan != nil || yc.GioBaoLanhDao != nil || yc.GioBaoChuTich != nil {
		t.Error("trường không gửi lên lại tới use case khác nil — màn hình sửa một ô sẽ ghi đè bốn ô kia")
	}
}

// AN EMPTY BODY IS 400, NOT A SILENT 200. `{}` means the client sent a form it failed to read, and
// answering 200 tells the person their edit was saved.
func TestSuaSLAThanRongTraLoi400VaKhongGoiUseCase(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, than := range []string{`{}`, `{"unknown_field":5}`} {
		truoc := m.ghiSLA.soLanGoi()
		w := m.goi(t, "PATCH", hostA, "/api/v1/sla/"+idDongSLAThu, than, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("thân %s: mã = %d, muốn 400", than, w.Code)
		}
		if m.ghiSLA.soLanGoi() != truoc {
			t.Errorf("thân %s: use case vẫn chạy", than)
		}
	}
}

// A ROW THAT IS NOT THIS COMMUNE'S IS 404, NOT 403. Every statement is scoped, so an invented id, a
// soft-deleted row and another authority's row genuinely produce one answer, and none can be told
// apart by trying (rule 4, forbidden #2 applied to configuration).
func TestSuaSLAKhongTimThayTraLoi404(t *testing.T) {
	m := dungMayChuSLA(t)
	m.ghiSLA.loi = idstore.ErrDongSLAKhongTonTai

	w := m.goi(t, "PATCH", hostA, "/api/v1/sla/01JKHONGCO000000000000000",
		`{"resolve_hours":12}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusNotFound {
		t.Fatalf("mã = %d, muốn 404 — thân: %s", w.Code, w.Body.String())
	}
}

// AN INVALID FIGURE IS 400 AND THE DOMAIN'S OWN SENTENCE. 500 here would send an operator hunting a
// broken server for what is a typed number out of range.
func TestSuaSLASoGioSaiTraLoi400(t *testing.T) {
	m := dungMayChuSLA(t)
	m.ghiSLA.loi = domain.ErrGioPhaiDuong

	w := m.goi(t, "PATCH", hostA, "/api/v1/sla/"+idDongSLAThu,
		`{"resolve_hours":0}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mã = %d, muốn 400 — thân: %s", w.Code, w.Body.String())
	}
}

// --- the seeding route -------------------------------------------------------------------------------

// THE TWO COUNTS REACH THE WIRE, AND THE SECOND RUN'S SHAPE IS THE ONE THAT MATTERS: `seeded: 0,
// kept: 15` is a SUCCESS, not a failure. A 409 or a 500 here would push a client to retry and would
// make the screen show an error for a button that did exactly what it should.
func TestGieoSLATraSoDongDaGieoVaDaGiuNguyen(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)

	doc := func(t *testing.T, than string) (int, int) {
		t.Helper()
		var ra struct {
			Seeded int `json:"seeded"`
			Kept   int `json:"kept"`
		}
		if err := json.Unmarshal([]byte(than), &ra); err != nil {
			t.Fatalf("thân không đọc được: %v — %s", err, than)
		}
		return ra.Seeded, ra.Kept
	}

	// First run.
	w := m.goi(t, "POST", hostA, "/api/v1/sla/defaults", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("lần gieo đầu: mã = %d — thân: %s", w.Code, w.Body.String())
	}
	if s, k := doc(t, w.Body.String()); s != 15 || k != 0 {
		t.Errorf("lần đầu: seeded=%d kept=%d, muốn 15/0", s, k)
	}

	// Second run, on a commune that now has everything.
	m.ghiSLA.gieo = app.KetQuaGieo{DaGieo: 0, DaCo: 15}
	w = m.goi(t, "POST", hostA, "/api/v1/sla/defaults", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("lần gieo thứ hai: mã = %d, muốn 200 — bấm lại nút không phải một lỗi. Thân: %s",
			w.Code, w.Body.String())
	}
	if s, k := doc(t, w.Body.String()); s != 0 || k != 15 {
		t.Errorf("lần hai: seeded=%d kept=%d, muốn 0/15", s, k)
	}
}

// TWO ADMINISTRATORS PRESSING AT THE SAME INSTANT IS 409, NOT 500. Nothing is wrong and nothing was
// half-written: the winner's rows are there, and retrying answers `seeded: 0`.
func TestGieoSLATrungKhoaTraLoi409(t *testing.T) {
	m := dungMayChuSLA(t)
	m.ghiSLA.loi = idstore.ErrDongSLATrungKhoa

	w := m.goi(t, "POST", hostA, "/api/v1/sla/defaults", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — thân: %s", w.Code, w.Body.String())
	}
}
