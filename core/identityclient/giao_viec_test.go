package identityclient

// What these tests defend on THIS side of the wire: that the caller DECIDES from a set that is
// exactly what identity answered — an outage is never "not assignable" and never "assignable", a
// code nobody asked for is a contract fault, and the commune travels in metadata.
//
// The expensive mistake here is a wrong YES: work handed to somebody who can never sign in to act.

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// mayChuGiaoViecGia is identity answering ResolveAssignableStaff, and nothing else.
type mayChuGiaoViecGia struct {
	identityv1.UnimplementedIdentityServiceServer

	tra    []string
	loi    error
	goi    int
	thayXa []string
	thay   *identityv1.ResolveAssignableStaffRequest
}

func (s *mayChuGiaoViecGia) ResolveAssignableStaff(ctx context.Context, in *identityv1.ResolveAssignableStaffRequest) (
	*identityv1.ResolveAssignableStaffResponse, error) {
	s.goi++
	s.thay = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.thayXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.loi != nil {
		return nil, s.loi
	}
	return &identityv1.ResolveAssignableStaffResponse{AssignableMa: s.tra}, nil
}

// Đường xanh: mã được trả là tập được giao việc, mã vắng mặt là KHÔNG được giao, và xã đi trong metadata.
func TestGiaoViecTraTapVaMangXaTrongMetadata(t *testing.T) {
	srv := &mayChuGiaoViecGia{tra: []string{"CB-DUOC"}}
	c := moMay(t, srv)

	duoc, err := c.CanBoGiaoViecDuoc(ngucCanh(), []string{"CB-DUOC", "CB-BI-KHOA"})
	if err != nil {
		t.Fatalf("CanBoGiaoViecDuoc: %v", err)
	}
	if _, co := duoc["CB-DUOC"]; !co {
		t.Error("mã được giao việc không có trong tập")
	}
	if _, co := duoc["CB-BI-KHOA"]; co {
		t.Error("MÃ KHÔNG ĐƯỢC TRẢ LẠI CÓ TRONG TẬP — việc sẽ giao cho người không đăng nhập được")
	}
	if len(srv.thayXa) != 1 || srv.thayXa[0] != string(xaA) {
		t.Errorf("x-tenant-id trên dây = %v, muốn đúng một giá trị %q — RPC này KHÔNG được miễn xã",
			srv.thayXa, string(xaA))
	}
}

// RPC này không được nằm trong danh sách miễn xã: miễn xã biến "thuộc xã của bên gọi" thành
// "thuộc một xã nào đó".
func TestGiaoViecKhongDuocMienXa(t *testing.T) {
	if grpcx.ExemptFromTenant(identityv1.IdentityService_ResolveAssignableStaff_FullMethodName) {
		t.Fatal("ResolveAssignableStaff nằm trong danh sách miễn xã")
	}
}

// Sự cố KHÔNG BAO GIỜ thành một câu trả lời: không phải "không được giao", càng không phải "được giao".
func TestGiaoViecLoiGoiKhongThanhCauTraLoi(t *testing.T) {
	for _, loi := range []error{status.Error(codes.Unavailable, "identity đang xuống"), errors.New("giả lập: hỏng")} {
		c := moMay(t, &mayChuGiaoViecGia{loi: loi})
		duoc, err := c.CanBoGiaoViecDuoc(ngucCanh(), []string{"CB-A"})
		if err == nil {
			t.Fatalf("gọi hỏng (%v) mà không báo lỗi", loi)
		}
		if duoc != nil {
			t.Errorf("trả về tập %v kèm lỗi — bên gọi có thể quyết định từ nó", duoc)
		}
	}
}

// Phản hồi mang một mã KHÔNG ĐƯỢC HỎI (kể cả chuỗi rỗng) là lỗi hợp đồng — bên gọi sắp quyết định từ tập này.
func TestGiaoViecTraVeMaKhongDuocHoiLaLoiHopDong(t *testing.T) {
	for _, la := range []string{"CB-KHONG-AI-HOI", ""} {
		c := moMay(t, &mayChuGiaoViecGia{tra: []string{"CB-CO", la}})
		if _, err := c.CanBoGiaoViecDuoc(ngucCanh(), []string{"CB-CO"}); err == nil {
			t.Fatalf("phản hồi mang mã chưa được hỏi %q mà không báo lỗi", la)
		}
	}
}

// Danh sách rỗng, hoặc toàn chuỗi rỗng, KHÔNG ĐI RA DÂY và trả tập rỗng — không phải "tất cả".
func TestGiaoViecRongKhongGoi(t *testing.T) {
	for _, ma := range [][]string{nil, {"", ""}} {
		srv := &mayChuGiaoViecGia{tra: []string{"CB-X"}}
		c := moMay(t, srv)
		duoc, err := c.CanBoGiaoViecDuoc(ngucCanh(), ma)
		if err != nil {
			t.Fatalf("yêu cầu rỗng bị báo lỗi: %v", err)
		}
		if len(duoc) != 0 || srv.goi != 0 {
			t.Errorf("số mã = %d, số lần gọi = %d — yêu cầu rỗng KHÔNG được thành 'tất cả'", len(duoc), srv.goi)
		}
	}
}

// Quá trần bị TỪ CHỐI TẠI CHỖ, không gửi đi và không cắt bớt.
func TestGiaoViecQuaTranTuChoiTaiCho(t *testing.T) {
	srv := &mayChuGiaoViecGia{}
	c := moMay(t, srv)

	ma := make([]string, TranMaGiaoViecMotLo+1)
	for i := range ma {
		ma[i] = "CB-QUA-TRAN"
	}
	if _, err := c.CanBoGiaoViecDuoc(ngucCanh(), ma); err == nil {
		t.Fatal("quá trần mà không báo lỗi")
	}
	if srv.goi != 0 {
		t.Error("đã gửi một yêu cầu chỉ có thể thất bại")
	}
}
