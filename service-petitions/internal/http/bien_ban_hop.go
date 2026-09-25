package http

// The STAFF READ surface of the meeting-minutes register (docs/ui-ux/04-bien-ban-hop.md §2, §5).
//
//	GET /api/v1/meetings                                  task.read
//	GET /api/v1/meetings/{id}                             task.read
//	GET /api/v1/meetings/{id}/conclusions/{stt}/tasks     task.read
//
// # THE URL NOUN IS `meetings`, AND IT IS NOT IN THE MAPPING TABLE YET
//
// ADR 0011 puts URL segments in ENGLISH and says a new resource name is looked up in
// kb/00-foundation/ubiquitous-language.md rather than translated on the spot. THAT TABLE HAS NO ROW
// FOR `bien_ban_hop` — so this name is not read from it, and it is not invented here either:
// `meetings` (with `conclusions` nested inside it) is what the RUNNING sibling implementation
// serves, `apps/api/app/modules/tasks/router.py:45`, read under the project owner's instruction of
// 2026-09-23 to consult that repository where this one has no answer. Reported as a finding so the
// row is added by the session that owns that file (rule 9, invariant 2) — not written from here.
//
// `bien-ban` AND `ket-luan` ARE BLOCKED SEGMENTS in hooks/rest_api_guard.py, which is the same
// decision expressed as a gate: the specification's own §6 sketch (`/api/bien-ban`) could not ship.
//
// # `task.read` AND NOT A KEY OF ITS OWN
//
// The `quyen` table has no `meeting.*` key (service-identity/migrations/0001_init.sql:273-305), and
// NO KEY IS INVENTED HERE (rule 5, invariant 3c): a key no migration seeds is a right no
// administrator can grant, so the route would answer 403 to every account forever while the tests
// stayed green. `task.read` ("Xem nhiệm vụ", :304) is the honest fit rather than the nearest one —
// this screen lives at `/nhiem-vu/bien-ban` inside the task module, everything it shows is either
// the origin of a task or a count OF tasks, and anybody who may not read the register has no use
// for a page whose badges are made of it.
//
// ⚠ WHETHER A COMMUNE WANTS TO SEPARATE "may read meeting minutes" FROM "may read tasks" IS A
// QUESTION FOR THE CUSTOMER (open question #27), not for this file. Splitting it later is a seeded
// key plus one literal here.
//
// # THE WRITE ROUTES ARE IN internal/http/bien_ban_hop_ghi.go
//
// Recording minutes (§4), appending a conclusion and SPLITTING A CONCLUSION INTO A TASK (§3) live
// next door, under `task.create`. The third one does not derive a deadline out of "báo cáo trước
// ngày 20/8": §3's suggestion is the SCREEN's, the person confirms a date, and working-hours
// arithmetic keeps its single implementation in identity (rule 10, forbidden #2; ADR 0007).
//
// WHAT IS STILL ABSENT on this read surface: nothing reads `dinh_kem` (no file store). Edit, delete
// and signing are write acts and are not in this file.

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// bienBanRa is one meeting's minutes as they leave the API to a member of staff.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017).
//
// ONE SHAPE FOR THE LIST, THE DETAIL AND THE CREATE REPLY. The minutes body, the attendee list and
// the supplements are carried by the DETAIL route only (pointer fields, absent elsewhere — see
// SupplementedBy); the attachments by no route yet (no file store).
type bienBanRa struct {
	// ID is the internal id, and it is on the wire for ONE reason: §3's split flow sends the
	// CONCLUSION's id back as `nguon_id`, so ids on this surface are load-bearing rather than
	// decorative. The minutes have no business code of their own — `reference_no` is typed by hand,
	// is optional, and repeats across years (migration 0007).
	ID string `json:"id"`

	Title string `json:"title"`

	// HeldOn is a CALENDAR DAY, `2026-08-05`, NOT an instant. The column is DATE; sending it as an
	// RFC 3339 timestamp would attach a midnight in some time zone to it, and a browser one zone
	// away would render the previous day on a card whose whole content is that date.
	HeldOn string `json:"held_on"`

	// ReferenceNo is `31/BB-UBND`. EMPTY IS ORDINARY — §2 drops the segment from the meta line when
	// it is missing, rather than showing a placeholder.
	ReferenceNo string `json:"reference_no"`
	Location    string `json:"location"`

	// ChairedBy is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id (rule 6, invariant
	// 8). No name is joined in: this service does not own the staff directory (rule 2), and a name is
	// not this response's to hold.
	ChairedBy string `json:"chaired_by"`

	// Conclusions are IN THE ORDER THE CIRCLES ARE DRAWN (by `thu_tu`, fixed in the store's ORDER
	// BY). Empty is a real state, not an error: §7.3 saves minutes with no conclusions.
	//
	// make(…, 0, …) AND NEVER nil — see the note in the handler on `items`.
	Conclusions []ketLuanRa `json:"conclusions"`

	// TaskCount and TaskDoneCount are the card header's `{x}/{y} nhiệm vụ xong` — §7.5: the SUM over
	// every conclusion of this meeting. Derived here from the conclusions, by domain.TienDoNhiemVu,
	// and stored nowhere (migration 0007 says why there is no counter column).
	TaskCount     int `json:"task_count"`
	TaskDoneCount int `json:"task_done_count"`

	// ConclusionCount and ConclusionDoneCount are the card's MAIN figure since user decision 4
	// (25/09/2026): `x/y kết luận hoàn thành`. Done = the derived status is `hoan-thanh`, which
	// includes a conclusion marked "không phát sinh nhiệm vụ". domain.BienBanHop.TienDoKetLuan.
	ConclusionCount     int `json:"conclusion_count"`
	ConclusionDoneCount int `json:"conclusion_done_count"`

	// Status is `du-thao` or `da-ky` (migration 0012). Signed minutes are locked; a correction is
	// supplementary minutes (`supplements_id` on the correction, `supplemented_by` here).
	Status string `json:"status"`

	// SignedAt and SignedBy record the signing act — absent on a draft. SignedBy is a STAFF BUSINESS
	// CODE (rule 6, invariant 8).
	SignedAt *time.Time `json:"signed_at,omitempty"`
	SignedBy string     `json:"signed_by,omitempty"`

	// MinutesTaker is a STAFF BUSINESS CODE; absent when the meeting named none.
	MinutesTaker string `json:"minutes_taker,omitempty"`

	// Notice is the conclusion notice (Thông báo kết luận), TRANSCRIBED — absent until recorded. The
	// number and the day travel together or not at all (the schema's CHECK).
	Notice *thongBaoKetLuanRa `json:"notice,omitempty"`

	// SupplementsID is the original these supplementary minutes correct — absent on ordinary minutes.
	SupplementsID string `json:"supplements_id,omitempty"`

	// SupplementedBy, Content and Attendees are ON THE DETAIL ROUTE ONLY.
	//
	// ⚠ ABSENT AND EMPTY ARE TWO STATEMENTS, the same split `documents` makes on a task: absent =
	// "this surface (the list) does not carry it"; `[]` / `""` on the detail route = "none recorded".
	// A pointer is what lets the contract say so — a bare slice would be declared a required array
	// by tools/apidoc and be `null` on the list.
	SupplementedBy *[]string `json:"supplemented_by,omitempty"`
	Content        *string   `json:"content,omitempty"`
	Attendees      *[]string `json:"attendees,omitempty"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// thongBaoKetLuanRa is the transcribed reference of the conclusion notice.
type thongBaoKetLuanRa struct {
	ReferenceNo string `json:"reference_no"`
	// IssuedOn is a CALENDAR DAY, `2026-08-12`, like `held_on`.
	IssuedOn string `json:"issued_on"`
}

// ketLuanRa is one numbered conclusion (§2's `①` rows).
type ketLuanRa struct {
	// ID is what §3 sends back as `nguon_id` when a conclusion is split into a task. It is the whole
	// reason the back-link can be traced afterwards (§1).
	ID string `json:"id"`

	// Ordinal is the number in the circle, from 1. IT IS NOT THE ARRAY INDEX: §7.2 appends and never
	// renumbers, so a removed conclusion leaves a gap the screen must show as it is.
	Ordinal int    `json:"ordinal"`
	Content string `json:"content"`

	// The per-conclusion `{x}/{y} nhiệm vụ đã hoàn thành` of §2. `task_count == 0` is the `Chưa tách
	// thành nhiệm vụ nào` line, which is a DIFFERENT statement from `0/3` — see
	// domain.KetLuanHop.ChuaTachNhiemVu. No boolean is sent for it: the client has both numbers and
	// a second representation of one fact is what rule 9 refuses.
	TaskCount     int `json:"task_count"`
	TaskDoneCount int `json:"task_done_count"`

	// Status is DERIVED on every read, never stored (user decision 4; migration 0012 refuses a status
	// column): `chua-giao` · `dang-thuc-hien` · `qua-han` · `hoan-thanh`, precedence and the treatment
	// of `chuyen-tiep`/`tam-dung` on domain.KetLuanHop.TrangThai.
	Status string `json:"status"`

	// NoTask is the human-set mark "không phát sinh nhiệm vụ". When true, Status is `hoan-thanh`; the
	// flag is what tells that `hoan-thanh` apart from "every task finished".
	NoTask bool `json:"no_task"`

	CreatedAt time.Time `json:"created_at"`
}

// ngayHopRa formats the meeting day as a calendar date.
//
// ONE FUNCTION, ONE FORMAT. `2006-01-02` is ISO 8601, which is what a JSON contract carries; the
// `5/8/2026` of §2 is a RENDERING and belongs to the screen (ADR 0017). Doing it in two places is
// how one of them ends up with the browser's time zone in it.
func ngayHopRa(t time.Time) string { return t.Format("2006-01-02") }

// bienBanRaNgoai builds the response.
//
// NO MASKING BRANCH AND NO PERMISSION ARGUMENT, unlike the petition register's equivalent: every
// person named on these minutes is a MEMBER OF STAFF, by business code. The day a citizen's name
// appears on this record, this function needs the branch phieuRaNgoai has — and migration 0007 says
// the same thing about the columns.
func bienBanRaNgoai(b domain.BienBanHop) bienBanRa {
	xong, tong := b.TienDoNhiemVu()
	klXong, klTong := b.TienDoKetLuan()
	ra := bienBanRa{
		ID:                  b.ID,
		Title:               b.TenCuocHop,
		HeldOn:              ngayHopRa(b.NgayHop),
		ReferenceNo:         b.SoHieu,
		Location:            b.DiaDiem,
		ChairedBy:           b.ChuTriMa,
		Conclusions:         make([]ketLuanRa, 0, len(b.KetLuan)),
		TaskCount:           tong,
		TaskDoneCount:       xong,
		ConclusionCount:     klTong,
		ConclusionDoneCount: klXong,
		Status:              b.TrangThai,
		SignedBy:            b.KyBoiMa,
		MinutesTaker:        b.ThuKyMa,
		SupplementsID:       b.BoSungChoID,
		CreatedBy:           b.NguoiTaoMa,
		CreatedAt:           b.TaoLuc,
	}
	// A ZERO time.Time BECOMES AN ABSENT FIELD, never `0001-01-01` on the wire.
	if !b.KyLuc.IsZero() {
		t := b.KyLuc
		ra.SignedAt = &t
	}
	if b.TbSoKyHieu != "" || !b.TbNgay.IsZero() {
		tb := thongBaoKetLuanRa{ReferenceNo: b.TbSoKyHieu}
		if !b.TbNgay.IsZero() {
			tb.IssuedOn = ngayHopRa(b.TbNgay)
		}
		ra.Notice = &tb
	}
	for _, k := range b.KetLuan {
		ra.Conclusions = append(ra.Conclusions, ketLuanRa{
			ID:            k.ID,
			Ordinal:       k.ThuTu,
			Content:       k.NoiDung,
			TaskCount:     k.SoNhiemVu,
			TaskDoneCount: k.SoNhiemVuXong,
			Status:        string(k.TrangThai()),
			NoTask:        k.KhongPhatSinh,
			CreatedAt:     k.TaoLuc,
		})
	}
	return ra
}

// bienBanChiTietRaNgoai is the DETAIL response: the card plus the three fields only this route
// carries, always present (possibly empty) — see the note on SupplementedBy.
func bienBanChiTietRaNgoai(b domain.BienBanHop) bienBanRa {
	ra := bienBanRaNgoai(b)
	noiDung := b.NoiDung
	ra.Content = &noiDung
	thanhPhan := b.ThanhPhan
	if thanhPhan == nil {
		thanhPhan = []string{}
	}
	ra.Attendees = &thanhPhan
	boSung := b.DuocBoSungBoi
	if boSung == nil {
		boSung = []string{}
	}
	ra.SupplementedBy = &boSung
	return ra
}

// DanhSachBienBan serves one page of the commune's meeting minutes. GET /api/v1/meetings
//
// # IT TAKES NO FILTER, AND THAT IS THE SPECIFICATION RATHER THAN AN OMISSION
//
// §2 draws one vertical list of cards, newest first, with no filter bar, no search box and no tabs.
// Accepting a parameter the screen does not offer would be a surface nobody asked for and nobody
// tests; the page cursor is the only thing this route reads from the query string.
//
// # NO AUDIT ENTRY
//
// Rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes. This is
// neither: these minutes carry no citizen personal data, and the query cannot leave the commune the
// request arrived in. An audit ledger that grew a row per screen opened would bury the disclosures
// it exists to make findable.
func (h *Handler) DanhSachBienBan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING HERE READS `tenant_id`
	// FROM THE QUERY STRING — a client naming its own commune is a client granting itself access
	// (rule 1, forbidden #2).
	yc, err := page.Parse(r.URL.Query(), petstore.SapXepBienBan)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	kq, err := h.d.DanhSachBienBan.DanhSach(ctx, yc)
	if errors.Is(err, page.ErrCursor) {
		// The `held_on` cursor carries a composite key the STORE decodes (the meeting day must be
		// bound as a DATE — see petstore.SapXepBienBan), so a malformed one surfaces here rather than
		// in page.Parse. Same answer as any other bad cursor, same refusal to echo it.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}
	if err != nil {
		// The wrapped error carries the store failure. IT DOES NOT REACH THE CLIENT — and on this
		// route that is more than a habit: the statement behind it quotes no row, but an error text
		// from a register whose columns hold minutes would be the easiest way for a case to reach a
		// log (rule 3, forbidden #3).
		h.d.Log.Error("danh sách biên bản họp: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// page.Result[T] DIRECTLY — tools/apidoc understands it, so there is no second three-field struct
	// copying it and no way for the two to drift.
	//
	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune that has
	// recorded no meeting, never as null. A newly onboarded commune has exactly that, and a client
	// that has to handle both shapes handles one of them wrong. The same holds for `conclusions` on
	// every card — see bienBanRaNgoai.
	ra := page.Result[bienBanRa]{
		Items:      make([]bienBanRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, b := range kq.Items {
		ra.Items = append(ra.Items, bienBanRaNgoai(b))
	}
	vietJSON(w, http.StatusOK, ra)
}

// QuyenDocBienBan guards the read route. It is `task.read` — the key seeded at
// service-identity/migrations/0001_init.sql:304 — and NOT a `meeting.*` key, which does not exist in
// the `quyen` table and is not invented here (rule 5, invariant 3c). See the note at the top.
//
// It is a constant because the handler's own reasoning refers to it; the ROUTE declaration in
// routes.go spells the key out as a literal, because tools/apidoc refuses anything there that is not
// one.
const QuyenDocBienBan authz.Perm = "task.read"

// DocBienBan serves one meeting with everything. GET /api/v1/meetings/{id}
//
// ONE 404 BODY FOR THREE CAUSES — unknown id, another commune's id, soft-deleted minutes — because
// the store answers ErrBienBanKhongTonTai for all three and this handler has one branch for it.
//
// NO AUDIT ENTRY, for DanhSachBienBan's reason.
func (h *Handler) DocBienBan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		h.khongTimThayBienBan(w)
		return
	}

	b, err := h.d.DanhSachBienBan.TheoID(ctx, id)
	if errors.Is(err, petstore.ErrBienBanKhongTonTai) {
		h.khongTimThayBienBan(w)
		return
	}
	if err != nil {
		// Includes petstore.ErrQuaNhieuBoSung — refused, not truncated. The wrapped error never
		// reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("đọc một biên bản họp: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, bienBanChiTietRaNgoai(b))
}

// khongTimThayBienBan is THE ONE 404 of the meeting read routes — the same sentence the write routes
// answer (traLoiLoiBienBan), so a caller cannot tell which door told it "no".
func (h *Handler) khongTimThayBienBan(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy biên bản họp này.", "")
}

// nhiemVuKetLuanRa is the body of GET /api/v1/meetings/{id}/conclusions/{stt}/tasks.
//
// THE ITEMS ARE THE TASK REGISTER'S OWN ROW SHAPE (nhiemVuRa, built by nhiemVuRaNgoai) — one row
// representation for one record, whichever list it appears in. That shape deliberately carries NO
// `overdue` field: overdue is derived by the client from `due_at` / `completed_at`, the reason is on
// nhiemVuRa, and a second representation here would be the first place the two disagree.
//
// NOT PAGINATED — the whole list, bounded by petstore.TranNhiemVuMotKetLuan (see the route).
type nhiemVuKetLuanRa struct {
	Items []nhiemVuRa `json:"items"`
}

// NhiemVuCuaKetLuan serves the live tasks split from one conclusion.
// GET /api/v1/meetings/{id}/conclusions/{stt}/tasks
func (h *Handler) NhiemVuCuaKetLuan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Same parse and same sentence as the split route next door.
	thuTu, err := strconv.Atoi(r.PathValue("stt"))
	if err != nil || thuTu < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Số thứ tự kết luận phải là một số nguyên dương.", "")
		return
	}

	ds, err := h.d.DanhSachBienBan.NhiemVuCuaKetLuan(ctx, r.PathValue("id"), thuTu)
	if errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
		// ONE ANSWER for an unknown / other-commune / removed meeting and an unknown / removed
		// conclusion — the split route's sentence.
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy kết luận này trong biên bản.", "")
		return
	}
	if err != nil {
		if errors.Is(err, petstore.ErrQuaNhieuNhiemVuKetLuan) {
			h.d.Log.Error("nhiệm vụ của một kết luận vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", petstore.TranNhiemVuMotKetLuan)
		} else {
			h.d.Log.Error("nhiệm vụ của một kết luận: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(…, 0, …): a conclusion nobody has split answers `"items":[]`, never null.
	ra := nhiemVuKetLuanRa{Items: make([]nhiemVuRa, 0, len(ds))}
	for _, n := range ds {
		ra.Items = append(ra.Items, nhiemVuRaNgoai(n))
	}
	vietJSON(w, http.StatusOK, ra)
}
