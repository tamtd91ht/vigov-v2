package http

import (
	"net/http"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// THE HTTP HALF OF ADR 0059 §1 ON THE SINGLE FORMS: a create without an address is accepted and
// reaches the use case as "", and the two new/changed refusals reach the client with the right
// status and code. Whether "" is stored as NULL and whether clearing is refused are decisions of the
// use case, proven in app/staff_email_optional_test.go — asserting them here through a fake would
// assert that the fake returns what it was told.

func TestStaffCreateWithoutEmailAnswers201(t *testing.T) {
	for _, body := range []string{
		`{"full_name":"Trần Thị B","position":"Công chức"}`,
		`{"full_name":"Trần Thị B","email":"","position":"Công chức"}`,
	} {
		m := dungMayChu(t)
		tok := m.tokenCho(t, xaA, sidA)
		tg := moiTuyenGhi()[0]
		tg.than = body

		doiMa(t, m.goiGhi(t, tg, hostA, tok), http.StatusCreated)
		if got := m.ghiDanhBa.themCuoi.Email; got != "" {
			t.Errorf("%s: email tới use case = %q, muốn rỗng", body, got)
		}
	}
}

// `email: ""` ON THE EDIT IS A CLEAR — a non-nil pointer to "" — and never dropped as "unchanged".
func TestStaffEditEmptyEmailReachesUseCaseAsClear(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	tg := moiTuyenGhi()[1]
	tg.than = `{"email":""}`

	doiMa(t, m.goiGhi(t, tg, hostA, tok), http.StatusOK)
	if e := m.ghiDanhBa.suaCuoi.Email; e == nil || *e != "" {
		t.Errorf("email tới use case = %v, muốn con trỏ tới chuỗi rỗng", e)
	}
}

func TestStaffEmailRefusalsMapToStatus(t *testing.T) {
	cases := []struct {
		name   string
		route  int
		err    error
		status int
		code   string
	}{
		{"xoá email người có tài khoản", 1, app.ErrStaffEmailIsLogin, http.StatusConflict, "staff_email_is_login"},
		{"email sai định dạng khi thêm", 0, domain.ErrEmailSaiDinhDang, http.StatusBadRequest, "invalid_request"},
		{"email sai định dạng khi sửa", 1, domain.ErrEmailSaiDinhDang, http.StatusBadRequest, "invalid_request"},
	}
	for _, c := range cases {
		m := dungMayChu(t)
		m.ghiDanhBa.loi = c.err
		tok := m.tokenCho(t, xaA, sidA)

		w := m.goiGhi(t, moiTuyenGhi()[c.route], hostA, tok)
		if w.Code != c.status {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", c.name, w.Code, c.status, w.Body.String())
			continue
		}
		if e := loiTra(t, w); e.Code != c.code {
			t.Errorf("%s: code = %q, muốn %q", c.name, e.Code, c.code)
		}
	}
}
