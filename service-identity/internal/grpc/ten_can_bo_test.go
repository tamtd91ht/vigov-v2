package grpc

// What these tests defend: the BRANCHES of ResolveStaffNames — the ceiling, the fail-closed commune
// check, the mapping of `deleted_at` onto the enum, and the properties that keep this a lookup
// rather than a roster.
//
// WHAT THEY CANNOT DEFEND, STATED SO NOBODY READS MORE INTO THEM: the SQL predicate. Whether a
// soft-deleted row is really returned, and whether another commune's row really is not, lives
// entirely in the query — a fake here would simply agree with whatever this file asserts. Those two
// are defended in internal/store/can_bo_ten_pg_test.go against a real PostgreSQL.

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

const (
	inDirectory = identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_IN_DIRECTORY
	daGoKhoiDB  = identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_REMOVED_FROM_DIRECTORY
)

// tenGia is the name read, absent. It records exactly what the server passed down, which is how the
// dedup and empty-dropping assertions are made without reaching into locID.
type tenGia struct {
	hang     []domain.TenCanBo
	err      error
	daNhan   []string
	soLanGoi int
}

func (f *tenGia) TenTheoNhieuMa(_ context.Context, ma []string) ([]domain.TenCanBo, error) {
	f.soLanGoi++
	f.daNhan = ma
	return f.hang, f.err
}

// goiTen runs the RPC with a fake holding the given rows, and hands back both the response and the
// fake so a test can assert on what went down as well as what came back.
func goiTen(t *testing.T, hang []domain.TenCanBo, ma []string) (*identityv1.ResolveStaffNamesResponse, *tenGia) {
	t.Helper()
	kho := &tenGia{hang: hang}
	s, _ := may(t, func(d *Deps) { d.Ten = kho })
	ra, err := s.ResolveStaffNames(ctxXa(xaA), &identityv1.ResolveStaffNamesRequest{Ma: ma})
	if err != nil {
		t.Fatalf("ResolveStaffNames: %v", err)
	}
	return ra, kho
}

// timMa reads one item out of the response by its code. The caller maps by `ma` and never by
// position — asserting on items[0] would let a wrong-order answer pass.
func timMa(ra *identityv1.ResolveStaffNamesResponse, ma string) (*identityv1.StaffName, bool) {
	for _, it := range ra.GetItems() {
		if it.GetMa() == ma {
			return it, true
		}
	}
	return nil, false
}

// CA BẮT BUỘC 1 — mã ĐÃ XOÁ MỀM trả về TÊN, kèm REMOVED_FROM_DIRECTORY.
//
// This is the whole reason the RPC exists (ADR 0034): an archival record naming somebody who was
// taken off the directory must still print a name. Absent here means a blank on a 2026 handling
// sheet opened in 2029, at the moment an inspection asks who handled it.
func TestTenCanBoDaXoaMemVanTraVeTenKemDaGoKhoiDanhBa(t *testing.T) {
	ra, _ := goiTen(t,
		[]domain.TenCanBo{{Ma: "CB-2026-7K3M9Q", HoTen: "Trần Thị B", ConTrongDanhBa: false}},
		[]string{"CB-2026-7K3M9Q"})

	it, co := timMa(ra, "CB-2026-7K3M9Q")
	if !co {
		t.Fatal("MÃ ĐÃ XOÁ MỀM KHÔNG TRẢ VỀ GÌ — hồ sơ lưu trữ sẽ hiện mã trần ở chỗ tên người xử lý")
	}
	if it.GetFullName() != "Trần Thị B" {
		t.Errorf("full_name = %q, muốn tên người", it.GetFullName())
	}
	if it.GetStanding() != daGoKhoiDB {
		t.Errorf("standing = %v, muốn REMOVED_FROM_DIRECTORY — bên gọi sẽ tưởng người này còn trong danh bạ",
			it.GetStanding())
	}
}

// CA BẮT BUỘC 3 — mã ĐANG TẠI CHỨC trả IN_DIRECTORY, và TÀI KHOẢN BỊ KHOÁ CŨNG IN_DIRECTORY.
//
// Câu mở #10 (chốt 22/09/2026): nghỉ hưu / chuyển công tác là KHOÁ và người ấy VẪN hiện trong danh
// bạ. The store deliberately does not read `dang_hoat_dong`, so both rows below reach the handler
// as ConTrongDanhBa true — this test is what turns red if somebody adds that column to the
// predicate and starts printing "không còn công tác" about somebody who has not left.
func TestTenCanBoTaiChucVaBiKhoaDeuConTrongDanhBa(t *testing.T) {
	ra, _ := goiTen(t, []domain.TenCanBo{
		{Ma: "CB-2026-TAICHUC", HoTen: "Nguyễn Văn A", ConTrongDanhBa: true},
		// The locked account. `dang_hoat_dong = false` never reaches this type, on purpose.
		{Ma: "CB-2026-BIKHOA0", HoTen: "Lê Văn C", ConTrongDanhBa: true},
	}, []string{"CB-2026-TAICHUC", "CB-2026-BIKHOA0"})

	for _, ma := range []string{"CB-2026-TAICHUC", "CB-2026-BIKHOA0"} {
		it, co := timMa(ra, ma)
		if !co {
			t.Fatalf("%s không trả về", ma)
		}
		if it.GetStanding() != inDirectory {
			t.Errorf("%s: standing = %v, muốn IN_DIRECTORY (tài khoản bị KHOÁ vẫn trong danh bạ — câu mở #10)",
				ma, it.GetStanding())
		}
	}
}

// Danh sách RỖNG trả RỖNG, và KHÔNG CHẠM TỚI KHO. "Everybody" has no spelling in this request, and
// this is the single line a later edit could turn a lookup into a listing with (ADR 0034).
func TestTenCanBoDanhSachRongTraRongVaKhongDocKho(t *testing.T) {
	ra, kho := goiTen(t, []domain.TenCanBo{{Ma: "KHONG-DUOC-HOI", HoTen: "Không ai hỏi"}}, nil)

	if len(ra.GetItems()) != 0 {
		t.Fatalf("số mục = %d, muốn 0 — DANH SÁCH RỖNG KHÔNG BAO GIỜ NGHĨA LÀ 'TẤT CẢ'", len(ra.GetItems()))
	}
	if len(kho.daNhan) != 0 {
		t.Errorf("kho nhận %v, muốn không có mã nào", kho.daNhan)
	}
}

// Một danh sách chỉ gồm chuỗi rỗng cũng KHÔNG thành "tất cả". An empty string is dropped before the
// read, so the store sees an empty slice and returns without a query — the same outcome as an empty
// list, reached by a different route a caller could actually send.
func TestTenCanBoMaRongKhongThanhTatCa(t *testing.T) {
	ra, kho := goiTen(t, []domain.TenCanBo{{Ma: "KHONG-DUOC-HOI", HoTen: "Không ai hỏi"}},
		[]string{"", ""})

	if len(ra.GetItems()) != 0 {
		t.Fatalf("số mục = %d, muốn 0 — mã rỗng KHÔNG được thành 'trả tất cả'", len(ra.GetItems()))
	}
	if len(kho.daNhan) != 0 {
		t.Errorf("kho nhận %v, muốn không có mã nào", kho.daNhan)
	}
}

// Trùng lặp được GOM Ở MÁY CHỦ, đúng như hợp đồng khai — a caller building codes from a list of rows
// naturally repeats the same handler.
func TestTenCanBoGomTrungLap(t *testing.T) {
	_, kho := goiTen(t, nil, []string{"CB-A", "CB-A", "CB-B", ""})

	if len(kho.daNhan) != 2 {
		t.Fatalf("kho nhận %v, muốn đúng 2 mã sau khi gom trùng và bỏ rỗng", kho.daNhan)
	}
}

// Quá trần 200 là INVALID_ARGUMENT, KHÔNG BAO GIỜ cắt bớt lặng lẽ: không có con trỏ để đi tiếp, nên
// một lô bị cắt làm những dòng có thật hiện trống mà không gì báo.
func TestTenCanBoQuaTranLaInvalidArgument(t *testing.T) {
	kho := &tenGia{}
	s, _ := may(t, func(d *Deps) { d.Ten = kho })

	ma := make([]string, TranIDMotLo+1)
	for i := range ma {
		ma[i] = "CB-" + strings.Repeat("A", 7)
	}

	_, err := s.ResolveStaffNames(ctxXa(xaA), &identityv1.ResolveStaffNamesRequest{Ma: ma})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
	if kho.soLanGoi != 0 {
		t.Error("đã đọc kho dù vượt trần — trần phải chặn TRƯỚC khi chạm cơ sở dữ liệu")
	}
}

// TRẦN ĐƯỢC KIỂM TRÊN THỨ BÊN GỌI GỬI, trước khi gom trùng: mục đích là chặn một yêu cầu không có
// trần của chính nó, nên 201 mã mà chỉ 1 mã khác nhau vẫn bị từ chối.
func TestTenCanBoTranTinhTrenSoMaDaGuiChuKhongPhaiSauKhiGomTrung(t *testing.T) {
	kho := &tenGia{}
	s, _ := may(t, func(d *Deps) { d.Ten = kho })

	ma := make([]string, TranIDMotLo+1)
	for i := range ma {
		ma[i] = "CB-TRUNG"
	}

	_, err := s.ResolveStaffNames(ctxXa(xaA), &identityv1.ResolveStaffNamesRequest{Ma: ma})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument", status.Code(err))
	}
}

// Thiếu xã trong context nghĩa là grpcx.UnaryServerInterceptor không nằm trong chuỗi. Đó là lỗi cấu
// hình triển khai nên trả Internal — và TUYỆT ĐỐI KHÔNG PANIC: một panic trong handler gRPC không
// được phục hồi, nó giết tiến trình đang phục vụ 200+ xã. Đây cũng là chỗ chặn store.Scoped gọi
// tenant.MustFrom.
func TestTenCanBoThieuXaLaInternalChuKhongPanic(t *testing.T) {
	kho := &tenGia{}
	s, _ := may(t, func(d *Deps) { d.Ten = kho })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()

	_, err := s.ResolveStaffNames(context.Background(), &identityv1.ResolveStaffNamesRequest{Ma: []string{"CB-A"}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if kho.soLanGoi != 0 {
		t.Error("ĐÃ ĐỌC KHO KHI KHÔNG BIẾT XÃ NÀO — đây là đường đi tới một truy vấn không có phép cắt theo xã")
	}
}

// Kho hỏng là Internal, KHÔNG phải "không có tên". A caller that renders an outage as a blank writes
// an absence onto an administrative record that a person then reads as fact.
func TestTenCanBoKhoHongLaInternalChuKhongPhaiRong(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.Ten = &tenGia{err: loiKho} })

	_, err := s.ResolveStaffNames(ctxXa(xaA), &identityv1.ResolveStaffNamesRequest{Ma: []string{"CB-A"}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal — sự cố hạ tầng không được thành 'không có tên'", status.Code(err))
	}
}

// Ít mục hơn số mã đã hỏi là BÌNH THƯỜNG, không phải lỗi: một mã của xã khác, hay của không ai cả,
// đơn giản là vắng mặt — và hai trường hợp ấy cố ý không phân biệt được.
func TestTenCanBoItHonSoMaDaHoiLaBinhThuong(t *testing.T) {
	ra, _ := goiTen(t, []domain.TenCanBo{{Ma: "CB-CO", HoTen: "Nguyễn Văn A", ConTrongDanhBa: true}},
		[]string{"CB-CO", "CB-KHONG-TON-TAI", "CB-CUA-XA-KHAC"})

	if len(ra.GetItems()) != 1 {
		t.Fatalf("số mục = %d, muốn 1", len(ra.GetItems()))
	}
	if _, co := timMa(ra, "CB-CUA-XA-KHAC"); co {
		t.Error("mã của xã khác lại có mặt")
	}
}

// Hợp đồng không có trường nào để ĐÁNH DẤU "vắng mặt", và đây là phép kiểm rằng handler không tự
// nghĩ ra một mục rỗng cho mã không phân giải được. Một mục có `ma` mà `full_name` rỗng chính là ô
// trống in lên hồ sơ lưu trữ.
func TestTenCanBoKhongTuDeMucRongChoMaKhongPhanGiaiDuoc(t *testing.T) {
	ra, _ := goiTen(t, nil, []string{"CB-KHONG-TON-TAI"})

	if len(ra.GetItems()) != 0 {
		t.Fatalf("số mục = %d, muốn 0 — mã không phân giải được là MỤC VẮNG MẶT, không phải mục rỗng",
			len(ra.GetItems()))
	}
}
