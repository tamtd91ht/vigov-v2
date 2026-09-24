package http

// The stand-ins for the two document registers, used by van_ban_test.go.
//
// EVERY ONE OF THEM RECORDS THE COMMUNE IT WAS CALLED IN, read from the context exactly as
// *store.Scoped reads it. A fake that ignored the commune would let the "right permission, wrong
// commune" case of rule 5, invariant 7 pass while proving nothing at all — which is the trap
// loaiVanBanGia avoids by keying its rows by commune, and the one that makes that case the easiest
// of the four to fake.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// vanBanDenGia is the incoming register's READ half, KEYED BY COMMUNE.
type vanBanDenGia struct {
	theo map[tenant.ID][]domain.VanBanDen
	loi  error

	goi    int
	locCuo docstore.LocVanBanDen
	xaCuoi tenant.ID
}

func (k *vanBanDenGia) DanhSach(ctx context.Context, loc docstore.LocVanBanDen,
	_ page.Request) (page.Result[domain.VanBanDen], error) {

	k.goi++
	k.locCuo = loc
	k.xaCuoi = tenant.MustFrom(ctx)
	if k.loi != nil {
		return page.Result[domain.VanBanDen]{}, k.loi
	}
	return page.Result[domain.VanBanDen]{Items: k.theo[tenant.MustFrom(ctx)]}, nil
}

// ghiVanBanDenGia stands in for the incoming register's write use case.
type ghiVanBanDenGia struct {
	ra  domain.VanBanDen
	loi error

	themGoi, suaGoi, goGoi, chuyenGoi int
	xaCuoi                            tenant.ID
	nguoiCuoi                         audit.Actor
	themCuoi                          app.YeuCauVaoSoVanBanDen
	suaCuoi                           app.YeuCauSuaVanBanDen
	chuyenCuoi                        app.YeuCauChuyenVanBan
	idCuoi, lyDoCuoi                  string
}

func (g *ghiVanBanDenGia) ghiNhan(ctx context.Context, nguoi audit.Actor) {
	g.xaCuoi = tenant.MustFrom(ctx)
	g.nguoiCuoi = nguoi
}

func (g *ghiVanBanDenGia) Them(ctx context.Context, yc app.YeuCauVaoSoVanBanDen,
	nguoi audit.Actor) (domain.VanBanDen, error) {
	g.themGoi++
	g.themCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.VanBanDen{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiVanBanDenGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaVanBanDen,
	nguoi audit.Actor) (domain.VanBanDen, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.VanBanDen{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiVanBanDenGia) Go(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.goGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiVanBanDenGia) Chuyen(ctx context.Context, id string, yc app.YeuCauChuyenVanBan,
	nguoi audit.Actor) (domain.VanBanDen, error) {
	g.chuyenGoi++
	g.idCuoi, g.chuyenCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.VanBanDen{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiVanBanDenGia) tongGoi() int {
	return g.themGoi + g.suaGoi + g.goGoi + g.chuyenGoi
}

// chiTietVanBanDenGia stands in for the detail drawer's two reads, KEYED BY COMMUNE.
//
// IT MODELS WHAT THE STORE'S PREDICATE DOES, so the handler's 404 is exercised on all three causes:
// a row of another commune is simply absent from this commune's map, and a row in `daGo` is present
// but removed — both return docstore.ErrKhongThayVanBanDen, exactly as `tenant_id = $1 AND
// deleted_at IS NULL` would. The SQL itself is asserted in the app tests.
type chiTietVanBanDenGia struct {
	theo   map[tenant.ID][]domain.VanBanDen
	daGo   map[string]bool
	lichSu map[string][]domain.ChuyenVanBan // by document id; stored oldest first
	loi    error

	chiTietGoi, lichSuGoi int
	xaCuoi                tenant.ID
}

func (k *chiTietVanBanDenGia) tim(ctx context.Context, id string) (domain.VanBanDen, error) {
	k.xaCuoi = tenant.MustFrom(ctx)
	if k.loi != nil {
		return domain.VanBanDen{}, k.loi
	}
	for _, v := range k.theo[tenant.MustFrom(ctx)] {
		if v.ID == id && !k.daGo[id] {
			return v, nil
		}
	}
	return domain.VanBanDen{}, docstore.ErrKhongThayVanBanDen
}

func (k *chiTietVanBanDenGia) ChiTiet(ctx context.Context, id string) (domain.VanBanDen, error) {
	k.chiTietGoi++
	return k.tim(ctx, id)
}

func (k *chiTietVanBanDenGia) LichSuChuyen(ctx context.Context, id string) ([]domain.ChuyenVanBan, error) {
	k.lichSuGoi++
	v, err := k.tim(ctx, id)
	if err != nil {
		return nil, err
	}
	return k.lichSu[v.ID], nil
}

func (k *chiTietVanBanDenGia) tongGoi() int { return k.chiTietGoi + k.lichSuGoi }

func chiTietVanBanDenMau() *chiTietVanBanDenGia {
	return &chiTietVanBanDenGia{
		theo: vanBanDenMau().theo,
		daGo: map[string]bool{},
		lichSu: map[string][]domain.ChuyenVanBan{
			"vbd-a-001": {
				{ID: "ls-1", VanBanDenID: "vbd-a-001", ThoiDiem: time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC),
					NguoiMa: "CB-00001", TrangThaiTaiThoiDiem: domain.VanBanDaPhanCong,
					DenBoPhan: "bp-van-phong", NoiDung: "Chuyển văn phòng xem xét"},
				{ID: "ls-2", VanBanDenID: "vbd-a-001", ThoiDiem: time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC),
					NguoiMa: "CB-00002", TrangThaiTaiThoiDiem: domain.VanBanDangXuLy,
					TuBoPhan: "bp-van-phong", DenBoPhan: "bp-dia-chinh", CanBoXuLyMa: "CB-00003",
					NoiDung: "Thuộc thẩm quyền bộ phận Địa chính"},
			},
		},
	}
}

// vanBanDiGia is the outgoing register's READ half, KEYED BY COMMUNE.
type vanBanDiGia struct {
	theo map[tenant.ID][]domain.VanBanDi
	loi  error

	goi    int
	locCuo docstore.LocVanBanDi
	xaCuoi tenant.ID
}

func (k *vanBanDiGia) DanhSach(ctx context.Context, loc docstore.LocVanBanDi,
	_ page.Request) (page.Result[domain.VanBanDi], error) {

	k.goi++
	k.locCuo = loc
	k.xaCuoi = tenant.MustFrom(ctx)
	if k.loi != nil {
		return page.Result[domain.VanBanDi]{}, k.loi
	}
	return page.Result[domain.VanBanDi]{Items: k.theo[tenant.MustFrom(ctx)]}, nil
}

// ghiVanBanDiGia stands in for the outgoing register's write use case.
type ghiVanBanDiGia struct {
	ra  domain.VanBanDi
	loi error

	capSoGoi, suaGoi, goGoi int
	xaCuoi                  tenant.ID
	nguoiCuoi               audit.Actor
	capSoCuoi               app.YeuCauCapSoVanBanDi
	suaCuoi                 app.YeuCauSuaVanBanDi
	idCuoi, lyDoCuoi        string
}

func (g *ghiVanBanDiGia) ghiNhan(ctx context.Context, nguoi audit.Actor) {
	g.xaCuoi = tenant.MustFrom(ctx)
	g.nguoiCuoi = nguoi
}

func (g *ghiVanBanDiGia) CapSo(ctx context.Context, yc app.YeuCauCapSoVanBanDi,
	nguoi audit.Actor) (domain.VanBanDi, error) {
	g.capSoGoi++
	g.capSoCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.VanBanDi{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiVanBanDiGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaVanBanDi,
	nguoi audit.Actor) (domain.VanBanDi, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.VanBanDi{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiVanBanDiGia) Go(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.goGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiVanBanDiGia) tongGoi() int { return g.capSoGoi + g.suaGoi + g.goGoi }

// The fixtures. TWO COMMUNES WITH DIFFERENT DATA, because two communes whose registers looked the
// same could not show a leak.
func vanBanDenMau() *vanBanDenGia {
	return &vanBanDenGia{theo: map[tenant.ID][]domain.VanBanDen{
		xaA: {
			{ID: "vbd-a-002", SoVaoSo: 2, Nam: 2026, CoQuanBanHanh: "Huyện uỷ",
				LoaiVanBan: "cong-van", TrichYeu: "Về việc rà soát hộ nghèo",
				TrangThai: domain.VanBanMoiVaoSo},
			{ID: "vbd-a-001", SoVaoSo: 1, Nam: 2026, CoQuanBanHanh: "UBND huyện",
				LoaiVanBan: "quyet-dinh", TrichYeu: "Quyết định giao dự toán",
				TrangThai: domain.VanBanDaPhanCong},
		},
		xaB: {
			{ID: "vbd-b-001", SoVaoSo: 1, Nam: 2026, CoQuanBanHanh: "Sở Nội vụ xã B",
				LoaiVanBan: "thong-bao", TrichYeu: "Thông báo của xã B",
				TrangThai: domain.VanBanMoiVaoSo},
		},
	}}
}

func vanBanDiMau() *vanBanDiGia {
	return &vanBanDiGia{theo: map[tenant.ID][]domain.VanBanDi{
		xaA: {
			{ID: "vbdi-a-001", SoDi: 1, Nam: 2026, LoaiVanBan: "cong-van",
				TrichYeu: "Trả lời đơn của công dân", NoiNhan: "UBND huyện"},
		},
		xaB: {
			{ID: "vbdi-b-001", SoDi: 1, Nam: 2026, LoaiVanBan: "bao-cao",
				TrichYeu: "Báo cáo của xã B", NoiNhan: "Huyện uỷ"},
		},
	}}
}

// --- the idempotency store ------------------------------------------------------------------------

// khoIdemGia is idem.Store in a map.
//
// WHY THE REGISTER TESTS NEED A REAL ONE WHILE THE CATALOGUE TESTS DO NOT: the two routes that
// ISSUE A NUMBER declare idem.Required(idem.DongKhiHong), and a nil store is an UNREACHABLE store —
// which that declaration answers with 503, by design. With nil, every assertion about those routes
// would be an assertion about a missing cache.
//
// TTLs are ignored: nothing in these tests waits.
type khoIdemGia struct {
	mu  sync.Mutex
	gia map[string]string
}

func khoIdemMau() *khoIdemGia { return &khoIdemGia{gia: map[string]string{}} }

func (k *khoIdemGia) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, co := k.gia[key]; co {
		return false, nil
	}
	k.gia[key] = "1"
	return true, nil
}

func (k *khoIdemGia) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.gia[key], nil
}

func (k *khoIdemGia) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.gia[key] = value
	return nil
}

func (k *khoIdemGia) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.gia, key)
	return nil
}

// dungMayChuCoIdem is dungMayChu with a working idempotency store behind it.
func dungMayChuCoIdem(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.khoIdem = khoIdemMau()
	m.dungLai(t, nil)
	return m
}
