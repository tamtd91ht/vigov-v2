package store

import (
	"context"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The task-priority read, against the fake driver in driver_gia_test.go — NO PostgreSQL.
//
// THE ONE PROPERTY THAT MATTERS MORE HERE THAN ANYWHERE ELSE IN THIS SERVICE: this list is a SCALE,
// so its ORDER is the meaning of the data rather than a presentation choice. The handler test
// pins the order of what the store RETURNS; until this file existed, nothing checked that the
// ORDER BY actually reaching PostgreSQL says what that test assumes. A store that sorted by `ma`
// alone, or dropped the ORDER BY entirely and relied on insertion order, passed every test in the
// repository.

func dungKhoUuTien(k *khoGia) *MucUuTienNhiemVuStore {
	return NewMucUuTienNhiemVuStore(pkgstore.New(moKhoGia(k)))
}

func mauUuTien() []hangGia { return mauMotDong("uu-001", "khan", "Khẩn") }

// --- the statement the store builds -------------------------------------------------------------

func TestMucUuTienCauLenhBuocXaVaXepTheoThang(t *testing.T) {
	// ORDER BY thu_tu, ma IS ASSERTED ON THE STATEMENT ITSELF, not on the rows: the fake returns
	// whatever it was handed, so the only way to know the database is being asked for rank order is
	// to read the statement. `thu_tu` is the commune's own ranking; `ma` makes the order total, so
	// two levels sharing a rank cannot swap places between two calls.
	//
	// The rest of doiCauLenhCoXaVaLoc matters as much here as on the type catalogue: a `dang_dung`
	// predicate creeping in would silently shorten the scale at whichever end holds the disabled
	// level, and a missing LIMIT+1 would turn "too many" into a quiet truncation of one end of it.
	k := &khoGia{hang: mauUuTien()}

	if _, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu)); err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	doiCauLenhCoXaVaLoc(t, k, xaThu, "muc_uu_tien_nhiem_vu", TranDanhMucMucUuTien)
}

func TestMucUuTienGiuNguyenThuTuKhoTraVe(t *testing.T) {
	// THE STORE MUST NOT RE-SORT WHAT THE DATABASE ORDERED. The fake hands back the rows in the
	// sequence the fixture lists them — deliberately neither alphabetical by code nor by label —
	// so a `sort.Slice` added anywhere in the read path turns this red. Position by position, never
	// as a set: a set comparison would agree with every possible ordering.
	k := &khoGia{hang: []hangGia{
		{id: "uu-001", ma: "khan", nhan: "Khẩn", dangDung: true},
		{id: "uu-002", ma: "cao", nhan: "Cao", dangDung: true},
		{id: "uu-003", ma: "thuong", nhan: "Thường", macDinh: true, dangDung: true},
	}}

	ra, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	muon := []string{"khan", "cao", "thuong"}
	if len(ra) != len(muon) {
		t.Fatalf("nhận %d mức, muốn %d", len(ra), len(muon))
	}
	for i, ma := range muon {
		if ra[i].Ma != ma {
			t.Fatalf("kho đổi thứ tự thang ở vị trí %d: %q, muốn %q", i, ra[i].Ma, ma)
		}
	}
}

// --- what comes back ------------------------------------------------------------------------------

func TestMucUuTienDocDungTungCot(t *testing.T) {
	// Same trap as on the type catalogue, and the same fixture discipline: the fake builds its row
	// BY COLUMN NAME from cotMucUuTien, and the two BOOLEANs are opposite. `ma`/`nhan` and
	// `la_mac_dinh`/`dang_dung` are two pairs a swap cannot be caught in by the compiler.
	k := &khoGia{hang: mauUuTien()}

	ra, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("DanhSach lỗi: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.ID != "uu-001" || mot.Ma != "khan" || mot.Nhan != "Khẩn" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	if !mot.LaMacDinh || mot.DangDung {
		t.Errorf("la_mac_dinh / dang_dung đọc ngược: %+v", mot)
	}
}

func TestMucUuTienXaChuaCoDongNaoTraLatRong(t *testing.T) {
	// Every commune, today: migration 0003 seeds nothing. A list, never a nil.
	k := &khoGia{}

	ra, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("thang rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra) != 0 {
		t.Fatalf("nhận %d dòng từ một xã chưa có dòng nào", len(ra))
	}
}

// --- the ceiling ------------------------------------------------------------------------------

func TestMucUuTienDungTranThiVanTraDu(t *testing.T) {
	k := &khoGia{hang: nhieuDong(TranDanhMucMucUuTien)}

	ra, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranDanhMucMucUuTien {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranDanhMucMucUuTien)
	}
}

func TestMucUuTienVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// Worse here than on the type catalogue: the levels arrive in rank order, so a truncated scale
	// loses the levels at ONE END of it. Whichever end that is, the picker then offers a scale that
	// silently stops short and every task filed from it is ranked wrong.
	k := &khoGia{hang: nhieuDong(TranDanhMucMucUuTien + 1)}

	ra, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, ErrQuaNhieuMucUuTien) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuMucUuTien", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

// --- failures ---------------------------------------------------------------------------------

func TestMucUuTienLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGia{loi: goc}

	ra, err := dungKhoUuTien(k).DanhSach(ctxXa(xaThu))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuMucUuTien) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestMucUuTienKhongCoXaTrongContextThiPanic(t *testing.T) {
	// Fail closed, loudly — see the twin of this test on the type catalogue.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc thang ưu tiên khi context không có xã mà không panic")
		}
	}()
	k := &khoGia{hang: mauUuTien()}
	_, _ = dungKhoUuTien(k).DanhSach(context.Background())
}
