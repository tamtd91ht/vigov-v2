package event

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/events"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// WHAT THIS FILE PROVES beyond the shape it shares with phieu_doi_trang_thai_test.go:
//
//  1. The deduplication key built from what this consumer hands over makes a republication under a
//     fresh envelope id ONE row, a second merge (occurrence 2) a NEW row, and a merge notice never
//     collides with a status notice for the same petition — the reason the event exists.
//  2. `kind` outside {gop-phieu, tach-phieu} is refused: an unvalidated kind equal to a status code
//     would swallow that status's real notice.
//
// The fake ledger below dedupes on domain.KhoaLanGui — the same function app.GhiNo uses — standing in
// for UNIQUE (tenant_id, khoa_lan_gui). The real constraint is proved by the store's PG tests.

// dedupLedger stands in for the ledger's UNIQUE (tenant_id, khoa_lan_gui).
type dedupLedger struct {
	rows map[string]app.YeuCauGhiNo
}

// vi-name-ok: implements the existing SoThongBao interface method (rule 12 inv 3)
func (l *dedupLedger) GhiNo(ctx context.Context, yc app.YeuCauGhiNo) (app.KetQuaGhiNo, error) {
	key, err := domain.KhoaLanGui(yc.DoiTuongLoai, yc.DoiTuongMa, yc.Moc, yc.Lan, yc.NguoiNhanMa, yc.Kenh)
	if err != nil {
		return app.KetQuaGhiNo{}, err
	}
	key = string(tenant.MustFrom(ctx)) + "|" + key
	if _, ok := l.rows[key]; ok {
		return app.KetQuaGhiNo{Moi: false}, nil
	}
	l.rows[key] = yc
	return app.KetQuaGhiNo{Moi: true}, nil
}

func newMergeConsumer(t *testing.T) (*PetitionMergeChangedConsumer, *soGia, *mauGia, *bytes.Buffer) {
	t.Helper()
	ledger := &soGia{moi: true}
	templates := &mauGia{ma: maMauThu}
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))
	return NewPetitionMergeChangedConsumer(ledger, templates, log), ledger, templates, &logBuf
}

// sampleMerge is a well-formed merge that owes the child's citizen a message.
func sampleMerge() *petitionsv1.PetitionMergeChanged {
	return &petitionsv1.PetitionMergeChanged{
		LookupCode: maPhieuThu,
		Kind:       MergeKindMerge,
		Occurrence: 1,
		CitizenId:  maCongDanThu,
		CitizenMessage: &petitionsv1.CitizenMessage{
			StatusLabel: "Đã gộp vào phiếu chính",
			NextStep:    "Phản ánh của bạn được xử lý cùng một vụ việc đã tiếp nhận",
		},
	}
}

func mergeEnvelope(t *testing.T, msg *petitionsv1.PetitionMergeChanged) events.Envelope {
	t.Helper()
	b, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(msg)
	if err != nil {
		t.Fatalf("mã hoá payload: %v", err)
	}
	return events.Envelope{
		ID:       "01JPHONGBIGOPPHIEUTHU0001",
		Name:     EventPetitionMergeChanged,
		TenantID: xaThu,
		At:       time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC),
		Payload:  b,
	}
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

// --- rule 1 -------------------------------------------------------------------------------------

func TestMergeEnvelopeWithoutCommuneIsRefused(t *testing.T) {
	c, ledger, templates, logBuf := newMergeConsumer(t)
	env := mergeEnvelope(t, sampleMerge())
	env.TenantID = ""

	if err := c.Nhan(context.Background(), env); !errors.Is(err, events.ErrNoTenant) {
		t.Fatalf("muốn ErrNoTenant, nhận %v", err)
	}
	if len(ledger.nhan) != 0 || len(templates.hoi) != 0 || logBuf.Len() != 0 {
		t.Fatalf("đã làm việc cho một phong bì không có xã")
	}
}

func TestMergeCommuneComesFromEnvelope(t *testing.T) {
	c, ledger, _, _ := newMergeConsumer(t)
	if err := c.Nhan(context.Background(), mergeEnvelope(t, sampleMerge())); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(ledger.xa) != 1 || ledger.xa[0] != xaThu {
		t.Fatalf("xã trong context: %v, muốn %v", ledger.xa, xaThu)
	}
}

// --- the contract's refusals --------------------------------------------------------------------

func TestMergeOtherEventNameIsRefused(t *testing.T) {
	for _, name := range []string{"petitions.merge_changed.v2", TenSuKienPhieuDoiTrangThai} {
		t.Run(name, func(t *testing.T) {
			c, ledger, _, _ := newMergeConsumer(t)
			env := mergeEnvelope(t, sampleMerge())
			env.Name = name
			err := c.Nhan(context.Background(), env)
			if !errors.Is(err, ErrSaiTenSuKien) || !errors.Is(err, ErrKhongThuLai) {
				t.Fatalf("muốn ErrSaiTenSuKien + ErrKhongThuLai, nhận %v", err)
			}
			if len(ledger.nhan) != 0 {
				t.Fatalf("đã ghi sổ cho một sự kiện không thuộc consumer này")
			}
		})
	}
}

func TestMergeMalformedPayloadIsRefusedWithoutQuotingIt(t *testing.T) {
	c, ledger, _, _ := newMergeConsumer(t)
	env := mergeEnvelope(t, sampleMerge())
	env.Payload = []byte(`{"occurrence": "DAUVETRIENGBIET"}`)

	err := c.Nhan(context.Background(), env)
	if !errors.Is(err, ErrGiaiMaPayload) || !errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("muốn ErrGiaiMaPayload + ErrKhongThuLai, nhận %v", err)
	}
	if strings.Contains(err.Error(), "DAUVETRIENGBIET") {
		t.Fatalf("lỗi trả về nhắc lại nội dung payload: %q", err.Error())
	}
	if len(ledger.nhan) != 0 {
		t.Fatalf("đã ghi sổ cho một payload hỏng")
	}
}

func TestMergeUnknownFieldIsTolerated(t *testing.T) {
	c, ledger, _, _ := newMergeConsumer(t)
	env := mergeEnvelope(t, sampleMerge())
	env.Payload = []byte(`{"lookup_code":"` + maPhieuThu + `","kind":"gop-phieu","occurrence":1,` +
		`"citizen_id":"` + maCongDanThu + `",` +
		`"citizen_message":{"status_label":"Đã gộp vào phiếu chính","next_step":"Được xử lý cùng vụ việc đã tiếp nhận"},` +
		`"future_optional_field":"x"}`)
	if err := c.Nhan(context.Background(), env); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(ledger.nhan) != 1 {
		t.Fatalf("ghi sổ %d dòng, muốn 1", len(ledger.nhan))
	}
}

func TestMergeMissingOrInvalidFieldIsRefused(t *testing.T) {
	cases := []struct {
		name string
		edit func(*petitionsv1.PetitionMergeChanged)
		want error
	}{
		{"thiếu mã tra cứu", func(m *petitionsv1.PetitionMergeChanged) { m.LookupCode = "" }, ErrThieuMaTraCuu},
		{"thiếu loại", func(m *petitionsv1.PetitionMergeChanged) { m.Kind = "" }, ErrThieuMoc},
		// A status code as kind is the collision the event exists to prevent.
		{"loại là mã trạng thái", func(m *petitionsv1.PetitionMergeChanged) { m.Kind = "da-dong" }, ErrUnknownMergeKind},
		{"loại lạ", func(m *petitionsv1.PetitionMergeChanged) { m.Kind = "gop_phieu" }, ErrUnknownMergeKind},
		{"thiếu người nhận", func(m *petitionsv1.PetitionMergeChanged) { m.CitizenId = "" }, ErrThieuNguoiNhan},
		{"lần bằng 0", func(m *petitionsv1.PetitionMergeChanged) { m.Occurrence = 0 }, ErrLanKhongHopLe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ledger, templates, _ := newMergeConsumer(t)
			msg := sampleMerge()
			tc.edit(msg)
			err := c.Nhan(context.Background(), mergeEnvelope(t, msg))
			if !errors.Is(err, tc.want) || !errors.Is(err, ErrKhongThuLai) {
				t.Fatalf("muốn %v + ErrKhongThuLai, nhận %v", tc.want, err)
			}
			if len(ledger.nhan) != 0 || len(templates.hoi) != 0 {
				t.Fatalf("đã làm việc cho một payload sai hợp đồng")
			}
		})
	}
}

// --- what gets handed to the ledger -------------------------------------------------------------

func TestMergeHandsOverEveryField(t *testing.T) {
	for _, kind := range []string{MergeKindMerge, MergeKindUnmerge} {
		t.Run(kind, func(t *testing.T) {
			c, ledger, templates, _ := newMergeConsumer(t)
			msg := sampleMerge()
			msg.Kind = kind
			if err := c.Nhan(context.Background(), mergeEnvelope(t, msg)); err != nil {
				t.Fatalf("Nhan: %v", err)
			}
			want := app.YeuCauGhiNo{
				DoiTuongLoai: domain.DoiTuongPhieuPhanAnh,
				DoiTuongMa:   maPhieuThu,
				Moc:          kind,
				Lan:          1,
				Kenh:         domain.KenhZaloZNS,
				NguoiNhanMa:  maCongDanThu,
				MauMa:        maMauThu,
				ThamSo: domain.ThamSoThongBao{
					MaTraCuu:     maPhieuThu,
					MocNhan:      "Đã gộp vào phiếu chính",
					ViecTiepTheo: "Phản ánh của bạn được xử lý cùng một vụ việc đã tiếp nhận",
				},
			}
			if len(ledger.nhan) != 1 || ledger.nhan[0] != want {
				t.Fatalf("yêu cầu ghi nợ:\n nhận %+v\n muốn %+v", ledger.nhan, want)
			}
			if ledger.nhan[0].NguoiGay.ID != "" {
				t.Fatalf("người gây phải để trống cho app điền chủ thể hệ thống")
			}
			// The template is asked for BY KIND — a per-commune template per act (ADR 0018).
			if len(templates.hoi) != 1 || templates.hoi[0] != kind {
				t.Fatalf("hỏi mẫu tin: %v, muốn [%s]", templates.hoi, kind)
			}
		})
	}
}

func TestMergeLedgerErrorIsWrappedAndRetryable(t *testing.T) {
	c, ledger, _, _ := newMergeConsumer(t)
	busy := errors.New("sổ đang bận")
	ledger.loi = busy
	err := c.Nhan(context.Background(), mergeEnvelope(t, sampleMerge()))
	if !errors.Is(err, busy) || errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("muốn bọc lỗi sổ và cho giao lại, nhận %v", err)
	}
}

// --- deduplication, against the real key function -----------------------------------------------

func TestMergeDeduplication(t *testing.T) {
	ledger := &dedupLedger{rows: map[string]app.YeuCauGhiNo{}}
	c := NewPetitionMergeChangedConsumer(ledger, &mauGia{ma: maMauThu}, quietLog())
	ctx := context.Background()

	// 1. The same act, republished under a fresh envelope id — ONE row.
	first := mergeEnvelope(t, sampleMerge())
	republished := mergeEnvelope(t, sampleMerge())
	republished.ID = "01JPHONGBIGOPPHIEUTHU0002"
	for _, env := range []events.Envelope{first, republished} {
		if err := c.Nhan(ctx, env); err != nil {
			t.Fatalf("Nhan: %v", err)
		}
	}
	if len(ledger.rows) != 1 {
		t.Fatalf("phát lại cùng một lần gộp sinh %d dòng, muốn 1", len(ledger.rows))
	}

	// 2. The petition merged a SECOND time after an unmerge — occurrence 2 owes a NEW row.
	second := sampleMerge()
	second.Occurrence = 2
	if err := c.Nhan(ctx, mergeEnvelope(t, second)); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(ledger.rows) != 2 {
		t.Fatalf("lần gộp thứ hai bị nuốt: %d dòng, muốn 2", len(ledger.rows))
	}

	// 3. The unmerge is its own act with its own occurrence count — a NEW row.
	unmerge := sampleMerge()
	unmerge.Kind = MergeKindUnmerge
	unmerge.CitizenMessage.StatusLabel = "Đã tách khỏi phiếu chính"
	if err := c.Nhan(ctx, mergeEnvelope(t, unmerge)); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(ledger.rows) != 3 {
		t.Fatalf("lần tách bị nuốt: %d dòng, muốn 3", len(ledger.rows))
	}

	// 4. The status consumer's notice for the same petition, occurrence 1, does NOT collide.
	status := NewPhieuDoiTrangThai(ledger, &mauGia{ma: maMauThu}, quietLog())
	if err := status.Nhan(ctx, phongBi(t, tinMau())); err != nil {
		t.Fatalf("Nhan trạng thái: %v", err)
	}
	if len(ledger.rows) != 4 {
		t.Fatalf("tin gộp phiếu va khoá với tin đổi trạng thái: %d dòng, muốn 4", len(ledger.rows))
	}
}

// --- absent citizen_message ---------------------------------------------------------------------

func TestMergeWithoutCitizenMessageDoesNothing(t *testing.T) {
	c, ledger, templates, logBuf := newMergeConsumer(t)
	msg := sampleMerge()
	msg.CitizenMessage = nil
	if err := c.Nhan(context.Background(), mergeEnvelope(t, msg)); err != nil {
		t.Fatalf("không nợ tin không phải lỗi: %v", err)
	}
	if len(ledger.nhan) != 0 || len(templates.hoi) != 0 || logBuf.Len() != 0 {
		t.Fatalf("đã ghi/hỏi/log cho một lần gộp không nợ tin nào")
	}
}

// --- a commune with no OA -----------------------------------------------------------------------

func TestMergeCommuneWithoutChannelIsVisibleAndNotRetried(t *testing.T) {
	c, ledger, templates, logBuf := newMergeConsumer(t)
	templates.loi = ErrXaChuaCauHinhKenh
	err := c.Nhan(context.Background(), mergeEnvelope(t, sampleMerge()))
	if !errors.Is(err, ErrXaChuaCauHinhKenh) || !errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("muốn ErrXaChuaCauHinhKenh + ErrKhongThuLai, nhận %v", err)
	}
	if len(ledger.nhan) != 0 {
		t.Fatalf("đã ghi sổ dù chưa có mẫu tin")
	}
	if !strings.Contains(logBuf.String(), maPhieuThu) || !strings.Contains(logBuf.String(), MergeKindMerge) {
		t.Fatalf("suy giảm phải nhìn thấy được, nhật ký: %q", logBuf.String())
	}
}

func TestMergeOtherTemplateErrorIsRetried(t *testing.T) {
	c, _, templates, _ := newMergeConsumer(t)
	templates.loi = errors.New("kho cấu hình không với tới được")
	err := c.Nhan(context.Background(), mergeEnvelope(t, sampleMerge()))
	if err == nil || errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("một sự cố hạ tầng PHẢI được giao lại: %v", err)
	}
}

// --- rule 3 -------------------------------------------------------------------------------------

func TestMergeNeverLogsCitizenID(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{{"gửi được", nil}, {"xã chưa cấu hình kênh", ErrXaChuaCauHinhKenh}} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, templates, logBuf := newMergeConsumer(t)
			templates.loi = tc.err
			_ = c.Nhan(context.Background(), mergeEnvelope(t, sampleMerge()))
			if strings.Contains(logBuf.String(), maCongDanThu) {
				t.Fatalf("mã công dân lọt vào nhật ký: %q", logBuf.String())
			}
		})
	}
}
