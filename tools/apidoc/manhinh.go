package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Reading "has this route got a screen yet" OUT OF THE WEB SOURCE, instead of waiting for
// somebody to rename a file.
//
// # WHY THIS REPLACES THE MANUAL CLAIM
//
// The queue used to learn that a route was finished in exactly one way: an agent renaming
// open/<id>.json to done/<id>.json. That rename is a signal somebody has to REMEMBER TO TYPE,
// and a signal you have to remember to type is a signal that drifts — silently. Measured on
// 2026-09-23: all 36 tasks then sitting in open/ already had client code. Every single entry in
// the queue was a lie, and a queue that is believed while lying is worse than no queue: the same
// day, one module ledger nearly handed out a whole batch of finished work a second time.
//
// The rename is NOT removed. open/ -> claimed/ still says "somebody is on it right now", and
// only a person can say that. What moves here is the one fact the source code already holds:
// whether the client call exists.
//
// # WHAT COUNTS AS EVIDENCE
//
// Two signals, either one is enough. Neither alone covers the corpus, measured:
//
//	operation id   `petitions_get_tasks_by_ma["duongDan"]` — the client annotates the path
//	               variable with the type generated FOR THAT EXACT ROUTE. Method-precise,
//	               because maOperation puts the method in the name.
//	path literal   a quoted string equal to the route path, `"/api/v1/tasks/{ma}"`, plus the
//	               method spelled as a string somewhere in the SAME FILE (GET needs nothing:
//	               it is the default of every fetch wrapper). It catches the routes that share
//	               a path with another method and therefore borrow that method's generated
//	               type, e.g. POST /api/v1/announcements is annotated
//	               `comms_get_announcements["duongDan"]` while really calling goiGhi(…,"POST").
//
// BOTH, NOT THE TIDIER ONE, and that is a measurement rather than a preference. Over the 113
// routes on 2026-09-24, 100 have a client call: 2 are found only by the operation id, 6 only by
// the path literal. Either signal alone leaves a finished screen sitting in open/ forever.
//
// # THE APPROXIMATION, STATED RATHER THAN IMPLIED
//
// This is TEXT MATCHING over comment-stripped TypeScript, not a TypeScript parse. Go has no
// TS parser in its standard library and pulling one in to answer a yes/no question would be a
// dependency nobody can audit. The limits, each one measured on this repository:
//
//  1. A PATH BUILT BY CONCATENATION IS INVISIBLE. `const duongDan = ${goc}/${id}/conclusions`
//     (bien-ban.ts:177) contains no literal equal to /api/v1/meetings/{id}/conclusions. That
//     route is detected only because its test file spells the path out in full. Direction of
//     the error: the task stays in open/ though the screen exists — annoying, self-correcting,
//     and VISIBLE. The opposite error is the dangerous one.
//  2. THE METHOD CHECK IS PER FILE, NOT PER CALL SITE. A file holding a GET and a POST on the
//     same path marks both as done as soon as either is built. Narrowing it further would mean
//     hard-coding this repository's fetch wrapper names into a generator, which is a second
//     copy of a fact owned by web-admin.
//  3. schema.gen.ts IS EXCLUDED, and that exclusion is load-bearing. It is generated FROM the
//     contract, so it names every path and every operation id in the repository. Scanning it
//     answers "does this route exist", not "did anybody build it" — with it in scope the
//     detector reported 42 of 42 open tasks finished, i.e. exactly the lie being fixed.
//  4. ONLY web-admin/src/lib/api IS SCANNED. A call made from anywhere else is invisible:
//     GET /api/v1/communes/current lives in web-admin/src/lib/tenant-config.ts, and its task
//     sits in done/ only because somebody moved it by hand back when that still happened.
//     Widening the scope to all of src/ was measured and found that ONE extra route at the cost
//     of ~135 extra files, so the narrow scope stays. This paragraph is the record that it is a
//     choice, not an oversight — and the one place to change if a second such call appears.
//  5. A REGEX LITERAL CONTAINING // OR /* WOULD BE READ AS A COMMENT and blank the rest of the
//     line. There is none in scope today (the only regexes are /\s+/g and /docDanhMuc…\(/g).
//     If one appeared, the effect is a swallowed line — a missed detection, never a false one.
//
// Every limit above errs the same way: it may leave a finished task in open/. None of them can
// move an unbuilt task into done/, which is the failure that would silently drop a screen.
var thuMucManHinh = filepath.Join("web-admin", "src", "lib", "api")

// tepHopDong is generated from openapi.json and names every route in the repository, so it is
// evidence of nothing. See limit 3 above — this exclusion is why the detector is not a tautology.
const tepHopDong = "schema.gen.ts"

// manHinh is the admin web's client layer, with every comment blanked out.
//
// Keyed by path relative to the repository root, because the only thing a caller ever does with
// the key is print it in a message a person reads.
type manHinh struct {
	nguon map[string]string
}

// docManHinh loads the client layer.
//
// AN EMPTY RESULT IS AN ERROR, NOT AN ANSWER — the same shape as dongBoViec's safety net 2.
// Zero files means the directory moved, the tool is being run from somewhere unexpected, or the
// scan is broken; and the observable consequence of each of those is identical to "no screen was
// ever built", which would freeze the whole queue in open/ while looking perfectly healthy.
func docManHinh(goc string) (manHinh, error) {
	thuMuc := filepath.Join(goc, thuMucManHinh)
	m := manHinh{nguon: map[string]string{}}

	err := filepath.WalkDir(thuMuc, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ten := d.Name()
		if ten == tepHopDong {
			return nil
		}
		if ext := filepath.Ext(ten); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(goc, p)
		if err != nil {
			rel = p
		}
		m.nguon[filepath.ToSlash(rel)] = boChuThichTS(string(b))
		return nil
	})
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return manHinh{}, fmt.Errorf(
			"không đọc được lớp gọi API của web-admin ở %s — hàng đợi việc suy trạng thái từ đó, "+
				"nên thiếu nó thì MỌI việc sẽ đứng nguyên ở open/ mà không có gì báo", thuMucManHinh)
	case err != nil:
		return manHinh{}, fmt.Errorf("đọc %s: %w", thuMucManHinh, err)
	}
	if len(m.nguon) == 0 {
		return manHinh{}, fmt.Errorf(
			"%s không có tệp .ts nào — đó là bộ quét hỏng, không phải một lớp gọi API trống", thuMucManHinh)
	}
	return m, nil
}

// daDung answers whether the admin web already calls this route. See the block comment above
// for what the two signals are and for what this deliberately cannot see.
func (m manHinh) daDung(t tuyen) bool {
	op := maOperation(t)
	for _, s := range m.nguon {
		if coDinhDanh(s, op) {
			return true
		}
	}
	for _, s := range m.nguon {
		if coChuoi(s, t.Path) && coPhuongThuc(s, t.Method) {
			return true
		}
	}
	return false
}

// coPhuongThuc looks for the HTTP method written as a string in the same file.
//
// GET IS EXEMPT ON PURPOSE. It is the default of fetch and of every wrapper built on it, so a
// GET call site has no reason to spell the word — demanding it would reject every read route.
func coPhuongThuc(nguon, phuongThuc string) bool {
	if phuongThuc == http.MethodGet {
		return true
	}
	return coChuoi(nguon, phuongThuc)
}

// coChuoi reports whether the source contains `v` as a complete string literal, in any of the
// three TypeScript quotes.
//
// COMPLETE, not a substring: /api/v1/tasks must not be answered by /api/v1/tasks/{ma}. Two
// routes whose paths nest are two different screens.
func coChuoi(nguon, v string) bool {
	for _, q := range []string{`"`, `'`, "`"} {
		if strings.Contains(nguon, q+v+q) {
			return true
		}
	}
	return false
}

// coDinhDanh reports whether `id` occurs as a whole identifier — not glued to a longer name.
//
// Without the boundary check, identity_get_staff would be found inside
// identity_get_staff_by_id, and a route would be marked finished by its neighbour's screen.
func coDinhDanh(nguon, id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i+len(id) <= len(nguon); {
		j := strings.Index(nguon[i:], id)
		if j < 0 {
			return false
		}
		j += i
		truoc := j == 0 || !kyTuTen(nguon[j-1])
		sau := j+len(id) == len(nguon) || !kyTuTen(nguon[j+len(id)])
		if truoc && sau {
			return true
		}
		i = j + 1
	}
	return false
}

func kyTuTen(b byte) bool {
	return b == '_' || b == '$' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// boChuThichTS blanks the CONTENT of every TypeScript comment while keeping every character
// position and every newline.
//
// WHY BLANK RATHER THAN DELETE: exactly the shape tools/check_khoa_duy_nhat.py settled on for
// SQL (commit b0d3ff0). Deleting text shifts every offset after it, so any line number or
// neighbouring-character test computed on the result is quietly wrong; padding with spaces of
// the same length keeps the file a faithful map of itself.
//
// WHY IT IS NEEDED AT ALL: the comment blocks in this repository's client layer are unusually
// rich — nhiem-vu.ts opens with a table listing all eight task routes, paths and permissions
// included, and phieu-phan-anh.ts and bien-ban.ts do the same. A bare string search reads that
// prose as eight finished screens. This is the defect that had just been measured next door:
// 17 of 58 UNIQUE matches in check_khoa_duy_nhat.py were words in comments, and the threshold
// built on that inflated number sat ABOVE the real one.
//
// A HAND-ROLLED SCANNER, NOT A PARSER, and the cases it gets wrong are listed in limit 5 of the
// block comment at the top of this file. It tracks four states — code, //, /* */, and string —
// and treats a template literal as an opaque string, so a nested `${…}` containing a backtick
// would end it early. None exists in scope.
func boChuThichTS(nguon string) string {
	b := []byte(nguon)
	xoa := func(i int) {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}

	const (
		ma   = iota // in code
		dong        // inside a // comment
		khoi        // inside a /* */ comment
	)
	trong := ma
	var nhay byte // the quote character when inside a string literal, 0 in code

	for i := 0; i < len(b); i++ {
		switch {
		case trong == dong:
			if b[i] == '\n' {
				trong = ma
				continue
			}
			xoa(i)

		case trong == khoi:
			if b[i] == '*' && i+1 < len(b) && b[i+1] == '/' {
				xoa(i)
				xoa(i + 1)
				i++
				trong = ma
				continue
			}
			xoa(i)

		case nhay != 0:
			// An escape hides the next character, including a closing quote.
			if b[i] == '\\' {
				i++
				continue
			}
			if b[i] == nhay {
				nhay = 0
			}

		case b[i] == '/' && i+1 < len(b) && b[i+1] == '/':
			trong = dong
			xoa(i)
			xoa(i + 1)
			i++

		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			trong = khoi
			xoa(i)
			xoa(i + 1)
			i++

		case b[i] == '"' || b[i] == '\'' || b[i] == '`':
			nhay = b[i]
		}
	}
	return string(b)
}
