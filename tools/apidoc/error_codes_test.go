package main

import (
	"strings"
	"testing"
)

// The named codes reach the contract as a response-level `x-vigov-error-codes`, and NOTHING ELSE about
// the response moves: the schema stays the bare $ref, so web-admin's generator (which refuses allOf)
// and every existing response without codes stay byte-identical.
func TestErrorCodesEmittedOnResponse(t *testing.T) {
	goc := khoThu(t, `package http

import (
	"net/http"

	"vd.test/core/authz"
)

type errorOut struct {
	Code    string `+"`json:\"code\"`"+`
	Message string `+"`json:\"message\"`"+`
}

func Register(mux *http.ServeMux) {
	// @summary  Có
	// @reply    200 -
	// @reply    400 errorOut
	// @reply    409 errorOut task_tree petition_state
	// @reply    409 errorOut code_taken
	mux.Handle("POST /api/v1/things", authz.Public("lý do")(http.HandlerFunc(nil)))
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
	doc, _, err := dungTaiLieu(tuyens, gm)
	if err != nil {
		t.Fatal(err)
	}
	b, err := machJSON(doc, "")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	want409 := `"409":{"description":"Conflict","content":{"application/json":{"schema":{"$ref":"#/components/schemas/thu.errorOut"}}},"x-vigov-error-codes":["code_taken","petition_state","task_tree"]}`
	if !strings.Contains(s, want409) {
		t.Errorf("409 không mang mã đã gộp và sắp xếp, hoặc schema đã đổi khỏi $ref trần:\n%s", s)
	}
	want400 := `"400":{"description":"Bad Request","content":{"application/json":{"schema":{"$ref":"#/components/schemas/thu.errorOut"}}}}`
	if !strings.Contains(s, want400) {
		t.Errorf("400 không khai mã thì phải giữ nguyên hình dạng cũ:\n%s", s)
	}
	if strings.Count(s, "x-vigov-error-codes") != 1 {
		t.Errorf("x-vigov-error-codes chỉ được xuất hiện đúng trên 409:\n%s", s)
	}
}
