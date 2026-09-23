package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The directories ARE the state machine, and renaming between them is the only transition.
// See the package comment in main.go.
//
// THE ORDER HERE IS LOOKUP PRIORITY, NOT THE LIFECYCLE. When one id somehow appears in two
// directories, the most finished one wins: a task that is done must never be read as open.
var thuMucViec = []string{"done", "claimed", "open", "stale"}

// cachNhanViec ships inside every task file, because the one place a reader of a task is
// certainly looking is the task.
const cachNhanViec = "Nhận việc = ĐỔI TÊN tệp này sang tasks/web/claimed/. " +
	"os.Rename là khóa: hai agent cùng nhận một việc thì một thành công, một nhận ENOENT. " +
	"XONG THÌ KHÔNG PHẢI ĐỔI TÊN: bộ sinh tự chuyển sang tasks/web/done/ khi thấy lời gọi trong " +
	"web-admin/src/lib/api/** (xem `manhinh.go`) — một dấu hiệu phải nhớ gõ là một dấu hiệu sẽ trôi. " +
	"Đừng thêm cờ trạng thái, tệp khóa hay bản ghi nào khác — thư mục đã là nguồn duy nhất. " +
	"Nếu route biến mất khỏi mã nguồn, bộ sinh chuyển việc đang ở open/ sang tasks/web/stale/ " +
	"(chuyển chứ không xoá — \"vì sao nó biến mất\" là câu sẽ có người hỏi); route quay lại thì " +
	"chính tệp này được chuyển ngược về open/. Việc đã ở claimed/ thì KHÔNG bị đụng, chỉ được báo ra."

// viec is one unit of web work. Field order here is the field order in the file.
type viec struct {
	CachNhan string `json:"_how_to_claim"`
	SinhBoi  string `json:"_generated_by"`
	HopDong  string `json:"_contract"`

	ID      string `json:"id"`
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
	Screen  string `json:"screen,omitempty"`

	QuyenKieu string `json:"permission_kind"`
	Quyen     string `json:"permission,omitempty"`
	QuyenLyDo string `json:"permission_reason,omitempty"`

	Idem     string `json:"idempotency"`
	IdemMode string `json:"idempotency_on_store_failure,omitempty"`

	RequestType string            `json:"request_type,omitempty"`
	ReplyTypes  map[string]string `json:"reply_types"`

	OperationID string `json:"operation_id"`
	DeclaredIn  string `json:"declared_in"`
}

// viecNgan names a task by its id AND the route it was for, because a hash alone is unreadable
// in a message a person has to act on.
type viecNgan struct {
	ID     string
	Method string
	Path   string
}

// ketQuaViec is what one synchronisation did, so the run can say it out loud.
type ketQuaViec struct {
	Moi     int
	Stale   int
	HoiSinh int
	// MoCoi: claimed right now, for a route that no longer exists.
	MoCoi []viecNgan
	// DaCoManHinh: moved open/ -> done/ because the client call already exists.
	DaCoManHinh []viecNgan
}

// maViec is the identity of a route, for as long as the route exists.
//
// Derived from service|METHOD|path and nothing else — not from the summary, not from the
// handler name, not from a counter. A renamed handler or a reworded summary must not produce
// a second task for work already finished, and a counter would hand the same id to two
// different routes depending on scan order.
func maViec(service, method, path string) string {
	h := sha256.Sum256([]byte(service + "|" + method + "|" + path))
	return hex.EncodeToString(h[:6])
}

// dongBoViec brings the queue in line with the routes that currently exist.
//
// FOUR OUTCOMES, BECAUSE A DISAPPEARED ROUTE MEANS FOUR DIFFERENT THINGS depending on where its
// task is sitting. Treating them alike would be wrong in three of the four cases:
//
//	open/     -> stale/     nobody started. MOVED, never deleted: "why did this vanish" is a
//	                        question somebody will ask, and a moved file answers it.
//	claimed/  -> untouched  somebody is BUILDING A SCREEN for a route that no longer exists —
//	                        against a contract that is gone. This is the case that matters most
//	                        and it has to be LOUD, not quiet. Taking the file out of their hands
//	                        would destroy their context at the moment they need it, and claimed/
//	                        belongs to admin-web-builder (ROUTING §3), not to this generator.
//	done/     -> silent     correct history: a screen was built, the route later changed. A real
//	                        record, not litter.
//	stale/    -> open/      the route came back with the same id. The SAME file goes back, not a
//	                        new one, so any note somebody left on it survives.
//
// A FIFTH MOVE, AND IT IS NOT ABOUT A DISAPPEARED ROUTE:
//
//	open/     -> done/      the admin web already calls this route. That used to be a manual
//	                        rename; it stopped happening, and the queue then spent weeks
//	                        declaring finished work unstarted. manhinh.go holds the reasoning
//	                        and the limits, step 3 below holds what keeps a VANISHED route out
//	                        of this move.
//
// TWO SAFETY NETS, because one run over a half-written tree could otherwise sweep the whole
// board into stale/ and the next run sweep it back — noise that ends with nobody trusting the
// queue at all:
//
//  1. Any parse error anywhere aborts BEFORE this function is reached (quetTuyen joins its
//     errors and chay returns on them), so a smaller route set is never mistaken for routes
//     having disappeared. Nothing at all is written on a failed run.
//  2. Zero routes means the generator is broken, not that the repository is empty — refuse
//     and move nothing.
//
// Deliberately NO percentage threshold. A number like "stop if more than half vanish" is a
// number nobody can justify, and it is wrong on exactly the run where it matters.
func dongBoViec(goc string, tuyens []tuyen, mh manHinh) (ketQuaViec, error) {
	var kq ketQuaViec

	// SAFETY NET 2.
	if len(tuyens) == 0 {
		return kq, errors.New("quét được 0 route — đó là bộ sinh hỏng, không phải kho mã trống; " +
			"không chuyển việc nào sang stale/")
	}

	for _, d := range thuMucViec {
		if err := os.MkdirAll(filepath.Join(goc, d), 0o755); err != nil {
			return kq, err
		}
		// git does not track empty directories, and a missing claimed/ would make the first
		// rename fail on a fresh clone.
		giu := filepath.Join(goc, d, ".gitkeep")
		if _, err := os.Stat(giu); os.IsNotExist(err) {
			if err := os.WriteFile(giu, nil, 0o644); err != nil {
				return kq, err
			}
		}
	}

	// hienTai IS BUILT FROM EVERY ROUTE, INCLUDING THE CITIZEN ONES THAT NEVER GET A TASK BELOW.
	//
	// The two loops ask different questions and must not share an answer. This map answers "does
	// this route still exist in the source", which decides whether an existing file is moved to
	// stale/. The create loop below answers "is this web-admin's work", which decides whether a
	// file is made at all.
	//
	// Leaving citizen routes out of this map would answer the first question with the second one's
	// answer, and stale/ is a CLAIM ABOUT CAUSE — `tasks/web/` is read as "the route disappeared
	// from the source" (ADR 0014). Filing a live route there is a confident answer to a question
	// nobody asked, which is worse than the misfiling it would be fixing.
	hienTai := map[string]bool{}
	// theoMa answers a THIRD question the two maps below cannot: given a task id already on the
	// board, which route is it, so the screen detector has something to look for.
	theoMa := map[string]tuyen{}
	for _, t := range tuyens {
		id := maViec(t.Service, t.Method, t.Path)
		hienTai[id] = true
		theoMa[id] = t
	}

	// 1. Revive or create.
	for _, t := range tuyens {
		// A CITIZEN ROUTE IS NOT WEB-ADMIN'S WORK. `tasks/web/` is the queue for the staff admin
		// web; a `citizen-only` route is consumed by `citizen-app`, a different surface in a
		// different framework with different rules (rule 4, not rule 5). Filing one here invites
		// an admin-web-builder agent to build a staff screen for a citizen endpoint — the concrete
		// failure this skips, observed on 2026-09-22 with GET /api/v1/my-citizen-reports.
		//
		// WHAT THIS DOES NOT DO, said plainly: it does not create a queue for citizen-app. That is
		// a decision about structure, not a defect, and it belongs to the project owner. Today the
		// citizen surface is tracked in kb/90-ephemeral/tien-do/citizen-app.json like any other
		// module.
		if t.Quyen.Kind == "citizen-only" {
			continue
		}
		id := maViec(t.Service, t.Method, t.Path)
		switch oDau(goc, id) {
		case "done", "claimed", "open":
			// done: already built. claimed: somebody has it. open: already waiting.
			continue
		case "stale":
			ok, err := chuyen(goc, "stale", "open", id)
			if err != nil {
				return kq, err
			}
			if ok {
				kq.HoiSinh++
			}
		default:
			if err := vietJSON(filepath.Join(goc, "open", id+".json"), dungViec(t, id)); err != nil {
				return kq, err
			}
			kq.Moi++
		}
	}

	// 2. open/ orphans -> stale/.
	for _, id := range maTrong(goc, "open") {
		if hienTai[id] {
			continue
		}
		ok, err := chuyen(goc, "open", "stale", id)
		if err != nil {
			return kq, err
		}
		if ok {
			kq.Stale++
		}
	}

	// 3. open/ tasks whose screen already exists -> done/.
	//
	// A ROUTE THAT NO LONGER EXISTS MUST NEVER LAND HERE: `stale/` is the true statement about it,
	// and a client call still lying around in the web is evidence of dead code, not of finished
	// work. TWO INDEPENDENT THINGS STOP IT, and the redundancy is stated because it was measured
	// rather than assumed — a mutation run on 2026-09-24 removed each one separately and the
	// suite stayed green both times, then went red when both went:
	//
	//	the order    step 2 has already swept vanished tasks out of open/, so they are not in
	//	             the list this loop reads.
	//	theoMa       a task id with no live route has no tuyen to ask daDung about, and the loop
	//	             skips it.
	//
	// Neither is decoration and neither is load-bearing alone. TestRouteBienMatThiStaleThangHon-
	// DaCoManHinh is what actually holds the invariant; do not delete one of the two on the
	// grounds that the tests still pass.
	//
	// ONLY open/ IS READ HERE, and the three directories left out are each left out for a reason:
	//
	//	claimed/  somebody is on it RIGHT NOW. Moving the file out from under them destroys their
	//	          context, and it is admin-web-builder's directory, not this generator's
	//	          (ROUTING §3). A "you already built this" note would also be permanent noise:
	//	          it stays true until they rename, which is exactly what they are about to do.
	//	done/     nothing to do.
	//	stale/    the route does not exist; step 1 revives it first if it comes back.
	//
	// A BRAND-NEW ROUTE THAT ALREADY HAS A SCREEN passes through open/ within this same run:
	// step 1 creates the file, this step moves it. It is counted in both Moi and DaCoManHinh,
	// and that is the honest description of what happened rather than a special case that would
	// need its own branch and its own test.
	for _, id := range maTrong(goc, "open") {
		t, co := theoMa[id]
		if !co || !mh.daDung(t) {
			continue
		}
		ok, err := chuyen(goc, "open", "done", id)
		if err != nil {
			return kq, err
		}
		if ok {
			kq.DaCoManHinh = append(kq.DaCoManHinh, viecNgan{ID: id, Method: t.Method, Path: t.Path})
		}
	}

	// 4. claimed/ orphans -> report only, touch nothing.
	for _, id := range maTrong(goc, "claimed") {
		if hienTai[id] {
			continue
		}
		kq.MoCoi = append(kq.MoCoi, docMoCoi(goc, "claimed", id))
	}

	// 5. done/ orphans: deliberately silent. Reporting them would turn correct history into a
	//    permanent warning, and a warning that can never be cleared is a warning people learn
	//    to scroll past.

	return kq, nil
}

func dungViec(t tuyen, id string) viec {
	v := viec{
		CachNhan:    cachNhanViec,
		SinhBoi:     canhBao,
		HopDong:     "kb/20-contracts/openapi.json",
		ID:          id,
		Service:     t.Service,
		Method:      t.Method,
		Path:        t.Path,
		Summary:     t.Summary,
		Screen:      t.Screen,
		QuyenKieu:   t.Quyen.Kind,
		Quyen:       t.Quyen.Key,
		QuyenLyDo:   t.Quyen.LyDo,
		Idem:        t.Idem.Kind,
		IdemMode:    t.Idem.Mode,
		RequestType: t.Request,
		ReplyTypes:  map[string]string{},
		OperationID: maOperation(t),
		DeclaredIn:  t.File,
	}
	for _, r := range t.Replies {
		v.ReplyTypes[fmt.Sprint(r.Status)] = r.Kieu
	}
	return v
}

// oDau says which directory holds a task id, "" for none.
func oDau(goc, id string) string {
	for _, d := range thuMucViec {
		if _, err := os.Stat(filepath.Join(goc, d, id+".json")); err == nil {
			return d
		}
	}
	return ""
}

// chuyen moves a task between two directories, REUSING THE LOCK THAT IS ALREADY THERE.
//
// A generator moving open/x.json to stale/ races a web agent claiming it at the same instant.
// os.Rename decides that atomically, so there is nothing to add: whoever loses gets ENOENT.
// The generator losing is the harmless side — it means the task was just claimed, and the next
// run will find it in claimed/ and report it as a claimed orphan, which is the louder and more
// correct outcome anyway.
func chuyen(goc, tu, den, id string) (bool, error) {
	err := os.Rename(filepath.Join(goc, tu, id+".json"), filepath.Join(goc, den, id+".json"))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil // somebody got there first; carry on without a word
	default:
		return false, fmt.Errorf("chuyển việc %s từ %s sang %s: %w", id, tu, den, err)
	}
}

// maTrong lists the task ids in one directory, sorted so a run is deterministic.
func maTrong(goc, thuMuc string) []string {
	ents, err := os.ReadDir(filepath.Join(goc, thuMuc))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(n, ".json"))
	}
	sort.Strings(out)
	return out
}

// docMoCoi recovers the route a claimed task was for, so the warning can name the path
// somebody is currently building against instead of only a hash.
func docMoCoi(goc, thuMuc, id string) viecNgan {
	m := viecNgan{ID: id}
	b, err := os.ReadFile(filepath.Join(goc, thuMuc, id+".json"))
	if err != nil {
		return m
	}
	var v viec
	if json.Unmarshal(b, &v) == nil {
		m.Method, m.Path = v.Method, v.Path
	}
	return m
}
