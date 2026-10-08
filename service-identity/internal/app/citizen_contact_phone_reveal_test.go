package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The citizen contact-phone disclosure (ResolveCitizenContactPhone): the read and its audit entry in
// ONE committed transaction, on the fake driver of dang_nhap_giao_dich_test.go. The real predicate is
// the store's (store/citizen_contact_phone_test.go, against PostgreSQL).

const (
	sessionSample = "SID-01JSESSIONSAMPLE0000000"
	citizenSample = "CD-01JCITIZENSAMPLE00000000"
)

func contactPhoneHarness(t *testing.T) (*CitizenContactPhoneReveal, *ghiChep) {
	t.Helper()
	db, g := moDB(t)
	// The store's raw handle is never used by ContactPhoneForReveal (it runs on the caller's
	// transaction); it is the same fake driver so a stray use would be recorded, not hidden.
	raw := sql.OpenDB(ketNoiGia{g: g})
	t.Cleanup(func() { raw.Close() })
	return NewCitizenContactPhoneReveal(db, idstore.NewPhienCongDanStore(raw, slog.New(slog.NewTextHandler(io.Discard, nil)))), g
}

func TestContactPhoneReadAndEntryShareOneCommittedTransaction(t *testing.T) {
	uc, g := contactPhoneHarness(t)

	phone, err := uc.Reveal(ctxXa(xaThu), sessionSample, citizenSample)
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if phone != dienThoaiGia {
		t.Fatalf("phone = %q, want the verified number", phone)
	}
	read := g.tim("SELECT d.so_dien_thoai FROM phien_cong_dan")
	entry := g.tim("INSERT INTO audit_log")
	if read == nil || entry == nil {
		t.Fatal("no read or no audit entry")
	}
	if read.tx == 0 || read.tx != entry.tx || g.ketThucCua(read.tx) != "commit" {
		t.Fatalf("read tx %d, entry tx %d, end %q — must be ONE committed transaction",
			read.tx, entry.tx, g.ketThucCua(read.tx))
	}
	// Scoped: commune $1 from the context, the sid $2, the citizen $3; live sessions only.
	if read.args[0] != string(xaThu) || read.args[1] != sessionSample || read.args[2] != citizenSample {
		t.Errorf("read args %v", read.args)
	}
	for _, want := range []string{"WHERE tenant_id = $1", "p.id = $2", "p.cong_dan_id = $3",
		"p.thu_hoi_luc IS NULL", "p.het_han_luc > now()"} {
		if !strings.Contains(read.sql, want) {
			t.Errorf("read lacks %q: %s", want, read.sql)
		}
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if entry.args[0] != string(xaThu) || entry.args[1] != citizenSample || entry.args[2] != authz.KindCitizen ||
		entry.args[3] != "" || entry.args[4] != ActionRevealCitizenContactPhone || entry.args[5] != citizenSample {
		t.Errorf("entry %v — want commune, the CITIZEN id as actor, kind citizen, NO IP, the verb, the citizen", entry.args[:6])
	}
	raw := entry.args[7].([]byte)
	if strings.Contains(string(raw), dienThoaiGia) {
		t.Fatalf("the audit delta stores the number: %s", raw)
	}
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatal(err)
	}
	if delta["muc_dich"] != "gan_vao_phan_anh" || delta["phien"] != sessionSample {
		t.Errorf("delta = %v", delta)
	}
	if f, _ := delta["truong_da_mo"].([]any); len(f) != 1 || f[0] != "so_dien_thoai" {
		t.Errorf("truong_da_mo = %v, want [so_dien_thoai]", delta["truong_da_mo"])
	}
	if n := countStatements(g, "INSERT INTO audit_log"); n != 1 {
		t.Errorf("%d audit entries, want exactly one", n)
	}
}

func TestContactPhoneEntryFailureDisclosesNothing(t *testing.T) {
	uc, g := contactPhoneHarness(t)
	g.loiTheo["audit_log"] = errors.New("audit write failed")

	phone, err := uc.Reveal(ctxXa(xaThu), sessionSample, citizenSample)
	if err == nil || phone != "" {
		t.Fatalf("audit failed and the number was returned: %q, %v", phone, err)
	}
	if strings.Contains(err.Error(), dienThoaiGia) || strings.Contains(err.Error(), citizenSample) ||
		strings.Contains(err.Error(), sessionSample) {
		t.Errorf("the error carries personal data or identifiers: %v", err)
	}
	read := g.tim("SELECT d.so_dien_thoai FROM phien_cong_dan")
	if read == nil || g.ketThucCua(read.tx) != "rollback" {
		t.Fatal("a disclosure with no trail was committed")
	}
}

func TestContactPhoneNoUsableSessionIsSentinelAndUnaudited(t *testing.T) {
	uc, g := contactPhoneHarness(t)
	g.noContactPhone = true

	phone, err := uc.Reveal(ctxXa(xaThu), sessionSample, citizenSample)
	if !errors.Is(err, idstore.ErrNoContactPhone) || phone != "" {
		t.Fatalf("got %q, %v — want ErrNoContactPhone", phone, err)
	}
	if g.tim("INSERT INTO audit_log") != nil {
		t.Error("nothing was disclosed, yet an entry was written")
	}
}

func TestContactPhoneEmptyKeysTouchNothing(t *testing.T) {
	uc, g := contactPhoneHarness(t)
	for _, c := range [][2]string{{"", citizenSample}, {sessionSample, ""}} {
		if _, err := uc.Reveal(ctxXa(xaThu), c[0], c[1]); !errors.Is(err, idstore.ErrNoContactPhone) {
			t.Errorf("Reveal(%q, %q): err = %v, want ErrNoContactPhone", c[0], c[1], err)
		}
	}
	if n := g.soGiaoDich(); n != 0 {
		t.Errorf("%d transactions opened for an empty key", n)
	}
}

func countStatements(g *ghiChep, part string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, l := range g.lenh {
		if strings.Contains(l.sql, part) {
			n++
		}
	}
	return n
}
