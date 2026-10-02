package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The STAFF intake use case — staff_intake.go.
//
//	PROVED HERE   only the RESOLVE clock is asked, for the picked field, from `goc_dem_han` · the
//	              field is checked BEFORE identity · `han_tiep_nhan` and `han_phan_loai` stay NULL ·
//	              `cong_dan_id` stays empty and no outbox row is written · the 7-day window refuses
//	              (never clamps) at both ends and asks nobody · a typed origin is audited before/after,
//	              the default one is not · the row and its trail share ONE committed transaction · the
//	              trail names the officer's business code · nothing is produced on any refusal.
//
//	NOT PROVED    PostgreSQL's CHECKs (0004's `nhap_ho_*`, `goc_dem_trong_khoang`) — the fake driver
//	              accepts any SQL; the store's pg suite skips without VIGOV_TEST_DSN.

type staffFieldsFake struct {
	calls int
	asked string
	err   error
}

func (f *staffFieldsFake) CheckStaffIntakeField(_ context.Context, code string) (string, error) {
	f.calls++
	f.asked = code
	if f.err != nil {
		return "", f.err
	}
	return code, nil
}

var (
	// The booking instant, and a resolve deadline deliberately NOT a round number of hours from either
	// origin — a local `.Add(n * time.Hour)` must not be able to pass.
	staffBookedAt   = time.Date(2026, 10, 2, 2, 7, 11, 0, time.UTC)
	staffResolveDue = time.Date(2026, 10, 9, 3, 41, 0, 0, time.UTC)
)

func staffOfficer() audit.Actor {
	return audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
}

func staffRequest() StaffIntakeRequest {
	return StaffIntakeRequest{
		Content:       "Bà con gọi điện báo đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		Address:       "Đầu ngõ thôn Hà Lam",
		ReporterName:  "Nguyễn Văn An",
		ReporterPhone: "0900000000", // the agreed fake number (rule 3, invariant 5)
		Field:         "rac-thai",
	}
}

func staffDeadlines() *hanGia {
	return &hanGia{tra: map[identityv1.DeadlineKind]time.Time{
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG: staffResolveDue,
	}}
}

// staffIntakeStoreFake is the citizen intake's petition fake plus the REAL timeline writer, so the row the
// fake driver sees is the store's own INSERT with its own argument order.
type staffIntakeStoreFake struct {
	rows   *khoPhieuGia
	log    *petstore.PhieuPhanAnhStore
	logErr error // when set, the timeline write fails after its statement ran
}

func (k *staffIntakeStoreFake) Tao(ctx context.Context, tx *pkgstore.ScopedTx, p domain.PhieuPhanAnh) error { // vi-name-ok: implements the existing store method
	return k.rows.Tao(ctx, tx, p)
}

func (k *staffIntakeStoreFake) GhiNhatKy(ctx context.Context, tx *pkgstore.ScopedTx, e domain.NhatKyPhanAnh) error { // vi-name-ok: implements the existing store method
	if err := k.log.GhiNhatKy(ctx, tx, e); err != nil {
		return err
	}
	return k.logErr
}

func newStaffIntakeForTest(k *khoGia, kho *khoPhieuGia, han ResolveDeadlineReader,
	fields StaffIntakeFields) *StaffIntake {
	db := pkgstore.New(sql.OpenDB(k))
	uc := NewStaffIntake(db, &staffIntakeStoreFake{rows: kho, log: petstore.NewPhieuPhanAnhStore(db)},
		petstore.NewSuKienDiStore(db), han, fields)
	uc.newID = func() (string, error) { return idCoDinh, nil }
	uc.newCode = func() (string, error) { return maCoDinh, nil }
	uc.clock = func() time.Time { return staffBookedAt }
	return uc
}

func TestStaffIntakeAsksOnlyTheResolveClockForTheField(t *testing.T) {
	k, kho, han, fields := &khoGia{}, &khoPhieuGia{}, staffDeadlines(), &staffFieldsFake{}
	if _, err := newStaffIntakeForTest(k, kho, han, fields).Book(ctxXa(xaThu), staffRequest(), staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if fields.calls != 1 || fields.asked != "rac-thai" {
		t.Errorf("kiểm lĩnh vực %d lần với %q", fields.calls, fields.asked)
	}
	if han.goi != 1 {
		t.Fatalf("hỏi hạn %d lần, muốn 1", han.goi)
	}
	if han.thayLoai[0] != identityv1.WorkKind_WORK_KIND_PHAN_ANH || han.thayLinh[0] != "rac-thai" {
		t.Errorf("hỏi loại %v lĩnh vực %q", han.thayLoai[0], han.thayLinh[0])
	}
	if !han.thayTuLuc[0].Equal(staffBookedAt) {
		t.Errorf("gốc đếm = %v, muốn lúc vào sổ %v khi cán bộ không ghi mốc", han.thayTuLuc[0], staffBookedAt)
	}
	if c := han.thayCanMoc[0]; len(c) != 1 || c[0] != identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG {
		t.Errorf("đồng hồ đã hỏi = %v, muốn CHỈ XU_LY_XONG — hạn tiếp nhận của phiếu nhập hộ là NULL (ADR 0028 F5)", c)
	}
	if han.goiTran != 0 {
		t.Error("hỏi trần phân loại — phiếu nhập hộ đến đã phân loại, han_phan_loai phải NULL")
	}
}

func TestStaffIntakeRowShape(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	p, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).
		Book(ctxXa(xaThu), staffRequest(), staffOfficer())
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if len(kho.thay) != 1 {
		t.Fatalf("ghi %d dòng", len(kho.thay))
	}
	row := kho.thay[0]
	if row.Kenh != domain.KenhCanBoNhapHo {
		t.Errorf("kenh = %q", row.Kenh)
	}
	if row.CongDanID != "" {
		t.Errorf("cong_dan_id = %q — phiếu nhập hộ không gắn tài khoản nào (ADR 0028 §Bổ sung 2026-10-02)", row.CongDanID)
	}
	if !row.HanTiepNhanKhongApDung() || !row.TranPhanLoaiKhongApDung() {
		t.Errorf("han_tiep_nhan = %v, han_phan_loai = %v — cả hai phải NULL, không bao giờ 0 giờ",
			row.HanTiepNhan, row.HanPhanLoai)
	}
	if !row.HanXuLyXong.Equal(staffResolveDue) {
		t.Errorf("han_xu_ly_xong = %v, muốn nguyên văn identity trả %v", row.HanXuLyXong, staffResolveDue)
	}
	if row.LinhVuc != "rac-thai" || row.TrangThai != domain.DaTiepNhan || !row.PhanLoaiLuc.IsZero() {
		t.Errorf("linh_vuc %q, trang_thai %q, phan_loai_luc %v", row.LinhVuc, row.TrangThai, row.PhanLoaiLuc)
	}
	if !row.GocDemHan.Equal(staffBookedAt) || !row.VaoSoLuc.Equal(staffBookedAt) {
		t.Errorf("goc_dem_han %v, vao_so_luc %v, muốn cả hai = lúc vào sổ", row.GocDemHan, row.VaoSoLuc)
	}
	if row.PublicationStatus != domain.PublicationPending {
		t.Errorf("publication_status = %q", row.PublicationStatus)
	}
	if row.NguoiGuiHoTen != "Nguyễn Văn An" || row.NguoiGuiDienThoai != "0900000000" {
		t.Error("người gửi cán bộ gõ không được lưu")
	}
	if p.MaTraCuu != maCoDinh {
		t.Errorf("mã trả về = %q", p.MaTraCuu)
	}
}

// `can-bo` is born hidden from the public page, as on every other path.
func TestStaffIntakeRestrictedFieldIsBornHidden(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	req := staffRequest()
	req.Field = domain.LinhVucHanChe
	if _, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).
		Book(ctxXa(xaThu), req, staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if kho.thay[0].PublicationStatus != domain.PublicationHidden {
		t.Errorf("publication_status = %q, muốn an", kho.thay[0].PublicationStatus)
	}
}

func TestStaffIntakeOneTransactionNoOutboxRow(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	if _, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).
		Book(ctxXa(xaThu), staffRequest(), staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if len(k.lenh) != 3 {
		t.Fatalf("chạy %d câu lệnh, muốn 3 (phiếu + nhật ký + vết; KHÔNG sự kiện vì không có người nhận): %v", len(k.lenh), k.lenh)
	}
	if !strings.Contains(k.lenh[0].sql, "INSERT INTO phieu_phan_anh") ||
		!strings.Contains(k.lenh[1].sql, "INSERT INTO nhat_ky_phan_anh") ||
		!strings.Contains(k.lenh[2].sql, "INSERT INTO audit_log") {
		t.Errorf("thứ tự câu lệnh sai: %q / %q / %q", k.lenh[0].sql, k.lenh[1].sql, k.lenh[2].sql)
	}
	for i, l := range k.lenh {
		if !l.trongGiaoDich {
			t.Errorf("câu lệnh %d chạy NGOÀI giao dịch (luật 6 bất biến 3)", i)
		}
	}
	if k.commit != 1 || k.rollback != 0 {
		t.Errorf("commit=%d rollback=%d", k.commit, k.rollback)
	}
	// $1 tenant · $2 actor · $3 kind · $4 ip · $5 action · $6 subject
	a := k.lenh[2].args
	if a[0] != string(xaThu) || a[1] != "CB-00123" || a[2] != "staff" || a[4] != ActionStaffIntake || a[5] != maCoDinh {
		t.Errorf("vết sai: %v", a[:6])
	}
}

// ADR 0028 Bổ sung 2026-10-02 row 6: ONE timeline row, `nhap-ho`, in the petition's transaction — the
// officer's business code, the booking instant, the status the petition is born in, no note.
//
// Argument order is the store's INSERT: tenant, id, petition id, at, author, action, status, unit,
// assignee, note.
func TestStaffIntakeWritesOneTimelineRow(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	if _, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).
		Book(ctxXa(xaThu), staffRequest(), staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	var rows []int
	for i, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO nhat_ky_phan_anh") {
			rows = append(rows, i)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("%d dòng nhật ký, muốn đúng 1", len(rows))
	}
	r := k.lenh[rows[0]]
	if !r.trongGiaoDich {
		t.Error("dòng nhật ký ghi ngoài giao dịch (luật 6 bất biến 3)")
	}
	if len(r.args) != 10 {
		t.Fatalf("dòng nhật ký có %d tham số, muốn 10", len(r.args))
	}
	if r.args[0] != string(xaThu) || r.args[2] != idCoDinh || r.args[4] != "CB-00123" ||
		r.args[5] != string(domain.LogActionStaffIntake) || r.args[6] != string(domain.DaTiepNhan) ||
		r.args[7] != nil || r.args[8] != nil || r.args[9] != nil {
		t.Errorf("dòng nhật ký sai: %v", r.args)
	}
	if at, _ := r.args[3].(time.Time); !at.Equal(staffBookedAt) {
		t.Errorf("thời điểm = %v, muốn lúc vào sổ %v", r.args[3], staffBookedAt)
	}
	for _, cam := range []string{"Nguyễn Văn An", "0900000000", "Hà Lam"} {
		for _, a := range r.args {
			if s, ok := a.(string); ok && strings.Contains(s, cam) {
				t.Errorf("dòng nhật ký mang dữ liệu cá nhân %q", cam)
			}
		}
	}
	if k.commit != 1 {
		t.Errorf("commit = %d", k.commit)
	}
}

// The timeline row failing rolls the petition back with it: never a petition with no "who booked it".
func TestStaffIntakeTimelineFailureLeavesNothing(t *testing.T) {
	k := &khoGia{}
	uc := newStaffIntakeForTest(k, &khoPhieuGia{}, staffDeadlines(), &staffFieldsFake{})
	uc.petitions.(*staffIntakeStoreFake).logErr = errors.New("pg: nhat_ky_phan_anh_hanh_vi_hop_le")
	p, err := uc.Book(ctxXa(xaThu), staffRequest(), staffOfficer())
	if err == nil || k.commit != 0 || k.rollback != 1 || p.MaTraCuu != "" {
		t.Errorf("err=%v commit=%d rollback=%d mã=%q", err, k.commit, k.rollback, p.MaTraCuu)
	}
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") {
			t.Error("the trail was written although the timeline row failed")
		}
	}
}

func TestStaffIntakeAuditFailureLeavesNothing(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: audit_log read only")}
	kho := &khoPhieuGia{}
	p, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).
		Book(ctxXa(xaThu), staffRequest(), staffOfficer())
	if err == nil || k.commit != 0 || k.rollback != 1 || p.MaTraCuu != "" {
		t.Errorf("err=%v commit=%d rollback=%d mã=%q", err, k.commit, k.rollback, p.MaTraCuu)
	}
}

func TestStaffIntakeDeltaHasNoPersonalDataAndNullClocks(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	if _, err := newStaffIntakeForTest(k, kho, staffDeadlines(), &staffFieldsFake{}).
		Book(ctxXa(xaThu), staffRequest(), staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	raw := k.lenh[2].args[7].([]byte)
	for _, cam := range []string{"Nguyễn Văn An", "0900000000", "Đống rác", "đống rác", "Hà Lam"} {
		if strings.Contains(string(raw), cam) {
			t.Errorf("delta mang dữ liệu cá nhân %q: %s", cam, raw)
		}
	}
	var d map[string]any
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if v, ok := d["han_tiep_nhan"]; !ok || v != nil {
		t.Errorf("han_tiep_nhan trong delta = %v (có=%v), muốn null — 'không áp dụng'", v, ok)
	}
	if d["kenh_tiep_nhan"] != string(domain.KenhCanBoNhapHo) || d["linh_vuc"] != "rac-thai" {
		t.Errorf("delta: %s", raw)
	}
	if _, ok := d["clock_from_override"]; ok {
		t.Error("mốc mặc định (lúc vào sổ) bị ghi như một mốc cán bộ tự gõ")
	}
}

// ADR 0028 F1 + F4: a typed origin is used for the deadline AND audited with before/after.
func TestStaffIntakeTypedClockFromIsUsedAndAudited(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, staffDeadlines()
	req := staffRequest()
	typed := staffBookedAt.Add(-50 * time.Hour).In(time.FixedZone("ICT", 7*3600))
	req.ClockFrom = typed
	if _, err := newStaffIntakeForTest(k, kho, han, &staffFieldsFake{}).Book(ctxXa(xaThu), req, staffOfficer()); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if !han.thayTuLuc[0].Equal(typed) || !kho.thay[0].GocDemHan.Equal(typed) {
		t.Errorf("gốc đếm hỏi %v, ghi %v, muốn %v", han.thayTuLuc[0], kho.thay[0].GocDemHan, typed)
	}
	if !kho.thay[0].VaoSoLuc.Equal(staffBookedAt) {
		t.Errorf("vao_so_luc = %v, muốn lúc vào sổ", kho.thay[0].VaoSoLuc)
	}
	var d struct {
		Override *struct {
			Before time.Time `json:"before"`
			After  time.Time `json:"after"`
		} `json:"clock_from_override"`
	}
	if err := json.Unmarshal(k.lenh[2].args[7].([]byte), &d); err != nil {
		t.Fatal(err)
	}
	if d.Override == nil || !d.Override.Before.Equal(staffBookedAt) || !d.Override.After.Equal(typed) {
		t.Errorf("vết mốc cán bộ gõ thiếu trước/sau: %+v", d.Override)
	}
}

// ADR 0028 F3: REFUSED at both ends, never clamped, and nothing asked or written.
func TestStaffIntakeClockFromWindowRefuses(t *testing.T) {
	for name, at := range map[string]time.Time{
		"sau lúc vào sổ":       staffBookedAt.Add(time.Second),
		"sớm hơn 7 ngày":       staffBookedAt.AddDate(0, 0, -7).Add(-time.Second),
		"rất xa trong quá khứ": staffBookedAt.AddDate(-1, 0, 0),
	} {
		t.Run(name, func(t *testing.T) {
			k, kho, han, fields := &khoGia{}, &khoPhieuGia{}, staffDeadlines(), &staffFieldsFake{}
			req := staffRequest()
			req.ClockFrom = at
			_, err := newStaffIntakeForTest(k, kho, han, fields).Book(ctxXa(xaThu), req, staffOfficer())
			if !errors.Is(err, domain.ErrGocDemHanNgoaiKhoang) {
				t.Fatalf("err = %v, muốn ErrGocDemHanNgoaiKhoang", err)
			}
			if han.goi+fields.calls+len(k.lenh) != 0 {
				t.Errorf("mốc ngoài khoảng mà vẫn hỏi/ghi: hạn %d, lĩnh vực %d, câu lệnh %d", han.goi, fields.calls, len(k.lenh))
			}
		})
	}
	// The two boundaries themselves are inside.
	for name, at := range map[string]time.Time{
		"đúng lúc vào sổ":   staffBookedAt,
		"đúng 7 ngày trước": staffBookedAt.AddDate(0, 0, -7),
	} {
		t.Run(name, func(t *testing.T) {
			req := staffRequest()
			req.ClockFrom = at
			if _, err := newStaffIntakeForTest(&khoGia{}, &khoPhieuGia{}, staffDeadlines(), &staffFieldsFake{}).
				Book(ctxXa(xaThu), req, staffOfficer()); err != nil {
				t.Errorf("biên %s bị từ chối: %v", name, err)
			}
		})
	}
}

func TestStaffIntakeRefusalsProduceNothing(t *testing.T) {
	cases := map[string]struct {
		mutate func(*StaffIntakeRequest, *staffFieldsFake, *hanGia)
		want   error
	}{
		"thiếu lĩnh vực":         {func(r *StaffIntakeRequest, _ *staffFieldsFake, _ *hanGia) { r.Field = " " }, domain.ErrThieuLinhVuc},
		"thiếu nội dung":         {func(r *StaffIntakeRequest, _ *staffFieldsFake, _ *hanGia) { r.Content = "  " }, domain.ErrNoiDungTrong},
		"lĩnh vực xã không nhận": {func(_ *StaffIntakeRequest, f *staffFieldsFake, _ *hanGia) { f.err = ErrFieldNotOffered }, ErrFieldNotOffered},
		"chưa đọc được bộ mã":    {func(_ *StaffIntakeRequest, f *staffFieldsFake, _ *hanGia) { f.err = ErrFieldCatalogueUnavailable }, ErrFieldCatalogueUnavailable},
		"identity từ chối":       {func(_ *StaffIntakeRequest, _ *staffFieldsFake, h *hanGia) { h.loi = errors.New("FailedPrecondition") }, ErrChuaAnDinhDuocHan},
		"identity trả rỗng": {func(_ *StaffIntakeRequest, _ *staffFieldsFake, h *hanGia) {
			h.tra = map[identityv1.DeadlineKind]time.Time{}
		}, ErrChuaAnDinhDuocHan},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			k, kho, han, fields := &khoGia{}, &khoPhieuGia{}, staffDeadlines(), &staffFieldsFake{}
			req := staffRequest()
			c.mutate(&req, fields, han)
			p, err := newStaffIntakeForTest(k, kho, han, fields).Book(ctxXa(xaThu), req, staffOfficer())
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, muốn %v", err, c.want)
			}
			if len(k.lenh) != 0 || p.MaTraCuu != "" {
				t.Errorf("lượt hỏng vẫn ghi %d câu lệnh / cấp mã %q", len(k.lenh), p.MaTraCuu)
			}
			for _, cam := range []string{"Nguyễn Văn An", "0900000000", maCoDinh} {
				if strings.Contains(err.Error(), cam) {
					t.Errorf("lỗi mang %q: %v", cam, err)
				}
			}
		})
	}
	// The field is checked BEFORE identity: a refused field asks identity nothing.
	han, fields := staffDeadlines(), &staffFieldsFake{err: ErrFieldNotOffered}
	_, _ = newStaffIntakeForTest(&khoGia{}, &khoPhieuGia{}, han, fields).Book(ctxXa(xaThu), staffRequest(), staffOfficer())
	if han.goi != 0 {
		t.Error("hỏi identity cho một lĩnh vực xã không nhận — identity trả dòng mặc định, lặng lẽ")
	}
}

func TestStaffIntakeRefusesNonStaffActor(t *testing.T) {
	for name, a := range map[string]audit.Actor{
		"không mã": {Kind: "staff", IP: "10.0.0.7"},
		"công dân": {ID: idCongDan, Kind: "citizen", IP: "10.0.0.9"},
	} {
		t.Run(name, func(t *testing.T) {
			k, han := &khoGia{}, staffDeadlines()
			if _, err := newStaffIntakeForTest(k, &khoPhieuGia{}, han, &staffFieldsFake{}).
				Book(ctxXa(xaThu), staffRequest(), a); err == nil {
				t.Fatal("nhận phiếu không có cán bộ đứng tên")
			}
			if han.goi != 0 || len(k.lenh) != 0 {
				t.Error("chạm identity/cơ sở dữ liệu dù chủ thể sai")
			}
		})
	}
}

func TestStaffIntakeNilFieldCheckerIsWiringFault(t *testing.T) {
	k, han := &khoGia{}, staffDeadlines()
	if _, err := newStaffIntakeForTest(k, &khoPhieuGia{}, han, nil).Book(ctxXa(xaThu), staffRequest(), staffOfficer()); err == nil {
		t.Fatal("không có bộ kiểm lĩnh vực mà vẫn nhận phiếu")
	}
	if han.goi != 0 {
		t.Error("hỏi identity khi chưa kiểm lĩnh vực")
	}
}

// --- the staff field rule (PetitionFieldCatalogue.CheckStaffIntakeField) ---------------------------

// The staff modal offers what the commune offers — active and enabled — AND `can-bo`, which the
// citizen form never offers (domain.PetitionFieldView.OfferedToStaffIntake).
func TestStaffIntakeFieldCheck(t *testing.T) {
	c := catalogueForIntake(nil)
	ctx := tenant.Into(context.Background(), communeFields)
	for _, code := range []string{"rac-thai", "can-bo"} {
		if got, err := c.CheckStaffIntakeField(ctx, code); err != nil || got != code {
			t.Errorf("%q bị từ chối: %q %v", code, got, err)
		}
	}
	for name, code := range map[string]string{
		"unknown":      "khong-co",
		"retired":      "ma-cu",
		"switched off": "giao-thong",
		"case trick":   "RAC-THAI",
		"empty":        "",
	} {
		if _, err := c.CheckStaffIntakeField(ctx, code); !errors.Is(err, ErrFieldNotOffered) {
			t.Errorf("%s (%q): err = %v, want ErrFieldNotOffered", name, code, err)
		}
	}
	if _, err := catalogueForIntake(fmt.Errorf("%w: x", ErrFieldCatalogueUnavailable)).
		CheckStaffIntakeField(ctx, "rac-thai"); !errors.Is(err, ErrFieldCatalogueUnavailable) {
		t.Errorf("platform down: err = %v", err)
	}
}
