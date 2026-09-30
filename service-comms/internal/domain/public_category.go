package domain

// THE CATEGORY CHIPS OF THE MINI APP NEWS TAB — which of the commune's own categories a resident is
// shown, and what `category=<id>` may look like on the public list (user decision 2026-09-30).
//
// Pure functions over rows the store already read; no SQL, no I/O. The store answers two questions —
// "the live tree" and "which live categories hold at least one published item" — and the rule that
// turns them into a chip row lives here, where it can be tested without a database.

// PublicCategoryIDMaxLen bounds `category=` before any lookup. Category ids are 26-character ULIDs
// (core/ulid.Do); anything longer names none. The bound is deliberately looser than the ULID shape
// (see ValidPublicCategoryID) — it is the same 64 the detail route uses for `{id}`.
const PublicCategoryIDMaxLen = 64

// ValidPublicCategoryID reports whether s has the SHAPE of a category id: 1..64 characters of ASCII
// letters, digits, `-` or `_`.
//
// NOT A STRICT ULID CHECK, ON PURPOSE. `danh_muc_mini_app.id` is TEXT with no CHECK (migration
// 0006:155), and the only writer today mints ULIDs — but a strict check here would turn any future
// non-ULID id (an import, a portal sync) into a 400 for a category the commune really has. The point of
// validating at all is that garbage, control characters and oversized values are refused BEFORE the
// platform is asked, so the 400/200 split says nothing about which domains are communes. A well-formed
// id that names nothing is an empty page, never an error.
func ValidPublicCategoryID(s string) bool {
	if s == "" || len(s) > PublicCategoryIDMaxLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// CategoriesWithPublishedItems keeps the categories a resident may be offered as a chip: those that
// hold, THEMSELVES OR THROUGH ANY DESCENDANT, at least one id in withItems. Order of `live` is kept.
//
// `live` is the commune's tree with soft-deleted rows already excluded (store.DanhMucMiniAppStore.
// DanhSach). `withItems` is the set of live category ids with ≥1 published item of the requested type.
//
// A LIVE CATEGORY WHOSE CHAIN TO A ROOT PASSES THROUGH A MISSING (soft-deleted) PARENT IS DROPPED, with
// its whole subtree. The chip row is built from the roots down; promoting an orphan to a root would
// show a category the commune never placed at the top, and the list filter (store.DanhSachCongKhai)
// walks the same live edges, so what is reachable here is exactly what a chip can select.
//
// CYCLES ARE NOT POSSIBLE TODAY (the only write is a create, migration 0006:223-228) but this walk
// refuses to loop anyway: a node met twice on one walk up is treated as not rooted.
func CategoriesWithPublishedItems(live []DanhMucMiniApp, withItems []string) []DanhMucMiniApp {
	byID := make(map[string]DanhMucMiniApp, len(live))
	for _, c := range live {
		byID[c.ID] = c
	}

	// rooted memoises "the chain from this id to a root is entirely live".
	rooted := make(map[string]bool, len(live))
	isRooted := func(id string) bool {
		var path []string
		seen := make(map[string]bool)
		ok := false
		for cur := id; ; {
			if v, done := rooted[cur]; done {
				ok = v
				break
			}
			c, found := byID[cur]
			if !found || seen[cur] {
				ok = false
				break
			}
			seen[cur] = true
			path = append(path, cur)
			if c.ChaID == "" {
				ok = true
				break
			}
			cur = c.ChaID
		}
		for _, p := range path {
			rooted[p] = ok
		}
		return ok
	}

	visible := make(map[string]bool, len(withItems))
	for _, id := range withItems {
		if !isRooted(id) {
			continue
		}
		// Mark the category and every ancestor: choosing a parent includes its descendants' items,
		// so a parent with no item of its own is still a chip when a child has one.
		for cur := id; cur != "" && !visible[cur]; cur = byID[cur].ChaID {
			visible[cur] = true
		}
	}

	out := make([]DanhMucMiniApp, 0, len(visible))
	for _, c := range live {
		if visible[c.ID] {
			out = append(out, c)
		}
	}
	return out
}
