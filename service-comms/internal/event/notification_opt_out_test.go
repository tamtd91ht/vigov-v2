package event

// ADR 0091 §4 on the send path: a citizen who turned Zalo notifications off still gets a ledger row,
// born `dan-da-tat-nhan`, for every ordinary transition — and is still sent `khong-tiep-nhan` and
// `chuyen-cap-tren`. NO ROW (the fake's zero value) READS AS ENABLED. A preference read failure is
// retried, never guessed.

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// prefsFake is the preference reader. off lists the citizens who turned notifications off; anyone else
// has no row, which reads as ENABLED — the same as the real store.
type prefsFake struct {
	off  map[string]bool
	err  error
	asks []string
	xa   []tenant.ID
}

func (p *prefsFake) Get(ctx context.Context, citizenID string) (domain.CitizenNotificationPreference, error) {
	p.asks = append(p.asks, citizenID)
	p.xa = append(p.xa, tenant.MustFrom(ctx))
	if p.err != nil {
		return domain.CitizenNotificationPreference{}, p.err
	}
	if p.off[citizenID] {
		return domain.CitizenNotificationPreference{ZaloEnabled: false, Stored: true}, nil
	}
	return domain.DefaultCitizenNotificationPreference(), nil
}

func TestStatusChangeOptedOutWritesTheRowBornOptedOut(t *testing.T) {
	so := &soGia{moi: true}
	prefs := &prefsFake{off: map[string]bool{maCongDanThu: true}}
	c := NewPhieuDoiTrangThai(so, &mauGia{ma: maMauThu}, prefs, quietLog())

	if err := c.Nhan(context.Background(), phongBi(t, tinMau())); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(so.nhan) != 1 || !so.nhan[0].CitizenOptedOut {
		t.Fatalf("an opted-out citizen's ordinary notice must still be recorded, born opted out: %+v", so.nhan)
	}
	if len(prefs.xa) != 1 || prefs.xa[0] != xaThu {
		t.Errorf("preference read in commune %v, want the envelope's %q", prefs.xa, xaThu)
	}
}

func TestStatusChangeWithNoPreferenceRowIsSent(t *testing.T) {
	so := &soGia{moi: true}
	c := NewPhieuDoiTrangThai(so, &mauGia{ma: maMauThu}, &prefsFake{}, quietLog())
	if err := c.Nhan(context.Background(), phongBi(t, tinMau())); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(so.nhan) != 1 || so.nhan[0].CitizenOptedOut {
		t.Fatalf("no row = enabled: %+v", so.nhan)
	}
}

func TestClosingTransitionsAreSentDespiteOptOut(t *testing.T) {
	for _, status := range []string{"khong-tiep-nhan", "chuyen-cap-tren"} {
		so := &soGia{moi: true}
		prefs := &prefsFake{off: map[string]bool{maCongDanThu: true}}
		c := NewPhieuDoiTrangThai(so, &mauGia{ma: maMauThu}, prefs, quietLog())
		tin := tinMau()
		tin.Status = status
		if err := c.Nhan(context.Background(), phongBi(t, tin)); err != nil {
			t.Fatalf("%s: Nhan: %v", status, err)
		}
		if len(so.nhan) != 1 || so.nhan[0].CitizenOptedOut {
			t.Errorf("%s must be sent whatever the switch says: %+v", status, so.nhan)
		}
		if len(prefs.asks) != 0 {
			t.Errorf("%s: the preference need not even be read", status)
		}
	}
}

func TestPreferenceReadFailureIsRetriedNotGuessed(t *testing.T) {
	so := &soGia{moi: true}
	c := NewPhieuDoiTrangThai(so, &mauGia{ma: maMauThu}, &prefsFake{err: errors.New("db down")}, quietLog())
	err := c.Nhan(context.Background(), phongBi(t, tinMau()))
	if err == nil || errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("a preference outage must be a RETRYABLE error, got %v", err)
	}
	if len(so.nhan) != 0 {
		t.Error("nothing may be recorded on a guess")
	}
}

func TestMergeNoticeFollowsTheSwitch(t *testing.T) {
	// ADR 0091 open #1: merge/unmerge notices are treated as ordinary (the recorder's reading).
	ledger := &soGia{moi: true}
	prefs := &prefsFake{off: map[string]bool{maCongDanThu: true}}
	c := NewPetitionMergeChangedConsumer(ledger, &mauGia{ma: maMauThu}, prefs, quietLog())
	if err := c.Nhan(context.Background(), mergeEnvelope(t, sampleMerge())); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(ledger.nhan) != 1 || !ledger.nhan[0].CitizenOptedOut {
		t.Fatalf("an opted-out citizen's merge notice is recorded opted out: %+v", ledger.nhan)
	}
}

func TestConsumersRefuseNilPreferences(t *testing.T) {
	for name, build := range map[string]func(){
		"status": func() { NewPhieuDoiTrangThai(&soGia{}, &mauGia{}, nil, quietLog()) },
		"merge":  func() { NewPetitionMergeChangedConsumer(&soGia{}, &mauGia{}, nil, quietLog()) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: a nil preference reader must be refused at construction", name)
				}
			}()
			build()
		}()
	}
}
