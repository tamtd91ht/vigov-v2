package store

import (
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The commune's petition-field label overrides — tier 2 of ADR 0026 — against the fake driver in
// driver_gia_test.go. NO PostgreSQL. What that file can and cannot prove is written at the top of
// it; read it before trusting a green run here.

func dungKhoNhan(k *khoGia) *NhanLinhVucStore {
	return NewNhanLinhVucStore(pkgstore.New(moKhoGia(k)))
}

func TestNhanLinhVucCauLenhBuocXaVaLocDungDong(t *testing.T) {
	k := &khoGia{hang: mauMotDong("nlv-001", "rac-thai", "Rác thải – Vệ sinh môi trường")}

	if _, err := dungKhoNhan(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	// The same shared assertion the two task catalogues use: $1 from the context, the
	// soft-delete predicate, a TOTAL order, and LIMIT = ceiling + 1.
	doiCauLenhCoXaVaLoc(t, k, xaThu, "nhan_linh_vuc", TranNhanLinhVuc)
}

func TestNhanLinhVucDocDungTungCot(t *testing.T) {
	k := &khoGia{hang: mauMotDong("nlv-001", "rac-thai", "Rác thải – Vệ sinh môi trường")}

	ds, err := dungKhoNhan(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("trả %d dòng, muốn 1", len(ds))
	}
	// `ma` and `nhan` are ADJACENT TEXT COLUMNS: swapping them in the Scan produces no error at
	// all, only a screen showing slugs where the commune's own wording belongs.
	if ds[0].ID != "nlv-001" || ds[0].Ma != "rac-thai" || ds[0].Nhan != "Rác thải – Vệ sinh môi trường" {
		t.Errorf("đọc sai cột: %+v", ds[0])
	}
}

func TestNhanLinhVucXaChuaDoiTenNaoThiTraLatRong(t *testing.T) {
	k := &khoGia{} // no overrides at all — the ordinary case for a new commune

	ds, err := dungKhoNhan(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		// AN EMPTY SET IS THE CORRECT ANSWER, NOT AN ERROR. A commune that has renamed nothing
		// shows the platform's default labels for all twelve codes; the codes are still valid.
		t.Fatalf("xã chưa đặt lại tên nào lại thành lỗi: %v", err)
	}
	if ds == nil || len(ds) != 0 {
		t.Errorf("muốn lát rỗng khác nil, được %#v", ds)
	}
}

func TestNhanLinhVucVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// The code set is CLOSED at twelve (ADR 0026), so a commune holding more than a hundred
	// overrides is not a commune with many opinions — it is an import that ran twice.
	hang := make([]hangGia, 0, TranNhanLinhVuc+1)
	for i := 0; i <= TranNhanLinhVuc; i++ {
		hang = append(hang, mauMotDong("nlv", "ma", "nhãn")...)
	}
	k := &khoGia{hang: hang}

	ds, err := dungKhoNhan(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, ErrQuaNhieuNhanLinhVuc) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuNhanLinhVuc", err)
	}
	// REFUSED, NOT TRUNCATED, and the rows already read are DROPPED rather than handed back:
	// returning a list the caller might render anyway is how a refusal turns back into a silent
	// truncation, one careless `if err != nil { log }` later.
	if ds != nil {
		t.Errorf("trả %d dòng kèm lỗi — phải trả nil", len(ds))
	}
}

func TestNhanLinhVucLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: connection refused")}

	_, err := dungKhoNhan(k).DanhSach(ctxXa(xaThu))
	if err == nil {
		t.Fatal("lỗi driver bị nuốt")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("lỗi gốc không được bọc bằng %%w: %v", err)
	}
}
