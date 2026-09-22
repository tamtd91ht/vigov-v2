package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/role-permissions is the first route in this service that hands back
// the commune's AUTHORISATION MODEL — which role may do what. Five things have to hold, and every
// one of them fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on a route that really does declare a permission —
//     `admin.role`, the key spec §12.8 puts on the Phân quyền tab;
//  2. one commune's matrix never reaches another commune's caller (rule 1);
//  3. the matrix is returned WHOLE, grouped in the store's order, or refused — never trimmed,
//     because a missing row reads exactly like a role that does not hold that right;
//  4. a commune with nothing set up serialises as [] and not null;
//  5. nothing about a PERSON is in the response — only a count (rule 3).

const duongPhanQuyen = "/api/v1/role-permissions"

// quyenPhanQuyen is the key the route declares. Spelled out here rather than reused from the route
// so that changing the declaration without meaning to turns these tests red.
const quyenPhanQuyen = authz.Perm("admin.role")

// mayChuPhanQuyen builds the real chain with a checker that grants `admin.role` in the named
// communes AND NOTHING ELSE — not admin.user, not task.read. A fixture that granted everything
// could not tell "this route asked for admin.role" from "this route asked for anything at all".
func mayChuPhanQuyen(t *testing.T, xa ...tenant.ID) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	cap := make(map[tenant.ID]map[string]map[authz.Perm]bool, len(xa))
	for _, x := range xa {
		cap[x] = map[string]map[authz.Perm]bool{idNoiBo: {quyenPhanQuyen: true}}
	}
	m.dungLai(t, func(d *Deps) { d.Checker = checkerGia{quyen: cap} })
	return m
}

func docMaTran(t *testing.T, than []byte) maTranQuyenRa {
	t.Helper()
	var ra maTranQuyenRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestMaTranQuyen_401KhongToken(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)

	doiMa(t, m.goi(t, "GET", hostA, duongPhanQuyen, "", ""), http.StatusUnauthorized)
	if m.maTran.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc ma trận phân quyền")
	}
}

func TestMaTranQuyen_403SaiQuyen(t *testing.T) {
	// Signed in, right commune, holding `admin.user` — the permission on the NEIGHBOURING tab of
	// the very same screen (Cấu hình → Người dùng). That is the realistic near miss: the two tabs
	// sit side by side and an administrator who may manage accounts is not thereby entitled to
	// read where authority sits in the authority.
	m := dungMayChu(t) // the default checker grants quyenThu = admin.user in commune A

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
	if m.maTran.goi != 0 {
		t.Error("thiếu admin.role mà đã đọc ma trận phân quyền")
	}
}

func TestMaTranQuyen_401TokenXaAToiHostXaB(t *testing.T) {
	// A token issued by commune A presented at commune B's domain. Refused at the TOKEN layer,
	// before any store is touched — which is what makes the matrix unreachable across communes
	// rather than merely unrequested (rule 1, invariant 8).
	m := mayChuPhanQuyen(t, xaA, xaB)

	w := m.goi(t, "GET", hostB, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.maTran.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc ma trận của xã này")
	}
}

func TestMaTranQuyen_403DungQuyenSaiXa(t *testing.T) {
	// The same person, holding `admin.role` in commune A, signed in PROPERLY at commune B. Nothing
	// about the request is malformed — the grant simply does not exist in commune B, and a
	// permission that crossed the commune would be privilege escalation (rule 5, invariant 3).
	m := mayChuPhanQuyen(t, xaA)

	w := m.goi(t, "GET", hostB, duongPhanQuyen, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusForbidden)
	if m.maTran.goi != 0 {
		t.Error("không có quyền ở xã B mà vẫn đọc ma trận của xã B")
	}
}

func TestMaTranQuyen_200(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	ra := docMaTran(t, w.Body.Bytes())

	// --- the rows: grouped by `nhom`, in the order the keys arrived --------------------------
	//
	// The fixture interleaves the groups (QUẢN TRỊ, NHIỆM VỤ, QUẢN TRỊ) precisely so this asserts
	// something: a handler that grouped by scanning for runs would produce THREE groups with one
	// name appearing twice — two bands with one heading, which reads as two different things.
	if len(ra.Groups) != 2 {
		t.Fatalf("nhận %d nhóm quyền, muốn 2: %+v", len(ra.Groups), ra.Groups)
	}
	if ra.Groups[0].Name != "QUẢN TRỊ" || ra.Groups[1].Name != "NHIỆM VỤ" {
		t.Errorf("thứ tự nhóm sai — nhóm phải theo lần xuất hiện đầu tiên: %q, %q",
			ra.Groups[0].Name, ra.Groups[1].Name)
	}
	if len(ra.Groups[0].Permissions) != 2 {
		t.Fatalf("nhóm QUẢN TRỊ có %d quyền, muốn 2", len(ra.Groups[0].Permissions))
	}
	// Order INSIDE the group is the store's `thu_tu` order as well, not alphabetical: admin.role
	// came first in the fixture and admin.user second.
	if ra.Groups[0].Permissions[0].Code != "admin.role" || ra.Groups[0].Permissions[1].Code != "admin.user" {
		t.Errorf("thứ tự quyền trong nhóm đã bị đổi: %+v", ra.Groups[0].Permissions)
	}
	if ra.Groups[0].Permissions[0].Label != "Phân quyền" {
		t.Errorf("nhãn quyền sai: %+v — màn hình sẽ hiện mã thay cho nhãn",
			ra.Groups[0].Permissions[0])
	}

	// --- the columns: roles with a COUNT of staff ---------------------------------------------
	if len(ra.Roles) != 2 {
		t.Fatalf("nhận %d vai trò, muốn 2", len(ra.Roles))
	}
	if ra.Roles[0].ID != "vt-001" || ra.Roles[0].Code != "chu-tich-ubnd" || ra.Roles[0].Name != "Chủ tịch UBND" {
		t.Errorf("vai trò đầu sai: %+v", ra.Roles[0])
	}
	if !ra.Roles[0].IsLeader || ra.Roles[1].IsLeader {
		t.Errorf("nhãn Lãnh đạo sai: %+v / %+v", ra.Roles[0], ra.Roles[1])
	}
	// The header reads "{n} cán bộ" (spec §4.1). A count that is always 1 is what a LEFT JOIN
	// counted with count(*) produces, so the fixture uses 1 and 3 — two different non-zero values.
	if ra.Roles[0].StaffCount != 1 || ra.Roles[1].StaffCount != 3 {
		t.Errorf("số cán bộ sai: %d / %d, muốn 1 / 3", ra.Roles[0].StaffCount, ra.Roles[1].StaffCount)
	}
	// THE SECOND NUMBER IS ITS OWN NUMBER, not a copy of the first. vt-002 is the expensive case:
	// three people hold the role and not one of them can sign in, so every tick on that column
	// reaches nobody. See TestMaTranQuyenHaiSoDemLaHaiSoKhacNhau for why that case is asserted on
	// its own as well.
	if ra.Roles[0].ActiveAccountCount != 1 || ra.Roles[1].ActiveAccountCount != 0 {
		t.Errorf("số tài khoản đang hoạt động sai: %d / %d, muốn 1 / 0",
			ra.Roles[0].ActiveAccountCount, ra.Roles[1].ActiveAccountCount)
	}

	// --- the cells: sparse pairs, role FIRST ---------------------------------------------------
	if len(ra.Grants) != 2 {
		t.Fatalf("nhận %d ô đã cấp, muốn 2: %+v", len(ra.Grants), ra.Grants)
	}
	// THE FIELD NAMES ARE THE CONTRACT and are asserted on the RAW BODY, not on the decoded struct:
	// the decoded form cannot show a renamed or reordered JSON key, and the web builds its types
	// from the generated contract rather than from this struct (ADR 0014).
	if !strings.Contains(w.Body.String(),
		`"grants":[{"role_id":"vt-001","permission":"admin.role"},{"role_id":"vt-001","permission":"task.extend"}]`) {
		t.Errorf("ô đã cấp phải là {role_id, permission} đúng thứ tự: %s", w.Body.String())
	}
	// A cell that is absent is NOT granted — there is no third state, and nothing in the response
	// says `false`. vt-002 holds nothing in the fixture, so it must not appear among the cells.
	for _, g := range ra.Grants {
		if g.RoleID == "vt-002" {
			t.Errorf("vai trò không được cấp gì mà vẫn có ô: %+v", g)
		}
	}
	// SPARSE, NOT DENSE: two roles times three keys is six cells, and only the two ticked ones are
	// on the wire. Asserting the count rather than scanning the body for `false` — `is_leader` is
	// legitimately false on one of the roles, and a scan would pin the wrong thing.
	if len(ra.Grants) == len(ra.Roles)*3 {
		t.Errorf("ma trận dày đặc đã quay lại — phải là danh sách ô đã bật: %s", w.Body.String())
	}
}

func TestMaTranQuyenHaiSoDemLaHaiSoKhacNhau(t *testing.T) {
	// THE CASE THE CUSTOMER ASKED FOR BOTH NUMBERS BECAUSE OF: a role that people hold and that
	// NONE of them can currently use — three staff in the register, no live account among them.
	//
	// The column has to read (3, 0). The two ways it can be wrong are both plausible-looking and
	// both silent:
	//
	//	(3, 3)  the second count is the first one copied — the FILTER was dropped, or pushed into
	//	        the JOIN's ON clause where it would narrow both. The header then claims every tick
	//	        on this column reaches three people, when it reaches nobody.
	//	(0, 0)  the predicate was pushed up into the WHERE, which drops the whole ROLE the moment no
	//	        holder has a live account — so the column with the most to say disappears from the
	//	        matrix entirely.
	//
	// The store's SQL is where that is decided (TestPgMaTranHaiSoDem); what is asserted here is
	// that the handler carries the two numbers through as two numbers and does not derive one from
	// the other.
	m := mayChuPhanQuyen(t, xaA)

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docMaTran(t, w.Body.Bytes())
	var thay *vaiTroCotRa
	for i := range ra.Roles {
		if ra.Roles[i].ID == "vt-002" {
			thay = &ra.Roles[i]
		}
	}
	if thay == nil {
		t.Fatalf("vai trò không ai dùng được đã biến mất khỏi ma trận: %+v", ra.Roles)
	}
	if thay.StaffCount != 3 {
		t.Errorf("staff_count = %d, muốn 3 — con số này phải khớp tab Người dùng", thay.StaffCount)
	}
	if thay.ActiveAccountCount != 0 {
		t.Errorf("active_account_count = %d, muốn 0 — cột này bật/tắt quyền của KHÔNG AI",
			thay.ActiveAccountCount)
	}
	// Asserted on the RAW BODY as well: the field names are the contract, and the web builds its
	// types from the generated contract rather than from this struct (ADR 0014).
	if !strings.Contains(w.Body.String(), `"staff_count":3,"active_account_count":0`) {
		t.Errorf("hai số đếm phải là hai trường riêng đúng tên: %s", w.Body.String())
	}
}

// --- (2) one commune's matrix never reaches another ---------------------------------------------

func TestMaTranQuyenKhongVuotSangXaKhac(t *testing.T) {
	// The same account, holding `admin.role` in BOTH communes and signed in properly at commune B.
	// Nothing about the request is malformed — this is the shape a leak actually takes. It is the
	// heaviest leak this service could have: the matrix is another authority's authorisation model.
	m := mayChuPhanQuyen(t, xaA, xaB)

	w := m.goi(t, "GET", hostB, duongPhanQuyen, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	for _, cuaXaA := range []string{"vt-001", "vt-002", "Chủ tịch UBND", "Cán bộ một cửa"} {
		if strings.Contains(than, cuaXaA) {
			t.Fatalf("RÒ RỈ: ở xã B nhận được vai trò/ô cấp của xã A (%q): %s", cuaXaA, than)
		}
	}
	ra := docMaTran(t, w.Body.Bytes())
	if len(ra.Roles) != 1 || ra.Roles[0].ID != "vt-101" {
		t.Fatalf("xã B phải nhận đúng vai trò của mình, nhận: %+v", ra.Roles)
	}
	if len(ra.Grants) != 1 || ra.Grants[0].RoleID != "vt-101" || ra.Grants[0].Permission != "admin.user" {
		t.Fatalf("xã B phải nhận đúng ô đã cấp của mình, nhận: %+v", ra.Grants)
	}
	// THE CATALOGUE IS THE SAME IN BOTH COMMUNES AND THAT IS NOT A LEAK: `quyen` has no tenant_id
	// because the set of rights the SOFTWARE enforces is the same everywhere (migration
	// 0001_init.sql:44). Asserted here so nobody "fixes" the catalogue into per-commune data.
	if len(ra.Groups) != 2 {
		t.Errorf("danh mục quyền phải giống nhau ở mọi xã, nhận %d nhóm", len(ra.Groups))
	}
}

// --- (3) whole matrix, store's order, or refused ------------------------------------------------

func TestMaTranQuyenVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the part it managed to read.
	//
	// A short matrix is a permission key that has vanished from the screen. The administrator reads
	// the role as not holding that right, ticks or unticks accordingly, and nothing anywhere says
	// the list was incomplete. A refusal breaks ONE commune's screen loudly and names itself in the
	// log. Between a wrong answer nobody notices and no answer somebody fixes, this chooses the
	// second (fail closed).
	//
	// All THREE ceilings are asserted: whichever list overflows, the answer is the same refusal.
	// The ceilings themselves live in the store, because only the store knows the LIMIT.
	for ten, loi := range map[string]error{
		"quá nhiều quyền":   idstore.ErrQuaNhieuQuyen,
		"quá nhiều vai trò": idstore.ErrQuaNhieuVaiTro,
		"quá nhiều ô cấp":   idstore.ErrQuaNhieuCapQuyen,
	} {
		t.Run(ten, func(t *testing.T) {
			m := mayChuPhanQuyen(t, xaA)
			m.maTran.loi = loi

			w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
			doiMa(t, w, http.StatusInternalServerError)

			if e := loiTra(t, w); e.Code != "internal" {
				t.Errorf("code = %q, muốn internal", e.Code)
			}
			// No partial matrix came back with the error. A body carrying items alongside a 500 is
			// how a refusal turns back into a truncation on a client that reads the body anyway.
			for _, truong := range []string{`"groups"`, `"roles"`, `"grants"`} {
				if strings.Contains(w.Body.String(), truong) {
					t.Errorf("phản hồi từ chối vẫn kèm %s: %s", truong, w.Body.String())
				}
			}
		})
	}
}

func TestMaTranQuyenLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	// Rule 3, forbidden #3: a wrapped driver error can carry a column value with it, so the edge
	// answers with a fixed sentence and keeps the detail on the server side.
	m := mayChuPhanQuyen(t, xaA)
	m.maTran.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) a commune with nothing set up ----------------------------------------------------------

func TestMaTranQuyenXaChuaCauHinhTraMangRong(t *testing.T) {
	// [] AND NOT null, on every list. A newly onboarded commune has no roles and no grants yet — an
	// ordinary state, not an error — while the CATALOGUE is already full, because it ships with the
	// software. A client that has to handle both shapes handles one of them wrong.
	m := mayChuPhanQuyen(t, xaA)
	m.maTran.theo[xaA] = domain.MaTranQuyen{}

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	for _, phai := range []string{`"roles":[]`, `"grants":[]`} {
		if !strings.Contains(than, phai) {
			t.Errorf("thiếu %s — danh sách rỗng phải tuần tự hoá thành [], nhận: %s", phai, than)
		}
	}
	if strings.Contains(than, "null") {
		t.Errorf("phản hồi chứa null: %s", than)
	}
	if len(docMaTran(t, w.Body.Bytes()).Groups) == 0 {
		t.Error("xã chưa cấu hình vẫn phải nhận đủ danh mục quyền — nếu không thì không tick được gì")
	}
}

func TestMaTranQuyenDanhMucRongVanTraMangRong(t *testing.T) {
	// The other half of the same property: `groups` must be [] and not null too. It cannot happen
	// in production — the catalogue is seeded by the migration the binary carries — but the nil
	// slice it would produce is one line away at all times.
	m := mayChuPhanQuyen(t, xaA)
	m.maTran.danhMuc = nil

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"groups":[]`) {
		t.Errorf("groups phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- (5) no personal data ------------------------------------------------------------------------

func TestMaTranQuyenChiDemKhongTraTenNguoi(t *testing.T) {
	// Rule 3: the column header says "{n} cán bộ" and that is all the screen needs. The moment this
	// route starts naming the people in a role, a configuration screen becomes a personal-data
	// surface — and the obvious "improvement" (a tooltip listing who holds the role) is exactly the
	// change this test exists to stop.
	//
	// The fixture account carries the agreed fake number and a name precisely so this has something
	// to look for.
	m := mayChuPhanQuyen(t, xaA)

	w := m.goi(t, "GET", hostA, duongPhanQuyen, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	than := strings.ToLower(w.Body.String())
	for _, cam := range []string{"0900000000", "nguyễn văn a", emailDung, maCanBo, idNoiBo,
		"ho_ten", "full_name", "email", "phone", "dien_thoai_co_quan"} {
		if strings.Contains(than, strings.ToLower(cam)) {
			t.Errorf("ma trận phân quyền chứa %q: %s", cam, w.Body.String())
		}
	}
}
