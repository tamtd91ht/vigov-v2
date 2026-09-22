package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
)

// WHAT THIS FILE IS FOR: GET /api/v1/sessions/current is the route every screen calls on load, and
// it is the one place the server tells a client what that client may draw. Four things have to
// hold:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestPhienHienTai_401XaKhac for why the third one is 401 here and not 403;
//  2. the permissions are the CALLER's, in THIS commune — the same account signed in at another
//     commune must not see the grants it holds here (rule 1, rule 5 invariant 3);
//  3. each permission is one FLAT key (rule 5, invariant 3b);
//  4. nothing a credential could be reconstructed from comes back: no token, no hash, no phone
//     number, no email.

const duongPhienHienTai = "/api/v1/sessions/current"

func docPhien(t *testing.T, than []byte) phienHienTaiRa {
	t.Helper()
	var ra phienHienTaiRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestPhienHienTai_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongPhienHienTai, "", ""), http.StatusUnauthorized)
	if m.quyen.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh sách quyền")
	}
}

func TestPhienHienTai_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, AS IT READS ON AN AnyAuthenticated ROUTE. There
	// is no permission to be right or wrong about, so the case it becomes is the one dimension
	// that still applies: a token issued by commune A, presented at commune B's domain. It is
	// refused at the token layer, before any store is touched — a browser does not send a cookie
	// across hosts, so this is never a user mistake.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.quyen.goi != 0 || m.phien.soLanDoc != 0 {
		t.Error("token của xã khác mà vẫn đọc dữ liệu của xã này")
	}
}

func TestPhienHienTai_401PhienDaThuHoi(t *testing.T) {
	// The session registry is the whole reason a signed token is not enough: an account locked or
	// a session revoked must stop working on the very NEXT request, not at expiry. This route is
	// the one a client polls, so it is where a revoked session is noticed first.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	doiMa(t, m.goi(t, "GET", hostA, duongPhienHienTai, "", tok), http.StatusOK)

	m.phien.thuHoi[sidA] = true

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", tok)
	doiMa(t, w, http.StatusUnauthorized)
	if c := cookiePhien(t, w); c == nil || c.MaxAge >= 0 {
		t.Error("phiên đã thu hồi mà cookie chết vẫn nằm lại trình duyệt")
	}
}

func TestPhienHienTai_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docPhien(t, w.Body.Bytes())
	if ra.Sid != sidA {
		t.Errorf("sid = %q, muốn %q", ra.Sid, sidA)
	}
	if ra.ExpiresAt.IsZero() {
		t.Error("thiếu expires_at — client không biết khi nào phải đăng nhập lại")
	}
	if ra.Staff.Code != maCanBo {
		t.Errorf("staff.code = %q, muốn mã cán bộ %q — mã nghiệp vụ, không phải id nội bộ",
			ra.Staff.Code, maCanBo)
	}
	if ra.Staff.Code == idNoiBo {
		t.Error("staff.code đang là id nội bộ — đây là mã nghiệp vụ, thứ vết kiểm toán trích dẫn")
	}
	if ra.Staff.FullName != "Nguyễn Văn A" || ra.Staff.Position != "Công chức Văn phòng" {
		t.Errorf("thông tin cán bộ sai: %+v", ra.Staff)
	}
}

// --- (2) the permissions are the caller's, in this commune --------------------------------------

func TestPhienHienTaiTraQuyenCuaChinhNguoiGoi(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docPhien(t, w.Body.Bytes())
	// Sorted by the query (ORDER BY quyen_ma) and by the fixture, so this compares a sequence,
	// not a set: an unstable order is a client-side diff that flickers on every poll.
	if got := strings.Join(ra.Permissions, ","); got != "admin.user,task.read" {
		t.Errorf("permissions = %q, muốn đúng quyền của tài khoản này ở xã này", got)
	}
}

func TestPhienHienTaiQuyenKhongVuotSangXaKhac(t *testing.T) {
	// THE ONE THAT MATTERS MOST. The SAME account, signed in properly at commune B, where it has
	// been granted nothing. Commune A's grants must not travel with the person. Nothing about the
	// request is malformed — this is the shape a leak actually takes (rule 5, invariant 3).
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongPhienHienTai, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	ra := docPhien(t, w.Body.Bytes())
	if len(ra.Permissions) != 0 {
		t.Fatalf("RÒ RỈ: ở xã B tài khoản này nhận %v — đó là quyền của xã A", ra.Permissions)
	}
	// [] AND NOT null: an account whose roles were withdrawn is a real, ordinary state — it is
	// precisely the account this route is AnyAuthenticated for — and a client that has to handle
	// both shapes handles one of them wrong.
	if !strings.Contains(w.Body.String(), `"permissions":[]`) {
		t.Errorf("permissions phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- (3) one flat key ---------------------------------------------------------------------------

func TestPhienHienTaiQuyenLaKhoaPhang(t *testing.T) {
	// Rule 5, invariant 3b: ONE flat key, "<nhóm>.<việc>" — the same string the Phân quyền screen
	// shows and the `quyen` table stores. Never a (subsystem, action) pair: `task.approve` closes a
	// commitment made to a citizen and `task.extend` moves its deadline, and a model that composes
	// them makes holding one imply the other.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	// Read as raw JSON, not into the typed struct: a shape change from ["a.b"] to [{"subsystem":
	// ...}] would still unmarshal into some Go value somewhere, and the contract the web builds
	// against is the JSON.
	var than struct {
		Permissions []json.RawMessage `json:"permissions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &than); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(than.Permissions) == 0 {
		t.Fatal("không có quyền nào để kiểm — fixture hỏng")
	}
	for _, q := range than.Permissions {
		var s string
		if err := json.Unmarshal(q, &s); err != nil {
			t.Fatalf("quyền %s không phải một chuỗi — khoá quyền là MỘT khoá phẳng", q)
		}
		if strings.Count(s, ".") != 1 || strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") {
			t.Errorf("khoá quyền %q không có dạng <nhóm>.<việc>", s)
		}
	}
}

// --- (4) nothing that is a credential ------------------------------------------------------------

func TestPhienHienTaiKhongTraTokenHashHayDuLieuCaNhan(t *testing.T) {
	// The token is in an httpOnly cookie SO THAT JavaScript cannot read it; returning it in a body
	// undoes that in one line. The hash is a credential. The phone number is personal data under
	// Decree 13/2023 (rule 3) and no screen this feeds needs it.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", tok)
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, tok) {
		t.Fatal("phản hồi chứa CHÍNH token phiên — trả lại một thông tin xác thực đang còn hiệu lực")
	}
	for _, cam := range []string{
		"0900000000", "argon2", "mat_khau", "password", "refresh", "token",
		"dien_thoai_co_quan", "phone", emailDung, "email",
	} {
		if strings.Contains(strings.ToLower(than), strings.ToLower(cam)) {
			t.Errorf("phản hồi chứa %q: %s", cam, than)
		}
	}
}

// --- failing closed --------------------------------------------------------------------------

func TestPhienHienTaiKhongDocDuocQuyenTra500(t *testing.T) {
	// 500 AND NOT AN EMPTY LIST. Empty is a legitimate answer — the account in commune B above has
	// exactly that — so answering [] on failure would be indistinguishable from it, and the screen
	// would state "you may do nothing" when the truth is "we could not find out". Refusing widens
	// nothing: the server-side check is untouched either way.
	m := dungMayChu(t)
	m.quyen.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	e := loiTra(t, w)
	if e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	// The wrapped store error never reaches the client (rule 3, forbidden #3).
	if strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

func TestPhienHienTaiDocQuyenTheoPrincipalIDNoiBo(t *testing.T) {
	// The mirror of TestPrincipalIDLaIDNoiBoChuKhongPhaiMaCanBo, for the permission list: the real
	// query matches `nd.id = $2` against Principal.ID. Asking with the business code matches no
	// row, so the list comes back empty and every menu disappears — with nothing in the response,
	// the logs or a test to say why.
	m := dungMayChu(t)
	var thay authz.Principal
	m.dungLai(t, func(d *Deps) {
		d.Quyen = quyenGhiNhan{trong: m.quyen, ghi: &thay}
	})

	doiMa(t, m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA)), http.StatusOK)

	if thay.ID != idNoiBo {
		t.Fatalf("hỏi quyền bằng %q, muốn id nội bộ %q", thay.ID, idNoiBo)
	}
	if thay.Kind != "staff" {
		t.Errorf("Kind = %q, muốn staff", thay.Kind)
	}
}

// --- (5) the caller's role --------------------------------------------------------------------
//
// The role is on this reply for ONE reason — the header prints it, and `is_leader` chooses which
// screen the app opens on (docs/ui-ux/15-phu-luc-giao-dien-chung.md §1). Everything asserted below
// is about keeping it that and nothing more.

func TestPhienHienTaiTraVaiTroCuaChinhNguoiGoi(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docPhien(t, w.Body.Bytes())
	if ra.Role == nil {
		t.Fatal("thiếu vai trò — tài khoản này CÓ vai trò ở xã A")
	}
	if ra.Role.Code != "chu-tich-ubnd" || ra.Role.Name != "Chủ tịch UBND" {
		t.Errorf("vai trò sai: %+v", *ra.Role)
	}
	if !ra.Role.IsLeader {
		t.Error("is_leader = false với vai trò lãnh đạo — màn hình mặc định sẽ mở sai")
	}
}

func TestPhienHienTaiCoVaiTroThiDocTrongHandlerChuKhongPhaiONhungTuyenKhac(t *testing.T) {
	// THE DECISION THIS PINS: the role is read by ONE route, not by the edge.
	//
	// XacThuc runs on every request of every route and already holds this person's row, so carrying
	// the role along there looks free. It is not: the role is a second table, therefore a join, and
	// the edge would pay it on every request of every screen to save one query on one route.
	//
	// Asserted by counting: a request to a DIFFERENT route must not read the role register at all.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", tok), http.StatusOK)
	if m.vaiTro.goi != 0 {
		t.Fatalf("đọc vai trò %d lần trên một tuyến KHÔNG cần vai trò — phép đọc đã trôi vào biên",
			m.vaiTro.goi)
	}

	doiMa(t, m.goi(t, "GET", hostA, duongPhienHienTai, "", tok), http.StatusOK)
	if m.vaiTro.goi != 1 {
		t.Errorf("đọc vai trò %d lần cho một lần gọi, muốn đúng 1", m.vaiTro.goi)
	}
}

func TestPhienHienTaiChuaGanVaiTroThiTraNull(t *testing.T) {
	// "CHƯA GÁN VAI TRÒ" IS A REAL STATE, not an error: `nguoi_dung.vai_tro_id` is nullable and a
	// person can sit in the commune's directory before anybody decides what they do.
	//
	// null AND NOT AN EMPTY OBJECT. `{"code":"","name":"","is_leader":false}` reads as a role that
	// exists and is nameless, and the `is_leader:false` inside it is an assertion nobody made — a
	// client that renders a role name would print an empty chip in the header.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongPhienHienTai, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	if ra := docPhien(t, w.Body.Bytes()); ra.Role != nil {
		t.Fatalf("chưa gán vai trò mà trả về %+v", *ra.Role)
	}
	// Asserted on the raw JSON as well: the typed struct cannot tell null from an absent key, and
	// an absent key is a second response shape for the client to get wrong.
	if !strings.Contains(w.Body.String(), `"role":null`) {
		t.Errorf(`phải tuần tự hoá thành "role":null, nhận: %s`, w.Body.String())
	}
}

func TestPhienHienTaiVaiTroKhongVuotSangXaKhac(t *testing.T) {
	// The SAME account, signed in properly at commune B, where it has been given no role. Commune
	// A's role — a LEADER role — must not travel with the person. Nothing about the request is
	// malformed; this is the shape a leak actually takes (rule 1, invariant 4).
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongPhienHienTai, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "chu-tich-ubnd") || strings.Contains(than, "Chủ tịch UBND") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được vai trò của xã A: %s", than)
	}
	if strings.Contains(than, `"is_leader":true`) {
		t.Fatalf("RÒ RỈ: cờ lãnh đạo của xã A theo người sang xã B: %s", than)
	}
}

func TestPhienHienTaiKhongDocDuocVaiTroTra500ChuKhongPhaiNull(t *testing.T) {
	// THE DISTINCTION THE RESPONSE SHAPE EXISTS FOR. null already means "no role assigned", so
	// answering null on a failure makes the two indistinguishable — and the screen would tell a
	// member of staff they hold no role when the truth is that nobody could find out. Refusing
	// widens nothing: this value decides no access.
	m := dungMayChu(t)
	m.vaiTro.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	e := loiTra(t, w)
	if e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	if strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

func TestPhienHienTaiDocVaiTroTheoIDNoiBoChuKhongPhaiMaCanBo(t *testing.T) {
	// The same trap as the permission list: the real query matches `nd.id = $2`. Asking with the
	// business code matches no row, and the person is then reported as having no role — a wrong
	// answer that looks exactly like a legitimate one.
	//
	// vaiTroGia is keyed by the internal id, so a swap makes this red rather than silently null.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongPhienHienTai, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if ra := docPhien(t, w.Body.Bytes()); ra.Role == nil {
		t.Fatal("vai trò rỗng — handler nhiều khả năng đang hỏi bằng mã cán bộ chứ không phải id nội bộ")
	}
}

// quyenGhiNhan records the principal the handler asked with, then delegates.
type quyenGhiNhan struct {
	trong *quyenGia
	ghi   *authz.Principal
}

func (q quyenGhiNhan) QuyenCua(ctx context.Context, p authz.Principal) ([]authz.Perm, error) {
	*q.ghi = p
	return q.trong.QuyenCua(ctx, p)
}
