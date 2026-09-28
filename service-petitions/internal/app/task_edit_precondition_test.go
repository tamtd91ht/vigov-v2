package app

import (
	"errors"
	"testing"
	"time"

	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Optimistic locking on PATCH /api/v1/tasks/{ma} (28/09/2026), over the real store on the fake driver.
// The fixture row's `cap_nhat_luc` is mocSuaNV (with microseconds).

func titleEdit(expected *time.Time) petstore.SuaNhiemVu {
	title := "Rà soát tiến độ tuyến đường Hà Lam – Bình Trị (bản sửa)"
	return petstore.SuaNhiemVu{TieuDe: &title, ExpectedUpdatedAt: expected}
}

func TestEditPrecondition_MatchApplies(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	expected := mocSuaNV

	sau, err := uc.Sua(ctx, maNVGoc, titleEdit(&expected), canBoThu())
	if err != nil {
		t.Fatalf("sửa với đúng mốc: %v", err)
	}
	if len(k.cau("UPDATE nhiem_vu")) != 1 || len(k.cau("INSERT INTO audit_log")) != 1 {
		t.Error("đúng mốc mà không ghi")
	}
	// THE REPLY'S TOKEN IS READ AFTER THE WRITE (refreshUpdatedAt), never the pre-write value.
	if !k.coCau("SELECT cap_nhat_luc FROM nhiem_vu") {
		t.Error("phản hồi không đọc lại cap_nhat_luc sau khi ghi — lần PATCH kế tiếp sẽ tự trượt")
	}
	if sau.UpdatedAt.IsZero() {
		t.Error("phản hồi không mang updated_at")
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestEditPrecondition_ComparedAtColumnPrecision: the column keeps microseconds; a token that differs
// only below that is the same stored instant.
func TestEditPrecondition_ComparedAtColumnPrecision(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	expected := mocSuaNV.Add(700 * time.Nanosecond)
	if _, err := uc.Sua(ctx, maNVGoc, titleEdit(&expected), canBoThu()); err != nil {
		t.Fatalf("khác dưới micro giây mà bị từ chối: %v", err)
	}
}

func TestEditPrecondition_MismatchIs409AndWritesNothing(t *testing.T) {
	for name, expected := range map[string]time.Time{
		"cũ hơn một micro giây": mocSuaNV.Add(-time.Microsecond),
		"mới hơn":               mocSuaNV.Add(time.Second),
		"mốc tạo":               mocTaoNV,
	} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			_, err := uc.Sua(ctx, maNVGoc, titleEdit(&expected), canBoThu())
			if !errors.Is(err, ErrTaskEditConflict) {
				t.Fatalf("lỗi = %v, muốn ErrTaskEditConflict", err)
			}
			khongGhiGi(t, k)
		})
	}
}

// TestEditPrecondition_StaleNoOpStillConflicts: checked BEFORE the no-change shortcut, so a stale
// screen is told to reload even when its body would change nothing.
func TestEditPrecondition_StaleNoOpStillConflicts(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	stale := mocSuaNV.Add(-time.Hour)
	_, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{ExpectedUpdatedAt: &stale}, canBoThu())
	if !errors.Is(err, ErrTaskEditConflict) {
		t.Fatalf("lỗi = %v, muốn ErrTaskEditConflict", err)
	}
}

// TestEditPrecondition_AbsentKeepsTodaysBehaviour: no token, no check — additive to a published contract.
func TestEditPrecondition_AbsentKeepsTodaysBehaviour(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.Sua(ctx, maNVGoc, titleEdit(nil), canBoThu()); err != nil {
		t.Fatalf("sửa không kèm mốc: %v", err)
	}
	if len(k.cau("UPDATE nhiem_vu")) != 1 {
		t.Error("không kèm mốc mà không ghi")
	}
}

// TestCreate_WritesAndRepliesTheSameToken: the INSERT writes `cap_nhat_luc` itself, at microsecond
// precision, and the reply carries that same value — so a client can PATCH right after creating.
func TestCreate_WritesAndRepliesTheSameToken(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.nay = func() time.Time { return mocThaoTacNV.Add(123456789 * time.Nanosecond) }

	n, err := uc.Tao(ctx, taoMau(), canBoThu())
	if err != nil {
		t.Fatalf("giao việc: %v", err)
	}
	want := mocThaoTacNV.Add(123456 * time.Microsecond)
	if !n.UpdatedAt.Equal(want) {
		t.Errorf("updated_at phản hồi = %v, muốn %v (cắt về micro giây)", n.UpdatedAt, want)
	}
	ins := k.cau("INSERT INTO nhiem_vu")
	if len(ins) != 1 {
		t.Fatalf("chạy %d câu INSERT nhiem_vu, muốn 1", len(ins))
	}
	if got, ok := ins[0].args[23].(time.Time); !ok || !got.Equal(want) {
		t.Errorf("INSERT ghi cap_nhat_luc = %v, muốn %v", ins[0].args[23], want)
	}
}
