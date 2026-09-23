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
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for GET /api/v1/meetings.
//
// THE FOUR CASES RULE 5 INVARIANT 7 REQUIRES ARE ALL HERE, and the third is the one no test of a
// single commune can produce: 401 with no session · 403 with the wrong permission · 401 with the
// right permission but a session issued by ANOTHER commune · 200 with both correct.
//
// The commune case answers 401 rather than 403, and that is authz.xacNhanXa's behaviour, not a slip:
// the commune on the principal is compared with the commune resolved from Host BEFORE the permission
// is consulted, so a token from another commune never reaches the permission check. The property
// asserted is that NO QUERY RUNS — `goi` on the fake counts the reads.

// --- fakes ------------------------------------------------------------------------------------

// bienBanGia is the meeting register, KEYED BY COMMUNE, reading the commune from the context exactly
// as *store.Scoped does. Keyed any other way, the isolation cases would pass while proving nothing.
type bienBanGia struct {
	theo map[tenant.ID][]domain.BienBanHop
	loi  error
	goi  int
}

func (b *bienBanGia) DanhSach(ctx context.Context, _ page.Request) (
	page.Result[domain.BienBanHop], error) {

	b.goi++
	if b.loi != nil {
		return page.Result[domain.BienBanHop]{}, b.loi
	}
	kq := page.NewResult[domain.BienBanHop]()
	kq.Items = append(kq.Items, b.theo[tenant.MustFrom(ctx)]...)
	return kq, nil
}

// The fixture instants are FIXED, not relative to time.Now(): the one thing these dates must do is
// come out of the response byte for byte.
var (
	mocNgayHopA = time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	mocTaoBBA   = time.Date(2026, 8, 6, 3, 30, 0, 0, time.UTC)
	mocTaoKLA   = time.Date(2026, 8, 6, 4, 0, 0, 0, time.UTC)
)

// bienBanMau gives commune A §8's sample meeting — three conclusions with the specification's own
// counters (0/1, 0/1, 1/1, badge 1/3) — plus a SECOND meeting that exercises the two states the
// first cannot: no reference number, no location, no chair, and no conclusions at all (§7.3 saves
// such minutes).
//
// Commune B holds one meeting of its own, so a leak has something distinguishable to leak.
func bienBanMau() *bienBanGia {
	return &bienBanGia{theo: map[tenant.ID][]domain.BienBanHop{
		xaA: {
			{
				ID:         "bb-001",
				TenCuocHop: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
				NgayHop:    mocNgayHopA,
				SoHieu:     "31/BB-UBND",
				DiaDiem:    "Phòng họp UBND xã",
				ChuTriMa:   "CB-00007",
				NguoiTaoMa: maCanBo,
				TaoLuc:     mocTaoBBA,
				KetLuan: []domain.KetLuanHop{
					{ID: "kl-1", BienBanID: "bb-001", ThuTu: 1, TaoLuc: mocTaoKLA,
						NoiDung:   "Giao Địa chính rà soát tiến độ tuyến đường, báo cáo trước ngày 20/8.",
						SoNhiemVu: 1, SoNhiemVuXong: 0},
					{ID: "kl-2", BienBanID: "bb-001", ThuTu: 2, TaoLuc: mocTaoKLA,
						NoiDung:   "Giao Văn hoá – Xã hội hoàn tất hồ sơ hỗ trợ sinh kế đợt 3.",
						SoNhiemVu: 1, SoNhiemVuXong: 0},
					{ID: "kl-3", BienBanID: "bb-001", ThuTu: 3, TaoLuc: mocTaoKLA,
						NoiDung:   "Giao Tài chính – Kế toán đối chiếu số liệu giải ngân sáu tháng.",
						SoNhiemVu: 1, SoNhiemVuXong: 1},
				},
			},
			{
				ID:         "bb-002",
				TenCuocHop: "giao ban",
				NgayHop:    time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC),
				NguoiTaoMa: maCanBo,
				TaoLuc:     mocTaoBBA,
			},
		},
		xaB: {
			{
				ID:         "bb-b-001",
				TenCuocHop: "Giao ban xã B",
				NgayHop:    mocNgayHopA,
				NguoiTaoMa: maCanBo,
				TaoLuc:     mocTaoBBA,
			},
		},
	}}
}

func docTrangBienBan(t *testing.T, than []byte) page.Result[bienBanRa] {
	t.Helper()
	var ra page.Result[bienBanRa]
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

// --- the four permission cases -------------------------------------------------------------------

func TestDanhSachBienBanKhongPhienThi401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", nil)
	doiMa(t, w, http.StatusUnauthorized)
	if m.bienBan.goi != 0 {
		t.Error("đã đọc kho dù chưa đăng nhập — phép kiểm quyền phải chạy TRƯỚC mọi truy vấn")
	}
}

func TestDanhSachBienBanThieuQuyenThi403(t *testing.T) {
	m := dungMayChu(t)
	// Commune B's account holds NOTHING in the harness's checker.
	w := m.goi(t, http.MethodGet, hostB, "/api/v1/meetings", canBoCuaXa(xaB))
	doiMa(t, w, http.StatusForbidden)
	if m.bienBan.goi != 0 {
		t.Error("đã đọc kho dù thiếu quyền `task.read`")
	}
}

// TestDanhSachBienBanDungQuyenNhungSaiXaThi401 is the case no single-commune test can produce: a
// valid session, holding the right key IN ITS OWN COMMUNE, used against another commune's host.
func TestDanhSachBienBanDungQuyenNhungSaiXaThi401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostB, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusUnauthorized)
	if m.bienBan.goi != 0 {
		t.Error("ĐÃ CHẠY TRUY VẤN với phiên của xã khác — dữ liệu một xã đã rời khỏi ranh giới của nó")
	}
}

func TestDanhSachBienBanDuQuyenThi200(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if m.bienBan.goi != 1 {
		t.Errorf("gọi kho %d lần, muốn 1", m.bienBan.goi)
	}
}

// --- what comes back -------------------------------------------------------------------------------

// TestDanhSachBienBanTraDungThongTinThe checks every field the card of §2 renders, and the two
// counters in particular: the badge is `{n} kết luận · {x}/{y} nhiệm vụ xong`, and §7.5 says the
// fraction is the SUM over the conclusions.
func TestDanhSachBienBanTraDungThongTinThe(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	trang := docTrangBienBan(t, w.Body.Bytes())
	if len(trang.Items) != 2 {
		t.Fatalf("trả %d biên bản, muốn 2", len(trang.Items))
	}
	b := trang.Items[0]

	if b.ID != "bb-001" || b.Title != "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026" {
		t.Errorf("id/tiêu đề sai: %+v", b)
	}
	// A CALENDAR DAY, not an instant: `2026-08-05` and nothing after it. A timestamp here renders as
	// the previous day for a browser one time zone away — on a card whose content is that date.
	if b.HeldOn != "2026-08-05" {
		t.Errorf("held_on = %q, muốn %q (ngày lịch, không phải mốc thời gian)", b.HeldOn, "2026-08-05")
	}
	if b.ReferenceNo != "31/BB-UBND" || b.Location != "Phòng họp UBND xã" || b.ChairedBy != "CB-00007" {
		t.Errorf("dòng meta sai: %+v", b)
	}
	if len(b.Conclusions) != 3 {
		t.Fatalf("trả %d kết luận, muốn 3", len(b.Conclusions))
	}
	if b.TaskCount != 3 || b.TaskDoneCount != 1 {
		t.Errorf("badge = %d/%d, muốn 1/3 — §7.5 cộng dồn trên mọi kết luận",
			b.TaskDoneCount, b.TaskCount)
	}

	// The circles are ① ② ③ — the ordinal is the stored number, never the array index.
	for i, k := range b.Conclusions {
		if k.Ordinal != i+1 {
			t.Errorf("kết luận %d mang số thứ tự %d", i, k.Ordinal)
		}
	}
	cuoi := b.Conclusions[2]
	if cuoi.ID != "kl-3" || cuoi.TaskCount != 1 || cuoi.TaskDoneCount != 1 {
		t.Errorf("kết luận cuối sai: %+v", cuoi)
	}
	// The conclusion id is what §3 sends back as `nguon_id` when the conclusion is split into a
	// task. Without it on the wire the whole back-link of §1 cannot be created.
	if b.Conclusions[0].ID == "" {
		t.Error("kết luận không mang id — không tách thành nhiệm vụ được, và §1 mất đường truy vết")
	}
}

// TestBienBanKhongCoKetLuanTraMangRongChuKhongPhaiNull — §7.3 saves minutes with no conclusions, and
// a client that has to handle both `[]` and `null` handles one of them wrong.
func TestBienBanKhongCoKetLuanTraMangRongChuKhongPhaiNull(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"conclusions":[]`) {
		t.Errorf("biên bản chưa có kết luận trả `null` thay vì `[]`: %s", w.Body.String())
	}
	trang := docTrangBienBan(t, w.Body.Bytes())
	nhap := trang.Items[1]
	if nhap.TaskCount != 0 || nhap.TaskDoneCount != 0 {
		t.Errorf("badge của biên bản nháp = %d/%d, muốn 0/0", nhap.TaskDoneCount, nhap.TaskCount)
	}
	// The optional fields of §4 come back empty, not as a placeholder the screen would print.
	if nhap.ReferenceNo != "" || nhap.Location != "" || nhap.ChairedBy != "" {
		t.Errorf("trường tuỳ chọn không rỗng trên biên bản nháp: %+v", nhap)
	}
}

// TestDanhSachBienBanChiTraBienBanCuaXaMinh — the same request against the other commune's host
// returns THAT commune's minutes and nothing of this one's.
func TestDanhSachBienBanChiTraBienBanCuaXaMinh(t *testing.T) {
	m := dungMayChu(t)
	// Commune B's account has to hold the key for this case to reach the store at all.
	m.d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
		xaA: {idCanBo: {authz.Perm("task.read"): true}},
		xaB: {idCanBo: {authz.Perm("task.read"): true}},
	}}
	m.dungLai(t, nil)

	w := m.goi(t, http.MethodGet, hostB, "/api/v1/meetings", canBoCuaXa(xaB))
	doiMa(t, w, http.StatusOK)

	trang := docTrangBienBan(t, w.Body.Bytes())
	if len(trang.Items) != 1 || trang.Items[0].ID != "bb-b-001" {
		t.Fatalf("xã B nhận %d biên bản: %+v", len(trang.Items), trang.Items)
	}
	for _, b := range trang.Items {
		if strings.Contains(b.Title, "Uỷ ban nhân dân xã tháng 8") {
			t.Error("BIÊN BẢN CỦA XÃ A LỌT SANG XÃ B — rò rỉ giữa hai cơ quan nhà nước")
		}
	}
}

// TestDanhSachBienBanXaRongTraMangRong — a newly onboarded commune. `items` must be `[]`, never
// `null`.
func TestDanhSachBienBanXaRongTraMangRong(t *testing.T) {
	m := dungMayChu(t)
	m.bienBan.theo[xaA] = nil

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("xã chưa có biên bản nào trả `null` thay vì `[]`: %s", w.Body.String())
	}
}

// TestDanhSachBienBanConTroHongThi400 — page.Parse refuses before the store is touched, and the body
// never echoes what the client sent: a cursor is opaque and a rejected sort key is often a probe.
func TestDanhSachBienBanConTroHongThi400(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings?cursor=khong-phai-con-tro",
		canBoCuaXa(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.bienBan.goi != 0 {
		t.Error("đã chạy truy vấn dù yêu cầu phân trang bị từ chối")
	}
	if strings.Contains(w.Body.String(), "khong-phai-con-tro") {
		t.Errorf("thân lỗi dội lại chuỗi client gửi: %s", w.Body.String())
	}
}

// TestDanhSachBienBanLoiKhoThi500KhongLoNoiDung — the minutes and the conclusions are free text a
// clerk typed, and a commune's minutes quote cases. A store failure must come back as the same
// sentence every other route uses, with nothing of the register in it (rule 3, forbidden #3).
func TestDanhSachBienBanLoiKhoThi500KhongLoNoiDung(t *testing.T) {
	m := dungMayChu(t)
	m.bienBan.loi = errors.New("kết nối rụng: bien_ban_hop")

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "bien_ban_hop") ||
		strings.Contains(w.Body.String(), "kết nối rụng") {
		t.Errorf("lỗi hệ thống lọt ra ngoài: %s", w.Body.String())
	}
}
