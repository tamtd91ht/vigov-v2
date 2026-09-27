package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// commune_host_hint (ADR 0047) — one test per row of the status table in
// proto/vigov/identity/v1/citizen_session_bridge.proto. Fakes and the bench: cau_phien_cong_dan_test.go.

// INVALID_ARGUMENT for a malformed domain, in EVERY mode and whether confirmed or not — and before
// the platform is asked anything.
func TestCauTenMienSaiHinhDangLaInvalidArgumentKhongHoiNenTang(t *testing.T) {
	for _, host := range []string{
		"Xa-Thu.vigov.vn", "https://" + hostThu, hostThu + ":443", hostThu + "/x", hostThu + "?a=1",
		hostThu + "#x", "canbo@" + hostThu, hostThu + ".", "." + hostThu, "xa..vigov.vn", "current",
		"-xa.vigov.vn", "xa-.vigov.vn", "xa_thu.vigov.vn", "xã.vigov.vn", "10.0.0.1",
		strings.Repeat("a", 64) + ".vn",
	} {
		for _, app := range []string{cauAppChinh, cauAppRieng} {
			for _, xacNhan := range []bool{true, false} {
				b := dungBanThuCau(t)
				yc := yeuCauCau(app)
				yc.GoiYTenMien, yc.DaXacNhanXa = host, xacNhan
				if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauYeuCauSai) {
					t.Errorf("%q app=%s xác nhận=%v: err = %v, muốn ErrCauYeuCauSai", host, app, xacNhan, err)
				}
				if b.nt.goiApp != 0 || len(b.nt.goiHost) != 0 || b.g.soGiaoDich() != 0 {
					t.Errorf("%q: nền tảng bị hỏi (MiniApp %d, ResolveHost %v) hoặc đã mở giao dịch",
						host, b.nt.goiApp, b.nt.goiHost)
				}
			}
		}
	}
}

// Surrounding whitespace is trimmed — the one normalisation the contract allows.
func TestCauTenMienCatKhoangTrangHaiDau(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYTenMien, yc.DaXacNhanXa = "  "+hostThu+"\t", true
	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil || kq.Xa != xaThu {
		t.Fatalf("kq = %+v, err = %v", kq, err)
	}
	if len(b.nt.goiHost) != 1 || b.nt.goiHost[0] != hostThu {
		t.Fatalf("ResolveHost nhận %q, muốn đúng %q", b.nt.goiHost, hostThu)
	}
}

// tenant_hint is retired: non-empty is INVALID_ARGUMENT in every mode, alone or beside the domain.
func TestCauTenantHintDaNgungLuonBiTuChoi(t *testing.T) {
	for ten, sua := range map[string]func(*YeuCauMoPhienCau){
		"ULID, xác nhận":      func(y *YeuCauMoPhienCau) { y.GoiYXa, y.DaXacNhanXa = string(xaThu), true },
		"ULID, chưa xác nhận": func(y *YeuCauMoPhienCau) { y.GoiYXa = string(xaThu) },
		"ULID kèm tên miền hợp lệ": func(y *YeuCauMoPhienCau) {
			y.GoiYXa, y.GoiYTenMien, y.DaXacNhanXa = string(xaThu), hostThu, true
		},
		"không phải ULID":  func(y *YeuCauMoPhienCau) { y.GoiYXa = "thang-binh" },
		"chỉ khoảng trắng": func(y *YeuCauMoPhienCau) { y.GoiYXa = " " },
	} {
		for _, app := range []string{cauAppChinh, cauAppRieng} {
			t.Run(ten+"/"+app, func(t *testing.T) {
				b := dungBanThuCau(t)
				yc := yeuCauCau(app)
				sua(&yc)
				if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauYeuCauSai) {
					t.Fatalf("err = %v, muốn ErrCauYeuCauSai", err)
				}
				if b.nt.goiApp != 0 || len(b.nt.goiHost) != 0 || b.g.soGiaoDich() != 0 {
					t.Fatal("tenant_hint bị từ chối phải trước mọi lời gọi nền tảng và mọi ghi")
				}
			})
		}
	}
}

// Confirmed without a domain: INVALID_ARGUMENT in every mode, before the platform.
func TestCauXacNhanMaKhongCoTenMienLaInvalidArgumentMoiCheDo(t *testing.T) {
	for _, app := range []string{cauAppChinh, cauAppRieng} {
		b := dungBanThuCau(t)
		yc := yeuCauCau(app)
		yc.DaXacNhanXa = true
		if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauYeuCauSai) {
			t.Fatalf("app=%s: err = %v, muốn ErrCauYeuCauSai", app, err)
		}
		if b.nt.goiApp != 0 {
			t.Fatalf("app=%s: đã hỏi nền tảng", app)
		}
	}
}

// Unclaimed, platform-reserved and deactivated: ONE FAILED_PRECONDITION, byte for byte.
func TestCauTenMienKhongCoDanhRiengNgungMotCauTraLoi(t *testing.T) {
	var cacLoi []string
	for ten, chuanBi := range map[string]func(b banThuCau) string{
		"không xã nào giữ": func(banThuCau) string { return hostKhongCo },
		"của nền tảng":     func(banThuCau) string { return hostNenTang },
		"xã ngừng hoạt động": func(b banThuCau) string {
			b.nt.xa[xaKhac] = tenant.Tenant{ID: xaKhac, Name: "Xã Bình Dương", Active: false}
			return hostKhac
		},
	} {
		b := dungBanThuCau(t)
		yc := yeuCauCau(cauAppChinh)
		yc.GoiYTenMien, yc.DaXacNhanXa = chuanBi(b), true
		_, err := b.uc.Mo(context.Background(), yc)
		if !errors.Is(err, ErrCauXaKhongHoatDong) {
			t.Fatalf("%s: err = %v, muốn ErrCauXaKhongHoatDong (FAILED_PRECONDITION)", ten, err)
		}
		if b.g.soGiaoDich() != 0 || len(b.k.taiKhoan) != 0 {
			t.Fatalf("%s: đã ghi", ten)
		}
		cacLoi = append(cacLoi, err.Error())
	}
	for _, l := range cacLoi[1:] {
		if l != cacLoi[0] {
			t.Fatalf("ba trường hợp trả lời khác nhau: %q — bên gọi dò được tên miền nào tồn tại", cacLoi)
		}
	}
}

// ResolveHost unreachable: UNAVAILABLE, never FAILED_PRECONDITION, never the remembered commune.
func TestCauResolveHostHongLaUnavailableKhongPhaiKhongCoXa(t *testing.T) {
	b := dungBanThuCau(t)
	b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo] = crosstenant.TaiKhoanZalo{ID: "TK-A", XaDaNho: xaThu}
	b.nt.loiHost = errors.New("platform down")
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYTenMien, yc.DaXacNhanXa = hostKhac, true

	_, err := b.uc.Mo(context.Background(), yc)
	if !errors.Is(err, ErrCauNenTang) || errors.Is(err, ErrCauXaKhongHoatDong) {
		t.Fatalf("err = %v, muốn ErrCauNenTang (UNAVAILABLE) và KHÔNG phải ErrCauXaKhongHoatDong", err)
	}
	if b.g.soGiaoDich() != 0 || len(b.k.phien) != 0 {
		t.Fatal("ResolveHost hỏng mà vẫn ghi — hay dùng xã đã nhớ")
	}
}

// A dedicated app never resolves the domain — confirmed or not, even when it names another commune.
func TestCauAppRiengKhongBaoGioHoiResolveHost(t *testing.T) {
	for _, xacNhan := range []bool{true, false} {
		b := dungBanThuCau(t)
		b.nt.loiHost = errors.New("không được gọi")
		yc := yeuCauCau(cauAppRieng)
		yc.GoiYTenMien, yc.DaXacNhanXa = hostKhac, xacNhan
		kq, err := b.uc.Mo(context.Background(), yc)
		if err != nil || kq.Xa != xaThu {
			t.Fatalf("xác nhận=%v: kq = %+v, err = %v — app riêng theo xã của app", xacNhan, kq, err)
		}
		if len(b.nt.goiHost) != 0 {
			t.Fatalf("xác nhận=%v: app riêng đã hỏi ResolveHost %v", xacNhan, b.nt.goiHost)
		}
	}
}

// Main app, not confirmed: the domain is ignored, and ignored means not even resolved.
func TestCauAppChinhChuaXacNhanKhongHoiResolveHost(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYTenMien = hostThu
	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil || kq.Xa != "" || kq.Token != "" {
		t.Fatalf("kq = %+v, err = %v — chưa xác nhận, chưa nhớ gì thì không xã", kq, err)
	}
	if len(b.nt.goiHost) != 0 {
		t.Fatalf("chưa xác nhận mà đã hỏi ResolveHost %v", b.nt.goiHost)
	}
}

// Success: the session, the remembered commune and every audit entry carry the RESOLVED ULID; the
// domain itself reaches no stored row and no audit entry (rule 1, forbidden #5; ADR 0047).
func TestCauTenMienThanhULIDVaKhongDuocLuuODauCa(t *testing.T) {
	b := dungBanThuCau(t)
	// First into xaThu, then switch to xaKhac by its domain — so doi_xa_da_nho is written too.
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYTenMien, yc.DaXacNhanXa, yc.SoDaXacThuc = hostThu, true, "84900000000"
	if _, err := b.uc.Mo(context.Background(), yc); err != nil {
		t.Fatal(err)
	}
	yc.GoiYTenMien = hostKhac
	kq, err := b.uc.Mo(context.Background(), yc)
	if err != nil {
		t.Fatal(err)
	}
	if kq.Xa != xaKhac || kq.TenXa != "Xã Bình Dương" || kq.Token == "" {
		t.Fatalf("kq = %+v, muốn xã %s", kq, xaKhac)
	}
	if got := b.nt.goiHost; len(got) != 2 || got[0] != hostThu || got[1] != hostKhac {
		t.Fatalf("ResolveHost được hỏi %v", got)
	}
	if b.k.taiKhoan[cauAppChinh+"|"+cauMaZalo].XaDaNho != xaKhac {
		t.Fatal("xã đã nhớ không phải ULID vừa phân giải")
	}
	if b.k.phien[1].xa != string(xaKhac) {
		t.Fatalf("phiên ghi ở xã %q", b.k.phien[1].xa)
	}
	if len(b.vet(HanhDongMoPhienCongDan)) != 2 || len(b.vet(HanhDongDoiXaDaNho)) != 2 {
		t.Fatal("thiếu mục vết mở phiên / đổi xã")
	}

	for _, host := range []string{hostThu, hostKhac, "vigov.vn"} {
		for _, l := range b.g.lenh {
			for _, a := range l.args {
				switch v := a.(type) {
				case string:
					if strings.Contains(v, host) {
						t.Fatalf("tên miền %q lọt vào tham số ghi CSDL: %q (%s)", host, v, l.sql)
					}
				case []byte:
					if bytes.Contains(v, []byte(host)) {
						t.Fatalf("tên miền %q lọt vào nội dung vết: %s", host, v)
					}
				}
			}
		}
		for _, p := range b.k.phien {
			if strings.Contains(p.xa+p.sid+p.taiKhoan+p.congDan, host) {
				t.Fatalf("tên miền %q lọt vào phiên: %+v", host, p)
			}
		}
		for _, tk := range b.k.taiKhoan {
			if strings.Contains(string(tk.XaDaNho)+tk.ID+tk.CongDanID, host) {
				t.Fatalf("tên miền %q lọt vào tài khoản Zalo: %+v", host, tk)
			}
		}
	}
}

// ResolveHost says active, GetTenant (read again with the commune in context) says not: refused.
// The re-check stays — it is the read the transaction's commune is opened under.
func TestCauResolveHostHoatDongNhungGetTenantNgungThiTuChoi(t *testing.T) {
	b := dungBanThuCau(t)
	b.nt.xaSauHost = &tenant.Tenant{ID: xaKhac, Active: false} // deactivated between the two reads
	yc := yeuCauCau(cauAppChinh)
	yc.GoiYTenMien, yc.DaXacNhanXa = hostKhac, true
	if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauXaKhongHoatDong) {
		t.Fatalf("err = %v, muốn ErrCauXaKhongHoatDong", err)
	}
	if b.g.soGiaoDich() != 0 {
		t.Fatal("đã mở giao dịch cho xã vừa ngừng")
	}
}
