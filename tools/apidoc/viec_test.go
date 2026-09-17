package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func tuyenGia(method, path string) tuyen {
	return tuyen{
		Service: "identity",
		Method:  method,
		Path:    path,
		Summary: "việc thử",
		Quyen:   quyenDecl{Kind: "public", LyDo: "thử"},
		Idem:    idemDecl{Kind: "khong-can", LyDo: "thử"},
		Replies: []traLoi{{Status: 200}},
		File:    "services/identity/internal/http/routes.go",
	}
}

// The id has to survive everything except the route itself, or a finished task comes back on
// the board under a new name.
func TestMaViecTatDinh(t *testing.T) {
	a := maViec("identity", "POST", "/api/v1/sessions")
	b := maViec("identity", "POST", "/api/v1/sessions")
	if a != b {
		t.Fatalf("cùng route ra hai mã: %s vs %s", a, b)
	}
	khac := []string{
		maViec("identity", "DELETE", "/api/v1/sessions"),
		maViec("identity", "POST", "/api/v1/sessions/{sid}"),
		maViec("petitions", "POST", "/api/v1/sessions"),
	}
	for _, k := range khac {
		if k == a {
			t.Fatalf("hai route khác nhau dùng chung mã %s", k)
		}
	}
	if len(a) != 12 {
		t.Fatalf("mã phải ngắn và cố định, được %q", a)
	}
}

func TestSinhRoiSinhLaiKhongTaoTrung(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ts := []tuyen{tuyenGia("POST", "/api/v1/sessions")}

	moi, err := sinhViec(goc, ts)
	if err != nil || moi != 1 {
		t.Fatalf("lần đầu phải sinh 1 việc: %d %v", moi, err)
	}
	moi, err = sinhViec(goc, ts)
	if err != nil || moi != 0 {
		t.Fatalf("chạy lại không được sinh thêm: %d %v", moi, err)
	}
	if n := demTep(t, filepath.Join(goc, "open")); n != 1 {
		t.Fatalf("open/ có %d tệp việc, mong 1", n)
	}
}

// "Already done" must be visible from the filesystem alone. If the generator only looked at
// open/, every regeneration would resurrect finished work — and a queue that hands back
// completed tasks is a queue people stop reading.
func TestViecDaOClaimedHoacDoneThiKhongSinhLai(t *testing.T) {
	for _, thuMuc := range []string{"claimed", "done"} {
		goc := filepath.Join(t.TempDir(), "web")
		ts := []tuyen{tuyenGia("POST", "/api/v1/sessions")}
		if _, err := sinhViec(goc, ts); err != nil {
			t.Fatal(err)
		}
		id := maViec("identity", "POST", "/api/v1/sessions")
		cu := filepath.Join(goc, "open", id+".json")
		moi := filepath.Join(goc, thuMuc, id+".json")
		if err := os.Rename(cu, moi); err != nil {
			t.Fatal(err)
		}

		n, err := sinhViec(goc, ts)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("việc đang ở %s/ lại được sinh lại vào open/", thuMuc)
		}
		if demTep(t, filepath.Join(goc, "open")) != 0 {
			t.Fatalf("open/ không được rỗng sau khi việc chuyển sang %s/", thuMuc)
		}
	}
}

func TestRouteMoiThiSinhViecMoi(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ts := []tuyen{tuyenGia("POST", "/api/v1/sessions")}
	if _, err := sinhViec(goc, ts); err != nil {
		t.Fatal(err)
	}
	ts = append(ts, tuyenGia("GET", "/api/v1/staff"))
	n, err := sinhViec(goc, ts)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("route mới phải sinh đúng 1 việc, được %d", n)
	}
	if demTep(t, filepath.Join(goc, "open")) != 2 {
		t.Fatal("open/ phải có 2 việc")
	}
}

// THE LOCK IS os.Rename AND NOTHING ELSE.
//
// Two agents claiming the same task is not a rare case — it is the normal case when work is
// handed out in parallel. The operating system already decides it atomically; a lock file or a
// status field would be a second source for that fact, and the copy that loses would let two
// agents build the same screen twice while another route goes untouched.
func TestHaiLanNhanViecChiMotLanThanhCong(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ts := []tuyen{tuyenGia("POST", "/api/v1/sessions")}
	if _, err := sinhViec(goc, ts); err != nil {
		t.Fatal(err)
	}
	id := maViec("identity", "POST", "/api/v1/sessions")
	cu := filepath.Join(goc, "open", id+".json")
	moi := filepath.Join(goc, "claimed", id+".json")

	if err := os.Rename(cu, moi); err != nil {
		t.Fatalf("agent thứ nhất phải nhận được việc: %v", err)
	}
	err := os.Rename(cu, moi)
	if err == nil {
		t.Fatal("agent thứ hai cũng nhận được cùng một việc — khóa của hệ điều hành không giữ")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("agent thứ hai phải nhận ENOENT, được: %v", err)
	}
}

func demTep(t *testing.T, dir string) int {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			n++
		}
	}
	return n
}
