package domain

// MapAssetTypeDefault is one of the eleven groups POST /api/v1/map-asset-types/defaults sows into a
// commune's catalogue.
type MapAssetTypeDefault struct {
	Code  string // `ma`, ADR 0011 slug — the value asset records store, never renumbered
	Label string // `nhan`, the commune may re-word it later
	Order int    // `thu_tu`, the spec's order
}

// DefaultMapAssetTypes returns the eleven groups, in the spec's order.
//
// THE OWNER OF THIS LIST IS ADR 0072 §3 (kb/10-decisions/0072-ban-do-kinh-te-so-nen-tu-host.md, the
// `ma` / label table): codes and labels are copied from that table and nowhere else. Changing one here
// without changing the ADR is a second source for the list; changing a CODE after any commune has
// sown it is impossible (rule 7, invariant 3 — asset records hold it as a value).
//
// A FUNCTION, NOT A PACKAGE VARIABLE: a caller that appended to a shared slice would change the list
// every later run sows.
func DefaultMapAssetTypes() []MapAssetTypeDefault {
	return []MapAssetTypeDefault{
		{Code: "doanh-nghiep", Label: "Doanh nghiệp", Order: 1},
		{Code: "ho-kinh-doanh", Label: "Hộ kinh doanh", Order: 2},
		{Code: "hop-tac-xa", Label: "Hợp tác xã", Order: 3},
		{Code: "cho", Label: "Chợ, trung tâm thương mại", Order: 4},
		{Code: "truong-hoc", Label: "Trường học", Order: 5},
		{Code: "co-so-y-te", Label: "Cơ sở y tế", Order: 6},
		{Code: "di-tich", Label: "Di tích lịch sử – văn hoá", Order: 7},
		{Code: "du-lich-lang-nghe", Label: "Du lịch, làng nghề", Order: 8},
		{Code: "ocop", Label: "Sản phẩm OCOP", Order: 9},
		{Code: "ha-tang", Label: "Hạ tầng công cộng", Order: 10},
		{Code: "cong-trinh-dau-tu-cong", Label: "Công trình đầu tư công", Order: 11},
	}
}
