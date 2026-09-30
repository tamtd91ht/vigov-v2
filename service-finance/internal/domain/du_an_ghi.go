package domain

// The WRITE half of the investment project register (docs/ui-ux/06-giai-ngan.md §9, §13).
//
// WHERE THE RULES REALLY LIVE, said first so nothing here is mistaken for the enforcement:
//
//	UNIQUE (tenant_id, ma), soft-deleted rows INCLUDED   migrations/0004:216-221
//	CHECK (ke_hoach_von_nam >= 0)                        migrations/0004:224-229
//	CHECK (tong_muc_duoc_duyet IS NULL OR >= 0)          migrations/0004:230
//	CHECK (nam BETWEEN 2000 AND 2100)                    migrations/0004:231-233
//	hard DELETE refused outright                         migrations/0004, `ho_so_luu_tru_cam_xoa_cung`
//	CHECK (so_tien_phan_bo >= 0)                         migrations/0007, phan_bo_nguon_von
//
// Those are the floor, and they hold against every writer — this service, a psql prompt, an import
// job written next year. The rules restated in this file exist for ONE reason: the database answers
// with a PostgreSQL exception whose text is English and names a constraint, which tells an
// accountant in a commune nothing they can act on. This layer refuses first, in Vietnamese, naming
// the operation.
//
// SO A DRIFT BETWEEN THIS FILE AND THE DATABASE IS A WORSE ERROR MESSAGE, NOT A HOLE. The direction
// that WOULD be a hole — this layer allowing what the database forbids — cannot happen: the
// constraint runs last and refuses, and the transaction rolls back with the audit entry inside it
// (rule 6, invariant 3).
//
// THIS PACKAGE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4, and doc.go).

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// --- what a client may actually supply ----------------------------------------------------------

var (
	// ErrThieuMaDuAn — §9 offers `☑ Tự sinh mã` beside the code box, and THIS SERVICE DOES NOT
	// GENERATE ONE. That is a refusal to decide, not an omission, and it is the one thing to read
	// before "fixing" it.
	//
	// THE SPECIFICATION GIVES TWO INCOMPATIBLE FORMATS FOR THE SAME COLUMN:
	//
	//	§9      "Tự sinh sẽ cấp số tiếp theo trong dãy DA01, DA02…"
	//	§7.2 · §8 · §11   `DA-2026-be-tong-hoa-duong-ngo-xo-2`
	//
	// and the scope of the sequence is undecidable too. `UNIQUE (tenant_id, ma)` (0004:216-221) has
	// no `nam` in it, so a per-year sequence would make 2027's `DA01` collide with 2026's — while a
	// per-commune sequence exhausts `DA01..DA99` inside two budget years, since §14's own commune
	// carries 63 projects in ONE year.
	//
	// WHY GUESSING IS THE EXPENSIVE MOVE RATHER THAN THE CHEAP ONE: a project code is an ISSUED
	// CODE. Rule 7, invariant 3 forbids reissuing one and forbidden #4 forbids renumbering one that
	// has already been issued — so the first commune to enter a project under a guessed format is a
	// commune whose codes can never be corrected. Refusing writes nothing and can be loosened with
	// one function the day the customer answers; generating cannot be taken back at all.
	ErrThieuMaDuAn = errors.New("du_an: thiếu `code` — hệ thống chưa tự sinh mã dự án, hãy nhập mã")

	ErrMaDuAnQuaDai      = errors.New("du_an: `code` quá dài")
	ErrMaDuAnSaiDinhDang = errors.New("du_an: `code` chỉ gồm chữ cái, chữ số và dấu gạch nối")

	// ErrMaDuAnBatBien — a request tried to change an issued project code.
	//
	// REFUSED RATHER THAN IGNORED. Rule 7, forbidden #4 is "renumbering file codes that have already
	// been issued", and a project's code is printed on the row of §7.2, quoted in the disbursement
	// decisions filed against it, and carried by every voucher through the audit trail's `subject`.
	// Changing it silently detaches a paper trail from the record it describes.
	ErrMaDuAnBatBien = errors.New("du_an: không đổi `code` của một dự án đã cấp mã — mã đã cấp thì không đánh lại")

	// ErrNamBatBien — a request tried to move a project to another budget year.
	//
	// §13 rule 8: "Năm ngân sách chuyển đổi không xoá dữ liệu năm cũ — mỗi năm là một tập dự án
	// riêng." Moving a project between years moves its whole plan AND every voucher filed against it
	// out of one year's totals and into another's — the KPI cards of §3, the category table of §5 and
	// the chart of §4, all of which have already been read off a screen and may have been reported
	// upward. One UPDATE, two years wrong, and no row looks wrong.
	//
	// THE SAME SHAPE AS ErrDuAnBatBien ON A VOUCHER, and for the same reason: if a project was
	// entered under the wrong year, the operation is to remove it with a reason and enter it again —
	// two events, both audited, both naming the year they belong to.
	ErrNamBatBien = errors.New("du_an: không chuyển dự án sang năm ngân sách khác — hãy xoá dự án kèm lý do rồi nhập lại ở năm đúng")

	ErrThieuNamDuAn     = errors.New("du_an: thiếu `year`")
	ErrNamDuAnNgoaiLich = errors.New("du_an: `year` ngoài khoảng năm hợp lệ (2000..2100)")

	ErrThieuHangMuc  = errors.New("du_an: thiếu `category_id` — báo cáo tiến độ cộng dồn theo hạng mục nên mỗi dự án thuộc đúng một hạng mục")
	ErrHangMucIDSai  = errors.New("du_an: `category_id` không phải một mã hạng mục hợp lệ")
	ErrThieuTenDuAn  = errors.New("du_an: thiếu `name`")
	ErrTenDuAnQuaDai = errors.New("du_an: `name` quá dài")
	ErrMoTaQuaDai    = errors.New("du_an: `description` quá dài")

	// ErrKeHoachVonAm — a NEGATIVE year plan. Zero is admitted: §9's modal is filled in before the
	// allocation is decided often enough that 0004:224-229 wrote the state down as real. A negative
	// plan is not a state, it is a sign error or a parsing accident, and it would drag the commune's
	// headline "KẾ HOẠCH VỐN NĂM" below the truth with no row looking wrong.
	ErrKeHoachVonAm     = errors.New("du_an: `planned_amount` không được âm")
	ErrKeHoachVonQuaLon = errors.New("du_an: `planned_amount` vượt mức kế hoạch vốn một dự án cấp xã có thể có")
	ErrTongMucAm        = errors.New("du_an: `approved_amount` không được âm")
	ErrTongMucQuaLon    = errors.New("du_an: `approved_amount` vượt mức một dự án cấp xã có thể có")

	ErrNgayDuAnNgoaiLich = errors.New("du_an: ngày ngoài khoảng năm hợp lệ (2000..2100)")
	ErrThamChieuQuaDai   = errors.New("du_an: mã tham chiếu (đơn vị / cán bộ) quá dài")

	ErrThieuLyDoXoaDuAn  = errors.New("du_an: thiếu lý do xoá dự án")
	ErrLyDoXoaDuAnQuaDai = errors.New("du_an: lý do xoá quá dài")

	// ErrPhanBoTrungNguon — one create request named the same funding source twice.
	//
	// REFUSED HERE AND NOT IN THE DATABASE, AND THE DIFFERENCE IS WHAT IS BEING DECIDED. Migration
	// 0007 deliberately leaves `UNIQUE (tenant_id, du_an_id, nguon_von_id)` undeclared and says so
	// outright: whether one project may hold TWO lines naming one source is the customer's call, and
	// this repository is not entitled to answer it (0007:56-66).
	//
	// THIS ERROR DOES NOT ANSWER IT. What it refuses is one REQUEST naming a source twice, which is a
	// malformed body rather than a business state — the §9 modal lists one row per source, so two
	// rows for one source is a client bug, and accepting it would silently double that source's share
	// of a project's plan. The open question stays open: nothing here forbids two lines arriving by
	// any other route, and no constraint was added to the schema.
	ErrPhanBoTrungNguon = errors.New("du_an: một nguồn vốn chỉ được khai một dòng phân bổ trong cùng một lần tạo dự án")

	ErrThieuNguonVonPhanBo = errors.New("du_an: dòng phân bổ thiếu `funding_source_id`")
	ErrPhanBoAm            = errors.New("du_an: `amount` của dòng phân bổ không được âm")
	ErrPhanBoQuaLon        = errors.New("du_an: `amount` của dòng phân bổ vượt mức một dự án cấp xã có thể có")
	ErrQuaNhieuDongPhanBo  = errors.New("du_an: quá nhiều dòng phân bổ nguồn vốn trong một lần tạo dự án")
)

// The bounds. They are not business rules and are not pretending to be: they are the point past
// which a value stops being a project field and starts being a mistake or an attack. An unbounded
// client-supplied string in a government database is a liability, not a feature.
const (
	// MaDuAnToiDa bounds the project code. §11's own sample is 34 characters
	// (`DA-2026-be-tong-hoa-duong-ngo-xo-2`), so this is three times over — it is not a format
	// assertion, it is the point past which a code stops being a code.
	MaDuAnToiDa = 100

	// TenDuAnToiDa — §7 and §14 show real project names running to a full line and past it
	// ("Bê tông hóa đường ngõ xóm tổ 6 và Tuyến GTNT thôn Thanh Ly 1 (tổ 8;9;10)"), so the bound is
	// generous on purpose. What it refuses is a document pasted into a name box.
	TenDuAnToiDa  = 500
	MoTaDuAnToiDa = 5000

	// ThamChieuToiDa bounds `org_unit_id` and `assignee_id` — ids of rows OWNED BY ANOTHER SERVICE
	// (rule 2). A ULID is 26 characters, so this is twice over and is deliberately not a format
	// check: what it refuses is an unbounded client string being carried into a query parameter.
	//
	// ⚠ NEITHER ID IS VERIFIED TO EXIST, AND THAT IS STATED RATHER THAN HIDDEN. An org unit and a
	// member of staff both live in service-identity, so checking them means a gRPC call — and it
	// would have to run INSIDE the transaction that writes the project, which is a synchronous call
	// to a third party inside a write (rule 2, forbidden #5). The consequence, said plainly: a
	// project can name a unit or an officer that does not exist, and the screen shows it as
	// "Chưa phân công" — §9's own default — rather than leaking a name from another commune. Nothing
	// here is on the isolation path: neither id decides what anybody may read.
	ThamChieuToiDa = 64

	LyDoXoaDuAnToiDa = 500

	NamDuAnSom  = 2000
	NamDuAnMuon = 2100

	// PhanBoToiDaMotLanTao bounds the allocation lines one create request may carry. It mirrors
	// store.TranPhanBoMotDuAn rather than importing it — domain imports nothing (rule 4) — and the
	// two are checked against each other by TestTranPhanBoKhopVoiDomain in the store package, so a
	// change to one cannot silently outgrow the other.
	PhanBoToiDaMotLanTao = 200
)

// DongPhanBoMoi is one allocation line as §9's modal supplies it: a source and an amount.
//
// IT IS A PLAN, NOT A PAYMENT. Allocation says where the money is SUPPOSED to come from; a voucher
// says where it actually came from, and §6 shows both on one card precisely because they are
// allowed to disagree (0007:191-199).
type DongPhanBoMoi struct {
	NguonVonID string
	SoTien     Dong
}

// ChuanHoaMaDuAn trims and validates a project code typed by a member of staff.
//
// IT DOES NOT LOWER-CASE AND MUST NOT START TO. `ChuanHoaMa` in danh_muc_ba_tang.go admits lower
// case only, because a catalogue code is a slug this system mints; a PROJECT code is a value the
// commune types, and §11's own sample is `DA-2026-…` in capitals. Folding the case here would store
// a code that differs from the one on the paper decision it came off.
//
// THE UNIQUENESS IS NOT CHECKED HERE and cannot be: `UNIQUE (tenant_id, ma)` counts SOFT-DELETED
// ROWS (0004:216-221), so the question "has this code ever been issued in this commune" is a
// question only the store can answer, and it is answered inside the transaction.
func ChuanHoaMaDuAn(ma string) (string, error) {
	ma = strings.TrimSpace(ma)
	switch {
	case ma == "":
		return "", ErrThieuMaDuAn
	case len(ma) > MaDuAnToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMaDuAnQuaDai, MaDuAnToiDa)
	}
	// Hand-rolled rather than a regexp, matching ChuanHoaMa's own reasoning: the rule is a few lines
	// and a regexp would be one more thing to read carefully.
	if ma[0] == '-' || ma[len(ma)-1] == '-' {
		return "", ErrMaDuAnSaiDinhDang
	}
	truocLaGach := false
	for i := 0; i < len(ma); i++ {
		c := ma[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			truocLaGach = false
		case c == '-':
			if truocLaGach {
				// `DA--01` reads as one code and sorts as another. Refuse it while it is still a typo
				// rather than after it is the code on an archival record.
				return "", ErrMaDuAnSaiDinhDang
			}
			truocLaGach = true
		default:
			return "", ErrMaDuAnSaiDinhDang
		}
	}
	return ma, nil
}

// KiemTraNamDuAn bounds the budget year.
//
// THE WINDOW IS THE MIGRATION'S OWN (0004:231-233). A year of 1026 or 20226 is a typo that makes the
// project invisible on every year-filtered screen while it sits in the table looking healthy — so
// its plan is missing from the commune's total with no row appearing wrong.
func KiemTraNamDuAn(nam int) error {
	if nam == 0 {
		return ErrThieuNamDuAn
	}
	if nam < NamDuAnSom || nam > NamDuAnMuon {
		return ErrNamDuAnNgoaiLich
	}
	return nil
}

// ChuanHoaTenDuAn trims and validates the project name.
//
// It carries Vietnamese WITH diacritics: it is a sentence a person reads, so nothing here restricts
// the character set beyond refusing control characters, which corrupt a screen, a CSV export and a
// log line alike — and a government record is read on all three.
func ChuanHoaTenDuAn(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuTenDuAn
	case len([]rune(s)) > TenDuAnToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrTenDuAnQuaDai, TenDuAnToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrThieuTenDuAn
	}
	return s, nil
}

// ChuanHoaMoTaDuAn trims and validates the optional description. An empty result is a legitimate
// answer — §9 lists it under "Thông tin thêm (không bắt buộc)".
//
// NEWLINES ARE REFUSED ALONG WITH EVERY OTHER CONTROL CHARACTER, and that is a deliberate narrowing
// rather than an oversight: this field lands in an Excel export beside the amounts (§10's template),
// where an embedded newline silently splits one project across two rows.
func ChuanHoaMoTaDuAn(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > MoTaDuAnToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMoTaQuaDai, MoTaDuAnToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrMoTaQuaDai
	}
	return s, nil
}

// ChuanHoaHangMucID trims and bounds the capital plan category a project is classified under.
//
// REQUIRED, and §9 says why in the modal's own footnote: *"Báo cáo tiến độ cộng dồn theo hạng mục,
// nên mỗi dự án thuộc đúng một hạng mục."* A project with no category is money missing from the
// table of §5 while still counted in the KPI card of §3 — two totals on one screen that disagree,
// with no row looking wrong.
//
// IT DOES NOT VALIDATE THE FORMAT, and must not start to: whether the id names a LIVE category OF
// THIS COMMUNE is the question that actually matters, and it is answered inside the transaction
// (rule 1 — there is no foreign key, 0004:182-190 says why).
func ChuanHoaHangMucID(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrThieuHangMuc
	case len([]rune(s)) > ThamChieuToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrHangMucIDSai, ThamChieuToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrHangMucIDSai
	}
	return s, nil
}

// ChuanHoaThamChieu trims and bounds an optional id owned by ANOTHER service — the executing org
// unit and the officer in charge (§9's "Thông tin thêm").
//
// THE EMPTY RESULT IS A MEANINGFUL ANSWER AND NOT AN ERROR: §9's own defaults are
// "— Chưa xác định —" and "— Chưa phân công —", and §7.2 prints "Chưa phân công" on the row. The
// store turns a blank into NULL.
func ChuanHoaThamChieu(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case len([]rune(s)) > ThamChieuToiDa:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrThamChieuQuaDai, ThamChieuToiDa)
	case coKyTuDieuKhien(s):
		return "", ErrThamChieuQuaDai
	}
	return s, nil
}

// KiemTraKeHoachVon refuses a plan that is negative or absurd.
//
// ZERO IS ADMITTED AND NEGATIVE IS NOT, which is `CHECK (ke_hoach_von_nam >= 0)` (0004:224-229) said
// in Vietnamese and first.
//
// THE CEILING IS SoTienToiDa, THE SAME TYPO GUARD A VOUCHER USES, and sharing it is deliberate: both
// numbers are đồng typed by the same accountant on the same screen, and a ceiling that differed
// between them would let a figure through in one box that is refused in the other. What it catches
// is an amount pasted with the thousands separators stripped, or a figure meant in nghìn đồng typed
// as đồng — a project whose plan is 10^3 times the commune's entire budget, dragging every ratio on
// §3 to a number leadership reads and acts on.
func KiemTraKeHoachVon(so Dong) error {
	switch {
	case so < 0:
		return ErrKeHoachVonAm
	case so > SoTienToiDa:
		return fmt.Errorf("%w (tối đa %d đồng)", ErrKeHoachVonQuaLon, int64(SoTienToiDa))
	}
	return nil
}

// KiemTraTongMuc refuses an approved total that is negative or absurd.
//
// ZERO MEANS "THE COMMUNE LEFT IT BLANK" and is not an error — §9: *"Để trống thì lấy bằng số tiền
// bố trí năm nay."* DuAn.TongMucHieuLuc applies that rule in one place, so nothing here has to
// substitute a value and no second copy of the default can drift.
//
// IT IS NOT CHECKED AGAINST THE YEAR PLAN, and that is a refusal to invent a rule. A total approved
// for the WHOLE project that is smaller than this year's allocation looks wrong and can be real —
// a plan revised upward mid-year before the approval decision is reissued. The specification says
// nothing, so nothing here decides; §8 prints both figures side by side and lets a person see it.
func KiemTraTongMuc(so Dong) error {
	switch {
	case so < 0:
		return ErrTongMucAm
	case so > SoTienToiDa:
		return fmt.Errorf("%w (tối đa %d đồng)", ErrTongMucQuaLon, int64(SoTienToiDa))
	}
	return nil
}

// KiemTraNgayDuAn bounds an optional project date — start, completion, or disbursement deadline.
//
// THE ZERO TIME IS ADMITTED because all three are optional (§9's collapsible "Thông tin thêm"), and
// the deadline's blank is filled by HanGiaiNganMacDinh rather than refused.
//
// THE CHECK IS ON THE YEAR AND NOT ON "not in the past": a commune records a project whose works
// began last year and whose money runs into this one — that is precisely what the `chuyen-tiep`
// category of §5 is. What a year of 1026 or 20226 does is make the date sort to an end of the
// timeline, which silently reorders every chart it appears on.
func KiemTraNgayDuAn(ngay time.Time) error {
	if ngay.IsZero() {
		return nil
	}
	if n := ngay.Year(); n < NamDuAnSom || n > NamDuAnMuon {
		return ErrNgayDuAnNgoaiLich
	}
	return nil
}

// HanGiaiNganMacDinh is 31/12 of the budget year — §11's "mặc định 31/12" for `thoi_han_giai_ngan`.
//
// A FUNCTION RATHER THAN A DEFAULT IN THE SCHEMA, because the schema does not know the year: the
// column is `DATE NOT NULL` with no DEFAULT (0004), and a database default would have to be
// `now()`-derived, which on 02/01/2027 would stamp a 2026 project with a 2027 deadline.
//
// THE LOCATION IS UTC BECAUSE THE COLUMN IS A `DATE`. A date has no timezone; taking the caller's
// location here would let a clock in UTC+7 produce 30/12 for the same year.
func HanGiaiNganMacDinh(nam int) time.Time {
	return time.Date(nam, time.December, 31, 0, 0, 0, 0, time.UTC)
}

// ChuanHoaLyDoXoaDuAn validates the reason recorded beside a soft delete.
//
// MANDATORY, AND THAT IS RULE 7, INVARIANT 1: `deleted_at`, `deleted_by` AND `delete_reason`. A
// project that vanished from the commune's plan with no reason attached is a whole year's
// allocation nobody can account for — and the row is still there, so the question WILL be asked.
func ChuanHoaLyDoXoaDuAn(s string) (string, error) {
	return chuanHoaLyDo(s, LyDoXoaDuAnToiDa, ErrThieuLyDoXoaDuAn, ErrLyDoXoaDuAnQuaDai)
}

// ChuanHoaPhanBoMoi validates the allocation lines of ONE create request and returns them trimmed.
//
// §9 PUTS THIS LIST IN THE CREATE MODAL AND MAKES IT OPTIONAL: *"Chưa gắn nguồn nào. Xã theo dõi kế
// hoạch vốn theo hạng mục thì để trống cũng được."* A project with no line here is a NORMAL project,
// not an incomplete one, and §11 names that state (`Chưa gắn nguồn`). Nothing below requires a line.
//
// ⚠ IT DOES NOT COMPARE THE TOTAL AGAINST THE YEAR PLAN, AND MUST NEVER START TO. §9 says the system
// *"đối chiếu tổng các nguồn với số ấy và CẢNH BÁO khi thiếu hoặc vượt"* — a warning, not a refusal —
// and §11 turns the same two numbers into the chip `Đủ` / `Chưa đủ` / `Chưa gắn nguồn`. Both are
// things a SCREEN says about a state the system holds. Turning either into a constraint here would
// refuse the entry at the moment the commune is still working the figures out, which is the only
// moment the modal is open. GanNguon computes the chip; the API returns the two raw numbers; nobody
// is refused.
//
// A ZERO AMOUNT IS ADMITTED, matching `CHECK (so_tien_phan_bo >= 0)` (0007) and its stated reason: a
// source attached before its figure is agreed is a real intermediate state a commune types.
func ChuanHoaPhanBoMoi(ds []DongPhanBoMoi) ([]DongPhanBoMoi, error) {
	if len(ds) == 0 {
		return nil, nil
	}
	if len(ds) > PhanBoToiDaMotLanTao {
		return nil, fmt.Errorf("%w (tối đa %d dòng)", ErrQuaNhieuDongPhanBo, PhanBoToiDaMotLanTao)
	}

	ra := make([]DongPhanBoMoi, 0, len(ds))
	daGap := make(map[string]struct{}, len(ds))
	for _, mot := range ds {
		nguon := strings.TrimSpace(mot.NguonVonID)
		switch {
		case nguon == "":
			// '' IS NOT "NO SOURCE" ON THIS TABLE, IT IS A BROKEN REFERENCE. `phan_bo_nguon_von
			// .nguon_von_id` is NOT NULL with `CHECK (btrim(nguon_von_id) <> '')` (0007) — unlike
			// `chung_tu_giai_ngan.nguon_von_id`, where a blank is the meaningful "not yet recorded
			// against a source". An allocation line names a source by definition; a line naming none
			// is a row the screen would count in "N nguồn" and could match to nothing.
			return nil, ErrThieuNguonVonPhanBo
		case len([]rune(nguon)) > ThamChieuToiDa, coKyTuDieuKhien(nguon):
			return nil, ErrThieuNguonVonPhanBo
		case mot.SoTien < 0:
			return nil, ErrPhanBoAm
		case mot.SoTien > SoTienToiDa:
			return nil, fmt.Errorf("%w (tối đa %d đồng)", ErrPhanBoQuaLon, int64(SoTienToiDa))
		}
		if _, trung := daGap[nguon]; trung {
			return nil, ErrPhanBoTrungNguon
		}
		daGap[nguon] = struct{}{}
		ra = append(ra, DongPhanBoMoi{NguonVonID: nguon, SoTien: mot.SoTien})
	}
	return ra, nil
}
