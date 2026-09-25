package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// The citizen session registry — the table behind core/httpx.CitizenSessions.
//
// IT IS A SEPARATE REGISTRY FROM `phien`, AND THE TWO MUST NEVER MERGE. `phien` has a foreign
// key to `nguoi_dung`: a staff identity is ACCOUNTABLE. A citizen identity is a phone number,
// which rule 4 calls deliberately weak. One table would put two trust levels behind one lookup,
// and the weaker one would decide the shape of it.
//
// # THE SHORT-TTL CACHE ADR 0022 ASKS FOR IS NOT BUILT, AND THIS IS WHY
//
// ADR 0022 says this lookup "must be treated as a hot path from the start (short TTL cache,
// invalidated on revocation), not after it has become slow". It is not built here, and the
// reason is that the second half of that sentence cannot hold today:
//
//   - ADR 0010 fixes the data infrastructure at PostgreSQL only. There is no shared cache, so
//     any cache added here is PER PROCESS.
//   - service-identity is deployed with more than one replica. A revocation served by replica A
//     cannot invalidate the copy held by replica B; there is no channel to send the message on.
//   - The staleness window would therefore be the full TTL on every replica that did not serve
//     the revocation. A revoked citizen session would keep working for that long.
//
// WHAT THAT WINDOW WOULD COST, in the case that matters most: the "log this screen out" button
// of ADR 0019, step 6. That screen stands at a one-stop counter and the citizen has walked
// away from it. "Logged out, but it may keep working for a few more seconds on some of the
// servers" is not an answer a public authority can give, and it is not a property anybody would
// discover from a test.
//
// SO THE ORDER IS: measure first, then cache with a real invalidation channel. The measurement
// is written down already — the lookup probes one child index per partition, 32 of them, on a
// table that is empty today (see phien_cong_dan_theo_token in
// service-identity/migrations/0004_kenh_cong_dan.sql). A cache added before there is anything
// to measure, and before revocation can reach every replica, buys latency nobody has complained
// about with a correctness hole on the revocation path.
//
// This paragraph is the deliverable, not a TODO: the next person reads the reason, not a
// half-built cache whose invalidation looks complete until a second replica exists.
//
// # WHERE THE AUDIT ENTRY IS WRITTEN — read this before adding a method here
//
// Every mutating method takes the caller's transaction and runs inside it; the CALLER writes
// the audit entry in that same transaction (rule 6, invariant 3). This layer does not write the
// entry because it does not know the business fact: the same ThuHoi call is "công dân đăng
// xuất", "đổi xã" or "thu hồi màn hình dùng chung" depending on why the use case invoked it,
// and the trail has to answer what a person DID (rule 6, invariant 2), not which rows moved.
//
// TraCuu and GhiNhanDung write no business fact and are deliberately unaudited: auditing a
// session lookup would add one row per citizen request and bury the entries that matter.
type PhienCongDanStore struct {
	// THE RAW HANDLE, AND THE ONE IN THIS PACKAGE. core/store.For(ctx) panics without a commune
	// (core/store/scoped.go) and the lookup below runs BEFORE any commune is known, so there is
	// no scoped repository that could serve it. See TraCuu for the full exemption.
	raw *sql.DB
	log *slog.Logger
}

// NewPhienCongDanStore takes a raw *sql.DB, which no other constructor in this package does.
// The exemption is TraCuu's alone — see the comment on it before adding a second method that
// touches s.raw.
func NewPhienCongDanStore(db *sql.DB, log *slog.Logger) *PhienCongDanStore {
	if db == nil || log == nil {
		// Fail at wiring time. The alternative is a nil dereference on the first citizen
		// request of the day, in the one code path that runs before anything else.
		panic("store: NewPhienCongDanStore cần cả kết nối lẫn sổ ghi")
	}
	return &PhienCongDanStore{raw: db, log: log}
}

// The concrete type and the interface the citizen edge consumes cannot drift apart: a change to
// either signature stops compiling here rather than at the first citizen request.
var _ httpx.CitizenSessions = (*PhienCongDanStore)(nil)

// The two origins a citizen session may have. Mirrors CHECK phien_cong_dan_nguon_hop_le.
//
// A PAIRED SESSION IS NOT A SECOND KIND OF ENTITY. It is this entity with a different origin,
// and it is weaker because it lives on a device the citizen walks away from (ADR 0019).
const (
	NguonApp  = "app"
	NguonGhep = "ghep"
)

// xaChuaChon is the tenant_id of a session issued before the citizen has chosen a commune.
//
// IT IS A REAL ANSWER, NOT A MISSING VALUE, AND NOT A DEFAULT. ADR 0005 separates discovery
// from session: a citizen signs in first and the screen that offers the choice of commune is
// called WITH that session. Rule 1, forbidden #1 bans a default on the isolation path — so this
// value is never filled in for a caller who forgot to say which commune. It is written by
// exactly one method, whose name says that is what it is doing, and every business route
// refuses a session carrying it (core/httpx.XaTuPhien returns 401).
const xaChuaChon = ""

var (
	ErrThieuCongDan    = errors.New("phiên công dân: thiếu định danh công dân")
	ErrNguonKhongHopLe = errors.New("phiên công dân: nguồn phiên không hợp lệ")
	ErrThieuThoiHan    = errors.New("phiên công dân: thiếu thời hạn phiên")
	ErrThieuGiaoDich   = errors.New("phiên công dân: cần giao dịch của lời gọi để ghi vết đi cùng")
	ErrThieuPhien      = errors.New("phiên công dân: thiếu định danh phiên")
	ErrThieuLyDo       = errors.New("phiên công dân: thu hồi phải có lý do")
)

// cotPhienCongDan is the SELECT list, kept next to the one function that scans it.
//
// POSITIONAL, AND ALL THREE ARE STRINGS. Swapping `id` with `cong_dan_id` compiles, returns
// rows, and hands every request the wrong citizen — a silent, symmetric defect of exactly the
// shape this repository has already been bitten by. The list and quetPhien move together.
const cotPhienCongDan = `tenant_id, id, cong_dan_id`

// ONE `WHERE`, THREE CONDITIONS, ONE NEGATIVE ANSWER — and the filtering happens HERE rather
// than in Go, deliberately.
//
// Unknown token, expired session and revoked session must be indistinguishable from outside
// (core/httpx.CitizenSessions: "một câu trả lời cho mọi trường hợp"). Filtering in Go gives the
// three cases different amounts of work, and a difference in timing tells whoever is probing
// which one they hit — the repository has already been bitten by exactly that, on a query that
// distinguished an existing account from a missing one.
//
// `het_han_luc > now()` IS A COMPARISON, NEVER A STORED FLAG (rule 10, invariant 3). A boolean
// written by a nightly job is wrong the moment the job is late or the clock skews, and the
// stale value is the one that reaches the caller.
//
// NO deleted_at HERE: phien_cong_dan has no soft-delete columns. A session is ended by
// thu_hoi_luc, which this already excludes. Rule 7 applies to the tables that carry those
// columns, and this is not one of them.
const truyVanPhienTheoToken = `
SELECT ` + cotPhienCongDan + `
FROM phien_cong_dan
WHERE bam_token = $1
  AND thu_hoi_luc IS NULL
  AND het_han_luc > now()`

// TraCuu resolves a bearer token to the session the server issued for it. It implements
// core/httpx.CitizenSessions and runs on EVERY citizen request.
//
// # THIS LOOKUP IS UNSCOPED BY DESIGN, AND IT IS NOT A HOLE TO BE CLOSED
//
// The same shape as service-platform's Directory, for the same reason: this lookup is what
// ESTABLISHES the commune. The Mini App has no domain (ADR 0005), so the citizen edge has no
// Host to resolve and derives the commune FROM the session (ADR 0022). Scoping the query would
// mean knowing the commune in order to find the commune.
//
// THE WRONG FIX, NAMED SO NOBODY REACHES FOR IT: taking the commune from something the client
// sent — a query parameter, a header, a field in the body — to "make the query scoped". That is
// rule 1, forbidden #2 exactly, dressed as a stricter check. A client naming its own commune is
// a client granting itself access.
//
// WHAT KEEPS THE EXEMPTION SAFE: the key is 256 bits of randomness that the server minted, it
// is compared as a SHA-256 hash, and the query returns AT MOST ONE session — never a list. So
// there is no query here that could span communes.
//
// IT REFUSES WHEN IT FINDS TWO ROWS. The UNIQUE constraint on bam_token is composite with
// tenant_id, because a unique index on a hash-partitioned table must contain the partition key,
// so uniqueness is per commune and not global (see the migration). With 256-bit tokens a
// collision is not a practical concern — but "not practical" is not "impossible", and the one
// thing that must never happen is handing commune B's session to the holder of commune A's
// token. Two rows means the registry cannot say whose session this is, and the answer to that
// is no.
//
// WHAT IT DOES NOT ANSWER, STATED BECAUSE THE INTERFACE'S DOC SAYS OTHERWISE: core/httpx lists
// "phiên của một xã đã ngừng hoạt động" among the cases that must return ok=false. This
// registry cannot tell that. Whether a commune is still operating lives in the `tenant` table,
// which belongs to service-platform, and reading another service's database is rule 2,
// forbidden #2. Answering it needs a platformclient call on this path, and that is a decision
// with a cost (a second hop on every citizen request) that has not been made. Until it is, a
// merged commune's citizen sessions keep working — written down here rather than left for
// somebody to discover.
func (s *PhienCongDanStore) TraCuu(ctx context.Context, token string) (httpx.CitizenSession, bool) {
	p, ok, err := s.TraCuuCoLoi(ctx, token)
	if err != nil {
		// A database failure is not "no such session", but the interface has one negative
		// answer and that is right FOR THE EDGE, where identity's own routes are failing in the
		// same breath. It is logged — WITHOUT the token and without any citizen identifier —
		// because the alternative is an outage that looks like every citizen in the country
		// mistyping their session at once.
		//
		// THE COLLAPSE IS MADE HERE AND NOWHERE DEEPER, so the ONE caller that must not make it
		// can avoid it: across a service boundary, "identity is down" read as "everybody is
		// signed out" tells every citizen in every commune that their session ended, through the
		// one channel a commune is judged on (rule 10). See TraCuuCoLoi.
		s.log.Error("phiên công dân: không tra cứu được, từ chối", "err", err)
		return httpx.CitizenSession{}, false
	}
	return p, ok
}

// TraCuuCoLoi is TraCuu with the third answer the edge deliberately does not have: the registry
// COULD NOT BE READ.
//
// WHY BOTH EXIST. core/httpx.CitizenSessions promises ONE negative answer, and that promise is
// right at the HTTP edge: unknown token, expired session and revoked session must be
// indistinguishable from outside. An outage is not one of those three, and the difference only
// becomes actionable ACROSS A SERVICE BOUNDARY — which is where vigov.identity.v1's
// ResolveCitizenSession sits. Its contract is explicit: "UNAVAILABLE etc. — the call did not
// happen. IT IS NOT 'no session'". That distinction cannot be recovered once TraCuu has collapsed
// it, so the collapse happens in TraCuu and the RPC calls this instead. Exactly the split
// ResolveStaffPrincipal already documents against XacThuc, for the identical reason.
//
// IT DOES NOT LOG. Each of the two callers logs once, in its own voice: TraCuu at the edge, the
// gRPC handler through Server.loi. Logging here as well would put every citizen outage into the
// log pipeline twice.
//
// WHAT IT STILL COLLAPSES, STATED RATHER THAN LEFT TO BE DISCOVERED: a Scan failure returns
// ok=false with no error. That is a column-type or column-order disagreement between this Go and
// this schema — a fault in THIS service that no caller can act on and that would be identical on
// every row of every commune, not the transient dependency failure the third answer exists for. A
// connection that drops mid-read IS surfaced, through rows.Err() below.
func (s *PhienCongDanStore) TraCuuCoLoi(ctx context.Context, token string) (httpx.CitizenSession, bool, error) {
	if token == "" {
		return httpx.CitizenSession{}, false, nil
	}

	// THE TOKEN NEVER LEAVES THIS LINE. Only its hash is bound as a parameter, so a driver
	// error that echoes the statement cannot carry the token into a log — and the token
	// substitutes for a whole session (rule 3, and core/httpx says the same at the edge).
	//
	// @cross-tenant: rìa kênh công dân SUY xã từ chính phiên này (ADR 0022), nên tại thời điểm
	// tra cứu chưa có xã nào để phạm vi hoá. Trả về nhiều nhất MỘT phiên, tìm theo mã băm của
	// token 256 bit do máy chủ phát — không bao giờ trả về danh sách.
	rows, err := s.raw.QueryContext(ctx, truyVanPhienTheoToken, bamRefresh(token))
	if err != nil {
		// WRAPPED WITHOUT THE TOKEN AND WITHOUT ANY CITIZEN IDENTIFIER. This error travels into
		// a log at whichever caller handles it (rule 3).
		return httpx.CitizenSession{}, false, fmt.Errorf("phiên công dân: tra cứu: %w", err)
	}
	defer rows.Close()

	p, ok := quetPhien(rows)
	// ASKED AGAIN AFTER quetPhien, and that is not redundant: quetPhien answers the EDGE's
	// question, so it folds a dropped connection into the same ok=false as an unknown token. The
	// sticky error is still on *sql.Rows here, and this is the one place that can tell the caller
	// the read never completed rather than that the session does not exist.
	if err := rows.Err(); err != nil {
		return httpx.CitizenSession{}, false, fmt.Errorf("phiên công dân: tra cứu: %w", err)
	}
	return p, ok, nil
}

// dongPhien is what *sql.Rows satisfies, so quetPhien can be tested without a database. The
// decisions it makes — zero rows, two rows, a scan failure — are the ones that matter most and
// the ones an integration suite silently skips on a machine with no DSN.
type dongPhien interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// quetPhien reads AT MOST ONE session, in the order cotPhienCongDan names.
//
// EVERY FAILURE RETURNS THE SAME ok=false. There is no error channel out of here on purpose:
// the edge has one answer for "unusable session", and a caller that could tell the cases apart
// would eventually tell a client.
func quetPhien(rows dongPhien) (httpx.CitizenSession, bool) {
	if !rows.Next() {
		// A connection that dropped mid-read lands here too (rows.Err() would be non-nil), and
		// the answer is the same one: no usable session. It is not distinguished because the
		// edge has a single negative answer and because an outage must never be readable as a
		// statement about somebody's session.
		return httpx.CitizenSession{}, false
	}

	// cong_dan_id IS NULLABLE SINCE MIGRATION 0011 (ADR 0045 §Phiên chưa có số): a bridge session
	// opened before the phone is verified has no citizen identity yet. Scanning NULL into a plain
	// string is a Scan error, which below means "no session" — every such citizen would be signed
	// out on every request. NULL becomes "", and "" is what core/httpx.XaTuPhien refuses.
	var xa, sid string
	var congDanID sql.NullString
	if err := rows.Scan(&xa, &sid, &congDanID); err != nil {
		return httpx.CitizenSession{}, false
	}
	if rows.Next() {
		// Two sessions behind one token — see TraCuu. The registry cannot say whose session
		// this is, so it says nothing.
		return httpx.CitizenSession{}, false
	}
	if err := rows.Err(); err != nil {
		return httpx.CitizenSession{}, false
	}

	return httpx.CitizenSession{
		ID:        sid,
		CitizenID: congDanID.String,
		TenantID:  tenant.ID(xa),
	}, true
}

// PhienMoi is everything needed to open one citizen session.
//
// A STRUCT AND NOT FIVE POSITIONAL ARGUMENTS: three of them are strings that mean completely
// different things, and swapping cong_dan_id with thiet_bi compiles. Named fields are what make
// that impossible rather than merely unlikely.
type PhienMoi struct {
	// The platform-wide identity (crosstenant.DinhDanh.ID), never a phone number.
	CongDanID string

	// NguonApp or NguonGhep.
	Nguon string

	// THE TTL IS THE CALLER'S DECISION, AND IT HAS TO BE, because it is policy and the policy is
	// not settled. ADR 0019 requires a paired session to be shorter-lived than an app session
	// and deliberately leaves the number open (§CÒN MỞ #1: the TTL cannot be chosen before
	// anybody decides what the shared screen actually is). A constant here would answer an open
	// question in the one place that is expensive to change, and it would answer it silently.
	// The schema guarantees only that an expiry exists at all.
	ThoiHan time.Duration

	// Diagnostics. Never personal data (rule 3): ThietBi is a user agent or a screen label.
	IP      string
	ThietBi string
}

func (p PhienMoi) kiemTra() error {
	switch {
	case strings.TrimSpace(p.CongDanID) == "":
		return ErrThieuCongDan
	case p.Nguon != NguonApp && p.Nguon != NguonGhep:
		// Fail closed on an unknown origin instead of letting the CHECK constraint decide. A
		// constraint violation arrives as a driver error that no caller can act on, and this
		// field decides how much a session is trusted.
		return ErrNguonKhongHopLe
	case p.ThoiHan <= 0:
		return ErrThieuThoiHan
	}
	return nil
}

const chenPhienCongDan = `
INSERT INTO phien_cong_dan
    (tenant_id, id, cong_dan_id, bam_token, nguon, het_han_luc, ip_tao, thiet_bi)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

// Tao opens a session belonging to ONE commune and returns the sid plus the bearer token.
//
// THE COMMUNE IS NOT A PARAMETER. It comes from the transaction, which got it from the context
// (rule 1, invariant 4), so no caller can open a session in one commune while holding another.
//
// THE TOKEN IS RETURNED ONCE, IN PLAINTEXT, AND ONLY ITS SHA-256 IS STORED — exactly as the
// staff registry does with its refresh token. A leaked backup must not hand over working
// sessions. SHA-256 and not argon2: this is a 256-bit random value, not a human-chosen
// password, so there is nothing to brute-force and the check sits on the hot path.
//
// CHANGING COMMUNE ISSUES A NEW SESSION; it never updates an existing row (ADR 0005). That is
// also what keeps tenant_id immutable, which matters more here than elsewhere: it is the
// partition key, so an update would move the row between partitions and the old sid would go on
// working.
func (s *PhienCongDanStore) Tao(ctx context.Context, tx *store.ScopedTx, p PhienMoi) (sid, token string, err error) {
	if err := p.kiemTra(); err != nil {
		return "", "", err
	}
	if tx == nil {
		return "", "", ErrThieuGiaoDich
	}
	return s.chen(ctx, tx.Underlying(), string(tx.TenantID()), p)
}

// TaoChuaChonXa opens the session a citizen holds BEFORE choosing a commune (ADR 0005).
//
// IT IS A SEPARATE METHOD RATHER THAN AN EMPTY COMMUNE SLIPPING THROUGH Tao, and that is the
// whole point: the empty tenant_id is written by a method whose name says it is writing one.
// There is no path where a caller who forgot the commune gets this behaviour by accident
// (rule 1, forbidden #1).
//
// IT TAKES A RAW *sql.Tx because core/store cannot produce a scoped transaction without a
// commune — For(ctx) panics. The session this opens can reach no business data: every business
// route runs core/httpx.XaTuPhien, which answers 401 for a session with no commune.
func (s *PhienCongDanStore) TaoChuaChonXa(ctx context.Context, tx *sql.Tx, p PhienMoi) (sid, token string, err error) {
	if err := p.kiemTra(); err != nil {
		return "", "", err
	}
	if tx == nil {
		return "", "", ErrThieuGiaoDich
	}
	return s.chen(ctx, tx, xaChuaChon, p)
}

func (s *PhienCongDanStore) chen(ctx context.Context, tx *sql.Tx, xa string, p PhienMoi) (sid, token string, err error) {
	sid, err = maNgauNhien()
	if err != nil {
		return "", "", err
	}
	token, err = maNgauNhien()
	if err != nil {
		return "", "", err
	}

	// Client-supplied strings are truncated: they are diagnostics, and an unbounded string from
	// a client in a government database is a liability, not a feature.
	thietBi := catNgan(p.ThietBi, 200)
	ip := catNgan(p.IP, 45) // an IPv6 address in text form is at most 45 characters

	if _, err := tx.ExecContext(ctx, chenPhienCongDan,
		xa, sid, p.CongDanID, bamRefresh(token), p.Nguon,
		time.Now().UTC().Add(p.ThoiHan), ip, thietBi); err != nil {
		// No token, no sid and no citizen identifier in the message — an error travels into
		// logs (rule 3).
		return "", "", fmt.Errorf("phiên công dân: tạo: %w", err)
	}
	return sid, token, nil
}

// PhienCauMoi is everything needed to open one citizen session through the Mini App bridge
// (ADR 0045). A separate type from PhienMoi, because the invariant differs: a bridge session always
// names its Zalo account and MAY lack a citizen identity (phone not verified yet), while every other
// session must name a citizen.
type PhienCauMoi struct {
	// The tai_khoan_zalo row that opened it. Required.
	TaiKhoanZaloID string

	// The verified citizen identity, "" when the phone is not verified yet — stored as NULL, and
	// refused by core/httpx.XaTuPhien on every route that reads the citizen's own records.
	CongDanID string

	// CITIZEN_SESSION_TTL, the owner's platform-wide constant (core/config).
	ThoiHan time.Duration

	// Diagnostics reported by the bridge caller. Never personal data (rule 3).
	IP      string
	ThietBi string
}

var ErrThieuTaiKhoanZalo = errors.New("phiên công dân: phiên qua cầu phải gắn tài khoản Zalo")

const chenPhienCau = `
INSERT INTO phien_cong_dan
    (tenant_id, id, cong_dan_id, tai_khoan_zalo_id, bam_token, nguon, het_han_luc, ip_tao, thiet_bi)
VALUES ($1, $2, NULLIF($3, ''), $4, $5, 'app', $6, $7, $8)`

// TaoQuaCau opens a bridge session in THE COMMUNE OF THE TRANSACTION and returns sid, token and
// expiry. Same properties as Tao: the commune is not a parameter, the token is returned once and
// only its SHA-256 is stored, and a change of commune is a NEW row, never an update.
//
// THERE IS NO NO-COMMUNE VARIANT, ON PURPOSE. Open question #25's table for trails that belong to
// no commune is not built, so a session with an empty tenant_id could not be audited in the same
// transaction (rule 6, invariant 3) — the bridge answers "no commune" WITHOUT issuing a session.
func (s *PhienCongDanStore) TaoQuaCau(ctx context.Context, tx *store.ScopedTx, p PhienCauMoi) (sid, token string, hetHan time.Time, err error) {
	switch {
	case strings.TrimSpace(p.TaiKhoanZaloID) == "":
		return "", "", time.Time{}, ErrThieuTaiKhoanZalo
	case p.ThoiHan <= 0:
		return "", "", time.Time{}, ErrThieuThoiHan
	case tx == nil:
		return "", "", time.Time{}, ErrThieuGiaoDich
	}
	if !tx.TenantID().Valid() {
		// Unreachable through core/store (For panics without a commune); checked because a bridge
		// session with no commune is the one row this method must never write.
		return "", "", time.Time{}, fmt.Errorf("phiên công dân: giao dịch không có xã hợp lệ")
	}

	sid, err = maNgauNhien()
	if err != nil {
		return "", "", time.Time{}, err
	}
	token, err = maNgauNhien()
	if err != nil {
		return "", "", time.Time{}, err
	}
	hetHan = time.Now().UTC().Add(p.ThoiHan)

	if _, err := tx.Exec(ctx, chenPhienCau,
		string(tx.TenantID()), sid, strings.TrimSpace(p.CongDanID), p.TaiKhoanZaloID, bamRefresh(token),
		hetHan, catNgan(p.IP, 45), catNgan(p.ThietBi, 200)); err != nil {
		// No token, no sid, no identifier in the message (rule 3).
		return "", "", time.Time{}, fmt.Errorf("phiên công dân: tạo qua cầu: %w", err)
	}
	return sid, token, hetHan, nil
}

// ThuHoiCuaTaiKhoanZalo ends EVERY live session of one Zalo account, in EVERY commune, and returns
// how many it ended.
//
// THE ONE REVOCATION HERE THAT CROSSES COMMUNES, and it is the owner's decision, not a convenience:
// switching commune in the main app revokes the old session IMMEDIATELY — "mỗi lúc một phiên còn
// sống" (ADR 0045, answer to CÒN MỞ #3). The old session is in the commune the citizen is LEAVING,
// so a query scoped to the new commune cannot reach it.
//
// KEYED ON ONE ACCOUNT, NEVER ON A COMMUNE, and that is what keeps it from being a cross-commune
// write in the sense rule 1 forbids: it touches only sessions this very account opened, the
// citizen's own, the same shape crosstenant/doc.go calls shape 3.
//
// HOW THE OLD SESSIONS ARE FOUND IS AN ASSUMPTION, stated: the owner's answer says the switching
// request carries the old token, and the contract (citizen_session_bridge.proto) has no field for
// it. Revoking by account reaches every session the token could have named — and any other live
// session of the same account — without reshaping the contract. Whether to add the token field
// anyway is the contract owner's call (ledger service-identity/cau-phien-cong-dan-mini-app).
func (s *PhienCongDanStore) ThuHoiCuaTaiKhoanZalo(ctx context.Context, tx *store.ScopedTx, taiKhoanID, lyDo string) (int64, error) {
	if err := kiemThuHoi(taiKhoanID, lyDo, ErrThieuTaiKhoanZalo); err != nil {
		return 0, err
	}
	if tx == nil {
		return 0, ErrThieuGiaoDich
	}
	// @cross-tenant: đổi xã ở app chính phải thu hồi NGAY phiên ở xã cũ (ADR 0045, trả lời CÒN MỞ #3);
	// phiên ấy nằm ở xã công dân đang rời đi. Khoá theo MỘT tài khoản Zalo — chỉ phiên do chính tài
	// khoản ấy mở, không bao giờ theo xã, không bao giờ trả về dòng nào.
	kq, err := tx.Underlying().ExecContext(ctx,
		`UPDATE phien_cong_dan SET thu_hoi_luc = now(), thu_hoi_ly_do = $2
		 WHERE tai_khoan_zalo_id = $1 AND thu_hoi_luc IS NULL`, taiKhoanID, lyDo)
	if err != nil {
		return 0, fmt.Errorf("phiên công dân: thu hồi theo tài khoản Zalo: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("phiên công dân: đếm phiên đã thu hồi: %w", err)
	}
	return n, nil
}

// dongThuHoiPhienCongDan is shared by the three revocation paths so the predicate cannot drift
// between them. `thu_hoi_luc IS NULL` makes a second revocation a no-op instead of overwriting
// the time and the reason of the first — the first one is the one that happened.
const dongThuHoiPhienCongDan = `
UPDATE phien_cong_dan SET thu_hoi_luc = now(), thu_hoi_ly_do = $3
WHERE tenant_id = $1 AND thu_hoi_luc IS NULL AND `

// ThuHoi ends one session of this commune.
//
// THE REASON IS MANDATORY. A revocation with no reason is an entry nobody can account for six
// months later, and the audit entry the caller writes beside this is only as good as what it
// can say about why.
func (s *PhienCongDanStore) ThuHoi(ctx context.Context, tx *store.ScopedTx, sid, lyDo string) error {
	if err := kiemThuHoi(sid, lyDo, ErrThieuPhien); err != nil {
		return err
	}
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if _, err := tx.Underlying().ExecContext(ctx, dongThuHoiPhienCongDan+`id = $2`,
		string(tx.TenantID()), sid, lyDo); err != nil {
		return fmt.Errorf("phiên công dân: thu hồi: %w", err)
	}
	return nil
}

// ThuHoiChuaChonXa ends a session issued before a commune was chosen.
//
// WITHOUT IT, TaoChuaChonXa WOULD BE A ONE-WAY DOOR: a citizen signing out at the commune-choice
// screen, or choosing a commune and being issued a new session, would leave the old one usable
// until it expired on its own. A registry that can open a session it cannot close is not a
// registry.
func (s *PhienCongDanStore) ThuHoiChuaChonXa(ctx context.Context, tx *sql.Tx, sid, lyDo string) error {
	if err := kiemThuHoi(sid, lyDo, ErrThieuPhien); err != nil {
		return err
	}
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if _, err := tx.ExecContext(ctx, dongThuHoiPhienCongDan+`id = $2`,
		xaChuaChon, sid, lyDo); err != nil {
		return fmt.Errorf("phiên công dân: thu hồi phiên chưa chọn xã: %w", err)
	}
	return nil
}

// ThuHoiCuaCongDan ends EVERY live session one citizen holds IN THIS COMMUNE.
//
// SCOPED TO ONE COMMUNE, AND THAT IS THE CORRECT SCOPE, not a limitation. A citizen deals with
// several communes at once (ADR 0002); ending their session in commune A because commune B
// revoked one would be commune B acting on commune A's relationship with a citizen. The action
// behind it is the "log this screen out" button of ADR 0019, step 6, which belongs to one
// commune's counter.
func (s *PhienCongDanStore) ThuHoiCuaCongDan(ctx context.Context, tx *store.ScopedTx, congDanID, lyDo string) error {
	if err := kiemThuHoi(congDanID, lyDo, ErrThieuCongDan); err != nil {
		return err
	}
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if _, err := tx.Underlying().ExecContext(ctx, dongThuHoiPhienCongDan+`cong_dan_id = $2`,
		string(tx.TenantID()), congDanID, lyDo); err != nil {
		return fmt.Errorf("phiên công dân: thu hồi theo công dân: %w", err)
	}
	return nil
}

// kiemThuHoi checks the two arguments every revocation shares, BEFORE the transaction is
// touched — which is what lets these refusals be tested without a database.
func kiemThuHoi(khoa, lyDo string, thieuKhoa error) error {
	if strings.TrimSpace(khoa) == "" {
		return thieuKhoa
	}
	if strings.TrimSpace(lyDo) == "" {
		return ErrThieuLyDo
	}
	return nil
}

// GhiNhanDung records that a session was used, for the "last active" column.
//
// IT IS NOT CALLED BY TraCuu, ON PURPOSE. TraCuu runs on every citizen request; writing a row
// from inside it would turn every read of the session registry into a write to it. The caller
// decides how often this is worth doing.
//
// DELIBERATELY NOT PART OF ANY BUSINESS TRANSACTION, and the error is deliberately dropped —
// the same decision as the staff registry's GhiNhanDung. This column is a diagnostic; failing a
// citizen's real operation because a timestamp could not be updated trades the operation for a
// nicety.
//
// BOTH VALUES IN THE FILTER COME FROM THE SESSION THE SERVER ISSUED, never from the request:
// the commune is pinned in the WHERE, so this statement can touch exactly the row TraCuu
// already resolved.
func (s *PhienCongDanStore) GhiNhanDung(ctx context.Context, p httpx.CitizenSession) {
	if p.ID == "" {
		return
	}
	_, _ = s.raw.ExecContext(ctx,
		`UPDATE phien_cong_dan SET dung_gan_nhat = now()
		 WHERE tenant_id = $1 AND id = $2 AND thu_hoi_luc IS NULL`,
		string(p.TenantID), p.ID)
}

// catNgan truncates to at most n BYTES, never inside a character.
//
// WHY THE RUNE BOUNDARY MATTERS AND IS NOT TIDINESS: a Vietnamese user agent or a screen label
// is multi-byte UTF-8. Cutting at byte n can leave half a character, and PostgreSQL REFUSES an
// invalid UTF-8 byte sequence outright — the INSERT fails, and because the audit entry shares
// that transaction the whole sign-in rolls back. A citizen would be unable to sign in because
// of the name of their phone.
func catNgan(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
