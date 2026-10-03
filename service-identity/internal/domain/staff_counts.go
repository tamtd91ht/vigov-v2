package domain

// StaffTally is one pair of numbers on the Danh bạ cán bộ screen (12-danh-ba-can-bo.md §2 and §3):
// how many rows of the register, and how many of those a citizen actually sees in the Mini App.
type StaffTally struct {
	Total     int
	Published int
}

// DepartmentStaffTally is the tally of the people whose `bo_phan_id` is DepartmentID. Never empty:
// the people in no department are StaffCounts.NoDepartment, not a department with an empty id.
type DepartmentStaffTally struct {
	DepartmentID string
	StaffTally
}

// StaffCounts is the whole answer of GET /api/v1/staff-counts for ONE commune, read in one statement
// so every number describes the same instant of the register.
//
// Total == NoDepartment.Total + Σ Departments[i].Total, and the same holds for Published. A
// department with no row is ABSENT from Departments: it counts 0/0.
type StaffCounts struct {
	StaffTally
	NoDepartment StaffTally
	Departments  []DepartmentStaffTally
}
