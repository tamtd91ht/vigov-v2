package store_test

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Tests for page.KindRef — a sort whose key never travels in the cursor.
//
//	PROVED HERE   a walk by a ref key visits every row of the commune exactly once, across a tie
//	              that straddles a page boundary (a3/a4 share ngay_tao) · the cursor carries the id and
//	              NO key value · the anchor is looked up in the request's commune: an id of another
//	              commune reads nothing · a derived-table relation is refused · a cursor that smuggles
//	              a key is refused by page.Decode.
//
//	NOT PROVED    PostgreSQL's evaluation of the scalar subquery — the fake engine in page_test.go
//	              models it; the service's own pg test exercises the real one.

var (
	colCreatedRef = page.Col("created_ref", "ngay_tao", page.KindRef)
	refAllowlist  = page.NewAllowlist(page.Asc, colCreatedRef)
	refAnchors    = store.NewMoc[phanAnh](refAllowlist, map[string]func(phanAnh) page.Key{
		"created_ref": func(phanAnh) page.Key { return page.RefKey() },
	})
)

func refPage(t *testing.T, b *banThu, xa tenant.ID, query string, spec store.PageSpec) (page.Result[phanAnh], error) {
	t.Helper()
	ctx := tenant.Into(context.Background(), xa)
	q, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("chuỗi truy vấn hỏng: %v", err)
	}
	yc, err := page.Parse(q, refAllowlist)
	if err != nil {
		return page.NewResult[phanAnh](), err
	}
	return store.QueryPage(ctx, b.db.For(ctx), spec, yc, refAnchors, quet)
}

func TestRefKeyWalkVisitsEveryRowOnceAndCursorCarriesNoKey(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		t.Run(order, func(t *testing.T) {
			b := dungBanThu(t)
			seen := map[string]int{}
			cursor := ""
			for guard := 0; guard < 10; guard++ {
				q := "limit=3&order=" + order
				if cursor != "" {
					q += "&cursor=" + url.QueryEscape(cursor)
				}
				res, err := refPage(t, b, xaA, q, spec())
				if err != nil {
					t.Fatalf("trang: %v", err)
				}
				for _, it := range res.Items {
					seen[it.ID]++
				}
				if !res.HasMore {
					break
				}
				raw, err := base64.RawURLEncoding.DecodeString(res.NextCursor)
				if err != nil {
					t.Fatalf("con trỏ không giải được: %v", err)
				}
				// THE CURSOR NAMES THE ROW AND NOTHING ELSE: an empty key, and no trace of the value.
				if !strings.Contains(string(raw), `"k":""`) || strings.Contains(string(raw), "2026-03-14") {
					t.Fatalf("con trỏ mang giá trị khoá: %s", raw)
				}
				cursor = res.NextCursor
			}
			if len(seen) != len(idCuaA) {
				t.Fatalf("duyệt được %d dòng, muốn %d", len(seen), len(idCuaA))
			}
			for id, n := range seen {
				if n != 1 {
					t.Errorf("dòng %s hiện %d lần", id, n)
				}
			}
		})
	}
}

func TestRefKeyAnchorOfAnotherCommuneReadsNothing(t *testing.T) {
	b := dungBanThu(t)
	// A well-formed ref cursor naming commune B's row, replayed at commune A.
	forged := page.Encode(colCreatedRef, page.Asc, page.Anchor{Key: page.RefKey(), ID: "01JB0000000000000000000001"})

	res, err := refPage(t, b, xaA, "cursor="+url.QueryEscape(forged), spec())
	if err != nil {
		t.Fatalf("trang: %v", err)
	}
	if len(res.Items) != 0 {
		t.Errorf("con trỏ của xã khác trả %d dòng, muốn 0", len(res.Items))
	}
	l := b.ghi.chua("SELECT r.ngay_tao FROM phan_anh r WHERE r.tenant_id = $1")
	if l == nil || l.args[0] != string(xaA) {
		t.Fatalf("mốc không được tra trong xã của yêu cầu: %+v", b.ghi.tatCa())
	}
}

func TestRefKeyRefusesDerivedTable(t *testing.T) {
	b := dungBanThu(t)
	cursor := page.Encode(colCreatedRef, page.Asc, page.Anchor{Key: page.RefKey(), ID: idCuaA[0]})
	s := spec()
	s.Table = "(SELECT * FROM phan_anh WHERE tenant_id = $1) AS pa"

	if _, err := refPage(t, b, xaA, "cursor="+url.QueryEscape(cursor), s); err == nil {
		t.Fatal("chấp nhận tra mốc trên một bảng dẫn xuất")
	}
	if len(b.ghi.tatCa()) != 0 {
		t.Error("đã chạy câu lệnh dù bị từ chối")
	}
}

func TestRefCursorCarryingAKeyIsRefused(t *testing.T) {
	// Hand-built: kind `ref` with a non-empty key — a value smuggled into a cursor that must carry none.
	smuggled := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":1,"c":"created_ref","d":"asc","t":"ref","k":"Tiêu đề lộ ra","i":"` + idCuaA[0] + `"}`))
	if _, err := page.Decode(smuggled, colCreatedRef, page.Asc); err == nil {
		t.Fatal("con trỏ theo dòng mang khoá mà vẫn được nhận")
	}
}
