package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the two write use cases of the deadline table, proved against a REAL store
// over a REAL transaction (see driver_gia_sla_test.go for what that buys and what it does not).
//
// THE CASE THAT CARRIES THE MOST WEIGHT IS TestGieoLanHaiKhongGhiDeConSoXaDaSua. Everything else
// here is ordinary care; that one is the property a plausible implementation — an UPSERT — silently
// destroys, and the destruction is invisible: no error, no duplicate row, just a commune's edited
// commitment quietly back to the vendor's example figure.

// --- GieoMacDinh: the first run ------------------------------------------------------------------

// A COMMUNE WITH AN EMPTY TABLE GETS ALL FIFTEEN ROWS, and each one is asserted by (loai_viec,
// linh_vuc) rather than by count alone: fifteen INSERTs of the wrong fifteen rows would pass a
// count.
func TestGieoLanDauChenDuMuoiLamDong(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)

	kq, err := uc.GieoMacDinh(ctx, nguoiSLA())
	if err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 15 || kq.DaCo != 0 {
		t.Fatalf("kết quả = {gieo:%d, đã có:%d}, muốn {15, 0}", kq.DaGieo, kq.DaCo)
	}

	chen := k.cau("INSERT INTO sla")
	if len(chen) != 15 {
		t.Fatalf("có %d câu INSERT, muốn 15", len(chen))
	}

	// Every seed pair reached the database, exactly once.
	//
	// THE PAIR IS READ OFF THE BOUND PARAMETERS ($3 loai_viec, $4 linh_vuc), not off the SQL text,
	// so a statement that named the columns correctly and bound them in the wrong order fails here.
	co := map[string]bool{}
	for _, l := range chen {
		if len(l.args) < 9 {
			t.Fatalf("INSERT có %d tham số, muốn 9: %s", len(l.args), l.sql)
		}
		lv, _ := l.args[2].(string)
		linhVuc := ""
		if l.args[3] != nil {
			linhVuc, _ = l.args[3].(string)
		}
		k := lv + "/" + linhVuc
		if co[k] {
			t.Errorf("dòng %q được chèn hai lần trong MỘT lượt gieo", k)
		}
		co[k] = true
	}
	for _, g := range domain.BoGieoSLA() {
		if !co[string(g.LoaiViec)+"/"+g.LinhVuc] {
			t.Errorf("lượt gieo không chèn dòng %q/%q", g.LoaiViec, g.LinhVuc)
		}
	}
}

// THE DEFAULT ROW GOES IN AS NULL, NEVER AS ”.
//
// THE SCHEMA REFUSES ” OUTRIGHT (sla_linh_vuc_khong_rong), so getting this wrong does not create a
// second default row — it fails the whole seeding transaction at a commune's very first
// configuration. The fold happens in exactly one place (store.Chen) and this is what pins it.
func TestGieoChenDongMacDinhVoiLinhVucNULL(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.GieoMacDinh(ctx, nguoiSLA()); err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}

	var soMacDinh int
	for _, l := range k.cau("INSERT INTO sla") {
		if l.args[3] == nil {
			soMacDinh++
			continue
		}
		if s, _ := l.args[3].(string); s == "" {
			t.Errorf("dòng mặc định được chèn với linh_vuc = '' thay vì NULL — "+
				"CHECK sla_linh_vuc_khong_rong sẽ từ chối và hỏng cả lượt gieo (câu: %s)", l.sql)
		}
	}
	// Three kinds of work, three default rows.
	if soMacDinh != 3 {
		t.Errorf("có %d dòng chèn với linh_vuc NULL, muốn 3 (van-ban-den, phan-anh, nhiem-vu)", soMacDinh)
	}
}

// EVERY COMMUNE IS $1 ON EVERY STATEMENT, from the context (rule 1, invariants 4 and 5). Nothing in
// either use case takes a commune as a parameter, and this is what proves the store does not offer a
// way round that.
func TestGieoMoiCauMangDungXaCuaNguCanh(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.GieoMacDinh(ctx, nguoiSLA()); err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}

	for _, l := range k.lenh {
		if strings.Contains(l.sql, "sla") || strings.Contains(l.sql, "audit_log") {
			if len(l.args) == 0 || l.args[0] != string(xaSLA) {
				t.Errorf("câu lệnh không mang xã ở $1: %s (args[0] = %v)", l.sql, l.args[0])
			}
		}
	}
}

// --- GieoMacDinh: the second run — THE LOAD-BEARING CASES ----------------------------------------

// PRESSING THE BUTTON A SECOND TIME WRITES NOTHING AND AUDITS NOTHING.
//
// A duplicate row is the obvious failure; the quiet one is an audit entry recording an act that did
// not happen, which is how a ledger that is never deleted fills with noise that buries the entries
// carrying legal weight.
//
// MUTATION THAT MUST TURN THIS RED: drop the `if co[...] { kq.DaCo++; continue }` skip in
// GieoMacDinh.
func TestGieoLanHaiKhongTaoBanTrung(t *testing.T) {
	k := &khoSLAGia{hang: bangDayDu()}
	uc, ctx := dungUseCaseSLA(t, k)

	kq, err := uc.GieoMacDinh(ctx, nguoiSLA())
	if err != nil {
		t.Fatalf("GieoMacDinh lần hai lỗi: %v", err)
	}
	if kq.DaGieo != 0 || kq.DaCo != 15 {
		t.Fatalf("kết quả = {gieo:%d, đã có:%d}, muốn {0, 15}", kq.DaGieo, kq.DaCo)
	}
	if n := k.soCau("INSERT INTO sla"); n != 0 {
		t.Errorf("lượt gieo thứ hai chèn %d dòng — phải chèn 0", n)
	}
	if k.coCau("audit_log") {
		t.Error("lượt gieo thứ hai ghi vết kiểm toán dù không ghi gì — " +
			"một vết cho một hành vi không xảy ra sẽ chôn lấp những vết có giá trị pháp lý")
	}
}

// ==================================================================================================
// THE CASE THIS WHOLE FILE EXISTS FOR: A SECOND SEEDING RUN MUST NOT RESTORE THE VENDOR'S FIGURE
// OVER A NUMBER THE COMMUNE TYPED.
//
// The fixture is a fully configured commune whose `van-ban-den` default row has gio_xu_ly_xong = 24,
// where the seed set says 40. An UPSERT — the obvious way to make seeding idempotent — satisfies
// "no duplicate row" and silently puts 40 back. Nothing errors, the row count is right, and the
// commune's published commitment has changed to one nobody chose.
//
// THE ASSERTION IS ABOUT STATEMENTS, NOT ABOUT RESULTS, deliberately: it demands that NO UPDATE of
// any kind was issued against `sla`. Asserting on a returned value would let an implementation that
// writes and then reads back pass.
//
// MUTATION THAT MUST TURN THIS RED: make GieoMacDinh call kho.CapNhatGio for a row it found.
// ==================================================================================================
func TestGieoLanHaiKhongGhiDeConSoXaDaSua(t *testing.T) {
	k := &khoSLAGia{hang: bangDayDu()}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.GieoMacDinh(ctx, nguoiSLA()); err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}

	if n := k.soCau("UPDATE sla"); n != 0 {
		t.Fatalf("lượt gieo phát ra %d câu UPDATE sla — bộ gieo KHÔNG BAO GIỜ được ghi đè. "+
			"Xã đã sửa 40 giờ thành %d; một lần bấm nút gieo mà mất con số ấy là hỏng nặng hơn "+
			"không có nút", n, gioXaDaSua)
	}
	// Belt and braces: not one statement of any shape carried the seed's own 40 back towards the
	// edited row. An UPSERT spelled as INSERT ... ON CONFLICT DO UPDATE would be caught here even if
	// it never said the word UPDATE at the start of a statement.
	for _, l := range k.cau("sla") {
		if strings.Contains(strings.ToUpper(l.sql), "ON CONFLICT") {
			t.Errorf("bộ gieo dùng ON CONFLICT — hình dạng ấy làm 'đã có sẵn' không phân biệt được "+
				"với 'vừa ghi', và biến thể DO UPDATE ghi đè con số xã đã sửa: %s", l.sql)
		}
	}
}

// A PARTLY CONFIGURED COMMUNE GETS ONLY WHAT IT LACKS — the case between the two above, and the one
// a commune actually meets after the field catalogue grows.
func TestGieoChiBuNhungDongConThieu(t *testing.T) {
	day := bangDayDu()
	// Remove two rows: one field row and one DEFAULT row, because the default row is the one whose
	// absence is invisible on the screen.
	var giu []hangSLA
	for _, h := range day {
		if h.loaiViec == "phan-anh" && h.linhVuc == "dien" {
			continue
		}
		if h.loaiViec == "nhiem-vu" && h.linhVuc == nil {
			continue
		}
		giu = append(giu, h)
	}

	k := &khoSLAGia{hang: giu}
	uc, ctx := dungUseCaseSLA(t, k)

	kq, err := uc.GieoMacDinh(ctx, nguoiSLA())
	if err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 2 || kq.DaCo != 13 {
		t.Fatalf("kết quả = {gieo:%d, đã có:%d}, muốn {2, 13}", kq.DaGieo, kq.DaCo)
	}

	chen := k.cau("INSERT INTO sla")
	if len(chen) != 2 {
		t.Fatalf("có %d câu INSERT, muốn 2", len(chen))
	}
	if n := k.soCau("UPDATE sla"); n != 0 {
		t.Errorf("bù dòng thiếu mà vẫn phát ra %d câu UPDATE — 13 dòng còn lại phải nguyên vẹn", n)
	}
}

// THE SEEDING DECISION IS MADE INSIDE THE TRANSACTION IT WRITES IN.
//
// A read outside it is a check with a gap in the middle: two concurrent runs both see an empty
// table and both insert. What actually serialises them is the unique key, but only if the read that
// decided is inside the transaction the INSERTs roll back with.
//
// MUTATION THAT MUST TURN THIS RED: have GieoMacDinh read through uc.db.For(ctx) instead of the tx.
func TestGieoDocBangTrongCungGiaoDichVoiCacCauChen(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.GieoMacDinh(ctx, nguoiSLA()); err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}

	if k.batDau != 1 {
		t.Fatalf("mở %d giao dịch, muốn đúng 1", k.batDau)
	}
	if k.daCommit != 1 {
		t.Fatalf("commit %d lần, muốn 1", k.daCommit)
	}
	// The SELECT is on the record, and it is the first statement — so it ran after BeginTx and
	// before the first INSERT.
	doc := k.cau("SELECT")
	if len(doc) == 0 {
		t.Fatal("không có câu SELECT nào — quyết định gieo đang dựa vào đâu?")
	}
	if !strings.Contains(k.lenh[0].sql, "SELECT") {
		t.Errorf("câu đầu tiên của giao dịch không phải SELECT: %s", k.lenh[0].sql)
	}
	// FOR UPDATE must NOT appear: the rows this decision is about are the ones that do not exist,
	// and PostgreSQL cannot lock an absent row. A lock here would read as a protection that is not
	// the one holding (see store.DanhSachDeGhi).
	if strings.Contains(doc[0].sql, "FOR UPDATE") {
		t.Error("câu đọc của bộ gieo mang FOR UPDATE — nó khoá đúng những dòng KHÔNG ai định " +
			"đụng tới, và giả vờ là phép bảo vệ đang giữ (khoá duy nhất mới là thứ giữ)")
	}
}

// THE AUDIT ENTRY SHARES THE TRANSACTION WITH THE INSERTS (rule 6, invariant 3).
//
// THE ACTOR IS THE STAFF CODE, NEVER THE INTERNAL ID (rule 6, invariant 8). The two are different
// values in nguoiSLA() precisely so this can tell them apart — a ULID is a perfectly valid string in
// `audit_log.actor_id` and nothing downstream would notice.
func TestGieoGhiVetCungGiaoDichVaChuTheLaMaCanBo(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.GieoMacDinh(ctx, nguoiSLA()); err != nil {
		t.Fatalf("GieoMacDinh lỗi: %v", err)
	}

	vet := k.cau("audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết kiểm toán, muốn đúng 1 cho một lượt gieo "+
			"(15 vết sẽ mô tả 15 hành vi không hề xảy ra)", len(vet))
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0",
			k.batDau, k.daCommit, k.daRollback)
	}

	// audit_log columns: tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta
	a := vet[0].args
	if len(a) < 8 {
		t.Fatalf("vết có %d tham số, muốn 8", len(a))
	}
	if a[1] != maCanBoSLA {
		t.Errorf("actor_id = %v, muốn MÃ CÁN BỘ %q — một ULID không gọi tên ai với người "+
			"đọc hồ sơ nhiều năm sau (luật 6 bất biến 8)", a[1], maCanBoSLA)
	}
	if a[1] == nguoiSLA().ID {
		t.Error("actor_id đang là ID nội bộ")
	}
	if a[4] != HanhViGieoSLA {
		t.Errorf("action = %v, muốn %q", a[4], HanhViGieoSLA)
	}
	if a[5] != chuDeGieoSLA {
		t.Errorf("subject = %v, muốn %q", a[5], chuDeGieoSLA)
	}

	// The delta names the rows written, so an inspection can see which figures the commune started
	// from without the source of that day's release.
	var delta struct {
		DaGieo int      `json:"da_gieo"`
		DaCo   int      `json:"da_co"`
		Dong   []string `json:"dong"`
	}
	b, _ := a[7].([]byte)
	if err := json.Unmarshal(b, &delta); err != nil {
		t.Fatalf("delta không đọc được: %v (%s)", err, string(b))
	}
	if delta.DaGieo != 15 || len(delta.Dong) != 15 {
		t.Errorf("delta = {da_gieo:%d, dong:%d}, muốn {15, 15}", delta.DaGieo, len(delta.Dong))
	}
}

// A FAILURE ON THE LAST STATEMENT — THE AUDIT ENTRY — TAKES EVERY INSERT DOWN WITH IT.
//
// This is the invariant rule 6, forbidden #2 is about, and it is the one a fake store cannot have an
// opinion about: it needs the inserts to have really run first.
func TestGieoHongVetThiKhongDongNaoDuocGhi(t *testing.T) {
	k := &khoSLAGia{loiSau: "audit_log"}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.GieoMacDinh(ctx, nguoiSLA()); err == nil {
		t.Fatal("vết hỏng mà GieoMacDinh vẫn báo thành công")
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần dù vết kiểm toán hỏng — 15 dòng đã vào bảng mà không ai biết ai gieo", k.daCommit)
	}
	if k.daRollback != 1 {
		t.Errorf("rollback %d lần, muốn 1", k.daRollback)
	}
	// The INSERTs really ran — otherwise this case would prove nothing about the rollback.
	if n := k.soCau("INSERT INTO sla"); n != 15 {
		t.Errorf("có %d câu INSERT trước khi vết hỏng, muốn 15 — nếu 0 thì ca này không chứng minh gì", n)
	}
}

// AN ACTOR WITH NO STAFF CODE REFUSES THE WRITE, AND NEVER FALLS BACK TO THE INTERNAL ID.
// Empty happens when this service talks to an identity older than the `ma` field; a substitute here
// would put ULIDs into `audit_log.actor_id` silently, with every test still green.
func TestGieoTuChoiKhiThieuMaCanBo(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)

	nguoi := nguoiSLA()
	nguoi.Vet.ID = ""
	if _, err := uc.GieoMacDinh(ctx, nguoi); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn gieo")
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch dù người thực hiện không hợp lệ — phải từ chối TRƯỚC", k.batDau)
	}
}

// --- Sua -----------------------------------------------------------------------------------------

// dongDeSua is one live row the FOR UPDATE read hands back: `phan-anh` / `an-ninh`, the row whose
// real figures are 2/16/4/8/16.
func dongDeSua() []hangSLA {
	return []hangSLA{{
		id: idDongSLA, loaiViec: "phan-anh", linhVuc: "an-ninh", gio: [5]int{2, 16, 4, 8, 16},
	}}
}

func gio(n int) *int { return &n }

// A PARTIAL EDIT CHANGES ONLY THE FIGURE THAT WAS SENT, and the UPDATE still carries all five —
// the four unchanged ones read back off the row inside the transaction.
func TestSuaChiDoiConSoDuocGui(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	sau, err := uc.Sua(ctx, idDongSLA, YeuCauSuaSLA{GioXuLyXong: gio(12)}, nguoiSLA())
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}

	muon := domain.DongSLA{
		ID: idDongSLA, LoaiViec: domain.LoaiViecPhanAnh, LinhVuc: "an-ninh",
		GioTiepNhan: 2, GioXuLyXong: 12, GioSapDenHan: 4, GioBaoLanhDao: 8, GioBaoChuTich: 16,
	}
	if sau != muon {
		t.Fatalf("dòng sau khi sửa = %+v, muốn %+v", sau, muon)
	}

	up := k.cau("UPDATE sla")
	if len(up) != 1 {
		t.Fatalf("có %d câu UPDATE, muốn 1", len(up))
	}
	// $1 xã · $2 id · $3..$7 năm con số, in the order of cotSLA.
	muonArgs := []any{string(xaSLA), idDongSLA, 2, 12, 4, 8, 16}
	if len(up[0].args) != len(muonArgs) {
		t.Fatalf("UPDATE có %d tham số, muốn %d: %s", len(up[0].args), len(muonArgs), up[0].sql)
	}
	for i, m := range muonArgs {
		if got, ok := up[0].args[i].(int64); ok {
			if int(got) != m.(int) {
				t.Errorf("UPDATE $%d = %d, muốn %v", i+1, got, m)
			}
			continue
		}
		if up[0].args[i] != m {
			t.Errorf("UPDATE $%d = %v, muốn %v", i+1, up[0].args[i], m)
		}
	}
}

// THE UPDATE NAMES THE FIVE HOUR COLUMNS AND NOTHING ELSE.
//
// `loai_viec` or `linh_vuc` in a SET clause would let a screen silently re-point a live commitment
// at another field — and it is the parameter that would need the write-time code check ADR 0026
// stop condition #2 governs. A soft-delete column there would resurrect a removed row.
//
// MUTATION THAT MUST TURN THIS RED: add `linh_vuc = $8` to store.CapNhatGio.
func TestSuaKhongDongToiLoaiViecLinhVucHayCotXoaMem(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.Sua(ctx, idDongSLA, YeuCauSuaSLA{GioTiepNhan: gio(3)}, nguoiSLA()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}

	up := k.cau("UPDATE sla")[0].sql
	dat := up[strings.Index(up, "SET"):]
	if i := strings.Index(dat, "WHERE"); i >= 0 {
		dat = dat[:i]
	}
	for _, cam := range []string{"loai_viec", "linh_vuc", "deleted_at", "deleted_by", "delete_reason", "tenant_id"} {
		if strings.Contains(dat, cam) {
			t.Errorf("mệnh đề SET có %q — câu lệnh sửa số giờ không được đụng tới cột này: %s", cam, dat)
		}
	}
	for _, phai := range []string{
		"gio_tiep_nhan", "gio_xu_ly_xong", "gio_sap_den_han", "gio_bao_lanh_dao", "gio_bao_chu_tich",
	} {
		if !strings.Contains(dat, phai) {
			t.Errorf("mệnh đề SET thiếu %q: %s", phai, dat)
		}
	}
}

// THE ROW IS READ `FOR UPDATE` INSIDE THE TRANSACTION. Without the lock two administrators editing
// one row both read the old figures and the second write discards the first — on a table whose
// values are commitments to citizens.
func TestSuaDocDongBangFORUPDATETrongGiaoDich(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.Sua(ctx, idDongSLA, YeuCauSuaSLA{GioXuLyXong: gio(20)}, nguoiSLA()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if !k.coCau("FOR UPDATE") {
		t.Error("không có câu đọc FOR UPDATE — hai người sửa cùng một dòng thì lần ghi sau âm thầm " +
			"xoá lần ghi trước")
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("giao dịch: mở %d, commit %d — muốn 1/1", k.batDau, k.daCommit)
	}
}

// SENDING THE FIGURES A ROW ALREADY HAS WRITES NOTHING AND AUDITS NOTHING. That is what makes the
// route's idem.KhongCan declaration true rather than hopeful.
func TestSuaKhongDoiGiThiKhongGhiVaKhongCoVet(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	// The very figures the row already holds.
	_, err := uc.Sua(ctx, idDongSLA, YeuCauSuaSLA{
		GioTiepNhan: gio(2), GioXuLyXong: gio(16), GioSapDenHan: gio(4),
		GioBaoLanhDao: gio(8), GioBaoChuTich: gio(16),
	}, nguoiSLA())
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if n := k.soCau("UPDATE sla"); n != 0 {
		t.Errorf("gửi đúng giá trị đang có mà vẫn phát ra %d câu UPDATE", n)
	}
	if k.coCau("audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết — những vết như thế chôn lấp vết có giá trị pháp lý")
	}
}

// THE RESULT IS VALIDATED, NOT THE REQUEST, AND A REFUSAL COMMITS NOTHING.
func TestSuaTuChoiSoGioKhongHopLeVaKhongGhiGi(t *testing.T) {
	for ten, yc := range map[string]YeuCauSuaSLA{
		"0 giờ":  {GioXuLyXong: gio(0)},
		"số âm":  {GioTiepNhan: gio(-4)},
		"quá to": {GioSapDenHan: gio(domain.GioToiDa + 1)},
	} {
		k := &khoSLAGia{hang: dongDeSua()}
		uc, ctx := dungUseCaseSLA(t, k)

		_, err := uc.Sua(ctx, idDongSLA, yc, nguoiSLA())
		if err == nil {
			t.Errorf("%s: không bị từ chối", ten)
			continue
		}
		if !LaLoiDauVaoSLA(err) {
			t.Errorf("%s: lỗi %v không được nhận là lỗi đầu vào, nên tuyến sẽ trả 500 thay vì 400", ten, err)
		}
		if n := k.soCau("UPDATE sla"); n != 0 {
			t.Errorf("%s: bị từ chối mà vẫn phát ra %d câu UPDATE", ten, n)
		}
		if k.daCommit != 0 {
			t.Errorf("%s: commit %d lần dù bị từ chối", ten, k.daCommit)
		}
	}
}

// AN ID MATCHING NO LIVE ROW OF THIS COMMUNE IS ErrDongSLAKhongTonTai — the sentinel the handler
// turns into 404. The same answer for an invented id, a soft-deleted row and another authority's
// row, because every statement is scoped and all three genuinely produce no match.
func TestSuaIDLaTraVeKhongTonTai(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	_, err := uc.Sua(ctx, "01JKHONGTONTAI0000000000XX", YeuCauSuaSLA{GioXuLyXong: gio(9)}, nguoiSLA())
	if !errors.Is(err, idstore.ErrDongSLAKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrDongSLAKhongTonTai", err)
	}
	if n := k.soCau("UPDATE sla"); n != 0 {
		t.Errorf("id không tồn tại mà vẫn phát ra %d câu UPDATE", n)
	}
}

// THE AUDIT ENTRY CARRIES BEFORE AND AFTER OF THE FIGURES THAT MOVED (rule 6, invariant 5), names
// the row by `<loai_viec>/<linh_vuc>` rather than by ULID, and shares the transaction.
func TestSuaGhiVetCoTruocVaSauVaChuDeDocDuoc(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.Sua(ctx, idDongSLA, YeuCauSuaSLA{GioXuLyXong: gio(12)}, nguoiSLA()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}

	vet := k.cau("audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	a := vet[0].args
	if a[1] != maCanBoSLA {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q (luật 6 bất biến 8)", a[1], maCanBoSLA)
	}
	if a[4] != HanhViSuaSLA {
		t.Errorf("action = %v, muốn %q", a[4], HanhViSuaSLA)
	}
	if a[5] != "phan-anh/an-ninh" {
		t.Errorf("subject = %v, muốn %q — một ULID không nói với người xử lý khiếu nại là "+
			"cam kết NÀO đã đổi", a[5], "phan-anh/an-ninh")
	}

	var delta struct {
		Truoc map[string]int `json:"truoc"`
		Sau   map[string]int `json:"sau"`
	}
	b, _ := a[7].([]byte)
	if err := json.Unmarshal(b, &delta); err != nil {
		t.Fatalf("delta không đọc được: %v (%s)", err, string(b))
	}
	if delta.Truoc["gio_xu_ly_xong"] != 16 || delta.Sau["gio_xu_ly_xong"] != 12 {
		t.Errorf("delta gio_xu_ly_xong: trước=%d sau=%d, muốn 16 -> 12",
			delta.Truoc["gio_xu_ly_xong"], delta.Sau["gio_xu_ly_xong"])
	}
	// The unchanged figures are carried on both halves, so the entry is a complete picture of the
	// row rather than a fragment somebody has to reconstruct.
	if delta.Truoc["gio_tiep_nhan"] != 2 || delta.Sau["gio_tiep_nhan"] != 2 {
		t.Errorf("delta gio_tiep_nhan: trước=%d sau=%d, muốn 2 -> 2",
			delta.Truoc["gio_tiep_nhan"], delta.Sau["gio_tiep_nhan"])
	}
}

// ==================================================================================================
// CHANGING A FIGURE TOUCHES NO DEADLINE ALREADY ISSUED.
//
// 14-cau-hinh.md:289 and rule 10, invariant 2. `han_tiep_nhan` and `han_xu_ly_xong` are columns on
// the PETITION and the DOCUMENT, in other services' schemas; this service cannot write to them at
// all. The assertion is therefore that the edit's statements NAME NOTHING BUT `sla` AND
// `audit_log` — which is what "no recomputation" looks like from inside this service.
//
// STATED LIMIT: this proves the edit path issues no such statement. It cannot prove that no OTHER
// code anywhere recomputes a stored deadline from this table — that is a repository-wide property,
// and the turn that added this file checked it by hand and found no such path.
// ==================================================================================================
func TestSuaKhongChamToiHanDaLuuTrenBatKyHoSoNao(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	if _, err := uc.Sua(ctx, idDongSLA, YeuCauSuaSLA{GioXuLyXong: gio(12)}, nguoiSLA()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}

	for _, l := range k.lenh {
		for _, cam := range []string{"han_tiep_nhan", "han_xu_ly_xong", "phieu_phan_anh", "van_ban_den", "nhiem_vu"} {
			if strings.Contains(l.sql, cam) {
				t.Errorf("câu lệnh nhắc tới %q — đổi bảng SLA KHÔNG được tính lại hạn đã phát ra "+
					"(14-cau-hinh.md:289, luật 10 bất biến 2): %s", cam, l.sql)
			}
		}
		if !strings.Contains(l.sql, "sla") && !strings.Contains(l.sql, "audit_log") {
			t.Errorf("câu lệnh đụng tới bảng ngoài `sla` và `audit_log`: %s", l.sql)
		}
	}
}
