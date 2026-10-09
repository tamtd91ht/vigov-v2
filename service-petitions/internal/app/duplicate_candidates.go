package app

// The suspected-duplicate search of ADR 0087 §6 — GET /api/v1/citizen-reports/{code}/duplicate-candidates.
//
// A SUGGESTION, NEVER AN ACT: it lists petitions an officer might merge with this one, within the
// commune's own radius and window (migration 0038, per-commune configuration — ADR 0087 stop condition #5).
// Merging stays a staff act (Merge). A read: no transaction, no audit entry — the rows carry what the
// register list already shows, reporters masked by the handler.

import (
	"context"
	"sort"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// DuplicateCandidateReader is the register as this search needs it. *petstore.PhieuPhanAnhStore satisfies it.
type DuplicateCandidateReader interface {
	TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) // vi-name-ok: the existing store method this interface is satisfied by
	DuplicateCandidates(ctx context.Context, q petstore.DuplicateQuery) ([]domain.PhieuPhanAnh, error)
}

// DuplicateThresholdReader reads the commune's radius and window. *petstore.PetitionSettingsStore
// satisfies it: no row is 50 m / 7 days, a failure is an error and never a default.
type DuplicateThresholdReader interface {
	DuplicateThresholds(ctx context.Context) (petstore.DuplicateThresholds, error)
}

// duplicateCandidateScan bounds the box read. Fifty metres and a week of one commune's reports is a
// handful of rows; past this many the answer says `Truncated` rather than pretending to be complete.
const duplicateCandidateScan = 200

// DuplicateCandidateResult is the answer: the candidates CLOSEST FIRST, the thresholds they were found
// with (so the screen can say "trong 50 m, 7 ngày"), and whether the box read hit its bound.
type DuplicateCandidateResult struct {
	Items        []domain.PhieuPhanAnh
	RadiusMeters int
	WindowDays   int
	Truncated    bool
}

// DuplicateCandidates is the search.
type DuplicateCandidates struct {
	petitions  DuplicateCandidateReader
	thresholds DuplicateThresholdReader
}

func NewDuplicateCandidates(petitions DuplicateCandidateReader, thresholds DuplicateThresholdReader) *DuplicateCandidates {
	return &DuplicateCandidates{petitions: petitions, thresholds: thresholds}
}

// Candidates lists the suspected duplicates of petition `code`.
//
// AN EMPTY LIST, NOT A REFUSAL, when the petition itself can take no merge: it has no location, it is
// resolved, it is already merged, or it is `can-bo` (read by a holder of `feedback.restricted`). The
// search has nothing to offer, which is a true answer; whether a merge is allowed is the merge act's.
//
// `can-bo` WITHOUT `feedback.restricted` is ErrPhieuHanChe — the unknown code's 404, as everywhere.
func (uc *DuplicateCandidates) Candidates(ctx context.Context, code string, restricted QuyenXemHanChe) (
	DuplicateCandidateResult, error) {

	p, err := uc.petitions.TheoMaTraCuu(ctx, code)
	if err != nil {
		return DuplicateCandidateResult{}, bocPhieu(ctx, "tìm phiếu nghi trùng", err)
	}
	if err := duocChamPhieuHanChe(p, restricted); err != nil {
		return DuplicateCandidateResult{}, bocPhieu(ctx, "tìm phiếu nghi trùng", err)
	}
	th, err := uc.thresholds.DuplicateThresholds(ctx)
	if err != nil {
		return DuplicateCandidateResult{}, bocPhieu(ctx, "tìm phiếu nghi trùng", err)
	}
	out := DuplicateCandidateResult{Items: []domain.PhieuPhanAnh{}, RadiusMeters: th.RadiusMeters,
		WindowDays: th.WindowDays}
	if p.Lat == nil || p.Lng == nil || p.MergedInto != "" || !domain.MergeOpen(p.TrangThai) ||
		p.LinhVuc == domain.LinhVucHanChe {
		return out, nil
	}

	rows, err := uc.petitions.DuplicateCandidates(ctx, petstore.DuplicateQuery{
		ExcludeID:  p.ID,
		Box:        domain.BoxAround(*p.Lat, *p.Lng, float64(th.RadiusMeters)),
		ReportedAt: p.GocDemHan,
		WindowDays: th.WindowDays,
		Field:      p.LinhVuc,
		Limit:      duplicateCandidateScan,
	})
	if err != nil {
		return DuplicateCandidateResult{}, bocPhieu(ctx, "tìm phiếu nghi trùng", err)
	}
	out.Truncated = len(rows) >= duplicateCandidateScan

	// THE BOX ERRS WIDE (domain.BoxAround); THE CIRCLE DECIDES. Distances are computed once and sorted on.
	type near struct {
		p domain.PhieuPhanAnh
		d float64
	}
	kept := make([]near, 0, len(rows))
	for _, c := range rows {
		if c.Lat == nil || c.Lng == nil {
			continue
		}
		if d := domain.DistanceMeters(*p.Lat, *p.Lng, *c.Lat, *c.Lng); d <= float64(th.RadiusMeters) {
			kept = append(kept, near{c, d})
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].d < kept[j].d })
	for _, k := range kept {
		out.Items = append(out.Items, k.p)
	}
	return out, nil
}
