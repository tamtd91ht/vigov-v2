package platformclient

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

const (
	hostThu = "thangbinh.example.gov.vn"
	ulidThu = "01JD8ZQK9M3NPXR7TVWYB2C4EF"
)

// nenTangGia is the platform service, absent. Injecting the generated client interface is what
// lets the failure behaviour be tested with no process to start — and the failure behaviour is
// the whole reason this file exists.
type nenTangGia struct {
	platformv1.PlatformServiceClient
	ra  *platformv1.ResolveHostResponse
	loi error
	goi int
}

func (n *nenTangGia) ResolveHost(_ context.Context, _ *platformv1.ResolveHostRequest,
	_ ...grpc.CallOption) (*platformv1.ResolveHostResponse, error) {
	n.goi++
	return n.ra, n.loi
}

func dungThu(t *testing.T, gia *nenTangGia) (*Directory, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewDirectory(gia, log), &buf
}

func TestPhanGiaiDuocThiTraVeXa(t *testing.T) {
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{
		Tenant: &platformv1.Tenant{
			Id: ulidThu, Host: hostThu, DisplayName: "Xã Thăng Bình", Active: true,
			Province: "Thành phố Đà Nẵng",
		},
	}})

	got, ok := d.ByHost(context.Background(), hostThu)
	if !ok {
		t.Fatal("host hợp lệ mà không phân giải được")
	}
	if got.ID != tenant.ID(ulidThu) || got.Host != hostThu || !got.Active {
		t.Errorf("xã trả về sai: %+v", got)
	}
	// THIS IS THE HOP WHERE A FIELD GOES MISSING WITHOUT A TRACE. The wire type and tenant.Tenant
	// are mapped by hand, field by field, in both directions; a line forgotten here leaves every
	// commune in the country with an empty province and nothing to show why — the registry has the
	// value, the RPC carries it, and the seven services that read the registry over gRPC drop it.
	if got.Province != "Thành phố Đà Nẵng" {
		t.Errorf("Province = %q, muốn %q — ánh xạ ngược từ proto đánh rơi một trường",
			got.Province, "Thành phố Đà Nẵng")
	}
}

func TestNenTangKhongGuiTinhThanhThiLaChuoiRongChuKhongPhaiLoi(t *testing.T) {
	// "" IS A VALID ANSWER, and it arrives two ways that are deliberately NOT told apart: a
	// commune that has not declared a province, and a platform service old enough not to send
	// field 6 at all. The only behaviour that could distinguish them is guessing a province, which
	// is the one thing no consumer may do — so both must resolve normally and both yield "".
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{
		Tenant: &platformv1.Tenant{
			Id: ulidThu, Host: hostThu, DisplayName: "Xã Thăng Bình", Active: true,
		},
	}})

	got, ok := d.ByHost(context.Background(), hostThu)
	if !ok {
		t.Fatal("thiếu tỉnh/thành mà không phân giải được — một trường hiển thị không được phép " +
			"làm hỏng phép phân giải xã")
	}
	if got.Province != "" {
		t.Errorf("Province = %q, muốn chuỗi rỗng", got.Province)
	}
}

func TestNenTangKhongToiDuocThiTraFalseVaCoCanhBao(t *testing.T) {
	// THE CASE THIS FILE EXISTS FOR.
	//
	// ByHost returns a bool, so an outage and an unknown Host are the same answer to the edge:
	// 404. That is the correct answer — a request whose commune is unknown must never be served
	// (rule 1, invariant 3) — but with the platform down it is EVERY Host, so every member of
	// staff sees their own commune appear not to exist. Without a loud line at the moment it
	// happens, an infrastructure incident is indistinguishable from a batch of ordinary 404s.
	for _, ma := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.Unauthenticated} {
		t.Run(ma.String(), func(t *testing.T) {
			d, buf := dungThu(t, &nenTangGia{loi: status.Error(ma, "nền tảng không phản hồi")})

			if _, ok := d.ByHost(context.Background(), hostThu); ok {
				t.Fatal("nền tảng hỏng mà vẫn nhận là phân giải được — fail-open trên đường cô lập")
			}

			ra := buf.String()
			if !strings.Contains(ra, "level=WARN") {
				t.Fatalf("hỏng vì hạ tầng mà không có dòng WARN nào:\n%s", ra)
			}
			if !strings.Contains(ra, "CẢNH BÁO HẠ TẦNG") {
				t.Errorf("dòng cảnh báo không nói rõ đây là sự cố hạ tầng:\n%s", ra)
			}
			if !strings.Contains(ra, ma.String()) {
				t.Errorf("dòng cảnh báo không mang mã lỗi gRPC — không truy được nguyên nhân:\n%s", ra)
			}
		})
	}
}

func TestHostKhongThuocXaNaoThiIm(t *testing.T) {
	// An unknown Host is an ordinary daily event on any public address. Raising the same alarm
	// for it as for an outage is how an alarm stops meaning anything.
	d, buf := dungThu(t, &nenTangGia{loi: status.Error(codes.NotFound, "không có xã nào")})

	if _, ok := d.ByHost(context.Background(), "khong-ai-biet.example.gov.vn"); ok {
		t.Fatal("host lạ mà trả ok=true")
	}
	if strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("host lạ không được báo động mức WARN:\n%s", buf.String())
	}
}

func TestLoiKhongPhaiGrpcCungCoiLaHaTang(t *testing.T) {
	// A plain error carries codes.Unknown. It must not fall through as a quiet negative.
	d, buf := dungThu(t, &nenTangGia{loi: errors.New("kết nối bị đóng giữa chừng")})

	if _, ok := d.ByHost(context.Background(), hostThu); ok {
		t.Fatal("lỗi lạ mà vẫn nhận là phân giải được")
	}
	if !strings.Contains(buf.String(), "CẢNH BÁO HẠ TẦNG") {
		t.Errorf("lỗi không rõ nguồn gốc phải được báo động:\n%s", buf.String())
	}
}

func TestTraVeRongHoacMaKhongPhaiUlidThiTuChoi(t *testing.T) {
	// Fail closed on a broken contract. A commune id that is not a ULID means some end is using
	// an administrative code or a name — rule 1, invariant 2 — and accepting it would write a
	// meaningful identifier onto archival records that a merger cannot rewrite.
	cases := map[string]*platformv1.ResolveHostResponse{
		"không có xã":     {},
		"mã rỗng":         {Tenant: &platformv1.Tenant{Id: "", Host: hostThu, Active: true}},
		"mã hành chính":   {Tenant: &platformv1.Tenant{Id: "20302", Host: hostThu, Active: true}},
		"mã dài quá ULID": {Tenant: &platformv1.Tenant{Id: ulidThu + "X", Host: hostThu, Active: true}},
	}
	for ten, ra := range cases {
		t.Run(ten, func(t *testing.T) {
			d, buf := dungThu(t, &nenTangGia{ra: ra})

			if _, ok := d.ByHost(context.Background(), hostThu); ok {
				t.Fatal("hợp đồng hỏng mà vẫn nhận là phân giải được")
			}
			if !strings.Contains(buf.String(), "CẢNH BÁO HỢP ĐỒNG") {
				t.Errorf("hợp đồng hỏng phải được báo động:\n%s", buf.String())
			}
		})
	}
}

func TestXaNgungHoatDongVanTraVeDeEdgeTuQuyetDinh(t *testing.T) {
	// httpx.TenantMiddleware refuses a deactivated commune itself, and rule 7 keeps a merged
	// commune describable. Folding Active into the bool here would take that decision away from
	// the one place that makes it.
	d, _ := dungThu(t, &nenTangGia{ra: &platformv1.ResolveHostResponse{
		Tenant: &platformv1.Tenant{Id: ulidThu, Host: hostThu, DisplayName: "Xã đã sáp nhập", Active: false},
	}})

	got, ok := d.ByHost(context.Background(), hostThu)
	if !ok {
		t.Fatal("xã ngừng hoạt động vẫn phải mô tả được")
	}
	if got.Active {
		t.Error("Active phải được truyền nguyên vẹn")
	}
}

func TestDialKhongCoDiaChiThiTuChoi(t *testing.T) {
	// Fail closed and by name. A client with no address resolves no Host, so every commune gets
	// a 404 that reads like a misconfigured domain.
	if _, err := Dial("", slog.Default()); err == nil {
		t.Fatal("địa chỉ rỗng phải bị từ chối")
	} else if !strings.Contains(err.Error(), "PLATFORM_GRPC_ADDR") {
		t.Errorf("thông báo phải nói rõ thiếu biến nào: %v", err)
	}
}

func TestCoHanGoiDeMotPhuThuocChamKhongKeoSapMoiThu(t *testing.T) {
	// Without a deadline, a platform that accepts connections but never answers holds every
	// in-flight request of every other service open. The client must impose its own bound.
	var hanCo bool
	d, _ := dungThu(t, &nenTangGia{})
	d.cl = kiemHan{&hanCo}

	_, _ = d.ByHost(context.Background(), hostThu)
	if !hanCo {
		t.Fatal("ByHost gọi đi mà không đặt hạn — một nền tảng treo sẽ treo cả tám service")
	}
}

// kiemHan reports whether the outgoing context carried a deadline.
type kiemHan struct{ thay *bool }

func (k kiemHan) ResolveHost(ctx context.Context, _ *platformv1.ResolveHostRequest,
	_ ...grpc.CallOption) (*platformv1.ResolveHostResponse, error) {
	_, *k.thay = ctx.Deadline()
	return nil, status.Error(codes.NotFound, "không có xã nào")
}

func (k kiemHan) GetTenant(context.Context, *platformv1.GetTenantRequest,
	...grpc.CallOption) (*platformv1.GetTenantResponse, error) {
	return nil, status.Error(codes.Unimplemented, "không dùng trong test này")
}

// Hai phương thức dưới đây có mặt để kiemHan còn thoả PlatformServiceClient, không vì test nào
// gọi tới. `platformclient` là kho ĐỌC THEO HOST của mọi service — nó phân giải một xã mỗi lần
// và không có lý do nào để đọc cả sổ đăng ký. Ngày nó có, đó phải là một thay đổi người ta thấy
// được ở đây chứ không phải một phương thức lặng lẽ có sẵn.

func (k kiemHan) ListTenants(context.Context, *platformv1.ListTenantsRequest,
	...grpc.CallOption) (*platformv1.ListTenantsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "không dùng trong test này")
}

func (k kiemHan) ResolveTenantSuccession(context.Context, *platformv1.ResolveTenantSuccessionRequest,
	...grpc.CallOption) (*platformv1.ResolveTenantSuccessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "không dùng trong test này")
}
