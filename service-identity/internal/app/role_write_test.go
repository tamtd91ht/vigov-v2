package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the commune's own role writes (owner decision 15/10/2026). Every refusal
// here fails SILENTLY when removed — a second "Bí thư", a renamed administrator role, a role deleted
// from under the people holding it — so each one has a case.
//
// It runs on the fake database/sql driver of dang_nhap_giao_dich_test.go; the fake repo's writes run a
// REAL statement through the transaction they were handed, so "the write and its entry share one
// transaction" is an assertion about what actually ran.
//
// WHAT IT CANNOT PROVE: the advisory lock serialising two creations, FOR UPDATE / FOR SHARE ordering
// against a concurrent assignment, the constraint names, and the SQL itself — those are in
// store/role_write_pg_test.go (VIGOV_TEST_DSN).

type roleRow struct {
	domain.ExistingRole
	deleted bool
	holders int
}

type roleWriteRepoFake struct {
	calls []string
	rows  []roleRow
}

func (r *roleWriteRepoFake) LockCatalogue(context.Context, *store.ScopedTx) error {
	r.calls = append(r.calls, "lock-catalogue")
	return nil
}

func (r *roleWriteRepoFake) LiveRoles(context.Context, *store.ScopedTx) ([]domain.ExistingRole, error) {
	r.calls = append(r.calls, "live")
	var out []domain.ExistingRole
	for _, x := range r.rows {
		if !x.deleted {
			out = append(out, x.ExistingRole)
		}
	}
	return out, nil
}

func (r *roleWriteRepoFake) CodesWithBase(_ context.Context, _ *store.ScopedTx, base string) ([]string, error) {
	r.calls = append(r.calls, "codes:"+base)
	var out []string
	for _, x := range r.rows {
		if x.Code == base || strings.HasPrefix(x.Code, base+"-") {
			out = append(out, x.Code)
		}
	}
	return out, nil
}

func (r *roleWriteRepoFake) InsertRole(ctx context.Context, tx *store.ScopedTx, x domain.ExistingRole) error {
	r.calls = append(r.calls, "insert:"+x.Code)
	r.rows = append(r.rows, roleRow{ExistingRole: x})
	_, err := tx.Exec(ctx, "GHI-GIA insert vai_tro", string(tx.TenantID()), x.ID, x.Code)
	return err
}

func (r *roleWriteRepoFake) find(id string) *roleRow {
	for i := range r.rows {
		if r.rows[i].ID == id && !r.rows[i].deleted {
			return &r.rows[i]
		}
	}
	return nil
}

func (r *roleWriteRepoFake) LockRole(_ context.Context, _ *store.ScopedTx, id string) (domain.ExistingRole, error) {
	r.calls = append(r.calls, "lock-role:"+id)
	if x := r.find(id); x != nil {
		return x.ExistingRole, nil
	}
	return domain.ExistingRole{}, idstore.ErrVaiTroKhongTonTaiDeGhi
}

func (r *roleWriteRepoFake) UpdateRole(ctx context.Context, tx *store.ScopedTx, id, name string, isLeader bool) error {
	r.calls = append(r.calls, "update:"+id)
	x := r.find(id)
	x.Name, x.IsLeader = name, isLeader
	_, err := tx.Exec(ctx, "GHI-GIA update vai_tro", string(tx.TenantID()), id, name)
	return err
}

func (r *roleWriteRepoFake) CountHolders(_ context.Context, _ *store.ScopedTx, id string) (int, error) {
	r.calls = append(r.calls, "holders:"+id)
	return r.find(id).holders, nil
}

func (r *roleWriteRepoFake) SoftDeleteRole(ctx context.Context, tx *store.ScopedTx, id, by, reason string) (time.Time, error) {
	r.calls = append(r.calls, "soft-delete:"+id+":"+by)
	r.find(id).deleted = true
	_, err := tx.Exec(ctx, "GHI-GIA soft-delete vai_tro", string(tx.TenantID()), id, by, reason)
	return time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC), err
}

type roleWriteBench struct {
	uc   *RoleCatalogue
	repo *roleWriteRepoFake
	ghi  *ghiChep
}

// newRoleWriteBench: a commune holding the administrator role, Chủ tịch UBND, and a soft-deleted
// "Bí thư" (code bi-thu) — so a new "Bí thư" must take bi-thu-2.
func newRoleWriteBench(t *testing.T) *roleWriteBench {
	t.Helper()
	db, g := moDB(t)
	repo := &roleWriteRepoFake{rows: []roleRow{
		{ExistingRole: domain.ExistingRole{ID: "vt-admin", Code: domain.CommuneAdminRoleCode, Name: "Quản trị hệ thống", Order: 0}},
		{ExistingRole: domain.ExistingRole{ID: "vt-ct", Code: "chu-tich-ubnd", Name: "Chủ tịch UBND", IsLeader: true, Order: 1}, holders: 1},
		{ExistingRole: domain.ExistingRole{ID: "vt-bt-cu", Code: "bi-thu", Name: "Bí thư", Order: 2}, deleted: true},
		{ExistingRole: domain.ExistingRole{ID: "vt-kt", Code: "ke-toan", Name: "Kế toán", Order: 6}},
	}}
	uc := NewRoleCatalogue(db, repo)
	uc.newID = func() (string, error) { return "vt-moi", nil }
	return &roleWriteBench{uc: uc, repo: repo, ghi: g}
}

// oneEntryInTheWriteTx asserts exactly one audit entry carrying `action`, in the same committed
// transaction as every GHI-GIA write.
func oneEntryInTheWriteTx(t *testing.T, b *roleWriteBench, action string) string {
	t.Helper()
	vet := vetDaGhi(b.ghi)
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	tx := vet[0].tx
	if tx == 0 {
		t.Fatal("vết ghi ngoài giao dịch")
	}
	b.ghi.mu.Lock()
	for _, l := range b.ghi.lenh {
		if strings.Contains(l.sql, "GHI-GIA") && l.tx != tx {
			t.Errorf("câu %q ở giao dịch %d, vết ở %d — luật 6 bất biến 3", l.sql, l.tx, tx)
		}
	}
	b.ghi.mu.Unlock()
	if ket := b.ghi.ketThucCua(tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q", ket)
	}
	var parts []string
	for _, a := range vet[0].args {
		switch v := a.(type) {
		case []byte:
			parts = append(parts, string(v))
		default:
			parts = append(parts, fmt.Sprint(v))
		}
	}
	all := strings.Join(parts, "|")
	if !strings.Contains(all, action) || !strings.Contains(all, maCanBo) || strings.Contains(all, idNoiBo) {
		t.Fatalf("vết = %s — muốn hành vi %q, người thực hiện là mã %q, không phải id nội bộ", all, action, maCanBo)
	}
	return all
}

func assertRoleNothingWritten(t *testing.T, b *roleWriteBench) {
	t.Helper()
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Errorf("có %d vết dù không được ghi gì", n)
	}
	if b.ghi.tim("GHI-GIA") != nil {
		t.Error("vẫn ghi vai trò dù bị từ chối")
	}
}

// --- create ---------------------------------------------------------------------------------------

// The code skips a SOFT-DELETED role's code (bi-thu → bi-thu-2), the role is last on the matrix, holds
// nothing, and the insert and its entry share one transaction.
func TestRoleAddDerivesCodePastDeletedAndAuditsInOneTx(t *testing.T) {
	b := newRoleWriteBench(t)
	got, err := b.uc.Add(ctxXa(xaThu), RoleInput{Name: "  Bí thư ", IsLeader: true}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got.ID != "vt-moi" || got.Ma != "bi-thu-2" || got.Ten != "Bí thư" || !got.LaLanhDao {
		t.Fatalf("vai trò = %+v", got)
	}
	created := b.repo.find("vt-moi")
	if created.Order != 7 {
		t.Errorf("thu_tu = %d, muốn 7 (sau vai trò đang dùng cuối cùng)", created.Order)
	}
	all := oneEntryInTheWriteTx(t, b, ActionAddRole)
	if !strings.Contains(all, `"quyen":[]`) {
		t.Errorf("vết phải ghi rõ vai trò mới chưa có quyền: %s", all)
	}
	if b.repo.calls[0] != "lock-catalogue" {
		t.Errorf("khoá danh mục phải đi trước mọi lượt đọc: %v", b.repo.calls)
	}
}

// A name whose slug is the administrator code never takes it.
func TestRoleAddNeverIssuesTheAdministratorCode(t *testing.T) {
	b := newRoleWriteBench(t)
	// The fake commune holds `quan-tri-he-thong`; remove it so the code is free — it must STILL be skipped.
	b.repo.rows = b.repo.rows[1:]
	got, err := b.uc.Add(ctxXa(xaThu), RoleInput{Name: "Quản trị Hệ thống"}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got.Ma != domain.CommuneAdminRoleCode+"-2" {
		t.Fatalf("mã = %q, muốn %q", got.Ma, domain.CommuneAdminRoleCode+"-2")
	}
}

func TestRoleAddRefusals(t *testing.T) {
	cases := []struct {
		name string
		in   RoleInput
		prep func(*roleWriteBench)
		want error
	}{
		{"duplicate live name, other case", RoleInput{Name: "KẾ TOÁN"}, nil, domain.ErrRoleNameTaken},
		// "Kế toán" with the ế decomposed (e + U+0302 + U+0301): NFC makes it the same name.
		{"duplicate after NFC", RoleInput{Name: "Kế toán"}, nil, domain.ErrRoleNameTaken},
		{"blank", RoleInput{Name: "   "}, nil, domain.ErrRoleNameInvalid},
		{"too long", RoleInput{Name: strings.Repeat("ạ", domain.MaxRoleName+1)}, nil, domain.ErrRoleNameInvalid},
		{"control char", RoleInput{Name: "Bí\nthư"}, nil, domain.ErrRoleNameInvalid},
		{"no letter or digit", RoleInput{Name: "!!!"}, nil, domain.ErrRoleNameInvalid},
		{"catalogue full", RoleInput{Name: "Bí thư"}, func(b *roleWriteBench) {
			for i := len(b.repo.rows); i <= idstore.TranDanhMucVaiTro; i++ {
				b.repo.rows = append(b.repo.rows, roleRow{ExistingRole: domain.ExistingRole{
					ID: fmt.Sprintf("vt-%03d", i), Code: fmt.Sprintf("r-%03d", i), Name: fmt.Sprintf("Vai trò %d", i)}})
			}
		}, ErrRoleCatalogueFull},
	}
	for _, c := range cases {
		b := newRoleWriteBench(t)
		if c.prep != nil {
			c.prep(b)
		}
		if _, err := b.uc.Add(ctxXa(xaThu), c.in, nguoiThucHienGia()); !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		assertRoleNothingWritten(t, b)
	}
}

// A soft-deleted role's NAME is free again (its code is not).
func TestRoleAddReusesADeletedRolesName(t *testing.T) {
	b := newRoleWriteBench(t)
	if _, err := b.uc.Add(ctxXa(xaThu), RoleInput{Name: "bí thư"}, nguoiThucHienGia()); err != nil {
		t.Fatalf("tên của vai trò đã xoá phải dùng lại được: %v", err)
	}
}

// --- edit -----------------------------------------------------------------------------------------

func TestRoleEditWritesBeforeAndAfterInOneTx(t *testing.T) {
	b := newRoleWriteBench(t)
	got, err := b.uc.Edit(ctxXa(xaThu), "vt-kt", RoleInput{Name: "Kế toán ngân sách", IsLeader: true}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got.Ma != "ke-toan" || got.Ten != "Kế toán ngân sách" || !got.LaLanhDao {
		t.Fatalf("vai trò = %+v — mã không bao giờ đổi", got)
	}
	all := oneEntryInTheWriteTx(t, b, ActionEditRole)
	for _, want := range []string{`"truoc":{"la_lanh_dao":false,"ten":"Kế toán"}`, `"sau":{"la_lanh_dao":true,"ten":"Kế toán ngân sách"}`} {
		if !strings.Contains(all, want) {
			t.Errorf("vết thiếu %s: %s", want, all)
		}
	}
}

// Unchanged → nothing written, nothing audited (the route's idem.KhongCan rests on it).
func TestRoleEditNoOpWritesNothing(t *testing.T) {
	b := newRoleWriteBench(t)
	got, err := b.uc.Edit(ctxXa(xaThu), "vt-kt", RoleInput{Name: " Kế toán "}, nguoiThucHienGia())
	if err != nil || got.Ten != "Kế toán" {
		t.Fatalf("Edit = %+v, %v", got, err)
	}
	assertRoleNothingWritten(t, b)
}

// Renaming to its own name in another case is a change, not a duplicate of itself.
func TestRoleEditCaseOnlyChangeIsNotADuplicate(t *testing.T) {
	b := newRoleWriteBench(t)
	if _, err := b.uc.Edit(ctxXa(xaThu), "vt-kt", RoleInput{Name: "KẾ TOÁN"}, nguoiThucHienGia()); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	oneEntryInTheWriteTx(t, b, ActionEditRole)
}

func TestRoleEditRefusals(t *testing.T) {
	cases := []struct {
		name, id string
		in       RoleInput
		want     error
	}{
		{"administrator role", "vt-admin", RoleInput{Name: "Bí thư"}, ErrSystemRole},
		{"administrator role, unchanged", "vt-admin", RoleInput{Name: "Quản trị hệ thống"}, ErrSystemRole},
		{"another live role's name", "vt-kt", RoleInput{Name: "chủ tịch ubnd"}, domain.ErrRoleNameTaken},
		{"soft-deleted role", "vt-bt-cu", RoleInput{Name: "Bí thư"}, idstore.ErrVaiTroKhongTonTaiDeGhi},
		{"unknown id", "vt-khong-co", RoleInput{Name: "Bí thư"}, idstore.ErrVaiTroKhongTonTaiDeGhi},
		{"over-long id", strings.Repeat("x", 300), RoleInput{Name: "Bí thư"}, idstore.ErrVaiTroKhongTonTaiDeGhi},
		{"blank name", "vt-kt", RoleInput{Name: ""}, domain.ErrRoleNameInvalid},
	}
	for _, c := range cases {
		b := newRoleWriteBench(t)
		if _, err := b.uc.Edit(ctxXa(xaThu), c.id, c.in, nguoiThucHienGia()); !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		assertRoleNothingWritten(t, b)
	}
}

// --- deletion -------------------------------------------------------------------------------------

func TestRoleRemoveSoftDeletesWithReasonAndStaffCode(t *testing.T) {
	b := newRoleWriteBench(t)
	del, err := b.uc.Remove(ctxXa(xaThu), "vt-kt", "  Gộp vào Văn phòng  ", nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if del.Role.Ma != "ke-toan" || del.DeletedBy != maCanBo || del.DeletedAt.IsZero() {
		t.Fatalf("kết quả = %+v", del)
	}
	if b.ghi.tim("GHI-GIA soft-delete") == nil {
		t.Fatal("không xoá mềm")
	}
	// deleted_by is the STAFF CODE (rule 6, invariant 8), never the internal id.
	if !hasCall(b.repo.calls, "soft-delete:vt-kt:"+maCanBo) {
		t.Errorf("xoá mềm phải ghi mã cán bộ: %v", b.repo.calls)
	}
	all := oneEntryInTheWriteTx(t, b, ActionRemoveRole)
	if !strings.Contains(all, `"ly_do":"Gộp vào Văn phòng"`) || !strings.Contains(all, `"da_xoa":true`) {
		t.Errorf("vết thiếu lý do hoặc trạng thái sau: %s", all)
	}
}

func TestRoleRemoveRefusals(t *testing.T) {
	cases := []struct {
		name, id, reason string
		want             error
	}{
		{"administrator role", "vt-admin", "x", ErrSystemRole},
		{"still held", "vt-ct", "x", &RoleInUseError{}},
		{"blank reason", "vt-kt", "  ", domain.ErrRoleDeleteReasonInvalid},
		{"reason too long", "vt-kt", strings.Repeat("ạ", domain.MaxRoleDeleteReason+1), domain.ErrRoleDeleteReasonInvalid},
		{"already deleted", "vt-bt-cu", "x", idstore.ErrVaiTroKhongTonTaiDeGhi},
		{"unknown id", "vt-khong-co", "x", idstore.ErrVaiTroKhongTonTaiDeGhi},
	}
	for _, c := range cases {
		b := newRoleWriteBench(t)
		_, err := b.uc.Remove(ctxXa(xaThu), c.id, c.reason, nguoiThucHienGia())
		var inUse *RoleInUseError
		switch want := c.want.(type) {
		case *RoleInUseError:
			if !errors.As(err, &inUse) || inUse.Holders != 1 {
				t.Errorf("%s: lỗi = %v, muốn RoleInUseError{1}", c.name, err)
			}
		default:
			if !errors.Is(err, want) {
				t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, want)
			}
		}
		assertRoleNothingWritten(t, b)
	}
}

// A half-built actor is refused before any transaction: an entry without a staff code is what rule 6
// invariant 8 forbids.
func TestRoleWritesRefuseAnActorWithoutStaffCode(t *testing.T) {
	b := newRoleWriteBench(t)
	actor := nguoiThucHienGia()
	actor.Vet.ID = ""
	if _, err := b.uc.Add(ctxXa(xaThu), RoleInput{Name: "Bí thư"}, actor); err == nil {
		t.Error("Add nhận người thực hiện không có mã")
	}
	if _, err := b.uc.Edit(ctxXa(xaThu), "vt-kt", RoleInput{Name: "X"}, actor); err == nil {
		t.Error("Edit nhận người thực hiện không có mã")
	}
	if _, err := b.uc.Remove(ctxXa(xaThu), "vt-kt", "x", actor); err == nil {
		t.Error("Remove nhận người thực hiện không có mã")
	}
	if b.ghi.soGiaoDich() != 0 {
		t.Error("đã mở giao dịch cho người thực hiện không hợp lệ")
	}
}

func hasCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}
