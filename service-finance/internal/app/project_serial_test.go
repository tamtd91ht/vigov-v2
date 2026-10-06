package app

// The auto-issued project code (§9 `☑ Tự sinh mã`, user decision 06/10/2026): a blank `code` gets the
// next number of ONE per-commune series, DA01, DA02 … DA100. Every case runs the REAL use case over the
// REAL store over the fake driver, whose counter row lock models PostgreSQL's (driver_gia_du_an_test.go).
//
// WHAT IS NOT PROVED HERE: that PostgreSQL's `ON CONFLICT … DO UPDATE` really locks, and that
// `UNIQUE (tenant_id, ma)` refuses a duplicate. internal/store/project_code_counter_pg_test.go does,
// when VIGOV_TEST_DSN is set.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

func autoCodeRequest() YeuCauThemDuAn {
	yc := themDuAnHopLe()
	yc.Ma = ""
	return yc
}

func serialStore(taken ...string) *khoDAGia {
	k := khoDuAnSan()
	k.takenCodes = map[string]bool{}
	for _, c := range taken {
		k.takenCodes[c] = true
	}
	return k
}

// The first auto-coded project of a commune is DA01; the counter is locked, the code checked, the
// counter advanced and the project inserted — all in the ONE create transaction, with the trail.
func TestAutoCodeFirstProjectIsDA01InOneTransaction(t *testing.T) {
	k := serialStore()
	uc, ctx := dungUseCaseDuAn(t, k)

	kq, err := uc.Them(ctx, autoCodeRequest(), canBo)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if kq.DuAn.Ma != "DA01" {
		t.Fatalf("mã = %q, muốn DA01", kq.DuAn.Ma)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("giao dịch: mở %d commit %d, muốn 1/1", k.batDau, k.daCommit)
	}
	// ORDER INSIDE THE TRANSACTION: lock first, then the check, then the advance, then the insert.
	order := []string{"INSERT INTO project_code_counters", "count(*) FROM du_an",
		"UPDATE project_code_counters", "INSERT INTO du_an", "INSERT INTO audit_log"}
	pos := 0
	for _, l := range k.lenh {
		if pos < len(order) && strings.Contains(l.sql, order[pos]) {
			pos++
		}
	}
	if pos != len(order) {
		t.Errorf("thứ tự câu lệnh sai; dừng ở %q", order[pos])
	}
	// The counter moves to 2 ($2 of the advance).
	adv := k.cau("UPDATE project_code_counters")
	if len(adv) != 1 || adv[0].args[1] != int64(2) {
		t.Errorf("tăng bộ đếm = %+v, muốn next_number 2", adv)
	}
	// The trail names the issued code as its subject and says the system chose it.
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || vet[0].args[5] != "DA01" {
		t.Fatalf("vết = %+v, muốn subject DA01", vet)
	}
	var delta map[string]any
	if err := json.Unmarshal(vet[0].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	if delta["auto_code"] != true {
		t.Errorf("delta.auto_code = %v, muốn true", delta["auto_code"])
	}
}

// Numbers already taken — by a TYPED code, or by a project since removed (soft-deleted rows are in
// takenCodes, as MaDaDung counts them) — are stepped over, never issued again (rule 7, invariant 3).
func TestAutoCodeSkipsTakenIncludingRemovedProjects(t *testing.T) {
	// DA01 issued then removed; DA02 typed by hand; DA04 typed by hand.
	k := serialStore("DA01", "DA02", "DA04")
	uc, ctx := dungUseCaseDuAn(t, k)

	var got []string
	for i := 0; i < 3; i++ {
		kq, err := uc.Them(ctx, autoCodeRequest(), canBo)
		if err != nil {
			t.Fatalf("Them #%d: %v", i+1, err)
		}
		got = append(got, kq.DuAn.Ma)
	}
	want := []string{"DA03", "DA05", "DA06"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("mã cấp = %v, muốn %v", got, want)
	}
	if *k.counter != 7 {
		t.Errorf("bộ đếm = %d, muốn 7", *k.counter)
	}
}

// NEVER REISSUED AFTER A REMOVAL: the series continues past a removed project's code even if the
// counter row were reset — the check against `du_an` (removed rows included) is what decides.
func TestAutoCodeNeverReissuedAfterRemovalEvenIfCounterReset(t *testing.T) {
	k := serialStore()
	uc, ctx := dungUseCaseDuAn(t, k)
	first, err := uc.Them(ctx, autoCodeRequest(), canBo)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	// The project is removed: its row stays, so its code stays taken.
	k.hang = &hangDA{id: first.DuAn.ID, ma: first.DuAn.Ma, nam: 2026, hangMucID: "hm-chuyen-tiep",
		ten: "x", hanGiaiNgan: first.DuAn.ThoiHanGiaiNgan}
	if err := uc.Xoa(ctx, first.DuAn.ID, "", canBo); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	// Worst case: the counter row lost its value.
	one := int64(1)
	k.counter = &one

	second, err := uc.Them(ctx, autoCodeRequest(), canBo)
	if err != nil {
		t.Fatalf("Them sau xoá: %v", err)
	}
	if second.DuAn.Ma == first.DuAn.Ma {
		t.Fatalf("mã %q của dự án đã rút bị cấp lại", first.DuAn.Ma)
	}
	if second.DuAn.Ma != "DA02" {
		t.Errorf("mã = %q, muốn DA02", second.DuAn.Ma)
	}
}

// A TYPED code still works, does not touch the counter, and a taken typed code is still a 409-shaped
// refusal — removed projects included.
func TestTypedCodeStillWorksAndLeavesCounterAlone(t *testing.T) {
	k := serialStore("DA-2026-cu")
	uc, ctx := dungUseCaseDuAn(t, k)

	yc := themDuAnHopLe()
	yc.Ma = "DA07"
	kq, err := uc.Them(ctx, yc, canBo)
	if err != nil {
		t.Fatalf("Them mã tự nhập: %v", err)
	}
	if kq.DuAn.Ma != "DA07" {
		t.Errorf("mã = %q, muốn DA07", kq.DuAn.Ma)
	}
	if k.coCau("project_code_counters") {
		t.Error("mã tự nhập mà vẫn chạm bộ đếm")
	}

	yc.Ma = "DA-2026-cu"
	if _, err := uc.Them(ctx, yc, canBo); !errors.Is(err, fistore.ErrMaDuAnDaTonTai) {
		t.Errorf("mã đã dùng: lỗi = %v, muốn ErrMaDuAnDaTonTai", err)
	}

	// And the next AUTO code steps over the typed DA07 when it reaches it.
	k.counter = func() *int64 { n := int64(7); return &n }()
	auto, err := uc.Them(ctx, autoCodeRequest(), canBo)
	if err != nil {
		t.Fatalf("Them tự sinh: %v", err)
	}
	if auto.DuAn.Ma != "DA08" {
		t.Errorf("mã tự sinh = %q, muốn DA08 (bỏ qua DA07 đã nhập tay)", auto.DuAn.Ma)
	}
}

// A refused create (here: a category this commune does not have) rolls back — the counter does not
// advance, so the number is offered again to the next create. It was never issued.
func TestAutoCodeRolledBackCreateDoesNotBurnNumber(t *testing.T) {
	k := serialStore()
	uc, ctx := dungUseCaseDuAn(t, k)

	bad := autoCodeRequest()
	bad.HangMucID = "hm-cua-xa-khac"
	if _, err := uc.Them(ctx, bad, canBo); !errors.Is(err, fistore.ErrKhongThayHangMuc) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayHangMuc", err)
	}
	kq, err := uc.Them(ctx, autoCodeRequest(), canBo)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if kq.DuAn.Ma != "DA01" {
		t.Errorf("mã = %q, muốn DA01 — lần trước bị huỷ nên chưa cấp", kq.DuAn.Ma)
	}
}

// A long run of hand-typed DA codes ahead of the counter is refused rather than looped on under the
// row lock.
func TestAutoCodeGivesUpAfterSkipLimit(t *testing.T) {
	var taken []string
	for n := int64(1); n <= domain.ProjectSerialSkipLimit; n++ {
		taken = append(taken, domain.FormatProjectSerial(n))
	}
	k := serialStore(taken...)
	uc, ctx := dungUseCaseDuAn(t, k)

	if _, err := uc.Them(ctx, autoCodeRequest(), canBo); !errors.Is(err, domain.ErrProjectSerialExhausted) {
		t.Fatalf("lỗi = %v, muốn ErrProjectSerialExhausted", err)
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d, muốn 0", k.daCommit)
	}
}

// CONCURRENCY: twenty clerks press `Thêm` at once, on a POOL of connections. The fake's counter lock
// behaves like PostgreSQL's row lock (taken by the upsert, held to commit), so this proves the use
// case reads the number UNDER the lock and advances it in the SAME transaction: twenty distinct codes,
// DA01..DA20, no gap, no duplicate.
func TestAutoCodeConcurrentCreatesGetDistinctCodes(t *testing.T) {
	const clerks = 20
	k := serialStore()
	k.serialLock = make(chan struct{}, 1)

	db := sql.OpenDB(k)
	db.SetMaxOpenConns(clerks)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	uc := NewDuAn(kho, fistore.NewDuAnGhiStore(kho))
	var ids atomic.Int64
	uc.sinhID = func() (string, error) { return fmt.Sprintf("01JCONCURRENT%013d", ids.Add(1)), nil }
	ctx := tenant.Into(context.Background(), xaA)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		codes []string
		errs  []error
	)
	for i := 0; i < clerks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kq, err := uc.Them(ctx, autoCodeRequest(), canBo)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			codes = append(codes, kq.DuAn.Ma)
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		t.Fatalf("%d lần thêm lỗi, đầu tiên: %v", len(errs), errs[0])
	}
	sort.Slice(codes, func(i, j int) bool {
		if len(codes[i]) != len(codes[j]) {
			return len(codes[i]) < len(codes[j])
		}
		return codes[i] < codes[j]
	})
	for i, c := range codes {
		if want := domain.FormatProjectSerial(int64(i + 1)); c != want {
			t.Fatalf("mã thứ %d = %q, muốn %q (toàn bộ: %v)", i+1, c, want, codes)
		}
	}
}
