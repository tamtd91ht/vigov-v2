package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The READ and WRITE routes behind Cấu hình → Thời hạn xử lý (14-cau-hinh.md §8).
//
//	GET   /api/v1/sla            the commune's whole deadline table, with its problems
//	PATCH /api/v1/sla/{id}       change the five figures of one row
//	POST  /api/v1/sla/defaults   sow the rows a commune that has none needs
//
// ALL THREE DECLARE `admin.sla`. The key already exists in the `quyen` table (migration 0001:277,
// "Cấu hình thời hạn xử lý") and ADR 0029 §"Một bằng chứng phụ" reads its presence as evidence that
// configuring deadlines was understood as ONE job of ONE person before the question was asked.
// Nothing here invents a key — rule 5, invariant 3c: a key no migration seeds is a right no
// administrator can grant, so the route would answer 403 to every account forever while its tests
// stayed green.
//
// =================================================================================================
// THE READ IS `admin.sla` AND NOT AnyAuthenticated — DELIBERATELY UNLIKE THE THREE CALENDAR READS.
//
// Those three are AnyAuthenticated because office hours and holidays sit under every deadline
// PRINTED ON A SCREEN — the due date on a task, the "còn mấy ngày" chip — so a configuration
// permission there would empty those screens for everybody who is not an administrator. That
// argument does not transfer, and the difference is not cosmetic:
//
//	THE CALENDAR IS READ TO RENDER.   Every screen showing a date needs it.
//	THIS TABLE IS READ TO CONFIGURE.  No screen renders `gio_xu_ly_xong`; screens render the STORED
//	                                  deadline on the record (rule 10, invariant 2), which comes
//	                                  from the record, not from here. The one consumer that needs
//	                                  the figures is another SERVICE, and it does not come through
//	                                  HTTP — it calls grpc ResolveDeadlines (ADR 0029 §Bổ sung).
//
// So restricting the read breaks no screen, and 14-cau-hinh.md:394 says the same thing from the
// other side: the tab is hidden from accounts without the key. A wider grant would have to be
// justified by a screen that needs it, and there is none.
//
// =================================================================================================
// ⚠ THE URL RESOURCE NAME `sla` IS NOT ONE THE USER HAS APPROVED — STATED, NOT GLOSSED.
//
// kb/00-foundation/ubiquitous-language.md:296 says of this very table: *"Chưa có tài nguyên URL, vì
// chưa có tuyến nào: đừng điền sẵn một cái tên"*, and ADR 0011 makes the URL noun the user's
// decision rather than a session's translation. `sla` is used here because the task that
// commissioned these routes named it; it is NOT a settled noun, and two things are wrong with it on
// the conventions this repository already follows:
//
//	it is an ACRONYM, where every other resource is a word (`working-hours`, `public-holidays`);
//	it is SINGULAR, where the convention is plural, and it does not match the entity name the
//	migration already carries — `-- @entity: ProcessingDeadline` (0008), which would give
//	`processing-deadlines`.
//
// RENAMING LATER IS CHEAP HERE AND ONLY HERE: no migration mark mentions it (unlike
// `public-holidays`, which is welded to an APPLIED migration's `@entity` and therefore could not be
// renamed — ubiquitous-language.md:303). It is a path string in this file plus the generated
// contract. **This needs the user's decision before the contract is published to the admin web.**
//
// =================================================================================================
// WHAT IS DELIBERATELY NOT HERE — absences that are findings, not omissions.
//
//	`+ Thêm thời hạn cho một lĩnh vực`   14-cau-hinh.md:293. A route that lets a commune name a
//	                                     FIELD must validate that code against the closed tier-1
//	                                     code set in service `platform`, and whether that read is
//	                                     gRPC or an event-fed replica HAS NO ADR — writing it
//	                                     without one is ADR 0026 stop condition #2. The two routes
//	                                     here accept no field code at all (see app.YeuCauSuaSLA),
//	                                     which is exactly why they are not that stop condition.
//	DELETING A ROW                       no screen specifies it, and an SLA row is the basis of
//	                                     commitments already issued (migration 0008 §RETENTION).
//	AN APPROVAL STEP                     ADR 0029 stop condition #4: `admin.sla` answers WHO may
//	                                     press the button, but nobody has asked the customer whether
//	                                     changing a figure needs a leader's sign-off. These routes
//	                                     apply the change immediately. If the answer turns out to be
//	                                     "yes", this is a new state on the write path, not a tweak.

// thanSLAToiDa bounds the request body. 4 KiB is far past five integers and far short of anything
// worth streaming: an unbounded body is memory a client chooses, on a process serving 200+ communes.
const thanSLAToiDa = 4 << 10

// dongSLARa is one deadline row as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): every value is a count of working hours in a commune's
// published policy, and a field code from a platform-level set.
//
// THE FIVE FIGURE NAMES CARRY `_hours`, ON EVERY ONE OF THEM. The unit is the single most
// consequential fact about these numbers (ADR 0007, rule 10, invariant 4) and a client that reads
// `acknowledge` as days widens or narrows a commitment by a factor of eight with nothing erroring.
// The security field really is 2 WORKING hours.
type dongSLARa struct {
	ID string `json:"id"` // ULID — what PATCH /api/v1/sla/{id} references

	// WorkKind is the raw `loai_viec` value: `van-ban-den` · `phan-anh` · `nhiem-vu`. The same
	// string the CHECK constraint admits, so contract and schema cannot drift into two spellings.
	WorkKind string `json:"work_kind"`

	// Field is the tier-1 field code, EMPTY for the default row.
	//
	// "" AND NOT null, with IsDefault beside it carrying the meaning. A client that has to read
	// `field === null` as "applies to everything" is a client one `??` away from rendering the
	// default row as a field called "null" — and 14-cau-hinh.md:308 is this specification's own
	// evidence that raw codes do reach the screen.
	Field string `json:"field"`

	// IsDefault is `linh_vuc IS NULL` — the row every field without one of its own falls back to,
	// and the row ADR 0028 decision E reads for every petition a citizen sends.
	IsDefault bool `json:"is_default"`

	// THE FIVE FIGURES. All counts of WORKING hours, never wall-clock, never days.
	//
	// AcknowledgeHours fixes `han_tiep_nhan` at the act that CREATES a record; ResolveHours fixes
	// `han_xu_ly_xong` at the act that SETTLES THE FIELD (ADR 0028). DueSoonHours is hours
	// REMAINING at which "sắp đến hạn" begins — it drives the reminder, the filter and the bell
	// figure at once (14-cau-hinh.md §8).
	AcknowledgeHours int `json:"acknowledge_hours"`
	ResolveHours     int `json:"resolve_hours"`
	DueSoonHours     int `json:"due_soon_hours"`

	// EscalateLeaderHours and EscalatePresidentHours ARE RETURNED WITH NO AGREED ANCHOR. §8 heads
	// them "sau 24 giờ" without saying after what; §9's job counts from the deadline being missed
	// and DOUBLES the figure for the president instead of reading a second column. They are carried
	// because the screen edits them; NOTHING MAY COMPUTE AN ESCALATION FROM THEM until somebody
	// answers "after what" (migration 0008; ADR 0029).
	EscalateLeaderHours    int `json:"escalate_leader_hours"`
	EscalatePresidentHours int `json:"escalate_president_hours"`
}

// vanDeSLARa is one problem found in the table THIS RESPONSE CARRIES.
//
// SAME SHAPE AND SAME REASONING AS vanDeLichRa: derived on every read, never stored (rule 10,
// invariant 3), and the route still answers 200 because the screen that FIXES the problem is the
// one asking. Refusal belongs where a DEADLINE IS COMPUTED — grpc.ResolveDeadlines answers
// FAILED_PRECONDITION, and `service-documents` turns that into 409 `sla_chua_cau_hinh`. That
// refusal is correct and this route must not soften it.
type vanDeSLARa struct {
	// Kind is a stable English identifier a client branches on: `empty_sla` or
	// `missing_default_row`.
	Kind string `json:"kind"`

	// WorkKind is the kind of work the problem sits on, "" when it is about the whole table.
	WorkKind string `json:"work_kind"`

	// Message is the Vietnamese sentence the screen shows. It names no person (rule 3).
	Message string `json:"message"`
}

// danhSachSLARa wraps the list in an OBJECT rather than a bare JSON array — same reasoning as the
// calendar, and the same absence of paging: the table is only correct whole, because answering
// "what is the deadline for this field" needs the field's row AND the default row it falls back on
// (domain.DongTheoLinhVuc), and a page cannot promise to hold both.
type danhSachSLARa struct {
	Items    []dongSLARa  `json:"items"`
	Problems []vanDeSLARa `json:"problems"`
}

func dongSLARaNgoai(d domain.DongSLA) dongSLARa {
	return dongSLARa{
		ID:                     d.ID,
		WorkKind:               string(d.LoaiViec),
		Field:                  d.LinhVuc,
		IsDefault:              d.LaDongMacDinh(),
		AcknowledgeHours:       d.GioTiepNhan,
		ResolveHours:           d.GioXuLyXong,
		DueSoonHours:           d.GioSapDenHan,
		EscalateLeaderHours:    d.GioBaoLanhDao,
		EscalatePresidentHours: d.GioBaoChuTich,
	}
}

func vanDeSLARaNgoai(v domain.VanDeSLA) vanDeSLARa {
	ra := vanDeSLARa{Kind: string(v.Loai), WorkKind: string(v.LoaiViec)}
	switch v.Loai {
	case domain.VanDeSLATrong:
		// THE SENTENCE NAMES THE BUTTON, because this is the one problem with a one-click answer and
		// the commune is otherwise in a closed loop: no deadline can be computed, so no petition and
		// no incoming document can be registered at all.
		ra.Message = "Xã chưa cấu hình thời hạn xử lý nào. Chưa tiếp nhận được văn bản đến hay " +
			"phản ánh cho tới khi có cấu hình — hãy bấm “Gieo thời hạn mặc định” rồi sửa lại các " +
			"con số cho đúng với xã."
	case domain.VanDeThieuDongMacDinh:
		ra.Message = "Loại việc “" + string(v.LoaiViec) + "” có dòng riêng nhưng thiếu dòng mặc " +
			"định, nên lĩnh vực nào không có dòng riêng sẽ không tính được hạn."
	default:
		// A kind was added to the domain and not here. The identifier still travels, so the client
		// is not left guessing what the empty sentence meant.
		ra.Message = "Cấu hình thời hạn xử lý có vấn đề chưa được mô tả: " + string(v.Loai)
	}
	return ra
}

// DanhSachSLA serves the commune's deadline table. GET /api/v1/sla
//
// NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes.
// This is neither — a commune's own published policy, read inside the commune the request arrived
// in.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachSLA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ds, err := h.d.SLA.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, idstore.ErrQuaNhieuDongSLA) {
			// REFUSED, NOT TRUNCATED, and the cost of the alternative is specific here: a truncated
			// read that dropped a field's own row does not fail — DongTheoLinhVuc falls back to the
			// default row and QUIETLY answers with a different promise than the commune made.
			h.d.Log.Error("bảng thời hạn xử lý vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranSLA)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("bảng thời hạn xử lý: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) on BOTH lists: `items` and `problems` must marshal as [] and never as null.
	ra := danhSachSLARa{
		Items:    make([]dongSLARa, 0, len(ds)),
		Problems: make([]vanDeSLARa, 0, 2),
	}
	for _, mot := range ds {
		ra.Items = append(ra.Items, dongSLARaNgoai(mot))
	}
	// COMPUTED FROM THE ROWS THIS RESPONSE CARRIES, in this same handler, so the two halves cannot
	// describe two different instants. The rule lives in the domain package because it is a property
	// of a deadline table, not of HTTP — grpc.ResolveDeadlines needs the same rule.
	for _, vd := range domain.VanDeCuaSLA(ds) {
		ra.Problems = append(ra.Problems, vanDeSLARaNgoai(vd))
	}

	// WARN, once per read, because an empty table is the state in which this commune can be promised
	// nothing: every entry into the document register and every petition is refused until it is
	// filled. It is NOT an error and must not become one, or the screen that fixes it could not load.
	if len(ds) == 0 {
		h.d.Log.Warn("xã chưa cấu hình thời hạn xử lý — mọi tuyến vào sổ và tiếp nhận đang bị từ chối",
			"xa", string(tenant.MustFrom(ctx)))
	}
	vietJSON(w, http.StatusOK, ra)
}

// suaSLAVao is a PARTIAL edit: an ABSENT figure means "leave it alone", which is why every field is
// a pointer.
//
// WITH PLAIN INTS THIS COULD NOT TELL "not mentioned" FROM "sent zero", and zero is refused by both
// domain.KiemTraGio and the CHECK constraint — so a screen editing one figure would send four zeros
// and have the whole edit refused. See app.YeuCauSuaSLA.
//
// THERE IS NO `work_kind` AND NO `field`. What a row applies to is fixed when the row is created:
// a field here would let a screen silently re-point an existing commitment, and it is the parameter
// that would need the write-time code check ADR 0026 stop condition #2 governs.
type suaSLAVao struct {
	AcknowledgeHours       *int `json:"acknowledge_hours"`
	ResolveHours           *int `json:"resolve_hours"`
	DueSoonHours           *int `json:"due_soon_hours"`
	EscalateLeaderHours    *int `json:"escalate_leader_hours"`
	EscalatePresidentHours *int `json:"escalate_president_hours"`
}

// SuaSLA changes the figures of one row. PATCH /api/v1/sla/{id}
//
// IT CHANGES NO DEADLINE ALREADY ISSUED, and that is the specification's own sentence at
// 14-cau-hinh.md:289 — *"Thay đổi chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu"* — as well as
// rule 10, invariant 2. Nothing in this service can reach `han_tiep_nhan` or `han_xu_ly_xong`:
// those columns live on records in `service-petitions` and `service-documents`, fixed once at the
// act that set them and stored. No code anywhere recomputes them from this table.
func (h *Handler) SuaSLA(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}

	var than suaSLAVao
	r.Body = http.MaxBytesReader(w, r.Body, thanSLAToiDa)
	if err := json.NewDecoder(r.Body).Decode(&than); err != nil {
		// The decoder's own message quotes the offending input and is not returned (rule 3,
		// forbidden #3). The sentence says what to fix without repeating what was sent.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}

	// EVERY FIGURE ABSENT IS REFUSED RATHER THAN TREATED AS A NO-OP. `{}` means the client sent a
	// form it failed to read, and answering 200 would tell the person their edit was saved.
	if than.AcknowledgeHours == nil && than.ResolveHours == nil && than.DueSoonHours == nil &&
		than.EscalateLeaderHours == nil && than.EscalatePresidentHours == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Không có số giờ nào được gửi lên để sửa.", "")
		return
	}

	d, err := h.d.GhiSLA.Sua(r.Context(), r.PathValue("id"), app.YeuCauSuaSLA{
		GioTiepNhan:   than.AcknowledgeHours,
		GioXuLyXong:   than.ResolveHours,
		GioSapDenHan:  than.DueSoonHours,
		GioBaoLanhDao: than.EscalateLeaderHours,
		GioBaoChuTich: than.EscalatePresidentHours,
	}, nguoi)
	if err != nil {
		h.traLoiLoiGhiSLA(w, r, "sửa thời hạn xử lý", err)
		return
	}
	vietJSON(w, http.StatusOK, dongSLARaNgoai(d))
}

// gieoSLARa is what one seeding run did. See app.KetQuaGieo for why two counts and not a boolean.
type gieoSLARa struct {
	// Seeded is how many rows this request actually wrote.
	Seeded int `json:"seeded"`

	// Kept is how many rows the commune already had AND WHICH WERE LEFT EXACTLY AS THEY WERE,
	// including any figure the commune had edited. The name is `kept` rather than `skipped` on
	// purpose: the screen has to be able to say "chúng tôi không đụng vào 15 dòng của xã".
	Kept int `json:"kept"`
}

// GieoSLAMacDinh sows the rows a commune without a configured table needs.
// POST /api/v1/sla/defaults
//
// THE ROWS ARE THE SPECIFICATION'S OWN TABLE (14-cau-hinh.md §8), MINUS THE DEAD CODE AT :308 —
// domain.BoGieoSLA owns that list and the argument for it, including why it is not rule 10,
// forbidden #3. Not restated here: a second copy would drift (rule 9).
//
// IT NEVER OVERWRITES. Pressing it a second time writes no duplicate row and changes no figure the
// commune edited — the property is proved against a transaction in app/sla_test.go, because it is
// the one that a plausible implementation (an UPSERT) silently breaks.
//
// 200 AND NOT 201, INCLUDING ON THE FIRST RUN. The request creates no single addressable resource —
// it brings a commune's existing configuration table to a known state, and the response body says
// how much of it moved. A 201 would owe a Location, and there is no one URL to point at.
func (h *Handler) GieoSLAMacDinh(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}

	kq, err := h.d.GhiSLA.GieoMacDinh(r.Context(), nguoi)
	if err != nil {
		h.traLoiLoiGhiSLA(w, r, "gieo thời hạn xử lý mặc định", err)
		return
	}
	vietJSON(w, http.StatusOK, gieoSLARa{Seeded: kq.DaGieo, Kept: kq.DaCo})
}

// traLoiLoiGhiSLA maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR BOTH ROUTES: two copies of this mapping would drift, and the copy that drifts
// answers 500 where it meant 409 — which reads to an operator as a broken server rather than as a
// rule doing its job.
func (h *Handler) traLoiLoiGhiSLA(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, idstore.ErrDongSLAKhongTonTai):
		// 404 FOR ANOTHER COMMUNE'S ID, NOT 403. Every statement is scoped, so the id matches no
		// row; an invented id, a soft-deleted row and another authority's row are one single answer,
		// so none can be told apart by trying (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "sla_row_not_found",
			"Không tìm thấy dòng thời hạn xử lý này.", "")

	case errors.Is(err, idstore.ErrDongSLATrungKhoa):
		// 409 AND NOT 500: two administrators pressed the seed button at the same instant and this
		// transaction lost the race. Nothing is wrong and nothing was half-written — the winner's
		// rows are there. Retrying answers `seeded: 0`, which is the correct final state.
		httpx.WriteError(w, http.StatusConflict, "sla_row_exists",
			"Cấu hình thời hạn của xã vừa được người khác thay đổi. Hãy tải lại trang rồi thử lại.", "")

	case errors.Is(err, idstore.ErrLoaiViecLa):
		// 500: the CHECK constraint should have made this impossible (sla_loai_viec_hop_le), so
		// reaching it means the database is not the schema this code was built against. It is an
		// operator's problem, not the caller's, and the client is told nothing about the row.
		h.d.Log.Error("bảng thời hạn xử lý có loại việc lạ — CHECK sla_loai_viec_hop_le không còn hiệu lực",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")

	case app.LaLoiDauVaoSLA(err):
		// The domain's own sentence is returned: it names the rule, holds no personal data and no
		// internal detail, and a second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")

	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is what an operator can act on.
		h.d.Log.Error("ghi thời hạn xử lý: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
