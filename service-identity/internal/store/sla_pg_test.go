package store

import (
	"database/sql"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Integration tests for the deadline table against a real PostgreSQL, sharing the harness in
// checker_pg_test.go (moKetNoi, xaRieng, ctxXa).
//
// WHY A REAL DATABASE, AND WHY THESE PROPERTIES SPECIFICALLY: every one of them lives entirely
// in the DDL, where nothing on the Go side can see it and no fake can disagree with it —
//
//	the NULL dedup       UNIQUE (tenant_id, loai_viec, linh_vuc) alone does NOT stop a commune
//	                     having two default rows, because PostgreSQL does not consider two NULLs
//	                     equal. The generated column linh_vuc_khoa is the whole answer to that,
//	                     and whether it works is a property of the server.
//	the freed slot       the same generated column goes NULL when the row is soft-deleted, so a
//	                     commune that removed a row can add one back. The catalogue tables do the
//	                     OPPOSITE on purpose (0005:69); only a server can show which this is.
//	the CHECKs           '' refused as a field code, 0 refused as a number of hours, a fourth
//	                     `loai_viec` refused. Each is one line of DDL and untestable without it.
//	NULLS FIRST          the default row is read first. With one default row per commune the
//	                     wrong ordering is invisible in the rows themselves.
//	the commune          two communes each holding a default row for the same kind of work, and
//	                     neither reading the other's.
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8).

// idSLA pads a name out to the 26 characters sla_id_la_ulid demands. Not a real ULID — this is a
// fixture, and a fixture that looked like a real identifier is a fixture somebody copies.
func idSLA(ten string) string {
	if len(ten) >= 26 {
		return ten[:26]
	}
	return ten + strings.Repeat("0", 26-len(ten))
}

// themSLA inserts one row. `linhVuc` empty means the DEFAULT row, which is SQL NULL.
func themSLA(t *testing.T, db *sql.DB, tenantID, id, loaiViec, linhVuc string,
	tn, xl, sdh, bld, bct int) error {
	t.Helper()
	var lv any
	if linhVuc != "" {
		lv = linhVuc
	}
	// `linh_vuc_khoa` IS NEVER INSERTED: it is GENERATED ALWAYS, and naming it in an INSERT is
	// an error rather than an override.
	_, err := db.Exec(
		`INSERT INTO sla (tenant_id, id, loai_viec, linh_vuc,
		                  gio_tiep_nhan, gio_xu_ly_xong, gio_sap_den_han,
		                  gio_bao_lanh_dao, gio_bao_chu_tich)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		tenantID, idSLA(id), loaiViec, lv, tn, xl, sdh, bld, bct)
	return err
}

func themSLABuoc(t *testing.T, db *sql.DB, tenantID, id, loaiViec, linhVuc string,
	tn, xl, sdh, bld, bct int) {
	t.Helper()
	if err := themSLA(t, db, tenantID, id, loaiViec, linhVuc, tn, xl, sdh, bld, bct); err != nil {
		t.Fatalf("thêm dòng SLA %s: %v", id, err)
	}
}

func dungSLAStore(db *sql.DB) *SLAStore { return NewSLAStore(pkgstore.New(db)) }

func TestPgSLAMotXaChiCoMotDongMacDinhMoiLoaiViec(t *testing.T) {
	// THE ONE CONSTRAINT THIS TABLE EXISTS TO CARRY, and the plain unique key cannot express it:
	// two NULLs do not collide in PostgreSQL, so without linh_vuc_khoa a commune could hold two
	// default rows for `phan-anh` and which one a deadline came from would depend on read order.
	// ADR 0029 consequence #1 names this case explicitly.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themSLABuoc(t, db, xa, "sla-pa-md-1", "phan-anh", "", 8, 56, 24, 25, 49)

	err := themSLA(t, db, xa, "sla-pa-md-2", "phan-anh", "", 4, 24, 12, 13, 25)
	if err == nil {
		t.Fatal("xã nhận được HAI dòng mặc định cho một loại việc — khoá duy nhất không chặn NULL")
	}

	// A different kind of work is a different slot, and must still be accepted.
	themSLABuoc(t, db, xa, "sla-vb-md", "van-ban-den", "", 7, 40, 23, 26, 50)
}

func TestPgSLAXoaMemGiaiPhongChoDongMacDinh(t *testing.T) {
	// THE OPPOSITE OF WHAT THE CATALOGUE TABLES DO, and the difference is the point. A catalogue
	// counts soft-deleted rows in its unique key because AN ISSUED CODE IS NEVER REISSUED
	// (0005:69, rule 7 invariant 3). An SLA row issues nothing: the deadlines it produced are
	// stored on the petitions themselves and never recomputed (rule 10, invariant 2). So a
	// commune that removes its default row and later adds one back is changing policy from that
	// moment forward — exactly what ADR 0007 decision 6 permits — not reusing an identifier.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themSLABuoc(t, db, xa, "sla-pa-md-cu", "phan-anh", "", 8, 56, 24, 25, 49)
	if _, err := db.Exec(
		`UPDATE sla SET deleted_at = now(), delete_reason = $3 WHERE tenant_id = $1 AND id = $2`,
		xa, idSLA("sla-pa-md-cu"), "xã đổi chính sách xử lý"); err != nil {
		t.Fatalf("xoá mềm dòng mặc định: %v", err)
	}

	themSLABuoc(t, db, xa, "sla-pa-md-moi", "phan-anh", "", 4, 24, 12, 13, 25)

	ds, err := dungSLAStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1 — dòng đã xoá mềm vẫn lên danh sách?", len(ds))
	}
	if ds[0].GioXuLyXong != 24 {
		t.Errorf("đọc phải dòng cũ: %+v", ds[0])
	}

	// Rule 7: the removed row is still there. Soft delete, never destruction.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sla WHERE tenant_id = $1 AND id = $2`,
		xa, idSLA("sla-pa-md-cu")).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("bản ghi đã bị xoá thật: count = %d", n)
	}
}

func TestPgSLARefuseLinhVucRong(t *testing.T) {
	// '' MUST NOT BECOME A SECOND WAY TO SAY "DEFAULT". Without sla_linh_vuc_khong_rong an empty
	// string arriving from a form would sit beside the NULL row as a separate key, and no screen
	// distinguishes the two.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if _, err := db.Exec(
		`INSERT INTO sla (tenant_id, id, loai_viec, linh_vuc, gio_tiep_nhan, gio_xu_ly_xong,
		                  gio_sap_den_han, gio_bao_lanh_dao, gio_bao_chu_tich)
		 VALUES ($1,$2,'phan-anh','   ',8,56,24,25,49)`,
		xa, idSLA("sla-linh-vuc-rong")); err == nil {
		t.Fatal("mã lĩnh vực rỗng/khoảng trắng được nhận — sla_linh_vuc_khong_rong không chặn")
	}
}

func TestPgSLARefuseSoGioKhongDuong(t *testing.T) {
	// ZERO WORKING HOURS IS A DEADLINE BREACHED AT THE INSTANT IT IS MADE. Each of the five is
	// checked separately here, because sla_gio_phai_duong is one constraint covering five
	// columns and a missing conjunct in it would be invisible from any single case.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	gio := [][5]int{
		{0, 56, 24, 25, 49},
		{8, 0, 24, 25, 49},
		{8, 56, 0, 25, 49},
		{8, 56, 24, 0, 49},
		{8, 56, 24, 25, -1},
	}
	for i, g := range gio {
		err := themSLA(t, db, xa, idSLA("sla-gio-"+string(rune('a'+i))), "phan-anh",
			"linh-vuc-"+string(rune('a'+i)), g[0], g[1], g[2], g[3], g[4])
		if err == nil {
			t.Errorf("bộ giờ %v được nhận — sla_gio_phai_duong không chặn cột thứ %d", g, i+1)
		}
	}
}

func TestPgSLARefuseLoaiViecThuTu(t *testing.T) {
	// ADR 0029, stop condition #1: a fourth kind of work is a decision, not a deployment. The
	// CHECK is what makes adding one impossible to do by accident on a running database.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	if err := themSLA(t, db, xa, "sla-loai-la", "giai-ngan", "", 8, 40, 24, 25, 49); err == nil {
		t.Fatal("loại việc thứ tư được nhận — sla_loai_viec_hop_le không chặn")
	}
}

func TestPgSLADongMacDinhDocRaTruoc(t *testing.T) {
	// NULLS FIRST, and PostgreSQL's ASC default is NULLS LAST — so this is the only place the
	// clause can be shown to do anything. The default row first is what the configuration screen
	// shows and what makes the fallback visible to a person reading the table.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themSLABuoc(t, db, xa, "sla-pa-an-ninh", "phan-anh", "an-ninh-trat-tu", 2, 16, 4, 9, 17)
	themSLABuoc(t, db, xa, "sla-pa-md", "phan-anh", "", 8, 56, 24, 25, 49)

	ds, err := dungSLAStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("nhận %d dòng, muốn 2", len(ds))
	}
	if !ds[0].LaDongMacDinh() {
		t.Fatalf("dòng mặc định không đứng đầu: %+v", ds)
	}

	// And the lookup the deadline path actually runs, against rows a server produced.
	d, co := domain.DongTheoLinhVuc(ds, domain.LoaiViecPhanAnh, "an-ninh-trat-tu")
	if !co || d.GioXuLyXong != 16 {
		t.Errorf("tra lĩnh vực an ninh trật tự ra %+v (tìm thấy=%v), muốn 16 giờ", d, co)
	}
}

func TestPgSLAKhongDocSangXaKhac(t *testing.T) {
	// The commune is bound from the context by Scoped.Query and cannot be passed in (rule 1,
	// invariant 5). This also proves the composite unique key lets EVERY commune hold its own
	// default row for the same kind of work — a single-column key would let exactly one commune
	// onboard (rule 1, forbidden #4).
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	themSLABuoc(t, db, xaA, "sla-a-md", "phan-anh", "", 8, 56, 24, 25, 49)
	themSLABuoc(t, db, xaB, "sla-b-md", "phan-anh", "", 2, 16, 4, 9, 17)

	ds, err := dungSLAStore(db).DanhSach(ctxXa(xaB))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("RÒ RỈ: nhận %d dòng cho xã B", len(ds))
	}
	if ds[0].GioXuLyXong != 16 {
		t.Fatalf("RÒ RỈ: xã B đọc phải cam kết của xã A: %+v", ds[0])
	}
}

func TestPgSLAXaChuaCauHinhKhongCoDong(t *testing.T) {
	// EVERY COMMUNE IS IN THIS STATE TODAY: migration 0008 seeds nothing and the onboarding step
	// does not exist. The read must not fail — the configuration screen has to load — and the
	// caller must not be able to mistake it for "no deadline applies".
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	ds, err := dungSLAStore(db).DanhSach(ctxXa(xa))
	if err != nil {
		t.Fatalf("DanhSach trên xã chưa cấu hình: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("xã chưa cấu hình mà có %d dòng", len(ds))
	}
	if vd := domain.VanDeCuaSLA(ds); len(vd) != 1 || vd[0].Loai != domain.VanDeSLATrong {
		t.Fatalf("bảng rỗng không được nêu tên là vấn đề: %+v", vd)
	}
	if _, co := domain.DongMacDinh(ds, domain.LoaiViecPhanAnh); co {
		t.Fatal("xã chưa cấu hình mà vẫn tra ra một hạn")
	}
}
