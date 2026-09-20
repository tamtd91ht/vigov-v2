package http

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// The read route behind the commune's document-type catalogue. GET /api/v1/document-types
//
// THERE IS NO WRITE ROUTE, and no scaffolding for one is left here. Open question #21 — whether a
// commune may edit the CODE LIST itself or only the labels and the order — is unanswered, and a
// write route answers it silently: the first PATCH that lets `ma` through decides that a commune
// may mint its own codes, on a column document records already hold AS A VALUE. The database
// refuses the six operations that would break records (migrations/0003_danh_muc_loai_van_ban.sql,
// danh_muc_ba_tang), which is a floor, not a design.
//
// THE CATALOGUE SHIPS EMPTY, AND AN EMPTY LIST IS THE CORRECT ANSWER. No commune has rows here
// until commune onboarding sows them, and that step does not exist in this repository yet. A
// screen reading this route shows an empty list — visibly empty, which is the failure people
// report, unlike a half-seeded catalogue that looks complete.

// loaiVanBanRa is one catalogue row as it leaves the API.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a document type is how the authority classifies its
// paperwork, not anything about a person. That is what makes this route's AnyAuthenticated
// declaration a question about convenience rather than about privacy — see the reason on the route
// itself.
type loaiVanBanRa struct {
	ID    string `json:"id"`    // ULID — what a document record would reference
	Code  string `json:"code"`  // "cong-van" — the value a document record stores, never renumbered
	Label string `json:"label"` // "Công văn" — the wording the commune may change

	// Active is `dang_dung`. Rows out of use ARE returned, carrying false: the configuration
	// screen lists them with a "Đã tắt" chip, and a document registered under a type since retired
	// still has to render its own label. A picker filling a form offers only the rows with
	// `active: true` — the filtering is the client's, because a second, filtered route would give
	// two answers to "what types does this commune have" and the stale one would reach a screen.
	Active bool `json:"active"`

	// IsDefault is the row a form pre-selects. At most one live row per commune carries it — the
	// schema's generated `moc_mac_dinh` column admits no second — and a commune whose catalogue
	// has not been sown carries none at all, which every caller must handle.
	IsDefault bool `json:"is_default"`
}

// danhSachLoaiVanBanRa wraps the list in an OBJECT rather than returning a bare JSON array.
//
// A bare array cannot grow: the day this needs to say anything about the list itself — that it was
// truncated, when it was last changed — every client has to change shape at once. An object with
// one field costs one line now and nothing later. It is also the shape page.Result already gives
// every other list route, so a client reads `items` on all of them.
//
// THERE IS NO next_cursor AND NO has_more, and their absence is the contract: this route returns
// the WHOLE list or it fails. A `has_more` here would invite exactly the paging behaviour the
// route was designed not to need — see docstore.LoaiVanBanStore.DanhSach.
type danhSachLoaiVanBanRa struct {
	Items []loaiVanBanRa `json:"items"`
}

func loaiVanBanRaNgoai(l domain.LoaiVanBan) loaiVanBanRa {
	return loaiVanBanRa{
		ID:        l.ID,
		Code:      l.Ma,
		Label:     l.Nhan,
		Active:    l.DangDung,
		IsDefault: l.LaMacDinh,
	}
}

// DanhSachLoaiVanBan serves the commune's document-type catalogue. GET /api/v1/document-types
//
// NO AUDIT ENTRY, and that is a decision rather than an omission. Rule 6, invariant 7 audits
// reading FULL personal data and reading ACROSS communes; this is neither — it is a reference list
// of the authority's own classifications, read inside the commune the request arrived in. An entry
// for every dropdown fill would bury the entries that carry legal weight under thousands that
// carry none.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DanhSachLoaiVanBan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	lvb, err := h.d.LoaiVanBan.DanhSach(ctx)
	if err != nil {
		if errors.Is(err, docstore.ErrQuaNhieuLoaiVanBan) {
			// REFUSED, NOT TRUNCATED. This list is what a document is registered under, and
			// numbering follows the type (ADR 0024), so a list missing a type files the document
			// under the wrong one with nothing on the screen to show it. The log line names the
			// commune because that is the only thing an operator can act on — the ceiling is about
			// thirty times the shipped code list, so reaching it means the data is wrong, not that
			// the commune is large.
			h.d.Log.Error("danh mục loại văn bản vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", docstore.TranDanhMucLoaiVanBan)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3).
		h.d.Log.Error("danh mục loại văn bản: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune whose
	// catalogue has not been sown yet, never as null. TODAY THAT IS EVERY COMMUNE — the table ships
	// empty on purpose — so this is the ordinary path, not an edge case, and a client that has to
	// handle both shapes handles one of them wrong.
	//
	// THE ORDER IS THE STORE'S and is not touched here: `thu_tu` is the order the commune arranged
	// its own catalogue in, and a handler that re-sorted would silently overrule it.
	ra := danhSachLoaiVanBanRa{Items: make([]loaiVanBanRa, 0, len(lvb))}
	for _, mot := range lvb {
		ra.Items = append(ra.Items, loaiVanBanRaNgoai(mot))
	}
	vietJSON(w, http.StatusOK, ra)
}
