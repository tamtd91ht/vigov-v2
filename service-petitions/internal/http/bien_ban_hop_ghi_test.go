package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the three STAFF WRITE routes of the meeting-minutes register.
//
//	PROVED HERE   each route's four cases (rule 5, invariant 7): 401 no session · 403 wrong
//	              permission · 401 right permission WRONG COMMUNE · 201 both correct — and in the
//	              first three cases the use case is NOT REACHED, which is what proves the guard runs
//	              before any data is touched · the acting principal reaches the use case as its
//	              BUSINESS CODE · the meeting day travels as a CALENDAR DAY in both directions · the
//	              split sends the confirmed body down untouched and can NOT be told which source to
//	              point at · the refusals map onto the statuses the task register already uses.
//
//	NOT PROVED    the transaction boundary, the numbering rule and the delegation to the task use
//	              case. Those are properties of internal/app over the real store, and they live in
//	              internal/app/bien_ban_hop_test.go.

// --- the fake ---------------------------------------------------------------------------------------

// ghiBienBanGia is the three acts. IT RECORDS THE COMMUNE AND THE ACTOR of every call, because those
// are the two things a route can get wrong in a way no status code shows.
type ghiBienBanGia struct {
	goi   int
	viec  string
	xa    tenant.ID
	nguoi audit.Actor

	ycTao     app.YeuCauTaoBienBan
	ycThem    app.YeuCauThemKetLuan
	ycTach    app.YeuCauTaoNhiemVu
	bienBanID string
	thuTu     int

	ycSua   app.YeuCauSuaBienBan
	ycKy    app.YeuCauKyBienBan
	ycSuaKL app.YeuCauSuaKetLuan
	lyDo    string

	loi error
}

func (g *ghiBienBanGia) ghi(ctx context.Context, viec string, nguoi audit.Actor) {
	g.goi++
	g.viec, g.nguoi = viec, nguoi
	g.xa = tenant.MustFrom(ctx)
}

func (g *ghiBienBanGia) TaoBienBan(ctx context.Context, yc app.YeuCauTaoBienBan, nguoi audit.Actor) (
	domain.BienBanHop, error) {

	g.ghi(ctx, "tao", nguoi)
	g.ycTao = yc
	if g.loi != nil {
		return domain.BienBanHop{}, g.loi
	}
	return domain.BienBanHop{
		ID: "bb-moi", TenCuocHop: yc.TenCuocHop, NgayHop: yc.NgayHop, SoHieu: yc.SoHieu,
		DiaDiem: yc.DiaDiem, ChuTriMa: yc.ChuTriMa, NguoiTaoMa: nguoi.ID, TaoLuc: mocTaoBBA,
		KetLuan: []domain.KetLuanHop{
			{ID: "kl-moi-1", BienBanID: "bb-moi", ThuTu: 1, NoiDung: "Kết luận thứ nhất.",
				TaoLuc: mocTaoBBA},
		},
	}, nil
}

func (g *ghiBienBanGia) ThemKetLuan(ctx context.Context, bienBanID string, yc app.YeuCauThemKetLuan,
	nguoi audit.Actor) (domain.KetLuanHop, error) {

	g.ghi(ctx, "them-ket-luan", nguoi)
	g.bienBanID, g.ycThem = bienBanID, yc
	if g.loi != nil {
		return domain.KetLuanHop{}, g.loi
	}
	// ORDINAL 4 AND NOT 2: the fixture meeting shows two live conclusions, so a route (or a client)
	// deriving the number from the list length would come out with a different answer here.
	return domain.KetLuanHop{
		ID: "kl-moi", BienBanID: bienBanID, ThuTu: 4, NoiDung: yc.NoiDung, TaoLuc: mocTaoKLA,
	}, nil
}

func (g *ghiBienBanGia) TachKetLuanThanhNhiemVu(ctx context.Context, bienBanID string, thuTu int,
	yc app.YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error) {

	g.ghi(ctx, "tach", nguoi)
	g.bienBanID, g.thuTu, g.ycTach = bienBanID, thuTu, yc
	if g.loi != nil {
		return domain.NhiemVu{}, g.loi
	}
	return domain.NhiemVu{
		ID: "nv-moi", Ma: "NV20", Loai: yc.Loai, TieuDe: yc.TieuDe, TrangThai: domain.MoiGiao,
		NguonGiao: domain.NguonKetLuanHop, NguonID: "kl-1", NguoiTaoMa: nguoi.ID,
	}, nil
}

// --- the lifecycle acts (user decisions 25/09/2026) ----------------------------------------------------

func (g *ghiBienBanGia) SuaBienBan(ctx context.Context, id string, yc app.YeuCauSuaBienBan,
	nguoi audit.Actor) (domain.BienBanHop, error) {
	g.ghi(ctx, "sua", nguoi)
	g.bienBanID, g.ycSua = id, yc
	return domain.BienBanHop{ID: id}, g.loi
}

func (g *ghiBienBanGia) XoaBienBan(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.ghi(ctx, "xoa", nguoi)
	g.bienBanID, g.lyDo = id, lyDo
	return g.loi
}

func (g *ghiBienBanGia) KyBienBan(ctx context.Context, id string, yc app.YeuCauKyBienBan,
	nguoi audit.Actor) (domain.BienBanHop, error) {
	g.ghi(ctx, "ky", nguoi)
	g.bienBanID, g.ycKy = id, yc
	return domain.BienBanHop{ID: id}, g.loi
}

func (g *ghiBienBanGia) SuaKetLuan(ctx context.Context, bienBanID string, thuTu int,
	yc app.YeuCauSuaKetLuan, nguoi audit.Actor) (domain.KetLuanHop, error) {
	g.ghi(ctx, "sua-ket-luan", nguoi)
	g.bienBanID, g.thuTu, g.ycSuaKL = bienBanID, thuTu, yc
	return domain.KetLuanHop{}, g.loi
}

func (g *ghiBienBanGia) XoaKetLuan(ctx context.Context, bienBanID string, thuTu int, lyDo string,
	nguoi audit.Actor) error {
	g.ghi(ctx, "xoa-ket-luan", nguoi)
	g.bienBanID, g.thuTu, g.lyDo = bienBanID, thuTu, lyDo
	return g.loi
}

func (g *ghiBienBanGia) DanhDauKhongPhatSinh(ctx context.Context, bienBanID string, thuTu int,
	nguoi audit.Actor) (domain.KetLuanHop, error) {
	g.ghi(ctx, "danh-dau", nguoi)
	g.bienBanID, g.thuTu = bienBanID, thuTu
	return domain.KetLuanHop{}, g.loi
}

func (g *ghiBienBanGia) BoDanhDauKhongPhatSinh(ctx context.Context, bienBanID string, thuTu int,
	nguoi audit.Actor) error {
	g.ghi(ctx, "bo-dau", nguoi)
	g.bienBanID, g.thuTu = bienBanID, thuTu
	return g.loi
}

// --- fixtures ----------------------------------------------------------------------------------------

const (
	duongMeetings = "/api/v1/meetings"
	idBBThu       = "bb-001"
)

func duongKetLuan(id string) string { return duongMeetings + "/" + id + "/conclusions" }
func duongTachNV(id string, stt int) string {
	return duongKetLuan(id) + "/" + itoaThu(stt) + "/task"
}

func itoaThu(n int) string { return strconv.Itoa(n) }

func thanTaoBienBan() taoBienBanVao {
	return taoBienBanVao{
		Title:       "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
		HeldOn:      "2026-08-05",
		ReferenceNo: "31/BB-UBND",
		Location:    "Phòng họp UBND xã",
		ChairedBy:   "CB-00007",
		Conclusions: []string{"Kết luận thứ nhất."},
	}
}

func thanTachNhiemVu() tachKetLuanVao {
	return tachKetLuanVao{
		AutoCode: true,
		Type:     "theo-van-ban",
		Title:    "Rà soát tiến độ tuyến đường Hà Lam – Bình Trị",
		Assigner: "CB-00007",
	}
}

// --- rule 5, invariant 7: four cases per route ----------------------------------------------------
//
// ONE TABLE FOR THE THREE ROUTES, and the table is the point rather than a convenience: written out
// three times, one route's 403 case quietly uses the key that route does not need. Each row names the
// method, the path, the body, THE KEY THAT MUST OPEN IT, and a key of this subsystem that must NOT.

type caGhiBienBan struct {
	ten     string
	duong   string
	than    any
	khoa    authz.Perm
	khoaSai authz.Perm
	ok      int
}

func caCacTuyenGhiBienBan() []caGhiBienBan {
	return []caGhiBienBan{
		// `khoaSai` IS `task.read` — the key the READ route of this very register declares. A 403
		// against a nonsense string would also pass if the route checked nothing at all, and a 403
		// against `task.read` in particular is what proves that being able to LOOK at the minutes
		// does not let anybody write into them.
		{"nhập biên bản", duongMeetings, thanTaoBienBan(),
			authz.Perm("task.create"), authz.Perm("task.read"), http.StatusCreated},
		{"thêm kết luận", duongKetLuan(idBBThu), themKetLuanVao{Content: "Một kết luận mới."},
			authz.Perm("task.create"), authz.Perm("task.read"), http.StatusCreated},
		{"tách thành nhiệm vụ", duongTachNV(idBBThu, 1), thanTachNhiemVu(),
			authz.Perm("task.create"), authz.Perm("task.update"), http.StatusCreated},
	}
}

func TestTuyenGhiBienBanKhongCoPhienThi401(t *testing.T) {
	for _, ca := range caCacTuyenGhiBienBan() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goiGhiNV(t, http.MethodPost, hostA, ca.duong, nil, ca.than)

			doiMa(t, w, http.StatusUnauthorized)
			// THE COUNT IS THE ASSERTION. A route that refused only AFTER touching the data would
			// still have written into a commune's register.
			if m.ghiBienBan.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù chưa có phiên (%d lần)", m.ghiBienBan.goi)
			}
		})
	}
}

func TestTuyenGhiBienBanSaiQuyenThi403(t *testing.T) {
	for _, ca := range caCacTuyenGhiBienBan() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoaSai)

			w := m.goiGhiNV(t, http.MethodPost, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusForbidden)
			if m.ghiBienBan.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù sai quyền (%d lần)", m.ghiBienBan.goi)
			}
		})
	}
}

// TestTuyenGhiBienBanDungQuyenSaiXaThi401 is the case no test of a single commune can produce.
//
// IT ANSWERS 401 AND NOT 403, and that is authz.xacNhanXa's behaviour rather than a slip: the commune
// on the principal is compared with the commune resolved from Host BEFORE the permission is
// consulted. The property asserted is that NO DATA IS TOUCHED — minutes leaking between two communes
// is a breach between two public authorities, not a software bug (rule 1).
func TestTuyenGhiBienBanDungQuyenSaiXaThi401(t *testing.T) {
	for _, ca := range caCacTuyenGhiBienBan() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			w := m.goiGhiNV(t, http.MethodPost, hostB, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusUnauthorized)
			if m.ghiBienBan.goi != 0 {
				t.Errorf("đã GHI vào sổ biên bản của xã B bằng phiên của xã A (%d lần) — "+
					"đây là rò rỉ giữa hai cơ quan nhà nước, không phải một lỗi phần mềm thường",
					m.ghiBienBan.goi)
			}
		})
	}
}

func TestTuyenGhiBienBanDungQuyenDungXaThiQua(t *testing.T) {
	for _, ca := range caCacTuyenGhiBienBan() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			w := m.goiGhiNV(t, http.MethodPost, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, ca.ok)
			if m.ghiBienBan.goi != 1 {
				t.Fatalf("gọi use case %d lần, muốn 1", m.ghiBienBan.goi)
			}
			// THE COMMUNE THE USE CASE SAW IS THE ONE RESOLVED FROM Host, never one a client named.
			if m.ghiBienBan.xa != xaA {
				t.Errorf("use case thấy xã %q, muốn %q", m.ghiBienBan.xa, xaA)
			}
		})
	}
}

// TestTuyenGhiBienBanChuTheLaMaCanBo pins rule 6, invariant 8 at the boundary it is broken at.
//
// `idCanBo` AND `maCanBo` ARE DIFFERENT STRINGS IN THE FIXTURES on purpose. A handler passing
// `Principal.ID` would produce a perfectly valid-looking audit entry naming nobody — which is exactly
// what six write paths in this repository did until 2026-09-22, with no test turning red.
func TestTuyenGhiBienBanChuTheLaMaCanBo(t *testing.T) {
	for _, ca := range caCacTuyenGhiBienBan() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			m.goiGhiNV(t, http.MethodPost, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			if m.ghiBienBan.nguoi.ID != maCanBo {
				t.Errorf("chủ thể = %q, muốn mã cán bộ %q — một ULID ở cột này không gọi tên ai",
					m.ghiBienBan.nguoi.ID, maCanBo)
			}
			if m.ghiBienBan.nguoi.Kind != "staff" {
				t.Errorf("loại chủ thể = %q, muốn staff", m.ghiBienBan.nguoi.Kind)
			}
			if m.ghiBienBan.nguoi.IP == "" {
				t.Error("vết không mang địa chỉ IP — luật 6 bất biến 2 đòi 'từ IP nào'")
			}
		})
	}
}

// --- the bodies reach the use case, and come back ----------------------------------------------------

// TestTaoBienBan_NgayHopLaNgayLichCaHaiChieu is the one field on this form that can be wrong without
// anybody noticing: the column is DATE, the card renders `5/8/2026`, and an RFC 3339 instant would
// let a browser's time zone decide which DAY the minutes were filed under.
func TestTaoBienBan_NgayHopLaNgayLichCaHaiChieu(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongMeetings, canBoCuaXa(xaA), thanTaoBienBan())
	doiMa(t, w, http.StatusCreated)

	ngay := m.ghiBienBan.ycTao.NgayHop
	if !ngay.Equal(time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày họp tới use case = %v, muốn 2026-08-05 nửa đêm UTC", ngay)
	}

	var ra bienBanRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", w.Body.String())
	}
	if ra.HeldOn != "2026-08-05" {
		t.Errorf("held_on = %q, muốn `2026-08-05` (ngày lịch, không phải mốc thời gian)", ra.HeldOn)
	}
	// THE CONCLUSION IDS ARE THE REASON THIS ROUTE ANSWERS WITH THE WHOLE CARD: §3's split sends one
	// of them back, and without it the back-link of §1 cannot be created.
	if len(ra.Conclusions) != 1 || ra.Conclusions[0].ID == "" || ra.Conclusions[0].Ordinal != 1 {
		t.Errorf("kết luận trong phản hồi: %+v", ra.Conclusions)
	}
}

// TestTaoBienBan_NgayHopHongThi400VaKhongChamDuLieu — a malformed date is the client's fault, and the
// register must not be touched to find that out.
func TestTaoBienBan_NgayHopHongThi400VaKhongChamDuLieu(t *testing.T) {
	for _, ca := range []struct{ ten, ngay string }{
		{"thiếu ngày họp", ""},
		{"ngày họp là mốc thời gian RFC 3339", "2026-08-05T00:00:00Z"},
		{"ngày họp không đọc được", "05/08/2026"},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.create"))

			than := thanTaoBienBan()
			than.HeldOn = ca.ngay

			w := m.goiGhiNV(t, http.MethodPost, hostA, duongMeetings, canBoCuaXa(xaA), than)
			doiMa(t, w, http.StatusBadRequest)
			if m.ghiBienBan.goi != 0 {
				t.Error("đã gọi use case với một ngày họp không đọc được")
			}
			if strings.Contains(w.Body.String(), ca.ngay) && ca.ngay != "" {
				t.Errorf("thân lỗi dội lại chuỗi client gửi: %s", w.Body.String())
			}
		})
	}
}

// TestTaoBienBan_TruongTuyChonVaKetLuanDiNguyenVenXuongUseCase — §4 marks most of the form optional
// and §7.3 saves minutes with no conclusions at all.
func TestTaoBienBan_TruongTuyChonVaKetLuanDiNguyenVenXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	than := thanTaoBienBan()
	than.Content = "Toàn văn biên bản."
	than.Attendees = []string{"CB-00007", "Đại diện Mặt trận Tổ quốc xã"}
	than.Conclusions = []string{"Kết luận ①.", "Kết luận ②."}

	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongMeetings, canBoCuaXa(xaA), than),
		http.StatusCreated)

	yc := m.ghiBienBan.ycTao
	if yc.TenCuocHop != than.Title || yc.SoHieu != than.ReferenceNo || yc.DiaDiem != than.Location ||
		yc.ChuTriMa != than.ChairedBy || yc.NoiDung != than.Content {
		t.Errorf("yêu cầu tới use case = %+v", yc)
	}
	if len(yc.ThanhPhan) != 2 || len(yc.KetLuan) != 2 || yc.KetLuan[1] != "Kết luận ②." {
		t.Errorf("thành phần/kết luận tới use case = %+v / %+v", yc.ThanhPhan, yc.KetLuan)
	}
}

// TestThemKetLuan_SoThuTuLaCuaMayChuChuKhongPhaiCuaClient — §7.2 numbers from the high-water mark, so
// the ordinal is something only the server can know. The fake answers ④ for a meeting showing two
// live conclusions; a route inventing `len + 1` would produce ③.
func TestThemKetLuan_SoThuTuLaCuaMayChuChuKhongPhaiCuaClient(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongKetLuan(idBBThu), canBoCuaXa(xaA),
		themKetLuanVao{Content: "Giao Tài chính – Kế toán đối chiếu số liệu giải ngân."})
	doiMa(t, w, http.StatusCreated)

	if m.ghiBienBan.bienBanID != idBBThu {
		t.Errorf("mã biên bản tới use case = %q, muốn %q", m.ghiBienBan.bienBanID, idBBThu)
	}
	if m.ghiBienBan.ycThem.NoiDung == "" {
		t.Error("nội dung kết luận không tới use case")
	}

	var ra ketLuanRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", w.Body.String())
	}
	if ra.Ordinal != 4 {
		t.Errorf("ordinal = %d, muốn 4 — số thứ tự nối tiếp số ĐÃ CẤP, không đếm dòng còn sống",
			ra.Ordinal)
	}
	if ra.ID == "" || ra.CreatedAt.IsZero() {
		t.Errorf("kết luận trả về thiếu id hoặc thời điểm tạo: %+v", ra)
	}
}

// TestTachKetLuan_DuongDanDiXuongUseCaseVaThanKhongDoi — §3 confirms a whole task form, and the
// server derives nothing from the conclusion's sentence.
func TestTachKetLuan_DuongDanDiXuongUseCaseVaThanKhongDoi(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	han := time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)
	than := thanTachNhiemVu()
	than.DueAt = &han
	than.Parent = "nv-cha-001"

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTachNV(idBBThu, 3), canBoCuaXa(xaA), than)
	doiMa(t, w, http.StatusCreated)

	if m.ghiBienBan.bienBanID != idBBThu || m.ghiBienBan.thuTu != 3 {
		t.Errorf("đường dẫn tới use case = %q / %d, muốn %q / 3",
			m.ghiBienBan.bienBanID, m.ghiBienBan.thuTu, idBBThu)
	}
	yc := m.ghiBienBan.ycTach
	if yc.TieuDe != than.Title || yc.Loai != than.Type || !yc.TuSinhMa ||
		yc.LanhDaoGiaoViecMa != than.Assigner || yc.NhiemVuChaID != "nv-cha-001" {
		t.Errorf("thân yêu cầu tới use case = %+v", yc)
	}
	if !yc.HanXuLy.Equal(han) {
		t.Errorf("hạn xử lý = %v, muốn %v — hạn là thứ người dùng xác nhận", yc.HanXuLy, han)
	}
	// THE SOURCE PAIR IS THE USE CASE'S. The handler must not put anything there: the value comes
	// from the conclusion named in the PATH.
	if yc.NguonGiao != "" || yc.NguonID != "" {
		t.Errorf("handler tự điền cặp nguồn giao (%q/%q) — §3 khoá trường ấy, và giá trị phải lấy "+
			"từ kết luận trên đường dẫn", yc.NguonGiao, yc.NguonID)
	}
}

// TestTachKetLuan_ClientKhongChiDinhDuocNguon is the security property of §3's locked field: even a
// client that sends `source` / `source_id` cannot influence where the task points, because the
// request type has no such fields at all.
func TestTachKetLuan_ClientKhongChiDinhDuocNguon(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	than := map[string]any{
		"auto_code": true,
		"type":      "theo-van-ban",
		"title":     "Rà soát tiến độ tuyến đường",
		"source":    "phan-anh",
		"source_id": "phieu-cua-xa-khac",
	}

	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongTachNV(idBBThu, 1), canBoCuaXa(xaA), than),
		http.StatusCreated)

	yc := m.ghiBienBan.ycTach
	if yc.NguonGiao != "" || yc.NguonID != "" {
		t.Errorf("client chỉ định được nguồn giao (%q/%q) — một nhiệm vụ có thể trỏ vào bản ghi bất kỳ",
			yc.NguonGiao, yc.NguonID)
	}
}

// TestTachKetLuan_SoThuTuKhongHopLeThi400 — a path segment that is not a positive integer is a
// malformed request, refused before the register is touched.
func TestTachKetLuan_SoThuTuKhongHopLeThi400(t *testing.T) {
	for _, stt := range []string{"khong-phai-so", "0", "-1"} {
		t.Run(stt, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.create"))

			w := m.goiGhiNV(t, http.MethodPost, hostA,
				duongKetLuan(idBBThu)+"/"+stt+"/task", canBoCuaXa(xaA), thanTachNhiemVu())

			doiMa(t, w, http.StatusBadRequest)
			if m.ghiBienBan.goi != 0 {
				t.Error("đã gọi use case với một số thứ tự không hợp lệ")
			}
		})
	}
}

// --- the refusals map onto the right status codes ------------------------------------------------------

// TestLoiBienBanAnhXaDungMa keeps the 404 / 400 / 500 line honest, and the last row is the one that
// matters most: a store failure answered as a 400 makes a client retry with different input for ever
// while nobody is told the server is broken.
func TestLoiBienBanAnhXaDungMa(t *testing.T) {
	for _, ca := range []struct {
		ten  string
		loi  error
		muon int
	}{
		{"không có biên bản", petstore.ErrBienBanKhongTonTai, http.StatusNotFound},
		{"không có kết luận", petstore.ErrKetLuanKhongTonTai, http.StatusNotFound},
		{"thiếu tên cuộc họp", domain.ErrThieuTenCuocHop, http.StatusBadRequest},
		{"kết luận rỗng", domain.ErrThieuNoiDungKetLuan, http.StatusBadRequest},
		{"quá nhiều kết luận", domain.ErrQuaNhieuKetLuan, http.StatusBadRequest},
		{"lỗi hệ thống", errors.New("kết nối cơ sở dữ liệu hỏng"), http.StatusInternalServerError},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.create"))
			m.ghiBienBan.loi = ca.loi

			w := m.goiGhiNV(t, http.MethodPost, hostA, duongMeetings, canBoCuaXa(xaA),
				thanTaoBienBan())

			doiMa(t, w, ca.muon)
		})
	}
}

// TestTachKetLuan_LoiCuaSoNhiemVuAnhXaNhuTuyenTaoNhiemVu — one act, one set of answers, whichever
// door it came through: a refusal from the task register must get the SAME status here as it gets on
// POST /api/v1/tasks.
func TestTachKetLuan_LoiCuaSoNhiemVuAnhXaNhuTuyenTaoNhiemVu(t *testing.T) {
	for _, ca := range []struct {
		ten  string
		loi  error
		muon int
	}{
		{"không có kết luận", petstore.ErrKetLuanKhongTonTai, http.StatusNotFound},
		{"mã nhiệm vụ đã dùng", petstore.ErrMaNhiemVuDaTonTai, http.StatusConflict},
		{"không có nhiệm vụ cha", domain.ErrChaKhongTonTai, http.StatusConflict},
		{"chu trình cây", domain.ErrChuTrinhCayNhiemVu, http.StatusConflict},
		{"thiếu tiêu đề", domain.ErrThieuTieuDeNhiemVu, http.StatusBadRequest},
		{"thiếu loại nhiệm vụ", domain.ErrThieuLoaiNhiemVu, http.StatusBadRequest},
		{"lỗi hệ thống", errors.New("kết nối cơ sở dữ liệu hỏng"), http.StatusInternalServerError},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.create"))
			m.ghiBienBan.loi = ca.loi

			w := m.goiGhiNV(t, http.MethodPost, hostA, duongTachNV(idBBThu, 1), canBoCuaXa(xaA),
				thanTachNhiemVu())

			doiMa(t, w, ca.muon)
		})
	}
}

// TestLoiBienBanKhongLoNoiDungHeThongRaClient — this register's columns hold a commune's minutes, so
// a store failure's text must not reach the wire (rule 3, forbidden #3).
func TestLoiBienBanKhongLoNoiDungHeThongRaClient(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))
	m.ghiBienBan.loi = errors.New("pq: connection to 10.1.2.3:5432 refused")

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongMeetings, canBoCuaXa(xaA), thanTaoBienBan())

	doiMa(t, w, http.StatusInternalServerError)
	if than := w.Body.String(); contains(than, "10.1.2.3") || contains(than, "pq:") {
		t.Errorf("thân lỗi lộ chi tiết hệ thống: %s", than)
	}
}
