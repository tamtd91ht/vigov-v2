package identityclient

// What these tests defend on THIS side of the wire: that an OUTAGE never becomes "no name", that a
// name is mapped by CODE and never by position, and that the response is not trusted to be
// well-formed — an item with no code, no name, no standing, or a code nobody asked for is a contract
// fault rather than something to render.
//
// The expensive mistake here is a wrong NOTHING: a blank printed where a person's name belongs, on
// an archival record a reader takes as fact.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

var (
	conTrongDanhBa = identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_IN_DIRECTORY
	daGoKhoiDanhBa = identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_REMOVED_FROM_DIRECTORY
)

// mayChuTenGia is identity answering ResolveStaffNames, and nothing else.
type mayChuTenGia struct {
	identityv1.UnimplementedIdentityServiceServer

	tra    []*identityv1.StaffName
	loi    error
	goi    int
	thayXa []string
	thay   *identityv1.ResolveStaffNamesRequest
}

func (s *mayChuTenGia) ResolveStaffNames(ctx context.Context, in *identityv1.ResolveStaffNamesRequest) (
	*identityv1.ResolveStaffNamesResponse, error) {
	s.goi++
	s.thay = in
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.thayXa = md.Get(grpcx.MetadataTenantKey)
	}
	if s.loi != nil {
		return nil, s.loi
	}
	return &identityv1.ResolveStaffNamesResponse{Items: s.tra}, nil
}

// Đường xanh: tên được ánh xạ THEO MÃ chứ không theo vị trí, người đã bị gỡ khỏi danh bạ vẫn có tên,
// và xã đi trong metadata.
func TestTenCanBoAnhXaTheoMaVaMangXaTrongMetadata(t *testing.T) {
	srv := &mayChuTenGia{tra: []*identityv1.StaffName{
		// CỐ Ý TRẢ NGƯỢC THỨ TỰ so với lúc hỏi: nếu bên gọi ánh xạ theo vị trí thì nó gắn tên người
		// này vào dòng của người kia — trên hồ sơ hành chính đó là một bản ghi bị làm sai lệch, chứ
		// không phải một lỗi hiển thị.
		{Ma: "CB-DA-GO", FullName: "Trần Thị B", Standing: daGoKhoiDanhBa},
		{Ma: "CB-TAI-CHUC", FullName: "Nguyễn Văn A", Standing: conTrongDanhBa},
	}}
	c := moMay(t, srv)

	ten, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-TAI-CHUC", "CB-DA-GO"})
	if err != nil {
		t.Fatalf("TenCanBoTheoMa: %v", err)
	}
	if ten["CB-TAI-CHUC"].HoTen != "Nguyễn Văn A" || ten["CB-TAI-CHUC"].TrangThai != conTrongDanhBa {
		t.Errorf("người tại chức = %+v", ten["CB-TAI-CHUC"])
	}
	if ten["CB-DA-GO"].HoTen != "Trần Thị B" {
		t.Errorf("NGƯỜI ĐÃ BỊ GỠ KHỎI DANH BẠ KHÔNG CÓ TÊN — hồ sơ lưu trữ sẽ hiện mã trần")
	}
	if ten["CB-DA-GO"].TrangThai != daGoKhoiDanhBa {
		t.Errorf("trạng thái = %v, muốn REMOVED_FROM_DIRECTORY", ten["CB-DA-GO"].TrangThai)
	}
	if len(srv.thayXa) != 1 || srv.thayXa[0] != string(xaA) {
		t.Errorf("x-tenant-id trên dây = %v, muốn đúng một giá trị %q — RPC này KHÔNG được miễn xã",
			srv.thayXa, string(xaA))
	}
}

// Sự cố hạ tầng KHÔNG BAO GIỜ thành "không có tên". Bên gọi vẽ một sự cố thành ô trống là viết một
// sự vắng mặt lên hồ sơ hành chính, và người đọc sau này coi đó là sự thật.
func TestTenCanBoLoiGoiKhongThanhKhongCoTen(t *testing.T) {
	c := moMay(t, &mayChuTenGia{loi: status.Error(codes.Unavailable, "identity đang xuống")})

	ten, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-A"})
	if err == nil {
		t.Fatal("GỌI HỎNG MÀ KHÔNG BÁO LỖI — bên gọi sẽ in ô trống lên hồ sơ lưu trữ")
	}
	if ten != nil {
		t.Errorf("trả về bản đồ %v kèm lỗi — bên gọi có thể vẽ nó", ten)
	}
	if status.Code(err) != codes.Unavailable && !strings.Contains(err.Error(), "Unavailable") {
		t.Errorf("lỗi không giữ được mã gRPC: %v", err)
	}
}

// Ít tên hơn số mã đã hỏi là BÌNH THƯỜNG — khác hẳn HanXuLy, và sự khác biệt ấy là cố ý: ở đó một
// mục thiếu thành hạn năm 1 ghi vào cột, ở đây nó là một cái tên bên gọi vốn phải vẽ được là chưa
// biết.
func TestTenCanBoItHonSoMaDaHoiKhongPhaiLoi(t *testing.T) {
	srv := &mayChuTenGia{tra: []*identityv1.StaffName{
		{Ma: "CB-CO", FullName: "Nguyễn Văn A", Standing: conTrongDanhBa},
	}}
	c := moMay(t, srv)

	ten, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-CO", "CB-KHONG-CO"})
	if err != nil {
		t.Fatalf("ít mục hơn số mã đã hỏi bị báo lỗi: %v", err)
	}
	if len(ten) != 1 {
		t.Fatalf("số tên = %d, muốn 1", len(ten))
	}
	if _, co := ten["CB-KHONG-CO"]; co {
		t.Error("mã không phân giải được lại có mặt trong bản đồ — bên gọi sẽ vẽ một mục rỗng")
	}
}

// PHẢN HỒI MANG MỘT MÃ KHÔNG ĐƯỢC HỎI là lỗi hợp đồng, không phải thứ để bỏ qua: đó chính là tính
// chất chống-liệt-kê thứ 3 của ADR 0034, kiểm từ đầu gần. Ngày nó bật nghĩa là đầu kia đã mọc một
// hành vi hợp đồng cấm.
func TestTenCanBoTraVeMaKhongDuocHoiLaLoiHopDong(t *testing.T) {
	srv := &mayChuTenGia{tra: []*identityv1.StaffName{
		{Ma: "CB-CO", FullName: "Nguyễn Văn A", Standing: conTrongDanhBa},
		{Ma: "CB-KHONG-AI-HOI", FullName: "Lê Văn C", Standing: conTrongDanhBa},
	}}
	c := moMay(t, srv)

	if _, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-CO"}); err == nil {
		t.Fatal("phản hồi mang mã chưa được hỏi mà không báo lỗi — bên gọi học được rằng mã ấy tồn tại")
	}
}

// Một mục KHÔNG CÓ HỌ TÊN là lỗi hợp đồng: `nguoi_dung.ho_ten` là NOT NULL, nên chuỗi rỗng không bao
// giờ nghĩa là "người này không có tên" — vẽ ra, nó là một dòng không ai đứng tên trên hồ sơ.
func TestTenCanBoMucThieuHoTenLaLoiHopDong(t *testing.T) {
	c := moMay(t, &mayChuTenGia{tra: []*identityv1.StaffName{
		{Ma: "CB-CO", FullName: "", Standing: conTrongDanhBa},
	}})

	if _, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-CO"}); err == nil {
		t.Fatal("mục không có họ tên mà không báo lỗi")
	}
}

// Một mục KHÔNG NÓI RÕ TRẠNG THÁI BẢN GHI là lỗi hợp đồng. Giá trị 0 của enum cố ý là UNSPECIFIED
// chứ không phải `false` của một bool: một phép gán bị quên không được âm thầm khẳng định một trong
// hai câu trả lời thật, mà một trong hai câu ấy là lời nói về một con người in lên màn hình nhà nước.
func TestTenCanBoMucThieuTrangThaiLaLoiHopDong(t *testing.T) {
	c := moMay(t, &mayChuTenGia{tra: []*identityv1.StaffName{
		{Ma: "CB-CO", FullName: "Nguyễn Văn A"},
	}})

	if _, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-CO"}); err == nil {
		t.Fatal("mục không nói rõ bản ghi còn trong danh bạ hay không mà không báo lỗi")
	}
}

// Danh sách rỗng KHÔNG ĐI RA DÂY và trả bản đồ rỗng — không phải lỗi, và tuyệt đối không phải "tất
// cả". Một trang danh sách không có dòng nào thì sinh ra không mã nào, đó là lời gọi thường ngày.
func TestTenCanBoDanhSachRongKhongGoiVaTraRong(t *testing.T) {
	srv := &mayChuTenGia{}
	c := moMay(t, srv)

	ten, err := c.TenCanBoTheoMa(ngucCanh(), nil)
	if err != nil {
		t.Fatalf("danh sách rỗng bị báo lỗi: %v", err)
	}
	if len(ten) != 0 {
		t.Errorf("số tên = %d, muốn 0", len(ten))
	}
	if srv.goi != 0 {
		t.Error("đã gọi qua mạng cho một yêu cầu không hỏi gì")
	}
}

// Một danh sách chỉ gồm chuỗi rỗng cũng không đi ra dây, và cũng không thành "tất cả".
func TestTenCanBoToanMaRongKhongGoi(t *testing.T) {
	srv := &mayChuTenGia{}
	c := moMay(t, srv)

	ten, err := c.TenCanBoTheoMa(ngucCanh(), []string{"", ""})
	if err != nil {
		t.Fatalf("mã rỗng bị báo lỗi: %v", err)
	}
	if len(ten) != 0 || srv.goi != 0 {
		t.Errorf("số tên = %d, số lần gọi = %d — mã rỗng KHÔNG được thành 'trả tất cả'", len(ten), srv.goi)
	}
}

// Quá trần bị TỪ CHỐI TẠI CHỖ, không gửi đi và KHÔNG BAO GIỜ cắt bớt: máy chủ trả INVALID_ARGUMENT
// cho đúng việc này, và một lô bị cắt làm những dòng có thật hiện trống mà không gì báo.
func TestTenCanBoQuaTranTuChoiTaiChoKhongCatBot(t *testing.T) {
	srv := &mayChuTenGia{}
	c := moMay(t, srv)

	ma := make([]string, TranMaMotLo+1)
	for i := range ma {
		ma[i] = "CB-QUA-TRAN"
	}

	if _, err := c.TenCanBoTheoMa(ngucCanh(), ma); err == nil {
		t.Fatal("quá trần mà không báo lỗi")
	}
	if srv.goi != 0 {
		t.Error("đã gửi một yêu cầu chỉ có thể thất bại")
	}
}

// Một lỗi vận chuyển thuần (không phải status) vẫn là lỗi, không phải "không có tên".
func TestTenCanBoLoiThuongVanLaLoi(t *testing.T) {
	c := moMay(t, &mayChuTenGia{loi: errors.New("giả lập: hỏng")})

	if _, err := c.TenCanBoTheoMa(ngucCanh(), []string{"CB-A"}); err == nil {
		t.Fatal("lỗi vận chuyển bị nuốt")
	}
}
