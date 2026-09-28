package domain

// The display reads behind ResolveOrgUnitNames and ResolveTaskBlocLabels, and the code translation
// behind ResolveLiveOrgUnitCodes (identity.proto). Each struct holds exactly what its contract
// message holds — nothing a caller did not ask for can leave through a field that is not here.

// OrgUnitName is one `bo_phan` row as the task register prints it: the id it is keyed by, the name
// `bo_phan.ten` holds today, and whether the row is still on the org chart.
//
// Live is `deleted_at IS NULL`. A REMOVED unit is still returned — the register is an archival
// printout and must name what the record holds (ADR 0034's argument, applied to units).
type OrgUnitName struct {
	ID   string
	Name string
	Live bool
}

// TaskBlocLabel is one `khoi_nhiem_vu` row as the task register prints it. Live is
// `deleted_at IS NULL` and nothing else — a row switched off with `dang_dung = false` is live.
type TaskBlocLabel struct {
	Ma    string
	Label string
	Live  bool
}

// OrgUnitCodeMatch pairs a unit business code (`bo_phan.ma`) with the id of the LIVE unit carrying
// it. Only live units are ever matched — the caller writes the id.
type OrgUnitCodeMatch struct {
	Ma string
	ID string
}
