package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/vihat/vigov/core/documentsclient"
	"github.com/vihat/vigov/core/petitionsclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// SoDoToChuc.Remove over the REAL store and a REAL transaction boundary (driver_gia_bo_phan_test.go),
// with the two owners faked at the interface the use case declares.

type fakePetitionHoldings struct {
	answer   petitionsclient.OrgUnitHoldings
	err      error
	calls    int
	sawID    string
	sawXa    tenant.ID
	sawXaSet bool
}

func (f *fakePetitionHoldings) OrgUnitHoldings(ctx context.Context, id string) (petitionsclient.OrgUnitHoldings, error) {
	f.calls++
	f.sawID = id
	f.sawXa, f.sawXaSet = tenant.From(ctx)
	return f.answer, f.err
}

type fakeDocumentHoldings struct {
	answer documentsclient.OrgUnitHoldings
	err    error
	calls  int
	sawID  string
}

func (f *fakeDocumentHoldings) OrgUnitHoldings(_ context.Context, id string) (documentsclient.OrgUnitHoldings, error) {
	f.calls++
	f.sawID = id
	return f.answer, f.err
}

// removeSetup builds the use case over the fake database, connected to two fake owners. bp-c is a
// leaf (TỔ MỘT CỬA) so it holds nothing by default.
func removeSetup(t *testing.T, k *khoBoPhanGia) (*SoDoToChuc, context.Context, *fakePetitionHoldings, *fakeDocumentHoldings) {
	t.Helper()
	uc, ctx := dungUseCaseBoPhan(t, k)
	p, d := &fakePetitionHoldings{}, &fakeDocumentHoldings{}
	uc.WithHoldingsSources(p, d)
	return uc, ctx, p, d
}

func TestRemoveOrgUnitSoftDeletesAndAuditsInOneTransaction(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx, p, d := removeSetup(t, k)

	if err := uc.Remove(ctx, "bp-c", "  sáp nhập vào Văn phòng  ", nguoiBoPhan()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if p.calls != 1 || d.calls != 1 || p.sawID != "bp-c" || d.sawID != "bp-c" {
		t.Errorf("hỏi chủ sở hữu: petitions %d lần (%q), documents %d lần (%q)", p.calls, p.sawID, d.calls, d.sawID)
	}
	if !p.sawXaSet || p.sawXa != xaBoPhan {
		t.Errorf("lời hỏi petitions không mang xã của yêu cầu: %q", p.sawXa)
	}
	upd := k.cau("UPDATE bo_phan SET deleted_at")
	if len(upd) != 1 {
		t.Fatalf("có %d câu xoá mềm, muốn 1", len(upd))
	}
	a := upd[0].args
	if a[0] != string(xaBoPhan) || a[1] != "bp-c" || a[2] != maCanBoBoPhan || a[3] != "sáp nhập vào Văn phòng" {
		t.Errorf("tham số xoá mềm = %v — muốn xã, id, MÃ CÁN BỘ (không phải id nội bộ), lý do đã cắt", a)
	}
	if len(k.cau("DELETE")) != 0 {
		t.Error("có câu DELETE — bộ phận là hồ sơ lưu trữ, chỉ xoá mềm (luật 7)")
	}
	delta := motVetBoPhan(t, k, ActionDeleteOrgUnit, "to-mot-cua")
	if n, _ := delta["do_dai_ly_do"].(float64); int(n) != len([]rune("sáp nhập vào Văn phòng")) {
		t.Errorf("do_dai_ly_do = %v", delta["do_dai_ly_do"])
	}
	for _, v := range delta {
		if s, ok := v.(string); ok && s == "sáp nhập vào Văn phòng" {
			t.Error("lý do nằm nguyên văn trong delta — chỉ được ghi độ dài")
		}
	}
}

// EACH KIND REFUSES ON ITS OWN, with its count, and nothing is opened, locked or written.
func TestRemoveOrgUnitRefusedForEachKind(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(k *khoBoPhanGia, p *fakePetitionHoldings, d *fakeDocumentHoldings)
		want  domain.OrgUnitHoldings
	}{
		{"staff", func(k *khoBoPhanGia, _ *fakePetitionHoldings, _ *fakeDocumentHoldings) {
			k.staff = map[string]int{"bp-c": 2}
		}, domain.OrgUnitHoldings{Staff: 2}},
		{"child unit", func(k *khoBoPhanGia, _ *fakePetitionHoldings, _ *fakeDocumentHoldings) {
			k.hang = append(k.hang, hangBoPhan{xa: xaBoPhan, id: "bp-d", ma: "to-con", ten: "TỔ CON", cha: "bp-c"})
		}, domain.OrgUnitHoldings{ChildUnits: 1}},
		{"open petitions", func(_ *khoBoPhanGia, p *fakePetitionHoldings, _ *fakeDocumentHoldings) {
			p.answer.OpenPetitions = 4
		}, domain.OrgUnitHoldings{OpenPetitions: 4}},
		{"open tasks", func(_ *khoBoPhanGia, p *fakePetitionHoldings, _ *fakeDocumentHoldings) {
			p.answer.OpenTasks = 5
		}, domain.OrgUnitHoldings{OpenTasks: 5}},
		{"open incoming documents", func(_ *khoBoPhanGia, _ *fakePetitionHoldings, d *fakeDocumentHoldings) {
			d.answer.OpenIncomingDocuments = 6
		}, domain.OrgUnitHoldings{OpenIncomingDocuments: 6}},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := &khoBoPhanGia{hang: cayBaTang()}
			uc, ctx, p, d := removeSetup(t, k)
			c.setup(k, p, d)

			err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan())
			var inUse *OrgUnitInUseError
			if !errors.As(err, &inUse) {
				t.Fatalf("lỗi = %v, muốn *OrgUnitInUseError", err)
			}
			if inUse.Holdings != c.want {
				t.Errorf("số còn giữ = %+v, muốn %+v", inUse.Holdings, c.want)
			}
			if k.batDau != 0 || len(k.cau("UPDATE")) != 0 || len(k.cau("INSERT")) != 0 {
				t.Errorf("bị từ chối mà vẫn mở giao dịch (%d) hoặc ghi", k.batDau)
			}
		})
	}
}

// AN OUTAGE IS NEVER ZERO: either owner failing refuses the delete with a 503-class error, before
// any transaction, and the owner's own sentinel survives the wrap.
func TestRemoveOrgUnitOwnerErrorWritesNothing(t *testing.T) {
	down := fmt.Errorf("petitionsclient: CountOrgUnitHoldings: %w", petitionsclient.ErrPetitionsUnavailable)
	for _, c := range []struct {
		name     string
		petErr   error
		docErr   error
		sentinel error
	}{
		{"petitions down", down, nil, petitionsclient.ErrPetitionsUnavailable},
		{"documents down", nil, documentsclient.ErrDocumentsUnavailable, documentsclient.ErrDocumentsUnavailable},
		{"petitions misconfigured", errors.New("caller key refused"), nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := &khoBoPhanGia{hang: cayBaTang()}
			uc, ctx, p, d := removeSetup(t, k)
			p.err, d.err = c.petErr, c.docErr

			err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan())
			if !errors.Is(err, ErrOrgUnitHoldingsUnavailable) {
				t.Fatalf("lỗi = %v, muốn ErrOrgUnitHoldingsUnavailable", err)
			}
			if c.sentinel != nil && !errors.Is(err, c.sentinel) {
				t.Errorf("mất lỗi gốc %v khi bọc: %v", c.sentinel, err)
			}
			if k.batDau != 0 || len(k.cau("UPDATE")) != 0 {
				t.Error("chủ sở hữu không trả lời mà vẫn mở giao dịch hoặc xoá")
			}
		})
	}
}

// THE AUDIT ENTRY FAILING TAKES THE SOFT DELETE WITH IT (rule 6, invariant 3).
func TestRemoveOrgUnitAuditFailureRollsBack(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang(), loiSau: "INSERT INTO audit_log"}
	uc, ctx, _, _ := removeSetup(t, k)

	if err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan()); err == nil {
		t.Fatal("vết hỏng mà xoá vẫn thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — muốn 0 và 1", k.daCommit, k.daRollback)
	}
}

// RE-COUNTED UNDER THE LOCK: a member of staff who arrived after the first read refuses the delete
// inside the transaction, which rolls back without writing.
func TestRemoveOrgUnitRecountsUnderLock(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang(), staffUnderLock: map[string]int{"bp-c": 1}}
	uc, ctx, _, _ := removeSetup(t, k)

	err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan())
	var inUse *OrgUnitInUseError
	if !errors.As(err, &inUse) || inUse.Holdings.Staff != 1 {
		t.Fatalf("lỗi = %v, muốn *OrgUnitInUseError với 1 cán bộ", err)
	}
	if len(k.cau("FOR UPDATE")) != 1 {
		t.Error("không khoá dòng trước khi đếm lại")
	}
	if len(k.cau("UPDATE bo_phan SET deleted_at")) != 0 || k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("đếm lại thấy cán bộ mà vẫn ghi (commit %d, rollback %d)", k.daCommit, k.daRollback)
	}
}

// Deleted by somebody else between the read and the lock: the same 404, nothing written.
func TestRemoveOrgUnitDeletedBeforeLockIsNotFound(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang(), deletedAtLock: true}
	uc, ctx, _, _ := removeSetup(t, k)

	if err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan()); !errors.Is(err, idstore.ErrKhongTimThayBoPhan) {
		t.Fatalf("lỗi = %v, muốn ErrKhongTimThayBoPhan", err)
	}
	if len(k.cau("UPDATE bo_phan SET deleted_at")) != 0 || k.daCommit != 0 {
		t.Error("đã bị xoá trước mà vẫn ghi đè")
	}
}

// ANOTHER COMMUNE'S UNIT, AN ALREADY-DELETED ONE, AN INVENTED ONE: one answer, and the owners are
// not even asked — a 404 leaks nothing and costs nothing.
func TestRemoveOrgUnitNotFoundAnswersAlike(t *testing.T) {
	for _, id := range []string{"bp-khac", "bp-xoa", "bp-khong-co", ""} {
		k := &khoBoPhanGia{hang: cayBaTang()}
		uc, ctx, p, d := removeSetup(t, k)
		if err := uc.Remove(ctx, id, "sáp nhập", nguoiBoPhan()); !errors.Is(err, idstore.ErrKhongTimThayBoPhan) {
			t.Errorf("%q: lỗi = %v, muốn ErrKhongTimThayBoPhan", id, err)
		}
		if p.calls+d.calls != 0 || k.batDau != 0 {
			t.Errorf("%q: không tìm thấy mà vẫn hỏi chủ sở hữu hoặc mở giao dịch", id)
		}
	}
}

func TestRemoveOrgUnitReasonRequiredBeforeAnything(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx, p, _ := removeSetup(t, k)
	if err := uc.Remove(ctx, "bp-c", "   ", nguoiBoPhan()); !errors.Is(err, domain.ErrOrgUnitDeleteReasonMissing) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitDeleteReasonMissing", err)
	}
	if !LaLoiDauVaoBoPhan(domain.ErrOrgUnitDeleteReasonMissing) {
		t.Error("thiếu lý do phải là lỗi đầu vào (400)")
	}
	if len(k.lenh) != 0 || p.calls != 0 {
		t.Error("thiếu lý do mà vẫn đọc hoặc hỏi chủ sở hữu")
	}
}

// NOT CONFIGURED: no address for an owner → a named refusal, never a delete on half the question.
func TestRemoveOrgUnitWithoutSourcesIsNotConfigured(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)
	if err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan()); !errors.Is(err, ErrOrgUnitDeleteNotConfigured) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitDeleteNotConfigured", err)
	}
	uc.WithHoldingsSources(&fakePetitionHoldings{}, nil)
	if err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan()); !errors.Is(err, ErrOrgUnitDeleteNotConfigured) {
		t.Fatalf("chỉ có petitions: lỗi = %v, muốn ErrOrgUnitDeleteNotConfigured", err)
	}
	if len(k.lenh) != 0 {
		t.Error("chưa cấu hình mà vẫn đọc cơ sở dữ liệu")
	}
}

// A SOFT-DELETED CHILD HOLDS NOTHING: only live children block.
func TestRemoveOrgUnitIgnoresDeletedChildren(t *testing.T) {
	k := &khoBoPhanGia{hang: append(cayBaTang(),
		hangBoPhan{xa: xaBoPhan, id: "bp-d", ma: "to-con-cu", ten: "TỔ CON CŨ", cha: "bp-c", daXoa: true})}
	uc, ctx, _, _ := removeSetup(t, k)
	if err := uc.Remove(ctx, "bp-c", "sáp nhập", nguoiBoPhan()); err != nil {
		t.Fatalf("con đã xoá mềm mà vẫn chặn: %v", err)
	}
}
