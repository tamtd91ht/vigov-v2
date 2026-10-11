package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The staff-recorded rating (owner rule 11/10/2026, bug sheet row 60), over the REAL petition store on the
// fake driver and a recording fake of the record table.
//
//	PROVED HERE   the record and its audit entry are written in ONE transaction that commits once · the
//	              petition row is read FOR UPDATE and NEVER updated (no `diem_hai_long`, no status) · the
//	              record carries `staff_recorded`, the STAFF BUSINESS CODE, the channel and the trimmed note
//	              · the audit delta names the source and the note's LENGTH, never its text · refused with
//	              nothing written when the citizen has rated, when a staff record exists, on `can-bo`
//	              without the key, and on bad input (before any transaction).
//	NOT PROVED    the record table's SQL and CHECKs (internal/store, migrations/).

type staffRatingRecordsFake struct {
	existing  bool
	appended  []domain.StaffRecordedRating
	appendTx  []bool
	appendErr error
}

func (f *staffRatingRecordsFake) ForPetitionTx(_ context.Context, tx *pkgstore.ScopedTx, _ string) (
	domain.StaffRecordedRating, bool, error) {
	if tx == nil {
		return domain.StaffRecordedRating{}, false, errors.New("đọc ngoài giao dịch")
	}
	return domain.StaffRecordedRating{}, f.existing, nil
}

func (f *staffRatingRecordsFake) ForPetition(context.Context, string) (domain.StaffRecordedRating, bool, error) {
	return domain.StaffRecordedRating{}, f.existing, nil
}

func (f *staffRatingRecordsFake) Append(_ context.Context, tx *pkgstore.ScopedTx, r domain.StaffRecordedRating) error {
	f.appended = append(f.appended, r)
	f.appendTx = append(f.appendTx, tx != nil)
	return f.appendErr
}

var staffRatedAt = time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)

func buildStaffRatings(t *testing.T, k *khoPhieuXuLyGia, rec *staffRatingRecordsFake) (*StaffRecordedRatings, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := pkgstore.New(db)
	uc := NewStaffRecordedRatings(kho, petstore.NewPhieuPhanAnhStore(kho), rec)
	uc.newID = func() (string, error) { return "sr-01", nil }
	uc.now = func() time.Time { return staffRatedAt }
	return uc, ctxXa(xaThu)
}

func TestStaffRatingRecordAndTrailInOneTransaction(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DaXuLy), "linh_vuc": "rac-thai"})
	rec := &staffRatingRecordsFake{}
	uc, ctx := buildStaffRatings(t, k, rec)

	p, r, err := uc.Record(ctx, maPhieuThu,
		StaffRatingRequest{Stars: 4, Channel: " dien-thoai ", Note: "  Dân gọi điện, hài lòng.  "}, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(rec.appended) != 1 || !rec.appendTx[0] {
		t.Fatalf("bản ghi: %d lần, trong giao dịch=%v", len(rec.appended), rec.appendTx)
	}
	want := domain.StaffRecordedRating{ID: "sr-01", PetitionID: idPhieuThu, Stars: 4, Channel: domain.StaffRatingByPhone,
		Note: "Dân gọi điện, hài lòng.", Source: domain.StaffRecordedSource, RecordedBy: maCanBoThu, RecordedAt: staffRatedAt}
	if rec.appended[0] != want || r != want {
		t.Errorf("bản ghi = %+v, muốn %+v", rec.appended[0], want)
	}
	// THE PETITION IS NOT TOUCHED: the citizen's column and the status stay as read.
	if k.coCau("UPDATE phieu_phan_anh") || p.Rating != 0 || p.TrangThai != domain.DaXuLy {
		t.Errorf("phiếu bị sửa: UPDATE=%v rating=%d status=%s", k.coCau("UPDATE phieu_phan_anh"), p.Rating, p.TrangThai)
	}
	sel := k.cau("FROM phieu_phan_anh")
	if len(sel) != 1 || !strings.Contains(sel[0].sql, "FOR UPDATE") || !sel[0].trongGiaoDich {
		t.Errorf("phiếu không được khoá trong giao dịch: %v", sel)
	}
	audits := k.cau("INSERT INTO audit_log")
	if len(audits) != 1 || !audits[0].trongGiaoDich || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("vết: %d (trong giao dịch=%v) commit=%d rollback=%d", len(audits),
			len(audits) == 1 && audits[0].trongGiaoDich, k.daCommit, k.daRollback)
	}
	if !coThamSo(audits[0].args, maCanBoThu) || !coThamSo(audits[0].args, AuditActionStaffRecordedRating) ||
		!coThamSo(audits[0].args, maPhieuThu) {
		t.Errorf("vết: chủ thể/hành vi/đối tượng sai: %v", audits[0].args)
	}
	var delta string
	for _, a := range audits[0].args {
		switch v := a.(type) {
		case []byte:
			if json.Valid(v) {
				delta = string(v)
			}
		case string:
			if strings.HasPrefix(v, "{") {
				delta = v
			}
		}
	}
	for _, w := range []string{`"nguon":"staff_recorded"`, `"so_sao":4`, `"kenh":"dien-thoai"`, `"do_dai_ghi_chu":23`,
		`"dan_da_tu_danh_gia":false`} {
		if !strings.Contains(delta, w) {
			t.Errorf("delta thiếu %s: %s", w, delta)
		}
	}
	if strings.Contains(delta, "hài lòng") {
		t.Errorf("chữ ghi chú lọt vào vết kiểm toán: %s", delta)
	}
}

func TestStaffRatingRefusedWritesNothing(t *testing.T) {
	for name, c := range map[string]struct {
		row        map[string]any
		existing   bool
		restricted QuyenXemHanChe
		want       error
	}{
		"citizen already rated": {map[string]any{"trang_thai": string(domain.ChoDanXacNhan), "diem_hai_long": int64(2),
			"danh_gia_luc": mocThaoTac}, false, false, domain.ErrStaffRatingCitizenRated},
		"staff record exists":    {map[string]any{"trang_thai": string(domain.DaXuLy)}, true, false, domain.ErrStaffRatingAlreadyRecorded},
		"can-bo without the key": {map[string]any{"linh_vuc": domain.LinhVucHanChe}, false, false, ErrPhieuHanChe},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(c.row)
			rec := &staffRatingRecordsFake{existing: c.existing}
			uc, ctx := buildStaffRatings(t, k, rec)
			_, _, err := uc.Record(ctx, maPhieuThu, StaffRatingRequest{Stars: 5, Channel: "tai-quay"}, canBoThu(), c.restricted)
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if len(rec.appended) != 0 || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
				t.Errorf("từ chối mà vẫn ghi: bản ghi=%d vết=%v commit=%d", len(rec.appended),
					k.coCau("INSERT INTO audit_log"), k.daCommit)
			}
		})
	}
}

// Any status — the prototype's block has no status gate (FeedbackDetailDrawer.tsx:516).
func TestStaffRatingAnyStatus(t *testing.T) {
	for _, s := range []domain.TrangThai{domain.DaTiepNhan, domain.DangXuLy, domain.DaXuLy, domain.ChoDanXacNhan, domain.DaDong} {
		k := khoPhieuMau()
		k.hang = dongPhieuMau(map[string]any{"trang_thai": string(s)})
		uc, ctx := buildStaffRatings(t, k, &staffRatingRecordsFake{})
		if _, _, err := uc.Record(ctx, maPhieuThu, StaffRatingRequest{Stars: 3, Channel: "tai-quay"}, canBoThu(), khongQuyenHanChe); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
}

func TestStaffRatingBadInputNeverOpensTransaction(t *testing.T) {
	for name, c := range map[string]struct {
		req  StaffRatingRequest
		want error
	}{
		"zero stars": {StaffRatingRequest{Stars: 0, Channel: "tai-quay"}, domain.ErrRatingStarsOutOfRange},
		"six stars":  {StaffRatingRequest{Stars: 6, Channel: "tai-quay"}, domain.ErrRatingStarsOutOfRange},
		"no channel": {StaffRatingRequest{Stars: 3}, domain.ErrStaffRatingChannelInvalid},
		"zalo":       {StaffRatingRequest{Stars: 3, Channel: "zalo"}, domain.ErrStaffRatingChannelInvalid},
		"long note":  {StaffRatingRequest{Stars: 3, Channel: "tai-quay", Note: strings.Repeat("a", domain.RatingCommentMaxLen+1)}, domain.ErrStaffRatingNoteTooLong},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoPhieuMau()
			uc, ctx := buildStaffRatings(t, k, &staffRatingRecordsFake{})
			_, _, err := uc.Record(ctx, maPhieuThu, c.req, canBoThu(), khongQuyenHanChe)
			if !errors.Is(err, c.want) || !domain.IsStaffRatingInputError(err) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if k.batDau != 0 {
				t.Error("mở giao dịch cho một yêu cầu sai dạng")
			}
		})
	}
}
