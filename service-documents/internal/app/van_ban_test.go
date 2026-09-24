package app

// The properties of the two document registers that live in the SQL and in the transaction
// boundaries — asserted against the REAL store over the fake driver next door, never against a
// recorded list of method calls.
//
// THE MOST EXPENSIVE ONE IS FIRST: an issued number is never issued twice, and never given back.

import (
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// nguoiMau is the acting member of staff. `ID` HOLDS THE BUSINESS CODE — `CB-00123` names somebody
// years later with no lookup still alive, a ULID names nobody, and the two are indistinguishable on
// sight (rule 6, invariant 8). The handler builds this from `Principal.Ma` and from nothing else.
var nguoiMau = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func vaoSoMau() YeuCauVaoSoVanBanDen {
	return YeuCauVaoSoVanBanDen{
		NgayDen:       time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		SoKyHieu:      "1742-CV/BTCTU",
		CoQuanBanHanh: "Huyện uỷ",
		LoaiVanBan:    "cong-van",
		TrichYeu:      "Về việc rà soát hộ nghèo",
	}
}

func capSoDiMau() YeuCauCapSoVanBanDi {
	return YeuCauCapSoVanBanDi{
		NgayVanBan: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		LoaiVanBan: "cong-van",
		TrichYeu:   "Trả lời đơn của công dân",
		NoiNhan:    "UBND huyện",
		NguoiKy:    "Nguyễn Văn A, Chủ tịch",
	}
}

// --- the number series ----------------------------------------------------------------------------

func TestCapSo_DocDayDuoiFOR_UPDATE(t *testing.T) {
	// THE SYNTACTIC HALF. `FOR UPDATE` on the series read is the only thing that closes the window
	// between two clerks, and it is one phrase in one constant: deleting it breaks nothing that
	// compiles, nothing that a single-threaded test can see, and nothing PostgreSQL complains about.
	//
	// The BEHAVIOURAL half is the next test. Both are kept: this one names the mechanism, so a reader
	// who removes the phrase is told what it was for rather than watching a concurrency test go red
	// for reasons that could be scheduling.
	k := khoVBMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.Them(ctx, vaoSoMau(), nguoiMau); err != nil {
		t.Fatalf("Them: %v", err)
	}

	doc := k.cau("FROM day_so_van_ban")
	if len(doc) != 1 {
		t.Fatalf("đọc dãy số %d lần, muốn đúng 1", len(doc))
	}
	if !strings.Contains(doc[0].sql, "FOR UPDATE") {
		t.Fatalf("câu đọc dãy số KHÔNG có `FOR UPDATE`:\n%s\n\n"+
			"Không có khoá hàng thì hai cán bộ cùng bấm sẽ đọc cùng một số và cùng ghi số kế tiếp — "+
			"một số đã cấp được cấp lại (luật 7 bất biến 3). Phép kiểm ở tầng ứng dụng không thay "+
			"được: nó đọc một giá trị mà giao dịch khác có thể đã dời.", doc[0].sql)
	}
}

func TestCapSo_HaiLuotSongSongRaHaiSoKhacNhau(t *testing.T) {
	// THE BEHAVIOURAL HALF, and the fake driver models the row lock so that this means something:
	// a `SELECT … FOR UPDATE` on the series takes a token held until the transaction ends, exactly
	// as PostgreSQL holds the row under READ COMMITTED.
	//
	// `treDocDay` MAKES THE WINDOW DETERMINISTIC rather than a matter of scheduling: the fake sleeps
	// at the end of every series read. With the lock, the second reader is still blocked and the
	// delay costs nothing. Without it, both readers see the same value on EVERY run — which is the
	// difference between a test that catches the mutation and a test that catches it sometimes.
	k := khoVBMau()
	k.treDocDay = 30 * time.Millisecond
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 2) // two connections: two clerks, two transactions

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		so  []int
		loi []error
	)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				loi = append(loi, err)
				return
			}
			so = append(so, v.SoVaoSo)
		}()
	}
	wg.Wait()

	if len(loi) > 0 {
		t.Fatalf("một lượt vào sổ hỏng: %v", loi[0])
	}
	if len(so) != 2 {
		t.Fatalf("có %d số, muốn 2", len(so))
	}
	if so[0] == so[1] {
		t.Fatalf("HAI VĂN BẢN CÙNG MỘT SỐ (%d) — số đã cấp bị cấp lại. Đây là hai hồ sơ lưu trữ "+
			"không phân biệt được nữa, và một trong hai đã có người ký (luật 7 bất biến 3)", so[0])
	}
	if a, b := min2(so[0], so[1]), max2(so[0], so[1]); a != 1 || b != 2 {
		t.Fatalf("hai số = %d và %d, muốn 1 và 2 — dãy số phải liên tục, không nhảy cóc", a, b)
	}
}

func TestCapSo_XoaMemKhongTraSoVeDay(t *testing.T) {
	// A SOFT DELETE TOUCHES `day_so_van_ban` NOWHERE, and the next document takes the NEXT number —
	// so the removed entry leaves a visible gap in the register. THE GAP IS THE RECORD: a register
	// that silently closed its own holes would be a register nobody can audit, and the number of a
	// removed document may already be on paper somebody holds.
	k := khoVBMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	dau, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if dau.SoVaoSo != 1 {
		t.Fatalf("số đầu = %d, muốn 1", dau.SoVaoSo)
	}

	k.hangDen = &hangVBD{
		id: dau.ID, soVaoSo: dau.SoVaoSo, nam: dau.Nam, ngayDen: dau.NgayDen,
		coQuan: dau.CoQuanBanHanh, loai: dau.LoaiVanBan, trichYeu: dau.TrichYeu,
		han: dau.HanXuLyXong, trangThai: string(domain.VanBanMoiVaoSo), nguoiTao: nguoiMau.ID,
	}
	truocKhiGo := len(k.cau("UPDATE day_so_van_ban"))
	if err := uc.Go(ctx, dau.ID, "vào sổ nhầm, đã có bản khác", nguoiMau); err != nil {
		t.Fatalf("Go: %v", err)
	}
	if n := len(k.cau("UPDATE day_so_van_ban")); n != truocKhiGo {
		t.Fatal("xoá mềm đã chạm vào dãy số — số đã cấp không bao giờ được trả về dãy")
	}

	// The three columns rule 7, invariant 1 names, in ONE statement so none can be forgotten.
	go1 := k.cau("UPDATE van_ban_den")
	cuoi := go1[len(go1)-1].sql
	for _, cot := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !strings.Contains(cuoi, cot) {
			t.Errorf("câu xoá mềm thiếu cột %q:\n%s", cot, cuoi)
		}
	}
	if !strings.Contains(cuoi, "AND deleted_at IS NULL") {
		t.Error("câu xoá mềm thiếu `AND deleted_at IS NULL` — lần gỡ thứ hai sẽ ghi đè người gỡ và lý do")
	}

	k.hangDen = nil // the row is gone from every read path now
	sau, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
	if err != nil {
		t.Fatalf("Them lần hai: %v", err)
	}
	if sau.SoVaoSo != 2 {
		t.Fatalf("số sau khi gỡ = %d, muốn 2 — số 1 đã cấp thì không cấp lại", sau.SoVaoSo)
	}
}

func TestCapSo_SangNamMoiDayBatDauLaiTuMot(t *testing.T) {
	// ADMINISTRATIVE PRACTICE, AND IT IS THE `nam` COLUMN OF THE COUNTER THAT IMPLEMENTS IT: a new
	// year is a key that does not exist yet, so `so_cuoi` starts at 0 and the first document takes 1.
	// "Công văn số 12/2026" and "số 12/2027" are two different documents and both are correct.
	k := khoVBMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	v2026, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
	if err != nil {
		t.Fatalf("Them 2026: %v", err)
	}
	if v2026.Nam != 2026 || v2026.SoVaoSo != 1 {
		t.Fatalf("2026: số %d năm %d, muốn 1/2026", v2026.SoVaoSo, v2026.Nam)
	}

	// The clock moves to January. THE YEAR OF THE SERIES IS THE YEAR OF THE ACT, not of `ngay_den` —
	// see VanBanDen.Them, where that reading is stated and flagged.
	uc.nay = func() time.Time { return time.Date(2027, 1, 4, 8, 0, 0, 0, time.UTC) }
	yc := vaoSoMau()
	yc.NgayDen = time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC)

	v2027, err := uc.Them(ctx, yc, nguoiMau)
	if err != nil {
		t.Fatalf("Them 2027: %v", err)
	}
	if v2027.Nam != 2027 || v2027.SoVaoSo != 1 {
		t.Fatalf("2027: số %d năm %d, muốn 1/2027 — sang năm mới dãy bắt đầu lại",
			v2027.SoVaoSo, v2027.Nam)
	}
	// AND THE 2026 SERIES IS UNTOUCHED: the next 2026 document would still be number 2. Two series,
	// two counter rows, told apart by `nam` in the key.
	if k.soCuoi["den|2026"] != 1 {
		t.Fatalf("dãy 2026 = %d, muốn 1 — dãy năm cũ không được đụng tới", k.soCuoi["den|2026"])
	}
}

func TestCapSo_TuChoiTruocKhiLaySo(t *testing.T) {
	// A BOOKING THAT IS GOING TO BE REFUSED MUST NOT CONSUME A NUMBER. The counter only moves
	// forward, so a number taken by a request that then fails is a permanent hole in the register —
	// and "why is there no số 7" is a question somebody has to answer years later.
	//
	// The order of two statements inside the transaction is the whole of this property: the type is
	// checked, THEN the number is taken.
	k := khoVBMau()
	k.loaiCoTrongDanhMuc = false
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.Them(ctx, vaoSoMau(), nguoiMau); err == nil {
		t.Fatal("loại văn bản không có trong danh mục mà vẫn vào sổ được")
	}
	if k.coCau("UPDATE day_so_van_ban") {
		t.Fatal("một lượt vào sổ BỊ TỪ CHỐI đã lấy mất một số của dãy")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit=%d rollback=%d, muốn 0/1 — từ chối thì không được commit gì",
			k.daCommit, k.daRollback)
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Fatal("từ chối mà vẫn ghi vết — không có hành vi nghiệp vụ nào xảy ra")
	}
}

// --- rule 6: the trail shares the transaction -----------------------------------------------------

func TestVaoSo_VetNamTrongCUNGGiaoDichVoiBanGhi(t *testing.T) {
	// RULE 6, INVARIANT 3 — the one the whole of internal/app exists for. One BEGIN, one COMMIT, and
	// the audit INSERT recorded between them: there is no window where the document exists and the
	// trail does not.
	k := khoVBMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	v, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("begin=%d commit=%d rollback=%d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("ghi vết %d lần, muốn 1", len(vet))
	}
	// THE SUBJECT IS A BUSINESS CODE, not the internal id (rule 6, invariant 8): an inspection reads
	// `VB-DEN-2026-0001` and can open that page of the register; a ULID names nothing.
	muonMa := domain.MaVanBanDen(v.Nam, v.SoVaoSo)
	if !coGiaTri(vet[0].args, muonMa) {
		t.Fatalf("vết không mang mã nghiệp vụ %q: %v", muonMa, vet[0].args)
	}
	if !coGiaTri(vet[0].args, nguoiMau.ID) {
		t.Fatalf("vết không mang MÃ cán bộ %q — `actor_id` giữ mã nghiệp vụ, không giữ id nội bộ "+
			"(luật 6 bất biến 8): %v", nguoiMau.ID, vet[0].args)
	}
	if !coGiaTri(vet[0].args, HanhViVaoSoVanBanDen) {
		t.Fatalf("vết không mang hành vi %q: %v", HanhViVaoSoVanBanDen, vet[0].args)
	}

	// AND `nguoi_tao_ma` HOLDS THE SAME KIND OF VALUE. Two kinds of identifier in one column is a
	// column nobody can query.
	chen := k.cau("INSERT INTO van_ban_den")
	if len(chen) != 1 || !coGiaTri(chen[0].args, nguoiMau.ID) {
		t.Fatalf("câu chèn không mang mã cán bộ: %v", chen)
	}
}

func TestVaoSo_VetHongThiKhongConSoVaKhongConVanBan(t *testing.T) {
	// THE PROPERTY THAT CANNOT BE PROVEN BY A FAKE STORE. The audit INSERT is the LAST statement of
	// the transaction, so failing it proves that everything before it — the number and the document —
	// rolls back with it. "Every write leaves a trail" is only true if the two cannot come apart.
	k := khoVBMau()
	k.loiSau = "INSERT INTO audit_log"
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.Them(ctx, vaoSoMau(), nguoiMau); err == nil {
		t.Fatal("ghi vết hỏng mà Them vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit=%d rollback=%d, muốn 0/1 — vết hỏng thì bản ghi nghiệp vụ phải cuộn lại theo",
			k.daCommit, k.daRollback)
	}
}

// --- rule 10: the commitment ----------------------------------------------------------------------

func TestVaoSo_HanXuLyHoiIdentityMotLanRoiLUU(t *testing.T) {
	// THE DEADLINE IS NOT COMPUTED HERE AND CANNOT BE. It is asked of identity, which owns both
	// halves of the arithmetic — the commune's SLA hours and the commune's working calendar
	// (ADR 0007, ADR 0029) — and the answer is STORED (rule 10, invariant 2). This pins the three
	// decisions that ride in the request.
	k := khoVBMau()
	uc, han, ctx := dungUseCaseVanBanDen(t, k, 1)

	v, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if han.goi != 1 {
		t.Fatalf("hỏi identity %d lần, muốn ĐÚNG 1 — tính lại lúc đọc là dời một cam kết đã hứa "+
			"(luật 10 bất biến 2)", han.goi)
	}
	if han.loai != identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN {
		t.Errorf("hỏi cho loại việc %v, muốn WORK_KIND_VAN_BAN_DEN", han.loai)
	}
	if han.linh != "" {
		t.Errorf("hỏi lĩnh vực %q, muốn dòng mặc định (rỗng) — sổ văn bản không có khái niệm lĩnh vực",
			han.linh)
	}
	if !han.tu.Equal(lucVaoSo) {
		t.Errorf("gốc đếm = %v, muốn đúng thời điểm vào sổ %v", han.tu, lucVaoSo)
	}
	if len(han.can) != 1 || han.can[0] != identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG {
		t.Errorf("hỏi các đồng hồ %v, muốn ĐÚNG MỘT: XU_LY_XONG — đồng hồ tiếp nhận không áp dụng "+
			"cho sổ do chính cán bộ vào (ADR 0028 quyết định E)", han.can)
	}
	if !v.HanXuLyXong.Equal(hanMau) {
		t.Errorf("hạn lưu = %v, muốn đúng giá trị identity trả %v", v.HanXuLyXong, hanMau)
	}

	// AND IT REACHED THE INSERT UNCHANGED. A deadline recomputed, rounded or shifted between the
	// answer and the column is a commitment nobody made.
	if !coGiaTri(k.cau("INSERT INTO van_ban_den")[0].args, hanMau) {
		t.Error("hạn identity trả không nằm trong câu chèn")
	}
}

func TestVaoSo_IdentityKhongTraLoiThiKHONGVaoSoVaKHONGMoGiaoDich(t *testing.T) {
	// THE ORDINARY CASE IN EVERY COMMUNE TODAY — `sla` is empty everywhere — and it must fail the
	// booking rather than invent a number of hours (rule 10, forbidden #3).
	//
	// IT ALSO ASSERTS THAT NO TRANSACTION WAS OPENED, which is a second property worth having: the
	// gRPC call happens BEFORE the transaction, so an identity outage never holds the counter's row
	// lock for the length of a network timeout — which would queue every booking in the commune
	// behind it.
	k := khoVBMau()
	uc, han, ctx := dungUseCaseVanBanDen(t, k, 1)
	han.loi = errThuNghiem

	_, err := uc.Them(ctx, vaoSoMau(), nguoiMau)
	if err == nil {
		t.Fatal("identity không trả lời được mà vẫn vào sổ — một hạn do phần mềm bịa vẫn được báo " +
			"lên như cam kết của chính quyền")
	}
	if !laLoi(err, ErrChuaAnDinhDuocHan) {
		t.Fatalf("lỗi = %v, muốn bọc ErrChuaAnDinhDuocHan", err)
	}
	if k.batDau != 0 {
		t.Fatalf("đã mở %d giao dịch — lời gọi identity phải nằm NGOÀI giao dịch, nếu không một sự "+
			"cố của identity sẽ giữ khoá dãy số suốt thời gian chờ mạng", k.batDau)
	}
	if k.coCau("INSERT INTO van_ban_den") {
		t.Fatal("chưa có hạn mà vẫn chèn văn bản")
	}
}

// --- routing --------------------------------------------------------------------------------------

func TestChuyen_BaViecMotGiaoDichVaLichSuChiThem(t *testing.T) {
	// THE DOCUMENT MOVES, THE TIMELINE RECORDS THE MOVE, THE TRAIL RECORDS WHO — in ONE transaction.
	// Split apart, any window between them is a register whose "Đang giữ" column and whose timeline
	// disagree, and the timeline is append-only, so there is no repair afterwards.
	k := khoVBMau()
	k.hangDen = &hangVBD{
		id: "vbd-001", soVaoSo: 7, nam: 2026, ngayDen: lucVaoSo,
		coQuan: "Huyện uỷ", loai: "cong-van", trichYeu: "x",
		han: hanMau, trangThai: string(domain.VanBanMoiVaoSo), nguoiTao: "CB-00001",
	}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	sau, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
		DenBoPhan:   "bp-dia-chinh",
		CanBoXuLyMa: "CB-00456",
		LyDo:        "Thuộc thẩm quyền của bộ phận Địa chính",
	}, nguoiMau)
	if err != nil {
		t.Fatalf("Chuyen: %v", err)
	}

	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("begin=%d commit=%d, muốn 1/1 — ba việc phải nằm trong MỘT giao dịch",
			k.batDau, k.daCommit)
	}
	if !k.coCau("UPDATE van_ban_den") {
		t.Error("không có câu chuyển bộ phận")
	}
	if !k.coCau("INSERT INTO lich_su_chuyen_van_ban") {
		t.Error("không có dòng lịch sử chuyển")
	}
	if !k.coCau("INSERT INTO audit_log") {
		t.Error("không có vết kiểm toán")
	}

	// THE STATE ONLY MOVES FORWARD: `moi-vao-so` becomes `da-phan-cong`, which is what assigning a
	// document to a department means.
	if sau.TrangThai != domain.VanBanDaPhanCong {
		t.Errorf("trạng thái sau khi chuyển = %q, muốn %q", sau.TrangThai, domain.VanBanDaPhanCong)
	}

	// THE TIMELINE ROW IS WRITTEN BY AN INSERT AND BY NOTHING ELSE. There is no UPDATE and no DELETE
	// against that table anywhere in this service — the trigger refuses both, and this is the half
	// that can be checked without a server.
	for _, cam := range []string{"UPDATE lich_su_chuyen_van_ban", "DELETE FROM lich_su_chuyen_van_ban"} {
		if k.coCau(cam) {
			t.Fatalf("có câu %q — lịch sử chuyển là bản ghi lịch sử, chỉ thêm (luật 7 cấm #5)", cam)
		}
	}
}

func TestChuyen_DangSuaMotDongLichSuThiKhongCoDuongNao(t *testing.T) {
	// THE "EDIT A ROUTING ENTRY → REFUSED" CASE, ASSERTED WHERE IT CAN ACTUALLY BE ASSERTED WITHOUT A
	// SERVER: there is no code path that reaches it. The store exposes exactly one method WRITING
	// that table and it INSERTs (the other, LichSuChuyen, only SELECTs); the use case calls it once
	// per routing. An edit would need a method
	// that does not exist, and the trigger `lich_su_chuyen_chi_them` refuses one underneath even if
	// somebody writes it (that half needs PostgreSQL — VIGOV_TEST_DSN is unset here).
	//
	// THIS IS THE HONEST SHAPE OF THE ASSERTION. A test that "tried to edit and expected an error"
	// would have to call a method nobody wrote, which proves nothing; what can be proven is that the
	// service emits no such statement, and that is what the scan below is.
	k := khoVBMau()
	k.hangDen = &hangVBD{
		id: "vbd-001", soVaoSo: 7, nam: 2026, ngayDen: lucVaoSo, coQuan: "H", loai: "cong-van",
		trichYeu: "x", han: hanMau, trangThai: string(domain.VanBanMoiVaoSo), nguoiTao: "CB-1",
	}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	for i := 0; i < 2; i++ {
		if _, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
			DenBoPhan: "bp-tu-phap", LyDo: "chuyển tiếp",
		}, nguoiMau); err != nil {
			t.Fatalf("Chuyen lần %d: %v", i+1, err)
		}
	}
	// TWO ROUTINGS, TWO ROWS — never one row rewritten. A correction is a new entry whose reason says
	// so, which is what an append-only history means in practice.
	if n := len(k.cau("INSERT INTO lich_su_chuyen_van_ban")); n != 2 {
		t.Fatalf("có %d dòng lịch sử sau hai lần chuyển, muốn 2", n)
	}
	for _, l := range k.lenh {
		s := strings.ToUpper(l.sql)
		if strings.Contains(s, "LICH_SU_CHUYEN_VAN_BAN") &&
			(strings.Contains(s, "UPDATE ") || strings.Contains(s, "DELETE ")) {
			t.Fatalf("dịch vụ phát ra câu sửa/xoá lịch sử chuyển:\n%s", l.sql)
		}
	}
}

func TestChuyen_VanBanDaKetThucThiTuChoiVaKhongGhiGi(t *testing.T) {
	// A settled document is not routed further: doing so would reopen a closed record as a side
	// effect of a routing form. THE REFUSAL WRITES NOTHING — no state change, no timeline row, no
	// trail — which is what "a refusal leaves the transaction with nothing committed" means.
	for _, tt := range []domain.TrangThaiVanBanDen{
		domain.VanBanDaGiaiQuyet, domain.VanBanChuyenCapTren, domain.VanBanLuuKhongThuLy,
	} {
		t.Run(string(tt), func(t *testing.T) {
			k := khoVBMau()
			k.hangDen = &hangVBD{
				id: "vbd-001", soVaoSo: 7, nam: 2026, ngayDen: lucVaoSo, coQuan: "H",
				loai: "cong-van", trichYeu: "x", han: hanMau, trangThai: string(tt), nguoiTao: "CB-1",
			}
			uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

			_, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
				DenBoPhan: "bp-tu-phap", LyDo: "chuyển tiếp",
			}, nguoiMau)
			if !laLoi(err, domain.ErrVanBanDaKetThuc) {
				t.Fatalf("lỗi = %v, muốn ErrVanBanDaKetThuc", err)
			}
			if k.coCau("INSERT INTO lich_su_chuyen_van_ban") || k.coCau("INSERT INTO audit_log") {
				t.Fatal("từ chối mà vẫn ghi lịch sử hoặc vết")
			}
			if k.daCommit != 0 {
				t.Fatalf("commit=%d, muốn 0", k.daCommit)
			}
		})
	}
}

func TestChuyen_ThieuLyDoThiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	// THE REASON IS MANDATORY, and it is refused BEFORE the transaction opens: a request that fails
	// its shape must never hold a row lock while doing so.
	k := khoVBMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{DenBoPhan: "bp"}, nguoiMau); !laLoi(err,
		domain.ErrThieuLyDoChuyen) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoChuyen", err)
	}
	if k.batDau != 0 {
		t.Fatalf("đã mở %d giao dịch cho một yêu cầu sai hình dạng", k.batDau)
	}
}

// --- the outgoing register ------------------------------------------------------------------------

func TestCapSoDi_CapSoRoiGhiVetTrongCungGiaoDich(t *testing.T) {
	// THE SAME THREE PROPERTIES AS THE INCOMING REGISTER, on the register whose number leaves the building:
	// the series is read under a lock, the number is stored with the document, and the trail shares
	// the transaction.
	k := khoVBMau()
	uc, ctx := dungUseCaseVanBanDi(t, k, 1)

	v, err := uc.CapSo(ctx, capSoDiMau(), nguoiMau)
	if err != nil {
		t.Fatalf("CapSo: %v", err)
	}
	if v.SoDi != 1 || v.Nam != 2026 {
		t.Fatalf("số/năm = %d/%d, muốn 1/2026", v.SoDi, v.Nam)
	}
	doc := k.cau("FROM day_so_van_ban")
	if len(doc) != 1 || !strings.Contains(doc[0].sql, "FOR UPDATE") {
		t.Fatal("câu đọc dãy số đi không có `FOR UPDATE` — số đi đã in ra giấy, đóng dấu và gửi ra " +
			"ngoài xã; hai văn bản cùng số là hai văn bản không ai phân biệt được")
	}
	// THE SERIES IS `di`, NOT `den`. Two registers, two independent counters — văn bản đến số 7 and
	// văn bản đi số 7 are two different documents in the same commune in the same year.
	if !coGiaTri(doc[0].args, string(docstore.SoSachDi)) {
		t.Fatalf("đọc dãy số với sổ %v, muốn %q", doc[0].args, docstore.SoSachDi)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("begin=%d commit=%d, muốn 1/1", k.batDau, k.daCommit)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || !coGiaTri(vet[0].args, domain.MaVanBanDi(v.Nam, v.SoDi)) {
		t.Fatalf("vết không mang mã nghiệp vụ %q: %v", domain.MaVanBanDi(v.Nam, v.SoDi), vet)
	}
}

func TestCapSoDi_HaiSoDocLapVoiSoDen(t *testing.T) {
	// ONE COUNTER TABLE, TWO SERIES, told apart by `so_sach` INSIDE the statement rather than by
	// which Go object issued them. A commune's số đến 1 and số đi 1 both exist in 2026 and both are
	// correct; a shared counter would make the outgoing register start at whatever the incoming one
	// had reached, and every outgoing number would then be wrong on paper.
	k := khoVBMau()
	ucDen, _, ctx := dungUseCaseVanBanDen(t, k, 1)
	ucDi, _ := dungUseCaseVanBanDi(t, k, 1)

	for i := 0; i < 3; i++ {
		if _, err := ucDen.Them(ctx, vaoSoMau(), nguoiMau); err != nil {
			t.Fatalf("Them %d: %v", i, err)
		}
	}
	di, err := ucDi.CapSo(ctx, capSoDiMau(), nguoiMau)
	if err != nil {
		t.Fatalf("CapSo: %v", err)
	}
	if di.SoDi != 1 {
		t.Fatalf("số đi đầu tiên = %d, muốn 1 — sổ đi có dãy riêng, không nối tiếp sổ đến", di.SoDi)
	}
	if k.soCuoi["den|2026"] != 3 || k.soCuoi["di|2026"] != 1 {
		t.Fatalf("dãy đến=%d đi=%d, muốn 3 và 1", k.soCuoi["den|2026"], k.soCuoi["di|2026"])
	}
}

// --- the edit path --------------------------------------------------------------------------------

func TestSua_KhongDoiThiKhongGhiVaKhongVet(t *testing.T) {
	// A NO-OP IS NOT AN EVENT. Recording it would fill a public authority's ledger with entries
	// saying nothing changed, and those are the entries that bury the ones carrying legal weight. It
	// is also what makes the route's `idem.KhongCan` declaration true rather than hopeful.
	k := khoVBMau()
	k.hangDen = &hangVBD{
		id: "vbd-001", soVaoSo: 7, nam: 2026,
		ngayDen: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		coQuan:  "Huyện uỷ", loai: "cong-van", trichYeu: "Về việc rà soát hộ nghèo",
		han: hanMau, trangThai: string(domain.VanBanMoiVaoSo), nguoiTao: "CB-1",
	}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	giuNguyen := "Về việc rà soát hộ nghèo"
	if _, err := uc.Sua(ctx, "vbd-001", YeuCauSuaVanBanDen{TrichYeu: &giuNguyen}, nguoiMau); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.coCau("UPDATE van_ban_den") {
		t.Fatal("không có gì đổi mà vẫn chạy UPDATE")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Fatal("không có gì đổi mà vẫn ghi vết")
	}
}

func TestSua_KhongChamVaoSoNamTrangThaiVaHan(t *testing.T) {
	// THE FOUR FACTS AN EDIT MAY NEVER MOVE, checked in the SQL rather than in a comment: the issued
	// number and its year (rule 7), the state (it moves by routing alone) and the deadline (rule 10,
	// invariant 2 — a commitment already made).
	k := khoVBMau()
	k.hangDen = &hangVBD{
		id: "vbd-001", soVaoSo: 7, nam: 2026,
		ngayDen: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		coQuan:  "Huyện uỷ", loai: "cong-van", trichYeu: "cũ",
		han: hanMau, trangThai: string(domain.VanBanMoiVaoSo), nguoiTao: "CB-1",
	}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	moi := "Trích yếu đã sửa"
	if _, err := uc.Sua(ctx, "vbd-001", YeuCauSuaVanBanDen{TrichYeu: &moi}, nguoiMau); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	cau := k.cau("UPDATE van_ban_den")
	if len(cau) != 1 {
		t.Fatalf("chạy %d câu UPDATE, muốn 1", len(cau))
	}
	for _, cot := range []string{"so_vao_so", "nam =", "trang_thai", "han_xu_ly_xong"} {
		if strings.Contains(cau[0].sql, cot) {
			t.Errorf("câu sửa có chạm vào %q — không được:\n%s", cot, cau[0].sql)
		}
	}
}

// --- helpers ---------------------------------------------------------------------------------------

// errThuNghiem stands in for any failure identity can return — no `sla` row, no working calendar,
// the service unreachable, the wrong caller key. The use case must answer the same way to all of
// them, so the test needs only one.
var errThuNghiem = errors.New("thử nghiệm: identity không trả lời")

// laLoi is errors.Is, named in Vietnamese so the assertions below read as sentences.
func laLoi(err, muon error) bool { return errors.Is(err, muon) }

// coGiaTri reports whether one bound parameter equals `muon`. Comparing VALUES rather than argument
// positions, so adding a column to a statement does not turn every assertion red for no reason.
func coGiaTri(args []driver.Value, muon any) bool {
	for _, a := range args {
		if a == muon {
			return true
		}
		if t, ok := a.(time.Time); ok {
			if m, ok2 := muon.(time.Time); ok2 && t.Equal(m) {
				return true
			}
		}
	}
	return false
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- the detail drawer ----------------------------------------------------------------------------

func hangDenMau() *hangVBD {
	return &hangVBD{
		id: "vbd-001", soVaoSo: 7, nam: 2026, ngayDen: lucVaoSo, coQuan: "Huyện uỷ",
		loai: "cong-van", trichYeu: "x", han: hanMau, trangThai: string(domain.VanBanDaPhanCong),
		nguoiTao: "CB-00001",
	}
}

func hangLSMau(id string, luc time.Time) []driver.Value {
	return []driver.Value{id, "vbd-001", luc, "CB-00002", string(domain.VanBanDangXuLy),
		"bp-van-phong", "bp-dia-chinh", "CB-00003", "Thuộc thẩm quyền Địa chính", luc}
}

// khongGhiGi asserts a read wrote nothing — no business write, no audit entry. Reading a register
// that holds no masked citizen field, inside one commune, is not audited (rule 6, invariant 7).
func khongGhiGi(t *testing.T, k *khoVBGia) {
	t.Helper()
	for _, l := range k.lenh {
		s := strings.ToUpper(l.sql)
		if strings.Contains(s, "INSERT ") || strings.Contains(s, "UPDATE ") || strings.Contains(s, "DELETE ") {
			t.Fatalf("tuyến đọc phát ra câu ghi:\n%s", l.sql)
		}
	}
}

func TestChiTiet_DocTheoXaBoDongDaGoVaKhongKhoa(t *testing.T) {
	// THE PREDICATE IS THE ISOLATION: `tenant_id = $1` bound from the context (rule 1) and
	// `deleted_at IS NULL` (rule 7, invariant 2). Without either, the 404 the handler promises for
	// "another commune's" and "removed" would be a 200. And NO `FOR UPDATE`: opening a drawer must not
	// queue a routing behind it.
	k := khoVBMau()
	k.hangDen = hangDenMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	v, err := uc.ChiTiet(ctx, "vbd-001")
	if err != nil {
		t.Fatalf("ChiTiet: %v", err)
	}
	if v.ID != "vbd-001" || v.SoVaoSo != 7 {
		t.Fatalf("đọc sai dòng: %+v", v)
	}
	cau := k.cau("FROM van_ban_den")
	if len(cau) != 1 {
		t.Fatalf("có %d câu đọc văn bản, muốn 1", len(cau))
	}
	for _, can := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL"} {
		if !strings.Contains(cau[0].sql, can) {
			t.Errorf("câu đọc thiếu %q:\n%s", can, cau[0].sql)
		}
	}
	if strings.Contains(cau[0].sql, "FOR UPDATE") {
		t.Error("câu đọc chi tiết khoá dòng — mở ngăn chi tiết không được chặn việc chuyển")
	}
	if cau[0].args[0] != string(xaA) {
		t.Errorf("$1 = %v, muốn xã của ngữ cảnh %q", cau[0].args[0], xaA)
	}
	khongGhiGi(t, k)
}

func TestChiTiet_KhongThayThiMotLoiDuyNhat(t *testing.T) {
	// No row — whether it never existed, was removed, or is another commune's — is ONE sentinel. The
	// handler maps it to one 404 body.
	k := khoVBMau()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	for _, id := range []string{"vbd-khong-co", ""} {
		if _, err := uc.ChiTiet(ctx, id); !errors.Is(err, docstore.ErrKhongThayVanBanDen) {
			t.Fatalf("id %q: lỗi = %v, muốn ErrKhongThayVanBanDen", id, err)
		}
	}
}

func TestLichSuChuyen_DocVanBanTruocRoiLichSuCuNhatTruoc(t *testing.T) {
	// THE DOCUMENT IS READ FIRST, in the same transaction — the timeline table has no soft-delete
	// column, so that read is what keeps a removed document's history off the wire. Then the rows,
	// OLDEST FIRST, bounded by the ceiling plus one.
	k := khoVBMau()
	k.hangDen = hangDenMau()
	cu := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	k.hangLichSu = [][]driver.Value{hangLSMau("ls-1", cu), hangLSMau("ls-2", cu.Add(time.Hour))}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	ds, err := uc.LichSuChuyen(ctx, "vbd-001")
	if err != nil {
		t.Fatalf("LichSuChuyen: %v", err)
	}
	if len(ds) != 2 || ds[0].ID != "ls-1" || ds[1].ID != "ls-2" {
		t.Fatalf("lịch sử = %+v, muốn ls-1 rồi ls-2", ds)
	}
	if ds[1].TuBoPhan != "bp-van-phong" || ds[1].DenBoPhan != "bp-dia-chinh" ||
		ds[1].CanBoXuLyMa != "CB-00003" || ds[1].TrangThaiTaiThoiDiem != domain.VanBanDangXuLy {
		t.Fatalf("cột đọc lệch vị trí: %+v", ds[1])
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("begin=%d commit=%d, muốn 1/1 — hai câu đọc trong MỘT giao dịch", k.batDau, k.daCommit)
	}

	thuTu := -1
	for i, l := range k.lenh {
		if strings.Contains(l.sql, "FROM van_ban_den") && thuTu == -1 {
			thuTu = i
		}
		if strings.Contains(l.sql, "FROM lich_su_chuyen_van_ban") && thuTu == -1 {
			t.Fatal("đọc lịch sử TRƯỚC khi kiểm văn bản còn thấy được")
		}
	}
	ls := k.cau("FROM lich_su_chuyen_van_ban")
	if len(ls) != 1 {
		t.Fatalf("có %d câu đọc lịch sử, muốn 1", len(ls))
	}
	for _, can := range []string{"tenant_id = $1", "van_ban_den_id = $2", "ORDER BY thoi_diem ASC, id ASC", "LIMIT $3"} {
		if !strings.Contains(ls[0].sql, can) {
			t.Errorf("câu đọc lịch sử thiếu %q:\n%s", can, ls[0].sql)
		}
	}
	if ls[0].args[0] != string(xaA) || ls[0].args[1] != "vbd-001" ||
		ls[0].args[2] != int64(docstore.TranLichSuChuyen+1) {
		t.Errorf("tham số = %v, muốn [%s vbd-001 %d]", ls[0].args, xaA, docstore.TranLichSuChuyen+1)
	}
	khongGhiGi(t, k)
}

func TestLichSuChuyen_VanBanKhongThayThiKhongDocLichSu(t *testing.T) {
	// Removed / another commune's / unknown: the same sentinel as ChiTiet, and the timeline is never
	// even queried.
	k := khoVBMau()
	k.hangLichSu = [][]driver.Value{hangLSMau("ls-1", lucVaoSo)}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.LichSuChuyen(ctx, "vbd-001"); !errors.Is(err, docstore.ErrKhongThayVanBanDen) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayVanBanDen", err)
	}
	if k.coCau("FROM lich_su_chuyen_van_ban") {
		t.Fatal("văn bản không thấy được mà vẫn đọc lịch sử chuyển của nó")
	}
}

func TestLichSuChuyen_VuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	k := khoVBMau()
	k.hangDen = hangDenMau()
	for i := 0; i <= docstore.TranLichSuChuyen; i++ {
		k.hangLichSu = append(k.hangLichSu, hangLSMau("ls", lucVaoSo))
	}
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	ds, err := uc.LichSuChuyen(ctx, "vbd-001")
	if !errors.Is(err, docstore.ErrQuaNhieuLichSuChuyen) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuLichSuChuyen", err)
	}
	if ds != nil {
		t.Fatal("vượt trần mà vẫn trả một danh sách — lịch sử cụt đọc như lịch sử đủ")
	}
}
