package http

// The parts of the catalogue WRITE routes that belong to the whole service rather than to one
// catalogue.
//
// SAME ARGUMENT AS store/danh_muc_ghi.go: a service may own more than one reference catalogue, and
// one mapping from refusal to status code is the only way two catalogues answer the same question
// the same way. A per-catalogue copy drifts, and the copy that drifts answers 500 where it meant
// 409 — which reads to an operator as a broken server rather than as a rule doing its job.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// thanToiDa bounds the request body. 64 KiB is far past any of these bodies and far short of
// anything worth streaming: the point is that an unbounded body is memory a client chooses.
const thanToiDa = 64 << 10

// docThan decodes a JSON body, answering 400 itself on failure.
//
// THE DECODER'S OWN MESSAGE NEVER REACHES THE CLIENT. It quotes the offending input, which on this
// surface is free text somebody typed and on other surfaces would be personal data (rule 3,
// forbidden #3). The sentence returned says what to fix without repeating what was sent.
func docThan(w http.ResponseWriter, r *http.Request, vao any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanToiDa)
	if err := json.NewDecoder(r.Body).Decode(vao); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// nguoiThucHien builds the audit actor from the request.
//
// THE TRAIL RECORDS THE BUSINESS CODE `ma`, NEVER THE INTERNAL id (rule 6, invariant 2), and that
// is why this reads `p.Ma` off a principal whose `p.ID` is right there beside it. `audit_log
// .actor_id` is read years later by somebody handling a complaint or an inspection: `CB-00123`
// names a person to them with no lookup still alive, a ULID names nobody. The policy was written
// at service-identity/internal/app/dang_nhap.go:151 long before this file existed — and this file
// broke it on 2026-09-22 with nothing turning red, because a comment is not a check. What reads
// the policy now is `.claude/hooks/audit_actor_guard.py` and `tools/check_audit_actor.py`.
//
// AN EMPTY `Ma` REFUSES THE WRITE, AND NEVER FALLS BACK TO p.ID. It is empty only when this
// service is talking to an identity older than the `ma` field, which rule 2, forbidden #4 made
// optional on purpose. A fallback there would put internal ids back into the column silently, one
// deployment window at a time, with every test still green. 500 is the honest answer: rule 6 does
// not permit a business write whose trail cannot name who made it.
//
// THE IP COMES FROM THIS PROCESS'S OWN SOCKET. httpx.ClientIP does not trust X-Forwarded-For, and
// rule 6, invariant 2 wants the address the request really arrived from — not one the caller chose
// to name. The same call phieu_phan_anh.go makes for the same reason.
//
// A MISSING PRINCIPAL IS A 500 AND NOT AN ANONYMOUS ENTRY. These routes sit behind
// authz.RequirePermission, so there is always one; reaching here without one means the route was
// mounted wrong, and an audit entry attributed to nobody is exactly what rule 6 exists to prevent.
func nguoiThucHien(r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok || p.Ma == "" {
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.Ma, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}

// traLoiLoiGhi maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL THREE ROUTES, because three copies of this mapping would drift and the copy
// that drifts is the one that answers 500 where it meant 409 — which reads to an operator as a
// broken server rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR A TIER REFUSAL: the caller holds `admin.lookup` and is allowed to manage
// the catalogue. What is refused is this operation on THIS row, because of what the row is. 403
// would send an administrator to the Phân quyền screen to grant a permission that would change
// nothing.
func (h *Handler) traLoiLoiGhi(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, docstore.ErrDanhMucKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy mục danh mục này.", "")
	case errors.Is(err, docstore.ErrMaDaTonTai):
		// The message says WHY a code that is nowhere on the screen is nonetheless taken: a
		// soft-deleted row keeps its code forever. Without that sentence this reads as a bug.
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			"Mã này đã được dùng trong xã — kể cả khi dòng mang mã đó đã bị xoá. "+
				"Mã đã cấp thì không cấp lại. Hãy chọn một mã khác.", "")
	case errors.Is(err, docstore.ErrDanhMucDayTran):
		httpx.WriteError(w, http.StatusConflict, "catalogue_full",
			"Danh mục loại văn bản của xã đã đạt số mục tối đa. Hãy tắt hoặc xoá bớt mục không dùng.", "")
	case errors.Is(err, domain.ErrKhongXoaDuocMucHeThong):
		httpx.WriteError(w, http.StatusConflict, "system_row",
			"Mục do hệ thống cấp thì không xoá được. Hãy tắt mục đó thay vì xoá.", "")
	case errors.Is(err, domain.ErrKhongTatDuocMucReNhanh):
		httpx.WriteError(w, http.StatusConflict, "code_branch_row",
			"Phần mềm có xử lý riêng theo mã của mục này nên không tắt được. Có thể đổi nhãn hiển thị.", "")
	case laLoiDauVao(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on.
		h.d.Log.Error("danh mục loại văn bản: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVao reports whether this is a refusal of what the client sent, as opposed to a failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is the
// client's fault" turns a database outage into a 400, and a client that believes its input is wrong
// retries with different input forever while nobody is told the server is broken.
func laLoiDauVao(err error) bool {
	for _, mot := range []error{
		domain.ErrMaTrong, domain.ErrMaSaiDinhDang, domain.ErrMaQuaDai,
		domain.ErrNhanTrong, domain.ErrNhanQuaDai,
		domain.ErrThuTuNgoaiKhoang,
		domain.ErrThieuLyDoXoa, domain.ErrLyDoXoaQuaDai,
		domain.ErrMaBatBien, domain.ErrNguonDoTuClient,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
