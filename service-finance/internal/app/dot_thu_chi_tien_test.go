package app

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE ADDS to dot_thu_chi_test.go: the edges of the MONEY, where a defect leaves every
// figure plausible and nobody notices until an inspection compares the report with the receipts.
//
//  1. entries -> manual copies each column EXACTLY: a sum of 0 becomes a stored 0 (not an empty
//     cell), a negative sum stays negative, and a percentage column never receives an amount;
//  2. a batch sum that does not fit int64, or exceeds the typo guard, REFUSES the switch and writes
//     nothing — it is never skipped into an empty cell or truncated into a wrong one;
//  3. changing a sheet's display unit writes the header and the entry, and NOTHING ELSE: the stored
//     đồng are never rescaled;
//  4. the `⇄` list, read through the REAL store, reaches only live batches of a live line of a live
//     sheet — the three soft-delete predicates of that read are in the statements.

// --- (1) the entries -> manual hand-over, column by column ----------------------------------------------

func TestDoiTheoDotSangNhapTayChepDungTungCot0VanLa0SoAmGiuDau(t *testing.T) {
	k := khoMau()
	k.khoanMuc[1].CachTinh = domain.TinhTheoDot
	// No typed figure at all on the line, so EVERY write below is the hand-over's own.
	k.gia[idDongKia] = map[string]domain.Dong{}
	k.giaDot = map[string]map[string]domain.Dong{idDongKia: {
		// Two instalments that cancel out: the batches STATED an amount, and it adds up to 0. The screen
		// showed `0`, and the manual starting value must be 0 — not `—` (§9 rule 4).
		"c-chi": 0,
		// A net refund.
		"c-dt": -123_456_789,
		// A sum row for the PERCENTAGE column. 0008 says the database cannot stop such a row; §9 rule 3
		// says a percentage is never stored. The hand-over must not write it.
		"c-ty": 55,
	}}
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaKhoanMuc(ctx, idDongKia, YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTay)}, nguoiGhi()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}

	ghi := k.cau("INSERT INTO gia_tri_khoan_muc")
	theoCot := map[string]any{}
	for _, g := range ghi {
		// args: tenant, khoan_muc_id, cot_id, gia_tri
		cot, _ := g.args[2].(string)
		theoCot[cot] = g.args[3]
	}
	if v, co := theoCot["c-chi"]; !co || v != int64(0) {
		t.Fatalf("c-chi (tổng đợt = 0) ghi %v (có ghi: %v), muốn int64(0) — 0 là một con số, không phải ô trống", v, co)
	}
	if v := theoCot["c-dt"]; v != int64(-123_456_789) {
		t.Fatalf("c-dt (tổng đợt âm) ghi %v, muốn -123456789 đúng từng đồng", v)
	}
	if _, co := theoCot["c-ty"]; co {
		t.Fatal("cột phần trăm nhận số tiền từ tổng đợt (§9 quy tắc 3)")
	}
	if len(ghi) != 2 {
		t.Fatalf("có %d câu ghi ô, muốn đúng 2 (c-chi, c-dt)", len(ghi))
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || !coChuoiTrongDelta(vet[0], "-123456789") {
		t.Fatalf("vết không ghi số âm được chép vào ô: %v", vet)
	}
}

// --- (2) a sum that is not a budget figure refuses the switch -------------------------------------------

func TestDoiTheoDotSangNhapTayTongVuotMucThiTuChoiKhongGhiGi(t *testing.T) {
	for ten, tc := range map[string]struct {
		sua  func(*khoNSGia)
		muon error
	}{
		// 2^63: SUM(BIGINT) is NUMERIC in PostgreSQL and CAN exceed int64. Skipping the row would copy
		// NULL (`—`) over the figure; wrapping would copy a negative number. Both look healthy.
		"tổng vượt int64": {func(k *khoNSGia) {
			k.tongDotTho = map[string]map[string]string{idDongKia: {"c-chi": "9223372036854775808"}}
		}, domain.ErrTongDotVuotMuc},
		"tổng âm vượt int64": {func(k *khoNSGia) {
			k.tongDotTho = map[string]map[string]string{idDongKia: {"c-chi": "-9223372036854775809"}}
		}, domain.ErrTongDotVuotMuc},
		// Fits int64 but is past the typo guard every typed cell passes: copying it would put into a
		// manual cell what no person could have typed there.
		"tổng vượt chặn gõ nhầm": {func(k *khoNSGia) {
			k.giaDot = map[string]map[string]domain.Dong{idDongKia: {"c-chi": domain.GiaTriToiDa + 1}}
		}, domain.ErrGiaTriQuaLon},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			k.khoanMuc[1].CachTinh = domain.TinhTheoDot
			k.gia[idDongKia] = map[string]domain.Dong{"c-chi": 42}
			tc.sua(k)
			uc, ctx := dungUseCaseNganSach(t, k)

			_, err := uc.SuaKhoanMuc(ctx, idDongKia, YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTay)}, nguoiGhi())
			if !errors.Is(err, tc.muon) {
				t.Fatalf("= %v, muốn %v", err, tc.muon)
			}
			if k.coCau("INSERT INTO gia_tri_khoan_muc") || k.coCau("SET cach_tinh = $3") ||
				k.coCau("INSERT INTO audit_log") {
				t.Fatal("tổng đợt không phải một con số ngân sách mà vẫn ghi")
			}
			if k.daCommit != 0 {
				t.Fatalf("commit %d, muốn 0", k.daCommit)
			}
			// The refusal never quotes the figure (it is the commune's budget, and it travels into logs).
			if strings.Contains(err.Error(), "9223372036854775808") {
				t.Fatalf("câu lỗi trích con số: %q", err.Error())
			}
		})
	}
}

// --- (3) the display unit never touches the stored đồng -------------------------------------------------

func TestSuaBangDoiDonViKhongDungToiSoDaLuu(t *testing.T) {
	// ĐƠN VỊ LÀ CÁCH HIỂN THỊ. Changing "Triệu đồng" to "Đồng" must write the header and its entry,
	// and nothing else: a rescale of the cells would multiply every figure of the report by 10^6 in
	// one click, with an entry that reads as a label change.
	for _, ma := range []string{"dong", "nghin-dong"} {
		t.Run(ma, func(t *testing.T) {
			k := khoMau()
			k.giaDot = map[string]map[string]domain.Dong{idDongKia: {"c-chi": 7_000}}
			uc, ctx := dungUseCaseNganSach(t, k)

			if _, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{DonViTinh: chuoi(ma)}, nguoiGhi()); err != nil {
				t.Fatalf("SuaBang lỗi: %v", err)
			}
			if !k.coCau("UPDATE bang_ngan_sach") {
				t.Fatal("đổi đơn vị mà không ghi đầu bảng — phép kiểm dưới đây sẽ xanh vô nghĩa")
			}
			k.mu.Lock()
			defer k.mu.Unlock()
			for _, l := range k.lenh {
				// By the statement's FIRST word: `SELECT … FOR UPDATE` is the lock, not a write.
				dau := strings.ToUpper(strings.TrimSpace(l.sql))
				ghi := strings.HasPrefix(dau, "INSERT") || strings.HasPrefix(dau, "UPDATE") ||
					strings.HasPrefix(dau, "DELETE") || strings.HasPrefix(dau, "WITH")
				if !ghi {
					continue
				}
				if !strings.Contains(l.sql, "UPDATE bang_ngan_sach") && !strings.Contains(l.sql, "INSERT INTO audit_log") {
					t.Errorf("đổi đơn vị mà ghi thêm câu khác: %s", l.sql)
				}
				if strings.Contains(l.sql, "gia_tri") {
					t.Errorf("đổi đơn vị mà câu ghi chạm tới giá trị: %s", l.sql)
				}
			}
		})
	}
}

// --- (4) the `⇄` list through the real store --------------------------------------------------------------

func TestDanhSachDotQuaStoreThatChiThayDotSongCuaKhoanMucSongTrongBangSong(t *testing.T) {
	// The HTTP tests read the list through a fake READER, so the store's own statements — the only
	// place the three soft-delete predicates live — ran under no test at all. A removed batch shown in
	// the list is a batch an accountant reconciles against a receipt while it no longer counts; a line
	// of a removed sheet answering 200 is a sheet that is off every screen except this one.
	k := khoMau()
	k.khoanMuc = k.khoanMuc[1:] // the line read answers with THIS line
	k.khoanMuc[0].CachTinh = domain.TinhTheoDot
	k.dot = dotSong()
	k.dot.GiaTri = map[string]domain.Dong{"c-chi": -1_500_000, "c-dt": 0}

	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, ctx := dungUseCaseNganSach(t, khoMau()) // only for a context carrying commune A
	s := fistore.NewNganSachStore(store.New(db))

	ds, err := s.DotCuaKhoanMuc(ctx, idDongKia)
	if err != nil {
		t.Fatalf("DotCuaKhoanMuc lỗi: %v", err)
	}
	if len(ds.Dot) != 1 || ds.Dot[0].GiaTri["c-chi"] != -1_500_000 {
		t.Fatalf("đợt đọc ra: %+v", ds.Dot)
	}
	// 0 is a stated amount and must survive the read as a PRESENT key.
	if g, co := ds.Dot[0].GiaTri["c-dt"]; !co || g != 0 {
		t.Fatalf("số tiền 0 của đợt: %v (có khoá: %v), muốn 0 có mặt", g, co)
	}

	dong := k.cau("JOIN bang_ngan_sach b")
	if len(dong) != 1 {
		t.Fatalf("có %d câu đọc khoản mục kèm bảng, muốn 1", len(dong))
	}
	for _, manh := range []string{"k.tenant_id = $1", "k.deleted_at IS NULL", "b.deleted_at IS NULL",
		"b.tenant_id = k.tenant_id"} {
		if !strings.Contains(dong[0].sql, manh) {
			t.Errorf("câu đọc khoản mục của hộp đợt thiếu %q: %s", manh, dong[0].sql)
		}
	}

	ds2 := k.cau("FROM dot_thu_chi WHERE")
	if len(ds2) != 1 {
		t.Fatalf("có %d câu đọc danh sách đợt, muốn 1", len(ds2))
	}
	for _, manh := range []string{"tenant_id = $1", "deleted_at IS NULL", "khoan_muc_id = $2"} {
		if !strings.Contains(ds2[0].sql, manh) {
			t.Errorf("câu danh sách đợt thiếu %q: %s", manh, ds2[0].sql)
		}
	}
	if !coGiaTri(ds2[0], int64(fistore.TranDotMotKhoanMuc+1)) {
		t.Errorf("LIMIT của danh sách đợt không phải trần cộng một: %v", ds2[0].args)
	}

	so := k.cau("d.khoan_muc_id = $2")
	if len(so) != 1 {
		t.Fatalf("có %d câu đọc số tiền đợt, muốn 1", len(so))
	}
	for _, manh := range []string{"g.tenant_id = $1", "d.tenant_id = g.tenant_id", "d.deleted_at IS NULL"} {
		if !strings.Contains(so[0].sql, manh) {
			t.Errorf("câu số tiền đợt thiếu %q: %s", manh, so[0].sql)
		}
	}
}
