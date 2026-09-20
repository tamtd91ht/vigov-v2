package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for GET /api/v1/citizen-reports/{maTraCuu}.
//
// THE FOUR CASES RULE 5 INVARIANT 7 REQUIRES ARE ALL HERE, and the third is the one no test of a
// single commune can produce: 401 with no session · 403 with the wrong permission · 401 with the
// right permission but a session issued by ANOTHER commune · 200 with both correct.
//
// The commune case answers 401 rather than 403, and that is authz.xacNhanXa's behaviour, not a
// slip: the commune on the principal is compared with the commune resolved from Host BEFORE the
// permission is consulted, so a token from another commune never reaches the permission check.
// The property being asserted is that NO QUERY RUNS — `goi` on the fake counts the reads.

// --- fakes ---------------------------------------------------------------------------------

const (
	// The codes fixtures use. UNGUESSABLE SHAPE ON PURPOSE even in a test: a fixture is what the
	// next person copies, and a `PA-0001` here would be a sequential code in the repository for
	// somebody to imitate (rule 10, forbidden #5).
	maPhieuThuong = "PA-4K7M-92XR-BTVD"
	maPhieuCanBo  = "PA-9WDN-3HQK-72FM"
	maPhieuNhapHo = "PA-2TYX-K8PV-49RJ"
	maPhieuXaB    = "PA-7HMC-5NQZ-83WK"
)

// phieuGia is the petition register, KEYED BY COMMUNE AND BY CODE, reading the commune from the
// context exactly as *store.Scoped does. Keyed any other way, the isolation case would pass
// while proving nothing.
//
// `goi` counts the reads. The count is what proves the commune check happens BEFORE any store
// access — a route that refused only after reading would still have read another commune's row.
type phieuGia struct {
	theo map[tenant.ID]map[string]domain.PhieuPhanAnh
	loi  error
	goi  int
}

func (p *phieuGia) TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) {
	p.goi++
	if p.loi != nil {
		return domain.PhieuPhanAnh{}, p.loi
	}
	pa, co := p.theo[tenant.MustFrom(ctx)][ma]
	if !co {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return pa, nil
}

// mocMau are the fixture instants. FIXED, not relative to time.Now(): a deadline expressed as
// "two hours ago" makes the assertions depend on when the suite runs, and the one thing these
// deadlines must do is come out of the response byte for byte.
var (
	mocGui    = time.Date(2026, 9, 9, 7, 20, 0, 0, time.UTC)
	mocVaoSo  = time.Date(2026, 9, 9, 7, 20, 3, 0, time.UTC)
	mocTiepNh = time.Date(2026, 9, 9, 15, 20, 0, 0, time.UTC) // 8 giờ làm việc sau mốc gửi
	mocXuLy   = time.Date(2026, 9, 16, 9, 20, 0, 0, time.UTC) // 56 giờ làm việc sau mốc gửi
)

// phieuMau gives commune A three petitions with DIFFERENT shapes, and commune B one petition
// whose code exists ONLY in commune B.
//
//	maPhieuThuong  citizen channel, classified — BOTH deadlines set
//	maPhieuCanBo   citizen channel, classified into the RESTRICTED field `can-bo`
//	maPhieuNhapHo  staff-booked — han_tiep_nhan NULL ("không áp dụng"), han_xu_ly_xong set
//
// A fourth shape is asserted from maPhieuThuong by mutating it in the one test that needs it:
// unclassified, where han_xu_ly_xong is NULL meaning "chưa có" — the OPPOSITE of the NULL on
// maPhieuNhapHo. Two rows are the only way to show that both NULLs exist and mean different
// things.
func phieuMau() *phieuGia {
	honTen := "Nguyễn Văn An"
	// THE AGREED FAKE NUMBER (rule 3, invariant 5). Never a real one, not even in a fixture.
	dienThoai := "0900000000"

	return &phieuGia{theo: map[tenant.ID]map[string]domain.PhieuPhanAnh{
		xaA: {
			maPhieuThuong: {
				ID: "pa-001", MaTraCuu: maPhieuThuong,
				Kenh: domain.KenhZaloMiniApp, CongDanID: "cd-001",
				NoiDung: "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
				LinhVuc: "rac-thai", DiaChi: "Đầu ngõ thôn Hà Lam",
				NguoiGuiHoTen: honTen, NguoiGuiDienThoai: dienThoai,
				TrangThai:   domain.DangPhanLoai,
				GocDemHan:   mocGui,
				VaoSoLuc:    mocVaoSo,
				HanTiepNhan: mocTiepNh,
				HanXuLyXong: mocXuLy,
				PhanLoaiLuc: time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC),
			},
			maPhieuCanBo: {
				ID: "pa-002", MaTraCuu: maPhieuCanBo,
				Kenh: domain.KenhZaloMiniApp, CongDanID: "cd-002",
				NoiDung: "Cán bộ tiếp dân nói năng không đúng mực.",
				LinhVuc: LinhVucHanChe,
				AnDanh:  true,
				// Present even though AnDanh is true — the record keeps the identity, the
				// response must not show it (ADR 0008).
				NguoiGuiHoTen: honTen, NguoiGuiDienThoai: dienThoai,
				TrangThai: domain.DangPhanLoai,
				GocDemHan: mocGui, VaoSoLuc: mocVaoSo,
				HanTiepNhan: mocTiepNh, HanXuLyXong: mocXuLy,
			},
			maPhieuNhapHo: {
				ID: "pa-003", MaTraCuu: maPhieuNhapHo,
				Kenh:    domain.KenhCanBoNhapHo,
				NoiDung: "Bà con thôn Hà Lam phản ánh mất điện thường xuyên.",
				LinhVuc: "dien",
				// GocDemHan is THREE DAYS before VaoSoLuc: the citizen reported it to the
				// hamlet leader, who booked it later. Inside the seven-day window.
				TrangThai: domain.DaTiepNhan,
				GocDemHan: mocGui.AddDate(0, 0, -3),
				VaoSoLuc:  mocVaoSo,
				// HanTiepNhan DELIBERATELY ZERO: "KHÔNG ÁP DỤNG", never 0 hours.
				HanXuLyXong: mocXuLy,
			},
		},
		xaB: {
			maPhieuXaB: {
				ID: "pa-b-001", MaTraCuu: maPhieuXaB,
				Kenh: domain.KenhWebXa, NoiDung: "Phiếu của xã B.",
				TrangThai: domain.DaTiepNhan,
				GocDemHan: mocGui, VaoSoLuc: mocVaoSo,
			},
		},
	}}
}

// nhanLinhVucGia is the commune's label overrides, KEYED BY COMMUNE.
type nhanLinhVucGia struct {
	theo map[tenant.ID][]domain.NhanLinhVuc
	loi  error
	goi  int
}

func (n *nhanLinhVucGia) DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error) {
	n.goi++
	if n.loi != nil {
		return nil, n.loi
	}
	return n.theo[tenant.MustFrom(ctx)], nil
}

// nhanLinhVucMau gives commune A an override for `rac-thai` AND NOT for `dien`. The second
// absence is the interesting one: a code with no override is the ordinary case, and the response
// must carry an empty label rather than fail or invent one.
func nhanLinhVucMau() *nhanLinhVucGia {
	return &nhanLinhVucGia{theo: map[tenant.ID][]domain.NhanLinhVuc{
		xaA: {
			{ID: "nlv-001", Ma: "rac-thai", Nhan: "Rác thải – Vệ sinh môi trường"},
		},
		xaB: {},
	}}
}

func docPhieu(t *testing.T, than []byte) phieuPhanAnhRa {
	t.Helper()
	var ra phieuPhanAnhRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

func duong(ma string) string { return "/api/v1/citizen-reports/" + ma }

// --- rule 5, invariant 7: the four cases ------------------------------------------------------

func TestDocPhieuKhongCoPhienThi401(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), nil)

	doiMa(t, w, http.StatusUnauthorized)
	if m.phieu.goi != 0 {
		t.Errorf("đã đọc kho %d lần dù chưa có phiên — phép kiểm phải chặn TRƯỚC khi chạm dữ liệu", m.phieu.goi)
	}
}

func TestDocPhieuSaiQuyenThi403(t *testing.T) {
	m := dungMayChu(t)
	// An account of commune A that holds NO permission at all — a real state: somebody whose
	// role was withdrawn still has a valid session.
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {}}}
	})

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusForbidden)
	if e := loiTra(t, w); e.Code != "forbidden" {
		t.Errorf("mã lỗi = %q, muốn %q", e.Code, "forbidden")
	}
	if m.phieu.goi != 0 {
		t.Errorf("đã đọc kho %d lần dù thiếu quyền", m.phieu.goi)
	}
}

// TestDocPhieuDungQuyenSaiXaThi401 is the case a single-commune test suite can never produce,
// and the one rule 1 exists for: a principal issued by commune B, holding the right permission
// IN COMMUNE B, arriving at commune A's host.
func TestDocPhieuDungQuyenSaiXaThi401(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			// Deliberately granted in BOTH communes, so the refusal cannot be mistaken for a
			// missing grant. What refuses is the commune comparison, before the grant is read.
			xaA: {idCanBo: {authz.Perm("feedback.read"): true}},
			xaB: {idCanBo: {authz.Perm("feedback.read"): true}},
		}}
	})

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaB))

	doiMa(t, w, http.StatusUnauthorized)
	if m.phieu.goi != 0 {
		t.Errorf("đã đọc kho %d lần cho một phiên của xã khác — lẽ ra không truy vấn nào được chạy", m.phieu.goi)
	}
}

func TestDocPhieuDuQuyenDungXaThi200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	ra := docPhieu(t, w.Body.Bytes())

	if ra.Code != maPhieuThuong {
		t.Errorf("code = %q, muốn %q", ra.Code, maPhieuThuong)
	}
	if ra.Status != string(domain.DangPhanLoai) {
		t.Errorf("status = %q, muốn %q", ra.Status, domain.DangPhanLoai)
	}
	if ra.Field != "rac-thai" {
		t.Errorf("field = %q, muốn %q", ra.Field, "rac-thai")
	}
	if ra.FieldLabel != "Rác thải – Vệ sinh môi trường" {
		t.Errorf("field_label = %q — nhãn của xã phải được ghép vào", ra.FieldLabel)
	}
}

// --- rule 1: a code of another commune is simply not there ------------------------------------

func TestDocPhieuMaCuaXaKhacThi404(t *testing.T) {
	m := dungMayChu(t)

	// A VALID code — it exists, in commune B — presented at commune A with a commune A session.
	// Nothing about it is malformed, so the only thing that can refuse it is the commune scope.
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuXaB), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusNotFound)
	if e := loiTra(t, w); e.Code != "not_found" {
		t.Errorf("mã lỗi = %q, muốn %q", e.Code, "not_found")
	}
}

// --- rule 3: personal data never leaves unmasked ----------------------------------------------

func TestDocPhieuCheDuLieuCaNhan(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()

	// THE ASSERTION IS ON THE RAW BODY, not on the decoded struct, and that is the point: it
	// catches the value appearing ANYWHERE in the response — including a field somebody adds
	// later without reading this test.
	if strings.Contains(than, "0900000000") {
		t.Error("số điện thoại đi ra nguyên vẹn — luật 3 bất biến 3 đòi che")
	}
	if strings.Contains(than, "Nguyễn Văn An") {
		t.Error("họ tên đi ra nguyên vẹn — luật 3 bất biến 3 đòi che")
	}

	ra := docPhieu(t, w.Body.Bytes())
	if ra.ReporterPhone == "" || ra.ReporterName == "" {
		t.Error("che thành rỗng — cán bộ không nhận ra được bản ghi; phải là giá trị ĐÃ CHE, không phải mất")
	}
}

// TestDocPhieuAnDanhKhongTraTenLanSo asserts the stronger rule for an anonymous petition:
// NEITHER field, not even a masked one. A masked name is still a name in a commune of a few
// thousand people.
func TestDocPhieuAnDanhKhongTraTenLanSo(t *testing.T) {
	m := dungMayChu(t)
	// Grant the restricted key: this fixture is in the `can-bo` field, and the point of the test
	// is anonymity, not the restriction.
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {
				authz.Perm("feedback.read"):       true,
				authz.Perm("feedback.restricted"): true,
			}},
		}}
	})

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuCanBo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docPhieu(t, w.Body.Bytes())
	if !ra.Anonymous {
		t.Fatal("anonymous = false trên phiếu ẩn danh")
	}
	if ra.ReporterName != "" || ra.ReporterPhone != "" {
		t.Errorf("phiếu ẩn danh vẫn trả người gửi: ten=%q so=%q", ra.ReporterName, ra.ReporterPhone)
	}
}

// --- the restricted field ----------------------------------------------------------------------

// TestDocPhieuLinhVucHanCheKhongCoQuyenThi404 asserts 404 and not 403, and the difference
// matters: a 403 confirms to a colleague of the officer complained about that such a report
// exists under this code.
func TestDocPhieuLinhVucHanCheKhongCoQuyenThi404(t *testing.T) {
	m := dungMayChu(t) // grants feedback.read, NOT feedback.restricted

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuCanBo), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusNotFound)

	// THE RESPONSE MUST BE INDISTINGUISHABLE FROM AN UNKNOWN CODE. Comparing the two bodies is
	// what makes that checkable — asserting only the status would miss a message that said
	// "bạn không có quyền xem lĩnh vực này".
	khac := m.goi(t, http.MethodGet, hostA, duong("PA-0000-0000-0000"), canBoCuaXa(xaA))
	doiMa(t, khac, http.StatusNotFound)
	if w.Body.String() != khac.Body.String() {
		t.Errorf("phản hồi lĩnh vực hạn chế khác phản hồi mã không tồn tại:\n  %s\n  %s",
			w.Body.String(), khac.Body.String())
	}
}

func TestDocPhieuLinhVucHanCheCoQuyenThi200(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {
				authz.Perm("feedback.read"):       true,
				authz.Perm("feedback.restricted"): true,
			}},
		}}
	})

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuCanBo), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	if ra := docPhieu(t, w.Body.Bytes()); ra.Field != LinhVucHanChe {
		t.Errorf("field = %q, muốn %q", ra.Field, LinhVucHanChe)
	}
}

// --- the two NULLs, on the wire ------------------------------------------------------------------

// TestDocPhieuHaiLoaiNULLRaKhacNhau is the test this whole file exists for.
//
// Two petitions, two NULL deadline columns, TWO OPPOSITE MEANINGS, and the only way either can
// be shown to be right is by showing the other is different:
//
//	staff-booked   acknowledge_due = null  "KHÔNG ÁP DỤNG"  resolve_due = a value
//	unclassified   resolve_due     = null  "CHƯA CÓ"        acknowledge_due = a value
//
// A response that rendered either NULL as a zero instant would pass a status assertion and fail
// here — and a zero instant reaching a report is a petition overdue since the year 1.
func TestDocPhieuHaiLoaiNULLRaKhacNhau(t *testing.T) {
	m := dungMayChu(t)

	// Make an unclassified citizen-channel petition out of the classified fixture, changing
	// exactly the two things classification sets.
	chuaPhanLoai := m.phieu.theo[xaA][maPhieuThuong]
	chuaPhanLoai.MaTraCuu = "PA-5QKD-7WNB-2XTH"
	chuaPhanLoai.TrangThai = domain.DaTiepNhan
	chuaPhanLoai.LinhVuc = ""
	chuaPhanLoai.HanXuLyXong = time.Time{}
	chuaPhanLoai.PhanLoaiLuc = time.Time{}
	m.phieu.theo[xaA][chuaPhanLoai.MaTraCuu] = chuaPhanLoai

	t.Run("phiếu nhập hộ: hạn tiếp nhận là null KHÔNG ÁP DỤNG", func(t *testing.T) {
		w := m.goi(t, http.MethodGet, hostA, duong(maPhieuNhapHo), canBoCuaXa(xaA))
		doiMa(t, w, http.StatusOK)
		ra := docPhieu(t, w.Body.Bytes())

		if ra.AcknowledgeDue != nil {
			t.Errorf("acknowledge_due = %v, muốn null — phiếu nhập hộ KHÔNG ÁP DỤNG hạn tiếp nhận", *ra.AcknowledgeDue)
		}
		if ra.ResolveDue == nil {
			t.Fatal("resolve_due = null — phiếu nhập hộ chốt cả hai hạn lúc vào sổ (ADR 0028 quyết định E)")
		}
		// AND IT MUST NOT BE A ZERO INSTANT DRESSED UP AS A VALUE.
		if ra.ResolveDue.IsZero() {
			t.Error("resolve_due là mốc rỗng — tức hạn năm 1, tức quá hạn ngay lúc tiếp nhận")
		}
	})

	t.Run("phiếu chưa phân loại: hạn xử lý là null CHƯA CÓ", func(t *testing.T) {
		w := m.goi(t, http.MethodGet, hostA, duong(chuaPhanLoai.MaTraCuu), canBoCuaXa(xaA))
		doiMa(t, w, http.StatusOK)
		ra := docPhieu(t, w.Body.Bytes())

		if ra.ResolveDue != nil {
			t.Errorf("resolve_due = %v, muốn null — chưa phân loại thì CHƯA CÓ hạn xử lý", *ra.ResolveDue)
		}
		if ra.AcknowledgeDue == nil {
			t.Fatal("acknowledge_due = null trên phiếu kênh công dân — đồng hồ tiếp nhận PHẢI đang chạy trong khoảng chờ phân loại")
		}
	})

	t.Run("hai null nằm ở hai cột khác nhau", func(t *testing.T) {
		// The assertion that neither of the two above can make alone: if the handler had swapped
		// the two columns, each sub-test above would still see one null and one value.
		nhapHo := docPhieu(t, m.goi(t, http.MethodGet, hostA, duong(maPhieuNhapHo), canBoCuaXa(xaA)).Body.Bytes())
		chua := docPhieu(t, m.goi(t, http.MethodGet, hostA, duong(chuaPhanLoai.MaTraCuu), canBoCuaXa(xaA)).Body.Bytes())

		if (nhapHo.AcknowledgeDue == nil) == (chua.AcknowledgeDue == nil) {
			t.Error("acknowledge_due null giống nhau trên cả hai phiếu — hai cột đã bị hoán đổi hoặc bị gộp")
		}
		if (nhapHo.ResolveDue == nil) == (chua.ResolveDue == nil) {
			t.Error("resolve_due null giống nhau trên cả hai phiếu — hai cột đã bị hoán đổi hoặc bị gộp")
		}
	})
}

// TestDocPhieuKhongCoTruongQuaHan pins the ABSENCE of an overdue field on the contract.
//
// It is an assertion about a thing that is not there, which is unusual and deliberate: adding
// `overdue` back would be an easy, plausible-looking change, and rule 9 is the reason not to —
// the deadline is already on the response, so a boolean beside it is a second representation of
// one fact, frozen at the instant the response was built.
func TestDocPhieuKhongCoTruongQuaHan(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var tho map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	for _, khoa := range []string{"overdue", "acknowledge_overdue", "is_overdue", "qua_han"} {
		if _, co := tho[khoa]; co {
			t.Errorf("phản hồi mang trường %q — quá hạn phải SUY RA từ hạn, không gửi kèm (luật 10 bất biến 3, luật 9)", khoa)
		}
	}
}

// --- the label override is not required ---------------------------------------------------------

func TestDocPhieuLinhVucChuaDoiTenThiNhanRong(t *testing.T) {
	m := dungMayChu(t)

	// maPhieuNhapHo is in `dien`, for which commune A has NO override.
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuNhapHo), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	ra := docPhieu(t, w.Body.Bytes())
	if ra.Field != "dien" {
		t.Errorf("field = %q, muốn %q", ra.Field, "dien")
	}
	if ra.FieldLabel != "" {
		t.Errorf("field_label = %q — xã chưa đặt lại tên cho mã này, nhãn mặc định thuộc tầng 1 ở platform", ra.FieldLabel)
	}
}

// --- failures fail closed and say nothing -------------------------------------------------------

func TestDocPhieuKhoHongThi500VaKhongLoNoiDung(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Phieu = &phieuGia{loi: errors.New("pg: connection refused tại phiếu PA-4K7M-92XR-BTVD của Nguyễn Văn An")}
	})

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusInternalServerError)
	than := w.Body.String()
	// The store's error text is deliberately hostile here: it carries a name and a lookup code,
	// which is exactly what must not travel back to a client (rule 3, forbidden #3).
	if strings.Contains(than, "Nguyễn Văn An") || strings.Contains(than, "connection refused") ||
		strings.Contains(than, maPhieuThuong) {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %s", than)
	}
}
