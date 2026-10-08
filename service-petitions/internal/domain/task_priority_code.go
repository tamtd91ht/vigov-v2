package domain

// TaskPriorityCodeState is one live task priority of the commune, reduced to what another service may
// learn about it through ResolveTaskPriorityCodes (proto/vigov/petitions/v1/petitions.proto): its code
// (`muc_uu_tien_nhiem_vu.ma`) and whether the commune has it switched on (`dang_dung`).
//
// Nothing else — no label, no rank, no id. The caller (identity, before it writes an SLA row) needs one
// bit about a code it already holds; anything more is a copy of the catalogue nobody reviews.
type TaskPriorityCodeState struct {
	Code   string
	Active bool
}
