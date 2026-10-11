package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

type staffCountReaderFake struct {
	rows  []crosstenant.CommuneStaffCount
	err   error
	calls int
}

func (f *staffCountReaderFake) ActiveStaffCounts(context.Context) ([]crosstenant.CommuneStaffCount, error) {
	f.calls++
	return f.rows, f.err
}

var ascAt = time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)

func newASC(rows []crosstenant.CommuneStaffCount, readErr error) (*ActiveStaffCounts, *fakeOperatorStore, *staffCountReaderFake) {
	st := newFakeOperatorStore()
	rd := &staffCountReaderFake{rows: rows, err: readErr}
	return NewActiveStaffCounts(st, rd, func() time.Time { return ascAt }), st, rd
}

// ACCEPTED path: the counts pass through, and ONE committed entry names the reader (VH- code, never
// the id), the client IP, and the number of communes answered — not a count per commune.
func TestActiveStaffCountsAuditsTheRead(t *testing.T) {
	rows := []crosstenant.CommuneStaffCount{{TenantID: "01JA", Count: 3}, {TenantID: "01JB", Count: 1}}
	u, st, _ := newASC(rows, nil)
	got, err := u.ReadAsOperator(context.Background(), MiniAppOperator{Code: "VH-00001", IP: opIP})
	if err != nil {
		t.Fatalf("ReadAsOperator: %v", err)
	}
	want := []CommuneActiveStaffCount{{TenantID: "01JA", Count: 3}, {TenantID: "01JB", Count: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}
	if len(st.txs) != 1 || !st.lastTx().committed || len(st.state.audit) != 1 {
		t.Fatalf("txs %d, entries %d — want one committed transaction holding one entry", len(st.txs), len(st.state.audit))
	}
	e := st.state.audit[0]
	if e.Actor != "VH-00001" || e.Subject != "VH-00001" || e.ActorIP != opIP ||
		e.Action != domain.OperatorAuditActiveStaffCountsRead || !e.OccurredAt.Equal(ascAt) {
		t.Fatalf("entry = %+v", e)
	}
	if !reflect.DeepEqual(e.After, map[string]any{"communes": 2}) || e.Before != nil {
		t.Fatalf("entry after = %v before = %v — want the number of communes only", e.After, e.Before)
	}
}

// An empty answer is a valid answer, and it is audited too.
func TestActiveStaffCountsEmptyIsAudited(t *testing.T) {
	u, st, _ := newASC(nil, nil)
	got, err := u.ReadAsOperator(context.Background(), MiniAppOperator{Code: "VH-00001"})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v err %v, want an empty non-nil list", got, err)
	}
	if len(st.state.audit) != 1 || st.state.audit[0].After["communes"] != 0 {
		t.Fatalf("entries = %+v", st.state.audit)
	}
}

// FAIL CLOSED: the entry cannot be written → ErrActiveStaffCountsUnaudited and NO counts.
func TestActiveStaffCountsAuditFailureReturnsNothing(t *testing.T) {
	u, st, rd := newASC([]crosstenant.CommuneStaffCount{{TenantID: "01JA", Count: 3}}, nil)
	st.failAudit = domain.OperatorAuditActiveStaffCountsRead
	got, err := u.ReadAsOperator(context.Background(), MiniAppOperator{Code: "VH-00001", IP: opIP})
	if !errors.Is(err, ErrActiveStaffCountsUnaudited) || got != nil {
		t.Fatalf("got %v err %v, want ErrActiveStaffCountsUnaudited and no counts", got, err)
	}
	if rd.calls != 1 || st.lastTx().committed || len(st.state.audit) != 0 {
		t.Fatal("the failed entry was committed")
	}
}

// A read failure is not "unaudited": it is a store fault, and no entry claims an answer.
func TestActiveStaffCountsReadFailureWritesNoEntry(t *testing.T) {
	u, st, _ := newASC(nil, errors.New("db down"))
	_, err := u.ReadAsOperator(context.Background(), MiniAppOperator{Code: "VH-00001"})
	if err == nil || errors.Is(err, ErrActiveStaffCountsUnaudited) {
		t.Fatalf("err = %v, want a read error that is not the unaudited sentinel", err)
	}
	if len(st.txs) != 0 {
		t.Fatal("an entry was attempted for a read that failed")
	}
}

// No VH- code: refused before anything is read (rule 6, invariant 8 — no fallback). A shape the
// operator trail refuses (an internal id passed as the code) rolls back as unaudited, with no counts.
func TestActiveStaffCountsRefusesAnUnattributableReader(t *testing.T) {
	u, st, rd := newASC([]crosstenant.CommuneStaffCount{{TenantID: "01JA", Count: 3}}, nil)
	if _, err := u.ReadAsOperator(context.Background(), MiniAppOperator{Code: " "}); !errors.Is(err, ErrMiniAppNoOperatorCode) {
		t.Fatalf("err = %v, want ErrMiniAppNoOperatorCode", err)
	}
	if rd.calls != 0 || len(st.txs) != 0 {
		t.Fatal("read or audited without a reader")
	}
	got, err := u.ReadAsOperator(context.Background(), MiniAppOperator{Code: "01JDOPERATORAAAAAAAAAAAAAA"})
	if !errors.Is(err, ErrActiveStaffCountsUnaudited) || got != nil {
		t.Fatalf("an id as actor: %v %v — want unaudited and nothing", got, err)
	}
}
