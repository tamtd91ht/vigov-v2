package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// The three directories ARE the state machine. See the package comment in main.go for why
// renaming is the only permitted transition.
var thuMucViec = []string{"open", "claimed", "done"}

// cachNhanViec ships inside every task file, because the one place a reader of a task is
// certainly looking is the task.
const cachNhanViec = "Nhận việc = ĐỔI TÊN tệp này sang tasks/web/claimed/, xong thì sang tasks/web/done/. " +
	"os.Rename là khóa: hai agent cùng nhận một việc thì một thành công, một nhận ENOENT. " +
	"Đừng thêm cờ trạng thái, tệp khóa hay bản ghi nào khác — thư mục đã là nguồn duy nhất."

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

// sinhViec creates a task for every route that has none, in ANY of the three directories.
//
// Checking all three is the whole design: a task in claimed/ or done/ is work somebody has
// already taken, and regenerating must not put it back on the board. That also means "what is
// left to do" is answered by listing a directory, with no second ledger to keep in sync.
func sinhViec(goc string, tuyens []tuyen) (int, error) {
	for _, d := range thuMucViec {
		if err := os.MkdirAll(filepath.Join(goc, d), 0o755); err != nil {
			return 0, err
		}
		// git does not track empty directories, and a missing claimed/ would make the first
		// rename fail on a fresh clone.
		giu := filepath.Join(goc, d, ".gitkeep")
		if _, err := os.Stat(giu); os.IsNotExist(err) {
			if err := os.WriteFile(giu, nil, 0o644); err != nil {
				return 0, err
			}
		}
	}

	moi := 0
	for _, t := range tuyens {
		id := maViec(t.Service, t.Method, t.Path)
		if daCo(goc, id) {
			continue
		}
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
		if err := vietJSON(filepath.Join(goc, "open", id+".json"), v); err != nil {
			return moi, err
		}
		moi++
	}
	return moi, nil
}

func daCo(goc, id string) bool {
	for _, d := range thuMucViec {
		if _, err := os.Stat(filepath.Join(goc, d, id+".json")); err == nil {
			return true
		}
	}
	return false
}
