package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The staff import over a REAL transaction boundary (moDB). The row rules are the planner's
// (domain/staff_import_test.go); what is proved here is what the use case owns: the `admin.role` gate,
// #14's second constraint, ONE write transaction for the whole file with every entry inside it, the
// passwords minted outside it and absent from every statement, and nothing committed on a refusal.

const plainMarker = "TAM-KHONG-PHAI-THAT"

type staffRepoFake struct {
	snap      domain.StaffImportSnapshot
	held      []string
	roleKeys  map[string][]string
	chenFails int // the first N inserts answer ErrMaCanBoDaDung

	locks    int
	inserted []domain.CanBoTomTat
	roles    map[string]string
	accounts map[string]string // id -> hash
	txs      map[*store.ScopedTx]bool
}

func newStaffRepo() *staffRepoFake {
	return &staffRepoFake{
		snap: domain.StaffImportSnapshot{
			TakenEmails: []string{"da.co@xa.gov.vn"},
			OrgUnits:    []domain.StaffImportOrgUnitChoice{{ID: "bp-1", Code: "van-phong", Name: "VĂN PHÒNG"}},
			Roles: []domain.StaffImportRoleChoice{
				{ID: "vt-cv", Code: "chuyen-vien", Name: "Chuyên viên"},
				{ID: "vt-ct", Code: "chu-tich", Name: "Chủ tịch"},
			},
		},
		held:     []string{"admin.user", "admin.role", "task.view"},
		roleKeys: map[string][]string{"vt-cv": {"task.view"}, "vt-ct": {"task.view", "budget.confirm"}},
		roles:    map[string]string{}, accounts: map[string]string{}, txs: map[*store.ScopedTx]bool{},
	}
}

func (f *staffRepoFake) LockStaffImport(_ context.Context, tx *store.ScopedTx) error {
	f.locks++
	return nil
}

func (f *staffRepoFake) StaffImportSnapshot(context.Context, *store.ScopedTx) (domain.StaffImportSnapshot, error) {
	s := f.snap
	s.Roles = append([]domain.StaffImportRoleChoice(nil), f.snap.Roles...)
	return s, nil
}

func (f *staffRepoFake) QuyenCuaVaiTro(_ context.Context, _ *store.ScopedTx, id string) ([]string, bool, error) { // vi-name-ok: implements the existing store method
	k, ok := f.roleKeys[id]
	return k, ok, nil
}

func (f *staffRepoFake) QuyenDangGiu(context.Context, *store.ScopedTx, string) ([]string, error) { // vi-name-ok: implements the existing store method
	return f.held, nil
}

func (f *staffRepoFake) Chen(_ context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error {
	if f.chenFails > 0 {
		f.chenFails--
		return idstore.ErrMaCanBoDaDung
	}
	f.txs[tx] = true
	f.inserted = append(f.inserted, cb)
	return nil
}

func (f *staffRepoFake) DatVaiTro(_ context.Context, tx *store.ScopedTx, id, role string) error { // vi-name-ok: implements the existing store method
	f.txs[tx] = true
	f.roles[id] = role
	return nil
}

func (f *staffRepoFake) CapTaiKhoan(_ context.Context, tx *store.ScopedTx, id, h string) error { // vi-name-ok: implements the existing store method
	f.txs[tx] = true
	f.accounts[id] = h
	return nil
}

func (f *staffRepoFake) StaffTemplateChoices(context.Context) ([]domain.StaffImportOrgUnitChoice, []domain.StaffImportRoleChoice, error) {
	return f.snap.OrgUnits, f.snap.Roles, nil
}

func staffImporterForTest(t *testing.T, repo *staffRepoFake) (*StaffImporter, *ghiChep, context.Context) {
	t.Helper()
	db, g := moDB(t)
	uc := NewStaffImporter(db, repo)
	n, c, p := 0, 0, 0
	uc.newID = func() (string, error) { n++; return fmt.Sprintf("01JSTAFFIMPORT%012d", n), nil }
	uc.newCode = func(time.Time) (string, error) { c++; return fmt.Sprintf("CB-2026-%06d", c), nil }
	uc.newPassword = func() (string, error) { p++; return fmt.Sprintf("%s-%d", plainMarker, p), nil }
	uc.hash = func(s string) (string, error) { return "$argon2id$gia$" + fmt.Sprint(len(s)), nil }
	return uc, g, tenant.Into(context.Background(), xaThu)
}

func staffRows() []domain.StaffImportRow {
	return []domain.StaffImportRow{
		{Row: 2, FullName: "Nguyễn Văn An", Email: "an@xa.gov.vn", OrgUnit: "van-phong · VĂN PHÒNG", Role: "chuyen-vien · Chuyên viên", Mobile: dienThoaiGia},
		{Row: 3, FullName: "Nguyễn Văn An"},                          // same name, no address: no account
		{Row: 4, FullName: "Trần Thị Bình", Email: "binh@xa.gov.vn"}, // account, no role
	}
}

func auditEntries(g *ghiChep) []lenhGhi {
	var out []lenhGhi
	for _, l := range g.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") {
			out = append(out, l)
		}
	}
	return out
}

func TestStaffImport_WritesAllInOneTransactionWithEveryEntry(t *testing.T) {
	repo := newStaffRepo()
	uc, g, ctx := staffImporterForTest(t, repo)
	res, err := uc.Import(ctx, staffRows(), catalogueActor())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// Two transactions: the unlocked plan (rolled back) and the write (committed).
	if g.soGiaoDich() != 2 || g.ketThucCua(1) != "rollback" || g.ketThucCua(2) != "commit" {
		t.Fatalf("giao dịch: %d, %q, %q", g.soGiaoDich(), g.ketThucCua(1), g.ketThucCua(2))
	}
	if repo.locks != 1 || len(repo.txs) != 1 {
		t.Errorf("mọi ghi phải trong MỘT giao dịch có khoá: khoá %d, giao dịch ghi %d", repo.locks, len(repo.txs))
	}
	if len(repo.inserted) != 3 || len(repo.roles) != 1 || len(repo.accounts) != 2 {
		t.Fatalf("chèn %d, vai trò %d, tài khoản %d", len(repo.inserted), len(repo.roles), len(repo.accounts))
	}
	if repo.inserted[1].Email != "" {
		t.Error("dòng không thư điện tử phải lưu rỗng (store ghi NULL)")
	}
	if len(res.People) != 3 || res.Batch == "" {
		t.Fatalf("kết quả: %+v", res)
	}
	a, b, c := res.People[0], res.People[1], res.People[2]
	if !a.AccountIssued || !strings.HasPrefix(a.TemporaryPassword, plainMarker) || a.Staff.Ma == "" || a.RoleID != "vt-cv" {
		t.Errorf("dòng 2: %+v", a)
	}
	if b.AccountIssued || b.TemporaryPassword != "" || b.Staff.Ma == "" {
		t.Errorf("dòng 3 không có tài khoản: %+v", b)
	}
	if !c.AccountIssued || c.TemporaryPassword == a.TemporaryPassword {
		t.Errorf("dòng 4: mỗi người một mật khẩu: %+v", c)
	}
	if repo.accounts[a.Staff.ID] == a.TemporaryPassword || !strings.HasPrefix(repo.accounts[a.Staff.ID], "$argon2id$") {
		t.Error("kho chỉ nhận chuỗi băm, không bao giờ bản trần")
	}

	// 3 them_can_bo + 1 doi_vai_tro_can_bo + 2 cap_tai_khoan_can_bo, all in the write transaction.
	count := map[any]int{}
	for _, e := range auditEntries(g) {
		count[e.args[4]]++
		if e.tx != 2 || e.args[1] != maCanBo {
			t.Errorf("vết %v: tx=%d actor=%v", e.args[4], e.tx, e.args[1])
		}
		raw, _ := e.args[7].([]byte)
		var d map[string]any
		if err := json.Unmarshal(raw, &d); err != nil || d["lo_nhap"] != res.Batch || d["nguon"] != staffImportSource {
			t.Errorf("delta %s (%v)", raw, err)
		}
		if strings.Contains(string(raw), dienThoaiGia) || strings.Contains(string(raw), "Nguyễn Văn An") {
			t.Errorf("delta mang dữ liệu cá nhân chưa che: %s", raw)
		}
	}
	if count[HanhViThemCanBo] != 3 || count[HanhViDoiVaiTro] != 1 || count[HanhViCapTaiKhoan] != 2 {
		t.Errorf("số vết theo hành vi: %v", count)
	}

	// THE PLAINTEXT NEVER REACHED A STATEMENT (rule 3, rule 8).
	for _, s := range g.moiThamSo() {
		if strings.Contains(s, plainMarker) {
			t.Fatalf("mật khẩu tạm lọt vào câu lệnh: %q", s)
		}
	}
}

func TestStaffImport_RoleColumnWithoutAdminRoleIs403AndWritesNothing(t *testing.T) {
	repo := newStaffRepo()
	repo.held = []string{"admin.user"}
	uc, g, ctx := staffImporterForTest(t, repo)
	minted := 0
	uc.newPassword = func() (string, error) { minted++; return plainMarker, nil }

	for name, run := range map[string]func() error{
		"nhập":      func() error { _, err := uc.Import(ctx, staffRows(), catalogueActor()); return err },
		"xem trước": func() error { _, err := uc.Preview(ctx, staffRows(), catalogueActor()); return err },
	} {
		if err := run(); !errors.Is(err, ErrRoleAssignmentNotPermitted) {
			t.Errorf("%s: lỗi = %v, muốn ErrRoleAssignmentNotPermitted", name, err)
		}
	}
	if len(repo.inserted) != 0 || repo.locks != 0 || minted != 0 || len(auditEntries(g)) != 0 {
		t.Errorf("không có admin.role mà vẫn chạy: chèn %d, khoá %d, sinh %d", len(repo.inserted), repo.locks, minted)
	}
	// Without a role cell the same actor imports.
	rows := staffRows()
	rows[0].Role = ""
	if _, err := uc.Import(ctx, rows, catalogueActor()); err != nil {
		t.Errorf("không có cột vai trò thì chỉ cần admin.user: %v", err)
	}
}

func TestStaffImport_RoleCarryingAKeyTheActorLacksIsARowError(t *testing.T) {
	repo := newStaffRepo()
	uc, g, ctx := staffImporterForTest(t, repo)
	rows := staffRows()
	rows[0].Role = "chu-tich · Chủ tịch"
	_, err := uc.Import(ctx, rows, catalogueActor())
	var rej *StaffImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 2 || !strings.Contains(rej.Errors[0].Message, "budget.confirm") {
		t.Fatalf("lỗi = %v, muốn lỗi dòng 2 nêu budget.confirm", err)
	}
	if len(repo.inserted) != 0 || g.soGiaoDich() != 1 || g.ketThucCua(1) != "rollback" {
		t.Errorf("tệp bị từ chối phải không ghi gì và không mở giao dịch ghi: %d %d", len(repo.inserted), g.soGiaoDich())
	}
}

func TestStaffImport_OneBadRowWritesNothingAndMintsNothing(t *testing.T) {
	repo := newStaffRepo()
	uc, _, ctx := staffImporterForTest(t, repo)
	minted := 0
	uc.newPassword = func() (string, error) { minted++; return plainMarker, nil }
	rows := append(staffRows(), domain.StaffImportRow{Row: 5, FullName: "X", Email: "DA.CO@xa.gov.vn"})
	_, err := uc.Import(ctx, rows, catalogueActor())
	var rej *StaffImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 5 {
		t.Fatalf("lỗi = %v", err)
	}
	if len(repo.inserted) != 0 || minted != 0 {
		t.Errorf("một dòng lỗi mà đã chèn %d / sinh %d mật khẩu", len(repo.inserted), minted)
	}
}

func TestStaffImport_LateFailureRollsBackEverything(t *testing.T) {
	repo := newStaffRepo()
	uc, g, ctx := staffImporterForTest(t, repo)
	g.loiTheo["INSERT INTO audit_log"] = errors.New("ổ đĩa đầy")
	if _, err := uc.Import(ctx, staffRows(), catalogueActor()); err == nil {
		t.Fatal("vết lỗi mà nhập vẫn thành công")
	}
	if g.ketThucCua(2) != "rollback" {
		t.Errorf("giao dịch ghi phải rollback: %q", g.ketThucCua(2))
	}
}

func TestStaffImport_CodeCollisionRetriesTheWholeTransaction(t *testing.T) {
	repo := newStaffRepo()
	repo.chenFails = 1
	uc, g, ctx := staffImporterForTest(t, repo)
	res, err := uc.Import(ctx, staffRows(), catalogueActor())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// plan (rollback) + attempt 1 (rollback) + attempt 2 (commit)
	if g.soGiaoDich() != 3 || g.ketThucCua(2) != "rollback" || g.ketThucCua(3) != "commit" {
		t.Errorf("giao dịch: %d %q %q", g.soGiaoDich(), g.ketThucCua(2), g.ketThucCua(3))
	}
	if len(res.People) != 3 || len(repo.inserted) != 3 {
		t.Errorf("sau khi thử lại: %d người, %d dòng chèn", len(res.People), len(repo.inserted))
	}
	seen := map[string]bool{}
	for _, p := range res.People {
		if seen[p.Staff.Ma] {
			t.Errorf("mã trùng trong lô: %s", p.Staff.Ma)
		}
		seen[p.Staff.Ma] = true
	}
}

func TestStaffImport_PreviewWritesNothingMintsNothing(t *testing.T) {
	repo := newStaffRepo()
	uc, g, ctx := staffImporterForTest(t, repo)
	minted := 0
	uc.newPassword = func() (string, error) { minted++; return plainMarker, nil }
	res, err := uc.Preview(ctx, staffRows(), catalogueActor())
	if err != nil || len(res.People) != 3 || len(res.Errors) != 0 {
		t.Fatalf("xem trước: %+v %v", res, err)
	}
	for _, p := range res.People {
		if p.TemporaryPassword != "" || p.Staff.Ma != "" {
			t.Errorf("xem trước không sinh mã, không sinh mật khẩu: %+v", p)
		}
	}
	if len(repo.inserted) != 0 || repo.locks != 0 || minted != 0 || g.ketThucCua(1) != "rollback" {
		t.Errorf("xem trước phải không ghi gì")
	}
}

func TestStaffImport_RefusesActorWithoutStaffCode(t *testing.T) {
	uc, g, ctx := staffImporterForTest(t, newStaffRepo())
	if _, err := uc.Import(ctx, staffRows(), NguoiThucHien{ID: idNoiBo}); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn nhập")
	}
	if g.soGiaoDich() != 0 {
		t.Error("phải từ chối trước khi mở giao dịch")
	}
}

// OVER THE REAL STORE: the writes are the single-create statements, and every one binds the commune at
// $1. An address-less row reaches the INSERT as ” — which nullif turns into NULL.
func TestStaffImport_RealStoreStatements(t *testing.T) {
	db, g := moDB(t)
	g.khongCoNguoiDung = true // an empty directory: the snapshot reads find nothing
	uc := NewStaffImporter(db, idstore.NewCanBoStore(db))
	uc.hash = func(string) (string, error) { return "$argon2id$gia", nil }
	rows := []domain.StaffImportRow{{Row: 2, FullName: "Không Thư", Mobile: dienThoaiGia}}
	if _, err := uc.Import(tenant.Into(context.Background(), xaThu), rows, catalogueActor()); err != nil {
		t.Fatalf("Import: %v", err)
	}
	ins := g.tim("INSERT INTO nguoi_dung")
	if ins == nil || !strings.Contains(ins.sql, "nullif($5,'')") {
		t.Fatalf("câu chèn phải là của form và ghi NULL khi không có thư điện tử: %+v", ins)
	}
	if g.tim("pg_advisory_xact_lock") == nil || g.tim("mat_khau_hash = $3") != nil {
		t.Error("phải khoá lượt nhập; dòng không thư điện tử không được cấp tài khoản")
	}
	for _, l := range g.lenh {
		if len(l.args) == 0 || l.args[0] != string(xaThu) {
			t.Errorf("câu lệnh không mang xã của ngữ cảnh ở $1: %q", l.sql)
		}
	}
}
