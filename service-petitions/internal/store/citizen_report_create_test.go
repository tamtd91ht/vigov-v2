package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// PhieuPhanAnhStore.Tao binds the scene location (ADR 0050) into `lat` and `lng`, on the recording
// driver of stored_file_test.go.
//
//	PROVED HERE   $10 is `lat` and $11 is `lng` in that order and carry the row's values · a row with
//	              no location binds two NULLs rather than two zeros.
//	NOT PROVED    NUMERIC(9,6) round-tripping on a real PostgreSQL — phieu_phan_anh_pg_test.go, which
//	              skips without VIGOV_TEST_DSN.

func createRow(lat, lng *float64) domain.PhieuPhanAnh {
	return domain.PhieuPhanAnh{
		ID: "pa-001", MaTraCuu: "PA-4K7M-92XR-BTV1", Kenh: domain.KenhZaloMiniApp,
		CongDanID: "cd-001", NoiDung: "x", Lat: lat, Lng: lng,
		TrangThai: domain.DaTiepNhan, GocDemHan: sfAt, VaoSoLuc: sfAt,
		PublicationStatus: domain.PublicationPending,
	}
}

func runCreate(t *testing.T, p domain.PhieuPhanAnh) sfStmt {
	t.Helper()
	d := &sfDB{affected: 1}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.Tao(ctx, tx, p)
	}); err != nil {
		t.Fatalf("Tao: %v", err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(d.stmts))
	}
	return d.stmts[0]
}

func TestCreateBindsSceneLocation(t *testing.T) {
	lat, lng := 16.0544, 108.2022
	st := runCreate(t, createRow(&lat, &lng))

	if !strings.Contains(st.sql, "dia_chi, thon_id, lat, lng,") {
		t.Fatalf("column list moved; the positions below no longer mean lat/lng: %s", st.sql)
	}
	gotLat, ok1 := st.args[9].(*float64)
	gotLng, ok2 := st.args[10].(*float64)
	if !ok1 || !ok2 || gotLat == nil || gotLng == nil || *gotLat != lat || *gotLng != lng {
		t.Errorf("$10/$11 = %#v/%#v, want lat %v then lng %v — two adjacent NUMERICs swap silently",
			st.args[9], st.args[10], lat, lng)
	}
}

func TestCreateWithoutSceneLocationBindsNull(t *testing.T) {
	st := runCreate(t, createRow(nil, nil))

	for i, name := range map[int]string{9: "lat", 10: "lng"} {
		if v, ok := st.args[i].(*float64); !ok || v != nil {
			t.Errorf("%s = %#v, want a nil *float64 (SQL NULL), never 0 — 0,0 is a real place", name, st.args[i])
		}
	}
}
