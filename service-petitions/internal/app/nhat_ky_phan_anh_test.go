package app

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the petition processing logbook `nhat_ky_phan_anh` (migration 0013), over the REAL store
// and the fake driver.
//
//	PROVED HERE   each of the six acts writes EXACTLY ONE row, inside the act's transaction, with the
//	              right act, the status AFTER the act, the assignment pair only on `phan-cong`, the
//	              acting officer's BUSINESS CODE and the use case's clock · the optional note reaches
//	              the row and NOTHING ELSE (not the audit delta — its length does —, not the event) ·
//	              a blank note is absent · the result / reason / receiving body are not copied into the
//	              row · a failing row insert rolls the whole act back · the manual note's matrix
//	              (assignee · commune-wide fact · neither · restricted field · final status).
//
//	NOT PROVED    migration 0013's CHECKs and the append-only trigger — PostgreSQL only
//	              (internal/store/nhat_ky_phan_anh_pg_test.go, skipped without VIGOV_TEST_DSN).

// ghiChuThu carries a marker a test can search every statement for. It names a third person on
// purpose: this is exactly the free text that must not leak into the trail or the queue (rule 3).
const (
	ghiChuThu    = "  Đã gọi ông Trần Văn Hải — dauvetghichunoibo.  "
	dauVetGhiChu = "dauvetghichunoibo"
)

// Positions of the logbook INSERT's bound values — the store's column order.
const (
	nkPhieu, nkLuc, nkNguoi, nkHanhVi, nkTrangThai, nkBoPhan, nkCanBo, nkNoiDung = 2, 3, 4, 5, 6, 7, 8, 9
)

type hanhViNhatKyThu struct {
	hang      func() map[string]driver.Value
	chay      func(uc *XuLyPhanAnh, ctx context.Context, ghiChu string) error
	hanhVi    domain.HanhViNhatKy
	trangThai domain.TrangThai
	boPhan    any // nil = must be NULL
	canBo     any
}

// sauHanhVi is the six acts plus the two variants the card names: a re-assignment that moves no
// status, and a closing with no citizen behind the petition.
func sauHanhVi() map[string]hanhViNhatKyThu {
	dangXuLyDaGiao := func() map[string]driver.Value {
		h := phieuDaGiaoCho(maCanBoThu)
		h["trang_thai"] = string(domain.DangXuLy)
		return h
	}
	return map[string]hanhViNhatKyThu{
		"phân loại": {func() map[string]driver.Value { return dongPhieuMau(nil) },
			func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
				_, err := uc.ChotLinhVuc(ctx, maPhieuThu,
					YeuCauChotLinhVuc{LinhVuc: "rac-thai", GhiChu: g}, canBoThu(), khongQuyenHanChe)
				return err
			}, domain.NhatKyPhanLoai, domain.DangPhanLoai, nil, nil},
		"phân công": {func() map[string]driver.Value { return phieuDangPhanLoai().hang },
			func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
				_, err := uc.PhanCong(ctx, maPhieuThu,
					YeuCauPhanCong{BoPhan: "bp-002", CanBo: "CB-00777", GhiChu: g}, canBoThu(),
					khongQuyenHanChe)
				return err
			}, domain.NhatKyPhanCong, domain.DaChuyenXuLy, "bp-002", "CB-00777"},
		"phân công lại, không đổi trạng thái, không nêu cán bộ": {dangXuLyDaGiao,
			func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
				_, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-003", GhiChu: g},
					canBoThu(), khongQuyenHanChe)
				return err
			}, domain.NhatKyPhanCong, domain.DangXuLy, "bp-003", nil},
		"chuyển trạng thái (người được giao)": {func() map[string]driver.Value {
			return phieuDaGiaoCho(maCanBoThu)
		}, func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
			_, err := uc.TienTrangThai(ctx, maPhieuThu, g, canBoThu(), khongQuyenCaXa, khongQuyenHanChe)
			return err
		}, domain.NhatKyChuyenTrangThai, domain.DangXuLy, nil, nil},
		"đóng phiếu": {func() map[string]driver.Value {
			return phieuLinhVuc(domain.ChoDanXacNhan, "rac-thai")
		}, func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
			_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, g, canBoThu(), khongQuyenHanChe)
			return err
		}, domain.NhatKyDongPhieu, domain.DaDong, nil, nil},
		"đóng phiếu không có người dân xác nhận": {func() map[string]driver.Value {
			h := phieuLinhVuc(domain.DaXuLy, "rac-thai")
			h["cong_dan_id"] = nil
			return h
		}, func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
			_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, g, canBoThu(), khongQuyenHanChe)
			return err
		}, domain.NhatKyDongPhieu, domain.DaDong, nil, nil},
		"không tiếp nhận": {func() map[string]driver.Value { return phieuDangPhanLoai().hang },
			func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
				_, err := uc.KhongTiepNhan(ctx, maPhieuThu, lyDoThat, g, canBoThu(), khongQuyenHanChe)
				return err
			}, domain.NhatKyKhongTiepNhan, domain.KhongTiepNhan, nil, nil},
		"chuyển cấp trên": {func() map[string]driver.Value { return phieuDangPhanLoai().hang },
			func(uc *XuLyPhanAnh, ctx context.Context, g string) error {
				_, err := uc.ChuyenCapTren(ctx, maPhieuThu,
					YeuCauChuyenCapTren{LyDo: lyDoThat, CoQuanNhan: coQuanThat, GhiChu: g}, canBoThu(),
					khongQuyenHanChe)
				return err
			}, domain.NhatKyChuyenCapTren, domain.ChuyenCapTren, nil, nil},
	}
}

// motDongNhatKy asserts there is exactly ONE logbook row, written inside the transaction, and
// returns it.
func motDongNhatKy(t *testing.T, k *khoPhieuXuLyGia) lenhPhieu {
	t.Helper()
	nk := k.cau("INSERT INTO nhat_ky_phan_anh")
	if len(nk) != 1 {
		t.Fatalf("có %d dòng nhật ký, muốn đúng 1", len(nk))
	}
	if !nk[0].trongGiaoDich {
		t.Fatal("dòng nhật ký ghi NGOÀI giao dịch — hành vi có thể thành công mà dòng nhật ký thì mất")
	}
	return nk[0]
}

// chuaDauVet reports whether any bound value of the statement carries the note's marker.
func chuaDauVet(l lenhPhieu) bool {
	for _, a := range l.args {
		switch v := a.(type) {
		case string:
			if strings.Contains(v, dauVetGhiChu) {
				return true
			}
		case []byte:
			if strings.Contains(string(v), dauVetGhiChu) {
				return true
			}
		}
	}
	return false
}

func TestMoiHanhViGhiDungMotDongNhatKy(t *testing.T) {
	for ten, hv := range sauHanhVi() {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = hv.hang()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if err := hv.chay(uc, ctx, ""); err != nil {
				t.Fatalf("hành vi: %v", err)
			}
			nk := motDongNhatKy(t, k)
			muon := map[int]any{
				nkPhieu:     idPhieuThu,
				nkLuc:       mocThaoTac,
				nkNguoi:     maCanBoThu,
				nkHanhVi:    string(hv.hanhVi),
				nkTrangThai: string(hv.trangThai),
				nkBoPhan:    hv.boPhan,
				nkCanBo:     hv.canBo,
				nkNoiDung:   nil,
			}
			for i, m := range muon {
				if nk.args[i] != m {
					t.Errorf("tham số %d = %#v, muốn %#v", i, nk.args[i], m)
				}
			}
			if k.daCommit != 1 {
				t.Errorf("commit %d lần, muốn 1", k.daCommit)
			}
			// THE RESULT, THE REASON AND THE RECEIVING BODY ARE NOT COPIED INTO THE ROW. They live on the
			// petition; a copy in an append-only table is a second permanent store of the same text.
			for _, cam := range []string{ketQuaThat, lyDoThat, coQuanThat} {
				if coThamSo(nk.args, cam) {
					t.Errorf("dòng nhật ký chép lại văn bản nghiệp vụ %q", cam)
				}
			}
			// No note, no length in the delta.
			if _, co := deltaDong(t, k)["do_dai_ghi_chu"]; co {
				t.Error("delta mang do_dai_ghi_chu dù không có ghi chú")
			}
		})
	}
}

// The optional note reaches the ROW, trimmed — and NOTHING ELSE. The audit delta carries its length.
func TestGhiChuTuyChonChiVaoDongNhatKy(t *testing.T) {
	for ten, hv := range sauHanhVi() {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = hv.hang()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if err := hv.chay(uc, ctx, ghiChuThu); err != nil {
				t.Fatalf("hành vi: %v", err)
			}
			nk := motDongNhatKy(t, k)
			daCat := strings.TrimSpace(ghiChuThu)
			if nk.args[nkNoiDung] != daCat {
				t.Errorf("noi_dung = %#v, muốn %q (đã cắt khoảng trắng)", nk.args[nkNoiDung], daCat)
			}
			for _, l := range k.lenh {
				if strings.Contains(l.sql, "INSERT INTO nhat_ky_phan_anh") {
					continue
				}
				if chuaDauVet(l) {
					t.Errorf("ghi chú nội bộ lọt vào câu lệnh khác: %s", l.sql)
				}
			}
			d := deltaDong(t, k)
			if n, _ := d["do_dai_ghi_chu"].(float64); int(n) != utf8.RuneCountInString(daCat) {
				t.Errorf("do_dai_ghi_chu = %v, muốn %d", d["do_dai_ghi_chu"], utf8.RuneCountInString(daCat))
			}
		})
	}
}

// Blank after trim is absent: the row carries NULL, which migration 0013 requires of a non-note row.
func TestGhiChuRongLaVang(t *testing.T) {
	hv := sauHanhVi()["không tiếp nhận"]
	k := khoPhieuMau()
	k.hang = hv.hang()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if err := hv.chay(uc, ctx, " \n\t "); err != nil {
		t.Fatalf("hành vi: %v", err)
	}
	if v := motDongNhatKy(t, k).args[nkNoiDung]; v != nil {
		t.Errorf("noi_dung = %#v, muốn NULL", v)
	}
}

// An oversized note is refused BEFORE the transaction: nothing written, nothing locked.
func TestGhiChuQuaDaiThiTuChoiTruocMoiThu(t *testing.T) {
	quaDai := strings.Repeat("ồ", domain.GhiChuToiDa+1)
	for ten, hv := range sauHanhVi() {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = hv.hang()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			if err := hv.chay(uc, ctx, quaDai); !errors.Is(err, domain.ErrGhiChuQuaDai) {
				t.Fatalf("lỗi = %v, muốn ErrGhiChuQuaDai", err)
			}
			if len(k.lenh) != 0 {
				t.Errorf("đã chạy %d câu lệnh dù ghi chú quá dài", len(k.lenh))
			}
		})
	}
}

// A failing logbook insert takes the business write, the audit entry and the outbox row with it.
func TestDongNhatKyHongThiCaHanhViQuayLui(t *testing.T) {
	for ten, hv := range sauHanhVi() {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = hv.hang()
			k.loiSau = "INSERT INTO nhat_ky_phan_anh"
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if err := hv.chay(uc, ctx, ghiChuThu); err == nil {
				t.Fatal("hành vi thành công dù dòng nhật ký không ghi được")
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("commit %d, rollback %d — muốn 0 và 1", k.daCommit, k.daRollback)
			}
			if k.coCau("INSERT INTO su_kien_di") {
				t.Error("vẫn ghi nghĩa vụ báo người dân sau khi dòng nhật ký hỏng")
			}
		})
	}
}

// THE OTHER DIRECTION: the logbook row is written and a statement of the SAME act fails — the
// petition's UPDATE, its audit entry, or the outbox row that follows the logbook row. The row must go
// down with it: a timeline entry for an act that never committed tells an officer, and later an
// inspection, that something happened to the petition when nothing did.
//
// Each statement the act actually ran on a clean pass is failed in turn, so an act that gains or loses
// a statement is covered without editing this table. At least one case must fail AFTER the logbook
// INSERT ran, or the "row written, act failed" direction was never exercised and this test would be
// green for the wrong reason.
func TestHanhViHongSauDongNhatKyThiDongNhatKyQuayLui(t *testing.T) {
	loaiCau := []string{"UPDATE phieu_phan_anh", "INSERT INTO audit_log", "INSERT INTO su_kien_di"}
	saiSauDongNhatKy := 0
	for ten, hv := range sauHanhVi() {
		sach := khoPhieuMau()
		sach.hang = hv.hang()
		ucSach, ctxSach := dungXuLy(t, sach, hanXuLyThu())
		if err := hv.chay(ucSach, ctxSach, ghiChuThu); err != nil {
			t.Fatalf("%s — lượt sạch: %v", ten, err)
		}
		for _, cau := range loaiCau {
			if !sach.coCau(cau) {
				continue
			}
			t.Run(ten+" / hỏng "+cau, func(t *testing.T) {
				k := khoPhieuMau()
				k.hang = hv.hang()
				k.loiSau = cau
				uc, ctx := dungXuLy(t, k, hanXuLyThu())

				if err := hv.chay(uc, ctx, ghiChuThu); err == nil {
					t.Fatalf("hành vi thành công dù %q hỏng", cau)
				}
				if k.daCommit != 0 || k.daRollback != 1 {
					t.Errorf("commit %d, rollback %d — muốn 0 và 1: dòng nhật ký đã ghi có thể ở lại "+
						"cho một hành vi không xảy ra", k.daCommit, k.daRollback)
				}
				for _, nk := range k.cau("INSERT INTO nhat_ky_phan_anh") {
					if !nk.trongGiaoDich {
						t.Error("dòng nhật ký ghi NGOÀI giao dịch — rollback không kéo được nó theo")
					}
					saiSauDongNhatKy++
				}
			})
		}
	}
	if saiSauDongNhatKy == 0 {
		t.Fatal("không ca nào hỏng SAU khi dòng nhật ký đã ghi — chiều 'dòng ghi được, hành vi hỏng' chưa được thử")
	}
}

// --- the manual note ---------------------------------------------------------------------------------

const (
	coQuyenGhiChu    QuyenGhiChuCaXa = true
	khongQuyenGhiChu QuyenGhiChuCaXa = false
)

func deltaGhiChu(t *testing.T, k *khoPhieuXuLyGia) map[string]any {
	t.Helper()
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	if !coThamSo(vet[0].args, HanhViGhiChuPhanAnh) {
		t.Errorf("vết không mang hành vi %q", HanhViGhiChuPhanAnh)
	}
	for _, a := range vet[0].args {
		if b, ok := a.([]byte); ok {
			var d map[string]any
			if json.Unmarshal(b, &d) == nil {
				if _, co := d["do_dai_ghi_chu"]; co {
					return d
				}
			}
		}
	}
	t.Fatalf("không tìm thấy delta trong vết: %v", vet[0].args)
	return nil
}

// The allowed cells: the assignee without any commune-wide key; a holder of one who is not the
// assignee; a restricted petition with `feedback.restricted`; a CLOSED petition.
func TestGhiChuNoiBoDuocPhep(t *testing.T) {
	for ten, ca := range map[string]struct {
		hang      map[string]driver.Value
		quyen     QuyenGhiChuCaXa
		hanChe    QuyenXemHanChe
		trangThai domain.TrangThai
	}{
		"người được giao, không có quyền cả xã": {phieuDaGiaoCho(maCanBoThu), khongQuyenGhiChu,
			khongQuyenHanChe, domain.DaChuyenXuLy},
		"có quyền cả xã, không phải người được giao": {phieuDaGiaoCho("CB-00999"), coQuyenGhiChu,
			khongQuyenHanChe, domain.DaChuyenXuLy},
		"lĩnh vực hạn chế, có feedback.restricted": {phieuLinhVuc(domain.DangXuLy, domain.LinhVucHanChe),
			khongQuyenGhiChu, coQuyenHanChe, domain.DangXuLy},
		"phiếu đã đóng": {phieuLinhVuc(domain.DaDong, "rac-thai"), khongQuyenGhiChu,
			khongQuyenHanChe, domain.DaDong},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = ca.hang
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			dong, err := uc.GhiChuNoiBo(ctx, maPhieuThu, ghiChuThu, canBoThu(), ca.quyen, ca.hanChe)
			if err != nil {
				t.Fatalf("GhiChuNoiBo: %v", err)
			}
			if !k.coCau("FOR UPDATE") {
				t.Error("phiếu không được đọc FOR UPDATE — trạng thái ghi vào dòng có thể đã cũ")
			}
			nk := motDongNhatKy(t, k)
			daCat := strings.TrimSpace(ghiChuThu)
			for i, m := range map[int]any{
				nkPhieu: idPhieuThu, nkLuc: mocThaoTac, nkNguoi: maCanBoThu,
				nkHanhVi: string(domain.NhatKyGhiChu), nkTrangThai: string(ca.trangThai),
				nkBoPhan: nil, nkCanBo: nil, nkNoiDung: daCat,
			} {
				if nk.args[i] != m {
					t.Errorf("tham số %d = %#v, muốn %#v", i, nk.args[i], m)
				}
			}
			if dong.HanhVi != domain.NhatKyGhiChu || dong.TrangThai != ca.trangThai ||
				dong.NoiDung != daCat || dong.NguoiMa != maCanBoThu {
				t.Errorf("dòng trả về = %+v", dong)
			}
			d := deltaGhiChu(t, k)
			if n, _ := d["do_dai_ghi_chu"].(float64); int(n) != utf8.RuneCountInString(daCat) {
				t.Errorf("do_dai_ghi_chu = %v", d["do_dai_ghi_chu"])
			}
			for _, vet := range k.cau("INSERT INTO audit_log") {
				if chuaDauVet(vet) {
					t.Error("văn bản ghi chú lọt vào vết kiểm toán — chỉ được mang độ dài")
				}
			}
			// NO EVENT, NO UPDATE: a note moves nothing the citizen can see.
			if k.coCau("INSERT INTO su_kien_di") || k.coCau("UPDATE phieu_phan_anh") {
				t.Error("ghi chú nội bộ sinh sự kiện hoặc sửa phiếu")
			}
			if k.daCommit != 1 {
				t.Errorf("commit %d lần, muốn 1", k.daCommit)
			}
		})
	}
}

// The refused cells. Each writes NOTHING — no row, no audit entry — and commits nothing.
func TestGhiChuNoiBoBiTuChoi(t *testing.T) {
	for ten, ca := range map[string]struct {
		hang   map[string]driver.Value
		quyen  QuyenGhiChuCaXa
		hanChe QuyenXemHanChe
		muon   error
	}{
		"không phải người được giao, chỉ có feedback.read": {phieuDaGiaoCho("CB-00999"),
			khongQuyenGhiChu, khongQuyenHanChe, ErrKhongPhaiNguoiDuocGiao},
		// "" == "" must not make every reader the assignee of every unassigned petition.
		"phiếu chưa giao cho ai, chỉ có feedback.read": {phieuDaGiaoCho(""),
			khongQuyenGhiChu, khongQuyenHanChe, ErrKhongPhaiNguoiDuocGiao},
		// The restricted field wins over the commune-wide fact AND over being the assignee: 404, not 403.
		"lĩnh vực hạn chế, không có feedback.restricted": {phieuLinhVuc(domain.DangXuLy,
			domain.LinhVucHanChe), coQuyenGhiChu, khongQuyenHanChe, ErrPhieuHanChe},
		"phiếu không tồn tại / xã khác / đã xoá mềm": {nil, coQuyenGhiChu, coQuyenHanChe,
			petstore.ErrPhieuKhongTonTai},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = ca.hang
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			_, err := uc.GhiChuNoiBo(ctx, maPhieuThu, ghiChuThu, canBoThu(), ca.quyen, ca.hanChe)
			if !errors.Is(err, ca.muon) {
				t.Fatalf("lỗi = %v, muốn %v", err, ca.muon)
			}
			if k.coCau("INSERT INTO nhat_ky_phan_anh") || k.coCau("INSERT INTO audit_log") {
				t.Error("đã ghi dù bị từ chối")
			}
			if k.daCommit != 0 {
				t.Errorf("commit %d lần dù bị từ chối", k.daCommit)
			}
			if strings.Contains(err.Error(), dauVetGhiChu) || strings.Contains(err.Error(), maPhieuThu) {
				t.Errorf("lỗi lộ ghi chú hoặc mã tra cứu: %v", err)
			}
		})
	}
}

// Refused before the transaction: a blank or oversized note, and an actor with no business code.
func TestGhiChuNoiBoDauVaoSai(t *testing.T) {
	khongMa := canBoThu()
	khongMa.ID = ""
	for ten, ca := range map[string]struct {
		ghiChu string
		nguoi  audit.Actor
		muon   error // nil = any refusal (the actor case has no sentinel of its own)
	}{
		"rỗng":    {" \n ", canBoThu(), domain.ErrThieuGhiChu},
		"quá dài": {strings.Repeat("ồ", domain.GhiChuToiDa+1), canBoThu(), domain.ErrGhiChuQuaDai},
		// Rule 6, invariant 8: no business code, no write — never a fallback to the internal id.
		"không mã cán bộ": {"Ghi chú hợp lệ.", khongMa, nil},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = phieuDaGiaoCho(maCanBoThu)
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			_, err := uc.GhiChuNoiBo(ctx, maPhieuThu, ca.ghiChu, ca.nguoi, coQuyenGhiChu, coQuyenHanChe)
			if err == nil || (ca.muon != nil && !errors.Is(err, ca.muon)) {
				t.Fatalf("lỗi = %v, muốn %v", err, ca.muon)
			}
			if len(k.lenh) != 0 {
				t.Errorf("đã chạy %d câu lệnh dù đầu vào sai", len(k.lenh))
			}
		})
	}
}

// A failing row insert leaves no audit entry committed.
func TestGhiChuNoiBoDongHongThiQuayLui(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuDaGiaoCho(maCanBoThu)
	k.loiSau = "INSERT INTO nhat_ky_phan_anh"
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	if _, err := uc.GhiChuNoiBo(ctx, maPhieuThu, ghiChuThu, canBoThu(), khongQuyenGhiChu,
		khongQuyenHanChe); err == nil {
		t.Fatal("thành công dù dòng nhật ký không ghi được")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — muốn 0 và 1", k.daCommit, k.daRollback)
	}
}
