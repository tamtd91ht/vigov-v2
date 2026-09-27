package app

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/store"
)

// The assigner check on task creation (owner decision 2026-09-27): "Lãnh đạo giao việc" is verified
// with identity.ResolveAssignableStaff BEFORE the transaction opens, in GhiNhiemVu.TaoTuNguon — the one
// function both doors reach (Tao for POST /api/v1/tasks; TaoTuNguon with a source check for the
// meeting-conclusion split, which delegates there: bien_ban_hop.go TachKetLuanThanhNhiemVu).
//
//	PROVED HERE   a valid code is written · unknown / inactive / other-commune are ONE refusal with
//	              no transaction opened and nothing written · identity down and missing wiring both
//	              refuse as "not checked" with nothing written · an empty assigner asks nothing ·
//	              the call carries the commune from the context · the same on the split's door.
//
//	NOT PROVED    what "active staff" means — identity's predicate, proved in
//	              service-identity/internal/store/can_bo_giao_viec_test.go. And task.extend is NOT
//	              checked here by the owner's decision; nothing below asserts it either way.

const maLanhDaoThu = "CB-00007"

func taoCoLanhDao() YeuCauTaoNhiemVu {
	yc := taoMau()
	yc.LanhDaoGiaoViecMa = maLanhDaoThu
	return yc
}

// khongMoGiaoDich asserts the refusal left the register untouched (khongGhiGi, nhiem_vu_test.go) AND
// came BEFORE any transaction: the check exists to run outside one.
func khongMoGiaoDich(t *testing.T, k *khoNhiemVuGia) {
	t.Helper()
	khongGhiGi(t, k)
	if k.batDau != 0 || len(k.lenh) != 0 {
		t.Errorf("mở %d giao dịch, chạy %d câu lệnh — từ chối phải xảy ra TRƯỚC giao dịch", k.batDau, len(k.lenh))
	}
}

// Hợp lệ → tạo, và identity được hỏi ĐÚNG mã đó, MỘT lần, TRONG xã của context.
func TestTaoNhiemVu_LanhDaoHopLeThiTao(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	gv := &giaoViecGia{duoc: map[string]struct{}{maLanhDaoThu: {}}}
	uc.giaoViec = gv

	n, err := uc.Tao(ctx, taoCoLanhDao(), canBoThu())
	if err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}
	if n.LanhDaoGiaoViecMa != maLanhDaoThu {
		t.Errorf("lãnh đạo giao việc = %q, muốn %q", n.LanhDaoGiaoViecMa, maLanhDaoThu)
	}
	if gv.goi != 1 || len(gv.daHoi[0]) != 1 || gv.daHoi[0][0] != maLanhDaoThu {
		t.Fatalf("identity được hỏi %v (%d lần), muốn đúng [%s] một lần", gv.daHoi, gv.goi, maLanhDaoThu)
	}
	if gv.xa[0] != xaThu {
		t.Errorf("identity được hỏi trong xã %q, muốn %q", gv.xa[0], xaThu)
	}
	if !k.coCau("INSERT INTO nhiem_vu") || !k.coCau("INSERT INTO audit_log") || k.daCommit != 1 {
		t.Error("mã hợp lệ mà không ghi nhiệm vụ cùng dấu vết")
	}
}

// Không có trong tập trả về (không tồn tại · nghỉ việc/khoá · xã khác) → MỘT lỗi, không ghi gì.
func TestTaoNhiemVu_LanhDaoKhongHopLeThiTuChoiTruocMoiThu(t *testing.T) {
	// THREE NAMES FOR ONE ANSWER: identity collapses the reasons into "absent", so the fake answers
	// each the same way — and the assertion is that this layer produces the SAME error for all.
	for _, ten := range []string{"không tồn tại", "không còn hoạt động", "thuộc xã khác"} {
		t.Run(ten, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			gv := &giaoViecGia{duoc: map[string]struct{}{"CB-00999": {}}}
			uc.giaoViec = gv

			_, err := uc.Tao(ctx, taoCoLanhDao(), canBoThu())
			if !errors.Is(err, ErrLanhDaoGiaoViecKhongHopLe) {
				t.Fatalf("lỗi = %v, muốn ErrLanhDaoGiaoViecKhongHopLe", err)
			}
			if errors.Is(err, ErrChuaKiemDuocLanhDaoGiaoViec) {
				t.Error("mã bị từ chối lại mang cả nghĩa 'chưa kiểm được'")
			}
			khongMoGiaoDich(t, k)
		})
	}
}

// identity không trả lời → "chưa kiểm được" (503), KHÔNG phải "không hợp lệ", không ghi gì.
func TestTaoNhiemVu_IdentityHongThiChuaKiemDuocVaKhongGhi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{loi: errors.New("rpc error: code = Unavailable")}

	_, err := uc.Tao(ctx, taoCoLanhDao(), canBoThu())
	if !errors.Is(err, ErrChuaKiemDuocLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrChuaKiemDuocLanhDaoGiaoViec", err)
	}
	if errors.Is(err, ErrLanhDaoGiaoViecKhongHopLe) {
		t.Error("identity hỏng bị báo thành 'lãnh đạo không hợp lệ' — người dùng sẽ đi chọn người khác vô ích")
	}
	khongMoGiaoDich(t, k)
}

// Chưa nối dây phép kiểm → từ chối (fail closed), không bao giờ ghi mã chưa kiểm.
func TestTaoNhiemVu_ChuaNoiDayKiemThiTuChoi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = nil

	_, err := uc.Tao(ctx, taoCoLanhDao(), canBoThu())
	if !errors.Is(err, ErrChuaKiemDuocLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrChuaKiemDuocLanhDaoGiaoViec", err)
	}
	khongMoGiaoDich(t, k)
}

// Không ghi lãnh đạo giao việc → không hỏi identity, vẫn tạo được (ADR 0038 chặn ở lúc duyệt), và
// KHÔNG lấy người tạo thay vào.
func TestTaoNhiemVu_KhongCoLanhDaoThiKhongHoiIdentity(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	gv := &giaoViecGia{}
	uc.giaoViec = gv

	n, err := uc.Tao(ctx, taoMau(), canBoThu())
	if err != nil {
		t.Fatalf("giao việc mới không có lãnh đạo: %v", err)
	}
	if gv.goi != 0 {
		t.Errorf("identity bị hỏi %d lần khi không có mã lãnh đạo nào", gv.goi)
	}
	if n.LanhDaoGiaoViecMa != "" {
		t.Errorf("lãnh đạo giao việc = %q — không được lấy người tạo thay vào", n.LanhDaoGiaoViecMa)
	}
	if k.daCommit != 1 {
		t.Error("không có lãnh đạo mà không tạo được — việc chặn thuộc về lúc duyệt (ADR 0038)")
	}
}

// CỬA THỨ HAI — tách kết luận họp đi qua TaoTuNguon có phép kiểm nguồn. Mã bị từ chối thì phép kiểm
// nguồn cũng KHÔNG chạy, vì chưa có giao dịch nào để nó chạy trong đó.
func TestTaoTuNguon_LanhDaoKhongHopLeThiTuChoiTruocKiemNguon(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{}

	var daKiemNguon bool
	_, err := uc.TaoTuNguon(ctx, taoCoLanhDao(), canBoThu(), func(context.Context, *store.ScopedTx) error {
		daKiemNguon = true
		return nil
	})
	if !errors.Is(err, ErrLanhDaoGiaoViecKhongHopLe) {
		t.Fatalf("lỗi = %v, muốn ErrLanhDaoGiaoViecKhongHopLe", err)
	}
	if daKiemNguon {
		t.Error("phép kiểm nguồn đã chạy dù lãnh đạo giao việc bị từ chối")
	}
	khongMoGiaoDich(t, k)
}

func TestTaoTuNguon_IdentityHongThiChuaKiemDuocVaKhongGhi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{loi: errors.New("rpc error: code = DeadlineExceeded")}

	_, err := uc.TaoTuNguon(ctx, taoCoLanhDao(), canBoThu(), func(context.Context, *store.ScopedTx) error {
		return nil
	})
	if !errors.Is(err, ErrChuaKiemDuocLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrChuaKiemDuocLanhDaoGiaoViec", err)
	}
	khongMoGiaoDich(t, k)
}

func TestTaoTuNguon_LanhDaoHopLeThiTao(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	gv := &giaoViecGia{duocTatCa: true}
	uc.giaoViec = gv

	_, err := uc.TaoTuNguon(ctx, taoCoLanhDao(), canBoThu(), func(context.Context, *store.ScopedTx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("tách kết luận có lãnh đạo hợp lệ: %v", err)
	}
	if gv.goi != 1 || gv.xa[0] != xaThu {
		t.Errorf("identity được hỏi %d lần trong xã %v, muốn 1 lần trong %q", gv.goi, gv.xa, xaThu)
	}
	if !k.coCau("INSERT INTO nhiem_vu") || k.daCommit != 1 {
		t.Error("mã hợp lệ mà không ghi nhiệm vụ")
	}
}
