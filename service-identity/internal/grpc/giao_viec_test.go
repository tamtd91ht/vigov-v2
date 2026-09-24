package grpc

// What these tests defend: the BOUNDARY of ResolveAssignableStaff — ceiling, fail-closed commune,
// empty-is-empty, dedup, and "the answer is a subset, each once".
//
// WHAT THEY CANNOT DEFEND: the SQL predicate (locked / no account / deleted / other commune). That
// lives in the query and is defended in internal/store/can_bo_giao_viec_test.go (fake engine that
// executes the predicate) and can_bo_giao_viec_pg_test.go (real PostgreSQL). The fake below keys its
// answer by the commune in the context, so the isolation case here proves the handler reads the
// commune from ctx and nowhere else — not that the SQL filters it.

import (
	"context"
	"sort"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
)

// giaoViecGia answers, per commune, the codes that commune holds as assignable — filtered by what was
// asked unless traLai is set (which returns it verbatim, to exercise the subset guard).
type giaoViecGia struct {
	theoXa   map[tenant.ID][]string
	traLai   []string
	err      error
	daNhan   []string
	soLanGoi int
}

func (f *giaoViecGia) GiaoViecDuoc(ctx context.Context, ma []string) ([]string, error) {
	f.soLanGoi++
	f.daNhan = ma
	if f.err != nil {
		return nil, f.err
	}
	if f.traLai != nil {
		return f.traLai, nil
	}
	xa, _ := tenant.From(ctx)
	var ra []string
	for _, co := range f.theoXa[xa] {
		for _, hoi := range ma {
			if co == hoi {
				ra = append(ra, co)
			}
		}
	}
	return ra, nil
}

func goiGiaoViec(t *testing.T, kho *giaoViecGia, ctx context.Context, ma []string) (*identityv1.ResolveAssignableStaffResponse, error) {
	t.Helper()
	s, _ := may(t, func(d *Deps) { d.GiaoViec = kho })
	return s.ResolveAssignableStaff(ctx, &identityv1.ResolveAssignableStaffRequest{Ma: ma})
}

func sapXep(ma []string) string {
	c := append([]string(nil), ma...)
	sort.Strings(c)
	return strings.Join(c, ",")
}

// Mã của xã khác VẮNG MẶT — câu trả lời được cắt theo xã trong context, không theo gì khác.
func TestGiaoViecCatTheoXaTrongContext(t *testing.T) {
	kho := &giaoViecGia{theoXa: map[tenant.ID][]string{
		xaA: {"CB-A1"},
		xaB: {"CB-B1"},
	}}
	ra, err := goiGiaoViec(t, kho, ctxXa(xaA), []string{"CB-A1", "CB-B1"})
	if err != nil {
		t.Fatalf("ResolveAssignableStaff: %v", err)
	}
	if got := sapXep(ra.GetAssignableMa()); got != "CB-A1" {
		t.Fatalf("xã A nhận %s, muốn chỉ CB-A1 — mã của xã B phải vắng (luật 1)", got)
	}
}

// Trùng lặp được GOM Ở MÁY CHỦ trước khi đọc, và rỗng bị bỏ.
func TestGiaoViecGomTrungLapVaBoRong(t *testing.T) {
	kho := &giaoViecGia{theoXa: map[tenant.ID][]string{xaA: {"CB-A"}}}
	ra, err := goiGiaoViec(t, kho, ctxXa(xaA), []string{"CB-A", "CB-A", "", "CB-B", "CB-A"})
	if err != nil {
		t.Fatalf("ResolveAssignableStaff: %v", err)
	}
	if sapXep(kho.daNhan) != "CB-A,CB-B" {
		t.Fatalf("kho nhận %v, muốn đúng CB-A,CB-B", kho.daNhan)
	}
	if got := ra.GetAssignableMa(); len(got) != 1 || got[0] != "CB-A" {
		t.Fatalf("trả %v, muốn [CB-A] đúng một lần", got)
	}
}

// Câu trả lời là TẬP CON của câu hỏi, mỗi mã MỘT LẦN — kể cả khi kho trả thừa hoặc trả lặp.
func TestGiaoViecTraTapConMoiMaMotLan(t *testing.T) {
	kho := &giaoViecGia{traLai: []string{"CB-A", "CB-A", "CB-KHONG-HOI", ""}}
	ra, err := goiGiaoViec(t, kho, ctxXa(xaA), []string{"CB-A"})
	if err != nil {
		t.Fatalf("ResolveAssignableStaff: %v", err)
	}
	if got := ra.GetAssignableMa(); len(got) != 1 || got[0] != "CB-A" {
		t.Fatalf("trả %v, muốn [CB-A] — không được mang mã không hỏi, không được lặp", got)
	}
}

// Quá 50 là INVALID_ARGUMENT, KHÔNG cắt bớt, và KHÔNG chạm kho. Trần tính trên số mã ĐÃ GỬI.
func TestGiaoViecQuaTranLaInvalidArgument(t *testing.T) {
	kho := &giaoViecGia{}
	ma := make([]string, TranMaGiaoViecMotLo+1)
	for i := range ma {
		ma[i] = "CB-TRUNG"
	}
	_, err := goiGiaoViec(t, kho, ctxXa(xaA), ma)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
	if kho.soLanGoi != 0 {
		t.Error("đã đọc kho dù vượt trần")
	}
}

// Đúng bằng trần KHÔNG bị từ chối.
func TestGiaoViecDungBangTranThiDuoc(t *testing.T) {
	kho := &giaoViecGia{}
	ma := make([]string, TranMaGiaoViecMotLo)
	for i := range ma {
		ma[i] = "CB-" + strings.Repeat("X", i+1)
	}
	if _, err := goiGiaoViec(t, kho, ctxXa(xaA), ma); err != nil {
		t.Fatalf("đúng bằng trần mà bị từ chối: %v", err)
	}
}

// Rỗng trả rỗng và KHÔNG chạm kho — kể cả `ma: [""]`. Không có cách viết nào nghĩa là "mọi người".
func TestGiaoViecRongTraRongKhongDocKho(t *testing.T) {
	for ten, ma := range map[string][]string{"nil": nil, "chuỗi rỗng": {"", ""}} {
		t.Run(ten, func(t *testing.T) {
			kho := &giaoViecGia{traLai: []string{"CB-KHONG-HOI"}}
			ra, err := goiGiaoViec(t, kho, ctxXa(xaA), ma)
			if err != nil {
				t.Fatalf("ResolveAssignableStaff: %v", err)
			}
			if len(ra.GetAssignableMa()) != 0 {
				t.Fatalf("trả %v, muốn rỗng", ra.GetAssignableMa())
			}
			if kho.soLanGoi != 0 {
				t.Error("danh sách rỗng vẫn đọc kho")
			}
		})
	}
}

// Không có mã nào giao được là OK rỗng — KHÔNG BAO GIỜ NOT_FOUND.
func TestGiaoViecKhongMaNaoLaOKRongKhongPhaiNotFound(t *testing.T) {
	ra, err := goiGiaoViec(t, &giaoViecGia{}, ctxXa(xaA), []string{"CB-KHONG-TON-TAI"})
	if err != nil {
		t.Fatalf("mã = %v, muốn OK rỗng", status.Code(err))
	}
	if len(ra.GetAssignableMa()) != 0 {
		t.Fatalf("trả %v", ra.GetAssignableMa())
	}
}

// Thiếu xã → Internal, KHÔNG panic, KHÔNG đọc kho.
func TestGiaoViecThieuXaLaInternalChuKhongPanic(t *testing.T) {
	kho := &giaoViecGia{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()
	_, err := goiGiaoViec(t, kho, context.Background(), []string{"CB-A"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if kho.soLanGoi != 0 {
		t.Error("ĐÃ ĐỌC KHO KHI KHÔNG BIẾT XÃ NÀO")
	}
}

// Kho hỏng → Internal, không phải "không ai giao được". Không lộ mã nào trong nhật ký.
func TestGiaoViecKhoHongLaInternal(t *testing.T) {
	kho := &giaoViecGia{err: loiKho}
	s, nhatKy := may(t, func(d *Deps) { d.GiaoViec = kho })
	_, err := s.ResolveAssignableStaff(ctxXa(xaA),
		&identityv1.ResolveAssignableStaffRequest{Ma: []string{"CB-BI-MAT-01"}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if strings.Contains(nhatKy.String(), "CB-BI-MAT-01") {
		t.Error("nhật ký chứa mã cán bộ — chỉ được ghi SỐ LƯỢNG")
	}
}
