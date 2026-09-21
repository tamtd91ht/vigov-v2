// Package grpc serves the identity contract to the other services.
//
// WHY IT EXISTS AT ALL. `XacThuc` (service-identity/internal/http/middleware.go) is the only
// thing in this system that builds an authz.Principal, and it sits inside this service's
// `internal/`, which rule 2, forbidden #1 forbids any other service from importing. Until this
// server answered, a guarded route in service-documents, -finance, -comms or -petitions replied
// 401 to EVERY caller — including a member of staff holding a perfectly valid session. Six
// catalogue routes shipped in that state.
//
// # What this server is deliberately incapable of
//
//	IT RETURNS NO PERSONAL DATA. Not a name, not a phone number, not an email, not a national id.
//	`message Staff` and `message StaffPrincipal` declare none of them, and the read paths behind
//	this file (domain.CanBoVaiTro, store.TheoNhieuID) do not even LOAD them. Having nothing in
//	hand is a stronger guarantee than remembering not to send it — and the day somebody widens
//	the wire type, the read path still has nothing to put in the new field, so the change cannot
//	pass unnoticed. Widening toward a name is BLOCKED anyway: open question #11 (is a staff
//	member's mobile number masked from other staff) decides what "staff personal data" may cross
//	a boundary at all, and the customer has not answered it.
//
//	IT MINTS NOTHING. ResolveStaffPrincipal READS a credential the caller relayed; it never
//	issues one. There is no path here by which a caller obtains a session, extends one, or names
//	a staff member it has no credential for.
//
//	IT HAS NO CROSS-COMMUNE READ. Every query below goes through store.Scoped, whose commune
//	comes from the context that core/grpcx.UnaryServerInterceptor filled from metadata. There is
//	no `// @cross-tenant:` in this package, and adding one is rule 1, forbidden #6 — it needs a
//	written reason, not a helper function.
//
// # Nothing here is audited, and that is a decision rather than an omission
//
// Rule 6, invariant 1 covers WRITES to business data. All four RPCs here are reads —
// AdvanceWorkingHours reads a commune's configuration and writes nothing at all — and two of them
// run on EVERY request, one per staff request of four services and one per citizen request of the
// Mini App (ResolveCitizenSession, phien_cong_dan.go). An audit entry per session resolution would
// add rows at the rate of page loads — recording nothing anybody would ever look for, and
// burying the entries that carry legal weight.
//
// THREE THINGS WOULD CHANGE THAT ANSWER, and each is a stop condition rather than a judgement
// call for whoever hits it:
//
//  1. the first RPC here that WRITES anything;
//  2. an RPC that returns UNMASKED personal data, or reads across communes — rule 6, invariant 7
//     audits both explicitly, regardless of them being reads;
//  3. attribution becoming possible. Today it is NOT: ADR 0025's one shared caller key proves a
//     call came from inside the deployment and never WHICH service made it, so an audit entry
//     written here could not name a "who" (rule 6, invariant 2). Per-service identity — mTLS or
//     a mesh — is what unblocks that, and it is still owed.
//
// # The commune arrives the same way it does at the HTTP edge
//
// core/grpcx.UnaryServerInterceptor lifts "x-tenant-id" out of metadata into context.Context
// before any handler runs, so every handler below reads it exactly as an HTTP handler does, and
// none of them touches metadata itself. THREE OF THE FOUR RPCs here must always carry a commune and
// must never be exempted: each is called from a service edge that has ALREADY resolved its commune
// from Host, so carrying it costs the caller nothing, and for AdvanceWorkingHours the exemption is
// not even expressible — "x-tenant-id" names WHOSE calendar is read.
//
// THE FOURTH IS THE EXCEPTION AND IT IS NOT YET GRANTED. ResolveCitizenSession CANNOT carry a
// commune, because it is the call that establishes one for the citizen channel (ADR 0022) — the
// same loop ADR 0012 §A describes for ResolveHost. It therefore belongs on
// grpcx.methodsWithoutTenant and is NOT on it: adding a second name to that list is a STOP
// CONDITION for the user (ADR 0012, decision 1). Until that is answered the interceptor refuses the
// call with InvalidArgument and phien_cong_dan.go is never entered. Do not read the sentence above
// as covering it, and do not lift the exemption while writing code.
//
// ONE CONSEQUENCE FOR READERS OF THIS FILE: Server.xa must NOT be called from that handler. Every
// other handler starts with it; that one runs with no commune in context, deliberately.
package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// TranIDMotLo is the ceiling on BatchGetStaffRequest.ids, declared where it is enforced.
//
// Exceeding it is INVALID_ARGUMENT and never a silent truncation — ADR 0012, decision 2 spells
// out why this deliberately differs from pagination, where clamping is safe: a clamped page
// leaves a cursor to carry on from, a clamped batch leaves real rows rendered blank with nothing
// to report it.
const TranIDMotLo = 200

// The collaborators, declared HERE at the point of use and not imported as concrete stores, so
// every branch below is testable without a PostgreSQL. *idstore.CanBoStore, *idstore.PhienStore
// and *idstore.Checker satisfy these as they are; nothing was changed to accommodate them.
//
// FOUR NARROW INTERFACES AND NOT ONE WIDE ONE, mirroring internal/http's split for the same
// reason: the two staff reads have DIFFERENT predicates on purpose, and one interface carrying
// both would put the register's looser filter one careless edit away from the authentication
// path.
type (
	// PhienDoc is the session registry — the thing that makes a signed token revocable.
	PhienDoc interface {
		KiemTra(ctx context.Context, sid string) (idstore.Phien, error)
		GhiNhanDung(ctx context.Context, sid string)
	}

	// CanBoDoc rebuilds the staff account behind a session. Its query excludes soft-deleted
	// accounts, accounts that do not exist as sign-ins, and locked accounts — which is what makes
	// a person locked out mid-session stop being a principal on the very next request.
	CanBoDoc interface {
		TheoID(ctx context.Context, id string) (domain.CanBo, error)
	}

	// CanBoLo is the register read behind BatchGetStaff. SEPARATE FROM CanBoDoc, and see
	// internal/store/can_bo_lo.go for why the two predicates must not be merged.
	CanBoLo interface {
		TheoNhieuID(ctx context.Context, ids []string) ([]domain.CanBoVaiTro, error)
	}

	// QuyenDoc lists every permission key one staff member holds in this commune.
	//
	// IT IS NOT authz.Checker, although the same *idstore.Checker implements both and the same
	// value is wired into this field. Checker DECIDES one key at a time and guards routes;
	// QuyenCua DESCRIBES the grant set. The distinction internal/http/routes.go argues at length
	// is not collapsed by sending the set across this boundary — see the obligation on the caller
	// stated on StaffPrincipal.permission_keys, and the note on ResolveStaffPrincipal below.
	QuyenDoc interface {
		QuyenCua(ctx context.Context, p authz.Principal) ([]authz.Perm, error)
	}
)

// Deps is what the server cannot work without. Every field is required; Server refuses to be
// built with any of them missing — see NewServer.
type Deps struct {
	// Signer verifies incoming session tokens. IT MUST BE THE SAME *token.Signer the HTTP side
	// and app.DangNhap were given: two signers built from two calls would mean a token issued at
	// sign-in cannot be read here, and four services would answer 401 to a session identity's own
	// routes accept.
	Signer *token.Signer

	Phien PhienDoc
	CanBo CanBoDoc
	Lo    CanBoLo
	Quyen QuyenDoc

	// The CITIZEN session registry, read by ResolveCitizenSession and by nothing else here.
	//
	// A SEPARATE FIELD FROM Phien, AND THE TWO MUST NEVER BE MERGED — the same discipline
	// internal/store keeps between `phien` and `phien_cong_dan`. `phien` has a foreign key to an
	// accountable staff account; this one is behind a phone number and an OTP, which rule 4 calls
	// deliberately weak. One field would put two trust levels behind one lookup, and the weaker
	// one would decide the shape of it. Declared in phien_cong_dan.go, at the point of use.
	PhienCongDan PhienCongDanDoc

	// The commune's working calendar, read by AdvanceWorkingHours and by nothing else here. The
	// three interfaces are declared in lich_lam_viec.go, at the point of use — three tables, three
	// reads, three different windows.
	//
	// ALL THREE OR NONE: a deadline computed from the week without its holidays, or with its
	// holidays but without its swap days, is not a shorter answer — it is a WRONG one, and wrong
	// in a direction nothing on any screen shows. Migration 0006 puts the three tables in one
	// transaction for the same reason.
	Lich   LichLamViecDoc
	NghiLe NgayNghiLeDoc
	LamBu  NgayLamBuDoc

	Log *slog.Logger
}

// Server implements identityv1.IdentityServiceServer.
//
// THE EMBEDDED UnimplementedIdentityServiceServer IS LOAD-BEARING, not boilerplate: it is what
// makes an RPC declared in the contract but not written here answer codes.Unimplemented instead
// of failing to compile. That is the seam a contract gains RPCs through — a stub written to
// "fill the gap" would answer something, and something is what a caller ships against.
type Server struct {
	identityv1.UnimplementedIdentityServiceServer

	d Deps
}

// NewServer refuses incomplete wiring AT CONSTRUCTION, where a human is watching a process fail
// to start — never at request time, where the failure is a nil dereference in a handler that
// four other services are already calling. Same discipline as identity/http.Register and
// grpcx.UnaryServerCallerAuth.
//
// Each panic names the RPC that would have failed, because "missing dependency" sends the reader
// to the wrong file.
func NewServer(d Deps) *Server {
	switch {
	case d.Signer == nil:
		panic("identity/grpc: thiếu token.Signer — ResolveStaffPrincipal không giải được token phiên")
	case d.Phien == nil:
		panic("identity/grpc: thiếu kho phiên — ResolveStaffPrincipal không kiểm được phiên đã thu hồi chưa")
	case d.CanBo == nil:
		panic("identity/grpc: thiếu kho cán bộ — ResolveStaffPrincipal không dựng được principal")
	case d.Lo == nil:
		panic("identity/grpc: thiếu kho đọc cán bộ theo lô — BatchGetStaff sẽ panic khi có người gọi")
	case d.Quyen == nil:
		panic("identity/grpc: thiếu kho quyền — ResolveStaffPrincipal sẽ trả principal rỗng quyền, không phân biệt được với người thật sự không có quyền")
	case d.PhienCongDan == nil:
		panic("identity/grpc: thiếu sổ phiên công dân — ResolveCitizenSession sẽ panic, và kênh công dân của mọi service khác không dựng được rìa")
	case d.Lich == nil:
		panic("identity/grpc: thiếu kho lịch làm việc — AdvanceWorkingHours sẽ panic khi có người gọi")
	case d.NghiLe == nil:
		panic("identity/grpc: thiếu kho ngày nghỉ lễ — AdvanceWorkingHours sẽ tính hạn xuyên qua ngày cơ quan đóng cửa")
	case d.LamBu == nil:
		panic("identity/grpc: thiếu kho ngày làm bù — AdvanceWorkingHours sẽ tính hạn như thể xã nghỉ đúng những ngày nó có làm")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}

	// THE ZONE IS RESOLVED AT CONSTRUCTION, where a human is watching a process fail to start —
	// never at the first intake, where the failure lands on a citizen's deadline. A binary whose
	// image lost its zone database cannot combine a commune's wall-clock calendar with a date, so
	// it cannot answer this RPC at all; starting anyway would mean discovering it from a
	// FAILED_PRECONDITION on the first petition received.
	if _, err := domain.MuiGio(); err != nil {
		panic("identity/grpc: " + err.Error() +
			" — ảnh chạy thiếu cơ sở dữ liệu múi giờ, AdvanceWorkingHours không tính được hạn")
	}
	return &Server{d: d}
}

// ResolveStaffPrincipal turns the session credential a staff member's browser holds into the
// principal the calling service's guards need.
//
// # The order of the steps IS the security property
//
// Token decode → COMMUNE COMPARISON → session registry → account → grants. The comparison sits
// where it does, before any database read, for a reason that is not performance: looking a
// commune-A sid up while the request names commune B is a query in one commune for a value
// issued in another. Doing the comparison first means there is no such query to justify, no
// `// @cross-tenant:` to write, and no path by which one commune's request touches another
// commune's rows. This mirrors XacThuc steps 2-5 exactly, and reordering it is the single
// cheapest way to put a hole here.
//
// # Every unusable credential gets ONE answer
//
// Unknown, malformed, expired, revoked, locked, soft-deleted, wrong commune: OK with no
// principal. No reason code, no enum, and deliberately no way to add one without editing this
// paragraph — a response that varies with the reason tells whoever is probing how close they
// are, the same discipline rule 4, forbidden #2 imposes on 404 versus 403.
//
// # Where this DIVERGES from XacThuc, and why it is not a lapse
//
// XacThuc collapses every failure of the registry read and the account read into "no principal",
// because it runs inside identity, where a database outage surfaces on every other route at the
// same moment. ACROSS THIS BOUNDARY THAT COLLAPSE IS WRONG, and the contract's status-code table
// says so: "UNAVAILABLE etc. — the call did not happen. IT IS NOT 'no principal'". A caller that
// reads an outage as anonymity turns "identity is down" into "everybody is signed out", and
// staff are then told to sign in again through the service that is down. So a store failure is
// Internal here, and only the two sentinel "this session is not usable" errors are no-principal.
//
// # No cache, and the argument is already written down
//
// service-identity/internal/store/phien_cong_dan.go states it for the citizen path and it holds
// identically here: ADR 0010 fixes the data infrastructure at PostgreSQL only, so there is no
// shared cache; identity runs several replicas, so any cache is PER PROCESS; and a revocation
// served by one replica has no channel on which to reach the copy held by another. A TTL cache —
// here or in the four callers — would keep a revoked session, a locked account or a withdrawn
// role working for the length of that TTL on every process that did not serve the revocation.
// Rule 5, invariant 4 exists precisely so that cannot happen, and nothing would turn red.
//
// THE OBLIGATION THAT TRAVELS WITH THE RESPONSE: permission_keys is scoped to the single request
// it authenticated. Never written into a token, never attached to a session, never cached across
// requests, never written to disk.
func (s *Server) ResolveStaffPrincipal(ctx context.Context, req *identityv1.ResolveStaffPrincipalRequest) (
	*identityv1.ResolveStaffPrincipalResponse, error) {

	// NEVER LOG req. The generated String() prints session_token in full, and `debug_redact` is a
	// no-op in protobuf-go v1.36.12 — measured, not assumed. Nothing in the generated code
	// protects this field; the protection is this sentence and the fields chosen by hand below.

	tok := req.GetSessionToken()
	if tok == "" {
		// LOUD, not "no principal". A caller with no credential must not call at all; answering
		// quietly would cost a round trip on every anonymous request, forever, with nothing to
		// report the wiring fault.
		return nil, status.Error(codes.InvalidArgument, "session_token trống")
	}

	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Decode. An unreadable or expired token is not usable — and nothing is logged, because a
	// stale cookie is an ordinary event and the only thing there would be to log is the
	// credential itself.
	claims, err := s.d.Signer.Giai(tok)
	if err != nil {
		return khongCoPrincipal(), nil
	}

	// 2. COMMUNE COMPARISON — HERE, BEFORE ANY DATABASE READ. See the note above.
	//
	// A browser does not send a cookie across hosts, so a mismatch is never a user mistake: it is
	// a deliberate probe or a stolen token, and it is a SECURITY SIGNAL rather than a 4xx to
	// count on a dashboard (skills/session-and-token §2). This is the ONE place that comparison
	// is made for all four calling services; four services each remembering to make it is four
	// places for one of them to forget, and the one that forgets serves one commune's staff
	// inside another commune's data (rule 1, invariant 8).
	if claims.TenantID != xa {
		s.d.Log.WarnContext(ctx, "CẢNH BÁO AN NINH: token của xã khác gửi tới xã này",
			"rpc", "ResolveStaffPrincipal",
			"xa_trong_token", string(claims.TenantID),
			"xa_theo_metadata", string(xa),
			// The address the BROWSER connected from, as the CALLING service observed it. "" is
			// valid and means "not observed" — the line is logged without it rather than the call
			// being refused, because failing authentication over a missing diagnostic field
			// trades a staff member's access for a log line. IT IS A CLAIM AND DECIDES NOTHING:
			// no rate limit, no block, and never the audit trail's "from which IP" (rule 6,
			// invariant 2 wants an address the writer observed itself).
			"ip_trinh_duyet", req.GetClientIp(),
			// A fingerprint, never the sid and never the token. An alert carrying a working
			// credential is a second copy of that credential, in the log pipeline, from where it
			// cannot be recalled (rule 8).
			"sid_van_tay", vanTay(claims.Sid))
		return khongCoPrincipal(), nil
	}

	// 3. Session registry: revoked, expired or unknown sid is not usable. Anything else is an
	// outage and must not be reported as anonymity.
	ph, err := s.d.Phien.KiemTra(ctx, claims.Sid)
	if err != nil {
		if errors.Is(err, idstore.ErrPhienKhongTonTai) || errors.Is(err, idstore.ErrPhienHetHan) {
			return khongCoPrincipal(), nil
		}
		return nil, s.loi(ctx, err, "ResolveStaffPrincipal/phien")
	}

	// 4. The account. TheoID excludes soft-deleted, account-less and LOCKED rows, which is what
	// makes locking an account take effect on the very next request rather than when the session
	// happens to expire. Reaching for a looser read here would silently undo that.
	cb, err := s.d.CanBo.TheoID(ctx, ph.NguoiDungID)
	if err != nil {
		if errors.Is(err, idstore.ErrCanBoKhongTonTai) {
			return khongCoPrincipal(), nil
		}
		return nil, s.loi(ctx, err, "ResolveStaffPrincipal/can_bo")
	}

	// 5. The grants, read live, through the SAME predicate the guard path uses
	// (store.truyVanQuyenGoc). A second predicate written here would drift from it, and drift in
	// either direction is a defect with no error attached: looser and the web draws what the
	// server refuses; stricter and work is hidden — a missing button asks no questions.
	//
	// cb.ID AND NOT cb.Ma. The query matches `nd.id`; the business code matches no row, so every
	// permission answers false and every guarded route in four services returns 403 with nothing
	// in the response, the logs or a test to point at the cause.
	quyen, err := s.d.Quyen.QuyenCua(ctx, authz.Principal{ID: cb.ID, Kind: "staff", TenantID: xa})
	if err != nil {
		// A GRANT-READ FAILURE FAILS THE RPC. It must never become a principal with an empty key
		// list: empty is a REAL answer — a live session held by somebody who currently holds
		// nothing — and the caller cannot tell the two apart. Answering empty would silently
		// downgrade every one of that person's permissions to "no" while telling them they are
		// signed in.
		return nil, s.loi(ctx, err, "ResolveStaffPrincipal/quyen")
	}

	// 6. Last-seen stamp. Outside any transaction and its failure ignored, exactly as XacThuc
	// does it: it is a diagnostic column, and failing a real authentication because a timestamp
	// could not be written trades an operation for a nicety.
	s.d.Phien.GhiNhanDung(ctx, claims.Sid)

	return &identityv1.ResolveStaffPrincipalResponse{
		Principal: &identityv1.StaffPrincipal{
			StaffId:        cb.ID,
			PermissionKeys: sangKhoaQuyen(quyen),
		},
	}, nil
}

// BatchGetStaff resolves staff by id, within the commune the metadata names.
//
// ⚠ THE LIMIT OF THIS RPC, STATED RATHER THAN FIXED. Its own contract comment says the caller is
// "always a list screen decorating rows with A HANDLER'S NAME" — AND `message Staff` CARRIES NO
// NAME. It declares `id`, `tenant_id` and `roles`, and field 3 is permanently `reserved`. So this
// RPC cannot serve the purpose it names: a list screen calling it learns which of its ids exist
// in this commune and which role each holds, and still has nothing to render in the column it
// opened the call for.
//
// IT IS NOT FIXED HERE, AND THE BLOCKER IS NOT TECHNICAL. Adding `ho_ten` means deciding what a
// staff member's personal data may do across a service boundary, and that is OPEN QUESTION #11 —
// "is a staff member's mobile number masked from other staff, and if so which permission key
// opens it" — which the customer has not answered and which cannot be answered here, because
// there is no key in the 33 seeded into `quyen` that means "view full staff detail". Choosing
// one would be designing the customer's authorisation model. Widening the message is also
// contract-designer's to do, not this service's (rule 2, invariant 7).
//
// So: no field is added, no workaround is built, and no second route is opened to fetch the name
// out of band — an out-of-band route would answer the open question by building the thing it is
// about. The next person stands exactly here.
//
// WHAT IT IS USABLE FOR TODAY: existence and role within one commune. Everything below is
// correct for that, and stays correct when the message is widened.
func (s *Server) BatchGetStaff(ctx context.Context, req *identityv1.BatchGetStaffRequest) (
	*identityv1.BatchGetStaffResponse, error) {

	ids := req.GetIds()

	// THE CEILING IS CHECKED ON WHAT THE CALLER SENT, before duplicates are collapsed: the point
	// is to bound the request, and a caller that sends 5.000 ids of which 40 are distinct is
	// still a caller with no ceiling of its own. Refusing is the contract — never a truncation,
	// because a truncated batch makes real rows render blank and nothing reports it.
	if len(ids) > TranIDMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ids vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", TranIDMotLo)
	}

	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	hang, err := s.d.Lo.TheoNhieuID(ctx, locID(ids))
	if err != nil {
		return nil, s.loi(ctx, err, "BatchGetStaff")
	}

	// FEWER ITEMS THAN IDS IS A VALID RESPONSE and the order is not the order of `ids`. An id is
	// absent when it does not exist, is soft deleted, or belongs to another commune — the three
	// are deliberately indistinguishable, because telling them apart answers "does this record
	// exist in a commune you may not read" (ADR 0012, decision 2).
	ra := make([]*identityv1.Staff, 0, len(hang))
	for _, cb := range hang {
		ra = append(ra, sangProtoCanBo(cb, xa))
	}
	return &identityv1.BatchGetStaffResponse{Items: ra}, nil
}

// ListCitizenCommunes IS DELIBERATELY NOT IMPLEMENTED, and the embedded
// UnimplementedIdentityServiceServer is what answers it: codes.Unimplemented, loudly, rather
// than a stub returning an empty list that a caller would ship against.
//
// TWO INDEPENDENT BLOCKERS. Removing either one alone changes nothing:
//
//  1. IT CANNOT CARRY A COMMUNE, AND IT IS NOT EXEMPT FROM CARRYING ONE. The RPC answers about a
//     citizen across EVERY commune they deal with (ADR 0002: one phone number is one record for
//     the whole platform), so there is no single commune to put in "x-tenant-id" — and it is
//     absent from core/grpcx.methodsWithoutTenant, so today the interceptor refuses it with
//     InvalidArgument before this file is reached. ADDING A NAME TO THAT LIST IS A STOP
//     CONDITION in its own right (ADR 0012, decision 1), and the user has not been asked. The
//     caller-key interceptor arriving (ADR 0025) did not lift it: one condition being met does
//     not lift a stop condition that was never conditional on it.
//
//  2. THE STORE SIDE DOES NOT EXIST. service-identity/internal/store/crosstenant/ holds exactly
//     one query — identity by phone — and no read of `quan_he_cong_dan_xa`. That package's own
//     doc states what may be added to it and requires a written ADR decision first; a read that
//     returns rows of more than one commune is precisely what it names as never addable while
//     open question #4 (how far up the administrative ladder aggregate reading goes) is open.

// xa reports the commune this call is for.
//
// IT IS NOT tenant.MustFrom. MustFrom panics, which is right inside an HTTP handler where
// httpx.Recover turns it into a traceable 500 — there is NO equivalent recovery on a gRPC
// handler, so a panic here takes down a process serving 200+ communes because one call arrived
// misrouted.
//
// REACHING THIS BRANCH MEANS core/grpcx.UnaryServerInterceptor IS NOT IN THE CHAIN. No RPC that
// CALLS THIS FUNCTION is exempt from carrying a commune, so with the interceptor installed the call
// would already have been refused with InvalidArgument. Internal is therefore the honest code:
// the fault is this deployment's wiring, not the caller's request. It also closes the panic path
// through store.Scoped, which calls MustFrom itself.
//
// ResolveCitizenSession DOES NOT CALL THIS, and must not: it is the call that establishes the
// commune, so it runs with none in context by design (phien_cong_dan.go). Calling this from there
// would turn its every success into Internal.
func (s *Server) xa(ctx context.Context) (tenant.ID, error) {
	id, ok := tenant.From(ctx)
	if !ok {
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO: handler gRPC chạy mà không có xã trong context — "+
			"grpcx.UnaryServerInterceptor không nằm trong chuỗi", "err", tenant.ErrNoTenant)
		return "", status.Error(codes.Internal, "lỗi cấu hình máy chủ")
	}
	return id, nil
}

// khongCoPrincipal is the ONE answer every unusable credential receives.
//
// A named function and not an inline literal, so the several branches that produce it are
// provably the same answer: an absent principal with nothing beside it. The moment one branch
// grows a field the others do not have, the response starts varying with the reason.
func khongCoPrincipal() *identityv1.ResolveStaffPrincipalResponse {
	return &identityv1.ResolveStaffPrincipalResponse{}
}

// loi maps a store failure to a gRPC code.
//
// THE MESSAGE RETURNED TO THE CALLER NEVER CARRIES THE CAUSE. An internal failure text can hold
// a DSN fragment, a query, or a value from a row, and this response crosses a service boundary
// into another process's logs. The cause goes to THIS service's log, where the operator is
// (rule 3, forbidden #3).
//
// Everything reaching here is an infrastructure failure: the two "this is not usable" sentinels
// are handled at their call sites, where the right answer is a response rather than an error.
func (s *Server) loi(ctx context.Context, err error, cho string) error {
	s.d.Log.ErrorContext(ctx, "identity/grpc: đọc thất bại", "cho", cho, "err", err)
	return status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
}

// locID drops empty ids and collapses duplicates.
//
// DUPLICATES ARE COLLAPSED BY THE SERVER, as the contract states — a caller building ids from a
// list of rows naturally repeats the same handler. Empty strings are dropped because "" matches
// no ULID and only widens the array the database has to scan.
//
// Order is not preserved and does not matter: the response's order carries no meaning either,
// and the caller maps by Staff.id.
func locID(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	thay := make(map[string]struct{}, len(ids))
	ra := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, co := thay[id]; co {
			continue
		}
		thay[id] = struct{}{}
		ra = append(ra, id)
	}
	return ra
}

// sangProtoCanBo copies the register view onto the wire type.
//
// FIELD BY FIELD, never by reflection or a generic mapper. This is the boundary the package doc
// describes, and a mapper that copies "whatever is on the struct" is how a personal-data field
// eventually leaves this service without anybody deciding that it should.
//
// TenantId COMES FROM THE CONTEXT, not from a column. It is an attribute of the stored record —
// which commune the row belongs to — and the row is in this commune by construction, because the
// query was scoped to it. Selecting the column as well would create a second source for one fact
// on the isolation path, and a second source is where a disagreement gets resolved by guessing.
//
// ROLES CARRIES THE SLUG (`vai_tro.ma`), NOT `vai_tro_id`. ASSUMPTION, STATED: the contract
// comment names the nullable column as the SOURCE of the list but does not say which of the
// role's two identifiers travels. The slug is chosen because it is the value every other surface
// of this system already uses across a boundary — GET /api/v1/sessions/current returns
// `role.code` = `vai_tro.ma`, "the stable slug a client keys on" — and because a ULID would be
// unresolvable by any consumer: no RPC on this contract turns a role id into anything. If
// contract-designer pins the other reading, this function and truyVanCanBoLo are the two places
// to change, and nothing else moves.
//
// The list is EMPTY, not a one-element list holding "", when the person holds no role. An empty
// string in a roles list is a role whose name is nothing, and a consumer filtering on it would
// count that person as holding one.
func sangProtoCanBo(cb domain.CanBoVaiTro, xa tenant.ID) *identityv1.Staff {
	st := &identityv1.Staff{
		Id:       cb.ID,
		TenantId: string(xa),
	}
	if cb.VaiTroMa != "" {
		st.Roles = []string{cb.VaiTroMa}
	}
	return st
}

// sangKhoaQuyen copies the grant set onto the wire.
//
// nil IN, EMPTY-NON-NIL OUT IS NOT A DISTINCTION PROTOBUF CARRIES — both travel as an absent
// repeated field, and the caller sees an empty slice. That is correct here: an empty key list is
// a real answer meaning "holds nothing right now". The case this must never represent is "could
// not find out", and that case never reaches this function — it fails the RPC at the call site.
func sangKhoaQuyen(quyen []authz.Perm) []string {
	if len(quyen) == 0 {
		return nil
	}
	ra := make([]string, 0, len(quyen))
	for _, q := range quyen {
		ra = append(ra, string(q))
	}
	return ra
}

// vanTay is a short, one-way fingerprint of a secret, so two log lines about one session can be
// correlated without the log holding anything replayable.
//
// A SECOND COPY OF identity/internal/http.vanTay, AND THE TWO MUST STAY IDENTICAL — same hash,
// same truncation. The alert raised here replaces the one XacThuc raises; an operator correlating
// the centralised alert with the four scattered ones compares these strings, and two functions
// that disagree produce two fingerprints for one sid, which reads as two different sessions.
//
// It is duplicated rather than shared because the original is unexported in another package and
// core/ is outside this change's scope. THAT IS A STATED DEBT, not a design: the right home is
// core/, next to the other things every service needs to do identically.
func vanTay(s string) string {
	tong := sha256.Sum256([]byte(s))
	return hex.EncodeToString(tong[:])[:12]
}
