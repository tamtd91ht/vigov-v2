package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// AddFieldRule and RemoveFieldRule (ADR 0079 lô 2 Q4), over the REAL store and the fake driver of
// driver_gia_sla_test.go, with the platform's tier-1 list behind a fake gRPC client.

// platformFieldsFake answers ListPetitionFields.
type platformFieldsFake struct {
	fields []*platformv1.PetitionField
	err    error
	calls  int
}

func (f *platformFieldsFake) ListPetitionFields(context.Context, *platformv1.ListPetitionFieldsRequest,
	...grpc.CallOption) (*platformv1.ListPetitionFieldsResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &platformv1.ListPetitionFieldsResponse{Fields: f.fields}, nil
}

// tier1 holds one active code (`an-ninh`) and one retired code (`cu`).
func tier1() *platformFieldsFake {
	return &platformFieldsFake{fields: []*platformv1.PetitionField{
		{Code: "an-ninh", DefaultLabel: "An ninh", SortOrder: 1, Active: true},
		{Code: "cu", DefaultLabel: "Cũ", SortOrder: 2, Active: false},
	}}
}

func useCaseWithFields(t *testing.T, k *khoSLAGia, f *platformFieldsFake) (*SLA, context.Context) {
	t.Helper()
	uc, ctx := dungUseCaseSLA(t, k)
	uc.WithPetitionFields(platformclient.NewPetitionFields(f, nil))
	return uc, ctx
}

func addReq(kind domain.LoaiViec, field string) AddFieldRuleRequest {
	return AddFieldRuleRequest{Kind: kind, Field: field, AcknowledgeHours: 2, ResolveHours: 16,
		DueSoonHours: 4, EscalateLeaderHours: 8, EscalatePresidentHours: 17}
}

// THE ROW AND ITS AUDIT ENTRY IN ONE TRANSACTION, the row carrying the kind, the field and the five
// figures in their positions, the entry naming the staff code and the commitment.
func TestAddFieldRuleInsertsRowAndAuditInOneTx(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := useCaseWithFields(t, k, tier1())

	row, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecPhanAnh, " an-ninh "), nguoiSLA())
	if err != nil {
		t.Fatalf("AddFieldRule lỗi: %v", err)
	}
	if row.LinhVuc != "an-ninh" || row.ID != idGieo(1) {
		t.Errorf("dòng trả về = %+v", row)
	}
	ins := k.cau("INSERT INTO sla")
	if len(ins) != 1 {
		t.Fatalf("có %d câu INSERT, muốn 1", len(ins))
	}
	a := ins[0].args
	if a[0] != string(xaSLA) || a[2] != "phan-anh" || a[3] != "an-ninh" {
		t.Errorf("INSERT tham số = %v", a)
	}
	gio := []any{a[4], a[5], a[6], a[7], a[8]}
	for i, want := range []int64{2, 16, 4, 8, 17} {
		if gio[i] != want {
			t.Errorf("số giờ vị trí %d = %v, muốn %d", i, gio[i], want)
		}
	}
	if a[9] != nil {
		t.Errorf("unassigned_hold_hours = %v, muốn NULL", a[9])
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	vet := k.cau("audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	v := vet[0].args
	if v[1] != maCanBoSLA || v[4] != ActionAddSLAFieldRow || v[5] != "phan-anh/an-ninh" {
		t.Errorf("vết = actor %v, action %v, subject %v", v[1], v[4], v[5])
	}
}

// THE AUDIT ENTRY FAILING TAKES THE INSERT DOWN WITH IT (rule 6, forbidden #2).
func TestAddFieldRuleAuditFailureRollsBack(t *testing.T) {
	k := &khoSLAGia{loiSau: "audit_log"}
	uc, ctx := useCaseWithFields(t, k, tier1())

	if _, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecPhanAnh, "an-ninh"), nguoiSLA()); err == nil {
		t.Fatal("vết hỏng mà vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 || k.soCau("INSERT INTO sla") != 1 {
		t.Errorf("commit %d rollback %d insert %d — muốn 0/1/1", k.daCommit, k.daRollback, k.soCau("INSERT INTO sla"))
	}
}

// THE LIVE UNIQUE KEY REFUSING IS ErrSLAFieldRowExists — what the handler turns into 409
// `sla_rule_exists`. The error text is the partition's constraint name, as PostgreSQL reports it.
func TestAddFieldRuleDuplicateIsRowExists(t *testing.T) {
	k := &khoSLAGia{loiSau: "INSERT INTO sla",
		loiSauLa: errors.New(`duplicate key value violates unique constraint "sla_p07_tenant_id_loai_viec_linh_vuc_khoa_key"`)}
	uc, ctx := useCaseWithFields(t, k, tier1())

	_, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecPhanAnh, "an-ninh"), nguoiSLA())
	if !errors.Is(err, ErrSLAFieldRowExists) {
		t.Fatalf("lỗi = %v, muốn ErrSLAFieldRowExists", err)
	}
	if k.daCommit != 0 || k.soCau("audit_log") != 0 {
		t.Errorf("trùng khoá mà vẫn commit %d / ghi %d vết", k.daCommit, k.soCau("audit_log"))
	}
}

// THE FIELD IS CHECKED WITH THE PLATFORM, AND EVERY ANSWER OTHER THAN "ACTIVE CODE" WRITES NOTHING.
func TestAddFieldRuleChecksPetitionFieldWithPlatform(t *testing.T) {
	cases := []struct {
		name  string
		field string
		src   *platformFieldsFake
		want  error
	}{
		{"mã không có", "khong-co", tier1(), ErrSLAFieldNotInList},
		{"mã đã ngừng", "cu", tier1(), ErrSLAFieldNotInList},
		{"platform không trả lời", "an-ninh", &platformFieldsFake{err: errors.New("unavailable")}, ErrSLAFieldListUnavailable},
	}
	for _, c := range cases {
		k := &khoSLAGia{}
		uc, ctx := useCaseWithFields(t, k, c.src)
		_, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecPhanAnh, c.field), nguoiSLA())
		if !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		if k.batDau != 0 {
			t.Errorf("%s: mở %d giao dịch — phải từ chối TRƯỚC", c.name, k.batDau)
		}
	}
}

// NO READER WIRED = UNAVAILABLE, never "accept unchecked" (fail closed).
func TestAddFieldRuleWithoutReaderFailsClosed(t *testing.T) {
	k := &khoSLAGia{}
	uc, ctx := dungUseCaseSLA(t, k)
	_, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecPhanAnh, "an-ninh"), nguoiSLA())
	if !errors.Is(err, ErrSLAFieldListUnavailable) || k.batDau != 0 {
		t.Errorf("lỗi = %v, giao dịch %d — muốn ErrSLAFieldListUnavailable và không ghi gì", err, k.batDau)
	}
}

// `don-thu` has the default row only, and an unknown kind is refused — before any owner is asked.
func TestAddFieldRuleRefusesUncheckableKinds(t *testing.T) {
	cases := []struct {
		kind domain.LoaiViec
		want error
	}{
		{domain.LoaiViecDonThu, domain.ErrSLAKindHasNoFieldRows},
		{domain.LoaiViec("la"), domain.ErrSLAKindUnknown},
	}
	for _, c := range cases {
		k := &khoSLAGia{}
		src := tier1()
		uc, ctx := useCaseWithFields(t, k, src)
		_, err := uc.AddFieldRule(ctx, addReq(c.kind, "an-ninh"), nguoiSLA())
		if !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.kind, err, c.want)
		}
		if k.batDau != 0 || src.calls != 0 {
			t.Errorf("%s: giao dịch %d, hỏi platform %d lần — muốn 0/0", c.kind, k.batDau, src.calls)
		}
	}
}

// --- van-ban-den: the document type is checked with service-documents ---------------------------

// documentTypesFake answers code → active for the codes it knows; absent otherwise.
type documentTypesFake struct {
	known map[string]bool
	err   error
	calls int
	asked []string
}

func (f *documentTypesFake) DocumentTypeCodes(_ context.Context, codes []string) (map[string]bool, error) {
	f.calls++
	f.asked = append([]string(nil), codes...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]bool{}
	for _, c := range codes {
		if a, ok := f.known[c]; ok {
			out[c] = a
		}
	}
	return out, nil
}

func documentTypes() *documentTypesFake {
	return &documentTypesFake{known: map[string]bool{"cong-van": true, "to-trinh": false}}
}

// AN ACTIVE TYPE IS WRITTEN, with its audit entry, after ONE question carrying the normalised code —
// and the platform is not asked about a code that is not its own.
func TestAddFieldRuleAcceptsActiveDocumentType(t *testing.T) {
	k := &khoSLAGia{}
	platform := tier1()
	uc, ctx := useCaseWithFields(t, k, platform)
	docs := documentTypes()
	uc.WithDocumentTypes(docs)

	row, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecVanBanDen, " cong-van "), nguoiSLA())
	if err != nil {
		t.Fatalf("AddFieldRule lỗi: %v", err)
	}
	if row.LinhVuc != "cong-van" || docs.calls != 1 || len(docs.asked) != 1 || docs.asked[0] != "cong-van" {
		t.Errorf("row %+v, asked %v (%d calls)", row, docs.asked, docs.calls)
	}
	if platform.calls != 0 {
		t.Errorf("hỏi platform %d lần về một mã loại văn bản", platform.calls)
	}
	if k.soCau("INSERT INTO sla") != 1 || k.soCau("audit_log") != 1 || k.daCommit != 1 {
		t.Errorf("insert %d, vết %d, commit %d — muốn 1/1/1", k.soCau("INSERT INTO sla"), k.soCau("audit_log"), k.daCommit)
	}
}

// EVERY ANSWER OTHER THAN "ACTIVE" WRITES NOTHING: switched off and absent are 400, any error is 503,
// and no client wired is 503 — fail closed.
func TestAddFieldRuleDocumentTypeFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		field string
		src   *documentTypesFake // nil = not wired
		want  error
	}{
		{"loại đã tắt", "to-trinh", documentTypes(), ErrSLAFieldNotInList},
		{"mã không có", "khong-co", documentTypes(), ErrSLAFieldNotInList},
		{"documents không trả lời", "cong-van", &documentTypesFake{err: errors.New("unavailable")}, ErrSLAFieldListUnavailable},
		{"chưa nối documents", "cong-van", nil, ErrSLAFieldListUnavailable},
	}
	for _, c := range cases {
		k := &khoSLAGia{}
		uc, ctx := useCaseWithFields(t, k, tier1())
		if c.src != nil {
			uc.WithDocumentTypes(c.src)
		}
		_, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecVanBanDen, c.field), nguoiSLA())
		if !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		if k.batDau != 0 {
			t.Errorf("%s: mở %d giao dịch — phải từ chối TRƯỚC", c.name, k.batDau)
		}
	}
}

func TestAddFieldRuleRefusesBadShape(t *testing.T) {
	bad := addReq(domain.LoaiViecPhanAnh, "an-ninh")
	bad.EscalatePresidentHours = 4 // below the leader's 8
	cases := []struct {
		name string
		req  AddFieldRuleRequest
		want error
	}{
		{"thiếu lĩnh vực", addReq(domain.LoaiViecPhanAnh, "  "), domain.ErrSLAFieldMissing},
		{"lĩnh vực có khoảng trắng", addReq(domain.LoaiViecPhanAnh, "an ninh"), domain.ErrSLAFieldMalformed},
		{"Y < X", bad, domain.ErrChairmanBeforeUnitHead},
		{"số giờ 0", AddFieldRuleRequest{Kind: domain.LoaiViecPhanAnh, Field: "an-ninh"}, domain.ErrGioPhaiDuong},
	}
	for _, c := range cases {
		k := &khoSLAGia{}
		uc, ctx := useCaseWithFields(t, k, tier1())
		if _, err := uc.AddFieldRule(ctx, c.req, nguoiSLA()); !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		if k.batDau != 0 {
			t.Errorf("%s: mở giao dịch", c.name)
		}
	}
}

// --- RemoveFieldRule ------------------------------------------------------------------------------

// SOFT DELETE, THREE COLUMNS, STAFF CODE AS deleted_by, AND THE ENTRY IN THE SAME TRANSACTION.
func TestRemoveFieldRuleSoftDeletesWithReasonAndAudit(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	if err := uc.RemoveFieldRule(ctx, idDongSLA, "  Gộp vào dòng mặc định ", nguoiSLA()); err != nil {
		t.Fatalf("RemoveFieldRule lỗi: %v", err)
	}
	if k.coCau("DELETE") {
		t.Error("có câu DELETE — luật 7 cấm xoá cứng")
	}
	upd := k.cau("UPDATE sla")
	if len(upd) != 1 {
		t.Fatalf("có %d câu UPDATE, muốn 1", len(upd))
	}
	u := upd[0]
	for _, col := range []string{"deleted_at", "deleted_by", "delete_reason", "linh_vuc IS NOT NULL", "deleted_at IS NULL"} {
		if !strings.Contains(u.sql, col) {
			t.Errorf("câu UPDATE thiếu %q: %s", col, u.sql)
		}
	}
	if u.args[0] != string(xaSLA) || u.args[1] != idDongSLA || u.args[2] != maCanBoSLA || u.args[3] != "Gộp vào dòng mặc định" {
		t.Errorf("tham số = %v", u.args)
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit %d rollback %d", k.daCommit, k.daRollback)
	}
	vet := k.cau("audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	v := vet[0].args
	if v[1] != maCanBoSLA || v[4] != ActionRemoveSLAFieldRow || v[5] != "phan-anh/an-ninh" {
		t.Errorf("vết = actor %v, action %v, subject %v", v[1], v[4], v[5])
	}
	var delta struct {
		LyDo   string `json:"ly_do"`
		XoaMem bool   `json:"xoa_mem"`
	}
	b, _ := v[7].([]byte)
	if err := json.Unmarshal(b, &delta); err != nil || delta.LyDo != "Gộp vào dòng mặc định" || !delta.XoaMem {
		t.Errorf("delta = %s", b)
	}
}

// THE DEFAULT ROW IS NEVER REMOVED — refused before any UPDATE.
func TestRemoveFieldRuleRefusesDefaultRow(t *testing.T) {
	k := &khoSLAGia{hang: []hangSLA{{id: idDongSLA, loaiViec: "phan-anh", linhVuc: nil, gio: [5]int{8, 56, 24, 24, 48}}}}
	uc, ctx := dungUseCaseSLA(t, k)

	err := uc.RemoveFieldRule(ctx, idDongSLA, "lý do", nguoiSLA())
	if !errors.Is(err, domain.ErrSLADefaultRowNotRemovable) {
		t.Fatalf("lỗi = %v, muốn ErrSLADefaultRowNotRemovable", err)
	}
	if k.coCau("UPDATE sla") || k.coCau("audit_log") || k.daCommit != 0 {
		t.Error("dòng mặc định bị đụng tới")
	}
}

// ANOTHER COMMUNE'S ROW, AN INVENTED ID AND A REMOVED ROW ARE ONE ANSWER: not found.
func TestRemoveFieldRuleUnknownIDIsNotFound(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)

	err := uc.RemoveFieldRule(ctx, "01JKHONGCODONGNAO00000000X", "lý do", nguoiSLA())
	if !errors.Is(err, idstore.ErrDongSLAKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrDongSLAKhongTonTai", err)
	}
	if k.coCau("UPDATE sla") || k.coCau("audit_log") {
		t.Error("id lạ mà vẫn ghi")
	}
}

func TestRemoveFieldRuleNeedsReason(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)
	if err := uc.RemoveFieldRule(ctx, idDongSLA, "   ", nguoiSLA()); !errors.Is(err, domain.ErrSLADeleteReasonMissing) {
		t.Fatalf("lỗi = %v, muốn ErrSLADeleteReasonMissing", err)
	}
	if k.batDau != 0 {
		t.Error("thiếu lý do mà vẫn mở giao dịch")
	}
}

func TestRemoveFieldRuleRefusesActorWithoutStaffCode(t *testing.T) {
	k := &khoSLAGia{hang: dongDeSua()}
	uc, ctx := dungUseCaseSLA(t, k)
	n := nguoiSLA()
	n.Vet.ID = ""
	if err := uc.RemoveFieldRule(ctx, idDongSLA, "lý do", n); err == nil || k.batDau != 0 {
		t.Errorf("thiếu mã cán bộ: lỗi %v, giao dịch %d", err, k.batDau)
	}
}

// --- nhiem-vu: the task priority is checked with service-petitions -------------------------------

// taskPrioritiesFake answers code → active for the codes it knows; absent otherwise.
type taskPrioritiesFake struct {
	known map[string]bool
	err   error
	calls int
	asked []string
}

func (f *taskPrioritiesFake) TaskPriorityCodes(_ context.Context, codes []string) (map[string]bool, error) {
	f.calls++
	f.asked = append([]string(nil), codes...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]bool{}
	for _, c := range codes {
		if a, ok := f.known[c]; ok {
			out[c] = a
		}
	}
	return out, nil
}

func taskPriorities() *taskPrioritiesFake {
	return &taskPrioritiesFake{known: map[string]bool{"khan": true, "cao": false}}
}

// AN ACTIVE PRIORITY IS WRITTEN, with its audit entry, after ONE question carrying the normalised code
// — and neither the platform nor documents is asked about a code that is not theirs.
func TestAddFieldRuleAcceptsActiveTaskPriority(t *testing.T) {
	k := &khoSLAGia{}
	platform := tier1()
	uc, ctx := useCaseWithFields(t, k, platform)
	docs := documentTypes()
	prios := taskPriorities()
	uc.WithDocumentTypes(docs).WithTaskPriorities(prios)

	row, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecNhiemVu, " khan "), nguoiSLA())
	if err != nil {
		t.Fatalf("AddFieldRule lỗi: %v", err)
	}
	if row.LinhVuc != "khan" || prios.calls != 1 || len(prios.asked) != 1 || prios.asked[0] != "khan" {
		t.Errorf("row %+v, asked %v (%d calls)", row, prios.asked, prios.calls)
	}
	if platform.calls != 0 || docs.calls != 0 {
		t.Errorf("hỏi platform %d lần, documents %d lần về một mã mức ưu tiên", platform.calls, docs.calls)
	}
	if k.soCau("INSERT INTO sla") != 1 || k.soCau("audit_log") != 1 || k.daCommit != 1 {
		t.Errorf("insert %d, vết %d, commit %d — muốn 1/1/1", k.soCau("INSERT INTO sla"), k.soCau("audit_log"), k.daCommit)
	}
}

// EVERY ANSWER OTHER THAN "ACTIVE" WRITES NOTHING: switched off and absent are 400, any error is 503,
// and no client wired is 503 — fail closed.
func TestAddFieldRuleTaskPriorityFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		field string
		src   *taskPrioritiesFake // nil = not wired
		want  error
	}{
		{"mức đã tắt", "cao", taskPriorities(), ErrSLAFieldNotInList},
		{"mã không có", "khong-co", taskPriorities(), ErrSLAFieldNotInList},
		{"petitions không trả lời", "khan", &taskPrioritiesFake{err: errors.New("unavailable")}, ErrSLAFieldListUnavailable},
		{"chưa nối petitions", "khan", nil, ErrSLAFieldListUnavailable},
	}
	for _, c := range cases {
		k := &khoSLAGia{}
		uc, ctx := useCaseWithFields(t, k, tier1())
		if c.src != nil {
			uc.WithTaskPriorities(c.src)
		}
		_, err := uc.AddFieldRule(ctx, addReq(domain.LoaiViecNhiemVu, c.field), nguoiSLA())
		if !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		if k.batDau != 0 {
			t.Errorf("%s: mở %d giao dịch — phải từ chối TRƯỚC", c.name, k.batDau)
		}
	}
}
