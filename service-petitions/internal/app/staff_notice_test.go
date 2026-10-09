package app

// Tests for the staff-notice outbox writes of ADR 0086 (kinds 18–21, 23, 24), over the REAL stores on
// the fake drivers.
//
//	PROVED HERE   each act writes its outbox row INSIDE its own transaction, after the audit entry · the
//	              row rolls back with the act when it fails, and the act rolls back with it · the actor
//	              is never a recipient, and an empty list writes no row · the key is `<kind>:<id of the
//	              row the act wrote>` · the payload is a StaffNotification comms accepts · the
//	              non-trigger cases (a status move not into `cho-duyet`, a unit-only petition assignment,
//	              a rating that does not reopen) write nothing · the extension REASON and the citizen's
//	              rating COMMENT never reach a notice · mentions are checked with identity before
//	              anything is written.
//
//	NOT PROVED    the partitioned table and its UNIQUE key (migration 0036 text test; PostgreSQL is not
//	              reachable here) · delivery (staff_notice_relay_test.go).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

const outboxInsert = "INSERT INTO staff_notice_outbox"

// outboxRow is one recorded outbox INSERT, decoded. Args: tenant, id, name, key, payload, occurred_at.
type outboxRow struct {
	tenant, id, name, key string
	n                     *commsv1.StaffNotification
	inTx                  bool
}

func decodeOutbox(t *testing.T, ls []lenhPhieu) []outboxRow {
	t.Helper()
	var out []outboxRow
	for _, l := range ls {
		var n commsv1.StaffNotification
		raw, ok := l.args[4].([]byte)
		if !ok {
			t.Fatalf("payload không phải []byte: %T", l.args[4])
		}
		if err := protojson.Unmarshal(raw, &n); err != nil {
			t.Fatalf("payload không đọc được: %v", err)
		}
		out = append(out, outboxRow{tenant: l.args[0].(string), id: l.args[1].(string), name: l.args[2].(string),
			key: l.args[3].(string), n: &n, inTx: l.trongGiaoDich})
	}
	return out
}

// oneOutboxRow returns the single outbox row of a task act, failing unless there is exactly one, and
// checks it was written inside the transaction, in the commune of the context, after the audit entry.
func oneOutboxRow(t *testing.T, k *khoNhiemVuGia) outboxRow {
	t.Helper()
	rows := decodeOutbox(t, k.cau(outboxInsert))
	if len(rows) != 1 {
		t.Fatalf("ghi %d dòng outbox, muốn 1", len(rows))
	}
	if !rows[0].inTx {
		t.Error("dòng outbox ghi NGOÀI giao dịch của hành vi")
	}
	if rows[0].tenant != string(xaThu) {
		t.Errorf("dòng outbox ở xã %q, muốn %q", rows[0].tenant, xaThu)
	}
	auditAt, outboxAt := -1, -1
	for i, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT INTO audit_log") {
			auditAt = i
		}
		if strings.HasPrefix(l.sql, outboxInsert) {
			outboxAt = i
		}
	}
	if auditAt < 0 || outboxAt < auditAt {
		t.Errorf("dòng outbox (%d) không đứng sau vết kiểm toán (%d)", outboxAt, auditAt)
	}
	return rows[0]
}

func lastTaskLogID(t *testing.T, k *khoNhiemVuGia) string {
	t.Helper()
	ls := k.cau("INSERT INTO nhat_ky_nhiem_vu")
	if len(ls) == 0 {
		t.Fatal("không có dòng nhật ký nhiệm vụ")
	}
	return ls[len(ls)-1].args[1].(string)
}

type holdersFake struct {
	holders map[string][]string
	err     error
	asked   [][]string
	keys    []string
}

func (f *holdersFake) OrgUnitPermissionHolders(_ context.Context, units []string, key string) (map[string][]string, error) {
	f.asked = append(f.asked, append([]string(nil), units...))
	f.keys = append(f.keys, key)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string][]string{}
	for _, u := range units {
		if c, ok := f.holders[u]; ok {
			out[u] = c
		}
	}
	return out, nil
}

// --- writeStaffNotice itself ----------------------------------------------------------------------

type outboxFake struct {
	rows []petstore.StaffNoticeRow
	err  error
}

func (f *outboxFake) Record(_ context.Context, _ *store.ScopedTx, r petstore.StaffNoticeRow) error {
	if f.err != nil {
		return f.err
	}
	f.rows = append(f.rows, r)
	return nil
}

func seqIDs() func() (string, error) {
	n := 0
	return func() (string, error) {
		n++
		return "01JOUTBOX" + strings.Repeat("0", 16) + string(rune('0'+n)), nil
	}
}

func TestWriteStaffNoticeDropsActor(t *testing.T) {
	out := &outboxFake{}
	n := domain.ActNotice{Kind: domain.ActNoticeTaskMention, ActRowID: "nk-1", Recipients: []string{"CB-1", "CB-1"},
		Title: "Bạn được nhắc trong nhiệm vụ: X"}
	if err := writeStaffNotice(context.Background(), nil, out, seqIDs(), n, "CB-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(out.rows) != 0 {
		t.Fatalf("người bấm nút nhận tin của chính mình: %d dòng", len(out.rows))
	}
	// Nobody left and NO outbox wired: still no error — nothing was owed.
	if err := writeStaffNotice(context.Background(), nil, nil, seqIDs(), n, "CB-1", time.Now()); err != nil {
		t.Errorf("danh sách rỗng mà vẫn đòi outbox: %v", err)
	}
}

func TestWriteStaffNoticePayloadAndKey(t *testing.T) {
	out := &outboxFake{}
	at := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	n := domain.ActNotice{Kind: domain.ActNoticePetitionAssigned, ActRowID: "01JNK", Recipients: []string{"CB-9", "CB-2"},
		Title: "Bạn được giao xử lý phản ánh PA-XYZ", Link: "/phan-anh?q=PA-XYZ"}
	if err := writeStaffNotice(context.Background(), nil, out, seqIDs(), n, "CB-ACTOR", at); err != nil {
		t.Fatal(err)
	}
	if len(out.rows) != 1 {
		t.Fatalf("%d dòng, muốn 1", len(out.rows))
	}
	r := out.rows[0]
	if r.Name != "petitions.assigned.v1" || r.IdempotencyKey != "phan-anh.phan-cong:01JNK" || !r.OccurredAt.Equal(at) {
		t.Errorf("dòng = %+v", r)
	}
	var m commsv1.StaffNotification
	if err := protojson.Unmarshal(r.Payload, &m); err != nil {
		t.Fatal(err)
	}
	if m.GetKind() != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ASSIGNED ||
		m.GetIdempotencyKey() != r.IdempotencyKey || strings.Join(m.GetRecipientMa(), ",") != "CB-2,CB-9" ||
		m.GetLink() != "/phan-anh?q=PA-XYZ" {
		t.Errorf("payload = %v", &m)
	}
}

func TestWriteStaffNoticeSplitsOver200Recipients(t *testing.T) {
	out := &outboxFake{}
	var codes []string
	for i := 0; i < 250; i++ {
		codes = append(codes, "CB-"+string(rune('A'+i/26))+string(rune('a'+i%26)))
	}
	n := domain.ActNotice{Kind: domain.ActNoticeTaskAssigned, ActRowID: "nk-9", Recipients: codes, Title: "t"}
	if err := writeStaffNotice(context.Background(), nil, out, seqIDs(), n, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(out.rows) != 2 || out.rows[0].IdempotencyKey != "nhiem-vu.giao-moi:nk-9" ||
		out.rows[1].IdempotencyKey != "nhiem-vu.giao-moi:nk-9:2" {
		t.Fatalf("chia = %d dòng %v", len(out.rows), out.rows)
	}
}

func TestWriteStaffNoticeRefusesWiringAndContractFaults(t *testing.T) {
	ok := domain.ActNotice{Kind: domain.ActNoticeTaskAssigned, ActRowID: "nk", Recipients: []string{"CB-1"}, Title: "t"}
	if err := writeStaffNotice(context.Background(), nil, nil, seqIDs(), ok, "", time.Now()); err == nil {
		t.Error("không có outbox mà vẫn để hành vi qua — tin mất không dấu vết")
	}
	bad := ok
	bad.Kind = "khong-biet"
	if err := writeStaffNotice(context.Background(), nil, &outboxFake{}, seqIDs(), bad, "", time.Now()); err == nil {
		t.Error("loại không biết được ghi")
	}
	noRow := ok
	noRow.ActRowID = ""
	if err := writeStaffNotice(context.Background(), nil, &outboxFake{}, seqIDs(), noRow, "", time.Now()); err == nil {
		t.Error("không có dòng hành vi mà vẫn ghi — khoá không lặp lại được")
	}
	sentinel := errors.New("ghi hỏng")
	if err := writeStaffNotice(context.Background(), nil, &outboxFake{err: sentinel}, seqIDs(), ok, "", time.Now()); !errors.Is(err, sentinel) {
		t.Errorf("lỗi ghi outbox bị nuốt: %v", err)
	}
}

// --- kind 18: task created / handed over ----------------------------------------------------------

func TestCreateTaskNotifiesNamedAssigneeInSameTransaction(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	req := taoMau()
	req.NguoiThucHienMa = maNguoiThucHien

	if _, err := uc.Tao(ctx, req, canBoThu()); err != nil {
		t.Fatalf("giao việc: %v", err)
	}
	chiGhiTrongGiaoDich(t, k)
	r := oneOutboxRow(t, k)
	if r.key != "nhiem-vu.giao-moi:"+lastTaskLogID(t, k) || r.name != "tasks.assigned.v1" {
		t.Errorf("khoá/tên = %q / %q", r.key, r.name)
	}
	if r.n.GetKind() != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ASSIGNED ||
		strings.Join(r.n.GetRecipientMa(), ",") != maNguoiThucHien {
		t.Errorf("payload = %v", r.n)
	}
	if r.n.GetTitle() != "Bạn được giao nhiệm vụ: "+req.TieuDe || r.n.GetBody() != "Hạn 15/07/2026" ||
		r.n.GetLink() != "/nhiem-vu?q=NV19" {
		t.Errorf("câu chữ = %q / %q / %q", r.n.GetTitle(), r.n.GetBody(), r.n.GetLink())
	}
}

func TestCreateTaskSelfAssignmentWritesNothing(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	req := taoMau()
	req.NguoiThucHienMa = maCanBoThu // the actor

	if _, err := uc.Tao(ctx, req, canBoThu()); err != nil {
		t.Fatalf("giao việc: %v", err)
	}
	if k.coCau(outboxInsert) {
		t.Error("người tự giao việc cho mình nhận thông báo")
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestCreateTaskUnitOnlyNotifiesUnitAssigners(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	h := &holdersFake{holders: map[string][]string{"bp-dia-chinh": {"CB-HEAD", maCanBoThu}}}
	uc.WithUnitHolders(h)
	req := taoMau()
	req.BoPhanID = "bp-dia-chinh"

	if _, err := uc.Tao(ctx, req, canBoThu()); err != nil {
		t.Fatalf("giao việc: %v", err)
	}
	if len(h.keys) != 1 || h.keys[0] != permTaskAssign || h.asked[0][0] != "bp-dia-chinh" {
		t.Errorf("hỏi identity %v / %v", h.asked, h.keys)
	}
	r := oneOutboxRow(t, k)
	// The actor holds task.assign in that unit too — dropped.
	if strings.Join(r.n.GetRecipientMa(), ",") != "CB-HEAD" {
		t.Errorf("người nhận = %v", r.n.GetRecipientMa())
	}
}

func TestCreateTaskUnitHoldersOutageKeepsAct(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.WithUnitHolders(&holdersFake{err: errors.New("identity xuống")})
	req := taoMau()
	req.BoPhanID = "bp-dia-chinh"

	if _, err := uc.Tao(ctx, req, canBoThu()); err != nil {
		t.Fatalf("giao việc hỏng vì không lấy được người nhận thông báo: %v", err)
	}
	if k.coCau(outboxInsert) {
		t.Error("ghi thông báo không người nhận")
	}
}

func TestCreateTaskOutboxFailureRollsBack(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	k.loiSau = outboxInsert
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	req := taoMau()
	req.NguoiThucHienMa = maNguoiThucHien

	if _, err := uc.Tao(ctx, req, canBoThu()); err == nil {
		t.Fatal("outbox hỏng mà nhiệm vụ vẫn được giao — người được giao sẽ không bao giờ biết")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestReassignKeyedByHandoverRow(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	assignee := "CB-00555"

	if _, err := uc.Reassign(ctx, maNVGoc, TaskAssignmentRequest{Change: domain.TaskAssignmentChange{Assignee: &assignee}},
		canBoThu()); err != nil {
		t.Fatalf("giao lại: %v", err)
	}
	r := oneOutboxRow(t, k)
	if r.key != "nhiem-vu.giao-moi:"+lastTaskLogID(t, k) || strings.Join(r.n.GetRecipientMa(), ",") != assignee {
		t.Errorf("khoá/người nhận = %q / %v", r.key, r.n.GetRecipientMa())
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestReassignClearedAssigneeAsksCurrentUnit(t *testing.T) {
	k := khoNVMau() // the fixture task sits in bp-vpdu
	uc, ctx := dungGhiNhiemVu(t, k)
	h := &holdersFake{holders: map[string][]string{"bp-vpdu": {"CB-HEAD"}}}
	uc.WithUnitHolders(h)
	cleared := ""

	if _, err := uc.Reassign(ctx, maNVGoc, TaskAssignmentRequest{Change: domain.TaskAssignmentChange{Assignee: &cleared}},
		canBoThu()); err != nil {
		t.Fatalf("giao lại: %v", err)
	}
	if len(h.asked) != 1 || h.asked[0][0] != "bp-vpdu" {
		t.Errorf("hỏi identity về %v, muốn [bp-vpdu]", h.asked)
	}
	rows := decodeOutbox(t, k.cau(outboxInsert))
	if len(rows) != 1 || strings.Join(rows[0].n.GetRecipientMa(), ",") != "CB-HEAD" {
		t.Errorf("dòng outbox = %v", rows)
	}
}

// --- kind 20: sent for review ---------------------------------------------------------------------

func TestStatusMoveIntoReviewNotifiesLeader(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)},
		staffActor(maNguoiThucHien), false, false); err != nil {
		t.Fatalf("chuyển chờ duyệt: %v", err)
	}
	r := oneOutboxRow(t, k)
	if r.key != "nhiem-vu.cho-duyet:"+lastTaskLogID(t, k) || strings.Join(r.n.GetRecipientMa(), ",") != maLanhDao ||
		r.n.GetKind() != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_APPROVAL_REQUESTED {
		t.Errorf("dòng = %q %v %v", r.key, r.n.GetRecipientMa(), r.n.GetKind())
	}
}

func TestStatusMoveWithoutLeaderNotifiesCreator(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["lanh_dao_giao_viec_ma"] = nil
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)},
		staffActor(maNguoiThucHien), false, false); err != nil {
		t.Fatalf("chuyển chờ duyệt: %v", err)
	}
	if r := oneOutboxRow(t, k); strings.Join(r.n.GetRecipientMa(), ",") != "CB-00123" {
		t.Errorf("người nhận = %v, muốn người tạo", r.n.GetRecipientMa())
	}
}

func TestStatusMoveElsewhereWritesNothing(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.TamDung)},
		staffActor(maNguoiThucHien), false, false); err != nil {
		t.Fatalf("tạm dừng: %v", err)
	}
	if k.coCau(outboxInsert) {
		t.Error("tạm dừng mà báo chờ duyệt")
	}
}

// --- kind 19: extension requested -----------------------------------------------------------------

func TestExtensionRequestCarriesDateNeverReason(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	const reason = "Chờ số liệu của ông Nguyễn Văn A ở thôn 3."

	req, err := uc.DeNghiLuiHan(ctx, maNVGoc, YeuCauDeNghiLuiHan{HanMoi: mocHanMoiNV, LyDo: reason},
		staffActor(maNguoiThucHien))
	if err != nil {
		t.Fatalf("đề nghị lùi hạn: %v", err)
	}
	r := oneOutboxRow(t, k)
	if r.key != "nhiem-vu.de-nghi-lui-han:"+req.ID || strings.Join(r.n.GetRecipientMa(), ",") != maLanhDao {
		t.Errorf("khoá/người nhận = %q / %v", r.key, r.n.GetRecipientMa())
	}
	if r.n.GetBody() != "Đề nghị hạn mới 20/08/2026" {
		t.Errorf("nội dung = %q", r.n.GetBody())
	}
	if strings.Contains(r.n.GetTitle()+r.n.GetBody(), "Nguyễn") {
		t.Error("lý do lùi hạn lọt vào thông báo (ADR 0086 điều kiện dừng #3)")
	}
}

func TestExtensionRequestByLeaderWritesNothing(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.DeNghiLuiHan(ctx, maNVGoc, YeuCauDeNghiLuiHan{HanMoi: mocHanMoiNV, LyDo: "Chờ số liệu."},
		staffActor(maLanhDao)); err != nil {
		t.Fatalf("đề nghị lùi hạn: %v", err)
	}
	if k.coCau(outboxInsert) {
		t.Error("lãnh đạo tự đề nghị mà tự nhận tin")
	}
}

// --- kind 21: mentions ----------------------------------------------------------------------------

func TestLogEntryMentionSkipsAuthor(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	gv := &giaoViecGia{duocTatCa: true}
	uc.giaoViec = gv

	row, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, []string{" " + maLanhDao + " ", maNguoiThucHien, maLanhDao},
		staffActor(maNguoiThucHien), false)
	if err != nil {
		t.Fatalf("ghi nhật ký: %v", err)
	}
	// Asked once, trimmed, de-duplicated, in the context's commune.
	if gv.goi != 1 || strings.Join(gv.daHoi[0], ",") != maLanhDao+","+maNguoiThucHien || gv.xa[0] != xaThu {
		t.Errorf("identity được hỏi %v trong %v", gv.daHoi, gv.xa)
	}
	r := oneOutboxRow(t, k)
	if r.key != "nhiem-vu.nhac-ten:"+row.ID || strings.Join(r.n.GetRecipientMa(), ",") != maLanhDao {
		t.Errorf("khoá/người nhận = %q / %v", r.key, r.n.GetRecipientMa())
	}
	if r.n.GetBody() != logText {
		t.Errorf("nội dung = %q", r.n.GetBody())
	}
	if d := auditDeltaText(t, k); !strings.Contains(d, `"nhac_ten":["`+maLanhDao+`","`+maNguoiThucHien+`"]`) {
		t.Errorf("vết không ghi người được nhắc: %s", d)
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestLogEntryMentionRefusals(t *testing.T) {
	for _, c := range []struct {
		name     string
		gv       *giaoViecGia
		mentions []string
		want     error
	}{
		{"mã sai dạng", &giaoViecGia{duocTatCa: true}, []string{"CB 1"}, domain.ErrMentionInvalid},
		{"mã rỗng", &giaoViecGia{duocTatCa: true}, []string{"  "}, domain.ErrMentionInvalid},
		{"không phải cán bộ của xã", &giaoViecGia{duoc: map[string]struct{}{}}, []string{"CB-OTHER"}, ErrMentionStaffInvalid},
		{"identity xuống", &giaoViecGia{loi: errors.New("unavailable")}, []string{"CB-1"}, ErrMentionStaffUnchecked},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			uc.giaoViec = c.gv
			_, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, c.mentions, staffActor(maNguoiThucHien), false)
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			khongGhiGi(t, k)
		})
	}
}

func TestLogEntryTooManyMentions(t *testing.T) {
	var codes []string
	for i := 0; i <= domain.MentionsMax; i++ {
		codes = append(codes, "CB-"+string(rune('A'+i)))
	}
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	if _, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, codes, staffActor(maNguoiThucHien), false); !errors.Is(err, domain.ErrMentionsTooMany) {
		t.Fatalf("lỗi = %v", err)
	}
}

func TestLogEntryWithoutMentionsAsksNobody(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	gv := &giaoViecGia{loi: errors.New("không được gọi")}
	uc.giaoViec = gv
	if _, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, nil, staffActor(maNguoiThucHien), false); err != nil {
		t.Fatal(err)
	}
	if gv.goi != 0 || k.coCau(outboxInsert) {
		t.Errorf("không nhắc ai mà hỏi identity %d lần / ghi outbox", gv.goi)
	}
}

// --- kind 23: petition assigned -------------------------------------------------------------------

func TestPetitionAssignmentNotifiesOfficer(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.giaoViec = &giaoViecGia{duocTatCa: true}

	if _, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: "CB-00999"},
		canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("phân công: %v", err)
	}
	rows := decodeOutbox(t, k.cau(outboxInsert))
	logs := k.cau("INSERT INTO nhat_ky_phan_anh")
	if len(rows) != 1 || len(logs) != 1 {
		t.Fatalf("outbox=%d nhật ký=%d, muốn 1/1", len(rows), len(logs))
	}
	r := rows[0]
	if !r.inTx || k.daCommit != 1 {
		t.Error("dòng outbox không cùng giao dịch")
	}
	if r.key != "phan-anh.phan-cong:"+logs[0].args[1].(string) || strings.Join(r.n.GetRecipientMa(), ",") != "CB-00999" ||
		r.n.GetKind() != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ASSIGNED {
		t.Errorf("dòng = %q %v %v", r.key, r.n.GetRecipientMa(), r.n.GetKind())
	}
	if !strings.Contains(r.n.GetTitle(), maPhieuThu) {
		t.Errorf("tiêu đề không mang mã tra cứu: %q", r.n.GetTitle())
	}
}

func TestPetitionAssignmentUnitOnlyWritesNothing(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("phân công: %v", err)
	}
	if k.coCau(outboxInsert) {
		t.Error("phân công chỉ bộ phận mà ghi thông báo")
	}
}

func TestPetitionAssignmentOutboxFailureRollsBack(t *testing.T) {
	k := khoPhieuMau()
	phieuChoPhanCong(k)
	k.loiSau = outboxInsert
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	if _, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: "CB-00999"},
		canBoThu(), khongQuyenHanChe); err == nil {
		t.Fatal("outbox hỏng mà phân công vẫn qua")
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần", k.daCommit)
	}
}

// --- kind 24: reopened by a low rating ------------------------------------------------------------

func TestRatingReopenNotifiesHolderWithoutComment(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, map[string]any{"can_bo_xu_ly_id": "CB-00999"}))
	uc, ctx := buildRating(t, k)

	if _, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 1, Comment: fixtureComment}, citizenActor()); err != nil {
		t.Fatalf("đánh giá: %v", err)
	}
	rows := decodeOutbox(t, k.cau(outboxInsert))
	logs := k.cau("INSERT INTO nhat_ky_phan_anh")
	if len(rows) != 1 || len(logs) != 1 {
		t.Fatalf("outbox=%d nhật ký=%d", len(rows), len(logs))
	}
	r := rows[0]
	if r.key != "phan-anh.mo-lai:"+logs[0].args[1].(string) || strings.Join(r.n.GetRecipientMa(), ",") != "CB-00999" ||
		!r.inTx || k.daCommit != 1 {
		t.Errorf("dòng = %q %v trong=%v commit=%d", r.key, r.n.GetRecipientMa(), r.inTx, k.daCommit)
	}
	if strings.Contains(r.n.GetTitle()+r.n.GetBody(), fixtureComment) {
		t.Error("lời nhận xét của người dân lọt vào thông báo cán bộ")
	}
}

func TestRatingWithoutReopenOrHolderWritesNothing(t *testing.T) {
	for _, c := range []struct {
		name  string
		stars int
		extra map[string]any
	}{
		{"4 sao không mở lại", 4, map[string]any{"can_bo_xu_ly_id": "CB-00999"}},
		{"mở lại nhưng phiếu chỉ giao bộ phận", 2, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, c.extra))
			uc, ctx := buildRating(t, k)
			if _, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: c.stars}, citizenActor()); err != nil {
				t.Fatalf("đánh giá: %v", err)
			}
			if k.coCau(outboxInsert) {
				t.Error("ghi thông báo cán bộ khi không có ai để báo hoặc không mở lại")
			}
		})
	}
}
