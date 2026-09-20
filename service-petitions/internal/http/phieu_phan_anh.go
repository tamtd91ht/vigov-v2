package http

import (
	"context"
	"errors"
	"net/http"
	"time"

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
// EVERY PIECE OF PERSONAL DATA ON IT IS MASKED, AND THE DEFAULT IS THE DECISION (rule 3,
// invariant 3): anything leaving the API is masked unless the caller holds an EXPLICIT full-view
// permission. There is no such key in `quyen` today — the five `feedback.*` keys are read,
// create, assign, resolve and restricted — so there is nothing to check, and inventing one here
// would be granting full view of a citizen's contact details to whoever happens to be reading.
//
// THAT IS A REAL COST AND IT IS STATED RATHER THAN HIDDEN: an officer cannot ring the reporter
// back from this response. Which permission opens the full number is rule 3's stop condition #1
// and belongs to the customer; masking is the answer that fails closed while they decide.
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

	// ReporterName and ReporterPhone are MASKED — "Nguyễn V. A." and "09****5678" — and are
	// EMPTY, both of them, when the citizen asked to file anonymously. Anonymous means hidden
	// from the staff screen; the identity is still recorded in the database (ADR 0008), and a
	// path that reveals it is itself audited (rule 6, invariant 7).
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

	Public bool `json:"public"`
}

func phieuRaNgoai(p domain.PhieuPhanAnh, nhan string) phieuPhanAnhRa {
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
		Public:     p.HienCongKhai,
	}

	// An anonymous petition carries NEITHER field, not a masked one. A masked name is still a
	// name: "Nguyễn V. A." in a commune of a few thousand people identifies somebody, and the
	// whole point of the flag is that the officer handling it does not know who filed it.
	if !p.AnDanh {
		ra.ReporterName = privacy.MaskName(p.NguoiGuiHoTen)
		ra.ReporterPhone = privacy.MaskPhone(p.NguoiGuiDienThoai)
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
	return ra
}

// LinhVucHanChe is the one field code whose petitions are not visible to every reader.
//
// `can-bo` is "Thái độ / tác phong cán bộ" — a report ABOUT a member of staff
// (docs/ui-ux/09 §5 and §14.5). It needs its own key because the ordinary readers of this
// register are the colleagues of the person being reported on, and a channel a citizen does not
// trust stops carrying the reports a commune actually needs.
const LinhVucHanChe = "can-bo"

// QuyenHanChe opens it. The key already exists in `quyen`
// (service-identity/migrations/0001_init.sql:294, "Xem phản ánh về tác phong cán bộ") — it is
// not invented here.
const QuyenHanChe authz.Perm = "feedback.restricted"

// DocPhieuPhanAnh serves one petition to a member of staff.
// GET /api/v1/citizen-reports/{maTraCuu}
//
// NO AUDIT ENTRY, and that is a decision with a boundary rather than an omission. Rule 6,
// invariant 7 audits reading FULL personal data and reading ACROSS communes. This is neither:
// the contact details are masked on the way out, and the query cannot leave the commune the
// request arrived in. THE DAY A FULL-VIEW PERMISSION EXISTS, the path that uses it is audited —
// that is the same invariant, and it will apply to the route that unmasks, not to this one.
//
// NO idem.* DECLARATION: a GET changes no state.
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

	vietJSON(w, http.StatusOK, phieuRaNgoai(p, nhan))
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
