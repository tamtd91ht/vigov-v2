package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// THE ACCOUNTLESS INTAKE — ADR 0083 (TEMPORARY).
//
//	PROVED HERE   the row has NEITHER owner column, channel zalo-mini-app, so domain.Accountless holds ·
//	              the trail's actor is the fixed marker with Kind anonymous and the request IP, in the
//	              SAME transaction · the delta says accountless and holds no typed value · no outbox row
//	              (no ZNS) · the commune ceiling is counted inside the transaction after the commune's
//	              lock, from the commune's midnight · at 200 nothing is written · the Zalo-account ceiling
//	              is not consulted.

func TestAccountlessIntakeStoresNoOwnerAndAnonymousTrail(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{communeToday: domain.AccountlessDailyCeiling - 1}, hanThu()

	p, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), AccountlessSender("10.0.0.9"))
	if err != nil {
		t.Fatalf("Gui: %v", err)
	}
	got := kho.thay[0]
	if got.CongDanID != "" || got.ZaloAccountID != "" || got.Kenh != domain.KenhZaloMiniApp || !p.Accountless() {
		t.Errorf("row = (cong_dan_id %q, zalo %q, channel %q), want an accountless petition", got.CongDanID, got.ZaloAccountID, got.Kenh)
	}
	want := []string{"pg_advisory_xact_lock", "SELECT count(*)", "INSERT INTO phieu_phan_anh", "INSERT INTO audit_log"}
	if len(k.lenh) != len(want) {
		t.Fatalf("%d statements, want %d (lock, count, row, trail — no outbox): %v", len(k.lenh), len(want), k.lenh)
	}
	for i, w := range want {
		if !strings.Contains(k.lenh[i].sql, w) || !k.lenh[i].trongGiaoDich {
			t.Errorf("statement %d = %q (in tx %v), want %q inside the transaction", i, k.lenh[i].sql, k.lenh[i].trongGiaoDich, w)
		}
	}
	if kho.accountlessLocks != 1 || len(kho.lockedFor) != 0 {
		t.Errorf("locks: accountless %d, zalo %v", kho.accountlessLocks, kho.lockedFor)
	}
	if !kho.countedSince[0].Equal(time.Date(2026, 9, 21, 17, 0, 0, 0, time.UTC)) {
		t.Errorf("counted from %v, want the commune's midnight", kho.countedSince[0])
	}
	vet := k.lenh[3].args
	if vet[1] != audit.AnonymousActorID || vet[2] != audit.KindAnonymous || vet[3] != "10.0.0.9" {
		t.Errorf("actor = (%v, %v, %v), want (anonymous, anonymous, 10.0.0.9)", vet[1], vet[2], vet[3])
	}
	raw, _ := vet[7].([]byte)
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil || delta["accountless"] != true {
		t.Errorf("delta = %s, want accountless=true", raw)
	}
	for _, banned := range []string{"0900000000", "Nguyễn Văn An", "Đống rác"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("delta carries %q", banned)
		}
	}
}

func TestAccountlessIntakeAtCommuneCeilingWritesNothing(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{communeToday: domain.AccountlessDailyCeiling}, hanThu()

	p, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), AccountlessSender("10.0.0.9"))
	if !errors.Is(err, ErrAccountlessDailyLimit) {
		t.Fatalf("err = %v, want ErrAccountlessDailyLimit", err)
	}
	if p.MaTraCuu != "" || len(kho.thay) != 0 || k.commit != 0 || k.rollback != 1 {
		t.Errorf("code %q, rows %d, commit %d rollback %d", p.MaTraCuu, len(kho.thay), k.commit, k.rollback)
	}
}

// A session sender never takes the commune lock: the 200 are the accountless path's alone.
func TestSessionIntakeIsNotCountedAgainstTheAccountlessCeiling(t *testing.T) {
	for name, s := range map[string]IntakeSender{"citizen": congDanThu(), "zalo account": zaloThu()} {
		t.Run(name, func(t *testing.T) {
			k, kho := &khoGia{}, &khoPhieuGia{communeToday: 10_000}
			if _, err := dungGui(k, kho, hanThu()).Gui(ctxXa(xaThu), ycThu(), s); err != nil {
				t.Fatalf("Gui: %v", err)
			}
			if kho.accountlessLocks != 0 {
				t.Errorf("a %s send took the accountless lock", name)
			}
		})
	}
}

func TestAccountlessSenderIsTheOnlyOwnerlessActor(t *testing.T) {
	a, err := AccountlessSender("10.0.0.9").Actor()
	if err != nil || a.Kind != audit.KindAnonymous || a.ID != audit.AnonymousActorID {
		t.Errorf("actor = %+v, %v", a, err)
	}
	// An owner with no kind and no id is still a wiring fault, never anonymous.
	if _, err := (IntakeSender{}).Actor(); err == nil {
		t.Error("an empty sender became an actor")
	}
	if _, err := (IntakeSender{Owner: domain.PetitionOwner{Kind: domain.OwnerAccountless, ID: "x"}}).Actor(); err == nil {
		t.Error("an accountless sender carrying an id became an actor")
	}
}
