package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for GET /api/v1/meetings/{id}, GET /api/v1/meetings/{id}/conclusions/{stt}/tasks, the new
// fields on the list, and the task back-link.
//
// THE FOUR CASES OF RULE 5 INVARIANT 7 FOR EACH NEW ROUTE: 401 no session · 403 wrong permission ·
// 401 right permission but ANOTHER commune's session (authz compares the commune before the
// permission — see bien_ban_hop_test.go's header) · 200. The property asserted on every refusal is
// that NO READ RAN — `goi` on the fake counts them.
//
// NOT PROVED HERE: which rows the store excludes. The fake answers ErrBienBanKhongTonTai for an id
// it does not hold; the SQL predicates (tenant, soft delete, the blurred pair) are asserted in
// internal/store on the fake driver, and against a real server in the skipped *_pg_test.go.

const (
	duongBienBan001  = "/api/v1/meetings/bb-001"
	duongNhiemVuKL11 = "/api/v1/meetings/bb-001/conclusions/1/tasks"
)

func docBienBanMot(t *testing.T, than []byte) bienBanRa {
	t.Helper()
	var ra bienBanRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

// --- rule 5 matrix, both routes --------------------------------------------------------------------

func TestTuyenDocBienBanMoiMaTranQuyen(t *testing.T) {
	for _, duong := range []string{duongBienBan001, duongNhiemVuKL11} {
		t.Run(duong, func(t *testing.T) {
			// 401 — no session.
			m := dungMayChu(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, duong, nil), http.StatusUnauthorized)
			if m.bienBan.goi != 0 {
				t.Error("đã đọc kho dù chưa đăng nhập")
			}

			// 403 — commune A's account holding a DIFFERENT key only.
			m = dungMayChu(t)
			m.dungLai(t, func(d *Deps) {
				d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
					xaA: {idCanBo: {authz.Perm("task.create"): true}},
				}}
			})
			doiMa(t, m.goi(t, http.MethodGet, hostA, duong, canBoCuaXa(xaA)), http.StatusForbidden)
			if m.bienBan.goi != 0 {
				t.Error("đã đọc kho dù thiếu `task.read`")
			}

			// Wrong commune — commune B's session, holding `task.read` in BOTH communes, at A's host.
			m = dungMayChu(t)
			m.dungLai(t, func(d *Deps) {
				d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
					xaA: {idCanBo: {authz.Perm("task.read"): true}},
					xaB: {idCanBo: {authz.Perm("task.read"): true}},
				}}
			})
			doiMa(t, m.goi(t, http.MethodGet, hostA, duong, canBoCuaXa(xaB)), http.StatusUnauthorized)
			if m.bienBan.goi != 0 {
				t.Error("ĐÃ CHẠY TRUY VẤN với phiên của xã khác")
			}

			// 200 — right key, right commune.
			m = dungMayChu(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, duong, canBoCuaXa(xaA)), http.StatusOK)
			if m.bienBan.goi != 1 {
				t.Errorf("gọi kho %d lần, muốn 1", m.bienBan.goi)
			}
		})
	}
}

// --- GET /api/v1/meetings/{id} ---------------------------------------------------------------------

func TestDocBienBanTraDuTruong(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongBienBan001, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	b := docBienBanMot(t, w.Body.Bytes())

	if b.ID != "bb-001" || b.HeldOn != "2026-08-05" || b.ReferenceNo != "31/BB-UBND" ||
		b.Location != "Phòng họp UBND xã" || b.ChairedBy != "CB-00007" {
		t.Errorf("trường cũ sai: %+v", b)
	}
	if b.Status != domain.TrangThaiBienBanDaKy {
		t.Errorf("status = %q, muốn da-ky", b.Status)
	}
	if b.SignedAt == nil || !b.SignedAt.Equal(mocKyBBA) || b.SignedBy != "CB-00031" {
		t.Errorf("thời điểm/người ký sai: %v %q", b.SignedAt, b.SignedBy)
	}
	if b.Secretary != "CB-00042" {
		t.Errorf("secretary = %q", b.Secretary)
	}
	// The notice day is a CALENDAR DAY, like held_on.
	if b.Notice == nil || b.Notice.ReferenceNo != "45/TB-UBND" || b.Notice.IssuedOn != "2026-08-12" {
		t.Errorf("notice sai: %+v", b.Notice)
	}
	if b.Content == nil || *b.Content != "Toàn văn biên bản giao ban tháng 8." {
		t.Errorf("content sai: %v", b.Content)
	}
	if b.Attendees == nil || len(*b.Attendees) != 3 || (*b.Attendees)[2] != "Đại diện Mặt trận xã" {
		t.Errorf("attendees sai: %v", b.Attendees)
	}
	if b.SupplementedBy == nil || len(*b.SupplementedBy) != 1 || (*b.SupplementedBy)[0] != "bb-003" {
		t.Errorf("supplemented_by sai: %v", b.SupplementedBy)
	}
	if b.SupplementsID != "" {
		t.Errorf("biên bản gốc lại mang supplements_id = %q", b.SupplementsID)
	}

	// The conclusions carry the derived status; the card figures carry both counts.
	muon := []string{"qua-han", "dang-thuc-hien", "hoan-thanh"}
	if len(b.Conclusions) != 3 {
		t.Fatalf("trả %d kết luận, muốn 3", len(b.Conclusions))
	}
	for i, k := range b.Conclusions {
		if k.Status != muon[i] || k.NoTask {
			t.Errorf("kết luận %d: status=%q no_task=%v, muốn %q/false", k.Ordinal, k.Status, k.NoTask, muon[i])
		}
	}
	if b.ConclusionCount != 3 || b.ConclusionDoneCount != 1 {
		t.Errorf("x/y kết luận = %d/%d, muốn 1/3", b.ConclusionDoneCount, b.ConclusionCount)
	}
	if b.TaskCount != 3 || b.TaskDoneCount != 1 {
		t.Errorf("x/y nhiệm vụ = %d/%d, muốn 1/3 (giữ nguyên)", b.TaskDoneCount, b.TaskCount)
	}
}

// TestDocBienBanNhapVaBoSungKhongDauKhongPhatSinh — a DRAFT that supplements another, with no
// conclusions but one marked "không phát sinh": absent-able fields are absent, detail-only fields are
// PRESENT and empty, and the mark counts as done.
func TestDocBienBanNhapVaBoSungKhongDauKhongPhatSinh(t *testing.T) {
	m := dungMayChu(t)
	m.bienBan.theo[xaA] = append(m.bienBan.theo[xaA], domain.BienBanHop{
		ID: "bb-003", TenCuocHop: "Bổ sung biên bản giao ban tháng 8", NgayHop: mocNgayHopA,
		NguoiTaoMa: maCanBo, TaoLuc: mocTaoBBA, TrangThai: domain.TrangThaiBienBanDuThao,
		BoSungChoID: "bb-001", ThanhPhan: []string{}, DuocBoSungBoi: []string{},
		KetLuan: []domain.KetLuanHop{{ID: "kl-bs-1", BienBanID: "bb-003", ThuTu: 1,
			NoiDung: "Không phát sinh việc mới.", KhongPhatSinh: true, TaoLuc: mocTaoKLA}},
	})

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings/bb-003", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	than := w.Body.String()
	for _, vang := range []string{`"signed_at"`, `"signed_by"`, `"secretary"`, `"notice"`} {
		if strings.Contains(than, vang) {
			t.Errorf("biên bản nháp mang trường %s: %s", vang, than)
		}
	}
	for _, co := range []string{`"supplemented_by":[]`, `"attendees":[]`, `"content":""`,
		`"supplements_id":"bb-001"`, `"status":"du-thao"`} {
		if !strings.Contains(than, co) {
			t.Errorf("thiếu %s trên tuyến chi tiết: %s", co, than)
		}
	}
	b := docBienBanMot(t, w.Body.Bytes())
	if k := b.Conclusions[0]; k.Status != "hoan-thanh" || !k.NoTask || k.TaskCount != 0 {
		t.Errorf("kết luận đánh dấu không phát sinh: %+v", k)
	}
	if b.ConclusionDoneCount != 1 || b.ConclusionCount != 1 {
		t.Errorf("x/y kết luận = %d/%d, muốn 1/1", b.ConclusionDoneCount, b.ConclusionCount)
	}
}

// TestDocBienBan404MotThanChoBaTruongHop — unknown id, a soft-deleted one, and ANOTHER COMMUNE's id
// answer byte-for-byte the same body.
func TestDocBienBan404MotThanChoBaTruongHop(t *testing.T) {
	m := dungMayChu(t)
	var thans []string
	for _, id := range []string{"bb-khong-co", "bb-da-xoa", "bb-b-001"} {
		w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings/"+id, canBoCuaXa(xaA))
		doiMa(t, w, http.StatusNotFound)
		thans = append(thans, w.Body.String())
	}
	if thans[0] != thans[1] || thans[1] != thans[2] {
		t.Errorf("ba nguyên nhân 404 trả ba thân khác nhau — lộ sự tồn tại của hồ sơ:\n%s", strings.Join(thans, "\n"))
	}
	if strings.Contains(thans[2], "Giao ban xã B") {
		t.Error("BIÊN BẢN CỦA XÃ B LỌT SANG XÃ A")
	}
}

func TestDocBienBanLoiKhoThi500KhongLoNoiDung(t *testing.T) {
	m := dungMayChu(t)
	m.bienBan.loi = errors.New("kết nối rụng: bien_ban_hop")
	w := m.goi(t, http.MethodGet, hostA, duongBienBan001, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "bien_ban_hop") {
		t.Errorf("lỗi hệ thống lọt ra ngoài: %s", w.Body.String())
	}
}

// --- the list carries the new fields ------------------------------------------------------------------

// TestDanhSachBienBanMangTrangThaiVaDemKetLuan — status and the conclusion figure on every card; the
// detail-only fields are ABSENT from the list (absent = "not carried here", not "none").
func TestDanhSachBienBanMangTrangThaiVaDemKetLuan(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	trang := docTrangBienBan(t, w.Body.Bytes())
	if trang.Items[0].Status != "da-ky" || trang.Items[1].Status != "du-thao" {
		t.Errorf("status trên thẻ: %q, %q", trang.Items[0].Status, trang.Items[1].Status)
	}
	if trang.Items[0].ConclusionCount != 3 || trang.Items[0].ConclusionDoneCount != 1 {
		t.Errorf("thẻ 1: %d/%d kết luận", trang.Items[0].ConclusionDoneCount, trang.Items[0].ConclusionCount)
	}
	if trang.Items[0].Conclusions[0].Status != "qua-han" {
		t.Errorf("kết luận ① trên thẻ: %q", trang.Items[0].Conclusions[0].Status)
	}
	// Checked on the CARD's own keys: a conclusion legitimately carries its own `content`.
	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatal(err)
	}
	for _, the := range tho.Items {
		for _, vang := range []string{"content", "attendees", "supplemented_by"} {
			if _, co := the[vang]; co {
				t.Errorf("danh sách mang trường chỉ-chi-tiết %q", vang)
			}
		}
	}
}

// --- GET /api/v1/meetings/{id}/conclusions/{stt}/tasks -------------------------------------------------

func TestNhiemVuCuaKetLuanTraHangCuaSoNhiemVu(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVuKL11, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var ra nhiemVuKetLuanRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if len(ra.Items) != 1 {
		t.Fatalf("trả %d nhiệm vụ, muốn 1", len(ra.Items))
	}
	n := ra.Items[0]
	// The register's own row shape: code, title, assignee, unit, status, deadline.
	if n.Code != "NV07" || n.Title != "Rà soát tiến độ tuyến đường" || n.Assignee != "CB-00311" ||
		n.Unit != "bp-dia-chinh" || n.Status != "dang-thuc-hien" || n.Source != "ket-luan-hop" ||
		n.SourceID != "kl-1" || n.DueAt == nil || !n.DueAt.Equal(mocHanNVKL) {
		t.Errorf("hàng nhiệm vụ sai: %+v", n)
	}
	// NO `overdue` field: the row shape derives it client-side from due_at (nhiemVuRa's note).
	if strings.Contains(w.Body.String(), "overdue") {
		t.Errorf("hàng mang cờ quá hạn — hai biểu diễn của một sự thật: %s", w.Body.String())
	}
}

func TestNhiemVuCuaKetLuan404VaSttHong(t *testing.T) {
	m := dungMayChu(t)
	var thans []string
	// Unknown conclusion of a real meeting, another commune's meeting, unknown meeting.
	for _, duong := range []string{
		"/api/v1/meetings/bb-001/conclusions/9/tasks",
		"/api/v1/meetings/bb-b-001/conclusions/1/tasks",
		"/api/v1/meetings/bb-khong-co/conclusions/1/tasks",
	} {
		w := m.goi(t, http.MethodGet, hostA, duong, canBoCuaXa(xaA))
		doiMa(t, w, http.StatusNotFound)
		thans = append(thans, w.Body.String())
	}
	if thans[0] != thans[1] || thans[1] != thans[2] {
		t.Errorf("404 khác thân giữa các nguyên nhân:\n%s", strings.Join(thans, "\n"))
	}
	if strings.Contains(thans[1], "NV01") {
		t.Error("NHIỆM VỤ CỦA XÃ B LỌT SANG XÃ A")
	}

	for _, stt := range []string{"0", "-1", "mot"} {
		m := dungMayChu(t)
		w := m.goi(t, http.MethodGet, hostA, "/api/v1/meetings/bb-001/conclusions/"+stt+"/tasks",
			canBoCuaXa(xaA))
		doiMa(t, w, http.StatusBadRequest)
		if m.bienBan.goi != 0 {
			t.Errorf("stt=%q: đã chạy truy vấn với số thứ tự hỏng", stt)
		}
	}
}

func TestNhiemVuCuaKetLuanVuotTranThi500(t *testing.T) {
	m := dungMayChu(t)
	m.bienBan.loi = petstore.ErrQuaNhieuNhiemVuKetLuan
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVuKL11, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
}

// --- the task back-link --------------------------------------------------------------------------------

func TestNhiemVuMangLienKetNguocVeBienBan(t *testing.T) {
	m := dungMayChu(t)

	// A `ket-luan-hop` task: the three fields are present.
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	n := docNhiemVu(t, w.Body.Bytes())
	if n.MeetingID != "bb-001" || n.MeetingTitle != "Giao ban UBND xã tháng 8" || n.ConclusionNo != 2 {
		t.Errorf("liên kết ngược sai: id=%q title=%q no=%d", n.MeetingID, n.MeetingTitle, n.ConclusionNo)
	}

	// A `truc-tiep` task: none of them is on the wire at all.
	w = m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuXo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	for _, vang := range []string{"meeting_id", "meeting_title", "conclusion_no"} {
		if strings.Contains(w.Body.String(), vang) {
			t.Errorf("nhiệm vụ giao trực tiếp mang %q: %s", vang, w.Body.String())
		}
	}

	// And the register list carries it per row.
	w = m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	trang := docTrangNhiemVu(t, w.Body.Bytes())
	for _, r := range trang.Items {
		coLienKet := r.MeetingID != ""
		if coLienKet != (r.Source == "ket-luan-hop") {
			t.Errorf("hàng %s: nguồn %q nhưng meeting_id = %q", r.Code, r.Source, r.MeetingID)
		}
	}
}
