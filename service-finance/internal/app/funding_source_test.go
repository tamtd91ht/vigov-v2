package app

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// What these cases hold, each a property that fails silently if it stops holding:
//
//  1. the source, its year amount and the audit entry commit in ONE transaction — and a refusal or a
//     failed entry commits NOTHING (rule 6, invariant 3);
//  2. the trail names the STAFF BUSINESS CODE and the source's name, never an internal id (rule 6,
//     invariant 8);
//  3. the commune is $1 of every statement, taken from the context (rule 1);
//  4. the name is checked and stored TRIMMED (0013's CHECK) and a taken name is refused before any
//     insert, including the race the unique key catches;
//  5. a granted amount left blank writes no year row; an explicit 0 does;
//  6. setting the figure already recorded writes and audits nothing (the route's idem.KhongCan).

func clerk() audit.Actor { return audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"} }

func amountPtr(v domain.Dong) *domain.Dong { return &v }

func int64Ptr(v int64) *int64 { return &v }

// auditDelta decodes the delta of the ONE audit entry the case expects.
func fundingSourceEntry(t *testing.T, k *fakeFundingSourceDB) (args []any, delta map[string]any) {
	t.Helper()
	entries := k.stmts("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d vết, muốn đúng 1", len(entries))
	}
	for _, a := range entries[0].args {
		args = append(args, a)
	}
	if err := json.Unmarshal(entries[0].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta không phải JSON: %v", err)
	}
	return args, delta
}

func TestCreateFundingSourceWritesSourceAmountAndEntryInOneTransaction(t *testing.T) {
	k := &fakeFundingSourceDB{nextOrder: 4}
	uc, ctx := buildFundingSources(t, k)

	got, err := uc.AddFundingSource(ctx, FundingSourceCreateRequest{
		Name: "  Ngân sách xã, phường ", Year: 2026, GrantedAmount: amountPtr(9_200_000_000),
	}, clerk())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if k.begun != 1 || k.committed != 1 || k.rolledBack != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0", k.begun, k.committed, k.rolledBack)
	}
	if got.ID != idSourceNew || got.Ten != "Ngân sách xã, phường" || got.ThuTu != 4 ||
		got.Nam != 2026 || got.TongNguon != 9_200_000_000 {
		t.Fatalf("trả về %+v", got)
	}

	// The name is checked and stored TRIMMED; the commune is $1 everywhere.
	if s := k.stmts("ten = $2"); len(s) != 1 || s[0].args[1] != "Ngân sách xã, phường" {
		t.Fatalf("kiểm tên trùng với %v — muốn tên đã cắt", s)
	}
	ins := k.stmts("INSERT INTO nguon_von")
	if len(ins) != 1 || ins[0].args[1] != idSourceNew || ins[0].args[2] != "Ngân sách xã, phường" {
		t.Fatalf("INSERT nguon_von = %v", ins)
	}
	amount := k.stmts("INSERT INTO funding_source_annual_amounts")
	if len(amount) != 1 || amount[0].args[1] != idAmountNew || amount[0].args[2] != idSourceNew ||
		amount[0].args[3] != int64(2026) || amount[0].args[4] != int64(9_200_000_000) {
		t.Fatalf("INSERT vốn được giao = %v", amount)
	}
	for _, s := range k.statements {
		if len(s.args) == 0 || s.args[0] != string(xaA) {
			t.Fatalf("câu không buộc xã ở $1: %q %v", s.sql, s.args)
		}
	}

	args, delta := fundingSourceEntry(t, k)
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if args[1] != "CB-00123" || args[4] != ActionFundingSourceCreate || args[5] != "Ngân sách xã, phường" {
		t.Fatalf("vết: người %v, động từ %v, đối tượng %v", args[1], args[4], args[5])
	}
	after := delta["sau"].(map[string]any)
	if after["granted_amount"] != float64(9_200_000_000) || after["year"] != float64(2026) ||
		after["funding_source_id"] != idSourceNew {
		t.Fatalf("delta sau = %v", after)
	}
}

func TestCreateFundingSourceWithoutAmountWritesNoYearRow(t *testing.T) {
	k := &fakeFundingSourceDB{nextOrder: 1}
	uc, ctx := buildFundingSources(t, k)

	got, err := uc.AddFundingSource(ctx, FundingSourceCreateRequest{Name: "Nguồn xã hội hoá", Year: 2026}, clerk())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(k.stmts("funding_source_annual_amounts")) != 0 {
		t.Fatal("để trống vốn được giao mà vẫn ghi dòng năm — để trống nghĩa là KHÔNG có dòng (đọc ra 0)")
	}
	if got.TongNguon != 0 {
		t.Fatalf("vốn được giao = %d, muốn 0", got.TongNguon)
	}
	_, delta := fundingSourceEntry(t, k)
	if v, ok := delta["sau"].(map[string]any)["granted_amount"]; !ok || v != nil {
		t.Fatalf("delta granted_amount = %v, muốn null (để trống, khác với 0)", v)
	}
}

func TestCreateFundingSourceExplicitZeroIsWritten(t *testing.T) {
	k := &fakeFundingSourceDB{nextOrder: 1}
	uc, ctx := buildFundingSources(t, k)
	if _, err := uc.AddFundingSource(ctx, FundingSourceCreateRequest{
		Name: "Chương trình mục tiêu quốc gia", Year: 2026, GrantedAmount: amountPtr(0),
	}, clerk()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s := k.stmts("INSERT INTO funding_source_annual_amounts"); len(s) != 1 || s[0].args[4] != int64(0) {
		t.Fatalf("số 0 khai rõ phải được ghi: %v", s)
	}
}

func TestCreateFundingSourceRefusalsCommitNothing(t *testing.T) {
	for name, c := range map[string]struct {
		k    *fakeFundingSourceDB
		want error
	}{
		"tên đã có (kể cả nguồn đã xoá mềm)": {&fakeFundingSourceDB{nameCount: 1}, fistore.ErrFundingSourceNameTaken},
		"tên vừa bị tạo song song":           {&fakeFundingSourceDB{insertSourceErr: uniqueViolation}, fistore.ErrFundingSourceNameTaken},
		"danh mục đủ trần":                   {&fakeFundingSourceDB{liveCount: fistore.TranNguonVonMotNam}, fistore.ErrFundingSourceCatalogueFull},
	} {
		t.Run(name, func(t *testing.T) {
			uc, ctx := buildFundingSources(t, c.k)
			_, err := uc.AddFundingSource(ctx, FundingSourceCreateRequest{
				Name: "Ngân sách xã, phường", Year: 2026, GrantedAmount: amountPtr(1),
			}, clerk())
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if c.k.committed != 0 || c.k.rolledBack != 1 {
				t.Fatalf("commit %d, rollback %d — muốn 0/1", c.k.committed, c.k.rolledBack)
			}
			if len(c.k.stmts("funding_source_annual_amounts")) != 0 || len(c.k.stmts("audit_log")) != 0 {
				t.Fatal("bị từ chối mà vẫn ghi số vốn hoặc vết")
			}
		})
	}
}

func TestCreateFundingSourceShapeRefusedBeforeTransaction(t *testing.T) {
	for name, c := range map[string]struct {
		req   FundingSourceCreateRequest
		actor audit.Actor
		want  error
	}{
		"thiếu tên":      {FundingSourceCreateRequest{Name: "  ", Year: 2026}, clerk(), domain.ErrFundingSourceNameMissing},
		"thiếu năm":      {FundingSourceCreateRequest{Name: "A"}, clerk(), domain.ErrFundingSourceYearMissing},
		"năm ngoài lịch": {FundingSourceCreateRequest{Name: "A", Year: 1999}, clerk(), domain.ErrFundingSourceYearOutOfRange},
		"vốn âm":         {FundingSourceCreateRequest{Name: "A", Year: 2026, GrantedAmount: amountPtr(-1)}, clerk(), domain.ErrGrantedAmountNegative},
		"không có người": {FundingSourceCreateRequest{Name: "A", Year: 2026}, audit.Actor{}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			k := &fakeFundingSourceDB{}
			uc, ctx := buildFundingSources(t, k)
			_, err := uc.AddFundingSource(ctx, c.req, c.actor)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if k.begun != 0 {
				t.Fatal("dữ liệu sai dạng mà vẫn mở giao dịch")
			}
		})
	}
}

func TestCreateFundingSourceFailedEntryTakesTheSourceDown(t *testing.T) {
	// The audit entry is the LAST statement; its failure must roll back the source and the amount.
	k := &fakeFundingSourceDB{failOn: "INSERT INTO audit_log"}
	uc, ctx := buildFundingSources(t, k)
	if _, err := uc.AddFundingSource(ctx, FundingSourceCreateRequest{
		Name: "Ngân sách xã, phường", Year: 2026, GrantedAmount: amountPtr(5),
	}, clerk()); err == nil {
		t.Fatal("vết hỏng mà Create vẫn thành công")
	}
	if len(k.stmts("INSERT INTO nguon_von")) != 1 || k.committed != 0 || k.rolledBack != 1 {
		t.Fatalf("nguồn đã chèn rồi vết hỏng: commit %d, rollback %d — muốn 0/1", k.committed, k.rolledBack)
	}
}

func TestSetGrantedAmountFirstTimeInsertsAndAuditsBeforeNull(t *testing.T) {
	k := &fakeFundingSourceDB{sourceName: "Ngân sách xã, phường"}
	uc, ctx := buildFundingSources(t, k)

	got, err := uc.SetGrantedAmount(ctx, "nv-xa", 2027, 11_000_000_000, clerk())
	if err != nil {
		t.Fatalf("SetGrantedAmount: %v", err)
	}
	if got.FundingSourceID != "nv-xa" || got.Year != 2027 || got.GrantedAmount != 11_000_000_000 {
		t.Fatalf("trả về %+v", got)
	}
	if len(k.stmts("FOR UPDATE")) != 1 {
		t.Fatal("không khoá nguồn trước khi đọc-quyết-ghi số vốn của năm")
	}
	if s := k.stmts("INSERT INTO funding_source_annual_amounts"); len(s) != 1 ||
		s[0].args[2] != "nv-xa" || s[0].args[3] != int64(2027) || s[0].args[4] != int64(11_000_000_000) {
		t.Fatalf("INSERT = %v", s)
	}
	if len(k.stmts("UPDATE funding_source_annual_amounts")) != 0 {
		t.Fatal("lần đầu mà lại UPDATE")
	}
	args, delta := fundingSourceEntry(t, k)
	if args[1] != "CB-00123" || args[4] != ActionFundingSourceGrantedSet || args[5] != "Ngân sách xã, phường" {
		t.Fatalf("vết: %v %v %v", args[1], args[4], args[5])
	}
	if b := delta["truoc"].(map[string]any)["granted_amount"]; b != nil {
		t.Fatalf("trước = %v, muốn null (chưa từng nhập)", b)
	}
	if a := delta["sau"].(map[string]any)["granted_amount"]; a != float64(11_000_000_000) {
		t.Fatalf("sau = %v", a)
	}
}

func TestSetGrantedAmountCorrectionUpdatesAndAuditsBeforeAfter(t *testing.T) {
	// A PAST year's figure is correctable (stated assumption) — and the entry keeps what it was.
	k := &fakeFundingSourceDB{sourceName: "Ngân sách xã, phường", amount: int64Ptr(9_200_000_000)}
	uc, ctx := buildFundingSources(t, k)

	if _, err := uc.SetGrantedAmount(ctx, "nv-xa", 2025, 9_500_000_000, clerk()); err != nil {
		t.Fatalf("SetGrantedAmount: %v", err)
	}
	if s := k.stmts("UPDATE funding_source_annual_amounts"); len(s) != 1 ||
		s[0].args[0] != string(xaA) || s[0].args[1] != idAmountExisting || s[0].args[2] != int64(9_500_000_000) {
		t.Fatalf("UPDATE = %v", s)
	}
	if len(k.stmts("INSERT INTO funding_source_annual_amounts")) != 0 {
		t.Fatal("đã có dòng mà vẫn INSERT — khoá duy nhất sẽ từ chối")
	}
	_, delta := fundingSourceEntry(t, k)
	if delta["truoc"].(map[string]any)["granted_amount"] != float64(9_200_000_000) ||
		delta["sau"].(map[string]any)["granted_amount"] != float64(9_500_000_000) {
		t.Fatalf("delta = %v", delta)
	}
}

func TestSetGrantedAmountSameFigureWritesNothing(t *testing.T) {
	k := &fakeFundingSourceDB{sourceName: "Ngân sách xã, phường", amount: int64Ptr(7)}
	uc, ctx := buildFundingSources(t, k)
	got, err := uc.SetGrantedAmount(ctx, "nv-xa", 2026, 7, clerk())
	if err != nil || got.GrantedAmount != 7 {
		t.Fatalf("= %+v, %v", got, err)
	}
	// "UPDATE funding_source…", not "UPDATE": the source's FOR UPDATE lock read is expected here.
	if len(k.stmts("UPDATE funding_source_annual_amounts")) != 0 || len(k.stmts("INSERT")) != 0 {
		t.Fatal("gửi lại đúng số đang có mà vẫn ghi — idem.KhongCan của tuyến thành lời nói dối")
	}
}

func TestSetGrantedAmountUnknownSourceIsNotFound(t *testing.T) {
	// Another commune's source looks exactly like this from inside: the FOR UPDATE read binds $1.
	k := &fakeFundingSourceDB{}
	uc, ctx := buildFundingSources(t, k)
	_, err := uc.SetGrantedAmount(ctx, "nv-cua-xa-khac", 2026, 1, clerk())
	if !errors.Is(err, fistore.ErrFundingSourceNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrFundingSourceNotFound", err)
	}
	if len(k.stmts("INSERT")) != 0 || k.committed != 0 {
		t.Fatal("nguồn không có mà vẫn ghi")
	}
}

func TestSetGrantedAmountShapeRefusedBeforeTransaction(t *testing.T) {
	for name, c := range map[string]struct {
		year   int
		amount domain.Dong
		actor  audit.Actor
	}{
		"năm ngoài lịch":    {2101, 1, clerk()},
		"vốn âm":            {2026, -5, clerk()},
		"vượt trần gõ nhầm": {2026, domain.SoTienToiDa + 1, clerk()},
		"không có người":    {2026, 1, audit.Actor{}},
	} {
		t.Run(name, func(t *testing.T) {
			k := &fakeFundingSourceDB{sourceName: "x"}
			uc, ctx := buildFundingSources(t, k)
			if _, err := uc.SetGrantedAmount(ctx, "nv-xa", c.year, c.amount, c.actor); err == nil {
				t.Fatal("được chấp nhận")
			}
			if k.begun != 0 {
				t.Fatal("dữ liệu sai dạng mà vẫn mở giao dịch")
			}
		})
	}
}
