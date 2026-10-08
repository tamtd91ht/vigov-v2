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

	// Lat and Lng are the SCENE LOCATION (ADR 0050, docs/ui-ux/09 §8.4) — the map pin beside the
	// address. ABSENT when the citizen sent none; both present otherwise.
	//
	// UNDER `feedback.read`, like `address` (owner's decision): it is where the problem is, a place
	// the citizen chose to send, not a home address from a profile. AND SHOWN ON AN ANONYMOUS
	// PETITION, exactly as `address` is — anonymity hides the reporter's identity, and the place is
	// what the officer has to go to.
	//
	// OMITEMPTY POINTERS: absent means "not sent", while a real 0 still travels as 0; omitempty also
	// makes tools/apidoc declare them optional so yesterday's fixtures stay valid.
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`

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

	// Reason, ReceivingBody and BranchEndedAt are the two terminal branches (migration 0011):
	// `ly_do_ket_thuc_nhanh`, `co_quan_nhan`, `ket_thuc_nhanh_luc`. ABSENT ON EVERY OTHER STATUS.
	//
	// OMITEMPTY, AND THE POINTER IS OMITEMPTY TOO, ON PURPOSE: tools/apidoc declares a field without
	// omitempty REQUIRED, and a required field that seven of nine statuses never carry breaks every
	// web-admin and citizen fixture that was right yesterday. BranchEndedAt is deliberately NOT
	// ClosedAt: a refused petition is not a resolved one (migration 0011's header).
	Reason        string     `json:"reason,omitempty"`
	ReceivingBody string     `json:"receiving_body,omitempty"`
	BranchEndedAt *time.Time `json:"branch_ended_at,omitempty"`

	// HasCitizen says whether a citizen ACCOUNT stands behind the petition (`cong_dan_id` non-empty)
	// — somebody the commune can notify and who can confirm the result. It is the fact
	// domain.DongDuoc decides the closing point on (owner's decision of 2026-09-24): without it a
	// screen cannot tell "close from `da-xu-ly`" from "wait for `cho-dan-xac-nhan`", and learns which
	// by drawing the button and taking the 409.
	//
	// A BOOLEAN AND NEVER THE ID. `cong_dan_id` is the key of a citizen's identity in another
	// service; nothing on a staff screen needs it, and a flag answers the one question asked.
	//
	// A POINTER WITH omitempty, SET ON EVERY RESPONSE BUILT BY phieuRaNgoai. The pointer makes `false`
	// travel as `false` rather than vanish; omitempty makes tools/apidoc declare the key OPTIONAL
	// (tools/apidoc/schema.go:723 — without it every existing fixture would fail a new required key).
	// Absent therefore means "this server predates the field", never "no citizen".
	HasCitizen *bool `json:"has_citizen,omitempty"`

	// ContactUnverified is the EXPLICIT server fact behind Web Admin's label "Số tự khai — chưa xác
	// thực" (ADR 0080 decision 5): the petition came from a Mini App session WITHOUT a verified phone,
	// so `reporter_name` / `reporter_phone` were typed by hand and prove nothing; it is owned by a Zalo
	// account, not by a citizen identity.
	//
	// NOT INFERABLE FROM `channel` + `has_citizen`, and that is why it exists: a staff-booked petition
	// also has no citizen, and so may the next channel — a rule written in the UI would be a second
	// copy of "does this petition carry an identity" that drifts the day a third shape appears (rule 9).
	// DERIVED from `zalo_account_id IS NOT NULL` on every response (domain.ContactUnverified), never
	// stored beside it. A BOOLEAN AND NEVER THE ACCOUNT ID: an internal id on a staff screen would let
	// staff link one person's anonymous reports together (ADR 0008; migration 0032).
	//
	// Pointer + omitempty, SET ON EVERY RESPONSE — the HasCitizen precedent: `false` travels as
	// `false`, and the key stays optional in the contract. Absent means "this server predates the field".
	ContactUnverified *bool `json:"contact_unverified,omitempty"`

	// Accountless is the EXPLICIT server fact that the petition was sent through the TEMPORARY
	// accountless path (ADR 0083 row 11): Mini App channel, no citizen, no Zalo account — nobody behind
	// it, its contact details self-declared, nobody notified. DERIVED by domain.PhieuPhanAnh.Accountless,
	// the one definition; Web Admin labels from this field and never re-derives it from `channel` +
	// `has_citizen` (rule 9). Pointer + omitempty, set on every response — the ContactUnverified precedent.
	Accountless *bool `json:"accountless,omitempty"`

	// PublicationStatus is `publication_status` (migration 0017, ADR 0050 point 8): the staff
	// moderation of the public page — `cho-duyet` · `cong-khai` · `an`. Separate from `status`.
	//
	// SET ON EVERY RESPONSE (the store never reads it empty), with omitempty ONLY so tools/apidoc
	// declares it optional and yesterday's web-admin fixtures stay valid — the ReopenCount precedent.
	// Absent therefore means "this server predates the field", never a fourth value.
	PublicationStatus string `json:"publication_status,omitempty"`

	// Public is DERIVED from PublicationStatus (`cong-khai`) and kept for the clients that already read
	// it. It no longer reads the superseded `hien_cong_khai`, which no code writes to true. One fact,
	// one source: the two fields on the wire can never disagree because one is computed from the other.
	Public bool `json:"public"`

	// Rating, RatingComment and RatedAt are the CITIZEN'S verdict (ADR 0050 point 2) — `diem_hai_long`,
	// `rating_comment`, `danh_gia_luc`. ABSENT until the citizen rates; a later rating replaces an
	// earlier one (the earlier one is in the audit trail).
	//
	// RatingComment IS CITIZEN FREE TEXT AND FOLLOWS `content` EXACTLY: not masked (an officer who
	// cannot read "rác vẫn còn ở cuối ngõ" cannot act on it), shown to whoever may read the petition —
	// `feedback.read`, plus `feedback.restricted` on `can-bo`, which the read path already enforces by
	// answering 404 before this struct is built — and shown on an ANONYMOUS petition too, as `content`
	// is. What masking protects is the identifiers of a named person; the citizen's own words about the
	// work are what the commune has to act on.
	//
	// OMITEMPTY on all three (pointers where zero is not "absent"), so tools/apidoc marks them optional
	// and yesterday's fixtures stay valid.
	Rating        *int       `json:"rating,omitempty"`
	RatingComment string     `json:"rating_comment,omitempty"`
	RatedAt       *time.Time `json:"rated_at,omitempty"`

	// ReopenCount is `so_lan_mo_lai`: how many times a citizen's low rating reopened the petition. ZERO
	// IS A MEANINGFUL ANSWER ("never reopened"), so it is a pointer SET ON EVERY RESPONSE — `0` travels
	// as `0` — with omitempty only so tools/apidoc declares it optional (the HasCitizen precedent).
	// Absent therefore means "this server predates the field", never "not reopened". There is no cap
	// (ADR 0050): this is a monitoring figure, "một phiếu có thể quay vòng mãi".
	ReopenCount *int `json:"reopen_count,omitempty"`
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
		Lat:        p.Lat,
		Lng:        p.Lng,
		Anonymous:  p.AnDanh,
		ClockFrom:  p.GocDemHan,
		BookedAt:   p.VaoSoLuc,
		Unit:       p.BoPhanID,
		Assignee:   p.CanBoXuLyID,
		Result:     p.KetQuaXuLy,
		Public:     p.PublicationStatus == domain.PublicationPublic,

		PublicationStatus: string(p.PublicationStatus),
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
	// Carried as the row holds them. The database binds the three to the two branch statuses, so on
	// any other status they are empty and omitted.
	ra.Reason = p.LyDoKetThucNhanh
	ra.ReceivingBody = p.CoQuanNhan
	if !p.KetThucNhanhLuc.IsZero() {
		t := p.KetThucNhanhLuc
		ra.BranchEndedAt = &t
	}
	// The SAME predicate domain.DongDuoc reads (`CongDanID == ""`), so the flag and the closing rule
	// cannot disagree about which petitions have somebody to confirm.
	coCongDan := p.CongDanID != ""
	ra.HasCitizen = &coCongDan
	unverified := p.ContactUnverified()
	ra.ContactUnverified = &unverified
	accountless := p.Accountless()
	ra.Accountless = &accountless

	if p.Rating != 0 {
		stars, at := p.Rating, p.RatedAt
		ra.Rating, ra.RatedAt = &stars, &at
		ra.RatingComment = p.RatingComment
	}
	reopened := p.SoLanMoLai
	ra.ReopenCount = &reopened
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
			// 503 when platform is unreachable — never a raw code in place of a label (ADR 0060 §3).
			writeFieldCatalogueError(w, r, h.d.Log.Error, "đọc nhãn lĩnh vực", err)
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
