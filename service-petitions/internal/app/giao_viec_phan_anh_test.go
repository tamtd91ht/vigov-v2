package app

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The assignee check on PhanCong (user decision 2026-09-25): a client-supplied officer code is
// verified with identity.ResolveAssignableStaff BEFORE the transaction opens.
//
//	PROVED HERE   an assignable code proceeds to the write · an absent code is refused with ONE error,
//	              before any statement runs, no transaction opened · an identity failure is refused
//	              as "not checked" (never "not assignable", never written unchecked) · a unit-only
//	              assignment asks identity nothing · the call carries the commune from the context ·
//	              missing wiring refuses rather than writing unchecked.
//
//	NOT PROVED    what "assignable" means — that predicate is identity's and is proved in
//	              service-identity/internal/store/can_bo_giao_viec_test.go.

// giaoViecGia is identity's answer, recording what it was asked and IN WHICH COMMUNE.
type giaoViecGia struct {
	duocTatCa bool                // answer every asked code as assignable
	duoc      map[string]struct{} // otherwise: exactly these
	loi       error

	goi   int
	daHoi [][]string
	xa    []tenant.ID
}

func (g *giaoViecGia) CanBoGiaoViecDuoc(ctx context.Context, ma []string) (map[string]struct{}, error) {
	g.goi++
	g.daHoi = append(g.daHoi, append([]string(nil), ma...))
	xa, _ := tenant.From(ctx)
	g.xa = append(g.xa, xa)
	if g.loi != nil {
		return nil, g.loi
	}
	ra := map[string]struct{}{}
	for _, m := range ma {
		if _, co := g.duoc[m]; co || g.duocTatCa {
			ra[m] = struct{}{}
		}
	}
	return ra, nil
}

// phieuChoPhanCong is a petition that may be assigned (classified, deadline fixed).
func phieuChoPhanCong(k *khoPhieuXuLyGia) {
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DangPhanLoai), "linh_vuc": "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac,
	})
}

// Hợp lệ → đi tiếp tới phần ghi, và identity được hỏi ĐÚNG mã đó, MỘT lần, TRONG xã của context.
func TestPhanCongCanBoGiaoDuocThiGhi(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	gv := &giaoViecGia{duoc: map[string]struct{}{"CB-00999": {}}}
	uc.giaoViec = gv

	sau, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: " CB-00999 "},
		canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("PhanCong: %v", err)
	}
	if sau.CanBoXuLyID != "CB-00999" {
		t.Errorf("cán bộ xử lý = %q", sau.CanBoXuLyID)
	}
	if gv.goi != 1 || len(gv.daHoi[0]) != 1 || gv.daHoi[0][0] != "CB-00999" {
		t.Fatalf("identity được hỏi %v (%d lần), muốn đúng [CB-00999] một lần — mã đã cắt khoảng trắng", gv.daHoi, gv.goi)
	}
	// THE COMMUNE COMES FROM THE CONTEXT — core/identityclient lifts it into "x-tenant-id", which is
	// what makes another commune's code absent.
	if gv.xa[0] != xaThu {
		t.Errorf("identity được hỏi trong xã %q, muốn %q", gv.xa[0], xaThu)
	}
	if !k.coCau("UPDATE phieu_phan_anh") || k.daCommit != 1 {
		t.Error("mã giao được mà không ghi phân công")
	}
}

// Vắng mặt → ErrCanBoKhongNhanDuocViec, KHÔNG câu lệnh nào chạy, KHÔNG mở giao dịch.
func TestPhanCongCanBoKhongGiaoDuocThiTuChoiTruocMoiThu(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.giaoViec = &giaoViecGia{duoc: map[string]struct{}{}}

	_, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: "CB-BI-KHOA"},
		canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, ErrCanBoKhongNhanDuocViec) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoKhongNhanDuocViec", err)
	}
	if errors.Is(err, ErrChuaKiemDuocCanBo) {
		t.Error("\"không giao được\" lẫn với \"chưa kiểm được\" — hai câu trả lời khác nhau (400 vs 503)")
	}
	if n := len(k.cau("")); n != 0 {
		t.Errorf("đã chạy %d câu lệnh dù từ chối", n)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch dù từ chối", k.batDau)
	}
}

// Identity hỏng → ErrChuaKiemDuocCanBo (503), KHÔNG ghi gì, KHÔNG BAO GIỜ ghi mã chưa kiểm.
func TestPhanCongIdentityHongThiKhongGhiMaChuaKiem(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.giaoViec = &giaoViecGia{loi: errors.New("Unavailable: identity không tới được")}

	_, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: "CB-00999"},
		canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, ErrChuaKiemDuocCanBo) {
		t.Fatalf("lỗi = %v, muốn ErrChuaKiemDuocCanBo", err)
	}
	if errors.Is(err, ErrCanBoKhongNhanDuocViec) {
		t.Error("sự cố identity bị báo thành \"cán bộ không nhận được việc\"")
	}
	if len(k.cau("")) != 0 || k.batDau != 0 {
		t.Errorf("đã chạm cơ sở dữ liệu dù chưa kiểm được: %d câu, %d giao dịch", len(k.cau("")), k.batDau)
	}
}

// Chỉ bộ phận → KHÔNG hỏi identity, và vẫn ghi.
func TestPhanCongChiBoPhanKhongHoiIdentity(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	gv := &giaoViecGia{loi: errors.New("không được gọi")}
	uc.giaoViec = gv

	if _, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("PhanCong chỉ bộ phận: %v", err)
	}
	if gv.goi != 0 {
		t.Errorf("hỏi identity %d lần cho phân công chỉ bộ phận — không có mã nào để kiểm", gv.goi)
	}
}

// Nối dây thiếu → từ chối như "chưa kiểm được", không bao giờ ghi mã chưa kiểm.
func TestPhanCongThieuNoiDayKiemCanBoThiTuChoi(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.giaoViec = nil

	_, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: "CB-00999"},
		canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, ErrChuaKiemDuocCanBo) {
		t.Fatalf("lỗi = %v, muốn ErrChuaKiemDuocCanBo", err)
	}
	if k.coCau("UPDATE") || k.batDau != 0 {
		t.Error("ghi phân công khi không có gì kiểm mã cán bộ")
	}
}
