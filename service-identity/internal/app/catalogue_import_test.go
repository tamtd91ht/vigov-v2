package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The catalogue import over a REAL transaction boundary (moDB, dang_nhap_giao_dich_test.go). The row
// rules are the planner's (domain/catalogue_import_test.go); what is proved here is what the
// transaction owns: lock → snapshot → inserts + one entry per row, all in ONE transaction, and a
// refusal or a late failure that leaves nothing committed.

// catalogueRepoFake records Chen inside the transaction it is handed.
type catalogueRepoFake[T any] struct {
	existing []domain.ExistingCatalogueEntry
	chenErr  error

	locks, snapshots int
	inserted         []T
	txOfInsert       []*store.ScopedTx
}

func (f *catalogueRepoFake[T]) LockImport(context.Context, *store.ScopedTx) error {
	f.locks++
	return nil
}

func (f *catalogueRepoFake[T]) ImportSnapshot(context.Context, *store.ScopedTx) ([]domain.ExistingCatalogueEntry, error) {
	f.snapshots++
	return f.existing, nil
}

func (f *catalogueRepoFake[T]) Chen(_ context.Context, tx *store.ScopedTx, row T) error {
	if f.chenErr != nil {
		return f.chenErr
	}
	f.inserted = append(f.inserted, row)
	f.txOfInsert = append(f.txOfInsert, tx)
	return nil
}

func catalogueActor() NguoiThucHien {
	return NguoiThucHien{ID: idNoiBo, Vet: audit.Actor{ID: maCanBo, Kind: "staff", IP: ipGia}}
}

func catalogueRows() []domain.CatalogueImportRow {
	return []domain.CatalogueImportRow{{Row: 2, Label: "Khu phố", Order: "3"}, {Row: 3, Label: "Ấp", Code: "ap"}}
}

func pinIDs[T any](uc *CatalogueImporter[T]) {
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("01JCATALOGUE%014d", n), nil
	}
}

func TestCatalogueImport_BothCataloguesWriteRowsAndEntriesInOneTransaction(t *testing.T) {
	for _, c := range []struct {
		name   string
		action string
		run    func(db *store.DB) (CatalogueImportResult, int, error)
	}{
		{"loại đơn vị dân cư", HanhViThemLoaiDonViDanCu, func(db *store.DB) (CatalogueImportResult, int, error) {
			repo := &catalogueRepoFake[domain.LoaiDonViDanCu]{}
			uc := NewResidentialUnitTypeImporter(db, repo)
			pinIDs(uc)
			res, err := uc.Import(tenant.Into(context.Background(), xaThu), catalogueRows(), catalogueActor())
			for _, r := range repo.inserted {
				if !r.DangDung || r.LaMacDinh || r.Nguon != domain.NguonDonVi {
					t.Errorf("dòng nhập phải là bậc 1, đang dùng, không mặc định: %+v", r)
				}
			}
			return res, len(repo.inserted), err
		}},
		{"khối nhiệm vụ", HanhViThemKhoiNhiemVu, func(db *store.DB) (CatalogueImportResult, int, error) {
			repo := &catalogueRepoFake[domain.KhoiNhiemVu]{}
			uc := NewTaskBlocImporter(db, repo)
			pinIDs(uc)
			res, err := uc.Import(tenant.Into(context.Background(), xaThu), catalogueRows(), catalogueActor())
			return res, len(repo.inserted), err
		}},
	} {
		db, g := moDB(t)
		res, n, err := c.run(db)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if n != 2 || len(res.Entries) != 2 || res.Entries[0].Code != "khu-pho" || res.Entries[0].ID == "" {
			t.Fatalf("%s: tạo %d, kết quả %+v", c.name, n, res.Entries)
		}
		if g.soGiaoDich() != 1 || g.ketThucCua(1) != "commit" {
			t.Fatalf("%s: %d giao dịch, kết thúc %q — muốn MỘT giao dịch commit", c.name, g.soGiaoDich(), g.ketThucCua(1))
		}
		var entries []lenhGhi
		for _, l := range g.lenh {
			if strings.Contains(l.sql, "INSERT INTO audit_log") {
				entries = append(entries, l)
			}
		}
		if len(entries) != 2 {
			t.Fatalf("%s: %d vết, muốn một vết cho mỗi dòng", c.name, len(entries))
		}
		batch := ""
		for i, e := range entries {
			if e.tx != 1 || e.args[1] != maCanBo || e.args[4] != c.action {
				t.Errorf("%s: vết %d: tx=%d actor=%v action=%v", c.name, i, e.tx, e.args[1], e.args[4])
			}
			var delta map[string]any
			b, _ := e.args[7].([]byte)
			if err := json.Unmarshal(b, &delta); err != nil || delta["nguon"] != catalogueImportSource || delta["lo_nhap"] == nil {
				t.Errorf("%s: delta %s (%v)", c.name, b, err)
			}
			if batch == "" {
				batch, _ = delta["lo_nhap"].(string)
			} else if delta["lo_nhap"] != batch {
				t.Errorf("%s: hai dòng của một tệp phải chung mã lô", c.name)
			}
		}
		if entries[0].args[5] != "khu-pho" || entries[1].args[5] != "ap" {
			t.Errorf("%s: chủ thể vết phải là MÃ: %v %v", c.name, entries[0].args[5], entries[1].args[5])
		}
	}
}

func TestCatalogueImport_OneBadRowWritesNothing(t *testing.T) {
	db, g := moDB(t)
	repo := &catalogueRepoFake[domain.LoaiDonViDanCu]{existing: []domain.ExistingCatalogueEntry{{Code: "ap", Label: "Ấp cũ", Deleted: true}}}
	uc := NewResidentialUnitTypeImporter(db, repo)
	_, err := uc.Import(tenant.Into(context.Background(), xaThu), catalogueRows(), catalogueActor())
	var rej *CatalogueImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 3 {
		t.Fatalf("lỗi = %v, muốn đúng một lỗi dòng 3 (mã của dòng đã xoá)", err)
	}
	if len(repo.inserted) != 0 || g.tim("INSERT INTO audit_log") != nil || g.ketThucCua(1) != "rollback" {
		t.Errorf("tệp có lỗi mà vẫn ghi: %d dòng, kết thúc %q", len(repo.inserted), g.ketThucCua(1))
	}
	if repo.locks != 1 {
		t.Errorf("phải khoá lượt nhập trước khi đọc ảnh chụp: %d", repo.locks)
	}
}

func TestCatalogueImport_LateFailureRollsBackEverything(t *testing.T) {
	db, g := moDB(t)
	g.loiTheo["INSERT INTO audit_log"] = errors.New("ổ đĩa đầy")
	repo := &catalogueRepoFake[domain.KhoiNhiemVu]{}
	uc := NewTaskBlocImporter(db, repo)
	if _, err := uc.Import(tenant.Into(context.Background(), xaThu), catalogueRows(), catalogueActor()); err == nil {
		t.Fatal("vết lỗi mà nhập vẫn thành công")
	}
	if g.ketThucCua(1) != "rollback" {
		t.Errorf("vết lỗi phải huỷ cả tệp: %q", g.ketThucCua(1))
	}
}

func TestCatalogueImport_RefusesActorWithoutStaffCode(t *testing.T) {
	db, g := moDB(t)
	uc := NewTaskBlocImporter(db, &catalogueRepoFake[domain.KhoiNhiemVu]{})
	if _, err := uc.Import(tenant.Into(context.Background(), xaThu), catalogueRows(), NguoiThucHien{ID: idNoiBo}); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn nhập")
	}
	if g.soGiaoDich() != 0 {
		t.Error("từ chối người thực hiện phải xảy ra trước khi mở giao dịch")
	}
}

func TestCatalogueImport_PreviewWritesNothing(t *testing.T) {
	db, g := moDB(t)
	repo := &catalogueRepoFake[domain.LoaiDonViDanCu]{}
	uc := NewResidentialUnitTypeImporter(db, repo)
	res, err := uc.Preview(tenant.Into(context.Background(), xaThu), catalogueRows())
	if err != nil || len(res.Entries) != 2 || len(res.Errors) != 0 {
		t.Fatalf("xem trước: %+v %v", res, err)
	}
	if len(repo.inserted) != 0 || repo.locks != 0 || g.ketThucCua(1) != "rollback" {
		t.Errorf("xem trước phải không khoá, không ghi, và luôn rollback: %d %d %q", len(repo.inserted), repo.locks, g.ketThucCua(1))
	}
}

// OVER THE REAL STORE: the lock, the snapshot and the inserts all bind the commune of the context at
// $1, and the insert is the form's statement (nguon a literal).
func TestCatalogueImport_RealStoreBindsTheCommune(t *testing.T) {
	db, g := moDB(t)
	uc := NewResidentialUnitTypeImporter(db, idstore.NewLoaiDonViDanCuStore(db))
	pinIDs(uc)
	if _, err := uc.Import(tenant.Into(context.Background(), xaThu), catalogueRows(), catalogueActor()); err != nil {
		t.Fatalf("Import: %v", err)
	}
	for _, want := range []string{"pg_advisory_xact_lock", "deleted_at IS NOT NULL", "INSERT INTO loai_don_vi_dan_cu"} {
		if g.tim(want) == nil {
			t.Errorf("thiếu câu lệnh %q", want)
		}
	}
	if ins := g.tim("INSERT INTO loai_don_vi_dan_cu"); ins != nil && !strings.Contains(ins.sql, "'don-vi'") {
		t.Errorf("nguon phải là hằng trong câu chèn: %s", ins.sql)
	}
	for _, l := range g.lenh {
		if len(l.args) == 0 || l.args[0] != string(xaThu) {
			t.Errorf("câu lệnh không mang xã của ngữ cảnh ở $1: %q %v", l.sql, l.args)
		}
	}
}
