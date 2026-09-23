package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for GET /api/v1/tasks and GET /api/v1/tasks/{ma}.
//
// THE FOUR CASES RULE 5 INVARIANT 7 REQUIRES ARE ALL HERE, and the third is the one no test of a
// single commune can produce: 401 with no session · 403 with the wrong permission · 401 with the
// right permission but a session issued by ANOTHER commune · 200 with both correct.
//
// The commune case answers 401 rather than 403, and that is authz.xacNhanXa's behaviour, not a slip:
// the commune on the principal is compared with the commune resolved from Host BEFORE the permission
// is consulted, so a token from another commune never reaches the permission check. The property
// being asserted is that NO QUERY RUNS — `goi` on the fake counts the reads.

// --- fakes ------------------------------------------------------------------------------------

const (
	maNhiemVuA  = "NV19"
	maNhiemVuB  = "NV07" // exists ONLY in commune B
	maNhiemVuXo = "NV88" // finished, and finished LATE against the original deadline
)

// nhiemVuGia is the task register, KEYED BY COMMUNE, reading the commune from the context exactly as
// *store.Scoped does. Keyed any other way, the isolation cases would pass while proving nothing.
//
// `goi` counts the reads — the count is what proves the permission and commune checks happen BEFORE
// any store access. `locCuoi` keeps the filter the route handed down, which is the only way to assert
// that `scope=mine` resolved the caller's OWN code rather than whatever the query string said.
type nhiemVuGia struct {
	theo    map[tenant.ID][]domain.NhiemVu
	loi     error
	goi     int
	locCuoi petstore.LocNhiemVu
}

func (n *nhiemVuGia) TheoMa(ctx context.Context, ma string) (domain.NhiemVu, error) {
	n.goi++
	if n.loi != nil {
		return domain.NhiemVu{}, n.loi
	}
	for _, nv := range n.theo[tenant.MustFrom(ctx)] {
		if nv.Ma == ma {
			return nv, nil
		}
	}
	return domain.NhiemVu{}, petstore.ErrNhiemVuKhongTonTai
}

func (n *nhiemVuGia) DanhSach(ctx context.Context, loc petstore.LocNhiemVu, _ page.Request) (
	page.Result[domain.NhiemVu], error) {

	n.goi++
	n.locCuoi = loc
	if n.loi != nil {
		return page.Result[domain.NhiemVu]{}, n.loi
	}
	kq := page.NewResult[domain.NhiemVu]()
	kq.Items = append(kq.Items, n.theo[tenant.MustFrom(ctx)]...)
	return kq, nil
}

// The fixture instants are FIXED, not relative to time.Now(): a deadline expressed as "two hours ago"
// makes the assertions depend on when the suite runs, and the one thing these deadlines must do is
// come out of the response byte for byte.
var (
	mocHanNVA    = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mocHanGocNVA = time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	mocXongNVA   = time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	mocTaoNVA    = time.Date(2026, 6, 1, 3, 30, 0, 0, time.UTC)
)

// nhiemVuMau gives commune A two tasks with DIFFERENT shapes and commune B one task whose number
// exists only there.
//
//	maNhiemVuA   running, EXTENDED — the two deadlines differ, which is what makes a swap visible
//	maNhiemVuXo  finished, before the NEW deadline and after the ORIGINAL one: TreHan is false and
//	             the on-time ratio must say "late". No single boolean can carry both answers, which
//	             is why the response carries two dates and no verdict.
func nhiemVuMau() *nhiemVuGia {
	return &nhiemVuGia{theo: map[tenant.ID][]domain.NhiemVu{
		xaA: {
			{
				ID: "nv-001", Ma: maNhiemVuA,
				Loai: "theo-van-ban", Khoi: "khoi-dang", MucUuTien: "cao",
				TieuDe:    "Báo cáo tổng kết việc thực hiện chủ trương về công tác cán bộ",
				MoTa:      "Tổng hợp số liệu từ các chi bộ trực thuộc.",
				TrangThai: domain.DangThucHien,
				NguonGiao: domain.NguonKetLuanHop, NguonID: "klh-007",
				BoPhanID: "bp-vpdu", NguoiThucHienMa: "CB-00311",
				LanhDaoGiaoViecMa: "CB-00007", ChuyenVienTheoDoiMa: "CB-00412",
				HanXuLy: mocHanNVA, HanBanDau: mocHanGocNVA,
				TienDo: 40, NguoiTaoMa: maCanBo, TaoLuc: mocTaoNVA,
			},
			{
				ID: "nv-002", Ma: maNhiemVuXo,
				Loai: "co-ban", MucUuTien: "thuong",
				TieuDe:    "Rà soát tiến độ tuyến đường Hà Lam – Bình Trị",
				TrangThai: domain.HoanThanh,
				NguonGiao: domain.NguonTrucTiep,
				// ASSIGNED TO THE CALLER — this is the row `scope=mine` must return.
				NguoiThucHienMa: maCanBo,
				HanXuLy:         mocHanNVA, HanBanDau: mocHanGocNVA,
				NgayHoanThanh: mocXongNVA,
				TienDo:        100, NguoiTaoMa: maCanBo, TaoLuc: mocTaoNVA,
			},
		},
		xaB: {
			{
				ID: "nv-b-001", Ma: maNhiemVuB,
				Loai: "kiem-tra", TieuDe: "Nhiệm vụ của xã B.",
				TrangThai: domain.MoiGiao, NguonGiao: domain.NguonTrucTiep,
				NguoiTaoMa: maCanBo, TaoLuc: mocTaoNVA,
			},
		},
	}}
}

func docNhiemVu(t *testing.T, than []byte) nhiemVuRa {
	t.Helper()
	var ra nhiemVuRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

func docTrangNhiemVu(t *testing.T, than []byte) page.Result[nhiemVuRa] {
	t.Helper()
	var ra page.Result[nhiemVuRa]
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

func duongNhiemVu(ma string) string { return "/api/v1/tasks/" + ma }

// --- rule 5, invariant 7: the four cases, on the DETAIL route ------------------------------------

func TestDocNhiemVuKhongCoPhienThi401(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), nil)

	doiMa(t, w, http.StatusUnauthorized)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần dù chưa có phiên — phép kiểm phải chặn TRƯỚC khi chạm dữ liệu", m.nhiemVu.goi)
	}
}

func TestDocNhiemVuSaiQuyenThi403(t *testing.T) {
	m := dungMayChu(t)
	// An account of commune A holding NO permission at all — a real state: somebody whose role was
	// withdrawn still has a valid session.
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {}}}
	})

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusForbidden)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần dù thiếu quyền `task.read`", m.nhiemVu.goi)
	}
}

// TestDocNhiemVuDungQuyenSaiXaThi401 is the case a single-commune test suite can never produce, and
// the one rule 1 exists for: a principal issued by commune B, holding `task.read` IN COMMUNE B,
// arriving at commune A's host.
func TestDocNhiemVuDungQuyenSaiXaThi401(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			// Deliberately granted in BOTH communes, so the refusal cannot be mistaken for a missing
			// grant. What refuses is the commune comparison, before the grant is read.
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaB))

	doiMa(t, w, http.StatusUnauthorized)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần cho một phiên của xã khác — lẽ ra không truy vấn nào chạy", m.nhiemVu.goi)
	}
}

func TestDocNhiemVuDuQuyenDungXaThi200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	ra := docNhiemVu(t, w.Body.Bytes())

	if ra.Code != maNhiemVuA {
		t.Errorf("code = %q, muốn %q", ra.Code, maNhiemVuA)
	}
	if ra.Status != string(domain.DangThucHien) {
		t.Errorf("status = %q, muốn %q", ra.Status, domain.DangThucHien)
	}
	if ra.Type != "theo-van-ban" || ra.Bloc != "khoi-dang" || ra.Priority != "cao" {
		t.Errorf("ba mã danh mục sai: type=%q bloc=%q priority=%q", ra.Type, ra.Bloc, ra.Priority)
	}
	// FOUR STAFF BUSINESS CODES, and the pair most expensive to swap: the extension request goes to
	// `assigner`, and sending it to `assignee` would route it to the person who has to ask for it.
	if ra.Assignee != "CB-00311" || ra.Assigner != "CB-00007" {
		t.Errorf("assignee=%q assigner=%q — hai mã cán bộ này không được đổi chỗ nhau",
			ra.Assignee, ra.Assigner)
	}
	if ra.Source != string(domain.NguonKetLuanHop) || ra.SourceID != "klh-007" {
		t.Errorf("nguồn giao sai: source=%q source_id=%q", ra.Source, ra.SourceID)
	}
}

// --- rule 1: a number of another commune is simply not there -------------------------------------

func TestDocNhiemVuMaCuaXaKhacThi404(t *testing.T) {
	m := dungMayChu(t)

	// A VALID number — it exists, in commune B — presented at commune A with a commune A session.
	// Nothing about it is malformed, so the only thing that can refuse it is the commune scope.
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuB), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusNotFound)
	if e := loiTra(t, w); e.Code != "not_found" {
		t.Errorf("mã lỗi = %q, muốn %q", e.Code, "not_found")
	}
}

// --- rule 10, invariant 3: the response carries DATES, never a verdict ---------------------------

// TestNhiemVuRaKhongCoCoQuaHan is the assertion that keeps rule 10 invariant 3 true at the edge of
// the service. Overdue is DERIVED; a boolean on the wire is a second representation of one fact,
// frozen at the instant the response was built while the chip that shows it stays on screen.
func TestNhiemVuRaKhongCoCoQuaHan(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	// ON THE RAW BODY, not on the decoded struct: it catches the field appearing under ANY name a
	// later edit might give it.
	than := w.Body.String()
	for _, cam := range []string{"overdue", "is_late", "qua_han", "tre_han"} {
		if strings.Contains(than, cam) {
			t.Errorf("phản hồi mang trường %q — quá hạn là SUY RA từ hạn, không phải một giá trị gửi đi", cam)
		}
	}

	// AND BOTH DATES ARE THERE, or the client could not derive it. `due_at` is the current
	// commitment and `original_due_at` the one first made; §5.6 shows the two side by side.
	ra := docNhiemVu(t, w.Body.Bytes())
	if ra.DueAt == nil || !ra.DueAt.Equal(mocHanNVA) {
		t.Errorf("due_at = %v, muốn %v", ra.DueAt, mocHanNVA)
	}
	if ra.OriginalDueAt == nil || !ra.OriginalDueAt.Equal(mocHanGocNVA) {
		t.Errorf("original_due_at = %v, muốn %v — §5.8 hứa hạn gốc KHÔNG lùi theo", ra.OriginalDueAt, mocHanGocNVA)
	}
	if ra.CompletedAt != nil {
		t.Errorf("completed_at = %v cho nhiệm vụ chưa xong", ra.CompletedAt)
	}
}

// TestNhiemVuKhongCoHanThiHaiMocLaNull: a zero time.Time marshalled straight would travel as
// `0001-01-01T00:00:00Z` — a date a screen renders and a comparison calls overdue. §4.1 renders this
// state as `Hạn —`.
func TestNhiemVuKhongCoHanThiHaiMocLaNull(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostB, duongNhiemVu(maNhiemVuB), canBoCuaXa(xaB))
	// Commune B's account holds nothing in the default harness, so grant it the read key only.
	if w.Code != http.StatusOK {
		m.dungLai(t, func(d *Deps) {
			d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
				xaB: {idCanBo: {authz.Perm("task.read"): true}},
			}}
		})
		w = m.goi(t, http.MethodGet, hostB, duongNhiemVu(maNhiemVuB), canBoCuaXa(xaB))
	}
	doiMa(t, w, http.StatusOK)

	if strings.Contains(w.Body.String(), "0001-01-01") {
		t.Error("mốc rỗng đi ra thành ngày năm 1 — màn hình sẽ vẽ nó và phép so sẽ gọi nó là quá hạn")
	}
	ra := docNhiemVu(t, w.Body.Bytes())
	if ra.DueAt != nil || ra.OriginalDueAt != nil {
		t.Errorf("nhiệm vụ không có hạn lại có due_at=%v original_due_at=%v", ra.DueAt, ra.OriginalDueAt)
	}
}

// --- the list route -------------------------------------------------------------------------------

func TestDanhSachNhiemVuKhongCoPhienThi401(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", nil)

	doiMa(t, w, http.StatusUnauthorized)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần dù chưa có phiên", m.nhiemVu.goi)
	}
}

func TestDanhSachNhiemVuSaiQuyenThi403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {}}}
	})

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))

	doiMa(t, w, http.StatusForbidden)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần dù thiếu quyền", m.nhiemVu.goi)
	}
}

func TestDanhSachNhiemVuDungQuyenSaiXaThi401(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaB))

	doiMa(t, w, http.StatusUnauthorized)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần cho một phiên của xã khác", m.nhiemVu.goi)
	}
}

func TestDanhSachNhiemVuChiTraVeXaCuaMinh(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docTrangNhiemVu(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("trả %d nhiệm vụ, muốn 2 của xã A", len(ra.Items))
	}
	for _, it := range ra.Items {
		if it.Code == maNhiemVuB {
			t.Errorf("nhiệm vụ của xã B lọt vào danh sách xã A: %q", it.Code)
		}
	}
}

// TestDanhSachNhiemVuPhamViToiLayMaTuPHIENChuKhongPhaiTuURL is the isolation assertion of this
// route, and it is not about leaking data — it is about a screen that answers a different question
// from the one it asked.
//
// `scope=mine` must resolve the caller's OWN staff code from the session. A handler that read it
// from the query string would let `?scope=mine&assignee=CB-99999` render somebody else's board under
// the heading "Giao cho tôi" — and nothing would report it.
func TestDanhSachNhiemVuPhamViToiLayMaTuPhienChuKhongPhaiTuURL(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?scope=mine&assignee=CB-99999", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if got := m.nhiemVu.locCuoi.NguoiThucHienMa; got != maCanBo {
		t.Errorf("lọc theo người thực hiện = %q, muốn mã của CHÍNH người đăng nhập (%q) — "+
			"`Giao cho tôi` đọc danh tính từ phiên, không từ URL", got, maCanBo)
	}
}

// TestDanhSachNhiemVuPhamViToiMaThieuMaCanBoThi500 — FAIL CLOSED. With no business code to compare
// against, the two honest answers are "refuse" and "return the whole register", and the second is a
// screen labelled `Giao cho tôi` showing every task in the commune, which nobody would report as a
// fault.
func TestDanhSachNhiemVuPhamViToiMaThieuMaCanBoThi500(t *testing.T) {
	m := dungMayChu(t)

	// A principal with no `Ma` — the shape identity returns when the staff record carries no
	// business code. Authorisation still works (it joins on ID), so the route is reached.
	p := &authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xaA}
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?scope=mine", p)

	doiMa(t, w, http.StatusInternalServerError)
	if m.nhiemVu.goi != 0 {
		t.Errorf("đã đọc kho %d lần — lẽ ra từ chối TRƯỚC khi truy vấn, không mở rộng ra toàn xã", m.nhiemVu.goi)
	}
}

// TestDanhSachNhiemVuBoLocKhongHopLeBiTuChoiChuKhongBiBoQua.
//
// A filter silently dropped returns a page that answers a DIFFERENT question from the one the screen
// asked, and the screen has no way to know — which here means an officer believing they are looking
// at every task of one kind. Each case below must be a 400, and NO query may run.
func TestDanhSachNhiemVuBoLocKhongHopLeBiTuChoiChuKhongBiBoQua(t *testing.T) {
	for ten, truyVan := range map[string]string{
		// `chua-thuc-hien` is what the KANBAN BOARD calls `moi-giao` (§4.1) — a LABEL, and the one
		// most likely to be sent as a code by a client written from the screen.
		"trạng thái là NHÃN chứ không phải mã": "?status=chua-thuc-hien",
		"trạng thái bịa":                       "?status=dang-cho-gi-do",
		"nguồn giao bịa":                       "?source=excel",
		"ô trễ hạn gửi `1` thay vì `true`":     "?late=1",
		"phạm vi bịa":                          "?scope=toan-quoc",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+truyVan, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if m.nhiemVu.goi != 0 {
				t.Errorf("chạy %d truy vấn dù bộ lọc bị từ chối", m.nhiemVu.goi)
			}
		})
	}
}

// TestHaiBoLocChuaCoDuongDocThiNoiThangRaChuKhongDoanBua is the pair this pass refuses on purpose.
//
// Both need a number or a field that no contract exposes today, and both would be trivially easy to
// fake: 72 hours is written in §3, and three of `related`'s four clauses are expressible. Faking
// either produces a screen that is wrong and silent — a "sắp đến hạn" list measured against a
// threshold no commune chose, or a "Liên quan đến tôi" tab missing the tasks the officer's own
// department holds.
func TestHaiBoLocChuaCoDuongDocThiNoiThangRaChuKhongDoanBua(t *testing.T) {
	for ten, tr := range map[string]struct{ truyVan, chua string }{
		"sắp đến hạn":       {"?soon=true", "sla"},
		"liên quan đến tôi": {"?scope=related", "bộ phận"},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+tr.truyVan, canBoCuaXa(xaA))

			doiMa(t, w, http.StatusBadRequest)
			if m.nhiemVu.goi != 0 {
				t.Errorf("chạy %d truy vấn cho một bộ lọc chưa cài đặt được", m.nhiemVu.goi)
			}
			// THE MESSAGE HAS TO NAME WHAT IS MISSING. A bare "invalid_request" here reads as "the
			// client sent rubbish", and the next person to see it re-implements the filter with a
			// guessed constant.
			if e := loiTra(t, w); !strings.Contains(e.Message, tr.chua) {
				t.Errorf("thông báo không nói thiếu gì (%q): %q", tr.chua, e.Message)
			}
		})
	}
}

// TestDanhSachNhiemVuBoLocHopLeDiXuongKho — the other direction. A route that refused everything
// would pass the test above while serving nobody.
func TestDanhSachNhiemVuBoLocHopLeDiXuongKho(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA,
		"/api/v1/tasks?status=dang-thuc-hien&source=ket-luan-hop&type=theo-van-ban"+
			"&bloc=khoi-dang&priority=cao&unit=bp-vpdu&assignee=CB-00311&q=Hà+Lam&late=true",
		canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	muon := petstore.LocNhiemVu{
		TrangThai: "dang-thuc-hien", Loai: "theo-van-ban", Khoi: "khoi-dang",
		MucUuTien: "cao", BoPhanID: "bp-vpdu", NguoiThucHienMa: "CB-00311",
		NguonGiao: "ket-luan-hop", Tim: "Hà Lam", ChiTreHan: true,
	}
	if m.nhiemVu.locCuoi != muon {
		t.Errorf("bộ lọc xuống kho = %+v,\nmuốn %+v", m.nhiemVu.locCuoi, muon)
	}
}

// TestDanhSachNhiemVuRongThiItemsLaMangRong: `items` must marshal as [] on a commune whose register
// is empty, never as null. A newly onboarded commune has exactly that, and a client that has to
// handle both shapes handles one of them wrong.
func TestDanhSachNhiemVuRongThiItemsLaMangRong(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		trong := &nhiemVuGia{theo: map[tenant.ID][]domain.NhiemVu{}}
		d.NhiemVu = trong
		d.DanhSachNhiemVu = trong
	})

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("sổ rỗng không trả `items: []`: %s", w.Body.String())
	}
}

func TestDanhSachNhiemVuLoiKhoThi500VaKhongLoRaNgoai(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		hong := &nhiemVuGia{loi: errors.New("pg: connection refused on host db-07")}
		d.NhiemVu = hong
		d.DanhSachNhiemVu = hong
	})

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	// The wrapped store failure never reaches the client: an error message returned to a caller is
	// where infrastructure detail leaks out of a government system.
	if strings.Contains(w.Body.String(), "db-07") || strings.Contains(w.Body.String(), "connection refused") {
		t.Errorf("chi tiết hạ tầng lọt ra phản hồi: %s", w.Body.String())
	}
}
