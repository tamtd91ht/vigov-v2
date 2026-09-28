package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// What this file proves, over the REAL store on the fake driver (driver_gia_phieu_test.go):
//
//	the moderation UPDATE and its audit entry share ONE transaction · the UPDATE names only
//	`publication_status` — never `trang_thai` nor a deadline — and carries the expected old value ·
//	the audit entry is `dat_trang_thai_cong_khai` with before/after and the staff BUSINESS code ·
//	no timeline row and no outbox row · the same value twice writes nothing · staff conduct can never
//	be published · the restricted field refuses first, writing nothing · classifying INTO `can-bo`
//	sets `an` in the classifying UPDATE itself.
//
// NOT PROVED: PostgreSQL's CHECKs from migration 0017 (need a DSN — migrations/petition_publication_test.go
// reads the file text instead).

// auditDelta decodes the delta argument of the one audit INSERT.
func auditDelta(t *testing.T, k *khoPhieuXuLyGia) map[string]map[string]any {
	t.Helper()
	rows := k.cau("INSERT INTO audit_log")
	if len(rows) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(rows))
	}
	for _, a := range rows[0].args {
		var b []byte
		switch v := a.(type) {
		case []byte:
			b = v
		case string:
			b = []byte(v)
		default:
			continue
		}
		// Decoded loosely: a delta also carries scalars beside the two sides (`tre_tran_phan_loai`).
		var raw map[string]any
		if json.Unmarshal(b, &raw) != nil {
			continue
		}
		before, ok1 := raw["truoc"].(map[string]any)
		after, ok2 := raw["sau"].(map[string]any)
		if ok1 && ok2 {
			return map[string]map[string]any{"truoc": before, "sau": after}
		}
	}
	t.Fatalf("không tìm thấy delta trong vết: %v", rows[0].args)
	return nil
}

func TestSetPublicationWritesDecisionAndTrailInOneTransaction(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DangXuLy), "linh_vuc": "rac-thai"})
	uc, ctx := dungXuLy(t, k, han)

	after, err := uc.SetPublication(ctx, maPhieuThu, "cong-khai", canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("SetPublication: %v", err)
	}
	if after.PublicationStatus != domain.PublicationPublic {
		t.Errorf("PublicationStatus = %q, muốn cong-khai", after.PublicationStatus)
	}
	// THE STATUS IS UNCHANGED (user decision 28/09/2026 — the requirement's RECEIVED -> SCREENING
	// side effect is not copied).
	if after.TrangThai != domain.DangXuLy {
		t.Errorf("trạng thái xử lý đổi thành %q — kiểm duyệt không được đổi trạng thái", after.TrangThai)
	}

	upd := k.cau("UPDATE phieu_phan_anh")
	if len(upd) != 1 || !upd[0].trongGiaoDich {
		t.Fatalf("UPDATE: %d câu (trong giao dịch=%v), muốn 1 trong giao dịch", len(upd),
			len(upd) == 1 && upd[0].trongGiaoDich)
	}
	set := upd[0].sql[strings.Index(upd[0].sql, "SET"):strings.Index(upd[0].sql, "WHERE")]
	for _, forbidden := range []string{"trang_thai", "han_", "linh_vuc", "goc_dem_han"} {
		if strings.Contains(set, forbidden) {
			t.Errorf("câu UPDATE kiểm duyệt ghi cả %q: %q", forbidden, set)
		}
	}
	if !strings.Contains(upd[0].sql, "publication_status = $4") ||
		!coThamSo(upd[0].args, "cho-duyet") || !coThamSo(upd[0].args, "cong-khai") {
		t.Errorf("UPDATE không mang giá trị cũ làm điều kiện / giá trị mới: %q %v", upd[0].sql, upd[0].args)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || !vet[0].trongGiaoDich {
		t.Fatalf("vết: %d câu, muốn 1 trong giao dịch", len(vet))
	}
	if !coThamSo(vet[0].args, AuditActionSetPublication) || AuditActionSetPublication != "dat_trang_thai_cong_khai" {
		t.Errorf("vết không mang hành vi dat_trang_thai_cong_khai (chuỗi migration 0017 đã dùng): %v", vet[0].args)
	}
	if !coThamSo(vet[0].args, maCanBoThu) || !coThamSo(vet[0].args, maPhieuThu) {
		t.Errorf("vết không mang mã cán bộ / mã tra cứu: %v", vet[0].args)
	}
	d := auditDelta(t, k)
	if d["truoc"]["publication_status"] != "cho-duyet" || d["sau"]["publication_status"] != "cong-khai" {
		t.Errorf("delta trước/sau sai: %v", d)
	}
	if d["truoc"]["trang_thai"] != "dang-xu-ly" || d["sau"]["trang_thai"] != "dang-xu-ly" {
		t.Errorf("delta phải ghi trạng thái KHÔNG ĐỔI hai phía: %v", d)
	}

	// NO TIMELINE ROW (the requirement writes no event), NO OUTBOX ROW (not a transition, ADR 0041).
	if k.coCau("nhat_ky_phan_anh") || k.coCau("su_kien_di") {
		t.Error("kiểm duyệt ghi nhật ký hoặc sự kiện báo dân — không có trong bảng ADR 0041")
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit=%d rollback=%d, muốn 1/0", k.daCommit, k.daRollback)
	}
}

func TestSetPublicationTrailFailureRollsBackTheDecision(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.SetPublication(ctx, maPhieuThu, "an", canBoThu(), khongQuyenHanChe); err == nil {
		t.Fatal("thành công dù vết hỏng")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1 — quyết định công khai không vết là trạng thái luật 6 cấm",
			k.daCommit, k.daRollback)
	}
}

func TestSetPublicationSameValueWritesNothing(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.hang = dongPhieuMau(map[string]any{"publication_status": "an"})
	uc, ctx := dungXuLy(t, k, han)

	after, err := uc.SetPublication(ctx, maPhieuThu, "an", canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("PUT lần hai phải thành công: %v", err)
	}
	if after.PublicationStatus != domain.PublicationHidden {
		t.Errorf("PublicationStatus = %q, muốn an", after.PublicationStatus)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") {
		t.Error("cùng giá trị mà vẫn ghi — vết của một hành vi không xảy ra")
	}
}

func TestSetPublicationNeverPublishesStaffConduct(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.hang = dongPhieuMau(map[string]any{"linh_vuc": domain.LinhVucHanChe, "publication_status": "an"})
	uc, ctx := dungXuLy(t, k, han)

	_, err := uc.SetPublication(ctx, maPhieuThu, "cong-khai", canBoThu(), QuyenXemHanChe(true))
	if !errors.Is(err, domain.ErrNeverPublic) {
		t.Fatalf("err = %v, muốn ErrNeverPublic", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi")
	}
	// Hiding it is allowed (and here a no-op: it is already `an`).
	if _, err := uc.SetPublication(ctx, maPhieuThu, "an", canBoThu(), QuyenXemHanChe(true)); err != nil {
		t.Errorf("ẩn phiếu tác phong cán bộ bị từ chối: %v", err)
	}
}

// The restricted field refuses BEFORE the publication rule, so a caller without the key never learns
// from a 409 `never_public` that a staff-conduct report exists under this code.
func TestSetPublicationRestrictedFieldRefusesFirst(t *testing.T) {
	for _, to := range []string{"cong-khai", "an"} {
		k, han := khoPhieuMau(), hanXuLyThu()
		k.hang = dongPhieuMau(map[string]any{"linh_vuc": domain.LinhVucHanChe, "publication_status": "cho-duyet"})
		uc, ctx := dungXuLy(t, k, han)

		_, err := uc.SetPublication(ctx, maPhieuThu, to, canBoThu(), khongQuyenHanChe)
		if !errors.Is(err, ErrPhieuHanChe) {
			t.Errorf("%s: err = %v, muốn ErrPhieuHanChe (404)", to, err)
		}
		if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") {
			t.Errorf("%s: từ chối mà vẫn ghi", to)
		}
	}
}

func TestSetPublicationRefusesBadInputBeforeTheTransaction(t *testing.T) {
	for _, raw := range []string{"", "cho-duyet", "approved"} {
		k, han := khoPhieuMau(), hanXuLyThu()
		uc, ctx := dungXuLy(t, k, han)
		if _, err := uc.SetPublication(ctx, maPhieuThu, raw, canBoThu(), khongQuyenHanChe); !errors.Is(err,
			domain.ErrPublicationStatusInvalid) {
			t.Errorf("%q: err = %v, muốn ErrPublicationStatusInvalid", raw, err)
		}
		if k.batDau != 0 {
			t.Errorf("%q: mở giao dịch cho một yêu cầu sai dạng", raw)
		}
	}
}

func TestSetPublicationLostRaceIs409(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.doiDong = 0
	uc, ctx := dungXuLy(t, k, han)
	if _, err := uc.SetPublication(ctx, maPhieuThu, "cong-khai", canBoThu(), khongQuyenHanChe); !errors.Is(err,
		petstore.ErrPhieuDaChuyenTrang) {
		t.Errorf("err = %v, muốn ErrPhieuDaChuyenTrang", err)
	}
}

// Classifying INTO `can-bo` hides the petition IN THE SAME UPDATE — a petition published while
// unclassified would otherwise fail migration 0017's CHECK (the 500 its header warns about).
func TestClassifyIntoStaffConductSetsHidden(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.hang = dongPhieuMau(map[string]any{"publication_status": "cong-khai"})
	uc, ctx := dungXuLy(t, k, han)

	after, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: domain.LinhVucHanChe},
		canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	if after.PublicationStatus != domain.PublicationHidden {
		t.Errorf("PublicationStatus = %q, muốn an", after.PublicationStatus)
	}
	upd := k.cau("UPDATE phieu_phan_anh")
	if len(upd) != 1 || !strings.Contains(upd[0].sql, "publication_status = $8") || !coThamSo(upd[0].args, "an") {
		t.Fatalf("câu phân loại không đặt publication_status = 'an' cùng lúc: %v", upd)
	}
	d := auditDelta(t, k)
	if d["truoc"]["publication_status"] != "cong-khai" || d["sau"]["publication_status"] != "an" {
		t.Errorf("vết phân loại không ghi việc ẩn: %v", d)
	}
}

func TestClassifyIntoOtherFieldKeepsPublication(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.hang = dongPhieuMau(map[string]any{"publication_status": "cong-khai"})
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(),
		khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	upd := k.cau("UPDATE phieu_phan_anh")
	if len(upd) != 1 || !coThamSo(upd[0].args, "cong-khai") {
		t.Errorf("phân loại vào lĩnh vực thường làm mất quyết định công khai: %v", upd)
	}
}
