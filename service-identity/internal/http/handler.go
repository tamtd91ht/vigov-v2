package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
)

// Handler serves the session routes. It holds no business logic: the use cases in
// internal/app own that, including the transaction the audit entry shares (rule 6).
type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{d: d}
}

// thanDangNhap is the request body. Field names are English, per rest-api-design §1.
//
// The password is read into a local and handed straight to the use case. It is never logged,
// never put in an error and never stored anywhere but the argon2 comparison (rule 3, rule 8).
type thanDangNhap struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// phanHoiDangNhap is what the browser gets back. THE FIELDS THAT ARE ABSENT ARE THE DESIGN:
// no password hash, no phone number, no email. A phone number is personal data under Decree
// 13/2023 (rule 3) and nothing on the sign-in screen needs it; the hash is a credential.
type phanHoiDangNhap struct {
	// Sid lets the client end its own session later: DELETE /api/v1/sessions/{sid}.
	//
	// Safe to hand to JavaScript even though the cookie is httpOnly: a sid on its own is not a
	// credential. Only a token signed with the server's key is accepted, and the sid inside it
	// is what makes that token revocable.
	Sid       string    `json:"sid"`
	ExpiresAt time.Time `json:"expires_at"`
	Staff     canBoGon  `json:"staff"`
}

type canBoGon struct {
	Code     string `json:"code"`      // cb.Ma — the business code, the one the audit trail shows
	FullName string `json:"full_name"` // shown in the header of the admin screens
	Position string `json:"position"`
}

// tuChoiDangNhap is the ONE answer given for a wrong email, a wrong password and an empty
// field.
//
// Distinguishing them tells an attacker which email addresses exist on this commune's domain,
// which is a staff directory they are not entitled to. app.ErrDangNhapThatBai is built the
// same way on purpose; keep it that way.
func (h *Handler) tuChoiDangNhap(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials",
		"Email hoặc mật khẩu không đúng.", "")
}

// DangNhap opens a session. POST /api/v1/sessions
func (h *Handler) DangNhap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The body is bounded: this route is Public, and one process serves 200+ communes. An
	// unbounded JSON body on an unauthenticated route is a way to spend the whole platform's
	// memory from outside.
	var than thanDangNhap
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&than); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body",
			"Dữ liệu gửi lên không hợp lệ.", "")
		return
	}

	// An empty field answers exactly like a wrong password. Answering "missing password"
	// instead would confirm that the email exists, which is the leak the single message above
	// exists to prevent.
	if than.Email == "" || than.Password == "" {
		h.tuChoiDangNhap(w)
		return
	}

	kq, err := h.d.DangNhap.Chay(ctx, app.YeuCauDangNhap{
		Email:   than.Email,
		MatKhau: than.Password,
		IP:      ipTu(r),
		ThietBi: r.UserAgent(),
	})
	if err != nil {
		if errors.Is(err, app.ErrDangNhapThatBai) {
			h.tuChoiDangNhap(w)
			return
		}
		// The wrapped error carries store failures, never the password and never the body.
		h.d.Log.Error("đăng nhập: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE TOKEN IS SIGNED BY THE USE CASE, INSIDE ITS TRANSACTION — not here.
	//
	// It used to be signed here, after the commit. A signing failure then left the session row
	// and its audit entry committed: the archive of a public authority asserting a sign-in that
	// never happened, in an entry that is append-only and cannot be corrected (rule 6). Moving
	// the signature inside the transaction makes the three facts agree — token, session, trail
	// all exist, or none does.
	//
	// kq.Refresh is DROPPED ON PURPOSE. skills/session-and-token required #4 asks for rotating
	// refresh tokens whose replay revokes the chain; that chain is not built yet. Handing out a
	// refresh token without it would look like protection and be none — a leaked refresh token
	// would be replayable for its whole lifetime with nothing detecting the reuse. Until the
	// rotation is built, the access token simply lives exactly as long as the session
	// (idstore.ThoiHanPhien, 12h) and is revocable through the registry. STATED GAP.
	datCookiePhien(w, kq.Token, kq.HetHanLuc)
	vietJSON(w, http.StatusCreated, phanHoiDangNhap{
		Sid:       kq.Sid,
		ExpiresAt: kq.HetHanLuc,
		Staff: canBoGon{
			Code:     kq.CanBo.Ma,
			FullName: kq.CanBo.HoTen,
			Position: kq.CanBo.ChucVu,
		},
	})
}

// DangXuat ends one session. DELETE /api/v1/sessions/{sid}
//
// A member of staff may end their OWN session and no other. Staff identity is accountable, but
// accountable is not the same as trusted with other people's sessions: the sid in the path
// comes from the client, so it is compared against the session this request actually arrived
// on (rule 4, invariant 2, applied to staff).
func (h *Handler) DangXuat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ph, ok := PhienTu(ctx)
	if !ok {
		// authz.AnyAuthenticated has already refused requests with no principal; reaching here
		// means the middleware was mounted wrong, and answering 401 is the safe reading.
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
			"Bạn chưa đăng nhập.", "")
		return
	}

	// 404, NOT 403. A 403 would confirm that the sid exists and belongs to somebody else — the
	// existence of another person's session is itself information (rule 4, forbidden #2). The
	// same answer is given whether the sid is another person's, already revoked, or invented.
	if r.PathValue("sid") != ph.Sid {
		httpx.WriteError(w, http.StatusNotFound, "session_not_found",
			"Không tìm thấy phiên đăng nhập.", "")
		return
	}

	// The use case revokes the session and writes the audit entry in ONE transaction (rule 6,
	// invariant 3). MaCanBo, not the internal id: the trail records a business code.
	if err := h.d.DangXuat.Chay(ctx, ph.Sid, ph.MaCanBo, ipTu(r)); err != nil {
		h.d.Log.Error("đăng xuất: lỗi hệ thống",
			"can_bo", ph.MaCanBo, "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	xoaCookiePhien(w)
	w.WriteHeader(http.StatusNoContent)
}

func vietJSON(w http.ResponseWriter, status int, than any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(than)
}
