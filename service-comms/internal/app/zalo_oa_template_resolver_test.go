package app_test

// An EXTERNAL test package on purpose: internal/event imports internal/app, so only from outside can a
// test hold the resolver against the REAL event.MauTinXa interface and the REAL
// event.ErrXaChuaCauHinhKenh value the consumer compares with.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/event"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

var _ event.MauTinXa = (*app.ZaloOATemplateResolver)(nil)

type fakeLookup struct {
	a     docstore.AttachedTemplate
	found bool
	err   error
	asked string
}

func (f *fakeLookup) AttachedForMilestone(_ context.Context, m string) (docstore.AttachedTemplate, bool, error) {
	f.asked = m
	return f.a, f.found, f.err
}

func sendable() docstore.AttachedTemplate {
	return docstore.AttachedTemplate{
		Template: domain.ZaloOATemplate{TemplateID: "1001", ZaloStatus: domain.ZaloTemplateStatusEnabled, ParamsMatch: true,
			Milestone: domain.MilestoneReceived},
		OA: domain.CommuneZaloOA{ID: "01JOAROW", OAID: "579745863508352884"},
	}
}

func TestResolverAnswersTheAttachedTemplate(t *testing.T) {
	l := &fakeLookup{a: sendable(), found: true}
	r := app.NewZaloOATemplateResolver(l, event.ErrXaChuaCauHinhKenh)
	ctx := tenant.Into(context.Background(), tenant.ID("01JAAAAAAAAAAAAAAAAAAAAAAA"))
	id, err := r.MauTin(ctx, domain.MilestoneReceived)
	if err != nil || id != "1001" || l.asked != domain.MilestoneReceived {
		t.Fatalf("= %q, %v (asked %q)", id, err, l.asked)
	}
}

func TestResolverRefusesEveryUnsendableCaseAsNotConfigured(t *testing.T) {
	cases := map[string]func(*fakeLookup){
		"no template attached":  func(l *fakeLookup) { l.found = false },
		"OA retired":            func(l *fakeLookup) { l.a.OA.RetiredAt = time.Now() },
		"OA needs reauthorize":  func(l *fakeLookup) { l.a.OA.NeedsReauthorizationSince = time.Now() },
		"template not ENABLE":   func(l *fakeLookup) { l.a.Template.ZaloStatus = domain.ZaloTemplateStatusRejected },
		"parameters incomplete": func(l *fakeLookup) { l.a.Template.ParamsMatch = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			l := &fakeLookup{a: sendable(), found: true}
			mutate(l)
			r := app.NewZaloOATemplateResolver(l, event.ErrXaChuaCauHinhKenh)
			id, err := r.MauTin(context.Background(), domain.MilestoneReceived)
			if id != "" || !errors.Is(err, event.ErrXaChuaCauHinhKenh) {
				t.Fatalf("= %q, %v", id, err)
			}
		})
	}
}

func TestResolverUnknownMilestoneIsNotConfiguredAndNotRead(t *testing.T) {
	l := &fakeLookup{a: sendable(), found: true}
	r := app.NewZaloOATemplateResolver(l, event.ErrXaChuaCauHinhKenh)
	if _, err := r.MauTin(context.Background(), "moi"); !errors.Is(err, event.ErrXaChuaCauHinhKenh) {
		t.Fatalf("err = %v", err)
	}
	if l.asked != "" {
		t.Fatal("an unknown milestone reached the store")
	}
}

// A read failure is transient and must be redelivered — never collapsed into "not configured", which
// the consumer treats as final.
func TestResolverReadFailureIsNotNotConfigured(t *testing.T) {
	boom := errors.New("db down")
	r := app.NewZaloOATemplateResolver(&fakeLookup{err: boom}, event.ErrXaChuaCauHinhKenh)
	_, err := r.MauTin(context.Background(), domain.MilestoneReceived)
	if !errors.Is(err, boom) || errors.Is(err, event.ErrXaChuaCauHinhKenh) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolverRefusesANilSentinel(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a nil 'not configured' value was accepted — the consumer would read refusal as success")
		}
	}()
	app.NewZaloOATemplateResolver(&fakeLookup{}, nil)
}
