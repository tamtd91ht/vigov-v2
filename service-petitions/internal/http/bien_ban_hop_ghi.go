package http

// The STAFF WRITE surface of the meeting-minutes register (docs/ui-ux/04-bien-ban-hop.md §3, §4).
//
//	POST /api/v1/meetings                                  task.create
//	POST /api/v1/meetings/{id}/conclusions                  task.create
//	POST /api/v1/meetings/{id}/conclusions/{stt}/task       task.create
//
// # THE PERMISSION KEY WAS CHECKED AGAINST THE `quyen` TABLE FIRST, AND NO KEY WAS INVENTED
//
// The table seeds 33 keys at service-identity/migrations/0001_init.sql:272-305 and TWO more in its
// 0007; there is NO `meeting.*` key among them, and none is created here (rule 5, invariant 3c): a
// key no migration seeds is a right no administrator can grant, so the route would answer 403 to
// every account for ever while the tests stayed green, because a fake checker grants any string.
//
// `task.create` — "Tạo nhiệm vụ", :301 — is the honest fit for all three rather than the nearest:
//
//	…/conclusions/{stt}/task   IS the creation of a task. It runs the same use case
//	                           POST /api/v1/tasks runs and mints a number from the same series.
//	POST /meetings             the minutes exist to PRODUCE those tasks (§1), the screen lives at
//	…/conclusions              `/nhiem-vu/bien-ban` inside the task module, and every conclusion is
//	                           a task waiting for somebody to confirm it. An account that may not
//	                           file work into the register has nothing to do with typing the meeting
//	                           that generates it.
//
// ⚠ WHAT THAT COSTS, STATED RATHER THAN GLOSSED: an office clerk who should only TYPE MINUTES has to
// be given `task.create`, which also lets them book a task directly. Whether a commune wants "may
// record minutes" separated from "may create tasks" is a question for the CUSTOMER — it is open
// question #27, whose `ask_before` list names exactly this case ("khai quyền cho một tuyến mà bảng
// quyen không có khoá đúng nghĩa"). It is REPORTED as a finding; splitting it later is one seeded
// key plus one literal in routes.go. The alternative available today was `document.create` ("Vào sổ
// văn bản"), and it is worse: it would hand this register to the văn thư role of a different
// subsystem, on a table migration 0007 deliberately placed in `petitions`.
//
// # THE URL NOUNS
//
//	meetings      the noun the read route already serves — see internal/http/bien_ban_hop.go for
//	              where it comes from and why it is not in the mapping table yet.
//	conclusions   the same source, the same finding. `bien-ban` and `ket-luan` are BLOCKED segments
//	              in hooks/rest_api_guard.py, so §6's own sketch (`/api/bien-ban`) could not ship.
//	task          the split, expressed as the RESOURCE it produces rather than as a verb — §6
//	              sketched it as `/tach-nhiem-vu`, which is both Vietnamese and a verb. It is
//	              SINGULAR because the path was fixed by the session coordinating this pass with the
//	              admin web; a conclusion may be split MANY times (§3), so `tasks` would have read
//	              more naturally as a collection, and that is noted rather than changed underneath a
//	              client already building against it.

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// --- request bodies -------------------------------------------------------------------------------

// taoBienBanVao is the body of POST /api/v1/meetings — §4's "Nhập biên bản" modal.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017), and they are the
// same names GET /api/v1/meetings already returns for the same columns.
//
// # THE ABSENCES ARE REFUSALS RATHER THAN OMISSIONS
//
// There is no `created_by`: that is the session's own principal, and a request that could name its
// author is a request that can forge the trail. There is no `attachments`: §4 offers a file field
// and this repository has no file store, so a list on the wire would be a promise nothing keeps —
// the column keeps its `'[]'` default until there is one.
type taoBienBanVao struct {
	Title string `json:"title"`

	// HeldOn is a CALENDAR DAY, `2026-08-05`, and REQUIRED (§4). A string and not a time.Time: an
	// RFC 3339 instant would carry a midnight in some time zone, and a browser one zone away would
	// file the meeting on the previous day. Parsed by ngayHopVao, which owns the format for both
	// directions.
	HeldOn string `json:"held_on"`

	// The optional half of §4. EMPTY IS ORDINARY — the card drops the segment rather than showing a
	// placeholder, and the prototype's own draft carries only two of them.
	ReferenceNo string `json:"reference_no,omitempty"`
	Location    string `json:"location,omitempty"`

	// ChairedBy is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id (rule 6, invariant
	// 8). §5 of the specification models it as a uuid; the rule wins, and migration 0007 says why.
	ChairedBy string `json:"chaired_by"`

	// Content is §4's "Nội dung biên bản" — the minutes in full. THE SAME NAME THE CONCLUSION
	// RESPONSE USES FOR ITS OWN `noi_dung`: one column meaning, one field name.
	//
	// ⚠ A COMMUNE'S MINUTES QUOTE CASES. Nothing may put this value into a log line, an error message
	// or a file name (rule 3, forbidden #1 and #4).
	Content string `json:"content,omitempty"`

	// Attendees is §4's "Thành phần tham dự": staff codes and/or free text.
	//
	// ⚠ IT IS NOT A PLACE FOR CITIZEN DETAILS (rule 3) — see migration 0007 on the column.
	Attendees []string `json:"attendees,omitempty"`

	// Conclusions are §4's dynamic list, in the order the clerk typed them, which becomes ① ② ③.
	//
	// ⚠ A LIST OF STRINGS HERE, A LIST OF RECORDS IN THE RESPONSE, and the asymmetry is deliberate:
	// what a person types is a sentence, while what comes back carries the id §3 needs, the ordinal
	// the circle shows and the two task counters. A request shaped like the response would invite a
	// client to send an id or a counter, neither of which it may decide.
	//
	// EMPTY IS A REAL STATE: §7.3 saves minutes with no conclusions ("nhập nháp trước, bổ sung sau").
	Conclusions []string `json:"conclusions,omitempty"`
}

// themKetLuanVao is the body of POST /api/v1/meetings/{id}/conclusions — the one-line textarea at
// the bottom of every card (§2).
//
// THERE IS NO `ordinal` FIELD. §7.2 numbers conclusions continuously within one meeting and the
// server mints the next number from the high-water mark, under the meeting's lock; a client naming
// its own ordinal could overwrite the numbering of a document that is already printed.
type themKetLuanVao struct {
	Content string `json:"content"`
}

// tachKetLuanVao is the body of POST /api/v1/meetings/{id}/conclusions/{stt}/task — §3's
// confirmation box.
//
// # IT IS THE FULL TASK FORM, AND THAT IS THE POINT OF THE FLOW
//
// §3 pre-fills the "Giao việc mới" modal from the conclusion and waits for a person to confirm it.
// The server therefore receives what that person confirmed — the same fields taoNhiemVuVao carries —
// and derives NOTHING: not the title from the conclusion's sentence, and not the deadline from a
// date inside it ("báo cáo trước ngày 20/8"). The sibling implementation measured what guessing
// costs: right three times out of four, and the fourth leaves a wrongly titled task in a register
// that cannot be deleted, only withdrawn.
//
// # `source` AND `source_id` ARE NOT ON IT, AND THAT IS THE WHOLE SECURITY PROPERTY
//
// §3 shows "Nguồn giao = Từ kết luận họp" as LOCKED. Here that is expressed by the pair being
// absent from the wire rather than validated on it: the use case sets `nguon_giao` and `nguon_id`
// from the conclusion named in the PATH, so there is no value a client could send to point a task at
// a record of its choosing. A field that had to be checked would be a field somebody could forget to
// check.
type tachKetLuanVao struct {
	Code     string `json:"code,omitempty"`
	AutoCode bool   `json:"auto_code"`

	Type     string `json:"type"`
	Bloc     string `json:"bloc,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"description,omitempty"`
	Priority string `json:"priority,omitempty"`

	Unit     string `json:"unit,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Assigner string `json:"assigner,omitempty"`
	LeadUnit string `json:"lead_unit,omitempty"`
	Monitor  string `json:"monitor,omitempty"`

	// DueAt is "Hạn hoàn thành". A POINTER, so "no deadline" stays expressible — §3 only SUGGESTS a
	// date when the sentence contains one, and the person may clear it.
	//
	// ⚠ IT IS THE ONLY MOMENT THIS VALUE CAN EVER BE SET: `han_ban_dau` takes the same instant and
	// migration 0006's trigger refuses every later change to it.
	DueAt *time.Time `json:"due_at,omitempty"`

	// Parent makes the new task a sub-task (§5.10 of chapter 02). A conclusion CAN be split into a
	// child of an existing task — the two facts are independent: `nguon_giao` says where the work
	// came from, `nhiem_vu_cha_id` says which work it belongs under.
	Parent string `json:"parent,omitempty"`
}

// --- the three handlers ----------------------------------------------------------------------------

// ngayHopVao reads §4's meeting day off the wire.
//
// ONE FUNCTION, ONE FORMAT, AND IT IS THE INVERSE OF ngayHopRa (internal/http/bien_ban_hop.go).
// `2006-01-02` is ISO 8601 with no time and no zone, which is what a DATE column means; accepting an
// RFC 3339 instant here would let a browser's zone decide which DAY the minutes were filed under.
//
// AN EMPTY STRING IS "not sent" AND IS REFUSED BY THE DOMAIN, not here: §4 marks the field required,
// and one sentence about it belongs in one place.
func ngayHopVao(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, domain.ErrThieuNgayHop
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		// THE CLIENT'S STRING IS NOT ECHOED BACK. The domain's own sentence names the format, which
		// is the only thing the caller can act on.
		return time.Time{}, domain.ErrNgayHopKhongDocDuoc
	}
	return t, nil
}

// TaoBienBan records one meeting's minutes, with the conclusions typed on the form.
// POST /api/v1/meetings
func (h *Handler) TaoBienBan(w http.ResponseWriter, r *http.Request) {
	var vao taoBienBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	ngay, err := ngayHopVao(vao.HeldOn)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	bb, err := h.d.GhiBienBan.TaoBienBan(r.Context(), app.YeuCauTaoBienBan{
		TenCuocHop: vao.Title,
		NgayHop:    ngay,
		SoHieu:     vao.ReferenceNo,
		DiaDiem:    vao.Location,
		ChuTriMa:   vao.ChairedBy,
		NoiDung:    vao.Content,
		ThanhPhan:  vao.Attendees,
		KetLuan:    vao.Conclusions,
	}, nguoi)
	if err != nil {
		h.traLoiLoiBienBan(w, r, "nhập biên bản họp", err)
		return
	}
	// 201 AND THE WHOLE CARD, because the caller's next act is on this record: §3's split needs the
	// CONCLUSION IDS, which are the one thing the client cannot have known before asking.
	vietJSON(w, http.StatusCreated, bienBanRaNgoai(bb))
}

// ThemKetLuan appends one conclusion to minutes that already exist.
// POST /api/v1/meetings/{id}/conclusions
func (h *Handler) ThemKetLuan(w http.ResponseWriter, r *http.Request) {
	var vao themKetLuanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	kl, err := h.d.GhiBienBan.ThemKetLuan(r.Context(), r.PathValue("id"),
		app.YeuCauThemKetLuan{NoiDung: vao.Content}, nguoi)
	if err != nil {
		h.traLoiLoiBienBan(w, r, "thêm kết luận họp", err)
		return
	}
	// The ORDINAL is what the caller could not have known: §7.2 mints it from the high-water mark,
	// so a client that guessed `len(conclusions) + 1` would be wrong on any meeting a conclusion has
	// ever been removed from.
	vietJSON(w, http.StatusCreated, ketLuanRa{
		ID:      kl.ID,
		Ordinal: kl.ThuTu,
		Content: kl.NoiDung,
		// A conclusion that did not exist a statement ago has no task and no mark: `chua-giao`,
		// derived by the same function every read uses rather than spelled here.
		Status:    string(kl.TrangThai()),
		CreatedAt: kl.TaoLuc,
	})
}

// TachKetLuanThanhNhiemVu books a task whose origin is this conclusion.
// POST /api/v1/meetings/{id}/conclusions/{stt}/task
//
// THE REPLY IS THE TASK, in the same shape GET /api/v1/tasks/{ma} and POST /api/v1/tasks return —
// one act, one representation, whichever door it came through. The MINTED REGISTER NUMBER is the
// thing the caller needs and could not have known.
func (h *Handler) TachKetLuanThanhNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao tachKetLuanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	// THE ORDINAL IS PARSED HERE AND NOT FURTHER DOWN. A path segment that is not a number is a
	// malformed request, and turning it into a 400 at the edge keeps the use case's signature honest
	// about what it takes (an ordinal, not a string that might be one).
	thuTu, err := strconv.Atoi(r.PathValue("stt"))
	if err != nil || thuTu < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Số thứ tự kết luận phải là một số nguyên dương.", "")
		return
	}

	yc := app.YeuCauTaoNhiemVu{
		Ma:                  vao.Code,
		TuSinhMa:            vao.AutoCode,
		Loai:                vao.Type,
		Khoi:                vao.Bloc,
		TieuDe:              vao.Title,
		MoTa:                vao.Body,
		MucUuTien:           vao.Priority,
		BoPhanID:            vao.Unit,
		NguoiThucHienMa:     vao.Assignee,
		LanhDaoGiaoViecMa:   vao.Assigner,
		CoQuanChuTriID:      vao.LeadUnit,
		ChuyenVienTheoDoiMa: vao.Monitor,
		NhiemVuChaID:        vao.Parent,
		// NguonGiao AND NguonID ARE DELIBERATELY NOT SET HERE. The use case fills them from the
		// conclusion it resolves out of the path — see app.TachKetLuanThanhNhiemVu.
	}
	if vao.DueAt != nil {
		yc.HanXuLy = *vao.DueAt
	}

	n, err := h.d.GhiBienBan.TachKetLuanThanhNhiemVu(r.Context(), r.PathValue("id"), thuTu, yc, nguoi)
	if err != nil {
		if errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
			// THE SAME ANSWER AS AN UNKNOWN MEETING, ANOTHER COMMUNE'S MEETING AND A REMOVED
			// CONCLUSION. Telling them apart tells a caller which minutes exist in a register they
			// are not reading.
			httpx.WriteError(w, http.StatusNotFound, "not_found",
				"Không tìm thấy kết luận này trong biên bản.", "")
			return
		}
		// EVERY OTHER REFUSAL IS THE TASK REGISTER'S, AND IT IS MAPPED BY THE TASK REGISTER'S OWN
		// FUNCTION. A second mapping here would answer the same refusal with a different status
		// depending on which door the act came through — and the one that drifts is the one nobody
		// re-reads.
		h.traLoiLoiNhiemVu(w, r, "tách kết luận thành nhiệm vụ", err)
		return
	}
	vietJSON(w, http.StatusCreated, nhiemVuRaNgoai(n))
}

// --- shared ----------------------------------------------------------------------------------------

// traLoiLoiBienBan maps one use-case failure of the MINUTES register onto a status and a sentence.
//
// ONE FUNCTION FOR THE TWO ROUTES THAT WRITE THIS REGISTER. The split route does not use it: its
// refusals are the task register's, and it calls traLoiLoiNhiemVu so one act has one set of answers.
//
// ⚠ THE LAST ARGUMENT OF httpx.WriteError IS THE TRACE ID, NOT A FIELD NAME. Every call below passes
// "" for it. Worth saying because the petition path next door passes field names there
// (internal/http/xu_ly_phan_anh.go), which lands them in `trace_id` on the wire — httpx.Error has
// three members and none of them names a field.
func (h *Handler) traLoiLoiBienBan(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, petstore.ErrBienBanKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy biên bản họp này.", "")
	case errors.Is(err, petstore.ErrKetLuanKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy kết luận này trong biên bản.", "")

	case domain.LaLoiDauVaoBienBan(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")

	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3, forbidden
		// #3) — on this register that is more than a habit: its columns hold a commune's minutes. The
		// commune is logged because it is the only thing an operator can act on.
		h.d.Log.Error("ghi biên bản họp: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
