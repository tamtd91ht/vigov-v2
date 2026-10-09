package http

// Merging duplicate petitions (ADR 0087) — the handlers of routes_citizen_report_merge.go. HTTP
// translation and nothing else: the rules, the transaction and the trail are app.XuLyPhanAnh's
// (internal/app/petition_merge.go), the search is app.DuplicateCandidates'.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CitizenReportMerging is the two acts. *app.XuLyPhanAnh satisfies it — the SAME instance as XuLyPhieu,
// so a merge and a closing of the same main petition share one lock discipline. Each takes the
// restricted fact, like every petition act, so neither can touch a `can-bo` petition for a colleague.
type CitizenReportMerging interface {
	Merge(ctx context.Context, code string, req app.MergeRequest, actor audit.Actor,
		restricted app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
	Unmerge(ctx context.Context, code, reason string, actor audit.Actor,
		restricted app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
}

// CitizenReportMergeLinks reads the link CODES the staff detail shows. *petstore.PhieuPhanAnhStore
// satisfies it. Read-only: a GET must not reach a write.
type CitizenReportMergeLinks interface {
	MergeLinks(ctx context.Context, p domain.PhieuPhanAnh) (domain.MergeLinks, error)
}

// CitizenReportDuplicateCandidates is the suspected-duplicate search. *app.DuplicateCandidates satisfies it.
type CitizenReportDuplicateCandidates interface {
	Candidates(ctx context.Context, code string, restricted app.QuyenXemHanChe) (app.DuplicateCandidateResult, error)
}

// --- bodies and replies -------------------------------------------------------------------------------

// citizenReportMergeIn is the body of POST …/{maTraCuu}/merge: the petition in the PATH is merged INTO
// the one named here. `reason` is optional, staff-internal, may hold personal data — never in the trail,
// never logged; it is kept on the history row.
type citizenReportMergeIn struct {
	MainCode string `json:"main_code"`
	Reason   string `json:"reason,omitempty"`
}

// citizenReportUnmergeIn is the body of POST …/{maTraCuu}/unmerge. `reason` is MANDATORY (owner,
// 09/10/2026).
type citizenReportUnmergeIn struct {
	Reason string `json:"reason"`
}

// duplicateCandidatesOut is GET …/{maTraCuu}/duplicate-candidates: the suspected duplicates, CLOSEST
// FIRST, in the register list's item shape (reporters masked, as on the list), plus the commune's radius
// and window they were found with. `truncated` says the box read hit its bound and the list may be short.
type duplicateCandidatesOut struct {
	Items        []phieuPhanAnhRa `json:"items"`
	RadiusMeters int              `json:"radius_meters"`
	WindowDays   int              `json:"window_days"`
	Truncated    bool             `json:"truncated"`
}

// --- handlers -----------------------------------------------------------------------------------------------

// DuplicateCandidatesOf serves GET /api/v1/citizen-reports/{maTraCuu}/duplicate-candidates.
//
// THE REPORTER IS MASKED, ALWAYS — the list's rule (DanhSachPhieu): a list cannot carry one honest audit
// entry per disclosure. Labels and residential-unit names are read ONCE for the whole answer.
func (h *Handler) DuplicateCandidatesOf(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res, err := h.d.DuplicateCandidates.Candidates(ctx, r.PathValue("maTraCuu"), h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerMergeError(w, r, "tìm phiếu nghi trùng", err)
		return
	}
	nhan, err := h.d.NhanLinhVuc.DanhSach(ctx)
	if err != nil {
		writeFieldCatalogueError(w, r, h.d.Log.Error, "đọc nhãn lĩnh vực cho phiếu nghi trùng", err)
		return
	}
	labels := make(map[string]string, len(nhan))
	for _, n := range nhan {
		labels[n.Ma] = n.Nhan
	}
	unitIDs := make([]string, 0, len(res.Items))
	for _, p := range res.Items {
		unitIDs = append(unitIDs, p.ThonID)
	}
	unitNames, err := h.residentialUnitNames(ctx, unitIDs)
	if err != nil {
		h.d.Log.Warn("CẢNH BÁO: phiếu nghi trùng từ chối vì chưa tra được tên thôn / tổ dân phố",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		writeResidentialUnitNamesUnavailable(w)
		return
	}
	out := duplicateCandidatesOut{Items: make([]phieuPhanAnhRa, 0, len(res.Items)),
		RadiusMeters: res.RadiusMeters, WindowDays: res.WindowDays, Truncated: res.Truncated}
	for _, p := range res.Items {
		item := phieuRaNgoai(p, labels[p.LinhVuc], false) // `false`: masked, a literal — see DanhSachPhieu
		item.ResidentialUnitName = unitNames[p.ThonID].Name
		out.Items = append(out.Items, item)
	}
	vietJSON(w, http.StatusOK, out)
}

// MergeCitizenReport serves POST /api/v1/citizen-reports/{maTraCuu}/merge — the petition in the path is
// merged into `main_code`. Replies with the merged petition, `merged_into` = the main petition's code.
func (h *Handler) MergeCitizenReport(w http.ResponseWriter, r *http.Request) {
	var in citizenReportMergeIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	after, err := h.d.CitizenReportMerge.Merge(ctx, r.PathValue("maTraCuu"),
		app.MergeRequest{MainCode: in.MainCode, Reason: in.Reason}, actor, h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerMergeError(w, r, "gộp phiếu", err)
		return
	}
	mainCode := strings.TrimSpace(in.MainCode)
	h.traPhieu(w, r, after, func(ra *phieuPhanAnhRa) { ra.MergedInto = mainCode })
}

// UnmergeCitizenReport serves POST /api/v1/citizen-reports/{maTraCuu}/unmerge.
func (h *Handler) UnmergeCitizenReport(w http.ResponseWriter, r *http.Request) {
	var in citizenReportUnmergeIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	after, err := h.d.CitizenReportMerge.Unmerge(ctx, r.PathValue("maTraCuu"), in.Reason, actor, h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerMergeError(w, r, "tách phiếu", err)
		return
	}
	h.traPhieu(w, r, after)
}

// --- refusals ----------------------------------------------------------------------------------------------

// mergeRefusals is the sentence an officer reads for each merge refusal. FIXED SENTENCES, never
// err.Error(): the chain arrives wrapped with the commune id by app.bocPhieu (the traLoiLoiXuLy reason).
// None names a petition's content, field or reporter — a refusal travels to logs and screens.
var mergeRefusals = []struct {
	err    error
	status int
	code   string
	text   string
}{
	{domain.ErrMergeMainMissing, http.StatusBadRequest, "invalid_request", "Chưa chọn phiếu chính để gộp vào."},
	{domain.ErrMergeSelf, http.StatusBadRequest, "invalid_request", "Không gộp một phiếu vào chính nó."},
	{domain.ErrMergeReasonTooLong, http.StatusBadRequest, "invalid_request", "Lý do quá dài (tối đa 2000 ký tự)."},
	{domain.ErrUnmergeReasonMissing, http.StatusBadRequest, "invalid_request", "Chưa nhập lý do tách phiếu."},
	{domain.ErrMergeDeadlineBeforeOrigin, http.StatusConflict, "merge_deadline_before_origin",
		"Không gộp được: hạn xử lý của phiếu được gộp sớm hơn lúc phiếu chính được phản ánh. " +
			"Hãy chọn phiếu được phản ánh trước làm phiếu chính."},
	{domain.ErrMergeNotOpen, http.StatusConflict, "merge_state",
		"Chỉ gộp được hai phiếu đều chưa xử lý xong (đã tiếp nhận, đang phân loại, đã chuyển xử lý, đang xử lý)."},
	{domain.ErrMergeAlreadyMerged, http.StatusConflict, "merge_state",
		"Phiếu này đã được gộp vào một phiếu chính. Hãy tách phiếu trước nếu muốn gộp vào phiếu khác."},
	{domain.ErrMergeTargetIsMerged, http.StatusConflict, "merge_state",
		"Phiếu được chọn đã được gộp vào phiếu khác. Hãy gộp vào phiếu chính của nó."},
	{domain.ErrMergeHasChildren, http.StatusConflict, "merge_state",
		"Phiếu này đang là phiếu chính của phiếu khác nên không gộp tiếp được. Hãy chọn phiếu này làm phiếu chính."},
	{domain.ErrMergeStaffConduct, http.StatusConflict, "merge_state",
		"Phiếu về thái độ, tác phong cán bộ không bao giờ được gộp."},
	{domain.ErrNotMerged, http.StatusConflict, "merge_state", "Phiếu này không được gộp vào phiếu nào."},
	{domain.ErrUnmergeNotOpen, http.StatusConflict, "merge_state",
		"Chỉ tách được phiếu chưa xử lý xong."},
}

// answerMergeError maps a merge / unmerge / search failure. 404 for every cause that would disclose a
// record (unknown, another commune's, soft-deleted, `can-bo` without the key); the merge refusals above;
// everything else — a state race, a missing actor, a system failure — through traLoiLoiXuLy, the one
// mapping every petition act shares.
func (h *Handler) answerMergeError(w http.ResponseWriter, r *http.Request, act string, err error) {
	if errors.Is(err, petstore.ErrPhieuKhongTonTai) || errors.Is(err, app.ErrPhieuHanChe) {
		h.khongTimThay(w)
		return
	}
	for _, m := range mergeRefusals {
		if errors.Is(err, m.err) {
			staff := ""
			if p, ok := authz.From(r.Context()); ok {
				staff = p.Ma
			}
			// INFO: a refusal is the rule doing its job. The chain holds no code, content or reporter
			// (app.bocPhieu); the officer is named by business code.
			h.d.Log.Info("gộp phiếu phản ánh: từ chối "+act, "xa", string(tenant.MustFrom(r.Context())),
				"can_bo", staff, "ma_loi", m.code, "err", err)
			httpx.WriteError(w, m.status, m.code, m.text, "")
			return
		}
	}
	h.traLoiLoiXuLy(w, r, act, err)
}
