package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/migrate"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/migrations"
)

// THE CASES THAT ONLY A REAL POSTGRESQL CAN DECIDE. Everything in danh_ba_can_bo_test.go runs on a
// fake driver, which is right for the decisions — but two of the properties this turn rests on are
// NOT decisions. They are behaviours of the server, and a fake agrees with whatever it is told:
//
//	#13 THE RACE       two administrators locking each other AT THE SAME MOMENT. The guard is a
//	                   read-decide-write, and what stops the two reads from both seeing "there is
//	                   somebody else" is `SELECT … FOR UPDATE` plus PostgreSQL re-evaluating the
//	                   predicate after the lock is released. No fake can exhibit that, and a test
//	                   that pretended to would be the worst kind of green.
//	#15 THE CODE       "a code, once issued, is never issued again, INCLUDING after a soft delete"
//	                   lives in `UNIQUE (tenant_id, ma)` — which is deliberately NOT partial — and
//	                   in the retry that answers it. The retry only works if the store recognises
//	                   the violation, and `nguoi_dung` is PARTITION BY HASH, so the server names the
//	                   PARTITION's copy of the key (`nguoi_dung_p16_tenant_id_ma_key`). Matching the
//	                   parent's name would compile, pass every unit test, and never fire.
//
// Skipped unless VIGOV_TEST_DSN is set. The DSN carries a password and lives only in the
// environment (rule 8).

var (
	dsnChung    string
	schemaChung string
)

// TestMain builds the schema ONCE for the whole package. Every test isolates itself with its own
// commune ids instead of its own schema — 160 CREATE TABLE statements per test is a suite nobody
// runs, and per-commune isolation is also closer to how the data really looks.
func TestMain(m *testing.M) {
	dsnChung = os.Getenv("VIGOV_TEST_DSN")
	if dsnChung == "" {
		os.Exit(m.Run()) // every pg test skips itself; the fake-driver tests still run
	}

	db, err := sql.Open("pgx", dsnChung)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}
	// ONE physical connection while the schema is built: `SET search_path` is SESSION state, so on
	// a pool it applies to whichever connection served that statement and to no other.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schemaChung = fmt.Sprintf("vigov_app_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schemaChung); err != nil {
		fmt.Fprintln(os.Stderr, "tạo schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schemaChung); err != nil {
		fmt.Fprintln(os.Stderr, "đặt search_path:", err)
		os.Exit(1)
	}
	// THE REAL RUNNER, not a hand-picked file: every migration this service ships is exercised
	// here, including the one added tomorrow.
	kq, err := migrate.Chay(ctx, db, migrations.FS, "identity")
	if err != nil {
		fmt.Fprintln(os.Stderr, "chạy migration:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "migration đã áp:", kq.DaAp)

	ma := m.Run()

	// A schema this suite created, in a test database. Rule 7 protects archival business data;
	// this holds none.
	if _, err := db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schemaChung+" CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "dọn schema:", err)
	}
	db.Close()
	os.Exit(ma)
}

// moPool opens a pool of its OWN, pinned to the suite's schema.
//
// ONE POOL PER CONNECTION, AND THAT IS THE WHOLE REASON THIS HELPER EXISTS. `SET search_path` is
// session state, so a pool that may open a second connection would run half its statements in
// `public`, where none of these tables exist. MaxOpenConns(1) pins it — and a test that needs TWO
// transactions AT ONCE (the #13 race) therefore opens TWO pools. The store package's own harness
// shares a single connection for the whole suite, which is exactly what a race test cannot use.
func moPool(t *testing.T) *sql.DB {
	t.Helper()
	if dsnChung == "" {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}
	db, err := sql.Open("pgx", dsnChung)
	if err != nil {
		t.Fatalf("mở kết nối: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("SET search_path TO " + schemaChung); err != nil {
		t.Fatalf("đặt search_path: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func xaRiengPg(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%026d", time.Now().UnixNano())[:26]
}

const bamGiaPg = "$argon2id$gia$KHONG-PHAI-HASH-THAT"

// dungXaPg seeds one commune: a role carrying the given keys, and n accounts holding it.
func dungXaPg(t *testing.T, db *sql.DB, xa, vaiTroID string, quyen []string, nguoiIDs ...string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO vai_tro (tenant_id, id, ten, ma) VALUES ($1,$2,$3,$4)`,
		xa, vaiTroID, "Vai trò thử", "vt-"+vaiTroID); err != nil {
		t.Fatalf("thêm vai trò: %v", err)
	}
	for _, q := range quyen {
		if _, err := db.Exec(
			`INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma) VALUES ($1,$2,$3)`,
			xa, vaiTroID, q); err != nil {
			t.Fatalf("cấp quyền %s: %v", q, err)
		}
	}
	for _, id := range nguoiIDs {
		// co_tai_khoan IS SET EXPLICITLY: the schema defaults it to false, because authority is
		// granted by a named person and never by a default. These people are accounts, which is
		// what makes them count as administrators.
		if _, err := db.Exec(
			`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, vai_tro_id, co_tai_khoan, mat_khau_hash)
			 VALUES ($1,$2,$3,$4,$5,$6,true,$7)`,
			xa, id, "CB-2026-"+id, "Nguyễn Văn A", id+"@xa.danang.gov.vn", vaiTroID, bamGiaPg); err != nil {
			t.Fatalf("thêm cán bộ %s: %v", id, err)
		}
	}
}

func ucThat(db *sql.DB) *DanhBaCanBo {
	kho := pkgstore.New(db)
	return NewDanhBaCanBo(kho, idstore.NewCanBoStore(kho))
}

func nguoiPg(id string) NguoiThucHien {
	return NguoiThucHien{ID: id, Vet: audit.Actor{ID: "CB-2026-" + id, Kind: "staff", IP: "10.0.0.7"}}
}

func demQuanTriConSong(t *testing.T, db *sql.DB, xa string) int {
	t.Helper()
	var n int
	err := db.QueryRow(`
		SELECT count(*)
		  FROM nguoi_dung nd
		  JOIN vai_tro       vt ON vt.tenant_id = nd.tenant_id AND vt.id         = nd.vai_tro_id
		  JOIN vai_tro_quyen vq ON vq.tenant_id = nd.tenant_id AND vq.vai_tro_id = vt.id
		 WHERE nd.tenant_id = $1 AND vq.quyen_ma = 'admin.user'
		   AND nd.deleted_at IS NULL AND nd.co_tai_khoan AND nd.dang_hoat_dong
		   AND vt.deleted_at IS NULL`, xa).Scan(&n)
	if err != nil {
		t.Fatalf("đếm quản trị viên: %v", err)
	}
	return n
}

// --- #13: the race the guard exists for -----------------------------------------------------------

// TWO ADMINISTRATORS LOCK EACH OTHER AT THE SAME INSTANT, ON TWO CONNECTIONS, AGAINST A REAL
// SERVER. Exactly one may succeed.
//
// This is the case the customer's decision is about, and it is the one an application-level check
// cannot pass on its own: both transactions read "there are two of us", both decide the operation
// is safe, and the commune ends with nobody. What makes it come out right is that
// QuanTriDangHoatDong takes `FOR UPDATE` over the rows the ANSWER depends on — so the second
// transaction blocks on the first, and when the first commits PostgreSQL RE-EVALUATES the
// predicate against the new row version, drops the administrator who has just been locked, and the
// second transaction sees a set of one.
//
// MUTATION THAT MUST TURN THIS RED: remove `FOR UPDATE` from truyVanQuanTriDeGhi. Every other test
// in the repository stays green — a plain SELECT returns the same rows in every sequential test.
func TestPgHaiQuanTriKhoaNhauCungLucThiXaVANConQuanTri(t *testing.T) {
	db1 := moPool(t)
	db2 := moPool(t)
	xa := xaRiengPg(t)
	a, b := "nd-quantri-a", "nd-quantri-b"

	dungXaPg(t, db1, xa, "vt-quan-tri", []string{"admin.user"}, a, b)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc1, uc2 := ucThat(db1), ucThat(db2)

	var wg sync.WaitGroup
	loi := make([]error, 2)
	batDau := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-batDau
		_, loi[0] = uc1.DatKhoa(ctx, b, true, nguoiPg(a)) // A khoá B
	}()
	go func() {
		defer wg.Done()
		<-batDau
		_, loi[1] = uc2.DatKhoa(ctx, a, true, nguoiPg(b)) // B khoá A
	}()
	close(batDau)
	wg.Wait()

	soXong := 0
	soChan := 0
	for _, e := range loi {
		switch {
		case e == nil:
			soXong++
		case errors.Is(e, ErrQuanTriCuoiCung):
			soChan++
		default:
			t.Fatalf("lỗi ngoài dự kiến: %v", e)
		}
	}

	if con := demQuanTriConSong(t, db1, xa); con < 1 {
		t.Fatalf("XÃ MẤT SẠCH QUẢN TRỊ VIÊN — %d người còn quyền admin.user. "+
			"Hai lượt khoá đồng thời đã cùng qua cửa: khe đọc-rồi-ghi chưa được đóng "+
			"(kiểm `FOR UPDATE` trong truyVanQuanTriDeGhi). Kết quả hai lượt: %v", con, loi)
	}
	if soXong != 1 || soChan != 1 {
		t.Fatalf("có %d lượt thành công và %d lượt bị chặn, muốn đúng 1 và 1 — kết quả: %v",
			soXong, soChan, loi)
	}
}

// THE SEQUENTIAL HALF OF THE SAME RULE, kept because the race case above would also pass if the
// guard refused EVERYTHING.
func TestPgKhoaQuanTriCuoiCungBiTuChoiConNguoiThuongThiKhong(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	quanTri, thuong := "nd-qt-duy-nhat", "nd-thuong"

	dungXaPg(t, db, xa, "vt-qt", []string{"admin.user"}, quanTri)
	dungXaPg(t, db, xa, "vt-thuong", []string{"document.read"}, thuong)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)

	if _, err := uc.DatKhoa(ctx, quanTri, true, nguoiPg(thuong)); !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("khoá quản trị viên cuối cùng: lỗi = %v, muốn ErrQuanTriCuoiCung", err)
	}
	if n := demQuanTriConSong(t, db, xa); n != 1 {
		t.Fatalf("còn %d quản trị viên sau một lượt khoá BỊ TỪ CHỐI, muốn 1", n)
	}
	// The same operation on somebody who is not an administrator must go through, in a commune
	// that has exactly one administrator. A guard written `len(quanTri) <= 1` would refuse this.
	if _, err := uc.DatKhoa(ctx, thuong, true, nguoiPg(quanTri)); err != nil {
		t.Fatalf("khoá một cán bộ thường bị chặn: %v", err)
	}
}

// THE THIRD PATH OF #13, against the real grant tables: moving the last administrator to a role
// that does not carry `admin.user`. Nobody is locked and nobody is deleted — the screen shows an
// ordinary role change.
func TestPgHaVaiTroQuanTriCuoiCungBiTuChoi(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	quanTri, khac := "nd-qt-mot", "nd-khac"

	dungXaPg(t, db, xa, "vt-qt", []string{"admin.user"}, quanTri)
	dungXaPg(t, db, xa, "vt-yeu", []string{"document.read"}, khac)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)

	// The actor holds admin.user + document.read, so #14's second constraint is satisfied and the
	// refusal below can only come from #13.
	nguoi := nguoiPg(quanTri)
	_ = nguoi
	if _, err := uc.DoiVaiTro(ctx, quanTri, "vt-yeu", nguoiPg(khac)); !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("hạ vai trò quản trị viên cuối cùng: lỗi = %v, muốn ErrQuanTriCuoiCung", err)
	}
	if n := demQuanTriConSong(t, db, xa); n != 1 {
		t.Fatalf("còn %d quản trị viên sau một lượt đổi vai trò BỊ TỪ CHỐI, muốn 1", n)
	}
}

// --- #15: a code, once issued, is never issued again ------------------------------------------------

// THE FULL PATH, AGAINST THE REAL UNIQUE KEY: a staff member leaves, their row is soft-deleted —
// kept, because the audit trail points at it — and the next arrival must NOT receive the code that
// now names somebody else in years of administrative records.
//
// WHAT THIS PROVES THAT store/ma_can_bo_pg_test.go DOES NOT: that the WRITE PATH survives the
// refusal. The unique key firing is only half the mechanism; the other half is the use case
// recognising it and minting another code. `nguoi_dung` is PARTITION BY HASH, so the server names
// the PARTITION's copy of the key — a matcher written against the parent's name would compile,
// pass every unit test, and never once fire here.
//
// MUTATION THAT MUST TURN THIS RED: change `tenant_id_ma_key` in dichLoiGhiCanBo to
// `nguoi_dung_tenant_id_ma_key`, the name the parent table carries.
func TestPgMaCuaNguoiDaXoaMemKhongCapLaiDuocVaDuongGhiTuSinhMaKhac(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	maCu := "CB-2026-CUCU01"

	// Somebody who left, soft-deleted, still holding their code.
	if _, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash, deleted_at)
		 VALUES ($1,$2,$3,$4,$5,false,'',now())`,
		xa, "nd-da-nghi", maCu, "Nguyễn Văn A", "danghi@xa.danang.gov.vn"); err != nil {
		t.Fatalf("thêm người đã xoá mềm: %v", err)
	}

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)
	// The generator hands out the departed person's code FIRST. That is the collision, forced
	// rather than waited for.
	lan := 0
	uc.sinhMa = func(time.Time) (string, error) {
		lan++
		if lan == 1 {
			return maCu, nil
		}
		return "CB-2026-MOI00" + fmt.Sprint(lan), nil
	}

	cb, err := uc.Them(ctx, YeuCauThemCanBo{
		HoTen: "Trần Thị B", Email: "canbo.b@xa.danang.gov.vn",
	}, nguoiPg("nd-quan-tri"))
	if err != nil {
		t.Fatalf("Them sau va chạm mã: %v — đường ghi không nhận ra vi phạm khoá duy nhất, "+
			"kiểm dichLoiGhiCanBo: bảng phân mảnh nên máy chủ nêu TÊN CỦA MẢNH", err)
	}
	if cb.Ma == maCu {
		t.Fatal("MÃ CỦA NGƯỜI ĐÃ XOÁ MỀM ĐƯỢC CẤP LẠI — luật 7 bất biến 3 vỡ")
	}
	if lan != 2 {
		t.Fatalf("sinh mã %d lần, muốn 2 (lần đầu va chạm, lần sau mã mới)", lan)
	}

	// The departed person's row is untouched, and the new one exists exactly once.
	var soCu int
	if err := db.QueryRow(`SELECT count(*) FROM nguoi_dung WHERE tenant_id=$1 AND ma=$2 AND deleted_at IS NOT NULL`,
		xa, maCu).Scan(&soCu); err != nil {
		t.Fatal(err)
	}
	if soCu != 1 {
		t.Fatalf("dòng đã xoá mềm mang mã cũ: %d, muốn 1", soCu)
	}
}

// THE INSERT WRITES LITERALS FOR THE FOUR COLUMNS NO REQUEST MAY REACH, and the only way to see
// that is to read the row back off a real server.
//
//	co_tai_khoan = false, mat_khau_hash = ''   this route creates a DIRECTORY ENTRY, never an
//	                                           account (#9/#17/#18 are a different flow)
//	vai_tro_id IS NULL                         a new person holds nothing; granting a role is the
//	                                           route that carries #14's two guards
//	phai_doi_mat_khau                          left at its fail-closed DEFAULT (migration 0009 §1)
func TestPgThemCanBoTaoDongDanhBaChuKhongPhaiTaiKhoan(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))

	cb, err := ucThat(db).Them(ctx, YeuCauThemCanBo{
		HoTen: "Trần Thị B", Email: "  CanBo.B@Xa.DaNang.GOV.VN ", ChucVu: "Công chức Văn phòng",
		DienThoaiCoQuan: "0900000000", DiDongCaNhan: "0900000011",
	}, nguoiPg("nd-quan-tri"))
	if err != nil {
		t.Fatalf("Them: %v", err)
	}

	var (
		coTaiKhoan, phaiDoi bool
		bam, email          string
		vaiTro              sql.NullString
		diDong, coQuan      string
	)
	err = db.QueryRow(
		`SELECT co_tai_khoan, mat_khau_hash, phai_doi_mat_khau, vai_tro_id, email,
		        dien_thoai_co_quan, di_dong_ca_nhan
		   FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, cb.ID).
		Scan(&coTaiKhoan, &bam, &phaiDoi, &vaiTro, &email, &coQuan, &diDong)
	if err != nil {
		t.Fatalf("đọc lại dòng vừa tạo: %v", err)
	}

	if coTaiKhoan || bam != "" {
		t.Errorf("tuyến danh bạ đã tạo một TÀI KHOẢN: co_tai_khoan=%v, hash rỗng=%v", coTaiKhoan, bam == "")
	}
	if vaiTro.Valid {
		t.Errorf("người mới đã có vai trò %q — phải là NULL, gán vai trò là tuyến riêng (#14)", vaiTro.String)
	}
	if !phaiDoi {
		t.Error("phai_doi_mat_khau = false — mặc định fail-closed của migration 0009 §1 đã mất")
	}
	// Normalisation reached the column: the address is stored lower-cased and trimmed, which is
	// what keeps `UNIQUE (tenant_id, email)` from admitting two spellings of one mailbox.
	if email != "canbo.b@xa.danang.gov.vn" {
		t.Errorf("email lưu = %q, muốn bản đã hạ thấp và cắt khoảng trắng", email)
	}
	// The two telephone columns did not trade places. They are adjacent TEXT columns and a swap
	// produces no error anywhere — and since #16 they are two kinds of data in law.
	if coQuan != "0900000000" || diDong != "0900000011" {
		t.Errorf("hai cột điện thoại bị lẫn: co_quan=%q, di_dong=%q", coQuan, diDong)
	}
}

// THE AUDIT ENTRY LANDS IN THE SAME TRANSACTION, ON THE REAL append-only TABLE (migration 0002),
// carrying the STAFF CODE as its actor (rule 6, invariant 8).
func TestPgThemCanBoGhiVetThatTrenBangAppendOnly(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	nguoi := nguoiPg("nd-quan-tri")

	cb, err := ucThat(db).Them(ctx, YeuCauThemCanBo{
		HoTen: "Trần Thị B", Email: "canbo.b@xa.danang.gov.vn", DiDongCaNhan: "0900000011",
	}, nguoi)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}

	var actor, hanhVi, chuThe, delta string
	err = db.QueryRow(
		`SELECT actor_id, action, subject, delta::text FROM audit_log
		  WHERE tenant_id=$1 AND subject=$2`, xa, cb.Ma).Scan(&actor, &hanhVi, &chuThe, &delta)
	if err != nil {
		t.Fatalf("không đọc được vết kiểm toán: %v — luật 6 bất biến 1", err)
	}
	if actor != nguoi.Vet.ID {
		t.Errorf("actor_id = %q, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", actor, nguoi.Vet.ID)
	}
	if actor == nguoi.ID {
		t.Error("actor_id đang mang ĐỊNH DANH NỘI BỘ")
	}
	if hanhVi != HanhViThemCanBo {
		t.Errorf("action = %q, muốn %q", hanhVi, HanhViThemCanBo)
	}
	// Rule 6, forbidden #4: the trail names the column and holds the masked value, never the raw
	// personal datum.
	if strings.Contains(delta, "0900000011") {
		t.Errorf("vết mang số di động TRẦN: %s", delta)
	}
	if !strings.Contains(delta, "di_dong_ca_nhan") {
		t.Errorf("vết không nêu tên cột: %s", delta)
	}
}

// THE TWO OTHER CONSTRAINT NAMES THIS LAYER TRANSLATES, against the partitioned tables that
// actually carry them.
func TestPgEmailTrungVaBoPhanLaBiDichThanhLoiDocDuoc(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)
	nguoi := nguoiPg("nd-quan-tri")

	if _, err := uc.Them(ctx, YeuCauThemCanBo{
		HoTen: "Trần Thị B", Email: "trung@xa.danang.gov.vn",
	}, nguoi); err != nil {
		t.Fatalf("Them lần đầu: %v", err)
	}
	_, err := uc.Them(ctx, YeuCauThemCanBo{
		HoTen: "Lê Văn C", Email: "trung@xa.danang.gov.vn",
	}, nguoi)
	if !errors.Is(err, idstore.ErrEmailDaDung) {
		t.Fatalf("thư điện tử trùng: lỗi = %v, muốn ErrEmailDaDung", err)
	}

	_, err = uc.Them(ctx, YeuCauThemCanBo{
		HoTen: "Lê Văn C", Email: "khac@xa.danang.gov.vn", BoPhanID: "bp-khong-co",
	}, nguoi)
	if !errors.Is(err, idstore.ErrBoPhanKhongTonTai) {
		t.Fatalf("bộ phận không tồn tại: lỗi = %v, muốn ErrBoPhanKhongTonTai", err)
	}
}

// --- #14, second constraint, against the real grant tables -------------------------------------------

// THE CALLER MAY NOT HAND OUT A KEY THEY DO NOT HOLD, and both halves of the comparison are read
// from `vai_tro_quyen` inside the transaction.
func TestPgKhongTraoDuocQuyenMinhKhongCam(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	yeu, manh := "nd-yeu", "nd-muc-tieu"

	// The caller holds admin.user and nothing else; the destination role also carries
	// budget.confirm.
	dungXaPg(t, db, xa, "vt-chi-admin", []string{"admin.user"}, yeu)
	dungXaPg(t, db, xa, "vt-manh", []string{"admin.user", "budget.confirm"})
	dungXaPg(t, db, xa, "vt-trung-tinh", []string{"document.read"}, manh)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	_, err := ucThat(db).DoiVaiTro(ctx, manh, "vt-manh", nguoiPg(yeu))
	if !errors.Is(err, ErrTraoQuyenKhongCam) {
		t.Fatalf("lỗi = %v, muốn ErrTraoQuyenKhongCam (câu #14)", err)
	}

	var vaiTro sql.NullString
	if err := db.QueryRow(`SELECT vai_tro_id FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`,
		xa, manh).Scan(&vaiTro); err != nil {
		t.Fatal(err)
	}
	if vaiTro.String != "vt-trung-tinh" {
		t.Fatalf("vai trò đã bị ghi dù thao tác bị từ chối: %q", vaiTro.String)
	}
}
