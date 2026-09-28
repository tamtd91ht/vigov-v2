package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: POST /api/v1/roles/defaults grants authority in a commune, and every one of
// its refusals FAILS SILENTLY when removed — the run succeeds, the matrix fills, and the trail
// records an escalation, or a re-grant a commune had withdrawn, as an ordinary seed.
//
// It runs on the fake database/sql driver of dang_nhap_giao_dich_test.go, and the fake repo's two
// writes run a REAL statement through the transaction they were handed, so "every role, every grant
// and every audit entry share one transaction" is an assertion about what actually ran.
//
// WHAT IT CANNOT PROVE: `UNIQUE (tenant_id, ma)` refusing a concurrent run, the partition constraint
// name store.translateRoleInsertError matches on, or the lock order actually serialising against
// PUT /api/v1/roles/{id}/permissions. Those need a PostgreSQL; VIGOV_TEST_DSN is unset here.

type roleTemplateRepoFake struct {
	calls []string

	held   []string
	states map[string]bool // code -> deleted; absent = no row

	inserted []domain.RoleTemplate
	granted  map[string][]string // role id -> keys
	grantBy  []string
}

func (r *roleTemplateRepoFake) LockGrantorSets(context.Context, *store.ScopedTx) error {
	r.calls = append(r.calls, "lock")
	return nil
}

func (r *roleTemplateRepoFake) HeldPermissions(_ context.Context, _ *store.ScopedTx, staffID string) ([]string, error) {
	r.calls = append(r.calls, "held:"+staffID)
	return r.held, nil
}

func (r *roleTemplateRepoFake) RoleCodeStates(context.Context, *store.ScopedTx, []string) (map[string]bool, error) {
	r.calls = append(r.calls, "states")
	out := map[string]bool{}
	for k, v := range r.states {
		out[k] = v
	}
	return out, nil
}

func (r *roleTemplateRepoFake) InsertRole(ctx context.Context, tx *store.ScopedTx, id string, t domain.RoleTemplate) error {
	r.calls = append(r.calls, "insert:"+t.Code)
	r.inserted = append(r.inserted, t)
	_, err := tx.Exec(ctx, "GHI-GIA insert vai_tro", string(tx.TenantID()), id, t.Code)
	return err
}

func (r *roleTemplateRepoFake) GrantPermissions(ctx context.Context, tx *store.ScopedTx, roleID string, keys []string, grantedBy string) error {
	r.calls = append(r.calls, "grant:"+roleID)
	if r.granted == nil {
		r.granted = map[string][]string{}
	}
	r.granted[roleID] = append([]string{}, keys...)
	r.grantBy = append(r.grantBy, grantedBy)
	_, err := tx.Exec(ctx, "GHI-GIA grant vai_tro_quyen", string(tx.TenantID()), roleID, strings.Join(keys, ","), grantedBy)
	return err
}

type roleTemplateBench struct {
	uc   *RoleTemplateSeeder
	repo *roleTemplateRepoFake
	ghi  *ghiChep
}

// newRoleTemplateBench: an empty commune and an actor holding every template key.
func newRoleTemplateBench(t *testing.T) *roleTemplateBench {
	t.Helper()
	db, g := moDB(t)
	repo := &roleTemplateRepoFake{
		held:   append([]string{"feedback.unmask"}, domain.RoleTemplatePermissions()...),
		states: map[string]bool{},
	}
	uc := NewRoleTemplateSeeder(db, repo)
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("vt-mau-%02d", n), nil
	}
	return &roleTemplateBench{uc: uc, repo: repo, ghi: g}
}

func assertNothingWritten(t *testing.T, b *roleTemplateBench) {
	t.Helper()
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Errorf("có %d vết kiểm toán dù không được ghi gì", n)
	}
	if b.ghi.tim("GHI-GIA") != nil || len(b.repo.inserted) != 0 {
		t.Error("vẫn chèn vai trò hoặc cấp quyền dù không được ghi gì")
	}
}

// --- the first run --------------------------------------------------------------------------------

// All eight roles, all their grants and eight audit entries, in ONE committed transaction.
//
// MUTATION THAT MUST TURN THIS RED: move audit.Write out of the Tx closure, or drop it.
func TestSeedRoleTemplatesFirstRunCreatesEightInOneTransaction(t *testing.T) {
	b := newRoleTemplateBench(t)

	res, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if len(res.Created) != 8 || len(res.SkippedExisting) != 0 || len(res.SkippedDeleted) != 0 {
		t.Fatalf("kết quả = %d tạo / %d có sẵn / %d đã xoá, muốn 8/0/0",
			len(res.Created), len(res.SkippedExisting), len(res.SkippedDeleted))
	}

	vet := vetDaGhi(b.ghi)
	if len(vet) != 8 {
		t.Fatalf("có %d vết, muốn 8 — một vết cho mỗi vai trò được tạo", len(vet))
	}
	tx := vet[0].tx
	if tx == 0 {
		t.Fatal("vết ghi ngoài giao dịch")
	}
	b.ghi.mu.Lock()
	for _, l := range b.ghi.lenh {
		if (strings.Contains(l.sql, "GHI-GIA") || strings.Contains(l.sql, "audit_log")) && l.tx != tx {
			t.Errorf("câu %q nằm ở giao dịch %d, muốn %d — luật 6 bất biến 3", l.sql, l.tx, tx)
		}
	}
	b.ghi.mu.Unlock()
	if ket := b.ghi.ketThucCua(tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}

	// Each role got EXACTLY its template's keys — not the union, not the catalogue.
	for i, tpl := range domain.RoleTemplates() {
		id := fmt.Sprintf("vt-mau-%02d", i+1)
		if got := strings.Join(b.repo.granted[id], ","); got != strings.Join(tpl.Permissions, ",") {
			t.Errorf("%s được cấp %q, muốn %q", tpl.Code, got, strings.Join(tpl.Permissions, ","))
		}
	}
}

// Actor = STAFF CODE on every entry and every grant cell; subject = the new role id; tenant = context.
//
// MUTATION THAT MUST TURN THIS RED: write nguoi.ID into the actor or into cap_boi.
func TestSeedRoleTemplatesAuditCarriesStaffCodeAndRoleID(t *testing.T) {
	b := newRoleTemplateBench(t)
	if _, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	for i, v := range vetDaGhi(b.ghi) {
		if got := chuoiArg(t, v, 0); got != string(xaThu) {
			t.Errorf("vết %d mang xã %q, muốn %q", i, got, xaThu)
		}
		if got := chuoiArg(t, v, viTriActor); got != maCanBo {
			t.Errorf("vết %d: actor_id = %q, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", i, got, maCanBo)
		}
		if got := chuoiArg(t, v, viTriHanhVi); got != ActionSeedRoleTemplate {
			t.Errorf("vết %d: action = %q", i, got)
		}
		if got := chuoiArg(t, v, viTriChuThe); got != fmt.Sprintf("vt-mau-%02d", i+1) {
			t.Errorf("vết %d: subject = %q, muốn id vai trò mới", i, got)
		}
		d := deltaCua(t, v)
		if d["ma"] != domain.RoleTemplates()[i].Code {
			t.Errorf("vết %d: delta.ma = %v", i, d["ma"])
		}
	}
	for _, by := range b.repo.grantBy {
		if by != maCanBo {
			t.Errorf("cap_boi = %q, muốn mã cán bộ %q", by, maCanBo)
		}
	}
}

// The locks are taken before the caller's keys are read, and both before any write.
func TestSeedRoleTemplatesLocksBeforeReadingHeldKeys(t *testing.T) {
	b := newRoleTemplateBench(t)
	if _, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	want := []string{"lock", "held:" + idNoiBo, "states"}
	for i, w := range want {
		if i >= len(b.repo.calls) || b.repo.calls[i] != w {
			t.Fatalf("thứ tự gọi = %v, muốn bắt đầu bằng %v", b.repo.calls, want)
		}
	}
}

// --- idempotency and existing roles ----------------------------------------------------------------

// A SECOND RUN CREATES NOTHING, WRITES NOTHING AND AUDITS NOTHING.
//
// MUTATION THAT MUST TURN THIS RED: drop the `if ... exists { continue }` skip.
func TestSeedRoleTemplatesSecondRunCreatesNothing(t *testing.T) {
	b := newRoleTemplateBench(t)
	for _, tpl := range domain.RoleTemplates() {
		b.repo.states[tpl.Code] = false
	}
	res, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if len(res.Created) != 0 || len(res.SkippedExisting) != 8 {
		t.Fatalf("lần hai: %d tạo / %d có sẵn, muốn 0/8", len(res.Created), len(res.SkippedExisting))
	}
	assertNothingWritten(t, b)
}

// AN EXISTING ROLE IS UNTOUCHED, GRANTS INCLUDED — no top-up of missing keys.
//
// MUTATION THAT MUST TURN THIS RED: call GrantPermissions for a role found present.
func TestSeedRoleTemplatesLeavesExistingRoleAndItsGrantsAlone(t *testing.T) {
	b := newRoleTemplateBench(t)
	b.repo.states["ke-toan"] = false

	res, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if len(res.Created) != 7 {
		t.Errorf("tạo %d, muốn 7", len(res.Created))
	}
	if len(res.SkippedExisting) != 1 || res.SkippedExisting[0] != (RoleTemplateRef{Code: "ke-toan", Name: "Kế toán"}) {
		t.Errorf("có sẵn = %+v, muốn [ke-toan Kế toán]", res.SkippedExisting)
	}
	for _, c := range b.repo.calls {
		if c == "insert:ke-toan" {
			t.Error("vai trò ke-toan đã có mà vẫn bị chèn lại")
		}
	}
	if len(b.repo.granted) != 7 {
		t.Errorf("cấp quyền cho %d vai trò, muốn 7 — vai trò có sẵn KHÔNG được bù quyền", len(b.repo.granted))
	}
	if n := len(vetDaGhi(b.ghi)); n != 7 {
		t.Errorf("có %d vết, muốn 7", n)
	}
}

// A SOFT-DELETED ROLE WITH A TEMPLATE'S CODE IS SKIPPED AND REPORTED, never revived or recreated.
func TestSeedRoleTemplatesSkipsAndReportsDeletedRole(t *testing.T) {
	b := newRoleTemplateBench(t)
	b.repo.states["truong-thon"] = true

	res, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	if len(res.SkippedDeleted) != 1 || res.SkippedDeleted[0].Code != "truong-thon" {
		t.Errorf("đã xoá = %+v, muốn [truong-thon]", res.SkippedDeleted)
	}
	if len(res.SkippedExisting) != 0 || len(res.Created) != 7 {
		t.Errorf("%d tạo / %d có sẵn, muốn 7/0", len(res.Created), len(res.SkippedExisting))
	}
	for _, c := range b.repo.calls {
		if c == "insert:truong-thon" {
			t.Error("vai trò đã xoá mềm bị chèn lại")
		}
	}
}

// --- refusals -------------------------------------------------------------------------------------

// A CALLER MISSING ONE KEY IS REFUSED WHOLE: no role, no grant, no entry, and the key is named.
//
// MUTATION THAT MUST TURN THIS RED: check only the templates that will be created, or drop the check.
func TestSeedRoleTemplatesRefusedWhenCallerLacksAKey(t *testing.T) {
	b := newRoleTemplateBench(t)
	var held []string
	for _, k := range domain.RoleTemplatePermissions() {
		if k != "budget.confirm" {
			held = append(held, k)
		}
	}
	b.repo.held = held

	_, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia())
	var missing *LoiTraoQuyenKhongCam
	if !errors.As(err, &missing) || strings.Join(missing.Thieu, ",") != "budget.confirm" {
		t.Fatalf("lỗi = %v, muốn LoiTraoQuyenKhongCam{budget.confirm}", err)
	}
	if !errors.Is(err, ErrTraoQuyenKhongCam) {
		t.Error("lỗi không nhận là ErrTraoQuyenKhongCam — tuyến sẽ không trả 403")
	}
	assertNothingWritten(t, b)
}

// The check covers templates that already exist too: a second run does not become a way round it.
func TestSeedRoleTemplatesRefusedEvenWhenEverythingExists(t *testing.T) {
	b := newRoleTemplateBench(t)
	for _, tpl := range domain.RoleTemplates() {
		b.repo.states[tpl.Code] = false
	}
	b.repo.held = []string{"admin.role"}
	if _, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia()); !errors.Is(err, ErrTraoQuyenKhongCam) {
		t.Fatalf("lỗi = %v, muốn ErrTraoQuyenKhongCam", err)
	}
}

// AN AUDIT FAILURE TAKES EVERY ROLE AND GRANT DOWN WITH IT (rule 6, forbidden #2).
func TestSeedRoleTemplatesAuditFailureRollsBack(t *testing.T) {
	b := newRoleTemplateBench(t)
	b.ghi.loiTheo["audit_log"] = errors.New("driver giả: vết hỏng")

	if _, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia()); err == nil {
		t.Fatal("vết hỏng mà vẫn báo thành công")
	}
	ins := b.ghi.tim("GHI-GIA insert")
	if ins == nil {
		t.Fatal("không có câu chèn nào chạy trước vết — ca này không chứng minh gì")
	}
	if ket := b.ghi.ketThucCua(ins.tx); ket != "rollback" {
		t.Errorf("giao dịch kết thúc bằng %q, muốn rollback", ket)
	}
}

// NO STAFF CODE, NO WRITE — and never a fallback to the internal id.
func TestSeedRoleTemplatesRefusesActorWithoutStaffCode(t *testing.T) {
	b := newRoleTemplateBench(t)
	actor := nguoiThucHienGia()
	actor.Vet.ID = ""
	if _, err := b.uc.SeedDefaults(ctxXa(xaThu), actor); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn gieo")
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch dù người thực hiện không hợp lệ", n)
	}
}

// A lost race on the unique key surfaces as the store's sentinel, with nothing committed.
func TestSeedRoleTemplatesUniqueCollisionRollsBack(t *testing.T) {
	b := newRoleTemplateBench(t)
	b.ghi.loiTheo["GHI-GIA insert"] = idstore.ErrRoleCodeTaken

	_, err := b.uc.SeedDefaults(ctxXa(xaThu), nguoiThucHienGia())
	if !errors.Is(err, idstore.ErrRoleCodeTaken) {
		t.Fatalf("lỗi = %v, muốn ErrRoleCodeTaken", err)
	}
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Errorf("có %d vết dù lượt gieo thua cuộc đua", n)
	}
}
