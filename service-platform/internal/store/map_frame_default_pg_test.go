package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// The default map frame (migration 0020, ADR 0072 amendment 2) against a real PostgreSQL — skipped
// without VIGOV_TEST_DSN, like every *_pg_test.go here. What only the database can show: the row and
// its audit entry commit together; a repeat writes nothing; the operator log lists the entry; another
// commune never sees the row; 0020's CHECKs and guard refuse what the service would never send.

func TestPgMapFrameDefault(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	s := NewMapFrameDefaultStore(corestore.New(db))
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Cũ", false)
	trail := `SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = $2 AND actor_kind = 'operator'`

	if _, ok, err := s.OperatorMapFrameDefault(inCommune(ulidA)); err != nil || ok {
		t.Fatalf("fresh: ok=%v err=%v, want not configured", ok, err)
	}
	if _, _, err := s.OperatorMapFrameDefault(inCommune("01JD8ZQK9M3NPXR7TVWYB2C4EZ")); !errors.Is(err, ErrCommuneNotFound) {
		t.Fatalf("unknown commune: %v", err)
	}

	f, _ := domain.NormalizeMapFrameDefault(15.73, 108.37, 10)
	out, changed, err := s.SetMapFrameDefault(inCommune(ulidA), f, false, "Đặt mặc định", operatorFake)
	if err != nil || !changed || out.CreatedBy != "VH-00001" || out.RadiusKm != 10 {
		t.Fatalf("first set: %+v changed=%v err=%v", out, changed, err)
	}
	if _, changed, err := s.SetMapFrameDefault(inCommune(ulidA), f, false, "Lặp", operatorFake); err != nil || changed {
		t.Fatalf("repeat: changed=%v err=%v", changed, err)
	}
	if count(t, db, trail, ulidA, ActionSetMapFrameDefault) != 1 {
		t.Error("want exactly one entry after a set and a no-op repeat")
	}

	g, _ := domain.NormalizeMapFrameDefault(15.8, 108.4, 25)
	if _, changed, err := s.SetMapFrameDefault(inCommune(ulidA), g, true, "Mở rộng", domain.OperatorActor{Code: "VH-00002", IP: "203.0.113.9"}); err != nil || !changed {
		t.Fatalf("change: changed=%v err=%v", changed, err)
	}
	var d, ip string
	if err := db.QueryRow(`SELECT delta::text, actor_ip FROM audit_log WHERE tenant_id = $1 AND action = $2
		ORDER BY id DESC LIMIT 1`, ulidA, ActionSetMapFrameDefault).Scan(&d, &ip); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"truoc": {"radius_km": 10`, `"sau": {"radius_km": 25`, `"ly_do": "Mở rộng"`, `"acknowledged_unusual": true`} {
		// jsonb prints keys shortest first, so radius_km leads each object.
		if !strings.Contains(d, want) {
			t.Errorf("delta %s lacks %s", d, want)
		}
	}
	if ip != "203.0.113.9" {
		t.Errorf("actor_ip %q", ip)
	}
	got, ok, err := s.MapFrameDefault(inCommune(ulidA))
	if err != nil || !ok || got.RadiusKm != 25 || got.CreatedBy != "VH-00001" || got.UpdatedBy != "VH-00002" {
		t.Errorf("read after change: %+v ok=%v err=%v", got, ok, err)
	}

	// Another commune never sees it (rule 1).
	if _, ok, err := s.MapFrameDefault(inCommune(ulidB)); err != nil || ok {
		t.Errorf("commune B sees A's default: ok=%v err=%v", ok, err)
	}
	// Inactive: refused, nothing written.
	if _, _, err := s.SetMapFrameDefault(inCommune(ulidB), f, false, "x", operatorFake); !errors.Is(err, ErrCommuneInactive) {
		t.Errorf("inactive commune: %v", err)
	}
	if _, _, err := s.SetMapFrameDefault(inCommune(ulidA), f, false, "x", domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("no actor: %v", err)
	}

	// The operator log lists the entry under the commune.
	firstLogPage, _ := page.New(OperatorLogOrder, "", "", "", "")
	res, err := NewOperatorLog(db).Read(context.Background(), OperatorLogQuery{CommuneID: ulidA, Page: firstLogPage}, operatorFake)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range res.Items {
		found = found || e.Action == ActionSetMapFrameDefault
	}
	if !found {
		t.Error("the operator log does not list the default-frame entry")
	}

	// 0020's floor under the service.
	for name, stmt := range map[string]string{
		"radius 50.1":  `UPDATE commune_map_frame_default SET radius_km = 50.1 WHERE tenant_id = $1`,
		"radius 0":     `UPDATE commune_map_frame_default SET radius_km = 0 WHERE tenant_id = $1`,
		"east of box":  `UPDATE commune_map_frame_default SET center_lng = 111.6 WHERE tenant_id = $1`,
		"staff code":   `UPDATE commune_map_frame_default SET updated_by = 'CB-00001' WHERE tenant_id = $1`,
		"created_by":   `UPDATE commune_map_frame_default SET created_by = 'VH-00009' WHERE tenant_id = $1`,
		"delete":       `DELETE FROM commune_map_frame_default WHERE tenant_id = $1`,
		"move commune": `UPDATE commune_map_frame_default SET tenant_id = '` + ulidB + `' WHERE tenant_id = $1`,
	} {
		if _, err := db.Exec(stmt, ulidA); err == nil {
			t.Errorf("%s: the database accepted it", name)
		}
	}
}
