package domain

// The two reads behind ResolveActiveResidentialUnits and ResolveResidentialUnitNames (identity.proto,
// ADR 0088). Each struct holds exactly what its contract message holds — no slug, no type, no counts,
// no head of unit (a person): nothing a caller did not ask for can leave through a field that is not
// here.

// ActiveResidentialUnit is one `thon_to_dan_pho` row a caller may WRITE onto a record: live
// (`deleted_at IS NULL`) AND in use (`dang_dung`). Name is TODAY'S `ten`, for answering the request —
// never for storing.
type ActiveResidentialUnit struct {
	ID   string
	Name string
}

// ResidentialUnitName is one `thon_to_dan_pho` row as a record's printout names it. Live is
// `deleted_at IS NULL` and NOTHING ELSE: a unit taken out of use (`dang_dung = false`) is live — ADR
// 0059 §2, "filtered from pickers, never from labels".
type ResidentialUnitName struct {
	ID   string
	Name string
	Live bool
}
