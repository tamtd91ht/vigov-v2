package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
		File:    "identity/internal/http/routes.go",
	}
}

var (
	tuyenPhien = tuyenGia("POST", "/api/v1/sessions")
	maPhien    = maViec("identity", "POST", "/api/v1/sessions")
)

// dongBo runs one synchronisation and fails the test on error.
func dongBo(t *testing.T, goc string, ts []tuyen) ketQuaViec {
	t.Helper()
	kq, err := dongBoViec(goc, ts)
	if err != nil {
		t.Fatal(err)
	}
	return kq
}

// hangDoi sets up a queue holding exactly one task for tuyenPhien, then moves it where the
// test needs it.
func hangDoi(t *testing.T, dat string) string {
	t.Helper()
	goc := filepath.Join(t.TempDir(), "web")
	dongBo(t, goc, []tuyen{tuyenPhien})
	if dat != "open" {
		if err := os.Rename(
			filepath.Join(goc, "open", maPhien+".json"),
			filepath.Join(goc, dat, maPhien+".json")); err != nil {
			t.Fatal(err)
		}
	}
	return goc
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
	ts := []tuyen{tuyenPhien}

	if kq := dongBo(t, goc, ts); kq.Moi != 1 {
		t.Fatalf("lần đầu phải sinh 1 việc, được %d", kq.Moi)
	}
	if kq := dongBo(t, goc, ts); kq.Moi != 0 || kq.Stale != 0 || kq.HoiSinh != 0 {
		t.Fatalf("chạy lại không được đổi gì: %+v", kq)
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
		goc := hangDoi(t, thuMuc)
		kq := dongBo(t, goc, []tuyen{tuyenPhien})
		if kq.Moi != 0 {
			t.Fatalf("việc đang ở %s/ lại được sinh lại vào open/", thuMuc)
		}
		if demTep(t, filepath.Join(goc, "open")) != 0 {
			t.Fatalf("open/ không được rỗng khi việc đang ở %s/", thuMuc)
		}
	}
}

func TestRouteMoiThiSinhViecMoi(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ts := []tuyen{tuyenPhien}
	dongBo(t, goc, ts)

	ts = append(ts, tuyenGia("GET", "/api/v1/staff"))
	if kq := dongBo(t, goc, ts); kq.Moi != 1 {
		t.Fatalf("route mới phải sinh đúng 1 việc, được %d", kq.Moi)
	}
	if demTep(t, filepath.Join(goc, "open")) != 2 {
		t.Fatal("open/ phải có 2 việc")
	}
}

// --- mồ côi: bốn chỗ, bốn cách xử ---------------------------------------------------------

// Nobody had started. MOVED to stale/, not deleted: "why did this vanish" is a question
// somebody will ask, and a moved file answers it while a deleted one cannot.
func TestMoCoiOOpenChuyenSangStale(t *testing.T) {
	goc := hangDoi(t, "open")

	kq := dongBo(t, goc, []tuyen{tuyenGia("GET", "/api/v1/staff")})
	if kq.Stale != 1 {
		t.Fatalf("mong 1 việc chuyển sang stale, được %d", kq.Stale)
	}
	if _, err := os.Stat(filepath.Join(goc, "open", maPhien+".json")); err == nil {
		t.Error("việc mồ côi vẫn nằm ở open/")
	}
	if _, err := os.Stat(filepath.Join(goc, "stale", maPhien+".json")); err != nil {
		t.Errorf("việc mồ côi không có trong stale/: %v", err)
	}
}

// THE CASE THAT MATTERS MOST. Somebody is building a screen for a route that no longer exists.
// Taking the file out of their hands destroys their context at the moment they need it, and
// claimed/ belongs to admin-web-builder, not to this generator. So: report, do not touch.
func TestMoCoiOClaimedKhongBiDungVaCoBao(t *testing.T) {
	goc := hangDoi(t, "claimed")

	kq := dongBo(t, goc, []tuyen{tuyenGia("GET", "/api/v1/staff")})
	if len(kq.MoCoi) != 1 {
		t.Fatalf("mong đúng 1 mồ côi đã nhận, được %d", len(kq.MoCoi))
	}
	m := kq.MoCoi[0]
	if m.ID != maPhien || m.Method != "POST" || m.Path != "/api/v1/sessions" {
		t.Errorf("báo cáo phải nêu mã việc VÀ đường dẫn cũ, được %+v", m)
	}
	if _, err := os.Stat(filepath.Join(goc, "claimed", maPhien+".json")); err != nil {
		t.Errorf("tệp trong claimed/ bị đụng vào: %v", err)
	}
	if demTep(t, filepath.Join(goc, "stale")) != 0 {
		t.Error("việc đã nhận không được chuyển sang stale/")
	}
	if kq.Stale != 0 {
		t.Errorf("không được đếm việc đã nhận là stale: %d", kq.Stale)
	}
}

// Correct history, not litter: a screen was built, the route later changed. Reporting it would
// be a warning that can never be cleared, and those are the warnings people learn to scroll
// past — taking the loud claimed/ warning down with them.
func TestMoCoiODoneKhongBiDungVaKhongBao(t *testing.T) {
	goc := hangDoi(t, "done")

	kq := dongBo(t, goc, []tuyen{tuyenGia("GET", "/api/v1/staff")})
	if len(kq.MoCoi) != 0 {
		t.Errorf("việc đã xong không được báo là mồ côi: %+v", kq.MoCoi)
	}
	if kq.Stale != 0 {
		t.Errorf("việc đã xong không được chuyển sang stale: %d", kq.Stale)
	}
	if _, err := os.Stat(filepath.Join(goc, "done", maPhien+".json")); err != nil {
		t.Errorf("tệp trong done/ bị đụng vào: %v", err)
	}
}

// --- hồi sinh -----------------------------------------------------------------------------

// The SAME file goes back, not a new one, so any note somebody left on it survives.
func TestHoiSinhTuStaleVeOpen(t *testing.T) {
	goc := hangDoi(t, "stale")
	dau := filepath.Join(goc, "stale", maPhien+".json")
	if err := os.WriteFile(dau, []byte(`{"id":"`+maPhien+`","dau_rieng":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	kq := dongBo(t, goc, []tuyen{tuyenPhien})
	if kq.HoiSinh != 1 || kq.Moi != 0 {
		t.Fatalf("mong hồi sinh 1 và sinh mới 0, được %+v", kq)
	}
	b, err := os.ReadFile(filepath.Join(goc, "open", maPhien+".json"))
	if err != nil {
		t.Fatalf("việc không quay về open/: %v", err)
	}
	if !strings.Contains(string(b), "dau_rieng") {
		t.Error("đã sinh tệp mới thay vì chuyển chính tệp cũ — ghi chú của người khác bị mất")
	}
	if demTep(t, filepath.Join(goc, "stale")) != 0 {
		t.Error("stale/ phải rỗng sau khi hồi sinh")
	}
}

func TestDaXongThiKhongHoiSinh(t *testing.T) {
	goc := hangDoi(t, "done")
	kq := dongBo(t, goc, []tuyen{tuyenPhien})
	if kq.HoiSinh != 0 || kq.Moi != 0 {
		t.Fatalf("việc đã ở done/ không được hồi sinh hay sinh lại: %+v", kq)
	}
	if demTep(t, filepath.Join(goc, "open")) != 0 {
		t.Error("open/ phải rỗng")
	}
}

// --- khóa và lưới an toàn -----------------------------------------------------------------

// THE LOCK IS os.Rename AND NOTHING ELSE.
//
// Two agents claiming the same task is not a rare case — it is the normal case when work is
// handed out in parallel. The operating system already decides it atomically; a lock file or a
// status field would be a second source for that fact, and the copy that loses would let two
// agents build the same screen twice while another route goes untouched.
func TestHaiLanNhanViecChiMotLanThanhCong(t *testing.T) {
	goc := hangDoi(t, "open")
	cu := filepath.Join(goc, "open", maPhien+".json")
	moi := filepath.Join(goc, "claimed", maPhien+".json")

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

// The generator losing that same race is the HARMLESS side: it means the task was claimed a
// moment ago. It must carry on without a word, and the next run reports it from claimed/.
func TestChuyenGapENOENTThiBoQuaEm(t *testing.T) {
	goc := hangDoi(t, "open")
	ok, err := chuyen(goc, "open", "stale", "khongtontai")
	if err != nil {
		t.Fatalf("ENOENT không được là lỗi: %v", err)
	}
	if ok {
		t.Fatal("chuyển một tệp không tồn tại lại báo thành công")
	}
}

// SAFETY NET 2. No routes means the generator is broken, not that the repository is empty. A
// run that swept the whole board into stale/ and the next run swept it back would be noise —
// and worse, it would end with nobody trusting the queue.
func TestKhongCoRouteThiKhongChuyenGi(t *testing.T) {
	goc := hangDoi(t, "open")

	kq, err := dongBoViec(goc, nil)
	if err == nil {
		t.Fatal("quét 0 route phải là lỗi, không phải một lượt dọn sạch hàng đợi")
	}
	if kq.Stale != 0 {
		t.Errorf("không được chuyển gì sang stale: %d", kq.Stale)
	}
	if _, err := os.Stat(filepath.Join(goc, "open", maPhien+".json")); err != nil {
		t.Errorf("việc trong open/ bị đụng vào: %v", err)
	}
}

func TestStaleCoGitkeep(t *testing.T) {
	goc := hangDoi(t, "open")
	for _, d := range []string{"open", "claimed", "done", "stale"} {
		if _, err := os.Stat(filepath.Join(goc, d, ".gitkeep")); err != nil {
			t.Errorf("%s/ thiếu .gitkeep — git không theo dõi thư mục rỗng: %v", d, err)
		}
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
