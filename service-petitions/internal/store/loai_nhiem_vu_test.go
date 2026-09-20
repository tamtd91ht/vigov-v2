package store

import (
	"context"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// The task-type catalogue read, against the fake driver in driver_gia_test.go — NO PostgreSQL.
// What this can and cannot prove is written out at the top of that file; read it before trusting a
// green run here.

func dungKhoLoai(k *khoGia) *LoaiNhiemVuStore {
	return NewLoaiNhiemVuStore(pkgstore.New(moKhoGia(k)))
}

func mauLoai() []hangGia { return mauMotDong("lnv-001", "theo-van-ban", "Theo văn bản") }

// --- the statement the store builds -------------------------------------------------------------

func TestLoaiNhiemVuCauLenhBuocXaVaLocDungDong(t *testing.T) {
	k := &khoGia{hang: mauLoai()}

	if _, err := dungKhoLoai(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	doiCauLenhCoXaVaLoc(t, k, xaThu, "loai_nhiem_vu", TranDanhMucLoaiNhiemVu)
}

func TestLoaiNhiemVuThamSoMotTheoContextChuKhongNhoXaCu(t *testing.T) {
	// THE SAME STORE TYPE, A DIFFERENT COMMUNE IN THE CONTEXT, A DIFFERENT $1. This is what "scoped
	// repository" means in practice, and a store that cached the first commune it ever saw — a
	// field set in the constructor, a package-level variable — would fail here and nowhere else.
	xaKhac := tenant.ID("01JB" + "BBBBBBBBBBBBBBBBBBBBBB")

	k1 := &khoGia{hang: mauLoai()}
	if _, err := dungKhoLoai(k1).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	k2 := &khoGia{hang: mauLoai()}
	if _, err := dungKhoLoai(k2).DanhSach(ctxXa(xaKhac)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}

	if k1.lenh[0].args[0] != string(xaThu) || k2.lenh[0].args[0] != string(xaKhac) {
		t.Errorf("$1 không đi theo context: %v rồi %v", k1.lenh[0].args[0], k2.lenh[0].args[0])
	}
}

// --- what comes back ------------------------------------------------------------------------------

func TestLoaiNhiemVuDocDungTungCot(t *testing.T) {
	// THE SCAN IS POSITIONAL AND THE FAKE BUILDS ITS ROW BY COLUMN NAME, so this test fails the
	// moment cotLoaiNhiemVu and the Scan below it stop agreeing. Two pairs cannot be caught by the
	// type system: `ma`/`nhan` are both TEXT and adjacent, `la_mac_dinh`/`dang_dung` are both
	// BOOLEAN and adjacent. Swapping either compiles, passes an ordinary test, and is wrong — slugs
	// where labels belong, or a form pre-selecting a row the commune has switched off.
	k := &khoGia{hang: mauLoai()}

	ra, err := dungKhoLoai(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.ID != "lnv-001" || mot.Ma != "theo-van-ban" || mot.Nhan != "Theo văn bản" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	// The fixture sets the two BOOLEANs to OPPOSITE values on purpose.
	if !mot.LaMacDinh || mot.DangDung {
		t.Errorf("la_mac_dinh / dang_dung đọc ngược: %+v", mot)
	}
}

func TestLoaiNhiemVuXaChuaCoDongNaoTraLatRong(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE: migration 0003 creates the table and seeds nothing, because
	// a catalogue row carries tenant_id and the step that sows a commune's first rows does not
	// exist yet. An empty catalogue is correct, not a failure — and it is a LIST, never a nil the
	// caller has to branch on before the handler can marshal `"items":[]`.
	k := &khoGia{}

	ra, err := dungKhoLoai(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("danh mục rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra) != 0 {
		t.Fatalf("nhận %d dòng từ một xã chưa có dòng nào", len(ra))
	}
}

// --- the ceiling ------------------------------------------------------------------------------

func TestLoaiNhiemVuDungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list, not a refusal. An off-by-one here refuses a
	// commune whose data is perfectly valid, and the refusal is a 500 on a real screen.
	k := &khoGia{hang: nhieuDong(TranDanhMucLoaiNhiemVu)}

	ra, err := dungKhoLoai(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranDanhMucLoaiNhiemVu {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranDanhMucLoaiNhiemVu)
	}
}

func TestLoaiNhiemVuVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// REFUSE, DO NOT TRUNCATE — and the rows already read are DROPPED. Handing back a list the
	// caller might render anyway is how a refusal turns back into a silent truncation, one careless
	// `if err != nil { log }` later.
	k := &khoGia{hang: nhieuDong(TranDanhMucLoaiNhiemVu + 1)}

	ra, err := dungKhoLoai(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, ErrQuaNhieuLoaiNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuLoaiNhiemVu", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- failures ---------------------------------------------------------------------------------

func TestLoaiNhiemVuLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Wrapped with %w, never swallowed. The handler tells the ceiling apart from an ordinary
	// failure with errors.Is, and that only works while the chain is intact — a `%v` here would
	// send the ceiling case down the generic branch and the log line would name the wrong cause.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGia{loi: goc}

	ra, err := dungKhoLoai(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuLoaiNhiemVu) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestLoaiNhiemVuKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would query either every commune's
	// rows or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover turns that
	// into a traceable 500 at the edge. What must never happen is a default commune (rule 1,
	// forbidden #1).
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc danh mục khi context không có xã mà không panic")
		}
	}()
	k := &khoGia{hang: mauLoai()}
	_, _ = dungKhoLoai(k).DanhSach(context.Background())
}
