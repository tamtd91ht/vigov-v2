package http

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// THE DEFECT CLASS THIS FILE CLOSES: a log line with no commune on it.
//
// One process serves 200+ communes into ONE log stream. "A sign-in failed with a system error"
// is not something an operator can act on; "…in commune X" is. The security alert in
// middleware.go already carries both communes — these two lines, the ones written when a
// government service actually fails somebody, did not.
//
// A commune id is an opaque ULID (rule 1, invariant 2). It is not personal data and not an
// administrative code, so nothing about rule 3 stands in the way.

func TestLoi500CuaDangXuatMangXa(t *testing.T) {
	m, log := mayChuLog(t)
	m.dangXuat.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	ra := log.String()
	if !strings.Contains(ra, "xa="+string(xaA)) {
		t.Errorf("dòng log 500 của đăng xuất thiếu xã — không biết xã nào đang hỏng:\n%s", ra)
	}
	if !strings.Contains(ra, "can_bo="+maCanBo) {
		t.Errorf("dòng log 500 của đăng xuất thiếu mã cán bộ:\n%s", ra)
	}
}

func TestLoi500CuaDangNhapMangXa(t *testing.T) {
	m, log := mayChuLog(t)
	m.dangNhap.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "POST", hostB, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "")
	doiMa(t, w, http.StatusInternalServerError)

	ra := log.String()
	if !strings.Contains(ra, "xa="+string(xaB)) {
		t.Errorf("dòng log 500 của đăng nhập thiếu xã — xã nào không đăng nhập được:\n%s", ra)
	}
	// And still nothing from the body: the commune is the only thing that was added.
	for _, cam := range []string{matKhauDung, emailDung} {
		if strings.Contains(ra, cam) {
			t.Errorf("dòng log chứa %q:\n%s", cam, ra)
		}
	}
}
