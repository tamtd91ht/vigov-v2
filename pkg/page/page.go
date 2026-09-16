// Package page bounds every list route to one page, and makes "the next page" mean the same
// thing in all eight services.
//
// WHY THIS EXISTS BEFORE THE FIRST LIST ROUTE: .claude/skills/rest-api-design/SKILL.md §5
// requires cursor pagination on every list, and nothing implemented it. Eight services each
// inventing a cursor is eight cursor shapes, eight definitions of "the next page", and eight
// chances to get the one thing wrong that nobody notices — a sort with no tie-break, which
// silently skips or repeats records between pages.
//
// WHAT A CURSOR IS HERE: the opaque encoding of ONE anchor — (sort key, id) of the last row
// handed out. Not an offset. A client that can build an offset chooses where to start reading;
// a client that can build an anchor chooses the same thing, which is why the value is opaque
// and is decoded into a TYPED value that is BOUND as a parameter, never concatenated into SQL.
//
// WHAT A CURSOR IS DELIBERATELY NOT:
//
//   - It is NOT signed. A cursor is not a credential: a forged one only yields a different page
//     of the SAME commune's data, because the commune comes from the context and is bound to $1
//     by pkg/store, never from here. Signing it would suggest it carries authority it does not.
//   - It NEVER carries tenant_id. A cursor naming its own commune is a client naming its own
//     commune (rule 1, forbidden #2), and it would survive being pasted into another commune's
//     domain, which is the exact shape of a breach between two authorities.
//   - It carries no personal data (rule 3, forbidden #4): the anchor is a sort key and an id, and
//     an allowlist that made a phone number or a full name sortable would put it in a URL, an
//     access log and a browser history. Sort on ngay_tao / ma, never on a person's attributes.
//
// The response shape is items · next_cursor · has_more, and there is deliberately no `total` and
// no `page`: a keyset read does not know either without a COUNT(*) over a 32-way partitioned
// table (§5 #4), and reporting a number you did not compute is lying about what you know.
package page

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultLimit and MaxLimit bound every list route in the system.
//
// MaxLimit is a SERVER-SIDE CAP, not a validation rule: a client asking for 10.000 is served
// MaxLimit rows and a cursor to continue with — not an error (§5 #2). That is the opposite of
// the batch gRPC calls, where a silent clamp IS a silent wrong answer because the caller has no
// way to fetch the remainder. Here there is a way, and it is in the response.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Query parameter names. English, per §1 of the REST skill.
const (
	ParamLimit  = "limit"
	ParamCursor = "cursor"
	ParamSort   = "sort"
	ParamOrder  = "order"
)

// Kind is the type of a sort key. The set is closed on purpose: every member has to survive the
// round trip through a cursor and come back as a value the driver can bind with the column's own
// type. Adding one means deciding its textual form first.
type Kind string

const (
	KindTime Kind = "time" // timestamptz — the common case: ngay_tao, cap_nhat_luc
	KindText Kind = "text" // text — a business code such as `ma`
	KindInt  Kind = "int"  // bigint — a counter or a sequence number
)

// NOT SUPPORTED, and the reason, so the next person does not have to rediscover it:
//
//   - float/numeric: equality on a float sort key is not reliable, so the tie-break cannot be
//     trusted, and a cursor that round-trips through decimal text may land between two rows.
//   - bool / low-cardinality enums: a sort key with three distinct values makes the id the real
//     ordering, which is slower than sorting by id and is almost never what the screen wanted.
//   - NULL-able columns: `(col, id) > ($2, $3)` evaluates to NULL when col is NULL, so those rows
//     vanish from every page after the first — silently. A sortable column must be NOT NULL.

// Dir is the sort direction.
type Dir string

const (
	Asc  Dir = "asc"
	Desc Dir = "desc"
)

var (
	// ErrCursor means the cursor did not decode, or does not belong to this sort. Answer 400 and
	// let the client restart from the first page — never panic, and never fall back to "start from
	// the beginning" silently, which shows the citizen page 1 while they believe they are on page 4.
	ErrCursor = errors.New("page: con trỏ không hợp lệ")
	// ErrSort means the requested sort column or direction is not on the route's allowlist.
	ErrSort = errors.New("page: tiêu chí sắp xếp không hợp lệ")
	// ErrLimit means limit was present but was not a positive integer. Note that a limit ABOVE
	// MaxLimit is not an error — it is capped.
	ErrLimit = errors.New("page: limit không hợp lệ")
)

// HTTPError maps a Parse failure onto the single error shape of pkg/httpx.
//
// `code` is an identifier and stays English; `message` is read by a person and is Vietnamese
// (rest-api-design §1). It never quotes what the client sent back at them: the cursor is opaque
// and a rejected sort key is not worth echoing into an access log.
func HTTPError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, ErrCursor):
		return http.StatusBadRequest, "invalid_cursor",
			"Con trỏ phân trang không hợp lệ. Vui lòng tải lại danh sách."
	case errors.Is(err, ErrSort):
		return http.StatusBadRequest, "invalid_sort", "Tiêu chí sắp xếp không hợp lệ."
	case errors.Is(err, ErrLimit):
		return http.StatusBadRequest, "invalid_limit", "Số bản ghi mỗi trang không hợp lệ."
	default:
		return http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại."
	}
}

// --- the allowlist -----------------------------------------------------------------------------

var (
	reSQLIdent  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	reParamName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)
)

// Column is one sortable column, declared IN SOURCE by the route that owns the list.
//
// Param is what a client may send (`?sort=created_at`); SQL is the identifier that reaches the
// statement. They are two fields on purpose: the client-facing name follows the JSON field
// convention (English) while the column is whatever the schema calls it (`ngay_tao`), and the
// client string is only ever COMPARED, never used to build SQL (§5 #5).
type Column struct {
	Param string
	SQL   string
	Kind  Kind
}

// Col declares a sortable column, and panics if the SQL identifier is not a plain identifier.
//
// WHY PANIC: this runs at wiring time, from a constant in source. The only way to reach it with
// anything else is to have piped a client string into an allowlist — the one mistake that would
// turn this package into an injection route. Failing at start-up is the cheapest place to find it.
func Col(param, sqlName string, k Kind) Column {
	if !reParamName.MatchString(param) {
		panic("page: tên tham số sắp xếp không hợp lệ: " + param)
	}
	if !reSQLIdent.MatchString(sqlName) {
		panic("page: tên cột không phải định danh: " + sqlName)
	}
	switch k {
	case KindTime, KindText, KindInt:
	default:
		panic("page: kiểu khoá sắp xếp không hỗ trợ: " + string(k))
	}
	return Column{Param: param, SQL: sqlName, Kind: k}
}

// Allowlist is the closed set of sorts one route offers, plus its default.
type Allowlist struct {
	def    Column
	defDir Dir
	cols   []Column
}

// NewAllowlist declares the default sort first, then any alternatives.
//
// The default is mandatory: a list with no ORDER BY has no stable order at all, so "page 2"
// would be meaningless even before a cursor is involved.
func NewAllowlist(defDir Dir, def Column, rest ...Column) Allowlist {
	if defDir != Asc && defDir != Desc {
		panic("page: chiều sắp xếp mặc định không hợp lệ: " + string(defDir))
	}
	cols := append([]Column{def}, rest...)
	seen := map[string]bool{}
	for _, c := range cols {
		if c.SQL == "" || c.Param == "" {
			panic("page: cột rỗng trong danh sách trắng — dùng page.Col để khai báo")
		}
		if seen[c.Param] {
			panic("page: tham số sắp xếp trùng: " + c.Param)
		}
		seen[c.Param] = true
	}
	return Allowlist{def: def, defDir: defDir, cols: cols}
}

func (a Allowlist) lookup(param string) (Column, bool) {
	for _, c := range a.cols {
		if c.Param == param {
			return c, true
		}
	}
	return Column{}, false
}

// --- the sort key ------------------------------------------------------------------------------

// Key is one sort-key value, carrying its own type.
//
// The fields are unexported so a Key can only be built through a constructor: the type has to
// match the column, because it decides how the value is written into a cursor and how it comes
// back as a bound parameter.
type Key struct {
	kind Kind
	t    time.Time
	s    string
	n    int64
}

func TimeKey(t time.Time) Key  { return Key{kind: KindTime, t: t} }
func TextKey(s string) Key     { return Key{kind: KindText, s: s} }
func IntKey(n int64) Key       { return Key{kind: KindInt, n: n} }
func (k Key) Kind() Kind       { return k.kind }
func (k Key) Time() time.Time  { return k.t }
func (k Key) Text() string     { return k.s }
func (k Key) Int() int64       { return k.n }
func (k Key) String() string   { return k.wire() }
func (k Key) Equal(o Key) bool { return k.kind == o.kind && k.wire() == o.wire() }

// Value is the TYPED value to bind as a parameter. It is the only way out of a Key, which is what
// keeps a decoded cursor off the statement text.
func (k Key) Value() any {
	switch k.kind {
	case KindTime:
		return k.t
	case KindInt:
		return k.n
	default:
		return k.s
	}
}

func (k Key) wire() string {
	switch k.kind {
	case KindTime:
		return k.t.UTC().Format(time.RFC3339Nano)
	case KindInt:
		return strconv.FormatInt(k.n, 10)
	default:
		return k.s
	}
}

func parseKey(kind Kind, raw string) (Key, error) {
	switch kind {
	case KindTime:
		t, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return Key{}, fmt.Errorf("%w: mốc thời gian: %v", ErrCursor, err)
		}
		return TimeKey(t), nil
	case KindInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Key{}, fmt.Errorf("%w: mốc số: %v", ErrCursor, err)
		}
		return IntKey(n), nil
	case KindText:
		if raw == "" {
			return Key{}, fmt.Errorf("%w: mốc rỗng", ErrCursor)
		}
		return TextKey(raw), nil
	default:
		return Key{}, fmt.Errorf("%w: kiểu khoá %q", ErrCursor, kind)
	}
}

// Anchor is the position a cursor encodes: the sort key of the last row handed out, plus its id.
//
// THE ID IS NOT DECORATION. Two records created in the same millisecond sort equal, and without a
// tie-break the next page either repeats one of them or drops the other — silently, with every
// test on distinct timestamps still green.
type Anchor struct {
	Key Key
	ID  string
}

// --- the request -------------------------------------------------------------------------------

// Request is a validated page request. Its fields are unexported so the sort column can only come
// from an Allowlist: a Request that could be assembled field by field would let a client-supplied
// string reach the ORDER BY of pkg/store.
type Request struct {
	col   Column
	dir   Dir
	limit int
	after *Anchor
}

func (r Request) Column() Column { return r.col }
func (r Request) Dir() Dir       { return r.dir }
func (r Request) Limit() int     { return r.limit }

// After reports the anchor to continue from. ok is false on the first page.
func (r Request) After() (Anchor, bool) {
	if r.after == nil {
		return Anchor{}, false
	}
	return *r.after, true
}

// Parse reads ?limit=&cursor=&sort=&order= and validates all four against the allowlist.
//
// Call it BEFORE touching the database: a rejected request must run no statement at all.
func Parse(q url.Values, a Allowlist) (Request, error) {
	return New(a, q.Get(ParamSort), q.Get(ParamOrder), q.Get(ParamLimit), q.Get(ParamCursor))
}

// New builds a Request from raw strings — the one place a client value is turned into a sort.
//
// Empty strings mean "not supplied": the route's default sort, the default direction and
// DefaultLimit. It is exported for callers that are not HTTP handlers (a background job walking a
// commune's records page by page still has to be bounded, and still needs the tie-break).
func New(a Allowlist, sort, order, limit, cursor string) (Request, error) {
	if len(a.cols) == 0 {
		return Request{}, fmt.Errorf("%w: tuyến chưa khai báo danh sách trắng", ErrSort)
	}

	col := a.def
	if sort != "" {
		c, ok := a.lookup(sort)
		if !ok {
			// The name is not echoed back: an unknown sort key is often a probe, and repeating it
			// into the response and the access log helps nobody.
			return Request{}, fmt.Errorf("%w: cột không nằm trong danh sách trắng", ErrSort)
		}
		col = c
	}

	dir := a.defDir
	switch order {
	case "":
	case string(Asc):
		dir = Asc
	case string(Desc):
		dir = Desc
	default:
		return Request{}, fmt.Errorf("%w: chiều sắp xếp phải là asc hoặc desc", ErrSort)
	}

	n, err := parseLimit(limit)
	if err != nil {
		return Request{}, err
	}

	r := Request{col: col, dir: dir, limit: n}
	if cursor != "" {
		anchor, err := Decode(cursor, col, dir)
		if err != nil {
			return Request{}, err
		}
		r.after = &anchor
	}
	return r, nil
}

// parseLimit applies §5 #2 in one place.
//
// ABOVE THE CAP IS NOT AN ERROR — it is capped, and the client gets a cursor to continue with.
// BELOW ONE IS an error, and the asymmetry is deliberate: "give me more than you allow" is a
// legitimate request the server can answer with what it allows, while "give me zero rows" or
// "give me minus five rows" is not a request for anything. Defaulting those to 20 would accept a
// broken client and hide the bug in whatever built the URL.
func parseLimit(raw string) (int, error) {
	if raw == "" {
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%w: không phải số nguyên", ErrLimit)
	}
	if n < 1 {
		return 0, fmt.Errorf("%w: phải lớn hơn 0", ErrLimit)
	}
	if n > MaxLimit {
		return MaxLimit, nil
	}
	return n, nil
}

// --- the wire form -----------------------------------------------------------------------------

// cursorVersion is carried so a future change of shape can be rejected cleanly instead of being
// misread. Cursors are short-lived — a client holds one for the seconds between two pages — so a
// version bump needs no migration, only a clear refusal.
const cursorVersion = 1

// dayCursor is the JSON that gets base64url-encoded. Short keys because it travels in a URL.
//
// There is no tenant field, and there must never be one: see the package comment.
type dayCursor struct {
	V int    `json:"v"`
	C string `json:"c"` // the sort param this cursor belongs to
	D string `json:"d"` // the direction it belongs to
	T string `json:"t"` // the kind of the key
	K string `json:"k"` // the key, in its textual form
	I string `json:"i"` // the id — the tie-break
}

// Encode turns an anchor into the opaque string the client sends back.
func Encode(col Column, dir Dir, a Anchor) string {
	b, err := json.Marshal(dayCursor{
		V: cursorVersion,
		C: col.Param,
		D: string(dir),
		T: string(a.Key.Kind()),
		K: a.Key.wire(),
		I: a.ID,
	})
	if err != nil {
		// Every field is a string or an int; json.Marshal cannot fail on this struct.
		panic("page: không mã hoá được con trỏ: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Decode reverses Encode and checks the cursor belongs to THIS sort.
//
// WHY THE COLUMN AND DIRECTION ARE CHECKED: a cursor produced while sorting by ngay_tao and then
// replayed against a sort by ma anchors the walk at a position that means nothing in the new
// order — the client gets a page that skips records and repeats others, with no error anywhere.
// Refusing costs the client one restart; accepting costs it a list it cannot trust.
//
// Every failure path returns ErrCursor. Nothing here panics on client input: the input is a
// string from a URL, and a malformed one is an ordinary 400.
func Decode(s string, col Column, dir Dir) (Anchor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Anchor{}, fmt.Errorf("%w: không giải được base64", ErrCursor)
	}
	var d dayCursor
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Anchor{}, fmt.Errorf("%w: nội dung không đọc được", ErrCursor)
	}
	if d.V != cursorVersion {
		return Anchor{}, fmt.Errorf("%w: phiên bản %d", ErrCursor, d.V)
	}
	if d.C != col.Param || d.D != string(dir) {
		return Anchor{}, fmt.Errorf("%w: thuộc về một cách sắp xếp khác", ErrCursor)
	}
	if Kind(d.T) != col.Kind {
		return Anchor{}, fmt.Errorf("%w: kiểu khoá không khớp cột", ErrCursor)
	}
	if d.I == "" {
		return Anchor{}, fmt.Errorf("%w: thiếu khoá phá hoà", ErrCursor)
	}
	k, err := parseKey(col.Kind, d.K)
	if err != nil {
		return Anchor{}, err
	}
	return Anchor{Key: k, ID: d.I}, nil
}

// --- the response ------------------------------------------------------------------------------

// Result is the response body of every list route.
//
// NO `total`, NO `page`, NO `page_count` — and this is not an omission to be helpfully filled in
// later. A keyset read never computes them, so any number put here would come from a COUNT(*)
// over a partitioned table on every list request (§5 #4), or from an invention. A total that
// somebody needs is a separate, cached route.
type Result[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"` // empty when has_more is false
	HasMore    bool   `json:"has_more"`
}

// NewResult returns an empty result whose Items marshal as `[]`, never as `null`.
func NewResult[T any]() Result[T] { return Result[T]{Items: []T{}} }
