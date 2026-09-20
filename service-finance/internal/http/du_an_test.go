package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The four cases rule 5, invariant 7 requires, for both disbursement routes:
//
//	401  no session
//	403  a session with the WRONG permission
//	403  the RIGHT permission, the WRONG commune   <- the one a single-commune test cannot produce
//	200  both correct
//
// plus what those four cannot show: that a commune's projects never reach another commune's
// caller, that the delay score is derived rather than stored, and that a budget year is demanded
// rather than defaulted.
//
// THE 403-WRONG-COMMUNE CASE ANSWERS 401 HERE, NOT 403, and that is the shipped behaviour rather
// than a compromise: authz.RequirePermission compares the commune in the principal against the
// commune resolved from Host BEFORE it consults the checker, and answers 401 — a browser does not
// send a cookie across hosts, so a mismatch is never an ordinary user error but a stolen or
// replayed token. What the rule asks for is that the case be EXERCISED and refused; the assertion
// below pins the code that ships so that a change to it cannot pass unnoticed.

// lucDaQua7096 is the instant at which the 2026 budget year is 70,96% elapsed — the figure §3 uses
// in its worked examples. The routes are built with a clock fixed here (dungMayChuVoi), so every
// delay score below is reproducible.
var lucDaQua7096 = time.Date(2026, time.September, 17, 0, 6, 0, 0, time.FixedZone("ICT", 7*3600))

// duAnGia is the project store, KEYED BY COMMUNE, reading the commune from the context exactly as
// *store.Scoped does. Keyed any other way, the isolation cases below would pass while proving
// nothing.
//
// `goi` counts the reads: the count is what proves the permission guard runs BEFORE any store
// access, rather than merely producing the right status afterwards.
type duAnGia struct {
	theo map[tenant.ID][]domain.TienDoDuAn
	loi  error
	goi  int
}

func (d *duAnGia) DanhSach(ctx context.Context, loc fistore.LocDuAn) ([]domain.TienDoDuAn, error) {
	d.goi++
	if d.loi != nil {
		return nil, d.loi
	}
	var ra []domain.TienDoDuAn
	for _, mot := range d.theo[tenant.MustFrom(ctx)] {
		if mot.DuAn.Nam != loc.Nam {
			continue
		}
		if loc.HangMucID != "" && mot.DuAn.HangMucID != loc.HangMucID {
			continue
		}
		ra = append(ra, mot)
	}
	return ra, nil
}

func (d *duAnGia) ChiTiet(ctx context.Context, id string) (domain.TienDoDuAn, error) {
	d.goi++
	if d.loi != nil {
		return domain.TienDoDuAn{}, d.loi
	}
	for _, mot := range d.theo[tenant.MustFrom(ctx)] {
		if mot.DuAn.ID == id {
			return mot, nil
		}
	}
	// The store answers the same way for "not here" and "belongs to another commune", because it
	// cannot tell them apart — the query never reaches the other commune's rows.
	return domain.TienDoDuAn{}, fistore.ErrKhongThayDuAn
}

// duAnMau gives commune A two projects of 2026 and one of 2025, and commune B one project whose
// id COLLIDES with one of A's.
//
// THE COLLIDING ID IS THE POINT: it is the only fixture shape that can show a detail route reading
// across communes, and ids do collide in the field — two communes onboarded from the same import
// produce the same sequence.
func duAnMau() *duAnGia {
	han := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	return &duAnGia{theo: map[tenant.ID][]domain.TienDoDuAn{
		xaA: {
			{
				// §8's worked project: 100 triệu planned, 90 triệu disbursed -> 90%, ahead of the
				// calendar at 70,96% elapsed, so NOT delayed.
				DuAn: domain.DuAn{ID: "da-001", Ma: "DA-2026-be-tong-hoa-duong-ngo-xo-2", Nam: 2026,
					HangMucID: "hm-001", Ten: "Bê tông hoá đường ngõ xóm tổ 6",
					KeHoachVonNam: 100_000_000, ThoiHanGiaiNgan: han},
				DaGiaiNgan: 90_000_000,
			},
			{
				// Nothing disbursed: delay score is exactly the elapsed share of the year, 7096.
				DuAn: domain.DuAn{ID: "da-002", Ma: "DA-2026-nong-thon-moi", Nam: 2026,
					HangMucID: "hm-002", Ten: "Kế hoạch vốn nông thôn mới",
					KeHoachVonNam: 25_000_000_000, ThoiHanGiaiNgan: han},
				DaGiaiNgan: 0,
			},
			{
				// A DIFFERENT budget year. It must never appear in a 2026 list (§13 rule 8).
				DuAn: domain.DuAn{ID: "da-2025", Ma: "DA-2025-cu", Nam: 2025,
					HangMucID: "hm-001", Ten: "Dự án năm cũ",
					KeHoachVonNam: 50_000_000, ThoiHanGiaiNgan: han},
				DaGiaiNgan: 50_000_000,
			},
		},
		xaB: {
			{
				DuAn: domain.DuAn{ID: "da-001", Ma: "DA-2026-CUA-XA-B", Nam: 2026,
					HangMucID: "hm-b-001", Ten: "Dự án của XÃ B",
					KeHoachVonNam: 9_000_000_000, ThoiHanGiaiNgan: han},
				DaGiaiNgan: 1_000_000,
			},
		},
	}}
}

const duongDanDuAn = "/api/v1/disbursements/projects?year=2026"

// --- rule 5, invariant 7: the four cases, on the list route -----------------------------------

func TestDanhSachDuAnKhongCoPhienThi401(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, nil)
	doiMa(t, w, http.StatusUnauthorized)
	if m.duAn.goi != 0 {
		t.Fatal("không có phiên mà vẫn chạm kho — rào quyền phải chặn TRƯỚC")
	}
}

func TestDanhSachDuAnSaiQuyenThi403(t *testing.T) {
	// An account that can sign in and holds a different budget key. `budget.update` is a real key
	// and deliberately NOT the one the route asks for: rule 5 invariant 3b says these rights are
	// not a Cartesian product, so holding one budget permission grants nothing about another.
	m := dungMayChuVoi(t, coQuyen("budget.update"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusForbidden)
	if m.duAn.goi != 0 {
		t.Fatal("sai quyền mà vẫn chạm kho — rào quyền phải chặn TRƯỚC")
	}
	if loiTra(t, w).Code != "forbidden" {
		t.Fatalf("mã lỗi = %q, muốn forbidden", loiTra(t, w).Code)
	}
}

func TestDanhSachDuAnDungQuyenSaiXaThiTUCHOI(t *testing.T) {
	// THE CASE A SINGLE-COMMUNE TEST CANNOT PRODUCE: a principal issued for commune B, holding the
	// right permission, arriving at commune A's host. Refused before the checker is consulted, and
	// before the store is touched.
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaB))
	doiMa(t, w, http.StatusUnauthorized)
	if m.duAn.goi != 0 {
		t.Fatal("token của xã khác mà vẫn chạm kho — leo thang quyền chéo xã")
	}
}

func TestDanhSachDuAnDuQuyenDungXaThi200(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	var ra danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	// Two projects of 2026 — the 2025 one must not be here (§13 rule 8).
	if len(ra.Items) != 2 {
		t.Fatalf("số dự án = %d, muốn 2 (dự án năm 2025 KHÔNG được lẫn vào)", len(ra.Items))
	}
	if ra.Year != 2026 {
		t.Fatalf("year = %d, muốn 2026", ra.Year)
	}
	if ra.DelayThreshold != int64(domain.NguongCanhBaoChamMacDinh) {
		t.Fatalf("ngưỡng = %d, muốn %d — phản hồi phải nói NÓ đã dùng ngưỡng nào",
			ra.DelayThreshold, int64(domain.NguongCanhBaoChamMacDinh))
	}
}

// --- rule 5, invariant 7: the four cases, on the detail route ---------------------------------

func TestChiTietDuAnBonCaQuyen(t *testing.T) {
	duong := "/api/v1/disbursements/projects/da-001"

	t.Run("401 không phiên", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		doiMa(t, m.goi(t, http.MethodGet, hostA, duong, nil), http.StatusUnauthorized)
		if m.duAn.goi != 0 {
			t.Fatal("chạm kho khi chưa có phiên")
		}
	})
	t.Run("403 sai quyền", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.confirm"))
		doiMa(t, m.goi(t, http.MethodGet, hostA, duong, canBoCua(xaA)), http.StatusForbidden)
		if m.duAn.goi != 0 {
			t.Fatal("chạm kho khi sai quyền")
		}
	})
	t.Run("từ chối khi đúng quyền nhưng sai xã", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		doiMa(t, m.goi(t, http.MethodGet, hostA, duong, canBoCua(xaB)), http.StatusUnauthorized)
		if m.duAn.goi != 0 {
			t.Fatal("chạm kho với token của xã khác")
		}
	})
	t.Run("200 đúng quyền đúng xã", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		w := m.goi(t, http.MethodGet, hostA, duong, canBoCua(xaA))
		doiMa(t, w, http.StatusOK)
		var ra duAnRa
		if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
			t.Fatalf("thân không phải JSON: %v", err)
		}
		if ra.Code != "DA-2026-be-tong-hoa-duong-ngo-xo-2" {
			t.Fatalf("mã dự án = %q", ra.Code)
		}
	})
}

// --- isolation --------------------------------------------------------------------------------

func TestChiTietDuAnKhongVoiSangXaKHACDuTrungID(t *testing.T) {
	// Commune B holds a project with the SAME id. A caller in commune B must get B's project, not
	// A's — and this is the only fixture shape that can show it.
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostB, "/api/v1/disbursements/projects/da-001", canBoCua(xaB))
	doiMa(t, w, http.StatusOK)

	var ra duAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if ra.Code != "DA-2026-CUA-XA-B" {
		t.Fatalf("mã dự án = %q — đọc sang dữ liệu của xã khác là rò rỉ giữa hai cơ quan", ra.Code)
	}
}

func TestDanhSachDuAnChiTraDuAnCuaXaTrongHost(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostB, duongDanDuAn, canBoCua(xaB))
	doiMa(t, w, http.StatusOK)

	var ra danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if len(ra.Items) != 1 || ra.Items[0].Code != "DA-2026-CUA-XA-B" {
		t.Fatalf("xã B nhận được %d dự án: %+v", len(ra.Items), ra.Items)
	}
}

// --- the derived figures ----------------------------------------------------------------------

func TestDanhSachDuAnSuyRaDiemChamKhongLuuCot(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	var ra danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}

	theoMa := map[string]duAnRa{}
	for _, mot := range ra.Items {
		theoMa[mot.Code] = mot
	}

	// 90% disbursed against 70,96% of the year elapsed: ahead, so delay score is negative and the
	// project is NOT flagged. The figure comes from the clock, not from any stored column.
	truoc := theoMa["DA-2026-be-tong-hoa-duong-ngo-xo-2"]
	if truoc.DisbursedRatio == nil || *truoc.DisbursedRatio != 9000 {
		t.Fatalf("tỷ lệ = %v, muốn 9000", truoc.DisbursedRatio)
	}
	if truoc.DelayScore == nil || *truoc.DelayScore != 7096-9000 {
		t.Fatalf("điểm chậm = %v, muốn %d", truoc.DelayScore, 7096-9000)
	}
	if truoc.IsDelayed {
		t.Fatal("dự án đi trước lịch bị đánh dấu chậm")
	}
	if truoc.RemainingAmount != 10_000_000 {
		t.Fatalf("còn lại = %d, muốn 10000000 (§8)", truoc.RemainingAmount)
	}
	// §9: a blank approved total reads as this year's plan.
	if truoc.ApprovedAmount != 100_000_000 {
		t.Fatalf("tổng mức được duyệt = %d, muốn 100000000", truoc.ApprovedAmount)
	}

	// Nothing disbursed: the delay score is exactly the elapsed share of the year (§3).
	cham := theoMa["DA-2026-nong-thon-moi"]
	if cham.DelayScore == nil || *cham.DelayScore != 7096 {
		t.Fatalf("điểm chậm = %v, muốn 7096", cham.DelayScore)
	}
	if !cham.IsDelayed {
		t.Fatal("chậm 70,96 điểm mà không bị đánh dấu chậm")
	}
}

// --- the budget year --------------------------------------------------------------------------

func TestDanhSachDuAnThieuNamThi400ChuKhongMacDinh(t *testing.T) {
	// A default would report another year's money under this year's heading, with every figure on
	// the page internally consistent and wrong (§13 rule 8).
	for _, duong := range []string{
		"/api/v1/disbursements/projects",
		"/api/v1/disbursements/projects?year=",
		"/api/v1/disbursements/projects?year=khong-phai-so",
		"/api/v1/disbursements/projects?year=12",
	} {
		t.Run(duong, func(t *testing.T) {
			m := dungMayChuVoi(t, coQuyen("budget.read"))
			w := m.goi(t, http.MethodGet, hostA, duong, canBoCua(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if m.duAn.goi != 0 {
				t.Fatal("năm sai mà vẫn chạy truy vấn — phải từ chối TRƯỚC khi chạm kho")
			}
		})
	}
}

// --- failures ---------------------------------------------------------------------------------

func TestDanhSachDuAnVuotTranThi500ChuKhongCat(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	m.duAn.loi = fistore.ErrQuaNhieuDuAn
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusInternalServerError)
}

func TestChiTietDuAnKhongCoThi404(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/disbursements/projects/khong-co", canBoCua(xaA))
	doiMa(t, w, http.StatusNotFound)
}

func TestChiTietDuAnLoiKhoThi500VaKhongLoRaNgoai(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	m.duAn.loi = errors.New("chi tiết kết nối kho: dsn=postgres://nguoi:matkhau@may-chu")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/disbursements/projects/da-001", canBoCua(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if than := w.Body.String(); len(than) > 0 && (contains(than, "matkhau") || contains(than, "dsn")) {
		t.Fatalf("chi tiết lỗi hệ thống lọt ra cho client: %s", than)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Register must refuse incomplete wiring at construction, not at request time: a route mounted
// without its store would accept requests it cannot honour, and the first person to find out would
// be a member of staff in front of a government screen.
func TestRegisterTuChoiKhiThieuKhoDuAn(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("thiếu kho dự án mà Register vẫn gắn tuyến")
		}
	}()
	Register(http.NewServeMux(), Deps{Checker: khongQuyen(), HangMuc: hangMucMau()})
}

func TestRegisterTuChoiKhiThieuChecker(t *testing.T) {
	// A nil Checker would make authz.RequirePermission meet a nil interface at request time —
	// the worst possible moment to find out.
	defer func() {
		if recover() == nil {
			t.Fatal("thiếu Checker mà Register vẫn gắn tuyến khai budget.read")
		}
	}()
	Register(http.NewServeMux(), Deps{HangMuc: hangMucMau(), DuAn: duAnMau()})
}

var _ authz.Checker = checkerGia{}
