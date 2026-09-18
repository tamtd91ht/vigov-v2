package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The read route behind the Phân quyền matrix. GET /api/v1/role-permissions
//
// THERE IS NO WRITE ROUTE, AND NO SCAFFOLDING FOR ONE IS LEFT HERE — NOT EVEN AN EMPTY HANDLER.
// This is the place the next person will reach for when adding `PUT /api/v1/roles/{id}/permissions`
// (the spec's "Lưu cả cột", §11 and §12.5), so the reason it is absent is written HERE rather than
// in a commit message:
//
//	Saving one role's column is the operation that decides WHO MAY GRANT WHAT, and two questions
//	the customer has not answered sit directly underneath it
//	(kb/00-foundation/open-questions.json):
//
//	  #13  is a commune to be stopped from removing its LAST administrator — by lock, by delete, or
//	       by taking `admin.user` off the last role that has it? Saving a column is the third of
//	       those three paths, and it is the one nobody thinks of: untick one cell, press Lưu, and
//	       the commune can no longer administer itself. Nobody inside it can undo that, and the
//	       vendor is not permitted to (ADR 0003). It is a procedural dead end, not a defect with a
//	       hotfix.
//	  #14  may a holder of `admin.user` act on THEMSELVES — grant their own role the strongest set
//	       of keys? If not constrained, `admin.user` in practice subsumes every other key, and the
//	       33-key model the commune's leadership signed off describes something the software does
//	       not actually do.
//
//	Both are decisions for the authority, not for whoever writes the handler. A half-written write
//	path looks like a decision somebody made, and the audit trail it leaves cannot afterwards
//	distinguish a legitimate grant from a self-elevation.
//
// The READ side has no such question attached: showing an administrator the grants as they stand
// changes nothing and can be un-shown.

// quyenMucRa is one permission key as it leaves the API — one ROW of the matrix.
//
// `code` AND NOT `id`: the key IS the identifier (`quyen.ma` is the primary key), it is the string
// the route declaration passes to authz.RequirePermission, and it is what `grants` points at. A
// surrogate id here would be a second name for one thing (rule 5, invariant 3b).
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a permission key and its Vietnamese label describe what
// the software can enforce, not a person.
type quyenMucRa struct {
	Code  string `json:"code"`  // "task.extend"
	Label string `json:"label"` // "Duyệt gia hạn"
}

// nhomQuyenRa is one group of rows — the `NHIỆM VỤ` band on the matrix.
//
// THE GROUP HAS A NAME AND NO CODE, because `quyen.nhom` is a display label and there is no stable
// key behind it. Nothing may branch on it: a group is a heading on a screen, never an authority
// axis — the same line domain.VaiTro.LaLanhDao draws for the leader flag.
type nhomQuyenRa struct {
	Name        string       `json:"name"`
	Permissions []quyenMucRa `json:"permissions"`
}

// vaiTroCotRa is one COLUMN of the matrix: a role of this commune and TWO counts of its holders.
//
// BOTH ARE COUNTS AND CARRY NO NAMES. A list of the people in a role would be personal data on a
// screen that exists to edit permissions (rule 3). What each count includes is argued on
// domain.VaiTroCot.
//
// `active_account_count` NAMES THE ACCOUNT FACT, NOT THE SCREEN'S WORDS (ADR 0017). A field called
// `effective_count` would assert that an open account and an effective permission are one concept,
// and they are not — the permission is effective only if the role also holds the key, which is the
// `grants` list in this very response. The contract states the fact it holds and lets the screen do
// the arithmetic.
//
// THE SCREEN'S EXACT WORDING IS DELIBERATELY NOT QUOTED HERE. An earlier version of this comment
// said the header reads "3 cán bộ · 1 đang hiệu lực" — and by the time the tab was built it read
// "3 cán bộ · 1 trong số đó có tài khoản đang hoạt động", for the same reason this field is not
// called `effective_count`. Two copies of one sentence in two languages in two services is two
// copies that drift, and the stale one is the one a reader trusts. The wording lives in
// web-admin/src/features/cau-hinh/nhan-ma-tran.ts, which is its single owner (rule 9).
//
// `is_leader` CHOOSES THE DEFAULT SCREEN AND NOTHING ELSE — the whole argument is on
// domain.VaiTro.LaLanhDao. It is repeated here because a matrix is exactly where the line is
// easiest to cross: a client holding every role and every grant can branch on any field it likes.
type vaiTroCotRa struct {
	ID       string `json:"id"` // ULID — what `grants` and `staff.role_id` reference
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsLeader bool   `json:"is_leader"`

	// THE TWO COUNTS CARRY THEIR MEANING IN A TRAILING COMMENT, not in a doc comment above:
	// tools/apidoc copies the trailing one into the OpenAPI `description`, and that is what reaches
	// web-admin's generated types. A doc comment here would explain the difference to Go readers
	// only — and the reader who most needs it is the one drawing that header (ADR 0014). The full
	// argument stays on domain.VaiTroCot, in one place (rule 9).
	StaffCount         int `json:"staff_count"`          // mọi cán bộ giữ vai trò này — đúng con số tab Người dùng hiện
	ActiveAccountCount int `json:"active_account_count"` // trong số đó, ai có tài khoản và đang mở: bật/tắt một ô chỉ đổi quyền của những người này
}

// capQuyenRa is ONE TICKED CELL: this role holds this permission key.
//
// WHY A SPARSE LIST OF CELLS RATHER THAN A DENSE MATRIX OF BOOLEANS, which is the shape the screen
// draws and therefore the obvious thing to send:
//
//   - eight roles by thirty-three keys is 264 cells, nearly all `false`. Sending the false ones
//     costs more than the whole rest of this response and says nothing;
//   - a dense matrix carries the two axes A SECOND TIME, implied by position. The moment a client
//     sorts the roles for display, or the catalogue gains a key in the middle, every cell is read
//     one column across — with no error, and a plausible-looking screen. A cell that names both of
//     its own ends cannot fall out of phase with anything;
//   - absence is the only other state. There is no "unknown": a cell that is not here is not
//     granted, which is also how rule 5, invariant 2 reads a missing declaration.
//
// WHY AN OBJECT AND NOT THE TERSER PAIR `["vt-001","admin.role"]`, which was the shape first
// reached for and is a few kilobytes cheaper on a response already under 20 KB: tools/apidoc types
// a Go `[2]string` as `{"type":"array","items":{"type":"string"}}` — OpenAPI as generated here
// cannot say "exactly two, role first". The web builds its types from that file and from nothing
// else (ADR 0014), so the position would have been documented only in this comment, on the side of
// the wire that never reads it. A contract that cannot state its own invariant is the v1 failure
// ADR 0014 exists to prevent, and two field names cost less than one screen drawn one column across.
//
// `role_id` AND `permission` ARE THE WORDS RULE 5, INVARIANT 3 ALREADY USES for the same pair.
type capQuyenRa struct {
	RoleID string `json:"role_id"` // vaiTroCotRa.ID — a role OF THIS COMMUNE
	// Permission is quyenMucRa.Code — the flat key, e.g. "admin.role".
	Permission string `json:"permission"`
}

// maTranQuyenRa is the whole tab in one response.
//
// AN OBJECT, and the three lists inside it are objects' fields rather than three routes: the matrix
// is only correct if its rows, its columns and its cells were read from the same instant. Three
// separate calls can straddle a grant being changed, and the screen that results shows a tick in a
// column that no longer exists.
//
// THERE IS NO next_cursor AND NO has_more: this route returns the whole matrix or it fails. See
// idstore.TranDanhMucQuyen for what "or it fails" means and why it is not a truncation.
type maTranQuyenRa struct {
	Groups []nhomQuyenRa `json:"groups"`
	Roles  []vaiTroCotRa `json:"roles"`
	Grants []capQuyenRa  `json:"grants"`
}

// gomTheoNhom groups the catalogue by `nhom`, KEEPING THE STORE'S ORDER on both axes.
//
// Groups appear in the order their FIRST key appears, and keys keep the order they arrived in
// (`thu_tu`). That is the specification's order (§4.2), and re-sorting either axis here — however
// tidy — would silently overrule the order the rows were given in the migration.
//
// It does not assume a group's keys are contiguous. They are today, and a key inserted later with
// a `thu_tu` in the wrong band would otherwise produce the same group heading twice — two bands
// with one name on one screen, which reads as two different things.
func gomTheoNhom(danhMuc []domain.Quyen) []nhomQuyenRa {
	// [] AND NOT null when the catalogue is empty: a client that has to handle both shapes handles
	// one of them wrong.
	nhom := make([]nhomQuyenRa, 0, 16)
	viTri := make(map[string]int, 16)
	for _, q := range danhMuc {
		i, co := viTri[q.Nhom]
		if !co {
			i = len(nhom)
			viTri[q.Nhom] = i
			nhom = append(nhom, nhomQuyenRa{Name: q.Nhom, Permissions: make([]quyenMucRa, 0, 8)})
		}
		nhom[i].Permissions = append(nhom[i].Permissions, quyenMucRa{Code: q.Ma, Label: q.Nhan})
	}
	return nhom
}

// MaTranQuyen serves the Phân quyền matrix. GET /api/v1/role-permissions
//
// NO AUDIT ENTRY, AND THAT IS A DECISION RATHER THAN AN OMISSION. Rule 6, invariant 7 names the two
// reads that must themselves be audited: reading FULL personal data, and reading ACROSS communes.
// This is neither. It carries no personal data at all — the only figure about people is a count
// (rule 3) — and it reads inside the commune the request arrived in, with `tenant_id` bound from
// the context on both scoped queries. An entry every time an administrator opens a configuration
// tab would bury the entries that carry legal weight under thousands that carry none.
//
// WHAT WILL NEED AN ENTRY is the write this route deliberately does not have: changing a grant is a
// privilege change, and rule 6, invariant 5 requires it audited in the same transaction. See the
// note at the top of this file for why that route is not here yet.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) MaTranQuyen(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mt, err := h.d.MaTran.MaTran(ctx)
	if err != nil {
		// THE CEILINGS ANSWER 500 AND REFUSE — they do not return what was read. A short matrix is
		// a permission key missing from the screen, which reads as "this role does not hold that
		// right" and is indistinguishable from the truth. The log line names the commune and the
		// ceiling reached, because that is the only thing an operator can act on: one process
		// serves 200+ communes into one log stream.
		switch {
		case errors.Is(err, idstore.ErrQuaNhieuQuyen):
			h.d.Log.Error("danh mục quyền vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhMucQuyen)
		case errors.Is(err, idstore.ErrQuaNhieuVaiTro):
			h.d.Log.Error("danh mục vai trò vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranDanhMucVaiTro)
		case errors.Is(err, idstore.ErrQuaNhieuCapQuyen):
			h.d.Log.Error("số ô đã cấp vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", idstore.TranCapQuyen)
		default:
			// The wrapped error carries the store failure and never reaches the client (rule 3,
			// forbidden #3).
			h.d.Log.Error("ma trận phân quyền: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) on all three and not nil slices: every list must marshal as [] on a commune
	// that has not been set up yet, never as null. A newly onboarded commune has no roles and no
	// grants, which is an ordinary state and not an error.
	//
	// THE ORDER IS THE STORE'S on both axes and is not touched here — see gomTheoNhom.
	ra := maTranQuyenRa{
		Groups: gomTheoNhom(mt.DanhMuc),
		Roles:  make([]vaiTroCotRa, 0, len(mt.Cot)),
		Grants: make([]capQuyenRa, 0, len(mt.DaCap)),
	}
	for _, c := range mt.Cot {
		ra.Roles = append(ra.Roles, vaiTroCotRa{
			ID: c.ID, Code: c.Ma, Name: c.Ten, IsLeader: c.LaLanhDao,
			StaffCount: c.SoCanBo, ActiveAccountCount: c.SoTaiKhoanDangHoatDong,
		})
	}
	for _, g := range mt.DaCap {
		ra.Grants = append(ra.Grants, capQuyenRa{RoleID: g.VaiTroID, Permission: g.QuyenMa})
	}
	vietJSON(w, http.StatusOK, ra)
}
