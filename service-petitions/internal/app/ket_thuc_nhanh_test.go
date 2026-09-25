package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the two TERMINAL BRANCHES — POST …/rejection (`khong-tiep-nhan`) and …/referral
// (`chuyen-cap-tren`) — over the REAL store on the fake driver of driver_gia_phieu_test.go.
//
//	PROVED HERE   only `dang-phan-loai` may take a branch, every other status is refused with
//	              NOTHING written · the reason (and the body) are validated before any statement ·
//	              the UPDATE, the audit entry and the outbox row share ONE transaction, and a failure of
//	              the last takes the other two down · the status is the statement's literal · the
//	              instant is the use case's clock · the actor is the staff BUSINESS CODE · the reason
//	              and the body reach NEITHER the event NOR the audit delta · a staff-booked petition
//	              gets no event · a `can-bo` petition is refused without `feedback.restricted`.
//
//	NOT PROVED    migration 0011's CHECK and trigger — that needs PostgreSQL (VIGOV_TEST_DSN unset).

// lyDoThat and coQuanThat are texts of the shape the act asks for. The reason NAMES A THIRD PERSON on
// purpose ("ông Trần Văn Bình"): that is exactly the free text that must not leak into the trail or the
// queue, so the assertions below have something real to look for.
const (
	lyDoThat   = "Việc tranh chấp ranh giới đất với ông Trần Văn Bình thuộc thẩm quyền Toà án nhân dân huyện."
	coQuanThat = "Toà án nhân dân huyện Thăng Bình"
	// dauVetLyDo and dauVetCoQuan are substrings searched for in the event and the audit entry.
	dauVetLyDo   = "Trần Văn Bình"
	dauVetCoQuan = "Thăng Bình"
)

// nhanhThu is one of the two branch acts, runnable against the same fixture.
type nhanhThu struct {
	dich   domain.TrangThai
	hanhVi string
	chay   func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) (domain.PhieuPhanAnh, error)
}

func haiNhanh() map[string]nhanhThu {
	return map[string]nhanhThu{
		"không tiếp nhận": {domain.KhongTiepNhan, HanhViKhongTiepNhanPhanAnh,
			func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) (domain.PhieuPhanAnh, error) {
				return uc.KhongTiepNhan(ctx, maPhieuThu, lyDoThat, "", canBoThu(), q)
			}},
		"chuyển cấp trên": {domain.ChuyenCapTren, HanhViChuyenCapTrenPhanAnh,
			func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) (domain.PhieuPhanAnh, error) {
				return uc.ChuyenCapTren(ctx, maPhieuThu,
					YeuCauChuyenCapTren{LyDo: lyDoThat, CoQuanNhan: coQuanThat}, canBoThu(), q)
			}},
	}
}

// phieuDangPhanLoai is the one status both branches leave from.
func phieuDangPhanLoai() *khoPhieuXuLyGia {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai":     string(domain.DangPhanLoai),
		"linh_vuc":       "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu,
		"phan_loai_luc":  mocThaoTac,
	})
	return k
}

func TestKetThucNhanhGhiBaThuTrongMotGiaoDich(t *testing.T) {
	for ten, n := range haiNhanh() {
		t.Run(ten, func(t *testing.T) {
			k := phieuDangPhanLoai()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			sau, err := n.chay(uc, ctx, khongQuyenHanChe)
			if err != nil {
				t.Fatalf("%s: %v", ten, err)
			}
			if sau.TrangThai != n.dich || sau.LyDoKetThucNhanh != lyDoThat ||
				!sau.KetThucNhanhLuc.Equal(mocThaoTac) {
				t.Errorf("phiếu sau: trạng thái=%q lý do=%q lúc=%v", sau.TrangThai,
					sau.LyDoKetThucNhanh, sau.KetThucNhanhLuc)
			}

			for tenCau, tu := range map[string]string{
				"đổi phiếu":  "UPDATE phieu_phan_anh",
				"vết kiểm":   "INSERT INTO audit_log",
				"sự kiện đi": "INSERT INTO su_kien_di",
			} {
				cau := k.cau(tu)
				if len(cau) != 1 {
					t.Fatalf("%s: có %d câu, muốn 1", tenCau, len(cau))
				}
				if !cau[0].trongGiaoDich {
					t.Errorf("%s chạy NGOÀI giao dịch", tenCau)
				}
			}
			if k.daCommit != 1 || k.daRollback != 0 {
				t.Errorf("commit=%d rollback=%d, muốn 1/0", k.daCommit, k.daRollback)
			}

			// THE STATUS IS THE STATEMENT'S LITERAL, and the expected status travels as a parameter.
			up := k.cau("UPDATE phieu_phan_anh")[0]
			if !strings.Contains(up.sql, "trang_thai = '"+string(n.dich)+"'") {
				t.Errorf("câu UPDATE không mang trạng thái đích dạng hằng: %q", up.sql)
			}
			if !coThamSo(up.args, string(domain.DangPhanLoai)) {
				t.Errorf("câu UPDATE không mang trạng thái đang chờ: %v", up.args)
			}
			if !coThamSo(up.args, lyDoThat) {
				t.Error("lý do KHÔNG xuống câu UPDATE — kết thúc phiếu mà người dân không đọc được vì sao")
			}
			// THE SAME CLOCK AS EVERY OTHER ACT — migration 0011 requires it to be >= phan_loai_luc.
			coLuc := false
			for _, a := range up.args {
				if tm, ok := a.(time.Time); ok && tm.Equal(mocThaoTac) {
					coLuc = true
				}
			}
			if !coLuc {
				t.Errorf("câu UPDATE không mang ket_thuc_nhanh_luc = đồng hồ của use case: %v", up.args)
			}

			vet := k.cau("INSERT INTO audit_log")[0]
			if !coThamSo(vet.args, maCanBoThu) || !coThamSo(vet.args, n.hanhVi) ||
				!coThamSo(vet.args, maPhieuThu) {
				t.Errorf("vết phải mang mã cán bộ, hành vi %q và mã tra cứu: %v", n.hanhVi, vet.args)
			}
		})
	}
}

func TestChuyenCapTrenGhiCoQuanNhan(t *testing.T) {
	k := phieuDangPhanLoai()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	sau, err := uc.ChuyenCapTren(ctx, maPhieuThu,
		YeuCauChuyenCapTren{LyDo: "  " + lyDoThat + "\n", CoQuanNhan: " " + coQuanThat + " "},
		canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("ChuyenCapTren: %v", err)
	}
	// TRIMMED, and stored trimmed: the CHECK tests btrim, and a padded value would render padded.
	if sau.CoQuanNhan != coQuanThat || sau.LyDoKetThucNhanh != lyDoThat {
		t.Errorf("cơ quan=%q lý do=%q — chưa cắt khoảng trắng", sau.CoQuanNhan, sau.LyDoKetThucNhanh)
	}
	if !coThamSo(k.cau("UPDATE phieu_phan_anh")[0].args, coQuanThat) {
		t.Error("cơ quan tiếp nhận KHÔNG xuống câu UPDATE")
	}
}

// TestKhongTiepNhanKhongGhiCoQuanNhan — the refusal statement does not name `co_quan_nhan` at all, so
// it stays NULL, which migration 0011's CHECK requires of `khong-tiep-nhan`.
func TestKhongTiepNhanKhongGhiCoQuanNhan(t *testing.T) {
	k := phieuDangPhanLoai()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	sau, err := uc.KhongTiepNhan(ctx, maPhieuThu, lyDoThat, "", canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("KhongTiepNhan: %v", err)
	}
	if sau.CoQuanNhan != "" {
		t.Errorf("phiếu không tiếp nhận mang cơ quan nhận %q", sau.CoQuanNhan)
	}
	if strings.Contains(k.cau("UPDATE phieu_phan_anh")[0].sql, "co_quan_nhan") {
		t.Error("câu UPDATE của nhánh không tiếp nhận ghi vào co_quan_nhan")
	}
}

// TestKetThucNhanhLyDoKhongRaSuKienVaKhongVaoVet — the reason and the receiving body are free text
// about ONE case. events.proto forbids staff-written text on the queue, and rule 6 forbidden #4 keeps
// personal data out of the append-only trail.
func TestKetThucNhanhLyDoKhongRaSuKienVaKhongVaoVet(t *testing.T) {
	for ten, n := range haiNhanh() {
		t.Run(ten, func(t *testing.T) {
			k := phieuDangPhanLoai()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			if _, err := n.chay(uc, ctx, khongQuyenHanChe); err != nil {
				t.Fatalf("%s: %v", ten, err)
			}

			raw := string(k.cau("INSERT INTO su_kien_di")[0].args[4].([]byte))
			for _, cam := range []string{dauVetLyDo, dauVetCoQuan, "Toà án"} {
				if strings.Contains(raw, cam) {
					t.Errorf("thân sự kiện mang %q: %s", cam, raw)
				}
			}
			than := thanSuKien(t, k.cau("INSERT INTO su_kien_di")[0].args)
			if than["status"] != string(n.dich) {
				t.Errorf("sự kiện mang trạng thái %v, muốn %s", than["status"], n.dich)
			}
			// THE CITIZEN IS STILL TOLD (rule 10, invariant 5) — with the fixed sentence pointing at
			// the lookup code, which is where the reason is read.
			if _, co := than["citizen_message"]; !co {
				t.Error("nhánh kết thúc mà KHÔNG báo cho dân")
			}

			for _, a := range k.cau("INSERT INTO audit_log")[0].args {
				var s string
				switch v := a.(type) {
				case []byte:
					s = string(v)
				case string:
					s = v
				}
				for _, cam := range []string{dauVetLyDo, dauVetCoQuan} {
					if strings.Contains(s, cam) {
						t.Errorf("vết kiểm toán mang %q: %s", cam, s)
					}
				}
			}
		})
	}
}

// TestKetThucNhanhSaiBuocThiTuChoiKhongGhiGi — `dang-phan-loai` is the only status the branches leave
// from (ADR 0027 map). In particular `dang-xu-ly` -> `chuyen-cap-tren` is undecided and must be refused.
func TestKetThucNhanhSaiBuocThiTuChoiKhongGhiGi(t *testing.T) {
	for ten, n := range haiNhanh() {
		for _, tt := range []domain.TrangThai{
			domain.DaTiepNhan, domain.DaChuyenXuLy, domain.DangXuLy, domain.DaXuLy,
			domain.ChoDanXacNhan, domain.DaDong, domain.KhongTiepNhan, domain.ChuyenCapTren,
		} {
			t.Run(ten+"/"+string(tt), func(t *testing.T) {
				k := khoPhieuMau()
				k.hang = dongPhieuMau(map[string]any{"trang_thai": string(tt)})
				uc, ctx := dungXuLy(t, k, hanXuLyThu())

				if _, err := n.chay(uc, ctx, khongQuyenHanChe); !errors.Is(err, domain.ErrKetThucNhanhSaiLuc) {
					t.Fatalf("lỗi = %v, muốn ErrKetThucNhanhSaiLuc", err)
				}
				if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") ||
					k.coCau("INSERT INTO su_kien_di") {
					t.Error("đã ghi dù bước không cho kết thúc nhánh")
				}
				if k.daCommit != 0 {
					t.Errorf("commit=%d, muốn 0", k.daCommit)
				}
			})
		}
	}
}

// TestKetThucNhanhDauVaoSaiThiTuChoiTruocMoiThu — refused before the transaction opens: no statement.
func TestKetThucNhanhDauVaoSaiThiTuChoiTruocMoiThu(t *testing.T) {
	quaDai := strings.Repeat("ồ", domain.LyDoToiDa+1)
	cases := map[string]struct {
		chay func(uc *XuLyPhanAnh, ctx context.Context) error
		muon error
	}{
		"từ chối/lý do rỗng": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.KhongTiepNhan(ctx, maPhieuThu, "", "", canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrThieuLyDo},
		"từ chối/toàn khoảng trắng": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.KhongTiepNhan(ctx, maPhieuThu, " \n\t ", "", canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrThieuLyDo},
		"từ chối/quá ngắn": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.KhongTiepNhan(ctx, maPhieuThu, "sai xã", "", canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrLyDoQuaNgan},
		"từ chối/quá dài": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.KhongTiepNhan(ctx, maPhieuThu, quaDai, "", canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrLyDoQuaDai},
		"chuyển/thiếu cơ quan": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.ChuyenCapTren(ctx, maPhieuThu, YeuCauChuyenCapTren{LyDo: lyDoThat},
				canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrThieuCoQuanNhan},
		"chuyển/cơ quan quá dài": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.ChuyenCapTren(ctx, maPhieuThu, YeuCauChuyenCapTren{LyDo: lyDoThat,
				CoQuanNhan: strings.Repeat("ệ", domain.CoQuanNhanToiDa+1)}, canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrCoQuanNhanQuaDai},
		"chuyển/thiếu lý do": {func(uc *XuLyPhanAnh, ctx context.Context) error {
			_, err := uc.ChuyenCapTren(ctx, maPhieuThu, YeuCauChuyenCapTren{CoQuanNhan: coQuanThat},
				canBoThu(), khongQuyenHanChe)
			return err
		}, domain.ErrThieuLyDo},
	}
	for ten, ca := range cases {
		t.Run(ten, func(t *testing.T) {
			k := phieuDangPhanLoai()
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			if err := ca.chay(uc, ctx); !errors.Is(err, ca.muon) {
				t.Fatalf("lỗi = %v, muốn %v", err, ca.muon)
			}
			if len(k.lenh) != 0 || k.batDau != 0 {
				t.Errorf("đã chạm dữ liệu dù sẽ từ chối (%d câu, %d giao dịch)", len(k.lenh), k.batDau)
			}
		})
	}
}

// TestKetThucNhanhSuKienHongThiKhongCoGiDuoc — the outbox row is the LAST write; its failure must take
// the status change and the audit entry down with it.
func TestKetThucNhanhSuKienHongThiKhongCoGiDuoc(t *testing.T) {
	for ten, n := range haiNhanh() {
		t.Run(ten, func(t *testing.T) {
			k := phieuDangPhanLoai()
			k.loiSau = "INSERT INTO su_kien_di"
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if _, err := n.chay(uc, ctx, khongQuyenHanChe); err == nil {
				t.Fatal("thành công dù dòng sự kiện hỏng")
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("commit=%d rollback=%d, muốn 0/1", k.daCommit, k.daRollback)
			}
		})
	}
}

// TestKetThucNhanhPhieuNhapHoKhongGhiSuKien — no citizen account, nobody to tell, no outbox row.
func TestKetThucNhanhPhieuNhapHoKhongGhiSuKien(t *testing.T) {
	for ten, n := range haiNhanh() {
		t.Run(ten, func(t *testing.T) {
			k := phieuDangPhanLoai()
			k.hang["cong_dan_id"] = nil
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if _, err := n.chay(uc, ctx, khongQuyenHanChe); err != nil {
				t.Fatalf("%s: %v", ten, err)
			}
			if k.coCau("INSERT INTO su_kien_di") {
				t.Error("ghi dòng sự kiện cho phiếu không có công dân")
			}
			if len(k.cau("INSERT INTO audit_log")) != 1 || k.daCommit != 1 {
				t.Errorf("vết=%d commit=%d, muốn 1/1", len(k.cau("INSERT INTO audit_log")), k.daCommit)
			}
		})
	}
}

// TestKetThucNhanhLinhVucHanChe — the four-cell table of TestBonHanhViVaLinhVucHanChe, on the two
// branch acts. The third cell is the leak: a colleague refusing or passing on a report about a member
// of staff. It must write NOTHING.
func TestKetThucNhanhLinhVucHanChe(t *testing.T) {
	for tenNhanh, n := range haiNhanh() {
		for tenO, ca := range map[string]struct {
			linhVuc string
			quyen   QuyenXemHanChe
			lamDuoc bool
		}{
			"thường + không quyền":  {"rac-thai", khongQuyenHanChe, true},
			"thường + có quyền":     {"rac-thai", coQuyenHanChe, true},
			"HẠN CHẾ + KHÔNG quyền": {domain.LinhVucHanChe, khongQuyenHanChe, false},
			"hạn chế + có quyền":    {domain.LinhVucHanChe, coQuyenHanChe, true},
		} {
			t.Run(tenNhanh+"/"+tenO, func(t *testing.T) {
				k := khoPhieuMau()
				k.hang = phieuLinhVuc(domain.DangPhanLoai, ca.linhVuc)
				uc, ctx := dungXuLy(t, k, hanXuLyThu())

				_, err := n.chay(uc, ctx, ca.quyen)
				if ca.lamDuoc {
					if err != nil {
						t.Fatalf("%v", err)
					}
					return
				}
				if !errors.Is(err, ErrPhieuHanChe) {
					t.Fatalf("lỗi = %v, muốn ErrPhieuHanChe", err)
				}
				if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") ||
					k.coCau("INSERT INTO su_kien_di") || k.daCommit != 0 {
					t.Error("đã ghi dù từ chối")
				}
			})
		}
	}
}

// TestKetThucNhanhThieuMaCanBoThiTuChoi — rule 6, invariant 8: no business code, no write.
func TestKetThucNhanhThieuMaCanBoThiTuChoi(t *testing.T) {
	k := phieuDangPhanLoai()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	nguoi := canBoThu()
	nguoi.ID = ""

	if _, err := uc.KhongTiepNhan(ctx, maPhieuThu, lyDoThat, "", nguoi, khongQuyenHanChe); err == nil {
		t.Fatal("ghi được dù vết không gọi tên được người làm")
	}
	if len(k.lenh) != 0 {
		t.Errorf("đã chạm dữ liệu (%d câu)", len(k.lenh))
	}
}
