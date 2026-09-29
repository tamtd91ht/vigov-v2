package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"

	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/capital-plan-categories is the FIRST route in this service.
// Five things have to hold, and each fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestCategories_200WithoutSettingsPermission for what the "403 wrong permission" case becomes here;
//  2. one commune's catalogue never reaches another commune's caller;
//  3. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  4. a commune with no rows serialises as [] and not null — which is EVERY commune today;
//  5. the response carries the five fields of the contract and nothing more.

const categoryPath = "/api/v1/capital-plan-categories"

func readCategoryList(t *testing.T, body []byte) danhSachHangMucRa {
	t.Helper()
	var result danhSachHangMucRa
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(body))
	}
	return result
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestCategories_401WithoutSession(t *testing.T) {
	m := newTestServer(t)

	wantStatus(t, m.call(t, "GET", hostA, categoryPath, nil), http.StatusUnauthorized)
	if m.categories.calls != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục hạng mục kế hoạch vốn")
	}
}

func TestCategories_200WithoutSettingsPermission(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision this route was declared with, so it is asserted rather than assumed.
	//
	// fakeChecker grants NOTHING, to anybody, in any commune. The account must still get 200. That
	// is what AnyAuthenticated means here, and it is the whole reason the route is declared that
	// way: category names fill the classifier on a capital plan line and every filter beside it, so
	// a configuration permission on this list would empty those boxes for everybody who is not an
	// administrator.
	//
	// If somebody later "tightens" this to RequirePermission, this test goes red — which is the
	// point. The trade-off it protects is stated on the route: the catalogue of ONE commune is
	// readable by every signed-in account OF THAT COMMUNE.
	m := newTestServer(t)

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)
	if len(readCategoryList(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
}

func TestCategories_401OtherCommune(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here: there is no permission to be
	// right or wrong about, so what remains is an account of commune A presenting itself at commune
	// B's domain. authz.AnyAuthenticated compares the principal's commune with the one resolved
	// from Host and refuses — BEFORE the store is touched, which is what makes another commune's
	// catalogue unreachable rather than merely unrequested.
	//
	// THE ERROR `code` IS DELIBERATELY NOT ASSERTED. Today the refusal comes from authz, which
	// answers "unauthorized". When this service gets its own token layer, the refusal will move
	// earlier and answer "tenant_mismatch" the way the identity service already does — a correct
	// improvement that an assertion here would make look like a regression. What must not change is
	// the pair below: refused, and nothing read.
	m := newTestServer(t)

	w := m.call(t, "GET", hostB, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusUnauthorized)
	if m.categories.calls != 0 {
		t.Error("tài khoản của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestCategories_200(t *testing.T) {
	m := newTestServer(t)

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	result := readCategoryList(t, w.Body.Bytes())
	if len(result.Items) != 3 {
		t.Fatalf("nhận %d hạng mục, muốn 3", len(result.Items))
	}
	// `Source` AND `Tier` ARE IN THE EXPECTED VALUE, not left to a zero. The fixture row carries
	// nguon = "don-vi", so the tier is 1 — and tier 1 is the ONLY tier the configuration screen may
	// offer `Xoá` on. A mapper that dropped either field would answer tier 0 here, which is no tier
	// at all, and the screen would draw buttons from it.
	if result.Items[0] != (hangMucRa{
		ID: "hm-001", Code: "xay-dung-moi", Label: "Xây dựng mới", IsDefault: true, Active: true,
		Source: domain.SourceCommune, Tier: int(domain.TierCommune),
	}) {
		t.Errorf("hạng mục đầu sai: %+v", result.Items[0])
	}

	// THE TWO BOOLEANS ARE ADJACENT AND A SWAP BETWEEN THEM IS INVISIBLE on any row where they
	// agree. hm-002 is the row where they do not: swapped, it would report a row the commune took
	// out of use as the one the form pre-selects, and nothing on any screen would say so.
	if result.Items[1].IsDefault || !result.Items[1].Active {
		t.Errorf("hm-002: is_default/active đã bị hoán đổi: %+v", result.Items[1])
	}
}

// --- (2) one commune's catalogue never reaches another -------------------------------------------

func TestCategoriesNeverCrossToOtherCommune(t *testing.T) {
	// The same person, signed in properly at commune B. Commune A's catalogue must not travel with
	// them. Nothing about the request is malformed — this is the shape a leak actually takes, and
	// the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := newTestServer(t)

	w := m.call(t, "GET", hostB, categoryPath, staffOf(tenantB))
	wantStatus(t, w, http.StatusOK)

	body := w.Body.String()
	if strings.Contains(body, "xay-dung-moi") || strings.Contains(body, "hm-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", body)
	}
	result := readCategoryList(t, w.Body.Bytes())
	if len(result.Items) != 1 || result.Items[0].ID != "hm-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", result.Items)
	}
}

// --- (3) whole list, store's order, or refused ---------------------------------------------------

func TestCategoriesKeepStoreOrder(t *testing.T) {
	// `thu_tu` IS THE ORDER THE COMMUNE ARRANGED ITS OWN CATALOGUE IN. The store sorts by it; a
	// handler that re-sorted — alphabetically, by id, by anything — would silently overrule the
	// commune on its own catalogue. The fixture is deliberately NOT in alphabetical order, by code
	// or by label, so any re-sort turns this red.
	m := newTestServer(t)

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	result := readCategoryList(t, w.Body.Bytes())
	label := []string{result.Items[0].Code, result.Items[1].Code, result.Items[2].Code}
	want := []string{"xay-dung-moi", "cai-tao-nang-cap", "tra-no"}
	for i := range want {
		if label[i] != want[i] {
			t.Fatalf("thứ tự đã bị đổi: %v, muốn %v", label, want)
		}
	}
	// Named explicitly so the failure above cannot be read as "the fixture happened to be sorted".
	ordered := append([]string(nil), label...)
	sort.Strings(ordered)
	if ordered[0] == label[0] && ordered[1] == label[1] && ordered[2] == label[2] {
		t.Fatal("fixture đang xếp theo bảng chữ cái — ca này không còn phân biệt được handler có xếp lại hay không")
	}
}

func TestDisabledCategoryStaysInList(t *testing.T) {
	// A ROW OUT OF USE IS STILL RETURNED, carrying active:false. Two reasons, and the second is the
	// expensive one: the catalogue screen lists it with a "Đã tắt" chip, and a capital plan line
	// recorded in an earlier budget year still holds that code AS A VALUE — a reader that never saw
	// the row would render an existing figure with no category name at all.
	//
	// What must NOT be returned is a SOFT-DELETED row, and that is a different question, settled in
	// SQL (rule 7, invariant 2). This route never sees one.
	m := newTestServer(t)

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	result := readCategoryList(t, w.Body.Bytes())
	var found bool
	for _, one := range result.Items {
		if one.ID == "hm-003" {
			found = true
			if one.Active {
				t.Errorf("hm-003 đã tắt mà trả về active:true: %+v", one)
			}
		}
	}
	if !found {
		t.Error("hạng mục đã tắt bị loại khỏi danh sách — màn hình danh mục mất dòng, và dòng kế hoạch cũ mất tên hạng mục")
	}
}

func TestCategoriesPastCeilingAreRefusedNotTruncated(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N categories.
	//
	// This list fills the classifier on a capital plan line, and those figures are totalled and
	// reported upward. A silently short list is a category that has disappeared from that picker —
	// the line is filed under the wrong heading or under none, and every screen looks entirely
	// normal. A refusal breaks one commune's screen loudly and names itself in the log.
	//
	// The ceiling itself lives in the store (fistore.MaxCapitalPlanCategories) because only the store
	// knows the LIMIT; what is asserted here is that the handler does not quietly render the error
	// away.
	m := newTestServer(t)
	m.categories.err = fistore.ErrTooManyCapitalPlanCategories

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusInternalServerError)

	if e := errorBody(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	// No partial list came back with the error. A body carrying items alongside a 500 is how a
	// refusal turns back into a truncation on a client that reads the body anyway.
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestCategoriesStoreErrorIs500WithoutLeakingIt(t *testing.T) {
	m := newTestServer(t)
	m.categories.err = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusInternalServerError)

	if e := errorBody(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) the empty catalogue — the ordinary case today -------------------------------------------

func TestCategoriesCommuneWithNoRowsReturnsEmptyArray(t *testing.T) {
	// [] AND NOT null, AND THIS IS NOT AN EDGE CASE: the table ships empty for every commune, on
	// purpose (0003_danh_muc_hang_muc_ke_hoach_von.sql). Until commune onboarding sows the rows,
	// this is what the route returns for everybody, so it is the shape most likely to reach a real
	// client — and a client that has to handle both [] and null handles one of them wrong.
	m := newTestServer(t)
	m.categories.byTenant[tenantA] = nil

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- (5) the contract shape ----------------------------------------------------------------------

func TestCategoriesReturnOnlyContractFields(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, so their absence is asserted rather than assumed.
	//
	//	tenant_id    never leaves this service — it is not data, it is the dimension every row is
	//	             already filtered by (rule 1, invariant 4)
	//	deleted_at   a soft-deleted row never leaves the store, so no reader needs to ask
	//
	// `order`, `source` AND `tier` ARE NOW IN THE CONTRACT, and the comment that used to stand here
	// said the opposite: that `thu_tu` invites a client to re-sort, and that `nguon` /
	// `ma_nguon_re_nhanh` describe buttons open question #21 had not settled. THAT READING IS OUT
	// OF DATE — #21 is about the TASK STATUS catalogue, whose codes a fixed state machine walks;
	// this catalogue's own answer is in the schema and is enforced by a trigger
	// (0003_danh_muc_hang_muc_ke_hoach_von.sql:74-76). The write routes exist, and the
	// configuration screen has to know which buttons it may draw: `Tắt` is refused at tier 3,
	// `Xoá` at tiers 2 and 3.
	//
	// The re-sort argument still stands as an instruction to CLIENTS and is written on the field
	// itself; it was never an argument for hiding the value from a screen that edits it.
	m := newTestServer(t)

	w := m.call(t, "GET", hostA, categoryPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// `label`, NOT `name` — the field is `nhan`, and ADR 0017 names a contract field after what the
	// data IS. Every ADR 0024 catalogue in this system answers `label`; entities with a `ten` column
	// answer `name`. This literal is where a drift back to `name` turns red.
	want := map[string]bool{
		"id": true, "code": true, "label": true, "is_default": true, "active": true,
		"order": true, "source": true, "tier": true,
	}
	for _, one := range raw.Items {
		for key := range one {
			if !want[key] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", key, w.Body.String())
			}
		}
		if len(one) != len(want) {
			t.Errorf("thiếu trường: có %v, muốn %v", one, want)
		}
	}
}
