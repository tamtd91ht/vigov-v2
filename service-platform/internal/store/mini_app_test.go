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
	// Soft-deleted profile rows AND soft-deleted / unfinished / wrong-purpose image files are excluded;
	// every joined table is pinned to the commune ($1) — QueryJoin's contract.
	for _, clause := range []string{
		"WHERE h.tenant_id = $1 AND h.deleted_at IS NULL",
		"l.tenant_id = $1 AND l.id = h.logo_file_id",
		"l.purpose = 'tenant-logo' AND l.status = 'ready' AND l.deleted_at IS NULL",
		"b.tenant_id = $1 AND b.id = h.web_admin_banner_file_id",
		"b.purpose = 'tenant-banner' AND b.status = 'ready' AND b.deleted_at IS NULL",
	} {
		if !strings.Contains(profileReadStmt, clause) {
			t.Errorf("profile read lacks %q:\n%s", clause, profileReadStmt)
		}
	}
	// The column list is positional — see Doc. logo_url stays its own column and is never the source
	// of the logo key (ADR 0069: no fallback to the typed URL).
	const cols = `SELECT h.dia_chi_tru_so, COALESCE(h.logo_url, ''), h.duong_day_nong,
	h.gio_lam_viec_hien_thi, h.gioi_thieu,
	COALESCE(l.public_object_key, ''), COALESCE(b.public_object_key, '')`
	if !strings.HasPrefix(profileReadStmt, cols) {
		t.Fatalf("column order changed — Doc scans by POSITION:\n%s", profileReadStmt)
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
