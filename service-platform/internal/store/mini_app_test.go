package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Statement-level checks that run WITHOUT a database. The pg tests beside this file prove the
// behaviour; these pin the three clauses whose loss would turn nothing red on a one-row fixture.

func TestTruyVanMiniAppLoaiDongTatVaDongXoaMem(t *testing.T) {
	for _, menhDe := range []string{"m.dang_hoat_dong", "m.deleted_at IS NULL", "m.app_id = $1"} {
		if !strings.Contains(truyVanMiniApp, menhDe) {
			t.Errorf("truy vấn mini app thiếu %q — app tắt hoặc đã xoá mềm sẽ vẫn cấp xã:\n%s",
				menhDe, truyVanMiniApp)
		}
	}
	// Never follows a successor on its own (ADR 0045 §Chế độ).
	if strings.Contains(truyVanMiniApp, "tenant_succession") {
		t.Error("truy vấn mini app đi theo tenant_succession — app riêng không được tự chuyển xã")
	}
}

func TestHoSoHienThiLoaiDongXoaMemVaQuetDungThuTu(t *testing.T) {
	if !strings.Contains(duoiHoSoHienThi, "deleted_at IS NULL") {
		t.Errorf("đọc hồ sơ hiển thị không loại dòng xoá mềm: %q", duoiHoSoHienThi)
	}
	// The column list is positional — see Doc.
	const muon = `dia_chi_tru_so, COALESCE(logo_url, ''), duong_day_nong, gio_lam_viec_hien_thi, gioi_thieu`
	if cotHoSoHienThi != muon {
		t.Fatalf("thứ tự cột = %q, muốn %q — Doc quét theo VỊ TRÍ", cotHoSoHienThi, muon)
	}
}

func TestMiniAppAppIDRongBiTuChoiTruocKhiChamCSDL(t *testing.T) {
	// A nil *sql.DB: reaching the database would panic, so a pass proves the refusal came first.
	d := &Directory{}
	_, err := d.MiniApp(context.Background(), "")
	if !errors.Is(err, domain.ErrAppIDTrong) {
		t.Fatalf("err = %v, muốn ErrAppIDTrong", err)
	}
}
