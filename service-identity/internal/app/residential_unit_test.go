package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The residential-unit write surface and import over the REAL store and a REAL transaction boundary —
// the argument driver_gia_bo_phan_test.go makes. The fake answers from the statement it is given, and
// only for the commune bound at $1, so a statement that dropped the commune reads nothing of it.

type ruUnitRow struct {
	xa                    tenant.ID
	id, ma, ten, loai     string
	head                  string
	active, deleted       bool
	households, populaton any
	order                 int64
}

type ruTypeRow struct {
	xa        tenant.ID
	ma, label string
	usable    bool
}

type ruStaffRow struct {
	xa             tenant.ID
	id, code, name string
	eligible       bool
}

type ruFakeDB struct {
	mu   sync.Mutex
	stmt []lenhGhi

	begun, committed, rolledBack int

	units []ruUnitRow
	types []ruTypeRow
	staff []ruStaffRow

	failOn string // fail the FIRST statement containing this substring
	failed bool
}

func (k *ruFakeDB) record(q string, args []driver.NamedValue) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stmt = append(k.stmt, lenhGhi{sql: q, args: giaTri(args)})
	if k.failOn != "" && !k.failed && strings.Contains(q, k.failOn) {
		k.failed = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *ruFakeDB) find(sub string) []lenhGhi {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []lenhGhi
	for _, l := range k.stmt {
		if strings.Contains(l.sql, sub) {
			out = append(out, l)
		}
	}
	return out
}

func (k *ruFakeDB) Connect(context.Context) (driver.Conn, error) { return &ruConn{k: k}, nil }
func (k *ruFakeDB) Driver() driver.Driver                        { return trinhGia{} }

type ruConn struct{ k *ruFakeDB }

func (c *ruConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("không hỗ trợ Prepare")
}
func (c *ruConn) Close() error { return nil }
func (c *ruConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *ruConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	return &ruTx{k: c.k}, nil
}

func (c *ruConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *ruConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	c.k.mu.Lock()
	defer c.k.mu.Unlock()
	if !strings.Contains(q, "tenant_id = $1") {
		return nil, fmt.Errorf("driver giả: câu đọc không lọc theo xã: %q", q)
	}
	xa := tenant.ID(chuoiThu(args, 0))
	switch {
	case strings.Contains(q, "FOR UPDATE OF tt"):
		id := chuoiThu(args, 1)
		cot := []string{"id", "ma", "ten", "loai", "nhan", "so_ho", "nhan_khau", "dang_dung", "head", "head_ma", "head_ten", "sort_order"}
		for _, u := range c.k.units {
			if u.xa == xa && u.id == id && !u.deleted {
				label, headCode, headName := "", "", ""
				for _, t := range c.k.types {
					if t.xa == xa && t.ma == u.loai {
						label = t.label
					}
				}
				for _, s := range c.k.staff {
					if s.xa == xa && s.id == u.head {
						headCode, headName = s.code, s.name
					}
				}
				return &rowsGia{cot: cot, hang: [][]driver.Value{{u.id, u.ma, u.ten, u.loai, label,
					u.households, u.populaton, u.active, u.head, headCode, headName, u.order}}}, nil
			}
		}
		return &rowsGia{cot: cot}, nil

	case strings.Contains(q, "FROM thon_to_dan_pho"):
		var out [][]driver.Value
		for _, u := range c.k.units {
			if u.xa == xa {
				out = append(out, []driver.Value{u.id, u.ma, u.ten, u.deleted})
			}
		}
		return &rowsGia{cot: []string{"id", "ma", "ten", "deleted"}, hang: out}, nil

	case strings.Contains(q, "FROM loai_don_vi_dan_cu") && strings.Contains(q, "ma = $2"):
		for _, t := range c.k.types {
			if t.xa == xa && t.ma == chuoiThu(args, 1) && t.usable {
				return &rowsGia{cot: []string{"nhan"}, hang: [][]driver.Value{{t.label}}}, nil
			}
		}
		return &rowsGia{cot: []string{"nhan"}}, nil

	case strings.Contains(q, "FROM loai_don_vi_dan_cu"):
		var out [][]driver.Value
		for _, t := range c.k.types {
			if t.xa == xa && t.usable {
				out = append(out, []driver.Value{t.ma, t.label})
			}
		}
		return &rowsGia{cot: []string{"ma", "nhan"}, hang: out}, nil

	case strings.Contains(q, "FROM nguoi_dung") && strings.Contains(q, "ma = $2"):
		for _, s := range c.k.staff {
			if s.xa == xa && s.code == chuoiThu(args, 1) && s.eligible {
				return &rowsGia{cot: []string{"id", "ma", "ho_ten"}, hang: [][]driver.Value{{s.id, s.code, s.name}}}, nil
			}
		}
		return &rowsGia{cot: []string{"id", "ma", "ho_ten"}}, nil

	case strings.Contains(q, "FROM nguoi_dung"):
		var out [][]driver.Value
		for _, s := range c.k.staff {
			if s.xa == xa && s.eligible {
				out = append(out, []driver.Value{s.id, s.code, s.name})
			}
		}
		return &rowsGia{cot: []string{"id", "ma", "ho_ten"}, hang: out}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

type ruTx struct{ k *ruFakeDB }

func (t *ruTx) Commit() error {
	t.k.mu.Lock()
	t.k.committed++
	t.k.mu.Unlock()
	return nil
}

func (t *ruTx) Rollback() error {
	t.k.mu.Lock()
	t.k.rolledBack++
	t.k.mu.Unlock()
	return nil
}

// --- fixtures -------------------------------------------------------------------------------------

const (
	ruCommune      = tenant.ID("01JTHONXAA00000000000000AA")
	ruOtherCommune = tenant.ID("01JTHONXAB00000000000000BB")
)

// ruFixture: commune A has two units (one OUT OF USE), one soft-deleted row, two types (one retired),
// two staff (one locked); commune B has a unit, a type and a member of staff of its own.
func ruFixture() *ruFakeDB {
	return &ruFakeDB{
		units: []ruUnitRow{
			{xa: ruCommune, id: "tt-1", ma: "thon-binh-an", ten: "Thôn Bình An", loai: "thon", head: "nd-1", active: true, households: int64(284), order: 1},
			{xa: ruCommune, id: "tt-2", ma: "thon-cu", ten: "Thôn Cũ", active: false},
			{xa: ruCommune, id: "tt-x", ma: "da-xoa", ten: "Đã xoá", deleted: true},
			{xa: ruOtherCommune, id: "tt-b", ma: "thon-dong", ten: "Thôn Đông", active: true},
		},
		types: []ruTypeRow{
			{xa: ruCommune, ma: "thon", label: "Thôn", usable: true},
			{xa: ruCommune, ma: "cu", label: "Loại cũ", usable: false},
			{xa: ruOtherCommune, ma: "khu-pho", label: "Khu phố", usable: true},
		},
		staff: []ruStaffRow{
			{xa: ruCommune, id: "nd-1", code: "CB-2026-AAAAAA", name: "Cán Bộ Một", eligible: true},
			{xa: ruCommune, id: "nd-2", code: "CB-2026-LOCKED", name: "Cán Bộ Khoá", eligible: false},
			{xa: ruOtherCommune, id: "nd-b", code: "CB-2026-XAKHAC", name: "Cán Bộ Xã B", eligible: true},
		},
	}
}

func ruOpen(t *testing.T, k *ruFakeDB) (*store.DB, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return store.New(db), tenant.Into(context.Background(), ruCommune)
}

func ruUseCase(t *testing.T, k *ruFakeDB) (*ResidentialUnits, context.Context) {
	t.Helper()
	db, ctx := ruOpen(t, k)
	uc := NewResidentialUnits(db, idstore.NewThonToDanPhoStore(db))
	uc.newID = func() (string, error) { return "01JTHONMOI0000000000000000", nil }
	return uc, ctx
}

func ruDelta(t *testing.T, l lenhGhi) map[string]any {
	t.Helper()
	b, _ := l.args[7].([]byte)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("delta: %s (%v)", b, err)
	}
	return m
}

func ptr[T any](v T) *T { return &v }

// --- Create -----------------------------------------------------------------------------------------

func TestCreateResidentialUnit_InsertAndAuditInOneTransaction(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	u, err := uc.Create(ctx, CreateResidentialUnit{
		Name: "Thôn Hoà Bình", TypeCode: "thon", HeadCode: "CB-2026-AAAAAA", Households: ptr(0), Order: 3,
	}, nguoiBoPhan())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.Ma != "thon-hoa-binh" || u.LoaiNhan != "Thôn" || u.HeadStaffCode != "CB-2026-AAAAAA" || !u.DangDung {
		t.Errorf("đơn vị trả về: %+v", u)
	}
	if u.SoHo == nil || *u.SoHo != 0 || u.NhanKhau != nil {
		t.Errorf("0 phải là 0, trống phải là nil: so_ho=%v nhan_khau=%v", u.SoHo, u.NhanKhau)
	}
	if k.begun != 1 || k.committed != 1 {
		t.Fatalf("giao dịch: mở %d, commit %d", k.begun, k.committed)
	}
	ins := k.find("INSERT INTO thon_to_dan_pho")
	if len(ins) != 1 || ins[0].args[8] != "nd-1" || ins[0].args[5] != int64(0) || ins[0].args[6] != nil {
		t.Fatalf("INSERT: %+v — head phải là ID nội bộ, số hộ 0 là 0, nhân khẩu trống là NULL", ins)
	}
	vet := k.find("INSERT INTO audit_log")
	if len(vet) != 1 || vet[0].args[1] != maCanBoBoPhan || vet[0].args[4] != ActionCreateResidentialUnit || vet[0].args[5] != "thon-hoa-binh" {
		t.Fatalf("vết: %+v", vet)
	}
	if d := ruDelta(t, vet[0]); d["sau"].(map[string]any)["truong_thon"] != "CB-2026-AAAAAA" || strings.Contains(fmt.Sprint(d), "Cán Bộ Một") {
		t.Errorf("vết phải ghi MÃ trưởng thôn, không ghi tên: %v", d)
	}
	if len(k.find("pg_advisory_xact_lock")) != 1 {
		t.Error("thiếu khoá sổ theo xã")
	}
	for _, l := range k.stmt {
		if len(l.args) == 0 || l.args[0] != string(ruCommune) {
			t.Errorf("câu lệnh không mang xã của ngữ cảnh ở $1: %q %v", l.sql, l.args)
		}
	}
}

// THE CODE IS NEVER REISSUED, whether the unit holding it is out of use or soft-deleted.
func TestCreateResidentialUnit_CodeNeverReissued(t *testing.T) {
	for _, c := range []struct {
		name string
		req  CreateResidentialUnit
		want error
		code string
	}{
		{"mã gõ tay của đơn vị ĐÃ NGƯNG DÙNG", CreateResidentialUnit{Name: "Thôn Mới", Code: "thon-cu"}, idstore.ErrResidentialUnitCodeTaken, ""},
		{"mã gõ tay của dòng đã xoá mềm", CreateResidentialUnit{Name: "Thôn Mới", Code: "da-xoa"}, idstore.ErrResidentialUnitCodeTaken, ""},
		// "Thôn - Cũ" folds to another name but derives the retired unit's slug: it gets a suffix.
		{"mã tự sinh trùng mã đã ngưng dùng", CreateResidentialUnit{Name: "Thôn - Cũ"}, nil, "thon-cu-2"},
		// Another commune's code is not this commune's: it is free here.
		{"mã của xã khác", CreateResidentialUnit{Name: "Thôn Đông"}, nil, "thon-dong"},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := ruFixture()
			uc, ctx := ruUseCase(t, k)
			u, err := uc.Create(ctx, c.req, nguoiBoPhan())
			if !errors.Is(err, c.want) || (c.want == nil && u.Ma != c.code) {
				t.Fatalf("lỗi = %v, mã = %q; muốn lỗi %v, mã %q", err, u.Ma, c.want, c.code)
			}
			if c.want != nil && (len(k.find("INSERT")) != 0 || k.committed != 0) {
				t.Error("bị từ chối mà vẫn ghi")
			}
		})
	}
}

func TestCreateResidentialUnit_Refusals(t *testing.T) {
	for _, c := range []struct {
		name string
		req  CreateResidentialUnit
		want error
	}{
		{"trùng tên đơn vị đang dùng (khác hoa thường)", CreateResidentialUnit{Name: "THÔN BÌNH AN"}, idstore.ErrResidentialUnitNameTaken},
		{"trùng tên đơn vị ĐÃ NGƯNG DÙNG", CreateResidentialUnit{Name: "Thôn Cũ", Code: "thon-cu-moi"}, idstore.ErrResidentialUnitNameTaken},
		{"loại đã ngưng dùng", CreateResidentialUnit{Name: "Thôn Mới", TypeCode: "cu"}, idstore.ErrResidentialUnitTypeNotFound},
		{"loại của xã khác", CreateResidentialUnit{Name: "Thôn Mới", TypeCode: "khu-pho"}, idstore.ErrResidentialUnitTypeNotFound},
		{"trưởng thôn bị khoá", CreateResidentialUnit{Name: "Thôn Mới", HeadCode: "CB-2026-LOCKED"}, idstore.ErrHeadStaffNotFound},
		{"trưởng thôn của xã khác", CreateResidentialUnit{Name: "Thôn Mới", HeadCode: "CB-2026-XAKHAC"}, idstore.ErrHeadStaffNotFound},
		{"số hộ âm", CreateResidentialUnit{Name: "Thôn Mới", Households: ptr(-1)}, domain.ErrResidentialUnitCountRange},
		{"thứ tự quá lớn", CreateResidentialUnit{Name: "Thôn Mới", Order: 10000}, domain.ErrResidentialUnitOrderRange},
		{"tên trống", CreateResidentialUnit{Name: "  "}, domain.ErrResidentialUnitNameMissing},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := ruFixture()
			uc, ctx := ruUseCase(t, k)
			if _, err := uc.Create(ctx, c.req, nguoiBoPhan()); !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if len(k.find("INSERT")) != 0 || k.committed != 0 {
				t.Error("bị từ chối mà vẫn ghi")
			}
		})
	}
}

func TestCreateResidentialUnit_AuditFailureRollsBack(t *testing.T) {
	k := ruFixture()
	k.failOn = "INSERT INTO audit_log"
	uc, ctx := ruUseCase(t, k)
	if _, err := uc.Create(ctx, CreateResidentialUnit{Name: "Thôn Mới"}, nguoiBoPhan()); err == nil {
		t.Fatal("vết hỏng mà vẫn tạo")
	}
	if k.committed != 0 || k.rolledBack != 1 {
		t.Fatalf("commit %d rollback %d — vết hỏng phải huỷ cả lần ghi (luật 6 bất biến 3)", k.committed, k.rolledBack)
	}
}

func TestCreateResidentialUnit_RefusesActorWithoutStaffCode(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	n := nguoiBoPhan()
	n.Vet.ID = ""
	if _, err := uc.Create(ctx, CreateResidentialUnit{Name: "Thôn Mới"}, n); err == nil || k.begun != 0 {
		t.Fatalf("thiếu mã cán bộ: lỗi %v, mở %d giao dịch", err, k.begun)
	}
}

// --- Update / deactivate ------------------------------------------------------------------------------

// TAKING A UNIT OUT OF USE writes ONE boolean on ONE row: no statement touches any other table, no
// soft-delete column is written, and the trail says who took it out of use.
func TestDeactivateResidentialUnit_KeepsReferences(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	u, err := uc.Update(ctx, "tt-1", UpdateResidentialUnit{Active: ptr(false)}, nguoiBoPhan())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if u.DangDung || u.Ma != "thon-binh-an" || u.Ten != "Thôn Bình An" || u.HeadStaffCode != "CB-2026-AAAAAA" || u.SoHo == nil || *u.SoHo != 284 {
		t.Errorf("chỉ `dang_dung` được đổi: %+v", u)
	}
	for _, l := range k.stmt {
		q := strings.TrimSpace(l.sql)
		if strings.HasPrefix(q, "DELETE") || strings.Contains(q, "deleted_at =") || strings.Contains(q, "deleted_by") {
			t.Errorf("ngưng dùng không được xoá gì: %q", q)
		}
		if (strings.HasPrefix(q, "UPDATE") && !strings.HasPrefix(q, "UPDATE thon_to_dan_pho SET")) ||
			(strings.HasPrefix(q, "INSERT") && !strings.HasPrefix(q, "INSERT INTO audit_log")) {
			t.Errorf("ngưng dùng chạm tới bảng khác: %q", q)
		}
	}
	upd := k.find("UPDATE thon_to_dan_pho")
	if len(upd) != 1 || upd[0].args[6] != false || upd[0].args[1] != "tt-1" || upd[0].args[7] != "nd-1" {
		t.Fatalf("UPDATE: %+v", upd)
	}
	vet := k.find("INSERT INTO audit_log")
	if len(vet) != 1 || vet[0].args[4] != ActionDeactivateResidentialUnit || vet[0].args[5] != "thon-binh-an" {
		t.Fatalf("vết: %+v", vet)
	}
	d := ruDelta(t, vet[0])
	if d["truoc"].(map[string]any)["dang_dung"] != true || d["sau"].(map[string]any)["dang_dung"] != false || len(d["sau"].(map[string]any)) != 1 {
		t.Errorf("delta chỉ mang trường đã đổi: %v", d)
	}
}

func TestReactivateResidentialUnit_OwnVerb(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	if _, err := uc.Update(ctx, "tt-2", UpdateResidentialUnit{Active: ptr(true)}, nguoiBoPhan()); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if vet := k.find("INSERT INTO audit_log"); len(vet) != 1 || vet[0].args[4] != ActionReactivateResidentialUnit {
		t.Fatalf("vết: %+v", vet)
	}
}

// A NO-OP WRITES NOTHING AND AUDITS NOTHING — what makes idem.KhongCan true.
func TestUpdateResidentialUnit_NoOpWritesNothing(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	if _, err := uc.Update(ctx, "tt-1", UpdateResidentialUnit{
		Name: ptr("Thôn Bình An"), Active: ptr(true), TypeCode: ptr("thon"), HeadCode: ptr("CB-2026-AAAAAA"),
		Households: OptionalCount{Set: true, Value: ptr(284)},
	}, nguoiBoPhan()); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if n := len(k.find("UPDATE thon_to_dan_pho SET")) + len(k.find("INSERT")); n != 0 {
		t.Errorf("không đổi gì mà có %d câu ghi", n)
	}
}

func TestUpdateResidentialUnit_ClearsAndSets(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	u, err := uc.Update(ctx, "tt-1", UpdateResidentialUnit{
		TypeCode: ptr(""), HeadCode: ptr(""), Households: OptionalCount{Set: true}, Population: OptionalCount{Set: true, Value: ptr(1132)},
	}, nguoiBoPhan())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if u.LoaiMa != "" || u.HeadStaffCode != "" || u.SoHo != nil || u.NhanKhau == nil || *u.NhanKhau != 1132 {
		t.Errorf("xoá / đặt: %+v", u)
	}
	upd := k.find("UPDATE thon_to_dan_pho")
	if len(upd) != 1 || upd[0].args[3] != "" || upd[0].args[4] != nil || upd[0].args[5] != int64(1132) || upd[0].args[7] != "" {
		t.Fatalf("UPDATE: %+v", upd)
	}
	if vet := k.find("INSERT INTO audit_log"); len(vet) != 1 || vet[0].args[4] != ActionUpdateResidentialUnit {
		t.Fatalf("vết: %+v", vet)
	}
}

func TestUpdateResidentialUnit_Refusals(t *testing.T) {
	for _, c := range []struct {
		name string
		id   string
		req  UpdateResidentialUnit
		want error
	}{
		{"đơn vị của xã khác", "tt-b", UpdateResidentialUnit{Active: ptr(false)}, idstore.ErrResidentialUnitNotFound},
		{"dòng đã xoá mềm", "tt-x", UpdateResidentialUnit{Active: ptr(false)}, idstore.ErrResidentialUnitNotFound},
		{"đổi tên trùng đơn vị đã ngưng dùng", "tt-1", UpdateResidentialUnit{Name: ptr("thôn cũ")}, idstore.ErrResidentialUnitNameTaken},
		{"đổi sang loại đã ngưng", "tt-1", UpdateResidentialUnit{TypeCode: ptr("cu")}, idstore.ErrResidentialUnitTypeNotFound},
		{"đổi trưởng thôn sang người bị khoá", "tt-1", UpdateResidentialUnit{HeadCode: ptr("CB-2026-LOCKED")}, idstore.ErrHeadStaffNotFound},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := ruFixture()
			uc, ctx := ruUseCase(t, k)
			if _, err := uc.Update(ctx, c.id, c.req, nguoiBoPhan()); !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if len(k.find("UPDATE thon_to_dan_pho SET")) != 0 || len(k.find("INSERT")) != 0 || k.committed != 0 {
				t.Error("bị từ chối mà vẫn ghi")
			}
		})
	}
}

// AFTER DEACTIVATION THE CODE IS STILL NOT FREE: the next create of the same name is refused, and a
// typed copy of the code is refused.
func TestDeactivatedUnitCodeNeverReissued(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruUseCase(t, k)
	if _, err := uc.Update(ctx, "tt-1", UpdateResidentialUnit{Active: ptr(false)}, nguoiBoPhan()); err != nil {
		t.Fatalf("ngưng dùng: %v", err)
	}
	k.units[0].active = false
	if _, err := uc.Create(ctx, CreateResidentialUnit{Name: "Thôn Mới", Code: "thon-binh-an"}, nguoiBoPhan()); !errors.Is(err, idstore.ErrResidentialUnitCodeTaken) {
		t.Errorf("mã của đơn vị vừa ngưng dùng được cấp lại: %v", err)
	}
	u, err := uc.Create(ctx, CreateResidentialUnit{Name: "Thôn Bình-An"}, nguoiBoPhan())
	if err != nil || u.Ma == "thon-binh-an" {
		t.Errorf("mã tự sinh trùng mã đã ngưng dùng phải có hậu tố: %q %v", u.Ma, err)
	}
}

// --- Import ---------------------------------------------------------------------------------------------

func ruImporter(t *testing.T, k *ruFakeDB) (*ResidentialUnitImporter, context.Context) {
	t.Helper()
	db, ctx := ruOpen(t, k)
	uc := NewResidentialUnitImporter(db, idstore.NewThonToDanPhoStore(db))
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("01JTHONNHAP%015d", n), nil
	}
	return uc, ctx
}

func ruImportRows() []domain.ResidentialUnitImportRow {
	return []domain.ResidentialUnitImportRow{
		{Row: 2, Name: "Thôn Hoà Bình", Type: "Thôn", Head: "CB-2026-AAAAAA · Cán Bộ Một", Households: "120", Population: "", Order: "2"},
		{Row: 3, Name: "Tổ dân phố số 1", Type: "thon", Code: "tdp-1"},
	}
}

func TestImportResidentialUnits_WritesAllWithBatchInOneTransaction(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruImporter(t, k)
	res, err := uc.Import(ctx, ruImportRows(), nguoiBoPhan())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Units) != 2 || k.begun != 1 || k.committed != 1 {
		t.Fatalf("units %d, mở %d commit %d", len(res.Units), k.begun, k.committed)
	}
	ins := k.find("INSERT INTO thon_to_dan_pho")
	if len(ins) != 2 || ins[0].args[3] != "thon-hoa-binh" || ins[0].args[8] != "nd-1" || ins[0].args[5] != int64(120) || ins[0].args[6] != nil || ins[1].args[3] != "tdp-1" {
		t.Fatalf("INSERT: %+v", ins)
	}
	vet := k.find("INSERT INTO audit_log")
	if len(vet) != 2 {
		t.Fatalf("có %d vết, muốn một vết cho mỗi đơn vị", len(vet))
	}
	batch := ""
	for i, v := range vet {
		d := ruDelta(t, v)
		if v.args[1] != maCanBoBoPhan || v.args[4] != ActionCreateResidentialUnit || d["nguon"] != residentialUnitImportSource || d["so_dong"] != float64(2) {
			t.Errorf("vết %d: %v %v", i, v.args, d)
		}
		lo, _ := d["lo_nhap"].(string)
		if lo == "" || (batch != "" && lo != batch) {
			t.Errorf("vết %d: lô nhập %q — mọi vết của một tệp mang CÙNG một mã lô", i, lo)
		}
		batch = lo
	}
}

// ONE BAD ROW = ZERO WRITES, and every error of the file is returned.
func TestImportResidentialUnits_AllOrNothing(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruImporter(t, k)
	rows := append(ruImportRows(), domain.ResidentialUnitImportRow{Row: 4, Name: "Thôn Bình An"})
	_, err := uc.Import(ctx, rows, nguoiBoPhan())
	var rej *ResidentialUnitImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) == 0 {
		t.Fatalf("lỗi = %v, muốn ResidentialUnitImportRejected", err)
	}
	for _, e := range rej.Errors {
		if e.Row != 4 {
			t.Errorf("chỉ dòng 4 sai, nhận lỗi ở dòng %d: %+v", e.Row, e)
		}
	}
	if len(k.find("INSERT")) != 0 || k.committed != 0 || k.rolledBack != 1 {
		t.Errorf("tệp có lỗi mà vẫn ghi: insert %d commit %d", len(k.find("INSERT")), k.committed)
	}
}

func TestImportResidentialUnits_LateFailureRollsBackEverything(t *testing.T) {
	for _, fail := range []string{"INSERT INTO audit_log", "INSERT INTO thon_to_dan_pho"} {
		k := ruFixture()
		k.failOn = fail
		uc, ctx := ruImporter(t, k)
		if _, err := uc.Import(ctx, ruImportRows(), nguoiBoPhan()); err == nil {
			t.Fatalf("%s hỏng mà vẫn nhập", fail)
		}
		if k.committed != 0 || k.rolledBack != 1 {
			t.Errorf("%s: commit %d rollback %d", fail, k.committed, k.rolledBack)
		}
	}
}

func TestPreviewResidentialUnits_WritesNothing(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruImporter(t, k)
	res, err := uc.Preview(ctx, ruImportRows())
	if err != nil || len(res.Units) != 2 || len(res.Errors) != 0 {
		t.Fatalf("Preview: %v %+v", err, res)
	}
	if len(k.find("INSERT")) != 0 || len(k.find("pg_advisory")) != 0 || k.committed != 0 || k.rolledBack != 1 {
		t.Errorf("xem trước phải không ghi, không khoá, luôn rollback")
	}
}

// ANOTHER COMMUNE'S TYPE AND STAFF ARE NOT CHOICES HERE.
func TestImportResidentialUnits_SnapshotIsThisCommune(t *testing.T) {
	k := ruFixture()
	uc, ctx := ruImporter(t, k)
	res, err := uc.Preview(ctx, []domain.ResidentialUnitImportRow{
		{Row: 2, Name: "Thôn A", Type: "khu-pho"},
		{Row: 3, Name: "Thôn B", Head: "CB-2026-XAKHAC"},
		{Row: 4, Name: "Thôn Đông"}, // commune B's name and code: free here
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.Errors) != 2 || res.Errors[0].Column != domain.ResidentialUnitImportColType || res.Errors[1].Column != domain.ResidentialUnitImportColHead {
		t.Errorf("RÒ RỈ hoặc sai lỗi: %+v", res.Errors)
	}
}
