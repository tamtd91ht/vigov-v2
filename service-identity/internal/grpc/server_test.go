package grpc

// What these tests defend: the BRANCHES of the handlers — which failure becomes "no principal",
// which becomes an error, and in what ORDER the steps run. AdvanceWorkingHours has its own file,
// lich_lam_viec_test.go, for the same split the source keeps.
//
// The wiring is a different question and is defended somewhere else: cmd/server/main_test.go
// starts the real construction function over a real connection, because nothing in this file can
// see whether the binary installs the interceptors.
//
// NO DATABASE HERE, ON PURPOSE. Every collaborator is an interface declared at the point of use,
// so these properties are checkable on every `go test` rather than only where a PostgreSQL
// happens to be running — and the ones that matter most (a store OUTAGE must not read as
// anonymity; the commune comparison must happen BEFORE the registry read) are properties of the
// Go code, not of the SQL.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// Fake key material, and the text says so in full (rule 8, forbidden #1). 32 bytes is
// token.KhoaToiThieu.
var khoaKyGia = secret.Secret("KHOA-KY-PHIEN-GIA-KHONG-PHAI-THAT-32B")

const (
	xaA = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")
	xaB = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EG")

	idCanBo = "01JD9AAAAAAAAAAAAAAAAAAAAA"
)

// loiKho stands for "the database is unreachable" — never for "this record is not usable". The
// difference between the two is the whole point of several tests below.
var loiKho = errors.New("giả lập: cơ sở dữ liệu không tới được")

type phienGia struct {
	p        idstore.Phien
	err      error
	soLanGoi int // how many times KiemTra ran — the step-order assertion reads this
	daGhiNho bool
}

func (f *phienGia) KiemTra(context.Context, string) (idstore.Phien, error) {
	f.soLanGoi++
	return f.p, f.err
}
func (f *phienGia) GhiNhanDung(context.Context, string) { f.daGhiNho = true }

type canBoGia struct {
	cb  domain.CanBo
	err error
}

func (f canBoGia) TheoID(context.Context, string) (domain.CanBo, error) { return f.cb, f.err }

type loGia struct {
	hang     []domain.CanBoVaiTro
	err      error
	daNhan   []string // exactly what the server passed down, after dedup and empty-dropping
	soLanGoi int
}

func (f *loGia) TheoNhieuID(_ context.Context, ids []string) ([]domain.CanBoVaiTro, error) {
	f.soLanGoi++
	f.daNhan = ids
	return f.hang, f.err
}

type quyenGia struct {
	quyen []authz.Perm
	err   error
}

func (f quyenGia) QuyenCua(context.Context, authz.Principal) ([]authz.Perm, error) {
	return f.quyen, f.err
}

// may builds a server whose collaborators all answer successfully, so each test overrides only
// the one thing it is about.
func may(t *testing.T, sua func(*Deps)) (*Server, *bytes.Buffer) {
	t.Helper()

	ky, err := token.NewSigner([]secret.Secret{khoaKyGia})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	nhatKy := &bytes.Buffer{}
	d := Deps{
		Signer: ky,
		Phien: &phienGia{p: idstore.Phien{
			ID: "sid-gia", NguoiDungID: idCanBo,
			HetHanLuc: time.Now().Add(time.Hour),
		}},
		CanBo: canBoGia{cb: domain.CanBo{ID: idCanBo, Ma: "CB001", CoTaiKhoan: true, DangHoatDong: true}},
		Lo:    &loGia{},
		Quyen: quyenGia{quyen: []authz.Perm{"admin.user", "task.extend"}},
		// The ordinary week, no closures, no swap days — so a test about AdvanceWorkingHours
		// overrides only the one table it is about. See lich_lam_viec_test.go.
		Lich:   &lichGia{cas: tuanGia()},
		NghiLe: &nghiLeGia{},
		LamBu:  &lamBuGia{},
		Log:    slog.New(slog.NewTextHandler(nhatKy, nil)),
	}
	if sua != nil {
		sua(&d)
	}
	return NewServer(d), nhatKy
}

func ctxXa(id tenant.ID) context.Context { return tenant.Into(context.Background(), id) }

// tokenCua mints a real, signed token for a commune, so the decode step is exercised rather than
// stubbed.
func tokenCua(t *testing.T, xa tenant.ID, sid string) string {
	t.Helper()
	ky, err := token.NewSigner([]secret.Secret{khoaKyGia})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	tok, err := ky.Ky(token.Claims{TenantID: xa, Sid: sid, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("Ky: %v", err)
	}
	return tok
}

// ---------------------------------------------------------------- ResolveStaffPrincipal

// An empty credential is a WIRING fault of the caller, answered loudly. The quiet answer would
// cost a round trip on every anonymous request, forever, with nothing to report it.
func TestResolveThieuTokenLaInvalidArgument(t *testing.T) {
	s, _ := may(t, nil)

	_, err := s.ResolveStaffPrincipal(ctxXa(xaA), &identityv1.ResolveStaffPrincipalRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

// Reaching a handler with no commune means the commune interceptor is not in the chain. It is a
// deployment fault, so it is Internal — and it must NOT be a panic, because a panic in a gRPC
// handler is not recovered and takes down a process serving 200+ communes.
func TestResolveKhongCoXaTrongContextLaInternalChuKhongPanic(t *testing.T) {
	s, nhatKy := may(t, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()

	_, err := s.ResolveStaffPrincipal(context.Background(),
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: tokenCua(t, xaA, "sid-gia")})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
	if !strings.Contains(nhatKy.String(), "UnaryServerInterceptor") {
		t.Errorf("log không chỉ ra nguyên nhân thật (interceptor thiếu):\n%s", nhatKy.String())
	}
}

// Unusable credentials all get ONE answer: OK, no principal, no reason code.
func TestResolveMoiCredentialKhongDungDuocChoCungMotCauTraLoi(t *testing.T) {
	tokA := tokenCua(t, xaA, "sid-gia")

	ca := []struct {
		ten string
		req *identityv1.ResolveStaffPrincipalRequest
		sua func(*Deps)
	}{
		{"token rác", &identityv1.ResolveStaffPrincipalRequest{SessionToken: "khong-phai-token"}, nil},
		{"phiên đã thu hồi", &identityv1.ResolveStaffPrincipalRequest{SessionToken: tokA},
			func(d *Deps) { d.Phien = &phienGia{err: idstore.ErrPhienKhongTonTai} }},
		{"phiên hết hạn", &identityv1.ResolveStaffPrincipalRequest{SessionToken: tokA},
			func(d *Deps) { d.Phien = &phienGia{err: idstore.ErrPhienHetHan} }},
		{"tài khoản khoá hoặc đã xoá mềm", &identityv1.ResolveStaffPrincipalRequest{SessionToken: tokA},
			func(d *Deps) { d.CanBo = canBoGia{err: idstore.ErrCanBoKhongTonTai} }},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			s, _ := may(t, c.sua)
			ra, err := s.ResolveStaffPrincipal(ctxXa(xaA), c.req)
			if err != nil {
				t.Fatalf("muốn OK, nhận lỗi: %v", err)
			}
			if ra.GetPrincipal() != nil {
				t.Errorf("muốn không có principal, nhận %+v", ra.GetPrincipal())
			}
		})
	}
}

// THE STEP-ORDER TEST, and it is the reason this file exists.
//
// A token issued for commune A arriving on a call that names commune B must be refused BEFORE the
// session registry is touched. Looking that sid up in commune B is a query in one commune for a
// value issued in another — a cross-commune read with no `// @cross-tenant:` justifying it.
// Moving the comparison after the registry read leaves every other assertion in this file green,
// so this counter is what turns red.
func TestResolveLechXaThiKhongDocKhoLanNao(t *testing.T) {
	ph := &phienGia{p: idstore.Phien{ID: "sid-gia", NguoiDungID: idCanBo,
		HetHanLuc: time.Now().Add(time.Hour)}}
	s, nhatKy := may(t, func(d *Deps) { d.Phien = ph })

	// The token says commune A; the call names commune B.
	ra, err := s.ResolveStaffPrincipal(ctxXa(xaB), &identityv1.ResolveStaffPrincipalRequest{
		SessionToken: tokenCua(t, xaA, "sid-bi-lo"),
		ClientIp:     "203.0.113.7",
	})
	if err != nil {
		t.Fatalf("muốn OK không principal, nhận lỗi: %v", err)
	}
	if ra.GetPrincipal() != nil {
		t.Fatalf("token của xã khác vẫn dựng được principal: %+v", ra.GetPrincipal())
	}
	if ph.soLanGoi != 0 {
		t.Errorf("đã đọc sổ phiên %d lần trước khi so xã — đó là một truy vấn chéo xã không có lý do khai",
			ph.soLanGoi)
	}

	// The alert is raised, it names both communes, and it carries NEITHER the token NOR the sid.
	ra2 := nhatKy.String()
	for _, phai := range []string{"CẢNH BÁO AN NINH", string(xaA), string(xaB), "203.0.113.7"} {
		if !strings.Contains(ra2, phai) {
			t.Errorf("cảnh báo thiếu %q:\n%s", phai, ra2)
		}
	}
	if strings.Contains(ra2, "sid-bi-lo") {
		t.Errorf("SID THÔ NẰM TRONG LOG — cảnh báo mang theo một bản sao còn dùng được của credential:\n%s", ra2)
	}
	if !strings.Contains(ra2, vanTay("sid-bi-lo")) {
		t.Errorf("cảnh báo không có vân tay sid, nên không đối chiếu được với cảnh báo của XacThuc:\n%s", ra2)
	}
}

// The token itself must never reach a log line, on ANY branch — including the ones that log
// nothing at all today. This is the assertion that turns red when somebody adds a debug line
// holding `req`.
func TestResolveKhongBaoGioGhiTokenVaoLog(t *testing.T) {
	tok := tokenCua(t, xaA, "sid-gia")

	ca := map[string]func(*Deps){
		"thành công":    nil,
		"lệch xã":       nil, // handled by the commune below
		"lỗi kho phiên": func(d *Deps) { d.Phien = &phienGia{err: loiKho} },
		"lỗi kho quyền": func(d *Deps) { d.Quyen = quyenGia{err: loiKho} },
	}

	for ten, sua := range ca {
		t.Run(ten, func(t *testing.T) {
			s, nhatKy := may(t, sua)
			xa := xaA
			if ten == "lệch xã" {
				xa = xaB
			}
			_, _ = s.ResolveStaffPrincipal(ctxXa(xa),
				&identityv1.ResolveStaffPrincipalRequest{SessionToken: tok})
			if strings.Contains(nhatKy.String(), tok) {
				t.Fatalf("TOKEN PHIÊN NẰM TRONG LOG — rule 8, và log không thu hồi lại được:\n%s",
					nhatKy.String())
			}
		})
	}
}

// AN OUTAGE IS NOT ANONYMITY. A caller that reads "the database is down" as "no principal" turns
// "identity is down" into "everybody is signed out", and staff are then told to sign in again
// through the service that is down.
func TestResolveLoiHaTangKhongBaoGioThanhKhongCoPrincipal(t *testing.T) {
	tok := tokenCua(t, xaA, "sid-gia")

	ca := map[string]func(*Deps){
		"kho phiên hỏng":  func(d *Deps) { d.Phien = &phienGia{err: loiKho} },
		"kho cán bộ hỏng": func(d *Deps) { d.CanBo = canBoGia{err: loiKho} },
		"đọc quyền hỏng":  func(d *Deps) { d.Quyen = quyenGia{err: loiKho} },
	}

	for ten, sua := range ca {
		t.Run(ten, func(t *testing.T) {
			s, nhatKy := may(t, sua)
			ra, err := s.ResolveStaffPrincipal(ctxXa(xaA),
				&identityv1.ResolveStaffPrincipalRequest{SessionToken: tok})
			if status.Code(err) != codes.Internal {
				t.Fatalf("mã = %v, muốn Internal (trả về: %+v, lỗi: %v)", status.Code(err), ra, err)
			}
			// The cause stays on THIS side of the boundary.
			if strings.Contains(status.Convert(err).Message(), loiKho.Error()) {
				t.Errorf("nguyên nhân nội bộ lọt sang bên gọi: %q", status.Convert(err).Message())
			}
			if !strings.Contains(nhatKy.String(), loiKho.Error()) {
				t.Errorf("nguyên nhân không được ghi ở phía máy chủ, nơi có người trực:\n%s", nhatKy.String())
			}
		})
	}
}

// A grant-read failure must never become a principal with an empty key list: empty is a REAL
// answer, and the caller cannot tell the two apart.
func TestResolveLoiDocQuyenKhongBaoGioThanhPrincipalRongQuyen(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.Quyen = quyenGia{err: loiKho} })

	ra, err := s.ResolveStaffPrincipal(ctxXa(xaA),
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: tokenCua(t, xaA, "sid-gia")})
	if err == nil {
		t.Fatalf("đọc quyền hỏng mà vẫn trả principal: %+v", ra.GetPrincipal())
	}
	if ra.GetPrincipal() != nil {
		t.Fatalf("trả cả lỗi lẫn principal: %+v", ra.GetPrincipal())
	}
}

// The other half of the same statement: a live session held by somebody with no grant at all IS a
// principal, with an empty list. Merging it into "no principal" would sign that person out
// instead of telling them they may not do this one thing.
func TestResolveKhongCoQuyenNaoVanLaMotPrincipal(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.Quyen = quyenGia{quyen: nil} })

	ra, err := s.ResolveStaffPrincipal(ctxXa(xaA),
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: tokenCua(t, xaA, "sid-gia")})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if ra.GetPrincipal() == nil {
		t.Fatal("người không có quyền nào bị trả về như phiên không hợp lệ")
	}
	if n := len(ra.GetPrincipal().GetPermissionKeys()); n != 0 {
		t.Errorf("số khoá quyền = %d, muốn 0", n)
	}
}

func TestResolveThanhCong(t *testing.T) {
	ph := &phienGia{p: idstore.Phien{ID: "sid-gia", NguoiDungID: idCanBo,
		HetHanLuc: time.Now().Add(time.Hour)}}
	s, _ := may(t, func(d *Deps) { d.Phien = ph })

	ra, err := s.ResolveStaffPrincipal(ctxXa(xaA),
		&identityv1.ResolveStaffPrincipalRequest{SessionToken: tokenCua(t, xaA, "sid-gia")})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	p := ra.GetPrincipal()
	if p == nil {
		t.Fatal("không có principal")
	}
	// THE INTERNAL id, never the business code "CB001": store.truyVanQuyenGoc matches `nd.id`, so
	// a principal carrying `ma` matches no row and every guarded route in four services answers
	// 403 with nothing to point at.
	if p.GetStaffId() != idCanBo {
		t.Errorf("staff_id = %q, muốn id nội bộ %q", p.GetStaffId(), idCanBo)
	}
	if got := strings.Join(p.GetPermissionKeys(), ","); got != "admin.user,task.extend" {
		t.Errorf("permission_keys = %q", got)
	}
	if !ph.daGhiNho {
		t.Error("không đóng dấu dùng gần nhất — cột chẩn đoán này im lặng chết đi")
	}
}

// ---------------------------------------------------------------- BatchGetStaff

func TestBatchVuotTranLaInvalidArgumentChuKhongCatBot(t *testing.T) {
	lo := &loGia{}
	s, _ := may(t, func(d *Deps) { d.Lo = lo })

	ids := make([]string, TranIDMotLo+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}

	_, err := s.BatchGetStaff(ctxXa(xaA), &identityv1.BatchGetStaffRequest{Ids: ids})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
	// Refused BEFORE the query: an unbounded id list from the caller must never become an
	// unbounded query on the receiver.
	if lo.soLanGoi != 0 {
		t.Errorf("đã chạy truy vấn %d lần dù đã vượt trần", lo.soLanGoi)
	}
}

func TestBatchDungTranThiVanPhucVu(t *testing.T) {
	s, _ := may(t, nil)

	ids := make([]string, TranIDMotLo)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}
	if _, err := s.BatchGetStaff(ctxXa(xaA), &identityv1.BatchGetStaffRequest{Ids: ids}); err != nil {
		t.Fatalf("đúng trần %d vẫn bị từ chối: %v", TranIDMotLo, err)
	}
}

func TestBatchGopTrungVaBoRong(t *testing.T) {
	lo := &loGia{}
	s, _ := may(t, func(d *Deps) { d.Lo = lo })

	_, err := s.BatchGetStaff(ctxXa(xaA),
		&identityv1.BatchGetStaffRequest{Ids: []string{"a", "b", "a", "", "b", "c"}})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if got := strings.Join(lo.daNhan, ","); got != "a,b,c" {
		t.Errorf("kho nhận %q, muốn \"a,b,c\" — trùng phải gộp, rỗng phải bỏ", got)
	}
}

// The mapping, field by field. tenant_id comes from the CONTEXT, never from a column: the row is
// in this commune by construction, and a second source for one fact on the isolation path is
// where a disagreement gets resolved by guessing.
func TestBatchAnhXaSangWire(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.Lo = &loGia{hang: []domain.CanBoVaiTro{
			{ID: "id-1", VaiTroMa: "chu-tich-ubnd"},
			{ID: "id-2"}, // holds no role — an ordinary answer, not an error
		}}
	})

	ra, err := s.BatchGetStaff(ctxXa(xaA),
		&identityv1.BatchGetStaffRequest{Ids: []string{"id-1", "id-2", "id-3"}})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}

	// FEWER ITEMS THAN IDS IS VALID: id-3 is absent, and the three reasons it could be absent are
	// deliberately indistinguishable.
	if len(ra.GetItems()) != 2 {
		t.Fatalf("số dòng = %d, muốn 2", len(ra.GetItems()))
	}
	for _, it := range ra.GetItems() {
		if it.GetTenantId() != string(xaA) {
			t.Errorf("%s: tenant_id = %q, muốn xã trong context", it.GetId(), it.GetTenantId())
		}
	}
	if got := ra.GetItems()[0].GetRoles(); len(got) != 1 || got[0] != "chu-tich-ubnd" {
		t.Errorf("roles của id-1 = %v, muốn slug vai trò", got)
	}
	// Empty, not a one-element list holding "": a consumer filtering on role would otherwise
	// count this person as holding one.
	if got := ra.GetItems()[1].GetRoles(); len(got) != 0 {
		t.Errorf("roles của người không có vai trò = %v, muốn rỗng", got)
	}
}

func TestBatchLoiKhoLaInternalVaKhongLoNguyenNhan(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.Lo = &loGia{err: loiKho} })

	_, err := s.BatchGetStaff(ctxXa(xaA), &identityv1.BatchGetStaffRequest{Ids: []string{"a"}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if strings.Contains(status.Convert(err).Message(), loiKho.Error()) {
		t.Errorf("nguyên nhân nội bộ lọt sang bên gọi: %q", status.Convert(err).Message())
	}
}

func TestBatchKhongCoXaLaInternalChuKhongPanic(t *testing.T) {
	lo := &loGia{}
	s, _ := may(t, func(d *Deps) { d.Lo = lo })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()

	_, err := s.BatchGetStaff(context.Background(), &identityv1.BatchGetStaffRequest{Ids: []string{"a"}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
	if lo.soLanGoi != 0 {
		t.Errorf("đã đọc kho %d lần mà không có xã — đó là một truy vấn không phạm vi", lo.soLanGoi)
	}
}

// ---------------------------------------------------------------- construction

// ListCitizenCommunes is declared in the contract and deliberately not written. The embedded
// UnimplementedIdentityServiceServer is what answers it, and that is the seam: a stub returning
// an empty list would be something a caller ships against.
//
// TWO INDEPENDENT BLOCKERS, both named on the method's doc in server.go: it cannot carry a
// commune and is not on grpcx.methodsWithoutTenant (adding it is a stop condition of ADR 0012,
// decision 1), and internal/store/crosstenant/ has no read of `quan_he_cong_dan_xa` at all.
func TestListCitizenCommunesVanChuaCaiDat(t *testing.T) {
	s, _ := may(t, nil)

	_, err := s.ListCitizenCommunes(ctxXa(xaA), &identityv1.ListCitizenCommunesRequest{CitizenId: "x"})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("mã = %v, muốn Unimplemented — một stub trả danh sách rỗng là thứ bên gọi sẽ tin",
			status.Code(err))
	}
}

// Incomplete wiring must fail where a human is watching a process fail to start, never at request
// time in four other services.
func TestNewServerTuChoiNoiDayKhongDu(t *testing.T) {
	ky, err := token.NewSigner([]secret.Secret{khoaKyGia})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	du := func() Deps {
		return Deps{
			Signer: ky,
			Phien:  &phienGia{},
			CanBo:  canBoGia{},
			Lo:     &loGia{},
			Quyen:  quyenGia{},
			Lich:   &lichGia{},
			NghiLe: &nghiLeGia{},
			LamBu:  &lamBuGia{},
			Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
	}

	ca := map[string]func(*Deps){
		"thiếu signer":        func(d *Deps) { d.Signer = nil },
		"thiếu phiên":         func(d *Deps) { d.Phien = nil },
		"thiếu cán bộ":        func(d *Deps) { d.CanBo = nil },
		"thiếu lô":            func(d *Deps) { d.Lo = nil },
		"thiếu quyền":         func(d *Deps) { d.Quyen = nil },
		"thiếu lịch làm việc": func(d *Deps) { d.Lich = nil },
		// A calendar read without its closures counts a deadline THROUGH a day the office was
		// shut; without its swap days it counts a day the office WAS open as closed. Neither is a
		// shorter answer — both are wrong, in opposite directions, with nothing on any screen to
		// show it.
		"thiếu ngày nghỉ lễ": func(d *Deps) { d.NghiLe = nil },
		"thiếu ngày làm bù":  func(d *Deps) { d.LamBu = nil },
	}
	for ten, sua := range ca {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("dựng được máy chủ với nối dây thiếu")
				}
			}()
			d := du()
			sua(&d)
			_ = NewServer(d)
		})
	}

	// And a complete Deps must still build, or the test above would pass on a constructor that
	// refuses everything.
	if NewServer(du()) == nil {
		t.Fatal("nối dây đủ mà vẫn không dựng được")
	}
}
