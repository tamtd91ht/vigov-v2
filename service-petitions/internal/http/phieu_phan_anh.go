package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The staff read path for one petition. GET /api/v1/citizen-reports/{maTraCuu}
//
// THERE IS NO WRITE ROUTE IN THIS FILE, and the absence is three separate unanswered questions
// rather than unfinished work. Each one is named where it bites, in routes.go.

// phieuPhanAnhRa is one petition as it leaves the API to a member of staff.
//
// PERSONAL DATA ON IT IS MASKED BY DEFAULT, AND THE DEFAULT IS STILL THE DECISION (rule 3,
// invariant 3): anything leaving the API is masked UNLESS the caller holds an EXPLICIT full-view
// permission. That permission now exists — `feedback.unmask`, seeded 2026-09-20 by
// service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql and decided by ADR 0030 —
// so the exception is a key a commune administrator ticked, never an assumption made here.
//
// THE COST THIS CLOSES, NAMED BY THE ADR ITSELF: with no such key there were only two states,
// mask everything or open everything, so the register masked unconditionally and AN OFFICER
// COULD NOT RING THE REPORTER BACK. That was fail-closed working as intended while the customer
// decided, and ADR 0030 is them deciding.
//
// WHAT THE KEY DOES NOT BUY — read before widening it:
//
//	AN ANONYMOUS PETITION STAYS ANONYMOUS. `feedback.unmask` removes MASKING; it does not
//	override `an_danh`. Those are two different promises: masking is a precaution the system
//	takes, anonymity is a choice the citizen made and docs/ui-ux/09 §348 says it hides the name
//	and number from EVERY interface. Reversing a citizen's own choice is a customer decision
//	nobody has made, so this route does not make it either (rule 3, stop condition #1).
//
//	IT IS NOT `feedback.restricted`. That key is a CONTENT scope — the `can-bo` field — and it
//	is checked separately below. Holding one has never implied the other (rule 5, invariant 3b).
//
// EVERY DISCLOSURE IS AUDITED, IN THE SAME REQUEST, BEFORE THE BODY IS WRITTEN — rule 6,
// invariant 7 and ADR 0030 stop condition #4. See DocPhieuPhanAnh.
//
// THE CONTENT ITSELF IS NOT MASKED, and that is not an inconsistency. `noi_dung` is what the
// commune has to act on — an officer who cannot read the report cannot process it, and there is
// no partial version of a sentence that stays useful. What masking protects is the IDENTIFIERS
// that tie a report to a named person, which is what turns a leaked response into a list of
// citizens.
//
// THERE IS NO `overdue` FIELD ON THIS RESPONSE, AND ITS ABSENCE IS THE DESIGN — read this before
// adding one back, because it looks like a convenience the screen is missing.
//
// Overdue is DERIVED from a deadline compared with the current instant (rule 10, invariant 3).
// Sending it alongside the deadline would put TWO REPRESENTATIONS OF ONE FACT on the wire
// (rule 9), and they do not stay in step: the boolean is frozen at the instant the response was
// built, while the chip that shows it is rendered later and stays on screen. The client has the
// deadline, so it can compute the state — and it needs to anyway, because the screen shows
// "Quá hạn 3 ngày" (docs/ui-ux/09 §8.3), a duration the boolean cannot express.
//
// The canonical derivation for anything computed INSIDE this service — filters, reports,
// reminders — is domain.PhieuPhanAnh.QuaHan / QuaHanTiepNhan. A second one is a second answer.
type phieuPhanAnhRa struct {
	// Code is `ma_tra_cuu` — the string the citizen was handed. Not `id`: the internal ULID
	// means nothing outside this service, and every screen and every audit entry names the
	// petition by this code.
	Code string `json:"code"`

	Channel string `json:"channel"` // `zalo-mini-app` | `zalo-oa` | `web-xa` | `can-bo-nhap-ho`
	Status  string `json:"status"`  // one of the nine (ADR 0027)

	// Field is the tier-1 code, EMPTY while the petition is unclassified. It is a code and not a
	// label; see FieldLabel.
	Field string `json:"field"`

	// FieldLabel is the commune's own wording, and it is EMPTY WHEN THE COMMUNE HAS NOT RENAMED
	// THE CODE — which is the ordinary case, not an error.
	//
	// IT IS NOT FILLED IN WITH A DEFAULT HERE, and that is the honest shape today: the default
	// label belongs to the tier-1 code set in service `platform`, and how `petitions` reads that
	// set — live gRPC or an event-fed replica — has no ADR yet (ADR 0026 §Bổ sung). A client
	// showing the raw code when this is empty is showing the truth about what this service
	// currently knows.
	//
	// `label` AND NOT `name`: the column is `nhan`, a LABEL — the one thing a commune may
	// re-word while `ma` stays immutable. kb/00-foundation/ubiquitous-language.md owns the rule.
	FieldLabel string `json:"field_label"`

	Content string `json:"content"`
	Address string `json:"address"`

	// ReporterName and ReporterPhone are MASKED — "Nguyễn V. A." and "09****5678" — unless the
	// caller holds `feedback.unmask`, in which case they carry the real values and the request
	// has already written an audit entry for the disclosure (rule 6, invariant 7).
	//
	// THEY ARE EMPTY, BOTH OF THEM, WHEN THE CITIZEN FILED ANONYMOUSLY — with or without the key.
	// Anonymous means hidden from the staff screen; the identity is still recorded in the
	// database (ADR 0008), and opening THAT is a different decision nobody has made.
	//
	// THERE IS NO `reporter_unmasked` FLAG BESIDE THEM, and that absence is deliberate for the
	// same reason there is no `overdue`: the client already holds the answer. A masked number
	// carries `*`, a full one does not, so a boolean would be a second representation of a fact
	// already on the wire (rule 9) — and the one that goes stale is the one a screen renders.
	ReporterName  string `json:"reporter_name"`
	ReporterPhone string `json:"reporter_phone"`
	Anonymous     bool   `json:"anonymous"`

	// ClockFrom is `goc_dem_han`: the instant BOTH deadlines were counted from — when the
	// citizen pressed send (ADR 0027, decision D). It is on the response because without it the
	// two deadlines below cannot be explained to anybody, including an inspection.
	ClockFrom time.Time `json:"clock_from"`
	BookedAt  time.Time `json:"booked_at"`

	// AcknowledgeDue is `han_tiep_nhan`. NULL ON THE WIRE MEANS "KHÔNG ÁP DỤNG" — a staff-booked
	// petition, where the officer IS the reader, so the interval does not exist. IT IS NOT ZERO
	// AND MUST NEVER BE RENDERED AS ZERO: a zero is a valid sample, and a commune that books
	// many petitions would report an average acknowledge time near nothing (ADR 0028).
	AcknowledgeDue *time.Time `json:"acknowledge_due"`

	// ResolveDue is `han_xu_ly_xong`. NULL HERE MEANS THE OPPOSITE: "CHƯA CÓ" — the petition has
	// not been classified, so no resolve commitment has been made yet. A screen must NOT invent
	// a date to show; on the citizen channel the correct sentence is "sẽ được xem trong N giờ
	// làm việc", from AcknowledgeDue.
	ResolveDue *time.Time `json:"resolve_due"`

	// ClassifyDue is `han_phan_loai` — the CLASSIFICATION CEILING of ADR 0035 §C. NULL means "KHÔNG
	// ÁP DỤNG", the same meaning AcknowledgeDue's NULL carries and for the same reason: a
	// staff-booked petition arrives classified.
	//
	// IT IS ON THE STAFF RESPONSE AND NOT ON THE CITIZEN ONE. It is an internal management figure —
	// how long the office may take to read a report — and it is not a commitment anybody made to the
	// person waiting. Showing a citizen two different deadlines for two internal steps would invite
	// exactly the question the commune cannot answer usefully.
	ClassifyDue *time.Time `json:"classify_due"`

	// Unit and Assignee are `bo_phan_id` and `can_bo_xu_ly_id` — the "ĐANG GIAO CHO" box of
	// docs/ui-ux/09 §8.3. Both are empty until the petition is assigned.
	//
	// STAFF SURFACE ONLY. phieuCuaToiRa deliberately omits both: naming the officer on a citizen's
	// screen is routing history in its smallest form and invites pressure on an individual over a
	// report they did not choose to receive (rule 4, forbidden #5).
	Unit     string `json:"unit"`
	Assignee string `json:"assignee"`

	// Result is `ket_qua_xu_ly`, empty until the petition is closed. It is written FOR the citizen
	// (rule 10, invariant 6) and is on this response so an officer can read back what the commune
	// told them.
	Result string `json:"result"`

	Public bool `json:"public"`
}

// phieuRaNgoai builds the response. `xemDayDu` is the ONLY switch between masked and full, and it
// is a decision the caller has already paid for: DocPhieuPhanAnh sets it true only after the
// permission was checked AND the audit entry was committed.
//
// IT IS A PARAMETER RATHER THAN A LOOKUP IN HERE. This function has no context, no Checker and no
// way to write a trail, so it cannot accidentally grow a path that unmasks without one.
func phieuRaNgoai(p domain.PhieuPhanAnh, nhan string, xemDayDu bool) phieuPhanAnhRa {
	ra := phieuPhanAnhRa{
		Code:       p.MaTraCuu,
		Channel:    string(p.Kenh),
		Status:     string(p.TrangThai),
		Field:      p.LinhVuc,
		FieldLabel: nhan,
		Content:    p.NoiDung,
		Address:    p.DiaChi,
		Anonymous:  p.AnDanh,
		ClockFrom:  p.GocDemHan,
		BookedAt:   p.VaoSoLuc,
		Unit:       p.BoPhanID,
		Assignee:   p.CanBoXuLyID,
		Result:     p.KetQuaXuLy,
		Public:     p.HienCongKhai,
	}

	// An anonymous petition carries NEITHER field, not a masked one and not a full one. A masked
	// name is still a name: "Nguyễn V. A." in a commune of a few thousand people identifies
	// somebody, and the whole point of the flag is that the officer handling it does not know who
	// filed it. `xemDayDu` is deliberately not consulted on this branch — see the type's doc.
	if !p.AnDanh {
		if xemDayDu {
			ra.ReporterName = p.NguoiGuiHoTen
			ra.ReporterPhone = p.NguoiGuiDienThoai
		} else {
			ra.ReporterName = privacy.MaskName(p.NguoiGuiHoTen)
			ra.ReporterPhone = privacy.MaskPhone(p.NguoiGuiDienThoai)
		}
	}

	// The two NULLs are produced from the two predicates that NAME which question is being
	// asked, never from a bare IsZero() at a call site.
	if !p.HanTiepNhanKhongApDung() {
		t := p.HanTiepNhan
		ra.AcknowledgeDue = &t
	}
	if !p.ChuaChotHanXuLy() {
		t := p.HanXuLyXong
		ra.ResolveDue = &t
	}
	if !p.TranPhanLoaiKhongApDung() {
		t := p.HanPhanLoai
		ra.ClassifyDue = &t
	}
	return ra
}

// LinhVucHanChe is the one field code whose petitions are not visible to every reader.
//
// IT IS NOW AN ALIAS AND NOT A LITERAL, and the move is deliberate: the LIST route has to exclude
// these petitions in the WHERE clause rather than after the page comes back, so `internal/store`
// needs the same value, and a store may not import a handler package. The value and the full
// reasoning live at domain.LinhVucHanChe; this line exists so the handlers below read the same
// short name they always did, from one source (rule 9, invariant 2).
const LinhVucHanChe = domain.LinhVucHanChe

// QuyenHanChe opens it. The key already exists in `quyen`
// (service-identity/migrations/0001_init.sql:294, "Xem phản ánh về tác phong cán bộ") — it is
// not invented here.
const QuyenHanChe authz.Perm = "feedback.restricted"

// QuyenXemDayDu opens the reporter's real name and phone number, in EVERY field.
//
// The key exists in `quyen` — service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql
// seeded it on 2026-09-20, the customer decided it in ADR 0030 — and it is NOT invented here. The
// string is copied from that migration character for character: rule 5, invariant 3b makes the
// key the same flat string the Phân quyền screen shows and the `quyen` table stores, so a
// spelling that differs by one character is a tick box that grants nothing and a route nobody can
// open, with no test red anywhere.
//
// IT IS NOT DERIVED FROM QuyenHanChe AND MUST NEVER BE. `feedback.restricted` is a scope over
// CONTENT (the `can-bo` field); this is a scope over PERSONAL DATA, in every field. ADR 0030
// spells out the direction that makes the difference concrete: somebody verifying a complaint
// about a colleague does not thereby need the phone number of everybody who reported fly-tipping.
const QuyenXemDayDu authz.Perm = "feedback.unmask"

// DocPhieuPhanAnh serves one petition to a member of staff.
// GET /api/v1/citizen-reports/{maTraCuu}
//
// AN AUDIT ENTRY IS WRITTEN ON EXACTLY ONE BRANCH OF THIS ROUTE, and the boundary is rule 6,
// invariant 7: what is audited is reading FULL personal data and reading ACROSS communes. An
// ordinary masked read is neither — the contact details are masked on the way out and the query
// cannot leave the commune the request arrived in — so it writes nothing, and an audit ledger
// that grew a row per screen opened would bury the disclosures it exists to make findable.
//
// A read by a holder of `feedback.unmask` IS the first of those two, so it writes one entry.
// (This comment used to say the day would come and the trail would live on "the route that
// unmasks". It lives here instead, on the route that already renders the two fields: ADR 0030 §43
// names THIS response as the thing it is fixing, and a second read route would have needed a URL
// noun nobody has decided and a screen the specification does not have — docs/ui-ux/09 §139 shows
// the number inline, with no reveal button.)
//
// WHY THE WRITE HAPPENS BEFORE THE BODY AND WHY ITS FAILURE IS A 500: rule 6 does not permit the
// disclosure without the trail, so the only two orderings that are honest are "trail first, then
// answer" and "refuse". Answering with masked values instead would be a government screen
// quietly showing something different from what the officer's permissions say, with nothing on
// it to explain why — the officer reads it as "my key was taken away" and nobody is told the
// ledger is broken.
//
// NO idem.* DECLARATION: a GET changes no BUSINESS state. The audit entry is append-only by
// design (rule 6, invariant 4) — two identical reads are two disclosures and are two rows,
// which is the correct count, not a duplicate to be suppressed.
func (h *Handler) DocPhieuPhanAnh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("maTraCuu")

	if ma == "" {
		// An empty path segment cannot match a code and has no business reaching the database.
		h.khongTimThay(w)
		return
	}

	p, err := h.d.Phieu.TheoMaTraCuu(ctx, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			h.khongTimThay(w)
			return
		}
		// The wrapped error never reaches the client, and it never carries the lookup code
		// either — the code is the one string that opens a citizen's petition (rule 3).
		h.d.Log.Error("đọc phiếu phản ánh: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE RESTRICTED FIELD, CHECKED AFTER THE READ AND ANSWERED AS 404.
	//
	// After, because the field is a property of the record and nothing outside it can say which
	// petition is which. 404 and not 403, because a 403 here would confirm that a report about a
	// member of staff exists under this code — to a colleague of that person — and the existence
	// of such a report is precisely what the restriction protects. Fail closed, and answer the
	// same thing an unknown code answers.
	if p.LinhVuc == LinhVucHanChe {
		principal, ok := authz.From(ctx)
		if !ok || !h.d.Checker.Allows(ctx, principal, QuyenHanChe) {
			h.khongTimThay(w)
			return
		}
	}

	// The commune's own wording, when it has one. A code with no override is normal — see
	// phieuPhanAnhRa.FieldLabel.
	nhan := ""
	if p.LinhVuc != "" {
		nhan, err = h.nhanCuaLinhVuc(ctx, p.LinhVuc)
		if err != nil {
			h.d.Log.Error("đọc nhãn lĩnh vực: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
	}

	// THE DISCLOSURE DECISION, LAST, AND THE TRAIL BEFORE THE BODY.
	//
	// Last on purpose: everything above can still fail with a 500, and an entry recording a
	// disclosure that never reached anybody is a trail that says a citizen's number was read when
	// it was not. An inspection cannot tell that row from a real one.
	xemDayDu := false
	if !p.AnDanh {
		if principal, ok := authz.From(ctx); ok && principal.Ma != "" &&
			h.d.Checker.Allows(ctx, principal, QuyenXemDayDu) {
			// The IP comes from this process's own socket. httpx.ClientIP does not trust
			// X-Forwarded-For, and rule 6, invariant 2 wants the address the request really
			// arrived from, not one the caller chose to name.
			//
			// `principal.Ma`, NEVER `principal.ID`. This entry is the record that a named officer
			// read a citizen's unmasked details (rule 6, invariant 7) — the single entry in this
			// service most likely to be produced in an inspection. A ULID in it names nobody, and
			// the lookup that could translate it may not still hold the row. It WAS `principal.ID`
			// until 2026-09-22 and nothing turned red; `.claude/hooks/audit_actor_guard.py` is why
			// that cannot recur silently.
			//
			// AND `Ma == ""` IS CHECKED IN THE CONDITION ABOVE, WHICH CHANGES WHAT THIS BRANCH
			// DOES RATHER THAN JUST WHAT IT WRITES: with no code to attribute the disclosure to,
			// the officer does not get the unmasked value at all. `xemDayDu` stays false and the
			// response is masked. That is the fail-closed direction — the alternative, disclosing
			// and failing to record it, is the one state rule 6 does not permit.
			err := h.d.Vet.GhiVet(ctx, p.MaTraCuu, audit.Actor{
				ID:   principal.Ma,
				Kind: principal.Kind,
				IP:   httpx.ClientIP(r),
			})
			if err != nil {
				h.d.Log.Error("ghi vết xem đầy đủ người gửi: lỗi hệ thống",
					"xa", string(tenant.MustFrom(ctx)), "err", err)
				httpx.WriteError(w, http.StatusInternalServerError, "internal",
					"Đã xảy ra lỗi. Vui lòng thử lại.", "")
				return
			}
			xemDayDu = true
		}
	}

	vietJSON(w, http.StatusOK, phieuRaNgoai(p, nhan, xemDayDu))
}

// khongTimThay is the ONE answer for "no such code", "another commune's code", "soft deleted"
// and "restricted field, no key".
//
// FOUR CAUSES, ONE RESPONSE, ON PURPOSE. Telling them apart tells somebody trying codes how
// close they are, and in the restricted case it tells a colleague that a report about them
// exists.
func (h *Handler) khongTimThay(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy phiếu phản ánh.", "")
}

// nhanCuaLinhVuc finds the commune's wording for one code.
//
// IT READS THE WHOLE OVERRIDE SET AND SCANS IT, rather than asking the store for one row, and
// the reason is the size of the thing: the code set is CLOSED at twelve (ADR 0026), so the set
// of overrides is at most twelve rows. A per-code query would buy nothing and would add a second
// read path to keep in step with the list one.
func (h *Handler) nhanCuaLinhVuc(ctx context.Context, ma string) (string, error) {
	ds, err := h.d.NhanLinhVuc.DanhSach(ctx)
	if err != nil {
		return "", err
	}
	for _, n := range ds {
		if n.Ma == ma {
			return n.Nhan, nil
		}
	}
	return "", nil
}
