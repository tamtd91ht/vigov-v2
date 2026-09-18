package domain

// The Phân quyền matrix (docs/ui-ux/14-cau-hinh.md §4): rows are permission keys, columns are the
// commune's roles, a ticked cell is a grant.
//
// THREE TABLES MEET HERE AND ONLY TWO OF THEM BELONG TO A COMMUNE. `vai_tro` and `vai_tro_quyen`
// carry `tenant_id` and are partitioned by it; `quyen` carries none. That asymmetry is the single
// thing a reader has to understand before touching any of this, so it is stated on Quyen below
// rather than left to be inferred from the schema.

// Quyen is ONE permission key out of the software's catalogue.
//
// THE CATALOGUE HAS NO tenant_id, AND THAT IS NOT A FORGOTTEN COLUMN — it is the reason the table
// exists in this shape (service-identity/migrations/0001_init.sql:44). The set of rights the
// software can ENFORCE is a property of the SOFTWARE: every key here corresponds to a string some
// route passes to authz.RequirePermission. A commune decides which of its roles holds which key
// (`vai_tro_quyen`, per commune, partitioned); it cannot invent a key, because a key no route
// checks grants nothing and would only mislead the administrator ticking it.
//
// So this type is NOT a commune's business data and reading it is not a cross-commune read. What is
// per-commune is the GRANT — CapQuyen below — and that one is scoped, always.
//
// `Ma` IS ONE FLAT KEY, never a (subsystem, action) pair: rule 5, invariant 3b. `task.approve` is
// deliberately not `task.extend`, and the same string appears on the Phân quyền screen, in the
// `quyen` table and in the route declaration — one string, no translation layer to drift.
type Quyen struct {
	Ma   string // "task.extend" — the key authz.RequirePermission is given
	Nhom string // "NHIỆM VỤ" — grouping label on the matrix. DISPLAY ONLY, never an authority axis
	Nhan string // "Duyệt gia hạn" — what a person reads on the row
}

// VaiTroCot is one COLUMN of the matrix: a role of this commune, plus TWO counts of the staff
// holding it.
//
// BOTH ARE COUNTS AND NOTHING ELSE — no names, no ids, no contact details. A list of the people in
// a role would be personal data (rule 3) travelling on a screen that exists to edit permissions.
//
// WHY TWO NUMBERS AND NOT ONE, decided by the customer: the header reads
// "3 cán bộ · 1 đang hiệu lực". Neither number is wrong, and the gap between them is itself the
// information. One number alone forces a choice that misleads in one direction or the other —
// either it disagrees with the register on the neighbouring tab, or it claims an authority reaches
// people it does not reach.
type VaiTroCot struct {
	VaiTro

	// SoCanBo counts the rows of `nguoi_dung` pointing at this role and not soft-deleted.
	//
	// IT DELIBERATELY DOES NOT FILTER `co_tai_khoan` OR `dang_hoat_dong`: this is the figure an
	// administrator can count by hand on the Cấu hình → Người dùng tab, and a figure that disagrees
	// with the screen next door is the figure somebody reports as a defect.
	SoCanBo int

	// SoTaiKhoanDangHoatDong counts the subset whose sign-in account exists AND is not locked —
	// `co_tai_khoan AND dang_hoat_dong`, the same two columns store.Checker requires before any
	// grant of this role takes effect (checker.go).
	//
	// THE NAME IS WHAT THE DATASTORE HOLDS, NOT WHAT THE HEADER PRINTS (ADR 0017). The screen says
	// "đang hiệu lực"; naming the field after that label would assert that "an active account" and
	// "an effective permission" are one concept. They are not: a permission is effective only if the
	// role ALSO holds the key, which is the grant the very same response carries. This field states
	// the account fact and nothing more, and it is still the right name on a day no screen shows it.
	//
	// WHAT THE GAP MEANS, and it is the reason the customer asked for both: ticking a cell on this
	// role changes what SoTaiKhoanDangHoatDong people may do, and changes nothing at all for the
	// remainder. A role reading "3 cán bộ · 0 đang hiệu lực" is a column whose every tick reaches
	// nobody — the case that is impossible to see behind a single number.
	SoTaiKhoanDangHoatDong int
}

// CapQuyen is ONE ticked cell: this role, in this commune, holds this permission key.
//
// IT IS PER COMMUNE — `vai_tro_quyen` carries `tenant_id`, and that column is what stops commune
// A's grant from applying in commune B (rule 5, invariant 3). Only Quyen above is platform-wide.
type CapQuyen struct {
	VaiTroID string // `vai_tro.id` within this commune — never another commune's role
	QuyenMa  string // `quyen.ma`, guaranteed to exist by the foreign key
}

// MaTranQuyen is everything the Phân quyền tab needs to draw itself, read in one pass.
//
// THE GRANTS ARE A LIST OF TICKED CELLS, NOT A FULL MATRIX OF BOOLEANS. Eight roles by
// thirty-three keys is 264 cells of which a handful are true, and a dense matrix has a second
// failure mode a sparse list cannot have: it carries the roles and the keys a SECOND time, in a
// fixed order, so a client that reorders either axis silently reads every cell one column across.
// A cell that names both of its own ends cannot.
type MaTranQuyen struct {
	// DanhMuc is the whole catalogue, in `thu_tu` order — INCLUDING keys no role holds. A matrix
	// showing only granted keys is a matrix an administrator cannot grant anything new on.
	DanhMuc []Quyen

	// Cot is the commune's roles, in the order the commune arranged them (`thu_tu`).
	Cot []VaiTroCot

	// DaCap is the ticked cells. Anything absent is not granted — there is no third state.
	DaCap []CapQuyen
}
