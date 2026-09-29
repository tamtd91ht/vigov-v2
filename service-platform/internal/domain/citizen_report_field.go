package domain

// CitizenReportField is one row of `petition_field` (migration 0011): a tier-1 citizen report field
// code and its platform defaults (ADR 0026, ADR 0060). Configuration shared by every commune — no
// commune, no counts, no personal data.
//
// There is no deleted/withdrawn state: a code is never removed, only retired (IsActive false), and a
// retired code is still returned so old petitions keep their label (ADR 0060 §4).
type CitizenReportField struct {
	Code         string
	DefaultLabel string
	SortOrder    int32
	Icon         string // "" = not declared
	Tone         string // "" = not declared
	IsActive     bool
}
