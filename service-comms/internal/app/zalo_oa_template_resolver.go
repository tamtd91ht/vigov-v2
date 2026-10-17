package app

// THE IMPLEMENTATION OF event.MauTinXa (ADR 0098 #10): which approved ZBS template THIS commune sends for
// THIS petition milestone — read at runtime from the commune's own configuration (rule 1, invariant 10),
// never a constant.
//
// IT CHECKS AT READ TIME what 0031 deliberately does not tie in the schema (0031 question 4, VENDOR
// CHOICES): a template attached to a milestone is not necessarily sendable. All five must hold, or the
// answer is "channel not configured" — the ledger records `chua-cau-hinh-kenh`, a degradation the commune
// can SEE (ADR 0006 consequence 3), never a guessed template:
//
//  1. a template is attached to the milestone
//  2. the OA row it belongs to is live (not retired)
//  3. that OA does not need re-authorization
//  4. the template is ENABLE at Zalo
//  5. the template declares the three parameters by name (params_match)
//
// SEVERAL OAs PER COMMUNE (ADR 0099 #1) CHANGE NOTHING HERE: a milestone is still ONE template in the
// whole commune (zalo_oa_template_one_per_milestone), and conditions 2 and 3 are checked on THAT
// template's own OA row — the OA the notice goes out through. No "default OA" exists or is needed.
//
// An expired ACCESS token is NOT a refusal here: it is the sender's to refresh before sending (block B;
// 0031 question 4 "refresh first, never send with it"). This answers which template, not whether a send
// can happen this second.
//
// WHY THE SENTINEL IS INJECTED: internal/event imports internal/app (it names app.YeuCauGhiNo), so this
// package cannot import event.ErrXaChuaCauHinhKenh without an import cycle. cmd/server passes that very
// value; the consumer compares with errors.Is against the same variable, so there is still ONE value.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ZaloOATemplateLookup is the template store's read the resolver needs. *store.ZaloOATemplateStore
// satisfies it; tests fake it.
type ZaloOATemplateLookup interface {
	AttachedForMilestone(ctx context.Context, milestone string) (docstore.AttachedTemplate, bool, error)
}

// ZaloOATemplateResolver implements event.MauTinXa.
type ZaloOATemplateResolver struct {
	templates     ZaloOATemplateLookup
	notConfigured error
}

// NewZaloOATemplateResolver panics on a missing dependency — including a nil sentinel: a resolver that
// answered `nil` for "not configured" would read as success to the consumer (fail closed).
func NewZaloOATemplateResolver(templates ZaloOATemplateLookup, notConfigured error) *ZaloOATemplateResolver {
	if templates == nil || notConfigured == nil {
		panic("app: NewZaloOATemplateResolver thiếu phụ thuộc")
	}
	return &ZaloOATemplateResolver{templates: templates, notConfigured: notConfigured}
}

// MauTin returns Zalo's template_id for milestone in the commune of ctx, or the injected
// "channel not configured" error (wrapped — compare with errors.Is). A read failure is returned as itself,
// never as "not configured": the queue redelivers a transient fault, while "not configured" is final.
//
// vi-name-ok: the method name of the existing event.MauTinXa interface this type implements.
func (r *ZaloOATemplateResolver) MauTin(ctx context.Context, moc string) (string, error) {
	if !domain.ValidMilestone(moc) {
		// A milestone 0031 does not know can never have a template attached.
		return "", fmt.Errorf("%w: milestone %q is not one a template can announce", r.notConfigured, moc)
	}
	a, found, err := r.templates.AttachedForMilestone(ctx, moc)
	if err != nil {
		return "", fmt.Errorf("zalo oa: read the template for milestone %s: %w", moc, err)
	}
	if reason := refuseReason(a, found); reason != "" {
		return "", fmt.Errorf("%w: %s (milestone %s)", r.notConfigured, reason, moc)
	}
	return a.Template.TemplateID, nil
}

// refuseReason names which of the five conditions failed; "" = sendable. The reason is a fixed phrase,
// never Zalo's text and never a token.
func refuseReason(a docstore.AttachedTemplate, found bool) string {
	switch {
	case !found:
		return "no template attached"
	case !a.OA.RetiredAt.IsZero():
		return "the OA of the attached template is retired"
	case a.OA.NeedsReauthorization():
		return "the OA needs re-authorization"
	case a.Template.ZaloStatus != domain.ZaloTemplateStatusEnabled:
		return "the attached template is not ENABLE at Zalo"
	case !a.Template.ParamsMatch:
		return "the attached template lacks a required parameter"
	}
	return ""
}
