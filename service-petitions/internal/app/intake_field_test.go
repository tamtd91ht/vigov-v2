package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The citizen intake WITH a picked field (ADR 0050 point 1): the field is checked against the
// commune's offered catalogue BEFORE identity is asked, both clocks are fixed in ONE identity call for
// that field, and the row, its trail and its outbox row share one transaction. Uses the harness of
// gui_phan_anh_test.go (khoGia fake driver, hanGia identity fake, pinned clock and generators).

// resolveDueFixture is identity's resolve deadline for the picked field — deliberately NOT a round
// number of hours after mocGuiThu, so a local `.Add(n * time.Hour)` could not produce it (rule 10,
// forbidden #2: only identity counts working hours).
var resolveDueFixture = time.Date(2026, 9, 25, 3, 17, 0, 0, time.UTC)

func hanWithResolve() *hanGia {
	h := hanThu()
	h.tra[identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG] = resolveDueFixture
	return h
}

// intakeFieldsFake answers the catalogue check and records what it was asked.
type intakeFieldsFake struct {
	offered map[string]bool
	err     error
	calls   int
	commune tenant.ID
}

func (f *intakeFieldsFake) CheckCitizenIntakeField(ctx context.Context, code string) (string, error) {
	f.calls++
	f.commune = tenant.MustFrom(ctx)
	if f.err != nil {
		return "", f.err
	}
	if !f.offered[code] {
		return "", ErrFieldNotOffered
	}
	return code, nil
}

func buildIntakeWithFields(k *khoGia, kho *khoPhieuGia, han HanTiepNhanDoc, f CitizenIntakeFields) *GuiPhanAnh {
	uc := dungGui(k, kho, han)
	uc.fields = f
	return uc
}

func requestWithField(code string) YeuCauGuiPhanAnh {
	yc := ycThu()
	yc.Field = code
	return yc
}

func TestIntakeWithFieldFixesBothClocksInOneIdentityCall(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanWithResolve()
	f := &intakeFieldsFake{offered: map[string]bool{"rac-thai": true}}

	p, err := buildIntakeWithFields(k, kho, han, f).Gui(ctxXa(xaThu), requestWithField("rac-thai"), congDanThu())
	if err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if f.calls != 1 || f.commune != xaThu {
		t.Errorf("catalogue check calls=%d commune=%q", f.calls, f.commune)
	}
	// ONE call, for THAT field, naming BOTH clocks, from the instant stored as goc_dem_han.
	if han.goi != 1 {
		t.Fatalf("identity asked %d times, want 1 — two calls could read two calendars", han.goi)
	}
	if han.thayLinh[0] != "rac-thai" || !han.thayTuLuc[0].Equal(mocGuiThu) {
		t.Errorf("asked linh_vuc=%q from %v", han.thayLinh[0], han.thayTuLuc[0])
	}
	clocks := fmt.Sprint(han.thayCanMoc[0])
	want := fmt.Sprint([]identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN,
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG})
	if clocks != want {
		t.Errorf("clocks = %s, want %s", clocks, want)
	}
	// STORED AS GIVEN (rule 10, invariant 2): the field, both deadlines, the ceiling.
	row := kho.thay[0]
	if row.LinhVuc != "rac-thai" || !row.HanXuLyXong.Equal(resolveDueFixture) ||
		!row.HanTiepNhan.Equal(mocHanThu) || !row.HanPhanLoai.Equal(mocTranThu) {
		t.Errorf("row field=%q resolve=%v ack=%v ceiling=%v", row.LinhVuc, row.HanXuLyXong, row.HanTiepNhan, row.HanPhanLoai)
	}
	if row.PublicationStatus != domain.PublicationPending || p.LinhVuc != "rac-thai" {
		t.Errorf("publication=%q returned field=%q", row.PublicationStatus, p.LinhVuc)
	}
	// ONE transaction: row, trail, outbox — committed once.
	if len(k.lenh) != 3 || k.commit != 1 {
		t.Fatalf("statements=%d commit=%d, want 3/1", len(k.lenh), k.commit)
	}
	var delta map[string]any
	if err := json.Unmarshal(k.lenh[1].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	if delta["linh_vuc"] != "rac-thai" || delta["han_xu_ly_xong"] == nil {
		t.Errorf("trail does not record the field and the commitment it fixed: %v", delta)
	}
}

func TestIntakeFieldNotOfferedAsksNoOneAndWritesNothing(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanWithResolve()
	f := &intakeFieldsFake{offered: map[string]bool{"rac-thai": true}}

	_, err := buildIntakeWithFields(k, kho, han, f).Gui(ctxXa(xaThu), requestWithField("can-bo"), congDanThu())
	if !errors.Is(err, ErrFieldNotOffered) {
		t.Fatalf("err = %v, want ErrFieldNotOffered", err)
	}
	if han.goi != 0 || han.goiTran != 0 {
		t.Error("identity asked for a field the form does not offer — it would answer the default row silently")
	}
	if len(k.lenh) != 0 || len(kho.thay) != 0 {
		t.Error("a refused field wrote something")
	}
}

func TestIntakeFieldCatalogueUnavailableFailsClosed(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanWithResolve()
	f := &intakeFieldsFake{err: fmt.Errorf("%w: x", ErrFieldCatalogueUnavailable)}

	_, err := buildIntakeWithFields(k, kho, han, f).Gui(ctxXa(xaThu), requestWithField("rac-thai"), congDanThu())
	if !errors.Is(err, ErrFieldCatalogueUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if han.goi != 0 || len(k.lenh) != 0 {
		t.Error("platform down, yet identity was asked or a row written")
	}
}

func TestIntakeWithFieldButNoResolveDeadlineIsRefused(t *testing.T) {
	// identity answered only the acknowledge clock: a row with a field and NULL han_xu_ly_xong would be
	// a settled petition whose commitment nobody counts.
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	f := &intakeFieldsFake{offered: map[string]bool{"rac-thai": true}}

	_, err := buildIntakeWithFields(k, kho, han, f).Gui(ctxXa(xaThu), requestWithField("rac-thai"), congDanThu())
	if !errors.Is(err, ErrChuaAnDinhDuocHan) {
		t.Fatalf("err = %v, want ErrChuaAnDinhDuocHan", err)
	}
	if len(k.lenh) != 0 {
		t.Error("wrote a row without the resolve commitment")
	}
}

func TestIntakeWithFieldIdentityRefusalWritesNothing(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanWithResolve()
	han.loi = errors.New("rpc error: code = FailedPrecondition desc = no sla row")
	f := &intakeFieldsFake{offered: map[string]bool{"rac-thai": true}}

	_, err := buildIntakeWithFields(k, kho, han, f).Gui(ctxXa(xaThu), requestWithField("rac-thai"), congDanThu())
	if !errors.Is(err, ErrChuaAnDinhDuocHan) {
		t.Fatalf("err = %v — the handler answers 503 intake_not_configured only for this sentinel", err)
	}
	if len(k.lenh) != 0 || len(kho.thay) != 0 {
		t.Error("identity refused, yet a row was written")
	}
}

func TestIntakeWithFieldAndNoCheckerIsAWiringFault(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanWithResolve()
	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), requestWithField("rac-thai"), congDanThu())
	if err == nil || errors.Is(err, ErrFieldNotOffered) {
		t.Fatalf("err = %v — an unchecked field must never reach identity or the row", err)
	}
	if han.goi != 0 || len(k.lenh) != 0 {
		t.Error("unchecked field went on")
	}
}

// --- the catalogue check and the label adapter ---------------------------------------------------

// overridesFake is tier 2 as a plain list; the write methods are not reached by these reads.
type overridesFake struct{ rows []domain.NhanLinhVuc }

// vi-name-ok: implements the existing FieldOverrideStore method.
func (o overridesFake) DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error) {
	_ = tenant.MustFrom(ctx)
	return o.rows, nil
}
func (overridesFake) ForUpdate(context.Context, *store.ScopedTx, string) (domain.NhanLinhVuc, bool, error) {
	return domain.NhanLinhVuc{}, false, errors.New("not reached")
}
func (overridesFake) Upsert(context.Context, *store.ScopedTx, domain.NhanLinhVuc) error {
	return errors.New("not reached")
}

// tier1WithStaffConduct adds `can-bo` and a third active code to tier1Fake's two codes.
type tier1WithStaffConduct struct{ tier1Fake }

func (t *tier1WithStaffConduct) Fields(ctx context.Context) ([]domain.FieldDefault, error) {
	base, err := t.tier1Fake.Fields(ctx)
	if err != nil {
		return nil, err
	}
	return append(base,
		domain.FieldDefault{Code: "can-bo", DefaultLabel: "Thái độ cán bộ", SortOrder: 11, Active: true},
		domain.FieldDefault{Code: "giao-thong", DefaultLabel: "Giao thông", SortOrder: 3, Active: true}), nil
}

func catalogueForIntake(t1err error) *PetitionFieldCatalogue {
	t1 := &tier1WithStaffConduct{tier1Fake{err: t1err}}
	return NewPetitionFieldCatalogue(nil, t1, overridesFake{rows: []domain.NhanLinhVuc{
		{Ma: "giao-thong", Enabled: false},                     // switched off by the commune
		{Ma: "rac-thai", Nhan: "Rác thải xã A", Enabled: true}, // re-worded
	}})
}

func TestCitizenIntakeFieldCheckOneAnswerForEveryRefusal(t *testing.T) {
	c := catalogueForIntake(nil)
	ctx := tenant.Into(context.Background(), communeFields)

	if got, err := c.CheckCitizenIntakeField(ctx, "rac-thai"); err != nil || got != "rac-thai" {
		t.Fatalf("offered code refused: %q %v", got, err)
	}
	for name, code := range map[string]string{
		"unknown":          "khong-co",
		"retired":          "ma-cu",
		"switched off":     "giao-thong",
		"staff conduct":    "can-bo",
		"not code-shaped":  " rac-thai",
		"case trick":       "RAC-THAI",
		"prefix of a code": "rac",
		"empty":            "",
	} {
		if _, err := c.CheckCitizenIntakeField(ctx, code); !errors.Is(err, ErrFieldNotOffered) {
			t.Errorf("%s (%q): err = %v, want ErrFieldNotOffered", name, code, err)
		}
	}
}

func TestCitizenIntakeFieldCheckPlatformDown(t *testing.T) {
	c := catalogueForIntake(fmt.Errorf("%w: x", ErrFieldCatalogueUnavailable))
	_, err := c.CheckCitizenIntakeField(tenant.Into(context.Background(), communeFields), "rac-thai")
	if !errors.Is(err, ErrFieldCatalogueUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestEffectiveFieldLabelsOverrideElseDefaultNeverFiltered(t *testing.T) {
	labels, err := NewEffectiveFieldLabels(catalogueForIntake(nil)).DanhSach(tenant.Into(context.Background(), communeFields))
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]string{}
	for _, l := range labels {
		byCode[l.Ma] = l.Nhan
	}
	for code, want := range map[string]string{
		"rac-thai":   "Rác thải xã A",  // commune wording
		"giao-thong": "Giao thông",     // switched off — still labelled (old petitions carry it)
		"ma-cu":      "Mã đã ngừng",    // retired — still labelled
		"can-bo":     "Thái độ cán bộ", // default
	} {
		if byCode[code] != want {
			t.Errorf("%s label = %q, want %q", code, byCode[code], want)
		}
	}
	if len(labels) != 4 {
		t.Errorf("labels for %d codes, want every tier-1 code (4): %v", len(labels), byCode)
	}
}
