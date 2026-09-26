// Command ingress generates the routing table of the Kubernetes Ingress from the REST
// contract, kb/20-contracts/openapi.json.
//
// WHY THIS EXISTS: the Ingress used to carry one line, `/api/v1` -> identity. That line was
// exactly right on the day it was written, when every REST resource belonged to identity. On
// 2026-09-21 the contract held 23 routes across FIVE services, so eight of them — every route
// of comms, documents, finance and petitions — would have reached identity and returned 404.
// A 404 on a real administrative route is not a rendering glitch: it is a citizen's petition
// that the system says does not exist.
//
// Nothing reports that kind of drift. The pods are green, the probes are green, the routes
// are mounted in Go, and the only thing wrong is a file in deploy/ that nobody re-read. So the
// file is generated from the one place that already knows which service owns which path, and
// a test in this package turns red when the file and the contract disagree.
//
// The project owner chose this route — option (b) of the three written in the old warning
// block at the head of deploy/overlays/prod/ingress.yaml — on 2026-09-21.
//
// FAIL CLOSED: a path whose owning service cannot be determined stops the generator. It is
// never defaulted to identity, and no output file is written. Rule 1: no default on the
// isolation path — and a route silently pointed at the wrong service is a request handled by
// a service that does not own the data.
//
// Output is deterministic (sorted everywhere) so regenerating an unchanged contract produces
// no diff.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func main() {
	root, err := timGoc()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingress:", err)
		os.Exit(1)
	}

	tep, err := sinhTuKho(root)
	if err != nil {
		// The message carries the offending path; the caller has to fix the contract or the
		// manifests, and the generator must not invent an answer for them.
		fmt.Fprintln(os.Stderr, "ingress: DỪNG —", err)
		os.Exit(1)
	}

	if err := ghiCaHai(root, tep); err != nil {
		fmt.Fprintln(os.Stderr, "ingress:", err)
		os.Exit(1)
	}
	ten := make([]string, 0, len(tep))
	for t := range tep {
		ten = append(ten, t)
	}
	sort.Strings(ten)
	for _, t := range ten {
		fmt.Printf("ingress: đã sinh %s\n", t)
	}
}

// ghiCaHai writes every file to a temporary sibling first and renames only once all of them
// are on disk. The outputs are views of one routing table (Ingress, its per-environment host
// patches, and the admin-web proxy); a failed write that left one updated and another stale
// would route the same path to two different services depending on which surface the request
// entered by — or leave an overlay patching host indices base no longer has.
func ghiCaHai(root string, tep map[string][]byte) error {
	ten := make([]string, 0, len(tep))
	for t := range tep {
		ten = append(ten, t)
	}
	sort.Strings(ten)

	tam := make([]string, 0, len(ten))
	donDep := func() {
		for _, p := range tam {
			_ = os.Remove(p) // best effort: the temp file is ours and carries nothing
		}
	}
	for _, t := range ten {
		dich := filepath.Join(root, t)
		p := dich + ".tam"
		if err := os.WriteFile(p, tep[t], 0o644); err != nil {
			donDep()
			return fmt.Errorf("ghi %s: %w", t, err)
		}
		tam = append(tam, p)
	}
	for i, t := range ten {
		if err := os.Rename(tam[i], filepath.Join(root, t)); err != nil {
			donDep()
			return fmt.Errorf("đổi tên %s: %w", t, err)
		}
	}
	return nil
}

// timGoc walks up from the working directory to the repository root, recognised by go.work.
// Same approach as tools/kb: the tool must work whether it is run from the root or from its
// own directory.
func timGoc() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		cha := filepath.Dir(dir)
		if cha == dir {
			return "", fmt.Errorf("không tìm thấy gốc kho (không có go.work ở bất kỳ thư mục cha nào)")
		}
		dir = cha
	}
}
