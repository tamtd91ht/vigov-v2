package identityclient

// What these tests defend on THIS side of the wire: that a refusal from identity stays a refusal.
//
// The expensive mistake here is not a wrong instant, it is a wrong NOTHING — an error folded into
// "no deadline", a missing item read as a zero time.Time, or a caller-side default filling a gap
// the commune deliberately left empty. Every one of those produces a record that looks complete and
// carries a commitment nobody made.

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

var (
	tiepNhan  = identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN
	xuLyXong  = identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG
	mocDemGia = time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)
)

// mayChuHanGia is identity answering ResolveDeadlines, and nothing else.
type mayChuHanGia struct {
	identityv1.UnimplementedIdentityServiceServer

	tra    []*identityv1.Deadline
	loi    error
	goi    int
	thayXa []string
	thay   *identityv1.ResolveDeadlinesRequest
}

func (s *mayChuHanGia) ResolveDeadlines(ctx context.Context, in *identityv1.ResolveDeadlinesRequest) (
	*identityv1.ResolveDeadlinesResponse, error) {
	s.goi++
	s.thay = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.thayXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.loi != nil {
		return nil, s.loi
	}
	return &identityv1.ResolveDeadlinesResponse{Items: s.tra}, nil
}

// Đường xanh: hai đồng hồ, ánh xạ THEO ĐỒNG HỒ chứ không theo vị trí — và xã đi trong metadata.
func TestHanXuLyTraDuHaiDongHoVaMangXaTrongMetadata(t *testing.T) {
	tn := mocDemGia.Add(8 * time.Hour)
	xl := mocDemGia.Add(40 * time.Hour)
	srv := &mayChuHanGia{tra: []*identityv1.Deadline{
		// CỐ Ý TRẢ NGƯỢC THỨ TỰ so với lúc hỏi: nếu bên gọi ánh xạ theo vị trí thì nó lấy hạn xử lý
		// xong làm hạn tiếp nhận, và cả hai đều là mốc hợp lệ nên không gì báo lỗi.
		{Kind: xuLyXong, DueAt: timestamppb.New(xl)},
		{Kind: tiepNhan, DueAt: timestamppb.New(tn)},
	}}
	c := moMay(t, srv)

	han, err := c.HanXuLy(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan, xuLyXong})
	if err != nil {
		t.Fatalf("HanXuLy: %v", err)
	}
	if !han[tiepNhan].Equal(tn) {
		t.Errorf("hạn tiếp nhận = %s, muốn %s", han[tiepNhan], tn)
	}
	if !han[xuLyXong].Equal(xl) {
		t.Errorf("hạn xử lý xong = %s, muốn %s", han[xuLyXong], xl)
	}
	if len(srv.thayXa) != 1 || srv.thayXa[0] != string(xaA) {
		t.Errorf("x-tenant-id trên dây = %v, muốn đúng một giá trị %q — RPC này KHÔNG được miễn xã",
			srv.thayXa, string(xaA))
	}
	// `linh_vuc` rỗng phải đi nguyên vẹn: nó là câu hỏi "cho tôi dòng mặc định" của kênh công dân
	// (ADR 0028 quyết định E), không phải một trường bị quên mà bên gọi được phép điền hộ.
	if srv.thay.GetLinhVuc() != "" {
		t.Errorf("linh_vuc trên dây = %q, muốn rỗng — bên bọc không được tự điền một mã lĩnh vực",
			srv.thay.GetLinhVuc())
	}
}

// XÃ CHƯA CẤU HÌNH LÀ MỘT LỖI, KHÔNG PHẢI MỘT MẶC ĐỊNH. Hôm nay đây là trạng thái của mọi xã
// (migration 0008 không gieo dòng nào), nên ca này canh đúng đường chạy thật của hệ thống.
func TestHanXuLyFailedPreconditionLaLoiChuKhongPhaiHanMacDinh(t *testing.T) {
	c := moMay(t, &mayChuHanGia{
		loi: status.Error(codes.FailedPrecondition, "xã chưa cấu hình bảng thời hạn xử lý"),
	})

	han, err := c.HanXuLy(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan})
	if err == nil {
		t.Fatalf("không có lỗi, và trả về %v — một xã chưa cấu hình phải làm HỎNG lượt tiếp nhận", han)
	}
	if han != nil {
		t.Errorf("trả về %v kèm lỗi — bên gọi có thể lưu nhầm bản đồ rỗng thành 'không có hạn'", han)
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("mã = %v, muốn giữ nguyên FailedPrecondition để người vận hành biết mở màn hình cấu hình",
			status.Code(err))
	}
}

// Sự cố hạ tầng cũng là lỗi, và mã phải giữ được để phân biệt với "chưa cấu hình": cả hai đều làm
// hỏng lượt tiếp nhận, nhưng chỉ một trong hai sửa được bằng màn hình cấu hình.
func TestHanXuLyUnavailableGiuNguyenMa(t *testing.T) {
	c := moMay(t, &mayChuHanGia{loi: status.Error(codes.Unavailable, "identity đang sập")})

	_, err := c.HanXuLy(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("mã = %v, muốn Unavailable (lỗi: %v)", status.Code(err), err)
	}
}

// KHÔNG CÓ THÀNH CÔNG MỘT PHẦN. Một đồng hồ đã hỏi mà không có trong phản hồi phải thành LỖI, chứ
// không phải một mục vắng mặt — vắng mặt rơi tới bên gọi thành time.Time rỗng, tức hạn năm 1, tức
// hồ sơ quá hạn ngay lúc sinh ra.
func TestHanXuLyThieuMotDongHoLaLoiChuKhongPhaiMocRong(t *testing.T) {
	c := moMay(t, &mayChuHanGia{tra: []*identityv1.Deadline{
		{Kind: tiepNhan, DueAt: timestamppb.New(mocDemGia.Add(8 * time.Hour))},
	}})

	han, err := c.HanXuLy(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan, xuLyXong})
	if err == nil {
		t.Fatalf("không có lỗi, trả về %v — thiếu một đồng hồ là lỗi hợp đồng", han)
	}
	if han != nil {
		t.Errorf("trả về %v kèm lỗi", han)
	}
}

// Một mục có đồng hồ nhưng KHÔNG có mốc cũng là lỗi hợp đồng, không phải một mốc rỗng đem đi lưu.
func TestHanXuLyMocRongLaLoi(t *testing.T) {
	c := moMay(t, &mayChuHanGia{tra: []*identityv1.Deadline{{Kind: tiepNhan}}})

	_, err := c.HanXuLy(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan})
	if err == nil {
		t.Fatal("không có lỗi cho một hạn không có due_at")
	}
}

// Lời gọi không thể thành công thì KHÔNG ĐƯỢC LÊN DÂY. Một yêu cầu chỉ có thể hỏng không có việc gì
// trên mạng, và từ chối tại chỗ cho thông điệp nói đúng nguyên nhân — bên gọi sai — thay vì một
// INVALID_ARGUMENT mô tả một trường trên dây.
func TestHanXuLyTuChoiTaiChoKhongGuiLenDay(t *testing.T) {
	ca := []struct {
		ten      string
		loaiViec identityv1.WorkKind
		tuLuc    time.Time
		can      []identityv1.DeadlineKind
	}{
		{"không có loại việc", identityv1.WorkKind_WORK_KIND_UNSPECIFIED, mocDemGia,
			[]identityv1.DeadlineKind{tiepNhan}},
		{"gốc đếm rỗng", identityv1.WorkKind_WORK_KIND_PHAN_ANH, time.Time{},
			[]identityv1.DeadlineKind{tiepNhan}},
		{"không nói rõ đồng hồ nào", identityv1.WorkKind_WORK_KIND_PHAN_ANH, mocDemGia, nil},
		{"đồng hồ chưa xác định", identityv1.WorkKind_WORK_KIND_PHAN_ANH, mocDemGia,
			[]identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_UNSPECIFIED}},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			srv := &mayChuHanGia{}
			cl := moMay(t, srv)

			if _, err := cl.HanXuLy(ngucCanh(), c.loaiViec, "", c.tuLuc, c.can); err == nil {
				t.Fatal("không có lỗi")
			}
			if srv.goi != 0 {
				t.Errorf("đã gọi máy chủ %d lần cho một yêu cầu chỉ có thể hỏng", srv.goi)
			}
		})
	}
}

// KHÔNG ĐƯỢC MIỄN XÃ. Gọi mà context không có xã thì interceptor phía client phải từ chối TẠI CHỖ:
// RPC này đọc cấu hình CỦA MỘT XÃ, và một lời gọi không nói xã nào là một lời gọi không có câu trả
// lời đúng. Thêm tên nó vào core/grpcx.methodsWithoutTenant là điều kiện dừng của ADR 0012.
func TestHanXuLyKhongCoXaThiBiTuChoiTaiCho(t *testing.T) {
	srv := &mayChuHanGia{}
	c := moMay(t, srv)

	_, err := c.HanXuLy(context.Background(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan})
	if err == nil {
		t.Fatal("gọi được ResolveDeadlines mà không có xã trong context")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("mã = %v, muốn InvalidArgument từ grpcx.UnaryClientInterceptor (lỗi: %v)",
			status.Code(err), err)
	}
	if srv.goi != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.goi)
	}
}

// Một lỗi không phải status của gRPC vẫn phải là lỗi, không phải một bản đồ rỗng.
func TestHanXuLyLoiThuongVanLaLoi(t *testing.T) {
	c := moMay(t, &mayChuHanGia{loi: errors.New("hỏng gì đó")})

	if _, err := c.HanXuLy(ngucCanh(), identityv1.WorkKind_WORK_KIND_PHAN_ANH, "",
		mocDemGia, []identityv1.DeadlineKind{tiepNhan}); err == nil {
		t.Fatal("không có lỗi")
	}
}
