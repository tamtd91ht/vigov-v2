package app

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// An intake path writes `sender_phone_normalized` (migration 0046) in the SAME INSERT as the typed number:
// the staff intake, through the REAL store statement (the other staff-intake tests use a fake row
// writer). Every other intake path (citizen, verified-phone attach, accountless) books through the same
// PhieuPhanAnhStore.Tao, whose binding is pinned in internal/store/contact_petition_test.go.
func TestStaffIntakeWritesSenderPhoneMatchKey(t *testing.T) {
	for name, c := range map[string]struct {
		typed string
		want  any
	}{
		"national":     {"0900000000", "0900000000"},
		"typed spaced": {"+84 900 000 000", "0900000000"},
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoGia{}
			db := pkgstore.New(sql.OpenDB(k))
			register := petstore.NewPhieuPhanAnhStore(db)
			uc := NewStaffIntake(db, register, petstore.NewSuKienDiStore(db), staffDeadlines(), &staffFieldsFake{})
			uc.newID = func() (string, error) { return idCoDinh, nil }
			uc.newCode = func() (string, error) { return maCoDinh, nil }
			uc.clock = func() time.Time { return staffBookedAt }

			req := staffRequest()
			req.ReporterPhone = c.typed
			if _, err := uc.Book(ctxXa(xaThu), req, staffOfficer()); err != nil {
				t.Fatalf("Book: %v", err)
			}
			ins := k.lenh[0]
			if !strings.Contains(ins.sql, "sender_phone_normalized)") || len(ins.args) != 24 {
				t.Fatalf("first statement = %q with %d args", ins.sql, len(ins.args))
			}
			if ins.args[23] != c.want || !ins.trongGiaoDich {
				t.Errorf("sender_phone_normalized = %#v (in tx %v), want %#v", ins.args[23], ins.trongGiaoDich, c.want)
			}
		})
	}
}
