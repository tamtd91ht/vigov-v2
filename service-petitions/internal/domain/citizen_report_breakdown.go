package domain

// The /phan-anh statistics read (owner decisions of 09/10/2026, batch A): the register's figures for one
// period, split by field, by the unit that finished the work and by residential unit, plus the heat-map
// points.
//
// THIS FILE NAMES THE FIGURES; IT DOES NOT COMPUTE THEM — summary_metrics.go's rule. Every count is a
// SQL predicate in internal/store, and the on-time / late figures are THE SAME TEXT as the
// /tong-quan tile (store.citizenReportMetricCondition), so the field rows add up to the tile.
//
// NO RATIO AND NO AVERAGE IS COMPUTED ON THE SERVER (summary_metrics.go:13-15): the client receives a
// sample and a sum and divides, showing a dash for an empty sample.

import "time"

// CitizenReportFieldFigures is one field's row — or, with FieldCode "", the row of petitions that are
// still unclassified (`linh_vuc` NULL), which the client labels and never merges into a field.
//
//	received                         `vao_so_luc` in the period, khong-tiep-nhan included (the tile's)
//	finished                         `xu_ly_xong_luc` in the period
//	on_time_sample · on_time · late  the tile's predicates, byte for byte
//	rating_sample · rating_sum       STOCK — the tile's rating figures, not bound to the period
//	overdue                          STOCK — past the classification ceiling or the resolve deadline now
type CitizenReportFieldFigures struct {
	FieldCode    string
	Received     int
	Finished     int
	OnTimeSample int
	OnTime       int
	Late         int
	RatingSample int
	RatingSum    int
	Overdue      int
}

// CitizenReportResidentialUnitFigures is one residential unit's row (thôn / tổ dân phố — the glossary's
// `ResidentialUnit`, never "hamlet"); ResidentialUnitID "" is the row of petitions with no `thon_id`
// ("Chưa xác định địa bàn", labelled by the client). Received EXCLUDES `khong-tiep-nhan`: a report the
// commune declined is not a problem located in that place.
//
// Name is the unit's name as identity answers it TODAY (ADR 0088: a retired unit still shows its name),
// filled by the use case, never stored. "" on the no-unit row, and on an id identity does not answer.
type CitizenReportResidentialUnitFigures struct {
	ResidentialUnitID string
	Name              string
	Received          int
	Overdue           int
}

// CitizenReportUnitFigures is one unit's row: the petitions FINISHED in the period
// (`xu_ly_xong_luc` in it), attributed to the unit holding the petition at that instant.
//
// HandlingWorkingSeconds is the WORKING time, summed over HandlingSample petitions, from the LAST
// assignment before `xu_ly_xong_luc` (owner decision 09/10/2026) to `xu_ly_xong_luc`, as identity's
// calendar counts it. The average
// is the client's division. HandlingSample < Finished when a petition has no recorded hand-over.
//
// OrgUnitID "" is the row of petitions finished with no recorded hand-over at all (assigned before the
// processing log existed, migration 0013) — never folded into a real unit.
type CitizenReportUnitFigures struct {
	OrgUnitID              string
	Finished               int
	HandlingSample         int
	HandlingWorkingSeconds uint64
}

// CitizenReportBreakdown is the whole read. Totals is section (a): the on-time / late figures of the
// period and the overdue stock, over every row the field rows cover.
type CitizenReportBreakdown struct {
	// AsOf is the database instant the stock figures were measured at.
	AsOf             time.Time
	Totals           CitizenReportFieldFigures
	Fields           []CitizenReportFieldFigures
	ResidentialUnits []CitizenReportResidentialUnitFigures
	Units            []CitizenReportUnitFigures
}

// CitizenReportHandling is one petition finished in the period, as the store hands it to the use case
// that measures it: the unit holding it when the work was finished, and since when that unit held it.
// AssignedAt is zero — and OrgUnitID "" — when no hand-over was recorded before the finishing instant.
type CitizenReportHandling struct {
	OrgUnitID  string
	AssignedAt time.Time
	FinishedAt time.Time
}

// CitizenReportPoint is one petition on the heat map: its scene coordinates and its status, AND
// NOTHING ELSE — no code, no content, no reporter (rule 3). Coordinates are the only location the map
// needs, and a code would turn a heat map into a lookup table of who reported what where.
type CitizenReportPoint struct {
	Lat    float64
	Lng    float64
	Status TrangThai
}
