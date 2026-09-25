package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// THE ARCHIVAL PROMISE, SWEPT RATHER THAN SPOT-CHECKED: once minutes are signed, no write act of this
// register changes their content, their conclusions, their chair or secretary, or removes them.
//
// bien_ban_hop_sua_test.go proves each act's refusal one at a time. What it cannot prove is that the
// list of acts is COMPLETE, nor that every FIELD of the edit request is part of the "did the content
// change" comparison. Both fail silently:
//
//	a new exported act on GhiBienBanHop that forgets the signed check   the unit suite has no test
//	                                                                      for an act it does not know
//	a field of YeuCauSuaBienBan missing from khacNgoaiThongBao          PATCH on signed minutes with
//	                                                                      only that field answers 200,
//	                                                                      writes nothing, and the clerk
//	                                                                      believes the correction landed
//	                                                                      (measured: dropping ThuKyMa or
//	                                                                      ChuTriMa from the comparison
//	                                                                      left the whole suite green)
//
// So both lists are ENUMERATED BY REFLECTION: a new method or a new request field fails this file until
// somebody states what it does to signed minutes.
//
// NOT PROVED HERE: migration 0012's row trigger (the floor under all of this) — pg suites, skipped
// without VIGOV_TEST_DSN.

// hanhViSauKy says what one act does to SIGNED minutes, and runs it against them.
type hanhViSauKy struct {
	// choPhep is the reason the act is still allowed after signing; empty means it must refuse
	// with ErrBienBanDaKy and write nothing.
	choPhep string
	chay    func(uc *GhiBienBanHop, nv *taoNhiemVuGia) error
}

func cacHanhViSauKy() map[string]hanhViSauKy {
	ctx := ctxXa(xaThu)
	stt := int(thuTuKLGoc)
	return map[string]hanhViSauKy{
		"SuaBienBan": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			// The per-field sweep is TestBienBanDaKy_MoiTruongSuaDeuBiTuChoi; this row only
			// classifies the act.
			_, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{NoiDung: chuoi("Sửa sau khi ký.")}, canBoThu())
			return err
		}},
		"XoaBienBan": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			return uc.XoaBienBan(ctx, idBBGoc, "Nhập trùng.", canBoThu())
		}},
		"KyBienBan": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			_, err := uc.KyBienBan(ctx, idBBGoc, YeuCauKyBienBan{}, canBoThu())
			return err
		}},
		"ThemKetLuan": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			_, err := uc.ThemKetLuan(ctx, idBBGoc, YeuCauThemKetLuan{NoiDung: "Kết luận thêm sau khi ký."}, canBoThu())
			return err
		}},
		"SuaKetLuan": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			_, err := uc.SuaKetLuan(ctx, idBBGoc, stt, YeuCauSuaKetLuan{NoiDung: "Câu khác sau khi ký."}, canBoThu())
			return err
		}},
		"XoaKetLuan": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			return uc.XoaKetLuan(ctx, idBBGoc, stt, "Ghi nhầm.", canBoThu())
		}},
		"DanhDauKhongPhatSinh": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			_, err := uc.DanhDauKhongPhatSinh(ctx, idBBGoc, stt, canBoThu())
			return err
		}},
		"BoDanhDauKhongPhatSinh": {chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
			// The mark must be SET for this to be a change at all; the fixture sets it below.
			return uc.BoDanhDauKhongPhatSinh(ctx, idBBGoc, stt, canBoThu())
		}},
		"TachKetLuanThanhNhiemVu": {
			choPhep: "signing freezes the record, not the assignment of the work it concluded",
			chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
				_, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, stt, tachMau(), canBoThu())
				return err
			}},
		"TaoBienBan": {
			choPhep: "a correction to signed minutes IS supplementary minutes — a new draft row",
			chay: func(uc *GhiBienBanHop, _ *taoNhiemVuGia) error {
				yc := taoBienBanMau()
				yc.BoSungChoID = idBBGoc
				_, err := uc.TaoBienBan(ctx, yc, canBoThu())
				return err
			}},
	}
}

// khongChamBanDaKy asserts no statement wrote to the signed meeting or its conclusions.
func khongChamBanDaKy(t *testing.T, k *khoBienBanGia) {
	t.Helper()
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "UPDATE bien_ban_hop") || strings.HasPrefix(l.sql, "UPDATE ket_luan_hop") {
			t.Errorf("đã sửa biên bản ĐÃ KÝ hoặc kết luận của nó: %q", l.sql)
		}
		if strings.HasPrefix(l.sql, "INSERT INTO ket_luan_hop") {
			for _, a := range l.args {
				if a == idBBGoc {
					t.Errorf("đã thêm kết luận vào biên bản ĐÃ KÝ: %q", l.sql)
				}
			}
		}
	}
}

func TestBienBanDaKy_MoiHanhViGhiDeuDaDuocPhanLoai(t *testing.T) {
	bang := cacHanhViSauKy()
	kieu := reflect.TypeOf(&GhiBienBanHop{})
	thay := map[string]bool{}
	for i := 0; i < kieu.NumMethod(); i++ {
		ten := kieu.Method(i).Name
		thay[ten] = true
		if _, co := bang[ten]; !co {
			t.Errorf("GhiBienBanHop.%s là hành vi ghi MỚI chưa được phân loại với biên bản đã ký — "+
				"thêm vào cacHanhViSauKy: hoặc từ chối ErrBienBanDaKy, hoặc nêu lý do được phép", ten)
		}
	}
	for ten := range bang {
		if !thay[ten] {
			t.Errorf("cacHanhViSauKy có %q nhưng GhiBienBanHop không còn phương thức ấy", ten)
		}
	}
}

func TestBienBanDaKy_KhongHanhViNaoDoiDuocBanGhi(t *testing.T) {
	for ten, hv := range cacHanhViSauKy() {
		t.Run(ten, func(t *testing.T) {
			k := khoBBMau()
			kyFixture(k)
			if ten == "BoDanhDauKhongPhatSinh" {
				k.ketLuan[idKLGoc]["khong_phat_sinh"] = true
			}
			uc, nv, _ := dungGhiBienBan(t, k)

			err := hv.chay(uc, nv)
			if hv.choPhep == "" {
				if !errors.Is(err, domain.ErrBienBanDaKy) {
					t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
				}
				khongGhiCauNaoBB(t, k)
				return
			}
			if err != nil {
				t.Fatalf("hành vi được phép sau khi ký (%s) bị từ chối: %v", hv.choPhep, err)
			}
			khongChamBanDaKy(t, k)
		})
	}
}

// giaTriKhac returns a valid value of the field's type that differs from every fixture value.
func giaTriKhac(t *testing.T, f reflect.StructField) reflect.Value {
	t.Helper()
	switch f.Type {
	case reflect.TypeOf((*string)(nil)):
		return reflect.ValueOf(chuoi("CB-00099"))
	case reflect.TypeOf((*time.Time)(nil)):
		d := mocNgayHopBB.AddDate(0, 0, 1)
		return reflect.ValueOf(&d)
	case reflect.TypeOf((*[]string)(nil)):
		ds := []string{"CB-00099"}
		return reflect.ValueOf(&ds)
	}
	t.Fatalf("YeuCauSuaBienBan.%s có kiểu %s mà bài quét chưa biết — thêm một giá trị khác cho kiểu ấy",
		f.Name, f.Type)
	return reflect.Value{}
}

// TestBienBanDaKy_MoiTruongSuaDeuBiTuChoi — every field of the edit request but the notice, alone
// and together with a first notice, is refused on signed minutes and writes nothing. The second shape
// is the one a form sends: the notice filled in, plus one field somebody also "tidied".
func TestBienBanDaKy_MoiTruongSuaDeuBiTuChoi(t *testing.T) {
	kieu := reflect.TypeOf(YeuCauSuaBienBan{})
	ngayTB := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	for i := 0; i < kieu.NumField(); i++ {
		f := kieu.Field(i)
		if f.Name == "ThongBao" {
			continue // the one thing signed minutes still accept, once — bien_ban_hop_sua_test.go
		}
		for _, kemTB := range []bool{false, true} {
			ten := f.Name
			if kemTB {
				ten += "+ThongBao"
			}
			t.Run(ten, func(t *testing.T) {
				k := khoBBMau()
				kyFixture(k)
				uc, _, ctx := dungGhiBienBan(t, k)

				var yc YeuCauSuaBienBan
				reflect.ValueOf(&yc).Elem().Field(i).Set(giaTriKhac(t, f))
				if kemTB {
					yc.ThongBao = &ThongBaoKetLuan{SoKyHieu: "12/TB-UBND", Ngay: ngayTB}
				}
				_, err := uc.SuaBienBan(ctx, idBBGoc, yc, canBoThu())
				if !errors.Is(err, domain.ErrBienBanDaKy) {
					t.Fatalf("sửa %s trên biên bản đã ký: lỗi = %v, muốn ErrBienBanDaKy", f.Name, err)
				}
				khongGhiCauNaoBB(t, k)
			})
		}
	}
}
