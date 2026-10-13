package store

import (
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Several roles per person (migration 0031, owner decision 13/10/2026), over the fake engine of
// can_bo_danh_sach_test.go. What only PostgreSQL can prove — the union in Checker, the backfill, the
// FOR UPDATE on the assignment row — is in staff_role_assignment_pg_test.go.

// withRoles gives one fixture row a role SET for the duration of a test.
func withRoles(t *testing.T, id string, roles []string) {
	t.Helper()
	for i := range dsDuLieu {
		if dsDuLieu[i].id == id {
			old := dsDuLieu[i].roles
			dsDuLieu[i].roles = roles
			t.Cleanup(func() { dsDuLieu[i].roles = old })
			return
		}
	}
	t.Fatalf("không có dòng mẫu %s", id)
}

// A KEY HELD THROUGH THE SECOND ROLE COUNTS. CB-005's primary role vt-009 is soft-deleted; given
// vt-001 as a SECOND role, they hold task.extend and the picker's permission filter — the checker's
// own join, noiGiuQuyen — must offer them.
//
// MUTATION THAT MUST TURN THIS RED: read grants off `nd.vai_tro_id` again.
func TestPickerPermissionFilterSeesSecondRole(t *testing.T) {
	withRoles(t, "nd-05", []string{"vt-009", "vt-001"})
	b := moBanThuDS(t)

	if got := maGiuQuyen(t, b, xaMau, domain.LocChonNguoi{QuyenMa: "task.extend"}); got != "CB-001,CB-005" {
		t.Fatalf("người giữ task.extend = %q, muốn CB-001,CB-005 — CB-005 giữ quyền qua vai trò thứ hai", got)
	}
}

// THE REGISTER READ CARRIES THE WHOLE SET, primary first; a person with no role reads [] (never nil).
func TestRegisterReadCarriesRoleIDs(t *testing.T) {
	withRoles(t, "nd-01", []string{"vt-001", "vt-002"})
	b := moBanThuDS(t)

	cb, err := b.kho.ChiTiet(ctxXa(xaMau), "nd-01")
	if err != nil {
		t.Fatalf("ChiTiet: %v", err)
	}
	if strings.Join(cb.RoleIDs, ",") != "vt-001,vt-002" || cb.VaiTroID != "vt-001" {
		t.Errorf("RoleIDs = %v, VaiTroID = %q", cb.RoleIDs, cb.VaiTroID)
	}
	none, err := b.kho.ChiTiet(ctxXa(xaMau), "nd-02")
	if err != nil {
		t.Fatalf("ChiTiet: %v", err)
	}
	if none.RoleIDs == nil || len(none.RoleIDs) != 0 {
		t.Errorf("người không vai trò: RoleIDs = %#v, muốn []", none.RoleIDs)
	}
}

// THE LAST-ADMINISTRATOR SET NAMES EACH PERSON ONCE. One administrator holding admin.user through two
// roles is two rows of the locked query; counted as two, #13 would let the commune's last
// administrator be locked out.
//
// MUTATION THAT MUST TURN THIS RED: return every row's id from QuanTriDangHoatDong.
func TestDistinctHolderIDsCollapsesTwoRolesOfOnePerson(t *testing.T) {
	got := distinctHolderIDs([]domain.NguoiGiuQuyen{
		{CanBoID: "nd-1", VaiTroID: "vt-a"},
		{CanBoID: "nd-1", VaiTroID: "vt-b"},
	})
	if len(got) != 1 || got[0] != "nd-1" {
		t.Fatalf("= %v, muốn [nd-1] — một người hai vai trò bị đếm thành hai quản trị viên", got)
	}
}

// THE CHECKER'S JOIN READS THE ASSIGNMENT TABLE, live rows only, bound to the commune — by text,
// because every reader of "who holds a key" is built from this one constant.
func TestHolderJoinReadsLiveAssignments(t *testing.T) {
	for _, want := range []string{
		"JOIN staff_role_assignment sra ON sra.tenant_id = nd.tenant_id AND sra.staff_id = nd.id",
		"AND sra.deleted_at IS NULL",
		"vt.id         = sra.role_id",
	} {
		if !strings.Contains(noiGiuQuyen, want) {
			t.Errorf("noiGiuQuyen thiếu %q", want)
		}
	}
	if strings.Contains(noiGiuQuyen, "nd.vai_tro_id") {
		t.Error("noiGiuQuyen còn đọc nd.vai_tro_id — vai trò thứ hai sẽ không cấp quyền")
	}
	if !strings.HasPrefix(strings.TrimSpace(truyVanQuyenGoc), "SELECT DISTINCT") {
		t.Error("danh sách quyền không DISTINCT — hai vai trò cùng cấp một khoá sẽ in khoá hai lần")
	}
}
