package main

import (
	"strings"
	"testing"
)

// `@multipart` describes an upload read with httpx.ReadUpload: an inline multipart/form-data body,
// parts in declared (= wire) order, `file` binary, `?` parts optional.
func TestMultipartEmittedAsFormDataBody(t *testing.T) {
	goc := khoThu(t, `package http

import (
	"net/http"

	"vd.test/core/authz"
)

type out struct {
	ID string `+"`json:\"id\"`"+`
}

func Register(mux *http.ServeMux) {
	// @summary   Tải ảnh
	// @multipart size purpose caption? file
	// @reply     201 out
	mux.Handle("POST /api/v1/things/{id}/photos", authz.RequirePermission(nil, "thing.edit")(http.HandlerFunc(nil)))
}
`)
	tuyens, err := quetTuyen(goc)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := moGiaiMa(goc)
	if err != nil {
		t.Fatal(err)
	}
	doc, surface, err := dungTaiLieu(tuyens, gm)
	if err != nil {
		t.Fatal(err)
	}
	b, err := machJSON(doc, "")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	want := `"requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","description":`
	if !strings.Contains(s, want) {
		t.Fatalf("no multipart requestBody:\n%s", s)
	}
	wantProps := `"properties":{"size":{"type":"string"},"purpose":{"type":"string"},"caption":{"type":"string"},` +
		`"file":{"type":"string","format":"binary"}},"required":["size","purpose","file"],"additionalProperties":false}`
	if !strings.Contains(s, wantProps) {
		t.Errorf("properties not in wire order / required wrong:\n%s", s)
	}
	if len(surface.Routes) != 1 || surface.Routes[0].RequestType != "multipart/form-data" || surface.Routes[0].Request != "" {
		t.Errorf("surface row = %+v", surface.Routes)
	}
}

func TestParseMultipartRules(t *testing.T) {
	ok := []string{"size", "purpose", "caption?", "file"}
	got, err := parseMultipart(ok)
	if err != nil {
		t.Fatalf("valid declaration refused: %v", err)
	}
	if len(got) != 4 || !got[2].Optional || !got[3].Binary || got[0].Optional || got[1].Binary {
		t.Errorf("parsed = %+v", got)
	}
	for name, toks := range map[string][]string{
		"empty":          nil,
		"no file":        {"size", "purpose"},
		"no size":        {"purpose", "file"},
		"file not last":  {"size", "file", "purpose"},
		"file optional":  {"size", "file?"},
		"size optional":  {"size?", "file"},
		"duplicate":      {"size", "purpose", "purpose", "file"},
		"bad name":       {"size", "Purpose", "file"},
		"dash in name":   {"size", "image-kind", "file"},
		"bare question":  {"size", "?", "file"},
		"two file parts": {"size", "file", "file"},
	} {
		if _, err := parseMultipart(toks); err == nil {
			t.Errorf("%s: %v accepted", name, toks)
		}
	}
}

func TestMultipartAnnotationConflicts(t *testing.T) {
	for name, block := range map[string]string{
		"with @request": "@summary x\n@request thing\n@multipart size file\n@reply 201 -",
		"twice":         "@summary x\n@multipart size file\n@multipart size file\n@reply 201 -",
	} {
		var tt tuyen
		if err := phanTichChuThich(block, &tt); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	var tt tuyen
	if err := phanTichChuThich("@summary x\n@multipart size file\n@reply 201 -", &tt); err != nil || len(tt.Multipart) != 2 {
		t.Errorf("plain @multipart: err=%v parts=%+v", err, tt.Multipart)
	}
}
