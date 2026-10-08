package http

// The THIRTEEN routes of SỔ ĐƠN THƯ CÔNG DÂN at the edge:
//
//  1. rule 5 invariant 7 on every route — 401 · 403 wrong key · 403 right key WRONG COMMUNE · 2xx;
//  2. each route's gate key is the seeded literal (`petition.create` / `petition.read`);
//  3. the "assignee OR petition.create" half reaches the use case with the caller's BUSINESS CODE and
//     the checker's answer for `petition.create` in THIS commune;
//  4. masking: a denunciation carries neither identity nor summary on the list, in the duplicate
//     warning or in the report; the drawer shows its sender to the assignee only; the phone is always
//     masked and the address never leaves;
//  5. the duplicate check reads the sender from the POST BODY, never the URL;
//  6. booking is idempotent per Idempotency-Key.
//
// Test data uses the agreed fake number 0900000000 (rule 3, invariant 5).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const (
	pathLetters = "/api/v1/citizen-letters"

	ordinaryName    = "Nguyễn Văn A"
	ordinarySummary = "Đề nghị sửa đường liên thôn bị sạt lở"
	whistleName     = "Trần Thị Tố Giác"
	whistleSummary  = "Tố cáo việc thu tiền trái quy định tại bộ phận một cửa"
	fakePhone       = "0900000000"
	maskedFakePhone = "09****0000"
	senderAddress   = "Thôn Bình An"
	otherOfficer    = "CB-00999"
)

// --- the fake ---------------------------------------------------------------------------------------

// citizenLettersFake stands in for app.CitizenLetters. ITS ROWS ARE KEYED BY COMMUNE, read from the
// context exactly as the store reads it — a fake ignoring the commune would let a wrong-commune test
// pass while proving nothing. Detail applies domain.DetailDisclosure, the same function the use case
// applies, so the masking asserted below is the masking production computes.
type citizenLettersFake struct {
	mu         sync.Mutex
	rows       map[tenant.ID][]domain.CitizenLetter
	dupOut     []domain.DuplicateCandidate
	calls      int
	books      int
	lastTenant tenant.ID
	lastCaller app.LetterCaller
	lastDup    app.DuplicateQuery
	lastList   app.LetterListQuery
	lastDue    *time.Time
	lastBook   app.BookLetterRequest
	lastResult app.LetterResultRequest
	bookErr    error // when set, Book refuses with it — the edge's mapping is what is under test
}

func citizenLettersSample() *citizenLettersFake {
	received := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	booked := time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC)
	mk := func(id string, n int, t domain.LetterType, name, summary, assignee string) domain.CitizenLetter {
		return domain.CitizenLetter{ID: id, Number: n, Year: 2026, ReceivedDate: received, Type: t,
			SenderName: name, SenderPhone: fakePhone, SenderAddress: senderAddress, Summary: summary,
			Status: domain.LetterStatusNew, AssigneeCode: assignee, CreatedByCode: "CB-00001",
			CreatedAt: booked, UpdatedAt: booked}
	}
	return &citizenLettersFake{rows: map[tenant.ID][]domain.CitizenLetter{
		xaA: {
			mk("dt-a-001", 1, domain.LetterTypeFeedback, ordinaryName, ordinarySummary, otherOfficer),
			mk("dt-a-002", 2, domain.LetterTypeDenunciation, whistleName, whistleSummary, otherOfficer),
			mk("dt-a-003", 3, domain.LetterTypeDenunciation, whistleName, whistleSummary, maCanBo),
		},
		xaB: {mk("dt-b-001", 1, domain.LetterTypeComplaint, "Lê Văn C", "Khiếu nại quyết định thu hồi đất", "")},
	}}
}

func (f *citizenLettersFake) note(ctx context.Context, c app.LetterCaller) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastTenant = tenant.MustFrom(ctx)
	f.lastCaller = c
}

func (f *citizenLettersFake) find(ctx context.Context, id string) (domain.CitizenLetter, error) {
	for _, l := range f.rows[tenant.MustFrom(ctx)] {
		if l.ID == id {
			return l, nil
		}
	}
	return domain.CitizenLetter{}, docstore.ErrCitizenLetterNotFound
}

func (f *citizenLettersFake) Book(ctx context.Context, req app.BookLetterRequest, c app.LetterCaller) (domain.CitizenLetter, error) {
	f.note(ctx, c)
	f.books++
	f.lastBook = req
	if f.bookErr != nil {
		return domain.CitizenLetter{}, f.bookErr
	}
	return domain.CitizenLetter{ID: "dt-moi", Number: 4, Year: 2026, ReceivedDate: req.ReceivedDate, Type: req.Type,
		Source:     req.Source,
		SenderName: req.SenderName, SenderPhone: req.SenderPhone, SenderAddress: req.SenderAddress,
		Summary: req.Summary, Status: domain.LetterStatusNew, CreatedByCode: c.Actor.ID}, nil
}
func (f *citizenLettersFake) Route(ctx context.Context, id string, _ app.RouteLetterRequest, c app.LetterCaller) (domain.CitizenLetter, error) {
	f.note(ctx, c)
	return f.find(ctx, id)
}
func (f *citizenLettersFake) Move(ctx context.Context, id string, _ app.MoveLetterRequest, c app.LetterCaller) (domain.CitizenLetter, error) {
	f.note(ctx, c)
	return f.find(ctx, id)
}
func (f *citizenLettersFake) RecordResult(ctx context.Context, id string, req app.LetterResultRequest, c app.LetterCaller) (domain.CitizenLetter, error) {
	f.note(ctx, c)
	f.lastResult = req
	return f.find(ctx, id)
}
func (f *citizenLettersFake) CorrectSender(ctx context.Context, id string, _ app.SenderCorrection, c app.LetterCaller) (domain.CitizenLetter, error) {
	f.note(ctx, c)
	return f.find(ctx, id)
}
func (f *citizenLettersFake) SetDeadline(ctx context.Context, id string, due time.Time, c app.LetterCaller) (domain.CitizenLetter, error) {
	f.note(ctx, c)
	f.lastDue = &due
	l, err := f.find(ctx, id)
	if err != nil {
		return l, err
	}
	return l.SetActiveDue(due)
}
func (f *citizenLettersFake) AddNote(ctx context.Context, id, content string, c app.LetterCaller) (domain.LetterLogEntry, error) {
	f.note(ctx, c)
	if _, err := f.find(ctx, id); err != nil {
		return domain.LetterLogEntry{}, err
	}
	return domain.LetterLogEntry{ID: "nk-moi", LetterID: id, At: testNow, ActorCode: c.Actor.ID,
		Kind: domain.LetterLogNote, Content: content}, nil
}
func (f *citizenLettersFake) Detail(ctx context.Context, id string, c app.LetterCaller) (domain.CitizenLetter, domain.LetterDisclosure, error) {
	f.note(ctx, c)
	l, err := f.find(ctx, id)
	if err != nil {
		return l, domain.LetterDisclosure{}, err
	}
	return l, domain.DetailDisclosure(l, c.Actor.ID, c.CanBook), nil
}
func (f *citizenLettersFake) Log(ctx context.Context, id string) ([]domain.LetterLogEntry, error) {
	f.note(ctx, app.LetterCaller{})
	if _, err := f.find(ctx, id); err != nil {
		return nil, err
	}
	return []domain.LetterLogEntry{{ID: "nk-1", LetterID: id, At: testNow, ActorCode: "CB-00001",
		Kind: domain.LetterLogSenderCorrection, Content: "Sửa thông tin người gửi"}}, nil
}
func (f *citizenLettersFake) List(ctx context.Context, q app.LetterListQuery, _ page.Request) (page.Result[domain.CitizenLetter], error) {
	f.note(ctx, app.LetterCaller{})
	f.lastList = q
	return page.Result[domain.CitizenLetter]{Items: f.rows[tenant.MustFrom(ctx)]}, nil
}
func (f *citizenLettersFake) Count(ctx context.Context, q app.LetterListQuery) (int, error) {
	f.note(ctx, app.LetterCaller{})
	f.lastList = q
	return len(f.rows[tenant.MustFrom(ctx)]), nil
}
func (f *citizenLettersFake) Duplicates(ctx context.Context, q app.DuplicateQuery) ([]domain.DuplicateCandidate, error) {
	f.note(ctx, app.LetterCaller{})
	f.lastDup = q
	return f.dupOut, nil
}
func (f *citizenLettersFake) Report(ctx context.Context, year int) (domain.LetterReport, error) {
	f.note(ctx, app.LetterCaller{})
	return domain.BuildLetterReport(year, f.rows[tenant.MustFrom(ctx)], testNow), nil
}

// --- the route table --------------------------------------------------------------------------------

const bookBody = `{"received_date":"2026-09-22","letter_type":"kien-nghi-phan-anh",` +
	`"sender_name":"` + ordinaryName + `","sender_phone":"` + fakePhone + `","summary":"` + ordinarySummary + `"}`

type letterRoute struct {
	name, method, path, body string
	key                      authz.Perm
	ok                       int
}

func letterRoutes() []letterRoute {
	const create, read authz.Perm = "petition.create", "petition.read"
	one := pathLetters + "/dt-a-001"
	return []letterRoute{
		{"vào sổ", http.MethodPost, pathLetters, bookBody, create, http.StatusCreated},
		{"kiểm trùng", http.MethodPost, pathLetters + "/duplicates", `{"summary":"` + ordinarySummary + `"}`, create, http.StatusOK},
		{"danh sách", http.MethodGet, pathLetters, "", read, http.StatusOK},
		{"chi tiết", http.MethodGet, one, "", read, http.StatusOK},
		{"nhật ký", http.MethodGet, one + "/log", "", read, http.StatusOK},
		{"chuyển", http.MethodPost, one + "/routings", `{"to_unit":"bp-dia-chinh","reason":"Thuộc lĩnh vực đất đai"}`, create, http.StatusOK},
		{"đổi trạng thái", http.MethodPost, one + "/status", `{"status":"dang-xu-ly-don"}`, read, http.StatusOK},
		{"kết quả", http.MethodPut, one + "/result", `{"result_document_no":"12/TB-UBND","result_document_date":"2026-09-25",` +
			`"result_signer":"Chủ tịch UBND xã","result_issuer":"UBND xã","result_summary":"Đã trả lời bằng văn bản"}`, read, http.StatusOK},
		{"sửa người gửi", http.MethodPatch, one + "/sender", `{"sender_address":null}`, create, http.StatusOK},
		{"đặt hạn xử lý", http.MethodPatch, one + "/deadline", `{"due_at":"2026-10-20T17:00:00+07:00"}`, create, http.StatusOK},
		{"ghi nhật ký", http.MethodPost, one + "/log-entries", `{"content":"Đã liên hệ bộ phận địa chính"}`, read, http.StatusCreated},
		{"báo cáo", http.MethodGet, "/api/v1/citizen-letter-report?year=2026", "", read, http.StatusOK},
		{"số đếm", http.MethodGet, "/api/v1/citizen-letter-counts?scope=related", "", read, http.StatusOK},
	}
}

// --- (2) the gate key -------------------------------------------------------------------------------

func TestCitizenLetter_GateKeyIsTheSeededLiteral(t *testing.T) {
	// Compared against LITERALS: a fake checker grants any string, so a route asking for an invented
	// key would pass every other test here while answering 403 to every real account (rule 5, 3c).
	for _, tc := range letterRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.path, canBoCua(xaA), tc.body), http.StatusForbidden)
			if len(m.checker.hoiGi) == 0 {
				t.Fatal("tuyến không hỏi khoá quyền nào")
			}
			want := map[authz.Perm]string{"petition.create": "petition.create", "petition.read": "petition.read"}[tc.key]
			if string(m.checker.hoiGi[0]) != want {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q", m.checker.hoiGi[0], want)
			}
		})
	}
}

// --- (1) rule 5, invariant 7 ------------------------------------------------------------------------

func TestCitizenLetter_401WithoutSession(t *testing.T) {
	for _, tc := range letterRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, tc.key)
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if m.letters.calls != 0 {
				t.Error("chưa đăng nhập mà sổ đơn thư đã bị đụng tới")
			}
		})
	}
}

func TestCitizenLetter_403WrongKey(t *testing.T) {
	for _, tc := range letterRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, "admin.lookup", "document.read")
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.path, canBoCua(xaA), tc.body), http.StatusForbidden)
			if m.letters.calls != 0 {
				t.Error("sai quyền mà sổ đơn thư vẫn chạy")
			}
		})
	}
}

func TestCitizenLetter_403RightKeyWrongCommune(t *testing.T) {
	// The grant lives in commune A; the account of commune B signs in at commune B. A checker that
	// ignored the commune would let commune A's clerk into commune B's register.
	for _, tc := range letterRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, "petition.create", "petition.read")
			doiMa(t, m.goiThan(t, tc.method, hostB, tc.path, canBoCua(xaB), tc.body), http.StatusForbidden)
			if m.letters.calls != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn vào được sổ đơn thư của xã này")
			}
		})
	}
}

func TestCitizenLetter_OKWithRightKeyAndCommune(t *testing.T) {
	for _, tc := range letterRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, tc.key)
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.path, canBoCua(xaA), tc.body), tc.ok)
			if m.letters.lastTenant != xaA {
				t.Fatalf("use case chạy ở xã %q, muốn %q", m.letters.lastTenant, xaA)
			}
		})
	}
}

// --- (3) the caller -----------------------------------------------------------------------------------

func TestCitizenLetter_CallerIsBusinessCodeAndCanBookIsAskedInThisCommune(t *testing.T) {
	for _, grants := range [][]authz.Perm{{"petition.read"}, {"petition.read", "petition.create"}} {
		m := dungMayChu(t)
		m.capQuyen(xaA, grants...)
		doiMa(t, m.goiThan(t, http.MethodPost, hostA, pathLetters+"/dt-a-001/status", canBoCua(xaA),
			`{"status":"dang-xu-ly-don"}`), http.StatusOK)
		c := m.letters.lastCaller
		if c.Actor.ID != maCanBo {
			t.Fatalf("người thực hiện = %q, muốn mã cán bộ %q (luật 6 bất biến 8 — không phải id nội bộ)", c.Actor.ID, maCanBo)
		}
		if want := len(grants) == 2; c.CanBook != want {
			t.Fatalf("CanBook = %v với quyền %v, muốn %v", c.CanBook, grants, want)
		}
	}
}

// --- (4) masking --------------------------------------------------------------------------------------

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
}

func TestCitizenLetter_ListMasksPhoneAndWithholdsDenunciations(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read", "petition.create")
	w := m.goi(t, http.MethodGet, hostA, pathLetters, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	for _, leak := range []string{fakePhone, whistleName, whistleSummary, senderAddress, "sender_address"} {
		if strings.Contains(body, leak) {
			t.Fatalf("danh sách lộ %q", leak)
		}
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	decodeJSON(t, w, &out)
	if len(out.Items) != 3 {
		t.Fatalf("có %d dòng, muốn 3 dòng của xã A", len(out.Items))
	}
	for _, it := range out.Items {
		switch it["letter_type"] {
		case "to-cao":
			if it["sender_name"] != nil || it["sender_phone"] != nil || it["summary"] != nil ||
				it["identity_withheld"] != true || it["summary_withheld"] != true {
				t.Fatalf("đơn tố cáo trên danh sách mang danh tính hoặc trích yếu: %v", it)
			}
		default:
			if it["sender_name"] != ordinaryName || it["sender_phone"] != maskedFakePhone || it["summary"] != ordinarySummary {
				t.Fatalf("đơn thường hiển thị sai: %v", it)
			}
		}
	}
}

func TestCitizenLetter_DetailOfDenunciation(t *testing.T) {
	cases := []struct {
		name               string
		id                 string
		grants             []authz.Perm
		wantName, wantSumm bool
	}{
		{"không được giao, chỉ xem", "dt-a-002", []authz.Perm{"petition.read"}, false, false},
		{"không được giao, có quyền tiếp nhận", "dt-a-002", []authz.Perm{"petition.read", "petition.create"}, false, true},
		{"cán bộ được giao", "dt-a-003", []authz.Perm{"petition.read"}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, tc.grants...)
			w := m.goi(t, http.MethodGet, hostA, pathLetters+"/"+tc.id, canBoCua(xaA))
			doiMa(t, w, http.StatusOK)
			if strings.Contains(w.Body.String(), fakePhone) || strings.Contains(w.Body.String(), senderAddress) {
				t.Fatal("ngăn chi tiết lộ SĐT đầy đủ hoặc địa chỉ — không có khoá xem đầy đủ nào cho đơn thư")
			}
			var out map[string]any
			decodeJSON(t, w, &out)
			if got := out["sender_name"] == whistleName; got != tc.wantName {
				t.Fatalf("thấy danh tính = %v, muốn %v", got, tc.wantName)
			}
			if got := out["summary"] == whistleSummary; got != tc.wantSumm {
				t.Fatalf("thấy trích yếu = %v, muốn %v", got, tc.wantSumm)
			}
			if tc.wantName && (out["sender_phone"] != maskedFakePhone || out["has_sender_address"] != true) {
				t.Fatalf("cán bộ được giao: SĐT phải che và chỉ báo có địa chỉ: %v", out)
			}
			if !tc.wantName && (out["sender_unknown"] != nil || out["identity_withheld"] != true) {
				t.Fatal("người không được giao suy ra được đơn tố cáo có ký tên hay không")
			}
		})
	}
}

func TestCitizenLetter_AnotherCommunesLetterIs404(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read")
	w := m.goi(t, http.MethodGet, hostA, pathLetters+"/dt-b-001", canBoCua(xaA))
	doiMa(t, w, http.StatusNotFound)
	if loiTra(t, w).Message != "Không tìm thấy đơn thư này." {
		t.Fatalf("câu 404 khác câu chung: %q", loiTra(t, w).Message)
	}
}

// --- (5) duplicates ---------------------------------------------------------------------------------

func TestCitizenLetter_DuplicateReadsSenderFromBodyOnly(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.create")
	doiMa(t, m.goiThan(t, http.MethodPost, hostA, pathLetters+"/duplicates?sender_name=X",
		canBoCua(xaA), `{"summary":"`+ordinarySummary+`"}`), http.StatusOK)
	if m.letters.lastDup.SenderName != "" {
		t.Fatal("họ tên người gửi được đọc từ URL — luật 3 cấm #4")
	}
	doiMa(t, m.goiThan(t, http.MethodPost, hostA, pathLetters+"/duplicates",
		canBoCua(xaA), `{"sender_name":"`+ordinaryName+`","summary":"x"}`), http.StatusOK)
	if m.letters.lastDup.SenderName != ordinaryName {
		t.Fatal("họ tên trong thân POST không tới use case")
	}
}

func TestCitizenLetter_DuplicateNeverCarriesADenunciationSummary(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.create")
	rows := m.letters.rows[xaA]
	m.letters.dupOut = []domain.DuplicateCandidate{{Letter: rows[0], Similarity: 0.9}, {Letter: rows[1], Similarity: 0.8}}

	w := m.goiThan(t, http.MethodPost, hostA, pathLetters+"/duplicates", canBoCua(xaA), `{"summary":"x"}`)
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	decodeJSON(t, w, &out)
	// The denunciation is ABSENT, not merely summary-less: its presence on a typed name would say the
	// named sender filed one (ADR 0078 #4).
	if len(out.Items) != 1 || out.Items[0]["id"] != "dt-a-001" || out.Items[0]["summary"] != ordinarySummary {
		t.Fatalf("ứng viên: %v — đơn tố cáo không bao giờ là ứng viên trùng", out.Items)
	}
	if strings.Contains(w.Body.String(), "dt-a-002") || strings.Contains(w.Body.String(), whistleName) ||
		strings.Contains(w.Body.String(), "sender") {
		t.Fatal("cảnh báo trùng lộ đơn tố cáo hoặc thông tin người gửi")
	}

	w = m.goiThan(t, http.MethodPost, hostA, pathLetters+"/duplicates", canBoCua(xaA), `{"summary":"x","letter_type":"to-cao"}`)
	doiMa(t, w, http.StatusOK)
	out.Items = nil
	decodeJSON(t, w, &out)
	if len(out.Items) != 1 || out.Items[0]["id"] != "dt-a-001" {
		t.Fatalf("đơn vào sổ là tố cáo: ứng viên %v — chỉ so với đơn không phải tố cáo", out.Items)
	}
	for _, it := range out.Items {
		if it["summary"] != nil {
			t.Fatal("đơn đang vào sổ là tố cáo mà cảnh báo vẫn trả trích yếu ứng viên")
		}
	}
}

func TestCitizenLetter_ReportNamesNobody(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-letter-report?year=2026", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	for _, leak := range []string{whistleName, whistleSummary, ordinaryName, fakePhone} {
		if strings.Contains(w.Body.String(), leak) {
			t.Fatalf("báo cáo mang %q", leak)
		}
	}
	var out map[string]any
	decodeJSON(t, w, &out)
	if out["received"] != float64(3) || out["on_time_percent"] != nil || out["overdue"] != float64(0) {
		t.Fatalf("số liệu báo cáo: %v", out)
	}
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-letter-report", canBoCua(xaA)), http.StatusBadRequest)
}

// --- the booking body ---------------------------------------------------------------------------------

func TestCitizenLetter_BookingRefusesNumberStatusAndDeadline(t *testing.T) {
	for _, extra := range []string{`"number":7`, `"status":"da-giai-quyet"`, `"processing_due_at":"2026-10-01T00:00:00Z"`} {
		m := dungMayChuCoIdem(t)
		m.capQuyen(xaA, "petition.create")
		body := strings.TrimSuffix(bookBody, "}") + "," + extra + "}"
		doiMa(t, m.goiThan(t, http.MethodPost, hostA, pathLetters, canBoCua(xaA), body), http.StatusBadRequest)
		if m.letters.books != 0 {
			t.Fatalf("%s được nhận thay vì bị từ chối", extra)
		}
	}
}

// ADR 0085 B3: a deadline identity could not give refuses the booking — 503 for an outage (retry), 409
// for a rule the commune must fix — and the client never sees the chain (it carries the commune id).
func TestCitizenLetter_BookingDeadlineFailuresMapToRefusals(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("don_thu: x: %w: identity sập", app.ErrLetterDeadlineUnavailable), http.StatusServiceUnavailable, "deadline_unavailable"},
		{fmt.Errorf("don_thu: x: %w: quy tắc hỏng", app.ErrLetterDeadlineUnusable), http.StatusConflict, "letter_deadline_misconfigured"},
	} {
		m := dungMayChuCoIdem(t)
		m.capQuyen(xaA, "petition.create")
		m.letters.bookErr = c.err
		w := m.goiThan(t, http.MethodPost, hostA, pathLetters, canBoCua(xaA), bookBody)
		doiMa(t, w, c.status)
		if e := loiTra(t, w); e.Code != c.code || strings.Contains(e.Message, "don_thu") {
			t.Fatalf("lỗi trả về: %+v, muốn mã %s và câu tiếng Việt không lộ chuỗi lỗi", e, c.code)
		}
	}
}

func TestCitizenLetter_BookingIsIdempotentPerKey(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	doiMa(t, m.goiThan(t, http.MethodPost, hostA, pathLetters, canBoCua(xaA), bookBody), http.StatusCreated)
	m.goiThan(t, http.MethodPost, hostA, pathLetters, canBoCua(xaA), bookBody)
	if m.letters.books != 1 {
		t.Fatalf("cùng một Idempotency-Key vào sổ %d lần — lần thứ hai lấy số kế tiếp, không trả lại được", m.letters.books)
	}

	// Without the header the route refuses before the use case.
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+pathLetters, strings.NewReader(bookBody))
	r.Host = hostA
	r = r.WithContext(context.WithValue(r.Context(), khoaChuTheThu{}, *canBoCua(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || m.letters.books != 1 {
		t.Fatalf("thiếu %s: mã %d, số lần vào sổ %d", idem.Header, w.Code, m.letters.books)
	}
}

func TestCitizenLetter_SenderPatchIsTriState(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.create")
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, pathLetters+"/dt-a-001/sender", canBoCua(xaA),
		`{"summary":"x"}`), http.StatusBadRequest)
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, pathLetters+"/dt-a-001/sender", canBoCua(xaA),
		`{"sender_name":7}`), http.StatusBadRequest)
}

// The deadline PATCH: the instant reaches the use case EXACTLY as sent (no rounding, no re-zoning to a
// day), null clears it, the answer carries it back, and a body that is not exactly `due_at` is refused
// before the use case runs.
func TestCitizenLetter_DeadlinePatch(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.create")
	path := pathLetters + "/dt-a-001/deadline"

	w := m.goiThan(t, http.MethodPatch, hostA, path, canBoCua(xaA), `{"due_at":"2026-10-20T16:45:30+07:00"}`)
	doiMa(t, w, http.StatusOK)
	want := time.Date(2026, 10, 20, 9, 45, 30, 0, time.UTC)
	if m.letters.lastDue == nil || !m.letters.lastDue.Equal(want) {
		t.Fatalf("hạn tới use case = %v, muốn đúng %v", m.letters.lastDue, want)
	}
	var out map[string]any
	decodeJSON(t, w, &out)
	if out["processing_due_at"] != "2026-10-20T09:45:30Z" || out["resolution_due_at"] != nil {
		t.Fatalf("trả về: processing %v, resolution %v", out["processing_due_at"], out["resolution_due_at"])
	}
	if m.letters.lastCaller.Actor.ID != maCanBo || !m.letters.lastCaller.CanBook {
		t.Fatalf("người đặt hạn = %+v, muốn mã cán bộ và quyền tiếp nhận", m.letters.lastCaller)
	}

	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, path, canBoCua(xaA), `{"due_at":null}`), http.StatusOK)
	if m.letters.lastDue == nil || !m.letters.lastDue.IsZero() {
		t.Fatalf("null phải là bỏ hạn (thời điểm rỗng), được %v", m.letters.lastDue)
	}

	calls := m.letters.calls
	for _, bad := range []string{`{}`, `{"due_at":"2026-10-20"}`, `{"due_at":7}`, `{"due_at":null,"status":"x"}`, `not json`} {
		doiMa(t, m.goiThan(t, http.MethodPatch, hostA, path, canBoCua(xaA), bad), http.StatusBadRequest)
	}
	if m.letters.calls != calls {
		t.Fatal("thân sai mà use case vẫn chạy")
	}

	// Another commune's letter: the same 404 sentence as everywhere (rule 1).
	w = m.goiThan(t, http.MethodPatch, hostA, pathLetters+"/dt-b-001/deadline", canBoCua(xaA), `{"due_at":null}`)
	doiMa(t, w, http.StatusNotFound)
}

func TestCitizenLetter_DeadlineOnFinishedLetterIs409(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.create")
	m.letters.rows[xaA][0].Status = domain.LetterStatusFiled
	w := m.goiThan(t, http.MethodPatch, hostA, pathLetters+"/dt-a-001/deadline", canBoCua(xaA), `{"due_at":null}`)
	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Message != domain.ErrLetterDueOnFinished.Msg {
		t.Fatalf("câu 409 = %q", loiTra(t, w).Message)
	}
}

func TestCitizenLetter_ListScopeAndFiltersAreValidated(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read")
	for _, q := range []string{"?scope=everyone", "?status=da-phan-cong", "?letter_type=phan-anh",
		"?received_from=2026-10-02&received_to=2026-10-01"} {
		doiMa(t, m.goi(t, http.MethodGet, hostA, pathLetters+q, canBoCua(xaA)), http.StatusBadRequest)
	}
	doiMa(t, m.goi(t, http.MethodGet, hostA, pathLetters+"?scope=mine", canBoCua(xaA)), http.StatusOK)
	if m.letters.lastList.Scope != "mine" || m.letters.lastList.CallerCode != maCanBo {
		t.Fatalf("phạm vi `mine` phải lấy mã cán bộ từ phiên: %+v", m.letters.lastList)
	}
}

// --- ADR 0084 (08/10/2026) --------------------------------------------------------------------------

// The booking route IS manual entry: it states `nhap-tay` itself, and a client naming a source is
// refused like a client naming the number.
func TestCitizenLetter_BookingWritesManualSourceAndRefusesAClientSource(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	w := m.goiThan(t, http.MethodPost, hostA, pathLetters, canBoCua(xaA), bookBody)
	doiMa(t, w, http.StatusCreated)
	if m.letters.lastBook.Source != domain.LetterSourceManual {
		t.Fatalf("nguồn tới use case = %q, muốn nhap-tay", m.letters.lastBook.Source)
	}
	var out map[string]any
	decodeJSON(t, w, &out)
	if out["source"] != "nhap-tay" || out["status_group"] != "moi-vao-so" {
		t.Fatalf("trả về: source %v, status_group %v", out["source"], out["status_group"])
	}

	m2 := dungMayChuCoIdem(t)
	m2.capQuyen(xaA, "petition.create")
	body := strings.TrimSuffix(bookBody, "}") + `,"source":"nhap-excel"}`
	w = m2.goiThan(t, http.MethodPost, hostA, pathLetters, canBoCua(xaA), body)
	doiMa(t, w, http.StatusBadRequest)
	if m2.letters.books != 0 || loiTra(t, w).Message != domain.ErrLetterSourceFromClient.Msg {
		t.Fatalf("client tự khai nguồn: vào sổ %d lần, câu %q", m2.letters.books, loiTra(t, w).Message)
	}
}

// `status_group` is a closed set validated at the edge, reaches the store filter, and every row and
// the drawer carry the DERIVED group.
func TestCitizenLetter_StatusGroupFilterAndDerivedField(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read")
	m.letters.rows[xaA][0].HoldingUnitID = "bp-dia-chinh"

	for _, bad := range []string{"cho-phan-cong", "thu-ly", "Moi-vao-so"} {
		doiMa(t, m.goi(t, http.MethodGet, hostA, pathLetters+"?status_group="+bad, canBoCua(xaA)), http.StatusBadRequest)
	}
	for _, g := range domain.LetterStatusGroups() {
		doiMa(t, m.goi(t, http.MethodGet, hostA, pathLetters+"?status_group="+string(g)+"&status=moi-vao-so", canBoCua(xaA)), http.StatusOK)
		if f := m.letters.lastList.Filter; f.StatusGroup != g || f.Status != domain.LetterStatusNew {
			t.Fatalf("bộ lọc tới use case: %+v, muốn nhóm %q và trạng thái moi-vao-so", f, g)
		}
	}

	w := m.goi(t, http.MethodGet, hostA, pathLetters, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	decodeJSON(t, w, &out)
	groups := map[any]any{}
	for _, it := range out.Items {
		groups[it["id"]] = it["status_group"]
		if _, ok := it["source"]; !ok {
			t.Fatalf("dòng thiếu `source`: %v", it)
		}
	}
	if groups["dt-a-001"] != "da-phan-cong" || groups["dt-a-002"] != "moi-vao-so" {
		t.Fatalf("nhóm suy ra trên danh sách: %v", groups)
	}

	w = m.goi(t, http.MethodGet, hostA, pathLetters+"/dt-a-001", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	var one map[string]any
	decodeJSON(t, w, &one)
	if one["status_group"] != "da-phan-cong" {
		t.Fatalf("ngăn chi tiết: status_group %v", one["status_group"])
	}
}

// The tab count: a number only, the list's filters and scope, the caller's code from the session.
func TestCitizenLetter_CountUsesTheListFilters(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read")
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-letter-counts?scope=related&status_group=dang-xu-ly&letter_type=to-cao", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if strings.TrimSpace(w.Body.String()) != `{"count":3}` {
		t.Fatalf("thân = %s, muốn đúng một con số", w.Body.String())
	}
	q := m.letters.lastList
	if q.Scope != "related" || q.CallerCode != maCanBo || q.Filter.StatusGroup != domain.LetterGroupInProgress ||
		q.Filter.Type != domain.LetterTypeDenunciation {
		t.Fatalf("truy vấn đếm: %+v", q)
	}
	calls := m.letters.calls
	for _, bad := range []string{"?scope=everyone", "?status_group=x", "?status=x"} {
		doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-letter-counts"+bad, canBoCua(xaA)), http.StatusBadRequest)
	}
	if m.letters.calls != calls {
		t.Fatal("bộ lọc sai mà vẫn đếm")
	}
}

// ADR 0084 #2: the result route takes the reply alone; which type needs what is the use case's rule.
func TestCitizenLetter_ResultRouteAcceptsReplyOnly(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "petition.read")
	doiMa(t, m.goiThan(t, http.MethodPut, hostA, pathLetters+"/dt-a-001/result", canBoCua(xaA),
		`{"result_summary":"Đã trả lời công dân"}`), http.StatusOK)
	r := m.letters.lastResult
	if r.Summary != "Đã trả lời công dân" || r.DocumentNo != "" || !r.DocumentDate.IsZero() {
		t.Fatalf("kết quả tới use case: %+v", r)
	}
	doiMa(t, m.goiThan(t, http.MethodPut, hostA, pathLetters+"/dt-a-001/result", canBoCua(xaA),
		`{"result_summary":"x","result_document_date":"25/09/2026"}`), http.StatusBadRequest)
}
