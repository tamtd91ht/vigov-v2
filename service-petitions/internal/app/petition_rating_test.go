package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
	"github.com/vihat/vigov/service-petitions/migrations"
)

// The citizen's rating (ADR 0050 point 2), over the REAL store over the transaction-capable fake
// driver — so the properties asserted are properties of the SQL and of the transaction boundary:
//
//	PROVED   the locking read filters by commune ($1, from the context) AND by the session citizen ·
//	         a status other than da-xu-ly / cho-dan-xac-nhan writes nothing · 3–5 stars keeps the status
//	         and publishes nothing · 1–2 stars reopens in ONE statement with so_lan_mo_lai + 1, the
//	         resolved/closed instants cleared and NO deadline column touched · the audit entry and the
//	         outbox row share the transaction with the change · a failure of the last write takes the
//	         others down · the comment is in neither the delta nor the event · a re-rating replaces.
//
//	NOT PROVED   PostgreSQL's CHECKs (0004 stars, 0017 comment) and the partition routing — needs
//	             VIGOV_TEST_DSN.

var ratedAtFixture = time.Date(2026, 9, 28, 2, 17, 9, 0, time.UTC)

const fixtureComment = "Rác ở cuối ngõ vẫn chưa được dọn."

// ratedPetition is the fixture petition, worked on and finished — the state a rating is allowed at.
// The deadlines are the fixture's distinct instants, so a statement that moved one is visible.
func ratedPetition(status domain.TrangThai, extra map[string]any) map[string]any {
	row := map[string]any{
		"trang_thai":     string(status),
		"linh_vuc":       "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu,
		"phan_loai_luc":  mocThaoTac,
		"xu_ly_xong_luc": mocThaoTac.Add(time.Hour),
		"bo_phan_id":     "bp-001",
		"so_lan_mo_lai":  int64(1),
	}
	for k, v := range extra {
		row[k] = v
	}
	return row
}

func buildRating(t *testing.T, k *khoPhieuXuLyGia) (*RatePetition, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	s := pkgstore.New(db)
	uc := NewRatePetition(s, petstore.NewPhieuPhanAnhStore(s), petstore.NewSuKienDiStore(s))
	uc.newID = func() (string, error) { return idSuKienThu, nil }
	uc.now = func() time.Time { return ratedAtFixture }
	return uc, ctxXa(xaThu)
}

// citizenActor is the session citizen who OWNS the fixture petition.
func citizenActor() audit.Actor {
	return audit.Actor{ID: idCongDanThu, Kind: "citizen", IP: "10.0.0.9"}
}

// ratingDelta is the one audit entry's delta; deltaDong is shape-generic (it finds the `truoc` map).
func ratingDelta(t *testing.T, k *khoPhieuXuLyGia) map[string]any {
	t.Helper()
	return deltaDong(t, k)
}

// --- isolation --------------------------------------------------------------------------------------

// TestRateLockingReadFiltersCommuneAndCitizen — rule 1 and rule 4, invariant 3, both on the one read
// that decides, and that read is `FOR UPDATE` inside the transaction.
func TestRateLockingReadFiltersCommuneAndCitizen(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, nil))
	uc, ctx := buildRating(t, k)

	if _, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 4}, citizenActor()); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	reads := k.cau("FROM phieu_phan_anh")
	if len(reads) != 1 {
		t.Fatalf("có %d câu đọc, muốn 1", len(reads))
	}
	l := reads[0]
	for _, want := range []string{"tenant_id = $1", "ma_tra_cuu = $2", "cong_dan_id = $3",
		"deleted_at IS NULL", "FOR UPDATE"} {
		if !strings.Contains(l.sql, want) {
			t.Errorf("câu đọc thiếu %q: %q", want, l.sql)
		}
	}
	if !l.trongGiaoDich {
		t.Error("câu đọc khoá chạy NGOÀI giao dịch — khoá không giữ tới lúc ghi")
	}
	if len(l.args) != 3 || l.args[0] != string(xaThu) || l.args[1] != maPhieuThu || l.args[2] != idCongDanThu {
		t.Errorf("tham số = %v, muốn [xã từ context, mã, công dân từ phiên]", l.args)
	}
}

// TestRateAnotherCitizensPetitionIsNotFound — the read matched no row (another citizen's code, another
// commune's, soft deleted, unknown): ErrPhieuKhongTonTai, and NOTHING is written.
func TestRateAnotherCitizensPetitionIsNotFound(t *testing.T) {
	k := khoPhieuMau()
	k.hang = nil
	uc, ctx := buildRating(t, k)

	_, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 1}, citizenActor())
	if !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuKhongTonTai", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") || k.coCau("INSERT INTO su_kien_di") {
		t.Error("ghi dữ liệu cho một phiếu không phải của người gọi")
	}
	if strings.Contains(err.Error(), maPhieuThu) || strings.Contains(err.Error(), idCongDanThu) {
		t.Errorf("lỗi mang mã tra cứu hoặc định danh công dân: %v", err)
	}
}

// TestRateNotACitizenIsRefused — a staff principal reaching this use case is a wiring fault, and the
// owner decided staff do NOT record ratings on behalf of citizens.
func TestRateNotACitizenIsRefused(t *testing.T) {
	k := khoPhieuMau()
	uc, ctx := buildRating(t, k)
	if _, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 5}, canBoThu()); err == nil {
		t.Fatal("cán bộ chấm sao thay dân được — chủ dự án chốt: cán bộ KHÔNG ghi đánh giá hộ")
	}
	if k.batDau != 0 {
		t.Error("mở giao dịch trước khi kiểm chủ thể")
	}
}

// --- shape and state refusals ----------------------------------------------------------------------

func TestRateBadInputRefusedBeforeTheTransaction(t *testing.T) {
	for name, req := range map[string]RatingRequest{
		"0 sao":            {Stars: 0},
		"6 sao":            {Stars: 6},
		"nhận xét quá dài": {Stars: 4, Comment: strings.Repeat("a", domain.RatingCommentMaxLen+1)},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoPhieuMau()
			uc, ctx := buildRating(t, k)
			_, err := uc.Rate(ctx, maPhieuThu, req, citizenActor())
			if !domain.IsRatingInputError(err) {
				t.Fatalf("lỗi = %v, muốn lỗi dữ liệu vào", err)
			}
			if k.batDau != 0 {
				t.Error("mở giao dịch cho một yêu cầu sai dạng — giữ khoá dòng trong lúc từ chối")
			}
		})
	}
}

// TestRateWrongStatusWritesNothing — every status but the two is refused, INCLUDING `da-dong` and
// `dang-xu-ly` (a petition already reopened cannot be rated again until it is resolved again).
func TestRateWrongStatusWritesNothing(t *testing.T) {
	for _, st := range []domain.TrangThai{domain.DaTiepNhan, domain.DangPhanLoai, domain.DaChuyenXuLy,
		domain.DangXuLy, domain.DaDong, domain.KhongTiepNhan, domain.ChuyenCapTren} {
		t.Run(string(st), func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(ratedPetition(st, nil))
			uc, ctx := buildRating(t, k)

			_, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 1}, citizenActor())
			if !errors.Is(err, domain.ErrRatingNotOpen) {
				t.Fatalf("lỗi = %v, muốn ErrRatingNotOpen", err)
			}
			if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") ||
				k.coCau("INSERT INTO su_kien_di") {
				t.Error("từ chối mà vẫn ghi")
			}
			if k.daCommit != 0 {
				t.Errorf("commit %d lần dù từ chối", k.daCommit)
			}
		})
	}
}

// TestRateStatusMovedMeanwhileIs409 — the UPDATE carries the expected status; zero rows affected (a
// staff closing won the race) is ErrPhieuDaChuyenTrang and the transaction rolls back.
func TestRateStatusMovedMeanwhileIs409(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, nil))
	k.doiDong = 0
	uc, ctx := buildRating(t, k)

	_, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 2}, citizenActor())
	if !errors.Is(err, petstore.ErrPhieuDaChuyenTrang) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuDaChuyenTrang", err)
	}
	if k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
		t.Error("ghi vết / commit dù UPDATE không khớp dòng nào")
	}
}

// --- 3–5 stars: recorded, status unchanged, no event ------------------------------------------------

func TestRateThreeOrMoreKeepsTheStatus(t *testing.T) {
	for _, st := range []domain.TrangThai{domain.DaXuLy, domain.ChoDanXacNhan} {
		for _, stars := range []int{3, 4, 5} {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(ratedPetition(st, nil))
			uc, ctx := buildRating(t, k)

			after, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: stars, Comment: fixtureComment},
				citizenActor())
			if err != nil {
				t.Fatalf("%s %d sao: %v", st, stars, err)
			}
			if after.TrangThai != st || after.SoLanMoLai != 1 || after.Rating != stars ||
				!after.RatedAt.Equal(ratedAtFixture) || after.RatingComment != fixtureComment {
				t.Errorf("%s %d sao: sau = %s lần=%d sao=%d lúc=%v", st, stars, after.TrangThai,
					after.SoLanMoLai, after.Rating, after.RatedAt)
			}
			updates := k.cau("UPDATE phieu_phan_anh")
			if len(updates) != 1 {
				t.Fatalf("có %d câu UPDATE, muốn 1", len(updates))
			}
			// THE STATUS IS NOT IN THE SET LIST — the statement cannot move the petition.
			setList := updates[0].sql[:strings.Index(updates[0].sql, "WHERE")]
			if strings.Contains(setList, "trang_thai") || strings.Contains(setList, "so_lan_mo_lai") {
				t.Errorf("chấm %d sao mà câu lệnh đổi trạng thái/số lần mở lại: %q", stars, updates[0].sql)
			}
			if !strings.Contains(updates[0].sql,
				"tenant_id = $1 AND id = $2 AND trang_thai = $6 AND deleted_at IS NULL") {
				t.Errorf("UPDATE không lọc đủ xã/id/trạng thái/xoá mềm: %q", updates[0].sql)
			}
			if updates[0].args[0] != string(xaThu) {
				t.Errorf("$1 = %v, muốn xã trong context", updates[0].args[0])
			}
			if k.coCau("INSERT INTO su_kien_di") {
				t.Errorf("%d sao không đổi trạng thái mà vẫn phát status_changed", stars)
			}
			if len(k.cau("INSERT INTO audit_log")) != 1 || k.daCommit != 1 {
				t.Errorf("vết = %d, commit = %d; muốn 1 và 1", len(k.cau("INSERT INTO audit_log")), k.daCommit)
			}
			if d := ratingDelta(t, k); d["mo_lai"] != false {
				t.Errorf("mo_lai = %v, muốn false", d["mo_lai"])
			}
			assertRatingLogRow(t, k, domain.LogActionCitizenRating, st, stars, false)
		}
	}
}

// --- 1–2 stars: reopen -----------------------------------------------------------------------------

// TestRateLowReopensInOneTransaction is THE case of this file: the reopening, its trail and the
// citizen's notification commit together, the counter goes up with no cap, and NO deadline moves.
func TestRateLowReopensInOneTransaction(t *testing.T) {
	for _, st := range []domain.TrangThai{domain.DaXuLy, domain.ChoDanXacNhan} {
		for _, stars := range []int{1, 2} {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(ratedPetition(st, map[string]any{"so_lan_mo_lai": int64(7)}))
			uc, ctx := buildRating(t, k)

			after, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: stars, Comment: fixtureComment},
				citizenActor())
			if err != nil {
				t.Fatalf("%s %d sao: %v", st, stars, err)
			}

			// NO CAP: the eighth reopening goes through like the first.
			if after.TrangThai != domain.DangXuLy || after.SoLanMoLai != 8 {
				t.Errorf("sau = %s lần=%d, muốn dang-xu-ly lần=8", after.TrangThai, after.SoLanMoLai)
			}
			// DEADLINES UNCHANGED (rule 10, invariant 2).
			if !after.HanXuLyXong.Equal(mocXuLyXongThu) || !after.HanTiepNhan.Equal(mocTiepNhanThu) ||
				!after.HanPhanLoai.Equal(mocTranPhanLoaiThu) {
				t.Errorf("hạn bị dời khi mở lại: xử lý=%v tiếp nhận=%v phân loại=%v",
					after.HanXuLyXong, after.HanTiepNhan, after.HanPhanLoai)
			}
			if !after.XuLyXongLuc.IsZero() {
				t.Error("xu_ly_xong_luc còn giữ — phiếu mở lại sẽ mãi 'đúng hạn' theo lần xong cũ")
			}

			updates := k.cau("UPDATE phieu_phan_anh")
			if len(updates) != 1 {
				t.Fatalf("có %d câu UPDATE, muốn 1 — đánh giá và mở lại là MỘT câu lệnh", len(updates))
			}
			for _, want := range []string{"trang_thai = 'dang-xu-ly'", "so_lan_mo_lai = so_lan_mo_lai + 1",
				"xu_ly_xong_luc = NULL", "dong_luc = NULL", "diem_hai_long = $3",
				"WHERE tenant_id = $1 AND id = $2 AND trang_thai = $6 AND deleted_at IS NULL"} {
				if !strings.Contains(updates[0].sql, want) {
					t.Errorf("câu mở lại thiếu %q: %q", want, updates[0].sql)
				}
			}
			for _, banned := range []string{"han_tiep_nhan", "han_xu_ly_xong", "han_phan_loai", "goc_dem_han"} {
				if strings.Contains(updates[0].sql, banned) {
					t.Errorf("câu mở lại chạm cột hạn %q — ADR 0050: không tính lại hạn", banned)
				}
			}
			if !coThamSo(updates[0].args, string(st)) {
				t.Errorf("UPDATE không mang trạng thái mong đợi %q: %v", st, updates[0].args)
			}

			// FOUR WRITES, ONE TRANSACTION.
			for name, prefix := range map[string]string{
				"đổi phiếu": "UPDATE phieu_phan_anh",
				"vết kiểm":  "INSERT INTO audit_log",
				"nhật ký":   "INSERT INTO nhat_ky_phan_anh",
				"sự kiện":   "INSERT INTO su_kien_di",
			} {
				stmts := k.cau(prefix)
				if len(stmts) != 1 {
					t.Fatalf("%s: %d câu, muốn 1", name, len(stmts))
				}
				if !stmts[0].trongGiaoDich {
					t.Errorf("%s chạy ngoài giao dịch (luật 6 cấm #2)", name)
				}
			}
			if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
				t.Errorf("giao dịch: mở=%d commit=%d rollback=%d", k.batDau, k.daCommit, k.daRollback)
			}
			assertRatingLogRow(t, k, domain.LogActionReopenByRating, domain.DangXuLy, stars, true)

			// THE NOTIFICATION: status dang-xu-ly, occurrence = the new count + 1, a sentence owed.
			event := k.cau("INSERT INTO su_kien_di")[0]
			body := thanSuKien(t, event.args)
			if body["status"] != string(domain.DangXuLy) || body["citizen_id"] != idCongDanThu ||
				body["lookup_code"] != maPhieuThu {
				t.Errorf("sự kiện sai: %v", body)
			}
			if body["occurrence"] != float64(9) {
				t.Errorf("occurrence = %v, muốn 9 (so_lan_mo_lai mới 8 + 1)", body["occurrence"])
			}
			msg, ok := body["citizen_message"].(map[string]any)
			if !ok || msg["next_step"] != domain.ReopenNextStep(after) ||
				msg["status_label"] != domain.NhanTrangThai(domain.DangXuLy) {
				t.Errorf("lời báo mở lại sai hoặc thiếu (luật 10 bất biến 5): %v", body["citizen_message"])
			}
			if strings.Contains(string(event.args[4].([]byte)), "Rác ở cuối ngõ") {
				t.Error("nhận xét của dân đi trên sự kiện — hàng đợi được sao lưu và nhân bản")
			}

			// THE TRAIL: who (the citizen), before/after, the unchanged deadlines, no comment text.
			d := ratingDelta(t, k)
			if d["mo_lai"] != true {
				t.Errorf("mo_lai = %v, muốn true", d["mo_lai"])
			}
			before, _ := d["truoc"].(map[string]any)
			afterSnap, _ := d["sau"].(map[string]any)
			if before["trang_thai"] != string(st) || afterSnap["trang_thai"] != string(domain.DangXuLy) ||
				before["so_lan_mo_lai"] != float64(7) || afterSnap["so_lan_mo_lai"] != float64(8) ||
				afterSnap["diem_hai_long"] != float64(stars) {
				t.Errorf("trước/sau sai: %v / %v", before, afterSnap)
			}
			if d["han_xu_ly_xong"] != mocXuLyXongThu.Format(time.RFC3339) {
				t.Errorf("vết không ghi hạn xử lý (không đổi): %v", d["han_xu_ly_xong"])
			}
			if d["do_dai_nhan_xet"] != float64(len([]rune(fixtureComment))) {
				t.Errorf("do_dai_nhan_xet = %v", d["do_dai_nhan_xet"])
			}
			entry := k.cau("INSERT INTO audit_log")[0]
			if !coThamSo(entry.args, idCongDanThu) || !coThamSo(entry.args, AuditActionCitizenRating) {
				t.Errorf("vết không ghi người dân là chủ thể hoặc sai hành vi: %v", entry.args)
			}
			for _, a := range entry.args {
				s, _ := a.(string)
				b, _ := a.([]byte)
				if strings.Contains(s, "Rác ở cuối ngõ") || strings.Contains(string(b), "Rác ở cuối ngõ") {
					t.Error("nhận xét của dân nằm trong vết kiểm (luật 6 cấm #4)")
				}
			}
		}
	}
}

// assertRatingLogRow checks the one timeline row: in the transaction, the requirement's sentence, the
// status AFTER the act, the fixed citizen marker as author — NEVER the citizen id — and no comment.
//
// Argument order is the store's INSERT: tenant, id, petition id, at, author, action, status, unit,
// assignee, note.
func assertRatingLogRow(t *testing.T, k *khoPhieuXuLyGia, action domain.HanhViNhatKy,
	status domain.TrangThai, stars int, reopened bool) {
	t.Helper()
	rows := k.cau("INSERT INTO nhat_ky_phan_anh")
	if len(rows) != 1 {
		t.Fatalf("có %d dòng nhật ký, muốn 1 — màn cán bộ không thấy dân đã chấm", len(rows))
	}
	r := rows[0]
	if !r.trongGiaoDich {
		t.Error("dòng nhật ký ghi ngoài giao dịch")
	}
	if len(r.args) != 10 {
		t.Fatalf("dòng nhật ký có %d tham số, muốn 10", len(r.args))
	}
	if r.args[0] != string(xaThu) || r.args[2] != idPhieuThu || r.args[4] != domain.CitizenLogActor ||
		r.args[5] != string(action) || r.args[6] != string(status) || r.args[7] != nil || r.args[8] != nil ||
		r.args[9] != domain.RatingLogText(stars, reopened) {
		t.Errorf("dòng nhật ký sai: %v", r.args)
	}
	if at, _ := r.args[3].(time.Time); !at.Equal(ratedAtFixture) {
		t.Errorf("thời điểm dòng nhật ký = %v, muốn %v", r.args[3], ratedAtFixture)
	}
	if coThamSo(r.args, idCongDanThu) {
		t.Error("định danh công dân lên dòng nhật ký màn cán bộ — phiếu ẩn danh sẽ bị nối được với nhau")
	}
	if coThamSo(r.args, fixtureComment) {
		t.Error("nhận xét của dân bị chép vào nhật ký — nó chỉ sống ở rating_comment")
	}
}

// TestLogActionsAreAllowedByTheSchema ties the timeline act codes this service WRITES to the CHECK
// that will accept them — the latest `nhat_ky_phan_anh_hanh_vi_hop_le` in the embedded migrations.
//
// WHY IT EXISTS: every fake-driver test above is green whether or not PostgreSQL would take the row,
// and a code the CHECK refuses rolls the WHOLE rating back — in production, on every rating, with this
// suite green. That is the "green for the wrong reason" defect class this repository keeps meeting.
//
// ⚠ RED UNTIL THE MIGRATION WIDENING THE CHECK LANDS (`danh-gia`, `mo-lai-theo-danh-gia`). That is
// the dependency made mechanical, not a flaky test.
func TestLogActionsAreAllowedByTheSchema(t *testing.T) {
	allowed := latestLogActionCheck(t)
	for _, a := range []domain.HanhViNhatKy{
		domain.NhatKyPhanLoai, domain.NhatKyPhanCong, domain.NhatKyChuyenTrangThai, domain.NhatKyDongPhieu,
		domain.NhatKyKhongTiepNhan, domain.NhatKyChuyenCapTren, domain.NhatKyGhiChu,
		domain.LogActionCitizenRating, domain.LogActionReopenByRating,
		domain.LogActionTaskCreated, // migration 0023 (task from a petition, 30/09/2026)
	} {
		if !allowed[string(a)] {
			t.Errorf("mã hành vi %q không có trong CHECK nhat_ky_phan_anh_hanh_vi_hop_le mới nhất — "+
				"mọi lần ghi sẽ bị PostgreSQL từ chối và cả giao dịch lùi lại", a)
		}
	}
}

// latestLogActionCheck reads the act list of the LAST migration (by file name) that declares the
// constraint, so a later widening supersedes 0013 exactly as it will in the database.
func latestLogActionCheck(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("đọc migrations nhúng: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	// THE DECLARATION ITSELF, not any mention of the name: a rollback comment such as
	// `-- ALTER TABLE … DROP CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le;` must not be read as a list.
	checkDecl := regexp.MustCompile(
		`(?is)nhat_ky_phan_anh_hanh_vi_hop_le\s+CHECK\s*\(\s*hanh_vi\s+IN\s*\(([^)]*)\)`)
	var list string
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatalf("đọc %s: %v", n, err)
		}
		if ms := checkDecl.FindAllStringSubmatch(string(b), -1); len(ms) > 0 {
			list = ms[len(ms)-1][1]
		}
	}
	if list == "" {
		t.Fatal("không tìm thấy CHECK nhat_ky_phan_anh_hanh_vi_hop_le trong migrations — bộ đọc đã mù")
	}
	allowed := map[string]bool{}
	for _, part := range strings.Split(list, ",") {
		if v := strings.Trim(strings.TrimSpace(part), "'"); v != "" {
			allowed[v] = true
		}
	}
	if !allowed[string(domain.NhatKyPhanLoai)] {
		t.Fatalf("bộ đọc lấy sai danh sách: %v", allowed)
	}
	return allowed
}

// TestRateLowOutboxFailureRollsEverythingBack — the last write failing takes the reopening and the
// trail down with it: there is no committed reopening the citizen was never told about.
func TestRateLowOutboxFailureRollsEverythingBack(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, nil))
	k.loiSau = "INSERT INTO su_kien_di"
	uc, ctx := buildRating(t, k)

	if _, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 1}, citizenActor()); err == nil {
		t.Fatal("lỗi ghi sự kiện bị nuốt")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d; muốn 0 và 1", k.daCommit, k.daRollback)
	}
}

// TestStaffAdvanceStillCannotReopen — the owner's condition on the new `da-xu-ly -> dang-xu-ly` edge:
// reachable ONLY through the citizen's rating. Before the edge, the staff advance route took `da-xu-ly`
// to `cho-dan-xac-nhan` and refused `cho-dan-xac-nhan`; it must do exactly that still, over the real
// store, and no statement it runs may touch the reopen counter.
//
// THE MUTATION THAT MUST TURN THIS RED: `DaXuLy: DangXuLy` in domain.tienTrinhChinh, or a target
// status accepted on the /status body.
func TestStaffAdvanceStillCannotReopen(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(ratedPetition(domain.DaXuLy, nil))
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	after, err := uc.TienTrangThai(ctx, maPhieuThu, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("TienTrangThai: %v", err)
	}
	if after.TrangThai != domain.ChoDanXacNhan {
		t.Fatalf("cán bộ tiến từ da-xu-ly tới %q — chỉ dân chấm 1–2 sao mới mở lại", after.TrangThai)
	}
	for _, l := range k.cau("UPDATE phieu_phan_anh") {
		if strings.Contains(l.sql, "so_lan_mo_lai") || strings.Contains(l.sql, "'dang-xu-ly'") {
			t.Errorf("tuyến cán bộ chạm số lần mở lại: %q", l.sql)
		}
		if coThamSo(l.args, string(domain.DangXuLy)) {
			t.Errorf("tuyến cán bộ ghi dang-xu-ly từ da-xu-ly: %v", l.args)
		}
	}

	k2 := khoPhieuMau()
	k2.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, nil))
	uc2, ctx2 := dungXuLy(t, k2, hanXuLyThu())
	if _, err := uc2.TienTrangThai(ctx2, maPhieuThu, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); !errors.Is(err, domain.ErrKhongConCamKet) {
		t.Errorf("cán bộ tiến từ cho-dan-xac-nhan: lỗi = %v, muốn ErrKhongConCamKet như trước", err)
	}
	if k2.coCau("UPDATE phieu_phan_anh") {
		t.Error("tuyến cán bộ ghi dữ liệu trên phiếu chờ dân xác nhận")
	}
}

// TestRateAgainReplacesThePreviousRating — after a reopen and a second resolve, the citizen rates
// again: the new rating replaces the old, and the old values are in the trail's `truoc`.
func TestRateAgainReplacesThePreviousRating(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, map[string]any{
		"diem_hai_long":  int64(1),
		"rating_comment": "Chưa dọn.",
		"danh_gia_luc":   mocThaoTac,
	}))
	uc, ctx := buildRating(t, k)

	after, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 5}, citizenActor())
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if after.Rating != 5 || after.RatingComment != "" || !after.RatedAt.Equal(ratedAtFixture) {
		t.Errorf("đánh giá mới không thay đánh giá cũ: %d %q %v", after.Rating, after.RatingComment, after.RatedAt)
	}
	update := k.cau("UPDATE phieu_phan_anh")[0]
	// $4 is NULL: "no comment" replaces the old comment whole, never half of it.
	if update.args[3] != nil {
		t.Errorf("rating_comment = %v, muốn NULL khi lần chấm mới không có nhận xét", update.args[3])
	}
	d := ratingDelta(t, k)
	before, _ := d["truoc"].(map[string]any)
	if before["diem_hai_long"] != float64(1) || before["co_nhan_xet"] != true {
		t.Errorf("vết không giữ đánh giá cũ: %v", before)
	}
	afterSnap, _ := d["sau"].(map[string]any)
	if afterSnap["diem_hai_long"] != float64(5) || afterSnap["co_nhan_xet"] != false {
		t.Errorf("vết ghi đánh giá mới sai: %v", afterSnap)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Chưa dọn") {
		t.Error("nhận xét cũ nằm trong vết kiểm")
	}
}
