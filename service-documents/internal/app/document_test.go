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

// sampleActor is the acting member of staff. `ID` HOLDS THE BUSINESS CODE — `CB-00123` names
// somebody years later with no lookup still alive, a ULID names nobody, and the two are
// indistinguishable on sight (rule 6, invariant 8). The handler builds this from `Principal.Ma` and
// from nothing else.
var sampleActor = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func sampleRegistration() RegisterIncomingDocumentRequest {
	return RegisterIncomingDocumentRequest{
		ReceivedDate: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		ReferenceNo:  "1742-CV/BTCTU",
		IssuingBody:  "Huyện uỷ",
		DocumentType: "cong-van",
		Summary:      "Về việc rà soát hộ nghèo",
	}
}

func sampleIssue() IssueOutgoingDocumentRequest {
	return IssueOutgoingDocumentRequest{
		DocumentDate: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		DocumentType: "cong-van",
		Summary:      "Trả lời đơn của công dân",
		Recipient:    "UBND huyện",
		Signer:       "Nguyễn Văn A, Chủ tịch",
	}
}

// --- the number series ----------------------------------------------------------------------------

func TestIssueNumber_ReadsTheSeriesUnderFOR_UPDATE(t *testing.T) {
	// THE SYNTACTIC HALF. `FOR UPDATE` on the series read is the only thing that closes the window
	// between two clerks, and it is one phrase in one constant: deleting it breaks nothing that
	// compiles, nothing that a single-threaded test can see, and nothing PostgreSQL complains about.
	//
	// The BEHAVIOURAL half is the next test. Both are kept: this one names the mechanism, so a reader
	// who removes the phrase is told what it was for rather than watching a concurrency test go red
	// for reasons that could be scheduling.
	k := newFakeRegisterDB()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	if _, err := uc.Register(ctx, sampleRegistration(), sampleActor); err != nil {
		t.Fatalf("Register: %v", err)
	}

	reads := k.stmtsContaining("FROM day_so_van_ban")
	if len(reads) != 1 {
		t.Fatalf("đọc dãy số %d lần, muốn đúng 1", len(reads))
	}
	if !strings.Contains(reads[0].sql, "FOR UPDATE") {
		t.Fatalf("câu đọc dãy số KHÔNG có `FOR UPDATE`:\n%s\n\n"+
			"Không có khoá hàng thì hai cán bộ cùng bấm sẽ đọc cùng một số và cùng ghi số kế tiếp — "+
			"một số đã cấp được cấp lại (luật 7 bất biến 3). Phép kiểm ở tầng ứng dụng không thay "+
			"được: nó đọc một giá trị mà giao dịch khác có thể đã dời.", reads[0].sql)
	}
}

func TestIssueNumber_TwoConcurrentBookingsGetTwoNumbers(t *testing.T) {
	// THE BEHAVIOURAL HALF, and the fake driver models the row lock so that this means something:
	// a `SELECT … FOR UPDATE` on the series takes a token held until the transaction ends, exactly
	// as PostgreSQL holds the row under READ COMMITTED.
	//
	// `seriesReadDelay` MAKES THE WINDOW DETERMINISTIC rather than a matter of scheduling: the fake
	// sleeps at the end of every series read. With the lock, the second reader is still blocked and
	// the delay costs nothing. Without it, both readers see the same value on EVERY run — which is the
	// difference between a test that catches the mutation and a test that catches it sometimes.
	k := newFakeRegisterDB()
	k.seriesReadDelay = 30 * time.Millisecond
	uc, _, ctx := newIncomingUseCase(t, k, 2) // two connections: two clerks, two transactions

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		nos  []int
		errs []error
	)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := uc.Register(ctx, sampleRegistration(), sampleActor)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			nos = append(nos, d.ArrivalNo)
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("một lượt vào sổ hỏng: %v", errs[0])
	}
	if len(nos) != 2 {
		t.Fatalf("có %d số, muốn 2", len(nos))
	}
	if nos[0] == nos[1] {
		t.Fatalf("HAI VĂN BẢN CÙNG MỘT SỐ (%d) — số đã cấp bị cấp lại. Đây là hai hồ sơ lưu trữ "+
			"không phân biệt được nữa, và một trong hai đã có người ký (luật 7 bất biến 3)", nos[0])
	}
	if a, b := min2(nos[0], nos[1]), max2(nos[0], nos[1]); a != 1 || b != 2 {
		t.Fatalf("hai số = %d và %d, muốn 1 và 2 — dãy số phải liên tục, không nhảy cóc", a, b)
	}
}

func TestIssueNumber_SoftDeleteNeverReturnsTheNumber(t *testing.T) {
	// A SOFT DELETE TOUCHES `day_so_van_ban` NOWHERE, and the next document takes the NEXT number —
	// so the removed entry leaves a visible gap in the register. THE GAP IS THE RECORD: a register
	// that silently closed its own holes would be a register nobody can audit, and the number of a
	// removed document may already be on paper somebody holds.
	k := newFakeRegisterDB()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	first, err := uc.Register(ctx, sampleRegistration(), sampleActor)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if first.ArrivalNo != 1 {
		t.Fatalf("số đầu = %d, muốn 1", first.ArrivalNo)
	}

	k.incoming = &incomingRow{
		id: first.ID, arrivalNo: first.ArrivalNo, year: first.Year, receivedDate: first.ReceivedDate,
		issuingBody: first.IssuingBody, documentType: first.DocumentType, summary: first.Summary,
		due: first.DueAt, status: string(domain.IncomingStatusRegistered), createdBy: sampleActor.ID,
	}
	beforeRemove := len(k.stmtsContaining("UPDATE day_so_van_ban"))
	if err := uc.Remove(ctx, first.ID, "vào sổ nhầm, đã có bản khác", sampleActor); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if n := len(k.stmtsContaining("UPDATE day_so_van_ban")); n != beforeRemove {
		t.Fatal("xoá mềm đã chạm vào dãy số — số đã cấp không bao giờ được trả về dãy")
	}

	// The three columns rule 7, invariant 1 names, in ONE statement so none can be forgotten.
	updates := k.stmtsContaining("UPDATE van_ban_den")
	last := updates[len(updates)-1].sql
	for _, col := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !strings.Contains(last, col) {
			t.Errorf("câu xoá mềm thiếu cột %q:\n%s", col, last)
		}
	}
	if !strings.Contains(last, "AND deleted_at IS NULL") {
		t.Error("câu xoá mềm thiếu `AND deleted_at IS NULL` — lần gỡ thứ hai sẽ ghi đè người gỡ và lý do")
	}

	k.incoming = nil // the row is gone from every read path now
	next, err := uc.Register(ctx, sampleRegistration(), sampleActor)
	if err != nil {
		t.Fatalf("Register lần hai: %v", err)
	}
	if next.ArrivalNo != 2 {
		t.Fatalf("số sau khi gỡ = %d, muốn 2 — số 1 đã cấp thì không cấp lại", next.ArrivalNo)
	}
}

func TestIssueNumber_NewYearRestartsAtOne(t *testing.T) {
	// ADMINISTRATIVE PRACTICE, AND IT IS THE `nam` COLUMN OF THE COUNTER THAT IMPLEMENTS IT: a new
	// year is a key that does not exist yet, so `so_cuoi` starts at 0 and the first document takes 1.
	// "Công văn số 12/2026" and "số 12/2027" are two different documents and both are correct.
	k := newFakeRegisterDB()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	d2026, err := uc.Register(ctx, sampleRegistration(), sampleActor)
	if err != nil {
		t.Fatalf("Register 2026: %v", err)
	}
	if d2026.Year != 2026 || d2026.ArrivalNo != 1 {
		t.Fatalf("2026: số %d năm %d, muốn 1/2026", d2026.ArrivalNo, d2026.Year)
	}

	// The clock moves to January. THE YEAR OF THE SERIES IS THE YEAR OF THE ACT, not of `ngay_den` —
	// see IncomingDocuments.Register, where that reading is stated and flagged.
	uc.now = func() time.Time { return time.Date(2027, 1, 4, 8, 0, 0, 0, time.UTC) }
	req := sampleRegistration()
	req.ReceivedDate = time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC)

	d2027, err := uc.Register(ctx, req, sampleActor)
	if err != nil {
		t.Fatalf("Register 2027: %v", err)
	}
	if d2027.Year != 2027 || d2027.ArrivalNo != 1 {
		t.Fatalf("2027: số %d năm %d, muốn 1/2027 — sang năm mới dãy bắt đầu lại",
			d2027.ArrivalNo, d2027.Year)
	}
	// AND THE 2026 SERIES IS UNTOUCHED: the next 2026 document would still be number 2. Two series,
	// two counter rows, told apart by `nam` in the key.
	if k.lastNumber["den|2026"] != 1 {
		t.Fatalf("dãy 2026 = %d, muốn 1 — dãy năm cũ không được đụng tới", k.lastNumber["den|2026"])
	}
}

func TestIssueNumber_RefusalComesBeforeTheNumberIsTaken(t *testing.T) {
	// A BOOKING THAT IS GOING TO BE REFUSED MUST NOT CONSUME A NUMBER. The counter only moves
	// forward, so a number taken by a request that then fails is a permanent hole in the register —
	// and "why is there no số 7" is a question somebody has to answer years later.
	//
	// The order of two statements inside the transaction is the whole of this property: the type is
	// checked, THEN the number is taken.
	k := newFakeRegisterDB()
	k.typeInCatalogue = false
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	if _, err := uc.Register(ctx, sampleRegistration(), sampleActor); err == nil {
		t.Fatal("loại văn bản không có trong danh mục mà vẫn vào sổ được")
	}
	if k.hasStmt("UPDATE day_so_van_ban") {
		t.Fatal("một lượt vào sổ BỊ TỪ CHỐI đã lấy mất một số của dãy")
	}
	if k.committed != 0 || k.rolledBack != 1 {
		t.Fatalf("commit=%d rollback=%d, muốn 0/1 — từ chối thì không được commit gì",
			k.committed, k.rolledBack)
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("từ chối mà vẫn ghi vết — không có hành vi nghiệp vụ nào xảy ra")
	}
}

// --- rule 6: the trail shares the transaction -----------------------------------------------------

func TestRegister_TrailSharesTheTransactionWithTheRow(t *testing.T) {
	// RULE 6, INVARIANT 3 — the one the whole of internal/app exists for. One BEGIN, one COMMIT, and
	// the audit INSERT recorded between them: there is no window where the document exists and the
	// trail does not.
	k := newFakeRegisterDB()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	d, err := uc.Register(ctx, sampleRegistration(), sampleActor)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if k.begun != 1 || k.committed != 1 || k.rolledBack != 0 {
		t.Fatalf("begin=%d commit=%d rollback=%d, muốn 1/1/0", k.begun, k.committed, k.rolledBack)
	}

	trail := k.stmtsContaining("INSERT INTO audit_log")
	if len(trail) != 1 {
		t.Fatalf("ghi vết %d lần, muốn 1", len(trail))
	}
	// THE SUBJECT IS A BUSINESS CODE, not the internal id (rule 6, invariant 8): an inspection reads
	// `VB-DEN-2026-0001` and can open that page of the register; a ULID names nothing.
	wantCode := domain.IncomingDocumentCode(d.Year, d.ArrivalNo)
	if !hasValue(trail[0].args, wantCode) {
		t.Fatalf("vết không mang mã nghiệp vụ %q: %v", wantCode, trail[0].args)
	}
	if !hasValue(trail[0].args, sampleActor.ID) {
		t.Fatalf("vết không mang MÃ cán bộ %q — `actor_id` giữ mã nghiệp vụ, không giữ id nội bộ "+
			"(luật 6 bất biến 8): %v", sampleActor.ID, trail[0].args)
	}
	if !hasValue(trail[0].args, ActionRegisterIncomingDocument) {
		t.Fatalf("vết không mang hành vi %q: %v", ActionRegisterIncomingDocument, trail[0].args)
	}

	// AND `nguoi_tao_ma` HOLDS THE SAME KIND OF VALUE. Two kinds of identifier in one column is a
	// column nobody can query.
	inserts := k.stmtsContaining("INSERT INTO van_ban_den")
	if len(inserts) != 1 || !hasValue(inserts[0].args, sampleActor.ID) {
		t.Fatalf("câu chèn không mang mã cán bộ: %v", inserts)
	}
}

func TestRegister_FailedTrailLeavesNoNumberAndNoDocument(t *testing.T) {
	// THE PROPERTY THAT CANNOT BE PROVEN BY A FAKE STORE. The audit INSERT is the LAST statement of
	// the transaction, so failing it proves that everything before it — the number and the document —
	// rolls back with it. "Every write leaves a trail" is only true if the two cannot come apart.
	k := newFakeRegisterDB()
	k.failOn = "INSERT INTO audit_log"
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	if _, err := uc.Register(ctx, sampleRegistration(), sampleActor); err == nil {
		t.Fatal("ghi vết hỏng mà Register vẫn báo thành công")
	}
	if k.committed != 0 || k.rolledBack != 1 {
		t.Fatalf("commit=%d rollback=%d, muốn 0/1 — vết hỏng thì bản ghi nghiệp vụ phải cuộn lại theo",
			k.committed, k.rolledBack)
	}
}

// --- rule 10: the commitment ----------------------------------------------------------------------

func TestRegister_DeadlineAskedOfIdentityOnceThenStored(t *testing.T) {
	// THE DEADLINE IS NOT COMPUTED HERE AND CANNOT BE. It is asked of identity, which owns both
	// halves of the arithmetic — the commune's SLA hours and the commune's working calendar
	// (ADR 0007, ADR 0029) — and the answer is STORED (rule 10, invariant 2). This pins the three
	// decisions that ride in the request.
	k := newFakeRegisterDB()
	uc, deadlines, ctx := newIncomingUseCase(t, k, 1)

	d, err := uc.Register(ctx, sampleRegistration(), sampleActor)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if deadlines.calls != 1 {
		t.Fatalf("hỏi identity %d lần, muốn ĐÚNG 1 — tính lại lúc đọc là dời một cam kết đã hứa "+
			"(luật 10 bất biến 2)", deadlines.calls)
	}
	if deadlines.workKind != identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN {
		t.Errorf("hỏi cho loại việc %v, muốn WORK_KIND_VAN_BAN_DEN", deadlines.workKind)
	}
	if deadlines.fieldCode != "" {
		t.Errorf("hỏi lĩnh vực %q, muốn dòng mặc định (rỗng) — sổ văn bản không có khái niệm lĩnh vực",
			deadlines.fieldCode)
	}
	if !deadlines.from.Equal(registeredAt) {
		t.Errorf("gốc đếm = %v, muốn đúng thời điểm vào sổ %v", deadlines.from, registeredAt)
	}
	if len(deadlines.kinds) != 1 || deadlines.kinds[0] != identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG {
		t.Errorf("hỏi các đồng hồ %v, muốn ĐÚNG MỘT: XU_LY_XONG — đồng hồ tiếp nhận không áp dụng "+
			"cho sổ do chính cán bộ vào (ADR 0028 quyết định E)", deadlines.kinds)
	}
	if !d.DueAt.Equal(identityDue) {
		t.Errorf("hạn lưu = %v, muốn đúng giá trị identity trả %v", d.DueAt, identityDue)
	}

	// AND IT REACHED THE INSERT UNCHANGED. A deadline recomputed, rounded or shifted between the
	// answer and the column is a commitment nobody made.
	if !hasValue(k.stmtsContaining("INSERT INTO van_ban_den")[0].args, identityDue) {
		t.Error("hạn identity trả không nằm trong câu chèn")
	}
}

func TestRegister_NoIdentityAnswerMeansNoBookingAndNoTransaction(t *testing.T) {
	// THE ORDINARY CASE IN EVERY COMMUNE TODAY — `sla` is empty everywhere — and it must fail the
	// booking rather than invent a number of hours (rule 10, forbidden #3).
	//
	// IT ALSO ASSERTS THAT NO TRANSACTION WAS OPENED, which is a second property worth having: the
	// gRPC call happens BEFORE the transaction, so an identity outage never holds the counter's row
	// lock for the length of a network timeout — which would queue every booking in the commune
	// behind it.
	k := newFakeRegisterDB()
	uc, deadlines, ctx := newIncomingUseCase(t, k, 1)
	deadlines.err = errTest

	_, err := uc.Register(ctx, sampleRegistration(), sampleActor)
	if err == nil {
		t.Fatal("identity không trả lời được mà vẫn vào sổ — một hạn do phần mềm bịa vẫn được báo " +
			"lên như cam kết của chính quyền")
	}
	if !isErr(err, ErrDeadlineNotEstablished) {
		t.Fatalf("lỗi = %v, muốn bọc ErrDeadlineNotEstablished", err)
	}
	if k.begun != 0 {
		t.Fatalf("đã mở %d giao dịch — lời gọi identity phải nằm NGOÀI giao dịch, nếu không một sự "+
			"cố của identity sẽ giữ khoá dãy số suốt thời gian chờ mạng", k.begun)
	}
	if k.hasStmt("INSERT INTO van_ban_den") {
		t.Fatal("chưa có hạn mà vẫn chèn văn bản")
	}
}

// --- routing --------------------------------------------------------------------------------------

func TestRoute_ThreeWritesOneTransactionAppendOnlyHistory(t *testing.T) {
	// THE DOCUMENT MOVES, THE TIMELINE RECORDS THE MOVE, THE TRAIL RECORDS WHO — in ONE transaction.
	// Split apart, any window between them is a register whose "Đang giữ" column and whose timeline
	// disagree, and the timeline is append-only, so there is no repair afterwards.
	k := newFakeRegisterDB()
	k.incoming = &incomingRow{
		id: "vbd-001", arrivalNo: 7, year: 2026, receivedDate: registeredAt,
		issuingBody: "Huyện uỷ", documentType: "cong-van", summary: "x",
		due: identityDue, status: string(domain.IncomingStatusRegistered), createdBy: "CB-00001",
	}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	after, err := uc.Route(ctx, "vbd-001", RouteDocumentRequest{
		ToOrgUnitID:  "bp-dia-chinh",
		AssigneeCode: "CB-00456",
		Reason:       "Thuộc thẩm quyền của bộ phận Địa chính",
	}, sampleActor)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if k.begun != 1 || k.committed != 1 {
		t.Fatalf("begin=%d commit=%d, muốn 1/1 — ba việc phải nằm trong MỘT giao dịch",
			k.begun, k.committed)
	}
	if !k.hasStmt("UPDATE van_ban_den") {
		t.Error("không có câu chuyển bộ phận")
	}
	if !k.hasStmt("INSERT INTO lich_su_chuyen_van_ban") {
		t.Error("không có dòng lịch sử chuyển")
	}
	if !k.hasStmt("INSERT INTO audit_log") {
		t.Error("không có vết kiểm toán")
	}

	// THE STATE ONLY MOVES FORWARD: `moi-vao-so` becomes `da-phan-cong`, which is what assigning a
	// document to a department means.
	if after.Status != domain.IncomingStatusAssigned {
		t.Errorf("trạng thái sau khi chuyển = %q, muốn %q", after.Status, domain.IncomingStatusAssigned)
	}

	// THE TIMELINE ROW IS WRITTEN BY AN INSERT AND BY NOTHING ELSE. There is no UPDATE and no DELETE
	// against that table anywhere in this service — the trigger refuses both, and this is the half
	// that can be checked without a server.
	for _, forbidden := range []string{"UPDATE lich_su_chuyen_van_ban", "DELETE FROM lich_su_chuyen_van_ban"} {
		if k.hasStmt(forbidden) {
			t.Fatalf("có câu %q — lịch sử chuyển là bản ghi lịch sử, chỉ thêm (luật 7 cấm #5)", forbidden)
		}
	}
}

func TestRoute_HistoryRowRecordsNewStatusAndBothUnits(t *testing.T) {
	// THE ROW THE DRAWER NOW READS BACK, ASSERTED ON THE INSERT ARGUMENTS. The test above proves an
	// INSERT happens; nothing proved WHAT it records. GET .../routings serves `status` as "the state
	// the document moved INTO by this act" and `from_unit → to_unit` as the direction of travel —
	// and the table is append-only (trigger `lich_su_chuyen_chi_them`), so a wrong value written
	// here is wrong forever, on the record a complaint is answered from.
	//
	// `moi-vao-so` IS THE ONLY STARTING STATE WHERE BEFORE AND AFTER DIFFER (StatusAfterRouting),
	// so it is the only fixture in which writing `before.Status` instead of the new state turns red.
	// The holder is set so that swapping $7 and $8 turns red too.
	k := newFakeRegisterDB()
	k.incoming = &incomingRow{
		id: "vbd-001", arrivalNo: 7, year: 2026, receivedDate: registeredAt,
		issuingBody: "Huyện uỷ", documentType: "cong-van", summary: "x", orgUnit: "bp-van-phong",
		due: identityDue, status: string(domain.IncomingStatusRegistered), createdBy: "CB-00001",
	}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	if _, err := uc.Route(ctx, "vbd-001", RouteDocumentRequest{
		ToOrgUnitID: "bp-dia-chinh",
		Reason:      "Thuộc thẩm quyền của bộ phận Địa chính",
	}, sampleActor); err != nil {
		t.Fatalf("Route: %v", err)
	}

	history := k.stmtsContaining("INSERT INTO lich_su_chuyen_van_ban")
	if len(history) != 1 {
		t.Fatalf("có %d dòng lịch sử được chèn, muốn 1", len(history))
	}
	a := history[0].args
	if len(a) != 10 {
		t.Fatalf("câu chèn lịch sử có %d tham số, muốn 10: %v", len(a), a)
	}
	for _, c := range []struct {
		name      string
		got, want driver.Value
	}{
		{"$1 tenant_id — xã của ngữ cảnh", a[0], string(tenantA)},
		{"$3 van_ban_den_id", a[2], "vbd-001"},
		{"$5 nguoi_ma — MÃ cán bộ, không phải id nội bộ (luật 6, bất biến 8)", a[4], "CB-00123"},
		{"$6 trang_thai_tai_thoi_diem — trạng thái ĐÃ CHUYỂN VÀO, không phải trạng thái cũ", a[5],
			string(domain.IncomingStatusAssigned)},
		{"$7 tu_bo_phan_id — bộ phận đang giữ TRƯỚC lần chuyển", a[6], "bp-van-phong"},
		{"$8 den_bo_phan_id — bộ phận nhận", a[7], "bp-dia-chinh"},
		{"$9 can_bo_xu_ly_ma — để trống thì NULL, không phải chuỗi rỗng", a[8], nil},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, muốn %v", c.name, c.got, c.want)
		}
	}
}

func TestRoute_NoPathEditsAHistoryRow(t *testing.T) {
	// THE "EDIT A ROUTING ENTRY → REFUSED" CASE, ASSERTED WHERE IT CAN ACTUALLY BE ASSERTED WITHOUT A
	// SERVER: there is no code path that reaches it. The store exposes exactly one method WRITING
	// that table and it INSERTs (the other, RoutingHistory, only SELECTs); the use case calls it once
	// per routing. An edit would need a method
	// that does not exist, and the trigger `lich_su_chuyen_chi_them` refuses one underneath even if
	// somebody writes it (that half needs PostgreSQL — VIGOV_TEST_DSN is unset here).
	//
	// THIS IS THE HONEST SHAPE OF THE ASSERTION. A test that "tried to edit and expected an error"
	// would have to call a method nobody wrote, which proves nothing; what can be proven is that the
	// service emits no such statement, and that is what the scan below is.
	k := newFakeRegisterDB()
	k.incoming = &incomingRow{
		id: "vbd-001", arrivalNo: 7, year: 2026, receivedDate: registeredAt, issuingBody: "H", documentType: "cong-van",
		summary: "x", due: identityDue, status: string(domain.IncomingStatusRegistered), createdBy: "CB-1",
	}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	for i := 0; i < 2; i++ {
		if _, err := uc.Route(ctx, "vbd-001", RouteDocumentRequest{
			ToOrgUnitID: "bp-tu-phap", Reason: "chuyển tiếp",
		}, sampleActor); err != nil {
			t.Fatalf("Route lần %d: %v", i+1, err)
		}
	}
	// TWO ROUTINGS, TWO ROWS — never one row rewritten. A correction is a new entry whose reason says
	// so, which is what an append-only history means in practice.
	if n := len(k.stmtsContaining("INSERT INTO lich_su_chuyen_van_ban")); n != 2 {
		t.Fatalf("có %d dòng lịch sử sau hai lần chuyển, muốn 2", n)
	}
	for _, l := range k.stmts {
		s := strings.ToUpper(l.sql)
		if strings.Contains(s, "LICH_SU_CHUYEN_VAN_BAN") &&
			(strings.Contains(s, "UPDATE ") || strings.Contains(s, "DELETE ")) {
			t.Fatalf("dịch vụ phát ra câu sửa/xoá lịch sử chuyển:\n%s", l.sql)
		}
	}
}

func TestRoute_FinishedDocumentRefusedAndNothingWritten(t *testing.T) {
	// A settled document is not routed further: doing so would reopen a closed record as a side
	// effect of a routing form. THE REFUSAL WRITES NOTHING — no state change, no timeline row, no
	// trail — which is what "a refusal leaves the transaction with nothing committed" means.
	for _, s := range []domain.IncomingDocumentStatus{
		domain.IncomingStatusResolved, domain.IncomingStatusReferred, domain.IncomingStatusFiledNotAdmitted,
	} {
		t.Run(string(s), func(t *testing.T) {
			k := newFakeRegisterDB()
			k.incoming = &incomingRow{
				id: "vbd-001", arrivalNo: 7, year: 2026, receivedDate: registeredAt, issuingBody: "H",
				documentType: "cong-van", summary: "x", due: identityDue, status: string(s), createdBy: "CB-1",
			}
			uc, _, ctx := newIncomingUseCase(t, k, 1)

			_, err := uc.Route(ctx, "vbd-001", RouteDocumentRequest{
				ToOrgUnitID: "bp-tu-phap", Reason: "chuyển tiếp",
			}, sampleActor)
			if !isErr(err, domain.ErrDocumentFinished) {
				t.Fatalf("lỗi = %v, muốn ErrDocumentFinished", err)
			}
			if k.hasStmt("INSERT INTO lich_su_chuyen_van_ban") || k.hasStmt("INSERT INTO audit_log") {
				t.Fatal("từ chối mà vẫn ghi lịch sử hoặc vết")
			}
			if k.committed != 0 {
				t.Fatalf("commit=%d, muốn 0", k.committed)
			}
		})
	}
}

func TestRoute_MissingReasonRefusedBeforeTheTransaction(t *testing.T) {
	// THE REASON IS MANDATORY, and it is refused BEFORE the transaction opens: a request that fails
	// its shape must never hold a row lock while doing so.
	k := newFakeRegisterDB()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	if _, err := uc.Route(ctx, "vbd-001", RouteDocumentRequest{ToOrgUnitID: "bp"}, sampleActor); !isErr(err,
		domain.ErrMissingRoutingReason) {
		t.Fatalf("lỗi = %v, muốn ErrMissingRoutingReason", err)
	}
	if k.begun != 0 {
		t.Fatalf("đã mở %d giao dịch cho một yêu cầu sai hình dạng", k.begun)
	}
}

// --- the outgoing register ------------------------------------------------------------------------

func TestIssueOutgoing_NumberAndTrailInOneTransaction(t *testing.T) {
	// THE SAME THREE PROPERTIES AS THE INCOMING REGISTER, on the register whose number leaves the building:
	// the series is read under a lock, the number is stored with the document, and the trail shares
	// the transaction.
	k := newFakeRegisterDB()
	uc, ctx := newOutgoingUseCase(t, k, 1)

	d, err := uc.IssueNumber(ctx, sampleIssue(), sampleActor)
	if err != nil {
		t.Fatalf("IssueNumber: %v", err)
	}
	if d.IssuedNo != 1 || d.Year != 2026 {
		t.Fatalf("số/năm = %d/%d, muốn 1/2026", d.IssuedNo, d.Year)
	}
	reads := k.stmtsContaining("FROM day_so_van_ban")
	if len(reads) != 1 || !strings.Contains(reads[0].sql, "FOR UPDATE") {
		t.Fatal("câu đọc dãy số đi không có `FOR UPDATE` — số đi đã in ra giấy, đóng dấu và gửi ra " +
			"ngoài xã; hai văn bản cùng số là hai văn bản không ai phân biệt được")
	}
	// THE SERIES IS `di`, NOT `den`. Two registers, two independent counters — văn bản đến số 7 and
	// văn bản đi số 7 are two different documents in the same commune in the same year.
	if !hasValue(reads[0].args, string(docstore.RegisterOutgoing)) {
		t.Fatalf("đọc dãy số với sổ %v, muốn %q", reads[0].args, docstore.RegisterOutgoing)
	}
	if k.begun != 1 || k.committed != 1 {
		t.Fatalf("begin=%d commit=%d, muốn 1/1", k.begun, k.committed)
	}
	trail := k.stmtsContaining("INSERT INTO audit_log")
	if len(trail) != 1 || !hasValue(trail[0].args, domain.OutgoingDocumentCode(d.Year, d.IssuedNo)) {
		t.Fatalf("vết không mang mã nghiệp vụ %q: %v", domain.OutgoingDocumentCode(d.Year, d.IssuedNo), trail)
	}
}

func TestIssueOutgoing_SeriesIndependentOfIncoming(t *testing.T) {
	// ONE COUNTER TABLE, TWO SERIES, told apart by `so_sach` INSIDE the statement rather than by
	// which Go object issued them. A commune's số đến 1 and số đi 1 both exist in 2026 and both are
	// correct; a shared counter would make the outgoing register start at whatever the incoming one
	// had reached, and every outgoing number would then be wrong on paper.
	k := newFakeRegisterDB()
	incoming, _, ctx := newIncomingUseCase(t, k, 1)
	outgoing, _ := newOutgoingUseCase(t, k, 1)

	for i := 0; i < 3; i++ {
		if _, err := incoming.Register(ctx, sampleRegistration(), sampleActor); err != nil {
			t.Fatalf("Register %d: %v", i, err)
		}
	}
	out, err := outgoing.IssueNumber(ctx, sampleIssue(), sampleActor)
	if err != nil {
		t.Fatalf("IssueNumber: %v", err)
	}
	if out.IssuedNo != 1 {
		t.Fatalf("số đi đầu tiên = %d, muốn 1 — sổ đi có dãy riêng, không nối tiếp sổ đến", out.IssuedNo)
	}
	if k.lastNumber["den|2026"] != 3 || k.lastNumber["di|2026"] != 1 {
		t.Fatalf("dãy đến=%d đi=%d, muốn 3 và 1", k.lastNumber["den|2026"], k.lastNumber["di|2026"])
	}
}

// --- the edit path --------------------------------------------------------------------------------

func TestUpdate_NoChangeWritesNothingAndNoTrail(t *testing.T) {
	// A NO-OP IS NOT AN EVENT. Recording it would fill a public authority's ledger with entries
	// saying nothing changed, and those are the entries that bury the ones carrying legal weight. It
	// is also what makes the route's `idem.KhongCan` declaration true rather than hopeful.
	k := newFakeRegisterDB()
	k.incoming = &incomingRow{
		id: "vbd-001", arrivalNo: 7, year: 2026,
		receivedDate: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		issuingBody:  "Huyện uỷ", documentType: "cong-van", summary: "Về việc rà soát hộ nghèo",
		due: identityDue, status: string(domain.IncomingStatusRegistered), createdBy: "CB-1",
	}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	same := "Về việc rà soát hộ nghèo"
	if _, err := uc.Update(ctx, "vbd-001", UpdateIncomingDocumentRequest{Summary: &same}, sampleActor); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if k.hasStmt("UPDATE van_ban_den") {
		t.Fatal("không có gì đổi mà vẫn chạy UPDATE")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("không có gì đổi mà vẫn ghi vết")
	}
}

func TestUpdate_NeverTouchesNumberYearStatusOrDeadline(t *testing.T) {
	// THE FOUR FACTS AN EDIT MAY NEVER MOVE, checked in the SQL rather than in a comment: the issued
	// number and its year (rule 7), the state (it moves by routing alone) and the deadline (rule 10,
	// invariant 2 — a commitment already made).
	k := newFakeRegisterDB()
	k.incoming = &incomingRow{
		id: "vbd-001", arrivalNo: 7, year: 2026,
		receivedDate: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		issuingBody:  "Huyện uỷ", documentType: "cong-van", summary: "cũ",
		due: identityDue, status: string(domain.IncomingStatusRegistered), createdBy: "CB-1",
	}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	changed := "Trích yếu đã sửa"
	if _, err := uc.Update(ctx, "vbd-001", UpdateIncomingDocumentRequest{Summary: &changed}, sampleActor); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updates := k.stmtsContaining("UPDATE van_ban_den")
	if len(updates) != 1 {
		t.Fatalf("chạy %d câu UPDATE, muốn 1", len(updates))
	}
	for _, col := range []string{"so_vao_so", "nam =", "trang_thai", "han_xu_ly_xong"} {
		if strings.Contains(updates[0].sql, col) {
			t.Errorf("câu sửa có chạm vào %q — không được:\n%s", col, updates[0].sql)
		}
	}
}

// --- helpers ---------------------------------------------------------------------------------------

// errTest stands in for any failure identity can return — no `sla` row, no working calendar, the
// service unreachable, the wrong caller key. The use case must answer the same way to all of them,
// so the test needs only one.
var errTest = errors.New("thử nghiệm: identity không trả lời")

// isErr is errors.Is, named so the assertions below read as sentences.
func isErr(err, want error) bool { return errors.Is(err, want) }

// hasValue reports whether one bound parameter equals `want`. Comparing VALUES rather than argument
// positions, so adding a column to a statement does not turn every assertion red for no reason.
func hasValue(args []driver.Value, want any) bool {
	for _, a := range args {
		if a == want {
			return true
		}
		if t, ok := a.(time.Time); ok {
			if m, ok2 := want.(time.Time); ok2 && t.Equal(m) {
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

func sampleIncomingRow() *incomingRow {
	return &incomingRow{
		id: "vbd-001", arrivalNo: 7, year: 2026, receivedDate: registeredAt, issuingBody: "Huyện uỷ",
		documentType: "cong-van", summary: "x", due: identityDue, status: string(domain.IncomingStatusAssigned),
		createdBy: "CB-00001",
	}
}

func sampleRoutingRow(id string, at time.Time) []driver.Value {
	return []driver.Value{id, "vbd-001", at, "CB-00002", string(domain.IncomingStatusInProgress),
		"bp-van-phong", "bp-dia-chinh", "CB-00003", "Thuộc thẩm quyền Địa chính", at}
}

// assertNoWrites asserts a read wrote nothing — no business write, no audit entry. Reading a register
// that holds no masked citizen field, inside one commune, is not audited (rule 6, invariant 7).
func assertNoWrites(t *testing.T, k *fakeRegisterDB) {
	t.Helper()
	for _, l := range k.stmts {
		s := strings.ToUpper(l.sql)
		if strings.Contains(s, "INSERT ") || strings.Contains(s, "UPDATE ") || strings.Contains(s, "DELETE ") {
			t.Fatalf("tuyến đọc phát ra câu ghi:\n%s", l.sql)
		}
	}
}

func TestDetail_ReadsByTenantSkipsRemovedAndDoesNotLock(t *testing.T) {
	// THE PREDICATE IS THE ISOLATION: `tenant_id = $1` bound from the context (rule 1) and
	// `deleted_at IS NULL` (rule 7, invariant 2). Without either, the 404 the handler promises for
	// "another commune's" and "removed" would be a 200. And NO `FOR UPDATE`: opening a drawer must not
	// queue a routing behind it.
	k := newFakeRegisterDB()
	k.incoming = sampleIncomingRow()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	d, err := uc.Detail(ctx, "vbd-001")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.ID != "vbd-001" || d.ArrivalNo != 7 {
		t.Fatalf("đọc sai dòng: %+v", d)
	}
	reads := k.stmtsContaining("FROM van_ban_den")
	if len(reads) != 1 {
		t.Fatalf("có %d câu đọc văn bản, muốn 1", len(reads))
	}
	for _, want := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL"} {
		if !strings.Contains(reads[0].sql, want) {
			t.Errorf("câu đọc thiếu %q:\n%s", want, reads[0].sql)
		}
	}
	if strings.Contains(reads[0].sql, "FOR UPDATE") {
		t.Error("câu đọc chi tiết khoá dòng — mở ngăn chi tiết không được chặn việc chuyển")
	}
	if reads[0].args[0] != string(tenantA) {
		t.Errorf("$1 = %v, muốn xã của ngữ cảnh %q", reads[0].args[0], tenantA)
	}
	assertNoWrites(t, k)
}

func TestDetail_NotFoundIsOneSentinel(t *testing.T) {
	// No row — whether it never existed, was removed, or is another commune's — is ONE sentinel. The
	// handler maps it to one 404 body.
	k := newFakeRegisterDB()
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	for _, id := range []string{"vbd-khong-co", ""} {
		if _, err := uc.Detail(ctx, id); !errors.Is(err, docstore.ErrIncomingDocumentNotFound) {
			t.Fatalf("id %q: lỗi = %v, muốn ErrIncomingDocumentNotFound", id, err)
		}
	}
}

func TestRoutingHistory_ReadsDocumentFirstThenOldestFirst(t *testing.T) {
	// THE DOCUMENT IS READ FIRST, in the same transaction — the timeline table has no soft-delete
	// column, so that read is what keeps a removed document's history off the wire. Then the rows,
	// OLDEST FIRST, bounded by the ceiling plus one.
	k := newFakeRegisterDB()
	k.incoming = sampleIncomingRow()
	older := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	k.routingRows = [][]driver.Value{sampleRoutingRow("ls-1", older), sampleRoutingRow("ls-2", older.Add(time.Hour))}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	history, err := uc.RoutingHistory(ctx, "vbd-001")
	if err != nil {
		t.Fatalf("RoutingHistory: %v", err)
	}
	if len(history) != 2 || history[0].ID != "ls-1" || history[1].ID != "ls-2" {
		t.Fatalf("lịch sử = %+v, muốn ls-1 rồi ls-2", history)
	}
	if history[1].FromOrgUnitID != "bp-van-phong" || history[1].ToOrgUnitID != "bp-dia-chinh" ||
		history[1].AssigneeCode != "CB-00003" || history[1].StatusAtTime != domain.IncomingStatusInProgress {
		t.Fatalf("cột đọc lệch vị trí: %+v", history[1])
	}
	if k.begun != 1 || k.committed != 1 {
		t.Fatalf("begin=%d commit=%d, muốn 1/1 — hai câu đọc trong MỘT giao dịch", k.begun, k.committed)
	}

	docRead := -1
	for i, l := range k.stmts {
		if strings.Contains(l.sql, "FROM van_ban_den") && docRead == -1 {
			docRead = i
		}
		if strings.Contains(l.sql, "FROM lich_su_chuyen_van_ban") && docRead == -1 {
			t.Fatal("đọc lịch sử TRƯỚC khi kiểm văn bản còn thấy được")
		}
	}
	reads := k.stmtsContaining("FROM lich_su_chuyen_van_ban")
	if len(reads) != 1 {
		t.Fatalf("có %d câu đọc lịch sử, muốn 1", len(reads))
	}
	for _, want := range []string{"tenant_id = $1", "van_ban_den_id = $2", "ORDER BY thoi_diem ASC, id ASC", "LIMIT $3"} {
		if !strings.Contains(reads[0].sql, want) {
			t.Errorf("câu đọc lịch sử thiếu %q:\n%s", want, reads[0].sql)
		}
	}
	if reads[0].args[0] != string(tenantA) || reads[0].args[1] != "vbd-001" ||
		reads[0].args[2] != int64(docstore.MaxRoutingHistory+1) {
		t.Errorf("tham số = %v, muốn [%s vbd-001 %d]", reads[0].args, tenantA, docstore.MaxRoutingHistory+1)
	}
	assertNoWrites(t, k)
}

func TestRoutingHistory_UnseenDocumentMeansNoHistoryRead(t *testing.T) {
	// Removed / another commune's / unknown: the same sentinel as Detail, and the timeline is never
	// even queried.
	k := newFakeRegisterDB()
	k.routingRows = [][]driver.Value{sampleRoutingRow("ls-1", registeredAt)}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	if _, err := uc.RoutingHistory(ctx, "vbd-001"); !errors.Is(err, docstore.ErrIncomingDocumentNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrIncomingDocumentNotFound", err)
	}
	if k.hasStmt("FROM lich_su_chuyen_van_ban") {
		t.Fatal("văn bản không thấy được mà vẫn đọc lịch sử chuyển của nó")
	}
}

func TestRoutingHistory_OverTheCeilingRefusesRatherThanTruncates(t *testing.T) {
	k := newFakeRegisterDB()
	k.incoming = sampleIncomingRow()
	for i := 0; i <= docstore.MaxRoutingHistory; i++ {
		k.routingRows = append(k.routingRows, sampleRoutingRow("ls", registeredAt))
	}
	uc, _, ctx := newIncomingUseCase(t, k, 1)

	history, err := uc.RoutingHistory(ctx, "vbd-001")
	if !errors.Is(err, docstore.ErrTooMuchRoutingHistory) {
		t.Fatalf("lỗi = %v, muốn ErrTooMuchRoutingHistory", err)
	}
	if history != nil {
		t.Fatal("vượt trần mà vẫn trả một danh sách — lịch sử cụt đọc như lịch sử đủ")
	}
}
