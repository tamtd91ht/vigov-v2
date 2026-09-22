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

// WHAT THIS FILE IS FOR: the ELEVEN write routes of the commune's working calendar. Rule 5,
// invariant 7 asks four cases of every one of them —
//
//	401  no token
//	403  a signed-in account holding the WRONG permission
//	403  the RIGHT permission, in the WRONG COMMUNE
//	2xx  both correct
//
// THE THIRD CASE ONLY PROVES ANYTHING BECAUSE checkerGia IS KEYED BY COMMUNE (routes_test.go). The
// harness's default checker grants `admin.user` and NOT `admin.sla`, so the "wrong permission" case
// is not a contrivance either: it is the shipped default.
//
// THE ASYMMETRY WITH THE THREE READ ROUTES IS ASSERTED, NOT ASSUMED. GET /api/v1/working-hours is
// AnyAuthenticated — office hours sit under every deadline printed on a screen — while every WRITE
// here demands `admin.sla`. A turn that "tidied" the writes to AnyAuthenticated would turn the whole
// of TestLichGhi_403SaiQuyen red.
//
// WHAT IS DELIBERATELY NOT HERE: the seeding, no-overwrite, overlap and cross-table properties. They
// are decisions of the use case, proved against a REAL transaction in app/lich_lam_viec_test.go, and
// asserting them again through a fake use case would assert that the fake returns what it was told
// to. What IS asserted here is what the handler itself owns — the permission, the status, the wire
// shape, the parsing of a time of day, and which of a principal's two identifiers reaches the trail.

// --- the fake -------------------------------------------------------------------------------------

// ghiLichGia stands in for *app.Lich on all three write fields.
//
// IT RECORDS THE ACTOR, and that is the point of `nguoiCuoi`: the handler is the one layer that
// decides which of a principal's two identifiers reaches the audit trail, and a fake that dropped it
// would let `p.ID` be written into `audit_log.actor_id` with nothing turning red.
type ghiLichGia struct {
	mu sync.Mutex

	goi       int
	nguoiCuoi app.NguoiThucHien
	xaCuoi    tenant.ID
	idCuoi    string
	lyDoCuoi  string
	namCuoi   int

	themCa     app.YeuCauThemCa
	suaCa      app.YeuCauSuaCa
	themNghi   app.YeuCauThemNgayNghi
	suaNghi    app.YeuCauSuaNgayNghi
	themLamBu  app.YeuCauThemLamBu
	suaLamBuYC app.YeuCauSuaLamBu

	ca    domain.CaLamViec
	nghi  domain.NgayNghiLe
	lamBu domain.CaLamBu
	gieo  app.KetQuaGieoLich
	loi   error
}

func ghiLichMau() *ghiLichGia {
	return &ghiLichGia{
		ca: domain.CaLamViec{
			ID: idCaLamViecThu, Thu: 1, BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60,
			GhiChu: "Buổi sáng",
		},
		nghi:  domain.NgayNghiLe{ID: idNgayNghiLeThu, Ngay: "2026-09-02", Ten: "Quốc khánh"},
		lamBu: domain.CaLamBu{ID: idNgayLamBuThu, Ngay: "2026-02-21", BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60, Ten: "Làm bù nghỉ Tết"},
		gieo:  app.KetQuaGieoLich{DaGieo: 10, DaCo: 0, BoQua: 0},
	}
}

const (
	idCaLamViecThu  = "01JCALAMVIECTRENTUYEN0000"
	idNgayNghiLeThu = "01JNGAYNGHILETRENTUYEN000"
	idNgayLamBuThu  = "01JNGAYLAMBUTRENTUYEN0000"
)

func (g *ghiLichGia) ghiNhan(ctx context.Context, nguoi app.NguoiThucHien) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goi++
	g.nguoiCuoi = nguoi
	g.xaCuoi = tenant.MustFrom(ctx)
}

func (g *ghiLichGia) soLanGoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.goi
}

func (g *ghiLichGia) ThemCa(ctx context.Context, yc app.YeuCauThemCa, nguoi app.NguoiThucHien) (domain.CaLamViec, error) {
	g.ghiNhan(ctx, nguoi)
	g.themCa = yc
	return g.ca, g.loi
}

func (g *ghiLichGia) SuaCa(ctx context.Context, id string, yc app.YeuCauSuaCa, nguoi app.NguoiThucHien) (domain.CaLamViec, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.suaCa = id, yc
	return g.ca, g.loi
}

func (g *ghiLichGia) XoaCa(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.lyDoCuoi = id, lyDo
	return g.loi
}

func (g *ghiLichGia) GieoTuanMacDinh(ctx context.Context, nguoi app.NguoiThucHien) (app.KetQuaGieoLich, error) {
	g.ghiNhan(ctx, nguoi)
	return g.gieo, g.loi
}

func (g *ghiLichGia) ThemNgayNghi(ctx context.Context, yc app.YeuCauThemNgayNghi, nguoi app.NguoiThucHien) (domain.NgayNghiLe, error) {
	g.ghiNhan(ctx, nguoi)
	g.themNghi = yc
	return g.nghi, g.loi
}

func (g *ghiLichGia) SuaNgayNghi(ctx context.Context, id string, yc app.YeuCauSuaNgayNghi, nguoi app.NguoiThucHien) (domain.NgayNghiLe, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.suaNghi = id, yc
	return g.nghi, g.loi
}

func (g *ghiLichGia) XoaNgayNghi(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.lyDoCuoi = id, lyDo
	return g.loi
}

func (g *ghiLichGia) GieoNgayNghiLeMacDinh(ctx context.Context, nam int, nguoi app.NguoiThucHien) (app.KetQuaGieoLich, error) {
	g.ghiNhan(ctx, nguoi)
	g.namCuoi = nam
	return g.gieo, g.loi
}

func (g *ghiLichGia) ThemLamBu(ctx context.Context, yc app.YeuCauThemLamBu, nguoi app.NguoiThucHien) (domain.CaLamBu, error) {
	g.ghiNhan(ctx, nguoi)
	g.themLamBu = yc
	return g.lamBu, g.loi
}

func (g *ghiLichGia) SuaLamBu(ctx context.Context, id string, yc app.YeuCauSuaLamBu, nguoi app.NguoiThucHien) (domain.CaLamBu, error) {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.suaLamBuYC = id, yc
	return g.lamBu, g.loi
}

func (g *ghiLichGia) XoaLamBu(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error {
	g.ghiNhan(ctx, nguoi)
	g.idCuoi, g.lyDoCuoi = id, lyDo
	return g.loi
}

// --- harness ----------------------------------------------------------------------------------------

// tuyenGhiLich is every write route of this screen with a body that satisfies it. ONE TABLE, so a
// case written once cannot be added to one route and forgotten on another — which is exactly how a
// list route gets a permission check and its write route does not.
type tuyenGhiLich struct {
	ten    string
	method string
	duong  string
	than   string
	ok     int
}

func moiTuyenGhiLich() []tuyenGhiLich {
	return []tuyenGhiLich{
		{"thêm ca làm việc", "POST", "/api/v1/working-hours",
			`{"weekday":1,"start":"07:30","end":"11:30","note":"Buổi sáng"}`, http.StatusCreated},
		{"sửa ca làm việc", "PATCH", "/api/v1/working-hours/" + idCaLamViecThu,
			`{"end":"12:00"}`, http.StatusOK},
		{"xoá ca làm việc", "DELETE", "/api/v1/working-hours/" + idCaLamViecThu,
			`{"reason":"Xã đổi khung giờ hành chính"}`, http.StatusNoContent},
		{"gieo tuần mặc định", "POST", "/api/v1/working-hours/defaults", "", http.StatusOK},

		{"thêm ngày nghỉ lễ", "POST", "/api/v1/public-holidays",
			`{"date":"2026-09-02","name":"Quốc khánh"}`, http.StatusCreated},
		{"sửa ngày nghỉ lễ", "PATCH", "/api/v1/public-holidays/" + idNgayNghiLeThu,
			`{"name":"Quốc khánh 2/9"}`, http.StatusOK},
		{"xoá ngày nghỉ lễ", "DELETE", "/api/v1/public-holidays/" + idNgayNghiLeThu,
			`{"reason":"Nhập nhầm ngày"}`, http.StatusNoContent},
		{"gieo ngày nghỉ lễ", "POST", "/api/v1/public-holidays/defaults",
			`{"year":2026}`, http.StatusOK},

		{"thêm ngày làm bù", "POST", "/api/v1/swap-working-days",
			`{"date":"2026-02-21","start":"07:30","end":"11:30","name":"Làm bù nghỉ Tết"}`, http.StatusCreated},
		{"sửa ngày làm bù", "PATCH", "/api/v1/swap-working-days/" + idNgayLamBuThu,
			`{"end":"12:00"}`, http.StatusOK},
		{"xoá ngày làm bù", "DELETE", "/api/v1/swap-working-days/" + idNgayLamBuThu,
			`{"reason":"Thông báo của Thủ tướng đổi ngày"}`, http.StatusNoContent},
	}
}

// dungMayChuGhiLich builds the chain with a checker that grants `admin.sla` in commune A and nothing
// in commune B.
func dungMayChuGhiLich(t *testing.T) *mayChu {
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

func (m *mayChu) goiGhiLich(t *testing.T, tg tuyenGhiLich, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	return m.goi(t, tg.method, host, tg.duong, tg.than, tok)
}

// --- rule 5, invariant 7: four cases, every one of the eleven routes ------------------------------------

func TestLichGhi_401KhongToken(t *testing.T) {
	m := dungMayChuGhiLich(t)

	for _, tg := range moiTuyenGhiLich() {
		w := m.goiGhiLich(t, tg, hostA, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.ghiLich.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case ghi đã chạy %d lần", n)
	}
}

// 403 WITH THE WRONG PERMISSION. The account holds `admin.user` — a real key, the one the rest of
// this service's administration routes use — which has nothing to do with configuring a calendar.
//
// THIS IS ALSO WHAT HOLDS THE READ/WRITE ASYMMETRY. The three GET routes are AnyAuthenticated and
// answer 200 to this very account (lich_lam_viec_test.go); every write must answer 403.
func TestLichGhi_403SaiQuyen(t *testing.T) {
	m := dungMayChu(t) // the shipped default: quyenThu = admin.user in commune A

	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range moiTuyenGhiLich() {
		w := m.goiGhiLich(t, tg, hostA, tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — `admin.user` KHÔNG được sửa lịch làm việc của xã. Thân: %s",
				tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.ghiLich.soLanGoi(); n != 0 {
		t.Errorf("sai quyền mà use case ghi đã chạy %d lần — phép kiểm quyền phải chặn TRƯỚC", n)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE — the case that catches a permission crossing a
// commune boundary (rule 5, invariant 3). The same person, holding `admin.sla` in commune A,
// properly signed in at commune B: nothing about the request is malformed, the grant simply does not
// exist there.
//
// IT MATTERS MORE ON THIS TABLE THAN ON MOST: a calendar row written across the boundary would make
// one commune's promise to its citizens be counted through another commune's office hours.
func TestLichGhi_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChuGhiLich(t)

	tok := m.tokenCho(t, xaB, sidB)
	for _, tg := range moiTuyenGhiLich() {
		w := m.goiGhiLich(t, tg, hostB, tok)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — thân: %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.ghiLich.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà use case ghi đã chạy %d lần", n)
	}
}

func TestLichGhi_2xxDuCaHai(t *testing.T) {
	m := dungMayChuGhiLich(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, tg := range moiTuyenGhiLich() {
		w := m.goiGhiLich(t, tg, hostA, tok)
		if w.Code != tg.ok {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", tg.ten, w.Code, tg.ok, w.Body.String())
		}
	}
	if m.ghiLich.xaCuoi != xaA {
		t.Errorf("use case ghi được gọi với xã %q, muốn %q", m.ghiLich.xaCuoi, xaA)
	}
}

// --- rule 6, invariant 8: which identifier reaches the trail --------------------------------------------

// THE HANDLER PASSES THE STAFF CODE AS THE AUDIT ACTOR AND THE INTERNAL ID AS THE DECIDER. This is
// the one layer that chooses between a principal's two identifiers, and the wrong choice is
// invisible: a ULID is a perfectly valid string in `audit_log.actor_id` and in `deleted_by`.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestLichGhiChuTheVetLaMaCanBo(t *testing.T) {
	m := dungMayChuGhiLich(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, tg := range moiTuyenGhiLich() {
		m.ghiLich.nguoiCuoi = app.NguoiThucHien{}
		if w := m.goiGhiLich(t, tg, hostA, tok); w.Code != tg.ok {
			t.Fatalf("%s: mã = %d — thân: %s", tg.ten, w.Code, w.Body.String())
		}
		n := m.ghiLich.nguoiCuoi
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

// --- the wire shape: what the handler itself owns ---------------------------------------------------

// A TIME OF DAY REACHES THE USE CASE AS SECONDS SINCE MIDNIGHT, PARSED ONCE. The fixture uses
// 07:30/11:30 with DIFFERENT values in the two positions for the usual reason: transposing `start`
// and `end` produces no error anywhere, only a week that runs from closing time to opening time.
//
// MUTATION THAT MUST TURN THIS RED: swap BatDau and KetThuc in ThemCaLamViec.
func TestThemCaLamViecDocDungGioVaThu(t *testing.T) {
	m := dungMayChuGhiLich(t)
	w := m.goi(t, "POST", hostA, "/api/v1/working-hours",
		`{"weekday":3,"start":"07:30","end":"11:30:15","note":"Buổi sáng"}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusCreated {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	yc := m.ghiLich.themCa
	if yc.Thu != 3 {
		t.Errorf("thứ = %d, muốn 3 (ISO: thứ Tư)", yc.Thu)
	}
	if yc.BatDau != domain.GioTrongNgay(7*3600+30*60) {
		t.Errorf("bắt đầu = %d giây, muốn 07:30", yc.BatDau)
	}
	// SECONDS ARE KEPT. The column can hold them, and rounding to the minute would move a configured
	// boundary by up to 59 seconds with nothing on any screen to show it.
	if yc.KetThuc != domain.GioTrongNgay(11*3600+30*60+15) {
		t.Errorf("kết thúc = %d giây, muốn 11:30:15", yc.KetThuc)
	}
}

func TestThemCaLamViecGioSaiKhuonTraLoi400VaKhongGoiUseCase(t *testing.T) {
	m := dungMayChuGhiLich(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, than := range []string{
		`{"weekday":1,"start":"7:30","end":"11:30"}`,                 // one digit
		`{"weekday":1,"start":"07h30","end":"11:30"}`,                // not a colon
		`{"weekday":1,"start":"07:30","end":"25:00"}`,                // past midnight
		`{"weekday":1,"start":"07:30","end":"11:75"}`,                // minute out of range
		`{"weekday":1,"start":"","end":"11:30"}`,                     // empty
		`{"weekday":1,"start":"07:30","end":"11:30", ` + `"note":1}`, // note is not a string
	} {
		w := m.goi(t, "POST", hostA, "/api/v1/working-hours", than, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("thân %s: mã = %d, muốn 400", than, w.Code)
		}
	}
	if n := m.ghiLich.soLanGoi(); n != 0 {
		t.Errorf("giờ sai khuôn mà use case đã chạy %d lần — phải từ chối ở tầng dịch HTTP", n)
	}
}

// THE PARTIAL EDIT REALLY IS PARTIAL: a field the client did not mention arrives as nil, and nil
// means "leave it alone". With plain values this could not tell "not mentioned" from "sent zero",
// and zero is a legal time (midnight) and an illegal weekday.
func TestSuaCaLamViecChiMangTruongDuocGui(t *testing.T) {
	m := dungMayChuGhiLich(t)
	w := m.goi(t, "PATCH", hostA, "/api/v1/working-hours/"+idCaLamViecThu,
		`{"end":"12:00"}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	yc := m.ghiLich.suaCa
	if yc.Thu != nil || yc.BatDau != nil || yc.GhiChu != nil {
		t.Error("trường không được gửi lại đi tới use case — sửa một phần phải là sửa một phần")
	}
	if yc.KetThuc == nil || *yc.KetThuc != domain.GioTrongNgay(12*3600) {
		t.Errorf("kết thúc = %v, muốn 12:00", yc.KetThuc)
	}
	if m.ghiLich.idCuoi != idCaLamViecThu {
		t.Errorf("id = %q, muốn %q", m.ghiLich.idCuoi, idCaLamViecThu)
	}
}

// `{}` IS REFUSED RATHER THAN TREATED AS A NO-OP. It means the client sent a form it failed to read,
// and answering 200 would tell the person their edit was saved.
func TestSuaLichThanRongTraLoi400VaKhongGoiUseCase(t *testing.T) {
	m := dungMayChuGhiLich(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, duong := range []string{
		"/api/v1/working-hours/" + idCaLamViecThu,
		"/api/v1/public-holidays/" + idNgayNghiLeThu,
		"/api/v1/swap-working-days/" + idNgayLamBuThu,
	} {
		w := m.goi(t, "PATCH", hostA, duong, `{}`, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: mã = %d, muốn 400", duong, w.Code)
		}
	}
	if n := m.ghiLich.soLanGoi(); n != 0 {
		t.Errorf("thân rỗng mà use case đã chạy %d lần", n)
	}
}

// THE REASON REACHES THE USE CASE, AND IT IS IN THE BODY RATHER THAN THE QUERY STRING: a free-text
// sentence somebody typed about a government record does not belong in every access log and proxy
// cache between here and the browser.
func TestXoaLichMangLyDoToiUseCase(t *testing.T) {
	m := dungMayChuGhiLich(t)
	const lyDo = "Nhập nhầm ngày, thông báo của Thủ tướng ghi ngày khác"

	w := m.goi(t, "DELETE", hostA, "/api/v1/public-holidays/"+idNgayNghiLeThu,
		`{"reason":"`+lyDo+`"}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusNoContent {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	if m.ghiLich.lyDoCuoi != lyDo {
		t.Errorf("lý do = %q, muốn %q", m.ghiLich.lyDoCuoi, lyDo)
	}
	if w.Body.Len() != 0 {
		t.Errorf("204 mà vẫn có thân: %s", w.Body.String())
	}
}

// --- the seed routes --------------------------------------------------------------------------------

func TestGieoTuanTraDuBaConSo(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.gieo = app.KetQuaGieoLich{DaGieo: 8, DaCo: 1, BoQua: 1}

	w := m.goi(t, "POST", hostA, "/api/v1/working-hours/defaults", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	var ra gieoLichRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// THREE COUNTS AND NOT A BOOLEAN. `skipped` is the one nobody expects and the one the screen
	// owes a sentence: it means "we did not write it, and you should look at why".
	if ra.Seeded != 8 || ra.Kept != 1 || ra.Skipped != 1 {
		t.Errorf("thân = %+v, muốn {Seeded:8 Kept:1 Skipped:1}", ra)
	}
}

// THE YEAR IS MANDATORY AND HAS NO DEFAULT — the same refusal docNamTruyVan makes on the read
// routes. "Năm nay" would silently switch at midnight on 31/12.
func TestGieoNgayNghiLeThieuNamTraLoi400VaKhongGoiUseCase(t *testing.T) {
	m := dungMayChuGhiLich(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, than := range []string{`{}`, `{"year":1999}`, `{"year":3000}`} {
		w := m.goi(t, "POST", hostA, "/api/v1/public-holidays/defaults", than, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("thân %s: mã = %d, muốn 400 — thân: %s", than, w.Code, w.Body.String())
		}
	}
	if n := m.ghiLich.soLanGoi(); n != 0 {
		t.Errorf("năm thiếu hoặc ngoài khoảng mà use case đã chạy %d lần", n)
	}
}

func TestGieoNgayNghiLeMangDungNamNguoiGoiChon(t *testing.T) {
	m := dungMayChuGhiLich(t)
	w := m.goi(t, "POST", hostA, "/api/v1/public-holidays/defaults",
		`{"year":2027}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	if m.ghiLich.namCuoi != 2027 {
		t.Errorf("năm tới use case = %d, muốn 2027", m.ghiLich.namCuoi)
	}
}

// --- the refusals: each one maps to the status the screen can act on -------------------------------------

// A CALENDAR THAT CONTRADICTS ITSELF ANSWERS 409 `calendar_conflict` — THE SAME CODE AND THE SAME
// SENTENCE THE TWO READ ROUTES ALREADY ANSWER WITH. One fault, one spelling: the screen that shows
// the conflict and the screen that refused the write must not describe one state with two words.
func TestGhiLichNgayVuaNghiVuaLamBuTraLoi409(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.loi = &domain.LoiNgayVuaNghiVuaLamBu{Ngay: []string{"2026-02-21"}}

	w := m.goi(t, "POST", hostA, "/api/v1/swap-working-days",
		`{"date":"2026-02-21","start":"07:30","end":"11:30","name":"Làm bù nghỉ Tết"}`,
		m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — thân: %s", w.Code, w.Body.String())
	}
	var loi map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &loi)
	if loi["code"] != "calendar_conflict" {
		t.Errorf("code = %v, muốn calendar_conflict", loi["code"])
	}
	// THE MESSAGE NAMES THE DATE. A date is not personal data (rule 3) — it says when an authority
	// is open — and a refusal that does not say which day to look at is a refusal nobody can act on.
	if msg, _ := loi["message"].(string); msg == "" || !chuaChuoi(msg, "2026-02-21") {
		t.Errorf("thông điệp không nêu ngày: %q", msg)
	}
}

// ADR 0007 DECISION 9 REACHES THE CLIENT AS ITS OWN CODE, so a screen can tell "you are closed and
// working on one day" from "this weekday already works" — two different rows to go and fix.
func TestGhiLichLamBuTrungNgayDaLamTraLoi409(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.loi = domain.LoiLamBuVaoNgayDaLamViec("2026-02-21", 6)

	w := m.goi(t, "POST", hostA, "/api/v1/swap-working-days",
		`{"date":"2026-02-21","start":"07:30","end":"11:30","name":"Làm bù nghỉ Tết"}`,
		m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — thân: %s", w.Code, w.Body.String())
	}
	var loi map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &loi)
	if loi["code"] != string(domain.LoiLamBuTrungNgayDaLam) {
		t.Errorf("code = %v, muốn %q", loi["code"], domain.LoiLamBuTrungNgayDaLam)
	}
}

// TWO OVERLAPPING SESSIONS ANSWER 409 `overlapping_sessions` — THE SAME IDENTIFIER THE READ ROUTE
// PUBLISHES IN ITS `problems` LIST. A client that already knows how to draw the problem needs no
// second vocabulary to draw the refusal.
func TestGhiLichCaChongNhauTraLoi409(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.loi = &domain.LoiCaChongCaKhac{CaID: idCaLamViecThu, Nhom: "1"}

	w := m.goi(t, "POST", hostA, "/api/v1/working-hours",
		`{"weekday":1,"start":"09:00","end":"12:00"}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — thân: %s", w.Code, w.Body.String())
	}
	var loi map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &loi)
	if loi["code"] != string(domain.VanDeCaChongNhau) {
		t.Errorf("code = %v, muốn %q", loi["code"], domain.VanDeCaChongNhau)
	}
}

// 404 FOR ANOTHER COMMUNE'S ID, NOT 403: every statement is scoped, so an invented id, a
// soft-deleted row and another authority's row are one single answer and none can be told apart by
// trying (rule 4, forbidden #2).
func TestGhiLichKhongTimThayTraLoi404(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.loi = idstore.ErrCaLamViecKhongTonTai

	w := m.goi(t, "PATCH", hostA, "/api/v1/working-hours/"+idCaLamViecThu,
		`{"end":"12:00"}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusNotFound {
		t.Fatalf("mã = %d, muốn 404 — thân: %s", w.Code, w.Body.String())
	}
}

// THE SOFT-DELETED ROW STILL HOLDS ITS OPENING MINUTE, and the sentence has to say so — otherwise
// the refusal reads as a bug. The unique key deliberately carries no `WHERE deleted_at IS NULL`,
// because a partial unique index is what lets an issued value be reissued (rule 7, invariant 3).
func TestGhiLichTrungGioMoTraLoi409VaNoiRoViSao(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.loi = idstore.ErrCaLamViecTrungGioMo

	w := m.goi(t, "POST", hostA, "/api/v1/working-hours",
		`{"weekday":1,"start":"07:30","end":"11:30"}`, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — thân: %s", w.Code, w.Body.String())
	}
	var loi map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &loi)
	if loi["code"] != "session_start_taken" {
		t.Errorf("code = %v, muốn session_start_taken", loi["code"])
	}
	if msg, _ := loi["message"].(string); !chuaChuoi(msg, "đã bị xoá") {
		t.Errorf("thông điệp không nói vì sao một ca không thấy trên màn hình lại chặn: %q", msg)
	}
}

// A STORE FAILURE NEVER REACHES THE CLIENT (rule 3, forbidden #3): the wrapped error carries the
// statement and the constraint name, neither of which a clerk in a commune can act on.
func TestGhiLichLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChuGhiLich(t)
	m.ghiLich.loi = errKhoLich

	w := m.goi(t, "POST", hostA, "/api/v1/working-hours/defaults", "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("mã = %d, muốn 500 — thân: %s", w.Code, w.Body.String())
	}
	if chuaChuoi(w.Body.String(), "pq: connection refused") {
		t.Errorf("lỗi kho lọt ra ngoài: %s", w.Body.String())
	}
}

var errKhoLich = &loiKhoGia{}

type loiKhoGia struct{}

func (e *loiKhoGia) Error() string { return "pq: connection refused" }

// chuaChuoi is strings.Contains under a name that reads in the assertions above. It exists so the
// message checks say what they mean — "the sentence names the date" — rather than restating the
// standard library at every call site.
func chuaChuoi(s, tu string) bool { return strings.Contains(s, tu) }
