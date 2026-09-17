// Command apidoc turns the route declarations in services/*/internal/ into two generated
// artefacts, and into a work queue the web team can pull from.
//
// # WHY THIS EXISTS
//
// The admin web apps are built in parallel with the backend, and they need two things this
// repository did not have:
//
//  1. A TYPE CONTRACT FOR REST. `.proto` is the source of truth for inter-service contracts,
//     but nothing described the HTTP surface the browser actually calls. Left alone, the web
//     types every response by hand — which is exactly the v1 failure recorded in
//     `.claude/agents/ROUTING.md:229`: "the canonical type source lived in the frontend, in
//     three hand-copied versions". Three copies of one shape drift, and the one that drifts is
//     the one a screen is built from.
//  2. A WAY TO SEE WHICH API IS FINISHED. Without it the web asks in chat, and the answer is
//     whatever somebody remembers.
//
// Both answers are GENERATED, never hand-written, because "which routes exist, with which
// permission, returning which shape" is derivable from the code — rule 9's one-line test.
// The only thing a person writes is the part no tool can derive: the intent behind the route.
//
// # WHAT A PERSON WRITES
//
// A comment block IMMEDIATELY above the route registration — no blank line between, the same
// discipline `authz` and `idem` already impose by living in the same statement. A declaration
// kept in a separate file drifts from the route it describes; one kept touching it cannot.
//
//	// @summary  Đăng nhập bằng email và mật khẩu
//	// @screen   15-phu-luc-giao-dien-chung §1
//	// @request  thanDangNhap
//	// @reply    201 phanHoiDangNhap
//	// @reply    401 httpx.Error
//	mux.Handle("POST /api/v1/sessions", ...)
//
//	@summary  required. One line, Vietnamese — it is read by people.
//	@screen   optional. Points into docs/ui-ux/. NOT derivable: it is design intent.
//	@request  optional. A Go type name. Absent means the route takes no body.
//	@reply    required, repeatable. "<status> <type>", or "<status> -" for an empty body.
//
// Type names are resolved through the AST of the package the route lives in — unqualified
// against that package, qualified (`httpx.Error`) through its imports. Nothing is looked up by
// string matching, so a renamed type fails loudly instead of silently producing a stale shape.
//
// The permission and the duplicate-request mode are NOT annotated. They are read from the
// `authz.*` and `idem.*` calls in the statement itself: they already exist in the code, and a
// second hand-written copy would be a second thing to drift (rule 9, forbidden #2).
//
// # WHAT IT REFUSES TO DO
//
// A struct field with no `json` tag is an ERROR, not a guess. encoding/json would emit it
// under its Go name, but inferring that here would publish a field name nobody chose — a
// contract that is wrong while looking right, which is worse than a missing one.
//
// A field whose name suggests a credential (MatKhau, password, token, secret, khoa, …) and
// which is NOT excluded with `json:"-"` is an ERROR when the type appears in a REPLY. A
// published response shape containing a password teaches every reader to expect one there,
// and the reader who acts on it is the one who logs it (rule 3, rule 8).
//
// In a REQUEST-ONLY shape the same field is allowed — the sign-in form has to send a password,
// and refusing to describe it would leave the web guessing the one shape it must get right.
// It is emitted `writeOnly: true` so no generator ever echoes it back, and the fact is printed
// to stderr on every run. A type reachable from BOTH a request and a reply is judged as a
// reply: fail closed.
//
// # THE WORK QUEUE — tasks/web/
//
//	tasks/web/open/<id>.json      generated here, never typed by hand
//	tasks/web/claimed/<id>.json
//	tasks/web/done/<id>.json
//	tasks/web/stale/<id>.json     route gone from the source while nobody had started
//
// <id> is a deterministic short hash of `service|METHOD|path`. The same route keeps the same
// id forever, and a task is created only when the id is absent from ALL FOUR directories —
// so rerunning this generator never resurrects finished work, and "already done" is visible
// from the filesystem without a second ledger to keep in sync.
//
// When a route DISAPPEARS the answer depends on where its task sits, and dongBoViec documents
// why the four cases cannot be treated alike. The short version: open/ is moved to stale/
// (moved, never deleted), claimed/ is left alone and shouted about, done/ is left in peace, and
// a route that comes back moves its own file from stale/ straight back to open/.
//
// CLAIMING A TASK IS `os.Rename` BETWEEN TWO DIRECTORIES, AND NOTHING ELSE. The operating
// system is the lock: two agents renaming the same file, one succeeds and the other gets
// ENOENT. Do not add a lock file, a status field, or a database row — each of those is a
// second source for a fact the filesystem already holds, and the copy that drifts is the one
// that decides two agents are both working on the same route.
//
//	open/<id>.json  --rename-->  claimed/<id>.json  --rename-->  done/<id>.json
//
// Only the admin web is queued here. The Zalo Mini App resolves its commune differently and
// its surface is not designed yet; an empty tasks/citizen/ would be a promise about work
// nobody has scoped.
//
// # DETERMINISM
//
// Every output is byte-identical across runs on unchanged source: keys are emitted in a fixed
// order, lists are sorted, and nothing carries a timestamp, a commit hash or a line number. A
// generated file that changes on every run is a file whose diff nobody reads — and the one
// real change then goes past unseen.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// canhBao is the first thing in every file this command writes.
const canhBao = "Code generated by tools/apidoc. DO NOT EDIT BY HAND — run `make kb` to regenerate. " +
	"Hand edits are silently lost on the next generate (rule 9, invariant 8). " +
	"The source of truth is the route declaration in services/*/internal/."

// cauHinh is where the outputs go. Kept as a parameter rather than derived inside the
// generator so the determinism test can write two runs into two temporary directories and
// compare them byte for byte.
type cauHinh struct {
	Root     string // repository root — everything under services/ is scanned
	OpenAPI  string // kb/20-contracts/openapi.json
	Surface  string // kb/30-indexes/api-surface.json
	TasksDir string // tasks/web
}

func main() {
	root, err := timGoc()
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidoc:", err)
		os.Exit(1)
	}
	c := cauHinh{
		Root:     root,
		OpenAPI:  filepath.Join(root, "kb", "20-contracts", "openapi.json"),
		Surface:  filepath.Join(root, "kb", "30-indexes", "api-surface.json"),
		TasksDir: filepath.Join(root, "tasks", "web"),
	}
	n, kq, err := chay(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidoc:", err)
		os.Exit(1)
	}

	// THE LINE A PERSON ACTUALLY NEEDS. A colleague is building a screen against a contract
	// that no longer exists, and the only way they find out is somebody reading this and going
	// to look for them. It gets the route, not just a hash.
	for _, m := range kq.MoCoi {
		fmt.Fprintf(os.Stderr,
			"apidoc: MỒ CÔI ĐANG LÀM DỞ — việc %s (%s %s) đã được nhận ở tasks/web/claimed/ "+
				"nhưng route đó KHÔNG CÒN trong mã nguồn. Tìm người đang dựng màn hình này.\n",
			m.ID, m.Method, m.Path)
	}

	fmt.Printf("apidoc: %d route · %d việc mới · %d chuyển sang stale · %d hồi sinh · %d mồ côi đang ở claimed\n",
		n, kq.Moi, kq.Stale, kq.HoiSinh, len(kq.MoCoi))
}

// chay is the whole command. Returns the number of routes documented and what the queue did.
//
// SAFETY NET 1 — NOTHING IS WRITTEN ON A FAILED RUN. Every error path below returns before the
// first vietJSON, and quetTuyen joins the errors from ALL files rather than reporting the
// first: a file that fails to parse must never be read as "that file declares no routes", or a
// half-written tree would look like a batch of routes that had disappeared, and the queue would
// sweep live work into stale/.
func chay(c cauHinh) (int, ketQuaViec, error) {
	var kq ketQuaViec

	tuyens, err := quetTuyen(c.Root)
	if err != nil {
		return 0, kq, err
	}

	gm, err := moGiaiMa(c.Root)
	if err != nil {
		return 0, kq, err
	}

	doc, surface, err := dungTaiLieu(tuyens, gm)
	if err != nil {
		return 0, kq, err
	}

	if err := vietJSON(c.OpenAPI, doc); err != nil {
		return 0, kq, err
	}
	if err := vietJSON(c.Surface, surface); err != nil {
		return 0, kq, err
	}

	kq, err = dongBoViec(c.TasksDir, tuyens)
	if err != nil {
		return 0, kq, err
	}
	return len(tuyens), kq, nil
}

// timGoc walks up to the directory holding go.mod.
func timGoc() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", fmt.Errorf("không tìm thấy go.mod — chạy từ bên trong repository")
}
