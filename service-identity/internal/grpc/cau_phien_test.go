package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

type moPhienCauGia struct {
	kq   app.KetQuaMoPhienCau
	loi  error
	nhan app.YeuCauMoPhienCau
}

func (m *moPhienCauGia) Mo(_ context.Context, yc app.YeuCauMoPhienCau) (app.KetQuaMoPhienCau, error) {
	m.nhan = yc
	return m.kq, m.loi
}

func TestCauServerChuyenDuTruongVaoUseCase(t *testing.T) {
	gia := &moPhienCauGia{kq: app.KetQuaMoPhienCau{Token: "tok", Sid: "sid", HetHan: time.Now().Add(time.Hour),
		Xa: tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EA"), TenXa: "Xã Thăng Bình", DaCoSo: true, CheDo: domain.CheDoAppRieng}}
	s := NewCauServer(gia, nil)

	ra, err := s.OpenCitizenSession(context.Background(), &identityv1.OpenCitizenSessionRequest{
		AppId: "a", ZaloUserId: "z", TenantHint: "t", CommuneConfirmed: true,
		VerifiedPhone: "84900000000", ClientIp: "10.0.0.1", Device: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if gia.nhan != (app.YeuCauMoPhienCau{AppID: "a", MaZalo: "z", GoiYXa: "t", DaXacNhanXa: true,
		SoDaXacThuc: "84900000000", IP: "10.0.0.1", ThietBi: "d"}) {
		t.Fatalf("use case nhận = %+v", gia.nhan)
	}
	if ra.GetSessionToken() != "tok" || ra.GetSessionId() != "sid" || ra.GetExpiresAt() == nil ||
		ra.GetTenantId() != "01JD8ZQK9M3NPXR7TVWYB2C4EA" || ra.GetTenantDisplayName() != "Xã Thăng Bình" ||
		!ra.GetPhoneVerified() || ra.GetAppMode() != identityv1.MiniAppMode_MINI_APP_MODE_COMMUNE {
		t.Fatalf("phản hồi = %v", ra)
	}
}

func TestCauServerKhongXaThiKhongTokenKhongHan(t *testing.T) {
	s := NewCauServer(&moPhienCauGia{kq: app.KetQuaMoPhienCau{CheDo: domain.CheDoAppChinh}}, nil)
	ra, err := s.OpenCitizenSession(context.Background(), &identityv1.OpenCitizenSessionRequest{AppId: "a", ZaloUserId: "z"})
	if err != nil {
		t.Fatal(err)
	}
	if ra.GetSessionToken() != "" || ra.GetTenantId() != "" || ra.GetExpiresAt() != nil ||
		ra.GetAppMode() != identityv1.MiniAppMode_MINI_APP_MODE_MAIN {
		t.Fatalf("phản hồi không xã = %v", ra)
	}
}

func TestCauServerMaLoiTheoBangTrangThai(t *testing.T) {
	for loi, ma := range map[error]codes.Code{
		fmt.Errorf("x: %w", app.ErrCauYeuCauSai):       codes.InvalidArgument,
		fmt.Errorf("x: %w", app.ErrCauAppChuaSanSang):  codes.FailedPrecondition,
		fmt.Errorf("x: %w", app.ErrCauXaKhongHoatDong): codes.FailedPrecondition,
		fmt.Errorf("x: %w", app.ErrCauNenTang):         codes.Unavailable,
		fmt.Errorf("x: %w", app.ErrCauXungDot):         codes.Aborted,
		errors.New("ổ đĩa đầy"):                        codes.Internal,
	} {
		var buf bytes.Buffer
		s := NewCauServer(&moPhienCauGia{loi: loi}, slog.New(slog.NewTextHandler(&buf, nil)))
		_, err := s.OpenCitizenSession(context.Background(), &identityv1.OpenCitizenSessionRequest{})
		if status.Code(err) != ma {
			t.Errorf("%v → %v, muốn %v", loi, status.Code(err), ma)
		}
		// The cause never crosses the boundary.
		if strings.Contains(status.Convert(err).Message(), "ổ đĩa") {
			t.Errorf("thông điệp trả về mang nguyên nhân nội bộ: %q", status.Convert(err).Message())
		}
	}
}

func TestCauServerKhongDungDuocKhiThieuUseCase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng CauServer không có use case mà không panic")
		}
	}()
	NewCauServer(nil, nil)
}
