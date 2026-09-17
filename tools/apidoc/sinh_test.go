package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Determinism, measured the only way that counts: generate twice from the same source and
// compare the bytes.
//
// A generated file carrying a timestamp makes every run a diff, and a diff nobody reads is a
// diff that hides the one real change — a route that quietly lost its permission, a reply
// shape that quietly grew a field. This is why there is no generated_at, no commit hash and no
// line number anywhere in the output.
func TestDauRaTatDinh(t *testing.T) {
	goc, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}

	chayMot := func() (openapi, surface []byte) {
		d := t.TempDir()
		c := cauHinh{
			Root:     goc,
			OpenAPI:  filepath.Join(d, "openapi.json"),
			Surface:  filepath.Join(d, "api-surface.json"),
			TasksDir: filepath.Join(d, "tasks", "web"),
		}
		if _, _, err := chay(c); err != nil {
			t.Fatal(err)
		}
		a, err := os.ReadFile(c.OpenAPI)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(c.Surface)
		if err != nil {
			t.Fatal(err)
		}
		return a, b
	}

	o1, s1 := chayMot()
	o2, s2 := chayMot()

	if !bytes.Equal(o1, o2) {
		t.Errorf("openapi.json khác nhau giữa hai lần chạy (%d vs %d byte)", len(o1), len(o2))
	}
	if !bytes.Equal(s1, s2) {
		t.Errorf("api-surface.json khác nhau giữa hai lần chạy (%d vs %d byte)", len(s1), len(s2))
	}
}

// SAFETY NET 1 — a run that cannot read the source must change NOTHING.
//
// This is the failure that would hurt most, because it is silent and plausible: one file fails
// to parse mid-edit, the scan comes back with a smaller set of routes, and every task for the
// routes it could not see gets swept into stale/. The next run sweeps them back. Nobody trusts
// the queue after that. quetTuyen therefore joins the errors from ALL files and chay returns
// before the first write.
func TestLoiPhanTichThiKhongDoiGiHet(t *testing.T) {
	goc := t.TempDir()
	viet := func(rel, noiDung string) {
		p := filepath.Join(goc, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(noiDung), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Mỗi đơn vị triển khai là một module riêng, và gốc kho KHÔNG có go.mod.
	// Fixture phải dựng đúng hình dạng ấy, nếu không nó kiểm một bố cục không tồn tại.
	viet("thu/go.mod", "module vd.test/thu\n\ngo 1.26.0\n")
	viet("core/go.mod", "module vd.test/core\n\ngo 1.26.0\n")
	viet("thu/cmd/server/main.go", "package main\n")
	viet("thu/internal/http/routes.go", `package http

import (
	"net/http"

	"vd.test/core/authz"
)

func Register(mux *http.ServeMux) {
	// @summary  Lành lặn
	// @reply    200 -
	mux.Handle("GET /api/v1/ok", authz.Public("lý do")(http.HandlerFunc(nil)))

	// @summary  Hỏng
	// @replyy   200 -
	mux.Handle("GET /api/v1/hong", authz.Public("lý do")(http.HandlerFunc(nil)))
}
`)

	// A task nobody has started, for a route that is NOT in the file above. A run that ignored
	// the parse error would see it as an orphan and move it.
	viecCu := filepath.Join(goc, "tasks", "web", "open", "deadbeef1234.json")
	if err := os.MkdirAll(filepath.Dir(viecCu), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(viecCu, []byte(`{"id":"deadbeef1234"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	c := cauHinh{
		Root:     goc,
		OpenAPI:  filepath.Join(goc, "out", "openapi.json"),
		Surface:  filepath.Join(goc, "out", "api-surface.json"),
		TasksDir: filepath.Join(goc, "tasks", "web"),
	}
	if _, _, err := chay(c); err == nil {
		t.Fatal("một tệp không phân tích được phải làm cả lượt chạy dừng")
	}
	if _, err := os.Stat(c.OpenAPI); err == nil {
		t.Error("lượt chạy hỏng vẫn ghi ra openapi.json")
	}
	if _, err := os.Stat(viecCu); err != nil {
		t.Errorf("lượt chạy hỏng đã đụng vào hàng đợi: %v", err)
	}
	if _, err := os.Stat(filepath.Join(goc, "tasks", "web", "stale")); err == nil {
		t.Error("lượt chạy hỏng đã tạo/dùng stale/")
	}
}

// The warning has to be the FIRST thing in the file. A generated file whose first line is not
// its warning is a generated file somebody edits by hand, and the edit is lost silently on the
// next `make kb` (rule 9, invariant 8).
func TestCanhBaoNamODauTep(t *testing.T) {
	goc, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}
	d := t.TempDir()
	c := cauHinh{
		Root:     goc,
		OpenAPI:  filepath.Join(d, "openapi.json"),
		Surface:  filepath.Join(d, "api-surface.json"),
		TasksDir: filepath.Join(d, "tasks", "web"),
	}
	if _, _, err := chay(c); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{c.OpenAPI, c.Surface} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		dong := bytes.SplitN(b, []byte("\n"), 3)
		if len(dong) < 2 || !bytes.Contains(dong[1], []byte("DO NOT EDIT")) {
			t.Errorf("%s: dòng đầu tiên không phải cảnh báo không sửa tay", filepath.Base(p))
		}
	}
}

// The repository's own two routes, end to end. Not a golden file: a golden file would have to
// be updated by hand on every legitimate change, and the thing being checked here is that what
// the code REALLY declares is what comes out.
func TestHaiRouteThatSinhDungHinhDang(t *testing.T) {
	goc, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}
	tuyens, err := quetTuyen(goc)
	if err != nil {
		t.Fatal(err)
	}
	theoKhoa := map[string]tuyen{}
	for _, x := range tuyens {
		theoKhoa[x.Method+" "+x.Path] = x
	}

	dn, ok := theoKhoa["POST /api/v1/sessions"]
	if !ok {
		t.Fatal("không trích được POST /api/v1/sessions")
	}
	if dn.Quyen.Kind != "public" || dn.Quyen.LyDo == "" {
		t.Errorf("route đăng nhập phải là Public có lý do: %+v", dn.Quyen)
	}
	if dn.Request != "thanDangNhap" {
		t.Errorf("@request sai: %q", dn.Request)
	}

	dx, ok := theoKhoa["DELETE /api/v1/sessions/{sid}"]
	if !ok {
		t.Fatal("không trích được DELETE /api/v1/sessions/{sid}")
	}
	if dx.Request != "" {
		t.Errorf("route 204 không được có @request: %q", dx.Request)
	}
	// 404 and not 403 for somebody else's sid is rule 4, forbidden #2 — it is part of the
	// contract, not an implementation detail, so the web must see it.
	co404 := false
	for _, r := range dx.Replies {
		if r.Status == 404 {
			co404 = true
		}
		if r.Status == 403 {
			t.Error("đăng xuất không được trả 403 — lộ sự tồn tại phiên của người khác")
		}
	}
	if !co404 {
		t.Error("thiếu @reply 404 trên route đăng xuất")
	}
}
