package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestChuanHoaTapQuyenGopTrungVaSapXep(t *testing.T) {
	ra, err := ChuanHoaTapQuyen([]string{"task.read", "admin.role", "task.read"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ra, ",") != "admin.role,task.read" {
		t.Errorf("= %v", ra)
	}
	rong, err := ChuanHoaTapQuyen(nil)
	if err != nil || rong == nil || len(rong) != 0 {
		t.Errorf("tập rỗng phải hợp lệ và không nil: %v %v", rong, err)
	}
}

func TestChuanHoaTapQuyenTuChoiSaiDang(t *testing.T) {
	for _, k := range []string{"", "khongdaucham", ".dau", strings.Repeat("a", 101) + ".b"} {
		if _, err := ChuanHoaTapQuyen([]string{k}); !errors.Is(err, ErrKhoaQuyenSaiDangThuc) {
			t.Errorf("%q: err = %v", k, err)
		}
	}
	nhieu := make([]string, TranSoKhoaQuyenGui+1)
	for i := range nhieu {
		nhieu[i] = "a.b"
	}
	if _, err := ChuanHoaTapQuyen(nhieu); !errors.Is(err, ErrQuaNhieuKhoaQuyen) {
		t.Errorf("err = %v", err)
	}
}

func TestHieuTapQuyen(t *testing.T) {
	them, bo := HieuTapQuyen([]string{"a.x", "b.y"}, []string{"b.y", "c.z"})
	if strings.Join(them, ",") != "c.z" || strings.Join(bo, ",") != "a.x" {
		t.Errorf("thêm %v bỏ %v", them, bo)
	}
}

func TestConNguoiGiuSauKhiLuu(t *testing.T) {
	const k, dich = "admin.user", "vt-dich"
	ngoai := NguoiGiuQuyen{CanBoID: "nd-1", VaiTroID: "vt-khac"}
	trong := NguoiGiuQuyen{CanBoID: "nd-2", VaiTroID: dich}
	cases := []struct {
		ten     string
		dangGiu []NguoiGiuQuyen
		sau     []string
		muon    bool
	}{
		{"còn người ở vai trò khác, bỏ khoá", []NguoiGiuQuyen{ngoai, trong}, nil, true},
		{"chỉ còn người ở vai trò này, bỏ khoá", []NguoiGiuQuyen{trong}, nil, false},
		{"chỉ còn người ở vai trò này, giữ khoá", []NguoiGiuQuyen{trong}, []string{k}, true},
		{"xã đã không còn ai, giữ khoá", nil, []string{k}, false},
		{"xã đã không còn ai, bỏ khoá", nil, nil, false},
	}
	for _, c := range cases {
		if got := ConNguoiGiuSauKhiLuu(c.dangGiu, dich, c.sau, k); got != c.muon {
			t.Errorf("%s: = %v, muốn %v", c.ten, got, c.muon)
		}
	}
}
