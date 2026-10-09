package app

// The residential unit on a petition — ADR 0088 §1, residential_unit.go.
//
//	PROVED HERE   on all three writing acts (citizen intake, staff intake, classification): a valid unit
//	              is asked of identity with the commune IN THE CONTEXT (the client puts it in gRPC
//	              metadata — core/identityclient's own suite proves the wire half) and the ID is what
//	              reaches the row · an unconfirmed unit refuses with ErrResidentialUnitNotActive and
//	              writes nothing · identity down refuses with ErrResidentialUnitCheckUnavailable and
//	              writes nothing, no code minted · no unit sent = identity not asked. Classification:
//	              the same value is a confirmation (no call, no UPDATE, no delta key); a different one
//	              is written IN the transaction with before/after ids in the audit delta. Breakdown: one
//	              batched names call, the "" row not asked, a failure refuses the read.
//
//	NOT PROVED    PostgreSQL's view of `thon_id IS NOT DISTINCT FROM` — the fake driver accepts any SQL.

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

const (
	unitA       = "01JUNITATRONGTEST00000000"
	unitB       = "01JUNITBTRONGTEST00000000"
	unitUnknown = "01JUNITUNKNOWNTEST0000000"
	unitAName   = "Thôn Một"
)

// activeUnitsFake answers identity's ResolveActiveResidentialUnits from a fixed set, recording the
// commune the context carried at each call.
type activeUnitsFake struct {
	active   map[string]string
	err      error
	calls    int
	asked    [][]string
	communes []tenant.ID
}

func (f *activeUnitsFake) ActiveResidentialUnits(ctx context.Context, ids []string) (map[string]string, error) {
	f.calls++
	f.asked = append(f.asked, ids)
	c, _ := tenant.From(ctx)
	f.communes = append(f.communes, c)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]string{}
	for _, id := range ids {
		if n, ok := f.active[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func activeUnits() *activeUnitsFake {
	return &activeUnitsFake{active: map[string]string{unitA: unitAName, unitB: "Tổ dân phố Hai"}}
}

// errIdentityDown is the shape identityclient returns for an unreachable identity.
var errIdentityDown = errors.New("identityclient: ResolveActiveResidentialUnits: Unavailable")

func assertAskedOnceForCommune(t *testing.T, f *activeUnitsFake, id string) {
	t.Helper()
	if f.calls != 1 || len(f.asked[0]) != 1 || f.asked[0][0] != id {
		t.Fatalf("identity được hỏi %d lần với %v, muốn đúng một lần với [%s]", f.calls, f.asked, id)
	}
	if f.communes[0] != xaThu {
		t.Errorf("ngữ cảnh lúc hỏi identity mang xã %q, muốn %q — xã đi trong metadata từ ngữ cảnh",
			f.communes[0], xaThu)
	}
}

// unitAuditDelta decodes the delta of an audit INSERT: $1 tenant · $2 actor · $3 kind · $4 ip · $5 action ·
// $6 subject · … · $8 delta.
func unitAuditDelta(t *testing.T, args []driver.Value) map[string]any {
	t.Helper()
	raw, ok := args[7].([]byte)
	if !ok {
		t.Fatalf("delta không phải []byte: %T", args[7])
	}
	var d map[string]any
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// --- staff intake ----------------------------------------------------------------------------------

func TestStaffIntakeResidentialUnitValidIsStoredAndAudited(t *testing.T) {
	k, kho, units := &khoGia{}, &khoPhieuGia{}, activeUnits()
	req := staffRequest()
	req.ResidentialUnitID = "  " + unitA + " "
	if _, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).WithResidentialUnits(units).
		Book(ctxXa(xaThu), req, staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	assertAskedOnceForCommune(t, units, unitA)
	if kho.thay[0].ThonID != unitA {
		t.Errorf("thon_id = %q, muốn mã đã kiểm %q (không bao giờ tên)", kho.thay[0].ThonID, unitA)
	}
	found := false
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") {
			found = true
			if d := unitAuditDelta(t, l.args); d["residential_unit_id"] != unitA {
				t.Errorf("delta residential_unit_id = %v", d["residential_unit_id"])
			}
		}
	}
	if !found {
		t.Fatal("không có vết")
	}
}

func TestStaffIntakeResidentialUnitRefusalsWriteNothing(t *testing.T) {
	for name, c := range map[string]struct {
		units *activeUnitsFake
		id    string
		want  error
	}{
		"không thuộc xã / đã ngưng": {activeUnits(), unitUnknown, ErrResidentialUnitNotActive},
		"quá dài":                   {activeUnits(), strings.Repeat("x", residentialUnitIDMax+1), ErrResidentialUnitNotActive},
		"identity xuống":            {&activeUnitsFake{err: errIdentityDown}, unitA, ErrResidentialUnitCheckUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			k, kho, han := &khoGia{}, &khoPhieuGia{}, staffDeadlines()
			req := staffRequest()
			req.ResidentialUnitID = c.id
			p, err := newStaffIntakeForTest(k, kho, han, &staffFieldsFake{}).WithResidentialUnits(c.units).
				Book(ctxXa(xaThu), req, staffOfficer())
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if len(k.lenh) != 0 || len(kho.thay) != 0 || p.MaTraCuu != "" {
				t.Errorf("lượt hỏng vẫn ghi %d câu lệnh / cấp mã %q", len(k.lenh), p.MaTraCuu)
			}
			if han.goi != 0 {
				t.Error("hỏi hạn dù thôn chưa được xác nhận — thôn được kiểm trước")
			}
		})
	}
}

func TestStaffIntakeNoResidentialUnitAsksNobody(t *testing.T) {
	k, kho, units := &khoGia{}, &khoPhieuGia{}, activeUnits()
	if _, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).WithResidentialUnits(units).
		Book(ctxXa(xaThu), staffRequest(), staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if units.calls != 0 || kho.thay[0].ThonID != "" {
		t.Errorf("không chọn thôn mà vẫn hỏi identity %d lần / ghi thon_id %q", units.calls, kho.thay[0].ThonID)
	}
}

func TestStaffIntakeResidentialUnitUnwiredIsRefused(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	req := staffRequest()
	req.ResidentialUnitID = unitA
	_, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).Book(ctxXa(xaThu), req, staffOfficer())
	if err == nil || len(kho.thay) != 0 {
		t.Fatalf("chưa nối bộ kiểm mà vẫn ghi thôn: err=%v", err)
	}
	if errors.Is(err, ErrResidentialUnitNotActive) {
		t.Error("lỗi nối dây bị trả như lỗi của người gửi (400)")
	}
}

// --- citizen intake --------------------------------------------------------------------------------

func TestCitizenIntakeResidentialUnit(t *testing.T) {
	t.Run("hợp lệ", func(t *testing.T) {
		k, kho, units := &khoGia{}, &khoPhieuGia{}, activeUnits()
		yc := ycThu()
		yc.ResidentialUnitID = unitB
		if _, err := dungGui(k, kho, hanThu()).WithResidentialUnits(units).Gui(ctxXa(xaThu), yc, congDanThu()); err != nil {
			t.Fatalf("Gui: %v", err)
		}
		assertAskedOnceForCommune(t, units, unitB)
		if kho.thay[0].ThonID != unitB {
			t.Errorf("thon_id = %q", kho.thay[0].ThonID)
		}
		for _, l := range k.lenh {
			if strings.Contains(l.sql, "INSERT INTO audit_log") {
				if d := unitAuditDelta(t, l.args); d["residential_unit_id"] != unitB {
					t.Errorf("delta residential_unit_id = %v", d["residential_unit_id"])
				}
			}
		}
	})
	for name, c := range map[string]struct {
		units *activeUnitsFake
		want  error
	}{
		"không thuộc xã": {&activeUnitsFake{active: map[string]string{}}, ErrResidentialUnitNotActive},
		"identity xuống": {&activeUnitsFake{err: errIdentityDown}, ErrResidentialUnitCheckUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
			yc := ycThu()
			yc.ResidentialUnitID = unitB
			p, err := dungGui(k, kho, han).WithResidentialUnits(c.units).Gui(ctxXa(xaThu), yc, congDanThu())
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if len(k.lenh) != 0 || len(kho.thay) != 0 || p.MaTraCuu != "" || han.goi != 0 {
				t.Errorf("lượt hỏng vẫn ghi %d câu / cấp mã %q / hỏi hạn %d lần", len(k.lenh), p.MaTraCuu, han.goi)
			}
		})
	}
	t.Run("không chọn thôn", func(t *testing.T) {
		units := activeUnits()
		if _, err := dungGui(&khoGia{}, &khoPhieuGia{}, hanThu()).WithResidentialUnits(units).
			Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
			t.Fatalf("Gui: %v", err)
		}
		if units.calls != 0 {
			t.Error("không chọn thôn mà vẫn hỏi identity")
		}
	})
}

// --- classification --------------------------------------------------------------------------------

// storedUnit is the `thon_id` the fixture petition holds (dongPhieuMau).
const storedUnit = "thon-001"

func classifyWithUnit(t *testing.T, units ActiveResidentialUnitChecker, unit *string) (
	*khoPhieuXuLyGia, *hanXuLyGia, domain.PhieuPhanAnh, error) {
	t.Helper()
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)
	uc.WithResidentialUnits(units, nil)
	after, err := uc.ChotLinhVuc(ctx, maPhieuThu,
		YeuCauChotLinhVuc{LinhVuc: "rac-thai", ResidentialUnitID: unit}, canBoThu(), khongQuyenHanChe)
	return k, han, after, err
}

func TestClassificationConfirmingUnitAsksNobodyAndWritesNoChange(t *testing.T) {
	for name, unit := range map[string]*string{
		"không gửi": nil, "gửi rỗng": strPtr(""), "gửi đúng giá trị đang lưu": strPtr(storedUnit),
	} {
		t.Run(name, func(t *testing.T) {
			units := activeUnits()
			k, _, after, err := classifyWithUnit(t, units, unit)
			if err != nil {
				t.Fatalf("ChotLinhVuc: %v", err)
			}
			if units.calls != 0 {
				t.Errorf("xác nhận thôn đang lưu mà vẫn hỏi identity %d lần", units.calls)
			}
			if k.coCau("SET thon_id") {
				t.Error("không đổi thôn mà vẫn ghi thon_id")
			}
			if after.ThonID != storedUnit {
				t.Errorf("thôn sau phân loại = %q, muốn giữ %q", after.ThonID, storedUnit)
			}
			d := unitAuditDelta(t, k.cau("INSERT INTO audit_log")[0].args)
			if _, ok := d["sau"].(map[string]any)["residential_unit_id"]; ok {
				t.Error("vết ghi đổi thôn dù thôn không đổi")
			}
		})
	}
}

func TestClassificationChangingUnitIsCheckedWrittenInTxAndAudited(t *testing.T) {
	units := activeUnits()
	k, _, after, err := classifyWithUnit(t, units, strPtr(unitA))
	if err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	assertAskedOnceForCommune(t, units, unitA)
	upd := k.cau("SET thon_id")
	if len(upd) != 1 {
		t.Fatalf("%d câu đổi thon_id, muốn 1", len(upd))
	}
	if !upd[0].trongGiaoDich {
		t.Error("đổi thôn chạy NGOÀI giao dịch phân loại (luật 6 bất biến 3)")
	}
	// $1 tenant · $2 id · $3 from · $4 to
	if a := upd[0].args; a[0] != string(xaThu) || a[1] != idPhieuThu || a[2] != storedUnit || a[3] != unitA {
		t.Errorf("đổi thon_id sai tham số: %v", a)
	}
	if after.ThonID != unitA {
		t.Errorf("thôn trả về = %q", after.ThonID)
	}
	trail := k.cau("INSERT INTO audit_log")
	if len(trail) != 1 || !trail[0].trongGiaoDich {
		t.Fatal("vết không nằm trong cùng giao dịch")
	}
	d := unitAuditDelta(t, trail[0].args)
	if d["truoc"].(map[string]any)["residential_unit_id"] != storedUnit ||
		d["sau"].(map[string]any)["residential_unit_id"] != unitA {
		t.Errorf("vết thiếu trước/sau của thôn: %v", d)
	}
	if strings.Contains(string(trail[0].args[7].([]byte)), unitAName) {
		t.Error("vết ghi TÊN thôn — tên là của identity hôm nay, vết ghi mã")
	}
	if k.daCommit != 1 {
		t.Errorf("commit = %d", k.daCommit)
	}
}

func TestClassificationUnitRefusalsWriteNothing(t *testing.T) {
	for name, c := range map[string]struct {
		units *activeUnitsFake
		want  error
	}{
		"không thuộc xã": {&activeUnitsFake{active: map[string]string{}}, ErrResidentialUnitNotActive},
		"identity xuống": {&activeUnitsFake{err: errIdentityDown}, ErrResidentialUnitCheckUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			k, han, _, err := classifyWithUnit(t, c.units, strPtr(unitA))
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if k.batDau != 0 || k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") {
				t.Error("lượt hỏng vẫn mở giao dịch / ghi")
			}
			if han.goi != 0 {
				t.Error("hỏi hạn dù thôn chưa được xác nhận")
			}
		})
	}
}

// --- breakdown -------------------------------------------------------------------------------------

type unitNamesFake struct {
	names map[string]identityclient.ResidentialUnitName
	err   error
	calls int
	asked []string
}

func (f *unitNamesFake) ResidentialUnitNamesInBatches(_ context.Context, ids []string) (
	map[string]identityclient.ResidentialUnitName, error) {
	f.calls++
	f.asked = append(f.asked, ids...)
	if f.err != nil {
		return nil, f.err
	}
	return f.names, nil
}

func TestBreakdownNamesResidentialUnitsInOneCall(t *testing.T) {
	st := &breakdownStoreFake{counts: domain.CitizenReportBreakdown{ResidentialUnits: []domain.CitizenReportResidentialUnitFigures{
		{ResidentialUnitID: "", Received: 2},
		{ResidentialUnitID: unitA, Received: 5},
		{ResidentialUnitID: unitB, Received: 1},
		{ResidentialUnitID: unitUnknown, Received: 1},
	}}}
	names := &unitNamesFake{names: map[string]identityclient.ResidentialUnitName{
		unitA: {Name: unitAName, Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE},
		unitB: {Name: "Tổ dân phố Hai", Standing: identityv1.RecordStanding_RECORD_STANDING_REMOVED},
	}}
	got, err := NewCitizenReportBreakdown(st, nil).WithResidentialUnitNames(names).Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if names.calls != 1 || len(names.asked) != 3 {
		t.Fatalf("tra tên %d lần với %v, muốn MỘT lần với 3 mã (không hỏi dòng \"\")", names.calls, names.asked)
	}
	ru := got.ResidentialUnits
	if ru[0].Name != "" || ru[1].Name != unitAName || ru[2].Name != "Tổ dân phố Hai" || ru[3].Name != "" {
		t.Errorf("tên thôn = %+v — đã gỡ vẫn hiện tên, không biết thì để trống, dòng \"\" để client đặt nhãn", ru)
	}
}

func TestBreakdownResidentialUnitNamesFailureRefusesRead(t *testing.T) {
	rows := func() *breakdownStoreFake {
		return &breakdownStoreFake{counts: domain.CitizenReportBreakdown{
			ResidentialUnits: []domain.CitizenReportResidentialUnitFigures{{ResidentialUnitID: unitA, Received: 1}}}}
	}
	for name, uc := range map[string]*CitizenReportBreakdown{
		"identity xuống": NewCitizenReportBreakdown(rows(), nil).WithResidentialUnitNames(&unitNamesFake{err: errIdentityDown}),
		"chưa nối dây":   NewCitizenReportBreakdown(rows(), nil),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := uc.Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe); !errors.Is(err, ErrResidentialUnitNamesUnavailable) {
				t.Errorf("err = %v, muốn ErrResidentialUnitNamesUnavailable — không bao giờ bảng tên thôn trống", err)
			}
		})
	}
	// Only the no-unit row: nobody is asked, the read stands.
	names := &unitNamesFake{}
	st := &breakdownStoreFake{counts: domain.CitizenReportBreakdown{
		ResidentialUnits: []domain.CitizenReportResidentialUnitFigures{{Received: 3}}}}
	if _, err := NewCitizenReportBreakdown(st, nil).WithResidentialUnitNames(names).
		Read(ctxXa(xaThu), brPeriod, khongQuyenHanChe); err != nil || names.calls != 0 {
		t.Errorf("chỉ có dòng chưa xác định địa bàn: err=%v, tra tên %d lần", err, names.calls)
	}
}
