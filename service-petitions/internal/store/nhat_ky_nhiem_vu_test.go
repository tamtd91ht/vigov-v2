package store

import (
	"database/sql/driver"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The task progress-log READ, over the fake driver: the ONE statement it builds and the row it scans.
// The PostgreSQL half (real ordering, real columns, the commune boundary) is
// nhat_ky_nhiem_vu_pg_test.go.

func TestNhatKyCuaNhiemVuBuocXaNhiemVuVaMoiNhatTruoc(t *testing.T) {
	xa := tenant.ID("01JA" + strings.Repeat("A", 22))
	luc := time.Date(2026, 9, 26, 3, 4, 5, 0, time.UTC)
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		{
			"id": "nknv-2", "nhiem_vu_id": "nv-001", "thoi_diem": luc, "nguoi_ma": "CB-00123",
			"trang_thai_tai_thoi_diem": "dang-thuc-hien",
			"bo_phan_id": "bp-001", "nguoi_phu_trach_ma": "CB-00777", "noi_dung": "Giao lại.",
		},
		{
			"id": "nknv-1", "nhiem_vu_id": "nv-001", "thoi_diem": luc, "nguoi_ma": "CB-00123",
			"trang_thai_tai_thoi_diem": "moi-giao",
			"bo_phan_id": nil, "nguoi_phu_trach_ma": nil, "noi_dung": "Tạo nhiệm vụ.",
		},
	}}
	s := NewNhiemVuStore(pkgstore.New(moKhoGia(k)))

	yc, err := page.Parse(url.Values{}, SapXepNhatKyNhiemVu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	kq, err := s.NhatKyCuaNhiemVu(ctxXa(xa), "nv-001", yc)
	if err != nil {
		t.Fatalf("NhatKyCuaNhiemVu: %v", err)
	}

	// ONE statement per page (skills/load-data-once).
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	if l.args[0] != string(xa) || l.args[1] != "nv-001" {
		t.Errorf("tham số = %v — $1 phải là xã từ ngữ cảnh, $2 là id nhiệm vụ", l.args)
	}
	for _, muon := range []string{"FROM nhat_ky_nhiem_vu", "tenant_id = $1", "nhiem_vu_id = $2",
		"ORDER BY thoi_diem DESC, id DESC", "LIMIT"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu lệnh thiếu %q: %s", muon, l.sql)
		}
	}

	if len(kq.Items) != 2 {
		t.Fatalf("đọc %d dòng, muốn 2", len(kq.Items))
	}
	gl, tao := kq.Items[0], kq.Items[1]
	if gl.ID != "nknv-2" || gl.NhiemVuID != "nv-001" || gl.BoPhanID != "bp-001" ||
		gl.NguoiPhuTrachMa != "CB-00777" || gl.TrangThaiTaiThoiDiem != domain.DangThucHien ||
		gl.NoiDung != "Giao lại." || gl.NguoiMa != "CB-00123" || !gl.ThoiDiem.Equal(luc) {
		t.Errorf("dòng giao lại quét sai: %+v", gl)
	}
	// NULL assignment columns scan to "" — never "<nil>", never an error.
	if tao.BoPhanID != "" || tao.NguoiPhuTrachMa != "" || tao.TrangThaiTaiThoiDiem != domain.MoiGiao {
		t.Errorf("dòng tạo quét sai: %+v", tao)
	}
}
