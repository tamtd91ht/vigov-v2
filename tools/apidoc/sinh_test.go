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
