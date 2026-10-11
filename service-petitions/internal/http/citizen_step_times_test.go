package http

// The citizen detail's step times (ADR 0047 G4, ADR 0091 §5) and the field list's placeholder
// (ADR 0091 §6) — the response builders, without a route.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

func TestCitizenDetailCarriesRecordedStepTimes(t *testing.T) {
	classified := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	forwarded := time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	awaiting := time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 10, 3, 5, 0, 0, 0, time.UTC)
	register := time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC)
	p := domain.PhieuPhanAnh{MaTraCuu: "PA-X", TrangThai: domain.DaDong, VaoSoLuc: register,
		PhanLoaiLuc: classified, ForwardedForProcessingAt: forwarded, XuLyXongLuc: resolved,
		AwaitingConfirmationAt: awaiting, DongLuc: closed}

	ra := phieuCuaToiRaNgoai(p, "")
	for name, c := range map[string]struct {
		got  *time.Time
		want time.Time
	}{
		"classified_at": {ra.ClassifiedAt, classified}, "forwarded_for_processing_at": {ra.ForwardedForProcessingAt, forwarded},
		"resolved_at": {ra.ResolvedAt, resolved}, "awaiting_confirmation_at": {ra.AwaitingConfirmationAt, awaiting},
		"closed_at": {ra.ClosedAt, closed},
	} {
		if c.got == nil || !c.got.Equal(c.want) {
			t.Errorf("%s = %v, want %v", name, c.got, c.want)
		}
	}
	b, err := json.Marshal(ra)
	if err != nil {
		t.Fatal(err)
	}
	// `vao_so_luc` stays internal (the list on phieuCuaToiRa): the first step is timed by clock_from.
	if strings.Contains(string(b), register.Format(time.RFC3339)) || strings.Contains(string(b), "received_at") {
		t.Errorf("the register instant reached the citizen: %s", b)
	}
}

// NULL = "not recorded" = ABSENT, never a zero date the screen would print as 0001-01-01.
func TestCitizenDetailOmitsUnrecordedStepTimes(t *testing.T) {
	b, err := json.Marshal(phieuCuaToiRaNgoai(domain.PhieuPhanAnh{MaTraCuu: "PA-X", TrangThai: domain.ChoDanXacNhan}, ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"classified_at", "forwarded_for_processing_at", "resolved_at",
		"awaiting_confirmation_at", "closed_at"} {
		if strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("%s present on a petition with no recorded time: %s", key, b)
		}
	}
}

func TestCitizenFieldCarriesPlaceholderWhenDeclared(t *testing.T) {
	with := citizenFieldOutOf(domain.PetitionFieldView{Code: "rac-thai", Label: "Rác thải",
		DescriptionPlaceholder: "Ví dụ: rác tồn đọng nhiều ngày"})
	b, err := json.Marshal(with)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"placeholder":"Ví dụ: rác tồn đọng nhiều ngày"`) {
		t.Errorf("placeholder missing: %s", b)
	}
	b, err = json.Marshal(citizenFieldOutOf(domain.PetitionFieldView{Code: "khac", Label: "Khác"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "placeholder") {
		t.Errorf("no sentence must be ABSENT (the client's generic hint), not \"\": %s", b)
	}
}

func TestMergeCarriesTierOnePlaceholder(t *testing.T) {
	got := domain.MergePetitionFields([]domain.FieldDefault{{Code: "rac-thai", DefaultLabel: "Rác thải", SortOrder: 1,
		Active: true, DescriptionPlaceholder: "Ví dụ"}}, []domain.NhanLinhVuc{{Ma: "rac-thai", Nhan: "Rác", Enabled: true}})
	if len(got) != 1 || got[0].DescriptionPlaceholder != "Ví dụ" {
		t.Errorf("merged = %+v — a commune override must not drop tier 1's sentence", got)
	}
}
