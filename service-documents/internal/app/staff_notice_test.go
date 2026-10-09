package app

// The `van-ban.chuyen-toi` outbox (ADR 0086 A1–A3) — the write half over the REAL stores on the fake
// driver (driver_gia_van_ban_test.go), so "the outbox row is in the act's transaction" is a statement
// about real statements inside a real transaction boundary — and the relay over fakes.
//
// WHAT IS PROVEN:
//   - routing to a named officer writes ONE outbox row in the routing's transaction; it commits with
//     the act and rolls back with it;
//   - the actor is dropped; no officer, or the actor routing to themselves → no row;
//   - the key is `van-ban.chuyen-toi:<the act's row id>` and the relay resends exactly that key;
//   - a citizen letter's notice carries no body at all, and neither notice carries `trich_yeu` / summary;
//   - the relay's status-code classes: OK → delivered, UNAVAILABLE → owed + backoff, INVALID_ARGUMENT →
//     only the refused row set aside, an undecodable payload set aside, another error → owed.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const outboxInsert = "INSERT INTO staff_notice_outbox"

// outboxPayload decodes the payload argument ($5) of the i-th outbox INSERT.
func outboxPayload(t *testing.T, k *khoVBGia, i int) (*commsv1.StaffNotification, []any) {
	t.Helper()
	rows := k.cau(outboxInsert)
	if len(rows) <= i {
		t.Fatalf("có %d dòng outbox, muốn ít nhất %d", len(rows), i+1)
	}
	a := rows[i].args
	if len(a) != 6 {
		t.Fatalf("câu chèn outbox có %d tham số, muốn 6: %v", len(a), a)
	}
	var n commsv1.StaffNotification
	if err := protojson.Unmarshal(a[4].([]byte), &n); err != nil {
		t.Fatalf("payload không giải mã được: %v", err)
	}
	out := make([]any, len(a))
	for j := range a {
		out[j] = a[j]
	}
	return &n, out
}

func incomingRow() *hangVBD {
	return &hangVBD{
		id: "vbd-001", soVaoSo: 7, nam: 2026, ngayDen: lucVaoSo, coQuan: "Huyện uỷ", loai: "cong-van",
		trichYeu: "Đơn của ông Nguyễn Văn A về đất", boPhan: "bp-van-phong",
		han: hanMau, trangThai: string(domain.VanBanMoiVaoSo), nguoiTao: "CB-00001",
	}
}

func TestRoutingIncomingToOfficerWritesOneOutboxRowInTheSameTransaction(t *testing.T) {
	k := khoVBMau()
	k.hangDen = incomingRow()
	uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)

	if _, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
		DenBoPhan: "bp-dia-chinh", CanBoXuLyMa: "CB-00456", LyDo: "Thuộc thẩm quyền Địa chính",
	}, nguoiMau); err != nil {
		t.Fatalf("Chuyen: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || len(k.cau(outboxInsert)) != 1 {
		t.Fatalf("begin=%d commit=%d outbox=%d — muốn MỘT giao dịch có đúng một dòng outbox",
			k.batDau, k.daCommit, len(k.cau(outboxInsert)))
	}
	n, a := outboxPayload(t, k, 0)
	// sinhID is pinned: the timeline row's id is idMoiVanBan, and the key is built from it.
	wantKey := "van-ban.chuyen-toi:" + idMoiVanBan
	if a[0] != string(xaA) || a[2] != domain.EventIncomingDocumentAssigned || a[3] != wantKey ||
		!a[5].(time.Time).Equal(lucVaoSo) {
		t.Errorf("tham số outbox = %v", a)
	}
	if n.GetIdempotencyKey() != wantKey || n.GetKind() != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED ||
		strings.Join(n.GetRecipientMa(), ",") != "CB-00456" ||
		n.GetTitle() != "Bạn được giao xử lý văn bản đến số 7/2026" || n.GetLink() != "/van-ban" {
		t.Errorf("payload = %+v", n)
	}
	// RULE 3: the summary is free text that names a citizen here — it never enters the notice.
	if strings.Contains(n.GetTitle()+n.GetBody(), "Nguyễn") || strings.Contains(n.GetBody(), "đất") {
		t.Errorf("thông báo chứa trích yếu: %+v", n)
	}
	if n.GetBody() != "Hạn xử lý: "+domain.FormatLocalInstant(hanMau)+"." {
		t.Errorf("nội dung = %q, muốn hạn xử lý đã lưu", n.GetBody())
	}
}

func TestIncomingRoutingOutboxRollsBackWithTheAct(t *testing.T) {
	for _, failAt := range []string{"INSERT INTO audit_log", outboxInsert} {
		t.Run(failAt, func(t *testing.T) {
			k := khoVBMau()
			k.hangDen = incomingRow()
			k.loiSau = failAt
			uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)
			if _, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
				DenBoPhan: "bp-dia-chinh", CanBoXuLyMa: "CB-00456", LyDo: "chuyển",
			}, nguoiMau); err == nil {
				t.Fatal("một câu trong giao dịch hỏng mà việc chuyển vẫn thành công")
			}
			// The outbox INSERT was issued INSIDE the transaction that rolled back: neither the routing
			// nor its notice survives, and neither survives without the other.
			if !k.coCau(outboxInsert) || k.daCommit != 0 || k.daRollback != 1 {
				t.Fatalf("outbox=%v commit=%d rollback=%d", k.coCau(outboxInsert), k.daCommit, k.daRollback)
			}
		})
	}
}

func TestIncomingRoutingWritesNoOutboxRowWithoutAnotherOfficer(t *testing.T) {
	for name, officer := range map[string]string{
		"unit assigns itself":  "",
		"actor routes to self": nguoiMau.ID,
	} {
		t.Run(name, func(t *testing.T) {
			k := khoVBMau()
			k.hangDen = incomingRow()
			uc, _, ctx := dungUseCaseVanBanDen(t, k, 1)
			if _, err := uc.Chuyen(ctx, "vbd-001", YeuCauChuyenVanBan{
				DenBoPhan: "bp-dia-chinh", CanBoXuLyMa: officer, LyDo: "chuyển",
			}, nguoiMau); err != nil {
				t.Fatalf("Chuyen: %v", err)
			}
			if k.coCau(outboxInsert) || k.daCommit != 1 {
				t.Fatalf("outbox=%v commit=%d — không ai khác để báo thì không có dòng", k.coCau(outboxInsert), k.daCommit)
			}
		})
	}
}

func TestRouteLetterToOfficerNoticeHasNoBody(t *testing.T) {
	for _, typ := range []domain.LetterType{domain.LetterTypeComplaint, domain.LetterTypeDenunciation} {
		t.Run(string(typ), func(t *testing.T) {
			k := khoVBMau()
			l := letterIn("dt-1", domain.LetterStatusNew, "")
			l.Type, l.Summary = typ, "Tố cáo ông Trần Văn B nhận hối lộ"
			repo := newLetterRepo(l)
			uc, ctx := buildLetters(t, k, repo, liveDirectory())

			if _, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: "bp-dia-chinh", AssigneeCode: "CB-00777",
				Reason: "Thẩm quyền Địa chính"}, clerk); err != nil {
				t.Fatalf("Route: %v", err)
			}
			if len(k.cau(outboxInsert)) != 1 || k.daCommit != 1 {
				t.Fatalf("outbox=%d commit=%d", len(k.cau(outboxInsert)), k.daCommit)
			}
			n, a := outboxPayload(t, k, 0)
			wantKey := "van-ban.chuyen-toi:" + repo.logs[0].ID
			if a[2] != domain.EventCitizenLetterAssigned || a[3] != wantKey || n.GetIdempotencyKey() != wantKey {
				t.Errorf("tên/khoá = %v / %v, muốn khoá theo dòng nhật ký %s", a[2], a[3], wantKey)
			}
			// ADR 0078 #4: no summary, no sender, not even the type — the body is EMPTY.
			if n.GetBody() != "" || n.GetTitle() != "Bạn được giao xử lý đơn thư số 5/2026" ||
				strings.Join(n.GetRecipientMa(), ",") != "CB-00777" ||
				n.GetKind() != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED {
				t.Errorf("payload = %+v", n)
			}
			raw := string(a[4].([]byte))
			for _, banned := range []string{"Trần", "hối lộ", "Nguyễn", "0900000000", "tố cáo", "Tố cáo"} {
				if strings.Contains(raw, banned) {
					t.Errorf("payload chứa %q", banned)
				}
			}
		})
	}
}

func TestRouteLetterWithoutOfficerAndBookingWriteNoOutboxRow(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	if _, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: "bp-dia-chinh", Reason: "x"}, clerk); err != nil {
		t.Fatalf("Route: %v", err)
	}
	req := bookingRequest()
	req.HoldingUnitID = "bp-tu-phap" // booked straight to a unit: a unit is not a named officer
	if _, err := uc.Book(ctx, req, clerk); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if k.coCau(outboxInsert) {
		t.Fatal("giao cho bộ phận (không cán bộ) mà vẫn ghi outbox")
	}
}

func TestDropActorAndKey(t *testing.T) {
	if got := domain.DropActor([]string{"CB-2", "CB-1", "CB-2", "", "CB-1"}, "CB-1"); strings.Join(got, ",") != "CB-2" {
		t.Errorf("DropActor = %v", got)
	}
	n := domain.IncomingAssignedNotice(domain.VanBanDen{SoVaoSo: 3, Nam: 2027}, "row-1", "CB-9")
	if n.Key() != "van-ban.chuyen-toi:row-1" {
		t.Errorf("khoá = %q", n.Key())
	}
}

// --- the relay ---------------------------------------------------------------------------------------

type relayOutboxFake struct {
	rows      map[tenant.ID][]docstore.StaffNoticeRow
	delivered map[string]bool
	attempts  map[string]int
	failed    map[string]string
}

func newRelayOutbox() *relayOutboxFake {
	return &relayOutboxFake{rows: map[tenant.ID][]docstore.StaffNoticeRow{}, delivered: map[string]bool{},
		attempts: map[string]int{}, failed: map[string]string{}}
}

func (f *relayOutboxFake) Undelivered(ctx context.Context, limit int) ([]docstore.StaffNoticeRow, error) {
	var out []docstore.StaffNoticeRow
	for _, r := range f.rows[communeOf(ctx)] {
		if !f.delivered[r.ID] && f.failed[r.ID] == "" && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *relayOutboxFake) MarkDelivered(_ context.Context, ids []string, _ time.Time) error {
	for _, id := range ids {
		f.delivered[id] = true
	}
	return nil
}
func (f *relayOutboxFake) RecordFailedAttempt(_ context.Context, ids []string) error {
	for _, id := range ids {
		f.attempts[id]++
	}
	return nil
}
func (f *relayOutboxFake) MarkFailed(_ context.Context, ids []string, class string) error {
	for _, id := range ids {
		f.attempts[id]++
		f.failed[id] = class
	}
	return nil
}

type pendingFake struct{ ids []tenant.ID }

func (p *pendingFake) CommunesWithPendingStaffNotices(context.Context) ([]tenant.ID, error) {
	return p.ids, nil
}

// relayComms answers per call: a `refuse` key makes the WHOLE call INVALID_ARGUMENT (comms is all or
// nothing), `err` fails every call.
type relayComms struct {
	err    error
	refuse map[string]bool
	calls  []relayCall
}

type relayCall struct {
	commune tenant.ID
	keys    []string
}

func (c *relayComms) DeliverStaffNotifications(ctx context.Context, notices []commsclient.Notice) (map[string]commsclient.Delivery, error) {
	call := relayCall{commune: communeOf(ctx)}
	for _, n := range notices {
		call.keys = append(call.keys, n.IdempotencyKey)
	}
	c.calls = append(c.calls, call)
	if c.err != nil {
		return nil, c.err
	}
	out := map[string]commsclient.Delivery{}
	for _, n := range notices {
		if c.refuse[n.IdempotencyKey] {
			return nil, fmt.Errorf("commsclient: DeliverStaffNotifications: %w", status.Error(codes.InvalidArgument, "kind unknown"))
		}
		out[n.IdempotencyKey] = commsclient.Delivery{Created: 1}
	}
	return out, nil
}

func outboxRow(t *testing.T, id, key string) docstore.StaffNoticeRow {
	t.Helper()
	p, err := staffNoticeMarshal.Marshal(&commsv1.StaffNotification{IdempotencyKey: key,
		Kind: commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ASSIGNED, RecipientMa: []string{"CB-1"},
		Title: "Bạn được giao xử lý văn bản đến số 1/2026", Link: "/van-ban"})
	if err != nil {
		t.Fatal(err)
	}
	return docstore.StaffNoticeRow{ID: id, Name: domain.EventIncomingDocumentAssigned, IdempotencyKey: key, Payload: p}
}

type relayHarness struct {
	relay  *StaffNoticeRelay
	outbox *relayOutboxFake
	comms  *relayComms
	locks  *fakeLocks
	clock  time.Time
}

func newRelayHarness(t *testing.T, communes ...tenant.ID) *relayHarness {
	t.Helper()
	h := &relayHarness{outbox: newRelayOutbox(), comms: &relayComms{}, locks: &fakeLocks{}, clock: runAt}
	r, err := NewStaffNoticeRelay(StaffNoticeRelayDeps{Communes: &pendingFake{ids: communes}, Locks: h.locks,
		Outbox: h.outbox, Comms: h.comms, Log: quietLogger})
	if err != nil {
		t.Fatal(err)
	}
	r.now = func() time.Time { return h.clock }
	h.relay = r
	return h
}

func TestNewStaffNoticeRelayRefusesMissingDependency(t *testing.T) {
	if _, err := NewStaffNoticeRelay(StaffNoticeRelayDeps{}); err == nil {
		t.Error("thiếu phụ thuộc mà vẫn dựng được bộ chuyển")
	}
}

func TestRelayDeliversEachCommuneInItsOwnContext(t *testing.T) {
	h := newRelayHarness(t, communeA, communeB)
	h.outbox.rows[communeA] = []docstore.StaffNoticeRow{outboxRow(t, "a1", "van-ban.chuyen-toi:la1")}
	h.outbox.rows[communeB] = []docstore.StaffNoticeRow{outboxRow(t, "b1", "van-ban.chuyen-toi:lb1")}
	h.relay.Tick(context.Background())

	if len(h.comms.calls) != 2 {
		t.Fatalf("gọi comms %d lần, muốn 2 (mỗi xã một lần)", len(h.comms.calls))
	}
	for _, c := range h.comms.calls {
		want := map[tenant.ID]string{communeA: "van-ban.chuyen-toi:la1", communeB: "van-ban.chuyen-toi:lb1"}[c.commune]
		if len(c.keys) != 1 || c.keys[0] != want {
			t.Errorf("xã %s gửi %v — rule 1: x-tenant-id là xã của dòng", c.commune, c.keys)
		}
	}
	if !h.outbox.delivered["a1"] || !h.outbox.delivered["b1"] {
		t.Errorf("đã gửi = %v", h.outbox.delivered)
	}
	if h.locks.releases() != 1 {
		t.Errorf("khoá nhả %d lần", h.locks.releases())
	}
}

// UNAVAILABLE: rows stay owed, the attempt is counted, the relay backs off — and when it retries it
// sends the SAME key, so comms delivers once.
func TestRelayUnavailableKeepsRowsBacksOffAndResendsTheSameKey(t *testing.T) {
	h := newRelayHarness(t, communeA, communeB)
	h.outbox.rows[communeA] = []docstore.StaffNoticeRow{outboxRow(t, "a1", "van-ban.chuyen-toi:la1")}
	h.outbox.rows[communeB] = []docstore.StaffNoticeRow{outboxRow(t, "b1", "van-ban.chuyen-toi:lb1")}
	h.comms.err = fmt.Errorf("wrap: %w", commsclient.ErrCommsUnavailable)
	h.relay.Tick(context.Background())

	if h.outbox.delivered["a1"] || h.outbox.attempts["a1"] != 1 || h.outbox.failed["a1"] != "" {
		t.Errorf("comms xuống: đã gửi=%v lần thử=%d lớp=%q", h.outbox.delivered["a1"], h.outbox.attempts["a1"], h.outbox.failed["a1"])
	}
	if len(h.comms.calls) != 1 {
		t.Errorf("comms xuống mà vẫn thử xã kế: %d lần gọi", len(h.comms.calls))
	}

	h.relay.Tick(context.Background()) // inside the backoff: nothing
	if len(h.comms.calls) != 1 {
		t.Fatalf("trong thời gian lùi mà vẫn gọi comms: %d", len(h.comms.calls))
	}

	h.comms.err = nil
	h.clock = h.clock.Add(StaffNoticeRelayInterval)
	h.relay.Tick(context.Background())
	if !h.outbox.delivered["a1"] || !h.outbox.delivered["b1"] {
		t.Fatalf("hết thời gian lùi mà chưa gửi: %v", h.outbox.delivered)
	}
	if h.comms.calls[0].keys[0] != h.comms.calls[1].keys[0] {
		t.Errorf("gửi lại với khoá khác: %v vs %v", h.comms.calls[0].keys, h.comms.calls[1].keys)
	}
}

func TestRelayBackoffDoublesAndIsCapped(t *testing.T) {
	h := newRelayHarness(t)
	h.relay.backOff()
	h.relay.backOff()
	if h.relay.backoff != 2*StaffNoticeRelayInterval {
		t.Errorf("lùi lần 2 = %v, muốn %v", h.relay.backoff, 2*StaffNoticeRelayInterval)
	}
	for i := 0; i < 12; i++ {
		h.relay.backOff()
	}
	if h.relay.backoff != maxRelayBackoff {
		t.Errorf("lùi = %v, muốn trần %v", h.relay.backoff, maxRelayBackoff)
	}
}

// INVALID_ARGUMENT refuses the whole call for one notice: only THAT row is set aside, the others leave.
func TestRelayInvalidArgumentSetsAsideOnlyTheRefusedRow(t *testing.T) {
	h := newRelayHarness(t, communeA)
	h.outbox.rows[communeA] = []docstore.StaffNoticeRow{
		outboxRow(t, "r1", "van-ban.chuyen-toi:l1"), outboxRow(t, "r2", "van-ban.chuyen-toi:bad"),
		outboxRow(t, "r3", "van-ban.chuyen-toi:l3"),
	}
	h.comms.refuse = map[string]bool{"van-ban.chuyen-toi:bad": true}
	h.relay.Tick(context.Background())

	if !h.outbox.delivered["r1"] || !h.outbox.delivered["r3"] || h.outbox.delivered["r2"] {
		t.Errorf("đã gửi = %v", h.outbox.delivered)
	}
	if h.outbox.failed["r2"] != docstore.FailedClassInvalidArgument || h.outbox.failed["r1"] != "" {
		t.Errorf("lớp lỗi = %v", h.outbox.failed)
	}

	// Set aside means not retried: the next tick sends nothing for it.
	calls := len(h.comms.calls)
	h.relay.Tick(context.Background())
	if len(h.comms.calls) != calls {
		t.Error("dòng đã đặt sang bên vẫn được gửi lại")
	}
}

func TestRelaySetsAsideAnUndecodablePayload(t *testing.T) {
	h := newRelayHarness(t, communeA)
	bad := outboxRow(t, "x1", "van-ban.chuyen-toi:x1")
	bad.Payload = []byte(`{"title": 7}`)
	h.outbox.rows[communeA] = []docstore.StaffNoticeRow{bad, outboxRow(t, "ok", "van-ban.chuyen-toi:ok")}
	h.relay.Tick(context.Background())
	if h.outbox.failed["x1"] != docstore.FailedClassPayloadInvalid || !h.outbox.delivered["ok"] {
		t.Errorf("lớp=%v đã gửi=%v", h.outbox.failed, h.outbox.delivered)
	}
}

// Any other error: rows stay owed, attempt counted, and the NEXT commune still drains.
func TestRelayOtherErrorKeepsRowsAndMovesOn(t *testing.T) {
	h := newRelayHarness(t, communeA, communeB)
	h.outbox.rows[communeA] = []docstore.StaffNoticeRow{outboxRow(t, "a1", "van-ban.chuyen-toi:la1")}
	h.outbox.rows[communeB] = []docstore.StaffNoticeRow{outboxRow(t, "b1", "van-ban.chuyen-toi:lb1")}
	h.comms.err = errors.New("lỗi hợp đồng")
	h.relay.Tick(context.Background())
	if len(h.comms.calls) != 2 || h.outbox.attempts["a1"] != 1 || h.outbox.attempts["b1"] != 1 ||
		h.outbox.delivered["a1"] || h.outbox.failed["a1"] != "" {
		t.Errorf("gọi=%d lần thử=%v đã gửi=%v lớp=%v", len(h.comms.calls), h.outbox.attempts, h.outbox.delivered, h.outbox.failed)
	}
}

func TestRelayWithoutTheLockDoesNothing(t *testing.T) {
	h := newRelayHarness(t, communeA)
	h.locks.held = map[string]bool{}
	h.outbox.rows[communeA] = []docstore.StaffNoticeRow{outboxRow(t, "a1", "van-ban.chuyen-toi:la1")}
	h.relay.Tick(context.Background())
	if len(h.comms.calls) != 0 {
		t.Error("không giữ khoá mà vẫn gửi")
	}
}

func TestRelayPagesAtMostOneHundred(t *testing.T) {
	h := newRelayHarness(t, communeA)
	for i := 0; i < 230; i++ {
		h.outbox.rows[communeA] = append(h.outbox.rows[communeA],
			outboxRow(t, fmt.Sprintf("r%03d", i), fmt.Sprintf("van-ban.chuyen-toi:l%03d", i)))
	}
	h.relay.Tick(context.Background())
	if len(h.comms.calls) != 3 {
		t.Fatalf("gọi %d lần, muốn 3 (100+100+30)", len(h.comms.calls))
	}
	for _, c := range h.comms.calls {
		if len(c.keys) > commsclient.MaxNoticesPerCall {
			t.Errorf("một lần gọi %d thông báo", len(c.keys))
		}
	}
	if len(h.outbox.delivered) != 230 {
		t.Errorf("đã gửi %d dòng, muốn 230", len(h.outbox.delivered))
	}
}
