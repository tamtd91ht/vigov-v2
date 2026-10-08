package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The full-email read (ADR 0082 §3): the read and its audit entry in ONE committed transaction, on
// the fake driver of dang_nhap_giao_dich_test.go. The real predicate is the store's.

const readerCode = "CB-002"

func revealHarness(t *testing.T) (*StaffEmailReveal, *ghiChep) {
	t.Helper()
	db, g := moDB(t)
	return NewStaffEmailReveal(db, idstore.NewCanBoStore(db)), g
}

func reader() NguoiThucHien {
	return NguoiThucHien{ID: "nd-01JREADERINTERNALID", Vet: audit.Actor{ID: readerCode, Kind: "staff", IP: ipGia}}
}

func TestRevealReadAndEntryShareOneCommittedTransaction(t *testing.T) {
	uc, g := revealHarness(t)

	email, err := uc.Reveal(ctxXa(xaThu), maCanBo, reader())
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if email != emailCB {
		t.Fatalf("email = %q, want the full address", email)
	}
	read := g.tim("SELECT email FROM nguoi_dung")
	entry := g.tim("INSERT INTO audit_log")
	if read == nil || entry == nil {
		t.Fatal("no read or no audit entry")
	}
	if read.tx == 0 || read.tx != entry.tx || g.ketThucCua(read.tx) != "commit" {
		t.Fatalf("read tx %d, entry tx %d, end %q — must be ONE committed transaction",
			read.tx, entry.tx, g.ketThucCua(read.tx))
	}
	// Scoped: commune $1 from the context, the code $2, soft-deleted and address-less rows excluded.
	if read.args[0] != string(xaThu) || read.args[1] != maCanBo {
		t.Errorf("read args %v", read.args)
	}
	for _, want := range []string{"WHERE tenant_id = $1", "ma = $2", "deleted_at IS NULL", "email IS NOT NULL"} {
		if !strings.Contains(read.sql, want) {
			t.Errorf("read lacks %q: %s", want, read.sql)
		}
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if entry.args[0] != string(xaThu) || entry.args[1] != readerCode || entry.args[2] != "staff" ||
		entry.args[3] != ipGia || entry.args[4] != ActionRevealStaffEmail || entry.args[5] != maCanBo {
		t.Errorf("entry %v — want commune, reader's STAFF CODE, IP, the verb, the read staff code", entry.args[:6])
	}
	// The trail records WHICH field was opened, never the value (rule 6, forbidden #4).
	raw := entry.args[7].([]byte)
	if strings.Contains(string(raw), emailCB) {
		t.Fatalf("the audit delta stores the address: %s", raw)
	}
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatal(err)
	}
	if delta["quyen"] != "task.read" {
		t.Errorf("delta = %v, want quyen task.read", delta)
	}
}

func TestRevealEntryFailureDisclosesNothing(t *testing.T) {
	uc, g := revealHarness(t)
	g.loiTheo["audit_log"] = errors.New("audit write failed")

	email, err := uc.Reveal(ctxXa(xaThu), maCanBo, reader())
	if err == nil || email != "" {
		t.Fatalf("audit failed and the address was returned: %q, %v", email, err)
	}
	if strings.Contains(err.Error(), emailCB) {
		t.Errorf("the error carries the address: %v", err)
	}
	read := g.tim("SELECT email FROM nguoi_dung")
	if read == nil || g.ketThucCua(read.tx) != "rollback" {
		t.Fatal("a disclosure with no trail was committed")
	}
}

func TestRevealUnknownCodeIsNotFoundAndUnaudited(t *testing.T) {
	uc, g := revealHarness(t)
	g.khongCoNguoiDung = true

	_, err := uc.Reveal(ctxXa(xaThu), "CB-999", reader())
	if !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("err = %v, want ErrCanBoKhongTonTai", err)
	}
	if g.tim("INSERT INTO audit_log") != nil {
		t.Error("nothing was disclosed, yet an entry was written")
	}
}

func TestRevealRefusesAnActorWithoutStaffCode(t *testing.T) {
	uc, g := revealHarness(t)
	actor := reader()
	actor.Vet.ID = "" // rule 6, invariant 8: no fallback to the internal id

	if _, err := uc.Reveal(ctxXa(xaThu), maCanBo, actor); err == nil {
		t.Fatal("a disclosure without a staff code to attribute it to was allowed")
	}
	if g.tim("SELECT email FROM nguoi_dung") != nil {
		t.Error("the address was read before the actor was validated")
	}
}
